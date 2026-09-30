param(
    [ValidateRange(1024, 65535)][int]$Port = 47475,
    [switch]$AllowLocalMock
)

$ErrorActionPreference = 'Stop'
if (!$AllowLocalMock) {
    throw 'Explicit consent required. Re-run with -AllowLocalMock. No listener was started.'
}

$command = Get-Command tailscale.exe -ErrorAction SilentlyContinue
$tailscale = if ($command) { $command.Source } else {
    Join-Path $env:ProgramFiles 'Tailscale\tailscale.exe'
}
$statusInfo = New-Object System.Diagnostics.ProcessStartInfo
$statusInfo.FileName = $tailscale
$statusInfo.Arguments = 'status --json'
$statusInfo.UseShellExecute = $false
$statusInfo.CreateNoWindow = $true
$statusInfo.RedirectStandardOutput = $true
$statusInfo.RedirectStandardError = $true
$statusInfo.StandardOutputEncoding = New-Object System.Text.UTF8Encoding($false, $true)
$statusInfo.StandardErrorEncoding = New-Object System.Text.UTF8Encoding($false, $true)
$statusProcess = New-Object System.Diagnostics.Process
$statusProcess.StartInfo = $statusInfo
try {
    $null = $statusProcess.Start()
    $outputTask = $statusProcess.StandardOutput.ReadToEndAsync()
    $errorTask = $statusProcess.StandardError.ReadToEndAsync()
    if (!$statusProcess.WaitForExit(10000)) {
        $statusProcess.Kill()
        throw 'Tailscale status timed out.'
    }
    if ($statusProcess.ExitCode -ne 0) { throw 'Tailscale status failed.' }
    try {
        $rawStatus = $outputTask.GetAwaiter().GetResult()
        $null = $errorTask.GetAwaiter().GetResult()
        $status = ConvertFrom-Json -InputObject $rawStatus -ErrorAction Stop
    } catch {
        throw 'Unable to parse Tailscale UTF-8 status JSON. No listener was started.'
    }
} finally {
    $statusProcess.Dispose()
}

if ($status.BackendState -ne 'Running' -or !$status.Self.Online) {
    throw 'Tailscale must be running and online.'
}
$localV4 = @($status.Self.TailscaleIPs | Where-Object { $_ -match '^100\.' })
if ($localV4.Count -ne 1) { throw 'Expected exactly one local Tailscale IPv4 address.' }
$macPeers = @($status.Peer.PSObject.Properties | ForEach-Object { $_.Value } |
    Where-Object { $_.OS -eq 'macOS' -and $_.Online })
if ($macPeers.Count -eq 0) {
    throw 'No online Mac peer. Connect Tailscale on the Mac, then retry. No listener was started.'
}
if ($macPeers.Count -gt 1) {
    throw 'Multiple online Mac peers. This mock requires exactly one. No listener was started.'
}
$remoteV4 = @($macPeers[0].TailscaleIPs | Where-Object { $_ -match '^100\.' })
if ($remoteV4.Count -ne 1) { throw 'Expected exactly one Mac Tailscale IPv4 address.' }
$bindAddress = [Net.IPAddress]::Parse($localV4[0])
$expectedRemote = [Net.IPAddress]::Parse($remoteV4[0])
$octets = $bindAddress.GetAddressBytes()
if ($octets[1] -lt 64 -or $octets[1] -gt 127) {
    throw 'Bind address is not in the Tailscale IPv4 range.'
}
$remoteOctets = $expectedRemote.GetAddressBytes()
if ($remoteOctets[1] -lt 64 -or $remoteOctets[1] -gt 127) {
    throw 'Peer address is not in the Tailscale IPv4 range.'
}
$assigned = @(Get-NetIPAddress -AddressFamily IPv4 -ErrorAction Stop |
    Where-Object { $_.IPAddress -eq $bindAddress.ToString() })
if ($assigned.Count -eq 0) { throw 'Tailscale address is not assigned locally.' }

$project = Join-Path $PSScriptRoot '..\..\windows\RemoteAgent\tools\InputMockServer\InputMockServer.csproj'
$bindText = $bindAddress.ToString()
$remoteText = $expectedRemote.ToString()
dotnet run --project $project -c Release -- --allow-local-mock $bindText $remoteText $Port
if ($LASTEXITCODE -ne 0) { throw 'Tailscale input mock failed.' }
