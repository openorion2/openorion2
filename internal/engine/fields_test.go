package engine

import (
	"testing"

	"github.com/openorion2/openorion2/internal/host"
)

func newFieldEngine() *Engine {
	e := &Engine{V: NewVideo(), Sys: &host.System{}}
	e.f.init()
	return e
}

func TestAltAlphaKey(t *testing.T) {
	cases := map[int]byte{
		0x1000: 'Q', 0x1e00: 'A', 0x2c00: 'Z', 0x3200: 'M', 0x1200: 'E',
		0x7800: '0', 0x7f00: '7',
		0x1071: 0xff, // plain q
		0x0200: 0xff, // what the port's biosWord gives for Alt-1
		0x8000: 0xff, // Alt-9 is not in the table
		0xada2: 0xff, // the 35th word scanned, really the jump table
	}
	for k, want := range cases {
		if got := altAlphaKey(k); got != want {
			t.Errorf("altAlphaKey(%#x) = %#x, want %#x", k, got, want)
		}
	}
}

func TestMultiHotKeyProgress(t *testing.T) {
	e := newFieldEngine()
	e.ClearFields()
	ein, _ := e.AddMultiHotKeyField("EIN")
	exit, _ := e.AddMultiHotKeyField("EX")
	if !e.f.multiActive || e.f.fields[ein].MultiLen != 3 || e.f.fields[ein].Hotkey != 0 {
		t.Fatalf("bad multi field %+v", e.f.fields[ein])
	}

	// E advances both, a mismatch keeps progress, X completes the second.
	if n := e.checkMultiHotKeyFields('E'); n != -1 {
		t.Fatalf("E returned %d", n)
	}
	if e.f.fields[ein].Progress != 1 || e.f.fields[exit].Progress != 1 {
		t.Fatalf("progress after E: %d %d", e.f.fields[ein].Progress, e.f.fields[exit].Progress)
	}
	e.checkMultiHotKeyFields('Q')
	if e.f.fields[ein].Progress != 1 {
		t.Fatalf("mismatch reset progress")
	}
	if n := e.checkMultiHotKeyFields('X'); n != exit {
		t.Fatalf("X returned %d, want %d", n, exit)
	}

	// Through Interpret_Keyboard_Input: Alt-I, Alt-N finish the first.
	field := 0
	if c := e.interpretKeyboardInput(0x1700, &field); c == -1 {
		t.Fatalf("Alt-I completed a field")
	}
	field = 0
	if c := e.interpretKeyboardInput(0x3100, &field); c != -1 || field != ein {
		t.Fatalf("Alt-N gave c=%d field=%d", c, field)
	}

	e.clearMultiHotKeyOffsets()
	if e.f.fields[ein].Progress != 0 || e.f.fields[exit].Progress != 0 {
		t.Fatalf("clear left progress")
	}
	e.ClearFields()
	if e.f.multiActive {
		t.Fatalf("Clear_Fields left multi_hotkey_active_field set")
	}
}

func TestGridField(t *testing.T) {
	e := newFieldEngine()
	e.ClearFields()
	var gx, gy int
	n, err := e.AddGridField(0x16, 0x16, 1, 1, 0x1fa, 400, &gx, &gy, 0)
	if err != nil {
		t.Fatal(err)
	}
	fl := e.f.fields[n]
	if fl.X1 != 0x16 || fl.Y1 != 0x16 || fl.X2 != 0x20f || fl.Y2 != 0x1a5 || fl.Type != FieldGrid {
		t.Fatalf("grid rect %+v", fl)
	}

	// Pressed: offset applied, clamped to the grid.
	for _, c := range []struct{ mx, my, gx, gy int }{
		{100, 50, 78, 28},
		{639, 479, 0x20f - 0x16, 0x1a5 - 0x16},
		{0, 0, 0, 0},
	} {
		e.Sys.WarpMouse(c.mx, c.my)
		if err := e.drawField(n, true); err != nil {
			t.Fatal(err)
		}
		if gx != c.gx || gy != c.gy {
			t.Errorf("pointer (%d,%d): grid (%d,%d), want (%d,%d)", c.mx, c.my, gx, gy, c.gx, c.gy)
		}
	}

	// Released, or with drawing disabled, the cell is left alone.
	e.Sys.WarpMouse(30, 40)
	gx, gy = -5, -5
	e.drawField(n, false)
	e.DisableDrawAllFields()
	e.drawField(n, true)
	e.EnableDrawAllFields()
	if gx != -5 || gy != -5 {
		t.Fatalf("grid written: %d %d", gx, gy)
	}

	// Scan_Input writes the hovered cell too.
	got, err := e.ScanInput()
	if err != nil || got != n || gx != 30-0x16 || gy != 40-0x16 {
		t.Fatalf("ScanInput = %d %v, grid (%d,%d)", got, err, gx, gy)
	}
	e.Sys.WarpMouse(5, 5)
	if got, _ := e.ScanInput(); got != 0 {
		t.Fatalf("ScanInput outside = %d", got)
	}
}

func TestClearFieldsAbove(t *testing.T) {
	e := newFieldEngine()
	e.ClearFields()
	for i := 0; i < 6; i++ {
		e.AddHotKey("A")
	}
	e.f.downMouseButton = 4
	e.ClearFieldsAbove(3)
	if e.FieldsCount() != 4 || e.f.downMouseButton != -1 {
		t.Fatalf("count %d down %d", e.FieldsCount(), e.f.downMouseButton)
	}
	if n, _ := e.AddHotKey("B"); n != 4 {
		t.Fatalf("next field %d", n)
	}
}

func TestIrregularOverflow(t *testing.T) {
	e := newFieldEngine()
	e.ClearFields()
	for e.FieldsCount() < maxFields {
		if _, err := e.AddHiddenField(0, 0, 1, 1, "", 0); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := e.AddHiddenField(0, 0, 1, 1, "", 0); err == nil {
		t.Fatal("hidden field 250 accepted")
	}
	if n, err := e.AddIrregularButtonField(0, 0, 1, 1, "", nil, 0, 0, "x", 0); err != nil || n != maxFields {
		t.Fatalf("irregular 250: %d %v", n, err)
	}
	if e.f.fields[maxFields].Hotkey != 'X' {
		t.Fatalf("hotkey not upper-cased")
	}
	if _, err := e.AddIrregularButtonField(0, 0, 1, 1, "", nil, 0, 0, "", 0); err == nil {
		t.Fatal("irregular 251 accepted")
	}
}

func TestScrollField(t *testing.T) {
	e := newFieldEngine()
	e.ClearFields()
	var pos int16
	// The fleet box bar: 10 x 0x66 at (0xc0, 0x42), values 0..0x66.
	n, err := e.AddScrollField(0xc0, 0x42, 0, 0x66, 0, 0x66, 10, 0x66, &pos, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	fl := &e.f.fields[n]
	if fl.Type != FieldScroll || !fl.Vertical || fl.X2 != 0xca || fl.Y2 != 0xa8 {
		t.Fatalf("scroll field %+v", *fl)
	}
	// Measured up from the bottom; the ends give min and max.
	for _, c := range []struct {
		y    int
		want int16
	}{{0xa8, 0}, {0xb0, 0}, {0x42, 0x66}, {0x10, 0x66}, {0x43, 0x65}, {0x75, 0x33}} {
		e.Sys.WarpMouse(0xc4, c.y)
		if err := e.drawField(n, true); err != nil {
			t.Fatal(err)
		}
		if pos != c.want {
			t.Errorf("y %#x: pos %#x, want %#x", c.y, pos, c.want)
		}
	}
	// Released, nothing changes; Draw_Visible_Fields clamps.
	pos = 0x70
	e.drawField(n, false)
	if pos != 0x70 {
		t.Fatalf("released field moved the bar")
	}
	if err := e.drawVisibleFieldsErr(); err != nil {
		t.Fatal(err)
	}
	if pos != 0x66 {
		t.Fatalf("Draw_Visible_Fields left %#x", pos)
	}

	// + and - step by 5 % of the range.
	pos = 0x33
	incrementScrollBar(fl)
	if pos != 0x38 {
		t.Errorf("increment: %#x", pos)
	}
	pos = 0x64
	incrementScrollBar(fl)
	if pos != 0x66 {
		t.Errorf("increment at the top: %#x", pos)
	}
	pos = 2
	decrementScrollBar(fl)
	if pos != 0 {
		t.Errorf("decrement at the bottom: %#x", pos)
	}
}
