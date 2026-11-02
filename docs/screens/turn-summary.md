# The turn summary (TURNSUM.C, screen 0x28) and the paged lists (LIST.C)

`Has_Msg_` (report code 4, `reports-and-begin-of-turn.md`) sends the human
to screen 0x28 with return screen 0x27 when the summary option (settings+0)
is on and a message counts. Addresses are ELF. Go:
`internal/game/turnsum.go`, `list.go`, `msgtext.go`.

## Turn_Summary_Popup_ (0x10fb02)

```
_current_screen = 0
n = N_Msgs_For_Player_(PLAYER_NUM); n < 1 → end
return screen 0x27 → _g_turn_message_ix = 0
else (back from the colony screen) → Fast_Fade_Out_, Load_Palette(0,0,255), Turn_Off_Palettes_
clear mouse buffer, Deactivate_Help_List, save the auto function, Clear_Fields, stencil
turnsum.lbx 0 (frame), 1 (up), 2 (down), 3 (close), fresh
redraw: _FULL_DRAW_SCREEN_FLAG = 0; Clear_Fields; auto Draw_Turn_Summary_Popup_ (2)
    up = button (0x1b9, 0x5f), down = button (0x1b9, 0x161), no hot keys
    entries in a 15800-byte local; Init_Turn_Summary_List_(&_g_turn_sum_header, n, style 4, LF 10)
    _g_selected_msg_p = Select_And_Page_By_Head_ID_(header, _g_turn_message_ix)
    Draw_Mini_Main_Screen_; Turn_Summary_Popup_Help_
page: Clear_Fields_Above(down); first = field counter; last = Add_Fields_To_List_Page_
    close = button (0xf2, 0x176) hot key Esc; hidden (0x54,0x33)..(0x1d0,0x189) Esc;
    full = hidden over the screen, Esc
    frame at (0x54, 0x33); Display_List_Page_; Set_List_Up_Down_Field_Drawing_(up, down)
    off → back; Clear_Palette_Changes(0,255); flip; Fast_Fade_In_
loop: f = Get_Input; full-draw flag → redraw
    f an item field: Goto_Msg_Colony_(msgs + 0x12·_g_selected_msg_p→id) → screen 1,
        return 0x28, _g_turn_message_ix = id, f = full
    else f < 1, ≥ full or close: f == 0 → Draw_Turn_Summary_Popup_, flip, Release_Time(2)
    else f == down and Next_List_Page_, or up and Previous_List_Page_ → page
    f ≥ full or close → auto function back, Clear_Fields, end
end: _current_screen still 0 → 0x27
```

Esc reaches the close button first (all three Esc hot keys; Esc is matched
as a hot key before it would cancel). A click in the list area (the second
hidden field) does nothing; outside it closes.

`Draw_Turn_Summary_Popup_` (0x10faac): with colour set 0 of the header
switched to `_turn_sum_high_color` (0x1842e0), the entry under the pointer
(`Scan_Input` within first..last) becomes the selection
(`Set_Selected_List_Entry_`), `_g_selected_msg_p` = the page's selected
entry, `Display_List_Text_` of it; colour set 0 back to normal (0x1842d8).

`Init_Turn_Summary_List_` (0x108ec3): count = n; from
`First_Msg_For_Player_`, per message: text "", state 0, id = item 0's id =
the message index; style 4 in `_info_normal_color` (0x183b1e), LF 10; items
= `Get_Formatted_Paragraph_Max_Height_(width, sprintf_msg_)` / LF, item
states 0, ids after the first 0xff; then `Init_List_Data_`.

`Print_Turn_Summary_Popup_Message_` (0x10fa35), the item printer: ids ≥ 0xfb
print nothing; LF 10, "^" at (x, y), the message as a formatted paragraph at
(x+0xd, y) width 0x140, type 0.

`Goto_Msg_Colony_` (0xef6f0): type-0 messages whose planet (+2) holds a
colony of PLAYER_NUM's: screen 1, return 9, `_screen_data` = star, orbit.

## The paged lists (LIST.C, 0x10a0a7..)

Header (`_g_turn_sum_header` 0x184324, `_g_tech_list_header` 0x184297): +0 x,
+2 y, +4 width, +6 bottom, +8 item y table (item n = entry height), +0xc /
+0x10 the item fields' top and bottom rows, +0x14 spacing, +0x15 count,
+0x16 entries, +0x1a the page's first entry, +0x1e pages, +0x1f current
page, +0x20 text style, +0x21 item style, +0x22 item printer, +0x26
indent, +0x27 arrow colour, +0x28 selected colours, +0x2c colour set
pointers by state. The summary's: (0x5c, 0x67), width 0x140, bottom
0x175, rows 0, 10, …, 140, fields to 9, 19, …, spacing 8, styles 4/4,
indent 0xd.

Entry (0x9e bytes): +0 x, +2 y, +4 page start, +5 selected, +6 selected
item, +7 text, +0x47 id, +0x48 state, +0x49 items, +0x4a item x (x +
indent), +0x4c right (x + width − indent), +0x4e item ys, +0x6e item fields
(bytes), +0x7e item ids, +0x8e item states.

- `Init_List_Data_`: pages = page = 1, page pointer = first entry; each
  entry at the running y; y += table[items] + spacing; past the bottom →
  page start, pages++, the entry moves to the top. The first entry always
  starts a page.
- `Add_Fields_To_List_Page_`: a hidden field per item of the page, (x,
  y+top[k])..(right, y+bottom[k]); returns the last field.
- `Display_List_Text_`: style +0x20 in colours[state], `Squeeze_Print_`
  the text (width +4); style +0x21, per item its state's colours and the
  printer at (item x, y + item y) with the item id (`_race_report_jim_hack`
  shading not ported).
- `Set_List_Page_Pointer_`, `Next_/Previous_List_Page_`,
  `Goto_Page_With_Entry_`, `Select_And_Page_By_Head_ID_` (the entry with
  that id, else the first, selected and paged to), `Set_Selected_List_Entry_`
  (byte compare of the item fields), `Get_Selected_List_Entry_`,
  `Set_List_Up_Down_Field_Drawing_` (`Enable/Disable_Draw_Field_`: buttons
  get type −1001, radio −1, …, which nothing draws).
- The page walks read the "page start" byte of the entry after the last
  one (past the array); the port treats it as 0.

## sprintf_msg_ (0xeec98)

The 80 strings of `Load_Msg_Strings_` (0xee4a1) follow the 60 Colcalc and
10 Combfind ones in RSTRING<lang>.LBX entry 0; their order is the constant
list in `msgtext.go`. Types (record +0): 0 colony (+2 planet, +3 subtype:
scrapped building, built (building, ship/refit at −100−index, freighter
−15, spy −7, androids −6/−5/−4; then "next in queue" / "autobuild" of
`Product_Name_` +6), genocide, assimilation, starvation, settlers,
plague, boom, comet, rebellion, android without room, android killing),
1 alliance, 2 war, 4 finances, 5 ship lost (+leader fate), 6 scrapped
ship, 7 lost colony, 8 flux, 9 hyperspace beast, 10/11 spies lost, 12/13
artemis net, 14 retreat losses, 15 assassination (Italian swaps two
arguments), 16 anomaly, 17 diplomacy (5 texts), 18/19 eels, 20
artifacts, 21 April 1st, 22 dismissed leader. Type 3 and > 0x16 leave the
buffer unset.

`Product_Name_` (0xcf398): −0x25 "AI trade goods"; 1..0x30 the building;
> −100 `Option_String_` (E strings by id; designs −50..−99 of the player of
`_colony_handle`'s colony, stale off the colony screen); ships, with
status 4 as E 0x2d "<refit> <name>".

## Check against the original

sum1 = t1 with MOX.SET byte 0 set (`build/turns/inputs/sum1.SET`, keys
`build/turns/sum1.keys`): the tribute demand, then the summary (five
messages, two of them two-liners), Esc. `TestTurnSum1Summary`: all draws,
SAVE10 = the original's; the summary frame identical but black holes.

## Not ported

The colony screen the messages lead to (screen 1 goes back to 0x28),
`_race_report_jim_hack`. The help list (one entry at 0x1836f0) is set.
