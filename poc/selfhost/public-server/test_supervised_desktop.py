import hashlib
import tempfile
import threading
import unittest

from activity_control import Activity
from supervised_desktop import DESKTOP_EVIDENCE, GUI_EVIDENCE


class Pair:
    def __init__(self, fault=None):
        self.fault = fault
        self.elapsed = 0
        self.calls = []
        self.confirmed = False
        self.stop_requested = False
        self.bootstraps = {
            role: {"desktop_permission": "desktop-control", "public_key": char * 64}
            for role, char in (("target", "1"), ("controller", "2"))
        }
        subjects = {role: hashlib.sha256(bytes.fromhex(value["public_key"])).hexdigest()
                    for role, value in self.bootstraps.items()}
        self.grant = {"claims": {"version": 1, "id": "a" * 32, **subjects,
                                 "permission": "desktop-control", "issued_at": 1000, "expires_at": 1300},
                      "signature": "b" * 128}
        self.provision = threading.Barrier(2, timeout=2)

    def endpoint(self, role):
        def call(request):
            op = request["Op"]
            self.calls.append((role, op))
            if op == "desktop_begin":
                if self.fault == "lost_begin" and role == "target":
                    raise RuntimeError("PRIVATE_BEGIN")
                return {"status": "desktop_pending"}
            if op == "desktop_status":
                request_id = "c" * 32 if role == "controller" and self.fault == "wrong_request" else "d" * 32
                reply = {"status": "desktop_status", "phase": "approved", "request_id": request_id}
                if self.fault in ("denied", "expired"):
                    return {**reply, "phase": self.fault}
                if role == "controller" and self.fault == "lost_approval" and self.elapsed > 0:
                    raise RuntimeError("PRIVATE_APPROVAL")
                if self.elapsed == 0:
                    return {**reply, "phase": "waiting"}
                grant = self.grant
                if role == "controller" and self.fault == "different_grant":
                    grant = {**grant, "signature": "e" * 128}
                return {**reply, "grant": grant}
            if op == "preprovision_terminal":
                return {"status": "preprovision_terminal", "passed": True}
            if op == "network":
                assert request == {"Op": op, "Network": {"action": "grant_clock", "grant": self.grant}}
                return {"status": "network_authorization", "phase": "grant_clock", "passed": True, "active": False}
            if op == "provision":
                assert request == {"Op": op, "Network": {"grant": self.grant}}
                self.provision.wait()
                if self.fault == "lost_provision" and role == "controller":
                    raise RuntimeError("PRIVATE_PROVISION")
                return {"status": "network_watch", "passed": True, "active": True}
            if op == "product_stage":
                assert role == "target"
                return {"status": "product_stage", "stage": "target_listening"}
            if op == "desktop_state":
                count = 2 if self.confirmed else 0
                return {"status": "desktop_state", "frames": count, "inputs": count, "releases": 0,
                        "held": 0, "ended": False, "failed": False, "stop_requested": False}
            if op == "start_controller":
                assert role == "controller"
                return {"status": "controller_started", "passed": True}
            if op == "desktop_arm_stop":
                assert self.stop_requested
                return {"status": "desktop_stop_armed"}
            if op == "quiesce_session":
                return {"status": "session_quiesced", "joined": True, "zero_exit": True}
            if op == "revoke_session":
                assert role == "target"
                if self.fault == "lost_revoke":
                    raise RuntimeError("PRIVATE_REVOKE")
                return {"status": "session_revoked", "passed": True}
            raise AssertionError(op)
        return call

    def sleep(self, seconds):
        self.elapsed += seconds

    def observe(self, expires_at):
        assert expires_at == 1300 and ("controller", "start_controller") in self.calls
        self.confirmed = True
        return {} if self.fault == "missing_gui" else dict.fromkeys(GUI_EVIDENCE, True)

    def stop(self, expires_at):
        assert expires_at == 1300 and self.confirmed
        self.stop_requested = True
        return True

    def run(self, activity):
        return activity.exercise_desktop(
            self.endpoint("target"), self.endpoint("controller"), 1500,
            target_bootstrap=self.bootstraps["target"], controller_bootstrap=self.bootstraps["controller"],
            observe_gui=self.observe, request_stop=self.stop,
            clock=lambda: 1000 + self.elapsed, monotonic=lambda: self.elapsed, sleep=self.sleep,
        )


class DesktopRoundTests(unittest.TestCase):
    def test_actual_observations_required_before_planned_join_and_revoke(self):
        with tempfile.TemporaryDirectory() as root:
            activity = Activity(root)
            activity.save({"version": 1, "phase": "owners_ready"})
            pair = Pair()
            self.assertEqual(pair.run(activity), dict.fromkeys(DESKTOP_EVIDENCE, True))
            self.assertEqual(activity.read()["phase"], "normal_revoked")
            self.assertEqual(pair.calls[:2], [("target", "desktop_begin"), ("controller", "desktop_begin")])
            self.assertEqual(pair.calls[-5:], [
                ("controller", "desktop_arm_stop"), ("target", "desktop_arm_stop"),
                ("controller", "quiesce_session"), ("target", "quiesce_session"), ("target", "revoke_session"),
            ])
            with self.assertRaisesRegex(RuntimeError, "join_unconfirmed"):
                activity.stop_owners(lambda: self.fail("unexpected target stop"), lambda: None)
            with self.assertRaisesRegex(RuntimeError, "desktop_owner_stop_unknown"):
                activity.stop_owners(self.fail, self.fail)

    def test_matching_denial_allows_only_confirmed_preprovision_stop(self):
        with tempfile.TemporaryDirectory() as root:
            activity = Activity(root)
            activity.save({"version": 1, "phase": "owners_ready"})
            pair = Pair("denied")
            self.assertEqual(pair.run(activity), {"preprovision_terminal": True})
            self.assertEqual(activity.read()["phase"], "preprovision_terminal")
            self.assertEqual(set(pair.calls[-2:]), {("target", "preprovision_terminal"),
                                                    ("controller", "preprovision_terminal")})

    def test_uncertain_episodes_preserve_owners_without_repeating_mutations(self):
        no_revoke = {"lost_begin", "wrong_request", "expired"}
        faults = no_revoke | {"lost_approval", "different_grant", "lost_provision", "missing_gui", "lost_revoke"}
        for fault in sorted(faults):
            with self.subTest(fault=fault), tempfile.TemporaryDirectory() as root:
                activity = Activity(root)
                activity.save({"version": 1, "phase": "owners_ready"})
                pair = Pair(fault)
                with self.assertRaisesRegex(RuntimeError, "^activity_outcome_unknown_preserve_owners_and_tunnels$"):
                    pair.run(activity)
                self.assertEqual(activity.read()["phase"], "exercise_outcome_unknown")
                self.assertNotIn("PRIVATE", activity.path.read_text())
                with self.assertRaisesRegex(RuntimeError, "fresh_ready"):
                    pair.run(activity)
                with self.assertRaisesRegex(RuntimeError, "preserve_signers"):
                    activity.stop_owners(self.fail, self.fail)
                self.assertEqual(pair.calls.count(("target", "desktop_begin")), 1)
                self.assertEqual(pair.calls.count(("target", "revoke_session")), 0 if fault in no_revoke else 1)
                if fault != "lost_revoke":
                    self.assertNotIn(("controller", "desktop_arm_stop"), pair.calls)
