# Evolutionary Mutation (RACEOPT.C, the _EM_ routines)

Addresses are ELF. Go: `internal/game/evolution.go` over the picks screen
of `raceopt.go` (`docs/screens/new-game.md`). Reached from the report
check `Has_Evolutionary_Upgrade_` (application 0x3e known, +0x8dd clear).

## Setup_Evolutionary_Upgrade_(p) (0x5b0aa)

Human (+0x28 == 100): picks 1..30 copied to the base picks (+0x8bf), base
government (+0x8be) = government & 0xe, `Evolutionary_Mutation_Screen_`
over the base picks (+0x8be), government (+0x89f) restored. AI: picks
1..30 from the base. Then +0x8dd = 1. `Apply_Evolutionary_Upgrades_`
copies the base into the picks on the next turn.

## Evolutionary_Mutation_Screen_(picks) (0x5b649)

Auto function saved; fade out; palette 9. `Convert_Player_To_Flags_EM_`
(0x5b4cd; the group values read back as levels 1..3 against
`_special_bonus` 0x17d1fa; government = picks[0]/2; toggles as
`Convert_Player_To_Flags_`). A local copy of the picks has its group
values turned into levels by comparing with levels 3, 2, 1 in turn (a
level found is compared again). Help list, `Reload_Option_Strings_`,
`Reload_Racial_Option_Screen_`, `Calculate_EM_Picks_` (`Calculate_Picks_`
+ 4); the picks then are the start. Auto `Draw_Racial_Option_Screen_`
(delay 1), colours 0x8e, 0x9d, 0x9c, 0x9d…, `Add_Race_Options_Fields_EM_`,
the buttons saved, background on the back page, the screen, darkened
palette, flip, fade in; a message box with Ken texts 0x4d + 0x4e; redraw.

Loop (`_FULL_DRAW_SCREEN_FLAG` = 0, `Get_Input_`):

- A group level (field si, group si/4, level si%4): group 1 with toggle 9
  → warning 0x49, button 0; the saved level again → button 0; a negative
  cost taking the negative picks below −10 → warning 0x4a, the saved
  level; else the level. Then, whatever happened: if the race's own level
  is above the button and the clicked level's cost is not negative, the
  button goes back to the race's level.
- A toggle: flipped. Turned off: a negative-cost toggle comes back on.
  Turned on: negative picks below −10 → warning 0x4a, off; then the
  exclusive pairs (0/1, 8/9 with group 1 cleared by 9, 10/11, 12/13; not
  5/6, which have no fields here).
- ACCEPT (click only): picks below 0 → warning 0x4b, else done.

Each pass: `Calculate_EM_Picks_`, the background again on a full draw
request, and unless done the fields again, the screen, flip, the saved
buttons = the buttons. Done: `Convert_Flags_To_Player_EM_` (0x5b5a5; the
government byte untouched), help and auto off, the auto function back,
`Fade_Into_Screen_(Draw_Mini_Main_Screen_)` (EDX left at
`&_draw_main_screen_to_back`, set to 1; palette 0).

`Add_Race_Options_Fields_EM_(local, start)` (0x5b14a): Clear_Fields; ACCEPT
at (0x1fa, 0x1c0) with no hot key; Esc and DEFAULT −1000. Group level l
(cost c, the race's level L, cur = the cost of L): a field when (c < 0 and
L == 1) or (c > 0 and (L ≤ l or the button is above L)), and c − cur ≤
start. No government fields. Toggles 4..7 never; else a field unless (c <
0 and the race lacks it) or (c > 0 and the race has it) or c > start.
