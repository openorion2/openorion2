package game

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/openorion2/openorion2/internal/lbx"
	"github.com/openorion2/openorion2/internal/save"
)

func TestEncodeSaveRoundTrip(t *testing.T) {
	t.Parallel()
	dir := filepath.Join("..", "..", "game")
	files, _ := filepath.Glob(filepath.Join(savesDir(), "SAVE*.GAM"))
	if len(files) == 0 {
		t.Skip("no saves in game/")
	}
	for _, p := range files {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		f, err := save.Parse(data)
		if err != nil {
			t.Fatal(err)
		}
		g := &Game{dir: dir, lbx: map[string]*lbx.Archive{}, Log: t.Logf}
		if err := g.state.readBlocks(f, 9); err != nil {
			t.Fatal(err)
		}
		g.settings = f.Settings()
		g.gameType = int(g.state.GameType) // as Load_Game_ sets it
		g.state.Random.Set(f.RandomSeed())
		if got := g.encodeSave(9); !bytes.Equal(got, data) {
			t.Errorf("%s: Save_Game_ bytes differ from the file", p)
		}
	}
}

func colonyGame(owners map[int]uint8, n int) *Game {
	g := &Game{}
	g.state.NumStars = int16(n)
	for i, m := range owners {
		g.state.Stars[i].ColonyOwners = m
	}
	return g
}

func TestCycleStarIcons(t *testing.T) {
	t.Parallel()
	g := colonyGame(map[int]uint8{2: 1, 5: 1, 7: 2}, 9)
	cases := []struct {
		cur  int
		back bool
		want int
	}{
		{-1, false, 2}, {2, false, 5}, {5, false, 2}, {8, false, 2},
		{5, true, 2}, {2, true, 5}, {0, true, 5}, {-1, true, 5},
	}
	for _, c := range cases {
		if got := g.cycleStarIcons(c.cur, 0, c.back); got != c.want {
			t.Errorf("cycle(%d, back %v) = %d, want %d", c.cur, c.back, got, c.want)
		}
	}
	if got := colonyGame(nil, 4).cycleStarIcons(1, 0, false); got != -1 {
		t.Errorf("no colonies: %d, want -1", got)
	}
}

func TestNStarRelocations(t *testing.T) {
	t.Parallel()
	g := &Game{}
	g.state.NumStars = 3
	for i := range g.state.Stars {
		for p := range g.state.Stars[i].Relocation {
			g.state.Stars[i].Relocation[p] = -1
		}
	}
	g.state.Stars[0].Relocation[1] = 2
	g.state.Stars[2].Relocation[1] = 0
	g.state.Stars[5].Relocation[1] = 0 // past NUM_STARS
	if n := g.nStarRelocations(1); n != 2 {
		t.Errorf("relocations = %d, want 2", n)
	}
}

func TestTechAppName(t *testing.T) {
	t.Parallel()
	dir := filepath.Join("..", "..", "game")
	if _, err := os.Stat(filepath.Join(dir, "TECHNAME.LBX")); err != nil {
		t.Skip("no TECHNAME.LBX in game/")
	}
	g := &Game{dir: dir, lbx: map[string]*lbx.Archive{}}
	for a, want := range map[int]string{0: "No Tech", 1: "Achilles Targeting Unit"} {
		got, err := g.techAppName(a)
		if err != nil || got != want {
			t.Errorf("app %d = %q, %v; want %q", a, got, err, want)
		}
	}
}

func TestCFormat(t *testing.T) {
	t.Parallel()
	if got := cFormat("%s has %ld bcs"); got != "%s has %d bcs" {
		t.Errorf("cFormat = %q", got)
	}
}
