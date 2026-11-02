package save

import (
	"encoding/binary"
	"path/filepath"
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

func TestPlanetsAndColonies(t *testing.T) {
	for _, p := range gameSaves(t) {
		f, err := Open(p)
		if err != nil {
			t.Fatal(err)
		}
		name := filepath.Base(p)
		planets := roundTrip[Planet](t, f, BPlanets, 360)
		colonies := roundTrip[Colony](t, f, BColonies, 250)
		stars := roundTrip[Star](t, f, BStars, 72)
		for i, c := range colonies[:f.U16(BNumColonies)] {
			if c.Owner < 0 {
				continue
			}
			pl := planets[c.Planet]
			if int(pl.Colony) != i {
				t.Errorf("%s colony %d: planet %d points to colony %d", name, i, c.Planet, pl.Colony)
			}
			s := stars[pl.Star]
			if s.Planets[pl.Orbit] != c.Planet {
				t.Errorf("%s colony %d: star %d orbit %d lists planet %d, not %d", name, i, pl.Star, pl.Orbit, s.Planets[pl.Orbit], c.Planet)
			}
			if s.ColonyOwners>>uint(c.Owner)&1 == 0 {
				t.Errorf("%s colony %d: owner %d missing from star %d", name, i, c.Owner, pl.Star)
			}
			for k := 0; k < int(c.PopCount); k++ {
				col := Colonist(c.Colonists[k])
				if col.Race() > 9 || col.Job() > 2 || col>>11 != 0 {
					t.Errorf("%s colony %d colonist %d: 0x%x", name, i, k, uint32(col))
				}
			}
		}
	}
}
