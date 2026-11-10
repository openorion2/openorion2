package game

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/openorion2/openorion2/internal/host"
	"github.com/openorion2/openorion2/internal/save"
)

// TestPopupFrameAfterShipViews: two ship views from the fleet screen leave
// the frame counter at -2; the system popup still draws its planets.
func TestPopupFrameAfterShipViews(t *testing.T) {
	t.Parallel()
	dir := scratchGameDir(t, filepath.Join("..", "..", "game"))
	putRecipeSave(t, "mid", dir, 1)
	step := 0
	var drawErr error
	var frame int16
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
					s := &g.state
					for i := 0; i < int(s.NumColonies); i++ {
						if c := &s.Colonies[i]; int(c.Owner) == int(s.PlayerNum) {
							s.MoveableBoxes[0].Content = int16(s.Planets[c.Planet].Star)
							break
						}
					}
					// Loaded first: a newly loaded system skips its planets once.
					if drawErr = g.initSystemDisplay(int(s.MoveableBoxes[0].Content)); drawErr == nil {
						g.sd.shipFrame, g.sd.skipPlanets = -2, false
						drawErr = g.drawSystemDisplayBox()
					}
					frame, step = g.sd.shipFrame, 1
					sys.Quit()
				}
			}
		}
		return Run(sys, dir, opt)
	})
	if err != nil && !errors.Is(err, host.ErrQuit) {
		t.Fatal(err)
	}
	if step != 1 {
		t.Fatalf("stopped at step %d", step)
	}
	if drawErr != nil || frame != 1535 {
		t.Errorf("frame %d, %v", frame, drawErr)
	}
}

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

// TestDesignCostOwnersGovernment: an AI's designs and refits cost by its
// own government, not the human's ("feudal bug").
func TestDesignCostOwnersGovernment(t *testing.T) {
	t.Parallel()
	g := techGame(t, "mid")
	s := &g.state
	me := int(s.PlayerNum)
	ci := -1
	for i := 0; i < int(s.NumColonies); i++ {
		if c := &s.Colonies[i]; int(c.Owner) != me && c.Owner >= 0 && c.IsOutpost == 0 {
			ci = i
			break
		}
	}
	if ci == -1 {
		t.Fatal("no AI colony")
	}
	p := int(s.Colonies[ci].Owner)
	s.Players[me].Picks[save.PickGovernment] = 2
	cost := func(gov int8) (int16, int16) {
		s.Players[p].Picks[save.PickGovernment] = gov
		g.updateAutomaticDesignEquipment(p)
		k := len(s.Ships) - 1
		s.Ships[k] = save.Ship{Owner: int8(p), Status: 4, Design: s.Players[p].Designs[0]}
		s.Colonies[ci].Queue[0] = int16(-(k + 100))
		g.completeShip(ci)
		return s.Players[p].Designs[0].Cost, s.Ships[k].Design.Cost
	}
	fd, fr := cost(0)
	dd, dr := cost(2)
	if int(s.PlayerNum) != me {
		t.Fatalf("PLAYER_NUM is now %d", s.PlayerNum)
	}
	if want := int16((int32(dd)*2 + 2) / 3); fd != want || fr != want || dr != dd {
		t.Errorf("feudal design %d refit %d, dictatorship %d refit %d; want feudal %d", fd, fr, dd, dr, want)
	}
}
