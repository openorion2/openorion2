package game

import "encoding/binary"

// WLIBS\special\angle.c: integer trigonometry on whole degrees. The tables
// live in the code segment of ORION2.EXE and are read from it.

const (
	atanTable = 0x138385 // 128 bytes: atan(i/128) in degrees
	sinTable  = 0x138405 // _angle_to_sin, uint16[90]: sin * 65536
)

// getAngle is Get_Angle (0x1384b9): the angle of (x, y) in degrees, 0..359.
func (g *Game) getAngle(x, y int) int {
	switch {
	case x == 0:
		switch {
		case y < 0:
			return 0x10e
		case y < 1:
			return 0
		}
		return 0x5a
	case y == 0:
		if x < 0 {
			return 0xb4
		}
		return 0
	}
	var corr, a, b int
	switch {
	case x < 0 && y < 0:
		a, b, corr = -x, -y, 0xb4
	case x < 0:
		a, b, corr = y, -x, 0x5a
	case y < 0:
		a, b, corr = -y, x, 0x10e
	default:
		a, b = x, y
	}
	tab := g.exe.bytes(atanTable, 0x80)
	var r int
	switch {
	case a == b:
		r = 0x2d
	case a < b:
		r = 0x5a - int(tab[uint32(a<<7)/uint32(b)])
	default:
		r = int(tab[uint32(b<<7)/uint32(a)])
	}
	r = int(int16(r + corr))
	if r == 0x168 {
		r = 0
	}
	return r
}

// sin is Sin (0x138615): length * sin(angle), rounded.
func (g *Game) sin(angle, length int) int {
	angle = int(int16(angle))
	if length < 0 {
		length = -length
		angle = int(int16(angle + 0xb4))
	}
	if angle < 0x168 {
		for angle < 0 {
			angle += 0x168
		}
	} else {
		for angle > 0x167 {
			angle -= 0x168
		}
	}
	tab := func(i int) int { return int(binary.LittleEndian.Uint16(g.exe.bytes(uint32(sinTable+2*i), 2))) }
	mul := func(t int) int { return int(uint32(t)*uint32(length)+0x8000) >> 16 }
	switch {
	case angle < 0x5a:
		return mul(tab(angle))
	case angle == 0x5a:
		return length
	case angle < 0xb4:
		return mul(tab(0xb4 - angle))
	case angle < 0x10e:
		return -mul(tab(angle - 0xb4))
	case angle == 0x10e:
		return -length
	}
	return -mul(tab(0x168 - angle))
}

// cos is Cos: Sin of angle + 90.
func (g *Game) cos(angle, length int) int { return g.sin(int(int16(angle+0x5a)), length) }

// interpolateLine is Interpolate_Line (0x13432d): move (x, y) step units
// toward (ex, ey) without overshooting.
func (g *Game) interpolateLine(x, y *int16, ex, ey int16, step int) {
	if step <= 0 || (*x == ex && *y == ey) {
		return
	}
	dx, dy := ex-*x, ey-*y
	abs := func(v int16) int {
		if v < 0 {
			return -int(v)
		}
		return int(v)
	}
	var a int
	if abs(dx) < 0x100 && abs(dy) < 0x100 {
		a = g.getAngle(int(dx), int(dy))
	} else {
		a = g.getAngle(int(dx)/2, int(dy)/2)
	}
	if *x < ex {
		*x += int16(g.cos(a, step))
		if ex < *x {
			*x = ex
		}
	} else {
		*x += int16(g.cos(a, step))
		if *x < ex {
			*x = ex
		}
	}
	if *y < ey {
		*y += int16(g.sin(a, step))
		if ey < *y {
			*y = ey
		}
	} else {
		*y += int16(g.sin(a, step))
		if *y < ey {
			*y = ey
		}
	}
}

// clampMapOrigin applies Center_Map_'s and Scroll_Map_'s clamps.
func (g *Game) clampMapOrigin(x, y int16) (int16, int16) {
	s := &g.state
	x -= int16(g.upScaled(0xfd))
	y -= int16(g.upScaled(200))
	if x < 0 {
		x = 0
	}
	if int(s.MapMaxX)-g.upScaled(0x1fa) < int(x) {
		x = s.MapMaxX - int16(g.upScaled(0x1fa))
	}
	if y < 0 {
		y = 0
	}
	if int(s.MapMaxY)-g.upScaled(400) < int(y) {
		y = s.MapMaxY - int16(g.upScaled(400))
	}
	return x, y
}

// centerMap is Center_Map_ (0x8238b).
func (g *Game) centerMap(mx, my int16) {
	s := &g.state
	ox, oy := s.CurMapX, s.CurMapY
	s.CurMapX, s.CurMapY = g.clampMapOrigin(mx, my)
	if s.CurMapX != ox || s.CurMapY != oy {
		g.getNewParalaxXYs(ox, oy, 1)
	}
	g.ms.drawToBack = true
}

// scrollMap is Scroll_Map_ (0x89df9): long moves are animated in steps of
// 50 map units with full redraws.
func (g *Game) scrollMap(mx, my int16) error {
	e, m, s := g.e, &g.ms, &g.state
	x, y := g.clampMapOrigin(mx, my)
	r := rangeXY(s.CurMapX, s.CurMapY, x, y)
	if r < 0x33 || (x == s.CurMapX && y == s.CurMapY) {
		m.animatedStar = -1
		m.drawToBack = true
		s.CurMapX, s.CurMapY = x, y
		return nil
	}
	steps := int(r / 0x32)
	e.V.SetRefreshFull()
	for i := 0; i < steps; i++ {
		g.getNewParalaxXYs(x, y, steps)
		g.interpolateLine(&s.CurMapX, &s.CurMapY, x, y, 0x32)
		m.animatedStar = -1
		e.V.SetPageOff()
		if err := g.drawMainScreenFilled(false); err != nil {
			return err
		}
		if err := g.drawMainScreen(); err != nil {
			return err
		}
		if err := e.TogglePages(); err != nil {
			return err
		}
	}
	m.animatedStar = -1
	e.V.SetPageOff()
	if err := g.drawMainScreenFilled(true); err != nil {
		return err
	}
	e.V.SetRefreshStencil()
	return nil
}
