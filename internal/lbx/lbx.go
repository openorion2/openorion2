// Package lbx reads LBX archives, the container nearly every game asset ships in.
//
// The layout follows the engine's loaders in WLIBS\lbx\farload.c and
// WLIBS\sound\lbx.c. A fixed 2048-byte header holds
//
//	int16  entries
//	uint16 magic       0xFEAD
//	int16  file_type
//	int16  dummy
//	uint32 offsets[510]
//
// Entry n occupies [offsets[n], offsets[n+1]), so the table holds entries+1
// meaningful offsets. Some entries are record tables, read by the *_Data
// loaders: a uint16 record count and a uint16 record size, then the records.
//
// Spec: docs/formats/lbx.md.
package lbx

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
)

// ErrSmacker reports a Smacker video that ships with an .LBX name, as the
// cutscenes (INTRO.LBX, *FIN.LBX) do.
var ErrSmacker = errors.New("Smacker video, not an LBX archive")

const (
	// Magic identifies an LBX archive.
	Magic = 0xFEAD
	// HeaderSize is the size of the fixed header the engine reads in one go.
	HeaderSize = 0x800
	// MaxEntries is how many entries the header has room for: 510 offsets
	// delimit 509 entries.
	MaxEntries = (HeaderSize-8)/4 - 1
)

// Archive is a parsed LBX file held in memory.
type Archive struct {
	FileType int16
	Dummy    int16
	Offsets  []uint32 // len(Offsets) == number of entries + 1
	data     []byte
}

// Open reads and parses the archive at path.
func Open(path string) (*Archive, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	a, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return a, nil
}

// Parse parses an archive from its bytes. It checks what the engine checks and
// also that every entry lies inside the file, which the engine takes on trust.
func Parse(data []byte) (*Archive, error) {
	if len(data) < 8 {
		return nil, fmt.Errorf("file too short for an LBX header")
	}
	if bytes.HasPrefix(data, []byte("SMK2")) || bytes.HasPrefix(data, []byte("SMK4")) {
		return nil, ErrSmacker
	}
	le := binary.LittleEndian
	n := int(int16(le.Uint16(data[0:])))
	if m := le.Uint16(data[2:]); m != Magic {
		return nil, fmt.Errorf("bad magic 0x%04x", m)
	}
	if n < 0 || n > MaxEntries {
		return nil, fmt.Errorf("entry count %d out of range", n)
	}
	if len(data) < 8+4*(n+1) {
		return nil, fmt.Errorf("offset table truncated")
	}
	a := &Archive{
		FileType: int16(le.Uint16(data[4:])),
		Dummy:    int16(le.Uint16(data[6:])),
		Offsets:  make([]uint32, n+1),
		data:     data,
	}
	for i := range a.Offsets {
		a.Offsets[i] = le.Uint32(data[8+4*i:])
	}
	for i := 0; i < n; i++ {
		if a.Offsets[i] > a.Offsets[i+1] {
			return nil, fmt.Errorf("entry %d ends before it starts (0x%x > 0x%x)", i, a.Offsets[i], a.Offsets[i+1])
		}
	}
	if n > 0 && uint64(a.Offsets[n]) > uint64(len(data)) {
		return nil, fmt.Errorf("last entry ends at 0x%x, past the end of the file (0x%x)", a.Offsets[n], len(data))
	}
	return a, nil
}

// Len returns the number of entries.
func (a *Archive) Len() int { return len(a.Offsets) - 1 }

// Entry returns the bytes of entry i. The slice aliases the archive's data.
func (a *Archive) Entry(i int) ([]byte, error) {
	if i < 0 || i >= a.Len() {
		return nil, fmt.Errorf("entry %d out of range [0,%d)", i, a.Len())
	}
	return a.data[a.Offsets[i]:a.Offsets[i+1]], nil
}

// Table is an entry laid out as fixed-size records.
type Table struct {
	Count, Size int
	Data        []byte // Count*Size bytes
}

// Record returns record i of the table.
func (t Table) Record(i int) []byte { return t.Data[i*t.Size : (i+1)*t.Size] }

// Table interprets entry i as a record table, as Farload_Library_Data does.
func (a *Archive) Table(i int) (Table, error) {
	e, err := a.Entry(i)
	if err != nil {
		return Table{}, err
	}
	if len(e) < 4 {
		return Table{}, fmt.Errorf("entry %d too short for a record table", i)
	}
	t := Table{
		Count: int(binary.LittleEndian.Uint16(e[0:])),
		Size:  int(binary.LittleEndian.Uint16(e[2:])),
	}
	if 4+t.Count*t.Size > len(e) {
		return Table{}, fmt.Errorf("entry %d: %d records of %d bytes overrun its %d bytes", i, t.Count, t.Size, len(e))
	}
	t.Data = e[4 : 4+t.Count*t.Size]
	return t, nil
}

// ReadEntry reads entry i of the archive at path without loading the rest
// (the music archives are tens of megabytes).
func ReadEntry(path string, i int) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var h [8]byte
	if _, err := f.ReadAt(h[:], 0); err != nil {
		return nil, err
	}
	n := int(int16(binary.LittleEndian.Uint16(h[0:])))
	if binary.LittleEndian.Uint16(h[2:]) != Magic || i < 0 || i >= n {
		return nil, fmt.Errorf("%s: no entry %d", path, i)
	}
	var o [8]byte
	if _, err := f.ReadAt(o[:], int64(8+4*i)); err != nil {
		return nil, err
	}
	lo, hi := binary.LittleEndian.Uint32(o[0:]), binary.LittleEndian.Uint32(o[4:])
	if hi < lo {
		return nil, fmt.Errorf("%s: entry %d ends before it starts", path, i)
	}
	b := make([]byte, hi-lo)
	if _, err := f.ReadAt(b, int64(lo)); err != nil {
		return nil, err
	}
	return b, nil
}
