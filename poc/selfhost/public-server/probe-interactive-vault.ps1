$ErrorActionPreference = 'Stop'
$taskFolder = $PSScriptRoot
$taskManifest = Get-Content -LiteralPath (Join-Path $taskFolder 'SHA256.json') -Raw | ConvertFrom-Json
foreach ($taskName in @('vault-relay.exe', 'interactive-bridge-tests.exe')) {
    $taskExpected = $taskManifest.PSObject.Properties[$taskName].Value
    if ($taskExpected -notmatch '^[0-9a-f]{64}$') { throw 'Invalid checksum manifest' }
    if ((Get-FileHash -LiteralPath (Join-Path $taskFolder $taskName) -Algorithm SHA256).Hash.ToLowerInvariant() -ne $taskExpected) { throw "Checksum mismatch: $taskName" }
}
$taskPrevious = $env:REMOTEAPP_INTERACTIVE_TEST_FOLDER
try {
    $env:REMOTEAPP_INTERACTIVE_TEST_FOLDER = (Resolve-Path -LiteralPath $taskFolder).Path
    $taskOutput = @(& (Join-Path $taskFolder 'interactive-bridge-tests.exe') '-test.run=^TestPinnedInteractiveVaultFromBootstrap$' '-test.v=true' '-test.timeout=45s' 2>&1)
    $taskExit = $LASTEXITCODE
    $taskOutput | ForEach-Object { Write-Output "$_" }
    $taskText = ($taskOutput | ForEach-Object { "$_" }) -join "`n"
    if ($taskExit -ne 0 -or $taskText -match '--- SKIP:' -or $taskText -notmatch '--- PASS: TestPinnedInteractiveVaultFromBootstrap') { throw "Pinned bridge probe NOT passed (exit=$taskExit)" }
    Write-Output 'PROBE PASSED: desktop OS signatures via same-user relay. NOT Headscale/TLS network acceptance.'
} finally { $env:REMOTEAPP_INTERACTIVE_TEST_FOLDER = $taskPrevious }
