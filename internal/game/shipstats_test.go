package game

import (
	"testing"

	"github.com/openorion2/openorion2/internal/save"
)

func shipStatsGame(t *testing.T) *Game {
	x, err := newExeImage("", nil)
	if err != nil {
		t.Skip(err)
	}
	return &Game{exe: x}
}

// TestShipStatsTables checks hand-read values of the hull and armour tables.
func TestShipStatsTables(t *testing.T) {
	t.Parallel()
	g := shipStatsGame(t)
	s := &g.state.Ships[0]
	set := func(size, armor uint8, owner int8, specials ...int) {
		*s = save.Ship{Owner: owner}
		s.Design.Size, s.Design.Armor = size, armor
		for _, b := range specials {
			s.Design.Specials[b>>3] |= 1 << (b & 7)
		}
	}
	cases := []struct {
		size, armor uint8
		owner       int8
		specials    []int
		st, ar      int
	}{
		{0, 1, 0, nil, 4, 4},                 // frigate, titanium
		{0, 0, 0, nil, 0, 0},                 // no armour: -100%
		{3, 3, 0, nil, 200, 200},             // battleship, zortrium
		{3, 3, 0, []int{27}, 600, 200},       // reinforced hull
		{3, 3, 0, []int{14}, 200, 600},       // heavy armour
		{5, 6, 2, []int{14, 27}, 4500, 4500}, // doom star, xentronium
		{4, 2, 9, nil, 160, 160},             // Guardian uses its design
	}
	for _, c := range cases {
		set(c.size, c.armor, c.owner, c.specials...)
		if st, ar := g.shipStructure(0), g.shipArmorHits(0); st != c.st || ar != c.ar {
			t.Errorf("%+v: structure %d armour %d", c, st, ar)
		}
		if m := g.maxShipHits(0); m != c.st+c.ar {
			t.Errorf("%+v: max %d", c, m)
		}
	}
	set(2, 1, 14)
	s.Design.Drive = 1
	if st := g.shipStructure(0); st != 1500 {
		t.Errorf("Hydra structure %d, want 1500", st)
	}
	s.StructureDamage, s.ArmorDamage = 100, 5
	if c := g.currentShipHits(0); c != 1500+30-105 {
		t.Errorf("Hydra current %d", c)
	}
	// Strategic combat: hull total, raised by the owner's best armour.
	g.settings[0xd8] = 1
	if m, c := g.maxShipHits(0), g.currentShipHits(0); m != 25 || c != -75 {
		t.Errorf("strategic Hydra: max %d current %d", m, c)
	}
	set(5, 1, 8)
	if m := g.maxShipHits(0); m != 3000 {
		t.Errorf("strategic Antaran doom star: max %d, want 3000", m)
	}
	set(1, 1, 0)
	if m := g.maxShipHits(0); m != 15 { // no armour tech known: best is titanium, +0%
		t.Errorf("strategic destroyer max %d, want 15", m)
	}
}

func TestShipStatsSaves(t *testing.T) {
	t.Parallel()
	x, err := newExeImage("", nil)
	if err != nil {
		t.Skip(err)
	}
	for _, p := range singleRecipes {
		g, _ := recipeState(t, p)
		g.exe = x
		s := &g.state
		for i := 0; i < int(s.NumPlayers); i++ {
			if b := g.bestArmor(i); b < 1 || b > 6 {
				t.Errorf("%s: player %d best armour %d", p, i, b)
			}
		}
		for _, simple := range []byte{0, 1} {
			g.settings[0xd8] = simple
			n := 0
			for i := 0; i < int(s.NumShips); i++ {
				sh := &s.Ships[i]
				if sh.Status == save.ShipFree {
					continue
				}
				m, c := g.maxShipHits(i), g.currentShipHits(i)
				if m <= 0 || c > m || c <= 0 {
					t.Errorf("%s: ship %d (owner %d size %d) hits %d/%d", p, i, sh.Owner, sh.Design.Size, c, m)
				}
				if simple == 0 && m != g.shipStructure(i)+g.shipArmorHits(i) {
					t.Errorf("%s: ship %d max %d", p, i, m)
				}
				if g.isCombatShip(i) {
					n++
				}
			}
			if simple == 0 {
				t.Logf("%s: %d ships, %d combat", p, s.NumShips, n)
			}
		}
	}
}
