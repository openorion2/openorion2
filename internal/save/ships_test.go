package save

import (
	"encoding/binary"
	"testing"
)

func TestRecordSizes(t *testing.T) {
	for _, c := range []struct {
		v    any
		size int
	}{{Design{}, 0x63}, {WeaponSlot{}, 8}, {Ship{}, 0x81}, {Leader{}, 0x3b}, {MoveableBox{}, 0x1c}} {
		if n := binary.Size(c.v); n != c.size {
			t.Errorf("%T is %d bytes, want %d", c.v, n, c.size)
		}
	}
}
