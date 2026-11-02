import subprocess
import sys
import unittest
import contextlib
import io
import json
from joined_pipe import JoinedPipe


class JoinedPipeTests(unittest.TestCase):
    def test_fault_terminal_waiting_for_parent_eof_joins_without_stop_command(self):
        code='import sys; print(\'{"status":"session_state","joined":true,"fault":true}\'); print(\'{"status":"fault_retired","joined":true,"fault":true}\'); received=sys.stdin.read(); print(\'{"status":"failed"}\'); sys.exit(1 if received=="" else 9)'
        p=self.child(code)
        p.join_fault()
        self.assertTrue(p.proc.stdin.closed)
        self.assertTrue(p.fault_joined)
        self.assertFalse(p.closed_seen)
        self.assertFalse(p.stop_sent)
        self.assertEqual(p.proc.returncode,1)

    def test_fault_eof_is_not_sent_before_both_strict_receipts(self):
        completed=dict(status="session_state",joined=True,fault=True)
        retired=dict(status="fault_retired",joined=True,fault=True)
        for first,second in [
            (dict(completed,joined=False),retired),
            (dict(completed,joined=1),retired),
            (dict(completed,fault=1),retired),
            (dict(completed,extra="private"),retired),
            (completed,dict(retired,joined=False)),
            (completed,dict(retired,joined=1)),
            (completed,dict(retired,fault=1)),
            (completed,dict(retired,extra="private")),
            (completed,dict(status="closed"))]:
            with self.subTest(first=first,second=second):
                code='import sys; print('+repr(json.dumps(first))+'); print('+repr(json.dumps(second))+'); sys.stdin.read(); sys.exit(1)'
                p=self.child(code)
                with self.assertRaises(RuntimeError):p.join_fault()
                self.assertFalse(p.proc.stdin.closed)
                self.assertFalse(p.stop_sent)
                self.assertFalse(p.fault_joined)

    def test_fault_eof_does_not_replace_terminal_receipt_or_exit_code(self):
        for terminal,exit_code in [(None,1),('{"status":"closed"}',1),
                                   ('{"status":"failed","extra":"private"}',1),
                                   ('{"status":"failed"}',0)]:
            with self.subTest(terminal=terminal,exit_code=exit_code):
                code='import sys; print(\'{"status":"session_state","joined":true,"fault":true}\'); print(\'{"status":"fault_retired","joined":true,"fault":true}\'); sys.stdin.read(); '
                if terminal is not None:code+='print('+repr(terminal)+'); '
                code+='sys.exit('+str(exit_code)+')'
                p=self.child(code)
                with self.assertRaises(RuntimeError):p.join_fault()
                self.assertTrue(p.proc.stdin.closed)
                self.assertFalse(p.stop_sent)
                self.assertFalse(p.fault_joined)

    def test_diagnostics_are_bounded_whitelisted_and_separate_from_protocol(self):
        p=self.child('import sys; sys.stderr.write("secret-not-a-stage\\nJOIN_STAGE unknown\\n"+"x"*1000+"\\nJOIN_STAGE node_closing\\n"); print(\'{"status":"ready"}\')')
        self.assertEqual(p.read(),{"status":"ready"})
        p.proc.wait(timeout=2);p._join_diagnostics()
        self.assertEqual(p.join_stages,["node_closing"])
    def test_fault_join_requires_retirement_terminal_and_actual_exit_one(self):
        code='import sys; print(\'{"status":"session_state","joined":true,"fault":true}\'); print(\'{"status":"fault_retired","joined":true,"fault":true}\'); print(\'{"status":"failed"}\'); sys.exit(1)'
        p=self.child(code)
        diagnostic=io.StringIO()
        with contextlib.redirect_stdout(diagnostic):
            p.join_fault()
        self.assertEqual(diagnostic.getvalue().splitlines(),[
            "DIAGNOSTIC fault_wait_completion",
            "DIAGNOSTIC fault_completion_verified_wait_retirement",
            "DIAGNOSTIC fault_retirement_verified_wait_terminal"])
        self.assertTrue(p.fault_joined)
        self.assertFalse(p.closed_seen)
        self.assertEqual(p.proc.returncode,1)
        p.emergency()

    def test_fault_join_rejects_missing_retirement_and_zero_exit(self):
        for receipt, terminal, exit_code in [
            ('{"status":"closed"}', '{"status":"failed"}', 1),
            ('{"status":"fault_retired","joined":true,"fault":true}', '{"status":"failed"}', 0),
            ('{"status":"fault_retired","joined":true,"fault":true}', '{"status":"failed","extra":"private"}', 1)]:
            code='import sys; print(\'{"status":"session_state","joined":true,"fault":true}\'); print('+repr(receipt)+'); print('+repr(terminal)+'); sys.exit('+str(exit_code)+')'
            p=self.child(code)
            with self.assertRaises(RuntimeError): p.join_fault()
            self.assertFalse(p.fault_joined)

    def test_local_cancel_requires_separate_frames_exit_and_no_remote_claim(self):
        ready=dict(status='local_cancel_ready',joined=True,fault=True,remote_revocation_verified=False)
        final=dict(ready,status='local_cancel_closed')
        for mode in ('good','integer','remote','extra','zero','duplicate','rejected'):
            with self.subTest(mode=mode):
                a=dict(ready);b=dict(final)
                if mode=='integer':a['joined']=1
                if mode=='remote':a['remote_revocation_verified']=True
                if mode=='extra':b['extra']='PRIVATE'
                if mode=='rejected':a={'status':'local_cancel_rejected'}
                code='import sys,json; r=json.loads(sys.stdin.readline()); assert r=={"Op":"stop_local_fault"}; print('+repr(json.dumps(a))+'); sys.stdin.read(); print('+repr(json.dumps(b))+'); '
                if mode=='duplicate':code+='print('+repr(json.dumps(b))+'); '
                code+='sys.exit('+('0' if mode=='zero' else '1')+')'
                p=self.child(code)
                if mode=='good':
                    p.stop_local_fault();p.stop_local_fault()
                    self.assertTrue(p.local_cancel_joined)
                    self.assertFalse(p.fault_joined);self.assertFalse(p.closed_seen)
                else:
                    with self.assertRaises(RuntimeError):p.stop_local_fault()
                    self.assertFalse(p.local_cancel_joined)
                    if mode in ('integer','remote','rejected'):self.assertFalse(p.proc.stdin.closed)

    def test_local_cancel_late_ready_uses_same_request(self):
        code='import sys,json,time; assert json.loads(sys.stdin.readline())=={"Op":"stop_local_fault"}; time.sleep(.15); print(\'{"status": "local_cancel_ready", "joined": true, "fault": true, "remote_revocation_verified": false}\'); assert sys.stdin.read()==""; print(\'{"status": "local_cancel_closed", "joined": true, "fault": true, "remote_revocation_verified": false}\'); sys.exit(1)'
        p=self.child(code);read=p.read
        p.read=lambda timeout=12:read(.02)
        with self.assertRaises(Exception):p.stop_local_fault()
        self.assertTrue(p.local_cancel_sent);self.assertFalse(p.proc.stdin.closed)
        with self.assertRaises(RuntimeError):p.request('product_count')
        with self.assertRaises(RuntimeError):p.stop()
        with self.assertRaises(RuntimeError):p.stop_fault()
        with self.assertRaises(RuntimeError):p.join_upgraded()
        p.read=read;p.stop_local_fault()
        self.assertTrue(p.local_cancel_joined)

    def child(self,code,config=None):
        p=JoinedPipe([sys.executable,"-u","-c",code],config)
        def cleanup():
            if not p.proc.stdin.closed:p.proc.stdin.close()
            p.proc.wait(timeout=2)
            p.proc.stdout.close()
        self.addCleanup(cleanup)
        return p

    def test_two_frames_buffered_and_real_stop(self):
        p=self.child('import sys; print(\'{"status":"ready"}\'); print(\'{"status":"count","pongs":0}\'); sys.stdin.readline(); print(\'{"status":"closed"}\')')
        self.assertEqual(p.read()["status"],"ready")
        self.assertEqual(p.read()["pongs"],0)
        p.stop()
        self.assertTrue(p.closed_seen)
        self.assertEqual(p.proc.returncode,0)

    def test_partial_line_deadline_not_unbounded_readline(self):
        p=self.child('import sys; sys.stdout.write(\'{"status":\'); sys.stdout.flush(); sys.stdin.read()')
        with self.assertRaises(RuntimeError):p.read(.05)

    def test_duplicate_secret_field_and_oversize_rejected(self):
        for value in ('{"status":"ready","Status":"closed"}', 'x'*8193):
            p=self.child('print('+repr(value)+')')
            with self.assertRaises((RuntimeError,ValueError)):p.read()

    def test_failed_join_never_kills_or_claims_success(self):
        class Process:
            returncode=None
            def poll(self):return None
            def kill(self):raise AssertionError("force kill forbidden")
        p=JoinedPipe.__new__(JoinedPipe)
        import threading
        p.lock=threading.Lock();p.proc=Process();p.one_shot=False;p.prepared_seen=False
        p.stop_sent=True;p.closed_seen=False
        def read(timeout):raise RuntimeError("unconfirmed")
        p.read=read
        with self.assertRaises(RuntimeError):p.emergency()
        self.assertIsNone(p.proc.returncode)

    def test_failed_one_shot_joins_nonzero_without_claiming_prepared(self):
        p=self.child('import sys; sys.stdin.readline(); print(\'{"status":"failed","failure_stage":"intent_sign"}\'); sys.exit(1)',{'Op':'prepare'})
        with self.assertRaises(RuntimeError): p.read()
        p.emergency()
        self.assertEqual(p.proc.returncode,1)
        self.assertFalse(p.prepared_seen)
        self.assertFalse(p.closed_seen)

    def test_failed_one_shot_zero_exit_rejected(self):
        p=self.child('import sys; sys.stdin.readline(); print(\'{"status":"failed","failure_stage":"node_state"}\')',{'Op':'prepare'})
        with self.assertRaises(RuntimeError): p.read()
        with self.assertRaises(RuntimeError): p.emergency()

    def test_upgraded_join_requires_closed_and_actual_zero_exit(self):
        p=self.child('import sys; sys.stdin.readline(); print(\'{"status":"upgraded","epoch":2}\',flush=True); sys.stdin.read(); print(\'{"status":"closed"}\',flush=True)',{'Op':'upgrade'})
        self.assertEqual(p.read(),dict(status="upgraded",epoch=2))
        p.join_upgraded()
        self.assertEqual(p.proc.poll(),0)

    def test_upgraded_closed_receipt_cannot_hide_nonzero_exit(self):
        p=self.child('import sys; sys.stdin.readline(); print(\'{"status":"upgraded","epoch":2}\',flush=True); sys.stdin.read(); print(\'{"status":"closed"}\',flush=True); sys.exit(7)',{'Op':'upgrade'})
        p.read()
        with self.assertRaisesRegex(RuntimeError,'upgrade_exit_mismatch'):p.join_upgraded()

if __name__=="__main__":unittest.main()
