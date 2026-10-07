import unittest
from supervised_checks import signed_refresh_denied, recovery_still_latched, revocation_not_active, bound_projection_matches

class SupervisorChecksTests(unittest.TestCase):
    def test_bound_projection_requires_exact_verified_pair_and_actual_acl(self):
        binding={"target_node":"t","controller_node":"c","target_ip":"100.120.0.1","controller_ip":"100.120.0.2"}
        policy={"acls":[{"action":"accept","src":["100.120.0.2"],"dst":["100.120.0.1:47476"]}]}
        reply={"passed":True,"network_state":{"claims":{"active":True,"binding":binding}}}
        self.assertTrue(bound_projection_matches(reply,binding,policy))
        self.assertFalse(bound_projection_matches(reply,binding,{"acls":[]}))
        self.assertFalse(bound_projection_matches(reply,binding,{"acls":policy["acls"]+[dict(policy["acls"][0],src=["*"])]}))
        self.assertFalse(bound_projection_matches(reply,dict(binding,target_node="wrong"),policy))
        self.assertFalse(bound_projection_matches({},binding,policy))
        self.assertFalse(bound_projection_matches({"passed":True,"network_state":{"claims":{"binding":binding}}},binding,policy))
        reply["network_state"]["claims"]["active"]=False
        self.assertTrue(bound_projection_matches(reply,binding,{"acls":[]}))
        self.assertFalse(bound_projection_matches(reply,binding,policy))

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
