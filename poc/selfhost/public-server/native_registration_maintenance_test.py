import errno,importlib.util,json,unittest
from contextlib import ExitStack,contextmanager
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
        def row(msg):return (json.dumps({'MESSAGE':msg,'_PID':'42','__MONOTONIC_TIMESTAMP':'100'})+'\n').encode()
        logs=[row('READY old process'),row('READY expected')]
        with patch.object(m,'props',return_value={'MainPID':'42','ActiveState':'active','ExecMainStartTimestampMonotonic':'100'}),patch.object(m,'command',side_effect=logs) as call,patch.object(m.time,'sleep') as sleep:
            m.wait_ready('authority','READY expected')
            self.assertEqual(call.call_count,2);sleep.assert_called_once()
            self.assertIn('_PID=42',call.call_args[0][0])
            self.assertIn('-b',call.call_args[0][0])
    def test_reused_pid_old_receipt_is_rejected(self):
        rows=[{'MESSAGE':'READY expected','_PID':'42','__MONOTONIC_TIMESTAMP':'99'},
              {'MESSAGE':'READY expected','_PID':'43','__MONOTONIC_TIMESTAMP':'101'}]
        with patch.object(m,'command',return_value=('\n'.join(json.dumps(r) for r in rows)).encode()):
            self.assertFalse(m.process_receipt('authority',42,100,'READY expected'))
    def test_restart_during_journal_read_requires_new_receipt(self):
        old={'MainPID':'42','ActiveState':'active','ExecMainStartTimestampMonotonic':'100'}
        new=dict(old,MainPID='43',ExecMainStartTimestampMonotonic='200')
        logs=[json.dumps({'MESSAGE':'READY','_PID':str(pid),'__MONOTONIC_TIMESTAMP':str(st)}).encode() for pid,st in [(42,100),(43,200)]]
        with patch.object(m,'props',side_effect=[old,new,new,new]),patch.object(m,'command',side_effect=logs),patch.object(m.time,'sleep'):
            self.assertEqual(m.wait_ready('authority','READY'),(43,200))
    def test_missing_start_identity_and_failed_service_refuse_readiness(self):
        for state,expected in [('active','restore_start_identity_missing'),('failed','restore_service_failed')]:
            with patch.object(m,'props',return_value={'MainPID':'42','ActiveState':state}),patch.object(m,'command') as call:
                with self.assertRaisesRegex(RuntimeError,expected):m.wait_ready('authority','READY')
                call.assert_not_called()

    @contextmanager
    def window(self,phase='paused',served=False):
        stage=m.BASE+'/remoteapp-native-0123456789abcdef'
        fx={'stage':stage,'state':{'version':1,'phase':phase},'served':served,'events':[],
            'snapshots':[],'fail_start':None,'no_ready':set(),'baseline_changed':False}
        services={n:{'MainPID':'0','ActiveState':'inactive','ExecMainStatus':'0','ExecMainStartTimestampMonotonic':'0'} for n in m.SERVICES}
        services['remoteapp-poc']={'ActiveState':'active'};fx['services']=services
        def atomic(stage,state):
            fx['state']=dict(state);fx['snapshots'].append(state['phase'])
        def baseline(state):
            if fx['baseline_changed']:raise RuntimeError('formal_store_changed')
        def command(args):
            if args[0]=='systemctl':
                action,name=args[1:];fx['events'].append((action,name))
                if fx['fail_start']==name:raise RuntimeError('command_failed')
                if action=='start':services[name]=dict(MainPID='42' if name=='remoteapp-authority' else '43',ActiveState='active',ExecMainStatus='0',ExecMainStartTimestampMonotonic='100')
                elif action=='stop':services[name]=dict(services[name],MainPID='0',ActiveState='inactive')
                else:raise AssertionError('unexpected mutation')
                return b''
            self.assertEqual(args[0],'journalctl')
            name=args[args.index('-u')+1];pid=next(x[5:] for x in args if x.startswith('_PID='))
            label=m.READY[name] if services[name]['ActiveState']=='active' else ('STOPPED authority' if name=='remoteapp-authority' else 'STOPPED frontend')
            fx['events'].append(('receipt',name))
            if name in fx['no_ready']:return b''
            return json.dumps({'MESSAGE':label,'_PID':pid,'__MONOTONIC_TIMESTAMP':'101'}).encode()
        def exists(path):
            if path.endswith('/maintenance.json'):return fx['state']['phase'] is not None
            if path.endswith('/public-ready.json'):return fx['served']
            return False
        with ExitStack() as stack:
            stack.enter_context(patch.object(m,'stage_valid'))
            stack.enter_context(patch.object(m,'load_stage_json',side_effect=lambda stage,name:dict(fx['state']) if name=='maintenance.json' else {'Version':1,'PID':9999}))
            stack.enter_context(patch.object(m,'baseline',side_effect=baseline))
            stack.enter_context(patch.object(m,'atomic',side_effect=atomic))
            stack.enter_context(patch.object(m,'props',side_effect=lambda name:dict(services[name])))
            stack.enter_context(patch.object(m,'command',side_effect=command))
            stack.enter_context(patch.object(m,'no_locks'))
            stack.enter_context(patch.object(m,'digest',return_value=m.EXPECTED))
            stack.enter_context(patch.object(m,'resource_fingerprint',return_value='public-fingerprint'))
            stack.enter_context(patch.object(m.os.path,'lexists',side_effect=exists))
            stack.enter_context(patch.object(m.os,'kill',side_effect=OSError(errno.ESRCH,'fixture')))
            stack.enter_context(patch.object(m.time,'sleep'))
            stack.enter_context(patch('builtins.print'))
            yield fx
    def test_prepare_pause_resume_and_read_only_audit(self):
        with self.window(None) as fx:
            for name in m.SERVICES:fx['services'][name]=dict(MainPID='42',ActiveState='active',ExecMainStatus='0',ExecMainStartTimestampMonotonic='100')
            m.main(fx['stage'],'prepare');m.main(fx['stage'],'pause')
            fx['served']=True
            m.main(fx['stage'],'resume')
            self.assertEqual(fx['snapshots'],['prepared','pausing','paused','resuming','restored'])
            starts=[e for e in fx['events'] if e[0]=='start']
            self.assertEqual(starts,[('start','remoteapp-authority'),('start','remoteapp-frontend')])
            self.assertLess(fx['events'].index(('receipt','remoteapp-authority'),fx['events'].index(starts[0])),fx['events'].index(starts[1]))
            before=list(fx['events']);writes=len(fx['snapshots'])
            m.main(fx['stage'],'verify-restored')
            self.assertEqual([e for e in fx['events'][len(before):] if e[0]!='receipt'],[])
            self.assertEqual(len(fx['snapshots']),writes)
    def test_explicit_finish_resumes_only_missing_original_frontend(self):
        with self.window(served=True) as fx:
            fx['fail_start']='remoteapp-frontend'
            with self.assertRaisesRegex(RuntimeError,'command_failed'):m.main(fx['stage'],'resume')
            self.assertEqual(fx['state']['phase'],'resuming')
            with self.assertRaisesRegex(RuntimeError,'unexpected_phase'):m.main(fx['stage'],'resume')
            fx['fail_start']=None;fx['events']=[]
            m.main(fx['stage'],'finish-resume')
            self.assertEqual([e for e in fx['events'] if e[0]=='start'],[('start','remoteapp-frontend')])
            self.assertEqual(fx['state']['phase'],'restored')
    def test_no_ready_does_not_start_frontend_or_mark_restored(self):
        with self.window() as fx:
            fx['no_ready'].add('remoteapp-authority')
            with self.assertRaisesRegex(RuntimeError,'restore_not_ready'):m.main(fx['stage'],'resume')
            self.assertNotIn(('start','remoteapp-frontend'),fx['events'])
            self.assertEqual(fx['snapshots'],['resuming'])
    def test_failed_original_service_requires_separate_decision_not_retry(self):
        with self.window('resuming') as fx:
            fx['services']['remoteapp-authority']['ActiveState']='failed'
            with self.assertRaisesRegex(RuntimeError,'restore_service_failed'):m.main(fx['stage'],'finish-resume')
            self.assertEqual(fx['events'],[])
    def test_live_frontend_with_inactive_authority_is_ambiguous_not_reused(self):
        with self.window('resuming') as fx:
            fx['services']['remoteapp-frontend']=dict(MainPID='43',ActiveState='active',ExecMainStatus='0',ExecMainStartTimestampMonotonic='100')
            with self.assertRaisesRegex(RuntimeError,'restore_order_ambiguous'):m.main(fx['stage'],'finish-resume')
            self.assertEqual(fx['events'],[])
    def test_changed_baseline_blocks_partial_restore_before_any_start(self):
        with self.window('resuming') as fx:
            fx['baseline_changed']=True
            with self.assertRaisesRegex(RuntimeError,'formal_store_changed'):m.main(fx['stage'],'finish-resume')
            self.assertEqual(fx['events'],[])
    def test_live_candidate_blocks_restore_even_with_a_join_file(self):
        with self.window(served=True) as fx,patch.object(m.os,'kill',return_value=None):
            with self.assertRaisesRegex(RuntimeError,'candidate_still_live'):m.main(fx['stage'],'resume')
            self.assertEqual(fx['events'],[]);self.assertEqual(fx['snapshots'],[])
    def test_missing_candidate_join_does_not_fabricate_exit(self):
        with self.window(served=True) as fx,patch.object(m,'load_stage_json',side_effect=lambda stage,name:dict(fx['state']) if name=='maintenance.json' else {}):
            with self.assertRaisesRegex(RuntimeError,'candidate_join_receipt_missing'):m.main(fx['stage'],'resume')
            self.assertEqual(fx['events'],[])
    def test_dependency_failure_after_frontend_ready_blocks_final_receipt(self):
        with self.window() as fx:
            def ready(name,label):
                if name=='remoteapp-frontend':fx['services']['remoteapp-authority']['ActiveState']='failed'
                return (int(fx['services'][name]['MainPID']),100)
            with patch.object(m,'wait_ready',side_effect=ready):
                with self.assertRaisesRegex(RuntimeError,'restore_identity_changed'):m.main(fx['stage'],'resume')
            self.assertEqual(fx['state']['phase'],'resuming')

if __name__=='__main__':unittest.main()
