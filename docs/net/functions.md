# Network game: function inventory

Every routine of the
network game layer (modules `NETMOX.C`, `NETSTART.C`, `NETTURN.C` and the
network screens of module 111), the `Net_*` API under it, and every routine
of the ported game code that branches on `_game_type` (0x199f3a) being 2 or 3,
reads `_settings`+0x16 (0x199bf2, the connection picked) or calls into the
network layer. How the pieces work together is in `overview.md`.

Sources: `build/orion2.dis` (calls, compares), the Watcom debug information (modules, typed prototypes). The tables were
made by a script that, for each routine, collects the `cmp byte ptr
[0x199f3a], N` tests (also `mov al, [0x199f3a]` followed by `cmp al, N`), the
calls into the four network modules and the callers, then greps
`internal/` for the routine name. Rerun it after a phase to update the
"Port" column (the script is described at the end).

Columns:

- **Size**: bytes up to the next symbol.
- **Tests**: `gt==N` is a compare of `_game_type` with N; `set` writes it;
  `conn` reads `_settings`+0x16.
- **Ph**: the part of network play it belongs to (1 transport and API, 2
  lobby and start, 3 turn, 4 combat, 5 diplomacy, 6 leaving, loading, end).
- **Port**: `—` not ported; `log` the port reaches the routine but logs
  "not ported" on the network branch; `skip` ported for game types 0/1, the
  network branch silently does nothing or differs; `ok` the network branch is
  ported (data only, nothing to send).

Not listed: the transport under `Net_*` (IPX in `net.c`/`transprt.c`/
`comcore.c`, the modem in `simmodem.c`/`motrans.c`/`MODEM.C`, Greenleaf,
`ComSend*`, `SendPDU`, keepalives), and the modem file helpers
(`fseek/fgetc/fread/fwrite_modem_file_`, `Open/Close/Write_Modem_File_`
0xf6f90–0xf74cd: the broadcast diff's previous buffer, kept in memory in the
port). `IPX_Present_` (0x15dcce), `EnterNet_` and `Get_Network_Address_` are
transport.

## The Net API (net/netcode.c, `Net_*`)

The port reimplements these over TCP (phase 1, done: `internal/game/net.go`
over `internal/netplay`, `transport.md`); everything above them is ported
from the assembly. Prototypes are from the Watcom debug information
(register order EAX, EDX, EBX, ECX, then the stack).

| Address | Routine | Prototype | Callers |
|---|---|---|---|
| 0x138f3b | `Net_Init_` | `int (int type, int gameID, void (*receiveMessage)(u16 from, u16 type, u16 size, u8 *data), s_modem_comm *modem)` | Join/Load/Start_Net_Screen_, Russ_Net_Init_ |
| 0x139062 | `Net_Uninit_` | `void ()` | Choose_Multi_Network_Game_Screen_, Do_Answer_, Do_Dial_, Do_Main_Game_Popup_, Join/Load/Start_Net_Screen_, Mox_Net_Uninit_, Reset_System_ |
| 0x1390bb | `Net_Update_` | `void ()`: pumps the transport; a `reentrant` guard (0x18466c) | Basic_Update_, Mox_Net_Update_, Mox_Send_Message_, Net_Send_Message_ |
| 0x139127 | `Net_Is_Sending_` | `int ()`: returns 0x1849a0 | Mox_Send_Message_, Mox_Net_Update_, Net_Next_Turn_, every Send_* that chunks |
| 0x139153 | `Net_Send_Message_` | `int (int dest, u16 userType, void *data, u16 dataSize)`: waits while sending, then `ComSendUserMessage_`; 0 = failure | Mox_Send_Message_ |
| 0x13920c | `Net_Status_` | `int ()` | (none found) |
| 0x139294 | `Net_Get_Users_` | `int (char *userList)` | Draw_Wait_For_New_Joiners_Screen_, Draw_Wait_For_Race_Info_Screen_, Dump_Users_, Load/Start_Net_Screen_ |
| 0x1393b6 | `Net_Is_User_Valid_` | `int (int id)` | (none found) |
| 0x139447 | `Net_Host_ID_` | `int ()`: own station id | Join/Load/Start_Net_Screen_ |
| 0x139476 | `Net_Update_Game_List_` | `int ()` | the four game-list calls below |
| 0x139659 | `Net_Get_Game_Count_` | `int ()` | Add/Draw_Choose_Multi_Net_Game_*, Choose_Multi_Network_Game_Screen_, Change_MP_Game_Name_ |
| 0x13968a | `Net_Get_Game_Name_` | `char *(int game)` | same |
| 0x1396f8 | `Net_Get_Game_Status_` | `int (int game)` | (none found) |
| 0x13974f | `Net_Join_Game_` | `int (char *gameName, char *password, char *playerName)` | Join_Net_Screen_ |
| 0x13982d | `Net_Create_Game_` | `int (int gameId, char *gameName, int numPlayers, char *password, char *playerName)` | Russ_Net_Create_Game_ |
| 0x1398ea | `Net_Get_Player_Count_` | `int ()` | (none found) |
| 0x13994e | `Net_Enable_New_Players_` | `void ()` | (none found) |
| 0x13998f | `Net_Disable_New_Players_` | `void ()` | Load/Start_Net_Screen_ |
| 0x1399d0 | `Net_New_Players_Enabled_` | `int ()` | (none found) |
| 0x139358 | `Set_Null_Modem_Slave_` | `void ()` | Join_Net_Screen_ (null modem only: drop) |
| 0x139387 | `Set_Null_Modem_Host_` | `void ()` | Load/Start_Net_Screen_ (null modem only: drop) |

The receive callback is `Receive_Message_` (0xf67c1), passed as a function
pointer to `Net_Init_` at 0xfaf5f, 0xfb2f5 and 0xfb868.

## Message plumbing (NETMOX.C, phase 1)

| Address | Routine | Size | Callers | Purpose | Port |
|---|---|---|---|---|---|
| 0xf67c1 | `Receive_Message_` | 85 | the transport; Mox_Send_Message_ (to self) | `(u16 from, u16 id, u16 len, u8 *buf)`: sets 0x1ab0df, routes by id (≥ 0x7d0 `Diplomacy_Receive_Message_`, ≥ 0x3e8 `Tactical_Combat_Receive_Message_`, else `Main_Receive_Message_`) | ok |
| 0xf6816 | `Mox_Send_Message_` | 488 | 87 call sites | `(int to, u16 id, void *buf, u16 len)`: len 0 sends one stack byte; to self → `Receive_Message_`; 0x7ffe → every other human (just one: rewritten to him; none: nothing); a failed send → `Exit_With_Message_` | ok |
| 0xf5a9f | `Main_Receive_Message_` | 3362 | Receive_Message_ | the handlers of ids < 1000 (jump tables 0xf594c, 0xf598f) | dispatch ok; every case |
| 0xf69fe | `Mox_Net_Update_` | 1426 | Mox_Update_ | the pump: `Net_Update_`, the host vote resend (msg 4), the msg 0x22 rebroadcast, `Do_Net_Combat_Update_`; tests gt==2/3 | ok |
| 0xf5939 | `Basic_Update_` | 358 | (by pointer) | `Sound_Update_` and the guarded `Net_Update_` (the bytes after it are the id table) | ok |
| 0xf5934 | `Net_Exit_With_Message_` | 5 | (by pointer) | jumps to `Exit_With_Message_` | ok (`ExitError`) |
| 0xfa16c | `Net_Id_To_Player_Id_` | 55 | Load_Net_Screen_, Receive_Message_ | net id → player index (player+0x2b) | ok |
| 0xfe8be | `Mox_Update_` | 10 | 150 sites | calls `Mox_Net_Update_` | ok (engine `MoxUpdate`, `NetUpdate` hook) |
| 0xf5883 | `Load_Netmox_Strings_` | 177 | Load_Russ_Strings_ | the module's strings | ok (string 154, `_net_send_error`, from RSTRING) |
| 0xfecda | `Load_Russ_Strings_` | 101 | main2_, Reload_Language_Strings_, Load_Mox_Global_Strings_ | loads the three modules' strings | — |
| 0x113dbd | `Reset_System_` | 75 | Exit_With_Message_ and the allocation errors | calls `Net_Uninit_` on exit | network part (`resetSystem`) |

## Lobby and game start (phase 2)

NETSTART.C and the network screens of module 111 (`MULTI*`).

| Address | Routine | Size | Callers | Purpose | Port |
|---|---|---|---|---|---|
| 0xf4d99 | `Multi_Player_Screen_` | 1137 | Screen_Control_ | the multiplayer screen (conn, gt==1) | ok (modem radios disabled, port-only) |
| 0xf5691 | `Set_Multi_Player_Game_Type_` | 230 | Add_MP_Setup_Screen_Fields_ | `_game_type` from `_settings`+0x16 | ok |
| 0xf009a | `Add_MP_Setup_Screen_Fields_` | 856 | Multi_Player_Screen_ | | ok |
| 0xf2082 | `Draw_MP_Setup_Screen_` | 304 | Multi_Player_Screen_ | | ok |
| 0xf21b2 | `Draw_MP_Buttons_Down_` | 291 | Draw_MP_Setup_Screen_ | | ok |
| 0xf42ca | `Load_MP_Setup_Pictures_` | 497 | Multi_Player_Screen_ | | ok |
| 0xf45db | `Copy_Modem_Defaults_To_Net_Struct_` | 268 | Load_MP_Setup_Pictures_ | | ok |
| 0x6fad5 | `Set_Multi_Player_Setup_Screen_Help_List_` | 21 | Multi_Player_Screen_ | | ok |
| 0xfaa57 | `Russ_Net_Init_` | 26 | (none found) | `Net_Init_` wrapper | — |
| 0xfaa71 | `Russ_Net_Create_Game_` | 35 | Load/Start_Net_Screen_ | `Net_Create_Game_` wrapper | ok |
| 0xfb7e5 | `Start_Net_Screen_` | 1680 | Screen_Control_ | creator: init, create, wait for joiners, msg 0, race info, `Init_New_Game_`, full broadcast | ok |
| 0xfaedb | `Join_Net_Screen_` | 791 | Screen_Control_ | joiner: init, game list, join, then `Net_Select_Race_` or `Net_Pick_Position_` | ok |
| 0xfaad8 | `Net_Select_Race_` | 429 | Join_Net_Screen_ | joiner's race pick (msg 1), waits for the first broadcast | ok |
| 0xfbee1 | `Request_Race_` | 149 | Race_Selection_Screen_ | msg 0x3a/0x3e | ok |
| 0xfbf76 | `Unrequest_Race_` | 57 | Race_Selection_Screen_ | msg 0x3f | ok |
| 0xfbe75 | `Request_Banner_Color_` | 108 | Flag_Screen_ | msg 0x2c | ok |
| 0xfae80 | `Do_Answer_` | 91 | Join_Net_Screen_ | modem answer (drop) | drop |
| 0xfb1f2 | `Do_Dial_` | 137 | Load/Start_Net_Screen_ | modem dial (drop) | drop |
| 0xfaac3 | `Dump_Users_` | 21 | (none found) | debug | — |
| 0xf0c8e | `Choose_Multi_Network_Game_Screen_` | 393 | Join_Net_Screen_ | the game list | ok (refreshed every 60 frames, port-only) |
| 0xeff87 | `Add_Choose_Multi_Net_Game_Fields_` | 275 | Choose_Multi_Network_Game_Screen_ | | ok |
| 0xf1af4 | `Draw_Choose_Multi_Net_Game_Screen_` | 498 | Choose_Multi_Network_Game_Screen_ | | ok |
| 0xf40d3 | `Load_Choose_Multi_Net_Game_Screen_` | 218 | Choose_Multi_Network_Game_Screen_ | | ok |
| 0xf0b6d | `Valid_Net_Games_` | 31 | (none found) | | — |
| 0xf5777 | `Change_MP_Game_Name_` | 268 | Load_Multi_Player_Game_Screen_ | makes the game name unique in the list | ok |
| 0xf19c7 | `Draw_Join_Net_Screen_` | 301 | Join/Load/Start_Net_Screen_, Net_Select_Race_ | | ok |
| 0xf54be | `Reload_Initializing_Net_Info_` | 17 | Join/Load/Start_Net_Screen_ | | ok |
| 0xf54cf | `Reload_Join_Net_Screen_` | 10 | Join_Net_Screen_ | | ok |
| 0xf53d7 | `Reload_Generic_Net_Info_` | 201 | the Reload_* below | | ok |
| 0xf2c8b | `Draw_SendGet_Net_Info_Screen_` | 346 | Load/Start_Net_Screen_, Net_Pick_Position_, Net_Select_Race_ | "sending/getting data" | ok |
| 0xf2de5 | `Draw_Wait_For_New_Joiners_Screen_` | 361 | Load/Start_Net_Screen_ | lists `Net_Get_Users_` | ok |
| 0xf2f4e | `Draw_Wait_For_Race_Info_Screen_` | 363 | Start_Net_Screen_ | | ok |
| 0xf27b7 | `Draw_Multi_Player_Banner_` | 253 | Draw_Choose_Net_Plyrs_Screen_, Draw_Net_Next_Turn_Screen_, Hotseat_Draw_Next_Player_ | | ok (hotseat) |
| 0xcd435 | `Newgame_Screen_` | 1200 | Screen_Control_ | gt==0..3: the network game name field | ok |
| 0xccd3c | `Draw_Newgame_Screen_` | 232 | Newgame_Screen_ | gt==2: the game name | skip |
| 0xcce2e | `Add_Fields_` (NEWGAME) | 1331 | Newgame_Screen_ | gt==2: the game name field | ok |
| 0x12479 | `Init_New_Game_` | 801 | Hotseat_Screen_, Start_Net_Screen_, main2_ | gt==2: keeps `PLAYER_NUM` | ok |
| 0x5c510 | `Race_Selection_Screen_` | 2599 | Hotseat_Screen_, Net_Select_Race_, Newgame_Screen_, Start_Net_Screen_ | calls Request/Unrequest_Race_ (gt ≥ 2) | ok (the saved custom race logs: it requests race 13, `leaving.md`) |
| 0x5d03c | `Flag_Screen_` | 567 | Race_Selection_Screen_, Racial_Option_Screen_ | calls Request_Banner_Color_ | ok |
| 0x1049b | `Screen_Control_` | 738 | main2_ | dispatches the network screens: Start/Join/Load_Net_Screen_, Net_Next_Turn_ | ok |

## The turn (phase 3)

| Address | Routine | Size | Callers | Purpose | Port |
|---|---|---|---|---|---|
| 0xfc470 | `Net_Next_Turn_` | 565 | Screen_Control_ | the election (msg 4), then host or client; sets `_game_type` 0 when alone | ok (`turn.md`) |
| 0xfc6a5 | `Net_Init_Host_Election_` | 143 | Host_Next_Turn_, Join/Load/Start_Net_Screen_, Main_Receive_Message_ | resets the votes | ok |
| 0xfbfe2 | `Host_Next_Turn_` | 695 | Net_Next_Turn_ | msg 0xc one client at a time, `Decode_Game_Orders_`, `Next_Turn_`, broadcast, msg 0x3b | ok |
| 0xfc2d2 | `Client_Next_Turn_` | 414 | Net_Next_Turn_ | msg 0xb, wait for 0xc, upload, `Client_Combat_Loop_`, decode the broadcast | ok |
| 0xfc299 | `Must_Broadcast_AI_Player_Data_` | 57 | Client_Next_Turn_ | pending msg 0x22 | ok |
| 0xf7e95 | `Send_Game_Orders_` | 1314 | Client_Next_Turn_ | the upload, msgs 0xd/0xe | ok |
| 0xf83d8 | `Decode_Game_Orders_` | 1280 | Host_Next_Turn_ | applies an upload | ok |
| 0xf83b7 | `Ditch_Leader_` | 33 | (none found by call) | frees a leader | ok |
| 0xf74cd | `Broadcast_Game_Data_Differential_` | 1321 | Host_Next_Turn_, Load/Start_Net_Screen_ | msgs 2/3, full or diff | ok (`broadcast.md`) |
| 0xf79f6 | `Decode_Broadcast_Game_Data_Differential_` | 1183 | Client_Next_Turn_, Net_Pick_Position_, Net_Select_Race_ | applies it, `Set_Random_Seed_` | ok |
| 0xf70eb | `Make_Difference_Buffer_` | 486 | Broadcast_Game_Data_Differential_ | byte-run diff | ok |
| 0xf72d1 | `Decode_Difference_Buffer_` | 241 | Decode_Broadcast_Game_Data_Differential_ | | ok |
| 0xfd7ea | `Client_Initialize_Reports_` | 50 | Decode_Broadcast_Game_Data_Differential_ | | ok (netbroadcast.go) |
| 0x13870 | `Next_Turn_` | 376 | Host_Next_Turn_, Screen_Control_ | gt==2/3: no autosave, no end-of-game check here | ok |
| 0x13822 | `Do_Autosave_` | 78 | Must_Display_Reports_, Next_Turn_ | every stardate for gt 2/3 | ok |
| 0x12175 | `Make_Autosave_File_Name_` | 123 | (by pointer) | gt==3 | — |
| 0x121f0 | `Get_Multi_Player_N_Humans_` | 55 | Make_Autosave_File_Name_ | | — |
| 0xfe86b | `Must_Display_Reports_` | 82 | Hotseat_Select_Player_, Main_Screen_Report_Handler_, Reports_Screen_ | gt==2: autosave | ok |
| 0x83411 | `Do_Next_Turn_` | 317 | Do_Begin_Of_Turn_, Main_Screen_ | gt==2/3: screen `Net_Next_Turn_` | ok |
| 0x86188 | `Main_Screen_` | 5308 | Screen_Control_ | gt==2/3 branches (diplomacy request lights, done lights, help) | skip |
| 0x84555 | `Draw_Main_Screen_` | 1143 | many | gt==2/3: the net player lights | ok |
| 0x84a95 | `Draw_Main_Screen_Filled_` | 313 | many | gt==2/3 | ok |
| 0x8520c | `Draw_Player_Done_Lights_` | 276 | Draw_Main_Screen_ | gt==2/3: who has ended his turn | ok |
| 0x6f280 | `Set_Main_Screen_Help_List_` | 685 | Main_Screen_ and the box code | gt==2 help entries | ok |
| 0xffeea | `Move_All_Ships_Toward_Stars_` | 294 | Next_Turn_Calc_ | gt==2: clears star+special popup | ok |
| 0x816f2 | `Main_Menu_Screen_` | 972 | Screen_Control_ | gt==2/3 saves cannot be continued | ok |
| 0xf55a4 | `Chat_Box_Input_Loop_` | 237 | Client_Combat_Loop_, Client/Host/Net_Next_Turn_, Do_1_Combat_, Do_Attacker_Beat_Colony_Stuff_, Send_Battle_Results_, Send_Game_Orders_ +2 | the wait screen with chat (conn) | ok |
| 0xf3e42 | `Load_Net_Next_Turn_Screen_` | 388 | Bomb_Results_Popups_, Client_Combat_Loop_, Do_1_Combat_, Do_Attacker_Beat_Colony_Stuff_, Net_Next_Turn_, Strategic_Results_Popup_ | | ok |
| 0xf1075 | `Draw_Net_Next_Turn_Screen_` | 1529 | Chat_Box_Input_Loop_ | | ok |
| 0xefcea | `Add_Net_Next_Turn_Fields_` | 400 | Load_Net_Next_Turn_Screen_, Net_Next_Turn_ | | ok |
| 0xf31bb | `Get_Net_Next_Turn_Player_Colors_` | 499 | Load_Net_Next_Turn_Screen_ | | ok |
| 0xdd3b8 | `Send_Chat_Msg_` | 114 | Chat_Box_Input_Loop_ | msg 0x27 | ok |
| 0xdd351 | `Receive_Chat_Msg_` | 103 | Main_Receive_Message_ | | ok |
| 0xdd42a | `Send_GNN_Chat_Msg_` | 139 | GNN_Reports_Combat_ | msg 0x2d | ok |

## Combat (phase 4)

| Address | Routine | Size | Callers | Purpose | Port |
|---|---|---|---|---|---|
| 0xe9d62 | `Search_For_Battles_` | 1423 | Next_Turn_Calc_ | network: hands battles to `Client_Combat_Loop_`, msg 0x1a | network: state 0xe, `Client_Combat_Loop_`, msg 0x1a ok |
| 0xea2f1 | `Client_Combat_Loop_` | 1497 | Client_Next_Turn_, Search_For_Battles_ | the per-machine battle loop | ok (`combat.md`) |
| 0xeaaa1 | `Do_Net_Combat_Update_` | 1777 | Mox_Net_Update_ | the host's slot machine (0x1aaf8b) | ok |
| 0xe84a5 | `Pick_Star_And_Attacker_` | 248 | Do_Net_Combat_Update_, Search_For_Battles_ | gt==2/3 | ok (`Some_Star_Player_Is_Busy_` 0xe841a) |
| 0xe938c | `Do_1_Combat_` | 1245 | Client_Combat_Loop_, Do_Net_Combat_Update_, Do_Nonhuman_Attack_Against_Human_, Search_For_Battles_ | gt==0..2 | ok |
| 0xe7343 | `Russ_Combat_` | 821 | Do_1_Combat_ | msgs 0x40/0x41 around `Tactical_Combat_` | ok |
| 0xe87d2 | `Do_Attacker_Beat_Colony_Stuff_` | 2950 | Do_1_Combat_ | gt==2/3; Russ_Change_Relations_, Russ_Event_Monster_Update_ | ok |
| 0xea91b | `Do_Nonhuman_Attack_Against_Human_` | 390 | Do_Net_Combat_Update_ | host side of AI/monster vs human | ok |
| 0xf88d8 | `Send_Pick_Target_Info_` | 950 | Do_Net_Combat_Update_ | msgs 0x10 / 0x2e, 0x2f | ok |
| 0xf8c8e | `Decode_Pick_Target_Info_` | 778 | Client_Combat_Loop_, Main_Receive_Message_ | | ok |
| 0xf8f98 | `Send_You_Are_Target_Info_` | 1099 | Do_Net_Combat_Update_, Do_Nonhuman_Attack_Against_Human_ | msgs 0x12 / 0x34, 0x35 | ok |
| 0xf93e3 | `Decode_You_Are_Target_Info_` | 814 | Client_Combat_Loop_, Main_Receive_Message_ | | ok |
| 0xf97cd | `Send_Battle_Results_` | 1163 | Client_Combat_Loop_ | msgs 0x15, 0x30..0x33 | ok |
| 0xf9c58 | `Decode_Battle_Results_` | 1029 | Client_Combat_Loop_, Main_Receive_Message_ | | ok |
| 0xf9711 | `Russ_Event_Monster_Update_` | 75 | Do_Attacker_Beat_Colony_Stuff_, Process_Retreating_Ships_, Strategic_Combat_, Tactical_Combat_ | gt==2/3: msg 0x3c | ok |
| 0xf975c | `Russ_Change_Relations_` | 113 | Apply_Bio_Weapon_Penalty_, Do_Attacker_Beat_Colony_Stuff_, End_Of_Combat_, Strategic_Combat_ | gt==2/3: msg 0x43 | ok |
| 0xfa104 | `Send_Dragon_Info_` | 62 | Do_Net_Combat_Update_ | msg 0x23 | ok |
| 0xfa142 | `Decode_Dragon_Info_` | 42 | (by Main_Receive_Message_'s table) | | ok |
| 0x40148 | `Strategic_Combat_` | 2786 | Russ_Combat_ | calls the two Russ_ senders | ok (Russ_ senders) |
| 0x47939 | `Tactical_Combat_` | 5542 | Russ_Combat_ | network setup, `Mox_Send_Message_` | ok (`_net_flag`, msg 0x3ed) |
| 0x42b0f | `Send_Tactical_Combat_Message_` | 97 | Do_Combat_Turn_, Initiate_Boarding_Action_, Resolve_Ai_Boarding_ | msg 0x3e8 with the seed | ok |
| 0x3fcd8 | `Tactical_Combat_Receive_Message_` | | Receive_Message_ | ids 0x3e8..0x7cf | ok |
| 0x42f7f | `Do_Combat_Turn_` | 7973 | Tactical_Combat_ | the network branches (remote side's actions) | ok |
| 0x455a4 | `Next_Ship_` | 680 | Do_Combat_Turn_ | sends a pass | ok |
| 0x3fbfe | `Wait_Until_Net_Opponent_Finished_` | 218 | Do_Combat_Turn_, Init_Ship_For_Start_Of_Turn_, Initiate_Boarding_Action_, Next_Ship_, Resolve_Ai_Boarding_, Tactical_Combat_ | msg 0x3ec | ok |
| 0x29616 | `Toggle_Auto_Combat_Flags_` | 378 | Do_Auto_Ship_Turn_, Do_Combat_Turn_ | msg 0x3ea | ok |
| 0x4b184 | `End_Of_Combat_` | 2141 | Tactical_Combat_ | Russ_Change_Relations_ | ok (Russ_Change_Relations_) |
| 0x4b11e | `Apply_Bio_Weapon_Penalty_` | 102 | Do_Attacker_Beat_Colony_Stuff_, End_Of_Combat_ | Russ_Change_Relations_ | ok |
| 0xe6e52 | `Process_Retreating_Ships_` | 1201 | Lose_Out_Of_Range_Ships_, Search_For_Battles_ | Russ_Event_Monster_Update_ | ok |

## Diplomacy (phase 5)

| Address | Routine | Size | Callers | Purpose | Port |
|---|---|---|---|---|---|
| 0xfa1a3 | `Get_Next_AI_Talker_` | 205 | Main_Receive_Message_ | the arbiter's queue | ok |
| 0xfa270 | `Net_Request_AI_Diplomacy_` | 57 | Has_Diplomacy_Messages_, Race_Screen_ | msg 5 | ok |
| 0xfa2a9 | `Net_May_Begin_AI_Diplomacy_` | 28 | Has_Diplomacy_Messages_, Race_Screen_ | | ok |
| 0xfa2c5 | `Net_End_AI_Diplomacy_` | 202 | Has_Diplomacy_Messages_, Race_Screen_ | msg 0xa (gt==2) | ok |
| 0xfa38f | `Net_Cancel_AI_Diplomacy_Request_` | 66 | Race_Screen_ | msg 8 | ok |
| 0xfa3d1 | `Net_Cancel_AI_Diplomacy_Confirmed_` | 11 | Race_Screen_ | | ok |
| 0xfa3dc | `No_AI_Diplomacy_Allowed_` | 26 | Race_Screen_ | | ok |
| 0xfa3f6 | `Net_Request_Human_Diplomacy_` | 299 | Race_Screen_ | msg 0x1b | ok |
| 0xfa521 | `Net_May_Begin_Human_Diplomacy_` | 42 | Race_Screen_ | | ok |
| 0xfa54b | `Allowed_To_Cancel_Human_Diplomacy_` | 11 | Race_Screen_ | | ok |
| 0xfa556 | `Net_Cancel_Human_Diplomacy_Request_` | 77 | Accept_Diplomacy_Request_, Race_Screen_ | msg 0x1f | ok |
| 0xfa5a3 | `Human_Hit_Next_Turn_` | 7 | Race_Screen_ | | ok |
| 0xfa5aa | `Net_End_Human_Diplomacy_` | 15 | Race_Screen_ | | ok |
| 0xfa5b9 | `Human_Diplomat_Says_Wait_` | 13 | Race_Screen_ | | ok |
| 0xfa5c6 | `Net_Diplomacy_With_` | 8 | Draw_Race_Screen_ | | ok |
| 0xfa5ce | `Human_Requesting_Diplomacy_` | 85 | Check_For_Diplomacy_Request_ | | ok |
| 0xfa623 | `Accept_Diplomacy_Request_` | 261 | Check_For_Diplomacy_Request_ | msg 0x1c | ok |
| 0xfa728 | `Refuse_Diplomacy_Request_` | 76 | Check_For_Diplomacy_Request_ | msg 0x1d | ok |
| 0xfa774 | `Tell_Diplomacy_Requester_To_Wait_` | 33 | Check_For_Diplomacy_Request_ | | ok |
| 0xfa795 | `Humans_Requesting_Diplomacy_` | 6 | Draw_Diplomacy_Request_Lights_ | | ok |
| 0xfa79b | `Net_Human_Rejected_Diplomacy_` | 21 | Race_Screen_ | | ok |
| 0x8250c | `Check_For_Diplomacy_Request_` | 156 | Main_Screen_ | the accept / refuse / wait box | ok |
| 0x83d06 | `Draw_Diplomacy_Request_Lights_` | 228 | Draw_Main_Main_Screen_, Draw_Main_Screen_ | | ok |
| 0x10acba | `Race_Screen_` | 3459 | Screen_Control_ | gt==2/3: requests to talk, cancels, the `_net_state` | ok |
| 0x10c8e0 | `Draw_Race_Screen_` | 767 | Race_Screen_ | Net_Diplomacy_With_ | ok |
| 0xfe408 | `Has_Diplomacy_Messages_` | 268 | Display_Report_Aux_ | gt==2/3: AI diplomacy goes through the arbiter | ok |
| 0x1f230 | `Diplomacy_Receive_Message_` | | Receive_Message_ | ids ≥ 0x7d0 | ok |
| 0x1def8 | `Get_Net_Diplomacy_Choices_` | 3615 | Diplomacy_Screen_ | the human-to-human talk | ok |
| 0x1ed17 | `Diplomacy_Net_Switcheroo_` | 133 | Get_Net_Diplomacy_Choices_ | code 0x3f3 | ok |
| 0x1ed9c | `Diplomacy_Initial_Synch_Up_` | 77 | (by pointer) | code 0x3f4 | — (by pointer only; sent inline) |
| 0x1ede9 | `Diplomacy_Listen_Mode_` | 370 | Check_Human_Treaty_Proposal_, Diplomacy_Exchange_Technology_, Diplomacy_Human_Need_Better_Offer_, Get_Human_Demand_Response_, Get_Net_Diplomacy_Choices_ | the waiting side | ok |
| 0x1ef5b | `Diplomacy_Chat_` | 353 | Diplomacy_Listen_Mode_, Diplomacy_Main_Net_Chat_ | msg 0x834 | ok |
| 0x200c3 | `Diplomacy_Main_Net_Chat_` | 159 | Get_Main_Repulsive_Diplomacy_Choices_, Get_Net_Diplomacy_Choices_ | | ok |
| 0x1f17d | `Start_Net_Treaty_` | 179 | Get_Net_Diplomacy_Choices_ | applies a treaty | ok |
| 0x17227 | `Get_Main_Repulsive_Diplomacy_Choices_` | 869 | Diplomacy_Screen_, Get_Net_Diplomacy_Choices_ | Mox_Send_Message_ | ok (net flag) |
| 0x179f4 | `Diplomacy_Declare_War_Resolution_` | 349 | Get_Main_*_Choices_, Get_Net_Diplomacy_Choices_ | | ok |
| 0x18309 | `Diplomacy_Break_Treaty_Resolution_` | 599 | Diplomacy_Break_Treaty_ | | ok |
| 0x1b06b | `Check_Human_Treaty_Proposal_` | 316 | Diplomacy_Determine_Treaty_Proposal_ | | ok |
| 0x1b5b8 | `Get_Human_Demand_Response_` | 364 | Diplomacy_Demand_ | | ok |
| 0x1b724 | `Get_Human_Gift_Response_` | 215 | Diplomacy_Offer_Gift_, Diplomacy_Offer_Money_ | | ok |
| 0x1bd5b | `Diplomacy_Human_Need_Better_Offer_` | 1724 | Diplomacy_Determine_Treaty_Proposal_ | | ok |
| 0x1cb4d | `Diplomacy_Exchange_Technology_` | 1124 | Get_Main_Diplomacy_Choices_, Get_Net_Diplomacy_Choices_ | | ok |
| 0x1d0f8 | `Diplomacy_Surrender_` | 319 | Get_Main_*_Choices_, Get_Net_Diplomacy_Choices_ | | ok |
| (Diplomacy_Screen_) | | | | a human ambassador in a network game | ok (`Get_Net_Diplomacy_Choices_`) |

The council has no `_game_type` 2/3 test of its own: it reaches the network
only through the report and diplomacy paths above. Spec of the phase:
`diplomacy.md`. Also ported: `Do_Reply_To_Diplomacy_Request_` (0x8354e)
with `Load_Reply_To_Diplomacy_Request_Popup_` (0x86111),
`Add_Reply_To_Diplomacy_Request_Fields_` (0x821a6),
`Draw_Reply_To_Diplomacy_Request_Popup_` (0x853d3),
`Set_Diplomacy_Request_Help_List_` (0x6fb73),
`Diplomacy_Message_Is_Interactive_` (0x20162), `Draw_Diplomacy_Listen_Mode_`
(0x1f0bc), `Get_Listen_Mode_Message_` (0x1f34b), `Diplomacy_Synch_Mode_`
(0x1f12f), and `Main_Receive_Message_` cases 5..0xa, 0x1b..0x1d, 0x1f,
0x22, 0x26, 0x44, 0x64.

## Leaving, loading and the end (phase 6)

| Address | Routine | Size | Callers | Purpose | Port |
|---|---|---|---|---|---|
| 0x7dd41 | `Do_Main_Game_Popup_` | 718 | _Game_Popup_ | Quit: `Human_Abdicates_`, `Net_Uninit_`; NEW sets gt 0 | ok (`leaving.md`) |
| 0xfa97c | `Human_Abdicates_` | 13 | Do_Main_Game_Popup_ | `Send_Drop_Out_Info_(0x37)` | ok |
| 0xfa989 | `Human_Surrenders_` | 8 | Net_Next_Turn_ | `Send_Drop_Out_Info_(0x38)` | ok |
| 0xfa7b0 | `Send_Drop_Out_Info_` | 242 | Human_Abdicates_ | msgs 0x37/0x38 | ok |
| 0xfa8a2 | `Decode_Drop_Out_Info_` | 218 | Main_Receive_Message_ | the leaver becomes AI | ok |
| 0xfaa94 | `Mox_Net_Uninit_` | 47 | Client/Host/Net_Next_Turn_ | the last human: single player | ok |
| 0xfb27b | `Load_Net_Screen_` | 1386 | Screen_Control_ | host of a loaded game | ok |
| 0xfac85 | `Net_Pick_Position_` | 507 | Join_Net_Screen_ | joiner of a loaded game (msg 0x21) | ok |
| 0xfa05d | `Send_Pick_Your_Position_` | 118 | Load_Net_Screen_ | msg 0x20 | ok |
| 0xfa0d3 | `Decode_Pick_Your_Position_` | 49 | (none: case 0x20 does it inline) | | ok |
| 0xf0e17 | `Choose_Network_Plyrs_Screen_` | 591 | Load_Net_Screen_, Net_Pick_Position_ | the slots | ok |
| 0xefb50 | `Add_Choose_Net_Plyrs_Fields_` | 410 | Choose_Network_Plyrs_Screen_ | | ok |
| 0xf166e | `Draw_Choose_Net_Plyrs_Screen_` | 857 | Choose_Network_Plyrs_Screen_ | | ok |
| 0xf3fc6 | `Load_Choose_Net_Plyrs_Screen_` | 269 | Choose_Network_Plyrs_Screen_ | | ok |
| 0xf5573 | `N_Net_Players_Chosen_Positions_` | 49 | (none found) | players with a net id | — (unused) |
| 0xf4760 | `Load_Multi_Player_Game_Screen_` | 545 | Multi_Player_Screen_ | gt read/set/==2 | ok |
| 0xefe7a | `Add_Load_MP_Game_Screen_Fields_` | 269 | Load_Multi_Player_Game_Screen_ | | ok |
| 0xf1ce6 | `Draw_Load_MP_Game_Screen_` | 924 | Load_Multi_Player_Game_Screen_ | gt==2 | ok |
| 0xf41ad | `Load_Load_MP_Game_Screen_` | 285 | Load_Multi_Player_Game_Screen_ | gt==2 | ok |
| 0xf33ae | `Get_Valid_MP_Save_Games_` | 64 | Add_Load_MP_Game_Screen_Fields_, Multi_Player_Screen_, Valid_Net_Games_ | | ok |

## Where the port stands

Phases 1 to 6 are in (`transport.md`, `lobby.md`, `broadcast.md`,
`turn.md`, `combat.md`, `diplomacy.md`, `leaving.md`). Where the game code
still reaches a network branch it logs "not ported" (`g.Log`):

- `Bomb_Results_Popups_`' replay of another machine's ground battle
  (netcombat.go, UI only);
- the saved custom race in a network game (racesel.go: the original
  requests race 13);
- `Multi_Player_Screen_`'s modem setup (modems are dropped);
- `Main_Receive_Message_`'s ids without a case are ignored as in the
  original.

The data-only branches are ported: `Do_Autosave_`, `Next_Turn_`,
`Must_Display_Reports_`, `Move_All_Ships_Toward_Stars_`, `Init_New_Game_`,
the main screen's drawing and help list, and the main menu's refusal to
continue a network save.

## Counts

- The network game layer: 58 routines in NETMOX.C, 15 in NETSTART.C, 6 in
  NETTURN.C and about 35 network screens and helpers in module 111.
- Ported game code with network branches or calls: about 50 routines.
- The Net API: 22 routines (4 unused, 2 null-modem only).

## Regenerating

The inventory was made by a throwaway Python script over `build/orion2.dis`
and the Watcom debug information's globals: functions are the
`<name>:` headers, callers come from `call`/`jmp` targets, the modules from
the globals' `mod` column (112 NETMOX.C, 113 NETSTART.C, 114 NETTURN.C, 187
net/netcode.c, 111 the multiplayer screens), and the Port column from
`grep -rlw NAME internal/` refined by hand. `Add_Fields_` and other static
names repeat across modules: key by address, not name.
