$ErrorActionPreference = 'Stop'
$taskFolder = $PSScriptRoot
$taskManifest = Get-Content -LiteralPath (Join-Path $taskFolder 'SHA256.json') -Raw | ConvertFrom-Json
foreach ($taskName in @('clientowner-tests.exe', 'trust-journal.exe', 'run-fault-replacement-validation.ps1')) {
    $taskExpected = $taskManifest.PSObject.Properties[$taskName].Value
    if ($taskExpected -isnot [string] -or $taskExpected -cnotmatch '^[0-9a-f]{64}$' -or
        (Get-FileHash -LiteralPath (Join-Path $taskFolder $taskName) -Algorithm SHA256).Hash.ToLowerInvariant() -cne $taskExpected) {
        throw 'Invalid replacement validation checksum'
    }
}
$taskPrior = [Environment]::GetEnvironmentVariable('REMOTEAPP_JOURNAL_TEST_EXECUTABLE', 'Process')
try {
    $env:REMOTEAPP_JOURNAL_TEST_EXECUTABLE = (Resolve-Path -LiteralPath (Join-Path $taskFolder 'trust-journal.exe')).Path
    $taskTests = @('TestReplacementRequiresSuccessfulFaultRetirement', 'TestReplacementIsDifferentIdleOwnerAndOldOwnerRemainsClosed', 'TestReplacementRejectsMissingCorruptStateAndFailedFactory', 'TestReplacementRejectsCanceledRequestAndExpiredInstallation', 'TestNativeFaultReplacementPreservesFloorAndExclusiveLease')
    $taskPattern = '^(' + ($taskTests -join '|') + ')$'
    $taskOutput = @(& (Join-Path $taskFolder 'clientowner-tests.exe') "-test.run=$taskPattern" '-test.v=true' '-test.timeout=90s' 2>&1)
    $taskExit = $LASTEXITCODE
    $taskOutput | ForEach-Object { Write-Output "$_" }
    $taskText = ($taskOutput | ForEach-Object { "$_" }) -join "`n"
    if ($taskExit -ne 0 -or $taskText -match '--- SKIP:|--- FAIL:|panic:') { throw 'Native replacement validation NOT passed' }
    foreach ($taskTest in $taskTests) {
        if ($taskText -notmatch ('(?m)^--- PASS: ' + [regex]::Escape($taskTest) + ' \(')) { throw 'Required replacement test missing' }
    }
} finally {
    [Environment]::SetEnvironmentVariable('REMOTEAPP_JOURNAL_TEST_EXECUTABLE', $taskPrior, 'Process')
}
Write-Output 'VALIDATION PASSED: explicit fresh owner after fault retirement + native lease and epoch floor. NOT cross-machine reconnect, fresh authorization, GUI or signed installer acceptance.'
