package save

import (
	"bytes"
	"encoding/binary"
	"path/filepath"
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

func TestStars(t *testing.T) {
	for _, p := range gameSaves(t) {
		f, err := Open(p)
		if err != nil {
			t.Fatal(err)
		}
		name := filepath.Base(p)
		stars := roundTrip[Star](t, f, BStars, 72)
		var neb [4]Nebula
		if err := binary.Read(bytes.NewReader(f.Block(BNebula)), binary.LittleEndian, &neb); err != nil {
			t.Fatal(err)
		}
		mx, my := f.U16(BMapMaxX), f.U16(BMapMaxY)
		n := f.U16(BNumStars)
		for i, s := range stars[:n] {
			if int(s.X) > mx || int(s.Y) > my || s.X < 0 || s.Y < 0 {
				t.Errorf("%s star %d at %d,%d outside %dx%d", name, i, s.X, s.Y, mx, my)
			}
			if s.WormholeTo >= 0 && stars[s.WormholeTo].WormholeTo != int8(i) {
				t.Errorf("%s star %d: wormhole to %d is one-way", name, i, s.WormholeTo)
			}
			for j := 0; j < n; j++ {
				a := s.BlackHoleBlocked[j>>3]>>(j&7)&1 != 0
				b := stars[j].BlackHoleBlocked[i>>3]>>(i&7)&1 != 0
				if a != b {
					t.Errorf("%s stars %d and %d: black hole blocking is not symmetric", name, i, j)
				}
			}
			for orbit, pl := range s.Planets {
				if pl >= 0 && int(pl) >= f.U16(BPlanetCount) {
					t.Errorf("%s star %d orbit %d: planet %d", name, i, orbit, pl)
				}
			}
		}
	}
}
