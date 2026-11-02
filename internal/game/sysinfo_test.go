package game

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/openorion2/openorion2/internal/engine"
	"github.com/openorion2/openorion2/internal/font"
	"github.com/openorion2/openorion2/internal/gfx"
	"github.com/openorion2/openorion2/internal/lbx"
	"github.com/openorion2/openorion2/internal/save"
)

// sysInfoGame is a Game with the exe, the H strings, a video page and the
// font, and SAVE<n>.GAM loaded when n is not empty.
func sysInfoGame(t *testing.T, n string) *Game {
	dir := filepath.Join("..", "..", "game")
	x, err := newExeImage("", nil)
	if err != nil {
		t.Skip(err)
	}
	g := &Game{dir: dir, lbx: map[string]*lbx.Archive{}, cache: map[cacheKey]*gfx.Picture{}, Log: t.Logf,
		e: &engine.Engine{V: engine.NewVideo()}, exe: x}
	if err := g.loadHStrings(); err != nil {
		t.Skip(err) // no game files
	}
	fonts, err := g.archive("FONTS.LBX")
	if err != nil {
		t.Skip(err)
	}
	fe, err := fonts.Entry(0)
	if err != nil {
		t.Fatal(err)
	}
	fnt, err := font.Parse(fe)
	if err != nil {
		t.Fatal(err)
	}
	g.e.SetFont(fnt)
	if n != "" {
		f, err := save.Open(filepath.Join(savesDir(), "SAVE"+n+".GAM"))
		if err != nil {
			t.Skip(err)
		}
		if err := g.state.readBlocks(f, 9); err != nil {
			t.Fatal(err)
		}
	}
	return g
}

func TestSysInfoStrings(t *testing.T) {
	t.Parallel()
	g := sysInfoGame(t, "")
	es, err := g.eStrings()
	if err != nil {
		t.Fatal(err)
	}
	want := map[int]string{
		planetSizeEStr[0]: "Tiny", planetSizeEStr[4]: "Huge",
		planetMineralEStr[0]: "Ultra Poor", planetMineralEStr[2]: "Abundant",
		planetGravityEStr[0]: "Low G", planetGravityEStr[2]: "Heavy G",
		planetClimateEStr[0]: "Toxic", planetClimateEStr[9]: "Gaia",
		planetSpecialEStr[1]: "Stable Wormhole", planetSpecialEStr[15]: "Lifeless",
		eStringPatch: "maintext.lbx",
	}
	for i, w := range want {
		if es[i] != w {
			t.Errorf("E %#x = %q, want %q", i, es[i], w)
		}
	}
	for id, w := range map[int]string{0: "No Building", 1: "Alien Control Center", 8: "Battlestation",
		15: "Biospheres", 40: "Star Base", 41: "Star Fortress", 48: "Artificial Planet"} {
		if n, err := g.buildingName(id); err != nil || n != w {
			t.Errorf("building %d = %q, %v; want %q", id, n, err, w)
		}
	}
	for _, c := range []struct {
		size   int
		plural bool
		want   string
	}{{0, false, "Frigate"}, {0, true, "Frigates"}, {5, false, "Doom Star"}, {5, true, "Doom Stars"}} {
		if n, err := g.hullName(c.size, c.plural); err != nil || n != c.want {
			t.Errorf("hull %d/%v = %q, %v; want %q", c.size, c.plural, n, err, c.want)
		}
	}
}

func TestPlanetMaxPopulation(t *testing.T) {
	t.Parallel()
	g := sysInfoGame(t, "")
	s := &g.state
	s.Planets[0] = save.Planet{Colony: -1, Type: 3, Size: 4, Climate: 8}
	pl := &s.Players[0]
	check := func(what string, want int16) {
		t.Helper()
		if got := g.planetMaxPopulationForPlayer(0, 0); got != want {
			t.Errorf("%s: %d, want %d", what, got, want)
		}
	}
	check("huge terran", 20)
	pl.Picks[save.PickTolerant] = 1
	check("tolerant", 25)
	pl.Picks[save.PickTolerant] = 0
	pl.Picks[save.PickAquatic] = 1
	check("aquatic terran = gaia", 25)
	s.Planets[0].Climate = 3
	check("aquatic desert", 6) // (25*25+50)/100
	pl.Picks[save.PickSubterranean] = 1
	check("subterranean", 16)
	pl.AppStatus[3] = 3
	check("application 3", 21)
	// A colony takes the best race living there; the planet's player is ignored.
	*pl = save.Player{}
	s.Planets[0] = save.Planet{Colony: 0, Type: 3, Size: 2, Climate: 2}
	s.Colonies[0] = save.Colony{Owner: 0, Planet: 0, PopCount: 2, Climate: 8}
	s.Colonies[0].Colonists[0] = 0 // race 0
	s.Colonies[0].Colonists[1] = 8 // androids: tolerant
	s.Colonies[0].Buildings[15] = 1
	check("colony", 17) // 15 (androids) + 2 biospheres
	s.Colonies[0].IsOutpost = 1
	check("outpost uses the planet climate", 4) // (25*15+50)/100

	// In the saves no live colony has more pop than its limit.
	for _, n := range []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10"} {
		g := sysInfoGame(t, n)
		s := &g.state
		over := 0
		for c := 0; c < int(s.NumColonies); c++ {
			col := &s.Colonies[c]
			if col.Owner < 0 || col.IsOutpost != 0 || col.Planet < 0 {
				continue
			}
			if m := g.planetMaxPopulationForPlayer(int(col.Planet), int(col.Owner)); int16(col.PopCount) > m {
				over++
				t.Logf("SAVE%s colony %d: pop %d > max %d", n, c, col.PopCount, m)
			}
		}
		if over > 0 {
			t.Errorf("SAVE%s: %d colonies over their limit", n, over)
		}
	}
}

// SAVE1, Beta Ceti III: a human colony with a battlestation.
func TestPlanetInfoBetaCeti(t *testing.T) {
	t.Parallel()
	g := sysInfoGame(t, "1")
	s, tx := &g.state, g.e.Text()
	pl := -1
	for i := 0; i < int(s.PlanetCount); i++ {
		if g.planetNameNoPrime(i) == "Beta Ceti III" {
			pl = i
		}
	}
	if pl < 0 {
		t.Skip("no Beta Ceti III")
	}
	w, h, err := g.doPrintPlanetInfo(0, 0, pl, false)
	if err != nil {
		t.Fatal(err)
	}
	lh := int16(tx.Height() + 1)
	if ww := int16(tx.Width([]byte("Beta Ceti III (Human)"))); w != ww {
		t.Errorf("w = %d, want %d", w, ww)
	}
	// Four lines, the building gap (style + 3), one building line, +2.
	if wh := 0xc + 3*lh + 5 + lh + 2; h != wh {
		t.Errorf("h = %d, want %d", h, wh)
	}
	if err := g.drawPlanetInfoBox(100, 100, pl); err != nil {
		t.Fatal(err)
	}
}

// Every planet and every ship stack of the saves measures and prints.
func TestSysInfoAllSaves(t *testing.T) {
	t.Parallel()
	for _, n := range []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10"} {
		g := sysInfoGame(t, n)
		s, tx := &g.state, g.e.Text()
		for pl := 0; pl < int(s.PlanetCount); pl++ {
			w, h, err := g.doPrintPlanetInfo(0, 0, pl, false)
			if err != nil {
				t.Fatal(err)
			}
			lh := int16(tx.Height() + 1)
			if w <= 0 || w > 0x13f || h < 0xe || (s.Planets[pl].Type == 3 && h < 0xc+3*lh+2) {
				t.Errorf("SAVE%s planet %d: %dx%d", n, pl, w, h)
			}
			if err := g.drawPlanetInfoBox(20, 40, pl); err != nil {
				t.Fatal(err)
			}
		}
		g.findShipStacks()
		for k := 0; k < int(g.stacks.count); k++ {
			node := g.stacks.start[k]
			w, h, err := g.doPrintSystemShipData(0, 0, node, false)
			if err != nil {
				t.Fatal(err)
			}
			if w <= 0 || h <= 0 {
				t.Errorf("SAVE%s node %d: %dx%d", n, node, w, h)
			}
			if err := g.drawShipInfoBox(20, 40, node); err != nil {
				t.Fatal(err)
			}
		}
		for star := 0; star < int(s.NumStars); star++ {
			for _, gate := range []int{0, 3} {
				if err := g.printGateInfo(star, gate, 20, 40); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
}

// The ship box counts by design type; its height is a line per non-empty
// group plus the header.
func TestShipInfoCounts(t *testing.T) {
	t.Parallel()
	g := sysInfoGame(t, "")
	s, st := &g.state, &g.stacks
	s.NumPlayers, s.NumShips = 1, 6
	copy(s.Players[0].RaceName[:], "Human")
	for i, d := range []struct{ typ, size uint8 }{{0, 2}, {0, 2}, {0, 9}, {1, 0}, {4, 0}, {4, 0}} {
		s.Ships[i] = save.Ship{X: 5, Y: 7}
		s.Ships[i].Design.Type, s.Ships[i].Design.Size = d.typ, d.size
	}
	n := s.nShipsByTypeAtLocation(5, 7, 0)
	if n.total != 6 || n.hulls[2] != 2 || n.hulls[5] != 1 || n.colony != 1 || n.outpost != 2 || n.transport != 0 {
		t.Fatalf("counts %+v", n)
	}
	st.node[0] = shipNode{0, -1, 0}
	w, h, err := g.doPrintSystemShipData(0, 0, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	lh := int16(g.e.Text().Height() + 1)
	if h != 5*lh {
		t.Errorf("h = %d, want %d", h, 5*lh)
	}
	tx := g.e.Text()
	widest := int16(tx.Width([]byte(fmt.Sprintf(g.hMessage(0x167), 2)))) + 3
	if hw := int16(tx.Width([]byte("Human Fleet"))); hw > widest {
		widest = hw
	}
	if w < widest {
		t.Errorf("w = %d, want at least %d", w, widest)
	}
}

// Two colony ships print H 0x165 with the previous line's first four bytes
// as the count.
func TestShipInfoColonyShipsQuirk(t *testing.T) {
	t.Parallel()
	g := sysInfoGame(t, "")
	s, tx := &g.state, g.e.Text()
	s.NumPlayers, s.NumShips = 1, 4
	copy(s.Players[0].RaceName[:], "Human")
	for i, typ := range []uint8{0, 0, 1, 1} {
		s.Ships[i] = save.Ship{X: 1, Y: 1}
		s.Ships[i].Design.Type, s.Ships[i].Design.Size = typ, 2
	}
	g.stacks.node[0] = shipNode{0, -1, 0}
	w, _, err := g.doPrintSystemShipData(0, 0, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	hull, _ := g.hullName(2, true)
	prev := []byte(fmt.Sprintf("%2d %s", 2, hull))
	v := int32(prev[0]) | int32(prev[1])<<8 | int32(prev[2])<<16 | int32(prev[3])<<24
	want := int16(tx.Width([]byte(fmt.Sprintf(g.hMessage(0x165), v)))) + 3
	if w != want {
		t.Errorf("w = %d, want %d (%d)", w, want, v)
	}
}
