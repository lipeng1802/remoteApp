"""Windows native identity signer tests; exact temporary files, no key output."""
import hashlib
import base64
import importlib.util
from pathlib import Path
import secrets
import subprocess
import time

p=Path(__file__).with_name("cross-network.py")
s=importlib.util.spec_from_file_location("network",p)
t=importlib.util.module_from_spec(s);s.loader.exec_module(t)
root="C:\\Users\\jarvis\\remoteapp-vault-validation-"+time.strftime("%Y%m%d%H%M%S")+secrets.token_hex(4)
created=False
try:
    t.powershell("$ErrorActionPreference='Stop'; "+f"if(Test-Path '{root}'){{throw 'exists'}}; New-Item -ItemType Directory '{root}' | Out-Null; "+"$sid=[System.Security.Principal.WindowsIdentity]::GetCurrent().User.Value; "+f"icacls '{root}' /inheritance:r /grant:r \"*${{sid}}:(OI)(CI)F\" '*S-1-5-18:(OI)(CI)F' | Out-Null")
    created=True
    for name in ("identity-vault-win.exe","privatepipe-tests-win.exe"):
        source=t.ARTIFACTS/name
        uploaded=subprocess.run(["scp","-O","-o","BatchMode=yes","-o","IdentitiesOnly=yes","-o","StrictHostKeyChecking=yes","-i",str(Path.home()/".ssh/remoteapp_win_ed25519"),str(source),"jarvis@100.73.4.118:"+root.replace("\\","/")+"/"+name],capture_output=True,timeout=60)
        t.check(uploaded.returncode==0,"vault_test_binary_uploaded")
        digest=t.powershell(f"(Get-FileHash -Algorithm SHA256 '{root}\\{name}').Hash")
        t.check(digest.lower()==hashlib.sha256(source.read_bytes()).hexdigest(),"vault_test_binary_hash_verified")
    command=f"$env:REMOTEAPP_VAULT_TEST_EXECUTABLE='{root}\\identity-vault-win.exe'; & '{root}\\privatepipe-tests-win.exe' '-test.run=TestNativeVaultIdentityAndIsolation' '-test.v=true' 2>&1; $nativeExit=$LASTEXITCODE; 'NATIVE_EXIT_'+$nativeExit; if($nativeExit -eq 0){{'NATIVE_RESULT_PASS'}}else{{'NATIVE_RESULT_FAILED'}}; exit 0"
    encoded=base64.b64encode(command.encode("utf-16le")).decode()
    run=subprocess.run(t.WIN+["powershell.exe -NoProfile -NonInteractive -EncodedCommand "+encoded],capture_output=True,timeout=60)
    if run.returncode:
        print("FAIL windows_vault_management_exit_"+str(run.returncode),flush=True)
    result=run.stdout.decode(errors="replace")
    if "interactive_logon_required" in result:
        print("FAIL windows_credential_store_has_no_interactive_logon_session",flush=True)
    for stage in ("native vault creation unavailable","native signing","native reopen","reopened signature","missing identity regenerated","wrong public identity","test identity cleanup failed"):
        if stage in result:
            print("FAIL windows_vault_stage_"+stage.replace(" ","_"),flush=True)
    for category in ("CommandNotFoundException","ApplicationFailedException","Win32Exception","panic:","Access is denied","PermissionDenied","fatal error:"):
        if category in result:
            print("FAIL windows_vault_launch_"+category.replace(" ","_").replace(":",""),flush=True)
    t.check("NATIVE_RESULT_PASS" in result,"windows_native_identity_sign_reopen_isolation_passed")
except Exception as error:
    print("FAIL windows_vault_runner_"+type(error).__name__,flush=True)
    if "interactive_logon_required" in str(error):
        print("FAIL windows_credential_store_has_no_interactive_logon_session",flush=True)
    print("FAIL windows_native_vault_validation_requires_diagnosis_or_interactive_logon",flush=True)
    raise SystemExit(1)
finally:
    if created:
        t.powershell(f"Remove-Item -LiteralPath '{root}' -Recurse -Force")
        print("PASS exact_temporary_vault_test_files_removed",flush=True)
