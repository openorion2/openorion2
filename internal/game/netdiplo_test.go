package game

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/openorion2/openorion2/internal/host"
	"github.com/openorion2/openorion2/internal/save"
)

// Phase 5 of network play: diplomacy (docs/net/diplomacy.md).

// netDipScript drives two machines through a talk: machine 0 asks machine
// 1 for an audience on the races screen, machine 1 answers with reply
// (a accept, i refuse, w wait); machine 0 withdraws the request if cancel.
// While a machine talks, its lists take the keys of talk[m] in order (then
// Good Bye), while it listens answer[m]; then both play nt.turns turns.
type netDipScript struct {
	asker    int // the machine that asks (0 unless set)
	talk     [2][]byte
	answer   [2]byte
	reply    byte
	cancel   bool   // withdraw the first request when told to wait
	asks     int    // requests made in a row (1 if 0)
	replies  []byte // the replies to each request, in order (else reply)
	asked    int
	replied  int
	contact  [2]bool
	phase    [2]int
	idle     [2]int
	lists    [2]int
	talkDone [2]bool
	treaty   [2][3]uint8 // players 0 and 1's trade, research, status when the talk ended
	bc       [2][2]int32 // players 0 and 1's BC when the talk ended
	nt       netTurner
	log      func(m int, f string, a ...any)
}

func (ds *netDipScript) mainScreen(g *Game) bool { return netMainScreenUp(g) }

// done notes the end of the talk on machine m (player m) and what it left.
func (ds *netDipScript) done(m int, g *Game) {
	ds.talkDone[m] = true
	p, o := &g.state.Players[m], 1-m
	ds.treaty[m] = [3]uint8{p.TradeTreaty[o], p.ResearchTreaty[o], p.TreatyStatus[o]}
	ds.bc[m] = [2]int32{g.state.Players[0].BC, g.state.Players[1].BC}
}

func (ds *netDipScript) after(m int, g *Game, sys *host.System, games []*Game) bool {
	if ds.talkDone[0] && ds.talkDone[1] {
		return ds.nt.after(m, g, sys, games)
	}
	if g.state.Stardate == 0 || g.state.NumPlayers == 0 {
		return false
	}
	main := ds.mainScreen(g)
	if main && !ds.contact[m] {
		// Both humans have met (the same on both machines).
		g.state.Players[0].Contact[1], g.state.Players[1].Contact[0] = 1, 1
		ds.contact[m] = true
	}
	if ds.idle[m]++; ds.idle[m] < 10 {
		return false
	}
	key := func(what string, c byte) {
		if ds.log != nil {
			ds.log(m, "%s (%q)", what, c)
		}
		ds.idle[m] = 0
		if c == 0x1b {
			sys.PushKey(host.Key{Code: 0x1b, Scancode: 41})
		} else {
			sys.PushKey(letterKey(c))
		}
	}
	rs := &g.race
	switch {
	case m != ds.asker && g.ms.replyShowing:
		r := ds.reply
		if ds.replied < len(ds.replies) {
			r = ds.replies[ds.replied]
		}
		ds.replied++
		key("reply", r)
	case g.e.ListFieldOn():
		if int(g.dip.net.talker) != int(g.state.PlayerNum) {
			key("answer", ds.answer[m])
			return false
		}
		k := ds.lists[m]
		ds.lists[m]++
		if k < len(ds.talk[m]) {
			key(fmt.Sprintf("list %d", k), ds.talk[m][k])
			return false
		}
		key("list, good bye", '0')
	case m == ds.asker && rs.showing && rs.inWhoMode:
		key("who", '1')
	case m == ds.asker && rs.showing && ds.asked < max(ds.asks, 1):
		ds.phase[m] = 1
		ds.asked++
		key("audience", 'a')
	case m == ds.asker && rs.netState == 2 && ds.cancel && ds.asked == 1:
		key("withdraw", ' ')
	case m == ds.asker && rs.showing:
		key("leave races", 0x1b)
		ds.phase[m] = 2
	case main && m == ds.asker && ds.phase[m] == 0:
		if !ds.contact[1-m] {
			return false
		}
		key("races", 'r')
	case main && m == ds.asker && ds.phase[m] == 2:
		ds.done(m, g)
	case main && m != ds.asker:
		if g.net.u8(netDipReq) == 0 && g.net.u8(netState) == netStIdle && ds.lists[m] > 0 || ds.talkDone[1-m] {
			ds.done(m, g)
		}
	default:
		if ds.idle[m] > 60 {
			key("enter", '\r')
		}
	}
	return false
}

// netDipRun plays ds on two machines and checks every client against the
// host after each broadcast; it returns the games and their sends.
func netDipRun(t *testing.T, ds *netDipScript) ([]*Game, [][]NetRecord) {
	t.Helper()
	var mu sync.Mutex
	hostSnaps := map[uint32][]byte{}
	type snap struct {
		stardate uint32
		data     []byte
	}
	clientSnaps := make([][]snap, 2)
	if os.Getenv("OPENORION2_NET_DIPLOG") != "" {
		ds.log = func(m int, f string, a ...any) { t.Logf("machine %d: "+f, append([]any{m}, a...)...) }
		ds.nt.log = ds.log
	}
	games, logs := netGameRunN(t, 2, func(m int, g *Game, opt *Options) {
		if opt != nil {
			opt.ListKey, opt.DiploKeys = true, true
			opt.Seed = uint32(7100 + m)
			return
		}
		g.settings[2] = 0    // no random events
		g.settings[0xd3] = 0 // no Antarans
		g.net.turnHook = func(host bool) {
			b := g.encodeSave(9)
			mu.Lock()
			defer mu.Unlock()
			if host {
				hostSnaps[g.state.Stardate] = b
			} else {
				clientSnaps[m] = append(clientSnaps[m], snap{g.state.Stardate, b})
			}
		}
	}, ds.after)
	for m, g := range games {
		if g == nil || !ds.talkDone[m] || g.state.Stardate < 35000+uint32(ds.nt.turns) {
			for k, g := range games {
				t.Logf("machine %d: stardate %d, screen %#x, net state %#x, talk with %d, lists %d",
					k, g.state.Stardate, g.currentScreen, g.net.u8(netState), g.net.i8(netTalkTo), ds.lists[k])
			}
			t.Fatalf("machine %d stopped early", m)
		}
	}
	checked := 0
	for m, l := range clientSnaps {
		for _, c := range l {
			h, ok := hostSnaps[c.stardate]
			if !ok {
				t.Errorf("machine %d: no host state for stardate %d", m, c.stardate)
				continue
			}
			a, err := save.Parse(h)
			if err != nil {
				t.Fatal(err)
			}
			b, err := save.Parse(c.data)
			if err != nil {
				t.Fatal(err)
			}
			diffs, err := save.Diff(a, b)
			if err != nil {
				t.Fatal(err)
			}
			for _, d := range diffs {
				if !netSoakLocal(d) {
					t.Errorf("machine %d, stardate %d: %v", m, c.stardate, d)
				}
			}
			checked++
		}
	}
	if checked < ds.nt.turns {
		t.Errorf("%d client states checked, want %d", checked, ds.nt.turns)
	}
	return games, logs
}

// netDipSends lists a machine's diplomacy sends (ids 5..0xa, 0x1b..0x26,
// 0x44, 0x7d0 with its code, 0x834).
func netDipSends(l []NetRecord) []string {
	var out []string
	for _, r := range l {
		if r.Dir != 'S' {
			continue
		}
		switch {
		case r.ID == 0x7d0 && len(r.Payload) == 8:
			out = append(out, fmt.Sprintf("7d0:%#x", uint16(r.Payload[4])|uint16(r.Payload[5])<<8))
		case r.ID >= 5 && r.ID <= 0xa, r.ID >= 0x1b && r.ID <= 0x26 && r.ID != 0x22 || r.ID == 0x44 || r.ID == 0x834:
			out = append(out, fmt.Sprintf("%#x", r.ID))
		}
	}
	return out
}

// TestNetDiplomacyHumans: two humans' requests and talks on two machines.
// The sends of both (ids and dialogue codes) must come in the order the
// protocol gives, and every client equals the host after each broadcast.
func TestNetDiplomacyHumans(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name         string
		ds           netDipScript
		want0, want1 string
		treaty       bool
	}{
		// Machine 0 proposes the first treaty of its list, machine 1
		// accepts, machine 0 says good bye.
		{"treaty", netDipScript{talk: [2][]byte{{'1', '1'}}, answer: [2]byte{'1', '1'}, reply: 'a'},
			"0x1b 0x44 7d0:0x3f4 7d0:0x3e8 7d0:0x82", "0x1c 0x44 7d0:0x3f4 7d0:0x85", true},
		// The same, then machine 0 lets machine 1 talk (Wait), machine 1
		// chats a line and hands the talk back, machine 0 says good bye.
		{"switch-chat", netDipScript{talk: [2][]byte{{'1', '1', '9'}, {'8', '9'}}, answer: [2]byte{'1', '1'}, reply: 'a'},
			"0x1b 0x44 7d0:0x3f4 7d0:0x3e8 7d0:0x3f3 7d0:0x82", "0x1c 0x44 7d0:0x3f4 7d0:0x85 0x834 7d0:0x3f3", true},
		// Machine 0 gives a gift (the first of its list), then declares war.
		{"gift-war", netDipScript{talk: [2][]byte{{'4', '1', '6', '1'}}, answer: [2]byte{'1', '1'}, reply: 'a'},
			"0x1b 0x44 7d0:0x3f4 7d0:0x3ec 7d0:0x3ea 7d0:0x82", "0x1c 0x44 7d0:0x3f4", true},
		// A trade treaty, then machine 0 breaks it (code 0x3e9): no treaty
		// left on either machine.
		{"break", netDipScript{talk: [2][]byte{{'1', '1', '2', '1'}}, answer: [2]byte{'1', '1'}, reply: 'a'},
			"0x1b 0x44 7d0:0x3f4 7d0:0x3e8 7d0:0x3e9 7d0:0x82", "0x1c 0x44 7d0:0x3f4 7d0:0x85", false},
		// Machine 1 refuses: msg 0x1d.
		{"refuse", netDipScript{reply: 'i'}, "0x1b", "0x1d", false},
		// Wait and withdraw, then a refusal; machine 1 now ignores machine
		// 0, so its round robin refuses the third request by itself.
		{"withdraw-refuse-ignored", netDipScript{reply: 'w', cancel: true, asks: 3, replies: []byte{'w', 'i'}},
			"0x1b 0x1f 0x1b 0x1b", "0x26 0x1d 0x1d", false},
		// Machine 1 says wait (msg 0x26), machine 0 withdraws (msg 0x1f).
		{"cancel", netDipScript{reply: 'w', cancel: true}, "0x1b 0x1f", "0x26", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			ds := c.ds
			ds.nt = netTurner{turns: 2, firstRow: true}
			games, logs := netDipRun(t, &ds)
			for m, want := range []string{c.want0, c.want1} {
				got := strings.Join(netDipSends(logs[m]), " ")
				t.Logf("machine %d sends: %s", m, got)
				if got != want {
					t.Errorf("machine %d sends %q, want %q", m, got, want)
				}
			}
			// Each machine signed the treaty on its own (Start_Net_Treaty_ on
			// the listener, Diplomacy_Propose_Treaty_ on the talker).
			for m := range games {
				if signed := ds.treaty[m] != [3]uint8{}; signed != c.treaty {
					t.Errorf("machine %d: treaty after the talk %v, want one: %v", m, ds.treaty[m], c.treaty)
				}
			}
			if ds.bc[0] != ds.bc[1] {
				t.Errorf("BC after the talk: machine 0 %v, machine 1 %v", ds.bc[0], ds.bc[1])
			}
			if c.name == "gift-war" && (int8(ds.treaty[0][2]) < 4 || int8(ds.treaty[1][2]) < 4) {
				t.Errorf("no war after the talk: %v", ds.treaty)
			}
			if c.name == "refuse" && games[1].state.Players[1].Ignoring&1 == 0 {
				t.Errorf("machine 1 does not ignore machine 0 after refusing")
			}
		})
	}
}

// netDipAIScript: machine first asks the first race of its races screen
// (an AI) for an audience; once it talks, machine 1-first asks the same AI
// and waits in the arbiter's queue; first leaves (Esc) only then, and the
// other talks and leaves. Then both play nt.turns turns.
type netDipAIScript struct {
	first    int
	phase    [2]int // 0 main, 1 asked, 2 talking, 3 left the talk, 4 done
	idle     [2]int
	queued   bool // the second machine was seen in the queue (msg 7)
	talkDone [2]bool
	nt       netTurner
	log      func(m int, f string, a ...any)
}

func (ds *netDipAIScript) after(m int, g *Game, sys *host.System, games []*Game) bool {
	if ds.talkDone[0] && ds.talkDone[1] {
		return ds.nt.after(m, g, sys, games)
	}
	if g.state.Stardate == 0 || g.state.NumPlayers == 0 {
		return false
	}
	if ds.idle[m]++; ds.idle[m] < 10 {
		return false
	}
	key := func(what string, c byte) {
		if ds.log != nil {
			ds.log(m, "%s (%q)", what, c)
		}
		if c == 0x1b {
			sys.PushKey(host.Key{Code: 0x1b, Scancode: 41})
		} else {
			sys.PushKey(letterKey(c))
		}
		ds.idle[m] = 0
	}
	other := games[1-m]
	if other == nil {
		return false
	}
	if g.net.u8(netState) == netStAIQueued {
		ds.queued = true
	}
	rs := &g.race
	main := (&netDipScript{}).mainScreen(g)
	switch {
	case g.e.ListFieldOn():
		ds.phase[m] = 2
		if m == ds.first && !ds.queued {
			return false // until the other waits in the queue
		}
		ds.phase[m] = 3
		key("good bye", '8')
	case rs.showing && rs.inWhoMode:
		ds.phase[m] = 1
		key("who", '1')
	case rs.showing && ds.phase[m] == 0:
		key("audience", 'a')
	case rs.showing && ds.phase[m] == 3:
		ds.phase[m] = 4
		key("leave races", 0x1b)
	case main && ds.phase[m] == 0:
		if m != ds.first && other.net.u8(netState) != netStAITalk {
			return false
		}
		key("races", 'r')
	case main && ds.phase[m] == 4:
		ds.talkDone[m] = true
	case rs.netState != 0:
		// Waiting for the arbiter: any input would withdraw the request.
	default:
		if ds.idle[m] > 60 {
			key("enter", '\r')
		}
	}
	return false
}

// TestNetDiplomacyAIQueue: two humans ask the same AI for an audience on
// the races screen; the second waits in the arbiter's queue (msg 7) until
// the first leaves (msg 0xa), then talks. Every client equals the host
// after the next broadcasts.
func TestNetDiplomacyAIQueue(t *testing.T) {
	t.Parallel()
	ds := &netDipAIScript{first: 1, nt: netTurner{turns: 2, firstRow: true}}
	if os.Getenv("OPENORION2_NET_DIPLOG") != "" {
		ds.log = func(m int, f string, a ...any) { t.Logf("machine %d: "+f, append([]any{m}, a...)...) }
		ds.nt.log = ds.log
	}
	var met [2]bool
	cases := map[uint32]int{}
	var mu sync.Mutex
	games, logs := netGameRunN(t, 2, func(m int, g *Game, opt *Options) {
		if opt != nil {
			opt.ListKey, opt.DiploKeys = true, true
			opt.Seed = uint32(7200 + m)
			return
		}
		g.settings[0xd5] = 3 // two humans and an AI
		g.settings[2], g.settings[0xd3] = 0, 0
		g.net.caseHook = func(c uint32) {
			mu.Lock()
			cases[c]++
			mu.Unlock()
		}
	}, func(m int, g *Game, sys *host.System, games []*Game) bool {
		if !met[m] && g.state.NumPlayers == 3 && (&netDipScript{}).mainScreen(g) {
			// Both humans have met the AI (the same on both machines).
			for _, h := range []int{0, 1} {
				g.state.Players[h].Contact[2], g.state.Players[2].Contact[h] = 1, 1
			}
			met[m] = true
		}
		if !met[0] || !met[1] {
			// The home star's name: Enter.
			if ds.idle[m]++; ds.idle[m] > 60 && g.state.Stardate > 0 && !met[m] {
				sys.PushKey(letterKey('\r'))
				ds.idle[m] = 0
			}
			return false
		}
		return ds.after(m, g, sys, games)
	})
	for m, g := range games {
		if g == nil || !ds.talkDone[m] {
			t.Fatalf("machine %d stopped early (phases %v)", m, ds.phase)
		}
		t.Logf("machine %d sends: %s", m, strings.Join(netDipSends(logs[m]), " "))
	}
	t.Logf("msg 5: %d, msg 6: %d, msg 7: %d, msg 0xa: %d", cases[0xf5d3d], cases[0xf5d9a], cases[0xf5dbc], cases[0xf5e55])
	if cases[0xf5d3d] != 2 || cases[0xf5dbc] != 1 || cases[0xf5d9a] != 2 || cases[0xf5e55] != 2 {
		t.Errorf("want two talks, one queued")
	}
}
