param([Parameter(Mandatory=$true)][string]$Executable,
      [Parameter(Mandatory=$true)][string]$ExpectedSHA256)
$ErrorActionPreference='Stop'
if ($ExpectedSHA256 -notmatch '^[0-9a-f]{64}$' -or (Get-FileHash -LiteralPath $Executable -Algorithm SHA256).Hash.ToLowerInvariant() -ne $ExpectedSHA256) {throw 'Test binary checksum mismatch'}
$taskOutput=@(& $Executable '-test.run=^TestTestPair' '-test.v=true' '-test.timeout=30s')
$taskExit=$LASTEXITCODE;$taskOutput|Write-Output
if ($taskExit -ne 0 -or ($taskOutput|Where-Object {$_ -match '^--- SKIP:|^--- FAIL:'})) {throw 'Test pair material validation NOT passed'}
foreach ($taskName in @('TestTestPairMatchingIssuerCAAndAdvance','TestTestPairRejectsBadTrustedBootstrap')) {
    if (!($taskOutput|Where-Object {$_ -like "--- PASS: $taskName (*)"})) {throw "Mandatory PASS missing: $taskName"}
}
Write-Output 'VALIDATION PASSED: isolated matching test trust material + signed epoch candidate + rejection checks. NOT native install or cross-machine acceptance.'
