# Network game: the broadcast (msgs 2 and 3)

The host sends the whole game state to every human after the new game
(full) and after every turn (a diff against its previous broadcast).
Ported in `internal/game/netbroadcast.go`; tests in
`netbroadcast_test.go` against `build/oracle/net2t`.

## Routines

| Address | Routine | Port |
|---|---|---|
| 0xf74cd | `Broadcast_Game_Data_Differential_(full)` | `broadcastGameDataDifferential` |
| 0xf79f6 | `Decode_Broadcast_Game_Data_Differential_(buf, len, full)` | `decodeBroadcastGameDataDifferential` |
| 0xf70eb | `Make_Difference_Buffer_` | `makeDifferenceBuffer` |
| 0xf72d1 | `Decode_Difference_Buffer_` | `decodeDifferenceBuffer` |
| 0xf73c2, 0xf744c, 0xf747b, 0xf6f90.. | `Open/Close/Write_Modem_File_`, `fseek/fgetc/fread/fwrite_modem_file_` | `netGame.prev` (the memory mode only) |
| 0xfd7ea | `Client_Initialize_Reports_` | `clientInitializeReports` |
| 0xf5a9f cases 2, 3 | msg 2 {u16 chunks}: a 0xe1000 block, counters to 0, 0x1ab09c = sender, `Net_Init_Host_Election_`; msg 3: the chunk at received × 0x7800 | `mainReceiveMessage` |

## The buffer

Built in a `Push_Block_` of 0xe1000 bytes, little endian, no padding:

| Field | Size | Global |
|---|---|---|
| creator's net id | 2 | 0x1aaf68 |
| `_stardate` | 4 | 0x192fd8 |
| settings +2 (random events) | 1 | 0x199bde |
| settings +0xd4..+0xd9 (difficulty, players, size, age, strategic, tech) | 6 | 0x199cb0.. |
| settings +0xd3 (Antarans) | 1 | 0x199caf |
| `Get_Random_Seed_()` | 4 | |
| `NUM_PLAYERS`, players | 2 + n × 0xea9 | |
| `NUM_STARS`, stars | 2 + (n + 1) × 0x71 | one record past the last star |
| `planet_count`, planets | 2 + n × 0x11 | |
| `NUM_COLONIES`, all 250 colonies | 2 + 0x1608a | |
| `NUM_SHIPS`, all 500 ships | 2 + 0xfbf4 | |
| leaders | 0xf71 | |
| `n_reports`, `_reports` | 2 + 0x3b6 | |
| `_event_data` | 0x144 | |
| `first_msg`, `max_msgs`, `n_msgs` | 3 × NUM_PLAYERS | 0x1aa40c, 0x1aab24, 0x1aab1c |
| `_msgs` | 0x708 | |
| `COUNCIL_FLAG`, `next_council_meeting_turn` | 2 + 2 | |
| `_antarans` | 0x42 | |
| `NUM_NEBULAS`, nebulas | 1 + n × 5 | |
| `last_event_year`, `event_delay_count` | 4 + 2 | |
| `bill_savegame` | 12 | |

`Mox_Update_` runs after the settings bytes, after the ships and at the
end. With 72 stars the extra star record lies past `_star`'s block; the
port sends zeros there and marks them stale (no oracle has 72 stars).

The full broadcast of `net2t` (2 players, 22 stars) is 203126 bytes: 7
chunks, 6 × 0x7800 and 18806.

## Sending

Full: the buffer is the previous buffer from now on and is sent as it
is. Diff: `Make_Difference_Buffer_` writes the diff right after the
buffer, then the new buffer replaces the previous one. Msg 2 {u16
(len + 0x77ff) / 0x7800} to 0x7ffe, then per chunk: `_temp_string` =
"sending chunk %d of %d\n" (RSTRING 156), then at least once
`Set_Page_Off_`, `Invoke_Auto_Function_`, `_TOGGLE_PAGES_`, `Mox_Update_`
while `Net_Is_Sending_` (one frame per chunk), then msg 3.

## The diff

`Make_Difference_Buffer_` compares the new buffer byte by byte with the
previous one (read with `fgetc_modem_file_`, −1 at its end). Codes:

- 0x00..0xf4: copy that many bytes of the previous buffer (a run of equal
  bytes, at most 0xf4; a run reaching 0xf4 is emitted at once);
- 0xf5 + k (k = 0..9): k literal bytes follow (a literal reaching 9 bytes
  is emitted at once as 0xfe; a literal ends before two equal bytes in a
  row: one equal byte stays inside it);
- 0xff: the end.

State machine (0 start, 1 equal run, 2 literal, 3 literal whose last byte
was equal, 4 done) as at 0xf7163..0xf72bc, jump table 0xf70d7. Quirks
kept: when the previous buffer ends before the new one, the game exits
("modem disk file error", RSTRING 150); when it is longer, the loop reads
on past the new buffer into the diff being written (the port keeps buffer
and diff in one block, as the original). An output reaching the block's
end exits with "modem diff buffer error" (151). `Mox_Update_` every 256
bytes.

`Decode_Difference_Buffer_` applies the codes against the previous buffer
(`fread`/`fseek` of the memory file), output after the input in the same
block; reading past the previous buffer, or the end code past the input
or the output limit, exits with the executable's messages at 0x17a110 and
0x17a138. `Mox_Update_` after each code but the end.

## Decoding

`Decode_Broadcast_Game_Data_Differential_` decodes a diff (an empty
result exits: "error decoding broadcast"), makes the result the previous
buffer, and copies every field back, `Set_Random_Seed_` with the host's
seed. After the players it recounts the network humans (0x1aaf5d): from
the last player down, an eliminated one gets net id −1 and objective 0,
otherwise objective 100 counts. After the reports it runs
`Client_Initialize_Reports_` (done-colonize flags, done-reports, displayed
messages and GNN, `_antarans`+0x41 cleared).

## Per-viewer state

The broadcast carries the host's star records, so `star[].Owner` (the
owner as the viewer sees it) arrives as the host's view. The joiner's own
`Update_Star_Owners_` afterwards fixes the stars it sees, and later diffs
are against the host's previous broadcast, so a byte the host never
changes is never sent again and the joiner's own value survives.
`TestNetBroadcastJoinerState`: after the decode only the two home stars'
owners differ from the joiner's autosave, none after
`Update_Star_Owners_`.

## Tests

- `TestNetBroadcastCodec`: the full broadcast of `net2t` decodes and
  encodes back byte for byte (seed included); each of the two turn diffs
  decodes against the buffer before it, and `Make_Difference_Buffer_` on
  the two whole buffers gives the original's diff byte for byte.
- `TestNetBroadcastJoinerState`: above.
