package host

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestLockstep runs three headless games on one clock. Each logs a line per
// frame into a shared, unlocked slice: the order must be machine by machine
// within a frame, the same in every run, and the race detector must see
// no race (the clock orders the games). Machine 1 ends after two frames;
// the others go on without it.
func TestLockstep(t *testing.T) {
	t.Parallel()
	for run := 0; run < 3; run++ {
		var got []string
		l := NewLockstep(3)
		frames := []int{4, 2, 4}
		var wg sync.WaitGroup
		errs := make([]error, 3)
		for i := range 3 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				errs[i] = Run(Options{Headless: true, Lockstep: l, Machine: i}, func(sys *System) error {
					for k := range frames[i] {
						got = append(got, fmt.Sprintf("%d:%d", i, k))
						if err := sys.Sleep(time.Second / 60); err != nil {
							return err
						}
					}
					return nil
				})
			}()
		}
		wg.Wait()
		for i, err := range errs {
			if err != nil {
				t.Fatalf("machine %d: %v", i, err)
			}
		}
		want := "0:0 1:0 2:0 0:1 1:1 2:1 0:2 2:2 0:3 2:3"
		if s := strings.Join(got, " "); s != want {
			t.Fatalf("run %d: %s, want %s", run, s, want)
		}
	}
}

// TestLockstepLeave: machine 1 never runs and leaves the rotation; the
// others run their frames.
func TestLockstepLeave(t *testing.T) {
	t.Parallel()
	l := NewLockstep(3)
	var got []string
	var wg sync.WaitGroup
	for _, i := range []int{0, 2} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			Run(Options{Headless: true, Lockstep: l, Machine: i}, func(sys *System) error {
				for k := range 2 {
					got = append(got, fmt.Sprintf("%d:%d", i, k))
					if err := sys.Sleep(time.Second / 60); err != nil {
						return err
					}
				}
				return nil
			})
		}()
	}
	l.Leave(1)
	l.Leave(1)
	wg.Wait()
	if s := strings.Join(got, " "); s != "0:0 2:0 0:1 2:1" {
		t.Fatal(s)
	}
}
