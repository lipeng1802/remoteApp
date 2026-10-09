"""Bounded private test process protocol; no force-kill/false join fallback."""
import json
import os
import select
import subprocess
import threading
import time


def unique_object(pairs):
    result = {}
    names = set()
    for key, value in pairs:
        if key.lower() in names:
            raise RuntimeError("private_duplicate_field")
        names.add(key.lower())
        result[key] = value
    return result


class JoinedPipe:
    def __init__(self, command, config=None, cwd=None):
        self.lock = threading.Lock()
        self.buffer = bytearray()
        self.closed_seen = self.stop_sent = False
        self.prepared_seen = False
        self.failed_seen = False
        self.one_shot = isinstance(config, dict) and config.get("Op") == "prepare"
        self.proc = subprocess.Popen(command, cwd=cwd, stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                                     stderr=subprocess.DEVNULL, bufsize=0)
        if config is not None:
            self.send(config)

    def send(self, value):
        raw = json.dumps(value, separators=(",", ":")).encode() + b"\n"
        if len(raw) > 8192:
            raise RuntimeError("private_input_too_large")
        rest = memoryview(raw)
        while rest:
            written = self.proc.stdin.write(rest)
            if not written:
                raise RuntimeError("private_input_closed")
            rest = rest[written:]

    def read(self, timeout=40):
        deadline = time.monotonic() + timeout
        while b"\n" not in self.buffer:
            left = deadline - time.monotonic()
            if left <= 0 or not select.select([self.proc.stdout], [], [], left)[0]:
                raise RuntimeError("private_pipe_timeout")
            raw = os.read(self.proc.stdout.fileno(), 4096)
            if not raw:
                raise RuntimeError("private_pipe_closed")
            self.buffer.extend(raw)
            if b"\n" not in self.buffer and len(self.buffer) > 8192:
                raise RuntimeError("private_reply_too_large")
        line, remainder = self.buffer.split(b"\n", 1)
        self.buffer = bytearray(remainder)
        if len(line) + 1 > 8192:
            raise RuntimeError("private_reply_too_large")
        value = json.loads(line, object_pairs_hook=unique_object)
        if not isinstance(value, dict) or value.get("status") in ("failed", "cleanup_timeout"):
            if isinstance(value, dict) and value.get("status") == "failed":
                self.failed_seen = True
                stage = value.get("failure_stage")
                if stage not in ("signer_open", "node_state", "intent_sign", "protocol"):
                    stage = "unknown"
                print("DIAGNOSTIC helper_" + stage, flush=True)
            raise RuntimeError("private_helper_failed")
        if value.get("status") == "closed":
            self.closed_seen = True
        if value.get("status") == "prepared":
            self.prepared_seen = True
        return value

    def request(self, op, **values):
        with self.lock:
            self.send(dict(Op=op, **values))
            return self.read()

    def stop(self, helper=True):
        with self.lock:
            if not self.stop_sent and self.proc.poll() is None:
                self.send(dict(Op="stop"))
                self.stop_sent = True
                self.proc.stdin.close()
            if helper and not self.closed_seen:
                if self.read(12).get("status") != "closed":
                    raise RuntimeError("private_close_ack_missing")
            self.proc.wait(timeout=12)
            if self.proc.returncode != 0:
                raise RuntimeError("private_join_nonzero_exit")

    def emergency(self):
        # A failed one-shot never emits a close acknowledgement. Join its
        # actual exit, but do not turn it into successful preparation or delete
        # its state. A still-running process remains a blocked teardown.
        if self.one_shot and self.failed_seen:
            if not self.proc.stdin.closed:
                self.proc.stdin.close()
            self.proc.wait(timeout=12)
            if self.proc.returncode == 0:
                raise RuntimeError("private_failed_one_shot_zero_exit")
            return
        # Legacy preparation calls this after an already joined one-shot child.
        # Everything else must still prove close acknowledgement + real exit.
        if self.one_shot and self.prepared_seen and self.proc.poll() == 0:
            return
        self.stop()
