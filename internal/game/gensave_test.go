package game

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"

	"github.com/openorion2/openorion2/internal/lbx"
	"github.com/openorion2/openorion2/internal/save"
)

// The tests' saves: made from the game's own data by NewSave, never
// anyone's save files. A recipe's save is the same on every machine with
// the GOG 1.40b23 files, so tests may pin its values.

// saveRecipes are the named games the tests load.
var saveRecipes = map[string]SaveRecipe{
	// Turn 1 of a new game: home worlds and starting ships only.
	"start": {Seed: 101, Difficulty: 1, Size: 1, Age: 1, Players: 5, TechLevel: 1, Race: 7, Color: 0},
	// Mid-game: colonies, ships at stars and in flight, tech, contacts.
	"mid": {Seed: 607, Difficulty: 1, Size: 2, Age: 1, Players: 6, TechLevel: 1, Race: 7, Color: 0, Turns: 200},
	// A crowded small galaxy played long: wars, an eliminated empire.
	"late": {Seed: 303, Difficulty: 3, Size: 0, Age: 2, Players: 5, TechLevel: 2, Race: 2, Color: 3, Turns: 160},
	// A large galaxy, advanced start, few turns.
	"large": {Seed: 404, Difficulty: 0, Size: 3, Age: 0, Players: 8, TechLevel: 2, Race: 11, Color: 5, Turns: 25},
	// "mid" saved as a network game of three humans.
	"net3": {Seed: 607, Difficulty: 1, Size: 2, Age: 1, Players: 6, TechLevel: 1, Race: 7, Color: 0, Turns: 200, Network: 3},
}

// singleRecipes are the single-player games, as the original's own saves.
var singleRecipes = []string{"start", "mid", "late", "large"}

// recipeNames are saveRecipes' names in order.
func recipeNames() []string {
	var ns []string
	for n := range saveRecipes {
		ns = append(ns, n)
	}
	sort.Strings(ns)
	return ns
}

type generated struct {
	once sync.Once
	data []byte
	err  error
}

// generatedSaves holds each recipe's save once made in this process: a
// recipe is played once however many tests load it (each gets a copy).
var generatedSaves sync.Map // name → *generated

func gameFilesDir() string { return filepath.Join("..", "..", "game") }

// recipeSave is recipe name's save. Without game/ it skips (in the
// caller's goroutine: call it before host.Run).
func recipeSave(t testing.TB, name string) []byte {
	t.Helper()
	r, ok := saveRecipes[name]
	if !ok {
		t.Fatalf("no save recipe %q", name)
	}
	if _, err := os.Stat(filepath.Join(gameFilesDir(), "FONTS.LBX")); err != nil {
		t.Skip("no game files: ", err)
	}
	v, _ := generatedSaves.LoadOrStore(name, &generated{})
	gen := v.(*generated)
	gen.once.Do(func() { gen.data, gen.err = makeSave(r) })
	if gen.err != nil {
		t.Fatalf("save %q: %v", name, gen.err)
	}
	return append([]byte(nil), gen.data...)
}

// makeSave plays r on game/'s files.
func makeSave(r SaveRecipe) ([]byte, error) { return NewSave(gameFilesDir(), r) }

// recipeSaveFile writes recipe name's save as SAVE<slot>.GAM into a
// temporary directory and returns its path.
func recipeSaveFile(t *testing.T, name string, slot int) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), fmt.Sprintf("SAVE%d.GAM", slot))
	if err := os.WriteFile(p, recipeSave(t, name), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// putRecipeSave writes recipe name's save as SAVE<slot>.GAM into dir (a
// scratch game directory).
func putRecipeSave(t *testing.T, name, dir string, slot int) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("SAVE%d.GAM", slot)), recipeSave(t, name), 0o644); err != nil {
		t.Fatal(err)
	}
}

// recipeFile is recipe name's save parsed.
func recipeFile(t testing.TB, name string) *save.File {
	t.Helper()
	f, err := save.Parse(recipeSave(t, name))
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// recipeState is a Game without screens holding recipe name's state (the
// blocks only, as the tests of a routine on a state want).
func recipeState(t testing.TB, name string) (*Game, *save.File) {
	t.Helper()
	f := recipeFile(t, name)
	g := &Game{dir: gameFilesDir(), lbx: map[string]*lbx.Archive{}, Log: t.Logf}
	if err := g.state.readBlocks(f, 9); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return g, f
}

// TestRecipeSaves: every recipe makes a save that loads, the same each
// time, with the state its comment promises.
func TestRecipeSaves(t *testing.T) {
	t.Parallel()
	for _, name := range recipeNames() {
		g, f := recipeState(t, name)
		s := &g.state
		r := saveRecipes[name]
		alive, colonies, transit := 0, 0, 0
		for p := 0; p < int(s.NumPlayers); p++ {
			if s.Players[p].Eliminated == 0 {
				alive++
			}
		}
		for i := 0; i < int(s.NumColonies); i++ {
			if c := &s.Colonies[i]; c.Owner == 0 && c.IsOutpost == 0 {
				colonies++
			}
		}
		for i := 0; i < int(s.NumShips); i++ {
			if s.Ships[i].Status == save.ShipInTransit {
				transit++
			}
		}
		t.Logf("%s: stardate %d, %d players (%d alive), %d stars, %d colonies (%d the human's), %d ships (%d in flight), game type %d",
			name, s.Stardate, s.NumPlayers, alive, s.NumStars, s.NumColonies, colonies, s.NumShips, transit, f.GameType())
		if int(s.NumPlayers) != r.Players || s.Players[0].Objective != save.ObjectiveHuman || int(s.Players[0].Race) != r.Race {
			t.Errorf("%s: %d players, human objective %d race %d", name, s.NumPlayers, s.Players[0].Objective, s.Players[0].Race)
		}
		if r.Turns > 0 && (colonies < 2 || transit == 0) {
			t.Errorf("%s: %d colonies of the human, %d ships in flight", name, colonies, transit)
		}
		if name == "late" && alive == int(s.NumPlayers) {
			t.Errorf("late: nobody eliminated")
		}
		if want := map[bool]int{false: save.GameSingle, true: save.GameNetwork}[r.Network > 0]; f.GameType() != want {
			t.Errorf("%s: game type %d", name, f.GameType())
		}
	}
	// Generation is deterministic: once more, uncached.
	b, err := makeSave(saveRecipes["start"])
	if err != nil {
		t.Fatal(err)
	}
	if a := recipeSave(t, "start"); string(a) != string(b) {
		t.Error("the start recipe differs between two runs")
	}
}
