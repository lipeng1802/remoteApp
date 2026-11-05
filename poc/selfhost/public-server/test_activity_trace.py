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

    def test_rejection_reason_is_fixed_and_private_text_is_discarded(self):
        import json
        with tempfile.TemporaryDirectory() as d:
            replies=iter((
                {"status":"request_rejected","reason":"grant_observation_rejected"},
                {"status":"request_rejected","reason":"PRIVATE"},
            ))
            traced=RecordedEndpoint(Path(d),"controller",lambda _:next(replies))
            for _ in range(2):traced({"Op":"device","Device":{"action":"status"}})
            rows=[json.loads(p.read_text()) for p in sorted(traced.directory.glob("*.json"))][1::2]
            self.assertEqual(rows[0]["reason"],"grant_observation_rejected")
            self.assertNotIn("reason",rows[1])
            self.assertNotIn("PRIVATE",repr(rows))

    def test_fixed_startup_and_pong_evidence_omits_private_or_invalid_values(self):
        import json
        with tempfile.TemporaryDirectory() as d:
            replies=iter((
                {"status":"product_stage","stage":"target_filter_match","grant":"PRIVATE"},
                {"status":"product_stage","stage":"PRIVATE"},
                {"status":"product_count","pongs":12,"preauth_rejected":True,"ended":False,"pin_rejected":False,"grant":"PRIVATE"},
                {"status":"controller_started","passed":True},
            ))
            traced=RecordedEndpoint(Path(d),"target",lambda _:next(replies))
            for op in ('product_stage','product_stage','product_count','start_controller'):
                traced({"Op":op})
            rows=[json.loads(p.read_text()) for p in sorted(traced.directory.glob('*.json'))][1::2]
            self.assertEqual(rows[0]['stage'],'target_filter_match')
            self.assertNotIn('stage',rows[1]);self.assertEqual(rows[2]['pongs'],12)
            self.assertNotIn('preauth_rejected',rows[2]);self.assertIs(rows[3]['passed'],True)
            self.assertNotIn('PRIVATE',repr(rows))

    def test_desktop_trace_keeps_only_phase_and_aggregate_state(self):
        import json
        with tempfile.TemporaryDirectory() as d:
            replies = iter((
                {"status": "desktop_status", "phase": "approved", "grant": "PRIVATE", "request_id": "PRIVATE"},
                {"status": "desktop_state", "frames": 5, "inputs": 2, "held": True,
                 "stop_requested": False, "unknown": "PRIVATE"},
            ))
            traced = RecordedEndpoint(Path(d), "target", lambda _: next(replies))
            traced({"Op": "desktop_status"})
            traced({"Op": "desktop_state"})
            rows = [json.loads(p.read_text()) for p in sorted(traced.directory.glob("*.json"))][1::2]
            self.assertEqual(rows[0]["desktop_phase"], "approved")
            self.assertEqual((rows[1]["frames"], rows[1]["inputs"]), (5, 2))
            self.assertIs(rows[1]["stop_requested"], False)
            self.assertNotIn("held", rows[1])
            self.assertNotIn("PRIVATE", repr(rows))
