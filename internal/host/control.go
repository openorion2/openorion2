//go:build !js

package host

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/openorion2/openorion2/internal/gfx"
)

// port-only, development: a remote control of a running game over HTTP
// (Options.Control), for playing it from another program as a person
// would: look at the screen, move the pointer (gliding there, so a watcher
// sees it move), click, drag, type.
//
//	GET  /screen.png                      the frame on screen
//	POST /click?x=&y=[&button=right]      glide there and click
//	POST /move?x=&y=                      glide there
//	POST /drag?x=&y=&tox=&toy=            press, glide, let go
//	POST /press?x=&y=[&button=right]      glide there and hold the button down
//	POST /release                         let the buttons go
//	POST /key?name=enter|esc|f10|t|alt+e  a key (names as -key's)
//	POST /text?s=...                      typed text (keys, or Key.Text when text input is on)
//	POST /paste?s=...                     a Ctrl+V of s
//	GET  /state                           the game's state for an agent (JSON)
//	GET  /fields                          the screen's input fields (JSON),
//	                                      as the game's last input poll saw them
//
// Each command answers once its gesture has played and a few frames more
// were drawn.

// gestureStep is one frame of a gesture: the pointer, the buttons, a key.
type gestureStep struct {
	x, y    int
	buttons int
	key     *Key
}

// controlState is the queue of steps and the last frame shown.
type controlState struct {
	steps []gestureStep
	done  []chan struct{} // closed when steps runs dry, after settle frames
	idle  int             // frames since the steps ran dry

	shotMu sync.Mutex
	shot   []byte // the last frame shown, RGBA

	fields []ControlField // the screen's input fields, under System.mu

	stateWanted bool     // /state waits for the game's next poll
	state       chan any // its answer

	act     []byte      // an /act waiting for the game's next poll
	actDone chan string // its answer
}

// TakeAct is a waiting /act's request, nil when none.
func (s *System) TakeAct() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.control == nil {
		return nil
	}
	a := s.control.act
	s.control.act = nil
	return a
}

// ActDone answers the /act taken.
func (s *System) ActDone(result string) {
	s.mu.Lock()
	ch := s.control.actDone
	s.mu.Unlock()
	select {
	case ch <- result:
	default:
	}
}

// StateWanted reports whether /state waits for the game's state.
func (s *System) StateWanted() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.control != nil && s.control.stateWanted
}

// PublishState answers a waiting /state with v (JSON-encodable).
func (s *System) PublishState(v any) {
	s.mu.Lock()
	c := s.control
	if c == nil || !c.stateWanted {
		s.mu.Unlock()
		return
	}
	c.stateWanted = false
	ch := c.state
	s.mu.Unlock()
	select {
	case ch <- v:
	default:
	}
}

// ControlField is an input field of the screen, for /fields.
type ControlField struct {
	N              int    `json:"n"`
	X1, Y1, X2, Y2 int    `json:"-"`
	Rect           [4]int `json:"rect"` // x1, y1, x2, y2 inclusive
	Type           int    `json:"type"`
	Hotkey         string `json:"hotkey,omitempty"`
	Label          string `json:"label,omitempty"`
}

// Controlled reports whether a remote control listens.
func (s *System) Controlled() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.control != nil
}

// PublishFields gives the control the screen's input fields.
func (s *System) PublishFields(f []ControlField) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.control != nil {
		s.control.fields = f
	}
}

// controlSettle is how many frames a command waits after its gesture.
const controlSettle = 12

// controlTick plays the next gesture step; under s.mu, from tick.
func (s *System) controlTick() {
	c := s.control
	if c == nil {
		return
	}
	if len(c.steps) == 0 {
		if c.idle++; c.idle >= controlSettle {
			for _, d := range c.done {
				close(d)
			}
			c.done = nil
		}
		return
	}
	c.idle = 0
	st := c.steps[0]
	c.steps = c.steps[1:]
	if st.key != nil {
		s.keys = append(s.keys, *st.key)
		return
	}
	// The game draws the pointer itself; the system's is not warped (its
	// motion events would come back scaled on a Retina screen).
	s.mx, s.my = clamp(st.x, gfx.ScreenW), clamp(st.y, gfx.ScreenH)
	down := st.buttons &^ s.buttons
	s.buttons = st.buttons
	if down != 0 {
		s.bufferPress()
	}
}

// controlShown keeps a copy of the frame on screen for /screen.png.
func (s *System) controlShown(shown *Screen) {
	c := s.control
	if c == nil {
		return
	}
	c.shotMu.Lock()
	if len(c.shot) != 4*len(shown.Pix) {
		c.shot = make([]byte, 4*len(shown.Pix))
	}
	shown.RGBA(c.shot)
	c.shotMu.Unlock()
}

// enqueue adds steps and waits until they played.
func (s *System) enqueue(steps []gestureStep) {
	d := make(chan struct{})
	s.mu.Lock()
	c := s.control
	c.steps = append(c.steps, steps...)
	c.done = append(c.done, d)
	s.mu.Unlock()
	select {
	case <-d:
	case <-time.After(30 * time.Second):
	}
}

// glide is the pointer's way from where it is to (x, y), about a pixel and
// a half a frame at first, at most half a second, ending on the spot.
func (s *System) glide(x, y, buttons int) []gestureStep {
	s.mu.Lock()
	fx, fy := s.mx, s.my
	s.mu.Unlock()
	d := math.Hypot(float64(x-fx), float64(y-fy))
	return glideFrom(fx, fy, x, y, buttons, min(30, max(4, int(d/12))))
}

// clickFrames is how long a click holds its button, about a fifth of a second.
const clickFrames = 12

// glideFrom is a glide from (fx, fy) to (x, y) in n frames.
func glideFrom(fx, fy, x, y, buttons, n int) []gestureStep {
	out := make([]gestureStep, 0, n)
	for i := 1; i <= n; i++ {
		t := float64(i) / float64(n)
		t = t * t * (3 - 2*t)
		out = append(out, gestureStep{x: fx + int(math.Round(float64(x-fx)*t)), y: fy + int(math.Round(float64(y-fy)*t)), buttons: buttons})
	}
	return out
}

func hold(x, y, buttons, frames int) []gestureStep {
	out := make([]gestureStep, frames)
	for i := range out {
		out[i] = gestureStep{x: x, y: y, buttons: buttons}
	}
	return out
}

// serveControl answers the control's commands on addr until the process ends.
func (s *System) serveControl(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("control: %w", err)
	}
	s.control = &controlState{}
	mux := http.NewServeMux()
	num := func(r *http.Request, k string) (int, error) {
		v, err := strconv.Atoi(r.FormValue(k))
		if err != nil {
			return 0, fmt.Errorf("%s: %w", k, err)
		}
		return v, nil
	}
	point := func(r *http.Request, kx, ky string) (int, int, error) {
		x, err := num(r, kx)
		if err != nil {
			return 0, 0, err
		}
		y, err := num(r, ky)
		return x, y, err
	}
	handle := func(pattern string, f func(r *http.Request) error) {
		mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
			if err := f(r); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			fmt.Fprintln(w, "ok")
		})
	}
	var actMu sync.Mutex // one /act at a time
	mux.HandleFunc("POST /act", func(w http.ResponseWriter, r *http.Request) {
		actMu.Lock()
		defer actMu.Unlock()
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<16))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.mu.Lock()
		c := s.control
		c.actDone = make(chan string, 1)
		c.act = body
		ch := c.actDone
		s.mu.Unlock()
		select {
		case res := <-ch:
			fmt.Fprintln(w, res)
		case <-time.After(5 * time.Second):
			s.mu.Lock()
			c.act = nil
			s.mu.Unlock()
			http.Error(w, "the game does not poll input now", http.StatusServiceUnavailable)
		}
	})
	mux.HandleFunc("GET /state", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		c := s.control
		if c.state == nil {
			c.state = make(chan any, 1)
		}
		c.stateWanted = true
		ch := c.state
		s.mu.Unlock()
		select {
		case v := <-ch:
			w.Header().Set("Content-Type", "application/json")
			enc := json.NewEncoder(w)
			enc.SetIndent("", " ")
			enc.Encode(v)
		case <-time.After(5 * time.Second):
			http.Error(w, "the game does not poll input now (busy, or a screen without input)", http.StatusServiceUnavailable)
		}
	})
	mux.HandleFunc("GET /fields", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		f := s.control.fields
		s.mu.Unlock()
		for i := range f {
			f[i].Rect = [4]int{f[i].X1, f[i].Y1, f[i].X2, f[i].Y2}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(f)
	})
	mux.HandleFunc("GET /screen.png", func(w http.ResponseWriter, r *http.Request) {
		c := s.control
		img := image.NewNRGBA(image.Rect(0, 0, gfx.ScreenW, gfx.ScreenH))
		c.shotMu.Lock()
		shown := len(c.shot) > 0
		copy(img.Pix, c.shot)
		c.shotMu.Unlock()
		if !shown {
			http.Error(w, "no frame shown yet", http.StatusServiceUnavailable)
			return
		}
		var b bytes.Buffer
		png.Encode(&b, img)
		w.Header().Set("Content-Type", "image/png")
		w.Write(b.Bytes())
	})
	handle("POST /move", func(r *http.Request) error {
		x, y, err := point(r, "x", "y")
		if err != nil {
			return err
		}
		s.enqueue(s.glide(x, y, 0))
		return nil
	})
	handle("POST /click", func(r *http.Request) error {
		x, y, err := point(r, "x", "y")
		if err != nil {
			return err
		}
		bit := ButtonLeft
		if r.FormValue("button") == "right" {
			bit = ButtonRight
		}
		// About a fifth of a second down, as a person clicks: some screens
		// read the button only now and then.
		steps := append(s.glide(x, y, 0), hold(x, y, 0, 3)...)
		steps = append(steps, hold(x, y, bit, clickFrames)...)
		s.enqueue(append(steps, hold(x, y, 0, 1)...))
		return nil
	})
	handle("POST /drag", func(r *http.Request) error {
		x, y, err := point(r, "x", "y")
		if err != nil {
			return err
		}
		tx, ty, err := point(r, "tox", "toy")
		if err != nil {
			return err
		}
		// Held on the spot long enough for a screen that reads the button now
		// and then to pick up what is under it, then a slow glide.
		steps := append(s.glide(x, y, 0), hold(x, y, ButtonLeft, clickFrames)...)
		steps = append(steps, glideFrom(x, y, tx, ty, ButtonLeft, 40)...)
		steps = append(steps, hold(tx, ty, ButtonLeft, clickFrames)...)
		s.enqueue(append(steps, hold(tx, ty, 0, clickFrames)...))
		return nil
	})
	handle("POST /press", func(r *http.Request) error {
		x, y, err := point(r, "x", "y")
		if err != nil {
			return err
		}
		bit := ButtonLeft
		if r.FormValue("button") == "right" {
			bit = ButtonRight
		}
		s.mu.Lock()
		held := s.buttons
		s.mu.Unlock()
		steps := s.glide(x, y, held)
		s.enqueue(append(steps, hold(x, y, held|bit, clickFrames)...))
		return nil
	})
	handle("POST /release", func(r *http.Request) error {
		s.mu.Lock()
		x, y := s.mx, s.my
		s.mu.Unlock()
		s.enqueue(hold(x, y, 0, clickFrames))
		return nil
	})
	handle("POST /key", func(r *http.Request) error {
		keys, err := ParseKeys(r.FormValue("name"))
		if err != nil {
			return err
		}
		s.enqueue(keySteps(s, keys))
		return nil
	})
	handle("POST /text", func(r *http.Request) error {
		text := r.FormValue("s")
		if s.TextInput() {
			s.enqueue([]gestureStep{{key: &Key{Text: text}}})
			return nil
		}
		var keys []Key
		for _, ch := range strings.ToLower(text) {
			k, err := ParseKeys(string(ch))
			if err != nil {
				return err
			}
			keys = append(keys, k...)
		}
		s.enqueue(keySteps(s, keys))
		return nil
	})
	handle("POST /paste", func(r *http.Request) error {
		s.enqueue([]gestureStep{{key: &Key{Paste: r.FormValue("s")}}})
		return nil
	})
	go http.Serve(ln, mux)
	return nil
}

// keySteps types keys one per two frames, the pointer kept where it is.
func keySteps(s *System, keys []Key) []gestureStep {
	s.mu.Lock()
	x, y, b := s.mx, s.my, s.buttons
	s.mu.Unlock()
	var out []gestureStep
	for i := range keys {
		out = append(out, gestureStep{key: &keys[i]}, gestureStep{x: x, y: y, buttons: b})
	}
	return out
}
