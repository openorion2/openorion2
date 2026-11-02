package host

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/openorion2/openorion2/internal/audio"
)

// runHeadless serves the game without a window: no SDL, no system pointer,
// no real keyboard or mouse; only scripted input reaches it. Time is
// virtual: a frame is 1/60 s of game time, taken as soon as the game waits
// (Sleep, Flip), so a run is the same however busy the machine is. The last
// frame can still be saved with Shot.
func runHeadless(opt Options, game func(*System) error) (err error) {
	if opt.WindowShot != "" {
		return fmt.Errorf("a window shot needs a window; drop -headless")
	}
	sys := &System{frame: NewScreen(), start: time.Now(), virtual: true, Audio: audio.NewMixer()}
	if opt.Control != "" {
		if err := sys.serveControl(opt.Control); err != nil {
			return err
		}
	}
	sys.cond = sync.NewCond(&sys.mu)
	ls, me := opt.Lockstep, opt.Machine
	if ls != nil {
		defer ls.leave(me)
		ls.wait(me, 0)
	}
	done := make(chan error, 1)
	go func() { done <- game(sys) }()
	defer func() {
		sys.mu.Lock()
		sys.closed = true
		sys.cond.Broadcast()
		sys.mu.Unlock()
		gerr := <-done
		if err == nil && gerr != nil && !errors.Is(gerr, ErrQuit) {
			err = gerr
		}
	}()
	shown := NewScreen()
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
		if gerr, ok := sys.waitIdle(done); ok {
			done <- gerr
			return nil
		}
		if ls != nil {
			ls.pass(me)
			ls.wait(me, n+1)
		}
		sys.tick(n, opt.Clicks, opt.Keys, shown)
		sys.controlShown(shown)
		sys.mu.Lock()
		sys.cond.Broadcast()
		sys.mu.Unlock()
		if err := shots.after(n+1, shown); err != nil {
			return err
		}
	}
	return nil
}

// waitIdle waits until the game blocks waiting for the next frame (Sleep,
// Flip, or its poll budget spent); it reports the game's result instead
// when the game has returned. No wall-clock limit: frames advance only when
// the game waits, so headless runs are the same under any load.
func (s *System) waitIdle(done chan error) (error, bool) {
	for {
		select {
		case gerr := <-done:
			return gerr, true
		default:
		}
		s.mu.Lock()
		w := s.waiting && s.waitAt == s.frames
		s.mu.Unlock()
		if w {
			return nil, false
		}
		time.Sleep(20 * time.Microsecond)
	}
}

// Lockstep runs several headless hosts on one virtual clock, for network
// games in tests (docs/net/README.md). Frame by frame, machine 0
// first, each game runs until it waits for its next frame while the others
// stay blocked: only one game runs at a time, so what one machine sends
// reaches the others at the same frame in every run. A game must never
// block on anything but the host (Sleep, Flip, input polls), or all stop.
type Lockstep struct {
	mu    sync.Mutex
	cond  *sync.Cond
	live  []bool
	turn  int // the machine that may run
	frame int // the frame it is in
}

// NewLockstep makes a clock for machines 0..n-1; each runs with
// Options{Headless: true, Lockstep: l, Machine: i}.
func NewLockstep(n int) *Lockstep {
	l := &Lockstep{live: make([]bool, n)}
	for i := range l.live {
		l.live[i] = true
	}
	l.cond = sync.NewCond(&l.mu)
	return l
}

// Frame is the frame being played (the clock of all machines).
func (l *Lockstep) Frame() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.frame
}

// wait blocks until it is machine i's turn in frame f.
func (l *Lockstep) wait(i, f int) {
	l.mu.Lock()
	for l.turn != i || l.frame != f {
		l.cond.Wait()
	}
	l.mu.Unlock()
}

// pass hands the turn to the next live machine, or to the first one in the
// next frame.
func (l *Lockstep) pass(i int) {
	l.mu.Lock()
	l.advance(i)
	l.mu.Unlock()
}

// Leave takes machine i out of the rotation without running it: a test
// machine that skips or fails before Run must call it, or the others wait
// for it forever. Leaving twice does no harm.
func (l *Lockstep) Leave(i int) { l.leave(i) }

// leave takes machine i out of the rotation (its run has ended).
func (l *Lockstep) leave(i int) {
	l.mu.Lock()
	l.live[i] = false
	if l.turn == i {
		l.advance(i)
	}
	l.mu.Unlock()
}

func (l *Lockstep) advance(i int) {
	for j := i + 1; j < len(l.live); j++ {
		if l.live[j] {
			l.turn = j
			l.cond.Broadcast()
			return
		}
	}
	l.frame++
	for j := range l.live {
		if l.live[j] {
			l.turn = j
			break
		}
	}
	l.cond.Broadcast()
}
