package engine

import (
	"fmt"
	"sort"
	"testing"
)

type pt struct{ x, y int }

// pixels lists the non-zero pixels of off, in row-major order.
func pixels(v *Video) []pt {
	var ps []pt
	for o, c := range v.Off {
		if c != 0 {
			ps = append(ps, pt{o % W, o / W})
		}
	}
	return ps
}

// runs expands {y, x1, x2} row runs into a sorted pixel list.
func runs(rs ...[3]int) []pt {
	var ps []pt
	for _, r := range rs {
		lo, hi := min(r[1], r[2]), max(r[1], r[2])
		for x := lo; x <= hi; x++ {
			ps = append(ps, pt{x, r[0]})
		}
	}
	sortPts(ps)
	return ps
}

// cols expands {x, y1, y2} column runs into a sorted pixel list.
func cols(cs ...[3]int) []pt {
	var ps []pt
	for _, c := range cs {
		for y := c[1]; y <= c[2]; y++ {
			ps = append(ps, pt{c[0], y})
		}
	}
	sortPts(ps)
	return ps
}

func sortPts(ps []pt) {
	sort.Slice(ps, func(i, j int) bool {
		if ps[i].y != ps[j].y {
			return ps[i].y < ps[j].y
		}
		return ps[i].x < ps[j].x
	})
}

func eqPts(t *testing.T, name string, got, want []pt) {
	t.Helper()
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("%s:\n got  %v\n want %v", name, got, want)
	}
}

func TestLineShapes(t *testing.T) {
	cases := []struct {
		name           string
		x1, y1, x2, y2 int
		want           []pt
	}{
		{"point", 4, 4, 4, 4, runs([3]int{4, 4, 4})},
		{"horizontal", 2, 5, 6, 5, runs([3]int{5, 2, 6})},
		{"horizontal reversed", 6, 5, 2, 5, runs([3]int{5, 2, 6})},
		{"vertical", 3, 1, 3, 4, cols([3]int{3, 1, 4})},
		{"vertical reversed", 3, 4, 3, 1, cols([3]int{3, 1, 4})},
		{"45 down-right", 0, 0, 3, 3, []pt{{0, 0}, {1, 1}, {2, 2}, {3, 3}}},
		{"45 down-left", 3, 0, 0, 3, []pt{{3, 0}, {2, 1}, {1, 2}, {0, 3}}},
		{"45 up-right", 0, 3, 3, 0, []pt{{3, 0}, {2, 1}, {1, 2}, {0, 3}}},
		// x-major: whole 2, adj_up 2, runs 2,2,2.
		{"shallow 5x2", 0, 0, 5, 2, runs([3]int{0, 0, 1}, [3]int{1, 2, 3}, [3]int{2, 4, 5})},
		// whole 3 odd, adj_up 0: runs 2,3,2.
		{"shallow 6x2", 0, 0, 6, 2, runs([3]int{0, 0, 1}, [3]int{1, 2, 4}, [3]int{2, 5, 6})},
		// whole 2 even, adj_up 0: initial drops to 1, runs 1,2,2.
		{"shallow 4x2", 0, 0, 4, 2, runs([3]int{0, 0, 0}, [3]int{1, 1, 2}, [3]int{2, 3, 4})},
		// whole 3, err -2 then 0 then 2: runs 2,3,4,2.
		{"shallow 10x3", 0, 0, 10, 3, runs([3]int{0, 0, 1}, [3]int{1, 2, 4}, [3]int{2, 5, 8}, [3]int{3, 9, 10})},
		{"shallow 10x3 left", 10, 0, 0, 3, runs([3]int{0, 9, 10}, [3]int{1, 6, 8}, [3]int{2, 2, 5}, [3]int{3, 0, 1})},
		{"shallow 10x3 up-right", 0, 3, 10, 0, runs([3]int{0, 9, 10}, [3]int{1, 6, 8}, [3]int{2, 2, 5}, [3]int{3, 0, 1})},
		{"shallow 10x3 up-left", 10, 3, 0, 0, runs([3]int{0, 0, 1}, [3]int{1, 2, 4}, [3]int{2, 5, 8}, [3]int{3, 9, 10})},
		{"steep 3x10", 0, 0, 3, 10, cols([3]int{0, 0, 1}, [3]int{1, 2, 4}, [3]int{2, 5, 8}, [3]int{3, 9, 10})},
		{"steep 3x10 left", 3, 0, 0, 10, cols([3]int{3, 0, 1}, [3]int{2, 2, 4}, [3]int{1, 5, 8}, [3]int{0, 9, 10})},
		{"steep 3x10 up-right", 0, 10, 3, 0, cols([3]int{3, 0, 1}, [3]int{2, 2, 4}, [3]int{1, 5, 8}, [3]int{0, 9, 10})},
		{"steep 3x10 up-left", 3, 10, 0, 0, cols([3]int{0, 0, 1}, [3]int{1, 2, 4}, [3]int{2, 5, 8}, [3]int{3, 9, 10})},
	}
	for _, c := range cases {
		v := NewVideo()
		v.Line(c.x1, c.y1, c.x2, c.y2, 7)
		eqPts(t, c.name, pixels(v), c.want)
	}
}

func TestLineClip(t *testing.T) {
	cases := []struct {
		name           string
		x1, y1, x2, y2 int
		want           []pt
	}{
		{"left", 0, 15, 15, 15, runs([3]int{15, 10, 15})},
		{"right", 15, 12, 30, 12, runs([3]int{12, 15, 20})},
		{"top", 12, 0, 12, 15, cols([3]int{12, 10, 15})},
		{"bottom", 12, 15, 12, 40, cols([3]int{12, 15, 20})},
		{"diagonal both", 5, 5, 25, 25, []pt{{10, 10}, {11, 11}, {12, 12}, {13, 13}, {14, 14}, {15, 15}, {16, 16}, {17, 17}, {18, 18}, {19, 19}, {20, 20}}},
		// x clip gives (10,5)-(20,10), y clip then leaves one point.
		{"slope to corner", 0, 0, 30, 15, []pt{{20, 10}}},
		// Clipped to (20,10)-(10,15): x-major whole 2, runs 1,2,2,2,2,2.
		{"slope negative", 0, 20, 30, 5, runs([3]int{10, 20, 20}, [3]int{11, 18, 19}, [3]int{12, 16, 17}, [3]int{13, 14, 15}, [3]int{14, 12, 13}, [3]int{15, 10, 11})},
		{"edge inclusive", 10, 5, 10, 10, []pt{{10, 10}}},
		{"reject left", 0, 0, 5, 5, nil},
		{"reject above", 12, 5, 18, 5, nil},
		{"reject vertical", 5, 0, 5, 30, nil},
		{"reject below", 0, 30, 30, 25, nil},
		{"reject right", 25, 0, 30, 30, nil},
	}
	for _, c := range cases {
		v := NewVideo()
		v.SetWindow(10, 10, 20, 20)
		v.ClipOn()
		v.Line(c.x1, c.y1, c.x2, c.y2, 7)
		eqPts(t, c.name, pixels(v), c.want)
	}
}

func TestLineMarking(t *testing.T) {
	v := NewVideo()
	v.Line(0, 0, 7, 1, 1)
	if v.scan[0].start != -1 || v.scan[1].start != -1 {
		t.Fatal("full refresh marked the scan list")
	}

	v = NewVideo()
	v.SetRefreshDifferential()
	v.Line(2, 5, 9, 5, 1)
	// Initial run 4 (x0..3) marks 0..4, final run (x4..7) marks 4..8.
	v.Line(0, 10, 7, 11, 1)
	// Vertical: dots per row.
	v.Line(13, 20, 13, 22, 1)
	if v.scan[5] != (span{0, 2}) || v.scan[10] != (span{0, 1}) || v.scan[11] != (span{1, 2}) {
		t.Errorf("scan %v %v %v", v.scan[5], v.scan[10], v.scan[11])
	}
	for y := 20; y <= 22; y++ {
		if v.scan[y] != (span{3, 3}) {
			t.Errorf("row %d: %v", y, v.scan[y])
		}
	}
	if v.stencil[5].start != -1 {
		t.Error("differential marked the stencil list")
	}

	v = NewVideo()
	v.SetRefreshStencil()
	v.Line(3, 0, 0, 3, 1) // 45 down-left
	for y, want := range []span{{0, 0}, {0, 0}, {0, 0}, {0, 0}} {
		if v.scan[y] != want || v.stencil[y] != want {
			t.Errorf("row %d: %v %v", y, v.scan[y], v.stencil[y])
		}
	}
	v.Line(8, 30, 0, 32, 1) // x-major leftward, initial 2 at x8..7
	if v.scan[30] != (span{1, 2}) || v.stencil[30] != (span{1, 2}) {
		t.Errorf("row 30: %v %v", v.scan[30], v.stencil[30])
	}
}

func TestAddDot(t *testing.T) {
	v := NewVideo()
	v.SetRefreshStencil()
	v.AddDot(-1, 3)
	if v.scan[3] != (span{-1, -1}) {
		t.Errorf("x=-1: %v", v.scan[3])
	}
	v.AddDot(9, 4)
	v.AddDot(5, 4)
	v.AddDot(25, 4)
	if v.scan[4] != (span{1, 6}) || v.stencil[4] != (span{1, 6}) {
		t.Errorf("row 4: %v %v", v.scan[4], v.stencil[4])
	}
	// A reversed span only has its start lowered.
	v.scan[6] = span{5, 3}
	v.AddDot(8, 6)
	if v.scan[6] != (span{2, 3}) {
		t.Errorf("row 6: %v", v.scan[6])
	}
	v.AddDot(0, -1)
	v.AddDot(0, H)
}

func TestMultiColoredLine(t *testing.T) {
	colors := [8]byte{10, 11, 12, 13, 14, 15, 16, 17}
	v := NewVideo()
	v.MultiColoredLine(0, 0, 9, 0, colors, 6)
	for x := 0; x <= 9; x++ {
		if got, want := v.Off[x], colors[(6+x)&7]; got != want {
			t.Errorf("x=%d: %d want %d", x, got, want)
		}
	}
	// Drawn from the top end: (0,10) gets rot 0.
	v = NewVideo()
	v.MultiColoredLine(3, 0, 0, 10, colors, 0)
	order := cols([3]int{3, 0, 1}, [3]int{2, 2, 4}, [3]int{1, 5, 8}, [3]int{0, 9, 10})
	for k, p := range order {
		if got := v.Off[p.y*W+p.x]; got != colors[k&7] {
			t.Errorf("pixel %d %v: %d", k, p, got)
		}
	}
}

func TestDirectionalMultiColoredLine(t *testing.T) {
	colors := [8]byte{10, 11, 12, 13, 14, 15, 16, 17}
	v := NewVideo()
	v.DirectionalMultiColoredLine(0, 0, 3, 0, colors, 2) // forward, rot 5
	for x, want := range []byte{15, 16, 17, 10} {
		if v.Off[x] != want {
			t.Errorf("forward x=%d: %d want %d", x, v.Off[x], want)
		}
	}
	v = NewVideo()
	v.DirectionalMultiColoredLine(3, 0, 0, 0, colors, 2) // reversed, rot 2
	for x, want := range []byte{12, 13, 14, 15} {
		if v.Off[x] != want {
			t.Errorf("reverse x=%d: %d want %d", x, v.Off[x], want)
		}
	}
	v = NewVideo()
	v.DirectionalMultiColoredLine(0, 0, 0, 2, colors, 9) // forward, rev clamps to 0
	for y, want := range []byte{10, 11, 12} {
		if v.Off[y*W] != want {
			t.Errorf("vertical y=%d: %d want %d", y, v.Off[y*W], want)
		}
	}
}
