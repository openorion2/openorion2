package le

import (
	"encoding/binary"
	"fmt"
	"slices"
)

// Flat is the image laid out linearly the way the loader maps it, so that an
// offset into it corresponds to an address in the program's own address space.
// That correspondence is the point: an address seen in the debugger can be used
// in a disassembler and the other way round, once the load offset of the object
// is accounted for.
type Flat struct {
	Base uint32 // address of Data[0]
	Data []byte
}

// At returns a slice of the flat image starting at address va.
func (f *Flat) At(va uint32) ([]byte, bool) {
	if va < f.Base || va-f.Base >= uint32(len(f.Data)) {
		return nil, false
	}
	return f.Data[va-f.Base:], true
}

func (f *Flat) put8(va uint32, v byte) bool {
	b, ok := f.At(va)
	if !ok || len(b) < 1 {
		return false
	}
	b[0] = v
	return true
}

func (f *Flat) put16(va uint32, v uint16) bool {
	b, ok := f.At(va)
	if !ok || len(b) < 2 {
		return false
	}
	binary.LittleEndian.PutUint16(b, v)
	return true
}

func (f *Flat) put32(va uint32, v uint32) bool {
	b, ok := f.At(va)
	if !ok || len(b) < 4 {
		return false
	}
	binary.LittleEndian.PutUint32(b, v)
	return true
}

// BuildFlat copies every mapped page to its address. Pages an object declares
// but does not back with file data (BSS) stay zero, as they would at run time.
func (img *Image) BuildFlat() (*Flat, error) {
	if len(img.Objects) == 0 {
		return nil, fmt.Errorf("image has no objects")
	}
	lo := img.Objects[0].Base
	hi := uint32(0)
	for _, o := range img.Objects {
		if o.Base < lo {
			lo = o.Base
		}
		// Round the virtual size up to a page: the loader maps whole pages.
		end := o.Base + roundUp(o.VSize, img.Header.PageSize)
		if end > hi {
			hi = end
		}
	}
	f := &Flat{Base: lo, Data: make([]byte, hi-lo)}

	for i := range img.Pages {
		off, ok := img.pageFileOff(i)
		if !ok {
			continue
		}
		if int(off)+int(img.Header.PageSize) > len(img.Raw) {
			return nil, fmt.Errorf("page %d data at 0x%x runs past end of file", i, off)
		}
		dst, ok := f.At(img.PageVA[i])
		if !ok {
			return nil, fmt.Errorf("page %d maps outside the flat image", i)
		}
		copy(dst[:img.Header.PageSize], img.Raw[off:off+img.Header.PageSize])
	}
	return f, nil
}

func roundUp(v, align uint32) uint32 {
	if r := v % align; r != 0 {
		v += align - r
	}
	return v
}

// FixupStats counts what Relocate did, broken down by source type, so that
// anything left unapplied is visible rather than silently wrong.
type FixupStats struct {
	Records    int
	Applied    int
	BySrcType  map[byte]int
	Skipped    map[string]int
	OutOfRange int
	// Targets counts how often each resolved target address was referenced.
	// Targets that land in an executable object are the addresses reached only
	// through function pointers, so they are the seeds a disassembler cannot
	// discover by following flow from the entry point.
	Targets map[uint32]int
	// Sources lists every patched field: its address and the object its target
	// lies in. The object matters because a loader is free to place objects at
	// unrelated addresses, so each one can carry a different relocation delta.
	Sources []FixupSite
}

// FixupSite is one patched field.
type FixupSite struct {
	VA      uint32
	Obj     int
	SrcType byte
}

func newFixupStats() *FixupStats {
	return &FixupStats{
		BySrcType: map[byte]int{},
		Skipped:   map[string]int{},
		Targets:   map[uint32]int{},
	}
}

// Relocate rewrites every fixup site to hold an absolute address in the flat
// image's own space, that is the target object's base plus the target offset.
//
// The file itself holds only the target offset at each site, because the loader
// is expected to add the object's run-time address. Leaving it that way would
// mean a disassembler resolves almost no cross-references, and the addresses it
// did resolve would be meaningless.
func (img *Image) Relocate(f *Flat) (*FixupStats, error) {
	st := newFixupStats()

	objBase := make(map[int]uint32, len(img.Objects))
	for _, o := range img.Objects {
		objBase[o.Index] = o.Base
	}

	err := img.EachFixup(func(fx Fixup) bool {
		st.Records++
		st.BySrcType[fx.SrcType]++

		if fx.TargetKind() != tgtInternal {
			st.Skipped[targetKindName(fx.TargetKind())]++
			return true
		}
		base, ok := objBase[fx.Obj]
		if !ok {
			st.Skipped["unknown target object"]++
			return true
		}
		value := base + fx.TOff + fx.Additive
		st.Targets[value]++
		st.Sources = append(st.Sources, FixupSite{VA: fx.SrcVA, Obj: fx.Obj, SrcType: fx.SrcType})

		va := fx.SrcVA
		var wrote bool
		switch fx.SrcType {
		case srcByte:
			wrote = f.put8(va, byte(value))
		case srcOff16:
			wrote = f.put16(va, uint16(value))
		case srcOff32:
			wrote = f.put32(va, value)
		case srcSelfRel:
			wrote = f.put32(va, value-(va+4))
		case srcPtr1632:
			// A 16:32 far pointer: write the 32-bit offset and leave the
			// selector alone, a flat image having no meaningful selector value.
			wrote = f.put32(va, value)
			st.Skipped["16:32 selector half"]++
		case srcPtr1616:
			wrote = f.put16(va, uint16(value))
			st.Skipped["16:16 selector half"]++
		case srcSel16:
			st.Skipped["selector-only"]++
			return true
		default:
			st.Skipped[fmt.Sprintf("src type 0x%02x", fx.SrcType)]++
			return true
		}
		if wrote {
			st.Applied++
		} else {
			st.OutOfRange++
		}
		return true
	})
	if err != nil {
		return nil, err
	}
	return st, nil
}

func targetKindName(k byte) string {
	switch k {
	case tgtImpOrd:
		return "import by ordinal"
	case tgtImpName:
		return "import by name"
	case tgtIntEntry:
		return "internal entry table"
	}
	return "internal"
}

// IsExecutableVA reports whether va falls inside an object marked executable.
func (img *Image) IsExecutableVA(va uint32) bool {
	for _, o := range img.Objects {
		if o.Flags&ObjExecutable == 0 {
			continue
		}
		if va >= o.Base && va < o.Base+o.VSize {
			return true
		}
	}
	return false
}

// CodePointers returns the sorted, unique fixup targets that land in an
// executable object. These are the entry points reached only through function
// pointers, jump tables and virtual tables, which is why a flow-following
// disassembler misses them when it starts from the entry point alone.
func (img *Image) CodePointers(st *FixupStats) []uint32 {
	if st == nil {
		return nil
	}
	out := make([]uint32, 0, len(st.Targets))
	for va := range st.Targets {
		if img.IsExecutableVA(va) {
			out = append(out, va)
		}
	}
	slices.Sort(out)
	return out
}

// EntryVA is the address of the image entry point.
func (img *Image) EntryVA() (uint32, error) {
	for _, o := range img.Objects {
		if uint32(o.Index) == img.Header.StartObj {
			return o.Base + img.Header.EIP, nil
		}
	}
	return 0, fmt.Errorf("start object %d not in object table", img.Header.StartObj)
}

// StackTopVA is the initial value of ESP as an address.
func (img *Image) StackTopVA() (uint32, error) {
	for _, o := range img.Objects {
		if uint32(o.Index) == img.Header.StackObj {
			return o.Base + img.Header.ESP, nil
		}
	}
	return 0, fmt.Errorf("stack object %d not in object table", img.Header.StackObj)
}

type reader struct {
	d   []byte
	p   uint32
	end uint32
}

func (r *reader) u8() (byte, bool) {
	if r.p+1 > r.end {
		r.p = r.end
		return 0, false
	}
	v := r.d[r.p]
	r.p++
	return v, true
}

func (r *reader) u16() (uint16, bool) {
	if r.p+2 > r.end {
		r.p = r.end
		return 0, false
	}
	v := binary.LittleEndian.Uint16(r.d[r.p:])
	r.p += 2
	return v, true
}

func (r *reader) i16() (int16, bool) {
	v, ok := r.u16()
	return int16(v), ok
}

func (r *reader) u32() (uint32, bool) {
	if r.p+4 > r.end {
		r.p = r.end
		return 0, false
	}
	v := binary.LittleEndian.Uint32(r.d[r.p:])
	r.p += 4
	return v, true
}
