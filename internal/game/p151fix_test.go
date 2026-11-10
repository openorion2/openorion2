package game

import (
	"testing"
)

// TestBlockadedStaleFood: a blockaded AI colony on a planet that grows
// nothing for its owner, left with the previous owner's food per farmer
// (Biomorphic Fungi), puts its free colonists to work (1.40b23 froze).
func TestBlockadedStaleFood(t *testing.T) {
	t.Parallel()
	g := techGame(t, "mid")
	s := &g.state
	ci := -1
	for i := 0; i < int(s.NumColonies); i++ {
		if c := &s.Colonies[i]; int(c.Owner) != int(s.PlayerNum) && c.Owner >= 0 && c.IsOutpost == 0 && c.PopCount >= 3 {
			ci = i
			break
		}
	}
	if ci == -1 {
		t.Fatal("no AI colony")
	}
	c := &s.Colonies[ci]
	s.Planets[c.Planet].FoodPerFarmer = 0
	s.Players[c.Owner].AppStatus[0x1d] = 0
	for i := 0; i < int(c.PopCount); i++ {
		c.Colonists[i] = c.Colonists[i]&^0xf | uint32(c.Owner) // no natives or androids
	}
	c.FoodPerFarmer = 2
	ca := &colonyAI{p: int(c.Owner)}
	r := &aiColonyRec{colony: uint8(ci)}
	func() {
		defer func() {
			if e := recover(); e != nil {
				t.Fatal(e)
			}
		}()
		g.doBlockadedColony(ca, r)
	}()
	if r.farmers != 0 || r.first != r.end {
		t.Errorf("farmers %d, free %d..%d", r.farmers, r.first, r.end)
	}
	for i := 0; i < int(c.PopCount); i++ {
		if j := c.Colonists[i] & colJobMask; j == 0 {
			t.Errorf("colonist %d farms", i)
		}
	}
}
