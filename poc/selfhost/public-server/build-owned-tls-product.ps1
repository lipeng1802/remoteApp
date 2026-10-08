param(
    [Parameter(Mandatory=$true)][string]$WorkDirectory,
    [Parameter(Mandatory=$true)][string]$SourceSHA256
)
# Trusted development build, not a desktop launcher or installer. Archive is
# the current repository's trusted source; never accept peer upload/hash pairs.
$ErrorActionPreference = 'Stop'
if ($SourceSHA256 -cnotmatch '^[0-9a-f]{64}$' -or ![IO.Path]::IsPathRooted($WorkDirectory)) { throw 'Invalid trusted build input' }
$taskBase = (Resolve-Path -LiteralPath $WorkDirectory).Path
$taskArchive = Join-Path $taskBase 'product-source.tar.gz'
if ((Get-FileHash -LiteralPath $taskArchive -Algorithm SHA256).Hash.ToLowerInvariant() -ne $SourceSHA256) { throw 'Source hash mismatch' }
$taskPublish = Join-Path $taskBase 'product-publish'
if ((Test-Path -LiteralPath (Join-Path $taskBase 'windows')) -or (Test-Path -LiteralPath $taskPublish)) { throw 'Build output already exists; do not overwrite' }
& tar -xzf $taskArchive -C $taskBase
if ($LASTEXITCODE -ne 0) { throw 'Source extraction failed' }
& dotnet publish (Join-Path $taskBase 'windows\RemoteAgent\tools\TlsBridgeProbe') -c Release -r win-x64 --self-contained true -p:PublishSingleFile=true -p:IncludeNativeLibrariesForSelfExtract=true -o $taskPublish
if ($LASTEXITCODE -ne 0) { throw 'Product build failed; partial output retained' }
$taskExe = Join-Path $taskPublish 'TlsBridgeProbe.exe'
if (!(Test-Path -LiteralPath $taskExe -PathType Leaf) -or (Get-Item -LiteralPath $taskExe).Length -gt 104857600) { throw 'Bounded single-file product unavailable' }
Write-Output 'BUILD_READY: single-file TLS probe. NOT started, trusted installation, native signer or cross-machine acceptance.'
Get-FileHash -LiteralPath $taskExe -Algorithm SHA256 | Select-Object Hash
