package game

import (
	"testing"

	"github.com/openorion2/openorion2/internal/save"
)

// buildScoreColony is the late game's first colony of the human, made
// "not busy" (barracks and automated factory built) with a neutral AI
// record: no threats, no objective, a non-lithovore race.
func buildScoreColony(t *testing.T) (*Game, int) {
	g := sysInfoGame(t, "late")
	g.computeAIData()
	s := &g.state
	ci := -1
	for i := range s.Colonies[:s.NumColonies] {
		if int(s.Colonies[i].Owner) == int(s.PlayerNum) && s.Colonies[i].IsOutpost == 0 {
			ci = i
			break
		}
	}
	if ci < 0 {
		t.Fatal("no colony")
	}
	c := &s.Colonies[ci]
	c.Buildings[2], c.Buildings[7], c.Buildings[0x16] = 1, 1, 1
	pl := &s.Players[c.Owner]
	pl.Objective, pl.Unknown59D = 0, 0
	info := g.ai.colonyInfo(ci)
	clear(info)
	info[2] = 1
	return g, ci
}

// TestBuildingScores151: the six building scores community patch 1.51
// changes for the AI and AUTO build.
func TestBuildingScores151(t *testing.T) {
	t.Parallel()
	g, ci := buildScoreColony(t)
	c := &g.state.Colonies[ci]
	pl := &g.state.Players[c.Owner]
	score := func(b int) int32 { return g.colonyBuildingScore(ci, b) }

	c.PopCount = 7
	if v := score(4); v != 7 {
		t.Errorf("Astro University = %d, want the pops, 7", v)
	}
	if v := score(33); v != 7 {
		t.Errorf("Recyclotron = %d, want the pops, 7", v)
	}
	pl.Objective = 3
	if v := score(6); v != 13 {
		t.Errorf("Autolab with objective 3 = %d, want 13", v)
	}
	pl.Objective = 0

	for _, b := range []int{29, 39} {
		c.PopCount = 3
		if v := score(b); v != 0 {
			t.Errorf("building %d at 3 pops = %d, want 0", b, v)
		}
		c.PopCount = 4
		if v := score(b); v == 0 {
			t.Errorf("building %d at 4 pops scores 0", b)
		}
	}

	c.PopCount = 5
	c.Output[save.Food] = 5
	if v := score(16); v != 0 {
		t.Errorf("Food Replicators on a fed colony = %d, want 0", v)
	}
	c.Output[save.Food] = 4
	if v := score(16); v == 0 {
		t.Error("Food Replicators on an underfed colony scores 0")
	}
}

// TestAutobuildKeepsColony: AUTO build picks the next building without
// recalculating the colony (community patch 1.51).
func TestAutobuildKeepsColony(t *testing.T) {
	t.Parallel()
	g, ci := buildScoreColony(t)
	c := &g.state.Colonies[ci]
	c.Output[save.Industry] = 9999
	g.playerColonyAutobuild(ci)
	if c.Output[save.Industry] != 9999 {
		t.Errorf("industry %d: the colony was recalculated", c.Output[save.Industry])
	}
}
