import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
from local_cancel_proof import receipt,mac_file
import local_cancel_activity as activity


class LocalProofTests(unittest.TestCase):
    def test_strict_role_identity_and_local_only_claim(self):
        b={'local_cancel_protocol':1,'process_id':123,'supervisor_process_id':456}
        v={'version':1,'status':'owner_local_cancel_closed','joined':True,'fault':True,'exit_code':1,'remote_revocation_verified':False,'holder_process_id':123,'supervisor_process_id':456}
        self.assertEqual(receipt(v,'mac',b),v)
        for key,value in [('version',True),('joined',1),('fault',1),('exit_code',True),('remote_revocation_verified',0),('remote_revocation_verified',True),('holder_process_id',456),('status','owner_fault_closed'),('extra',True)]:
            bad=dict(v);bad[key]=value
            with self.assertRaises(RuntimeError):receipt(bad,'mac',b)
        with self.assertRaises(RuntimeError):receipt(v,'windows',b)
        with tempfile.TemporaryDirectory() as d:
            p=Path(d)/'proof';p.write_text(json.dumps(v));p.chmod(0o600)
            self.assertEqual(mac_file(p,b),v)
            q=Path(d)/'link';q.symlink_to(p)
            with self.assertRaises(RuntimeError):mac_file(q,b)
            p.chmod(0o644)
            with self.assertRaises(RuntimeError):mac_file(p,b)

    def test_old_pair_rejected_before_any_cancel_or_journal_write(self):
        with tempfile.TemporaryDirectory() as d:
            w=Path(d);(w/'activity-state.json').write_text('{"version":1,"phase":"exercise_outcome_unknown"}')
            for role in ('mac','windows'):(w/(role+'-bootstrap.json')).write_text('{"process_id":123}')
            with patch.object(activity.r,'metadata',return_value=(w,{})),patch.object(activity,'Endpoint') as endpoint,patch.object(activity.r,'ps') as ps:
                with self.assertRaisesRegex(RuntimeError,'requires_new_capable_pair'):activity.stop(w)
                endpoint.assert_not_called();ps.assert_not_called()
            self.assertFalse((w/'local-cancel-state.json').exists())

    def test_saved_join_requires_receipts_again_and_never_authorizes_retirement(self):
        with tempfile.TemporaryDirectory() as d:
            w=Path(d);holder=w/'holder';holder.mkdir()
            m={'holder_dir':str(holder),'nonce':'fixture','win_dir':'FIXTURE'}
            (w/'activity-state.json').write_text('{"version":1,"phase":"exercise_outcome_unknown"}')
            (w/'local-cancel-state.json').write_text('{"version":1,"phase":"local_joined_remote_unverified","joined":{"controller":true,"target":true}}')
            for role in ('mac','windows'):(w/(role+'-bootstrap.json')).write_text('{"local_cancel_protocol":1,"process_id":123,"supervisor_process_id":456}')
            (w/'mac-local-cancel-exit.json').write_text('{"version":1,"nonce":"fixture","exit_code":1,"remote_revocation_verified":false}')
            v={'version':1,'status':'owner_local_cancel_closed','joined':True,'fault':True,'exit_code':1,'remote_revocation_verified':False,'holder_process_id':123,'supervisor_process_id':456}
            proof=holder/'owner-local-cancel-joined.json';proof.write_text(json.dumps(v));proof.chmod(0o600)
            target=dict(v);del target['holder_process_id'];del target['supervisor_process_id'];target['process_id']=123
            with patch.object(activity.r,'metadata',return_value=(w,m)),patch.object(activity,'Endpoint') as endpoint,patch.object(activity.r,'ps',return_value=json.dumps(target)) as ps:
                activity.stop(w);endpoint.assert_not_called();self.assertEqual(ps.call_count,1)
                self.assertNotIn('New-Item',ps.call_args.args[0])
                state=json.loads((w/'local-cancel-state.json').read_text())
                self.assertEqual(state['phase'],'local_joined_remote_unverified');self.assertIs(state['remote_revocation_verified'],False)
                proof.unlink()
                with self.assertRaises(Exception):activity.stop(w)
                self.assertEqual(ps.call_count,1)
            self.assertFalse((w/'fault-state.json').exists())

    def test_monitor_marker_requires_distinct_receipt_and_actual_wait_result(self):
        import subprocess
        for code in (0,1):
            with self.subTest(code=code),tempfile.TemporaryDirectory() as d:
                w=Path(d);holder=w/'holder';holder.mkdir()
                b={'local_cancel_protocol':1,'process_id':123,'supervisor_process_id':456}
                (w/'mac-bootstrap.json').write_text(json.dumps(b))
                v={'version':1,'status':'owner_local_cancel_closed','joined':True,'fault':True,'exit_code':1,'remote_revocation_verified':False,'holder_process_id':123,'supervisor_process_id':456}
                proof=holder/'owner-local-cancel-joined.json';proof.write_text(json.dumps(v));proof.chmod(0o600)
                m={'holder_dir':str(holder),'nonce':'fixture'}
                with patch.object(activity.r,'metadata',return_value=(w,m)),patch.object(activity.r.subprocess,'run',return_value=subprocess.CompletedProcess([],code)):
                    if code==1:activity.r.monitor(w)
                    else:
                        with self.assertRaises(RuntimeError):activity.r.monitor(w)
                self.assertEqual((w/'mac-local-cancel-exit.json').exists(),code==1)
