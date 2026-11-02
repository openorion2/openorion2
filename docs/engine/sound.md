# Sound and music

Port: `internal/sound` (WAV), `internal/audio` (mixer), `internal/host`
(`speaker.go`: SDL3 output), `internal/engine/sound.go` (sound.c),
`internal/game/sound.go` (MOX.C music, `Mox_Play_Sound_`). Formats:
`docs/formats/sound.md`. Bit-exact audio is not the goal (DOSBox cannot
check it); what is checked is which sound or music starts when.

**State and RNG.** No sound routine calls `Random_` or writes game
state. `Play_Ship_Explosion_SFX_` (0xaec0b) draws `Random_(3)` to pick
its sound — that draw was always ported; it now also plays. The music
routines pick tracks with `clock_()` (hundredths of a second; virtual
time in headless runs). The only waits are loops that count
`Mox_Update_` calls while a fade runs (`Sound_Update_` takes one step
per call), so they end without time passing. The game popup's sliders
write settings +0x11..+0x14 (already ported; they now also switch the
sound).

## Engine (sound.c, F:\WLIBS\sound)

- `Play_Sound_(n)` (0x112399): returns 0 when sound is off, n is 0, no
  handle (1..16, `getFreeSampleHandle_`) is free or the entry fails;
  else plays SOUND.LBX entry n (a cache of 0x21 loaded entries) at
  volume `0x1843a4` and pan `0x1843a5` and returns the handle.
  `Get_Input_` plays the selected field's sound this way (0x11724b).
- `Fadeout_Sound_(h, step)` (0x112c96): handles 1..16; `Sound_Update_`
  (0x1131f0, from `Mox_Update_`) lowers the volume by step a call and
  stops it at 0. `Pitch_Bend_Sound_(h, d)`: playback rate + d (≥ 0).
  `Loop_Sound_(h, n)`: n more passes (−1 → 0, AIL's "for ever").
- `Set_Sound_Max_Volume_(pct)`: 0x1843a7 = pct·127/100.
- Streams (music): `Set_Stream_Filename_`, `Play_Stream_(n)` on its own
  handle (0x184447), `Set_Stream_Volume_(pct)` (capped by
  `Set_Stream_Max_Volume_`, ·127/100), `Get_Stream_Volume_` (·100/127, 0
  without a stream), `Fade_Stream_Down_(step)` = `Fadeout_Sound_` on the
  stream's handle, `Fade_Stream_Up_(step)` (+step % a `Sound_Update_` up
  to the maximum), `Loop_Stream_`, `Stop_Stream_`, `Stream_On_/Off_`.

## Game

- `Mox_Play_Sound_(n)` (0x147d7): `Play_Sound_(n)` if settings +0x11
  (sound effects on), else 0.
- `Set_Sound_Volume_To_Settings_` (0x7a06c): +0x13 music on → `Stream_On_`
  else `Stream_Off_`; stream maximum and volume = min(+0x14, 100); +0x11
  off → `Stop_All_Sounds_`, else `Sound_On_`; `Set_Sound_Max_Volume_(+0x12)`.
  Called by main2_ (0x1014a) and `Load_Game_Settings_` (0x11c79, not yet).
- Music globals 0x180eb4 current track, 0x180eb6 state, 0x180eb8 wanted,
  0x180eba next (all −1 at start).
- `Play_Background_Music_` (0x2484f): music on and STREAM 1..3 not
  playing: fade down by 1 and wait (`Mox_Update_` each pass), stop,
  STREAM `clock_()%3+1`, looped, volume 0, fade up by 30.
  `Play_Combat_Music_` (0x2496c): the same with STREAM `clock_()%3+4`.
  `Fade_Down_Background_Music_` (0x2490d): the wait calls `Mox_Update_`
  every 18th pass; then stop. `Stop_Background_Music_` (0x24831).
- `Play_Streaming_Music_(n, next, loop)` (0x24677), each pass of a screen:
  a new n fades the old track down (state 1 → 2); once silent, n starts
  (STREAM ≤ 100, STREAMHD n−100) faded up (state 3, or 4 when next is −1,
  looped if loop); when it ends, next (−2: STREAM `clock_()%3+1`) follows.

## Call sites

| Routine | Site | Sound | Port |
|---|---|---|---|
| main2_ | 0x1014a, 0x10193 | settings; background music | `newGame`, `Run` |
| Load_Game_Settings_ | 0x11c79 | Set_Sound_Volume_To_Settings_ | `loadGameSettings` |
| Play_Cinematic_ | 0x14e14, 0x14e8f, 0x1505d | fade down 0x19, stop, background music | yes |
| Draw_Logos_ | 0x2504f | stop stream after SIMTEX | yes |
| Do_Main_Menu_File_Animation_ | 0x80fe3, 0x810b8 | 0x35 at frame 5, 0x36 at frame 0x2e and on an early stop | yes |
| Get_Input_ | 0x11724b | the field's button sound | yes (engine) |
| Check_Ships_XY_, Check_Stars_XY_ | 0x82c99, 0x82d64 | 0x29 | yes |
| Tactical_Combat_ | 0x47e18, 0x48e51, 0x48e5b | combat music; Fadeout_Sound_(-1) (no-op); background music | `combatScreenLoad/Close` |
| Destroy_Ship_FX_ → Play_Ship_Explosion_SFX_ | 0xaec3f, 0xaec50 | size+Random(3)−1: 0 → 0x1d, 5 → 0x1e+0x2b, else 0x18 | yes |
| Beam_SFX_ | 0xa8ea7, 0xa8f8c, 0xa8ff0 | 0x1d ship hit, 0x1c missile hit; the handles forgotten at the end | yes |
| Fire_Weapon_ | 0xa8137, 0xa817f | the beam's sound on the mount's first frame (frame = mount or 0), handle per side and mount (0x182443); faded when the side stops firing or the frames run out | yes |
| Draw_Shield_Hit_, Draw_Reflection_Field_Hit_ | 0xa9c0f, 0xaa41e | 0x26 (0x2a with a reflection), pitch −2500·hull (planet −15000), faded on the last frame (the shield also when its display went off) | yes |
| Missile_Launch_FX_ | 0xaf40e.. | weapon 0xe: 0x20 +500; 0x10/0x11: 0x20 −1500/−2500; 0x12..0x14: 0x3d, 0x3c, 0x3b; else 0x20 (not in super fast) | `missileLaunchSound` |
| Start/End_Ship_Engine_SFX_ (Move_Ship_) | 0x3eea2, 0x3f53e | 0x1f looped while moving, faded after (not in super fast) | `moveShipStart/End` |
| Do_Spherical_SFX_ | 0xadec5.. | kinds 0..2 0x14; 3 0x2f, 8 and on Random(10)=1 0x21; 4 0x19, 8 | yes |
| Draw_Retreating_Ship_, Draw_Teleporting_Ship_ | 0x3302f, 0x3e6cd | 0x34, 0x37, pitch −2500·(hull−2); the teleport's faded | yes |
| Draw_Bombs_, Anti_Missile_Rockets_ | 0x341d9, 0xabb66 | 0x18 (faded after the messages), 0x1c | yes |
| Resolve_Raid_, Resolve_Capture_ | 0x37c73, 0x382c9 | 0x43, faded after the report | yes |
| Gyro_Destablizer_, Draw_Gyro_Special_Effect_, Lightning_Field_, Draw_Stasis_Beam_, Draw_Cloak_, Plasma_Web_, Draw_Transporter_Bomb_Beam_ | 0xac28d, 0xabd6e/0xabe9d, 0xac57e, 0xac9c7/0xacac6, 0xacc28, 0xad0e5/0xad24a, 0xad85d | 3, 0xb ×2, 4, 0xa ×2, 0xa (never faded), 0x1b then 0x44, 0x14; each faded after its animation | yes |
| BHG_, Stellar_Converter_ | 0x3b753, 0xadae5 | 0xa, 0x14, each faded by 10 after its beam | yes (BHG: tcbh1; the converter: tcsc2) |
| Do_Unit_Turn_, Pick_Next_Action_, Play_Unit_Beam_Sound_, Move_Is_Legal_, Do_Legal_Move_Check_ (ground battle) | 0xb80a2.., 0xb83e1, 0xb935d | hit 0x1c (0x1e on a dying trooper) halfway, the shot by weapon (0x49 → 9, 0x65 → 1, 0x80 → 0x13, 0x8a → 0xd, 0x91 → 0xc; others play the caller's EBX — the port none), faded at frame 10; footsteps 0x1f, faded | yes |
| Colony_Combat_Screen_ | 0xb764b, 0xb76db, 0xb76e0 | theme 0xa each pass; background music, Stop_All_Sounds_ | yes |
| Colony_Combat_Popup_ | 0xcbea2 | Stop_All_Sounds_ | yes |
| Draw_Colony_Landing_Screen_, Colony_Landing_Screen_ | 0xb94c3.., 0xb989e.. | by frame: 1, 2 → 0x3a; 0x39, 0x3e → fade the matching 0x3a by 2, 0x39; all four faded at the end | yes |
| Evaluate_Screen_Input_ (colony) | 0xc013e | 0x33 when the cheats toggle | yes |
| Apply_Next_Bomb_Hit_ | 0xca9df, 0xcaa7f | 0x18 per explosion, 0x20 per biological hit | yes |
| Strategic_Results_Popup_ | 0xe85a7 | 0x33 | caller not ported |
| Set_Sound_For_Game_Popup_ | 0x80918.. | volume, Sound_On_, test 0x21; clears _global_var | yes |
| Set_Music_For_Game_Popup_ | 0x80892.. | Stream_On_ + background music, or Stream_Off_ + stop; stream maximum and volume | yes |
| Main_Antaran_Room_Screen_ (+ draw) | 0x14ae9, 0x14c7a, 0x14bc9 | theme 0x78; background music on leaving | yes |
| Main_Council_Screen_ (+ draws) | 0x1526d, 0x156c3, 0x16bed, 0x1535d | theme 0x77; background music | yes |
| Diplomacy (Draw_Diplomacy_Synch_Mode_, Diplomacy_Fade_In_, Draw_Diplomacy_Fade_In_Screen_) | 0x171df, 0x17cec, 0x17d3f | theme = current music (good/bad by race) + 100 | yes |
| Diplomacy_Screen_, Npc_Display_Diplomacy_Messages_ | 0x16ea0, 0x1ab6e, 0x1abac | _jim_current_stream = −1 and background music; fade down 10 | yes |
| Start_Main_Event_, Draw_Event_Screen_, Check_For_Event_ | 0x2035b, 0x2052f, 0x202ef | theme 0x76; background music | yes |
| Design_Screen_ (+ Draw_Design_Screen_, Draw_Generic_Replacement_Box_) | 0x6bb31.., 0x6c6fc, 0x65849, 0x6c076 | theme 8; background music on leaving (unless to screen 3) | yes |
| Science_Room_, _Tech_Select_ | 0x10d5fb, 0x10d693, 0x10da4a, 0x10e297 | theme 0x75 then random STREAM 1..3 | yes |
| Show_Off_Stolen/Captured_Tech_, Tech_Select_ | 0x10d0ca, 0x10d15d, 0x10dbc7 | background music | yes |
| Scientist_With_Tech_ | 0x10d1f2 | background music | caller not ported |
| Fade_Down_Background_Music_ | Jim_Hot_Key_ 0x2466e | | not ported (debug hot key) |
| Start_Diplomacy_Music_ | 0x1d0d5 | no direct caller | — |

Smacker audio tracks: decoded and mixed as stream voices (docs/screens/cinematics.md, "Audio").
