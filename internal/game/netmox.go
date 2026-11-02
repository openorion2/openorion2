package game

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/openorion2/openorion2/internal/save"
)

// NETMOX.C's message plumbing: sending, receiving and the per-update pump
// of a network game, ported from the assembly. Spec: docs/net/transport.md.

const (
	netAllHumans      = 0x7ffe            // Mox_Send_Message_'s "every human"
	mainReceiveScan   = 0xf594c           // Main_Receive_Message_'s 0x44 message ids (repne scasb)
	mainReceiveJumps  = 0xf598f           // and its jump table, indexed by what ecx is left
	mainReceiveReturn = 0xf5bf2           // the shared return: the ids without a case
	netPlayerSize     = 0xea9             // a player record
	net22Size         = 0x1da2            // msg 0x22: two player records and the stars' bytes
	net22Stars        = 4 + netPlayerSize // where msg 0x22's star bytes start
)

// netSpan is a payload's byte range [lo, hi) taken from stale stack.
type netSpan [2]int

// playerRecord is player k's 0xea9 bytes.
func (g *Game) playerRecord(k int) []byte {
	var b bytes.Buffer
	binary.Write(&b, binary.LittleEndian, &g.state.Players[k])
	return b.Bytes()
}

// netIDToPlayerID is Net_Id_To_Player_Id_ (0xfa16c): the player whose net
// id (player+0x2b) is id, searched from the last; -1 if none.
func (g *Game) netIDToPlayerID(id int32) int32 {
	for k := int32(g.state.NumPlayers) - 1; k >= 0; k-- {
		if int32(g.state.Players[k].NetID) == id {
			return k
		}
	}
	return -1
}

// receiveMessage is Receive_Message_ (0xf67c1), the callback Net_Init_
// registers: it routes by id, 2000 and up to diplomacy, 1000..1999 to
// tactical combat, the rest to Main_Receive_Message_.
func (g *Game) receiveMessage(from, id uint16, buf []byte) { g.receiveMessageStale(from, id, buf, nil) }

func (g *Game) receiveMessageStale(from, id uint16, buf []byte, stale []netSpan) {
	n := &g.net
	g.netLog('R', id, from, buf, uint16(len(buf)), stale)
	// The player index is only passed on to Main_Receive_Message_ (eax).
	p := g.netIDToPlayerID(int32(from))
	n.set8(netInReceive, 1)
	switch {
	case id >= 0x7d0:
		g.diplomacyReceiveMessage(from, id, buf)
	case id >= 0x3e8:
		g.tacticalCombatReceiveMessage(from, id, buf)
	default:
		g.mainReceiveMessage(p, from, id, buf)
	}
	n.set8(netInReceive, 0)
}

// netPump is the Sound_Update_ and guarded Net_Update_ pair that
// Mox_Send_Message_ and Basic_Update_ run around every step.
func (g *Game) netPump() {
	g.e.SoundUpdate()
	if g.net.u8(netInReceive) == 0 {
		g.netUpdate()
	}
}

// basicUpdate is Basic_Update_ (0xf5939).
func (g *Game) basicUpdate() { g.netPump() }

// moxSendMessage is Mox_Send_Message_ (0xf6816): a message to net id to.
// To oneself it goes straight to Receive_Message_; netAllHumans goes to
// every other human; a failed send ends the game (Exit_With_Message_).
func (g *Game) moxSendMessage(to int32, id uint16, buf []byte) int32 {
	return g.moxSendMessageStale(to, id, buf, nil)
}

func (g *Game) moxSendMessageStale(to int32, id uint16, buf []byte, stale []netSpan) int32 {
	n := &g.net
	g.netLog('S', id, uint16(to), buf, uint16(len(buf)), stale)
	r := int32(1)
	n.sending = true
	if len(buf) == 0 {
		// One byte of the routine's frame ([ebp-4]) is sent instead.
		buf, stale = []byte{0}, []netSpan{{0, 1}}
	}
	g.netPump()
	own := n.i16(netOwnID)
	if int32(own) == to {
		g.receiveMessageStale(uint16(to), id, buf, stale)
		n.sending = false
		return 1
	}
	none := false
	for {
		g.netPump()
		if !g.netIsSending() {
			break
		}
	}
	players := g.state.Players[:]
	if to == netAllHumans {
		count, last := 0, int32(-1)
		for k := int(g.state.NumPlayers) - 1; k >= 0; k-- {
			nid := int32(players[k].NetID)
			if nid == -1 || uint16(nid) == uint16(own) {
				continue
			}
			count++
			last = nid
		}
		if count == 0 {
			none = true
		} else if count == 1 {
			to = last
		}
	}
	if !none {
		if n.u8(netConnType) == 0 && to == netAllHumans {
			for k := int(g.state.NumPlayers) - 1; k >= 0; k-- {
				nid := players[k].NetID
				if nid <= 0 || uint16(nid) == uint16(own) {
					continue
				}
				g.netPump()
				r = g.netSendMessage(int32(nid), id, buf)
				g.netPump()
				if r == 0 {
					g.exitWithMessage(g.netSendError())
				}
			}
		} else {
			g.netPump()
			r = g.netSendMessage(to, id, buf)
			g.netPump()
		}
	}
	if r == 0 {
		g.exitWithMessage(g.netSendError())
	}
	n.sending = false
	return r
}

// mainReceiveMessage is Main_Receive_Message_ (0xf5a9f): ids up to 0x64 are
// looked up in its id table and jump to their case.
func (g *Game) mainReceiveMessage(player int32, from, id uint16, buf []byte) {
	c := uint32(mainReceiveReturn)
	if id <= 0x64 {
		ecx := uint32(0x44)
		for _, b := range g.exe.bytes(mainReceiveScan, 0x44) {
			ecx--
			if b == byte(id) {
				break
			}
		}
		c = g.exe.u32(mainReceiveJumps + 4*ecx)
	}
	if c == mainReceiveReturn {
		return
	}
	if g.net.caseHook != nil {
		g.net.caseHook(c)
	}
	if !g.mainReceiveCase(c, player, from, buf) {
		g.Log("Main_Receive_Message_: msg %#x from net id %d (player %d, case %#x) not ported", id, from, player, c)
	}
}

// mainReceiveCase runs Main_Receive_Message_'s case at address c (the
// cases of the lobby and the start, phase 2); false for the others.
func (g *Game) mainReceiveCase(c uint32, player int32, from uint16, buf []byte) bool {
	n, s, rs := &g.net, &g.state, &g.rsel
	sent := buf
	// A short payload reads on in the receive buffer: zeros here.
	buf = append(append([]byte(nil), buf...), make([]byte, max(0, netMsg0Size-len(buf)))...)
	b := func(i int) byte { return buf[i] }
	if player < 0 {
		// The original indexes player -1; no sender outside the game in the port.
		switch c {
		case 0xf5bfe, 0xf6541, 0xf661c, 0xf6676, 0xf66ec, 0xf5ce2, 0xf5f02, 0xf662b:
			return true
		}
	}
	switch c {
	case 0xf5adc: // msg 0: the game's setup
		s.NumPlayers = int16(b(0x28))
		g.settings[2] = b(0x26)
		copy(g.settings[0xd4:0xda], buf[0x27:0x2d])
		g.settings[0xd3] = b(0x2d)
		ours := g.netBuildDate(netDateOwn)
		if theirs := save.CString(buf[0x2e:]); theirs != ours {
			g.exitWithMessage(fmt.Sprintf(cFormat(g.netRString(netVersionsS, "Incompatible program versions (host %s, you %s)")), theirs, ours))
		}
		s.PlayerNum = -1
		n.set16(netGameHost, int16(binary.LittleEndian.Uint16(buf)))
		n.set8(netHumans, b(4))
		for i := 0; i < int(s.NumPlayers); i++ {
			id := binary.LittleEndian.Uint32(buf[6+4*i:])
			s.Players[i].NetID = int16(id)
			if int32(n.i16(netOwnID)) == int32(id) {
				s.PlayerNum = int16(i)
			}
		}
		if s.PlayerNum != -1 {
			n.set8(netState, 6)
		} else {
			n.set8(netState, 0x2d)
		}
	case 0xf5bfe: // msg 1: a joiner's player record after his race pick
		n.set8(netRaceInfo+uint32(player), 1)
		rec := g.playerRecord(int(player))
		copy(rec, sent)
		binary.Read(bytes.NewReader(rec), binary.LittleEndian, &s.Players[player])
		for k := int(s.NumPlayers) - 1; k >= 0; k-- {
			if k != int(player) && s.Players[k].Color == s.Players[player].Color {
				s.Players[player].Color = 8
			}
		}
	case 0xf5c5c: // msg 2: a broadcast of {chunks} follows
		n.set16(netChunks, int16(binary.LittleEndian.Uint16(buf)))
		n.set16(netRecvChunks, 0)
		binary.LittleEndian.PutUint32(n.mem[netRecvLen-netBase:], 0)
		n.recv = make([]byte, netBlockSize)
		n.set16(netLastHost, int16(from))
		g.netInitHostElection()
	case 0xf5cb0: // msg 3: a chunk
		if n.recv == nil {
			n.recv = make([]byte, netBlockSize) // port-only: no msg 2 came first
		}
		copy(n.recv[int(n.i16(netRecvChunks))*netChunk:], sent)
		n.set16(netRecvChunks, n.i16(netRecvChunks)+1)
		l := binary.LittleEndian.Uint32(n.mem[netRecvLen-netBase:])
		binary.LittleEndian.PutUint32(n.mem[netRecvLen-netBase:], l+uint32(len(sent)))
	case 0xf5ce2: // msg 4 {s32 vote, ready}: the lowest vote wins
		if v := int8(b(0)); v >= 0 && n.dropped[v&7] {
			// port-only: a vote for a lost player, sent before he was lost.
			n.set8(netVote+uint32(player), 0xff)
			n.set8(netReady+uint32(player), b(4))
			break
		}
		n.set8(netVote+uint32(player), b(0))
		n.set8(netReady+uint32(player), b(4))
		me := uint32(int32(s.PlayerNum))
		if n.i8(netVote+me) == -1 || n.i8(netVote+uint32(player)) < n.i8(netVote+me) {
			n.set8(netVote+me, n.u8(netVote+uint32(player)))
		}
	case 0xf5f02: // msg 0xb: the client is ready to upload
		n.set8(netWantsSend+uint32(player), 1)
	case 0xf5f11: // msg 0xc: the host takes our upload now
		n.set8(netState, 0xd)
	case 0xf5f1d: // msg 0xd {u16 chunks}: an upload follows
		n.set16(netChunks, int16(binary.LittleEndian.Uint16(buf)))
		n.set16(netRecvChunks, 0)
		binary.LittleEndian.PutUint32(n.mem[netRecvLen-netBase:], 0)
		n.recv = make([]byte, netBlockSize)
	case 0xf5f63: // msg 0xe: a chunk of the upload
		if n.recv == nil {
			n.recv = make([]byte, netBlockSize) // port-only: no msg 0xd came first
		}
		copy(n.recv[int(n.i16(netRecvChunks))*netChunk:], sent)
		n.set16(netRecvChunks, n.i16(netRecvChunks)+1)
		l := binary.LittleEndian.Uint32(n.mem[netRecvLen-netBase:])
		binary.LittleEndian.PutUint32(n.mem[netRecvLen-netBase:], l+uint32(len(sent)))
		if n.i16(netRecvChunks) == n.i16(netChunks) {
			n.set8(netUploaded, n.u8(netUploaded)+1)
			if int16(n.u8(netUploaded)) == s.NumPlayers {
				n.set8(netState, 0xb)
			} else {
				n.set8(netState, 9)
			}
		}
	case 0xf610d: // msg 0x1a: the combat phase is over
		n.set8(netState, 7)
	case 0xf648e: // msg 0x27: a chat line
		if n.chat != nil {
			g.receiveChatMsg(byte(player), sent)
		}
	case 0xf657d: // msg 0x2d: a GNN chat line
		if n.chat != nil {
			g.receiveChatMsg(8, sent)
		}
	case 0xf633d: // msg 0x20: pick your empire of the loaded game
		n.set8(netState, 0x2c)
		n.set16(netGameHost, int16(from))
		s.NumPlayers = int16(binary.LittleEndian.Uint16(buf))
		for k := 0; k < int(s.NumPlayers); k++ {
			rec := g.playerRecord(k)
			copy(rec, sent[min(len(sent), 2+k*netPlayerSize):])
			setRec(&s.Players[k], rec)
		}
	case 0xf637c: // msg 0x21 {s32 player}: the joiner's pick
		k := int32(binary.LittleEndian.Uint32(buf))
		if k >= 0 && int(k) < len(s.Players) { // the original writes player -1 too
			s.Players[k].NetID = int16(from)
		}
		n.set32(netPicking, -1)
	case 0xf64a8: // msg 0x2a: the loaded game's net ids
		ours := g.netBuildDate(netDateOwn)
		if theirs := save.CString(buf[0x2e:]); theirs != ours {
			g.exitWithMessage(fmt.Sprintf(cFormat(g.netRString(netVersionsS, "Incompatible program versions (host %s, you %s)")), theirs, ours))
		}
		// The count byte is decremented in place: a count of 0 would run
		// 255 times past the players (not modelled).
		for c := buf[0x28]; ; {
			c--
			if int(c) < len(s.Players) {
				s.Players[c].NetID = int16(binary.LittleEndian.Uint16(buf[6+4*int(c):]))
			}
			if c == 0 {
				break
			}
		}
	case 0xf662b: // msgs 0x37, 0x38: the player quit or surrendered
		g.decodeDropOutInfo(int(player), buf)
	case 0xf66a7: // msg 0x3b: we are eliminated
		s.Players[s.PlayerNum].Eliminated = 1
	case 0xf651d: // msg 0x2b {banner, player}: the creator's decision
		rs.colorFlag[b(0)&7] = true
		s.Players[b(1)].Color = b(0)
	case 0xf6541: // msg 0x2c {banner}: a request, on the creator
		if !rs.colorFlag[b(0)&7] {
			rs.colorFlag[b(0)&7] = true
			s.Players[player].Color = b(0)
			n.set8(netSend2b+uint32(player), 1)
		}
	case 0xf661c: // msg 0x36: the player is ready for msg 0x22
		n.set8(netGets22+uint32(player), 1)
	case 0xf6638: // msg 0x39 {race, player}: the creator's decision
		if n.i16(netGameHost) == n.i16(netOwnID) {
			g.exitWithMessage(g.netRString(netOneHostS, "only 1 host allowed"))
		}
		rs.raceFlag[b(0)] = true
		s.Players[b(1)].Race = b(0)
	case 0xf6676: // msg 0x3a {race}: a request, on the creator
		if !rs.raceFlag[b(0)] {
			rs.raceFlag[b(0)] = true
			s.Players[player].Race = b(0)
			n.set8(netSend39+uint32(player), 1)
		}
	case 0xf66dd: // msg 0x3d {race}: a race given up
		rs.raceFlag[b(0)] = false
	case 0xf66ec: // msg 0x3e {race}: given up, on the creator
		rs.raceFlag[b(0)] = false
		n.set8(netSend3d+uint32(player), 1)
	case 0xf6705: // msg 0x3f: the give-up is through
		n.set8(netRaceFlagWait, 0)
	case 0xf6729: // msg 0x42 {u32}: a joiner without nebulas
		if binary.LittleEndian.Uint32(buf) == 0 {
			n.set8(netNebulasOK, 0)
		}
	default:
		if g.netDiploCase(c, player, from, buf) {
			return true
		}
		return g.netCombatCase(c, player, buf, sent)
	}
	return true
}

// netInitHostElection is Net_Init_Host_Election_ (0xfc6a5): no votes, nobody
// ready, no host; the pending msg 0x44 and the 0x22 takers are cleared.
func (g *Game) netInitHostElection() {
	n := &g.net
	for i := uint32(0); i < 8; i++ {
		n.set8(netVote+i, 0xff)
		n.set8(netSentVote+i, 0xff)
		n.set8(netReady+i, 0)
		n.set8(0x1ab06a+i, 0)
		n.set8(0x1ab072+i, 0)
		n.set8(netGets22+i, 0)
	}
	n.set16(netHostID, -1)
	n.set8(0x1aaf88, 0xff)
	n.set8(0x1aaf8a, 0)
	n.set8(netSend44, 0xff)
}

// moxNetUpdate is Mox_Net_Update_ (0xf69fe), run by every Mox_Update_ in
// network games: Net_Update_, then at most one pending message to the
// player whose turn it is in the round robin.
func (g *Game) moxNetUpdate() {
	n := &g.net
	if g.gameType != 2 && g.gameType != 3 {
		return
	}
	if n.inUpdate || n.u8(netInReceive) != 0 {
		return
	}
	n.inUpdate = true
	defer func() { n.inUpdate = false }()
	g.netUpdate()
	if n.u8(netActive) == 0 {
		return
	}
	s := &g.state
	p := int32(-1)
	if s.NumPlayers != 0 {
		p = int32(n.turn) % int32(s.NumPlayers)
	}
	g.e.MoxUpdate() // Mox_Update_: the sound; the reentry guard stops the rest
	n.turn++
	if p == -1 || g.netIsSending() {
		return
	}
	bit := uint8(1) << uint(p)
	netID := func(k int32) int32 { return int32(s.Players[k].NetID) }
	sent := false
	if st := n.u8(netState); (st == 4 || st == 8) && n.u8(netSent36+uint32(p)) == 0 {
		n.set8(netSent36+uint32(p), 1)
		if netID(p) != -1 {
			sent = true
			g.moxSendMessage(netID(p), 0x36, nil)
		}
	}
	if !sent && n.u8(netSend7)&bit != 0 {
		arg := []byte{n.u8(netCombatArg + 12*uint32(p))}
		n.set8(netSend7, n.u8(netSend7)&^bit)
		sent = true
		g.moxSendMessage(netID(p), 7, arg)
	}
	// Msg 0x22: an AI's player record after a talk, for the other humans.
	for k := int32(s.NumPlayers) - 1; k >= 0; k-- {
		if n.u8(netSend22+uint32(k))&bit == 0 {
			continue
		}
		partner := n.u8(netPartner + uint32(k))
		buf := make([]byte, net22Size)
		binary.LittleEndian.PutUint32(buf, uint32(k))
		copy(buf[4:], g.playerRecord(int(k)))
		ns := int(s.NumStars)
		for i := ns - 1; i >= 0; i-- {
			buf[net22Stars+i] = byte(s.Stars[i].SurrenderTo[k])
		}
		binary.LittleEndian.PutUint32(buf[net22Stars+72:], uint32(partner))
		copy(buf[net22Stars+72+4:], g.playerRecord(int(partner)))
		if int32(s.PlayerNum) != p && int32(partner) != p && s.Players[p].Objective == 100 &&
			!sent && n.u8(netGets22+uint32(p)) != 0 {
			sent = true
			var stale []netSpan
			if ns < 72 { // the rest of the 72 bytes is the frame's old stack
				stale = []netSpan{{net22Stars + ns, net22Stars + 72}}
			}
			g.moxSendMessageStale(netID(p), 0x22, buf, stale)
		}
		n.set8(netSend22+uint32(k), n.u8(netSend22+uint32(k))&^bit)
	}
	if !sent && n.u8(netSend6)&bit != 0 {
		arg := n.i8(netCombatArg + 12*uint32(p))
		// The talker's index is signed: -1 reads the byte before the table.
		if (n.u8(uint32(int32(netSend22)+int32(arg))))&bit == 0 {
			n.set8(netSend6, n.u8(netSend6)&^bit)
			sent = true
			g.moxSendMessage(netID(p), 6, []byte{byte(arg)})
		}
	}
	if !sent && n.u8(netSend9)&bit != 0 {
		arg := []byte{n.u8(netCombatArg + 12*uint32(p))}
		n.set8(netSend9, n.u8(netSend9)&^bit)
		sent = true
		g.moxSendMessage(netID(p), 9, arg)
	}
	if !sent && n.u8(netSend1d)&bit != 0 {
		n.set8(netSend1d, n.u8(netSend1d)&^bit)
		sent = true
		g.moxSendMessage(netID(p), 0x1d, nil)
	}
	me := int32(s.PlayerNum)
	if !sent && me != p && s.Players[p].Objective == 100 &&
		n.i8(netSentVote+uint32(p)) != n.i8(uint32(int32(netVote)+me)) {
		vote := n.i8(uint32(int32(netVote) + me))
		buf := make([]byte, 5)
		binary.LittleEndian.PutUint32(buf, uint32(int32(vote)))
		buf[4] = n.u8(uint32(int32(netReady) + me))
		n.set8(netSentVote+uint32(p), uint8(vote))
		sent = true
		g.moxSendMessage(netID(p), 4, buf)
	}
	if !sent && n.i16(netHostID) == n.i16(netOwnID) {
		if st := n.u8(netState); st >= 0xe && st < 0x23 {
			g.doNetCombatUpdate()
		}
	}
	if !sent && n.u8(netSend2b+uint32(p)) != 0 {
		buf := []byte{s.Players[p].Color, byte(p)}
		n.set8(netSend2b+uint32(p), 0)
		sent = true
		g.moxSendMessage(netAllHumans, 0x2b, buf)
	}
	if !sent && n.u8(netSend39+uint32(p)) != 0 {
		buf := []byte{s.Players[p].Race, byte(p)}
		n.set8(netSend39+uint32(p), 0)
		sent = true
		g.moxSendMessage(netAllHumans, 0x39, buf)
	}
	if !sent && n.u8(netSend3d+uint32(p)) != 0 {
		buf := []byte{s.Players[p].Race}
		n.set8(netSend3d+uint32(p), 0)
		n.set8(netSend3f+uint32(p), 1)
		sent = true
		g.moxSendMessage(netAllHumans, 0x3d, buf)
	}
	if !sent && n.u8(netSend3f+uint32(p)) != 0 {
		n.set8(netSend3f+uint32(p), 0)
		sent = true
		g.moxSendMessage(netID(p), 0x3f, nil)
	}
	if !sent && n.i8(netSend44) != -1 {
		to := netID(int32(n.i8(netTalkTo)))
		n.set8(netSend44, 0xff)
		g.moxSendMessage(to, 0x44, g.playerRecord(int(me)))
	}
}
