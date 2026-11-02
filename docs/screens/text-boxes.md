# Text and box helpers

Port: `internal/game/textboxes.go`. Small game routines (utility module 55
and `Print_Paragraph_Centered_Vertically_`) that screens use for text and
framed boxes. Coordinates are shorts throughout.

## Shadowed text

`_Print_Shadow_Down_(x, y, c, s)` (0x900ce): save the eight
`current_colors`; set normal and current colours to `{saved[0], c, c, c, c,
c, c, c}` (shade 0 is kept) and `Print_(x+1, y+1, s)`; restore normal and
current colours to the saved ones and `Print_(x, y, s)`. `Print_` still runs
the font's own shadow/outline passes if `shadow_flag` is set.

- `_Print_Centered_Shadow_Down_` (0x90089): x − `Get_String_Width(s)`/2
  (truncated);
- `_Print_Right_Shadow_Down_` (0x900b2): x − width (not width − 1 as
  `Print_Right`).

## Change_Font_Style_(style) (0x8ebbb)

`Set_Font_Style(style, current_colors)` and put `shadow_flag` back. Spacing,
line feed and space width are reset to the style's (that is
`Set_Font_Style`). It takes one argument; callers leave other registers set
for the call that follows.

## Print_Paragraph_Centered_Vertically_(x, y, mw, h, s, type) (0xc5ed3)

`mh = _Get_Paragraph_Max_Height_(mw, s)`; `_Print_Paragraph_(x, (short)(y +
(ushort)h/2 − mh/2), mw, s, type)` (both halves truncated). See
`docs/engine/paragraphs.md`.

## Draw_Highlight_Box_(x, y, w, h, c[4]) (0x8f0bd)

With r = x + w, b = y + h, eight `Line_` calls in this order:

| line | colour |
|---|---|
| (x, y)–(r, y), (r, y)–(r, b) | c[3] |
| (x+1, y+1)–(r−1, y+1), (r−1, y+1)–(r−1, b−1) | c[2] |
| (x, y+1)–(x, b), (x, b)–(r, b) | c[0] |
| (x+1, y+1)–(x+1, b−1), (x+1, b−1)–(r−1, b−1) | c[1] |

So the bottom-right corners take the bottom colours.

## Draw_Darkened_Box_(x, y, w, h, c) (0x8f055)

x += 2, y += 2; `_Darken_Fill_(x+1, y+1, x+w−1, y+h−1)`;
`Draw_Highlight_Box_(x, y, w, h, {c, c+1, c+2, c+3})` (byte adds).

`_Darken_Fill_(x1, y1, x2, y2)` (0x90006): when clipping is on, cut to the
window; `Tint_Fill(x1, y1, x2, y2, 0)` (glass colour 0). Port: `darkenFill`
in `zoom.go`.

## Diagonal_Line_Fill_(x1, y1, x2, y2, step, main, side) (0x8ec70)

`Save_Window_`, remember `clipping_flag`. step 0 → 16; if `(640/step)*step
!= 640` or `(480/step)*step != 480` → 16. `Set_Window_(x1, y1, x2, y2)`,
`Clip_On_`. With a = i·step, rows = 480/step, cols = 640/step, four loops of
three `Line_` calls each (side, side, main):

| loop | i < | side 1 | side 2 | main |
|---|---|---|---|---|
| 1 | cols | (a−1, 0)–(a+479, 480) | (a+1, 0)–(a+481, 480) | (a, 0)–(a+480, 480) |
| 2 | rows | (−1, a)–(479−a, 480) | (1, a)–(481−a, 480) | (0, a)–(480−a, 480) |
| 3 | rows | (a−1, 0)–(1, a) | (a+1, 0)–(1, a) | (a, 0)–(0, a) |
| 4 | cols | (a−1, 480)–(a+479, 0) | (a+1, 480)–(a+481, 0) | (a, 480)–(a+480, 0) |

The whole screen is hatched in both diagonals and the window keeps what
falls inside. Loop 3's side lines all end at (1, a) instead of running
beside the main line, so near the top-left they fan out (kept). Then
`Restore_Window_` and `Clip_On_`/`Clip_Off_` as before.

## Get_LBX_Message_(i, buf) (0x8f6df)

`strcpy(buf, H_Message_(i))`. Port: `lbxMessage` returns the string.
