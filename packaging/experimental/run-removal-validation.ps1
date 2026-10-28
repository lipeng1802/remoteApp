$ErrorActionPreference = 'Stop'
$taskOldJournal = $env:REMOTEAPP_JOURNAL_TEST_EXECUTABLE
$taskOldVault = $env:REMOTEAPP_VAULT_TEST_EXECUTABLE
try {
    $env:REMOTEAPP_JOURNAL_TEST_EXECUTABLE = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot 'trust-journal.exe')).Path
    $env:REMOTEAPP_VAULT_TEST_EXECUTABLE = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot 'identity-vault.exe')).Path
    & (Join-Path $PSScriptRoot 'trust-tests.exe') '-test.run=^TestRemovalTransaction' '-test.v=true' '-test.timeout=2m'
    if ($LASTEXITCODE -ne 0) { throw 'Native transaction schema validation failed' }
    & (Join-Path $PSScriptRoot 'file-tests.exe') '-test.run=^TestRemovalPlan' '-test.v=true' '-test.timeout=2m'
    if ($LASTEXITCODE -ne 0) { throw 'Signed inventory and object validation failed' }
    & (Join-Path $PSScriptRoot 'removal-tests.exe') '-test.run=^TestNativeRemoval' '-test.v=true' '-test.timeout=2m'
    if ($LASTEXITCODE -ne 0) { throw 'Native removal and crash recovery failed' }
    Write-Output 'VALIDATION PASSED: TEST signed program inventory + native digest-bound removal + real partial-delete crash/resume + retained OS identity/activation/epoch floor. NOT production uninstall, power-loss durability, signed installer or live-session acceptance.'
} finally {
    if ($null -eq $taskOldJournal) { Remove-Item Env:REMOTEAPP_JOURNAL_TEST_EXECUTABLE -ErrorAction SilentlyContinue } else { $env:REMOTEAPP_JOURNAL_TEST_EXECUTABLE = $taskOldJournal }
    if ($null -eq $taskOldVault) { Remove-Item Env:REMOTEAPP_VAULT_TEST_EXECUTABLE -ErrorAction SilentlyContinue } else { $env:REMOTEAPP_VAULT_TEST_EXECUTABLE = $taskOldVault }
}
