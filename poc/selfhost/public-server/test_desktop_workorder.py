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
    def test_unknown_desktop_workorder_does_not_touch_endpoints_or_trace(self):
        with tempfile.TemporaryDirectory() as directory:
            work = Path(directory)
            Activity(work).save({"version": 1, "phase": "exercise_outcome_unknown", "desktop": True})
            with (patch.object(runner, "metadata", return_value=(work, {"desktop_permission": desktop.SCOPE})),
                  patch.object(runner, "WindowsRelay") as relay,
                  patch.object(runner, "Endpoint") as endpoint):
                with self.assertRaisesRegex(RuntimeError, "activity_requires_fresh_ready_owners"):
                    runner.exercise_desktop(work, observe_gui=self.fail, request_stop=self.fail)
                relay.assert_not_called()
                endpoint.assert_not_called()
            self.assertEqual([item.name for item in work.iterdir()], ["activity-state.json"])

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

    def test_forward_proof_pins_stage_ca_and_https_hostname(self):
        for status in (405, 200):
            with self.subTest(status=status), tempfile.TemporaryDirectory() as root:
                work = Path(root)
                (work / "service-profile.json").write_text(json.dumps(source(2500)))
                with (patch.object(runner.ssl, "SSLContext") as tls,
                      patch.object(runner.socket, "create_connection") as dial,
                      patch.object(runner.http.client, "HTTPConnection") as request):
                    context = tls.return_value
                    response = request.return_value.getresponse.return_value.__enter__.return_value
                    response.status = status
                    if status == 405:
                        runner.prove_desktop_authority_forward(work, 18450)
                    else:
                        with self.assertRaisesRegex(RuntimeError, "forward_unconfirmed"):
                            runner.prove_desktop_authority_forward(work, 18450)
                    tls.assert_called_once_with(runner.ssl.PROTOCOL_TLS_CLIENT)
                    self.assertEqual(context.minimum_version, runner.ssl.TLSVersion.TLSv1_3)
                    context.load_verify_locations.assert_called_once_with(cafile=work / "desktop-authority-ca.pem")
                    dial.assert_called_once_with(("127.0.0.1", 18450), timeout=6)
                    context.wrap_socket.assert_called_once_with(
                        dial.return_value.__enter__.return_value, server_hostname="mk.fengmap.com")
                    request.assert_called_once_with("mk.fengmap.com", 8444, timeout=6)
                    request.return_value.request.assert_called_once_with("GET", "/v1/request")

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
