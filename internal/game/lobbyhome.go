package game

import (
	"fmt"
	"strings"
	"time"

	"github.com/openorion2/openorion2/internal/host"
	"github.com/openorion2/openorion2/internal/lobby"
	"github.com/openorion2/openorion2/lobbynet/lobbyv1"
)

// port-only: the online lobby's screen once logged in (docs/lobby/client.md),
// drawn on the multiplayer frame in the frame's greens: the chat over the
// screen's height with who is online beside it, the input line, and
// CREATE (LEAVE while in a game), LOAD, JOIN (the open games and a game id,
// in a box over the chat), LOGOUT, BACK. The lobby's own news (a game
// created, a refusal) comes as system lines of the chat.

const (
	lhX1, lhX2           = 0x30, 0x24f // the panels' sides
	lhChatY, lhChatY2    = 0x2c, 0x172 // the chat and online panels
	lhChatX2, lhOnlineX  = 0x1c8, 0x1d0
	lhInputY, lhInputH   = 0x177, 0x10 // the input line
	lhButtonY, lhButtonH = 0x18a, buttonArtH
	lhButtonGap          = 8
	lhPanelFill          = 0
	// Greens of the multiplayer palette (0xb), the ramp the frame's grid
	// is drawn with: the palette is left alone, so the frame and the
	// buttons keep their colours.
	lhText      byte = 0x80
	lhTextLit   byte = 0x81
	lhSystem    byte = 0x7f // the lobby's own lines in the chat
	lhPanelLine byte = 0x7e
	lhRowLit    byte = 0x7d // the picked row
	lhButtonInk byte = 0x03 // the buttons' labels, near black
)

// lobbyHomeScr is the screen's state.
type lobbyHomeScr struct {
	conn *lobby.Conn
	rem  *lobby.Remembered

	line                                  uniLine // the chat's input line
	scroll                                int     // the chat's wrapped lines scrolled back, 0 at its end
	said                                  int     // conn.State.Said when the chat was last followed
	onlineTop                             int     // the first name shown of who is online
	createF, loadF, joinF, logoutF, backF int
	hover                                 int // the field under the pointer

	// The chat's tabs (lobbytabs.go): the dialogs' users, the shown tab (0
	// ALL), the fields of the tabs, their close marks and the names online.
	tabs         []string
	cur          int
	tabF, closeF []int
	onlineF      [lhOnlineFit]int
	fieldsSig    string // the tabs' layout the fields were made for
}

var lobbyButtons = [5]string{"CREATE", "LOAD", "JOIN", "LOGOUT", "BACK"}

// buttons are the buttons' fields, in lobbyButtons' order.
func (s *lobbyHomeScr) buttons() []*int {
	return []*int{&s.createF, &s.loadF, &s.joinF, &s.logoutF, &s.backF}
}

// lobbyButtonW is a button's width, the buttons spread over the panels'.
const lobbyButtonW = (lhX2 - lhX1 + 1 - (len(lobbyButtons)-1)*lhButtonGap) / len(lobbyButtons)

// lobbyButtonX is the left of button i.
func lobbyButtonX(i int) int { return lhX1 + i*(lobbyButtonW+lhButtonGap) }

func (g *Game) addLobbyHomeFields(s *lobbyHomeScr) error {
	e := g.e
	e.ClearFields()
	var err error
	for i, f := range s.buttons() {
		x := lobbyButtonX(i)
		if *f, err = e.AddHiddenField(x, lhButtonY, x+lobbyButtonW-1, lhButtonY+lhButtonH-1, "", 0x28); err != nil {
			return err
		}
	}
	return g.addLobbyTabFields(s)
}

// lobbyText prints s (any language: the lobby's own font) on a line whose
// top is y, from x, in the lobby's green, cut to width w.
func (g *Game) lobbyText(x, y, w int, s string, lit bool) {
	c := lhText
	if lit {
		c = lhTextLit
	}
	g.uniPrint(x, y, x+w-1, s, c)
}

func (g *Game) lobbyPanel(x1, y1, x2, y2 int) {
	v := g.e.V
	v.Fill(x1, y1, x2, y2, lhPanelFill)
	v.Line(x1, y1, x2, y1, lhPanelLine)
	v.Line(x1, y2, x2, y2, lhPanelLine)
	v.Line(x1, y1, x1, y2, lhPanelLine)
	v.Line(x2, y1, x2, y2, lhPanelLine)
}

// lobbyButton draws a button w x h at (x, y): the port's metal art
// (buttonart.go), the label dark and bold in the lobby's font.
func (g *Game) lobbyButton(x, y, w, h int, label string, lit bool) {
	state := buttonNormal
	if lit {
		state = buttonHover
	}
	g.lobbyButtonIn(x, y, w, h, label, state)
}

// lobbyButtonIn is lobbyButton in any state of the art; pressed, the label
// sinks a pixel.
func (g *Game) lobbyButtonIn(x, y, w, h int, label string, state int) {
	by := y + (h-buttonArtH)/2
	g.drawButtonArt(x, by, w, state)
	dy := 0
	if state == buttonPressed {
		dy = 1
	}
	g.uniPrintBold(x+(w-uniWidthBold(label))/2, by+(buttonArtH-uniLineH)/2+dy, x+w-1, label, lhButtonInk)
}

func (g *Game) drawLobbyHome(s *lobbyHomeScr) error {
	st := &s.conn.State
	if err := g.drawNetAddressBack(&netAddressBox{}); err != nil {
		return err
	}

	// Who is online, the player lit, scrolled by the wheel over it.
	g.lobbyPanel(lhOnlineX, lhChatY, lhX2, lhChatY2)
	g.uniPrintBold(lhOnlineX+6, lhChatY+2, lhX2-6, fmt.Sprintf("ONLINE (%d)", len(st.Online)), lhTextLit)
	s.onlineTop = max(0, min(s.onlineTop, len(st.Online)-lhOnlineFit))
	for i := range lhOnlineFit {
		if s.onlineTop+i >= len(st.Online) {
			break
		}
		n := st.Online[s.onlineTop+i]
		g.lobbyText(lhOnlineX+6, lhOnlineY+i*uniLineH, lhOnlineBarX-4-(lhOnlineX+6), n, n == s.conn.User)
	}
	g.lobbyScrollBar(lhOnlineBarX, lhOnlineY-1, lhChatY2-3, len(st.Online), lhOnlineFit, s.onlineTop)

	// The shown tab's chat, wrapped, scrolled back s.scroll lines from its
	// end, with a scroll bar once it overflows.
	g.lobbyPanel(lhX1, lhChatY, lhChatX2, lhChatY2)
	g.drawLobbyTabs(s)
	chat, _ := s.source()
	lines := lobbyChatLines(chat)
	s.scroll = max(0, min(s.scroll, len(lines)-lhChatFit))
	end := len(lines) - s.scroll
	for i := max(0, end-lhChatFit); i < end; i++ {
		y := lhChatTop + (lhChatFit-(end-i))*uniLineH
		c := lhText
		if lines[i].system {
			c = lhSystem
		}
		g.uniPrint(lhX1+6, y, lhChatBarX-4, lines[i].text, c)
	}
	g.lobbyScrollBar(lhChatBarX, lhChatTop, lhChatY2-3, len(lines), lhChatFit, len(lines)-lhChatFit-s.scroll)

	g.lobbyPanel(lhX1, lhInputY-2, lhX2, lhInputY+lhInputH)
	g.drawUniLine(&s.line, lhX1+6, lhInputY, lhX2-4, lhTextLit)

	// The buttons, lit under the pointer; CREATE is LEAVE while in a game;
	// JOIN counts the open games.
	for i, f := range s.buttons() {
		label := lobbyButtons[i]
		switch {
		case i == 0 && st.Mine != nil:
			label = "LEAVE"
		case f == &s.joinF && len(st.Games) > 0:
			label = fmt.Sprintf("JOIN (%d)", len(st.Games))
		}
		g.lobbyButton(lobbyButtonX(i), lhButtonY, lobbyButtonW, lhButtonH, label, *f > 0 && s.hover == *f)
	}
	return nil
}

// lobbyScrollBar draws a scroll bar at x from y1 to y2 over a list of n
// rows, fit of them shown from row top; nothing when they all fit.
func (g *Game) lobbyScrollBar(x, y1, y2, n, fit, top int) {
	if n <= fit {
		return
	}
	h := max(6, (y2-y1+1)*fit/n)
	t := y1 + (y2-y1+1-h)*top/(n-fit)
	g.e.V.Line(x, y1, x, y2, lhRowLit)
	g.e.V.Fill(x-1, t, x+1, t+h-1, lhText)
}

const (
	lhOnlineY    = lhChatY + 2 + uniLineH                // the first name online
	lhOnlineFit  = (lhChatY2 - 2 - lhOnlineY) / uniLineH // the names shown
	lhOnlineBarX = lhX2 - 4
	lhChatFit    = (lhChatY2 - 2 - lhChatTop) / uniLineH // the chat's lines on screen
	lhChatBarX   = lhChatX2 - 4                          // its scroll bar
	lhChatWrap   = lhChatBarX - 4 - (lhX1 + 6)           // its lines' width
	lhWheel      = 3                                     // lines a notch of the wheel scrolls
)

type lobbyChatLine struct {
	text   string
	system bool
}

// lobbyChatLines is the chat wrapped to its panel, oldest first.
func lobbyChatLines(chat []lobby.Line) []lobbyChatLine {
	var out []lobbyChatLine
	for _, c := range chat {
		text := c.User + ": " + c.Text
		if c.System {
			text = c.Text
		}
		for _, w := range uniWrap(text, lhChatWrap) {
			out = append(out, lobbyChatLine{w, c.System})
		}
	}
	return out
}

// followChat keeps the shown tab's chat scrolled back where it was as
// lines come in (at its end it follows them).
func (s *lobbyHomeScr) followChat() {
	chat, said := s.source()
	n := said - s.said
	s.said = said
	if n < 0 { // a new snapshot
		s.scroll = 0
	}
	if s.scroll > 0 && n > 0 {
		s.scroll += len(lobbyChatLines(chat[max(0, len(chat)-n):]))
	}
}

// chatScrollKey is the chat's scroll by key k (PgUp, PgDn, Home, End),
// 0 for other keys.
func chatScrollKey(k host.Key) int {
	switch k.Scancode {
	case 75: // PgUp
		return lhChatFit - 1
	case 78: // PgDn
		return -(lhChatFit - 1)
	case 74: // Home
		return lobby.ChatKeep * 8
	case 77: // End
		return -lobby.ChatKeep * 8
	}
	return 0
}

// lobbyHome is the lobby once logged in. It returns the game the player
// created (load: for a saved game) or joined, to play it; nil when he goes
// BACK (the session kept) or LOGOUT (forgotten), or the connection drops.
func (g *Game) lobbyHome(conn *lobby.Conn, rem *lobby.Remembered) (*lobbyv1.GameJoined, bool, error) {
	e := g.e
	s := &lobbyHomeScr{conn: conn, rem: rem, line: uniLine{max: 300}}
	g.lobbyScr = s
	saved := e.GetAutoFunction()
	defer func() {
		g.lobbyScr = nil
		e.ClearFields()
		e.PutAutoFunction(saved)
		g.fullDrawScreen = true
	}()
	e.DeactivateHelpList()
	redraw := func() error {
		e.V.SetPageOff()
		if err := g.drawLobbyHome(s); err != nil {
			return err
		}
		return e.TogglePages()
	}
	setup := func() error {
		if err := g.addLobbyHomeFields(s); err != nil {
			return err
		}
		e.Sys.SetTextInput(true) // the chat's line takes any language
		e.Sys.Wheel()            // turned before the lobby or over a box: not the chat's
		e.AssignAutoFunction(func() error { return g.drawLobbyHome(s) }, 1)
		return redraw()
	}
	if _, err := conn.Poll(); err != nil {
		return nil, false, g.messageBox(lobbyErrorText("the lobby", err))
	}
	s.said = conn.State.Said
	if m := conn.State.Mine; m != nil {
		conn.State.Note("You are in the game " + m.Description + ". CREATE is LEAVE.")
	}
	if err := setup(); err != nil {
		return nil, false, err
	}
	defer e.Sys.SetTextInput(false)
	note := func(err error) { conn.State.Note(lobbyErrorText("the lobby", err)) }
	for {
		// The chat's line takes the keys first, the rest are the engine's;
		// only here: the boxes opened from this loop have their own.
		var typed uniKey
		unhook := g.hookUniLine(&s.line, &typed)
		lineKey := e.KeyHook
		e.KeyHook = func(k host.Key) bool {
			if d := chatScrollKey(k); d != 0 {
				s.scroll += d
				return true
			}
			return lineKey(k)
		}
		r, err := e.GetInput()
		unhook()
		if err != nil {
			return nil, false, err
		}
		if s.hover, err = e.ScanInput(); err != nil {
			return nil, false, err
		}
		if _, err := conn.Poll(); err != nil {
			return nil, false, g.messageBox(lobbyErrorText("the lobby", err))
		}
		g.announceToLobby(&conn.State) // port-only
		if s.cur > len(s.tabs) || s.dialog() == nil && s.cur != 0 {
			s.show(0)
		}
		s.syncTabs()
		s.followChat()
		if n := e.Sys.Wheel(); n != 0 {
			if x, y, _ := e.Sys.Mouse(); x >= lhOnlineX && y >= lhChatY && y <= lhChatY2 {
				s.onlineTop -= n
			} else {
				s.scroll += n * lhWheel
			}
		}
		again := false // a box was shown: fields and screen anew
		switch {
		case typed == uniEnter:
			text := strings.TrimSpace(s.line.String())
			s.line.set("")
			s.scroll = 0
			if text != "" {
				s.say(text)
			}
		case typed == uniEsc && s.cur > 0: // out of a dialog to ALL
			s.show(0)
		case typed == uniEsc, r == s.backF: // Esc, BACK
			return nil, false, nil
		case r == s.logoutF:
			if err := conn.Logout(); err != nil {
				g.Log("lobby: logout: %v", err)
			}
			rem.Session = ""
			return nil, false, rem.Save(g.dir)
		case r == s.createF && conn.State.Mine != nil:
			if err := conn.LeaveGame(); err != nil {
				note(err)
			}
		case r == s.createF, r == s.loadF:
			load := r == s.loadF
			j, err := g.lobbyCreate(s, load)
			if err != nil || j != nil {
				return j, load, err
			}
			again = true
		case r == s.joinF:
			j, err := g.lobbyJoinBox(s)
			if err != nil || j != nil {
				return j, false, err
			}
			again = true
		default:
			s.lobbyTabClick(r)
		}
		if !again && s.tabSig() != s.fieldsSig {
			again = true // tabs came or went: their fields anew
		}
		if again {
			if err := setup(); err != nil {
				return nil, false, err
			}
		} else if err := redraw(); err != nil {
			return nil, false, err
		}
		// About 60 passes a second: the original's Release_Time_(2) (two
		// 55 ms ticks) made clicks and the pointer's highlight lag.
		if err := e.Sys.Sleep(lobbyFrame); err != nil {
			return nil, false, err
		}
	}
}

// lobbyFrame is the lobby screen's pass.
const lobbyFrame = time.Second / 60

// lobbyCreate asks for the description and public or private, makes the
// game (for a saved one with load) and copies its id to the clipboard;
// nil when the player changed his mind or the lobby refused (and said why).
func (g *Game) lobbyCreate(s *lobbyHomeScr, load bool) (*lobbyv1.GameJoined, error) {
	line := &uniLine{max: 60}
	if load {
		line.suggest(s.conn.User + "'s saved game")
	} else {
		line.suggest(s.conn.User + "'s game")
	}
	box := &netAddressBox{heading: lobbyHeading, label: "Game Description", uni: line}
	ok, err := g.uniAddressScreen(box)
	if err != nil || !ok {
		return nil, err
	}
	public, err := g.confirmationBox("List the game for everyone?\nNo makes it private: joined only by its id.")
	if err != nil {
		return nil, err
	}
	j, err := s.conn.CreateGame(line.String(), public)
	if err != nil {
		return nil, g.messageBox(lobbyErrorText("the lobby", err))
	}
	g.e.Sys.SetClipboard(j.Game.Id)
	if !public {
		if err := g.messageBox("The game's id is on the clipboard: give it to the players you invite."); err != nil {
			return nil, err
		}
	}
	return j, nil
}

// lobbyJoin joins game id; nil when the lobby refused (and said why).
func (g *Game) lobbyJoin(s *lobbyHomeScr, id string) (*lobbyv1.GameJoined, error) {
	j, err := s.conn.JoinGame(id)
	if err != nil {
		return nil, g.messageBox(lobbyErrorText("the lobby", err))
	}
	return j, nil
}
