"""Explicit trusted TEST management steps, never implicit finally teardown.

Only public paths/bootstrap/phase evidence reach disk. Package keys stay inside
the existing trusted builder. This is SSH test coordination, not product UX.
"""
import base64
import hashlib
import json
import os
from pathlib import Path
import secrets
import shutil
import subprocess
import sys
import tarfile
import time

from activity_control import Activity
from activity_trace import RecordedEndpoint
from persistent_supervisor import Endpoint
from supervised_requests import WindowsRelay, reply as strict_reply

REPO=Path(__file__).resolve().parents[3]
WIN="jarvis@100.73.4.118"
SERVER="root@182.92.117.114"
OPTIONS=["-o","BatchMode=yes","-o","ConnectTimeout=12","-o","IPQoS=none",
         "-o","ServerAliveInterval=5","-o","ServerAliveCountMax=2"]


def command(args,timeout=90):
    try:r=subprocess.run(args,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=timeout)
    except Exception:raise RuntimeError("activity_management_outcome_unknown") from None
    if r.returncode:raise RuntimeError("activity_management_command_failed")
    return r.stdout


def ps(body):
    # Windows PowerShell otherwise emits the desktop OEM code page over SSH.
    # Public localized diagnostics must not break the UTF-8 management reader.
    body="$OutputEncoding=[Text.UTF8Encoding]::new($false);[Console]::OutputEncoding=$OutputEncoding;"+body
    encoded=base64.b64encode(body.encode("utf-16le")).decode()
    if len(encoded)>6500:raise RuntimeError("activity_script_requires_file_upload")
    return command(["ssh",*OPTIONS,WIN,"powershell.exe -NoProfile -EncodedCommand "+encoded]).decode("utf8").strip()


def quote(value):return "'"+value.replace("'","''")+"'"


def pack_windows(directory,archive,omit=()):
    # Solid compression shares repeated Go/runtime bytes across the bundle.
    # Windows' own bsdtar/liblzma extracts it; no extra installer/service.
    with tarfile.open(archive,"x:xz",preset=3) as output:
        for path in sorted(Path(directory).rglob("*")):
            if path.is_symlink():raise RuntimeError("activity_archive_link_rejected")
            if path.is_dir():continue
            if not path.is_file():raise RuntimeError("activity_archive_type_rejected")
            name=path.relative_to(directory).as_posix()
            if any(part in ("..","") for part in name.split("/")) or ":" in name:raise RuntimeError("activity_archive_name_rejected")
            if name in omit:continue
            info=tarfile.TarInfo(name);info.size=path.stat().st_size;info.mode=0o500 if path.suffix==".exe" else 0o600
            with path.open("rb") as source:output.addfile(info,source)


def verified_windows_inputs(work):
    # Reuse only immutable executable bytes, NEVER the old installed root,
    # identity, node directory, native journal or prior authorization.
    old="C:\\Users\\jarvis\\remoteapp-r67-package-156f90fb5c70\\payload"
    files=[]
    for path in (work/"windows").rglob("*"):
        if path.is_file() and path.name in ("trust-journal.exe","identity-vault.exe","tls-product.exe"):
            files.append({"relative":path.relative_to(work/"windows").as_posix(),"name":path.name,"sha256":hashlib.sha256(path.read_bytes()).hexdigest()})
    body="$ErrorActionPreference='Stop';$s="+quote(old)+";"
    for item in files:
        body+="if((Get-FileHash (Join-Path $s "+quote(item["name"])+")).Hash.ToLowerInvariant() -ne "+quote(item["sha256"])+"){throw 'Input mismatch'};"
    body+="'INPUTS_VERIFIED'"
    try:verified=ps(body)=="INPUTS_VERIFIED"
    except RuntimeError:raise RuntimeError("activity_input_verification_unavailable") from None
    if not verified or not files:raise RuntimeError("activity_input_verification_unavailable")
    return old,files


def test_build_wrapper(work,go):
    # Only this explicit TEST runner uses stripped compiler output. Preserve
    # every original embedded -X anchor/path/digest; sign the final bytes as usual.
    wrapper=work/'test-go-build'
    source='''#!PYTHON
import os,sys
actual=ACTUAL
args=sys.argv[1:]
if args and args[0]=='build':
    found=False
    for i,arg in enumerate(args):
        if arg.startswith('-ldflags='):
            args[i]='-ldflags=-s -w '+arg[len('-ldflags='):];found=True
    if not found:args.insert(1,'-ldflags=-s -w')
os.execv(actual,[actual,*args])
'''.replace('PYTHON',sys.executable,1).replace('ACTUAL',repr(str(go)),1)
    with wrapper.open('x') as f:f.write(source)
    wrapper.chmod(0o500)
    return wrapper


def metadata(work):
    work=Path(work).resolve(strict=True)
    m=json.loads((work/"management.json").read_text())
    n=m["nonce"]
    if len(n)!=12 or any(c not in "0123456789abcdef" for c in n):raise RuntimeError("activity_nonce_invalid")
    if (m["win_dir"]!="C:\\Users\\jarvis\\remoteapp-r75-package-"+n
            or m["win_root"]!="C:\\Users\\jarvis\\remoteapp-r75-"+n
            or m["mac_root"]!="/private/tmp/remoteapp-r75-"+n+"/installed"
            or m["holder_dir"]!="/private/tmp/remoteapp-r75-"+n+"/holder"):
        raise RuntimeError("activity_fixed_roots_invalid")
    return work,m


def prepare():
    n=secrets.token_hex(6)
    work=REPO/"artifacts/connection-poc"/("supervised-activity-r75-"+n)
    work.mkdir(mode=0o700)
    parent=Path("/private/tmp/remoteapp-r75-"+n);parent.mkdir(mode=0o700)
    (parent/"holder").mkdir(mode=0o700)
    m=dict(nonce=n,win_dir="C:\\Users\\jarvis\\remoteapp-r75-package-"+n,
           win_root="C:\\Users\\jarvis\\remoteapp-r75-"+n,mac_root=str(parent/"installed"),holder_dir=str(parent/"holder"))
    (work/"management.json").write_text(json.dumps(m,sort_keys=True))
    a=Activity(work);a.save({"version":1,"phase":"build_outcome_unknown"})
    prior=REPO/"artifacts/connection-poc/supervised-consent-r67-156f90fb5c70"
    # Read-only availability check BEFORE minting any short-lived TEST profile.
    _,inputs=verified_windows_inputs(prior)
    (work/'immutable-input-preflight.json').write_text(json.dumps(inputs,sort_keys=True))
    profile=work/"service-profile.json"
    profile.write_bytes(command(["ssh",*OPTIONS,SERVER,"cat /var/lib/remoteapp-prerelease/client-public/service-profile.json"]))
    go=REPO/"artifacts/connection-poc/toolchain/go/bin/go"
    build_go=test_build_wrapper(work,go)
    for target in ("darwin","windows"):
        suffix=".exe" if target=="windows" else ""
        source=prior/target/"payload"
        manifest=json.loads((source/"release-manifest.json").read_text())["manifest"]
        helpers=[]
        for name in ("trust-journal","identity-vault","tls-product"):
            path=source/(name+suffix)
            expected=next(f["sha256"] for f in manifest["files"] if f["name"]==name+suffix)
            if hashlib.sha256(path.read_bytes()).hexdigest()!=expected:raise RuntimeError("activity_helper_source_changed")
            helpers.append(str(path))
        args=[str(go),"run","./cmd/product-maintenance-validation-build",str(build_go),str(REPO/"poc/selfhost"),target,str(work/target),*helpers,
              "--verified-live-profile",str(profile),"--test-tunnel","--test-install-root",m["win_root"] if target=="windows" else m["mac_root"]]
        # Go's cwd is fixed without changing this process's working directory.
        try:r=subprocess.run(args,cwd=REPO/"poc/selfhost",stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=240)
        except Exception:raise RuntimeError("activity_build_outcome_unknown") from None
        if r.returncode:raise RuntimeError("activity_build_failed")
    a.save({"version":1,"phase":"built"})
    return work


def expiry(work):
    values=[]
    for target in ("darwin","windows"):
        spec=json.loads(base64.b64decode(json.loads((work/target/"fixture.json").read_text())["release"]))
        if spec.get("test_namespace") is not True:raise RuntimeError("activity_test_bundle_required")
        values.append(spec["bundled_profile"]["claims"]["expires_at"])
    return min(values)


def monitor(work):
    work,m=metadata(work)
    result=subprocess.run([sys.executable,str(Path(__file__).with_name("persistent_supervisor.py")),m["holder_dir"],str(work/"darwin/supervisor")],stdin=subprocess.DEVNULL)
    # A dedicated resident manager owns wait(), so a later CLI can inspect
    # actual holder exit status, never substitute PID disappearance for exit0.
    out=work/"mac-exit.json"
    with out.open("x") as f:
        json.dump({"version":1,"nonce":m["nonce"],"exit_code":result.returncode},f);f.flush();os.fsync(f.fileno())


def start(work):
    work,m=metadata(work);a=Activity(work)
    if a.read()["phase"]!="built" or expiry(work)-time.time()<600:raise RuntimeError("activity_fresh_build_required")
    a.save({"version":1,"phase":"startup_outcome_unknown"})
    shutil.copyfile(REPO/"packaging/experimental/validate-supervised-install.ps1",work/"windows/validate-supervised-install.ps1")
    shutil.copyfile(REPO/"packaging/experimental/supervised-fault-stop.ps1",work/"windows/supervised-fault-stop.ps1")
    # Wrapper contains no grant/key and preserves actual desktop script errors.
    (work/"windows/run-owner.ps1").write_text("""$ErrorActionPreference='Stop'
$ProgressPreference='SilentlyContinue'
$log=Join-Path $PSScriptRoot 'owner-result.txt'
try { & (Join-Path $PSScriptRoot 'validate-supervised-install.ps1') -StayReady -RetainUntilStop *>&1 | Out-File $log -Encoding utf8
 'EXITCODE=0' | Out-File $log -Append -Encoding utf8
} catch { 'OWNER_FAILED_STATE_PRESERVED' | Out-File $log -Append -Encoding utf8; 'EXITCODE=1' | Out-File $log -Append -Encoding utf8;exit 1 }
""")
    archive=work/"windows.tar.xz"
    input_source,inputs=verified_windows_inputs(work)
    pack_windows(work/"windows",archive,omit={item["relative"] for item in inputs})
    remote="remoteapp-r75-"+m["nonce"]+".tar.xz"
    command(["scp","-q",*OPTIONS,str(archive),WIN+":"+remote],600)
    if expiry(work)-time.time()<240:raise RuntimeError("activity_transfer_consumed_validity_window")
    d=quote(m["win_dir"]);task=quote("RemoteApp-R75-"+m["nonce"])
    digest=quote(hashlib.sha256(archive.read_bytes()).hexdigest())
    ps("$ErrorActionPreference='Stop';$d="+d+";if((Test-Path $d) -or (Test-Path "+quote(m["win_root"])+")){throw 'Fresh roots required'};"
       "$z=Join-Path $env:USERPROFILE "+quote(remote)+";if((Get-FileHash $z).Hash.ToLowerInvariant() -ne "+digest+"){throw 'Package mismatch'};New-Item -ItemType Directory -Path $d | Out-Null;& tar.exe -xf $z -C $d;if($LASTEXITCODE -ne 0){throw 'Package extraction failed'}")
    for item in inputs:
        ps("$ErrorActionPreference='Stop';$s=Join-Path "+quote(input_source)+" "+quote(item["name"])+";$p=Join-Path "+d+" "+quote(item["relative"].replace('/','\\'))+";if(Test-Path $p){throw 'Input target exists'};"
           "if((Get-FileHash $s).Hash.ToLowerInvariant() -ne "+quote(item["sha256"])+"){throw 'Input changed'};Copy-Item -LiteralPath $s -Destination $p;if((Get-FileHash $p).Hash.ToLowerInvariant() -ne "+quote(item["sha256"])+"){throw 'Copied input mismatch'}")
    tunnels=[]
    for label,host,flags in (("server",SERVER,["-L","127.0.0.1:18444:127.0.0.1:8444"]),("windows",WIN,["-R","127.0.0.1:18444:127.0.0.1:18444"])):
        sock="/tmp/ra75-"+m["nonce"]+"-"+label+".sock"
        command(["ssh",*OPTIONS,"-M","-S",sock,"-fNT","-o","ExitOnForwardFailure=yes",*flags,host])
        tunnels.append({"socket":sock,"host":host})
        (work/"tunnels.json").write_text(json.dumps(tunnels))
    ps("$ErrorActionPreference='Stop';$d="+d+";$t="+task+";if(Get-ScheduledTask -TaskName $t -ErrorAction SilentlyContinue){throw 'Task exists'};"
       "$a=New-ScheduledTaskAction -Execute powershell.exe -Argument ('-NoProfile -ExecutionPolicy Bypass -File '+[char]34+(Join-Path $d 'run-owner.ps1')+[char]34);"
       "$p=New-ScheduledTaskPrincipal -UserId jarvis -LogonType Interactive -RunLevel Limited;Register-ScheduledTask -TaskName $t -Action $a -Principal $p | Out-Null;Start-ScheduledTask -TaskName $t")
    for unused in range(80):
        raw=ps("$d="+d+";$p=Join-Path $d 'owner-ready.json';if(Test-Path $p){[IO.File]::ReadAllText($p)}elseif(Test-Path (Join-Path $d 'owner-result.txt')){if((Get-Content (Join-Path $d 'owner-result.txt') -Raw) -match 'EXITCODE=1'){'FAILED'}}")
        if raw=="FAILED":raise RuntimeError("activity_windows_setup_failed_explicit_recovery_required")
        if raw:break
        time.sleep(.5)
    else:raise RuntimeError("activity_windows_ready_unconfirmed")
    public=json.loads(raw)
    WindowsRelay(WIN,m["win_dir"]+"\\supervised-control-relay.exe",public["control_pipe"])
    (work/"windows-bootstrap.json").write_text(json.dumps(public))
    start_mac(work,m)


def setup_mac(work,m):
    # Never replay an install after an unknown reply or overwrite a partial root.
    if Path(m["mac_root"]).exists() or (work/"mac-setup-result.json").exists():
        raise RuntimeError("activity_mac_setup_explicit_recovery_required")
    with (work/"mac-setup-attempt.json").open("x") as output:
        json.dump({"version":1,"phase":"outcome_unknown"},output);output.flush();os.fsync(output.fileno())
    descriptor=os.open(work,os.O_RDONLY)
    try:os.fsync(descriptor)
    finally:os.close(descriptor)
    try:
        result=subprocess.run([str(work/"darwin/test-setup")],stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=90)
    except Exception:raise RuntimeError("activity_mac_setup_outcome_unknown") from None
    value={"version":1,"status":"unavailable","stage":"unavailable","exit_code":result.returncode}
    stages={"compiled_configuration","signed_payload","protected_layout","signed_import","payload_promote",
            "bootstrap_import","native_trust","native_activation","native_enrollment",
            "native_identity_intent","native_identity_reservation","native_identity_scope","native_identity_create","native_enrollment_scope",
            "native_enrollment_initialize","native_binding_commit","native_identity_join"}
    try:
        raw=strict_reply(result.stdout) if len(result.stdout)<=4096 else None
        if type(raw) is dict and raw=={"status":"test_installed"} and result.returncode==0:
            value.update(status="test_installed",stage="completed")
        elif type(raw) is dict and set(raw)=={"status","stage"} and raw["status"]=="setup_failed_state_preserved" and type(raw["stage"]) is str and raw["stage"] in stages and result.returncode!=0:
            value.update(status="setup_failed_state_preserved",stage=raw["stage"])
    except Exception:pass
    with (work/"mac-setup-result.json").open("x") as output:
        json.dump(value,output,sort_keys=True);output.flush();os.fsync(output.fileno())
    if value["status"]!="test_installed":raise RuntimeError("activity_mac_setup_unconfirmed")


def start_mac(work,m):
    a=Activity(work)
    setup_mac(work,m)
    with (work/"mac-public.log").open("x") as log:
        manager=subprocess.Popen([sys.executable,str(Path(__file__)),str(work),"monitor"],stdin=subprocess.DEVNULL,stdout=log,stderr=subprocess.STDOUT,start_new_session=True)
    (work/"mac-manager.json").write_text(json.dumps({"pid":manager.pid}))
    ready=Path(m["holder_dir"])/"owner.json"
    for unused in range(100):
        if (work/"mac-exit.json").exists():raise RuntimeError("activity_mac_holder_failed")
        if ready.exists():break
        time.sleep(.2)
    else:raise RuntimeError("activity_mac_ready_unconfirmed")
    public=json.loads(ready.read_text());(work/"mac-bootstrap.json").write_text(json.dumps(public))
    if Endpoint(m["holder_dir"])({"Op":"owner_status"})!={"status":"owner_status","joined":False}:raise RuntimeError("activity_mac_owner_not_resident")
    a.save({"version":1,"phase":"owners_ready"})


def resume_windows(work):
    work,m=metadata(work)
    if (Activity(work).read()["phase"]!="startup_outcome_unknown" or expiry(work)-time.time()<300
            or Path(m["mac_root"]).exists() or (work/"mac-manager.json").exists()):
        raise RuntimeError("activity_windows_recovery_not_admitted")
    d=quote(m["win_dir"]);task=quote("RemoteApp-R75-"+m["nonce"])
    ps("$ErrorActionPreference='Stop';$d="+d+";$t="+task+";if((Get-ScheduledTask -TaskName $t).State -eq 'Running' -or (Test-Path (Join-Path $d 'owner-ready.json'))){throw 'Owner still resident'};"
       "$log=Get-Content (Join-Path $d 'owner-result.txt') -Raw;if($log -notmatch 'EXITCODE=1'){throw 'Prior failure not recorded'};"
       "$root="+quote(m["win_root"])+";$live=@(Get-CimInstance Win32_Process | Where-Object {$_.ExecutablePath -and ($_.ExecutablePath.StartsWith($d+'\\',[StringComparison]::OrdinalIgnoreCase) -or $_.ExecutablePath.StartsWith($root+'\\',[StringComparison]::OrdinalIgnoreCase))});if($live.Count){throw 'Native process remains'}")
    source=(work/"windows/run-owner.ps1").read_text()
    if source.count("-RetainUntilStop")!=1 or "-ResumeFiles" in source:raise RuntimeError("activity_recovery_wrapper_changed")
    with (work/"windows-resume-attempted.json").open("x") as f:json.dump({"explicit_resume_files":True},f)
    recovered=work/"resume-owner.ps1";recovered.write_text(source.replace("-RetainUntilStop","-RetainUntilStop -ResumeFiles"))
    command(["scp","-q",*OPTIONS,str(recovered),WIN+":"+"remoteapp-r75-package-"+m["nonce"]+"/resume-owner.ps1"])
    ps("$ErrorActionPreference='Stop';$d="+d+";Move-Item -LiteralPath (Join-Path $d 'owner-result.txt') -Destination (Join-Path $d 'first-owner-failure.txt');"
       "Copy-Item -LiteralPath (Join-Path $d 'resume-owner.ps1') -Destination (Join-Path $d 'run-owner.ps1');Start-ScheduledTask -TaskName "+task)
    for unused in range(80):
        raw=ps("$d="+d+";$p=Join-Path $d 'owner-ready.json';if(Test-Path $p){[IO.File]::ReadAllText($p)}elseif(Test-Path (Join-Path $d 'owner-result.txt')){if((Get-Content (Join-Path $d 'owner-result.txt') -Raw) -match 'EXITCODE=1'){'FAILED'}}")
        if raw=="FAILED":raise RuntimeError("activity_explicit_setup_recovery_failed")
        if raw:break
        time.sleep(.5)
    else:raise RuntimeError("activity_recovery_ready_unconfirmed")
    public=json.loads(raw)
    WindowsRelay(WIN,m["win_dir"]+"\\supervised-control-relay.exe",public["control_pipe"])
    (work/"windows-bootstrap.json").write_text(json.dumps(public))
    start_mac(work,m)


def exercise(work):
    work,m=metadata(work)
    public=json.loads((work/"windows-bootstrap.json").read_text())
    target=WindowsRelay(WIN,m["win_dir"]+"\\supervised-control-relay.exe",public["control_pipe"])
    target=RecordedEndpoint(work,'target',target)
    controller=RecordedEndpoint(work,'controller',Endpoint(m["holder_dir"]))
    try:
        return Activity(work).exercise(target,controller,expiry(work))
    except Exception:
        # Cache-only native telemetry cannot authorize retries, close the owner,
        # or replace the original failure with a diagnostic transport failure.
        from startup_diagnostic import capture
        try:capture(work,target,controller)
        except Exception:pass
        raise


def stop(work):
    work,m=metadata(work)
    def mac():
        marker=work/"mac-exit.json"
        if not marker.exists():
            receipt=Endpoint(m["holder_dir"])({"Op":"owner_stop"})
            if receipt!={"status":"owner_closed","joined":True,"zero_exit":True}:raise RuntimeError("activity_mac_stop_unconfirmed")
        for unused in range(120):
            if marker.exists():break
            time.sleep(.25)
        else:raise RuntimeError("activity_mac_wait_unconfirmed")
        value=json.loads(marker.read_text())
        if value!={"version":1,"nonce":m["nonce"],"exit_code":0} or type(value.get("exit_code")) is not int:
            raise RuntimeError("activity_mac_exit_not_zero")
        return {"status":"owner_closed","joined":True,"zero_exit":True}
    def windows():
        d=quote(m["win_dir"]);task=quote("RemoteApp-R75-"+m["nonce"])
        ps("$ErrorActionPreference='Stop';$p=Join-Path "+d+" 'stop-owner.request';if(-not(Test-Path $p)){New-Item -ItemType File -Path $p | Out-Null}else{$f=Get-Item $p;if($f.PSIsContainer -or $f.Length -ne 0 -or ($f.Attributes -band [IO.FileAttributes]::ReparsePoint)){throw 'Unknown stop marker'}}")
        for unused in range(120):
            value=ps("$ErrorActionPreference='Stop';$d="+d+";$t="+task+";$log=Join-Path $d 'owner-result.txt';if(Test-Path $log){$s=Get-Content $log -Raw;if($s -match 'EXITCODE=1'){'FAILED'}elseif(($s -match 'EXITCODE=0') -and -not(Test-Path (Join-Path $d 'owner-ready.json')) -and (Get-ScheduledTask -TaskName $t).State -ne 'Running' -and (Get-ScheduledTaskInfo -TaskName $t).LastTaskResult -eq 0){'JOINED'}}")
            if value=="JOINED":return {"status":"owner_closed","joined":True,"zero_exit":True}
            if value=="FAILED":break
            time.sleep(.25)
        raise RuntimeError("activity_windows_exit_unconfirmed")
    Activity(work).stop_owners(windows,mac)


def retire(work):
    work,m=metadata(work);a=Activity(work);state=a.read()
    if state["phase"]!="owners_joined_retirement_pending":raise RuntimeError("activity_join_before_retirement_required")
    return retire_pair(work,m,a,state,'normal_activity_joined_and_test_roots_retired')


def retire_pair(work,m,a,state,final_phase,before_empty_cleanup=None):
    # No automatic rerun after uncertain destructive cleanup. Explicit
    # finish-files/recovery requires the native tool's separate gates.
    state["phase"]="retirement_outcome_unknown";state["retired"]={};a.save(state)
    raw=command([str(work/"darwin/retire-node-test")],60)
    if json.loads(raw)!={"status":"registered_test_retired"} or Path(m["mac_root"]).exists():raise RuntimeError("activity_mac_retirement_unconfirmed")
    state["retired"]["controller"]=True;a.save(state)
    body="""$ErrorActionPreference='Stop'
$ProgressPreference='SilentlyContinue'
$log=Join-Path $PSScriptRoot 'retire-result.txt'
try { $lines=@(& (Join-Path $PSScriptRoot 'retire-node-test.exe'));$code=$LASTEXITCODE
 $lines | Out-File $log -Encoding utf8
 if($code -ne 0 -or ($lines -join "`n" | ConvertFrom-Json).status -ne 'registered_test_retired'){throw 'Retirement unconfirmed'}
 'EXITCODE=0' | Out-File $log -Append -Encoding utf8
} catch { 'EXITCODE=1' | Out-File $log -Append -Encoding utf8;exit 1 }
"""
    (work/"run-retire.ps1").write_text(body)
    command(["scp","-q",*OPTIONS,str(work/"run-retire.ps1"),WIN+":"+"remoteapp-r75-package-"+m["nonce"]+"/run-retire.ps1"])
    d=quote(m["win_dir"]);task=quote("RemoteApp-R75-Retire-"+m["nonce"])
    ps("$ErrorActionPreference='Stop';$d="+d+";$t="+task+";if(Get-ScheduledTask -TaskName $t -ErrorAction SilentlyContinue){throw 'Retirement task exists'};"
       "$a=New-ScheduledTaskAction -Execute powershell.exe -Argument ('-NoProfile -ExecutionPolicy Bypass -File '+[char]34+(Join-Path $d 'run-retire.ps1')+[char]34);"
       "$p=New-ScheduledTaskPrincipal -UserId jarvis -LogonType Interactive -RunLevel Limited;Register-ScheduledTask -TaskName $t -Action $a -Principal $p | Out-Null;Start-ScheduledTask -TaskName $t")
    for unused in range(120):
        value=ps("$d="+d+";$p=Join-Path $d 'retire-result.txt';if(Test-Path $p){$s=Get-Content $p -Raw;if($s -match 'EXITCODE=1'){'FAILED'}elseif(($s -match 'EXITCODE=0') -and (Get-ScheduledTask -TaskName "+task+").State -ne 'Running' -and (Get-ScheduledTaskInfo -TaskName "+task+").LastTaskResult -eq 0 -and -not(Test-Path "+quote(m["win_root"])+")){'RETIRED'}}")
        if value=="RETIRED":break
        if value=="FAILED":raise RuntimeError("activity_windows_retirement_explicit_recovery_required")
        time.sleep(.25)
    else:raise RuntimeError("activity_windows_retirement_unconfirmed")
    state["retired"]["target"]=True;state["phase"]="test_roots_retired_cleanup_pending";a.save(state)
    # Job removal is precise and only after both joined desktop tasks finish.
    for name in ("RemoteApp-R75-"+m["nonce"],"RemoteApp-R75-Retire-"+m["nonce"]):
        ps("$ErrorActionPreference='Stop';$t="+quote(name)+";if((Get-ScheduledTask -TaskName $t).State -eq 'Running'){throw 'Task still running'};Unregister-ScheduledTask -TaskName $t -Confirm:$false")
    for item in reversed(json.loads((work/"tunnels.json").read_text())):
        command(["ssh",*OPTIONS,"-S",item["socket"],"-O","exit",item["host"]])
    if before_empty_cleanup is not None:before_empty_cleanup()
    Path(m["holder_dir"]).rmdir();Path(m["mac_root"]).parent.rmdir()
    state["phase"]=final_phase;a.save(state)


if __name__=="__main__":
    try:
        if sys.argv[1:]==["prepare"]:print(prepare())
        elif len(sys.argv)==3 and sys.argv[2] in ("start","monitor","exercise","stop","retire","resume-windows"):
            {"start":start,"monitor":monitor,"exercise":exercise,"stop":stop,"retire":retire,"resume-windows":resume_windows}[sys.argv[2]](sys.argv[1])
        elif len(sys.argv)==3 and sys.argv[2] in ('fault-revoke','fault-stop','fault-retire'):
            import fault_activity
            {'fault-revoke':fault_activity.revoke,'fault-stop':fault_activity.stop,'fault-retire':fault_activity.retire}[sys.argv[2]](sys.argv[1])
        else:raise RuntimeError("activity_explicit_step_required")
    except Exception:
        raise SystemExit("activity_step_failed_consult_saved_state") from None
