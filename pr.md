# Revue de code de PicaGo

Date de la revue : 12 juillet 2026

## Périmètre

La revue couvre principalement :

- la boucle applicative et la gestion des commandes ;
- le chargement, la navigation et le cache d'images ;
- les animations et les états de vue ;
- les rendus simple, comparatif et circulaire ;
- les scripts de compilation Windows et Linux ;
- les tests actuellement présents.

Commandes de validation exécutées :

```text
go test ./...
go vet ./...
git diff --check
```

Ces commandes passent. Les remarques ci-dessous concernent donc surtout des
comportements qui ne sont pas détectés par la compilation ou les tests actuels.

## Synthèse

Le projet a une base saine : les responsabilités sont maintenant bien séparées,
les ressources CPU/GPU ont des méthodes de libération explicites, les résultats
asynchrones sont identifiés pour éviter d'appliquer une ancienne image, et les
calculs délicats de rotation/miroir du comparateur commencent à être couverts
par des tests ciblés.

Les principaux risques restants sont la gestion des commandes temporaires, le
manque d'annulation des chargements devenus inutiles et l'existence de plusieurs
chemins de rendu dont les fonctionnalités ne sont pas strictement équivalentes.

## Constats

### [Corrigé] Les touches `1` et `2` n'étaient pas réellement temporaires

Référence : `internal/app/commands.go:343-348`

La documentation indique qu'il faut maintenir `1` ou `2` pour inspecter A ou B.
Le code change cependant durablement `v.mode` en `displayModeSingleA` ou
`displayModeSingleB`. Aucun traitement au relâchement ne restaure le mode de
comparaison précédent.

Conséquence : après avoir relâché la touche, l'utilisateur reste sur une seule
image et doit appuyer sur `H`, `V` ou `C` pour retrouver la comparaison.

Correction appliquée : le mode précédent est maintenant mémorisé au premier
appui et restauré lorsque les touches sont relâchées. Des tests couvrent `1`,
`2`, les deux touches simultanées et l'absence d'image B.

### [Élevé] Les décodages remplacés continuent à consommer des ressources

Références :

- `internal/app/loading.go:15-61`
- `internal/app/loading.go:137-152`
- `internal/app/navigation.go:101-111`

Les identifiants de requête empêchent correctement un ancien résultat d'être
affiché, mais ils n'annulent pas le travail. Une navigation rapide peut lancer
plusieurs décodages complets ; chacun continue jusqu'au bout avant que son
résultat soit rejeté. Les chargements principaux ne partagent pas non plus le
sémaphore utilisé par le préchargement.

Conséquence : pics de CPU et de mémoire sur des dossiers contenant de grosses
images, précisément lorsque l'utilisateur parcourt rapidement les fichiers.
Les goroutines terminées peuvent également rester bloquées temporairement sur
le canal de résultats si sa capacité est atteinte.

Recommandation : introduire une file de travail bornée et une annulation par
génération ou `context.Context`. Au minimum, limiter le nombre de décodages A/B
simultanés et donner la priorité au fichier actuellement demandé sur le
préchargement.

### [Moyen] Le flou du split disparaît après une rotation ou un miroir

Références :

- `internal/render/comparison.go:14-77`
- `internal/render/comparison.go:82-115`
- `internal/render/comparison.go:374-410`
- `internal/app/viewer.go:282-297`

`DrawCompare` quitte vers un découpage polygonal dès que la vue possède une
rotation ou un `MirrorScale`. Ce chemin ne reçoit ni n'applique l'option
`feathered`. `DrawCompareRotating` ne possède pas non plus ce paramètre.

Comme `MirrorScaleX/Y` reste ensuite à `1` ou `-1`, le split peut conserver
définitivement le chemin sans flou après le premier miroir. Toute rotation non
nulle produit le même effet.

Conséquence : la touche `B` semble ne plus fonctionner selon l'historique des
transformations, même lorsque l'animation est terminée.

Recommandation : appliquer le feathering dans le même repère écran que le plan
de coupe polygonal, et utiliser un seul chemin de rendu pour les états animés et
stables. Ajouter des tests visuels avant/après rotation et miroir.

### [Moyen] Les animations peuvent s'écraser lorsqu'elles se chevauchent

Référence : `internal/app/animations.go:122-219`

`animateView` met successivement à jour le miroir, la rotation, puis l'animation
générale de vue. Cette dernière peut réassigner la structure `v.view` complète
après le calcul de `RotationAngle` ou de `MirrorScale`. Les commandes permettent
par ailleurs de commencer un miroir pendant une rotation, alors que les deux
animations partagent `rotationSplitLocalSide` et `rotationSplitLocalRatio`.

Conséquence possible : saut d'une frame, masque temporairement incorrect ou
transformation perdue lorsqu'une rotation/un miroir est demandé pendant un fit,
un zoom animé ou une autre transformation.

Recommandation : sérialiser les transformations, ou composer toutes les
animations dans une vue finale unique sans réassigner `v.view` en plusieurs
étapes. Ajouter une matrice de tests : fit + rotation, miroir + rotation,
rotation + miroir et commandes répétées rapidement.

### [Moyen] Le script PowerShell ne nettoie pas son état en cas d'échec

Références : `scripts/build.ps1:133-182`

Les variables `GOOS`, `GOARCH`, `CGO_ENABLED` et `GOAMD64`, ainsi que les fichiers
`.syso`, sont nettoyés uniquement après la boucle. Avec
`$ErrorActionPreference = "Stop"`, une erreur de `goversioninfo` ou de
`go build` interrompt le script avant ce nettoyage.

Conséquence : le terminal appelant peut conserver un environnement de cross
compilation et le dépôt un fichier de ressource temporaire. Un build Go manuel
effectué ensuite peut viser la mauvaise plateforme ou embarquer une ressource
inattendue.

Recommandation : envelopper la génération dans `try { ... } finally { ... }` et
restaurer les anciennes valeurs des variables au lieu de seulement les
supprimer.

### [Faible] `Get-WindowsVersion` est déclaré deux fois

Références : `scripts/build.ps1:27-37` et `scripts/build.ps1:164-173`

La seconde déclaration est identique et intervient après la boucle de build.
Elle ne casse pas le script actuel, car la première définition est disponible,
mais elle rend le fichier ambigu et peut conduire à ne modifier qu'une seule
copie lors d'une future correction.

Recommandation : supprimer la seconde déclaration et ajouter des tests de
conversion pour `v1.2.3`, `v1.2.3+4.sha`, une version `dev` et des valeurs hors
des limites de `VERSIONINFO`.

### [Corrigé] La détection de transparence parcourt tous les pixels

Référence : `internal/image/decode.go:120-172`

Pour tout type d'image autre que quelques formats opaques connus,
`hasTransparency` appelle `At` sur chaque pixel jusqu'à trouver un alpha
partiel. Sur une grande image RGBA opaque, cela ajoute un second parcours
complet après le décodage, via une interface relativement coûteuse.

Conséquence : temps d'ouverture et consommation CPU inutilement élevés pour de
grands PNG/TIFF opaques.

Correction appliquée : la détection automatique de l'alpha est désactivée afin
d'éviter le second parcours complet des pixels après le décodage. Le champ
`HasTransparency` est désormais faux par défaut ; les images sont donc
considérées comme opaques pour la logique d'ombre.

### Fonctionnalités ajoutées depuis la revue

- Un mode fit persistant est activé à l'ouverture ou avec `R` / le retour de
  `Z` vers le fit. Un déplacement ou un zoom manuel le désactive. Le fit est
  recalculé lors d'un redimensionnement ou d'un passage plein écran/fenêtre,
  sans rejouer l'animation de zoom.
- Le glisser-déposer d'une image unique cible A dans la moitié gauche de la
  fenêtre et B dans la moitié droite. Sans image A, toute la fenêtre cible A.
- En mode comparaison, `Ctrl + molette` règle l'opacité de l'image révélée par
  le masque entre 0 et 100 %. Le rendu circulaire utilise des bandes contiguës
  pour éviter les lignes verticales à opacité partielle.

### [Faible] Du code de clipping transformé semble devenu inaccessible

Références :

- `internal/render/comparison.go:25-47`
- `internal/render/comparison.go:248-309`
- `internal/render/comparison.go:374-380`

`DrawCompare` intercepte toutes les rotations et tous les `MirrorScale` avant
d'appeler `drawClippedCompareImage`. La branche transformée interne de cette
dernière fonction paraît donc ne plus être utilisée par le flux actuel. Elle
duplique une ancienne logique de conversion par quart de tour et augmente le
risque qu'un futur changement corrige un chemin mais pas l'autre.

Recommandation : confirmer avec des tests ou une instrumentation, puis supprimer
le chemin mort et réduire les paramètres des fonctions restantes.

### [Faible] Les sorties par `os.Exit` contournent toute fermeture future

Références : `internal/app/commands.go:96` et `internal/app/commands.go:193`

`os.Exit(0)` termine immédiatement le processus sans exécuter les `defer`. Cela
n'a pas encore d'impact majeur, car l'application ne persiste pas de préférences,
mais deviendra problématique dès qu'un fichier de configuration, un journal ou
une opération de sauvegarde sera ajouté.

Recommandation : retourner une erreur sentinelle reconnue par la boucle de jeu,
ou demander une fermeture propre de la fenêtre.

## Couverture de tests

Les quatre tests existants couvrent utilement les transformations du slider,
mais aucun test ne couvre actuellement :

- le chargement, l'invalidation et l'ordre des résultats asynchrones ;
- la navigation et le cache de préchargement ;
- les modes temporaires `1` / `2` ;
- le redimensionnement et la restauration du slider ;
- le rendu pixel à pixel des masques ;
- le flou après rotation ou miroir ;
- les scripts de build et la conversion de version ;
- le décodage de chaque format annoncé.

Priorité suggérée : commencer par des tests de logique sans GPU, puis ajouter
quelques tests de rendu sur de petites textures synthétiques (couleurs pleines,
ratios différents et positions connues).

## Points positifs

- Le découpage récent rend les responsabilités beaucoup plus faciles à suivre.
- Les résultats asynchrones utilisent des identifiants pour éviter d'afficher
  une requête devenue obsolète.
- Les images CPU et textures GPU ont des chemins explicites de libération.
- La position du masque est exprimée dans un repère local commun pendant les
  rotations et miroirs.
- Les scripts injectent une version cohérente dans le programme et les
  métadonnées Windows.
- `go test`, `go vet` et la vérification du diff passent sans erreur.

## Ordre de correction recommandé

1. Corriger la restauration après les touches `1` et `2`.
2. Borner/annuler les décodages devenus obsolètes.
3. Unifier le rendu feathered avec le clipping transformé.
4. Définir une politique claire pour les animations simultanées.
5. Sécuriser le nettoyage du script PowerShell.
6. Étendre les tests avant d'ajouter de nouveaux modes de comparaison.
