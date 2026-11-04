"""Fixed TEST startup labels. Never persist a raw response or exception."""
import json
import os

STAGES=frozenset(('target_tls','target_listen','target_secret','target_serve',
    'controller_listen','controller_dial','controller_secret','controller_tls',
    'secret_target_config','secret_target_authorize','secret_target_hello','secret_target_nonce','secret_target_sign','secret_target_welcome','secret_target_proof','secret_target_reauthorize','secret_target_send','secret_controller_config','secret_controller_authorize','secret_controller_hello','secret_controller_welcome','secret_controller_verify','secret_controller_sign','secret_controller_proof','secret_controller_reauthorize','secret_controller_receive','secret_controller_canonical','deadline_route_query_failed','deadline_route_peer_missing','deadline_route_not_ready','deadline_route_engine_pending','deadline_route_engine_present','deadline_route_engine_present_send_unavailable','deadline_route_engine_present_send_filter_drop','deadline_route_engine_present_send_observed','deadline_route_peer_send_observed','deadline_route_discovery_unavailable','deadline_route_discovery_direct','deadline_route_discovery_udp_relay','deadline_route_discovery_derp','deadline_controller_syn_observed','deadline_controller_syn_reply_observed','deadline_route_derp_discovery_tsmp_observed','deadline_route_derp_discovery_tsmp_unconfirmed','deadline_route_peer_send_direct_selected','deadline_route_peer_send_udp_relay_selected','deadline_route_peer_send_unchanged','deadline_route_engine_present_send_unchanged','controller_route_wait','controller_route_denied','deadline_ready_self_relay_unknown','deadline_ready_peer_relay_unknown','deadline_ready_handshake_unseen','controller_accept','controller_bridge','unreported','lifecycle_state_rejected','lifecycle_inactive','lifecycle_gate_rejected','lifecycle_session_ended','target_gate_denied','target_local_failed','target_self_denied','target_listen_failed','target_listening','target_accept_ended','target_receive_unavailable','target_receive_filter_drop','target_receive_observed','target_receive_unchanged','target_tcp_accepted','target_syn_observed','target_syn_not_observed','target_syn_stream_unavailable','target_syn_open_failed','target_syn_parent_ended','target_syn_window_ended','target_syn_header_unavailable','target_syn_reply_observed','target_filter_unavailable','target_filter_match','target_filter_no_ip_match','target_accept_denied','target_peer_denied','controller_gate_denied','controller_local_failed','controller_upgrade','controller_peer','controller_peer_denied','overlay_request_deadline','overlay_request_canceled','overlay_request_eof','overlay_request_refused','overlay_request_reset','overlay_request_timeout','overlay_request_failed','overlay_upgrade_rejected','overlay_route_missing','overlay_response_invalid','overlay_upgraded','overlay_bad_upgrade','overlay_permission_denied','overlay_resolve_failed','overlay_connect_refused','overlay_connect_timeout','overlay_connect_failed'))


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
