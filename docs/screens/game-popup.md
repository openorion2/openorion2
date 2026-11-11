# The game popup and Load Game (GAME.C, LOADSAVE.C)

Go: `internal/game/loadsave.go`. Addresses are ELF. `_screen_data` picks the
page: 0 main, 1 options, 2 load, 3 save. Ported: the main menu's Load Game,
and screen 8 (the GAME button) with its main, options and load pages and the
slide-in animation, and the save page with the engine's text input fields.
Changing the language cannot happen: `Do_Options_Game_Popup_` (0x7e118)
reloads the strings when the popup's language (+0x25e) differs from the
one it started with, but no field writes +0x25e (only
`Set_Current_Game_Option_Flags_` and `Update_Game_Settings_` copy it to and
from settings +0xd2), and `Reload_Language_Strings_` forces English anyway
(0x7a9be). The port keeps the branch as a comment. Its callees
`Set_Language_Race_Names_` (0x7abd7) and `Update_Ship_Names_To_New_Language_`
(0x56484) have no other caller, so they are not ported; `Load_Language_Fonts_`
(0x7aa05) is called only by the modem screens.

## _Game_Popup_ (0x8012f), screen 8

Pictures: GAME.LBX 0..0x13 into `_game_anims` 0..0x13 (8 from 0x1d), 0x1b and
0x1c into 0x17, 0x18 (darkened LOAD and NEW). `Set_Current_Game_Option_Flags_`
copies the settings into the popup (sliders = level·0x9b/100, the 13 option
flags from `_settings` +0, +1 (single player only), +3..+0xd, the language
+0xd2). Then pages until one leaves: the first pass with animations
(`_settings` +6) slides the main popup in with `Animate_Game_Popup_`; load and
save run `Set_Up_Load_Save_Popup_` and `_return_screen` = `_previous_screen`.
On leaving: `Update_Game_Settings_`, `Save_Session_Related_Settings_`,
`_current_screen` = `_return_screen`; with animations and not slid away yet,
the main popup is drawn without the darkened buttons, 7 ticks, slides out.
New game (screen 13) fades out; otherwise, unless a game was loaded, the
mini main screen is redrawn.

`Animate_Game_Popup_(pic, y, dir, page)` (0x7edf2) draws the page into a bitmap
of the popup picture (`_print_data_to_bitmap`, with its own quirks: the main
page adds the darkened buttons only for multiplayer games, the options page
prints "(ALT_Fn)" strings that skip ALT_F4), then each step moves y by
h·step/100·dir and draws the bitmap at (0x90, 0x15−h+y) clipped to
(0x90, 0x19)..(0x90+w, y+0x1a), step 14 sliding in pinned at 0x19.

Main page (`Do_Main_Game_Popup_` 0x7dd41) at (0x90, 0x19): SAVE "S", LOAD "L"
and NEW "N" (single and hotseat only), QUIT "Q", SETTINGS "O", RETURN (Esc),
two scroll fields for music and sound (0x9b wide, the bars drawn clipped to
the level). NEW and QUIT ask H 0xba / 0xbb; QUIT saves slot 10, sets
`_settings` +0x15 = 10 and goes to screen 7. SAVE with `_disallow_saves_code`
shows H 0xb9. The Esc field slides the popup out at once.

Options page (`Do_Options_Game_Popup_` 0x7e00f): 13 rows of a check box
(GAME.LBX 9, frame = flag) and a label (H 0xa6..0xb1, 0x187), each with a
hidden field on the box and one on the label that toggle the flag; ACCEPT
"A" goes back to the main page, Esc leaves.

## Do_Mainmenu_Load_Screen_ (0x810c7)

MAINMENU.LBX 0x15 onto the back page (after `Fill(0, 0, 0x289, 0x1df, 0)`,
wider than the screen), copied to off, flipped; help off;
`_previous_screen` = 10. Then `Mainmenu_Load_Game_Popup_` until its out
flag is 1; it returns AL of the last call, whether a game was loaded.
`Main_Menu_Screen_`: loaded → the Continue path with message 0xd0 (network
and modem games refused); otherwise screen 10 (screen 0 if
`_previous_screen` were 0, which it never is here).

## Mainmenu_Load_Game_Popup_ (0x804b7)

`_screen_data` = 2, `_draw_game_popup_to_back` = 1,
`_mainmenu_first_seg` = MAINMENU.LBX 0x15, `_game_anims` zeroed and
`_game_popup_fields` (0x278 bytes) set to 0xff. `Load_Month_Names_` (H
strings 0xbe..0xc9 into 6-byte slots), GAME.LBX 0x14..0x1a into
`_game_anims` +0x2c (popup), +0x30 (LOAD), +0x34 (CANCEL), +0x40..+0x48 (game
type icons: single, hotseat, network/modem), +0x4c; `Set_Up_Load_Save_Popup_`,
`Do_Load_Game_Popup_`, `Clear_Fields`.

## Set_Up_Load_Save_Popup_ (0x80556)

`Get_Saved_Game_Descriptions_` (0x11c83), per SAVE<n>.GAM: no file → H 0x184
(the empty slot text), stardate −1; version ≠ 0xe0 → the same; else the
description (+4, 0x25 bytes) and stardate (+0x29); slot 10 becomes
"(Auto Save)", an empty description "<< no description >>". Descriptions are
zero padded. `Get_Save_Game_Date_Strings_`: the DOS time stamp as
`"%3s %2d, %-4d  %2d:%2d"` (month, day, year, hour, minute), a blank at 14 or
17 turned into '0'. `Get_Star_Date_Strings_`: H 0xca + the stardate with a
dot before its last digit; −1 → "* INVALID *", 0 → empty. An empty
description with a stardate gets H 0xcb with the 0-based slot. The selected
slot (`_settings` +0x15, the last one loaded) becomes −1 unless 0..9 with a
positive stardate.

The selected slot's description is shown between the colour codes 0x03 and
0x01 (`Embed_Special_Color_Codes_` 0x806a2), the others have them taken off
(`Remove_Embedded_Special_Codes_` 0x80658: `memmove(d, d+1, strlen(d))`,
which moves the terminating NUL too, then a NUL at the 0x01). A save made
while its slot is selected keeps the codes in its file, so a description
can start with 0x03 without the 0x01 after it.

## Add_Game_Popup_Fields_ (0x7d061), _screen_data 2

The popup centred on the screen. `Check_For_Saved_Games_`: 1 no file, 2 no
positive stardate or bad magic, 3 a network/modem game, else 0. Colours from
`Get_Game_Popup_Colors_` (main menu: `Font_Colors2_(style, 0xfe, 0xff)` and
`(style, 0xfb, 0xfc)` highlighted, the latter as the font's special colours).
The selected slot's description is wrapped in 0x03..0x01, others lose it.
Per slot i (y = base+0x18+i·0x1f): status 2 → "* INVALID *", the game type
(file byte 0x2d) stored, a hidden field (base+0x1d, y, +0xc5, +0x18). Then a
hidden "D" field in the corner (sound '3'), LOAD (base+0x25, +0x151, "L"),
CANCEL (base+0xab, +0x152, "C"), the whole popup, a full-screen Esc field.

## Do_Load_Game_Popup_ (0x7da76)

In the game it sets the load help list; a successful load with animations
from the main screen slides the load popup out first, and after it the game
fades in (`Fade_Into_Screen_`, palette 0) when `_stardate_at_beg_of_turn` is
set. CANCEL goes back to the main page in the game. A click on a slot loads it; LOAD loads the selected slot (and sets the
unread `_draw_down`). Status 0: `Load_Game_`, the slot becomes `_settings`
+0x15, `Save_Session_Related_Settings_`; a failure shows H 0xb8 with the slot
in `Game_Popup_User_Box_` (a warning, then `Restore_Game_Popup_`). Any other
status: `Bad_Game_File_Message_` (H 0xb2/0xb3/0xb4 with slot+1) and the popup
is rebuilt. CANCEL (`_screen_data` = 0) and Esc leave. Each pass otherwise
redraws through `_Draw_Load_Save_Game_Popup_` (0x7f206): MAINMENU 0x15, the
popup, per slot the game type icon at (+0xe4, +0x24), the description in
style 2, the stardate (unless the slot shows the empty text) and the date at
+100 in style 1, highlighted for the selected slot. On leaving, the colour
codes are removed again.

## Save page (Do_Save_Game_Popup_ 0x7e154)

Fields: per slot a text input field (`Add_Continuous_String_Input_Field`,
x−3 of the load page's, 200 wide, the font height high, editing the
description in place, 0x1e characters, sound 0x29), then per slot a hidden
field on its stardate line, SAVE "S" at (+0x25, +0x152), CANCEL. A click on a
slot's text makes it the edited field; picking the same slot again saves
with the typed text. A click on a stardate line starts editing that slot by
hand (`active_input_field_first` set, so the first Backspace clears it), or
saves if it is the slot already picked. Enter in the field returns its
number, which saves. SAVE saves the picked slot, else the last one used.
Saving: `_settings` +0x15 = slot; an empty description (or the empty slot
text) becomes "%s, %s, %d %s" of the emperor, the race, N_Colonies_ and
H 0xbc/0xbd, cut to 36 characters; `Save_Game_`, `Save_Session_Related_
Settings_`, the dates and stardates again; the popup slides out. A cancel
(−1, from Esc or the right button, lands in the slot fields' range) or
CANCEL goes back to the main page.

The text input fields (type 0xb, engine): `input_field_active` (0x1844ca),
`active_input_field_number` (0x1844d0), `cursor_on`/`cursor_count`,
`continuous_string`. While one is edited, hot keys are off
(`Interpret_Keyboard_Input` compares them only with no active field);
printable keys append (with up to ten queued letters at once) while the text
fits the field, Backspace removes the last character, Ctrl-Backspace clears.
Enter copies the text back (cutting a trailing '_') and returns the field;
the redraw it triggers can blink the cursor back into `continuous_string`
before the caller reads it, as in the original. `Draw_Visible_Fields` prints
the edited text in the highlight colours at (x+10, centred) and blinks '_'
every two draws.

## Check

`TestMainMenuLoadPopup`: the popup over the saves in
`build/oracle/loadpopup` (with their file times) is pixel-identical to the
original's DOSBox capture `original.png`. `TestGamePopupPages`: the main,
options, load and save pages over `build/oracle/gamepopup` (SAVE10
continued) match the original's captures inside the popup; the map's black
holes animate. `TestGamePopupSave`: SAVE into the last slot writes the same
SAVE2.GAM and MOX.SET as the original (`saved/`). `TestGamePopupLoad`
(build/oracle/loadgame, capture.sh with `-clicks`): GAME, LOAD, an F11 click
on the third slot loads SAVE3 and play goes on: the popup pages, the map
after the load, every draw and the F10 save match the original's.
The in-game load page shows no month names, in the original as well.
