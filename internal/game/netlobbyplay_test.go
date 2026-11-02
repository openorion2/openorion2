package game

import (
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/openorion2/openorion2/internal/host"
	"github.com/openorion2/openorion2/internal/lobby"
	"github.com/openorion2/openorion2/internal/lobby/lobbytest"
	"github.com/openorion2/openorion2/internal/netplay"
	"github.com/openorion2/openorion2/internal/save"
)

// lobbyNetworks connects n players to lobby srv; the first creates a game
// the others join, and each gets the netplay.Network playing it.
func lobbyNetworks(t *testing.T, srv *lobbytest.Server, n int) []netplay.Network {
	t.Helper()
	nets, _ := lobbyConns(t, srv, n)
	return nets
}

// lobbyConns is lobbyNetworks with the connections.
func lobbyConns(t *testing.T, srv *lobbytest.Server, n int) ([]netplay.Network, []*lobby.Conn) {
	t.Helper()
	conns := make([]*lobby.Conn, n)
	nets := make([]netplay.Network, n)
	var gameID string
	for i := range n {
		conn, err := lobby.Dial(lobby.Settings{Server: srv.Addr, TLS: srv.TLS})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { conn.Close() })
		conns[i] = conn
		if err := conn.Resume(string(rune('a' + i))); err != nil {
			t.Fatal(err)
		}
		conn.Poll()
		if i == 0 {
			// Public: its start takes it off the list (GameRemoved), which
			// must not end it for its members.
			j, err := conn.CreateGame("lobby game", true)
			if err != nil {
				t.Fatal(err)
			}
			gameID = j.Game.Id
			nets[i] = lobby.NewNetwork(conn, j)
			continue
		}
		j, err := conn.JoinGame(gameID)
		if err != nil {
			t.Fatal(err)
		}
		nets[i] = lobby.NewNetwork(conn, j)
	}
	return nets, conns
}

// lobbyScripts: the creator starts from START once n humans are in; each
// joiner's JOIN goes to the lobby's game without the list, and picks race
// k once joiner k-1 is through.
func lobbyScripts(n int, steps []int) [][]lobbyStep {
	inRace := lobbyInRace
	scripts := make([][]lobbyStep, n)
	scripts[0] = []lobbyStep{
		{"multiplayer", func(g *Game) bool { return g.mpSetup != nil }, pushKey('s')},
		{"new game", func(g *Game) bool { return g.currentScreen == ScreenNewGame && g.ng.acceptField > 0 },
			clickFieldOf(func(g *Game) int { return g.ng.acceptField })},
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
		prev := func() bool { return steps[k-1] == len(scripts[k-1]) }
		scripts[k] = []lobbyStep{
			{"multiplayer", func(g *Game) bool { return g.mpSetup != nil }, pushKey('j')},
			{"race", func(g *Game) bool { return inRace(g) && prev() }, pushKey(byte('a' + k))},
			{"name", func(g *Game) bool { return g.rsel.player != nil && g.e.FieldsCount() <= 3 }, pushKey('\r')},
			{"banner", func(g *Game) bool { return g.e.FieldsCount() >= 4 && g.e.FieldsCount() <= 10 }, pushKey('1')},
		}
	}
	return scripts
}

// TestNetTurnsLobby plays the start and three turns of a two-machine game
// through the online lobby's relay (lobbytest, the real server's rooms).
func TestNetTurnsLobby(t *testing.T) {
	t.Parallel()
	srv := lobbytest.Start(t)
	nets := lobbyNetworks(t, srv, 2)
	steps := make([]int, 2)
	nt := &netTurner{turns: 3, firstRow: true}
	games, _, errs := netRunScripts(t, lobbyScripts(2, steps), steps, func(m int, g *Game, opt *Options) {
		if opt != nil {
			opt.Net = nets[m]
			opt.ListKey = true
		}
	}, func(m int, g *Game, sys *host.System, games []*Game) bool {
		return nt.after(m, g, sys, games)
	}, nil)
	for m, err := range errs {
		if err != nil && !errors.Is(err, host.ErrQuit) {
			t.Fatalf("machine %d: %v", m, err)
		}
	}
	for m, g := range games {
		if g == nil || g.state.Stardate != 35003 {
			t.Fatalf("machine %d did not reach stardate 35003", m)
		}
		g.netUninit()
	}
	if srv.Frames() == 0 {
		t.Fatal("nothing went through the lobby")
	}
	t.Logf("%d frames relayed", srv.Frames())
}

// TestNetDropLobby: three machines through the lobby; the third leaves
// mid-game and the other two go on with his empire run by the AI, as over
// TCP (TestNetDropTCP).
func TestNetDropLobby(t *testing.T) {
	t.Parallel()
	srv := lobbytest.Start(t)
	nets := lobbyNetworks(t, srv, 3)
	steps := make([]int, 3)
	nt := &netTurner{turns: 5, firstRow: true}
	gone := false
	games, _, errs := netRunScripts(t, lobbyScripts(3, steps), steps, func(m int, g *Game, opt *Options) {
		if opt != nil {
			opt.Net = nets[m]
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
	}, nil)
	if !gone {
		t.Fatal("machine 2 never left")
	}
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

// TestNetLoadLobby loads a network save through the lobby's screens: the
// creator's TEN, LOAD, the description and the save picker (no game name
// box: the lobby's game has its name), the joiner's TEN and JOIN; then two
// turns. The save is made first by a two-machine game over the in-memory
// transport.
func TestNetLoadLobby(t *testing.T) {
	t.Parallel()
	nt0 := &netTurner{turns: 1, firstRow: true}
	first, _ := netGameRunN(t, 2, func(m int, g *Game, opt *Options) {
		if opt != nil {
			opt.ListKey = true
			opt.Seed = uint32(6000 + m)
		}
	}, func(m int, g *Game, sys *host.System, games []*Game) bool {
		return nt0.after(m, g, sys, games)
	})
	sav, err := os.ReadFile(filepath.Join(first[0].dir, "SAVE10.GAM"))
	if err != nil {
		t.Fatal(err)
	}
	start := first[0].state.Stardate

	srv := lobbytest.Start(t)
	const earlier = "00000000-0000-4000-8000-0000000000aa" // the evening the save was made
	creator := func(g *Game) bool { return g.lobbyConn != nil && g.lobbyConn.State.Mine != nil }
	scripts := func(m int) []lobbyStep {
		empire := netLoadSteps(m, 2, m)
		if m == 0 {
			return append([]lobbyStep{
				{"multiplayer", mpScreenUp, pushKey('t')},
				{"lobby", lobbyUp, clickFieldOf(func(g *Game) int { return g.lobbyScr.loadF })},
				{"description", uniBoxUp, enterKey},
				{"public?", confirmUp, pushKey('y')},
			}, empire[2:]...) // the slot on: no game name box
		}
		return append([]lobbyStep{
			{"multiplayer", mpScreenUp, pushKey('t')},
			{"lobby", func(g *Game) bool { return lobbyUp(g) && len(g.lobbyScr.conn.State.Games) == 1 },
				clickFieldOf(func(g *Game) int { return g.lobbyScr.joinF })},
			{"join box", joinBoxUp, clickFieldOf(func(g *Game) int { return g.lobbyJoinScr.rowF[0] })},
			{"picked", func(g *Game) bool { return joinBoxUp(g) && g.lobbyJoinScr.sel != "" },
				clickFieldOf(func(g *Game) int { return g.lobbyJoinScr.joinF })},
		}, empire[2:]...) // the empire: no game list
	}
	nt := &netTurner{turns: 2, firstRow: true}
	games := netLoadRunWith(t, sav, make([]netplay.Network, 2), scripts, func(m int, g *Game) {
		g.lobbyTLS = srv.TLS
		os.WriteFile(filepath.Join(g.dir, "lobby.yaml"), []byte("server: "+srv.Addr+"\n"), 0o644)
		lobby.Remembered{User: string(rune('a' + m)), Session: string(rune('a' + m))}.Save(g.dir)
		if m == 0 { // the save's link to the lobby game it was made in
			os.WriteFile(filepath.Join(g.dir, "SAVE1.LBY"), []byte(earlier+"\n"), 0o644)
		}
	}, func(m int, g *Game, sys *host.System, games []*Game) bool {
		return nt.after(m, g, sys, games)
	})
	for m, g := range games {
		if g.state.Stardate != start+2 || g.state.PlayerNum != int16(m) {
			t.Errorf("machine %d at stardate %d (the save's %d), player %d", m, g.state.Stardate, start, g.state.PlayerNum)
		}
		if !creator(g) {
			t.Errorf("machine %d is not in the lobby's game", m)
		}
	}
	if srv.Frames() == 0 {
		t.Fatal("nothing went through the lobby")
	}
	if len(srv.Continues()) != 1 {
		t.Fatalf("continues %v", srv.Continues())
	}
	for _, prev := range srv.Continues() {
		if prev != earlier {
			t.Fatalf("the game continues %q", prev)
		}
	}
}

// TestNetDesyncLobby: a lobby game checks its state across the machines
// without -netcheck, and a mismatch goes to the lobby as a report with the
// machine's save (here machine 1's state is spoilt after one broadcast).
func TestNetDesyncLobby(t *testing.T) {
	t.Parallel()
	srv := lobbytest.Start(t)
	nets := lobbyNetworks(t, srv, 2)
	steps := make([]int, 2)
	nt := &netTurner{turns: 3, firstRow: true}
	var mu sync.Mutex
	var sent []error
	games, _, errs := netRunScripts(t, lobbyScripts(2, steps), steps, func(m int, g *Game, opt *Options) {
		if opt != nil {
			opt.Net = nets[m]
			opt.ListKey = true
			return
		}
		g.Log = func(string, ...any) {}
		g.net.checks.done = func(err error) {
			mu.Lock()
			sent = append(sent, err)
			mu.Unlock()
		}
		if m == 1 {
			g.net.turnHook = func(bool) {
				if g.state.Stardate == 35002 {
					g.state.Players[0].BC++
				}
			}
		}
	}, func(m int, g *Game, sys *host.System, games []*Game) bool {
		return nt.after(m, g, sys, games)
	}, nil)
	for m, err := range errs {
		if err != nil && !errors.Is(err, host.ErrQuit) {
			t.Fatalf("machine %d: %v", m, err)
		}
	}
	for i := 0; i < 100; i++ {
		mu.Lock()
		n := len(sent)
		mu.Unlock()
		if n >= 2 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	for m, g := range games {
		if !g.net.checks.told {
			t.Errorf("machine %d: the player was not told", m)
		}
	}
	ds := srv.Desyncs()
	if len(ds) != 2 {
		t.Fatalf("%d reports, want 2 (one from each machine): %v", len(ds), sent)
	}
	for _, d := range ds {
		r := d.Report
		zr, err := gzip.NewReader(bytes.NewReader(r.Save))
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(zr)
		if err != nil {
			t.Fatal(err)
		}
		sv, err := save.Parse(b)
		if err != nil {
			t.Fatalf("net id %d: the save: %v", d.NetID, err)
		}
		if r.Stardate != 35002 || r.OtherNetId == d.NetID || len(r.Hash) != 32 || bytes.Equal(r.Hash, r.OtherHash) || sv == nil {
			t.Errorf("net id %d: report %d, other %d", d.NetID, r.Stardate, r.OtherNetId)
		}
	}
}
