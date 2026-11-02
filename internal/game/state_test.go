package game

import (
	"path/filepath"
	"testing"

	"github.com/openorion2/openorion2/internal/lbx"
	"github.com/openorion2/openorion2/internal/save"
)

func TestStateFromSaves(t *testing.T) {
	t.Parallel()
	dir := filepath.Join("..", "..", "game")
	files, _ := filepath.Glob(filepath.Join(savesDir(), "SAVE*.GAM"))
	if len(files) == 0 {
		t.Skip("no saves in game/")
	}
	for _, p := range files {
		f, err := save.Open(p)
		if err != nil {
			t.Fatal(err)
		}
		g := &Game{dir: dir, lbx: map[string]*lbx.Archive{}, Log: t.Logf}
		s := &g.state
		if err := s.readBlocks(f, 9); err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		if int(s.NumStars) != f.U16(save.BNumStars) || int(s.PlayerNum) != f.U16(save.BPlayerNum) {
			t.Fatalf("%s: counts differ", p)
		}
		// Update_Star_Owners_ also runs after each turn, so the saved owners
		// must come out unchanged.
		saved := s.Stars
		s.updateStarOwners()
		for i := 0; i < int(s.NumStars); i++ {
			if saved[i].Owner != s.Stars[i].Owner {
				t.Fatalf("%s: star %d owner %d, saved %d", p, i, s.Stars[i].Owner, saved[i].Owner)
			}
		}
		savedLeaders := s.Leaders
		if err := g.loadLeaderTitles(); err != nil {
			t.Fatal(err)
		}
		// The saves are English games, so HERODATA.LBX gives the same titles.
		for i := range s.Leaders {
			if s.Leaders[i] != savedLeaders[i] {
				t.Fatalf("%s: leader %d changed by Load_Leader_Titles_", p, i)
			}
		}
	}
}
