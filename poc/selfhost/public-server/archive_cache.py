"""TEST byte transport only. Cached executables are never started or trusted.

Reconstructed output must equal every newly built file's full SHA-256. The
ordinary signed inventory, fresh profile and native installation gates follow.
"""
import hashlib
import io
import json
from pathlib import Path
import re
import tarfile
import tempfile

BLOCK=32768
NAME=re.compile(r'[A-Za-z0-9_.-]+(?:/[A-Za-z0-9_.-]+)*\Z')


def digest(path):
    h=hashlib.sha256()
    with path.open('rb') as f:
        while b:=f.read(1<<20):h.update(b)
    return h.hexdigest()


def files(directory,omit=()):
    result={}
    for p in sorted(Path(directory).rglob('*')):
        if p.is_symlink():raise RuntimeError('archive_cache_link_rejected')
        if p.is_dir():continue
        name=p.relative_to(directory).as_posix()
        if not p.is_file() or not NAME.fullmatch(name) or name.startswith('_') or any(x in ('.','..') or x.strip(' .')!=x for x in name.split('/')):
            raise RuntimeError('archive_cache_name_rejected')
        if name in omit:continue
        if name.lower() in (n.lower() for n in result):raise RuntimeError('archive_cache_duplicate')
        result[name]=p
    return result


def spec(name,path):return {'name':name,'size':path.stat().st_size,'sha256':digest(path)}


def pack(directory,archive,baseline,restorer,omit=()):
    current=files(directory,omit);cached=files(baseline)
    inventory={name:spec(name,p) for name,p in current.items()}
    helper_digest=digest(restorer)
    index={};cache_specs={}
    for name,p in cached.items():
        if not name.endswith('.exe'):continue
        cache_specs[name]=spec(name,p)
        with p.open('rb') as f:
            while b:=f.read(BLOCK):
                offset=f.tell()-len(b)
                index.setdefault((len(b),hashlib.sha256(b).digest()),(name,offset))
    plan={'version':1,'block':BLOCK,'literal':None,'sources':[],'targets':[]}
    source_ids={};literal_index={};copied=0;total=0
    with tempfile.TemporaryDirectory(prefix='archive-cache-',dir=archive.parent) as temporary:
        literal=Path(temporary)/'literal.bin'
        with literal.open('xb') as out:
            for name,p in current.items():
                if not name.endswith('.exe'):continue
                target=spec(name,p);target['steps']=[]
                with p.open('rb') as f:
                    while b:=f.read(BLOCK):
                        key=(len(b),hashlib.sha256(b).digest());total+=len(b)
                        if key in index:
                            source,offset=index[key]
                            if source not in source_ids:
                                plan['sources'].append(cache_specs[source]);source_ids[source]=len(plan['sources'])
                            target['steps'].append([source_ids[source],offset,len(b)]);copied+=len(b)
                        else:
                            if key not in literal_index:
                                literal_index[key]=out.tell();out.write(b)
                            target['steps'].append([0,literal_index[key],len(b)])
                if spec(name,p)!={k:target[k] for k in ('name','size','sha256')}:
                    raise RuntimeError('archive_cache_input_changed')
                plan['targets'].append(target)
        if not plan['targets'] or total>256<<20 or len(plan['sources'])>64 or len(plan['targets'])>64:
            raise RuntimeError('archive_cache_budget_rejected')
        plan['literal']=spec('literal.bin',literal)
        raw=json.dumps(plan,sort_keys=True,separators=(',',':')).encode()
        if len(raw)>4<<20:raise RuntimeError('archive_cache_budget_rejected')
        with tarfile.open(archive,'x:xz',preset=3) as bundle:
            def add(name,p=None,raw=None):
                info=tarfile.TarInfo(name);info.size=len(raw) if raw is not None else p.stat().st_size
                info.mode=0o500 if name.endswith('.exe') else 0o600
                if raw is not None:bundle.addfile(info,io.BytesIO(raw))
                else:
                    with p.open('rb') as f:bundle.addfile(info,f)
            for name,p in current.items():
                if not name.endswith('.exe'):add(name,p)
            add('_transport/plan.json',raw=raw)
            add('_transport/literal.bin',literal)
            add('_transport/restore.exe',restorer)
    if inventory!={name:spec(name,p) for name,p in current.items()} or digest(restorer)!=helper_digest:
        raise RuntimeError('archive_cache_input_changed')
    return {'version':1,'mode':'block_cache','archive_sha256':digest(archive),'files':inventory,
            'output_bytes':total,'copied_bytes':copied,'literal_bytes':plan['literal']['size'],
            'archive_bytes':archive.stat().st_size,'restorer_sha256':helper_digest}


def verify(directory,archive,audit,omit=()):
    expected={'version','mode','archive_sha256','files','output_bytes','copied_bytes','literal_bytes','archive_bytes','restorer_sha256'}
    if type(audit) is not dict or set(audit)!=expected or type(audit['version']) is not int or audit['version']!=1 or audit['mode']!='block_cache' or archive.is_symlink() or digest(archive)!=audit['archive_sha256'] or archive.stat().st_size!=audit['archive_bytes']:
        raise RuntimeError('archive_cache_audit_rejected')
    if any(type(audit[k]) is not int or not 0<=audit[k]<=256<<20 for k in ('output_bytes','copied_bytes','literal_bytes','archive_bytes')) or not re.fullmatch('[0-9a-f]{64}',audit['restorer_sha256']):
        raise RuntimeError('archive_cache_audit_rejected')
    inventory={name:spec(name,p) for name,p in files(directory,omit).items()}
    if inventory!=audit['files']:raise RuntimeError('archive_cache_input_changed')

