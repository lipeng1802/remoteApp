$ErrorActionPreference = 'Stop'
$taskProcess = New-Object System.Diagnostics.Process
$taskProcess.StartInfo = New-Object System.Diagnostics.ProcessStartInfo
$taskProcess.StartInfo.FileName = Join-Path $PSScriptRoot 'enrollment-sdk-validation.exe'
$taskProcess.StartInfo.UseShellExecute = $false
$taskProcess.StartInfo.RedirectStandardOutput = $true
$taskProcess.StartInfo.RedirectStandardError = $true
try {
    if (-not $taskProcess.Start()) { throw 'SDK validation did not start' }
    $taskOutput = $taskProcess.StandardOutput.ReadToEndAsync()
    $taskError = $taskProcess.StandardError.ReadToEndAsync()
    if (-not $taskProcess.WaitForExit(120000)) { throw 'SDK actual exit NOT confirmed; do not force-close or delete TEST evidence' }
    $taskText = $taskOutput.GetAwaiter().GetResult()
    $taskErrorText = $taskError.GetAwaiter().GetResult()
    Write-Output $taskText
    if ($taskProcess.ExitCode -ne 0 -or $taskErrorText.Length -ne 0 -or $taskText -notmatch '(?m)^VALIDATION PASSED: native TEST lease \+ actual SDK Up failure/cancellation' -or $taskText -match 'VALIDATION FAILED|FAIL |SKIP') { throw 'SDK validation NOT passed; preserve TEST evidence' }
} finally {
    $taskProcess.Dispose()
}
