param([Parameter(Mandatory=$true)][string]$IdentityBootstrap)
# Borrows an already READY test identity. Never creates/deletes/stops its owner.
# Stop this window FIRST, then the identity creator window.
$ErrorActionPreference='Stop'
function Write-Control($Process,$Value) {
    $taskBytes=[Text.Encoding]::UTF8.GetBytes(($Value|ConvertTo-Json -Depth 30 -Compress)+"`n")
    $Process.StandardInput.BaseStream.Write($taskBytes,0,$taskBytes.Length);$Process.StandardInput.BaseStream.Flush()
}
$taskManifest=Get-Content -LiteralPath (Join-Path $PSScriptRoot 'SHA256.json') -Raw|ConvertFrom-Json
foreach ($taskName in @('desktop-session.exe','desktop-relay.exe','identity-vault.exe','trust-journal.exe','tls-product.exe','vault-relay.exe','build-public.json','probe-desktop-session.ps1')) {
    $taskExpected=$taskManifest.PSObject.Properties[$taskName].Value
    if ($taskExpected -notmatch '^[0-9a-f]{64}$' -or (Get-FileHash -LiteralPath (Join-Path $PSScriptRoot $taskName) -Algorithm SHA256).Hash.ToLowerInvariant() -ne $taskExpected) { throw "Invalid compiled bundle: $taskName" }
}
$taskPublic=Get-Content -LiteralPath (Join-Path $PSScriptRoot 'build-public.json') -Raw|ConvertFrom-Json
if ($taskPublic.release.test_namespace -ne $true -or $taskPublic.release.channel -notmatch '^desktop-test-[0-9a-f]{32}$') { throw 'Test-only compiled bundle required' }
# Trace data is hash checked; it is not accepted as a runtime root/profile.
$taskHash=[Security.Cryptography.SHA256]::Create()
try { $taskDigest=$taskHash.ComputeHash([Text.Encoding]::UTF8.GetBytes("remoteapp-installed-trust-channel-v1`n"+$taskPublic.release.channel));$taskScope=([BitConverter]::ToString($taskDigest,0,16)).Replace('-','').ToLowerInvariant() } finally { $taskHash.Dispose() }
$taskIdentity=Get-Content -LiteralPath $IdentityBootstrap -Raw|ConvertFrom-Json
if ($taskIdentity.status -ne 'ready' -or $taskIdentity.scope -notmatch '^[0-9a-f]{32}$' -or $taskIdentity.public_key -notmatch '^[0-9a-f]{64}$' -or $taskIdentity.pipe -ne ('\\.\pipe\remoteapp-vault-'+$taskIdentity.scope)) { throw 'Invalid public identity bootstrap' }
# Verify that the actual creator pipe is still READY before borrowing native key.
$taskPrevious=$env:REMOTEAPP_INTERACTIVE_VAULT_PIPE
try {
    $env:REMOTEAPP_INTERACTIVE_VAULT_PIPE=$taskIdentity.pipe
    $taskLines=@((@{Scope=$taskIdentity.scope;Expected=$taskIdentity.public_key;Create=$false}|ConvertTo-Json -Compress),'{"Op":"stop"}')
    $taskReplies=@($taskLines|& (Join-Path $PSScriptRoot 'vault-relay.exe'))
    if ($LASTEXITCODE -ne 0 -or $taskReplies.Count -ne 2) { throw 'Test identity creator not READY' }
    $taskHello=$taskReplies[0]|ConvertFrom-Json;$taskEnd=$taskReplies[1]|ConvertFrom-Json
    if ($taskHello.status -ne 'ready' -or $taskHello.public_key -ne $taskIdentity.public_key -or $taskEnd.status -ne 'closed') { throw 'Test creator identity mismatch' }
} finally { $env:REMOTEAPP_INTERACTIVE_VAULT_PIPE=$taskPrevious }
$taskBootstrap=Join-Path $PSScriptRoot 'desktop-bootstrap.json'
if (Test-Path -LiteralPath $taskBootstrap) { throw 'Existing desktop bootstrap: do not overwrite active owner' }
$taskInfo=New-Object Diagnostics.ProcessStartInfo
$taskInfo.FileName=Join-Path $PSScriptRoot 'desktop-session.exe';$taskInfo.UseShellExecute=$false
$taskInfo.RedirectStandardInput=$true;$taskInfo.RedirectStandardOutput=$true
$taskChild=New-Object Diagnostics.Process;$taskChild.StartInfo=$taskInfo
$taskStarted=$false;$taskCreated=$false;$taskAccepted=$false
try {
    if (!$taskChild.Start()) { throw 'Desktop owner launch failed' };$taskStarted=$true
    Write-Control $taskChild @{Op='initialize';VaultScope=$taskIdentity.scope;VaultExpected=$taskIdentity.public_key}
    $taskRead=$taskChild.StandardOutput.ReadLineAsync()
    if (!$taskRead.Wait(15000)) { throw 'Initialization timeout; shutdown will wait for actual exit' }
    $taskReady=$taskRead.Result|ConvertFrom-Json
    if ($taskReady.status -ne 'ready' -or $taskReady.scope -ne $taskScope -or $taskReady.identity_scope -ne $taskIdentity.scope -or $taskReady.public_key -ne $taskIdentity.public_key -or $taskReady.pipe -ne ('\\.\pipe\remoteapp-session-'+$taskScope) -or $taskReady.channel -ne $taskPublic.release.channel) { throw 'Compiled desktop owner unavailable/mismatched' }
    $taskReady|Add-Member -NotePropertyName relay_executable -NotePropertyValue (Join-Path $PSScriptRoot 'desktop-relay.exe')
    $taskStream=[IO.File]::Open($taskBootstrap,[IO.FileMode]::CreateNew,[IO.FileAccess]::Write,[IO.FileShare]::Read);$taskCreated=$true
    try {$taskBytes=[Text.Encoding]::UTF8.GetBytes(($taskReady|ConvertTo-Json -Compress)+"`n");$taskStream.Write($taskBytes,0,$taskBytes.Length)} finally {$taskStream.Dispose()}
    Write-Output "READY: $taskBootstrap (public metadata only). Keep BOTH desktop windows open."
    [void](Read-Host 'Enter only after coordinated verification; stop this borrower BEFORE the identity creator')
    $taskAccepted=$true
} finally {
    $taskClean=$false
    if ($taskStarted) {
        if (!$taskChild.HasExited) {try {Write-Control $taskChild @{Op='stop'}} catch {}; $taskChild.StandardInput.Close();Write-Output 'Waiting for actual desktop owner exit; do not force-close.';$taskChild.WaitForExit()}
        $taskFinal=$taskChild.StandardOutput.ReadToEnd().Trim()
        try {$taskClean=$taskChild.ExitCode -eq 0 -and ($taskFinal|ConvertFrom-Json).status -eq 'closed'} catch {}
    }
    $taskChild.Dispose()
    if ($taskAccepted -and $taskClean) {if ($taskCreated) {Remove-Item -LiteralPath $taskBootstrap};Write-Output 'STOPPED: borrowed desktop owner joined and its isolated test journal cleaned. Identity creator remains READY; not deleted.'}
    else {Write-Warning 'Cleanup NOT confirmed; public bootstrap/partial test journal preserved. Borrowed identity was not deleted.'}
}
