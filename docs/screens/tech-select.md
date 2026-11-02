# The science room and the research choice (SCIENCE.C, TECH.C)

What a research breakthrough (`Has_Research_Breakthrough_`) or the first
research field (`Set_Initial_Tech_`) shows: `Tech_Select_` lets the scientist
show the finished applications in the science room, then offers the next
field in eight group boxes. The spy's show-off of a stolen technology
(`Has_Report_` → `Show_Off_Stolen_Tech_`) uses the same room. Addresses are
ELF (Watcom + 0x10000). Go: `internal/game/science.go`, `techselect.go`;
engine pieces in `internal/engine/multianim.go`, `internal/gfx/bitmap.go`.

## Tech_Select_ (0x10db69)

`Save_Palette_`, a 0x7c830-byte file animation buffer,
`Show_Off_Researched_Tech_(PLAYER_NUM)`, `_Tech_Select_(0)`, `Fast_Fade_Out_`,
`Restore_Palette_`, `Turn_Off_Palettes_` (`Darken_Palette_(100)`,
`Clear_Palette_Changes(0,255)`), `Clear_Palette_Changes(0,255)`,
`Deactivate_Help_List_`, `Reset_Mouse_Picture_` (arrow list, fields drawn),
`Reset_File_Animation_Buffer_`, `Play_Background_Music_`.
`Has_Research_Breakthrough_` clears player+0x30 after it and returns 2.

## Show_Off_Researched_Tech_(p) (0x10d1fc)

The applications (4 × i16 at `_technology_fields` 0x17d904 + 0x17·field +
0xa) of p's field +0x321 that are > 0 and known (+0x117+app == 3). None:
`Clear_Fields`, `Fast_Fade_Out_`. Else the text billtext.lbx 1 through
`Decode_Text_Field_` (code 0x89 = `Technology_Fields_Name_`), the scientist
`SR_R%x_SC.LBX` (race +0x25) and `Science_Room_(anim, n, list, text)`.

`Show_Off_Stolen_Tech_(tech, victim)` (0x10d041): billtext 2 decoded with
DX = PLAYER_NUM, CX = tech, param 6 = victim; the spy `SR_R%x_SP.LBX` of
PLAYER_NUM's race (`Cache_Player_Spy_Anim_` copies it from the CD drive
when missing); one item; then `Reset_Mouse_Picture_`.

## Science_Room_(anim, n, list, text) (0x10d31c)

Registers: EAX anim, DX n, EBX the u16 list, ECX the text. Item k's
animation is `TANM_%03hi.LBX` of list[k] & 0x1ff.

1. Save the auto function, `Deactivate_Help_List`, `Clip_Off`, `Clear_Fields`.
   SCIENCE.LBX 0 (the cursor and palette) fresh; two 0xaf×0xc3 blank bitmaps
   (B1 [ebp+0x62] the text, B2 [ebp+0x5e] scratch) and an unused 600×0x3c one.
   `Flush_Buffer_Space(_global_cache_seg)`: SCIENCE.LBX 1 (the door) and 2
   (the room) are `Buffer_Reload`ed from now on, so their frame counters
   carry over. Without animations (settings+6) the door is set to frame 5.
2. Outline colour (0, 0xf0), style 4 outline in `_science_normal_color`
   (0x184144); no cursor; `Fast_Fade_Out_`; `Open_Multi_File_Animation_(anim,
   0, slot 1)`; a hidden field over the screen (hot key "") and the hot key
   Esc; page off; auto function `Draw_Science_Room_` (empty, delay 2).
3. Cursor: `Clear_Mouse_Picture_` (image 0x13 cleared, `_mouse_list_variable`
   = image 0x14), image 0 cleared, SCIENCE 0 drawn into a bitmap and into
   image 0, arrow list. billtext 4 is the label for items (bit 0x4000).
4. `Set_Palette_Changes`, `Draw_Palette`(SCIENCE 0), `Turn_Off_Palettes_`,
   stencil refresh, black fill, the scientist's first frame
   (`Draw_Multi_File_Animation_(1)`), off → back, `Turn_Off_Palettes_`, mouse
   buffer cleared. Fade in: 100 → 0 by 20, each step darkened, the room
   (SCIENCE 2) drawn, flipped, `Release_Time(1)`. `Set_Fade_Percent_(0)`.
5. With n > 0, the loop: `Mark_Time`, `Get_Input`. On input or the first pass:
   Esc or nothing left → close; else `Update_Science_Anims_(dl 0, bl 1, cl 1)`,
   flip, item k's animation into slot 0, its palette, `Refresh_Palette_`,
   B2 = copy of B1, help record list[k] & 0x1ff (`Far_Reload_Data` of the
   help file, 0x57b bytes), B1 blank, its title in style 3 high colours
   (0x18413c) at (2, 1) width 0xab, h = title height + 1, style 2, LF 1; bit
   0x4000: k++, the label at (2, h), then for each set bit i (from 1) of
   list[k] the name at `_weapon_modifications` 0x17fd0b + 0xf·i plus "\r",
   at (2, h + 0xf); else the record text (+0x67) at (2, h). k++, n--; jumble
   = 100 with animations, else 0; range 6; `Update_Science_Anims_(0, 1, 1)`
   until the button is up; clear the mouse buffer. Other passes:
   `Update_Science_Anims_(1, 1, 1)`.
   Then: once the door has reached frame 5 (`ready`), B1 is drawn at
   (0xdd, 0x2c) — while jumble > 0 as `Jumble_Bitmap_(B2, B1, jumble, range)`
   then B2, jumble −= 20, range down to 1. Outline (0, 0xf2), style 4 outline,
   the text at (0x91, 0x1a4) width 0x17c; `Release_Time(1)`, flip.
6. Close: from the door's frame − 1 down to 0 (animations only): set the
   door's frame, `Update_Science_Anims_(0, 1, 1)`, below frame 5
   `Scale_Bitmap(B2, B1, _bm_scale_x[f], _bm_scale_y[f])` (0x18414c,
   0x184158: 6, 21, 37, 60, 85 / 6, 20, 37, 58, 85) drawn at `_bm_scaled_x1/y1`
   (0x184164, 0x18416e), else B1 at (0xdd, 0x2c); `Release_Time(1)`, flip.
   Then `Update_Science_Anims_(0, 0, 0)`, flip; `Clear_Fields`, mouse buffer,
   the auto function back, `Set_Fade_Percent_(0)`.

`Update_Science_Anims_(dl item, bl room, cl door)` (0x10da98): with
animations `Draw_Multi_File_Animation_Stencil_(1)` (the scientist into both
pages), else `Set_File_Animation_Frame_(0x12)` — which sets slot 0's frame,
the item's; room → SCIENCE 2 at (0, 0); item → `Draw_Multi_File_Animation_(0)`;
door → SCIENCE 1.

**RNG.** Only `Jumble_Bitmap_` (0x12eee7) draws: it saves the seed,
`Jumble_Bitmap_Pixels_` (0x147e9f) takes `Random_(1000)`, and the seed is put
back. Five calls per item (jumble 100, 80, …, 20), all drawing the same
value. How many happen depends on when the player clicks: a click or Esc
before the text has settled cuts them short.

`Jumble_Bitmap_Pixels_(src, range, percent, dst, w, h)`: count w·h − 2·range
− 2 from offset range + 1 in both; table index (Random − 1) & 0xff into the
256 bytes at 0x189024; where table[i] < percent the pixel swaps with the one
(table[i] & range) + 1 away, left or right as the static flag 0x189010
(toggled by each swap) says.

`Scale_Bitmap` → `Reduce_Bitmap_` (0x147a12) for scales ≤ 100: steps
0x6400 / scale (16-bit) in 8.8 fixed point; each row advances the source by
`mul ch` = (width & 0xff) · whole rows, the upper half of the width kept.

## _Tech_Select_(changing) (0x10dc12)

`Clear_Fields`, stencil, auto function saved; the 8 hyper levels (+0x21c)
saved to `_saved_hyper_advanced_tech` (0x1ad59c) and each incremented (the
costs and names show the next level); `Disable_Cancel`. Mode 0
(Tech_Select_): screen x 0xa1, cursor colour 0xff, `_tech_select_colors`
(0x184247), TECHSEL.LBX 0 (panel), 1..8 (group buttons), 0xd (cursor).
Mode 1 (Tech_Change_): x 0x50, colour 0xef, `_tech_change_colors`, entries
0xe, 0xf..0x16, 0x1b and a done button at (x+0xbd, y+0x1c4) with hot key
Esc. The 5 × 8 shades (`_tech_color` 0x1ad5a4): normal, selected, being
researched, and two for the list.

Boxes: `Init_Entry_Positions_` (positions `_tech_select_pos` 0x1841b5 /
`_tech_change_pos`, group `_entry_to_group`, rows at `_app_name_y` + 0x15)
and `Init_Entry_Data_(spent)`: the first field of the group's chain
(`_first_field_in_group` 0x17cf18, then field+8) that is researchable (2)
and not the field +0x321; its obtainable (1) applications as rows, each a
hidden field (x, y+`_tech_field_y1`[row]+0x15)..(x+0xda, y+y2+0x15); none →
billtext 0x3e; a hyper field (≥ 0x4b) one row, pseudo application field +
0x81 ("Computers I": base name, " ", roman numeral of the (incremented)
level). Cost `Player_Research_Cost_` − spent (≥ 0) as "%i RP" ("%i FP"
German, "%i PR" Italian). Mode 1 zeroes +0x321 around it (spent = +0x1eb);
mode 0 zeroes +0x321 for good if it is ≥ 0x4b. Then a hidden field per box
(x−2, y+0x12)..(x+0xd7, y+0x63) (`_g_first/_last_tech_field`), the radio
buttons (`Setup_Entry_Buttons_`, `Add_Radio_Button_Field_` type 1 over
entry+0x4b), a full-screen hidden field; mode 0 opens `SR_R%x_SC.LBX`
(`Open_File_Animation_`) and SCIENCE.LBX 0.

Redraw: mode 0 black, the room frame 0 (0x11 for race 7), black panel area,
both palettes refreshed; input delay 5; the help list; `_FULL_DRAW_SCREEN_FLAG
= 0`; auto `Draw_Tech_Select_` (delay 4); panel; `Display_Entry_Text_` of
each box; off → back; `Draw_Tech_Select_`; mode 0 the TECHSEL 0xd cursor
(`Bill_Picture_To_Mouse_`); `Fast_Fade_In_`; `Reset_Cycle_Palette_Color_`.

Loop on `Get_Input` (redraw when the full-draw flag is set):

- negative (right button): outside the radio buttons,
  `Set_Selected_Entry_(−f)` and with a selected application
  `Draw_Application_Description_` (help text and cost in `Text_Box_`), then
  redraw; on a radio button it counts as a left click;
- a row or box (done < f < first radio): the selected entry blinks (three
  flips, drawn on the second), +0x321 = its field, +0x322 = its selected
  application, leave;
- a radio button: frame 1 drawn, off → back, `_Tech_List_` of that group,
  redraw;
- nothing: `Draw_Tech_Select_`, `Release_Time(2)`, flip.

**The choice follows the hover, not the click.** `Draw_Tech_Select_`
(0x10e4b2) selects the row under the pointer (`Scan_Input`; over a box the
box's last row) through `Set_Selected_Entry_`; a pointer over anything else
clears the selection. The click only says "take the selection".

Exit (also straight from the setup when no box is filled): `Pop_Block_`,
`Put_Auto_Function_(saved, edx = 0)`, `Clear_Fields`, the eight hyper
levels restored, `Deactivate_Help_List_`. The restore loop's index is that
edx (Watcom callees keep it), so all eight come back in both modes: the
original's save after a change has the levels unchanged.

The help lists (`Set_Help_List_` over 10-byte records: id, x1, y1, x2, y2)
are set on every redraw: `Technology_Select_Help_` (0x183718, 3 records) or
`Technology_Change_Help_` (0x183736, 3). `_Tech_List_` sets one of
`Tech_Select_List_Right/Left_Help_` (0x183754 / 0x183768, 2 each) or
`Tech_Change_List_Right/Left_Help_` (0x18377c / 0x183790) by the box's list
x: right when list x − screen x > 6 (both as unsigned words).

`Display_Entry_Text_` (0x10ed00): style 4 field name `Squeeze_Print_` at
(x, y+0x15) width 0xda, style 3 cost right at (x+0xd4, y+2), rows at (x+10,
row y) width 0xd0; the box of the field being researched in the third
shades, its row (stricmp with the application name, or every row for
creative races and fields 0x16, 0x17, 0x1c, 0x1d, 0x37, 0x39) too.
`Display_Selected_Entry_Text_` (0x10ee6f): second shades, the selected row,
`Box(x−2, y+0x12, 0xda, 0x52, 1, cursor colour)`. `Draw_Little_Arrow_`
(0x10e563): four lines from (x+1, row0 y−3). `Cycle_Palette_Color(colour, 8,
0x20, 0, 0x28, 0x3f, 8, 8)` cycles the cursor colour in the DAC and
current_palette (not marked changed); `cycle_direction_flag` 0x1845ec is
global and starts from .data.

`Squeeze_Print_` (0x102f10, `_Squeeze_Print_` 0x102dab): too wide → space
width down to `_space_limits[style]`, glyph spacing down to 1, smaller
styles; `_Print_String_Bill_` prints left-aligned only for width 10..640
and a corner on screen.

## Tech_Change_ (0x10dbce), screen 0x24

Main_Screen_'s research panel case (field 0x19c168, hot key "", not while
`_skip_fields`): `_dont_draw_main_screen_to_back` = 1, `Clear_Fields`,
`Set_Main_Screen_Fields_To_Null_`, page off, `Draw_Mini_Main_Screen_`,
`Copy_Off_To_Back`, flip, stencil refresh; with the fleet box (box 2) open
`_force_main_screen_fade` = 1 (the main screen fades back in); screen 0x24,
return screen 0, `_clear_fleet_fields` = 0, leave. So the map is on every
page when the panel is drawn over it.

Tech_Change_: `Allocate_First_Block_`, page off, `Draw_Visible_Fields_`,
`Copy_Off_To_Back`, `_Tech_Select_(1)`, then `_current_screen` = the return
screen (0x199a10); Screen_Control_ sets `_previous_screen` 0x24, and
Main_Screen_ redraws the mini main screen into both pages for it.

Mode 1 differs from mode 0 only where the spec above says: panel at x 0x50,
cursor colour 0xef, TECHSEL 0xe..0x16 and 0x1b (the CANCEL button at (0x10d,
0x1c4), hot key Esc, field 1), the list frames 0x17..0x1a; no room, no
scientist palette, no cursor picture, no music, no fade out. The field
being researched is offered again (+0x321 is zeroed around
`Init_Entry_Data_`), drawn in the third shades with its application, and
every cost is the field's less the research points spent (+0x1eb). Picking
sets +0x321/+0x322 only: **the points are kept**, so they count towards the
new field (seen in the original: 4080 RP stay, Computers 25000 → 20920 RP).
Esc (the done button) changes nothing.

## _Tech_List_(entry, changing) (0x10e6c7)

A browser: nothing in it changes the choice. billtext 0x3f (" LIST") after
the group's name (billtext 0x40 + group); `Save_Field_Stats_`; TECHSEL 9..12
(frame, the pressed return button, up, down; 0x17..0x1a for Tech_Change_);
the screen behind captured (0x54,0x28)..(0x207,0x1b7); `_g_tech_list_header`
(0x184297) at x = the box's list x + 0x10, arrow colour = the cursor colour,
printer `Print_Tech_Indent_Item_` ("^" + the application's name, squeezed
to 0xd5). `Get_Group_List_`: the group's chain, each field with status 4
when researched now, 3 (skipped) for a hyper field past level 20, else its
own status (0 becomes 1 when an application is known; still 0 → skipped);
its applications with status ≠ 0 (obtainable ones of a researchable field
shown as researchable, all as 4 for the current field). Buttons up (x+0xf7,
0x4d) and down (x+0xf8, 0x18e); auto `Draw_Tech_List_` (delay 4: the hovered
item selected and drawn in the selected colours, the cursor colour cycled).
Each page: the capture, the frame at (x, 0x1f), the title centred at
(x+0x86, 0x2f) in style 4, the page, off → back, page buttons, flip,
`Clear_Fields_Above`, item fields, an Esc field and an empty-key field over
the screen. Right click on an item: `Display_Selected_List_Text_` (with the
arrow) and `Draw_Application_Description_`; Esc: the pressed button at
(x+0xbb, 0x1b2), flip, fields restored.

## Checks against the original

bt1 (`build/turns/inputs/bt1.GAM`: t1 with player 0's research points at
100000, so field 42 completes): traced with `-listkey -capture`
(build/turns/bt1.keys: Esc in the room, key 1 on the choice; the trace pins
the hover on field 1). `TestTurnBt1Breakthrough`
plays it: all 1525 draws (five jumbles) as traced, SAVE10 = the original's
(field 58, application 15). Frames: the room identical but the scientist's,
the item's and the glass's animation phases; the choice identical but the
cycled box colour. bt2 (keys build/turns/bt2.keys; -listkey gives the group
buttons key 2) opens the Construction list first: `TestTurnBt2TechList`
passes the same way and the list frame is pixel-identical.

Tech_Change_: `build/oracle/techchange` (SAVE1 as SAVE10; field 42,
4080 RP), captured with `-listkey -reskey -techhover N`: `-reskey` gives the
research panel hot key 9 (operand 0x820cc → "9"), `-techhover N` pins
`Draw_Tech_Select_`'s hover on field N (rows start at 2 in mode 1: CANCEL is
field 1). runA (N = 3, Heavy Fighter Bays): 9, f12, 2 (the Construction
list), f12, Esc, f12, 1, f12, F10; runC (N = 10, Computers I, hyper field
81): 9, f12, 1, f12, 9, f12, Esc, f12, F10. `TestTechChange` replays both
(port options ListKey, ResearchKey, TechHover): all eight frames identical
but the cycled cursor box, a black hole and the map's animations; both
SAVE10s byte-identical (field 42 app 83; field 81 app 210).

## Not ported

The right-click description and the help in mode 1 are mouse-only and not
compared with the original; the fleet box path of the research case
(forced fade) is untested. `Enlarge_Bitmap_`, glassed file animations, the weapon modification names
of items (bit 0x4000; read from the pointer table, unverified).
