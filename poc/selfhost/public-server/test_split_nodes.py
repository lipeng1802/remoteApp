import hashlib
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

from split_nodes import upload_helper, checked_forward_probe, checked_route_status, wait_overlay_route
import contextlib
import io


class NodeUploadTests(unittest.TestCase):
    def route(self,ready=True):
        return dict(status="route_status",query_ok=True,running=True,peer_known=ready,ip_matches=ready,
                    in_network_map=ready,allowed_ip=ready,in_engine=False,ready=ready)

    def test_route_exact_schema_and_boolean_consistency(self):
        self.assertTrue(checked_route_status(self.route())["ready"])
        for change in (dict(peer_known=1),dict(ready=False),dict(allowed_ip=False),dict(extra="private")):
            with self.assertRaises(RuntimeError):checked_route_status(dict(self.route(),**change))

    def test_route_wait_only_queries_metadata_not_payload(self):
        calls=[];values=[self.route(False),self.route()]
        class Peer:
            def request(self,op,**args):calls.append(op);return values.pop(0)
        class T:
            def check(self,ok,label):assert ok
        with patch("split_nodes.time.monotonic",return_value=1),patch("split_nodes.time.sleep"):
            wait_overlay_route(T(),Peer(),"test-ip","test-node",12)
        self.assertEqual(calls,["route_status","route_status"])

    def test_route_reply_after_existing_deadline_is_not_ready(self):
        value=self.route()
        class Peer:
            def request(self,*args,**kwargs):return value
        with patch("split_nodes.time.monotonic",side_effect=[1,12]):
            with self.assertRaises(RuntimeError):wait_overlay_route(None,Peer(),"ip","node",12)

    def test_expired_route_budget_makes_no_query(self):
        class Peer:
            def request(self,*args,**kwargs):raise AssertionError("expired query")
        with patch("split_nodes.time.monotonic",return_value=12):
            with self.assertRaises(RuntimeError):wait_overlay_route(None,Peer(),"ip","node",12)

    def test_forward_requires_exact_payload_and_only_emits_fixed_diagnostics(self):
        good=dict(status="probe",passed=True,reason="none",stage="payload_verified")
        output=io.StringIO()
        with contextlib.redirect_stdout(output):
            self.assertTrue(checked_forward_probe(good))
            for change in (dict(passed=1),dict(passed=False),dict(stage="read_failed"),
                           dict(reason="dial_denied"),dict(extra="secret"),dict(stage="secret",reason="secret"),
                           dict(stage={"secret":1},reason=["secret"])):
                self.assertFalse(checked_forward_probe(dict(good,**change)))
        self.assertNotIn("secret",output.getvalue())
        self.assertIn("forward_probe_stage_unknown_reason_unknown",output.getvalue())

    def test_compressed_bounded_transfer_then_hash_check(self):
        with tempfile.TemporaryDirectory() as directory:
            binary=Path(directory)/"enrollment-helper-win.exe"
            binary.write_bytes(b"public helper")
            class Transport:
                ARTIFACTS=Path(directory)
                def powershell(self, script): return hashlib.sha256(binary.read_bytes()).hexdigest()
                def check(self, ok, label):
                    if not ok: raise RuntimeError(label)
            class Result: returncode=0
            with patch("split_nodes.subprocess.run", return_value=Result()) as run:
                upload_helper(Transport(), "C:\\isolated")
                self.assertIn("-C",run.call_args.args[0])
                self.assertEqual(run.call_args.kwargs["timeout"],180)
            Transport.powershell=lambda self, script: "0"*64
            with patch("split_nodes.subprocess.run", return_value=Result()):
                with self.assertRaisesRegex(RuntimeError,"split_node_helper_binary_verified"):
                    upload_helper(Transport(), "C:\\isolated")
