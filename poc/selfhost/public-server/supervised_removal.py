"""Private TEST removal coordination, no identity/path selectors or retries."""
import json
import os
import re
import time
from concurrent.futures import ThreadPoolExecutor


def checked_receipt(v):
    if (type(v) is not dict or set(v)!={'status','epoch','transaction','preserved','joined','zero_exit'}
        or v['status']!='files_removed' or type(v['epoch']) is not int or not 1<=v['epoch']<=2**53-1
        or type(v['transaction']) is not str or not re.fullmatch('[0-9a-f]{32}',v['transaction'])
        or any(v[k] is not True for k in ('preserved','joined','zero_exit'))):
        raise RuntimeError('program_removal_receipt_rejected')
    return v


def checked_bootstrap(v):
    if (type(v) is not dict or type(v.get('removal_protocol')) is not int or v['removal_protocol']!=1
        or type(v.get('uninstaller_process_id')) is not int or v['uninstaller_process_id']<=0
        or v['uninstaller_process_id']==v.get('supervisor_process_id',v.get('process_id'))):
        raise RuntimeError('program_removal_bootstrap_rejected')


def pair(work,target,controller,*,expires_at,clock=time.time,monotonic=time.monotonic,sleep=time.sleep):
    # Both callbacks start exactly once, then only observe the SAME native work.
    with (work/'program-removal-intent.json').open('x') as f:
        json.dump({'version':1,'phase':'outcome_unknown'},f);f.flush();os.fsync(f.fileno())
    with ThreadPoolExecutor(max_workers=2) as workers:
        pending=[workers.submit(start) for start,status in (target,controller)]
        results=[];failed=False
        for future in pending:
            try:results.append(future.result())
            except Exception:failed=True
        if failed:raise RuntimeError('program_removal_start_unknown_preserve_owners')
    if any(v!={'status':'removal_pending'} for v in results):raise RuntimeError('program_removal_start_rejected')
    deadline=monotonic()+90;receipts={}
    while len(receipts)<2 and monotonic()<deadline and clock()<expires_at:
        for role,(start,status) in zip(('target','controller'),(target,controller)):
            if role in receipts:continue
            value=status()
            if value=={'status':'removal_pending'}:continue
            receipts[role]=checked_receipt(value)
        if len(receipts)<2:sleep(.25)
    if len(receipts)!=2:raise RuntimeError('program_removal_unknown_preserve_owners')
    with (work/'program-removal-result.json').open('x') as f:
        json.dump({'version':1,'roles':receipts},f,sort_keys=True);f.flush();os.fsync(f.fileno())
    return receipts
