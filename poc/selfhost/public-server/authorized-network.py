"""Private dual-machine authorization fixture; NOT a public enrollment service.

Backend responses/keys are private pipe data and must never be logged. SSH is
management only. Actual payload goes through isolated tsnet to Windows.
"""
import hashlib
import json
from pathlib import Path
import secrets
import select
import shutil
import subprocess
import sys
import tempfile
import threading
import time

import importlib.util
spec = importlib.util.spec_from_file_location("transport", Path(__file__).with_name("cross-network.py"))
t = importlib.util.module_from_spec(spec)
spec.loader.exec_module(t)
t.ALLOWED.update({"authorized_stream_open", "authorized_stream_closed", "authorized_tick",
                  "session_denied", "node_binding_rejected", "invalid_authorization",
                  "invalid_authorization_update", "wrong_proof_rejected", "negative_proof_failed"})


class Authority:
    def __init__(self):
        self.proc = subprocess.Popen([str(t.ARTIFACTS/"grant-fixture")], cwd=t.HERE,
                                     stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                                     stderr=subprocess.DEVNULL, bufsize=0)
        self.lock = threading.Lock()

    def request(self, op, **fields):
        with self.lock:
            self.proc.stdin.write(json.dumps(dict(op=op, **fields)).encode()+b"\n")
            ready, _, _ = select.select([self.proc.stdout], [], [], 3)
            if not ready:
                raise RuntimeError("authority_timeout")
            line = self.proc.stdout.readline()
            if not line:
                raise RuntimeError("authority_closed")
            return json.loads(line)

    def close(self):
        self.proc.stdin.close()
        self.proc.wait(timeout=6)
        t.check(self.proc.returncode == 0, "private_authority_closed")


class Refresh:
    def __init__(self, authority, server):
        self.stop_event = threading.Event()
        self.failed = False

        self.last_active = None
        self.requests = 0

        def work():
            try:
                while not self.stop_event.wait(0.5):
                    n = authority.request("lease")["state"]
                    server.proc.stdin.write(json.dumps(n).encode()+b"\n")
                    self.requests += 1
                    if n["claims"]["active"]: self.last_active = n
            except Exception:
                self.failed = True  # fixed label only, never raw exception/key
        self.thread = threading.Thread(target=work, daemon=True)
        self.thread.start()

    def close(self):
        self.stop_event.set()
        self.thread.join(timeout=5)
        t.check(not self.thread.is_alive() and not self.failed, "signed_refresh_stopped")


def main():
    if sys.argv[1:] not in ([],["--negative-expiry"]):
        raise RuntimeError("invalid_arguments")
    extended = bool(sys.argv[1:])
    t.os.umask(0o077)
    t.check(not (t.admin("nodes", "list") or []), "isolated_database_empty")
    p = subprocess.run(t.SERVER+["systemctl restart remoteapp-poc"], capture_output=True, timeout=25)
    t.check(p.returncode == 0, "empty_fixture_control_restarted")
    p = subprocess.run(["curl", "--silent", "--fail", "--noproxy", "*", "--retry", "5",
                        "--retry-connrefused", "--retry-delay", "1", "--max-time", "5",
                        "https://mk.fengmap.com:8443/health"], capture_output=True, timeout=35)
    t.check(p.returncode == 0, "public_control_healthy")
    run = time.strftime("%Y%m%d%H%M%S")+secrets.token_hex(3)
    root = "C:\\Users\\jarvis\\remoteapp-grant-poc-"+run
    work = root+"\\poc\\selfhost"
    binary = t.ARTIFACTS/"selfhost-node-public-win-x64.exe"
    digest = hashlib.sha256(binary.read_bytes()).hexdigest()
    nodes, keys, folders = [], [], []
    user_id, authority, refresh = None, None, None
    try:
        t.powershell("$ErrorActionPreference='Stop'; "
                     f"if (Test-Path '{root}') {{throw 'exists'}}; "
                     f"New-Item -ItemType Directory -Path '{work}' -Force | Out-Null; "
                     "$sid=[System.Security.Principal.WindowsIdentity]::GetCurrent().User.Value; "
                     f"icacls '{root}' /inheritance:r /grant:r \"*${{sid}}:(OI)(CI)F\" '*S-1-5-18:(OI)(CI)F' | Out-Null; "
                     "if ($LASTEXITCODE -ne 0) {throw 'acl_failed'}")
        destination = root.replace("\\", "/")+"/poc/selfhost/selfhost-node.exe"
        p = subprocess.run(["scp", "-O", "-o", "BatchMode=yes", "-o", "IdentitiesOnly=yes",
                            "-o", "StrictHostKeyChecking=yes", "-i", str(Path.home()/".ssh/remoteapp_win_ed25519"),
                            str(binary), "jarvis@100.73.4.118:"+destination], capture_output=True, timeout=90)
        t.check(p.returncode == 0, "windows_helper_uploaded")
        actual = t.powershell(f"(Get-FileHash -Algorithm SHA256 '{work}\\selfhost-node.exe').Hash")
        t.check(actual.lower() == digest, "windows_helper_checksum")
        user_id = str(int(t.admin("users", "create", "grant-poc-"+run)["id"]))
        def key():
            result = t.admin("preauthkeys", "create", "--user", user_id, "--expiration", "5m")
            keys.append(str(int(result["id"])))
            return result["key"]
        node_root = t.ARTIFACTS/"nodes"
        node_root.mkdir(mode=0o700, exist_ok=True)
        folder = Path(tempfile.mkdtemp(prefix="grant-client-", dir=node_root))
        folders.append(folder)
        common = dict(control_url="https://mk.fengmap.com:8443", session_token=secrets.token_hex(32), force_relay=True)
        sc = dict(common, role="serve", hostname="poc-server", auth_key=key(),
                  state_dir="../../artifacts/connection-poc/nodes/grant-server", peer="")
        pc = dict(common, role="probe", hostname="poc-client", auth_key=key(), state_dir=str(folder), peer="100.120.0.1")
        wc = t.WIN+[f'cmd.exe /d /c "cd /d {work} && selfhost-node.exe --control-stdin"']
        mc = [str(t.ARTIFACTS/"selfhost-node"), "--control-stdin"]
        def launch(cmd, cfg):
            node = t.Node(cmd, cfg); nodes.append(node); return node
        server = launch(wc, sc); server.until("listening")
        client = launch(mc, pc)
        events = client.finish()
        t.check(client.proc.returncode == 0 and "selfhost_relay" in events and "probe_passed" in events,
                "isolated_network_nodes_registered")
        server.stop()
        records = t.admin("nodes", "list") or []
        t.check(len(records) == 2 and all(str(n["user"]["id"]) == user_id for n in records), "two_owned_nodes_only")
        def node_key(ip):
            for n in records:
                if ip in (n.get("ip_addresses") or n.get("ipAddresses") or []):
                    return n.get("node_key") or n.get("nodeKey")
            raise RuntimeError("node_binding_missing")
        binding = dict(target_node=node_key("100.120.0.1"), controller_node=node_key("100.120.0.2"),
                       target_ip="100.120.0.1", controller_ip="100.120.0.2")
        authority = Authority()
        credentials = authority.request("bind", binding=binding)
        sc["session_token"] = pc["session_token"] = ""
        def start_pair(creds, wrong_proof=False):
            nonlocal server, refresh
            sc["authorization"] = creds["server"]
            server = launch(wc, sc)
            refresh = Refresh(authority, server)
            server.until("listening")
            fresh = authority.request("lease")
            pc["authorization"] = fresh["probe"]
            if wrong_proof: pc["authorization"]["reject_proof"] = True
            return launch(mc, pc)
        def ended(client, label):
            events = client.finish()
            t.check(client.proc.returncode != 0 and "session_denied" in events and "closed" in events
                    and "cleanup_timeout" not in events, label)
        client = start_pair(credentials)
        for _ in range(3): client.until("authorized_tick")
        t.check(any(e["status"] == "selfhost_relay" for e in client.events), "authorized_payload_path_verified")
        t.check(True, "approved_signed_grant_carries_real_payload")
        authority.request("revoke")
        ended(client, "backend_revoke_closes_active_stream")
        refresh.close(); refresh = None
        server.stop()
        credentials = authority.request("restart")
        t.check(not credentials["state"]["claims"]["active"], "backend_restart_preserves_revoke")
        client = start_pair(credentials)
        ended(client, "restarted_server_rejects_revoked_grant")
        t.check(not any(e["status"] == "authorized_tick" for e in client.events), "revoked_grant_emits_no_payload")
        refresh.close(); refresh = None
        server.stop()
        credentials = authority.request("renew")
        client = start_pair(credentials)
        for _ in range(3): client.until("authorized_tick")
        t.check(True, "new_explicit_grant_succeeds")
        refresh.close(); refresh = None  # backend outage: no TTL reset on receipt
        ended(client, "signed_state_expiry_closes_active_stream")
        server.stop()
        if extended:
            credentials = authority.request("pending")
            t.check(credentials["request_status"] == "pending" and not credentials["state"]["claims"]["active"], "unapproved_request_has_no_authorization")
            client = start_pair(credentials)
            ended(client, "pending_request_cannot_carry_payload")
            t.check(not any(e["status"] == "authorized_tick" for e in client.events), "pending_zero_payload")
            refresh.close(); refresh = None; server.stop()
            credentials = authority.request("deny")
            t.check(credentials["request_status"] == "denied", "owner_explicitly_denied_request")
            client = start_pair(credentials)
            ended(client, "denied_request_cannot_carry_payload")
            t.check(not any(e["status"] == "authorized_tick" for e in client.events), "denied_zero_payload")
            refresh.close(); refresh = None; server.stop()

            authority.request("renew")
            wrong_binding = dict(binding, target_node="nodekey:"+"c"*64)
            credentials = authority.request("bind", binding=wrong_binding)
            client = start_pair(credentials)
            ev = client.finish()
            t.check(client.proc.returncode != 0 and "node_binding_rejected" in ev and "authorized_tick" not in ev and "cleanup_timeout" not in ev, "signed_wrong_network_node_rejected")
            refresh.close(); refresh = None; server.stop()
            credentials = authority.request("bind", binding=binding)
            client = start_pair(credentials, wrong_proof=True)
            ev = client.finish()
            t.check(client.proc.returncode == 0 and "wrong_proof_rejected" in ev and "authorized_tick" not in ev and "closed" in ev, "third_private_key_proof_rejected_on_allowed_node")
            server.until("session_denied")
            refresh.close(); refresh = None; server.stop()

            credentials = authority.request("lease")
            client = start_pair(credentials)
            client.until("authorized_tick")
            refresh.close(); refresh = None
            old_positive = authority.request("lease")["state"]
            revoked = authority.request("revoke")["state"]
            server.proc.stdin.write(json.dumps(revoked).encode()+b"\n")
            server.proc.stdin.write(json.dumps(old_positive).encode()+b"\n")
            ended(client, "revoke_then_positive_replay_closes_stream")
            ev = server.finish()
            t.check(server.proc.returncode != 0 and "invalid_authorization_update" in ev and "closed" in ev and "cleanup_timeout" not in ev, "old_positive_state_cannot_restore_revoked_helper")

            credentials = authority.request("renew")
            grant_end = credentials["state"]["claims"]["grant"]["claims"]["expires_at"]
            client = start_pair(credentials)
            started = time.monotonic()
            last_report = started
            ticks = 0
            deadline = started+330
            while time.monotonic() < deadline:
                event = client.read(timeout=5)
                if event is None: break
                if event["status"] == "authorized_tick": ticks += 1
                if time.monotonic()-last_report >= 30:
                    print("PROGRESS grant_expiry_elapsed_"+str(int(time.monotonic()-started))+"s", flush=True)
                    last_report = time.monotonic()
            else: raise RuntimeError("grant_expiry_deadline")
            client.proc.wait(timeout=5)
            ev = [e["status"] for e in client.events]
            t.check(client.proc.returncode != 0 and "session_denied" in ev and "closed" in ev and "cleanup_timeout" not in ev and ticks > 100 and time.monotonic()-started >= 280 and grant_end-0.5 <= time.time() <= grant_end+8, "real_300_second_grant_expiry_closes_stream")
            last = refresh.last_active
            t.check(not refresh.failed and refresh.requests > 400 and last is not None and last["claims"]["expires_at"] > grant_end*1_000_000_000, "fresh_state_remained_valid_beyond_grant_expiry")
            refresh.close(); refresh = None; server.stop()
            t.check(True, "public_negative_and_grant_expiry_complete")
        t.check(len(t.admin("nodes", "list") or []) == 2, "authorization_did_not_duplicate_nodes")
        t.check(True, "dual_machine_signed_authorization_complete")
    finally:
        cleanup_errors = []
        def cleanup(fn):
            try: fn()
            except Exception: cleanup_errors.append(True)
        if refresh is not None: cleanup(refresh.close)
        for node in reversed(nodes): cleanup(node.emergency)
        if authority is not None: cleanup(authority.close)
        if user_id is not None:
            for kid in keys: cleanup(lambda kid=kid: t.admin("preauthkeys", "expire", "--id", kid))
            def remove_nodes():
                for n in t.admin("nodes", "list") or []:
                    if str(n["user"]["id"]) == user_id:
                        cleanup(lambda n=n: t.admin("nodes", "delete", "--identifier", str(int(n["id"])), "--force"))
            cleanup(remove_nodes)
            cleanup(lambda: t.admin("users", "destroy", "--identifier", user_id, "--force"))
            cleanup(lambda: t.check(not (t.admin("nodes", "list") or []), "test_registrations_removed"))
        cleanup(lambda: t.powershell(f"$ErrorActionPreference='Stop'; $p='{root}\\artifacts\\connection-poc\\nodes'; "
                                    "if (Test-Path -LiteralPath $p) {Remove-Item -LiteralPath $p -Recurse -Force}"))
        for folder in folders: cleanup(lambda folder=folder: shutil.rmtree(folder))
        if cleanup_errors: raise RuntimeError("fixture_cleanup_incomplete")


if __name__ == "__main__":
    try:
        main()
    except Exception:
        # Do not surface command output or exception strings containing secrets.
        print("FAIL authorized_network_verification", flush=True)
        raise SystemExit(1)
