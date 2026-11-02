package game

import (
	"strconv"
	"strings"

	"github.com/openorion2/openorion2/internal/save"
)

// The strategic combat result box (Russ_Combat_'s Message_Box_Titled_).
// Spec: docs/screens/combat-result.md.

// Build_End_Of_Combat_Message_'s pieces in .data (0x178dc1..0x178e17):
// \x07X<n>. moves the printer to column n, \x07X+<n>. by n.
const (
	endMsgAntares  = 0x178dc1 // "Antares"
	endMsgAttCol   = 0x178dc9 // "\x07X35."
	endMsgDefCol   = 0x178dcf // "\x07X255."
	endMsgNL       = 0x178dd6 // "\n"
	endMsgLostCol1 = 0x178dd8 // "\x07X+25."
	endMsgDefHead  = 0x178ddf // "\x07X227."
	endMsgLostCol2 = 0x178de6 // "\x07X+21."
	endMsgRowAtt   = 0x178ded // "\x07X18."
	endMsgNone     = 0x178df3 // "-"
	endMsgRowLost  = 0x178df5 // "\x07X80."
	endMsgRowName  = 0x178dfb // "\x07X120."
	endMsgRowDef   = 0x178e02 // "\x07X250."
	endMsgRowDLost = 0x178e09 // "\x07X312."
	endMsgOne      = 0x178e10 // "1"
	endMsgGap      = 0x178e12 // "\n\n"
	endMsgResult   = 0x178e15 // "\x1a2"
	nonPlayerNames = 0x178d90 // Antaran, Guardian, Amoeba, Dragon, Hydra, Eel, Crystal
)

// nonPlayerName is Get_Non_Player_Name_ (0x40cc3): the jump table at
// 0x40ca7 for owners 8..14. Other owners copy an uninitialised stack buffer
// (stale bytes, never reached by the callers here); the port returns "".
func (g *Game) nonPlayerName(p int) string {
	off := map[int]uint32{8: 0, 9: 8, 10: 0x11, 11: 0x29, 12: 0x18, 13: 0x25, 14: 0x1f}
	o, ok := off[p]
	if !ok {
		g.logOnce("Get_Non_Player_Name_: owner without a name (stale stack)")
		return ""
	}
	return cStringAt(g.exe, nonPlayerNames+o)
}

// combatSideName is the attacker or defender name of the box: the race
// name (player +0x15) or Get_Non_Player_Name_.
func (g *Game) combatSideName(p int16) string {
	if p < 8 {
		return save.CString(g.state.Players[p].RaceName[:])
	}
	return g.nonPlayerName(int(p))
}

// buildEndOfCombatMessage is Build_End_Of_Combat_Message_ (0x40d40): the
// title "Combat at <place>" and the table of ships per hull size, lost
// ones, the defenders' last satellite or monster, the colony defences of
// flags0 (those loaded before the battle) and the result line.
func (g *Game) buildEndOfCombatMessage(star int16, flags0 uint16, result int16) (title, text string, err error) {
	s, q := &g.state, g.qc
	lit := func(a uint32) string { return cStringAt(g.exe, a) }
	var attAlive, attLost, defAlive, defLost [6]int16
	last := int16(-1)
	lastAlive := true
	for i := int16(1); i < q.n; i++ {
		r := &q.rec[i]
		switch {
		case r.size > 5:
			last, lastAlive = int16(r.size), r.dead == 0
		case int16(r.owner) == q.att && r.dead == 0:
			attAlive[r.size]++
		case int16(r.owner) == q.att:
			attLost[r.size]++
		case r.dead == 0:
			defAlive[r.size]++
		default:
			defLost[r.size]++
		}
	}
	for i := range attAlive {
		attAlive[i] += attLost[i]
		defAlive[i] += defLost[i]
	}
	ken := func(n int) string {
		v, e := g.kenText(n)
		if e != nil && err == nil {
			err = e
		}
		return v
	}
	title = ken(0x62)
	switch {
	case star == s.NumStars:
		title += lit(endMsgAntares)
	case q.colony == -1:
		title += save.CString(s.Stars[star].Name[:])
	default:
		title += g.planetName(int(s.Colonies[q.colony].Planet))
	}
	var b strings.Builder
	b.WriteString(lit(endMsgAttCol))
	b.WriteString(g.combatSideName(q.att))
	b.WriteString(lit(endMsgDefCol))
	b.WriteString(g.combatSideName(q.def))
	b.WriteString(lit(endMsgNL))
	b.WriteString(ken(99))
	b.WriteString(lit(endMsgLostCol1))
	b.WriteString(ken(100))
	b.WriteString(lit(endMsgDefHead))
	b.WriteString(ken(99))
	b.WriteString(lit(endMsgLostCol2))
	b.WriteString(ken(100))
	b.WriteString(lit(endMsgNL))
	count := func(n int16) string {
		if n == 0 {
			return lit(endMsgNone)
		}
		return strconv.Itoa(int(n))
	}
	hull := func(size int, plural bool) string {
		v, e := g.hullName(size, plural)
		if e != nil && err == nil {
			err = e
		}
		return v
	}
	for i := 0; i < 6; i++ {
		if int(attAlive[i])+int(defAlive[i]) <= 0 {
			continue
		}
		b.WriteString(lit(endMsgRowAtt) + count(attAlive[i]))
		b.WriteString(lit(endMsgRowLost) + count(attLost[i]))
		b.WriteString(lit(endMsgRowName) + hull(i, true))
		b.WriteString(lit(endMsgRowDef) + count(defAlive[i]))
		b.WriteString(lit(endMsgRowDLost) + count(defLost[i]))
		b.WriteString(lit(endMsgNL))
	}
	row := func(name string, alive bool) {
		b.WriteString(lit(endMsgRowName) + name)
		b.WriteString(lit(endMsgRowDef) + lit(endMsgOne))
		if alive {
			b.WriteString(lit(endMsgRowDLost) + lit(endMsgNone))
		} else {
			b.WriteString(lit(endMsgRowDLost) + lit(endMsgOne))
		}
		b.WriteString(lit(endMsgNL))
	}
	if last > -1 {
		row(hull(int(last), false), lastAlive)
	}
	// Colony defences: missile base, ground batteries, fighter garrison,
	// stellar converter, in this order (pointers 0x17ed2b, 0x17ed3e,
	// 0x17eeba, 0x17ee5b into the building names).
	for _, d := range []struct {
		bit uint16
		bld int
	}{{1, 0x1a}, {2, 0x1b}, {8, 0x2f}, {4, 0x2a}} {
		if flags0&d.bit == 0 {
			continue
		}
		name, e := g.buildingName(d.bld)
		if e != nil && err == nil {
			err = e
		}
		row(name, uint16(q.flags)&d.bit != 0)
	}
	b.WriteString(lit(endMsgGap) + lit(endMsgResult))
	switch {
	case result >= 1:
		b.WriteString(ken(0x65))
	case result < 0:
		b.WriteString(ken(0x66))
	default:
		b.WriteString(ken(0x67))
	}
	return title, b.String(), err
}
