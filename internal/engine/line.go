package engine

// Line primitives: Line, Clip_Line, Multi_Colored_Line, Add_Dot_To_Scan_List
// and the game's Draw_Directional_Multi_Colored_Line_.
//
// Spec: docs/engine/lines.md.

// Line is Line_ (0x134d2d): a run-sliced line in colour c on the current
// page, clipped to the window when clipping is on.
func (v *Video) Line(x1, y1, x2, y2 int, c byte) {
	v.line(x1, y1, x2, y2, func(o int) { v.put(o, c) })
}

// MultiColoredLine is Multi_Colored_Line_ (0x135870): Line where pixel k,
// counted from the top end after clipping, gets colors[(rot+k)&7].
func (v *Video) MultiColoredLine(x1, y1, x2, y2 int, colors [8]byte, rot int) {
	v.line(x1, y1, x2, y2, func(o int) {
		v.put(o, colors[rot&7])
		rot++
	})
}

// DirectionalMultiColoredLine is Draw_Directional_Multi_Colored_Line_
// (0xa11c0): picks the pattern phase or reverses it so it flows from end 1.
func (v *Video) DirectionalMultiColoredLine(x1, y1, x2, y2 int, colors [8]byte, start int) {
	ax, ay, bx, by := int16(x1), int16(y1), int16(x2), int16(y2)
	if (bx > ax && by >= ay) || (bx <= ax && by > ay) {
		rev := 7 - int(int16(start))
		if rev <= 0 {
			rev = 0
		}
		v.MultiColoredLine(x1, y1, x2, y2, colors, int(int16(rev)))
		return
	}
	var b [8]byte
	for i := range 8 {
		b[7-i] = colors[i]
	}
	v.MultiColoredLine(x1, y1, x2, y2, b, int(int16(start)))
}

// put writes one pixel at a linear offset; the original has no bounds check,
// so offsets off the page are dropped rather than corrupting memory.
func (v *Video) put(o int, c byte) {
	if o >= 0 && o < Size {
		v.Current[o] = c
	}
}

// line is the body shared by Line_ and Multi_Colored_Line_; plot is called
// for every pixel in drawing order. Coordinates are int16 as in the binary.
func (v *Video) line(ix1, iy1, ix2, iy2 int, plot func(o int)) {
	x1, y1, x2, y2 := int16(ix1), int16(iy1), int16(ix2), int16(iy2)
	if v.Clipping && v.clipLine(&x1, &y1, &x2, &y2) {
		return
	}
	if y1 > y2 {
		x1, x2 = x2, x1
		y1, y2 = y2, y1
	}
	o := int(y1)*W + int(x1)
	dx := x2 - x1
	adv := int16(1)
	if dx < 0 {
		adv, dx = -1, -dx
	}
	dy := y2 - y1
	mark := v.refresh >= RefreshDifferential

	switch {
	case dx == 0:
		for i := 0; i <= int(dy); i++ {
			plot(o)
			o += W
		}
		if mark {
			for y := int(y1); y <= int(y2); y++ {
				v.AddDot(int(x1), y)
			}
		}
	case dy == 0:
		for i := 0; i <= int(dx); i++ {
			plot(o)
			o += int(adv)
		}
		if mark {
			if x2 < x1 {
				x1, x2 = x2, x1
			}
			v.AddSquare(int(x1), int(y1), int(x2), int(y1))
		}
	case dx == dy:
		for i := 0; i <= int(dx); i++ {
			plot(o)
			o += int(adv) + W
		}
		if mark {
			for y := int(y1); y <= int(y2); y++ {
				if x1 < x2 {
					v.AddDot(int(x1)+(y-int(y1)), y)
				} else {
					v.AddDot(int(x1)-(y-int(y1)), y)
				}
			}
		}
	default:
		xMajor := dx > dy
		major, minor := dx, dy
		if !xMajor {
			major, minor = dy, dx
		}
		whole := int(major) / int(minor)
		rem := int(major) % int(minor)
		adjUp, adjDown := rem*2, int(minor)*2
		errTerm := rem - int(minor)*2
		initial := int(int16(whole))>>1 + 1
		final := initial
		if int16(adjUp) == 0 && whole&1 == 0 {
			initial--
		}
		if whole&1 != 0 {
			errTerm += int(minor)
		}
		mx, my := int(x1), int(y1)
		run := func(n int) {
			n = int(int16(n))
			if xMajor {
				for i := 0; i < n; i++ {
					plot(o)
					o += int(adv)
				}
				o += W
				if mark {
					// Add_Square covers n+1 pixels: one past the run.
					if adv > 0 {
						v.AddSquare(mx, my, mx+n, my)
						mx += n
					} else {
						v.AddSquare(mx-n, my, mx, my)
						mx -= n
					}
					my++
				}
				return
			}
			for i := 0; i < n; i++ {
				plot(o)
				o += W
			}
			o += int(adv)
			if mark {
				for i := 0; i < n; i++ {
					v.AddDot(mx, my+i)
				}
				my += n
				mx += int(adv)
			}
		}
		run(initial)
		for i := 0; i < int(minor)-1; i++ {
			n := whole
			errTerm += adjUp
			if int16(errTerm) > 0 {
				n++
				errTerm -= adjDown
			}
			run(n)
		}
		run(final)
	}
}

// clipLine is Clip_Line_ (0x135357): clip to the inclusive window, first in
// x, then (always, even after an x rejection) in y. True means rejected.
func (v *Video) clipLine(x1, y1, x2, y2 *int16) bool {
	wx1, wy1, wx2, wy2 := int16(v.WinX1), int16(v.WinY1), int16(v.WinX2), int16(v.WinY2)
	out := false
	if *x1 == *x2 {
		if *x1 < wx1 || *x2 > wx2 {
			out = true
		} else {
			if *y2 < *y1 {
				*y1, *y2 = *y2, *y1
			}
			if *y2 < wy1 || *y1 > wy2 {
				out = true
			} else {
				*y1 = max(*y1, wy1)
				*y2 = min(*y2, wy2)
			}
		}
	} else {
		if *x2 < *x1 {
			*x1, *x2 = *x2, *x1
			*y1, *y2 = *y2, *y1
		}
		ddy, ddx := int32(*y2)-int32(*y1), int32(*x2)-int32(*x1)
		if *x1 < wx1 {
			*y1 = int16((int32(wx1)-int32(*x1))*ddy/ddx + int32(*y1))
			*x1 = wx1
		}
		if *x1 > *x2 {
			out = true
		} else {
			if *x2 > wx2 {
				*y2 = int16((int32(wx2)-int32(*x1))*ddy/ddx + int32(*y1))
				*x2 = wx2
			}
			if *x2 < *x1 {
				out = true
			}
		}
	}
	if *y1 == *y2 {
		if *y1 < wy1 || *y2 > wy2 {
			out = true
		} else {
			if *x2 < *x1 {
				*x1, *x2 = *x2, *x1
			}
			if *x2 < wx1 || *x1 > wx2 {
				out = true
			} else {
				*x1 = max(*x1, wx1)
				*x2 = min(*x2, wx2)
			}
		}
		return out
	}
	if *y2 < *y1 {
		*x1, *x2 = *x2, *x1
		*y1, *y2 = *y2, *y1
	}
	ddx, ddy := int32(*x2)-int32(*x1), int32(*y2)-int32(*y1)
	if *y1 < wy1 {
		*x1 = int16((int32(wy1)-int32(*y1))*ddx/ddy + int32(*x1))
		*y1 = wy1
	}
	if *y1 > *y2 {
		return true
	}
	if *y2 > wy2 {
		*x2 = int16((int32(wy2)-int32(*y1))*ddx/ddy + int32(*x1))
		*y2 = wy2
	}
	if *y2 < *y1 {
		out = true
	}
	return out
}

// AddDot is Add_Dot_To_Scan_List_ (0x125e13): mark dword column x>>2
// (arithmetic shift) of row y in the scan list, and in the stencil list
// unless the mode is differential. No clamping; rows off the list, which
// the original would write past, are dropped.
func (v *Video) AddDot(x, y int) {
	if y < 0 || y >= H {
		return
	}
	c := int16(x >> 2)
	dot(&v.scan[y], c)
	if v.refresh != RefreshDifferential {
		dot(&v.stencil[y], c)
	}
}

// dot merges one column: an empty row takes it, otherwise it lowers start
// or else raises end (never both).
func dot(s *span, c int16) {
	switch {
	case s.start == -1:
		*s = span{c, c}
	case s.start > c:
		s.start = c
	case s.end < c:
		s.end = c
	}
}

// Dot is Dot (0x12972d): one pixel, dropped outside the window when
// clipping is on.
func (v *Video) Dot(x, y int, c byte) {
	if v.Clipping && (x < v.WinX1 || y < v.WinY1 || x > v.WinX2 || y > v.WinY2) {
		return
	}
	if v.refresh >= RefreshDifferential {
		v.AddDot(x, y)
	}
	if o := y*W + x; o >= 0 && o < Size {
		v.Current[o] = c
	}
}

// PlasmaLine is Plasma_Line (0x136a2f): Line where pixel k, counted from the
// top end after clipping, gets colors[(rot+k)&0x3ff].
func (v *Video) PlasmaLine(x1, y1, x2, y2 int, colors []byte, rot int) {
	v.line(x1, y1, x2, y2, func(o int) {
		v.put(o, colors[int(int16(rot))&0x3ff])
		rot++
	})
}
