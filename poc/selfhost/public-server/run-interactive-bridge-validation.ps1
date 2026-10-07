$ErrorActionPreference = 'Stop'
$taskFolder = $PSScriptRoot
$taskManifest = Get-Content -LiteralPath (Join-Path $taskFolder 'SHA256.json') -Raw | ConvertFrom-Json
foreach ($taskName in @('identity-vault.exe', 'interactive-vault.exe', 'vault-relay.exe', 'interactive-bridge-tests.exe')) {
    $taskPath = Join-Path $taskFolder $taskName
    $taskExpected = $taskManifest.PSObject.Properties[$taskName].Value
    if ($taskExpected -notmatch '^[0-9a-f]{64}$' -or !(Test-Path -LiteralPath $taskPath -PathType Leaf)) { throw 'Invalid validation bundle' }
    if ((Get-FileHash -LiteralPath $taskPath -Algorithm SHA256).Hash.ToLowerInvariant() -ne $taskExpected) { throw "Checksum mismatch: $taskName" }
}
$taskPrevious = $env:REMOTEAPP_INTERACTIVE_TEST_FOLDER
try {
    $env:REMOTEAPP_INTERACTIVE_TEST_FOLDER = (Resolve-Path -LiteralPath $taskFolder).Path
    $taskOutput = @(& (Join-Path $taskFolder 'interactive-bridge-tests.exe') '-test.run=^TestInteractiveVaultBridge$' '-test.v=true' '-test.timeout=60s' 2>&1)
    $taskExit = $LASTEXITCODE
    $taskOutput | ForEach-Object { Write-Output "$_" }
    $taskText = ($taskOutput | ForEach-Object { "$_" }) -join "`n"
    if ($taskExit -ne 0 -or $taskText -match '--- SKIP:' -or $taskText -notmatch '--- PASS: TestInteractiveVaultBridge') { throw "Bridge NOT passed (exit=$taskExit). Run in normal desktop PowerShell, not SSH." }
    Write-Output 'VALIDATION PASSED: interactive OS vault bridge. NOT cross-machine Headscale acceptance.'
} finally {
    $env:REMOTEAPP_INTERACTIVE_TEST_FOLDER = $taskPrevious
}
