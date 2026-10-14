import importlib.util,json,unittest
from unittest.mock import patch
from pathlib import Path

spec=importlib.util.spec_from_file_location('native_maint',Path(__file__).with_name('native-registration-maintenance.py'))
m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)

class NativeMaintenanceTests(unittest.TestCase):
    def test_resource_digest_ignores_online_but_never_accepts_extra_resources(self):
        user={'id':1,'name':'long-lived'}
        node={'id':2,'user':user,'node_key':'public-key','ip_addresses':['100.120.0.1'],'online':True}
        key={'id':3,'user':user,'key':'public-test-fixture'}
        data={'users':[user],'nodes':[node],'preauthkeys':[key],'policy':{'acls':[]}}
        def call(args):return json.dumps(data[args[len(m.HS)]]).encode()
        with patch.object(m,'command',side_effect=call):
            before=m.resource_fingerprint();node['online']=False
            self.assertEqual(before,m.resource_fingerprint())
            data['nodes'].append(dict(node,id=4))
            with self.assertRaisesRegex(RuntimeError,'original_resource_count'):m.resource_fingerprint()
    def test_nonzero_policy_rejected(self):
        user={'id':1,'name':'long-lived'}
        data={'users':[user],'nodes':[{'id':2,'user':user,'node_key':'key','ip_addresses':[]}],'preauthkeys':[{'id':3,'user':user,'key':'fixture'}],'policy':{'acls':[{}]}}
        with patch.object(m,'command',side_effect=lambda a:json.dumps(data[a[len(m.HS)]]).encode()):
            with self.assertRaisesRegex(RuntimeError,'not_zero_policy'):m.resource_fingerprint()
    def test_retained_lock_rejected_without_removing_it(self):
        with patch.object(m.os.path,'lexists',return_value=True):
            with self.assertRaisesRegex(RuntimeError,'owner_not_released'):m.no_locks()
    def test_formal_store_change_rejected_before_restore(self):
        with patch.object(m,'digest',return_value='wrong'):
            with self.assertRaisesRegex(RuntimeError,'formal_store_changed'):m.baseline({'store_sha':m.EXPECTED})
    def test_stage_alias_and_outside_path_rejected(self):
        with patch.object(m.os,'geteuid',return_value=0):
            for stage in ['/','/var/lib/remoteapp-authority',m.BASE+'/remoteapp-native-bad']:
                with self.assertRaisesRegex(RuntimeError,'invalid_stage'):m.stage_valid(stage)
    def test_active_without_current_pid_ready_receipt_is_not_ready(self):
        logs=[b'READY old process\n',b'READY expected\n']
        with patch.object(m,'props',return_value={'MainPID':'42','ActiveState':'active'}),patch.object(m,'command',side_effect=logs) as call,patch.object(m.time,'sleep') as sleep:
            m.wait_ready('authority','READY expected')
            self.assertEqual(call.call_count,2);sleep.assert_called_once()
            self.assertIn('_PID=42',call.call_args[0][0])

if __name__=='__main__':unittest.main()
