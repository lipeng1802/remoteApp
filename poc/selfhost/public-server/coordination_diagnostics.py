"""Fixed, non-secret phase evidence; never a join or authorization receipt."""

STAGES = frozenset({
    "bound_controller_join", "bound_target_join", "bound_handoff",
    "mac_native_initialize", "target_start_send", "target_product_ready",
    "target_session_ready", "controller_start", "controller_session_ready",
    "product_observe", "controller_normal_stop", "target_normal_stop",
    "reconnect_authorization", "reconnect_target_reopen", "reconnect_mac_inspect",
    "fresh_authorization_bound_handoff",
})


def run_stage(stage, operation):
    if stage not in STAGES:
        raise RuntimeError("unknown_coordination_stage")
    print("DIAGNOSTIC coordination_" + stage + "_begin", flush=True)
    try:
        result = operation()
    except Exception:
        # No exception text, reply, argv, path, key or configuration is logged.
        print("DIAGNOSTIC coordination_" + stage + "_failed", flush=True)
        raise
    print("DIAGNOSTIC coordination_" + stage + "_completed", flush=True)
    return result
