package game

import (
	"path/filepath"
	"testing"

	"github.com/openorion2/openorion2/internal/engine"
	"github.com/openorion2/openorion2/internal/gfx"
	"github.com/openorion2/openorion2/internal/lbx"
	"github.com/openorion2/openorion2/internal/save"
)

func galaxyMapGame(t *testing.T) *Game {
	dir := filepath.Join("..", "..", "game")
	f := recipeFile(t, "mid")
	g := &Game{dir: dir, lbx: map[string]*lbx.Archive{}, cache: map[cacheKey]*gfx.Picture{}, Log: t.Logf,
		e: &engine.Engine{V: engine.NewVideo()}}
	if err := g.state.readBlocks(f, 9); err != nil {
		t.Fatal(err)
	}
	if err := g.loadGalaxyMapPictures(); err != nil {
		t.Skip(err)
	}
	g.galaxyMapDefaults()
	return g
}

// TestDrawGalaxyMapBox opens box 1 by hand (the game never does) and checks
// the black backdrop and that every star got a dot at its spot.
func TestDrawGalaxyMapBox(t *testing.T) {
	t.Parallel()
	g := galaxyMapGame(t)
	s, v := &g.state, g.e.V
	b := &s.MoveableBoxes[1]
	b.X, b.Y, b.Width, b.Height, b.DragDX, b.DragDY = 100, 50, 0xb0, 0xa5, 0, 0
	b.Content = 1
	for i := range v.Current {
		v.Current[i] = 0xff
	}
	if err := g.drawMoveableBox(1); err != nil {
		t.Fatal(err)
	}
	px := func(x, y int) byte { return v.Current[y*engine.W+x] }
	if px(100, 50) != 0 || px(100+0xb0, 50+0xa5) != 0 || px(99, 50) != 0xff || px(100, 50+0xa6) != 0xff {
		t.Errorf("backdrop not filled exactly over the box")
	}
	sx, sy := int32(1000*0x1fa)/0xa0, int32(1000*400)/0x75
	for i := 0; i < int(s.NumStars); i++ {
		st := &s.Stars[i]
		x := 8 + 100 + int(int32(st.X)*1000/int32(s.MaxMapScale)*10/sx)
		y := 50 + 0x26 + int(int32(st.Y)*1000/int32(s.MaxMapScale)*10/sy)
		lit := false
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				if c := px(x+dx, y+dy); c != 0 && c != 0xff {
					lit = true
				}
			}
		}
		if !lit {
			t.Errorf("star %d: no dot around (%d,%d)", i, x, y)
		}
	}
}

func TestScanGalaxyMapFields(t *testing.T) {
	t.Parallel()
	g := galaxyMapGame(t)
	g.clearGalaxyMapFields()
	gm := &g.ms.galaxyMap
	bh := -1
	for i := range gm.starField {
		gm.starField[i] = 100 + i
		if bh < 0 && g.state.Stars[i].SpectralClass == save.SpectralBlackHole {
			bh = i
		}
	}
	gm.gridField = 7
	if res, hov, clk := g.scanGalaxyMapFields(0, 103); res != 0 || clk != 3 || hov != -1 {
		t.Errorf("click: %d %d %d", res, hov, clk)
	}
	if res, hov, clk := g.scanGalaxyMapFields(104, 7); res != 4 || hov != 4 || clk != -1 {
		t.Errorf("hover: %d %d %d", res, hov, clk)
	}
	if res, _, _ := g.scanGalaxyMapFields(0, 7); res != 2 {
		t.Errorf("grid: %d", res)
	}
	if bh >= 0 {
		if res, hov, _ := g.scanGalaxyMapFields(100+bh, 0); res != -1 || hov != -1 {
			t.Errorf("black hole hover: %d %d", res, hov)
		}
	}
}
