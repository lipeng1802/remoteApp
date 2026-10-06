# Windows x64 isolated lab. PowerShell 5.1+, Docker Desktop Linux containers.
# Bootstrap secrets only pass over redirected stdin; never print child logs.
[CmdletBinding()]
param([ValidateRange(1, 5)][int]$RestartCount = 3)
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$script:Stage = 'preflight'
$script:Children = New-Object System.Collections.Generic.List[object]
$script:Folders = New-Object System.Collections.Generic.List[string]
$script:Container = $null
$script:RunId = [Guid]::NewGuid().ToString('N')
$script:Image = 'ghcr.io/juanfont/headscale:0.29.4'
$script:Docker = $null
$originalLocation = Get-Location
$pocRoot = $PSScriptRoot
$repoRoot = [IO.Path]::GetFullPath((Join-Path $pocRoot '..\..'))
$artifacts = Join-Path $repoRoot 'artifacts\connection-poc'
$nodesRoot = Join-Path $artifacts 'nodes'
$nodeExe = Join-Path $artifacts 'selfhost-node-win-x64.exe'
$derpExe = Join-Path $artifacts 'lab-derp-win-x64.exe'
$utf8 = New-Object Text.UTF8Encoding($false)

function Assert-Lab([bool]$Condition, [string]$Label) {
    $script:Stage = $Label
    if (-not $Condition) { throw 'lab_check_failed' }
    Write-Host "PASS $Label"
}

# Windows CommandLineToArgvW quoting; no shell invocation or interpolation.
function Quote-Argument([string]$Value) {
    if ($Value -notmatch '[\s"]' -and $Value.Length -gt 0) { return $Value }
    $escaped = [regex]::Replace($Value, '(\\*)"', '$1$1\"')
    $escaped = [regex]::Replace($escaped, '(\\+)$', '$1$1')
    return '"' + $escaped + '"'
}

function Start-Child([string]$Executable, [string[]]$Arguments) {
    $info = New-Object Diagnostics.ProcessStartInfo
    $info.FileName = $Executable
    $info.Arguments = (($Arguments | ForEach-Object { Quote-Argument $_ }) -join ' ')
    $info.WorkingDirectory = $pocRoot
    $info.UseShellExecute = $false
    $info.CreateNoWindow = $true
    $info.RedirectStandardInput = $true
    $info.RedirectStandardOutput = $true
    $info.RedirectStandardError = $true
    $info.StandardOutputEncoding = $utf8
    $info.StandardErrorEncoding = $utf8
    $proc = New-Object Diagnostics.Process
    $proc.StartInfo = $info
    [void]$proc.Start()
    $proc.StandardInput.NewLine = "`n"
    $child = [PSCustomObject]@{ Process = $proc; ReadTask = $null; Errors = $proc.StandardError.ReadToEndAsync() }
    $script:Children.Add($child)
    return $child
}

function Invoke-Docker([string[]]$Arguments, [string]$Label) {
    $script:Stage = $Label
    $child = Start-Child $script:Docker $Arguments
    $child.Process.StandardInput.Close()
    $out = $child.Process.StandardOutput.ReadToEndAsync()
    if (-not $child.Process.WaitForExit(30000)) { throw 'docker_timeout' }
    if ($child.Process.ExitCode -ne 0) { throw 'docker_failed' }
    return $out.GetAwaiter().GetResult()
}

function New-PrivateFolder([string]$Parent, [string]$Name) {
    $path = Join-Path $Parent $Name
    if (Test-Path -LiteralPath $path) { throw 'existing_fixture_rejected' }
    $acl = New-Object Security.AccessControl.DirectorySecurity
    $acl.SetAccessRuleProtection($true, $false)
    $sid = [Security.Principal.WindowsIdentity]::GetCurrent().User
    $rule = New-Object Security.AccessControl.FileSystemAccessRule($sid, 'FullControl', 'ContainerInherit,ObjectInherit', 'None', 'Allow')
    $acl.AddAccessRule($rule)
    [void][IO.Directory]::CreateDirectory($path)
    $script:Folders.Add($path)
    Set-Acl -LiteralPath $path -AclObject $acl
    return $path
}

function Read-Event($Child, [int]$TimeoutMs = 35000) {
    if ($null -eq $Child.ReadTask) { $Child.ReadTask = $Child.Process.StandardOutput.ReadLineAsync() }
    if (-not $Child.ReadTask.Wait($TimeoutMs)) { throw 'status_timeout' }
    $line = $Child.ReadTask.GetAwaiter().GetResult()
    $Child.ReadTask = $null
    if ($null -eq $line) { return $null }
    $item = $line | ConvertFrom-Json
    $allowed = @('registered','listening','exchange_passed','token_rejected','selfhost_relay','direct','unknown',
        'probe_passed','registration_failed','dial_failed','request_failed','response_failed','status_failed',
        'relay_not_verified','dialing','closing','closed','cleanup_timeout','wrong_token_rejected',
        'negative_dial_failed','wrong_token_accepted','invalid_config','invalid_control','invalid_arguments')
    if ($item.status -notin $allowed) { throw 'unexpected_status' }
    return $item
}

function Collect-Events($Child) {
    $items = New-Object System.Collections.Generic.List[object]
    $deadline = [DateTime]::UtcNow.AddSeconds(45)
    while ($true) {
        $remaining = [int]($deadline - [DateTime]::UtcNow).TotalMilliseconds
        if ($remaining -le 0) { throw 'node_timeout' }
        $item = Read-Event $Child $remaining
        if ($null -eq $item) { break }
        $items.Add($item)
        if ($item.status -eq 'closing') { $deadline = [DateTime]::UtcNow.AddSeconds(12) }
    }
    if (-not $Child.Process.WaitForExit(5000)) { throw 'node_exit_timeout' }
    return ,$items.ToArray()
}

function Stop-Node($Child) {
    $script:Stage = 'node_graceful_stop'
    Assert-Lab (-not $Child.Process.HasExited) 'server_alive_before_stop'
    $Child.Process.StandardInput.WriteLine('stop')
    $Child.Process.StandardInput.Flush()
    $Child.Process.StandardInput.Close()
    $events = Collect-Events $Child
    $statuses = @($events | ForEach-Object { $_.status })
    Assert-Lab ($Child.Process.ExitCode -eq 0 -and 'closed' -in $statuses -and 'cleanup_timeout' -notin $statuses) 'server_closed_gracefully'
}

function Admin([string[]]$Arguments) {
    $raw = Invoke-Docker (@('exec', $script:Container, '/ko-app/headscale', '--config', '/lab/config.yaml') + $Arguments + @('--output','json')) 'headscale_admin'
    return ($raw | ConvertFrom-Json)
}

function New-NodeConfig([string]$Role, [string]$Name) {
    $folder = New-PrivateFolder $nodesRoot ($Name + '-' + [Guid]::NewGuid().ToString('N'))
    $registration = Admin @('preauthkeys','create','--user', [string]$script:UserId, '--expiration','5m')
    return @{ role = $Role; control_url = 'http://127.0.0.1:18443'; state_dir = $folder; hostname = $Name;
        auth_key = $registration.key; session_token = $script:Token; peer = $(if ($Role -eq 'probe') {'100.120.0.1'} else {''});
        force_relay = $true; check_reject = ($Name -eq 'poc-client') }
}

function Start-Node($Config) {
    $child = Start-Child $nodeExe @('--control-stdin')
    $child.Process.StandardInput.WriteLine(($Config | ConvertTo-Json -Compress))
    $child.Process.StandardInput.Flush()
    return $child
}

function Verify-Probe($Config) {
    $child = Start-Node $Config
    $events = Collect-Events $child
    $statuses = @($events | ForEach-Object { $_.status })
    $registered = @($events | Where-Object { $_.status -eq 'registered' })
    Assert-Lab ($registered.Count -eq 1 -and '100.120.0.2' -in $registered[0].addresses) 'client_identity_retained'
    Assert-Lab ('selfhost_relay' -in $statuses -and 'probe_passed' -in $statuses) 'fixed_payload_over_selfhost_derp'
    Assert-Lab ('wrong_token_rejected' -in $statuses) 'wrong_token_rejected'
    Assert-Lab ($child.Process.ExitCode -eq 0 -and 'closed' -in $statuses -and 'cleanup_timeout' -notin $statuses) 'client_closed_gracefully'
}

function Verify-ServerExchange($Server) {
    Assert-Lab ((Read-Event $Server).status -eq 'exchange_passed') 'server_confirmed_payload'
    Assert-Lab ((Read-Event $Server).status -eq 'token_rejected') 'server_confirmed_wrong_token'
}

$passed = $false
$cleaned = $true
try {
    Set-Location -LiteralPath $pocRoot
    Assert-Lab ($env:OS -eq 'Windows_NT' -and [Environment]::Is64BitOperatingSystem) 'windows_x64'
    foreach ($file in @($nodeExe, $derpExe)) { Assert-Lab (Test-Path -LiteralPath $file -PathType Leaf) 'test_binary_present' }
    $manifestPath = Join-Path $repoRoot 'checksums.json'
    Assert-Lab (Test-Path -LiteralPath $manifestPath -PathType Leaf) 'bundle_manifest_present'
    $manifest = Get-Content -LiteralPath $manifestPath -Raw | ConvertFrom-Json
    $expectedFiles = @('poc/selfhost/verify-windows.ps1','poc/selfhost/headscale.yaml','poc/selfhost/policy.json',
        'artifacts/connection-poc/selfhost-node-win-x64.exe','artifacts/connection-poc/lab-derp-win-x64.exe','THIRD_PARTY_NOTICES.txt')
    foreach ($relative in $expectedFiles) {
        $hash = (Get-FileHash -LiteralPath (Join-Path $repoRoot $relative) -Algorithm SHA256).Hash.ToLowerInvariant()
        Assert-Lab ($hash -eq $manifest.$relative) 'bundle_file_hash_matches'
    }
    $script:Docker = (Get-Command docker.exe -ErrorAction Stop).Source
    $context = @(Invoke-Docker @('context','inspect') 'docker_context' | ConvertFrom-Json)[0]
    Assert-Lab ($context.Endpoints.docker.Host.StartsWith('npipe://')) 'local_docker_context'
    $osType = Invoke-Docker @('info','--format','{{.OSType}}') 'docker_engine'
    Assert-Lab ($osType.Trim() -eq 'linux') 'docker_linux_containers'
    # No implicit image pull, Docker config changes, WSL install or firewall edits.
    $imageId = (Invoke-Docker @('image','inspect','--format','{{.Id}}', $script:Image) 'headscale_image_missing_pull_first').Trim()
    Assert-Lab ($imageId -match '^sha256:[a-f0-9]{64}$') 'headscale_image_present'
    foreach ($port in @(18443,18444,19090,15443)) {
        $listener = New-Object Net.Sockets.TcpListener([Net.IPAddress]::Loopback, $port)
        try { $listener.Start() } finally { $listener.Stop() }
    }
    [void][IO.Directory]::CreateDirectory($nodesRoot)
    $lab = New-PrivateFolder $nodesRoot ('lab-' + $script:RunId)
    $derp = Start-Child $derpExe @()
    $derp.Process.StandardInput.Close()
    $relay = Read-Event $derp
    Assert-Lab ($relay.status -eq 'listening' -and $relay.cert_name -match '^sha256-raw:[a-f0-9]{64}$') 'pinned_local_derp_started'
    $derpMap = "regions:`n  999:`n    regionid: 999`n    regioncode: lab`n    regionname: RemoteApp Local Lab`n    nodes:`n      - name: lab-1`n        regionid: 999`n        hostname: 127.0.0.1`n        ipv4: 127.0.0.1`n        ipv6: none`n        stunport: -1`n        derpport: 18444`n        certname: $($relay.cert_name)`n"
    [IO.File]::WriteAllText((Join-Path $lab 'derp.yaml'), $derpMap, $utf8)
    $config = [IO.File]::ReadAllText((Join-Path $pocRoot 'headscale.yaml'))
    $config = $config.Replace('listen_addr: 127.0.0.1:18443', 'listen_addr: 0.0.0.0:18443')
    $config = $config.Replace('../../artifacts/connection-poc/headscale', '/lab')
    $config = $config.Replace('paths: [derp.yaml]', 'paths: [/lab/derp.yaml]').Replace('path: policy.json', 'path: /lab/policy.json')
    [IO.File]::WriteAllText((Join-Path $lab 'config.yaml'), $config, $utf8)
    Copy-Item -LiteralPath (Join-Path $pocRoot 'policy.json') -Destination (Join-Path $lab 'policy.json')
    $containerId = Invoke-Docker @('create','--name', ('remoteapp-poc-' + $script:RunId), '--label', ('remoteapp.poc=' + $script:RunId),
        '--read-only','--tmpfs','/tmp','--cap-drop','ALL','--security-opt','no-new-privileges',
        '--publish','127.0.0.1:18443:18443','--mount', ("type=bind,source=$lab,target=/lab"),
        $imageId, '--config','/lab/config.yaml','serve') 'create_headscale_fixture'
    $script:Container = $containerId.Trim()
    Assert-Lab ($script:Container -match '^[a-f0-9]{64}$') 'isolated_container_created'
    [void](Invoke-Docker @('start', $script:Container) 'start_headscale_fixture')
    $deadline = [DateTime]::UtcNow.AddSeconds(20)
    $healthy = $false
    while ([DateTime]::UtcNow -lt $deadline) {
        try {
            $request = [Net.HttpWebRequest]::Create('http://127.0.0.1:18443/health')
            $request.Proxy = $null; $request.Timeout = 1000
            $response = $request.GetResponse()
            $healthy = ([int]$response.StatusCode -eq 200); $response.Close()
            if ($healthy) { break }
        } catch { Start-Sleep -Milliseconds 200 }
    }
    Assert-Lab $healthy 'isolated_headscale_healthy'
    $user = Admin @('users','create','poc')
    $script:UserId = $user.id
    $bytes = New-Object byte[] 32
    $rng = [Security.Cryptography.RandomNumberGenerator]::Create()
    try { $rng.GetBytes($bytes) } finally { $rng.Dispose() }
    $script:Token = ([BitConverter]::ToString($bytes)).Replace('-', '').ToLowerInvariant()
    $serverConfig = New-NodeConfig 'serve' 'poc-server'
    $clientConfig = $null
    $server = Start-Node $serverConfig
    $registered = Read-Event $server
    Assert-Lab ($registered.status -eq 'registered' -and '100.120.0.1' -in $registered.addresses) 'server_registered_without_browser'
    Assert-Lab ((Read-Event $server).status -eq 'listening') 'server_listener_ready'
    $clientConfig = New-NodeConfig 'probe' 'poc-client'
    for ($i = 0; $i -le $RestartCount; $i++) {
        Verify-Probe $clientConfig
        Verify-ServerExchange $server
    }
    Assert-Lab (@(Admin @('nodes','list')).Count -eq 2) 'client_restart_no_new_identity'
    $outsider = Start-Node (New-NodeConfig 'probe' 'poc-denied')
    $events = Collect-Events $outsider
    $statuses = @($events | ForEach-Object { $_.status })
    Assert-Lab ($outsider.Process.ExitCode -ne 0 -and 'registered' -in $statuses -and 'dial_failed' -in $statuses) 'third_node_denied_by_acl'
    Assert-Lab ('closed' -in $statuses -and 'cleanup_timeout' -notin $statuses) 'denied_node_closed_gracefully'
    Stop-Node $server
    $server = Start-Node $serverConfig
    $registered = Read-Event $server
    Assert-Lab ($registered.status -eq 'registered' -and '100.120.0.1' -in $registered.addresses) 'server_restart_same_identity'
    Assert-Lab ((Read-Event $server).status -eq 'listening') 'restarted_listener_ready'
    Verify-Probe $clientConfig
    Verify-ServerExchange $server
    Assert-Lab (@(Admin @('nodes','list')).Count -eq 3) 'restart_no_fourth_node'
    Stop-Node $server
    $passed = $true
} catch {
    # Never print exception text, Docker stdout/stderr, bootstrap or private state.
    Write-Host "FAIL $script:Stage"
} finally {
    foreach ($child in $script:Children) {
        try {
            if (-not $child.Process.HasExited) {
                # Emergency cleanup only. Any node using this path has failed.
                $child.Process.Kill()
                [void]$child.Process.WaitForExit(5000)
            }
            $child.Process.Dispose()
        } catch { $cleaned = $false }
    }
    if ($null -ne $script:Container -and $script:Container -match '^[a-f0-9]{64}$') {
        try { [void](Invoke-Docker @('rm','--force', $script:Container) 'remove_owned_container') }
        catch { $cleaned = $false }
    }
    # Exact paths created by this run only; never recurse over a parent/root.
    if ($cleaned) {
        foreach ($folder in $script:Folders) {
            try { Remove-Item -LiteralPath $folder -Recurse -Force } catch { $cleaned = $false }
        }
    }
    Set-Location $originalLocation
}
if ($passed -and $cleaned) { Write-Host 'PASS windows_smoke_complete'; exit 0 }
if (-not $cleaned) { Write-Host 'FAIL fixture_cleanup_incomplete' }
exit 1
