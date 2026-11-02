package game

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/openorion2/openorion2/internal/host"
	"github.com/openorion2/openorion2/internal/save"
)

// The turn of a network game (NETTURN.C and NETMOX.C): the host election,
// the clients' uploads, the host's Next_Turn_ and its broadcast, ported
// from the assembly. Spec: docs/net/turn.md.

const (
	ScreenNetNextTurn = 0x25 // Net_Next_Turn_

	netUploader   = 0x1aaf88 // i8: the player whose upload the host takes now
	netUploaded   = 0x1aaf89 // u8: players whose upload is in (or need none)
	netHasUpload  = 0x1ab06a // u8[8]: the player's upload is in
	netWantsSend  = 0x1ab072 // u8[8]: the player sent msg 0xb (ready to upload)
	netTalker22   = 0x1ab054 // u8: the byte before netSend22 (talker -1)
	netTurnFlag68 = 0x1ab068 // u8: cleared after Next_Turn_ (read by the diplomacy code)
	netTurnFlag69 = 0x1ab069 // u8: cleared around Net_Next_Turn_
	netPickDone   = 0x1ab04b // u8: Do_Net_Combat_Update_ found no star to fight at
	netResult48   = 0x1ab048 // i8: set -1 with the last chunk of the upload
	netTurnFirstS = 167      // RSTRING: Load_Netturn_Strings_' first ("waiting ..."), 0x1ab108
	netTurnRecvS  = 168      // its second, 0x1ab104: "have %d of %d chunks\n"
	netOrdersSize = 0x55730  // Send_Game_Orders_' Push_Block_
	netStarOrder  = 0x18     // an upload's entry per star
	netStarOrders = 0x6c0    // the entries' frame buffer (72 stars)
	netCraziness  = 0x17a18c // "craziness"
)

// The star bitmasks an upload carries, bit k of the entry from byte
// starOrderBits[k] of the star (the sender's bit only).
var starOrderBits = [10]int{0x35, 0x36, 0x38, 0x39, 0x3b, 0x3c, 0x3d, 0x3e, 0x3f, 0x40}

// recBytes and setRec move a save record to and from its bytes.
func recBytes(v any) []byte {
	var b bytes.Buffer
	if err := binary.Write(&b, binary.LittleEndian, v); err != nil {
		panic(err)
	}
	return b.Bytes()
}

func setRec(v any, b []byte) {
	if err := binary.Read(bytes.NewReader(b), binary.LittleEndian, v); err != nil {
		panic(err)
	}
}

// netNextTurn is Net_Next_Turn_ (0xfc470), screen 0x25: Turn pressed in a
// network game. The vote (msg 4), the election, then the host's or the
// client's turn; the reports follow.
func (g *Game) netNextTurn() error {
	s, n := &g.state, &g.net
	me := int(s.PlayerNum)
	if s.Players[me].SurrenderTo != -1 {
		if err := g.humanSurrenders(me); err != nil {
			return err
		}
		g.moxNetUninit()
		g.currentScreen, g.returnScreen, g.previousScreen = ScreenMainMenu, ScreenMainMenu, ScreenMainMenu
		return nil
	}
	n.set8(netTurnFlag69, 0)
	n.set8(netTalker22, 0)
	n.chat = &chatInfo{}
	if err := g.loadNetNextTurnScreen(false); err != nil {
		return err
	}
	g.e.AssignAutoFunction(g.drawNetNextTurnScreen, 1)
	if err := g.addNetNextTurnFields(); err != nil {
		return err
	}
	n.set8(netState, 8)
	n.set8(netReady+uint32(me), 1)
	if n.i8(netVote+uint32(me)) == -1 {
		n.set8(netVote+uint32(me), uint8(me))
	}
	for {
		again, err := g.netElectAndPlay()
		if err != nil {
			return err
		}
		if !again {
			break
		}
		// port-only: the turn's host was lost mid-turn (netdrop.go).
		g.netRestartTurnState()
	}
	n.set8(netTalker22, 0)
	n.set8(netTurnFlag69, 0)
	n.chat = nil
	if s.Players[s.PlayerNum].Eliminated == 0 {
		g.moxPlaySound(0x33)
		g.currentScreen = ScreenReports
	} else {
		// port-only: the end, in the log (tools/nettest reads it).
		g.Log("network game over at stardate %d: player %d eliminated", s.Stardate, s.PlayerNum)
		if err := g.playFadedFlics(0, eLoserFlic); err != nil { // Play_Loose_Flic_
			return err
		}
		for g.netIsSending() {
			g.e.MoxUpdate()
		}
		g.moxNetUninit()
		g.gameType = 0
		g.currentScreen = ScreenMainMenu
	}
	g.previousScreen = ScreenNetNextTurn
	return nil
}

// netElectAndPlay is Net_Next_Turn_ from its msgs 4 to the end of the turn;
// again (port-only): the turn's host was lost, the turn starts over.
func (g *Game) netElectAndPlay() (again bool, err error) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(netRestartTurn); !ok {
				panic(r)
			}
			again = true
		}
	}()
	s, n := &g.state, &g.net
	me := int(s.PlayerNum)
	for k := int(s.NumPlayers) - 1; k >= 0; k-- {
		if s.Players[k].Objective == save.ObjectiveHuman && k != me {
			buf := make([]byte, 5)
			binary.LittleEndian.PutUint32(buf, uint32(int32(n.i8(netVote+uint32(me)))))
			buf[4] = 1
			g.moxSendMessage(int32(s.Players[k].NetID), 4, buf)
		}
		if err := g.chatBoxInputLoop(); err != nil {
			return false, err
		}
	}
	// The election: every human holds the same vote.
	for {
		host := int32(-1)
		for k := int(s.NumPlayers) - 1; k >= 0; k-- {
			if s.Players[k].Objective != save.ObjectiveHuman {
				continue
			}
			v := int32(n.i8(netVote + uint32(k)))
			if v == -1 {
				host = -1
				break
			}
			if host == -1 {
				host = v
			} else if host != v {
				host = -1
				break
			}
		}
		if host != -1 {
			n.set16(netHostID, s.Players[host].NetID)
		}
		if err := g.chatBoxInputLoop(); err != nil {
			return false, err
		}
		if n.i16(netHostID) != -1 {
			break
		}
	}
	if n.i16(netHostID) == n.i16(netOwnID) {
		return false, g.hostNextTurn()
	}
	return false, g.clientNextTurn()
}

// hostNextTurn is Host_Next_Turn_ (0xfbfe2): take each client's upload in
// turn (msg 0xc, then Decode_Game_Orders_), run Next_Turn_, tell the
// eliminated (msg 0x3b) and broadcast the result.
func (g *Game) hostNextTurn() error {
	s, n := &g.state, &g.net
	me := int(s.PlayerNum)
	n.set8(netUploaded, 0)
	for k := int(s.NumPlayers) - 1; k >= 0; k-- {
		pl := &s.Players[k]
		if pl.Objective != save.ObjectiveHuman || k == me || pl.Eliminated != 0 {
			n.set8(netUploaded, n.u8(netUploaded)+1)
			n.set8(netHasUpload+uint32(k), 1)
		}
	}
	if int16(n.u8(netUploaded)) < s.NumPlayers {
		n.set8(netState, 9)
	} else {
		n.set8(netState, 0xb)
	}
	for n.u8(netState) != 0xb {
		for k := 0; ; {
			if err := g.chatBoxInputLoop(); err != nil {
				return err
			}
			if n.u8(netReady+uint32(k)) != 0 && n.u8(netWantsSend+uint32(k)) != 0 {
				n.set8(netWantsSend+uint32(k), 0)
				n.set8(netUploader, uint8(k))
			}
			k++
			if n.i8(netUploader) != -1 || k >= int(s.NumPlayers) {
				break
			}
		}
		u := int(n.i8(netUploader))
		if u == -1 {
			continue
		}
		n.set8(netState, 0xa)
		g.moxSendMessage(int32(s.Players[u].NetID), 0xc, nil)
		for n.u8(netState) == 0xa {
			if err := g.chatBoxInputLoop(); err != nil {
				return err
			}
		}
		if n.upDropped {
			// port-only: the uploader was lost (netdrop.go); nothing to decode.
			n.upDropped = false
			n.upLost++
			n.recv = nil
			n.set16(netChunks, 0)
			continue
		}
		g.decodeGameOrders(n.recv, int(binary.LittleEndian.Uint32(n.mem[netRecvLen-netBase:])))
		n.recv = nil // Pop_Block_
		n.set16(netChunks, 0)
		n.set8(netHasUpload+uint32(u), 1)
		n.set8(netUploader, 0xff)
	}
	g.updateShipIconsInBetweenTurns()
	// Rebuild_WFI_List_ (0xe5407) is a bare ret.
	n.set8(netPickDone, 0)
	for i := uint32(0); i < 0xb4; i++ {
		n.set8(netCombat+i, 0)
	}
	for k := int(s.NumPlayers) - 1; k >= 0; k-- {
		n.set8(netCombat+12*uint32(k)+9, 0xff)
		n.set8(netCombat+12*uint32(k), 0xe)
	}
	n.set8(netState, 0xe)
	if n.preTurnHook != nil {
		n.preTurnHook()
	}
	if err := g.nextTurn(); err != nil {
		return err
	}
	n.set16(netLastHost, n.i16(netOwnID))
	g.netInitHostElection()
	n.set8(netTurnFlag68, 0)
	for k := int(s.NumPlayers) - 1; k >= 0; k-- {
		if k == me {
			continue
		}
		pl := &s.Players[k]
		if pl.NetID == -1 || pl.Eliminated == 0 {
			continue
		}
		g.moxSendMessage(int32(pl.NetID), 0x3b, nil)
		pl.Objective, pl.NetID = 0, -1
		n.set8(netHumans, n.u8(netHumans)-1)
		g.e.MoxUpdate()
	}
	if err := g.broadcastGameDataDifferential(false); err != nil {
		return err
	}
	if n.turnHook != nil {
		n.turnHook(true)
	}
	g.netCheckState()    // port-only: the desync check (-netcheck)
	g.netKeepTurnStart() // port-only: a lost player's return goes back here
	if s.Players[me].Eliminated != 0 {
		return nil
	}
	n.set8(netGets22+uint32(me), 1)
	for i := uint32(0); i < 8; i++ {
		n.set8(netSent36+i, 0)
	}
	n.set8(netState, 4)
	for k := int(s.NumPlayers) - 1; k >= 0; k-- {
		n.set8(netCombat+12*uint32(k), 4)
		n.set8(netCombatArg+12*uint32(k), 0xff)
	}
	g.updateShipIconsInBetweenTurns()
	if n.u8(netHumans) == 1 {
		g.Log("network game goes on alone at stardate %d (game type 0)", s.Stardate) // port-only
		g.moxNetUninit()
		g.gameType = 0
		s.SomethingHappened[me] = 1 // 0x1ab124
	}
	return nil
}

// mustBroadcastAIPlayerData is Must_Broadcast_AI_Player_Data_ (0xfc299):
// a msg 0x22 is still due to some player.
func (g *Game) mustBroadcastAIPlayerData() bool {
	n, np := &g.net, int(g.state.NumPlayers)
	for k := np - 1; k >= 0; k-- {
		for p := np - 1; p >= 0; p-- {
			if n.u8(netSend22+uint32(k))>>uint(p)&1 != 0 {
				return true
			}
		}
	}
	return false
}

// clientNextTurn is Client_Next_Turn_ (0xfc2d2): msg 0xb to the host, the
// upload once it says so (msg 0xc), the battles, then its broadcast.
func (g *Game) clientNextTurn() error {
	s, n := &g.state, &g.net
	g.moxSendMessage(int32(n.i16(netHostID)), 0xb, nil)
	for {
		if err := g.chatBoxInputLoop(); err != nil {
			return err
		}
		if n.u8(netState) != 8 {
			break
		}
	}
	for g.mustBroadcastAIPlayerData() {
		if err := g.chatBoxInputLoop(); err != nil {
			return err
		}
	}
	if err := g.sendGameOrders(); err != nil {
		return err
	}
	if err := g.clientCombatLoop(); err != nil {
		return err
	}
	n.tempString = g.netRString(netTurnFirstS, "waiting ...")
	eliminated := func() bool { return s.Players[s.PlayerNum].Eliminated != 0 }
	for n.i16(netChunks) == 0 && !eliminated() {
		if err := g.chatBoxInputLoop(); err != nil {
			return err
		}
	}
	if eliminated() {
		return nil
	}
	for n.i16(netRecvChunks) < n.i16(netChunks) {
		n.tempString = fmt.Sprintf(cFormat(g.netRString(netTurnRecvS, "have %d of %d chunks\n")),
			int(n.i16(netRecvChunks)), int(n.i16(netChunks)))
		if err := g.chatBoxInputLoop(); err != nil {
			return err
		}
	}
	// The flag is a frame variable set to 0 and never changed: a diff.
	if !g.decodeBroadcastGameDataDifferential(n.recv, int(binary.LittleEndian.Uint32(n.mem[netRecvLen-netBase:])), false) {
		g.exitWithMessage(cStringAt(g.exe, netCraziness))
	}
	if n.turnHook != nil {
		n.turnHook(false)
	}
	g.netCheckState()    // port-only: the desync check (-netcheck)
	g.netKeepTurnStart() // port-only: a lost player's return goes back here
	for i := uint32(0); i < 8; i++ {
		n.set8(netSent36+i, 0)
	}
	n.recv = nil // Pop_Block_
	n.set16(netChunks, 0)
	n.set8(netState, 4)
	g.updateShipIconsInBetweenTurns()
	if n.u8(netHumans) == 1 {
		g.Log("network game goes on alone at stardate %d (game type 0)", s.Stardate) // port-only
		g.moxNetUninit()
		g.gameType = 0
	}
	return nil
}

// sendGameOrders is Send_Game_Orders_ (0xf7e95): everything the player
// owns, as msg 0xd {chunks} and msgs 0xe of up to 0x7800 bytes to the host.
func (g *Game) sendGameOrders() error {
	s, n := &g.state, &g.net
	me := int(s.PlayerNum)
	b := binary.LittleEndian.AppendUint32(nil, s.Stardate)
	b = append(b, byte(me))
	b = append(b, g.playerRecord(me)...)
	g.e.MoxUpdate()
	ent := make([]byte, netStarOrders)
	ns := int(s.NumStars)
	for i := ns - 1; i >= 0; i-- {
		st := recBytes(&s.Stars[i])
		e := ent[i*netStarOrder:]
		var bits uint16
		for k, off := range starOrderBits {
			bits |= uint16(st[off]>>uint(me)&1) << uint(k)
		}
		binary.LittleEndian.PutUint16(e, binary.LittleEndian.Uint16(e)|bits)
		binary.LittleEndian.PutUint32(e[0x13:], uint32(int32(int8(st[0x42+me]))))
		if int(int8(st[0x37])) == me {
			copyCStr(e[4:], st)
		}
		e[0x17] = st[0x67+me]
	}
	b = append(b, ent[:ns*netStarOrder]...)
	g.e.MoxUpdate()
	b = binary.LittleEndian.AppendUint16(b, uint16(s.NumColonies))
	for i := int(s.NumColonies) - 1; i >= 0; i-- {
		if int(s.Colonies[i].Owner) == me {
			b = binary.LittleEndian.AppendUint16(b, uint16(i))
			b = append(b, recBytes(&s.Colonies[i])...)
		}
	}
	b = binary.LittleEndian.AppendUint16(b, 0xffff)
	g.e.MoxUpdate()
	b = binary.LittleEndian.AppendUint16(b, uint16(s.NumShips))
	for i := 0; i < int(s.NumShips); i++ {
		if sh := &s.Ships[i]; int(sh.Owner) == me && sh.Status != 5 {
			b = binary.LittleEndian.AppendUint16(b, uint16(i))
			b = append(b, recBytes(sh)...)
		}
	}
	b = binary.LittleEndian.AppendUint16(b, 0xffff)
	g.e.MoxUpdate()
	for i := 0; i < maxLeaders; i++ {
		if int(s.Leaders[i].Owner) == me {
			b = binary.LittleEndian.AppendUint16(b, uint16(i))
			b = append(b, recBytes(&s.Leaders[i])...)
		}
	}
	b = binary.LittleEndian.AppendUint16(b, 0xffff)
	g.e.MoxUpdate()
	chunks := (len(b) + netChunk - 1) / netChunk
	var cb [2]byte
	binary.LittleEndian.PutUint16(cb[:], uint16(chunks))
	g.moxSendMessage(int32(n.i16(netHostID)), 0xd, cb[:])
	for i, p := 0, 0; p < len(b); i, p = i+1, p+netChunk {
		k := len(b) - p
		g.e.MoxUpdate()
		k = min(k, netChunk)
		if i+1 == chunks {
			n.set8(netState, 0xe)
			n.set8(netResult48, 0xff)
		}
		n.tempString = fmt.Sprintf(cFormat(g.netRString(netSendingS, "sending chunk %d of %d\n")), i+1, chunks)
		for {
			if err := g.chatBoxInputLoop(); err != nil {
				return err
			}
			if !g.netIsSending() {
				break
			}
		}
		g.moxSendMessage(int32(n.i16(netHostID)), 0xe, b[p:p+k])
	}
	return nil
}

// copyCStr is strcpy from src into dst, cut at dst's end.
func copyCStr(dst, src []byte) {
	for i := 0; i < len(dst) && i < len(src); i++ {
		dst[i] = src[i]
		if src[i] == 0 {
			return
		}
	}
}

// decodeGameOrders is Decode_Game_Orders_ (0xf83d8): a client's upload
// over the host's state. Nothing is checked; the stardate is ignored.
func (g *Game) decodeGameOrders(buf []byte, size int) {
	s := &g.state
	// A short or missing upload reads zeros (the original reads its block).
	buf = append(append([]byte(nil), buf[:min(size, len(buf))]...), make([]byte, netOrdersSize)...)
	p := int(buf[4])
	setRec(&s.Players[p], buf[5:5+netPlayerSize])
	off := 5 + netPlayerSize
	g.e.MoxUpdate()
	ns := int(s.NumStars)
	ent := buf[off : off+ns*netStarOrder]
	off += ns * netStarOrder
	bit := byte(1) << uint(p)
	for i := ns - 1; i >= 0; i-- {
		st := recBytes(&s.Stars[i])
		e := ent[i*netStarOrder:]
		bits := binary.LittleEndian.Uint16(e)
		for k, o := range starOrderBits {
			if bits>>uint(k)&1 != 0 {
				st[o] |= bit
			} else {
				st[o] &^= bit
			}
		}
		st[0x42+p] = e[0x13]
		if int(int8(st[0x37])) == p {
			copyCStr(st, e[4:])
		}
		st[0x67+p] = e[0x17]
		setRec(&s.Stars[i], st)
	}
	i16 := func() int16 {
		v := int16(binary.LittleEndian.Uint16(buf[off:]))
		off += 2
		return v
	}
	g.e.MoxUpdate()
	if nc := i16(); s.NumColonies < nc {
		s.NumColonies = nc
	}
	for {
		idx := i16()
		if idx == -1 {
			break
		}
		setRec(&s.Colonies[idx], buf[off:off+0x169])
		off += 0x169
		if pl := &s.Planets[s.Colonies[idx].Planet]; pl.Colony != idx {
			pl.Colony = idx
		}
		g.e.MoxUpdate()
	}
	g.e.MoxUpdate()
	if nsh := i16(); s.NumShips < nsh {
		s.NumShips = nsh
	}
	kill := func(i int) {
		if sh := &s.Ships[i]; int(sh.Owner) == p && sh.Status != 5 {
			sh.Officer = -1
			g.killShip(i)
		}
	}
	k := 0
	for {
		idx := i16()
		if idx == -1 {
			break
		}
		for ; k < int(idx); k++ {
			kill(k)
		}
		setRec(&s.Ships[idx], buf[off:off+0x81])
		off += 0x81
		k = int(idx) + 1
		g.e.MoxUpdate()
	}
	for ; k < int(s.NumShips); k++ {
		kill(k)
	}
	g.e.MoxUpdate()
	l := 0
	for {
		idx := i16()
		if idx == -1 {
			break
		}
		for ; l < int(idx); l++ {
			if int(s.Leaders[l].Owner) == p {
				g.ditchLeader(l)
			}
		}
		setRec(&s.Leaders[idx], buf[off:off+0x3b])
		off += 0x3b
		l = int(idx) + 1
	}
	for ; l < maxLeaders; l++ {
		if int(s.Leaders[l].Owner) == p {
			g.ditchLeader(l)
		}
	}
	g.e.MoxUpdate()
}

// ditchLeader is Ditch_Leader_ (0xf83b7): no owner, nowhere.
func (g *Game) ditchLeader(l int) {
	ld := &g.state.Leaders[l]
	ld.ETA, ld.Status, ld.Location, ld.Owner = 0, -1, -1, -1
}

// moxNetUninit is Mox_Net_Uninit_ (0xfaa94): the network is closed and no
// player has a net id any more.
func (g *Game) moxNetUninit() {
	g.netUninit()
	for k := range g.state.Players {
		g.state.Players[k].NetID = -1
	}
	g.net.set8(netActive, 0)
}

// netQuit reports whether err is the host closing (a scripted run ending),
// which the turn code passes up by panicking.
func netQuit(err error) bool { return errors.Is(err, host.ErrQuit) }
