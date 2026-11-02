package save

import (
	"fmt"
	"reflect"
)

// FieldDiff is one difference between two saves: a field of a record, or a
// byte range of a block without a typed record.
type FieldDiff struct {
	Block  string
	Record int    // -1 for one-record blocks
	Field  string // dotted path, "" for raw bytes
	A, B   any
}

func (d FieldDiff) String() string {
	r := ""
	if d.Record >= 0 {
		r = fmt.Sprintf("[%d]", d.Record)
	}
	f := ""
	if d.Field != "" {
		f = "." + d.Field
	}
	return fmt.Sprintf("%s%s%s: %v -> %v", d.Block, r, f, d.A, d.B)
}

// typedBlocks maps the record blocks to their Go types.
var typedBlocks = map[string]reflect.Type{
	BColonies:    reflect.TypeOf(Colony{}),
	BPlanets:     reflect.TypeOf(Planet{}),
	BStars:       reflect.TypeOf(Star{}),
	BLeaders:     reflect.TypeOf(Leader{}),
	BPlayers:     reflect.TypeOf(Player{}),
	BShips:       reflect.TypeOf(Ship{}),
	BMoveableBox: reflect.TypeOf(MoveableBox{}),
}

// Diff lists the differences between two saves, field by field for the
// typed records and byte by byte for the other blocks.
func Diff(a, b *File) ([]FieldDiff, error) {
	var out []FieldDiff
	for _, bl := range Layout {
		if t, ok := typedBlocks[bl.Name]; ok {
			for i := 0; i < bl.Count; i++ {
				if string(a.Record(bl.Name, i)) == string(b.Record(bl.Name, i)) {
					continue
				}
				va, vb := reflect.New(t), reflect.New(t)
				if err := a.Decode(bl.Name, i, va.Interface()); err != nil {
					return nil, err
				}
				if err := b.Decode(bl.Name, i, vb.Interface()); err != nil {
					return nil, err
				}
				diffValue(&out, bl.Name, i, "", va.Elem(), vb.Elem())
			}
			continue
		}
		x, y := a.Block(bl.Name), b.Block(bl.Name)
		for i := 0; i < len(x); i++ {
			if x[i] == y[i] {
				continue
			}
			j := i
			for j < len(x) && x[j] != y[j] {
				j++
			}
			rec := -1
			if bl.Count > 1 {
				rec = i / bl.Size
			}
			out = append(out, FieldDiff{bl.Name, rec, fmt.Sprintf("bytes %#x..%#x", i, j-1), fmt.Sprintf("% x", x[i:j]), fmt.Sprintf("% x", y[i:j])})
			i = j
		}
	}
	return out, nil
}

func diffValue(out *[]FieldDiff, block string, rec int, path string, a, b reflect.Value) {
	switch a.Kind() {
	case reflect.Struct:
		for k := 0; k < a.NumField(); k++ {
			p := a.Type().Field(k).Name
			if path != "" {
				p = path + "." + p
			}
			diffValue(out, block, rec, p, a.Field(k), b.Field(k))
		}
	case reflect.Array:
		if a.Type().Elem().Kind() == reflect.Uint8 && a.Len() > 8 {
			// Byte arrays such as names: report them whole.
			if !reflect.DeepEqual(a.Interface(), b.Interface()) {
				*out = append(*out, FieldDiff{block, rec, path, fmt.Sprintf("% x", bytesOf(a)), fmt.Sprintf("% x", bytesOf(b))})
			}
			return
		}
		for k := 0; k < a.Len(); k++ {
			diffValue(out, block, rec, fmt.Sprintf("%s[%d]", path, k), a.Index(k), b.Index(k))
		}
	default:
		if a.Interface() != b.Interface() {
			*out = append(*out, FieldDiff{block, rec, path, a.Interface(), b.Interface()})
		}
	}
}

func bytesOf(v reflect.Value) []byte {
	b := make([]byte, v.Len())
	for i := range b {
		b[i] = byte(v.Index(i).Uint())
	}
	return b
}
