package engine

import (
	"fmt"

	"github.com/openorion2/openorion2/internal/gfx"
)

// DrawPalette is Draw_Palette: apply a picture's palette chunk without
// drawing it.
func (e *Engine) DrawPalette(p *gfx.Picture) {
	if p.Mask&gfx.MaskPalette != 0 {
		e.setAnimationPalette(p)
	}
}

// DrawFileAnimation is Draw_File_Animation (file_ani.c): draw the current
// frame of an animation streamed from its LBX entry at (0, 0) and advance.
// The original reads each frame from the file on demand; the port keeps the
// entry in memory, which gives the same pixels.
func (e *Engine) DrawFileAnimation(p *gfx.Picture) error {
	v := e.V
	if p.Mask&gfx.MaskPalette != 0 {
		e.setAnimationPalette(p)
	}
	if v.refresh > RefreshFull+1 {
		v.AddSquare(0, 0, p.Width-1, p.Height-1)
	}
	if p.Mask&gfx.MaskGlass != 0 {
		return fmt.Errorf("glassed file animations are not ported")
	}
	if err := p.DrawAt(v.Current, W, 0, 0, p.CurrentFrame); err != nil {
		return err
	}
	if p.Kind() == gfx.KindBackground && v.refresh > RefreshFull+1 {
		v.AddSquare(0, 0, W-1, H-1)
	}
	// Unlike Draw, a file animation stops on its last frame.
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
	return nil
}

// OpenFileAnimation is Open_File_Animation_ (0x12c607): p, freshly read from
// its LBX entry, becomes the animation the other file routines draw.
func (e *Engine) OpenFileAnimation(p *gfx.Picture) { e.fileAnim = p }

// FileAnimation is the animation Open_File_Animation_ opened last.
func (e *Engine) FileAnimation() *gfx.Picture { return e.fileAnim }

// SetFileAnimationFrame is Set_File_Animation_Frame_ (0x12d6b2), clamped to
// the last frame.
func (e *Engine) SetFileAnimationFrame(n int) {
	p := e.fileAnim
	if n >= p.Frames {
		n = p.Frames - 1
	}
	p.CurrentFrame = n
}

// FileAnimationFrame is Get_File_Animation_Frame_ (0x12d70b).
func (e *Engine) FileAnimationFrame() int { return e.fileAnim.CurrentFrame }

// DrawFileAnimationStencil is Draw_File_Animation_Stencil_ (0x12cd2d): the
// current frame, a sprite, goes into both the current and the back page
// (Draw_Animated_Sprite_Back_Diff_, no clipping), so stencil erasing keeps
// it. Every frame but the last is marked as in differential mode.
func (e *Engine) DrawFileAnimationStencil() error {
	v, p := e.V, e.fileAnim
	old := v.refresh
	if p.CurrentFrame+1 != p.Frames {
		v.refresh = RefreshDifferential
	}
	defer func() { v.refresh = old }()
	if p.Mask&gfx.MaskPalette != 0 {
		e.setAnimationPalette(p)
	}
	if p.Mask&gfx.MaskGlass != 0 || p.Kind() != gfx.KindSprite {
		return fmt.Errorf("Draw_File_Animation_Stencil of a %d-kind picture", p.Kind())
	}
	// Each run marks x..x+count of its row; a row's span is their union.
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
	return nil
}

// SetFadeSpeedPercent is Set_Fade_Speed_Percent_ (0xc5b85): the step of
// Fast_Fade_Out_.
func (e *Engine) SetFadeSpeedPercent(n int) { e.fadeStep = n }
