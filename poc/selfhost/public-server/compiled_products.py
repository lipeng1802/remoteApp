"""Trusted test coordinator for compiled products, not a public peer API.

Public service bootstrap is received over management SSH. Runtime roots,
endpoints and sibling digests remain compiled. HMAC/grant/authkey are carried
only in private pipes. This first round stops normally BEFORE owner revocation;
it deliberately does not claim live-revocation/fault/upgrade acceptance.
"""
import base64
import hashlib
import json
import os
from pathlib import Path
import platform
import re
import secrets
import subprocess
import time

from native_bootstrap import checked_desktop_bootstrap


def checked_borrower(value, identity, folder, trace):
    desktop = checked_desktop_bootstrap(value, identity, folder)
    release = trace.get("release", {})
    channel = release.get("channel", "")
    expected_scope = hashlib.sha256(("remoteapp-installed-trust-channel-v1\n" + channel).encode()).hexdigest()[:32]
    if (release.get("test_namespace") is not True or value["channel"] != channel
            or value["epoch"] != 1 or value["scope"] != expected_scope):
        raise RuntimeError("borrower_compiled_channel_mismatch")
    return desktop


def build_helpers(t):
    # Do not launch a stale daemon artifact that lacks --os-compiled. These
    # builds use current trusted source and the local offline Go toolchain.
    parent = t.ARTIFACTS.resolve()
    go = parent / "toolchain/go/bin/go"
    base = dict(os.environ, GOENV="off", GOTOOLCHAIN="local", GOPROXY="off",
                GOMODCACHE=str(parent / "gomod"), GOCACHE=str(parent / "gocache"))
    builds = (("linux", "daemon-network-check", "daemon-network-check"),
              ("linux", "authority-daemon", "authority-daemon-network"),
              ("linux", "device-frontend", "device-frontend-private"),
              ("windows", "device-api-client", "device-api-client-win.exe"),
              ("windows", "enrollment-helper", "enrollment-helper-win.exe"),
              ("windows", "identity-vault", "identity-vault-win.exe"),
              ("windows", "trust-journal", "trust-journal-win.exe"),
              ("darwin", "device-api-client", "device-api-client"),
              ("darwin", "enrollment-helper", "enrollment-helper"))
    mac_arch = {"arm64":"arm64", "x86_64":"amd64"}.get(platform.machine())
    if mac_arch is None:
        raise RuntimeError("unsupported_mac_build_architecture")
    for os_name, package, name in builds:
        env = dict(base, GOOS=os_name, GOARCH="amd64" if os_name != "darwin" else mac_arch, CGO_ENABLED="0")
        p = subprocess.run([str(go), "build", "-o", str(parent / name), "./cmd/" + package],
                           cwd=t.HERE, env=env, capture_output=True, timeout=120)
        t.check(p.returncode == 0, "compiled_current_" + os_name + "_" + package + "_built")


def verify_bundle(folder):
    folder = Path(folder)
    manifest = json.loads((folder / "SHA256.json").read_bytes())
    if not isinstance(manifest, dict) or not manifest:
        raise RuntimeError("invalid_compiled_manifest")
    for name, digest in manifest.items():
        if (not re.fullmatch(r"[A-Za-z0-9_.-]+", name) or name in (".", "..")
                or not isinstance(digest, str) or not re.fullmatch(r"[0-9a-f]{64}", digest)):
            raise RuntimeError("invalid_compiled_manifest_entry")
        file = folder / name
        if file.is_symlink() or hashlib.sha256(file.read_bytes()).hexdigest() != digest:
            raise RuntimeError("compiled_bundle_integrity_failed")
    return manifest


def product_secret(value):
    if set(value) != {"status", "Key", "Fingerprint"} or value["status"] != "product_ready":
        raise RuntimeError("invalid_private_product_ready")
    key = value["Key"]
    raw = base64.b64decode(key, validate=True)
    if (len(raw) != 32 or base64.b64encode(raw).decode() != key
            or not re.fullmatch(r"[0-9a-f]{64}", value["Fingerprint"])):
        raise RuntimeError("invalid_private_product_secret")
    return key


def product_count(value, controller=False):
    allowed = {"status", "pongs", "preauth_rejected", "sessions_closed", "ended", "result", "pin_rejected"}
    if (not isinstance(value, dict) or set(value) - allowed
            or value.get("status") != "product_count"
            or type(value.get("pongs")) is not int or value["pongs"] < 0
            or value.get("ended") is not False or value.get("pin_rejected") is not False):
        raise RuntimeError("product_not_live")
    for name in ("preauth_rejected", "sessions_closed"):
        if type(value.get(name, 0)) is not int or value.get(name, 0) < 0:
            raise RuntimeError("invalid_product_counter")
    if controller:
        if value.get("result") != "pending" or value.get("preauth_rejected", 0) or value.get("sessions_closed", 0):
            raise RuntimeError("controller_not_live")
    elif "result" in value:
        raise RuntimeError("target_role_mismatch")
    return value["pongs"]


def observe(target, controller, clock=time.monotonic, sleep=time.sleep):
    # A ready message/one positive count cannot pass. Require both products to
    # keep exchanging on the same session across the initial short watch lease.
    end = clock() + 12
    previous = (0, 0)
    first = None
    while clock() < end:
        pair = (product_count(target.request("count")),
                product_count(controller.request("count"), True))
        if any(n < p for n, p in zip(pair, previous)):
            raise RuntimeError("product_counter_regressed")
        previous = pair
        if first is None and min(pair) >= 3:
            first = (clock(), pair)
        if first is not None and clock() - first[0] >= 5:
            if all(n > p + 10 for n, p in zip(pair, first[1])):
                return
            raise RuntimeError("product_payload_did_not_continue")
        sleep(.2)
    raise RuntimeError("compiled_payload_deadline")


def prepare(t, identities, ready, ports, work):
    """Fresh matching public test material, then normal-desktop borrower READY.

    Runs before invitation/grant. Never invokes the borrower under SSH (native
    Credential Manager requires normal logon); stdout prints public paths only.
    """
    stamp = secrets.token_hex(8)
    parent = t.ARTIFACTS.resolve()
    out = parent / ("compiled-live-" + stamp)
    source = parent / ("compiled-input-" + stamp + ".json")
    payload = dict(version=1, certificate=ready["api_certificate"], issuer=ready["issuer"],
                   target_endpoint=f"https://127.0.0.1:{ports[0]}",
                   controller_endpoint=f"https://127.0.0.1:{ports[1]}")
    with source.open("x") as file:
        json.dump(payload, file)
    go = parent / "toolchain/go/bin/go"
    root = t.HERE.parents[1]
    args = {"go": go, "module": t.HERE, "input": source, "out": out,
            "mac-journal": parent / "trust-journal-mac", "win-journal": parent / "trust-journal-win.exe",
            "mac-vault": root / "macos/RemoteController/.build/release/ConnectionIdentityVault",
            "win-vault": parent / "identity-vault-win.exe",
            "mac-product": root / "macos/RemoteController/.build/release/TLSBridgeProbe",
            "win-product": parent / "TlsBridgeProbe-win-4s.exe"}
    env = dict(os.environ, GOENV="off", GOTOOLCHAIN="local", GOPROXY="off",
               GOMODCACHE=str(parent / "gomod"), GOCACHE=str(parent / "gocache"))
    command = [str(go), "run", "./cmd/compiled-pair-build"]
    for name, path in args.items():
        command.extend(("--" + name, str(path)))
    built = subprocess.run(command, cwd=t.HERE, env=env, capture_output=True, timeout=250)
    t.check(built.returncode == 0 and b"BUILD_READY:" in built.stdout, "compiled_matching_fresh_pair_built")
    source.unlink()  # Generated public input only; preserve packages for diagnosis.
    verify_bundle(out / "mac")
    manifest = verify_bundle(out / "win")
    destination = work + "\\compiled-win"
    copied = subprocess.run(["scp", "-O", "-C", "-r", "-o", "BatchMode=yes", "-o", "IdentitiesOnly=yes",
                             "-o", "StrictHostKeyChecking=yes", "-i", str(Path.home() / ".ssh/remoteapp_win_ed25519"),
                             str(out / "win"), "jarvis@100.73.4.118:" + destination.replace("\\", "/")],
                            capture_output=True, timeout=180)
    t.check(copied.returncode == 0, "compiled_windows_fresh_pair_uploaded")
    for name, digest in manifest.items():
        actual = t.powershell(f"(Get-FileHash -LiteralPath '{destination}\\{name}' -Algorithm SHA256).Hash")
        if actual.lower() != digest:
            raise RuntimeError("compiled_remote_package_integrity_failed")
    identity_folder = os.environ["REMOTEAPP_WINDOWS_INTERACTIVE_DIR"].replace("'", "''")
    print("ACTION: normal Windows desktop, second PowerShell window (keep creator READY):", flush=True)
    print("powershell.exe -NoProfile -ExecutionPolicy Bypass -File '" + destination +
          "\\start-borrowed-desktop-session.ps1' -IdentityBootstrap '" + identity_folder +
          "\\interactive-bootstrap.json'", flush=True)
    print("ACTION: wait for borrower READY; do not press Enter in either window yet.", flush=True)
    deadline = time.monotonic() + 240
    while time.monotonic() < deadline:
        raw = t.powershell("$ErrorActionPreference='Stop'; " +
                           f"if (Test-Path -LiteralPath '{destination}\\desktop-bootstrap.json') " +
                           f"{{[Convert]::ToBase64String([IO.File]::ReadAllBytes('{destination}\\desktop-bootstrap.json'))}}")
        if raw:
            trace = json.loads((out / "win/build-public.json").read_bytes())
            value = json.loads(base64.b64decode(raw, validate=True))
            desktop = checked_borrower(value, identities.target, destination, trace)
            # The public JSON alone is not readiness; the actual local desktop
            # pipe is probed below before any invitation/enrollment.
            return CompiledPair(t, out / "mac", destination, desktop, identities)
        time.sleep(1)
    raise RuntimeError("borrower_ready_deadline")


class CompiledPair:
    def __init__(self, t, mac, windows, desktop, identities):
        self.t, self.mac, self.windows = t, Path(mac), windows
        self.desktop, self.identities = desktop, identities
        self.target = self.controller = None
        self.mac_initialized = False
        self.sessions_joined = False

    def attach(self, Pipe):
        d = self.desktop
        command = self.t.WIN + ['cmd.exe /d /c "set REMOTEAPP_DESKTOP_SESSION_PIPE=' + d["Pipe"] +
                               '&& ' + d["Executable"] + '"']
        self.target = Pipe(command, dict(Scope=d["Scope"], Expected=d["Expected"]))
        hello = self.target.read()
        if hello != dict(status="ready", public_key=d["Expected"]):
            raise RuntimeError("borrower_relay_hello_failed")
        if self.target.request("probe") != dict(status="probe_passed"):
            raise RuntimeError("borrower_native_probe_failed")
        self.t.check(True, "compiled_borrower_real_desktop_probe")

    def maintenance(self, op):
        verify_bundle(self.mac)
        p = subprocess.run([str(self.mac / "trust-install"), op], capture_output=True, timeout=20)
        value = json.loads(p.stdout)
        if p.returncode != 0 or value.get("status") != "usable" or (op != "cleanup-test" and value.get("epoch") != 1):
            raise RuntimeError("compiled_mac_maintenance_failed")

    def run(self, Pipe, handoff):
        # Obtain private inputs only after old node owners actually joined.
        inputs = handoff.take()
        self.maintenance("initialize")
        self.mac_initialized = True
        self.target.send(dict(Op="start", Config=inputs["target"]))
        key = product_secret(self.target.read())
        if self.target.read() != dict(status="ready", port=0):
            raise RuntimeError("compiled_target_not_ready")
        identity = self.identities.controller
        self.controller = Pipe([str(self.mac / "installed-session")],
                               dict(Op="start", Config=inputs["controller"], ProductKey=key,
                                    VaultScope=identity["VaultScope"], VaultExpected=identity["ExpectedPublic"]))
        key = None  # Never output/persist/retain it in the pair's public state.
        ready = self.controller.read()
        if set(ready) != {"status", "port"} or ready["status"] != "ready" or type(ready["port"]) is not int or not 0 < ready["port"] < 65536:
            raise RuntimeError("compiled_controller_not_ready")
        observe(self.target, self.controller)
        self.t.check(True, "compiled_actual_product_pongs_continue_beyond_initial_lease")
        # First end-to-end slice uses normal joined shutdown. Do not label this
        # live-revocation: consent() sends owner revoke only after we return.
        self.stop_sessions()
        self.t.check(True, "compiled_product_node_watch_signer_normal_join")

    def stop_sessions(self):
        if self.controller is not None:
            self.controller.stop()
            self.controller = None
        if self.target is not None:
            if self.target.request("stop_session") != dict(status="session_closed"):
                raise RuntimeError("compiled_target_join_unconfirmed")
        self.sessions_joined = True

    def close(self):
        # Do NOT release signer identities, delete state or restore policy if
        # any actual owner is still unjoined. No emergency kill is a success.
        if not self.sessions_joined:
            self.stop_sessions()
        if self.target is not None:
            self.target.proc.stdin.close()  # relay EOF only; desktop owner stays READY
            self.target.proc.wait(timeout=12)
            if self.target.proc.returncode != 0:
                raise RuntimeError("compiled_relay_join_unconfirmed")
            self.target = None
        if self.mac_initialized:
            self.maintenance("inspect")  # native lease must really be released
            self.maintenance("cleanup-test")
            self.mac_initialized = False
        print("PRESERVED_WINDOWS_COMPILED_PACKAGE " + self.windows, flush=True)
