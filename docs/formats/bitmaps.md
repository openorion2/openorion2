# Bitmap operations

Code: `internal/gfx/bitmap.go`. Source of truth: `WLIBS\animate\bitmap.c`
routines and the assembly pixel loops they call, plus two MOX helpers. A
"bitmap" is a kind-1 picture (see `pictures.md`) as `Create_Bitmap_Header`
builds it: one frame, pixels at +0x10.

Several routines address pixels at `bitmap + 0x10` directly and ignore the
frame table; others use `frame_offsets[current_frame]`. Both are noted below.

| Routine | Address | Go |
|---|---|---|
| `Create_Blank_Bitmap(w,h,bm)` | 0x12d8f5 | `NewBlankBitmap` |
| `Create_Bitmap_Header(w,h,bm)` | 0x12d943 | (inside `NewBitmap`) |
| `Get_Bitmap_Size(w,h)` | 0x12daa4 | `BitmapSize` |
| `Copy_Bitmap_To_Bitmap(dst,src)` | 0x12e0b2 | (inside `Outline`) |
| `Outline_Bitmap(dst,src,color,ignore_glass)` | 0x12e64f | `dst.Outline` |
| `Outline_Bitmap_Pixels_` / `_No_Glass_` | 0x147b89 / 0x147c6a | `outlinePixels` |
| `Mask_Out_Bitmap_With_Bitmap(x,y,tgt,mask,inv)` | 0x12efbd | `tgt.MaskOut` |
| `Mask_Bitmap_Pixels_` / `_Inverse_` | 0x147f3e / 0x147fdf | (inside `MaskOut`) |
| `Draw_Picture_To_Bitmap(dst,src)` | 0x12f578 | `dst.DrawPictureToBitmap` |
| `Add_Picture_To_Bitmap(x,y,src,dst)` | 0x12f7e6 | `dst.AddPicture` |
| `Get_Bitmap_Actual_Size(bm,&x1,&y1,&w,&h)` | 0x131528 | `ActualSize` |
| `Replace_Color(pict,from,to)` | 0x13719f | `ReplaceColor` |
| `Swap_Bitmap_Colors_` (MOX) | 0x8fe88 | `SwapColors` |
| `Tint_Bitmap_Colors_` (MOX) | 0x8ff08 | `TintColors` |

## Creation

`Create_Bitmap_Header`: width, height, current 0, total 1, repeat/mfs/groups
0, mask 1, `frame_offsets[0] = 0x10`. `frame_offsets[1]` is not written.
`Create_Blank_Bitmap` adds `memset(bm+0x10, 0, w*h)`.
`Get_Bitmap_Size = (w*h + 0x13) / 4 * 4` bytes (header plus pixels, rounded
up to a dword).

## Draw_Picture_To_Bitmap (0x12f578)

`Create_Blank_Bitmap(src.w, src.h, dst)`, then:

- src kind 1: copy `w*h` bytes of `frame_offsets[current_frame]` to dst+0x10.
- any other kind (read as a sprite, kind 2 included): render frames
  `start .. current_frame` inclusive, `start = (current / mfs) * mfs`, or 0 if
  `mfs` (byte 9) is 0. Each frame: `y = u16 at +2`, `pos = y*w`, word index 2;
  while `y < h`: `(s16 count, s16 skip)`; count 0: `y += skip`, row start
  `+= w*skip`, `pos =` row start; otherwise `pos += skip`, copy `count`
  bytes **without a transparency test** (zero bytes overwrite earlier frames),
  then word index `+= (count+1)/2` (realigned relative to the frame start).
  No horizontal clipping: a run past the right edge continues on the next row.
- src == dst ends up blank (the header is rewritten before src is read).

## Add_Picture_To_Bitmap (0x12f7e6)

dst must be kind 1, otherwise `Exit_With_Message("Add_Picture_To_Bitmap:
target must be a bitmap")` (Go returns an error). Uses src's current frame;
dst pixels at dst+0x10.

`x2 = x + src.w - 1`, `y2 = y + src.h - 1` (int16). Nothing is drawn unless
`x < dst.w && y < dst.h && x2 >= 0 && y2 >= 0`. `skip_x = max(-x, 0)`,
`start_x = max(x, 0)`, `skip_y = max(-y, 0)`, `y = max(y, 0)`;
`aw = x2 < dst.w-1 ? x2-start_x+1 : dst.w-start_x`, likewise `ah`.

- src kind 1: copy the `aw x ah` window from `(skip_x, skip_y)`, skipping 0.
- otherwise sprite: from `y = top`, pairs are consumed while `y < skip_y`
  (count 0: `y += skip`; else skip the run); the row start is
  `dst.w*y + x` (unclamped `x`) plus `(y - skip_y)*dst.w` when the skip
  overshot. Then `y -= skip_y`; while `y < ah` runs are clipped to
  `[row+skip_x, row+skip_x+aw)` and copied **including zeros**.

## Outline_Bitmap (0x12e64f)

`src = frame_offsets[current]` of the source. If target != source:
`Copy_Bitmap_To_Bitmap` (a header plus a `w*h` copy of the source's current
frame) and write to target+0x10; else read and write the source frame in place.
Then the pixel loop, `ignore_glass ? _No_Glass_ : plain`:

- A pixel is *solid* if it is not 0, not `color`, and (No_Glass only) below
  0xf0. Everything else is *empty*.
- Horizontal pass, row by row: solid after empty paints the previous pixel;
  empty after solid paints the current one. State resets at each row start,
  so edges never get a pixel outside the row.
- Vertical pass, column by column, the same with the pixel above.
- **Quirk**: both passes count `width*width` pixels (`mul eax` where
  `Bitmap_Aura_Pixels_` has `mul ebx`), so the bitmap is treated as
  `width x width`. A tall bitmap loses its lower rows; a wide one would read
  and write past its end (Go drops those accesses). The game calls it on 35x35
  bitmaps.
- In place and out of place give the same result: painted pixels take
  `color`, which counts as empty.

## Mask_Out_Bitmap_With_Bitmap (0x12efbd)

Both pictures use their current frame. Returns unless `y <= tgt.h && x <=
tgt.w`. Offsets: mask start skips `-y*mw` and `-x` for negative coordinates;
target start `tgt.w*max(y,0) + max(x,0)`. `x1 = max(x,0)`, `x2 = min(x+mw,
tw)` (strict `<`), `y1 = max(y,0)`, and **`y2 = y + mw < th ? y + mh : th`**
(mask width in the test). Per-row strides are `mw - width` and `tw - width`.
`Mask_Bitmap_Pixels_` clears target pixels where the mask is nonzero, the
`_Inverse_` one where it is zero. A zero or negative width or height
makes the assembly loop ~2^32 times; Go does nothing.

## Replace_Color (0x13719f)

In place, every frame, by kind:

- 0: sprite runs as above (`y = u16 at +2`, `y < height`, signed skip),
  each run byte compared zero-extended to 16 bits with `from` (int16).
- 1: `w*h` bytes of each frame compared as bytes (low byte of `from`).
- 2: per bank of five: `(skip, count)` dword pairs, of which only the low
  words are read; ends at both 0; then `int16(count << 2)` bytes compared
  like kind 0.
- 3: `No_Replace_Mask_Msg` (Go returns an error).

## Get_Bitmap_Actual_Size (0x131528)

Over the `w*h` pixels at +0x10: `x1 = y1 = 1000`, `maxx = maxy = 0`; each
nonzero pixel lowers x1/y1 and raises maxx/maxy. `w = maxx - x1 + 1`,
`h = maxy - y1 + 1` (int16); a blank bitmap gives -999 x -999.

## Swap_Bitmap_Colors_ (0x8fe88), Tint_Bitmap_Colors_ (0x8ff08)

`Swap(x1,y1,x2,y2, from, to, n, bm)`: for `y1 <= y < y2`, `x1 <= x < x2`,
pixel `bm + 0x10 + y*w + x`: the first `k < n` with `pix == from[k]` sets
`to[k]`. `Tint(x1,y1,x2,y2, whole, value, bm)`: `whole` replaces the rectangle
by the full bitmap; every nonzero pixel gets `+= value` (byte wrap).
