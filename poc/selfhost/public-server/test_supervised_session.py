import threading
import unittest

from supervised_session import active_round, counts
from test_supervised_consent import Pair


class ActivePair(Pair):
    def __init__(self, fault=None):
        super().__init__()
        self.session_fault = fault
        self.elapsed = 0
        self.barrier = threading.Barrier(2)
        self.provision_finished = set()
        self.revoke_calls = 0
        self.count_calls = 0

    def sleep(self, seconds):
        self.elapsed += seconds

    def endpoint(self, role):
        ordinary = super().endpoint(role)

        def call(frame):
            if frame["Op"] == "device":
                if self.provision_finished:
                    raise AssertionError("sealed child business must never be reused")
                return ordinary(frame)
            if frame["Op"] == "network":
                assert frame["Network"]["action"] == "grant_clock"
                return {"status": "network_authorization", "phase": "grant_clock", "active": False,
                        "passed": False if self.session_fault == "clock" else True}
            if frame["Op"] == "provision":
                self.barrier.wait(timeout=2)
                self.provision_finished.add(role)
                if self.session_fault == "provision" and role == "controller":
                    raise RuntimeError("PRIVATE-GRANT must not leak")
                return {"status": "network_watch", "passed": True,
                        "active": 1 if self.session_fault == "integer_active" else True}
            if frame["Op"] == "product_count":
                self.count_calls += 1
                if self.session_fault == "unavailable" and self.elapsed < 1:
                    return {"status": "product_unavailable"}
                n = int(self.elapsed) + 1
                if self.session_fault == "constant": n = 1
                if self.session_fault == "regression" and self.elapsed >= 2: n = 0
                if self.session_fault == "boolean": n = True
                return {"status": "product_count", "pongs": n, "ended": self.session_fault == "ended",
                        "pin_rejected": False}
            if frame["Op"] == "revoke_session":
                assert frame == {"Op": "revoke_session"} and role == "target"
                self.revoke_calls += 1
                assert self.provision_finished == {"target", "controller"}
                if self.session_fault == "lost_revoke":
                    raise RuntimeError("PRIVATE-RECEIPT")
                return {"status": "session_revoked", "passed": True}
            raise AssertionError("unapproved op")
        return call


class ActiveSessionBoundaries(unittest.TestCase):
    def run_pair(self, pair, expiry=1500):
        return active_round(pair.endpoint("target"), pair.endpoint("controller"),
                            profile_expires_at=expiry, clock=lambda: 1000 + pair.elapsed,
                            monotonic=lambda: pair.elapsed, sleep=pair.sleep)

    def test_parallel_provision_and_growth_across_multiple_state_leases(self):
        for fault in (None, "unavailable"):
            with self.subTest(fault=fault):
                p = ActivePair(fault)
                result = self.run_pair(p)
                self.assertEqual(p.provision_finished, {"target", "controller"})
                self.assertTrue(result["active_watch_both"])
                self.assertTrue(result["payload_increased_both"])
                self.assertTrue(result["owner_revocation_verified"])
                self.assertGreaterEqual(result["growth_samples"], 3)
                self.assertGreaterEqual(p.elapsed, 6)
                self.assertEqual(p.revoke_calls, 1)
                self.assertNotIn("grant", repr(result))

    def test_one_failed_provision_joins_both_calls_then_uses_independent_revoke(self):
        p = ActivePair("provision")
        with self.assertRaisesRegex(RuntimeError, "^session_parallel_outcome_unknown$"):
            self.run_pair(p)
        self.assertEqual(p.provision_finished, {"target", "controller"})
        self.assertEqual(p.revoke_calls, 1)
        self.assertEqual(p.count_calls, 0)

    def test_active_or_one_pong_is_never_enough(self):
        for fault in ("constant", "boolean", "regression", "ended", "integer_active"):
            with self.subTest(fault=fault):
                p = ActivePair(fault)
                with self.assertRaises(RuntimeError): self.run_pair(p)
                self.assertEqual(p.revoke_calls, 1)

    def test_uncertain_revoke_preserves_signer_and_is_not_automatically_retried(self):
        p = ActivePair("lost_revoke")
        with self.assertRaisesRegex(RuntimeError, "^session_revocation_unconfirmed_preserve_signer$"):
            self.run_pair(p)
        self.assertEqual(p.revoke_calls, 1)

    def test_before_provision_failure_uses_exact_ordinary_cleanup(self):
        p = ActivePair("clock")
        with self.assertRaisesRegex(RuntimeError, "session_native_clock_unconfirmed"):
            self.run_pair(p)
        self.assertFalse(p.provision_finished)
        self.assertEqual(p.revoked, ["gr0"])
        self.assertEqual(p.revoke_calls, 0)

    def test_short_profile_never_registers(self):
        p = ActivePair()
        with self.assertRaisesRegex(RuntimeError, "session_profile_window_insufficient"):
            self.run_pair(p, 1100)
        self.assertEqual(p.calls, [])

    def test_counter_schema_rejects_secret_and_integer_bool(self):
        for bad in ({"status": "product_count", "pongs": 2, "ended": 0, "pin_rejected": False},
                    {"status": "product_count", "pongs": 2, "ended": False, "pin_rejected": False, "key": "PRIVATE"}):
            with self.subTest(bad=bad):
                with self.assertRaisesRegex(RuntimeError, "session_payload_reply_rejected"):
                    counts((lambda frame: bad, lambda frame: bad))


if __name__ == "__main__": unittest.main()
