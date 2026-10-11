param([switch]$TestProcessCapture)
# Normal-desktop TEST-only native journal acceptance. Not cross-machine TLS or
# an actual OS power-loss/write-failure simulation. No product fault endpoint.
$ErrorActionPreference='Stop'
function Invoke-ValidationProcess([string]$Executable,[string]$Arguments) {
    # PowerShell 5.1 turns native stderr redirected with 2>&1 into ErrorRecord;
    # under Stop even an ordinary JOIN_STAGE would abort the script. Keep both
    # OS pipes as text, drain concurrently, and use actual exit/required PASS.
    $taskInfo=New-Object Diagnostics.ProcessStartInfo
    $taskInfo.FileName=$Executable
    $taskInfo.Arguments=$Arguments
    $taskInfo.UseShellExecute=$false
    $taskInfo.RedirectStandardOutput=$true
    $taskInfo.RedirectStandardError=$true
    $taskProcess=New-Object Diagnostics.Process
    $taskProcess.StartInfo=$taskInfo
    $taskStarted=$false
    try {
        if (!$taskProcess.Start()) {throw 'Validation launch failed'}
        $taskStarted=$true
        $taskOutput=$taskProcess.StandardOutput.ReadToEndAsync()
        $taskError=$taskProcess.StandardError.ReadToEndAsync()
        $taskProcess.WaitForExit() # no kill or cancellation-as-join fallback
        return @{Exit=$taskProcess.ExitCode;Out=$taskOutput.Result;Err=$taskError.Result}
    } finally {
        if ($taskStarted -and !$taskProcess.HasExited) {$taskProcess.WaitForExit()}
        $taskProcess.Dispose()
    }
}
if ($TestProcessCapture) {
    # No journal/key access. Distinct diagnostic output, never native acceptance.
    foreach ($taskExpectedExit in @(0,7)) {
        # cmd emits ordinary native text, like the Go test executable; nested
        # encoded PowerShell may serialize stderr as CLIXML instead.
        $taskArguments='/d /s /c "echo JOIN_STAGE product_stopping 1>&2 & echo CAPTURE_FIXTURE_PASS & exit /b '+$taskExpectedExit+'"'
        $taskResult=Invoke-ValidationProcess (Join-Path $env:SystemRoot 'System32\cmd.exe') $taskArguments
        if ($taskResult.Exit -ne $taskExpectedExit -or $taskResult.Out.Trim() -ne 'CAPTURE_FIXTURE_PASS' -or $taskResult.Err.Trim() -ne 'JOIN_STAGE product_stopping') {throw 'Process capture regression failed'}
    }
    Write-Output 'PROCESS_CAPTURE_PASSED: stderr text and actual zero/nonzero exits preserved. NOT native journal or upgrade acceptance.'
    return
}
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
    $taskResult=Invoke-ValidationProcess (Join-Path $PSScriptRoot 'owner-upgrade-tests.exe') ('"-test.run='+$taskPattern+'" -test.v=true -test.timeout=90s')
    $taskExit=$taskResult.Exit
    $taskText=$taskResult.Out+"`n"+$taskResult.Err
    Write-Output $taskText
    if ($taskExit -ne 0 -or $taskText -match '(--- SKIP:|--- FAIL:|no tests to run)') {throw 'Upgrade/floor acceptance not passed'}
    foreach ($taskName in $taskRequired) {
        if ($taskText -notmatch ('(?m)^--- PASS: '+[regex]::Escape($taskName)+' [(]')) {throw 'Required acceptance missing'}
    }
} finally {$env:REMOTEAPP_JOURNAL_TEST_EXECUTABLE=$taskPrevious}
Write-Output 'VALIDATION PASSED: native durable upgrade + uncertain receipt locks owner + explicit idle reopen and epoch floor + actual product fixture join. NOT cross-machine TLS upgrade, real OS write failure, GUI or signed installer acceptance.'
