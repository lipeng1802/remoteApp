param([Parameter(Mandatory=$true)][string]$Fixture)
$ErrorActionPreference='Stop'
function Assert-True {param([bool]$Value,[string]$Label)if(-not $Value){throw $Label}}
function Await-File {param([string]$Path) $until=[DateTime]::UtcNow.AddSeconds(10);while(-not(Test-Path $Path)){if([DateTime]::UtcNow -ge $until){throw 'Fixture file timeout'};Start-Sleep -Milliseconds 50}}
foreach($mode in @('normal','fault','rejected-retry')){
    $d=Join-Path $PSScriptRoot ('retained-'+$mode)
    if(Test-Path $d){throw 'Fresh fixture root required'}
    $null=New-Item -ItemType Directory -Path $d
    Copy-Item $Fixture (Join-Path $d 'test-setup.exe')
    Copy-Item $Fixture (Join-Path $d 'supervisor.exe')
    foreach($name in @('validate-supervised-install.ps1','supervised-fault-stop.ps1')){Copy-Item (Join-Path $PSScriptRoot $name) (Join-Path $d $name)}
    $wrapper=@'
$ErrorActionPreference='Stop'
$log=Join-Path $PSScriptRoot 'result.txt'
try { & (Join-Path $PSScriptRoot 'validate-supervised-install.ps1') -StayReady -RetainUntilStop *>&1 | Out-File $log -Encoding utf8
 'EXITCODE=0' | Out-File $log -Append -Encoding utf8
} catch { 'EXITCODE=1' | Out-File $log -Append -Encoding utf8;exit 1 }
'@
    [IO.File]::WriteAllText((Join-Path $d 'run-holder.ps1'),$wrapper)
    if($mode -ne 'rejected-retry'){$null=New-Item -ItemType File -Path (Join-Path $d 'fixture-authorize.request')}
    $info=New-Object System.Diagnostics.ProcessStartInfo
    $info.FileName='powershell.exe';$info.Arguments='-NoProfile -ExecutionPolicy Bypass -File '+[char]34+(Join-Path $d 'run-holder.ps1')+[char]34
    $info.UseShellExecute=$false
    $parent=New-Object System.Diagnostics.Process;$parent.StartInfo=$info
    if(-not $parent.Start()){throw 'Fixture holder unavailable'}
    $ready=Join-Path $d 'owner-ready.json';Await-File $ready
    $boot=Get-Content $ready -Raw | ConvertFrom-Json
    Assert-True ($boot.fault_protocol -eq 1) 'Actual holder did not advertise fault protocol'
    if($mode -eq 'normal'){$null=New-Item -ItemType File -Path (Join-Path $d 'stop-owner.request')}else{$null=New-Item -ItemType File -Path (Join-Path $d 'stop-owner-fault.request')}
    if($mode -eq 'rejected-retry'){
        $until=[DateTime]::UtcNow.AddSeconds(10)
        do{$text=Get-Content (Join-Path $d 'result.txt') -Raw -ErrorAction SilentlyContinue;if($text -match 'FAULT_STOP_PHASE rejected'){break};Start-Sleep -Milliseconds 50}while([DateTime]::UtcNow -lt $until)
        Assert-True ($text -match 'FAULT_STOP_PHASE rejected') 'Holder did not retain rejected request'
        Start-Sleep -Milliseconds 400
        Assert-True (-not $parent.HasExited -and (Test-Path $ready) -and -not(Test-Path (Join-Path $d 'owner-fault-joined.json'))) 'Rejected holder lost ownership'
        Assert-True (@(Get-Content (Join-Path $d 'fixture-ops.txt') | Where-Object {$_ -eq 'stop_fault'}).Count -eq 1) 'Holder automatically retried native fault'
        $null=New-Item -ItemType File -Path (Join-Path $d 'fixture-authorize.request')
        $null=New-Item -ItemType File -Path (Join-Path $d 'retry-owner-fault.request')
    }
    if(-not $parent.WaitForExit(15000)){throw 'Fixture holder actual exit not joined; no force kill'}
    $text=Get-Content (Join-Path $d 'result.txt') -Raw
    Assert-True (-not(Test-Path $ready)) 'Actual holder left discovery after known exit'
    if($mode -eq 'normal'){
        Assert-True ($parent.ExitCode -eq 0 -and $text -match 'EXITCODE=0' -and -not(Test-Path (Join-Path $d 'owner-fault-joined.json'))) 'Normal path changed'
    }else{
        $receipt=Get-Content (Join-Path $d 'owner-fault-joined.json') -Raw | ConvertFrom-Json
        Assert-True ($parent.ExitCode -eq 1 -and $text -match 'EXITCODE=1' -and $text -notmatch 'EXITCODE=0') 'Fault became normal zero wrapper exit'
        Assert-True ($receipt.process_id -eq $boot.process_id -and $receipt.joined -is [bool] -and $receipt.joined -and $receipt.fault -is [bool] -and $receipt.fault -and $receipt.exit_code -eq 1) 'Actual holder fault proof invalid'
    }
    $parent.Dispose();Write-Output ('PASS retained-holder-'+$mode)
}
Write-Output 'VALIDATION PASSED: actual retained script + same-user pipe + two-phase fault/no zero wrapper. NOT native credentials, Headscale or product acceptance.'
