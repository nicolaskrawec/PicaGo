# Objectif

Créer un viewer d’image desktop en **Go**, orienté performance, qui permet :

```
- d’ouvrir une image depuis l’explorateur Windows- de lancer l’application avec une image passée en argument- d’ouvrir la fenêtre en mode maximisé- d’afficher l’image en taille maximale dans la fenêtre, sans déformation- de zoomer avec la molette- de déplacer l’image avec la souris- de déposer une deuxième image par drag-and-drop- de comparer les deux images avec un système de slide avant/après
```

# Stack technique recommandée

```
Langage : GoRendu : Ebitengine / EbitenDécodage image : package image + extensions x/imagePlateforme cible principale : WindowsBuild : go build avec mode windowsgui
```

Ebitengine est adapté ici parce qu’il permet un rendu 2D GPU fluide, la fenêtre redimensionnable/maximisable, le fullscreen, et la gestion du drag-and-drop via son API desktop. La documentation officielle indique que les apps Ebitengine peuvent changer leur taille de fenêtre ou passer en plein écran via les fonctions de fenêtre, et que le mode de redimensionnement doit être activé pour certains comportements comme la maximisation.

Pour une fenêtre redimensionnable, Ebitengine recommande d’utiliser `RunGame` avec une fonction `Layout` adaptée, et précise que l’utilisateur peut maximiser la fenêtre lorsque le redimensionnement est activé.

# Formats image à supporter en v1

Support prioritaire :

```
- JPEG / JPG- PNG- GIF première frame- WebP- BMP- TIFF
```

À éviter dans la première version :

```
- HEIC / HEIF- AVIF- RAW appareil photo- PSD
```

Ces formats pourront venir plus tard via une dépendance externe comme `libvips`, ImageMagick, ou une conversion intermédiaire.

# Comportement au lancement

Quand l’utilisateur ouvre une image depuis l’explorateur Windows :

```
imageviewer.exe "C:\chemin\vers\image.jpg"
```

L’application doit :

```
1. Lire le premier argument d’exécution.2. Vérifier que le fichier existe.3. Décoder l’image.4. Créer une texture GPU.5. Ouvrir une fenêtre maximisée.6. Afficher l’image en mode "fit to window".7. Centrer l’image.8. Mettre le nom du fichier dans le titre de la fenêtre.
```

Le mode initial recommandé n’est pas “zoom 100 %”, mais :

```
zoom = min(windowWidth / imageWidth, windowHeight / imageHeight)
```

Cela permet d’avoir l’image la plus grande possible sans la couper ni la déformer.

# Comportement fenêtre

Au démarrage :

```
- fenêtre redimensionnable- fenêtre maximisée automatiquement- fond neutre sombre- image centrée- pas de console Windows visible
```

Build Windows recommandé :

```
go build -ldflags="-H windowsgui" -o imageviewer.exe
```

La fenêtre doit rester fluide même quand elle est redimensionnée.

# Interactions utilisateur

## Zoom

```
Molette vers le haut   : zoom avantMolette vers le bas    : zoom arrièreZoom centré sur la sourisZoom minimum           : fit / ou 5 %Zoom maximum           : 1600 % ou 3200 %
```

Important : le zoom doit être centré sur le point sous la souris. Sinon l’expérience sera désagréable.

## Pan

```
Clic gauche maintenu + déplacement souris = déplacer l’image
```

Le pan doit fonctionner en temps réel, sans recalculer l’image.

## Reset

```
R ou double-clic = revenir au mode fit to window
```

## Quitter

```
Échap = fermer
```

# Drag-and-drop

Quand l’utilisateur glisse une image dans la fenêtre :

```
- si aucune image secondaire n’existe, l’image déposée devient l’image B- l’image initiale reste l’image A- le mode comparaison est activé automatiquement- le zoom/pan est partagé entre A et B- les deux images sont alignées sur le même viewport
```

Ne pas remplacer l’image courante. Le drop sert à **ajouter une image de comparaison**.

# Système de comparaison par slide

Le viewer doit avoir deux modes :

```
Mode simple       : une seule image affichéeMode comparaison  : image A + image B avec séparateur vertical
```

En mode comparaison :

```
- image A affichée à gauche du curseur- image B affichée à droite du curseur- un séparateur vertical indique la limite- l’utilisateur peut déplacer le séparateur avec la souris
```

Comportement attendu :

```
Avant/après :[ image A à gauche ] | [ image B à droite ]Le séparateur peut être déplacé horizontalement.Le zoom et le pan s’appliquent aux deux images en même temps.
```

Raccourcis proposés :

```
Souris sur le séparateur + drag : déplacer le slideTouche C                        : activer/désactiver comparaisonTouche 1                        : afficher seulement image ATouche 2                        : afficher seulement image BTouche S                        : revenir au mode slideSuppr / Backspace               : retirer image B
```

# Alignement des images

Pour comparer correctement deux images, il faut utiliser un même système de coordonnées.

Chaque image doit être rendue avec :

```
screenX = imageX * zoom + offsetXscreenY = imageY * zoom + offsetY
```

En comparaison, les deux images doivent utiliser :

```
même zoommême offsetXmême offsetYmême centre de vue
```

Si les images ont des dimensions différentes, garder le même zoom logique et les centrer toutes les deux sur leur propre centre au chargement initial.

# Architecture interne

Structure recommandée :

```
cmd/imageviewer/main.gointernal/app/viewer.gointernal/image/decode.gointernal/image/formats.gointernal/input/mouse.gointernal/render/render.gointernal/compare/slider.go
```

Modèle de données :

```
ViewerState- imageA- imageB- activeMode- zoom- offsetX- offsetY- isDraggingImage- isDraggingSlider- sliderPosition- windowWidth- windowHeight
```

Objet image :

```
LoadedImage- fileName- filePath si disponible- width- height- decodedImage- gpuTexture
```

Modes d’affichage :

```
DisplayModeSingleADisplayModeSingleBDisplayModeCompareSlider
```

# Pipeline de chargement image

Pour la première version, le chargement peut être synchrone :

```
1. ouvrir fichier2. décoder image3. convertir en texture Ebiten4. stocker dimensions5. calculer fit-to-window
```

Pour une version plus fluide ensuite :

```
1. afficher l’image actuelle2. charger la nouvelle image dans une goroutine3. afficher un indicateur discret "loading"4. remplacer la texture une fois prête
```

Attention : la création ou modification de textures GPU doit rester compatible avec les contraintes d’Ebitengine. Le décodage peut être fait hors thread principal, mais l’intégration dans le rendu doit être proprement synchronisée.

# Gestion des très grandes images

Prévoir dès le design :

```
- images 8000 x 8000- panoramas très larges- TIFF lourds- fichiers de plusieurs centaines de Mo
```

Stratégie v1 :

```
- charger normalement- refuser proprement les images trop grosses si échec mémoire- afficher une erreur lisible
```

Stratégie v2 :

```
- downscale optionnel pour les images énormes- limite maximale configurable- cache mémoire- prévisualisation basse résolution puis remplacement haute résolution
```

# Étapes de développement pour Codex

## Étape 1 — Base du projet

```
- créer projet Go- ajouter Ebitengine- ajouter support JPEG, PNG, GIF, WebP, BMP, TIFF- créer l’entrée main- lire os.Args[1]- ouvrir une image- afficher une fenêtre
```

Critère de validation :

```
go run . "test.jpg"
```

ouvre l’image.

## Étape 2 — Fenêtre maximisée + fit image

```
- activer fenêtre redimensionnable- ouvrir maximisé- calculer zoom de fit- centrer l’image- recalculer le fit si reset demandé
```

Critère de validation :

```
L’image remplit au maximum la fenêtre sans être coupée.
```

## Étape 3 — Zoom et pan fluides

```
- molette = zoom- zoom centré sur la souris- clic gauche + drag = pan- limites de zoom- reset avec R
```

Critère de validation :

```
Le zoom ne saute pas, le point sous la souris reste stable.
```

## Étape 4 — Drag-and-drop image B

```
- détecter les fichiers déposés- filtrer les fichiers image- charger la première image valide- la stocker en imageB- conserver imageA- activer le mode comparaison
```

Critère de validation :

```
Après drop, l’image initiale n’est pas remplacée.
```

## Étape 5 — Mode slide comparatif

```
- afficher imageA sur toute la fenêtre- afficher imageB seulement à droite du slider- créer un masque de rendu ou clipping- afficher une ligne verticale de séparation- déplacer le slider à la souris
```

Critère de validation :

```
Le slide révèle progressivement l’image B au-dessus de l’image A.
```

Variante possible :

```
A à gauche, B à droite
```

Mais je recommande plutôt :

```
A en fond complet, B révélée par le slider
```

C’est plus naturel pour comparer deux versions d’une image.

## Étape 6 — Raccourcis clavier

```
R       reset fitÉchap   quitterC       comparaison on/off1       image A seule2       image B seuleS       slideSuppr   retirer image BF       fullscreen optionnel
```

## Étape 7 — UX propre

```
- titre de fenêtre avec nom de fichier- message si aucun fichier n’est fourni- message si format non supporté- message si drop invalide- curseur différent sur le slider- overlay discret avec zoom actuel
```

# Comportement final attendu

```
Double-clic sur image dans l’explorateur→ imageviewer.exe s’ouvre maximisé→ l’image est affichée au maximum dans la fenêtre→ molette pour zoomer→ clic gauche pour déplacer→ drag-and-drop d’une deuxième image→ mode comparaison slide activé→ déplacement du séparateur pour comparer
```

# Priorité de développement

Ordre recommandé :

```
1. Ouverture depuis argument2. Fenêtre maximisée3. Fit-to-window4. Zoom/pan5. Drag-and-drop6. Deuxième image7. Slider comparatif8. Raccourcis et UX9. Optimisations grosses images
```

# Points à préciser à Codex

Donne-lui ces contraintes explicitement :

```
- ne pas utiliser Fyne- utiliser Ebitengine- privilégier la fluidité du rendu- ne pas recalculer l’image à chaque frame- utiliser les transformations GPU pour zoom/pan- charger l’image A depuis os.Args[1]- le drag-and-drop ajoute image B, il ne remplace pas image A- le zoom et le pan doivent être communs aux deux images en mode comparaison- la fenêtre doit démarrer maximisée- l’image doit démarrer en fit-to-window
```

# MVP cible

Le MVP doit se limiter à :

```
- Windows- une image initiale en argument- fenêtre maximisée- fit-to-window- zoom/pan souris- drag-and-drop d’une deuxième image- comparaison par slider vertical
```