"""Two OS signers -> private HTTPS frontend -> kernel broker -> sole daemon.

Consent phase only: no client enrollment/nodeproof/product TLS claims. SSH is
management transport; API CA is private test bootstrap, not installed trust.
"""
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import secrets
import socket
import subprocess
import time
import sys
import shutil

from native_bootstrap import NativeIdentities

def load(name, filename):
    spec = importlib.util.spec_from_file_location(name, Path(__file__).with_name(filename))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module

enrollment = load("split_transport", "enrollment-network.py")
t, Pipe = enrollment.t, enrollment.Pipe
uploads = load("split_uploads", "daemon-network.py")

def checked_ready(value, target, controller, nodes=False, fault=False, compiled=False):
    import base64
    import re
    expected = {"status", "split_process", "target", "controller", "issuer", "api_certificate", "api_port"}
    if compiled:
        if not nodes or value.get("compiled_phase") is not True:
            raise RuntimeError("invalid_compiled_phase")
        expected.add("compiled_phase")
    if nodes:
        expected.update(("node_phase","verifier"))
        if value.get("node_phase") is not True or not re.fullmatch(r"nodekey:[0-9a-f]{64}",value.get("verifier","")):
            raise RuntimeError("invalid_split_node_bootstrap")
    if fault:
        expected.add("fault_phase")
        if not nodes or value.get("fault_phase") is not True: raise RuntimeError("invalid_fault_bootstrap")
    if (set(value) != expected or value["status"] != "ready" or value["split_process"] is not True
            or value["target"] != target or value["controller"] != controller
            or not re.fullmatch(r"[0-9a-f]{64}", value["issuer"])
            or type(value["api_port"]) is not int or not 0 < value["api_port"] < 65536
            or not 0 < len(base64.b64decode(value["api_certificate"], validate=True)) < 8192):
        raise RuntimeError("invalid_split_bootstrap")
    return value

def consent(target, controller, node_phase=None):
    def call(client, **body):
        response = client.request("device", Device=body)
        if response.get("passed") is not True:
            stage = response.get("failure_stage")
            if stage not in ("signer", "transport", "response", "protocol"):
                stage = "unknown"
            print("DIAGNOSTIC device_request_" + stage, flush=True)
        t.check(response.get("passed") is True, "split_os_signed_" + body["action"])
        return response["device"]
    owner = call(target, action="register")
    peer = call(controller, action="register")
    t.check(owner["device_id"] != peer["device_id"], "split_distinct_registered_os_subjects")
    for client in (target, controller):
        t.check(not client.request("network_state")["passed"], "split_no_network_before_consent")
    invitation = call(target, action="invite")
    pending = call(controller, action="claim", target_code=owner["device_code"], code=invitation["code"])
    t.check(pending["status"] == "pending" and "grant" not in pending, "split_code_not_auto_approval")
    incoming = call(target, action="incoming")
    t.check(incoming["request_id"] == pending["request_id"] and incoming["controller_id"] == peer["device_id"], "split_owner_sees_exact_peer")
    foreign = controller.request("device", Device={"action": "approve", "request_id": pending["request_id"], "allow": True})
    t.check(not foreign["passed"], "split_controller_cannot_self_approve")
    call(target, action="approve", request_id=pending["request_id"], allow=False)
    rejected = call(controller, action="status", request_id=pending["request_id"])
    t.check(rejected["status"] == "denied" and "grant" not in rejected, "split_owner_denial_has_no_grant")
    invitation = call(target, action="invite")
    pending = call(controller, action="claim", target_code=owner["device_code"], code=invitation["code"])
    t.check(pending["status"] == "pending" and "grant" not in pending, "split_second_claim_still_pending")
    approved = call(target, action="approve", request_id=pending["request_id"], allow=True)
    received = call(controller, action="status", request_id=pending["request_id"])
    t.check(approved.get("grant") is not None and approved["grant"] == received.get("grant"), "split_same_owner_approved_grant")
    grant = received["grant"]
    for client in (target, controller):
        t.check(client.request("use_grant", Grant=grant)["passed"], "split_client_verifies_issuer_subjects_expiry")
        t.check(not client.request("network_state")["passed"], "split_grant_without_client_bindings_no_network")
    phase_failed = False
    try:
        if node_phase is not None: node_phase(grant)
    except Exception as error:
        phase_failed = True
        import traceback
        frame = traceback.extract_tb(error.__traceback__)[-1]
        print("NODE_PHASE_DIAGNOSTIC " + type(error).__name__ + " " +
              Path(frame.filename).name + ":" + str(frame.lineno), flush=True)
        raise
    finally:
        try:
            t.check(call(target, action="revoke", grant_id=grant["claims"]["id"])["status"] == "revoked", "split_os_owner_signed_revocation")
        except Exception:
            print("FAIL split_owner_revocation_unconfirmed", flush=True)
            if not phase_failed:
                raise
    final = call(controller, action="status", request_id=pending["request_id"])
    t.check(final["status"] == "revoked" and "grant" not in final, "split_revocation_not_offline_signature_check")

class Coordinator(Pipe):
    cleanup_seen = False

    def observe_cleanup(self, line):
        if line == "STOPPED isolated_coordinator_resources_joined":
            self.cleanup_seen = True
            print(line, flush=True)
            return True
        return False

    def read(self, timeout=50):
        end = time.monotonic() + timeout
        import select
        while time.monotonic() < end:
            if not select.select([self.proc.stdout], [], [], max(0, end-time.monotonic()))[0]:
                break
            raw = self.proc.stdout.readline()
            if not raw:
                break
            line = raw.decode().strip()
            if self.observe_cleanup(line):
                continue
            if line.startswith("PASS "):
                print(line, flush=True)
                continue
            if line.startswith("FAIL "):
                print(line, flush=True)
                raise RuntimeError("split_server_declined")
            return json.loads(line)
        raise RuntimeError("split_readiness_failed")

    def join(self):
        # EOF causes failure teardown unless the exact stop command was sent.
        if not self.proc.stdin.closed:
            self.proc.stdin.close()
        end = time.monotonic()+100
        import select
        while time.monotonic() < end:
            if not select.select([self.proc.stdout], [], [], max(0,end-time.monotonic()))[0]:
                raise RuntimeError("split_join_unconfirmed")
            raw = self.proc.stdout.readline()
            if not raw:
                break
            line = raw.decode().strip()
            if self.observe_cleanup(line):
                continue
            if line.startswith(("PASS ", "FAIL ", "PRESERVED /tmp/remoteapp-daemon-check-")):
                print(line, flush=True)
        self.proc.wait(timeout=5)
        if not self.cleanup_seen:
            raise RuntimeError("split_remote_cleanup_unconfirmed")
        return self.proc.returncode

def mode_flags(args):
    if args not in ([],["--nodes"],["--tls"],["--tls-fault"],["--compiled"],["--compiled-revoke"],["--compiled-fault"]): raise RuntimeError("invalid_arguments")
    live_revoke=args==["--compiled-revoke"]
    compiled=live_revoke or args in (["--compiled"],["--compiled-fault"])
    fault=args in (["--tls-fault"],["--compiled-fault"])
    tls=args in (["--tls-fault"],["--tls"])
    nodes=compiled or tls or args==["--nodes"]
    return compiled,live_revoke,fault,tls,nodes


def main():
    compiled,live_revoke,fault,tls,nodes=mode_flags(sys.argv[1:])
    pipe_type=Pipe
    if compiled:
        from joined_pipe import JoinedPipe
        def pipe_type(command,config=None): return JoinedPipe(command,config,cwd=t.HERE)
    os.umask(0o077)
    desktop_stage = None
    if os.environ.get("REMOTEAPP_WINDOWS_DESKTOP_STAGE"):
        if not compiled:
            raise RuntimeError("desktop_stage_requires_compiled_mode")
        from desktop_stage import DesktopStage
        desktop_stage = DesktopStage(t,os.environ["REMOTEAPP_WINDOWS_DESKTOP_STAGE"])
    identities = NativeIdentities(t, pipe_type,compiled_desktop=compiled)  # Actual desktop probe before mutations.
    coordinator = None
    clients, tunnels = [], []
    node_helpers, folders, node_result = [], [], []
    active_tls=[]
    product_command=None
    compiled_pair=None
    compiled_pair_joined=not compiled
    node_cleanup_confirmed = not nodes
    backup = None
    joined = False
    work_created = False
    prefix = "/tmp/remoteapp-split-consent-"+secrets.token_hex(8)
    program, daemon, frontend, mode = [prefix+x for x in ("-check", "-daemon", "-frontend", "-mode.py")]
    work = desktop_stage.root+"\\run" if desktop_stage else "C:\\Users\\jarvis\\remoteapp-split-consent-"+secrets.token_hex(8)
    def create_work():
        nonlocal work_created
        t.powershell("$ErrorActionPreference='Stop'; "+f"if (Test-Path -LiteralPath '{work}') {{throw 'exists'}}; New-Item -ItemType Directory -Path '{work}' | Out-Null")
        work_created = True
        t.powershell("$ErrorActionPreference='Stop'; $sid=[System.Security.Principal.WindowsIdentity]::GetCurrent().User.Value; "+f"icacls '{work}' /inheritance:r /grant:r \"*${{sid}}:(OI)(CI)F\" '*S-1-5-18:(OI)(CI)F' | Out-Null; if ($LASTEXITCODE -ne 0) {{throw 'acl_failed'}}")
    try:
        if compiled:
            import compiled_products
            compiled_products.build_helpers(t)
        t.check(not t.admin("nodes", "list") and not t.admin("users", "list") and not t.admin("preauthkeys", "list"), "split_empty_isolated_control_plane")
        if tls:
            import split_tls
            create_work()
            product_command=split_tls.prepare(t,work)
        for source, destination in [(t.ARTIFACTS/"daemon-network-check",program), (t.ARTIFACTS/"authority-daemon-network",daemon),
                (t.ARTIFACTS/"device-frontend-private",frontend), (Path(__file__).with_name("enrollment-mode.py"),mode)]:
            uploads.upload(source, destination)
        # Public executables must be executable by the deliberately non-root
        # probe/frontend. Never chmod material, config, journal or private keys.
        permissions=subprocess.run(t.SERVER+["chmod 755 -- "+" ".join((program,daemon,frontend))],capture_output=True,timeout=15)
        t.check(permissions.returncode==0,"split_public_binaries_nonroot_executable")
        prepared = subprocess.run(t.SERVER+["python "+mode+" prepare"], capture_output=True, timeout=45)
        if prepared.stdout.splitlines():
            backup = json.loads(prepared.stdout.splitlines()[0]).get("backup")
        t.check(prepared.returncode == 0 and backup is not None, "split_database_deny_prepared")
        coordinator = Coordinator(t.SERVER+[program+" "+daemon+" --frontend "+frontend+(" --os-compiled-fault " if compiled and fault else " --os-compiled " if compiled else " --os-tls-fault " if fault else " --os-nodes " if nodes else " --os-consent ")+identities.target["ExpectedPublic"]+" "+identities.controller["ExpectedPublic"]])
        ready = checked_ready(coordinator.read(), identities.target["ExpectedPublic"], identities.controller["ExpectedPublic"],nodes,fault,compiled)
        # Existing compiled server lifetime is 15 minutes. Include build/upload
        # in the ten-minute readiness budget; leave five minutes for acceptance.
        readiness_deadline = time.monotonic() + 600
        t.check(True, "split_independent_daemon_nonroot_frontend_ready_public_only")
        local_socket = socket.socket(); local_socket.bind(("127.0.0.1",0)); local_port = local_socket.getsockname()[1]; local_socket.close()
        forward = subprocess.Popen(t.SERVER[:-1]+["-N","-o","ExitOnForwardFailure=yes","-L",f"127.0.0.1:{local_port}:127.0.0.1:{ready['api_port']}",t.SERVER[-1]],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
        tunnels.append(forward)
        end = time.monotonic()+10
        while True:
            try:
                connection=socket.create_connection(("127.0.0.1",local_port),timeout=.5); connection.close(); break
            except OSError:
                if forward.poll() is not None or time.monotonic()>end: raise RuntimeError("split_forward_failed")
                time.sleep(.1)
        remote_port = int(t.powershell("$l=[System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Loopback,0);$l.Start();$p=$l.LocalEndpoint.Port;$l.Stop();$p"))
        reverse = subprocess.Popen(t.WIN[:-1]+["-N","-o","ExitOnForwardFailure=yes","-R",f"127.0.0.1:{remote_port}:127.0.0.1:{local_port}",t.WIN[-1]],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
        tunnels.append(reverse); time.sleep(1)
        t.check(reverse.poll() is None,"split_private_windows_api_tunnel")
        if not work_created: create_work()
        if nodes:
            import split_nodes
            split_nodes.upload_helper(t,work)
        if compiled:
            import compiled_products
            compiled_pair=compiled_products.prepare(t,identities,ready,(remote_port,local_port),work,readiness_deadline,desktop_stage)
            compiled_pair.attach(pipe_type)
        binary=t.ARTIFACTS/"device-api-client-win.exe"
        uploaded=subprocess.run(["scp","-O","-o","BatchMode=yes","-o","IdentitiesOnly=yes","-o","StrictHostKeyChecking=yes","-i",str(Path.home()/".ssh/remoteapp_win_ed25519"),str(binary),"jarvis@100.73.4.118:"+work.replace("\\","/")+"/device-api-client.exe"],capture_output=True,timeout=60)
        t.check(uploaded.returncode==0,"split_windows_api_client_uploaded")
        actual=t.powershell(f"(Get-FileHash -Algorithm SHA256 '{work}\\device-api-client.exe').Hash")
        t.check(actual.lower()==hashlib.sha256(binary.read_bytes()).hexdigest(),"split_windows_api_binary_verified")
        for windows, port, role in [(True,remote_port,"native-target"),(False,local_port,"native-controller")]:
            config={"Endpoint":f"https://127.0.0.1:{port}","Certificate":ready["api_certificate"],"Issuer":ready["issuer"],
                "Target":hashlib.sha256(bytes.fromhex(ready["target"])).hexdigest(),"Controller":hashlib.sha256(bytes.fromhex(ready["controller"])).hexdigest()}
            config.update(identities.config(role,windows))
            command=identities.command(work,"device-api-client.exe") if windows else [str(t.ARTIFACTS/"device-api-client")]
            client=pipe_type(command,config); clients.append(client)
            t.check(client.read()["status"]=="ready","split_device_os_signer_ready")
        if nodes:
            import split_nodes
            def node_phase(grant):
                node=split_nodes.run(t,pipe_type,identities,ready,clients,(remote_port,local_port),work,grant,node_helpers,folders)
                node_result.append(node)
                if compiled:
                    compiled_pair.run(pipe_type,node["compiled_handoff"],live_revoke=live_revoke,authority_fault=fault)
                    if fault:
                        compiled_products.fault_round(t,coordinator,clients,node,compiled_pair)
                if tls:
                    active=split_tls.run(t,Pipe,ready,clients,node,grant,product_command,node_helpers)
                    if fault: active=split_tls.fault_round(t,Pipe,ready,clients,node,grant,product_command,node_helpers,coordinator,active)
                    active_tls.append(active)
            consent(*clients,node_phase=node_phase)
            if live_revoke: compiled_pair.revoked()
            if tls: active_tls[0].revoked()
            split_nodes.reclaimed(t)
            node_cleanup_confirmed=True
            if not tls and not compiled:
                node=node_result[0]
                t.check(not node["peer"].request("probe",Peer=node["target_ip"])["passed"],"split_revocation_actual_network_payload_denied")
            for helper in reversed(node_helpers): helper.stop()
            node_helpers.clear()
            if compiled:
                compiled_pair.close()
                compiled_pair_joined=True
        else: consent(*clients)
        for client in reversed(clients): client.stop()
        clients.clear()
        coordinator.send({"Op":"stop"})
        code=coordinator.join(); joined=True
        t.check(code==0,"split_consent_coordinator_zero_exit")
    except Exception as original:
        # Record the primary stage before finally can fail its independent
        # teardown checks. Never print raw exceptions/private protocol replies.
        import traceback
        frame = traceback.extract_tb(original.__traceback__)[-1]
        print("PRIMARY_DIAGNOSTIC " + type(original).__name__ + " " +
              Path(frame.filename).name + ":" + str(frame.lineno), flush=True)
        raise
    finally:
        if compiled and work_created:
            print("PRESERVED_WINDOWS_COMPILED_WORK "+work,flush=True)
            if compiled_pair is None:
                print("ACTION: if a borrower was started, stop it FIRST and confirm STOPPED before stopping the identity creator. No compiled session acceptance was reached.",flush=True)
        if compiled and compiled_pair is not None and not compiled_pair_joined:
            # If this cannot join, deliberately stop teardown here: keep
            # signers/tunnels/state/policy evidence. Never emergency-kill them.
            compiled_pair.close()
            compiled_pair_joined=True
        for helper in reversed(node_helpers): helper.emergency()
        for client in reversed(clients): client.emergency()
        if coordinator is not None and not joined:
            try:
                coordinator.join(); joined=True
            except Exception:
                # Never restore policy/delete evidence while remote process
                # lifetime is unknown. These handles contain no secret data.
                if backup is not None: print("PRESERVED_POLICY_BACKUP "+backup,flush=True)
                print("PRESERVED_COORDINATOR_PROGRAM "+program,flush=True)
                for tunnel in reversed(tunnels):
                    tunnel.terminate(); tunnel.wait(timeout=5)
                identities.close()
                raise
        for tunnel in reversed(tunnels):
            tunnel.terminate(); tunnel.wait(timeout=5)
        identities.close()  # Only this run's random Mac identities, never Windows owner.
        if work_created and not compiled and (node_cleanup_confirmed or coordinator is None):
            t.powershell(f"Remove-Item -LiteralPath '{work}' -Recurse -Force -ErrorAction Stop")
        if node_cleanup_confirmed:
            for folder in folders: shutil.rmtree(folder)
        elif folders or work_created:
            print("PRESERVED_CLIENT_NODE_JOURNALS",flush=True)
        if backup is not None:
            restored=subprocess.run(t.SERVER+["python "+mode+" restore "+backup],capture_output=True,timeout=45)
            if restored.returncode!=0: print("PRESERVED_POLICY_BACKUP "+backup,flush=True)
            t.check(restored.returncode==0,"split_original_file_policy_restored")
        removed=subprocess.run(t.SERVER+["rm -f -- "+" ".join((program,daemon,frontend,mode))],capture_output=True,timeout=15)
        t.check(removed.returncode==0,"split_uploaded_executables_removed")
    t.check(not t.admin("nodes","list") and not t.admin("users","list") and not t.admin("preauthkeys","list"),"split_control_resources_zero")
    t.check(t.admin("policy","get")=={"acls":[{"action":"accept","src":["100.120.0.2"],"dst":["100.120.0.1:47476"]}]},"split_original_policy_unchanged")
    status=subprocess.run(t.SERVER+["systemctl is-active remoteapp-authority remoteapp-frontend remoteapp-poc && sha256sum --check /opt/remoteapp-prerelease/nginx-baseline.sha256 && sha256sum /var/lib/remoteapp-authority/state.json"],capture_output=True,timeout=15)
    t.check(status.returncode==0 and b"5c40c3230bd331b5ff47c9aefb18dd16c7f4e3d4d19f1ae18caed4caf1db33ca" in status.stdout,"split_online_services_store_nginx_unchanged")
    if compiled:
        print("ACTION: verification and server/client teardown finished. Enter in BORROWER window FIRST and confirm its STOPPED, then Enter in identity creator and confirm STOPPED.",flush=True)
        if fault:
            print("VALIDATION PASSED: compiled native pair + real Headscale/product TLS + isolated authority outage with stale ACL + autonomous fault join/native retirement + explicit same-verifier recovery with old owners closed. NOT new product-session reconnect, borrower STOPPED cleanup, upgrade, GUI or signed installer acceptance.",flush=True)
        elif live_revoke:
            print("VALIDATION PASSED: compiled native pair + real Headscale/product TLS + signed live-session revocation + autonomous fault join/native retirement. NOT borrower STOPPED cleanup, authority-outage fault, upgrade, GUI or signed installer acceptance.",flush=True)
        else:
            print("VALIDATION PASSED: compiled native pair + real Headscale binding + product TLS payload/autonomous refresh + normal join + post-stop signed revoke. NOT live-session revocation, borrower STOPPED cleanup, upgrade/fault, GUI or signed installer acceptance.",flush=True)
        return
    scope=" + real nodeproof + actual product TLS/autonomous refresh/revocation" if tls else " + real nodeproof/minimum ACL/revoked payload" if nodes else " only"
    print("VALIDATION PASSED: split-process OS consent"+scope+". NOT installed trust/public product acceptance.",flush=True)

if __name__=="__main__":
    try: main()
    except Exception as error:
        # Frame metadata only: raw errors/replies can contain private inputs.
        import traceback
        frame=traceback.extract_tb(error.__traceback__)[-1]
        print("DIAGNOSTIC "+type(error).__name__+" "+Path(frame.filename).name+":"+str(frame.lineno),flush=True)
        print("FAIL split_os_consent_coordinator",flush=True)
        raise SystemExit(1)
