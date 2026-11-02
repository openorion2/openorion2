# Fields and input (hidden fields, hot keys, buttons, grids, cursor)

The subset of the field/input system that the main menu and the galaxy main
screen use: types 0 (button), 7 (hidden field, hot key), 8 (multi hot key),
0xc (grid) and 0xd (irregular button). Source of
truth: `WLIBS\fields\fields.c` (`Clear_Fields`, `Add_Hidden_Field`,
`Add_Hot_Key`, `Get_Input`, `Interpret_Mouse_Input`,
`Interpret_Keyboard_Input`, `Scan_Input`, `Scan_Field`, `Push_Field_Down`,
the auto-function routines), `WLIBS\mouse\mouse.c` (`User_Mouse_Handler`,
`Check_Mouse_Buffer`, `Check_Mouse_Shape`, `Mouse_Button`),
`WLIBS\keybdjoy\key.c` (`New09_Handler`, `Read_Key`, `Keyboard_Status`),
`Toggle_Pages`, `Mark_Time_` / `Release_Time_`, and the game routines
`Main_Menu_Screen_` (0x816f2), `Add_Main_Menu_Fields_` (0x80ae4),
`Draw_Main_Menu_Screen_` (0x813a4), `Fade_Into_Screen_` (0x8f1c4).

Addresses are ELF addresses in `build/orion2.elf`. Data-segment statics are
listed as DWARF `2:off` and ELF `0x178000+off`.

## Field record (`struct s_fields`, 55 bytes, packed)

`sfields[250]` at 2:0x38848 (ELF 0x1b0848..0x1b3dfd, 250 × 55 bytes, followed
directly by `simtex_field_chosen`, `last_button_*`, `y_pointer`,
`fields_count`); the pointer `fields` (2:0xc480, ELF 0x184480) points at it.
`max_fields_count` = 250 (2:0xc47e, ELF 0x18447e). Field 0 is never
assigned: numbering starts at 1. (`_y_pointer >> 16` in Ghidra output is
`fields_count`, an overlapping-symbol artefact.)

| Off | Type | Name | Hidden field / hot key |
|---|---|---|---|
| 0x00 | int16 | xmin | rect, inclusive |
| 0x02 | int16 | ymin | |
| 0x04 | int16 | xmax | |
| 0x06 | int16 | ymax | |
| 0x08 | uint16 | type | 7 for both |
| 0x0a | int16 | text_style | not touched |
| 0x0c | ptr | text_color | not touched |
| 0x10 | int16 | fill_type | not touched |
| 0x12 | int16 | active_flag | not touched |
| 0x14 | ptr | highlight_color | not touched |
| 0x18–0x33 | 7 × 4-byte unions | var1..var7 | not touched |
| 0x34 | uint8 | hotkey | first char of the string, `a`–`z` upper-cased |
| 0x35 | uint16 | button_sound | hidden field: the 6th argument; hot key: **not written** (stale) |

Fields not written keep whatever the slot held before (slots are reused
after `Clear_Fields`); nothing in the hidden/hot-key paths reads them.

Other types use the same record (the Go `Field` names the union members):

| Member | Button 0 / irregular 0xd | Multi hot key 8 | Grid 0xc |
|---|---|---|---|
| rect | 0: x..x+w-1, y..y+h-1 from the picture header; 0xd: explicit | -1,-1,-1,-1 | x..x+bw·nx-1, y..y+bh·ny-1 |
| text_style / text_color | `Get_Current_Font_Style()` / `Get_Current_Font_Color()` (`current_font_style`, `current_font_color_array`) at add time | 0 | stale |
| var1 (0x18) | label string | the string | stale |
| var2 (0x1c) | 0xd: draw x | progress (short) | box width |
| var3 (0x20) | 0xd: draw y | strlen (short) | box height |
| var4 (0x24) | `min_value` = `Get_Full_Store_Flag(pic)` = header byte 0x0b & 4 | 0 | `short *` grid x |
| var5 (0x28) | stale | 0 | `short *` grid y |
| var6 (0x2c) | picture | 0 | stale |
| hotkey | `hot[0]` upper-cased | 0 | 0 |
| button_sound | argument | 0 | argument |

Note: the full-store flag is bit 2 of the header *mask* byte (0x0b), not
`gfx.Picture.FullStore` (byte 9).

## State (fields.c / mouse.c / key.c)

| Name | Addr (ELF) | Init | Meaning |
|---|---|---|---|
| fields_count | 0x1b3e0e | 1 after Clear_Fields | next free field number |
| input_delay | 0x1b3e12 | 0 | Get_Input skips this many calls |
| pointer_offset | 0x1b3e14 | — | hotspot offset of the current mouse list entry |
| down_mouse_button | 0x184490 | -1 | field currently "pushed" |
| selected_count | 0x18448e | 0 | reset to 0 when ≥ fields_count (Interpret_Keyboard_Input) |
| auto_active_flag | 0x18448c | 0 | auto function enabled |
| function | 0x1b0840 | — | auto function |
| auto_function_delay | 0x184498 | 2 | ticks for Call_Auto_Function |
| update_function | 0x184488 | 0 | main2_ sets it to `Mox_Update_` (sound + net) |
| auto_input_variable | 0x1844a6 | 0 | field being held (read by `Auto_Input`) |
| mouse_cancel_disabled | 0x1844b0 | 0 | 0: right button / Esc return -1 |
| mouse_auto_exit | 0x1844b2 | 0 | 1: held-button loop exits after one pass |
| field_box_mode, help_box_mode | 0x1844cc, 0x1844ce | 0 | debug outlines; toggled by `` ` ``/`~` only when `release_flag`==0 |
| input_field_active, active_input_field_number | 0x1844ca, 0x1844d0 | 0, -1 | text input fields (not covered) |
| help_list_active / help_list_count / help_struct_pointer | 0x1844d8 / 0x1844da / 0x1b0838 | 0 | right-click help |
| multi_hotkey_active_field | 0x1844dc | 0 | set by `Add_Multi_Hot_Key_Field`, cleared by `Clear_Fields` |
| down_x, down_y | 0x184492, 0x184494 | 1, -1 | label shift of a pressed button (`Set_Button_Down_Offsets`) |
| field_outline_color, help_field_outline_color | 0x1844b4, 0x1844b6 | 0x0e, 0xb4 | debug outlines (`Set_Field_Outline_Color`) |
| pressed_button_flag, pressed_mouse_x/y | 0x1844e2, 0x1844de/e0 | 0 | first press position (`Pressed_Mouse_X/Y`) |
| dynamic_field_refresh | 0x1844e4 | 1 | rescan fields while a button is held |
| draw_fields_flag | 0x1844e6 | 1 | `Draw_Field` / `Draw_Visible_Fields` enabled |
| last_button_number/x/y | 0x1b3e06/08/0a | — | written on a selection |
| release_flag | 0x184374 | 1 (main2_ calls `Release_Version_`) | |
| mouse_x, mouse_y | 0x1bba38, 0x1bba36 | | pointer, 0..639 / 0..479 |
| current_mouse_list, _count | 0x1b9204, 0x1b9224 | | cursor regions |
| current_mouse_list_ptr, current_mouse_shape | 0x1bba3a, 0x1bba3c | | |
| mouse_buffer_flag/flag2/x/y/button | 0x1b9228/9220/921c/921e/9222 | 0 | buffered press |
| mouse_buttons | 0x1b921a | | buttons at the last driver callback |
| mouse_enabled | 0x184518 | | cursor drawn by the interrupt handler |
| mouse_palette | 0x1bb900 | | cursor images (see Cursor) |
| last_key_pressed[10], key_read, key_write, key_pressed | 0x1bc2ac, 0x1bc2e3, 0x1bc2e2, 0x1bc2e4 | | key ring |

## Registration

`Clear_Fields` (0x11c2f0): `fields_count=1; input_delay=0;
down_mouse_button=-1; auto_input_variable=0; input_field_active=0;
active_input_field_number=-1; multi_hotkey_active_field=0;` then
`Mouse_Buffer(); Mouse_Buffer2()` (drop a pending click). It does **not**
touch the key ring, the help list, the auto function or `selected_count`.

`Add_Hidden_Field(xmin, ymin, xmax, ymax, hotkey, button_sound)`
(0x11438b; Watcom: EAX, EDX, EBX, ECX, two stack args, `ret 8`):

1. `fields_count >= max_fields_count` → `Exit_With_Message("Too many fields")`.
2. `xmin > xmax || ymin > ymax` → `Exit_With_Message("ERROR: Bad Rect in Add_Hidden_Field")`.
3. Store rect, `type=7`, `button_sound` (16-bit), `hotkey = hotkey[0]`;
   if `0x60 < hotkey < 0x7b` subtract 0x20.
4. Return `fields_count++` (the new field number).

The 6th argument is the click sound (`button_sound`), not a help character:
the main menu passes `')'` = 0x29 = sound 41.

`Add_Hot_Key(str)` (0x114c72): same as above with the rect
`5000,5000,5000,5000` (never hit by the mouse), `type=7`, hotkey from
`str[0]` upper-cased; `button_sound` left stale. Full → `Exit_With_Message`.

`Clear_Fields_Above(n)` (keeps fields 0..n): `fields_count=n+1;
down_mouse_button=-1;` clear mouse buffer; `multi_hotkey_active_field` is
left as it is. `Set_Input_Delay(n)`: `input_delay=n;` clear mouse buffer.

### Buttons, multi hot keys and grids

- `Add_Button_Field(x, y, str, pic, hot, snd)` (0x1151b0): `fields_count >=
  max_fields_count` → `Exit_With_Message(field_exceed_message)` ("Too many
  fields", 0x184464); `Add_Button_Info` (0x114fba) fills the record (table
  above); `type = 0`; return `fields_count++`.
- `Add_Irregular_Button_Field(x1, y1, x2, y2, str, pic, dx, dy, hot, snd)`
  (0x11a92e): the same info with the explicit rect, `var2 = dx, var3 = dy`,
  `type = 0xd`. Its check is `fields_count <= max_fields_count`, so with 250
  fields one more is accepted and written to `sfields[250]`, i.e. over
  `simtex_field_chosen`..`x_pointer` and beyond (type lands on
  `last_button_number`); the port keeps a spare slot and does not emulate
  the clobber. Full → `sprintf("Maximum number of button fields exceeded
  (%d)", max_fields_count)` and exit.
- `Add_Multi_Hot_Key_Field(str)` (0x11adb4): full ("You exceeded the max
  number of fields (150)") → exit; `memset` the slot (55 bytes), rect -1,
  `type 8`, `var1=str`, `var2=0`, `var3=strlen(str)`,
  `multi_hotkey_active_field=1`.
- `Add_Grid_Field(x, y, bw, bh, nx, ny, &gx, &gy, snd)` (0x114dca): full
  (`fields_count >= max`) → exit; rect as above, `button_sound`, `type 0xc`,
  `var2=bw, var3=bh, var4=&gx, var5=&gy`, `hotkey=0`. Nothing else written.

`Auto_Input()` returns `auto_input_variable`. `Set_Field_Outline_Color(c)`
stores `field_outline_color`. `Enable_Mouse_Pressed_Field_Update()`:
`dynamic_field_refresh=1`, clear mouse buffer; `Disable_...` only stores 0.
`Disable_Cancel` / `Enable_Cancel` set `mouse_cancel_disabled` to 1 / 0.

## Raw input

### Keyboard (key.c)

`New09_Handler` chains the old INT 9, then if the BIOS buffer is non-empty
stores the BIOS keystroke word (scan<<8 | ascii) sign-extended to int into
`last_key_pressed[key_write]`, `key_write=(key_write+1)%10`, sets
`key_pressed=1`, and empties the BIOS buffer. No overflow check (an 11th
pending key overwrites). Typematic repeats arrive as keys.

- `Keyboard_Status()`: returns `key_pressed==1` **and clears `key_pressed`**.
- `Read_Key()`: returns `last_key_pressed[key_read]`; `key_read=(key_read+1)%10`;
  `key_pressed = key_read != key_write`. No emptiness check.
- `Check_Key()`: peeks `last_key_pressed[key_read]`.

One key is consumed per `Get_Input` call at most.

### Mouse (mouse.c)

The driver (INT 33h) is set up in `Init_Mouse_Driver`: X range 0..1278,
Y 0..479, `mouse_x = CX>>1`, `mouse_y = DX`. `User_Mouse_Handler` is the
driver callback; on each event it updates `mouse_x/y/buttons`, calls
`Check_Mouse_Buffer`, and unless a page flip is in progress
(`toggle_flag`) recomputes the cursor shape and, if `mouse_enabled`, moves
the cursor on screen (Restore/Save/Draw function pointers).

`Check_Mouse_Buffer`: if `mouse_buttons & 3` ≠ 0, store
`mouse_buffer_button = mouse_buttons & 3`, `mouse_buffer_x/y = mouse_x/y`,
`mouse_buffer_flag = mouse_buffer_flag2 = 1`. Release events never record.

The callback mask is switched by fields.c: `Mouse_Button_Handler` installs
mask 0x2B (move, left press, right press, middle press) and sets
`mouse_enabled=1`; `Mouse_Movement_Handler` installs mask 0x01 (move only).
`Interpret_Mouse_Input` uses the move-only mask while it polls and restores
0x2B on every exit, so between `Get_Input` calls a press is buffered even
without movement. Port rule: a press, or a move with any button down,
records the buffer; releases do not.

- `Mouse_Button()`: live state, INT 33h fn 3, `BX & 3` (1 left, 2 right, 3 both).
- `Mouse_Buffer()` / `Mouse_Buffer2()`: return the flag and clear it.
- `Mouse_Buffer_X/Y/Button()`: the stored values.
- `Set_Pointer_Position(x,y)`: `mouse_x=x; mouse_y=y;` driver fn 4 with `CX=2x`.
  This moves the visible pointer.

## Get_Input (0x1171ab)

```
if input_delay > 0 { input_delay--; return 0 }
if input_delay < 0 { input_delay = 0 }
if fields_count <= 1 { return 0 }          // keys stay queued
r = Interpret_Mouse_Input()
Set_Page_Off()
if r != 0 { Play_Sound(fields[abs16(r)].button_sound) }
return r
```

Result: `n>0` field selected (left button, Enter, hot key); `-n` field
selected with the right button (only when `mouse_cancel_disabled`, see
below); `-1` cancel (Esc or right button); `0` nothing; F-key codes (see
`Interpret_Keyboard_Input`). `Play_Sound(0)` is silent. For -1 the sound of
field 1 plays; for F-keys see the open questions.

## Interpret_Mouse_Input (0x11cef5), hidden-field and hot-key paths

Entry: `down_mouse_button=-1; auto_input_variable=0; field=0;`
`mx,my = Pointer_X/Y(); pointer_offset = Get_Pointer_Offset();`
On every exit (0x11e6ef): if `multi_hotkey_active_field` and the path set
the local `clear` flag (`[ebp-4]`), `Clear_Multi_Hot_Key_Offsets()`
(0x11aef7: every type-8 field gets `var2=0`). `clear` is set only by: the
key path's first exit (`c==0 || field<0 || c==-1`, which includes F-keys
and a completed multi hot key), Esc cancel, the buffered right-click cancel
(path C, which calls it directly at 0x11dbe6), and path B's return for
`b==1` or `b==2` (even with field 0). Path B's right-button cancel (-1),
path C selections, hot keys, Enter and D do **not** clear.

### A. A key is waiting (`Keyboard_Status()` true)

```
c = Interpret_Keyboard_Input(&field)          // field preset 0
if c == 0 || field < 0 || c == -1: Mouse_Button_Handler(); return field (clear)
if c == 0x8500 (F11) && release_flag != 1: F11_Key()   // Exit_With_Size; dead in release
if c == 0x8600 (F12): Screen_Flic_Capture() ...         // not covered
if c == 0x011B (Esc) && !mouse_cancel_disabled: return -1 (clear)
(` and ~ debug toggles, release_flag==0 only)
if field != 0 && fields[field].hotkey (zero-extended) == c:
    if type == 6: Mouse_Button_Handler(); return 0
    Push_Field_Down(field, mx, my); Mouse_Button_Handler()
    (types 1, 2, 9 specials — not covered)
    if type != 7 && type != 8: Quick_Call_Auto_Function()   // not for hidden fields
    down_mouse_button = -1; return field
if c == 0x1C0D or 0xE00D (Enter, keypad Enter):
    (input-field branch when input_field_active — not covered)
    f = Scan_Field()                     // field under the pointer
    if f == 0: return 0 (list fields, if list_field_on — not covered)
    if type(f) != 6 && != 0xB: Push_Field_Down(f, mx, my)
    Mouse_Button_Handler()
    if !mouse_auto_exit: Quick_Call_Auto_Function()   // also for hidden fields
    down_mouse_button = -1; return f
if c == 0x0D2B ('+') or 0x0C2D ('-'):      // only when no field has the key
    f = Scan_Field()
    if f != 0 && type(f) == 6:
        Increment_Scroll_Bar(f) / Decrement_Scroll_Bar(f)
        Mouse_Button_Handler(); down_mouse_button = -1; return f
(typing into input fields — not covered)
down_mouse_button = -1; Mouse_Button_Handler(); return 0
```

The key path wins over a buffered click; the click stays buffered for the
next call.

### B. A button is held (`Mouse_Button()` ≠ 0 after `Mouse_Movement_Handler()`)

```
b = Mouse_Button()
if b == 2:
    if help_list_active && Check_Help_List() == 0: clear buffer; return 0
    if !mouse_cancel_disabled:
        while Mouse_Button() == 2: Quick_Call_Auto_Function()
        clear buffer; return -1
record pressed_mouse_x/y if !pressed_button_flag
loop:
    if Mouse_Button() == 0: break
    x,y = Pointer_X/Y(); pointer_offset = Get_Pointer_Offset(); field = 0
    if dynamic_field_refresh: field = Scan_Field(); down_mouse_button = -1
    if field == 0: break               // pointer left all fields: give up
    Push_Field_Down(field, x, y)       // type 7: sets down_mouse_button=field
    auto_input_variable = field
    if mouse_auto_exit: break
    if Mouse_Button() != 0: Call_Auto_Function()
    Mouse_Buffer(); Mouse_Buffer2()
auto_input_variable = 0
if field != 0: clear buffer; last_button_number/x/y = b, x, y
Mouse_Button_Handler(); down_mouse_button = -1
return b==1 ? field : b==2 ? -field : 0       // b==3 (both) gives 0
```

So a hidden field is selected when the button is **released over it**
(the field under the pointer at the last poll before release); moving off
every field while held returns 0 at once. `b` is sampled once at entry.

### C. Nothing held, a buffered press exists (`Mouse_Buffer()` was 1)

```
b = Mouse_Buffer_Button()
if b == 2: (help check as in B → 0); if !mouse_cancel_disabled: clear buffer; return -1
x,y = Mouse_Buffer_X/Y()
Check_Mouse_Shape(); pointer_offset = Get_Pointer_Offset()   // from the *current* pointer
field = first i in 1..fields_count-1 with xmin ≤ x+off ≤ xmax and
        ymin ≤ y+off ≤ ymax (circles, type 0xE, use a distance test)
if field && type != 0xB:
    auto_input_variable = field
    Push_Field_Down(field, x, y)       // warps the pointer back to (x,y)
    Quick_Call_Auto_Function()
down_mouse_button = -1
if field: Mouse_Buffer2(); last_button_* = b, x, y
Mouse_Button_Handler(); auto_input_variable = 0
return b==2 ? -field : field          // b==3 counts as left
```

This catches a click that was pressed and released between two polls; it
is selected at the press position.

### D. Nothing

`pressed_button_flag = 0; Mouse_Button_Handler(); return 0`.

## Interpret_Keyboard_Input(&field) (0x11cace)

```
k = Read_Key()                        // full BIOS word
if selected_count >= fields_count: selected_count = 0
F-key table (repne scasw, 22 words at 0x11cb19, jump table 0x11cb43):
  0x3B00..0x4400 (F1..F10)       → *field = -1001..-1010, return k
  0x6800..0x7100 (Alt-F1..F10)   → *field = -1101..-1110, return k
  0x01F0                         → *field = -1500, return k
if multi_hotkey_active_field:                 // 0x11ccc4
    c = Get_Alt_Alpha_Key_((int16)k)          // zero-extended byte
    n = Check_Multi_Hot_Key_Fields_(c)
    if n > -1: *field = n; return -1
c = (int16)k % 256                    // C remainder, may be negative
if c == 0: c = k                      // extended keys keep the full word
if 0x60 < c < 0x7B: c -= 0x20         // case-insensitive
for i = 0 .. fields_count-1:          // note: starts at 0
    if active_input_field_number == -1 && fields[i].hotkey == c (16-bit compare):
        *field = i; break
if fields[*field].hotkey != c: c = k
return c
```

First match in field order wins. A dead block (starts at `fields_count`,
never loops) would centre `x_pointer/y_pointer` on a field. F11/F12 are not
in the table (`0x8500`/`0x8600`, low byte 0 → returned whole). Alt+letter
returns the full word and matches nothing. Esc returns 0x011B (low byte
0x1B, matched only if some hotkey is 0x1B).

### Multi hot keys

`Get_Alt_Alpha_Key_(k)` (0x11ab8a): `repne scasw` over 35 words at 0x11abad
with jump table 0x11abf1. Alt+letter scan words map to the letter
(0x1e00 A, 0x3000 B, 0x2e00 C, 0x2000 D, 0x1200 E, 0x2100 F, 0x2200 G,
0x2300 H, 0x1700 I, 0x2400 J, 0x2500 K, 0x2600 L, 0x3200 M, 0x3100 N,
0x1800 O, 0x1900 P, 0x1000 Q, 0x1300 R, 0x1f00 S, 0x1400 T, 0x1600 U,
0x2f00 V, 0x1100 W, 0x2d00 X, 0x1500 Y, 0x2c00 Z); 0x7800..0x7f00
(Alt-1..Alt-8) map to '0'..'7'; anything else 0xff. The 35th word scanned
is really the first jump-table entry (0xada2) and lands on the default too.

`Check_Multi_Hot_Key_Fields_(c)` (0x11af64): for `i = 0 ..
fields_count-1` while no hit: a type-8 field whose `str[progress]`
(zero-extended) equals `c` gets `progress++`; reaching `strlen` makes it the
hit. So **every** matching field advances until one completes; fields after
the completed one do not. A mismatch never resets progress (only the clear
flag above does); at `progress == strlen` the terminator 0 never matches.
Keys are typed with Alt held (cheats, Alt-Q exit on the main screen).

Port gap: `biosWord` gives Alt+digit as `scan<<8` (0x0200..), not the BIOS
0x7800.. words, so digit multi hot keys cannot be typed yet.

## Scan_Input / Scan_Field (hover)

`Scan_Field()` (0x113fb9): `Check_Mouse_Shape(); pointer_offset =
Get_Pointer_Offset();` then the first `i` in `1..fields_count-1` with
`xmin ≤ mouse_x+off ≤ xmax && ymin ≤ mouse_y+off ≤ ymax` (type 0xE adds a
circle test); 0 if none. The same offset is added to x and y.

`Scan_Input()` (0x114177): `Check_Mouse_Shape(); pointer_offset=...;
f = Scan_Field();` then by `fields[f].type` (f may be 0; field 0 is type 0):
9 (multi value) and 10 (inactive list → 0) not covered; 0xc (grid), when
`down_mouse_button == -1` or that field is not a grid:
`*var4 = (Pointer_X - xmin) / bw; *var5 = (Pointer_Y - ymin) / bh` — no
pointer offset and **no clamping** (Draw_Field below has both). Other types
return `f` unchanged.

## Push_Field_Down / Draw_Field / Draw_Visible_Fields

`Push_Field_Down(f, x, y)` (0x118fd4): only if `0≤x<640 && 0≤y<480`:
`Save_Mouse_State()` (mouse_enabled off), and if `f != down_mouse_button`:

```
d = down_mouse_button
if d != -1:
    if !(type(d)==3 && type(f)!=3):
        if type(d)==10: if type(f)==10 && fields[f].active_flag: Draw_Field(d,0)
        else Draw_Field(d,0)
down_mouse_button = f; Draw_Field(f,1)
if type(f)==4: x,y = pointer
Set_Pointer_Position(x,y)
```

then `Restore_Mouse_State()`. `Interpret_Mouse_Input` resets
`down_mouse_button=-1` on entry and in every held-loop pass (with
`dynamic_field_refresh`), so in practice only `Draw_Field(f,1)` and the warp
happen.

In the held loop (path B) the push is skipped when `field ==
down_mouse_button`, for types 4 and 0xb, and when both the pushed and the
new field are grids; with `dynamic_field_refresh` none of these hold, so
every pass pushes (and a grid recomputes its cell).

`Draw_Field(f, up)` (0x11b05a), nothing at all if `draw_fields_flag == 0`:

- types 0 and 0xd: position `(xmin,ymin)`, or `(var2,var3)` for 0xd.
  `up==0`: `Reset_Animation_Frame(pic); Draw(x,y,pic)` (frame 0).
  `up!=0`: if `var4.min_value == 0` `Reset_Animation_Frame + Draw`, else
  `Set_Animation_Frame(pic,1)` (clamped to `frames-1`); then `Draw` again,
  i.e. frame 1. Label: printed **only when the string is non-null and
  empty** (`cmp byte [str],0; je print` at 0x11b1eb and 0x11b3a2 — the
  reverse of Draw_Visible_Fields): `Set_Font_Style(text_style, text_color)`,
  `h = (Get_Font_Height()-1)/2`, `Print_Centered(xmin+(xmax-xmin)/2 [+down_x],
  ymin+(ymax-ymin)/2 [+down_y] - h, str)`, with the down offsets when
  pressed. So a non-empty label is never drawn pressed, and an empty one
  changes the font style (shadow off) and marks the refresh squares.
- 0xc (grid): `pointer_offset = Get_Pointer_Offset()` always; if `up`:
  `gx = (off + Pointer_X - xmin) / bw`, `gy = (Pointer_Y + off - ymin) / bh`
  (C division), clamped above to `(xmax-xmin)/bw`, `(ymax-ymin)/bh`, then
  below to 0, written through var4/var5. With drawing disabled the cell is
  not updated.
- 1, 2, 3 always and 4, 6 when pressed draw (not ported); 5, 7..11 and
  ≥ 0xe draw nothing.

`Draw_Visible_Fields` (0x11e718), run by every `Toggle_Pages`: save the
window, `Reset_Window(); Set_Page_Off();` and if `draw_fields_flag`, for
`i = 1 .. fields_count-1`: if `i == down_mouse_button` and type ∉ {4, 0xc,
10} → `Draw_Field(i,1)`; else by type (jump table 0x11e7fd for 0..0xb):
0 → `Reset_Animation_Frame; Draw(xmin,ymin,pic)` and, if the label is
non-empty, the centred print without offsets; 5, 7, 8, 9 and > 0xb nothing;
1, 2, 3, 4, 6, 10, 11 draw (not ported). Then, with `field_box_mode`, an
outline in `field_outline_color` around every non-type-8 field (debug, only
reachable when `release_flag==0`; not ported). After the loop, with
`help_box_mode`, help rectangles in `help_field_outline_color`. Finally
`Set_Window(saved)`. So irregular buttons are invisible until pressed, and
type-0 buttons are redrawn in frame 0 on every flip. The port reports draw
errors from the next `GetInput` (the flip hook returns nothing).

## Auto function and page flips

- `Assign_Auto_Function(fn, d)` (0x1191ca): `function=fn; auto_active_flag=1;
  auto_function_delay = (1 ≤ d ≤ 9) ? d : 2`.
- `Deactivate_Auto_Function` / `Activate_Auto_Function`: flag 0 / 1.
- `Invoke_Auto_Function`: `if auto_active_flag: function()`.
- `Call_Auto_Function` (0x1193a0): if active `Mark_Time_(); Set_Page_Off();
  function(); Toggle_Pages(); Release_Time_(auto_function_delay);` else
  `Toggle_Pages()` only; then `update_function()` if set.
- `Quick_Call_Auto_Function` (0x119325): `Mark_Time_();` if active
  `Set_Page_Off(); function();` then `Toggle_Pages(); Release_Time_(1);
  update_function()`.

The auto function never runs in an idle `Get_Input`. It runs only inside
`Interpret_Mouse_Input`: once after a hot key on a non-7/8 field, after
Enter, after a buffered click; repeatedly while a button is held over a
field (`Call_Auto_Function`, delay `auto_function_delay`) and while the
right button is held for cancel (`Quick_Call`). Each run is a full
draw + page flip, so the screen keeps animating during a held click.
`update_function` is `Mox_Update_` (sound/network update).

`Toggle_Pages` (0x124ecb) draws visible fields, shows the off page, puts the
cursor on it (below), refreshes the palette, and calls `Set_Page_Off`.

## Timing

`Mark_Time_` stores the BIOS tick count (dword 0x46C) in 0x1845dc;
`Release_Time_(n)` busy-waits until `ticks - mark ≥ n` (returns at once if
the difference is negative). Ticks are the 18.2 Hz BIOS clock (see open
questions). `input_delay` counts `Get_Input` calls, not ticks.

## Help list (right button)

`Set_Help_List(list, n)`: `help_struct_pointer=list; help_list_active=1;
help_list_count=n`. `Deactivate_Help_List`: all zero. `Check_Help_List`
(0x119732): with the *current* pointer (no offset), the first entry
`{help_id, x1, y1, x2, y2}` (int16 × 5) with `help_id ≠ -1` and the point
inside → `Draw_Help_Entry_(help_id)` (modal help box, not covered; it sets
`_FULL_DRAW_SCREEN_FLAG=1`) and return 0; none → 1, and the right button
then means cancel (-1).

Main menu list `_main_menu_screen_help_list` (0x181e66), ids 645..650:
`(645,415,172,567,193) (646,415,195,567,215) (647,415,217,567,238)
(648,415,240,567,260) (649,415,262,567,283) (650,415,285,567,306)`.

## Cursor

- A mouse list is an array of `s_mouse_list {int16 image_num,
  center_offset, x1, y1, x2, y2}`. `Set_Mouse_List(list, n)` sets it and
  calls `Check_Mouse_Shape`.
- `Check_Mouse_Shape` (0x123ea7): `ptr = -1`; for every entry `i` (no break)
  with `x1 ≤ mouse_x+center ≤ x2 && y1 ≤ mouse_y+center ≤ y2` (each entry's
  own `center_offset`) set `ptr=i`; `shape = ptr==-1 ? 0 : list[ptr].image_num`.
- `Get_Pointer_Offset` returns `list[ptr].center_offset` (reads `list[-1]`
  when `ptr == -1`). This is the hit-test offset; the picture is not shifted.
- Main menu: `Fade_Into_Screen_` sets `_mouse_list_none` (0x17cf4a,
  `{0,0,0,0,639,479}`: no cursor) during the fade, then `_mouse_list_arrow`
  (0x17cf00, `{1,0,0,0,639,479}`). main2_ also sets the arrow list after
  `Load_Palette(5)`. On the `previous_screen==10` path the list is left as
  the previous screen set it.
- Images: `Load_Palette(n, ...)` sets `mouse_palette` = record `n+1` of
  `FONTS.LBX` + 0x400. Each palette record holds 20 cursor images of
  24×24 bytes (0x240) after the 0x400 bytes of colours. Image `shape`
  is at `mouse_palette + (shape-1)*0x240`; bytes are palette indices, 0 is
  transparent. Shape 0 draws nothing. The arrow is image 1 of palette 5
  (FONTS.LBX entry 6), hotspot at its top-left pixel.
- Drawing (`Draw_Mouse_Hi_Res` → `Draw_Mouse_Buffer_`,
  `Draw_Mouse_Off_Page` → `Draw_Mouse_Off_Page_Buffer_`): top-left at
  `(mouse_x, mouse_y)`, clipped to 640×480 (width `640-x` when
  `x+24>639`, height `480-y` when `y+24>479`). It is drawn on the visible
  page after each flip and moved by the interrupt handler between flips;
  pages themselves never contain it. Port: composite the cursor over the
  presented page at the current pointer with the current shape.
- `Save_Back`, `Restore`, `Put_Mouse`, `Mouse_Setup`, `Save_Back_Always`
  in fields.c are empty stubs.

## Main menu usage

`Add_Main_Menu_Fields_`: `Clear_Fields()`, then hidden fields, sound 0x29,
x 0x19f..0x237:

| Slot (`_mainmenu`+) | y | Hotkey | Condition |
|---|---|---|---|
| 0x0c Continue | 0xac..0xc1 | C | `save10.gam` exists and `Get_Game_Type_From_Saved_Game_(9)` gave 0/1 (read from DL; not re-checked), else -1000 |
| 0x1a Load | 0xc3..0xd7 | L | `!_disallow_saves_code` and `save?.gam` exists, else -1000 |
| 0x28 New | 0xd9..0xee | N | always |
| 0x36 Multiplayer | 0xf0..0x104 | M | |
| 0x44 Hall of Fame | 0x106..0x11b | H | |
| 0x52 Quit | 0x11d..0x132 | Q | |

`Main_Menu_Screen_` then calls `Add_Hot_Key("N")` (string 0x17969e, the
same as New's), `Assign_Auto_Function(Draw_Main_Menu_Screen_, 2)`,
`Set_Main_Menu_Screen_Help_List_()`, and loops:

```
r = Get_Input(); _scanned_field = Scan_Input(); Mark_Time_()
dispatch r (and the key returned by the opening animation)
if not leaving: Set_Page_Off(); Draw_Main_Menu_Screen_(); _TOGGLE_PAGES_()
Release_Time_(3)
```

Hover: `Draw_Main_Menu_Screen_` draws the highlight picture of the field
equal to `_scanned_field` (from the previous `Scan_Input`; not refreshed
while `Get_Input` is inside a held-button loop). Since New's hidden field
has a lower number than the hot key, `N` always matches the hidden field;
the extra hot key is never returned.

## Scroll fields (type 6)

`Add_Scroll_Field(x, y, min_value, max_value, min_valid, max_valid, w, h,
&variable, hotkey, sound)` (0x115478): rectangle `(x, y)-(x + w, y + h)`;
`var1` = min_valid, `var2` = max_valid, `var3` = the variable (i16 *),
`var4` = min_value, `var5` = max_value (s_fields offsets +0x18, +0x1c,
+0x20, +0x24, +0x28); `var6` (+0x2c) = 0 if `h < w` (horizontal) else 1
(vertical); hot key upper-cased as usual.

- `Draw_Field(f, down)`: up draws nothing; down (only when
  `draw_fields_flag`) is `Find_Bar_Position(f)`.
- `Find_Bar_Position` (0x1156e2, assembly), `p` = pointer + `_pointer_offset`
  (i16; the global 0x1b3e14 as `Scan_Input` last stored it, the port
  recomputes it from the mouse list), `R = max_value - min_value`:
  - horizontal: `p ≥ xmax` → max_value; `p ≤ xmin` → min_value; else
    `min_value + (p - xmin)*R / (i16)(xmax - xmin)`;
  - vertical: `p ≥ ymax` → min_value; `p ≤ ymin` → max_value; else
    `(ymax - p)*R / (i16)(ymax - ymin)` (measured up from the bottom, **not**
    offset by min_value);
  - then clamped to `[min_valid, max_valid]` and stored.
- `Draw_Visible_Fields`: a scroll field that is not the pushed one only
  clamps its variable to `[min_valid, max_valid]`.
- `Scan_Input` returns the field, nothing else.
- Held button (path B): every loop pass rescans and pushes the field under
  the pointer, so `Push_Field_Down` → `Draw_Field(f, 1)` moves the bar each
  pass and `Call_Auto_Function` redraws the screen. Before pushing a new
  field, a pushed scroll field gets `Invoke_Auto_Function` (only without
  `dynamic_field_refresh`, where `down_mouse_button` survives the pass);
  the same when the pointer leaves all fields. After the loop, if
  `fields[auto_input_variable]` is a scroll field: `Invoke_Auto_Function`
  (the auto function alone, no page flip). The result is the field number
  as for any field.
- Buffered press (path C): `Push_Field_Down` sets the bar once from the
  current pointer (not the buffered position), then
  `Quick_Call_Auto_Function`.
- Key path: a scroll field's own hot key returns 0; Enter does not push
  it; `+`/`-` (above) step it with `Increment_Scroll_Bar` (0x115994) /
  `Decrement_Scroll_Bar` (0x115ac0): `pct = (i16)((v - min_value)*100 /
  (i16)R) ± 5`; up: `pct < 0x65` → `min_value + pct*R/100` else max_valid,
  then at most max_valid; down: `pct < 0` → min_valid else `min_value +
  pct*R/100`, then at least min_valid.

## Not covered

Field types 1 (toggle), 2 (radio/locked), 3, 4, 9 (multi value),
10 (list), 11 (string input), 14 (circle). Also string/list fields,
`Wait_For_Input`, `Timed_Wait_For_Input`, `Screen_Flic_Capture` (F12),
`Draw_Help_Entry_`, the mouse override inputs, the debug field/help
outlines, the `dynamic_field_refresh==0` branch of the held loop, and the
`sfields[250]` clobber of the irregular-button overflow.

## Open questions

1. F-key results (-1001.., -1101.., -1500) make `Get_Input` play
   `fields[1001+].button_sound`, far past `sfields[250]` (into other .bss).
   The value read at run time is unknown.
2. Key word 0x01F0 (→ -1500) is not a standard BIOS code; meaning unknown.
3. Which LBX `Play_Sound` numbers index (`soundLBX`), i.e. what sound 41 is.
4. Tick rate: assumed 18.2 Hz BIOS ticks. Miles (AIL) reprograms the PIT;
   that it keeps 0x46C at the BIOS rate is not verified here.
5. `Add_Hot_Key` leaves `button_sound` stale, and `Interpret_Keyboard_Input`
   compares against field 0, which is never assigned; both are harmless for
   the main menu but the port must keep stale slot contents if it wants
   bit-exact sounds for hot keys elsewhere.
6. `default_mouse` (fields.c, 0x18449a) reads `{0,0,0,639,479,0}`, which
   looks shifted by one word; harmless because `Init_Mouse_Driver` installs
   `default_mouse_list` afterwards.
7. In `Main_Menu_Screen_`, when no fade path runs and `previous_screen≠10`,
   `Add_Main_Menu_Fields_` is skipped and the `_mainmenu` slots stay -1
   (from the `memset`), so Esc or right click (-1) would match Continue.
   When that path is reached is not established.
8. `Get_Current_Font_Color` returns `current_font_color_array`, a pointer;
   the port copies the printer's normal colours at add time. If the game
   later rewrites that array in place, the original would see the change.
9. `Draw_Field` prints a button label only when it is empty (see above).
   This looks like an inverted test in the original; it is ported as is.

## Field types the port leaves out (2026-10-19)

`Draw_Field`, `Scan_Input` and `Push_Field_Down` still return an error for
a few field types; none is created in a single-player game:

- type 4 comes only from `Add_Input_Field_` (0x115e3b), called only by
  `Get_Input_String_`, which nothing calls;
- type 0xe (circle) from `Add_Circle_Field_` (no caller); the clipped
  variant, `Add_Clipped_Circle_Field_`, is ported for `Add_Star_Fields_`
  of the F9 star distance screen (`docs/screens/star-distances.md`);
- types 5 and 9 are never written by any `Add_*_Field_`;
- the list rows' gradient fill: every `Get_List_Field_` caller passes fill
  type 0x11 (the council's lists included), which the port draws.

## Glassed animations the port leaves out

`DrawFileAnimation`, `DrawMultiFileAnimation` and glassed pictures of a
kind other than sprite or bitmap return an error. A scan of every picture
in the game's LBX files finds glassed pictures only in BEAMS.LBX (67, 69,
87, 99, 109), CMBTSFX.LBX (15, 52) and SPHERSFX.LBX (0, 1), all of kind 0
(sprite), drawn by the combat effects through `Draw` (the frame oracles
tcfx1, tc10f cover them). None is a file or multi-file animation (those
are the ambassadors, GNN, the science room, the council, the logos), so
the two missing paths are not reached.
