param([Parameter(Mandatory=$true)][string]$BundleDirectory)
$ErrorActionPreference = 'Stop'
$bundle = (Resolve-Path -LiteralPath $BundleDirectory).Path
$groups = @(
    @{ File='product-observation-tests-win-4v.exe'; Hash='542b49053e611f3d2bfd8c30124f248c1d4ff759e5819f394c8510ea5b935231'; Tests=@('TestProductObservationRoleSchemas','TestProductObservationUnavailableAndJoined') },
    @{ File='desktop-count-tests-win-4v.exe'; Hash='045018884e7d655af22d85ffcde38b1a966e0d5ac17fa885865858c172c9a3f1'; Tests=@('TestPrivateStartAndStopAckOnlyAfterJoin','TestCountRejectsRuntimeAndProfileOverrides','TestCancellationJoinsIdleConnections') }
)
foreach ($group in $groups) {
    $exe = Join-Path $bundle $group.File
    if ((Get-FileHash -LiteralPath $exe -Algorithm SHA256).Hash.ToLowerInvariant() -ne $group.Hash) { throw 'Validation binary digest mismatch' }
    $pattern = '^(' + ($group.Tests -join '|') + ')$'
    $lines = @(& $exe "-test.run=$pattern" '-test.v=true' 2>&1)
    $exit = $LASTEXITCODE
    $lines | ForEach-Object { Write-Host $_ }
    $result = $lines -join "`n"
    if ($exit -ne 0 -or $result -match '(?m)^--- SKIP:' -or $result -match '(?m)^--- FAIL:') { throw 'Product observation validation failed or skipped' }
    foreach ($test in $group.Tests) {
        if ($result -notmatch ('(?m)^--- PASS: ' + [regex]::Escape($test) + ' [(]')) { throw "Mandatory test missing: $test" }
    }
}
Write-Host 'VALIDATION PASSED: product count projection + local private control + actual child join. NOT native storage, cross-machine Headscale or real TLS payload acceptance.'
