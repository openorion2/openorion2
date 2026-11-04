# The tactical combat screen (CMBTDRW1.C, COMBAT1.C, COMBINIT.C)

Go: `internal/game/combat_screen.go` (state, pictures, palettes, remaps,
close), `combat_draw.go` (Full_Draw/Refresh/Draw_Main, ships, missiles),
`combat_panel.go` (panels, radar, shield arcs, buttons), `combat_fields.go`
(fields, help list, cursors, view), `combat_input.go` (the human's input in
Do_Combat_Turn_), `combat_popups.go` (the human attacker's target popup and
treaty question). The logic it drives is in `turn_tac*.go`
(`docs/combat/tactical.md`). Addresses are ELF.

`tc.scr` is the screen; it is nil when the engine runs without one (the
step tests, `Tactical_Bombardment_`), and then every drawing call is
skipped and the human's input is the stand-in that presses Auto. With an
engine (`g.e`) and no `combatUI` answers, `Tactical_Combat_` opens the
screen.

## Before the battle: the human attacker's popups

`Get_Local_Human_Target_` (0xe7cdb) opens `Defense_Colony_Selection_Popup_`
(0xc9676): the system display popup (as `New_Colony_Selection_Popup_`,
same fields and drawing) titled E 0x45 ("%s select combat at %s", race
name and star). A click on another player's colony returns 15 + colony;
on a fleet icon the fleet's owner, unless that player has a colony or
outpost at the star (E 0x24e); own ships E 0x253, own colony E 0x252, an
empty planet E 0x24f/0x250 (asteroids); CLOSE is −1. `Human_Confirms_
Attack_` (0xe7b79) then lists the treaties (COMBFIND strings: RSTRING
records 60..69, slots filled by `Load_Combfind_Strings_` 0xe6828; treaty
labels are E strings at `_treaty_labels` 0x199150) and asks with
`Do_Confirmation_Box_`.

## Tactical_Combat_'s screen part

After the attackers are loaded (and the Artemis net and the "nothing to
fight" exits): the old auto function saved, fields cleared, the palette
faded out and palette 3 loaded, then COMBAT.LBX buttons (scan 1, options
panel lights 2, self destruct 3, missile warning 4, weapons 7, specials 8,
info 10, crew star 0x2a, auto 0x32, board 0x33, wait 0x34, flee 0x35, done
0x36, warnings 0x25/0x26, ship cursors 0x20..0x22, shield masks 0xc..0x1f),
the 112 CMBTMISL.LBX missile pictures (7 kinds × 16 facings, kind by
`Missile_Type_`), three 0x62×0x62 bitmaps, and the mouse grid (0x36e
regions: whole view, 32×18 squares of 20 px, the panel). The defenders and
`Load_Colony_Defense_` follow (`Load_Combat_Colony_Picture_`: CMBTPLNT.LBX
palette type·6+5, pictures size+type·6 and type·6). Then
`Load_Combat_Ship_Palette_` for both sides (CMBTSHP.LBX colour·0x2d+0x2c
shifted to 0x20 for the attacker, 0x60 for the defender; the defending
colony's radar picture COMBAT.LBX colour·4+size+0x36 recoloured 0x50→0x60),
`Load_Combat_Beam_Stuff_`, the glass remap blocks (shield: slots 1..15 =
(2i+0xd, 2i+0x1e, 0x3f) at 0x17+3(i−1)%, palette 0x90..0xaf black; pulsar
from CMBTSFX 0x34's palette, warp core from SPHERSFX 0, tractor, black hole
CMBTSFX 0x2e, plasma web, stellar converter CMBTSFX 0x28, each only when
the battle has it), `Load_Individual_Ship_Pictures_` (one CMBTSHP picture
per picture number and side, the defender's recoloured 0x20→0x60; the
record keeps it at +0xc6, which travels with the record when
`Deploy_Ships_` sorts), the shield remap restored, the fields, the auto
function `Draw_Main_Combat_Screen_` and the arrow.

After `End_Of_Combat_`: fields, auto function and help cleared, fade out,
palette 0xa0..0xfe black, the galaxy screen's auto function back,
`Update_Ship_Icons_In_Between_Turns_`, the saved glass block,
`Fade_Into_Screen_(Draw_Mini_Main_Screen_)` with palette 0, and the five
option flags written to MOX.SET (0xb4 speed ×2, 0xb5 legal boxes, 0xb6
missile impact warning, 0xb7 shields, 0xb8 grid).

## Drawing

- `Refresh_Combat_Screen_Full_` (0x2c601): fields, grids, then on the back
  page `Full_Draw_Combat_Screen_` (0x2c6ba: black unless in a nebula,
  panel fills, the panel picture COMBAT 0, the Auto button down when on,
  the three star layers (far 0x2e or 0x23 in a nebula, the legal move boxes
  between far and mid, mid 0x2f, near 0x30) each four times for the wrap,
  the current ship's name, status picture, damage bars and crew stars,
  the grid lines, the radar), the weapon or special list, the internals;
  copied to the off page and `Draw_Main_Combat_Screen_` over it.
- `Draw_Main_Combat_Screen_` (0x2f4ee) per frame: the hovered square's box
  when the current (human) ship may move there; the hovered ship's or
  missile's status panel drawn once on the back page (`_last_ship_status`);
  the shield arcs (`Draw_Shield_Arcs_`: four lines and the letters F R B L,
  "A D I S" in Italian, rotated with the facing); every ship on screen
  (`Draw_Ship_`: the record's picture, frame facing·4 + engine phase, the
  four facing quarters mirrored with `Flip_Bitmap`); the current ship's
  cursor (COMBAT 0x20..0x22 by hull, frame by `_ship_frame`); the missiles;
  on the radar the current ship's blinking blip (blink table 0x2c54d) and
  the view's corners — the other blips stay from `Full_Draw` (missile
  warning 2 redraws the whole radar, 1 shows the warning picture instead).
- `Draw_Ship_Status_` (0x2d1ed): the ship still in a 98×98 bitmap ringed by
  outlines by shield level ((max+0x13)/10, five bands), each arc's ring
  colours knocked out by `_shield_mask_bitmap[facing%4][(4−facing/4+arc)%4]`
  as its strength drops; the planet: picture, population, marines and
  armour, its defence buildings.
- Views follow the action: `Fire_Ship_` (0x38e0c) centres on a target
  outside the view before each beam mount, `Resolve_Missile_` on the impact
  point when a missile hurts a ship outside it, `Do_Auto_Ship_Turn_` and
  `Next_Ship_` on the ship to play.
- `Draw_Weapon_Status_Display_` (0x2e2cd): count, mount name (heavy or
  point defence prefix, singular when one shot is left), ammunition or arc;
  colours bright ("SVVV") when the mount can fire at the hovered ship or
  missile, normal ("PSSS"), dark when spent or off.

Engine additions: `Flip_Bitmap`, `Replace_Color_Range` for sprites,
`Shift_Palette_Range`, `Set_Glass_Remap_Color`, Save/Restore glass remap
blocks, `Set_Palette_Gradient`, `Add_Clipped_Hidden_Field`,
`Get_Pointer_Image_Number`.

## Animations

The logic calls the drawing directly; several drawing routines change
state, so they run whenever the screen would animate (`_apply_damage_to_
planet` = 1 and not super fast), screen or not:

- Beams (`Beam_SFX_`, BEAMS.C): the records `Fire_Ship_` collects, per
  frame `Fire_Weapon_` (bolts, wide bolts, generic beams with
  `Bolt_Segment` seed-bracketed jitter), fire points (FIREPTS.LBX), bursts
  and explosions on the target, shield hits (`Draw_Shield_Hit_` brightens
  the ship under the shield picture with `Mask_Brighten_Bitmap`; it ends
  with a `Get_Input`; the picture's size comes from
  `Ship_Size_To_Shield_Size_`, words 0 1 1 2 3 3 at 0xa65f0, so sizes 1
  and 2 share shield size 1; a mask colour below 0xf0 makes the byte
  level wrap and read 256 × level bytes past the 16 brighten palettes in
  the screen segment: not modelled, frame oracles mask such pixels via
  `Game.staleScreen`), reflection fields, the damage numbers
  (`Draw_Damage_Message_Queue_`, nine frames each, colours on the stack
  rows 0xa659f..). `Set_Beam_Colors_` leaves colours 7 and 12 to stale
  stack (logged).
- Missiles: `Resolve_Missile_` adds the damage numbers and plays
  `Draw_Shield_Hit_Once_Through_` when something was absorbed; fighters:
  `Draw_Fighter_` (jitter from `clock_()` behind a restored seed, so it
  is not reproducible), `Fighter_Combat_SFX_`, `Draw_Fighter_Beam_`;
  anti-missile rockets (`Draw_Anti_Missile_Rockets_`, then the kills as a
  number aged by the explosion frames but left in the queue).
- Explosions: `Destroy_Ship_FX_` (kind 1: CMBTSFX 3..5 by size, the record
  gone from the middle frame on; monsters their 16-frame death strip;
  others `Do_Spherical_SFX_`, whose frame loop shakes ships in the wave
  with two Random(5) each and collapses the source with
  `Draw_Ship_Collapsing_`, which sets +0x24 = 5 at scale ≤ 1).
- Cloaks: `Create_Ship_Darken_Map_`, `Plasma_Darken_` (COMBAT 0x31's noise,
  levels by kind), `Draw_Cloak_` (24/60/36 frames by kind) from
  `Fire_Ship_` and the start of `Do_Combat_Turn_`.
- Over a ship (`Draw_Ship_`): energy absorber glow, stasis ball, plasma
  web, black hole; tractor beams (`Draw_Continuing_Tractor_`,
  `Draw_Moving_Tractor_` through `Draw_Bitmap_Line_`, a bitmap stretched
  and rotated along a line — both callers draw them under the following
  `Draw_Main_Combat_Screen_`).
- The black hole generator and the stellar converter (`BHG_` 0x3b627,
  `Stellar_Converter_` 0xad98a, only with `_apply_damage_to_planet` and
  not super fast): CMBTSFX 0x2f (BHG) or 0x28 (converter) drawn into a
  bitmap and stretched with `Draw_Bitmap_Line_`, not glassed. The BHG
  runs 12 steps (2 a frame at double speed) from the firer's centre to
  half the picture's height above the target's, the line drawn first and
  `Draw_Main_Combat_Screen_` over it; `Draw_Palette_` without a refresh,
  sound 0xa, `_dont_draw_ship` = −1 at the end. The converter snaps the
  view to the target, loads the picture's palette and refreshes it, then
  per frame (2 at double speed) the screen, the line and from frame 9 on
  CMBTSFX 0x29 frame f−9 at the target's centre − 0x3c through
  `_stellar_converter_remap_colors` (left in place until the damage);
  sound 0x14. The four arcs' damage and absorption are then summed into
  `Add_Damage_Indicator_For_Ship_` after the shield remap is restored.
- Specials: `Draw_Gyro_Special_Effect_` (turns the target one facing per
  frame of its spin: logic), `Lightning_Field_`, `Draw_Stasis_Beam_`,
  `Draw_Bombs_` with `Draw_Transporter_Bomb_Beam_`, the teleporter's
  `Vanish_Bitmap_` fade, the retreat's jump flash, `Draw_Missile_Status_`.
- Boarding: `Initiate_Boarding_Action_`, `Board_Popup_`, the raid and
  capture reports; scrolling at the edges (`Scroll_Combat_Screen_`).
  Checked against the original by tcb2 and tcb3 ("Oracle").

## Scrolling (port-only pace)

`Scroll_Combat_Screen_` (0x49a41) moves the view while the pointer is
within 5 pixels of an edge: 1 square a frame, 2 from the third, 3 from the
fifth, drawing and flipping each step with no wait, so its pace was the
machine's drawing speed. The port draws a frame in a 60th of a second and
waits after each step until a timer tick (55 ms, `combatScrollTicks`) has
passed since it began: about the pace of the DOS machines.

## Ground battle

`Colony_Combat_Screen_` (0xb7491, COLCBT.C; it first sets
`_at_least_one_tactical_combat`, 0xb749e) replays an invasion from the
seed `Invade_` started from: troops from COLGCBT.LBX in the players'
colours (`Load_Combat_Anims_`; states 1..5 take entries 1, 4, 2, 0, 3 of
each five; the defender's armour uses its second set when it knows
application 0x18), up to ten a side on the field (`Spatter_Down_Starting_
Troops_`, `Reinforce_Units_` from the edges), each frame `Draw_Troops_`
(`Do_Unit_Turn_`: fall when the class lost a unit, flinch when it was hit,
aim at a dying or hit enemy, walk, `Pick_Next_Action_`, `Do_Delta_Y_`),
`Force_Firing_Pairs_`, `Draw_Unit_Beam_` with `Draw_CGCBT_Beam_` in the
colours of the best ground weapon (bonus +9), the summary boxes, the
buildings (`Make_Bldg_Array_For_Colony_` small; `Calc_Top_Line_` reads
the last colony screen's array first and clears its first cell). The
drawing restores the seed each frame; `Ground_Combat_Round_` comes every
sixth frame after the first, all at once after a key (fast); the next key
leaves (the first when it is over). Both callers restore the seed after
the screen; it also sets `_current_screen = _return_screen`.
`Show_Off_Captured_Tech_` (a trooper in the science room, BILLTEXT 3)
follows a capture.

## Cinematics

`Play_Planetary_Destruction_Flic_`: the mini main screen with MAINPUPS
0x53 as the auto function, then `Play_Cinematic_` (no fades) plays
PLNTDFIN.LBX — a Smacker file (internal/smk decodes SMK2/SMK4 video and
palettes) — centred, colours 10..245 from its palettes, until the last but
one frame, a key or a button.

## Input (Do_Combat_Turn_ with the screen)

Per pass (`combatScreenPass`): the computer's ship (`Do_Auto_Ship_Turn_`,
which centres the view on its ship, reads `Get_Input` three times for the
Auto button and redraws after each mount) or one `Get_Input` for the
human's (`combatHumanPass`): missile fields fire at a missile, the option
switches, super fast, the missile warning, Auto (the fields replaced by
one full-screen field), weapons/specials, wait (`Next_Ship_`), the weapon
rows (on → off → off-hold), the options panel (`Move_Options_Background_`
slides it), scan, flee (refusals E text 0x3a/0x3b/0x4f), board (0x3c),
the radar (centre there), centre, a right click on a square (turn to
face), end of turn, self destruct (planet: K text 0x38; else the
confirmation K 0x39, then `Destroy_Ship_` with the drive class set to 1
for the call and the ship's turn over), then a click on a square (`combatGridClick`: the
square from the grid field's half squares, shifted by one for big ships
when the cursor is a move arrow; a legal square moves (`Move_Ship_`), an
own ship becomes current, an enemy is fired at, a cloaked one says 0x50,
one in stasis asks K 0x3d with palette 0x90..0xaf black and is released
with `Remove_Stasis_Beam_`)
and Done. Then the frame is redrawn, fully when the moves left changed.

`Assign_Mouse_Images_` (0x46f42) sets each square's cursor (move arrows
4..11 by facing, 0xc target, 1 own ship, 2 nothing) and for big ships moves
the arrow regions by 10 or 20 px so the pointer sits at the ship's centre;
this decides which square a click means.

## After the battle: the human attacker and the colony

`Do_Attacker_Beat_Colony_Stuff_`'s human branch (0xe89da, port
`humanBeatColony` in turn_combatcolony.go): `Bombardment_` into a scratch
record for the estimate (hits + biological hits), the converter check,
`Get_Invasion_Info_`, then `Colony_Selection_Popup_` (0xca6a6) with the
seed; then by the choice: −1 leave; 1 the stellar converter
(`Destroy_Colony_` 2, the planet becomes asteroids, its special and the
star's go); 2 bomb (seed saved, `Bombardment_` into the result,
`Get_Invasion_Info_` again, `Colony_Bombing_Popup_`, relations; an emptied
colony is destroyed; without marines, converter or mind control that is
the end, else the bombing popup's choice is taken next); 3 invade
(`Invade_`; `Colony_Combat_Screen_` replays it between a saved and
restored seed: not ported, no net draws); 4 mind control
(`Change_Colony_Ownership_`). The biological weapon penalty last.

Both popups are `Colony_Combat_Popup_` (0xcbad0, port combat_colony.go):
palette 0, MAINPUPS.LBX pictures 0x35..0x52 (buttons), COLONY2.LBX
0x2e..0x30 (icons), explosions MAINPUPS 0x3d/0x49, the planet by climate
and size (E string 0xdf names the LBX), `Do_Popup_Setup_` at the star, and
`Set_Random_Seed_(seed)`: the bombing popup thus takes back the draws of
the `Bombardment_` just before it and replays them hit by hit
(`Apply_Next_Bomb_Hit_`, one hit every fifth frame, all at once after ALL,
each hit with up to five explosions at random spots — real draws). Fields
(`Add_Colony_Combat_Fields_`, bottom up from CLOSE): resume (b), halt (h),
all (a) while bombing; bombard (b), invade (i), mind control (m), convert
(d); right clicks give help boxes (`Evaluate_Colony_Combat_Input_`; the
resume button's help is unreachable, its test repeats the halt button's
id). The left column: buildings, the best shield, colonists, tanks and
marines; the right: the attacker's marines and the bomb hits.

`build/turns/tc11` (inputs/tc11.GAM: `saveedit tc6.GAM tc11.GAM
'colony+68*0x169+0x14c:u8=0'` and, for ships 36 51 67 73 105 132 139 140,
`'ship+N*0x81+0x65:i16=-1' 'ship+N*0x81+0x64:u8=5'`: Jordanus without its
building 22 and player 4's ships): TURN, F11 on the
colony (360,195), Y, F12 (the selection popup), B, F12, A, F12, Esc.
`TestBattleTc11` matches the three popups (inside the popup's rectangle),
every net draw (the port's draws pass through the same `netDraws` as the
trace's, so a seed taken back drops out on both sides) and the step save.

## Not ported yet

Network play; `Play_Cinematic_`'s fading mode (other cinematics); the
sounds. Fighters jitter by the clock in the original.
