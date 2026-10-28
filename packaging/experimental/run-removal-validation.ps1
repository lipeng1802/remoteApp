$ErrorActionPreference = 'Stop'
$taskOldJournal = $env:REMOTEAPP_JOURNAL_TEST_EXECUTABLE
$taskOldVault = $env:REMOTEAPP_VAULT_TEST_EXECUTABLE
$taskOldProductFixture = $env:REMOTEAPP_PRODUCT_MAINT_FIXTURE
try {
    $env:REMOTEAPP_JOURNAL_TEST_EXECUTABLE = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot 'trust-journal.exe')).Path
    $env:REMOTEAPP_VAULT_TEST_EXECUTABLE = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot 'identity-vault.exe')).Path
    $env:REMOTEAPP_PRODUCT_MAINT_FIXTURE = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot 'fixture\fixture.json')).Path
    & (Join-Path $PSScriptRoot 'child-tests.exe') '-test.run=^TestMaintenanceFinish' '-test.v=true' '-test.timeout=2m'
    if ($LASTEXITCODE -ne 0) { throw 'Actual process-exit finish validation failed' }
    & (Join-Path $PSScriptRoot 'supervisor-tests.exe') '-test.run=^TestProduct(Close|Release|Forward)' '-test.v=true' '-test.timeout=2m'
    if ($LASTEXITCODE -ne 0) { throw 'Supervisor exit/retained-ownership validation failed' }
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
    Write-Output 'VALIDATION PASSED: TEST actual compiled product zero exit/EOF/reader join + native binding identity + independent authenticated removal + interrupted finish retains live product + failure exit/timeout rejection + identity/epoch floor. NOT active Headscale session, publisher identity, production discovery/consent or signed installer acceptance.'
} finally {
    if ($null -eq $taskOldJournal) { Remove-Item Env:REMOTEAPP_JOURNAL_TEST_EXECUTABLE -ErrorAction SilentlyContinue } else { $env:REMOTEAPP_JOURNAL_TEST_EXECUTABLE = $taskOldJournal }
    if ($null -eq $taskOldVault) { Remove-Item Env:REMOTEAPP_VAULT_TEST_EXECUTABLE -ErrorAction SilentlyContinue } else { $env:REMOTEAPP_VAULT_TEST_EXECUTABLE = $taskOldVault }
    if ($null -eq $taskOldProductFixture) { Remove-Item Env:REMOTEAPP_PRODUCT_MAINT_FIXTURE -ErrorAction SilentlyContinue } else { $env:REMOTEAPP_PRODUCT_MAINT_FIXTURE = $taskOldProductFixture }
}
