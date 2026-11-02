# The game's constant data (exedata.bin)

The port does not need `ORION2.EXE`. The constant data the original keeps in
its executable, and the port reads, lives in `internal/game/exedata.bin`,
built into the binary: the tables of buildings, specials, weapons, armour,
shields, computers and drives, race and AI tables, screen layouts, sound and
animation tables, and short strings (file names, number formats, "Colony
Ship"). The texts of the game come from its LBX files, as in the original.

The data keeps the original's addresses (the ELF addresses of the specs), so
the code reads `g.exe.bytes(addr, n)`, `cStringAt(g.exe, addr)` and the
like as before, from the built-in ranges instead of the file.

## Format

Little-endian:

| Field | Size | |
|---|---|---|
| magic | 4 | `OO2X` |
| ranges | 4 | the number of ranges |
| per range: address, length, bytes | 4 + 4 + length | ascending, disjoint |
| slots | 4 | the number of E string slots |
| per slot: pointer slot, E string index | 4 + 4 | what `Load_H_And_Pro_File_Strings_` (0xce0e5) stores where |

## Building it

`go run ./tools/exedata -exe game/ORION2.EXE` writes it from:

- every constant the game's code reads the image at (`go/types` over
  `internal/game`): the exact length when the read has a literal one, else
  32 bytes, or the whole C string there;
- the same through wrappers: a function whose parameter (or a local
  computed from one) is the address of a read, as `helpListAt(addr, n)`,
  reads a table at the constant its callers pass;
- tables: a read at a constant plus an index (`tbl + k*0xd`), directly or
  through a local (`row := tbl + k*n`), takes 2 KB from the constant, or
  256 rows of the index's stride when longer (at most 8 KB);
- every integer literal of `internal/game` in the data object, the same way;
- `tools/exedata/tables.txt`: tables the code indexes further, taken whole
  up to the next constant;
- `tools/exedata/reads.txt`: ranges runs of the game read (the test suite,
  long autopilot games, `tools/monkey` over the menus and saved games);
- what any of those ranges points at: a C string, or 32 bytes from an
  aligned pointer;
- the E string slots, read from `Load_H_And_Pro_File_Strings_`'s code.

## A read past the data

While an `ORION2.EXE` lies in the game directory, a read past the built-in
ranges is taken from it and logged ("exe data: … add them to
tools/exedata/reads.txt"); add the line and rebuild. Without the executable
such a read panics with the address. `OPENORION2_NO_EXE=1` turns the
fallback off, to test the data alone.
