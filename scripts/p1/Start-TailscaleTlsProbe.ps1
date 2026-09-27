param(
    [ValidateRange(1024, 65535)][int]$Port = 47475
)

$ErrorActionPreference = 'Stop'
$command = Get-Command tailscale.exe -ErrorAction SilentlyContinue
$tailscale = if ($command) { $command.Source } else {
    Join-Path $env:ProgramFiles 'Tailscale\tailscale.exe'
}
$rawStatus = & $tailscale status --json
if ($LASTEXITCODE -ne 0) { throw 'Tailscale status failed.' }
$status = $rawStatus | ConvertFrom-Json
if ($status.BackendState -ne 'Running' -or !$status.Self.Online) {
    throw 'Tailscale must be running and online.'
}
$localV4 = @($status.Self.TailscaleIPs | Where-Object { $_ -match '^100\.' })
if ($localV4.Count -ne 1) { throw 'Expected exactly one local Tailscale IPv4 address.' }
$macPeers = @($status.Peer.PSObject.Properties | ForEach-Object { $_.Value } |
    Where-Object { $_.OS -eq 'macOS' -and $_.Online })
if ($macPeers.Count -ne 1) { throw 'Expected exactly one online Mac peer.' }
$remoteV4 = @($macPeers[0].TailscaleIPs | Where-Object { $_ -match '^100\.' })
if ($remoteV4.Count -ne 1) { throw 'Expected exactly one Mac Tailscale IPv4 address.' }
$bindAddress = [Net.IPAddress]::Parse($localV4[0])
$expectedRemote = [Net.IPAddress]::Parse($remoteV4[0])
$octets = $bindAddress.GetAddressBytes()
if ($octets[1] -lt 64 -or $octets[1] -gt 127) {
    throw 'Bind address is not in the Tailscale IPv4 range.'
}
$assigned = @(Get-NetIPAddress -AddressFamily IPv4 -ErrorAction Stop |
    Where-Object { $_.IPAddress -eq $bindAddress.ToString() })
if ($assigned.Count -eq 0) { throw 'Tailscale address is not assigned locally.' }

$project = Join-Path $PSScriptRoot '..\..\windows\RemoteAgent\tools\TlsProbeServer\TlsProbeServer.csproj'
$bindText = $bindAddress.ToString()
$remoteText = $expectedRemote.ToString()
dotnet run --project $project -c Release -- $bindText $remoteText $Port
if ($LASTEXITCODE -ne 0) { throw 'TLS probe server failed.' }
