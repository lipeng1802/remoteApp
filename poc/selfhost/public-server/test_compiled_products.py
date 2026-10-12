import base64
import json
from pathlib import Path
import tempfile
import unittest
from types import SimpleNamespace
from unittest.mock import patch

from compiled_products import CompiledPair, build_helpers, checked_borrower, observe, product_count, product_secret, verify_bundle, upload_public_bundle, wait_borrower, session_state
import hashlib
import contextlib
import io


class CompiledProductTests(unittest.TestCase):
    def test_completion_state_is_exact_and_fault_requires_join(self):
        v=dict(status="session_state",joined=False,fault=False)
        self.assertEqual(session_state(v),(False,False))
        self.assertEqual(session_state(dict(v,joined=True,fault=True)),(True,True))
        self.assertEqual(session_state(dict(v,joined=True)),(True,False))
        for change in (dict(fault=True),dict(joined=1),dict(fault=0),dict(status="closed"),dict(error="private detail")):
            with self.subTest(change=change),self.assertRaises(RuntimeError):
                session_state(dict(v,**change))

    def test_readiness_budget_expired_does_not_query_or_accept(self):
        class Transport:
            def powershell(self, script): raise AssertionError("expired query")
        with patch("compiled_products.time.monotonic", return_value=601):
            with self.assertRaisesRegex(RuntimeError, "borrower_ready_deadline"):
                wait_borrower(Transport(), {}, "public", {}, 600)

    def test_public_response_after_deadline_is_not_accepted(self):
        class Transport:
            def powershell(self, script): return base64.b64encode(b'{}').decode()
        with patch("compiled_products.time.monotonic", side_effect=[599,601]), patch("compiled_products.checked_borrower") as checked:
            with self.assertRaisesRegex(RuntimeError, "borrower_ready_deadline"):
                wait_borrower(Transport(), {}, "public", {}, 600)
            checked.assert_not_called()

    def test_public_reuse_uploads_only_partition_and_verifies_manifest_itself(self):
        with tempfile.TemporaryDirectory() as directory:
            folder=Path(directory)
            contents={"helper.exe":b"shared-public", "entry.exe":b"fresh-public"}
            for name,data in contents.items(): (folder/name).write_bytes(data)
            manifest={name:hashlib.sha256(data).hexdigest() for name,data in contents.items()}
            (folder/"SHA256.json").write_text(json.dumps(manifest))
            hashes=dict(manifest,**{"SHA256.json":hashlib.sha256((folder/"SHA256.json").read_bytes()).hexdigest()})
            class Transport:
                def powershell(self,script):
                    if "FromBase64String" in script:
                        self_first=script
                        if "copy_hash_failed" not in self_first: raise AssertionError("unverified copy")
                        return json.dumps(dict(needed=["entry.exe","SHA256.json"],reused=["helper.exe"]))
                    return next(digest for name,digest in hashes.items() if "\\"+name+"'" in script)
                def check(self,ok,label):
                    if not ok: raise AssertionError(label)
            class Result:returncode=0
            with patch("compiled_products.subprocess.run",return_value=Result()) as run:
                upload_public_bundle(Transport(),folder,r"C:\fresh",r"C:\source")
                args=run.call_args.args[0]
                self.assertNotIn(str(folder/"helper.exe"),args)
                self.assertIn(str(folder/"entry.exe"),args)
                self.assertIn(str(folder/"SHA256.json"),args)
                self.assertIn("-C",args)
            class Invalid(Transport):
                def powershell(self,script): return json.dumps(dict(needed=["entry.exe"],reused=["entry.exe"]))
            with patch("compiled_products.subprocess.run") as run:
                with self.assertRaises(RuntimeError):upload_public_bundle(Invalid(),folder,r"C:\fresh",r"C:\source")
                run.assert_not_called()

    def test_borrower_pins_compiled_scope_not_only_public_identity(self):
        channel="desktop-test-"+"a"*32
        scope=hashlib.sha256(("remoteapp-installed-trust-channel-v1\n"+channel).encode()).hexdigest()[:32]
        folder=r"C:\test"
        identity=dict(VaultScope="c"*32,ExpectedPublic="b"*64)
        value=dict(status="ready",pipe=r"\\.\pipe\remoteapp-session-"+scope,scope=scope,identity_scope="c"*32,
                   public_key="b"*64,channel=channel,epoch=1,relay_executable=folder+r"\desktop-relay.exe")
        trace=dict(release=dict(test_namespace=True,channel=channel))
        self.assertEqual(checked_borrower(value,identity,folder,trace)["Scope"],scope)
        old=dict(value,scope="c"*32,pipe=r"\\.\pipe\remoteapp-session-"+"c"*32)
        with self.assertRaises(RuntimeError):checked_borrower(old,identity,folder,trace)
        with self.assertRaises(RuntimeError):checked_borrower(value,identity,folder,dict(release=dict(test_namespace=False,channel=channel)))
    def test_helpers_rebuilt_for_current_host_before_network(self):
        class Transport:
            ARTIFACTS=Path("/private/tmp/compiled-test")
            HERE=Path("/private/tmp/test-module")
            def check(self,ok,label):self.assertion=ok
        class Result:returncode=0
        for machine,want in (("x86_64","amd64"),("arm64","arm64")):
            with patch("compiled_products.platform.machine",return_value=machine),patch("compiled_products.subprocess.run",return_value=Result()) as run:
                build_helpers(Transport())
                self.assertEqual(run.call_count,9)
                packages={call.args[0][-1] for call in run.call_args_list}
                self.assertIn("./cmd/identity-vault",packages)
                self.assertIn("./cmd/trust-journal",packages)
                envs=[c.kwargs["env"] for c in run.call_args_list]
                self.assertEqual([e["GOARCH"] for e in envs if e["GOOS"]=="darwin"],[want,want])
                self.assertTrue(all(e["GOPROXY"]=="off" for e in envs))
    def test_private_ready_has_no_override_or_ambiguous_secret(self):
        v = dict(status="product_ready", Key=base64.b64encode(bytes(32)).decode(), Fingerprint="a"*64)
        self.assertEqual(product_secret(v), v["Key"])
        for change in ({"Root":"evil"}, {"Key":"bad"}, {"Fingerprint":"A"*64}, {"status":"ready"}):
            with self.assertRaises(Exception): product_secret(dict(v, **change))

    def test_count_is_live_role_specific_and_not_ready(self):
        v = dict(status="product_count", pongs=3, ended=False, pin_rejected=False)
        self.assertEqual(product_count(v), 3)
        self.assertEqual(product_count(dict(v,result="pending"),True),3)
        for change in ({"status":"ready"},{"pongs":True},{"pongs":-1},{"ended":True},
                       {"Key":"secret"},{"pin_rejected":True},{"result":"pending"}):
            with self.assertRaises(RuntimeError):product_count(dict(v,**change))
        for result in (None,"success","authentication_rejected"):
            with self.assertRaises(RuntimeError): product_count(dict(v,result=result),True)

    def test_payload_requires_both_growth_and_time_not_startup(self):
        class Clock:
            now=0
            def clock(self):return self.now
            def sleep(self,n):self.now+=n
        class Product:
            def __init__(self,clock,controller=False,stuck=False,regress=False):
                self.clock,self.controller,self.stuck,self.regress=clock,controller,stuck,regress
            def request(self,op):
                n=3 if self.stuck else 3+int(self.clock.now*5)
                if self.regress and self.clock.now>1:n=0
                v=dict(status="product_count",pongs=n,ended=False,pin_rejected=False)
                if self.controller:v["result"]="pending"
                return v
        clock=Clock();observe(Product(clock),Product(clock,True),clock.clock,clock.sleep)
        self.assertGreaterEqual(clock.now,5)
        for options in ({"stuck":True},{"regress":True}):
            clock=Clock()
            with self.assertRaises(RuntimeError):observe(Product(clock,**options),Product(clock,True),clock.clock,clock.sleep)

    def test_manifest_rejects_traversal_and_modified_file(self):
        with tempfile.TemporaryDirectory() as folder:
            path=Path(folder)
            (path/"SHA256.json").write_text(json.dumps({"../outside":"a"*64}))
            with self.assertRaises(RuntimeError):verify_bundle(path)
            (path/"tool").write_bytes(b"modified")
            (path/"SHA256.json").write_text(json.dumps({"tool":"a"*64}))
            with self.assertRaises(RuntimeError):verify_bundle(path)

    def pair(self,events):
        class Transport:
            def check(self,ok,label):
                if not ok:raise RuntimeError(label)
        class Identities:
            controller=dict(VaultScope="b"*32,ExpectedPublic="b"*64)
        class Pair(CompiledPair):
            def maintenance(self,op):events.append(op)
        return Pair(Transport(),"/private/tmp/test",r"C:\test",{},Identities())

    def test_bound_handoff_before_native_initialize_and_both_normal_join(self):
        events=[];pair=self.pair(events)
        class Target:
            replies=[dict(status="product_ready",Key=base64.b64encode(bytes(32)).decode(),Fingerprint="a"*64),dict(status="ready",port=0)]
            def send(self,v):events.append("target_start")
            def read(self):return self.replies.pop(0)
            def request(self,op):events.append(op);return dict(status="session_closed")
        class Controller:
            proc=SimpleNamespace(pid=1,poll=lambda:1)
            def read(self):return dict(status="ready",port=12345)
            def stop(self):events.append("controller_joined")
        class Handoff:
            def take(self):events.append("old_nodes_joined");return dict(target={},controller={})
        pair.target=Target()
        def launch(cmd,cfg):
            self.assertEqual(cfg["VaultExpected"],"b"*64)
            self.assertNotIn("Endpoint",cfg)
            events.append("controller_start");return Controller()
        with patch("compiled_products.observe",side_effect=lambda a,b:events.append("actual_growth")):
            pair.run(launch,Handoff())
        events.append("owner_revoke")
        self.assertEqual(events,["old_nodes_joined","initialize","target_start","controller_start","actual_growth","controller_joined","stop_session","owner_revoke"])
        self.assertTrue(pair.sessions_joined)

    def test_unconfirmed_join_prevents_native_cleanup(self):
        events=[];pair=self.pair(events);pair.mac_initialized=True
        class BadController:
            def stop(self):raise RuntimeError("unjoined")
        pair.controller=BadController()
        with self.assertRaises(RuntimeError):pair.close()
        self.assertEqual(events,[])
        self.assertFalse(pair.sessions_joined)
        self.assertTrue(pair.mac_initialized)

    def test_live_revocation_never_sends_stop_before_autonomous_join(self):
        events=[];pair=self.pair(events)
        class Target:
            replies=[dict(status="product_ready",Key=base64.b64encode(bytes(32)).decode(),Fingerprint="a"*64),dict(status="ready",port=0)]
            joined=False
            def send(self,v):events.append("target_start")
            def read(self):return self.replies.pop(0)
            def request(self,op):
                events.append(op)
                if op=="session_state":return dict(status="session_state",joined=self.joined,fault=self.joined)
                if op=="retire_fault":return dict(status="fault_retired",joined=True,fault=True)
                raise AssertionError("stop forbidden")
        class Controller:
            def read(self):return dict(status="ready",port=12345)
            def request(self,op):return dict(status="session_state",joined=False,fault=False)
            def stop(self):raise AssertionError("stop forbidden")
            proc=SimpleNamespace(pid=1,poll=lambda:1)
            def join_fault(self):events.append("controller_fault_joined")
        class Handoff:
            def take(self):return dict(target={},controller={})
        pair.target=Target()
        with patch("compiled_products.observe"):
            pair.run(lambda cmd,cfg:Controller(),Handoff(),live_revoke=True)
        self.assertFalse(pair.sessions_joined)
        events.append("owner_signed_revoke")
        pair.target.joined=True
        with patch("compiled_products.time.sleep"):
            pair.revoked()
        self.assertTrue(pair.sessions_joined)
        self.assertLess(events.index("owner_signed_revoke"),events.index("controller_fault_joined"))
        self.assertLess(events.index("controller_fault_joined"),events.index("retire_fault"))
        self.assertEqual(events[-1],"inspect")

    def test_unconfirmed_fault_retirement_prevents_cleanup(self):
        events=[];pair=self.pair(events)
        class Controller:
            def join_fault(self):raise RuntimeError("missing retirement")
        pair.controller=Controller()
        with self.assertRaises(RuntimeError):pair.revoked()
        self.assertFalse(pair.sessions_joined)
        self.assertIsNotNone(pair.controller)
        self.assertEqual(events,[])

    def test_fault_recovery_requires_retired_owner_and_absent_controller(self):
        events=[];pair=self.pair(events)
        class Target:
            state=dict(status="session_state",joined=True,fault=True)
            def request(self,op):
                if op!="session_state":raise AssertionError("restart forbidden")
                return self.state
        pair.target=Target()
        with self.assertRaises(RuntimeError):pair.assert_fault_latched()
        pair.sessions_joined=True
        pair.controller=object()
        with self.assertRaises(RuntimeError):pair.assert_fault_latched()
        pair.controller=None
        pair.assert_fault_latched()
        self.assertEqual(events,["inspect"])
        pair.target.state=dict(status="session_state",joined=False,fault=False)
        with self.assertRaises(RuntimeError):pair.assert_fault_latched()

    def test_conflicting_modes_fail_before_consuming_handoff(self):
        pair=self.pair([])
        with self.assertRaisesRegex(RuntimeError,"conflicting_live_modes"):
            pair.run(None,None,live_revoke=True,authority_fault=True)

    def test_handoff_failure_is_distinct_and_blocks_native_initialize(self):
        events=[];pair=self.pair(events);out=io.StringIO()
        class Handoff:
            def take(self):raise RuntimeError("private detail")
        with contextlib.redirect_stdout(out),self.assertRaises(RuntimeError):
            pair.run(None,Handoff())
        self.assertEqual(events,[])
        self.assertFalse(pair.mac_initialized)
        self.assertIn("coordination_bound_handoff_failed",out.getvalue())
        self.assertNotIn("mac_native_initialize",out.getvalue())
        self.assertNotIn("private",out.getvalue())

    def test_target_failure_after_handoff_is_not_named_helper_join(self):
        events=[];pair=self.pair(events);out=io.StringIO()
        class Handoff:
            def take(self):return dict(target={},controller={})
        class Target:
            def send(self,v):pass
            def read(self):raise RuntimeError("private detail")
        pair.target=Target()
        with contextlib.redirect_stdout(out),self.assertRaises(RuntimeError):
            pair.run(None,Handoff())
        self.assertTrue(pair.mac_initialized)
        self.assertEqual(events,["initialize"])
        self.assertIn("coordination_bound_handoff_completed",out.getvalue())
        self.assertIn("coordination_target_product_ready_failed",out.getvalue())
        self.assertNotIn("controller_start",out.getvalue())
        self.assertNotIn("private",out.getvalue())

if __name__=="__main__":unittest.main()
