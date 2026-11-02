package engine

import (
	"encoding/binary"
	"fmt"

	"github.com/openorion2/openorion2/internal/gfx"
)

// Planet palettes and remapped drawing: reduced palette groups (palette\reduce.c),
// picture remap colours (palette\remap.c) and the game's masked remap blitter
// (MOX). Spec: docs/engine/remap.md.

// remapState holds the statics of reduce.c and remap.c and the game's gray
// tables. It lives beside the engine until engine.go can carry the field.
type remapState struct {
	picture [256]byte     // picture_remap_color_list (0x1bc79c)
	gray    [10][256]byte // _remap_gray_mask_palette (0x19c814), level m-1 for mask byte m

	reduced      [512][4]byte // reduced_palette_list (0x1be354): flag, R, G, B
	reducedCount int          // reduced_palette_count (0x1beb56)
	r            reducer
}

func (e *Engine) remap() *remapState { return &e.remapS }

// SortPaletteList is Sort_Palette_List (0x13372a); FindClosestColor needs it
// to have run on the current palette.
func (e *Engine) SortPaletteList() { e.sortPaletteList() }

// FindClosestColor is Find_Closest_Color (0x1338c9) over the last sorted palette.
func (e *Engine) FindClosestColor(r, g, b int) byte {
	return e.findClosestColor(r&0xff, g&0xff, b&0xff)
}

// PictureRemap is picture_remap_color_list as Calculate_Picture_Remap_Colors left it.
func (e *Engine) PictureRemap() *[256]byte { return &e.remap().picture }

// GrayRemap is _remap_gray_mask_palette: table l is used for mask byte l+1.
func (e *Engine) GrayRemap() *[10][256]byte { return &e.remap().gray }

// ClearReducedPaletteGroups is Clear_Reduced_Palette_Groups (0x13aac1). Only
// the count is reset; the colours stay and Compress can read them back.
func (e *Engine) ClearReducedPaletteGroups() { e.remap().reducedCount = 0 }

// AddReducedPaletteGroup is Add_Reduced_Palette_Group (0x13aae9): append the
// RGB of every colour in p's palette chunk (flag bytes are not written).
func (e *Engine) AddReducedPaletteGroup(p *gfx.Picture) error {
	if p.Mask&gfx.MaskPalette == 0 || p.Palette == nil {
		return fmt.Errorf("Add_Reduced_Palette_Group: picture has no palette")
	}
	s := e.remap()
	raw := p.Palette.Raw
	for i := 0; i < len(raw)/4; i++ {
		c := &s.reduced[s.reducedCount]
		c[1], c[2], c[3] = raw[4*i+1], raw[4*i+2], raw[4*i+3]
		s.reducedCount++
		if s.reducedCount >= 0x200 {
			return fmt.Errorf("Add_Reduced_Palette_Group: too many colours")
		}
	}
	return nil
}

// CompressReducedPaletteGroups is Compress_Reduced_Palette_Groups (0x13ac01):
// median-cut the collected colours to count when there are more, then copy
// entries 0..count-1 into current_palette[start..] with the changed flag set.
// With fewer colours, stale entries past the count are copied too.
func (e *Engine) CompressReducedPaletteGroups(start, count int) error {
	s := e.remap()
	if s.reducedCount > count {
		if err := s.r.reducePalette(s.reduced[:], 0, s.reducedCount, count); err != nil {
			return err
		}
	}
	for i := 0; i < count; i++ {
		c := s.reduced[i]
		e.V.Palette[start+i] = Color{1, c[1], c[2], c[3]}
	}
	return nil
}

// CalculatePictureRemapColors is Calculate_Picture_Remap_Colors (0x133c9c) with
// Create_Picture_Remap_Palette (0x133d16): identity, except that each colour
// of p's palette chunk maps to the closest colour of the current palette.
func (e *Engine) CalculatePictureRemapColors(p *gfx.Picture) error {
	if p.Mask&gfx.MaskPalette == 0 || p.Palette == nil {
		return fmt.Errorf("Calculate_Picture_Remap_Colors: picture has no palette")
	}
	s := e.remap()
	e.sortPaletteList()
	for i := range s.picture {
		s.picture[i] = byte(i)
	}
	ch := p.Palette
	for i := 0; i < len(ch.Raw)/4; i++ {
		r := ch.Raw[4*i : 4*i+4]
		s.picture[(ch.Start+i)&0xff] = e.findClosestColor(int(r[1]), int(r[2]), int(r[3]))
	}
	return nil
}

// RemapGrayPercentages is _remap_gray_percentages (0x1823e0).
var RemapGrayPercentages = [10]int{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}

// CalculateRemapGrayPalettes is Calculate_Remap_Gray_Palettes_ (0xa5050): table
// l holds each colour darkened by percent[l] and matched back to the palette.
func (e *Engine) CalculateRemapGrayPalettes(percent [10]int) {
	s := e.remap()
	e.sortPaletteList()
	pal := &e.V.Palette
	for l := 0; l < 10; l++ {
		f := 100 - int(byte(percent[l]))
		for c := 0; c < 256; c++ {
			r := byte(int(pal[c].R) * f / 100)
			g := byte(int(pal[c].G) * f / 100)
			b := byte(int(pal[c].B) * f / 100)
			s.gray[l][c] = e.findClosestColor(int(r), int(g), int(b))
		}
	}
}

// RemapDrawWithMask is Remap_Draw_With_Mask_ (0xa512e): draw pict's current
// sprite frame through the picture remap table, darkened by mask's current
// frame, and advance both. header is animation_header on entry (the picture
// the engine touched last; nil means pict): the clip test and the clipped
// path's scan rectangle use its size, not pict's. A clipped draw does not
// advance the frames.
func (e *Engine) RemapDrawWithMask(x, y int, pict, mask, header *gfx.Picture) error {
	v := e.V
	if header == nil {
		header = pict
	}
	clipped := false
	if v.Clipping {
		x2, y2 := x+header.Width-1, y+header.Height-1
		if x > v.WinX2 || y > v.WinY2 || x2 < v.WinX1 || y2 < v.WinY1 {
			return nil
		}
		clipped = x < v.WinX1 || y < v.WinY1 || x2 > v.WinX2 || y2 > v.WinY2
	}
	f, err := pict.Frame(pict.CurrentFrame)
	if err != nil {
		return err
	}
	m, err := mask.Frame(mask.CurrentFrame)
	if err != nil {
		return err
	}
	s := e.remap()
	if !clipped {
		if v.refresh >= RefreshDifferential {
			v.AddSquare(x, y, x+pict.Width-1, y+pict.Height-1)
			v.AddSquare(x+100, y+100, x+100+pict.Width-1, y+100+pict.Height-1)
		}
		if pict.Kind() != gfx.KindSprite {
			return fmt.Errorf("Remap draw not supported for kind %d", pict.Kind())
		}
		if err := remapSpriteMask(v.Current, x, y, pict, f, m, s); err != nil {
			return err
		}
		advanceFrame(pict)
		advanceFrame(mask)
		return nil
	}
	// dot_stencil_flag is not modelled (always 0): the stencil branch would
	// skip drawing and still advance the frames.
	if v.refresh >= RefreshDifferential {
		v.AddSquare(x, y, x+header.Width-1, y+header.Height-1)
	}
	if pict.Kind() != gfx.KindSprite {
		return fmt.Errorf("Remap clipped draw not supported for kind %d", pict.Kind())
	}
	w := gfx.Window{X1: v.WinX1, Y1: v.WinY1, X2: v.WinX2, Y2: v.WinY2}
	return remapClippedSpriteMask(v.Current, x, y, pict, f, m, s, w)
}

// maskedColor is one pixel: remap, then the gray level of mask byte mv.
func (s *remapState) maskedColor(src, mv byte) byte {
	c := s.picture[src]
	if mv != 0 && mv <= 10 {
		c = s.gray[mv-1][c]
	}
	return c // bytes above 10 read past the tables in the original
}

// spriteReader reads a frame and whatever follows it, as the blitters do.
type spriteReader struct{ b []byte }

func (r spriteReader) s16(pos int) (int, error) {
	if pos < 0 || pos+2 > len(r.b) {
		return 0, fmt.Errorf("sprite frame runs out at byte %d", pos)
	}
	return int(int16(binary.LittleEndian.Uint16(r.b[pos:]))), nil
}

func (r spriteReader) byteAt(pos int) byte {
	if pos >= 0 && pos < len(r.b) {
		return r.b[pos]
	}
	return 0
}

// remapSpriteMask is Remap_Draw_Animated_Sprite_Mask_ (0xa53dc). The mask is
// read at row*width + skip + k: skip is the run's own skip word, not the x
// reached on the row, and run data restarts word-aligned to the frame.
func remapSpriteMask(page []byte, x, y int, p *gfx.Picture, frame, mask []byte, s *remapState) error {
	f, m := spriteReader{frame[:cap(frame)]}, spriteReader{mask[:cap(mask)]}
	top, err := f.s16(2)
	if err != nil {
		return err
	}
	row := int(uint16(top))
	line := (y+row)*W + x
	dst := line
	pos := 4
	for guard := 0; int(int16(p.Height)) > row; guard++ {
		if guard > 1<<20 {
			return fmt.Errorf("sprite frame does not end")
		}
		count, err := f.s16(pos)
		if err != nil {
			return err
		}
		skip, err := f.s16(pos + 2)
		if err != nil {
			return err
		}
		pos += 4
		if count == 0 {
			row += skip
			line += skip * W
			dst = line
			continue
		}
		dst += skip
		for k := 0; k < count; k++ {
			c := s.maskedColor(f.byteAt(pos+k), m.byteAt(p.Width*row+skip+k))
			if o := dst + k; o >= 0 && o < len(page) {
				page[o] = c
			}
		}
		if count > 0 {
			dst += count
		}
		pos += 2 * ((count + 1) / 2)
	}
	return nil
}

// remapClippedSpriteMask is Remap_Clipped_Draw_Animated_Sprite_Mask_ (0xa54f2).
// Rows count from the clipped top, so the mask row is off by the rows cut
// away above the window.
func remapClippedSpriteMask(page []byte, x, y int, p *gfx.Picture, frame, mask []byte, s *remapState, w gfx.Window) error {
	f, m := spriteReader{frame[:cap(frame)]}, spriteReader{mask[:cap(mask)]}
	cx1, skipX := x, 0
	if x < w.X1 {
		cx1, skipX = w.X1, w.X1-x
	}
	cy1, skipY := y, 0
	if y < w.Y1 {
		cy1, skipY = w.Y1, w.Y1-y
	}
	width := w.X2 - cx1 + 1
	if x+p.Width-1 < w.X2 {
		width = x + p.Width - 1 - cx1 + 1
	}
	y2 := y + p.Height - 1
	if y2 >= w.Y2 {
		y2 = w.Y2
	}
	height := y2 - cy1 + 1
	line := x + cy1*W
	top, err := f.s16(2)
	if err != nil {
		return err
	}
	row := int(uint16(top))
	pos := 4
	for row < skipY {
		count, err := f.s16(pos)
		if err != nil {
			return err
		}
		if count == 0 {
			sk, err := f.s16(pos + 2)
			if err != nil {
				return err
			}
			row += int(uint16(sk))
		} else {
			pos += 2 * ((count + 1) / 2)
		}
		pos += 4
	}
	if row > skipY {
		line += (row - skipY) * W
	}
	row -= skipY
	dst := line
	left, right := line+skipX, line+skipX+width
	for guard := 0; row < height; guard++ {
		if guard > 1<<20 {
			return fmt.Errorf("sprite frame does not end")
		}
		count, err := f.s16(pos)
		if err != nil {
			return err
		}
		skip, err := f.s16(pos + 2)
		if err != nil {
			return err
		}
		pos += 4
		if count == 0 {
			row += skip
			line += skip * W
			dst = line
			left, right = line+skipX, line+skipX+width
			continue
		}
		dst += skip
		src, k := pos, 0
		runEnd := dst + count
		if dst < right {
			if dst < left {
				n := int(int16(left - dst))
				if n > count {
					n = count
				}
				src, k, dst = src+n, k+n, dst+n
			}
			e := int(int16(right - dst))
			if dst+e >= runEnd {
				e = int(int16(runEnd - dst))
			}
			for end := dst + e; dst < end; dst, src, k = dst+1, src+1, k+1 {
				c := s.maskedColor(f.byteAt(src), m.byteAt(p.Width*row+skip+k))
				if dst >= 0 && dst < len(page) {
					page[dst] = c
				}
			}
		}
		pos += 2 * ((count + 1) / 2)
	}
	return nil
}

// cube is s_cube of reduce.c: a range of hist and its colour bounds.
type cube struct {
	lower, upper, level    int
	rmin, rmax, gmin, gmax byte
	bmin, bmax             byte
}

// reducer holds reduce.c's working statics: color_weight, cube_list,
// HistPtr and ncubes.
type reducer struct {
	weight [512]byte
	cubes  [256]cube
	hist   [512]int
	ncubes int
}

// reducePalette is Reduce_Palette (0x139af3): median cut of list[0..total) to
// count colours written to list[start..], each the centre of its cube's box.
func (r *reducer) reducePalette(list [][4]byte, start, total, count int) error {
	if total > 0x200 {
		return fmt.Errorf("Too many colors to reduce from")
	}
	if count > 0x100 {
		return fmt.Errorf("Too many colors to reduce to")
	}
	r.initialCube(list, total)
	for r.ncubes < count {
		c := r.findNextLowestCubeLevel()
		if c == -1 {
			break
		}
		axis := r.findLongestAxis(c)
		cb := &r.cubes[c]
		sortCubePoints(list, r.hist[cb.lower:], int(int16(cb.upper-cb.lower+1)), axis)
		r.splitCube(list, c, axis)
	}
	i := 0
	for ; i < r.ncubes; i++ {
		cb := &r.cubes[i]
		list[start+i] = [4]byte{1,
			byte((int(cb.rmin) + int(cb.rmax)) / 2),
			byte((int(cb.gmin) + int(cb.gmax)) / 2),
			byte((int(cb.bmin) + int(cb.bmax)) / 2)}
	}
	for ; i < count; i++ {
		p := list[start+i-1]
		list[start+i] = [4]byte{1, p[1], p[2], p[3]}
	}
	return nil
}

// initialCube is Initial_Cube (0x13a7cc).
func (r *reducer) initialCube(list [][4]byte, total int) {
	for i := 0; i < total; i++ {
		r.hist[i] = i
	}
	r.cubes[0] = cube{lower: 0, upper: total - 1}
	r.shrinkCube(&r.cubes[0], list)
	r.ncubes = 1
}

// shrinkCube is Shrink_Cube (0x13a993): the colour bounds of the cube's points.
func (r *reducer) shrinkCube(c *cube, list [][4]byte) {
	c.rmin, c.rmax, c.gmin, c.gmax, c.bmin, c.bmax = 0xff, 0, 0xff, 0, 0xff, 0
	for i := c.lower; i <= c.upper; i++ {
		p := list[r.hist[i&0x1ff]]
		c.rmin, c.rmax = min(c.rmin, p[1]), max(c.rmax, p[1])
		c.gmin, c.gmax = min(c.gmin, p[2]), max(c.gmax, p[2])
		c.bmin, c.bmax = min(c.bmin, p[3]), max(c.bmax, p[3])
	}
}

// findNextLowestCubeLevel is Find_Next_Lowest_Cube_Level (0x13a84f): the
// first cube with the longest side, tested red, green, blue; -1 if none is
// longer than 0.
func (r *reducer) findNextLowestCubeLevel() int {
	next, largest := -1, 0
	for i := 0; i < r.ncubes; i++ {
		c := &r.cubes[i]
		if c.lower == c.upper || c.level >= 30000 {
			continue
		}
		for _, l := range [3]byte{c.rmax - c.rmin, c.gmax - c.gmin, c.bmax - c.bmin} {
			if int(l) > largest {
				largest, next = int(l), i
			}
		}
	}
	return next
}

// findLongestAxis is Find_Longest_Axis (0x13a711): 0 red, 1 green, 2 blue;
// ties go to red, then green.
func (r *reducer) findLongestAxis(i int) int {
	c := &r.cubes[i]
	lr, lg, lb := c.rmax-c.rmin, c.gmax-c.gmin, c.bmax-c.bmin
	if lr >= lg && lr >= lb {
		return 0
	}
	if lg >= lr && lg >= lb {
		return 1
	}
	return 2
}

// sortCubePoints is Sort_Cube_Points (0x13a4a9): a bubble sort of the point
// indices, ascending on one channel, at most count passes.
func sortCubePoints(list [][4]byte, points []int, count, axis int) {
	ch := axis + 1
	if axis < 0 || axis > 2 {
		return
	}
	changed := true
	for i := 0; i < count && changed; i++ {
		changed = false
		for j := 0; j < count-1; j++ {
			if list[points[j]][ch] > list[points[j+1]][ch] {
				points[j], points[j+1] = points[j+1], points[j]
				changed = true
			}
		}
	}
}

// splitCube is Split_Cube (0x139d7e): weight each sorted point by its larger
// gap to a neighbour (at least 1) and cut where half the weight is reached.
func (r *reducer) splitCube(list [][4]byte, i, axis int) {
	c := &r.cubes[i]
	ch := axis + 1
	if axis >= 0 && axis <= 2 {
		val := func(k int) byte { return list[r.hist[k&0x1ff]][ch] }
		for k := c.lower; k <= c.upper; k++ {
			var w byte
			switch {
			case k == c.lower:
				w = val(k+1) - val(k)
			case k == c.upper:
				w = val(k) - val(k-1)
			default:
				a, b := val(k)-val(k-1), val(k+1)-val(k)
				w = max(a, b)
			}
			if w == 0 {
				w = 1
			}
			r.weight[k&0x1ff] = w
		}
	}
	total := 0
	for k := c.lower; k <= c.upper; k++ {
		total += int(r.weight[k&0x1ff])
	}
	total /= 2
	slice := c.lower
	total -= int(r.weight[slice&0x1ff])
	for total > 0 && slice < c.upper {
		slice++
		total -= int(r.weight[slice&0x1ff])
	}
	n := &r.cubes[r.ncubes&0xff]
	n.lower = int(int16(slice + 1))
	n.upper = c.upper
	n.level = int(int16(c.level + 1))
	c.upper = slice
	c.level = int(int16(c.level + 1))
	r.shrinkCube(c, list)
	r.shrinkCube(n, list)
	r.ncubes++
}
