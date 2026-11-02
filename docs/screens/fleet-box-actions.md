# Fleet box actions: colonize, outpost, unload troops

The fleet box's COLONIZE PLANET, BUILD OUTPOST and UNLOAD TROOPS buttons
(`docs/screens/fleet-box.md`, Scan_Moveable_Box_Fields_) set
`_current_screen` to 0x1e, 0x1f or 0x22 with `_return_screen = 0`.
Screen_Control_ (0x106e5..0x1070d) runs one of three MAINSCR.C routines.
Port: `internal/game/mainscreen_fleetactions.go`.

## Shared head

All three (0x8b2de, 0x8a97a, 0x8b7a5) start the same way:

- `saved = settings[4]`; `star = Ship_Stack_Star_Id_(_ship_stack_start[_fleet_box_ship_stack])`
  (the head ship's location); `settings[4] = 0`.
- `Assign_Auto_Function_(Draw_Mini_Main_Screen_, 1)`, `Clear_Fields_`.
- The box's corner is compared with 0x20f and 0x1a5 and the flags are
  never used (a removed condition): `Vanish_Fleet_Movement_Box_` always.
- `Disable_Draw_All_Fields_`, `Set_Page_Off_`, `Fill_(0, 0, 0x27f, 0x1df, 0)`,
  `Draw_Mini_Main_Screen_`, `_TOGGLE_PAGES_`, `Enable_Draw_All_Fields_`.

## Shared tail (0x8ab19)

`settings[4] = saved`, `_current_screen = _return_screen`, `Clear_Fields_`,
`Deactivate_Auto_Function_`, `Clear_System_Display_Loaded_Flag_`. The
colonize routine jumps into this tail of Building_Outposts_In_Main_Screen_.

## Planet_Colonization_In_Main_Screen_ (0x8b2de, screen 0x1e)

- `Can_Change_Star_Name_(star)` is called and its result dropped.
- `pl = New_Colony_Selection_Popup_(star, 1)` (the colony-ship title
  E 0x1ce, the confirmation box).
- `pl > -1`: the first ship of the box (k = 0.. `_fleet_box.n`) whose design
  type (+0x11) is 1. Only if one is found: `landing = settings[6] != 0`,
  `Make_New_Colony_(pl, PLAYER_NUM)`, that ship's status (+0x64) = 5,
  `last = (_fleet_box.n == 1)`, `_first_node_ptr_in_stack = -1`,
  `_fleet_icon_buttons_added--`. The ship is not killed here: status 5
  ships go at the next turn (Make_Scrap_Ships_Dead_).
- `landing`: `_screen_data[0] = star`, `[1] = Planet_Orbit_(pl)`,
  `Colony_Landing_Screen_` (it sets `_current_screen = 1`, which the tail
  overwrites), then `Fade_Into_Screen_(Draw_Mini_Main_Screen_)` with EDX
  still `&_draw_main_screen_to_back` (set to 1) and palette 0.
- `!last`: `Set_Page_Off_`, `Draw_Main_Screen_Filled_(1)`,
  `Draw_Main_Screen_`, `_TOGGLE_PAGES_`, `Load_Player_Ship_Palette_(PLAYER_NUM)`,
  `moveable_box[2].content = 1`, `Build_Ship_Icons_`,
  `_fleet_box_ship_stack = Ship_Stack_From_Saved_Array_()`, −1 →
  `Vanish_Fleet_Movement_Box_`.

## Building_Outposts_In_Main_Screen_ (0x8a97a, screen 0x1f)

As colonize with `New_Outpost_Selection_Popup_(star)` (no confirmation),
design type 4, `Make_New_Outpost_`, no landing screen and no
Can_Change_Star_Name_; the redraw also sets `_eta_star_id = -1` after
Build_Ship_Icons_.

## Unloading_Transports_In_Main_Screen_ (0x8b7a5, screen 0x22)

- `pl = Drop_Troops_Selection_Popup_(star)` (0xc8d3c: selection mode 2,
  `using_ship = 0`, the title E 0x1cf; only the player's settled colonies
  are accepted, E 0x23b otherwise).
- `pl > -1`, its colony `c > -1` and owned (signed byte) by PLAYER_NUM:
  `n = Unload_Transports_(&colony[c], ids, Get_Fleet_Box_Selected_Ship_Ids_(ids, 500, 2))`
  (transports killed one by one, 4 marines each, until the colony's limit),
  `last = (n == _fleet_box.n)`, `_first_node_ptr_in_stack = -1`,
  `_fleet_icon_buttons_added--`.
- `!last`: `Update_Ship_Icons_In_Between_Turns_` then the redraw above
  without Build_Ship_Icons_.

After the screen the main screen comes back; 0x1e and 0x1f are in
Is_Main_Screen_Palette_'s list, 0x22 is not, so Main_Screen_ fades in.
