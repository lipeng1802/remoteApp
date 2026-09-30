#ifndef RepoRoot
    #error RepoRoot must be provided by build-installer.ps1
#endif
#ifndef AppVersion
    #define AppVersion "0.1.0"
#endif
#ifndef SourceRevision
    #define SourceRevision "unknown"
#endif

#ifndef PublishDir
#define PublishDir RepoRoot + "\artifacts\windows\publish"
#endif
#define OutputDir RepoRoot + "\artifacts\windows"

[Setup]
AppId={{756FE82F-3D9F-4AB1-9652-3532142CB7A7}
AppName=Personal Remote Desktop Agent
AppVersion={#AppVersion}
AppPublisher=Personal Remote Desktop
AppComments=Source revision {#SourceRevision}
DefaultDirName={autopf}\Personal Remote Desktop Agent
DefaultGroupName=Personal Remote Desktop Agent
DisableProgramGroupPage=yes
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
OutputDir={#OutputDir}
OutputBaseFilename=PersonalRemoteDesktopAgent-{#AppVersion}-win-x64-Setup
VersionInfoVersion={#AppVersion}.0
VersionInfoProductVersion={#AppVersion}
VersionInfoDescription=Personal Remote Desktop Agent Setup
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
UninstallDisplayIcon={app}\RemoteAgent.exe
CloseApplications=yes

[Files]
Source: "{#PublishDir}\*"; DestDir: "{app}"; Flags: ignoreversion recursesubdirs createallsubdirs

[Icons]
Name: "{group}\Personal Remote Desktop Agent"; Filename: "{app}\RemoteAgent.exe"
Name: "{autodesktop}\Personal Remote Desktop Agent"; Filename: "{app}\RemoteAgent.exe"; Tasks: desktopicon

[Tasks]
Name: "desktopicon"; Description: "Create a desktop shortcut"; GroupDescription: "Additional shortcuts:"; Flags: unchecked

[Run]
Filename: "{app}\RemoteAgent.exe"; Description: "Launch Personal Remote Desktop Agent"; Flags: nowait postinstall skipifsilent
