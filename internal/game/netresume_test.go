package game

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"

	"github.com/openorion2/openorion2/internal/host"
	"github.com/openorion2/openorion2/internal/lobby"
	"github.com/openorion2/openorion2/internal/lobby/lobbytest"
	"github.com/openorion2/openorion2/internal/netplay"
)

// Port-only: a lost player's return (netresume.go, docs/net/hardening.md).

// netResumeCase is one game of TestNetResume. humans machines start it;
// the lost machines' links are cut at once, when the first of them is at
// when; backs of them come back on new machines, the last lost first. The
// current resumer presses START once those are in (or at once), so the
// others' empires go to the computer.
type netResumeCase struct {
	humans      int
	lost        []int
	when        string // "main" on the second turn, "first" turn, "turnhost" (hosting it, uploads coming in), "battle" (a tactical lockstep battle)
	backs       int
	resumerLost bool   // the resumer is lost too while it waits
	again       bool   // the first returner is lost again two turns later: nobody comes back
	net         string // "mem", "lobby" (lobbytest, the real server's rooms), "tcp"
}

func (c netResumeCase) String() string {
	when := c.when
	if when == "" {
		when = "main"
	}
	s := fmt.Sprintf("%s-%dh-lost%v-%s-back%d", c.net, c.humans, c.lost, when, c.backs)
	if c.resumerLost {
		s += "-resumerlost"
	}
	if c.again {
		s += "-again"
	}
	return s
}

// netResumeOut is what netResumeRun saw.
type netResumeOut struct {
	games    []*Game
	empire   map[int]int // an original machine's empire when it was lost
	returner []int       // the machine each returner came back for
}

// netResumeRun plays c.
func netResumeRun(t *testing.T, c netResumeCase) netResumeOut {
	t.Helper()
	var mu sync.Mutex
	var mem *netplay.Mem
	n := c.humans + c.backs
	alive := make([]bool, n)
	for m := range n {
		alive[m] = true
	}
	out := netResumeOut{empire: map[int]int{}}
	for i := range c.backs {
		out.returner = append(out.returner, c.lost[len(c.lost)-1-i])
	}
	dropped, redropped := false, false
	barriers := 0 // the lost machine's lockstep barriers (battle)
	tacs := make([]int, c.humans+c.backs)
	nt := &netTurner{turns: 6, firstRow: true}
	if os.Getenv("OPENORION2_NET_SOAKLOG") != "" {
		nt.log = func(m int, f string, a ...any) { t.Logf("machine %d: "+f, append([]any{m}, a...)...) }
	}
	firstAlive := func() int {
		for m, a := range alive {
			if a && (m < c.humans || dropped) {
				return m
			}
		}
		return 0
	}
	nt.hostFor = func(sd uint32) int {
		mu.Lock()
		defer mu.Unlock()
		if !dropped && c.when == "turnhost" {
			return c.lost[0]
		}
		return firstAlive()
	}
	var tcpAddr string
	if c.net == "tcp" {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Skip(err)
		}
		tcpAddr = l.Addr().String()
		l.Close()
	}
	steps := make([]int, n)
	scripts := make([][]lobbyStep, n)
	var nets []netplay.Network
	var conns []*lobby.Conn
	var srv *lobbytest.Server
	if c.net == "lobby" {
		srv = lobbytest.Start(t)
		nets, conns = lobbyConns(t, srv, c.humans)
		copy(scripts, lobbyScripts(c.humans, steps[:c.humans]))
	} else {
		copy(scripts, netResumeScripts(c.humans, steps))
	}
	var games []*Game
	resumerWaits := func() int {
		mu.Lock()
		defer mu.Unlock()
		if !dropped || games == nil {
			return -1
		}
		for m, g := range games[:n] {
			if alive[m] && g != nil && g.net.ep != nil && g.net.resume.active && g.netResumer() {
				return m
			}
		}
		return -1
	}
	returnerConns := make([]*lobby.Conn, n)
	for i := range c.backs {
		m := c.humans + i
		who := out.returner[i]
		ready := func(g *Game) bool {
			if g.mpSetup == nil || resumerWaits() < 0 || i > 0 && steps[m-1] < len(scripts[m-1]) {
				return false
			}
			mu.Lock()
			defer mu.Unlock()
			return !c.resumerLost || redropped
		}
		if c.net == "lobby" {
			// He logs in again and joins his started game.
			scripts[m] = []lobbyStep{{"multiplayer", ready, func(g *Game, sys *host.System) {
				conn, err := lobby.Dial(lobby.Settings{Server: srv.Addr, TLS: srv.TLS})
				if err != nil {
					t.Error(err)
					return
				}
				t.Cleanup(func() { conn.Close() })
				if err := conn.Resume(string(rune('a' + who))); err != nil {
					t.Error(err)
					return
				}
				conn.Poll()
				j, err := conn.JoinGame(nets[0].(*lobby.Network).GameID())
				if err != nil {
					t.Errorf("the lost player joins again: %v", err)
					return
				}
				mu.Lock()
				returnerConns[m] = conn
				mu.Unlock()
				g.net.network = lobby.NewNetwork(conn, j)
				sys.PushKey(letterKey('j'))
			}}}
		} else {
			scripts[m] = []lobbyStep{
				{"multiplayer", ready, pushKey('j')},
				{"game list", func(g *Game) bool { return g.netScr != nil && g.netScr.rows[0] > 0 },
					clickFieldOf(func(g *Game) int { return g.netScr.rows[0] })},
			}
		}
		scripts[m] = append(scripts[m], lobbyStep{"pick", func(g *Game) bool { return netResumeFree(g) >= 0 },
			clickFieldOf(func(g *Game) int { return g.hs.mp.nameField[netResumeFree(g)] })})
	}
	// The empires still human with a station, once everyone who comes back is in.
	want := c.humans - len(c.lost) - int(b2i(c.resumerLost)) + c.backs
	setup := func(m int, g *Game, opt *Options) {
		if opt != nil {
			opt.ListKey = true
			opt.Seed = uint32(7000 + m)
			if c.when == "battle" {
				// 7000's first lockstep battle ends in the original's deadlock
				// (TestNetSoak, docs/net/combat.md, "The lockstep").
				opt.Seed = uint32(7100 + m)
			}
			opt.NetCheck = true
			switch {
			case c.net == "lobby" && m < c.humans:
				opt.Net = nets[m]
			case c.net == "tcp":
				opt.Net = &netplay.TCP{Listen: tcpAddr, Addr: tcpAddr}
			}
			return
		}
		g.settings[0xd5] = byte(c.humans)
		g.settings[2] = 0
		g.settings[0xd3] = 0
		if c.when == "battle" {
			// As TestNetDropLockstep: battles between the humans, fought in
			// the lockstep without the combat screen.
			g.settings[0xd8] = 0
			g.net.preTurnHook = func() { netSoakBattles(g, netBattlesHumans) }
			g.combatUI = netSoakUI{g, false}
			g.net.tacHook = func() {
				mu.Lock()
				if m == c.lost[0] {
					barriers++
				}
				tacs[m]++
				mu.Unlock()
			}
		}
		g.net.checkHook = func(sd uint32, from uint16, same bool) {
			if !same {
				t.Errorf("machine %d: the desync check fired at stardate %d against net id %d", m, sd, from)
				return
			}
			mu.Lock()
			defer mu.Unlock()
			if g.net.resume.resumed == 0 {
				return
			}
			p := int(g.netIDToPlayerID(int32(from)))
			for _, w := range out.returner {
				if e, ok := out.empire[w]; ok && e == p {
					g.net.resume.checked++ // a returner's state equals this one's
				}
			}
		}
		waits := 0
		pressed := -1 // the resume START was pressed in
		g.net.resume.waitHook = func() {
			if d := os.Getenv("OPENORION2_NET_SHOTS"); d != "" && waits == 30 {
				g.e.SaveScreenPNG(filepath.Join(d, fmt.Sprintf("%s-m%d-resume.png", c, m)))
			}
			waits++
			if pressed == g.net.resume.resumed || waits < 40 || !g.netResumer() || g.netScr == nil || g.netScr.startF <= 0 || g.e.Sys == nil {
				return
			}
			mu.Lock()
			again := redropped && c.again
			mu.Unlock()
			if g.netResumePlaced() >= want || again {
				pressed = g.net.resume.resumed
				g.e.Sys.PushKey(letterKey('s'))
				g.e.Sys.PushKey(letterKey('y')) // the confirmation
			}
		}
	}
	wrap := func(m int, nw netplay.Network) netplay.Network {
		mu.Lock()
		defer mu.Unlock()
		if c.net == "mem" {
			mem = nw.(*netplay.Mem)
			mem.Hosted = slices.Contains(c.lost, 0) || c.resumerLost // the lobby's relay
		}
		return nw
	}
	cut := func(m int, g *Game) {
		alive[m] = false
		switch {
		case c.net == "lobby" && m < c.humans:
			conns[m].Close()
		case c.net == "lobby":
			returnerConns[m].Close()
		case c.net == "mem":
			mem.Drop(g.net.ep.Game().Name, g.net.ep.ID())
		default:
			g.net.ep.Close()
		}
	}
	lostAt := func(g *Game, m int) bool {
		sd := nt.start[m]
		switch c.when {
		case "first":
			return netMainScreenUp(g) && g.state.Stardate == sd
		case "turnhost":
			return g.state.Stardate == sd+1 && g.net.chat != nil && g.net.i16(netHostID) == g.net.i16(netOwnID) && g.net.u8(netState) >= 9
		case "battle":
			return barriers >= 4
		}
		return netMainScreenUp(g) && g.state.Stardate == sd+1
	}
	after := func(m int, g *Game, sys *host.System, gs []*Game) bool {
		mu.Lock()
		games = gs
		if !alive[m] {
			mu.Unlock()
			return true
		}
		if !dropped && m == c.lost[0] && nt.start[m] != 0 && lostAt(g, m) {
			dropped = true
			for _, k := range c.lost {
				if gs[k] != nil {
					out.empire[k] = int(gs[k].state.PlayerNum)
					cut(k, gs[k])
				}
			}
			mu.Unlock()
			return true
		}
		if c.resumerLost && dropped && !redropped && g.net.resume.active && g.netResumer() {
			redropped = true
			cut(m, g)
			mu.Unlock()
			return true
		}
		if c.again && dropped && !redropped && m == c.humans && nt.start[m] != 0 &&
			netMainScreenUp(g) && g.state.Stardate == nt.start[m]+2 {
			redropped = true
			cut(m, g)
			mu.Unlock()
			return true
		}
		active := g.net.resume.active
		mu.Unlock()
		if active {
			return false // the waiting screen's START is waitHook's
		}
		return nt.after(m, g, sys, gs)
	}
	gs, _, errs := netRunScripts(t, scripts, steps, setup, after, wrap)
	for m, err := range errs {
		var ee *ExitError
		if err != nil && !errors.Is(err, host.ErrQuit) && !(!alive[m] && errors.As(err, &ee)) {
			t.Errorf("machine %d: %v", m, err)
		}
	}
	if !dropped {
		t.Fatal("nobody was dropped")
	}
	for m, g := range gs {
		if g != nil {
			t.Logf("machine %d: stardate %d, screen %#x, net state %#x, resumes %d, barriers %d, in battle %v, players %d, net ids %v", m, g.state.Stardate, g.currentScreen, g.net.u8(netState), g.net.resume.resumed, tacs[m], g.tc != nil, g.state.PlayerNum, func() []int16 {
				var ids []int16
				for k := 0; k < int(g.state.NumPlayers); k++ {
					ids = append(ids, g.state.Players[k].NetID)
				}
				return ids
			}())
		}
	}
	out.games = gs
	return out
}

// netResumeScripts: as netGameRunNE's, for humans of the n machines.
func netResumeScripts(humans int, steps []int) [][]lobbyStep {
	inRace := lobbyInRace
	scripts := make([][]lobbyStep, humans)
	scripts[0] = []lobbyStep{
		{"multiplayer", func(g *Game) bool { return g.mpSetup != nil }, pushKey('s')},
		{"new game", func(g *Game) bool { return g.currentScreen == ScreenNewGame && g.ng.acceptField > 0 },
			clickFieldOf(func(g *Game) int { return g.ng.acceptField })},
		{"start", func(g *Game) bool {
			return g.netScr != nil && g.netScr.startF > 0 && g.netGetUsers(nil) == int32(humans) && g.net.u8(netState) == 1 || inRace(g)
		}, func(g *Game, sys *host.System) {
			if !inRace(g) {
				sys.PushKey(letterKey('s'))
			}
		}},
		{"race", inRace, pushKey('a')},
		{"name", func(g *Game) bool { return g.rsel.player != nil && g.e.FieldsCount() <= 3 }, pushKey('\r')},
		{"banner", func(g *Game) bool { return g.e.FieldsCount() >= 4 && g.e.FieldsCount() <= 10 }, pushKey('1')},
	}
	for k := 1; k < humans; k++ {
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
	return scripts
}

// netResumeFree is the first human empire the picker shows free, or -1.
func netResumeFree(g *Game) int {
	if g.hs.mp == nil {
		return -1
	}
	s := &g.state
	for k := 0; k < int(s.NumPlayers); k++ {
		if s.Players[k].Objective == 100 && s.Players[k].NetID == -1 && g.hs.mp.nameField[k] > 0 {
			return k
		}
	}
	return -1
}

// netResumePlaced counts the human empires with a station (the resumer).
func (g *Game) netResumePlaced() int {
	n := 0
	for k := 0; k < int(g.state.NumPlayers); k++ {
		if pl := &g.state.Players[k]; pl.Objective == 100 && pl.NetID != -1 {
			n++
		}
	}
	return n
}

// TestNetResume: lost players come back on other machines and take their
// empires, and everyone plays on from the start of the turn with the same
// state (the desync check compares every machine with each returner); the
// empires nobody comes back for go to the computer after START.
func TestNetResume(t *testing.T) {
	t.Parallel()
	for _, c := range []netResumeCase{
		{humans: 2, lost: []int{1}, backs: 1, net: "mem"},
		{humans: 2, lost: []int{1}, net: "mem"},
		{humans: 2, lost: []int{1}, when: "first", backs: 1, net: "mem"},
		{humans: 3, lost: []int{0}, backs: 1, net: "mem"}, // the creator, as in the lobby
		{humans: 3, lost: []int{1}, when: "turnhost", backs: 1, net: "mem"},
		{humans: 3, lost: []int{2}, when: "turnhost", net: "mem"},
		{humans: 2, lost: []int{1}, when: "battle", backs: 1, net: "mem"},
		{humans: 3, lost: []int{2}, backs: 1, resumerLost: true, net: "mem"},
		{humans: 2, lost: []int{1}, backs: 1, again: true, net: "mem"},
		{humans: 3, lost: []int{1, 2}, backs: 2, net: "mem"},
		{humans: 2, lost: []int{1}, backs: 1, net: "lobby"},
		{humans: 3, lost: []int{0}, backs: 1, net: "lobby"},
		{humans: 3, lost: []int{1}, net: "lobby"},
		{humans: 3, lost: []int{1, 2}, backs: 2, net: "lobby"}, // each gets his own empire
		{humans: 3, lost: []int{1}, backs: 1, net: "tcp"},
	} {
		t.Run(c.String(), func(t *testing.T) {
			t.Parallel()
			out := netResumeRun(t, c)
			gs := out.games
			returned := map[int]bool{}
			for i, w := range out.returner {
				g := gs[c.humans+i]
				if g == nil {
					t.Fatalf("returner %d never ran", i)
				}
				if _, ok := out.empire[w]; !ok {
					t.Fatalf("machine %d's empire unknown", w)
				}
				p := int(g.state.PlayerNum)
				returned[p] = true
				if c.net == "lobby" && p != out.empire[w] {
					t.Errorf("returner for machine %d plays empire %d, want his own %d", w, p, out.empire[w])
				}
			}
			resumes := 1 + int(b2i(c.again))
			for m := range c.humans {
				g := gs[m]
				if g == nil || slices.Contains(c.lost, m) || c.resumerLost && g.net.resume.resumed < 2 {
					continue // lost (a resumer lost while it waited resumed once)
				}
				if r := g.net.resume.resumed; r < resumes {
					t.Errorf("machine %d: %d resumes, want %d", m, r, resumes)
				}
				if c.backs > 0 && !c.again && g.net.resume.checked < 2 {
					t.Errorf("machine %d: %d desync checks against the returners, want 2 or more", m, g.net.resume.checked)
				}
				for _, k := range c.lost {
					pl := &g.state.Players[out.empire[k]]
					back := returned[out.empire[k]] && !c.again
					if back != (pl.Objective == 100 && pl.NetID != -1) {
						t.Errorf("machine %d: lost empire %d: net id %d, objective %d (back %v)", m, out.empire[k], pl.NetID, pl.Objective, back)
					}
				}
			}
		})
	}
}
