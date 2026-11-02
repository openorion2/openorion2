package game

import "testing"

// The fleet box bar for 20 ships: 7 rows of 3, three visible, track 0x66.
func TestScrollBarFirstItem(t *testing.T) {
	t.Parallel()
	b := scrollBar{visible: 3, track: 0x66, rows: 7, step: 1}
	b.h = b.height()
	if b.h != 44 {
		t.Fatalf("height %d", b.h)
	}
	for _, c := range []struct{ pos, want int16 }{
		{0x66 - 22, 0}, // top
		{70, 0},
		{50, 2},
		{30, 3},
		{22, 4}, // bottom: the last page
	} {
		b.pos = c.pos
		if got := b.firstItem(&b); got != c.want {
			t.Errorf("pos %d: first %d, want %d", c.pos, got, c.want)
		}
	}
	b.pos = 0
	if !b.setMaxima() || b.pos != 22 {
		t.Errorf("clamp at the bottom: %d", b.pos)
	}
	b.pos = 0x66
	if !b.setMaxima() || b.pos != 0x66-22 {
		t.Errorf("clamp at the top: %d", b.pos)
	}
	if b.setMaxima() {
		t.Errorf("clamped twice")
	}
	// Few rows: the thumb fills the track.
	s := scrollBar{visible: 3, track: 0x66, rows: 1, step: 1}
	if s.height() != 0x66 {
		t.Errorf("one row height %d", s.height())
	}
}
