param([Parameter(Mandatory=$true)][string]$Executable,
      [Parameter(Mandatory=$true)][string]$SHA256,
      [Parameter(Mandatory=$true)][string]$OutputPath)
$ErrorActionPreference='Stop'
[System.Diagnostics.Process]::GetCurrentProcess().PriorityClass='BelowNormal'
[System.Diagnostics.Process]::GetCurrentProcess().ProcessorAffinity=[IntPtr]3
if(Test-Path -LiteralPath $OutputPath){throw 'existing component receipt'}
$report=[ordered]@{test='owned_target_preparation_only';sha256=$SHA256;runs=@()}
function Save-Report { $report | ConvertTo-Json -Depth 6 | Set-Content -LiteralPath $OutputPath -Encoding UTF8 }
function Send-Frame($Process,$Value) {
    $Process.StandardInput.WriteLine(($Value | ConvertTo-Json -Compress));$Process.StandardInput.Flush()
}
function Read-Frame($Process) {
    $bytes=New-Object System.Collections.Generic.List[byte]
    $buffer=New-Object byte[] 1
    while($bytes.Count -lt 8192){
        $read=$Process.StandardOutput.BaseStream.ReadAsync($buffer,0,1)
        if(-not $read.Wait(8000)){throw 'component frame timeout'}
        if($read.Result -eq 0){throw 'component frame EOF'}
        if($buffer[0] -eq 10){return ([System.Text.Encoding]::UTF8.GetString($bytes.ToArray()) | ConvertFrom-Json)}
        $bytes.Add($buffer[0])
    }
    throw 'component oversized frame'
}
function Require-Fields($Value,[string[]]$Names) {
    if((@($Value.PSObject.Properties.Name | Sort-Object) -join ',') -cne (@($Names | Sort-Object) -join ',')){throw 'component receipt shape'}
}
foreach($mode in @('normal_view','normal_control','close_view','malformed_view')){
    if((Get-FileHash -LiteralPath $Executable -Algorithm SHA256).Hash.ToLowerInvariant() -cne $SHA256){throw 'component executable changed'}
    $permission='desktop-view';if($mode -eq 'normal_control'){$permission='desktop-control'}
    $info=New-Object System.Diagnostics.ProcessStartInfo
    $info.FileName=$Executable;$info.UseShellExecute=$false;$info.CreateNoWindow=$true
    $info.RedirectStandardInput=$true;$info.RedirectStandardOutput=$true;$info.RedirectStandardError=$true
    $process=New-Object System.Diagnostics.Process;$process.StartInfo=$info
    if(-not $process.Start()){throw 'component process failed'}
    $process.StandardInput.NewLine="`n" # exact inherited Go pipe framing, not Windows CRLF
    $errorReader=$process.StandardError.ReadToEndAsync() # drain, never print private content
    $run=[ordered]@{mode=$mode;pid=$process.Id;passed=$false};$report.runs+=,$run
    $stopped=$false
    $step='prepare'
    try{
        Send-Frame $process ([ordered]@{Op='prepare';Permission=$permission;Role='target';Self=('a'*64)})
        $ready=Read-Frame $process;Require-Fields $ready @('status','Permission','Role')
        if($ready.status -cne 'desktop_prepared' -or $ready.Permission -cne $permission -or $ready.Role -cne 'target'){throw 'component preparation'}
        $step='idle_and_endpoints'
        if(-not $process.WaitForInputIdle(8000)){throw 'component window not idle'}
        if(@(Get-NetTCPConnection -OwningProcess $process.Id -ErrorAction SilentlyContinue).Count -ne 0 -or
           @(Get-NetUDPEndpoint -OwningProcess $process.Id -ErrorAction SilentlyContinue).Count -ne 0){throw 'component unexpected endpoint'}
        $run.no_network_endpoints=$true
        $step='local_stop_or_fault'
        if($mode -eq 'close_view'){
            $process.Refresh()
            if(-not $process.CloseMainWindow()){throw 'component close intent unavailable'}
            Start-Sleep -Milliseconds 300
        }
        if($mode -eq 'malformed_view'){
            # Open a real pending dialog; never click Allow. A duplicate request
            # must fault once without recursive refusal or a fabricated decision.
            $question=[ordered]@{Controller=('b'*64);Op='consent';RequestID=('c'*32)}
            Send-Frame $process $question;Send-Frame $process $question
            $fault=Read-Frame $process;Require-Fields $fault @('status')
            if($fault.status -cne 'failed'){throw 'component pending fault receipt'}
        }
        Send-Frame $process ([ordered]@{Op='desktop_state'})
        $step='state'
        $state=Read-Frame $process
        Require-Fields $state @('status','frames','inputs','releases','held','stop_requested','ended','failed')
        $stopping=$mode -in @('close_view','malformed_view')
        if($state.status -cne 'desktop_state' -or $state.frames -ne 0 -or $state.inputs -ne 0 -or
           $state.releases -ne 0 -or $state.held -ne 0 -or $state.ended -ne $false -or
           $state.stop_requested -ne $stopping -or $state.failed -ne ($mode -eq 'malformed_view') -or $process.HasExited){throw 'component unsafe unactivated state'}
        $run.stop_intent_retained=($mode -eq 'close_view')
        $stopped=$true # even an unknown write is not retried
        $step='owner_stop_join'
        Send-Frame $process ([ordered]@{Op='stop'});$process.StandardInput.Close()
        $closed=Read-Frame $process;Require-Fields $closed @('status')
        if($closed.status -cne 'closed'){throw 'component closed receipt'}
        if(-not $process.WaitForExit(10000)){throw 'component exit timeout'}
        if(-not $errorReader.Wait(1000)){throw 'component stderr reader not joined'}
        $expected=0;if($mode -eq 'malformed_view'){$expected=1}
        if($process.ExitCode -ne $expected -or $process.StandardOutput.BaseStream.ReadByte() -ne -1){throw 'component exit or EOF mismatch'}
        $run.passed=$true;$run.exit_code=$process.ExitCode;$run.stdout_eof=$true
    }catch{$run.failure='component validation failed';$run.stage=$step}
    finally{
        if(-not $stopped){
            $stopped=$true
            try{Send-Frame $process ([ordered]@{Op='stop'});$process.StandardInput.Close()}catch{}
        }
        if(-not $process.HasExited -and -not $process.WaitForExit(15000)){
            $run.join_pending=$true;Save-Report
            # No kill, task timeout, retry or replacement child. Keep this same
            # driver/owner until the original process actually ends.
            $process.WaitForExit()
        }
        $run.exit_code=$process.ExitCode;Save-Report
        $process.Dispose()
    }
    if(-not $run.passed){exit 1}
}
exit 0
