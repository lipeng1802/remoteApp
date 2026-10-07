$ErrorActionPreference = 'Stop'
$taskFolder = $PSScriptRoot
$taskManifest = Get-Content -LiteralPath (Join-Path $taskFolder 'SHA256.json') -Raw | ConvertFrom-Json
foreach ($taskName in @('identity-vault.exe', 'profile-tests.exe', 'profile-client-tests.exe')) {
    $taskExpected = $taskManifest.PSObject.Properties[$taskName].Value
    if ($taskExpected -notmatch '^[0-9a-f]{64}$') { throw 'Invalid manifest' }
    if ((Get-FileHash -LiteralPath (Join-Path $taskFolder $taskName) -Algorithm SHA256).Hash.ToLowerInvariant() -ne $taskExpected) { throw "Checksum mismatch: $taskName" }
}
$taskPrevious = $env:REMOTEAPP_VAULT_TEST_EXECUTABLE
try {
    $env:REMOTEAPP_VAULT_TEST_EXECUTABLE = (Resolve-Path -LiteralPath (Join-Path $taskFolder 'identity-vault.exe')).Path
    $taskCases = @(
        @{ Binary='profile-tests.exe'; Run='^(TestReleaseProfileVerificationAndStrictParsing|TestRotationRequiresAdvanceAndNoSameEpochMutation)$'; Required=@('TestReleaseProfileVerificationAndStrictParsing','TestRotationRequiresAdvanceAndNoSameEpochMutation') },
        @{ Binary='profile-client-tests.exe'; Run='^(TestProfileHTTPSClientReleaseTrustAndExpiry|TestNativeOSSignerReleaseProfileHTTPS)$'; Required=@('TestProfileHTTPSClientReleaseTrustAndExpiry','TestNativeOSSignerReleaseProfileHTTPS') }
    )
    foreach ($taskCase in $taskCases) {
        $taskOutput = @(& (Join-Path $taskFolder $taskCase.Binary) ("-test.run="+$taskCase.Run) '-test.v=true' '-test.timeout=60s' 2>&1)
        $taskExit = $LASTEXITCODE
        $taskOutput | ForEach-Object { Write-Output "$_" }
        $taskText = ($taskOutput | ForEach-Object { "$_" }) -join "`n"
        if ($taskExit -ne 0 -or $taskText -match '--- SKIP:') { throw "Service trust test NOT passed (exit=$taskExit)" }
        foreach ($taskRequired in $taskCase.Required) {
            if ($taskText -notmatch ('--- PASS: '+[regex]::Escape($taskRequired)+'\s')) { throw "Missing PASS: $taskRequired" }
        }
    }
    Write-Output 'VALIDATION PASSED: signed release profile + OS signer HTTPS. NOT production trust installation or full product acceptance.'
} finally { $env:REMOTEAPP_VAULT_TEST_EXECUTABLE = $taskPrevious }
