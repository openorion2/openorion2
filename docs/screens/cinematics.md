# Cinematics (Smacker videos) and the start-up logos

Video and the Smacker audio tracks. Port: `internal/game/cinematic.go`,
`logos.go`; decoder `internal/smk`.

## Audio

Every video has one track (header rates at 0x48): compressed 8-bit DPCM,
11025 Hz stereo (flags 0xd0) or 22050 Hz mono (0xc0: AMEBAFIN, WININFIN,
GENWINFN). `Open_Smacker_Animation_(name, sound, flags)` (0x132646):
`SMACKSOUNDUSEMSS(0x3c, digital driver)`; with sound = 1 the open flags are
0xfe040 (all seven tracks) and `SMACKVOLUMEPAN(track mask 1, 0x8000,
0x8000)` (full volume, centred). `Play_Cinematic_` passes settings +0x13
(music on). Chunk: the unpacked size, then a bit stream (LSB first): data
present, stereo, 16-bit, one 8-bit Huffman tree per channel (two per
channel for 16-bit; an absent tree yields 0), the base samples (right
channel first; 16-bit as high, low byte), then the deltas, channels
interleaved. `smk.Track` decodes it; `TestAudioTracks` matches FFmpeg's
decode of all ten videos byte for byte (size and SHA-256). The port feeds
each decoded frame's samples to a mixer stream voice
(`Engine.StartVideoSound` / `VideoSound`, `audio.Mixer.StartStream` /
`Append`) and stops it at the close; `TestCinematicSound` checks every
decoded frame's samples reach it. RAD's own buffering and timing of the
sound (it plays ahead of the video) is not modelled: the voice plays the
samples as the frames are decoded.

## Files

Ten LBX files are Smacker (SMK2) files, every one at 76.92 ms a frame
(header rate −7692):

| File | Size | Frames | Played by |
|---|---|---|---|
| INTRO.LBX | 480×160 | 1407 | `Draw_Logos_` |
| PLNTDFIN.LBX | 288×208 | 214 | `Play_Planetary_Destruction_Flic_` (string 0x182a7a) |
| AMEBAFIN.LBX | 288×208 | 176 | `Play_Amoeba_Flic_` (0x1829ce) |
| DIMTVFIN.LBX | 288×208 | 350 | `Play_Dimensional_Portal_Flic_` (0x182a2e) |
| ANATKFIN.LBX | 288×208 | 133 | `Play_Antarans_Attack_Flic_` (E 0xae) |
| ORIONFIN.LBX | 288×208 | 173 | `Play_Take_Orion_Flic_` (E 0x194) |
| WININFIN.LBX | 480×160 | 503 | E 0x22c, `Play_Diplomatic_Win_Flic_` |
| GENWINFN.LBX | 480×160 | 483 | E 0x136, the win flics |
| ANWINFIN.LBX | 480×160 | 323 | E 0xb7, `Play_Antaran_Win_Flic_` |
| LOSERFIN.LBX | 480×160 | 387 | E 0x16b, `Play_Loose_Flic_` |

LOGO.LBX is an ordinary LBX: entries 0 (MicroProse) and 1 (Simtex) are
file animations. SIMTEX.LBX (string 0x178c20) is not shipped.

## Play_Cinematic_(file, flags, fade) (0x14df7)

1. `Set_Page_Off`. Fade: `Smack_Fade_(4)`, the empty mouse list (0x180178).
   No fade: `Get_Auto_Function_(0x19a018)`, deactivate it,
   `Set_Refresh_Stencil`, `Enable_Draw_All_Fields`, `Draw_Visible_Fields`,
   `Copy_Off_To_Back`.
2. `Set_Refresh_Differential`; the auto function is `Draw_Smack_Screen_`
   (0x150e5); `Clear_Fields`, `Deactivate_Help_List`, window the whole
   screen, `Clip_On`; open the file (settings +0x13 goes to the open: sound).
3. x = (640 − w)/2; y = (480 − h)/2, or (480 − h)/4 with flags 0x100000
   (signed divisions).
4. Fade: black off page, `Toggle_Pages_`, black again, the first frame
   drawn, `Refresh_Palette_`, `Darken_Palette_(100)`, `Toggle_Pages_`,
   `Set_Fade_Percent_(100)`, `Smack_Fade_(3)` (darken 90, 80 … 0).
5. Loop: a waiting key is read and ends it, a held button ends it; draw
   the frame (`Draw_Smacker_Animation_`), `Toggle_Pages_`, `Mox_Update_`;
   it ends when the frame number reaches the last but one (the last frame
   is never shown: the number stops at frames − 1 and the loop then ends).
6. `Deactivate_Auto_Function`. Fade: `Smack_Fade_(4)` — a no-op, since
   the fade percent is still 100 (**quirk**: the video never fades out),
   black off page, `Toggle_Pages_`, `Set_Fade_Percent_(0)` (arrow
   cursor), fade speed 10, `Fast_Fade_Out_` (darkens a black screen and
   leaves the fade percent at 100).
7. Close, `Mox_Update_`, music. No fade: `_FULL_DRAW_SCREEN_FLAG` = 1 and
   the saved auto function back. `Set_Refresh_Stencil`.

`Smack_Fade_(mode)` (0x15085): mode 3 returns at fade percent 0, else
darkens 90 … 0; mode 4 returns at 100, else darkens 0 … 90; then
`Clear_Palette_Changes(0, 255)`. It never sets the fade percent.

`Draw_Smacker_Animation_(x, y)` (0x132869): `SmackWait` until the frame is
due, `Set_Smacker_Palette_` (colours 10..245, marked changed) when the
frame has a new palette, the frame into the off page at (x, y), each
changed rectangle onto the scan list, and `SmackNextFrame` unless it is the
last frame. The port copies the whole frame and adds its rectangle (the
screen shows the same; tcs1's frames match) and waits one frame period
after each advance.

## The wrappers (CINEMATI.C)

Popup ones — `Play_Amoeba_Flic_` 0xc64ab, `Play_Planetary_Destruction_
Flic_` 0xc6513, `Play_Dimensional_Portal_Flic_` 0xc657b, `Play_Antarans_
Attack_Flic_` 0xc65e3, `Play_Take_Orion_Flic_` 0xc6650 —
`Do_Cinematic_Popup_Setup_` (0xc63bb: saves the auto function at
0x1a08f4; with music or sound on (settings +0x11, +0x13) saves the volumes
+0x12, +0x14 at 0x18319f/0x1831a1 and raises +0x14 to the louder; empty
mouse list 0x17cf4a; `Cinematic_Autofunction_` (the mini main screen and
MAINPUPS 0x53 framed) drawn and made the auto function; `Push_Block`,
`Copy_Off_To_Back`), `Play_Cinematic_(file, 0, 0)`, then the cleanup
inline: arrow, the auto function back, `Pop_Block`, the volumes back if
either saved one is non-zero.

End of game — full screen with fades (bl = 1 from the first
`Set_Mouse_List_(0x17cf4a)` survives the calls): `Play_Loose_Flic_`
0xc6762 (E 0x16b), `Play_Normal_Win_Flic_` 0xc671b (E 0x136, flags
0x400), `Play_Antaran_Win_Flic_` 0xc673d (E 0xb7 with 0x400, then E
0x136), `Play_Diplomatic_Win_Flic_` 0xc66bd (E 0x22c, then E 0x136); the
arrow cursor after.

## Callers

| Site | Routine | When | Port |
|---|---|---|---|
| 0x10164 | main2_ → `Draw_Logos_` | start-up unless 0x19a004 (a command line switch of the 1.40 patch) | `Options.Logos`; `cmd/openorion2 -logos on/off/auto` (auto: with a window and no `-load`) |
| 0x13905 | `Next_Turn_` | no living human (after a fast fade out and black pages), or the council elected someone else | `checkEndOfGame` |
| 0x1393b | `Next_Turn_` | one empire left (after `Fast_Fade_Out_`) | `checkEndOfGame` |
| 0x13992 | `Next_Turn_` | a player took Antares | `checkEndOfGame` |
| 0x1673e | `Council_Accept_Ruling_` | the human elected, after the cheers | `turn_council.go` |
| 0x47980 | `Tactical_Combat_` | a battle at Antares (star = NUM_STARS) | `tacticalCombat` |
| 0xe8619, 0xe8aef | `Bomb_Results_Popups_`, `Do_Attacker_Beat_Colony_Stuff_` | stellar converter | `playPlanetaryDestructionFlic` (the second; the first's popup is not ported) |
| 0xe891d | `Do_Attacker_Beat_Colony_Stuff_` | the amoeba hit a human colony (before `Colony_Bombing_Screen_`, not ported) | `turn_combatcolony.go` |
| 0xe8633 | `Bomb_Results_Popups_` | the same for the bombing popup | not ported (its caller) |
| 0xe96a4, 0xe9780 | `Do_1_Combat_` | a human took Orion, single player or tactical; then `Rescued_Loknar_Popup_` (not ported) | `takeOrionFlic` |
| 0xe85cc | `Strategic_Results_Popup_` | the same in strategic combat | not ported (its caller) |
| 0xfe621 | `Antarans_` (report) | the Antarans set out and do not come for the human's colony (else the Antaran room screen) | `antaransReport` |
| 0xfc66f | `Net_Next_Turn_` | network | not ported |

Input: any key (read and dropped) or a held button ends a video; nothing
else changes state, apart from the volume bytes above.

## Draw_Logos_ (0x24ed3)

Empty mouse list (0x180ebc), `_logo_fade` (0x19acf4) = 0, `Clear_Fields`,
stencil refresh, black off page. If `Check_For_Simtex_Logo_` (DIR of
SIMTEX.LBX, also under the alternate path) finds it, `Play_Cinematic_`
(SIMTEX.LBX, 0, 0) instead of the first logo. Else LOGO.LBX 0 is opened,
`Darken_Palette_(100)`, `Clear_Palette_Changes`, `Toggle_Pages_`, its
first frame (palette chunk) drawn, `Copy_Off_To_Back`, `Toggle_Pages_`,
`Slow_Fade_In_` (0x251ef: darken 100 → 0 one percent a step; a key or a
button stops it, `_logo_fade` = the level reached and the result 1). Then
one frame per tick (`Mark_Time_` / `Release_Time_(1)`): a key ends it
after this pass, a button at once; on the last frame (`Get_File_Animation_
Total_Frames_` = frames − 1) it counts 15 passes down, drawing nothing.
Unless stopped: `Logo_Fade_Out_` (0x2518f: darken from `_logo_fade` to
100 by 10, black pages, `_TOGGLE_PAGES_`), LOGO.LBX 1 the same way with
`Fast_Fade_In_` (a no-op at fade percent 0: the logo shows when its
palette chunk reaches the DAC), `Draw_File_Animation_Stencil_`, 5 passes
on the last frame. Then `Logo_Fade_Out_`, black off and back pages, and
unless a key or button stopped a logo, `Play_Cinematic_(INTRO.LBX, 0, 1)`.
main2_ then loads palette 5 and the arrow cursor.
