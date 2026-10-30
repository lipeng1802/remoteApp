param([switch]$ResumeFiles, [switch]$StayReady, [switch]$RetainUntilStop)
$ErrorActionPreference = 'Stop'
if ($RetainUntilStop -and -not $StayReady) { throw 'Retained owner requires StayReady' }
# Run in the logged-in desktop (or an Interactive scheduled task). This only
# initializes a fresh isolated TEST root; it does not register an online device.
$publicReady = Join-Path $PSScriptRoot 'owner-ready.json'
$stopRequest = Join-Path $PSScriptRoot 'stop-owner.request'
$faultRequest = Join-Path $PSScriptRoot 'stop-owner-fault.request'
$faultRetry = Join-Path $PSScriptRoot 'retry-owner-fault.request'
$faultReceipt = Join-Path $PSScriptRoot 'owner-fault-joined.json'
$faultModule = Join-Path $PSScriptRoot 'supervised-fault-stop.ps1'
$faultAvailable = Test-Path -LiteralPath $faultModule
if ($faultAvailable) { . $faultModule }
if ($StayReady -and (@($publicReady,$stopRequest,$faultRequest,$faultRetry,$faultReceipt) | Where-Object {Test-Path -LiteralPath $_}).Count) { throw 'Fresh owner metadata required' }
$faultState=$null
$faultLastPhase=''
$faultRetryUsed=$false
function Test-EmptyOwnerRequest {
    param([string]$Path)
    if (-not (Test-Path -LiteralPath $Path)) { return $false }
    $item=Get-Item -LiteralPath $Path
    return -not $item.PSIsContainer -and $item.Length -eq 0 -and -not ($item.Attributes -band [IO.FileAttributes]::ReparsePoint)
}
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
$published = $null
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
    if ($StayReady) {
        # Public discovery only. Invitations/grants never enter this file.
        $public=@{version=1;process_id=$process.Id;control_pipe=$ready.control_pipe;vault_scope=$ready.vault_scope;public_key=$ready.public_key}
        if($RetainUntilStop -and $faultAvailable){$public.fault_protocol=1}
        $published = ($public | ConvertTo-Json -Compress)
        $pendingReady = $publicReady + '.' + $process.Id + '.new'
        $file = [System.IO.File]::Open($pendingReady, [System.IO.FileMode]::CreateNew, [System.IO.FileAccess]::Write, [System.IO.FileShare]::None)
        try {
            $bytes = [System.Text.Encoding]::UTF8.GetBytes($published)
            $file.Write($bytes,0,$bytes.Length)
            $file.Flush($true)
        } finally { $file.Dispose() }
        [System.IO.File]::Move($pendingReady, $publicReady)
        if ($RetainUntilStop) {
            Write-Output 'OWNER_READY: public bootstrap published; retained until explicit management stop. Trust expiry is unchanged.'
        } else {
            Write-Output 'OWNER_READY: public bootstrap published; waiting for explicit management stop, maximum 10 minutes.'
        }
        $deadline = [DateTime]::UtcNow.AddMinutes(10)
        while ($true) {
            if ($null -ne $faultState) {
                # Poll only the SAME ReadLineAsync. No timeout -> finally stop,
                # no discarded late reply, no repeated native status mutation.
                $faultCompleted=$false
                try {
                    if ($faultState.Phase -in @('ready_pending','terminal_pending','exit_pending')) { $null=Receive-SupervisedFaultStop $faultState }
                    if ($faultState.Phase -eq 'rejected' -and -not $faultRetryUsed -and (Test-EmptyOwnerRequest $faultRetry)) {
                        $faultRetryUsed=$true
                        Start-SupervisedFaultStop $faultState -ExplicitRetry
                    }
                    if ($faultState.Phase -ne $faultLastPhase) { $faultLastPhase=$faultState.Phase;Write-Output ('FAULT_STOP_PHASE '+$faultState.Phase) }
                    if ($faultState.Phase -eq 'joined') { Publish-SupervisedFaultStop $faultState $faultReceipt;$faultCompleted=$true }
                } catch { $faultState.Phase='unknown' }
                # Outside the retaining catch: the outer wrapper/task must
                # NOT write EXITCODE=0 for a verified expected native fault.
                if ($faultCompleted) { throw 'EXPECTED_FAULT_JOINED_NORMAL_ZERO_EXIT_NOT_CLAIMED' }
                Start-Sleep -Milliseconds 200
                continue
            }
            if (Test-Path -LiteralPath $faultRequest) {
                $faultState=[pscustomobject]@{Phase='unknown'}
                try {
                    if ($RetainUntilStop -and $faultAvailable -and (Test-EmptyOwnerRequest $faultRequest) -and -not (Test-Path -LiteralPath $stopRequest)) {
                        $faultState=New-SupervisedFaultStopState $process
                        Start-SupervisedFaultStop $faultState
                    }
                } catch { $faultState=[pscustomobject]@{Phase='unknown'} }
                continue
            }
            if ($process.HasExited) { throw 'Desktop supervisor exited before requested stop' }
            if (Test-Path -LiteralPath $stopRequest) {
                $item = Get-Item -LiteralPath $stopRequest
                if (-not $item.PSIsContainer -and $item.Length -eq 0 -and -not ($item.Attributes -band [IO.FileAttributes]::ReparsePoint)) { break }
            }
            if (-not $RetainUntilStop -and [DateTime]::UtcNow -ge $deadline) { break }
            Start-Sleep -Milliseconds 200
        }
    }
    $process.StandardInput.WriteLine('{"Op":"stop"}')
    $process.StandardInput.Close()
    if ((Read-Reply).status -ne 'closed') { throw 'Supervisor did not close cleanly' }
    if (-not $process.WaitForExit(45000)) { throw 'Actual exit not joined; preserve process/state' }
    if ($process.ExitCode -ne 0 -or $null -ne $process.StandardOutput.ReadLine()) { throw 'Failed or extra terminal receipt' }
    Write-Output 'VALIDATION PASSED: fresh TEST signed installation + native initialization + actual desktop supervisor/product + business pipe + zero exit. NOT online consent, Headscale or payload acceptance. TEST identity/root retained for explicit maintenance.'
} finally {
    if ($null -eq $faultState -and -not $process.HasExited) {
        try { $process.StandardInput.WriteLine('{"Op":"stop"}'); $process.StandardInput.Close() } catch {}
        if (-not $process.WaitForExit(45000)) { Write-Output 'JOIN PENDING: no force kill; retain test root and native state.' }
    }
    if ($process.HasExited -and ($null -eq $faultState -or $faultState.Phase -eq 'joined') -and $null -ne $published -and (Test-Path -LiteralPath $publicReady)) {
        # Remove only this process's exact public metadata after actual exit.
        if ([System.IO.File]::ReadAllText($publicReady) -ceq $published) { Remove-Item -LiteralPath $publicReady }
    }
}
