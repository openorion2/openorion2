package game

import (
	_ "embed"
	"encoding/binary"
	"fmt"
	"os"
	"sort"
	"sync"

	"github.com/openorion2/openorion2/internal/le"
	"github.com/openorion2/openorion2/internal/save"
)

// exeImage is the constant data of ORION2.EXE the game reads, at the
// addresses DOS/4GW maps it to (the ELF addresses used throughout the
// specs): the ranges tools/exedata took from the executable into
// exedata.bin (port-only, docs/formats/exedata.md). The game needs no
// executable; while one lies in the game directory, a read past the
// ranges is taken from it and logged, to be added to the ranges.
type exeImage struct {
	starts []uint32 // the ranges' first addresses, ascending
	ranges [][]byte // their bytes, this game's own copy (initTechTables writes)
	slots  map[uint32]int
	orig   *le.Flat // the executable, when present (the fallback)
	log    func(f string, args ...any)
	missed map[uint32]bool

	techOnce sync.Once // initTech's table writes
}

//go:embed exedata.bin
var exeData []byte

// newExeImage is the image from exedata.bin; origPath, if the file exists,
// is the executable for reads past it.
func newExeImage(origPath string, log func(f string, args ...any)) (*exeImage, error) {
	d := exeData
	if len(d) < 8 || string(d[:4]) != "OO2X" {
		return nil, fmt.Errorf("exedata.bin: not the game's data")
	}
	x := &exeImage{log: log, missed: map[uint32]bool{}, slots: map[uint32]int{}}
	n := int(binary.LittleEndian.Uint32(d[4:]))
	p := 8
	for range n {
		a, l := binary.LittleEndian.Uint32(d[p:]), int(binary.LittleEndian.Uint32(d[p+4:]))
		x.starts = append(x.starts, a)
		x.ranges = append(x.ranges, append([]byte(nil), d[p+8:p+8+l]...))
		p += 8 + l
	}
	ns := int(binary.LittleEndian.Uint32(d[p:]))
	p += 4
	for range ns {
		x.slots[binary.LittleEndian.Uint32(d[p:])] = int(binary.LittleEndian.Uint32(d[p+4:]))
		p += 8
	}
	if os.Getenv("OPENORION2_NO_EXE") != "" {
		return x, nil // no fallback: a read past exedata.bin panics (tests of the data)
	}
	if data, err := os.ReadFile(origPath); err == nil {
		if flat, err := relocated(data); err == nil {
			x.orig = flat
		}
	}
	return x, nil
}

func relocated(data []byte) (*le.Flat, error) {
	img, err := le.Parse(data)
	if err != nil {
		return nil, err
	}
	if _, err := img.SettleDataPage(); err != nil {
		return nil, err
	}
	flat, err := img.BuildFlat()
	if err != nil {
		return nil, err
	}
	if _, err := img.Relocate(flat); err != nil {
		return nil, err
	}
	return flat, nil
}

// at is the data from addr to the end of its range, nil when addr is in
// none.
func (x *exeImage) at(addr uint32) []byte {
	i := sort.Search(len(x.starts), func(i int) bool { return x.starts[i] > addr }) - 1
	if i < 0 {
		return nil
	}
	off := addr - x.starts[i]
	if off >= uint32(len(x.ranges[i])) {
		return nil
	}
	return x.ranges[i][off:]
}

func (x *exeImage) bytes(addr uint32, n int) []byte {
	if b := x.at(addr); len(b) >= n {
		return b[:n]
	}
	if x.orig != nil {
		if b, ok := x.orig.At(addr); ok && len(b) >= n {
			x.miss(addr, n)
			return b[:n]
		}
	}
	panic(fmt.Sprintf("exe data: %d bytes at %#x are not in exedata.bin (tools/exedata)", n, addr))
}

// miss logs a read past exedata.bin, once per address.
func (x *exeImage) miss(addr uint32, n int) {
	if x.missed[addr] {
		return
	}
	x.missed[addr] = true
	if x.log != nil {
		x.log("exe data: %d bytes at %#x read from ORION2.EXE; add them to tools/exedata/reads.txt: %x %d", n, addr, addr, n)
	}
}

// cstring is the NUL-terminated string at addr.
func (x *exeImage) cstring(addr uint32) string {
	b := x.at(addr)
	if i := bytesIndexByte(b, 0); i >= 0 {
		return string(b[:i])
	}
	if x.orig != nil {
		if o, ok := x.orig.At(addr); ok {
			s := save.CString(o[:min(len(o), 256)])
			x.miss(addr, len(s)+1)
			return s
		}
	}
	panic(fmt.Sprintf("exe data: the string at %#x is not in exedata.bin (tools/exedata)", addr))
}

func bytesIndexByte(b []byte, c byte) int {
	for i, v := range b {
		if v == c {
			return i
		}
	}
	return -1
}

func (x *exeImage) u32(addr uint32) uint32 { return binary.LittleEndian.Uint32(x.bytes(addr, 4)) }

// eStringSlot is the E string index Load_H_And_Pro_File_Strings_
// (0xce0e5) stores in the pointer at slot ("mov eax, i; call E_Strings_;
// mov [slot], eax"), taken from its code by tools/exedata.
func (g *Game) eStringSlot(slot uint32) (int, bool) {
	n, ok := g.exe.slots[slot]
	return n, ok
}

// eStringAt is the string Load_H_And_Pro_File_Strings_ points slot at.
func (g *Game) eStringAt(slot uint32) (string, error) {
	n, ok := g.eStringSlot(slot)
	if !ok {
		return "", fmt.Errorf("no E string is stored at %#x", slot)
	}
	es, err := g.eStrings()
	if err != nil {
		return "", err
	}
	return es[n], nil
}
