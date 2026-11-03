from pathlib import Path
import tempfile
import unittest
from activity_trace import RecordedEndpoint


class PublicActivityTrace(unittest.TestCase):
    def test_private_payload_and_unknown_labels_never_reach_trace(self):
        with tempfile.TemporaryDirectory() as d:
            request={'Op':'PRIVATE','Device':{'action':'PRIVATE','grant':'PRIVATE'}}
            response={'status':'PRIVATE','grant':'PRIVATE'}
            seen=[]
            def endpoint(frame):seen.append(frame);return response
            traced=RecordedEndpoint(Path(d),'target',endpoint)
            self.assertIs(traced(request),response);self.assertIs(seen[0],request)
            text=''.join(p.read_text() for p in traced.directory.iterdir())
            self.assertNotIn('PRIVATE',text);self.assertIn('calling',text);self.assertIn('returned',text)
            with self.assertRaises(FileExistsError):RecordedEndpoint(Path(d),'target',endpoint)

    def test_unknown_rpc_retains_prior_stage_without_private_exception_text(self):
        with tempfile.TemporaryDirectory() as d:
            def endpoint(frame):raise RuntimeError('PRIVATE')
            traced=RecordedEndpoint(Path(d),'controller',endpoint)
            with self.assertRaisesRegex(RuntimeError,'PRIVATE'):traced({'Op':'provision','Network':{'grant':'PRIVATE'}})
            text=''.join(p.read_text() for p in traced.directory.iterdir())
            self.assertNotIn('PRIVATE',text);self.assertIn('provision',text);self.assertIn('outcome_unknown',text)

    def test_status_trace_records_only_public_terminal_evidence(self):
        import json
        with tempfile.TemporaryDirectory() as d:
            responses = iter((
                {'status':'device_result','passed':True,'device':{'status':'revoked','request_id':'PRIVATE'}},
                {'status':'device_result','passed':False,'device':{'status':'PRIVATE'}},
                {'status':'request_rejected'},
            ))
            traced = RecordedEndpoint(Path(d), 'controller', lambda _:next(responses))
            for _ in range(3):
                traced({'Op':'device','Device':{'action':'status','request_id':'PRIVATE'}})
            rows = [json.loads(p.read_text()) for p in sorted(traced.directory.glob('*.json'))][1::2]
            self.assertEqual(rows[0]['device_state'], 'revoked')
            self.assertTrue(rows[0]['passed'])
            self.assertFalse(rows[0]['grant_present'])
            self.assertFalse(rows[1]['passed'])
            self.assertNotIn('device_state', rows[1])
            self.assertEqual(rows[2]['status'], 'request_rejected')
            self.assertNotIn('PRIVATE', repr(rows))
