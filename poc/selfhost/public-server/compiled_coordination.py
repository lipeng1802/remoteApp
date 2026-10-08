"""Private coordination inputs, NOT authority or enrollment substitutes.

Only the signed API's verified real node phase may create BoundHandoff. The
compiled runtime re-verifies the grant, binding/watch and actual node identity.
Nothing in this module signs, creates keys, authorizes a gate or logs secrets.
"""
import copy
import hashlib
import ipaddress
import re


def runtime_inputs(ready, binding, grant, target_dir, controller_dir, tickets):
    """Build private Go Configs. Runtime still checks state and signatures."""
    for name in ("target", "controller"):
        if not re.fullmatch(r"[0-9a-f]{64}", ready.get(name, "")):
            raise RuntimeError("invalid_runtime_identity")
    if ready["target"] == ready["controller"] or not re.fullmatch(r"nodekey:[0-9a-f]{64}", ready.get("verifier", "")):
        raise RuntimeError("invalid_runtime_pair")
    if set(binding) != {"target_node", "controller_node", "target_ip", "controller_ip"}:
        raise RuntimeError("invalid_runtime_binding")
    for name in ("target", "controller"):
        if not re.fullmatch(r"nodekey:[0-9a-f]{64}", binding[name+"_node"]):
            raise RuntimeError("invalid_runtime_node")
        ip = ipaddress.ip_address(binding[name+"_ip"])
        if ip.version != 4 or str(ip) != binding[name+"_ip"] or ip not in ipaddress.ip_network("100.64.0.0/10"):
            raise RuntimeError("invalid_runtime_address")
        identity = hashlib.sha256(bytes.fromhex(ready[name])).hexdigest()
        if grant.get("claims", {}).get(name) != identity:
            raise RuntimeError("grant_runtime_subject_mismatch")
    if binding["target_node"] == binding["controller_node"] or binding["target_ip"] == binding["controller_ip"]:
        raise RuntimeError("runtime_pair_collision")
    # Canonical paths are subsequently checked against actual state/symlinks by
    # each native Go runtime. No automatic prepare/rekey on a missing directory.
    if (not isinstance(target_dir, str) or not isinstance(controller_dir, str)
            or any(c in controller_dir for c in "\r\n\x00")
            or not re.fullmatch(r"[A-Za-z]:\\[^\r\n\x00]+", target_dir)
            or "/" in target_dir or any(part in (".", "..", "") for part in target_dir[3:].split("\\"))
            or not controller_dir.startswith("/") or any(part in (".", "..", "") for part in controller_dir[1:].split("/"))):
        raise RuntimeError("invalid_runtime_state_path")
    configs = []
    if not isinstance(tickets, (list, tuple)) or len(tickets) != 2:
        raise RuntimeError("both_private_tickets_required")
    for windows, directory, ticket in zip((True, False), (target_dir, controller_dir), tickets):
        secret = ticket.get("credential", {}).get("Secret", "")
        if not isinstance(secret, str) or not 0 < len(secret) <= 256 or any(c in secret for c in "\r\n\x00"):
            raise RuntimeError("invalid_private_enrollment_credential")
        configs.append(dict(StateDir=directory, AuthKey=secret, Verifier=ready["verifier"],
                            Target=ready["target"], Controller=ready["controller"],
                            Grant=copy.deepcopy(grant), Binding=copy.deepcopy(binding), Server=windows))
    if len(configs) != 2:
        raise RuntimeError("both_private_tickets_required")
    return dict(target=configs[0], controller=configs[1])


class BoundHandoff:
    """One explicit handoff; failed join cannot be retried into silent reuse."""
    def __init__(self, inputs, stop_bound_helpers):
        self._inputs = copy.deepcopy(inputs)
        self._stop = stop_bound_helpers
        self._attempted = False

    def take(self):
        if self._attempted:
            raise RuntimeError("bound_handoff_already_attempted")
        self._attempted = True
        # Return authkeys/state only AFTER both old node owners actually join.
        self._stop()
        return copy.deepcopy(self._inputs)
