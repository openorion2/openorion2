# LBX archives

Reader: `internal/lbx`. Source of truth in the binary: `WLIBS\lbx\farload.c`
(`Farload_Library`, `Farload_Library_Data`, `struct_lbx_header`) and
`WLIBS\sound\lbx.c` (`LBX_init`, `LBX_readRecord`).

```
0x000  int16  entries
0x002  uint16 magic          0xFEAD
0x004  int16  file_type      0 in every shipped archive
0x006  int16  dummy
0x008  uint32 offsets[510]   entries+1 meaningful
0x800  entry data
```

- Entry `n` is `[offsets[n], offsets[n+1])`. The engine reads the 0x800-byte
  header in one go and checks only the magic and `entry < entries`.
- In all 363 shipped archives entry data starts at 0x800 and the last offset
  equals the file size.
- Record-table entries (read by the `*_Data` loaders): `u16 count, u16 size`,
  then `count` records of `size` bytes. The loader fails if the caller's
  record size differs. Used by text (`*MSG*`, `*TEXT*`) and data archives
  (`TECHDESC`, `HERODATA`, `NDATA`, ...).
- `INTRO.LBX` and the nine `*FIN.LBX` files are Smacker videos (`SMK2`), not
  archives.
- Loading searches the current directory, then `secondary_drive_path`.
