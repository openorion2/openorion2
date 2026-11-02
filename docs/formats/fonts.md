# Fonts

Reader: `internal/font`. Source of truth: `WLIBS\fonts\fonts.c`
(`Load_Font_File`, `Set_Font`, `Print_Character`, `Get_String_Width`,
`struct s_font_header`) and the assembly blitter `Print_Character_ASM_`.

## Entry layout

`FONTS.LBX` entry 0 (entries 1–13 are palettes, see `pictures.md`):

```
0x0000  0x40 bytes   glass_colors
0x0040  0x235c bytes s_font_header (copied whole into font_header)
0x239c               glyph data (font_data)
```

Offsets inside `s_font_header` (relative to 0x40):

```
0x52c  short[8]            base_height        glyph rows
0x53c  short[8]            horizontal_spacing pixels after every glyph
0x54c  short[8]            vertical_spacing   line_feed = height + this
0x55c  char[6][256]        font_widths
0xb5c  uint32[6][256]      data_offsets       relative to font_data
```

The fields before 0x52c are the "current" copies `Set_Font` fills in at run
time. Six styles are present: heights 6, 9, 12, 13, 15 and 23. `Set_Font`
copies widths and offsets for codes 0..254 only.

## Glyphs

A glyph is `base_height` rows. Each row is a byte stream:

- `b < 0x80`: one pixel in `current_colors[b-1]` (eight shades per font);
- `b > 0x80`: skip `b & 0x7f` transparent pixels;
- `0x80`: end of row.

`Print_Character` (0x121deb) draws at (x, y) with the current offsets and
base height and returns `x + current width + current spacing`.

## Colours

`Set_Font(style, n)` sets all eight shades to `default_font_colors[n]`, and
that table (data 2:0xc4ec, 16 shorts) is the identity, so the shades are
palette index `n`. `Set_Font_Style(style, colors)` takes eight explicit
shades for the normal and current colours; highlight and special colours
come from `Set_Font_Highlight_Colors` / `Set_Font_Special_Colors`. The remap
variants (`Set_Remap_Font_Style*`) are not decoded yet.

## Current metrics

`Set_Font_Style` (0x120bb5) copies the style's tables into the "current"
fields of `font_header` (globals at 0x1b3e7c + offset), and everything that
prints or measures reads those copies (port: `Printer`, fields set in
`SetStyle`):

| Global | Field | Set by `Set_Font_Style` to | Changed by |
|---|---|---|---|
| 0x1b61e6 | `current_font_style` | style | `Get_Font_Style` 0x8f64c, `Get_Current_Font_Style` 0x122201 read it |
| 0x1b3e84 | `height` | base height | `Set_Font_LF(n)`: base height + n; `Get_Font_Height` 0x122259 |
| 0x1b3e86 | `shadow_flag` | 0 | shadow setters |
| 0x1b3ea0 | `current_base_height` | base height | glyph rows drawn |
| 0x1b3ea2 | `current_line_feed` | height + vertical spacing | `Set_Font_LF(n)` (0x120e24): n |
| 0x1b3ea4 | `current_vertical_spacing` | vertical spacing | `Get_Font_Vertical_Spacing` 0x122286 |
| 0x1b3ea6 | `current_horizontal_spacing` | horizontal spacing | `Set_Font_Spacing(n)` (0x120e8c); `Get_Font_Spacing` 0x8f645 |
| 0x1b3ea8 | `current_font_widths[256]` | widths | `Set_Font_Spacing_Width(n)` (0x120e60): width of ' ' (0x1b3ec8) |
| 0x1b3fa8 | `current_data_offsets[256]` | offsets | |

`Set_Font_LF` really stores n as the line feed and base + n as the height.
`height` (not the base height) is what clipping and scan-list marking use.
The current spacing is read by `Get_String_Width`, `Print_Character`,
`Print_Clipped_Character` and the bitmap printers alike, so it moves glyphs,
justification (`Print_Full`), centring and right alignment together; the
style's own spacing is only read by `Set_Font_Style`.

## Strings

`Get_String_Width` (0x12066f) sums current `width + current spacing` over
characters from 0x20, ignores codes below 0x20, and stops at 0x00 or 0x15.
Callers take the low 16 bits (the sum's high half is garbage from the string
pointer), so the port truncates to int16.

`Print_String(x, y, s, colors_ok, full)` (port: `Printer.String`):

| Code | Effect |
|---|---|
| 0x00, 0x15 | end of string |
| 0x01 | normal colours (if `colors_ok`) |
| 0x02, 0x04 | highlight colours (if `colors_ok`) |
| 0x03 | special colours (if `colors_ok`) |
| 0x08 | ignored |
| 0x0d, 0x14 | new line: y += line_feed, x = start x |
| 0x19, 0x1d | tab: x = start x + next byte |
| anything else | drawn as a glyph, including the other codes below 0x20 |

Colour changes persist until changed again. With `full` > 0 the line is
justified: `full - width` extra pixels go to the spaces before the first
line break, `extra / spaces` each and one more to the first `extra % spaces`.

## Print_Display, shadows and outlines

Every printer ends in `Print_Display(x, y, s, full)` (port: `Printer.Display`;
`Print` = full 0, `Print_Full` clamps negative to 0, `Print_Centered` starts
at `x - width/2`, `Print_Right` at `x - (width - 1)`).

`shadow_flag` is set by the style setters: 1 `Shadow_Down`, 2 `Shadow_Up`,
3 `Shadow_Heavy`, 4 `Outline`, 5 `Outline_Heavy` (`Set_Font_Style` clears it).
When nonzero, shades 2..8 become `outline_color`, shade 1 becomes
`alias_outline_color` (both from `Set_Outline_Color`), and the string is
printed with colour codes disabled at these offsets, in this order:

- unless 2: (+1,+1), (0,+1), (+1,0)
- unless 1 or 3: (−1,0), (−1,−1), (0,−1)
- 3 and 5: (+2,+2), (+1,+2), (+2,+1)
- 4 and 5: (+1,−1), (−1,+1)
- 5: (+2,0), (0,+2)

Those passes use `Print_Character_No_Alias_ASM`, which differs from the plain
blitter only in leaving glyph value 0 transparent (the plain one would read
the byte before the colour table); no shipped glyph contains a 0. Then the
shades are restored to the normal colours and the text is printed with colour
codes enabled.

`Print_Display` then marks `Add_Square(x-d, y-d, x+d+width (or x+d+full),
y+d+height)` clamped to the screen when the refresh flag is at least 2 (d =
2 for shadows 3 and 5, 1 for the others, 0 without shadow). The mark is the
same whether or not clipping is on.

## Clipped printing

With `clipping_flag` (0x1845d8) == 1, `Print_Display` calls
`Clipped_Print_String` instead of `Print_String` for every pass, shadows
included (port: `Printer.Clip`; `engine.Text` sets it from `Video.Clipping`
and the window on each call). The window is `screen_window_x1, x2, y1, y2`
(words at 0x1bba4c, 0x1bba4e, 0x1bba50, 0x1bba52), inclusive.

`Clipped_Print_String` (0x122309): if `y > wy2` or `y + height < wy1`
(first line only) print nothing and return 1. Otherwise as `Print_String`,
with every glyph going through `Print_Clipped_Character` (never the
`_No_Alias` blitter).

`Print_Clipped_Character(x, y, c)` (0x12260f), w = current width, h =
`height`, sp = current spacing:

- `c > 0x80`: return x — not drawn and not advanced over (the plain printer
  draws them);
- `x >= wx1 && x+w <= wx2 && y >= wy1 && y+h <= wy2`: `Print_Character`
  draws the whole glyph; return x + w + sp;
- otherwise next = x + w + sp; left: sx = wx1 − x, return next if `w <= sx`,
  w −= sx, x = wx1; right: if `x+w > wx2`, w = wx2 + 1 − x, return next if
  `w < 1`; top: sy = wy1 − y, y = wy1; maxy = h, or `wy2 + 1 − y` (the cut
  y) if `y+h > wy2`; `Print_Clipped_Letter`; return x + w + sp from the cut
  values.

`Print_Clipped_Letter(x, y, c, sx, w, sy, maxy)` (0x1227ec): maxy =
min(maxy, base height); skip sy glyph rows; draw `maxy − sy` rows from y,
only glyph columns sx..sx+w−1, a byte below 0x81 writing
`current_colors[b−1]` (address 0x1b3e7b + b; the "palette_block + 3" in the
decompiler is that address under another symbol) with no zero test.

Quirks the port keeps:

- a glyph cut at both top and bottom loses sy rows at the bottom (maxy is
  measured from the cut y, then sy is subtracted again);
- after a right-cut glyph the returned x is wx2 + 1 + sp, not the unclipped
  x + w + sp; it only shows with negative spacing or in the return value;
- the letter blitter indexes glyph data with a signed 16-bit offset
  (`movsx word`), so a cut glyph whose data lies at 0x8000 or beyond reads
  memory before `font_data`. In the shipped font that is style 5 from 'W'
  (whose data crosses 0x8000) through 0x7f. The port returns
  `ErrGlyphOffset` instead of guessing;
- glyph pixels past the glyph's width are cut in the clipped path but not by
  `Print_Character`; only style 0 codes ≥ 0xa0 have such pixels, and those
  are above 0x80.

Without these, clipped output equals unclipped output cut to the window
(`TestClippedMatchesWindow`).

## Printing into a bitmap

`Print_To_Bitmap(x, y, s, bitmap)` (0x122a6e), `Print_Full_To_Bitmap`
(0x122a1f, negative right side → 0), `Print_Centered_To_Bitmap` (0x1229c5,
x − width/2), `Print_Right_To_Bitmap` (0x12297b, x − (width − 1)) all end in
`Print_Display_To_Bitmap(x, y, s, full, bitmap)` (0x122aab). Port:
`Printer.ToBitmap` etc. on a `Target` whose `Pix` is the frame and `Stride`
the bitmap width.

`Print_Display_To_Bitmap` exits with "Print only works on bitmaps" unless
the header's frame kind (byte 0x0b & 3) is 1. It runs the same shadow and
outline passes as `Print_Display` through `Print_String_To_Bitmap`, ignores
`clipping_flag` and marks no scan list. The frame drawn into is the current
one: `bitmap + frame_offsets[current_frame]`, `width*height` bytes.

`Print_String_To_Bitmap` (0x122d8a) differs from `Print_String`:

- prints nothing if `y > bitmap height`, `y + height <= 0`,
  `x > bitmap width` or `x + string width <= 0`;
- 0x0d, 0x14, 0x15, 0x19, 0x1d and 0x00 all end the string (no line breaks
  or tabs); 0x08 is ignored, 0x01..0x04 switch colours as usual;
- x advances by current width + spacing after every other code, including
  those above 0x80.

`Print_Character_To_Bitmap` (0x123070): codes above 0x80 draw nothing.
Otherwise it cuts like `Print_Clipped_Character` against the bitmap
(left at 0; right when `x + w > width − 1`, w = width − x; top at 0; maxy =
`height − y` when `y + h > height − 1`) and always calls
`Print_Clipped_Letter_To_Bitmap` (0x1231b1), the bitmap twin of
`Print_Clipped_Letter` that does test for zero (glyph value 0 transparent).
So the 16-bit offset and both-edges quirks apply to every bitmap glyph,
even ones wholly inside: style 5 from 'W' on never prints into a bitmap.

## Oddities

- Style 5 codes 126 and 127 have width 0 and an offset equal to the length of
  the glyph data, so the engine would read past the font; they are treated as
  empty.
- Style 0 alone has data for codes 128–254, but it does not look like glyphs
  when drawn 6 rows high. The game never prints them: style 0 is set (eax =
  0 before `Set_Font_Style_*`) only in `Print_Main_Screen_Data_`,
  `Draw_Turn_Count_`, `Print_Colony_Outpost_Marker_`,
  `Draw_Colony_Summary_For_Colony_`, `Draw_Ship_Status_`,
  `Draw_Missile_Status_`, `Draw_Shield_Arcs_`, `Draw_Captains_Picture_`,
  `Print_Officer_Data_` and the uncalled `Set_Foreground_Color_`; those print numbers,
  ESTRINGS/RSTRING texts and weapon names (no byte above 0x7f in either
  file), and leader, ship and star names: input fields take only 0x20..0x7e
  (`Get_Input_`, `engine/fields.go`), and no name in the ten saves or any
  traced turn has a high byte. The executable loads only FONTS.LBX
  (IFONTS.LBX is never named), and the clipped and bitmap printers skip
  codes above 0x80 anyway.
