// Package host is the SDL3 layer the port runs on: a window showing the
// engine's 640x480 8-bit screen through its palette, and input.
//
// The game runs as the original did, a sequential program with its own loops,
// in a goroutine of its own. It hands finished pages over with Flip and reads
// the mouse and keyboard through System; the main goroutine only pumps SDL
// events and presents the latest page. Nothing above this package knows SDL.
package host

import (
	"errors"
	"image"
	"image/png"
	"os"
	"sync"
	"time"

	"github.com/openorion2/openorion2/internal/audio"

	"github.com/openorion2/openorion2/internal/gfx"
)

// ErrQuit is returned from System calls once the window has been closed; the
// game should unwind and return it.
var ErrQuit = errors.New("window closed")

// CursorSize is the side of a mouse cursor image.
const CursorSize = 24

// Screen is a page the game draws into.
type Screen struct {
	Pix     []byte // gfx.ScreenW*gfx.ScreenH palette indices
	Palette gfx.Palette
	// Cursor is the pointer image, CursorSize squared palette indices with 0
	// transparent, drawn over the page at the pointer with its top left corner
	// on the hotspot; nil for none. It is never part of the page itself.
	Cursor []byte
}

// NewScreen returns a black screen.
func NewScreen() *Screen {
	return &Screen{Pix: make([]byte, gfx.ScreenW*gfx.ScreenH)}
}

// RGBA converts the screen to 8-bit RGBA, row after row.
func (s *Screen) RGBA(dst []byte) {
	for i, v := range s.Pix {
		c := s.Palette[v]
		dst[4*i], dst[4*i+1], dst[4*i+2], dst[4*i+3] = c.R, c.G, c.B, 0xff
	}
}

// SavePNG writes the screen as it would be shown.
func (s *Screen) SavePNG(path string) error {
	img := image.NewNRGBA(image.Rect(0, 0, gfx.ScreenW, gfx.ScreenH))
	s.RGBA(img.Pix)
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

// Mouse button bits as System.Mouse reports them.
const (
	ButtonLeft  = 1
	ButtonRight = 2
)

// Key is one key press. Code is the SDL keycode, which for printable keys is
// the lowercase character; Scancode is the physical key (USB HID usage).
type Key struct {
	Code     uint32
	Scancode int
	Shift    bool
	Ctrl     bool
	Alt      bool
	Paste    string // port-only: Ctrl+V (Cmd+V) with the clipboard's text; Code is 0
	Text     string // port-only: typed text (UTF-8) while text input is on; Code is 0
}

// Press is a mouse button press remembered between polls, as the original's
// mouse interrupt records it.
type Press struct{ X, Y, Buttons int }

// Common key codes (SDL keycodes, which for these are their ASCII).
const (
	KeyEscape    = uint32(0x1b)
	KeyReturn    = uint32(0x0d)
	KeyBackspace = uint32(0x08)
)

// System is what the game goroutine sees of the host.
type System struct {
	mu      sync.Mutex
	frame   *Screen // latest page handed over, owned by the host
	fresh   bool
	mx, my  int
	buttons int
	keys    []Key
	control *controlState // port-only: the remote control (control.go), nil when off
	wheel   float32       // port-only: the wheel's notches not read yet, + up
	closed  bool
	start   time.Time
	frames  int

	// animSpeed is community patch 1.51's animation speed, -3..3, which
	// PageUp and PageDown step as the keys arrive (AnimSpeed).
	animSpeed   int
	animKeysOff bool // port-only: true while PageUp/PageDown scroll (the lobby)

	// The buffered press and its two flags (mouse_buffer_flag/flag2).
	press          Press
	pressA, pressB bool
	warp           bool // a pointer warp is pending

	// Virtual time (headless): the clock is frames/60 s and the host takes
	// the next frame once the game waits, so runs don't depend on the CPU.
	virtual bool
	cond    *sync.Cond // signalled on every frame

	// Audio is what the game plays; the windowed host feeds it to the
	// sound card, headless runs leave it unheard.
	Audio   *audio.Mixer
	waiting bool // the game is blocked in Sleep or Flip
	waitAt  int  // the frame count it blocked at
	polls   int  // input and clock reads since the last frame (headless)

	lastYield time.Time // when a cooperative host last let the browser run

	clipboard     string // port-only: the last text the game copied
	clipboardSent bool   // the window has put it on the system's clipboard

	// port-only: text input (typed UTF-8 and the system's IME), asked for by
	// the game, applied by the window on its own thread.
	textWanted, textOn bool
	textCapable        bool // the host delivers Key.Text (the window)
}

// SetTextInput asks for typed text as Key.Text (any language, through the
// system's input method) or not.
func (s *System) SetTextInput(on bool) {
	s.mu.Lock()
	s.textWanted = on
	s.mu.Unlock()
}

// TextInput reports whether typed text comes as Key.Text: asked for and
// the host can (the window can; headless runs and the browser cannot).
func (s *System) TextInput() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.textWanted && s.textCapable
}

// textChange is the text input state to apply, if it changed.
func (s *System) textChange() (on, changed bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.textWanted == s.textOn {
		return false, false
	}
	s.textOn = s.textWanted
	return s.textOn, true
}

// SetClipboard puts text on the system's clipboard (the window does it on
// its own thread at the next frame; headless runs only keep it).
func (s *System) SetClipboard(text string) {
	s.mu.Lock()
	s.clipboard, s.clipboardSent = text, false
	s.mu.Unlock()
}

// Clipboard is the last text the game copied (tests).
func (s *System) Clipboard() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.clipboard
}

// takeClipboard is a copy not yet on the system's clipboard, if any.
func (s *System) takeClipboard() (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.clipboardSent || s.clipboard == "" {
		return "", false
	}
	s.clipboardSent = true
	return s.clipboard, true
}

// pollBudget is how many input and clock reads a headless game makes in one
// frame before it waits for the next: a loop polling without Sleep or Flip
// then advances by its own reads, never by wall time.
const pollBudget = 4000

// poll counts a read of the input or the clock; past the budget the game
// waits for the next frame (virtual time only); s.mu is held.
func (s *System) poll() {
	if !s.virtual {
		if cooperative {
			s.yield()
		}
		return
	}
	if s.closed {
		return
	}
	if s.polls++; s.polls >= pollBudget {
		s.wait()
	}
}

// wait blocks the game until the next frame (virtual time); s.mu is held.
func (s *System) wait() {
	s.waiting, s.waitAt = true, s.frames
	s.cond.Wait()
	s.waiting = false
}

// Flip hands a finished page to the host. The page is copied, so the game
// may keep drawing into it. It waits until the host has taken the previous
// page, which paces the game to the display as the copy to video memory
// paced the original. It returns ErrQuit once the window is closed.
func (s *System) Flip(page *Screen) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for s.fresh && !s.closed {
		if s.virtual {
			s.wait()
			continue
		}
		s.mu.Unlock()
		time.Sleep(time.Millisecond)
		s.mu.Lock()
	}
	if s.closed {
		return ErrQuit
	}
	copy(s.frame.Pix, page.Pix)
	s.frame.Palette = page.Palette
	s.frame.Cursor = append(s.frame.Cursor[:0], page.Cursor...)
	if page.Cursor == nil {
		s.frame.Cursor = nil
	}
	s.fresh = true
	return nil
}

// MouseBuffer reports whether a press was buffered since the last call, and
// clears that flag (Mouse_Buffer).
func (s *System) MouseBuffer() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.poll()
	f := s.pressA
	s.pressA = false
	return f
}

// MouseBuffer2 is the same for the second flag (Mouse_Buffer2).
func (s *System) MouseBuffer2() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.poll()
	f := s.pressB
	s.pressB = false
	return f
}

// BufferedPress returns the last buffered press.
func (s *System) BufferedPress() Press {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.poll()
	return s.press
}

// WarpMouse moves the pointer, as Set_Pointer_Position does.
func (s *System) WarpMouse(x, y int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mx, s.my = clamp(x, gfx.ScreenW), clamp(y, gfx.ScreenH)
	s.warp = true
}

// KeyWaiting reports whether a key press is queued.
func (s *System) KeyWaiting() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.poll()
	return len(s.keys) > 0
}

// Mouse returns the pointer position in screen pixels and the buttons held.
func (s *System) Mouse() (x, y, buttons int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.poll()
	return s.mx, s.my, s.buttons
}

// PopKey returns the oldest unread key press.
func (s *System) PopKey() (Key, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.poll()
	if len(s.keys) == 0 {
		return Key{}, false
	}
	k := s.keys[0]
	s.keys = s.keys[1:]
	return k, true
}

// Wheel takes the mouse wheel's whole notches turned since the last call,
// positive up (away from the user); port-only, the original has no wheel.
func (s *System) Wheel() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.poll()
	n := int(s.wheel)
	s.wheel -= float32(n)
	return n
}

// PushWheel turns the wheel n notches, as a test's input.
func (s *System) PushWheel(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.wheel += float32(n)
}

// PeekKey returns the next key press without taking it.
func (s *System) PeekKey() (Key, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.poll()
	if len(s.keys) == 0 {
		return Key{}, false
	}
	return s.keys[0], true
}

// ClearKeys drops unread key presses.
func (s *System) ClearKeys() {
	s.mu.Lock()
	s.keys = s.keys[:0]
	s.mu.Unlock()
}

// Elapsed is the time since the host started, for the engine's timers.
func (s *System) Elapsed() time.Duration {
	if s.virtual {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.poll()
		return s.virtualNow()
	}
	if cooperative {
		s.mu.Lock()
		s.yield()
		s.mu.Unlock()
	}
	return time.Since(s.start)
}

func (s *System) virtualNow() time.Duration { return time.Duration(s.frames) * time.Second / 60 }

// Sleep pauses the game goroutine, returning early with ErrQuit if the window
// is closed meanwhile.
func (s *System) Sleep(d time.Duration) error {
	if s.virtual {
		s.mu.Lock()
		defer s.mu.Unlock()
		deadline := s.virtualNow() + d
		for !s.closed && s.virtualNow() < deadline {
			s.wait()
		}
		if s.closed {
			return ErrQuit
		}
		return nil
	}
	deadline := time.Now().Add(d)
	for {
		if s.Closed() {
			return ErrQuit
		}
		left := time.Until(deadline)
		if left <= 0 {
			return nil
		}
		time.Sleep(min(left, 5*time.Millisecond))
	}
}

// FlipLatest hands a finished page to the host like Flip, but in real time
// it does not wait for the host to take the previous one: that page is
// replaced, so a loop drawing faster than the display shows its latest
// frame. Under virtual time it is Flip.
func (s *System) FlipLatest(page *Screen) error {
	if s.virtual {
		return s.Flip(page)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrQuit
	}
	copy(s.frame.Pix, page.Pix)
	s.frame.Palette = page.Palette
	s.frame.Cursor = append(s.frame.Cursor[:0], page.Cursor...)
	if page.Cursor == nil {
		s.frame.Cursor = nil
	}
	s.fresh = true
	return nil
}

// Virtual reports whether the host runs on virtual time (headless).
func (s *System) Virtual() bool { return s.virtual }

// Closed reports whether the window has been closed.
func (s *System) Closed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

// Options configure the window.
// Click is a scripted mouse click (development). With ToX/ToY set (Drag)
// the pointer moves there over ten frames before the button is released.
type Click struct {
	X, Y, Frame int
	Drag        bool
	ToX, ToY    int
	Right       bool // the right button instead of the left
}

// ScriptedKey is a key press queued at host frame Frame (development).
type ScriptedKey struct {
	Key
	Frame int
}

type Options struct {
	Title string
	// Scale is the initial window size in multiples of 640x480; zero picks the
	// largest 4:3 window that fits the screen.
	Scale int
	// Clicks are scripted left clicks for testing: at host frame F the
	// pointer moves to X,Y and the button is held for four frames.
	Clicks []Click
	// Keys are scripted key presses for testing.
	Keys []ScriptedKey
	// WindowShot saves the last frame as shown in the window (BMP).
	WindowShot string
	// Frames closes the window after that many presented frames when
	// positive, so the host can be exercised without anyone closing it.
	Frames int
	// HideCursor hides the system pointer, for when the game draws its own.
	HideCursor bool
	// Control, if set, is the address of the remote control (control.go,
	// development): screenshots and input over HTTP.
	Control string
	// Shot, if set, receives the last presented frame as a PNG on exit.
	Shot string
	// ShotsAt saves, during one run, the frame a run with Frames: F would
	// leave in Shot, for each F (a PNG path); those past the end get the last.
	ShotsAt map[int]string
	// Headless runs without a window, without touching the system pointer
	// and without reading real input: frames are produced at the display
	// rate and only scripted input (Clicks, WarpMouse) reaches the game.
	Headless bool
	// Watch shows the frames in a window but keeps headless input: only
	// scripted input, the system pointer is never warped.
	Watch bool
	// Lockstep, for headless runs, shares one virtual clock between several
	// hosts; Machine is this host's place in its rotation.
	Lockstep *Lockstep
	Machine  int
}

// bufferPress is Check_Mouse_Buffer: any button down records the position.
func (s *System) bufferPress() {
	s.press = Press{s.mx, s.my, s.buttons & 3}
	s.pressA, s.pressB = true, true
}

// overlayCursor draws the cursor onto the RGBA frame with its top left on the
// pointer, clipped at the screen edge, as Draw_Mouse_Buffer does.
func overlayCursor(rgba []byte, s *Screen, mx, my int) {
	if len(s.Cursor) < CursorSize*CursorSize {
		return
	}
	for y := 0; y < CursorSize && my+y < gfx.ScreenH; y++ {
		for x := 0; x < CursorSize && mx+x < gfx.ScreenW; x++ {
			v := s.Cursor[y*CursorSize+x]
			if v == 0 {
				continue
			}
			c := s.Palette[v]
			o := 4 * ((my+y)*gfx.ScreenW + mx + x)
			rgba[o], rgba[o+1], rgba[o+2], rgba[o+3] = c.R, c.G, c.B, 0xff
		}
	}
}

func clamp(v, n int) int { return max(0, min(v, n-1)) }

// tick does the host's per-frame bookkeeping shared by both loops: take a
// fresh page from the game and apply the scripted input of frame n. It
// returns the pointer position and whether a warp is pending.
func (s *System) tick(n int, clicks []Click, keys []ScriptedKey, shown *Screen) (mx, my int, warp bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.polls = 0
	if s.fresh {
		copy(shown.Pix, s.frame.Pix)
		shown.Palette = s.frame.Palette
		shown.Cursor = append(shown.Cursor[:0], s.frame.Cursor...)
		if s.frame.Cursor == nil {
			shown.Cursor = nil
		}
		s.fresh = false
	}
	for _, k := range keys {
		if n == k.Frame {
			s.addKey(k.Key)
		}
	}
	for _, c := range clicks {
		hold := 4
		if c.Drag {
			hold = 14
		}
		bit := ButtonLeft
		if c.Right {
			bit = ButtonRight
		}
		switch {
		case n == c.Frame:
			s.mx, s.my = clamp(c.X, gfx.ScreenW), clamp(c.Y, gfx.ScreenH)
			s.buttons |= bit
			s.bufferPress()
		case c.Drag && n > c.Frame+2 && n <= c.Frame+12:
			k := n - c.Frame - 2
			s.mx = clamp(c.X+(c.ToX-c.X)*k/10, gfx.ScreenW)
			s.my = clamp(c.Y+(c.ToY-c.Y)*k/10, gfx.ScreenH)
		case n == c.Frame+hold:
			s.buttons &^= bit
		}
	}
	s.controlTick()
	mx, my, warp = s.mx, s.my, s.warp
	s.warp = false
	s.frames++
	return mx, my, warp
}

// Press moves the pointer to (x, y) and buffers a press of buttons there, as
// a click between two polls would (development: scripted input in tests).
func (s *System) Press(x, y, buttons int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mx, s.my = clamp(x, gfx.ScreenW), clamp(y, gfx.ScreenH)
	s.press = Press{s.mx, s.my, buttons & 3}
	s.pressA, s.pressB = true, true
}

// addKey queues a key press as the keyboard interrupt does; s.mu is held.
func (s *System) addKey(k Key) {
	s.keys = append(s.keys, k)
	if k.Ctrl || k.Alt {
		return // the BIOS words differ: 0x84E0, 0x9900 and so on
	}
	// community patch 1.51 steps the speed in New09_Handler_ (0x40a34e) on
	// PageUp and PageDown (0x49E0, 0x51E0), on any screen; the key stays queued.
	if s.animKeysOff {
		return
	}
	switch k.Scancode {
	case 75: // PageUp
		s.animSpeed = min(s.animSpeed+1, 3)
	case 78: // PageDown
		s.animSpeed = max(s.animSpeed-1, -3)
	}
}

// SetAnimSpeedKeys turns the PageUp/PageDown animation speed keys off
// (port-only: the lobby scrolls with them) or on again.
func (s *System) SetAnimSpeedKeys(on bool) {
	s.mu.Lock()
	s.animKeysOff = !on
	s.mu.Unlock()
}

// AnimSpeed is community patch 1.51's animation speed, -3..3: the number of
// ticks Release_Time_ waits less (0x403e71); 0 until PageUp or PageDown.
func (s *System) AnimSpeed() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.animSpeed
}

// PushKey queues a key press (development: scripted input in tests).
func (s *System) PushKey(k Key) {
	s.mu.Lock()
	s.addKey(k)
	s.mu.Unlock()
}

// Quit closes the session as closing the window would: the game's next
// flip or sleep returns ErrQuit.
func (s *System) Quit() {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
}

// shotSaver writes Options.ShotsAt as the frames go by.
type shotSaver struct{ at map[int]string }

func newShotSaver(at map[int]string) *shotSaver {
	m := make(map[int]string, len(at))
	for k, v := range at {
		m[k] = v
	}
	return &shotSaver{m}
}

// after saves the shot due once n frames have been shown.
func (s *shotSaver) after(n int, shown *Screen) error {
	p, ok := s.at[n]
	if !ok {
		return nil
	}
	delete(s.at, n)
	return shown.SavePNG(p)
}

// rest saves the shots the run ended before, with the last frame.
func (s *shotSaver) rest(shown *Screen) error {
	for n, p := range s.at {
		if err := shown.SavePNG(p); err != nil {
			return err
		}
		delete(s.at, n)
	}
	return nil
}
