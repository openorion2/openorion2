// Command exedata writes the constant data of ORION2.EXE the game reads
// into internal/game/exedata.bin, so the game runs without the executable
// (port-only, docs/formats/exedata.md): the ranges at the addresses the
// game's code reads, directly or through wrappers (literal constants, with
// a short span after each, the exact length, or a table when indexed; whole
// C strings), what those ranges point at, the ranges a recorded run read
// (-trace), and the E string slots Load_H_And_Pro_File_Strings_ sets up.
//
//	go run ./tools/exedata -exe game/ORION2.EXE [-trace reads.txt]
package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"go/ast"
	"go/constant"
	"go/importer"
	"go/parser"
	"go/scanner"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/openorion2/openorion2/internal/le"
)

const (
	codeBase          = 0x10000
	dataBase, dataEnd = 0x178000, 0x178000 + 0x5dcd0
	span              = 0x20  // after a constant with no known length
	tableMax          = 0x800 // after the base of an indexed table
	loadStrings       = 0xce0e5
	loadStringsEnd    = 0xcec99
	eStringsRoutine   = 0xcdf5c
)

func main() {
	exe := flag.String("exe", "game/ORION2.EXE", "the original executable")
	trace := flag.String("trace", "tools/exedata/reads.txt", "recorded reads: \"addr(hex) len\" lines")
	out := flag.String("o", "internal/game/exedata.bin", "the output")
	tables := flag.String("tables", "tools/exedata/tables.txt", "constants whose table runs to the next constant")
	flag.Parse()
	data, err := os.ReadFile(*exe)
	check(err)
	img, err := le.Parse(data)
	check(err)
	_, err = img.SettleDataPage()
	check(err)
	flat, err := img.BuildFlat()
	check(err)
	_, err = img.Relocate(flat)
	check(err)
	at := func(a uint32) []byte {
		b, ok := flat.At(a)
		if !ok {
			return nil
		}
		return b
	}
	cov := map[uint32]bool{}
	add := func(a uint32, n int) {
		for i := 0; i < n; i++ {
			cov[a+uint32(i)] = true
		}
	}
	cstring := func(a uint32) int { // the length with its NUL, 0 if not a string
		b := at(a)
		n := 0
		for n < len(b) && n < 512 && b[n] != 0 {
			if b[n] < 0x20 && b[n] != '\n' && b[n] != '\r' && b[n] != '\t' {
				return 0
			}
			n++
		}
		if n < len(b) && b[n] == 0 {
			return n + 1
		}
		return 0
	}
	acc, tbls := accesses()
	lits := literals()
	for a, n := range acc {
		if n == 0 {
			n = span
			if s := cstring(a); s > n {
				n = s
			}
		}
		add(a, n)
	}
	// Whole tables: from the constant to the next one.
	var all []uint32
	for a := range acc {
		all = append(all, a)
	}
	all = append(all, lits...)
	sort.Slice(all, func(i, j int) bool { return all[i] < all[j] })
	table := func(a uint32) {
		i := sort.Search(len(all), func(i int) bool { return all[i] > a })
		end := uint32(dataEnd)
		if i < len(all) {
			end = all[i]
		}
		add(a, int(end-a))
	}
	// Indexed reads: tableMax bytes or 256 rows of the index's stride (a
	// constant inside a table, as tbl+1, would cut it short at the next one).
	for a, stride := range tbls {
		add(a, min(max(tableMax, stride*0x100), 0x2000))
	}
	if f, err := os.Open(*tables); err == nil {
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			t := strings.TrimSpace(sc.Text())
			if t == "" || t[0] == '#' {
				continue
			}
			a, _ := strconv.ParseUint(t, 16, 32)
			table(uint32(a))
		}
		f.Close()
	}
	for _, a := range lits {
		n := span
		if s := cstring(a); s > n {
			n = s
		}
		add(a, n)
	}
	if f, err := os.Open(*trace); err == nil {
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			p := strings.Fields(sc.Text())
			if len(p) != 2 {
				continue
			}
			a, _ := strconv.ParseUint(p[0], 16, 32)
			n, _ := strconv.Atoi(p[1])
			if a >= loadStrings && a < loadStringsEnd {
				continue // the slots below stand for this code
			}
			add(uint32(a), n)
		}
		f.Close()
	}
	for a := uint32(loadStrings); a < loadStringsEnd; a++ {
		delete(cov, a) // the slots below stand for this code
	}
	// What the words in the data point at: a string, or a span when aligned.
	var keys []uint32
	for k := range cov {
		keys = append(keys, k)
	}
	for _, k := range keys {
		b := at(k)
		if len(b) < 4 {
			continue
		}
		p := binary.LittleEndian.Uint32(b)
		if p >= dataBase && p < dataEnd && !cov[p] {
			n := cstring(p)
			if n == 0 && k%4 == 0 && k >= dataBase {
				n = span
			}
			add(p, n)
		}
	}
	// Ranges.
	var xs []uint32
	for k := range cov {
		if at(k) != nil {
			xs = append(xs, k)
		}
	}
	sort.Slice(xs, func(i, j int) bool { return xs[i] < xs[j] })
	var buf bytes.Buffer
	buf.WriteString("OO2X")
	var ranges [][2]uint32
	for _, x := range xs {
		if l := len(ranges); l > 0 && ranges[l-1][1] == x {
			ranges[l-1][1]++
		} else {
			ranges = append(ranges, [2]uint32{x, x + 1})
		}
	}
	w := func(v uint32) { binary.Write(&buf, binary.LittleEndian, v) }
	w(uint32(len(ranges)))
	total := 0
	for _, r := range ranges {
		w(r[0])
		w(r[1] - r[0])
		buf.Write(at(r[0])[:r[1]-r[0]])
		total += int(r[1] - r[0])
	}
	// E string slots.
	slots := eSlots(at(loadStrings)[:loadStringsEnd-loadStrings])
	w(uint32(len(slots)))
	var sk []uint32
	for k := range slots {
		sk = append(sk, k)
	}
	sort.Slice(sk, func(i, j int) bool { return sk[i] < sk[j] })
	for _, k := range sk {
		w(k)
		w(uint32(slots[k]))
	}
	check(os.WriteFile(*out, buf.Bytes(), 0o644))
	fmt.Printf("%s: %d bytes in %d ranges, %d E string slots\n", *out, total, len(ranges), len(slots))
}

// eSlots reads Load_H_And_Pro_File_Strings_: "mov eax, i; call
// E_Strings_; mov [slot], eax".
func eSlots(code []byte) map[uint32]int {
	m := map[uint32]int{}
	for i := 0; i+15 <= len(code); i++ {
		if code[i] != 0xb8 || code[i+5] != 0xe8 || code[i+10] != 0xa3 {
			continue
		}
		call := uint32(loadStrings+i+10) + binary.LittleEndian.Uint32(code[i+6:])
		if call != eStringsRoutine {
			continue
		}
		m[binary.LittleEndian.Uint32(code[i+11:])] = int(binary.LittleEndian.Uint32(code[i+1:]))
	}
	return m
}

// literals are the integer literals of internal/game in the data object.
func literals() []uint32 {
	var out []uint32
	files, _ := filepath.Glob("internal/game/*.go")
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		check(err)
		var s scanner.Scanner
		fs := token.NewFileSet()
		s.Init(fs.AddFile(f, -1, len(src)), src, nil, 0)
		for {
			_, tok, lit := s.Scan()
			if tok == token.EOF {
				break
			}
			if tok != token.INT {
				continue
			}
			if v, err := strconv.ParseUint(strings.ReplaceAll(lit, "_", ""), 0, 64); err == nil && v >= dataBase && v < dataEnd {
				out = append(out, uint32(v))
			}
		}
	}
	return out
}

// accesses are the constants in the address of every read of the image,
// with the literal length when the read has one (0 otherwise).
func accesses() (map[uint32]int, map[uint32]int) {
	fset := token.NewFileSet()
	var files []*ast.File
	paths, _ := filepath.Glob("internal/game/*.go")
	for _, p := range paths {
		if strings.HasSuffix(p, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, p, nil, 0)
		if err != nil {
			panic(err)
		}
		files = append(files, f)
	}
	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil), Error: func(error) {}}
	info := &types.Info{Types: map[ast.Expr]types.TypeAndValue{}}
	conf.Check("github.com/openorion2/openorion2/internal/game", fset, files, info)
	isConst := func(e ast.Expr) bool {
		tv, ok := info.Types[e]
		return ok && tv.Value != nil
	}
	baseReaders := map[string]bool{"bytes": true, "u32": true, "cStringAt": true, "exeI16": true, "exeU16": true, "exeU8": true, "exeI32": true, "exeU32": true}
	readers := map[string]int{"bytes": 0, "u32": 0, "cStringAt": 1, "exeI16": 0, "exeU16": 0, "exeU8": 0, "exeI32": 0, "exeU32": 0}
	// Wrappers: functions that pass one of their parameters into the address
	// of a read (exeColors(addr), helpListAt(addr, n), ...), to a fixpoint.
	for changed := true; changed; {
		changed = false
		for _, f := range files {
			for _, d := range f.Decls {
				fd, ok := d.(*ast.FuncDecl)
				if !ok || fd.Body == nil {
					continue
				}
				if _, known := readers[fd.Name.Name]; known {
					continue
				}
				params := map[string]int{}
				i := 0
				for _, fl := range fd.Type.Params.List {
					for _, n := range fl.Names {
						params[n.Name] = i
						i++
					}
					if len(fl.Names) == 0 {
						i++
					}
				}
				// Locals computed from a parameter stand for it (a := addr + i*10).
				for grew := true; grew; {
					grew = false
					ast.Inspect(fd.Body, func(n ast.Node) bool {
						as, ok := n.(*ast.AssignStmt)
						if !ok {
							return true
						}
						from := -1
						for _, r := range as.Rhs {
							ast.Inspect(r, func(m ast.Node) bool {
								if id, ok := m.(*ast.Ident); ok {
									if pi, ok := params[id.Name]; ok && from < 0 {
										from = pi
									}
								}
								return true
							})
						}
						if from < 0 {
							return true
						}
						for _, l := range as.Lhs {
							if id, ok := l.(*ast.Ident); ok && id.Name != "_" {
								if _, ok := params[id.Name]; !ok {
									params[id.Name] = from
									grew = true
								}
							}
						}
						return true
					})
				}
				found := -1
				ast.Inspect(fd.Body, func(n ast.Node) bool {
					call, ok := n.(*ast.CallExpr)
					if !ok || found >= 0 {
						return found < 0
					}
					name := ""
					switch fn := call.Fun.(type) {
					case *ast.SelectorExpr:
						name = fn.Sel.Name
					case *ast.Ident:
						name = fn.Name
					}
					argi, ok := readers[name]
					if !ok || len(call.Args) <= argi {
						return true
					}
					ast.Inspect(call.Args[argi], func(m ast.Node) bool {
						if id, ok := m.(*ast.Ident); ok {
							if pi, ok := params[id.Name]; ok && found < 0 {
								found = pi
							}
						}
						return true
					})
					return true
				})
				if found >= 0 {
					readers[fd.Name.Name] = found
					changed = true
				}
			}
		}
	}
	out := map[uint32]int{}
	tbls := map[uint32]int{} // table base: the largest row stride seen
	// stride is the largest constant factor of a product in e (k*0xd).
	stride := func(e ast.Expr) int {
		st := 1
		ast.Inspect(e, func(m ast.Node) bool {
			b, ok := m.(*ast.BinaryExpr)
			if !ok || b.Op != token.MUL {
				return true
			}
			for _, x := range []ast.Expr{b.X, b.Y} {
				if tv, ok := info.Types[x]; ok && tv.Value != nil && tv.Value.Kind() == constant.Int {
					if v, ok := constant.Int64Val(tv.Value); ok && v < 0x1000 {
						st = max(st, int(v))
					}
				}
			}
			return true
		})
		return st
	}
	table := func(v uint32, st int) { tbls[v] = max(tbls[v], st) }
	dataConst := func(e ast.Expr) (uint32, bool) {
		if tv, ok := info.Types[e]; ok && tv.Value != nil && tv.Value.Kind() == constant.Int {
			v, _ := constant.Uint64Val(tv.Value)
			if v >= codeBase && v < dataEnd {
				return uint32(v), true
			}
		}
		return 0, false
	}
	for _, f := range files {
		// Locals set from a constant and an index (row := tbl + k*n) make
		// the reads through them reads of a table at the constant.
		type local struct {
			v  uint32
			st int
		}
		locals := map[string][]local{}
		ast.Inspect(f, func(n ast.Node) bool {
			as, ok := n.(*ast.AssignStmt)
			if !ok || len(as.Lhs) != len(as.Rhs) {
				return true
			}
			for i, r := range as.Rhs {
				id, ok := as.Lhs[i].(*ast.Ident)
				if !ok || isConst(r) {
					continue
				}
				ast.Inspect(r, func(m ast.Node) bool {
					if e, ok := m.(ast.Expr); ok {
						if v, ok := dataConst(e); ok {
							locals[id.Name] = append(locals[id.Name], local{v, stride(r)})
							return false
						}
					}
					return true
				})
			}
			return true
		})
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			name := ""
			switch fn := call.Fun.(type) {
			case *ast.SelectorExpr:
				name = fn.Sel.Name
			case *ast.Ident:
				name = fn.Name
			}
			argi, ok := readers[name]
			if !ok || len(call.Args) <= argi {
				return true
			}
			length := int64(0)
			if _, base := baseReaders[name]; !base {
				length = -1 // a wrapper: a table at the constant
			} else if !isConst(call.Args[argi]) {
				length = -1 // constant + index: a table
			} else if name == "bytes" && len(call.Args) > 1 {
				if tv, ok := info.Types[call.Args[1]]; ok && tv.Value != nil {
					length, _ = constant.Int64Val(tv.Value)
				}
			}
			// Every constant subexpression of the address big enough to be one.
			ast.Inspect(call.Args[argi], func(m ast.Node) bool {
				if id, ok := m.(*ast.Ident); ok {
					for _, l := range locals[id.Name] {
						table(l.v, l.st)
					}
				}
				e, ok := m.(ast.Expr)
				if !ok {
					return true
				}
				if tv, ok := info.Types[e]; ok && tv.Value != nil && tv.Value.Kind() == constant.Int {
					v, _ := constant.Uint64Val(tv.Value)
					if v >= 0x10000 && v < 0x178000+0x5dcd0 {
						if length < 0 {
							table(uint32(v), stride(call.Args[argi]))
							length = 0
						}
						if int(length) > out[uint32(v)] || out[uint32(v)] == 0 {
							out[uint32(v)] = int(length)
						}
					}
					return false
				}
				return true
			})
			return true
		})
	}
	return out, tbls
}

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "exedata:", err)
		os.Exit(1)
	}
}
