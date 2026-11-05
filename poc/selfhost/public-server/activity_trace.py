"""Public TEST stage evidence only: never serialize a request, reply or error."""
import json
import os
import threading
import time


class RecordedEndpoint:
    def __init__(self,work,role,endpoint):
        if role not in ('target','controller'):raise RuntimeError('activity_trace_role_rejected')
        self.directory=work/('trace-'+role)
        self.directory.mkdir(mode=0o700)  # Existing evidence must be preserved.
        self.started = time.monotonic()
        self.endpoint=endpoint;self.sequence=0;self.lock=threading.Lock()

    def event(self,phase,op,action,status=None, evidence=None):
        self.sequence+=1
        value={'version':1,'phase':phase,'op':op,'action':action,'elapsed_ms':int((time.monotonic()-self.started)*1000)}
        if status is not None:value['status']=status
        if evidence is not None:value.update(evidence)
        with (self.directory/('%06d.json'%self.sequence)).open('x') as output:
            json.dump(value,output,sort_keys=True);output.flush();os.fsync(output.fileno())

    def __call__(self,frame):
        with self.lock:
            op=frame.get('Op') if isinstance(frame,dict) else None
            if op not in ('device','network','provision','product_stage','product_count','quiesce_session','revoke_session','preprovision_terminal','start_controller',
                          'desktop_begin','desktop_status','desktop_state','desktop_arm_stop'):op='other'
            nested=frame.get('Device' if op=='device' else 'Network',{}) if isinstance(frame,dict) else {}
            action=nested.get('action') if isinstance(nested,dict) else None
            if action not in ('register','invite','claim','incoming','approve','status','revoke','grant_clock'):action='other'
            self.event('calling',op,action)
            try:result=self.endpoint(frame)
            except Exception:
                self.event('outcome_unknown',op,action)
                raise
            status=result.get('status') if isinstance(result,dict) else None
            if status not in ('device_result','network_authorization','network_watch','product_stage','product_count','product_unavailable','session_quiesced','session_revoked','preprovision_terminal','controller_started','failed','unavailable','rejected','request_rejected',
                              'desktop_pending','desktop_status','desktop_state','desktop_stop_armed'):status='other'
            evidence = {}
            if status == 'product_stage':
                from startup_diagnostic import STAGES
                if type(result.get('stage')) is str and result['stage'] in STAGES:
                    evidence['stage'] = result['stage']
            if status == 'product_count':
                for key in ('pongs','preauth_rejected','sessions_closed'):
                    n = result.get(key)
                    if type(n) is int and 0 <= n <= 2**63-1:
                        evidence[key] = n
                for key in ('ended','pin_rejected'):
                    if type(result.get(key)) is bool:
                        evidence[key] = result[key]
            if status == 'desktop_status':
                if result.get('phase') in ('idle','waiting','approved','denied','revoked','expired','declined','fault','stopped'):
                    evidence['desktop_phase'] = result['phase']
            if status == 'desktop_state':
                for key in ('frames','inputs','releases','held'):
                    n = result.get(key)
                    if type(n) is int and 0 <= n <= 2**63-1:
                        evidence[key] = n
                for key in ('stop_requested','ended','failed'):
                    if type(result.get(key)) is bool:
                        evidence[key] = result[key]
            if status == 'controller_started' and type(result.get('passed')) is bool:
                evidence['passed'] = result['passed']
            if status == 'request_rejected' and result.get('reason') in ('child_exchange_rejected','grant_observation_rejected','request_rejected'):
                evidence['reason'] = result['reason']
            if status == 'device_result' and action in ('approve', 'status', 'revoke'):
                passed = result.get('passed')
                if type(passed) is bool:
                    evidence['passed'] = passed
                device = result.get('device')
                if passed is True and isinstance(device, dict):
                    state = device.get('status')
                    if state in ('pending', 'approved', 'denied', 'revoked'):
                        evidence['device_state'] = state
                    evidence['grant_present'] = 'grant' in device
            self.event('returned',op,action,status,evidence)
            return result
