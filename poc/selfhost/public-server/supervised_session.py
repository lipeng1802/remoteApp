"""TEST management coordination for the current supervised product protocol.

Private grant material remains in memory. This observes native product telemetry;
it does not interpret ACTIVE replies as TLS acceptance or perform root deletion.
Endpoint calls must be bounded and their owners remain the caller's obligation.
"""
from concurrent.futures import ThreadPoolExecutor
import time

from supervised_consent import close_request, device, require


def parallel(endpoints, frame):
    # Join both management calls even if one fails. Never abandon a still-running
    # provision future and pretend no network enrollment could have happened.
    with ThreadPoolExecutor(max_workers=2) as workers:
        pending = [workers.submit(endpoint, frame) for endpoint in endpoints]
        results = []
        failed = False
        for future in pending:
            try:
                results.append(future.result())
            except Exception:
                failed = True
        if failed:
            raise RuntimeError("session_parallel_outcome_unknown") from None
        return results


def counts(endpoints):
    values = parallel(endpoints, {"Op": "product_count"})
    output = []
    for value in values:
        if value == {"status": "product_unavailable"}:
            output.append(None)
            continue
        require(isinstance(value, dict) and value.get("status") == "product_count"
                and not (set(value) - {"status", "pongs", "preauth_rejected", "sessions_closed",
                                       "ended", "result", "pin_rejected"})
                and type(value.get("pongs")) is int and value["pongs"] >= 0
                and value.get("ended") is False and value.get("pin_rejected") is False,
                "session_payload_reply_rejected")
        # A controller's non-pending terminal result is never an active session.
        require("result" not in value or value["result"] == "pending", "session_payload_ended")
        for field in ("preauth_rejected", "sessions_closed"):
            require(field not in value or type(value[field]) is int and value[field] >= 0,
                    "session_payload_counter_rejected")
        output.append(value["pongs"])
    return output


def observe_growth(endpoints, *, monotonic=time.monotonic, sleep=time.sleep,
                   clock=time.time, expires_at):
    deadline = monotonic() + 25
    previous = None
    first_seen = None
    increases = 0
    while monotonic() < deadline and clock() < expires_at:
        current = counts(endpoints)
        if previous is None:
            if all(type(n) is int and n > 0 for n in current):
                previous, first_seen = current, monotonic()
        else:
            require(all(type(n) is int and n >= old for n, old in zip(current, previous)),
                    "session_payload_lost_or_counter_regressed")
            if all(n > old for n, old in zip(current, previous)):
                increases += 1
                previous = current
            # Network state lives only three seconds. Six elapsed seconds and
            # multiple actual increases prevent one PONG/one lease acceptance.
            if increases >= 3 and monotonic() - first_seen >= 6:
                return {"payload_increased_both": True, "growth_samples": increases}
        sleep(.5)
    raise RuntimeError("session_payload_growth_unconfirmed")


def active_round(target, controller, *, profile_expires_at, clock=time.time,
                 monotonic=time.monotonic, sleep=time.sleep):
    """Fresh explicit approval -> clocks -> concurrent provision -> PONG -> revoke.

    A mutation/receipt error fails this episode. It does not stop/delete owners,
    retry an uncertain revoke, or claim all network resources have been joined.
    Caller must preserve signer access when revocation remains unconfirmed.
    """
    require(type(profile_expires_at) is int and profile_expires_at - clock() >= 240,
            "session_profile_window_insufficient")
    owner = device(target, "register")
    peer = device(controller, "register")
    require(owner.get("status") == peer.get("status") == "registered"
            and isinstance(owner.get("device_id"), str) and owner["device_id"]
            and isinstance(peer.get("device_id"), str) and peer["device_id"]
            and owner["device_id"] != peer["device_id"]
            and isinstance(owner.get("device_code"), str) and owner["device_code"],
            "session_subjects_invalid")
    invitation = device(target, "invite")
    require(isinstance(invitation.get("code"), str) and invitation["code"], "session_invite_missing")
    pending = device(controller, "claim", target_code=owner["device_code"], code=invitation["code"])
    request_id = pending.get("request_id")
    require(isinstance(request_id, str) and request_id, "session_claim_outcome_unknown")
    attempted = False
    try:
        require(pending.get("status") == "pending" and "grant" not in pending, "session_auto_approved")
        incoming = device(target, "incoming")
        require(incoming.get("request_id") == request_id and incoming.get("controller_id") == peer["device_id"],
                "session_incoming_mismatch")
        approved = device(target, "approve", request_id=request_id, allow=True)
        received = device(controller, "status", request_id=request_id)
        grant = approved.get("grant")
        require(approved.get("request_id") == received.get("request_id") == request_id
                and approved.get("status") == received.get("status") == "approved"
                and isinstance(grant, dict) and grant == received.get("grant"), "session_approval_mismatch")
        claims = grant.get("claims", {})
        require(isinstance(claims, dict) and claims.get("target") == owner["device_id"]
                and claims.get("controller") == peer["device_id"]
                and type(claims.get("expires_at")) is int and claims["expires_at"] - clock() >= 90,
                "session_grant_subject_or_expiry")
        expiry = min(profile_expires_at, claims["expires_at"])
        for result in parallel((target, controller), {"Op": "network", "Network": {"action": "grant_clock", "grant": grant}}):
            require(result == {"status": "network_authorization", "phase": "grant_clock", "passed": True, "active": False}
                    and type(result["passed"]) is bool and type(result["active"]) is bool,
                    "session_native_clock_unconfirmed")
        # Mark before submitting either call; a timeout can follow real enroll.
        attempted = True
        for result in parallel((target, controller), {"Op": "provision", "Network": {"grant": grant}}):
            require(isinstance(result, dict) and set(result) == {"status", "passed", "active"}
                    and result["status"] == "network_watch" and result["passed"] is True
                    and result["active"] is True, "session_active_watch_unconfirmed")
        result = observe_growth((target, controller), expires_at=expiry, clock=clock,
                                monotonic=monotonic, sleep=sleep)
        result["active_watch_both"] = True
    finally:
        if attempted:
            # No grant selector and no child business IPC after provision.
            try:
                revoked = target({"Op": "revoke_session"})
                require(isinstance(revoked, dict) and set(revoked) == {"status", "passed"}
                        and revoked["status"] == "session_revoked" and revoked["passed"] is True,
                        "session_revocation_unconfirmed_preserve_signer")
            except Exception:
                raise RuntimeError("session_revocation_unconfirmed_preserve_signer") from None
        else:
            # Before any provision, ordinary exact STATUS/revoke is still valid.
            close_request(target, controller, request_id)
    result["owner_revocation_verified"] = True
    return result
