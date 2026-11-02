// Command monkey plays running games through their remote control (-control,
// docs/control.md) at random, as a fuzzer of network play: clicks on the
// screen's input fields, drags between them, a few keys, and TURN on the
// main screen every so often. Run each game with -netcheck to catch a
// desync the turn it happens.
//
//	go run ./tools/monkey -seed 1 127.0.0.1:7100 127.0.0.1:7001
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand/v2"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"
)

type field struct {
	N      int    `json:"n"`
	Rect   [4]int `json:"rect"`
	Type   int    `json:"type"`
	Hotkey string `json:"hotkey"`
}

// The main screen's GAME button opens the menu with QUIT: never clicked, so a
// machine never leaves the game.
func forbidden(x, y int) bool { return y < 30 && x > 230 && x < 330 }

// turnField is the main screen's TURN, bottom right.
func turnField(f field) bool { return f.Rect[0] > 500 && f.Rect[1] > 400 }

type machine struct {
	addr string
	rng  *rand.Rand
	n    int // actions done
	last string
	same int // polls in a row with the same fields
	try  int // the turner's next way out of a screen
}

// turn is the turner's step: TURN on the main screen, else a way out of
// whatever is up (Enter, Esc, its first field), one per poll.
func (m *machine) turn() error {
	var fs []field
	if err := m.get("/fields", &fs); err != nil {
		return err
	}
	for _, f := range fs {
		if turnField(f) && len(fs) > 15 {
			m.try = 0
			m.logf("TURN")
			return m.post("/click", url.Values{"x": {fmt.Sprint((f.Rect[0] + f.Rect[2]) / 2)}, "y": {fmt.Sprint((f.Rect[1] + f.Rect[3]) / 2)}})
		}
	}
	m.n++
	switch m.try++; m.try % 3 {
	case 1:
		m.logf("enter")
		return m.post("/key", url.Values{"name": {"enter"}})
	case 2:
		m.logf("esc")
		return m.post("/key", url.Values{"name": {"esc"}})
	default:
		for _, f := range fs {
			x, y := (f.Rect[0]+f.Rect[2])/2, (f.Rect[1]+f.Rect[3])/2
			if x >= 0 && x < 640 && y >= 0 && y < 480 && !forbidden(x, y) {
				m.logf("click field %d", f.N)
				return m.post("/click", url.Values{"x": {fmt.Sprint(x)}, "y": {fmt.Sprint(y)}})
			}
		}
		return nil
	}
}

func (m *machine) get(path string, v any) error {
	r, err := http.Get("http://" + m.addr + path)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

func (m *machine) post(path string, q url.Values) error {
	r, err := http.Post("http://"+m.addr+path+"?"+q.Encode(), "", nil)
	if err != nil {
		return err
	}
	r.Body.Close()
	if r.StatusCode != 200 {
		return fmt.Errorf("%s: %s", path, r.Status)
	}
	return nil
}

func (m *machine) point(f field) (int, int) {
	r := f.Rect
	x := r[0] + m.rng.IntN(max(1, r[2]-r[0]+1))
	y := r[1] + m.rng.IntN(max(1, r[3]-r[1]+1))
	return x, y
}

func (m *machine) logf(f string, a ...any) {
	log.Printf("%s #%d: "+f, append([]any{m.addr, m.n}, a...)...)
}

// step does one random action.
func (m *machine) step(turnEvery int) error {
	var fs []field
	if err := m.get("/fields", &fs); err != nil {
		return err
	}
	sig := fmt.Sprint(fs)
	if sig == m.last {
		m.same++
	} else {
		m.last, m.same = sig, 0
	}
	var ok []field
	var turn *field
	for i, f := range fs {
		if forbidden((f.Rect[0]+f.Rect[2])/2, (f.Rect[1]+f.Rect[3])/2) {
			continue
		}
		if f.Rect[2] < f.Rect[0] || f.Rect[3] < f.Rect[1] || f.Rect[0] > 639 || f.Rect[1] > 479 || f.Rect[2] < 0 || f.Rect[3] < 0 {
			continue
		}
		if turnField(f) && len(fs) > 15 {
			turn = &fs[i]
		}
		ok = append(ok, f)
	}
	m.n++
	q := func(kv ...any) url.Values {
		v := url.Values{}
		for i := 0; i+1 < len(kv); i += 2 {
			v.Set(fmt.Sprint(kv[i]), fmt.Sprint(kv[i+1]))
		}
		return v
	}
	switch r := m.rng.IntN(100); {
	case turn != nil && m.n%turnEvery == 0:
		x, y := m.point(*turn)
		m.logf("TURN")
		return m.post("/click", q("x", x, "y", y))
	case len(ok) == 0 || m.same > 40 || r < 6:
		k := []string{"esc", "enter", "enter", "1", "2", "y", "n"}[m.rng.IntN(7)]
		m.logf("key %s", k)
		return m.post("/key", q("name", k))
	case r < 14 && len(ok) > 1:
		a, b := ok[m.rng.IntN(len(ok))], ok[m.rng.IntN(len(ok))]
		x, y := m.point(a)
		tx, ty := m.point(b)
		m.logf("drag %d to %d", a.N, b.N)
		return m.post("/drag", q("x", x, "y", y, "tox", tx, "toy", ty))
	default:
		f := ok[m.rng.IntN(len(ok))]
		x, y := m.point(f)
		button := "left"
		if m.rng.IntN(20) == 0 {
			button = "right"
		}
		m.logf("click field %d type %d at %d,%d %s", f.N, f.Type, x, y, button)
		return m.post("/click", q("x", x, "y", y, "button", button))
	}
}

func main() {
	seed := flag.Uint64("seed", uint64(time.Now().UnixNano()), "the random seed")
	turnEvery := flag.Int("turn", 25, "press TURN on the main screen every this many actions")
	pause := flag.Duration("pause", 150*time.Millisecond, "the pause between actions")
	turner := flag.Bool("turner", false, "only press TURN and close what comes up, no random play")
	flag.Parse()
	if flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: monkey [-seed n] [-turn n] host:port...")
		os.Exit(2)
	}
	log.Printf("seed %d", *seed)
	var wg sync.WaitGroup
	for i, a := range flag.Args() {
		m := &machine{addr: a, rng: rand.New(rand.NewPCG(*seed, uint64(i)))}
		wg.Add(1)
		go func() {
			defer wg.Done()
			fails := 0
			for {
				step := func() error { return m.step(*turnEvery) }
				if *turner {
					step = m.turn
				}
				if err := step(); err != nil {
					if fails++; fails > 20 {
						m.logf("gone: %v", err)
						return
					}
					time.Sleep(time.Second)
					continue
				}
				fails = 0
				time.Sleep(*pause)
			}
		}()
	}
	wg.Wait()
}
