param(
    [string]$Version,
    [string]$OutputDir = "dist",
    [string]$InstallerOutputDir = "dist\\installer"
)

$ErrorActionPreference = "Stop"

function Get-BuildVersion {
    param([string]$ExplicitVersion)

    if ($ExplicitVersion) {
        return $ExplicitVersion
    }

    $sha = (git rev-parse --short HEAD 2>$null).Trim()
    if (-not $sha) {
        return "dev-" + (Get-Date -Format "yyyyMMddHHmmss")
    }

    $tag = git tag --list "v*" --sort=-creatordate | Select-Object -First 1
    $exact = git tag --points-at HEAD --list "v*" | Select-Object -First 1
    git diff --quiet --ignore-submodules HEAD --
    $dirty = $LASTEXITCODE -ne 0

    if ($exact) {
        $resolved = $exact.Trim()
    } elseif ($tag) {
        $tag = $tag.Trim()
        $count = (git rev-list "$tag..HEAD" --count 2>$null).Trim()
        if (-not $count) {
            $count = "0"
        }
        $resolved = "$tag+$count.$sha"
    } else {
        $resolved = "v0.0.0+$sha"
    }

    if ($dirty) {
        $resolved += ".dirty"
    }

    return $resolved
}

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
$resolvedVersion = Get-BuildVersion -ExplicitVersion $Version
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

New-Item -ItemType Directory -Force -Path $installerOutputAbsolute | Out-Null

$iscc = Get-InnoSetupCompiler
& $iscc `
    "/DAppVersion=$resolvedVersion" `
    "/DBuildDir=$outputDirAbsolute" `
    "/DInstallerOutputDir=$installerOutputAbsolute" `
    $installerScript

if ($LASTEXITCODE -ne 0) {
    throw "La generation de l'installeur Windows a echoue"
}

Write-Host "Installeur genere dans $InstallerOutputDir"
