package game

import (
	"errors"
	"path/filepath"
	"slices"
	"testing"

	"github.com/openorion2/openorion2/internal/host"
)

// TestLeadersMapStars: on the leaders screen's colony page the pointer on
// any star of the map picks that star's field (port-only: the original's
// system display whole-box field, x and y swapped, covers the map's lower
// left corner; placed "right" it covered its top).
func TestLeadersMapStars(t *testing.T) {
	t.Parallel()
	dir := scratchGameDir(t, filepath.Join("..", "..", "game"))
	putRecipeSave(t, "mid", dir, 1)
	checked, step := 0, 0
	err := host.Run(host.Options{Headless: true, Frames: 60 * 60}, func(sys *host.System) error {
		opt := Options{Load: 1}
		opt.testHook = func(g *Game) {
			g.Log = func(string, ...any) {}
			idle := 0
			g.e.InputHook = func() {
				switch {
				case step == 0 && !netMainScreenUp(g) && g.currentScreen != ScreenLeaders:
					// The turn's reports (a leader's new level): Enter.
					if idle++; idle%20 == 0 {
						sys.PushKey(letterKey('\r'))
					}
				case step == 0 && g.currentScreen != ScreenLeaders:
					// LEADERS, again until it shows (the first presses may
					// meet the turn's last reports).
					if idle++; idle%30 == 0 {
						sys.PushKey(letterKey('l'))
					}
				case step == 0 && g.ms.galaxyMap.starFieldOf(0) > 0:
					s, gm := &g.state, &g.ms.galaxyMap
					for i := 0; i < int(s.NumStars); i++ {
						f := gm.starFieldOf(i)
						x1, y1, x2, y2 := g.e.FieldRect(f)
						sys.WarpMouse((x1+x2)/2, (y1+y2)/2)
						got, err := g.e.ScanInput()
						if err != nil {
							t.Fatal(err)
						}
						// A ship stack's icon or a near star may lie over it, as
						// in the original; nothing else may.
						if got != f && !g.leadersMapField(got) {
							t.Errorf("star %d (%d,%d): the pointer picks field %d %v, not its own %d",
								i, (x1+x2)/2, (y1+y2)/2, got, fieldRect(g, got), f)
						}
						checked++
					}
					step++
					sys.Quit()
				}
			}
		}
		return Run(sys, dir, opt)
	})
	if err != nil && !errors.Is(err, host.ErrQuit) {
		t.Fatal(err)
	}
	if checked == 0 {
		t.Fatal("the leaders screen's map was never checked")
	}
}

func fieldRect(g *Game, f int) [4]int {
	x1, y1, x2, y2 := g.e.FieldRect(f)
	return [4]int{x1, y1, x2, y2}
}

// leadersMapField: f is one of the map's fields, a ship stack's icon or
// a star.
func (g *Game) leadersMapField(f int) bool {
	st := &g.stacks
	for i := 0; i < int(st.iconCount); i++ {
		if int(st.icon[i].field) == f {
			return true
		}
	}
	for i := 0; i < int(g.state.NumStars); i++ {
		if g.ms.galaxyMap.starFieldOf(i) == f {
			return true
		}
	}
	return false
}

// TestSystemBoxOtherStars: with a star's system box open on the main
// screen, a click on any other star beside the box opens that one's box
// (the box's whole-box field must not reach over the map around it; the
// disassembly's x, y swap did).
func TestSystemBoxOtherStars(t *testing.T) {
	t.Parallel()
	dir := scratchGameDir(t, filepath.Join("..", "..", "game"))
	putRecipeSave(t, "mid", dir, 1)
	switched, tried := 0, 0
	err := host.Run(host.Options{Headless: true, Frames: 60 * 600}, func(sys *host.System) error {
		opt := Options{Load: 1}
		opt.testHook = func(g *Game) {
			g.Log = func(string, ...any) {}
			idle, target, wait := 0, -1, 0
			var done []int
			g.e.InputHook = func() {
				s := &g.state
				if !netMainScreenUp(g) {
					if idle++; idle%20 == 0 {
						sys.PushKey(letterKey('\r')) // the turn's reports
					}
					return
				}
				if target >= 0 {
					if wait++; wait == 5 {
						// The pointer is there; now the click.
						x, y := g.starDrawCoords(target)
						sys.Press(int(x), int(y), 1)
					}
					if wait < 40 {
						return
					}
					switch {
					case g.moveableBoxSelected(0) && int(s.MoveableBoxes[0].Content) == target:
						switched++
					case g.moveableBoxSelected(2):
						// A ship stack's icon at the star took the click.
					default:
						x, y := g.starDrawCoords(target)
						t.Errorf("a click on star %d (%d,%d, class %d, fleet box %v) left the box on star %d", target, x, y,
							s.Stars[target].SpectralClass, g.moveableBoxSelected(2), s.MoveableBoxes[0].Content)
					}
					done = append(done, target)
					target = -1
				}
				if tried == 60 {
					sys.Quit()
					return
				}
				// The next star on screen, outside the open box.
				b := &s.MoveableBoxes[0]
				open := g.moveableBoxSelected(0)
				for i := 0; i < int(s.NumStars); i++ {
					x, y := g.starDrawCoords(i)
					inBox := open && x >= b.X-12 && x <= b.X+b.Width+12 && y >= b.Y-12 && y <= b.Y+b.Height+12
					if !g.starOnScreen(i) || inBox || slices.Contains(done, i) || open && int(b.Content) == i {
						continue
					}
					target, wait = i, 0
					tried++
					sys.WarpMouse(int(x), int(y))
					return
				}
				sys.Quit()
			}
		}
		return Run(sys, dir, opt)
	})
	if err != nil && !errors.Is(err, host.ErrQuit) {
		t.Fatal(err)
	}
	if switched < 3 {
		t.Fatalf("%d of %d clicks opened their star's box", switched, tried)
	}
}
