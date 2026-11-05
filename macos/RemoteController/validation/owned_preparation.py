#!/usr/bin/env python3
"""Start/prepare/stop the dedicated Mac GUI; never click or approve consent.

Only synthetic public IDs are sent. No bootstrap/key/frame/input is supplied.
Unknown joins retain this original driver/process; no terminate/kill fallback.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import selectors
import subprocess
import time


def read_frame(stream, seconds):
    deadline = time.monotonic() + seconds
    raw = bytearray()
    with selectors.DefaultSelector() as ready:
        ready.register(stream, selectors.EVENT_READ)
        while len(raw) < 8192:
            if not ready.select(max(0, deadline - time.monotonic())):
                raise RuntimeError("private_frame_timeout")
            byte = os.read(stream.fileno(), 1)
            if not byte:
                raise RuntimeError("private_frame_eof")
            if byte == b"\n":
                try:
                    return json.loads(raw)
                except (ValueError, UnicodeError):
                    raise RuntimeError("private_frame_invalid") from None
            raw.extend(byte)
    raise RuntimeError("private_frame_oversized")


def send(stream, value):
    stream.write(json.dumps(value, separators=(",", ":"), sort_keys=True).encode() + b"\n")
    stream.flush()


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--executable", required=True)
    parser.add_argument("--output", required=True)
    parser.add_argument("--pending-fault", action="store_true")
    args = parser.parse_args()
    executable = Path(args.executable).resolve(strict=True)
    output = Path(args.output)
    if output.exists() or not executable.is_file():
        raise RuntimeError("validation_path_rejected")
    pin = hashlib.sha256(executable.read_bytes()).hexdigest()
    report = {"test": "owned_gui_pending_fault" if args.pending_fault else "owned_gui_preparation_only", "sha256": pin, "runs": []}
    output.parent.mkdir(parents=True, exist_ok=True)

    def save():
        output.write_text(json.dumps(report, indent=2) + "\n")

    scopes = ("desktop-view",) if args.pending_fault else ("desktop-view", "desktop-control")
    for permission in scopes:
        if hashlib.sha256(executable.read_bytes()).hexdigest() != pin:
            raise RuntimeError("executable_changed")
        child = subprocess.Popen([str(executable)], stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                                 stderr=subprocess.DEVNULL, bufsize=0)
        run = {"permission": permission, "pid": child.pid, "passed": False}
        report["runs"].append(run)
        stopped = False
        try:
            send(child.stdin, {"Op": "prepare", "Permission": permission,
                               "Role": "controller", "Self": "ab" * 32})
            if read_frame(child.stdout, 8) != {"status": "desktop_prepared", "Permission": permission,
                                               "Role": "controller"}:
                raise RuntimeError("preparation_rejected")
            if args.pending_fault:
                question = {"Op": "consent", "Controller": "ab" * 32, "RequestID": "cd" * 16}
                send(child.stdin, question)
                send(child.stdin, question)
                if read_frame(child.stdout, 4) != {"status": "failed"}:
                    raise RuntimeError("pending_fault_not_terminal")
            send(child.stdin, {"Op": "desktop_state"})
            if read_frame(child.stdout, 4) != {"status": "desktop_state", "frames": 0, "inputs": 0,
                                               "stop_requested": args.pending_fault, "ended": False, "failed": args.pending_fault}:
                raise RuntimeError("unactivated_state_rejected")
            # Observe just this process, never dump endpoint details to logs.
            sockets = subprocess.run(["/usr/sbin/lsof", "-nP", "-a", "-p", str(child.pid), "-i"],
                                     stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, timeout=4)
            if sockets.returncode != 1 or sockets.stdout:
                raise RuntimeError("unexpected_network_endpoint")
            run["no_network_endpoints"] = True
            send(child.stdin, {"Op": "stop"})
            stopped = True
            child.stdin.close()
            if read_frame(child.stdout, 10) != {"status": "closed"}:
                raise RuntimeError("closed_receipt_rejected")
            code = child.wait(timeout=10)
            if child.stdout.read(1) != b"" or code != 0:
                raise RuntimeError("exit_or_eof_rejected")
            run.update(passed=True, zero_exit=True, stdout_eof=True)
        except Exception as error:
            run["failure"] = str(error) if isinstance(error, RuntimeError) else "validation_failed"
        finally:
            if not stopped:
                try:
                    send(child.stdin, {"Op": "stop"})
                    child.stdin.close()
                except (BrokenPipeError, OSError, ValueError):
                    pass
            if child.poll() is None:
                try:
                    child.wait(timeout=15)
                except subprocess.TimeoutExpired:
                    run["join_pending"] = True
                    save()
                    print("GUI_JOIN_PENDING", flush=True)
                    child.wait()  # retain ownership, never kill or start another run
            run["exit_code"] = child.returncode
            child.stdout.close()
            save()
        if not run["passed"]:
            print("GUI_PREPARATION_FAILED", flush=True)
            return 1
    print("GUI_PENDING_FAULT_JOINED_NO_CONSENT" if args.pending_fault else "GUI_PREPARATION_JOINED_NO_CONSENT", flush=True)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
