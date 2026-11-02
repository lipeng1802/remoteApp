import json
import os
from pathlib import Path
import sys
import tempfile
import threading
import time
import unittest
from joined_pipe import JoinedPipe
from persistent_supervisor import serve, Endpoint, Owner


class LocalCancelHolderTests(unittest.TestCase):
    def test_private_holder_publishes_distinct_receipt_after_actual_exit(self):
        with tempfile.TemporaryDirectory(dir='/tmp') as d:
            root=Path(d).resolve();script=root/'child.py'
            script.write_text('''import sys,json
assert json.loads(sys.stdin.readline())=={'Op':'open'}
print(json.dumps({'status':'ready','vault_scope':'a'*32,'public_key':'b'*64}),flush=True)
assert json.loads(sys.stdin.readline())=={'Op':'stop_local_fault'}
print(json.dumps({'status':'local_cancel_ready','joined':True,'fault':True,'remote_revocation_verified':False}),flush=True)
assert sys.stdin.read()==''
print(json.dumps({'status':'local_cancel_closed','joined':True,'fault':True,'remote_revocation_verified':False}),flush=True)
sys.exit(1)
''')
            children=[];outcomes=[]
            def factory(command,config):
                p=JoinedPipe([sys.executable,str(script)],config);children.append(p);return p
            worker=threading.Thread(target=lambda:outcomes.append(serve(root,[],factory,require_native=False)))
            worker.start();endpoint=Endpoint(root)
            deadline=time.monotonic()+5
            while not(root/'owner.json').exists() and time.monotonic()<deadline:time.sleep(.01)
            self.assertEqual(endpoint({'Op':'owner_stop_local_fault','extra':True}),{'status':'holder_outcome_unknown_state_retained'})
            self.assertIsNone(children[0].proc.poll())
            self.assertEqual(endpoint({'Op':'owner_stop_local_fault'}),{'status':'owner_local_cancel_closed','joined':True,'fault':True,'exit_code':1,'remote_revocation_verified':False})
            worker.join(5);self.assertFalse(worker.is_alive());self.assertEqual(outcomes,[True])
            self.assertTrue(children[0].local_cancel_joined);self.assertFalse(children[0].fault_joined)
            self.assertEqual(children[0].proc.returncode,1)
            self.assertFalse((root/'owner.json').exists());self.assertFalse((root/'owner.sock').exists())
            self.assertFalse((root/'owner-fault-joined.json').exists())
            self.assertEqual(json.loads((root/'owner-local-cancel-joined.json').read_text()),{
                'version':1,'status':'owner_local_cancel_closed','joined':True,'fault':True,'exit_code':1,
                'remote_revocation_verified':False,'supervisor_process_id':children[0].proc.pid,'holder_process_id':os.getpid()})

    def test_local_receipt_blocks_normal_or_remote_fault_conversion(self):
        class Child:
            local_cancel_sent=True
            class Proc:
                class Stream:
                    def close(self):pass
                stdout=Stream()
            proc=Proc()
            def stop_local_fault(self):pass
        owner=Owner(Child(),lambda r:None)
        owner.call({'Op':'owner_stop_local_fault'})
        for op in ('owner_stop','owner_stop_fault'):
            with self.assertRaises(RuntimeError):owner.call({'Op':op})
        self.assertFalse(owner.fault)

    def test_existing_cancel_receipt_never_starts_another_child(self):
        with tempfile.TemporaryDirectory(dir='/tmp') as d:
            root=Path(d).resolve();(root/'owner-local-cancel-joined.json').write_text('{}')
            def factory(*args):self.fail('child started over old receipt')
            with self.assertRaisesRegex(RuntimeError,'metadata_exists'):serve(root,[],factory,require_native=False)
