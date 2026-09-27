param(
    [string]$Version = "0.1.0"
)

$ErrorActionPreference = "Stop"
$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
$Project = Join-Path $RepoRoot "windows\RemoteAgent\src\RemoteAgent\RemoteAgent.csproj"
$PublishDirectory = Join-Path $RepoRoot "artifacts\windows\publish"
$InstallerScript = Join-Path $PSScriptRoot "RemoteAgent.iss"

if (Test-Path $PublishDirectory) {
    Remove-Item -Recurse -Force $PublishDirectory
}
New-Item -ItemType Directory -Force $PublishDirectory | Out-Null

dotnet publish $Project `
    --configuration Release `
    --runtime win-x64 `
    --self-contained true `
    --output $PublishDirectory `
    -p:PublishSingleFile=true `
    -p:Version=$Version

$Compiler = Get-Command ISCC.exe -ErrorAction SilentlyContinue
if ($null -eq $Compiler) {
    $DefaultCompiler = Join-Path ${env:ProgramFiles(x86)} "Inno Setup 6\ISCC.exe"
    if (Test-Path $DefaultCompiler) {
        $CompilerPath = $DefaultCompiler
    } else {
        throw "Inno Setup 6 was not found. Install it, then rerun this script."
    }
} else {
    $CompilerPath = $Compiler.Source
}

& $CompilerPath "/DRepoRoot=$RepoRoot" "/DAppVersion=$Version" $InstallerScript
if ($LASTEXITCODE -ne 0) {
    throw "Inno Setup failed with exit code $LASTEXITCODE."
}

Write-Host "Installer created in $(Join-Path $RepoRoot 'artifacts\windows')"
