$ErrorActionPreference = 'Stop'
$taskFolder = $PSScriptRoot
$taskManifest = Get-Content -LiteralPath (Join-Path $taskFolder 'SHA256.json') -Raw | ConvertFrom-Json
$taskName = 'trust-install-validation.exe'
$taskPath = Join-Path $taskFolder $taskName
$taskExpected = $taskManifest.PSObject.Properties[$taskName].Value
if ($taskExpected -notmatch '^[0-9a-f]{64}$') { throw 'Invalid validation checksum' }
if ((Get-FileHash -LiteralPath $taskPath -Algorithm SHA256).Hash.ToLowerInvariant() -ne $taskExpected) {
    throw 'Validation executable checksum mismatch'
}
$taskOutput = @(& $taskPath 2>&1)
$taskExit = $LASTEXITCODE
$taskOutput | ForEach-Object { Write-Output "$_" }
$taskText = ($taskOutput | ForEach-Object { "$_" }) -join "`n"
if ($taskExit -ne 0 -or $taskText -notmatch 'VALIDATION PASSED: compiled anchor' -or $taskText -match 'VALIDATION FAILED|FAIL exact') {
    throw "Compiled trust installation acceptance NOT passed (exit=$taskExit). Use a normal signed-in desktop PowerShell, not SSH."
}
