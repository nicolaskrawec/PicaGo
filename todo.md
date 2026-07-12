# PicaGo - pistes d'amelioration

Ce document regroupe les fonctions absentes et les améliorations qui me
semblent les plus utiles. L'ordre proposé privilégie d'abord la fiabilité du
viewer et de la comparaison, puis le confort d'utilisation et la distribution.

## Priorité haute - fiabilité

- [ ] Ajouter des tests visuels de non-régression pour les modes de comparaison.
  Couvrir les splits horizontal et vertical, le cercle, l'inversion A/B, les
  rotations, les miroirs et surtout les images ayant des ratios différents.
- [ ] Tester les animations image par image. Vérifier que le masque, la ligne et
  les deux images restent alignés pendant toute une rotation ou un miroir, pas
  uniquement à l'état final.
- [ ] Prendre en compte l'orientation EXIF des JPEG. Une photo issue d'un appareil
  ou d'un téléphone doit s'afficher directement dans le bon sens.
- [ ] Renforcer la gestion des images très grandes. Définir une limite mémoire,
  éviter les allocations excessives et afficher une erreur claire si une image
  ne peut pas être chargée.
- [ ] Remplacer le commutateur de diagnostic temporaire
  `fullResolutionUploadEnabled` par une vraie stratégie documentée de preview et
  de chargement haute résolution.
- [ ] Ajouter des tests aux décodeurs, à la navigation dans les dossiers et au
  calcul de version des scripts de build.

## Priorité haute - expérience utilisateur

- [ ] Sauvegarder les préférences entre deux lancements : ombre, flou, synchro du
  slider, dernier mode de comparaison, plein écran et éventuellement niveau de
  zoom par défaut.
- [ ] Ajouter un petit écran d'accueil lorsqu'aucune image n'est ouverte, avec les
  actions possibles : ouvrir, glisser-déposer et afficher l'aide.
- [ ] Ajouter une commande « Ouvrir » avec un sélecteur de fichier (`Ctrl+O`) en
  complément du lancement par argument et du glisser-déposer.
- [ ] Rendre l'aide `F1` plus lisible avec des groupes (navigation, affichage,
  comparaison) et un fond semi-transparent.
- [ ] Afficher des retours temporaires discrets pour les actions : rotation,
  miroir, zoom 100 %, fit, ombre, flou et synchro activée/désactivée.
- [ ] Clarifier visuellement les zones interactives des coins et de la barre
  basse sans alourdir l'interface.

## Comparaison d'images

- [ ] Permettre de remplacer explicitement l'image A ou l'image B sans devoir
  reconstruire la comparaison par l'ordre du glisser-déposer.
- [ ] Ajouter une commande pour échanger A et B sans changer le masque ni le
  cadrage.
- [ ] Ajouter un mode clignotement A/B avec une fréquence réglable, utile pour
  repérer rapidement de petites différences.
- [ ] Ajouter un mode différence visuelle (diff absolue ou surimpression avec
  opacité réglable).
- [ ] Pouvoir verrouiller ou déverrouiller séparément le zoom, le déplacement et
  la rotation des deux images.
- [ ] Afficher les dimensions de A et B et signaler clairement lorsque leurs
  tailles ou ratios diffèrent.
- [ ] Permettre un réglage précis du slider au clavier et une remise au centre en
  une touche.

## Navigation et consultation

- [ ] Ajouter une petite bande de miniatures optionnelle pour se déplacer dans un
  dossier sans perdre le mode plein écran.
- [ ] Proposer un tri par nom, date, taille ou ordre naturel des nombres.
- [ ] Ajouter le passage à la première/dernière image (`Home` / `End`).
- [ ] Ajouter un diaporama simple avec délai configurable.
- [ ] Afficher les informations utiles de l'image à la demande : dimensions,
  poids, format, date et principales métadonnées EXIF.
- [ ] Ajouter une commande pour copier l'image ou son chemin dans le presse-papiers.
- [ ] Prévoir une option de rechargement lorsque le fichier est modifié sur le
  disque, pratique pour vérifier un export en cours de travail.

## Accessibilité et personnalisation

- [ ] Permettre de reconfigurer les raccourcis clavier et les actions souris.
- [ ] Ajouter des réglages pour la couleur, l'épaisseur et la visibilité de la
  ligne de comparaison.
- [ ] Proposer un fond clair, sombre, quadrillé ou personnalisé pour les images
  transparentes.
- [ ] Vérifier le comportement avec les mises à l'échelle Windows élevées et les
  configurations multi-écrans.
- [ ] Ajouter une option pour réduire ou désactiver les animations.
- [ ] Prévoir une traduction française/anglaise de l'aide et des messages.

## Plateformes et distribution

- [ ] Ajouter une CI qui compile et teste automatiquement Windows, Linux et macOS
  sur chaque pull request et chaque tag.
- [ ] Publier automatiquement les exécutables et leurs sommes SHA-256 lors de la
  création d'un tag Git.
- [ ] Créer un installateur Windows avec association optionnelle aux formats
  d'image et raccourci dans le menu Démarrer.
- [ ] Fournir un paquet Linux simple (AppImage, `.deb` ou archive autonome) et un
  fichier `.desktop` avec associations MIME.
- [ ] Signer les exécutables Windows et, si une version macOS est distribuée,
  prévoir signature et notarisation.
- [ ] Vérifier le script Bash dans une vraie CI Linux, notamment la compilation
  croisée et la génération du `VERSIONINFO` Windows.

## Maintenance du projet

- [ ] Découper progressivement `internal/app/viewer.go` en modules plus ciblés :
  chargement, navigation, comparaison, animations, commandes et fenêtre.
- [ ] Centraliser les raccourcis dans une table unique utilisée à la fois par
  l'application, l'aide `F1` et le README afin d'éviter les divergences.
- [ ] Documenter les invariants du rendu comparatif : repère commun, échelle,
  rotation, masque et comportement avec des ratios différents.
- [ ] Ajouter un journal de versions (`CHANGELOG.md`) et une procédure de release.
- [ ] Ajouter une licence au dépôt et préciser les règles de contribution.

## Idées à plus long terme

- [ ] Gestion des profils colorimétriques ICC pour une comparaison plus fiable.
- [ ] Prise en charge de formats photo supplémentaires selon les besoins (AVIF,
  HEIC, éventuellement RAW via une dépendance dédiée).
- [ ] Outils de mesure simples : coordonnées du pixel, valeur RGB et distance.
- [ ] Export d'une capture de comparaison avec le masque ou la différence visible.

