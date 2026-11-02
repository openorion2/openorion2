# Help box (right click on a help entry)

Port: `internal/game/helpbox.go`; the engine side is `checkHelpList` in
`internal/engine/fields.go` with the callback `Engine.SetHelpFunc`.

## Check_Help_List (0x119732)

Called from `Interpret_Mouse_Input` when the right button is held or a
right press was buffered and `help_list_active` is set. The first entry of
the help list (`{id, x1, y1, x2, y2}`, id ≠ −1) containing the pointer →
`Draw_Help_Entry_(id)`, return 0 (the input path then clears the mouse
buffer and returns 0). None → 1, and the right click goes on as a cancel or
a −field.

## HELP.LBX

`Get_Help_Lbx_Name_` (0xa6547) by the language byte (settings+0xd2): 1
GER_HELP.LBX, 2 FRE_HELP.LBX, 3 SPA_HELP.LBX, 4 ITA_HELP.LBX, else
HELP.LBX. Entry 0 is a record table of 707 records of 0x57b bytes, read with
`Far_Reload_Next_Data(name, 0, …, id, 1, 0x57b)`:

| offset | size | field |
|---|---|---|
| 0x00 | 0x50 | title; first byte 0x14 means no title |
| 0x50 | 0x0e | LBX file of a picture; empty means none |
| 0x5e | s16 | entry in that file |
| 0x60 | u8 | frame |
| 0x63 | s32 | next record; −1 means id+1, 0 ends the chain |
| 0x67 | to the end | text |

## Draw_Help_Entry_(id) (0xa59a1)

1. `Text_Box_Startup_(0x11b34)` (see `user-box.md`), base colours 0: c4 =
   `Font_Colors2_(4, a, b)`, c2 = `Font_Colors2_(2, a, b)`;
   `Set_Remap_Font_Style(4, c4)`, `Set_Remap_Font_Style(2, c2)`.
2. Read the chain: record id, then its next (−1 → id+1), until the next is
   0 or nine records are read. Each text is measured once at width 0x153
   (result unused).
3. Close button `textbox.lbx` 3. Layout from yy = 0xb, per record i:
   title width 0x153 and title y = yy; with a picture, it is loaded fresh,
   set to its frame, drawn into a bitmap (`Draw_Picture_To_Bitmap`) and
   `Get_Bitmap_Actual_Size` gives x1, y1, w, h: picture x = 0x162 − (w +
   x1), picture y = title y − y1, title width = 0x153 − (w + 6), pad = h.
   `Set_Font_Style(4, c4)`; title height = 0 for a 0x14 title, else its
   formatted height at the title width; raised to pad when pad > 0; text y
   = yy + that + 4, at least 0x2a for the first record. `Set_Font_Style(2,
   c2)`; th = formatted text height at 0x153; yy = text y + th + 8.
4. `Box_Centered_On_XY_(&x, &y, (u16)(last text y + last th − 0x22))`.
5. Per record: the title (unless 0x14) in remapped style 4 at (x+0x14,
   y + title y), title width; the text in remapped style 2 at (x+0x14, y +
   text y), width 0x153; the picture with `Draw` at (x + picture x, y +
   picture y).
6. `Copy_Off_To_Back`, hidden full-screen field with hot key Esc,
   `Text_Box_Get_Input_(0)`, close button frame 1 at (x+0x9e, _bottom_y −
   0x1b), `Text_Box_Cleanup_`, `_FULL_DRAW_SCREEN_FLAG = 1`.

Titles and texts go through the formatted paragraph printer (FMTPARA.C,
`docs/engine/paragraphs.md`).
