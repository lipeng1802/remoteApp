"""Trusted management-only runner for fresh r67 TEST bundles, not product UX.

Input is an operator-created local directory with fresh darwin/windows bundles
and public management.json. No legacy empty-control cleanup or Store restore.
Only public phase evidence is persisted. OS TEST identities remain until an
explicit retirement step after authoritative revocation and process joins.
"""
import base64
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import time
import zipfile

from joined_pipe import JoinedPipe
from supervised_requests import LocalSupervisor, WindowsRelay
from supervised_consent import consent_round

WIN = "jarvis@100.73.4.118"
SERVER = "root@182.92.117.114"


def run(args, data=None):
    try:
        p = subprocess.run(args, input=data, stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=90)
    except (OSError, subprocess.TimeoutExpired):
        raise RuntimeError("management_command_outcome_unknown") from None
    if p.returncode:
        raise RuntimeError("management_command_failed")
    return p.stdout


def ps(script):
    encoded = base64.b64encode(script.encode("utf-16le")).decode()
    if len(encoded)>6500:
        # Windows OpenSSH may invoke cmd.exe with its command-line limit.
        # Upload large reviewed scripts as files instead of nesting base64.
        raise RuntimeError("management_script_requires_file_upload")
    return run(["ssh", "-o", "BatchMode=yes", "-o", "ConnectTimeout=12", WIN,
                "powershell.exe -NoProfile -EncodedCommand " + encoded]).decode("utf-8", errors="strict").strip()


def quote(value):
    return "'" + value.replace("'", "''") + "'"


def main(work, resume_setup=False):
    work = Path(work).resolve(strict=True)
    meta = json.loads((work / "management.json").read_text())
    nonce = meta["nonce"]
    if len(nonce) != 12 or any(c not in "0123456789abcdef" for c in nonce):
        raise RuntimeError("invalid_management_nonce")
    if meta["win_dir"] != "C:\\Users\\jarvis\\remoteapp-r67-package-"+nonce:
        raise RuntimeError("invalid_management_directory")
    repo = Path(__file__).resolve().parents[3]
    win = work / "windows"
    shutil.copy2(repo / "packaging/experimental/validate-supervised-install.ps1", win / "validate-supervised-install.ps1")
    if not (win/"supervised-control-relay.exe").is_file():
        raise RuntimeError("fresh_matching_relay_required")
    profile = json.loads(base64.b64decode(json.loads((work/"darwin/fixture.json").read_text())["release"]))
    expiry = profile["bundled_profile"]["claims"]["expires_at"]
    if expiry-time.time() < (180 if resume_setup else 600):
        raise RuntimeError("fresh_build_window_required")
    if (work/"execution.json").exists():
        previous=json.loads((work/"execution.json").read_text())
        if not resume_setup or previous.get("consent") is not False or Path(meta["mac_root"]).exists():
            raise RuntimeError("existing_execution_requires_explicit_recovery")
        history=work/"execution-first-attempt.json"
        if history.exists():raise RuntimeError("recovery_already_attempted")
        shutil.copy2(work/"execution.json",history)
    evidence = {"phase":"preparing", "consent":False, "mac_joined":False, "windows_joined":False}
    def save():
        (work/"execution.json").write_text(json.dumps(evidence,sort_keys=True))
    save()
    archive=work/"windows.zip"
    with zipfile.ZipFile(archive,"w",zipfile.ZIP_DEFLATED) as z:
        for file in win.rglob("*"):
            if file.is_file(): z.write(file,file.relative_to(win))
    remote_zip="remoteapp-r67-"+nonce+".zip"
    if not resume_setup:run(["scp","-q","-o","BatchMode=yes",str(archive),WIN+":"+remote_zip])
    task="RemoteApp-R67-"+nonce
    directory=quote(meta["win_dir"])
    # Parent stdout contains phase markers only; supervisor READY stays private
    # until the holder writes its explicitly public discovery metadata.
    wrapper="""$ErrorActionPreference='Stop'
$log=Join-Path $PSScriptRoot 'owner-result.txt'
try {
 & (Join-Path $PSScriptRoot 'validate-supervised-install.ps1') -StayReady RESUME *>&1 | Out-File $log -Append -Encoding utf8
 'EXITCODE=0' | Out-File $log -Append -Encoding utf8
} catch {'OWNER_FAILED_STATE_PRESERVED' | Out-File $log -Append -Encoding utf8; 'EXITCODE=1' | Out-File $log -Append -Encoding utf8; exit 1}
"""
    if not resume_setup:
        digest=hashlib.sha256(archive.read_bytes()).hexdigest()
        ps("$ErrorActionPreference='Stop'; $d="+directory+"; if(Test-Path $d){throw 'fresh directory required'};"
           "$z=Join-Path $env:USERPROFILE "+quote(remote_zip)+";if((Get-FileHash $z -Algorithm SHA256).Hash.ToLowerInvariant() -ne "+quote(digest)+"){throw 'management package mismatch'}; Expand-Archive $z $d")
    def start(resume=False):
        if resume:
            ps("$ErrorActionPreference='Stop';if((Get-ScheduledTask -TaskName "+quote(task)+").State -eq 'Running'){throw 'previous task not exited'};"
               "$d="+directory+";if(Test-Path (Join-Path $d 'owner-ready.json')){throw 'owner metadata exists'};"
               "$s=Join-Path $d 'stop-owner.request';if(Test-Path $s){if((Get-Item $s).Length -ne 0){throw 'unknown stop request'};Remove-Item -LiteralPath $s};"
               "Move-Item -LiteralPath (Join-Path $d 'owner-result.txt') -Destination (Join-Path $d 'initial-setup-failure.txt')")
        body=wrapper.replace("RESUME","-ResumeFiles" if resume else "")
        encoded=base64.b64encode(body.encode()).decode()
        ps("$ErrorActionPreference='Stop';$d="+directory+";[IO.File]::WriteAllBytes((Join-Path $d 'run-owner.ps1'),[Convert]::FromBase64String("+quote(encoded)+"));"
           "$a=New-ScheduledTaskAction -Execute 'powershell.exe' -Argument ('-NoProfile -ExecutionPolicy Bypass -File '+[char]34+(Join-Path $d 'run-owner.ps1')+[char]34);"
           "$p=New-ScheduledTaskPrincipal -UserId 'jarvis' -LogonType Interactive -RunLevel Limited;"
           "Register-ScheduledTask -TaskName "+quote(task)+" -Action $a -Principal $p -Force | Out-Null;Start-ScheduledTask -TaskName "+quote(task))
    sockets=[]
    owner=None
    started=False
    try:
        for label,host,flag in (("server",SERVER,["-L","127.0.0.1:18444:127.0.0.1:8444"]),
                                ("windows",WIN,["-R","127.0.0.1:18444:127.0.0.1:18444"])):
            sock="/tmp/remoteapp-r67-"+nonce+"-"+label+".sock"
            run(["ssh","-M","-S",sock,"-fNT","-o","BatchMode=yes","-o","ConnectTimeout=12","-o","ExitOnForwardFailure=yes",*flag,host])
            sockets.append((sock,host))
        start(resume_setup);started=True
        ready=None
        resumed=False
        for unused in range(80):
            value=ps("$d="+directory+";$p=Join-Path $d 'owner-ready.json';if(Test-Path $p){[IO.File]::ReadAllText($p)}elseif(Test-Path (Join-Path $d 'owner-result.txt')){if((Get-Content (Join-Path $d 'owner-result.txt') -Raw) -match 'EXITCODE=1'){ 'FAILED' }}")
            if value=="FAILED":
                if resumed:raise RuntimeError("desktop_setup_failed_after_explicit_recovery")
                # Native setup itself requires absent trust before resume-files;
                # an initialized or unknown installation cannot be reinitialized.
                start(True);resumed=True
            elif value:
                ready=json.loads(value);break
            time.sleep(.5)
        if ready is None:raise RuntimeError("desktop_ready_unconfirmed")
        result=run([str(work/"darwin/test-setup")])
        if json.loads(result).get("status")!="test_installed":raise RuntimeError("mac_install_unconfirmed")
        owner=JoinedPipe([str(work/"darwin/supervisor")],{"Op":"open"})
        if owner.read().get("status")!="ready":raise RuntimeError("mac_owner_unconfirmed")
        target=WindowsRelay(WIN,meta["win_dir"]+"\\supervised-control-relay.exe",ready["control_pipe"])
        evidence["phase"]="consent"
        save()
        evidence["public_checks"]=consent_round(target,LocalSupervisor(owner),profile_expires_at=expiry)
        evidence["consent"]=True
        evidence["phase"]="revoked"
        save()
    finally:
        if owner is not None:
            try:owner.stop();evidence["mac_joined"]=True
            except Exception:evidence["mac_joined"]=False
        if started:
            try:
                ps("New-Item -ItemType File -Path (Join-Path "+directory+" 'stop-owner.request') -ErrorAction Stop | Out-Null")
                for unused in range(80):
                    value=ps("$p=Join-Path "+directory+" 'owner-result.txt';if(Test-Path $p){$s=Get-Content $p -Raw;if($s -match 'EXITCODE=0'){'JOINED'}elseif($s -match 'EXITCODE=1'){'FAILED'}}")
                    if value=="JOINED":evidence["windows_joined"]=True;break
                    if value=="FAILED":break
                    time.sleep(.5)
            except Exception:evidence["windows_joined"]=False
        for sock,host in reversed(sockets):
            try:run(["ssh","-S",sock,"-O","exit",host])
            except Exception:evidence["management_tunnel_close_unconfirmed"]=True
        save()
    if not all(evidence[k] for k in ("consent","mac_joined","windows_joined")):
        raise RuntimeError("consent_or_join_unconfirmed_preserve_test_state")
    evidence["phase"]="consent_revoked_and_products_joined"
    save()
    print("PASS actual supervised consent and both products joined; TEST identities retained for explicit retirement")


if __name__=="__main__":
    if len(sys.argv) not in (2,3) or (len(sys.argv)==3 and sys.argv[2]!="--resume-setup"):raise SystemExit("trusted management directory and optional explicit --resume-setup required")
    main(sys.argv[1],len(sys.argv)==3)
