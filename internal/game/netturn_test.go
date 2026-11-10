package game

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"net"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/openorion2/openorion2/internal/host"
	"github.com/openorion2/openorion2/internal/netplay"
	"github.com/openorion2/openorion2/internal/save"
)

// Phase 3 of network play: the turn (docs/net/turn.md).

// netTurner answers each machine's screens for turns turns after the
// start: Enter where nothing else fits, key 1 for the research choice
// (ListKey), Esc on the colony screen, Turn on the main screen (machine 0
// first, the others once machine 0 waits).
type netTurner struct {
	turns    int
	start    [8]uint32
	turned   [8]uint32 // the stardate Turn was last pressed at, plus one
	idle     [8]int
	end      [8]int
	tries    [8]int
	firstRow bool // the research choice is the first row
	eager    bool // the others press Turn as soon as the host's vote came
	log      func(m int, f string, a ...any)
	hostFor  func(sd uint32) int // the machine that presses Turn first (0 if nil)
}

// netMainScreenUp reports whether the main screen's own input loop runs:
// its TURN field in the corner (an ambassador's screen over it has other
// fields). The current screen may still say races (6) there after a talk:
// Diplomacy_Generic_System_List_ sets it and nothing restores it.
func netMainScreenUp(g *Game) bool {
	if g.currentScreen != ScreenMain && (g.currentScreen != ScreenRaces || g.race.showing) {
		return false
	}
	if g.ms.fields.next <= 0 || g.e.FieldsCount() <= g.ms.fields.next {
		return false
	}
	x1, y1, _, _ := g.e.FieldRect(g.ms.fields.next)
	return x1 > 500 && y1 > 400
}

func (nt *netTurner) after(m int, g *Game, sys *host.System, games []*Game) bool {
	if g.net.u8(netState) != 4 && g.gameType == 2 && g.net.chat == nil && nt.start[m] == 0 {
		return false
	}
	if nt.start[m] == 0 {
		nt.start[m] = g.state.Stardate
	}
	main := netMainScreenUp(g)
	if main && g.state.Stardate >= nt.start[m]+uint32(nt.turns) {
		// A while longer for the round robin's msgs 0x36.
		nt.end[m]++
		return nt.end[m] > 200
	}
	if g.ts.showing && nt.firstRow {
		// The research choice is the row under the pointer: the first
		// row of the first box (as TestTurnBt1).
		x := int(g.exeI16(techSelectPos))
		y := int(g.exeI16(techSelectPos+2) + g.exeI16(techFieldY1) + 0x15)
		sys.WarpMouse(x+0x6d, y+4)
	}
	// The combat screen asks for our ship's order: AUTO, at once (a press read
	// after the ship changed would cancel the other side's AUTO).
	if tc := g.tc; tc != nil && tc.scr != nil && tc.scr.humanInput && tc.autoCombat == 0 && g.e.FieldsCount() > tc.scr.autoField {
		x1, y1, x2, y2 := g.e.FieldRect(tc.scr.autoField)
		sys.Press((x1+x2)/2, (y1+y2)/2, 1)
		nt.idle[m] = 0
		return false
	}
	if nt.idle[m]++; nt.idle[m] < 8 {
		return false
	}
	act := func(what string, k host.Key) {
		if nt.log != nil {
			nt.log(m, "sd %d: %s", g.state.Stardate, what)
		}
		if d := os.Getenv("OPENORION2_NET_SHOTS"); d != "" {
			g.e.SaveScreenPNG(filepath.Join(d, fmt.Sprintf("m%d-%d-%s.png", m, g.state.Stardate, what)))
		}
		sys.PushKey(k)
		nt.idle[m] = 0
	}
	st := g.net.u8(netState)
	switch {
	case g.net.chat != nil && (st < 0x10 || st > 0x1c):
		nt.idle[m] = 0 // waiting for the others
	case g.ts.showing:
		act("research", letterKey('1'))
	case g.currentScreen == ScreenColony:
		act("colony", host.Key{Code: 0x1b, Scancode: 41})
	case main:
		if nt.turned[m] == g.state.Stardate+1 {
			if nt.idle[m] < 300 {
				return false
			}
			// TURN went to a screen over the map (an AI's ambassador).
			nt.turned[m] = 0
		}
		first := 0
		if nt.hostFor != nil {
			first = nt.hostFor(g.state.Stardate)
		}
		if m != first {
			// The others press Turn once the first machine waits (it hosts).
			g0 := games[first]
			if g0 == nil || g0.net.chat == nil {
				return false
			}
			// and its own round robin has sent its vote back (not ready yet);
			// eager: once the host's vote is in, before it is sent back.
			me := uint32(g.state.PlayerNum)
			v := g.net.i8(netVote + me)
			if v == -1 || !nt.eager && g.net.i8(netSentVote+uint32(g0.state.PlayerNum)) != v {
				return false
			}
		}
		nt.turned[m] = g.state.Stardate + 1
		act("turn", letterKey('t'))
	default:
		// Something else waits: Enter, then Esc, then a click on its
		// first field (a popup's first button).
		if nt.idle[m] > 40 {
			switch nt.tries[m] % 3 {
			case 0:
				act(fmt.Sprintf("enter-screen%#x", g.currentScreen), letterKey('\r'))
			case 1:
				act(fmt.Sprintf("esc-screen%#x", g.currentScreen), host.Key{Code: 0x1b, Scancode: 41})
			case 2:
				if g.e.FieldsCount() > 1 {
					x1, y1, x2, y2 := g.e.FieldRect(1)
					sys.Press((x1+x2)/2, (y1+y2)/2, 1)
				}
				nt.idle[m] = 0
			}
			nt.tries[m]++
		}
	}
	return false
}

// TestNetTurns: two machines start a game and play three turns.
func TestNetTurns(t *testing.T) {
	t.Parallel()
	nt := &netTurner{turns: 3, log: func(m int, f string, a ...any) { t.Logf("machine %d: "+f, append([]any{m}, a...)...) }}
	games, logs := netGameRun(t, func(m int, g *Game, opt *Options) {
		if opt != nil {
			opt.ListKey = true
		}
	}, func(m int, g *Game, sys *host.System, games []*Game) bool {
		return nt.after(m, g, sys, games)
	})
	for m, g := range games {
		t.Logf("machine %d: stardate %d", m, g.state.Stardate)
		t.Logf("machine %d sends: %v", m, lobbySends(logs[m]))
	}
}

// TestNetSoak: n machines (2 and 3 humans; 2 humans and 2 AIs) play many turns over the
// in-memory transport. After every broadcast each client's state must be
// the host's, but for what each viewer keeps (netSoakLocal).
func TestNetSoak(t *testing.T) {
	t.Parallel()
	turns := 40
	if s := os.Getenv("OPENORION2_NET_SOAK"); s != "" {
		fmt.Sscan(s, &turns)
	}
	if testing.Short() {
		turns = 6
	}
	for _, c := range []struct{ n, ais, battles int }{{2, 0, 0}, {3, 0, 0}, {2, 2, 0}, {2, 2, netBattlesStrategic}, {3, 1, netBattlesStrategic}, {2, 2, netBattlesTactical}, {2, 0, netBattlesHumans}, {2, 0, netBattlesHumansScreen}, {2, 3, netSoakDiplo}, {3, 2, netSoakDiplo},
		{3, 1, netSoakQuit}, {2, 1, netSoakQuit}, {3, 0, netSoakSurrender}, {3, 1, netSoakEliminate}, {2, 1, netSoakEliminate},
		{3, 1, netSoakDropMain}, {2, 1, netSoakDropMain}, {3, 0, netSoakDropVote}, {2, 1, netSoakDropVote}, {3, 0, netSoakDropUpload}, {2, 1, netSoakDropUpload},
		{3, 1, netSoakDropHost}, {2, 1, netSoakDropHost}, {3, 1, netSoakCreatorQuit}, {2, 1, netSoakCreatorQuit}, {3, 1, netSoakEliminateCreator}, {2, 1, netSoakEliminateCreator},
		{2, 6, netBattlesNested}} {
		name := fmt.Sprintf("%d+%dai", c.n, c.ais)
		switch c.battles {
		case netBattlesStrategic:
			name += "-battles"
		case netBattlesTactical:
			name += "-tactical"
		case netBattlesNested:
			name += "-nested"
		case netBattlesHumans:
			name += "-lockstep"
		case netBattlesHumansScreen:
			name += "-lockstep-screen"
		case netSoakDiplo:
			name += "-diplo"
		case netSoakQuit:
			name += "-quit"
		case netSoakSurrender:
			name += "-surrender"
		case netSoakEliminate:
			name += "-eliminated"
		case netSoakDropMain:
			name += "-drop-main"
		case netSoakDropVote:
			name += "-drop-vote"
		case netSoakDropUpload:
			name += "-drop-upload"
		case netSoakDropHost:
			name += "-drop-host"
		case netSoakCreatorQuit:
			name += "-creator-quit"
		case netSoakEliminateCreator:
			name += "-creator-eliminated"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			netSoak(t, c.n, c.ais, turns, c.battles)
		})
	}
}

// The soak's battles: strategic combat (any two players), or tactical
// combat against AIs only (two humans' lockstep battle is not ported).
const (
	netBattlesStrategic = 1
	netBattlesTactical  = 2
	netBattlesHumans    = 3 // tactical between humans: the lockstep
	// the lockstep on the combat screen, AUTO clicked
	netBattlesHumansScreen = 4
	// no battles: everybody has met, so AIs send their ambassadors
	// through the arbiter (docs/net/diplomacy.md)
	netSoakDiplo = 5
	// the last joiner quits through the game menu on turn 3 (msg 0x37)
	netSoakQuit = 6
	// the last joiner has surrendered and presses Turn on turn 3 (msg 0x38)
	netSoakSurrender = 7
	// the host's turn on turn 3 eliminates the last joiner (msg 0x3b):
	// Eliminate_Player_ before Next_Turn_, as the battles are set up
	netSoakEliminate = 8
	// port-only (hardening.md): on turn 3 the last joiner is lost on the main
	// screen, after pressing Turn, or when the host asks for his upload
	netSoakDropMain   = 9
	netSoakDropVote   = 10
	netSoakDropUpload = 11
	// machine 1 presses Turn first on turn 3, hosts it, and is lost while
	// taking an upload: the others start the turn over
	netSoakDropHost = 12
	// the creator quits through the game menu on turn 3: the relay stays
	netSoakCreatorQuit = 13
	// the creator hosts turn 3 and it eliminates him (his own msg 0x3b is
	// not sent: he leaves on the broadcast); the others play on
	netSoakEliminateCreator = 14
	// tactical: the humans attack an AI's last star, the AIs each other's
	// first: AI battles come up while the host fights (netSoakNested)
	netBattlesNested = 15
)

// netSoakDrop is netSoakLeave for the drop modes, on machine m: true once
// the leaver's link is to be cut now.
func netSoakDrop(m, leaver int, g *Game, mode int, at uint32) bool {
	n := &g.net
	main := g.currentScreen == ScreenMain && g.ms.fields.next > 0 && g.e.FieldsCount() > g.ms.fields.next
	if mode == netSoakDropUpload {
		// On the host: it has just asked him for his upload (msg 0xc).
		return m == 0 && g.state.Stardate == at && n.u8(netState) == 0xa && int(n.i8(netUploader)) == leaver
	}
	if m != leaver {
		return false
	}
	switch mode {
	case netSoakDropMain:
		return main && g.state.Stardate >= at && n.chat == nil
	case netSoakDropVote:
		return g.state.Stardate == at && n.chat != nil && n.u8(netState) == 8
	case netSoakDropHost:
		return g.state.Stardate == at && n.chat != nil && n.i16(netHostID) == n.i16(netOwnID) && n.u8(netState) == 0xa
	}
	return false
}

// netSoakLeaver is the machine that leaves in a leave or drop mode.
func netSoakLeaver(n, mode int) int {
	switch mode {
	case netSoakDropHost:
		return 1
	case netSoakCreatorQuit, netSoakEliminateCreator:
		return 0
	}
	return n - 1
}

// netSoakLeaveAt is the stardate the leaver of netSoakQuit and
// netSoakSurrender leaves at.
const netSoakLeaveAt = 35002

// netSoakLeave drives the leaver's machine: on the main screen at stardate at
// the game menu's Quit and Yes (quit), or Turn after giving up to player 0
// (surrender); then it stops once at the main menu. True: the hook is done.
func netSoakLeave(g *Game, sys *host.System, mode int, at uint32, step *int, idle *int) (bool, bool) {
	if *idle++; *idle < 8 {
		return true, false
	}
	click := func(f int) {
		x1, y1, x2, y2 := g.e.FieldRect(f)
		sys.Press((x1+x2)/2, (y1+y2)/2, 1)
		*idle = 0
		*step++
	}
	press := func(c byte) {
		sys.PushKey(letterKey(c))
		*idle = 0
		*step++
	}
	main := g.currentScreen == ScreenMain && g.ms.fields.next > 0 && g.e.FieldsCount() > g.ms.fields.next
	if mode == netSoakEliminate {
		// He plays on until msg 0x3b sends him to the main menu.
		*step = 3
		return g.currentScreen == ScreenMainMenu, g.currentScreen == ScreenMainMenu
	}
	switch {
	case *step == 0:
		if !main || g.state.Stardate < at || g.net.chat != nil {
			return false, false
		}
		if mode == netSoakSurrender {
			g.state.Players[g.state.PlayerNum].SurrenderTo = 0
			click(g.ms.fields.next)
			*step = 3
			return true, false
		}
		// GAME, QUIT and Yes by their hot keys, as the oracle (netquit1).
		press('g')
	case *step == 1:
		if main && *idle > 60 {
			*step = 0 // GAME was read by a screen before the main loop's: again
		}
		if g.currentScreen == ScreenGamePopup && g.gp.quitBtn > 0 && g.e.FieldsCount() > g.gp.quitBtn {
			press('q')
		}
	case *step == 2:
		if g.ub.confirming {
			sys.PushKey(letterKey('y'))
			*idle = 0
			*step++
		}
	case *step == 3:
		if g.currentScreen == ScreenMainMenu {
			return true, true
		}
	}
	return true, false
}

// netSoakUI attacks the first target Collect_Targets_For_Combat_ found;
// with screen the tactical battle runs on the combat screen.
type netSoakUI struct {
	g      *Game
	screen bool
}

func (u netSoakUI) tacticalOnScreen() bool { return u.screen }

func (u netSoakUI) selectTarget(star, p int16) int16 {
	t := &u.g.ct
	if t.nColT > 0 {
		return t.colT[0] + 15
	}
	if t.nShipT > 0 {
		return t.shipT[0]
	}
	return -1
}
func (u netSoakUI) confirmAttack(p, q int16) bool   { return true }
func (u netSoakUI) dragonBribe(p, star int16) int16 { return -1 }

// netSoakBattles moves, every other turn on the host before Next_Turn_, one
// warship of each human to another player's colony star, and every fourth
// turn one of each AI (at war) to the first human's: battles between humans
// (strategic only), humans against AIs and AIs against humans.
func netSoakBattles(g *Game, battles int) {
	s := &g.state
	if s.Stardate%2 != 0 {
		return
	}
	np := int(s.NumPlayers)
	for p := 0; p < np; p++ {
		if s.Players[p].Objective != save.ObjectiveHuman && s.Stardate%4 != 0 {
			continue
		}
		q := (p + 1 + int(s.Stardate/2)%max(np-1, 1)) % np
		ai := s.Players[p].Objective != save.ObjectiveHuman
		if ai {
			// An AI attacks a human, at war (Player_May_Attack_Player_).
			for q = 0; q < np && s.Players[q].Objective != save.ObjectiveHuman; q++ {
			}
			s.Players[p].TreatyStatus[q] = 5
		}
		if q == p || q == np || !ai && battles == netBattlesTactical && s.Players[q].Objective == save.ObjectiveHuman {
			continue
		}
		star := int16(-1)
		for i := 0; i < int(s.NumColonies); i++ {
			if c := &s.Colonies[i]; int(c.Owner) == q && c.Planet != -1 {
				star = int16(s.Planets[c.Planet].Star)
				break
			}
		}
		if star == -1 {
			continue
		}
		for i := 0; i < int(s.NumShips); i++ {
			sh := &s.Ships[i]
			if int(sh.Owner) == p && sh.Design.Type == 0 && sh.Status == save.ShipAtStar && sh.Location != star {
				sh.Location, sh.X, sh.Y = star, s.Stars[star].X, s.Stars[star].Y
				break
			}
		}
	}
}

// netSoakNested moves, every other turn on the host before Next_Turn_, a
// warship of the host's player to the AIs' last colony star, and one of the
// other human and of each other AI (at war) to the first AI's first. Stars
// are taken from the last: the host fights first; the AIs' battles at the
// other star wait for the human's there, which ends while the host still
// fights (community patch 1.51: they wait for the host's too).
func netSoakNested(g *Game) {
	s := &g.state
	if s.Stardate%2 != 0 {
		return
	}
	np := int(s.NumPlayers)
	ai := func(p int) bool { return s.Players[p].Objective != save.ObjectiveHuman }
	colonyStar := func(q int, last bool) (star, planet int16) {
		star = -1
		for i := 0; i < int(s.NumColonies); i++ {
			if c := &s.Colonies[i]; int(c.Owner) == q && c.Planet != -1 {
				if st := int16(s.Planets[c.Planet].Star); star == -1 || last == (st > star) {
					star, planet = st, c.Planet
				}
			}
		}
		return
	}
	move := func(p int, star int16) {
		for i := 0; i < int(s.NumShips); i++ {
			sh := &s.Ships[i]
			if int(sh.Owner) == p && sh.Design.Type == 0 && sh.Status == save.ShipAtStar && sh.Location != star {
				sh.Location, sh.X, sh.Y = star, s.Stars[star].X, s.Stars[star].Y
				return
			}
		}
	}
	// x: the AIs' last colony star; y: another AI's (owner's) first below it.
	x, xo, y, owner, planet := int16(-1), -1, int16(-1), -1, int16(-1)
	for q := 0; q < np; q++ {
		if st, _ := colonyStar(q, true); ai(q) && st > x {
			x, xo = st, q
		}
	}
	for q := 0; q < np; q++ {
		if st, pl := colonyStar(q, false); ai(q) && q != xo && st != -1 && st < x && st > y {
			y, owner, planet = st, q, pl
		}
	}
	if y == -1 {
		return
	}
	for p := 0; p < np; p++ {
		switch {
		case p == int(s.PlayerNum):
			move(p, x)
		case p != owner:
			if ai(p) {
				// At war, and the colony there is p's war target (Get_AI_Target_).
				P := &s.Players[p]
				P.TreatyStatus[owner], s.Players[owner].TreatyStatus[p] = 5, 5
				P.AIWarTarget, P.Diplo7C9[1] = planet, uint8(owner)
			}
			move(p, y)
		}
	}
}

// netSoakContacts makes, on the host before Next_Turn_, every player know
// every other at good relations: the AIs propose treaties to the humans.
func netSoakContacts(g *Game) {
	s := &g.state
	for p := 0; p < int(s.NumPlayers); p++ {
		for q := 0; q < int(s.NumPlayers); q++ {
			if p != q {
				s.Players[p].Contact[q] = 1
				s.Players[p].Relations[q] = max(s.Players[p].Relations[q], 80)
			}
		}
	}
}

func netSoak(t *testing.T, n, ais, turns, battles int) {
	t.Helper()
	type snap struct {
		stardate uint32
		data     []byte
	}
	var mu sync.Mutex
	hostSnaps := map[uint32][]byte{}
	notPorted := map[string]int{}
	cases := map[uint32]int{}
	barriers := make([][]string, n)
	mems := make([][][]byte, n)
	lastTC := make([]*tactical, n)
	inBattle := make([]int, n)
	resultsChecked := 0
	var live []*Game
	var clientSnaps [][]snap = make([][]snap, n)
	nt := &netTurner{turns: turns, firstRow: true}
	if os.Getenv("OPENORION2_NET_SOAKLOG") != "" {
		nt.log = func(m int, f string, a ...any) { t.Logf("machine %d: "+f, append([]any{m}, a...)...) }
	}
	var games []*Game
	elim := battles == netSoakEliminate || battles == netSoakEliminateCreator
	leave := battles == netSoakQuit || battles == netSoakSurrender || elim || battles >= netSoakDropMain && battles <= netSoakEliminateCreator
	drop := battles >= netSoakDropMain && battles <= netSoakDropHost
	leaver := netSoakLeaver(n, battles)
	leaveStep, leaveIdle := 0, 0
	dropped, dropOuts, mismatches := false, 0, 0
	mem := netplay.NewMem()
	switch battles {
	case netSoakDropHost:
		nt.hostFor = func(sd uint32) int { return map[bool]int{true: 1, false: 0}[sd == netSoakLeaveAt] }
	case netSoakCreatorQuit:
		nt.hostFor = func(sd uint32) int { return map[bool]int{true: 1, false: 0}[sd >= netSoakLeaveAt] }
	case netSoakEliminateCreator:
		nt.hostFor = func(sd uint32) int { return map[bool]int{true: 1, false: 0}[sd > netSoakLeaveAt] }
	}
	var errs []error
	games, _, errs = netGameRunNE(t, n, func(m int, g *Game, opt *Options) {
		if opt != nil {
			opt.ListKey = true
			opt.Seed = uint32(7000 + m)
			if battles == netBattlesHumans || battles == netBattlesHumansScreen {
				// 7000's first lockstep battle ends in the original's deadlock
				// after an AI boarding (docs/net/combat.md, "The lockstep").
				opt.Seed = uint32(7100 + m)
			}
			opt.Net = mem
			opt.NetCheck = true // port-only: the desync check stays silent
			return
		}
		g.net.checkHook = func(sd uint32, from uint16, same bool) {
			if !same {
				mu.Lock()
				mismatches++
				mu.Unlock()
				t.Errorf("machine %d: the desync check fired at stardate %d against net id %d", m, sd, from)
			}
		}
		g.net.dropHook = func(p int) {
			mu.Lock()
			dropOuts++
			mu.Unlock()
		}
		g.net.resume.off = drop          // the drop-outs at once (TestNetResume: the return)
		g.settings[0xd5] = byte(n + ais) // players
		g.settings[2] = 0                // no random events
		g.settings[0xd3] = 0             // no Antarans
		if elim {
			g.net.preTurnHook = func() {
				if g.state.Stardate == netSoakLeaveAt {
					g.eliminatePlayer(leaver)
				}
			}
		}
		if leave {
			g.net.caseHook = func(c uint32) {
				mu.Lock()
				cases[c]++
				mu.Unlock()
			}
		} else if battles == netSoakDiplo {
			g.net.preTurnHook = func() { netSoakContacts(g) }
			g.net.caseHook = func(c uint32) {
				mu.Lock()
				cases[c]++
				mu.Unlock()
			}
		} else if battles != 0 {
			g.settings[0xd8] = byte(b2i(battles == netBattlesStrategic))
			g.net.tacHook = func() {
				// Each barrier of the lockstep: the engine's records and the seed.
				// The seed is shared from the first action of a battle on (its
				// first two barriers come before any).
				h := sha256.Sum256(g.tc.mem)
				mu.Lock()
				if g.tc != lastTC[m] {
					lastTC[m], inBattle[m] = g.tc, 0
				}
				inBattle[m]++
				seed := fmt.Sprintf("%08x", g.state.Random.Get())
				if inBattle[m] <= 2 {
					seed = "-"
				}
				barriers[m] = append(barriers[m], fmt.Sprintf("%x %s", h[:8], seed))
				mems[m] = append(mems[m], slices.Clone(g.tc.mem))
				mu.Unlock()
			}
			g.net.preTurnHook = func() { netSoakBattles(g, battles) }
			if battles == netBattlesNested {
				g.net.preTurnHook = func() { netSoakNested(g) }
			}
			g.combatUI = netSoakUI{g, battles == netBattlesHumansScreen || battles == netBattlesNested && m == 0}
			g.net.battleHook = func(sender, att, def, star, planet int) {
				// The host's ships and colony there now equal the fighting machine's.
				var f *Game
				mu.Lock()
				all := live
				mu.Unlock()
				for _, o := range all {
					if o != nil && o != g && int(o.state.PlayerNum) == sender {
						f = o
					}
				}
				if f == nil {
					return
				}
				mu.Lock()
				defer mu.Unlock()
				resultsChecked++
				for i := 0; i < int(f.state.NumShips); i++ {
					sh := &f.state.Ships[i]
					if o := int(sh.Owner); (o == att || o == def) && int(sh.Location) == star && sh.Status != 5 &&
						!slices.Equal(recBytes(sh), recBytes(&g.state.Ships[i])) {
						t.Errorf("stardate %d: ship %d differs on the host after the results", g.state.Stardate, i)
					}
				}
				if planet != -1 {
					if c := f.state.Planets[planet].Colony; c != -1 && !slices.Equal(recBytes(&f.state.Colonies[c]), recBytes(&g.state.Colonies[c])) {
						t.Errorf("stardate %d: colony %d differs on the host after the results", g.state.Stardate, c)
					}
				}
			}
			g.net.caseHook = func(c uint32) {
				mu.Lock()
				cases[c]++
				mu.Unlock()
			}
		}
		g.Log = func(f string, a ...any) {
			mu.Lock()
			notPorted[fmt.Sprintf(f, a...)]++
			mu.Unlock()
		}
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
	}, func(m int, g *Game, sys *host.System, games []*Game) bool {
		mu.Lock()
		live = games
		mu.Unlock()
		if drop && !dropped && netSoakDrop(m, leaver, g, battles, netSoakLeaveAt) {
			// The link is cut under the leaver's machine, which ends.
			dropped = true
			if ep := games[leaver].net.ep; ep != nil {
				mem.Drop(ep.Game().Name, ep.ID())
			}
			if m == leaver {
				return true
			}
		}
		if leave && !drop && m == leaver {
			mode := battles
			switch mode {
			case netSoakCreatorQuit:
				mode = netSoakQuit
			case netSoakEliminateCreator:
				mode = netSoakEliminate
			}
			if handled, stop := netSoakLeave(g, sys, mode, netSoakLeaveAt, &leaveStep, &leaveIdle); handled {
				return stop
			}
		}
		return nt.after(m, g, sys, games)
	})
	for m, err := range errs {
		if err != nil && !errors.Is(err, host.ErrQuit) && !(drop && m == leaver) {
			t.Fatalf("machine %d: %v", m, err)
		}
	}
	for m, g := range games {
		if leave && m == leaver {
			// The lost machine itself ends on its lost link.
			var ee *ExitError
			if drop && errs[m] != nil && !errors.Is(errs[m], host.ErrQuit) && (!errors.As(errs[m], &ee) || !ee.Notice && ee.Msg != "net send error!") {
				t.Errorf("the lost machine: %v", errs[m])
			}
			if drop && (g == nil || g.state.Stardate != netSoakLeaveAt || !dropped) {
				t.Errorf("the leaver was not dropped at stardate %d", netSoakLeaveAt)
			} else if at := uint32(netSoakLeaveAt) + uint32(b2i(battles == netSoakEliminateCreator)); !drop && (g == nil || g.state.Stardate != at || leaveStep < 3) {
				t.Errorf("the leaver did not leave at stardate %d (step %d)", netSoakLeaveAt, leaveStep)
			}
			continue
		}
		if g == nil || g.state.Stardate < 35000+uint32(turns) {
			for k, g := range games {
				t.Logf("machine %d: stardate %d, screen %#x, net state %#x, chat %v, fields %d, votes % x, ready % x, host %d",
					k, g.state.Stardate, g.currentScreen, g.net.u8(netState), g.net.chat != nil, g.e.FieldsCount(),
					g.net.mem[netVote-netBase:netVote-netBase+4], g.net.mem[netReady-netBase:netReady-netBase+4], g.net.i16(netHostID))
			}
			t.Fatalf("machine %d stopped early", m)
		}
	}
	for msg, k := range notPorted {
		t.Logf("%d× %s", k, msg)
	}
	if leave {
		// Every other machine took the drop-out (msgs 0x37/0x38, case
		// 0xf662b) and plays on with the leaver as an AI.
		t.Logf("drop-outs decoded: %d (sent for a lost player: %d), eliminations: %d", cases[0xf662b], dropOuts, cases[0xf66a7])
		want := map[bool]int{true: 0, false: n - 1}[elim]
		if drop {
			// One machine sends the lost player's drop-out and applies it
			// without msg 0x37; the others decode it as his msg 0x37.
			want = n - 2
			if dropOuts != n-1 {
				t.Errorf("%d drop-outs applied for the lost player, want %d", dropOuts, n-1)
			}
		}
		if cases[0xf662b] != want {
			t.Errorf("%d drop-outs decoded, want %d", cases[0xf662b], want)
		}
		if want := int(b2i(battles == netSoakEliminate)); cases[0xf66a7] != want {
			t.Errorf("%d msgs 0x3b, want %d", cases[0xf66a7], want)
		}
		for m, g := range games {
			if m == leaver {
				continue
			}
			pl := &g.state.Players[leaver]
			if pl.Objective == save.ObjectiveHuman || pl.NetID != -1 {
				t.Errorf("machine %d: the leaver is objective %d, net id %d", m, pl.Objective, pl.NetID)
			}
			if (battles == netSoakSurrender || elim) && g.state.Players[leaver].Eliminated == 0 {
				t.Errorf("machine %d: the leaver is not eliminated", m)
			}
			switch {
			case battles == netSoakDropHost && g.net.restartAt != netSoakLeaveAt:
				t.Errorf("machine %d: the turn did not start over (at %d)", m, g.net.restartAt)
			case battles == netSoakDropUpload && m == 0 && g.net.upLost != 1:
				t.Errorf("machine %d: %d uploads given up, want 1", m, g.net.upLost)
			}
			if want := map[bool]int{true: 0, false: 2}[n == 2]; g.gameType != want {
				t.Errorf("machine %d: game type %d, want %d", m, g.gameType, want)
			}
		}
	} else if battles == netSoakDiplo {
		// The arbiter's cases: msg 5 (asked), 0xa (talk over); msg 0x22 on
		// the other humans; msgs 6 and 7 on the askers.
		t.Logf("msg 5: %d, msg 6: %d, msg 7: %d, msg 8: %d, msg 0xa: %d, msg 0x22: %d",
			cases[0xf5d3d], cases[0xf5d9a], cases[0xf5dbc], cases[0xf5dd5], cases[0xf5e55], cases[0xf63a2])
		if cases[0xf5e55] != cases[0xf5d3d] {
			t.Errorf("%d talks asked for, %d ended", cases[0xf5d3d], cases[0xf5e55])
		}
		switch {
		case turns < 40: // the first talks come after a dozen turns (not in -short)
		case cases[0xf5d3d] == 0 || cases[0xf5d9a] == 0:
			t.Errorf("no talk with an AI went through the arbiter")
		case n > 2 && cases[0xf63a2] == 0:
			t.Errorf("no msg 0x22 reached a third human")
		}
	} else if battles != 0 {
		// Battle results decoded on the host (msg 0x15), strategic records
		// seen by a human defender (msg 0x16).
		t.Logf("msg 0x15: %d, msg 0x16: %d, msg 0x10: %d, msg 0x12: %d, msg 0x14: %d, msg 0x3c: %d, msg 0x43: %d",
			cases[0xf609e], cases[0xf60b1], cases[0xf5fef], cases[0xf6032], cases[0xf6051], cases[0xf66c4], cases[0xf673e])
		if cases[0xf609e] == 0 || resultsChecked == 0 {
			t.Errorf("no battle results came in (%d checked)", resultsChecked)
		}
		if battles == netBattlesHumans || battles == netBattlesHumansScreen {
			t.Logf("lockstep barriers: %d, %d", len(barriers[0]), len(barriers[1]))
			if len(barriers[0]) == 0 {
				t.Errorf("no lockstep battle")
			}
			for k := 0; k < len(barriers[0]) || k < len(barriers[1]); k++ {
				var a, b string
				if k < len(barriers[0]) {
					a = barriers[0][k]
				}
				if k < len(barriers[1]) {
					b = barriers[1][k]
				}
				if a != b {
					t.Errorf("lockstep barrier %d: machine 0 %q, machine 1 %q", k, a, b)
					if k < len(mems[0]) && k < len(mems[1]) {
						var d []string
						for o := range mems[0][k] {
							if o < len(mems[1][k]) && mems[0][k][o] != mems[1][k][o] && len(d) < 12 {
								d = append(d, fmt.Sprintf("rec %d +%#x: %02x %02x", o/0x139, o%0x139, mems[0][k][o], mems[1][k][o]))
							}
						}
						t.Logf("  %s", strings.Join(d, "; "))
					}
				}
			}
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
				// The surrendered player is eliminated on the host; a client's
				// decode zeroes an eliminated player's objective, the host's not.
				if battles == netSoakSurrender && strings.HasPrefix(d.String(), fmt.Sprintf("player[%d].Objective:", n-1)) &&
					strings.HasSuffix(d.String(), "-> 0") {
					continue
				}
				// The creator's turn eliminated him: his machine keeps his own
				// objective and net id (no msg 0x3b to himself), the others not.
				if battles == netSoakEliminateCreator && (d.String() == "player[0].Objective: 100 -> 0" || d.String() == "player[0].NetID: 1 -> -1") {
					continue
				}
				if !netSoakLocal(d) {
					t.Errorf("machine %d, stardate %d: %v", m, c.stardate, d)
				}
			}
			checked++
		}
	}
	want := (n - 1) * turns
	if leave {
		want = (n-2)*turns + (netSoakLeaveAt - 35000)
	}
	if checked < want {
		t.Errorf("%d client states checked, want %d", checked, want)
	}
}

// netSoakLocal: what a client may hold differently from the host after a
// broadcast: its player number, the local settings bytes, the star owners
// as it sees them (broadcast.md, "Per-viewer state") and the save blocks
// the broadcast does not carry (the AI turn's something_interesting_happened).
func netSoakLocal(d save.FieldDiff) bool {
	s := d.String()
	switch {
	case strings.HasPrefix(s, "PLAYER_NUM"), strings.HasPrefix(s, "settings.bytes 0xda"),
		d.Block == save.BDescription, d.Block == save.BSomethingHappened, d.Block == save.BMoveableBox,
		d.Block == save.BMoveableBoxOrder, d.Block == save.BFleetIconButtons,
		d.Block == save.BCurMapScale, d.Block == save.BCurMapX, d.Block == save.BCurMapY:
		return true
	case strings.HasPrefix(s, "star[") && strings.Contains(s, "].Owner:"):
		return true
	}
	return false
}

// TestNetTurnsWeb is TestNetTurnsTCP as browsers play it: the creator's
// game is a relay hosted by a WebRelay (cmd/webserve's /net) and the joiner
// comes through its bridge, both over WebSocket.
func TestNetTurnsWeb(t *testing.T) {
	t.Parallel()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skip(err)
	}
	addr := l.Addr().String()
	l.Close()
	hosted := make(chan *netplay.Relay, 1)
	srv := httptest.NewServer(&netplay.WebRelay{Started: func(r *netplay.Relay) { hosted <- r }})
	defer srv.Close()
	web := "ws" + strings.TrimPrefix(srv.URL, "http") + "/net"
	nt := &netTurner{turns: 3, firstRow: true}
	games, _ := netGameRun(t, func(m int, g *Game, opt *Options) {
		if opt != nil {
			opt.Net = &netplay.TCP{Listen: addr, Addr: addr, Web: web}
			opt.ListKey = true
		}
	}, func(m int, g *Game, sys *host.System, games []*Game) bool {
		return nt.after(m, g, sys, games)
	})
	if len(hosted) != 1 {
		t.Error("the game was not created on the web relay")
	}
	for m, g := range games {
		if g.state.Stardate != 35003 {
			t.Errorf("machine %d at stardate %d", m, g.state.Stardate)
		}
		g.netUninit()
	}
}

// TestNetTurnsTCP plays three turns of a two-machine game over real TCP
// on localhost (the frames arrive when they arrive: only the end is
// checked).
func TestNetTurnsTCP(t *testing.T) {
	t.Parallel()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skip(err)
	}
	addr := l.Addr().String()
	l.Close()
	nt := &netTurner{turns: 3, firstRow: true}
	games, _ := netGameRun(t, func(m int, g *Game, opt *Options) {
		if opt != nil {
			opt.Net = &netplay.TCP{Listen: addr, Addr: addr}
			opt.ListKey = true
		}
	}, func(m int, g *Game, sys *host.System, games []*Game) bool {
		return nt.after(m, g, sys, games)
	})
	for m, g := range games {
		if g.state.Stardate != 35003 {
			t.Errorf("machine %d at stardate %d", m, g.state.Stardate)
		}
		g.netUninit()
	}
}
