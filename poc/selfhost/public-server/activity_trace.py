"""Public TEST stage evidence only: never serialize a request, reply or error."""
import json
import os
import threading


class RecordedEndpoint:
    def __init__(self,work,role,endpoint):
        if role not in ('target','controller'):raise RuntimeError('activity_trace_role_rejected')
        self.directory=work/('trace-'+role)
        self.directory.mkdir(mode=0o700)  # Existing evidence must be preserved.
        self.endpoint=endpoint;self.sequence=0;self.lock=threading.Lock()

    def event(self,phase,op,action,status=None):
        self.sequence+=1
        value={'version':1,'phase':phase,'op':op,'action':action}
        if status is not None:value['status']=status
        with (self.directory/('%06d.json'%self.sequence)).open('x') as output:
            json.dump(value,output,sort_keys=True);output.flush();os.fsync(output.fileno())

    def __call__(self,frame):
        with self.lock:
            op=frame.get('Op') if isinstance(frame,dict) else None
            if op not in ('device','network','provision','product_stage','product_count','quiesce_session','revoke_session'):op='other'
            nested=frame.get('Device' if op=='device' else 'Network',{}) if isinstance(frame,dict) else {}
            action=nested.get('action') if isinstance(nested,dict) else None
            if action not in ('register','invite','claim','incoming','approve','status','revoke','grant_clock'):action='other'
            self.event('calling',op,action)
            try:result=self.endpoint(frame)
            except Exception:
                self.event('outcome_unknown',op,action)
                raise
            status=result.get('status') if isinstance(result,dict) else None
            if status not in ('device_result','network_authorization','network_watch','product_stage','product_count','product_unavailable','session_quiesced','session_revoked','failed','unavailable','rejected'):status='other'
            self.event('returned',op,action,status)
            return result
