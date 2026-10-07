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
import threading
import base64
import socket
from native_bootstrap import NativeIdentities

spec=importlib.util.spec_from_file_location("transport",Path(__file__).with_name("cross-network.py"))
t=importlib.util.module_from_spec(spec)
spec.loader.exec_module(t)

class Pipe:
    def __init__(self, command, config=None):
        self.lock=threading.Lock()
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
        with self.lock:
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
    if sys.argv[1:] not in ([],["--streams"],["--tls"],["--maintenance"],["--api"],["--device-flow"],["--identity"],["--managed"],["--native"]):
        raise RuntimeError("invalid_arguments")
    streams=bool(sys.argv[1:])
    native=sys.argv[1:]==["--native"]
    managedmode=native or sys.argv[1:]==["--managed"]
    identitymode=sys.argv[1:] in (["--identity"],["--managed"])
    deviceflow=sys.argv[1:] in (["--device-flow"],["--identity"],["--managed"])
    apimode=sys.argv[1:] in (["--api"],["--device-flow"],["--identity"],["--managed"])
    periodic=sys.argv[1:] in (["--maintenance"],["--api"],["--device-flow"],["--identity"],["--managed"])
    tlsmode=sys.argv[1:] in (["--tls"],["--maintenance"],["--api"],["--device-flow"],["--identity"],["--managed"])
    if native: identitymode=deviceflow=apimode=periodic=tlsmode=True
    os.umask(0o077)
    if tlsmode:
        env=dict(os.environ,COPYFILE_DISABLE="1")
        packed=subprocess.run(["tar","--format=ustar","--no-xattrs","--no-acls","--exclude=bin","--exclude=obj","--exclude=._*",
            "-czf",str(t.ARTIFACTS/"product-tls-source.tar.gz"),"windows/RemoteAgent/Directory.Build.props",
            "windows/RemoteAgent/src/RemoteProtocol","windows/RemoteAgent/tools/TlsBridgeProbe",
            "windows/RemoteAgent/tests/RemoteProtocol.Tests","protocol/testdata"],cwd=t.HERE.parents[1],env=env,capture_output=True,timeout=15)
        t.check(packed.returncode==0,"product_source_archive_without_macos_resource_forks")
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
    active_refresh=None
    tunnels=[]
    target_api=None
    controller_api=None
    identities=None
    def network_request(op,role="target",**values):
        if apimode:return (target_api if role=="target" else controller_api).request(op,**values)
        if op=="enroll":return authority.request(op,Role=role,**values)
        return authority.request(op,**values)
    root="C:\\Users\\jarvis\\remoteapp-enrollment-"+stamp
    work=root+"\\poc\\selfhost"
    try:
        if native: identities=NativeIdentities(t,Pipe)
        switched=subprocess.run(t.SERVER+["python "+script+" prepare"],capture_output=True,timeout=40)
        values=[json.loads(line) for line in switched.stdout.splitlines()]
        backup=next((v["backup"] for v in values if "backup" in v),None)
        if values and values[-1].get("status")=="failed":
            print("FAIL mode_switch_"+values[-1].get("reason","internal_error"),flush=True)
        t.check(switched.returncode==0 and values[-1].get("status")=="db_deny_ready","isolated_db_policy_ready")
        authority=Pipe(t.SERVER+[remote+(" --native" if native else " --device-flow" if deviceflow else "")],
                       {"Target":identities.target["ExpectedPublic"],"Controller":identities.controller["ExpectedPublic"]} if native else None)
        ready=authority.read()
        t.check(ready.get("status")=="ready","real_backend_verifier_prepared_key_matched")
        if native:
            t.check("target_key" not in ready and "controller_key" not in ready and
                    ready["target"]==identities.target["ExpectedPublic"] and ready["controller"]==identities.controller["ExpectedPublic"],
                    "authority_receives_only_pinned_os_public_identities")
            # Local dispatch labels, not app keys. Never sent as AppKey.
            ready["target_key"]="native-target";ready["controller_key"]="native-controller"
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
        if native: win=identities.command(work,"enrollment-helper.exe")
        if apimode:
            # Management tunnels expose only private loopback TLS; not a final
            # product dependency and no new firewall/public nginx route.
            local_socket=socket.socket();local_socket.bind(("127.0.0.1",0));local_port=local_socket.getsockname()[1];local_socket.close()
            forward=subprocess.Popen(t.SERVER[:-1]+["-N","-o","ExitOnForwardFailure=yes","-L",f"127.0.0.1:{local_port}:127.0.0.1:{ready['api_port']}",t.SERVER[-1]],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
            tunnels.append(forward)
            end=time.monotonic()+10
            while True:
                try:
                    connection=socket.create_connection(("127.0.0.1",local_port),timeout=0.5);connection.close();break
                except OSError:
                    if forward.poll() is not None or time.monotonic()>end:raise RuntimeError("private_api_forward_failed")
                    time.sleep(0.1)
            remote_port=int(t.powershell("$l=[System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Loopback,0);$l.Start();$p=$l.LocalEndpoint.Port;$l.Stop();$p"))
            reverse=subprocess.Popen(t.WIN[:-1]+["-N","-o","ExitOnForwardFailure=yes","-R",f"127.0.0.1:{remote_port}:127.0.0.1:{local_port}",t.WIN[-1]],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
            tunnels.append(reverse);time.sleep(1)
            t.check(reverse.poll() is None,"private_windows_api_reverse_forward_ready")
            api_binary=t.ARTIFACTS/"device-api-client-win.exe"
            uploaded=subprocess.run(["scp","-O","-o","BatchMode=yes","-o","IdentitiesOnly=yes","-o","StrictHostKeyChecking=yes","-i",str(Path.home()/".ssh/remoteapp_win_ed25519"),str(api_binary),"jarvis@100.73.4.118:"+work.replace("\\","/")+"/device-api-client.exe"],capture_output=True,timeout=60)
            t.check(uploaded.returncode==0,"device_api_client_uploaded")
            actual=t.powershell(f"(Get-FileHash -Algorithm SHA256 '{work}\\device-api-client.exe').Hash")
            t.check(actual.lower()==hashlib.sha256(api_binary.read_bytes()).hexdigest(),"device_api_client_binary_verified")
            def api_client(command,port,key):
                cfg={"Endpoint":f"https://127.0.0.1:{port}","Certificate":ready["api_certificate"],"Grant":ready["grant"],"Issuer":ready["issuer"],"Target":hashlib.sha256(bytes.fromhex(ready["target"])).hexdigest(),"Controller":hashlib.sha256(bytes.fromhex(ready["controller"])).hexdigest()}
                cfg.update(identities.config(key,key=="native-target") if native else {"AppKey":key})
                p=Pipe(command,cfg);helpers.append(p)
                t.check(p.read()["status"]=="ready","device_owned_http_signer_ready");return p
            target_api=api_client(identities.command(work,"device-api-client.exe") if native else t.WIN+[f'cmd.exe /d /c "cd /d {work} && device-api-client.exe"'],remote_port,ready["target_key"])
            controller_api=api_client([str(t.ARTIFACTS/"device-api-client")],local_port,ready["controller_key"])
            for device in (target_api,controller_api):
                t.check(device.request("device",Device={"action":"register"})["passed"],"device_signed_https_registration_passed")
            if deviceflow:
                def device_call(p,**body):
                    r=p.request("device",Device=body)
                    t.check(r["passed"],"device_flow_"+body["action"]+"_https_passed")
                    return r["device"]
                target_registration=device_call(target_api,action="register")
                controller_registration=device_call(controller_api,action="register")
                t.check(not ready["grant"]["claims"]["id"],"no_preapproved_fixture_grant")
                t.check(not controller_api.request("network_state")["passed"],"no_network_authorization_before_consent")
                def claim_invitation():
                    invitation=device_call(target_api,action="invite")
                    denied=controller_api.request("device",Device={"action":"claim","target_code":"000000000000","code":invitation["code"]})
                    t.check(not denied["passed"],"unknown_device_code_denied")
                    claimed=device_call(controller_api,action="claim",target_code=target_registration["device_code"],code=invitation["code"])
                    t.check(claimed["status"]=="pending" and "grant" not in claimed,"code_exchange_never_auto_approves")
                    incoming=device_call(target_api,action="incoming")
                    t.check(incoming["request_id"]==claimed["request_id"] and incoming["controller_id"]==controller_registration["device_id"] and "code" not in incoming and "grant" not in incoming,"owner_discovers_exact_pending_peer")
                    t.check(device_call(controller_api,action="incoming")["status"]=="idle","controller_cannot_discover_target_queue")
                    reused=controller_api.request("device",Device={"action":"claim","target_code":target_registration["device_code"],"code":invitation["code"]})
                    t.check(not reused["passed"],"verification_code_single_use")
                    return claimed["request_id"]
                request_id=claim_invitation()
                device_call(target_api,action="approve",request_id=request_id,allow=False)
                rejected=device_call(controller_api,action="status",request_id=request_id)
                t.check(rejected["status"]=="denied" and "grant" not in rejected,"explicit_denial_never_grants")
                # First invitation consumed three attempts; next gets fresh
                # target budget, controller stays within its five attempts/minute.
                invitation=device_call(target_api,action="invite")
                claimed=device_call(controller_api,action="claim",target_code=target_registration["device_code"],code=invitation["code"])
                incoming=device_call(target_api,action="incoming")
                t.check(incoming["request_id"]==claimed["request_id"],"second_pending_discovered")
                foreign=controller_api.request("device",Device={"action":"approve","request_id":claimed["request_id"],"allow":True})
                t.check(not foreign["passed"],"controller_cannot_self_approve")
                pending=device_call(controller_api,action="status",request_id=claimed["request_id"])
                t.check(pending["status"]=="pending" and "grant" not in pending,"pending_poll_has_no_grant")
                approved=device_call(target_api,action="approve",request_id=incoming["request_id"],allow=True)
                received=device_call(controller_api,action="status",request_id=claimed["request_id"])
                t.check(approved["grant"]==received["grant"],"both_devices_receive_same_approved_grant")
                grant=received["grant"]
                for device in (target_api,controller_api):
                    t.check(device.request("use_grant",Grant=grant)["passed"],"device_verifies_issuer_subjects_expiry")
                t.check(authority.request("adopt_grant",Grant=grant)["passed"],"private_fixture_adopts_only_device_approved_grant")
                ready["grant"]=grant;ready["grant_id"]=grant["claims"]["id"]
        product=None
        tlsclient=None
        product_ready=None
        if tlsmode:
            archive=t.ARTIFACTS/"product-tls-source.tar.gz"
            dest=root.replace("\\","/")+"/product-source.tar.gz"
            p=subprocess.run(["scp","-O","-o","BatchMode=yes","-o","IdentitiesOnly=yes","-o","StrictHostKeyChecking=yes",
                "-i",str(Path.home()/".ssh/remoteapp_win_ed25519"),str(archive),"jarvis@100.73.4.118:"+dest],capture_output=True,timeout=60)
            t.check(p.returncode==0,"product_tls_sources_uploaded")
            script_text=(f"$ErrorActionPreference='Stop'; tar -xzf '{root}\\product-source.tar.gz' -C '{root}'; "
                f"dotnet run --project '{root}\\windows\\RemoteAgent\\tests\\RemoteProtocol.Tests' -c Release -- --bridge-validation; "
                "if ($LASTEXITCODE -ne 0) {throw 'protocol_tests_failed'}; "
                f"dotnet publish '{root}\\windows\\RemoteAgent\\tools\\TlsBridgeProbe' -c Release -r win-x64 --self-contained true -o '{root}\\product-publish'; "
                "if ($LASTEXITCODE -ne 0) {throw 'publish_failed'}")
            encoded=base64.b64encode(script_text.encode("utf-16le")).decode()
            built=subprocess.run(t.WIN+["powershell.exe -NoProfile -NonInteractive -EncodedCommand "+encoded],capture_output=True,timeout=120)
            if built.returncode:
                for line in (built.stdout+built.stderr).decode(errors="replace").splitlines():
                    if ": error " in line or line.startswith("FAIL "): print(line[:240],flush=True)
            t.check(built.returncode==0,"windows_product_protocol_build_succeeded")
            result=built.stdout.decode(errors="replace")
            t.check("FAIL " not in result and "65/65 tests passed" in result and "SKIP Windows credential persistence" in result,
                    "windows_65_protocol_tests_passed_dpapi_explicitly_not_validated")
            product=Pipe(t.WIN+[f'cmd.exe /d /c "{root}\\product-publish\\TlsBridgeProbe.exe"'])
            helpers.append(product)
            product_ready=product.read()
            t.check(product_ready["status"]=="ready","actual_product_tls_agent_loopback_ready")
        local=[str(t.ARTIFACTS/"enrollment-helper")]
        nodes=t.ARTIFACTS/"nodes"
        nodes.mkdir(mode=0o700,exist_ok=True)
        folder=Path(tempfile.mkdtemp(prefix="enrollment-client-",dir=nodes))
        folders.append(folder)
        win_dir="../../artifacts/connection-poc/nodes/enrollment-target"

        def prepare(command,folder,app_key):
            cfg={"Op":"prepare","StateDir":str(folder),"GrantID":ready["grant_id"]}
            cfg.update(identities.config(app_key,command==win) if native else {"AppKey":app_key})
            p=Pipe(command,cfg)
            value=p.read()
            p.proc.stdin.close()
            p.proc.wait(timeout=8)
            t.check(p.proc.returncode==0 and value.get("status")=="prepared","offline_pre_registration_public_key")
            return value

        target_prepared=prepare(win,win_dir,ready["target_key"])
        target_key=target_prepared["node_key"]
        target=network_request("enroll",Intent=target_prepared["intent"])
        client_prepared=prepare(local,folder,ready["controller_key"])
        client_key=client_prepared["node_key"]
        t.check(not t.admin("nodes","list") or len(t.admin("nodes","list"))==1,"prepare_did_not_register_client_nodes")
        client=network_request("enroll",role="controller",Intent=client_prepared["intent"])
        t.check(target["passed"] and client["passed"],"real_backend_signed_intents_issued_once")

        def join(command,folder,node_key,enrollment,app_key,autonomous=False):
            cfg={"Op":"join","StateDir":str(folder),"AuthKey":enrollment["credential"]["Secret"],
                 "NodeKey":node_key,"Verifier":ready["verifier"],"Token":"" if autonomous else token,"GrantID":ready["grant_id"]}
            cfg.update(identities.config(app_key,command==win) if native else {"AppKey":app_key})
            if autonomous:
                cfg.update(Endpoint=f"https://127.0.0.1:{remote_port if command==win else local_port}",Certificate=ready["api_certificate"],Issuer=ready["issuer"])
            p=Pipe(command,cfg)
            helpers.append(p)
            registered=p.read()
            t.check(registered.get("status")=="registered" and registered["node_key"]==node_key,"actual_registered_key_matches_offline_key")
            t.check(p.read().get("status")=="ready","helper_listeners_ready")
            return p,next(v for v in registered["addresses"] if ":" not in v)

        server,target_ip=join(win,win_dir,target_key,target,ready["target_key"])
        controller,client_ip=join(local,folder,client_key,client,ready["target_key"])  # deliberately wrong app identity
        time.sleep(1)
        t.check(not controller.request("probe",Peer=target_ip)["passed"],"unbound_zero_policy_denies_payload")
        t.check(network_request("bind",Ticket=target["ticket"])["passed"],"windows_live_network_and_app_possession_proof")
        bad=network_request("bind",role="controller",Ticket=client["ticket"])
        if apimode:bad.update(authority.request("proof_result",Ticket=client["ticket"]))
        t.check(not bad["passed"] and bad["proof_stage"]=="peer_rejected" and not bad["registration_expired"],"wrong_app_identity_live_challenge_rejected")
        counts=controller.request("count")
        t.check(counts["challenges"]==1 and counts["signed"]==0,"wrong_app_received_actual_challenge_without_signature")
        t.check(authority.request("rules")["rules"] in (None,[]),"failed_proof_cannot_create_payload_rule")
        t.check(not authority.request("network_state")["passed"],"unverified_pair_cannot_receive_signed_binding")
        controller.stop()
        controller,again_ip=join(local,folder,client_key,client,ready["controller_key"])
        t.check(again_ip==client_ip,"controller_restart_retains_network_identity")
        binding=network_request("bind",role="controller",Ticket=client["ticket"])
        if not binding["passed"]:
            if apimode:binding.update(authority.request("proof_result",Ticket=client["ticket"]))
            print("FAIL mac_bind_"+binding["proof_stage"]+"_expired_"+str(binding["registration_expired"]),flush=True)
        t.check(binding["passed"],"mac_live_network_and_app_possession_proof")
        pending=authority.request("network_state")
        t.check(pending["passed"] and not pending["network_state"]["claims"]["active"],"bound_pair_inactive_before_policy_reconcile")
        t.check(authority.request("reconcile")["passed"],"both_bound_minimum_rule_applied")
        rules=authority.request("rules")["rules"]
        t.check(rules==[{"source":client_ip,"destination":target_ip+":47476"}],"only_approved_pair_port_projected")
        signed=authority.request("network_state")
        claims=signed.get("network_state",{}).get("claims",{})
        t.check(signed["passed"] and claims.get("active") and claims.get("binding")=={
            "target_node":target_key,"controller_node":client_key,"target_ip":target_ip,"controller_ip":client_ip},
            "signed_binding_matches_actual_dynamic_verified_pair")
        time.sleep(1)
        t.check(controller.request("probe",Peer=target_ip)["passed"],"real_cross_network_fixed_payload_allowed")
        reverse=server.request("probe",Peer=client_ip)
        t.check(not reverse["passed"] and reverse["reason"] in ("route_denied","dial_denied") and controller.request("count")["value"]==0,
                "reverse_direction_cannot_use_approved_payload_rule")
        t.check(authority.request("restart")["passed"],"backend_restart_restores_approved_rule")
        restored=authority.request("network_state")
        t.check(restored["passed"] and restored["network_state"]["claims"]["active"] and
                restored["network_state"]["claims"]["binding"]==claims["binding"],"reconciled_restart_restores_same_signed_binding")
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
        if streams:
            if managedmode:
                controller.stop();server.stop()
                server,ip=join(win,win_dir,target_key,target,ready["target_key"],True)
                controller,cip=join(local,folder,client_key,client,ready["controller_key"],True)
                t.check(ip==target_ip and cip==client_ip,"autonomous_restart_keeps_network_identity")
                t.check(not controller.request("probe",Peer=target_ip)["passed"],"autonomous_token_probe_disabled")

            class ManagedRefresh:
                # No thread/HTTP polling/state relay in the parent. Only fixed
                # diagnostics and explicit test teardown of helper-owned watches.
                def __init__(self,state):
                    self.done=threading.Event();self.thread=None;self.last=state
                @property
                def authority_denied(self):
                    return all(helper.request("count")["managed_done"] for helper in (server,controller))
                def close(self,stop=True):
                    self.done.set()
                    if stop:
                        for helper in (server,controller):
                            t.check(helper.request("watch_stop")["passed"],"helper_owned_watch_stop_joined")

            class Refresh:
                def __init__(self,binding):
                    self.done=threading.Event()
                    self.failed=False
                    self.authority_denied=False
                    self.last=None
                    if identitymode:
                        t.check(controller_api.request("watch_start",Binding=binding)["passed"],"device_owned_autonomous_state_watch_started")
                    def work():
                        try:
                            while not self.done.wait(0.5):
                                n=controller_api.request("watch_state") if identitymode else network_request("network_state")
                                if not n["passed"]:
                                    self.authority_denied=True
                                    return
                                self.last=n["network_state"]
                                for helper in (server,controller):
                                    if not helper.request("state",State=self.last)["passed"]:
                                        raise RuntimeError("refresh_rejected")
                        except Exception:
                            self.failed=True
                    self.thread=threading.Thread(target=work,daemon=True)
                    self.thread.start()
                def close(self):
                    self.done.set()
                    self.thread.join(timeout=45)
                    t.check(not self.thread.is_alive() and not self.failed,"dynamic_state_refresh_joined")
                    if identitymode:
                        t.check(controller_api.request("watch_stop")["passed"],"device_state_watch_cancelled_and_joined")

            def wait_for(check,label,timeout=8):
                end=time.monotonic()+timeout
                while time.monotonic()<end:
                    if check():
                        t.check(True,label)
                        return
                    time.sleep(0.2)
                t.check(False,label)

            def authorize():
                nonlocal tlsclient,active_refresh
                if not managedmode:
                    for helper in (server,controller):
                        challenge=helper.request("clock_challenge")["challenge"]
                        proof=network_request("clock_proof",role="target" if helper is server else "controller",Challenge=challenge)
                        t.check(proof["passed"] and helper.request("clock",Issuer=ready["issuer"],Proof=proof["proof"])["passed"],
                                "fresh_signed_authority_clock_calibrated")
                n=network_request("network_state")
                t.check(n["passed"] and n["network_state"]["claims"]["active"],"current_dynamic_state_available")
                for helper,role in ((server,"target"),(controller,"controller")):
                    t.check(helper.request("authorize",Role=role,Issuer=ready["issuer"],Target=ready["target"],
                        Controller=ready["controller"],State=n["network_state"],Bridge=tlsmode)["passed"],"helper_dynamic_gate_enabled")
                if managedmode:
                    for helper in (server,controller):
                        t.check(helper.request("count")["managed_active"],"helper_owned_https_clock_watch_gate_active")
                        t.check(not helper.request("state",State=n["network_state"])["passed"],"parent_state_cannot_authorize_managed_gate")
                    active_refresh=ManagedRefresh(n["network_state"])
                else:
                    active_refresh=Refresh(n["network_state"]["claims"]["binding"])
                if tlsmode:
                    fingerprint=product_ready["Fingerprint"]
                    if identitymode:
                        if not getattr(authorize,"identity_published",False):
                            challenge=secrets.token_hex(32)
                            certificate=product.request("certificate_proof",GrantID=ready["grant_id"],Target=ready["grant"]["claims"]["target"],Challenge=challenge)
                            published=target_api.request("publish_tls",Challenge=challenge,Certificate=certificate["Certificate"],CertificateProof=certificate["CertificateProof"])
                            t.check(published["passed"],"target_publishes_app_signed_tls_private_key_possession")
                            authorize.identity_published=True
                    port=controller.request("bridge",Role="controller")["port"]
                    executable=t.HERE.parents[1]/"macos/RemoteController/.build/release/TLSBridgeProbe"
                    def launch(**changes):
                        pin=fingerprint
                        if identitymode:
                            descriptor=controller_api.request("tls_identity",Binding=n["network_state"]["claims"]["binding"])
                            t.check(descriptor["passed"],"controller_verifies_fresh_issuer_tls_identity")
                            pin=descriptor["tls_identity"]["claims"]["fingerprint"]
                        cfg=dict(Port=port,Key=product_ready["Key"],Fingerprint=pin,Target=n["network_state"]["claims"]["grant"]["claims"]["target"])
                        cfg.update(changes)
                        p=Pipe([str(executable)],cfg);helpers.append(p)
                        t.check(p.read()["status"]=="ready","actual_product_tls_controller_ready")
                        return p
                    if not getattr(authorize,"negatives_done",False):
                        for mode in ("pin","key","preauth"):
                            before=product.request("count")
                            changes={"pin":{"Fingerprint":"00"*32},"key":{"Key":base64.b64encode(bytes(32)).decode()},"preauth":{"Preauth":True}}[mode]
                            negative=launch(**changes)
                            try:
                                wait_for(lambda:negative.request("count")["ended"],"product_tls_negative_completed",6)
                            except RuntimeError:
                                # Only counters/fixed outcome labels; never bootstrap secrets.
                                print("TLS_DIAGNOSTIC",mode,negative.request("count"),server.request("count"),controller.request("count"),product.request("count"),flush=True)
                                raise
                            counts=negative.request("count")
                            t.check(counts["pongs"]==0 and product.request("count")["pongs"]==before["pongs"],"product_tls_negative_zero_protected_payload")
                            if mode=="pin":t.check(counts["pin_rejected"],"actual_product_certificate_pin_mismatch_rejected")
                            if mode=="key":t.check(counts["result"]=="authentication_rejected","actual_product_wrong_hmac_key_rejected")
                            if mode=="preauth":t.check(product.request("count")["preauth_rejected"]>before["preauth_rejected"],"actual_product_preauth_ping_rejected")
                            negative.stop()
                        authorize.negatives_done=True
                        wait_for(lambda:server.request("count")["stream_closed"]>=3 and product.request("count")["sessions_closed"]>=3,
                                 "negative_tls_tunnels_fully_closed")
                    authorize.baseline=server.request("count")["stream_closed"]
                    authorize.product_baseline=product.request("count")["sessions_closed"]
                    tlsclient=launch()
                    wait_for(lambda:tlsclient.request("count")["pongs"]>=3,"actual_product_tls_hmac_framed_pongs")
                else:
                    t.check(controller.request("stream")["passed"],"dynamic_stream_requested")
                    wait_for(lambda:controller.request("count")["stream_ticks"]>=3,"mutual_device_proof_fixed_payload_ticks")

            def ticks():
                return tlsclient.request("count")["pongs"] if tlsmode else controller.request("count")["stream_ticks"]

            def ended():
                if tlsmode:
                    wait_for(lambda:tlsclient.request("count")["ended"] and server.request("count")["stream_closed"]==authorize.baseline+1 and
                             product.request("count")["sessions_closed"]==authorize.product_baseline+1,
                             "actual_product_active_tls_stream_closed",6)
                    a=ticks();p=product.request("count")["pongs"];time.sleep(0.5)
                    final=tlsclient.request("count");product_final=product.request("count")["pongs"]
                    closed=a==final["pongs"] and p==product_final and final["result"]!="timeout"
                    if not closed:print("TLS_CLOSURE_DIAGNOSTIC",{"client_before":a,"client_after":final["pongs"],"product_before":p,"product_after":product_final,"result":final["result"]},flush=True)
                    t.check(closed,"product_tls_closure_no_new_pongs_or_client_timeout")
                    return
                wait_for(lambda:not controller.request("count")["probing"] and server.request("count")["stream_closed"]==1,
                         "same_dynamic_active_stream_closed",6)
                a=controller.request("count")
                time.sleep(0.5)
                b=controller.request("count")
                t.check(a["stream_ticks"]==b["stream_ticks"] and b["stream_closed"]==1 and
                        server.request("count")["stream_opened"]==1,"no_payload_or_connection_reopen_after_close")

            def restart_helpers():
                nonlocal server,controller
                if tlsmode:tlsclient.stop()
                controller.stop(); server.stop()
                server,ip=join(win,win_dir,target_key,target,ready["target_key"],managedmode)
                controller,cip=join(local,folder,client_key,client,ready["controller_key"],managedmode)
                t.check(ip==target_ip and cip==client_ip,"stream_restart_retains_verified_network_identity")

            authorize()
            refresh=active_refresh
            try:
                start=ticks()
                time.sleep(5)
                t.check(ticks()>start+10 and server.request("count")["stream_closed"]==(authorize.baseline if tlsmode else 0),"refresh_keeps_same_connection_beyond_initial_lease")
            finally:
                refresh.close()
            ended()
            t.check(not server.request("state",State=refresh.last)["passed"],"expired_positive_state_replay_rejected")

            if periodic:
                restart_helpers()
                t.check(authority.request("maintenance_start")["passed"],"serial_periodic_maintenance_started")
                wait_for(lambda:authority.request("maintenance_count")["maintenance"]["Ready"],"periodic_startup_projection_ready")
                authorize()
                start=ticks();time.sleep(5)
                t.check(ticks()>start+10 and server.request("count")["stream_closed"]==authorize.baseline,
                        "periodic_maintenance_preserves_refreshed_active_tls")
                if managedmode:active_refresh.close(stop=False)
                else:active_refresh.close()
                failures=authority.request("maintenance_count")["maintenance"]["Failures"]
                t.check(authority.request("policy_unavailable")["passed"],"privileged_policy_failure_injected")
                wait_for(lambda:authority.request("maintenance_count")["maintenance"]["Failures"]>failures,
                         "periodic_policy_failure_observed",10)
                denied=network_request("network_state")
                t.check(denied["passed"] and not denied["network_state"]["claims"]["active"],"policy_failure_fences_active_state")
                for helper in (server,controller):
                    if not managedmode:
                        t.check(helper.request("state",State=denied["network_state"])["passed"],"policy_failure_inactive_delivered")
                ended()
                t.check(t.admin("policy","get")=={"acls":[{"action":"accept","src":[client_ip],"dst":[target_ip+":47476"]}]},
                        "policy_failure_tls_closes_with_old_acl_present")
                t.check(authority.request("policy_available")["passed"],"privileged_policy_service_recovered")
                wait_for(lambda:authority.request("maintenance_count")["maintenance"]["Ready"],"periodic_retry_recovers_without_manual_reconcile",12)
                t.check(authority.request("restart")["passed"],"authority_restart_joins_old_worker_and_starts_new")
                wait_for(lambda:authority.request("maintenance_count")["maintenance"]["Ready"],"cold_periodic_startup_recovers_projection",12)
                restored=authority.request("network_state")
                t.check(restored["passed"] and restored["network_state"]["claims"]["active"],"periodic_restart_verified_binding_restored")

            restart_helpers()
            authorize()
            refresh=active_refresh
            try:
                t.check(authority.request("authority_unavailable")["passed"],"durable_authority_closed_without_acl_withdrawal")
                wait_for(lambda:refresh.authority_denied,"unavailable_authority_cannot_refresh_signed_state",4)
                ended()
            finally:
                refresh.close()
            t.check(t.admin("policy","get")=={"acls":[{"action":"accept","src":[client_ip],"dst":[target_ip+":47476"]}]},
                    "authority_outage_left_network_rule_present")
            t.check(authority.request("restart")["passed"],"authority_recovers_via_reconcile")

            restart_helpers()
            authorize()
            if managedmode:active_refresh.close(stop=False)
            else:active_refresh.close()
            # No parallel refresh here: get a fresh positive, then an authenticated
            # inactive snapshot while actual nodes and payload ACL still exist.
            old=network_request("network_state")["network_state"]
            revoked=target_api.request("device",Device={"action":"revoke","grant_id":ready["grant_id"]}) if apimode else authority.request("revoke_only")
            t.check(revoked["passed"],"grant_revoked_without_node_or_acl_cleanup")
            inactive=network_request("network_state")
            t.check(inactive["passed"] and not inactive["network_state"]["claims"]["active"],"revocation_signed_inactive_with_original_binding")
            if managedmode:
                wait_for(lambda:all(helper.request("count")["managed_done"] and not helper.request("count")["managed_active"] for helper in (server,controller)),
                         "both_helpers_observe_revocation_without_parent_state_relay",2)
            for helper in (server,controller):
                if not managedmode:
                    t.check(helper.request("state",State=inactive["network_state"])["passed"],"inactive_state_delivered")
                replay=helper.request("state",State=old)
                t.check(not replay["passed"] and replay["fresh"],"still_valid_positive_cannot_unlatch_revocation")
            ended()
            t.check(t.admin("policy","get")=={"acls":[{"action":"accept","src":[client_ip],"dst":[target_ip+":47476"]}]},
                    "revoked_stream_closed_while_network_rule_present")
            if periodic:
                t.check(authority.request("maintenance_start")["passed"],"periodic_revoked_resource_cleanup_started")
                def reclaimed():
                    records=authority.request("records")["records"]
                    return len(records)==2 and all(r["Status"]=="cleaned" for r in records)
                wait_for(reclaimed,
                         "revoked_resources_automatically_reclaimed",12)
                wait_for(lambda:authority.request("maintenance_count")["maintenance"]["Ready"],"periodic_zero_rule_projection_ready",12)
                t.check(t.admin("policy","get")=={"acls":[]},"periodic_cleanup_explicit_zero_acl")
                t.check(authority.request("maintenance_stop")["passed"],"maintenance_cancel_and_join_completed")
            else:
                t.check(authority.request("reconcile")["passed"],"revoked_resources_reconciled_after_gate_test")
            if tlsmode:tlsclient.stop();product.stop()
        else:
            t.check(authority.request("revoke")["passed"],"grant_revoke_withdraws_rules_and_nodes")
        t.check(authority.request("restart")["passed"],"restart_does_not_resurrect_revoked_grant")
        t.check(not authority.request("network_state")["passed"],"cleaned_revoked_pair_cannot_receive_new_signed_binding")
        time.sleep(1)
        t.check(not controller.request("probe",Peer=target_ip)["passed"],"revoked_data_path_rejected")
        controller.stop()
        server.stop()
        if apimode:target_api.stop();controller_api.stop()
        authority.stop(helper=False)
        authority=None
        t.check(not t.admin("nodes","list") and not t.admin("users","list") and not t.admin("preauthkeys","list"),"all_owned_control_resources_reclaimed")
    finally:
        if active_refresh is not None:
            active_refresh.done.set()
            if active_refresh.thread is not None:active_refresh.thread.join(timeout=45)
        for p in helpers:
            p.emergency()
        for tunnel in reversed(tunnels):
            if tunnel.poll() is None:
                tunnel.terminate()
                try:tunnel.wait(timeout=5)
                except subprocess.TimeoutExpired:tunnel.kill();tunnel.wait(timeout=5)
        if authority is not None:
            authority.emergency()
        for folder in folders:
            shutil.rmtree(folder)
        # Only this run's exact private node-state directory; binaries remain.
        try:
            t.powershell(f"if (Test-Path '{root}\\artifacts\\connection-poc\\nodes') {{Remove-Item -LiteralPath '{root}\\artifacts\\connection-poc\\nodes' -Recurse -Force}}")
            if backup is not None:
                p=subprocess.run(t.SERVER+["python "+script+" restore "+backup],capture_output=True,timeout=40)
                t.check(p.returncode==0,"original_file_policy_mode_restored")
        finally:
            if identities is not None:
                identities.close()
    t.check(t.admin("policy","get")=={"acls":[{"action":"accept","src":["100.120.0.2"],"dst":["100.120.0.1:47476"]}]},"original_poc_policy_unchanged")
    p=subprocess.run(t.SERVER+["systemctl is-active remoteapp-poc && sha256sum --check /tmp/remoteapp-poc-deploy.hbMUEVFe/existing-config.sha256"],capture_output=True,timeout=15)
    t.check(p.returncode==0,"service_active_business_nginx_hashes_unchanged")

if __name__=="__main__":
    try:
        main()
    except Exception:
        print("FAIL enrollment_network",flush=True)
        sys.exit(1)
