# PicaGo

Visualiseur d'images en Go, base sur Ebitengine.

## Prerequis

- Go installe
- Git pour la version auto au build
- Windows pour le script PowerShell de build

## Lancer en mode dev

Depuis la racine du projet :

```powershell
go run .
```

Ou avec une image :

```powershell
go run . "C:\chemin\vers\image.jpg"
```

Sans argument, l'application s'ouvre dans une fenetre `640x480`.

## Builds automatises

Depuis la racine du projet :

```powershell
.\scripts\build.ps1
```

Le script :

- calcule automatiquement la version a partir de Git
- injecte cette version dans `viewergo/internal/app.Version`
- produit des binaires multiplateformes dans `dist/`
- genere des binaires Windows sans console visible
- genere aussi l'icone du fichier `.exe` a partir de `internal/assets/icon.ico`

Version automatique :

- tag exact sur `HEAD` : `v1.2.3`
- commit apres un tag : `v1.2.3+4.6839b09`
- sans tag : `v0.0.0+6839b09`
- modifications locales : suffixe `.dirty`

Exemples :

```powershell
.\scripts\build.ps1 -Targets windows/amd64
.\scripts\build.ps1 -Targets windows/amd64,linux/amd64,darwin/arm64
.\scripts\build.ps1 -Version v1.3.0
```

Pour les builds Windows, le script utilise `rsrc`.
Installation une fois pour toutes :

```powershell
go install github.com/akavel/rsrc@latest
```

Le fichier `internal/assets/icon.ico` est utilise :

- comme icone de fenetre dans l'application
- comme icone du fichier `.exe` Windows au build

## Notes

- Les formats supportes actuellement sont : JPEG, PNG, GIF, WebP, BMP et TIFF.
