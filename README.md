# PicaGo

Visualiseur d'images en Go pour Windows, base sur Ebitengine.

## Prerequis

- Go installe
- Windows

## Lancer en mode dev

Depuis la racine du projet :

```powershell
go run . "C:\chemin\vers\image.jpg"
```

## Compiler un exe Windows optimise

Depuis la racine du projet :

```powershell
go build -trimpath -buildvcs=false -ldflags="-s -w -H windowsgui -X viewergo/internal/app.Version=v1.0.0" -o PicaGo.exe .
```

Cela produit `PicaGo.exe` sans console visible, avec un binaire plus compact.
La version affichée dans le help est injectee au build via `-ldflags -X`.
Si tu changes de version, remplace `v1.0.0` par la valeur voulue.

## Notes

- Le premier argument de la ligne de commande doit etre un fichier image.
- Les formats supportes actuellement sont : JPEG, PNG, GIF, WebP, BMP et TIFF.
