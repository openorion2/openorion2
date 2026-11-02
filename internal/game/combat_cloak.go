package game

import "github.com/openorion2/openorion2/internal/gfx"

// Cloaks and plasma darkening (CMBTDRAW.C, CMBTSPEC.C): Create_Ship_Darken_
// Map_ builds the colour-by-level table, Plasma_Darken_ applies it through
// COMBAT.LBX 0x31's noise frames, and Draw_Cloak_ fades a ship in or out.

const darkenMapRows = 0x100 // _ship_darken_map (0x18c1a8): 16 levels per colour

// createShipDarkenMap is Create_Ship_Darken_Map_ (0x34286): each ramp's
// colours step down by level and drop to 0 below the ramp's start; rows
// 0x50..0x5f and from 0x80 stay 0 (BSS).
func (g *Game) createShipDarkenMap() {
	m := &g.tc.scr.darkenMap
	set := func(c, b int, v byte, floor byte, above bool) {
		m[c*0x10+b] = v
		if (above && v > floor) || (!above && v < floor) {
			m[c*0x10+b] = 0
		}
	}
	for c := 0; c < 0x20; c++ {
		for b := 0; b < 0x10; b++ {
			set(c, b, byte(c-2*b), 0x20, true)
		}
	}
	for _, r := range [][2]int{{0x20, 0x30}, {0x30, 0x40}, {0x60, 0x70}, {0x70, 0x80}} {
		for c := r[0]; c < r[1]; c++ {
			for b := 0; b < 0x10; b++ {
				set(c, b, byte(c-b), byte(r[0]), false)
			}
		}
	}
	for _, r := range [][2]int{{0x40, 0x48}, {0x48, 0x50}} {
		for c := r[0]; c < r[1]; c++ {
			for b := 0; b < 0x10; b++ {
				set(c, b, byte(c-b/2), byte(r[0]), false)
			}
		}
	}
}

// plasmaDarken is Plasma_Darken_ (0x33dc3): bm's pixels darkened by the
// noise frame frame%16, at a strength by kind (1/2 fading by frame/4, 3
// fixed, 4/5 by frame/3).
func (g *Game) plasmaDarken(bm *gfx.Picture, frame, kind int16) {
	sc := g.tc.scr
	p := sc.darkenPlasma
	if p == nil {
		g.logOnce("Plasma_Darken_ before the darken picture is loaded")
		return
	}
	g.setAnimationFrame(p, int(frame%0x10))
	var lvl int16
	switch kind {
	case 0:
		return
	case 1:
		lvl = min(frame/4, 6)
	case 2:
		lvl = max(6-frame/4, 0)
	case 3:
		lvl = 6
	case 4:
		lvl = min(frame/3, 0x14)
	case 5:
		lvl = max(0xc-frame/3, 0)
	default:
		g.logOnce("Plasma_Darken_: a kind past 5 keeps a stale level")
		return
	}
	if lvl == 0 {
		return
	}
	src := p.FrameData()
	dst := bm.Pixels()
	dw, dh := int16(p.Width), int16(p.Height)
	bw, bh := int16(bm.Width), int16(bm.Height)
	for y := int16(0); y < bh; y++ {
		if y >= dh {
			continue
		}
		di := int(int16(int32(y) * int32(bw)))
		si := int(int16(int32(y) * int32(dw)))
		for x := int16(0); x < bw; x, di, si = x+1, di+1, si+1 {
			if x >= dw || si >= len(src) || di >= len(dst) {
				continue
			}
			var v uint32
			if lvl < 5 {
				v = uint32(int32(lvl)*int32(src[si])) >> 2
			} else {
				v = uint32(int32(src[si]) + int32(lvl) - 4)
			}
			if v > 0xf {
				v = 0xf
			}
			dst[di] = sc.darkenMap[int(dst[di])<<4+int(v)]
		}
	}
}

// drawCloak is Draw_Cloak_ (0xacb4a): record i fading through Plasma_Darken_
// of kind over 24 (kinds 1, 2), 60 (4) or 36 (5) frames.
func (g *Game) drawCloak(i, kind int16) {
	tc := g.tc
	sc := tc.scr
	if sc == nil || !g.shipOnScreen(i) {
		return
	}
	var frames int16
	switch kind {
	case 1, 2:
		frames = 0x18
	case 4:
		frames = 0x3c
	case 5:
		frames = 0x24
	default:
		g.logOnce("Draw_Cloak_: kind 3 keeps a stale frame count")
		return
	}
	e := g.e
	T := g.tsp(i)
	x := (int16(T.u8(tsX)) - tc.curX) * 0x14
	y := (int16(T.u8(tsY)) - tc.curY) * 0x14
	sc.dontDrawShip = i
	step := int16(1)
	if tc.fastFX() {
		step = 2
	}
	g.moxPlaySound(0xa) // 0xacc28, never faded
	for f := int16(0); f < frames; f += step {
		e.MarkTime()
		e.V.SetPageOff()
		g.must2(g.drawMainCombatScreen())
		g.must2(g.drawShipToBitmap(i, kind, f))
		dx, dy := g.getDrawShipOffsets(i)
		e.V.SetWindow(0, 0, 0x27f, 0x15e)
		e.V.ClipOn()
		g.must2(e.Draw(int(x)+dx, int(y)+dy, sc.drawShipBitmap))
		e.V.ClipOff()
		e.V.ResetWindow()
		g.must2(e.TogglePages())
		g.must2(e.ReleaseTime(1))
	}
	sc.dontDrawShip = -1
}
