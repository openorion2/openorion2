package game

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/openorion2/openorion2/internal/netplay"
	"github.com/openorion2/openorion2/internal/update"
)

// port-only: the desync check (-netcheck, and always in a lobby game): a
// hash of the state after each broadcast, compared across machines; a
// lobby game reports a mismatch to the lobby. Spec: docs/net/hardening.md.

// netCheckHashes holds the hashes by stardate: the own one and each net id's.
type netCheckHashes struct {
	own    map[uint32][32]byte
	theirs map[uint32]map[uint16][32]byte
	saves  map[uint32][]byte // the own saves, with -netcheckdir or a reporter
	// reported: the reports sent; done hears how each went (tests); told:
	// the player was told of a desync (once a game).
	reported int
	done     func(error)
	told     bool
}

// netCheckReports bounds the desync reports of a game: the first ones tell
// what went wrong, the rest follow from them.
const netCheckReports = 4

// desyncReporter is a transport that takes desync reports (the lobby's,
// internal/lobby.Network): sent in the background, the game goes on.
type desyncReporter interface {
	ReportDesync(stardate uint32, other uint16, own, theirs [32]byte, build string, save []byte, done func(error))
}

// netReporter is the game's transport as a desyncReporter, or nil.
func (g *Game) netReporter() desyncReporter {
	r, _ := g.net.network.(desyncReporter)
	return r
}

// netChecking: the desync check runs (-netcheck, or a lobby game).
func (g *Game) netChecking() bool { return g.net.check || g.netReporter() != nil }

// netCheckKeep is how many stardates back the hashes are kept.
const netCheckKeep = 4

// zeroed sets *p to its zero value and returns what puts it back.
func zeroed[T any](p *T) func() {
	old := *p
	var z T
	*p = z
	return func() { *p = old }
}

// netStateHash hashes the state as a save without the per-viewer fields
// (TestNetSoak's exceptions; docs/net/hardening.md, "The desync check").
func (g *Game) netStateHash() [32]byte {
	s := &g.state
	var undo []func()
	undo = append(undo, zeroed(&s.PlayerNum), zeroed(&s.Descriptions), zeroed(&s.SomethingHappened),
		zeroed(&s.MoveableBoxes), zeroed(&s.MoveableBoxOrder), zeroed(&s.FleetIconButtons),
		zeroed(&s.CurMapScale), zeroed(&s.CurMapX), zeroed(&s.CurMapY))
	// Of the settings only the universe's count (+0x02, +0xd3..0xd9, what
	// Load_Game_ takes from a save); the rest is each machine's MOX.SET.
	settings := g.settings
	for i := range g.settings {
		if i != 0x02 && (i < 0xd3 || i > 0xd9) {
			g.settings[i] = 0
		}
	}
	undo = append(undo, func() { g.settings = settings })
	for i := range s.Stars {
		undo = append(undo, zeroed(&s.Stars[i].Owner))
	}
	// The nebula, planet and star slots past the counts: what a regenerated
	// galaxy left in the host's (Init_New_Game_ generates again until the
	// home worlds fit); a client gets the counted ones (and one star more).
	for i := int(s.NumNebulas); i < len(s.Nebulas); i++ {
		undo = append(undo, zeroed(&s.Nebulas[i]))
	}
	for i := max(int(s.PlanetCount), 0); i < len(s.Planets); i++ {
		undo = append(undo, zeroed(&s.Planets[i]))
	}
	for i := max(int(s.NumStars)+1, 0); i < len(s.Stars); i++ {
		undo = append(undo, zeroed(&s.Stars[i]))
	}
	// An eliminated player's objective and net id: the host of the turn
	// that eliminated him keeps his own (he leaves on that broadcast).
	for k := range s.Players {
		if s.Players[k].Eliminated != 0 {
			undo = append(undo, zeroed(&s.Players[k].Objective), zeroed(&s.Players[k].NetID))
		}
	}
	b := g.encodeSave(9)
	for i := len(undo) - 1; i >= 0; i-- {
		undo[i]()
	}
	return sha256.Sum256(b)
}

// netCheckState hashes the state after a broadcast and sends the hash.
func (g *Game) netCheckState() {
	n := &g.net
	if !g.netChecking() || n.ep == nil || g.gameType != 2 {
		return
	}
	sd := g.state.Stardate
	h := g.netStateHash()
	if n.checks.own == nil {
		n.checks.own = map[uint32][32]byte{}
	}
	n.checks.own[sd] = h
	if n.checkDir != "" || g.netReporter() != nil {
		if n.checks.saves == nil {
			n.checks.saves = map[uint32][]byte{}
		}
		n.checks.saves[sd] = g.encodeSave(9)
	}
	for _, m := range []map[uint32][32]byte{n.checks.own} {
		for k := range m {
			if k+netCheckKeep < sd {
				delete(m, k)
			}
		}
	}
	for k := range n.checks.theirs {
		if k+netCheckKeep < sd {
			delete(n.checks.theirs, k)
		}
	}
	for k := range n.checks.saves {
		if k+netCheckKeep < sd {
			delete(n.checks.saves, k)
		}
	}
	froms := make([]uint16, 0, len(n.checks.theirs[sd]))
	for from := range n.checks.theirs[sd] {
		froms = append(froms, from)
	}
	slices.Sort(froms)
	for _, from := range froms {
		t := n.checks.theirs[sd][from]
		g.netCheckCompare(sd, from, t)
	}
	b := binary.LittleEndian.AppendUint32(nil, sd)
	n.ep.Send(netplay.Broadcast, netCtlCheck, append(b, h[:]...))
}

// netCheckRecv takes another machine's hash.
func (g *Game) netCheckRecv(from uint16, p []byte) {
	n := &g.net
	if !g.netChecking() || len(p) < 4+32 {
		return
	}
	sd := binary.LittleEndian.Uint32(p)
	var h [32]byte
	copy(h[:], p[4:])
	if n.checks.theirs == nil {
		n.checks.theirs = map[uint32]map[uint16][32]byte{}
	}
	if n.checks.theirs[sd] == nil {
		n.checks.theirs[sd] = map[uint16][32]byte{}
	}
	n.checks.theirs[sd][from] = h
	if _, ok := n.checks.own[sd]; ok {
		g.netCheckCompare(sd, from, h)
	}
}

// netCheckCompare logs a mismatch loudly and keeps the own save.
func (g *Game) netCheckCompare(sd uint32, from uint16, theirs [32]byte) {
	n := &g.net
	own := n.checks.own[sd]
	same := bytes.Equal(own[:], theirs[:])
	if n.checkHook != nil {
		n.checkHook(sd, from, same)
	}
	if same {
		return
	}
	me := n.i16(netOwnID)
	g.Log("NET DESYNC: stardate %d: the state of net id %d differs from net id %d's", sd, me, from)
	b := n.checks.saves[sd]
	if b != nil && n.checkDir != "" {
		p := filepath.Join(n.checkDir, fmt.Sprintf("netcheck-%d-id%d.GAM", sd, me))
		if err := os.WriteFile(p, b, 0o644); err != nil {
			g.Log("netcheck: %v", err)
		}
	}
	r := g.netReporter()
	if r != nil && b != nil && n.checks.reported < netCheckReports {
		n.checks.reported++
		r.ReportDesync(sd, from, own, theirs, update.Version, b, n.checks.done)
	}
	if !n.checks.told {
		n.checks.told = true
		who := fmt.Sprintf("net id %d", from)
		if k := g.netIDToPlayerID(int32(from)); k >= 0 {
			who = g.netWho(int(k))
		}
		msg := fmt.Sprintf("Desync: at stardate %d.%d the game here differs from %s's.", sd/10, sd%10, who)
		if r != nil {
			msg += " A report went to the lobby."
		}
		g.netNotice(msg + " The game goes on.")
	}
}
