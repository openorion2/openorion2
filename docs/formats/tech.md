# Technology, and what a new application sets off

How the code uses research fields and applications, how a player gains an
application (`Player_Gets_Tech_App_`, 0xe4204) and everything that reaches:
government changes, star masks, fuel ranges, ship designs and the colony
economy. The EINSTEIN cheat (`Give_Me_All_Tech_Advances_`, 0x85ff2) is a
loop over it. Go: `internal/game/tech.go`, `tech_designs.go`,
`tech_economy.go`; tests in `tech_test.go`.

## Tables

`_technology_fields` (0x17d904), 0x17-byte rows, 83 fields:

| Off | Type | Meaning |
|---|---|---|
| +0x00 | ptr | name (Load_Tech_Names_) |
| +0x04 | i16 | index |
| +0x08 | i16 | the field researched after this one (0x4b.. are the hyper-advanced fields) |
| +0x0a | 4 x i16 | applications, **empty in the executable** |

`_technology_applications` (0x17e079), 0xd-byte rows, 0xd4 applications:

| Off | Type | Meaning |
|---|---|---|
| +0x00 | ptr | name |
| +0x06 | i16 | field, -1 none |
| +0x0b | u8 | 0: not available when settings byte 0xd8 is set |
| +0x0c | u8 | equipment kind, set by Init_Tech_ |

`Init_Tech_` (0x5e1e3, called once at start-up) writes the kind byte
(buildings 1, shields 2, weapons 3, fuel cells 4, drives 5, armour 7,
computers 8, specials 9, from the `tech app` column of each equipment table)
and fills the four application slots of every field: applications 0..0xd3 in
order go to the first empty slot of their field, skipping fields 0, -1 and
0x4a; a fifth application stops the game ("<n> tech has too many apps"). The
port applies this to the loaded executable image on first use
(`initTech`).

Player record: `FieldStatus` (+0xc4, 83 bytes) and `AppStatus` (+0x117)
hold 0 unknown, 1 obtainable (offered), 2 researchable (fields only), 3
known. `ResearchField`/`ResearchApp` (+0x321/+0x322) are the current target;
+0x323 is set to 1 by every gain. `HyperLevels` (+0x21c) count
hyper-advanced levels.

## Who may get what

`Player_May_Get_Tech_App_` (0xe412b): false if already known. Applications 0
and 0x20 never; 0x34 only with Antarans (settings 0xd3); the government
applications only from the government before them: 0x2a from 0 (feudal),
0x41 from 4, 0x4d from 6, 0x5c from 2. Others: not with settings 0xd8 and a
zero +0xb byte; the index must be below 0xcc and the field not -1.

`Field_Is_Gray_Box_` (0xe41db): fields 0x16, 0x17, 0x1c, 0x1d, 0x37, 0x39.

`Advance_Player_Field_` (0xe401d, player, field, keep): marks the field 3
and its next field 2, repeatedly, until the field reached has an
application with status 1 or its index is 0x4a or more. With `keep` the
first round only checks the field itself.

`Ensure_Uncreative_Field_OK_` (0xe408f): if no application of the field is
offered, one of its unknown applications (not 0x34 without Antarans, not in
field 6, not tactical-only) becomes status 1, chosen by reservoir sampling
with `Random_(n)` per candidate (it moves the RNG).

## Player_Gets_Tech_App_ (0xe4204)

1. `AppStatus[app] = 3`, +0x323 = 1.
2. By application: 0x5c → government 3, 0x4d → 7, 0x2a → 1, 0x41 → 5;
   0x63, 0xaa, 0xb0, 0xb4 → `Compute_Star_Colony_Stuff_` for every star;
   0x18 → also gets 0xe if it may; any other → `Update_Player_Ship_Designs_`
   and `Update_All_Ship_Ranges_`. So the listed applications change neither
   designs nor ranges (SAVE7 shows design costs left stale by 0x63/0xb0).
3. `Do_Player_Colony_Calculations_` for the player (below).
4. Research target: if app is the current `ResearchApp` and the current field
   is 2: for a creative race or a gray-box field, another offered
   application of the field becomes the target (the last one found);
   without one, `Advance_Player_Field_(field, keep)`, target field 0, research
   points 0. Otherwise: an uncreative race runs `Ensure_Uncreative_Field_OK_`
   on the application's field, and if that field is 2 it advances.

`Give_Me_All_Tech_Advances_` calls this for applications 1..0xcb the human
may get, counts them in a short and shows H message 0xdc ("%d new technology
applications are now known.") with `User_Box_`. Check_Hot_Keys_ then sets
`Cheated` = 1 and `CheatFlag` = 2. On the ten saves this gives 102..193
applications; afterwards fields 0..0x49 are 3, 0x4a stays 0 and the
hyper-advanced fields 0x4b.. are 2.

## Star masks, ranges

`Compute_Star_Colony_Stuff_` (0xe5296) rebuilds per-owner masks of the star
from its colonies: +0x38 colonies, +0x39 building 0x2d, +0x3b/+0x3c a
starbase/battlestation/fortress (buildings 0x28, 8, 0x29) with application
0xb4/0xb0, +0x3f building 3, +0x40 building 0xe, +0x3e application 0x63,
+0x3d application 0xaa. The bit is `1 << owner` on a byte.

`Update_All_Ship_Ranges_` (0x10038c), players from last to first:
`FuelRange` (+0x324) from `Update_Player_Ship_Range_` (0x10034d: the i16 range
at 0x17ffe0+i of the first known application at 0x17ffde+i for i = 0x3c,
0x32, .., 0x14, else 0), +0x59f `Best_Warp_Drive_`, +0x5a0
`Calc_Player_FTL_Speed_` (drive speed, +2 trans-dimensional).

## Design updates

`Update_Player_Ship_Designs_` (0x57112):

- settings 0xd8 == 1: `Update_Player_Strategic_Ship_Designs_` (0x574a1).
  Each of the six slots is cleared and rebuilt by
  `Auto_Strategic_Design_Ship_` (0x57871): name `strcpy`d from hull name
  pointer slot (0x18003a + slot*0x24), hull = slot, best drive, shield,
  computer, armour, cost = hull cost (_hull_data 0x18001e, i16) * 3 / 2
  through `Cost_Reduction_For_Govt_Type_`, strategic weapons and specials,
  picture slot*8 (0x2b for slot 5), builder, FTL speed, build date = low 16
  bits of the stardate. The caller then steps the picture: old picture + 1,
  wrapping to slot*8 at slot*8+8; slot 5 keeps 0x2b.
- human player (objective 100): `Update_Automatic_Design_Equipment_`
  (0x572ab: every slot gets the best armour; a better drive replaces the old
  one, FTL speed from it, and the combat speed (byte +0x60) moves by the
  difference of the drive table's per-hull speed bytes at 0x17fe90; the
  cost becomes `Cost_Of_Design_`) and `Update_Equipment_On_Ships_Being_Built_`
  (0x57197: unless the player is eliminated, ships with status 4 or 6 get
  the same drive and armour change).
- AI: `Update_Equipment_On_Ships_Being_Built_` and `Auto_Design_Ship_` for
  slots 0..4. **Not ported** (the port logs it): EINSTEIN only reaches the
  human player.

`Cost_Reduction_For_Govt_Type_` (0x6e1a0): government 0: (2v + 2) / 3;
government 1: (v + 2) / 3; others unchanged.

`Cost_Of_Design_` (0x6b577) fills the design screen's work record and runs
`Update_Calculated_Design_Data_`; only the cost matters here. Quirks: the
**shield is never copied**, so it is always costed as class 0; weapon
miniaturisation and the government reduction use `_PLAYER_NUM`, the rest the
design owner. Parts (`Design_Template_Costs_` 0x6c087, `Total_Design_Cost_`
0x6e777): hull cost; shield, drive, computer, specials and weapons through
`Cost_Given_M_Level_` (0x6b519: 100/75/55/40/30% for levels 0..4, 25% above,
at least 1 for a positive cost) with `Tech_Level_` (0x6e70a: 0 for
application 0 or status 1, else `Miniaturization_Level_` 0x6d048: researched
fields after the application's own, plus the hyper level once the chain
reaches 0x4b); armour = hull cost * armour percent / 100; computer cost 0
becomes 1 for a computer; special 4 is not miniaturised. `Weapon_Cost_`
(0x6ec74): base (i16 +0x10) plus missile ammunition (2:10, 5:20, 10:30,
15:35, 20:40), times the special-mod percent, times (100 + arc extra (beams:
25 for arcs 2/4, 50 for 0x10) + regular mods) / 100, times the count, then
miniaturised. On the saves, 43 of 48 human designs match the saved cost;
the five others are SAVE5's design made this turn on the design screen
(which costs the shield) and SAVE7's four stale costs (above).

Community patch 1.51 (the "feudal bug"): wrapper 0x4001b0 sets
`_PLAYER_NUM` to the design's owner (EAX) around `Cost_Of_Design_` at
three calls: `Update_Automatic_Design_Equipment_` (0x573a1),
`Auto_Design_Ship_` (0x61b36) and `Apply_Production_`'s finished refit
(0xe3aec). There an AI's design and refit cost come from its own
government and miniaturisation, not the human's (1.40b23 made a feudal
human's rivals' ships a third cheaper, and a feudal AI's no cheaper).
`Add_Refited_Ship_To_Queue_` (0xb2290, 0xb22c5) still calls it directly:
the build screen's refit belongs to the player viewing it.

## Colony calculations

`Do_Player_Colony_Calculations_` (0xe2d72): `Pre_Import_Computing_` for the
player's colonies (last to first, outposts skipped), then
`Do_Player_Colony_Post_Production_` (0xe2d09): `Pass_Out_Imports_(p, none)`,
`Colony_Pop_Grows_` and `Colony_Specialty_` per colony, `Update_Player_Stats_`.
`Mox_Update_` calls in between only poll sound and the network.

`Pre_Import_Computing_` (0xe1d59): upkeep cleared; food per farmer (half
units, `Colony_Food2_Per_Farmer_`), climate and can-farm
(`Colony_Environmental_Stuff_`), industry and research per pop, morale,
industry upkeep, industry (and pollution), food, food upkeep, replicators,
`IndustryPaid` = min(upkeep, industry), research, tax (`Colony_Industry_To_Tax_`
moves rate% of the industry above upkeep to BC), BC, BC upkeep. Only the
paths without an explanation buffer (EDX = 0) are ported; the text paths of
`Colony_Food_Production_` alone add the replicated food.

Notes that matter for bit-exactness:

- `Colony_Job_Production_` sums per colonist (job bits 7-8, bit 9 set) in
  1/20 units: base * 20, gravity (4 - factor) * base * -5, governor bonus,
  conquered -5 * base, morale * base (not under unification, not for
  android non-workers), government and blockade terms, AI difficulty bonus
  (0xdd4d7); only positive sums count; result (sum + 10) / 20, food
  (sum + 20) / 40.
- `Colony_Empire_Base_{Food2,Industry,Research}_Produced_` cache per race
  for the last (colony, player) asked; a different colony or player resets
  the cache. Values cached while a colony's per-pop output was lower stay
  until another colony is asked, so a one-colony empire keeps stale values
  after new technology. The port models the three caches from process start
  (no colony).
- `Colony_Pop_Grows_` goes through a patch (0xe1b98 → 0x16946e): if bit 3 of
  byte 14 of star 0's name is set, the growth rate gets +150 before the
  0xc1/0x6b application bonus.
- `Pass_Out_Imports_` (0xdf8f0) works on 16-bit totals; freighters =
  total - 5 * settlers. Without enough food or freighters it feeds the
  starving colonies one unit at a time round robin, in four passes: until
  twice (food + import) covers the first demand group (own race, in half
  units), then the first two groups, three groups, and finally the upkeep.
- `Compute_Player_Maintenance_` (0xe2000) and `Update_Player_Stats_`
  (0xe2710) give command points, the six maintenance parts (colony BC
  upkeep, freighters, command overflow, spies, tribute paid, officers) and
  the empire totals. Tribute received depends on the payer's income at the
  time, so it depends on the order players were last recalculated.

Verification: recalculating every player of the ten saves reproduces every
colony record and every player record byte for byte, except tribute
receivers (SAVE2 player 2); star masks and fuel ranges match too; after
EINSTEIN a further recalculation changes nothing.
