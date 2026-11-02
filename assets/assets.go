// Package assets is the port's own art, embedded in the game: pictures
// that are not the original's (those stay in its LBX files).
//
//   - lobby/button.png: the online lobby's metal buttons, per state (normal, hover, pressed,
//     disabled) a row 24 px high of the left cap, an 8 px middle tile and
//     the right cap; cut by tools/buttoncut from a generated sheet.
//   - lobby/font: the lobby's Unicode text (chat, games, players), Fusion
//     Pixel Font 12 px (OFL; see its README).
package assets

import _ "embed"

// LobbyButton is lobby/button.png.
//
//go:embed lobby/button.png
var LobbyButton []byte

// LobbyFont is the lobby's TrueType font.
//
//go:embed lobby/font/fusion-pixel-12px-proportional-zh_hans.ttf
var LobbyFont []byte
