import base64
import json
from pathlib import Path
import subprocess
import tempfile
import time
import unittest
from unittest.mock import patch
import run_supervised_consent as runner


class ManagementAdmission(unittest.TestCase):
    def fixture(self, directory, expiry):
        work=Path(directory)
        (work/"darwin").mkdir()
        (work/"windows").mkdir()
        (work/"windows/supervised-control-relay.exe").write_bytes(b"TEST fixture; never executed")
        nonce="a"*12
        meta={"nonce":nonce,"win_dir":"C:\\Users\\jarvis\\remoteapp-r67-package-"+nonce,"mac_root":str(work/"installed")}
        (work/"management.json").write_text(json.dumps(meta))
        profile={"bundled_profile":{"claims":{"expires_at":expiry}}}
        (work/"darwin/fixture.json").write_text(json.dumps({"release":base64.b64encode(json.dumps(profile).encode()).decode()}))
        return work

    def test_expired_build_never_starts_ssh_or_installs(self):
        with tempfile.TemporaryDirectory() as d, patch.object(runner.shutil,"copy2"), patch.object(runner,"run") as run:
            work=self.fixture(d,int(time.time())-1)
            with self.assertRaisesRegex(RuntimeError,"fresh_build_window"):runner.main(work)
            run.assert_not_called()
            self.assertFalse((work/"execution.json").exists())

    def test_completed_authorization_cannot_be_replayed_by_setup_recovery(self):
        with tempfile.TemporaryDirectory() as d, patch.object(runner.shutil,"copy2"), patch.object(runner,"run") as run:
            work=self.fixture(d,int(time.time())+1200)
            (work/"execution.json").write_text(json.dumps({"consent":True}))
            with self.assertRaisesRegex(RuntimeError,"explicit_recovery"):runner.main(work,True)
            run.assert_not_called()

    def test_existing_mac_install_cannot_be_overwritten_during_recovery(self):
        with tempfile.TemporaryDirectory() as d, patch.object(runner.shutil,"copy2"), patch.object(runner,"run") as run:
            work=self.fixture(d,int(time.time())+1200)
            (work/"installed").mkdir()
            (work/"execution.json").write_text(json.dumps({"consent":False}))
            with self.assertRaisesRegex(RuntimeError,"explicit_recovery"):runner.main(work,True)
            run.assert_not_called()

    def test_command_failure_does_not_publish_private_stderr(self):
        with patch.object(runner.subprocess,"run",return_value=subprocess.CompletedProcess([],1,b"PRIVATE-GRANT",b"PRIVATE-INVITE")):
            with self.assertRaisesRegex(RuntimeError,"^management_command_failed$"):runner.run(["trusted-command"])

    def test_large_powershell_requires_file_upload_before_ssh(self):
        with patch.object(runner,"run") as run:
            with self.assertRaisesRegex(RuntimeError,"requires_file_upload"):runner.ps("x"*3000)
            run.assert_not_called()

    def test_management_timeout_is_not_retried_or_reported_joined(self):
        with patch.object(runner.subprocess,"run",side_effect=subprocess.TimeoutExpired(["ssh"],90)) as run:
            with self.assertRaisesRegex(RuntimeError,"outcome_unknown"):runner.run(["ssh"])
            self.assertEqual(run.call_count,1)


if __name__=="__main__":unittest.main()
