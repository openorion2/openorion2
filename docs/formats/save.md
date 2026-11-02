# Saved game (SAVEn.GAM)

Reader: `internal/save`. Source of truth: `Save_Game_` (ELF 0x1160b) and
`Load_Game_` (0x10e2f) in `MOX\OTHERS\LOADSAVE.C`.

A save is the game's global state written with one `fwrite` per block, in
the order below, with no headers or padding between blocks. Every save is
exactly 208000 bytes (all ten shipped saves are). The first dword is the
format number 0xe0. The random seed block is `Get_Random_Seed()` at the
moment of saving. The last block is 1000 zero bytes (a cleared local
buffer). Slot 10 is the autosave; its description is "(Auto Save)".
`Save_Game_(n)` writes `SAVE<n+1>.GAM` and takes the description of slot
`min(n, 9)`.

| Offset | Block (game global) | Record size | Records |
|---|---|---|---|
| 0x00000 | `version` | 4 | 1 |
| 0x00004 | `save_game_description` | 37 | 1 |
| 0x00029 | `stardate` | 4 | 1 |
| 0x0002d | `game_type` | 1 | 1 |
| 0x0002e | `settings` | 553 | 1 |
| 0x00257 | `random_seed` | 4 | 1 |
| 0x0025b | `NUM_COLONIES` | 2 | 1 |
| 0x0025d | `colony` | 361 | 250 |
| 0x162e7 | `planet_count` | 2 | 1 |
| 0x162e9 | `planet` | 17 | 360 |
| 0x17ad1 | `NUM_STARS` | 2 | 1 |
| 0x17ad3 | `star` | 113 | 72 |
| 0x19a9b | `leaders` | 59 | 67 |
| 0x1aa0c | `NUM_PLAYERS` | 2 | 1 |
| 0x1aa0e | `player` | 3753 | 8 |
| 0x21f56 | `NUM_SHIPS` | 2 | 1 |
| 0x21f58 | `ship` | 129 | 500 |
| 0x31b4c | `moveable_box` | 28 | 5 |
| 0x31bd8 | `moveable_box_order` | 2 | 5 |
| 0x31be2 | `PLAYER_NUM` | 2 | 1 |
| 0x31be4 | `max_map_scale` | 2 | 1 |
| 0x31be6 | `max_zoom_count` | 2 | 1 |
| 0x31be8 | `star_bg_index` | 1 | 1 |
| 0x31be9 | `MAP_MAX_X` | 2 | 1 |
| 0x31beb | `MAP_MAX_Y` | 2 | 1 |
| 0x31bed | `fleet_icon_button_count` | 2 | 1 |
| 0x31bef | `nebula` | 20 | 1 |
| 0x31c03 | `NUM_NEBULAS` | 1 | 1 |
| 0x31c04 | `bill_savegame` | 12 | 1 |
| 0x31c10 | `n_reports` | 2 | 1 |
| 0x31c12 | `reports` | 950 | 1 |
| 0x31fc8 | `done_player_reports` | 1 | 1 |
| 0x31fc9 | `something_interesting_happened` | 8 | 1 |
| 0x31fd1 | `done_colonize_check` | 9 | 1 |
| 0x31fda | `event_data` | 9 | 36 |
| 0x3211e | `msgs` | 18 | 100 |
| 0x32826 | `first_msg` | 1 | 8 |
| 0x3282e | `max_msgs` | 1 | 8 |
| 0x32836 | `n_msgs` | 1 | 8 |
| 0x3283e | `displayed_msgs` | 1 | 1 |
| 0x3283f | `displayed_gnn` | 1 | 1 |
| 0x32840 | `COUNCIL_FLAG` | 2 | 1 |
| 0x32842 | `next_council_meeting_turn` | 2 | 1 |
| 0x32844 | `antaran_resource_level` | 2 | 1 |
| 0x32846 | `chance_for_antaran_invasion` | 2 | 1 |
| 0x32848 | `disallow_saves_code` | 2 | 1 |
| 0x3284a | `last_event_year` | 4 | 1 |
| 0x3284e | `event_delay_count` | 2 | 1 |
| 0x32850 | `antarans` | 66 | 1 |
| 0x32892 | `cur_map_scale` | 2 | 1 |
| 0x32894 | `cur_map_x` | 2 | 1 |
| 0x32896 | `cur_map_y` | 2 | 1 |
| 0x32898 | `padding` | 1000 | 1 |

Counts (`NUM_COLONIES`, `planet_count`, `NUM_STARS`, `NUM_PLAYERS`,
`NUM_SHIPS`) say how many records of the fixed arrays are in use. In the
shipped saves: 20–71 stars, 54–182 planets, 8–79 colonies, 3–8 players, 5–194
ships.

Record layouts, all decoded by `internal/save` and checked against the ten
saves (round trip and cross links):

- `save-stars.md`: star, nebula, map globals
- `save-planets.md`: planet, colony (with colonists, queue codes, buildings)
- `save-players.md`: player (identity, economy, tech, designs, diplomacy,
  picks, history)
- `save-ships.md`: ship, design, leader, moveable box
- `save-misc.md`: header, settings (= MOX.SET), reports, messages, events,
  council, Antarans, and what `Load_Game_` does after reading
