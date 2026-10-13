import importlib.util
from pathlib import Path
import unittest
import tempfile
import os
from unittest.mock import patch

spec=importlib.util.spec_from_file_location("maintenance",Path(__file__).with_name("network-maintenance.py"))
m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)

class MaintenanceGates(unittest.TestCase):
    def test_legacy_frontend_stop_is_pinned_and_does_not_relax_new_receipts(self):
        for legacy,status,listener,hash_value,expected in (
            ("pinned","0",b"","pinned",True),
            (None,"0",b"","pinned",False),
            ("pinned","15",b"","pinned",False),
            ("pinned","0",b"LISTEN 0 1 127.0.0.1:8444 *:*\n","pinned",False),
            ("pinned","0",b"","other",False),
        ):
            with patch.object(m,"properties",side_effect=[{"MainPID":"12345","ActiveState":"active"},{"MainPID":"0","ExecMainStatus":status}]),patch.object(m,"system"),patch.object(m,"gone",return_value=True),patch.object(m,"logged",return_value=False),patch.object(m,"digest",return_value=hash_value),patch.object(m,"command",return_value=listener),patch("builtins.open",return_value=__import__("io").BytesIO(b"device-only 127.0.0.1:8444")):
                if expected: m.stopped("remoteapp-frontend","STOPPED frontend",legacy)
                else:
                    with self.assertRaises(RuntimeError): m.stopped("remoteapp-frontend","STOPPED frontend",legacy)
    def test_stage_rejects_formal_targets_traversal_and_injection(self):
        for path in ("/var/lib/remoteapp-authority","/run/remoteapp-network-maint-0123456789abcdef/..","/run/remoteapp-network-maint-ABCDEF0123456789","/run/remoteapp-network-maint-0123456789abcdef\nother","/run/remoteapp-network-maint-0123456789abcdef\n","relative"):
            self.assertFalse(m.stage_valid(path))
    def test_atomic_preserves_complete_bytes_and_mode(self):
        # Exercise the writer only in a throwaway root-owned directory.
        if os.geteuid()!=0:
            from unittest.mock import patch
            with tempfile.TemporaryDirectory() as d,patch.object(m.os,"chown"):
                path=str(Path(d).resolve()/"checkpoint.json")
                m.atomic(path,b'{"phase":"prepared"}',0o600)
                self.assertEqual(Path(path).read_bytes(),b'{"phase":"prepared"}')
                self.assertEqual(os.stat(path).st_mode & 0o777,0o600)
        else:
            with tempfile.TemporaryDirectory() as d:
                path=str(Path(d).resolve()/"checkpoint.json")
                m.atomic(path,b'{"phase":"prepared"}',0o600)
                self.assertEqual(Path(path).read_bytes(),b'{"phase":"prepared"}')
                self.assertEqual(os.stat(path).st_mode & 0o777,0o600)

if __name__=="__main__": unittest.main()
