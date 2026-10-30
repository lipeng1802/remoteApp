param([Parameter(Mandatory=$true)][string]$Fixture)
$ErrorActionPreference='Stop'
. (Join-Path $PSScriptRoot 'supervised-fault-stop.ps1')
function Assert-True { param([bool]$Value,[string]$Label) if(-not $Value){throw $Label} }
function Await-FaultPhase {
    param($State)
    $deadline=[DateTime]::UtcNow.AddSeconds(6)
    do {
        $phase=Receive-SupervisedFaultStop $State
        if($phase -in @('joined','rejected','unknown')){return $phase}
        Start-Sleep -Milliseconds 20
    } while([DateTime]::UtcNow -lt $deadline)
    throw 'Fixture phase timeout; not joined'
}
foreach($mode in @('valid','late-ready','late-final','rejected','integer','extra','duplicate','zero','failed')) {
    $log=Join-Path $PSScriptRoot ('ops-'+$mode+'.txt')
    $receipt=Join-Path $PSScriptRoot ('receipt-'+$mode+'.json')
    if((Test-Path $log) -or (Test-Path $receipt)){throw 'Fresh fixture output paths required'}
    $info=New-Object System.Diagnostics.ProcessStartInfo
    $info.FileName=$Fixture
    $info.Arguments='-test.run=^TestWindowsFaultHolderFixture$ r81-holder-'+$mode+' '+[char]34+$log+[char]34
    $info.UseShellExecute=$false;$info.RedirectStandardInput=$true;$info.RedirectStandardOutput=$true
    $process=New-Object System.Diagnostics.Process;$process.StartInfo=$info
    if(-not $process.Start()){throw 'Fixture unavailable'}
    try {
        Assert-True ($process.StandardOutput.ReadLine() -ceq '{"status":"ready"}') 'Fixture readiness'
        $state=New-SupervisedFaultStopState $process
        Start-SupervisedFaultStop $state
        if($mode -eq 'late-ready'){
            Assert-True ($state.Phase -eq 'ready_pending' -and $process.StandardInput.BaseStream.CanWrite) 'Unknown ready closed input'
        }
        $phase=Await-FaultPhase $state
        if($mode -eq 'rejected'){
            Assert-True ($phase -eq 'rejected' -and -not $process.HasExited -and $process.StandardInput.BaseStream.CanWrite) 'Rejected confirmation dropped owner'
            1..3 | ForEach-Object { $null=Receive-SupervisedFaultStop $state }
            Assert-True (@(Get-Content $log | Where-Object {$_ -eq 'stop_fault'}).Count -eq 1) 'Implicit fault request retry'
            $process.StandardInput.WriteLine('{"Op":"fixture_authorize"}')
            Assert-True ($process.StandardOutput.ReadLine() -ceq '{"status":"fixture_authorized"}') 'Fixture authorization'
            Start-SupervisedFaultStop $state -ExplicitRetry
            $phase=Await-FaultPhase $state
        }
        if($mode -in @('valid','late-ready','late-final','rejected')){
            Assert-True ($phase -eq 'joined' -and $process.ExitCode -eq 1) 'Fault proof missing actual failed exit'
            Publish-SupervisedFaultStop $state $receipt
            $value=Get-Content $receipt -Raw | ConvertFrom-Json
            Assert-True ($value.joined -is [bool] -and $value.joined -and $value.fault -is [bool] -and $value.fault -and $value.exit_code -eq 1 -and $value.process_id -eq $process.Id) 'Bad public fault receipt'
        } else {
            Assert-True ($phase -eq 'unknown') 'Malformed fault proof accepted'
            if($mode -eq 'integer'){Assert-True $process.StandardInput.BaseStream.CanWrite 'Invalid ready closed input'}
            $rejected=$false
            try { Publish-SupervisedFaultStop $state $receipt } catch { $rejected=$true }
            Assert-True ($rejected -and -not(Test-Path $receipt)) 'Unknown state published join'
        }
        $count=@(Get-Content $log | Where-Object {$_ -eq 'stop_fault'}).Count
        Assert-True ($count -eq $(if($mode -eq 'rejected'){2}else{1})) 'Late receipt caused resend'
        Assert-True (@(Get-Content $log | Where-Object {$_ -eq 'stop'}).Count -eq 0) 'Fault fell back to normal stop'
        Write-Output ('PASS fault-holder-'+$mode)
    } finally {
        # Fixture-only cleanup; no keys, SDK or native state are involved.
        try {$process.StandardInput.Close()}catch{}
        if(-not $process.WaitForExit(5000)){throw 'Fixture exit not joined; no force kill'}
        $process.StandardOutput.Close();$process.Dispose()
    }
}
$tokens=$null;$errors=$null
$null=[Management.Automation.Language.Parser]::ParseFile((Join-Path $PSScriptRoot 'validate-supervised-install.ps1'),[ref]$tokens,[ref]$errors)
Assert-True ($errors.Count -eq 0) 'Actual retained holder AST errors'
Write-Output 'VALIDATION PASSED: real Windows process two-phase fault holder transport. NOT OS signer, Headscale or product acceptance.'
