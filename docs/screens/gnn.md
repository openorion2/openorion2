# Event news, the Antaran messenger and evolution in the report phase

Three report checks of `Display_Report_Aux_` (`reports-and-begin-of-turn.md`).
Addresses are ELF. Go: `internal/game/gnn.go`, `antaran_room.go`,
`reports_checks.go` (`setupEvolutionaryUpgrade`).

## Check_For_Event_ (0x2027e), report code 7

`_current_event` = 0; `Setup_Next_Event_(i)` for every event (0..0x23) in
state 4, 5 or 6 (record `_event_data` 0x19aba4, 9 bytes: +0 player (s8),
+1 state, +2 s8, +3, +5, +7 s16). With news: deactivate the auto function,
`Clear_Fields`, `Reset_File_Animation_Buffer`, with GNN on (settings+9) a
fade out, music, `_auto_turn_exit`, blocks released, `_jim_saved_autofunction`
back. Always `_auto_turn_exit` = 1 and `Reset_Events_` (every 20 stardates,
with no idle event left, `Init_Events_`: records −1/0, `_event_delay_count`
0, `_last_event_year` = stardate). `displayed_gnn` gets p's bit.

Port-only: with GNN off the news comes as a box over the map
(`Mini_Main_Screen_Text_Box_`) and nothing ever set
`_jim_saved_autofunction`, so the original put back a stale or null auto
function; the next popup of the report phase (a discovery, say) then drew
over a map without stars, names and the side panel's figures, as the
original does (checked in DOSBox). The port puts back the auto function
it found instead (`TestGNNBoxKeepsAutoFunction`). With GNN on it is as the
original.

## Setup_Next_Event_(i) (0x21371)

A jump table (0x212e1) by event picks the message variant (ESI, −1 = no
news) and the arguments from the record and the state; locals a case does
not set are stale stack, EDI stale register (the port uses 0). Cases
0x1d, 0x20..0x23 draw `Random_(4)` for the variant; 0x13..0x17 in state 4
clear +3 (the planet) after reading its star, 0x13 in state 6 writes −1
there too. With news and GNN on: before the first news (`_current_event`
0, 0x19acf0) `Start_Main_Event_` and the opening message
`Get_Event_Message_(0, Random_(4) − 1)`, before the others "in other news"
`Get_Event_Message_(1, Random_(4) − 1)` with 0x19acef = 0; the picture
0x19aa70 = 0; `Drive_Event_Screen_`. Then `_current_event` +1; GNN on:
`Load_Event_Picture_(i)`. `Get_Event_Message_(i+2, variant, …)`; GNN on:
0x19acef = 0, `Drive_Event_Screen_`; GNN off → jimtext.lbx 0x1d as the
title and `Mini_Main_Screen_Text_Box_` (`Text_Box_`, then black, the mini
main screen, flip, `_draw_main_screen_to_back`), `Clear_Fields`.

## The GNN screen (EVENTS.C)

- `Start_Main_Event_` (0x2031d): help off, fields drawn, window reset, clip
  off, auto function off, `Clear_Fields`, the blank full-screen cursor
  list 0x180ea8, music 0x76, fade out (0 %, speed 10), input delay 1,
  `Clear_Fields`, stencil refresh, `Get_Auto_Function_` into
  `_jim_saved_autofunction` (0x19aba0, also the council's), auto off,
  `Event_Fade_In_`, auto = `Draw_Event_Screen_` (delay 2), outline colour
  (0, 0xf8), style 4 outline with colours 0x180e7c.
- `Event_Fade_In_` (0x20538): EVENTS.LBX 0 as the 30×30 cursor bitmap and
  the palette (`Draw_Palette_`, changes 0..0xff), black fill, 0x19acef = 1,
  entry 1 (the studio) as the file animation drawn off page and copied
  to the back page, the cursor into mouse image 0, the arrow list.
- `Load_Event_Picture_(i)` (0x203cb): EVENTS.LBX entry i+2.
- `Drive_Event_Screen_` (0x20400): `Mark_Time_`, `Clear_Fields`, a hidden
  full-screen field with no hot key, `Get_Input_`; that field ends it,
  keys do nothing; else draw, flip, `Release_Time_(2)`.
- `Draw_Event_Screen_` (0x20460): off page; with animations the studio's
  stencil frame (0 → 1). If 0x19acef is 0 it becomes 1 and only the
  picture's palette goes in (the frame shows no text). Else the picture at
  (0x140, 0xe) and the message: h = formatted max height (width 0x226),
  h > 0x3c → h + 0xe, else h > 0x12 → h − 4; printed formatted, centred,
  at (0x37, 0x1bd − h/2). Music 0x76.

`GNN_Reports_Combat_` (0xea8ca) belongs to network games (called by
`Do_Nonhuman_Attack_Against_Human_` and `Do_Net_Combat_Update_` only): a
`Random_(2)` picks the order of two player names for a chat line. Not ported.

## Get_Event_Message_ (0x21b6d)

Record 0 (300 bytes) of entry n·4 + variant of EVENTMSE.LBX (EVENTMSG, -F,
-S, -I, -P by language). Codes: 0x80 colony BX's planet, 0x81/0x82 numbers
CX / arg 5, 0x83/0x84 an English plural "s" for them, 0x85 race of arg 6,
0x86/0x87 field name of arg 7 / CX, 0x88 emperor of arg 8, 0x89 leader arg
9, 0x8a race arg 10, 0x8b star arg 11, 0x8c mineral class CX, 0x8d/0x8e
application arg 12 / 13, 0x8f/0x90 and 0x91/0x92 "you"/"your" (jimtext2
0x3d / 0x3e, the race name otherwise) for args 6 and 10, capitalised at
the start of the message (0x8f its second byte). Event 0x1f appends a
ranking: "\n", "\x1a0", then the living players sorted (bubble, larger
first) by variant 0 building costs (`Compute_Total_Building_Costs_`), 1
population, 2 known tech cost (+0x224), 3 `Event_Get_Fleet_Strength_`
(1 + hull size of ships with status ≤ 1); the first six as
"\x07X%d.%d. %s" (x, rank, race name), x from 0x78 with GNN on else 0xa,
+0x7d per column, "\n" after every third.

## Antarans_ (report code 1 or 6)

The attack cinematic (`Play_Antarans_Attack_Flic_`, a Smacker video) is
not ported; it changes no state. `Main_Antaran_Room_Screen_(attack date <
35000)` (0x14aac): blank cursor, fade out, `Reload_Main_Antaran_Room_Screen_`
(antaroom.LBX 0 the cursor and palette, entry 1 the room as a file
animation, first frame on both pages), `Get_Antaran_Room_Message_(1 −
arg)`: **`Random_(4)`** picks record 0 (200 bytes) of antarmsg.LBX entry
set·4 + language·8 + r − 1. Loop: a hidden field over the screen; a click
on it or any key leaves; else draw (with animations the room's next frame,
the message in style 4 outline `_antaran_room_normal_colors` 0x180170 as a
paragraph at (0x37, 0x1d6 − height) width 0x226), flip, `Release_Time(2)`.
Leaving: `_auto_turn_exit`, fade out, the auto function back.

## Setup_Evolutionary_Upgrade_(p) (0x5b0aa)

See `evolution.md`.

## Checks against the original

gnn2 = s8 with GNN on and animations off (`build/turns/gnn2/MOX.SET`,
keys `build/turns/gnn2.keys`, Enter for each screen): `TestTurnGnn2Screen`
compares the four screens (opening, event 5, "in other news", event 0x1e)
with the original's captures pixel for pixel, all 790 draws and the F10
save. gnn3 adds event 0x1f (state 6, variant 1) to it
(`build/turns/inputs/gnn3.GAM`): the ranking. gnn4 (`build/turns/gnn4.sh`)
is gnn3 with animations on: `TestTurnGnn4Animated` finds each of the six
captures among the port's frames of that screen (the studio's stencil
phase depends on when Enter came), all 791 draws match, and the F10 save
matches gnn3's (gnn4's F10 was lost).

gnn1 = s8 (two events with news: 5 and 0x1e) with t1's MOX.SET (GNN
off): see `TestTurnGnn1Events`. Esc closes the first news box and
is ignored on the second, a quirk of the field array: `Main_Screen_` has
turned cancel off (`Disable_Cancel_`, never undone), so Esc only works as a
hot key; `Do_Next_Turn_` left a full-screen Esc field at field 1; the first
box saves the fields above the main screen's, but after `Setup_Next_Event_`'s
`Clear_Fields` the second saves above field 1, so that stale field becomes
its field 0 and Esc matches it, which means nothing. Enter and F10 work. Keys: `build/turns/gnn1.keys`. ant1 = a1 with the seed's low byte +4
(`build/turns/inputs/ant1.GAM`), found by running the port's turn: the
attack heads for a human colony and `Random_(10)` = 1, so the messenger
speaks. `TestTurnAnt1Messenger` plays it end to end: the room, then t1's
tribute. Keyboard keys do not leave the room (Get_Input_ eats a lone key
before Keyboard_Status_ looks), so the trace leaves it with Enter, which
clicks the full-screen field under the pointer. Keys: `build/turns/ant1.keys`.
