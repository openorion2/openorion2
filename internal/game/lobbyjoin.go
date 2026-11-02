package game

import (
	"fmt"
	"strings"

	"github.com/openorion2/openorion2/internal/host"
	"github.com/openorion2/openorion2/lobbynet/lobbyv1"
)

// port-only: the lobby's JOIN box, over its screen: the open games (a
// click picks one) and a line for a game's id (a private game's, typed or
// pasted); JOIN joins the id if one is typed, else the picked game; Enter
// is JOIN, Esc and CANCEL go back to the lobby. The list follows the
// lobby as games open and start, and scrolls (the wheel, PgUp/PgDn,
// Home/End; Up/Down move the pick).

const (
	ljX1, ljY1, ljX2, ljY2 = 0x60, 0x40, 0x21f, 0x15f
	ljRows                 = 9
	ljRowsY                = ljY1 + 0x18
	ljIDY                  = ljRowsY + ljRows*uniLineH + 0x14 // the id line's top
	ljButtonY              = ljY2 - buttonArtH - 6
	ljButtonW              = 0x60
	ljBarX                 = ljX2 - 6 // the list's scroll bar
)

type lobbyJoinScr struct {
	id             uniLine
	rowF           [ljRows]int
	joinF, cancelF int
	sel            string // the picked game's id, "" none
	top            int    // the game on the first row
	hover          int
	note           string
}

func ljButtonX(i int) int {
	mid := (ljX1 + ljX2) / 2
	if i == 0 {
		return mid - 8 - ljButtonW
	}
	return mid + 8
}

func (g *Game) addLobbyJoinFields(j *lobbyJoinScr) error {
	e := g.e
	e.ClearFields()
	var err error
	for i := range j.rowF {
		y := ljRowsY + i*uniLineH
		if j.rowF[i], err = e.AddHiddenField(ljX1+2, y, ljX2-2, y+uniLineH-1, "", 0x28); err != nil {
			return err
		}
	}
	for i, f := range []*int{&j.joinF, &j.cancelF} {
		x := ljButtonX(i)
		if *f, err = e.AddHiddenField(x, ljButtonY, x+ljButtonW-1, ljButtonY+buttonArtH-1, "", 0x28); err != nil {
			return err
		}
	}
	return nil
}

func (g *Game) drawLobbyJoin(s *lobbyHomeScr, j *lobbyJoinScr) error {
	if err := g.drawLobbyHome(s); err != nil {
		return err
	}
	games := s.conn.State.Games
	g.lobbyPanel(ljX1, ljY1, ljX2, ljY2)
	g.lobbyPanel(ljX1+1, ljY1+1, ljX2-1, ljY2-1) // a double rim over the chat
	g.uniPrintBold(ljX1+8, ljY1+4, ljX2-8, "JOIN A GAME", lhTextLit)
	if len(games) == 0 {
		g.lobbyText(ljX1+8, ljRowsY, ljX2-ljX1-16, "No open games now.", false)
	}
	j.top = max(0, min(j.top, len(games)-ljRows))
	for i := range ljRows {
		if j.top+i >= len(games) {
			break
		}
		gm, y := games[j.top+i], ljRowsY+i*uniLineH
		lit := gm.Id == j.sel
		if lit {
			g.e.V.Fill(ljX1+3, y-1, ljBarX-4, y+uniLineH-2, lhRowLit)
		}
		g.lobbyText(ljX1+8, y, 0xf0, gm.Description, lit)
		g.lobbyText(ljX1+0x100, y, 0x70, gm.Creator, lit)
		g.lobbyText(ljBarX-0x2c, y, 0x24, fmt.Sprintf("%d/8", len(gm.Players)), lit)
	}
	g.lobbyScrollBar(ljBarX, ljRowsY-1, ljRowsY+ljRows*uniLineH-2, len(games), ljRows, j.top)
	g.uniPrintBold(ljX1+8, ljIDY-uniLineH-2, ljX2-8, "OR A GAME'S ID:", lhTextLit)
	g.lobbyPanel(ljX1+6, ljIDY-2, ljX2-6, ljIDY+uniLineH)
	g.drawUniLine(&j.id, ljX1+10, ljIDY, ljX2-10, lhTextLit)
	if j.note != "" {
		g.uniPrint(ljX1+8, ljIDY+uniLineH+4, ljX2-8, j.note, lhSystem)
	}
	for i, f := range []*int{&j.joinF, &j.cancelF} {
		g.lobbyButton(ljButtonX(i), ljButtonY, ljButtonW, buttonArtH, []string{"JOIN", "CANCEL"}[i], *f > 0 && j.hover == *f)
	}
	return nil
}

// lobbyJoinBox runs the JOIN box: the game joined, or nil (CANCEL, Esc, a
// refusal told).
func (g *Game) lobbyJoinBox(s *lobbyHomeScr) (*lobbyv1.GameJoined, error) {
	e, conn := g.e, s.conn
	j := &lobbyJoinScr{id: uniLine{max: 64}}
	g.lobbyJoinScr = j
	defer func() { g.lobbyJoinScr = nil }()
	if err := g.addLobbyJoinFields(j); err != nil {
		return nil, err
	}
	e.Sys.Wheel() // turned over the chat: not the list's
	for {
		var typed uniKey
		unhook := g.hookUniLine(&j.id, &typed)
		lineKey := e.KeyHook
		e.KeyHook = func(k host.Key) bool {
			return j.listKey(k, conn.State.Games) || lineKey(k)
		}
		r, err := e.GetInput()
		unhook()
		if err != nil {
			return nil, err
		}
		if j.hover, err = e.ScanInput(); err != nil {
			return nil, err
		}
		if _, err := conn.Poll(); err != nil {
			return nil, g.messageBox(lobbyErrorText("the lobby", err))
		}
		games := conn.State.Games
		if j.picked(games) < 0 {
			j.sel = "" // started or gone
		}
		j.top -= e.Sys.Wheel()
		switch {
		case typed == uniEsc, r == j.cancelF:
			return nil, nil
		case typed == uniEnter, r == j.joinF:
			id := strings.TrimSpace(j.id.String())
			if id == "" {
				id = j.sel
			}
			if id == "" {
				j.note = "Pick a game, or type a game's id."
				break
			}
			got, err := g.lobbyJoin(s, id)
			if err != nil || got != nil {
				return got, err
			}
			if err := g.addLobbyJoinFields(j); err != nil { // after the refusal's box
				return nil, err
			}
			e.Sys.Wheel()
		default:
			for i, f := range j.rowF {
				if r == f && j.top+i < len(games) {
					j.sel, j.note = games[j.top+i].Id, ""
				}
			}
		}
		e.V.SetPageOff()
		if err := g.drawLobbyJoin(s, j); err != nil {
			return nil, err
		}
		if err := e.TogglePages(); err != nil {
			return nil, err
		}
		if err := e.Sys.Sleep(lobbyFrame); err != nil {
			return nil, err
		}
	}
}

// picked is the picked game's index in games, -1 none.
func (j *lobbyJoinScr) picked(games []*lobbyv1.GameInfo) int {
	for i, gm := range games {
		if gm.Id == j.sel {
			return i
		}
	}
	return -1
}

// listKey scrolls the list by k (PgUp, PgDn, Home, End) or moves the pick
// (Up, Down) in view; false for other keys.
func (j *lobbyJoinScr) listKey(k host.Key, games []*lobbyv1.GameInfo) bool {
	switch k.Scancode {
	case 75: // PgUp
		j.top -= ljRows - 1
	case 78: // PgDn
		j.top += ljRows - 1
	case 74: // Home
		j.top = 0
	case 77: // End
		j.top = len(games)
	case 82, 81: // Up, Down
		if len(games) == 0 {
			return true
		}
		i := j.picked(games)
		switch {
		case k.Scancode == 81:
			i = min(i+1, len(games)-1)
		case i < 0:
			i = len(games) - 1
		default:
			i = max(i-1, 0)
		}
		j.sel, j.note = games[i].Id, ""
		j.top = max(min(j.top, i), i-ljRows+1)
	default:
		return false
	}
	return true
}
