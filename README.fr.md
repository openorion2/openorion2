# OpenOrion2

[English](README.md) · [Русский](README.ru.md) · [中文](README.zh-CN.md) · [Deutsch](README.de.md) · **Français**

![OpenOrion2](https://github.com/openorion2/openorion2/releases/download/v0.9.0/openorion2-play.gif)

OpenOrion2 est une réimplémentation libre de **Master of Orion II: Battle
at Antares** (1996) : le jeu sous forme de code source que chacun peut
lire, corriger, modifier et faire vivre. Il tourne sur macOS, Windows et
Linux actuels, sans DOSBox. Il se joue exactement comme l'original : la
logique du jeu a été reconstruite routine par routine à partir de
l'exécutable d'origine et vérifiée contre lui, jusqu'aux tirages du
générateur aléatoire et aux décisions de l'IA. Les sauvegardes de
l'original se chargent et se poursuivent.

Par-dessus l'original, il ajoute :

- des versions natives pour macOS, Windows et Linux, en x86-64 et ARM
  (Apple Silicon compris) : un seul fichier, sans DOSBox, sans émulateur,
  rien à installer ;
- le jeu en réseau par TCP (à la place d'IPX), en réseau local ou par
  internet ;
- le **[Galactic Council](https://openorion2.org)**, un salon en ligne :
  chat, messages privés et liste des parties, relayées par le serveur, si
  bien qu'aucune redirection de port n'est nécessaire ;
- le plein écran (agrandir la fenêtre, ou Alt+Entrée) et une fenêtre de
  n'importe quelle taille ;
- une version navigateur (WebAssembly).

Ce qui est porté et ce qui ne l'est pas : [docs/STATUS.md](docs/STATUS.md)
(en anglais).

> OpenOrion2 est un projet de fans. Il n'est ni affilié aux propriétaires
> de Master of Orion ni approuvé par eux. Master of Orion est une marque de
> son propriétaire. OpenOrion2 ne contient aucun fichier du jeu original :
> il vous faut votre propre exemplaire de Master of Orion II.

## Pourquoi Go ?

Go est le langage principal de l'auteur du projet, et les agents d'IA qui
font une grande partie du travail écrivent aussi bien dans n'importe quel
langage. L'essentiel n'est pas le langage, mais que le jeu soit ouvert.

## Installation

OpenOrion2 est le programme ; les données du jeu viennent de votre propre
exemplaire de Master of Orion II. Il est conçu et testé pour **la version
1.40b23, celle de [GOG](https://www.gog.com/en/game/master_of_orion_1_2)**.
Les autres éditions (le CD de 1996, la 1.31, les autres boutiques) ne sont
pas testées.

1. **Procurez-vous le jeu.** Achetez Master of Orion II sur GOG, où il est
   vendu sous le nom
   [Master of Orion 1+2](https://www.gog.com/en/game/master_of_orion_1_2),
   et installez-le.
2. **Procurez-vous OpenOrion2.** Téléchargez le binaire pour votre système
   dans les [releases](https://github.com/openorion2/openorion2/releases)
   (`openorion2-<système>-<processeur>` : `darwin` est macOS ; `arm64`
   correspond à Apple Silicon et aux PC ARM, `amd64` au reste), ou
   compilez-le (voir plus bas).
3. **Placez-le dans le répertoire du jeu**, celui qui contient les
   fichiers `.LBX` (sous Windows, le répertoire d'installation de GOG), et
   lancez-le depuis là : il trouve les fichiers du jeu à côté de lui. Sous
   macOS ou Linux, copiez ce répertoire depuis une installation Windows, ou
   décompressez l'installateur GOG pour Windows avec
   [innoextract](https://constexpr.org/innoextract/).
4. **Le premier lancement.** Les binaires ne sont pas signés par Apple ni
   par Microsoft. Sous macOS, après `chmod +x openorion2-darwin-*`, retirez
   la quarantaine de téléchargement avec `xattr -d com.apple.quarantine
   openorion2-darwin-*` (ou, après un premier refus de macOS : Réglages
   Système, Confidentialité et sécurité, « Ouvrir quand même »). Sous
   Windows, dans SmartScreen : « Informations complémentaires », «
   Exécuter quand même ».

OpenOrion2 trouve aussi les fichiers du jeu dans le répertoire courant ou
dans un répertoire nommé `game` à côté de lui ; s'ils sont ailleurs,
indiquez le chemin :

```
openorion2 -game "/path/to/Master of Orion 2"
```

OpenOrion2 écrit ses sauvegardes (`SAVE1.GAM` … `SAVE10.GAM`), les
réglages et le Panthéon dans le répertoire du jeu, comme l'original, et y
lit les sauvegardes de l'original. Travaillez sur une copie si vous voulez
garder les sauvegardes de l'original à part.

Une version publiée cherche au démarrage une version plus récente et
propose de l'installer (`-noupdate` désactive cela).

## Compilation

Il faut Go 1.27 ou plus récent. SDL3 est intégré au binaire : il n'y a
rien d'autre à installer.

```
go build ./cmd/openorion2
./openorion2 -game /path/to/game
```

`go run ./cmd/openorion2 -h` liste les options.

Les versions que vous compilez vous-même ne cherchent jamais de mises à
jour. Publier une version : [docs/release.md](docs/release.md).

## Jouer à plusieurs

- **Hotseat** (sur un même ordinateur) : Multiplayer, Hotseat.
- **Réseau** : Multiplayer, Network. Start New Game écoute sur le port TCP
  52525 (`-host :port` en choisit un autre) ; Join Game trouve les parties
  du réseau local, ou `-join hôte[:port]` en rejoint une directement. Voir
  [docs/net/ui.md](docs/net/ui.md).
- **Galactic Council Online Lobby** : inscrivez-vous sur
  [openorion2.org](https://openorion2.org), puis Multiplayer, Galactic
  Council Online Lobby. Créez une partie ou rejoignez-en une ; le salon la
  relaie, donc personne n'a besoin d'ouvrir un port. Messages privés :
  cliquez sur un nom, ou tapez `/w nom texte`.

## Version navigateur

OpenOrion2 tourne aussi dans un navigateur, en WebAssembly : le même code
de jeu, les sauvegardes gardées dans le navigateur, et le jeu en réseau
avec les joueurs des versions natives.
[tools/web/build.sh](tools/web/build.sh) la compile et
[cmd/webserve](cmd/webserve) la sert avec vos fichiers du jeu :
[docs/web.md](docs/web.md).

Il n'existe pas de site public pour y jouer. La version navigateur a
besoin des fichiers de l'original sur le serveur d'où elle est chargée, et
les servir à quiconque ouvre la page reviendrait à distribuer le jeu, ce
que ce projet n'a pas le droit de faire. Hébergez-la pour vous-même, sur
votre machine ou derrière `webserve -auth` pour vos amis, avec votre
propre exemplaire du jeu.

## Projets

- Ouvrir le code source du serveur du salon en ligne, pour que chacun
  puisse en faire tourner un.
- Intégrer autant que possible le patch non officiel 1.50 de la
  communauté.
- Signer et notariser les versions macOS, pour que macOS les lance sans
  avertissement.

## Documentation

[docs/](docs) contient les spécifications d'après lesquelles le jeu a été
reconstruit : les formats de fichiers, le moteur, chaque écran, le tour,
le combat, le jeu en réseau et le salon, avec les noms et adresses des
routines de l'original (en anglais).

- [ARCHITECTURE.md](ARCHITECTURE.md) : une carte du code, et par où commencer dans chaque partie (en anglais).
- [CONTRIBUTING.md](CONTRIBUTING.md) : compiler, tester, les règles, et comment modifier sans risque une routine portée (en anglais).
- [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) : le code de conduite, le Contributor Covenant (en anglais).

## Licence

OpenOrion2 est un logiciel libre sous la GNU General Public License,
version 3 : voir [LICENSE](LICENSE).

Les graphismes propres au portage ([assets/](assets)) ne contiennent rien
du jeu original. Ils incluent la police Fusion Pixel Font (SIL Open Font
License 1.1, [assets/lobby/font/](assets/lobby/font)), et le binaire
intègre SDL3 (licence zlib) via
[go-sdl3](https://github.com/Zyko0/go-sdl3).
