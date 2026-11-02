# Paragraphs

Ports: `internal/engine/paragraph.go`, `internal/engine/fmtpara.go`. Two
systems share the name:

- the game's simple word-wrapping printer, `_Print_Paragraph_Bill_` and its
  wrappers (module 122, around 0x1035af–0x103990);
- the formatted paragraph system (`D:\MOX\FMTPARA.C`, module 124,
  0x103bc4–0x106cac): control codes, functions, tabs, variables and inserts,
  printed glyph by glyph. See *Formatted paragraphs* below.

WLIBS `fonts\paragrph.c` (`Print_Paragraph_` 0x13e246, `Mark_Paragraph_`,
exclusion areas) is a third, separate printer; the routines below do not
call it.

## Globals

| Address | Name | Use |
|---|---|---|
| 0x1aceb8 | `__pp_max_height` (short) | set by `_Print_Paragraph_Bill_` |
| 0x1aceba | `__pp_max_width` (ushort) | reset and raised by every Bill call |
| 0x1acebc | `_variable[16]` (int) | paragraph variables |
| 0x1acf18 | `_para_global` (0x5e bytes) | +0: extent x1, y1, x2, y2 (shorts); +8: pointer per variable; +0x48: type byte per variable; +0x58: user call; +0x5c: test flag; +0x5d: plural flag |
| 0x1838f7 | `_tabs` (0x16 bytes) | short count, then ten short stops |
| 0x1acf76 | `_insert_case` (short) | 0 as is, 1 upper, 2 lower, 3 capitalise the next letter |
| 0x1acf78 | char[32] | ordinal insert buffer |
| 0x1acf98 | char[16] | number insert buffer (`itoa_`) |
| 0x1acfa8 | 13 bytes | style saved by function `^` (zero until first saved) |
| 0x1acfb5 | short | variable index of function `=` |
| 0x1acfb7 | int | user call argument |
| 0x1acfbb, 0x1acfbc | bytes | `Print_Line_`'s last glyph style and attribute |

Both `__pp_*` keep their value when a call rejects its arguments, so
`_Get_Paragraph_Max_Height_`/`_Width_` then return the previous result.
Port: `Engine.Paragraphs()`.

## Reset

`Initialize_Formatted_Paragraph_System_` (0x103d1b), tail-called from
`Bill_Default_` (0x10206e, which first calls `Set_Plyr_Info_Btns_(4)` when
byte 0x199cb0 < 2): `memset(_tabs, 0, 0x16)`, `memset(_para_global, 0,
0x5e)`, then `Reset_A_Paragraph_Variable_(i)` for i = 0..15.

`Reset_A_Paragraph_Variable_(i)` (0x104b6b): for `(ushort)i < 16`, pointer
i = `&_variable[i]`, `_variable[i] = 0`, type i = 2.
`Reset_Paragraph_Variables_` (0x104b2f) is the same loop inlined.

## Entry points

| Routine | Address | Calls Bill as (x, y, max_width, s, type, print, bitmap) |
|---|---|---|
| `_Print_Paragraph_(x, y, mw, s, type)` | 0x103915 | (x, y, mw, s, type, 1, 0) |
| `_Print_Paragraph_To_Bitmap_` | 0x103933 | (x, y, mw, s, type, 1, bitmap) — not ported |
| `_Get_Paragraph_Max_Height_(mw, s)` | 0x103952 | (0, 0, mw, s, 0, 0, 0), returns `__pp_max_height` |
| `_Get_Paragraph_Max_Width_(mw, s, type)` | 0x103971 | (0, 0, mw, s, type, 0, 0), returns `__pp_max_width` |

Port: `Text.PrintParagraph`, `Text.ParagraphMaxHeight`,
`Text.ParagraphMaxWidth`. Results are 16-bit; callers sign-extend.

## _Print_Paragraph_Bill_ (0x10370a)

Arguments are checked as shorts: `10 <= mw <= 640`, `0 <= x < 640`,
`0 <= y < 480`, s non-null; otherwise nothing happens (not even the reset of
`__pp_max_width`). Then `__pp_max_width = 0`, `limit = 0x1df −
current_base_height`, `ypos = y`.

State (all 16-bit): `src` index into s, `dst` index into a 200-byte stack
buffer, `acc` pixel width (unsigned), `brk` break character and
`brk_src`/`brk_dst` where it was seen. Per character `c = s[src]`:

1. 0 ends the loop. Tab becomes a space.
2. Break candidates: a space, or 0x08 (soft hyphen) when
   `acc + width('-') <= mw` (no spacing added): `brk = c`, `brk_src = src`,
   `brk_dst = dst`.
3. 0x0d or 0x0a: `brk = 0x0d`, go to *flush*.
4. Other codes below 0x20 (0x08 included): `src++`, nothing stored.
5. `acc += width(c)`. If c is not a space and `acc > mw`: `dst = brk_dst`
   if `brk_dst != 0`, `src = brk_src` if `brk_src != 0`, go to *flush*.
   Otherwise store c, `src++`, `dst++`, `acc += current spacing`.
   Spaces never overflow.

*Flush*: drop trailing spaces while `dst > 1`. If `dst > 0` or `brk ==
0x0d`: on `brk == 8` append '-'; else if `brk <= 0x20` (unsigned, so also
no break at all) `src++`; terminate the buffer and `_Print_String_Bill_` it
at (x, ypos); `ypos += current_line_feed`; `dst = 0`; stop if `ypos >
limit`. In every case then `acc = brk = brk_src = brk_dst = 0`.

At the end the rest of the buffer, if any, is printed, with type 3 turned
into 0 (the last line of a justified paragraph is left aligned).
`__pp_max_height = ypos + current_base_height − y`.

Quirks (kept):

- a word that overflows with no break before it on the line is cut where it
  overflows and the glyph that did not fit is lost (the `src++` meant for
  the break character);
- a break at buffer position 0 or string position 0 is not restored (the
  tests are on the values being nonzero);
- CR LF gives two line breaks, CR CR an empty line, which still advances
  ypos and counts as a line of width 0;
- a glyph wider than mw at the start of a line loops forever: *flush* finds
  `dst == 0`, resets and reads the same character again. The port returns
  `ErrParagraphStuck` when the same `src` reaches that path twice;
- the line buffer has no bound; past 200 bytes it overwrites the routine's
  locals. The port returns `ErrParagraphLine`.

## _Print_String_Bill_ (0x1035af)

`(x, y, mw, line, type, print, bitmap)`: the same argument check (silently
nothing). `w = Get_String_Width(line)`, or mw for types 1 and 3. If `print`,
`type | (bitmap ? 0x800 : 0)` is looked up with `repne scasw` in the 8 words
at 0x10357b `{0x803, 0x802, 0x801, 0x800, 3, 2, 1, 0}` and jumps through
0x10358b:

| type | screen | bitmap (0x800) |
|---|---|---|
| 0 | `Print_(x, y)` | `Print_To_Bitmap_` |
| 1 | `Print_Right_(x + mw − 1, y)` | `Print_Right_To_Bitmap_` |
| 2 | `Print_Centered_(x + mw/2, y)` | `Print_Centered_To_Bitmap_` |
| 3 | `Print_Full_(x, y, line, mw)` | `Print_Full_To_Bitmap_` |

Other types print nothing (the scan runs one word into the jump table, whose
first entry is the skip). Printed or not, `__pp_max_width = w` when
`(short)w > (ushort)__pp_max_width`.

# Formatted paragraphs

Port: `internal/engine/fmtpara.go` (`Text.PrintFormatted`,
`PrintFormattedToBitmap`, `FormattedMaxHeight`, `FormattedMaxWidth`,
`Paragraphs.LastFormattedMaxHeight`/`Width`, `Paragraphs.SetVar*`).

## Entry points

| Routine | Address | Does |
|---|---|---|
| `Print_Formatted_Paragraph_(x, y, mw, s, type)` | 0x103bc4 | `_Print_Formatted_Paragraph_(x, y, mw, s, type, bitmap 0, print 1)` |
| `Print_Formatted_Paragraph_To_Bitmap_(…, bitmap)` | 0x103be2 | the same with a bitmap |
| `Get_Formatted_Paragraph_Max_Height_(mw, s)` | 0x103caf | `(0, 0, mw, s, 0, 0, 0)`, returns `y2 − y1 + 1` (16 bits) |
| `Get_Formatted_Paragraph_Max_Width_(mw, s, type)` | 0x103cd6 | `(0, 0, mw, s, type, 0, 0)`, returns `x2 − x1 + 1` |
| `Get_Last_Formatted_Paragraph_Max_Height_` / `_Width_` | 0x103cfd / 0x103d0c | the same from the stored extent |
| `Set_Paragraph_User_Call_(fn)` | 0x103c01 | `_para_global+0x58 = fn`; never called |
| `Set_Paragraph_{String,Integer,Uint,Sint,Char,Schar}_Pointer_(i, p)` | 0x103c07…0x103c93 | for `(u16)i < 16`: pointer i = p, type i = 1…6 |
| `Initialize_Formatted_Paragraph_System_` | 0x103d1b | see *Reset* |

Game callers: the user and message boxes (`Do_Text_Box_` and friends,
`_Print_1_Or_More_Formatted_Lines_Centered_` 0x77a74), the help box
(`Draw_Help_Entry_` 0xa59a1), the gate info box (`Print_Gate_Info_`
0xa4989 measures with it, prints with `_Print_Paragraph_`), and others not
ported yet (eight `Set_Paragraph_Sint_Pointer_` calls at 0x37fac–0x38017,
one `Set_Paragraph_Integer_Pointer_` at 0x10a741, seven bitmap prints).

Measuring is a full run with printing off: it still calls
`Add_Square_To_Scan_List_` with the extent (unconditionally, whatever the
refresh mode), and it changes the module's statics (tabs, variables, insert
case, flags) just as printing does.

## _Print_Formatted_Paragraph_ (0x103d53)

Arguments checked as shorts: `10 <= mw <= 640`, `0 <= x < 640`, `0 <= y <
480`, s non-null; otherwise nothing (the extent keeps its old value). The
state lives in its stack frame, reached through globals (`_para_p`
0x1acf14, `_line_p` 0x1acf08, `_style_p` 0x1acf00 = line + 0x1a,
`_break_p` 0x1acf0c, `_first_char_p` 0x1acefc, `_char_p` 0x1acf10,
`_end_char_p` 0x1acf04).

Paragraph record: +0 y, +2 char pointer, +6 x, +8 x2 = x + mw − 1, +0xa
y0 = y, +0xc y limit (0x1df; each function sets it to 0x1e0 − base
height), +0xe mw, +0x10 bitmap, +0x14 print flag, +0x19 y max, +0x1d line
feed, +0x1f form feed size, +0x21 indent, +0x23 right margin, +0x25 left
margin, +0x27 justification (the type byte), +0x28 new paragraph flag (1),
+0x29 nesting level, +0x2b ten saved char pointers.

Style record (13 bytes): +0 style, +1 remap flag, +2 shadow flag, +3 colour
byte (uninitialised, stored per glyph, never read), +4 colour set, +5 base
height, +7 widths table (`font_header.font_widths[style]`, the font file's,
not the current copy), +0xb space width.

Line record: +0 x, +2 left, +4 right, +6/+8 continuation x and right (only
set by exclusion areas), +0xa last column of the last glyph, +0xc glyph
count, +0xe space count, +0x10 raise, +0x12 previous glyph x, +0x14 width,
+0x16 horizontal spacing, +0x18 done, +0x19 at line start, +0x1a style.

Glyph records (150 × 8 bytes): +0 x relative to the previous glyph's x, +2
y, +4 style (| 0x80 when remapped), +5 attribute = shadow | colour set, +6
colour byte, +7 character.

Set-up: style = `current_font_style`, remap 0, shadow =
`font_header.shadow_flag`, colour set = `Get_Current_Color_Set_` (0x1067f0:
0 if all eight current colours equal the normal ones, 0x20 if all that do
not equal the highlight ones, else 0x40), base height, line feed = vertical
spacing + base, spacing, widths and space width of the style, form feed =
line feed, y max = 0x1e0 − base. Then `Set_Line_Limits_` for y, y+1, …
while y <= y max; if no row fits, return 0. Extent = (x, y0, 0, 0); the
whole `font_header` (0x235c bytes) and `current_font_style` are saved,
`Process_Paragraph_Text_` runs, both are restored. Returns the char
pointer.

`Set_Line_Limits_` (0x106af3): l = x + left margin (+ indent on a new
paragraph), r = x2 − right margin; clears raise, spaces, count, previous x
and the break's count and spaces. If l < r: left = x = last = l, right = r,
true. Else right = x = last = left, false. Its exclusion-area loop
(`_exclusion_count` 0x1849ee, areas 0x1c0af4…) needs a count > 0, which
nothing sets, and would loop forever for an area that does not close the
row; not ported. `Vertical_Move_(d)` (0x10494e): y += d, then on to the
first row `Set_Line_Limits_` accepts; past y max y is restored and the
result is false.

## Process_Paragraph_Text_ (0x103f5d)

Per line: done 0, at start 1, previous x 0, last = x,
`Record_Break_Position_(0)` (invalidates the break). Per byte c at the
char pointer:

- c >= 0x20: letters go through the insert case (1, 3 upper, 2 lower).
  r = x + space width (for ' ') or x + width(c), minus 1. If count < 150
  and (r <= right or at line start): last = r, `Add_Character_To_Line_`,
  previous x = x, x = r + spacing + 1, at start 0; case 3 ends after a
  letter. Otherwise: for a non-space, `Return_To_Last_Break_` (and on a
  hyphen break the record at `_char_p` becomes '-'), pointer − 1; a space
  is dropped. Then if the continuation x > x, continue there; else c =
  0x0d.
- c < 0x20: `Process_Command_(c)`; true ends the paragraph.
- pointer + 1; the line loop ends when done.

At the end `Add_Square_To_Scan_List_(x1, y1, x2, y2)`.

`Add_Character_To_Line_` (0x104141) fills the record at `_char_p` (x −
previous x, y, style | 0x80 if remapped, shadow | colour set, colour, c),
steps it, and for a space records a break and counts it; count + 1.

`Record_Break_Position_(c)` (0x104292): at line start or for 0, invalid.
Else valid; for 0x08, r = x + width('-'), and if r > right nothing else
changes (an old break stays). It saves pointer + 1, x, last, insert case,
raise, spacing, nesting, count, spaces and the style; hyphen 0. For 0x08:
hyphen 1, count + 1, x = last = r. For ' ': last −= width(' ') + spacing.
`Return_To_Last_Break_` (0x1041bb) restores those; for a hyphen break
whose count − 1 is `_char_p`, it adds '-' and steps back, else `_char_p` =
count − 1. The glyph records are not restored, so a hyphen printed over a
later glyph's record keeps that record's offset.

## Process_Command_ (0x1043b0)

| Code | Action |
|---|---|
| 0x00 | insert case 0; nested: pop the pointer, − 1; else `Complete_Line_`, end |
| 0x07 | `Process_Function_`; end if it stops on the terminator (line not completed) |
| 0x08 | soft hyphen: `Record_Break_Position_(8)` |
| 0x09 | `Record_Break_Position_(9)`; x = first tab stop > x |
| 0x0a | new paragraph, then as 0x0d |
| 0x0d | `Complete_Line_`, `_char_p` = first, then as 0x0b |
| 0x0b | `Vertical_Move_(line feed)` (without completing: the pending glyphs are dropped); on failure `Quit_Paragraph_` (0x1046ce: leave all inserts, pointer to the byte before the terminator) |
| 0x0c | `Complete_Line_`, `_char_p` = first, new paragraph, `Vertical_Move_(form feed size)`, on failure quit |
| 0x17 | `Set_Value_` (next byte), then the inlined `Quit_Context_` |
| 0x18 | `Insert_Item_`, pointer − 1 |
| 0x19 | `Test_Item_` |
| 0x1a | next byte − '0' is the justification; with glyphs pending, the line so far completes with right = x, left/x stay, `Initialize_Line_Start_` |
| 0x1b | colour set = (next byte − '0') << 5 |
| 0x1c | `Set_Current_Style_`: '6' + argument <= 7 sets the colour byte; '0'..'5' are stored as ASCII into the shadow byte |
| 0x1d | `Reset_Paramaters_`: flags = table 0x183915[next & 0xf] (01 02 04 08 0c 0d 10 20 1d 01 03 01 0 0 0 0): 1 spacing, widths, space; 2 space; 4 base height and line feed; 8 form feed = line feed; 0x10 all variables; 0x20 one variable (argument) |
| 0x1f | the inlined `Quit_Context_` |
| others | nothing |

Jump table 0x10438c for 0x17–0x1f. The inlined `Quit_Context_` reads the
byte at the pointer (the command byte itself), steps, and quits on 'L'
(test flag: previous nesting) or 'P'; neither can match, so 0x17 and 0x1f
just skip the following byte.

`Set_Value_` (0x104de7), on the byte after 0x17, cx = x − paragraph x:
'T' tab stop at x (if fewer than ten; sorted), 't' clear stops, 'L'/'l'
left margin cx/0, 'R'/'r' right margin mw − cx/0, 'm' both 0, 'I'/'i'
indent cx − left margin/0.

## Complete_Line_ (0x10478c)

`Complete_Line_Data_` (0x1047d5): done = 1; with glyphs, a nonzero raise is
added to every glyph y (clamped at 0) and to the paragraph y; width =
|last − left| (one short of the span). Justification 3 becomes 0 when the
byte at the pointer is CR, LF, FF or (not nested) the terminator.
`Justify_Line_` (0x105fe9): extra = right − left + 1 − width; nothing for
type 0, no glyphs or extra <= 0; 1 adds extra to the first glyph's offset
(so a right-aligned line overshoots by a pixel), 2 adds extra / 2, 3 calls
`_Justify_Line_Fully_` (0x106053): pattern row 0 if no spaces or spaces >
count/4, row 1 if > count/8, else row 2 (table 0x18391e: 1 3 1 / 1 1 1 3 1
/ 1 1 1 1 3 1 3). Skipping leading spaces and the first glyph after them,
each pass adds one pixel to the offset of every space (bit 0) or every
glyph (bit 1), until extra is used up; after three bit-1 passes only spaces
widen (and with no spaces it stops). The port returns `ErrFmtJustify` where
that can never finish (spaces only before the first glyph), which hangs the
original.

`Print_Line_` (0x106171), x = 0 and per glyph x += offset. On a new
paragraph the first glyph's attribute sets `shadow_flag` (attr & 0xf), the
colours (attr & 0xe0: 0 `Set_Normal_Colors_On_`, 0x20 highlight, 0x40
special) and `Pick_Font_Number_`, and the flag clears. A glyph whose
attribute differs from the last sets the shadow flag, and the colours if
the top bits differ; a different style byte calls `Pick_Font_Number_`
(0x10688c: with bit 7, `Set_Remap_Font_Style_(style, normal_colors)`, which
remaps the normal colours in place and clears the shadow flag; the current
colours are kept). Extent: ax = x for the first glyph's position, else x +
width; x1 = min, else x2 = max; y1 = min y; y2 = max(y + base height). When
printing: with a shadow flag, all eight current colours = `outline_color`,
`Print_Character_` at the passes (sf ≠ 2: (1,1) (0,1) (1,0); sf ≠ 1, 3:
(−1,0) (−1,−1) (0,−1); sf 3, 5: (2,2) (1,2) (2,1); sf > 3: (1,−1) (−1,1);
sf 5: (2,0) (0,2)), then current = normal colours; then
`Print_Character_(x, y, c)` (or `_To_Bitmap_`), no clipping window.

## Functions (Process_Function_ 0x105533)

0x07, then letters from "FRTXYSHVPIMOC^=" (`_legal_functions`, case
insensitive; `strchr` also matches the terminator), each followed by
arguments separated by ','. A byte not in the list ends: skip to 0x07, '.'
or the terminator. Letters can follow each other directly. After the last
function, if it set tabs and there are more than one, they are sorted.

`Get_Function_Argument_` (0x10599e): skip spaces, relative = leading '+' or
'-', value = `atoi_`, then step over every byte of "1234567890+- " (the
terminator matches too, so an argument at the very end of a string reads on
past it: `ErrFmtOverrun`); "none" unless a non-space byte was stepped over.

Each handler may return a new letter for the next argument (0 = ignore):

| Letter | Effect (rel adds to the named base) | Next |
|---|---|---|
| F, R | `FN_Set_Font_` 0x105a2c: style (rel: + style) in 0..5; R sets the remap flag; spacing, widths, space, base, line feed, y max; a higher `_font_base_offsets` (0x18390d: 5 7 10 11 14 19) entry raises the line by the difference | 0 |
| T | `FN_Set_Tabs_` 0x105bfb: T clears the stops; each argument (rel: line x, else paragraph x) inside x..x2 and not empty adds a stop, at most ten | T |
| X | x = value + (rel: line x, else paragraph x) if inside x..x2 | Y |
| Y | `FN_Set_Y_Pos_` 0x105ca4: y = value + (rel: y, else y0) clamped to 0..limit; raise = y0 − y if larger | 0 |
| H | spacing (rel: + spacing) if not empty and in 0..63 | S |
| S | space width = spacing if empty, else value (rel: + space) in 0..63 | 0 |
| V | line feed (rel: + line feed) if not empty and \|v\| < 64 | P |
| P | form feed = line feed if empty, else value (rel) if \|v\| < 64 | 0 |
| M | `FN_Set_Margins_` 0x105def: 1st argument left margin (< mw, negative 0; clears the right margin if x + left >= right + x2), 2nd right margin (0 <= v < mw − left); then `Set_Line_Limits_` | M, then I |
| I | indent (rel) if left + indent + right < mw (left + indent < 0 gives indent = left); `Set_Line_Limits_` | 0 |
| O | `FN_Set_Origin_` 0x105edc: x, y0, mw; afterwards the paragraph is printed again from the pointer by a nested `_Print_Formatted_Paragraph_`, which leaves the frame globals on its own dead frame: `ErrFmtOrigin` | O |
| C | `FN_User_Call_` 0x105f89: nothing without a user call (none is ever set) | 0 |
| ^ | `FN_Superscript_` 0x105b24: n ≠ 0 saves the style to 0x1acfa8 and measures with style − n (clamped 0..5; base height unchanged); 0 restores it | 0 |
| = | `FN_Set_Variable_` 0x105ba0: 1st argument the variable index (\|v\|), 2nd sets it (rel: adds) | =, then 0 |

## Variables, inserts and tests

Variable i (< 16) has a pointer and a type: 1 string, 2 int, 3 unsigned
short, 4 short, 5 unsigned char, 6 signed char. `Get_Item_Value_`
(0x104f6a, jump table 0x104f52) reads int, ushort, short, schar for 2, 3,
4, 6 and the first byte for 1, 5 and anything else; index >= 16 gives 0.
`Set_Item_Value_` (0x104feb, table 0x104fd3) stores 4, 2, 2, 1, 1 bytes for
2..6 and fails for strings. `Reset_A_Paragraph_Variable_` points i at
`_variable[i]` (type 2, value 0). The port keeps int pointers in `VarPtr`
and other pointees in an `any` slot; a non-string read as a string gives
its little-endian bytes.

`Get_Insert_Argument_Next_` (0x10544b): pointer + 1, skip spaces (unless
only spaces remain), one digit 0-9 or letter A-F (upper-cased; G-Z fail),
the pointer left on it.

`Insert_Item_` (0x105089), code 0x18 and a letter. The letter's byte in the
table at 0x17a228 (for 'A'..'T' "11110101121212120000") decides whether an
argument v is read first (anything but '0'); a failed argument or a letter
outside A..T ends it. Jump table 0x105039:

| Item | Inserts |
|---|---|
| A | the string of variable v |
| B | value of v as a number (`itoa_` into 0x1acf98); plural flag = value ≠ 1 |
| C | ordinal number: `sprintf(0x1acf78, "%s\a^2.%s\a^0.", number, ending[last digit])` (endings th st nd rd th…; so 11 gives "11st"); plural flag set |
| D | 0..10: "zeroeth", "first" … "tenth"; else as C; plural flag set |
| E, G | "s", "es" if the plural flag is set |
| F, H | "s", "es" if v ≠ 1 |
| I, K | string of v if plural flag set / clear |
| M, O | string of v if test flag set / clear |
| J, L, N, P | second argument w; string of w if v ≠ 1 / = 1 / ≠ 0 / = 0 (`Insert_Argument_String_` 0x10541b) |
| Q, R, S, T | insert case 1, 2, 3, 0 |

Inserting (`Insert_A_String_` 0x1054e1, `Skip_Arg_Insert_A_String_`
0x1054b7 which first steps over the argument) pushes the pointer (at most
ten levels, else nothing) and continues in the inserted string; its
terminator pops back. The number and ordinal buffers are shared, so a later
insert rewrites them. Quirk (kept): an item that inserts nothing and does
not step over its letter or argument (E, G without plural, F, H with 1, Q-T,
failed arguments) prints that byte, because `Process_Command_` steps the
pointer back one.

`Test_Item_` (0x104c2c), code 0x19, an operator digit and up to two
arguments (missing ones count 0); the test flag (0x1acf74) is cleared
first: '0' v ≠ 0; '1' plural flag; '2' always 1 (the cleared flag
negated); '3' sets variable 0 to the first argument's digit; '4' sets
the variable numbered by v's value to the second argument's digit (flag =
success); '5'..'9', 'A' compare v with w's value: =, ≠, <, >, <=, >= (v
being the first argument's value). '1'..'3' put the pointer back on the
operator.
