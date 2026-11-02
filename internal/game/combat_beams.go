package game

import (
	"github.com/openorion2/openorion2/internal/gfx"
)

// The beams' drawing (BEAMS.C): Beam_SFX_ plays the records Fire_Ship_
// collected, with fire points, bolts, bursts and explosions on the ships,
// shield and reflection field hits and the damage numbers. Spec:
// docs/screens/combat-screen.md ("Beams").

const (
	beamSteveStructs = 0xa6646 // Build_Steve_Stuff_Struct_For_Weapon_'s 0x2c-byte rows
	beamRotBase      = 0xa6606 // Rot_To_Base_Rot_'s 16 int16
	beamRotMethod    = 0xa6626 // Rot_To_Rot_Method_'s 16 int16
	beamDmgColors    = 0xa659f // Draw_Damage_Indicator_'s 27-byte colour rows
	beamDmgColors2   = 0xa65ba
	beamDmgColors3   = 0xa65d5
	beamShieldDiam   = 0x182477 // _shield_diameters
	beamReflDiam     = 0x182485 // _reflection_field_diameters
	beamColorDepth   = 0x182475 // _color_depth
	beamLowRamp      = 0x182490 // _low_ramp_values (int32)
	beamCross        = 0x1824e0 // _low/_moderate/_high_primary_cross_values, 0xf0 apart
	beamSaturated    = 0x182530 // _low/_moderate/_high_saturated_values, 0xf0 apart
	beamFirePtsLBX   = 0x182a3b // _FIREPTS_LBX
)

// beamState is BEAMS.C's state and pictures.
type beamState struct {
	pierced, shieldHitNow, reflHitNow bool
	shipHitNow, shipFiring            [2]bool
	// Sound handles: Fire_Weapon_'s per side and mount (0x182443),
	// Draw_Shield_Hit_'s (0x18246b), Draw_Reflection_Field_Hit_'s (0x18246f).
	weaponSound                [2][5]int
	shieldSound, reflSound     int
	firingReflected            bool
	maxFrames, fCtr, reflFCtr  int16
	shieldDisplay, reflDisplay int16
	reflState                  int16
	oldColor                   [2]int16
	msgs                       [0x24]damageMsg
	nMsgs                      int16

	burst     [2][2][16]*gfx.Picture // _laser_burst_anim[set][hit][facing]
	explosion [2][16]*gfx.Picture
	fp        [8][]byte // _fp_data_ptr: fire points by ship picture set
	brighten  [16][256]byte
	mauler    *gfx.Picture
}

// damageMsg is a _msg_queue entry (0xd bytes).
type damageMsg struct {
	x, y, dmg, abs, bio, age int16
	right                    byte
}

func (g *Game) beamsLBX() string { return g.cmbtLBX(cmbtBeamsLBX) }

func (g *Game) beamPic(n int) *gfx.Picture {
	p, err := g.cached(g.beamsLBX(), n)
	g.must2(err)
	return p
}

// loadCombatBeamStuff is Load_Combat_Beam_Stuff_ (0xa7518) up to the
// reflection palette, which the caller draws.
func (g *Game) loadCombatBeamStuff() error {
	b := &g.tc.beam
	b.oldColor = [2]int16{-1, -1}
	if err := g.loadFPData(); err != nil {
		return err
	}
	n := 1
	for set := 0; set < 2; set++ {
		if set == 1 {
			n = 0x21
		}
		for hit := 0; hit < 2; hit++ {
			for f := 0; f < 16; f++ {
				b.burst[set][hit][f] = g.beamPic(n)
				n++
			}
		}
	}
	for k := 0; k < 2; k++ {
		for f := 0; f < 16; f++ {
			p, err := g.picture(g.beamsLBX(), 0x41+k)
			if err != nil {
				return err
			}
			b.explosion[k][f] = p
		}
	}
	for size := 0; size < 4; size++ {
		for rot := 0; rot < 5; rot++ {
			g.beamPic(0x43 + size*5 + rot)
		}
	}
	var pct [16]int
	for i := range pct {
		pct[i] = 0x1e + 4*i
	}
	g.createBrightenBitmapPalettes(pct[:])
	b.mauler = g.beamPic(0x58)
	return nil
}

// loadFPData is Load_FP_Data_From_LBX_ (0xc6af9): the fire points of the
// eight ship picture sets. The port loads them off screen too, since they
// move the beams' start and so their frame counts.
func (g *Game) loadFPData() error {
	b := &g.tc.beam
	a, err := g.archive(g.cmbtLBX(beamFirePtsLBX))
	if err != nil {
		return err
	}
	tab, err := a.Table(0)
	if err != nil {
		return err
	}
	for i := range b.fp {
		b.fp[i] = tab.Record(i)
	}
	return nil
}

// createBrightenBitmapPalettes is Create_Brighten_Bitmap_Palettes
// (0x12e9a1): per level, every colour brightened in HLS space.
func (g *Game) createBrightenBitmapPalettes(pct []int) {
	b := &g.tc.beam
	pal := &g.e.V.Palette
	for j := 0; j < 256; j++ {
		h, l, s := rgbToHLS(pal[j].R, pal[j].G, pal[j].B)
		for i, p := range pct {
			t := l + p*10*l/1000
			if t > 1000 {
				t = 1000
			}
			r, gg, bb := hlsToRGB(h, t, s)
			b.brighten[i][j] = g.e.FindClosestColor(int(r), int(gg), int(bb))
		}
	}
}

// rgbToHLS is RGB_To_HLS (0x132137), in thousandths; the hue keeps its
// quirk of adding 360 before the division by 1000.
func rgbToHLS(r, g, b byte) (hue, lum, sat int) {
	fr := uint32(r) * 1000 / 0x3f
	fg := uint32(g) * 1000 / 0x3f
	fb := uint32(b) * 1000 / 0x3f
	mx := max(fr, fg, fb)
	mn := min(fr, fg, fb)
	lum = int((mx + mn) / 2)
	if mx == mn {
		return -1, lum, 0
	}
	if lum < 0x1f5 {
		sat = int(int32((mx-mn)*1000) / int32(mx+mn))
	} else {
		sat = int(int32((mx-mn)*1000) / int32(2000-mx-mn))
	}
	d := int32(mx - mn)
	switch {
	case fr == mx:
		hue = int(int32((fg-fb)*1000) / d)
	case fg == mx:
		hue = int(int32((fb-fr)*1000)/d + 2000)
	case fb == mx:
		hue = int(int32((fr-fg)*1000)/d + 4000)
	}
	hue *= 0x3c
	if hue < 0 {
		hue += 0x168
	}
	return int(int32(hue) / 1000), lum, sat
}

// hlsToRGB is HLS_To_RGB (0x13234e).
func hlsToRGB(hue, lum, sat int) (r, g, b byte) {
	var m2 int
	if lum < 0x1f5 {
		m2 = (sat + 1000) * lum / 1000
	} else {
		m2 = lum + sat - lum*sat/1000
	}
	m1 := lum*2 - m2
	if sat == 0 {
		if hue == -1 {
			v := byte(lum * 0x3f / 1000)
			return v, v, v
		}
		return 0, 0, 0
	}
	return hlsValue(m1, m2, hue+0x78), hlsValue(m1, m2, hue), hlsValue(m1, m2, hue-0x78)
}

// hlsValue is Value (0x132461).
func hlsValue(n1, n2, hue int) byte {
	switch {
	case hue > 0x168:
		hue -= 0x168
	case hue < 0:
		hue += 0x168
	}
	switch {
	case hue < 0x3c:
		return byte((hue*1000/0x3c*(n2-n1)/1000 + n1) * 0x3f / 1000)
	case hue < 0xb4:
		return byte(n2 * 0x3f / 1000)
	case hue < 0xf0:
		return byte(((0xf0-hue)*1000/0x3c*(n2-n1)/1000 + n1) * 0x3f / 1000)
	}
	return byte(n1 * 0x3f / 1000)
}

// maskBrightenBitmap is Mask_Brighten_Bitmap (0x12eb23): the target's
// pixels under the mask's glass colours brightened by level. The y clip
// tests the mask's width, as the original does. It returns the target
// pixels whose level (a byte, mask - start) lies past the 16 palettes.
func (g *Game) maskBrightenBitmap(x, y int, target, mask *gfx.Picture, start byte) (stale []int) {
	b := &g.tc.beam
	mw, mh := mask.Width, mask.Height
	mpix := mask.Pixels()
	if x < 0 && -x >= mw {
		return
	}
	if y < 0 && -y >= mh {
		return
	}
	mstart := 0
	switch {
	case x < 0 && y < 0:
		mstart = -y*mw - x
	case x < 0:
		mstart = -x
	case y < 0:
		mstart = -y * mw
	}
	tw, th := target.Width, target.Height
	tpix := target.Pixels()
	if y > th || x > tw {
		return
	}
	tstart := 0
	switch {
	case x >= 0 && y >= 0:
		tstart = tw*y + x
	case x < 0 && y >= 0:
		tstart = tw * y
	case x >= 0:
		tstart = x
	}
	mskip := 0
	if x < 0 {
		mskip = -x
	}
	if tw < x+mw {
		mskip += x + mw - tw
	}
	tskip := max(x, 0)
	if x+mw < tw {
		tskip += tw - x - mw
	}
	x1 := max(x, 0)
	x2 := min(x+mw, tw)
	y1 := max(y, 0)
	y2 := th
	if y+mw < th {
		y2 = y + mh
	}
	if mstart < 0 {
		mstart = 0
	}
	if tstart < 0 {
		tstart = 0
	}
	w, h := x2-x1, y2-y1
	mi, ti := mstart, tstart
	for r := 0; r < h; r++ {
		for c := 0; c < w; c++ {
			if mi < len(mpix) && ti < len(tpix) && mpix[mi] != 0 && tpix[ti] != 0 {
				if l := mpix[mi] - start; l < 16 {
					tpix[ti] = b.brighten[l][tpix[ti]]
				} else {
					// The original reads level*256 bytes into the screen segment
					// past the palettes (Mask_Brighten_Bitmap_Pixels_ 0x147f7b).
					stale = append(stale, ti)
				}
			}
			mi++
			ti++
		}
		mi += mskip
		ti += tskip
	}
	return stale
}
