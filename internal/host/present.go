//go:build !js

package host

import (
	"fmt"
	"math"

	"github.com/Zyko0/go-sdl3/sdl"

	"github.com/openorion2/openorion2/internal/gfx"
)

// presenter shows the 640x480 frame in the window, letterboxed at 4:3. A
// plain nearest-neighbour stretch by a fractional factor makes pixels
// uneven, which wrecks small text; a linear one blurs it. So the frame is
// first enlarged by a whole factor with nearest neighbour into an
// intermediate texture, which is then scaled linearly to the final size
// ("sharp bilinear"): pixels stay square and even, edges only get a
// one-pixel blend. The window uses the display's full pixel density.
type presenter struct {
	renderer *sdl.Renderer
	src      *sdl.Texture // 640x480, nearest
	up       *sdl.Texture // 640k x 480k, linear
	k        int
	dst      sdl.FRect // where the frame lands, in output pixels
	capture  string    // when set, the next presented frame is saved here (BMP)
}

func (p *presenter) destroy() {
	if p.up != nil {
		p.up.Destroy()
	}
}

// layout recomputes the destination rectangle and the integer factor for
// the current output size.
func (p *presenter) layout() error {
	ow, oh, err := p.renderer.CurrentOutputSize()
	if err != nil || ow <= 0 || oh <= 0 {
		return nil
	}
	scale := math.Min(float64(ow)/gfx.ScreenW, float64(oh)/gfx.ScreenH)
	w, h := float32(gfx.ScreenW*scale), float32(gfx.ScreenH*scale)
	p.dst = sdl.FRect{X: (float32(ow) - w) / 2, Y: (float32(oh) - h) / 2, W: w, H: h}
	k := max(1, int(math.Ceil(scale-1e-6)))
	if k == p.k && p.up != nil {
		return nil
	}
	if p.up != nil {
		p.up.Destroy()
		p.up = nil
	}
	up, err := p.renderer.CreateTexture(sdl.PIXELFORMAT_RGBA32, sdl.TEXTUREACCESS_TARGET, gfx.ScreenW*k, gfx.ScreenH*k)
	if err != nil {
		return fmt.Errorf("create upscale texture: %w", err)
	}
	if err := up.SetScaleMode(sdl.SCALEMODE_LINEAR); err != nil {
		return fmt.Errorf("upscale texture mode: %w", err)
	}
	p.up, p.k = up, k
	return nil
}

func (p *presenter) present() error {
	r := p.renderer
	if p.up == nil {
		return nil
	}
	if err := r.SetRenderTarget(p.up); err != nil {
		return fmt.Errorf("render target: %w", err)
	}
	if err := r.RenderTexture(p.src, nil, nil); err != nil {
		return fmt.Errorf("render: %w", err)
	}
	if err := r.SetRenderTarget(nil); err != nil {
		return fmt.Errorf("render target: %w", err)
	}
	r.SetDrawColor(0, 0, 0, 255)
	r.Clear()
	if err := r.RenderTexture(p.up, nil, &p.dst); err != nil {
		return fmt.Errorf("render: %w", err)
	}
	if p.capture != "" {
		surf, err := r.ReadPixels(nil)
		if err != nil {
			return fmt.Errorf("read pixels: %w", err)
		}
		err = surf.SaveBMP(p.capture)
		surf.Destroy()
		if err != nil {
			return fmt.Errorf("save window shot: %w", err)
		}
		p.capture = ""
	}
	r.Present()
	return nil
}

// toScreen maps window coordinates to the 640x480 screen.
func (p *presenter) toScreen(wx, wy float32) (int, int) {
	px, py, err := p.renderer.RenderCoordinatesFromWindow(wx, wy)
	if err != nil || p.dst.W <= 0 || p.dst.H <= 0 {
		return 0, 0
	}
	x := int(math.Floor(float64((px - p.dst.X) * gfx.ScreenW / p.dst.W)))
	y := int(math.Floor(float64((py - p.dst.Y) * gfx.ScreenH / p.dst.H)))
	return clamp(x, gfx.ScreenW), clamp(y, gfx.ScreenH)
}

// toWindow maps the centre of a screen pixel to window coordinates.
func (p *presenter) toWindow(x, y int) (float32, float32, bool) {
	if p.dst.W <= 0 {
		return 0, 0, false
	}
	px := p.dst.X + (float32(x)+0.5)*p.dst.W/gfx.ScreenW
	py := p.dst.Y + (float32(y)+0.5)*p.dst.H/gfx.ScreenH
	wx, wy, err := p.renderer.RenderCoordinatesToWindow(px, py)
	return wx, wy, err == nil
}
