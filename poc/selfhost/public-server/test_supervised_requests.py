import subprocess
import os
from pathlib import Path
import socket
import tempfile
import threading
import unittest
from unittest.mock import patch
from supervised_requests import LocalSupervisor, NativeUnix, WindowsRelay, reply


class PrivateTransportBoundaries(unittest.TestCase):
    def adapter(self):
        return WindowsRelay("jarvis@100.73.4.118", r"C:\Users\jarvis\remoteapp-test\supervised-control-relay.exe",
                            "\\\\.\\pipe\\remoteapp-session-" + "a"*32)

    def test_secret_request_uses_stdin_only(self):
        adapter = self.adapter()
        with patch("supervised_requests.subprocess.run", return_value=subprocess.CompletedProcess([], 0, b'{"status":"device_result","passed":true}\n', b"")) as run:
            value = adapter({"Op":"device", "Device":{"action":"claim", "code":"PRIVATE-INVITE"}})
        self.assertTrue(value["passed"])
        self.assertNotIn("PRIVATE-INVITE", str(run.call_args.args))
        self.assertIn(b"PRIVATE-INVITE", run.call_args.kwargs["input"])

    def test_timeout_is_unknown_never_joined_or_retried(self):
        with patch("supervised_requests.subprocess.run", side_effect=subprocess.TimeoutExpired([], 35)) as run:
            with self.assertRaisesRegex(RuntimeError, "outcome_unknown"): self.adapter()({"Op":"device"})
        self.assertEqual(run.call_count, 1)

    def test_reply_rejects_duplicate_extra_frame_nonobject_and_oversize(self):
        for raw in (b'{"passed":true,"PASSED":false}', b'{}\n{}', b'[]', b'x'*8193, b'\xff'):
            with self.subTest(raw=raw[:30]):
                with self.assertRaises(RuntimeError): reply(raw)

    def test_untrusted_selectors_cannot_become_shell_commands(self):
        for host, exe, pipe in (("-oProxyCommand=bad", r"C:\a.exe", "bad"),
                                ("host", r"C:\a.exe & bad", "bad"),
                                ("host", r"C:\a.exe", "\\\\remote\\pipe\\other")):
            with self.assertRaises(RuntimeError): WindowsRelay(host, exe, pipe)

    def test_local_timeout_drains_old_reply_before_sending_new_operation(self):
        class Pipe:
            lock = threading.Lock()
            def __init__(self):
                self.sent = []
                self.replies = [RuntimeError("private_pipe_timeout"),
                                {"status":"PRIVATE-OLD-APPROVAL"}, {"status":"session_revoked","passed":True}]
            def send(self, frame): self.sent.append(frame)
            def read(self, **unused):
                if len(self.sent) == 2: assert len(self.replies) == 1
                value = self.replies.pop(0)
                if isinstance(value,Exception): raise value
                return value
        pipe=Pipe()
        endpoint=LocalSupervisor(pipe)
        with self.assertRaisesRegex(RuntimeError,"supervised_local_outcome_unknown"):
            endpoint({"Op":"provision"})
        self.assertEqual(endpoint({"Op":"revoke_session"}), {"status":"session_revoked","passed":True})
        self.assertEqual(pipe.sent,[{"Op":"provision"},{"Op":"revoke_session"}])
        self.assertFalse(endpoint.pending)

    def test_local_malformed_response_seals_without_releasing_or_resending(self):
        class Pipe:
            lock=threading.Lock()
            def __init__(self): self.sent=0
            def send(self, frame): self.sent+=1
            def read(self, **unused): raise ValueError("PRIVATE malformed response")
        pipe=Pipe()
        endpoint=LocalSupervisor(pipe)
        with self.assertRaisesRegex(RuntimeError,"supervised_local_outcome_unknown"):
            endpoint({"Op":"provision"})
        with self.assertRaisesRegex(RuntimeError,"pipe_sealed_preserve_owner"):
            endpoint({"Op":"revoke_session"})
        self.assertEqual(pipe.sent,1)

    def test_native_unix_fresh_connection_after_lost_or_invalid_reply(self):
        with tempfile.TemporaryDirectory(prefix="ra-uds-",dir="/tmp") as raw:
            directory=Path(raw).resolve()
            address=directory/"control.sock"
            listener=socket.socket(socket.AF_UNIX,socket.SOCK_STREAM)
            listener.bind(str(address)); os.chmod(address,0o600); listener.listen(4)
            requests=[]
            def serve():
                try:
                    for response in (b'{}\n{}\n',b'{"status":"session_revoked","passed":true}\n'):
                        connection,unused=listener.accept()
                        with connection:
                            requests.append(connection.recv(8192))
                            connection.sendall(response)
                finally: listener.close()
            thread=threading.Thread(target=serve,daemon=True)
            thread.start()
            endpoint=NativeUnix(str(address))
            try:
                with self.assertRaisesRegex(RuntimeError,"outcome_unknown_preserve_owner"):
                    endpoint({"Op":"revoke_session"})
                self.assertEqual(endpoint({"Op":"revoke_session"}),{"status":"session_revoked","passed":True})
                self.assertEqual(requests,[b'{"Op":"revoke_session"}\n']*2)
            finally:
                thread.join(timeout=3)
            self.assertFalse(thread.is_alive())

    def test_native_unix_permissions_and_replacement_identity_rejected(self):
        with tempfile.TemporaryDirectory(prefix="ra-uds-",dir="/tmp") as raw:
            directory=Path(raw).resolve()
            address=directory/"control.sock"
            listener=socket.socket(socket.AF_UNIX,socket.SOCK_STREAM)
            listener.bind(str(address)); os.chmod(address,0o600)
            try:
                endpoint=NativeUnix(str(address))
                os.chmod(directory,0o755)
                with self.assertRaisesRegex(RuntimeError,"permissions_rejected"):endpoint({"Op":"revoke_session"})
                os.chmod(directory,0o700)
                os.rename(address,directory/"original.sock")
                replacement=socket.socket(socket.AF_UNIX,socket.SOCK_STREAM)
                try:
                    replacement.bind(str(address)); os.chmod(address,0o600)
                    with self.assertRaisesRegex(RuntimeError,"identity_changed"):endpoint({"Op":"revoke_session"})
                finally: replacement.close()
            finally: listener.close()


if __name__ == "__main__": unittest.main()
