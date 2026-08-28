param(
    [string]$OutputDir = "dist",
    [string]$WasmName = "PicaGo.wasm",
    [string]$HtmlName = "index.html",
    [switch]$Serve
)

$ErrorActionPreference = "Stop"

$projectRoot = Split-Path -Parent $PSScriptRoot
$outputPath = Join-Path $projectRoot $OutputDir
$goRoot = (go env GOROOT).Trim()
$wasmRuntime = Join-Path $goRoot "lib\wasm\wasm_exec.js"

if (-not (Test-Path -LiteralPath $wasmRuntime)) {
    throw "Runtime WASM Go introuvable: $wasmRuntime"
}

New-Item -ItemType Directory -Force -Path $outputPath | Out-Null

$wasmPath = Join-Path $outputPath $WasmName
$runtimePath = Join-Path $outputPath "wasm_exec.js"
$htmlPath = Join-Path $outputPath $HtmlName

$oldGoos = $env:GOOS
$oldGoarch = $env:GOARCH
$oldCgoEnabled = $env:CGO_ENABLED

try {
    $env:GOOS = "js"
    $env:GOARCH = "wasm"
    $env:CGO_ENABLED = "0"

    Write-Host "Compilation WASM -> $wasmPath"
    go build -trimpath -buildvcs=false -ldflags "-s -w" -o $wasmPath .
    if ($LASTEXITCODE -ne 0) {
        throw "Build WASM echoue"
    }

    Copy-Item -LiteralPath $wasmRuntime -Destination $runtimePath -Force

    $html = @'
<!doctype html>
<html lang="fr">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>PicaGo WASM</title>
</head>
<body>
  <script src="wasm_exec.js"></script>
  <script>
    const go = new Go();
    go.argv = ["__WASM_NAME__"];
    WebAssembly.instantiateStreaming(fetch("__WASM_NAME__"), go.importObject)
      .then(result => {
        go.run(result.instance);
      })
      .catch(error => {
        document.body.insertAdjacentHTML("beforeend",
          `<p>Impossible de charger <code>__WASM_NAME__</code> : ${error}</p>`);
      });
  </script>
</body>
</html>
'@
    $html = $html.Replace("__WASM_NAME__", $WasmName)
    Set-Content -LiteralPath $htmlPath -Value $html -Encoding utf8
}
finally {
    if ($null -eq $oldGoos) { Remove-Item Env:GOOS -ErrorAction SilentlyContinue } else { $env:GOOS = $oldGoos }
    if ($null -eq $oldGoarch) { Remove-Item Env:GOARCH -ErrorAction SilentlyContinue } else { $env:GOARCH = $oldGoarch }
    if ($null -eq $oldCgoEnabled) { Remove-Item Env:CGO_ENABLED -ErrorAction SilentlyContinue } else { $env:CGO_ENABLED = $oldCgoEnabled }
}

Write-Host "Fichiers WASM ecrits dans $outputPath"

if ($Serve) {
    $server = Get-Command python -ErrorAction SilentlyContinue
    if (-not $server) {
        throw "Python est necessaire pour lancer le serveur de test."
    }

    Start-Process -FilePath $server.Source -ArgumentList "-m", "http.server", "8080" `
        -WorkingDirectory $outputPath -WindowStyle Hidden | Out-Null
    Write-Host "Serveur de test lance sans fenetre console: http://localhost:8080"
} else {
    Write-Host "Pour tester sans fenetre console: .\scripts\build-wasm.ps1 -Serve"
}
