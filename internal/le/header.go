// Package le parses Linear Executable (LE) images as produced by Open Watcom
// and loaded by the Rational Systems DOS/4G extender.
//
// ORION2.EXE is a "bound" executable: a real-mode MZ stub containing DOS/4G
// itself, with the LE image appended. The LE header is therefore not at
// e_lfanew (the stub is an old-style DOS EXE with a relocation table there);
// it has to be located by scanning for the signature.
package le

import (
	"encoding/binary"
	"fmt"
)

// Signature offsets within the LE header. Field order follows exe386.h from the
// OS/2 toolkit. Note the four fields dirtab/dircnt/fpagetab/frectab, which are
// easy to omit and which shift every following offset if you do.
const (
	offLevel       = 0x04
	offCPU         = 0x08 // word
	offOS          = 0x0a // word
	offVer         = 0x0c
	offMFlags      = 0x10
	offMPages      = 0x14
	offStartObj    = 0x18
	offEIP         = 0x1c
	offStackObj    = 0x20
	offESP         = 0x24
	offPageSize    = 0x28
	offPageShift   = 0x2c
	offFixupSize   = 0x30
	offFixupSum    = 0x34
	offLdrSize     = 0x38
	offLdrSum      = 0x3c
	offObjTab      = 0x40
	offObjCnt      = 0x44
	offObjMap      = 0x48
	offIterMap     = 0x4c
	offRsrcTab     = 0x50
	offRsrcCnt     = 0x54
	offResTab      = 0x58
	offEntTab      = 0x5c
	offDirTab      = 0x60
	offDirCnt      = 0x64
	offFPageTab    = 0x68
	offFRecTab     = 0x6c
	offImpMod      = 0x70
	offImpModCnt   = 0x74
	offImpProc     = 0x78
	offPageSum     = 0x7c
	offDataPage    = 0x80
	offPreload     = 0x84
	offNResTab     = 0x88
	offCbNResTab   = 0x8c
	offNResSum     = 0x90
	offAutoData    = 0x94
	offDebugInfo   = 0x98
	offDebugLen    = 0x9c
	offInstPreload = 0xa0
	offInstDemand  = 0xa4
	offHeapSize    = 0xa8
	offStackSize   = 0xac

	headerSize = 0xb0
)

// Object flags.
const (
	ObjReadable   = 0x0001
	ObjWritable   = 0x0002
	ObjExecutable = 0x0004
	ObjResource   = 0x0008
	ObjDiscardabl = 0x0010
	ObjShared     = 0x0020
	ObjPreload    = 0x0040
	ObjInvalid    = 0x0080
	ObjBigDefault = 0x2000 // USE32: 32-bit segment
)

// Page map entry flags.
const (
	PageLegal    = 0
	PageIterated = 1
	PageInvalid  = 2
	PageZeroed   = 3
	PageRange    = 4
)

// Header holds the parsed LE header fields we care about.
type Header struct {
	// FileOff is where the LE header starts inside the containing file. Almost
	// every other offset in the header is relative to it, not to the file.
	FileOff   uint32
	Magic     string
	CPU       uint16
	OS        uint16
	MFlags    uint32
	MPages    uint32
	StartObj  uint32
	EIP       uint32
	StackObj  uint32
	ESP       uint32
	PageSize  uint32
	PageShift uint32
	FixupSize uint32
	LdrSize   uint32
	ObjTab    uint32
	ObjCnt    uint32
	ObjMap    uint32
	IterMap   uint32
	FPageTab  uint32
	FRecTab   uint32
	DataPage  uint32
	AutoData  uint32
	HeapSize  uint32
	StackSize uint32
}

// Object is one entry of the LE object table: a contiguous region mapped at a
// fixed linear address by the loader.
type Object struct {
	Index    int    // 1-based, as referenced by fixups
	VSize    uint32 // virtual size, may exceed the number of mapped pages (BSS)
	Base     uint32 // relocation base address
	Flags    uint32
	PageMap  uint32 // 1-based index into the object page map
	PageCnt  uint32
	FirstVA  uint32 // == Base
	MappedSz uint32 // PageCnt * PageSize, i.e. how much comes from the file
}

// Perms renders the object flags the way a disassembler would show them.
func (o Object) Perms() string {
	s := []byte("---")
	if o.Flags&ObjReadable != 0 {
		s[0] = 'r'
	}
	if o.Flags&ObjWritable != 0 {
		s[1] = 'w'
	}
	if o.Flags&ObjExecutable != 0 {
		s[2] = 'x'
	}
	return string(s)
}

// Page is one entry of the object page map.
type Page struct {
	Number uint32 // 1-based physical page number in the data pages area
	Flags  byte
}

// Image is a fully parsed LE image plus the object/page tables needed to lay it
// out in memory.
type Image struct {
	Raw     []byte // the whole containing file
	Header  Header
	Objects []Object
	Pages   []Page
	// PageVA[i] is the linear address at which page i (0-based, in page map
	// order) is mapped.
	PageVA []uint32
}

// FindHeader scans data for a plausible LE header and returns its offset.
// A signature match alone is not enough: the byte pattern "LE\0\0" occurs
// inside the game's data too, so candidates are validated against the fields
// that a real 32-bit DOS/4G image must have.
func FindHeader(data []byte) (uint32, error) {
	for i := 0; i+headerSize <= len(data); i++ {
		if data[i] != 'L' || data[i+1] != 'E' || data[i+2] != 0 || data[i+3] != 0 {
			continue
		}
		h := parseHeader(data, uint32(i))
		if h.CPU < 1 || h.CPU > 4 {
			continue
		}
		if h.PageSize != 0x1000 && h.PageSize != 0x200 {
			continue
		}
		if h.ObjCnt == 0 || h.ObjCnt > 64 || h.MPages == 0 {
			continue
		}
		// The object table always follows the header immediately in practice,
		// and the page map follows the object table.
		if h.ObjTab < headerSize || h.ObjMap != h.ObjTab+h.ObjCnt*24 {
			continue
		}
		return uint32(i), nil
	}
	return 0, fmt.Errorf("no plausible LE header found")
}

func parseHeader(d []byte, base uint32) Header {
	u32 := func(off uint32) uint32 { return binary.LittleEndian.Uint32(d[base+off:]) }
	u16 := func(off uint32) uint16 { return binary.LittleEndian.Uint16(d[base+off:]) }
	return Header{
		FileOff:   base,
		Magic:     string(d[base : base+2]),
		CPU:       u16(offCPU),
		OS:        u16(offOS),
		MFlags:    u32(offMFlags),
		MPages:    u32(offMPages),
		StartObj:  u32(offStartObj),
		EIP:       u32(offEIP),
		StackObj:  u32(offStackObj),
		ESP:       u32(offESP),
		PageSize:  u32(offPageSize),
		PageShift: u32(offPageShift),
		FixupSize: u32(offFixupSize),
		LdrSize:   u32(offLdrSize),
		ObjTab:    u32(offObjTab),
		ObjCnt:    u32(offObjCnt),
		ObjMap:    u32(offObjMap),
		IterMap:   u32(offIterMap),
		FPageTab:  u32(offFPageTab),
		FRecTab:   u32(offFRecTab),
		DataPage:  u32(offDataPage),
		AutoData:  u32(offAutoData),
		HeapSize:  u32(offHeapSize),
		StackSize: u32(offStackSize),
	}
}

// Parse locates and parses the LE image inside data.
func Parse(data []byte) (*Image, error) {
	off, err := FindHeader(data)
	if err != nil {
		return nil, err
	}
	img := &Image{Raw: data, Header: parseHeader(data, off)}
	h := &img.Header

	otab := off + h.ObjTab
	for i := uint32(0); i < h.ObjCnt; i++ {
		p := otab + i*24
		if int(p)+24 > len(data) {
			return nil, fmt.Errorf("object table entry %d out of bounds", i+1)
		}
		o := Object{
			Index:   int(i) + 1,
			VSize:   binary.LittleEndian.Uint32(data[p+0:]),
			Base:    binary.LittleEndian.Uint32(data[p+4:]),
			Flags:   binary.LittleEndian.Uint32(data[p+8:]),
			PageMap: binary.LittleEndian.Uint32(data[p+12:]),
			PageCnt: binary.LittleEndian.Uint32(data[p+16:]),
		}
		o.FirstVA = o.Base
		o.MappedSz = o.PageCnt * h.PageSize
		img.Objects = append(img.Objects, o)
	}

	pmap := off + h.ObjMap
	img.Pages = make([]Page, h.MPages)
	for i := uint32(0); i < h.MPages; i++ {
		p := pmap + i*4
		if int(p)+4 > len(data) {
			return nil, fmt.Errorf("page map entry %d out of bounds", i)
		}
		// LE stores the page number as a 24-bit big-endian value followed by a
		// flags byte. This is the one big-endian field in an otherwise
		// little-endian format.
		img.Pages[i] = Page{
			Number: uint32(data[p])<<16 | uint32(data[p+1])<<8 | uint32(data[p+2]),
			Flags:  data[p+3],
		}
	}

	// Assign a linear address to every page via the object that owns it.
	img.PageVA = make([]uint32, h.MPages)
	for _, o := range img.Objects {
		for k := uint32(0); k < o.PageCnt; k++ {
			idx := o.PageMap - 1 + k
			if idx >= h.MPages {
				return nil, fmt.Errorf("object %d page map range out of bounds", o.Index)
			}
			img.PageVA[idx] = o.Base + k*h.PageSize
		}
	}
	return img, nil
}

// pageFileOff returns the file offset of the data for page i (0-based, page map
// order), or false if the page has no backing data.
func (img *Image) pageFileOff(i int) (uint32, bool) {
	pg := img.Pages[i]
	if pg.Flags != PageLegal || pg.Number == 0 {
		return 0, false
	}
	h := &img.Header
	// Every page in this image is a plain, uncompressed 4K page laid out
	// sequentially, so the simple formula applies. Iterated (compressed) pages
	// would need to be expanded via IterMap instead.
	return h.FileOff + h.DataPage + (pg.Number-1)*h.PageSize, true
}

// Rebase moves objects to different addresses and recomputes the page mapping.
//
// The point is to produce an image laid out the way the loader really lays it
// out. DOS/4G places the objects at addresses unrelated to the bases in the
// object table, and it adds those run-time addresses to every fixup, so an image
// built with the table's bases has absolute operands pointing into an address
// space that does not exist at run time. Rebasing to the observed load addresses
// yields an image byte-identical to the running program's.
func (img *Image) Rebase(bases map[int]uint32) error {
	for i := range img.Objects {
		if base, ok := bases[img.Objects[i].Index]; ok {
			img.Objects[i].Base = base
			img.Objects[i].FirstVA = base
		}
	}
	h := &img.Header
	for i := range img.PageVA {
		img.PageVA[i] = 0
	}
	for _, o := range img.Objects {
		for k := uint32(0); k < o.PageCnt; k++ {
			idx := o.PageMap - 1 + k
			if idx >= h.MPages {
				return fmt.Errorf("object %d page map range out of bounds", o.Index)
			}
			img.PageVA[idx] = o.Base + k*h.PageSize
		}
	}
	return nil
}
