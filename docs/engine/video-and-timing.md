# Video pages and frame timing

The page, refresh and timer layer every game screen runs on. Source of truth:
`WLIBS\video\video.c` (typed, see `wsyms -show detail -mod 144`), the VESA
assembly in `vesa.c`/`vga.c`, `MOX\` glue (`_TOGGLE_PAGES_`, `Mox_Update_`,
`Fade_Into_Screen_`, `Fast_Fade_Out_`/`Fast_Fade_In_`) and the timer helpers
next to `Read_Key_`. All addresses are ELF addresses (`build/orion2.elf`;
debug-info addresses are 0x10000 lower).

## Buffers

`Init_Video_Drivers(4)` (0x1248ab) sets 640x480, `screen_pixel_size` =
307200, and `Create_Screen_Buffers` (0x12537d) allocates:

| Global | Addr | What |
|---|---|---|
| `off_page_buffer` | 0x1bb90c | 640x480 bytes (+46 slack), zeroed. All drawing goes here by default. |
| `back_page_buffer` | 0x1bb8fc | Same size, zeroed. Clean background copy for restore. |
| `current_video_page` | 0x1bb904 | Pointer every primitive draws into: `off` or `back`. |
| `video_memory` | 0x1bb910 | 0xa0000, the VGA window. In the port: the visible `screen` buffer. |
| `refresh_scan_list` | 0x1bb908 | 480 x `{int16 x_start, int16 x_end}`; dirty span per row. |
| `refresh_stencil_list` | 0x1bb8c0 | Same shape; spans to restore from `back` after the flip. |

`.data` initial values: `screen_pixel_width` 640 (0x184534),
`screen_pixel_height` 480 (0x184538), `screen_pixel_size` 0x4b000
(0x18453c), `screen_dump_flag` 0 (0x184540).

So there are **three** 640x480 images: `off`, `back`, and the visible
screen. There is no "on" buffer in memory. Span units in both lists are
**dword columns** (`x >> 2`, 0..159); `x_start == -1` means the row is clean;
a span covers pixels `4*x_start .. 4*x_end+3`. Clear = fill with 0xff bytes
(`Clear_Scan_List` 0x125ce1, `Clear_Stencil_List` 0x125d18).

### Hardware pages (pixel-level summary)

VESA mode 0x101 (`Set_Video_Mode_Vesa_` 0x146348), 64 KB bank windows. If the
card has at least 600 KB (`multiple_video_pages`, 0x1bba4a) there are two
VRAM pages (bank 0..4 and 5..9, display start line 0 or 512); `draw_page_num`
(0x1bba66) is the page being shown. A full dump flips to the other page,
writes all of `off` there, then sets the display start to it
(`Switch_Video_Pages_Vesa_` 0x138c87, int 10h 4F07). Partial dumps write into
the shown page. Since a full dump always rewrites the whole hidden page, the
visible result is identical to one plain `screen` buffer; the port needs no
second VRAM page and no banks.

## Page selection

| Routine | Addr | Effect |
|---|---|---|
| `Set_Page_Off` | 0x124d41 | `current_page_flag`=1, `current_video_page`=`off`, `copy_to_on_page_flag`=0 |
| `Set_Page_Back` | 0x124d7a | flag=2, page=`back`, copy flag 0 |
| `Set_Page_On` | 0x124db3 | flag=0, page=**`off`**, copy flag 1. Never called by the game. |
| `Copy_Off_To_Back` | 0x124dec | `back = off`, all 307200 bytes (`Fast_Screen_Block_Copy_` 0x138ce0, length in KB) |
| `Copy_Back_To_Off` | 0x124e36 | `off = back`, all bytes |
| `Copy_Off_To_On` | 0x124e80 | Copies a rectangle of `off` to the screen, widened to dword columns. Only used by the debug printers (`Disp_PSTR_`). |
| `Cls` | 0x124b06 | Zero `off`. Unused. |

`current_page_flag` (0x1bba6b) and `copy_to_on_page_flag` (0x1bba44) are
only read by mouse helpers; with `Set_Page_On` dead they stay 1/2 and 0.
None of the copies touch the scan lists. Drawing to `back` still marks the
scan lists (primitives do not look at the page).

## Refresh modes

`screen_refresh_flag` (int, 0x1bba28) chooses how `Toggle_Pages` gets `off`
onto the screen; `screen_dump_flag` (int, 0x184540) says whether the next
flip is full (0) or partial (1 masked, 2 scan list).

| Routine | Addr | refresh | dump | Callers |
|---|---|---|---|---|
| `Set_Refresh_Full` | 0x124b65 | 0 | 0 | 28 |
| `Set_Refresh_Masked(mask)` | 0x124d01 | 1 | 0 | none (mode 1 is dead) |
| `Set_Refresh_Differential` | 0x124c9b | 2 | 0 | 4 |
| `Set_Refresh_Differential_No_Full` | 0x124cce | 2 | 2 | 1 (`Blinds_Fade_`) |
| `Set_Refresh_Stencil` | 0x124c08 | 3 | 0 | 82, incl. the end of every `Fast_Fade_*` |
| `Set_Refresh_Stencil_No_Full` | 0x124c68 | 3 | 2 | none |
| `Save_Refresh`/`Restore_Refresh` | 0x124b98/0x124bcf | save/restore both (as int16 in 0x1bba2e/0x1bba2c) | | `Blinds_Fade_` |
| `Get_Refresh_Mode` | 0x124c3b | returns refresh | | |
| `New_Screen` | 0x124b37 | dump=0, clear scan list | | none |
| `Set_Refresh_Mode_(n)` (MOX) | 0xc5b64 | 0 Full, 2 Differential, 3 Stencil, 1 and >3 ignored | | `Do_Popup_Cleanup_` |

`Init_Video_Drivers` starts with refresh 0. `Draw_File_Animation_Stencil_`
and `Draw_File_Animation_On_` set the flag temporarily and restore it.

Meaning:

- **0 Full**: every flip copies all of `off`. Scan lists are ignored for the
  dump and cleared after it.
- **2 Differential**: the first flip after selecting the mode is full, later
  flips copy only the rows/spans in `refresh_scan_list`. Primitives mark only
  the scan list. Cleared after each flip.
- **3 Stencil**: like 2 for the dump, but primitives mark both lists, and
  after the flip the stencil spans of `off` are restored from `back`
  (sprites drawn this frame are erased from `off`), and those spans become
  the next frame's scan list so the erase reaches the screen.

The `_No_Full` variants skip the initial full flip.

### Marking dirty areas

`Add_Square_To_Scan_List_(x1, y1, x2, y2)` (asm, 0x138cee; EAX EDX EBX ECX):
clamp x to 0..639 and y to 0..479 (hard-coded, not the window), then for
every row `y1..y2` (none if `y1 > y2`) merge columns `x1>>2 .. x2>>2` into
`refresh_scan_list[y]`, and, unless `screen_refresh_flag == 2`, into
`refresh_stencil_list[y]` too. Merge: empty row takes `{x1>>2, x2>>2}` as is
(even if `x2 < x1`); otherwise `x_start = min`, `x_end = max`. It does
**not** test the refresh flag for 0/1: callers do (`Fill` only calls it when
`screen_refresh_flag >= 2`); `Toggle_Pages` calls it on the partial path.

`Add_Dot_To_Scan_List(x, y)` (0x125e13, no clamping) and
`Add_Line_To_Scan_List(x1, y, x2)` (0x125ffb, clamps x1,y >= 0, x2,y <=
max) do the same for one row.

Callers (which primitives mark what) — each primitive's spec must record its
marked rectangle, since in stencil mode that rectangle is what gets erased
next frame: `Fill_`, the other `*_Fill_`s, `Draw_`, `Draw_No_Glass_`,
`Remap_Draw*`, `Draw_File_Animation*`, `Draw_Smacker_Animation_`, `Line_`
family (squares and dots), `Dot_`, `Print_Display_`,
`Process_Paragraph_Text_`, `Message_Box_Exploding_`, `Do_Confirmation_Box_`,
`Blinds_Fade_`, `Clipped_Draw_Stenciled_Animated_Sprite_` (lines).

## Toggle_Pages (0x124ecb)

`_TOGGLE_PAGES_` (0x1077d, 280 callers) is `Mox_Update_(); Toggle_Pages()`.
`Toggle_Pages` (21 direct callers) in order:

1. `toggle_flag` (0x1b922a) = 1 (blocks the mouse interrupt's redraw);
   `Disable_Mouse`; `old_mouse_x/y = mouse_x/y`.
2. `Draw_Visible_Fields` (0x11e718): resets the window, `Set_Page_Off`, draws
   every active field (buttons, labels, input fields, list items) into `off`,
   restores the window. Fields are therefore redrawn on every flip.
3. If `screen_dump_flag == 0` (**full**):
   - copy all of `off` to the screen (`Full_Screen_Dump`, 0x1255df);
   - `screen_dump_flag` = 2 if refresh is 2 or 3, = 1 if refresh is 1, else
     stays 0;
   - draw the mouse cursor onto the screen (`Save_Mouse`/`Draw_Mouse`
     pointers 0x1b920c/0x1b9210; `Save_Mouse_Hi_Res_` records the position
     in `last_on_page_mouse_x/y` 0x1bba42/0x1bba40).
4. Else (**partial**):
   - `Add_Square_To_Scan_List(lx, ly, lx+26, ly+24)` with the old
     `last_on_page_mouse_x/y` (erases the old cursor);
   - `Save_Mouse_Off_Page_` (0x145719): `last_on_page = mouse`, save the
     26x24 block of `off` at `y*640 + x` rounded down to even (height cut to
     `480-y` near the bottom);
   - `Draw_Mouse_Off_Page` draws the cursor into `off`;
   - `Add_Square` again for the new position;
   - dump: `Masked_Screen_Dump` if `screen_dump_flag == 1` (unreachable),
     else `Partial_Screen_Dump` (below);
   - `Restore_Mouse_Off_Page_` puts the saved block back into `off`.
5. `Refresh_Palette` (0x131f7b, see Palette); `Enable_Mouse`.
6. If `screen_refresh_flag == 3`: `Stencil_Out_Background` (below), then
   `refresh_scan_list = refresh_stencil_list` (`Copy_Structure_`, 480*4
   bytes), then `Clear_Stencil_List`. Otherwise `Clear_Scan_List`.
7. `Set_Page_Off`; `toggle_flag` = 0.

Note step 6 in stencil mode also restores the two cursor rectangles from
`back` (they were added to the stencil list), and that a fresh
`Set_Refresh_*` makes the next flip full again.

### Partial_Screen_Dump_Hi_Res (0x125814)

For every row `y` with `x_start != -1`: copy dwords `x_start..x_end`
(`x_end - x_start + 1` dwords) from `off` to the screen at the same place.
**Except** the four rows that straddle a 64 KB bank (102, 204, 307, 409):
if their `x_start != -1`, the **whole row** (640 bytes) is copied regardless
of the span. Rows are visited top to bottom; there is no clipping of the
span other than a patch helper (0x1694d9) that turns a count `<= 0` into 1
dword (so a span with `x_end < x_start` copies one dword at `x_start`).

The masked dump (`Masked_Screen_Dump_Hi_Res`, 0x12567f; `video_mask` is
`(skip, count)` dword pairs ended by 0xffffffff) is unreachable.

### Stencil_Out_Background (0x125d4f)

For every row with stencil `x_start != -1`: `off[o .. o+4n) = back[o ..
o+4n)` where `o = (y*160 + x_start)*4` and `n = x_end - x_start + 1`. The
address goes through a bounds-check patch (0x1694b7 → 0x169506) with a
quirk the port must reproduce:

- if `o < 0`, `o >= 307200`, `n < 0`, or `o + 4n >= 307200` (note `>=`):
  the destination becomes `off + 0` and `n` becomes 1, the source stays
  `back + o`. One dword of `back` lands at `off[0..3]` and the row is **not**
  restored.

`o + 4n == 307200` happens for any row-479 span reaching column 159, e.g.
after a full-screen `Fill` or with the cursor at the bottom right.

## Fill (0x128c32)

`Fill(x1, y1, x2, y2, color)` — EAX EDX EBX ECX, `color` (int16) on the
stack, low byte used.

1. Swap so `x1 <= x2` and `y1 <= y2`.
2. If `clipping_flag` (int16 0x1845d8, initially 0) == 1, with the window
   `screen_window_x1/x2/y1/y2` (0x1bba4c/4e/50/52): return if `x1 >= wx2`,
   `x2 <= wx1`, `y1 >= wy2` or `y2 <= wy1` (strict, so a rectangle that only
   touches the window edge is dropped); then clamp `x1 = max(x1, wx1)`,
   `y1 = max(y1, wy1)`, `x2 = min(x2, wx2)`, `y2 = min(y2, wy2)`.
3. If `screen_refresh_flag >= 2`: `Add_Square_To_Scan_List(x1, y1, x2, y2)`.
4. Set pixels `[x1..x2] x [y1..y2]` (inclusive) of `current_video_page` to
   `color`, stride 640 (`Fill_Column_`/`Fill_Region_`).

Quirk: dword columns are computed with C division (`x/4`, toward zero), so
an unclipped `x1` in -3..-1 fills pixel(s) left of 0 and then skips pixels
0..3. Not seen in practice.

## Palette at the flip

`current_palette` (0x1bb358) is 256 x `[changed, r, g, b]`, 6-bit colours
(`docs/formats/pictures.md`). The DAC (what is displayed) is separate state.

- `Refresh_Palette` (0x131f7b): walk entries; each run of consecutive
  entries with `changed != 0` (cut at 128) is written to the DAC after one
  `Wait_For_Vsync_`; then clear all `changed` flags.
- `Darken_Palette_(p)` (asm 0x132c80): DAC[i] = `c * (100-p) / 100`
  (integer, truncated; uses the low byte of `100-p`) for all 256 entries,
  colours 0–127 after `Forced_Wait_For_Vsync_`, 128–255 after
  `Wait_For_Vsync_`; `current_palette` is unchanged but **every `changed`
  flag is set to 1**, so the next `Refresh_Palette` (i.e. the next flip)
  restores full brightness unless the caller clears the flags first.
- `Wait_For_Vsync_` (0x132b27) waits for the start of the next vertical
  retrace (skipped if `vsync_speed_flag` 0x1bc798 is set; it never is).
  `Forced_Wait_For_Vsync_` (0x132b41) always waits.

## Timing

Timer source: the BIOS tick counter at linear 0x46c (dword).

| Routine | Addr | Effect |
|---|---|---|
| `Mark_Time_` | 0x12c2a0 | `mark` (unnamed dword 0x1845dc) = ticks |
| `Release_Time_(n)` | 0x12c2c6 | busy-wait: loop `d = ticks - mark`; return if `d < 0` (signed) or `d - n >= 0`. Registers preserved. |
| `Time_Passed_` / `Timer_Value_` | 0x12c2ae / 0x12c2bd | `ticks - mark` / `ticks`; never called |

So `Release_Time_(n)` returns once at least `n` whole ticks have passed since
the last `Mark_Time_`; it does not re-mark. Because the mark lands anywhere
inside a tick, the real wait after `Mark_Time_` is between `n-1` and `n` tick
periods (and 0 if the drawing already took `n` ticks). The idiom is:

```
loop: Mark_Time_(); ...input, draw...; _TOGGLE_PAGES_(); Release_Time_(n)
```

Arguments at the 95 call sites: 1 (45x), 2 (35x), 4 (4x), 7 (2x), 3, 16,
plus a few computed.

Tick rate: the game installs the Miles (AIL) timer on INT 8 and reprograms
the PIT, but AIL keeps a BIOS chain slot with period 54925 µs
(`_set_PIT_period` caps at 0xd68d; `_API_timer` 0x154d9a chains to the old
handler when that slot's accumulator overflows). So the tick still averages
18.2065 Hz (54.925 ms), quantised to AIL's fast PIT period. Port: `ticks =
floor(elapsed_µs / 54925)` from a monotonic clock; `Release_Time_` sleeps
until `ticks - mark >= n`. Midnight wrap (the `d < 0` exit) can be ignored.

Nothing else in the layer waits on the timer. `Toggle_Pages` waits only on
vsync inside `Refresh_Palette` (one wait per changed run). Fades are paced
by vsync, not ticks.

## Mox_Update_ (0xfe8be)

`Sound_Update_()` then `Mox_Net_Update_()`. Called 699 times, including from
`_TOGGLE_PAGES_` and every fade step.

- `Sound_Update_` (0x1131f0): per-channel volume fades (`fades[i]` subtracted
  each call, stop at 0), streaming-music fade-up and double-buffer refill,
  loop restart. No drawing, no RNG.
- `Mox_Net_Update_` (0xf69fe): no-op unless `_game_type` is 2 or 3 (network
  play), not re-entered, and not inside `_in_receive_message`.

For single-player the port can treat it as "service audio".

## Fades

`fade_percent` (byte 0x18319c, initially 0) is the current darkening;
`fade_speed_in_percent` (byte 0x18319d, initially 10; every
`Set_Fade_Speed_Percent_` caller passes 10).

`Fast_Fade_Out_` (0xc5bb9), only if `fade_percent == 0`:
`Set_Refresh_Full`; for `p = 0, s, 2s, ... <= 100`: `Darken_Palette_(p)`,
`fade_percent = p`, `Mox_Update_()`. Then `Set_Page_Back`,
`Fill(0,0,639,479,0)`, `Copy_Back_To_Off`, `Clear_Palette_Changes(0,255)`,
`_TOGGLE_PAGES_`, and finally `Set_Refresh_Stencil`. If `fade_percent != 0`
it does nothing at all.

`Fast_Fade_In_` (0xc5c44), only if `fade_percent != 0`: `Set_Refresh_Full`;
for `p = fade_percent, p-10, ... >= 0`: `Darken_Palette_(p)`,
`fade_percent = p`, `Mox_Update_()`; then `Set_Refresh_Stencil`. Step is
always 10. No flip.

### Fade_Into_Screen_ (0x8f1c4)

`Fade_Into_Screen_(draw, flag, palette)`: EAX = draw callback (no
arguments, e.g. `Draw_Main_Screen_`, `Draw_Mini_Main_Screen_`,
`Main_Menu_Fade_Function_`), EDX = pointer to a byte flag, BX = palette
number. In order:

1. `Disable_Draw_All_Fields_`; `*flag = 1` (callers pass
   `draw_main_screen_to_back` 0x199f28 or a dummy local; it is never cleared
   here).
2. `Clear_Palette_Changes(0,255)`; mouse cursor list = none.
3. `Fast_Fade_Out_` (if not already faded: screen to black, `off`/`back`
   filled with 0, mode stencil; otherwise nothing).
4. `Load_Palette(palette, 0, 255)` (FONTS.LBX entry `palette+1`, sets all
   `changed`), `Set_Palette_Changes(0,255)`, `Refresh_Palette` (DAC = new
   palette at full brightness for a moment), `Darken_Palette_(100)` (DAC
   black).
5. Mouse cursor list = arrow; `Set_Page_Off`.
6. `draw()` — draws the new screen into `off` (and into `back` if it honours
   the flag). The callbacks seen do not flip.
7. `Clear_Palette_Changes(0,255)` (so the coming flip keeps the DAC state).
8. If `reloaded_ship_palette` (0x199f38) == 0: `Fast_Fade_In_`; else
   `Set_Fade_Percent_(0)` + `Darken_Palette_(0)` (instant full brightness).
9. `_TOGGLE_PAGES_` (refresh is stencil, `screen_dump_flag` 0, so a full
   flip); `reloaded_ship_palette = 0`; `Enable_Draw_All_Fields_`.

Related: `Fade_To_Black_` (0x8f26e): `Fast_Fade_Out_`, `Set_Page_Off`, fill
`off` black, flip. `Fade_To_Palette_(n)` (0x8f299): `Set_Refresh_Full`,
fade out, fill black, flip, `Load_Palette(n,0,255)`.

## Port model

- State: `off`, `back`, `screen` (640x480 bytes each), the two span lists,
  `current_palette` with flags, a separate 256-entry DAC, the flags above.
- Present to SDL = `screen` through the DAC (widen `v<<2|v>>4`) plus the
  cursor overlaid at the current mouse position (the mouse interrupt redraws
  the cursor on the screen between flips, `User_Mouse_Handler_` 0x1236d1,
  suppressed while `toggle_flag` is set).
- Vsync waits become "wait for the next host frame at ~60 Hz"; ticks come
  from the 54925 µs clock.
- Keep the scan-list and stencil arithmetic exact, including the bank-row
  whole-row copies and the `Stencil_Out_Background` bounds quirk: they decide
  which `off` pixels reach the screen.

## Open questions

1. ~~**Fade-in shows nothing.**~~ Confirmed against the original: fade1
   (a capture of every page flip, those of `Fade_Into_Screen_`, `Fade_To_Black_` and
   `Fast_Fade_Out_`) shows `Fast_Fade_Out_`'s black page and then the main
   menu at full palette; `Fast_Fade_In_` brightens the DAC over the black
   screen, so the picture cuts in. The port does the same
   (`TestFadeIntoScreenFrames`, all 79 flips). Captures hold
   `current_palette`, so the DAC steps themselves are not visible.
2. **Vertical refresh rate** of VESA 0x101 under the original setup (assumed
   59.94/60 Hz). It sets the speed of every fade (2 waits per
   `Darken_Palette_` step, 11 steps each way) and `Refresh_Palette`.
3. **Cursor details** (image, hotspot, the even-x save alignment vs the drawn
   position, `Save_Mouse_Hi_Res_`/`Draw_Mouse_Hi_Res`) are left to a mouse
   spec; this one only fixes where the cursor rectangles enter the scan and
   stencil lists.
4. **Wall-clock dependence.** The number of frames per second depends on the
   tick and vsync; if any per-frame code consumes the game RNG, RNG traces
   differ between runs even in the original. Not checked here.
5. `Init_Video_Drivers` is assumed to get mode 4 (640x480); the caller
   (0x113d6c) passes it from a parameter not traced.
