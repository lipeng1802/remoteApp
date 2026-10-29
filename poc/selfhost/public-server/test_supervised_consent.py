import copy
import unittest
from supervised_consent import consent_round


class Pair:
    def __init__(self, fault=None):
        self.requests = {}
        self.calls = []
        self.fault = fault
        self.approval_attempts = 0
        self.revoked = []

    def endpoint(self, role):
        def call(frame):
            assert frame["Op"] == "device", "must never provision from consent episode"
            b = frame["Device"]
            action = b["action"]
            self.calls.append((role, copy.deepcopy(b)))
            reply = {"status": "device_result", "passed": True}
            result = {}
            if action == "register":
                result = {"status": "registered", "device_id": role, "device_code": "target-code"}
            elif action == "invite":
                result = {"status": "inviting", "code": "PRIVATE-INVITE"}
            elif action == "claim":
                request_id = "r" + str(len(self.requests))
                result = {"status": "pending", "request_id": request_id}
                self.requests[request_id] = copy.deepcopy(result)
            elif action == "incoming":
                request_id = list(self.requests)[-1]
                result = {"status": "pending", "request_id": request_id, "controller_id": "controller"}
                if self.fault == "foreign_incoming": result["controller_id"] = "other"
            elif action == "approve":
                if role != "target":
                    reply["passed"] = 0 if self.fault == "integer_false" else False
                    return reply
                request_id = b["request_id"]
                if b["allow"]:
                    self.approval_attempts += 1
                    self.requests[request_id] = {"status": "approved", "request_id": request_id,
                        "grant": {"claims": {"id": "g"+request_id, "target": "target", "controller": "controller", "expires_at": 1500}, "signature": "PRIVATE-GRANT"}}
                    if self.fault == "wrong_subject": self.requests[request_id]["grant"]["claims"]["target"] = "other"
                    if self.fault == "expired": self.requests[request_id]["grant"]["claims"]["expires_at"] = 900
                    if self.fault == "lost_approve": raise RuntimeError("private_reply_lost")
                else:
                    self.requests[request_id] = {"status": "denied", "request_id": request_id}
                result = copy.deepcopy(self.requests[request_id])
            elif action == "status":
                result = copy.deepcopy(self.requests[b["request_id"]])
                if role == "controller" and result["status"] == "approved":
                    if self.fault == "different_grant": result["grant"]["claims"]["id"] = "foreign"
            elif action == "revoke":
                if self.fault == "lost_revoke": raise RuntimeError("revocation_unknown")
                self.revoked.append(b["grant_id"])
                for request_id, value in self.requests.items():
                    if value.get("grant", {}).get("claims", {}).get("id") == b["grant_id"]:
                        self.requests[request_id] = {"status": "revoked", "request_id": request_id}
                result = {"status": "revoked"}
            else: raise AssertionError("unapproved operation")
            reply["device"] = result
            return reply
        return call


class ConsentBoundaries(unittest.TestCase):
    def run_pair(self, pair, expiry=1500):
        return consent_round(pair.endpoint("target"), pair.endpoint("controller"), profile_expires_at=expiry, clock=lambda: 1000)

    def test_success_returns_only_public_evidence_and_all_requests_terminal(self):
        p = Pair()
        result = self.run_pair(p)
        self.assertTrue(all(type(v) is bool and v for v in result.values()))
        self.assertEqual([r["status"] for r in p.requests.values()], ["denied", "revoked"])
        self.assertEqual(p.approval_attempts, 1)

    def test_lost_approval_is_resolved_by_status_and_exact_revoke_not_reapproval(self):
        p = Pair("lost_approve")
        with self.assertRaisesRegex(RuntimeError, "private_reply_lost"): self.run_pair(p)
        self.assertEqual(p.approval_attempts, 1)
        self.assertEqual(p.revoked, ["gr1"])
        self.assertEqual(p.requests["r1"], {"status": "revoked", "request_id": "r1"})

    def test_bad_observations_never_skip_cleanup(self):
        for fault in ("foreign_incoming", "integer_false", "different_grant", "wrong_subject", "expired"):
            with self.subTest(fault=fault):
                p = Pair(fault)
                with self.assertRaises(RuntimeError): self.run_pair(p)
                self.assertTrue(all(v["status"] in ("denied", "revoked") for v in p.requests.values()))

    def test_unknown_revoke_cannot_be_reported_as_success(self):
        p = Pair("lost_revoke")
        with self.assertRaisesRegex(RuntimeError, "revocation_unknown"): self.run_pair(p)
        self.assertEqual(p.requests["r1"]["status"], "approved")

    def test_insufficient_or_boolean_expiry_refuses_before_registration(self):
        for expiry in (1100, True):
            p = Pair()
            with self.assertRaisesRegex(RuntimeError, "profile_window"): self.run_pair(p, expiry)
            self.assertEqual(p.calls, [])


if __name__ == "__main__": unittest.main()
