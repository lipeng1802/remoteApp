"""Private real-driver / Mac-Windows enrollment and ACL acceptance fixture.

All credentials go through captured private pipes; never print raw exceptions.
Management SSH uses installed Tailscale, fixed payload/proofs use isolated tsnet.
"""
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import secrets
import select
import shutil
import subprocess
import sys
import tempfile
import time

spec=importlib.util.spec_from_file_location("transport",Path(__file__).with_name("cross-network.py"))
t=importlib.util.module_from_spec(spec)
spec.loader.exec_module(t)

class Pipe:
    def __init__(self, command, config=None):
        self.proc=subprocess.Popen(command,cwd=t.HERE,stdin=subprocess.PIPE,stdout=subprocess.PIPE,
                                   stderr=subprocess.DEVNULL,bufsize=0)
        if config is not None:
            self.send(config)
    def send(self, value):
        self.proc.stdin.write(json.dumps(value).encode()+b"\n")
    def read(self, timeout=40):
        ready,unused,unused2=select.select([self.proc.stdout],[],[],timeout)
        if not ready:
            raise RuntimeError("private_pipe_timeout")
        line=self.proc.stdout.readline()
        if not line:
            raise RuntimeError("private_pipe_closed")
        value=json.loads(line)
        if value.get("status") in ("failed","cleanup_timeout"):
            raise RuntimeError("helper_failed")
        return value
    def request(self, op, **values):
        self.send(dict(Op=op,**values))
        return self.read()
    def stop(self, helper=True):
        if self.proc.poll() is None:
            self.send({"Op":"stop"})
            self.proc.stdin.close()
        if helper:
            t.check(self.read(12).get("status")=="closed","helper_closed_without_force")
        self.proc.wait(timeout=12)
        t.check(self.proc.returncode==0,"private_process_zero_exit")
    def emergency(self):
        if self.proc.poll() is None:
            try:
                self.send({"Op":"stop"})
                self.proc.stdin.close()
                self.proc.wait(timeout=12)
            except (OSError,subprocess.TimeoutExpired):
                self.proc.kill()
                self.proc.wait(timeout=5)

def upload_server(source,target):
    p=subprocess.run(["scp","-o","BatchMode=yes","-o","IdentitiesOnly=yes",
                      "-o","StrictHostKeyChecking=yes","-i",str(Path.home()/".ssh/remoteapp_poc_server_ed25519"),
                      str(source),"root@182.92.117.114:"+target],capture_output=True,timeout=45)
    t.check(p.returncode==0,"server_fixture_uploaded")

def main():
    if sys.argv[1:]:
        raise RuntimeError("invalid_arguments")
    os.umask(0o077)
    t.check(not t.admin("nodes","list") and not t.admin("users","list"),"isolated_control_plane_empty")
    stamp=time.strftime("%Y%m%d%H%M%S")+secrets.token_hex(3)
    remote="/tmp/remoteapp-enrollment-"+stamp
    script="/tmp/remoteapp-enrollment-mode-"+stamp+".py"
    upload_server(t.ARTIFACTS/"enrollment-fixture",remote)
    upload_server(Path(__file__).with_name("enrollment-mode.py"),script)
    subprocess.run(t.SERVER+["chmod 700 "+remote],capture_output=True,check=True,timeout=15)
    backup=None
    helpers=[]
    authority=None
    folders=[]
    root="C:\\Users\\jarvis\\remoteapp-enrollment-"+stamp
    work=root+"\\poc\\selfhost"
    try:
        switched=subprocess.run(t.SERVER+["python "+script+" prepare"],capture_output=True,timeout=40)
        values=[json.loads(line) for line in switched.stdout.splitlines()]
        backup=next((v["backup"] for v in values if "backup" in v),None)
        if values and values[-1].get("status")=="failed":
            print("FAIL mode_switch_"+values[-1].get("reason","internal_error"),flush=True)
        t.check(switched.returncode==0 and values[-1].get("status")=="db_deny_ready","isolated_db_policy_ready")
        authority=Pipe(t.SERVER+[remote])
        ready=authority.read()
        t.check(ready.get("status")=="ready","real_backend_verifier_prepared_key_matched")
        # Private test keys only; production enrollment must receive signatures,
        # not device private keys. This privileged fixture emulates both devices.
        token=secrets.token_hex(32)
        t.powershell("$ErrorActionPreference='Stop'; "
                     f"if (Test-Path '{root}') {{throw 'exists'}}; New-Item -ItemType Directory -Path '{work}' -Force | Out-Null; "
                     "$sid=[System.Security.Principal.WindowsIdentity]::GetCurrent().User.Value; "
                     f"icacls '{root}' /inheritance:r /grant:r \"*${{sid}}:(OI)(CI)F\" '*S-1-5-18:(OI)(CI)F' | Out-Null; "
                     "if ($LASTEXITCODE -ne 0) {throw 'acl_failed'}")
        binary=t.ARTIFACTS/"enrollment-helper-win.exe"
        p=subprocess.run(["scp","-O","-o","BatchMode=yes","-o","IdentitiesOnly=yes",
                          "-o","StrictHostKeyChecking=yes","-i",str(Path.home()/".ssh/remoteapp_win_ed25519"),
                          str(binary),"jarvis@100.73.4.118:"+work.replace("\\","/")+"/enrollment-helper.exe"],capture_output=True,timeout=60)
        t.check(p.returncode==0,"windows_enrollment_helper_uploaded")
        actual=t.powershell(f"(Get-FileHash -Algorithm SHA256 '{work}\\enrollment-helper.exe').Hash")
        t.check(actual.lower()==hashlib.sha256(binary.read_bytes()).hexdigest(),"windows_enrollment_binary_verified")
        win=t.WIN+[f'cmd.exe /d /c "cd /d {work} && enrollment-helper.exe"']
        local=[str(t.ARTIFACTS/"enrollment-helper")]
        nodes=t.ARTIFACTS/"nodes"
        nodes.mkdir(mode=0o700,exist_ok=True)
        folder=Path(tempfile.mkdtemp(prefix="enrollment-client-",dir=nodes))
        folders.append(folder)
        win_dir="../../artifacts/connection-poc/nodes/enrollment-target"

        def prepare(command,folder,app_key):
            p=Pipe(command,{"Op":"prepare","StateDir":str(folder),"AppKey":app_key,"GrantID":ready["grant_id"]})
            value=p.read()
            p.proc.stdin.close()
            p.proc.wait(timeout=8)
            t.check(p.proc.returncode==0 and value.get("status")=="prepared","offline_pre_registration_public_key")
            return value

        target_prepared=prepare(win,win_dir,ready["target_key"])
        target_key=target_prepared["node_key"]
        target=authority.request("enroll",Role="target",Intent=target_prepared["intent"])
        client_prepared=prepare(local,folder,ready["controller_key"])
        client_key=client_prepared["node_key"]
        t.check(not t.admin("nodes","list") or len(t.admin("nodes","list"))==1,"prepare_did_not_register_client_nodes")
        client=authority.request("enroll",Role="controller",Intent=client_prepared["intent"])
        t.check(target["passed"] and client["passed"],"real_backend_signed_intents_issued_once")

        def join(command,folder,node_key,enrollment,app_key):
            p=Pipe(command,{"Op":"join","StateDir":str(folder),"AuthKey":enrollment["credential"]["Secret"],
                            "NodeKey":node_key,"AppKey":app_key,"Verifier":ready["verifier"],"Token":token})
            helpers.append(p)
            registered=p.read()
            t.check(registered.get("status")=="registered" and registered["node_key"]==node_key,"actual_registered_key_matches_offline_key")
            t.check(p.read().get("status")=="ready","helper_listeners_ready")
            return p,next(v for v in registered["addresses"] if ":" not in v)

        server,target_ip=join(win,win_dir,target_key,target,ready["target_key"])
        controller,client_ip=join(local,folder,client_key,client,ready["target_key"])  # deliberately wrong app identity
        time.sleep(1)
        t.check(not controller.request("probe",Peer=target_ip)["passed"],"unbound_zero_policy_denies_payload")
        t.check(authority.request("bind",Ticket=target["ticket"])["passed"],"windows_live_network_and_app_possession_proof")
        bad=authority.request("bind",Ticket=client["ticket"])
        t.check(not bad["passed"] and bad["proof_stage"]=="peer_rejected" and not bad["registration_expired"],"wrong_app_identity_live_challenge_rejected")
        counts=controller.request("count")
        t.check(counts["challenges"]==1 and counts["signed"]==0,"wrong_app_received_actual_challenge_without_signature")
        t.check(authority.request("rules")["rules"] in (None,[]),"failed_proof_cannot_create_payload_rule")
        controller.stop()
        controller,again_ip=join(local,folder,client_key,client,ready["controller_key"])
        t.check(again_ip==client_ip,"controller_restart_retains_network_identity")
        binding=authority.request("bind",Ticket=client["ticket"])
        if not binding["passed"]:
            print("FAIL mac_bind_"+binding["proof_stage"]+"_expired_"+str(binding["registration_expired"]),flush=True)
        t.check(binding["passed"],"mac_live_network_and_app_possession_proof")
        t.check(authority.request("reconcile")["passed"],"both_bound_minimum_rule_applied")
        rules=authority.request("rules")["rules"]
        t.check(rules==[{"source":client_ip,"destination":target_ip+":47476"}],"only_approved_pair_port_projected")
        time.sleep(1)
        t.check(controller.request("probe",Peer=target_ip)["passed"],"real_cross_network_fixed_payload_allowed")
        reverse=server.request("probe",Peer=client_ip)
        t.check(not reverse["passed"] and reverse["reason"] in ("route_denied","dial_denied") and controller.request("count")["value"]==0,
                "reverse_direction_cannot_use_approved_payload_rule")
        t.check(authority.request("restart")["passed"],"backend_restart_restores_approved_rule")
        time.sleep(1)
        t.check(controller.request("probe",Peer=target_ip)["passed"],"restart_data_path_still_works")
        before=server.request("count")["value"]
        t.check(authority.request("zero")["passed"],"explicit_empty_acl_applied")
        t.check(t.admin("policy","get")=={"acls":[]},"empty_acl_readback")
        time.sleep(1)
        for unused in range(3):
            denied=controller.request("probe",Peer=target_ip)
            t.check(not denied["passed"] and denied["reason"] in ("route_denied","dial_denied"),"real_overlay_denied_without_system_fallback")
        t.check(server.request("count")["value"]==before,"zero_policy_delivers_no_payload_to_live_target")
        t.check(authority.request("reconcile")["passed"],"approved_rule_restored_after_zero_test")
        time.sleep(1)
        t.check(controller.request("probe",Peer=target_ip)["passed"],"positive_control_after_zero_test")
        t.check(authority.request("revoke")["passed"],"grant_revoke_withdraws_rules_and_nodes")
        t.check(authority.request("restart")["passed"],"restart_does_not_resurrect_revoked_grant")
        time.sleep(1)
        t.check(not controller.request("probe",Peer=target_ip)["passed"],"revoked_data_path_rejected")
        controller.stop()
        server.stop()
        authority.stop(helper=False)
        authority=None
        t.check(not t.admin("nodes","list") and not t.admin("users","list"),"all_owned_control_resources_reclaimed")
    finally:
        for p in helpers:
            p.emergency()
        if authority is not None:
            authority.emergency()
        for folder in folders:
            shutil.rmtree(folder)
        # Only this run's exact private node-state directory; binaries remain.
        t.powershell(f"if (Test-Path '{root}\\artifacts\\connection-poc\\nodes') {{Remove-Item -LiteralPath '{root}\\artifacts\\connection-poc\\nodes' -Recurse -Force}}")
        if backup is not None:
            p=subprocess.run(t.SERVER+["python "+script+" restore "+backup],capture_output=True,timeout=40)
            t.check(p.returncode==0,"original_file_policy_mode_restored")
    t.check(t.admin("policy","get")=={"acls":[{"action":"accept","src":["100.120.0.2"],"dst":["100.120.0.1:47476"]}]},"original_poc_policy_unchanged")
    p=subprocess.run(t.SERVER+["systemctl is-active remoteapp-poc && sha256sum --check /tmp/remoteapp-poc-deploy.hbMUEVFe/existing-config.sha256"],capture_output=True,timeout=15)
    t.check(p.returncode==0,"service_active_business_nginx_hashes_unchanged")

if __name__=="__main__":
    try:
        main()
    except Exception:
        print("FAIL enrollment_network",flush=True)
        sys.exit(1)
