package game

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/openorion2/openorion2/internal/host"
	"github.com/openorion2/openorion2/internal/lobby"
	"github.com/openorion2/openorion2/internal/lobby/lobbytest"
	"github.com/openorion2/openorion2/internal/netplay"
)

// Port-only: a lost player's return (netresume.go, docs/net/hardening.md).

// netResumeCase is one game of TestNetResume: humans machines start it; on
// its second turn machine lost's link is cut, on the main screen or (turn)
// as the host of that turn, while the others wait for it. With back, one
// machine more joins the game again and takes his empire; else the first
// machine left presses START and the computer plays it.
type netResumeCase struct {
	humans, lost      int
	turn, back, lobby bool // lobby: through lobbytest, the real server's rooms
}

func (c netResumeCase) String() string {
	s := fmt.Sprintf("%dh-lost%d", c.humans, c.lost)
	if c.lobby {
		s = "lobby-" + s
	}
	if c.turn {
		s += "-turnhost"
	}
	if c.back {
		return s + "-back"
	}
	return s + "-goon"
}

// netResumeRun plays c; it returns every machine's game and the lost player.
func netResumeRun(t *testing.T, c netResumeCase) ([]*Game, int) {
	t.Helper()
	var mu sync.Mutex
	var mem *netplay.Mem
	dropped, lost := false, -1
	nt := &netTurner{turns: 5, firstRow: true}
	// The machine that presses Turn first: the lost one on the turn he is
	// lost in (turn), then one that is left.
	nt.hostFor = func(sd uint32) int {
		mu.Lock()
		defer mu.Unlock()
		switch {
		case dropped:
			return int(b2i(c.lost == 0))
		case c.turn:
			return c.lost
		}
		return 0
	}
	inRace := lobbyInRace
	n := c.humans + int(b2i(c.back))
	scripts := make([][]lobbyStep, n)
	steps := make([]int, n)
	scripts[0] = []lobbyStep{
		{"multiplayer", func(g *Game) bool { return g.mpSetup != nil }, pushKey('s')},
		{"new game", func(g *Game) bool { return g.currentScreen == ScreenNewGame && g.ng.acceptField > 0 },
			clickFieldOf(func(g *Game) int { return g.ng.acceptField })},
		{"start", func(g *Game) bool {
			return g.netScr != nil && g.netScr.startF > 0 && g.netGetUsers(nil) == int32(c.humans) && g.net.u8(netState) == 1 || inRace(g)
		}, func(g *Game, sys *host.System) {
			if !inRace(g) {
				sys.PushKey(letterKey('s'))
			}
		}},
		{"race", inRace, pushKey('a')},
		{"name", func(g *Game) bool { return g.rsel.player != nil && g.e.FieldsCount() <= 3 }, pushKey('\r')},
		{"banner", func(g *Game) bool { return g.e.FieldsCount() >= 4 && g.e.FieldsCount() <= 10 }, pushKey('1')},
	}
	for k := 1; k < c.humans; k++ {
		prev := func() bool { return steps[k-1] == len(scripts[k-1]) }
		scripts[k] = []lobbyStep{
			{"multiplayer", func(g *Game) bool { return g.mpSetup != nil && (k == 1 || steps[k-1] >= 2) }, pushKey('j')},
			{"game list", func(g *Game) bool { return g.netScr != nil && g.netScr.rows[0] > 0 },
				clickFieldOf(func(g *Game) int { return g.netScr.rows[0] })},
			{"race", func(g *Game) bool { return inRace(g) && prev() }, pushKey(byte('a' + k))},
			{"name", func(g *Game) bool { return g.rsel.player != nil && g.e.FieldsCount() <= 3 }, pushKey('\r')},
			{"banner", func(g *Game) bool { return g.e.FieldsCount() >= 4 && g.e.FieldsCount() <= 10 }, pushKey('1')},
		}
	}
	var nets []netplay.Network
	var conns []*lobby.Conn
	var srv *lobbytest.Server
	if c.lobby {
		srv = lobbytest.Start(t)
		nets, conns = lobbyConns(t, srv, c.humans)
		scripts = append(lobbyScripts(c.humans, steps[:c.humans]), scripts[c.humans:]...)
	}
	var games []*Game
	waiting := func() bool {
		mu.Lock()
		defer mu.Unlock()
		if !dropped || games == nil {
			return false
		}
		for m, g := range games[:c.humans] {
			if m != c.lost && g != nil && g.net.resume.active && g.netResumer() {
				return true
			}
		}
		return false
	}
	if c.back && c.lobby {
		// The lost player logs in again and joins his started game.
		scripts[c.humans] = []lobbyStep{
			{"multiplayer", func(g *Game) bool { return g.mpSetup != nil && waiting() }, func(g *Game, sys *host.System) {
				conn, err := lobby.Dial(lobby.Settings{Server: srv.Addr, TLS: srv.TLS})
				if err != nil {
					t.Error(err)
					return
				}
				t.Cleanup(func() { conn.Close() })
				if err := conn.Resume(string(rune('a' + c.lost))); err != nil {
					t.Error(err)
					return
				}
				conn.Poll()
				j, err := conn.JoinGame(nets[0].(*lobby.Network).GameID())
				if err != nil {
					t.Errorf("the lost player joins again: %v", err)
					return
				}
				g.net.network = lobby.NewNetwork(conn, j)
				sys.PushKey(letterKey('j'))
			}},
		}
	} else if c.back {
		scripts[c.humans] = []lobbyStep{
			{"multiplayer", func(g *Game) bool { return g.mpSetup != nil && waiting() }, pushKey('j')},
			{"game list", func(g *Game) bool { return g.netScr != nil && g.netScr.rows[0] > 0 },
				clickFieldOf(func(g *Game) int { return g.netScr.rows[0] })},
		}
	}
	if c.back {
		scripts[c.humans] = append(scripts[c.humans], []lobbyStep{
			{"pick", func(g *Game) bool {
				mu.Lock()
				p := lost
				mu.Unlock()
				return p >= 0 && g.hs.mp != nil && g.hs.mp.nameField[p] > 0
			}, clickFieldOf(func(g *Game) int { return g.hs.mp.nameField[lost] })},
		}...)
	}
	setup := func(m int, g *Game, opt *Options) {
		if opt != nil {
			opt.ListKey = true
			opt.Seed = uint32(7000 + m)
			opt.NetCheck = true
			if c.lobby && m < c.humans {
				opt.Net = nets[m]
			}
			return
		}
		g.settings[0xd5] = byte(c.humans)
		g.settings[2] = 0
		g.settings[0xd3] = 0
		g.net.checkHook = func(sd uint32, from uint16, same bool) {
			if same && g.net.resume.resumed > 0 && g.netIDToPlayerID(int32(from)) == int32(lost) {
				g.net.resume.checked++ // the returner's state equals this one's
			}
			if !same {
				t.Errorf("machine %d: the desync check fired at stardate %d against net id %d", m, sd, from)
			}
		}
		waits := 0
		shoot := func() {
			if d := os.Getenv("OPENORION2_NET_SHOTS"); d != "" && waits == 30 {
				// The waiting screen, as each machine shows it.
				g.e.SaveScreenPNG(filepath.Join(d, fmt.Sprintf("%s-m%d-resume.png", c, m)))
			}
			waits++
		}
		g.net.resume.waitHook = shoot
		if !c.back {
			pressed := false
			g.net.resume.waitHook = func() {
				shoot()
				if waits < 40 {
					return // a while on the waiting screen first
				}
				if !pressed && g.netResumer() && g.netScr != nil && g.netScr.startF > 0 && g.e.Sys != nil {
					pressed = true
					g.e.Sys.PushKey(letterKey('s'))
					g.e.Sys.PushKey(letterKey('y')) // the confirmation
				}
			}
		}
	}
	wrap := func(m int, nw netplay.Network) netplay.Network {
		mu.Lock()
		mem = nw.(*netplay.Mem)
		mem.Hosted = c.lost == 0 // the lobby's relay: the creator may be lost alone
		mu.Unlock()
		return nw
	}
	after := func(m int, g *Game, sys *host.System, gs []*Game) bool {
		mu.Lock()
		games = gs
		mu.Unlock()
		if m == c.lost && nt.start[m] != 0 && g.state.Stardate == nt.start[m]+1 {
			at := netMainScreenUp(g)
			if c.turn {
				// He hosts the turn: the others' uploads are coming in.
				at = g.net.chat != nil && g.net.i16(netHostID) == g.net.i16(netOwnID) && g.net.u8(netState) >= 9
			}
			if at {
				mu.Lock()
				dropped, lost = true, int(g.state.PlayerNum)
				mu.Unlock()
				if c.lobby {
					conns[m].Close() // his link to the lobby goes
				} else {
					mem.Drop(g.net.ep.Game().Name, g.net.ep.ID())
				}
				return true
			}
		}
		if g.net.resume.active {
			return false // the waiting screen's START is the test's to press
		}
		return nt.after(m, g, sys, gs)
	}
	gs, _, errs := netRunScripts(t, scripts, steps, setup, after, wrap)
	for m, err := range errs {
		var ee *ExitError
		if err != nil && !errors.Is(err, host.ErrQuit) && !(m == c.lost && errors.As(err, &ee)) {
			t.Errorf("machine %d: %v", m, err)
		}
	}
	if !dropped {
		t.Fatal("the lost machine was never dropped")
	}
	return gs, lost
}

// TestNetResume: a lost player comes back on another machine, takes his
// empire, and everyone plays on from the start of the turn with the same
// state (the desync check compares them); or nobody comes back, START goes
// on and the computer plays his empire.
func TestNetResume(t *testing.T) {
	t.Parallel()
	for _, c := range []netResumeCase{
		{humans: 2, lost: 1, back: true},
		{humans: 2, lost: 1},
		{humans: 3, lost: 0, back: true}, // the creator, in the lobby: another machine resumes
		{humans: 3, lost: 1, turn: true, back: true},
		{humans: 3, lost: 2, turn: true},
		{humans: 2, lost: 1, back: true, lobby: true},
		{humans: 3, lost: 0, back: true, lobby: true},
		{humans: 3, lost: 1, lobby: true},
	} {
		t.Run(c.String(), func(t *testing.T) {
			t.Parallel()
			gs, lost := netResumeRun(t, c)
			for m, g := range gs {
				if m == c.lost || g == nil {
					continue
				}
				if m < c.humans && g.net.resume.resumed != 1 {
					t.Errorf("machine %d: %d resumes, want 1", m, g.net.resume.resumed)
				}
				pl := &g.state.Players[lost]
				switch {
				case c.back && m == c.humans:
					if g.state.PlayerNum != int16(lost) {
						t.Errorf("the returner plays player %d, want %d", g.state.PlayerNum, lost)
					}
				case c.back:
					if pl.Objective != 100 || pl.NetID == -1 {
						t.Errorf("machine %d: the lost player: net id %d, objective %d", m, pl.NetID, pl.Objective)
					}
					if g.net.resume.checked < 2 {
						t.Errorf("machine %d: %d desync checks against the returner, want 2 or more", m, g.net.resume.checked)
					}
				default:
					if pl.Objective == 100 || pl.NetID != -1 {
						t.Errorf("machine %d: the lost player is still human: net id %d, objective %d", m, pl.NetID, pl.Objective)
					}
				}
			}
		})
	}
}
