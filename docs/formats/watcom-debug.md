# Watcom debug information, version 3

As found at the end of `Orion2.exe`. Reader: `internal/wdbg`. Layout follows
Open Watcom's `wdbginfo.h`; the points below are what had to be learned from
the data.

## Block

```
[language table][segment table][section header + section data]...[master header]
```

Master header (last 14 bytes): `u16 signature 0x8386`, exe version 3.0, object
version 1.3, `u16 lang_size`, `u16 segment_size`, `u32 debug_size` (whole
block). Languages: `"C"`. Segments: 1, 2 (LE objects).

Section header (18 bytes): `u32 mod_offset, gbl_offset, addr_offset,
section_size`, `u16 id`. Offsets are relative to the section header.

- Module (21 + n bytes): `u16 language`, three demand pointers (`u32 offset,
  u16 count`) for locals, types, lines, length-prefixed name.
- Global (10 + n bytes): `u32 offset, u16 segment, u16 module, u8 kind`
  (1 static, 2 data, 4 code), length-prefixed name.
- Address map: per block `u32 offset, u16 segment, u16 count`, then `count`
  runs of `u32 size, u16 module` laid end to end.

## Demand-loaded tables

A demand pointer addresses `count+1` section offsets; block `i` spans
`[off[i], off[i+1])`. Neighbouring modules share boundary entries.

Records are `u8 length (including itself), u8 kind, body`. Only the 51
`F:\WLIBS\...` engine modules have them; `MOX\*.C` game modules have only
globals.

### Types

- Indexes count records in order starting at 1, but cue-table (0x13), EOF
  (0x14), struct field (0x61–0x66) and enumerator (0x51–0x53) records take no
  index. A type block may contain unrelated data after its EOF record.
- An index is one byte, or two big-endian bytes when the first has bit 7 set.
- Scalar (0x10): `u8 class|size-1` — class 0x00 signed, 0x10 unsigned, 0x20
  float, 0x30 void. Watcom's `char` is unsigned.
- Name (0x12): `index scope, index type, name`. A nonzero scope ("struct",
  "union", "enum" scope records, 0x11) names the list itself; zero is a typedef.
- Arrays 0x20/0x21/0x22: high bound (u8/u16/u32), element type.
- Pointers 0x40–0x49: target type (0x46 near386 is the common one).
- Struct list 0x60: `u16 field count, u32 size`, then fields last to first:
  0x61/0x62/0x63 `offset (u8/u16/u32), type, name`; 0x64–0x66 bitfields add
  `u8 start, u8 size`.
- Enum list 0x50: `u16 count, u8 scalar`, then 0x51–0x53 `value, name`.
- Procedure 0x72 (near386): `return type, u8 count, parameter types`.

### Symbols (locals)

- 0x32 set base: `u32 offset, u16 segment`; code offsets that follow are
  relative to it.
- 0x12 module static: `u32 offset, u16 segment, type, name`.
- 0x24 near routine 386: `u32 start, u32 size, u16 parent record offset, u8
  prologue size, u8 epilogue size, u32 return address offset from EBP, type
  (procedure), return location, u8 parameter count, parameter locations,
  name`. The locals that follow belong to it; its first N locals are the
  parameters' spill slots.
- 0x23 block 386: `u32 start, u32 size, u16 parent`.
- 0x11 local: `location, type, name`.

Locations: 0x00 none (void result); 0x10/0x11/0x12 EBP offset s8/s16/s32;
0x21 static `u32 offset, u16 segment`; `0x3n` then `n+1` register bytes;
`0x4n` single register `n` (0–15); a byte with bit 7 set starts a stack
expression of that many bytes. Register numbers: 0–7 AL..DH, 8–15 AX..SP,
16–19 CS SS DS ES, 20–27 ST0–7, 28–35 EAX EBX ECX EDX ESI EDI EBP ESP.

### Lines

Runs of `u32 ref, u16 count`, then `count` × `u16 line, u32 offset`. `ref` is
a byte offset into the address map naming the run's code range; offsets are
relative to that range's start. Runs hold at most 32 entries.
