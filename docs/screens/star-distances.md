# Star distances (F9, screen 0x1c)

F9 on the galaxy screen: `Check_Function_Keys_` (0x825a8, jump table
0x82580, branch 1) sets `_current_screen = 0x1c`, `_return_screen = 0` and
leaves Main_Screen_; Screen_Control_ runs `Distance_Between_Stars_Screen_`.
A click picks a star; hovering another one prints the parsecs between them
next to it and draws a dotted line. Port: `internal/game/stardistances.go`.

## Distance_Between_Stars_Screen_ (0x8ab6d)

1. `Clear_Fields_`, `Set_Main_Screen_Fields_To_Null_` (also every
   `_star_button[i] = -1000`).
2. `Set_Window_(0x16, 0x16, 0x20f, 0x1a5)`, `Clip_On_`, `Add_Star_Fields_`,
   `Clip_Off_`, `Reset_Window_`.
3. `esc = Add_Hot_Key_("\x1b")`, `map = Add_Hidden_Field_(0x16, 0x16, 0x1fa,
   0x190, "", 0x29)` (ECX keeps 0x190 from the hot key call),
   `whole = Add_Hidden_Field_(0, 0, 0x27f, 0x1df, "", 0)`.
4. `Assign_Auto_Function_(Draw_Main_Screen_For_Star_Distances_, 2)`;
   `_screen_data[3]` (the picked star) and `[4]` (the hovered one) = -1.
5. `Set_Page_Off_`, `Draw_Main_Screen_Filled_(0)`, `Set_Palette_Changes_(0,
   0xff)`, `Update_Glass_Remap_Colors_`, `_Darken_Fill_` over the four
   borders of the map ((0,0)-(0x15,0x1df), (0,0)-(0x27f,0x15),
   (0x20f,0)-(0x27f,0x1df), (0x15,0x1a5)-(0x20f,0x1df)), `Copy_Off_To_Back_`.
   The first redraw covers it again: the darkened border never shows.
6. `_global_var = -1`; then until done: `Mark_Time_`, `r = Get_Input_`,
   `_scanned_field = Scan_Input_`, `Set_Mouse_List_` with MAINSCR.C's
   `_mouse_list_from` (0x1821cc, 5 entries) while no star is picked, else
   `_mouse_list_to` (0x182208); for every star `r == _star_button[i]` picks
   it, `r == -_star_button[i]` (right click) ends; `r` = whole, -whole,
   -map, -1009 (F9 again) or esc ends. Not done: `Set_Page_Off_`, the draw
   routine, `_TOGGLE_PAGES_`. `Release_Time_(1)`.
7. `Set_Mouse_List_(arrow, 1)`, `_current_screen = 0` and the shared
   epilogue at 0x8ab3c (no Clear_Fields_).

A left click on the map outside the stars is the map field: nothing happens.

## Add_Star_Fields_ (0x822af)

For each star: `Get_Star_Draw_Coords_`, the size `dim` = the local table
0x81bfc `{0x27, 0x21, 0x21, 0x18}` by zoom level for a black hole, else
`_star_fields_dim[size + zoom]`; `_star_button[i] =
Add_Clipped_Circle_Field_(x, y, dim/2 - 2, "", 0)` (-10000 when outside the
window). It ends in Draw_Black_Holes_' epilogue.

## Circle fields (type 0xe)

`Add_Clipped_Circle_Field_` (0x1146fd): the box `x±r, y±r` cut to the
window (-10000 when disjoint), var1/var2 the centre, var3 `r*r`.
`Scan_Field` (0x113fb9) and the buffered click of `Interpret_Mouse_Input_`
(0x11dc58) first test the box with the pointer offset added, then for a
circle `dx*dx + dy*dy <= r*r` with the raw pointer (no offset).
`Scan_Input_` returns the field as for any other type, `Draw_Field_` and
`Push_Field_Down_` draw nothing. Port: `engine.AddClippedCircleField`,
`fieldAt`.

## Draw_Main_Screen_For_Star_Distances_ (0x8af4e)

1. Font colours from the code segment: 0x8a83b (parsecs) and 0x8a843 (the
   black hole message); `sel = _screen_data[3]`.
2. `Draw_Mini_Main_Screen_`, `Draw_Main_Main_Screen_` (a second time),
   `Set_Window_` to the map, `Clip_On_`, `_global_var = (int8)_screen_data[4]`.
3. `sel > -1`: `Highlight_Star_(sel)` (0x86076: not for a black hole,
   `Print_Highlighted_Star_Name_` and the `_selected_star_color_offset`
   pulse). Then for i over the stars while nothing is found: if
   `_star_button[i] == _scanned_field` and i ≠ sel, `_screen_data[4] = i`
   and found. Inside the same loop, if `_screen_data[4] > -1`, ≠ sel and
   equal to `_global_var` (last pass's value): both stars' coordinates,
   `d = Parsecs_Between_Stars_`, `Set_Outline_Color_(0, 0)`, and
   - star sel's black hole bits (+0x1f) have bit `_screen_data[4]`:
     `Set_Font_Style_Outline_(2, 0x8a843)`, H 0xea, `Change_Font_Style_(1)`;
   - else `Set_Font_Style_Outline_(2, 0x8a83b)`, `sprintf(H 0xeb, d)`;
   - printed at (x2 + 10, y2), or `Print_Right_(0x20a, y2)` when it would
     pass 0x20a; `Draw_Directional_Multi_Colored_Line_(x1, y1, x2, y2,
     0x182135, _multi_colored_line_start)`.
   Quirk: the test does not depend on i, so the label and the line are drawn
   once per star up to the hovered one (all stars when the pointer left the
   stars), and nothing on the first pass over a new star.
4. `_multi_colored_line_start = (start + 1) % 8`, `Clip_Off_`, `Reset_Window_`.
