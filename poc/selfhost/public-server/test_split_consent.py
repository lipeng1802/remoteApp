import base64
import copy
import importlib.util
from pathlib import Path
import unittest

spec=importlib.util.spec_from_file_location("split_consent",Path(__file__).with_name("split-consent.py"))
split=importlib.util.module_from_spec(spec)
spec.loader.exec_module(split)

class SplitConsentTests(unittest.TestCase):
    def test_bootstrap_is_exact_public_split_marker(self):
        ready={"status":"ready","split_process":True,"target":"a"*64,"controller":"b"*64,
               "issuer":"c"*64,"api_certificate":base64.b64encode(b"public-ca").decode(),"api_port":12345}
        self.assertEqual(split.checked_ready(ready,"a"*64,"b"*64),ready)
        changes=[{"split_process":False},{"target_key":"secret"},{"grant":{}},{"api_port":True},
                 {"api_port":65536},{"issuer":"C"*64},{"target":"d"*64},{"api_certificate":"%%%"}]
        for change in changes:
            with self.subTest(change=change),self.assertRaises(Exception):
                split.checked_ready(dict(ready,**change),"a"*64,"b"*64)
        node_ready=dict(ready,node_phase=True,verifier="nodekey:"+"d"*64)
        self.assertEqual(split.checked_ready(node_ready,"a"*64,"b"*64,True),node_ready)
        with self.assertRaises(Exception): split.checked_ready(ready,"a"*64,"b"*64,True)
        with self.assertRaises(Exception): split.checked_ready(node_ready,"a"*64,"b"*64)

    def test_consent_never_accepts_self_approval_or_auto_grant(self):
        class Client:
            def __init__(self, answers): self.answers=list(answers)
            def request(self,*args,**kwargs):
                if not self.answers: raise AssertionError("unexpected extra request")
                return self.answers.pop(0)
        def device(**body): return {"passed":True,"device":body}
        grant={"claims":{"id":"e"*32},"signature":"test-only"}
        owner=[device(device_id="owner",device_code="123"),{"passed":False},device(code="one"),
               device(request_id="r1",controller_id="peer"),device(status="denied"),device(code="two"),
               device(grant=grant),{"passed":True},{"passed":False},device(status="revoked")]
        peer=[device(device_id="peer"),{"passed":False},device(status="pending",request_id="r1"),
              {"passed":False},device(status="denied"),device(status="pending",request_id="r2"),
              device(grant=grant),{"passed":True},{"passed":False},device(status="revoked")]
        a,b=Client(copy.deepcopy(owner)),Client(copy.deepcopy(peer))
        split.consent(a,b)
        self.assertFalse(a.answers or b.answers)
        a,b=Client(copy.deepcopy(owner)),Client(copy.deepcopy(peer))
        def failed_phase(grant): raise ValueError("phase_failed")
        with self.assertRaises(ValueError): split.consent(a,b,node_phase=failed_phase)
        self.assertFalse(a.answers)  # Owner revoke attempted even on phase error.
        bad=copy.deepcopy(peer); bad[3]={"passed":True}
        with self.assertRaises(RuntimeError): split.consent(Client(copy.deepcopy(owner)),Client(bad))
        bad=copy.deepcopy(peer); bad[2]["device"]["grant"]=grant
        with self.assertRaises(RuntimeError): split.consent(Client(copy.deepcopy(owner)),Client(bad))

if __name__=="__main__": unittest.main()
