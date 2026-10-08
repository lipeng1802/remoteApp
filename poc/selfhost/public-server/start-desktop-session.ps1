# Fresh isolated TEST journal + random TEST device key. No production trust,
# Tailscale installation or existing application credentials are changed.
$ErrorActionPreference = 'Stop'
function Write-Control($Process, $Value) {
    $taskBytes = [Text.Encoding]::UTF8.GetBytes(($Value | ConvertTo-Json -Depth 30 -Compress) + "`n")
    $Process.StandardInput.BaseStream.Write($taskBytes, 0, $taskBytes.Length)
    $Process.StandardInput.BaseStream.Flush()
}
function Launch-Control($Name, $Value) {
    $taskInfo = New-Object Diagnostics.ProcessStartInfo
    $taskInfo.FileName = Join-Path $PSScriptRoot $Name
    $taskInfo.UseShellExecute = $false
    $taskInfo.RedirectStandardInput = $true
    $taskInfo.RedirectStandardOutput = $true
    $taskProcess = New-Object Diagnostics.Process
    $taskProcess.StartInfo = $taskInfo
    if (!$taskProcess.Start()) { throw 'Desktop child launch failed' }
    # Keep ownership even if startup fails; finally still joins this child.
    $script:taskChildren.Add($taskProcess)
    Write-Control $taskProcess $Value
    $taskRead = $taskProcess.StandardOutput.ReadLineAsync()
    if (!$taskRead.Wait(15000)) { throw 'Desktop initialization timed out; shutdown will wait for actual join, without force killing.' }
    $taskReply = $taskRead.Result | ConvertFrom-Json
    if ($taskReply.status -ne 'ready' -or $taskReply.scope -notmatch '^[0-9a-f]{32}$' -or $taskReply.public_key -notmatch '^[0-9a-f]{64}$') { throw 'Desktop initialization failed. No plaintext fallback; partial test journal preserved.' }
    return @{Process=$taskProcess; Reply=$taskReply}
}
function Write-PublicBootstrap($Path, $Value) {
    $taskStream = [IO.File]::Open($Path, [IO.FileMode]::CreateNew, [IO.FileAccess]::Write, [IO.FileShare]::Read)
    $script:taskCreatedFiles.Add($Path)
    try { $taskBytes=[Text.Encoding]::UTF8.GetBytes(($Value | ConvertTo-Json -Depth 10 -Compress)+"`n"); $taskStream.Write($taskBytes,0,$taskBytes.Length) } finally { $taskStream.Dispose() }
}
$taskManifest = Get-Content -LiteralPath (Join-Path $PSScriptRoot 'SHA256.json') -Raw | ConvertFrom-Json
foreach ($taskName in @('identity-vault.exe','trust-journal.exe','tls-product.exe','desktop-session.exe','desktop-relay.exe','interactive-vault.exe','vault-relay.exe','probe-desktop-session.ps1')) {
    $taskExpected = $taskManifest.PSObject.Properties[$taskName].Value
    if ($taskExpected -notmatch '^[0-9a-f]{64}$' -or (Get-FileHash -LiteralPath (Join-Path $PSScriptRoot $taskName) -Algorithm SHA256).Hash.ToLowerInvariant() -ne $taskExpected) { throw "Invalid bundle: $taskName" }
}
$taskVaultFile=Join-Path $PSScriptRoot 'interactive-bootstrap.json'
$taskSessionFile=Join-Path $PSScriptRoot 'desktop-bootstrap.json'
if ((Test-Path -LiteralPath $taskVaultFile) -or (Test-Path -LiteralPath $taskSessionFile)) { throw 'Existing bootstrap: do not overwrite or run a second owner.' }
$script:taskChildren=New-Object 'Collections.Generic.List[Diagnostics.Process]'
$script:taskCreatedFiles=New-Object 'Collections.Generic.List[string]'
$taskSuccess=$false
try {
    $taskVault=Launch-Control 'interactive-vault.exe' @{VaultExecutable=(Join-Path $PSScriptRoot 'identity-vault.exe')}
    $taskVault.Reply | Add-Member -NotePropertyName relay_executable -NotePropertyValue (Join-Path $PSScriptRoot 'vault-relay.exe')
    $taskSession=Launch-Control 'desktop-session.exe' @{Op='initialize';VaultScope=$taskVault.Reply.scope;VaultExpected=$taskVault.Reply.public_key}
    if ($taskSession.Reply.identity_scope -ne $taskVault.Reply.scope -or $taskSession.Reply.public_key -ne $taskVault.Reply.public_key -or $taskSession.Reply.channel -notmatch '^desktop-test-[0-9a-f]{32}$' -or $taskSession.Reply.pipe -ne ('\\.\pipe\remoteapp-session-'+$taskSession.Reply.scope)) { throw 'Desktop instance mismatch' }
    $taskSession.Reply | Add-Member -NotePropertyName relay_executable -NotePropertyValue (Join-Path $PSScriptRoot 'desktop-relay.exe')
    Write-PublicBootstrap $taskVaultFile $taskVault.Reply
    Write-PublicBootstrap $taskSessionFile $taskSession.Reply
    Write-Output "READY: $taskSessionFile (public metadata only). Keep this desktop window open."
    Write-Output 'In another desktop PowerShell window run probe-desktop-session.ps1. This is NOT cross-machine product acceptance.'
    [void](Read-Host 'Press Enter only after verification to stop, join and clean test resources')
    $taskSuccess=$true
} finally {
    $taskClean=$true
    # Desktop owner FIRST. A blocking join deliberately keeps the creator/key
    # alive. Never kill to turn a timeout into a successful cleanup claim.
    for ($taskIndex=$script:taskChildren.Count-1; $taskIndex -ge 0; $taskIndex--) {
        $taskChild=$script:taskChildren[$taskIndex]
        if (!$taskChild.HasExited) {
            try { Write-Control $taskChild @{Op='stop'} } catch { $taskClean=$false }
            $taskChild.StandardInput.Close()
            Write-Output 'Waiting for actual child exit; do not force-close this window.'
            $taskChild.WaitForExit()
        }
        if ($taskChild.ExitCode -ne 0) { $taskClean=$false }
        $taskFinal=$taskChild.StandardOutput.ReadToEnd().Trim()
        try { if (($taskFinal | ConvertFrom-Json).status -ne 'closed') { $taskClean=$false } } catch { $taskClean=$false }
        $taskChild.Dispose()
    }
    if ($taskSuccess -and $taskClean) {
        foreach ($taskFile in $script:taskCreatedFiles) { Remove-Item -LiteralPath $taskFile }
        Write-Output 'STOPPED: desktop workers and native signer joined; isolated test journal and random test identity cleaned.'
    } else { Write-Warning 'Acceptance/cleanup NOT confirmed. Public bootstrap and any partial test journal preserved for diagnosis. Do not delete broad credential namespaces.' }
}
