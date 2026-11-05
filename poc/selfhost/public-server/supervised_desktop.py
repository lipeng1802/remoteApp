"""One GUI TEST episode over already verified, resident owner endpoints."""
import hashlib
import re
import time

from supervised_consent import require
from supervised_session import desktop_live_state, parallel, quiesce_desktop, start_after_target_ready


GUI_EVIDENCE = frozenset(("windows_frames_visible", "windows_mouse_effect", "windows_keyboard_effect"))
DESKTOP_EVIDENCE = GUI_EVIDENCE | {"active_watch_both", "products_quiesced_both", "owner_revocation_verified"}
TERMINAL = ("denied", "revoked")


def canonical_hex(value, length):
    return type(value) is str and re.fullmatch("[0-9a-f]{" + str(length) + "}", value) is not None


def subject(bootstrap):
    require(type(bootstrap) is dict and bootstrap.get("desktop_permission") == "desktop-control"
            and canonical_hex(bootstrap.get("public_key"), 64), "desktop_bootstrap_rejected")
    return hashlib.sha256(bytes.fromhex(bootstrap["public_key"])).hexdigest()


def progress(value):
    require(type(value) is dict and value.get("status") == "desktop_status",
            "desktop_progress_unconfirmed")
    phase = value.get("phase")
    request = value.get("request_id")
    allowed = {"status", "phase"}
    if "request_id" in value:
        require(canonical_hex(request, 32), "desktop_request_invalid")
        allowed.add("request_id")
    if phase == "approved":
        allowed.add("grant")
    require(set(value) == allowed and (
        phase == "waiting"
        or phase in ("fault", "stopped")
        or phase == "idle" and request is None
        or phase in ("approved", "denied", "revoked", "expired", "declined") and request is not None
    ), "desktop_progress_invalid")
    return phase, request, value.get("grant")


def approved_grant(grant, target, controller):
    require(type(grant) is dict and set(grant) == {"claims", "signature"}
            and canonical_hex(grant["signature"], 128), "desktop_grant_invalid")
    claims = grant["claims"]
    require(type(claims) is dict
            and set(claims) == {"version", "id", "target", "controller", "permission", "issued_at", "expires_at"}
            and type(claims["version"]) is int and claims["version"] == 1
            and canonical_hex(claims["id"], 32)
            and claims["target"] == target and claims["controller"] == controller
            and claims["permission"] == "desktop-control"
            and type(claims["issued_at"]) is int and type(claims["expires_at"]) is int
            and 0 < claims["issued_at"] < claims["expires_at"], "desktop_grant_scope_rejected")
    return grant


def desktop_round(target, controller, *, target_bootstrap, controller_bootstrap, profile_expires_at,
                  observe_gui, request_stop, clock=time.time, monotonic=time.monotonic, sleep=time.sleep):
    """Run only the exact control scope. Both UI decisions stay in the native owners.

    observe_gui and request_stop are trusted bounded operator adapters. Evidence
    of the actual displayed image and the Windows effects of mouse and keyboard
    must come from direct observation. This function never stops the holders.
    """
    target_id, controller_id = subject(target_bootstrap), subject(controller_bootstrap)
    require(target_id != controller_id and callable(observe_gui) and callable(request_stop),
            "desktop_roles_or_observers_invalid")
    require(type(profile_expires_at) is int and profile_expires_at - clock() >= 240,
            "desktop_profile_window_insufficient")
    endpoints = (target, controller)
    request_id = None
    target_grant = None
    expires_at = profile_expires_at
    consent_deadline = monotonic() + 120
    try:
        for endpoint in endpoints:
            require(monotonic() < consent_deadline and clock() < profile_expires_at,
                    "desktop_consent_deadline")
            require(endpoint({"Op": "desktop_begin"}) == {"status": "desktop_pending"},
                    "desktop_begin_unconfirmed_preserve_owners")
        while True:
            require(monotonic() < consent_deadline and clock() < profile_expires_at,
                    "desktop_consent_deadline")
            statuses = []
            for role, endpoint in enumerate(endpoints):
                phase, request, grant = progress(endpoint({"Op": "desktop_status"}))
                if request is not None:
                    require(request_id in (None, request), "desktop_request_changed")
                    request_id = request
                if phase == "approved":
                    grant = approved_grant(grant, target_id, controller_id)
                    if role == 0:
                        require(target_grant is None or target_grant == grant, "desktop_target_grant_changed")
                        target_grant = grant
                require(phase not in ("idle", "expired", "declined", "fault", "stopped"),
                        "desktop_consent_unconfirmed_preserve_owners")
                statuses.append((phase, request, grant))
            require(monotonic() < consent_deadline and clock() < profile_expires_at,
                    "desktop_consent_deadline")
            first, second = statuses
            if first[0] in TERMINAL and first == second:
                terminal = parallel(endpoints, {"Op": "preprovision_terminal"})
                require(all(type(r) is dict and r == {"status": "preprovision_terminal", "passed": True}
                            and r["passed"] is True for r in terminal), "desktop_terminal_unconfirmed")
                target_grant = None
                return {"preprovision_terminal": True}
            if first[0] == second[0] == "approved":
                require(first == second, "desktop_approval_mismatch")
                break
            sleep(.5)

        expires_at = min(profile_expires_at, target_grant["claims"]["expires_at"])
        require(expires_at - clock() >= 90, "desktop_grant_window_insufficient")
        for r in parallel(endpoints, {"Op": "network", "Network": {"action": "grant_clock", "grant": target_grant}}):
            require(type(r) is dict and r == {"status": "network_authorization", "phase": "grant_clock",
                                            "passed": True, "active": False}
                    and r["passed"] is True and r["active"] is False, "desktop_native_clock_unconfirmed")
        require(clock() < expires_at, "desktop_grant_expired")
        for r in parallel(endpoints, {"Op": "provision", "Network": {"grant": target_grant}}):
            require(type(r) is dict and r == {"status": "network_watch", "passed": True, "active": True}
                    and r["passed"] is True and r["active"] is True, "desktop_active_watch_unconfirmed")
        start_after_target_ready(target, controller, expires_at=expires_at, clock=clock,
                                 monotonic=monotonic, sleep=sleep, desktop=True)
        evidence = observe_gui(expires_at)
        require(type(evidence) is dict and set(evidence) == GUI_EVIDENCE
                and all(v is True for v in evidence.values()) and clock() < expires_at,
                "desktop_direct_observation_unconfirmed")
        for state in parallel(endpoints, {"Op": "desktop_state"}):
            live = desktop_live_state(state)
            require(live is not None and live["frames"] > 0 and live["inputs"] > 0,
                    "desktop_gui_activity_unconfirmed")
        require(clock() < expires_at and request_stop(expires_at) is True, "desktop_stop_unconfirmed")
        quiesce_desktop(target, controller, expires_at=expires_at, clock=clock)
    finally:
        if target_grant is not None:
            # The supervisor has already observed this precise target grant.
            # Revoke once even after an uncertain provision/stop. The holder
            # stays resident, including when that independent reply is lost.
            try:
                r = target({"Op": "revoke_session"})
                require(type(r) is dict and r == {"status": "session_revoked", "passed": True}
                        and r["passed"] is True, "desktop_revocation_unconfirmed_preserve_owners")
            except Exception:
                raise RuntimeError("desktop_revocation_unconfirmed_preserve_owners") from None
    return dict.fromkeys(DESKTOP_EVIDENCE, True)
