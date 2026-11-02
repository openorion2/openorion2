package game

import (
	"strconv"

	"github.com/openorion2/openorion2/internal/save"
)

// Fighters on the combat screen (CMBTMISL.C, BEAMS.C): Draw_Fighter_, the
// attack run of Fighter_Combat_SFX_ and its beams, Draw_Fighter_Beam_.

// drawFighter is Draw_Fighter_ (0x3dbd1): up to nine fighters of stack m
// around (x, y); mode 1 draws in the full screen window and holds them
// still. Otherwise every third frame they jitter by a seed from clock_(),
// restored after.
func (g *Game) drawFighter(m int16, x, y int, mode int) {
	tc := g.tc
	sc := tc.scr
	e := g.e
	r := &g.state.Random
	M := g.msl(m)
	n := min(int(M.i16(tmCount)/4), 9)
	if n < 1 {
		n = 1
	}
	xs, ys := g.missileCoords(m, n, true)
	bottom := 0x15e
	if mode == 1 {
		bottom = 0x1df
	}
	e.V.SetWindow(0, 0, 0x27f, bottom)
	e.V.ClipOn()
	var jx, jy int
	for k := 0; k < n; k++ {
		typ := missileType(M.i16(tmWeapon))
		if mode == 0 && sc.shipFrame%3 == 0 {
			seed := r.Seed
			clock := uint32(e.Sys.Elapsed().Milliseconds() / 10)
			r.Set(clock + uint32(k) + uint32(m))
			jx = int(r.Random(3)) - 2
			jy = int(r.Random(3)) - 2
			r.Set(seed)
		}
		owner := int(M.i16(tmOwner))
		idx := typ*0x10 + int(M[tmFace])
		if owner < 0 || owner >= len(sc.fighter) || idx >= len(sc.fighter[owner]) || sc.fighter[owner][idx] == nil {
			g.logOnce("Draw_Fighter_: no fighter picture for this owner")
			continue
		}
		p := sc.fighter[owner][idx]
		if M.i16(tmTarget) == tc.cur {
			g.setAnimationFrame(p, (int(sc.shipFrame)+int(m))%3)
		} else {
			p.CurrentFrame = 0
		}
		px := int(int16(x + jx - 0xc + xs[k]))
		py := int(int16(y + jy - 0xc + ys[k]))
		g.must2(e.Draw(px, py, p))
	}
	e.V.ClipOff()
	e.V.ResetWindow()
}

// drawFighterBeam is Draw_Fighter_Beam_ (0xa9033): frame of weapon w from
// each of the n points to (tx, ty); false once past the beam's frames.
func (g *Game) drawFighterBeam(pts [][2]int16, tx, ty, w, frame int16) bool {
	b := &g.tc.beam
	r := &g.state.Random
	seed := r.Seed
	draw := g.tc.scr != nil
	if draw {
		g.e.V.SetWindow(0, 0, 0x27f, 0x15e)
		g.e.V.ClipOn()
	}
	for _, p := range pts {
		facA := int16((int32(int16(g.getAngle(int(p[0])-int(tx), int(ty)-int(p[1]))))/0x16 + 8) % 0x10)
		facT := int16((int32(int16(g.getAngle(int(tx)-int(p[0]), int(p[1])-int(ty))))/0x16 + 8) % 0x10)
		if b.maxFrames < frame {
			r.Set(seed)
			return false
		}
		g.fireWeapon(p[0], p[1], tx, ty, facA, facT, w, frame, 0, 0, 0, 0, 1, 0, 0, -1, 0)
	}
	if draw {
		g.e.V.ClipOff()
	}
	r.Set(seed)
	return frame < b.maxFrames
}

// drawDamageMessageQueueUntilDone is Draw_Damage_Message_Queue_Until_Done_
// (0xa94a1).
func (g *Game) drawDamageMessageQueueUntilDone() {
	e := g.e
	sc := g.tc.scr
	for {
		if sc != nil {
			e.MarkTime()
			e.V.SetPageOff()
			g.must2(g.drawMainCombatScreen())
			e.V.SetWindow(0, 0, 0x27f, 0x15e)
			e.V.ClipOn()
		}
		g.drawDamageMessageQueue()
		if sc != nil {
			g.must2(e.ReleaseTime(1))
			g.must2(e.TogglePages())
			e.V.ClipOff()
			e.V.ResetWindow()
		}
		if g.doneDrawingDamageMessageQueue() {
			return
		}
	}
}

// fighterCombatSFX is Fighter_Combat_SFX_ (0x3e1a4): stack m's attack run
// on its target at (px, py), then the damage numbers.
func (g *Game) fighterCombatSFX(m, px, py, dmg, abs int16) {
	tc := g.tc
	sc := tc.scr
	e := g.e
	M := g.msl(m)
	w := M.i16(tmWeapon)
	t := M.i16(tmTarget)
	atMis := M[tmAtMis] != 0
	var tx, ty int16
	if atMis {
		T := g.msl(t)
		if sc != nil && !g.onCombatScreen(T.i16(tmX)/0x14, T.i16(tmY)/0x14) {
			g.must2(g.snapCenterCombatScreen(T.i16(tmX)/0x14, T.i16(tmY)/0x14))
		}
		tx, ty = T.i16(tmX)-tc.curX*0x14, T.i16(tmY)-tc.curY*0x14
	} else {
		if sc != nil && !g.shipCompletelyOnScreen(t) {
			T := g.tsp(t)
			g.must2(g.snapCenterCombatScreen(int16(T.u8(tsX)), int16(T.u8(tsY))))
		}
		tx, ty = px-tc.curX*0x14, py-tc.curY*0x14
	}
	fx, fy := M.i16(tmX)-tc.curX*0x14, M.i16(tmY)-tc.curY*0x14
	n := min(int(M.i16(tmCount)/4), 9)
	if n < 1 {
		n = 1
	}
	xs, ys := g.missileCoords(m, n, true)
	pts := make([][2]int16, n)
	for frame, more := int16(0), true; more; frame++ {
		if sc != nil {
			e.MarkTime()
			e.V.SetPageOff()
			M.set16(tmWeapon, 0)
			g.must2(g.drawMainCombatScreen())
			M.set16(tmWeapon, w)
			if !atMis && sc.dontDrawShip == t {
				cx, cy := g.shipCenterOffsets(t)
				e.V.SetWindow(0, 0, 0x27f, 0x15e)
				e.V.ClipOn()
				g.must2(g.drawShip(t, int(tx-cx), int(ty-cy)))
				e.V.ClipOff()
				e.V.ResetWindow()
			}
		}
		for k := range pts {
			pts[k] = [2]int16{fx - 2 + int16(xs[k]), fy - 2 + int16(ys[k])}
		}
		if sc != nil {
			g.drawFighter(m, int(fx), int(fy), 1)
		}
		more = g.drawFighterBeam(pts, tx, ty, g.fighterBeamOf(int(M.i16(tmOwner))), frame)
		if sc != nil {
			g.must2(e.TogglePages())
			g.must2(e.ReleaseTime(1))
		}
	}
	if atMis {
		g.addDamageIndicatorForMissile(tx, ty, 1, dmg)
		g.drawDamageMessageQueueUntilDone()
		return
	}
	cx, cy := g.shipCenterOffsets(t)
	g.addDamageIndicatorForShip(tx-cx, ty-cy, t, 1, dmg, abs)
	for !g.doneDrawingDamageMessageQueue() {
		if sc != nil {
			e.MarkTime()
			e.V.SetPageOff()
			g.must2(g.drawMainCombatScreen())
			e.V.SetWindow(0, 0, 0x27f, 0x15e)
			e.V.ClipOn()
			if t == sc.dontDrawShip {
				g.must2(g.drawShip(t, int(tx-cx), int(ty-cy)))
			}
		}
		g.drawDamageMessageQueue()
		if sc != nil {
			g.must2(e.TogglePages())
			g.must2(e.ReleaseTime(1))
			e.V.ClipOff()
			e.V.ResetWindow()
		}
	}
}

// drawMissileStatus is Draw_Missile_Status_ (0x3c539): the hovered stack m
// in the status box at (x, y) with its target and count.
func (g *Game) drawMissileStatus(x, y int, m int16) error {
	e := g.e
	v := e.V
	v.Fill(0x191, 0x17b, 0x1e8, 0x1d3, 0)
	if err := g.drawCached(0x191, 0x17b, 0x2d); err != nil {
		return err
	}
	if err := g.drawMissile(m, x+0x2c, y+0x2c, 1); err != nil {
		return err
	}
	t := e.Text()
	t.SetStyle(0, g.combatGreen())
	head, err := g.kenText(0x34)
	if err != nil {
		return err
	}
	M := g.msl(m)
	var target string
	if M[tmAtMis] == 0 {
		target = save.CString(g.tsp(M.i16(tmTarget))[:])
	} else if target, err = g.weaponName(int(g.msl(M.i16(tmTarget)).i16(tmWeapon)), false); err != nil {
		return err
	}
	if err := t.PrintCentered(x+0x2b, y+2, head+target); err != nil {
		return err
	}
	n := M.i16(tmCount)
	line := strconv.Itoa(int(n))
	var tail string
	switch w := M.i16(tmWeapon); g.wtype(w) {
	case 1:
		tail, err = g.kenText(map[bool]int{true: 0x35, false: 0x6b}[n == 1])
	case 4:
		line += cStringAt(g.exe, 0x178d8b)
		fw := int(w)
		if w != 0x1e && w != 0x1f && w != 0x1d {
			fw = 0x1c
		}
		tail, err = g.weaponName(fw, n != 1)
	default:
		tail, err = g.kenText(map[bool]int{true: 0x36, false: 0x6c}[n == 1])
	}
	if err != nil {
		return err
	}
	return t.PrintCentered(x+0x2b, y+0x51, line+tail)
}
