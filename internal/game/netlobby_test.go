package game

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/openorion2/openorion2/internal/host"
	"github.com/openorion2/openorion2/internal/netplay"
	"github.com/openorion2/openorion2/internal/save"
)

// Phase 2 of network play: a new network game from the multiplayer
// screen on two machines (docs/net/lobby.md).

// lobbyStep is one scripted input of a lobby machine: when holds, act runs
// (after a few more input polls, so the screen has settled).
type lobbyStep struct {
	name string
	when func(g *Game) bool
	act  func(g *Game, sys *host.System)
}

func letterKey(c byte) host.Key {
	switch {
	case c >= 'a' && c <= 'z':
		return host.Key{Code: uint32(c), Scancode: int(c-'a') + 4}
	case c >= 'A' && c <= 'Z':
		return host.Key{Code: uint32(c), Scancode: int(c-'A') + 4, Shift: true}
	case c >= '1' && c <= '9':
		return host.Key{Code: uint32(c), Scancode: int(c-'1') + 30}
	case c == '0':
		return host.Key{Code: '0', Scancode: 39}
	case c == ' ':
		return host.Key{Code: ' ', Scancode: 44}
	case c == '\r':
		return host.Key{Code: '\r', Scancode: 40}
	case c == '.':
		return host.Key{Code: '.', Scancode: 55}
	case c == '-':
		return host.Key{Code: '-', Scancode: 45}
	case c == ':':
		return host.Key{Code: ':', Scancode: 51, Shift: true}
	}
	panic(fmt.Sprintf("no key %q", c))
}

func pushKey(c byte) func(*Game, *host.System) {
	return func(_ *Game, sys *host.System) { sys.PushKey(letterKey(c)) }
}

func clickFieldOf(f func(g *Game) int) func(*Game, *host.System) {
	return func(g *Game, sys *host.System) {
		x1, y1, x2, y2 := g.e.FieldRect(f(g))
		sys.Press((x1+x2)/2, (y1+y2)/2, 1)
	}
}

// netLobbyRun plays a new two-player network game from the multiplayer
// screen on two machines (machine 0 creates, 1 joins) on one Lockstep
// clock over the in-memory transport, until both are past the first
// broadcast; it returns both games and every message each logged.
func netLobbyRun(t *testing.T, setup func(m int, g *Game, opt *Options)) ([2]*Game, [2][]NetRecord) {
	t.Helper()
	return netGameRun(t, setup, nil)
}

// netGameRun is netLobbyRun; with after, each machine goes on past the
// first broadcast, after deciding each input poll until it returns true.
func netGameRun(t *testing.T, setup func(m int, g *Game, opt *Options), after func(m int, g *Game, sys *host.System, games []*Game) bool) ([2]*Game, [2][]NetRecord) {
	t.Helper()
	gs, ls := netGameRunN(t, 2, setup, after)
	return [2]*Game{gs[0], gs[1]}, [2][]NetRecord{ls[0], ls[1]}
}

// netGameRunN is netGameRun with n machines: joiner k picks race k (keys
// a, b, c ...) once joiner k-1 is through its picks.
func netGameRunN(t *testing.T, n int, setup func(m int, g *Game, opt *Options), after func(m int, g *Game, sys *host.System, games []*Game) bool) ([]*Game, [][]NetRecord) {
	t.Helper()
	games, logs, errs := netGameRunNE(t, n, setup, after)
	for i, err := range errs {
		if err != nil && !errors.Is(err, host.ErrQuit) {
			t.Fatalf("machine %d: %v", i, err)
		}
	}
	return games, logs
}

// netGameRunNE is netGameRunN returning each machine's error.
func netGameRunNE(t *testing.T, n int, setup func(m int, g *Game, opt *Options), after func(m int, g *Game, sys *host.System, games []*Game) bool) ([]*Game, [][]NetRecord, []error) {
	t.Helper()
	steps := make([]int, n)
	scripts := make([][]lobbyStep, n)
	inRace := lobbyInRace
	scripts[0] = []lobbyStep{
		{"multiplayer", func(g *Game) bool { return g.mpSetup != nil }, pushKey('s')},
		{"new game", func(g *Game) bool { return g.currentScreen == ScreenNewGame && g.ng.acceptField > 0 },
			clickFieldOf(func(g *Game) int { return g.ng.acceptField })},
		// START, unless the waiting screen ended by itself (settings+0xd5
		// humans in).
		{"start", func(g *Game) bool {
			return g.netScr != nil && g.netScr.startF > 0 && g.netGetUsers(nil) == int32(n) && g.net.u8(netState) == 1 || inRace(g)
		}, func(g *Game, sys *host.System) {
			if !inRace(g) {
				sys.PushKey(letterKey('s'))
			}
		}},
		{"race", inRace, pushKey('a')},
		{"name", func(g *Game) bool { return g.rsel.player != nil && g.e.FieldsCount() <= 3 }, pushKey('\r')},
		{"banner", func(g *Game) bool { return g.e.FieldsCount() >= 4 && g.e.FieldsCount() <= 10 }, pushKey('1')},
	}
	for k := 1; k < n; k++ {
		prev := func() bool { return k == 1 || steps[k-1] == len(scripts[k-1]) }
		scripts[k] = []lobbyStep{
			{"multiplayer", func(g *Game) bool { return g.mpSetup != nil }, pushKey('j')},
			{"game list", func(g *Game) bool { return g.netScr != nil && g.netScr.rows[0] > 0 },
				clickFieldOf(func(g *Game) int { return g.netScr.rows[0] })},
			// The pointer over a race another machine took: its description
			// gives way to Ken text 0x48, which once failed the port.
			{"hover taken", func(g *Game) bool { return inRace(g) && prev() && takenRace(g) >= 0 }, func(g *Game, sys *host.System) {
				i := takenRace(g)
				sys.WarpMouse(i/7*0x7e+0x15f+0x20, i%7*0x30+0x5a+0x10)
			}},
			{"race", func(g *Game) bool { return inRace(g) && prev() }, pushKey(byte('a' + k))},
			{"name", func(g *Game) bool { return g.rsel.player != nil && g.e.FieldsCount() <= 3 }, pushKey('\r')},
			{"banner", func(g *Game) bool { return g.e.FieldsCount() >= 4 && g.e.FieldsCount() <= 10 }, pushKey('1')},
		}
	}
	return netRunScripts(t, scripts, steps, setup, after, nil)
}

// lobbyInRace: the machine shows the race selection screen of the lobby.
func lobbyInRace(g *Game) bool {
	return g.rsel.player != nil && g.e.FieldsCount() > 10 && g.net.u8(netState) != 4
}

// netRunScripts runs one machine per script on one Lockstep clock over the
// in-memory transport (wrap may wrap machine m's view of it); steps counts
// each machine's steps done.
func netRunScripts(t *testing.T, scripts [][]lobbyStep, steps []int, setup func(m int, g *Game, opt *Options), after func(m int, g *Game, sys *host.System, games []*Game) bool, wrap func(m int, nw netplay.Network) netplay.Network) ([]*Game, [][]NetRecord, []error) {
	t.Helper()
	n := len(scripts)
	clock := host.NewLockstep(n)
	var mem netplay.Network = netplay.NewMem()
	games := make([]*Game, n)
	logs := make([][]NetRecord, n)
	var mu sync.Mutex
	started := func(g *Game) bool { return g.net.u8(netState) == 4 }
	dirs := make([]string, n)
	for i := range dirs {
		dirs[i] = scratchGameDir(t, filepath.Join("..", "..", "game"))
		removeIfExists(filepath.Join(dirs[i], "LASTRACE.RAC"))
		removeIfExists(filepath.Join(dirs[i], "lastrace.rac"))
	}
	errs := make([]error, n)
	frames := 60 * 200
	if after != nil {
		frames = 60 * 600
	}
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			waited := 0
			nw := mem
			if wrap != nil {
				nw = wrap(i, mem)
			}
			opt := Options{NewGameKeys: true, Seed: uint32(1000 + i), Net: nw, NetSkipBoxes: true,
				NetLog: func(r NetRecord) {
					mu.Lock()
					logs[i] = append(logs[i], r)
					mu.Unlock()
				}}
			if setup != nil {
				setup(i, nil, &opt)
			}
			errs[i] = host.Run(host.Options{Headless: true, Lockstep: clock, Machine: i, Frames: frames}, func(sys *host.System) error {
				opt.testHook = func(g *Game) {
					games[i] = g
					g.Log = func(f string, a ...any) { t.Logf("machine %d: "+f, append([]any{i}, a...)...) }
					g.settings[settingsMPLink] = 1
					g.settings[0xd5] = 2 // players
					g.settings[0xd6] = 0 // small galaxy
					g.settings[0xd4] = 0 // difficulty
					g.currentScreen = ScreenMultiplayer
					if setup != nil {
						setup(i, g, nil)
					}
					g.e.InputHook = func() {
						step := steps[i]
						if started(g) && after == nil || step == len(scripts[i]) && after != nil && after(i, g, sys, games) {
							sys.Quit()
							return
						}
						if step == len(scripts[i]) || !scripts[i][step].when(g) {
							waited = 0
							return
						}
						if waited++; waited < 6 {
							return
						}
						waited = 0
						if d := os.Getenv("OPENORION2_NET_SHOTS"); d != "" {
							// What each machine shows before each scripted input.
							g.e.SaveScreenPNG(filepath.Join(d, fmt.Sprintf("m%d-%d-%s.png", i, step, scripts[i][step].name)))
						}
						scripts[i][step].act(g, sys)
						steps[i]++
					}
				}
				return Run(sys, dirs[i], opt)
			})
			if errs[i] == nil && steps[i] < len(scripts[i]) {
				errs[i] = fmt.Errorf("stopped before step %q", scripts[i][steps[i]].name)
			}
		}()
	}
	wg.Wait()
	return games, logs, errs
}

func removeIfExists(p string) { _ = os.Remove(p) }

// takenRace is the first race button another machine holds, or -1.
func takenRace(g *Game) int {
	for i := 0; i <= 0xc; i++ {
		if g.rsel.raceFlag[i] {
			return i
		}
	}
	return -1
}

// TestNetNewGame: two machines start a network game. Afterwards both hold
// the same state but for PLAYER_NUM and each viewer's star owners, and the
// messages follow the original's order (build/oracle/net2t).
func TestNetNewGame(t *testing.T) {
	t.Parallel()
	games, logs := netLobbyRun(t, nil)
	for m, g := range games {
		if g == nil || g.net.u8(netState) != 4 || g.gameType != 2 {
			t.Fatalf("machine %d did not start the game", m)
		}
	}
	a, err := save.Parse(games[0].encodeSave(9))
	if err != nil {
		t.Fatal(err)
	}
	b, err := save.Parse(games[1].encodeSave(9))
	if err != nil {
		t.Fatal(err)
	}
	diffs, err := save.Diff(a, b)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range diffs {
		t.Logf("%v", d)
	}
	checkLobbyDiffs(t, diffs)
	for m := range logs {
		t.Logf("machine %d sends: %v", m, lobbySends(logs[m]))
	}
}

// checkLobbyDiffs: the two machines may differ only in PLAYER_NUM, the
// local settings bytes 0xda.. and the star owners each viewer sees.
func checkLobbyDiffs(t *testing.T, diffs []save.FieldDiff) {
	t.Helper()
	owners := 0
	for _, d := range diffs {
		s := d.String()
		switch {
		case strings.HasPrefix(s, "PLAYER_NUM"), strings.HasPrefix(s, "settings.bytes 0xda"):
		case strings.HasPrefix(s, "star[") && strings.Contains(s, "].Owner:"):
			owners++
		default:
			t.Errorf("the machines differ: %s", s)
		}
	}
	if owners != 2 {
		t.Errorf("%d star owners differ, want the two home stars", owners)
	}
}

func lobbySends(log []NetRecord) (out []string) {
	for _, r := range log {
		if r.Dir == 'S' {
			out = append(out, fmt.Sprintf("%#x>%#x/%d", r.ID, r.Peer, r.Len))
		}
	}
	return
}

// TestNetNewGameTCP is TestNetNewGame over real TCP on localhost (the
// frames arrive when they arrive, so only the end state is checked).
func TestNetNewGameTCP(t *testing.T) {
	t.Parallel()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skip(err)
	}
	addr := l.Addr().String()
	l.Close()
	games, _ := netLobbyRun(t, func(m int, g *Game, opt *Options) {
		if opt != nil {
			opt.Net = &netplay.TCP{Listen: addr, Addr: addr}
		}
	})
	a, _ := save.Parse(games[0].encodeSave(9))
	b, _ := save.Parse(games[1].encodeSave(9))
	diffs, err := save.Diff(a, b)
	if err != nil {
		t.Fatal(err)
	}
	checkLobbyDiffs(t, diffs)
	for _, g := range games {
		g.netUninit()
	}
}

// TestNetReleaseMismatch: two releases of the port do not play together:
// the joiner of another release ends with the original's message, both
// versions in it (port-only: the release follows the build date).
func TestNetReleaseMismatch(t *testing.T) {
	t.Parallel()
	_, _, errs := netGameRunNE(t, 2, func(m int, g *Game, opt *Options) {
		if g != nil && m == 1 {
			g.release = "v9.9.9"
		}
	}, nil)
	var ee *ExitError
	if !errors.As(errs[1], &ee) || !strings.Contains(ee.Msg, "Incompatible program versions") || !strings.Contains(ee.Msg, "v9.9.9") {
		t.Fatalf("the joiner: %v", errs[1])
	}
	t.Logf("the joiner: %s", ee.Msg)
}
