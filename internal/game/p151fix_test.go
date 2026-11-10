package game

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/openorion2/openorion2/internal/host"
)

// TestRepeatAndroidNo: answering "No" to the androids-past-room question
// while placing a repeat mark queues nothing, not a bare repeat (1.40b23).
func TestRepeatAndroidNo(t *testing.T) {
	t.Parallel()
	dir := scratchGameDir(t, filepath.Join("..", "..", "game"))
	putRecipeSave(t, "mid", dir, 1)
	step := 0
	var ok bool
	var cur [7]int16
	var mode byte
	err := host.Run(host.Options{Headless: true, Frames: 60 * 60}, func(sys *host.System) error {
		opt := Options{Load: 1}
		opt.testHook = func(g *Game) {
			g.Log = func(string, ...any) {}
			idle := 0
			g.e.InputHook = func() {
				idle++
				switch {
				case step == 0 && !netMainScreenUp(g):
					if idle%20 == 0 {
						sys.PushKey(letterKey('\r'))
					}
				case step == 0:
					s, ci := &g.state, -1
					for i := 0; i < int(s.NumColonies); i++ {
						if c := &s.Colonies[i]; int(c.Owner) == int(s.PlayerNum) && c.IsOutpost == 0 {
							ci = i
							break
						}
					}
					c := &s.Colonies[ci]
					c.PopCount = uint8(g.planetMaxPopulationForPlayer(int(c.Planet), int(s.PlayerNum)))
					g.colonyHandle = ci
					g.bq.cur = [7]int16{-1, -1, -1, -1, -1, -1, -1}
					g.bq.active, g.bq.mode = -1, 2
					step, idle = 1, 0
					var err error
					ok, err = g.addToQueue(-6) // the box's input loop runs step 1
					if err != nil {
						t.Error(err)
					}
					cur, mode, step = g.bq.cur, g.bq.mode, 2
					sys.Quit()
				case step == 1 && idle == 10:
					sys.PushKey(letterKey('n'))
				}
			}
		}
		return Run(sys, dir, opt)
	})
	if err != nil && !errors.Is(err, host.ErrQuit) {
		t.Fatal(err)
	}
	if step != 2 {
		t.Fatalf("stopped at step %d", step)
	}
	if !ok || mode != 0 || cur != [7]int16{-1, -1, -1, -1, -1, -1, -1} {
		t.Errorf("after No: ok %v, mode %d, queue %v", ok, mode, cur)
	}
}

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
