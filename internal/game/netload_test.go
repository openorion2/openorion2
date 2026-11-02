package game

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/openorion2/openorion2/internal/host"
	"github.com/openorion2/openorion2/internal/netplay"
	"github.com/openorion2/openorion2/internal/save"
)

// Phase 6 of network play: loading a network save (docs/net/leaving.md).

// netLoadSteps is the lobby of a loaded game on machine m of n: the host
// loads the first slot, then machine k picks player pick (once the k
// before him have).
func netLoadSteps(m, n, pick int) []lobbyStep {
	empire := lobbyStep{"empire", func(g *Game) bool {
		mp := g.hs.mp
		if mp == nil || mp.nameField[pick] <= 0 || g.e.FieldsCount() <= mp.nameField[pick] {
			return false
		}
		taken := 0
		for _, c := range mp.chosen {
			taken += int(c)
		}
		return taken == m
	}, clickFieldOf(func(g *Game) int { return g.hs.mp.nameField[pick] })}
	if m == 0 {
		return []lobbyStep{
			{"multiplayer", func(g *Game) bool { return g.mpSetup != nil && g.currentScreen == ScreenMultiplayer }, pushKey('l')},
			{"game name", func(g *Game) bool { return g.currentScreen == ScreenLoadNet && g.rsel.box.inputField > 0 }, pushKey('\r')},
			{"slot", func(g *Game) bool { return g.netScr != nil && g.netScr.valid[0] != 0 && g.netScr.rows[0] > 0 },
				clickFieldOf(func(g *Game) int { return g.netScr.rows[0] })},
			empire,
			// With fewer machines than human empires: START once every
			// machine has picked, and Yes to "AI will take over".
			{"start", func(g *Game) bool {
				return g.net.u8(netState) == 4 || g.currentScreen == ScreenLoadNet && g.netScr != nil && g.netScr.startF > 0 &&
					g.net.i32(netPicking) == -1 && g.netGetUsers(nil) == int32(n) && g.nRemainingHumans() > n && g.hs.mp == nil
			}, func(g *Game, sys *host.System) {
				if g.net.u8(netState) != 4 {
					clickFieldOf(func(g *Game) int { return g.netScr.startF })(g, sys)
				}
			}},
			{"yes", func(g *Game) bool { return g.net.u8(netState) == 4 || g.ub.confirming }, func(g *Game, sys *host.System) {
				if g.ub.confirming {
					sys.PushKey(letterKey('y'))
				}
			}},
		}
	}
	return []lobbyStep{
		{"multiplayer", func(g *Game) bool { return g.mpSetup != nil }, pushKey('j')},
		{"game list", func(g *Game) bool { return g.netScr != nil && g.netScr.rows[0] > 0 && g.netGetGameCount() > 0 },
			clickFieldOf(func(g *Game) int { return g.netScr.rows[0] })},
		empire,
	}
}

// netLoadRunN loads sav (a network save) on n machines over the in-memory
// transport: machine 0 creates the loaded game, the others join; then after
// decides each input poll until it returns true.
func netLoadRunN(t *testing.T, n int, sav []byte, network netplay.Network, setup func(m int, g *Game), after func(m int, g *Game, sys *host.System, games []*Game) bool) []*Game {
	t.Helper()
	if network == nil {
		network = netplay.NewMem()
	}
	nets := make([]netplay.Network, n)
	for m := range nets {
		nets[m] = network
	}
	return netLoadRunWith(t, sav, nets, func(m int) []lobbyStep { return netLoadSteps(m, n, m) }, setup, after)
}

// netLoadRunWith is netLoadRunN with each machine's network and script.
func netLoadRunWith(t *testing.T, sav []byte, nets []netplay.Network, scripts func(m int) []lobbyStep, setup func(m int, g *Game), after func(m int, g *Game, sys *host.System, games []*Game) bool) []*Game {
	t.Helper()
	n := len(nets)
	clock := host.NewLockstep(n)
	games := make([]*Game, n)
	errs := make([]error, n)
	steps := make([]int, n)
	var wg sync.WaitGroup
	for m := range n {
		dir := scratchGameDir(t, filepath.Join("..", "..", "game"))
		if err := os.WriteFile(filepath.Join(dir, "SAVE1.GAM"), sav, 0o644); err != nil {
			t.Fatal(err)
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			script := scripts(m)
			waited := 0
			opt := Options{ListKey: true, Seed: uint32(5000 + m), Net: nets[m], NetSkipBoxes: true}
			errs[m] = host.Run(host.Options{Headless: true, Lockstep: clock, Machine: m, Frames: 60 * 900}, func(sys *host.System) error {
				opt.testHook = func(g *Game) {
					games[m] = g
					g.settings[settingsMPLink] = 1
					g.currentScreen = ScreenMultiplayer
					if setup != nil {
						setup(m, g)
					}
					g.e.InputHook = func() {
						if steps[m] < len(script) {
							st := script[steps[m]]
							if !st.when(g) {
								waited = 0
								return
							}
							if waited++; waited < 6 {
								return
							}
							waited = 0
							st.act(g, sys)
							steps[m]++
							return
						}
						if after(m, g, sys, games) {
							sys.Quit()
						}
					}
				}
				return Run(sys, dir, opt)
			})
			if errs[m] == nil && steps[m] < len(script) {
				errs[m] = fmt.Errorf("stopped before step %q", script[steps[m]].name)
			}
		}()
	}
	wg.Wait()
	for m, err := range errs {
		if err != nil && !errors.Is(err, host.ErrQuit) {
			t.Fatalf("machine %d: %v", m, err)
		}
	}
	return games
}

// TestNetLoadSoak: three humans and an AI play two turns; the autosave is
// then loaded with two humans (the third human's empire becomes an AI) and
// played on. After every broadcast each client equals the host but for
// what each viewer keeps.
func TestNetLoadSoak(t *testing.T) {
	t.Parallel()
	turns := 12
	if testing.Short() {
		turns = 4
	}
	nt := &netTurner{turns: 2, firstRow: true}
	first, _ := netGameRunN(t, 3, func(m int, g *Game, opt *Options) {
		if opt != nil {
			opt.ListKey = true
			opt.Seed = uint32(9000 + m)
			return
		}
		g.settings[0xd5] = 4 // players
		g.settings[2] = 0    // no random events
		g.settings[0xd3] = 0 // no Antarans
	}, func(m int, g *Game, sys *host.System, games []*Game) bool {
		return nt.after(m, g, sys, games)
	})
	sav, err := os.ReadFile(filepath.Join(first[0].dir, "SAVE10.GAM"))
	if err != nil {
		t.Fatal(err)
	}
	if f, err := save.Parse(sav); err != nil || f.Block(save.BGameType)[0] != save.GameNetwork {
		t.Fatalf("the autosave is not a network save (%v)", err)
	}
	var mu sync.Mutex
	hostSnaps := map[uint32][]byte{}
	clientSnaps := map[uint32][]byte{}
	nt2 := &netTurner{turns: turns, firstRow: true}
	games := netLoadRunN(t, 2, sav, nil, func(m int, g *Game) {
		g.Log = func(f string, a ...any) { t.Logf("machine %d: "+f, append([]any{m}, a...)...) }
		g.net.turnHook = func(host bool) {
			b := g.encodeSave(9)
			mu.Lock()
			defer mu.Unlock()
			if host {
				hostSnaps[g.state.Stardate] = b
			} else {
				clientSnaps[g.state.Stardate] = b
			}
		}
	}, func(m int, g *Game, sys *host.System, games []*Game) bool {
		return nt2.after(m, g, sys, games)
	})
	start := first[0].state.Stardate
	for m, g := range games {
		if g == nil || g.state.Stardate < start+uint32(turns) {
			t.Fatalf("machine %d stopped early", m)
		}
		s := &g.state
		if s.PlayerNum != int16(m) || s.Players[2].Objective == save.ObjectiveHuman || s.Players[2].NetID != -1 {
			t.Errorf("machine %d: player %d, the third human's objective %d, net id %d", m, s.PlayerNum, s.Players[2].Objective, s.Players[2].NetID)
		}
		if g.net.u8(netHumans) != 2 || g.gameType != 2 {
			t.Errorf("machine %d: %d humans, game type %d", m, g.net.u8(netHumans), g.gameType)
		}
	}
	checked := 0
	for sd, c := range clientSnaps {
		h, ok := hostSnaps[sd]
		if !ok {
			t.Errorf("no host state for stardate %d", sd)
			continue
		}
		a, _ := save.Parse(h)
		b, _ := save.Parse(c)
		diffs, err := save.Diff(a, b)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range diffs {
			if !netSoakLocal(d) {
				t.Errorf("stardate %d: %v", sd, d)
			}
		}
		checked++
	}
	t.Logf("%d client states checked, stardates %d..%d", checked, start, games[0].state.Stardate)
	if checked < turns {
		t.Errorf("%d client states checked, want %d", checked, turns)
	}
}
