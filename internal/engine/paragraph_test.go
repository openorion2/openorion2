package engine

import (
	"errors"
	"testing"

	"github.com/openorion2/openorion2/internal/font"
)

// paraText gives a Text over a synthetic font: glyphs 4 wide, ' ' and '-' 3,
// 1 pixel spacing, height 6 and line feed 8.
func paraText() *Text {
	f := &font.Font{}
	s := &f.Styles[0]
	s.Height, s.HSpacing, s.VSpacing = 6, 1, 2
	for i := range s.Widths {
		s.Widths[i] = 4
	}
	s.Widths[' '], s.Widths['-'] = 3, 3
	e := &Engine{V: NewVideo()}
	e.SetFont(f)
	return e.Text()
}

type paraLine struct {
	x, y int16
	s    string
	typ  int
}

func layout(t *Text, x, y, maxw int, s string, typ int) ([]paraLine, error) {
	var got []paraLine
	err := t.paragraphBill(x, y, maxw, s, typ, func(x, y, _ int16, line []byte, typ int) error {
		got = append(got, paraLine{x, y, string(line), typ})
		return nil
	})
	return got, err
}

func TestParagraphWrap(t *testing.T) {
	tx := paraText()
	// "aaa bbb ccc": each word is 3*5 = 15 wide, a space 4; 34 fits two words.
	got, err := layout(tx, 10, 20, 34, "aaa bbb ccc", ParaFull)
	if err != nil {
		t.Fatal(err)
	}
	want := []paraLine{{10, 20, "aaa bbb", ParaFull}, {10, 28, "ccc", ParaLeft}}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d = %+v, want %+v", i, got[i], want[i])
		}
	}
	if h := tx.e.Paragraphs().MaxHeight; h != 8+6 {
		t.Errorf("height %d", h)
	}
}

func TestParagraphControls(t *testing.T) {
	tx := paraText()
	// CR forces a line, CR CR gives an empty one, tab is a space, other
	// control codes vanish, trailing spaces go.
	got, _ := layout(tx, 0, 0, 100, "ab  \r\rc\td\x01e", ParaLeft)
	want := []string{"ab", "", "c de"}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for i := range want {
		if got[i].s != want[i] {
			t.Errorf("line %d = %q, want %q", i, got[i].s, want[i])
		}
	}
}

func TestParagraphSoftHyphenAndLongWord(t *testing.T) {
	tx := paraText()
	// 0x08 is a soft hyphen: breaking there prints '-'.
	got, _ := layout(tx, 0, 0, 20, "aaa\x08bbb", ParaLeft)
	if len(got) != 2 || got[0].s != "aaa-" || got[1].s != "bbb" {
		t.Errorf("soft hyphen: %+v", got)
	}
	// A word with no break before it is cut, and the glyph that did not fit
	// is dropped (src is stepped over it as if it were the break).
	got, _ = layout(tx, 0, 0, 12, "abcdef", ParaLeft)
	if len(got) != 2 || got[0].s != "ab" || got[1].s != "de" {
		t.Errorf("long word: %+v", got)
	}
	if h := tx.e.Paragraphs().MaxHeight; h != 22 {
		t.Errorf("long word height %d", h)
	}
}

func TestParagraphStuckAndRejected(t *testing.T) {
	tx := paraText()
	tx.Font.Styles[0].Widths['W'] = 12
	tx.SetStyle(0, font.Solid(1))
	if _, err := layout(tx, 0, 0, 10, "ab W", ParaLeft); !errors.Is(err, ErrParagraphStuck) {
		t.Errorf("err %v", err)
	}
	ps := tx.e.Paragraphs()
	ps.MaxHeight = 77
	// max_width below 10 is rejected and the old height comes back.
	if h, _ := tx.ParagraphMaxHeight(9, "abc"); h != 77 {
		t.Errorf("rejected height %d", h)
	}
}

func TestParagraphMaxWidth(t *testing.T) {
	tx := paraText()
	w, _ := tx.ParagraphMaxWidth(100, "abc", ParaFull)
	if w != 15 {
		t.Errorf("one line full: %d", w)
	}
	w, _ = tx.ParagraphMaxWidth(20, "abc def", ParaFull)
	if w != 20 {
		t.Errorf("two lines full: %d", w)
	}
	w, _ = tx.ParagraphMaxWidth(100, "ab\rabcd", ParaLeft)
	if w != 20 {
		t.Errorf("left: %d", w)
	}
}

func TestInitializeFormatted(t *testing.T) {
	var p Paragraphs
	p.Tabs[3], p.Global[0] = 1, 9
	p.Variables[5] = 42
	p.InitializeFormatted()
	if p.Tabs[3] != 0 || p.Global[0] != 0 || p.Variables[5] != 0 {
		t.Fatal("not cleared")
	}
	for i := range 16 {
		if p.VarPtr[i] != &p.Variables[i] || p.VarType(i) != 2 {
			t.Fatalf("variable %d", i)
		}
	}
	p.ResetVariable(16) // ignored
}
