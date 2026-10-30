import json
from pathlib import Path
import sys
import tempfile
import threading
import time
import unittest

from joined_pipe import JoinedPipe
from persistent_supervisor import Endpoint,serve


class FaultHolderProtocol(unittest.TestCase):
    def test_late_final_receipt_resumes_same_child_without_normal_stop_or_resend(self):
        with tempfile.TemporaryDirectory() as d:
            path=Path(d)/'child.py'
            path.write_text('''import json,sys,time
assert json.loads(sys.stdin.readline())=={'Op':'stop_fault'}
print(json.dumps({'status':'fault_stop_ready','joined':True,'fault':True}),flush=True)
assert sys.stdin.read()==''
time.sleep(.15)
print(json.dumps({'status':'fault_closed','joined':True,'fault':True}),flush=True)
sys.exit(1)
''')
            class ShortFinalRead(JoinedPipe):
                calls=0
                def read(self,timeout=40,**kwargs):
                    self.calls+=1
                    return super().read(.02 if self.calls==2 else timeout,**kwargs)
            child=ShortFinalRead([sys.executable,str(path)])
            try:
                with self.assertRaisesRegex(RuntimeError,'private_pipe_timeout'):child.stop_fault()
                self.assertTrue(child.fault_ready_seen);self.assertFalse(child.fault_joined)
                with self.assertRaisesRegex(RuntimeError,'not_normal_close'):child.stop()
                child.stop_fault()
                self.assertTrue(child.fault_joined);self.assertEqual(child.proc.returncode,1)
            finally:
                if not child.proc.stdin.closed:child.proc.stdin.close()
                child.proc.wait(3);child._join_diagnostics();child.proc.stdout.close()

    def test_revocation_rejection_keeps_owner_then_real_fault_exit_cleans_own_metadata(self):
        with tempfile.TemporaryDirectory(dir='/tmp') as d:
            root=Path(d).resolve();script=root/'child.py'
            script.write_text('''import json,sys
revoked=False
for line in sys.stdin:
 r=json.loads(line)
 if r['Op']=='open':v={'status':'ready','vault_scope':'a'*32,'public_key':'b'*64}
 elif r['Op']=='network':revoked=True;v={'status':'fixture_confirmed'}
 elif r['Op']=='stop_fault':
  if not revoked:v={'status':'fault_stop_rejected'}
  else:
   print(json.dumps({'status':'fault_stop_ready','joined':True,'fault':True}),flush=True);break
 else:v={'status':'product_unavailable'}
 print(json.dumps(v),flush=True)
sys.stdin.read()
print(json.dumps({'status':'fault_closed','joined':True,'fault':True}),flush=True)
sys.exit(1)
''')
            children=[];outcomes=[]
            def factory(command,config):
                p=JoinedPipe([sys.executable,str(script)],config);children.append(p);return p
            worker=threading.Thread(target=lambda:outcomes.append(serve(root,[],factory,require_native=False)))
            worker.start();endpoint=Endpoint(root)
            deadline=time.monotonic()+5
            while not(root/'owner.json').exists() and time.monotonic()<deadline:time.sleep(.01)
            self.assertEqual(endpoint({'Op':'owner_stop_fault'}),{'status':'holder_outcome_unknown_state_retained'})
            self.assertIsNone(children[0].proc.poll());self.assertFalse(children[0].proc.stdin.closed)
            self.assertEqual(endpoint({'Op':'owner_stop_fault','extra':True}),{'status':'holder_outcome_unknown_state_retained'})
            self.assertEqual(endpoint({'Op':'network'}),{'status':'fixture_confirmed'})
            self.assertEqual(endpoint({'Op':'owner_stop_fault'}),{'status':'owner_fault_closed','joined':True,'fault':True,'exit_code':1})
            worker.join(5);self.assertFalse(worker.is_alive());self.assertEqual(outcomes,[True])
            self.assertEqual(children[0].proc.returncode,1);self.assertTrue(children[0].fault_joined)
            self.assertFalse((root/'owner.sock').exists());self.assertFalse((root/'owner.json').exists())

    def test_fault_receipt_needs_true_booleans_exact_terminal_eof_and_exit_one(self):
        for mode in ('integer','extra','duplicate','zero','failed'):
            with self.subTest(mode=mode),tempfile.TemporaryDirectory() as d:
                path=Path(d)/'child.py'
                path.write_text('''import json,sys
json.loads(sys.stdin.readline())
ready={'status':'fault_stop_ready','joined':True,'fault':True}
MODE=MODE_VALUE
if MODE=='integer':ready['joined']=1
print(json.dumps(ready),flush=True)
sys.stdin.read()
terminal={'status':'fault_closed','joined':True,'fault':True}
if MODE=='extra':terminal['extra']=True
if MODE=='failed':terminal={'status':'failed'}
print(json.dumps(terminal),flush=True)
if MODE=='duplicate':print(json.dumps(terminal),flush=True)
sys.exit(0 if MODE=='zero' else 1)
'''.replace('MODE_VALUE',repr(mode)))
                child=JoinedPipe([sys.executable,str(path)])
                try:
                    with self.assertRaises(RuntimeError):child.stop_fault()
                    self.assertFalse(child.fault_joined)
                finally:
                    if not child.proc.stdin.closed:child.proc.stdin.close()
                    child.proc.wait(timeout=3);child._join_diagnostics();child.proc.stdout.close()

