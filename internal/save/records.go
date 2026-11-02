package save

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// Decode reads record i of a block into v, a pointer to a packed record
// struct whose size must equal the block's record size.
func (f *File) Decode(block string, i int, v any) error {
	r := f.Record(block, i)
	if r == nil {
		return fmt.Errorf("no record %d in block %s", i, block)
	}
	if n := binary.Size(v); n != len(r) {
		return fmt.Errorf("%T is %d bytes, %s records are %d", v, n, block, len(r))
	}
	return binary.Read(bytes.NewReader(r), binary.LittleEndian, v)
}

// Encode writes v back as record i of a block.
func (f *File) Encode(block string, i int, v any) error {
	r := f.Record(block, i)
	if r == nil {
		return fmt.Errorf("no record %d in block %s", i, block)
	}
	var b bytes.Buffer
	if err := binary.Write(&b, binary.LittleEndian, v); err != nil {
		return err
	}
	if b.Len() != len(r) {
		return fmt.Errorf("%T encodes to %d bytes, %s records are %d", v, b.Len(), block, len(r))
	}
	copy(r, b.Bytes())
	return nil
}

// CString returns a NUL-terminated string from a fixed buffer.
func CString(b []byte) string {
	if i := bytes.IndexByte(b, 0); i >= 0 {
		return string(b[:i])
	}
	return string(b)
}

// DecodeBlock reads a whole block into v, a pointer to an array of records
// or a packed value as large as the block.
func (f *File) DecodeBlock(block string, v any) error {
	b := f.Block(block)
	if n := binary.Size(v); n != len(b) {
		return fmt.Errorf("%T is %d bytes, block %s is %d", v, n, block, len(b))
	}
	return binary.Read(bytes.NewReader(b), binary.LittleEndian, v)
}
