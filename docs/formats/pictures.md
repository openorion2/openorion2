# Pictures and palettes

Decoder: `internal/gfx`. Source of truth: `WLIBS\animate\animate.c` (`Draw`,
`Set_Animation_Palette`), `bitmap.c` (`Create_Bitmap_Header`), and the
assembly blitters `Draw_Animated_Sprite_`, `Draw_Bitmap_Sprite_`,
`Draw_Screen_Background_Off` (`s_animation_header`, `s_palette_header`).

## Header

```
0x00 int16  ddx (width)
0x02 int16  ddy (height)
0x04 int16  current_frame
0x06 int16  total_frames
0x08 uint8  repeat_frame
0x09 uint8  multi_frame_full_store
0x0a uint8  group_count
0x0b uint8  mask
0x0c uint32 frame_offsets[total_frames+1]   relative to the header
```

`mask`: bits 0–1 frame kind; 0x08 glassed (translucent through the glass
remap table); 0x10 palette chunk present; 0x20 hold mode. After drawing,
`current_frame` advances; at the end it wraps to `repeat_frame` (or, with
0x20, holds per the logic in `Draw`).

Palette chunk, right after the offset table when 0x10 is set: `int16 start,
int16 count`, then `count` four-byte colours applied from `start`.

## Frame kinds

- **0 sprite.** `u16` (ignored by the blitter), `u16 top` (blank rows), then
  `(u16 count, u16 skip)` pairs until `height - top` rows are consumed. A zero
  count moves `skip` rows down and back to column 0. Otherwise skip `skip`
  pixels (transparent), copy `count` bytes, then realign the source to an
  even address (frames sit at even addresses).
- **1 bitmap.** `width*height` bytes, colour 0 transparent. Single frame at
  offset 0x10 when built by `Create_Bitmap_Header`.
- **2 background.** Full 640×480 screen as five banks of 16384 dwords. Each
  bank: `(u32 skip, u32 count)` dword pairs, then `count` dwords of pixels,
  closed by `(0, 0)`.
- **3** refresh mask, never drawn.

## Palettes

`Load_Palette(entry, start, end)` reads `FONTS.LBX` entry `entry+1` (entries
1–13 are palettes, entry 0 is fonts). 256 colours of four bytes: a changed
flag (always 1), then 6-bit red, green, blue. A mouse palette follows at
+0x400. 6-bit values are widened as `v<<2 | v>>4`.
