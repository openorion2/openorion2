package engine

import (
	"bytes"
	"errors"
	"path/filepath"
	"testing"

	"github.com/openorion2/openorion2/internal/font"
	"github.com/openorion2/openorion2/internal/lbx"
)

type fmtGlyph struct {
	x, y int16
	c    byte
}

// fmtRun prints s and returns the glyphs Print_Line_ placed.
func fmtRun(t *testing.T, tx *Text, x, y, maxw int, s string, typ int) []fmtGlyph {
	t.Helper()
	var got []fmtGlyph
	tx.glyphHook = func(x, y int16, c byte) { got = append(got, fmtGlyph{x, y, c}) }
	defer func() { tx.glyphHook = nil }()
	if err := tx.PrintFormatted(x, y, maxw, s, typ); err != nil {
		t.Fatal(err)
	}
	return got
}

func glyphText(g []fmtGlyph) string {
	var b []byte
	for _, x := range g {
		b = append(b, x.c)
	}
	return string(b)
}

func fmtText() *Text {
	tx := paraText()
	tx.e.Paragraphs().InitializeFormatted()
	tx.SetStyle(0, font.Solid(1))
	return tx
}

func TestFormattedWrap(t *testing.T) {
	tx := fmtText()
	// Glyphs 4 wide, ' ' 3, spacing 1: "aaa bbb" ends at column 42, the next
	// space would end at 46 > 43, so it is dropped and "ccc" starts a line.
	got := fmtRun(t, tx, 10, 20, 34, "aaa bbb ccc", 0)
	want := []fmtGlyph{{10, 20, 'a'}, {15, 20, 'a'}, {20, 20, 'a'}, {25, 20, ' '},
		{29, 20, 'b'}, {34, 20, 'b'}, {39, 20, 'b'}, {10, 28, 'c'}, {15, 28, 'c'}, {20, 28, 'c'}}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("glyph %d = %+v, want %+v", i, got[i], want[i])
		}
	}
	ps := tx.e.Paragraphs()
	if x1, y1, x2, y2 := ps.Box(); x1 != 10 || y1 != 20 || x2 != 43 || y2 != 34 {
		t.Errorf("box %d %d %d %d", x1, y1, x2, y2)
	}
	// The extent includes the glyph bottom row: one more than the simple
	// printer's height.
	if h, err := tx.FormattedMaxHeight(34, "aaa bbb ccc"); err != nil || h != 15 {
		t.Errorf("height %d %v", h, err)
	}
	if w, _ := tx.FormattedMaxWidth(34, "aaa bbb ccc", 0); w != 34 {
		t.Errorf("width %d", w)
	}
	if w := ps.LastFormattedMaxWidth(); w != 34 {
		t.Errorf("last width %d", w)
	}
}

func TestFormattedJustify(t *testing.T) {
	tx := fmtText()
	// Full: 2 spare pixels; one space in seven glyphs takes pattern 1 (1,1,1,
	// 3,1), so the space widens twice. The last line stays left.
	got := fmtRun(t, tx, 10, 20, 34, "aaa bbb ccc", 3)
	if got[4].x != 31 || got[6].x != 41 || got[7].x != 10 {
		t.Errorf("full: %+v", got)
	}
	// Right and centred move the first glyph by the spare pixels.
	// The line width is lastR − left, one short of the span, so a right
	// aligned line overshoots the right edge by a pixel.
	got = fmtRun(t, tx, 0, 0, 30, "ab", 1)
	if got[0].x != 22 || got[1].x != 27 {
		t.Errorf("right: %+v", got)
	}
	got = fmtRun(t, tx, 0, 0, 30, "ab", 2)
	if got[0].x != 11 {
		t.Errorf("centred: %+v", got)
	}
}

func TestFormattedLinesAndFunctions(t *testing.T) {
	tx := fmtText()
	// LF and CR end a line; \aX sets x from the paragraph's left edge; the
	// function ends at its '.'.
	got := fmtRun(t, tx, 5, 0, 200, "a\n\aX20.b\r\aX+3.c", 0)
	want := []fmtGlyph{{5, 0, 'a'}, {25, 8, 'b'}, {8, 16, 'c'}}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("glyph %d = %+v, want %+v", i, got[i], want[i])
		}
	}
	// Tab stops from function T, sorted; a tab moves to the first stop
	// right of x.
	got = fmtRun(t, tx, 0, 0, 200, "\aT60,30.a\tb\tc", 0)
	if len(got) != 3 || got[1].x != 30 || got[2].x != 60 {
		t.Errorf("tabs: %+v", got)
	}
	// A function argument that runs into the terminator reads past it.
	if err := tx.PrintFormatted(0, 0, 200, "a\aX20", 0); !errors.Is(err, ErrFmtOverrun) {
		t.Errorf("overrun: %v", err)
	}
	if err := tx.PrintFormatted(0, 0, 200, "\aO10.a", 0); !errors.Is(err, ErrFmtOrigin) {
		t.Errorf("origin: %v", err)
	}
}

func TestFormattedSoftHyphen(t *testing.T) {
	tx := fmtText()
	// Breaking at 0x08 prints '-' at the end of the first line.
	got := fmtRun(t, tx, 0, 0, 20, "aaa\x08bbb", 0)
	if s := glyphText(got); s != "aaa-bbb" {
		t.Fatalf("got %q %+v", s, got)
	}
	if got[3].y != 0 || got[4].y != 8 || got[4].x != 0 {
		t.Errorf("hyphen: %+v", got)
	}
}

func TestFormattedInserts(t *testing.T) {
	tx := fmtText()
	ps := tx.e.Paragraphs()
	var v int32 = 5
	ps.SetVarInt(3, &v)
	ps.SetVarString(4, []byte("Orion\x00junk"))
	got := fmtRun(t, tx, 0, 0, 400, "\x18B3 ship\x18F3, \x18A4\x18C3", 0)
	if s := glyphText(got); s != "5 ships, Orion5th" {
		t.Errorf("got %q", s)
	}
	// The ordinal ending is raised by function ^2 (style 0 minus 2 clamps to
	// 0, so only the record changes) and the test of variable 3 is true.
	if _, err := tx.FormattedMaxHeight(100, "\x1903\x18M4"); err != nil || ps.testFlag() != 1 {
		t.Errorf("test flag %d %v", ps.testFlag(), err)
	}
	// With 1 nothing is inserted and the argument digit prints (the
	// command steps back one byte too few).
	v = 1
	got = fmtRun(t, tx, 0, 0, 400, "ship\x18F3", 0)
	if s := glyphText(got); s != "ship3" {
		t.Errorf("singular %q", s)
	}
	// Function = sets a variable.
	fmtRun(t, tx, 0, 0, 400, "\a=3,42.x", 0)
	if v != 42 {
		t.Errorf("set variable: %d", v)
	}
	// Items Q..T take no argument and do not step over their letter, so it
	// prints: Q upper-cases it and what follows.
	got = fmtRun(t, tx, 0, 0, 400, "\x18Qab\x18Tcd", 0)
	if s := glyphText(got); s != "QABTcd" {
		t.Errorf("insert case %q", s)
	}
}

func TestFormattedRejected(t *testing.T) {
	tx := fmtText()
	ps := tx.e.Paragraphs()
	ps.setBox(1, 4)
	ps.setBox(3, 10)
	if h, _ := tx.FormattedMaxHeight(9, "abc"); h != 7 {
		t.Errorf("rejected height %d", h)
	}
}

// helpTexts returns the titles and texts of HELP.LBX.
func helpTexts(t *testing.T) [][2]string {
	a, err := lbx.Open(filepath.Join("..", "..", "game", "HELP.LBX"))
	if err != nil {
		t.Skip("game files not present")
	}
	tb, err := a.Table(0)
	if err != nil {
		t.Fatal(err)
	}
	cs := func(b []byte) string {
		if i := bytes.IndexByte(b, 0); i >= 0 {
			b = b[:i]
		}
		return string(b)
	}
	var out [][2]string
	for i := 0; i < len(tb.Data)/tb.Size; i++ {
		r := tb.Record(i)
		out = append(out, [2]string{cs(r[:0x50]), cs(r[0x67:])})
	}
	return out
}

func gameFontText(t *testing.T) *Text {
	a, err := lbx.Open(filepath.Join("..", "..", "game", "FONTS.LBX"))
	if err != nil {
		t.Skip("game files not present")
	}
	data, err := a.Entry(0)
	if err != nil {
		t.Fatal(err)
	}
	f, err := font.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	e := &Engine{V: NewVideo()}
	e.SetFont(f)
	e.Paragraphs().InitializeFormatted()
	return e.Text()
}

// TestFormattedHelpTexts lays out every HELP.LBX text in the help box's
// styles and widths; printing and measuring must agree.
func TestFormattedHelpTexts(t *testing.T) {
	recs := helpTexts(t)
	tx := gameFontText(t)
	for i, r := range recs {
		for k, s := range r {
			style := 2
			if k == 0 {
				style = 4
			}
			tx.SetStyle(style, font.Solid(1))
			h, err := tx.FormattedMaxHeight(0x153, s)
			if err != nil {
				t.Fatalf("record %d: %v", i, err)
			}
			var maxY int16 = -1
			tx.glyphHook = func(_, y int16, _ byte) { maxY = max(maxY, y) }
			err = tx.PrintFormatted(0, 0, 0x153, s, 0)
			tx.glyphHook = nil
			if err != nil {
				t.Fatalf("record %d: %v", i, err)
			}
			if maxY >= 0 && h != int(maxY)+tx.Font.Styles[style].Height+1 {
				t.Errorf("record %d/%d: height %d, last row %d", i, k, h, maxY)
			}
		}
	}
	// Record 289 is a table laid out with function X.
	tx.SetStyle(2, font.Solid(1))
	got := fmtRun(t, tx, 0x14, 0x2a, 0x153, recs[289][1], 0)
	lf := int16(tx.Font.Styles[2].Height + tx.Font.Styles[2].VSpacing)
	row := map[int16]byte{}
	for _, g := range got {
		if g.y == 0x2a {
			row[g.x] = g.c
		}
	}
	for x, c := range map[int16]byte{0x14 + 3: 'F', 0x14 + 97: '-', 0x14 + 150: 'S', 0x14 + 270: '+'} {
		if row[x] != c {
			t.Errorf("x %d: %q, want %q", x, row[x], c)
		}
	}
	if got[len(got)-1].y != 0x2a+5*lf {
		t.Errorf("last row y %d", got[len(got)-1].y)
	}
}

// TestFormattedHestrngs measures every HESTRNGS.LBX string in the user box
// style and width; a one-line string measures its glyph span plus one.
func TestFormattedHestrngs(t *testing.T) {
	a, err := lbx.Open(filepath.Join("..", "..", "game", "HESTRNGS.LBX"))
	if err != nil {
		t.Skip("game files not present")
	}
	tb, err := a.Table(0)
	if err != nil {
		t.Fatal(err)
	}
	tx := gameFontText(t)
	for i, s := range bytes.Split(tb.Record(0), []byte{0}) {
		if i >= 0x18d {
			break
		}
		tx.SetStyle(3, font.Solid(1))
		if _, err := tx.FormattedMaxHeight(0x153, string(s)); err != nil {
			t.Fatalf("string %#x: %v", i, err)
		}
		// The gate info box: pw = string width + 2 always fits one line.
		tx.SetStyle(1, font.Solid(1))
		if len(s) == 0 || bytes.ContainsAny(s, "\r\n\a\x17\x18\x19\x1a\x1b\x1c\x1d\x1f") {
			continue
		}
		w := tx.Width(s)
		if w+2 < 10 || w+2 > 0x280 {
			continue
		}
		got, err := tx.FormattedMaxWidth(w+2, string(s), 0)
		if err != nil {
			t.Fatalf("string %#x: %v", i, err)
		}
		if want := w - tx.Spacing + 1; got != want && s[0] != ' ' && s[len(s)-1] != ' ' {
			t.Errorf("string %#x %q: width %d, want %d", i, s, got, want)
		}
	}
}
