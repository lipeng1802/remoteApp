param(
    [string]$Version
)

$ErrorActionPreference = "Stop"
$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
$VersionFile = Join-Path $RepoRoot "VERSION"
if ([string]::IsNullOrWhiteSpace($Version)) {
    $Version = (Get-Content -LiteralPath $VersionFile -Raw).Trim()
}
if ($Version -notmatch '^\d+\.\d+\.\d+$') {
    throw "Version must use numeric major.minor.patch format: $Version"
}

$RelevantChanges = (& git -C $RepoRoot status --porcelain -- windows/RemoteAgent packaging/windows VERSION) -join "`n"
if (-not [string]::IsNullOrWhiteSpace($RelevantChanges)) {
    throw "Commit or remove Windows package input changes before building a traceable artifact.`n$RelevantChanges"
}

$SourceRevision = (& git -C $RepoRoot rev-parse --short=12 HEAD).Trim()
if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($SourceRevision)) {
    throw "Unable to determine the Git source revision."
}

$Solution = Join-Path $RepoRoot "windows\RemoteAgent\RemoteAgent.sln"
$Project = Join-Path $RepoRoot "windows\RemoteAgent\src\RemoteAgent\RemoteAgent.csproj"
$ProtocolTests = Join-Path $RepoRoot "windows\RemoteAgent\tests\RemoteProtocol.Tests\RemoteProtocol.Tests.csproj"
$InputTests = Join-Path $RepoRoot "windows\RemoteAgent\tests\WindowsInput.Tests\WindowsInput.Tests.csproj"
$PublishDirectory = Join-Path $RepoRoot "artifacts\windows\publish"
$InstallerScript = Join-Path $PSScriptRoot "RemoteAgent.iss"
$InstallerName = "PersonalRemoteDesktopAgent-$Version-win-x64-Setup.exe"
$InstallerPath = Join-Path (Join-Path $RepoRoot "artifacts\windows") $InstallerName

& dotnet build $Solution --configuration Release
if ($LASTEXITCODE -ne 0) { throw "Release build failed with exit code $LASTEXITCODE." }

& dotnet run --project $ProtocolTests --configuration Release
if ($LASTEXITCODE -ne 0) { throw "RemoteProtocol tests failed with exit code $LASTEXITCODE." }

& dotnet run --project $InputTests --configuration Release
if ($LASTEXITCODE -ne 0) { throw "WindowsInput tests failed with exit code $LASTEXITCODE." }

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
    -p:Version=$Version `
    -p:InformationalVersion="$Version+$SourceRevision" `
    -p:IncludeSourceRevisionInInformationalVersion=false

if ($LASTEXITCODE -ne 0) {
    throw "dotnet publish failed with exit code $LASTEXITCODE."
}

$PublishedExecutable = Join-Path $PublishDirectory "RemoteAgent.exe"
if (!(Test-Path -LiteralPath $PublishedExecutable)) {
    throw "Published executable was not produced at $PublishedExecutable."
}
$PublishedVersion = (Get-Item -LiteralPath $PublishedExecutable).VersionInfo
$ExpectedFileVersion = "$Version.0"
$ExpectedProductVersion = "$Version+$SourceRevision"
if ($PublishedVersion.FileVersion -ne $ExpectedFileVersion) {
    throw "Unexpected RemoteAgent.exe FileVersion: $($PublishedVersion.FileVersion); expected $ExpectedFileVersion."
}
if ($PublishedVersion.ProductVersion -ne $ExpectedProductVersion) {
    throw "Unexpected RemoteAgent.exe ProductVersion: $($PublishedVersion.ProductVersion); expected $ExpectedProductVersion."
}

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

& $CompilerPath "/DRepoRoot=$RepoRoot" "/DAppVersion=$Version" "/DSourceRevision=$SourceRevision" $InstallerScript
if ($LASTEXITCODE -ne 0) {
    throw "Inno Setup failed with exit code $LASTEXITCODE."
}

if (!(Test-Path -LiteralPath $InstallerPath)) {
    throw "Installer was not produced at $InstallerPath."
}
$Hash = (Get-FileHash -LiteralPath $InstallerPath -Algorithm SHA256).Hash.ToLowerInvariant()
Set-Content -LiteralPath "$InstallerPath.sha256" -Encoding ascii -NoNewline -Value "$Hash  $InstallerName`n"

Write-Host "Created $InstallerPath"
Write-Host "Checksum $InstallerPath.sha256"
