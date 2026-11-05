package game

import (
	"encoding/binary"
	"strings"

	"github.com/openorion2/openorion2/internal/engine"
	"github.com/openorion2/openorion2/internal/font"
	"github.com/openorion2/openorion2/internal/gfx"
)

// The tactical combat screen (COMBINIT.C, COMBAT1.C, CMBTDRW1.C): the
// pictures Tactical_Combat_ loads, the fields, the drawing. The logic is in
// turn_tac*.go; tc.scr is nil when the engine runs without a screen.
// Spec: docs/screens/combat-screen.md.

// LBX names and colour tables of the combat modules, read from the image.
const (
	cmbtCombatLBX   = 0x178e34 // "combat.lbx"
	cmbtMislLBX     = 0x178e3f // "cmbtmisl.lbx"
	cmbtSfxLBX      = 0x178e4c // "cmbtsfx.lbx"
	cmbtSpherLBX    = 0x178e58 // "sphersfx.lbx"
	cmbtShipsLBX    = 0x178e65 // "cmbtshp.lbx": the battle's ship pictures
	cmbtMonsterLBX  = 0x178e71 // "monster.lbx"
	cmbtFgtrLBX     = 0x178e7d // "cmbtfgtr.lbx"
	cmbtPlntLBX     = 0x178e92 // "cmbtplnt.lbx"
	cmbtBeamsLBX    = 0x1829db // _BEAMS_LBX
	cmbtBoardLBX    = 0x178cca // "combat.LBX" of CMBTFIR2.C
	cmbtBoardEsc    = 0x178cd5 // Board_Popup_'s Esc hot key
	cmbtGreen       = 0x17d879 // _combat_green
	cmbtArcLabels   = 0x178ca1 // the shield arc letters, Italian (language 4)
	cmbtArcLabelsEn = 0x178ca9 // the others
	cmbtSpace       = 0x178c52 // " "
	cmbtAmmoSep     = 0x178c54 // the "x" before the ammunition
	cmbtArcOpen     = 0x178c57 // " (" before the arc
	cmbtArcAll      = 0x178c5a // the all-round arc letters
	cmbtArcClose    = 0x178c5e // ")"
	cmbtColonyBreak = 0x178c4f // the line break after a colony line
	cmbtHelpList    = 0x192c80 // _help_entry_list (10-byte records)
	gridLegalBox    = 2000
	combatScreenW   = 0x20 // squares across the view
	combatScreenH   = 0x12
)

// combatScreen is the screen's state: CMBTDRW1.C and COMBAT1.C globals.
type combatScreen struct {
	engineSound                                                        int // 0x182760: Start_Ship_Engine_SFX_'s handle
	levelSymbol, scanBtn, boardBtn, autoBtn, fleeBtn, doneBtn, waitBtn *gfx.Picture
	infoBtn, option, selfDestruct, weaponsBtn, specialsBtn             *gfx.Picture
	mslWarningOn, warningMissile, warningFighter                       *gfx.Picture
	msl                                                                [0x70]*gfx.Picture
	shipCursor                                                         [3]*gfx.Picture
	shieldDiv                                                          [4]*gfx.Picture
	shieldMask                                                         [4][4]*gfx.Picture
	shipBitmap, drawShipBitmap, combatBitmap                           *gfx.Picture
	planet, planet2, mapPlanet                                         *gfx.Picture
	darkenPlasma, tractorPic                                           *gfx.Picture
	darkenMap                                                          [darkenMapRows * 0x10]byte
	shipPics                                                           []*gfx.Picture // what record +0xc6 points to (1-based)
	fighter                                                            [9][0x40]*gfx.Picture

	mouseGrid []engine.MouseRegion // _combat_mouse_grid, 0x36e entries

	savedAuto                                                         engine.AutoFunction
	savedGlass, shieldRemap, pulsarRemap, warpCoreRemap, tractorRemap *engine.GlassBlock
	bhgRemap, plasmaWebRemap, converterRemap                          *engine.GlassBlock

	clockFrame     int16 // _combat_clock_frame
	lastShipStatus int16 // _last_ship_status
	scanned        int16 // _scanned_ship (0x1998d2)
	viewing        bool  // port-only: in the scan's ship view, for tests
	statusDisplay  int16 // _ship_status_display (0 weapons, 1 specials)
	dontDrawShip   int16 // _dont_draw_ship
	activeMissiles int16 // _draw_active_missiles
	shipFrame      int16 // _ship_frame
	gridFlag       int16 // _grid_flag (MOX.SET)
	legalBoxes     int16 // _draw_legal_boxes_flag (MOX.SET)
	autoBtnValue   int16 // __auto_cmbt_button_value
	farX, farY     int16 // _combat_far_x/y: the three star layers' offsets
	midX, midY     int16
	nearX, nearY   int16
	missileCount   int16 // _cur_missile_count

	// Fields (Add_Tactical_Combat_Fields_).
	missileField                                          [nMissiles]int
	takeControlBtn, spaceField, mapField, mslWarningBtn   int
	speedx2Btn, legalBoxesBtn, mslImpactBtn, selfDestBtn  int
	shieldsBtn, gridBtn, scanField, boardField, fleeField int
	waitField, doneField, endOfTurnBtn, centerBtn         int
	weaponsField, specialsField, infoField, superFastBtn  int
	autoField                                             int
	weaponBtn                                             [8]int
	xpos, ypos, mapX, mapY                                int
	humanInput                                            bool  // port-only: in a human ship's Get_Input, for tests
	savedShields                                          int16 // Move_Ship_'s copy of _shields_flag
	board                                                 *boardPopup
}

func (g *Game) cmbtLBX(addr uint32) string { return strings.ToUpper(cStringAt(g.exe, addr)) }

// combatGreen is _combat_green.
func (g *Game) combatGreen() font.Colors { return g.exeColors(cmbtGreen) }

// combatScreenLoad is the screen half of Tactical_Combat_ (0x47939) before
// the ships are loaded: the fade, the pictures and the mouse grid.
func (g *Game) combatScreenLoad() error {
	e := g.e
	tc := g.tc
	sc := &combatScreen{clockFrame: 0xf, lastShipStatus: -2, dontDrawShip: -1, activeMissiles: 1}
	tc.scr = sc
	sc.gridFlag = int16(g.settings[0xb8])
	sc.legalBoxes = int16(g.settings[0xb5])
	sc.savedAuto = e.GetAutoFunction()
	e.V.ResetWindow()
	g.playCombatMusic() // 0x47e18
	e.MoxUpdate()
	e.ClearFields()
	e.DeactivateAutoFunction()
	e.SetMouseList([]engine.MouseRegion{{X2: 0x27f, Y2: 0x1df}})
	if err := e.FastFadeOut(); err != nil {
		return err
	}
	if err := e.LoadPalette(3, 0, 0xff); err != nil {
		return err
	}
	e.SetFadePercent(0)
	lbx := g.cmbtLBX(cmbtCombatLBX)
	var err error
	load := func(n int) *gfx.Picture {
		if err != nil {
			return nil
		}
		var p *gfx.Picture
		p, err = g.picture(lbx, n)
		return p
	}
	sc.levelSymbol = load(0x2a)
	sc.scanBtn = load(1)
	sc.boardBtn = load(0x33)
	sc.autoBtn = load(0x32)
	sc.fleeBtn = load(0x35)
	sc.doneBtn = load(0x36)
	sc.waitBtn = load(0x34)
	sc.infoBtn = load(10)
	sc.option = load(2)
	sc.selfDestruct = load(3)
	sc.weaponsBtn = load(7)
	ml := g.cmbtLBX(cmbtMislLBX)
	for i := range sc.msl {
		if err == nil {
			sc.msl[i], err = g.picture(ml, i)
		}
	}
	sc.specialsBtn = load(8)
	sc.mslWarningOn = load(4)
	sc.warningMissile = load(0x25)
	sc.warningFighter = load(0x26)
	for i := range sc.shipCursor {
		sc.shipCursor[i] = load(0x20 + i)
	}
	sc.shipBitmap = gfx.NewBlankBitmap(0x62, 0x62)
	sc.drawShipBitmap = gfx.NewBlankBitmap(0x62, 0x62)
	sc.combatBitmap = gfx.NewBlankBitmap(0x62, 0x62)
	for i, n := range [4]int{0xe, 0xf, 0xc, 0xd} {
		sc.shieldDiv[i] = load(n)
	}
	for i := range sc.shieldMask {
		for j := range sc.shieldMask[i] {
			sc.shieldMask[i][j] = load(0x10 + 4*i + j)
		}
	}
	if err != nil {
		return err
	}
	// The mouse grid: a region per square, row by row of columns.
	sc.mouseGrid = make([]engine.MouseRegion, 0x36e)
	sc.mouseGrid[0] = engine.MouseRegion{Image: 2, Center: 0xc, X1: 0, Y1: 0, X2: 0x27f, Y2: 0x15e}
	k := 1
	for x := 0; x < 0x20; x++ {
		for y := 0; y < 0x12; y++ {
			sc.mouseGrid[k] = engine.MouseRegion{X1: x * 0x14, Y1: y * 0x14, X2: x*0x14 + 0x13, Y2: y*0x14 + 0x13}
			k++
		}
	}
	sc.mouseGrid[0x36d] = engine.MouseRegion{Image: 1, Center: 0, X1: 0, Y1: 0x15f, X2: 0x27f, Y2: 0x1df}
	return nil
}

// combatShipPalette is Load_Combat_Ship_Palette_ (0x49f99): the ship
// colours of player p go to 0x20 (attacker) or 0x60; the defending colony's
// small map picture takes its banner colours.
func (g *Game) combatShipPalette(p int16) error {
	s := &g.state
	tc := g.tc
	var file string
	var entry int
	if p < 8 {
		file, entry = g.cmbtLBX(cmbtShipsLBX), int(s.Players[p].Color)*0x2d+0x2c
	} else {
		file, entry = g.cmbtLBX(cmbtMonsterLBX), int(p)+5
	}
	pal, err := g.picture(file, entry)
	if err != nil {
		return err
	}
	if p == tc.def && p < 9 && (tc.colony >= 0 || p == 8) {
		size, banner := 2, 2
		if p != 8 {
			banner = int(s.Players[p].Color)
			size = int(s.Planets[s.Colonies[tc.colony].Planet].Size)
		}
		if size != 0 {
			if tc.scr.mapPlanet, err = g.picture(g.cmbtLBX(cmbtCombatLBX), banner*4+size+0x36); err != nil {
				return err
			}
			pal.ShiftPaletteRange(0x60)
			g.e.DrawPalette(pal)
			var tab [0x10]byte
			for i := range tab {
				tab[i] = byte(0x60 + i)
			}
			tc.scr.mapPlanet.ReplaceColorRange(tab[:], 0x50, 0x10)
		}
	}
	at := 0x60
	if tc.att == p {
		at = 0x20
	}
	pal.ShiftPaletteRange(at)
	g.e.DrawPalette(pal)
	return nil
}

// combatScreenRemaps is the part of Tactical_Combat_ after the ships are
// loaded: palettes, the effect remap tables, the ship pictures. The flags
// say which special weapons are in the battle.
func (g *Game) combatScreenRemaps(darken, reflection, tractor, bhg, web, converter bool) error {
	e := g.e
	tc := g.tc
	sc := tc.scr
	sc.savedGlass = e.SaveGlassRemapBlock()
	if g.doesCombatPlanetHaveDefenses()&4 != 0 {
		converter = true
	}
	if err := g.combatShipPalette(tc.att); err != nil {
		return err
	}
	if err := g.combatShipPalette(tc.def); err != nil {
		return err
	}
	if darken {
		p, err := g.picture(g.cmbtLBX(cmbtCombatLBX), 0x31)
		if err != nil {
			return err
		}
		sc.darkenPlasma = p
		g.createShipDarkenMap()
	}
	if err := g.loadCombatBeamStuff(); err != nil {
		return err
	}
	if reflection {
		p, err := g.cached(g.cmbtLBX(cmbtBeamsLBX), 0x59)
		if err != nil {
			return err
		}
		e.DrawPalette(p)
		e.RefreshPalette()
	}
	pal := &e.V.Palette
	remap := func(colors func(i int) (int, int, int, int), from, to int, gradient bool) *engine.GlassBlock {
		for i := from; i < to; i++ {
			r, gg, b, pct := colors(i)
			e.SetGlassRemapColor(i, r, gg, b, pct)
		}
		if gradient {
			e.SetPaletteGradient(0x90, 0x20, 0, 0, 0, 0, 0, 0)
		}
		e.V.SetPaletteChanges(0, 0xff)
		e.UpdateGlassRemapColors()
		return e.SaveGlassRemapBlock()
	}
	fromPal := func(base, step, pct0, pctStep int) func(i int) (int, int, int, int) {
		return func(i int) (int, int, int, int) {
			c := pal[base+i*step]
			return int(c.R), int(c.G), int(c.B), pct0 + pctStep*(i-1)
		}
	}
	sc.shieldRemap = remap(func(i int) (int, int, int, int) {
		return i*2 + 0xd, i*2 + 0x1e, 0x3f, 0x17 + 3*(i-1)
	}, 1, 0x10, true)
	sfx := g.cmbtLBX(cmbtSfxLBX)
	p, err := g.cached(sfx, 0x34)
	if err != nil {
		return err
	}
	e.DrawPalette(p)
	sc.pulsarRemap = remap(fromPal(0x90, 1, 0x35, 3), 1, 0x10, false)
	if p, err = g.cached(g.cmbtLBX(cmbtSpherLBX), 0); err != nil {
		return err
	}
	e.DrawPalette(p)
	sc.warpCoreRemap = remap(fromPal(0x90, 1, 0x35, 3), 1, 0x10, false)
	if tractor {
		if sc.tractorPic, err = g.picture(sfx, 2); err != nil {
			return err
		}
		sc.tractorRemap = remap(func(i int) (int, int, int, int) {
			c := pal[0x50+i/2]
			return int(c.R), int(c.G), int(c.B), 0x17 + 3*(i-1)
		}, 1, 0x10, false)
	}
	if bhg {
		if p, err = g.cached(sfx, 0x2e); err != nil {
			return err
		}
		e.DrawPalette(p)
		sc.bhgRemap = remap(func(i int) (int, int, int, int) {
			c := pal[0x90+i]
			return int(c.R), int(c.G), int(c.B), 0x3c + 2*i
		}, 0, 0x10, true)
	}
	if web {
		sc.plasmaWebRemap = remap(func(i int) (int, int, int, int) {
			c := pal[0xb0+i]
			return int(c.R), int(c.G), int(c.B), min(i*6+0x1e, 100)
		}, 1, 0x10, false)
	}
	if converter {
		if p, err = g.cached(sfx, 0x28); err != nil {
			return err
		}
		e.DrawPalette(p)
		sc.converterRemap = remap(fromPal(0x90, 2, 0x35, 3), 1, 0x10, false)
		e.SetPaletteGradient(0x90, 0x20, 0, 0, 0, 0, 0, 0)
	}
	return nil
}

// combatEffectFlags are the flags Tactical_Combat_ gathers while it loads
// the ships: which effects need their pictures and remap tables.
func (g *Game) combatEffectFlags() (darken, reflection, tractor, bhg, web, converter bool) {
	tc := g.tc
	for i := int16(1); i < tc.n; i++ {
		t := g.tsp(i)
		if t.i16(tsID) < 0 {
			continue
		}
		for k := 0; k < 8; k++ {
			switch t.i16(tw(k, twWeapon)) {
			case 0x23, 0x2d:
				web = true
			case 0x25:
				bhg = true
			case 0x26:
				converter = true
			case 0x27:
				tractor = true
			}
		}
		if g.combatShipHasSpecial(i, 6) || g.combatShipHasSpecial(i, 0x17) {
			darken = true
		}
		if g.combatShipHasSpecial(i, 0x1a) {
			reflection = true
		}
	}
	return
}

// combatShipPictures is Load_Individual_Ship_Pictures_ (0x4a12a) for both
// sides: fighters and one picture per ship picture number, recoloured for
// the defender; then the shield remap is back in place.
func (g *Game) combatShipPictures() error {
	s := &g.state
	tc := g.tc
	sc := tc.scr
	for _, p := range []int16{tc.att, tc.def} {
		var low, high [0x20]byte
		for k := range low {
			low[k], high[k] = byte(0x20+k), byte(0x60+k)
		}
		if p < 8 {
			for k := 0; k < 0x40; k++ {
				f, err := g.picture(g.cmbtLBX(cmbtFgtrLBX), k)
				if err != nil {
					return err
				}
				if p == tc.att {
					f.ReplaceColorRange(low[:0x10], 0x50, 0x10)
				} else {
					f.ReplaceColorRange(high[:0x10], 0x50, 0x10)
				}
				sc.fighter[p][k] = f
			}
		} else if p == 8 {
			for k := 0; k < 0x10; k++ {
				f, err := g.picture(g.cmbtLBX(cmbtFgtrLBX), 0x40+k)
				if err != nil {
					return err
				}
				if tc.att == 8 {
					f.ReplaceColorRange(low[:0x10], 0x50, 0x10)
				} else {
					f.ReplaceColorRange(high[:0x10], 0x50, 0x10)
				}
				sc.fighter[8][k] = f
			}
		}
		loaded := map[int16]*gfx.Picture{}
		for i := int16(1); i < tc.n; i++ {
			t := g.tsp(i)
			if int16(t.u8(tsOwner)) != p {
				continue
			}
			pic := t.i16(tsPicture)
			if q, ok := loaded[pic]; ok {
				g.setRecordPicture(i, q)
				continue
			}
			var file string
			var entry int
			if b := t.u8(tsBuilder); b < 8 && pic != 0x2c {
				file, entry = g.cmbtLBX(cmbtShipsLBX), int(s.Players[p].Color)*0x2d+int(pic)
			} else {
				file, entry = g.cmbtLBX(cmbtMonsterLBX), int(pic)
				if pic == 0x2c {
					entry = 0x19
				}
			}
			q, err := g.picture(file, entry)
			if err != nil {
				return err
			}
			if p == tc.def {
				q.ReplaceColorRange(high[:], 0x20, 0x20)
			}
			loaded[pic] = q
			g.setRecordPicture(i, q)
		}
	}
	g.e.RestoreGlassRemapBlock(sc.shieldRemap)
	return nil
}

// loadCombatColonyPicture is Load_Combat_Colony_Picture_ (0x4b01a): the
// planet's palette and its combat pictures by climate and size.
func (g *Game) loadCombatColonyPicture(ci int16) error {
	s := &g.state
	sc := g.tc.scr
	P := &s.Planets[s.Colonies[ci].Planet]
	lbx := g.cmbtLBX(cmbtPlntLBX)
	pal, err := g.cached(lbx, int(P.Climate)*6+5)
	if err != nil {
		return err
	}
	g.e.DrawPalette(pal)
	if sc.planet, err = g.picture(lbx, int(P.Size)+int(P.Climate)*6); err != nil {
		return err
	}
	if P.Size != 0 {
		sc.planet2, err = g.picture(lbx, int(P.Climate)*6)
		return err
	}
	sc.planet2 = sc.planet
	return nil
}

// combatScreenClose is the end of Tactical_Combat_ with the screen: fade
// out, the galaxy screen's auto function, glass tables and palette back.
func (g *Game) combatScreenClose() error {
	e := g.e
	sc := g.tc.scr
	e.ClearFields()
	e.DeactivateAutoFunction()
	e.DeactivateHelpList()
	e.SetMouseList([]engine.MouseRegion{{X2: 0x27f, Y2: 0x1df}})
	if err := e.FastFadeOut(); err != nil {
		return err
	}
	e.SetPaletteGradient(0xa0, 0x5f, 0, 0, 0, 0, 0, 0)
	e.PutAutoFunction(sc.savedAuto)
	e.FadeoutSound(-1, 0) // 0x48e51: no handle -1, a no-op
	g.playBackgroundMusic()
	g.updateShipIconsInBetweenTurns()
	e.RestoreGlassRemapBlock(sc.savedGlass)
	return e.FadeIntoScreen(g.drawMiniMainScreen, &g.ms.drawToBack, 0)
}

// setRecordPicture stores the record's picture at +0xc6 the way the
// original stores a pointer there: it moves with the record when
// Deploy_Ships_ sorts them.
func (g *Game) setRecordPicture(i int16, p *gfx.Picture) {
	sc := g.tc.scr
	sc.shipPics = append(sc.shipPics, p)
	binary.LittleEndian.PutUint32(g.tsp(i)[0xc6:], uint32(len(sc.shipPics)))
}

// recordPicture is the picture at record +0xc6, nil for none.
func (g *Game) recordPicture(i int16) *gfx.Picture {
	n := binary.LittleEndian.Uint32(g.tsp(i)[0xc6:])
	if n == 0 || int(n) > len(g.tc.scr.shipPics) {
		return nil
	}
	return g.tc.scr.shipPics[n-1]
}
