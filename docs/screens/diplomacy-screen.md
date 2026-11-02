# The AI ambassador's screen (Npc_Diplomacy_Screen_, DIP-SCRN.C)

The screen an AI player's ambassador uses to deliver a message to the human
at the start of a turn: `Has_Diplomacy_Messages_` (report code 5, see
`reports-and-begin-of-turn.md`) calls it for every player i with
`player[i]+0x657+p` (Go `MessageFlags[p]`) set, then clears that byte.
Addresses are ELF (Watcom + 0x10000). Go: `internal/game/diplomacy_screen.go`,
the list field in `internal/engine/listfield.go`.

## Globals

| Address | Name | Go |
|---|---|---|
| 0x19aa66 | `_current_ambassador` (u8) | `dip.ambassador` |
| 0x19aa40 | `_response_message` (u16) | `dip.response` |
| 0x19aa36 | `_old_response_message` | `dip.oldResponse` |
| 0x19aa65 | `_ambassador_option` (u8) | `dip.option` |
| 0x19aa3c / 0x19aa46 / 0x19aa44 | good / bad / current music | `dip.goodMusic` … |
| 0x19a64a | `_diplomacy_message` | `dip.message` |
| 0x19a744 | `_diplomacy_list_choice`, 0x32-byte rows; a 0 at 0x19a7a8 ends the list | `dip.listChoice` |
| 0x19994c | `_FULL_DRAW_SCREEN_FLAG` | `g.fullDrawScreen` |
| 0x180dcc / 0x180dd4 / 0x180ddc | normal / highlight / special colours | `diplomacy*Colors` |
| 0x180e62 | `_diplomacy_blank_mouse` (image 0 over the screen) | `engine.MouseNone` |

## Npc_Diplomacy_Screen_(amb, delayed) (0x1afa6)

The prologue sits at 0x1afa6; objdump files the body (0x1a570..0x1abbd)
under `Npc_Display_Diplomacy_Messages_`.

```
if player[amb].eliminated (+0x24) == 1: return
Deactivate_Help_List; Enable_Draw_All_Fields; Reset_Window; Clip_Off
Set_Mouse_List(blank); save auto function (_jim_saved_autofunction); deactivate; Clear_Fields
old = -1; _current_ambassador = amb
Set_Fade_Percent_(0); Set_Fade_Speed_Percent_(10); Fast_Fade_Out_
Set_Input_Delay(1); Clear_Fields; Set_Refresh_Stencil; Deactivate_Auto_Function
option = 1
response = delayed ? player[amb]+0x7c9 : player[amb]+0x657+PLAYER_NUM
Diplomacy_Fade_In_()
if response in 0x53..0x55, 0x61..0x68, 0x7d, 0x56, 0x69, 0x6a, 0x1c..0x24:
    option = 2; saved = response
    response = 0x80 + (player[PLAYER_NUM].relations[amb] (+0x617) < -12)   ; the greeting
```

Option 1 (statements): `Diplomacy_Display_Response_`, then by message:
0x30..0x33 → `Break_Trade_`, `Break_Research_`, `Break_Tribute_`,
`Break_Treaties_` (P, amb); 0x39, 0x3a → `Declare_War_(P, amb)`; 0x60, 0x6b →
`Npc_Give_Gift_(amb, P)`.

Option 2 (demands and proposals): unless the ambassador is human (+0x28 ==
100) the greeting is shown (`Diplomacy_Display_Response_`); response =
saved; c = `Get_Non_Player_Proposals_()`.

Accepted (c == 0): `NPC_Proposal_Rejection_Accept_(amb, P, 1)`, then

| message | outcome |
|---|---|
| 0x1c..0x24, 0x69, 0x6a (demands, threats) | response 0x26 shown; `Accepted_Npc_Demand_Resolution_(P, amb)`; `Npc_Give_Gift_(P, amb)` |
| 0x53 | `Declare_Peace_(P, amb)` |
| 0x54 | `Npc_Give_Gift_(P, amb)`, `Declare_Peace_`, `Accepted_Npc_Demand_Resolution_` |
| 0x55 | `Npc_Give_Gift_(amb, P)`, `Declare_Peace_(P, amb)` |
| 0x56 | `Declare_War_(P, ally)`, response 0x58 shown; ally = (s8) player[amb]+0x787+P |
| 0x61 / 0x62 | `Start_Treaty_(P, amb, 1 / 2)` |
| 0x63 / 0x64 | `Start_Trade_Treaty_` / `Start_Research_Treaty_` (P, amb) |
| 0x65 | nothing more (the exchange happened in the list) |
| 0x66 | `Declare_War_(P, ally)` |
| 0x67 | `Npc_Give_Gift_(amb, P)`, `Declare_War_(P, ally)` |
| 0x68 | `Break_Treaties_(P, ally)` |

Rejected: `NPC_Proposal_Rejection_Accept_(amb, P, 0)`, then 0x1c..0x24 → the
four breaks, response 0x25 shown; 0x56 → `Break_Treaties_`, 0x57 shown; 0x68
→ `Break_Treaties_(P, amb)`; 0x69 → `Change_Relations_(-50, amb, P)`, 0x25
shown; 0x6a → if player[amb]+0x837+2P (sneak star) ≠ −1: +0x7c7 = it,
+0x7c9 = +0x887+P (a sneak attack follows), else `Declare_War_(amb, P)`, 0xb3
shown.

Tear-down (0x1ab69): `Fade_Stream_Down(10)`, restore the auto function,
`Clear_Fields`, `Reset_File_Animation_Buffer`, `Release_All_Blocks_(screen_seg)`,
`Set_Fade_Percent_(0)`, speed 10, `Fast_Fade_Out_`, `_jim_current_stream = -1`,
`Play_Background_Music_`.

## Drawing

- `Diplomacy_Mouse_Init_` (0x1b92e): DIPLOMAT.LBX entry `race` (the race
  icon) becomes cursor image 0 and gives the palette (`Draw_Palette`,
  `Update_Glass_Remap_Colors`, `Set_Palette_Changes(0,255)`), off page
  filled black twice; dipstars.lbx 0..11 and the text buffers are loaded
  (`Load_Diplomacy_Text_Messages_`: jimtext2 0x43 player number name, 0x44
  "<race> Ambassador", 0x45 "you were framed", 0x48 tribute %, 0x49/0x4a
  trade/research treaty, …); good music = race + 1, current = −1, **bad
  music = 0xd + `Random_(3)`**.
- `Diplomacy_Fade_In_` (0x17c07): Mouse_Init, `Get_Diplomacy_Statement_` of
  the message (only the draw; `_old_response_message` stays −1),
  `Setup_Back_Page_`, arrow cursor. With option ≠ 0: auto function
  `Draw_Diplomacy_Fade_In_Screen_`, file animation frame 1, and with
  animations (settings +6) up to 0x26 frames: hidden full-screen field,
  `Get_Input`, a click on it or a key skips; else off page,
  `Draw_File_Animation_Stencil`, flip, `Release_Time(2)`. Then
  `Setup_Ambassador_Pic_(skipped)`.
- `Setup_Back_Page_` (0x17b51): `_FULL_DRAW_SCREEN_FLAG = 0`; open
  DIPLOMAT.LBX entry `race*2 + 0xd` (the room and the ambassador's
  entrance), off page black, `Draw_File_Animation_` (frame 0), off → back.
- `Setup_Ambassador_Pic_(skipped)` (0x17bb2): open entry `race*2 + 0xe` (the
  idle loop), frame 0 when skipped else 1; without animations draw it once.
- `Draw_File_Animation_Stencil_` (0x12cd2d): the frame (kind 0 sprites only)
  goes through `Draw_Animated_Sprite_Back_Diff_` into the current **and**
  the back page, no clipping, marking each run (x..x+count) of its row;
  every frame but the last is marked with the refresh mode set to 2. Marking
  whole rows instead dumps stale off-page pixels (the text outline above
  the first line) and the frame no longer matches the original.
- `Draw_Diplomacy_Synch_Mode_` (0x16f00), the auto function: full-draw flag →
  Setup_Back_Page_ + Setup_Ambassador_Pic_(1); with option ≠ 0 and
  animations the next frame (frame 0 is skipped); outline colour (0, 0xf8),
  style 4 outline in the normal colours; **a changed message (< 1000) is
  worded anew** (one `Random_` per message shown, not per frame); the text
  as a formatted paragraph at (0x50, 0x1b8 − h/2), width 0x1d6, type 2; the
  title "<race> Ambassador" (`Decode_Text_Race_And_Name_`, max 0x78) centred
  at (0x140, 10), style 4 shadow-down, colour by banner (+0x26) from
  {f3, f0, fd, e3, f5, eb, f4, f1}, outline (0, 0); network chat lines;
  outline colour back to (0xf8, 0xf8).
- `Diplomacy_Display_Response_` (0x19919): auto function Synch_Mode, a
  hidden full-screen field (empty hot key) until it is clicked; each pass
  draws and flips with `Release_Time(2)`.

## Get_Non_Player_Proposals_ (0x1ac12)

Title jimtext 0x4a ("What do you think?"); for 0x1c..0x24, 0x53..0x56,
0x61..0x64, 0x66..0x6a, 0x7d the rows jimtext 0x4f, 0x50 (Accept, Reject).
Auto function Synch_Mode (delay 2), input delay 1, variable {1, 0, −1000, −1}
(row 1 highlighted until the pointer moves), style 4 normal colours,
highlight and special colours, `Set_Font_LF(4)`, help entry 0x2b6 over the
screen. 0x65 runs the tech exchange list (`Diplomacy_Generic_Exchange_Tech_List_`
and `Player_Gets_Tech_App_` both ways; not ported: logged and declined);
0x7d waits for a click; else `Get_List_Field(10, 0x76, 0xf5, title, rows,
0x32, var, all active, fill 0x11)`. −1 becomes the fallback (1, or −1 for
0x65); a result equal to var[2] becomes −1.

## Get_List_Field (0x11c4c1) and string list fields (type 0xa)

Rows are `Add_String_List_Field` (0x11c83d) fields: rect (x, y−1, x+w,
y+height+1) with the height at add time (LF included), value = row index,
the list variable, active flag, fill type and parameters, the list colour
arrays. Rows start one height + vertical spacing below the title; the
first active row is the default. The font is then reset to its style (the
LF goes), the list colours copied from the current normal, highlight and
special colours, and the title added as a row of value 1 over a variable 0.
The loop: `Get_Input`; a field ≠ 0 ends it unless it is an inactive row or
the title; between polls the auto function, then `Scan_Input` moves the
highlight to the row under the pointer, `Toggle_Pages`, `Release_Time(auto
delay)`. Returns field − 1 (−1 for cancel). Rows have empty hot keys, so they
need a click (Enter only picks a field under the pointer).

`Draw_Visible_Fields` type 0xa (0x11f4dc): style with the row's colours; not
selected → `Print(x1, y1+1)` (inactive rows in the special colours);
selected with fill 0x11 → in the highlight colours, other fills
`Gradient_Fill_` first (not ported). `Draw_Field_` draws nothing for type 0xa;
`Scan_Input` returns 0 over an inactive row; `Push_Field_Down_` from a list
row draws nothing.

## Get_Diplomacy_Statement_(msg, p, amb) (0x1d565)

DIPLOMSE.LBX (language 1..5: G, F, S, I, P) entry msg (msg − 1 for the
"framed" messages 0x14, 0x16, 0x1d, 0x1f, 0x28, 0x2a, 0x31, 0x33, 0x3a, 0x3c,
0x48, 0x4a, 0x8f, 0x91 and for 0x6a), record 0 of 0x1452 bytes: +0 good
news flag, +1 count n, then n wordings of 200 bytes. **`Random_(n)`** picks
one. Codes: 0x80 p's name, 0x81 p's race, 0x82 amb's race, 0x83 amb's name,
0x84/0x85 `An_` of p's/amb's race, 0x86/0x93 star name of amb+0x67f+2p,
0x87/0x88/0x91 race of the ally (amb+0x787+p), 0x89 tech name of
amb+0x7af+p, 0x8a treaty label (status 1/2) or trade/research treaty text,
0x8b ally's name, 0x8c jimtext2 0x4a+amb+0x6e7+p (1..4, else "FWEE"), 0x8d
building amb+0x66f+2p, 0x8e stardate, 0x8f nothing, 0x90/0x95 the offer
(tribute % via jimtext2 0x48, tech, "%d BC", star via jimtext2 0x3a), 0x92
treaty label or trade/research text by amb+0x6df+p, 0x94 race of
amb+0x7f6+p. Framed messages get "you were framed" appended. The music is
the good or bad one by the flag.

## Outcome routines

- `Accepted_Npc_Demand_Resolution_(p, amb)` (0x1abbe): **`Random_(13)`**;
  `Change_Relations_(r + 11, p, amb)`; DemandState (+0x71f) both ways 0.
- `Npc_Give_Gift_(from, to)` (0x1aeb5), by from's terms for to: +0x75f
  (tribute 1 → 5 %, else 10 %) → `Start_Tribute_Treaty_(from, to, 1/2)`;
  +0x767 → `Player_Gets_Tech_App_(to, it)`; +0x777+2·to BC moved, from's BC
  floored at 0 when below 1; +0x7b7 star → `May_Surrender_Star_` then
  `Surrender_Star_`.
- `NPC_Proposal_Rejection_Accept_(amb, p, accepted)` (0x276e6): bias v =
  amb+0x827+2p. Accepted: v ≥ 0 or personality 0 → v+1; personality 1..5 →
  1; else v. Rejected: v ≤ 0 → v−1; personality 0 → −1; 1..3 → v/2; 4 → −1
  if amb+0x727+p == 1 else v; 5 → v−1; else v. Stored both ways.
- `Start_Trade_Treaty_` (0x5223f) / `Start_Research_Treaty_` (0x52150): each
  side's treaty timer + the peace duration of its diplomacy personality,
  then `Start_Trade_Agreement_` (0x101ee3: value −`Base_Trade_Agreement_Goal_`,
  goal `Trade_Agreement_Goal_`, both sides, `Update_Player_Stats_`) /
  `Start_Research_Agreement_` (0x101f82: value −min(research pops)/2, goal
  `Research_Agreement_Goal_`).

## Check against the original (t1)

`TestTurnT1Tribute` (`internal/game/turn_e2e_test.go`) plays t1 from SAVE1
through the port's screens with scripted input (TURN; the greeting; the
first row, Accept; the "saw the light" response; F10): all 1520 draws match
`build/turns/t1/rng.log` (the last four: bad music `Random(3)`, the
statements of 0x6a, 0x80, 0x6a and 0x26, then `Random(13)` = 7) and SAVE10
equals after.GAM except the description. With `OPENORION2_E2E_SHOTS=dir` it
saves the greeting, demand, accepted and next main screen frames;
`OPENORION2_E2E_SEQ=1` saves 70 consecutive frames of each. At the matching
animation frame all four are pixel-identical to the original's F12
captures in `build/turns/t1cap/scr0..3.png` (0 pixels differ).

## The player's diplomacy (Diplomacy_Screen_, AUDIENCE)

Go: `internal/game/diplomacy_human.go` (menus), `diplomacy_ai.go` (the
answers, DIPLOMAC.C), `diplomacy_system.go` (the system map). Reached from
AUDIENCE on the races screen (`Race_Screen_` 0x10b94c in single player; the
screen restarts afterwards).

### Diplomacy_Screen_(amb) (0x16c4e)

Music −1, `_synch_up_established_flag` 0, help off, all fields drawable,
window reset, clipping off, blank mouse list, auto function off, fields
cleared, `_old_response_message` −1, `_net_diplomacy_esc_pressed` 0,
ambassador = amb; fade out (0, speed 10), input delay 1, fields cleared,
refresh stencil. Then:

- `Find_Worst_Modifier_(P, amb)` ≤ −100 or `_COUNCIL_FLAG` 2 or 3: response
  0x7e + (P's treaty with amb < 4), option 0 — the ambassador only speaks
  (`Diplomacy_Display_Response_`).
- else response 0x80 + (P's relation to amb < −12), option 1.

`Diplomacy_Fade_In_` as for the AI's screen. With option 1:
`Get_Main_Repulsive_Diplomacy_Choices_(P, amb, 0)` when either race is
Repulsive (Picks[19] == 1), else `Get_Main_Diplomacy_Choices_`. Network
games (`Get_Net_Diplomacy_Choices_`) and human ambassadors are not ported.
Tear-down: stream −1, music, auto function off, fields cleared, the file
animation buffer and screen_seg released, fade out.

### Menus

All lists are `Get_List_Field(10, 0x76, 0xf5, title, rows, …, fill 0x11)`
with the variable at 0, the auto function `Draw_Diplomacy_Synch_Mode_`
(delay 2), input delay 1, style 4 in the diplomacy colours with
highlight/special colours and `Set_Font_LF(4)`. The main and repulsive
menus use `Set_Font_Style_Outline_` with outline colour (0, 0xf8), the
others plain `Set_Font_Style_`. `Setup_Diplomacy_Help_(n, ids)` (0x1fc13)
gives rows 0..n−2 help (x 0xa..0xff from y 0x76 + one row height down) and
entry n−1 the whole screen (0x2b6); the id tables are in .data
(0x180de4 main, 0x180e0c repulsive, 0x180e22 propose, 0x180e30 break,
0x180e3e demand, 0x180e54 offer). Texts: `Load_Diplomacy_Text_Messages_`
(0x1f37c), jimtext 0x1e..0x46, jimtext2 2, 0x38..0x4a.

**Main** (`Get_Main_Diplomacy_Choices_` 0x1758c): first
`Get_Exchange_Tech_List_(P, amb)`; then a loop: rows active by
`Diplomacy_Set_Main_Options_` (0x17847) — at war (treaty ≥ 4) no Break,
Demand, Exchange or War; Exchange needs `_exchange_tech_count` > 0; Break
needs a trade/research treaty, tribute or treaty 1/2; Propose needs one of
`Valid_Treaty_Proposal_` 7, 8, 3, 1, 2; Surrender `May_Surrender_Empire_`.
The 1.31 patch (0x1692a2) turns Exchange off when bit 2 of byte 14 of star
0's name is set. Worst modifier ≤ −100 ends the audience: the pending
response (if new) and then 0x82 are shown. Choices: Propose, Break,
Demand, Offer Gift, Exchange Tech (each after `_diplomacy_message` = ""),
Declare War (on yes: the response, end), Surrender (on yes: end), Good Bye.

**Repulsive** (0x17227): Peace Treaty (only at war;
`Diplomacy_Determine_Treaty_Proposal_(…, 3)`, `Adjust_Diplomat_Modifiers_`,
`Declare_Peace_` on yes), Declare War (only at peace), Surrender, Good Bye.

**Propose** (0x17d5b): Trade (7), Research (8), Peace (3), Pact (1),
Alliance (2) by `Valid_Treaty_Proposal_` (0x5265e: not the current status;
pact not at war or allied; alliance only from a pact; peace only at war;
trade/research not at war and not already). On a choice: old response −1,
`Diplomacy_Determine_Treaty_Proposal_`, `Adjust_Diplomat_Modifiers_`, P's
modifier (trade, research: 1; peace: 3; pact, alliance: 0) for amb −20, and
on yes `Start_Trade_Treaty_`, `Start_Research_Treaty_`, `Declare_Peace_`,
`Start_Treaty_(1/2)`, then `Get_Exchange_Tech_List_` again.

**Diplomacy_Determine_Treaty_Proposal_** (0x19997): `Check_Treaty_Proposal_
(amb, P, kind)`; 0 no, 3 yes; 1 or 2: `Diplomacy_Need_Better_Offer_` (title
jimtext 0x4a or 0x4b by **Random_(2)**, Agree / Forget It): on Agree P pays
(gold BC to amb, a star surrendered, a tribute treaty value, a tech to amb)
and the response becomes 0x83/0x84/0x87/0x85/0x86 by kind, else 0x8a. Then
`NPC_Message_Rejection_Check_(amb, P)`; a response equal to the one before
becomes 0x8a.

**Check_Treaty_Proposal_** (0x5272d, a = amb, b = P): `Diplomacy_Test_(a, b,
need, 0)` with need 0x4b (pact), 0x7d + amb+0x6d7 (alliance; result 0 below
relation 50), 0x3c (trade), 0x28 (research), or for peace 100 − 25·wars(a,b)
+ 25·wars(b,a) + (Fleet_Strength_Comparison_ − 100)/2 + 100·attacks(a,b) −
50·attacks(b,a) − 0x7ee − 0x6d7 − Total_War_Loss_Comparison_ − (turns at war
− 9) + relation (result 0 under 10 turns at war). Result 0: a grudge — four
**Random_(4)** chances in turn (0x6df → 0x8b; personality 4 with 0x727 →
0x8c; 0x7ee → 0x8d; 0x6cf → 0x8d + it), else 0x8a. 1/2 (level 4/8): pact
after **Random_(2)** == 1, alliance after **Random_(4)** == 1, peace always,
`Get_Demands_` (pact/alliance: tech, gold; peace: all four) gives the price
(tribute, gold, star, tech in that order of assignment, each with 0xaa and
`Set_Demands_`), else 0 and 0x8a; trade returns 0 untouched, research 0
with 0x8a. 3: **Random_(3)** chance at 0x88 (allied; pact, trade, research),
**Random_(4)** chance at 0x89 (P's +0x6e7 for amb set), else 0x83 pact,
0x84 alliance, 0x87 peace, 0x85 trade, 0x86 research.

**Diplomacy_Test_** (0x53146): v = **Random_(100)** + 0x6d7 + relation +
0x7ee (twice for personality 4) − (need − personality modifier (0x180ccc;
entry 6 for a personality 4 offended, +0x727 == 1)) + modifier[mod]; then
`Adjust_Diplomat_Modifiers_` and the modifier wears off by **Random_(30)** +
20 (0, 1), **Random_(50)** + 20 (2), **Random_(50)** + 50 (3); +50
Charismatic / −50 Repulsive, +25 Telepathic, +30 with application 0xc8,
+ the best diplomat. < −75 → 0, < −50 → 2, < 0 → 1, else 3 (16-bit).

**Break** (0x180d4): Trade, Research, Tribute, Pact, Alliance, Cancel by
P's treaties. `Diplomacy_Break_Treaty_Resolution_`: the break, then the
message: `Get_Break_Trade/Research_Message_` (**Random_(200)** less
`Fleet_Comparison_(amb, P)` less 50 a war, **Random_(100)** against the
personality's chance 0x180db8; above 100 amb may end its pact; the answer is
always 0x9c / 0x9e, the other value overwritten), `Get_Break_Tribute_
Treaty_Message_` (same with **Random_(150)**, above 75 war; 0xab + (pers.
4)), `Get_Break_Treaty_Message_` (above 100 war; 0x99 + (pers. 4); +0x727
set both ways).

**Demand** (0x18b79): Declare War On / Make Peace With / Break Alliance With
(players both know whom amb is at peace with / at war with (single player
only) / allied with; `Get_Diplomacy_Demand_Treaty_List_` 0x1a44d, names in
`Diplomacy_Generic_List_`), 5% / 10% Tribute (neither when tribute flows
either way), Stop Spying, Technology (amb's applications P may get; off
under the 1.31 patch flag, 0x1692c7), Surrender System
(`Check_Surrender_System_(amb, P)`; the system map), Remove Fleet
(`Player_Has_Ships_Pestering_Other_Player_`), Cancel. Each:
old response −1, `Get_Demand_Response_`, `Adjust_Diplomat_Modifiers_`;
on yes (but Stop Spying) `Diplomacy_Give_In_To_Demand_` (amb's +0x897 for
the player 0/1/2, `Start_Tribute_Treaty_(amb, P, 1/2)`, the tech, the star,
amb's +0x80e for P = **Random_(50)** + 10 + **Random_(50)**).

**Get_Demand_Response_** (0x539d9): (`Fleet_Comparison_` +
`Population_Comparison_` (**Random_** of the share) + `Tech_Comparison_`)/3
+ **Random_(200)** + worst modifier + 2·personality modifier, less by kind
(war on X: 75/150/25 by amb's treaty with X, relation/2, X's fleet/4, 25 if
X is an AI; peace: X's personality modifier + 80, 0 if X is human; alliance:
2·relation; tribute 50 / 150; spying, fleet 50; tech: value·100 / P's best
field; star: population·1000 / amb's population, at most 300). Five
`Adjust_Diplomat_Modifiers_`. With a pact or alliance: < −100 (or diplomacy
personality 6) war 0xa5; < −50 the treaties end 0xa6; < −25 0xa7 (demand
counters, base relation −10, copied); < 0 0xa8; else 0xa9 yes. Without:
< −150 war; < −75 0xa7; < 0 0xa8; **exactly 0 sets nothing** (the caller's
flag is stale stack; the port logs and keeps 0); > 0 0xa9 yes. A yes to
Stop Spying sets amb's +0x806[item 0] = **Random_(50)** + 10 + **Random_(50)**.

**Offer Gift** (0x194c5): 5% / 10% Tribute (`Start_Tribute_Treaty_(P,
amb)`), Technology (P's applications amb may get; `Diplomacy_Generic_
Exchange_Tech_List_`), System (the map), Money (`Diplomacy_Offer_Money_`),
Cancel; then `Get_Gift_Response_` and `NPC_Proposal_Rejection_Accept_(amb,
P, 1)`. **Get_Gift_Response_** (0x53723): amb's +0x6e7 for P = kind; worth
w: money (sum/maintenance/10, at most 8), tribute 2, tech (rank·5/P's best
rank), star (P's population there/2 + 1); both +0x817 divided by w;
relation + w·(**Random_(4)** + 4) and base relation + w (each capped at 100),
copied to amb. **Diplomacy_Offer_Money_** (0x1d237): five sums from the
treasury (min 32000; 20..100 below 100 BC), sums above it inactive; a gift
of at least (stardate − 35000)·5 also raises amb's bias.

**Exchange Tech** (0x1cb4d): amb's applications (`_target_tech_list`);
what amb wants (`_exchange_tech_list`) 0 → 0xae and the rejection check;
else "<amb> offers X for Y" (jimtext2 0x2f) and Agree / Forget It via
`Diplomacy_Generic_List_` without Cancel (Forget It comes back as −1); on
Agree jimtext2 0x31, `Change_Relations_(12)`, both get their application,
the lists again, modifier 2 −20. **Get_Exchange_Tech_List_** (0x26d19):
`Get_Tech_Exchange_Reaction_` (**Random_(150)**: relation, 0x6d7, diplomat,
difficulty·−50, bias·20 + 150, Charismatic, app 0xc8, personality; ≤ 0
none, else %); P's applications amb may get valued by amb (· reaction /
100, sorted up), amb's applications P may get valued by P up to the best
of those (sorted down); for each the cheapest of P's worth at least as much
(from the middle when it is above the median), 1 when none, 0 when beyond.

**Diplomacy_Generic_Exchange_Tech_List_** (0x19f26): pages of 11 (8 with
Forget It) with Previous Page / Next Page rows; the item past the page's
end is written and then overwritten by Next Page. **Diplomacy_Generic_
List_** (0x19de8): up to 8 rows and Cancel; without Cancel the count is
decremented, so the last row reads as −1.

**Declare War** (0x179f4): "REALLY DECLARE WAR?!" with one row and Cancel;
on yes `Get_Declare_War_Message_` (0x42 for an offended pact/alliance
personality 4, else 0xa0), `Declare_War_`, `Adjust_Diplomat_Modifiers_`.
**Surrender** (0x1d0f8): one row and Cancel; on yes `Surrender_Empire_`.

### The system map (Diplomacy_System_Display_ 0x1c479)

`_diplomacy_system_giver/receiver`, `_current_screen` 6,
`_using_colony_screen_palette` 1 (restored after), help 0x261 (demand) or
0x268, the title jimtext2 0x3f / 0x37 (receiver's and giver's names), a
5×5 hidden field over every star (`Get_Galaxy_Map_Star_XY_(10, 0xab, 0xb4,
0x6e, big)`) and Cancel (0xf, 0x8b, 0x9d, 0xa2). Each pass: `Get_Input`,
`Scan_Input`; a hovered star of the giver fills the planet list
(`Diplomacy_System_Scan_`: the giver's planets, the capitol's planet as home
world), its population (jimtext2 0x38) and name (0x3a); a click on such a
star ends it when `May_Surrender_Star_` and not a home world; Cancel ends
with −1. `Draw_Diplomacy_System_Display_`: the ambassador, `Box(9, 0xaa,
0xb6, 0x70)`, `Darken_Fill`, `Draw_Galaxy_Map_Box_` mode 4 (DIPSTARS.LBX
1..11 in `_fleet_galaxy_star_seg`, loaded by `Diplomacy_Mouse_Init_`),
"Choose A System:", Cancel (lit while hovered), the hovered star's text
(special colours when unsurrenderable or a home world).

## Not ported

Network ambassadors (`Get_Net_Diplomacy_Choices_`; hotseat ones and the
`Get_Human_*` messages are in `docs/screens/hotseat.md`), the cursor image from the race icon (not shown by
the lists in use), music. The AI's tech exchange proposal (0x65) is ported:
its list is `Diplomacy_Generic_Exchange_Tech_List_` over the ambassador's
applications up to +0x78f's value, and since the chosen index is returned,
only the first application counts as accepted.
