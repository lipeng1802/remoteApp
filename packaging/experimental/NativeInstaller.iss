; TEST-only transport for a freshly built, fixed installer-adapter payload.
; No registered app, uninstaller, shortcuts, elevation, or old MVP replacement.
#ifndef PayloadDir
  #error PayloadDir is required
#endif
#ifndef OutputDir
  #error OutputDir is required
#endif
#ifndef Epoch
  #error Epoch is required
#endif
#if (Epoch != "1") && (Epoch != "2")
  #error Epoch must be 1 or 2
#endif

[Setup]
AppName=RemoteApp Native Installer Preview
AppVersion=0.0.{#Epoch}
AppId=RemoteAppNativeInstallerPreview
DefaultDirName={localappdata}\RemoteApp Native Installer Preview
DisableDirPage=yes
DisableProgramGroupPage=yes
CreateAppDir=no
Uninstallable=no
PrivilegesRequired=lowest
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
OutputDir={#OutputDir}
OutputBaseFilename=RemoteApp-Native-Preview-epoch{#Epoch}
Compression=zip/1
SolidCompression=no
CloseApplications=no
RestartApplications=no
WizardStyle=modern
SetupLogging=yes

[Files]
Source: "{#PayloadDir}\*"; DestDir: "{tmp}\payload"; Flags: dontcopy recursesubdirs createallsubdirs

[Code]
var
  Applied: Boolean;

function PrepareToInstall(var NeedsRestart: Boolean): String;
var
  Code: Integer;
begin
  Result := '';
  if Applied then Exit;
  ExtractTemporaryFiles('{tmp}\payload\*');
  if not Exec(ExpandConstant('{tmp}\payload\installer-adapter.exe'), '',
      ExpandConstant('{tmp}\payload'), SW_HIDE, ewWaitUntilTerminated, Code) then
  begin
    Result := 'Unable to start the installation. Existing identity and recovery state were retained.';
    Exit;
  end;
  if Code <> 0 then
  begin
    Log('Fixed installer-adapter rejected operation. Exit code: ' + IntToStr(Code));
    Result := 'Installation was not confirmed. Keep the recovery state and retry the same signed package.';
    Exit;
  end;
  Applied := True;
end;
