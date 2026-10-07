$ErrorActionPreference = 'Stop'
$taskFolder = $PSScriptRoot
$manifestFile = Join-Path $taskFolder 'SHA256.json'
$taskManifest = Get-Content -LiteralPath $manifestFile -Raw | ConvertFrom-Json
foreach ($taskName in @('identity-vault.exe', 'native-lifecycle-tests.exe')) {
    $taskPath = Join-Path $taskFolder $taskName
    if (!(Test-Path -LiteralPath $taskPath -PathType Leaf)) { throw "Missing validation binary: $taskName" }
    $taskExpected = $taskManifest.PSObject.Properties[$taskName].Value
    if ($taskExpected -notmatch '^[0-9a-f]{64}$') { throw 'Invalid checksum manifest' }
    $taskActual = (Get-FileHash -LiteralPath $taskPath -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($taskActual -ne $taskExpected) { throw "Checksum mismatch: $taskName" }
}
$taskPreviousVault = $env:REMOTEAPP_VAULT_TEST_EXECUTABLE
try {
    $env:REMOTEAPP_VAULT_TEST_EXECUTABLE = (Resolve-Path -LiteralPath (Join-Path $taskFolder 'identity-vault.exe')).Path
    $taskTest = Join-Path $taskFolder 'native-lifecycle-tests.exe'
    $taskOutput = @(& $taskTest '-test.run=^TestNativeVaultPortableLifecycle$' '-test.v=true' '-test.timeout=60s' 2>&1)
    $taskExit = $LASTEXITCODE
    $taskOutput | ForEach-Object { Write-Output "$_" }
    $taskText = ($taskOutput | ForEach-Object { "$_" }) -join "`n"
    if ($taskExit -ne 0 -or $taskText -match '--- SKIP:' -or $taskText -notmatch '--- PASS: TestNativeVaultPortableLifecycle') {
        throw "Native acceptance NOT passed (exit=$taskExit). Use a normal signed-in desktop PowerShell, not SSH."
    }
    Write-Output 'VALIDATION PASSED: native OS identity + HTTPS lifecycle + local opaque tunnel. NOT cross-machine Headscale acceptance.'
} finally {
    $env:REMOTEAPP_VAULT_TEST_EXECUTABLE = $taskPreviousVault
}
