"""Strict local-only join evidence. Never grants remote retirement permission."""
import os
import stat
from supervised_requests import reply


def receipt(value,role,public):
    if role not in ('mac','windows'):raise RuntimeError('local_cancel_role_rejected')
    expected={'version':1,'status':'owner_local_cancel_closed','joined':True,'fault':True,
              'exit_code':1,'remote_revocation_verified':False}
    keys=('process_id',) if role=='windows' else ('process_id','supervisor_process_id')
    if type(public.get('local_cancel_protocol')) is not int or public['local_cancel_protocol']!=1:
        raise RuntimeError('local_cancel_protocol_unavailable')
    if any(type(public.get(k)) is not int or public[k]<=0 for k in keys):
        raise RuntimeError('local_cancel_identity_rejected')
    if role=='windows':expected['process_id']=public['process_id']
    else:expected.update(holder_process_id=public['process_id'],supervisor_process_id=public['supervisor_process_id'])
    if type(value) is not dict or set(value)!=set(expected):raise RuntimeError('local_cancel_receipt_rejected')
    if any(type(value[k]) is not type(v) or value[k]!=v for k,v in expected.items()):
        raise RuntimeError('local_cancel_receipt_rejected')
    return value


def mac_file(path,public):
    info=path.lstat()
    if not stat.S_ISREG(info.st_mode) or info.st_uid!=os.getuid() or stat.S_IMODE(info.st_mode)!=0o600 or info.st_size>8192:
        raise RuntimeError('local_cancel_file_rejected')
    return receipt(reply(path.read_bytes()),'mac',public)
