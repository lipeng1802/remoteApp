param([Parameter(Mandatory=$true)][string]$Directory,
      [Parameter(Mandatory=$true)][string]$CacheDirectory,
      [Parameter(Mandatory=$true)][string]$ExpectedHelper,
      [Parameter(Mandatory=$true)][string]$ExpectedPayload)
$ErrorActionPreference='Stop'
$Directory=[IO.Path]::GetFullPath($Directory)
$CacheDirectory=[IO.Path]::GetFullPath($CacheDirectory)
$wire=Join-Path $Directory '_transport'
$exe=Join-Path $wire 'restore.exe'
if((Get-FileHash -LiteralPath $exe).Hash.ToLowerInvariant() -ne $ExpectedHelper){throw 'Helper mismatch'}
function Invoke-Restore([string]$cache,[string]$output,[string]$transport,[bool]$passed){
    $lines=@(& $exe $cache $output $transport);$code=$LASTEXITCODE
    $want=if($passed){'ARCHIVE_CACHE_RESTORED'}else{'ARCHIVE_CACHE_REJECTED'}
    if(($lines -join "`n") -ne $want -or ($passed -and $code -ne 0) -or (-not $passed -and $code -ne 1)){throw 'Restore result mismatch'}
}
Invoke-Restore $CacheDirectory $Directory $wire $true
$payload=Join-Path $Directory 'unconfigured-product.exe'
if((Get-FileHash -LiteralPath $payload).Hash.ToLowerInvariant() -ne $ExpectedPayload){throw 'Payload mismatch'}
# The reconstructed product is opaque byte data. Never start it.
$plan=Get-Content -LiteralPath (Join-Path $wire 'plan.json') -Raw | ConvertFrom-Json
foreach($mode in @('literal','cache')){
    $output=Join-Path $Directory ('negative-'+$mode)
    $transport=Join-Path $output '_transport'
    if(Test-Path $output){throw 'Existing negative output'}
    New-Item -ItemType Directory -Path $transport | Out-Null
    Copy-Item -LiteralPath (Join-Path $wire 'plan.json') -Destination (Join-Path $transport 'plan.json')
    Copy-Item -LiteralPath (Join-Path $wire 'literal.bin') -Destination (Join-Path $transport 'literal.bin')
    $cache=$CacheDirectory
    if($mode -eq 'literal'){
        $file=[IO.File]::Open((Join-Path $transport 'literal.bin'),[IO.FileMode]::Open,[IO.FileAccess]::ReadWrite,[IO.FileShare]::None)
        try{$value=$file.ReadByte();if($value -lt 0){throw 'Literal required'};$file.Position=0;$file.WriteByte($value -bxor 1)}finally{$file.Dispose()}
    }else{
        $cache=Join-Path $Directory 'negative-source'
        $first=$plan.sources[0]
        if($first.name -notmatch '^[A-Za-z0-9_.-]+(/[A-Za-z0-9_.-]+)*$' -or @($first.name.Split('/') | Where-Object {$_ -eq '..' -or $_ -eq '.'}).Count){throw 'Source path rejected'}
        $path=Join-Path $cache $first.name
        New-Item -ItemType Directory -Path ([IO.Path]::GetDirectoryName($path)) | Out-Null
        $file=[IO.File]::Open($path,[IO.FileMode]::CreateNew,[IO.FileAccess]::Write,[IO.FileShare]::None)
        try{$file.SetLength($first.size)}finally{$file.Dispose()}
    }
    Invoke-Restore $cache $output $transport $false
    if(Test-Path (Join-Path $output 'unconfigured-product.exe')){throw 'Rejected input created executable'}
}
Invoke-Restore $CacheDirectory $Directory $wire $false
if(@(Get-CimInstance Win32_Process | Where-Object {$_.ExecutablePath -and $_.ExecutablePath.Equals($exe,[StringComparison]::OrdinalIgnoreCase)}).Count){throw 'Restorer process remains'}
if((Get-FileHash -LiteralPath $exe).Hash.ToLowerInvariant() -ne $ExpectedHelper){throw 'Helper changed'}
Remove-Item -LiteralPath $exe -Force
if(Test-Path $exe){throw 'Helper deletion unconfirmed'}
'{"version":1,"restored":true,"corrupt_literal_rejected":true,"corrupt_cache_rejected":true,"overwrite_rejected":true,"helper_joined_and_deleted":true}'
