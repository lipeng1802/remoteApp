"""Upgrade orchestration fixtures, NOT native/live network acceptance."""
import json,tempfile,unittest
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import patch
from compiled_products import CompiledPair,epoch_receipt

class UpgradeTests(unittest.TestCase):
    def test_epoch_receipt_rejects_numeric_bool_extra_or_wrong_state(self):
        self.assertTrue(epoch_receipt(dict(status="upgraded",epoch=2),"upgraded",2))
        for value in (2.0,True,"2",None,1):
            self.assertFalse(epoch_receipt(dict(status="upgraded",epoch=value),"upgraded",2))
        self.assertFalse(epoch_receipt(dict(status="upgraded",epoch=2,extra="private"),"upgraded",2))

    def exercise(self,failure=None):
        events=[]
        with tempfile.TemporaryDirectory() as directory:
            parent=Path(directory);mac=parent/"mac";mac.mkdir()
            for name,epoch in (("target-profile",1),("target-upgrade",2),("controller-upgrade",2)):
                (parent/(name+".json")).write_text(json.dumps(dict(claims=dict(epoch=epoch),signature="valid")))
            old=SimpleNamespace(pid=10,poll=lambda:0 if "controller_join" in events else None)
            class Peer:
                def __init__(self,role):self.role=role;self.proc=old;self.upgraded=False
                def request(self,op,**kwargs):
                    events.append(self.role+"_"+op)
                    if op=="session_state":return dict(status=op,joined=self.upgraded,fault=False)
                    if op=="installation_state":return dict(status=op,epoch=2)
                    p=kwargs["Profile"]
                    if p["signature"]=="invalid" or p["claims"]["epoch"]==1:
                        return dict(status="failed" if failure=="candidate" else "upgrade_rejected")
                    self.upgraded=True
                    return dict(status="upgraded",epoch=2.0 if failure=="ack" else 2)
                def join_upgraded(self):
                    if failure=="join":raise RuntimeError("join_failed")
                    events.append("controller_join")
            class Pair(CompiledPair):
                def maintenance(self,op,expected_epoch=None):events.append((op,expected_epoch));
                def _start_products(self,Pipe,inputs):
                    if "authorization" not in events or inputs["target"].get("ReuseProductCertificate") is not True:raise RuntimeError("early_start")
                    events.append("start_new");self.controller=SimpleNamespace(proc=SimpleNamespace(pid=11))
                def stop_sessions(self):events.append("normal_join");self.sessions_joined=True
            class T:
                def check(self,ok,label):
                    if not ok:raise RuntimeError(label)
            p=Pair(T(),mac,"public",{},None);p.target=Peer("target");p.controller=Peer("controller")
            with patch("compiled_products.observe",side_effect=lambda *args:events.append("live_payload")),patch("compiled_products.verify_reconnect_authorization",side_effect=lambda *args:events.append("authorization")),patch("compiled_products.subprocess.run",return_value=SimpleNamespace(returncode=1,stdout=b'{"status":"failed","reason":"trust_rejected"}')):
                if failure:
                    with self.assertRaises(RuntimeError):p.upgrade_round(None,{"target":{},"controller":{}},[],{})
                    self.assertNotIn("start_new",events)
                else:
                    p.upgrade_round(None,{"target":{},"controller":{}},[],{})
                    self.assertLess(events.index("controller_join"),events.index("target_installation_state"))
                    self.assertLess(events.index("authorization"),events.index("start_new"))
                    self.assertEqual(events[-1],"normal_join")
                    self.assertEqual(p._maintenance_epoch,2)

    def test_upgrade_requires_join_native_floor_and_reauthorization_before_restart(self):self.exercise()
    def test_candidate_ack_or_join_failure_never_starts_new_product(self):
        for failure in ("candidate","ack","join"):
            with self.subTest(failure=failure):self.exercise(failure)

if __name__=="__main__":unittest.main()
