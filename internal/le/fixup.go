package le

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"slices"
)

// Fixup source types (low nibble of the SRC byte).
const (
	srcByte     = 0x00
	srcSel16    = 0x02
	srcPtr1616  = 0x03
	srcOff16    = 0x05
	srcPtr1632  = 0x06
	srcOff32    = 0x07
	srcSelfRel  = 0x08
	srcListFlag = 0x20
)

// Fixup target types (low two bits of the FLAGS byte).
const (
	tgtInternal = 0
	tgtImpOrd   = 1
	tgtImpName  = 2
	tgtIntEntry = 3
)

// Fixup is one relocation, already expanded so that a record with a source list
// yields one Fixup per patched address.
type Fixup struct {
	SrcVA    uint32 // address of the field to patch
	SrcType  byte   // low nibble of SRC: field width and kind
	Flags    byte
	Obj      int    // target object, 1-based; 0 if the target is not internal
	TOff     uint32 // offset of the target within that object
	Additive uint32
	// TOffAt is the file offset of the TOff field (0 if none) and TOffSize
	// its width; Shared counts the sources of a list record using it.
	TOffAt   uint32
	TOffSize int
	Shared   int
}

// TargetKind returns the low two bits of FLAGS.
func (f Fixup) TargetKind() byte { return f.Flags & 0x03 }

// EachFixup walks every fixup record in page order and calls fn once per patched
// address. fn returning false stops the walk.
//
// Source offsets come from the fixup page table and are independent of where the
// data pages turned out to be in the file, which is what makes them usable as a
// check on that: see ValidateDataPage.
func (img *Image) EachFixup(fn func(Fixup) bool) error {
	h := &img.Header

	ftab := h.FileOff + h.FPageTab
	if int(ftab)+int(h.MPages+1)*4 > len(img.Raw) {
		return fmt.Errorf("fixup page table out of bounds")
	}
	ft := make([]uint32, h.MPages+1)
	for i := range ft {
		ft[i] = binary.LittleEndian.Uint32(img.Raw[ftab+uint32(i)*4:])
	}

	for i := 0; i < int(h.MPages); i++ {
		start := h.FileOff + h.FRecTab + ft[i]
		end := h.FileOff + h.FRecTab + ft[i+1]
		if start > end || int(end) > len(img.Raw) {
			return fmt.Errorf("fixup records for page %d out of bounds", i)
		}
		r := &reader{d: img.Raw, p: start, end: end}
		pageVA := img.PageVA[i]

		for r.p < r.end {
			src, ok := r.u8()
			if !ok {
				break
			}
			flags, ok := r.u8()
			if !ok {
				break
			}

			var srcOffs []int32
			var cnt int
			if src&srcListFlag != 0 {
				c, _ := r.u8()
				cnt = int(c)
			} else {
				v, _ := r.i16()
				srcOffs = append(srcOffs, int32(v))
			}

			f := Fixup{SrcType: src & 0x0f, Flags: flags}

			switch flags & 0x03 {
			case tgtInternal:
				if flags&0x40 != 0 {
					v, _ := r.u16()
					f.Obj = int(v)
				} else {
					v, _ := r.u8()
					f.Obj = int(v)
				}
				// A selector-only fixup carries no offset.
				if src&0x0f != srcSel16 {
					f.TOffAt = r.p
					if flags&0x10 != 0 {
						f.TOff, _ = r.u32()
						f.TOffSize = 4
					} else {
						v, _ := r.u16()
						f.TOff = uint32(v)
						f.TOffSize = 2
					}
				}
			case tgtImpOrd:
				// DOS/4G images have no import tables, but consume the fields so
				// that parsing stays in step if they ever appear.
				if flags&0x40 != 0 {
					r.u16()
				} else {
					r.u8()
				}
				if flags&0x80 != 0 {
					r.u8()
				} else if flags&0x10 != 0 {
					r.u32()
				} else {
					r.u16()
				}
			case tgtImpName:
				if flags&0x40 != 0 {
					r.u16()
				} else {
					r.u8()
				}
				if flags&0x10 != 0 {
					r.u32()
				} else {
					r.u16()
				}
			case tgtIntEntry:
				r.u16()
			}

			if flags&0x04 != 0 {
				if flags&0x20 != 0 {
					f.Additive, _ = r.u32()
				} else {
					v, _ := r.u16()
					f.Additive = uint32(v)
				}
			}

			if src&srcListFlag != 0 {
				for range cnt {
					v, _ := r.i16()
					srcOffs = append(srcOffs, int32(v))
				}
			}

			f.Shared = len(srcOffs)
			for _, so := range srcOffs {
				// A negative source offset patches a field that started on the
				// previous page.
				f.SrcVA = uint32(int64(pageVA) + int64(so))
				if !fn(f) {
					return nil
				}
			}
		}
	}
	return nil
}

// ValidateDataPage measures whether the data pages were found in the right place.
//
// Watcom leaves the raw target offset in the file at each 32-bit fixup site --
// the loader is going to overwrite it with the object's run-time address plus
// that offset -- so for a correctly located image the dword at every such site
// equals the record's own TOff. Get the data page offset wrong and the two
// disagree almost everywhere, which turns a silently corrupt extraction into a
// number that can be checked.
func (img *Image) ValidateDataPage(f *Flat) (matched, total int) {
	img.EachFixup(func(fx Fixup) bool {
		if fx.SrcType != srcOff32 || fx.TargetKind() != tgtInternal {
			return true
		}
		b, ok := f.At(fx.SrcVA)
		if !ok || len(b) < 4 {
			return true
		}
		total++
		if binary.LittleEndian.Uint32(b) == fx.TOff {
			matched++
		}
		return true
	})
	return matched, total
}

// DeriveDataPage works out where the data pages really start, without trusting
// the header field. It takes fixup records with distinctive target offsets,
// finds every place in the file where that value appears, and turns each hit
// into the data page offset it would imply. The true offset is the one that
// many unrelated records agree on.
func (img *Image) DeriveDataPage(samples int) (dataPage uint32, votes int, err error) {
	type cand struct {
		dp uint32
		n  int
	}
	tally := map[uint32]int{}
	used := 0

	err = img.EachFixup(func(fx Fixup) bool {
		if used >= samples {
			return false
		}
		// Small or repeated values occur all over the file and would only add
		// noise; large ones pin the offset down.
		if fx.SrcType != srcOff32 || fx.TargetKind() != tgtInternal || fx.TOff < 0x1000 {
			return true
		}
		rel, ok := img.pageRelOffset(fx.SrcVA)
		if !ok {
			return true
		}
		used++

		var want [4]byte
		binary.LittleEndian.PutUint32(want[:], fx.TOff)
		for p := 0; ; {
			j := bytes.Index(img.Raw[p:], want[:])
			if j < 0 {
				break
			}
			pos := uint32(p + j)
			// pos = FileOff + dataPage + rel, so dataPage follows.
			if pos >= img.Header.FileOff+rel {
				tally[pos-img.Header.FileOff-rel]++
			}
			p = int(pos) + 1
		}
		return true
	})
	if err != nil {
		return 0, 0, err
	}
	if len(tally) == 0 {
		return 0, 0, fmt.Errorf("no fixup target values found in the file")
	}

	best := make([]cand, 0, len(tally))
	for dp, n := range tally {
		best = append(best, cand{dp, n})
	}
	slices.SortFunc(best, func(a, b cand) int { return b.n - a.n })
	return best[0].dp, best[0].n, nil
}

// pageRelOffset returns the distance from the start of the data pages area to
// the byte at va, following the object page map.
func (img *Image) pageRelOffset(va uint32) (uint32, bool) {
	ps := img.Header.PageSize
	for i := range img.Pages {
		if va >= img.PageVA[i] && va < img.PageVA[i]+ps {
			pg := img.Pages[i]
			if pg.Flags != PageLegal || pg.Number == 0 {
				return 0, false
			}
			return (pg.Number-1)*ps + (va - img.PageVA[i]), true
		}
	}
	return 0, false
}

// SettleDataPage makes sure the image knows where its data pages really are, and
// reports how that was decided.
//
// It belongs here rather than in a command because every consumer needs it: the
// header field is wrong in ORION2.EXE, and an image built from it has every byte
// present at the wrong address, with nothing to announce the problem. Anything
// that parses an image and then reads it must go through this first.
func (img *Image) SettleDataPage() (string, error) {
	flat, err := img.BuildFlat()
	if err != nil {
		return "", err
	}
	matched, total := img.ValidateDataPage(flat)
	if total == 0 {
		return "no 32-bit fixups to check the data page offset against", nil
	}
	if 100*matched >= 90*total {
		return fmt.Sprintf("from the header field, %.1f%% of %d fixup sites hold their own target offset",
			100*float64(matched)/float64(total), total), nil
	}

	fromHeader := img.Header.DataPage
	derived, votes, err := img.DeriveDataPage(64)
	if err != nil {
		return "", fmt.Errorf("data page offset 0x%x fails validation (%d of %d sites) and could not be derived: %w",
			fromHeader, matched, total, err)
	}
	img.Header.DataPage = derived
	if flat, err = img.BuildFlat(); err != nil {
		return "", err
	}
	matched, total = img.ValidateDataPage(flat)
	if 100*matched < 90*total {
		return "", fmt.Errorf("neither the header's data page offset 0x%x nor the derived 0x%x validates (%d of %d sites)",
			fromHeader, derived, matched, total)
	}
	return fmt.Sprintf("derived from file contents on %d agreeing samples; the header said 0x%x, off by %+#x."+
		" %.1f%% of %d fixup sites hold their own target offset",
		votes, fromHeader, int64(fromHeader)-int64(derived),
		100*float64(matched)/float64(total), total), nil
}

// FileOffset returns the file offset of the byte mapped at va.
func (img *Image) FileOffset(va uint32) (uint32, bool) {
	rel, ok := img.pageRelOffset(va)
	if !ok {
		return 0, false
	}
	return img.Header.FileOff + img.Header.DataPage + rel, true
}
