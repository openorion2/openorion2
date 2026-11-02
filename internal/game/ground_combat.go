package game

import (
	"github.com/openorion2/openorion2/internal/engine"
	"github.com/openorion2/openorion2/internal/gfx"
	"github.com/openorion2/openorion2/internal/save"
)

// The ground battle screen (COLCBT.C): Colony_Combat_Screen_ replays an
// invasion from its seed with troops that march, shoot and fall while
// Ground_Combat_Round_ decides who wins. Its Random draws are undone: each
// frame's drawing restores the seed, and the callers restore it after the
// screen.

const (
	gcTopLineY    = 0xb6cb0 // Top_Line_Y_'s 4 int16 by top line
	gcTopLineX    = 0xb6cb8 // Top_Line_X_'s 2 int16 by side
	gcDeltaY      = 0xb6cc4 // Do_Delta_Y_'s 11 int16 by frame
	gcAddUnitX    = 0xb6cda // Add_Unit_'s 2 int16 by side
	gcInitialX    = 0xb6cde // Add_Initial_Unit_'s 2 int16 by side
	gcColorArray  = 0x1828d3
	gcShortColors = 0x182913
	gcColonyLBX   = 0x182979 // "colony.lbx"
	gcColgcbtLBX  = 0x1829fd // "colgcbt.lbx"
	gcUnits       = 40
)

// gcUnit is a _g_units record (0x19 bytes). target is the slot (side*40+k)
// plus one, as the original keeps a pointer into the array.
type gcUnit struct {
	x, y    int16 // +0, +2
	state   uint8 // +5: 0 gone, 1 dying, 2 standing, 3 firing, 4 walking, 5 hit
	typ     int8  // +6: troop class, -1 none
	side    uint8 // +7
	frame   int8  // +8
	beam    int8  // +9
	dy      int16 // +0xa
	shot    uint8 // +0xc
	target  int   // +0xd
	beamSnd int   // +0x11: Play_Unit_Beam_Sound_'s handle
	moveSnd int   // +0x15: the walking sound's handle
}

type groundCombatScreen struct {
	units         [2][gcUnits]gcUnit
	anims         [2][4][5]*gfx.Picture // Action_Anim_ by side, class, state-1
	explosion     *gfx.Picture
	heart, frame  *gfx.Picture
	A, B          *groundForce
	topLine       int16 // 0x19f386
	ctr           int16 // _g_ctr (0x1828cc)
	started, fast bool  // 0x19f388, 0x19f384
	titleChanged  bool  // 0x1828d0
	lastShown     int16 // 0x1828d1, Print_Troop_Totals_' column count
	showing       bool  // port-only: in the input loop, for tests
}

func (g *Game) gcForce(side uint8) *groundForce {
	if side == 0 {
		return g.gcs.A
	}
	return g.gcs.B
}

func (g *Game) gcUnitAt(slot int) *gcUnit {
	return &g.gcs.units[(slot-1)/gcUnits][(slot-1)%gcUnits]
}

func (g *Game) gcDefenderApp18() bool {
	return g.state.Players[g.gcs.B.player].AppStatus[0x18] == techKnown
}

// actionAnim is Action_Anim_ (0xb81f7).
func (g *Game) actionAnim(side, state uint8, typ int8) *gfx.Picture {
	c := &g.gcs
	switch {
	case state == 0:
		return c.explosion
	case state > 5 || side > 1 || typ < 0 || typ > 3:
		return nil
	}
	return c.anims[side][typ][state-1]
}

func gcFrames(p *gfx.Picture) int16 {
	if p == nil {
		return 1
	}
	return int16(p.Frames)
}

func gcHeight(p *gfx.Picture) int16 {
	if p == nil {
		return 0
	}
	return int16(p.Height)
}

func gcWidth(p *gfx.Picture) int16 {
	if p == nil {
		return 0
	}
	return int16(p.Width)
}

// colonyCombatScreen is Colony_Combat_Screen_ (0xb7491): the replay of the
// battle between A and B for colony ci from seed; a key speeds it up, the
// next (or the first once it is over) leaves.
// groundCombatHelpList is the screen's help list (0x182953, 3 entries).
const groundCombatHelpList = 0x182953

func (g *Game) colonyCombatScreen(A, B *groundForce, seed uint32, ci int16) error {
	e, s := g.e, &g.state
	c := &g.gcs
	// 0xb749e: the next Draw_Mini_Main_Screen_ fades back into the main
	// palette (all 256 entries; the battle's planet colours sit in 0xc0..).
	g.ms.atLeastOneTacticalCombat = true
	// _g_ctr is Colony_Combat_Popup_'s counter too, and carries over.
	*c = groundCombatScreen{A: A, B: B, ctr: g.ccp.ctr}
	defer func() { g.ccp.ctr = c.ctr }()
	if g.tc != nil {
		g.tc.recordsLost = true
	}
	g.setColonyFont()
	if err := g.loadColonyScreenSeg(); err != nil {
		return err
	}
	if err := g.loadCombatAnims(); err != nil {
		return err
	}
	cs := &g.cscr
	cs.usingPal = true
	g.colonyHandle = int(ci)
	cs.planet = int(s.Colonies[ci].Planet)
	cs.starHandle = int(s.Planets[cs.planet].Star)
	cs.orbitN = int(s.Planets[cs.planet].Orbit)
	cs.valid = true
	g.ccp.queue = g.ccp.queue[:0]
	g.calcTopLine()
	g.spatterDownStartingTroops()
	if cs.valid {
		if err := g.makeBldgArrayForColony(g.colonyHandle, true); err != nil {
			return err
		}
	}
	e.SetMouseList(engine.MouseNone)
	if err := e.FastFadeOut(); err != nil {
		return err
	}
	if err := g.updateColonyPalette(); err != nil {
		return err
	}
	if err := e.DarkenPalette(100); err != nil {
		return err
	}
	e.V.ClearPaletteChanges(0, 0xff)
	e.RefreshPalette()
	g.resetFieldArrays()
	e.ClearFields()
	if err := g.gcAddFields(); err != nil {
		return err
	}
	g.col.fullDraw = true
	saved := e.GetAutoFunction()
	e.AssignAutoFunction(g.drawColonyCombatScreen, 1)
	g.setGamePopupHelpList(groundCombatHelpList, 3) // Eric_Set_Help_List_
	e.V.Fill(0, 0, 0x27f, 0x1df, 0)
	if err := g.drawColonyCombatScreen(); err != nil {
		return err
	}
	if err := e.DarkenPalette(100); err != nil {
		return err
	}
	e.V.ClearPaletteChanges(0, 0xff)
	if err := e.TogglePages(); err != nil {
		return err
	}
	if err := e.FastFadeIn(); err != nil {
		return err
	}
	e.SetMouseList(engine.MouseArrow)
	e.V.SetRefreshStencil()
	g.clearMouseBuffer()
	e.SetInputDelay(5)
	e.MarkTime()
	if err := e.ReleaseTime(0x10); err != nil {
		return err
	}
	s.Random.Set(seed)
	c.showing = true
	defer func() { c.showing = false }()
	for done := false; !done; {
		e.MarkTime()
		g.playStreamingMusic(0xa, -1, true) // 0xb764b: the battle's music
		e.ClearFields()
		if err := g.gcAddFields(); err != nil {
			return err
		}
		if err := g.drawColonyCombatScreen(); err != nil {
			return err
		}
		if err := e.TogglePages(); err != nil {
			return err
		}
		in, err := e.GetInput()
		if err != nil {
			return err
		}
		if _, err := e.ScanInput(); err != nil {
			return err
		}
		if in != 0 {
			if !c.fast && c.A.class < 4 && c.B.class < 4 {
				c.fast = true
			} else {
				done = true
			}
			// A confirmation box or popup cleanup before the battle left the
			// flag set; the first input redraws the whole screen (0xb769d).
			if g.fullDrawScreen {
				g.col.fullDraw = true
				g.fullDrawScreen = false
				g.setGamePopupHelpList(groundCombatHelpList, 3)
			}
		}
		if err := e.ReleaseTime(1); err != nil {
			return err
		}
	}
	g.playBackgroundMusic() // 0xb76db
	e.StopAllSounds()
	if err := g.resetFieldMode(); err != nil {
		return err
	}
	e.ClearFields()
	e.DeactivateHelpList()
	e.DeactivateAutoFunction()
	e.PutAutoFunction(saved)
	g.ccp.queue = g.ccp.queue[:0]
	cs.usingPal = false
	g.currentScreen = g.returnScreen
	return nil
}

// gcAddFields is the screen's Add_Fields_ (0xb6d05): the whole screen and
// a hot key, both ending it.
func (g *Game) gcAddFields() error {
	if _, err := g.e.AddHiddenField(0, 0, 0x27f, 0x1df, g.eStr(0xc), 0); err != nil {
		return err
	}
	_, err := g.e.AddHotKey(g.eStr(0x87))
	return err
}

// loadCombatAnims is Load_Combat_Anims_ (0xb6d51): COLGCBT.LBX troops for
// both sides in their colours (the defender's armour in its second set
// when it knows application 0x18), the hearts and frame, COLONY.LBX 7.
func (g *Game) loadCombatAnims() error {
	c := &g.gcs
	colgcbt := upperLBX(cStringAt(g.exe, gcColgcbtLBX))
	load := func(file string, n int) (*gfx.Picture, error) { return g.picture(file, n) }
	var err error
	if c.explosion, err = load(upperLBX(cStringAt(g.exe, gcColonyLBX)), 7); err != nil {
		return err
	}
	set := func(side int, first int, typs []int, order [5]int, player int16) error {
		var p [5]*gfx.Picture
		for k := 0; k < 5; k++ {
			q, err := load(colgcbt, first+k)
			if err != nil {
				return err
			}
			g.replaceColgcbtColors(q, player)
			p[k] = q
		}
		for _, t := range typs {
			for st := 0; st < 5; st++ {
				c.anims[side][t][st] = p[order[st]]
			}
		}
		return nil
	}
	// States 1..5 take entries 1, 4, 2, 0, 3 of each five.
	order := [5]int{1, 4, 2, 0, 3}
	if err := set(0, 0, []int{1, 2, 3}, order, c.A.player); err != nil {
		return err
	}
	if err := set(1, 5, []int{1, 2, 3}, order, c.B.player); err != nil {
		return err
	}
	armor := 10
	if g.gcDefenderApp18() {
		armor = 0xf
	}
	if err := set(1, armor, []int{0}, order, c.B.player); err != nil {
		return err
	}
	if c.heart, err = load(colgcbt, 0x14); err != nil {
		return err
	}
	c.frame, err = load(colgcbt, 0x15)
	return err
}

// upperLBX is an LBX file name in upper case.
func upperLBX(name string) string {
	b := []byte(name)
	for i, ch := range b {
		if ch >= 'a' && ch <= 'z' {
			b[i] = ch - 0x20
		}
	}
	return string(b)
}

// replaceColgcbtColors is Replace_Colgcbt_Color_With_Player_Colors_
// (0xb8efb).
func (g *Game) replaceColgcbtColors(p *gfx.Picture, player int16) {
	col := int(g.state.Players[player].Color)
	if col == 2 {
		return
	}
	p.ReplaceColorRange(g.exe.bytes(uint32(gcColorArray+col*8), 8), 0xc0, 8)
	p.ReplaceColorRange(g.exe.bytes(uint32(gcShortColors+col*8), 4), 0xe8, 4)
}

// calcTopLine is Calc_Top_Line_ (0xb77a6): how far up the troops may go,
// from the building array of the last colony shown; its first cell is
// cleared on the way.
func (g *Game) calcTopLine() {
	b := &g.cscr.surf.bldgs
	switch {
	case b[0][0] > 0:
		g.gcs.topLine = 1
		b[0][0] = 0
	case b[1][0] > 0 || b[0][1] > 0 || b[1][1] > 0:
		g.gcs.topLine = 1
	case b[2][0] > 0 || b[0][2] > 0 || b[2][1] > 0 || b[1][2] > 0 || b[2][2] > 0:
		g.gcs.topLine = 2
	default:
		g.gcs.topLine = 3
	}
}

func (g *Game) topLineY() int16 { return g.exeI16(uint32(gcTopLineY + 2*int32(g.gcs.topLine))) }
func (g *Game) topLineX(side uint8) int16 {
	return g.exeI16(uint32(gcTopLineX + 2*int32(side)))
}

// nSlotsLeft is N_Slots_Left_ (0xb771d).
func (g *Game) nSlotsLeft(side uint8) int16 {
	n := int16(0)
	for k := range g.gcs.units[side] {
		if g.gcs.units[side][k].state == 0 {
			n++
		}
	}
	return n
}

// firstFreeUnit is Get_First_Unit_ (0xb86f6).
func (g *Game) firstFreeUnit(side uint8) *gcUnit {
	for k := range g.gcs.units[side] {
		if g.gcs.units[side][k].state == 0 {
			return &g.gcs.units[side][k]
		}
	}
	return nil
}

// nTroopType is N_Troop_Type_ (0xb8732): units of class t on the field.
func (g *Game) nTroopType(t int16, side uint8) int16 {
	n := int16(0)
	for k := range g.gcs.units[side] {
		u := &g.gcs.units[side][k]
		if int16(u.typ) == t && u.state != 0 {
			n++
		}
	}
	return n
}

// nextTroopType is Get_Next_Troop_Type_ (0xb877d): the first class with
// fewer units on the field than troops, or -1.
func (g *Game) nextTroopType(side uint8) int16 {
	F := g.gcForce(side)
	for t := int16(0); t < 4; t++ {
		if g.nTroopType(t, side) < F.count[t] {
			return t
		}
	}
	return -1
}

// addInitialUnit is Add_Initial_Unit_ (0xb88b2).
func (g *Game) addInitialUnit(side uint8) {
	r := &g.state.Random
	u := g.firstFreeUnit(side)
	if u == nil {
		return
	}
	t := g.nextTroopType(side)
	u.typ = int8(t)
	x0 := g.exeI16(uint32(gcInitialX + 2*int32(side)))
	if int8(t) == -1 {
		u.x = x0
		return
	}
	u.state = 2
	u.x = x0 + int16(r.Random(0x32)) - 0x14
	if int32(r.Random(0x55))+0x168 < 0x1ae {
		u.y = int16(r.Random(0x55)) + 0x168
	} else {
		u.y = 0x1ae
	}
	u.side = side
	d := int16(r.Random(3)) - 2
	u.frame = 0
	u.dy = d
}

// addUnit is Add_Unit_ (0xb87cd): a reinforcement at the side's edge.
func (g *Game) addUnit(side uint8) {
	r := &g.state.Random
	u := g.firstFreeUnit(side)
	if u == nil {
		return
	}
	t := g.nextTroopType(side)
	u.typ = int8(t)
	x0 := g.exeI16(uint32(gcAddUnitX + 2*int32(side)))
	if int8(t) == -1 {
		u.x = x0
		return
	}
	u.state = 2
	u.x = x0
	lim := 0x1e0 - int32(gcHeight(g.actionAnim(u.side, u.state, u.typ)))
	var y int16
	if lim > int32(r.Random(0x55))+0x168 {
		y = int16(r.Random(0x55)) + 0x168
	} else {
		y = int16(0x1e0 - int32(gcHeight(g.actionAnim(u.side, u.state, u.typ))))
	}
	u.side = side
	u.y = y
	d := int16(r.Random(3)) - 2
	u.frame = 0
	u.target = 0
	u.dy = d
}

// spatterDownStartingTroops is Spatter_Down_Starting_Troops_ (0xb7825): up
// to ten units a side.
func (g *Game) spatterDownStartingTroops() {
	for side := uint8(0); side < 2; side++ {
		for k := range g.gcs.units[side] {
			u := &g.gcs.units[side][k]
			u.x, u.y, u.state = 0, 0, 0
		}
		n := g.nSlotsLeft(side)
		stop := n == 0
		if n > 10 {
			n = 10
		}
		for !stop {
			g.addInitialUnit(side)
			if n--; n == 0 || g.nextTroopType(side) == -1 {
				stop = true
			}
		}
		g.resortUnits(side)
	}
}

// resortUnits is Resort_Units_ (0xb7b92): bubble sort by the bottom edge.
func (g *Game) resortUnits(side uint8) {
	us := &g.gcs.units[side]
	for i := 0; i < gcUnits; i++ {
		for k := 0; k < gcUnits-1; k++ {
			a, b := &us[k], &us[k+1]
			ha := int32(gcHeight(g.actionAnim(a.side, a.state, a.typ)))
			hb := int32(gcHeight(g.actionAnim(b.side, b.state, b.typ)))
			if int32(a.y)+ha > int32(b.y)+hb {
				us[k], us[k+1] = us[k+1], us[k]
			}
		}
	}
}

// verifyLegalTotals is Verify_Legal_Totals_ (0xb7990): units of a class
// already beaten fall.
func (g *Game) verifyLegalTotals() {
	c := &g.gcs
	for k := 0; k < gcUnits; k++ {
		for side, F := range [2]*groundForce{c.A, c.B} {
			u := &c.units[side][k]
			if int8(u.state) > 1 && int32(u.typ) < int32(F.class) {
				u.state, u.frame, u.shot = 1, 0, 0
			}
		}
	}
}

// drawTroops is Draw_Troops_ (0xb7acc).
func (g *Game) drawTroops() error {
	g.verifyLegalTotals()
	for side := uint8(0); side < 2; side++ {
		for k := 0; k < gcUnits; k++ {
			if err := g.doUnitTurn(&g.gcs.units[side][k]); err != nil {
				return err
			}
		}
		g.forceFiringPairs()
		g.reinforceUnits(side)
		g.resortUnits(side)
	}
	return nil
}

// reinforceUnits is Reinforce_Units_ (0xb86b4).
func (g *Game) reinforceUnits(side uint8) {
	n := g.nSlotsLeft(side)
	stop := n == 0
	if n > 10 {
		n = 10
	}
	for !stop {
		g.addUnit(side)
		if n--; n == 0 {
			stop = true
		}
	}
}

// nLeftActive is N_Left_Active_To_Process_ (0xb7b57): units of u's class
// from u to the end of its side, neither gone nor dying.
func (g *Game) nLeftActive(u *gcUnit, from int) int16 {
	n := int16(0)
	for k := from; k < gcUnits; k++ {
		v := &g.gcs.units[u.side][k]
		if v.state != 0 && v.state != 1 && v.typ == u.typ {
			n++
		}
	}
	return n
}

// nLeftOfMe is N_Left_Of_Me_To_Process_ (0xb7b1c).
func (g *Game) nLeftOfMe(u *gcUnit, from int) int16 {
	n := int16(0)
	for k := from; k < gcUnits; k++ {
		v := &g.gcs.units[u.side][k]
		if v.state == u.state && v.typ == u.typ {
			n++
		}
	}
	return n
}

// pickTarget is Pick_Target_ (0xb7e22): the first unshot unit of side that
// stands hit or dies.
func (g *Game) pickTarget(side uint8) int {
	for k := 0; k < gcUnits; k++ {
		v := &g.gcs.units[side][k]
		if (v.state == 5 || v.state == 1) && v.shot == 0 {
			return int(side)*gcUnits + k + 1
		}
	}
	return 0
}

// unitIndex finds u's slot in its side.
func (g *Game) unitIndex(u *gcUnit) int {
	for k := range g.gcs.units[u.side] {
		if &g.gcs.units[u.side][k] == u {
			return k
		}
	}
	return 0
}

// doUnitTurn is Do_Unit_Turn_ (0xb7e84): a unit reacts to its side's hits,
// aims, walks or stands, and is drawn.
func (g *Game) doUnitTurn(u *gcUnit) error {
	c := &g.gcs
	r := &g.state.Random
	g.setColonyFontTo(1)
	if u.state == 0 {
		return nil
	}
	F := g.gcForce(u.side)
	idx := g.unitIndex(u)
	switch {
	case F.lost == u.typ && u.state != 1:
		u.state, u.frame, u.shot = 1, 0, 0
		F.lost = -1
		g.col.fullDraw = true
		F.hurt = -1
	case F.hurt == u.typ && u.state != 1 && u.state != 5:
		if int32(r.Random(uint32(g.nLeftActive(u, idx)))) <= 1 {
			u.state, u.shot, u.frame = 5, 0, 0
			F.hurt = -1
		}
	}
	anim := g.actionAnim(u.side, u.state, u.typ)
	if u.state == 3 || (u.state != 0 && u.target != 0) {
		if u.frame == 0 && u.state == 3 && u.beam >= 10 {
			u.target = 0
		}
		if u.target == 0 {
			other := uint8(0)
			if u.side == 0 {
				other = 1
			}
			u.target = g.pickTarget(other)
			pick := u.target == 0
			if !pick && int32(r.Random(uint32(g.nLeftOfMe(u, idx))))-1 <= 1 {
				pick = true
			}
			if pick {
				u.target = 0
				if r.Random(2) == 1 {
					u.state = 4
				} else {
					u.state = 2
					u.frame = 0
				}
				anim = g.actionAnim(u.side, u.state, u.typ)
			}
			u.beam = 0
			u.beamSnd = 0
		}
		if u.target != 0 && g.gcUnitAt(u.target).typ != -1 {
			g.gcUnitAt(u.target).shot = 1
			g.drawUnitBeam(u)
		}
	} else if u.target != 0 && g.gcUnitAt(u.target).typ == -1 {
		u.target = 0
	}
	if int32(u.frame) < int32(gcFrames(anim))-1 && u.state != 2 {
		if c.ctr%2 == 0 {
			u.frame++
		}
		if u.target != 0 {
			// 0xb80a2: the hit's sound halfway, the shot's at the start,
			// faded at frame 10.
			if u.beam == 5 {
				t := g.gcUnitAt(u.target)
				if t.typ == 0 && t.state == 1 {
					g.moxPlaySound(0x1e)
				} else {
					g.moxPlaySound(0x1c)
				}
			}
			if u.beam == 0 {
				g.playUnitBeamSound(u)
			}
			if u.beam == 10 {
				g.fadeoutSound(u.beamSnd, 0xa)
			}
			if u.beam++; u.beam > 0xb {
				u.target = 0
			}
		}
	} else {
		g.pickNextAction(u)
	}
	// The picture is the one chosen before Pick_Next_Action_.
	if u.state != 0 {
		if anim == nil {
			g.logOnce("Do_Unit_Turn_: a troop picture the screen never loaded")
		} else {
			g.setAnimationFrame(anim, int(u.frame))
			if err := g.e.Draw(int(u.x), int(u.y), anim); err != nil {
				return err
			}
		}
	}
	if u.state == 4 && (u.x < 0x136 || u.x > 0x14a) {
		app := g.gcDefenderApp18()
		move := false
		switch {
		case u.typ >= 1 && u.frame%2 == 0:
			move = true
		case u.typ == 0 && app && u.frame%2 == 0 && u.frame > 1:
			move = true
		case !app:
			move = true
		}
		if move {
			u.x += 1 - 2*int16(int8(u.side))
		}
	}
	g.doDeltaY(u)
	return nil
}

// pickNextAction is Pick_Next_Action_ (0xb828c).
func (g *Game) pickNextAction(u *gcUnit) {
	c := &g.gcs
	r := &g.state.Random
	over := c.A.class >= 4 || c.B.class >= 4
	u.frame = 0
	switch u.state {
	case 0:
		if u.typ != -1 {
			u.typ = -1
		}
	case 1:
		u.state = 0
		g.fadeoutSound(u.moveSnd, 0xa) // 0xb83a8, left set
		g.col.fullDraw = true
	case 2:
		if !over {
			u.state = 3
		} else {
			u.state = 4
		}
	case 3:
		if over {
			u.state = 2
			break
		}
		switch r.Random(4) {
		case 0:
			u.state = 4
			u.dy = int16(r.Random(3)) - 2
		case 2:
			u.state = 3
			u.x += int16(r.Random(3)) - 2
		default:
			u.state = 4
		}
	case 4:
		g.fadeoutSound(u.moveSnd, 0xa)
		u.moveSnd = 0
		if !over && r.Random(2) == 1 {
			u.state = 3
			break
		}
		if r.Random(2) != 1 {
			u.state = 4
		} else {
			u.state = 2
		}
	case 5:
		u.state = 2
	}
	g.doLegalMoveCheck(u)
	if u.state == 4 && u.typ == 0 && u.moveSnd == 0 {
		u.moveSnd = g.moxPlaySound(0x1f) // 0xb83e1: footsteps
	}
}

// moveIsLegal is Move_Is_Legal_ (0xb83ed); every path but one, which
// cannot be reached, says yes.
func (g *Game) moveIsLegal(u *gcUnit) bool {
	r := &g.state.Random
	anim := g.actionAnim(u.side, u.state, u.typ)
	y2 := int16(int32(gcFrames(anim))*int32(u.dy)/5 + int32(u.y) + int32(gcHeight(anim)))
	x2 := gcWidth(anim) + u.x
	if u.state == 4 && u.typ == 0 {
		x2 += (1 - 2*int16(int8(u.side))) * 0x28
	}
	if y2 < 0x1e0 && y2 > g.topLineY() {
		if u.state != 4 {
			return true
		}
		if u.side == 0 && x2 < g.topLineX(0) {
			return true
		}
		if u.side == 1 && x2 > g.topLineX(1) {
			return true
		}
	}
	if y2 >= 0x1e0 {
		u.dy = -1
		return true
	}
	if y2 <= g.topLineY() {
		var same int32
		if u.side == 0 {
			same = pointsOnSameSideOfLine(int32(x2), int32(y2), 0, 0x1df, 0xa, 0x169, 0xe1, 0x1cd)
		} else {
			same = pointsOnSameSideOfLine(int32(x2), int32(y2), 0x276, 0x1df, 0x196, 0x1cd, 0x275, 0x16c)
		}
		if same <= 0 {
			u.dy = 1
		}
		return true
	}
	if u.state == 2 {
		return false
	}
	u.state = 2
	g.fadeoutSound(u.moveSnd, 0xa)
	u.moveSnd = 0
	u.dy = int16(r.Random(3)) - 2
	return true
}

// doLegalMoveCheck is Do_Legal_Move_Check_ (0xb8575).
func (g *Game) doLegalMoveCheck(u *gcUnit) {
	r := &g.state.Random
	anim := g.actionAnim(u.side, u.state, u.typ)
	y2 := int16(int32(u.dy)*int32(gcFrames(anim))/5 + int32(u.y) + int32(gcHeight(anim)))
	for !g.moveIsLegal(u) {
		switch r.Random(4) - 1 {
		case 0:
			u.dy = 0
		case 1:
			u.dy = 1
		case 2:
			u.dy = -1
		case 3:
			u.state = 2
			g.fadeoutSound(u.moveSnd, 0xa)
			u.moveSnd = 0
		}
		if y2 > 0x1de {
			u.dy = -1
		}
	}
}

// doDeltaY is Do_Delta_Y_ (0xb861f): walkers drift up or down on the
// frames the table marks (the defender's armour on every third one).
func (g *Game) doDeltaY(u *gcUnit) {
	if u.state != 4 {
		return
	}
	step := u.typ == 0 && !g.gcDefenderApp18() && int32(u.frame)%3 == 2
	if !step {
		if u.frame < 0 || u.frame >= 11 {
			g.logOnce("Do_Delta_Y_: a frame past its table reads stale stack")
			return
		}
		step = g.exeI16(uint32(gcDeltaY+2*int32(u.frame))) == 1
	}
	if step {
		u.y += u.dy
	}
}

// forceFiringPairs is Force_Firing_Pairs_ (0xb7a12): each side's first
// unshot dying unit draws fire from the lowest-framed idle enemy, else
// explodes on its own.
func (g *Game) forceFiringPairs() {
	c := &g.gcs
	for side := uint8(0); side < 2; side++ {
		t := g.findUnshotTarget(side)
		if t == 0 {
			continue
		}
		other := uint8(0)
		if side == 0 {
			other = 1
		}
		f := g.findLowestFramedUnit(other, 3, true)
		if f == nil {
			f = g.findLowestFramedUnit(other, 2, false)
		}
		if f == nil {
			f = g.findLowestFramedUnit(other, 4, false)
		}
		T := g.gcUnitAt(t)
		if f == nil {
			g.ccp.addFlic(T.x+0xf, T.y+0xf, c.explosion, 1, 1)
			continue
		}
		f.state, f.frame, f.target = 3, 0, t
		T.shot = 1
	}
}

// findUnshotTarget is Find_Unshot_Target_ (0xb78be).
func (g *Game) findUnshotTarget(side uint8) int {
	for k := 0; k < gcUnits; k++ {
		v := &g.gcs.units[side][k]
		if v.state == 1 && v.shot == 0 {
			return int(side)*gcUnits + k + 1
		}
	}
	return 0
}

// findLowestFramedUnit is Find_Lowest_Framed_Unit_ (0xb7908): the last of
// the units in state with the lowest frame (ties go to the later one).
func (g *Game) findLowestFramedUnit(side, state uint8, idle bool) *gcUnit {
	var best *gcUnit
	low := int16(100)
	for k := 0; k < gcUnits; k++ {
		v := &g.gcs.units[side][k]
		if int16(int8(v.state)) != int16(state) || low < int16(v.frame) {
			continue
		}
		if idle && v.target != 0 {
			continue
		}
		low = int16(v.frame)
		best = v
	}
	return best
}

// firepointOffsets is Get_Firepoint_Offsets_ (0xb7c8d); ok is false where
// the original leaves the offsets unset.
func (g *Game) firepointOffsets(u *gcUnit) (dx, dy int16, ok bool) {
	w := gcWidth(g.actionAnim(u.side, u.state, u.typ))
	app := g.gcDefenderApp18()
	switch {
	case u.side > 1:
		return 0, 0, false
	case u.typ == 0 && u.side == 0:
		if app {
			return w - 9, 0xd, true
		}
		return w - 0xa, 0x14, true
	case u.typ == 0:
		if app {
			return 9, 0xd, true
		}
		return 0xa, 0x14, true
	case u.typ >= 1 && u.typ <= 3:
		return 0x10, 0x13, true
	}
	return 0, 0, false
}

// targetOffsets is Get_Target_Offsets_ (0xb7d48).
func (g *Game) targetOffsets(u *gcUnit) (dx, dy int16, ok bool) {
	w := gcWidth(g.actionAnim(u.side, u.state, u.typ))
	app := g.gcDefenderApp18()
	switch {
	case u.side > 1:
		return 0, 0, false
	case u.typ < 0 || u.typ > 3:
		return 0, 0, false
	case u.typ >= 1:
		if u.side == 0 {
			return 0xe, 0x12, true
		}
		return 0xa, 0x12, true
	case u.side == 0:
		if app {
			return w - 0xf, 0xe, true
		}
		return w - 0x16, 0x1a, true
	case app:
		return 0xf, 0xe, true
	case u.state == 5:
		return 0xe, 0x17, true
	}
	return 0x16, 0x1a, true
}

// cgcbtBeamLen is Get_CGCBT_Beam_Len_ (0xb92a4): by the side's best ground
// weapon (bonus +9).
func (g *Game) cgcbtBeamLen(side uint8) int16 {
	switch g.gcForce(side).bonus[9] {
	case 0x49, 0x8a:
		return 0x46
	case 0x65:
		return 0x64
	case 0x80:
		return 0x14
	}
	return 1
}

// drawUnitBeam is Draw_Unit_Beam_ (0xb8f5f): u's shot grows toward its
// target with the beam counter.
func (g *Game) drawUnitBeam(u *gcUnit) {
	T := g.gcUnitAt(u.target)
	fx, fy, ok1 := g.firepointOffsets(u)
	tx0, ty0, ok2 := g.targetOffsets(T)
	if !ok1 || !ok2 {
		g.logOnce("Draw_Unit_Beam_: offsets the original leaves on the stack")
		return
	}
	x1, y1 := u.x+fx, u.y+fy
	x2, y2 := x1, y1
	tx, ty := T.x+tx0, T.y+ty0
	abs := func(v int32) int32 {
		if v < 0 {
			return -v
		}
		return v
	}
	ax := abs(int32(int16(x1)) - (int32(tx0) + int32(T.x)))
	ay := abs(int32(int16(y1)) - (int32(ty0) + int32(T.y)))
	var dist int16
	if ax < ay {
		dist = int16(abs(int32(y1) - (int32(ty0) + int32(T.y))))
	} else {
		dist = int16(abs(int32(x1) - (int32(tx0) + int32(T.x))))
	}
	ln := g.cgcbtBeamLen(u.side)
	a := int16(int32(dist) / 5 * int32(u.beam))
	first := min(dist, a)
	if first < 0 {
		first = 0
	}
	robInterpolateLine(&x1, &y1, tx, ty, first)
	second := int16(dist)
	if s := int32(a) + int32(ln); s <= int32(dist) {
		second = int16(s)
	}
	robInterpolateLine(&x2, &y2, tx, ty, second)
	g.drawCGCBTBeam(x1, y1, x2, y2, u.side)
}

// drawCGCBTBeam is Draw_CGCBT_Beam_ (0xb90ba): a bolt in the colours of
// the side's best ground weapon.
func (g *Game) drawCGCBTBeam(x1, y1, x2, y2 int16, side uint8) {
	if x1 == x2 && y1 == y2 {
		return
	}
	var start, count, chance, maxChange, first, last, fading, interlock, rotation, head int16
	switch g.gcForce(side).bonus[9] {
	case 0x49:
		start, count, chance, maxChange, first, last = 0xb7, 0xb, 0x28, 4, 0xba, 0xbd
	case 0x65:
		start, count, chance, maxChange, first, last, interlock = 0xec, 4, 0xa, 1, 0xec, 0xef, 1
	case 0x80:
		start, count, chance, maxChange, first, last, interlock, rotation, head = 0xec, 4, 0x28, 2, 0xec, 0xef, 1, 1, 1
	case 0x8a:
		start, count, chance, maxChange, first, last, fading, head = 0xec, 4, 0x46, 2, 0xec, 0xef, 1, 1
	case 0x91:
		start, count, chance, maxChange, first, last, fading, head = 0x5f, 0xb, 0xa, 2, 0x60, 0x68, 1, 1
	default:
		g.logOnce("Draw_CGCBT_Beam_: a weapon without colours draws from stale stack")
		return
	}
	if x2 == x1 || y1 == y2 {
		return
	}
	g.boltSegment(x1, y1, x2, y2, start, count, chance, maxChange, first, last, fading, interlock, rotation, head)
}

// drawColonyCombatScreen is Draw_Colony_Combat_Screen_ (0xb7289), also the
// screen's auto function: the backdrop on full draws, the summary boxes,
// the buildings, the title, the troops; then, past the first frame, a
// Ground_Combat_Round_ every sixth (every one when fast, all of them in
// one go).
func (g *Game) drawColonyCombatScreen() error {
	e, s := g.e, &g.state
	c := &g.gcs
	v := e.V
	seed := s.Random.Get()
	v.SetPageOff()
	if g.col.fullDraw {
		v.Fill(0, 0, 0x27f, 0x1df, 0)
		for _, n := range []int{1, 0} {
			p, err := g.cAnimsColony(n)
			if err != nil {
				return err
			}
			if err := e.Draw(0, 0, p); err != nil {
				return err
			}
		}
	}
	g.setColonyFontTo(1)
	if err := g.drawSummaryBox(0); err != nil {
		return err
	}
	if err := g.drawSummaryBox(1); err != nil {
		return err
	}
	g.setColonyFontTo(1)
	if err := g.drawColonyBldgs(); err != nil {
		return err
	}
	if g.col.fullDraw {
		v.CopyOffToBack()
		v.SetRefreshStencil()
		g.col.fullDraw = false
	}
	g.setColonyFontTo(3)
	if c.A.class >= 4 || c.B.class >= 4 {
		if err := g.drawGroundSummary(); err != nil {
			return err
		}
		if !c.titleChanged {
			c.titleChanged = true
			g.col.fullDraw = true
		}
	} else {
		name := save.CString(s.Players[c.A.player].RaceName[:])
		t := cSprintf(g.eStr(0x2f), g.planetName(int(s.Colonies[g.colonyHandle].Planet)), name)
		if err := e.Text().PrintCentered(0x13f, 0xa, t); err != nil {
			return err
		}
	}
	v.SetWindow(0, 0, 0x27f, 0x1df)
	v.ClipOn()
	if err := g.drawTroops(); err != nil {
		return err
	}
	if err := g.drawFlicQueue(); err != nil {
		return err
	}
	v.ClipOff()
	s.Random.Set(seed)
	for {
		c.ctr++
		if c.A.class < 4 && c.B.class < 4 && c.started {
			if c.ctr%6 == 0 || c.fast {
				g.groundCombatRound(c.A, c.B)
			}
			if !c.fast {
				if err := e.ReleaseTime(1); err != nil {
					return err
				}
			}
		} else if !c.started {
			c.started = true
		}
		if !(c.fast && c.A.class < 4 && c.B.class < 4) {
			return nil
		}
	}
}

// drawGroundSummary is Draw_Summary_ (0xb71b9): who took the planet.
func (g *Game) drawGroundSummary() error {
	s := &g.state
	c := &g.gcs
	lost := true
	for k := 0; k < 4; k++ {
		if c.A.count[k] > 0 {
			lost = false
		}
	}
	g.setColonyFontTo(4)
	n := 0x3e
	if lost {
		n = 0x3d
	}
	name := save.CString(s.Players[c.A.player].RaceName[:])
	t := cSprintf(g.eStr(n), g.planetName(int(s.Colonies[g.colonyHandle].Planet)), name)
	return g.e.Text().PrintCentered(0x13f, 0xa, t)
}

// drawSummaryBox is Draw_Summary_Box_ATK_ (0xb8bc7) and _DEF_ (0xb8c8b),
// on full draws only.
func (g *Game) drawSummaryBox(side uint8) error {
	c := &g.gcs
	if !g.col.fullDraw {
		return nil
	}
	F := g.gcForce(side)
	x1, x2, frameX, titleX, title := 2, 0x103, 1, 0x82, 0xbb
	if side == 1 {
		x1, x2, frameX, titleX, title = 0x17b, 0x27e, 0x17a, 0x1fc, 0x100
	}
	g.darkenFill(x1, 0x29, x2, 0xb8)
	if err := g.e.Draw(frameX, 0x28, c.frame); err != nil {
		return err
	}
	g.setColonyFontToPlayer(2, int(F.player))
	line := 0
	if err := g.e.Text().PrintCentered(titleX, line*0xb+0x32, g.eStr(title)); err != nil {
		return err
	}
	line++
	pd := func(name string, v int16) error {
		x := 1
		if F != c.A {
			x = 0x17a
		}
		t := cSprintf(g.eStr(0x4b), cStringAt(g.exe, printDataFieldFmt), name, cStringAt(g.exe, printDataValueFmt), int(v))
		err := g.printFormattedParagraph(x+9, int(int16(line*0xb+0x32)), 0xf3, t, 0)
		line++
		return err
	}
	if err := g.bestSummary(F, pd); err != nil {
		return err
	}
	if err := g.raceSummary(F, pd); err != nil {
		return err
	}
	line++
	err := g.gcTroopTotals(frameX, F)
	line++
	return err
}

// gcTroopTotals is the screen's Print_Troop_Totals_ (0xb896d): a column per
// class present, in the attacker's colours whatever the side.
func (g *Game) gcTroopTotals(x int, F *groundForce) error {
	s := &g.state
	c := &g.gcs
	t := g.e.Text()
	n := 0
	for k := 0; k < 4; k++ {
		if F.count[k] != 0 {
			n++
		}
	}
	shown := int16(0)
	race := int(s.Players[F.player].Race)
	for k := 0; k < 4; k++ {
		if F.count[k] == 0 {
			continue
		}
		anim, err := g.playerTroopAnim(int(F.player), k, race)
		if err != nil {
			return err
		}
		g.setColonyFontToPlayer(1, int(c.A.player))
		cx := int(int16(int32(0x105)/int32(n+1)*int32(shown+1) + int32(int16(x))))
		if err := t.PrintCentered(cx, 0xb0, cSprintf(g.eStr(0xd), int(F.strength[k])-int(int8(F.bonus[0xf])))); err != nil {
			return err
		}
		f := 0x16
		if F.hits[k] > 1 {
			f = 0x17
		}
		if err := t.PrintCentered(cx, 0xa7, cSprintf(g.eStr(f), int(F.hits[k]))); err != nil {
			return err
		}
		var str string
		if g.settings[0xd2] == 4 {
			str = cSprintf(cStringAt(g.exe, 0x179dbc), g.playerTroopName(int(F.player), k), int(F.count[k]))
		} else {
			str = cSprintf(g.eStr(0xe), int(F.count[k]), g.playerTroopName(int(F.player), k))
		}
		if err := t.PrintCentered(cx, 0xb9-3*9, str); err != nil {
			return err
		}
		if anim != nil {
			if err := g.drawBottomCentered(cx, 0xb9-4*9+8, anim); err != nil {
				return err
			}
		}
		shown++
	}
	if shown != c.lastShown {
		g.col.fullDraw = true
		c.lastShown = shown
	}
	return nil
}

// playUnitBeamSound is Play_Unit_Beam_Sound_ (0xb92f8): the side's personal
// weapon (ground bonus +9, at force +0x2d) picks the sound: 0x49 → 9, 0x65
// → 1, 0x80 → 0x13, 0x8a → 0xd, 0x91 → 0xc. Any other plays the caller's
// leftover EBX in the original; the port plays nothing.
func (g *Game) playUnitBeamSound(u *gcUnit) {
	c := &g.gcs
	f := c.A
	if u.side == 1 {
		f = c.B
	}
	if u.side > 1 || f == nil {
		return
	}
	n := map[uint8]int{0x49: 9, 0x65: 1, 0x80: 0x13, 0x8a: 0xd, 0x91: 0xc}[f.bonus[9]]
	if n == 0 {
		g.logOnce("Play_Unit_Beam_Sound_: a weapon without a sound plays EBX's leftover")
		return
	}
	u.beamSnd = g.moxPlaySound(n)
}
