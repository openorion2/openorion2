# Detailed ship view (right click on a ship)

The popup a right click on a ship opens: the ship picture in its shield
rings, the fleet icon or officer portrait, combat bonuses, crew, drive,
armour, computer, shield, weapons and specials. It runs its own input loop
and returns to the caller, which redraws its screen.

Sources: `CMBTDRW1.C`-area combat drawing (`Detailed_View_Ship_` 0x31f25,
`Combat_Draw_View_Ship_` 0x30631, `Remap_Draw_Ship_Status_` 0x33199,
`Crew_Quality_String_` 0x33cfa), the combat ship loader
(`Load_Display_Combat_Ship_` 0x4cb1e, `Init_Internal_Space_` 0x36470),
bonuses (`Get_Ship_Combat_Bonuses_` 0x54e5b, `Get_Design_Combat_Bonuses_`
0x54d80, `Helmsman_Bonus_` 0x3677d, `Weaponry_Bonus_` 0x367ae), the
strategic-combat refit (`Update_Strategic_Ship_` 0x5794b and the `Best_*`
routines 0x56726..0x57675), `Ken_Get_Text_Message_` 0x58f1e,
`Get_Text_Message_` 0x249f9, engine `Interlaced_Fill` 0x128ede, `Box`
0x1297c3, `Trim` 0x13303e. Callers: fleet box (`Scan_Moveable_Box_Fields_`
0x89448), 0x33cb8 (combat), 0x73da8, 0x95281. Port:
`internal/game/shipview.go`, `shipview_data.go`.

## Detailed_View_Ship_(ship) (0x31f25)

`none = ship == -1`.

1. `Save_Help_List_` (count, active flag and pointer of the help list);
   `Get_Auto_Function_(&_k_save)`; `Set_Page_Back`; `_ship_frame--`
   (0x19987a); `Execute_Auto_Function_` (the caller's screen onto the back
   page); `Draw_Visible_Fields_Back_` (`Draw_Visible_Fields` with
   `Set_Page_Back`); `Copy_Back_To_Off`; `_TOGGLE_PAGES_`.
2. `Assign_Auto_Function(Combat_Draw_View_Ship_, 1)`; `Push_Block`.
3. Unless `none`: `_ship_data_id = -1`; `Load_Display_Combat_Ship_(ship)`;
   `strcpy(name, record name)` (16-byte local); `Get_Ship_Combat_Bonuses_(ship,
   EDX = &+0x36, EBX = &+0x38, ECX = &+0x34)` (registers checked in the
   assembly); two 0x62×0x62 bitmaps; `Calculate_Picture_Remap_Colors(Combat.lbx
   0xb)`; `_color_ken` (0x1a125c, 8 bytes) = remap of 0x84, 0x8c, 0x8a, 0x8c ×5.
4. Icon (`_detail_fleet_icon_seg`): officer (+0xad) == -1 →
   `Far_Reload_Next("flticons.lbx", player[owner > 7 ? 0 : owner].race
   (+0x25))`; else `Load_Officer_Picture_(0, officer, seg, 0, none ? 2 : 0)`
   = OFFICER.LBX (name via pointer 0x182278) entry `{0x15, 0xd2, 0x115}[mode]
   + leader.picture (+0x31)`.
5. `Mark_Fields`. Unless `none`: `Set_Font_Style(4, _color_ken)`,
   `Add_Continuous_String_Input_Field(0x106, 0x32, 0x67, 0xc, name, 15, 0,
   NULL, 0, 0)` and `Add_Hidden_Field(0x10c, 0x32, 0x17f, 0x40, "", 0)`.
   Then `f = Add_Hidden_Field(0, 0, 0x27f, 0x1df, "", 0)`.
6. Loop until `Get_Input() == f`: each pass `Set_Page_Off`,
   `Combat_Draw_View_Ship_`, `_TOGGLE_PAGES_` (also after the closing
   click). Only a left click outside the name area closes it; right click and
   Esc give -1 and keep it open.
7. Unless `none`: `Trim(name)` (leading and trailing blanks); if not empty
   `strcpy(ship.name, name)` — **this writes the save state** (the rename,
   or the trimmed original name).
8. `Release_Fields`, `Restore_Help_List_` (`Set_Help_List(saved)` if it was
   active, else `Deactivate_Help_List`), `Put_Auto_Function_(&_k_save)` (the
   auto function stays active), `Pop_Block`.

`Trim` copies from the first non-blank and clears trailing blanks.

### Renaming

The input field (engine `Add_Continuous_String_Input_Field` 0x115bea, type
0xb, docs/engine/fields-and-input.md) edits the 16-byte local copy in
place; a click on it starts editing, Enter or a click elsewhere ends it,
and only the whole-screen field closes the popup. Step 7's `strcpy` writes
the trimmed text over `ship.name` (ship record +0, 0x81-byte rows at
`*0x197f9c`) and leaves the old bytes after the new NUL ("Colony Ship" →
"Colony x\0ip"). The field is added on every caller, the combat scan
(ship -1) excepted.

## The record (_global_combat_data, 0x199a12)

Offsets from 0x199a12 (global addresses in brackets):

| Off | Field | Set by `Load_Display_Combat_Ship_` from ship (+offset) |
|---|---|---|
| +0 | name | `strcpy` of +0 |
| +0x1e [a30] | ship id | |
| +0x20 [a32] | owner | +0x63 |
| +0x24 [a36] | 0 | |
| +0x25 [a37] | hull size | +0x10 |
| +0x26 [a38] | shield class | +0x12 |
| +0x27 [a39] i16 | damage blocked | below |
| +0x29 [a3b] i16×4 | per-arc copy of +0x27 | |
| +0x31 [a43] | drive | +0x13 |
| +0x32 [a44] | computer | +0x15 |
| +0x33 [a45] | armour | +0x16 |
| +0x34/36/38 [a46/48/4a] i16 | beam attack, beam defense, missile evasion | `Get_Ship_Combat_Bonuses_` |
| +0x3b [a4d] i16 | combat speed | below |
| +0x4b [a5d] | 0 | |
| +0x4c [a5e] 5 | specials bit field | +0x17 |
| +0x52 [a64] 8×0xb | weapons | below |
| +0xaa [abc] i16 | structure | `Get_Ship_Structure_` |
| +0xac [abe] | crew level | +0x71 |
| +0xad [abf] i16 | officer | +0x74 |
| +0xaf [ac1] | marines, 0xff none | below |
| +0xb0 [ac2] | 1 = captured (combat only; read here) | not written |
| +0xb2 [ac4] 5 | damaged specials | +0x76 |
| +0xb8 [aca] 8 | 1 | |
| +0xc0 [ad2] i16 | structure damage | +0x7d |
| +0xc2 [ad4] i16 | armour hits left | `Get_Ship_Armor_Hits_ - (+0x7b)` |
| +0xc4 [ad6] i16 | picture | size 5 → 0x2b, else (i8) +0x5c |
| +0xca/cc [adc/ade] i16 | drive / computer hits | `Init_Internal_Space_` |
| +0xfe [b10] i16 | shield hits | `Init_Internal_Space_` |
| +0x100/102 [b12/b14] | current drive / computer | below |
| +0x134 [b46] | current shield | below |
| +0x138 [b4a] | builder | +0x5d |

`Load_Display_Combat_Ship_(id)` (0x4cb1e), in order:

- settings 0xd8 (0x199cb4) == 1 → `Update_Strategic_Ship_(id)` (below).
- Weapons k < 8: if the design type (+0x11) ≠ 0, only `weapon = 0`.
  Else from slot `+0x1c + 8k`: `weapon` (+0, i16), `count` = the slot's
  **working** count (+3), `arc` (+4, 0x10 → 0xf), `mods` (+5, u16), `ammo`
  = (u8) +7; `+8 = 1`, `+9 = count`, `+0xa = 0`. By the weapon type
  (`_weapons` 0x17f803 + 0x1c·w, byte +0xc): 2 → ammo 0; 1 → kept; others
  → max ammo (+0xd). Weapon 0x21: `ammo *= count` (16-bit).
- Marines: design type 0 and (i8) owner < 9 → byte `0x180022 + size·0x24`,
  doubled (byte) when special 0x24 is fitted and not damaged; else 0xff.
- `Init_Internal_Space_` (0x36470): drive hits = drive ? hull +6 : 0, ×3
  with special 0x1b fitted and undamaged; computer hits = hull +4; shield
  hits = shield ? hull +8 : 0 (hull table 0x180024, 0x24-byte rows). It also
  fills per-weapon spaces (+0xce, +0xde, via `One_Weapon_Space_Without_M_Level_`)
  and special spaces (+0xee); the view does not read them (**not ported**).
  The 0x36 bytes from +0xca are copied to +0x100.
- Current values: `shield = (100 - (+0x6e))·shieldhits / 100`, `drive =
  (100 - (+0x6f))·drivehits / 100`, `computer = computerhits - (+0x70)`.
- Damage blocked: shield class 0 → 0; else `(i16)(shields[class] +0x20 · 5)
  · (size < 5 ? size+1 : size+2)` (16-bit), `·3/2` with special 0x14
  undamaged, then `(cur·v + max - 1) / max` (ceil).
- Combat speed: design +0x60, +5 with special 2 undamaged; `a = drivehits·2/3`;
  `speed·(curdrive - a) / (drivehits - a)` (a design without drive hits
  divides by zero: the original crashes, the port returns an error); < 0 → 0;
  +4 for a Trans-Dimensional player owner.

`Get_Ship_Combat_Bonuses_` (0x54e5b), all 16-bit, `/` truncation:
`Get_Design_Combat_Bonuses_` (0x54d80, design by value): attack = computer
row +0x14 (0x17fdec + 0x16·c), +0x32 with special 5; defense =
(combat speed + 5·special 2)·5, +0x64 special 0x11, +0x32 special 0x12;
evasion = 0x46·sp9 + 0x32·sp11 + 0x19·sp12 + 0x64·sp15 + 0x82·sp26 (hex
bit numbers). Officer ≥ 0: `h = Helmsman_Bonus_`, defense += h, evasion +=
h/2, attack += `Weaponry_Bonus_`; the bonuses read special-skills byte +0x2a
(helmsman) / +0x2b (weaponry) of the leader: bit 6 → (level+1)·5, else bit 7
→ (level+1)·15/2 (`Owned_Officer_Level_`). Crew table 0x17d170 (8 bytes by
level): attack += +0, defense += +2, evasion += (+2)/2. Player owner: race
picks ship defense (+0x8a5) to defense and half to evasion, ship attack
(+0x8a6) to attack, Trans-Dimensional (+0x8bc) defense +0x14.

### Update_Strategic_Ship_ (0x5794b)

Only for owners < 8 with settings 0xd8 ≠ 0. **Changes the save state**: the
ship's design is refitted with `Best_Warp_Drive_`, `Get_FTL_Speed_`,
`Best_Ship_Shield_`, `Best_Computer_`, `Best_Armor_`, then
`Update_Strategic_Design_Weapons_Specials_(design, owner, size)`:

- Slot 0..: beam `Best_Beam_Weapon_` (types {0}, default 3) with
  `Strat_Beam_Weapon_Count_` (hull +0xa, ·3/2 with application 89 known);
  missile `Best_Missile_Weapon_` (types {1, 2}, default 0xe), hull +0xc;
  special `Best_Strat_Special_Weapon_` (types {4, 5} ranked by byte +0x19;
  weapon 0x25 needs size > 3, 0x26 size > 4; default 0), hull +0xe; bomb
  `Best_Bomb_Weapon_` ({3}, default 0x15), hull +0x10. Each pair with
  weapon > 0 and count > 0 writes arc 0x10, mods 0, ammo 5 (bombs 10),
  weapon, count and working count, and takes the next slot (the bomb does not
  advance). Other slots keep their contents.
- The best weapon is the known one among rows 0..0x27 with the greatest i16
  at +0x16, the first on ties (application at +0xa, `player+0x117+app == 3`).
- Specials set (bit > 0): `Best_Beam_To_Hit_Special_` (kind 1),
  `Best_Beam_Damage_Special_` (2, 7), `Best_Missile_Damage_Special_` (5, 7;
  0x22 → 0), 0xd if application 81 is known, `Best_Beam_Defense_Special_`
  (3, 6), `Best_Missile_Defense_Special_` (4, 6): known specials 0..0x26
  (`_specials` 0x17eee0, 0x2f-byte rows: app +6, kind +0x2c, i16 value +0x2d)
  by greatest value.
- `Best_Warp_Drive_`: last known drive 1..6 (0x17fe70, 0x2e rows, app +6);
  `Get_FTL_Speed_` = row +0x2c, +2 Trans-Dimensional; `Best_Computer_`: last
  known 1..5 (0x17fdec, 0x16 rows, app +6); `Best_Ship_Shield_` (0x5679e):
  players last known 1..5 (0x17f6a1, 0x3b rows, app +6), owner 8 → 7, 9 → 3,
  else 0.

The `Best_*` monster branches are not reached from here and not ported.

## Combat_Draw_View_Ship_ (0x30631)

`F` = combat.lbx 5 (the frame, 341×406); `X = 0x149 - F.w/2`, `Y = 0xf9 -
F.h/2`. `m = (+0x100 == -666)`. Unsigned owner compares throughout.

Colours on the main screen (`_current_screen` 0, 4 or 0x1d):
`_green[i]` (0x19b544) = remap(0x80 + 2i), `c = remap(0xc7)`, `a4 =
{remap(0x48), remap(0x4a)×7}`; other screens use the raw indices.
`b4 = {g2, g6, g5, g6×5}`, `ac = {g0, g4, g2, g4×5}`.

1. `Tri_Interlaced_Fill(X, Y, X+0x145, Y+0x183, t, t, 0)` with `t = g2`
   (main) or `g1`; `Interlaced_Fill(X+10, Y+0x1a, X+0x5f, Y+0x69, 0, c)`;
   owner < 8: the same at `X+0xec .. X+0x134`.
2. Main: `Remap_Draw_Ship_Status_(X+10, Y+0x18)`; owner < 8: officer -1 →
   `Remap_Draw(X+0xf6, Y+0x27, icon)`, else `Draw(X+0xec, Y+0x16, icon)`.
   Other screens (combat's scan): `Draw_Ship_Status_` of the scanned
   record; owner ≤ 8: officer -1 → (owner < 8) `Draw(X+0xf6, Y+0x27, icon)`,
   else the officer through `Gray_Scale_Bitmap_` at `(X+0xeb, Y+0x19)`.
3. Style 2 shadow-down `b4`: KEN 8 centred at `(X+0xa7, Y+0x1a)`; fill
   `(X+0x6a, y+0xc, X+0xdb, y+0x30)`. Style 1 `ac`, `y += 0xf`: KEN 9 at
   `X+0x6f`, attack right-aligned at `X+0xd6` with "+" when > 0; `y += 0xc`
   KEN 10 / defense likewise; `y += 0xc` KEN 0xb / evasion + "%"; `y +=
   0x12`. Owner < 9: style 2 `b4`, `Crew_Quality_String_` cut at the first
   "(", centred at `X+0xa6`. `y += 0xe`; marines ≠ 0xff: KEN 0xc and the
   value (current style).
4. Captured (+0xb0 == 1): style 4 `a4`, outline (0, 1), KEN 0xd at `(X+5,
   Y+3)` and right-aligned at `X+0x143`.
5. Style 4 `b4`, outline (0, 1); `Interlaced_Fill(0x10c, 0x31, 0x17f, 0x41,
   0, c)`; off the main screens the name is centred at `(0x145, 0x32)` (on
   them the string input field shows it).
6. Columns `L = X+5`, `R = X+0xa5`, `y = Y+0x76`. Style 2: KEN 0xe "Drive"
   (owner ≥ 10: KEN 0x57); `!m && drive`: style 1 `(cur/max)` right at
   `L+0x9b, y+3`. Style 2: armour name (owner ≥ 10: KEN 0x58) at `R`. Fills
   `(L|R, y+0xc, +0x9b, y+0xe+(owner < 10 ? 0x15 : 8))`. `y += 0xe`, style 1:
   owner < 10: "^ " + drive row +0x2c + KEN 0xf at `L+3`; structure: `!m` →
   KEN 0x59 `(structure - damage, structure)`, else "^ " + (best armour % +
   100) + " " + KEN 0x10, at `R+3`. Owner < 10: `y += 0xc`. "^ " + speed + KEN
   0x11 at `L+3`. Owner < 10: `!m` → KEN 0x5b `(armour left, max)` with max =
   hull armour·armour%/100 + hull armour (hull = size; no drive: 8 for owner
   8, else `Type_Of_Space_Station_(_cmbt_colony_id)`, in combat only), ×3
   with special 0xe; `m` → KEN 0x14 at `R+3+width("^ ")`.
7. Owner ≥ 10: `y += 0xc`. `y += 0x10`. Owner < 10: KEN 0x15 "Computer",
   `(cur/max)` right at `L+0x9b, y+3`; shield < 7: shield name at `R` and,
   class ≠ 0, `(cur/max)` right at `R+0x9b`; else `_buildings[shield]` name.
   Fills `(L|R, y+0xc, +0x9b, y+0x19)`, `y += 0xf`, style 1: computer ≥ 1 →
   KEN 0x16 with computer row +0x14, else KEN 0x17; KEN 0x18 with the damage
   blocked (`m`: owner 8 → 0, else + best shield strength). `y += 0x12`.
8. Style 2: KEN 0x19/0x1a/0x1b at `L`, `L+0x96`, `L+200`; fill `(L, y+0xd,
   L+0x13b, y+0x66)`. Weapon rows from `y+0x10`, 0xb apart (empty slots keep
   their row), style 1 `ac`, spacing 1: `count + " " + name` (singular table
   when count is 1, plural 0x17f807 otherwise) + `" (" arc ")"` unless the arc
   is 1 (arc: 2 KEN 4, 4 KEN 6, 8 KEN 5, 0xf "360", others empty) at `L+3`.
   Ammo by type: 0 "-" at `L+0x99`; 1, 3, 4 the ammo right at `L+0xa1`; 2:
   for each of max-ammo shots, ammo 0 → KEN 0x1c at `L+0x99` (repeated),
   else nested boxes 7/5/3/1 at `L+0x99+9k` (+1 per level), colours 0x49,
   0x4b, 0x4b, 0x4b for k < ammo, else 0x51, 0x54, 0x54, 0x54. Mods: 0 → KEN
   0x1d; else the second-name table 0x17fd0f of each set bit 0..14, joined
   by ", ", at `L+0xcb`.
9. `y = last row + 0x12`; style 2 KEN 0x1e; fill `(L, y+0xd, L+0x13b,
   y+0x3b)`; `y += 0x10`; specials 0..0x26 fitted: style 1 (`a4` when
   damaged, else `ac`), spacing 1, two columns (`L+3`, `R+3`), a new line
   before every even entry but the first.
10. `Draw(X-9, Y-9, F)`.

`Remap_Draw_Ship_Status_`: ship picture (`Get_Ship_Id_Picture_Seg_`) to a
bitmap, added at (0x10, 0xb) into a blank 0x62×0x62 bitmap; shield class ≠
0: outlines 1, 2, then by class 1 {1, 2, g2, g4}, 2 {1, 2, g1, g2, g3, g5},
3 {1, 2, g0, g1, g3, g4, g5, g6}, 4 {1, g0..g7}, 5 {g0, g1, g2, g2, g3, g4,
g4, g5, g6, g7} (6+ none), then colours 1 and 2 become 0; `Draw`.

`Interlaced_Fill(x1, y1, x2, y2, c1, c2)`: clipped; rows from y1 alternate
c1, c2. `Box(x, y, w, h, t, c)`: `t` nested rectangle outlines.

`Crew_Quality_String_(level)`: KEN 0x1f + level with 0, 100, 200, 300, 300
(levels 0..4; others ""). `Ken_Get_Text_Message_(n)`: n < 0x50 KENTEXT.LBX,
else KENTEXT1.LBX with n - 0x50; `Get_Text_Message_` reads record 0 of entry
`language + 6n` (language settings 0xd2, > 5 → 0) and exits with "Message %d
in %s too long" when the length is ≥ 0x4f. Names come from TECHNAME.LBX
(`Load_Tech_Names_` walks; `docs/screens/system-display.md`).

## Port notes and gaps

- The record is a local of `detailedViewShip`, so `+0xb0` (captured) and
  any other field only combat writes are zero; ship -1 shows a zero record
  (the original shows whatever combat left).
- **Not ported**: `Type_Of_Space_Station_`
  outside combat (no combat colony), the weapon/special space tables of
  `Init_Internal_Space_`. The combat variant: docs/screens/combat-screen.md
  (scan).
- `Restore_Help_List_` is ported as re-activating the unchanged list.
- Settings 0xd8 = 1 is strategic combat (`docs/formats/save-misc.md`), so
  the refit runs only in strategic games.
