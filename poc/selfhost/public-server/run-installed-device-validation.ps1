$ErrorActionPreference = 'Stop'
$taskManifest = Get-Content -LiteralPath (Join-Path $PSScriptRoot 'SHA256.json') -Raw | ConvertFrom-Json
foreach ($taskName in @('identity-vault.exe','trust-journal.exe','installed-device-client.exe','installed-device-validation.exe','run-installed-device-validation.ps1')) {
    $taskPath = Join-Path $PSScriptRoot $taskName
    $taskItem = Get-Item -LiteralPath $taskPath
    $taskExpected = $taskManifest.PSObject.Properties[$taskName].Value
    if ($taskItem.PSIsContainer -or ($taskItem.Attributes -band [IO.FileAttributes]::ReparsePoint) -or $taskExpected -cnotmatch '^[0-9a-f]{64}$' -or (Get-FileHash -LiteralPath $taskPath -Algorithm SHA256).Hash.ToLowerInvariant() -cne $taskExpected) { throw 'Validation bundle integrity rejected' }
}
# Capture actual native stdout/stderr as text, never PowerShell ErrorRecords.
$taskStart = New-Object Diagnostics.ProcessStartInfo
$taskStart.FileName = Join-Path $PSScriptRoot 'installed-device-validation.exe'
$taskStart.UseShellExecute = $false
$taskStart.RedirectStandardOutput = $true
$taskStart.RedirectStandardError = $true
$taskProcess = New-Object Diagnostics.Process
$taskProcess.StartInfo = $taskStart
$taskStarted = $false
try {
    if (-not $taskProcess.Start()) { throw 'Validation process not started' }
    $taskStarted = $true
    $taskOut = $taskProcess.StandardOutput.ReadToEndAsync()
    $taskErr = $taskProcess.StandardError.ReadToEndAsync()
    $taskProcess.WaitForExit()
    $taskText = $taskOut.Result + $taskErr.Result
    Write-Output $taskText
    if ($taskProcess.ExitCode -ne 0 -or $taskText -notmatch 'VALIDATION PASSED: actual compiled device client' -or $taskText -match 'VALIDATION FAILED|FAIL |SKIP') { throw 'Installed device native acceptance NOT passed; use a normal signed-in desktop PowerShell' }
} finally {
    if ($taskStarted -and -not $taskProcess.HasExited) { $taskProcess.WaitForExit() }
    $taskProcess.Dispose()
}
