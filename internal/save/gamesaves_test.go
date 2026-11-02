package save_test

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"

	"github.com/openorion2/openorion2/internal/game"
	"github.com/openorion2/openorion2/internal/save"
)

// The tests on whole saves: new games made from game/'s files and played
// on by the AI (game.NewSave), no one's own saves.

var gameSaveRecipes = map[string]game.SaveRecipe{
	"small": {Seed: 11, Difficulty: 4, Size: 0, Age: 0, Players: 2, TechLevel: 0, Race: 0, Turns: 30},
	"mid":   {Seed: 607, Difficulty: 1, Size: 2, Age: 1, Players: 6, TechLevel: 1, Race: 7, Turns: 200},
	"huge":  {Seed: 12, Difficulty: 2, Size: 3, Age: 2, Players: 8, TechLevel: 2, Race: 12, Color: 7, Turns: 40},
	"net":   {Seed: 13, Difficulty: 1, Size: 1, Age: 1, Players: 4, TechLevel: 1, Race: 5, Color: 2, Turns: 20, Network: 2},
}

// madeSaves are the recipes' saves, made once for all the tests.
var madeSaves struct {
	once sync.Once
	data map[string][]byte
	err  error
}

type namedSave struct {
	name string
	f    *save.File
}

// gameSaves parses each recipe's save afresh (a test may change its copy).
func gameSaves(t *testing.T) []namedSave {
	t.Helper()
	dir := filepath.Join("..", "..", "game")
	if _, err := os.Stat(filepath.Join(dir, "FONTS.LBX")); err != nil {
		t.Skip("game files not present")
	}
	madeSaves.once.Do(func() {
		madeSaves.data = map[string][]byte{}
		for name, r := range gameSaveRecipes {
			b, err := game.NewSave(dir, r)
			if err != nil {
				madeSaves.err = err
				return
			}
			madeSaves.data[name] = b
		}
	})
	if madeSaves.err != nil {
		t.Fatal(madeSaves.err)
	}
	var out []namedSave
	for name, b := range madeSaves.data {
		f, err := save.Parse(b)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		out = append(out, namedSave{name, f})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out
}

// roundTrip decodes and re-encodes every record of a block and checks the
// file is unchanged.
func roundTrip[T any](t *testing.T, f *save.File, block string, n int) []T {
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

func TestGameSaves(t *testing.T) {
	for _, gs := range gameSaves(t) {
		name, f := gs.name, gs.f
		// Counts stay within their arrays.
		for _, c := range []struct {
			block string
			max   int
		}{{save.BNumColonies, 250}, {save.BPlanetCount, 360}, {save.BNumStars, 72}, {save.BNumPlayers, 8}, {save.BNumShips, 500}} {
			if n := f.U16(c.block); n < 0 || n > c.max {
				t.Errorf("%s: %s = %d", name, c.block, n)
			}
		}
		for _, c := range f.Block(save.BPadding) {
			if c != 0 {
				t.Errorf("%s: padding is not zero", name)
				break
			}
		}
		if g, err := save.Parse(f.Bytes()); err != nil || !bytes.Equal(g.Bytes(), f.Bytes()) {
			t.Errorf("%s: does not round-trip", name)
		}
		t.Logf("%s %q stardate %d: %d stars, %d planets, %d colonies, %d players, %d ships",
			name, f.Description(), f.U32(save.BStardate), f.U16(save.BNumStars), f.U16(save.BPlanetCount),
			f.U16(save.BNumColonies), f.U16(save.BNumPlayers), f.U16(save.BNumShips))
	}
}

func TestSettings(t *testing.T) {
	for _, gs := range gameSaves(t) {
		name, f := gs.name, gs.f
		s := f.Settings()
		if s.Version() != 0x82 {
			t.Errorf("%s: MOX.SET version 0x%x", name, s.Version())
		}
		if s.Players() != f.U16(save.BNumPlayers) {
			t.Errorf("%s: settings say %d players, NUM_PLAYERS %d", name, s.Players(), f.U16(save.BNumPlayers))
		}
		stars := map[int]int{0: 20, 1: 36, 2: 54, 3: 72}[s.GalaxySize()]
		if n := f.U16(save.BNumStars); n != stars && n != stars-1 {
			t.Errorf("%s: galaxy size %d but %d stars", name, s.GalaxySize(), n)
		}
		if s.Difficulty() > 4 || s.TechLevel() > 2 || s.GalaxyAge() > 2 || f.GameType() > save.GameModem {
			t.Errorf("%s: settings out of range", name)
		}
		if want := map[bool]int{true: save.GameNetwork, false: save.GameSingle}[name == "net"]; f.GameType() != want {
			t.Errorf("%s: game type %d", name, f.GameType())
		}
	}
}

func TestPlanetsAndColonies(t *testing.T) {
	for _, gs := range gameSaves(t) {
		name, f := gs.name, gs.f
		planets := roundTrip[save.Planet](t, f, save.BPlanets, 360)
		colonies := roundTrip[save.Colony](t, f, save.BColonies, 250)
		stars := roundTrip[save.Star](t, f, save.BStars, 72)
		for i, c := range colonies[:f.U16(save.BNumColonies)] {
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
				col := save.Colonist(c.Colonists[k])
				if col.Race() > 9 || col.Job() > 2 || col>>11 != 0 {
					t.Errorf("%s colony %d colonist %d: 0x%x", name, i, k, uint32(col))
				}
			}
		}
	}
}

func TestPlayers(t *testing.T) {
	for _, gs := range gameSaves(t) {
		name, f := gs.name, gs.f
		players := roundTrip[save.Player](t, f, save.BPlayers, 8)
		n := f.U16(save.BNumPlayers)
		races, colors := map[uint8]bool{}, map[uint8]bool{}
		humans := 0
		for i, p := range players[:n] {
			if save.CString(p.EmperorName[:]) == "" || save.CString(p.RaceName[:]) == "" {
				t.Errorf("%s player %d: empty names", name, i)
			}
			if races[p.Race] || colors[p.Color] {
				t.Errorf("%s player %d: race %d or colour %d repeated", name, i, p.Race, p.Color)
			}
			races[p.Race], colors[p.Color] = true, true
			if p.Objective == save.ObjectiveHuman {
				humans++
			}
			sum := int32(0)
			for _, m := range p.MaintenanceParts {
				sum += int32(m)
			}
			if sum != p.Maintenance {
				t.Errorf("%s player %d: maintenance parts %d, total %d", name, i, sum, p.Maintenance)
			}
			if p.Eliminated == 0 && (p.HomePlanet < 0 || int(p.HomePlanet) >= f.U16(save.BPlanetCount)) {
				t.Errorf("%s player %d: home planet %d", name, i, p.HomePlanet)
			}
		}
		if humans == 0 {
			t.Errorf("%s: no human player", name)
		}
		t.Logf("%s: %s of the %s, BC %d", name, save.CString(players[0].EmperorName[:]), save.CString(players[0].RaceName[:]), players[0].BC)
	}
}

func TestShipsAndLeaders(t *testing.T) {
	for _, gs := range gameSaves(t) {
		name, f := gs.name, gs.f
		ships := roundTrip[save.Ship](t, f, save.BShips, 500)
		leaders := roundTrip[save.Leader](t, f, save.BLeaders, 67)
		roundTrip[save.MoveableBox](t, f, save.BMoveableBox, 5)
		stars, players := f.U16(save.BNumStars), f.U16(save.BNumPlayers)
		for i, s := range ships[:f.U16(save.BNumShips)] {
			if s.Status == save.ShipFree {
				continue
			}
			if s.Owner < 8 && int(s.Owner) >= players {
				t.Errorf("%s ship %d: owner %d with %d players", name, i, s.Owner, players)
			}
			loc := int(s.Location)
			if s.Status == save.ShipInTransit {
				loc -= save.LocationTransit
			} else if s.Status == save.ShipDeparting {
				loc -= save.LocationDeparted
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

func TestStars(t *testing.T) {
	for _, gs := range gameSaves(t) {
		name, f := gs.name, gs.f
		stars := roundTrip[save.Star](t, f, save.BStars, 72)
		var neb [4]save.Nebula
		if err := binary.Read(bytes.NewReader(f.Block(save.BNebula)), binary.LittleEndian, &neb); err != nil {
			t.Fatal(err)
		}
		mx, my := f.U16(save.BMapMaxX), f.U16(save.BMapMaxY)
		n := f.U16(save.BNumStars)
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
				if pl >= 0 && int(pl) >= f.U16(save.BPlanetCount) {
					t.Errorf("%s star %d orbit %d: planet %d", name, i, orbit, pl)
				}
			}
		}
	}
}
