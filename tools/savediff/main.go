// Command savediff lists the differences between two saves, field by field
// (internal/save's Diff). A gzipped save (a lobby desync report's) is read
// as it is. By default the fields that differ between the machines of a
// network game by design are left out, as the desync check leaves them out
// of its hash (netStateHash, docs/net/hardening.md): what remains of two
// sides of a desync is the desync. -all shows everything.
//
//	go run ./tools/savediff desync-41.GAM desync-42.GAM
package main

import (
	"bytes"
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/openorion2/openorion2/internal/save"
)

func main() {
	all := flag.Bool("all", false, "every difference, the per-machine fields too")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: savediff [-all] a.GAM b.GAM (either may be gzipped)")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 2 {
		flag.Usage()
		os.Exit(2)
	}
	var f [2]*save.File
	for i := range f {
		var err error
		if f[i], err = load(flag.Arg(i)); err != nil {
			fmt.Fprintln(os.Stderr, "savediff:", err)
			os.Exit(1)
		}
		if !*all {
			if err := netNormalize(f[i]); err != nil {
				fmt.Fprintln(os.Stderr, "savediff:", flag.Arg(i), err)
				os.Exit(1)
			}
		}
	}
	diffs, err := save.Diff(f[0], f[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "savediff:", err)
		os.Exit(1)
	}
	for _, d := range diffs {
		fmt.Println(d)
	}
	if len(diffs) > 0 {
		os.Exit(3)
	}
}

// load reads a save, gunzipping it if it is gzipped.
func load(path string) (*save.File, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(b) > 2 && b[0] == 0x1f && b[1] == 0x8b {
		z, err := gzip.NewReader(bytes.NewReader(b))
		if err != nil {
			return nil, fmt.Errorf("%s: %v", path, err)
		}
		if b, err = io.ReadAll(z); err != nil {
			return nil, fmt.Errorf("%s: %v", path, err)
		}
	}
	f, err := save.Parse(b)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", path, err)
	}
	return f, nil
}

// netNormalize zeroes what netStateHash leaves out (keep the two alike):
// the viewer's fields, the settings but the universe's, the stars' owners
// as the viewer sees them, an eliminated player's objective and net id,
// and the nebula, planet and star slots past their counts.
func netNormalize(f *save.File) error {
	zero := func(b []byte) { clear(b) }
	for _, bl := range []string{save.BPlayerNum, save.BDescription, save.BSomethingHappened, save.BMoveableBox,
		save.BMoveableBoxOrder, save.BFleetIconButtons, save.BCurMapScale, save.BCurMapX, save.BCurMapY} {
		zero(f.Block(bl))
	}
	set := f.Block(save.BSettings)
	for i := range set {
		if i != 0x02 && (i < 0xd3 || i > 0xd9) {
			set[i] = 0
		}
	}
	stars := f.U16(save.BNumStars)
	for i := 0; i < countOf(save.BStars); i++ {
		if i > stars {
			zero(f.Record(save.BStars, i))
			continue
		}
		var st save.Star
		if err := f.Decode(save.BStars, i, &st); err != nil {
			return err
		}
		st.Owner = 0
		if err := f.Encode(save.BStars, i, &st); err != nil {
			return err
		}
	}
	for i := max(f.U16(save.BPlanetCount), 0); i < countOf(save.BPlanets); i++ {
		zero(f.Record(save.BPlanets, i))
	}
	if n := int(f.Block(save.BNumNebulas)[0]); n < 4 {
		zero(f.Block(save.BNebula)[5*n:])
	}
	for i := 0; i < countOf(save.BPlayers); i++ {
		var p save.Player
		if err := f.Decode(save.BPlayers, i, &p); err != nil {
			return err
		}
		if p.Eliminated != 0 {
			p.Objective, p.NetID = 0, 0
			if err := f.Encode(save.BPlayers, i, &p); err != nil {
				return err
			}
		}
	}
	return nil
}

func countOf(block string) int {
	for _, b := range save.Layout {
		if b.Name == block {
			return b.Count
		}
	}
	return 0
}
