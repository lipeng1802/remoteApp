"""Public SSH-managed test staging, not runtime trust or network authority."""
import base64
import json
import re
import secrets
import time


def checked_ack(value, nonce):
    if value != {"nonce": nonce, "status": "waiting"}:
        raise RuntimeError("desktop_stage_ack_invalid")


def validate_stage_root(root):
    if not isinstance(root, str) or not re.fullmatch(r"C:\\Users\\jarvis\\remoteapp-compiled-stage-[0-9a-f]{16}", root):
        raise RuntimeError("desktop_stage_root_invalid")
    return root


class DesktopStage:
    def __init__(self, t, root):
        validate_stage_root(root)
        self.t, self.root, self.nonce = t, root, secrets.token_hex(16)
        t.powershell("$ErrorActionPreference='Stop'; $r=Get-Item -LiteralPath '"+root+"'; "
                     "if (-not $r.PSIsContainer -or ($r.Attributes -band [IO.FileAttributes]::ReparsePoint) -or "
                     "(Test-Path -LiteralPath '"+root+"\\run')) {throw 'staging_root_unusable'}")
        # This challenge proves a prestarted desktop script is polling now;
        # it is NOT OS-signature or identity evidence. Native probe follows.
        self.write("ping.json", {"nonce": self.nonce})
        deadline = time.monotonic() + 15
        while time.monotonic() < deadline:
            raw = t.powershell("if (Test-Path -LiteralPath '" + root + "\\ack.json') { Get-Content -Raw -LiteralPath '" + root + "\\ack.json' }")
            if raw:
                value = json.loads(raw)
                if value.get("nonce") == self.nonce:
                    checked_ack(value, self.nonce)
                    t.check(True, "desktop_waiter_live_before_server_mutation")
                    return
            time.sleep(.3)
        raise RuntimeError("desktop_stage_waiter_unavailable")

    def write(self, name, value):
        # Fixed names only, atomic public delivery. Never overwrite launch.
        if name not in ("ping.json", "launch.json"):
            raise RuntimeError("desktop_stage_filename_invalid")
        encoded = base64.b64encode(json.dumps(value).encode()).decode()
        self.t.powershell("$ErrorActionPreference='Stop'; $p='" + self.root + "\\" + name + "'; "
                          "if (Test-Path -LiteralPath $p) {throw 'staging_file_exists'}; "
                          "[IO.File]::WriteAllBytes($p+'.new',[Convert]::FromBase64String('" + encoded + "')); "
                          "Move-Item -LiteralPath ($p+'.new') -Destination $p")

    def launch(self, digest):
        if not re.fullmatch(r"[0-9a-f]{64}", digest):
            raise RuntimeError("desktop_stage_manifest_invalid")
        self.write("launch.json", {"nonce": self.nonce, "manifest_sha256": digest})
