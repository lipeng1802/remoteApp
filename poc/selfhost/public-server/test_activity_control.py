import json
from pathlib import Path
import tempfile
import unittest

from activity_control import Activity


def accepted():
    return dict(active_watch_both=True,payload_increased_both=True,growth_samples=3,
                products_quiesced_both=True,owner_revocation_verified=True)


class ActivityOwnership(unittest.TestCase):
    def setup_activity(self,directory):
        a=Activity(directory);a.save({"version":1,"phase":"owners_ready"});return a

    def test_unknown_episode_persists_before_effect_and_never_allows_stop_or_retry(self):
        with tempfile.TemporaryDirectory() as d:
            a=self.setup_activity(d)
            def episode(*args,**kwargs):
                self.assertEqual(a.read()["phase"],"exercise_outcome_unknown")
                raise RuntimeError("PRIVATE-GRANT")
            with self.assertRaisesRegex(RuntimeError,"^activity_outcome_unknown_preserve_owners_and_tunnels$"):
                a.exercise(None,None,1500,episode)
            with self.assertRaisesRegex(RuntimeError,"fresh_ready"):a.exercise(None,None,1500,episode)
            with self.assertRaisesRegex(RuntimeError,"preserve_signers"):a.stop_owners(lambda:self.fail(),lambda:self.fail())
            self.assertNotIn("PRIVATE",a.path.read_text())

    def test_exact_evidence_then_partial_join_explicit_continuation(self):
        with tempfile.TemporaryDirectory() as d:
            a=self.setup_activity(d)
            def episode(*args,**kwargs):
                self.assertIs(kwargs["quiesce_before_revoke"],True);return accepted()
            a.exercise(None,None,1500,episode)
            calls=[]
            def controller():calls.append("controller");return {"status":"owner_closed","joined":True,"zero_exit":True}
            def target():calls.append("target");raise RuntimeError("unknown stop")
            with self.assertRaisesRegex(RuntimeError,"join_unconfirmed"):a.stop_owners(target,controller)
            self.assertEqual(a.read()["joined"],{"controller":True})
            a.stop_owners(lambda:{"status":"owner_closed","joined":True,"zero_exit":True},lambda:self.fail("do not re-stop joined owner"))
            self.assertEqual(a.read()["phase"],"owners_joined_retirement_pending")
            self.assertEqual(calls,["controller","target"])

    def test_truthy_or_extra_evidence_never_authorizes_stop(self):
        for field,value in (("products_quiesced_both",1),("growth_samples",True),("secret","PRIVATE")):
            with self.subTest(field=field),tempfile.TemporaryDirectory() as d:
                a=self.setup_activity(d);result=accepted();result[field]=value
                with self.assertRaisesRegex(RuntimeError,"evidence_rejected"):a.exercise(None,None,1500,lambda *args,**kw:result)
                self.assertEqual(a.read()["phase"],"exercise_outcome_unknown")
                self.assertNotIn("PRIVATE",a.path.read_text())

    def test_integer_join_does_not_become_zero_exit(self):
        with tempfile.TemporaryDirectory() as d:
            a=self.setup_activity(d);a.exercise(None,None,1500,lambda *args,**kw:accepted())
            with self.assertRaisesRegex(RuntimeError,"join_unconfirmed"):
                a.stop_owners(lambda:self.fail(),lambda:{"status":"owner_closed","joined":1,"zero_exit":True})
            self.assertEqual(a.read()["joined"],{})


if __name__=="__main__":unittest.main()
