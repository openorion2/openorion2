//go:build aitrain

// Command aitrain plays games of the AI against itself without screens and
// writes what each empire saw and decided every turn, as int32 tables, to
// train a network to play (docs/aitrain.md).
//
//	go run ./tools/aitrain -game game -games 100 -seed 1 -o data/1
package main

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/openorion2/openorion2/internal/aiplay"
	"github.com/openorion2/openorion2/internal/game"
	"github.com/openorion2/openorion2/internal/host"
)

// table is one int32 file of fixed-width rows.
type table struct {
	f     *os.File
	w     *bufio.Writer
	width int
	buf   []byte
}

func (t *table) row(r []int) {
	if t.width == 0 {
		t.width = len(r)
	}
	if len(r) != t.width {
		panic(fmt.Sprintf("%s: a row of %d, not %d", t.f.Name(), len(r), t.width))
	}
	t.buf = t.buf[:0]
	for _, v := range r {
		t.buf = binary.LittleEndian.AppendUint32(t.buf, uint32(int32(v)))
	}
	t.w.Write(t.buf)
}

func main() {
	dir := flag.String("game", "game", "directory with the original game files")
	games := flag.Int("games", 10, "games to play")
	turns := flag.Int("turns", 400, "at most this many turns a game")
	seed := flag.Uint64("seed", 1, "game i uses seed+i")
	sample := flag.Float64("sample", 1, "keep an empire's turn with this probability")
	out := flag.String("o", "aidata", "the output directory")
	policy := flag.String("policy", "", "a command that plays empire -control instead of the AI (tools/aiplay/serve.py, or tcp:// to rl.py); results go to -o/results.jsonl")
	control := flag.Int("control", 0, "the empire -policy plays")
	baseline := flag.Bool("baseline", false, "write results.jsonl for empire -control played by the AI, to compare with -policy")
	saves := flag.String("saves", "", "a copy of the game directory to put SAVE1..9.GAM into, a save every -every turns of the first game (never game/)")
	every := flag.Int("every", 25, "turns between -saves")
	dagger := flag.Float64("dagger", -1, "with -policy: record empire -control's turns labelled by the AI (DAgger), the AI acting on this share of them")
	flag.Parse()
	check := func(err error) {
		if err != nil {
			fmt.Fprintln(os.Stderr, "aitrain:", err)
			os.Exit(1)
		}
	}
	check(os.MkdirAll(*out, 0o755))
	names := []string{"games", "stars", "planets", "recs", "me", "des", "pl", "star", "pn", "co", "sh"}
	tabs := map[string]*table{}
	for _, n := range names {
		f, err := os.Create(filepath.Join(*out, n+".i32"))
		check(err)
		tabs[n] = &table{f: f, w: bufio.NewWriterSize(f, 1<<20)}
	}
	t0 := time.Now()
	nGames, nRecs := 0, 0
	opt := game.SimOptions{Games: *games, Turns: *turns, Seed: *seed, Sample: *sample,
		Game: func(g *game.SimGame) {
			tabs["games"].row(append(append([]int{int(g.ID)}, g.Setup...), len(g.Stars), len(g.Planets)))
			for _, r := range g.Stars {
				tabs["stars"].row(r)
			}
			for _, r := range g.Planets {
				tabs["planets"].row(r)
			}
			nGames++
		},
		Record: func(r *game.SimRecord) {
			tabs["recs"].row([]int{nGames - 1, r.Turn, r.P, len(r.Co), len(r.Sh)})
			tabs["me"].row(r.Me)
			tabs["des"].row(r.Des)
			tabs["pl"].row(r.Pl)
			for _, x := range r.Star {
				tabs["star"].row(x)
			}
			for _, x := range r.Pn {
				tabs["pn"].row(x)
			}
			for _, x := range r.Co {
				tabs["co"].row(x)
			}
			for _, x := range r.Sh {
				tabs["sh"].row(x)
			}
			nRecs++
		},
		Done: func(id uint64, turns int) {
			fmt.Fprintf(os.Stderr, "game %d: %d turns, %d records so far (%v)\n", id, turns, nRecs, time.Since(t0).Round(time.Millisecond))
		}}
	var cur *game.SimGame
	prevGame := opt.Game
	opt.Game = func(g *game.SimGame) {
		cur = g
		prevGame(g)
	}
	if *policy != "" || *baseline {
		if *dagger < 0 || *policy == "" {
			opt.Record = nil
		} else {
			opt.Mix = *dagger
		}
		opt.Control = *control
		rf, err := os.Create(filepath.Join(*out, "results.jsonl"))
		check(err)
		defer rf.Close()
		enc := json.NewEncoder(rf)
		opt.Result = func(id uint64, r game.SimResult) {
			check(enc.Encode(map[string]any{"game": id, "setup": cur.Setup, "result": r}))
			fmt.Fprintf(os.Stderr, "game %d: rank %d of %d, %d colonies, pop %d, eliminated %v\n", id, r.Rank, r.Players, r.Colonies, r.Pop, r.Eliminated)
		}
	}
	if *policy != "" {
		p, err := aiplay.Start(*policy)
		check(err)
		opt.Policy = func(r *game.SimRecord) *game.SimAction {
			a, err := p.Ask(cur, r)
			check(err)
			return a
		}
		result := opt.Result
		opt.Result = func(id uint64, r game.SimResult) {
			result(id, r)
			check(p.Done(r))
		}
	}
	if *saves != "" {
		slot := 0
		opt.SnapshotEvery = *every
		opt.Snapshot = func(id uint64, turn int, b []byte) {
			if id != *seed || slot >= 9 {
				return
			}
			slot++
			check(os.WriteFile(filepath.Join(*saves, fmt.Sprintf("SAVE%d.GAM", slot)), b, 0o644))
			fmt.Fprintf(os.Stderr, "turn %d: SAVE%d.GAM\n", turn, slot)
		}
	}
	// A new game writes MOX.SET and SAVE10.GAM: into a scratch directory
	// that links the game's archives, never into the player's.
	tmp, err := os.MkdirTemp("", "aitrain")
	check(err)
	defer os.RemoveAll(tmp)
	check(linkGameFiles(*dir, tmp))
	check(host.Run(host.Options{Headless: true}, func(sys *host.System) error {
		return game.Simulate(sys, tmp, &opt)
	}))
	widths := map[string]int{}
	for n, t := range tabs {
		check(t.w.Flush())
		check(t.f.Close())
		widths[n] = t.width
	}
	b, err := json.MarshalIndent(map[string]any{"widths": widths, "games": nGames, "records": nRecs, "seed": *seed}, "", "  ")
	check(err)
	check(os.WriteFile(filepath.Join(*out, "meta.json"), b, 0o644))
}

// linkGameFiles links the archives of dir into tmp.
func linkGameFiles(dir, tmp string) error {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range ents {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".lbx") {
			continue
		}
		abs, err := filepath.Abs(filepath.Join(dir, e.Name()))
		if err != nil {
			return err
		}
		if err := os.Symlink(abs, filepath.Join(tmp, e.Name())); err != nil {
			return err
		}
	}
	return nil
}
