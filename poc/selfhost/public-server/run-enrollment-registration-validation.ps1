$ErrorActionPreference = 'Stop'
$taskProcess = New-Object System.Diagnostics.Process
$taskProcess.StartInfo = New-Object System.Diagnostics.ProcessStartInfo
$taskProcess.StartInfo.FileName = Join-Path $PSScriptRoot 'enrollment-registration-validation.exe'
$taskProcess.StartInfo.UseShellExecute = $false
$taskProcess.StartInfo.RedirectStandardOutput = $true
$taskProcess.StartInfo.RedirectStandardError = $true
try {
    if (-not $taskProcess.Start()) { throw 'Native registration validation did not start' }
    $taskOutput = $taskProcess.StandardOutput.ReadToEndAsync()
    $taskError = $taskProcess.StandardError.ReadToEndAsync()
    if (-not $taskProcess.WaitForExit(180000)) { throw 'Actual exit NOT confirmed; preserve TEST evidence, do not force-close' }
    $taskText = $taskOutput.GetAwaiter().GetResult()
    $taskErrorText = $taskError.GetAwaiter().GetResult()
    Write-Output $taskText
    if ($taskProcess.ExitCode -ne 0 -or $taskErrorText.Length -ne 0 -or $taskText -notmatch 'VALIDATION PASSED: native profile/node leases' -or $taskText -match 'VALIDATION FAILED|FAIL |SKIP') { throw 'Native registration combination NOT passed; use a normal signed-in desktop PowerShell' }
} finally {
    $taskProcess.Dispose()
}
