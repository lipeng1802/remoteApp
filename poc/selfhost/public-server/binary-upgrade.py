"""Explicit root-local binary-only prerelease upgrade; Python 2 compatible.

No Store restoration, registration, control-plane reset, lock deletion or kill.
Every step is explicit. Failure preserves checkpoint; no automatic recovery.
"""
from __future__ import print_function
import hashlib, imp, json, os, re, signal, stat, sys, time

NAMES = ("authority-daemon", "device-frontend", "network-deployment-check")
ROOT = "5b74a1a3b2b6c477b81016d71c38275d9fdd17d01e6c33ec7847dcd892a5198f"
DEST = "/opt/remoteapp-prerelease/"
BACKUPS = "/var/lib/remoteapp-binary-upgrades"
AUTH_READY = "READY sole authority; supervised network enabled on private broker"
FRONT_READY = "READY unprivileged frontend; explicit direct signed network mode"
PINNED = ("/etc/systemd/system/remoteapp-authority.service",
          "/etc/systemd/system/remoteapp-frontend.service",
          "/etc/remoteapp-authority/network.json",
          "/etc/remoteapp-authority/service-profile.json",
          "/etc/remoteapp-poc/config.yaml")

def read(path):
    with open(path, "rb") as f: return f.read()

def service_identity(m, service):
    raw = m.command(["systemctl", "show", service, "-p", "MainPID", "-p", "ActiveState", "-p", "ExecMainStartTimestampMonotonic"])
    p = dict(line.split("=", 1) for line in raw.decode().splitlines())
    return int(p["MainPID"]), int(p.get("ExecMainStartTimestampMonotonic", "0")), p["ActiveState"]

def receipt(m, service, pid, started, label):
    # Current boot + current ExecStart, never a prior process with a reused PID.
    m.require(pid > 1 and started > 0, "service_identity_missing")
    raw = m.command(["journalctl", "--no-pager", "-b", "-o", "json", "-u", service, "_PID=" + str(pid)])
    for line in raw.decode().splitlines():
        row = json.loads(line)
        if row.get("MESSAGE") == label and str(row.get("_PID")) == str(pid) and int(row.get("__MONOTONIC_TIMESTAMP", "0")) >= started:
            return True
    return False

def wait_ready(m, service, label):
    for unused in range(450):
        pid, started, active = service_identity(m, service)
        m.require(active != "failed", "candidate_failed")
        if pid > 1 and started > 0 and active == "active" and receipt(m, service, pid, started, label):
            m.require(service_identity(m, service) == (pid, started, active), "service_identity_changed")
            return pid
        time.sleep(.1)
    raise RuntimeError("candidate_not_ready")

def graceful_stop(m, service, label):
    pid, started, active = service_identity(m, service)
    m.require(pid > 1 and started > 0 and active == "active", "service_not_active")
    # systemctl stop signals the entire control group on this CentOS unit.
    # TERM only the main owner first so it can join its own nested workers.
    # Never kill a worker or count a systemd inactive state as the receipt.
    os.kill(pid, signal.SIGTERM)
    for unused in range(450):
        if m.gone(pid) and receipt(m, service, pid, started, label):
            m.system("stop", service)
            m.require(m.properties(service)["MainPID"] == "0", "service_still_live")
            return
        time.sleep(.1)
    raise RuntimeError("stop_not_joined")

def private(path, directory=False):
    i = os.lstat(path)
    return (os.path.realpath(path) == path and i.st_uid == 0 and
            stat.S_IMODE(i.st_mode) == (0o700 if directory else 0o600) and
            (stat.S_ISDIR(i.st_mode) if directory else stat.S_ISREG(i.st_mode)))

def load(stage):
    if os.geteuid() != 0 or not re.match(r"^/run/remoteapp-binary-upgrade-[0-9a-f]{16}\Z", stage) or not private(stage, True):
        raise RuntimeError("invalid_stage")
    manifest = json.loads(read(stage + "/manifest.json"))
    if set(manifest) != set(("version", "sha256", "previous")) or manifest["version"] != 1 or set(manifest["sha256"]) != set(NAMES + ("network-maintenance.py",)) or set(manifest["previous"]) != set(NAMES):
        raise RuntimeError("invalid_manifest")
    for name, expected in manifest["sha256"].items():
        path = stage + "/" + name
        i = os.lstat(path)
        if not re.match(r"^[0-9a-f]{64}\Z", expected) or not stat.S_ISREG(i.st_mode) or i.st_uid != 0 or i.st_mode & 0o022 or os.path.realpath(path) != path or hashlib.sha256(read(path)).hexdigest() != expected:
            raise RuntimeError("candidate_mismatch")
    m = imp.load_source("upgrade_primitives", stage + "/network-maintenance.py")
    return manifest, m

def main(stage, mode):
    os.umask(0o077)
    manifest, m = load(stage)
    backup = BACKUPS + "/" + os.path.basename(stage)
    def save(s): m.save(backup, s)
    def unchanged(s):
        m.require(all(m.digest(p) == h for p, h in s["config"].items()), "configuration_changed")
        m.require(m.digest(m.STORE) == s["store"], "store_changed_no_restore")
        m.require(m.identity() == s["identity"], "verifier_identity_changed")
        m.require(m.digest(m.VD + "/journal.json") == s["verifier_journal"], "verifier_journal_changed")
        m.zero_policy()
        m.command(["sha256sum", "--check", DEST + "nginx-baseline.sha256"])
    def idle():
        for service in ("remoteapp-frontend", "remoteapp-authority"):
            p = m.properties(service)
            m.require(p["MainPID"] == "0" and p["ActiveState"] in ("inactive", "failed"), "service_still_live")
        m.no_locks()
        m.zero_policy()
    def check():
        out = m.command([stage + "/authority-daemon", "/etc/remoteapp-authority/service-profile.json", ROOT, "remoteapp-prerelease", "1", "/var/lib/remoteapp-authority", "/run/remoteapp-authority/authority.sock", "995", "993", "--check-network-config", "/etc/remoteapp-authority/network.json"])
        m.require(out.startswith(b"CHECK PASSED:"), "idle_precheck_failed")
    if mode == "prepare":
        m.require(not os.path.lexists(backup), "existing_work_order")
        for service in ("remoteapp-authority", "remoteapp-frontend", "remoteapp-poc"):
            m.require(m.properties(service)["ActiveState"] == "active", "service_not_active")
        m.require(all(m.digest(DEST + n) == manifest["previous"][n] for n in NAMES), "previous_binary_mismatch")
        snapshot = json.loads(read(m.STORE))
        m.require(not snapshot.get("Provisions") and not snapshot.get("TLSIdentities"), "existing_network_work")
        # Stop before any currently valid consent grant could be interrupted.
        m.require(not any(v.get("Grant") and v["Grant"]["claims"]["expires_at"] > time.time() for v in snapshot.get("Requests", {}).values()), "live_grant")
        parent = os.path.dirname(backup)
        if not os.path.exists(parent): os.mkdir(parent, 0o700)
        m.require(private(parent, True), "unsafe_backup_parent")
        os.mkdir(backup, 0o700)
        s = {"phase": "prepared", "store": m.digest(m.STORE), "identity": m.identity(), "verifier_journal": m.digest(m.VD + "/journal.json"), "config": dict((p, m.digest(p)) for p in PINNED), "original": {}, "candidate": manifest["sha256"]}
        for n in NAMES:
            p = DEST + n
            i = os.lstat(p)
            m.require(stat.S_ISREG(i.st_mode) and os.path.realpath(p) == p and i.st_uid == 0 and not i.st_mode & 0o022, "unsafe_binary")
            s["original"][n] = {"hash": m.digest(p), "mode": stat.S_IMODE(i.st_mode), "uid": i.st_uid, "gid": i.st_gid}
            m.atomic(backup + "/" + n, read(p), 0o600)
        unchanged(s)
        save(s)
    else:
        m.require(private(backup, True) and private(backup + "/checkpoint.json"), "missing_work_order")
        s = json.loads(read(backup + "/checkpoint.json"))
        m.require(s["candidate"] == manifest["sha256"], "work_order_candidate_changed")
        unchanged(s)
        if mode in ("quiesce", "retry-quiesce"):
            allowed = ("stopping",) if mode == "retry-quiesce" else ("prepared", "verified", "started")
            m.require(s["phase"] in allowed, "wrong_phase")
            # Retry requires both services explicitly restored and healthy;
            # it cannot bless an earlier uncertain stop or delete a lock.
            s["service_pids"] = dict((n, wait_ready(m, n, label)) for n, label in (("remoteapp-frontend", FRONT_READY), ("remoteapp-authority", AUTH_READY)))
            s["phase"] = "stopping"; save(s)
            graceful_stop(m, "remoteapp-frontend", "STOPPED frontend")
            graceful_stop(m, "remoteapp-authority", "STOPPED authority")
            idle(); unchanged(s)
            s["phase"] = "stopped"; save(s)
        elif mode == "check":
            m.require(s["phase"] == "stopped", "wrong_phase")
            idle(); check(); unchanged(s)
            s["phase"] = "checked"; save(s)
        elif mode == "install":
            m.require(s["phase"] == "checked", "wrong_phase")
            idle(); check()
            m.require(all(m.digest(DEST+n) == s["original"][n]["hash"] for n in NAMES), "external_binary_change")
            s["phase"] = "installing"; save(s)
            for n in NAMES: m.atomic(DEST + n, read(stage + "/" + n), s["original"][n]["mode"], s["original"][n]["uid"], s["original"][n]["gid"])
            unchanged(s)
            s["phase"] = "installed"; save(s)
        elif mode == "start":
            m.require(s["phase"] in ("installed", "rolled_back"), "wrong_phase")
            idle(); unchanged(s)
            wanted = dict((n, s["original"][n]["hash"]) for n in NAMES) if s["phase"] == "rolled_back" else s["candidate"]
            m.require(all(m.digest(DEST+n) == wanted[n] for n in NAMES), "binary_changed_before_start")
            s["phase"] = "starting"; save(s)
            m.system("start", "remoteapp-authority"); wait_ready(m, "remoteapp-authority", AUTH_READY)
            m.system("start", "remoteapp-frontend"); wait_ready(m, "remoteapp-frontend", FRONT_READY)
            unchanged(s)
            s["phase"] = "started"; save(s)
        elif mode == "verify":
            m.require(s["phase"] in ("started", "verified"), "wrong_phase")
            wait_ready(m, "remoteapp-authority", AUTH_READY); wait_ready(m, "remoteapp-frontend", FRONT_READY)
            m.probe("active"); unchanged(s)
            s["phase"] = "verified"; save(s)
        elif mode == "rollback":
            m.require(s["phase"] in ("stopped", "checked", "installing", "installed", "starting"), "wrong_phase")
            idle(); unchanged(s)
            for n in NAMES:
                old = s["original"][n]
                m.require(m.digest(backup+"/"+n) == old["hash"] and m.digest(DEST+n) in (old["hash"], s["candidate"][n]), "rollback_binary_changed")
            for n in NAMES:
                old = s["original"][n]
                m.atomic(DEST+n, read(backup+"/"+n), old["mode"], old["uid"], old["gid"])
            s["phase"] = "rolled_back"; save(s)
        else: raise RuntimeError("unknown_step")
    print("PASS binary_upgrade_" + mode)

if __name__ == "__main__":
    try:
        if len(sys.argv) != 3: raise RuntimeError("arguments")
        main(sys.argv[1], sys.argv[2])
    except Exception as e:
        if isinstance(e, RuntimeError) and re.match(r"^[a-z_]+\Z", str(e)): print("DIAGNOSTIC binary_upgrade_" + str(e))
        print("FAIL binary_upgrade_paused_state_preserved")
        sys.exit(1)
