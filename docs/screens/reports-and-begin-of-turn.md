# Screen_Control_, Reports_Screen_ and the begin-of-turn reports

From `main2_` to `Main_Screen_`: the top-level screen loop, the reports
screen (0x27), `Do_Begin_Of_Turn_` and its report checks, and the hotseat
player picker (0x11). Addresses are ELF (Watcom + 0x10000). Globals are BSS
(zero at start). "Save state" = something written to `SAVEn.GAM`; those
writes must be ported bit-exactly.

## Globals

| Address | Name | Type | Written by |
|---|---|---|---|
| 0x199a08 | `_current_screen` | s16 | screens |
| 0x199f19 | `_previous_screen` | u8 | `Screen_Control_` after each screen |
| 0x199a10 | `_return_screen` | s16 | screens |
| 0x199a0e | `_global_esc` | s16 | 0 by `Screen_Control_` before each screen; report handlers set 1 to leave |
| 0x1975b8 | `_stardate_at_beg_of_turn` | s32 | 0 `Harold_Defaults_`; `= stardate` in `Reports_Screen_` (no report), `Main_Screen_Report_Handler_`, `Hotseat_Select_Player_` |
| 0x199f17 | `_previous_report` | u8 | 0 `Harold_Defaults_`, `Do_Next_Turn_`; last `Display_Report_` result |
| 0x199f2c | `_at_least_one_report` | u8 | 1 `Main_Screen_Report_Handler_`; 0 only `Do_Next_Turn_` 0x83416 (not reset by load) |
| 0x199f39 | `_at_least_one_tactical_combat` | u8 | 1 `Colony_Combat_Screen_`; 0 `Reports_Screen_`, `Main_Screen_Report_Handler_`, `Main_Screen_`, `Draw_Mini_Main_Screen_` (which then fades in) |
| 0x19c18e | `_already_faded_into_main_screen` | u8 | 0 `Harold_Defaults_`; `Fade_Into_Main_Screen_` toggles |
| 0x19c1a0 | `_main_screen_fade_in_flag` | u8 | `Reports_Screen_`; 0 `Fade_Into_Main_Screen_`, `Harold_Defaults_` |
| 0x19c1a2 | `_force_main_screen_fade` | u8 | `Reports_Screen_`, `Hotseat_Select_Player_`; 0 `Fade_Into_Main_Screen_`, `Begin_Of_Turn_` |
| 0x199f38 | `_reloaded_ship_palette` | u8 | `Reports_Screen_`; 0 `Fade_Into_Main_Screen_`, `Vanish_Fleet_Movement_Box_` |
| 0x19c198 | `_next_turn_hit` | u8 | 0 `Harold_Defaults_` 0x794d6, `Do_Next_Turn_`, `Do_Begin_Of_Turn_`; 1 `Main_Screen_` |
| 0x19c194 | `_check_next_turn_exit_field` | u8 | `Do_Begin_Of_Turn_`, `Do_Next_Turn_` |
| 0x19c1a1 | `_in_reports_phase` | u8 | 1/0 around `Do_Begin_Of_Turn_` |
| 0x199f09 | `_global_var` | u8 | 0 `Do_Begin_Of_Turn_`, `Harold_Defaults_` |
| 0x19c19d | `_officer_screen_return_screen` | u8 | 0x27 `Do_Begin_Of_Turn_` |
| 0x199f1d | `_mainscr_palette` | u8 | 0xff `Main_Screen_Report_Handler_`, `Harold_Defaults_` |
| 0x1ab50d | `done_player_reports` | u8 | save; bit p set by `Display_Report_Aux_` when nothing is left |
| 0x1ab50e | `displayed_gnn` | u8 | save; bit p after `Check_For_Event_` |
| 0x1ab50f | `displayed_msgs` | u8 | save; bit p by `Has_Msg_` (show) |
| 0x1ab504 | `done_colonize_check` | 72 bits | save; `Colonization_` |

Settings (base 0x199bdc, from `MOX.SET`, not the save): +0 0x199bdc summary,
+1 0x199bdd end-of-turn wait, +6 0x199be2 animations, +9 0x199be5 GNN,
+0xb 0x199be7 autosave, +0xc 0x199be8 only serious summary.

## main2_ → Screen_Control_ (0x102bd..0x10365)

1. Byte 0x19a007 (custom player data; cleared by `Check_Command_Line_Parameters_`)
   set: `Read_Moo_Custom_Player_Data_` ok → `Init_New_Game_`, screen 0x27; else 10.
2. Else `argv[1][0] == 'q'` (copy at 0x18ff78) and `DIR("save10.gam")`:
   `Load_Game_(9)`, result ignored; `game_type == 1` → 0x11, else whatever
   `Load_Game_` left (0x27 on success; on a bad version still BSS 0 →
   `Main_Screen_`, open question). Else screen 10.
3. `Ken_Init_` (empty), `Screen_Control_()`.
4. Afterwards a language-dependent goodbye string (settings +0xd2) → 0x199f3c
   → `Exit_With_Message_`.

## Screen_Control_ (0x1049b)

Loop: if done return; `_global_esc = 0`; s = `_current_screen` (u16); s > 0x2b
→ loop (spins); jump table 0x103eb:

| s | Calls | `_previous_screen` |
|---|---|---|
| 0 | `Main_Screen_` 0x86188 | 0 |
| 1 | `Colony_Screen_` 0xc058a | 1 |
| 3 | `Design_Screen_` 0x6b9b2 | 3 |
| 4 | `Fleet_Screen_` 0x73980 | 4 |
| 6 | `Race_Screen_` 0x10acba | 6 |
| 7 | done = 1 | — |
| 8 | `_Game_Popup_` 0x8012f | 8 |
| 9 | `Info_Screen_` 0x106cac | 9 |
| 0xa | `Main_Menu_Screen_` 0x816f2 | 0xa |
| 0xc | `Next_Turn_` 0x13870 | 0xc |
| 0xd | type 0: `Newgame_Screen_`; 0 → screen 10, else call 0x169410(set+0xd4, +0xd6, +0xd7, +0xd5, +0xd8, +0xd9) (patch area, unidentified); other types: `_return_screen = 0xf`, `Newgame_Screen_` | 0xd |
| 0xe | `Hall_Of_Fame_Screen_(-1)`, screen 10 | 0xe |
| 0xf | `Multi_Player_Screen_` 0xf4d99 | 0xf |
| 0x10 | `Hotseat_Screen_` 0x628e2 | 0x10 |
| 0x11 | `Hotseat_Select_Player_` 0xf3ae4 | 0x11 |
| 0x12 | `Planet_Data_Screen_` 0x79c1c | 0x12 |
| 0x14 | `Colony_Summary_Screen_` 0xc4562 | 0x14 |
| 0x15 | `Start_Net_Screen_`; true → 0x27 | 0x15 |
| 0x16 | `Join_Net_Screen_`; true → 0x27 | 0x16 |
| 0x19 | `Build_Queue_Popup_` 0xb4041 | 0x19 |
| 0x1c | `Distance_Between_Stars_Screen_` 0x8ab6d; screen = `_return_screen` | 0x1c |
| 0x1d | `Officers_Screen_` 0x94c1d | 0x1d |
| 0x1e | `Planet_Colonization_In_Main_Screen_` 0x8b2de | 0x1e |
| 0x1f | `Building_Outposts_In_Main_Screen_` 0x8a97a | 0x1f |
| 0x20 | `Planet_Summary_Screen_` 0x9d252 | 0x20 |
| 0x21 | `Colony_Landing_Screen_` 0xb9802 | 0x21 |
| 0x22 | `Unloading_Transports_In_Main_Screen_` 0x8b7a5 | 0x22 |
| 0x23 | `Scrap_Freighters_` 0x8b519 | 0x23 |
| 0x24 | `Tech_Change_` 0x10dbce | 0x24 |
| 0x25 | `Net_Next_Turn_` 0xfc470 | unchanged |
| 0x27 | `Reports_Screen_` 0x8b956 | 0x27 |
| 0x28 | `Turn_Summary_Popup_` 0x10fb02 | 0x28 |
| 0x29 | `Load_Net_Screen_` 0xfb27b; true → 0x27 (shares the 0x16 tail) | **0x16** |
| 0x2b | `Show_Command_Points_Screen_` 0x8bab9 | 0x2b |
| 2,5,0xb,0x13,0x17,0x18,0x1a,0x1b,0x26,0x2a,>0x2b | nothing | busy loop forever |

The Go `screenControl` sends screens it has not ported to the main menu
instead; for the unused numbers it should spin like the original (a hang).

## Reports_Screen_ (0x8b956)

```
_current_screen = 0
r = Must_Display_Reports_(PLAYER_NUM)              ; u8, used throughout
fade = 0
if r: Set_Mouse_List(arrow 0x17cf00, 1)
Set_Refresh_Stencil(); Assign_Auto_Function(Draw_Mini_Main_Screen_ 0x84e9d, 1)
if _stardate_at_beg_of_turn == 0: fade = !(r in {2,3,5,6} or (r == 7 and set+9))
else:                             fade = (_previous_report == 3 and r != 3)
if r == 3 and _previous_screen != 10: fade = 0
elif _at_least_one_tactical_combat and r != 0: fade = 1
if fade and !_already_faded_into_main_screen:
    _at_least_one_tactical_combat = 0
    Allocate_First_Screen_Seg_(); Clear_Fields()
    Set_Main_Screen_Fields_To_Null_()   ; 0x89f8a (alias Clear_Main_Screen_Fields_)
    Update_Star_Owners_()               ; 0x8a503, save state star+0x14
    Build_Ship_Icons_(); Fast_Fade_Out_(); Add_Map_Fields_(); Fade_Into_Main_Screen_()
Do_Begin_Of_Turn_()
if _current_screen == 1: _return_screen = 0x27
if _at_least_one_report: _main_screen_fade_in_flag = 1; Clear_System_Display_Loaded_Flag_()
else: _main_screen_fade_in_flag = 0
if Moveable_Box_Selected_(2) and _at_least_one_report:
    _force_main_screen_fade = 1; _reloaded_ship_palette = 1
if r == 0: _stardate_at_beg_of_turn = stardate
Deactivate_Auto_Function()
```

## Must_Display_Reports_ (0xfe86b)

```
r = player[p]+0x28 == 100 ? Display_Report_Aux_(p, 0) : 0
if r == 0:
    Redirect_Newly_Built_Ships_(p)            ; save state
    if game_type in {2,3}: Do_Autosave_()     ; Save_Game_(9) if set+0xb (stardate%1)
return r
```
`Display_Report_` (0xfe785) = `Display_Report_Aux_(p, 1)`. Callers:
`Reports_Screen_`, `Main_Screen_Report_Handler_` (2×), `Hotseat_Select_Player_`.

### Display_Report_Aux_(p, show) (0xfe63e)

```
if done_player_reports bit p or stardate == 35000: return 0     ; bit not set
r = Antarans_(p, show)
if !r:
    if displayed_gnn bit p == 0 and Check_For_Event_Message_():   ; 0x2023f
        r = 7; if show: Check_For_Event_(); displayed_gnn |= 1<<p
    if !r: r = first non-zero, in order, of (p, show):
        Set_Initial_Tech_, Has_Research_Breakthrough_, Has_Evolutionary_Upgrade_,
        Has_Officer_For_Hire_, Colonization_, Explored_New_Star_,
        Officer_Made_Level_Report_, Has_Report_, Has_Diplomacy_Messages_,
        Has_Msg_, Empty_Colony_Queue_, Need_To_Set_Occupation_Policy_
if !r: done_player_reports |= 1<<p          ; also in query mode
return r
```
`Has_GNN_` (0xfe514) duplicates the GNN test; no caller here.

### The checks

| Routine | Addr | Condition | Query | Show | Show result | Save state (show) | Save state (query) |
|---|---|---|---|---|---|---|---|
| `Antarans_` | 0xfe552 | `stardate−1 == antarans+0x3d` and bit **`PLAYER_NUM`** of `antarans+0x41` clear | 6 | Antaran (owner 8) in-transit (status 1) ship with the largest ETA (+0x6d, strictly greater than 0) → dest = location−500. None, or no `PLAYER_NUM` colony there (star+0x38), or `Random_(10) > 4`: `Play_Antarans_Attack_Flic_` → 1; else `Main_Antaran_Room_Screen_(antarans+0x3d < 35000)` → 6 | 1/6 | `antarans+0x41 \|= 1<<PLAYER_NUM`; RNG (only if a colony destination exists) | none |
| GNN | 0xfe684 | `displayed_gnn` bit p clear, an `event_data[i]+1` ∈ {4,5,6} | 7 | `Check_For_Event_` 0x2027e: `_current_event = 0`; `Setup_Next_Event_(i)` for each such event; if any: `Deactivate_Auto_Function`, `Clear_Fields`, `Reset_File_Animation_Buffer`, fade out when set+9 == 1, `Play_Background_Music_`, `Release_All_Blocks_(screen_seg)`, `Put_Auto_Function_(&_jim_saved_autofunction)`; `_auto_turn_exit = 1`; `Reset_Events_()` | 7 | `displayed_gnn`; event states (not traced) | none |
| `Set_Initial_Tech_` | 0xfd911 | stardate > 35000, +0x321 == 0, `FieldStatus[0]` +0xc4 == 3 | 2 | `Tech_Select_` | 2 | via it | none |
| `Has_Research_Breakthrough_` | 0xfe2e8 | +0x30 ≠ 0 | 1 | `Tech_Select_`, +0x30 = 0 | 2 | +0x30 | none |
| `Has_Evolutionary_Upgrade_` | 0xfe31f | +0x155 (`AppStatus[0x3e]`) == 3, +0x8dd == 0 | 1 | `Setup_Evolutionary_Upgrade_(p)` 0x5b0aa | 1 | via it | none |
| `Has_Officer_For_Hire_` | 0xfe251 | l = +0x5a1 ≠ −1 | 1 | h = `Random_New_Officer_Popup_(p)`; cost = `Calculate_Officer_Cost_(p,l,h)`; `Set_Officer_To_Player_(l,p)`; if h: +0x32 BC −= cost, `Do_Player_Colony_Calculations_(p)`; +0x5a1 = −1 | 1 | leader, BC, +0x5a1 | none |
| `Colonization_` | 0xfdb01 | below | 1 | popups, colonise | 1, or 3 (landing screen) | many | **`done_colonize_check`** |
| `Explored_New_Star_` | 0xfd95a | below | 1 | `Remove_Box_Check_`, `New_System_Discovery_Popup_(star, Draw_Special_(star,p))`, clear star+0x34 bit p, star+0x28 = 0 if marker | 1 | star+0x34, +0x28 | **star+0x34** (omniscient) |
| `Officer_Made_Level_Report_` | 0x949a7 | leader l<67: ((+0x39 == 1 and +0x35 ≠ −1) or +0x39 == 0), +0x3a == p, +0x38 ≠ 0 | 1 | first such: `Officer_Gains_Level_Popup_(loc, l)`, loc = −1 if its ship is in transit (500..999), else `Absolute_Location_` (ship officer +0x23 == 0 → ship's location) or raw; +0x38 = 0 | 1 | +0x38 | none |
| `Has_Report_` | 0xfe359 | a report < `n_reports` with +1 == p | 1 | each match: +0 == 1 and +3 == 0 → `Remove_Box_Check_`, `Show_Off_Stolen_Tech_(s16 +6, u8 +5)`; else `sprintf_report_` + `Message_Box_`; +1 = −1 | 2 if last match was show-off, else 1 | report +1 | none |
| `Has_Diplomacy_Messages_` | 0xfe408 | i<NUM_PLAYERS, i≠p, player[p]+0xe60 bit i clear, player[i]+0x657+p ≠ 0 | 5 | each i: `Npc_Diplomacy_Screen_(i,0)`, clear (network: interactive → `Net_Request_AI_Diplomacy_` wait first) | 5 | +0x657 | none |
| `Has_Msg_` | 0xfe0ea | below | 4 | screen 0x28, return 0x27, `_global_esc = 1`, `displayed_msgs \|= 1<<p` | 4 | `displayed_msgs` | none |
| `Empty_Colony_Queue_` | 0xfdf93 | last colony with owner p, +6 == 0, +0x115 == −1 | 3 | `Remove_Box_Check_`; screen 1, return 0x27, `_global_esc = 1`, `_screen_data` 0x199830/32 = planet+2/+3 | 3 | none | none |
| `Need_To_Set_Occupation_Policy_` | 0xfe02c | colony (from end) owner p, +0x12f == 4, a colonist dword (+0xc+4k, k < +0xa, from end) with byte1 & 4 | 1 | `Remove_Box_Check_`; screen 1, return 0x27, `_screen_data`; calls `Colony_Screen_()` directly | 1 | via screen | none |

Codes: 0 none; 1 popup over the mini main screen; 2 full screen (tech,
show-off); 3 leaves via `_global_esc` (colony, landing); 4 turn summary (0x28);
5 diplomacy; 6 Antarans; 7 GNN (full screen only with set+9). 2/5/6/7+GNN fade back.

**Colonization_(p, show).** Stars `NUM_STARS−1..0`, `star+0x37 == p`,
done bit clear: base = `Player_Colony_Base_At_Star_` 0xfda3f (orbit 4..0 colony
owned by p with +0x141 > 0); colony/outpost = `Player_Ship_Type_At_Star_` 0xfdaa7
(p, type 1 / 4, star, 0): last ship with that type, owner, location, status < 5,
`just_built` +0x80 == 0. Found if base, or a ship and `star+0x35` bit p clear →
query 1. Otherwise `Set_Bit_Field_(done_colonize_check, star)` and continue (query
writes). Show: selection/confirmation/star-name popups, colony base trashing (BC +=
`DAT_0017ec16/2`), `Make_New_Colony_`/`Make_New_Outpost_`, `Kill_Ship_`,
`Remove_Building_(c, 0xb)`, `Update_Ship_Icons_In_Between_Turns_`; declined →
done bit + star+0x35 bit; with animations (set+6) a new colony → screen 0x21,
`_global_esc = 1`, return 0x27, result 3. Not on the load path.

**Explored_New_Star_(p, show).** From the last star down, first with star+0x34
bit p or marker (the special, s8 +0x28, ≥ 50 and %10 == p; `Draw_Special_`
0xfd875 shows 50..59 as 2, 60.. as 3, 70.. as 7, 80.. as 8 to that player
and 90 and up as 10 to everyone; plain specials 4, 5, 6, 10, 11 show
themselves, others 0). None → 0. Omniscient (+0x8ba) and
not marker → clear bit p of +0x34 and return **0** (one star per call, also in
query). Else 1.

**Has_Msg_(p, show).** Nothing if `displayed_msgs` bit p, `N_Msgs_For_Player_ ≤ 0`
or set+0 == 0. Scan p's window from the last message down, stop at the first
serious; ns = (set+0xc == 0):

| type | serious |
|---|---|
| 0 | +3 == 4 or 9 yes; 6..8 no; else ns |
| 1..3, 6, 10..15, >17 | ns |
| 4, 5, 7 | yes |
| 8, 16 | no |
| 9 | yes if s16 +2 ≠ −1 and +7 ≠ 0, else no |
| 17 | no if +3 == `PLAYER_NUM`, else ns |

### Redirect_Newly_Built_Ships_(p) (0xfe793), save state

```
changed = 0
for ship NUM_SHIPS-1..0 with owner +0x63 == p and +0x80 != 0:
    +0x80 = 0; changed = 1
    if status +0x64 == 0 and star[loc]+0x54+2p != -1 (Star_Has_Relocation_ 0x78c94):
        d = Relocation_(loc, p) 0x784f0; ids = {ship, -1}
        Ships_Try_To_Move_To_(ids, d, buf) 0xff799
        if buf[5]: Make_Ships_Move_To_(ids, d, buf) 0xffd08
if changed: Do_Player_Colony_Calculations_(p) 0xe2d72
```

## Do_Begin_Of_Turn_ (0x8ad82)

```
_in_reports_phase = 1; 0x199f09 = 0
Begin_Of_Turn_(&rep); _officer_screen_return_screen = 0x27
enemy = Enemy_Ship_Going_To_My_Colony_()
if !set+1 and !rep and !something_interesting_happened[PLAYER_NUM]
   and _next_turn_hit and game_type == 0 and !enemy:
    ; auto next turn: 0x199988 = -1; _check_next_turn_exit_field = 1; banner
    ; H_Message_(0xe9) with player+0x15, stardate/10, %10 in Draw_Darkened_Box_
    ; over Draw_Mini_Main_Screen_, flip; Do_Next_Turn_(&x, 1)
else: _check_next_turn_exit_field = 0; Set_Mouse_List(arrow,1); _next_turn_hit = 0; Clear_Fields()
_in_reports_phase = 0
```
After a load `_next_turn_hit` = 0 → always the else branch.

Checked against the original: after TURN,
two turns run on their own with no input, and every draw up to the fifth
Next_Turn_Calc_, the state after the fourth and the captures match.

The automatic next turn in full (0x8adfc): `_animated_star` = −1,
`_check_next_turn_exit_field` = 1, `Disable_Draw_All_Fields`,
`Assign_Auto_Function(Draw_Mini_Main_Screen_, 1)`, `Set_Page_Off`,
`Draw_Mini_Main_Screen_`; `Get_Player_Font_Colors_(PLAYER_NUM, 3, c)` (hl 0),
`Set_Font_Style(3, c)`; the message is H_Message_ 0xe9 "%s, stardate %d.%d"
with the race name (player+0x15). Box w = width+8, h = font height+4 at
x = (0x1fa−w)/2+0x15, y = (400−h)/2+0x15 (C division), colour c[1]; text at
x+8, y+6. `Set_Palette_Changes(0, 0xff)`, `Update_Glass_Remap_Colors`,
`Draw_Darkened_Box_`, `Print`, flip, `Enable_Draw_All_Fields`, then
`Do_Next_Turn_` with DL = 1.

`Enemy_Ship_Going_To_My_Colony_` (0x77eed): over the ship stacks
(`_ship_stack_start`, `_ship_stack_count`), the first ship of each:
`Ship_Owner_` ≠ PLAYER_NUM and `Player_Has_Colony_(PLAYER_NUM,
Absolute_Location_(ship+0x65))` → 1 (stops at the first).

### Do_Next_Turn_ (0x83411)

TURN on the main screen first does `Assert_Settings_`, `_next_turn_hit` = 1,
`_auto_turn_exit` = 0, `_global_exit_field` = −1000 (Main_Screen_ 0x86d78).
Then: `_at_least_one_report` = `_previous_report` = `_just_moved_stack` = 0,
`_warn_about_cheaters` = 1 << PLAYER_NUM, `_animated_star` = −1; hotseat:
`Save_Hotseat_Map_Info_`, screen 0x11, player+0x2d = 1; network screen 0x25,
else 0xc, return screen 0x27; `_mainscr_palette` = 0x65,
`_ship_icon_player_id` = −1, `Clear_System_Display_Loaded_Flag_`.
- DL = 0: `_in_auto_next_turn_mode` = 0, `Clear_Fields`,
  `Clear_Main_Screen_Fields_`, and with set+1 off the full-screen hidden
  field (hot key Esc, `_global_exit_field`).
- DL = 1: `_in_auto_next_turn_mode` = 1, `Increment_Mouse_Clock_Image_`
  (0x8f855: `_mouse_list_clock` 0x182248 + (n>>2)·12, n = 0..15 cycling),
  `_auto_turn_exit` = (char)`Get_Input()`. Nonzero (the stale exit field
  clicked, Esc): `_check_next_turn_exit_field` = 0, arrow pointer,
  `_next_turn_hit` = 0, `Clear_Fields`, screen 0 — the run stops.

`_auto_turn_exit` is written elsewhere (Draw_Mini_Main_Screen_ on a held
button, GNN, the Antaran room) but never read; only Get_Input's result
stops the run. Next_Turn_Calc_ ends with the arrow pointer (0x17cf00) only
when `_in_auto_next_turn_mode` is 0 (ported, with the empty auto function
it sets first).

### Begin_Of_Turn_(rep) (0x8a84b)

```
*rep = 0; Main_Screen_Report_Handler_(rep)
if _global_esc == 1: return
Allocate_First_Screen_Seg_()
0x199966 (_ship_icon_player_id) = 0x19995c (_eta_star_id) = 0x1999f2 = 0x1999f4 = -1
Set_Main_Screen_Fields_To_Null_(); Update_Star_Owners_(); Build_Ship_Icons_()
if game_type != 1 and (Box_Was_Open_(2) or Moveable_Box_Selected_(2)):
    _fleet_box_ship_stack 0x1999b8 = Ship_Stack_From_Saved_Array_()
    if -1: Vanish_Fleet_Movement_Box_(); 0x199bd5 = 0; 0x199bd3 = _box_off_flag[2]
Disable_Draw_All_Fields(); Assign_Auto_Function(Draw_Main_Screen_ 0x84555, 2)
Box_Was_Open_(0); open2 = Box_Was_Open_(2); Restore_Boxes_()
if open2 or Moveable_Box_Selected_(2): Reload_Ship_Selection_Status_()
if open2:
    _fleet_box_ship_stack = Ship_Stack_From_Saved_Array_()
    if -1: box2.content 0x193016 = _box_off_flag[2]
    else: content = 1; Fade_Into_Main_Screen_(); _force_main_screen_fade = 0
Enable_Draw_All_Fields()
```

### Main_Screen_Report_Handler_(rep) (0x8b17b)

```
*rep = Must_Display_Reports_(PLAYER_NUM)
if _stardate_at_beg_of_turn and != stardate: _already_faded_into_main_screen = 0
if !*rep:
    if _at_least_one_tactical_combat:
        if Moveable_Box_Selected_(2) or (0): _force_main_screen_fade = 1
        _at_least_one_tactical_combat = 0
    return
_at_least_one_report = 1; _mainscr_palette = 0xff
Assign_Auto_Function(Draw_Mini_Main_Screen_, 1)
if _stardate_at_beg_of_turn != stardate:
    Is_Main_Screen_Palette_() ? (Set_Page_Off; Draw_Mini_Main_Screen_; _TOGGLE_PAGES_) : Fade_Into_Main_Screen_()
    _stardate_at_beg_of_turn = stardate
loop:
    _already_faded_into_main_screen = 0
    Clear_System_Display_Ship_Fields_(); Clear_System_Display_Loaded_Flag_(); Allocate_First_Screen_Seg_()
    _previous_report = Display_Report_(PLAYER_NUM)
    full = _previous_report in {2,5,6} or (== 7 and set+9)
    if _global_esc == 1: return
    n = Must_Display_Reports_(PLAYER_NUM)
    if full and n != 3: _skip_draw_system_planets 0x19d32c = Moveable_Box_Selected_(0); Fade_Into_Main_Screen_(); _already_faded_into_main_screen = 1
    if n == 0: Assign_Auto_Function(Draw_Main_Screen_, 1); return
```

### Helpers

| Routine | Addr | Behaviour |
|---|---|---|
| `Enemy_Ship_Going_To_My_Colony_` | 0x77eed | 1 if a stack (`_ship_stack_count`/`_ship_stack_start`, first node) of another owner has `Player_Has_Colony_(PLAYER_NUM, Absolute_Location_(loc))`; read only, needs `Build_Ship_Icons_`' stacks |
| `Is_Main_Screen_Palette_` | 0x860c3 | `_previous_screen` ∈ {0,0xb,0x1e,0x1f,0x23,0x1c,0x24,8,0x27,0x2b,0x28,0xc} |
| `Moveable_Box_Selected_(b)` | 0x918d5 | `_box_off_flag[b]` (0x1992c0) < `moveable_box[b].content` (0x192fde+0x1c·b) |
| `Box_Was_Open_(b)` | 0x91999 | `_saved_boxes` byte 0x199bcf+3b |
| `Restore_Boxes_` | 0x9191d | b<5, saved ≠ −1000: content = saved; was-open → b0 `Init_System_Display_`, b2 `Get_Stack_Start_From_Saved_Ship_Array_` or `Vanish_Fleet_Movement_Box_`; reset to 0 / −1000 |
| `Allocate_First_Screen_Seg_` | 0x82348 | `_other_ships_screen_seg` = first block of screen_seg (76000); `_global_bitmap_seg` = next (bitmap 60×60). EAX ignored |
| `Set_Main_Screen_Fields_To_Null_` | 0x89f8a | all main-screen field ids (buttons, stars, planets, system ships, cheats) = −1000 |
| `Update_Ship_Icons_In_Between_Turns_` | 0x8a4c4 | star owners, `Build_Ship_Icons_`, `Set_Ship_Icon_XYs_` |
| `Build_Ship_Icons_` | 0xa0305 | `Find_Ship_Stacks_`; `_ship_icon` (12 B, `_ship_icon_count`) per stack with per-star slot counters; Watcom `qsort_(…, qsort_ship_icon_)` when an in-transit icon exists. Cache, not save; see main-screen-ships.md |
| `Fade_Into_Main_Screen_` | 0x8a6c2 | b0 = box 0 open; if already-faded: clear, return. Else clear fade-in/force flags, already-faded = 1, disable fields, stencil, mouse none, `Fast_Fade_Out_`, `Load_Palette(0,0,255)`, `Darken_Palette_(100)`, `Add_Map_Fields_`; b0: `Do_System_Popup_(box0.content)` with set+4 held 0; off page `Draw_Mini_Main_Screen_`, `Clear_Palette_Changes(0,255)`, flip, mouse arrow; `Fast_Fade_In_` unless `_reloaded_ship_palette` and box 2 closed (`Set_Fade_Percent_(0)`); `_reloaded_ship_palette = 0`; enable fields; `Mox_Update_` between steps |
| `Draw_Mini_Main_Screen_` | 0x84e9d | if tactical-combat flag: clear + `Fade_Into_Main_Screen_`; `Draw_Main_Screen_Filled_(1)` (0 once with `_dont_draw_main_screen_to_back`), `Draw_Main_Main_Screen_`, `_multi_colored_line_start = (+1)%8`, restore map x/y/scale/zoom; exit-field: mouse → `_auto_turn_exit` (if !set+1), darkened turn button (0x220,0x1b9) |
| `Add_Map_Fields_` | 0x81d73 | main-screen buttons, windows, moveable box fields, star fields (clip 0x16..0x20f × 0x16..0x1a5), exit hot key, grid; zoomed-in variant |
| `Clear_System_Display_Loaded_Flag_` | 0xa20cd | zero 20 B of `_star_name_loaded_string` |
| `Clear_System_Display_Ship_Fields_` | 0xa20ec | zero `_system_display_ships` (0xb4); 15 buttons −1000/−1 |
| `Vanish_Fleet_Movement_Box_` | 0x91a40 | box2.content = `_box_off_flag[2]`, fields −1000, `Clear_Fleet_Selection_Status_Flags_`, `Set_Main_Screen_Help_List_`, `_reloaded_ship_palette = 0` |
| `Ship_Stack_From_Saved_Array_` | 0x789d4 | stack of first ship (<500, status<3) in `_fleet_box_ship_ids`, else first stack of its owner, else −1 |
| `Reload_Ship_Selection_Status_` | 0x71b1c | own fleet-box stack: per node selected = selection bit && `Ship_Can_Be_Selected_` |

## Hotseat_Select_Player_ (0xf3ae4), screen 0x11

1. `Fade_Into_Screen_(Fade_In_Function_ 0xf3ac8)`; auto `Hotseat_Draw_Next_Player_`
   0xf34ea (2); `Clear_Fields`; `Set_Input_Delay(3)`.
2. `Hotseat_Load_Next_Player_` 0xf390e: `Push_Block(screen_seg)`; `_mp_screen =
   &_g_mp_screen`, zero 0x1eb; `MULTIGM.LBX` 0, 0x13, 0x14, 0x15; x = (640−w13)/2
   (+0xf3); `Hotseat_Set_Next_Player_Status_` 0xf3d67: +0x110+i =
   `Hotseat_Player_Status_` (0 not live human; 1 human with +0x2d TurnDone; 2 to
   move); +0x1e3 = live humans, +0x1e5 = last one, +0x1e7 = (status-1 humans ==
   +0x1e3); +0x1e1 = count of 1/2; y = max(0,(480−(h15+n·h14+h13))/2) (+0xf5);
   `Hotseat_Get_Colors_`.
3. Outline colour 0xf0; pulse +0x1e8 = 0, +0x1e9 = 1.
4. One live human: `game_type = 0`, pick it. All done: no input. Else
   `Hotseat_Add_Next_Player_Fields_` 0xf33ee (hidden field +0xbb, banner +0xa7 per
   status 1/2 human; eliminated → −1000) and `Fade_Into_Screen_(Hotseat_Draw_Next_Player_, 0xb)`.
5. Input loop: click on a status-2 player's field → pick, `_force_main_screen_fade = 1`;
   hover change resets the pulse; redraw off-page + flip.
6. `Clear_Fields`, `Set_Input_Delay(3)`, `Deactivate_Auto_Function`,
   `Pop_Block(screen_seg)`, `_mp_screen = 0`.
7. All done: screen 0xc, return 0x11. Else `Restore_Hotseat_Map_Info_`;
   `PLAYER_NUM` = pick; screen 0x27, return 0x11; `Restore_Hotseat_Map_Info_`;
   `Center_Map_(star.x +0xf, star.y +0x11)` of planet[player+0x29]+2.
8. `Must_Display_Reports_(PLAYER_NUM)` ≠ 0 → `Update_Ship_Icons_In_Between_Turns_`,
   `_stardate_at_beg_of_turn = stardate`, `Fade_Into_Main_Screen_`.

`Restore_Hotseat_Map_Info_` 0x88e5a, from `_hotseat_system_box` 0x19c0a0 /
`_hotseat_fleet_box` 0x19c0e0 (8 B/player) / `_hotseat_map_scale` 0x19c120: box0
x, y (min 0x16), content; box2 x (min 0x16), y (**< 0x72 → 0x16**), content; box2
open → `Set_Hotseat_Fleet_Box_To_Last_Stack_`; `cur_map_scale` = saved or
`max_map_scale` if 0; `zoom_count = Map_Scale_To_Zoom_Level_()`. After a load
`Harold_Defaults_` zeroes the three arrays (game_type 1) and sets each player's
box0 content (+4) to −1.

`Center_Map_` 0x8238b: x − up(0xfd), y − up(200), clamp 0..MAP_MAX − up(0x1fa/400);
if moved `Get_New_Paralax_XYs_(oldx, oldy, 1)`; `_draw_main_screen_to_back = 1`.

## Boxes after a load

`Harold_Defaults_` 0x793e5..0x79416: for b<5 `_saved_boxes[b] = −1000`,
was-open 0, `moveable_box[b].content = _box_off_flag[b]` (−1,0,0,0,0). Every box
is closed after `Load_Game_`; `Restore_Boxes_` is a no-op.

## What a loaded save triggers

Probe `build/probe/reports`. All ten saves have the human's
`done_player_reports` bit set (SAVE10 0x3, others 0x1) → `Display_Report_Aux_`
returns 0 at once. Nothing shows, even though SAVE1, 2, 4–8 hold unshown
messages with `displayed_msgs`/`displayed_gnn` = 0. No human ship is
`just_built`; no network game, so no autosave.

SAVE10 (Continue): stardate 35001, hotseat, humans 0 and 1, player 1 TurnDone:
1. Screen 0x11: picker shows (2 humans, 1 done); only player 0 is clickable →
   `PLAYER_NUM = 0`, force-fade = 1, map scale = max, centred on the home star
   of planet 175. `Must_Display_Reports_(0)` = 0.
2. Screen 0x27 (previous 0x11): r = 0, `_stardate_at_beg_of_turn` = 0 → fade →
   full fade-in block; `_already_faded_into_main_screen` ends at 1, force-fade cleared.
3. `Do_Begin_Of_Turn_`: handler r = 0; allocate, icon globals −1, null fields,
   star owners, ship icons; box-2 test skipped; auto `Draw_Main_Screen_`;
   `Restore_Boxes_` no-op; enemy test; else branch.
4. Fade-in flag 0, `_stardate_at_beg_of_turn = 35001`; → `Main_Screen_`.

Needed for that path: `Screen_Control_` 0x11/0x27/0; `Hotseat_Select_Player_` +
loader/draw/fields/`Restore_Hotseat_Map_Info_`/`Center_Map_`/`Get_New_Paralax_XYs_`;
`Reports_Screen_`; `Must_Display_Reports_` early exit + empty redirect;
`Do_Begin_Of_Turn_`/`Begin_Of_Turn_`/`Main_Screen_Report_Handler_` r = 0 paths;
`Allocate_First_Screen_Seg_`, `Set_Main_Screen_Fields_To_Null_`,
`Update_Star_Owners_` (ported), `Build_Ship_Icons_` + `Find_Ship_Stacks_`,
`Fast_Fade_Out_`, `Add_Map_Fields_`, `Fade_Into_Main_Screen_`,
`Draw_Mini_Main_Screen_` and the main-screen drawing. Only save-state change:
star owners.

The single-player saves (SAVE1..9) take the same path from step 2 on, with
`_previous_screen` 10.

## Port status

Go: `internal/game/reports.go` (Reports_Screen_, Must_Display_Reports_,
Redirect_Newly_Built_Ships_, Do_Begin_Of_Turn_, Begin_Of_Turn_,
Main_Screen_Report_Handler_ with its loop), `reports_checks.go`
(Display_Report_Aux_ and every check), `reports_popups.go`,
`diplomacy_screen.go` (`docs/screens/diplomacy-screen.md`).

Every check runs in query mode as the original, with the query-time writes
(done_colonize_check, the omniscient star+0x34, done_player_reports). Show
mode is ported for Has_Diplomacy_Messages_ (the AI ambassador,
`diplomacy-screen.md`), Has_Officer_For_Hire_, Explored_New_Star_,
Officer_Made_Level_Report_ (the popups of `main-popups.md`), Has_Report_
(the spy reports' message boxes and the stolen-tech show-off),
Set_Initial_Tech_ and Has_Research_Breakthrough_ (Tech_Select_ and the
science room, `tech-select.md`), Has_Msg_ (screen 0x28, the turn summary,
`turn-summary.md`) and
Empty_Colony_Queue_ (screen 1), Colonization_ (`colonization.md`; with
animations it leaves for the unported landing screen). GNN (`gnn.md`) and Setup_Evolutionary_Upgrade_ (`evolution.md`) are
ported; the Antaran room too (`gnn.md`); the colony screen opens for an
empty queue (emptyq1, `TestEmptyQueueColonyScreen`) and runs for a new
occupation policy (occup1, `TestOccupationColonyScreen`). Begin_Of_Turn_ restores the boxes as above (`Restore_Boxes_` reopens the
system display and checks the fleet box's stack; `Fade_Into_Main_Screen_`
runs `Do_System_Popup_` with the exploding box switched off; box1,
`build/oracle/box1.sh`: s2 with Sol's box open, TURN, the level popup;
`TestBoxesAfterTurn` matches captures, draws and the save; fbox2, the
fleet box of Zuban's stack the same way: `TestFleetBoxAfterTurn`). Not ported: the Antaran attack flic. A show path that is not ported logs, makes the change
that ends its condition where the state allows (Colonization_ declines and
marks the star; a breakthrough clears +0x30) and is passed over by its query
for the rest of the stardate (`reportState.skipped`), so the handler's loop
ends as it would after the player's answer. Screens 1 and 0x21 go back to
their return screen while unported.

## Open questions

- 0x169410 in `Screen_Control_` case 0xd (patch area).
- `main2_` `q` quick load leaves screen 0 when `Load_Game_` fails.
- Not traced: `Reset_Events_`'s restart, `Tech_Select_`, `Relocation_`, `Ships_Try_To_Move_To_`,
  `Make_Ships_Move_To_`, `Do_Player_Colony_Calculations_`.
- The special (star+0x28) ≥ 50 with %10 == player: who writes this marker.
- Colonist byte1 bit 0x04 and colony +0x141 names are inferred.
- `Antarans_` tests `PLAYER_NUM`'s bit, not p's (matters only in hotseat).
