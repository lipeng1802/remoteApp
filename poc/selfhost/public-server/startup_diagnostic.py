"""Fixed TEST startup labels. Never persist a raw response or exception."""
import json
import os

STAGES=frozenset(('target_tls','target_listen','target_secret','target_serve',
    'controller_listen','controller_dial','controller_secret','controller_tls',
    'controller_accept','controller_bridge','unreported'))


def label(endpoint):
    try:
        value=endpoint({'Op':'product_stage'})
        if type(value) is dict and set(value)=={'status','stage'} and value['status']=='product_stage' and type(value['stage']) is str and value['stage'] in STAGES:
            return value['stage']
    except Exception:
        pass
    return 'unavailable'


def capture(work,target,controller):
    # Exclusive evidence: an old outcome cannot be silently overwritten.
    with (work/'startup-diagnostic.json').open('x') as output:
        value={'version':1,'roles':{'target':label(target),'controller':label(controller)}}
        json.dump(value,output,sort_keys=True);output.flush();os.fsync(output.fileno())
