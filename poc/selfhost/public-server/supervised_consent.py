"""Private current-product consent validation; no legacy control-plane imports.

Endpoints are trusted callables taking installeddevice.Request dictionaries.
Never log replies: invitations/grants stay in memory. This episode deliberately
does NOT provision, since provision seals the ordinary product business path.
"""
import time


def require(ok, label):
    if not ok:
        raise RuntimeError(label)


def device(endpoint, action, **fields):
    reply = endpoint({"Op": "device", "Device": {"action": action, **fields}})
    require(isinstance(reply, dict) and reply.get("status") == "device_result"
            and reply.get("passed") is True and isinstance(reply.get("device"), dict),
            "consent_private_device_reply_rejected")
    return reply["device"]


def close_request(target, controller, request_id):
    # Resolve uncertain approve receipts by signed STATUS, never repeat approve.
    current = device(target, "status", request_id=request_id)
    require(current.get("request_id") == request_id, "consent_cleanup_request_mismatch")
    if current.get("status") == "approved":
        grant = current.get("grant")
        require(isinstance(grant, dict) and isinstance(grant.get("claims"), dict)
                and isinstance(grant["claims"].get("id"), str) and grant["claims"]["id"],
                "consent_cleanup_grant_unknown")
        result = device(target, "revoke", grant_id=grant["claims"]["id"])
        require(result.get("status") == "revoked", "consent_revocation_unconfirmed")
    elif current.get("status") == "pending":
        result = device(target, "approve", request_id=request_id, allow=False)
        require(result.get("status") == "denied", "consent_denial_unconfirmed")
    else:
        require(current.get("status") in ("denied", "revoked"), "consent_cleanup_outcome_unknown")
    final = device(controller, "status", request_id=request_id)
    require(final.get("request_id") == request_id and final.get("status") in ("denied", "revoked")
            and "grant" not in final, "consent_terminal_status_unconfirmed")


def consent_round(target, controller, *, profile_expires_at, clock=time.time):
    """Only device consent/denial/revocation, no network or payload admission.

    Require fresh build validity BEFORE registration. Complete or fail closed;
    callers still own endpoint processes/tunnels and must join them afterwards.
    Returns public boolean evidence only, never the grant or invitation.
    """
    require(type(profile_expires_at) is int and profile_expires_at-clock() >= 180,
            "consent_profile_window_insufficient")
    owner = device(target, "register")
    peer = device(controller, "register")
    require(owner.get("status") == peer.get("status") == "registered"
            and isinstance(owner.get("device_id"), str) and owner["device_id"]
            and isinstance(peer.get("device_id"), str) and peer["device_id"]
            and owner["device_id"] != peer["device_id"], "consent_subjects_not_distinct")
    require(isinstance(owner.get("device_code"), str) and owner["device_code"], "consent_target_code_missing")
    approved_seen = False
    for allow in (False, True):
        invitation = device(target, "invite")
        require(isinstance(invitation.get("code"), str) and invitation["code"], "consent_invite_missing")
        pending = device(controller, "claim", target_code=owner["device_code"], code=invitation["code"])
        request_id = pending.get("request_id")
        require(isinstance(request_id, str) and request_id, "consent_request_unknown")
        try:
            require(pending.get("status") == "pending" and "grant" not in pending, "consent_claim_auto_approved")
            incoming = device(target, "incoming")
            require(incoming.get("request_id") == request_id and incoming.get("controller_id") == peer["device_id"],
                    "consent_incoming_subject_mismatch")
            foreign = controller({"Op": "device", "Device": {"action": "approve", "request_id": request_id, "allow": True}})
            require(isinstance(foreign, dict) and foreign.get("status") == "device_result"
                    and foreign.get("passed") is False, "consent_self_approval_not_rejected")
            result = device(target, "approve", request_id=request_id, allow=allow)
            received = device(controller, "status", request_id=request_id)
            require(result.get("request_id") == received.get("request_id") == request_id, "consent_response_request_mismatch")
            if not allow:
                require(result.get("status") == received.get("status") == "denied"
                        and "grant" not in result and "grant" not in received, "consent_denial_granted_access")
            else:
                grant = result.get("grant")
                require(result.get("status") == received.get("status") == "approved"
                        and isinstance(grant, dict) and grant == received.get("grant"), "consent_grants_differ")
                claims = grant.get("claims", {})
                require(isinstance(claims, dict) and claims.get("target") == owner["device_id"] and claims.get("controller") == peer["device_id"]
                        and type(claims.get("expires_at")) is int and claims["expires_at"] > clock(), "consent_grant_subject_or_expiry")
                # Signature validation remains the native product's obligation
                # before provision. Python matching is NOT cryptographic proof.
                approved_seen = True
        finally:
            close_request(target, controller, request_id)
    require(approved_seen, "consent_approval_not_observed")
    return {"denial_verified": True, "owner_approval_observed": True, "terminal_revocation_verified": True}
