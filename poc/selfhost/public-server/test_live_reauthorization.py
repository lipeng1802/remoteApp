"""Private orchestration regressions; no actual Headscale/TLS acceptance."""
import base64
import copy
import hashlib
from types import SimpleNamespace
import unittest
from unittest.mock import Mock
from compiled_products import CompiledPair


class LiveReauthorizationTests(unittest.TestCase):
    def fixture(self, fault=None):
        events=[]
        old_grant=dict(claims=dict(id='a'*32,target='target',controller='controller'))
        new_grant=dict(claims=dict(id='b'*32,target='target',controller='controller'))
        old_binding=dict(target_node='old-target',controller_node='old-controller')
        binding=dict(target_node='new-target',controller_node='new-controller')
        old={r:dict(Grant=copy.deepcopy(old_grant),Binding=old_binding,StateDir='/old/'+r,Server=r=='target') for r in ('target','controller')}
        inputs={r:dict(Grant=copy.deepcopy(new_grant),Binding=binding,StateDir='/fresh/'+r,Server=r=='target') for r in ('target','controller')}
        if fault=='same_grant':
            for r in inputs: inputs[r]['Grant']=copy.deepcopy(old_grant)
        if fault=='subject':
            for r in inputs: inputs[r]['Grant']['claims']['target']='foreign'
        if fault=='invalid_id':
            for r in inputs: inputs[r]['Grant']['claims']['id']=True
        if fault=='old_dir': inputs['target']['StateDir']=old['target']['StateDir']
        if fault=='old_node': binding['target_node']=old_binding['target_node']
        if fault=='reuse_certificate': inputs['target']['ReuseProductCertificate']=True
        class API:
            def request(self,op,**kwargs):
                events.append(op)
                if op=='use_grant': return dict(status=op,passed=fault=='old_allowed')
                if op=='recheck_grant':
                    return dict(status=op,passed=kwargs['Grant']==inputs['target']['Grant'])
                return dict(passed=True,network_state=dict(claims=dict(active=fault!='inactive',grant=inputs['target']['Grant'],binding=binding)))
        class Target:
            def request(self,op):
                events.append(op)
                if op=='reopen_fault': return dict(status='owner_reopened',started=0 if fault=='numeric_ack' else False)
                return dict(status='session_state',joined=False,fault=False)
        class Pair(CompiledPair):
            def assert_fault_latched(self):
                events.append('latched')
                if fault=='unjoined': raise RuntimeError('unjoined')
            def maintenance(self,op): events.append(op)
            def _start_products(self,Pipe,values,fresh_authorization=False):
                if not fresh_authorization: raise AssertionError('missing_fresh_authorization_gate')
                events.append('new_products')
                self.controller=SimpleNamespace(proc=SimpleNamespace(pid=2))
            def stop_sessions(self): events.append('normal_join');self.sessions_joined=True
        class T:
            def check(self,ok,label):
                if not ok: raise AssertionError(label)
        pair=Pair(T(),'/tmp/fixture',r'C:\fixture',{},None)
        pair._accepted_inputs=old
        pair.target=Target()
        pair._retired_controller_proc=SimpleNamespace(pid=1,poll=lambda:None if fault=='old_running' else 1)
        handoff=SimpleNamespace(take=lambda:(events.append('bound_join') or inputs))
        return pair,handoff,[API(),API()],binding,events

    def test_new_authorization_is_one_shot_verified_before_reopen(self):
        pair,handoff,apis,binding,events=self.fixture()
        pair.reauthorize(None,handoff,apis,binding)
        self.assertLess(events.index('network_state'),events.index('reopen_fault'))
        self.assertLess(events.index('bound_join'),events.index('new_products'))
        self.assertEqual(events[-2:],['new_products','normal_join'])
        with self.assertRaises(RuntimeError):pair.reauthorize(None,handoff,apis,binding)

    def test_failed_transition_never_launches_or_retries(self):
        for fault in ('unjoined','old_running','same_grant','subject','invalid_id','old_dir','old_node','reuse_certificate','old_allowed','inactive','numeric_ack'):
            pair,handoff,apis,binding,events=self.fixture(fault)
            with self.subTest(fault=fault),self.assertRaises(RuntimeError):pair.reauthorize(None,handoff,apis,binding)
            self.assertNotIn('new_products',events)
            with self.assertRaises(RuntimeError):pair.reauthorize(None,handoff,apis,binding)

    def test_new_grant_pin_must_be_distinct_and_hmac_must_not_recur(self):
        for fault in ('same_pin','same_key'):
            pair=CompiledPair(None,'/tmp/fixture',r'C:\fixture',{},None)
            key=base64.b64encode(bytes(32)).decode()
            pair._last_product_fingerprint='a'*64
            if fault=='same_key':pair._last_product_key_digest=hashlib.sha256(bytes(32)).digest()
            pair.target=SimpleNamespace(send=lambda value:None,read=lambda:dict(status='product_ready',Key=key,Fingerprint=('a' if fault=='same_pin' else 'b')*64))
            launch=Mock()
            with self.subTest(fault=fault),self.assertRaises(RuntimeError):
                pair._start_products(launch,dict(target={}),fresh_authorization=True)
            launch.assert_not_called()
            self.assertFalse(pair.sessions_joined)


if __name__=='__main__':unittest.main()
