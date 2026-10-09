import base64
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

from compiled_products import CompiledPair, build_helpers, checked_borrower, observe, product_count, product_secret, verify_bundle
import hashlib


class CompiledProductTests(unittest.TestCase):
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

if __name__=="__main__":unittest.main()
