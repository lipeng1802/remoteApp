"""Package only the explicit Windows lab allowlist; never include state/keys."""
import hashlib
import json
from pathlib import Path
import sys
import subprocess
import zipfile

FILES = (
    "poc/selfhost/verify-windows.ps1",
    "poc/selfhost/headscale.yaml",
    "poc/selfhost/policy.json",
    "poc/selfhost/WINDOWS_TEST.md",
    "artifacts/connection-poc/selfhost-node-win-x64.exe",
    "artifacts/connection-poc/lab-derp-win-x64.exe",
    "THIRD_PARTY_NOTICES.txt",
)


def main():
    root = Path(sys.argv[1]).resolve()
    artifacts = Path(__file__).resolve().parents[2] / "artifacts" / "connection-poc"
    if root.parent != artifacts or not root.name.startswith("windows-test-"):
        raise SystemExit("invalid_bundle_directory")
    go = sys.argv[2]
    module_dirs = subprocess.check_output(
        [go, "list", "-deps", "-f", "{{with .Module}}{{.Dir}}{{end}}", ".", "./cmd/lab-derp"],
        cwd=Path(__file__).resolve().parent, text=True).splitlines()
    source = Path(__file__).resolve().parent
    dirs = sorted({Path(p) for p in module_dirs if p and Path(p) != source})
    notices = ["Internal Windows PoC; not a signed product release.\n"]
    go_root = Path(subprocess.check_output([go, "env", "GOROOT"], text=True).strip())
    notices += ["Go runtime LICENSE\n", (go_root / "LICENSE").read_text()]
    for folder in dirs:
        files = sorted({p for pattern in ("LICENSE*", "COPYING*", "NOTICE*", "PATENTS*", "AUTHORS*")
                        for p in folder.glob(pattern) if p.is_file()})
        if not files:
            raise SystemExit("dependency_notice_missing: " + folder.name)
        notices.append("\nDependency: " + folder.name + "\n")
        for path in files:
            notices += [path.name + "\n", path.read_text(encoding="utf-8", errors="replace")]
    (root / "THIRD_PARTY_NOTICES.txt").write_text("\n".join(notices), encoding="utf-8")
    hashes = {name: hashlib.sha256((root / name).read_bytes()).hexdigest() for name in FILES}
    (root / "checksums.json").write_text(json.dumps(hashes, indent=2) + "\n", encoding="utf-8")
    output = root.with_suffix(".zip")
    with zipfile.ZipFile(output, "x", compression=zipfile.ZIP_DEFLATED) as archive:
        for name in (*FILES, "checksums.json"):
            archive.write(root / name, name)
    digest = hashlib.sha256(output.read_bytes()).hexdigest()
    output.with_suffix(".zip.sha256").write_text(digest + "  " + output.name + "\n", encoding="ascii")
    print("Windows bundle: " + str(output))
    print("SHA-256: " + digest)


if __name__ == "__main__":
    main()
