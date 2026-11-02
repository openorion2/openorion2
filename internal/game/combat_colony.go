package game

import (
	"fmt"

	"github.com/openorion2/openorion2/internal/font"
	"github.com/openorion2/openorion2/internal/gfx"
	"github.com/openorion2/openorion2/internal/save"
)

// The human attacker's colony popups (MAINPUPS.C): Colony_Selection_Popup_
// (bombard, invade, mind control, convert or leave) and Colony_Bombing_
// Popup_ (the bombs one by one), both Colony_Combat_Popup_. Spec:
// docs/screens/combat-screen.md, "After the battle".

const (
	ccpTitleColors = 0xc6e5e // Draw_Colony_Combat_Popup_'s font colours
	ccpTextColors  = 0xc6e66 // Colony_Combat_Popup_'s shadowed font colours
	ccpItalianFmt  = 0x179e71
)

// ccpData is the popup's display record (_g_data_ptr, 0x30 bytes).
type ccpData struct {
	colony   int16           // +0x00
	att      int8            // +0x02
	attCount [4]int16        // +0x03
	conv     uint8           // +0x0b a stellar converter is there
	def      int8            // +0x0c
	defCount [4]int16        // +0x0d
	pop      int16           // +0x15
	bldgs    int16           // +0x17
	shield   int16           // +0x19 best shield building, -1 none
	bomb     *bombHits       // +0x1b
	inv      *[2]groundForce // +0x1f, +0x23
	seed     uint32          // +0x27
	est      int16           // +0x2b bomb hits, -1 after a bombing
}

// flicEntry is one _flic_queue record (0xc bytes).
type flicEntry struct {
	x, y  int16
	frame int16
	pic   *gfx.Picture
	once  uint8 // 1: removed after its last frame
	clip  uint8
}

// colonyCombatPopup holds the popup's statics.
type colonyCombatPopup struct {
	d        *ccpData
	state    int16 // _g_state (0x1a1230): 0 selection, 1 bombing
	ctr      int16 // _g_ctr (0x1828cc)
	bg       *gfx.Picture
	invade   *gfx.Picture
	bombard  *gfx.Picture
	convert  *gfx.Picture
	halt     *gfx.Picture
	mind     *gfx.Picture
	noMind   *gfx.Picture
	cancel   *gfx.Picture
	bombIcon *gfx.Picture
	bldgIcon *gfx.Picture
	shield   *gfx.Picture
	explode  *gfx.Picture
	bio      *gfx.Picture
	planet   *gfx.Picture
	planet2  *gfx.Picture
	all      *gfx.Picture
	queue    []flicEntry
	// The field ids (0x1a11d0..0x1a11e8).
	cancelF, invadeF, bombardF, convertF, mindF, allF, haltF, resumeF int
	showing                                                           bool  // port-only: in the loop, for tests
	bombing                                                           int16 // port-only: the loop's bombing state, for tests
}

// updateCCPDisplayData is Update_CCP_Display_Data_Invasion_ (0xca6fb).
func (g *Game) updateCCPDisplayData(d *ccpData, inv *[2]groundForce, seed uint32, ci, est int16, conv uint8) {
	d.inv, d.seed, d.est, d.colony, d.conv = inv, seed, est, ci, conv
	d.att, d.def = int8(inv[0].player), int8(inv[1].player)
	for k := 0; k < 4; k++ {
		d.attCount[k] = inv[0].count[k]
		d.defCount[k] = inv[1].count[k]
	}
	d.pop = int16(g.state.Colonies[ci].PopCount)
	d.bldgs = int16(g.nBldgs(int(ci)))
	d.shield = g.colonyBestShieldBldg(ci)
}

// colonyBestShieldBldg is Get_Colony_Best_Shield_Bldg_ (0xca63e).
func (g *Game) colonyBestShieldBldg(ci int16) int16 {
	b := &g.state.Colonies[ci].Buildings
	r := int16(-1)
	for _, id := range []int16{0x1c, 0x18, 0x17} {
		if b[id] != 0 {
			r = id
		}
	}
	return r
}

// colonySelectionPopup is Colony_Selection_Popup_ (0xca6a6).
func (g *Game) colonySelectionPopup(inv *[2]groundForce, seed uint32, ci, est int16, conv uint8) (int16, error) {
	if err := g.e.GetAutoFunction().Run(); err != nil {
		return 0, err
	}
	if err := g.e.TogglePages(); err != nil {
		return 0, err
	}
	d := &ccpData{}
	g.ccp.d = d
	g.updateCCPDisplayData(d, inv, seed, ci, est, conv)
	return g.colonyCombatPopup(0)
}

// colonyBombingPopup is Colony_Bombing_Popup_ (0xcab21).
func (g *Game) colonyBombingPopup(b *bombHits, inv *[2]groundForce, seed uint32, ci, est int16, conv uint8) (int16, error) {
	d := &ccpData{bomb: b}
	g.ccp.d = d
	g.updateCCPDisplayData(d, inv, seed, ci, est, conv)
	d.defCount[1] -= b.MarinesKilled
	d.defCount[0] -= b.ArmorKilled
	g.mp.explodeOff = true
	return g.colonyCombatPopup(1)
}

// loadColonyCombatPopup is Load_Colony_Combat_Popup_ (0xcac18).
func (g *Game) loadColonyCombatPopup() error {
	c := &g.ccp
	mp := cStringAt(g.exe, 0x182a61)
	col := cStringAt(g.exe, colony2LBX)
	for _, l := range []struct {
		p    **gfx.Picture
		file string
		n    int
	}{
		{&c.convert, mp, 0x35}, {&c.invade, mp, 0x36}, {&c.bombard, mp, 0x37}, {&c.halt, mp, 0x3c},
		{&c.bg, mp, 0x4e}, {&c.all, mp, 0x4f}, {&c.mind, mp, 0x50}, {&c.noMind, mp, 0x51},
		{&c.cancel, mp, 0x52}, {&c.bombIcon, col, 0x2e}, {&c.bldgIcon, col, 0x2f}, {&c.shield, col, 0x30},
		{&c.explode, mp, 0x3d}, {&c.bio, mp, 0x49},
	} {
		p, err := g.picture(l.file, l.n)
		if err != nil {
			return err
		}
		*l.p = p
	}
	return g.loadCombatColonyPictureToScreenSeg(c.d.colony)
}

// loadCombatColonyPictureToScreenSeg is Load_Combat_Colony_Picture_To_
// Screen_Seg_ (0xcab7f): the planet by climate and size, and its palette.
func (g *Game) loadCombatColonyPictureToScreenSeg(ci int16) error {
	c, e := &g.ccp, g.e
	P := &g.state.Planets[g.state.Colonies[ci].Planet]
	lbx := g.eStr(0xdf)
	var err error
	if c.planet, err = g.picture(lbx, int(P.Size)+int(P.Climate)*6); err != nil {
		return err
	}
	if c.planet2, err = g.picture(lbx, (int(P.Climate)+1)*6-1); err != nil {
		return err
	}
	e.DrawPalette(c.planet2)
	e.RefreshPalette()
	e.V.SetPaletteChanges(0, 0xff)
	e.UpdateGlassRemapColors()
	return nil
}

// colonyCombatPopup is Colony_Combat_Popup_ (0xcbad0) for modes 0
// (selection) and 1 (bombing); mode 2, the ground battle, has no caller.
func (g *Game) colonyCombatPopup(mode int16) (int16, error) {
	e, mp, c := g.e, &g.mp, &g.ccp
	d := c.d
	var in, state int16
	done := false
	c.ctr = 0
	mp.lastScanned = -1
	if err := e.LoadPalette(0, 0, 0xff); err != nil {
		return 0, err
	}
	e.V.ClearPaletteChanges(0, 0xff)
	e.Text().SetStyleShadow(1, g.exeColors(ccpTextColors), font.ShadowDown)
	if err := g.loadColonyCombatPopup(); err != nil {
		return 0, err
	}
	c.queue = c.queue[:0]
	if err := g.ccpRemapColors(); err != nil {
		return 0, err
	}
	if err := g.doPopupSetup(int(g.state.Planets[g.state.Colonies[d.colony].Planet].Star), c.bg); err != nil {
		return 0, err
	}
	e.DeactivateAutoFunction()
	e.AssignAutoFunction(g.drawColonyCombatPopup, 1)
	g.ericPushFields()
	if err := g.addColonyCombatFields(mode, state); err != nil {
		return 0, err
	}
	if err := g.drawColonyCombatPopup(); err != nil {
		return 0, err
	}
	if err := e.TogglePages(); err != nil {
		return 0, err
	}
	e.V.SetRefreshStencil()
	var hits, bio int16
	if mode == 1 {
		b := d.bomb
		hits, bio = b.Hits, b.BioHits
		b.MarinesKilled, b.ArmorKilled, b.PopKilled = 0, 0, 0
		b.Destroyed = [49]uint8{}
		b.BioPopKilled, b.Hit, b.Emptied = 0, 0, 0
	}
	g.state.Random.Set(d.seed)
	c.state = mode
	c.bombing = state
	c.showing = true
	defer func() { c.showing = false }()
	result := int16(0)
	for !done {
		mp.fullDraw = true
		g.ericPopFields()
		g.ericPushFields()
		if err := g.addColonyCombatFields(mode, state); err != nil {
			return 0, err
		}
		r, err := e.GetInput()
		if err != nil {
			return 0, err
		}
		in = int16(r)
		if _, err := e.ScanInput(); err != nil {
			return 0, err
		}
		if mode == 1 {
			if err := g.applyNextBombHit(&state, &hits, &bio); err != nil {
				return 0, err
			}
		}
		if result, err = g.evaluateColonyCombatInput(in, &done, &state); err != nil {
			return 0, err
		}
		c.bombing = state
		if err := g.drawColonyCombatPopup(); err != nil {
			return 0, err
		}
		if err := e.TogglePages(); err != nil {
			return 0, err
		}
		c.ctr++
		if err := e.ReleaseTime(1); err != nil {
			return 0, err
		}
	}
	g.doPopupCleanup()
	c.queue = c.queue[:0]
	if g.e != nil {
		g.e.StopAllSounds() // 0xcbea2
	}
	if result == 3 {
		if err := e.FastFadeOut(); err != nil {
			return 0, err
		}
	}
	return result, nil
}

// ccpRemapColors is Calculate_Picture_Remap_Colors (C_Anims_ (0xd)).
func (g *Game) ccpRemapColors() error {
	p, err := g.cAnims(0xd)
	if err != nil {
		return err
	}
	return g.e.CalculatePictureRemapColors(p)
}

// addColonyCombatFields is Add_Colony_Combat_Fields_ (0xcadc8): CLOSE, the
// bombing buttons (mode 1), then from the bottom up bombard, invade, mind
// control and the stellar converter, and the whole screen.
func (g *Game) addColonyCombatFields(mode, state int16) error {
	e, mp, c := g.e, &g.mp, &g.ccp
	d := c.d
	x := mp.x + 0xdd
	y := mp.y + int(int16(c.bg.Height)) - 0x27
	add := func(f *int, row int, pic *gfx.Picture, key int) error {
		n, err := e.AddButtonField(x, y+row*-0x19, g.eStr(0xc), pic, g.eStr(key), 0x28)
		*f = n
		return err
	}
	row := 1
	if err := add(&c.cancelF, 0, c.cancel, 0x87); err != nil {
		return err
	}
	if mode == 1 {
		if d.est < 0 || state == 0 || state == 3 || d.pop < 1 || state == 2 {
			c.resumeF = -1000
		} else {
			n, err := e.AddButtonField(x, y-0x19, g.eStr(0xc), c.bombard, g.eStr(0xc1), 0x28)
			if err != nil {
				return err
			}
			c.resumeF = n
			row = 2
		}
		if d.est < 1 || state == 1 || state == 3 || d.pop < 1 || state == 2 {
			c.haltF = -1000
		} else {
			if err := add(&c.haltF, row, c.halt, 0x13c); err != nil {
				return err
			}
			row++
		}
		if d.est < 1 || state == 2 || state == 3 || d.pop < 1 {
			c.allF = -1000
		} else {
			if err := add(&c.allF, row, c.all, 0xa6); err != nil {
				return err
			}
			row++
		}
	}
	if d.est < 0 || mode == 1 || mode == 2 || d.pop < 1 {
		if c.bombardF != -1000 {
			e.SetInputDelay(3)
		}
		c.bombardF = -1000
	} else {
		if err := add(&c.bombardF, row, c.bombard, 0xc1); err != nil {
			return err
		}
		row++
	}
	if d.attCount[1] < 1 || d.pop < 1 || mode == 2 {
		c.invadeF = -1000
	} else {
		if err := add(&c.invadeF, row, c.invade, 0x147); err != nil {
			return err
		}
		row++
	}
	if g.state.Players[d.att].Picks[save.PickTelepathic] == 0 || mode == 2 || d.pop < 1 {
		c.mindF = -1000
	} else {
		pic := c.noMind
		if g.playerCanMindControlColony(int16(d.att), d.colony) {
			pic = c.mind
		}
		if err := add(&c.mindF, row, pic, 0x16d); err != nil {
			return err
		}
		row++
	}
	if d.conv == 0 || mode == 2 || d.pop < 1 {
		c.convertF = -1000
	} else if err := add(&c.convertF, row, c.convert, 0xfb); err != nil {
		return err
	}
	n, err := e.AddHiddenField(0, 0, 0x27f, 0x1df, g.eStr(0xc), 0)
	mp.hiddenField = n
	return err
}

// evaluateColonyCombatInput is Evaluate_Colony_Combat_Input_ (0xcb88c):
// the choice (-1 close, 1 convert, 2 bombard, 3 invade, 4 mind control),
// the bombing state, and the right-click help.
func (g *Game) evaluateColonyCombatInput(in int16, done *bool, state *int16) (int16, error) {
	c, mp := &g.ccp, &g.mp
	d := c.d
	help := func(title, msg int) error {
		t := g.eStr(title)
		return g.textBox(&t, g.eStr(msg))
	}
	finish := func(r int16) (int16, error) {
		*done = true
		return r, nil
	}
	var err error
	switch {
	case in == int16(c.cancelF):
		return finish(-1)
	case int32(in) == -int32(int16(c.cancelF)):
		err = help(0x182, 0x260)
	case in == int16(c.invadeF):
		return finish(3)
	case int32(in) == -int32(int16(c.invadeF)):
		err = help(0x150, 0x261)
	case in == int16(c.bombardF):
		mp.noFlicker = true
		return finish(2)
	case int32(in) == -int32(int16(c.bombardF)):
		err = help(0xcd, 0x262)
	case in == int16(c.convertF):
		return finish(1)
	case int32(in) == -int32(int16(c.convertF)):
		err = help(0x108, 0x263)
	case in == int16(c.mindF):
		ok, err := g.playerCanMindControlColonyWithHelp(int16(d.att), d.colony)
		if err != nil {
			return 0, err
		}
		if ok {
			return finish(4)
		}
	case int32(in) == -int32(int16(c.mindF)):
		if g.playerCanMindControlColony(int16(d.att), d.colony) {
			err = help(0x175, 0x264)
		} else {
			_, err = g.playerCanMindControlColonyWithHelp(int16(d.att), d.colony)
		}
	case in == int16(c.allF):
		mp.fullDraw = true
		*state = 2
		*done = false
		return 0, nil
	case int32(in) == -int32(int16(c.allF)):
		err = help(0x114, 0x265)
	case in == int16(c.haltF):
		mp.fullDraw = true
		*state = 1
		*done = false
		return 0, nil
	case int32(in) == -int32(int16(c.haltF)):
		err = help(0xd9, 0x266)
	case in == int16(c.resumeF):
		*state = 0
		g.e.SetInputDelay(1)
		g.e.MarkTime()
	default:
		// The original's last test repeats the halt button's negative id,
		// so the resume button's help (E 0x267) never shows.
		*done = false
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	mp.fullDraw = true
	*done = false
	return 0, nil
}

// playerCanMindControlColonyWithHelp is Player_Can_Mind_Control_Colony_With_
// Help_ (0xc613b): as Player_Can_Mind_Control_Colony_, but says why not.
func (g *Game) playerCanMindControlColonyWithHelp(p, ci int16) (bool, error) {
	s := &g.state
	C := &s.Colonies[ci]
	star := int16(s.Planets[C.Planet].Star)
	msg := 0
	switch {
	case s.Players[p].Picks[save.PickTelepathic] == 0:
		msg = 0x233
	case !s.playerHasShipSizeOrLargerAtStar(p, 2, star):
		msg = 0x24c
	case g.playerHasLeaderWithGeneralSkillAtStar(int16(C.Owner), 0, star):
		msg = 0x200
	case s.Players[C.Owner].Picks[save.PickTelepathic] != 0:
		msg = 0x201
	default:
		return true, nil
	}
	return false, g.messageBox(g.eStr(msg))
}

// applyNextBombHit is Apply_Next_Bomb_Hit_ (0xca85c): every fifth frame one
// hit (all of them in state 2), an explosion per point of damage up to five.
func (g *Game) applyNextBombHit(state, hits, bio *int16) error {
	c, mp, s := &g.ccp, &g.mp, &g.state
	d := c.d
	if *state == 1 || *state == 3 || c.ctr%5 != 0 {
		return nil
	}
	ci := int(d.bomb.Colony)
	cy := (int32(mp.y)*2 + int32(int16(c.bg.Height)) - 0x1e) / 2
	cx := (int32(mp.x)*2 + int32(int16(c.bg.Width)) - 0x1e) / 2
	for {
		if int32(*bio)+int32(*hits) < 1 {
			if *state == 2 {
				*bio = 0
				d.est = 0
			}
			*state = 3
		} else {
			if *bio == 0 {
				n := g.resolveBombHit(ci, d.bomb, int32(*hits))
				if n == 0 {
					*hits = 0
				} else {
					if n == int32(*hits) {
						d.est -= *hits
						*hits = 0
						n = 1
					} else {
						*hits -= int16(n)
						d.est -= int16(n)
					}
					half := int32(int16(c.planet.Width)) / 2
					span := half - int32(int16(c.explode.Width))
					if span < 0 {
						span = -span
					}
					for n = min(n, 5); n > 0; n-- {
						y := int16(cy + int32(s.Random.Random(uint32(span*2))) - span)
						x := int16(cx + int32(s.Random.Random(uint32(span*2))) - span)
						c.addFlic(x, y, c.explode, 1, 0)
						g.moxPlaySound(0x18) // 0xca9df
					}
				}
			} else {
				g.resolveBiologicalHit(ci, d.bomb)
				*bio--
				d.est--
				y := int16(cy + int32(s.Random.Random(0x28)) - 0x14)
				x := int16(cx + int32(s.Random.Random(0x28)) - 0x14)
				c.addFlic(x, y, c.bio, 1, 0)
				g.moxPlaySound(0x20) // 0xcaa7f
			}
			b := d.bomb
			g.updateCCPDisplayData(d, d.inv, d.seed, d.colony, d.est, d.conv)
			d.defCount[1] -= b.MarinesKilled
			d.defCount[0] -= b.ArmorKilled
			mp.fullDraw = true
		}
		if *state != 2 {
			return nil
		}
	}
}

// addFlic is Add_Flic_To_End_Of_Queue_ (0xc6cae), at most 100 entries.
func (c *colonyCombatPopup) addFlic(x, y int16, p *gfx.Picture, once, clip uint8) {
	if len(c.queue) < 100 {
		c.queue = append(c.queue, flicEntry{x: x, y: y, pic: p, once: once, clip: clip})
	}
}

// drawFlicQueue is Draw_Flic_Queue_ (0xc6cf7) with Draw_Queue_Entry_
// (0xc6d10): each entry's frame, then the next; one-shot ones end.
func (g *Game) drawFlicQueue() error {
	c, v := &g.ccp, g.e.V
	for k := 0; k < len(c.queue); k++ {
		q := &c.queue[k]
		g.setAnimationFrame(q.pic, int(q.frame))
		if q.clip != 0 {
			v.SetWindow(0, 0, 0x27f, 0x1df)
			v.ClipOn()
		}
		if err := g.e.Draw(int(q.x), int(q.y), q.pic); err != nil {
			return err
		}
		if q.clip != 0 {
			v.ClipOff()
		}
		q.frame++
		switch {
		case q.once == 0 && int(q.frame) >= q.pic.Frames:
			q.frame = 0
		case q.once == 1 && int(q.frame) >= q.pic.Frames:
			c.queue = append(c.queue[:k], c.queue[k+1:]...)
		}
	}
	return nil
}

// drawColonyCombatPopup is Draw_Colony_Combat_Popup_ (0xcb23c): the frame,
// planet and title once, then the colony's buildings, shield, colonists and
// troops on the left, the attacker's troops and the bomb hits on the right.
func (g *Game) drawColonyCombatPopup() error {
	e, mp, c, s := g.e, &g.mp, &g.ccp, &g.state
	d := c.d
	t := e.Text()
	colors := g.exeColors(ccpTitleColors)
	if err := e.ReleaseTime(1); err != nil {
		return err
	}
	e.MarkTime()
	x, y := mp.x, mp.y
	w, h := int(int16(c.bg.Width)), int(int16(c.bg.Height))
	if mp.fullDraw || g.fullDrawScreen {
		if err := g.mainpupsTestedExecuteAutofunction(); err != nil {
			return err
		}
		e.V.Fill(x+5, y+5, w-10, h-10, 0)
		if err := e.Draw(x, y, c.bg); err != nil {
			return err
		}
		if err := g.drawCentered((w+x*2)/2, (y*2+h)/2, c.planet); err != nil {
			return err
		}
		if err := g.ccpRemapColors(); err != nil {
			return err
		}
		t.SetStyle(3, colors)
		var title string
		switch c.state {
		case 0, 1:
			title = cSprintf(g.eStr(0x1ca), g.planetName(int(s.Colonies[d.colony].Planet)))
		case 2:
			title = cSprintf(g.eStr(0x151), g.planetName(int(s.Colonies[d.colony].Planet)))
		}
		if err := t.PrintCentered((w+x*2)/2, y+0x14, title); err != nil {
			return err
		}
		e.V.CopyOffToBack()
		mp.fullDraw = false
	}
	t.SetStyle(1, colors)
	line := func(row int) int { return y + 0x32 + row*0x1e }
	row := 0
	if d.bldgs > 0 {
		if err := g.remapDraw(x+0x14, line(0), c.bldgIcon); err != nil {
			return err
		}
		if err := t.Print(x+0x37, line(0), cSprintf(g.eStr(0x13), d.bldgs)); err != nil {
			return err
		}
		row = 1
	}
	if d.shield != -1 {
		if err := g.remapDraw(x+0x14, line(row), c.shield); err != nil {
			return err
		}
		name, err := g.crBuildingName(d.shield)
		if err != nil {
			return err
		}
		if err := t.Print(x+0x37, line(row), cSprintf(g.eStr(0x46), name)); err != nil {
			return err
		}
		row++
	}
	defRace := int(s.Players[d.def].Race)
	if d.pop > 0 {
		job := int(s.Colonies[d.colony].Colonists[0] >> 7 & 3)
		p, err := g.peopleAnim(job, 2, defRace)
		if err != nil {
			return err
		}
		if err := g.remapDraw(x+0x14, line(row), p); err != nil {
			return err
		}
		if err := t.Print(x+0x37, line(row), cSprintf(g.eStr(0x14), d.pop)); err != nil {
			return err
		}
		row++
	}
	for k := 1; k >= 0; k-- {
		if d.defCount[k] <= 0 {
			continue
		}
		p, err := g.playerTroopAnim(int(d.def), k, defRace)
		if err != nil {
			return err
		}
		if err := g.remapDraw(x+0x14, line(row), p); err != nil {
			return err
		}
		name := g.playerTroopName(int(d.def), k)
		var txt string
		if g.settings[0xd2] == 4 {
			txt = cSprintf(cStringAt(g.exe, ccpItalianFmt), name, d.defCount[k])
		} else {
			txt = cSprintf(g.eStr(0xe), d.defCount[k], name)
		}
		if err := t.Print(x+0x37, line(row), txt); err != nil {
			return err
		}
		row++
	}
	right := x + w
	row = 0
	if d.attCount[1] > 0 {
		attRace := int(s.Players[d.att].Race)
		p, err := g.playerTroopAnim(int(d.att), 1, attRace)
		if err != nil {
			return err
		}
		if err := g.remapDraw(right-0x28, line(0), p); err != nil {
			return err
		}
		txt := cSprintf(g.eStr(0xe), d.attCount[1], g.playerTroopName(int(d.att), 1))
		if err := t.PrintRight(right-0x2d, line(0), txt); err != nil {
			return err
		}
		row = 1
	}
	if d.est >= 0 {
		if err := g.remapDraw(right-0x28, line(row), c.bombIcon); err != nil {
			return err
		}
		f := 0x12
		if c.state == 0 || c.state == 2 {
			f = 0x15
		}
		if err := t.PrintRight(right-0x2d, line(row), cSprintf(g.eStr(f), d.est)); err != nil {
			return err
		}
	}
	return g.drawFlicQueue()
}

// playerTroopAnim is Player_Troop_Anim_ (0xbb723): RACEICON.LBX by class
// (0 armour, 1 marines, 2, 3) and the owner's armour or marine technology.
func (g *Game) playerTroopAnim(p, class, race int) (*gfx.Picture, error) {
	if class > 3 || class < 0 {
		return nil, fmt.Errorf("Player_Troop_Anim_: class %d", class)
	}
	pl := &g.state.Players[p]
	n := race*0xd + 7
	switch class {
	case 0:
		n = race*0xd + 9
		if pl.AppStatus[0x18] == 3 {
			n = race*0xd + 10
		}
	case 1:
		if pl.AppStatus[0x90] == 3 {
			n = race*0xd + 8
		}
	case 2:
		n = race*0xd + 6
	}
	return g.cached(cStringAt(g.exe, raceiconLBX), n)
}

// playerTroopName is Player_Troop_Name_ (0xbb695).
func (g *Game) playerTroopName(p, class int) string {
	pl := &g.state.Players[p]
	switch class {
	case 0:
		if pl.AppStatus[0x18] != 3 {
			return g.eStr(0x1f8)
		}
		return g.eStr(0xc3)
	case 1:
		if pl.AppStatus[0x90] != 3 {
			return g.eStr(0x14d)
		}
		return g.eStr(0x1ac)
	case 2:
		return g.eStr(0x174)
	case 3:
		return g.eStr(0x1bc)
	}
	return ""
}
