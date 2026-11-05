"""Fixed isolated-service bootstrap and user observation for a desktop TEST."""
import hashlib
import json
from pathlib import Path
import re
import select
import sys
import time

from supervised_requests import reply


SCOPE = "desktop-control"
SERVICE = "remoteapp-desktop-test"
ENDPOINT = "https://mk.fengmap.com:8444"
CONTROL = "https://mk.fengmap.com:19443"
STAGE_PREFIX = "/var/lib/remoteapp-desktop-test/remoteapp-desktop-test-"
GUI_INPUTS = {
    "darwin": ("OwnedDesktopController", 991232, "4cafc19adb1bfafdce26dae412f88e3a6d2cc1807fcd332123fc69f32ce1fdcf"),
    "windows": ("OwnedDesktopAgent.exe", 71663611, "4b7ce0f88fbd68cb72e260f995b638c67f008cd84927188289a1451a3cc75f9f"),
}


def stage_id(stage):
    if type(stage) is not str or not re.fullmatch(re.escape(STAGE_PREFIX) + r"[0-9a-f]{16}", stage):
        raise RuntimeError("desktop_stage_rejected")
    return stage[len(STAGE_PREFIX):]


def valid_root(root):
    return type(root) is str and re.fullmatch(r"[0-9a-f]{64}", root) is not None


def ready_profile(raw, root, now):
    ready = reply(raw)
    if not valid_root(root) or set(ready) != {"port", "profile_root", "profile"} or ready["profile_root"] != root:
        raise RuntimeError("desktop_authority_bootstrap_rejected")
    port, profile = ready["port"], ready["profile"]
    if type(port) is not int or not 1024 <= port <= 65535 or port in (8444, 19443):
        raise RuntimeError("desktop_authority_port_rejected")
    if type(profile) is not dict or set(profile) != {"claims", "signature"}:
        raise RuntimeError("desktop_source_profile_rejected")
    claims = profile["claims"]
    if (type(claims) is not dict or claims.get("service") != SERVICE or claims.get("endpoint") != ENDPOINT
            or type(claims.get("expires_at")) is not int or claims["expires_at"] - now < 1020):
        raise RuntimeError("desktop_source_profile_rejected")
    # Structural admission only. The compiled builder verifies the complete
    # signature and TLS/issuer claims against the independently supplied root.
    return port, json.dumps(profile, separators=(",", ":")).encode()


def gui_inputs(directory):
    products = {}
    for platform, (name, size, digest) in GUI_INPUTS.items():
        path = Path(directory) / name
        if (path.is_symlink() or not path.is_file() or path.stat().st_size != size
                or hashlib.sha256(path.read_bytes()).hexdigest() != digest):
            raise RuntimeError("desktop_gui_input_pin_rejected")
        products[platform] = path
    return products


def endpoint_arguments(port, authority_port):
    return [
        ("server", ["-L", f"127.0.0.1:{port}:127.0.0.1:{authority_port}"]),
        ("windows", ["-R", f"127.0.0.1:{port}:127.0.0.1:{port}"]),
    ]


def acknowledge(message, expires_at):
    if not sys.stdin.isatty():
        raise RuntimeError("desktop_interactive_operator_required")
    print(message, flush=True)
    seconds = expires_at - time.time()
    if seconds <= 0 or not select.select([sys.stdin], [], [], seconds)[0]:
        raise RuntimeError("desktop_operator_deadline")
    if sys.stdin.readline().strip() != "yes" or time.time() >= expires_at:
        raise RuntimeError("desktop_operator_confirmation_missing")
    return True


def observe_gui(expires_at):
    from supervised_desktop import GUI_EVIDENCE
    acknowledge("确认 Mac 已显示真实 Windows 画面，且 Windows 实际响应了鼠标和键盘操作后，输入 yes。", expires_at)
    return dict.fromkeys(GUI_EVIDENCE, True)


def request_stop(expires_at):
    return acknowledge("在 Mac 控制器窗口发起停止后，输入 yes。", expires_at)
