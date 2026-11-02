# Main menu

Port: `internal/game/mainmenu.go`. Source of truth: `MOX\OTHERS\MAINMENU.C`
— `Main_Menu_Screen_` (0x816f2), `Reload_Main_Menu_Screen_` (0x81abe),
`Add_Main_Menu_Fields_` (0x80ae4), `Draw_Main_Menu_Screen_` (0x813a4),
`Main_Menu_Fade_Function_` (0x81381) / `_Function2_` (0x81395),
`Font_Colors2_` (0x8e5xx). Engine behaviour: `docs/engine/*.md`.

## Pictures (MAINMENU.LBX)

| Entry | Use |
|---|---|
| 0 | the sliding panel animation (`Do_Main_Menu_File_Animation_`, file-streamed) |
| 1 | `_mainmenu_first_seg`, drawn by `Main_Menu_Fade_Function2_` (panel path) |
| 2 | `_main_menu_background_seg`, the menu with all buttons in their normal look |
| 3+3i, 4+3i, 5+3i | button i: normal (unused by the draw routine), highlighted, disabled |

Buttons i = 0..5: Continue, Load Game, New Game, Multi Player, Hall of Fame,
Quit Game. Drawn at x 0x19f, y 0xac, 0xc3, 0xd9, 0xf0, 0x106, 0x11d. The
`_mainmenu` block holds per button `{pic0, pic1, pic2, int16 field}` (14 bytes).

## Flow of Main_Menu_Screen_

1. `Set_Refresh_Full`, `Clear_Fields`, load entry 1, `Load_Credits_`, reset
   `_game_type`, allocate `_mainmenu` and fill it with 0xff (fields = -1).
2. Entry path:
   - `_pan_main_menu_panel` (set by `main2_` at start-up): reload, clear the
     flag, `Fade_Into_Screen_(Function2, &flag, 5)` (the station without the
     panel, entry 1), `Set_Refresh_Full`, then the panel slides out
     (`Do_Main_Menu_File_Animation_`, below); its key result drives the
     dispatch as if pressed;
   - previous screen 8, 12, 13, 14, 15 or `_FULL_DRAW_SCREEN_FLAG`: clear the
     flag, reload, `Fade_Into_Screen_(Main_Menu_Fade_Function_, &flag, 5)`;
   - previous screen 10: reload, off page black, draw entry 2, flip;
   - otherwise: nothing, and the fields are not added (see open questions).
   The first three continue with `Add_Main_Menu_Fields_`.
3. `Set_Refresh_Stencil`; back page black + entry 2; `Copy_Back_To_Off`; flip.
4. `Add_Hot_Key("N")`, `Assign_Auto_Function(Draw_Main_Menu_Screen_, 2)`, help
   list ids 645..650 over the buttons, credits position (0x42, 0x1a6).
5. Loop: `Get_Input`, `_scanned_field = Scan_Input`, `Mark_Time_`, dispatch,
   and unless leaving `Set_Page_Off`, `Draw_Main_Menu_Screen_`, flip; then
   `Release_Time_(3)` every pass.
6. On leaving: `_FULL_DRAW_SCREEN_FLAG = 0`, `_main_screen_fade_in_flag = 1`,
   `Deactivate_Auto_Function`.

Dispatch (`r` from `Get_Input`, `k` the panel animation's key, -1 otherwise):

| Condition | Result |
|---|---|
| r = Continue, or k = 'C' and Continue exists | `Load_Game_(9)`; on success screen 0x11 (`_game_type` 1) or 0x27, network types warn (message 0xcf); else screen 10 |
| r = New, r = the extra hot key, or k = 'N' | screen 13, return screen 0 |
| r = Load or k = 'L' | `Do_Mainmenu_Load_Screen_`; success as Continue (message 0xd0); else screen 0 if the previous screen was 0, otherwise stay (screen 10) |
| r = Multi Player or k = 'M' | screen 15, return 10 |
| r = Hall of Fame or k = 'H' | screen 14, return 10 |
| r = Quit or k = 'Q' | screen 7 |

## The sliding panel (Do_Main_Menu_File_Animation_, 0x80dxx)

`Open_File_Animation("MAINMENU.LBX", 0)` (reads the entry's first 0x2000
bytes, the header and frame table, into `io_buffer`; frames are read from
the file one at a time), then `Set_Palette_Changes(0,255)` and
`Draw_Palette(_main_menu_background_seg)`: the animation is shown with
palette 5 plus **entry 2's palette chunk**, which is why entry 0 matches no
FONTS.LBX palette on its own. Fields: six tiny hidden fields stepped down
the top left corner (x 10+10i..0x19+10i, y 0x14+10i..0x23+10i) whose hot
keys C, S, L, M, H, Q are what matters, `Add_Hot_Key(" ")`, and a
full-screen hidden field with hot key Esc (0x1b; it wins over the Esc
cancel because the key matches a hot key first). Auto function:
`Draw_Main_Menu_File_Animation_` (a jump to `Draw_File_Animation_`), delay 1.

Loop: `Mark_Time_`, `Get_Input`, frame = `Get_File_Animation_Frame` (the
header's current frame); sound 0x35 at frame 5, stop and sound 0x36 at frame
0x2e; leave on the full-screen field or the space key (sound 0x36), when
the frame reaches `total_frames - 1` (so the last frame is never drawn), or
on one of the six hot keys with result 'C','S','L','M','H','Q'; otherwise
`Set_Page_Off`, `Draw_File_Animation`, flip. `Release_Time_(1)` every pass,
so the panel moves at one frame per tick (about 18 per second). 'S' has no
case in the menu dispatch.

`Draw_File_Animation` draws at (0,0) like `Draw`, but its frame counter
stops on the last frame (or, with mask 0x20, wraps to 0 when
`repeat_frame + 1 == total_frames`, else to `repeat_frame`).

## Fields

`Clear_Fields`, then hidden fields x 0x19f..0x237 with sound 0x29 and hot
keys C, L, N, M, H, Q at rows 0xac–0xc1, 0xc3–0xd7, 0xd9–0xee, 0xf0–0x104,
0x106–0x11b, 0x11d–0x132. Continue only if `save10.gam` exists and its game
type (`Get_Game_Type_From_Saved_Game_(9)`) is 0 or 1; Load only if
`_disallow_saves_code` is 0 and `save?.gam` matches. A missing field is -1000.

## Draw_Main_Menu_Screen_

If `_FULL_DRAW_SCREEN_FLAG`: off page black, entry 2, `Copy_Off_To_Back`,
flip, reset credits, clear the flag. Then the highlighted picture of the
button whose field equals `_scanned_field`; the disabled picture for
Continue and Load when their field is -1000; credits if `_do_credits`; and
`Ver 1.40b23` (port-only: `Ver 1.40b23-v1.0.0`, the original's version
then the port's release, `update.Version`, `dev` in a build of one's own)
in font style 1, shades `Font_Colors2_(1, 0xd2, 0xd7)` (shade 1
= 0xd2, the rest 0xd7), centred on x 0x205 with its top at
`0x1bb - (height + 1)`.

`Font_Colors2_(mode, a, b)` for mode 0–1: shade 1 = a, shades 2–8 = b.
Modes 2–5 also adjust shade 2 (not ported).

## Credits

Port: `internal/game/credits.go`. The original shows them whenever the
main menu's loop runs: Main_Menu_Screen_ calls `Load_Credits_` on every
entry, which always sets `_do_credits`, and Draw_Main_Menu_Screen_ (the
loop's draw and auto function) calls `Draw_Credits_` while it is set. They
are not drawn during the panel's slide or the fades (their draw functions
only blit the background). Each entry, each full draw
(`_FULL_DRAW_SCREEN_FLAG`) and the end of the list restart them at the
bottom after ten skipped draws.

- `Init_Credits_` (0x81b7c), by setting 0xd2 (language; > 4 as 0, a jump
  table at 0x81b68): file CREDITS.LBX, GERCRDTS.LBX, FRECRDTS.LBX,
  SPACRDTS.LBX, ITACRDTS.LBX; second column at x 0x7c, 0x9a, 0xa4, 0x92,
  0x92; font style 3 (English) or 2; spacing 2 (German) or 1.
- `Load_Credits_` (0x8156b): entry 0 is a record table; record 0 is the
  line count (atoi), then each line's two columns as records 2i+1 and
  2i+2, strcpy'd into 0x7f-byte halves of a 0xfe-byte line. `_do_credits
  = 1`, delay 0, `_credit_x = 0x42`, `_credit_y = 0x1a6`, a 0x121×0xe
  bitmap. Main_Menu_Screen_ then sets delay 10 (0x818eb).
- `Draw_Credits_` (0x81147): while the delay is positive it only counts
  down. Otherwise `y -= 2`; top = `(0xb3 − y)/14` once `y ≤ 0xb3` (else −1),
  bottom = `(0x1a6 − y)/14`; every line i (all of them, clipped) gets a
  fresh blank bitmap, `Set_Outline_Color_(0xfd, 0xfd)`,
  `Set_Font_Style_Outline_(style, shades)`, `Set_Font_Spacing_`,
  Copy_Credit_String_To_Bitmap_, and is drawn at (0x42, y + 14i) in the
  window (0x42, 0xb3)–(0x162, 0x1a6) (x2, y2 are left over in EBX, ECX).
  Shades (8 bytes in the code segment): the top line 0x80adc plus
  `2 × (((y − 0xb3) mod 14) / 3)` on shades 1–7 (the remainder is ≤ 0: it
  darkens), the bottom line 0x80acc plus `2 × (((0x1a6 − y) mod 14) / 4)`,
  the rest 0x80ad4. Then, also after a skipped draw, `|y − 0xb3| > 14 ×
  count` restarts (delay 10, y 0x1a6).
- `Copy_Credit_String_To_Bitmap_` (0x80c8a): nothing for a line with both
  columns empty; column one at x 0; if it is not empty x = width + 4; with
  both columns, "." printed and x advanced by its width at least once and
  until x reaches the second column's x; column two at that x + 4.

Oracle: `build/oracle/cred1.sh` (three captures of the main menu some
seconds apart, no saves). The credits move by the clock, so
`TestMainMenuCredits` compares each capture with the port's frame that
shows the same position (1410, 2418, 4386; `OPENORION2_CREDITS_SEARCH=1`
finds them again): all three are identical. Not compared: the other
languages.

## Not ported

`Load_Game_`, `Do_Mainmenu_Load_Screen_`, the network game
type warnings, reading the game type from `save10.gam`, help boxes on right
click (`Draw_Help_Entry_`), sounds.

## Open questions

1. With no fade path and a previous screen other than 10, the fields are not
   added and every slot stays -1, so a cancel (-1) would act as Continue.
   When the game takes that path is unknown.
2. `Fade_Into_Screen_` by the code fades the palette up over a black screen
   and then cuts to the new picture at full brightness, so the menu appears
   at once; worth confirming against the original.
