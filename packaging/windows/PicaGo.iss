#define MyAppName "PicaGo"
#define MyAppExeName "PicaGo.exe"
#define MyAppPublisher "PicaGo"
#define MyAppAssocName MyAppName + " Image"
#define MyAppAssocKey StringChange(MyAppAssocName, " ", "") + ".1"

#ifndef AppVersion
  #define AppVersion "dev"
#endif

#ifndef BuildDir
  #error BuildDir non defini. Utilise scripts\build-installer.ps1.
#endif

#ifndef InstallerOutputDir
  #define InstallerOutputDir AddBackslash(SourcePath) + "..\\..\\dist\\installer"
#endif

[Setup]
AppId={{A4F49531-20B5-4DFD-8D7F-45F7466EFA90}
AppName={#MyAppName}
AppVersion={#AppVersion}
AppPublisher={#MyAppPublisher}
DefaultDirName={autopf}\{#MyAppName}
DefaultGroupName={#MyAppName}
UninstallDisplayIcon={app}\{#MyAppExeName}
SetupIconFile={#BuildDir}\..\icon.ico
Compression=lzma
SolidCompression=yes
WizardStyle=modern
ArchitecturesInstallIn64BitMode=x64compatible
ChangesAssociations=yes
OutputDir={#InstallerOutputDir}
OutputBaseFilename=PicaGo-Setup-{#AppVersion}

[Languages]
Name: "french"; MessagesFile: "compiler:Languages\French.isl"

[Tasks]
Name: "desktopicon"; Description: "Creer un raccourci sur le Bureau"; GroupDescription: "Options supplementaires :"
Name: "registerformats"; Description: "Enregistrer PicaGo pour les formats d'image supportes"; GroupDescription: "Integration Windows :"; Flags: checkedonce
Name: "opendefaultapps"; Description: "Ouvrir les reglages Windows des applications par defaut a la fin"; GroupDescription: "Integration Windows :"; Flags: unchecked

[Files]
Source: "{#BuildDir}\{#MyAppExeName}"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#BuildDir}\PicaGo.json"; DestDir: "{app}"; Flags: onlyifdoesntexist

[Icons]
Name: "{group}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"
Name: "{autodesktop}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; Tasks: desktopicon

[Registry]
Root: HKLM; Subkey: "Software\Microsoft\Windows\CurrentVersion\App Paths\{#MyAppExeName}"; ValueType: string; ValueData: "{app}\{#MyAppExeName}"; Flags: uninsdeletekey
Root: HKLM; Subkey: "Software\Classes\{#MyAppAssocKey}"; ValueType: string; ValueData: "{#MyAppAssocName}"; Flags: uninsdeletekey; Tasks: registerformats
Root: HKLM; Subkey: "Software\Classes\{#MyAppAssocKey}\DefaultIcon"; ValueType: string; ValueData: "{app}\{#MyAppExeName},0"; Tasks: registerformats
Root: HKLM; Subkey: "Software\Classes\{#MyAppAssocKey}\shell\open\command"; ValueType: string; ValueData: """{app}\{#MyAppExeName}"" ""%1"""; Tasks: registerformats

Root: HKLM; Subkey: "Software\Classes\Applications\{#MyAppExeName}"; ValueType: string; ValueData: "{#MyAppName}"; Flags: uninsdeletekey; Tasks: registerformats
Root: HKLM; Subkey: "Software\Classes\Applications\{#MyAppExeName}\DefaultIcon"; ValueType: string; ValueData: "{app}\{#MyAppExeName},0"; Tasks: registerformats
Root: HKLM; Subkey: "Software\Classes\Applications\{#MyAppExeName}\shell\open\command"; ValueType: string; ValueData: """{app}\{#MyAppExeName}"" ""%1"""; Tasks: registerformats
Root: HKLM; Subkey: "Software\Classes\Applications\{#MyAppExeName}\SupportedTypes"; ValueType: string; ValueName: ".jpg"; ValueData: ""; Tasks: registerformats
Root: HKLM; Subkey: "Software\Classes\Applications\{#MyAppExeName}\SupportedTypes"; ValueType: string; ValueName: ".jpeg"; ValueData: ""; Tasks: registerformats
Root: HKLM; Subkey: "Software\Classes\Applications\{#MyAppExeName}\SupportedTypes"; ValueType: string; ValueName: ".png"; ValueData: ""; Tasks: registerformats
Root: HKLM; Subkey: "Software\Classes\Applications\{#MyAppExeName}\SupportedTypes"; ValueType: string; ValueName: ".gif"; ValueData: ""; Tasks: registerformats
Root: HKLM; Subkey: "Software\Classes\Applications\{#MyAppExeName}\SupportedTypes"; ValueType: string; ValueName: ".webp"; ValueData: ""; Tasks: registerformats
Root: HKLM; Subkey: "Software\Classes\Applications\{#MyAppExeName}\SupportedTypes"; ValueType: string; ValueName: ".bmp"; ValueData: ""; Tasks: registerformats
Root: HKLM; Subkey: "Software\Classes\Applications\{#MyAppExeName}\SupportedTypes"; ValueType: string; ValueName: ".tif"; ValueData: ""; Tasks: registerformats
Root: HKLM; Subkey: "Software\Classes\Applications\{#MyAppExeName}\SupportedTypes"; ValueType: string; ValueName: ".tiff"; ValueData: ""; Tasks: registerformats

Root: HKLM; Subkey: "Software\{#MyAppName}\Capabilities"; ValueType: string; ValueName: "ApplicationName"; ValueData: "{#MyAppName}"; Flags: uninsdeletekey; Tasks: registerformats
Root: HKLM; Subkey: "Software\{#MyAppName}\Capabilities"; ValueType: string; ValueName: "ApplicationDescription"; ValueData: "Visionneuse d'images legere pour JPEG, PNG, GIF, WebP, BMP et TIFF."; Tasks: registerformats
Root: HKLM; Subkey: "Software\{#MyAppName}\Capabilities\FileAssociations"; ValueType: string; ValueName: ".jpg"; ValueData: "{#MyAppAssocKey}"; Tasks: registerformats
Root: HKLM; Subkey: "Software\{#MyAppName}\Capabilities\FileAssociations"; ValueType: string; ValueName: ".jpeg"; ValueData: "{#MyAppAssocKey}"; Tasks: registerformats
Root: HKLM; Subkey: "Software\{#MyAppName}\Capabilities\FileAssociations"; ValueType: string; ValueName: ".png"; ValueData: "{#MyAppAssocKey}"; Tasks: registerformats
Root: HKLM; Subkey: "Software\{#MyAppName}\Capabilities\FileAssociations"; ValueType: string; ValueName: ".gif"; ValueData: "{#MyAppAssocKey}"; Tasks: registerformats
Root: HKLM; Subkey: "Software\{#MyAppName}\Capabilities\FileAssociations"; ValueType: string; ValueName: ".webp"; ValueData: "{#MyAppAssocKey}"; Tasks: registerformats
Root: HKLM; Subkey: "Software\{#MyAppName}\Capabilities\FileAssociations"; ValueType: string; ValueName: ".bmp"; ValueData: "{#MyAppAssocKey}"; Tasks: registerformats
Root: HKLM; Subkey: "Software\{#MyAppName}\Capabilities\FileAssociations"; ValueType: string; ValueName: ".tif"; ValueData: "{#MyAppAssocKey}"; Tasks: registerformats
Root: HKLM; Subkey: "Software\{#MyAppName}\Capabilities\FileAssociations"; ValueType: string; ValueName: ".tiff"; ValueData: "{#MyAppAssocKey}"; Tasks: registerformats
Root: HKLM; Subkey: "Software\RegisteredApplications"; ValueType: string; ValueName: "{#MyAppName}"; ValueData: "Software\{#MyAppName}\Capabilities"; Tasks: registerformats

Root: HKLM; Subkey: "Software\Classes\.jpg\OpenWithProgids"; ValueType: string; ValueName: "{#MyAppAssocKey}"; ValueData: ""; Tasks: registerformats
Root: HKLM; Subkey: "Software\Classes\.jpeg\OpenWithProgids"; ValueType: string; ValueName: "{#MyAppAssocKey}"; ValueData: ""; Tasks: registerformats
Root: HKLM; Subkey: "Software\Classes\.png\OpenWithProgids"; ValueType: string; ValueName: "{#MyAppAssocKey}"; ValueData: ""; Tasks: registerformats
Root: HKLM; Subkey: "Software\Classes\.gif\OpenWithProgids"; ValueType: string; ValueName: "{#MyAppAssocKey}"; ValueData: ""; Tasks: registerformats
Root: HKLM; Subkey: "Software\Classes\.webp\OpenWithProgids"; ValueType: string; ValueName: "{#MyAppAssocKey}"; ValueData: ""; Tasks: registerformats
Root: HKLM; Subkey: "Software\Classes\.bmp\OpenWithProgids"; ValueType: string; ValueName: "{#MyAppAssocKey}"; ValueData: ""; Tasks: registerformats
Root: HKLM; Subkey: "Software\Classes\.tif\OpenWithProgids"; ValueType: string; ValueName: "{#MyAppAssocKey}"; ValueData: ""; Tasks: registerformats
Root: HKLM; Subkey: "Software\Classes\.tiff\OpenWithProgids"; ValueType: string; ValueName: "{#MyAppAssocKey}"; ValueData: ""; Tasks: registerformats

[Run]
Filename: "ms-settings:defaultapps"; Description: "Ouvrir les applications par defaut Windows"; Flags: postinstall shellexec nowait skipifsilent; Tasks: opendefaultapps
