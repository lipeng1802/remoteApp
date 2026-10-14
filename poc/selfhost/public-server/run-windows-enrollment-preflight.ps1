$ErrorActionPreference = 'Stop'
# Normal signed-in desktop only. No SSH, administrator or old installer required.
# Manifest detects transfer damage; it is NOT a signed installer trust anchor.
$taskExpected = @(
    'storage/enrollment-storage-tests.exe', 'storage/enrollment-storage-validation.exe',
    'storage/run-enrollment-storage-validation.ps1', 'storage/SHA256.json',
    'registration/enrollment-registration-validation.exe', 'registration/trust-journal.exe',
    'registration/identity-vault.exe', 'registration/run-enrollment-registration-validation.ps1',
    'registration/SHA256.json', 'enrollment-sdk-validation.exe', 'run-windows-enrollment-preflight.ps1'
)
if ((Get-Item -LiteralPath $PSScriptRoot).Attributes -band [IO.FileAttributes]::ReparsePoint) { throw 'Bundle root alias rejected' }
$taskManifest = Get-Content -Raw -LiteralPath (Join-Path $PSScriptRoot 'SHA256.json') | ConvertFrom-Json
$taskNames = @($taskManifest.PSObject.Properties.Name)
if (@(Compare-Object $taskExpected $taskNames -CaseSensitive).Count -ne 0) { throw 'Bundle manifest set mismatch' }
$taskItems = @(Get-ChildItem -LiteralPath $PSScriptRoot -Recurse -Force)
foreach ($taskItem in $taskItems) {
    if ($taskItem.Attributes -band [IO.FileAttributes]::ReparsePoint) { throw 'Bundle link rejected' }
}
$taskActual = @($taskItems | Where-Object { -not $_.PSIsContainer } | ForEach-Object {
    $_.FullName.Substring($PSScriptRoot.Length + 1).Replace('\', '/')
})
if (@(Compare-Object ($taskExpected + @('SHA256.json')) $taskActual -CaseSensitive).Count -ne 0) { throw 'Bundle extra or missing file rejected' }
foreach ($taskName in $taskExpected) {
    $taskHash = $taskManifest.PSObject.Properties[$taskName].Value
    if ($taskHash -cnotmatch '^[0-9a-f]{64}$' -or (Get-FileHash -Algorithm SHA256 -LiteralPath (Join-Path $PSScriptRoot $taskName)).Hash.ToLowerInvariant() -cne $taskHash) { throw 'Bundle checksum rejected' }
}
function Invoke-Preflight([string]$Name, [string]$Arguments, [string]$Expected) {
    $taskProcess = New-Object System.Diagnostics.Process
    $taskProcess.StartInfo = New-Object System.Diagnostics.ProcessStartInfo
    $taskProcess.StartInfo.FileName = Join-Path $PSScriptRoot $Name
    $taskProcess.StartInfo.Arguments = $Arguments
    $taskProcess.StartInfo.UseShellExecute = $false
    $taskProcess.StartInfo.RedirectStandardOutput = $true
    $taskProcess.StartInfo.RedirectStandardError = $true
    try {
        if (-not $taskProcess.Start()) { throw 'Preflight did not start' }
        $taskOutput = $taskProcess.StandardOutput.ReadToEndAsync()
        $taskError = $taskProcess.StandardError.ReadToEndAsync()
        if (-not $taskProcess.WaitForExit(180000)) { throw 'Actual exit NOT confirmed; preserve evidence and do not force-close' }
        $taskText = $taskOutput.GetAwaiter().GetResult()
        $taskErrorText = $taskError.GetAwaiter().GetResult()
        Write-Output $taskText
        if ($taskProcess.ExitCode -ne 0 -or $taskErrorText.Length -ne 0 -or $taskText -notmatch $Expected -or $taskText -match 'VALIDATION FAILED|FAIL |SKIP') { throw 'Preflight NOT passed; preserve evidence; use normal desktop PowerShell' }
    } finally { $taskProcess.Dispose() }
}
Invoke-Preflight 'storage/enrollment-storage-tests.exe' '-test.v=true' '(?m)^PASS\r?$'
Invoke-Preflight 'storage/enrollment-storage-validation.exe' '' '(?m)^VALIDATION PASSED: native enrollment journal'
Invoke-Preflight 'registration/enrollment-registration-validation.exe' '' '(?m)^VALIDATION PASSED: native profile/node leases'
Invoke-Preflight 'enrollment-sdk-validation.exe' '' '(?m)^VALIDATION PASSED: native TEST lease \+ actual SDK Up failure/cancellation'
Write-Output 'PREFLIGHT PASSED: Windows native storage + OS signed local HTTPS + actual SDK cancel/join. NOT real Headscale enrollment, verifier bind, public service, GUI or installer acceptance.'
