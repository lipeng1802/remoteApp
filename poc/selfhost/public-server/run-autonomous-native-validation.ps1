$ErrorActionPreference = 'Stop'
# One-shot coordinated REAL server acceptance, not an offline/preflight test.
# Wait for the coordinator's fresh private HTTPS forwarding and maintenance READY.
$taskExpected = @('autonomous-native-validation.exe', 'identity-vault.exe', 'trust-journal.exe', 'public-endpoint.json', 'run-autonomous-native-validation.ps1')
$taskManifest = Get-Content -Raw -LiteralPath (Join-Path $PSScriptRoot 'SHA256.json') | ConvertFrom-Json
if (@(Compare-Object $taskExpected @($taskManifest.PSObject.Properties.Name) -CaseSensitive).Count -ne 0) { throw 'Manifest set mismatch' }
$taskItems = @(Get-ChildItem -LiteralPath $PSScriptRoot -Force)
if ((Get-Item -LiteralPath $PSScriptRoot).Attributes -band [IO.FileAttributes]::ReparsePoint) { throw 'Bundle root alias rejected' }
foreach ($taskItem in $taskItems) {
    if ($taskItem.PSIsContainer -or ($taskItem.Attributes -band [IO.FileAttributes]::ReparsePoint)) { throw 'Bundle directory or link rejected' }
}
if (@(Compare-Object ($taskExpected + @('SHA256.json')) @($taskItems.Name) -CaseSensitive).Count -ne 0) { throw 'Fresh one-shot bundle required; retain previous recovery evidence' }
foreach ($taskName in $taskExpected) {
    $taskHash = $taskManifest.PSObject.Properties[$taskName].Value
    if ($taskHash -cnotmatch '^[0-9a-f]{64}$' -or (Get-FileHash -Algorithm SHA256 -LiteralPath (Join-Path $PSScriptRoot $taskName)).Hash.ToLowerInvariant() -cne $taskHash) { throw 'Bundle checksum rejected' }
}
$taskProcess = New-Object System.Diagnostics.Process
$taskProcess.StartInfo = New-Object System.Diagnostics.ProcessStartInfo
$taskProcess.StartInfo.FileName = Join-Path $PSScriptRoot 'autonomous-native-validation.exe'
$taskProcess.StartInfo.UseShellExecute = $false
$taskProcess.StartInfo.RedirectStandardOutput = $true
$taskProcess.StartInfo.RedirectStandardError = $true
try {
    if (-not $taskProcess.Start()) { throw 'Native acceptance did not start' }
    $taskOutput = $taskProcess.StandardOutput.ReadToEndAsync()
    $taskError = $taskProcess.StandardError.ReadToEndAsync()
    if (-not $taskProcess.WaitForExit(240000)) { throw 'Actual owner exit NOT confirmed; preserve evidence, do not force-close or retire server resources' }
    $taskText = $taskOutput.GetAwaiter().GetResult()
    $taskErrorText = $taskError.GetAwaiter().GetResult()
    Write-Output $taskText
    if ($taskProcess.ExitCode -ne 0 -or $taskErrorText.Length -ne 0 -or $taskText -notmatch '(?m)^VALIDATION PASSED: actual OS signed HTTPS approval/enroll' -or $taskText -match 'VALIDATION FAILED|FAIL |SKIP') { throw 'Real native acceptance NOT passed; retain scoped evidence' }
} finally { $taskProcess.Dispose() }
