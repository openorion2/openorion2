package engine

import (
	"encoding/binary"
	"path/filepath"
	"testing"

	"github.com/openorion2/openorion2/internal/gfx"
	"github.com/openorion2/openorion2/internal/lbx"
)

// buildPicture assembles a picture: header, offsets, optional palette chunk
// {start, colours as R,G,B triples}, frames.
func buildPicture(t *testing.T, w, h int, mask byte, palStart int, pal [][3]byte, frames ...[]byte) *gfx.Picture {
	t.Helper()
	le := binary.LittleEndian
	hdr := make([]byte, 12+4*(len(frames)+1))
	le.PutUint16(hdr[0:], uint16(w))
	le.PutUint16(hdr[2:], uint16(h))
	le.PutUint16(hdr[6:], uint16(len(frames)))
	hdr[11] = mask
	if pal != nil {
		hdr[11] |= gfx.MaskPalette
		ch := make([]byte, 4+4*len(pal))
		le.PutUint16(ch[0:], uint16(palStart))
		le.PutUint16(ch[2:], uint16(len(pal)))
		for i, c := range pal {
			copy(ch[4+4*i+1:], c[:])
		}
		hdr = append(hdr, ch...)
	}
	data := hdr
	for i, f := range frames {
		if len(data)&1 != 0 {
			data = append(data, 0)
		}
		le.PutUint32(data[12+4*i:], uint32(len(data)))
		data = append(data, f...)
	}
	le.PutUint32(data[12+4*len(frames):], uint32(len(data)))
	p, err := gfx.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func words(ws ...int) []byte {
	b := make([]byte, 0, 2*len(ws))
	for _, w := range ws {
		b = binary.LittleEndian.AppendUint16(b, uint16(w))
	}
	return b
}

func TestReducePaletteSplit(t *testing.T) {
	var r reducer
	list := make([][4]byte, 512)
	for i, red := range []byte{0, 12, 2, 10} {
		list[i] = [4]byte{0, red, 0, 0}
	}
	if err := r.reducePalette(list, 0, 4, 2); err != nil {
		t.Fatal(err)
	}
	// Sorted 0,2,10,12; weights 2,8,8,2; half is 10, reached after two points.
	if list[0] != [4]byte{1, 1, 0, 0} || list[1] != [4]byte{1, 11, 0, 0} {
		t.Errorf("got %v %v", list[0], list[1])
	}
}

func TestReducePaletteFill(t *testing.T) {
	var r reducer
	list := make([][4]byte, 512)
	list[0], list[1] = [4]byte{0, 5, 6, 7}, [4]byte{0, 5, 6, 7}
	if err := r.reducePalette(list, 0, 2, 3); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if list[i] != [4]byte{1, 5, 6, 7} {
			t.Errorf("list[%d] = %v", i, list[i])
		}
	}
}

func TestLongestAxisTies(t *testing.T) {
	var r reducer
	r.cubes[0] = cube{rmax: 4, gmax: 4, bmax: 4}
	r.cubes[1] = cube{rmax: 3, gmax: 4, bmax: 4}
	r.cubes[2] = cube{rmax: 3, gmax: 2, bmax: 4}
	for i, want := range []int{0, 1, 2} {
		if got := r.findLongestAxis(i); got != want {
			t.Errorf("cube %d: axis %d, want %d", i, got, want)
		}
	}
}

func TestReducedGroups(t *testing.T) {
	e := &Engine{V: NewVideo()}
	frame := words(0, 0, 0, 1)
	a := buildPicture(t, 1, 1, 0, 0xc0, [][3]byte{{1, 2, 3}, {4, 5, 6}}, frame)
	b := buildPicture(t, 1, 1, 0, 0xc0, [][3]byte{{7, 8, 9}}, frame)
	e.ClearReducedPaletteGroups()
	for _, p := range []*gfx.Picture{a, b} {
		if err := e.AddReducedPaletteGroup(p); err != nil {
			t.Fatal(err)
		}
	}
	if err := e.CompressReducedPaletteGroups(0xc0, 4); err != nil {
		t.Fatal(err)
	}
	// Three colours fit in four: copied as they are, the fourth is stale zero.
	want := []Color{{1, 1, 2, 3}, {1, 4, 5, 6}, {1, 7, 8, 9}, {1, 0, 0, 0}}
	for i, c := range want {
		if e.V.Palette[0xc0+i] != c {
			t.Errorf("palette[%#x] = %v, want %v", 0xc0+i, e.V.Palette[0xc0+i], c)
		}
	}
	if err := e.AddReducedPaletteGroup(buildPicture(t, 1, 1, 0, 0, nil, frame)); err == nil {
		t.Error("picture without palette accepted")
	}
}

func testPaletteEngine() *Engine {
	e := &Engine{V: NewVideo()}
	for i := range e.V.Palette {
		e.V.Palette[i] = Color{0, 40, 0, 40} // far from everything tested
	}
	e.V.Palette[1] = Color{0, 63, 63, 63}
	e.V.Palette[2] = Color{0, 32, 32, 32}
	e.V.Palette[3] = Color{0, 6, 6, 6}
	return e
}

func TestPictureRemapColors(t *testing.T) {
	e := testPaletteEngine()
	p := buildPicture(t, 1, 1, 0, 0xc0, [][3]byte{{60, 61, 62}, {30, 33, 31}}, words(0, 0, 0, 1))
	if err := e.CalculatePictureRemapColors(p); err != nil {
		t.Fatal(err)
	}
	r := e.PictureRemap()
	if r[0xc0] != 1 || r[0xc1] != 2 || r[0x10] != 0x10 || r[0xc2] != 0xc2 {
		t.Errorf("remap c0=%d c1=%d 10=%d c2=%d", r[0xc0], r[0xc1], r[0x10], r[0xc2])
	}
}

func TestRemapGrayPalettes(t *testing.T) {
	e := testPaletteEngine()
	e.CalculateRemapGrayPalettes(RemapGrayPercentages)
	g := e.GrayRemap()
	// 63 at 90 % is 56, still white; at 50 % 31, the mid gray; at 90 % 6.
	if g[0][1] != 1 || g[4][1] != 2 || g[8][1] != 3 {
		t.Errorf("levels for white: %d %d %d", g[0][1], g[4][1], g[8][1])
	}
}

// maskEngine has picture remap c -> c+0x10 and gray level l: c -> 0xa0+l.
func maskEngine() *Engine {
	e := &Engine{V: NewVideo()}
	s := e.remap()
	for i := range s.picture {
		s.picture[i] = byte(i + 0x10)
	}
	for l := range s.gray {
		for c := range s.gray[l] {
			s.gray[l][c] = byte(0xa0 + l)
		}
	}
	return e
}

// maskSprite is 5x2: row 0 runs (skip 1: 1,2) and (skip 1: 3), row 1 (skip 0: 4,5,6).
func maskSprite(t *testing.T) (*gfx.Picture, *gfx.Picture) {
	f := append(words(0, 0), words(2, 1)...)
	f = append(f, 1, 2)
	f = append(f, words(1, 1)...)
	f = append(f, 3, 0)
	f = append(f, words(0, 1)...)
	f = append(f, words(3, 0)...)
	f = append(f, 4, 5, 6, 0)
	f = append(f, words(0, 1)...)
	pict := buildPicture(t, 5, 2, 0, 0, nil, f, f)
	m := []byte{
		0, 1, 0, 0, 0,
		0, 0, 2, 0, 0,
	}
	mask := buildPicture(t, 5, 2, byte(gfx.KindBitmap), 0, nil, m, m)
	return pict, mask
}

func TestRemapDrawWithMask(t *testing.T) {
	e := maskEngine()
	pict, mask := maskSprite(t)
	if err := e.RemapDrawWithMask(10, 20, pict, mask, nil); err != nil {
		t.Fatal(err)
	}
	at := func(x, y int) byte { return e.V.Current[y*W+x] }
	// Row 0: x 11,12 then 14; the second run reads the mask at its skip (1),
	// so x 14 is darkened by m[1] = 1 although m[4] = 0.
	want := map[[2]int]byte{
		{11, 20}: 0xa0, {12, 20}: 0x12, {13, 20}: 0, {14, 20}: 0xa0,
		{10, 21}: 0x14, {11, 21}: 0x15, {12, 21}: 0xa1,
	}
	for p, c := range want {
		if got := at(p[0], p[1]); got != c {
			t.Errorf("(%d,%d) = %#x, want %#x", p[0], p[1], got, c)
		}
	}
	if pict.CurrentFrame != 1 || mask.CurrentFrame != 1 {
		t.Errorf("frames %d %d, want 1 1", pict.CurrentFrame, mask.CurrentFrame)
	}
}

func TestRemapDrawWithMaskClipped(t *testing.T) {
	e := maskEngine()
	pict, mask := maskSprite(t)
	e.V.SetWindow(12, 21, 100, 100)
	e.V.ClipOn()
	if err := e.RemapDrawWithMask(10, 20, pict, mask, nil); err != nil {
		t.Fatal(err)
	}
	at := func(x, y int) byte { return e.V.Current[y*W+x] }
	// Only row 1 from x 12; rows count from the clipped top, so the mask row
	// is 0: m[0*5+0+2] = 0, not m[1*5+2] = 2.
	want := map[[2]int]byte{{12, 21}: 0x16, {11, 21}: 0, {10, 21}: 0, {14, 20}: 0, {12, 20}: 0}
	for p, c := range want {
		if got := at(p[0], p[1]); got != c {
			t.Errorf("(%d,%d) = %#x, want %#x", p[0], p[1], got, c)
		}
	}
	if pict.CurrentFrame != 0 || mask.CurrentFrame != 0 {
		t.Errorf("clipped draw advanced frames: %d %d", pict.CurrentFrame, mask.CurrentFrame)
	}
	// A header larger than the window decides clipping even for a picture inside.
	e2 := maskEngine()
	e2.V.SetWindow(0, 0, 50, 50)
	e2.V.ClipOn()
	big := buildPicture(t, 288, 154, 0, 0, nil, words(0, 0, 0, 154))
	if err := e2.RemapDrawWithMask(10, 20, pict, mask, big); err != nil {
		t.Fatal(err)
	}
	if pict.CurrentFrame != 0 || e2.V.Current[20*W+11] != 0xa0 {
		t.Errorf("header clip: frame %d pixel %#x", pict.CurrentFrame, e2.V.Current[20*W+11])
	}
}

func TestPlanetPalettesFromGame(t *testing.T) {
	a, err := lbx.Open(filepath.Join("..", "..", "game", "BUFFER0.LBX"))
	if err != nil {
		t.Skip("game files not present")
	}
	e := testPaletteEngine()
	e.ClearReducedPaletteGroups()
	var picts []*gfx.Picture
	for _, n := range []int{0x5c, 0x61, 0x80, 0x8e} {
		data, err := a.Entry(n)
		if err != nil {
			t.Fatal(err)
		}
		p, err := gfx.Parse(data)
		if err != nil {
			t.Fatal(err)
		}
		if err := e.AddReducedPaletteGroup(p); err != nil {
			t.Fatal(err)
		}
		picts = append(picts, p)
	}
	if e.remap().reducedCount <= 0x40 {
		t.Fatalf("only %d colours collected", e.remap().reducedCount)
	}
	if err := e.CompressReducedPaletteGroups(0xc0, 0x40); err != nil {
		t.Fatal(err)
	}
	for i := 0xc0; i < 0x100; i++ {
		if c := e.V.Palette[i]; c.Changed != 1 || c.R > 63 || c.G > 63 || c.B > 63 {
			t.Errorf("palette[%#x] = %v", i, c)
		}
	}
	if err := e.CalculatePictureRemapColors(picts[0]); err != nil {
		t.Fatal(err)
	}
	ch := picts[0].Palette
	for i := 0; i < len(ch.Raw)/4; i++ {
		if c := e.PictureRemap()[ch.Start+i]; c == 0 {
			t.Errorf("colour %d remapped to 0", ch.Start+i)
		}
	}
}
