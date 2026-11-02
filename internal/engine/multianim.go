package engine

import (
	"fmt"

	"github.com/openorion2/openorion2/internal/gfx"
)

// Multi-file animations (file_ani.c), the saved palette, the cycled palette
// colour, mouse images and boxes: what the science room and the technology
// screens use. Spec: docs/screens/tech-select.md.

// multiSlots is the number of file animations Open_Multi_File_Animation_
// keeps open ("Max 3 file animations").
const multiSlots = 3

// OpenMultiFileAnimation is Open_Multi_File_Animation_ (0x12d1c5): p, read
// from its LBX entry, becomes animation slot. Slot 0 shares its header with
// Open_File_Animation_'s (io_buffer), so the single-file routines see it.
func (e *Engine) OpenMultiFileAnimation(slot int, p *gfx.Picture) error {
	if slot >= multiSlots {
		return fmt.Errorf("Max 3 file animations")
	}
	if slot == 0 {
		e.fileAnim = p
	}
	e.multi[slot] = p
	return nil
}

// multiAnim is the animation in slot, slot 0 being the single file's.
func (e *Engine) multiAnim(slot int) *gfx.Picture {
	if slot == 0 {
		return e.fileAnim
	}
	return e.multi[slot]
}

// DrawMultiFileAnimation is Draw_Multi_File_Animation_ (0x12d408): the
// slot's current frame at (0, 0), unclipped, then the frame advances as in
// Draw.
func (e *Engine) DrawMultiFileAnimation(slot int) error {
	v, p := e.V, e.multiAnim(slot)
	if p.Mask&gfx.MaskPalette != 0 {
		e.setAnimationPalette(p)
	}
	if v.refresh > RefreshFull+1 {
		v.AddSquare(0, 0, p.Width-1, p.Height-1)
	}
	if p.Mask&gfx.MaskGlass != 0 {
		return fmt.Errorf("glassed multi-file animations are not ported")
	}
	if err := p.DrawAt(v.Current, W, 0, 0, p.CurrentFrame); err != nil {
		return err
	}
	if p.Kind() == gfx.KindBackground && v.refresh > RefreshFull+1 {
		v.AddSquare(0, 0, W-1, H-1)
	}
	advanceFrame(p)
	return nil
}

// DrawMultiFileAnimationStencil is Draw_Multi_File_Animation_Stencil_
// (0x12cad6): the slot's frame, a sprite, into the current and the back
// page (Draw_Animated_Sprite_Back_Diff_), its box marked first.
func (e *Engine) DrawMultiFileAnimationStencil(slot int) error {
	v, p := e.V, e.multiAnim(slot)
	if p.Mask&gfx.MaskPalette != 0 {
		e.setAnimationPalette(p)
	}
	if v.refresh > RefreshFull+1 {
		v.AddSquare(0, 0, p.Width-1, p.Height-1)
	}
	if p.Kind() != gfx.KindSprite {
		return fmt.Errorf("Differential File Animation Can not be a bitmap")
	}
	if err := e.drawBackDiff(p); err != nil {
		return err
	}
	stencilAdvance(p)
	return nil
}

// drawBackDiff is Draw_Animated_Sprite_Back_Diff_ at (0, 0): every run goes
// into both pages and marks x..x+count of its row.
func (e *Engine) drawBackDiff(p *gfx.Picture) error {
	v := e.V
	var lo, hi [H]int
	for i := range lo {
		lo[i], hi[i] = -1, -1
	}
	if err := p.Render(p.CurrentFrame, func(row, col int, c byte) {
		if o := row*W + col; o >= 0 && o < Size {
			v.Back[o] = c
			v.Current[o] = c
		}
		if row >= 0 && row < H {
			if lo[row] == -1 || col < lo[row] {
				lo[row] = col
			}
			hi[row] = max(hi[row], col+1)
		}
	}); err != nil {
		return err
	}
	for y := range lo {
		if lo[y] != -1 {
			v.AddSquare(lo[y], y, hi[y], y)
		}
	}
	return nil
}

// stencilAdvance is the frame step of the stencil file animations: they
// stop on the last frame unless they loop.
func stencilAdvance(p *gfx.Picture) {
	p.CurrentFrame++
	if p.CurrentFrame >= p.Frames {
		switch {
		case p.Mask&gfx.MaskHold == 0:
			p.CurrentFrame = p.Frames - 1
		case p.RepeatFrame+1 == p.Frames:
			p.CurrentFrame = 0
		default:
			p.CurrentFrame = p.RepeatFrame
		}
	}
}

// DrawMultiFileAnimationPalette is Draw_Multi_File_Animation_Palette_
// (0x12d7e3): only the slot's palette chunk.
func (e *Engine) DrawMultiFileAnimationPalette(slot int) {
	if p := e.multiAnim(slot); p.Mask&gfx.MaskPalette != 0 {
		e.setAnimationPalette(p)
	}
}

// SavePalette is Save_Palette_ (0x131970): current_palette, flags included.
func (e *Engine) SavePalette() { e.savedPalette = e.V.Palette }

// RestorePalette is Restore_Palette_ (0x1319a3), every entry marked changed.
func (e *Engine) RestorePalette() {
	e.V.Palette = e.savedPalette
	e.V.SetPaletteChanges(0, 0xff)
}

// RefreshPalette is Refresh_Palette_ (0x131f7b): the changed entries reach
// the DAC and every flag is cleared.
func (e *Engine) RefreshPalette() {
	e.V.refreshPalette()
	e.V.ClearPaletteChanges(0, 0xff)
}

// SetCycleDirection sets cycle_direction_flag (0x1845ec), as .data or a
// previous screen left it.
func (e *Engine) SetCycleDirection(d int32) { e.cycle.direction = d }

// ResetCyclePaletteColor is Reset_Cycle_Palette_Color_ (0x131b34): the next
// cycle starts from its minimum.
func (e *Engine) ResetCyclePaletteColor() { e.cycle.direction = -1 }

// cycleState is the palette.c cycling state: cycle_direction_flag
// (0x1845ec), cycle_color_value (0x1bc790), cycle_step_value (0x1bc794).
type cycleState struct {
	direction, value, step int32
}

// CyclePaletteColor is Cycle_Palette_Color_ (0x131b5d): colour c moves one
// step between the two RGB ends, led by the component with the widest
// range, bouncing at the ends. It goes straight to the DAC and into
// current_palette without marking it changed.
func (e *Engine) CyclePaletteColor(c int, rMin, gMin, bMin, rMax, gMax, bMax, step int16) {
	cy := &e.cycle
	abs := func(v int32) int32 {
		if v < 0 {
			return -v
		}
		return v
	}
	dr := abs(int32(rMax) - int32(rMin))
	dg := abs(int32(gMax) - int32(gMin))
	db := abs(int32(bMax) - int32(bMin))
	cy.step = int32(step)
	primary := 2
	if dr < dg || dr < db {
		if dr <= dg && db <= dg {
			primary = 1
		}
	} else {
		primary = 0
	}
	if cy.direction == -1 {
		cy.direction = 0
		cy.value = int32([3]int16{rMin, gMin, bMin}[primary])
	}
	lerp := func(lo, hi, plo, phi int16) int32 {
		return (cy.value-int32(plo))*(int32(hi)-int32(lo))/(int32(phi)-int32(plo)) + int32(lo)
	}
	var r, g, b int32
	switch primary {
	case 0:
		r, g, b = cy.value, lerp(gMin, gMax, rMin, rMax), lerp(bMin, bMax, rMin, rMax)
		e.updateCycle(rMin, rMax)
	case 1:
		r, g, b = lerp(rMin, rMax, gMin, gMax), cy.value, lerp(bMin, bMax, gMin, gMax)
		e.updateCycle(gMin, gMax)
	default:
		r, g, b = lerp(rMin, rMax, bMin, bMax), lerp(gMin, gMax, bMin, bMax), cy.value
		e.updateCycle(bMin, bMax)
	}
	v := e.V
	v.DAC[c] = [3]byte{byte(r), byte(g), byte(b)}
	v.Palette[c].R, v.Palette[c].G, v.Palette[c].B = byte(r), byte(g), byte(b)
}

// updateCycle is Update_Cycle_ (0x131e6b): one step toward the end the
// direction points at, turning one short of it.
func (e *Engine) updateCycle(lo, hi int16) {
	cy := &e.cycle
	up := hi > lo
	if cy.direction != 0 {
		up = !up
	}
	if up {
		cy.value += cy.step
		if int32(hi) < cy.value {
			cy.value = int32(hi) - 1
			cy.direction = turn(cy.direction)
		}
		return
	}
	cy.value -= cy.step
	if int32(lo) > cy.value {
		cy.value = int32(lo) + 1
		cy.direction = turn(cy.direction)
	}
}

func turn(d int32) int32 {
	if d == 0 {
		return 1
	}
	return 0
}

// mouseImage returns mouse image n, 24x24 bytes, writable.
func (e *Engine) mouseImage(n int) []byte {
	img := e.f.mouseImages
	if len(img) < (n+1)*0x240 {
		return nil
	}
	return img[n*0x240 : (n+1)*0x240]
}

// ClearMouseImage is Clear_Mouse_Image_ (0x12e332).
func (e *Engine) ClearMouseImage(n int) {
	clear(e.mouseImage(n))
}

// DrawBitmapToMouse is Draw_Bitmap_To_Mouse_ (0x12e1e0): the bitmap's
// non-zero pixels into mouse image n at (x, y), cut to 24x24.
func (e *Engine) DrawBitmapToMouse(x, y int, bm *gfx.Picture, n int) {
	img := e.mouseImage(n)
	pix := bm.Pixels()
	if img == nil || pix == nil {
		return
	}
	w, h := int(int16(bm.Width)), int(int16(bm.Height))
	skip := 0
	if x+w >= 0x19 {
		skip = w + x - 0x18
		w = 0x18 - x
	}
	if y+h > 0x18 {
		h = 0x18 - y
	}
	pos, sp := 0, y*0x18+x
	for r := 0; r < h; r++ {
		for c := 0; c < w; c++ {
			if pos < len(pix) && pix[pos] != 0 && sp >= 0 && sp < len(img) {
				img[sp] = pix[pos]
			}
			pos++
			sp++
		}
		pos += skip
		sp += 0x18 - w
	}
}

// Box is Box (0x1297c3): a frame thickness pixels wide inside x, y, w, h.
func (e *Engine) Box(x, y, w, h, thickness int, c byte) {
	v := e.V
	for i := 0; i < thickness; i++ {
		x1, y1 := int(int16(x+i)), int(int16(y+i))
		x2, y2 := int(int16(x+w-1-i)), int(int16(y+h-1-i))
		v.Line(x1, y1, x2, y1, c)
		v.Line(x1, y1, x1, y2, c)
		v.Line(x2, y1, x2, y2, c)
		v.Line(x1, y2, x2, y2, c)
	}
}
