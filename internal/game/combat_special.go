package game

import (
	"github.com/openorion2/openorion2/internal/engine"
	"github.com/openorion2/openorion2/internal/gfx"
)

// The special weapons' drawing (CMBTSPEC.C, CMBTFIRE.C): tractor beams,
// cloaks, stasis, plasma webs, black holes, energy absorbers. Spec:
// docs/screens/combat-screen.md.

// tractorEnds are the view centres of record i and of the record its
// tractor holds (+0x42); a record that is cur is at the moving pixel.
func (g *Game) tractorEnd(i, cur, px, py int16) (int16, int16) {
	tc := g.tc
	cx, cy := g.shipCenterOffsets(i)
	if i == cur {
		return cx + px - tc.curX*0x14, cy + py - tc.curY*0x14
	}
	T := g.tsp(i)
	return cx + (int16(T.u8(tsX))-tc.curX)*0x14, cy + (int16(T.u8(tsY))-tc.curY)*0x14
}

// tractorBitmap is the tractor picture's frame f glassed through its remap.
func (g *Game) tractorBitmap(f int) *gfx.Picture {
	sc := g.tc.scr
	g.setAnimationFrame(sc.tractorPic, f)
	g.e.RestoreGlassRemapBlock(sc.tractorRemap)
	bm := gfx.NewBlankBitmap(1, 1)
	bm.DrawPictureToBitmap(sc.tractorPic)
	bm.SetGlassed()
	return bm
}

// drawContinuingTractor is Draw_Continuing_Tractor_ (0xaccdb): the beam
// from record i to the record it holds, frame 6.
func (g *Game) drawContinuingTractor(i int16) error {
	sc := g.tc.scr
	j := int16(g.tsp(i).u8(0x42))
	x1, y1 := g.tractorEnd(i, -1, 0, 0)
	x2, y2 := g.tractorEnd(j, -1, 0, 0)
	bm := g.tractorBitmap(6)
	v := g.e.V
	v.SetWindow(0, 0, 0x27f, 0x15e)
	v.ClipOn()
	g.drawBitmapLine(x1, y1, x2, y2, bm)
	v.ClipOff()
	g.e.RestoreGlassRemapBlock(sc.shieldRemap)
	return nil
}

// drawMovingTractor is Draw_Moving_Tractor_ (0xace08): the beam of record
// i while the current ship moves at pixel (px, py), frames 4..6.
func (g *Game) drawMovingTractor(i, px, py int16) {
	tc := g.tc
	sc := tc.scr
	j := int16(g.tsp(i).u8(0x42))
	x1, y1 := g.tractorEnd(i, tc.cur, px, py)
	x2, y2 := g.tractorEnd(j, tc.cur, px, py)
	bm := g.tractorBitmap(int(sc.shipFrame%3) + 4)
	g.drawBitmapLine(x1, y1, x2, y2, bm)
	g.e.RestoreGlassRemapBlock(sc.shieldRemap)
}

// drawBitmapLine is Draw_Bitmap_Line_ (0x130098): bm stretched along the
// line (x1, y1)-(x2, y2) and rotated to it, pixel by pixel into the current
// page within the window; glass colours go through the glass table. The
// pixels come from the byte the frame table holds at the frame's index.
func (g *Game) drawBitmapLine(x1, y1, x2, y2 int16, bm *gfx.Picture) {
	v := g.e.V
	page := v.Current
	dx, dy := int32(x2)-int32(x1), int32(y2)-int32(y1)
	length := int32(isqrt(uint32(dx*dx + dy*dy)))
	if x1 == x2 && y1 == y2 {
		return
	}
	glass := bm.Mask&gfx.MaskGlass != 0
	src := bm.Pixels()
	w, h := int32(int16(bm.Width)), int32(int16(bm.Height))
	cx := (int32(x1) + int32(x2)) / 2
	cy := (int32(y1) + int32(y2)) / 2
	sx1, sy1 := cx-w/2, cy-h/2
	sx2, sy2 := cx+w/2, cy+h/2
	for sx2-sx1 >= w {
		sx2--
	}
	for sy2-sy1 >= h {
		sy2--
	}
	theta := -int32(int16(g.getAngle(int(dx), int(dy))))
	nx1 := sx1 - (length-w)/2
	nx2 := sx2 + (length-w)/2
	scale := length * 100 / w
	bx1, by1, bx2, by2 := int32(0x27f), int32(0x1df), int32(0), int32(0)
	corner := func(ang int32, x, y int32) {
		i, j := x-cx, y-cy
		g.rotateCoordinates(int16(ang), &i, &j)
		i += cx
		j += cy
		bx1, by1 = min(bx1, i), min(by1, j)
		bx2, by2 = max(bx2, i), max(by2, j)
	}
	for _, ang := range [2]int32{theta, -theta} {
		corner(ang, nx1, sy1)
		corner(ang, nx1, sy2)
		corner(ang, nx2, sy1)
		corner(ang, nx2, sy2)
	}
	bx1, by1 = max(bx1, int32(v.WinX1)), max(by1, int32(v.WinY1))
	bx2, by2 = min(bx2, int32(v.WinX2)), min(by2, int32(v.WinY2))
	v.AddSquare(int(bx1), int(by1), int(bx2), int(by2))
	for j := by1; j <= by2; j++ {
		for i := bx1; i <= bx2; i++ {
			rx, ry := i-cx, j-cy
			g.rotateCoordinates(int16(theta), &rx, &ry)
			rx += cx
			ry += cy
			if rx < nx1 || rx > nx2 || ry < sy1 || ry > sy2 {
				continue
			}
			k := (ry-sy1)*w + cx + (rx-cx)*100/scale - sx1
			if k < 0 || int(k) >= len(src) {
				g.logOnce("Draw_Bitmap_Line_ reads past its bitmap")
				continue
			}
			c := src[k]
			o := int(i + j*0x280)
			if c == 0 || o < 0 || o >= len(page) {
				continue
			}
			if glass && c >= 0xf0 {
				page[o] = g.e.GlassRemap(c, page[o])
			} else {
				page[o] = c
			}
		}
	}
}

const vanishTable = 0x189024 // Vanish_Bitmap_Pixels_' 256-byte threshold table

// vanishBitmap is Vanish_Bitmap_ (0x12e550) with Vanish_Bitmap_Pixels_
// (0x147abc): src's pixels whose table byte is below pct turn 0, the table
// walked from a Random(1000) start; 100 and up leaves nothing to draw.
func (g *Game) vanishBitmap(src *gfx.Picture, pct int16) *gfx.Picture {
	w, h := int(int16(src.Width)), int(int16(src.Height))
	if pct >= 100 {
		return gfx.NewBlankBitmap(0, 0)
	}
	in := append(append([]byte(nil), src.Pixels()...), 0, 0, 0)
	out := gfx.NewBlankBitmap(w, h)
	if pct == 0 {
		copy(out.Pixels(), in)
		return out
	}
	tab := g.exe.bytes(vanishTable, 0x104)
	ebx := (g.state.Random.Random(1000) - 1) & 0xfffe
	n := uint32(uint16(w) * uint16(h))
	cnt := n >> 2
	if n&3 != 0 {
		cnt++
	}
	row := uint32(w) >> 2
	left := row
	ebx &= 0xfe
	pix := make([]byte, cnt*4)
	for si := 0; cnt > 0; cnt-- {
		for k := 0; k < 4; k++ {
			c := byte(0)
			if si+k < len(in) {
				c = in[si+k]
			}
			if tab[ebx+uint32(k)] < byte(pct) {
				c = 0
			}
			pix[si+k] = c
		}
		si += 4
		ebx = (ebx + 4) & 0xff
		if left--; left == 0 {
			left = row
			ebx = (ebx + cnt*cnt) & 0xfe
		}
	}
	copy(out.Pixels(), pix)
	return out
}

// addDamageIndicatorForBiological is Add_Damage_Indicator_For_Biological_
// (0xa943a).
func (g *Game) addDamageIndicatorForBiological(x, y int16, right byte, bio int16) {
	d := int16(-8)
	if right != 0 {
		d = 8
	}
	g.addDamageMessageToQueue(x+d, y-8, 0, 0, bio, right)
}

// shipCenterDrawXY is Get_Ship_Center_Draw_XY_ (0x34543).
func (g *Game) shipCenterDrawXY(i int16) (int16, int16) {
	tc := g.tc
	T := g.tsp(i)
	cx, cy := g.shipCenterOffsets(i)
	return cx + (int16(T.u8(tsX))-tc.curX)*0x14, cy + (int16(T.u8(tsY))-tc.curY)*0x14
}

// drawBombs is Draw_Bombs_ (0x33f60): bomb w of record a flies to t (from
// a transporter's beam end when a has special 0x23) and explodes with dmg.
func (g *Game) drawBombs(a, t, w, dmg int16) {
	tc := g.tc
	sc := tc.scr
	e := g.e
	v := e.V
	bio := g.wpnU8(int(w), 0x12)&0x80 != 0
	trans := g.combatShipHasSpecial(a, 0x23)
	step := int16(6)
	if tc.speedx2 != 0 || tc.netFlag != 0 {
		step = 0xc
	}
	if !g.shipCompletelyOnScreen(a) {
		A := g.tsp(a)
		g.must2(g.snapCenterCombatScreen(int16(A.u8(tsX)), int16(A.u8(tsY))))
	}
	tx, ty := g.shipCenterDrawXY(t)
	sx, sy := g.shipCenterDrawXY(a)
	if trans {
		pw, ph := int32(int16(sc.planet.Width)/2), int32(int16(sc.planet.Height)/2)
		limit := pw*pw + ph*ph + 0xe10
		x0, y0 := sx, sy
		for {
			r := int32(rangeXY(sx, sy, tx, ty))
			if r*r <= limit {
				break
			}
			absoluteInterpolateLine(&sx, &sy, x0, y0, tx, ty, 0x14)
		}
		g.drawTransporterBombBeam(a, sx, sy)
	}
	pic := g.getFacing(sx, sy, tx, ty) + 0x90
	if bio {
		pic += 0x10
	}
	v.SetPageBack()
	g.must2(g.fullDrawCombatScreen())
	x, y := sx, sy
	for x != tx || y != ty {
		e.MarkTime()
		v.SetPageOff()
		g.must2(g.drawMainCombatScreen())
		v.SetWindow(0, 0, 0x27f, 0x15e)
		v.ClipOn()
		absoluteInterpolateLine(&x, &y, sx, sy, tx, ty, step)
		p, err := g.cached(g.cmbtLBX(cmbtMislLBX), int(pic))
		g.must2(err)
		g.must2(e.Draw(int(x-0x11), int(y-0x11), p))
		g.must2(e.TogglePages())
		g.must2(e.ReleaseTime(1))
	}
	if bio {
		g.addDamageIndicatorForBiological(tx, y, 1, dmg)
	} else {
		g.addDamageIndicatorForMissile(tx, y, 1, dmg)
	}
	snd := g.moxPlaySound(0x18) // 0x341d9, faded after the messages
	p, err := g.cached(g.cmbtLBX(cmbtSfxLBX), 7)
	g.must2(err)
	for f := 0; f < p.Frames; f++ {
		e.MarkTime()
		v.SetPageOff()
		g.must2(g.drawMainCombatScreen())
		v.SetWindow(0, 0, 0x27f, 0x15e)
		v.ClipOn()
		g.setAnimationFrame(p, f)
		g.must2(e.Draw(int(tx-0xc), int(ty-0xc), p))
		g.drawDamageMessageQueue()
		g.must2(e.TogglePages())
		g.must2(e.ReleaseTime(1))
	}
	g.drawDamageMessageQueueUntilDone()
	g.fadeoutSound(snd, 0xa)
}

// drawTransporterBombBeam is Draw_Transporter_Bomb_Beam_ (0xad7f5): eight
// frames of the transporter beam (CMBTSFX 0xf, its palette loaded for good)
// from record a to (x, y).
func (g *Game) drawTransporterBombBeam(a, x, y int16) {
	sc := g.tc.scr
	e := g.e
	v := e.V
	ax, ay := g.shipCenterDrawXY(a)
	wx1, wy1, wx2, wy2 := v.WinX1, v.WinY1, v.WinX2, v.WinY2
	p, err := g.cached(g.cmbtLBX(cmbtSfxLBX), 0xf)
	g.must2(err)
	e.DrawPalette(p)
	sc.dontDrawShip = a
	e.RestoreGlassRemapBlock(sc.shieldRemap)
	A := g.tsp(a)
	snd := g.moxPlaySound(0x14) // drawTransporterBombBeam
	for f := 0; f < 8; f++ {
		e.MarkTime()
		v.SetPageOff()
		g.must2(g.drawMainCombatScreen())
		v.SetWindow(0, 0, 0x27f, 0x15e)
		v.ClipOn()
		g.setAnimationFrame(p, f%3)
		bm := gfx.NewBlankBitmap(1, 1)
		bm.DrawPictureToBitmap(p)
		bm.SetGlassed()
		g.drawBitmapLine(ax, ay, x, y, bm)
		v.SetWindow(0, 0, 0x27f, 0x15e)
		v.ClipOn()
		g.must2(g.drawShip(a, int(int16(A.u8(tsX))-g.tc.curX)*0x14, int(int16(A.u8(tsY))-g.tc.curY)*0x14))
		v.ClipOff()
		v.ResetWindow()
		g.must2(e.TogglePages())
		g.must2(e.ReleaseTime(1))
	}
	g.fadeoutSound(snd, 0xa)
	sc.dontDrawShip = -1
	v.WinX1, v.WinY1, v.WinX2, v.WinY2 = wx1, wy1, wx2, wy2
	e.RestoreGlassRemapBlock(sc.shieldRemap)
}

// drawGyroSpecialEffect is Draw_Gyro_Special_Effect_ (0xabc5d): the gyro
// beam (CMBTSFX 8, its palette 0x44 loaded for good) from a to t, then its
// last three frames while t spins: t turns one facing per frame, eight
// frames or four at double speed, drawn or not.
func (g *Game) drawGyroSpecialEffect(a, t int16) {
	var snd int // 0xabd6e, 0xabe9d: each phase's sound, faded after it
	tc := g.tc
	sc := tc.scr
	e := g.e
	var v *engine.Video
	if sc != nil {
		v = e.V
	}
	step := int16(1)
	if tc.fastFX() {
		step = 2
	}
	T := g.tsp(t)
	at := func(i int16) (int, int) {
		R := g.tsp(i)
		return int(int16(R.u8(tsX))-tc.curX) * 0x14, int(int16(R.u8(tsY))-tc.curY) * 0x14
	}
	var p *gfx.Picture
	var frames int16
	var ax, ay, tx, ty int16
	if sc != nil {
		pal, err := g.cached(g.cmbtLBX(cmbtSfxLBX), 0x44)
		g.must2(err)
		e.DrawPalette(pal)
		p, err = g.cached(g.cmbtLBX(cmbtSfxLBX), 8)
		g.must2(err)
		frames = int16(p.Frames)
		ax, ay = g.shipCenterDrawXY(a)
		tx, ty = g.shipCenterDrawXY(t)
		snd = g.moxPlaySound(0xb) // drawGyroSpecialEffect
		for f := int16(0); f < frames; f += step {
			e.MarkTime()
			v.SetPageOff()
			g.must2(g.drawMainCombatScreen())
			g.setAnimationFrame(p, int(f))
			bm := gfx.NewBlankBitmap(1, 1)
			bm.DrawPictureToBitmap(p)
			v.SetWindow(0, 0, 0x27f, 0x15e)
			v.ClipOn()
			g.drawBitmapLine(ax, ay, tx, ty, bm)
			x, y := at(a)
			g.must2(g.drawShip(a, x, y))
			x, y = at(t)
			g.must2(g.drawShip(t, x, y))
			v.ResetWindow()
			v.ClipOff()
			g.must2(e.TogglePages())
			g.must2(e.ReleaseTime(1))
		}
		g.fadeoutSound(snd, 0xa)
	}
	snd = g.moxPlaySound(0xb) // drawGyroSpecialEffect
	for f := int16(0); f < 8; f += step {
		T.set8(tsFacing, uint8((uint32(T.u8(tsFacing))+1)%0x10))
		if sc == nil {
			continue
		}
		e.MarkTime()
		v.SetPageOff()
		g.must2(g.drawMainCombatScreen())
		v.SetWindow(0, 0, 0x27f, 0x15e)
		v.ClipOn()
		g.setAnimationFrame(p, int(f%3+frames-3))
		bm := gfx.NewBlankBitmap(1, 1)
		bm.DrawPictureToBitmap(p)
		g.drawBitmapLine(ax, ay, tx, ty, bm)
		x, y := at(t)
		g.must2(g.drawShip(t, x, y))
		x, y = at(a)
		g.must2(g.drawShip(a, x, y))
		v.ClipOff()
		v.ResetWindow()
		g.must2(e.TogglePages())
		g.must2(e.ReleaseTime(2))
	}
	g.fadeoutSound(snd, 0xa)
}

// drawLightningField is Lightning_Field_ (0xac472): fifteen frames of the
// field's flash (CMBTSFX 0) round record t, hit by missile stack m at
// (px, py).
func (g *Game) drawLightningField(t, m, px, py int16) {
	tc := g.tc
	sc := tc.scr
	e := g.e
	v := e.V
	if !g.shipCompletelyOnScreen(t) {
		T := g.tsp(t)
		g.must2(g.snapCenterCombatScreen(int16(T.u8(tsX)), int16(T.u8(tsY))))
	}
	cx, cy := g.shipCenterOffsets(t)
	sx, sy := px-cx-tc.curX*0x14, py-cy-tc.curY*0x14
	saved := sc.dontDrawShip
	sc.dontDrawShip = t
	p, err := g.cached(g.cmbtLBX(cmbtSfxLBX), 0)
	g.must2(err)
	snd := g.moxPlaySound(4) // drawLightningField
	for f := 0; f < 0xf; f++ {
		e.MarkTime()
		v.SetPageOff()
		g.must2(g.drawMainCombatScreen())
		v.SetWindow(0, 0, 0x27f, 0x15e)
		v.ClipOn()
		g.must2(g.drawShip(t, int(sx), int(sy)))
		g.must2(e.Draw(int(sx+cx-0x24), int(sy+cy-0x24), p))
		v.ClipOff()
		v.ResetWindow()
		g.must2(e.TogglePages())
		g.must2(e.ReleaseTime(1))
	}
	g.fadeoutSound(snd, 0xa)
	sc.dontDrawShip = saved
}

// drawAntiMissileRockets is Draw_Anti_Missile_Rockets_ (0xab8bf): the
// rocket from record a at pixel (ax, ay) flying to missile stack m.
func (g *Game) drawAntiMissileRockets(a, ax, ay, m int16) {
	tc := g.tc
	sc := tc.scr
	e := g.e
	v := e.V
	M := g.msl(m)
	tx, ty := M.i16(tmX)-tc.curX*0x14, M.i16(tmY)-tc.curY*0x14
	cx, cy := g.shipCenterOffsets(a)
	sx, sy := cx+ax-tc.curX*0x14, cy+ay-tc.curY*0x14
	step := int16(0x14)
	if tc.fastFX() {
		step = 0x1e
	}
	x, y := sx, sy
	for x != tx || y != ty {
		e.MarkTime()
		v.SetPageOff()
		g.must2(g.drawMainCombatScreen())
		v.SetWindow(0, 0, 0x27f, 0x15e)
		v.ClipOn()
		if a == sc.dontDrawShip {
			g.must2(g.drawShip(a, int(ax-tc.curX*0x14), int(ay-tc.curY*0x14)))
		}
		absoluteInterpolateLine(&x, &y, sx, sy, tx, ty, step)
		v.Fill(int(x-1), int(y-1), int(x+1), int(y+1), 0x19)
		for _, d := range [4][2]int16{{-1, -1}, {-1, 1}, {1, -1}, {1, 1}} {
			v.Dot(int(x+d[0]), int(y+d[1]), 0x10)
		}
		g.must2(e.TogglePages())
		g.must2(e.ReleaseTime(1))
	}
}

// antiMissileHits is the end of Anti_Missile_Rockets_ (0xabb18..): the
// rockets' kills as a number over the stack's place and the explosion
// (CMBTSFX 7); the number is aged but left in the queue.
func (g *Game) antiMissileHits(a, ax, ay, mx, my, hits int16) {
	g.moxPlaySound(0x1c) // 0xabb66
	tc := g.tc
	sc := tc.scr
	e := g.e
	var v *engine.Video
	if sc != nil {
		v = e.V
	}
	p, err := g.cached(g.cmbtLBX(cmbtSfxLBX), 7)
	g.must2(err)
	g.addDamageIndicatorForMissile(mx, my, 1, hits)
	for f := 0; f < p.Frames; f++ {
		if sc != nil {
			e.MarkTime()
			v.SetPageOff()
			g.must2(g.drawMainCombatScreen())
			v.SetWindow(0, 0, 0x27f, 0x15e)
			v.ClipOn()
			if a == sc.dontDrawShip {
				g.must2(g.drawShip(a, int(ax-tc.curX*0x14), int(ay-tc.curY*0x14)))
			}
			g.setAnimationFrame(p, f)
			g.must2(e.Draw(int(mx-0xc), int(my-0xc), p))
			v.ClipOff()
			v.ResetWindow()
		}
		g.drawDamageMessageQueue()
		if sc != nil {
			g.must2(e.TogglePages())
			g.must2(e.ReleaseTime(1))
		}
	}
}

// drawStasisBeam is Draw_Stasis_Beam_ (0xac87a): the beam (CMBTSFX 1, its
// palette loaded for good) from a to t, then the ball growing round t.
func (g *Game) drawStasisBeam(a, t int16) {
	tc := g.tc
	sc := tc.scr
	e := g.e
	v := e.V
	if tc.onScreen != 1 || tc.superFast != 0 || sc == nil {
		return
	}
	if !g.shipCompletelyOnScreen(t) {
		T := g.tsp(t)
		g.must2(g.snapCenterCombatScreen(int16(T.u8(tsX)), int16(T.u8(tsY))))
	}
	p, err := g.cached(g.cmbtLBX(cmbtSfxLBX), 1)
	g.must2(err)
	e.DrawPalette(p)
	ax, ay := g.shipCenterDrawXY(a)
	tx, ty := g.shipCenterDrawXY(t)
	step := 1
	if tc.fastFX() {
		step = 2
	}
	sc.dontDrawShip = a
	A := g.tsp(a)
	snd := g.moxPlaySound(0xa) // drawStasisBeam
	for f := 0; f < p.Frames; f += step {
		e.MarkTime()
		v.SetPageOff()
		g.must2(g.drawMainCombatScreen())
		g.setAnimationFrame(p, f)
		bm := gfx.NewBlankBitmap(1, 1)
		bm.DrawPictureToBitmap(p)
		v.SetWindow(0, 0, 0x27f, 0x15e)
		v.ClipOn()
		g.drawBitmapLine(ax, ay, tx, ty, bm)
		g.must2(g.drawShip(a, int(int16(A.u8(tsX))-tc.curX)*0x14, int(int16(A.u8(tsY))-tc.curY)*0x14))
		v.ResetWindow()
		v.ClipOff()
		g.must2(e.TogglePages())
		g.must2(e.ReleaseTime(2))
	}
	g.fadeoutSound(snd, 0xa)
	sc.dontDrawShip = -1
	snd = g.moxPlaySound(0xa) // drawStasisBeam
	for f := 0; f < 10; f += step {
		e.MarkTime()
		v.SetPageOff()
		g.must2(g.drawMainCombatScreen())
		v.SetWindow(0, 0, 0x27f, 0x15e)
		v.ClipOn()
		g.drawStasisBall(t, int16(f*10), 0, 0)
		v.ClipOff()
		v.ResetWindow()
		g.must2(e.TogglePages())
		g.must2(e.ReleaseTime(1))
	}
	g.fadeoutSound(snd, 0xa)
}

// drawBHGShot is BHG_'s animation (0x3b64d): CMBTSFX 0x2f stretched from
// a's centre to above t's, 12 steps (two a frame at double speed).
func (g *Game) drawBHGShot(a, t int16) {
	tc := g.tc
	sc := tc.scr
	e := g.e
	v := e.V
	p := g.sfxPic(0x2f)
	// Draw_Palette_ without Refresh_Palette_: the DAC keeps its colours.
	e.DrawPalette(p)
	frames := int16(p.Frames)
	x1, y1 := g.shipCenterDrawXY(a)
	x2, y2 := g.shipCenterDrawXY(t)
	y2 -= int16(p.Height) / 2
	step := int16(1)
	if tc.fastFX() {
		step = 2
	}
	bm := gfx.NewBlankBitmap(1, 1)
	snd := g.moxPlaySound(0xa) // 0x3b753
	for f := int16(0); f < 0xc; f += step {
		e.MarkTime()
		v.SetPageOff()
		g.setAnimationFrame(p, int(f%frames))
		bm.DrawPictureToBitmap(p)
		v.SetWindow(0, 0, 0x27f, 0x15e)
		v.ClipOn()
		g.drawBitmapLine(x1, y1, x2, y2, bm)
		v.ResetWindow()
		v.ClipOff()
		g.must2(g.drawMainCombatScreen())
		g.must2(e.TogglePages())
		g.must2(e.ReleaseTime(1))
	}
	sc.dontDrawShip = -1
	g.fadeoutSound(snd, 0xa)
}

// drawStellarConverterShot is Stellar_Converter_'s animation (0xad9b5):
// CMBTSFX 0x28 stretched from a to t, from its tenth frame on 0x29 over t
// through the converter's remap.
func (g *Game) drawStellarConverterShot(a, t int16) {
	tc := g.tc
	sc := tc.scr
	e := g.e
	v := e.V
	if !g.shipCompletelyOnScreen(t) {
		T := g.tsp(t)
		g.must2(g.snapCenterCombatScreen(int16(T.u8(tsX)), int16(T.u8(tsY))))
	}
	burst := g.sfxPic(0x29)
	p := g.sfxPic(0x28)
	e.DrawPalette(p)
	e.RefreshPalette()
	x1, y1 := g.shipCenterDrawXY(a)
	x2, y2 := g.shipCenterDrawXY(t)
	frames := int16(p.Frames)
	step := int16(1)
	if tc.fastFX() {
		step = 2
	}
	bm := gfx.NewBlankBitmap(1, 1)
	snd := g.moxPlaySound(0x14) // 0xadae5
	for f := int16(0); f < frames; f += step {
		e.MarkTime()
		v.SetPageOff()
		g.must2(g.drawMainCombatScreen())
		g.setAnimationFrame(p, int(f))
		bm.DrawPictureToBitmap(p)
		v.SetWindow(0, 0, 0x27f, 0x15e)
		v.ClipOn()
		g.drawBitmapLine(x1, y1, x2, y2, bm)
		if f > 8 {
			g.setAnimationFrame(burst, int(f-9))
			e.RestoreGlassRemapBlock(sc.converterRemap)
			g.must2(e.Draw(int(x2-0x3c), int(y2-0x3c), burst))
		}
		v.ResetWindow()
		v.ClipOff()
		g.must2(e.TogglePages())
		g.must2(e.ReleaseTime(2))
	}
	g.fadeoutSound(snd, 0xa)
}
