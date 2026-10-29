import subprocess
import unittest
from unittest.mock import patch
from supervised_requests import WindowsRelay, reply


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


if __name__ == "__main__": unittest.main()
