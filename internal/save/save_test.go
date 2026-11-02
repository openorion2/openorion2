package save

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// gameSaves are the original's ten saves: the fixed copy in build/saves
// when present, since the user's game/ gets the port's autosaves.
func gameSaves(t *testing.T) []string {
	dir := filepath.Join("..", "..", "build", "saves")
	if _, err := os.Stat(filepath.Join(dir, "SAVE1.GAM")); err != nil {
		dir = filepath.Join("..", "..", "game")
	}
	files, _ := filepath.Glob(filepath.Join(dir, "SAVE*.GAM"))
	if len(files) == 0 {
		t.Skip("game files not present")
	}
	return files
}

func TestLayoutSize(t *testing.T) {
	n := 0
	for _, b := range Layout {
		n += b.Size * b.Count
	}
	if n != Size {
		t.Fatalf("layout covers %d bytes, want %d", n, Size)
	}
}

func TestGameSaves(t *testing.T) {
	for _, p := range gameSaves(t) {
		f, err := Open(p)
		if err != nil {
			t.Fatal(err)
		}
		// Counts stay within their arrays.
		for _, c := range []struct {
			block string
			max   int
		}{{BNumColonies, 250}, {BPlanetCount, 360}, {BNumStars, 72}, {BNumPlayers, 8}, {BNumShips, 500}} {
			if n := f.U16(c.block); n < 0 || n > c.max {
				t.Errorf("%s: %s = %d", filepath.Base(p), c.block, n)
			}
		}
		for _, c := range f.Block(BPadding) {
			if c != 0 {
				t.Errorf("%s: padding is not zero", filepath.Base(p))
				break
			}
		}
		if g, err := Parse(f.Bytes()); err != nil || !bytes.Equal(g.Bytes(), f.Bytes()) {
			t.Errorf("%s: does not round-trip", filepath.Base(p))
		}
		t.Logf("%s %q stardate %d: %d stars, %d planets, %d colonies, %d players, %d ships",
			filepath.Base(p), f.Description(), f.U32(BStardate), f.U16(BNumStars), f.U16(BPlanetCount),
			f.U16(BNumColonies), f.U16(BNumPlayers), f.U16(BNumShips))
	}
}

func TestSettings(t *testing.T) {
	for _, p := range gameSaves(t) {
		f, err := Open(p)
		if err != nil {
			t.Fatal(err)
		}
		s := f.Settings()
		name := filepath.Base(p)
		if s.Version() != 0x82 {
			t.Errorf("%s: MOX.SET version 0x%x", name, s.Version())
		}
		if s.Players() != f.U16(BNumPlayers) {
			t.Errorf("%s: settings say %d players, NUM_PLAYERS %d", name, s.Players(), f.U16(BNumPlayers))
		}
		stars := map[int]int{0: 20, 1: 36, 2: 54, 3: 72}[s.GalaxySize()]
		if n := f.U16(BNumStars); n != stars && n != stars-1 {
			t.Errorf("%s: galaxy size %d but %d stars", name, s.GalaxySize(), n)
		}
		if s.Difficulty() > 4 || s.TechLevel() > 2 || s.GalaxyAge() > 2 || f.GameType() > GameModem {
			t.Errorf("%s: settings out of range", name)
		}
	}
}
