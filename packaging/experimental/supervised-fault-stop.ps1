# Trusted TEST lifecycle transport, not a publisher/identity authentication API.
function New-SupervisedFaultStopState {
    param([System.Diagnostics.Process]$Process)
    return [pscustomobject]@{ Process=$Process; Phase='new'; PendingRead=$null }
}
function Start-SupervisedFaultStop {
    param($State, [switch]$ExplicitRetry)
    if ($State.Phase -ne 'new' -and -not ($ExplicitRetry -and $State.Phase -eq 'rejected')) { throw 'Fault lifecycle already pending or sealed' }
    # Persist in the resident parent's state BEFORE any write. No EOF/fallback
    # stop on an unknown write or read; the signer may still be needed.
    $State.Phase='ready_pending'
    try {
        $State.Process.StandardInput.WriteLine('{"Op":"stop_fault"}')
        $State.PendingRead=$State.Process.StandardOutput.ReadLineAsync()
    } catch { $State.Phase='unknown' }
}
function Test-SupervisedFaultFrame {
    param([string]$Line, [ValidateSet('fault_stop_ready','fault_closed')][string]$Status)
    # Go's encoder sorts these fixed map keys. Exact bytes reject duplicate
    # fields, truthy integers and extra data without ConvertFrom-Json's merging.
    $expected='{"fault":true,"joined":true,"status":"'+$Status+'"}'
    return [string]::Equals($Line,$expected,[StringComparison]::Ordinal)
}
function Receive-SupervisedFaultStop {
    param($State)
    try {
        if ($State.Phase -notin @('ready_pending','terminal_pending','exit_pending')) { return $State.Phase }
        if (-not $State.PendingRead.IsCompleted) { return $State.Phase }
        $line=$State.PendingRead.GetAwaiter().GetResult()
        if ($State.Phase -eq 'ready_pending') {
            if ([string]::Equals($line,'{"status":"fault_stop_rejected"}',[StringComparison]::Ordinal)) {
                $State.PendingRead=$null; $State.Phase='rejected'; return $State.Phase
            }
            if (-not (Test-SupervisedFaultFrame $line 'fault_stop_ready')) { $State.Phase='unknown'; return $State.Phase }
            $State.Phase='terminal_pending'
            $State.Process.StandardInput.Close() # only after native join/revoke ACK
            $State.PendingRead=$State.Process.StandardOutput.ReadLineAsync()
        } elseif ($State.Phase -eq 'terminal_pending') {
            if (-not (Test-SupervisedFaultFrame $line 'fault_closed')) { $State.Phase='unknown'; return $State.Phase }
            $State.Phase='exit_pending'
            $State.PendingRead=$State.Process.StandardOutput.ReadLineAsync()
        } else {
            if ($null -ne $line) { $State.Phase='unknown'; return $State.Phase }
            if (-not $State.Process.HasExited) { return $State.Phase }
            $State.Process.WaitForExit()
            if ($State.Process.ExitCode -ne 1) { $State.Phase='unknown'; return $State.Phase }
            $State.Phase='joined'
        }
    } catch { $State.Phase='unknown' }
    return $State.Phase
}
function Publish-SupervisedFaultStop {
    param($State,[string]$Path)
    if ($State.Phase -ne 'joined' -or -not $State.Process.HasExited -or $State.Process.ExitCode -ne 1) { throw 'Fault join not verified' }
    $receipt=[ordered]@{version=1;status='owner_fault_closed';joined=$true;fault=$true;exit_code=1;process_id=$State.Process.Id}
    $bytes=[Text.Encoding]::UTF8.GetBytes(($receipt | ConvertTo-Json -Compress))
    $pending=$Path+'.'+$State.Process.Id+'.new'
    $file=[IO.File]::Open($pending,[IO.FileMode]::CreateNew,[IO.FileAccess]::Write,[IO.FileShare]::None)
    try { $file.Write($bytes,0,$bytes.Length);$file.Flush($true) } finally { $file.Dispose() }
    [IO.File]::Move($pending,$Path)
}
