package game

import "github.com/openorion2/openorion2/internal/save"

// The sixth ship design (community patch 1.51): design slot 5, 1.40b23's
// scratch slot, is kept aside around scratch uses. Spec: docs/turn/design6.md.

// spareDesign is the patch's copy of the slot (0x400f06, 99 bytes) and its
// flag (0x400f6a).
type spareDesign struct {
	saved bool
	d     save.Design
}

// keepSpare copies design d aside before a scratch use overwrites it.
func (g *Game) keepSpare(d *save.Design) {
	g.spare.saved = true
	g.spare.d = *d
}

// restoreSpare puts the copy back into player p's slot 5 and clears the
// flag.
func (g *Game) restoreSpare(p int) {
	if g.spare.saved {
		g.state.Players[p].Designs[bqRefitSlot] = g.spare.d
	}
	g.spare.saved = false
}

// fixSpareDesigns is the patch's check after a save loads: a player's slot
// 5 still holding a support ship (type colony, transport, outpost: what
// 1.40b23 left there) becomes the doom star design.
func (g *Game) fixSpareDesigns() {
	s := &g.state
	for p := 0; p < int(s.NumPlayers); p++ {
		if d := &s.Players[p].Designs[bqRefitSlot]; d.Type != 0 {
			g.loadDoomStarShipDesign(p, d)
		}
	}
}

// autoDesignAll is ALT-AD in the build screen (community patch 1.51, its
// 0x406dd6): the player's six designs are made anew as at a new game, a
// hull up (destroyer to doom star) once the doom star hull is known.
func (g *Game) autoDesignAll() {
	s := &g.state
	p := int(s.PlayerNum)
	level := g.settings[0xd9]
	if s.Players[p].AppStatus[appDoomStar] == techKnown {
		g.settings[0xd9] = 2
	}
	g.initShipDesigns(p)
	g.settings[0xd9] = level
}
