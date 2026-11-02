package game

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/openorion2/openorion2/internal/host"
	"github.com/openorion2/openorion2/internal/netplay"
)

// The Net_* layer over internal/netplay and NETMOX.C's plumbing
// (docs/net/transport.md).

// withGames runs fn with n Games made in one headless host (fn never waits
// for a frame); each loads save into its own scratch directory, if given.
func withGames(t *testing.T, saves []string, fn func(gs []*Game) error) {
	t.Helper()
	// Skip here: scratchGameDir's skip inside the host's goroutine would
	// leave Run waiting.
	if _, err := os.Stat(filepath.Join("..", "..", "game")); err != nil {
		t.Skip(err)
	}
	for _, sv := range saves {
		if _, err := os.Stat(sv); sv != "" && err != nil {
			t.Skip(err)
		}
	}
	err := host.Run(host.Options{Headless: true}, func(sys *host.System) error {
		var gs []*Game
		for _, sv := range saves {
			dir := scratchGameDir(t, filepath.Join("..", "..", "game"))
			g, err := newGame(sys, dir)
			if err != nil {
				return err
			}
			g.Log = func(string, ...any) {}
			if sv != "" {
				copyFile(t, sv, filepath.Join(dir, "SAVE1.GAM"))
				if ok, err := g.loadGame(0); !ok || err != nil {
					return fmt.Errorf("load %s: %v", sv, err)
				}
			}
			gs = append(gs, g)
		}
		return fn(gs)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// netRun calls f and returns the ExitError it ends with, if any.
func netRun(g *Game, f func()) (err error) {
	defer g.recoverExit(&err)
	f()
	return nil
}

// attach makes g a station of a network game with net id ep.ID().
func attach(g *Game, ep netplay.Endpoint, log *[]NetRecord) {
	g.gameType = 2
	g.net.ep = ep
	g.net.inited = true
	g.net.receive = g.receiveMessage
	g.net.set16(netOwnID, int16(ep.ID()))
	g.net.set8(netActive, 1)
	g.netInitHostElection()
	if log != nil {
		g.net.log = func(r NetRecord) { *log = append(*log, r) }
	}
}

// TestNetReceiveRouting checks Receive_Message_'s split by id and
// Main_Receive_Message_'s case for every id up to 0x64 against the id
// table as read from the disassembly (docs/net/transport.md).
func TestNetReceiveRouting(t *testing.T) {
	t.Parallel()
	withGames(t, []string{""}, func(gs []*Game) error {
		g := gs[0]
		var logged []string
		g.Log = func(f string, a ...any) { logged = append(logged, fmt.Sprintf(f, a...)) }
		var hit uint32
		g.net.caseHook = func(c uint32) { hit = c }
		route := func(id uint16) string {
			logged, hit = nil, 0
			func() {
				// A ported case may end the game (msg 0 of another version,
				// msg 0x39 on the creator).
				defer func() { recover(); g.net.set8(netInReceive, 0) }()
				g.receiveMessage(1, id, []byte{0})
			}()
			if hit != 0 && len(logged) == 0 {
				logged = []string{fmt.Sprintf("case %#x", hit)}
			}
			if g.net.u8(netInReceive) != 0 {
				t.Fatalf("msg %#x: _in_receive_message left set", id)
			}
			return strings.Join(logged, "; ")
		}
		cases := g.exe.u32(mainReceiveJumps)
		for id := uint16(0); id <= 0x64; id++ {
			idx := -1 // the shared return
			switch {
			case id <= 0x1d:
				idx = int(id) + 1
			case id >= 0x1f && id <= 0x27:
				idx = 31 + int(id-0x1f)
			case id >= 0x2a && id <= 0x44:
				idx = 40 + int(id-0x2a)
			case id == 0x64:
				idx = 67
			}
			got := route(id)
			if idx < 0 {
				if got != "" {
					t.Errorf("msg %#x has no case, got %q", id, got)
				}
				continue
			}
			c := g.exe.u32(mainReceiveJumps + 4*uint32(idx))
			if c == cases || !strings.Contains(got, fmt.Sprintf("case %#x", c)) {
				t.Errorf("msg %#x: want case %#x, got %q", id, c, got)
			}
		}
		for _, k := range []struct {
			id   uint16
			want string
		}{{0x65, ""}, {999, ""}, {1000, ""}, {0x3ed, ""}, {1999, ""}} {
			if got := route(k.id); (k.want == "" && got != "") || !strings.Contains(got, k.want) {
				t.Errorf("msg %#x: %q", k.id, got)
			}
		}
		// Diplomacy_Receive_Message_ takes 2000 (the dialogue) and 0x834
		// (its chat) and drops ids from 3000 on.
		n := &g.dip.net
		g.receiveMessage(1, 2000, []byte{0x5a, 0, 0, 0, 0x99, 0, 0, 0})
		g.receiveMessage(1, 0x834, []byte{'h', 'i', 0})
		g.receiveMessage(1, 0xffff, []byte{0x33, 0})
		if n.pkt[0] != 0x5a || n.talker != 0x5a || g.dip.response != 0x99 || string(n.recvChat[:3]) != "hi\x00" {
			t.Errorf("diplomacy messages: packet % x, talker %d, response %#x, chat %q", n.pkt, n.talker, g.dip.response, n.recvChat[:3])
		}
		return nil
	})
}

// TestNetSendRouting: Mox_Send_Message_ to oneself, to one human, to every
// human (descending player order, net ids ≤ 0 skipped though counted), with
// nobody else, and a send to a gone station ending the game.
func TestNetSendRouting(t *testing.T) {
	t.Parallel()
	s1 := filepath.Join(savesDir(), "SAVE1.GAM")
	if !fileExists(s1) {
		t.Skip("no SAVE1.GAM")
	}
	withGames(t, []string{s1, s1, s1}, func(gs []*Game) error {
		mem := netplay.NewMem()
		logs := make([][]NetRecord, 3)
		for m, g := range gs {
			ep, err := mem.Create("G", "a")
			if m > 0 {
				ep, err = mem.Join("G", "b")
			}
			if err != nil {
				return err
			}
			attach(g, ep, &logs[m])
		}
		setIDs := func(ids ...int16) {
			for _, g := range gs {
				for k := range g.state.Players {
					g.state.Players[k].NetID = -1
				}
				for k, id := range ids {
					g.state.Players[k].NetID = id
				}
			}
		}
		recv := func(m int) (out []string) {
			logs[m] = nil
			gs[m].netUpdate()
			for _, r := range logs[m] {
				out = append(out, fmt.Sprintf("%#x<%d:%x", r.ID, r.Peer, r.Payload))
			}
			return
		}
		g := gs[0]
		// Players 0..2 are net ids 1..3, the rest AI.
		setIDs(1, 2, 3)
		logs[0] = nil
		if r := g.moxSendMessage(1, 0x3a, []byte{5}); r != 1 || len(logs[0]) != 2 || logs[0][1].Dir != 'R' || logs[0][1].Peer != 1 {
			t.Fatalf("to self: %d %+v", r, logs[0])
		}
		logs[0] = nil
		g.moxSendMessage(1, 0x36, nil)
		if r := logs[0]; len(r) != 2 || r[0].Len != 0 || r[1].Len != 1 || !slices.Equal(r[1].Stale, [][2]int{{0, 1}}) {
			t.Fatalf("zero length to self: %+v", r)
		}
		g.moxSendMessage(netAllHumans, 0x27, []byte("hi"))
		if a, b := recv(1), recv(2); !slices.Equal(a, []string{"0x27<1:6869"}) || !slices.Equal(b, a) {
			t.Fatalf("to all: %v %v", a, b)
		}
		g.moxSendMessage(3, 0xc, nil)
		if a, b := recv(1), recv(2); len(a) != 0 || !slices.Equal(b, []string{"0xc<1:00"}) {
			t.Fatalf("to 3: %v %v", a, b)
		}
		// Net id 0 counts as another human but is not sent to: with player
		// 1 at 0, msg 0x7ffe goes to net id 3 alone.
		setIDs(1, 0, 3)
		g.moxSendMessage(netAllHumans, 0x27, []byte{1})
		if a, b := recv(1), recv(2); len(a) != 0 || !slices.Equal(b, []string{"0x27<1:01"}) {
			t.Fatalf("net id 0: %v %v", a, b)
		}
		// Alone: nothing is sent and the send succeeds.
		setIDs(1)
		if r := g.moxSendMessage(netAllHumans, 0x27, []byte{2}); r != 1 || len(recv(1))+len(recv(2)) != 0 {
			t.Fatalf("alone: %d", r)
		}
		// A send to a station that left ends the game (port-only, with
		// -netdrop end: the lost-link check names him before the send fails).
		setIDs(1, 2, 3)
		g.net.dropEnd = true
		mem.Drop("G", 3)
		err := netRun(g, func() { g.moxSendMessage(3, 0xc, nil) })
		var ee *ExitError
		if !errors.As(err, &ee) || !ee.Notice || !strings.Contains(ee.Msg, "has lost the connection") {
			t.Fatalf("send to a dropped station: %v", err)
		}
		if g.net.ep != nil {
			t.Fatal("the link stays open after Exit_With_Message_")
		}
		return nil
	})
}

// TestNetDisconnect (port-only, hardening.md): with -netdrop end a lost joiner
// ends the creator's game naming him (mem, TCP); a lost relay the joiner's.
func TestNetDisconnect(t *testing.T) {
	t.Parallel()
	s1 := filepath.Join(savesDir(), "SAVE1.GAM")
	if !fileExists(s1) {
		t.Skip("no SAVE1.GAM")
	}
	for _, kind := range []string{"mem", "tcp"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			withGames(t, []string{s1, s1}, func(gs []*Game) error {
				var eps [2]netplay.Endpoint
				var drop func(i int)
				if kind == "mem" {
					mem := netplay.NewMem()
					eps[0], _ = mem.Create("G", "a")
					eps[1], _ = mem.Join("G", "b")
					drop = func(i int) { mem.Drop("G", eps[i].ID()) }
				} else {
					r, err := netplay.Listen("127.0.0.1:0", "G")
					if err != nil {
						t.Skip(err)
					}
					defer r.Close()
					eps[0] = r.Local()
					if eps[1], err = netplay.Dial(r.Addr().String(), "G", "b"); err != nil {
						return err
					}
					drop = func(i int) {
						if i == 0 {
							r.Close()
						} else {
							eps[1].Close()
						}
					}
				}
				for m, g := range gs {
					attach(g, eps[m], nil)
					g.net.dropEnd = true
					g.state.Players[0].NetID, g.state.Players[1].NetID = 1, 2
				}
				// Both up: Mox_Net_Update_ runs quietly.
				for _, g := range gs {
					if err := netRun(g, g.moxNetUpdate); err != nil {
						return err
					}
				}
				drop(1)
				exit := func(g *Game) error {
					for end := time.Now().Add(5 * time.Second); time.Now().Before(end); time.Sleep(time.Millisecond) {
						if err := netRun(g, g.moxNetUpdate); err != nil {
							return err
						}
					}
					return nil
				}
				var ee *ExitError
				if err := exit(gs[0]); !errors.As(err, &ee) || !strings.Contains(ee.Msg, gs[0].netWho(1)+" has lost the connection") {
					t.Fatalf("creator after the joiner dropped: %v", err)
				}
				return nil
			})
		})
	}
	// The relay leaving ends the joiner's game.
	t.Run("relay", func(t *testing.T) {
		t.Parallel()
		withGames(t, []string{s1}, func(gs []*Game) error {
			r, err := netplay.Listen("127.0.0.1:0", "G")
			if err != nil {
				t.Skip(err)
			}
			ep, err := netplay.Dial(r.Addr().String(), "G", "b")
			if err != nil {
				return err
			}
			g := gs[0]
			attach(g, ep, nil)
			g.state.Players[0].NetID, g.state.Players[1].NetID = 1, 2
			r.Close()
			var ee *ExitError
			for end := time.Now().Add(5 * time.Second); time.Now().Before(end); time.Sleep(time.Millisecond) {
				if err := netRun(g, g.moxNetUpdate); err != nil {
					if !errors.As(err, &ee) || !strings.Contains(ee.Msg, "who relays the game, is lost") {
						t.Fatal(err)
					}
					return nil
				}
			}
			t.Fatal("the joiner went on without the relay")
			return nil
		})
	})
}

// TestNetSinglePlayerUntouched: outside game type 2 and 3 Mox_Net_Update_
// does nothing, even with a station attached.
func TestNetSinglePlayerUntouched(t *testing.T) {
	t.Parallel()
	withGames(t, []string{""}, func(gs []*Game) error {
		g := gs[0]
		mem := netplay.NewMem()
		ep, _ := mem.Create("G", "a")
		var log []NetRecord
		attach(g, ep, &log)
		g.net.set8(netState, 4)
		for _, gt := range []int{0, 1} {
			g.gameType = gt
			for range 20 {
				g.e.MoxUpdate()
			}
			if len(log) != 0 || g.net.turn != 0 {
				t.Fatalf("game type %d: %d messages, round robin %d", gt, len(log), g.net.turn)
			}
		}
		return nil
	})
}

// TestNetLockstep runs two games on one virtual clock (host.Lockstep) over
// the in-memory transport. With the network state at 4 every Mox_Update_
// pumps Mox_Net_Update_, which sends msg 0x36 to each player once (itself
// included), one player per update in turn. Machine 0's messages reach machine 1 in
// the frame they are sent, machine 1's reach machine 0 one frame later,
// and three runs log the same frames.
func TestNetLockstep(t *testing.T) {
	t.Parallel()
	s1 := filepath.Join(savesDir(), "SAVE1.GAM")
	if !fileExists(s1) {
		t.Skip("no SAVE1.GAM")
	}
	var first []string
	for run := range 3 {
		got := netMachines(t, s1, 2, 12)
		if run == 0 {
			first = got
			want := []string{"0 S 0x36 to 1", "0 R 0x36 from 1", "1 S 0x36 to 1", "1 S 0x36 to 2", "1 R 0x36 from 2"}
			for _, w := range want {
				if !slices.ContainsFunc(got, func(s string) bool { return strings.HasSuffix(s, w) }) {
					t.Fatalf("no %q in\n%s", w, strings.Join(got, "\n"))
				}
			}
			checkArrivals(t, got)
		} else if !slices.Equal(got, first) {
			t.Fatalf("run %d differs:\n%s\nfirst:\n%s", run, strings.Join(got, "\n"), strings.Join(first, "\n"))
		}
	}
}

// checkArrivals: a message from machine i sent in frame f arrives at
// machine j in frame f when j > i and f+1 when j < i.
func checkArrivals(t *testing.T, got []string) {
	t.Helper()
	sent := map[string][]int{} // "from>to msg" → frames
	for _, l := range got {
		var f, m int
		var dir, msg, word string
		var peer int
		fmt.Sscanf(l, "f%d %d %s %s %s %d", &f, &m, &dir, &msg, &word, &peer)
		if dir == "S" && peer != m+1 {
			sent[fmt.Sprintf("%d>%d %s", m+1, peer, msg)] = append(sent[fmt.Sprintf("%d>%d %s", m+1, peer, msg)], f)
		}
	}
	for _, l := range got {
		var f, m int
		var dir, msg, word string
		var peer int
		fmt.Sscanf(l, "f%d %d %s %s %s %d", &f, &m, &dir, &msg, &word, &peer)
		if dir != "R" || peer == m+1 {
			continue
		}
		k := fmt.Sprintf("%d>%d %s", peer, m+1, msg)
		if len(sent[k]) == 0 {
			t.Fatalf("%s: never sent", l)
		}
		at := sent[k][0]
		sent[k] = sent[k][1:]
		if want := at + map[bool]int{true: 0, false: 1}[m+1 > peer]; f != want {
			t.Errorf("%s: sent in frame %d, want arrival in %d", l, at, want)
		}
	}
}

// netMachines plays n games from save on one Lockstep clock for frames
// frames over the in-memory transport, each machine a station whose net id
// is its player's (machine i plays player i, net id i+1), and returns the
// messages as "f<frame> <machine> S|R msg to|from <net id>" (port-only).
func netMachines(t *testing.T, save string, n, frames int) []string {
	t.Helper()
	clock := host.NewLockstep(n)
	mem := netplay.NewMem()
	var mu sync.Mutex
	var got []string
	var wg sync.WaitGroup
	errs := make([]error, n)
	eps := make([]netplay.Endpoint, n)
	for i := range n {
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
	// Every directory is made before any machine runs: a machine that
	// never starts would hold the others (or leave with clock.Leave).
	dirs := make([]string, n)
	for i := range dirs {
		dirs[i] = scratchGameDir(t, filepath.Join("..", "..", "game"))
		copyFile(t, save, filepath.Join(dirs[i], "SAVE1.GAM"))
	}
	for i, dir := range dirs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = host.Run(host.Options{Headless: true, Lockstep: clock, Machine: i, Frames: frames}, func(sys *host.System) error {
				g, err := newGame(sys, dir)
				if err != nil {
					return err
				}
				g.Log = func(string, ...any) {}
				if ok, err := g.loadGame(0); !ok || err != nil {
					return fmt.Errorf("load: %v", err)
				}
				attach(g, eps[i], nil)
				g.state.PlayerNum = int16(i)
				for k := range g.state.Players {
					g.state.Players[k].NetID = -1
				}
				for k := range n {
					g.state.Players[k].NetID = int16(k + 1)
					g.state.Players[k].Objective = 100
				}
				g.state.NumPlayers = int16(n)
				g.net.set8(netState, 4)
				g.net.log = func(r NetRecord) {
					w := map[uint16]string{'S': "to", 'R': "from"}[r.Dir]
					mu.Lock()
					got = append(got, fmt.Sprintf("f%d %d %c %#x %s %d", clock.Frame(), i, rune(r.Dir), r.ID, w, r.Peer))
					mu.Unlock()
				}
				for {
					g.e.MoxUpdate()
					if err := sys.Sleep(time.Second / 60); err != nil {
						return err
					}
				}
			})
		}()
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil && !errors.Is(err, host.ErrQuit) {
			t.Fatalf("machine %d: %v", i, err)
		}
	}
	return got
}
