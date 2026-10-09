param(
    [Parameter(Mandatory=$true)][string]$Root,
    [Parameter(Mandatory=$true)][string]$IdentityBootstrap
)
$ErrorActionPreference = 'Stop'
# Developer acceptance staging ONLY. Public management files are not product
# authorization, signed release trust or a substitute for OS consent.
if ($Root -notmatch '^C:\\Users\\jarvis\\remoteapp-compiled-stage-[0-9a-f]{16}$') { throw 'Invalid staging root' }
if (-not (Test-Path -LiteralPath $Root -PathType Container)) { throw 'Staging root missing' }
if ((Get-Item -LiteralPath $Root).Attributes -band [IO.FileAttributes]::ReparsePoint) { throw 'Staging root redirected' }
$IdentityBootstrap = (Resolve-Path -LiteralPath $IdentityBootstrap).Path
function Read-PublicJson([string]$Path, [int]$Limit) {
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) { return $null }
    $item = Get-Item -LiteralPath $Path
    if ($item.Length -gt $Limit -or ($item.Attributes -band [IO.FileAttributes]::ReparsePoint)) { throw 'Invalid public staging file' }
    return [IO.File]::ReadAllText($Path) | ConvertFrom-Json
}
function Check-Fields($Value, [string[]]$Names) {
    $actual = @($Value.PSObject.Properties.Name | Sort-Object)
    if (($actual -join ',') -cne (($Names | Sort-Object) -join ',')) { throw 'Invalid public staging fields' }
}
Write-Host 'WATCH_READY: waiting for a fresh verified borrower package. No network session has started.'
Write-Host 'Keep this desktop window open. The borrower will start here automatically after preparation.'
$deadline = [DateTime]::UtcNow.AddMinutes(30)
while ([DateTime]::UtcNow -lt $deadline) {
    $ping = Read-PublicJson (Join-Path $Root 'ping.json') 1024
    if ($null -ne $ping) {
        Check-Fields $ping @('nonce')
        if ($ping.nonce -isnot [string] -or $ping.nonce -cnotmatch '^[0-9a-f]{32}$') { throw 'Invalid staging challenge' }
        $reply = @{nonce=$ping.nonce; status='waiting'} | ConvertTo-Json -Compress
        [IO.File]::WriteAllText((Join-Path $Root 'ack.new'), $reply)
        Move-Item -LiteralPath (Join-Path $Root 'ack.new') -Destination (Join-Path $Root 'ack.json') -Force
    }
    $request = Read-PublicJson (Join-Path $Root 'launch.json') 2048
    if ($null -ne $request) {
        Check-Fields $request @('nonce','manifest_sha256')
        if ($null -eq $ping -or $request.nonce -cne $ping.nonce -or
            $request.manifest_sha256 -isnot [string] -or $request.manifest_sha256 -cnotmatch '^[0-9a-f]{64}$') { throw 'Invalid staging launch' }
        $package = Join-Path $Root 'run\compiled-win'
        foreach ($path in @((Join-Path $Root 'run'),$package)) {
            if ((Get-Item -LiteralPath $path).Attributes -band [IO.FileAttributes]::ReparsePoint) { throw 'Package redirected' }
        }
        $manifestPath = Join-Path $package 'SHA256.json'
        $manifest = Read-PublicJson $manifestPath 16384
        if ($null -eq $manifest -or (Get-FileHash -LiteralPath $manifestPath -Algorithm SHA256).Hash.ToLowerInvariant() -cne $request.manifest_sha256) { throw 'Manifest hash mismatch' }
        $required = @('build-public.json','desktop-relay.exe','desktop-session.exe','identity-vault.exe','interactive-vault.exe','probe-desktop-session.ps1','start-borrowed-desktop-session.ps1','start-desktop-session.ps1','tls-product.exe','trust-install.exe','trust-journal.exe','vault-relay.exe')
        Check-Fields $manifest $required
        foreach ($property in $manifest.PSObject.Properties) {
            $file = Join-Path $package $property.Name
            $item = Get-Item -LiteralPath $file
            if (($item.Attributes -band [IO.FileAttributes]::ReparsePoint) -or $item.PSIsContainer -or
                $property.Value -isnot [string] -or $property.Value -cnotmatch '^[0-9a-f]{64}$' -or
                (Get-FileHash -LiteralPath $file -Algorithm SHA256).Hash.ToLowerInvariant() -cne $property.Value) { throw 'Package hash mismatch' }
        }
        Write-Host 'PACKAGE_VERIFIED: starting the fresh borrower in this desktop session.'
        & powershell.exe -NoProfile -ExecutionPolicy Bypass -File (Join-Path $package 'start-borrowed-desktop-session.ps1') -IdentityBootstrap $IdentityBootstrap
        if ($LASTEXITCODE -ne 0) { throw 'Borrower cleanup NOT confirmed' }
        exit 0
    }
    Start-Sleep -Milliseconds 300
}
throw 'Waiting window expired; no borrower launched'
