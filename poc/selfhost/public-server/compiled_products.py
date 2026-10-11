"""Trusted test coordinator for compiled products, not a public peer API.

Public service bootstrap is received over management SSH. Runtime roots,
endpoints and sibling digests remain compiled. HMAC/grant/authkey are carried
only in private pipes. Normal baseline stops before revocation; explicit
live-revoke mode observes autonomous completion without a parent stop command.
Explicit authority-fault mode uses the isolated coordinator only. None of
these modes claims GUI, signed installer or live upgrade acceptance.
"""
import base64
import copy
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
from coordination_diagnostics import run_stage

def verify_reconnect_authorization(apis, inputs, binding):
    """Private trusted APIs revalidate the SAME still-active grant, not a new
    invitation/approval. Every runtime independently verifies it again. No
    cached lease/unsigned active response suffices to reopen a desktop owner.
    """
    if len(apis)!=2 or set(inputs)!={"target","controller"}:
        raise RuntimeError("compiled_reconnect_pair_invalid")
    grant=inputs["target"].get("Grant")
    if not isinstance(grant,dict) or grant!=inputs["controller"].get("Grant"):
        raise RuntimeError("compiled_reconnect_grant_mismatch")
    for role, api in zip(("target","controller"),apis):
        if inputs[role].get("Binding")!=binding or inputs[role].get("Server") is not (role=="target"):
            raise RuntimeError("compiled_reconnect_binding_mismatch")
        # Actual helper refuses substitution even with a valid foreign grant.
        denied=copy.deepcopy(grant); denied["signature"]="invalid-reconnect-signature"
        negative=api.request("recheck_grant",Grant=denied)
        if (set(negative)!={"status","passed"} or negative["status"]!="recheck_grant" or negative["passed"] is not False):
            raise RuntimeError("compiled_reconnect_invalid_grant_not_rejected")
        positive=api.request("recheck_grant",Grant=grant)
        if (set(positive)!={"status","passed"} or positive["status"]!="recheck_grant" or positive["passed"] is not True):
            raise RuntimeError("compiled_reconnect_grant_not_valid")
        fresh=api.request("network_state")
        claims=fresh.get("network_state",{}).get("claims",{})
        if (fresh.get("passed") is not True or claims.get("active") is not True
                or claims.get("binding")!=binding or claims.get("grant")!=grant):
            raise RuntimeError("compiled_reconnect_signed_binding_not_active")


def fault_round(t, coordinator, apis, node, pair):
    """Observe fault closure BEFORE recovery, without stop or owner revoke.

    Recovery is of the isolated authority/verifier only. A fresh client owner
    and fresh authorization are required for a new product session; this
    entry deliberately does not relaunch either retired product owner.
    """
    from split_tls import fault_ack, wait
    binding = node["binding"]
    expected = {"acls": [{"action": "accept", "src": [binding["controller_ip"]],
                          "dst": [binding["target_ip"] + ":47476"]}]}
    t.check(t.admin("policy", "get") == expected, "compiled_fault_exact_acl_before_pause")
    t.check(fault_ack(coordinator.request("freeze"), "frozen"),
            "compiled_isolated_authority_paused_https503_old_acl_retained")
    t.check(t.admin("policy", "get") == expected, "compiled_fault_old_acl_before_join")
    pair.join_fault()  # Actual product/watch/node/signer join + native retirement.
    t.check(t.admin("policy", "get") == expected, "compiled_fault_old_acl_after_join")
    t.check(fault_ack(coordinator.request("recover"), "recovered"),
            "compiled_fault_explicit_same_verifier_authority_restart")
    wait(lambda: t.admin("policy", "get") == expected,
         "compiled_fault_recovery_minimum_policy", t)
    for api in apis:
        fresh = api.request("network_state")
        claims = fresh.get("network_state", {}).get("claims", {})
        t.check(fresh.get("passed") is True and claims.get("active") is True
                and claims.get("binding") == binding,
                "compiled_fault_recovery_fresh_signed_same_binding")
    pair.assert_fault_latched()
    time.sleep(.5)
    pair.assert_fault_latched()
    t.check(True, "compiled_fault_old_product_owners_remain_closed_after_recovery")


def session_state(value):
    """Completion telemetry is never a close ACK or a successful exit."""
    if (not isinstance(value, dict) or set(value) != {"status", "joined", "fault"}
            or value["status"] != "session_state"
            or type(value["joined"]) is not bool or type(value["fault"]) is not bool
            or (value["fault"] and not value["joined"])):
        raise RuntimeError("invalid_session_completion_state")
    return value["joined"], value["fault"]


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


def upload_public_bundle(t, folder, destination, reuse_folder):
    """Reuse only exact manifest-matching PUBLIC files, never state/credentials.

    The new destination must not exist. A running creator's public helpers may
    be read but are never modified. Final per-file verification is mandatory.
    """
    folder = Path(folder)
    manifest = dict(verify_bundle(folder))
    if "SHA256.json" in manifest:
        raise RuntimeError("invalid_self_referencing_manifest")
    manifest["SHA256.json"] = hashlib.sha256((folder / "SHA256.json").read_bytes()).hexdigest()
    encoded = base64.b64encode(json.dumps(manifest).encode()).decode()
    quote = lambda value: "'" + value.replace("'", "''") + "'"
    script = ("$ErrorActionPreference='Stop'; $dest=" + quote(destination) +
              "; $source=" + quote(reuse_folder) +
              "; if (Test-Path -LiteralPath $dest) {throw 'destination_exists'}; "
              "New-Item -ItemType Directory -Path $dest | Out-Null; "
              "$manifest=[Text.Encoding]::UTF8.GetString([Convert]::FromBase64String('" + encoded +
              "')) | ConvertFrom-Json; $needed=@(); $reused=@(); "
              "foreach ($p in $manifest.PSObject.Properties) { $src=Join-Path $source $p.Name; "
              "$dst=Join-Path $dest $p.Name; "
              "if ((Test-Path -LiteralPath $src -PathType Leaf) -and "
              "(Get-FileHash -LiteralPath $src -Algorithm SHA256).Hash.ToLowerInvariant() -eq $p.Value) { "
              "Copy-Item -LiteralPath $src -Destination $dst; "
              "if ((Get-FileHash -LiteralPath $dst -Algorithm SHA256).Hash.ToLowerInvariant() -ne $p.Value) "
              "{throw 'copy_hash_failed'}; $reused += $p.Name "
              "} else { $needed += $p.Name } }; "
              "@{needed=@($needed);reused=@($reused)} | ConvertTo-Json -Compress")
    result = json.loads(t.powershell(script))
    if (not isinstance(result, dict) or set(result) != {"needed", "reused"}
            or not all(isinstance(result[key], list) for key in result)):
        raise RuntimeError("invalid_public_reuse_result")
    names = result["needed"] + result["reused"]
    if any(not isinstance(name, str) for name in names) or len(names) != len(set(names)) or set(names) != set(manifest):
        raise RuntimeError("invalid_public_reuse_partition")
    if result["needed"]:
        copied = subprocess.run(["scp", "-O", "-C", "-o", "BatchMode=yes", "-o", "IdentitiesOnly=yes",
                                 "-o", "StrictHostKeyChecking=yes", "-i", str(Path.home() / ".ssh/remoteapp_win_ed25519"),
                                 *[str(folder / name) for name in result["needed"]],
                                 "jarvis@100.73.4.118:" + destination.replace("\\", "/") + "/"],
                                capture_output=True, timeout=180)
        t.check(copied.returncode == 0, "compiled_different_public_files_uploaded")
    for name, digest in manifest.items():
        actual = t.powershell("(Get-FileHash -LiteralPath " + quote(destination + "\\" + name) + " -Algorithm SHA256).Hash")
        if actual.lower() != digest:
            raise RuntimeError("compiled_remote_package_integrity_failed")
    t.check(True, "compiled_windows_all_files_verified_after_public_reuse")
    return result


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


def wait_borrower(t, identity, destination, trace, deadline):
    while time.monotonic() < deadline:
        raw = t.powershell("$ErrorActionPreference='Stop'; " +
                           f"if (Test-Path -LiteralPath '{destination}\\desktop-bootstrap.json') " +
                           f"{{[Convert]::ToBase64String([IO.File]::ReadAllBytes('{destination}\\desktop-bootstrap.json'))}}")
        if time.monotonic() >= deadline:
            break
        if raw:
            return checked_borrower(json.loads(base64.b64decode(raw, validate=True)), identity, destination, trace)
        time.sleep(1)
    raise RuntimeError("borrower_ready_deadline")


def prepare(t, identities, ready, ports, work, readiness_deadline, desktop_stage=None):
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
    upload_public_bundle(t, out / "win", destination, os.environ["REMOTEAPP_WINDOWS_INTERACTIVE_DIR"])
    t.check(True, "compiled_windows_fresh_pair_uploaded")
    identity_folder = os.environ["REMOTEAPP_WINDOWS_INTERACTIVE_DIR"].replace("'", "''")
    if desktop_stage is not None:
        if destination != desktop_stage.root + "\\run\\compiled-win":
            raise RuntimeError("desktop_stage_destination_mismatch")
        desktop_stage.launch(hashlib.sha256((out / "win/SHA256.json").read_bytes()).hexdigest())
        print("ACTION: prestarted desktop waiter will launch the verified fresh borrower; no manual startup during the service window.", flush=True)
    else:
        print("ACTION: normal Windows desktop, second PowerShell window (keep creator READY):", flush=True)
        print("powershell.exe -NoProfile -ExecutionPolicy Bypass -File '" + destination +
              "\\start-borrowed-desktop-session.ps1' -IdentityBootstrap '" + identity_folder +
              "\\interactive-bootstrap.json'", flush=True)
        print("ACTION: wait for borrower READY; do not press Enter in either window yet.", flush=True)
    trace = json.loads((out / "win/build-public.json").read_bytes())
    desktop = wait_borrower(t, identities.target, destination, trace, readiness_deadline)
    # Public JSON alone is not readiness: probe the real pipe before enrollment.
    return CompiledPair(t, out / "mac", destination, desktop, identities)


class CompiledPair:
    def __init__(self, t, mac, windows, desktop, identities):
        self.t, self.mac, self.windows = t, Path(mac), windows
        self.desktop, self.identities = desktop, identities
        self.target = self.controller = None
        self.mac_initialized = False
        self.sessions_joined = False
        self._reconnect_inputs = None
        self._reconnect_attempted = False
        self._retired_controller_proc = None
        self._last_product_key_digest = None

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

    def run(self, Pipe, handoff, live_revoke=False, authority_fault=False, reconnect=False):
        if live_revoke and authority_fault:
            raise RuntimeError("compiled_conflicting_live_modes")
        if reconnect and not authority_fault:
            raise RuntimeError("compiled_reconnect_requires_fault")
        # Obtain private inputs only after old node owners actually joined.
        inputs = run_stage("bound_handoff",handoff.take)
        run_stage("mac_native_initialize",lambda:self.maintenance("initialize"))
        self.mac_initialized = True
        if reconnect:
            self._reconnect_inputs = copy.deepcopy(inputs) # private memory only
        self._start_products(Pipe, inputs)
        if live_revoke or authority_fault:
            if session_state(self.target.request("session_state")) != (False, False):
                raise RuntimeError("compiled_target_not_live_before_revoke")
            if session_state(self.controller.request("session_state")) != (False, False):
                raise RuntimeError("compiled_controller_not_live_before_revoke")
            self.t.check(True, "compiled_both_sessions_live_before_authority_fault" if authority_fault else "compiled_both_sessions_live_before_signed_revoke")
            return
        self.stop_sessions()
        self.t.check(True, "compiled_product_node_watch_signer_normal_join")

    def _start_products(self, Pipe, inputs):
        # Mark ownership pending BEFORE the first send/launch can create work.
        self.sessions_joined = False
        run_stage("target_start_send",lambda:self.target.send(dict(Op="start", Config=inputs["target"])))
        key = run_stage("target_product_ready",lambda:product_secret(self.target.read()))
        digest=hashlib.sha256(base64.b64decode(key,validate=True)).digest()
        if digest==self._last_product_key_digest:
            raise RuntimeError("compiled_reconnect_product_key_reused")
        self._last_product_key_digest=digest # memory only, never logged
        def target_ready():
            if self.target.read() != dict(status="ready", port=0):
                raise RuntimeError("compiled_target_not_ready")
        run_stage("target_session_ready",target_ready)
        identity = self.identities.controller
        self.controller = run_stage("controller_start",lambda:Pipe([str(self.mac / "installed-session")],
                               dict(Op="start", Config=inputs["controller"], ProductKey=key,
                                    VaultScope=identity["VaultScope"], VaultExpected=identity["ExpectedPublic"])))
        key = None  # Never output/persist/retain it in the pair's public state.
        def controller_ready():
            ready = self.controller.read()
            if set(ready) != {"status", "port"} or ready["status"] != "ready" or type(ready["port"]) is not int or not 0 < ready["port"] < 65536:
                raise RuntimeError("compiled_controller_not_ready")
        run_stage("controller_session_ready",controller_ready)
        run_stage("product_observe",lambda:observe(self.target, self.controller))
        self.t.check(True, "compiled_actual_product_pongs_continue_beyond_initial_lease")

    def reconnect(self, Pipe, apis, binding):
        if self._reconnect_attempted or self._reconnect_inputs is None:
            raise RuntimeError("compiled_reconnect_not_available")
        self._reconnect_attempted = True # failed attempts never silently retry
        inputs, self._reconnect_inputs = self._reconnect_inputs, None
        self.assert_fault_latched()
        if self._retired_controller_proc is None or self._retired_controller_proc.poll() != 1:
            raise RuntimeError("compiled_old_controller_not_exited")
        run_stage("reconnect_authorization",lambda:verify_reconnect_authorization(apis, inputs, binding))
        run_stage("reconnect_mac_inspect",lambda:self.maintenance("inspect")) # no reinitialize/floor reset
        def reopen():
            ack=self.target.request("reopen_fault")
            if (not isinstance(ack,dict) or set(ack)!={"status","started"}
                    or ack["status"]!="owner_reopened" or ack["started"] is not False):
                raise RuntimeError("compiled_reconnect_owner_ack_invalid")
            if session_state(self.target.request("session_state")) != (False,False):
                raise RuntimeError("compiled_reconnect_owner_not_idle")
        run_stage("reconnect_target_reopen",reopen)
        self._start_products(Pipe,inputs) # fresh target HMAC and fresh Mac process
        if self.controller.proc.pid == self._retired_controller_proc.pid or self._retired_controller_proc.poll()!=1:
            raise RuntimeError("compiled_reconnect_reused_controller")
        self.stop_sessions()
        self.t.check(True,"compiled_explicit_new_owner_product_reconnect_and_normal_join")

    def revoked(self):
        self.join_fault()

    def join_fault(self):
        # No stop command/cancellation is sent before autonomous completion.
        self.controller.join_fault()
        if self._reconnect_inputs is not None:
            self._retired_controller_proc = self.controller.proc
        self.controller = None
        self.t.check(True, "compiled_controller_autonomous_fault_join_and_native_retirement")
        end = time.monotonic() + 20
        while time.monotonic() < end:
            joined, fault = session_state(self.target.request("session_state"))
            if joined:
                if not fault:
                    raise RuntimeError("compiled_target_unexpected_normal_completion")
                break
            time.sleep(.2)
        else:
            raise RuntimeError("compiled_target_autonomous_join_deadline")
        # Stable terminal state cannot become a freshly active old owner.
        time.sleep(.5)
        if session_state(self.target.request("session_state")) != (True, True):
            raise RuntimeError("compiled_target_terminal_state_regressed")
        if self.target.request("retire_fault") != {"status":"fault_retired", "joined":True, "fault":True}:
            raise RuntimeError("compiled_target_native_fault_retirement_unconfirmed")
        self.t.check(True, "compiled_target_autonomous_fault_join_and_native_retirement")
        self.maintenance("inspect")  # actual controller lease released
        self.sessions_joined = True

    def assert_fault_latched(self):
        if (not self.sessions_joined or self.controller is not None or
                session_state(self.target.request("session_state")) != (True, True)):
            raise RuntimeError("compiled_fault_owner_reopened")
        self.maintenance("inspect")

    def stop_sessions(self):
        if self.controller is not None:
            run_stage("controller_normal_stop",self.controller.stop)
            self.controller = None
        if self.target is not None:
            def stop_target():
                if self.target.request("stop_session") != dict(status="session_closed"):
                    raise RuntimeError("compiled_target_join_unconfirmed")
            run_stage("target_normal_stop",stop_target)
        self.sessions_joined = True

    def close(self):
        self._reconnect_inputs = None
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
