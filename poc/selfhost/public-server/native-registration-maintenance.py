"""Approved root-local isolated acceptance window, CentOS Python 2 compatible.

No installation/config rewrite/Store restore/public port/automatic rollback.
Prepare captures public baseline; pause requires explicit STOPPED actual PID
receipts and released locks. Resume requires candidate joined, original resource
fingerprint and zero policy. Private provider JSON stays in server memory.
"""
from __future__ import print_function
import hashlib,json,os,re,stat,subprocess,sys,time

BASE='/var/lib/remoteapp-native-acceptance'
STORE='/var/lib/remoteapp-authority/state.json'
HS=['/opt/remoteapp-poc/headscale','-c','/etc/remoteapp-poc/config.yaml','-o','json']
SERVICES=('remoteapp-frontend','remoteapp-authority')
READY={'remoteapp-authority':'READY sole authority; supervised network enabled on private broker',
       'remoteapp-frontend':'READY unprivileged frontend; explicit direct signed network mode'}
EXPECTED='5c40c3230bd331b5ff47c9aefb18dd16c7f4e3d4d19f1ae18caed4caf1db33ca'

def require(ok,label):
    if not ok: raise RuntimeError(label)
def command(args):
    p=subprocess.Popen(args,stdout=subprocess.PIPE,stderr=subprocess.PIPE)
    out,unused=p.communicate()
    require(p.returncode==0,'command_failed')
    return out
def digest(path): return hashlib.sha256(open(path,'rb').read()).hexdigest()
def props(name):
    out=command(['systemctl','show',name,'-p','MainPID','-p','ExecMainStatus','-p','ActiveState','-p','ExecMainStartTimestampMonotonic'])
    return dict(line.split('=',1) for line in out.decode().splitlines())
def resource_fingerprint():
    users=json.loads(command(HS+['users','list']))
    nodes=json.loads(command(HS+['nodes','list']))
    keys=json.loads(command(HS+['preauthkeys','list']))
    require(len(users)==len(nodes)==len(keys)==1,'original_resource_count')
    public=[]
    for u in users: public.append(['user',str(u['id']),u['name']])
    for n in nodes: public.append(['node',str(n['id']),str(n['user']['id']),n['user']['name'],n['node_key'],sorted(n['ip_addresses'])])
    for k in keys: public.append(['key',str(k['id']),str(k['user']['id']),k['user']['name'],hashlib.sha256(k['key'].encode()).hexdigest()])
    require(json.loads(command(HS+['policy','get']))=={'acls':[]},'not_zero_policy')
    return hashlib.sha256(json.dumps(public,sort_keys=True).encode()).hexdigest()
def atomic(stage,state):
    path=stage+'/maintenance.json';tmp=path+'.new'
    require(not os.path.lexists(tmp) and not os.path.islink(path),'unsafe_checkpoint')
    fd=os.open(tmp,os.O_CREAT|os.O_EXCL|os.O_WRONLY,0o600)
    with os.fdopen(fd,'wb') as f: f.write(json.dumps(state,sort_keys=True).encode());f.flush();os.fsync(f.fileno())
    os.rename(tmp,path)
    fd=os.open(stage,os.O_RDONLY)
    try: os.fsync(fd)
    finally: os.close(fd)
def stage_valid(stage):
    require(os.geteuid()==0 and bool(re.match(r'^'+BASE+r'/remoteapp-native-[0-9a-f]{16}\Z',stage)) and os.path.realpath(stage)==stage,'invalid_stage')
    st=os.lstat(stage);require(st.st_uid==0 and stat.S_IMODE(st.st_mode)==0o700 and stat.S_ISDIR(st.st_mode),'stage_permissions')
    parent=os.lstat(BASE);require(parent.st_uid==0 and stat.S_IMODE(parent.st_mode)==0o700 and stat.S_ISDIR(parent.st_mode),'base_permissions')
def baseline(state):
    require(digest(STORE)==state['store_sha']==EXPECTED,'formal_store_changed')
    require(resource_fingerprint()==state['resources'],'resources_changed')
    require(digest('/etc/systemd/system/remoteapp-authority.service')==state['authority_unit'] and digest('/etc/systemd/system/remoteapp-frontend.service')==state['frontend_unit'],'unit_changed')
    command(['sha256sum','-c','/opt/remoteapp-prerelease/nginx-baseline.sha256'])
def no_locks():
    for p in ('/var/lib/remoteapp-authority/writer.lock','/var/lib/remoteapp-verifier/owner.lock'):
        require(not os.path.lexists(p),'owner_not_released')
def process_receipt(name,pid,started,label):
    require(pid>1 and started>0,'restore_start_identity_missing')
    lines=command(['journalctl','--no-pager','-b','-o','json','-u',name,'_PID='+str(pid)]).decode().splitlines()
    for line in lines:
        row=json.loads(line)
        if row.get('MESSAGE')==label and str(row.get('_PID'))==str(pid) and int(row.get('__MONOTONIC_TIMESTAMP','0'))>=started:return True
    return False
def wait_ready(name,label):
    for unused in range(450):
        p=props(name);pid=int(p['MainPID'])
        require(p['ActiveState']!='failed','restore_service_failed')
        if pid>1 and p['ActiveState']=='active':
            started=int(p.get('ExecMainStartTimestampMonotonic','0'))
            # Same PID can be reused. Require this boot and a receipt no older
            # than the current ExecStart, then recheck identity after journal IO.
            ready=process_receipt(name,pid,started,label)
            after=props(name)
            if ready and after['ActiveState']=='active' and after['MainPID']==str(pid) and after.get('ExecMainStartTimestampMonotonic')==str(started):return (pid,started)
        time.sleep(.1)
    raise RuntimeError('restore_not_ready')
def load_stage_json(stage,name):
    require(name in ('maintenance.json','candidate-joined.json'),'invalid_metadata_name')
    path=stage+'/'+name;st=os.lstat(path)
    require(stat.S_ISREG(st.st_mode) and st.st_uid==0 and stat.S_IMODE(st.st_mode)==0o600 and st.st_nlink==1 and 0<st.st_size<=16384,'unsafe_metadata')
    fd=os.open(path,os.O_RDONLY|os.O_NOFOLLOW)
    with os.fdopen(fd,'rb') as f:
        after=os.fstat(f.fileno())
        require((st.st_dev,st.st_ino,st.st_size,st.st_uid,st.st_mode,st.st_nlink)==(after.st_dev,after.st_ino,after.st_size,after.st_uid,after.st_mode,after.st_nlink),'metadata_changed')
        raw=f.read(16385)
    require(len(raw)<=16384,'metadata_too_large')
    return json.loads(raw)
def candidate_joined(stage):
    require(not os.path.lexists(stage+'/authority/writer.lock'),'candidate_store_owner_live')
    if os.path.lexists(stage+'/public-ready.json'):
        receipt=load_stage_json(stage,'candidate-joined.json')
        require(receipt.get('Version')==1 and isinstance(receipt.get('PID'),int) and not isinstance(receipt.get('PID'),bool) and receipt['PID']>1,'candidate_join_receipt_missing')
        try:os.kill(receipt['PID'],0)
        except OSError as e:require(e.errno==3,'candidate_exit_unknown')
        else:raise RuntimeError('candidate_still_live')
def service_inactive(name):
    p=props(name)
    require(p['ActiveState']=='inactive' and p['MainPID']=='0' and p['ExecMainStatus']=='0','original_service_not_joined')
def restore_services():
    require(props('remoteapp-poc')['ActiveState']=='active','provider_not_active')
    initial=dict((name,props(name)) for name in SERVICES)
    require(not (initial['remoteapp-authority']['ActiveState']=='inactive' and initial['remoteapp-frontend']['ActiveState']=='active'),'restore_order_ambiguous')
    identities={}
    for name in ('remoteapp-authority','remoteapp-frontend'):
        p=initial[name]
        if p['ActiveState']=='inactive':
            service_inactive(name)
            if name=='remoteapp-authority':no_locks()
            command(['systemctl','start',name])
        else:require(p['ActiveState']=='active','restore_service_failed')
        identities[name]=wait_ready(name,READY[name])
    current_services(identities)
def current_services(identities):
    # A dependency can fail/restart while the frontend reaches READY.
    require(props('remoteapp-poc')['ActiveState']=='active','provider_not_active')
    for name in identities:
        p=props(name)
        require(p['ActiveState']=='active' and (int(p['MainPID']),int(p.get('ExecMainStartTimestampMonotonic','0')))==identities[name],'restore_identity_changed')
def main(stage,mode):
    os.umask(0o077);stage_valid(stage)
    if mode=='prepare':
        require(not os.path.lexists(stage+'/maintenance.json'),'already_prepared')
        for name in SERVICES+('remoteapp-poc',):require(props(name)['ActiveState']=='active','service_not_active')
        state={'version':1,'phase':'prepared','store_sha':digest(STORE),'resources':resource_fingerprint(),
               'authority_unit':digest('/etc/systemd/system/remoteapp-authority.service'),'frontend_unit':digest('/etc/systemd/system/remoteapp-frontend.service'),
               'candidate':digest(stage+'/authority-candidate')}
        baseline(state);atomic(stage,state);print('PREPARED original_store_resources_units_and_nginx');return
    state=load_stage_json(stage,'maintenance.json');require(state['version']==1,'bad_checkpoint')
    if mode=='pause':
        require(state['phase']=='prepared','unexpected_phase');baseline(state)
        state['phase']='pausing';atomic(stage,state)
        for name,label in (('remoteapp-frontend','STOPPED frontend'),('remoteapp-authority','STOPPED authority')):
            p=props(name);pid=int(p['MainPID']);started=int(p.get('ExecMainStartTimestampMonotonic','0'));require(pid>1 and started>0 and p['ActiveState']=='active','not_active')
            command(['systemctl','stop',name])
            after=props(name)
            require(process_receipt(name,pid,started,label) and after['ActiveState']=='inactive' and after['MainPID']=='0' and after['ExecMainStatus']=='0','stop_join_not_confirmed')
        no_locks();baseline(state);state['phase']='paused';atomic(stage,state);print('PAUSED actual_stop_receipts_and_released_ownership');return
    if mode in ('resume','finish-resume'):
        require(state['phase']==('paused' if mode=='resume' else 'resuming'),'unexpected_phase')
        baseline(state);candidate_joined(stage)
        if mode=='resume':
            no_locks()
            for name in SERVICES:service_inactive(name)
            state['phase']='resuming';atomic(stage,state)
        # Explicit continuation only, never an automatic rollback/retry. Reuse
        # healthy original instances; a failed unit is NOT silently restarted.
        restore_services()
        baseline(state);state['phase']='restored';atomic(stage,state);print('RESTORED original_services_store_resources_zero_policy_nginx');return
    if mode=='verify-restored':
        require(state['phase']=='restored','unexpected_phase');baseline(state);candidate_joined(stage)
        require(props('remoteapp-poc')['ActiveState']=='active','provider_not_active')
        identities=dict((name,wait_ready(name,READY[name])) for name in SERVICES)
        current_services(identities)
        baseline(state);print('VERIFIED_RESTORED current_process_ready_and_unchanged_baseline');return
    raise RuntimeError('unknown_mode')

if __name__=='__main__':
    try:
        require(len(sys.argv)==3,'invalid_arguments');main(sys.argv[1],sys.argv[2])
    except Exception as e:
        print('FAIL '+(str(e) if isinstance(e,RuntimeError) else 'maintenance_internal'));sys.exit(1)
