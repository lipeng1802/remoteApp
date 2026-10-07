"""Pure assertions for the new native supervisor coordinator (no I/O)."""

def signed_refresh_denied(reply):
    # A failed helper request must not carry a usable positive snapshot.
    return (reply.get("passed") is False and
            not reply.get("network_state", {}).get("claims", {}).get("active", False))

def recovery_still_latched(status, reply):
    return status.get("network_ready") is False and signed_refresh_denied(reply)

def revocation_not_active(reply):
    if reply.get("passed") is False:
        return signed_refresh_denied(reply)
    return (reply.get("passed") is True and
            reply.get("network_state", {}).get("claims", {}).get("active") is False)
