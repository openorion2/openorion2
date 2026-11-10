package game

import (
	"testing"

	"github.com/openorion2/openorion2/internal/engine"
	"github.com/openorion2/openorion2/internal/font"
)

// TestColonyStoredProd: community patch 1.51 shows the turns left at 531 and
// the stored production (with bought) over the product's cost right at 625.
func TestColonyStoredProd(t *testing.T) {
	t.Parallel()
	g := sysInfoGame(t, "start")
	ci := -1
	for i := range g.state.Colonies[:g.state.NumColonies] {
		if g.state.Colonies[i].Owner == int8(g.state.PlayerNum) {
			ci = i
			break
		}
	}
	if ci < 0 {
		t.Fatal("no player colony")
	}
	g.colonyHandle = ci
	c := &g.state.Colonies[ci]
	c.Queue[0] = 5
	c.Production, c.Pending = 37, 12
	cost := g.colonyProductCost(c, 5)
	if cost <= 0 {
		t.Fatalf("cost %d", cost)
	}
	if err := g.drawTurnCount(); err != nil {
		t.Fatal(err)
	}
	got := append([]byte(nil), g.e.V.Current...)

	want := &engine.Engine{V: engine.NewVideo()}
	want.SetFont(g.e.Text().Printer.Font)
	tx := want.Text()
	tx.SetStyleShadow(0, g.colors(colorsFont), font.ShadowDown)
	if err := tx.Print(531, 103, "1 turn(s)"); err != nil {
		t.Fatal(err)
	}
	if err := tx.PrintRight(625, 103, cSprintf("49 / %d", int(cost))); err != nil {
		t.Fatal(err)
	}
	diff, set := 0, 0
	for i := range got {
		if got[i] != 0 {
			set++
		}
		if got[i] != want.V.Current[i] {
			diff++
		}
	}
	if set < 50 {
		t.Fatalf("only %d pixels drawn", set)
	}
	if diff != 0 {
		t.Errorf("%d pixels differ from the turns at 531 and \"49 / %d\" at 625", diff, cost)
	}
}
