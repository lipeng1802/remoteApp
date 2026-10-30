"""Fixed TEST startup labels. Never persist a raw response or exception."""
import json
import os

STAGES=frozenset(('target_tls','target_listen','target_secret','target_serve',
    'controller_listen','controller_dial','controller_secret','controller_tls',
    'controller_accept','controller_bridge','unreported','target_gate_denied','target_local_failed','target_self_denied','target_listen_failed','target_listening','target_accept_denied','target_peer_denied','controller_gate_denied','controller_local_failed','controller_upgrade','controller_peer','controller_peer_denied','overlay_request_failed','overlay_upgrade_rejected','overlay_route_missing','overlay_response_invalid','overlay_upgraded','overlay_bad_upgrade','overlay_permission_denied','overlay_resolve_failed','overlay_connect_refused','overlay_connect_timeout','overlay_connect_failed'))


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
