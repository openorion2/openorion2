// Package smk decodes Smacker (SMK2, SMK4) video: the frames and their
// palettes, as RAD's library hands them to Play_Cinematic_, and the
// audio tracks (raw or Huffman-coded DPCM, 8 or 16 bit, mono or stereo).
package smk

import (
	"encoding/binary"
	"errors"
	"fmt"
	"time"
)

// Video is an open Smacker file.
type Video struct {
	Width, Height int
	Frames        int
	Flags         uint32
	// Period is the time per frame: the header's rate is milliseconds when
	// positive, units of 10 µs when negative, 100 ms when zero.
	Period  time.Duration
	v4      bool
	sizes   []uint32
	types   []byte
	trees   [4]huff16 // mmap, mclr, full, type
	data    []byte
	offsets []int
	buf     []byte // the frame buffer, as large as the largest frame so far

	Audio [7]Track // the header's audio tracks

	Pixels  []byte       // the current frame, Width*Height
	Palette [256][3]byte // 6-bit VGA values
	NewPal  bool         // the last decoded frame changed the palette
	Frame   int          // the next frame Next decodes
}

const (
	treeMMap = iota
	treeMClr
	treeFull
	treeType
)

// Open parses the header and trees of data.
func Open(data []byte) (*Video, error) {
	if len(data) < 0x68 {
		return nil, errors.New("smk: short header")
	}
	sig := string(data[:4])
	if sig != "SMK2" && sig != "SMK4" {
		return nil, fmt.Errorf("smk: signature %q", sig)
	}
	le := binary.LittleEndian
	v := &Video{
		Width:  int(le.Uint32(data[4:])),
		Height: int(le.Uint32(data[8:])),
		Frames: int(le.Uint32(data[12:])),
		Flags:  le.Uint32(data[0x14:]),
		v4:     sig == "SMK4",
	}
	switch r := int32(le.Uint32(data[0x10:])); {
	case r > 0:
		v.Period = time.Duration(r) * time.Millisecond
	case r < 0:
		v.Period = time.Duration(-r) * 10 * time.Microsecond
	default:
		v.Period = 100 * time.Millisecond
	}
	for t := range v.Audio {
		r := le.Uint32(data[0x48+4*t:])
		v.Audio[t] = Track{Present: r&0x40000000 != 0, Compressed: r&0x80000000 != 0,
			Bits16: r&0x20000000 != 0, Stereo: r&0x10000000 != 0, Rate: int(r & 0xffffff)}
	}
	treeSize := int(le.Uint32(data[0x34:]))
	allocs := [4]int{int(le.Uint32(data[0x38:])), int(le.Uint32(data[0x3c:])), int(le.Uint32(data[0x40:])), int(le.Uint32(data[0x44:]))}
	n := v.Frames
	if v.Flags&1 != 0 {
		n++
	}
	p := 0x68
	if len(data) < p+n*5+treeSize {
		return nil, errors.New("smk: short tables")
	}
	v.sizes = make([]uint32, n)
	for i := range v.sizes {
		v.sizes[i] = le.Uint32(data[p:])
		p += 4
	}
	v.types = append([]byte(nil), data[p:p+n]...)
	p += n
	br := &bitReader{b: data[p : p+treeSize]}
	for i := range v.trees {
		if err := v.trees[i].build(br, allocs[i]); err != nil {
			return nil, err
		}
	}
	p += treeSize
	v.data = data
	v.offsets = make([]int, n+1)
	for i := 0; i < n; i++ {
		v.offsets[i] = p
		p += int(v.sizes[i] &^ 3)
	}
	v.offsets[n] = p
	if p > len(data) {
		return nil, errors.New("smk: frames past the end")
	}
	v.Pixels = make([]byte, v.Width*v.Height)
	return v, nil
}

// Next decodes the next frame into Pixels and Palette.
func (v *Video) Next() error {
	if v.Frame >= len(v.types) {
		return errors.New("smk: no more frames")
	}
	f := v.Frame
	v.Frame++
	// RAD reads each frame into one buffer; a bit stream that runs past
	// its chunk reads what the earlier frames left there.
	chunk := v.data[v.offsets[f]:v.offsets[f+1]]
	if len(v.buf) < len(chunk) {
		v.buf = append(v.buf, make([]byte, len(chunk)-len(v.buf))...)
	}
	copy(v.buf, chunk)
	b := v.buf[:len(chunk)]
	v.NewPal = false
	for t := range v.Audio {
		v.Audio[t].Samples = v.Audio[t].Samples[:0]
	}
	if v.types[f]&1 != 0 {
		if len(b) == 0 {
			return errors.New("smk: empty palette chunk")
		}
		n := int(b[0]) * 4
		if n > len(b) || n == 0 {
			return errors.New("smk: bad palette chunk")
		}
		v.palette(b[1:n])
		v.NewPal = true
		b = b[n:]
	}
	for t := 0; t < 7; t++ {
		if v.types[f]&(2<<uint(t)) == 0 {
			continue
		}
		if len(b) < 4 {
			return errors.New("smk: short audio chunk")
		}
		n := int(binary.LittleEndian.Uint32(b))
		if n > len(b) || n < 4 {
			return errors.New("smk: bad audio chunk")
		}
		if err := v.Audio[t].decode(b[4:n]); err != nil {
			return fmt.Errorf("smk: frame %d track %d: %w", f, t, err)
		}
		b = b[n:]
	}
	return v.video(b[:cap(b)]) // the rest of the buffer: see above
}

// palette applies a palette chunk: skips keep, copies take old entries,
// others are new colours.
func (v *Video) palette(b []byte) {
	old := v.Palette
	i, k := 0, 0
	next := func() byte {
		if k < len(b) {
			c := b[k]
			k++
			return c
		}
		k++
		return 0
	}
	for i < 256 && k < len(b) {
		t := next()
		switch {
		case t&0x80 != 0:
			i += int(t&0x7f) + 1
		case t&0x40 != 0:
			off := int(next())
			for j := int(t&0x3f) + 1; j > 0 && i < 256; j-- {
				v.Palette[i] = old[off&0xff]
				i++
				off++
			}
		default:
			g, bl := next(), next()
			v.Palette[i] = [3]byte{t & 0x3f, g & 0x3f, bl & 0x3f}
			i++
		}
	}
}

var runSizes = [64]int{
	1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16,
	17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32,
	33, 34, 35, 36, 37, 38, 39, 40, 41, 42, 43, 44, 45, 46, 47, 48,
	49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59, 128, 256, 512, 1024, 2048,
}

// video decodes the 4x4 blocks of a frame.
func (v *Video) video(b []byte) error {
	for i := range v.trees {
		v.trees[i].reset()
	}
	br := &bitReader{b: b}
	bw, bh := v.Width/4, v.Height/4
	total := bw * bh
	w := v.Width
	at := func(blk int) int { return (blk/bw)*4*w + (blk%bw)*4 }
	for blk := 0; blk < total; {
		typ := v.trees[treeType].get(br)
		run := runSizes[(typ>>2)&0x3f]
		switch typ & 3 {
		case 0: // mono
			for ; run > 0 && blk < total; run, blk = run-1, blk+1 {
				clr := v.trees[treeMClr].get(br)
				m := v.trees[treeMMap].get(br)
				hi, lo := byte(clr>>8), byte(clr)
				o := at(blk)
				for y := 0; y < 4; y++ {
					for x := 0; x < 4; x++ {
						if m&1 != 0 {
							v.Pixels[o+x] = hi
						} else {
							v.Pixels[o+x] = lo
						}
						m >>= 1
					}
					o += w
				}
			}
		case 1: // full
			for ; run > 0 && blk < total; run, blk = run-1, blk+1 {
				o := at(blk)
				mode := 0
				if v.v4 {
					if br.bit() != 0 {
						mode = 1
					} else if br.bit() != 0 {
						mode = 2
					}
				}
				switch mode {
				case 0:
					for y := 0; y < 4; y++ {
						c := v.trees[treeFull].get(br)
						v.Pixels[o+2], v.Pixels[o+3] = byte(c), byte(c>>8)
						c = v.trees[treeFull].get(br)
						v.Pixels[o], v.Pixels[o+1] = byte(c), byte(c>>8)
						o += w
					}
				case 1: // doubled pixels
					for y := 0; y < 2; y++ {
						c := v.trees[treeFull].get(br)
						p0, p1 := byte(c), byte(c>>8)
						for r := 0; r < 2; r++ {
							v.Pixels[o], v.Pixels[o+1], v.Pixels[o+2], v.Pixels[o+3] = p0, p0, p1, p1
							o += w
						}
					}
				case 2: // doubled lines
					for y := 0; y < 2; y++ {
						c := v.trees[treeFull].get(br)
						d := v.trees[treeFull].get(br)
						for r := 0; r < 2; r++ {
							v.Pixels[o], v.Pixels[o+1] = byte(d), byte(d>>8)
							v.Pixels[o+2], v.Pixels[o+3] = byte(c), byte(c>>8)
							o += w
						}
					}
				}
			}
		case 2: // void: the previous frame stays
			blk += run
		case 3: // solid
			c := byte(typ >> 8)
			for ; run > 0 && blk < total; run, blk = run-1, blk+1 {
				o := at(blk)
				for y := 0; y < 4; y++ {
					for x := 0; x < 4; x++ {
						v.Pixels[o+x] = c
					}
					o += w
				}
			}
		}
	}
	return nil
}

// bitReader reads bits from the low end of each byte up.
type bitReader struct {
	b   []byte
	pos int
}

func (r *bitReader) bit() uint32 {
	i := r.pos >> 3
	if i >= len(r.b) {
		r.pos++
		return 0
	}
	v := uint32(r.b[i]>>(uint(r.pos)&7)) & 1
	r.pos++
	return v
}

func (r *bitReader) bits(n int) uint32 {
	var v uint32
	for i := 0; i < n; i++ {
		v |= r.bit() << uint(i)
	}
	return v
}

// huff8 is an 8-bit Huffman tree: nodes with two children or a leaf.
type huff8 struct {
	nodes []node8
}

type node8 struct {
	leaf        bool
	value       byte
	left, right int
}

func (h *huff8) build(br *bitReader) error {
	h.nodes = h.nodes[:0]
	if br.bit() == 0 {
		h.nodes = append(h.nodes, node8{leaf: true})
		return nil
	}
	if _, err := h.rec(br, 0); err != nil {
		return err
	}
	if br.bit() != 0 {
		return errors.New("smk: tree not terminated")
	}
	return nil
}

func (h *huff8) rec(br *bitReader, depth int) (int, error) {
	if depth > 32 {
		return 0, errors.New("smk: tree too deep")
	}
	i := len(h.nodes)
	h.nodes = append(h.nodes, node8{})
	if br.bit() != 0 {
		l, err := h.rec(br, depth+1)
		if err != nil {
			return 0, err
		}
		r, err := h.rec(br, depth+1)
		if err != nil {
			return 0, err
		}
		h.nodes[i].left, h.nodes[i].right = l, r
	} else {
		h.nodes[i] = node8{leaf: true, value: byte(br.bits(8))}
	}
	return i, nil
}

func (h *huff8) get(br *bitReader) byte {
	i := 0
	for !h.nodes[i].leaf {
		if br.bit() != 0 {
			i = h.nodes[i].right
		} else {
			i = h.nodes[i].left
		}
	}
	return h.nodes[i].value
}

// huff16 is a 16-bit tree built from two 8-bit trees, with three escape
// leaves standing for the last three values decoded.
type huff16 struct {
	empty  bool
	nodes  []node16
	escape [3]uint16
	cache  [3]uint16
}

type node16 struct {
	leaf        bool
	value       uint16
	esc         int // 0 none, 1..3 escape slot + 1
	left, right int
}

func (h *huff16) build(br *bitReader, _ int) error {
	h.nodes = h.nodes[:0]
	if br.bit() == 0 {
		h.empty = true
		return nil
	}
	var lo, hi huff8
	if err := lo.build(br); err != nil {
		return err
	}
	if err := hi.build(br); err != nil {
		return err
	}
	for i := range h.escape {
		h.escape[i] = uint16(br.bits(16))
	}
	if _, err := h.rec(br, &lo, &hi, 0); err != nil {
		return err
	}
	if br.bit() != 0 {
		return errors.New("smk: tree not terminated")
	}
	return nil
}

func (h *huff16) rec(br *bitReader, lo, hi *huff8, depth int) (int, error) {
	if depth > 64 {
		return 0, errors.New("smk: tree too deep")
	}
	i := len(h.nodes)
	h.nodes = append(h.nodes, node16{})
	if br.bit() != 0 {
		l, err := h.rec(br, lo, hi, depth+1)
		if err != nil {
			return 0, err
		}
		r, err := h.rec(br, lo, hi, depth+1)
		if err != nil {
			return 0, err
		}
		h.nodes[i].left, h.nodes[i].right = l, r
		return i, nil
	}
	v := uint16(lo.get(br)) | uint16(hi.get(br))<<8
	n := node16{leaf: true, value: v}
	for k, e := range h.escape {
		if v == e {
			n.esc = k + 1
			break
		}
	}
	h.nodes[i] = n
	return i, nil
}

func (h *huff16) reset() { h.cache = [3]uint16{} }

func (h *huff16) get(br *bitReader) uint16 {
	if h.empty {
		return 0
	}
	i := 0
	for !h.nodes[i].leaf {
		if br.bit() != 0 {
			i = h.nodes[i].right
		} else {
			i = h.nodes[i].left
		}
	}
	n := h.nodes[i]
	v := n.value
	if n.esc != 0 {
		v = h.cache[n.esc-1]
	}
	if h.cache[0] != v {
		h.cache[2], h.cache[1], h.cache[0] = h.cache[1], h.cache[0], v
	}
	return v
}

// Track is one audio track: its format and the samples of the frame Next
// decoded last (unsigned 8-bit, or little-endian signed 16-bit; stereo
// interleaved left, right).
type Track struct {
	Present, Compressed, Bits16, Stereo bool
	Rate                                int
	Samples                             []byte
}

// decode unpacks one frame's chunk of the track.
func (t *Track) decode(b []byte) error {
	if !t.Compressed {
		t.Samples = append(t.Samples[:0], b...)
		return nil
	}
	if len(b) < 4 {
		return errors.New("short compressed chunk")
	}
	size := int(binary.LittleEndian.Uint32(b))
	br := &bitReader{b: b[4:]}
	if br.bit() == 0 {
		t.Samples = t.Samples[:0]
		return nil
	}
	stereo, bits16 := br.bit() != 0, br.bit() != 0
	if stereo != t.Stereo || bits16 != t.Bits16 {
		return errors.New("format differs from the header")
	}
	ch := 1
	if stereo {
		ch = 2
	}
	nt := 1
	if stereo || bits16 {
		nt = 2
	}
	if stereo && bits16 {
		nt = 4
	}
	trees := make([]huff8, nt)
	for i := range trees {
		if err := trees[i].build(br); err != nil {
			return err
		}
	}
	out := make([]byte, 0, size)
	if !bits16 {
		var base [2]byte
		for c := ch - 1; c >= 0; c-- {
			base[c] = byte(br.bits(8))
		}
		for c := 0; c < ch; c++ {
			out = append(out, base[c])
		}
		for len(out) < size {
			for c := 0; c < ch && len(out) < size; c++ {
				base[c] += trees[c].get(br)
				out = append(out, base[c])
			}
		}
	} else {
		var base [2]int16
		for c := ch - 1; c >= 0; c-- {
			hi := br.bits(8)
			lo := br.bits(8)
			base[c] = int16(uint16(hi<<8 | lo))
		}
		for c := 0; c < ch; c++ {
			out = append(out, byte(base[c]), byte(uint16(base[c])>>8))
		}
		for len(out) < size {
			for c := 0; c < ch && len(out) < size; c++ {
				lo := uint16(trees[2*c].get(br))
				hi := uint16(trees[2*c+1].get(br))
				base[c] += int16(lo | hi<<8)
				out = append(out, byte(base[c]), byte(uint16(base[c])>>8))
			}
		}
	}
	t.Samples = out
	return nil
}
