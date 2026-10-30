"""Explicit fault recovery for the same TEST owner pair. Never normal success."""
import json
import os
from pathlib import Path
import stat
import time
from activity_control import Activity
from persistent_supervisor import Endpoint
from supervised_requests import WindowsRelay,reply
import run_supervised_activity as r


def journal(work):
    value=Activity(work);value.path=work/'fault-state.json';return value


def bootstrap(work,role):
    value=reply((work/(role+'-bootstrap.json')).read_bytes())
    if type(value.get('fault_protocol')) is not int or value['fault_protocol']!=1:
        raise RuntimeError('fault_fresh_capable_owner_required')
    for key in (('process_id','supervisor_process_id') if role=='mac' else ('process_id',)):
        if type(value.get(key)) is not int or value[key]<=0:raise RuntimeError('fault_owner_pid_rejected')
    return value


def receipt(value,role,public):
    expected={'version':1,'status':'owner_fault_closed','joined':True,'fault':True,'exit_code':1}
    if role=='windows':expected['process_id']=public['process_id']
    else:expected.update(supervisor_process_id=public['supervisor_process_id'],holder_process_id=public['process_id'])
    if value!=expected or any(type(value[k]) is not bool for k in ('joined','fault')):
        raise RuntimeError('fault_receipt_rejected')
    if any(type(value[k]) is not int for k in expected if k not in ('status','joined','fault')):
        raise RuntimeError('fault_receipt_rejected')
    if any(value[k]<=0 for k in expected if k.endswith('_id')):raise RuntimeError('fault_receipt_rejected')
    return value


def local_receipt(work,m):
    path=Path(m['holder_dir'])/'owner-fault-joined.json';info=path.lstat()
    if not stat.S_ISREG(info.st_mode) or info.st_uid!=os.getuid() or info.st_size>8192:
        raise RuntimeError('fault_receipt_file_rejected')
    return receipt(reply(path.read_bytes()),'mac',bootstrap(work,'mac'))


def revoke(work):
    work,m=r.metadata(work);j=journal(work)
    if Activity(work).read()['phase']!='exercise_outcome_unknown':raise RuntimeError('fault_episode_required')
    if j.path.exists() and j.read()['phase']!='fault_revoke_unknown':raise RuntimeError('fault_explicit_revoke_not_admitted')
    public=bootstrap(work,'windows');bootstrap(work,'mac')
    j.save({'version':1,'phase':'fault_revoke_unknown'})
    target=WindowsRelay(r.WIN,m['win_dir']+'\\supervised-control-relay.exe',public['control_pipe'])
    result=target({'Op':'revoke_session'})
    if result!={'status':'session_revoked','passed':True} or result.get('passed') is not True:
        raise RuntimeError('fault_revoke_unconfirmed_preserve_owners')
    j.save({'version':1,'phase':'fault_revocation_confirmed'})


def mac_join(work,m):
    marker=work/'mac-exit.json'
    if not marker.exists():
        result=Endpoint(m['holder_dir'])({'Op':'owner_stop_fault'})
        if result!={'status':'owner_fault_closed','joined':True,'fault':True,'exit_code':1} or any(type(result[k]) is not bool for k in ('joined','fault')) or type(result.get('exit_code')) is not int:
            raise RuntimeError('fault_mac_join_unconfirmed')
    for unused in range(100):
        if marker.exists():break
        time.sleep(.2)
    else:raise RuntimeError('fault_mac_monitor_unconfirmed')
    result=reply(marker.read_bytes())
    if result!={'version':1,'nonce':m['nonce'],'exit_code':1} or type(result['version']) is not int or type(result['exit_code']) is not int:
        raise RuntimeError('fault_mac_exit_rejected')
    local_receipt(work,m)
    if any((Path(m['holder_dir'])/n).exists() or (Path(m['holder_dir'])/n).is_symlink() for n in ('owner.json','owner.sock')):
        raise RuntimeError('fault_mac_metadata_not_clean')


def windows_join(work,m):
    public=bootstrap(work,'windows');d=r.quote(m['win_dir']);task=r.quote('RemoteApp-R75-'+m['nonce'])
    r.ps("$ErrorActionPreference='Stop';$p=Join-Path "+d+" 'stop-owner-fault.request';if(-not(Test-Path $p)){New-Item -ItemType File -Path $p | Out-Null}else{$f=Get-Item $p;if($f.PSIsContainer -or $f.Length -ne 0 -or ($f.Attributes -band [IO.FileAttributes]::ReparsePoint)){throw 'Unknown marker'}}")
    for unused in range(120):
        raw=r.ps("$ErrorActionPreference='Stop';$d="+d+";$p=Join-Path $d 'owner-fault-joined.json';$t="+task+";if((Test-Path $p) -and -not(Test-Path (Join-Path $d 'owner-ready.json')) -and (Get-ScheduledTask -TaskName $t).State -ne 'Running' -and (Get-ScheduledTaskInfo -TaskName $t).LastTaskResult -eq 1){$f=Get-Item $p;if($f.PSIsContainer -or $f.Length -gt 8192 -or ($f.Attributes -band [IO.FileAttributes]::ReparsePoint)){throw 'Unknown receipt'};[IO.File]::ReadAllText($p)}")
        if raw:
            result=receipt(reply(raw.encode('utf8')),'windows',public)
            (work/'windows-fault-joined.json').write_text(json.dumps(result,sort_keys=True));return
        time.sleep(.2)
    raise RuntimeError('fault_windows_join_unconfirmed_preserve_owner')


def stop(work):
    work,m=r.metadata(work);j=journal(work);state=j.read()
    if state['phase'] not in ('fault_revocation_confirmed','fault_stop_unknown'):raise RuntimeError('fault_revocation_confirmation_required')
    bootstrap(work,'mac');bootstrap(work,'windows')
    joined=state.get('joined',{})
    if not isinstance(joined,dict) or any(k not in ('controller','target') or v is not True for k,v in joined.items()):raise RuntimeError('fault_join_state_rejected')
    state.update(phase='fault_stop_unknown',joined=joined);j.save(state)
    for role,call in (('controller',mac_join),('target',windows_join)):
        if joined.get(role) is not True:call(work,m);joined[role]=True;j.save(state)
    state['phase']='owners_fault_joined';j.save(state)


def retire(work):
    work,m=r.metadata(work);j=journal(work);state=j.read()
    if state['phase']!='owners_fault_joined' or state.get('joined')!={'controller':True,'target':True} or any(v is not True for v in state['joined'].values()):
        raise RuntimeError('fault_both_joins_required')
    # Re-check the actual published proof/monitor, not the phase file alone.
    mac_join(work,m)
    windows_join(work,m)
    def cleanup():
        path=Path(m['holder_dir'])/'owner-fault-joined.json';first=path.lstat();local_receipt(work,m);last=path.lstat()
        if (first.st_dev,first.st_ino)!=(last.st_dev,last.st_ino):raise RuntimeError('fault_receipt_replaced')
        path.unlink()
    return r.retire_pair(work,m,j,state,'fault_joined_and_test_roots_retired',cleanup)
