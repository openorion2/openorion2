package game

import "encoding/binary"

// WLIBS bolt lines (Bolt_Line, Wide_Bolt_Line, Bolt_Segment) and
// Rotate_Coordinates: the beams' plasma strokes. Spec:
// docs/screens/combat-screen.md ("Beams").

const (
	headWideBolts = 0x184670 // 16 rows of 12 int16: sides 1..4, edges 1..2 (x, y)
	tailWideBolts = 0x1847f0
	wideBoltRow   = 0x18
)

// boltBuffer is graphics_buffer (0x1bc89c), the colour run of a segment.
type boltBuffer [0x1000]byte

// boltSegment is Bolt_Segment (0x13bec4): a plasma line whose colours ramp
// from start over count with random jitter; the seed is restored after.
func (g *Game) boltSegment(x1, y1, x2, y2, start, count, chance, maxChange, first, last, fading, interlock, rotation, head int16) {
	r := &g.state.Random
	seed := r.Seed
	buf := &g.boltBuf
	n := int32(maxChange)*2 + 1
	dx := int32(x2) - int32(x1)
	dy := int32(y2) - int32(y1)
	if dx < 0 {
		dx = -dx
	}
	if dy < 0 {
		dy = -dy
	}
	l := dy + 1
	if dx > dy {
		l = dx + 1
	}
	step := (int32(count) << 16) / l
	acc := int32(32000) / l
	add := int32(0)
	put := func(i int32) {
		buf[i] = byte(int32(start) + acc>>16 + add)
		acc += step
		if int32(r.Random(100)) < int32(chance) {
			add = int32(r.Random(uint32(n))) - int32(maxChange)
		} else {
			add = 0
		}
	}
	if y2 < y1 {
		for i := int32(0); i <= l; i++ {
			put(l - i + 1)
		}
		buf[l] = byte(start)
		buf[0] = byte(start + count - 1)
	} else {
		for i := int32(0); i <= l; i++ {
			put(i + 1)
		}
		buf[0] = byte(start)
		buf[l] = byte(start + count - 1)
	}
	if interlock == 1 {
		for i := int32(rotation) % 8; l+2 > i; i += 8 {
			buf[i] += 3
		}
	}
	for i := int32(0); l+2 > i; i++ {
		if int16(buf[i]) < first {
			buf[i] = byte(first)
		}
		if int16(buf[i]) > last {
			buf[i] = byte(last)
		}
	}
	if head != 0 {
		inc := byte(3 / int32(head))
		if y2 < y1 {
			for i := int32(0); i <= 5; i++ {
				buf[i+1] += inc
			}
		} else if l > 5 {
			for i := int32(0); i <= 5; i++ {
				buf[l-i-1] += inc
			}
		}
	}
	top := int32(start) + int32(count)
	for i := int32(0); l+2 > i; i++ {
		if int32(buf[i]) > top {
			buf[i] = byte(top)
		}
	}
	g.e.V.PlasmaLine(int(x1), int(y1), int(x2), int(y2), buf[:], 0)
	r.Set(seed)
}

// boltLine is Bolt_Line (0x13bafb): a centre segment between two side
// segments, with an anti-aliased head.
func (g *Game) boltLine(x1, y1, x2, y2, start, count, chance, maxChange, rotation, aaHead, triangular, glowing, fading, interlock, noAA, cMax, cMin, sMax, sMin int16) {
	v := g.e.V
	if aaHead == 1 {
		card, corner := start+1, start
		if glowing == 1 {
			card, corner = start+4, start+1
			if count < card {
				card = count
			}
			if count < corner {
				corner = count
			}
		}
		dots := func(x, y int16) {
			v.Dot(int(x), int(y), byte(cMax+start))
			v.Dot(int(x), int(y-1), byte(card))
			v.Dot(int(x), int(y+1), byte(card))
			v.Dot(int(x-1), int(y), byte(card))
			v.Dot(int(x+1), int(y), byte(card))
			v.Dot(int(x-1), int(y-1), byte(corner))
			v.Dot(int(x+1), int(y-1), byte(corner))
			v.Dot(int(x-1), int(y+1), byte(corner))
			v.Dot(int(x+1), int(y+1), byte(corner))
		}
		dots(x2, y2)
		if fading == 0 {
			dots(x1, y1)
		}
	}
	ax, ay := int32(x2)-int32(x1), int32(y2)-int32(y1)
	if ax < 0 {
		ax = -ax
	}
	if ay < 0 {
		ay = -ay
	}
	var ox, oy int16
	if ax > ay {
		oy = 1
	} else {
		ox = 1
	}
	sStart := start + sMin
	sCount := sMax - sMin
	if glowing == 1 {
		sCount += 3
	}
	if noAA == 0 {
		g.boltSegment(x1-ox, y1-oy, x2-ox, y2-oy, sStart, sCount, chance, maxChange/2, start, count+start-1, 0, interlock, rotation, 0)
		g.boltSegment(x1+ox, y1+oy, x2+ox, y2+oy, sStart, sCount, chance, maxChange/2, start, count+start-1, 0, interlock, rotation, 0)
	}
	cStart := start + cMin
	cCount := cMax - cMin
	if fading == 0 {
		cStart += int16(int32(cCount) * 3 / 4)
		cCount = 0
	}
	g.boltSegment(x1, y1, x2, y2, cStart, cCount, chance, maxChange, start, count+start-1, fading, interlock, rotation, glowing)
}

// wideBoltLine is Wide_Bolt_Line (0x13c22c): a centre segment with four
// sides and two edges whose offsets come from tables by angle.
func (g *Game) wideBoltLine(x1, y1, x2, y2, start, count, chance, maxChange, rotation, aaHead, triangular, glowing, fading, interlock, noAA, cMax, cMin, sMax, sMin, eMax, eMin int16) {
	a := int16(0x168 - g.getAngle(int(x2)-int(x1), int(y2)-int(y1)) + 0xb)
	ang := int16(int32(int32(a)%0x168*2) / 0x2d)
	switch {
	case ang > 0xf:
		ang = 0xf
	case ang < 0:
		ang = 0
	}
	tab := func(base uint32, k int) int16 {
		return int16(binary.LittleEndian.Uint16(g.exe.bytes(base+uint32(ang)*wideBoltRow+uint32(2*k), 2)))
	}
	head := func(k int) int16 { return tab(headWideBolts, k) }
	tail := func(k int) int16 { return tab(tailWideBolts, k) }
	last := start + count - 1
	var l1x, l1y, l2x, l2y int16
	if triangular == 1 {
		l1x, l1y, l2x, l2y = x1, y1, x1, y1
	} else {
		l1x, l1y = x1+tail(8), y1+tail(9)
		l2x, l2y = x1+tail(10), y1+tail(11)
	}
	g.boltSegment(l1x, l1y, x2+head(8), y2+head(9), start+eMin, eMax-eMin, chance, maxChange/2, start, last, 0, interlock, rotation, 0)
	g.boltSegment(l2x, l2y, x2+head(10), y2+head(11), start+eMin, eMax-eMin, chance, maxChange/2, start, last, 0, interlock, rotation, 0)
	for k := 0; k < 4; k++ {
		sx, sy := x1, y1
		if triangular != 1 {
			sx, sy = x1+tail(2*k), y1+tail(2*k+1)
		}
		g.boltSegment(sx, sy, x2+head(2*k), y2+head(2*k+1), start+sMin, sMax-sMin, chance, maxChange/2, start, last, 0, interlock, rotation, glowing)
	}
	cStart := start + cMin
	cCount := cMax - cMin
	if fading == 0 {
		cStart += int16(int32(cCount) * 3 / 4)
		cCount = 0
	}
	g.boltSegment(x1, y1, x2, y2, cStart, cCount, chance, maxChange, start, last, fading, interlock, rotation, glowing)
}

// rotateCoordinates is Rotate_Coordinates (0x1387b2): (x, y) turned by
// angle degrees in 16.16 fixed point, rounded half away from zero.
func (g *Game) rotateCoordinates(angle int16, x, y *int32) {
	for angle >= 0x168 {
		angle -= 0x168
	}
	for angle < 0 {
		angle += 0x168
	}
	t := func(i int16) int32 {
		return int32(binary.LittleEndian.Uint16(g.exe.bytes(uint32(sinTable+2*int32(i)), 2)))
	}
	var sx, sy, cx, cy int32
	switch {
	case angle == 0:
		return
	case angle < 0x5a:
		sx, sy, cx, cy = *x*t(angle), *y*t(angle), *x*t(0x5a-angle), *y*t(0x5a-angle)
	case angle == 0x5a:
		*x, *y = -*y, *x
		return
	case angle < 0xb4:
		sx, sy, cx, cy = *x*t(0xb4-angle), *y*t(0xb4-angle), *x*-t(angle-0x5a), *y*-t(angle-0x5a)
	case angle == 0xb4:
		*x, *y = -*x, -*y
		return
	case angle < 0x10e:
		sx, sy, cx, cy = *x*-t(angle-0xb4), *y*-t(angle-0xb4), *x*-t(0x10e-angle), *y*-t(0x10e-angle)
	case angle == 0x10e:
		*x, *y = *y, -*x
		return
	default:
		sx, sy, cx, cy = *x*-t(0x168-angle), *y*-t(0x168-angle), *x*t(angle-0x10e), *y*t(angle-0x10e)
	}
	round := func(v int32) int32 {
		if v < 0 {
			return -int32((uint32(0x8000) - uint32(v)) >> 16)
		}
		return int32((uint32(v) + 0x8000) >> 16)
	}
	*x, *y = round(cx-sy), round(sx+cy)
}
