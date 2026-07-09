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

function Get-RsrcTool {
    $gopath = (go env GOPATH).Trim()
    $toolPath = Join-Path $gopath "bin\\rsrc.exe"
    if (-not (Test-Path $toolPath)) {
        throw "rsrc.exe introuvable. Installe-le avec: go install github.com/akavel/rsrc@latest"
    }
    return $toolPath
}

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
        [string]$Goarch
    )

    $rsrc = Get-RsrcTool
    $iconPath = Join-Path $ProjectRoot "internal\\assets\\icon.ico"
    if (-not (Test-Path $iconPath)) {
        throw "Icone introuvable: $iconPath"
    }

    $output = Join-Path $ProjectRoot "rsrc_windows_${Goarch}.syso"
    & $rsrc -ico $iconPath -arch $Goarch -o $output
    if ($LASTEXITCODE -ne 0) {
        throw "Generation de la ressource Windows echouee pour $Goarch"
    }

    return $output
}

$projectRoot = Split-Path -Parent $PSScriptRoot
$resolvedVersion = Get-BuildVersion -ExplicitVersion $Version
$binaryName = "PicaGo"
$ldflagsBase = @(
    "-s",
    "-w",
    "-X", "viewergo/internal/app.Version=$resolvedVersion"
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
    if ($goos -eq "windows") {
        $ldflags += @("-H", "windowsgui")
        $generatedResources += New-WindowsResource -ProjectRoot $projectRoot -Goarch $goarch
    }

    Write-Host " -> $target"
    $env:GOOS = $goos
    $env:GOARCH = $goarch
    $env:CGO_ENABLED = "0"
    if ($goarch -eq "amd64") {
        $env:GOAMD64 = $Goamd64
    } else {
        Remove-Item Env:GOAMD64 -ErrorAction SilentlyContinue
    }
    go build -trimpath -buildvcs=false -ldflags ($ldflags -join " ") -o $outputPath .
}

Remove-Item Env:GOOS -ErrorAction SilentlyContinue
Remove-Item Env:GOARCH -ErrorAction SilentlyContinue
Remove-Item Env:CGO_ENABLED -ErrorAction SilentlyContinue
Remove-Item Env:GOAMD64 -ErrorAction SilentlyContinue

foreach ($resource in ($generatedResources | Select-Object -Unique)) {
    Remove-Item -LiteralPath $resource -ErrorAction SilentlyContinue
}

Write-Host "Artifacts written to $OutputDir"
