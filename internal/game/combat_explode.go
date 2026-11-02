package game

import (
	"strings"

	"github.com/openorion2/openorion2/internal/gfx"
)

// The drawing of ships blowing up (CMBTSPEC.C): Destroy_Ship_FX_'s
// explosions and Do_Spherical_SFX_'s blast frames. Both set the dying
// record's +0x24 part way through, so the frame loops run whenever the
// screen would animate, with or without a screen to draw on.

// sphereHit is a ship in Do_Spherical_SFX_'s radius: its record, its ring
// (distance / 20) and squared distance.
type sphereHit struct {
	j    int16
	ring int16
	d2   int32
}

// sphericalSFXFrames is Do_Spherical_SFX_'s frame loop (0xae101..0xae615):
// the source ship (collapsing for kinds 3 and 4), the ships in the shock
// wave shaken by two Random(5) each, and the blast picture p.
func (g *Game) sphericalSFXFrames(i, x, y, kind int16, p *gfx.Picture, list []sphereHit) {
	tc := g.tc
	sc := tc.scr
	e := g.e
	r := &g.state.Random
	before := kind <= 2 // [ebp-4]: the source is drawn after the blast
	vx := x - tc.curX*0x14
	vy := y - tc.curY*0x14
	if sc != nil {
		sc.dontDrawShip = i
		e.V.SetPageBack()
		g.must2(g.drawMainCombatScreen())
		e.V.CopyBackToOff()
	}
	step := int16(1)
	if tc.fastFX() {
		step = 2
	}
	source := func(f int16) {
		T := g.tsp(i)
		if T.u8(tsGone) == 5 {
			return
		}
		switch kind {
		case 0, 1, 2:
			if sc != nil {
				g.must2(g.drawShip(i, int(vx), int(vy)))
			}
		case 3:
			g.drawShipCollapsing(i, vx, vy, 100-f*10, f)
		case 4:
			if before {
				g.drawShipCollapsing(i, vx, vy, 100-(f-4)*10, f)
			} else {
				g.drawShipCollapsing(i, vx, vy, 100-(f-4)*8, f)
			}
		}
	}
	frames := int16(p.Frames)
	for f := int16(0); f <= frames; f += step {
		if sc != nil {
			e.V.ResetWindow()
			e.MarkTime()
			e.V.SetPageOff()
			e.V.SetWindow(0, 0, 0x27f, 0x15e)
			e.V.ClipOn()
		}
		if !before {
			source(f)
		}
		R := int32(g.exeI16(uint32(0xab756 + 2*(int32(f)+int32(kind)*0x19))))
		R2 := R * R
		for _, h := range list {
			J := g.tsp(h.j)
			size := int32(J.u8(tsSize)) * 0x19
			shake := false
			if kind <= 2 {
				shake = (f < 5 && h.ring == 1) || (f > 0 && f < 7 && h.ring == 2) ||
					(f > 4 && f < 0xd && h.ring == 3) || (f > 8 && h.ring == 4)
			}
			if !shake && kind != 1 && kind != 0 {
				shake = (h.d2 <= R2+size && R < 0x55) || (h.d2 <= R2+size && R > 0x54 && h.d2 <= R2-0x271)
			}
			jx := (int16(J.u8(tsX)) - tc.curX) * 0x14
			jy := (int16(J.u8(tsY)) - tc.curY) * 0x14
			if shake {
				jx += int16(r.Random(5)) - 3
				jy += int16(r.Random(5)) - 3
			}
			if sc != nil {
				g.must2(g.drawShip(h.j, int(jx), int(jy)))
			}
		}
		if sc == nil {
			continue
		}
		if kind != 2 {
			if kind < 2 {
				e.RestoreGlassRemapBlock(sc.pulsarRemap)
			} else {
				e.RestoreGlassRemapBlock(sc.warpCoreRemap)
			}
		}
		g.setAnimationFrame(p, int(f))
		cx, cy := g.shipCenterOffsets(i)
		g.must2(g.drawCentered(int(vx+cx), int(vy+cy), p))
		if kind != 2 {
			e.RestoreGlassRemapBlock(sc.shieldRemap)
		}
		if before {
			source(f)
		}
		e.V.ClipOff()
		e.V.ResetWindow()
		g.must2(e.TogglePages())
		g.must2(e.ReleaseTime(1))
	}
	if sc != nil {
		sc.dontDrawShip = -1
	}
}

// drawShipCollapsing is Draw_Ship_Collapsing_ (0xadd40): record i turned
// by frame and shrunk to scale percent around its centre; at a scale of
// one or less it is gone (+0x24 = 5).
func (g *Game) drawShipCollapsing(i, x, y, scale, frame int16) {
	if scale >= 100 {
		scale = 99
	}
	T := g.tsp(i)
	if scale <= 1 {
		T.set8(tsGone, 5)
		return
	}
	sc := g.tc.scr
	if sc == nil {
		return
	}
	sc.combatBitmap = gfx.NewBlankBitmap(0x62, 0x62)
	sc.drawShipBitmap = gfx.NewBlankBitmap(0x62, 0x62)
	T.set8(tsFacing, T.u8(tsFacing)+uint8(frame))
	err := g.drawShipToBitmap(i, 0, 0)
	T.set8(tsFacing, T.u8(tsFacing)-uint8(frame))
	g.must2(err)
	g.must2(sc.combatBitmap.ReduceBitmap(sc.drawShipBitmap, int(scale), int(scale)))
	cx, cy := g.shipCenterOffsets(i)
	g.must2(g.drawCentered(int(x+cx), int(y+cy), sc.combatBitmap))
}

// destroyShipFX is Destroy_Ship_FX_ (0xaec7b): monsters (builder > 9) play
// their death strip, kind 1 a plain explosion (the record gone half way),
// other kinds the spherical blast with its damage.
func (g *Game) destroyShipFX(i, x, y, kind int16) {
	tc := g.tc
	sc := tc.scr
	e := g.e
	T := g.tsp(i)
	if sc != nil && !g.shipCompletelyOnScreen(i) {
		e.MarkTime()
		g.must2(g.snapCenterCombatScreen(x/0x14, y/0x14))
		g.must2(e.ReleaseTime(7))
	}
	switch {
	case T.u8(tsBuilder) > 9:
		g.monsterDeathFX(i, x, y)
	case kind != 1:
		g.doSphericalSFX(i, x, y, kind, g.damageFromShipExploding(i))
		T.set8(tsGone, 5)
		return
	default:
		g.shipExplosionFX(i, x, y)
	}
	g.assignCombatGrids(tc.cur)
	if sc != nil {
		g.must2(g.refreshCombatScreenFull())
	}
}

// shipExplosionFX is Destroy_Ship_FX_'s kind 1 (0xaed10..0xaef5f): CMBTSFX
// 3..5 by size over the ship, which is gone from the middle frame on.
func (g *Game) shipExplosionFX(i, x, y int16) {
	tc := g.tc
	sc := tc.scr
	e := g.e
	T := g.tsp(i)
	ox, oy := g.shipCenterOffsets(i)
	var entry int
	switch sz := T.u8(tsSize); {
	case sz == 0:
		ox, oy, entry = ox-0xc, oy-9, 3
	case sz <= 2:
		ox, oy, entry = ox-0x17, oy-0x14, 4
	case sz <= 5:
		ox, oy, entry = ox-0x2f, oy-0x2c, 5
	default:
		g.logOnce("Destroy_Ship_FX_: a ship size past 5 keeps a stale picture")
	}
	p, err := g.cached(g.cmbtLBX(cmbtSfxLBX), entry)
	g.must2(err)
	frames := int16(p.Frames)
	px := x - tc.curX*0x14 + ox
	py := y - tc.curY*0x14 + oy
	var saved int16
	if sc != nil {
		saved = sc.dontDrawShip
		sc.dontDrawShip = i
	}
	T.set16(0x43, 0)
	if sc != nil {
		e.V.SetPageBack()
		g.must2(g.fullDrawCombatScreen())
		e.V.CopyBackToOff()
	}
	step := int16(1)
	if tc.fastFX() {
		step = 2
	}
	g.playShipExplosionSFX(int16(T.u8(tsSize)))
	for f := int16(0); f < frames; f += step {
		if f >= frames/2 {
			T.set8(tsGone, 5)
		}
		if sc == nil {
			continue
		}
		e.MarkTime()
		e.V.SetPageOff()
		g.must2(g.drawMainCombatScreen())
		e.V.SetWindow(0, 0, 0x27f, 0x15e)
		e.V.ClipOn()
		if T.u8(tsGone) == 0 {
			g.must2(g.drawShip(i, int(x-tc.curX*0x14), int(y-tc.curY*0x14)))
		}
		g.setAnimationFrame(p, int(f))
		g.must2(e.Draw(int(px), int(py), p))
		e.V.ClipOff()
		e.V.ResetWindow()
		g.must2(e.TogglePages())
		g.must2(e.ReleaseTime(1))
	}
	if sc != nil {
		sc.dontDrawShip = saved
	}
}

// monsterDeathFX is Destroy_Ship_FX_'s monster branch (0xaef91..0xaf29c):
// sixteen frames of the monster's death strip (MONSTER entries by builder
// and size) in its facing.
func (g *Game) monsterDeathFX(i, x, y int16) {
	tc := g.tc
	sc := tc.scr
	e := g.e
	T := g.tsp(i)
	facing := int(T.u8(tsFacing))
	dx, dy := g.getDrawShipOffsets(i)
	px := int(x-tc.curX*0x14) + dx
	py := int(y-tc.curY*0x14) + dy
	T.set16(0x43, 0)
	T.set8(tsGone, 5)
	if sc == nil {
		return
	}
	e.V.SetPageBack()
	g.must2(g.fullDrawCombatScreen())
	step := 1
	if tc.fastFX() {
		step = 2
	}
	var tab [0x20]byte
	for k := range tab {
		tab[k] = byte(0x60 + k)
	}
	var p *gfx.Picture
	for f := 0; f < 0x10; f += step {
		e.MarkTime()
		e.V.SetPageOff()
		g.must2(g.drawMainCombatScreen())
		e.V.SetWindow(0, 0, 0x27f, 0x15e)
		e.V.ClipOn()
		// The jump table at 0xaec57: builders 10..14.
		entries := [5][2]int{{0x47, 0x48}, {0x49, 0x4a}, {0x45, 0x46}, {0x4b, 0x4c}, {0x4d, 0x4e}}
		if b := int(T.u8(tsBuilder)) - 10; b >= 0 && b < 5 {
			n := entries[b][0]
			if T.u8(tsSize) == 2 {
				n = entries[b][1]
			}
			var err error
			p, err = g.cached(strings.ToUpper(g.eStr(0xe0)), n)
			g.must2(err)
		}
		if p == nil {
			g.logOnce("Destroy_Ship_FX_: a monster builder past 14 has no death strip")
		} else {
			g.shipFrameBitmaps(p, facing, 0x10, f, false)
		}
		if int16(T.u8(tsOwner)) == tc.def {
			sc.drawShipBitmap.ReplaceColorRange(tab[:], 0x20, 0x20)
		}
		g.must2(e.Draw(px, py, sc.drawShipBitmap))
		e.V.ClipOff()
		e.V.ResetWindow()
		g.must2(e.TogglePages())
		g.must2(e.ReleaseTime(1))
	}
	e.V.SetPageOff()
	g.must2(g.drawMainCombatScreen())
	g.must2(e.TogglePages())
}

// playShipExplosionSFX is Play_Ship_Explosion_SFX_ (0xaec0b): by the hull
// size plus Random(3)-1 (jump table 0xaebf3): 0 sound 0x1d, 5 sounds 0x1e
// and 0x2b, anything else 0x18.
func (g *Game) playShipExplosionSFX(size int16) {
	switch uint16(size + int16(g.state.Random.Random(3)) - 1) {
	case 0:
		g.moxPlaySound(0x1d)
	case 5:
		g.moxPlaySound(0x1e)
		g.moxPlaySound(0x2b)
	default:
		g.moxPlaySound(0x18)
	}
}
