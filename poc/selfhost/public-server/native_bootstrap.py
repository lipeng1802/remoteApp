"""Private OS identities for the trusted two-machine acceptance coordinator.

No signing/private-key interface in this parent. Windows owner remains in the
desktop session; two Mac random test identities are deleted after worker join.
"""
import base64
import json
import os
from pathlib import Path
import re
import secrets


def checked_windows_bootstrap(value, folder):
    expected = folder.rstrip("\\") + "\\vault-relay.exe"
    scope = value.get("scope", "")
    public = value.get("public_key", "")
    if (value.get("status") != "ready" or not re.fullmatch(r"[0-9a-f]{32}", scope)
            or not re.fullmatch(r"[0-9a-f]{64}", public)
            or value.get("pipe") != "\\\\.\\pipe\\remoteapp-vault-" + scope
            or value.get("relay_executable") != expected):
        raise RuntimeError("invalid_desktop_bootstrap")
    return dict(VaultExecutable=expected, VaultScope=scope, ExpectedPublic=public)


class NativeIdentities:
    def __init__(self, transport, pipe):
        self.owners = []
        self.transport = transport
        folder = os.environ.get("REMOTEAPP_WINDOWS_INTERACTIVE_DIR", "")
        if not re.match(r"^[A-Za-z]:\\", folder) or any(c in folder for c in "\r\n\0"):
            raise RuntimeError("trusted_windows_bundle_directory_required")
        literal = folder.replace("'", "''")
        script = ("& powershell.exe -NoProfile -ExecutionPolicy Bypass -File '"+literal+
                  "\\probe-interactive-vault.ps1'; if ($LASTEXITCODE -ne 0) {throw 'probe_failed'}")
        probe = transport.powershell(script)
        transport.check("PROBE PASSED:" in probe and "--- SKIP:" not in probe, "desktop_os_identity_probe_before_server_mutation")
        raw = transport.powershell("[Convert]::ToBase64String([IO.File]::ReadAllBytes('"+literal+"\\interactive-bootstrap.json'))")
        value = json.loads(base64.b64decode(raw, validate=True))
        self.target = checked_windows_bootstrap(value, folder)
        self.pipe_name = value["pipe"]
        exe = os.environ.get("REMOTEAPP_MAC_VAULT_EXECUTABLE", str(transport.HERE.parents[1] / "macos/RemoteController/.build/release/ConnectionIdentityVault"))
        if not Path(exe).is_absolute():
            raise RuntimeError("trusted_absolute_mac_vault_required")
        try:
            self.controller = self.create_mac(pipe, exe)
            self.wrong = self.create_mac(pipe, exe)
        except Exception:
            self.close()
            raise

    def create_mac(self, pipe, exe):
        scope = secrets.token_hex(16)
        owner = pipe([exe], {"Scope": scope, "Create": True, "Expected": ""})
        self.owners.append(owner)
        ready = owner.read()
        if ready.get("status") != "ready" or not re.fullmatch(r"[0-9a-f]{64}", ready.get("public_key", "")):
            raise RuntimeError("native_mac_identity_unavailable")
        return dict(VaultExecutable=exe, VaultScope=scope, ExpectedPublic=ready["public_key"])

    def config(self, role, windows):
        if windows:
            if role != "native-target":
                raise RuntimeError("wrong_platform_identity")
            return dict(self.target)
        if role not in ("native-controller", "native-target"):
            raise RuntimeError("unknown_native_identity")
        return dict(self.controller if role == "native-controller" else self.wrong)

    def command(self, work, executable):
        # pipe_name has been matched against a fixed prefix + 128-bit hex.
        return self.transport.WIN + [f'cmd.exe /d /c "set REMOTEAPP_INTERACTIVE_VAULT_PIPE={self.pipe_name}&& cd /d {work} && {executable}"']

    def close(self):
        failed = False
        for owner in reversed(self.owners):
            try:
                value = owner.request("delete")
                owner.proc.stdin.close()
                owner.proc.wait(timeout=8)
                if value.get("status") != "deleted" or owner.proc.returncode != 0:
                    failed = True
            except Exception:
                failed = True
                owner.emergency()
        self.owners.clear()
        if failed:
            raise RuntimeError("random_mac_identity_cleanup_unconfirmed")
