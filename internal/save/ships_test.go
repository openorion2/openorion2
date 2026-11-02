package save

import (
	"bytes"
	"encoding/binary"
	"path/filepath"
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

// roundTrip decodes and re-encodes every record of a block and checks the
// file is unchanged.
func roundTrip[T any](t *testing.T, f *File, block string, n int) []T {
	before := f.Bytes()
	out := make([]T, n)
	for i := range out {
		if err := f.Decode(block, i, &out[i]); err != nil {
			t.Fatal(err)
		}
		if err := f.Encode(block, i, &out[i]); err != nil {
			t.Fatal(err)
		}
	}
	if !bytes.Equal(before, f.Bytes()) {
		t.Fatalf("%s does not round-trip", block)
	}
	return out
}

func TestShipsAndLeaders(t *testing.T) {
	for _, p := range gameSaves(t) {
		f, err := Open(p)
		if err != nil {
			t.Fatal(err)
		}
		name := filepath.Base(p)
		ships := roundTrip[Ship](t, f, BShips, 500)
		leaders := roundTrip[Leader](t, f, BLeaders, 67)
		roundTrip[MoveableBox](t, f, BMoveableBox, 5)
		stars, players := f.U16(BNumStars), f.U16(BNumPlayers)
		for i, s := range ships[:f.U16(BNumShips)] {
			if s.Status == ShipFree {
				continue
			}
			if s.Owner < 8 && int(s.Owner) >= players {
				t.Errorf("%s ship %d: owner %d with %d players", name, i, s.Owner, players)
			}
			loc := int(s.Location)
			if s.Status == ShipInTransit {
				loc -= LocationTransit
			} else if s.Status == ShipDeparting {
				loc -= LocationDeparted
			}
			if loc < 0 || loc >= stars {
				t.Errorf("%s ship %d: status %d location %d", name, i, s.Status, s.Location)
			}
			if s.Officer >= 0 && (leaders[s.Officer].Location != int16(i) || leaders[s.Officer].Status != 1) {
				t.Errorf("%s ship %d: officer %d does not point back", name, i, s.Officer)
			}
		}
	}
}
