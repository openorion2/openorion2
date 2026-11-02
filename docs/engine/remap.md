# Planet palettes and remapped drawing

Go: `internal/engine/remap.go`. Sources: `WLIBS\palette\reduce.c` (module
191), `WLIBS\palette\remap.c` (module 177), and the game's own masked
blitters (MOX, around 0xa5050). The system display (`docs/screens/system-display.md`)
is the user: every planet picture has its own palette chunk, all of them are
squeezed into colours 0xc0..0xff, and each planet is drawn through a remap
table and a light mask.

## State

| Address | Name | Go |
|---|---|---|
| 0x1be354 | `reduced_palette_list`, `u8[512][4]` (flag, R, G, B) | `remapState.reduced` |
| 0x1beb56 | `reduced_palette_count`, s16 | `remapState.reducedCount` |
| 0x1bd154 | `color_weight`, `u8[512]` (indexed by sorted position) | `reducer.weight` |
| 0x1bd354 | `cube_list`, `s_cube[256]` | `reducer.cubes` |
| 0x1bdf54 | `HistPtr`, `u16[512]` (indices into the list) | `reducer.hist` |
| 0x1beb54 | `ncubes`, u16 | `reducer.ncubes` |
| 0x1bc79c | `picture_remap_color_list`, `u8[256]` | `remapState.picture` |
| 0x19c814 | `_remap_gray_mask_palette`, `u8[10][256]` | `remapState.gray` |
| 0x1823e0 | `_remap_gray_percentages`, `u8[10]` = 10, 20, …, 100 | `RemapGrayPercentages` |
| 0x1bc2a8 | `animation_header`, the picture engine routines last touched | `header` argument |
| 0x19d224 | `_mask_header` | — |

`s_cube` (12 bytes): `s16 lower, upper, level; u8 rmin, rmax, gmin, gmax,
bmin, bmax`. A cube is the range `HistPtr[lower..upper]`.

## Reduced palette groups

- `Clear_Reduced_Palette_Groups` (0x13aac1): count = 0. The list keeps its
  contents.
- `Add_Reduced_Palette_Group(pict)` (0x13aae9): fatal unless mask has 0x10.
  The chunk is at `pict + 0x10 + 4*frames`; for each of its `count` colours
  copy bytes 1..3 (R, G, B) to `list[n]`, `n++`; fatal once `n >= 0x200`
  (after storing). The flag byte of the list entry is not written.
- `Compress_Reduced_Palette_Groups(start, count)` (0x13ac01): if `n > count`
  then `Reduce_Palette(list, 0, n, count)`. Then, always, for i < count:
  `current_palette[start+i] = {1, list[i].R, list[i].G, list[i].B}`. With
  `n <= count` the entries past n are whatever an earlier call left (zero at
  start-up). No `Set_Palette_Changes`; the flag byte is written directly.

### Reduce_Palette(list, start, total, count) (0x139af3)

Median cut. Fatal if total > 0x200 or count > 0x100.

1. `Initial_Cube` (0x13a7cc): `HistPtr[i] = i`; cube 0 = {0, total−1, level
   0}, `Shrink_Cube`; ncubes = 1.
2. While ncubes < count: `c = Find_Next_Lowest_Cube_Level()`; stop on −1;
   `axis = Find_Longest_Axis(c)`; `Sort_Cube_Points(list, &HistPtr[lower],
   upper−lower+1, axis)`; `Split_Cube(list, c, axis, total)`.
3. For i < ncubes: `list[start+i] = {1, (rmin+rmax)/2, (gmin+gmax)/2,
   (bmin+bmax)/2}`. For ncubes ≤ i < count: copy the RGB of entry i−1 (flag 1).
   The result overwrites the input list in place (start is 0).

- `Shrink_Cube` (0x13a993): min/max of each channel over the cube's points
  (unsigned bytes; starts at 0xff/0).
- `Find_Next_Lowest_Cube_Level` (0x13a84f): over cubes with lower ≠ upper and
  level < 30000, the first one whose red, then green, then blue extent
  (byte difference) is strictly larger than the best so far (best starts at
  0, so cubes of a single colour are never split). −1 if none.
- `Find_Longest_Axis` (0x13a711): 0 (red) if r ≥ g and r ≥ b, else 1 (green)
  if g ≥ r and g ≥ b, else 2 (blue).
- `Sort_Cube_Points` (0x13a4a9): bubble sort ascending on the channel
  (`Swap_Short_` on `pal[p[j]] > pal[p[j+1]]`), at most count passes, stops
  when a pass swaps nothing. Stable.
- `Split_Cube` (0x139d7e): for sorted positions k = lower..upper, `w[k]` =
  the gap to the next point at k = lower, to the previous at k = upper, else
  the larger of both (byte arithmetic); 0 becomes 1. `half = Σw / 2`;
  `slice = lower`, `half −= w[lower]`; while `half > 0 && slice < upper`:
  `slice++`, `half −= w[slice]`. New cube ncubes = {slice+1, upper, level+1};
  the old one becomes {lower, slice, level+1}; shrink both; ncubes++.

## Picture remap colours

`Calculate_Picture_Remap_Colors(pict)` (0x133c9c): fatal unless mask has
0x10; `animation_header = pict`; `Create_Picture_Remap_Palette(chunk+4,
picture_remap_color_list)` (0x133d16): `Sort_Palette_List()`; `remap[i] = i`
for all 256; for each chunk colour i: `remap[chunk.start + i] =
Find_Closest_Color(R, G, B)` over the current palette (which by then holds
the compressed colours at 0xc0..0xff). `Get_Remap_Palette_Color(c)`
(0x133de1) is `remap[c]`.

`Find_Closest_Color` and `Sort_Palette_List` are in `glass.go`
(`docs/screens/main-screen-zoom.md`); colour 0 is never returned unless
nothing is found.

## Calculate_Remap_Gray_Palettes_ (0xa5050)

`Sort_Palette_List()`; for level l = 0..9, `f = 100 − pct[l]` (pct byte at
0x1823e0+l), for colour c = 0..255: `gray[l][c] = Find_Closest_Color(R*f/100,
G*f/100, B*f/100)` (C int division, results as bytes). Level 9 is 100 %:
the nearest non-zero black.

## Remap_Draw_With_Mask_(x, y, pict, mask) (0xa512e)

Port: `RemapDrawWithMask(x, y, pict, mask, header)`, where `header` is
`animation_header` on entry.

1. If `clipping_flag == 1`, test against the window **with the size of
   `animation_header`, not of pict** (the global still points at whatever
   the engine last touched; in `Draw_Planet_With_Pierced_Ring_` that is the
   288×154 ring bitmap just drawn): `x > wx2 || y > wy2 || x+hw−1 < wx1 ||
   y+hh−1 < wy1` → return, frames not advanced. Not fully inside → clipped.
2. `animation_header = pict`, `_mask_header = mask`. Frame pointers: each
   picture's current frame (`pict + offsets[cur]`).
3. Unclipped: if `screen_refresh_flag >= 2`: `Add_Square_To_Scan_List(x, y,
   x+w−1, y+h−1)` and **again at (x+100, y+100, x+100+w−1, y+100+h−1)**
   (pict's size). Kind 0 → `Remap_Draw_Animated_Sprite_Mask_`; kinds 1..3 →
   `Exit_With_Message` (jump table 0xa510e). Then both pictures advance one
   frame with Draw's repeat/hold rules (mask first; independent).
4. Clipped: if `dot_stencil_flag` (0x1845da) → no drawing, frames advance
   (not modelled: the port has no stencil flag). Else if refresh ≥ 2:
   `Add_Square_To_Scan_List(x, y, x+hw−1, y+hh−1)` (header's size,
   unclipped). Kind 0 → `Remap_Clipped_Draw_Animated_Sprite_Mask_`, then
   **return without advancing either frame**; others fatal (table 0xa511e).

Per pixel both blitters write `v = remap[src]`, and if the mask byte `m ≠ 0`
`v = *(0x19c714 + m*256 + v)` = `gray[m−1][v]`. Every pixel of a run is
written (no colour-0 transparency). Mask bytes above 10 would read past the
tables; the port leaves `v` unchanged there (the planet masks hold 0..10).

### Remap_Draw_Animated_Sprite_Mask_(x, y, frame, maskframe) (0xa53dc)

`W, H` from `animation_header` (= pict). `row = u16 frame[+2]`, `line =
(y+row)*640 + x`, `dst = line`, word index 2. While `H > row`: read `count,
skip` (s16); count 0 → `row += skip`, `line += skip*640`, `dst = line`;
else `dst += skip`, for k < count: `m = maskframe[W*row + skip + k]` (**skip
is this run's own skip word**, not the column reached, so a second run on a
row reads the mask at the wrong column), write, `dst++`; word index +=
`(count+1)/2` — data are word-aligned relative to the frame start.

### Remap_Clipped_Draw_Animated_Sprite_Mask_(x, y, frame, maskframe) (0xa54f2)

`W, H` from `animation_header`. `cx1 = max(x, wx1)`, `skipX = cx1 − x`;
`cy1 = max(y, wy1)`, `skipY = cy1 − y`; `width = min(x+W−1, wx2) − cx1 +
1`; `height = min(y+H−1, wy2) − cy1 + 1`; `line = cy1*640 + x`. Starting
at `row = top`, records are skipped while `row < skipY` (count 0 adds the
u16 skip to row; data runs are passed over). If `row > skipY`, `line +=
(row−skipY)*640`. Then **`row −= skipY`: rows count from the clipped top**,
so the mask row is off by the rows cut away above the window. Per line `left
= line + skipX`, `right = left + width`. While `row < height`: count 0 →
`row += skip`, `line += skip*640`, `dst = line`; else `dst += skip`, `end =
dst + count`; if `dst < right`: cut `min(left−dst, count)` pixels on the
left (advancing source and k), draw up to `min(right, end)` with the same
mask index `W*row + skip + k`. After a right-clipped or skipped run `dst`
is not moved to the run's end (harmless: later runs are past `right`).
Word index += `(count+1)/2`.

## Notes

- `Get_Palette_Animation_Color` (0x133bae) reads one colour of a picture's
  chunk (fatal outside it); not needed yet.
- The `remapState` lives in a map keyed by `*Engine` until `engine.go` gets a
  field for it.
