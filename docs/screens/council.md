# The High Council (COUNCIL.C)

Addresses are ELF. Go: `internal/game/turn_council.go`. The council meets
inside `Next_Turn_Calc_` (step `Check_For_Council_Meeting_`), so its draws
and state changes land between the step saves 54 and 55.

## Globals (0x19a020..0x19a0e4)

| Address | Name | Meaning |
|---|---|---|
| 0x19a020 | `_council_member_votes` | s16[8], by member |
| 0x19a030 | `_council_members` | u8[8], sorted by votes, most first; +100 once cheered |
| 0x19a058 | `_council_nominees_seg` | the two nominees' pictures |
| 0x19a060 / 0x19a080 | `_council_light/dark_race_seg` | per seat: the voter's / the idle picture |
| 0x19a0a0 | `_player_voted_winner_message_id` | s8[20], cheers used |
| 0x19a0b4 | `_council_nominee_votes` | s16[2] |
| 0x19a0b8.. | `_council_no_votes/vote/votes/votes_total_message` | jimtext 0xb..0xe |
| 0x19a0c4 | `_council_message` | the paragraph at the bottom (0xc8) |
| 0x19a0c8 | `_council_nominee_vote_total` | s16[2], the tally |
| 0x19a0d4 | `_council_vote_cast` | votes of the current voter |
| 0x19a0d6 | `_council_total_votes` | |
| 0x19a0d8 | `_council_nominees` | u8[2]; stale when nobody has votes |
| 0x19a0da | `_council_voter` | s8, −1 none |
| 0x19a0db | `_council_total_members` | |
| 0x19a0dc | `_winning_player` | the human's choice, or the defied winner |
| 0x19a0dd | `_council_vote_for` | 0/1, the nominee last voted for / elected |
| 0x19a0de | `_council_state` | 0..6, 0x63 done |
| 0x19a0e0 | `_council_winner` | −1; set on an accepted election (not saved) |
| 0x19a0e2 | `COUNCIL_FLAG` (save) | 1 meeting held, 2 the human elected, 3 an AI elected |
| 0x19a0e4 | `next_council_meeting_turn` (save) | |

## Flow

`Check_For_Council_Meeting_` (0x168af): turn = stardate − 35000 ≥ 25, turn
≥ next meeting or COUNCIL_FLAG 0, more than two players alive, at least
half the stars (NUM_STARS/2, rounded to zero) owned → `Main_Council_Screen_`,
then next meeting = stardate − 34975.

`Main_Council_Screen_` (0x15239): help off, fields drawn, window reset,
clip off, the blank cursor list 0x180cb0, music 0x77, fade out, input
delay 1, stencil refresh, the auto function saved in 0x19aba0 (shared with
the GNN screen), `Reload_Main_Council_Screen_`, COUNCIL_FLAG = 1, state 0,
tally 0, auto `Draw_Main_Council_Screen_` (delay 2). Loop until state 0x63:
`Get_Council_Message_`, `Control_Main_Council_Screen_` (click), the state
steps 0→1→2, 4/5/6→0x63; in state 2 `Council_Votes_`,
`Council_Finished_Voting_`, `Council_Accept_Ruling_`. Then auto off,
`something_interesting_happened[PLAYER_NUM]` = 1, fade out, the auto
function back and `Fade_Into_Screen_(Draw_Main_Screen_)` with EDX =
`&_draw_main_screen_to_back` (left over: it is set to 1), palette 0.

`Reload_Main_Council_Screen_` (0x156d4): council.LBX 0 as cursor and
palette, black; nominee 0 = the first player with the most votes
(`Calc_Council_Vote_`: ⌈population/10⌉, strictly more than the best so far,
starting at 0), nominee 1 the same without nominee 0; the members are the
other living players, sorted (bubble, descending, stable); total votes.
Pictures: member k of race r: council.lbx r·6+2+k (voter) and r·6+0x50+k;
nominees r·2+0x9e and r·2+0x9f; council.LBX 1 the hall (multi-file
animation slot 0, first frame on both pages); the texts; voter −1.

`Get_Council_Message_` (0x15be9), jimtext.lbx by state: 0 → 0xf; 1 → 0x10
(both nominees' names); 2 → 0x11 (nominee voted for, votes cast); 3 →
0x12 (voter, votes: abstains); 4 → 0x13 (the elected); 5 → 0x14 (no
majority); 6 → 0x15, plus 0x16 with `_winning_player` when the human was
elected.

`Council_Votes_` (0x15ebc): each member that is not the human:
`Vote_Check_` for nominee 0 and 1; both or neither → abstain (0), else 1
or 2; `Council_Vote_Result_`. Then each AI nominee votes for itself (0
without votes); then `Council_Player_Vote_`.

`Vote_Check_(c, s, d)` (0x16021), 16-bit sum: treaty s→c ≥ 4 → −1, == 2 →
200, else relations + Diplo6D7 + 2·Diplo7EE − 20·HumanBias[c] (+100 at war
with d, +40 owner of Orion's planet (`Orion_Owner_`), +40 pick 20 of c, −100
pick 19 of c, +30 AppStatus[200] of c known, +50 last vote for c, +50 treaty
1, +25 trade, +25 research; ·6/(difficulty+6) for a human c above
difficulty 2); votes if `Random_(200)` ≤ it.

`Council_Vote_Result_(v, voter)` (0x161e4): 0 → state 3, relations −6 to
both nominees, vote −1; 1/2 → state 2, tally += votes cast, vote_for, +0x18
(kind 0x10) to the chosen, −12 (kind 9) to the other, CouncilVote = chosen.
Then message and click.

`Council_Player_Vote_` (0x1633c): votes cast = the human's; a list at
(0xd6, 0x184) width 0xb4: "  name" of each nominee (jimtext 0x18 when it
is the human), abstain (0x19); title 0x17 with the votes. Row 0/1: winning
player and CouncilVote = that nominee, vote 1/2; row 2: vote −1, 0; a
cancelled list (−1) abstains without touching CouncilVote.

`Council_Finished_Voting_` (0x15df8): need = (2·total + 2)/3; nominee 0
first, then 1: elected → vote_for, state 4, COUNCIL_FLAG 2 (the human) or
3; else state 5. Message, click, state step.

`Council_Accept_Ruling_` (0x1660b), only after an election: list "accept"
(0x1a), "defy" (0x1b), title 0x1c. Accept: state 0x63, `_council_winner`;
the human → `Council_Player_Voted_Winner_` and the diplomatic win flic;
WonCouncil (+0x1f1) = 1. Defy: state 6; the winning player is the other
nominee when the human was elected, else the elected; message, click; every
other living AI that `May_Surrender_Empire_` surrenders to it,
`Do_Surrenders_`, then trade, research, tribute and treaties with the human
broken and war declared.

`Council_Player_Voted_Winner_` (0x169c9): the losing nominee, then each
member (marked +100 so it is no longer drawn), cheers on its own screen:
`Get_Player_Voted_Winner_Message_` (`Random_(14)` until an unused one;
councmsg.lbx entry r + language·14), its race's cheer council.LBX race+0xb8
in animation slot 1, until a click. Without animations
`Draw_Council_Player_Voted_Winner_` draws neither the hall nor the cheer.

`Draw_Main_Council_Screen_` (0x15421): the hall's frame with animations;
the nominees' labels (style 2 outline, colours 0x180cbc) at x 0x1e and
0x1c3, y 0x1e: no votes / 1 vote / n votes; "%d %s" total centred at 0x140;
the members at the seats (x 0x2c, 0x208, 0x8f, 0x1bc, 0xd7, 0x179; y 0x60,
0x64, 0x94, 0x95, 0xa9, 0xa8), the voter in states 2/3 in its voting pose;
the nominees at (0x73 + 0xcd·k, 0xe1), animated only while voting; the
message in style 4 outline at (0x37, 0x1d6 − height), width 0x226.

The game ends after the turn when `_council_winner` is set (see
`endgame.md`).

## Port

`g.council.choose` answers the two lists when there is no screen (the
step tests run without an engine); nil answers −1 (abstain, and no
ruling to accept). The flic is not ported (Smacker).
