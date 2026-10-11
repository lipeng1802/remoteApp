"""Private orchestration regressions, not actual Headscale/TLS acceptance."""
import copy
import contextlib
import io
import unittest
from types import SimpleNamespace
from unittest.mock import patch
from compiled_products import CompiledPair,verify_reconnect_authorization

class ReconnectTests(unittest.TestCase):
    def inputs(self):
        grant=dict(claims=dict(id="private-grant"),signature="private-signature")
        binding=dict(target_node="target",controller_node="controller")
        return {role:dict(Grant=copy.deepcopy(grant),Binding=copy.deepcopy(binding),Server=role=="target") for role in ("target","controller")},binding

    def apis(self,inputs,binding,events,failure=None):
        class API:
            def request(self,op,**kwargs):
                events.append(op)
                if op=="recheck_grant":
                    correct=kwargs["Grant"]==inputs["target"]["Grant"]
                    passed=correct
                    if failure=="negative": passed=True
                    if failure=="denied" and correct: passed=False
                    if failure=="numeric" and correct: passed=1
                    return dict(status=op,passed=passed)
                state=dict(active=True,binding=binding,grant=inputs["target"]["Grant"])
                if failure=="inactive": state["active"]=False
                if failure=="binding": state["binding"]={}
                if failure=="foreign": state["grant"]={}
                return dict(passed=True,network_state=dict(claims=state))
        return [API(),API()]

    def test_both_signed_rechecks_and_negative_precede_reopen(self):
        inputs,binding=self.inputs();events=[]
        verify_reconnect_authorization(self.apis(inputs,binding,events),inputs,binding)
        self.assertEqual(events,["recheck_grant","recheck_grant","network_state"]*2)

    def test_invalid_grant_or_binding_never_accepts(self):
        for failure in ("negative","denied","numeric","inactive","binding","foreign"):
            inputs,binding=self.inputs()
            with self.subTest(failure=failure),self.assertRaises(RuntimeError):
                verify_reconnect_authorization(self.apis(inputs,binding,[],failure),inputs,binding)
        inputs,binding=self.inputs();inputs["controller"]["Grant"]={}
        with self.assertRaises(RuntimeError):verify_reconnect_authorization([],inputs,binding)

    def pair(self,events,ack=None,authorization_failure=None):
        inputs,binding=self.inputs()
        class Proc:
            pid=1
            def poll(self):return 1
        class Target:
            def request(self,op):
                events.append(op)
                if op=="reopen_fault":return dict(status="owner_reopened",started=False) if ack is None else ack
                return dict(status="session_state",joined=False,fault=False)
        class Pair(CompiledPair):
            def assert_fault_latched(self):events.append("old_fault_latched")
            def maintenance(self,op):events.append(op)
            def _start_products(self,Pipe,values):
                events.append("new_products");self.sessions_joined=False
                self.controller=SimpleNamespace(proc=SimpleNamespace(pid=2))
            def stop_sessions(self):events.append("actual_normal_join");self.sessions_joined=True
        class T:
            def check(self,ok,label):
                if not ok:raise RuntimeError(label)
        pair=Pair(T(),"/private/tmp/test",r"C:\test",{},None)
        pair._reconnect_inputs=inputs;pair._retired_controller_proc=Proc();pair.target=Target();pair.sessions_joined=True
        return pair,self.apis(inputs,binding,events,authorization_failure),binding

    def test_reconnect_is_one_shot_fresh_idle_then_start_and_join(self):
        events=[];pair,apis,binding=self.pair(events)
        output=io.StringIO()
        with contextlib.redirect_stdout(output):pair.reconnect(None,apis,binding)
        self.assertLess(events.index("network_state"),events.index("reopen_fault"))
        self.assertLess(events.index("reopen_fault"),events.index("new_products"))
        self.assertEqual(events[-2:],["new_products","actual_normal_join"])
        self.assertNotIn("private-signature",output.getvalue())
        self.assertIsNone(pair._reconnect_inputs)
        with self.assertRaises(RuntimeError):pair.reconnect(None,apis,binding)

    def test_denial_never_reopens_or_launches_and_failure_cannot_retry(self):
        events=[];pair,apis,binding=self.pair(events,authorization_failure="inactive")
        with self.assertRaises(RuntimeError):pair.reconnect(None,apis,binding)
        self.assertNotIn("reopen_fault",events);self.assertNotIn("new_products",events)
        with self.assertRaises(RuntimeError):pair.reconnect(None,apis,binding)

    def test_ack_numeric_or_started_never_launches(self):
        for ack in (dict(status="owner_reopened",started=0),dict(status="owner_reopened",started=True),dict(status="ready",started=False)):
            events=[];pair,apis,binding=self.pair(events,ack=ack)
            with self.subTest(ack=ack),self.assertRaises(RuntimeError):pair.reconnect(None,apis,binding)
            self.assertNotIn("new_products",events)

    def test_old_process_must_have_actual_fault_exit_before_reopen(self):
        events=[];pair,apis,binding=self.pair(events)
        pair._retired_controller_proc=SimpleNamespace(poll=lambda:None)
        with self.assertRaises(RuntimeError):pair.reconnect(None,apis,binding)
        self.assertNotIn("recheck_grant",events);self.assertNotIn("reopen_fault",events)

    def test_target_send_failure_keeps_join_barrier_pending(self):
        pair=CompiledPair(None,"/private/tmp/test",r"C:\test",{},None)
        pair.sessions_joined=True
        pair.target=SimpleNamespace(send=lambda value:(_ for _ in ()).throw(RuntimeError("failed")))
        with self.assertRaises(RuntimeError):pair._start_products(None,dict(target={}))
        self.assertFalse(pair.sessions_joined)

    def test_reused_product_key_never_starts_mac(self):
        import base64,hashlib
        pair=CompiledPair(None,"/private/tmp/test",r"C:\test",{},None)
        pair._last_product_key_digest=hashlib.sha256(bytes(32)).digest()
        pair.target=SimpleNamespace(send=lambda value:None,read=lambda:dict(status="product_ready",Key=base64.b64encode(bytes(32)).decode(),Fingerprint="a"*64))
        launch=unittest.mock.Mock()
        with self.assertRaisesRegex(RuntimeError,"product_key_reused"):
            pair._start_products(launch,dict(target={}))
        launch.assert_not_called()
        self.assertFalse(pair.sessions_joined)

if __name__=="__main__":unittest.main()
