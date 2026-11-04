# Trusted inherited-pipe TEST coordination. No identity/root/argv selectors.
function New-SupervisedRemovalState {
    param($Process)
    [pscustomobject]@{Process=$Process;Phase='armed';Attempted=$false;Read=$null;Operation='';Receipt=$null}
}
function Start-SupervisedRemoval {
    param($State)
    if($State.Attempted -or $State.Phase -ne 'armed'){throw 'Removal already attempted'}
    $State.Attempted=$true;$State.Phase='pending';$State.Operation='begin'
    try {
        $State.Process.StandardInput.WriteLine('{"Op":"remove_program"}')
        $State.Read=$State.Process.StandardOutput.ReadLineAsync()
    } catch { $State.Phase='unknown';throw 'Removal write outcome unknown' }
}
function Receive-SupervisedRemoval {
    param($State)
    if($State.Phase -ne 'pending'){return}
    if($null -ne $State.Read){
        if(-not $State.Read.IsCompleted){return}
        try {$raw=$State.Read.GetAwaiter().GetResult()}catch{$State.Phase='unknown';return}
        $State.Read=$null
        if($State.Operation -eq 'begin'){
            if($raw -cne '{"status":"removal_pending"}'){$State.Phase='unknown';return}
        } elseif($raw -ceq '{"status":"removal_pending"}'){
        } elseif($raw -cmatch '^\{"status":"files_removed","epoch":[1-9][0-9]{0,15},"transaction":"[0-9a-f]{32}","preserved":true,"joined":true,"zero_exit":true\}$'){
            $v=$raw | ConvertFrom-Json
            if($v.epoch -gt 9007199254740991){$State.Phase='unknown';return}
            $State.Receipt=$raw;$State.Phase='files_removed';return
        } else {$State.Phase='unknown';return}
    }
    # Read-only cache query; a pending ReadLineAsync is NEVER replaced/reissued.
    try {
        $State.Process.StandardInput.WriteLine('{"Op":"program_removal_status"}')
        $State.Operation='status';$State.Read=$State.Process.StandardOutput.ReadLineAsync()
    } catch {$State.Phase='unknown'}
}
function Publish-SupervisedRemoval {
    param($State,[string]$Path)
    if($State.Phase -ne 'files_removed' -or $null -eq $State.Receipt){throw 'Removal not joined'}
    $pending=$Path+'.'+$State.Process.Id+'.new'
    $f=[IO.File]::Open($pending,[IO.FileMode]::CreateNew,[IO.FileAccess]::Write,[IO.FileShare]::None)
    try {$b=[Text.Encoding]::UTF8.GetBytes($State.Receipt);$f.Write($b,0,$b.Length);$f.Flush($true)}finally{$f.Dispose()}
    [IO.File]::Move($pending,$Path)
}
