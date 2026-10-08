param([Parameter(Mandatory=$true)][string]$BundleDirectory,
      [Parameter(Mandatory=$true)][string]$ProtocolTests,
      [Parameter(Mandatory=$true)][string]$TestsSHA256)
$ErrorActionPreference='Stop'
if ($TestsSHA256 -notmatch '^[0-9a-f]{64}$' -or (Get-FileHash -LiteralPath $ProtocolTests -Algorithm SHA256).Hash.ToLowerInvariant() -ne $TestsSHA256) { throw 'Protocol test checksum mismatch' }
$taskManifest=Get-Content -LiteralPath (Join-Path $BundleDirectory 'SHA256.json') -Raw | ConvertFrom-Json
foreach ($taskProperty in $taskManifest.PSObject.Properties) {
    if ($taskProperty.Name -match '[/\\:]' -or $taskProperty.Value -notmatch '^[0-9a-f]{64}$') { throw 'Unsafe manifest' }
    $taskPath=Join-Path $BundleDirectory $taskProperty.Name
    if ((Get-FileHash -LiteralPath $taskPath -Algorithm SHA256).Hash.ToLowerInvariant() -ne $taskProperty.Value) { throw "Checksum mismatch: $($taskProperty.Name)" }
    if ($taskProperty.Name.EndsWith('.ps1')) {
        $taskTokens=$null; $taskErrors=$null
        [void][Management.Automation.Language.Parser]::ParseFile($taskPath,[ref]$taskTokens,[ref]$taskErrors)
        if ($taskErrors.Count -ne 0) { throw "PowerShell syntax failure: $($taskProperty.Name)" }
    }
}
$taskOutput=@(& $ProtocolTests '-test.v=true' '-test.timeout=30s')
$taskExit=$LASTEXITCODE
$taskOutput | Write-Output
if ($taskExit -ne 0 -or ($taskOutput | Where-Object { $_ -match '^--- SKIP:|^--- FAIL:' })) { throw 'Protocol acceptance NOT passed' }
foreach ($taskTest in @('TestPrivateStartAndStopAckOnlyAfterJoin','TestRejectUnknownFieldsAndWrongIdentity','TestCancellationJoinsIdleConnections','TestWindowsPipeNamespaceCollisionAndProbe')) {
    if (!($taskOutput | Where-Object { $_ -like "--- PASS: $taskTest (*)" })) { throw "Required PASS missing: $taskTest" }
}
Write-Output 'VALIDATION PASSED: transferred bundle checksums + PowerShell syntax + Windows same-user pipe protocol. NOT native identity/trust initialization or cross-machine acceptance.'
