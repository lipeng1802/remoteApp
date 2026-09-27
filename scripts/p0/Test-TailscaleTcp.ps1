param(
    [ValidateRange(1024, 65535)][int]$Port = 47474,
    [ValidateRange(1, 600)][int]$WaitSeconds = 300,
    [ValidateRange(1, 30)][int]$ReadSeconds = 5,
    [switch]$LoopbackTest
)

$ErrorActionPreference = 'Stop'
$listener = $null
$client = $null
try {
    if ($LoopbackTest) {
        $bindAddress = [Net.IPAddress]::Loopback
        $expectedRemote = $bindAddress
    } else {
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
        $macPeers = @($status.Peer.PSObject.Properties | ForEach-Object { $_.Value } |
            Where-Object { $_.OS -eq 'macOS' -and $_.Online })
        if ($macPeers.Count -ne 1) { throw 'Expected exactly one online Mac peer.' }
        $localV4 = @($status.Self.TailscaleIPs | Where-Object { $_ -match '^100\.' })
        $remoteV4 = @($macPeers[0].TailscaleIPs | Where-Object { $_ -match '^100\.' })
        if ($localV4.Count -ne 1 -or $remoteV4.Count -ne 1) {
            throw 'Expected Tailscale IPv4 addresses.'
        }
        $bindAddress = [Net.IPAddress]::Parse($localV4[0])
        $expectedRemote = [Net.IPAddress]::Parse($remoteV4[0])
        $octets = $bindAddress.GetAddressBytes()
        if ($octets[1] -lt 64 -or $octets[1] -gt 127) {
            throw 'Bind address is not in the Tailscale IPv4 range.'
        }
        $assigned = @(Get-NetIPAddress -AddressFamily IPv4 -ErrorAction Stop |
            Where-Object { $_.IPAddress -eq $bindAddress.ToString() })
        if ($assigned.Count -eq 0) { throw 'Tailscale address is not assigned locally.' }
    }

    $listener = [Net.Sockets.TcpListener]::new($bindAddress, $Port)
    $listener.Server.ExclusiveAddressUse = $true
    $listener.Start(1)
    $mode = if ($LoopbackTest) { 'loopback-only' } else { 'tailscale-only' }
    Write-Output "READY mode=$mode port=$Port wait_seconds=$WaitSeconds"
    $accept = $listener.AcceptTcpClientAsync()
    if (!$accept.Wait($WaitSeconds * 1000)) { throw 'Accept timeout; no test completed.' }
    $client = $accept.GetAwaiter().GetResult()
    if (!$client.Client.RemoteEndPoint.Address.Equals($expectedRemote)) {
        throw 'Unexpected peer; request rejected.'
    }
    $stream = $client.GetStream()
    $stream.WriteTimeout = $ReadSeconds * 1000
    $expected = [Text.Encoding]::ASCII.GetBytes("prd-p0-test`n")
    $timer = [Diagnostics.Stopwatch]::StartNew()
    # Exact fixed-length request: no unbounded ReadLine or input logging.
    foreach ($value in $expected) {
        $remaining = ($ReadSeconds * 1000) - [int]$timer.ElapsedMilliseconds
        if ($remaining -le 0) { throw 'Request timeout.' }
        $stream.ReadTimeout = $remaining
        $received = $stream.ReadByte()
        if ($received -ne $value) { throw 'Invalid or incomplete request.' }
    }
    $response = [Text.Encoding]::ASCII.GetBytes("prd-p0-ok`n")
    $stream.Write($response, 0, $response.Length)
    $stream.Flush()
    Write-Output 'PASS expected request received; response sent'
} catch {
    # Avoid printing raw socket errors, addresses, or arbitrary peer data.
    Write-Output ('FAIL probe stopped (' + $_.Exception.GetType().Name + ')')
    exit 1
} finally {
    if ($null -ne $client) { $client.Dispose() }
    if ($null -ne $listener) { $listener.Stop() }
    Write-Output 'CLOSED temporary listener'
}
