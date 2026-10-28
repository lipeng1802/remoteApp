$ErrorActionPreference = 'Stop'
$taskTests = Join-Path $PSScriptRoot 'maintenance-tests-win.exe'
$taskHelper = Join-Path $PSScriptRoot 'trust-journal.exe'
foreach ($taskFile in @($taskTests, $taskHelper)) {
    if (-not (Test-Path -LiteralPath $taskFile -PathType Leaf)) {
        throw 'The complete maintenance validation bundle is required.'
    }
}
$taskPreviousJournal = $env:REMOTEAPP_JOURNAL_TEST_EXECUTABLE
try {
    $env:REMOTEAPP_JOURNAL_TEST_EXECUTABLE = (Resolve-Path -LiteralPath $taskHelper).Path
    & $taskTests '-test.run=Test.*Maintenance' '-test.v=true' '-test.timeout=2m'
    if ($LASTEXITCODE -ne 0) { throw 'Native maintenance validation failed.' }
    Write-Output 'VALIDATION PASSED: native maintenance lease + durable pending/crash fence + retained epoch floor. NOT product uninstallation, full product IPC or signed installer acceptance.'
} finally {
    if ($null -eq $taskPreviousJournal) {
        Remove-Item Env:REMOTEAPP_JOURNAL_TEST_EXECUTABLE -ErrorAction SilentlyContinue
    } else {
        $env:REMOTEAPP_JOURNAL_TEST_EXECUTABLE = $taskPreviousJournal
    }
}
