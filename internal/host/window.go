//go:build !js

package host

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/Zyko0/go-sdl3/bin/binsdl"
	"github.com/Zyko0/go-sdl3/sdl"

	"github.com/openorion2/openorion2/internal/audio"
	"github.com/openorion2/openorion2/internal/gfx"
)

// mainCalls runs window sessions on the main thread for callers on other
// goroutines (tests), when ServeMain serves it.
var mainCalls chan func()

// ServeMain makes headless runs show their frames in a window when
// OPENORION2_WATCH is set: it serves them on the calling (main, locked) thread
// until done is closed. Tests call it from TestMain.
func ServeMain(done <-chan struct{}) {
	mainCalls = make(chan func())
	for {
		select {
		case f := <-mainCalls:
			f()
		case <-done:
			return
		}
	}
}

// Run opens the window, starts game in its own goroutine and serves it until
// the window is closed or game returns. A game returning ErrQuit after the
// window was closed is a normal exit.
func Run(opt Options, game func(*System) error) (err error) {
	if opt.Headless && mainCalls != nil && os.Getenv("OPENORION2_WATCH") != "" {
		opt.Headless, opt.Watch = false, true
		res := make(chan error)
		mainCalls <- func() { res <- Run(opt, game) }
		return <-res
	}
	if opt.Headless {
		return runHeadless(opt, game)
	}
	defer binsdl.Load().Unload()
	if err := sdl.Init(sdl.INIT_VIDEO); err != nil {
		return fmt.Errorf("SDL init: %w", err)
	}
	defer sdl.Quit()
	speaker := openSpeaker()
	defer speaker.close()

	w, h := windowSize(opt.Scale)
	window, renderer, err := sdl.CreateWindowAndRenderer(opt.Title, w, h, sdl.WINDOW_RESIZABLE|sdl.WINDOW_HIGH_PIXEL_DENSITY)
	if err != nil {
		return fmt.Errorf("create window: %w", err)
	}
	defer window.Destroy()
	defer renderer.Destroy()
	renderer.SetVSync(1)
	if opt.HideCursor {
		sdl.HideCursor()
	}

	tex, err := renderer.CreateTexture(sdl.PIXELFORMAT_RGBA32, sdl.TEXTUREACCESS_STREAMING, gfx.ScreenW, gfx.ScreenH)
	if err != nil {
		return fmt.Errorf("create texture: %w", err)
	}
	defer tex.Destroy()
	if err := tex.SetScaleMode(sdl.SCALEMODE_NEAREST); err != nil {
		return fmt.Errorf("texture scale mode: %w", err)
	}

	pv := &presenter{renderer: renderer, src: tex}
	defer pv.destroy()

	sys := &System{frame: NewScreen(), start: time.Now(), Audio: audio.NewMixer(), textCapable: true}
	if opt.Control != "" {
		if err := sys.serveControl(opt.Control); err != nil {
			return err
		}
	}
	done := make(chan error, 1)
	go func() { done <- game(sys) }()
	defer func() {
		// Make sure the game goroutine unwinds before SDL goes away.
		sys.mu.Lock()
		sys.closed = true
		sys.mu.Unlock()
		gerr := <-done
		if err == nil && gerr != nil && !errors.Is(gerr, ErrQuit) {
			err = gerr
		}
	}()

	shown := NewScreen()
	rgba := make([]byte, 4*gfx.ScreenW*gfx.ScreenH)
	if opt.Shot != "" {
		defer func() {
			if serr := shown.SavePNG(opt.Shot); serr != nil && err == nil {
				err = serr
			}
		}()
	}
	shots := newShotSaver(opt.ShotsAt)
	defer func() {
		if serr := shots.rest(shown); serr != nil && err == nil {
			err = serr
		}
	}()
	for n := 0; opt.Frames <= 0 || n < opt.Frames; n++ {
		var ev sdl.Event
		for sdl.PollEvent(&ev) {
			if ev.Type == sdl.EVENT_QUIT {
				return nil
			}
			if fullscreenEvent(window, &ev) {
				continue
			}
			if !opt.Watch {
				sys.handle(pv, &ev)
			}
		}
		select {
		case gerr := <-done:
			done <- gerr // the deferred cleanup reads it again
			return nil
		default:
		}
		mx, my, warp := sys.tick(n, opt.Clicks, opt.Keys, shown)
		sys.controlShown(shown)
		if text, ok := sys.takeClipboard(); ok {
			sdl.SetClipboardText(text)
		}
		if on, ok := sys.textChange(); ok {
			if on {
				window.StartTextInput()
			} else {
				window.StopTextInput()
			}
		}
		speaker.feed(sys.Audio)
		if err := shots.after(n+1, shown); err != nil {
			return err
		}
		if err := pv.layout(); err != nil {
			return err
		}
		if warp && !opt.Watch {
			if wx, wy, ok := pv.toWindow(mx, my); ok {
				window.WarpMouseIn(wx, wy)
			}
		}

		shown.RGBA(rgba)
		overlayCursor(rgba, shown, mx, my)
		if err := tex.Update(nil, rgba, 4*gfx.ScreenW); err != nil {
			return fmt.Errorf("texture update: %w", err)
		}
		if opt.WindowShot != "" && opt.Frames > 0 && n == opt.Frames-1 {
			pv.capture = opt.WindowShot
		}
		if err := pv.present(); err != nil {
			return err
		}
	}
	return nil
}

// fullscreenEvent turns maximizing the window into fullscreen and makes
// Alt+Enter toggle fullscreen; it reports whether it consumed the event.
// The window is restored before going fullscreen, so leaving fullscreen
// brings back the plain window rather than a maximized one, which would
// go straight back to fullscreen.
func fullscreenEvent(window *sdl.Window, ev *sdl.Event) bool {
	switch ev.Type {
	case sdl.EVENT_WINDOW_MAXIMIZED:
		window.Restore()
		window.SetFullscreen(true)
		return true
	case sdl.EVENT_KEY_DOWN:
		k := ev.KeyboardEvent()
		if k.Key != sdl.K_RETURN || k.Mod&sdl.KMOD_ALT == 0 {
			return false
		}
		if !k.Repeat {
			window.SetFullscreen(window.Flags()&sdl.WINDOW_FULLSCREEN == 0)
		}
		return true
	}
	return false
}

// handle folds one SDL event into the input state the game reads.
func (s *System) handle(pv *presenter, ev *sdl.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch ev.Type {
	case sdl.EVENT_MOUSE_MOTION:
		m := ev.MouseMotionEvent()
		s.mx, s.my = pv.toScreen(m.X, m.Y)
		if s.buttons != 0 {
			s.bufferPress()
		}
	case sdl.EVENT_MOUSE_BUTTON_DOWN, sdl.EVENT_MOUSE_BUTTON_UP:
		b := ev.MouseButtonEvent()
		s.mx, s.my = pv.toScreen(b.X, b.Y)
		bit := 0
		switch b.Button {
		case uint8(sdl.BUTTON_LEFT):
			bit = ButtonLeft
		case uint8(sdl.BUTTON_RIGHT):
			bit = ButtonRight
		}
		if b.Down {
			s.buttons |= bit
			s.bufferPress()
		} else {
			s.buttons &^= bit
		}
	case sdl.EVENT_MOUSE_WHEEL:
		s.wheel += ev.MouseWheelEvent().Y
	case sdl.EVENT_TEXT_INPUT:
		if t := ev.TextInputEvent().Text; t != "" {
			s.keys = append(s.keys, Key{Text: t})
		}
	case sdl.EVENT_KEY_DOWN:
		k := ev.KeyboardEvent()
		if k.Key == sdl.K_V && k.Mod&(sdl.KMOD_CTRL|sdl.KMOD_GUI) != 0 {
			// Paste: the text goes to the active input field, if any.
			if text, err := sdl.GetClipboardText(); err == nil && text != "" {
				s.keys = append(s.keys, Key{Paste: text})
			}
			break
		}
		s.keys = append(s.keys, Key{
			Code:     uint32(k.Key),
			Scancode: int(k.Scancode),
			Shift:    k.Mod&sdl.KMOD_SHIFT != 0,
			Ctrl:     k.Mod&sdl.KMOD_CTRL != 0,
			Alt:      k.Mod&sdl.KMOD_ALT != 0,
		})
	}
}

// windowSize returns the initial window size: scale times 640x480, or with
// scale zero, the largest 4:3 size that fits the primary display's usable area.
// Either way it is clamped to that area, which the window manager would do anyway.
func windowSize(scale int) (int, int) {
	fitW, fitH := gfx.ScreenW*2, gfx.ScreenH*2
	if r, err := sdl.GetPrimaryDisplay().UsableBounds(); err == nil && r.W > 0 && r.H > 0 {
		// Leave room for the title bar and window borders.
		fitW, fitH = int(r.W)*95/100, (int(r.H)-40)*95/100
	}
	w := gfx.ScreenW * scale
	if scale <= 0 || w > fitW || w*3/4 > fitH {
		w = min(fitW, fitH*4/3)
	}
	return w, w * 3 / 4
}

// ShowError shows msg in the system's message box, without a window: for
// what stops the game before it starts (no game files), where a player who
// started it from the desktop sees no console.
func ShowError(title, msg string) {
	defer binsdl.Load().Unload()
	sdl.ShowSimpleMessageBox(sdl.MESSAGEBOX_ERROR, title, msg, nil)
}
