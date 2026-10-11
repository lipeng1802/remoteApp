# Normal-desktop TEST-only native journal acceptance. Not cross-machine TLS or
# an actual OS power-loss/write-failure simulation. No product fault endpoint.
$ErrorActionPreference='Stop'
$taskManifest=Get-Content -LiteralPath (Join-Path $PSScriptRoot 'SHA256.json') -Raw | ConvertFrom-Json
foreach ($taskName in @('owner-upgrade-tests.exe','trust-journal.exe')) {
    $taskExpected=$taskManifest.PSObject.Properties[$taskName].Value
    if ($taskExpected -notmatch '^[0-9a-f]{64}$' -or (Get-FileHash -LiteralPath (Join-Path $PSScriptRoot $taskName) -Algorithm SHA256).Hash.ToLowerInvariant() -ne $taskExpected) {throw 'Use the exact new validation bundle'}
}
$taskPrevious=$env:REMOTEAPP_JOURNAL_TEST_EXECUTABLE
try {
    $env:REMOTEAPP_JOURNAL_TEST_EXECUTABLE=(Resolve-Path (Join-Path $PSScriptRoot 'trust-journal.exe')).Path
    $taskRequired=@('TestInstalledEpochRejectsBrokenOwnerAndInvalidCandidateKeepsRunner','TestNativeUpgradeUncertainReceiptLatchesClosedAndPreservesFloor','TestNativeLiveOwnerRetainsLeaseOnTimeoutAndReopensEpoch','TestOwnedProductUpgradeWaitsForProductAndNetworkJoin','TestOwnedProductNonzeroExitRejectsUpgrade')
    $taskPattern='^('+($taskRequired -join '|')+')$'
    $taskLines=@(& (Join-Path $PSScriptRoot 'owner-upgrade-tests.exe') ('-test.run='+$taskPattern) '-test.v=true' '-test.timeout=90s' 2>&1)
    $taskExit=$LASTEXITCODE
    $taskLines | ForEach-Object {Write-Output $_}
    $taskText=$taskLines -join "`n"
    if ($taskExit -ne 0 -or $taskText -match '(--- SKIP:|--- FAIL:|no tests to run)') {throw 'Upgrade/floor acceptance not passed'}
    foreach ($taskName in $taskRequired) {
        if ($taskText -notmatch ('(?m)^--- PASS: '+[regex]::Escape($taskName)+' [(]')) {throw 'Required acceptance missing'}
    }
} finally {$env:REMOTEAPP_JOURNAL_TEST_EXECUTABLE=$taskPrevious}
Write-Output 'VALIDATION PASSED: native durable upgrade + uncertain receipt locks owner + explicit idle reopen and epoch floor + actual product fixture join. NOT cross-machine TLS upgrade, real OS write failure, GUI or signed installer acceptance.'
