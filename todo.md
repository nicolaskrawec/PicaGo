# PicaGo - feuille de route

Cette liste reflète l'état du projet après la revue du 12 juillet 2026. Les
éléments terminés sont conservés comme historique ; les cases ouvertes sont
les prochaines actions proposées.

## Réalisé

- [x] Découper `internal/app/viewer.go` en modules ciblés (chargement,
  navigation, comparaison, animations, commandes, fenêtre et UI).
- [x] Ajouter le mode fit persistant, recalculé au redimensionnement et lors
  du passage en plein écran.
- [x] Permettre le remplacement direct de A ou B par glisser-déposer selon la
  moitié de la fenêtre.
- [x] Ajouter l'opacité du masque et le redimensionnement du masque circulaire.
- [x] Ajouter les réglages gamma, exposition/luminosité et contraste.
- [x] Ajouter le diaporama avec intervalle configurable.
- [x] Persister la configuration utilisateur (fond, ombre, diaporama,
  animation au démarrage et aide de diagnostic), avec migration de l'ancien
  fichier local.
- [x] Organiser l'aide `F1` par catégories et la documenter en anglais.
- [x] Corriger le comportement temporaire des touches `1` et `2`.
- [x] Ajouter les retours temporaires au centre de l'écran pour les actions
  principales.
- [x] Préparer les builds multi-plateformes et l'installeur Windows avec
  associations de fichiers optionnelles.
- [x] Ajouter des tests ciblés sur les transformations du slider et la gamma.

## Priorité haute - fiabilité

- [ ] Borner et annuler les décodages devenus obsolètes (`context.Context` ou
  génération), en donnant la priorité à l'image demandée sur le préchargement.
- [ ] Conserver le feathering du split après une rotation ou un miroir, et
  unifier les chemins de rendu transformé et stable.
- [ ] Définir une composition unique des animations simultanées pour éviter
  qu'une rotation, un miroir ou un fit n'écrase l'état d'une autre animation.
- [ ] Renforcer la gestion des images très grandes : limite mémoire, allocations
  bornées et message d'erreur explicite.
- [x] Remplacer `fullResolutionUploadEnabled` par une stratégie documentée de
  preview et de chargement haute résolution.
- [ ] Corriger le nettoyage du script PowerShell avec `try/finally` et
  restauration des variables d'environnement en cas d'échec.
- [ ] Supprimer la double déclaration de `Get-WindowsVersion` et tester le
  calcul de version (`v1.2.3`, build après tag, `dev`, limites de VERSIONINFO).
- [ ] Ajouter des tests de chargement asynchrone, navigation, cache,
  redimensionnement, décodage des formats et scripts de build.
- [ ] Ajouter des tests visuels de non-régression pour les splits horizontal et
  vertical, le cercle, l'inversion, les rotations, les miroirs et les ratios
  d'image différents.
- [ ] Tester les animations image par image, notamment l'alignement du masque
  et de la ligne pendant une rotation ou un miroir.
- [ ] Prendre en compte l'orientation EXIF des JPEG.

## Expérience utilisateur

- [ ] Ajouter un écran d'accueil lorsque aucune image n'est ouverte, avec ouvrir,
  glisser-déposer et aide.
- [ ] Ajouter une commande « Ouvrir » avec sélecteur de fichier (`Ctrl+O`).
- [ ] Étendre la configuration aux préférences de synchro, mode de comparaison,
  plein écran et zoom par défaut.
- [ ] Clarifier les zones interactives des coins et de la barre basse.
- [ ] Ajouter un réglage pour désactiver ou réduire les animations.

## Comparaison d'images

- [ ] Ajouter une commande explicite pour échanger A et B sans modifier le
  masque ni le cadrage.
- [ ] Ajouter un mode clignotement A/B à fréquence réglable.
- [ ] Ajouter un mode différence visuelle (diff absolue ou surimpression).
- [ ] Pouvoir verrouiller séparément zoom, déplacement et rotation de A et B.
- [ ] Afficher les dimensions de A et B et signaler les ratios différents.
- [ ] Permettre le réglage précis du slider au clavier et son recentrage en une
  touche.

## Navigation et consultation

- [ ] Ajouter une bande de miniatures optionnelle.
- [ ] Proposer un tri par nom, date, taille ou ordre naturel.
- [ ] Ajouter la première/dernière image (`Home` / `End`).
- [ ] Afficher à la demande dimensions, poids, format, date et métadonnées EXIF.
- [ ] Copier l'image ou son chemin dans le presse-papiers.
- [ ] Recharger une image lorsqu'elle est modifiée sur le disque.

## Accessibilité, distribution et maintenance

- [ ] Permettre de reconfigurer les raccourcis clavier et les actions souris.
- [ ] Personnaliser la ligne de comparaison et le fond des images transparentes.
- [ ] Vérifier les forts facteurs d'échelle Windows et les configurations
  multi-écrans.
- [ ] Ajouter une CI Windows/Linux/macOS sur chaque pull request et chaque tag.
- [ ] Publier les binaires et leurs sommes SHA-256 à chaque tag.
- [ ] Vérifier le script Bash dans une vraie CI Linux.
- [ ] Ajouter `CHANGELOG.md`, une procédure de release, une licence et les
  règles de contribution.

## Idées à plus long terme

- [ ] Gestion des profils colorimétriques ICC.
- [ ] Formats supplémentaires selon les besoins (AVIF, HEIC, RAW).
- [ ] Outils de mesure : coordonnées du pixel, RGB et distance.
- [ ] Export d'une capture de comparaison avec masque ou différence visible.
