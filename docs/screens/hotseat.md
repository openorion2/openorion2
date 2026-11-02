# Hotseat games

Several humans share one machine: each plays his part of the turn, hands
over through a picker, and the turn runs once all are done. Code:
`internal/game/hotseat.go` (turn loop), `hotseat_setup.go` (setup),
`multiplayer.go` (the multiplayer screen).

## Globals

| Address | Name | Type | Use |
|---|---|---|---|
| 0x199f3a | `_game_type` | u8 | 0 single, 1 hotseat, 2 network, 3 modem; also save byte 0x2d |
| 0x199bf2 | `_settings`+0x16 | u8 | connection picked on the multiplayer screen: 1 network, 2 modem, 3 null modem, 4 hotseat |
| 0x19b880 | `_hotseat_human_players` | s16 | humans set up so far (Hotseat_Screen_) |
| 0x1999aa | `_hot_seat_flag` | s16 | 3 in Hotseat_Screen_, 0 on Esc in Flag_Screen_; never read |
| 0x19c0a0 | `_hotseat_system_box` | 8 × {x, y, content, pad} s16 | moveable box 0 per human |
| 0x19c0e0 | `_hotseat_fleet_box` | 8 × {x, y, content, ship} s16 | moveable box 2 per human, ship of its first icon |
| 0x19c120 | `_hotseat_map_scale` | 8 × u8 | `_cur_map_scale` per human |
| 0x192680 | `_mp_screen` | ptr | → `_g_mp_screen` (0x1aacb4, 0x1eb bytes) while a multiplayer screen runs |

None of the per-human arrays is saved: after a load every human starts
with the defaults. `Harold_Defaults_` (0x79386..) zeroes the three arrays
when `_game_type == 1` and then, for every game type, sets each
`_hotseat_system_box[i].content = _box_off_flag[0]` (−1).

Player record: +0x24 eliminated, +0x28 objective (100 human), +0x29 home
planet, +0x2d turn done.

## Turn loop

`Do_Next_Turn_` (0x83411), hotseat: `Save_Hotseat_Map_Info_`, then
`player[PLAYER_NUM].turn_done = 1`, `_current_screen = 0x11`
(`_return_screen` unchanged). The main screen leaves (the flag argument is
always set).

`Save_Hotseat_Map_Info_` (0x88f5d) stores for `PLAYER_NUM`: box 0's x, y,
content; box 2's x, y, content; `Save_Hotseat_Fleet_Box_Ship_` (0x7872e:
`_ship_node[_fleet_movement_icon[0].node].ship` when box 2 is open, else
−1); `_cur_map_scale`.

`Restore_Hotseat_Map_Info_` (0x88e5a): box 0 x, y (each ≤ 0x15 → 0x16),
content; box 2 x (≤ 0x15 → 0x16), y (**≤ 0x71** → 0x16: the original's
test), content; if box 2 is open, `Set_Hotseat_Fleet_Box_To_Last_Stack_`
(0x78770: `Update_Ship_Icons_In_Between_Turns_`; the stack of the saved
ship if it still belongs to the player and is in space, else −1, which
vanishes the box and sets `_saved_boxes[2] = {_box_off_flag[2], 0}`);
`_cur_map_scale` = saved, or `_max_map_scale` when 0; `_zoom_count =
Map_Scale_To_Zoom_Level_()`.

### Hotseat_Select_Player_ (0xf3ae4), screen 0x11

1. `Fade_Into_Screen_(Fade_In_Function_, palette 0)`: black screen.
2. Auto function `Hotseat_Draw_Next_Player_` (2), `Clear_Fields_`, input
   delay 3, `Hotseat_Load_Next_Player_`, field outline 0xf0.
3. Status of each of the 8 players (`Hotseat_Player_Status_` 0xf3e09: 0
   not a living human, 1 done, 2 to play; `Hotseat_Set_Next_Player_Status_`
   0xf3d67 counts living humans, the last of them, and the done ones).
   - One living human: `_game_type = 0` (the game becomes single player),
     he is picked.
   - All done: the turn runs (`_current_screen = 0xc`, return 0x11).
   - Else `Hotseat_Add_Next_Player_Fields_`, `Fade_Into_Screen_(draw, 0xb)`,
     and the loop: `Get_Input_`, `Scan_Input_` into +0x101; a click on the
     name line or banner of a status-2 player picks him
     (`_force_main_screen_fade = 1`); hovering another field than last
     time restarts the colour pulse.
4. Leaving: `Clear_Fields_`, input delay 3, auto function off,
   `_mp_screen = NULL`. If picked: `Restore_Hotseat_Map_Info_` (still for
   the previous `PLAYER_NUM`), `PLAYER_NUM` = picked, screen 0x27, return
   0x11, `Restore_Hotseat_Map_Info_` again, `Center_Map_` on the home
   planet's star.
5. In both cases: if `Must_Display_Reports_(PLAYER_NUM)` is non-zero,
   `Update_Ship_Icons_In_Between_Turns_`, `_stardate_at_beg_of_turn =
   stardate`, `Fade_Into_Main_Screen_`.

`Next_Turn_` autosaves for game types 0 and 1, runs the turn, goes to
`_return_screen` (0x11) and clears every player's turn-done flag.

### Layout (MULTIGM.LBX)

`Hotseat_Load_Next_Player_` (0xf390e) clears `_g_mp_screen`, loads entry
0 (background), 0x13 (top), 0x14 (row), 0x15 (bottom); x = (640 − top
width)/2; `n` = players of status 1 or 2; y = (480 − (n·row h + top h +
bottom h))/2, at least 0; `Hotseat_Get_Colors_` (two colour tables per
player, from the shade offsets 0,0,1,2,3,4,5,5 at 0xefa52; not used by the
picker's drawing).

`Hotseat_Draw_Next_Player_` (0xf34ea): background at 0, top at (x, y),
n−1 rows, the bottom; font mode 4; for each of the 8 players with status
1 or 2, row r = 0, 1, …:
- `Get_Player_Colors_` (0xf3119): with base = colour·6 + 0x70,
  `Font_Colors2_(mode, base, base+4)`, `(…, base+2)`, `(…, base+5)`.
- status 1: style 4 in the base+2 colours (dim, no banner);
- status 2: `Draw_Multi_Player_Banner_` (entry race·8 + 0x2e + colour at
  (x+0x28, y + r·0x24 + 0x41)), then style 4 in the base+4 colours, or
  when the pointer is on its name or banner field the pulse:
  `Font_Colors2_(4, base, base+3)` with shades 1..7 moved by the phase
  (−2..2, clamped to base+1..base+5), phase += direction, bouncing at ±2;
- spacing 1, `Hotseat_Print_Player_Data_` (0xf3a27): `sprintf(H_Message_
  (0xfc), ruler, race name, Get_Comma_String_(Player_Score_(p)))` at
  (x+0x74, y + r·0x24 + 0x48).

Fields (`Hotseat_Add_Next_Player_Fields_` 0xf33ee, for living humans of
status 1 or 2): name (x+0x6a, y+0x40+0x24·r)..(x+0x1b3, +0x1d), banner
(`Add_Banner_Field_` 0xefaba) the banner picture's rectangle; both hidden,
no hot key, sound 0x29. Eliminated humans get −1000.

`Player_Score_` (0x9ed88) is `Calculate_Hi_Scores_` with the hi-score
block's +0x68 = p, returning +0xae.

`Draw_Player_Done_Lights_` (0x8520c) on the main screen: a light
(`_player_lights[colour]`, x = 0x49 + light width·k, y = 5) for each done
human; frame 5 is reset each draw unless done ≥ remaining humans − 1
(`Get_N_Remaining_Humans_` 0x7a54f). Network games read the done flags
from `_net` (not ported).

## Setup

Main menu MULTIPLAYER → screen 0xf, `Multi_Player_Screen_` (0xf4d99):
MULTIGM.LBX 0 background, 1 box (centred), 2..5 the radio buttons N, M,
U, H (network, modem, null modem, hotseat; frame = on), 6 Cancel (Esc), 7
START NEW GAME (S), 8 LOAD GAME (L), 9 JOIN GAME (J), 0xa COMM INFO (C),
0x100 the game name button (T; the 1.40 patch removed its handler).
`Add_MP_Setup_Screen_Fields_` (0xf009a) adds S, L only when a radio is on,
J unless hotseat, C only for modem state 2 or null modem;
`Draw_MP_Buttons_Down_` draws the absent ones pressed (frame 2).
`Set_Multi_Player_Game_Type_` (0xf5691, jump table at 0xf5681) sets the
radios and `_game_type` from `_settings`+0x16. START with an empty game
name sets it to "Moo II " + `'a' + clock_() & 0x1f`, then screen 0xd.
Leaving saves MOX.SET.

Screen 0xd with `_game_type ≠ 0`: `_return_screen = 0xf`,
`Newgame_Screen_`, whose ACCEPT goes to screen 0x10 for hotseat (0x15 for
network).

`Hotseat_Screen_` (0x628e2), screen 0x10: `_hot_seat_flag = 3`;
`Init_Hotseat_Screen_` (0x627dd: `Reload_Hotseat_Screen_` MULTIGM 0, 0xb
box, 0xc add, 0xd start, 0xe cancel; `Fade_Into_Hotseat_Screen_` only when
humans > 0 — stale on the first call; buttons add (x+0x7d, y+0x63) while
humans < 8, start (x+0x7d, y+0x88), cancel (x+0x8c, y+0xc8, Esc)).
`_hotseat_human_players = 0`, one temporary player record cleared once.
Loop:
- with no human yet the input is ADD;
- Cancel: screen 0xf;
- START: one human → `Do_Confirmation_Box_(H_Message_(0xa2))`, yes makes
  it a single player game; more → `H_Message_(0xa3)` with the humans,
  `H_Message_(0xa5)`, the computer players left and 0xa4/0xa5 for one/
  more; yes starts, no redraws the background;
- ADD: `PLAYER_NUM` = n, `Race_Selection_Screen_(temp)`,
  `Set_Hotseat_Setup_Screen_Help_List_` (0x181fba, 4 entries offset by the
  box corner); picked: the record is copied to player n, n++, humans++,
  objective 100; Esc with no human: back to screen 0xf.
  `Init_Hotseat_Screen_` again; humans = `_settings`+0xd5 (players) starts.
- `Draw_Hotseat_Screen_` (0x626d6): box, the numbers 1..players at
  (x+0xe, y+0x46+9i), bright (0x9a/0x9f) for humans so far, else 0x9a/0x9c,
  shadow down.
Start: `Init_New_Game_` called directly (not the 1.40 wrapper: no Cluster
size bump), screen 0x11.

`Race_Selection_Screen_` and `Racial_Option_Screen_` take the record to
fill as their argument (player 0 for single player).

## Diplomacy between humans

In a hotseat game `Diplomacy_Screen_` takes a human ambassador like an
AI's (network games go to `Get_Net_Diplomacy_Choices_`). The title is
`Decode_Text_Name_And_Stat_(jimtext2 0x43, amb, amb+1)` ("Player 2:
name"); the AI's tech exchange list (`Get_Exchange_Tech_List_`) is not
built for a human (Get_Main_Diplomacy_Choices_, Propose Treaty, Exchange
Technology test the objective first).

- `Diplomacy_Load_New_Ambassador_` (0x1b881): fade out,
  `Diplomacy_Mouse_Init_(amb)` (draws the bad-news music), DIPLOMAT.LBX
  race·2+0xd drawn and copied to the back page, race·2+0xe opened, the
  ambassador set, the stencil drawn.
- Proposal: `Diplomacy_Determine_Treaty_Proposal_` calls
  `Check_Human_Treaty_Proposal_(p, amb, kind)` (0x1b06b): response by
  kind (1 0x61, 2 0x62, 3 0x53, 7 0x63, 8 0x64), p's ambassador loaded,
  `Get_Human_Treaty_Choice_` (0x1b1a7): rows jimtext 0x4f, 0x50 and 0x52
  when `Check_Better_Offer_Available_` (a system to surrender, a tech
  (`Get_Differential_Tech_List_(amb, p)`) or BC > 100); response = old =
  0; the message `Get_Human_Propose_Treaty_Message_` (jimtext2 0x2d, 0x2e,
  0x42, 0x2a, 0x2b, 0x2c for kinds 1, 2, 3, 7, 8, 9/10); title jimtext
  0x4a; `Diplomacy_Generic_List_` (−1 is the last row). Row 0: response
  0x83/0x84/0x87/0x85/0x86, result 3 (accepted); row 1: 0x8a, 0; row 2: 1
  (better offer). amb's ambassador comes back. Back in
  `Diplomacy_Determine_Treaty_Proposal_`, a human amb gets
  `Get_Human_Form_Treaty_Message_(kind, 3 or 0)` (0x188e0: jimtext2
  0x20/0x21 kind 7, 0x22/0x23 kind 8, 0x24/0x25 kinds 9 and 10, 0x26/0x27
  kind 1, 0x28/0x29 kind 2, accepted/not; nothing for peace) into the
  message instead of `NPC_Message_Rejection_Check_`; the message stays
  only while the response does not change (rarely: the response = old
  check turns it into 0x8a).
- Demand: `Diplomacy_Demand_` calls `Get_Human_Demand_Response_(p, amb,
  kind, item)` (0x1b5b8): old = response; `Get_Human_Demand_Message_`
  (jimtext2 0x13..0x1a with the names, stat 5/10, tech or system); p's
  ambassador; `Get_Human_Demand_Choice_` (rows 0x4f/0x50, title jimtext2
  0; row 0 gives in); old = −1; response 0xa9 when given in, else 0xa8;
  amb's ambassador back.

- Better offer: `Diplomacy_Human_Need_Better_Offer_(p, amb)` (0x1bd5b):
  message jimtext2 0x33 (amb's race and name); rows 0x34 system (if p may
  surrender one), 0x35 technology (`Get_Diplomacy_Tech_List_`: up to 10),
  0x40 money (BC > 100); title 0x36, with Cancel. System:
  `Diplomacy_System_Display_(p, ambassador, give)`, amb's ambassador
  loaded (sic), `Diplomacy_Generic_System_List_(star, p, amb)`, the star
  surrendered on yes. Technology: up to 8 names, title `_offer_title`, p's
  ambassador, one-row list (jimtext2 9), `Player_Gets_Tech_App_(amb)` on
  yes. Money: `Diplomacy_Offer_Money_(p, amb, 0)`, p's ambassador, one row
  "%d BC", the sum moves on yes. Then amb's ambassador; true when given.
- `Diplomacy_Generic_System_List_` (0x1a1fd): screen 6 (not restored),
  colony palette, star background 0, the star scanned, its population and
  name texts, title jimtext 0x4b, the demand message (or jimtext 0x4e),
  yes/no at (10, 0x66); auto function synch mode + map + the star's text.
  0 yes, −1 otherwise.
- Gift: `Get_Human_Gift_Response_(p, amb, kind, item)` (0x1b724) for money,
  tribute and a system (a technology gift has no human path: the AI's
  `Get_Gift_Response_` answers it): old = response, the message
  (jimtext2 0x12 money, 0xf tribute 5/10, 0x11 system), p's ambassador,
  `Diplomacy_Display_Response_`, amb's ambassador; response 0xa3, 0xa2,
  0xa1, 0xa4 by kind. No NPC acceptance afterwards.
- Exchange Technology: `Diplomacy_Set_Main_Options_` builds the list for a
  human amb with `Get_Differential_Tech_List_(amb, p)` and, when it is not
  empty, `(p, amb)`. After the pick: p's ambassador, jimtext2 0x30, amb
  picks what to give from `Get_Differential_Tech_List_(amb, p)`, amb's
  ambassador, jimtext2 0x2f and p agrees (0x31) or not (0x32)
  (`Get_Human_Tech_Exchange_First_/_Message_/_Accept_/_Decline_Message_`); agreed:
  relations +12, both techs; no AI list or diplomat modifiers.
- Declare War, Surrender: p's ambassador shows amb jimtext2 0xe / 0xd (p's
  race and name; `Get_Human_Declare_War_Message_`,
  `Get_Human_Surrender_Message_`, inlined in `humanResponseMessage`), then amb's; war sets response 0xa0. Break Treaty: the
  same with jimtext2 0x1b..0x1f naming amb (sic), response and old = 0;
  responses 0x9c, 0x9e, 0xab, 0x99 instead of the AI's messages.

## Not ported / untested

- The second picker's pulse phase is not compared: it advances once per
  pass of the picker's loop, and how many passes the original makes before
  its F12 capture depends on DOSBox's real time.
- Loading a multiplayer save from the multiplayer screen
  (`Get_Valid_MP_Save_Games_`, `Load_Multi_Player_Game_Screen_`), the
  modem screens, network game types.
