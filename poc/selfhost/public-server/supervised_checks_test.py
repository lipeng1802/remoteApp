import unittest
from supervised_checks import signed_refresh_denied, recovery_still_latched, revocation_not_active

class SupervisorChecksTests(unittest.TestCase):
    def test_missing_and_positive_are_not_denial(self):
        for value in ({}, {"passed":True}, {"passed":False,"network_state":{"claims":{"active":True}}}):
            self.assertFalse(signed_refresh_denied(value))
        self.assertTrue(signed_refresh_denied({"passed":False}))

    def test_recovery_requires_explicit_closed_marker(self):
        for status in ({}, {"network_ready":True}, {"network_ready":0}):
            self.assertFalse(recovery_still_latched(status,{"passed":False}))
        self.assertTrue(recovery_still_latched({"network_ready":False},{"passed":False}))

    def test_revocation_accepts_only_denial_or_explicit_inactive(self):
        self.assertTrue(revocation_not_active({"passed":False}))
        self.assertTrue(revocation_not_active({"passed":True,"network_state":{"claims":{"active":False}}}))
        for value in ({}, {"passed":True}, {"passed":True,"network_state":{"claims":{"active":True}}}):
            self.assertFalse(revocation_not_active(value))

if __name__ == "__main__":
    unittest.main()
