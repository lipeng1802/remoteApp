"""Explicit local-only stop; no registration, revoke, retirement or file removal."""
import json
from pathlib import Path
import time
from activity_control import Activity
from persistent_supervisor import Endpoint
from supervised_requests import reply
from local_cancel_proof import receipt,mac_file
import run_supervised_activity as r


def stop(work):
    work,m=r.metadata(work)
    if Activity(work).read()['phase'] not in ('owners_ready','exercise_outcome_unknown'):
        raise RuntimeError('local_cancel_episode_rejected')
    public={role:reply((work/(name+'-bootstrap.json')).read_bytes()) for role,name in (('controller','mac'),('target','windows'))}
    # Capability checks for BOTH owners precede any cancellation side effect.
    for role,b in public.items():
        if type(b.get('local_cancel_protocol')) is not int or b['local_cancel_protocol']!=1:
            raise RuntimeError('local_cancel_requires_new_capable_pair')
        keys=('process_id','supervisor_process_id') if role=='controller' else ('process_id',)
        if any(type(b.get(k)) is not int or b[k]<=0 for k in keys):raise RuntimeError('local_cancel_identity_rejected')
    journal=Activity(work);journal.path=work/'local-cancel-state.json'
    state=journal.read() if journal.path.exists() else {'version':1,'phase':'local_cancel_unknown','joined':{}}
    if state.get('phase') not in ('local_cancel_unknown','local_joined_remote_unverified') or type(state.get('joined')) is not dict or any(k not in ('controller','target') or v is not True for k,v in state['joined'].items()):
        raise RuntimeError('local_cancel_state_rejected')
    journal.save(state)
    marker=work/'mac-local-cancel-exit.json'
    if state['joined'].get('controller') and not marker.exists():raise RuntimeError('local_cancel_saved_proof_missing')
    if not marker.exists():
        value=Endpoint(m['holder_dir'])({'Op':'owner_stop_local_fault'})
        if value!={'status':'owner_local_cancel_closed','joined':True,'fault':True,'exit_code':1,'remote_revocation_verified':False} or type(value.get('joined')) is not bool or type(value.get('fault')) is not bool or type(value.get('exit_code')) is not int or type(value.get('remote_revocation_verified')) is not bool:
            raise RuntimeError('local_cancel_mac_unknown')
    for _ in range(120):
        if marker.exists():break
        time.sleep(.25)
    else:raise RuntimeError('local_cancel_mac_exit_unknown')
    value=reply(marker.read_bytes())
    expected={'version':1,'nonce':m['nonce'],'exit_code':1,'remote_revocation_verified':False}
    if value!=expected or any(type(value.get(k)) is not type(v) for k,v in expected.items()):raise RuntimeError('local_cancel_mac_exit_rejected')
    mac_file(Path(m['holder_dir'])/'owner-local-cancel-joined.json',public['controller'])
    state['joined']['controller']=True;journal.save(state)
    d=r.quote(m['win_dir']);task=r.quote('RemoteApp-R75-'+m['nonce'])
    if not state['joined'].get('target'):r.ps("$ErrorActionPreference='Stop';$p=Join-Path "+d+" 'stop-owner-local-fault.request';if(-not(Test-Path $p)){New-Item -ItemType File -Path $p | Out-Null}else{$i=Get-Item $p;if($i.PSIsContainer -or $i.Length -ne 0 -or ($i.Attributes -band [IO.FileAttributes]::ReparsePoint)){throw 'Unknown marker'}}")
    for _ in range(120):
        raw=r.ps("$ErrorActionPreference='Stop';$p=Join-Path "+d+" 'owner-local-cancel-joined.json';$t="+task+";if((Test-Path $p) -and (Get-ScheduledTask -TaskName $t).State -ne 'Running' -and (Get-ScheduledTaskInfo -TaskName $t).LastTaskResult -eq 1){$f=Get-Item $p;if($f.PSIsContainer -or $f.Length -gt 8192 -or ($f.Attributes -band [IO.FileAttributes]::ReparsePoint)){throw 'Unknown receipt'};[IO.File]::ReadAllText($p)}")
        if raw:receipt(reply(raw.encode('utf8')),'windows',public['target']);break
        time.sleep(.25)
    else:raise RuntimeError('local_cancel_windows_exit_unknown')
    state['joined']['target']=True;journal.save(state)
    state.update(phase='local_joined_remote_unverified',remote_revocation_verified=False)
    journal.save(state)
