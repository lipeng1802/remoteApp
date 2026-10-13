$ErrorActionPreference = 'Stop'
function Invoke-NativeTest([string]$Name, [string]$Arguments, [string]$Expected) {
$taskProcess = New-Object System.Diagnostics.Process
$taskProcess.StartInfo = New-Object System.Diagnostics.ProcessStartInfo
$taskProcess.StartInfo.FileName = Join-Path $PSScriptRoot $Name
$taskProcess.StartInfo.Arguments = $Arguments
$taskProcess.StartInfo.UseShellExecute = $false
$taskProcess.StartInfo.RedirectStandardOutput = $true
$taskProcess.StartInfo.RedirectStandardError = $true
try {
    if (-not $taskProcess.Start()) { throw 'Native enrollment validation did not start' }
    $taskOutput = $taskProcess.StandardOutput.ReadToEndAsync()
    $taskError = $taskProcess.StandardError.ReadToEndAsync()
    if (-not $taskProcess.WaitForExit(120000)) { throw 'Native enrollment validation exit NOT confirmed; do not force-close or delete its test evidence' }
    $taskText = $taskOutput.GetAwaiter().GetResult()
    $taskErrorText = $taskError.GetAwaiter().GetResult()
    Write-Output $taskText
    if ($taskProcess.ExitCode -ne 0 -or $taskErrorText.Length -ne 0 -or $taskText -notmatch $Expected -or $taskText -match 'VALIDATION FAILED|FAIL |SKIP') { throw 'Native enrollment storage NOT passed; use a normal signed-in desktop PowerShell' }
} finally {
    $taskProcess.Dispose()
}
}
Invoke-NativeTest 'enrollment-storage-tests.exe' '-test.v=true' '(?m)^PASS\r?$'
Invoke-NativeTest 'enrollment-storage-validation.exe' '' 'VALIDATION PASSED: native enrollment journal'
