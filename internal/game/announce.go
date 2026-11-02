package game

import (
	"strings"

	"github.com/openorion2/openorion2/internal/announce"
	"github.com/openorion2/openorion2/internal/lobby"
)

// port-only: the project's announcements (docs/announce.md, internal/announce):
// a line on the main menu, a note in the lobby's chat, and in a game played
// through the lobby a notice on the main screen, each new one once.

// announceLobbyText is m's text for the lobby, whose font has Unicode:
// without control characters.
func announceLobbyText(m announce.Message) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f || r >= 0x80 && r < 0xa0 {
			return -1
		}
		return r
	}, m.Text)
}

// announceText is m's text in what the game's own fonts can print: ASCII,
// without control characters.
func announceText(m announce.Message) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r > 0x7e {
			return -1
		}
		return r
	}, m.Text)
}

// drawAnnouncement prints the announcements on the main menu, above its
// picture: one line, cut to the screen's width.
func (g *Game) drawAnnouncement() error {
	ms := g.announceFeed.Active()
	if len(ms) == 0 {
		return nil
	}
	var parts []string
	warning := false
	for _, m := range ms {
		if s := announceText(m); s != "" {
			parts = append(parts, s)
		}
		warning = warning || m.Warning()
	}
	line := strings.Join(strings.Fields(strings.Join(parts, "  -  ")), " ")
	t := g.e.Text()
	if warning {
		t.SetStyle(1, g.exeColors(creditsNormal)) // the credits' orange
	} else {
		t.SetStyle(1, fontColors2(1, announceFrame, announceFrame))
	}
	for len(line) > 3 && t.Width([]byte(line)) > announceMaxWidth {
		line = strings.TrimRight(line[:len(line)-4], " ") + "..."
	}
	if line == "" {
		return nil
	}
	// On black in a green frame: the stars behind would hide the letters.
	w, h := t.Width([]byte(line)), t.Font.Styles[1].Height
	x1, y1, x2, y2 := 320-w/2-6, announceY-4, 320+w/2+6, announceY+h+3
	v := g.e.V
	v.Fill(x1, y1, x2, y2, announceFrame)
	v.Fill(x1+1, y1+1, x2-1, y2-1, 0)
	return t.PrintCentered(320, announceY, line)
}

const (
	announceY        = 0x28 // between the frame and the title
	announceFrame    = 0xb4 // the menu's green
	announceMaxWidth = 590
)

// announceNew is the active announcements not in seen yet with something
// text keeps, which it marks.
func (g *Game) announceNew(seen map[string]bool, text func(announce.Message) string) []announce.Message {
	var out []announce.Message
	for _, m := range g.announceFeed.Active() {
		if k := m.Key(); !seen[k] && strings.TrimSpace(text(m)) != "" {
			seen[k] = true
			out = append(out, m)
		}
	}
	return out
}

// announceToLobby notes the new announcements in the lobby's chat.
func (g *Game) announceToLobby(st *lobby.State) {
	if g.announceLobby == nil {
		g.announceLobby = map[string]bool{}
	}
	for _, m := range g.announceNew(g.announceLobby, announceLobbyText) {
		st.Note("Announcement: " + announceLobbyText(m))
	}
}

// announceInGame shows the new announcements in a game played through the
// lobby (netNotice: a GNN line while waiting, a box on the main screen).
func (g *Game) announceInGame() {
	if _, ok := g.net.network.(*lobby.Network); !ok {
		return
	}
	if g.announceGame == nil {
		g.announceGame = map[string]bool{}
	}
	for _, m := range g.announceNew(g.announceGame, announceText) {
		g.netNotice("Announcement: " + announceText(m))
	}
}
