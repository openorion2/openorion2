package game

import (
	"encoding/binary"
	"errors"
	"slices"

	"github.com/openorion2/openorion2/internal/netplay"
)

// The network game's state and the Net_* API under it. The API
// (net/netcode.c) is reimplemented over internal/netplay; everything above
// it is ported from the assembly. Spec: docs/net/transport.md.

// _net (0x1aaec4) and the NETMOX.C data after it up to _in_receive_message
// (0x1ab0df) are kept as the original's bytes: the network code indexes them
// by player with values that may be -1. The addresses name the fields.
const (
	netBase       = 0x1aaec4
	netConnType   = 0x1aaf58 // _net+0x94: 0 network, 1 modem, 2 null modem
	netOwnID      = 0x1aaf5e // i16: this machine's net id
	netVote       = 0x1aaf76 // i8[8]: the host votes (msg 4)
	netReady      = 0x1aaf7e // u8[8]: who has pressed Turn
	netHostID     = 0x1aaf86 // i16: the net id hosting the turn
	netCombat     = 0x1aaf8b // [8][12]: Do_Net_Combat_Update_'s slots
	netCombatArg  = 0x1aaf96 // u8 at +0xb of each slot: the argument of msgs 6, 7, 9
	netState      = 0x1ab03f // u8: the network game's state
	netTalkTo     = 0x1ab04a // i8: the human of a human diplomacy (msg 0x44)
	netSend7      = 0x1ab050 // u8 bits per player: msg 7 due
	netSend6      = 0x1ab051 // msg 6 due
	netSend9      = 0x1ab052 // msg 9 due
	netSend1d     = 0x1ab053 // msg 0x1d due
	netSend22     = 0x1ab055 // u8[8]: per AI talker k, bit p: msg 0x22 due to p
	netPartner    = 0x1ab05d // u8[8]: the human who talked with k
	netSentVote   = 0x1ab07a // i8[8]: the vote last sent to each player
	netSend2b     = 0x1ab08b // u8[8]: msg 0x2b due
	netGets22     = 0x1ab094 // u8[8]: the player takes msg 0x22
	netLastHost   = 0x1ab09c // i16: last turn's host (the diplomacy arbiter)
	netActive     = 0x1ab09e // u8: a network game is set up
	netSent36     = 0x1ab09f // u8[8]: msg 0x36 sent to the player
	netSend39     = 0x1ab0a7 // u8[8]: msg 0x39 due
	netSend3d     = 0x1ab0af // u8[8]: msg 0x3d due
	netSend3f     = 0x1ab0b7 // u8[8]: msg 0x3f due
	netSend44     = 0x1ab0cb // i8: msg 0x44 due unless -1
	netInReceive  = 0x1ab0df // _in_receive_message
	netEnd        = 0x1ab0e0
	netSendErrorS = 60 + 10 + 80 + 4 // _net_send_error in RSTRING (Load_Netmox_Strings_)
)

// netGame is the network side of a Game.
type netGame struct {
	mem [netEnd - netBase]byte

	sending  bool  // 0x183608: Mox_Send_Message_ runs
	inUpdate bool  // 0x183609: Mox_Net_Update_ runs (its reentry guard)
	turn     uint8 // 0x18360a: Mox_Net_Update_'s round robin over the players
	reset    bool  // 0x184460: Reset_System_ has run

	prev []byte // the previous broadcast (Open_Modem_File_'s memory buffer)
	// port-only, for tests: where _msgs starts in the broadcast buffer
	msgsOff int
	recv    []byte // 0x1aaf72: the broadcast being received (msgs 2, 3)

	chat       *chatInfo  // _chat_info (0x1aa250): Net_Next_Turn_'s chat lines
	chatString [0x60]byte // _global_chat_string (0x1aac54)
	combatBusy bool       // 0x183579: Do_Net_Combat_Update_ runs
	resBuf     []byte     // 0x1ab04c: Client_Combat_Loop_'s record for msgs 0x16, 0x18
	tac        netTac     // the tactical lockstep's receive side

	tempString    string         // _temp_string of the net screens (Draw_SendGet_Net_Info_Screen_)
	name          [10]byte       // 0x1ab082: the game's name
	player        string         // port-only: Options.NetName, the player name for the transport
	numNetPlayers int32          // num_net_players (0x1849e2)
	noLobbyBoxes  bool           // port-only: -host/-join given, skip the create box
	box           *netAddressBox // port-only: the address box on screen, nil when none
	fadeIn        bool           // port-only: TCP skipped the initializing screen; the next one fades in

	// port-only: the Net_* layer over internal/netplay.
	network netplay.Network // Options.Net: what Net_Create/Join_Game_ reach
	// port-only: the network and player name a lobby game replaced (lobbyui.go).
	lobbySaved       netplay.Network
	lobbySavedPlayer string
	lobbyPlaying     bool
	ep               netplay.Endpoint // the joined game, nil before Net_*_Game_
	inited           bool             // Net_Init_ succeeded (netInitialized)
	connType         int32            // Net_Init_'s type (0x1bd150)
	reentrant        bool             // Net_Update_'s guard (0x18466c)
	receive          func(from, id uint16, buf []byte)
	games            []netplay.GameInfo                       // Net_Update_Game_List_'s netGameList
	log              func(NetRecord)                          // Options.NetLog
	caseHook         func(uint32)                             // tests: Main_Receive_Message_'s case address
	turnHook         func(host bool)                          // tests: after the host's broadcast and a client's decode
	sendingHook      func() bool                              // tests: IPX packets still unacknowledged
	preTurnHook      func()                                   // tests: the host's state before Next_Turn_ (battles set up)
	battleHook       func(sender, att, def, star, planet int) // tests: a full battle result decoded
	// tests: the host's state before a broadcast (the original's stale bytes)
	broadcastHook func()
	tacHook       func() // tests: after each lockstep barrier

	// port-only (phase 7, docs/net/hardening.md): lost players.
	dropEnd   bool           // Options.NetDropEnd: a lost player ends the game
	inGame    bool           // the game has started (past the lobby)
	dropping  int16          // the net id a sent-for drop-out is decoded for, 0 none
	dropped   [8]bool        // players turned AI by a lost link
	upDropped bool           // the host's uploader was lost: no upload to decode
	upLost    int            // tests: uploads given up for a lost uploader
	creator   string         // the creator's name, for the message when the relay is lost
	restart   bool           // the turn's host was lost mid-turn: the turn starts over
	restartAt uint32         // the stardate of the last restart
	notices   []string       // shown on the main screen
	check     bool           // Options.NetCheck
	checkDir  string         // Options.NetCheckDir
	checks    netCheckHashes // the desync check's hashes
	dropHook  func(p int)    // tests: a drop-out sent for a lost player was applied
	checkHook func(sd uint32, from uint16, same bool)
	abortHook func(msg string) // tests: the game ends with msg
}

func (n *netGame) u8(a uint32) uint8  { return n.mem[a-netBase] }
func (n *netGame) i32(a uint32) int32 { return int32(binary.LittleEndian.Uint32(n.mem[a-netBase:])) }
func (n *netGame) set32(a uint32, v int32) {
	binary.LittleEndian.PutUint32(n.mem[a-netBase:], uint32(v))
}
func (n *netGame) i8(a uint32) int8       { return int8(n.mem[a-netBase]) }
func (n *netGame) set8(a uint32, v uint8) { n.mem[a-netBase] = v }
func (n *netGame) i16(a uint32) int16 {
	return int16(binary.LittleEndian.Uint16(n.mem[a-netBase:]))
}
func (n *netGame) set16(a uint32, v int16) {
	binary.LittleEndian.PutUint16(n.mem[a-netBase:], uint16(v))
}

// port-only: NetRecord is a message sent or received (Options.NetLog): 'S'
// at Mox_Send_Message_, 'R' at Receive_Message_.
type NetRecord struct {
	ID, Dir, Peer, Len, Me uint16
	GameType               byte
	Stardate               uint32
	Payload                []byte
	Stale                  [][2]int // payload bytes [lo, hi) from stale stack
}

func (g *Game) netLog(dir, id, peer uint16, buf []byte, n uint16, stale []netSpan) {
	if g.net.log == nil {
		return
	}
	r := NetRecord{ID: id, Dir: dir, Peer: peer, Len: n, Me: uint16(g.net.i16(netOwnID)),
		GameType: byte(g.gameType), Stardate: g.state.Stardate,
		Payload: slices.Clone(buf[:min(int(n), len(buf))])}
	for _, sp := range stale {
		r.Stale = append(r.Stale, [2]int(sp))
	}
	g.net.log(r)
}

// port-only: ExitError ends the game as Exit_With_Message_ does (Reset_System_,
// the message printed, exit(1)); Run returns it.
type ExitError struct {
	Msg    string
	Notice bool // a port-only end (a lost player): Run shows Msg in a warning box first
}

func (e *ExitError) Error() string { return e.Msg }

// exitWithMessage is Exit_With_Message_ (0x126487) for the network code: it
// never returns (Run recovers the panic and returns the ExitError).
func (g *Game) exitWithMessage(msg string) {
	g.resetSystem()
	panic(&ExitError{Msg: msg})
}

// netBuildDate is the build date at at: msgs 0 and 0x2a carry the copy at
// netDate and are compared with the one at netDateOwn (both "May 22 2006").
// Port-only: the port's release follows the date ("May 22 2006 v1.0.0"),
// so that two releases do not play together (the date is every release's).
func (g *Game) netBuildDate(at uint32) string {
	d := cStringAt(g.exe, at) + " " + g.release
	return d[:min(len(d), 49)] // the messages' 50-byte buffer
}

// netExitWithMessage is Net_Exit_With_Message_ (0xf5934).
func (g *Game) netExitWithMessage(msg string) { g.exitWithMessage(msg) }

// resetSystem is Reset_System_ (0x113dbd), as far as the port has one: the
// first call closes the network (Net_Uninit_).
func (g *Game) resetSystem() {
	if !g.net.reset {
		g.netUninit()
	}
	g.net.reset = true
}

// netSendError is _net_send_error ("net send error!").
func (g *Game) netSendError() string {
	if rs, err := g.rStrings(); err == nil && len(rs) > netSendErrorS {
		return rs[netSendErrorS]
	}
	return "net send error!"
}

// netInit is Net_Init_ (0x138f3b): type 0 is the network (here the port's
// transport); the modem types are not ported and fail.
func (g *Game) netInit(typ int32, gameID int32, receive func(from, id uint16, buf []byte)) int32 {
	n := &g.net
	n.connType = typ
	if typ != 0 {
		return 0
	}
	if n.inited {
		return 1
	}
	if n.network == nil {
		return 0
	}
	n.inited = true
	n.receive = receive
	return 1
}

// netUninit is Net_Uninit_ (0x139062): leaves the game.
func (g *Game) netUninit() {
	n := &g.net
	if n.ep != nil {
		n.ep.Close()
		n.ep = nil
	}
	n.inited = false
}

// netUpdate is Net_Update_ (0x1390bb): the transport's pump. Each queued
// frame goes to the receive callback (Receive_Message_), on this goroutine.
func (g *Game) netUpdate() {
	n := &g.net
	if n.reentrant {
		return
	}
	n.reentrant = true
	defer func() { n.reentrant = false }()
	if n.ep == nil {
		return
	}
	for {
		f, ok := n.ep.Recv()
		if !ok {
			break
		}
		if f.ID >= netCtlDropOut && f.ID <= netCtlRestart {
			g.netPortFrame(f) // port-only (netdrop.go)
			continue
		}
		if n.receive != nil {
			n.receive(f.From, f.ID, f.Payload)
		}
	}
	g.netCheckLost()
}

// netIsSending is Net_Is_Sending_ (0x139127): sends are written at once. A
// test may say otherwise, as IPX does while a relay holds the packets.
func (g *Game) netIsSending() bool { return g.net.sendingHook != nil && g.net.sendingHook() }

// netSendMessage is Net_Send_Message_ (0x139153): 1 when sent, 0 without a
// link or when the transport refuses (the destination left, the link broke).
func (g *Game) netSendMessage(dest int32, id uint16, buf []byte) int32 {
	n := &g.net
	if n.connType != 0 || n.ep == nil {
		return 0
	}
	if err := n.ep.Send(uint16(dest), id, buf); err != nil {
		// port-only: as over IPX, a send to a station that is not there goes
		// nowhere; a lost player is turned into an AI (netdrop.go).
		if errors.Is(err, netplay.ErrNoUser) && !n.dropEnd && n.inGame {
			return 1
		}
		return 0
	}
	return 1
}

// netStatus is Net_Status_ (0x13920c), never called: bit 1 is "sending";
// bits 2 and 4 come from IPX internals and stay clear.
func (g *Game) netStatus() int32 { return 0 }

// netGetUsers is Net_Get_Users_ (0x139294): the station ids of the game's
// members (this one included, in id order) into list; returns the count.
func (g *Game) netGetUsers(list []byte) int32 {
	n := &g.net
	if n.ep == nil {
		return 0
	}
	ids := n.ep.Users()
	for i, id := range ids {
		if i < len(list) {
			list[i] = byte(id)
		}
	}
	return int32(len(ids))
}

// netIsUserValid is Net_Is_User_Valid_ (0x1393b6), never called.
func (g *Game) netIsUserValid(id int32) bool {
	return g.net.ep != nil && slices.Contains(g.net.ep.Users(), uint16(id))
}

// netHostID is Net_Host_ID_ (0x139447): this station's id.
func (g *Game) netHostID() int32 {
	if g.net.ep == nil {
		return 0
	}
	return int32(g.net.ep.ID())
}

// netUpdateGameList is Net_Update_Game_List_ (0x139476): the games that
// can be joined (at most 16, as netGameList holds).
func (g *Game) netUpdateGameList() int32 {
	n := &g.net
	n.games = nil
	if n.network == nil {
		return 0
	}
	games, err := n.network.Games()
	if err != nil {
		return 0
	}
	n.games = games[:min(len(games), 16)]
	return int32(len(n.games))
}

// netGetGameCount is Net_Get_Game_Count_ (0x139659).
func (g *Game) netGetGameCount() int32 { return int32(len(g.net.games)) }

// netGetGameName is Net_Get_Game_Name_ (0x13968a); names are char[10].
func (g *Game) netGetGameName(i int32) string {
	if i < 0 || int(i) >= len(g.net.games) {
		return ""
	}
	s := g.net.games[i].Name
	return s[:min(len(s), 9)]
}

// netGetGameStatus is Net_Get_Game_Status_ (0x1396f8): the open flag.
func (g *Game) netGetGameStatus(i int32) int32 {
	if i < 0 || int(i) >= len(g.net.games) || !g.net.games[i].Open {
		return 0
	}
	return 1
}

// netJoinGame is Net_Join_Game_ (0x13974f): 1 once joined.
func (g *Game) netJoinGame(name, password, player string) int32 {
	n := &g.net
	if !n.inited || n.network == nil {
		return 0
	}
	ep, err := n.network.Join(name, player)
	if err != nil {
		g.Log("Net_Join_Game_: %v", err)
		return 0
	}
	n.ep = ep
	return 1
}

// netCreateGame is Net_Create_Game_ (0x13982d): 1 once the game is up.
func (g *Game) netCreateGame(gameID int32, name string, numPlayers int32, password, player string) int32 {
	n := &g.net
	if !n.inited || n.network == nil {
		return 0
	}
	ep, err := n.network.Create(name, player)
	if err != nil {
		g.Log("Net_Create_Game_: %v", err)
		return 0
	}
	n.ep = ep
	return 1
}

// netGetPlayerCount is Net_Get_Player_Count_ (0x1398ea), never called.
func (g *Game) netGetPlayerCount() int32 { return g.netGetUsers(nil) }

// netEnableNewPlayers is Net_Enable_New_Players_ (0x13994e).
func (g *Game) netEnableNewPlayers() {
	if g.net.ep != nil {
		g.net.ep.SetOpen(true)
	}
}

// netDisableNewPlayers is Net_Disable_New_Players_ (0x13998f).
func (g *Game) netDisableNewPlayers() {
	if g.net.ep != nil {
		g.net.ep.SetOpen(false)
	}
}

// netNewPlayersEnabled is Net_New_Players_Enabled_ (0x1399d0), never called.
func (g *Game) netNewPlayersEnabled() bool {
	return g.net.ep != nil && g.net.ep.Game().Open
}

// port-only: recoverRunExit is recoverExit for Run: a port-only end
// (ExitError.Notice) is shown in a warning box before Run returns it.
func (g *Game) recoverRunExit(err *error) {
	r := recover()
	if r == nil {
		return
	}
	if ee, ok := r.(*ExitError); ok {
		*err = ee
		if ee.Notice {
			g.netEndBox(ee.Msg)
		}
		return
	}
	if e, ok := r.(error); ok && netQuit(e) {
		*err = e
		return
	}
	panic(r)
}

// netEndBox shows the reason a network game ended; a closed host or a
// failing screen just skips it.
func (g *Game) netEndBox(msg string) {
	defer func() { recover() }()
	if g.e == nil || g.e.Sys == nil || g.e.Sys.Closed() {
		return
	}
	g.net.chat = nil // the waiting screen is over
	g.e.ClearFields()
	g.e.DeactivateAutoFunction()
	g.warningBox(msg)
}

// port-only: recoverExit turns exitWithMessage's panic into Run's error;
// any other panic goes on.
func (g *Game) recoverExit(err *error) {
	r := recover()
	if r == nil {
		return
	}
	if ee, ok := r.(*ExitError); ok {
		*err = ee
		return
	}
	// The turn's steps pass a closed host up by panicking (dragon popup,
	// the network turn's waiting screens).
	if e, ok := r.(error); ok && netQuit(e) {
		*err = e
		return
	}
	panic(r)
}
