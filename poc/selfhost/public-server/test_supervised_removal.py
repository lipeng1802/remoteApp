import copy
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

from supervised_removal import checked_receipt, checked_bootstrap, pair
from persistent_supervisor import Owner


def receipt():
    return dict(status='files_removed',epoch=1,transaction='a'*32,preserved=True,joined=True,zero_exit=True)


class RemovalCoordination(unittest.TestCase):
    def test_strict_receipt_and_compiled_bootstrap(self):
        checked_receipt(receipt())
        for k,v in (('preserved',1),('joined',False),('zero_exit',1),('epoch',True),('transaction','B'*32),('extra','PRIVATE')):
            r=receipt();r[k]=v
            with self.assertRaises(RuntimeError):checked_receipt(r)
        good=dict(removal_protocol=1,uninstaller_process_id=20,process_id=10)
        checked_bootstrap(good)
        for k,v in (('removal_protocol',True),('removal_protocol',2),('uninstaller_process_id',10),('uninstaller_process_id',True)):
            r=copy.copy(good);r[k]=v
            with self.assertRaises(RuntimeError):checked_bootstrap(r)

    def test_parallel_start_once_and_same_operation_poll_then_durable_receipts(self):
        with tempfile.TemporaryDirectory() as d:
            work=Path(d);starts=[0,0];polls=[0,0];elapsed=[0.0]
            def endpoint(i):
                def start():starts[i]+=1;self.assertTrue((work/'program-removal-intent.json').exists());return {'status':'removal_pending'}
                def status():
                    polls[i]+=1
                    return {'status':'removal_pending'} if polls[i]<2 else receipt()
                return start,status
            out=pair(work,endpoint(0),endpoint(1),expires_at=1500,clock=lambda:1000+elapsed[0],monotonic=lambda:elapsed[0],sleep=lambda n:elapsed.__setitem__(0,elapsed[0]+n))
            self.assertEqual(starts,[1,1]);self.assertEqual(set(out),{'target','controller'})
            self.assertTrue((work/'program-removal-result.json').exists())
            with self.assertRaises(FileExistsError):pair(work,endpoint(0),endpoint(1),expires_at=1500)
            self.assertEqual(starts,[1,1])

    def test_unknown_never_restarts_or_writes_completion(self):
        with tempfile.TemporaryDirectory() as d:
            work=Path(d);starts=[]
            def start():starts.append(1);return {'status':'removal_pending'}
            with self.assertRaises(RuntimeError):
                pair(work,(start,lambda:{'status':'removal_unknown'}),(start,receipt),expires_at=1500,clock=lambda:1000)
            self.assertEqual(len(starts),2)
            self.assertFalse((work/'program-removal-result.json').exists())
            self.assertNotIn('PRIVATE',(work/'program-removal-intent.json').read_text())

    def test_parent_seals_duplicate_start_and_early_stop_without_closing_pipe(self):
        class Child:
            def stop(self):raise AssertionError('must retain pipe')
        owner=Owner(Child());owner.removal_enabled=True;sent=[]
        class Lifecycle:
            pending=False;poisoned=False
            def __call__(self,frame):
                sent.append(frame)
                return {'status':'removal_pending'} if frame['Op']=='remove_program' else receipt()
        owner.lifecycle=Lifecycle()
        self.assertEqual(owner.call({'Op':'owner_remove_program'}),{'status':'removal_pending'})
        with self.assertRaises(RuntimeError):owner.call({'Op':'owner_remove_program'})
        with self.assertRaises(RuntimeError):owner.call({'Op':'owner_stop'})
        self.assertEqual(owner.call({'Op':'owner_program_removal_status'}),receipt())
        self.assertEqual(len(sent),2)
