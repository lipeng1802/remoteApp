"""Failure boundaries for the operator; does not run systemd or touch OS state."""
import copy
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("binary_upgrade", Path(__file__).with_name("binary-upgrade.py"))
u = importlib.util.module_from_spec(spec)
spec.loader.exec_module(u)


class Primitives:
    STORE = "store"
    VD = "verifier"

    def __init__(self):
        self.ops = []
        self.locked = False
        self.stop_fault = False
        self.store_changed = False
        self.probe_fault = False
        self.state = None

    def require(self, ok, label):
        if not ok:
            raise RuntimeError(label)

    def digest(self, path):
        return "changed" if path == self.STORE and self.store_changed else "hash"

    def identity(self): return "identity"
    def zero_policy(self): self.ops.append("zero_policy")
    def command(self, args):
        self.ops.append(tuple(args))
        return b"CHECK PASSED: idle\n"
    def properties(self, name): return {"MainPID": "0", "ActiveState": "inactive"}
    def no_locks(self): self.require(not self.locked, "owner_lock_retained")
    def save(self, path, state):
        self.state = copy.deepcopy(state)
        Path(path, "checkpoint.json").write_text(json.dumps(state))
    def stopped(self, name, label):
        self.ops.append(("stop", name))
        if self.stop_fault and name == "remoteapp-authority": raise RuntimeError("stop_not_joined")
    def atomic(self, *args): self.ops.append("atomic_write")
    def ready(self, *args): pass
    def probe(self, mode): self.require(not self.probe_fault, "probe_failed")


class UpgradeBoundaries(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.stage = str(Path(self.tmp.name, "stage"))
        self.backup = Path(self.tmp.name, "backup", "stage")
        self.backup.mkdir(parents=True)
        self.m = Primitives()
        self.manifest = {"sha256": dict((n, "candidate") for n in u.NAMES)}
        self.state = {"phase": "stopped", "store": "hash", "identity": "identity", "verifier_journal": "hash", "config": {}, "candidate": self.manifest["sha256"], "original": {}}
        self.m.save(str(self.backup), self.state)
        for p in (patch.object(u, "BACKUPS", str(self.backup.parent)), patch.object(u, "load", return_value=(self.manifest, self.m)), patch.object(u, "private", return_value=True), patch.object(u, "wait_ready", return_value=123), patch.object(u, "graceful_stop", side_effect=lambda m, n, label: m.stopped(n, label))):
            p.start()
            self.addCleanup(p.stop)

    def test_live_lock_prevents_idle_precheck_and_install(self):
        self.m.locked = True
        with self.assertRaisesRegex(RuntimeError, "owner_lock_retained"): u.main(self.stage, "check")
        self.assertEqual(self.m.state["phase"], "stopped")
        self.assertFalse(any(isinstance(v, tuple) and "--check-network-config" in v for v in self.m.ops))
        self.assertNotIn("atomic_write", self.m.ops)

    def test_partial_stop_preserves_uncertain_checkpoint(self):
        self.state["phase"] = "prepared"
        self.m.save(str(self.backup), self.state)
        self.m.stop_fault = True
        with self.assertRaisesRegex(RuntimeError, "stop_not_joined"): u.main(self.stage, "quiesce")
        self.assertEqual(self.m.state["phase"], "stopping")
        self.assertNotIn("atomic_write", self.m.ops)

    def test_store_change_rejects_rollback_without_restoration(self):
        self.m.store_changed = True
        with self.assertRaisesRegex(RuntimeError, "store_changed_no_restore"): u.main(self.stage, "rollback")
        self.assertNotIn("atomic_write", self.m.ops)

    def test_probe_failure_cannot_mark_verified(self):
        self.state["phase"] = "started"
        self.m.save(str(self.backup), self.state)
        self.m.probe_fault = True
        with self.assertRaisesRegex(RuntimeError, "probe_failed"): u.main(self.stage, "verify")
        self.assertEqual(self.m.state["phase"], "started")

    def test_changed_candidate_cannot_continue_work_order(self):
        self.manifest["sha256"] = {"other": "hash"}
        with self.assertRaisesRegex(RuntimeError, "work_order_candidate_changed"): u.main(self.stage, "install")
        self.assertNotIn("atomic_write", self.m.ops)


class MainSignalBoundaries(unittest.TestCase):
    def test_main_term_waits_for_exit_and_receipt_before_systemd_stop(self):
        from unittest.mock import Mock
        m = Mock()
        m.require.side_effect = lambda ok, label: None if ok else (_ for _ in ()).throw(RuntimeError(label))
        m.properties.return_value = {"MainPID":"0"}
        m.gone.side_effect = [False, True, True]
        with patch.object(u.os, "kill") as kill, patch.object(u.time, "sleep"), patch.object(u, "service_identity", return_value=(123,100,"active")), patch.object(u,"receipt",side_effect=[False,True]):
            u.graceful_stop(m,"frontend","STOPPED")
        kill.assert_called_once_with(123, u.signal.SIGTERM)
        m.system.assert_called_once_with("stop", "frontend")

    def test_missing_join_receipt_never_escalates_or_accepts_inactive(self):
        from unittest.mock import Mock
        m = Mock()
        m.require.side_effect = lambda ok, label: None if ok else (_ for _ in ()).throw(RuntimeError(label))
        m.gone.return_value = True
        with patch.object(u.os, "kill") as kill, patch.object(u.time, "sleep"), patch.object(u, "service_identity", return_value=(123,100,"active")), patch.object(u,"receipt",return_value=False):
            with self.assertRaisesRegex(RuntimeError,"stop_not_joined"): u.graceful_stop(m,"frontend","STOPPED")
        kill.assert_called_once_with(123, u.signal.SIGTERM)
        m.system.assert_not_called()

    def test_old_start_receipt_with_same_pid_is_not_current_join(self):
        from unittest.mock import Mock
        m = Mock()
        m.require.side_effect = lambda ok, label: None if ok else (_ for _ in ()).throw(RuntimeError(label))
        rows = [{"MESSAGE":"STOPPED", "_PID":"123", "_SYSTEMD_UNIT":"frontend.service", "__MONOTONIC_TIMESTAMP":"90"}]
        m.command.return_value = json.dumps(rows[0]).encode()
        self.assertFalse(u.receipt(m,"frontend",123,100,"STOPPED"))
        rows[0]["__MONOTONIC_TIMESTAMP"] = "110"
        m.command.return_value = json.dumps(rows[0]).encode()
        self.assertTrue(u.receipt(m,"frontend",123,100,"STOPPED"))
        self.assertIn("-b", m.command.call_args.args[0])

    def test_unitless_terminal_requires_same_current_ready_stdout_stream(self):
        from unittest.mock import Mock
        ready={"MESSAGE":u.FRONT_READY,"_PID":"123","_SYSTEMD_UNIT":"remoteapp-frontend.service",
               "__MONOTONIC_TIMESTAMP":"101","_STREAM_ID":"a"*32,"_BOOT_ID":"boot",
               "_UID":"995","SYSLOG_IDENTIFIER":"device-frontend","_TRANSPORT":"stdout"}
        terminal=dict(ready,MESSAGE="STOPPED frontend",__MONOTONIC_TIMESTAMP="110")
        del terminal["_SYSTEMD_UNIT"]
        for mode in ("valid","stream","uid","boot","identifier","transport","foreign_unit","old_start","no_start"):
            with self.subTest(mode=mode):
                r,t=copy.deepcopy(ready),copy.deepcopy(terminal)
                if mode in ("stream","uid","boot","identifier","transport"):
                    key={"stream":"_STREAM_ID","uid":"_UID","boot":"_BOOT_ID","identifier":"SYSLOG_IDENTIFIER","transport":"_TRANSPORT"}[mode]
                    t[key]="other"
                if mode=="foreign_unit":t["_SYSTEMD_UNIT"]="other.service"
                if mode=="old_start":r["__MONOTONIC_TIMESTAMP"]="90"
                rows=[t] if mode=="no_start" else [r,t]
                m=Mock();m.command.return_value=b"\n".join(json.dumps(row).encode() for row in rows)
                self.assertEqual(u.receipt(m,"remoteapp-frontend",123,100,"STOPPED frontend"), mode=="valid")
                self.assertNotIn("-u",m.command.call_args.args[0])


if __name__ == "__main__": unittest.main()
