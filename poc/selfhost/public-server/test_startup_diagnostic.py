from pathlib import Path
import tempfile
import unittest
from startup_diagnostic import capture,label

class StartupDiagnostic(unittest.TestCase):
    def test_only_exact_fixed_reply_survives(self):
        for value in ({'status':'product_stage','stage':'controller_dial','PRIVATE':'PRIVATE'},
                      {'status':'product_stage','stage':'PRIVATE'},
                      {'status':'product_stage','stage':True},
                      {'status':'product_unavailable'},None):
            self.assertEqual(label(lambda frame:value),'unavailable')
        self.assertEqual(label(lambda frame:{'status':'product_stage','stage':'target_tls'}),'target_tls')

    def test_failure_is_read_only_fixed_and_evidence_exclusive(self):
        calls=[]
        def target(frame):
            calls.append(frame);return {'status':'product_stage','stage':'target_listen'}
        def controller(frame):
            calls.append(frame);raise RuntimeError('PRIVATE')
        with tempfile.TemporaryDirectory() as d:
            work=Path(d);capture(work,target,controller)
            text=(work/'startup-diagnostic.json').read_text()
            self.assertNotIn('PRIVATE',text);self.assertIn('target_listen',text);self.assertIn('unavailable',text)
            self.assertEqual(calls,[{'Op':'product_stage'}]*2)
            with self.assertRaises(FileExistsError):capture(work,target,controller)
            self.assertEqual(len(calls),2)
