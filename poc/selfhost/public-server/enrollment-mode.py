"""Server-local scoped policy-mode switch for the empty, isolated PoC service.

CentOS Python 2 compatible. Never prints config/key contents. Restore is explicit
and refuses populated control planes; no business Nginx operations.
"""
from __future__ import print_function
import json
import os
import shutil
import stat
import subprocess
import sys
import tempfile
import time

CONFIG = "/etc/remoteapp-poc/config.yaml"
HS = ["/opt/remoteapp-poc/headscale", "-c", CONFIG, "-o", "json"]

def command(args):
    p = subprocess.Popen(args, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    out, unused = p.communicate()
    if p.returncode:
        stage="policy" if "policy" in args else "restart" if "restart" in args else "admin"
        raise RuntimeError("management_failed_"+stage)
    return out

def empty():
    for kind in ("nodes", "users"):
        if json.loads(command(HS+[kind, "list"]) or "null"):
            raise RuntimeError("control_plane_not_empty")

def replace(contents):
    old = os.stat(CONFIG)
    fd, path = tempfile.mkstemp(prefix=".enrollment-", dir=os.path.dirname(CONFIG))
    try:
        with os.fdopen(fd, "wb") as f:
            f.write(contents)
            f.flush()
            os.fsync(f.fileno())
        os.chmod(path, stat.S_IMODE(old.st_mode))
        os.chown(path, old.st_uid, old.st_gid)
        os.rename(path, CONFIG)
    finally:
        if os.path.exists(path):
            os.unlink(path)

def ready():
    command(["systemctl", "restart", "remoteapp-poc"])
    for unused in range(30):
        try:
            command(HS+["nodes", "list"])
            return
        except RuntimeError:
            time.sleep(0.2)
    raise RuntimeError("restart_failed")

def main():
    os.umask(0o077)
    empty()
    if sys.argv[1:] == ["prepare"]:
        original = open(CONFIG, "rb").read()
        if original.count(b"  mode: file\n") != 1:
            raise RuntimeError("unexpected_policy_mode")
        backup = tempfile.mkdtemp(prefix="enrollment-mode-", dir="/var/lib/remoteapp-poc")
        shutil.copyfile(CONFIG, os.path.join(backup, "config.yaml"))
        # Print recoverable handle before mutation; callers keep it in memory.
        sys.stdout.write(json.dumps({"backup":backup})+"\n")
        sys.stdout.flush()
        try:
            replace(original.replace(b"  mode: file\n", b"  mode: database\n"))
            ready()
            policy = os.path.join(backup, "deny.json")
            with open(policy, "wb") as f:
                f.write(b'{"acls":[]}')
            command(HS+["policy", "set", "--file", policy])
            got = json.loads(command(HS+["policy", "get"]))
            if got != {"acls": []}:
                raise RuntimeError("deny_readback_failed")
            print(json.dumps({"status": "db_deny_ready"}))
        except Exception:
            replace(original)
            ready()
            raise
    elif len(sys.argv)==3 and sys.argv[1]=="restore":
        backup=sys.argv[2]
        if os.path.dirname(backup)!="/var/lib/remoteapp-poc" or not os.path.basename(backup).startswith("enrollment-mode-") or os.path.islink(backup):
            raise RuntimeError("invalid_backup")
        original=open(os.path.join(backup,"config.yaml"),"rb").read()
        if original.count(b"  mode: file\n")!=1:
            raise RuntimeError("invalid_original")
        replace(original)
        ready()
        shutil.rmtree(backup)
        print(json.dumps({"status":"file_mode_restored"}))
    else:
        raise RuntimeError("invalid_arguments")

if __name__ == "__main__":
    try:
        main()
    except Exception as e:
        print(json.dumps({"status":"failed","reason":str(e) if isinstance(e,RuntimeError) else "internal_error"}))
        sys.exit(1)
