package game

import (
	"path/filepath"
	"testing"

	"github.com/openorion2/openorion2/internal/engine"
	"github.com/openorion2/openorion2/internal/font"
	"github.com/openorion2/openorion2/internal/gfx"
	"github.com/openorion2/openorion2/internal/lbx"
)

// starsGame loads recipe n's save into a Game with a video page and the font.
func starsGame(t *testing.T, n string) *Game {
	dir := filepath.Join("..", "..", "game")
	f := recipeFile(t, n)
	g := &Game{dir: dir, lbx: map[string]*lbx.Archive{}, cache: map[cacheKey]*gfx.Picture{}, Log: t.Logf,
		e: &engine.Engine{V: engine.NewVideo()}}
	if err := g.state.readBlocks(f, 9); err != nil {
		t.Fatal(err)
	}
	fonts, err := g.archive("FONTS.LBX")
	if err != nil {
		t.Skip(err)
	}
	fe, err := fonts.Entry(0)
	if err != nil {
		t.Fatal(err)
	}
	fnt, err := font.Parse(fe)
	if err != nil {
		t.Fatal(err)
	}
	g.e.SetFont(fnt)
	return g
}

// The late game's star 4 (Nazin) has one colony of player 0 and one of player 2:
// the name bitmap is split in two equal shares of the string width.
func TestStarNameSplitByOwners(t *testing.T) {
	t.Parallel()
	g := starsGame(t, "late")
	const star = 4
	name, bm, ok := g.starName(star, false)
	if !ok || bm == nil || name != "Nazin" {
		t.Fatalf("starName = %q, %v, %v; want Nazin with a bitmap", name, bm != nil, ok)
	}
	tx := g.e.Text()
	w := tx.Width([]byte(name))
	per := (w*10/2 + 9) / 10
	set := func(p int) map[byte]bool {
		m := map[byte]bool{}
		c8 := g.playerFontColors(p, tx.Style, false)
		for _, c := range c8 {
			m[c] = true
		}
		return m
	}
	left, right := set(0), set(2)
	pix := bm.Pixels()
	var nl, nr int
	for y := 0; y < bm.Height; y++ {
		for x := 0; x < bm.Width; x++ {
			c := pix[y*bm.Width+x]
			if c == 0 {
				continue
			}
			switch {
			case x < per && left[c]:
				nl++
			case x >= per && x < min(2*per, w) && right[c]:
				nr++
			default:
				t.Fatalf("pixel (%d,%d) = %#x outside its owner's colours (share %d of %d)", x, y, c, per, w)
			}
		}
	}
	if nl == 0 || nr == 0 {
		t.Fatalf("shares: %d left, %d right pixels", nl, nr)
	}

	// Highlighting tints the bitmap by 0, 1, 2, 3, 2, 1, 0, 1, ...
	want := []int8{1, 2, 3, 2, 1, 0, 1}
	for k, v := range want {
		if err := g.printHighlightedStarName(star); err != nil {
			t.Fatal(err)
		}
		if h := g.starNameHighlight(); h.value != v {
			t.Fatalf("step %d: highlight value %d, want %d", k, h.value, v)
		}
	}
	if err := g.printStarNames(); err != nil {
		t.Fatal(err)
	}
}

// Gate icons: portal left of the star (two pixels lower), net right of it.
func TestGateIcons(t *testing.T) {
	t.Parallel()
	g := starsGame(t, "late")
	s := &g.state
	p := int(s.PlayerNum)
	star := -1
	for i := 0; i < int(s.NumStars); i++ {
		if s.playerHasVisited(p, i) && g.starOnScreen(i) {
			star = i
			break
		}
	}
	if star < 0 {
		t.Fatal("no visited star on screen")
	}
	st := &s.Stars[star]
	st.DimensionalPortal |= 1 << p
	st.ArtemisNet |= 1 << p
	if err := g.drawAGateIcon(star); err != nil {
		t.Fatal(err)
	}
	cx, cy := g.starDrawCoords(star)
	var l, r int
	for y := 0; y < engine.H; y++ {
		for x := 0; x < engine.W; x++ {
			if g.e.V.Current[y*engine.W+x] == 0 {
				continue
			}
			switch {
			case x < int(cx) && x >= int(cx)-30 && abs(y-int(cy)) < 30:
				l++
			case x >= int(cx)+4 && x < int(cx)+30 && abs(y-int(cy)) < 30:
				r++
			default:
				t.Fatalf("pixel at (%d,%d), star at (%d,%d)", x, y, cx, cy)
			}
		}
	}
	if l == 0 || r == 0 {
		t.Fatalf("icons: %d left, %d right pixels", l, r)
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
