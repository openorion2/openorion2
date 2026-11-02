# OpenOrion2

[English](README.md) · [Русский](README.ru.md) · [中文](README.zh-CN.md) · **Deutsch** · [Français](README.fr.md)

OpenOrion2 ist eine quelloffene Neuimplementierung von **Master of Orion
II: Battle at Antares** (1996): das Spiel als Quellcode, den jeder lesen,
korrigieren, verändern und am Leben halten kann. Es läuft auf aktuellem
macOS, Windows und Linux ohne DOSBox. Es spielt sich genau wie das
Original: Die Spiellogik wurde Routine für Routine aus der originalen
ausführbaren Datei nachgebaut und mit ihr abgeglichen, bis hin zu den
Zufallszahlen und den Entscheidungen der KI. Spielstände des Originals
lassen sich laden und weiterspielen.

Über das Original hinaus bietet es:

- native Builds für macOS, Windows und Linux, auf x86-64 und ARM (Apple
  Silicon eingeschlossen): eine Datei, kein DOSBox, kein Emulator, nichts
  zu installieren;
- Netzwerkspiel über TCP (statt IPX), im LAN oder über das Internet;
- den **[Galactic Council](https://openorion2.org)**, eine Online-Lobby:
  Chat, private Nachrichten und eine Liste der Spiele, die der Server
  weiterleitet, sodass keine Portweiterleitung nötig ist;
- Vollbild (Fenster maximieren oder Alt+Enter) und ein Fenster beliebiger
  Größe;
- eine Browser-Version (WebAssembly).

Was portiert ist und was nicht: [docs/STATUS.md](docs/STATUS.md) (auf
Englisch).

> OpenOrion2 ist ein Fanprojekt. Es steht in keiner Verbindung zu den
> Inhabern von Master of Orion und wird nicht von ihnen unterstützt.
> Master of Orion ist eine Marke ihres jeweiligen Inhabers. OpenOrion2
> enthält keine Dateien des Originalspiels: Du brauchst deine eigene Kopie
> von Master of Orion II.

## Warum Go?

Go ist die Hauptsprache des Projektautors, und den KI-Agenten, die einen
Großteil der Arbeit erledigen, ist jede Sprache gleich recht. Es geht nicht
um die Sprache, sondern darum, dass das Spiel offen ist.

## Installation

OpenOrion2 ist das Programm; die Spieldaten kommen aus deiner eigenen
Kopie von Master of Orion II. Es ist für **Version 1.40b23, die
[GOG](https://www.gog.com/en/game/master_of_orion_1_2)-Ausgabe** gemacht
und damit getestet. Andere Ausgaben (die CD von 1996, 1.31, andere Shops)
sind nicht getestet.

1. **Hol dir das Spiel.** Kauf Master of Orion II bei GOG, wo es als
   [Master of Orion 1+2](https://www.gog.com/en/game/master_of_orion_1_2)
   angeboten wird, und installiere es.
2. **Hol dir OpenOrion2.** Lade das Binary für dein System aus den
   [Releases](https://github.com/openorion2/openorion2/releases) herunter
   (`openorion2-<system>-<cpu>`: `darwin` ist macOS; `arm64` ist Apple
   Silicon und ARM-PCs, `amd64` der Rest) oder baue es selbst (siehe
   unten).
3. **Leg es in das Verzeichnis des Spiels**, das mit den `.LBX`-Dateien
   (unter Windows das GOG-Installationsverzeichnis), und starte es dort:
   Es findet die Spieldateien neben sich. Unter macOS oder Linux kopierst
   du dieses Verzeichnis von einer Windows-Installation oder entpackst den
   GOG-Installer für Windows mit
   [innoextract](https://constexpr.org/innoextract/).
4. **Der erste Start.** Die Binaries sind nicht von Apple oder Microsoft
   signiert. Unter macOS entfernst du nach `chmod +x openorion2-darwin-*`
   die Download-Quarantäne mit `xattr -d com.apple.quarantine
   openorion2-darwin-*` (oder, nachdem macOS den Start einmal verweigert
   hat: Systemeinstellungen, Datenschutz & Sicherheit, „Dennoch öffnen“).
   Unter Windows in SmartScreen „Weitere Informationen“, „Trotzdem
   ausführen“.

OpenOrion2 findet die Spieldateien auch im aktuellen Verzeichnis oder in
einem Verzeichnis namens `game` daneben; liegen sie woanders, gib den Pfad
an:

```
openorion2 -game "/path/to/Master of Orion 2"
```

OpenOrion2 schreibt seine Spielstände (`SAVE1.GAM` … `SAVE10.GAM`), die
Einstellungen und die Ruhmeshalle wie das Original in das Verzeichnis des
Spiels und liest dort die Spielstände des Originals. Arbeite mit einer
Kopie, wenn du die Spielstände des Originals getrennt halten willst.

Ein Release-Build sucht beim Start nach einer neueren Version und bietet
an, sie zu installieren (`-noupdate` schaltet das ab).

## Bauen

Du brauchst Go 1.27 oder neuer. SDL3 ist in das Binary eingebettet: Sonst
ist nichts zu installieren.

```
go build ./cmd/openorion2
./openorion2 -game /path/to/game
```

`go run ./cmd/openorion2 -h` listet die Optionen auf.

Eigene Builds suchen nie nach Updates. Wie ein Release entsteht:
[docs/release.md](docs/release.md).

## Mit anderen spielen

- **Hotseat** (an einem Rechner): Multiplayer, Hotseat.
- **Netzwerk**: Multiplayer, Network. Start New Game lauscht auf TCP-Port
  52525 (`-host :port` wählt einen anderen); Join Game findet Spiele im
  LAN, oder `-join host[:port]` tritt einem direkt bei. Siehe
  [docs/net/ui.md](docs/net/ui.md).
- **Galactic Council Online Lobby**: Registriere dich auf
  [openorion2.org](https://openorion2.org), dann Multiplayer, Galactic
  Council Online Lobby. Erstelle dort ein Spiel oder tritt einem bei; die
  Lobby leitet es weiter, sodass niemand einen offenen Port braucht.
  Private Nachrichten: auf einen Namen klicken oder `/w name text`
  eingeben.

## Browser-Version

OpenOrion2 läuft auch im Browser, als WebAssembly: derselbe Spielcode,
Spielstände im Browser und Netzwerkspiel mit nativen Spielern.
[tools/web/build.sh](tools/web/build.sh) baut sie, und
[cmd/webserve](cmd/webserve) liefert sie zusammen mit deinen Spieldateien
aus: [docs/web.md](docs/web.md).

Eine öffentliche Seite zum Spielen gibt es nicht. Die Browser-Version
braucht die Dateien des Originals auf dem Server, von dem sie geladen wird,
und sie jedem auszuliefern, der eine Seite öffnet, hieße das Spiel zu
verteilen, wozu dieses Projekt kein Recht hat. Betreibe sie für dich
selbst, auf deinem eigenen Rechner oder hinter `webserve -auth` für deine
Freunde, mit deiner eigenen Kopie des Spiels.

## Pläne

- Den Quellcode des Online-Lobby-Servers öffnen, damit jeder einen
  betreiben kann.
- So viel wie möglich vom inoffiziellen 1.50-Patch der Community
  übernehmen.
- Die macOS-Builds signieren und notarisieren, damit macOS sie ohne
  Warnung startet.

## Dokumentation

[docs/](docs) enthält die Spezifikationen, nach denen das Spiel
nachgebaut wurde: die Dateiformate, die Engine, jeder Bildschirm, der Zug,
der Kampf, das Netzwerkspiel und die Lobby, mit den Namen und Adressen der
Routinen des Originals (auf Englisch).

## Lizenz

OpenOrion2 ist freie Software unter der GNU General Public License,
Version 3: siehe [LICENSE](LICENSE).

Die eigene Grafik des Ports ([assets/](assets)) enthält nichts aus dem
Originalspiel. Sie enthält die Fusion Pixel Font (SIL Open Font License
1.1, [assets/lobby/font/](assets/lobby/font)), und das Binary bettet SDL3
(zlib-Lizenz) über [go-sdl3](https://github.com/Zyko0/go-sdl3) ein.
