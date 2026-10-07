"""Private Linux daemon acceptance: no Windows, payload grant or public API.

Temporarily switches only the empty isolated PoC policy mode; finally restores
the exact original file config after scoped verifier retirement. Secret-bearing
provider output is captured, never printed. No production private keys uploaded.
"""
import hashlib
import importlib.util
import json
from pathlib import Path
import secrets
import subprocess

spec=importlib.util.spec_from_file_location("transport",Path(__file__).with_name("cross-network.py"))
t=importlib.util.module_from_spec(spec)
spec.loader.exec_module(t)

def upload(source, target):
    cmd=["scp","-o","BatchMode=yes","-o","IdentitiesOnly=yes","-o","StrictHostKeyChecking=yes","-i",str(Path.home()/".ssh/remoteapp_poc_server_ed25519"),str(source),"root@182.92.117.114:"+target]
    p=subprocess.run(cmd,capture_output=True,timeout=60)
    t.check(p.returncode==0,"isolated_acceptance_artifact_uploaded")
    p=subprocess.run(t.SERVER+["sha256sum "+target],capture_output=True,timeout=15)
    t.check(p.returncode==0 and p.stdout.decode().split()[0]==hashlib.sha256(source.read_bytes()).hexdigest(),"uploaded_artifact_hash_verified")

def main():
    t.check(not t.admin("nodes","list") and not t.admin("users","list") and not t.admin("preauthkeys","list"),"isolated_control_plane_empty_before_daemon_test")
    prefix="/tmp/remoteapp-daemon-check-"+secrets.token_hex(8)
    program,daemon,mode=prefix+"-check",prefix+"-daemon",prefix+"-mode.py"
    backup=None
    try:
        upload(t.ARTIFACTS/"daemon-network-check",program)
        upload(t.ARTIFACTS/"authority-daemon-network",daemon)
        upload(Path(__file__).with_name("enrollment-mode.py"),mode)
        p=subprocess.run(t.SERVER+["python "+mode+" prepare"],capture_output=True,timeout=45)
        lines=p.stdout.splitlines()
        if lines:
            backup=json.loads(lines[0]).get("backup")
        t.check(p.returncode==0 and backup is not None,"isolated_database_deny_policy_prepared")
        # Program emits only fixed labels and a non-secret retained directory.
        p=subprocess.Popen(t.SERVER+[program+" "+daemon],stdout=subprocess.PIPE,stderr=subprocess.DEVNULL)
        for raw in p.stdout:
            line=raw.decode().strip()
            if line.startswith(("PASS ","FAIL ","PRESERVED /tmp/remoteapp-daemon-check-")):
                print(line,flush=True)
        p.wait(timeout=15)
        t.check(p.returncode==0,"real_daemon_cross_uid_lifecycle_acceptance_completed")
        t.check(not t.admin("nodes","list") and not t.admin("users","list") and not t.admin("preauthkeys","list"),"scoped_verifier_resources_all_reclaimed")
    finally:
        if backup is not None:
            p=subprocess.run(t.SERVER+["python "+mode+" restore "+backup],capture_output=True,timeout=45)
            if p.returncode != 0:
                print("PRESERVED_POLICY_BACKUP "+backup,flush=True)
            t.check(p.returncode==0,"original_file_policy_mode_restored")
        # Only uploaded executables/script, never journal or retained resources.
        p=subprocess.run(t.SERVER+["rm -- "+program+" "+daemon+" "+mode],capture_output=True,timeout=15)
        t.check(p.returncode==0,"temporary_uploaded_programs_removed")
    t.check(t.admin("policy","get")=={"acls":[{"action":"accept","src":["100.120.0.2"],"dst":["100.120.0.1:47476"]}]},"original_poc_policy_unchanged")
    p=subprocess.run(t.SERVER+["systemctl is-active remoteapp-authority remoteapp-frontend remoteapp-poc && sha256sum --check /opt/remoteapp-prerelease/nginx-baseline.sha256"],capture_output=True,timeout=15)
    t.check(p.returncode==0,"online_services_active_nginx_baseline_unchanged")

if __name__=="__main__":
    try:
        main()
    except Exception:
        print("FAIL isolated_daemon_coordinator",flush=True)
        raise SystemExit(1)
