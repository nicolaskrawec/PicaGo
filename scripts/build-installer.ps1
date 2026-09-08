param(
    [string]$Version,
    [string]$OutputDir = "dist",
    [string]$InstallerOutputDir = "dist\\installer"
)

$ErrorActionPreference = "Stop"

function Get-InnoSetupCompiler {
    $command = Get-Command ISCC.exe -ErrorAction SilentlyContinue
    if ($command) {
        return $command.Source
    }

    $candidates = @(
        "${env:ProgramFiles(x86)}\\Inno Setup 6\\ISCC.exe",
        "${env:ProgramFiles}\\Inno Setup 6\\ISCC.exe"
    )

    foreach ($candidate in $candidates) {
        if ($candidate -and (Test-Path $candidate)) {
            return $candidate
        }
    }

    throw "ISCC.exe introuvable. Installe Inno Setup 6 puis relance ce script."
}

$projectRoot = Split-Path -Parent $PSScriptRoot
$versionArguments = @("run", "./cmd/buildversion", "--format", "lines")
if ($Version) {
    $versionArguments += @("--version", $Version)
}
Push-Location $projectRoot
try {
    $versionInfo = @(& go @versionArguments)
    if ($LASTEXITCODE -ne 0) {
        throw "La resolution de la version a echoue"
    }
} finally {
    Pop-Location
}
if ($versionInfo.Count -ne 2) {
    throw "Sortie inattendue de l'outil de version"
}
$resolvedVersion = $versionInfo[0].Trim()
$windowsVersion = $versionInfo[1].Trim()
$buildScript = Join-Path $PSScriptRoot "build.ps1"
$outputDirAbsolute = Join-Path $projectRoot $OutputDir
$installerOutputAbsolute = Join-Path $projectRoot $InstallerOutputDir
$installerScript = Join-Path $projectRoot "packaging\\windows\\PicaGo.iss"

if (-not (Test-Path $installerScript)) {
    throw "Script Inno Setup introuvable: $installerScript"
}

& $buildScript -Version $resolvedVersion -Targets @("windows/amd64") -OutputDir $OutputDir
if ($LASTEXITCODE -ne 0) {
    throw "Le build Windows a echoue"
}

$stableExe = Join-Path $outputDirAbsolute "PicaGo.exe"
if (-not (Test-Path $stableExe)) {
    throw "Binaire Windows introuvable: $stableExe"
}

$configTemplate = Join-Path $projectRoot "PicaGo.json"
if (-not (Test-Path $configTemplate)) {
    throw "Fichier de configuration introuvable: $configTemplate"
}
Copy-Item -LiteralPath $configTemplate -Destination (Join-Path $outputDirAbsolute "PicaGo.json") -Force

New-Item -ItemType Directory -Force -Path $installerOutputAbsolute | Out-Null

$iscc = Get-InnoSetupCompiler
& $iscc `
    "/DAppVersion=$resolvedVersion" `
    "/DAppNumericVersion=$windowsVersion" `
    "/DBuildDir=$outputDirAbsolute" `
    "/DSetupIconFile=$(Join-Path $projectRoot 'icon.ico')" `
    "/DInstallerOutputDir=$installerOutputAbsolute" `
    $installerScript

if ($LASTEXITCODE -ne 0) {
    throw "La generation de l'installeur Windows a echoue"
}

Write-Host "Installeur genere dans $InstallerOutputDir"
