package gfx

import (
	"bytes"
	"encoding/binary"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openorion2/openorion2/internal/lbx"
)

// spriteRun is one (count, skip) pair of a sprite frame: count == 0 moves
// skip rows down, otherwise skip pixels are passed and pix copied.
type spriteRun struct {
	skip int
	pix  []byte
}

// makeSprite builds a kind-0 picture with one frame per entry of frames.
func makeSprite(w, h, top int, frames ...[]spriteRun) *Picture {
	le := binary.LittleEndian
	n := len(frames)
	hdr := headerSize + 4*(n+1)
	if hdr&1 != 0 {
		hdr++
	}
	data := make([]byte, hdr)
	le.PutUint16(data[0:], uint16(w))
	le.PutUint16(data[2:], uint16(h))
	le.PutUint16(data[6:], uint16(n))
	for i, runs := range frames {
		le.PutUint32(data[headerSize+4*i:], uint32(len(data)))
		f := []byte{0, 0, byte(top), byte(top >> 8)}
		for _, r := range runs {
			f = le.AppendUint16(f, uint16(len(r.pix)))
			f = le.AppendUint16(f, uint16(r.skip))
			f = append(f, r.pix...)
			if len(f)&1 != 0 {
				f = append(f, 0)
			}
		}
		data = append(data, f...)
	}
	le.PutUint32(data[headerSize+4*n:], uint32(len(data)))
	p, err := Parse(data)
	if err != nil {
		panic(err)
	}
	return p
}

func TestBlankBitmap(t *testing.T) {
	b := NewBlankBitmap(3, 2)
	if b.Kind() != KindBitmap || b.Frames != 1 || len(b.Pixels()) != 6 || b.offsets[0] != 0x10 {
		t.Fatalf("bad blank bitmap: %+v", b)
	}
	if got := BitmapSize(3, 3); got != 28 {
		t.Errorf("BitmapSize(3,3) = %d, want 28", got)
	}
}

func TestDrawPictureToBitmapFrames(t *testing.T) {
	row := func(r int, x int, pix ...byte) []spriteRun {
		return []spriteRun{{skip: r}, {skip: x, pix: pix}, {skip: 3}}
	}
	// Top 0; each frame moves r rows down (a zero-count pair) then paints.
	src := makeSprite(4, 3, 0, row(0, 1, 1), row(1, 2, 2), row(2, 0, 3, 0))
	tests := []struct {
		mfs, cur int
		want     []byte
	}{
		{0, 0, []byte{0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
		{0, 2, []byte{0, 1, 0, 0, 0, 0, 2, 0, 3, 0, 0, 0}},
		{2, 2, []byte{0, 0, 0, 0, 0, 0, 0, 0, 3, 0, 0, 0}},
		{2, 1, []byte{0, 1, 0, 0, 0, 0, 2, 0, 0, 0, 0, 0}},
	}
	for _, tc := range tests {
		src.data[9] = byte(tc.mfs)
		src.CurrentFrame = tc.cur
		dst := NewBlankBitmap(9, 9)
		dst.DrawPictureToBitmap(src)
		if !bytes.Equal(dst.Pixels(), tc.want) {
			t.Errorf("mfs %d cur %d: got %v, want %v", tc.mfs, tc.cur, dst.Pixels(), tc.want)
		}
	}
	// A zero byte in a run overwrites an earlier frame's pixel.
	over := makeSprite(2, 1, 0, []spriteRun{{pix: []byte{4, 4}}, {skip: 1}}, []spriteRun{{skip: 1, pix: []byte{0}}, {skip: 1}})
	over.CurrentFrame = 1
	dst := NewBlankBitmap(1, 1)
	dst.DrawPictureToBitmap(over)
	if !bytes.Equal(dst.Pixels(), []byte{4, 0}) {
		t.Errorf("zero run byte: got %v", dst.Pixels())
	}
	// The bitmap path copies only the current frame; src == dst comes out blank.
	b := NewBitmap(2, 1, []byte{1, 2})
	dst.DrawPictureToBitmap(b)
	if !bytes.Equal(dst.Pixels(), []byte{1, 2}) {
		t.Errorf("bitmap copy: %v", dst.Pixels())
	}
	dst.DrawPictureToBitmap(dst)
	if !bytes.Equal(dst.Pixels(), []byte{0, 0}) {
		t.Errorf("self copy: %v", dst.Pixels())
	}
}

func TestAddPicture(t *testing.T) {
	src := NewBitmap(2, 2, []byte{1, 0, 3, 4})
	dst := NewBitmap(3, 3, []byte{9, 9, 9, 9, 9, 9, 9, 9, 9})
	if err := dst.AddPicture(-1, 2, src); err != nil {
		t.Fatal(err)
	}
	// Column 1 of src's row 0 at dst (0, 2); row 1 is clipped.
	if want := []byte{9, 9, 9, 9, 9, 9, 9, 9, 9}; !bytes.Equal(dst.Pixels(), want) {
		t.Errorf("got %v, want %v (src pixel is 0)", dst.Pixels(), want)
	}
	dst.AddPicture(2, -1, src) // src (0, 1) lands on dst (2, 0)
	if want := []byte{9, 9, 3, 9, 9, 9, 9, 9, 9}; !bytes.Equal(dst.Pixels(), want) {
		t.Errorf("got %v, want %v", dst.Pixels(), want)
	}
	// Sprites copy zero bytes and clip runs on both sides.
	sp := makeSprite(4, 2, 0, []spriteRun{{pix: []byte{1, 0, 2, 3}}, {skip: 1}, {skip: 1, pix: []byte{5}}, {skip: 1}})
	dst = NewBitmap(2, 2, []byte{7, 7, 7, 7})
	dst.AddPicture(-1, 0, sp)
	if want := []byte{0, 2, 5, 7}; !bytes.Equal(dst.Pixels(), want) {
		t.Errorf("sprite: got %v, want %v", dst.Pixels(), want)
	}
	dst = NewBitmap(2, 2, []byte{7, 7, 7, 7})
	dst.AddPicture(0, -1, sp)
	if want := []byte{7, 5, 7, 7}; !bytes.Equal(dst.Pixels(), want) {
		t.Errorf("sprite skip rows: got %v, want %v", dst.Pixels(), want)
	}
	if err := sp.AddPicture(0, 0, dst); err == nil {
		t.Error("sprite target accepted")
	}
}

func TestOutline(t *testing.T) {
	src := NewBitmap(4, 4, []byte{
		0, 0, 0, 0,
		0, 5, 5, 0,
		0, 0, 0, 0,
		0, 0, 0, 0,
	})
	dst := NewBlankBitmap(1, 1)
	dst.Outline(src, 1, false)
	want := []byte{
		0, 1, 1, 0,
		1, 5, 5, 1,
		0, 1, 1, 0,
		0, 0, 0, 0,
	}
	if !bytes.Equal(dst.Pixels(), want) {
		t.Errorf("got %v, want %v", dst.Pixels(), want)
	}
	src.Outline(src, 1, false)
	if !bytes.Equal(src.Pixels(), want) {
		t.Errorf("in place: got %v", src.Pixels())
	}
	// Glass colours count as empty with ignoreGlass; edges are not outlined.
	g := NewBitmap(3, 3, []byte{5, 0xf0, 0, 0, 0, 0, 0, 0, 0})
	g.Outline(g, 1, true)
	if want := []byte{5, 1, 0, 1, 0, 0, 0, 0, 0}; !bytes.Equal(g.Pixels(), want) {
		t.Errorf("no glass: got %v, want %v", g.Pixels(), want)
	}
	// width*width: a 2x3 bitmap scans only its first two rows.
	tall := NewBitmap(2, 3, []byte{0, 0, 0, 0, 5, 0})
	tall.Outline(tall, 1, false)
	if want := []byte{0, 0, 0, 0, 5, 0}; !bytes.Equal(tall.Pixels(), want) {
		t.Errorf("tall: got %v, want %v", tall.Pixels(), want)
	}
}

func TestMaskOut(t *testing.T) {
	mask := NewBitmap(2, 2, []byte{1, 0, 0, 1})
	tgt := NewBitmap(3, 3, []byte{9, 9, 9, 9, 9, 9, 9, 9, 9})
	tgt.MaskOut(1, 1, mask, false)
	if want := []byte{9, 9, 9, 9, 0, 9, 9, 9, 0}; !bytes.Equal(tgt.Pixels(), want) {
		t.Errorf("got %v, want %v", tgt.Pixels(), want)
	}
	tgt = NewBitmap(3, 3, []byte{9, 9, 9, 9, 9, 9, 9, 9, 9})
	tgt.MaskOut(-1, -1, mask, true)
	if want := []byte{9, 9, 9, 9, 9, 9, 9, 9, 9}; !bytes.Equal(tgt.Pixels(), want) {
		t.Errorf("inverse: got %v, want %v", tgt.Pixels(), want)
	}
	tgt.MaskOut(2, 0, mask, true)
	if want := []byte{9, 9, 9, 9, 9, 0, 9, 9, 9}; !bytes.Equal(tgt.Pixels(), want) {
		t.Errorf("inverse edge: got %v, want %v", tgt.Pixels(), want)
	}
	tgt.MaskOut(2, 1, NewBitmap(2, 2, []byte{0, 1, 0, 1}), true)
	if want := []byte{9, 9, 9, 9, 9, 0, 9, 9, 0}; !bytes.Equal(tgt.Pixels(), want) {
		t.Errorf("inverse right clip: got %v, want %v", tgt.Pixels(), want)
	}
	// Bottom clip quirk: y + mask width decides, so a 1x3 mask at y 1 runs
	// three rows (the one past the target is dropped here).
	tgt = NewBitmap(3, 3, []byte{9, 9, 9, 9, 9, 9, 9, 9, 9})
	tgt.MaskOut(0, 1, NewBitmap(1, 3, []byte{1, 1, 1}), false)
	if want := []byte{9, 9, 9, 0, 9, 9, 0, 9, 9}; !bytes.Equal(tgt.Pixels(), want) {
		t.Errorf("narrow mask: got %v, want %v", tgt.Pixels(), want)
	}
}

func TestReplaceColor(t *testing.T) {
	b := NewBitmap(2, 2, []byte{3, 4, 3, 0})
	b.ReplaceColor(3, 7)
	if want := []byte{7, 4, 7, 0}; !bytes.Equal(b.Pixels(), want) {
		t.Errorf("bitmap: %v", b.Pixels())
	}
	b.ReplaceColor(0x104, 1) // kind 1 compares the low byte
	if want := []byte{7, 1, 7, 0}; !bytes.Equal(b.Pixels(), want) {
		t.Errorf("bitmap low byte: %v", b.Pixels())
	}
	sp := makeSprite(3, 1, 0, []spriteRun{{pix: []byte{3, 4, 3}}, {skip: 1}}, []spriteRun{{skip: 1, pix: []byte{3}}, {skip: 1}})
	sp.ReplaceColor(0x103, 9)
	sp.ReplaceColor(3, 8)
	c := sp.NewCanvas()
	sp.Draw(c, 0)
	if want := []byte{8, 4, 8}; !bytes.Equal(c.Pix, want) {
		t.Errorf("sprite frame 0: %v", c.Pix)
	}
	sp.Draw(c, 1)
	if c.Pix[1] != 8 {
		t.Errorf("sprite frame 1: %v", c.Pix)
	}
}

func TestBackgroundReplaceColor(t *testing.T) {
	le := binary.LittleEndian
	var f []byte
	for bank := 0; bank < 5; bank++ {
		if bank == 0 {
			f = le.AppendUint32(f, 1)
			f = le.AppendUint32(f, 1)
			f = append(f, 3, 3, 4, 3)
		}
		f = le.AppendUint32(f, 0)
		f = le.AppendUint32(f, 0)
	}
	data := make([]byte, 0x14)
	le.PutUint16(data[0:], ScreenW)
	le.PutUint16(data[2:], ScreenH)
	le.PutUint16(data[6:], 1)
	data[11] = byte(KindBackground)
	le.PutUint32(data[12:], 0x14)
	data = append(data, f...)
	le.PutUint32(data[16:], uint32(len(data)))
	p, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	p.ReplaceColor(3, 6)
	fr, _ := p.Frame(0)
	if !bytes.Equal(fr[8:12], []byte{6, 6, 4, 6}) {
		t.Errorf("got %v", fr[8:12])
	}
}

func TestActualSizeSwapTint(t *testing.T) {
	b := NewBitmap(4, 3, []byte{
		0, 0, 0, 0,
		0, 2, 0, 1,
		0, 0, 3, 0,
	})
	if x, y, w, h := b.ActualSize(); x != 1 || y != 1 || w != 3 || h != 2 {
		t.Errorf("ActualSize = %d,%d %dx%d", x, y, w, h)
	}
	if x, y, w, h := NewBlankBitmap(2, 2).ActualSize(); x != 1000 || y != 1000 || w != -999 || h != -999 {
		t.Errorf("blank ActualSize = %d,%d %dx%d", x, y, w, h)
	}
	b.SwapColors(0, 0, 3, 3, []byte{2, 3, 2}, []byte{3, 2, 9})
	if want := []byte{0, 0, 0, 0, 0, 3, 0, 1, 0, 0, 2, 0}; !bytes.Equal(b.Pixels(), want) {
		t.Errorf("swap: %v", b.Pixels())
	}
	b.TintColors(0, 0, 0, 0, true, 0xff)
	if want := []byte{0, 0, 0, 0, 0, 2, 0, 0, 0, 0, 1, 0}; !bytes.Equal(b.Pixels(), want) {
		t.Errorf("tint: %v", b.Pixels())
	}
	b.TintColors(2, 2, 3, 3, false, 5)
	if want := []byte{0, 0, 0, 0, 0, 2, 0, 0, 0, 0, 6, 0}; !bytes.Equal(b.Pixels(), want) {
		t.Errorf("tint rect: %v", b.Pixels())
	}
}

// TestGameDrawPictureToBitmap checks the sprite renderer against Draw on
// every single-frame sprite of the game files.
func TestGameDrawPictureToBitmap(t *testing.T) {
	files, _ := filepath.Glob(filepath.Join("..", "..", "game", "*"))
	var n int
	for _, f := range files {
		if !strings.EqualFold(filepath.Ext(f), ".lbx") {
			continue
		}
		a, err := lbx.Open(f)
		if errors.Is(err, lbx.ErrSmacker) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		for i := 0; i < a.Len(); i++ {
			e, _ := a.Entry(i)
			p, err := Parse(e)
			if err != nil || p.Kind() != KindSprite || p.Frames != 1 || p.Width*p.Height == 0 {
				continue
			}
			c := p.NewCanvas()
			if p.Draw(c, 0) != nil {
				continue
			}
			p.CurrentFrame = 0
			dst := NewBlankBitmap(1, 1)
			dst.DrawPictureToBitmap(p)
			if !bytes.Equal(dst.Pixels(), c.Pix) {
				t.Errorf("%s #%d: Draw_Picture_To_Bitmap differs from Draw", filepath.Base(f), i)
			}
			n++
		}
	}
	if n == 0 {
		t.Skip("game files not present")
	}
	t.Logf("%d sprites", n)
}
