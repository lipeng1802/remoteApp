import hashlib
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

from split_nodes import upload_helper


class NodeUploadTests(unittest.TestCase):
    def test_compressed_bounded_transfer_then_hash_check(self):
        with tempfile.TemporaryDirectory() as directory:
            binary=Path(directory)/"enrollment-helper-win.exe"
            binary.write_bytes(b"public helper")
            class Transport:
                ARTIFACTS=Path(directory)
                def powershell(self, script): return hashlib.sha256(binary.read_bytes()).hexdigest()
                def check(self, ok, label):
                    if not ok: raise RuntimeError(label)
            class Result: returncode=0
            with patch("split_nodes.subprocess.run", return_value=Result()) as run:
                upload_helper(Transport(), "C:\\isolated")
                self.assertIn("-C",run.call_args.args[0])
                self.assertEqual(run.call_args.kwargs["timeout"],180)
            Transport.powershell=lambda self, script: "0"*64
            with patch("split_nodes.subprocess.run", return_value=Result()):
                with self.assertRaisesRegex(RuntimeError,"split_node_helper_binary_verified"):
                    upload_helper(Transport(), "C:\\isolated")
