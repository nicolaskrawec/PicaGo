param(
    [string]$Version,
    [string[]]$Targets = @(
        "windows/amd64",
        "windows/arm64",
        "linux/amd64",
        "linux/arm64",
        "darwin/amd64",
        "darwin/arm64"
    ),
    [string]$OutputDir = "dist",
    [ValidateSet("v1", "v2", "v3", "v4")]
    [string]$Goamd64 = "v1"
)

$ErrorActionPreference = "Stop"

function Get-VersionInfoTool {
    $gopath = (go env GOPATH).Trim()
    $toolPath = Join-Path $gopath "bin\\goversioninfo.exe"
    if (-not (Test-Path $toolPath)) {
        throw "goversioninfo.exe introuvable. Installe-le avec: go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest"
    }
    return $toolPath
}

function Get-ArtifactName {
    param(
        [string]$Name,
        [string]$ResolvedVersion,
        [string]$Goos,
        [string]$Goarch
    )

    $suffix = if ($Goos -eq "windows") { ".exe" } else { "" }
    return "${Name}_${ResolvedVersion}_${Goos}_${Goarch}${suffix}"
}

function New-WindowsResource {
    param(
        [string]$ProjectRoot,
        [string]$Goarch,
        [string]$ResolvedVersion,
        [string]$WindowsVersion,
        [string]$OriginalFilename
    )

    $tool = Get-VersionInfoTool
    $iconPath = Join-Path $ProjectRoot "internal\\assets\\icon.ico"
    if (-not (Test-Path $iconPath)) {
        throw "Icone introuvable: $iconPath"
    }

    $output = Join-Path $ProjectRoot "rsrc_windows_${Goarch}.syso"
    $parts = $WindowsVersion.Split(".")
    $arguments = @(
        "-icon=$iconPath", "-o=$output",
        "-ver-major=$($parts[0])", "-ver-minor=$($parts[1])", "-ver-patch=$($parts[2])", "-ver-build=$($parts[3])",
        "-product-ver-major=$($parts[0])", "-product-ver-minor=$($parts[1])", "-product-ver-patch=$($parts[2])", "-product-ver-build=$($parts[3])",
        "-file-version=$WindowsVersion", "-product-version=$WindowsVersion",
        "-product-name=PicaGo", "-company=NkSoft", "-internal-name=PicaGo", "-original-name=$OriginalFilename",
        "-description=PicaGo image viewer", "-comment=Build $ResolvedVersion"
    )
    & $tool @arguments
    if ($LASTEXITCODE -ne 0) {
        throw "Generation de la ressource Windows echouee pour $Goarch"
    }

    return $output
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
$binaryName = "PicaGo"
$stableWindowsBinaryPath = Join-Path (Join-Path $projectRoot $OutputDir) "PicaGo.exe"
$ldflagsBase = @(
    "-s",
    "-w",
    "-X", "github.com/nicolaskrawec/PicaGo/internal/app.Version=$resolvedVersion"
)

New-Item -ItemType Directory -Force -Path (Join-Path $projectRoot $OutputDir) | Out-Null

Write-Host "Building version $resolvedVersion"

$generatedResources = @()

foreach ($target in $Targets) {
    $parts = $target.Split("/")
    if ($parts.Length -ne 2) {
        throw "Invalid target '$target'. Expected format goos/goarch."
    }

    $goos = $parts[0]
    $goarch = $parts[1]
    $artifact = Get-ArtifactName -Name $binaryName -ResolvedVersion $resolvedVersion -Goos $goos -Goarch $goarch
    $outputPath = Join-Path (Join-Path $projectRoot $OutputDir) $artifact

    $ldflags = @($ldflagsBase)
    $env:GOARCH = $goarch
    if ($goos -eq "windows") {
        $ldflags += @("-H", "windowsgui")
        $generatedResources += New-WindowsResource -ProjectRoot $projectRoot -Goarch $goarch -ResolvedVersion $resolvedVersion -WindowsVersion $windowsVersion -OriginalFilename $artifact
    }

    Write-Host " -> $target"
    $env:GOOS = $goos
    $env:CGO_ENABLED = "0"
    if ($goarch -eq "amd64") {
        $env:GOAMD64 = $Goamd64
    } else {
        Remove-Item Env:GOAMD64 -ErrorAction SilentlyContinue
    }
    go build -trimpath -buildvcs=false -ldflags ($ldflags -join " ") -o $outputPath .
    if ($LASTEXITCODE -ne 0) {
        throw "Build echoue pour $target"
    }

    if ($goos -eq "windows" -and $goarch -eq "amd64") {
        Copy-Item -LiteralPath $outputPath -Destination $stableWindowsBinaryPath -Force
    }
}

Remove-Item Env:GOOS -ErrorAction SilentlyContinue
Remove-Item Env:GOARCH -ErrorAction SilentlyContinue
Remove-Item Env:CGO_ENABLED -ErrorAction SilentlyContinue
Remove-Item Env:GOAMD64 -ErrorAction SilentlyContinue

foreach ($resource in ($generatedResources | Select-Object -Unique)) {
    Remove-Item -LiteralPath $resource -ErrorAction SilentlyContinue
}

Write-Host "Artifacts written to $OutputDir"
