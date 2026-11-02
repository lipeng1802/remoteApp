import tempfile
from pathlib import Path
import unittest
from unittest.mock import patch
from policy_observation import checked,observe
import run_supervised_activity as r

class PolicyObservationTests(unittest.TestCase):
    def test_fixed_schema_rejects_material_and_truthy_counters(self):
        good={'query_ok':True,'payload_rule_count':1,'proof_rule_count':0}
        self.assertEqual(checked(good),good)
        for change in ({'query_ok':1},{'payload_rule_count':True},{'proof_rule_count':257},{'payload_rule_count':-1},{'extra':'PRIVATE'}):
            with self.assertRaises(RuntimeError):checked(dict(good,**change))
    def test_unavailable_or_private_response_never_reaches_audit(self):
        with tempfile.TemporaryDirectory() as d:
            w=Path(d)
            with patch.object(r,'command',return_value=b'{"PRIVATE":"SECRET"}') as command:
                observe(w)
            text=(w/'provision-policy-observation.json').read_text()
            self.assertNotIn('PRIVATE',text);self.assertNotIn('SECRET',text)
            self.assertIn('false',text);self.assertEqual(command.call_args.args[0][0],'ssh')
            with self.assertRaises(FileExistsError):observe(w)
