package save

import "testing"

func TestLayoutSize(t *testing.T) {
	n := 0
	for _, b := range Layout {
		n += b.Size * b.Count
	}
	if n != Size {
		t.Fatalf("layout covers %d bytes, want %d", n, Size)
	}
}
