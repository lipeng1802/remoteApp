$ErrorActionPreference = 'Stop'
$taskFolder = $PSScriptRoot
$taskManifest = Get-Content -LiteralPath (Join-Path $taskFolder 'SHA256.json') -Raw | ConvertFrom-Json
foreach ($taskName in @('clientowner-tests.exe', 'streamgate-tests.exe', 'session-tests.exe', 'trust-journal.exe')) {
    $taskExpected = $taskManifest.PSObject.Properties[$taskName].Value
    if ($taskExpected -notmatch '^[0-9a-f]{64}$' -or (Get-FileHash -LiteralPath (Join-Path $taskFolder $taskName) -Algorithm SHA256).Hash.ToLowerInvariant() -ne $taskExpected) {
        throw "Invalid validation checksum: $taskName"
    }
}
$taskPrior = [Environment]::GetEnvironmentVariable('REMOTEAPP_JOURNAL_TEST_EXECUTABLE', 'Process')
try {
    $env:REMOTEAPP_JOURNAL_TEST_EXECUTABLE = (Resolve-Path -LiteralPath (Join-Path $taskFolder 'trust-journal.exe')).Path
    $taskExpectedTests = @('TestUpgradeJoinsBeforeCommitAndNeedsExplicitRestart', 'TestTimeoutLatchesClosedAndRetainsNativeLeaseUntilJoin', 'TestRunnerFaultAndCommitFailureRejectReuse', 'TestGroupSealsBeforeJoining', 'TestNativeLiveOwnerRetainsLeaseOnTimeoutAndReopensEpoch')
    $taskPattern = '^(' + ($taskExpectedTests -join '|') + ')$'
    $taskOutput = @(& (Join-Path $taskFolder 'clientowner-tests.exe') "-test.run=$taskPattern" '-test.v=true' '-test.timeout=90s' 2>&1)
    $taskExit = $LASTEXITCODE
    $taskOutput | ForEach-Object { Write-Output "$_" }
    $taskText = ($taskOutput | ForEach-Object { "$_" }) -join "`n"
    if ($taskExit -ne 0 -or $taskText -match '--- SKIP:|--- FAIL:|panic:') { throw 'Native live owner acceptance NOT passed' }
    foreach ($taskTest in $taskExpectedTests) {
        if ($taskText -notmatch ('(?m)^--- PASS: ' + [regex]::Escape($taskTest) + ' \(')) { throw "Required test missing: $taskTest" }
    }
    $taskTest = 'TestInstalledOwnerJoinsRealHTTPSWatchAndOpaqueTunnelBeforeUpgrade'
    $taskOutput = @(& (Join-Path $taskFolder 'streamgate-tests.exe') "-test.run=^$taskTest`$" '-test.v=true' '-test.timeout=90s' 2>&1)
    $taskExit = $LASTEXITCODE
    $taskOutput | ForEach-Object { Write-Output "$_" }
    $taskText = ($taskOutput | ForEach-Object { "$_" }) -join "`n"
    if ($taskExit -ne 0 -or $taskText -match '--- SKIP:|--- FAIL:|panic:' -or $taskText -notmatch ('(?m)^--- PASS: ' + [regex]::Escape($taskTest) + ' \(')) { throw 'HTTPS watch and opaque tunnel join acceptance NOT passed' }
    $taskExpectedTests = @('TestAuthorityCheckDoesNotDetachAfterDeadline', 'TestTunnelOpaqueBytesAndAuthorityClosure', 'TestTunnelDenialNeverOpensProductUpstream', 'TestFixedPayloadProofCannotEnterProductTunnel')
    $taskPattern = '^(' + ($taskExpectedTests -join '|') + ')$'
    $taskOutput = @(& (Join-Path $taskFolder 'session-tests.exe') "-test.run=$taskPattern" '-test.v=true' '-test.timeout=90s' 2>&1)
    $taskExit = $LASTEXITCODE
    $taskOutput | ForEach-Object { Write-Output "$_" }
    $taskText = ($taskOutput | ForEach-Object { "$_" }) -join "`n"
    if ($taskExit -ne 0 -or $taskText -match '--- SKIP:|--- FAIL:|panic:') { throw 'Joined authorization callback and tunnel regression NOT passed' }
    foreach ($taskTest in $taskExpectedTests) {
        if ($taskText -notmatch ('(?m)^--- PASS: ' + [regex]::Escape($taskTest) + ' \(')) { throw "Required test missing: $taskTest" }
    }
} finally {
    [Environment]::SetEnvironmentVariable('REMOTEAPP_JOURNAL_TEST_EXECUTABLE', $taskPrior, 'Process')
}
Write-Output 'VALIDATION PASSED: native live owner lease + HTTPS watch/tunnel cancel and join. NOT cross-machine Headscale, product TLS/GUI or signed installer acceptance.'
