import base64
import copy
import importlib.util
from pathlib import Path
import unittest
import sys

spec=importlib.util.spec_from_file_location("split_consent",Path(__file__).with_name("split-consent.py"))
split=importlib.util.module_from_spec(spec)
spec.loader.exec_module(split)

class SplitConsentTests(unittest.TestCase):
    def test_live_compiled_mode_is_explicit_and_not_authority_fault(self):
        self.assertEqual(split.mode_flags(["--compiled-revoke"]),(True,True,False,False,True))
        self.assertEqual(split.mode_flags(["--compiled"]),(True,False,False,False,True))
        self.assertEqual(split.mode_flags(["--tls-fault"]),(False,False,True,True,True))
        self.assertEqual(split.mode_flags(["--compiled-fault"]),(True,False,True,False,True))
        self.assertEqual(split.mode_flags(["--compiled-reconnect"]),(True,False,True,False,True))
        for args in (["--compiled-revoke","--tls-fault"],["--compiled-revoke","--compiled"],["--compiled-fault","--compiled"]):
            with self.assertRaises(RuntimeError):split.mode_flags(args)

    def test_remote_cleanup_requires_authoritative_marker_not_ssh_exit(self):
        for text, code, accepted in [("",0,False),
                ("STOPPED isolated_coordinator_resources_joined extra",0,False),
                ("STOPPED isolated_coordinator_resources_joined",0,True),
                ("STOPPED isolated_coordinator_resources_joined",1,True)]:
            with self.subTest(text=text,code=code):
                child=split.Coordinator([sys.executable,"-c",
                    "import sys; sys.stdin.read(); print(sys.argv[1]); sys.exit(int(sys.argv[2]))",text,str(code)])
                try:
                    if accepted:
                        self.assertEqual(child.join(),code)
                    else:
                        with self.assertRaises(RuntimeError): child.join()
                finally:
                    child.proc.wait(timeout=2)
                    child.proc.stdout.close()

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
        fault_ready=dict(node_ready,fault_phase=True)
        self.assertEqual(split.checked_ready(fault_ready,"a"*64,"b"*64,True,True),fault_ready)
        with self.assertRaises(Exception): split.checked_ready(node_ready,"a"*64,"b"*64,True,True)
        with self.assertRaises(Exception): split.checked_ready(fault_ready,"a"*64,"b"*64,True)
        compiled_ready=dict(node_ready,compiled_phase=True)
        self.assertEqual(split.checked_ready(compiled_ready,"a"*64,"b"*64,True,False,True),compiled_ready)
        combined=dict(compiled_ready,fault_phase=True)
        self.assertEqual(split.checked_ready(combined,"a"*64,"b"*64,True,True,True),combined)
        for marker in ("fault_phase","compiled_phase"):
            bad=dict(combined);bad.pop(marker)
            with self.assertRaises(Exception):split.checked_ready(bad,"a"*64,"b"*64,True,True,True)
            with self.assertRaises(Exception):split.checked_ready(dict(combined,**{marker:1}),"a"*64,"b"*64,True,True,True)
        for n,f,c in [(False,False,True),(True,True,True),(True,False,False)]:
            with self.assertRaises(Exception): split.checked_ready(compiled_ready,"a"*64,"b"*64,n,f,c)

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
        bad_owner=copy.deepcopy(owner)
        bad_owner[-1]={"passed":False,"failure_stage":"signer"}
        with self.assertRaises(ValueError):
            split.consent(Client(bad_owner),Client(copy.deepcopy(peer)),node_phase=failed_phase)
        # A secondary revoke failure must not obscure the first node error.
        bad=copy.deepcopy(peer); bad[3]={"passed":True}
        with self.assertRaises(RuntimeError): split.consent(Client(copy.deepcopy(owner)),Client(bad))
        bad=copy.deepcopy(peer); bad[2]["device"]["grant"]=grant
        with self.assertRaises(RuntimeError): split.consent(Client(copy.deepcopy(owner)),Client(bad))

if __name__=="__main__": unittest.main()
