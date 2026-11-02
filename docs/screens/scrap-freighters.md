# Scrapping freighters (screen 0x23) and the slider popup

A click on the freighters of the galaxy screen's panel (Main_Screen_,
field 0x221,0x10e..0x265,0x154) with `FreightersTotal ≥ 1` sets
`_current_screen = 0x23`, `_return_screen = 0`; with none it says H 0xdf.
Screen_Control_ (0x10712) runs Scrap_Freighters_. Port:
`internal/game/slider.go`.

## Scrap_Freighters_ (0x8b519)

- `box_open = Moveable_Box_Selected_(2)`, `confirmed = 1`.
- `n = N_Freighters_Player_Can_Scrap_(PLAYER_NUM)` (0xff477:
  `FreightersTotal (+0x36) − 5 × SettlersCount (+0x40)`, 16 bits).
- `n ≤ 0`: `User_Box_(&_draw_main_screen_to_back, H 0xe6, 3)` (a warning
  box; the string is first copied to the stack).
- Otherwise: `Assign_Auto_Function_(Draw_Mini_Main_Screen_, 1)`,
  `Set_Scrap_Freighters_Help_List_` (0x6f83b: help list 0x1819f2, one
  entry), `Clear_Fields_`, the box corner compared with 0x20f/0x1a5 and
  ignored, `moveable_box[2].content = 0`, the mini map into both pages
  (`Disable_Draw_All_Fields_`, `Set_Page_Off_`, `Fill_` black,
  `Draw_Mini_Main_Screen_`, `_TOGGLE_PAGES_`, `Enable_Draw_All_Fields_`),
  `r = Slider_Popup_(0, n, 0, H 0xed, star 0)`.
  - `r ≥ 0` and (`FreightersTotal − r ≤ 0` or `FreightersFree (+0x38) − r ≤ 0`):
    `confirmed = User_Box_(…, sprintf(H 0xee, free, free == 1 ? H 0xf0 : H 0xf1,
    r, r == 1 ? H 0xf0 : H 0xf1), 1)` (yes/no).
  - `confirmed ≠ 0` and `r > 0`: `User_Box_(…, sprintf(H 0xef, r, plural), 0)`,
    `FreightersTotal = max(FreightersTotal − r, 0)`,
    `Do_Player_Colony_Calculations_(PLAYER_NUM)`. FreightersFree is left to
    the colony calculations.
- Tail: `_current_screen = _return_screen`, `box_open` →
  `moveable_box[2].content = 1`, `Clear_Fields_`, `Deactivate_Auto_Function_`
  (it jumps into Building_Outposts_In_Main_Screen_'s epilogue: no
  settings restore, no Clear_System_Display_Loaded_Flag_).

## Slider_Popup_(min, max, value, msg, star) (0xc9eb0)

Also used by Leader_Cheat_Popup_ (not ported).

- `_last_scanned_field = -1`; `_g_min`, `_g_max`, `_g_var`, `_g_msg` set.
- Load_SLIDER_Popup_ (0xc99db): MAINPUPS.LBX 0x34 (frame), 0x4a (−),
  0x4b (+), 0x4c (cancel), 0x4d (OK), each Far_Reload_Next_.
- `Do_Popup_Setup_(star, frame)`, `Calculate_Picture_Remap_Colors_(frame)`,
  `Deactivate_Auto_Function_`, `Assign_Auto_Function_(Draw_Slider_Popup_, 1)`,
  `Eric_Push_Fields_`, Add_Slider_Fields_, Draw_Slider_Popup_,
  `_TOGGLE_PAGES_`.
- Loop (no Release_Time): `in = Get_Input_`, `Scan_Input_`,
  Draw_Slider_Popup_, `_TOGGLE_PAGES_`; then `in == OK` → done;
  `in == cancel` → done, `_g_var = -1`; `in == +` → `_g_var++`;
  `in == −` → `_g_var--`. The buttons do not clamp; the scroll field's
  Draw_Field in the next Get_Input_ clamps `_g_var` to [min, max] before
  it is printed, so − at the minimum shows no change (scrapf1).
- `Do_Popup_Cleanup_`, return `_g_var`.

Add_Slider_Fields_ (0xc9aa4), relative to the popup's `_g_x`, `_g_y`:
scroll field at (+0x25, +0x95), 0xcf × 0xd, values min..max+1, valid
min..max, variable `_g_var`; − at (+0xf, +0x9a) hot key E 0x89; + at
(+0x100, +0x9a) E 0xb; cancel at (+0xf, +0xae) E 0x87; OK at (+0xd0, +0xae)
E 0xc; all with sound 0x28; a hidden field over the whole screen.

Draw_Slider_Popup_ (0xc9c3d): `Set_Remap_Font_Style_(2, 0xc6e56)`; on
`_full_draw` or `_FULL_DRAW_SCREEN_FLAG`: Mainpups_Tested_Execute_Autofunction_,
Remap_Draw of the frame, `Squeeze_Formatted_Paragraph_Centered_(+0x13, +0x13,
0xfc, 0x5c, msg, 2)`, `Copy_Off_To_Back_`, `_full_draw = 0`. Then
Print_Integer_Centered_ of min at (+0x20, +0x84), value at (+0x91, +0x82),
max at (+0x101, +0x84); the thumb at `pos = (var − min) × 0xcf / (max − min)
+ 0x25`: five vertical lines (+pos+k, +0x99)–(+pos+k, +0xa4) in colours
0x20, 0x24, 0x28, 0x24, 0x20.
