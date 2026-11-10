//go:build js

package host

// The browser host: the page's script (web/host.js, the global openorion2Host)
// owns the canvas, the DOM events and Web Audio; this side feeds it frames
// and sound and folds its events into System. Everything runs on the
// browser's one thread, so JS callbacks only signal a channel and never
// block, and the game goroutine yields to the browser while it polls.

import (
	"errors"
	"syscall/js"
	"time"

	"github.com/openorion2/openorion2/internal/audio"
	"github.com/openorion2/openorion2/internal/gfx"
)

const cooperative = true

// yield lets the browser run (events, frames, fetches) when the game has been
// polling for a while without sleeping or flipping; s.mu is held.
func (s *System) yield() {
	if s.polls++; s.polls < 64 {
		return
	}
	s.polls = 0
	if time.Since(s.lastYield) < 8*time.Millisecond {
		return
	}
	s.mu.Unlock()
	time.Sleep(time.Millisecond)
	s.mu.Lock()
	s.lastYield = time.Now()
}

// Run serves the game in the page's canvas until it returns. Headless runs
// and screenshots have no meaning here; scripted Clicks and Keys still work.
func Run(opt Options, game func(*System) error) error {
	h := js.Global().Get("openorion2Host")
	if h.IsUndefined() {
		return errors.New("openorion2Host is missing: load host.js before the game")
	}
	sys := &System{frame: NewScreen(), start: time.Now(), lastYield: time.Now(), Audio: audio.NewMixer()}
	done := make(chan error, 1)
	go func() { done <- game(sys) }()

	frames := make(chan struct{}, 1)
	onFrame := js.FuncOf(func(js.Value, []js.Value) any {
		select {
		case frames <- struct{}{}:
		default:
		}
		return nil
	})
	h.Call("start", onFrame)
	defer func() {
		h.Call("stop")
		onFrame.Release()
	}()

	shown := NewScreen()
	rgba := make([]byte, 4*gfx.ScreenW*gfx.ScreenH)
	pixels := h.Get("pixels")
	var sp webSpeaker
	var pending []js.Value
	var downAt [3]int // host frame each button last went down
	for n := 0; ; n++ {
		select {
		case gerr := <-done:
			if errors.Is(gerr, ErrQuit) {
				gerr = nil
			}
			return gerr
		case <-frames:
		}
		evs := h.Call("drain")
		for i := range evs.Length() {
			pending = append(pending, evs.Index(i))
		}
		pending = sys.webEvents(pending, n, &downAt)
		mx, my, _ := sys.tick(n, opt.Clicks, opt.Keys, shown)
		sp.feed(h, sys.Audio)
		shown.RGBA(rgba)
		overlayCursor(rgba, shown, mx, my)
		js.CopyBytesToJS(pixels, rgba)
		h.Call("present")
	}
}

// webEvents applies the frame's events up to the release of a button held
// for less than minHold frames: that release and what follows wait, so the
// game sees an instant click (a tap, a scripted one) held as the scripted
// clicks of the other hosts are.
func (s *System) webEvents(evs []js.Value, n int, downAt *[3]int) []js.Value {
	const minHold = 4
	for i, e := range evs {
		switch e.Get("t").String() {
		case "down":
			downAt[e.Get("b").Int()&3] = n
		case "up":
			if n < downAt[e.Get("b").Int()&3]+minHold {
				return append(evs[:0], evs[i:]...)
			}
		}
		s.webEvent(e)
	}
	return evs[:0]
}

// webEvent folds one event from host.js into the input state, as handle
// does for SDL's. Pointer positions arrive in screen pixels.
func (s *System) webEvent(e js.Value) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch e.Get("t").String() {
	case "move":
		s.mx, s.my = clamp(e.Get("x").Int(), gfx.ScreenW), clamp(e.Get("y").Int(), gfx.ScreenH)
		if s.buttons != 0 {
			s.bufferPress()
		}
	case "down", "up":
		s.mx, s.my = clamp(e.Get("x").Int(), gfx.ScreenW), clamp(e.Get("y").Int(), gfx.ScreenH)
		bit := e.Get("b").Int() & 3
		if e.Get("t").String() == "down" {
			s.buttons |= bit
			s.bufferPress()
		} else {
			s.buttons &^= bit
		}
	case "key":
		s.addKey(Key{
			Code:     uint32(e.Get("code").Int()),
			Scancode: e.Get("hid").Int(),
			Shift:    e.Get("shift").Bool(),
			Ctrl:     e.Get("ctrl").Bool(),
			Alt:      e.Get("alt").Bool(),
		})
	}
}

// webSpeaker hands mixed sound to host.js, which schedules it on Web Audio
// (or, before the page may play sound, drops it at the real-time rate so
// the mixer keeps time).
type webSpeaker struct {
	buf   []int16
	bytes []byte
	arr   js.Value
}

func (w *webSpeaker) feed(h js.Value, m *audio.Mixer) {
	n := h.Call("audioNeed").Int()
	if n <= 0 {
		return
	}
	if cap(w.buf) < 2*n {
		w.buf = make([]int16, 2*n)
		w.bytes = make([]byte, 4*n)
		w.arr = js.Global().Get("Uint8Array").New(4 * n)
	}
	buf, b := w.buf[:2*n], w.bytes[:4*n]
	m.Mix(buf)
	for i, v := range buf {
		b[2*i], b[2*i+1] = byte(v), byte(uint16(v)>>8)
	}
	js.CopyBytesToJS(w.arr, b)
	h.Call("audioPush", w.arr, n)
}

// ShowError has no message box in the browser: the console has the line.
func ShowError(title, msg string) {}
