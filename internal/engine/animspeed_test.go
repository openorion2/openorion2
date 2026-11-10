package engine

import (
	"fmt"
	"testing"

	"github.com/openorion2/openorion2/internal/host"
)

// TestAnimSpeedReleaseTime: community patch 1.51's PageUp and PageDown step
// the animation speed, -3..3, and Release_Time_ waits that many ticks less.
func TestAnimSpeedReleaseTime(t *testing.T) {
	t.Parallel()
	pgUp, pgDn := host.Key{Scancode: 75}, host.Key{Scancode: 78}
	err := host.Run(host.Options{Headless: true}, func(sys *host.System) error {
		e := &Engine{Sys: sys}
		wait := func(n int) int64 {
			e.MarkTime()
			if err := e.ReleaseTime(n); err != nil {
				return -1
			}
			return e.Ticks() - e.mark
		}
		steps := []struct {
			keys  []host.Key
			speed int
			n     int
			ticks int64
		}{
			{nil, 0, 4, 4},
			{[]host.Key{pgUp}, 1, 4, 3},
			{[]host.Key{pgUp, pgUp, pgUp, pgUp}, 3, 4, 1},
			{nil, 3, 2, 0},
			{[]host.Key{{Scancode: 78, Ctrl: true}, {Scancode: 75, Alt: true}}, 3, 4, 1},
			{[]host.Key{pgDn, pgDn, pgDn, pgDn, pgDn, pgDn, pgDn}, -3, 1, 4},
			{[]host.Key{pgUp, pgUp, pgUp}, 0, 2, 2},
		}
		for i, s := range steps {
			for _, k := range s.keys {
				sys.PushKey(k)
			}
			if got := sys.AnimSpeed(); got != s.speed {
				return fmt.Errorf("step %d: speed %d, want %d", i, got, s.speed)
			}
			if got := wait(s.n); got != s.ticks {
				return fmt.Errorf("step %d: Release_Time_(%d) waited %d ticks, want %d", i, s.n, got, s.ticks)
			}
		}
		// The keys stay queued for the game, as the interrupt leaves them.
		n := 0
		for {
			if _, ok := sys.PopKey(); !ok {
				break
			}
			n++
		}
		if n != 17 {
			return fmt.Errorf("%d keys queued, want 17", n)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
