"""Local-only Headscale/tsnet/DERP test. Bootstrap keys never leave process pipes."""
import contextlib
import json
import os
from pathlib import Path
import secrets
import select
import signal
import socket
import subprocess
import sys
import tempfile
import time
import urllib.request

HERE = Path(__file__).resolve().parent
ARTIFACTS = HERE.parents[1] / "artifacts" / "connection-poc"
HEADSCALE = ARTIFACTS / "toolchain" / "headscale-0.29.4"
NODE = ARTIFACTS / "selfhost-node"
DERP = ARTIFACTS / "lab-derp"


def check(condition, label):
    if not condition:
        raise RuntimeError(label)
    print("PASS " + label, flush=True)


def stop(proc):
    if proc.poll() is None:
        proc.terminate()
        try:
            proc.wait(timeout=12)
        except subprocess.TimeoutExpired:
            proc.kill()
            proc.wait(timeout=5)


def start(stack, args, **kwargs):
    proc = subprocess.Popen(args, cwd=HERE, stderr=subprocess.DEVNULL, bufsize=0, **kwargs)
    stack.callback(stop, proc)
    return proc


def read_status(proc, timeout=30):
    # No raw stdout/stderr is displayed: it may contain bootstrap metadata.
    ready, _, _ = select.select([proc.stdout], [], [], timeout)
    if not ready:
        raise RuntimeError("status_timeout")
    line = proc.stdout.readline()
    if not line:
        raise RuntimeError("process_exited_before_status")
    return json.loads(line)


def collect_events(proc, timeout=45):
    deadline = time.monotonic() + timeout
    events = []
    while True:
        remaining = deadline - time.monotonic()
        if remaining <= 0:
            raise RuntimeError("node_timeout")
        readable, _, _ = select.select([proc.stdout], [], [], remaining)
        if not readable:
            # Only previously validated fixed status codes are used here.
            stage = events[-1]["status"] if events else "no_status"
            if hasattr(proc, "diagnostics"):
                proc.send_signal(signal.SIGQUIT)
                proc.wait(timeout=5)
                proc.diagnostics.seek(0)
                trace = proc.diagnostics.read().decode("utf-8", errors="replace")
                # Function names only; never print stack arguments or raw logs.
                lines = trace.splitlines()
                funcs = [line.rsplit("(", 1)[0] + " " + lines[i + 1].rsplit("/", 1)[-1].split(" +", 1)[0]
                         for i, line in enumerate(lines[:-1])
                         if line.startswith(("main.", "tailscale.com/", "gvisor.dev/"))]
                for func in funcs[:12]:
                    print("TRACE " + func, flush=True)
            raise RuntimeError("node_timeout_after_" + stage)
        line = proc.stdout.readline()
        if not line:
            break
        item = json.loads(line)
        allowed = {"registered", "selfhost_relay", "direct", "unknown", "probe_passed",
                   "registration_failed", "dial_failed", "request_failed", "response_failed",
                   "status_failed", "relay_not_verified", "dialing", "closing", "closed", "cleanup_timeout",
                   "wrong_token_rejected", "negative_dial_failed", "wrong_token_accepted"}
        if item.get("status") not in allowed:
            raise RuntimeError("unexpected_status")
        events.append(item)
        if item["status"] == "closing":
            deadline = min(deadline, time.monotonic() + 12)
    proc.wait(timeout=5)
    return [item["status"] for item in events]


def main():
    if sys.argv[1:] not in ([], ["--check-reconnect"]):
        raise RuntimeError("invalid_arguments")
    os.umask(0o077)
    for port in (18443, 18444, 19090, 15443):
        with socket.socket() as sock:
            sock.bind(("127.0.0.1", port))
    nodes_root = ARTIFACTS / "nodes"
    nodes_root.mkdir(parents=True, exist_ok=True, mode=0o700)
    with contextlib.ExitStack() as stack:
        # Short socket path avoids macOS sockaddr_un path limits.
        state = Path(stack.enter_context(tempfile.TemporaryDirectory(prefix="remoteapp-hs-")))
        derp = start(stack, [str(DERP)], stdout=subprocess.PIPE)
        relay_status = read_status(derp)
        check(relay_status["status"] == "listening", "local DERP started with pinned certificate")
        cert_name = relay_status["cert_name"]
        derp_yaml = state / "derp.yaml"
        derp_yaml.write_text(
            "regions:\n  999:\n    regionid: 999\n    regioncode: lab\n"
            "    regionname: RemoteApp Local Lab\n    nodes:\n"
            "      - name: lab-1\n        regionid: 999\n"
            "        hostname: 127.0.0.1\n        ipv4: 127.0.0.1\n"
            "        ipv6: none\n        stunport: -1\n        derpport: 18444\n"
            f"        certname: {cert_name}\n", encoding="utf-8")
        cfg = (HERE / "headscale.yaml").read_text(encoding="utf-8")
        cfg = cfg.replace("../../artifacts/connection-poc/headscale", str(state))
        cfg = cfg.replace("paths: [derp.yaml]", f"paths: [{derp_yaml}]")
        cfg = cfg.replace("path: policy.json", f"path: {HERE / 'policy.json'}")
        config_path = state / "config.yaml"
        config_path.write_text(cfg, encoding="utf-8")
        hs = start(stack, [str(HEADSCALE), "--config", str(config_path), "serve"], stdout=subprocess.DEVNULL)
        opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
        deadline = time.monotonic() + 15
        while True:
            if hs.poll() is not None:
                raise RuntimeError("headscale_start_failed")
            try:
                with opener.open("http://127.0.0.1:18443/health", timeout=1) as response:
                    if response.status == 200:
                        break
            except OSError:
                pass
            if time.monotonic() > deadline:
                raise RuntimeError("headscale_health_timeout")
            time.sleep(0.2)
        check(True, "isolated Headscale healthy")

        def admin(*args):
            result = subprocess.run([str(HEADSCALE), "--config", str(config_path), *args, "--output", "json"],
                                    cwd=HERE, capture_output=True, timeout=10)
            if result.returncode:
                raise RuntimeError("admin_failed")
            return json.loads(result.stdout)

        user = admin("users", "create", "poc")
        user_id = str(user["id"])

        def registration_key():
            result = admin("preauthkeys", "create", "--user", user_id, "--expiration", "5m")
            return result["key"]

        session_token = secrets.token_hex(32)

        def config(role, name, token=session_token, force=False):
            folder = stack.enter_context(tempfile.TemporaryDirectory(prefix=name + "-", dir=nodes_root))
            return {"role": role, "control_url": "http://127.0.0.1:18443", "state_dir": folder,
                    "hostname": name, "auth_key": registration_key(), "session_token": token,
                    "peer": "100.120.0.1" if role == "probe" else "", "force_relay": force,
                    "check_reject": name == "poc-client"}

        def node(c):
            diagnostics = stack.enter_context(tempfile.TemporaryFile())
            proc = subprocess.Popen([str(NODE)], cwd=HERE, stdin=subprocess.PIPE,
                                    stdout=subprocess.PIPE, stderr=diagnostics, bufsize=0)
            proc.diagnostics = diagnostics
            stack.callback(stop, proc)
            proc.stdin.write(json.dumps(c).encode("utf-8"))
            proc.stdin.close()
            return proc

        server_config = config("serve", "poc-server", force=True)
        server = node(server_config)
        registered = read_status(server)
        check(registered["status"] == "registered" and "100.120.0.1" in registered["addresses"], "server registered without browser login")
        check(read_status(server)["status"] == "listening", "embedded listener ready")
        client_config = config("probe", "poc-client", force=True)
        client = node(client_config)
        # stdin has been closed explicitly, so consume stdout then wait.
        statuses = collect_events(client)
        check("selfhost_relay" in statuses and "probe_passed" in statuses,
              "fixed payload traversed self-hosted DERP")
        shutdown_issues = ["cleanup_timeout" in statuses or client.returncode != 0]

        check("wrong_token_rejected" in statuses, "client observed wrong-token rejection")
        check(read_status(server)["status"] == "exchange_passed" and read_status(server)["status"] == "token_rejected",
              "server confirmed wrong-token rejection")

        if "--check-reconnect" in sys.argv[1:]:
            reconnect = node(client_config)
            reconnect_statuses = collect_events(reconnect)
            print("STATE reconnect: " + ",".join(reconnect_statuses), flush=True)
            check("registered" in reconnect_statuses and "probe_passed" in reconnect_statuses,
                  "client_reconnect_failed" if "probe_passed" not in reconnect_statuses else "client reconnected")

        outsider = node(config("probe", "poc-denied", force=True))
        statuses = collect_events(outsider)
        shutdown_issues.append("cleanup_timeout" in statuses)
        check(outsider.returncode != 0 and "registered" in statuses and "dial_failed" in statuses,
              "third registered node denied by ACL")
        stop(server)
        shutdown_issues.append(server.returncode != 0)
        restarted = node(server_config)
        status = read_status(restarted)
        check(status["status"] == "registered" and "100.120.0.1" in status["addresses"],
              "server identity retained across restart")
        check(len(admin("nodes", "list")) == 3, "restart did not create a fourth node")
        stop(restarted)
        shutdown_issues.append(restarted.returncode != 0)
    print("PASS all fixture processes stopped; temporary lab identities removed", flush=True)
    if any(shutdown_issues):
        raise RuntimeError("graceful_shutdown_unresolved_network_checks_passed")


if __name__ == "__main__":
    try:
        main()
    except Exception as exc:
        # Output fixed RuntimeError labels or exception class, never raw admin errors.
        label = str(exc) if isinstance(exc, RuntimeError) else type(exc).__name__
        print("FAIL local smoke: " + label, flush=True)
        raise SystemExit(1)
