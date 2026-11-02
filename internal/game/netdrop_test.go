package game

import (
	"errors"
	"net"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/openorion2/openorion2/internal/host"
	"github.com/openorion2/openorion2/internal/netplay"
	"github.com/openorion2/openorion2/internal/save"
)

// Port-only: lost players and the desync check (phase 7,
// docs/net/hardening.md). The game-long drops are TestNetSoak's drop modes.

// netStations attaches the games to one game of a transport: mem, or tcp
// on localhost; drop cuts station i's link (the creator's: the relay).
func netStations(t *testing.T, kind string, gs []*Game) (drop func(i int)) {
	t.Helper()
	eps := make([]netplay.Endpoint, len(gs))
	if kind == "mem" {
		mem := netplay.NewMem()
		for i := range gs {
			var err error
			if i == 0 {
				eps[i], err = mem.Create("G", "a")
			} else {
				eps[i], err = mem.Join("G", "b")
			}
			if err != nil {
				t.Fatal(err)
			}
		}
		drop = func(i int) { mem.Drop("G", eps[i].ID()) }
	} else {
		r, err := netplay.Listen("127.0.0.1:0", "G")
		if err != nil {
			t.Skip(err)
		}
		t.Cleanup(func() { r.Close() })
		eps[0] = r.Local()
		for i := 1; i < len(gs); i++ {
			if eps[i], err = netplay.Dial(r.Addr().String(), "G", "b"); err != nil {
				t.Fatal(err)
			}
		}
		for _, ep := range eps {
			for end := time.Now().Add(5 * time.Second); len(ep.Users()) < len(gs) && time.Now().Before(end); time.Sleep(time.Millisecond) {
			}
		}
		drop = func(i int) {
			if i == 0 {
				r.Close()
			} else {
				eps[i].Close()
			}
		}
	}
	for m, g := range gs {
		attach(g, eps[m], nil)
		g.net.inGame = true
		for k := range g.state.Players {
			g.state.Players[k].NetID = -1
		}
		for k := range gs {
			g.state.Players[k].NetID = int16(k + 1)
			g.state.Players[k].Objective = save.ObjectiveHuman
		}
		g.state.PlayerNum = int16(m)
		g.net.set8(netHumans, uint8(len(gs)))
	}
	return drop
}

// pumpUntil runs Mox_Net_Update_ on each game until cond holds (TCP
// delivers on other goroutines); an ExitError is returned.
func pumpUntil(gs []*Game, cond func() bool) error {
	for end := time.Now().Add(5 * time.Second); time.Now().Before(end); time.Sleep(time.Millisecond) {
		for _, g := range gs {
			if err := netRun(g, g.moxNetUpdate); err != nil {
				return err
			}
		}
		if cond() {
			return nil
		}
	}
	return errors.New("timed out")
}

// TestNetDropOut: of three stations the third is lost; both others have him as
// an AI with the same record. In the lobby both end, naming him.
func TestNetDropOut(t *testing.T) {
	t.Parallel()
	s1 := recipeSaveFile(t, "net3", 1)
	for _, kind := range []string{"mem", "tcp"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			withGames(t, []string{s1, s1, s1}, func(gs []*Game) error {
				drop := netStations(t, kind, gs)
				var mu sync.Mutex
				applied := []int{}
				for m, g := range gs[:2] {
					g.net.dropHook = func(p int) {
						mu.Lock()
						applied = append(applied, m*10+p)
						mu.Unlock()
					}
				}
				drop(2)
				ai := func(g *Game) bool {
					pl := &g.state.Players[2]
					return pl.NetID == -1 && pl.Objective != save.ObjectiveHuman
				}
				if err := pumpUntil(gs[:2], func() bool { return ai(gs[0]) && ai(gs[1]) }); err != nil {
					t.Fatal(err)
				}
				if a, b := recBytes(&gs[0].state.Players[2]), recBytes(&gs[1].state.Players[2]); !slices.Equal(a, b) {
					t.Errorf("the lost player's record differs: byte %#x", firstDiff(a, b))
				}
				slices.Sort(applied)
				if !slices.Equal(applied, []int{2, 12}) {
					t.Errorf("drop-outs applied (machine*10+player): %v", applied)
				}
				for m, g := range gs[:2] {
					if g.net.u8(netHumans) != 2 || len(g.net.notices) != 1 || !strings.Contains(g.net.notices[0], g.netWho(2)) {
						t.Errorf("machine %d: humans %d, notices %q", m, g.net.u8(netHumans), g.net.notices)
					}
				}
				return nil
			})
		})
	}
	t.Run("lobby", func(t *testing.T) {
		t.Parallel()
		withGames(t, []string{s1, s1, s1}, func(gs []*Game) error {
			drop := netStations(t, "mem", gs)
			for _, g := range gs {
				g.net.inGame = false
			}
			drop(2)
			for m, g := range gs[:2] {
				err := pumpUntil([]*Game{g}, func() bool { return false })
				var ee *ExitError
				if !errors.As(err, &ee) || !ee.Notice || !strings.Contains(ee.Msg, "left before the game started") {
					t.Errorf("machine %d: %v", m, err)
				}
			}
			return nil
		})
	})
}

// TestNetDesyncCheck: equal states hash the same on every station; a
// difference fires the check on both sides, which keep their saves.
func TestNetDesyncCheck(t *testing.T) {
	t.Parallel()
	s1 := recipeSaveFile(t, "net3", 1)
	dir := t.TempDir()
	withGames(t, []string{s1, s1}, func(gs []*Game) error {
		netStations(t, "mem", gs)
		var mu sync.Mutex
		seen := map[int][]bool{}
		for m, g := range gs {
			g.net.check, g.net.checkDir = true, dir
			g.net.checkHook = func(sd uint32, from uint16, same bool) {
				mu.Lock()
				seen[m] = append(seen[m], same)
				mu.Unlock()
			}
		}
		// The per-viewer fields do not count.
		gs[1].state.Stars[0].Owner ^= 1
		gs[1].settings[0xda] ^= 1
		gs[1].settings[0x11] ^= 1            // each machine's own MOX.SET: sound,
		gs[1].settings[0x1d] ^= 1            // the network game's name
		gs[1].state.MoveableBoxOrder[0] ^= 1 // the windows on the map and the map's view
		gs[1].state.CurMapX ^= 1
		for _, g := range gs {
			if err := netRun(g, g.netCheckState); err != nil {
				return err
			}
		}
		if err := pumpUntil(gs, func() bool { return len(seen[0]) == 1 && len(seen[1]) == 1 }); err != nil {
			t.Fatal(err)
		}
		if !seen[0][0] || !seen[1][0] {
			t.Fatalf("equal states differ: %v", seen)
		}
		for _, g := range gs {
			g.state.Stardate++
		}
		gs[1].state.Players[0].BC++
		for _, g := range gs {
			if err := netRun(g, g.netCheckState); err != nil {
				return err
			}
		}
		if err := pumpUntil(gs, func() bool { return len(seen[0]) == 2 && len(seen[1]) == 2 }); err != nil {
			t.Fatal(err)
		}
		if seen[0][1] || seen[1][1] {
			t.Fatalf("the difference went unseen: %v", seen)
		}
		files, _ := filepath.Glob(filepath.Join(dir, "netcheck-*-id*.GAM"))
		if len(files) != 2 {
			t.Fatalf("saves kept: %v", files)
		}
		return nil
	})
}

// netDropRun plays n machines over the in-memory transport (as TestNetSoak);
// drop says when machine m's link is cut (the creator's: the relay dies).
func netDropRun(t *testing.T, n int, setup func(m int, g *Game), drop func(m int, g *Game) bool) ([]*Game, []error) {
	t.Helper()
	nt := &netTurner{turns: 6, firstRow: true}
	mem := netplay.NewMem()
	gs, _, errs := netGameRunNE(t, n, func(m int, g *Game, opt *Options) {
		if opt != nil {
			opt.ListKey = true
			opt.Seed = uint32(7000 + m)
			opt.Net = mem
			return
		}
		g.settings[0xd5] = byte(n)
		g.settings[2] = 0
		g.settings[0xd3] = 0
		g.net.resume.off = true // the drop-outs at once (TestNetResume: the return)
		if setup != nil {
			setup(m, g)
		}
	}, func(m int, g *Game, sys *host.System, games []*Game) bool {
		if drop(m, g) {
			mem.Drop(g.net.ep.Game().Name, g.net.ep.ID())
			return true
		}
		return nt.after(m, g, sys, games)
	})
	return gs, errs
}

// TestNetDropCreator: the creator's machine (the relay) is lost on turn 3:
// every other machine ends with a message naming him.
func TestNetDropCreator(t *testing.T) {
	t.Parallel()
	gs, errs := netDropRun(t, 3, nil, func(m int, g *Game) bool {
		return m == 0 && g.state.Stardate == netSoakLeaveAt && g.currentScreen == ScreenMain && g.net.chat == nil
	})
	var own *ExitError
	if !errors.Is(errs[0], host.ErrQuit) && !errors.As(errs[0], &own) {
		t.Errorf("machine 0: %v", errs[0])
	}
	for m := 1; m < 3; m++ {
		var ee *ExitError
		if !errors.As(errs[m], &ee) || !ee.Notice || !strings.Contains(ee.Msg, gs[m].netWho(0)+", who relays the game, is lost") {
			t.Errorf("machine %d: %v", m, errs[m])
		}
	}
}

// TestNetDropLockstep: one side of a lockstep battle is lost: the other cannot
// finish it and ends with a message naming him.
func TestNetDropLockstep(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	barriers := 0
	gs, errs := netDropRun(t, 2, func(m int, g *Game) {
		g.settings[0xd5] = 2
		g.settings[0xd8] = 0
		g.net.preTurnHook = func() { netSoakBattles(g, netBattlesHumans) }
		g.combatUI = netSoakUI{g, false}
		if m == 1 {
			g.net.tacHook = func() {
				mu.Lock()
				barriers++
				mu.Unlock()
			}
		}
	}, func(m int, g *Game) bool {
		mu.Lock()
		defer mu.Unlock()
		return m == 1 && barriers >= 4
	})
	if barriers < 4 {
		t.Fatalf("no lockstep battle (%d barriers)", barriers)
	}
	var ee *ExitError
	if !errors.As(errs[0], &ee) || !ee.Notice || !strings.Contains(ee.Msg, gs[0].netWho(1)+" has lost the connection in a battle") {
		t.Errorf("machine 0: %v", errs[0])
	}
}

// TestNetDropTCP: of three machines over TCP the last joiner goes on turn 3;
// the other two play on with him as an AI.
func TestNetDropTCP(t *testing.T) {
	t.Parallel()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skip(err)
	}
	addr := l.Addr().String()
	l.Close()
	nt := &netTurner{turns: 5, firstRow: true}
	gone := false
	games, _, errs := netGameRunNE(t, 3, func(m int, g *Game, opt *Options) {
		if opt != nil {
			opt.Net = &netplay.TCP{Listen: addr, Addr: addr}
			opt.ListKey = true
			opt.Seed = uint32(7000 + m)
			return
		}
		g.settings[0xd5] = 3
		g.settings[2] = 0
		g.settings[0xd3] = 0
	}, func(m int, g *Game, sys *host.System, games []*Game) bool {
		if m == 2 && g.state.Stardate == netSoakLeaveAt && g.currentScreen == ScreenMain && g.net.chat == nil {
			gone = true
			g.net.ep.Close()
			return true
		}
		return nt.after(m, g, sys, games)
	})
	if !gone {
		t.Fatal("machine 2 never left")
	}
	// The machine that went may end on its own closed link; the others not.
	for m, err := range errs[:2] {
		if err != nil && !errors.Is(err, host.ErrQuit) {
			t.Fatalf("machine %d: %v", m, err)
		}
	}
	for m, g := range games[:2] {
		pl := &g.state.Players[2]
		if g.state.Stardate != 35005 || pl.NetID != -1 || pl.Objective == save.ObjectiveHuman || g.gameType != 2 {
			t.Errorf("machine %d: stardate %d, player 2 net id %d objective %d, game type %d",
				m, g.state.Stardate, pl.NetID, pl.Objective, g.gameType)
		}
		g.netUninit()
	}
}
