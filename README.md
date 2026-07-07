# ViewerGo

Visualiseur d'images en Go pour Windows, basé sur Ebitengine.

## Prerequis

- Go installe
- Windows

## Lancer en mode dev

Depuis la racine du projet :

```powershell
go run . "C:\chemin\vers\image.jpg"
```

## Compiler un exe Windows sans fenetre DOS

Depuis la racine du projet :

```powershell
go build -buildvcs=false -ldflags="-H windowsgui" -o imageviewer.exe .
```

Cela produit `imageviewer.exe` sans console visible.

## Notes

- Le premier argument de la ligne de commande doit etre un fichier image.
- Les formats supportes actuellement sont : JPEG, PNG, GIF, WebP, BMP et TIFF.
