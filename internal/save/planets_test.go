package save

import (
	"encoding/binary"
	"testing"
)

func TestPlanetSizes(t *testing.T) {
	if n := binary.Size(Planet{}); n != 0x11 {
		t.Errorf("Planet is %d bytes, want 17", n)
	}
	if n := binary.Size(Colony{}); n != 0x169 {
		t.Errorf("Colony is %d bytes, want 361", n)
	}
}
