"""Authorized Mac/Windows fixed-payload PoC; credentials remain in private pipes.

SSH uses the installed Tailscale only for administration. Helper data uses its
own tsnet identity and the exact public Headscale/DERP endpoint. No product IPC.
"""
import base64
import hashlib
import json
import os
from pathlib import Path
import secrets
import select
import subprocess
import tempfile
import time

HERE = Path(__file__).resolve().parents[1]
ARTIFACTS = HERE.parents[1] / "artifacts/connection-poc"
SSH_COMMON = ["ssh", "-o", "BatchMode=yes", "-o", "IdentitiesOnly=yes",
              "-o", "StrictHostKeyChecking=yes", "-o", "ConnectTimeout=8"]
SERVER = SSH_COMMON + ["-i", str(Path.home()/".ssh/remoteapp_poc_server_ed25519"), "root@182.92.117.114"]
WIN = SSH_COMMON + ["-i", str(Path.home()/".ssh/remoteapp_win_ed25519"), "-l", "jarvis", "100.73.4.118"]
HS = "/opt/remoteapp-poc/headscale -c /etc/remoteapp-poc/config.yaml"
ALLOWED = {"registered", "listening", "exchange_passed", "token_rejected", "dialing",
           "selfhost_relay", "direct", "unknown", "probe_passed", "wrong_token_rejected",
           "closing", "closed", "cleanup_timeout", "registration_failed", "dial_failed",
           "response_failed", "request_failed", "status_failed", "relay_not_verified",
           "negative_dial_failed", "wrong_token_accepted"}


def check(ok, label):
    if not ok:
        raise RuntimeError(label)
    print("PASS " + label, flush=True)


def admin(*args):
    # All arguments are internally chosen command words/decimal IDs, not secrets.
    result = subprocess.run(SERVER + [HS + " " + " ".join(args) + " -o json"],
                            capture_output=True, timeout=20)
    if result.returncode:
        raise RuntimeError("admin_failed")
    return json.loads(result.stdout or b"null")


def powershell(script):
    encoded = base64.b64encode(script.encode("utf-16le")).decode()
    result = subprocess.run(WIN + ["powershell.exe -NoProfile -NonInteractive -EncodedCommand " + encoded],
                            capture_output=True, timeout=25)
    if result.returncode:
        raise RuntimeError("windows_management_failed")
    return result.stdout.decode().strip()


class Node:
    def __init__(self, command, config):
        self.proc = subprocess.Popen(command, cwd=HERE, stdin=subprocess.PIPE,
                                     stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, bufsize=0)
        self.events = []
        self.proc.stdin.write(json.dumps(config).encode() + b"\n")

    def read(self, timeout=40):
        ready, _, _ = select.select([self.proc.stdout], [], [], timeout)
        if not ready:
            raise RuntimeError("status_timeout")
        line = self.proc.stdout.readline()
        if not line:
            return None
        event = json.loads(line)
        if event.get("status") not in ALLOWED:
            raise RuntimeError("unexpected_status")
        self.events.append(event)
        return event

    def until(self, status):
        deadline = time.monotonic()+45
        while time.monotonic() < deadline:
            event = self.read(max(0.1, deadline-time.monotonic()))
            if event is None:
                raise RuntimeError("node_exited_before_"+status)
            if event["status"] == status:
                return event
        raise RuntimeError("status_timeout")

    def finish(self):
        deadline = time.monotonic()+40
        while self.read(max(0.1, deadline-time.monotonic())) is not None:
            if time.monotonic() >= deadline:
                raise RuntimeError("node_timeout")
        self.proc.wait(timeout=5)
        return [e["status"] for e in self.events]

    def stop(self):
        if self.proc.poll() is None:
            self.proc.stdin.write(b"stop\n")
            self.proc.stdin.close()
        events = self.finish()
        check(self.proc.returncode == 0 and "closed" in events and "cleanup_timeout" not in events,
              "server_graceful_stop")

    def emergency(self):
        if self.proc.poll() is None:
            try:
                self.proc.stdin.write(b"stop\n")
                self.proc.stdin.close()
                self.proc.wait(timeout=12)
            except (OSError, subprocess.TimeoutExpired):
                self.proc.kill()
                self.proc.wait(timeout=5)


def main():
    os.umask(0o077)
    check(not (admin("nodes", "list") or []), "isolated_database_empty")
    run = time.strftime("%Y%m%d%H%M%S")+secrets.token_hex(3)
    root = "C:\\Users\\jarvis\\remoteapp-public-poc-"+run
    work = root+"\\poc\\selfhost"
    binary = ARTIFACTS/"selfhost-node-public-win-x64.exe"
    digest = hashlib.sha256(binary.read_bytes()).hexdigest()
    user_id = None
    key_ids = []
    nodes = []
    private_folders = []
    try:
        powershell("$ErrorActionPreference='Stop'; "
                   f"if (Test-Path '{root}') {{throw 'exists'}}; "
                   f"New-Item -ItemType Directory -Path '{work}' -Force | Out-Null; "
                   "$sid=[System.Security.Principal.WindowsIdentity]::GetCurrent().User.Value; "
                   f"icacls '{root}' /inheritance:r /grant:r \"*${{sid}}:(OI)(CI)F\" '*S-1-5-18:(OI)(CI)F' | Out-Null; "
                   "if ($LASTEXITCODE -ne 0) {throw 'acl_failed'}")
        target = root.replace("\\", "/")+"/poc/selfhost/selfhost-node.exe"
        upload = subprocess.run(["scp", "-O", "-o", "BatchMode=yes", "-o", "IdentitiesOnly=yes",
                                 "-o", "StrictHostKeyChecking=yes", "-i", str(Path.home()/".ssh/remoteapp_win_ed25519"),
                                 str(binary), "jarvis@100.73.4.118:"+target], capture_output=True, timeout=90)
        check(upload.returncode == 0, "windows_helper_uploaded")
        actual = powershell(f"(Get-FileHash -Algorithm SHA256 '{work}\\selfhost-node.exe').Hash")
        check(actual.lower() == digest, "windows_helper_checksum")
        user = admin("users", "create", "public-poc-"+run)
        user_id = str(int(user["id"]))
        token = secrets.token_hex(32)
        root_nodes = ARTIFACTS/"nodes"
        root_nodes.mkdir(mode=0o700, exist_ok=True)

        def key():
            result = admin("preauthkeys", "create", "--user", user_id, "--expiration", "5m")
            key_ids.append(str(int(result["id"])))
            return result["key"]

        def config(role, name, folder):
            return dict(role=role, hostname=name, state_dir=str(folder),
                        control_url="https://mk.fengmap.com:8443", auth_key=key(),
                        session_token=token, peer="100.120.0.1" if role == "probe" else "",
                        force_relay=True, check_reject=name == "poc-client")

        def launch(command, cfg):
            node = Node(command, cfg)
            nodes.append(node)
            return node

        windows_command = WIN + [f'cmd.exe /d /c "cd /d {work} && selfhost-node.exe --control-stdin"']
        server_cfg = config("serve", "poc-server", "../../artifacts/connection-poc/nodes/public-server")
        server = launch(windows_command, server_cfg)
        registered = server.until("registered")
        check("100.120.0.1" in registered["addresses"], "windows_server_address")
        server.until("listening")
        local_command = [str(ARTIFACTS/"selfhost-node"), "--control-stdin"]
        folder = Path(tempfile.mkdtemp(prefix="public-client-", dir=root_nodes))
        private_folders.append(folder)
        client_cfg = config("probe", "poc-client", folder)

        def probe(label):
            node = launch(local_command, client_cfg)
            events = node.finish()
            check(node.proc.returncode == 0 and all(e in events for e in
                  ("registered", "selfhost_relay", "probe_passed", "wrong_token_rejected", "closed"))
                  and "cleanup_timeout" not in events, label)
            check(any(e.get("addresses") and "100.120.0.2" in e["addresses"] for e in node.events),
                  "client_identity_preserved")
            server.until("exchange_passed")
            server.until("token_rejected")

        probe("cross_network_selfhost_relay")
        for attempt in range(3):
            probe("client_restart_"+str(attempt+1))
        server.stop()
        server = launch(windows_command, server_cfg)
        check("100.120.0.1" in server.until("registered")["addresses"], "server_identity_preserved")
        server.until("listening")
        probe("server_restart_cross_network")
        folder = Path(tempfile.mkdtemp(prefix="public-denied-", dir=root_nodes))
        private_folders.append(folder)
        denied_cfg = config("probe", "poc-denied", folder)
        denied = launch(local_command, denied_cfg)
        events = denied.finish()
        check(denied.proc.returncode != 0 and "probe_passed" not in events and
              "dial_failed" in events and "closed" in events and "cleanup_timeout" not in events,
              "third_node_denied")
        check(any("100.120.0.3" in e.get("addresses", []) for e in denied.events), "third_node_address")
        active = admin("nodes", "list") or []
        check(len(active) == 3, "node_count_stable_after_restarts")
        server.stop()
        check(True, "cross_network_complete")
    finally:
        for node in reversed(nodes):
            node.emergency()
        if user_id is not None:
            for key_id in key_ids:
                admin("preauthkeys", "expire", "--id", key_id)
            # Match the exact unique user created in this run, never global deletion.
            for node in admin("nodes", "list") or []:
                if str(node["user"]["id"]) == user_id:
                    admin("nodes", "delete", "--identifier", str(int(node["id"])), "--force")
            admin("users", "destroy", "--identifier", user_id, "--force")
            check(not (admin("nodes", "list") or []), "test_registrations_removed")
        # Only the private node subfolder in this newly created Windows test root.
        powershell(f"$ErrorActionPreference='Stop'; $p='{root}\\artifacts\\connection-poc\\nodes'; "
                   "if (Test-Path -LiteralPath $p) {Remove-Item -LiteralPath $p -Recurse -Force}")
        import shutil
        for folder in private_folders:
            shutil.rmtree(folder)
        check(True, "private_test_node_state_removed")


if __name__ == "__main__":
    try:
        main()
    except Exception as exc:
        # Never print raw SSH, Headscale, credentials, stack arguments or logs.
        label = str(exc) if isinstance(exc, RuntimeError) else type(exc).__name__
        if not label.replace("_", "").isalnum():
            label = "verification_failed"
        print("FAIL "+label, flush=True)
        raise SystemExit(1)
