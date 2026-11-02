package game

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/openorion2/openorion2/internal/engine"
	"github.com/openorion2/openorion2/internal/host"
)

// port-only: a line of Unicode text being typed, for the lobby (its chat
// and the game description): typed text comes as host.Key.Text while the
// host's text input is on (any language, the system's input method), as
// plain ASCII keys otherwise (headless runs, the browser). The engine's own
// input fields stay ASCII, as the game's are.

type uniLine struct {
	text  []rune
	max   int  // runes
	fresh bool // a default the first edit replaces whole, as the engine's fields do
	blink int
}

// uniKey is what a key did to a line.
type uniKey int

const (
	uniNone  uniKey = iota // not the line's: left for the engine
	uniTyped               // the line took it
	uniEnter
	uniEsc
)

func (l *uniLine) insert(s string) {
	if l.fresh {
		l.text, l.fresh = nil, false
	}
	for _, r := range s {
		if r == '\n' || r == '\r' || r == '\t' {
			r = ' '
		}
		if unicode.IsControl(r) || len(l.text) >= l.max {
			continue
		}
		l.text = append(l.text, r)
	}
}

// key folds k in; textMode: printable keys come again as Key.Text.
func (l *uniLine) key(k host.Key, textMode bool) uniKey {
	switch {
	case k.Text != "":
		l.insert(k.Text)
		return uniTyped
	case k.Paste != "":
		l.insert(strings.TrimSpace(k.Paste))
		return uniTyped
	}
	switch c := engine.KeyASCII(k); {
	case c == '\r':
		return uniEnter
	case c == 0x1b:
		return uniEsc
	case c == 0x08:
		if n := len(l.text); l.fresh {
			l.text, l.fresh = nil, false
		} else if n > 0 {
			l.text = l.text[:n-1]
		}
		return uniTyped
	case c >= 0x20 && c < 0x7f:
		if !textMode {
			l.insert(string(rune(c)))
		}
		return uniTyped
	}
	return uniNone
}

// hookUniLine makes l take the keys before the engine's fields until the
// returned func is called; Enter and Esc land in *done (the next GetInput
// returns with nothing for them).
func (g *Game) hookUniLine(l *uniLine, done *uniKey) func() {
	e := g.e
	old := e.KeyHook
	e.KeyHook = func(k host.Key) bool {
		r := l.key(k, e.Sys.TextInput())
		if r == uniEnter || r == uniEsc {
			*done = r
		}
		return r != uniNone
	}
	return func() { e.KeyHook = old }
}

func (l *uniLine) String() string { return string(l.text) }

func (l *uniLine) set(s string) { l.text, l.fresh = nil, false; l.insert(s) }

// suggest sets a default the first edit replaces.
func (l *uniLine) suggest(s string) { l.set(s); l.fresh = true }

// drawUniLine draws the line from x to x2 on a line whose top is y, its
// end in view, with a blinking cursor.
func (g *Game) drawUniLine(l *uniLine, x, y, x2 int, c byte) {
	l.blink++
	text := string(l.text)
	cursor := uniWidth("_")
	for text != "" && uniWidth(text)+cursor > x2-x {
		_, n := utf8.DecodeRuneInString(text)
		text = text[n:]
	}
	end := g.uniPrint(x, y, x2, text, c)
	if l.blink/24%2 == 0 { // ~0.4 s at the lobby's 60 passes a second
		g.uniPrint(end, y, x2, "_", c)
	}
}
