package engine

import (
	"fmt"
	"image/color"
	"time"

	"github.com/openorion2/openorion2/internal/gfx"
	"github.com/openorion2/openorion2/internal/host"
	"github.com/openorion2/openorion2/internal/lbx"
)

// TickPeriod is one BIOS timer tick, the unit Mark_Time_ and Release_Time_
// count in: the Miles timer keeps chaining the BIOS handler at this period.
const TickPeriod = 54925 * time.Microsecond

// Engine ties the video state to the host and the clock.
type Engine struct {
	snd soundState // sound.c
	V   *Video
	Sys *host.System
	// KeyHook, when set, sees every key before the fields do and keeps
	// those it returns true for (port-only: the lobby's Unicode lines).
	KeyHook func(host.Key) bool

	// DrawFields is Draw_Visible_Fields, run on every flip; the fields layer
	// installs it.
	DrawFields func()
	// NetUpdate is Mox_Net_Update_, which Mox_Update_ runs after the
	// sound; the game installs it (docs/net/transport.md).
	NetUpdate func()

	fonts   *lbx.Archive  // FONTS.LBX, for palettes
	mark    int64         // the tick Mark_Time_ recorded
	pacedAt time.Duration // when pace's next frame is due
	paceFPS int           // PaceNextToggle's rate for the next TogglePages
	page    *host.Screen  // what is handed to the host

	f    fieldState
	text *Text

	fadePercent int // fade_percent
	glass       glassState
	remapS      remapState
	para        Paragraphs
	// header is animation_header: the picture Draw handled last.
	header   *gfx.Picture
	fadeStep int                      // fade_speed_in_percent
	fileAnim *gfx.Picture             // the file animation Open_File_Animation_ opened
	multi    [multiSlots]*gfx.Picture // Open_Multi_File_Animation_'s slots 1, 2

	savedPalette [256]Color // saved_palette
	cycle        cycleState

	// InputHook, if set, runs at the start of every Get_Input: tests use it
	// to script input against the game's state.
	InputHook func()
	// OnToggle, if set, runs after every Toggle_Pages (tests).
	OnToggle func()
	// ControlState, if set, describes the game for the remote control's
	// /state (port-only, development).
	ControlState func() any
	// ControlAct, if set, carries out an /act request and says how it went.
	ControlAct func(req []byte) string
}

// New returns an engine drawing for sys, with game files in dir's FONTS.LBX.
func New(sys *host.System, fonts *lbx.Archive) *Engine {
	e := &Engine{V: NewVideo(), Sys: sys, fonts: fonts, page: host.NewScreen(), fadeStep: 10}
	e.f.init()
	e.DrawFields = e.drawVisibleFields
	// Init_Drivers_ sorts the still black palette (0x113d71); until the first
	// later sort Find_Closest_Color searches all colours through that list.
	e.sortPaletteList()
	return e
}

// Ticks is the BIOS tick count.
func (e *Engine) Ticks() int64 { return int64(e.Sys.Elapsed() / TickPeriod) }

// MarkTime is Mark_Time_.
func (e *Engine) MarkTime() { e.mark = e.Ticks() }

// ReleaseTime is Release_Time_(n): wait until n ticks have passed since the
// last MarkTime. It does not mark again.
func (e *Engine) ReleaseTime(n int) error {
	for {
		d := e.Ticks() - e.mark
		if d < 0 || d >= int64(n) {
			return nil
		}
		wait := time.Duration(e.mark+int64(n))*TickPeriod - e.Sys.Elapsed()
		if err := e.Sys.Sleep(max(wait, time.Millisecond)); err != nil {
			return err
		}
	}
}

// MoxUpdate is Mox_Update_ (0xfe8be): Sound_Update_, then Mox_Net_Update_
// (the game's NetUpdate, which does nothing outside network games).
func (e *Engine) MoxUpdate() {
	e.soundUpdate()
	if e.NetUpdate != nil {
		e.NetUpdate()
	}
}

// SoundUpdate is Sound_Update_ alone, as the network code calls it.
func (e *Engine) SoundUpdate() { e.soundUpdate() }

// TogglePages is _TOGGLE_PAGES_: Mox_Update_ then Toggle_Pages, which redraws
// the fields, gets off onto the screen and hands the result to the host.
func (e *Engine) TogglePages() error {
	fps := e.paceFPS
	e.paceFPS = 0
	latest := fps > 0 && !e.Sys.Virtual()
	e.MoxUpdate()
	if e.DrawFields != nil && e.f.drawFieldsEnabled {
		cur := e.V.Current
		e.V.SetPageOff()
		e.DrawFields()
		e.V.Current = cur
	}
	mx, my, _ := e.Sys.Mouse()
	e.V.flip(mx, my)
	err := e.present(latest)
	if e.OnToggle != nil {
		e.OnToggle()
	}
	if err != nil || !latest {
		return err
	}
	return e.pace(fps)
}

// PaceNextToggle makes the next TogglePages one of the original's loops
// that have no timer and so ran as fast as the machine drew (Move_Ship_):
// in real time its page replaces any the display has not shown yet and the
// loop is held to fps frames a second instead of to the display's refresh.
// Headless runs pace as usual, so traces and frame captures are unchanged.
func (e *Engine) PaceNextToggle(fps int) { e.paceFPS = fps }

// pace waits until the next of a run of frames at fps is due.
func (e *Engine) pace(fps int) error {
	period := time.Second / time.Duration(fps)
	now := e.Sys.Elapsed()
	if e.pacedAt == 0 || now-e.pacedAt > 4*period {
		e.pacedAt = now // a new run of paced frames, or one fell far behind
	}
	e.pacedAt += period
	if wait := e.pacedAt - now; wait > 0 {
		return e.Sys.Sleep(wait)
	}
	return nil
}

// present hands the screen to the host through the DAC.
func (e *Engine) present(latest bool) error {
	copy(e.page.Pix, e.V.Screen)
	for i, c := range e.V.DAC {
		e.page.Palette[i] = color.RGBA{vga(c[0]), vga(c[1]), vga(c[2]), 0xff}
	}
	e.page.Cursor = e.cursorImage()
	if latest {
		return e.Sys.FlipLatest(e.page)
	}
	return e.Sys.Flip(e.page)
}

func vga(v byte) byte {
	v &= 0x3f
	return v<<2 | v>>4
}

// LoadPalette is Load_Palette(entry, start, end): copy colours start..end of
// FONTS.LBX entry+1 into current_palette and mark them changed.
func (e *Engine) LoadPalette(entry, start, end int) error {
	data, err := e.fonts.Entry(entry + 1)
	if err != nil {
		return err
	}
	if len(data) < 4*(end+1) {
		return fmt.Errorf("palette entry %d too short", entry+1)
	}
	for i := start; i <= end; i++ {
		e.V.Palette[i] = Color{data[4*i], data[4*i+1], data[4*i+2], data[4*i+3]}
	}
	// mouse_palette: the cursor images stored after the colours.
	if len(data) > 0x400 {
		e.f.mouseImages = append([]byte(nil), data[0x400:]...)
	}
	e.V.SetPaletteChanges(start, end)
	return nil
}

// Draw is animate.c's Draw (0x12a478): apply the picture's palette chunk,
// mark the scan list, blit the current frame (glassed ones through the
// glass tables) and advance it.
func (e *Engine) Draw(x, y int, p *gfx.Picture) error { return e.draw(x, y, p, false) }

// DrawNoGlass is Draw_No_Glass (0x129ff9): unclipped, glass colours are left
// out; clipped, it calls the glassed blitters as Draw does.
func (e *Engine) DrawNoGlass(x, y int, p *gfx.Picture) error { return e.draw(x, y, p, true) }

func (e *Engine) draw(x, y int, p *gfx.Picture, noGlass bool) error {
	v := e.V
	e.header = p
	if p.Mask&gfx.MaskPalette != 0 {
		e.setAnimationPalette(p)
	}
	if v.Clipping {
		x2, y2 := x+p.Width-1, y+p.Height-1
		if v.WinX2 < x || v.WinY2 < y || x2 < v.WinX1 || y2 < v.WinY1 {
			advanceFrame(p)
			return nil
		}
		if x < v.WinX1 || y < v.WinY1 || v.WinX2 < x2 || v.WinY2 < y2 {
			return e.drawClipped(x, y, p)
		}
	}
	if v.refresh > RefreshFull+1 {
		if p.Width == 0 || p.Height == 0 {
			return nil // the original returns before advancing the frame
		}
		if p.Kind() != gfx.KindBackground {
			v.AddSquare(x, y, x+p.Width-1, y+p.Height-1)
		}
	}
	if p.Mask&gfx.MaskGlass != 0 {
		if err := p.DrawGlassAt(v.Current, W, x, y, p.CurrentFrame, &e.glass.remap, noGlass); err != nil {
			return err
		}
	} else if err := p.DrawAt(v.Current, W, x, y, p.CurrentFrame); err != nil {
		return err
	}
	if p.Kind() == gfx.KindBackground && v.refresh > RefreshFull+1 {
		v.AddSquare(0, 0, W-1, H-1)
	}
	advanceFrame(p)
	return nil
}

// drawClipped is Draw's path for a picture the window cuts: the scan list
// gets the whole, unclipped rectangle, then the clipped blitter runs.
func (e *Engine) drawClipped(x, y int, p *gfx.Picture) error {
	v := e.V
	switch p.Kind() {
	case gfx.KindBackground:
		return fmt.Errorf("Screen Background can't be clipped")
	case gfx.KindSprite, gfx.KindBitmap:
	default:
		return fmt.Errorf("Refresh Mask can't be drawn")
	}
	if v.refresh > RefreshFull+1 {
		v.AddSquare(x, y, x+p.Width-1, y+p.Height-1)
	}
	w := gfx.Window{X1: v.WinX1, Y1: v.WinY1, X2: v.WinX2, Y2: v.WinY2}
	if p.Mask&gfx.MaskGlass != 0 {
		if err := p.DrawClippedGlass(v.Current, W, x, y, p.CurrentFrame, w, &e.glass.remap); err != nil {
			return err
		}
	} else if err := p.DrawClipped(v.Current, W, x, y, p.CurrentFrame, w); err != nil {
		return err
	}
	advanceFrame(p)
	return nil
}

// advanceFrame is the frame bookkeeping at the end of Draw.
func advanceFrame(p *gfx.Picture) {
	p.CurrentFrame++
	if p.Mask&gfx.MaskHold == 0 {
		if p.CurrentFrame >= p.Frames {
			p.CurrentFrame = p.RepeatFrame
		}
		return
	}
	if p.RepeatFrame < p.CurrentFrame {
		if p.CurrentFrame == p.RepeatFrame+1 {
			p.CurrentFrame = 0
		} else if p.CurrentFrame >= p.Frames {
			p.CurrentFrame = p.Frames - 1
		}
	}
}

// setAnimationPalette is Set_Animation_Palette: entries of the chunk that
// differ from current_palette, changed flag included, are copied and marked.
func (e *Engine) setAnimationPalette(p *gfx.Picture) {
	ch := p.Palette
	if ch == nil {
		return
	}
	for i := 0; i < len(ch.Raw)/4; i++ {
		c := &e.V.Palette[ch.Start+i]
		r := ch.Raw[4*i : 4*i+4]
		if *c != (Color{r[0], r[1], r[2], r[3]}) {
			*c = Color{1, r[1], r[2], r[3]}
		}
	}
}

// MouseButton is Mouse_Button: the buttons held now, 1 left, 2 right.
func (e *Engine) MouseButton() int { return e.mouseButton() }

// Pointer is Pointer_X and Pointer_Y: mouse_x and mouse_y.
func (e *Engine) Pointer() (int, int) {
	x, y, _ := e.Sys.Mouse()
	return x, y
}

// AnimationHeader is animation_header: the last picture Draw handled.
func (e *Engine) AnimationHeader() *gfx.Picture { return e.header }

// SaveCapturePNG is what Screen_Flic_Capture_ (0x1101f0) writes: the
// screen with current_palette, not the DAC, which direct DAC writes
// (Store_Single_Palette_Color_) and fades change without it.
func (e *Engine) SaveCapturePNG(path string) error {
	s := host.NewScreen()
	copy(s.Pix, e.V.Screen)
	for i, c := range e.V.Palette {
		s.Palette[i] = color.RGBA{vga(c.R), vga(c.G), vga(c.B), 0xff}
	}
	return s.SavePNG(path)
}

// SaveScreenPNG writes what the screen shows through the DAC, without the
// cursor (development: frames to compare with the original's captures).
func (e *Engine) SaveScreenPNG(path string) error {
	s := host.NewScreen()
	copy(s.Pix, e.V.Screen)
	for i, c := range e.V.DAC {
		s.Palette[i] = color.RGBA{vga(c[0]), vga(c[1]), vga(c[2]), 0xff}
	}
	return s.SavePNG(path)
}
