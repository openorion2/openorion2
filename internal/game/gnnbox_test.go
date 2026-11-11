package game

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/openorion2/openorion2/internal/engine"
	"github.com/openorion2/openorion2/internal/host"
)

// TestGNNBoxKeepsAutoFunction: news shown as a box (settings 9 off) leaves
// the screen's auto function as it found it, so the next popup still
// draws the map behind it (port-only; the original put back a stale
// _jim_saved_autofunction and lost the stars and names).
func TestGNNBoxKeepsAutoFunction(t *testing.T) {
	t.Parallel()
	dir := scratchGameDir(t, filepath.Join("..", "..", "game"))
	putRecipeSave(t, "mid", dir, 1)
	ran, done := 0, false
	err := host.Run(host.Options{Headless: true, Frames: 60 * 120}, func(sys *host.System) error {
		opt := Options{Load: 1}
		opt.testHook = func(g *Game) {
			g.Log = func(string, ...any) {}
			idle, inside := 0, false
			g.e.InputHook = func() {
				idle++
				if inside {
					if idle%20 == 0 {
						sys.PushKey(letterKey('\r')) // the news box
					}
					return
				}
				if done {
					return
				}
				if !netMainScreenUp(g) {
					if idle%20 == 0 {
						sys.PushKey(letterKey('\r'))
					}
					return
				}
				s := &g.state
				g.settings[9] = 0
				ev := s.event(4)
				ev[1] = 4
				setEv16(ev, 3, 0)
				g.gnn.savedAuto = engine.AutoFunction{}
				g.e.AssignAutoFunction(func() error { ran++; return nil }, 1)
				inside = true
				if err := g.checkForEvent(); err != nil {
					t.Error(err)
				}
				inside, done = false, true
				before := ran
				if err := g.e.GetAutoFunction().Run(); err != nil {
					t.Error(err)
				}
				if ran == before {
					t.Error("after the news box the screen has no auto function")
				}
				sys.Quit()
			}
		}
		return Run(sys, dir, opt)
	})
	if err != nil && !errors.Is(err, host.ErrQuit) {
		t.Fatal(err)
	}
	if !done {
		t.Fatal("the news never ran")
	}
}
