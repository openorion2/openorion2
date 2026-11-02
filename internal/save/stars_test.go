package save

import (
	"encoding/binary"
	"testing"
)

func TestStarSizes(t *testing.T) {
	if n := binary.Size(Star{}); n != 0x71 {
		t.Errorf("Star is %d bytes, want 113", n)
	}
	if n := binary.Size([4]Nebula{}); n != 0x14 {
		t.Errorf("nebulas are %d bytes, want 20", n)
	}
}
