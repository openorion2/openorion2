# Network game: port-only UI

Everything the port shows or accepts in network games that the original
does not have. Each piece is marked `// port-only` in the code.

## Address boxes (internal/game/netui.go)

The original has no screen for a TCP address, so both boxes are put
together from its own pictures (phase 8; until then they were bare
placeholder rectangles):

- **Back** (`drawNetAddressBack`): the screen cleared to 0, the
  multiplayer frame MULTIGM.LBX 0 at (0, 0) (as `Change_MP_Game_Name_` and
  the game list draw it), a heading centred at y 0x5c in font style 4 with
  the net screens' colours (offset 0xf2, `Font_Colors2_(4, 0x9c, 0xf8)`).
  Nothing else: both buttons are inside the popup.
- **Popup**: the original's input box, as `Change_MP_Game_Name_` asks for
  the network game's name: `Setup_Input_Box_Popup_` with remapping
  (`setupInputBoxPopup`, INBOX.LBX 0 and its ACCEPT, 1) at
  `Star_Name_Popup_Screen_Center_X_/_Y_` (0xb1, 0x7d), its title the
  label, its line (field at +0x22, +0x36, 0xcd × 0x1a, style 3, colours
  offset 0x10, `Font_Colors2_(3, 0xc, 0x18)`) the value, up to 63
  characters, active at once (typing appends; Backspace deletes). The
  loop is `Input_Box_Popup_`'s; on top of the popup's fields the port adds
  a hidden field over ACCEPT with hot key O and CANCEL with its field (hot
  key Esc), and rebuilds them with the popup on a full redraw.
- **Buttons**: ACCEPT at +0x24 and CANCEL at +0x9a (both at +0x64, 0x62 ×
  0x1c, their fields the same rectangles), 0x14 pixels apart and symmetric
  about the popup's centre (0x24 from either side of its 0x120 width).
  The popup picture has an ACCEPT of its own drawn in at +0x60, +0x64, a
  socket with a one-pixel darker rim above, right and below it (x
  0x60..0xc2, y 0x63..0x80). `drawNetBoxSockets` repaints those rows of
  the popup from its own pixels (remapped as the popup is): a copy of the
  socket under each button, and in between the plain metal left of the
  socket (x 0x19..0x5e, repeated), so nothing of the old ACCEPT is left.
  CANCEL is ACCEPT's art relabelled (`netbutton.go`), shown pressed when
  clicked. Both boxes have this layout.
- **Keys and clicks**: OK is ACCEPT (shown pressed, frame 1) or Enter in
  the line; CANCEL is the button or Esc and leaves the value as it was.
  Nothing else leaves the box: unlike `Input_Box_Popup_`, which takes any
  return of its line field as OK, a click in the line keeps editing. The
  engine returns the line's field both for Enter and for a click on it;
  Enter also ends the edit (`active_input_field_number` -1), a click does
  not, and that tells them apart. The box runs with `Disable_Cancel`, so a
  right click cancels nothing (Esc then comes back as -line with the edit
  ended; a right click on a button ends the edit too, and the loop starts
  it again). The hot keys O and Esc of the hidden fields are never seen:
  hot keys are not matched while a line is edited, and the line always is.

| Screen | Heading | Label | Shown | Edits |
|---|---|---|---|---|
| Create a game (`netCreateGameScreen`) | CREATE NETWORK GAME  *name* | TCP Port To Listen On | in `Start_Net_Screen_` and `Load_Net_Screen_` before `Net_Create_Game_`, with the TCP transport | the port to listen on (default `:52525`; a bare number becomes `:number`); the game name is the new game screen's |
| Join a game (`netJoinAddressBox`) | JOIN NETWORK GAME | Host Or Host:Port To Join | from the game list's ADDRESS (below) | the creator's address, host or host:port (default port 52525) |

## No initializing screen (TCP)

`Start_Net_Screen_`, `Join_Net_Screen_` and `Load_Net_Screen_` open with
the "initializing network" screen (`Reload_Initializing_Net_Info_`,
MULTIGM.LBX 0x19, faded in with palette 0xb) while `Net_Init_` starts the
IPX driver. With TCP `Net_Init_` starts nothing, so the screen only
faded in and was gone at once. With the TCP transport (connection type 0)
the port skips it in all three (`reloadInitializingNetInfo`), and the
next screen fades in instead, the same way (`Fade_Into_Screen_`, palette
0xb; `netFadeInto`):

- Join: the game list (frame, list box, games, ADDRESS, CANCEL);
- Create (new or loaded game): the create box; with `-host` (no box) the
  new game's waiting-for-joiners screen; a loaded game's
  `Choose_Network_Plyrs_Screen_` fades in by itself;
- a "Comm failure" before any of these first fades in the bare frame.

The screens draw with the DAC dark: page flips inside the drawing do not
restore the palette, and the fade shows the page the drawing ended on.
The in-memory transport of the oracle tests keeps the original's
initializing screen (netlobby3, below).

## Leaving the boxes

The create box's CANCEL goes back to the multiplayer screen; with `-host`
or `-join` on the command line (or the in-memory transport of the tests)
it is skipped. Join Game has no box before the list: `Join_Net_Screen_`
goes straight to the game list, which lists the games on the LAN (no
address, "LAN games" below) or the one game of the relay at the address
given by `-join` (or the page's `?join=`). The browser cannot search the
LAN; its join address defaults to the page's host name (`web.md`), so its
list shows the games the serving host has. In the browser the create box
asks for the port the server listens on for the game, and the join box
for the address the server joins.

## Port-only buttons (internal/game/netbutton.go)

The address boxes' CANCEL and the game list's ADDRESS are original buttons with their label
erased and another printed (`drawNetButton`): in the label's rectangle
each pixel of the old letters (by colour) takes the plate's shade left of
it, then the label is printed centred in style 3 with a shadow below,
narrower (spacing, then style 2) when it does not fit. CANCEL is
INBOX.LBX 1 through the picture remap (dark letters, colour 2, shadow 5;
pressed: the up frame with lit letters 0x1c, shadow 0x27, as the pressed
frame's A runs into the bezel); ADDRESS is MULTIGM.LBX 0x101 (letters 1,
shadow 0xe3; pressed: frame 1 with 0xf9, shadow 0xee).

## The game list's ADDRESS

With the TCP transport (also with `-join`, and in the browser) the game
list (`Choose_Multi_Network_Game_Screen_`) gets ADDRESS in the left
channel of the list box's bottom bar (+0x35, +0x158, CANCEL's row), hot
key A, drawn at the end of the list's draw. It shows pressed and opens the
join box over the list, starting with the current address; its OK (ACCEPT
or Enter) sets the address and the list asks for games at once (an empty
line: the LAN), CANCEL or Esc keeps the address and the list as they were;
either way the list is drawn again. The list's own CANCEL leaves the
network for the multiplayer screen, as in the original. The in-memory
transport of the oracle tests has no ADDRESS, so the list's fields and
frames there are the original's.

`TestNetAddressBoxes`: no flip shows the initializing screen; Join lands
on the list, faded in fully (the LAN; with an address
given, that relay's game at once); ADDRESS opens the box, Esc and the
box's CANCEL return to the list with the address and the list unchanged;
clicks and right clicks in the line, beside the buttons and right clicks
on ACCEPT and CANCEL keep the box and the text; Enter and ACCEPT set the
address and refresh the list (with the periodic refresh held, the relay's
game shows); the list's CANCEL goes back to the multiplayer screen. The
create box's CANCEL goes back to the multiplayer screen, its Enter starts
the relay on the port given. It checks that the frame and the popup are
drawn, that ACCEPT and CANCEL lie inside the popup, symmetric about its
centre, at least 0x10 apart, their fields as wide as the art, and that
the gap holds only colours of the popup's metal left of ACCEPT;
`OPENORION2_NET_SHOTS=dir` saves each step's screen. No original capture
exists to compare with.

## Waiting for joiners

`Draw_Wait_For_New_Joiners_Screen_` is the original's; under its box the
port lists the stations in the game ("Player n", "(you)" for the own one)
(`drawNetJoinersList`).

## Lost players (phase 7, `hardening.md`)

No new art: the original's `Warning_Box_` (WARNING.LBX, the pulsing frame,
any input closes it) shows

- on the main screen, once per lost player: "*emperor (race)* has lost
  the connection. The computer plays the empire from now on." and, after a
  lost host of the turn, "The host of the turn has left. The turn starts
  over."; on the waiting screen the same lines come at once as GNN chat
  lines;
- when the game cannot go on, before `game.Run` returns: "The connection
  to *X*, who relays the game, is lost. The game ends.", "*X* left before
  the game started. The game ends.", "*X* has lost the connection in a
  battle with you. The game ends.", with `-netdrop end` "*X* has lost the
  connection. The game ends."

## Other changes

- The multiplayer screen's modem and null modem radios are drawn disabled
  (frame 2) and cannot be picked; a MOX.SET with a modem link is taken as
  the network.
- The game list (`Choose_Multi_Network_Game_Screen_`) asks the transport
  for its games every 60 frames (IPX kept the list current by itself); over
  TCP that is the one game at the join address, or with no address the
  games on the LAN (below).

## LAN games (port-only)

IPX listed every game on the network by itself; TCP needs an address. To
get the original's list back, a relay also answers on UDP port 52525
(`netplay.DiscoveryPort`; on the TCP listener's address, so a relay on
127.0.0.1 answers only there) a datagram `OPENORION2?` with `OPENORION2 <tcp
port>`. A join with an empty address (no `-join`, or the join box left
empty) broadcasts that question to the LAN and to 127.0.0.1, waits
0.4 s for answers, asks each relay found for its game over TCP as usual
(`Probe`) and lists them all; picking a row joins the relay of that name.
Only the first search is waited for: the list's later searches (every
60 frames) run in the background and the list shows the last one's
result. Without answers the search falls back to this machine's port
52525, as before. Limits: two games of the same name on the LAN join the
first that answered; a relay that cannot bind the UDP port (another relay
on the same machine has it) is not found, only reached by its address;
the browser build does not search (`TCP.Discovery` < 0 turns it off;
`TestLANDiscovery`).
- Player names: the original passes "myname" to the transport; the port
  passes `-name` when given.

## Screen comparisons

The network screens are compared with the original's captures (phase 9,
`internal/game/netscreens_test.go`). F12 does nothing on the original's
waiting screens (their loops read no keys), so a patched original captures the first page flip of each loop of MULTPLAY.C, NETMOX.C,
NETSTART.C, NETTURN.C and CHAT.C (0xefa52..0xfc734, 0xdd351..0xdd4b5): a
flip is captured when its caller differs from the previous flip's in that
code, as SCR(1000+n) beside the F12 captures' SCRn (the first capture of
a run is SCR00 either way). The port's tests record their counterpart
through the engine's `OnToggle` (the flips from the files ported from
those modules, the port-only address boxes left out) and compare the k-th
with the k-th, and the shots the scripts take where the original's F12
came:

- netlobby3 (`TestNetLobby3Oracle`): the multiplayer screen, initializing,
  waiting for joiners, the game list, waiting to join, the race screens
  with taken races, both refusals, the naming popup, the banner screen,
  receiving race setups, sending and receiving game data, generating map;
- netone1 (`TestNetOneOracle`): "Really start with only 1 player?", the
  race screen in custom mode, the custom race picks;
- netquit3 (`TestNetQuit3Oracle`): the joiners' lobby of a loaded game
  (network players, pick your empire, receiving data) and the turn's
  waiting screen (NETWORK PLAYERS) with a player who left;
- netdip1 (`TestNetDiplomacyOracle`): the request popup and the human
  ambassador's lists ("(Waiting for X)").

Masked, as they run by the clock: the light running over the waiting
boxes' top edge (its phase is the count of draws while the screen
waited), blinking cursors, the ambassador's hologram and the galaxy's
black hole; port-only: the modem radios (drawn disabled) and the list of
stations under the joiners box. Two F12 captures came before their screen
was drawn and are left out. Found and fixed: the game list's green
palette (16 colours of {changed, R, G, B} at 0x90, not RGB triples at
0xc0), the waiting screen's banners placed by player, and the game popup
opening on a stale page after a colony screen (`leaving.md`). Not compared:
the creator's waiting loop of a loaded game (two flips per pass, as many
captures as the clock gave) and screens of the battles' and talks' waits
beyond netdip1's (the battle runs' captures are the combat frame oracle's,
`combat.md`).

## Command line (cmd/openorion2)

- `-host [:port]` (also `-host=:port`): the transport listens there
  (default 52525) when this instance creates a game.
- `-join host[:port]`: the address the game list and the join use; none:
  the games on the LAN.
- `-name NAME`: the player's name for the transport.
- `-netdrop ai|end` (phase 7, `hardening.md`): a player whose link is
  lost becomes an AI and the game goes on (`ai`, the default), or the game
  ends for everyone (`end`).
- `-netcheck`: the desync check; `-netcheckdir DIR` (implies it) also
  writes this machine's save of a mismatching stardate into DIR.
- The creator's process stays after his game ends while the relay still
  has stations, printing "still relaying the network game for n
  player(s)"; Ctrl-C ends it (and the game for the others).

Trying two instances on one machine, each with its own copy of the game:

```
cp -R game build/game-h; cp -R game build/game-j
go run ./cmd/openorion2 -game build/game-h -host :52525 -name Alice
go run ./cmd/openorion2 -game build/game-j -join 127.0.0.1:52525 -name Bob
```

Creator: Multiplayer, Network, Start New Game, Accept, then START once the
other has joined. Joiner: Multiplayer, Network, Join Game (the list shows
the LAN's games, or with `-join` that relay's; ADDRESS types another),
click the game in the list. Both then pick a race and a banner.

The game then reaches the main screen of turn 1 on both. Each presses
TURN; the first to press it hosts the turn, the other waits on the chat
screen until the host's broadcast. Battles are not ported yet (phase 4):
a battle ends the game with a "not ported" message.
