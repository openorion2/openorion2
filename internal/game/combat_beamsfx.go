package game

import (
	"encoding/binary"
	"image"
	"strconv"

	"github.com/openorion2/openorion2/internal/engine"
	"github.com/openorion2/openorion2/internal/font"
	"github.com/openorion2/openorion2/internal/gfx"
)

// Beam_SFX_ and the beam strokes (BEAMS.C). The logic (frame counts, hit
// flags, the missile kills) runs without a screen too; the drawing only
// with one. Spec: docs/screens/combat-screen.md ("Beams").

// beamRec16 is one of Fire_Ship_'s 0xf-byte records as Beam_SFX_ reads it.
type beamRec16 struct {
	weapon, shots, result, pierced, dmg, abs, mods int16
	refl                                           byte
}

// beamRecs is Fire_Ship_'s record array; entries past the list read as 0.
type beamRecs [0x18]beamRec16

func (r *beamRecs) at(i int16) *beamRec16 {
	if i < 0 || int(i) >= len(r) {
		return &beamRec16{}
	}
	return &r[i]
}

// beamFX is a weapon's stroke in Fire_Weapon_ (0xa761c).
type beamFX struct {
	color, chance, maxChange, speed, rotation, length int16
	cMax, cMin, sMax, sMin                            int16
	aaHead, triangular, glowing, fading, interlock    int16
	noAA, swap, mauler, bolt, sound, burstSet, repeat int16
}

// beamFXFor is Fire_Weapon_'s switch on the weapon.
func beamFXFor(w int16) (beamFX, bool) {
	f := beamFX{}
	switch {
	case w == 1:
		f = beamFX{color: 7, speed: 0x2d, cMax: 0xa, sound: 0xc, burstSet: 1, rotation: 1, length: 1, aaHead: 1, triangular: 1, glowing: 1, bolt: 1, repeat: 1, sMax: 7}
	case w == 2:
		f = beamFX{color: 2, burstSet: 1, chance: 0x28, maxChange: 3, speed: 0x2d, rotation: 1, cMax: 0xa, sMax: 7, aaHead: 1, glowing: 1, fading: 1, bolt: 1, sound: 0xf, length: 2, repeat: 1}
	case w == 3:
		f = beamFX{chance: 0xa, maxChange: 1, speed: 0x37, rotation: 1, length: 0x5a, cMax: 0xa, cMin: 7, interlock: 1, sound: 1}
	case w == 4:
		f = beamFX{color: 1, chance: 0x32, maxChange: 3, speed: 0x2d, length: 0x3c, cMax: 0xf, cMin: 7, sMax: 3, sound: 0x17, rotation: 1, aaHead: 1, triangular: 1, glowing: 1, fading: 1, bolt: 1}
	case w == 5:
		f = beamFX{color: 8, chance: 0x28, maxChange: 5, speed: 0x28, length: 0x280, cMax: 0xf, cMin: 0xa, sMax: 7, aaHead: 1, triangular: 1, glowing: 1, fading: 1, interlock: 1, swap: 1, bolt: 2, sound: 9}
	case w == 6:
		f = beamFX{color: 3, chance: 0x32, speed: 0x2d, rotation: 1, length: 0x28, cMax: 0xf, cMin: 7, aaHead: 1, triangular: 1, glowing: 1, fading: 1, interlock: 1, bolt: 1, sound: 0xe, maxChange: 3, sMax: 3}
	case w == 7:
		f = beamFX{color: 6, chance: 0x1e, maxChange: 5, speed: 0x23, rotation: 1, length: 0x280, cMax: 7, cMin: 7, sMax: 3, aaHead: 1, glowing: 1, bolt: 2, sound: 0xb}
	case w == 8:
		f = beamFX{color: 2, chance: 0x1e, maxChange: 3, speed: 0x2d, rotation: 1, length: 0x5a, cMax: 0xf, cMin: 7, sMax: 3, aaHead: 1, triangular: 1, glowing: 1, fading: 1, bolt: 1, sound: 0x16}
	case w == 9 || w == 0x29:
		f = beamFX{chance: 0xf, maxChange: 3, speed: 0x32, rotation: 1, length: 0x5a, cMax: 0xf, cMin: 7, sMax: 6, sMin: 2, aaHead: 1, glowing: 1, interlock: 1, bolt: 1, sound: 0x13}
	case w == 0x2e:
		f = beamFX{color: 6, chance: 0xa, maxChange: 3, speed: 0x32, rotation: 1, length: 0x28, cMax: 0xf, cMin: 0xa, sMax: 0xa, sMin: 7, aaHead: 1, triangular: 1, glowing: 1, fading: 1, interlock: 1, noAA: 1, bolt: 2, sound: 0x12}
	case w == 0xa:
		f = beamFX{chance: 0x28, maxChange: 3, speed: 0x2d, rotation: 1, length: 0x28, cMax: 0xf, cMin: 7, sMax: 3, aaHead: 1, triangular: 1, glowing: 1, fading: 1, bolt: 1, sound: 2}
	case w == 0xb || w == 0x2a:
		f = beamFX{color: 6, chance: 0x28, maxChange: 3, speed: 0x23, rotation: 1, length: 0x5a, cMax: 0xf, cMin: 7, sMax: 3, aaHead: 1, triangular: 1, glowing: 1, fading: 1, bolt: 2, sound: 0x15}
	case w == 0xc:
		f = beamFX{chance: 0x32, maxChange: 5, speed: 0x32, rotation: 1, length: 0x3c, cMax: 0xd, cMin: 0xa, sMax: 8, aaHead: 1, triangular: 1, glowing: 1, fading: 1, interlock: 1, bolt: 2, sound: 0xd}
	case w == 0xd:
		f = beamFX{color: 2, chance: 0x1e, maxChange: 3, speed: 0x37, rotation: 1, length: 0x28, cMax: 0xf, cMin: 0xa, sMax: 0xa, aaHead: 1, triangular: 1, glowing: 1, fading: 1, interlock: 1, sound: 1, bolt: 2}
	case w == 0x1b || w == 0x28 || w == 0x2b:
		f = beamFX{color: 5, chance: 0x1e, speed: 0x28, rotation: 1, length: 0x14, cMax: 0xf, cMin: 0xa, sMax: 7, aaHead: 1, triangular: 1, glowing: 1, fading: 1, interlock: 1, mauler: 1, bolt: 1, maxChange: 5, repeat: 1, sound: 5}
	default:
		return f, false
	}
	return f, true
}

// weaponHasSteveStuff is Weapon_Has_Steve_Stuff_ (0xab4ec), as Fire_Weapon_
// tests it inline.
func weaponHasSteveStuff(w int16) bool {
	return (w >= 1 && w <= 0xc) || w == 0x29 || w == 0x2a
}

// steveStruct is Build_Steve_Stuff_Struct_For_Weapon_ (0xab11c): the 11
// int32 of the weapon's beam style.
func (g *Game) steveStruct(w, mods int16) [11]int32 {
	var a uint32
	m10, m100 := mods&0x10 != 0, mods&0x100 != 0
	pick := func(c bool, y, n uint32) uint32 {
		if c {
			return y
		}
		return n
	}
	switch {
	case w == 1:
		a = 0xa674e
	case w == 2:
		a = 0xa677a
	case w == 3:
		a = pick(m10, 0xa67a6, 0xa67d2)
	case w == 4:
		a = pick(m10, 0xa67fe, 0xa682a)
	case w == 5:
		switch {
		case m10 && m100:
			a = 0xa6646
		case m10:
			a = 0xa6672
		case m100:
			a = 0xa669e
		default:
			a = 0xa66ca
		}
	case w == 6:
		a = pick(m10, 0xa66f6, 0xa6722)
	case w == 7:
		a = pick(m10, 0xa6856, 0xa6882)
	case w == 8:
		a = pick(m10, 0xa68ae, 0xa68da)
	case w == 9 || w == 0x29:
		a = pick(m10, 0xa6906, 0xa6932)
	case w == 0xa:
		a = 0xa695e
	case w == 0xb || w == 0x2a:
		a = 0xa698a
	case w == 0xc:
		a = pick(m10, 0xa69b6, 0xa69e2)
	}
	var s [11]int32
	if a == 0 {
		return s
	}
	b := g.exe.bytes(a, 0x2c)
	for i := range s {
		s[i] = int32(binary.LittleEndian.Uint32(b[4*i:]))
	}
	return s
}

// addWeaponPaletteFragment is Add_Weapon_Palette_Fragment_To_Current_
// Palette_ (0xa75c3): BEAMS.LBX 0's colours straight to the DAC.
func (g *Game) addWeaponPaletteFragment(color, at, n int16) {
	pal := g.beamPic(0).Palette
	for i := int16(0); i < n; i++ {
		k := int(color)<<4 + int(i)
		if pal == nil || k < pal.Start || k >= pal.Start+len(pal.Colors) {
			g.logOnce("Get_Palette_Animation_Color: colour out of the picture's palette")
			continue
		}
		r := pal.Raw[4*(k-pal.Start):]
		g.e.V.DAC[int(at+i)&0xff] = [3]byte{r[1], r[2], r[3]}
	}
}

// fireWeapon is Fire_Weapon_ (0xa761c): one stroke of weapon w from
// (x1, y1) to (x2, y2) at frame, repeated thrice for repeating weapons.
func (g *Game) fireWeapon(x1, y1, x2, y2, facA, facT, w, frame, slot, sizeT, result, pierced, mode, fp, refl, sizeA, mods int16) {
	b := &g.tc.beam
	f, ok := beamFXFor(w)
	if !ok {
		return
	}
	if !weaponHasSteveStuff(w) {
		if f.color != b.oldColor[slot] {
			b.oldColor[slot] = f.color
			if g.tc.scr != nil {
				g.addWeaponPaletteFragment(f.color, slot<<4+0x90, 0x10)
			}
		}
		if mods&2 != 0 {
			if f.bolt+1 >= 2 {
				f.bolt = 2
			} else {
				f.bolt++
			}
		}
		if mods&0x80 != 0 {
			f.length /= 2
			f.speed <<= 1
		}
		if mods&4 != 0 {
			f.bolt = max(f.bolt-1, 0)
		}
		if mods&8 != 0 {
			f.glowing = 1
		}
		if mods&0x10 != 0 && f.length != 0x280 {
			f.length *= 2
		}
		if mods&0x20 != 0 {
			f.fading = 0
		}
		if mods&0x40 != 0 {
			f.glowing = 1
		}
	}
	n := int16(1)
	if f.repeat != 0 && sizeA != -1 {
		n = 3
	}
	var steve *[11]int32
	if weaponHasSteveStuff(w) {
		s := g.steveStruct(w, mods)
		steve = &s
		f.speed = int16(s[8])
		f.length = int16(s[9])
	}
	for k := frame; k < frame+n; k++ {
		g.drawGenericBeam(x1, y1, x2, y2, facA, facT, slot<<4+0x90, &f, k, sizeT, result, pierced, mode, refl, sizeA, steve)
	}
	// 0xa8114: the mount's sound starts on its first frame and fades when
	// the side stops firing or the stroke's frames run out.
	if fp < 0 || fp > 4 || slot < 0 || slot > 1 {
		return
	}
	h := &b.weaponSound[slot][fp]
	switch {
	case frame == fp || frame == 0:
		*h = g.moxPlaySound(int(f.sound))
	case !b.shipFiring[slot] || int32(frame) >= int32(b.maxFrames)-1:
		g.fadeoutSound(*h, 0xa)
		*h = 0
	}
}

// fadeoutSound is Fadeout_Sound_ when there is an engine.
func (g *Game) fadeoutSound(h, step int) {
	if g.e != nil {
		g.e.FadeoutSound(h, step)
	}
}

// pitchBendSound is Pitch_Bend_Sound_ when there is an engine.
func (g *Game) pitchBendSound(h, d int) {
	if g.e != nil {
		g.e.PitchBendSound(h, d)
	}
}

// getShipRange is Get_Ship_Range_ (0xa7342): the larger of |dx| and |dy|.
func getShipRange(x1, y1, x2, y2 int16) int16 {
	dx, dy := x2-x1, y2-y1
	if dx < 0 {
		dx = -dx
	}
	if dy < 0 {
		dy = -dy
	}
	return max(dx, dy)
}

// robInterpolateLine is Rob_Interpolate_Line_ (0xa6a2e): (x, y) moved n
// pixels toward (ex, ey) by halving.
func robInterpolateLine(x, y *int16, ex, ey, n int16) {
	dx, dy := int32(ex)-int32(*x), int32(ey)-int32(*y)
	if n <= 0 {
		return
	}
	abs := func(v int32) int32 {
		if v < 0 {
			return -v
		}
		return v
	}
	major := func(a, b int32) int32 {
		m := a
		if abs(a) < abs(b) {
			m = b
		}
		return abs(m)
	}
	if int32(n) >= major(dx, dy) {
		*x, *y = ex, ey
		return
	}
	switch {
	case dx == 0:
		if dy < 1 {
			*y -= n
		} else {
			*y += n
		}
	case dy == 0:
		if dx < 1 {
			*x -= n
		} else {
			*x += n
		}
	default:
		hiX, hiY := dx, dy
		var loX, loY int32
		cx, cy := dx>>1, dy>>1
		for {
			m := major(cx, cy)
			if m == int32(n) {
				break
			}
			if int32(n) < m {
				hiX, hiY = cx, cy
				cx -= (cx - loX) / 2
				cy -= (cy - loY) / 2
			} else {
				loX, loY = cx, cy
				cx += (hiX - cx) / 2
				cy += (hiY - cy) / 2
			}
		}
		*x += int16(cx)
		*y += int16(cy)
	}
}

// drawShipBurst is Draw_Ship_Burst_ (0xa7364): the muzzle flash.
func (g *Game) drawShipBurst(x, y, facing, set, frame, side int16) {
	if g.tc.scr == nil {
		return
	}
	p := g.tc.beam.burst[set&1][side&1][facing&0xf]
	setFrame(p, int(frame%3))
	g.must2(g.drawCentered(int(x), int(y), p))
}

// drawShipExplosion is Draw_Ship_Explosion_ (0xa73b7): a hit's flash.
func (g *Game) drawShipExplosion(x, y, facing, frame, side int16) {
	if g.tc.scr == nil {
		return
	}
	p := g.tc.beam.explosion[side&1][facing&0xf]
	setFrame(p, int(frame)%(p.Frames-1))
	setFrame(p, int(frame)%(p.Frames-1))
	g.must2(g.e.Draw(int(x)-8, int(y)-8, p))
}

// shieldDiameter and reflDiameter read _shield_diameters and
// _reflection_field_diameters.
func (g *Game) shieldDiameter(k int16) int16 { return int16(g.exeU8(beamShieldDiam + uint32(k))) }
func (g *Game) reflDiameter(k int16) int16   { return int16(g.exeU8(beamReflDiam + uint32(k))) }

// shipSizeToShieldSize is Ship_Size_To_Shield_Size_ (0xa9aad).
func shipSizeToShieldSize(s int16) int16 { return [6]int16{0, 1, 1, 2, 3, 3}[s] }

// planetSizeToShieldSize is Planet_Size_To_Shield_Size_ (0xa9ac8).
func planetSizeToShieldSize(s int16) int16 { return [5]int16{4, 5, 6, 7, 7}[s] }

// xyInRadius is XY_In_Shield_Radius_ (0xa96c7) and XY_In_Reflection_
// Field_Radius_ (0xaa55b) with the diameter given.
func xyInRadius(x, y, cx, cy, diam int16) bool {
	dx, dy := x-cx, y-cy
	if dx < 0 {
		dx = -dx
	}
	if dy < 0 {
		dy = -dy
	}
	return dx <= diam/2 && dy <= diam/2
}

// drawGenericBeam is Draw_Generic_Beam_ (0xa6b9c): frame k of a stroke.
func (g *Game) drawGenericBeam(x1, y1, x2, y2, facA, facT, color int16, fx *beamFX, k, sizeT, result, pierced, mode, refl, sizeA int16, steve *[11]int32) {
	b := &g.tc.beam
	f := *fx
	facing := int16((int32(g.getAngle(int(x1-x2), int(y2-y1)))/0x16 + 8) % 0x10)
	b.shipHitNow = [2]bool{}
	side := int16(b2i(color != 0x90))
	rng := getShipRange(x1, y1, x2, y2)
	if sizeA == -1 {
		f.bolt = 0
		if uint16(f.length) > 0x14 {
			f.length = 0x14
		}
		f.speed = 5
		f.burstSet = 1
	}
	base := int16(2 + b2i(sizeA != -1))
	if steve != nil {
		f.length = int16(steve[8])
		f.speed = int16(steve[9])
	}
	span := func() uint32 {
		l := uint32(int32(f.length))
		lo := max(l, 0x3c)
		r := uint32(int32(rng))
		if lo > r {
			return r + r
		}
		return lo + r
	}
	v := int16(span() / uint32(int32(f.speed)))
	b.maxFrames = max(base, v)
	if b.maxFrames > 6 {
		f.speed <<= 1
	}
	b.maxFrames = max(base, int16(span()/uint32(int32(f.speed))))
	if k >= b.maxFrames {
		return
	}
	if k == 0 && sizeA != -1 {
		g.drawShipBurst(x1, y1, facing, f.burstSet, 0, side)
		return
	}
	head := k * f.speed
	if steve != nil && steve[7] == 2 {
		head = rng
	}
	head = min(max(head, 0), rng)
	hx, hy := x1, y1
	robInterpolateLine(&hx, &hy, x2, y2, head)
	r := &g.state.Random
	if (hx == x2 && hy == y2) || int32(k) >= int32(b.maxFrames)-2 {
		b.shipHitNow[side] = true
		hx += int16(r.Random(3)) - 1
		hy += int16(r.Random(3)) - 1
	} else {
		b.shipHitNow[side] = false
	}
	hx, hy = x1, y1
	var stop int16
	switch {
	case result != 0 && pierced == 0 && !b.pierced && sizeA != -1:
		stop = g.shieldDiameter(shipSizeToShieldSize(sizeT)) / 3
	case refl == 1 && pierced == 0 && !b.pierced && sizeA != -1:
		stop = g.reflDiameter(shipSizeToShieldSize(sizeT)) / 3
	default:
		b.pierced = true
	}
	stop = min(stop, f.length-1)
	robInterpolateLine(&hx, &hy, x2, y2, head-stop)
	tx, ty := x1, y1
	tail := k*f.speed - f.length
	tail = min(max(tail, 0), rng)
	if steve != nil && steve[7] == 2 {
		tail = 0
	}
	if refl == 2 {
		s := sizeA
		if s < 0 {
			s = 0
		}
		tail += g.reflDiameter(shipSizeToShieldSize(s)) / 3
	}
	if tail != 0 {
		robInterpolateLine(&tx, &ty, x2, y2, tail)
	}
	swap := func() {
		if f.swap != 0 && steve == nil {
			tx, hx = hx, tx
			ty, hy = hy, ty
		}
	}
	swap()
	draw := g.tc.scr != nil
	depth := g.exeI16(beamColorDepth)
	if steve == nil {
		if tx != hx || ty != hy {
			switch f.bolt {
			case 1:
				if draw {
					g.boltLine(tx, ty, hx, hy, color, depth, f.chance, f.maxChange, f.rotation, f.aaHead, f.triangular, f.glowing, f.fading, f.interlock, f.noAA, f.cMax, f.cMin, f.sMax, f.sMin)
				}
			case 2:
				if draw {
					g.wideBoltLine(tx, ty, hx, hy, color, depth, f.chance, f.maxChange, f.rotation, f.aaHead, f.triangular, f.glowing, f.fading, f.interlock, f.noAA, f.cMax, f.cMin, (f.cMax+f.sMax)/2, (f.sMin+f.cMin)/2, f.sMax, f.sMin)
				}
			case 0:
				sMin, sMax := max(f.sMin, f.cMin), max(f.sMax, f.cMax)
				if draw {
					g.boltLine(tx, ty, hx, hy, color, depth, f.chance, f.maxChange, f.rotation, f.aaHead, f.triangular, f.glowing, f.fading, f.interlock, f.noAA, 0, 0, sMax, sMin)
				}
			}
			if f.mauler != 0 && !b.shipHitNow[side] && draw {
				g.must2(g.drawCentered(int(hx), int(hy), b.mauler))
			}
		}
	} else {
		if refl == 2 {
			x1, y1 = tx, ty
		}
		g.drawBeam2(x1, y1, x2, y2, tx, ty, hx, hy, b2i(result == 0), k, color, steve)
	}
	swap()
	if k < 3 || (tx == x1 && ty == y1) {
		if sizeA != -1 {
			g.drawShipBurst(x1, y1, facing, f.burstSet, k, side)
		}
		b.shipFiring[side] = true
	} else {
		b.shipFiring[side] = false
	}
	if b.shipHitNow[side] && sizeA != -1 {
		var e int16
		if int32(k) < int32(b.maxFrames)-1 {
			e = int16((int32(k) - (int32(rng)+10)/int32(f.speed) - 1) % 5)
		} else {
			e = 7 - (b.maxFrames - k)
		}
		if e < 0 {
			e = -e
		}
		if pierced != 0 || result != 0 {
			fac := facT
			if mode != 0 {
				fac = 0
			}
			g.drawShipExplosion(x2, y2, fac, e, side)
		}
	}
	b.shieldHitNow = result != 0 && xyInRadius(hx, hy, x2, y2, g.shieldDiameter(shipSizeToShieldSize(sizeT)))
	b.reflHitNow = refl == 1 && xyInRadius(hx, hy, x2, y2, g.reflDiameter(shipSizeToShieldSize(sizeT)))
}

// drawBeam2 is Draw_Beam2_ (0xab52e): the styled beams, then their colours.
func (g *Game) drawBeam2(x1, y1, x2, y2, tx, ty, hx, hy int16, noDmg, k, color int16, s *[11]int32) {
	if g.tc.scr == nil {
		return
	}
	mode := uint32(s[7])
	if !(tx == hx && ty == hy) && mode <= 3 {
		half := int16(int32(k) % 2)
		switch mode {
		case 0:
			if k == 0 {
				tx, hx = hx, tx
				ty, hy = hy, ty
			}
			cMin, cMax := int16(3), int16(0xa)
			switch {
			case k == 0:
			case k == 1:
				cMin = 6
			default:
				cMin, cMax = 3, 0xc
			}
			g.boltLine(tx, ty, hx, hy, color, 0xc, 0x1e, 3, k, 0, 0, 1, 1, 0, 0, cMax, cMin, 4, 0)
		case 1:
			g.wideBoltLine(tx, ty, hx, hy, color, 0xc, 0x32, 3, k, 0, 1, int16(b2i(s[8] >= 0xa)), 1, 0, 0, 0xc, 3, int16(s[10])*2+7, 1, 2, 0)
		case 2:
			d := half + 1
			r := &g.state.Random
			ey := int16(r.Random(5)) + y2 - 3
			ex := int16(r.Random(5)) + x2 - 3
			g.boltLine(x1, y1, ex, ey, color, 0xc, 0x64, 4, k<<2, 0, 0, 0, 0, 1, 0, 0xc/d, 8/d, 4/d, 0)
		case 3:
			c := 0x15 / (half + 2)
			sd := 5/(half*2+1) + 1
			e := 2 / (half + 1)
			g.wideBoltLine(x1, y1, x2, y2, color, 0x10, 0x64, 3, k*2, 1, 0, 0, 0, 1, 0, c, c, sd, sd, e, e)
		}
	}
	g.setBeamColors(color, s[0], s[1], s[2], s[3], s[4], s[5], s[6])
	g.e.RefreshPalette()
}

// setBeamColors is Set_Beam_Colors_ (0xaac37): sixteen colours from a ramp
// of the primary colour, crosses toward the secondary and its saturations.
// Entries 7 and 12 are the two colours themselves; with a level past 2
// the crosses and saturations stay stale stack, left as they are.
func (g *Game) setBeamColors(at int16, r1, g1, b1, r2, g2, b2, level int32) {
	tab := func(base uint32, i int) int32 {
		return int32(binary.LittleEndian.Uint32(g.exe.bytes(base+uint32(4*i), 4)))
	}
	var c [16][3]int32
	var set [16]bool
	for i := 0; i < 7; i++ {
		v := tab(beamLowRamp, i)
		c[i] = [3]int32{r1 * v / 100, g1 * v / 100, b1 * v / 100}
		set[i] = true
	}
	c[7], set[7] = [3]int32{r1, g1, b1}, true
	c[12], set[12] = [3]int32{r2, g2, b2}, true
	if level >= 0 && level <= 2 {
		cross := beamCross + uint32(level)*0xf0
		sat := beamSaturated + uint32(level)*0xf0
		m := int32(0x73)
		for i := 0; i < 4; i++ {
			v := tab(cross, i)
			rest := 100 - v
			c[8+i] = [3]int32{(r1*v*m/100 + r2*rest) / 100, (g1*v*m/100 + g2*rest) / 100, (b1*v*m/100 + b2*rest) / 100}
			for j := range c[8+i] {
				c[8+i][j] = min(c[8+i][j], 0xff)
			}
			set[8+i] = true
			m += 0xf
		}
		for i := 0; i < 3; i++ {
			v := tab(sat, i)
			c[13+i] = [3]int32{r2 * v / 100, g2 * v / 100, b2 * v / 100}
			for j := range c[13+i] {
				c[13+i][j] = min(c[13+i][j], 0xff)
			}
			set[13+i] = true
		}
	}
	pal := &g.e.V.Palette
	for i := range c {
		if !set[i] {
			g.logOnce("Set_Beam_Colors_: a level past 2 leaves stale-stack colours")
			continue
		}
		pal[int(at)+i] = engine.Color{Changed: 1, R: byte(c[i][0] / 4), G: byte(c[i][1] / 4), B: byte(c[i][2] / 4)}
	}
}

// nFirePoints is N_Fire_Points_ (0xc6b6b): the shots drawn at once.
func (g *Game) nFirePoints(w, shots int16) int16 {
	n := int16(0)
	switch {
	case shots < 0xb || g.tc.beam.firingReflected:
		n = 1
	case shots < 0x1e:
		n = 2
	case shots < 0x3c:
		n = 3
	case shots < 99:
		n = 4
	case shots == 99:
		n = 5
	}
	if w == 0x1b && n < 2 {
		return 2
	}
	return n
}

// calcFPOffsets is Calc_FP_Offsets_ (0xc6bd0): fire point k of the ship's
// picture, rotated with its facing.
func (g *Game) calcFPOffsets(w, shots, owner, picture int16, facing, arc uint8, k int16) (int16, int16) {
	n := g.nFirePoints(w, shots)
	if int32(n)%2 == 1 && int32(k) == int32(n)-1 {
		return 0, 0
	}
	set := 0
	if owner < 8 {
		set = int(g.state.Players[owner].Color)
	}
	d := g.tc.beam.fp[set]
	j := int((int32(k) + int32(arc)) % 4)
	o := int(picture)*0x10 + j*4
	var x, y int32
	if o+4 <= len(d) {
		x = int32(int16(binary.LittleEndian.Uint16(d[o:]))) - 0x1a
		y = int32(int16(binary.LittleEndian.Uint16(d[o+2:]))) - 0x18
	}
	g.rotateCoordinates(-int16(int32(facing)*0x168>>4), &x, &y)
	return int16(x), int16(y)
}

// addDamageMessageToQueue is Add_Damage_Message_To_Queue_ (0xa9179).
func (g *Game) addDamageMessageToQueue(x, y, dmg, abs, bio int16, right byte) {
	b := &g.tc.beam
	if b.nMsgs < 0x23 && (dmg > 0 || abs > 0 || bio > 0) {
		b.msgs[b.nMsgs] = damageMsg{x: x, y: y, dmg: dmg, abs: abs, bio: bio, right: right}
		b.nMsgs++
	}
}

// squishOutMsgQueueSlot is Squish_Out_Msg_Queue_Slot_ (0xa91e8): the tail
// of the 0x23 slots moves down regardless of the count.
func (g *Game) squishOutMsgQueueSlot(k int16) {
	b := &g.tc.beam
	for ; k < 0x22; k++ {
		b.msgs[k] = b.msgs[k+1]
	}
	b.nMsgs--
}

// drawDamageMessageQueue is Draw_Damage_Message_Queue_ (0xa9223): each
// message rises for nine frames; the one moved into a freed slot is skipped
// this frame, as in the original.
func (g *Game) drawDamageMessageQueue() {
	b := &g.tc.beam
	for k := int16(0); k < b.nMsgs; k++ {
		m := &b.msgs[k]
		g.drawDamageIndicator(m.x, m.y, m.dmg, m.abs, m.bio, m.right, m.age)
		m.age++
		if m.age > 8 {
			g.squishOutMsgQueueSlot(k)
		}
	}
}

// drawDamageIndicator is Draw_Damage_Indicator_ (0xa92a6): the numbers of
// damage, absorbed and biological hits fading by age.
func (g *Game) drawDamageIndicator(x, y, dmg, abs, bio int16, right byte, age int16) {
	if g.tc.scr == nil || age >= 9 {
		return
	}
	e := g.e
	v := e.V
	y -= age
	dx := age
	if right != 1 {
		dx = -age
	}
	x += dx
	v.SetWindow(0, 0, 0x27f, 0x15e)
	v.ClipOn()
	t := e.Text()
	// The three 27-byte colour rows sit 28 bytes apart on the stack and
	// a style reads 8 colours, so late ages read on into the next row.
	var mem [0x54 + 8]byte
	copy(mem[0:], g.exe.bytes(beamDmgColors2, 0x1b))
	copy(mem[0x1c:], g.exe.bytes(beamDmgColors, 0x1b))
	copy(mem[0x38:], g.exe.bytes(beamDmgColors3, 0x1b))
	print := func(row int, px int16, n int16) {
		var c font.Colors
		copy(c[:], mem[row+int(age)*3:])
		t.SetStyleShadow(1, c, font.ShadowDown)
		g.must2(t.Print(int(px), int(y), strconv.Itoa(int(n))))
	}
	if dmg > 0 {
		print(0x1c, x, dmg)
	}
	if bio > 0 {
		print(0x38, x-10, bio)
	}
	if abs > 0 {
		print(0, x-0x14, abs)
	}
	v.ClipOff()
}

// doneDrawingDamageMessageQueue is Done_Drawing_Damage_Message_Queue_.
func (g *Game) doneDrawingDamageMessageQueue() bool { return g.tc.beam.nMsgs == 0 }

// addDamageIndicatorForShip is Add_Damage_Indicator_For_Ship_ (0xa93aa).
func (g *Game) addDamageIndicatorForShip(x, y, i int16, right byte, dmg, abs int16) {
	size := int16(g.tsp(i).u8(tsSize))
	ox, oy := g.shipCenterOffsets(i)
	d := size * 8
	if right == 0 {
		d = -size * 8
	}
	g.addDamageMessageToQueue(ox+x+d, oy+y-size*8, dmg, abs, 0, right)
}

// addDamageIndicatorForMissile is Add_Damage_Indicator_For_Missile_.
func (g *Game) addDamageIndicatorForMissile(x, y int16, right byte, dmg int16) {
	d := int16(-8)
	if right != 0 {
		d = 8
	}
	g.addDamageMessageToQueue(x+d, y-8, dmg, 0, 0, right)
}

// getSFXDrawShieldFacing2 is Get_SFX_Draw_Shield_Facing2_ (0xa956c): the
// sixteenth of the circle from a's centre toward b's.
func (g *Game) getSFXDrawShieldFacing2(a, b int16) int16 {
	ax, ay := g.shipCenterOffsets(a)
	bx, by := g.shipCenterOffsets(b)
	A, B := g.tsp(a), g.tsp(b)
	px := ax + int16(A.u8(tsX))*0x14
	py := ay + int16(A.u8(tsY))*0x14
	qx := int16(B.u8(tsX))*0x14 + bx
	qy := int16(B.u8(tsY))*0x14 + by
	return int16((int32(g.getAngle(int(px)-int(qx), int(qy)-int(py))) / 0x16) % 0x10)
}

// rotToBaseRot and rotToRotMethod are Rot_To_Base_Rot_ (0xa9f9e) and
// Rot_To_Rot_Method_ (0xa9fcb): a sixteenth to a picture and a flip.
func (g *Game) rotToBaseRot(r int16) int16 { return g.exeI16(beamRotBase + uint32(int32(r)%0x10)*2) }
func (g *Game) rotToRotMethod(r int16) int16 {
	return g.exeI16(beamRotMethod + uint32(int32(r)%0x10)*2)
}

// shieldAnims is Shield_Anims_ (0xa97f7): the shield hit picture by facing
// base, shield size, pierced and the 0x100 modification.
func (g *Game) shieldAnims(rot, size int16, pierced bool, mods int16) *gfx.Picture {
	switch {
	case size >= 4:
		return g.beamPic(int(rot + (size-4)*5 + 0x85))
	case mods != 0:
		return g.beamPic(int(size + 0x81))
	case !pierced:
		return g.beamPic(int(0x43 + size*5 + rot))
	}
	return g.beamPic(int(rot + size*5 + 0x6d))
}

// adjustShieldDisplayStatus is Adjust_Shield_Display_Status_ (0xa9618).
func (g *Game) adjustShieldDisplayStatus(ctr *int16) {
	b := &g.tc.beam
	n := int16(g.shieldAnims(0, 0, false, 0).Frames)
	switch b.shieldDisplay {
	case 0:
		if b.shieldHitNow {
			b.shieldDisplay = 1
			*ctr = 0
		}
	case 1:
		if *ctr++; *ctr > 3 {
			b.shieldDisplay = 2
		}
	case 2:
		*ctr++
		if !b.shieldHitNow {
			if *ctr > 6 {
				b.shieldDisplay = 3
			}
		} else if *ctr > 6 {
			*ctr = 4
		}
	case 3:
		if *ctr++; *ctr >= n {
			b.shieldDisplay = 0
		}
	}
}

// adjustReflectionFieldDisplayStatus is Adjust_Reflection_Field_Display_
// Status_ (0xa9ffc).
func (g *Game) adjustReflectionFieldDisplayStatus(ctr *int16) {
	b := &g.tc.beam
	n := int16(g.beamPic(0x59).Frames)
	if b.reflState == 0 && b.reflDisplay != 0 {
		b.reflDisplay = 0
		return
	}
	switch b.reflDisplay {
	case 0:
		if b.reflHitNow || b.reflState > 0 {
			b.reflDisplay = 1
			*ctr = 0
		}
	case 1:
		if *ctr++; *ctr > 3 {
			b.reflDisplay = 2
		}
	case 2:
		*ctr++
		if !b.reflHitNow {
			if *ctr > 6 {
				b.reflDisplay = 3
			}
		} else if *ctr > 6 {
			*ctr = 4
		}
	case 3:
		if *ctr++; *ctr >= n {
			b.reflDisplay = 0
			b.reflState = 0
		}
	}
}

// shieldHitBitmaps is the flip stage Draw_Shield_Hit_ and Draw_Reflection_
// Field_Hit_ share: the picture into bm, turned by method into bm or bm2.
// It returns the bitmap holding the result.
func shieldHitFlip(bm, bm2 *gfx.Picture, method int16, glassFirst bool) *gfx.Picture {
	switch method {
	case 0:
		if glassFirst {
			bm.SetGlassed()
		}
		bm2.DrawPictureToBitmap(bm)
		return bm
	case 1:
		bm2.FlipBitmap(bm, 0)
		bm.DrawPictureToBitmap(bm2)
		return bm
	case 2:
		bm2.FlipBitmap(bm, 3)
		bm.FlipBitmap(bm2, 0)
		bm2.DrawPictureToBitmap(bm)
		return bm
	case 3:
		bm2.FlipBitmap(bm, 3)
		bm.DrawPictureToBitmap(bm2)
		return bm
	}
	return bm
}

// drawShieldHit is Draw_Shield_Hit_ (0xa9aed): the target brightened under
// the shield picture's glass, then the shield over it; ends with a
// Get_Input like the original.
func (g *Game) drawShieldHit(x, y, t int16, pierced bool, mods, facing, ctr int16) {
	b := &g.tc.beam
	base := g.rotToBaseRot(facing)
	method := g.rotToRotMethod(facing)
	if b.shieldDisplay == 0 {
		return
	}
	var size int16
	if t == 0 {
		size = planetSizeToShieldSize(int16(g.planetSize()))
	} else {
		size = shipSizeToShieldSize(int16(g.tsp(t).u8(tsSize)))
	}
	diam := g.shieldDiameter(size)
	if g.tc.scr == nil {
		return
	}
	e := g.e
	sc := g.tc.scr
	bm := gfx.NewBlankBitmap(int(diam), int(diam))
	bm2 := gfx.NewBlankBitmap(int(diam), int(diam))
	pic := g.shieldAnims(base, size, pierced, mods)
	// 0xa9be9: the hit's sound, pitched down by the hull, fades on the
	// last frame or when the display went off.
	if ctr == 0 && b.shieldSound == 0 {
		n := 0x26
		if b.reflState != 0 {
			n = 0x2a
		}
		b.shieldSound = g.moxPlaySound(n)
		d := -0x3a98
		if t != 0 {
			d = int(g.tsp(t).u8(tsSize)) * -0x9c4
		}
		g.pitchBendSound(b.shieldSound, d)
	} else if int32(ctr) >= int32(pic.Frames)-1 || b.shieldSound != 0 && b.shieldDisplay == 0 {
		g.fadeoutSound(b.shieldSound, 0xa)
		b.shieldSound = 0
	}
	setFrame(pic, int(ctr%int16(pic.Frames)))
	bm.DrawPictureToBitmap(pic)
	if method >= 0 && method <= 3 {
		shieldHitFlip(bm, bm2, method, true)
	}
	v := e.V
	var dx, dy int16
	var body *gfx.Picture
	if t == 0 {
		cx, cy := g.shipCenterOffsets(0)
		ox, oy := g.getDrawShipOffsets(0)
		dx = x + diam/2 - cx + int16(ox)
		dy = y + diam/2 - cy + int16(oy)
		bm2.DrawPictureToBitmap(sc.planet)
		body = bm2
	} else {
		g.must2(g.drawShipToBitmap(t, 0, 0))
		cx, cy := g.shipCenterOffsets(t)
		ox, oy := g.getDrawShipOffsets(t)
		dx = x + diam/2 - cx + int16(ox)
		dy = y + diam/2 - cy + int16(oy)
		body = sc.drawShipBitmap
	}
	_, _, w, h := body.ActualSize()
	stale := g.maskBrightenBitmap(int(int16(w))/2-int(diam)/2, int(int16(h))/2-int(diam)/2, body, bm, 0xf0)
	v.SetWindow(0, 0, 0x27f, 0x15e)
	v.ClipOn()
	g.must2(e.Draw(int(dx), int(dy), body))
	for _, ti := range stale {
		if !g.trackStale {
			g.logOnce("Mask_Brighten_Bitmap: a level past the brighten palettes")
			break
		}
		g.staleScreen = append(g.staleScreen, image.Pt(int(dx)+ti%body.Width, int(dy)+ti/body.Width))
	}
	bm.SetGlassed()
	g.must2(e.Draw(int(x), int(y), bm))
	_, err := e.GetInput()
	g.must2(err)
}

// drawReflectionFieldHit is Draw_Reflection_Field_Hit_ (0xaa38d).
func (g *Game) drawReflectionFieldHit(x, y, size, facing, ctr int16) {
	b := &g.tc.beam
	base := g.rotToBaseRot(facing)
	method := g.rotToRotMethod(facing)
	k := shipSizeToShieldSize(size)
	diam := g.shieldDiameter(k)
	if b.reflDisplay == 0 || g.tc.scr == nil {
		return
	}
	pic := g.beamPic(int(k*5 + 0x59 + base))
	// 0xaa3f9: as Draw_Shield_Hit_'s, pitched by the hull size.
	if ctr == 0 && b.reflSound == 0 {
		n := 0x26
		if b.reflState != 0 {
			n = 0x2a
		}
		b.reflSound = g.moxPlaySound(n)
		g.pitchBendSound(b.reflSound, int(size)*-0x9c4)
	} else if int32(ctr) >= int32(pic.Frames)-1 {
		g.fadeoutSound(b.reflSound, 0xa)
		b.reflSound = 0
	}
	setFrame(pic, int(ctr%int16(pic.Frames)))
	bm := gfx.NewBlankBitmap(int(diam), int(diam))
	bm2 := gfx.NewBlankBitmap(int(diam), int(diam))
	bm.DrawPictureToBitmap(pic)
	var out *gfx.Picture
	switch method {
	case 0:
		bm.SetGlassed()
		out = bm
	case 1, 3:
		bm2.FlipBitmap(bm, int(method-1)*3/2)
		bm2.SetGlassed()
		out = bm2
	case 2:
		bm2.FlipBitmap(bm, 3)
		bm.FlipBitmap(bm2, 0)
		bm.SetGlassed()
		out = bm
	default:
		return
	}
	g.must2(g.e.Draw(int(x), int(y), out))
}

// beamSFXPlay is Beam_SFX_ (0xa8197): the records of a's volley at t
// (pixels a(ax, ay), t(tx, ty); mode 1: t is a missile stack) played in
// two slots, frame by frame, with shield and reflection hits and the
// damage numbers; missiles lose the killed count as the beams arrive. The
// seed is restored at the end. A reflected record plays recursively.
func (g *Game) beamSFXPlay(a, ax, ay, t, tx, ty int16, recs *beamRecs, mode int16) {
	tc := g.tc
	b := &tc.beam
	r := &g.state.Random
	draw := tc.scr != nil
	cur := [2]int16{0, -1}
	var fr, offX, offY, cnt [2]int16
	seed := r.Seed
	var fpX, fpY int16
	k := int16(0)
	rng := getShipRange(ax, ay, tx, ty)
	b.pierced = false
	if b.fp[0] == nil {
		g.must2(g.loadFPData())
	}
	if draw {
		g.e.RestoreGlassRemapBlock(tc.scr.shieldRemap)
	}
	g.shieldAnims(0, 0, true, 0)
	b.oldColor = [2]int16{-1, -1}
	if w := recs.at(0).weapon; w != 0x2e && g.wpnU8(int(w), 0xc) != 0 {
		return
	}
	facA := int16(g.tsp(a).u8(tsFacing))
	facT := int16(g.tspAny(t).u8(tsFacing))
	acx, acy := g.shipCenterOffsets(a)
	acx += ax
	acy += ay
	var tcx, tcy int16
	if mode != 0 {
		tcx, tcy = tx, ty
	} else {
		tcx, tcy = g.shipCenterOffsets(t)
		tcx += tx
		tcy += ty
	}
	if recs.at(cur[0]).mods&0x80 != 0 {
		cnt[0] = 3
	}
	done := false
	for !done {
		if draw {
			g.e.MarkTime() // 0xa8335, 0xa8952: the frame's Release_Time_(1) counts from here
			g.e.V.SetPageOff()
			g.must2(g.drawMainCombatScreen())
			g.e.V.SetWindow(0, 0, 0x27f, 0x15e)
			g.e.V.ClipOn()
		}
		keep := false
		redraw := func() {
			if !draw {
				return
			}
			sc := tc.scr
			if sc.dontDrawShip == t && mode == 0 {
				g.must2(g.drawShip(t, int(tx), int(ty)))
			}
			if sc.dontDrawShip == a {
				g.must2(g.drawShip(a, int(ax), int(ay)))
			}
		}
		redraw()
		for s := int16(0); s < 2; s++ {
			o := 1 - s
			if cur[s] > -1 {
				rec := recs.at(cur[s])
				w, shots := rec.weapon, rec.shots
				A := g.tsp(a)
				pic := A.i16(tsPicture)
				owner := int16(A.u8(tsOwner))
				var arc int16
				if mode != 0 {
					arc = facingShield(g.relativeBearingXY(a, tx, ty))
				} else {
					arc = facingShield(g.relativeBearing(a, t))
				}
				fps := int16(1)
				if a > 0 && owner < 8 && b.reflState != 2 {
					fps = g.nFirePoints(w, shots)
				}
				if fr[s] == 0 {
					switch {
					case rec.result == 0 && rec.pierced == 0 && rec.refl == 0 && t != 0 && mode == 0:
						d := min(g.shieldDiameter(shipSizeToShieldSize(int16(g.tsp(t).u8(tsSize))))/2, rng/2)
						h := d / 2
						offY[s] = int16(r.Random(uint32(int32(d)))) - h
						offX[s] = int16(r.Random(uint32(int32(d)))) - h
					case t == 0 && rec.dmg == 0:
						d := min(rng/2, 0x1e)
						h := d / 2
						q := int16(int32(d) / 4)
						v := int16(r.Random(uint32(int32(q))))
						offY[s] = (int16(r.Random(2)) - 1) * (h + v)
						v = int16(r.Random(uint32(int32(q))))
						offX[s] = (int16(r.Random(2)) - 1) * (h + v)
					case mode != 0 && rec.dmg == 0:
						d := min(g.shieldDiameter(0)/2, rng/2)
						h := d / 2
						q := int16(int32(d)/4) + 1
						v := int16(r.Random(uint32(int32(q))))
						offY[s] = (int16(r.Random(2)) - 1) * (h + v)
						v = int16(r.Random(uint32(int32(q))))
						offX[s] = (int16(r.Random(2)) - 1) * (h + v)
					default:
						offY[s], offX[s] = 0, 0
					}
				}
				for fp := int16(0); fp < fps; fp++ {
					if a > 0 {
						fpX, fpY = g.calcFPOffsets(w, shots, owner, pic, uint8(facA), uint8(arc), fp)
					}
					g.fireWeapon(acx+fpX, acy+fpY, tcx+offY[s], tcy+offX[s], facA, facT, w, fr[s], s,
						int16(g.tspAny(t).u8(tsSize)), rec.result, rec.pierced, mode, fp, b.reflState, int16(g.tsp(a).u8(tsSize)), rec.mods)
				}
			}
			advance := false
			switch {
			case cur[s] >= 0x10:
			case cur[s] > -1:
				advance = b.maxFrames <= fr[s]
			default:
				oc := cur[o]
				switch {
				case oc == -1 || oc >= 0x10:
				case oc < -1:
					// cur[o] is -1000 after a reflection: the original reads the
					// record array at -1000 (stack garbage) unless fr[o] < 9 decides.
					if fr[o] >= 9 {
						g.logOnce("Beam_SFX_: a slot reads record -1000 (stale stack)")
					}
					advance = true
				case recs.at(oc).weapon <= 0 || fr[o] < 9:
					advance = true
				}
			}
			if advance {
				cnt[s]--
				fr[s] = 0
				if cnt[s] < 1 {
					cnt[s] = 0
					next := k + 1
					nr := recs.at(next)
					switch {
					case nr.weapon <= 0 || next >= 0x10:
						cur[s] = -1
					case nr.refl == 0:
						k++
						cur[s] = k
						if recs.at(k+1).refl != 0 {
							b.reflState = 1
							b.reflDisplay = 1
						}
						if recs.at(k).mods&0x80 != 0 {
							cnt[s] = 3
						}
					case cur[o] == -1000 && (b.shipHitNow[o] || cur[s] <= -1):
						b.shieldHitNow = false
						b.shieldDisplay = 0
						var loc beamRecs
						k++
						loc[0] = *recs.at(k)
						b.reflState = 2
						b.firingReflected = true
						g.beamSFXPlay(t, tx, ty, a, ax, ay, &loc, 0)
						b.firingReflected = false
						b.reflDisplay = 0
						cur[o] = -1000
						if draw {
							g.e.V.SetPageOff()
							g.must2(g.drawMainCombatScreen())
							g.e.V.SetWindow(0, 0, 0x27f, 0x15e)
							g.e.V.ClipOn()
						}
						keep = true
						redraw()
					default:
						cur[s] = -1000
						if recs.at(k+1).refl != 0 {
							b.reflState = 1
							b.reflDisplay = 1
						}
					}
				}
			} else {
				if tc.speedx2 == 0 && tc.netFlag == 0 {
					fr[s]++
				} else if fr[s]+2 < b.maxFrames {
					fr[s] += 2
				} else {
					fr[s]++
				}
			}
			if cur[0] <= -1 && cur[1] <= -1 {
				b.shieldHitNow = false
				b.reflHitNow = false
			}
		}
		if cur[0] <= -1 && cur[1] <= -1 && b.shieldDisplay == 0 && b.reflDisplay == 0 && g.doneDrawingDamageMessageQueue() && !keep {
			done = true
			b.reflState = 0
		}
		if mode == 0 {
			g.adjustShieldDisplayStatus(&b.fCtr)
			var pierced byte
			if cur[0] > -1 {
				pierced = byte(recs.at(cur[0]).pierced)
			}
			if cur[1] > -1 {
				pierced += byte(recs.at(int16(b2i(cur[0] == 0))).pierced)
			}
			if b.pierced {
				pierced = 1
			}
			var mods int16
			if cur[0] > -1 {
				mods = recs.at(cur[0]).mods & 0x100
			}
			if t != 0 {
				d := g.shieldDiameter(shipSizeToShieldSize(int16(g.tsp(t).u8(tsSize))))
				g.drawShieldHit(tcx-d/2, tcy-d/2, t, pierced != 0, mods, g.getSFXDrawShieldFacing2(a, t), b.fCtr)
			} else {
				d := g.shieldDiameter(planetSizeToShieldSize(int16(g.planetSize())))
				g.drawShieldHit(tcx-d/2, tcy-d/2, t, pierced != 0, 0, g.getSFXDrawShieldFacing2(a, t), b.fCtr)
			}
			if b.reflDisplay != 0 {
				switch b.reflState {
				case 2:
					g.adjustReflectionFieldDisplayStatus(&b.reflFCtr)
					cx, cy := g.shipCenterOffsets(a)
					cx += ax
					cy += ay
					size := int16(g.tsp(a).u8(tsSize))
					d := g.reflDiameter(shipSizeToShieldSize(size))
					g.drawReflectionFieldHit(cx-d/2, cy-d/2, size, g.getSFXDrawShieldFacing2(t, a), b.reflFCtr)
				case 1:
					g.adjustReflectionFieldDisplayStatus(&b.reflFCtr)
					size := int16(g.tsp(t).u8(tsSize))
					d := g.reflDiameter(shipSizeToShieldSize(size))
					cx, cy := g.shipCenterOffsets(t)
					cx += tx
					cy += ty
					g.drawReflectionFieldHit(cx-d/2, cy-d/2, size, g.getSFXDrawShieldFacing2(a, t), b.reflFCtr)
				}
			}
		} else {
			b.shieldDisplay = 0
			b.reflDisplay = 0
		}
		for s := 0; s < 2; s++ {
			if cur[s] <= -1 {
				continue
			}
			rec := recs.at(cur[s])
			if mode == 0 {
				if b.shipHitNow[s] && (rec.dmg > 0 || rec.abs > 0) {
					f := g.getSFXDrawShieldFacing2(t, a)
					g.addDamageIndicatorForShip(tx, ty, t, byte(b2i(int32(f+4)%0x10 < 8)), rec.dmg, rec.abs)
					g.moxPlaySound(0x1d) // 0xa8ea7
					rec.dmg, rec.abs = -1, -1
				}
				continue
			}
			if (rec.dmg > 0 || rec.abs > 0) && b.shipHitNow[s] {
				g.moxPlaySound(0x1c) // 0xa8f8c
				g.addDamageIndicatorForMissile(tcx, tcy, 1, rec.dmg)
				M := g.msl(t)
				M.set16(tmCount, M.i16(tmCount)-rec.dmg)
				if M.i16(tmCount) <= 0 {
					g.killMissile(t)
				}
				rec.dmg = -1
			}
		}
		g.drawDamageMessageQueue()
		if draw {
			g.must2(g.e.ReleaseTime(1))
			g.must2(g.e.TogglePages())
			g.e.V.ClipOff()
			g.e.V.ResetWindow()
		}
	}
	b.shipFiring = [2]bool{}
	b.shipHitNow = [2]bool{}
	b.weaponSound, b.shieldSound = [2][5]int{}, 0 // forgotten, not stopped
	b.shieldDisplay = 0
	b.reflDisplay = 0
	if draw {
		g.e.V.ClipOff()
	}
	r.Set(seed)
}

// tspAny is record i even past the array, where Beam_SFX_ indexes records
// by a missile number (as zeros there; the original reads on in memory).
func (g *Game) tspAny(i int16) *tship {
	if i < 0 || int(i) >= len(g.tc.ship) {
		g.logOnce("a combat record read past the array (missile number)")
		return &tship{}
	}
	return g.tsp(i)
}
