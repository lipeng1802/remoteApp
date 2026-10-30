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


def verified_fault_receipt(value, status):
    # Python dictionary equality alone accepts 1 as True. Completion evidence
    # must be exact-schema JSON with actual booleans, not truthy substitutes.
    return (isinstance(value, dict) and set(value) == {"status", "joined", "fault"}
            and value["status"] == status and type(value["joined"]) is bool
            and type(value["fault"]) is bool and value["joined"] and value["fault"])


class JoinedPipe:
    def __init__(self, command, config=None, cwd=None):
        self.lock = threading.Lock()
        self.buffer = bytearray()
        self.closed_seen = self.stop_sent = False
        self.prepared_seen = False
        self.failed_seen = False
        self.fault_joined = False
        self.fault_stop_sent = False
        self.fault_ready_seen = self.fault_closed_seen = False
        self.one_shot = isinstance(config, dict) and config.get("Op") == "prepare"
        self.join_stages = []
        self.diagnostics_done = threading.Event()
        self.proc = subprocess.Popen(command, cwd=cwd, stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                                     stderr=subprocess.PIPE, bufsize=0)
        threading.Thread(target=self._drain_diagnostics, daemon=True).start()
        if config is not None:
            self.send(config)

    def _drain_diagnostics(self):
        allowed = {b"product_stopping", b"product_joined", b"watch_stopping", b"watch_joined",
                   b"workers_joined", b"node_closing", b"node_closed", b"signer_closing", b"signer_closed"}
        line, oversized = bytearray(), False
        try:
            while True:
                raw = self.proc.stderr.read(1)
                if not raw:
                    break
                if raw == b"\n":
                    if not oversized and line.startswith(b"JOIN_STAGE ") and bytes(line[11:]) in allowed:
                        self.join_stages.append(bytes(line[11:]).decode("ascii"))
                        self.join_stages[:] = self.join_stages[-16:]
                    line.clear(); oversized = False
                elif len(line) < 128 and not oversized:
                    line.extend(raw)
                else:
                    line.clear(); oversized = True
        finally:
            self.proc.stderr.close()
            self.diagnostics_done.set()

    def _join_diagnostics(self):
        done = getattr(self, "diagnostics_done", None)
        if done is not None and not done.wait(2):
            raise RuntimeError("private_diagnostics_join_unconfirmed")

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

    def read(self, timeout=40, allow_exact_failure=False):
        deadline = time.monotonic() + timeout
        while b"\n" not in self.buffer:
            left = deadline - time.monotonic()
            if left <= 0 or not select.select([self.proc.stdout], [], [], left)[0]:
                stage = self.join_stages[-1] if self.join_stages else "not_reported"
                print("DIAGNOSTIC latest_join_stage_" + stage, flush=True)
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
                if allow_exact_failure and value == {"status":"failed"}:
                    return value
                stage = value.get("failure_stage")
                if stage not in ("signer_open", "node_state", "intent_sign", "protocol", "product_tls_publish", "product_certificate_open"):
                    stage = "unknown"
                print("DIAGNOSTIC helper_" + stage, flush=True)
            raise RuntimeError("private_helper_failed")
        if value.get("status") == "closed":
            self.closed_seen = True
        if value.get("status") == "prepared":
            self.prepared_seen = True
        return value

    def join_fault(self):
        # Dedicated expected-fault protocol. It is never a normal close ACK;
        # require nested join, successful journal retirement AND real exit 1.
        with self.lock:
            print("DIAGNOSTIC fault_wait_completion", flush=True)
            if not verified_fault_receipt(self.read(20), "session_state"):
                raise RuntimeError("private_fault_completion_missing")
            print("DIAGNOSTIC fault_completion_verified_wait_retirement", flush=True)
            if not verified_fault_receipt(self.read(12), "fault_retired"):
                raise RuntimeError("private_fault_retirement_missing")
            # These two receipts prove nested runner join and native journal
            # retirement. EOF now joins ONLY the command reader; it does not
            # stop a live session or substitute for either receipt. Inherited
            # blocking stdin cannot reliably be interrupted by child Close.
            if not self.proc.stdin.closed:
                self.proc.stdin.close()
            print("DIAGNOSTIC fault_retirement_verified_wait_terminal", flush=True)
            if self.read(12, allow_exact_failure=True) != {"status":"failed"}:
                raise RuntimeError("private_fault_terminal_missing")
            self.proc.wait(timeout=12)
            self._join_diagnostics()
            if self.proc.returncode != 1:
                raise RuntimeError("private_fault_exit_mismatch")
            self.fault_joined = True

    def request(self, op, **values):
        with self.lock:
            self.send(dict(Op=op, **values))
            return self.read()

    def join_upgraded(self):
        # Upgrade already quiesced the runner. EOF only releases the command
        # reader; never send a second stop or confuse its ACK with upgrade.
        with self.lock:
            if not self.proc.stdin.closed:
                self.proc.stdin.close()
            if self.read(12) != {"status":"closed"}:
                raise RuntimeError("private_upgrade_close_missing")
            self.proc.wait(timeout=12)
            self._join_diagnostics()
            if self.proc.returncode != 0:
                raise RuntimeError("private_upgrade_exit_mismatch")

    def stop(self, helper=True):
        if getattr(self,'fault_stop_sent',False):
            raise RuntimeError('private_fault_lifecycle_not_normal_close')
        with self.lock:
            if not self.stop_sent and self.proc.poll() is None:
                self.send(dict(Op="stop"))
                self.stop_sent = True
                self.proc.stdin.close()
            if helper and not self.closed_seen:
                if self.read(12).get("status") != "closed":
                    raise RuntimeError("private_close_ack_missing")
            self.proc.wait(timeout=12)
            self._join_diagnostics()
            if self.proc.returncode != 0:
                raise RuntimeError("private_join_nonzero_exit")

    def stop_fault(self):
        # Private TEST lifecycle only. A rejected/unknown revocation check must
        # leave stdin open: EOF would otherwise trigger generic signer release.
        with self.lock:
            if self.fault_joined:return
            if self.stop_sent:raise RuntimeError("private_normal_stop_not_fault_protocol")
            if not self.fault_stop_sent:
                self.send({'Op':'stop_fault'});self.fault_stop_sent=True
            if not self.fault_ready_seen:
                ready=self.read(25)
                if ready=={'status':'fault_stop_rejected'}:
                    self.fault_stop_sent=False
                    raise RuntimeError('private_fault_revocation_unconfirmed')
                if not verified_fault_receipt(ready,'fault_stop_ready'):
                    raise RuntimeError('private_fault_ready_unconfirmed')
                self.fault_ready_seen=True
                self.proc.stdin.close()
            if not self.fault_closed_seen:
                if not verified_fault_receipt(self.read(12),'fault_closed'):
                    raise RuntimeError('private_fault_terminal_unconfirmed')
                self.fault_closed_seen=True
            self.proc.wait(timeout=12);self._join_diagnostics()
            if self.buffer or not select.select([self.proc.stdout],[],[],12)[0] or os.read(self.proc.stdout.fileno(),1)!=b'':
                raise RuntimeError('private_fault_eof_unconfirmed')
            if self.proc.returncode!=1:raise RuntimeError('private_fault_exit_mismatch')
            self.fault_joined=True

    def emergency(self):
        if getattr(self, "fault_joined", False):
            self.proc.wait(timeout=12)
            self._join_diagnostics()
            if self.proc.returncode != 1:
                raise RuntimeError("private_fault_exit_mismatch")
            return
        # A failed one-shot never emits a close acknowledgement. Join its
        # actual exit, but do not turn it into successful preparation or delete
        # its state. A still-running process remains a blocked teardown.
        if self.one_shot and self.failed_seen:
            if not self.proc.stdin.closed:
                self.proc.stdin.close()
            self.proc.wait(timeout=12)
            self._join_diagnostics()
            if self.proc.returncode == 0:
                raise RuntimeError("private_failed_one_shot_zero_exit")
            return
        # Legacy preparation calls this after an already joined one-shot child.
        # Everything else must still prove close acknowledgement + real exit.
        if self.one_shot and self.prepared_seen and self.proc.poll() == 0:
            return
        self.stop()
