package game

import (
	"bytes"
	"errors"
	"path/filepath"
	"testing"

	"github.com/openorion2/openorion2/internal/engine"
	"github.com/openorion2/openorion2/internal/host"
	"github.com/openorion2/openorion2/internal/save"
)

// TestOfficerExperience: community patch 1.51's "XP/next level" in the
// crew experience text of KENTEXT.
func TestOfficerExperience(t *testing.T) {
	t.Parallel()
	g := sysInfoGame(t, "start")
	s := &g.state
	l := 0
	s.Leaders[l].Owner = int8(s.PlayerNum)
	s.Players[s.PlayerNum].Picks[save.PickWarlord] = 0
	for _, c := range []struct {
		xp      int16
		lang    uint8
		warlord int8
		want    string
	}{
		{0, 0, 0, "0/60 EP"}, {59, 0, 0, "59/60 EP"}, {60, 0, 0, "60/150 EP"},
		{299, 0, 0, "299/300 EP"}, {300, 0, 0, "300/500 EP"}, {500, 0, 0, "500/1000 EP"},
		{1200, 0, 0, "1200/1000 EP"}, {1200, 0, 1, "1200/1000 EP"},
		{120, 2, 0, "120/150 PE"}, {120, 4, 0, "Esp. 120/150"},
	} {
		s.Leaders[l].Experience = c.xp
		g.settings[0xd2] = c.lang
		s.Players[s.PlayerNum].Picks[save.PickWarlord] = c.warlord
		if got := g.officerExperience(l); got != c.want {
			t.Errorf("xp %d lang %d warlord %d: %q, want %q", c.xp, c.lang, c.warlord, got, c.want)
		}
	}
}

// TestLeadersScreenExperience opens the leaders screen and changes the
// first leader's experience: the text right of the row's foot must follow.
func TestLeadersScreenExperience(t *testing.T) {
	t.Parallel()
	dir := scratchGameDir(t, filepath.Join("..", "..", "game"))
	putRecipeSave(t, "mid", dir, 1)
	step := 0
	var before []byte
	region := func(v *engine.Video) []byte {
		var b []byte
		for y := 125; y < 145; y++ {
			b = append(b, v.Screen[y*engine.W+200:y*engine.W+295]...)
		}
		return b
	}
	err := host.Run(host.Options{Headless: true, Frames: 60 * 120}, func(sys *host.System) error {
		opt := Options{Load: 1}
		opt.testHook = func(g *Game) {
			g.Log = func(string, ...any) {}
			idle, wait := 0, 0
			g.e.InputHook = func() {
				idle++
				switch {
				case step == 0 && !netMainScreenUp(g):
					if idle%20 == 0 {
						sys.PushKey(letterKey('\r'))
					}
				case step == 0:
					step = 1
				case step == 1 && g.currentScreen != ScreenLeaders:
					if idle%30 == 0 {
						sys.PushKey(letterKey('l'))
					}
				case step == 1:
					step, wait = 2, 0
				case step == 2:
					if wait++; wait == 20 {
						before = region(g.e.V)
						l := g.flt.officerIDs[0]
						g.state.Leaders[l].Experience = 7
						step, wait = 3, 0
					}
				case step == 3:
					if wait++; wait == 20 {
						if bytes.Equal(before, region(g.e.V)) {
							t.Error("the experience shown did not change")
						}
						step = 4
						sys.Quit()
					}
				}
			}
		}
		return Run(sys, dir, opt)
	})
	if err != nil && !errors.Is(err, host.ErrQuit) {
		t.Fatal(err)
	}
	if step != 4 {
		t.Fatalf("stopped at step %d", step)
	}
}
