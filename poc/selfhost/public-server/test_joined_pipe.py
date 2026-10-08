import subprocess
import sys
import unittest
from joined_pipe import JoinedPipe


class JoinedPipeTests(unittest.TestCase):
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

if __name__=="__main__":unittest.main()
