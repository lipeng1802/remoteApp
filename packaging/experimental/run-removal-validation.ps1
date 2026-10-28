$ErrorActionPreference = 'Stop'
$taskOldJournal = $env:REMOTEAPP_JOURNAL_TEST_EXECUTABLE
$taskOldVault = $env:REMOTEAPP_VAULT_TEST_EXECUTABLE
try {
    $env:REMOTEAPP_JOURNAL_TEST_EXECUTABLE = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot 'trust-journal.exe')).Path
    $env:REMOTEAPP_VAULT_TEST_EXECUTABLE = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot 'identity-vault.exe')).Path
    & (Join-Path $PSScriptRoot 'protocol-tests.exe') '-test.run=^TestIndependentMaintenance' '-test.v=true' '-test.timeout=2m'
    if ($LASTEXITCODE -ne 0) { throw 'Independent maintenance authentication failed' }
    & (Join-Path $PSScriptRoot 'vault-domain-tests.exe') '-test.run=Test.*Domains' '-test.v=true' '-test.timeout=2m'
    if ($LASTEXITCODE -ne 0) { throw 'Native signer domain separation failed' }
    & (Join-Path $PSScriptRoot 'trust-tests.exe') '-test.run=^TestRemovalTransaction' '-test.v=true' '-test.timeout=2m'
    if ($LASTEXITCODE -ne 0) { throw 'Native transaction schema validation failed' }
    & (Join-Path $PSScriptRoot 'file-tests.exe') '-test.run=^TestRemoval(Plan|Handles)' '-test.v=true' '-test.timeout=2m'
    if ($LASTEXITCODE -ne 0) { throw 'Signed inventory and object validation failed' }
    & (Join-Path $PSScriptRoot 'removal-tests.exe') '-test.run=^TestNativeRemoval' '-test.v=true' '-test.timeout=2m'
    if ($LASTEXITCODE -ne 0) { throw 'Native removal and crash recovery failed' }
    Write-Output 'VALIDATION PASSED: TEST independent process OS-device-key mutual proof + replay/reflection/selector rejection + actual local TLS join + exact native removal + interrupted finish retains lease + identity/epoch floor. NOT publisher identity, production consent/discovery, full product exit, cross-machine session or signed installer acceptance.'
} finally {
    if ($null -eq $taskOldJournal) { Remove-Item Env:REMOTEAPP_JOURNAL_TEST_EXECUTABLE -ErrorAction SilentlyContinue } else { $env:REMOTEAPP_JOURNAL_TEST_EXECUTABLE = $taskOldJournal }
    if ($null -eq $taskOldVault) { Remove-Item Env:REMOTEAPP_VAULT_TEST_EXECUTABLE -ErrorAction SilentlyContinue } else { $env:REMOTEAPP_VAULT_TEST_EXECUTABLE = $taskOldVault }
}
