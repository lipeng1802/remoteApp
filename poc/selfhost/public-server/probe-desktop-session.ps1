$ErrorActionPreference='Stop'
$taskManifest=Get-Content -LiteralPath (Join-Path $PSScriptRoot 'SHA256.json') -Raw | ConvertFrom-Json
$taskRelay=Join-Path $PSScriptRoot 'desktop-relay.exe'
if ((Get-FileHash -LiteralPath $taskRelay -Algorithm SHA256).Hash.ToLowerInvariant() -ne $taskManifest.'desktop-relay.exe') { throw 'Relay checksum mismatch' }
$taskBootstrap=Get-Content -LiteralPath (Join-Path $PSScriptRoot 'desktop-bootstrap.json') -Raw | ConvertFrom-Json
if ($taskBootstrap.status -ne 'ready' -or $taskBootstrap.scope -notmatch '^[0-9a-f]{32}$' -or $taskBootstrap.public_key -notmatch '^[0-9a-f]{64}$' -or $taskBootstrap.pipe -ne ('\\.\pipe\remoteapp-session-'+$taskBootstrap.scope)) { throw 'Invalid desktop bootstrap' }
$taskPrevious=$env:REMOTEAPP_DESKTOP_SESSION_PIPE
try {
    $env:REMOTEAPP_DESKTOP_SESSION_PIPE=$taskBootstrap.pipe
    $taskLines=@((@{Scope=$taskBootstrap.scope;Expected=$taskBootstrap.public_key}|ConvertTo-Json -Compress),'{"Op":"probe"}')
    $taskReplies=@($taskLines | & $taskRelay)
    if ($LASTEXITCODE -ne 0 -or $taskReplies.Count -ne 2) { throw 'Desktop probe failed' }
    $taskHello=$taskReplies[0]|ConvertFrom-Json
    $taskProbe=$taskReplies[1]|ConvertFrom-Json
    if ($taskHello.status -ne 'ready' -or $taskHello.public_key -ne $taskBootstrap.public_key -or $taskProbe.status -ne 'probe_passed') { throw 'Desktop native probe NOT passed' }
    Write-Output 'VALIDATION PASSED: compiled desktop owner + reopened OS signature + exclusive native journal lease. NOT Headscale/product TLS/GUI or signed installer acceptance.'
} finally { $env:REMOTEAPP_DESKTOP_SESSION_PIPE=$taskPrevious }
