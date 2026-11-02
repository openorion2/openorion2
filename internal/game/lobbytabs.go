package game

import (
	"fmt"
	"strings"

	"github.com/openorion2/openorion2/internal/lobby"
)

// port-only: the lobby chat's tabs, ALL (the public chat) and one per
// private dialog. A click on a name online, "/w name", or a line coming
// from someone opens a dialog's tab; its x closes it. A tab with lines not
// read yet counts them; they are read once its tab is shown.

const (
	lhTabY    = lhChatY + 2            // the tabs' line
	lhTabPad  = 4                      // around a tab's label
	lhTabsX2  = lhChatX2 - 4           // where the tabs must end
	lhChatTop = lhChatY + uniLineH + 6 // the chat's first line, under the tabs
	lhTabMax  = 8                      // dialogs' tabs at most
	lhCloseX  = "x"                    // a dialog tab's close mark
)

// lobbyTab is one tab's place: the label from x1 to x2, its close mark
// from cx1 to cx2 (none for ALL).
type lobbyTab struct {
	label            string
	unread           bool
	x1, x2, cx1, cx2 int
}

// tabLayout is the tabs as they are drawn, ALL first.
func (s *lobbyHomeScr) tabLayout() []lobbyTab {
	st := &s.conn.State
	x := lhX1 + 4
	tabs := make([]lobbyTab, 0, 1+len(s.tabs))
	for i := -1; i < len(s.tabs); i++ {
		t := lobbyTab{label: "ALL"}
		if i >= 0 {
			t.label = s.tabs[i]
			if d := st.Dialog(s.tabs[i]); d != nil {
				t.label = d.With // as the server spells it, once it answered
				if d.Unread > 0 {
					t.label += fmt.Sprintf(" (%d)", d.Unread)
					t.unread = true
				}
			}
		}
		t.x1 = x
		t.x2 = x + 2*lhTabPad + uniWidthBold(t.label) - 1
		x = t.x2 + 1
		if i >= 0 {
			t.cx1, t.cx2 = x, x+uniWidthBold(lhCloseX)+2*lhTabPad-1
			x = t.cx2 + 1
		}
		x += 2
		tabs = append(tabs, t)
	}
	return tabs
}

// tabSig changes whenever the tabs' fields must be laid out again.
func (s *lobbyHomeScr) tabSig() string {
	var b strings.Builder
	for _, t := range s.tabLayout() {
		fmt.Fprintf(&b, "%d,%d,%d;", t.x1, t.x2, t.cx2)
	}
	return b.String()
}

// addLobbyTabFields adds the tabs' fields and the names' online. Under
// addLobbyHomeFields.
func (g *Game) addLobbyTabFields(s *lobbyHomeScr) error {
	e := g.e
	s.tabF, s.closeF = s.tabF[:0], s.closeF[:0]
	for i, t := range s.tabLayout() {
		f, err := e.AddHiddenField(t.x1, lhTabY, t.x2, lhTabY+uniLineH-1, "", 0x28)
		if err != nil {
			return err
		}
		s.tabF = append(s.tabF, f)
		if i > 0 {
			if f, err = e.AddHiddenField(t.cx1, lhTabY, t.cx2, lhTabY+uniLineH-1, "", 0x28); err != nil {
				return err
			}
			s.closeF = append(s.closeF, f)
		}
	}
	for i := range s.onlineF {
		y := lhOnlineY + i*uniLineH
		f, err := e.AddHiddenField(lhOnlineX+2, y, lhOnlineBarX-3, y+uniLineH-1, "", 0x28)
		if err != nil {
			return err
		}
		s.onlineF[i] = f
	}
	s.fieldsSig = s.tabSig()
	return nil
}

// drawLobbyTabs draws the tabs on the chat panel's top, the shown one lit.
func (g *Game) drawLobbyTabs(s *lobbyHomeScr) {
	v := g.e.V
	for i, t := range s.tabLayout() {
		if t.x1 > lhTabsX2 {
			break
		}
		x2 := min(t.x2, lhTabsX2)
		if i > 0 {
			x2 = min(t.cx2, lhTabsX2)
		}
		c := lhText
		if i == s.cur {
			v.Fill(t.x1, lhTabY-1, x2, lhTabY+uniLineH-1, lhRowLit)
			c = lhTextLit
		}
		v.Line(t.x1, lhTabY-1, t.x1, lhTabY+uniLineH-1, lhPanelLine)
		v.Line(x2, lhTabY-1, x2, lhTabY+uniLineH-1, lhPanelLine)
		if t.unread || i == s.cur {
			g.uniPrintBold(t.x1+lhTabPad, lhTabY, x2, t.label, lhTextLit)
		} else {
			g.uniPrint(t.x1+lhTabPad, lhTabY, x2, t.label, c)
		}
		if i > 0 && t.cx1 < lhTabsX2 {
			cc := lhText
			if s.hover > 0 && i-1 < len(s.closeF) && s.hover == s.closeF[i-1] {
				cc = lhTextLit
			}
			g.uniPrintBold(t.cx1+lhTabPad, lhTabY, x2, lhCloseX, cc)
		}
	}
	v.Line(lhX1+1, lhTabY+uniLineH, lhChatX2-1, lhTabY+uniLineH, lhPanelLine)
}

// dialog is the shown tab's dialog, nil on ALL.
func (s *lobbyHomeScr) dialog() *lobby.Dialog {
	if s.cur == 0 {
		return nil
	}
	return s.conn.State.Open(s.tabs[s.cur-1])
}

// source is the shown tab's lines and their count ever said.
func (s *lobbyHomeScr) source() ([]lobby.Line, int) {
	if d := s.dialog(); d != nil {
		return d.Lines, d.Said
	}
	return s.conn.State.Chat, s.conn.State.Said
}

// show makes tab i the shown one, at its end.
func (s *lobbyHomeScr) show(i int) {
	if i == s.cur {
		return
	}
	s.cur, s.scroll = i, 0
	_, s.said = s.source()
}

// openTab opens (or shows) the dialog with name and shows it.
func (s *lobbyHomeScr) openTab(name string) {
	for i, t := range s.tabs {
		if strings.EqualFold(t, name) {
			s.show(i + 1)
			return
		}
	}
	s.addTab(name)
	s.show(len(s.tabs))
}

// addTab adds a dialog's tab, closing the oldest read one past lhTabMax.
func (s *lobbyHomeScr) addTab(name string) {
	if len(s.tabs) >= lhTabMax {
		for i, t := range s.tabs {
			if d := s.conn.State.Dialog(t); i+1 != s.cur && (d == nil || d.Unread == 0) {
				s.closeTab(i)
				break
			}
		}
	}
	s.tabs = append(s.tabs, name)
}

// closeTab closes dialog tab i (0-based among the dialogs').
func (s *lobbyHomeScr) closeTab(i int) {
	if d := s.conn.State.Dialog(s.tabs[i]); d != nil && d.Unread > 0 {
		s.conn.MarkRead(d) // closed: seen enough
	}
	s.tabs = append(s.tabs[:i], s.tabs[i+1:]...)
	switch {
	case s.cur == i+1:
		s.cur = -1 // force show(0) to reset
		s.show(0)
	case s.cur > i+1:
		s.cur--
	}
}

// syncTabs opens a tab for every dialog with lines not read, and reads
// the shown one's.
func (s *lobbyHomeScr) syncTabs() {
	st := &s.conn.State
	for _, d := range st.Dialogs {
		if d.Unread == 0 {
			continue
		}
		open := false
		for _, t := range s.tabs {
			open = open || strings.EqualFold(t, d.With)
		}
		if !open {
			s.addTab(d.With)
		}
	}
	if d := s.dialog(); d != nil && d.Unread > 0 {
		if err := s.conn.MarkRead(d); err != nil {
			d.Note(lobbyErrorText("the lobby", err))
		}
	}
}

// lobbyTabClick handles a click r on a tab, a tab's x or a name online.
func (s *lobbyHomeScr) lobbyTabClick(r int) {
	if r <= 0 {
		return
	}
	for i, f := range s.tabF {
		if r == f {
			s.show(i)
			return
		}
	}
	for i, f := range s.closeF {
		if r == f {
			s.closeTab(i)
			return
		}
	}
	st := &s.conn.State
	for i, f := range s.onlineF {
		if r == f && s.onlineTop+i < len(st.Online) {
			if n := st.Online[s.onlineTop+i]; !strings.EqualFold(n, st.Me) {
				s.openTab(n)
			}
			return
		}
	}
}

// say sends a line typed in the shown tab: to the chat, or the dialog's
// user; "/w name [text]" on ALL opens the dialog with name (and says
// text in it).
func (s *lobbyHomeScr) say(text string) {
	conn := s.conn
	if s.cur == 0 {
		if name, rest, ok := whisperCommand(text); ok {
			if strings.EqualFold(name, conn.State.Me) {
				conn.State.Note("That is you.")
				return
			}
			s.openTab(name)
			text = rest
		} else {
			if err := conn.Say(text); err != nil {
				conn.State.Note(lobbyErrorText("the lobby", err))
			}
			return
		}
	}
	if text == "" {
		return
	}
	d := s.dialog()
	if err := conn.Whisper(d.With, text); err != nil {
		d.Note(lobbyErrorText("the lobby", err))
	}
}

// whisperCommand parses "/w name text" (or /msg), the text optional.
func whisperCommand(text string) (name, rest string, ok bool) {
	cmd, args, _ := strings.Cut(text, " ")
	if cmd != "/w" && cmd != "/msg" {
		return "", "", false
	}
	name, rest, _ = strings.Cut(strings.TrimSpace(args), " ")
	if name == "" {
		return "", "", false
	}
	return name, strings.TrimSpace(rest), true
}
