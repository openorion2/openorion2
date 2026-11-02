package game

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/openorion2/openorion2/internal/host"
	"github.com/openorion2/openorion2/internal/save"
)

// port-only: saves made from the game's own data, for the tests (which need
// nobody's save files): a new game as Init_New_Game_ makes it, played on
// by the AI for every empire as under -autopilot, without screens.

// SaveRecipe is such a game. The same recipe and game files give the same
// save, byte for byte.
type SaveRecipe struct {
	Seed uint32 // the command line seed (-seed), not 0
	// The new game screen's settings: 0..4, 0..3, 0..2, 2..8, 0..2.
	Difficulty, Size, Age, Players, TechLevel int
	Race, Color                               int  // the human's race (0..12) and banner (0..7)
	NoEvents, NoAntarans                      bool // random events and the Antarans off
	Turns                                     int  // played by the AI, the human's empire too
	// Network > 0 saves it as a network game whose players 0..Network-1
	// are its humans (net ids 1..Network); player 0 is the viewer.
	Network int
}

// NewSave plays r on the game files in gameDir, headless, and returns the
// save. The new game's own files go into a scratch directory, never there.
func NewSave(gameDir string, r SaveRecipe) ([]byte, error) {
	dir, err := os.MkdirTemp("", "openorion2-save")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	ents, err := os.ReadDir(gameDir)
	if err != nil {
		return nil, err
	}
	for _, e := range ents {
		up := strings.ToUpper(e.Name())
		if !strings.HasSuffix(up, ".LBX") && up != "ORION2.EXE" {
			continue
		}
		from, err := filepath.Abs(filepath.Join(gameDir, e.Name()))
		if err != nil {
			return nil, err
		}
		if err := os.Symlink(from, filepath.Join(dir, e.Name())); err != nil {
			return nil, err
		}
	}
	var data []byte
	err = host.Run(host.Options{Headless: true}, func(sys *host.System) error {
		var err error
		data, err = generateSave(sys, dir, r)
		return err
	})
	return data, err
}

// generateSave plays r in dir (a scratch game directory: the new game
// writes SAVE10.GAM and MOX.SET there).
func generateSave(sys *host.System, dir string, r SaveRecipe) ([]byte, error) {
	g, err := newGame(sys, dir)
	if err != nil {
		return nil, err
	}
	g.Log = func(string, ...any) {}
	g.setDefaultGameSettings() // not MOX.SET: the same rules everywhere
	st := &g.settings
	copy(st[0x17:0x1f], "MOX-0\x00\x00\x00") // not the clock's digit
	st[0xd4], st[0xd6], st[0xd7], st[0xd5], st[0xd9] =
		byte(r.Difficulty), byte(r.Size), byte(r.Age), byte(r.Players), byte(r.TechLevel)
	if r.NoEvents {
		st[2] = 0
	}
	if r.NoAntarans {
		st[0xd3] = 0
	}
	if err := g.humanPlayer(r.Race, r.Color); err != nil {
		return nil, err
	}
	g.cmdLineSeed = r.Seed
	if err := g.initNewGame(int16(r.Difficulty), int16(r.Size), int16(r.Age), int16(r.Players)); err != nil {
		return nil, err
	}
	s := &g.state
	// The autopilot's hand-over (Main_Screen_), back to the human at the end.
	g.allAIGame, g.noCouncil, g.autopilot = true, true, true
	g.makePlayerIntoAI(0)
	for k := 0; k < r.Turns && g.nLivingPlayers() > 1 && s.CouncilWinner < 0; k++ {
		g.nextTurnCalc()
	}
	s.Players[0].Objective = save.ObjectiveHuman
	if r.Network > 0 {
		g.gameType = save.GameNetwork
		for k := range s.Players {
			s.Players[k].NetID = -1
		}
		for k := 0; k < r.Network && k < int(s.NumPlayers); k++ {
			s.Players[k].Objective = save.ObjectiveHuman
			s.Players[k].NetID = int16(k + 1)
		}
	}
	setCString(s.Descriptions[0][:], fmt.Sprintf("seed %d turn %d", r.Seed, s.Stardate-35000))
	return g.encodeSave(0), nil
}

// humanPlayer is what the race selection screen leaves in player 0 for
// race and banner color: its picks, names and the human objective.
func (g *Game) humanPlayer(race, color int) error {
	pl := &g.state.Players[0]
	pl.Objective = save.ObjectiveHuman
	pl.Race, pl.Color = uint8(race), uint8(color)
	sp, err := g.raceSpecialsRecord(race)
	if err != nil {
		return err
	}
	setPicks(pl, sp)
	for k := 1; k < 10; k++ {
		if pl.Picks[k] != 0 {
			pl.Picks[k] = g.raceSelPickValue(k, pl.Picks[k])
		}
	}
	name, err := g.eStringAt(raceNames + 4*uint32(race))
	if err != nil {
		return err
	}
	strcpyField(pl.RaceName[:], name)
	a, err := g.archive(cStringAt(g.exe, raceNamesLBX))
	if err != nil {
		return err
	}
	t, err := a.Table(0)
	if err != nil {
		return err
	}
	strcpyField(pl.EmperorName[:], save.CString(t.Record(race<<3)))
	return nil
}
