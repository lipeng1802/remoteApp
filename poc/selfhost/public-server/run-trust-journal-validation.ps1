$ErrorActionPreference = 'Stop'
$taskFolder = $PSScriptRoot
$taskManifestPath = Join-Path $taskFolder 'SHA256.json'
$taskManifest = Get-Content -LiteralPath $taskManifestPath -Raw | ConvertFrom-Json
foreach ($taskName in @('trust-journal-win.exe', 'trust-journal-tests-win.exe')) {
    $taskPath = Join-Path $taskFolder $taskName
    if (!(Test-Path -LiteralPath $taskPath -PathType Leaf)) { throw "Missing validation binary: $taskName" }
    $taskExpected = $taskManifest.PSObject.Properties[$taskName].Value
    if ($taskExpected -notmatch '^[0-9a-f]{64}$') { throw 'Invalid checksum manifest' }
    $taskActual = (Get-FileHash -LiteralPath $taskPath -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($taskActual -ne $taskExpected) { throw "Checksum mismatch: $taskName" }
}
$taskPreviousJournal = $env:REMOTEAPP_JOURNAL_TEST_EXECUTABLE
try {
    $env:REMOTEAPP_JOURNAL_TEST_EXECUTABLE = (Resolve-Path -LiteralPath (Join-Path $taskFolder 'trust-journal-win.exe')).Path
    $taskTests = Join-Path $taskFolder 'trust-journal-tests-win.exe'
    $taskOutput = @(& $taskTests '-test.run=^TestDesktopJournalNative' '-test.v=true' '-test.timeout=120s' 2>&1)
    $taskExit = $LASTEXITCODE
    $taskOutput | ForEach-Object { Write-Output "$_" }
    $taskText = ($taskOutput | ForEach-Object { "$_" }) -join "`n"
    if ($taskExit -ne 0 -or $taskText -match '--- SKIP:') {
        throw "Native trust-journal validation NOT passed (exit=$taskExit). Run in normal signed-in desktop PowerShell, not SSH."
    }
    foreach ($taskTest in @(
        'TestDesktopJournalNativeRestartFloorAndExclusiveOwner',
        'TestDesktopJournalNativeMissingCorruptAndPartialRejectReset',
        'TestDesktopJournalNativeMaximumRecord',
        'TestDesktopJournalNativeInterruptedInitializationAndCASFailure'
    )) {
        if ($taskText -notmatch ('--- PASS: ' + [regex]::Escape($taskTest) + ' \(')) {
            throw "Missing required native acceptance: $taskTest"
        }
    }
    Write-Output 'VALIDATION PASSED: Windows native trust journal + exclusive helper + persistent epoch floor. NOT installer trust provisioning or product acceptance.'
} finally {
    $env:REMOTEAPP_JOURNAL_TEST_EXECUTABLE = $taskPreviousJournal
}
