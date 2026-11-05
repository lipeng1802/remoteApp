import base64
import json
from pathlib import Path
import tempfile
import time
import unittest
from unittest.mock import patch

from activity_control import Activity
import desktop_workorder as desktop
import run_supervised_activity as runner


STAGE = desktop.STAGE_PREFIX + "a" * 16
ROOT = "b" * 64


def source(expires_at):
    return {"claims": {"service": desktop.SERVICE, "endpoint": desktop.ENDPOINT, "expires_at": expires_at,
                       "certificate": base64.b64encode(b"test-certificate").decode(), "issuer": "c" * 64, "epoch": 1},
            "signature": "e" * 128}


def receipt(expires_at):
    return json.dumps({"port": 38051, "profile_root": ROOT, "profile": source(expires_at)}).encode()


class DesktopWorkorder(unittest.TestCase):
    def test_bootstrap_bound_to_fixed_stage_and_independently_supplied_root(self):
        raw = receipt(2500)
        self.assertEqual(desktop.stage_id(STAGE), "a" * 16)
        for wrong in (STAGE + "/", "/var/lib/remoteapp-poc", STAGE + ";id"):
            with self.assertRaisesRegex(RuntimeError, "stage_rejected"):
                desktop.stage_id(wrong)
        self.assertEqual(desktop.ready_profile(raw, ROOT, 1000)[0], 38051)
        with self.assertRaisesRegex(RuntimeError, "bootstrap_rejected"):
            desktop.ready_profile(raw, "f" * 64, 1000)
        for port in (8444, 19443, True):
            value = json.loads(raw)
            value["port"] = port
            with self.assertRaisesRegex(RuntimeError, "port_rejected"):
                desktop.ready_profile(json.dumps(value).encode(), ROOT, 1000)
        with self.assertRaisesRegex(RuntimeError, "profile_rejected"):
            desktop.ready_profile(raw, ROOT, 2000)

    def test_one_preflight_failure_prevents_build_and_repeat_for_stage(self):
        with tempfile.TemporaryDirectory() as root:
            repo = Path(root)
            (repo / "artifacts/connection-poc").mkdir(parents=True)
            with (patch.object(runner, "REPO", repo),
                  patch.object(runner, "command", return_value=receipt(int(time.time()) + 1500)) as ssh,
                  patch.object(runner, "desktop_preflight", return_value={"controller": True, "target": False}) as probe,
                  patch.object(runner, "build_current_helpers") as build):
                with self.assertRaisesRegex(RuntimeError, "preflight_failed"):
                    runner.prepare_desktop(STAGE, ROOT)
                with self.assertRaises(FileExistsError):
                    runner.prepare_desktop(STAGE, ROOT)
                work = repo / "artifacts/connection-poc" / ("desktop-activity-" + "a" * 16)
                self.assertEqual(Activity(work).read()["phase"], "isolated_ingress_unavailable")
                probe.assert_called_once()
                ssh.assert_called_once()
                build.assert_not_called()

    def test_preflight_checks_both_fixed_tls_routes_once(self):
        with (patch.object(runner, "command", side_effect=RuntimeError("timeout")) as local,
              patch.object(runner, "ps", return_value="PASS") as windows):
            self.assertEqual(runner.desktop_preflight(), {"controller": False, "target": True})
            local.assert_called_once()
            windows.assert_called_once()
            self.assertEqual(local.call_args.args[0][-1], desktop.CONTROL + "/health")
            self.assertIn(desktop.CONTROL + "/health", windows.call_args.args[0])
            self.assertNotIn("--insecure", windows.call_args.args[0])

    def test_bundle_expiry_requires_both_platforms_to_pin_the_same_source(self):
        with tempfile.TemporaryDirectory() as root:
            work = Path(root) / ("desktop-activity-" + "a" * 16)
            work.mkdir()
            profile = source(2500)
            (work / "service-profile.json").write_text(json.dumps(profile))
            for target in ("darwin", "windows"):
                (work / target).mkdir()
                spec = {"test_namespace": True, "bundled_profile": profile}
                bundle = {"release": base64.b64encode(json.dumps(spec).encode()).decode(),
                          "desktop_permission": desktop.SCOPE, "platform": target}
                (work / target / "fixture.json").write_text(json.dumps(bundle))
            self.assertEqual(runner.expiry(work), 2500)
            path = work / "windows/fixture.json"
            del bundle["desktop_permission"]
            path.write_text(json.dumps(bundle))
            with self.assertRaisesRegex(RuntimeError, "desktop_bundle_rejected"):
                runner.expiry(work)

    def test_forward_proof_precedes_scheduled_desktop_start(self):
        with tempfile.TemporaryDirectory() as root:
            work = Path(root)
            m = {"forward_port": 18450, "authority_port": 38051, "nonce": "a" * 12, "desktop_permission": desktop.SCOPE,
                 "win_dir": r"C:\Users\jarvis\fresh", "win_root": r"C:\Users\jarvis\installed"}
            archive = work / "windows.tar.xz"
            archive.write_bytes(b"fresh")
            with (patch.object(runner, "expiry", return_value=time.time() + 600),
                  patch.object(runner, "ps") as ps, patch.object(runner, "command") as command,
                  patch.object(runner, "prove_desktop_authority_forward", side_effect=RuntimeError("proof_failed"))):
                with self.assertRaisesRegex(RuntimeError, "proof_failed"):
                    runner.finish_start_after_transfer(work, m, archive, None, [])
                ps.assert_called_once()
                self.assertNotIn("Start-ScheduledTask", ps.call_args.args[0])
                self.assertIn("127.0.0.1:18450:127.0.0.1:38051", command.call_args_list[0].args[0])
                self.assertIn("127.0.0.1:18450:127.0.0.1:18450", command.call_args_list[1].args[0])
