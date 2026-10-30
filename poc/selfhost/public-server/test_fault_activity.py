import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
from activity_control import Activity
import fault_activity as fault


class FaultActivityGates(unittest.TestCase):
    def fixture(self,d):
        w=Path(d).resolve();n='a'*12
        m=dict(nonce=n,win_dir='C:\\Users\\jarvis\\remoteapp-r75-package-'+n,win_root='C:\\Users\\jarvis\\remoteapp-r75-'+n,mac_root='/private/tmp/remoteapp-r75-'+n+'/installed',holder_dir='/private/tmp/remoteapp-r75-'+n+'/holder')
        (w/'management.json').write_text(json.dumps(m));Activity(w).save({'version':1,'phase':'exercise_outcome_unknown'})
        win={'fault_protocol':1,'process_id':13,'control_pipe':'\\\\.\\pipe\\remoteapp-session-'+'b'*32}
        mac={'fault_protocol':1,'process_id':17,'supervisor_process_id':19}
        (w/'windows-bootstrap.json').write_text(json.dumps(win));(w/'mac-bootstrap.json').write_text(json.dumps(mac))
        return w,m,win,mac

    def test_unknown_revocation_checkpoint_precedes_effect_and_cannot_stop(self):
        with tempfile.TemporaryDirectory() as d:
            w,_,_,_=self.fixture(d)
            def endpoint(frame):
                self.assertEqual(fault.journal(w).read()['phase'],'fault_revoke_unknown')
                self.assertEqual(frame,{'Op':'revoke_session'});raise RuntimeError('PRIVATE')
            with patch.object(fault,'WindowsRelay',return_value=endpoint):
                with self.assertRaisesRegex(RuntimeError,'PRIVATE'):fault.revoke(w)
            with patch.object(fault,'mac_join') as mac:
                with self.assertRaisesRegex(RuntimeError,'revocation_confirmation_required'):fault.stop(w)
                mac.assert_not_called()
            self.assertNotIn('PRIVATE',(w/'fault-state.json').read_text())
            self.assertEqual(Activity(w).read()['phase'],'exercise_outcome_unknown')

    def test_strict_revocation_and_partial_stop_never_become_normal_success(self):
        with tempfile.TemporaryDirectory() as d:
            w,_,_,_=self.fixture(d)
            with patch.object(fault,'WindowsRelay',return_value=lambda frame:{'status':'session_revoked','passed':1}):
                with self.assertRaisesRegex(RuntimeError,'revoke_unconfirmed'):fault.revoke(w)
            with patch.object(fault,'WindowsRelay',return_value=lambda frame:{'status':'session_revoked','passed':True}):fault.revoke(w)
            calls=[]
            def mac(*args):calls.append('mac')
            def windows(*args):calls.append('windows');raise RuntimeError('unknown')
            with patch.object(fault,'mac_join',side_effect=mac),patch.object(fault,'windows_join',side_effect=windows):
                with self.assertRaisesRegex(RuntimeError,'unknown'):fault.stop(w)
            self.assertEqual(fault.journal(w).read()['joined'],{'controller':True})
            with patch.object(fault,'mac_join',side_effect=mac),patch.object(fault,'windows_join',side_effect=lambda *args:calls.append('windows')):fault.stop(w)
            self.assertEqual(calls,['mac','windows','windows'])
            self.assertEqual(fault.journal(w).read()['phase'],'owners_fault_joined')
            self.assertEqual(Activity(w).read()['phase'],'exercise_outcome_unknown')

    def test_receipts_reject_truthy_booleans_wrong_pids_and_mixed_fields(self):
        with tempfile.TemporaryDirectory() as d:
            _,_,win,_=self.fixture(d)
            good={'version':1,'status':'owner_fault_closed','joined':True,'fault':True,'exit_code':1,'process_id':13}
            self.assertEqual(fault.receipt(good,'windows',win),good)
            for field,value in (('version',True),('joined',1),('fault',1),('exit_code',True),('process_id',True),('process_id',14),('extra','PRIVATE')):
                bad={**good,field:value}
                with self.subTest(field=field,value=value),self.assertRaisesRegex(RuntimeError,'receipt_rejected'):fault.receipt(bad,'windows',win)

    def test_mac_monitor_zero_is_not_fault_join_even_with_matching_public_receipt(self):
        with tempfile.TemporaryDirectory() as d:
            w,m,_,mac=self.fixture(d);holder=w/'holder';holder.mkdir(mode=0o700);m['holder_dir']=str(holder)
            value={'version':1,'status':'owner_fault_closed','joined':True,'fault':True,'exit_code':1,'supervisor_process_id':19,'holder_process_id':17}
            (holder/'owner-fault-joined.json').write_text(json.dumps(value))
            (w/'mac-exit.json').write_text(json.dumps({'version':1,'nonce':m['nonce'],'exit_code':0}))
            with patch.object(fault,'Endpoint') as endpoint:
                with self.assertRaisesRegex(RuntimeError,'mac_exit_rejected'):fault.mac_join(w,m)
                endpoint.assert_not_called()
            (w/'mac-exit.json').write_text(json.dumps({'version':1,'nonce':m['nonce'],'exit_code':1}))
            fault.mac_join(w,m)

    def test_retirement_unknown_or_integer_join_cannot_repeat_native_deletion(self):
        for phase,joined in (('retirement_outcome_unknown',{'controller':True,'target':True}),('owners_fault_joined',{'controller':True,'target':1})):
            with tempfile.TemporaryDirectory() as d:
                w,_,_,_=self.fixture(d);fault.journal(w).save({'version':1,'phase':phase,'joined':joined})
                with patch.object(fault.r,'retire_pair') as retire:
                    with self.assertRaisesRegex(RuntimeError,'both_joins_required'):fault.retire(w)
                    retire.assert_not_called()

    def test_fault_retirement_rechecks_both_proofs_and_uses_separate_journal(self):
        with tempfile.TemporaryDirectory() as d:
            w,_,_,_=self.fixture(d)
            fault.journal(w).save({'version':1,'phase':'owners_fault_joined','joined':{'controller':True,'target':True}})
            with patch.object(fault,'mac_join') as mac,patch.object(fault,'windows_join') as windows,patch.object(fault.r,'retire_pair') as retire:
                fault.retire(w)
                mac.assert_called_once();windows.assert_called_once();retire.assert_called_once()
                self.assertEqual(retire.call_args.args[2].path,w/'fault-state.json')
                self.assertEqual(retire.call_args.args[4],'fault_joined_and_test_roots_retired')
            self.assertEqual(Activity(w).read()['phase'],'exercise_outcome_unknown')

    def test_old_or_boolean_capability_refused_before_signing_or_lifecycle(self):
        with tempfile.TemporaryDirectory() as d:
            w,_,win,_=self.fixture(d);win['fault_protocol']=True;(w/'windows-bootstrap.json').write_text(json.dumps(win))
            with patch.object(fault,'WindowsRelay') as endpoint:
                with self.assertRaisesRegex(RuntimeError,'fresh_capable_owner'):fault.revoke(w)
                endpoint.assert_not_called()
