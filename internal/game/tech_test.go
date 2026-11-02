package game

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/openorion2/openorion2/internal/save"
)

// techGame loads recipe n's save with its settings and the executable.
func techGame(t *testing.T, n string) *Game {
	t.Helper()
	x, err := newExeImage("", nil)
	if err != nil {
		t.Skip(err)
	}
	g, f := recipeState(t, n)
	g.exe = x
	g.settings = f.Settings()
	g.state.Random.Set(f.RandomSeed())
	if err := g.loadHStrings(); err != nil {
		t.Fatal(err)
	}
	return g
}

func recordBytes(v any) []byte {
	var b bytes.Buffer
	if err := binary.Write(&b, binary.LittleEndian, v); err != nil {
		panic(err)
	}
	return b.Bytes()
}

// diffRecord reports the first differing offsets of two records.
func diffRecord(a, b any) []int {
	x, y := recordBytes(a), recordBytes(b)
	var d []int
	for i := range x {
		if x[i] != y[i] {
			d = append(d, i)
		}
	}
	return d
}

// TestStarAndRangeRecalculationMatchSaves: the star masks and fuel ranges
// are recomputed whenever they can change, so the saved ones must agree.
func TestStarAndRangeRecalculationMatchSaves(t *testing.T) {
	t.Parallel()
	for _, n := range singleRecipes {
		g := techGame(t, n)
		s := &g.state
		before := *s
		for i := 0; i < int(s.NumStars); i++ {
			s.computeStarColonyStuff(i)
			if d := diffRecord(before.Stars[i], s.Stars[i]); len(d) > 0 {
				t.Errorf("%s star %d: offsets %#x differ", n, i, d)
			}
		}
		g.updateAllShipRanges()
		for p := 0; p < int(s.NumPlayers); p++ {
			if d := diffRecord(before.Players[p], s.Players[p]); len(d) > 0 {
				t.Errorf("%s player %d: offsets %#x differ", n, p, d)
			}
		}
	}
}

// TestStrategicDesigns checks Update_Player_Strategic_Ship_Designs_ by hand
// on the mid-game's human player with settings 0xd8 = 1.
func TestStrategicDesigns(t *testing.T) {
	t.Parallel()
	g := techGame(t, "mid")
	g.settings[0xd8] = 1
	s := &g.state
	p := int(s.PlayerNum)
	pics := [6]uint8{}
	for k := range pics {
		pics[k] = s.Players[p].Designs[k].Picture
	}
	g.updatePlayerShipDesigns(p)
	for k := 0; k < 6; k++ {
		d := &s.Players[p].Designs[k]
		name, _ := g.hullName(k, false)
		hull := int32(g.exeI16(rowAddr(hullCostTable, hullStride, k, 0))) * 3 / 2
		want := g.costReductionForGovtType(p, hull)
		wantPic := uint8(0x2b)
		if k < 5 {
			wantPic = uint8(int8(pics[k]) + 1)
			if int(wantPic) == k*8+8 {
				wantPic = uint8(k * 8)
			}
		}
		if save.CString(d.Name[:]) != name || int(d.Size) != k || int(d.Builder) != p ||
			int32(d.Cost) != want || d.Picture != wantPic || d.BuildDate != uint16(s.Stardate) ||
			int(d.Drive) != g.bestWarpDrive(p) || int(d.Armor) != g.bestArmor(p) {
			t.Errorf("design %d: %+v (want %q cost %d picture %d)", k, *d, name, want, wantPic)
		}
	}
}

// TestEinsteinUncreative runs the uncreative and creative research paths,
// which no save exercises: an uncreative player draws from the RNG.
func TestEinsteinUncreative(t *testing.T) {
	t.Parallel()
	for _, pick := range []int{save.PickUncreative, save.PickCreative} {
		g := techGame(t, "late")
		s := &g.state
		p := int(s.PlayerNum)
		pl := &s.Players[p]
		pl.Picks[pick] = 1
		// As for an uncreative race, nothing not yet researched is offered.
		for a := range pl.AppStatus {
			if pl.AppStatus[a] == techObtainable {
				pl.AppStatus[a] = techUnknown
			}
		}
		seed := s.Random.Get()
		if n := g.allTechAdvances(p); n == 0 {
			t.Errorf("pick %d: nothing given", pick)
		}
		if moved := s.Random.Get() != seed; moved != (pick == save.PickUncreative) {
			t.Errorf("pick %d: RNG moved %v", pick, moved)
		}
	}
}
