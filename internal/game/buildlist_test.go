package game

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/openorion2/openorion2/internal/host"
)

// TestParseBuildList: words, comments, case, stop, and an unknown word.
func TestParseBuildList(t *testing.T) {
	t.Parallel()
	ops, err := parseBuildList("# head\r\nPrepend spy freighter_fleet # two\n\nbuy50% repeat\nstop\nnonsense\n")
	if err != nil {
		t.Fatal(err)
	}
	want := []blOp{{isMode: true, mode: blPrepend}, {product: -7}, {product: -0xf}, {cmd: "buy50%"}, {product: -10}}
	if !slices.Equal(ops, want) {
		t.Errorf("ops %v, want %v", ops, want)
	}
	if _, err := parseBuildList("append\nresearch_labb\n"); err == nil || err.Error() != "research_labb" {
		t.Errorf("unknown word: %v", err)
	}
	ops, _ = parseBuildList("artificial_planet design6 colony_base")
	if !slices.Equal(ops, []blOp{{product: 48}, {product: -55}, {product: 0xb}}) {
		t.Errorf("ops %v", ops)
	}
}

// TestBuildListDefaults: every default list parses.
func TestBuildListDefaults(t *testing.T) {
	t.Parallel()
	for _, k := range buildListKeys {
		b, err := buildListDefaults.ReadFile("buildlists/" + k.file)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := parseBuildList(string(b)); err != nil {
			t.Errorf("%s: %v", k.file, err)
		}
	}
}

// TestBuildListNames: the building words are the game's English names.
func TestBuildListNames(t *testing.T) {
	t.Parallel()
	g := techGame(t, "start")
	for b := 1; b < 0x31; b++ {
		n, err := g.buildingName(b)
		if err != nil {
			t.Fatal(err)
		}
		if w := buildListBuildings[b]; w != toListWord(n) {
			t.Errorf("building %d %q: word %q", b, n, w)
		}
	}
}

func toListWord(s string) string {
	b := []byte(s)
	for i, c := range b {
		switch {
		case c == ' ':
			b[i] = '_'
		case c >= 'A' && c <= 'Z':
			b[i] = c + 0x20
		}
	}
	return string(b)
}

// TestReadBuildListWritesDefault: a missing list is written to the game's
// directory and read from there afterwards.
func TestReadBuildListWritesDefault(t *testing.T) {
	t.Parallel()
	g := &Game{dir: t.TempDir(), Log: t.Logf}
	text, err := g.readBuildList("BUILD8.CFG")
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(g.dir, buildListDir, "BUILD8.CFG")
	if b, err := os.ReadFile(p); err != nil || string(b) != text {
		t.Fatalf("written %q, %v", b, err)
	}
	// The player's own list, named in lower case, is read as it is.
	h := &Game{dir: t.TempDir(), Log: t.Logf}
	os.Mkdir(filepath.Join(h.dir, buildListDir), 0o755)
	os.WriteFile(filepath.Join(h.dir, buildListDir, "build8.cfg"), []byte("spy"), 0o644)
	if text, _ := h.readBuildList("BUILD8.CFG"); text != "spy" {
		t.Errorf("the player's own list not read: %q", text)
	}
	ents, _ := fs.ReadDir(os.DirFS(h.dir), buildListDir)
	if len(ents) != 1 {
		t.Errorf("the default was written beside the player's list: %v", ents)
	}
}

// blGame is recipe "mid" with the editing state on the player's first
// colony, its queue emptied and its purchase undone.
func blGame(t *testing.T) (*Game, int) {
	t.Helper()
	g := techGame(t, "mid")
	s := &g.state
	ci := -1
	for i := 0; i < int(s.NumColonies); i++ {
		if c := &s.Colonies[i]; int(c.Owner) == int(s.PlayerNum) && c.IsOutpost == 0 {
			ci = i
			break
		}
	}
	if ci == -1 {
		t.Fatal("no colony")
	}
	c := &s.Colonies[ci]
	c.Pending = 0
	c.Queue = [7]int16{-1, -1, -1, -1, -1, -1, -1}
	g.colonyHandle = ci
	g.bq.cur = c.Queue
	return g, ci
}

func blApply(t *testing.T, g *Game, text string) [7]int16 {
	t.Helper()
	ops, err := parseBuildList(text)
	if err != nil {
		t.Fatal(err)
	}
	g.applyBuildList(ops)
	return g.bq.cur
}

// buildable is a building the colony may build and has not queued.
func blBuildable(t *testing.T, g *Game, n int) []int16 {
	t.Helper()
	g.calculateBuildingArray()
	var out []int16
	for _, b := range g.bq.bldg {
		if b > 0 && !g.isInQueue(b) && len(out) < n {
			out = append(out, b)
		}
	}
	if len(out) < n {
		t.Skipf("only %d buildings to build", len(out))
	}
	return out
}

func TestBuildListModes(t *testing.T) {
	t.Parallel()
	g, _ := blGame(t)
	b := blBuildable(t, g, 3)
	w := func(i int) string { return buildListBuildings[b[i]] }
	if q := blApply(t, g, w(0)+" "+w(1)); q != [7]int16{b[0], b[1], -1, -1, -1, -1, -1} {
		t.Fatalf("append: %v", q)
	}
	if q := blApply(t, g, "prepend spy freighter_fleet"); q != [7]int16{-7, -0xf, b[0], b[1], -1, -1, -1} {
		t.Fatalf("prepend: %v", q)
	}
	// Queued buildings again: nothing doubles.
	if q := blApply(t, g, w(1)); q != [7]int16{-7, -0xf, b[0], b[1], -1, -1, -1} {
		t.Fatalf("append again: %v", q)
	}
	// Merge: b1 before b0 in the slots they hold, b2 added and placed.
	if q := blApply(t, g, "merge "+w(2)+" "+w(1)+" "+w(0)); q != [7]int16{-7, -0xf, b[2], b[1], b[0], -1, -1} {
		t.Fatalf("merge: %v", q)
	}
	// Merge clear takes only the listed ones out.
	if q := blApply(t, g, "merge spy "+w(1)+" clear"); q != [7]int16{-0xf, b[2], b[0], -1, -1, -1, -1} {
		t.Fatalf("merge clear: %v", q)
	}
	// Repeat after a product; a full queue takes no more.
	if q := blApply(t, g, "spy repeat housing trade_goods"); q != [7]int16{-0xf, b[2], b[0], -7, -10, -3, -2} {
		t.Fatalf("repeat: %v", q)
	}
	if q := blApply(t, g, "freighter_fleet"); q != [7]int16{-0xf, b[2], b[0], -7, -10, -3, -2} {
		t.Fatalf("full: %v", q)
	}
	if q := blApply(t, g, "clear"); q != [7]int16{-1, -1, -1, -1, -1, -1, -1} {
		t.Fatalf("clear: %v", q)
	}
}

// TestBuildListBuy: buy and unbuy round trip; a bought product stays at
// the head through prepend and clear.
func TestBuildListBuy(t *testing.T) {
	t.Parallel()
	g, ci := blGame(t)
	s := &g.state
	c := &s.Colonies[ci]
	b := blBuildable(t, g, 1)
	pl := &s.Players[c.Owner]
	pl.BC = 30000
	c.Production = 0
	blApply(t, g, buildListBuildings[b[0]]+" buy75%")
	if c.Pending != 0 {
		t.Fatalf("bought below 75%%: pending %d", c.Pending)
	}
	before := pl.BC
	blApply(t, g, "buy")
	if c.Pending == 0 || pl.BC >= before {
		t.Fatalf("not bought: pending %d, BC %d", c.Pending, pl.BC)
	}
	if q := blApply(t, g, "prepend spy clear"); q[0] != b[0] {
		t.Fatalf("the bought product moved: %v", q)
	}
	blApply(t, g, "unbuy")
	if c.Pending != 0 || pl.BC != before {
		t.Fatalf("unbuy: pending %d, BC %d, want %d", c.Pending, pl.BC, before)
	}
	// BUY on a bought product takes it back too.
	g.doExitScreenCleanup()
	g.colonyBuyOutright(ci)
	if err := g.testedColonyBuysOutright(ci); err != nil {
		t.Fatal(err)
	}
	if c.Pending != 0 || pl.BC != before {
		t.Fatalf("BUY again: pending %d, BC %d, want %d", c.Pending, pl.BC, before)
	}
}

// TestBuildListShips: designs and support ships become queued ships;
// cleared ones are freed.
func TestBuildListShips(t *testing.T) {
	t.Parallel()
	g, ci := blGame(t)
	s := &g.state
	n := s.NumShips
	q := blApply(t, g, "design1 colony_ship")
	if q[0] > -100 || q[1] > -100 || q[2] != -1 {
		t.Fatalf("ships: %v", q)
	}
	sh := &s.Ships[-(int(q[0]) + 100)]
	if sh.Status != 6 {
		t.Errorf("queued ship status %d", sh.Status)
	}
	blApply(t, g, "clear")
	if sh.Status != 5 {
		t.Errorf("cleared ship status %d, want 5 (free)", sh.Status)
	}
	if s.NumShips < n {
		t.Errorf("ships %d, were %d", s.NumShips, n)
	}
	_ = ci
}

// TestBuildListKeys: in the colony summary 8 and Shift+8 put spies first in
// one and every queue; in the build screen 0 empties it and Q fills it.
func TestBuildListKeys(t *testing.T) {
	t.Parallel()
	dir := scratchGameDir(t, filepath.Join("..", "..", "game"))
	putRecipeSave(t, "mid", dir, 1)
	step := 0
	var errs []string
	err := host.Run(host.Options{Headless: true, Frames: 60 * 120}, func(sys *host.System) error {
		opt := Options{Load: 1}
		opt.testHook = func(g *Game) {
			g.Log = func(string, ...any) {}
			idle, wait := 0, 0
			var row int
			g.e.InputHook = func() {
				s := &g.state
				idle++
				head := func(ci int) int16 {
					c := &s.Colonies[ci]
					if c.Pending != 0 {
						return c.Queue[1]
					}
					return c.Queue[0]
				}
				switch {
				case step == 0 && !netMainScreenUp(g):
					if idle%20 == 0 {
						sys.PushKey(letterKey('\r'))
					}
				case step == 0:
					for i := range s.Players {
						s.Players[i].BC = 30000
					}
					step, idle = 1, 0
				case step == 1 && g.currentScreen != ScreenColonySummary:
					if idle%30 == 0 {
						sys.PushKey(letterKey('c'))
					}
				case step == 1:
					x1, y1, x2, y2 := g.e.FieldRect(g.cs.listF[1])
					sys.WarpMouse((x1+x2)/2, (y1+y2)/2)
					row, step, wait = int(g.cs.listCol[1]), 2, 0
				case step == 2:
					if wait++; wait == 5 {
						sys.PushKey(letterKey('8'))
					}
					if wait == 30 {
						if head(row) != -7 {
							errs = append(errs, "8: no spy at the head of the pointed colony")
						}
						for _, ci := range g.cs.listCol {
							if ci != -1 && int(ci) != row && head(int(ci)) == -7 {
								errs = append(errs, "8: a spy in another colony")
							}
						}
						sys.PushKey(host.Key{Code: '*', Scancode: 37, Shift: true})
					}
					if wait == 60 {
						var list [250]int16
						g.buildGlobalColonyList(&list)
						for _, ci := range list {
							if ci != -1 && g.colonyCanBuildProduct(int(ci), -7) != 0 && head(int(ci)) != -7 {
								errs = append(errs, "Shift+8: a colony without a spy at the head")
							}
						}
						x1, y1, x2, y2 := g.e.FieldRect(g.cs.prodF[1])
						sys.WarpMouse((x1+x2)/2, (y1+y2)/2)
						step, wait = 3, 0
					}
				case step == 3 && g.currentScreen != ScreenBuildQueue:
					if wait++; wait == 5 {
						x1, y1, x2, y2 := g.e.FieldRect(g.cs.prodF[1])
						sys.Press((x1+x2)/2, (y1+y2)/2, 1)
					}
				case step == 3:
					step, wait = 4, 0
				case step == 4:
					wait++
					switch wait {
					case 5:
						sys.PushKey(letterKey('0'))
					case 30:
						if s.Colonies[row].Pending == 0 && g.bq.cur[0] != -1 {
							errs = append(errs, "0: the queue is not empty")
						}
						sys.PushKey(letterKey('q'))
					case 60:
						n := 0
						for _, v := range g.bq.cur {
							if v > 0 && v < 0x31 {
								n++
							}
						}
						if n == 0 {
							errs = append(errs, "Q: no buildings queued")
						}
						step = 5
						sys.Quit()
					}
				}
			}
		}
		return Run(sys, dir, opt)
	})
	if err != nil && !errors.Is(err, host.ErrQuit) {
		t.Fatal(err)
	}
	if step != 5 {
		t.Fatalf("stopped at step %d", step)
	}
	for _, e := range errs {
		t.Error(e)
	}
}
