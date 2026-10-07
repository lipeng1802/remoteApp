# Keep this window open. Only a random test credential is created, never the
# installed application's key. Bootstrap contains public fields, no secret.
$ErrorActionPreference = 'Stop'
$taskFolder = $PSScriptRoot
$taskManifest = Get-Content -LiteralPath (Join-Path $taskFolder 'SHA256.json') -Raw | ConvertFrom-Json
foreach ($taskName in @('identity-vault.exe', 'interactive-vault.exe', 'vault-relay.exe')) {
    $taskPath = Join-Path $taskFolder $taskName
    $taskExpected = $taskManifest.PSObject.Properties[$taskName].Value
    if ($taskExpected -notmatch '^[0-9a-f]{64}$' -or !(Test-Path -LiteralPath $taskPath -PathType Leaf)) { throw 'Invalid bundle' }
    if ((Get-FileHash -LiteralPath $taskPath -Algorithm SHA256).Hash.ToLowerInvariant() -ne $taskExpected) { throw "Checksum mismatch: $taskName" }
}
$taskBootstrap = Join-Path $taskFolder 'interactive-bootstrap.json'
if (Test-Path -LiteralPath $taskBootstrap) { throw 'Existing bootstrap: stop its owner first; do not overwrite an active run.' }
$taskInfo = New-Object System.Diagnostics.ProcessStartInfo
$taskInfo.FileName = Join-Path $taskFolder 'interactive-vault.exe'
$taskInfo.UseShellExecute = $false
$taskInfo.RedirectStandardInput = $true
$taskInfo.RedirectStandardOutput = $true
$taskProcess = New-Object System.Diagnostics.Process
$taskProcess.StartInfo = $taskInfo
$taskCreatedBootstrap = $false
try {
    if (!$taskProcess.Start()) { throw 'Broker launch failed' }
    $taskProcess.StandardInput.WriteLine((@{VaultExecutable=(Join-Path $taskFolder 'identity-vault.exe')} | ConvertTo-Json -Compress))
    $taskRead = $taskProcess.StandardOutput.ReadLineAsync()
    if (!$taskRead.Wait(10000)) { throw 'Broker startup timeout' }
    $taskReady = $taskRead.Result | ConvertFrom-Json
    if ($taskReady.status -ne 'ready' -or $taskReady.scope -notmatch '^[0-9a-f]{32}$' -or $taskReady.public_key -notmatch '^[0-9a-f]{64}$') { throw 'Interactive vault unavailable' }
    $taskReady | Add-Member -NotePropertyName relay_executable -NotePropertyValue (Join-Path $taskFolder 'vault-relay.exe')
    # CreateNew rejects a competing launcher, rather than replacing its file.
    $taskStream = [System.IO.File]::Open($taskBootstrap, [System.IO.FileMode]::CreateNew, [System.IO.FileAccess]::Write, [System.IO.FileShare]::Read)
    $taskCreatedBootstrap = $true
    try {
        $taskBytes = [System.Text.Encoding]::UTF8.GetBytes(($taskReady | ConvertTo-Json -Compress) + "`n")
        $taskStream.Write($taskBytes, 0, $taskBytes.Length)
    } finally { $taskStream.Dispose() }
    Write-Output "READY: $taskBootstrap (public identity only). Keep this desktop window open."
    [void](Read-Host 'Press Enter only when coordinated verification is finished, to stop and clean this test identity')
} finally {
    if ($taskProcess.Id -and !$taskProcess.HasExited) {
        $taskProcess.StandardInput.WriteLine('{"Op":"stop"}')
        $taskProcess.StandardInput.Close()
        if (!$taskProcess.WaitForExit(12000)) {
            $taskProcess.Kill()
            throw 'Broker forced termination; random test credential cleanup unconfirmed. Do not delete broad credential namespaces.'
        }
        if ($taskProcess.ExitCode -ne 0) { throw 'Broker failed; random test credential cleanup unconfirmed' }
        Write-Output 'STOPPED: workers joined and random test identity deleted.'
    }
    if ($taskCreatedBootstrap) { Remove-Item -LiteralPath $taskBootstrap }
    $taskProcess.Dispose()
}
