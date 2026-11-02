package game

import "github.com/openorion2/openorion2/internal/gfx"

// The continuing effects Draw_Ship_ draws over a ship (CMBTSPEC.C, CMBTDRAW.C):
// the energy absorber's glow, the stasis ball, the plasma web and the black
// hole. Their pictures are in CMBTSFX.LBX.

func (g *Game) sfxPic(n int16) *gfx.Picture {
	p, err := g.cached(g.cmbtLBX(cmbtSfxLBX), int(n))
	g.must2(err)
	return p
}

// drawEnergyAbsorber is Draw_Energy_Absorber_ (0x3b3ed): the glow by size
// around record i drawn at (x, y), its frame still for the ship being moved.
func (g *Game) drawEnergyAbsorber(i int16, x, y int) {
	sc := g.tc.scr
	size := int16(g.tsp(i).u8(tsSize))
	n := 0x33 - (size+1)/2
	if size == 4 {
		n--
	}
	p := g.sfxPic(n)
	cx, cy := g.shipCenterOffsets(i)
	f := 1
	if i != sc.dontDrawShip {
		f = (int(sc.shipFrame)/2 + int(i)) % p.Frames
	}
	g.setAnimationFrame(p, f)
	px := x + int(cx) - int(int16(p.Width))/2
	py := y + int(cy) - int(int16(p.Height))/2
	g.must2(g.e.Draw(px, py, p))
}

// drawBHG is Draw_BHG_ (0x3b4c0): the black hole over record i, glassed
// through its remap; frames 8..15 while it still grows (+0x136 != 1).
func (g *Game) drawBHG(i int16, dx, dy int16) {
	tc := g.tc
	sc := tc.scr
	e := g.e
	T := g.tsp(i)
	cx, cy := g.shipCenterOffsets(i)
	x := (int16(T.u8(tsX))-tc.curX)*0x14 + cx + dx
	y := (int16(T.u8(tsY))-tc.curY)*0x14 + cy + dy
	p := g.sfxPic(0x2e - (int16(T.u8(tsSize))+1)/2)
	f := int(sc.shipFrame) / 2 % 8
	if T.u8(0x136) != 1 {
		f += 8
	}
	g.setAnimationFrame(p, f)
	bm := gfx.NewBlankBitmap(1, 1)
	bm.DrawPictureToBitmap(p)
	e.RestoreGlassRemapBlock(sc.bhgRemap)
	bm.SetGlassed()
	_, _, w, h := bm.ActualSize()
	g.must2(e.Draw(int(x)-int(int16(w))/2, int(y)-int(int16(h))/2, bm))
	e.RestoreGlassRemapBlock(sc.shieldRemap)
}

// drawStasisBall is Draw_Stasis_Ball_ (0xac619): the ball by size and
// facing over record i, glassed, scaled to scale percent.
func (g *Game) drawStasisBall(i, scale, dx, dy int16) {
	tc := g.tc
	e := g.e
	T := g.tsp(i)
	cx, cy := g.shipCenterOffsets(i)
	x := (int16(T.u8(tsX))-tc.curX)*0x14 + cx + dx
	y := (int16(T.u8(tsY))-tc.curY)*0x14 + cy + dy
	abs := func(v int16) int16 {
		if v < 0 {
			return -v
		}
		return v
	}
	fac := int16(T.u8(tsFacing))
	v := fac
	switch {
	case fac > 4 && fac < 9:
		v = abs(fac - 8)
	case fac > 8 && fac < 0xd:
		v = (fac + 8) % 0x10
	case fac > 0xc && fac < 0x10:
		v = abs(fac - 0x10)
	}
	bm1 := gfx.NewBlankBitmap(1, 1)
	bm2 := gfx.NewBlankBitmap(1, 1)
	switch sz := T.u8(tsSize); {
	case sz == 0:
		bm1.DrawPictureToBitmap(g.sfxPic(v + 0x35))
	case sz <= 2:
		bm1.DrawPictureToBitmap(g.sfxPic(v + 0x3a))
	case sz == 3:
		bm1.DrawPictureToBitmap(g.sfxPic(v + 0x3f))
	case sz <= 5:
		bm1.DrawPictureToBitmap(g.sfxPic(v + 9))
	default:
		g.logOnce("Draw_Stasis_Ball_: a size past 5 draws a stale buffer")
	}
	switch {
	case fac > 4 && fac < 9:
		bm2.FlipBitmap(bm1, 0)
	case fac > 0xc && fac < 0x10:
		bm2.FlipBitmap(bm1, 3)
	default:
		bm2.DrawPictureToBitmap(bm1)
	}
	out := bm2
	if scale != 100 {
		g.must2(bm1.ReduceBitmap(bm2, int(scale), int(scale)))
		out = bm1
	}
	out.SetGlassed()
	_, _, w, h := out.ActualSize()
	g.must2(e.Draw(int(x)-int(int16(w))/2, int(y)-int(int16(h))/2, out))
}

// drawPlasmaWeb is Draw_Plasma_Web_ (0xad580): the web by size and facing
// over record i drawn at (x, y), glassed through its remap.
func (g *Game) drawPlasmaWeb(i int16, x, y int16) {
	sc := g.tc.scr
	e := g.e
	T := g.tsp(i)
	cx, cy := g.shipCenterOffsets(i)
	x += cx
	y += cy
	fac := int16(T.u8(tsFacing))
	r := fac % 4
	if fac == 4 || fac == 0xc {
		r = 4
	}
	odd := (fac/4)&1 != 0
	pick := func(base int16) int16 {
		if odd {
			return 4 - r + base
		}
		return r + base
	}
	bm := gfx.NewBlankBitmap(1, 1)
	var n int16 = -1
	switch sz := T.u8(tsSize); {
	case sz == 0:
		n = pick(0x1f)
	case sz <= 2:
		n = pick(0x1a)
	case sz == 3:
		n = pick(0x15)
	case sz <= 5:
		// The even quarters of the biggest hulls take size 3's web.
		if odd {
			n = 4 - r + 0x10
		} else {
			n = r + 0x15
		}
	default:
		g.logOnce("Draw_Plasma_Web_: a size past 5 draws a stale buffer")
	}
	if n >= 0 {
		p := g.sfxPic(n)
		g.setAnimationFrame(p, int(sc.shipFrame)%p.Frames)
		bm.DrawPictureToBitmap(p)
	}
	switch fac / 4 {
	case 1:
		bm.FlipBitmap(bm, 0)
	case 3:
		bm.FlipBitmap(bm, 3)
	}
	e.RestoreGlassRemapBlock(sc.plasmaWebRemap)
	x -= int16(bm.Width) / 2
	y -= int16(bm.Height) / 2
	bm.SetGlassed()
	g.must2(e.Draw(int(x), int(y), bm))
	// It leaves through Draw_Moving_Tractor_'s epilogue (0xacf6a), which
	// puts the shield remap back.
	e.RestoreGlassRemapBlock(sc.shieldRemap)
}
