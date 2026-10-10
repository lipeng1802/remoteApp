"""Coordination regressions only; not real two-machine outage acceptance."""
import unittest
from unittest.mock import patch
from compiled_products import fault_round


class CompiledFaultTests(unittest.TestCase):
    def run_round(self, failure=None):
        events = []
        binding = dict(controller_ip="100.120.0.2", target_ip="100.120.0.1")
        policy = {"acls": [dict(action="accept", src=[binding["controller_ip"]],
                               dst=[binding["target_ip"] + ":47476"])]}

        class Transport:
            def admin(self, *args):
                events.append("policy")
                return policy
            def check(self, ok, label):
                if not ok:
                    raise RuntimeError(label)

        class Coordinator:
            def request(self, op):
                events.append(op)
                if op == "freeze":
                    return dict(status="frozen", old_policy_retained=True,
                                https_503=1 if failure == "fake_ack" else True)
                return dict(status="recovered", explicit_restart=True, same_verifier=True)

        class Pair:
            def join_fault(self):
                events.append("actual_join")
                if failure == "join":
                    raise RuntimeError("unjoined")
            def assert_fault_latched(self):
                events.append("latched")
                if failure == "reopened":
                    raise RuntimeError("reopened")

        class API:
            def request(self, op):
                events.append("signed_state")
                return dict(passed=True, network_state=dict(claims=dict(
                    active=1 if failure == "fake_state" else True, binding=binding)))

        try:
            with patch("compiled_products.time.sleep"):
                fault_round(Transport(), Coordinator(), [API(), API()],
                            dict(binding=binding), Pair())
        finally:
            self.events = events

    def test_fault_join_precedes_recovery_and_old_owners_never_restart(self):
        self.run_round()
        self.assertLess(self.events.index("freeze"), self.events.index("actual_join"))
        self.assertLess(self.events.index("actual_join"), self.events.index("recover"))
        self.assertEqual(self.events.count("latched"), 2)
        self.assertEqual(self.events.count("signed_state"), 2)

    def test_missing_join_never_recovers_as_success(self):
        with self.assertRaises(RuntimeError):
            self.run_round("join")
        self.assertNotIn("recover", self.events)

    def test_fake_ack_does_not_join_or_recover(self):
        with self.assertRaises(RuntimeError):
            self.run_round("fake_ack")
        self.assertNotIn("actual_join", self.events)
        self.assertNotIn("recover", self.events)

    def test_numeric_state_or_old_owner_reopen_is_not_acceptance(self):
        for failure in ("fake_state", "reopened"):
            with self.subTest(failure=failure), self.assertRaises(RuntimeError):
                self.run_round(failure)


if __name__ == "__main__":
    unittest.main()
