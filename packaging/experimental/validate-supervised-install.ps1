param([switch]$ResumeFiles)
$ErrorActionPreference = 'Stop'
# Run in the logged-in desktop (or an Interactive scheduled task). This only
# initializes a fresh isolated TEST root; it does not register an online device.
$setup = Join-Path $PSScriptRoot 'test-setup.exe'
$lines = if ($ResumeFiles) { @(& $setup resume-files) } else { @(& $setup) }
if ($LASTEXITCODE -ne 0 -or ($lines -join "`n" | ConvertFrom-Json).status -ne 'test_installed') { Write-Output ($lines -join "`n"); throw 'TEST setup failed; preserve root and native state' }
$info = New-Object System.Diagnostics.ProcessStartInfo
$info.FileName = Join-Path $PSScriptRoot 'supervisor.exe'
$info.UseShellExecute = $false
$info.RedirectStandardInput = $true
$info.RedirectStandardOutput = $true
$process = New-Object System.Diagnostics.Process
$process.StartInfo = $info
if (-not $process.Start()) { throw 'Supervisor unavailable' }
function Read-Reply {
    $read = $process.StandardOutput.ReadLineAsync()
    if (-not $read.Wait(45000)) { throw 'Reply timeout; do not force close or reset TEST state' }
    if ($null -eq $read.Result) { throw 'Unexpected supervisor EOF' }
    return ($read.Result | ConvertFrom-Json)
}
try {
    $process.StandardInput.WriteLine('{"Op":"open"}')
    $ready = Read-Reply
    if ($ready.status -ne 'ready' -or $ready.control_pipe -notmatch '^\\\\\.\\pipe\\remoteapp-session-[0-9a-f]{32}$') { throw 'Invalid desktop supervisor readiness' }
    $name = $ready.control_pipe.Substring(9)
    $pipe = New-Object System.IO.Pipes.NamedPipeClientStream('.', $name, [System.IO.Pipes.PipeDirection]::InOut)
    try {
        $pipe.Connect(5000)
        $writer = New-Object System.IO.StreamWriter($pipe)
        $writer.AutoFlush = $true
        $reader = New-Object System.IO.StreamReader($pipe)
        $writer.WriteLine('{"Op":"product_count"}')
        $replyTask = $reader.ReadLineAsync()
        if (-not $replyTask.Wait(10000)) { throw 'Business pipe query timeout' }
        if (($replyTask.Result | ConvertFrom-Json).status -ne 'product_unavailable') { throw 'Idle product count did not match' }
    } finally { $pipe.Dispose() }
    $process.StandardInput.WriteLine('{"Op":"stop"}')
    $process.StandardInput.Close()
    if ((Read-Reply).status -ne 'closed') { throw 'Supervisor did not close cleanly' }
    if (-not $process.WaitForExit(45000)) { throw 'Actual exit not joined; preserve process/state' }
    if ($process.ExitCode -ne 0 -or $null -ne $process.StandardOutput.ReadLine()) { throw 'Failed or extra terminal receipt' }
    Write-Output 'VALIDATION PASSED: fresh TEST signed installation + native initialization + actual desktop supervisor/product + business pipe + zero exit. NOT online consent, Headscale or payload acceptance. TEST identity/root retained for explicit maintenance.'
} finally {
    if (-not $process.HasExited) {
        try { $process.StandardInput.WriteLine('{"Op":"stop"}'); $process.StandardInput.Close() } catch {}
        if (-not $process.WaitForExit(45000)) { Write-Output 'JOIN PENDING: no force kill; retain test root and native state.' }
    }
}
