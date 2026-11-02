package game

import (
	"time"

	"github.com/openorion2/openorion2/internal/save"
)

// msgSize is the size of one turn-summary message (docs/formats/save-misc.md).
const msgSize = 0x12

// addMsg is Add_Msg_ (0xef629): message m into player p's window; a full
// window replaces a lower-priority message (byte 1) instead. It returns the
// slot written, -1 for none.
func (g *Game) addMsg(p int, m [msgSize]byte) int {
	s := &g.state
	if s.MaxMsgs[p] == 0 {
		return -1
	}
	first, n := int(s.FirstMsg[p]), int(s.NMsgs[p])
	if n < int(s.MaxMsgs[p]) {
		copy(s.Msgs[(first+n)*msgSize:], m[:])
		s.NMsgs[p]++
		return first + n
	}
	// The scan runs from slot n, one past the window, down to slot 1 and
	// stops at the first one below slot 0.
	low := first
	for j := first + n; j > first; j-- {
		if s.msgByte(j, 1) < s.msgByte(first, 1) {
			low = j
			break
		}
	}
	if s.msgByte(low, 1) < m[1] {
		s.setMsg(low, m)
		return low
	}
	return -1
}

// staleSpan is a byte range of a saved block that the original fills from
// uninitialised stack; the port writes zeros there and the tests skip it.
type staleSpan struct {
	block  string
	off, n int
}

// markStaleMsg records stale bytes [lo, hi) of message slot i.
func (g *Game) markStaleMsg(i, lo, hi int) {
	if i >= 0 && i < 100 {
		g.stale = append(g.stale, staleSpan{save.BMsgs, i*msgSize + lo, hi - lo})
	}
}

// msgByte reads byte k of message slot i; slot 100 is the memory after
// _msgs: _n_msgs, _max_msgs and the _msg_april1_fmt pointer.
func (s *State) msgByte(i, k int) uint8 {
	if i < 100 {
		return s.Msgs[i*msgSize+k]
	}
	if k < 8 {
		return s.NMsgs[k]
	}
	if k < 16 {
		return s.MaxMsgs[k-8]
	}
	panic("msgByte: _msg_april1_fmt")
}

// setMsg writes message slot i; slot 100 overwrites _n_msgs and _max_msgs
// as in the original (the port does not model the clobbered pointer).
func (s *State) setMsg(i int, m [msgSize]byte) {
	if i < 100 {
		copy(s.Msgs[i*msgSize:], m[:])
		return
	}
	copy(s.NMsgs[:], m[:8])
	copy(s.MaxMsgs[:], m[8:16])
}

// Event records of event_data (9 bytes each): state at +1, colony or star
// word at +3.
const evComet = 2*9 + 1

// eventCheckComet is Event_Check_Comet_ (0x23536).
func (s *State) eventCheckComet(ci int) bool {
	return eventActive(s.EventData[evComet], true) && int16(ci) == s.eventWord(evComet+2)
}

// addOngoingEventMsgs is Add_Ongoing_Event_Msgs_ (0xef827): this turn's
// reminders of running events, and the April fool's message on 1 April
// of the first year.
func (g *Game) addOngoingEventMsgs() {
	s := &g.state
	if s.evActive(eventHyperspaceFlux) {
		var m [msgSize]byte
		m[0] = 8
		for p := int(s.NumPlayers) - 1; p >= 0; p-- {
			g.addMsg(p, m)
		}
	}
	for i := int(s.NumColonies) - 1; i >= 0; i-- {
		c := &s.Colonies[i]
		if c.Owner < 0 {
			continue
		}
		if s.eventCheckPlague(i) {
			g.addMsg(int(c.Owner), colonyMsg(c, 6, 0))
		}
		if s.eventCheckPopulationBoom(i) {
			g.addMsg(int(c.Owner), colonyMsg(c, 7, 0))
		}
		if s.eventCheckComet(i) {
			g.addMsg(int(c.Owner), colonyMsg(c, 8, 0))
		}
	}
	for st := int(s.NumStars) - 1; st >= 0; st-- {
		if !s.eventCheckSpaceAnomalyAtStar(int16(st)) {
			continue
		}
		var m [msgSize]byte
		m[0], m[2] = 0x10, uint8(st)
		for p := int(s.NumPlayers) - 1; p >= 0; p-- {
			g.addMsg(p, m)
		}
	}
	if s.Stardate != 35023 {
		return
	}
	if now := g.now(); now.Month() != time.April || now.Day() != 1 {
		return
	}
	// Bytes 1..17 come from stale stack in the original.
	var m [msgSize]byte
	m[0] = 0x15
	for p := int(s.NumPlayers) - 1; p >= 0; p-- {
		g.markStaleMsg(g.addMsg(p, m), 1, msgSize)
	}
}

// now is the host clock time_/localtime_ read; Game.Now replaces it in tests.
func (g *Game) now() time.Time {
	if g.Now != nil {
		return g.Now()
	}
	return time.Now()
}
