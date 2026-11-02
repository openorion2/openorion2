# User_Box_ and the message boxes

Port: `internal/game/userbox.go` (boxes), `internal/game/cheats.go` (the
galaxy screen's function keys and cheats, which mostly end in a box). The
call order of `Check_Function_Keys_`/`Check_Hot_Keys_` in `Main_Screen_` is
in `main-screen-panel.md`.

## User_Box_(flag, msg, type) (0x7a25f)

`flag` is a screen's "redraw into back" byte (`&_draw_main_screen_to_back`
on the galaxy screen); every path sets it to 1 before and after the box,
then `Set_Refresh_Stencil`. Result: -1, or the answer of type 1.

| type | path |
|---|---|
| 0, > 5 | `Set_Refresh_Full`, `Message_Box_(msg)` |
| 1 | `Set_Refresh_Full`, result = `Do_Confirmation_Box_(msg)` (0x77658) |
| 2 | title = copy of `_temp_string` (0x18ff78); `Set_Refresh_Full`, `Text_Box_(title, msg)` |
| 3 | `Set_Refresh_Full`, `Warning_Box_(msg)` |
| 4 | `_global_var` (0x199f09, s8) < 0 → 0x7e; `Timed_Text_Box_(buf, msg, _global_var)` with `buf` the uninitialised title buffer (ported as "") |
| 5 | as 4 with no title; then `_global_var = 0` and return (stencil already set) |

## Do_Text_Box_(title, msg, ticks) (0xa5ebf)

Entry points: `Text_Box_` (0xa5eb2, ticks 0), `Message_Box_` (0x77423,
title NULL, ticks 0), `Timed_Text_Box_` (0xa5ebc, falls into it).

1. `Get_Mox_Text_Box_Base_Colors_(0, &a, &b)`; `Text_Box_Startup_(60000)`;
   c5 = `Font_Colors2_(5, a, b)`, c3 = `Font_Colors2_(3, a, b)`;
   `Set_Font_Style(3, c5)`.
2. h = `Get_Formatted_Paragraph_Max_Height_(0x153, msg)`; w = last
   formatted width, at most 0x153.
3. `Box_Centered_On_XY_(&x, &y, h)`; close button = `Far_Reload_Next(
   textbox.lbx, 3)`.
4. With a title: `Set_Remap_Font_Style(5, c5)`, `Set_Fitted_Font_Style_(
   title, c5, 5, 2, -1, 0x17c, 3)`, then the fitted style with
   `Font_Colors2_(style, a, b)` through `Set_Remap_Font_Style`, spacing kept,
   `Print_Centered(x+0xbe, y+0xb, title)`.
5. `Set_Remap_Font_Style(3, c3)`. Text at (x+0x14, y+0x2a); without a title
   at ((0x16a − w)/2 + x + 8, ((s16)(h+0x2b) − (u16)h)/2 + y + 0xc), that is
   y + 0x21 for small h. `Save_Window_Status_`, `Set_Window(x+0x14, y+0x2a,
   x+0x167, y+0x1a2)`, `Clip_On`, `Print_Formatted_Paragraph_(px, py, w, msg,
   0)`, restore. The formatted printer draws through `Print_Character`, which
   ignores the window, so the text above y+0x2a is not cut.
6. `Copy_Off_To_Back`; `Add_Hidden_Field(0, 0, 0x27f, 0x1df, "\x1b", 0x29)`
   (Esc is its hot key); `Text_Box_Get_Input_(ticks)`;
   close button frame 1 `Remap_Draw(x+0x9e, _bottom_y−0x1b)`;
   `Text_Box_Cleanup_`; `_FULL_DRAW_SCREEN_FLAG = 1`.

### Text_Box_Startup_(size) (0xa62e0)

`Save_Mouse_List_` (at most 10 regions), arrow cursor, `_old_help_list_active
= help_list_active`, `help_list_active = 0`, `_textbox_af =` auto function
pointer, `Deactivate_Auto_Function`, `Set_Refresh_Stencil`, call the saved
auto function once, `_old_draw_fields_flag = draw_fields_flag`,
`Enable_Draw_All_Fields`, `Draw_Visible_Fields`, `Disable_Draw_All_Fields`,
`Save_Field_Stats_(&_textbox_field_save[_field_save_ix++])`, block
allocation (memory only), `Calculate_Picture_Remap_Colors(textbox.lbx 0)`.

### Text_Box_Cleanup_ (0xa63dd)

`_TOGGLE_PAGES_`, `_field_save_ix--`, `Restore_Field_Stats_(
&_textbox_field_save[0])` — always slot 0 —, `Enable_Draw_All_Fields`, auto
function pointer back, `Activate_Auto_Function`, `Clear_Mouse_Buffer`,
`Set_Input_Delay(3)`, help and draw-fields flags back, `Restore_Mouse_List_`.

### Text_Box_Get_Input_(ticks) (0xa644d)

ticks 0: while a button is held { Mark, toggle pages, Release_Time(1) };
`Clear_Mouse_Buffer`; then { Mark, toggle, Release_Time(1), `Get_Input` }
until non-zero. Otherwise `Timed_Wait_For_Input(ticks)` (0x120486: busy-wait
for the buttons to come up, then `Quick_Call_Auto_Function` until a key
(`Keyboard_Status` only clears `key_pressed`, the key stays queued), a button
or `ticks` calls), `Clear_Mouse_Buffer`, `Set_Input_Delay(3)`.

### Save_Field_Stats_ / Restore_Field_Stats_ (0x10fe41 / 0x10fec1)

Save keeps fields_count, input_delay, down_mouse_button,
auto_input_variable, input_field_active, active_input_field_number,
max_fields_count and the `fields` pointer, then `fields += fields_count`,
`max_fields_count -= fields_count`, `Clear_Fields`. So the box's field i
lives in outer slot count+i and the outer fields are untouched. Restore puts
the values back; `multi_hotkey_active_field` (cleared by `Clear_Fields`) is
not restored. Port: `Engine.SaveFieldStats` / `RestoreFieldStats`.

### Box_Centered_On_XY_(&x, &y, h) (0xa6166)

hh = h + 0x55. Galaxy screen (current_screen 0): x = 0x54, y = (400−hh)/2 +
0x15, at least 5 (y < 6 → 5). Other screens: x = 0x82, y = (480−hh)/2.
`Calculate_Picture_Remap_Colors(textbox.lbx 0)`; `Tri_Interlaced_Fill(x+5,
y+5, x+0x177, y+hh−5, remap[2], remap[3], remap[4])`; `Remap_Draw(x, y,
entry 0)`; inside `Set_Window(x, y+0x2a, x+0x17c, y+0x2a+h)` + clip,
`Remap_Draw(x, y+0x2a, entry 1)`; `Remap_Draw(x, y+0x2a+h, entry 2)`;
`_bottom_y = y + 0x2a + h + 0x2b`.

### Get_Mox_Text_Box_Base_Colors_(kind, &a, &b) (0xa64ab)

Base: kind 1 → 0xc0, kind 2 → 0xe0, else 0xa0 (bytes 0x182435..37). Plus 3×k
with k = 2 for screens 1, 0x14, 0x19, 0x21; 3 for 9; 1 for 10; else 0.
a = base, b = base + 2.

## Drawing helpers

- `Set_Remap_Font_Style(style, colors)` (0x120940): `Set_Font_Style` with
  each colour put through `picture_remap_color_list`; shadow off.
- `Remap_Draw(x, y, pic)` (0x12b7e1): kind-0 sprites only (other kinds exit
  with "Remap draw not support ..."). With clipping, off-window → nothing (no
  frame advance), partly outside → scan square to the clipped corner and
  `Remap_Clipped_Draw_Animated_Sprite` (0x12bc0b); else whole square and
  `Remap_Draw_Animated_Sprite` (0x12baeb). Pixels are
  `picture_remap_color_list[byte]`; run format and frame advance are
  `Draw`'s.
- `Tri_Interlaced_Fill(x1, y1, x2, y2, c1, c2, c3)` (0x129130): corners
  sorted, clipped to the window when clipping is on, scan square; row r from
  the (clipped) top gets c[r mod 3].

## Formatted paragraphs

`Get_Formatted_Paragraph_Max_Height_` (0x103caf, y2−y1+1 of the glyphs),
`Get_Last_Formatted_Paragraph_Max_Width_` (0x103d0c),
`Print_Formatted_Paragraph_` (0x103bc4) and
`_Print_1_Or_More_Formatted_Lines_Centered_` (0x77a74: y −= (height −
current_base_height)/2, print type 2) belong to FMTPARA.C, ported in `internal/engine/fmtpara.go`
(`docs/engine/paragraphs.md`).

## Message_Box_Startup_ / Message_Box_Cleanup_ (0x77961 / 0x779fc)

Startup: `Save_Mouse_List_`, arrow, `Fast_Fade_In_`, `Push_Block`,
`_gendraw_af =` auto pointer, help off (saved), call the auto function (still
active), draw-fields saved, `Enable_Draw_All_Fields`, `Draw_Visible_Fields`,
`Disable_Draw_All_Fields`, `Save_Field_Stats_(&_message_field_save)`,
`Set_Page_Off`, `_save_refresh_mode = Get_Refresh_Mode`, stencil,
`_animation_frame = 0`, `_direction = 1`. Cleanup: restore field stats, auto
pointer back (the delay stays what the box assigned), `Pop_Block`, refresh
mode back (0 full, 2 differential, 3 stencil, 1 nothing), draw-fields and
help flags back, `Clear_Mouse_Buffer`, `Set_Input_Delay(3)`,
`Restore_Mouse_List_`.

## Warning_Box_(msg) (0x77433)

`_warning_box = 1`, startup, `Set_Font_Style(4, _message_text_colors)`
(0x182098: a0 a3 a3 a5 ab ab ab ab), LF 0x12, `warning.lbx` 0 (cached) with
its remap colours, a blank 0x14b×0xbf bitmap, `_message_box_y = 0x90`,
`_first_field = Add_Hidden_Field(0, 0, 0x27f, 0x1df, "\x1b", 0x29)`. Style 4
(colours: an uninitialised local), or 3 if the formatted height at width 0xe2
is above 0x78; then that style with `Font_Colors2_(style, base colours 1)`
remapped, LF 0x12. `Remap_Draw(0x9a, 0x90)`, lines centred at (0xd0, 0xe8,
width 0xe2), `Copy_Off_To_Back`, full scan square, toggle; capture
(0xd0, y+0x25)–(0x1b2, y+0x9d) into the bitmap;
`Assign_Auto_Function(Draw_Animating_Message_Box_, 2)`. Loop until
`Get_Input` ≠ 0: Mark, draw, scan square (5, 0x14, 0x24d, 0x1cb) (stale
registers), toggle, `Release_Time(2)`. Cleanup.

`Draw_Animating_Message_Box_` (0x77860): set frame `_animation_frame`;
direction +1 at 0, −1 at 6; advance; `Remap_Draw(0x9a, y)`, draw the
captured text bitmap at (0xd0, y+0x25).

## Do_Confirmation_Box_(msg) (0x77658)

Remapped style 4 from base colours 2, startup, `confirm.lbx` 0..2 (remap
colours from 0), `_first_field` = hidden (0xeb, 0x12e)–(0x11e, 0x143) "Y",
no = (0x159, 0x12e)–(0x18c, 0x143) "N", `Remap_Draw(0xa1, 0x75, frame)`;
styles 4, 3, 2 until the height at width 0xe0 is ≤ 0x7e; lines centred at
(0xcc, 0xd0, 0xe0); `Draw_Confirm_Box_` (0x778e4: buttons at (0xeb, 0x12e)
and (0x159, 0x12e), frame 1 for the one `Auto_Input` holds); copy, square,
toggle; auto function `Draw_Confirm_Box_`, 2; loop { Mark, `Get_Input`,
toggle if 0, `Release_Time(2)` } until yes or no. Cleanup,
`_FULL_DRAW_SCREEN_FLAG = 1`; true for yes.

## Function keys and cheats (cheats.go)

`Check_Function_Keys_` and `Check_Hot_Keys_` are described in
`main-screen-panel.md`; what the port adds:

- Alt-F1..Alt-F8 (codes −1101..−1108) → `Toggle_Game_Option_(−1101 − r)`
  (0x8a216): option 0 settings+0 (H 0xa6), 1 +1 (0xa7), 2 +3 (0xa8), 4 +5
  (0xaa), 5 +6 (0xab), 6 +7 (0xac; on → `Set_All_Colony_Auto_Select_Flags_`),
  7 +8 (0xad); each flips 0/1. Option 3 has no case: the name buffer is stale
  and the on/off test reads the caller's DL (ported as "" and on). Then
  `sprintf("%s %s", name, H 0xe7 "ON" / 0xe8 "OFF")`,
  `Save_Session_Related_Settings_` (0x12030: read MOX.SET, copy
  `_settings[0..0xd2]` over it, write it back; a missing file exits) and the
  box.
- `Set_All_Colony_Auto_Select_Flags_` (0x78507): per star, skip if
  `LastOrbit[p]` (star+0x17+p) ≠ −1 and planet[`LastOrbit[p]`].colony == p
  (the orbit used as a planet index — sic); else for orbits 0..4 with a
  colony that is not an outpost: owner == p → `LastOrbit[p] = orbit` (the
  scan goes on), else `LastOrbit[p] = −1`.
- F10: `Save_Game_(9)` (0x1160b) writes SAVE10.GAM block by block
  (`docs/formats/save.md`; the description is slot min(slot, 9)'s, the seed
  `Get_Random_Seed`, 1000 zero bytes at the end), then `Save_Game_Settings_`
  (0x11be4) writes `_settings` to MOX.SET. A failed open shows
  `Message_Box_("Error saving game: " + strerror)`.
- F5/F6: `Cycle_Star_Icons_` (0x8302c) on `_system_star_id_for_auto_select`
  (0x1999d0): forward from cur+1 to the end, then from 0 to the end; back
  from cur−1 to 0, then from the last star down to cur; a star counts when
  bit p of its colony owner mask (star+0x38, `Player_Has_Colony_`) is set;
  none → −1. A colony with the auto-select setting → `Do_Colony_Screen_`
  (not ported), else `moveable_box[0].content = star` and
  `Do_System_Popup_`.
- F1/F2: `Cycle_Ship_Icons_(&_fleet_box_ship_stack, &icon, back)`
  (0x82dff). Owner = the player, or the owner of the fleet box's stack when
  0 < stack < 500 and box 2 is open. Nothing happens unless the stack is set
  (≠ −1) and icons exist. Forward: stacks cur+1.., then 0..count−1; back:
  cur−1..0, then count−1 down to cur. A stack matches when its first node's
  ship (`Ship_Owner_`, ship+99) is the owner and its status (+100) < 3. The
  icon is the first whose node is that stack's start; none → both −1. A
  valid icon → `Do_Fleet_Popup_(icon, 0)`.
- F4: `N_Star_Relocations_` (0x782ed, char count of stars with
  `Relocation[p]` ≠ −1): > 1 toggles `_merging_relocations` with
  `_mouse_list_to` (0x1821b1: image 2 everywhere, 0x12 over the map) or the
  arrow; 1 → box H 0xd1; 0 → H 0xd2.
- Cheats: ISEEALL toggles `Picks[Omniscient]` (player+0x8ba); on → box H
  0xd5 then `Cheated` (+0xe69) = 1, `CheatFlag` (+0xe77) = 1; off → box
  0xd6; then draw-to-back and all star owners recomputed. MOOLA: BC (+0x32)
  += 1000, box `sprintf(H 0xd7, race name, 1000)`, flags 1/3. MENLO: box
  `sprintf(H 0xd8, name of ResearchApp (+0x322))`, research points (+0x1eb)
  = 9999 before the box, flags 1/4. The names come from
  `_technology_applications` (0x17e079, 13-byte rows, name pointer first),
  filled by `Load_Tech_Names_` (0x5df0a) from TECHNAME.LBX entry
  <language>: NUL-separated strings (runs of NULs skipped), 83 field names
  then 212 applications. EINSTEIN: `Give_Me_All_Tech_Advances_` (0x85ff2,
  every app 1..0xcb that `Player_May_Get_Tech_App_` allows goes through
  `Player_Gets_Tech_App_`, which recomputes designs, ranges, colonies and
  research fields; box H 0xdc with the count) is not ported; the flags 1/2
  are set. SCORE: `sprintf("Current score for %s is %d", emperor name,
  Do_Calculate_Hi_Score_(p))` — not ported, since `Calculate_Hi_Scores_`
  needs the race-pick cost tables (`Player_Score_Modifier_`).
- The cheat flag travels with the player record to the other machines of
  a game against others (`_game_type` ≠ 0). `Do_Next_Turn_` sets
  `_warn_about_cheaters` (0x19c19f) to 1 << PLAYER_NUM; `Main_Screen_`
  (0x86443), before its loop, clears that bit and calls
  `Warn_About_Cheating_Players_` (0x8a571): for each other player with
  `CheatFlag` set of whom PLAYER_NUM's `CheatWarning` (+0xe76) bit is clear,
  a warning box (User_Box_ type 3) `sprintf(H 0x16, emperor name, race
  name)`, both in capitals (0x132f5c: bytes 'a'..0x7f less 0x20), "The High
  Council has found evidence that %s, the highest %s, has CHEATED!!! ...";
  that player's `Cheated` = 1, Set_Page_Off, `Draw_Mini_Main_Screen_`,
  `_TOGGLE_PAGES_`, and the warned bit set. Then PLAYER_NUM's own
  `CheatFlag` = 0.
