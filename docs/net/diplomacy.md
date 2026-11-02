# Network game: diplomacy

Network diplomacy: talks with an AI through the arbiter, requests
between humans and their dialogue, read from `build/orion2.dis`. Code:
`internal/game/netdiplo.go` (NETMOX.C), `netdiplo_talk.go` (DIP-SCRN.C's
network part), the network branches in `diplomacy_human.go`,
`diplomacy_hotseat.go`, `race.go`, `reports_checks.go` and
`mainscreen_loop.go`.

## The net state (`netState`, 0x1ab03f)

| Value | Meaning | Set by |
|---|---|---|
| 4 | idle | `Net_End_*`, msgs 9, 0x1f, `No_AI_Diplomacy_Allowed_` |
| 0x25 | asked the arbiter for an AI (msg 5 sent) | `Net_Request_AI_Diplomacy_` |
| 0x26 | queued: another human talks with that AI (msg 7) | msg 7 |
| 0x27 | withdrawing (msg 8 sent) | `Net_Cancel_AI_Diplomacy_Request_` |
| 0x28 | go ahead (msg 6) | msg 6 |
| 0x29 | asked a human (msg 0x1b sent) | `Net_Request_Human_Diplomacy_` |
| 0x2a | refused (msg 0x1d) | msg 0x1d |
| 0x2b | the talk begins (his msg 0x44 came) | msg 0x44 |
| 0x2e | accepted or crossing requests; our msg 0x44 is due | msgs 0x1b, 0x1c, `Accept_Diplomacy_Request_` |

Other NETMOX bytes: 0x1ab04a the player talked to (`netTalkTo`),
0x1ab054 the humans asking us (bits; the request lights), 0x1ab069 the
requests not yet put to the player, 0x1ab066 "we talk first", 0x1ab067
"he says wait" (msg 0x26), 0x1ab0cc "the talk began from his request",
0x1ab0cb the player msg 0x44 is due to.

## With an AI: the arbiter

The arbiter is last turn's host (`netLastHost`, 0x1ab09c; msg 2 sets it on
clients, `Host_Next_Turn_` and the start on the host). It keeps per player
the Do_Net_Combat_Update_ slot: +0 a state (0x26 queued, 0x28 talking, 4
idle), +0xb the AI asked for.

- **Asking.** `Net_Request_AI_Diplomacy_` (0xfa270) sends msg 5 {ai} and
  sets state 0x25; with no arbiter yet (`netLastHost` −1) nothing is sent
  and `No_AI_Diplomacy_Allowed_` (0xfa3dc) answers yes (state 4).
- **Msg 5** (case 0xf5d3d): the slot gets {0x26, ai};
  `Get_Next_AI_Talker_(ai)` (0xfa1a3) returns −1 while someone's slot is
  {0x28, ai}: then msg 7 is due to the asker. Otherwise the asker's slot
  becomes 0x28 and msg 6 is due. *Quirk:* the weighted pick (charismatic 3,
  normal 2, repulsive 1; only the top weight kept;
  `Get_Weighted_Choice_Char_`, one `Random_` draw on the arbiter) is only
  tested against −1: it is always the asker who goes ahead.
- **Msgs 6, 7** set 0x28 (from 0x25 or 0x26) and 0x26 (from 0x25).
- **Withdrawing** (`Net_Cancel_AI_Diplomacy_Request_` 0xfa38f): msg 8
  {−1}, state 0x27. Msg 8 (0xf5dd5): unless the slot was queued, its msg 6
  is no longer due and the next talker (if any) goes ahead; the slot is
  freed, msg 9 is due. Msg 9: state 4, `netTalkTo` −1
  (`Net_Cancel_AI_Diplomacy_Confirmed_` 0xfa3d1 waits for it).
- **The end** (`Net_End_AI_Diplomacy_` 0xfa2c5, game types 2 and 3): msg
  0xa {u32 ai, the AI's record, 72 bytes: `star[i].surrender_to[ai]` for
  the stars (the rest of the 72 is the frame's stale stack), u32 our
  player, our record} = 0x1da2 bytes. Msg 0xa (0xf5e55) writes both
  records and the AI's surrender bytes (the AI's index from the slot, not
  the payload), frees the slot, marks msg 0x22 due to every human
  (`netSend22[ai]` = 0xff, `netPartner[ai]` = the sender) and lets the
  next queued talker go ahead.
- **Msg 0x22** (`Mox_Net_Update_` 0xf6c32, ported in phase 1): the same
  layout, to every human but the arbiter and the partner. Its receive
  (0xf63a2) writes both records; *quirk:* its star loop copies the stars
  *into* the receive buffer, so the surrender bytes are not applied.

Who asks:

- `Has_Diplomacy_Messages_` (0xfe408), the report phase: in game types 2
  and 3 an interactive message (`Diplomacy_Message_Is_Interactive_`
  0x20162: 0x1c..0x24, 0x53..0x56, 0x61..0x6a, 0x7d) goes through the
  arbiter: request, `Mox_Update_` until 0x28, `Npc_Diplomacy_Screen_`,
  then `Net_End_AI_Diplomacy_`. Other messages show at once. So the AI's
  ambassador does open on the human's own machine, as in phases 3 and 4;
  only the answering is serialised. The loop has no other exit: a human
  queued behind another waits on the ambassador's screen.
  *Quirk:* the check does not skip humans: a human's message flags for
  another human (e.g. first contact, 3) open `Npc_Diplomacy_Screen_` with
  the human as the ambassador.
- AUDIENCE on the races screen (`Race_Screen_` 0x10b603..0x10b701): the
  request, then a loop of one frame each (`_net_state` 1 draws "waiting
  for ..." in the net box, picture 0x3b of RACES.LBX, BILLTEXT 0x4c);
  any input withdraws. No arbiter: BILLTEX2 0x1b in a message box. Then
  `Diplomacy_Screen_` and `Net_End_AI_Diplomacy_`.
- `Diplomacy_Screen_` (0x16c4e): with an AI in a network game the
  ambassador always talks (`_ambassador_option` 1), whatever his worst
  modifier.

## Between humans: requests

- **Asking** (`Net_Request_Human_Diplomacy_` 0xfa3f6): if he asked us
  already (bit in 0x1ab054) the talk begins: state 0x2e, msg 0x44 due, msg
  0x1c with our record. Otherwise state 0x29 and, unless he has pressed
  Turn, msg 0x1b {our record, 72 bytes of `surrender_to[me]` (stale past
  the stars)} = 0xef1 bytes.
- **Msg 0x1b** (0xf6119), ignored once we pressed Turn: his record and his
  surrender bytes; if we asked him too (0x29): crossing requests, state
  0x2e, msg 0x44 due, 0x1ab066 = (his index > ours). Else, if we ignore
  him (player+0xe60) msg 0x1d is due (`Mox_Net_Update_`), otherwise his
  bit lights up (0x1ab054, 0x1ab069).
- **Msg 0x1c** (0xf620c): his record; if we asked him: state 0x2e, msg
  0x44 due, we talk first. Else as msg 0x1b's tail.
- **Msg 0x44** (0xf6776): his record; in state 0x2e with him: 0x2b.
- **Msg 0x1d**: state 0x2a. **Msg 0x1f** (0xf62af): his bits cleared; in
  0x2b with him: 4; in 0x2e with him: 4 if the talk began from his request,
  else back to 0x29. **Msg 0x26**: in 0x29 with him, "he says wait".
- **Msg 0x64** {i16}: `Declare_War_(sender, value)`. Unreachable: no
  call of `Mox_Send_Message_` (87 sites) or `Net_Send_Message_` passes id
  0x64; every id is a constant there but the broadcast's chunk ids.
- **The main screen** (`Main_Screen_` 0x86490, each pass of its loop in
  game types 2 and 3): `Check_For_Diplomacy_Request_` (0x8250c) takes the
  new requests one at a time (`Human_Requesting_Diplomacy_` 0xfa5ce: a
  reservoir pick, one `Random_(k)` per set bit), each in
  `Do_Reply_To_Diplomacy_Request_` (0x8354e): BUFFER0.LBX 0xf..0x12, the
  buttons A(ccept) I(gnore) W(ait), H_Message 0xdb "<name> of the <race>";
  accept (`Accept_Diplomacy_Request_` 0xfa623: msg 0x1c, wait for 0x2b,
  `Diplomacy_Screen_`), ignore (`Refuse_Diplomacy_Request_` 0xfa728: we
  ignore him from now on, msg 0x1d), wait
  (`Tell_Diplomacy_Requester_To_Wait_` 0xfa774: msg 0x26). If any was
  answered the main screen is set up again and faded in.
- `Draw_Diplomacy_Request_Lights_` (0x83d06): a blinking light (BUFFER0
  0x109+banner, its own frame counter per banner at 0x19c148) for each bit
  of 0x1ab054, from x 0x1fa leftwards.
- **The races screen** (0x10b706..0x10b947): the request, then the loop
  (any input withdraws while still 0x29; "he says wait" shows BILLTEXT
  0x4d) until the talk begins, he refuses (BILLTEXT 0x38), he presses Turn
  (0x37) or we withdraw (msg 0x1f). AUDIENCE clears our ignore bit for the
  player asked (0x10b5e7), so a human we ask is never ignored by us; IGNORE
  (who mode) toggles it (0x10b4e5).
- **Msgs that wait for IPX** (netdip8): a due msg 0x44 goes out from
  `Mox_Net_Update_`'s round robin, which is skipped while `Net_Is_Sending_`
  (IPX packets not yet acknowledged). A machine whose 0x1c is still in
  flight therefore sends its 0x44 only later, and a 0x1f arriving
  meanwhile (state 0x2e, the request was his) cancels it. The port's
  transport never reports sending; the replay of netdip8 makes it do so
  while its relay holds the packets (`netplay` test endpoint).
- `Accept_Diplomacy_Request_`'s flag is set only in state 0x2b and
  cleared in 4, 0x2a and 0x2f: in any other state it is the frame's old
  byte (no message leads there; the port shows the message box).
  `Net_May_Begin_Human_Diplomacy_` (0xfa521) likewise returns the caller's
  ah outside 4 and 0x29..0x2f (the port: false).

## Between humans: the talk

`Diplomacy_Screen_`: the requester talks first (0x1ab066 = 1 from msg
0x1c; crossing requests: the higher index). `_talker_ambassador`
(0x19aa67), `_listen_ambassador` (0x19aa68), then
`Get_Net_Diplomacy_Choices_` (0x1def8).

Every step is msg 0x7d0 with `_net_diplomacy_message` (0x19a190), 8 bytes:

| Offset | Field |
|---|---|
| +0 | u8 talker |
| +1 | u8 listener |
| +2 | i16 a |
| +4 | u16 code |
| +6 | i16 b |

A global: fields a sender does not set keep the previous message's values
(sent or received). `Diplomacy_Receive_Message_` (0x1f230, ids
2000..2999) copies a received 0x7d0 into it; code 0x3ee sets
`_net_diplomacy_esc_pressed`, 0x3f4 `_synch_up_established_flag`, 0x3f6
nothing; any other code becomes `_response_message` (0x3f3 as 0, the
statement cleared) and the talker and listener are taken from the packet.
Msg 0x834 is a chat line: 100 bytes of `_net_diplomacy_chat_message`
(the bytes after its terminator are what the buffer held), copied to
`_receive_chat_message` and shown under the title as "%s sez: %s"
(`Draw_Diplomacy_Synch_Mode_`).

Codes (`Get_Net_Diplomacy_Choices_`' table at 0x1de96, its jumps at
0x1deb0):

| Code | Sent by | Meaning | The listener |
|---|---|---|---|
| 0x3f4 | both, first | synch | waits for the other's (`Diplomacy_Synch_Mode_`) |
| 0x82 | talker | good bye | the response shown, the talk ends |
| 0x3e8 | `Check_Human_Treaty_Proposal_` | treaty a, statement b | `Get_Human_Treaty_Choice_`, answers {code = response, b = choice}; 3 → `Start_Net_Treaty_` |
| 0x3e9 | `Diplomacy_Break_Treaty_Resolution_` | broken treaty a | the message, `Break_*` |
| 0x3ea | `Diplomacy_Declare_War_Resolution_` | war | the message, `Declare_War_` |
| 0x3eb | `Diplomacy_Surrender_` | surrender | the message, the talk ends |
| 0x3ec | `Get_Human_Gift_Response_` | gift a (1 BC, 2 tribute, 3 tech, 4 star), item b | applies it |
| 0x3ed | `Get_Human_Demand_Response_` | demand a, item b | `Get_Human_Demand_Choice_`, gives in or not; answers {0xa8/0xa9, b = choice} |
| 0x3ee | listener | Esc / Good Bye | (the talker's flag) |
| 0x3ef / 0x3f0 / 0x3f5 | `Diplomacy_Human_Need_Better_Offer_` | a star / a technology / a sum for the treaty | accepts or not, answers b 1/0 (then `Start_Net_Treaty_`) |
| 0x3f1 | `Diplomacy_Exchange_Technology_` | wants technology a | picks one of his (`Get_Differential_Tech_List_`), answers b |
| 0x3f2 | the talker | agrees (a ≠ −1) to swap for b | both get their technology |
| 0x3f3 | `Diplomacy_Net_Switcheroo_` | the listener talks now | roles swapped |

- **The talker's menu**: `_main_net_diplomacy_list` (11 rows: the main
  menu's first seven, jimtext2 0x41 chat, jimtext 0x27 wait, Good Bye,
  "") with `Diplomacy_Set_Main_Options_`; Esc or Good Bye sends 0x82. A
  repulsive race gets `Get_Main_Repulsive_Diplomacy_Choices_` with its net
  flag: rows peace, war, surrender, chat, Good Bye (0x19a0e8), no worst
  modifier check, Good Bye sends 0x82 and sets the Esc flag.
- **The questions** (treaty, demand, better offers, tech exchange) send
  their code and loop `Diplomacy_Listen_Mode_` until b changes from −1
  (−2 for the exchange) or the other side leaves (then 0, resp. −1).
- **Listen mode** (0x1ede9): one frame with hidden fields C (chat), G and
  Esc (good bye: code 0x3ee to the talker); the chat (`Diplomacy_Chat_`
  0x1ef5b) is a continuous input field at (10, 0x8d), 0x12c wide, the area
  darkened (`Darken_Fill_`); Enter sends msg 0x834.
- **Both machines apply** what is agreed: the talker in the shared
  routines (the treaty in `Diplomacy_Determine_Treaty_Proposal_`, the
  money, star or technology of a better offer), the listener in his
  handler (`Start_Net_Treaty_` 0x1f17d by `_net_diplomacy_treaty_type`).
  Some are one-sided: the talker alone adjusts the diplomat modifiers
  and, in an exchange, `Change_Relations_`. The host learns each from the
  next upload (each machine uploads only its own records).
- The help lists are off while two humans talk
  (`Setup_Diplomacy_Help_`, `Setup_Full_Screen_Diplomacy_Help_`).
- *Quirk* (`Diplomacy_Human_Need_Better_Offer_` 0x1bd5b, network part):
  the menu choice variable ([ebp-8]) is reused for the star chosen and the
  technology row, so a star index equal to the technology row's number
  opens the technology list too, and so on to the money row. netdip4
  shows it in the original: star 2 offered with the rows system, technology,
  money (2) goes on into the money list. The hotseat path has no such
  fall-through: there [ebp-8] becomes the yes/no answer of the other human
  (0 or −1, 0x1bf44 and 0x1c14e) and the later rows are 1 or more, so
  the hotseat port (no fall-through) is right as it is.
- *Quirk:* a war or a surrender from the repulsive menu ends the talker's
  talk without a good bye (no code 0x82; the Esc flag is set instead). The
  listener stays in listen mode until he presses Esc or G (code 0x3ee to
  the talker, who has left; it sets his Esc flag, which his next
  `Diplomacy_Screen_` clears). After a surrender the listener's response
  box ends his side (netdip6).
- `Diplomacy_Generic_System_List_` (0x1a210) sets `_current_screen` to 6
  (the races screen's number) and nothing restores it: after a star was
  offered the main screen runs with that value until the next screen
  change (netdip4, the host).
- **Music.** `Diplomacy_Mouse_Init_` sets `_diplomacy_current_music` to −1,
  and `Diplomacy_Load_New_Ambassador_` (hotseat) calls it after the
  statement that chose the music, so the lists' `Draw_Diplomacy_Synch_Mode_`
  asks `Play_Streaming_Music_` for track 99 (−1 + 100). The old track is
  faded out and stopped, then `Play_Stream_Vol_` (0x113765) plays STREAM.LBX
  entry 99 without a range check: the offset table slot of entry 99
  (file offset 0x194) holds 0x10, and the 0x2c bytes read there as a WAV
  header give a data length of 0 (file bytes 0x38..0x3b): a sample of
  nothing, so silence. The port does the same (fade, stop, `Play_Stream_`
  of 99 logged) and finds no entry 99, so it is silent too; nothing to
  fix. (Network talks never reach it: `Get_Diplomacy_Statement_` sets the
  music before every list there.)
- *Quirk:* `Get_Main_Repulsive_Diplomacy_Choices_` sets the Esc flag after
  a war or a surrender in single player too; nothing clears it until the
  next `Diplomacy_Screen_`, so an AI ambassador's list in between
  (`Draw_Diplomacy_Synch_Mode_` calls `Exit_List_Field_` while it is set)
  closes at once. Ported as is.
- *Possible hang (original):* `Diplomacy_Screen_` clears the synch flag
  on entry; a 0x3f4 that arrives before the other side enters it is lost.

## Tests

- `TestNetDiplomacyOracle`: the original's netdip1 (talk and treaty),
  netdip2 (AI queue), netdip3 (wait, withdraw, refuse, ignored), netdip4
  (the better offers: star, money by the fall-through, technology),
  netdip5 (demands given in and refused, a technology exchange), netdip6
  (the repulsive menu: war, then surrender, msg 0x38), netdip7 (crossing
  requests; a request to an AI withdrawn from the queue, msgs 8 and 9),
  netdip8 (asking back with the request lit, msg 0x1c, refused as from an
  ignored human) replay send for send, autosaves included (`oracle.md`).
  Drivers: `netDipKeys` (a talk by keys, as the oracle's scripts),
  `netDipCross`, `netDipIgnored`; `netDipOracleHold` holds every receive
  as the oracle's relay did.
- `TestNetDiplomacyAIQueue`: two humans and an AI, the same queue in the
  port alone.
- `TestNetDiplomacyHumans` (netdiplo_test.go): two machines on the
  in-memory transport; machine 0 asks machine 1 from the races screen.
  `treaty` (a proposal accepted), `break` (then broken), `switch-chat` (then Wait: machine 1
  talks, chats a line, hands back), `gift-war` (a gift, a declaration of
  war), `refuse`, `cancel` (machine 1 says wait, machine 0 withdraws), `withdraw-refuse-ignored`:
  each machine's diplomacy sends (ids and codes) in order, the treaty or
  war on both machines, equal BC, then two turns with every client equal
  to the host after each broadcast.
- `TestNetSoak` `2+3ai-diplo`, `3+2ai-diplo`: everybody has met
  (`netSoakContacts`, on the host before `Next_Turn_`), 40 turns: talks
  with AIs go through the arbiter (msgs 5, 6, 7, 0xa, 0x22 counted; every
  asked talk ends), every client equals the host after each broadcast.
- `TestNetReceiveRouting`: ids 2000 and 0x834 reach
  `Diplomacy_Receive_Message_`, 0xffff is dropped.

## Not ported / open

- Not reached by an oracle run: the normal menu's surrender (row 7; the
  same `Diplomacy_Surrender_` as netdip6's), demanding a star or the
  removal of a fleet, peace from the repulsive menu, the chat and the hand
  over (port-only test `switch-chat`), a gift between humans (port-only
  `gift-war`), msg 0x64 (no sender, above).

- `Diplomacy_Initial_Synch_Up_` (0x1ed9c): only referenced by pointer;
  `Get_Net_Diplomacy_Choices_` sends the same packet inline.
- The council: it has no game type 2 branch of its own (functions.md);
  network games reach it through the report phase as single player does,
  on each machine. Not run in a network test.
