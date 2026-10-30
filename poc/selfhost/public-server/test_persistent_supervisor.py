import json
import os
from pathlib import Path
import socket
import sys
import tempfile
import threading
import time
import unittest

from joined_pipe import JoinedPipe
from persistent_supervisor import Endpoint, Owner, private_directory, serve


class PersistentOwnerTests(unittest.TestCase):
    def test_actual_private_child_survives_unknown_and_disconnected_management(self):
        # A protocol fixture, not an OS signer. The real child process remains
        # owned across distinct sockets and is joined only by explicit stop.
        with tempfile.TemporaryDirectory(prefix="ra-hold-", dir="/tmp") as raw:
            directory = Path(raw).resolve()
            script = directory/"child.py"
            script.write_text('''import json,sys
for line in sys.stdin:
 r=json.loads(line)
 if r['Op']=='open': v={'status':'ready','vault_scope':'a'*32,'public_key':'b'*64}
 elif r['Op']=='stop': print(json.dumps({'status':'closed'}),flush=True);break
 elif r['Op']=='device': v={'status':'request_rejected'}
 else: v={'status':'product_unavailable'}
 print(json.dumps(v),flush=True)
''')
            children, failures = [], []
            def factory(command, config):
                child = JoinedPipe([sys.executable, str(script)], config)
                children.append(child)
                return child
            def work():
                try: serve(directory, [], factory, require_native=False)
                except Exception as exc: failures.append(type(exc).__name__)
            thread = threading.Thread(target=work, daemon=True)
            thread.start()
            endpoint = Endpoint(directory)
            deadline = time.monotonic()+5
            while not (directory/"owner.json").exists() and time.monotonic()<deadline:
                time.sleep(.02)
            try:
                metadata = json.loads((directory/"owner.json").read_text())
                self.assertEqual(set(metadata), {"version","process_id","vault_scope","public_key"})
                self.assertEqual(endpoint({"Op":"device","Device":{"action":"register"}}),
                                 {"status":"request_rejected"})
                with socket.socket(socket.AF_UNIX) as connection:
                    connection.connect(str(directory/"owner.sock"))
                    connection.sendall(b'{"Op":"product_count"}\n')
                    # Drop this management connection without reading receipt.
                self.assertEqual(endpoint({"Op":"owner_status"}), {"status":"owner_status","joined":False})
                self.assertIsNone(children[0].proc.poll())
                self.assertEqual(endpoint({"Op":"owner_stop","extra":True}),
                                 {"status":"holder_outcome_unknown_state_retained"})
                self.assertEqual(endpoint({"Op":"product_count"}), {"status":"product_unavailable"})
            finally:
                self.assertEqual(endpoint({"Op":"owner_stop"}),
                                 {"status":"owner_closed","joined":True,"zero_exit":True})
                thread.join(timeout=5)
            self.assertFalse(thread.is_alive())
            self.assertFalse(failures)
            self.assertEqual(children[0].proc.returncode, 0)
            self.assertFalse((directory/"owner.json").exists())
            self.assertFalse((directory/"owner.sock").exists())

    def test_preflight_refuses_existing_metadata_before_starting_child(self):
        for name in ("owner.sock", "owner.json", "owner-fault-joined.json"):
            with self.subTest(name=name), tempfile.TemporaryDirectory(dir="/tmp") as raw:
                directory = Path(raw).resolve()
                (directory/name).write_text("preserved")
                with self.assertRaisesRegex(RuntimeError,"holder_metadata_exists"):
                    serve(directory, [], lambda *args:self.fail("must not start"))
                self.assertEqual((directory/name).read_text(), "preserved")

    def test_private_directory_and_socket_permissions(self):
        with tempfile.TemporaryDirectory(dir="/tmp") as raw:
            directory = Path(raw).resolve()
            os.chmod(directory,0o755)
            with self.assertRaisesRegex(RuntimeError,"holder_directory_rejected"):
                private_directory(directory)
            os.chmod(directory,0o700)
            address=directory/"owner.sock"
            address.write_text("not a socket")
            with self.assertRaisesRegex(RuntimeError,"holder_socket_rejected"):
                Endpoint(directory)({"Op":"owner_status"})

    def test_nonzero_exit_cannot_be_reported_as_zero_or_joined(self):
        class Broken:
            def stop(self): raise RuntimeError("private_join_nonzero_exit")
        owner = Owner(Broken())
        with self.assertRaisesRegex(RuntimeError,"nonzero_exit"):
            owner.call({"Op":"owner_stop"})
        self.assertFalse(owner.joined)


if __name__ == "__main__": unittest.main()
