package game

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/openorion2/openorion2/internal/lobby"
	"github.com/openorion2/openorion2/internal/netplay"
	"github.com/openorion2/openorion2/internal/save"
)

// port-only: a lost player becomes an AI everywhere through the original's
// drop-out (msg 0x37), or the game ends naming him. Spec: docs/net/hardening.md.

// Port-only frames, sent outside Mox_Send_Message_ (never in the game's
// message stream); ids between the game's and netplay's control frames.
const (
	netCtlDropOut = 0xfe00 // {u16 lost net id, u16 msg id, payload}: his drop-out
	netCtlAbort   = 0xfe01 // the reason as text: the game cannot go on
	netCtlCheck   = 0xfe02 // {u32 stardate, 32-byte hash}: the desync check
	netCtlRestart = 0xfe03 // {u32 stardate}: the sender starts the turn over
)

// netRestartTurn is the panic value that unwinds a client's turn to the
// election when the turn's host was lost (netNextTurn recovers it).
type netRestartTurn struct{}

// netWho names player p as the network screens do: "emperor (race)".
func (g *Game) netWho(p int) string {
	pl := &g.state.Players[p]
	return fmt.Sprintf("%s (%s)", save.CString(pl.EmperorName[:]), save.CString(pl.RaceName[:]))
}

// netCheckLost runs after every Net_Update_: a lost relay ends the game; a
// lost player becomes an AI (or, with -netdrop end, ends it).
func (g *Game) netCheckLost() {
	n, s := &g.net, &g.state
	if g.gameType != 2 || n.u8(netActive) == 0 || n.ep == nil {
		return
	}
	if p := s.PlayerNum; p >= 0 && int(p) < len(s.Players) && s.Players[p].Eliminated != 0 {
		// An eliminated player is on his way out (msg 0x3b): the creator,
		// left alone, may already have closed the relay.
		return
	}
	if k := g.netIDToPlayerID(1); k >= 0 && k != int32(s.PlayerNum) {
		n.creator = g.netWho(int(k)) // kept: the record may change before the end
	}
	if errors.Is(n.ep.Err(), netplay.ErrLost) {
		who := n.creator
		if who == "" {
			who = "the game's creator"
		}
		if _, ok := g.net.network.(*lobby.Network); ok && g.netCanResume() {
			// port-only: the lobby relays; the others wait for him (netresume.go).
			g.netEnd("The connection to the lobby is lost. The others go back to the start of the turn and wait for you: join the game again from the lobby to go on.")
		}
		msg := fmt.Sprintf("The connection to %s, who relays the game, is lost. The game ends.", who)
		if g.netCanResume() {
			// port-only: only this machine's link may be gone (netresume.go).
			msg = fmt.Sprintf("The connection to %s, who relays the game, is lost. If only your own link went, the others wait for you at the start of the turn: join the game again to go on.", who)
		}
		g.netEnd(msg)
	}
	for _, id := range n.ep.Dropped() {
		k := int(g.netIDToPlayerID(int32(id)))
		if k < 0 || k == int(s.PlayerNum) {
			continue
		}
		who := g.netWho(k)
		switch {
		case n.dropEnd:
			g.netEnd(fmt.Sprintf("%s has lost the connection. The game ends.", who))
		case !n.inGame:
			g.netAbort(fmt.Sprintf("%s left before the game started. The game ends.", who))
		case g.netCanResume():
			g.netLose(k) // port-only: he may come back (netresume.go)
		case g.netInjector():
			g.netSendDropOut(k, int16(id))
		}
	}
}

// netInjector: this machine sends the drop-outs of lost players, being the
// lowest net id among the stations that are still human players.
func (g *Game) netInjector() bool {
	for _, id := range g.net.ep.Users() {
		if g.netIDToPlayerID(int32(id)) >= 0 {
			return id == g.net.ep.ID()
		}
	}
	return false
}

// netSendDropOut is Send_Drop_Out_Info_ done for lost player p (net id id):
// he becomes an AI here and his msg 0x37 goes to every other station.
func (g *Game) netSendDropOut(p int, id int16) {
	s, n := &g.state, &g.net
	if n.i16(netLastHost) == id {
		n.set16(netLastHost, -1)
	}
	s.Players[p].NetID = -1
	g.makePlayerIntoAI(p)
	buf := make([]byte, 4+netDropOutSize)
	binary.LittleEndian.PutUint16(buf, uint16(id))
	binary.LittleEndian.PutUint16(buf[2:], 0x37)
	copy(buf[4:], g.playerRecord(p))
	for i := int(s.NumStars) - 1; i >= 0; i-- {
		buf[4+netPlayerSize+i] = byte(s.Stars[i].SurrenderTo[p])
	}
	n.ep.Send(netplay.Broadcast, netCtlDropOut, buf) // a failure shows as our own lost link
	n.dropping = id
	g.decodeDropOutTail(p)
	n.dropping = 0
	if n.dropHook != nil {
		n.dropHook(p)
	}
}

// netPortFrame takes one of the port-only frames.
func (g *Game) netPortFrame(f netplay.Frame) {
	n, s := &g.net, &g.state
	switch f.ID {
	case netCtlDropOut:
		if len(f.Payload) < 4 {
			return
		}
		id := binary.LittleEndian.Uint16(f.Payload)
		p := g.netIDToPlayerID(int32(int16(id)))
		if p < 0 || n.receive == nil {
			return // already taken (his own quit, or a second sender)
		}
		n.dropping = int16(id)
		n.receive(id, binary.LittleEndian.Uint16(f.Payload[2:]), f.Payload[4:])
		n.dropping = 0
		if n.dropHook != nil {
			n.dropHook(int(p))
		}
	case netCtlAbort:
		g.netEnd(string(f.Payload))
	case netCtlCheck:
		g.netCheckRecv(f.From, f.Payload)
	case netCtlRestart:
		if len(f.Payload) >= 4 && binary.LittleEndian.Uint32(f.Payload) != s.Stardate {
			g.netAbort("The host of the turn left after some machines had his turn and others not. The game ends.")
		}
	}
}

// netLostFixups is what a drop-out sent for a lost player changes beyond
// the original's decode: whatever this machine waited for from him.
func (g *Game) netLostFixups(p int) {
	s, n := &g.state, &g.net
	d := n.dropping
	n.dropped[p] = true
	g.netNotice(fmt.Sprintf("%s has lost the connection. The computer plays the empire from now on.", g.netWho(p)))
	// Votes for him: the lowest vote among the others wins again.
	me := int(s.PlayerNum)
	changed := false
	for k := uint32(0); k < 8; k++ {
		if int(n.i8(netVote+k)) == p {
			n.set8(netVote+k, 0xff)
			changed = true
		}
	}
	if changed {
		if n.u8(netReady+uint32(me)) != 0 {
			v := int8(me)
			for k := 0; k < int(s.NumPlayers); k++ {
				if w := n.i8(netVote + uint32(k)); k != p && w != -1 && w < v {
					v = w
				}
			}
			n.set8(netVote+uint32(me), uint8(v))
		}
		for k := uint32(0); k < 8; k++ {
			n.set8(netSentVote+k, 0xff)
		}
	}
	// The turn's host: the turn starts over, unless his broadcast is all in.
	host := n.i16(netHostID)
	if n.chat != nil && host == d && host != n.i16(netOwnID) && n.u8(netState) != 4 {
		complete := n.u8(netState) == 7 && n.i16(netChunks) > 0 && n.i16(netRecvChunks) >= n.i16(netChunks)
		if !complete {
			n.restart = true
		}
	}
	// On the turn's host, in the combat phase: his battles are off; a star
	// left to fight is picked again with him as an AI.
	if host == n.i16(netOwnID) {
		if st := n.u8(netState); st >= 0xe && st < 0x23 {
			for k := uint32(0); k < 15; k++ {
				if int(k) == p || int(n.i8(netCombat+12*k+9)) == p {
					if st := n.u8(netCombat + 12*k); st != 0xe && st != 4 {
						n.set8(netCombat+12*k, 0xe)
					}
					n.set8(netCombat+12*k+9, 0xff)
				}
			}
			n.set8(netPickDone, 0)
		}
	}
	// On the diplomacy arbiter: his place in an AI's queue is withdrawn (as
	// msg 8 does, without the answer to him).
	if n.i16(netLastHost) == n.i16(netOwnID) {
		if st := n.u8(netSlot(int32(p), 0)); st == netStAIQueued || st == netStAITalk {
			bit := func(k int32) uint8 { return uint8(1) << uint(k&0x1f) }
			if st != netStAIQueued {
				n.set8(netSend6, n.u8(netSend6)&^bit(int32(p)))
				if k := g.getNextAITalker(int32(n.i8(netSlot(int32(p), 0xb)))); k != -1 {
					n.set8(netSend6, n.u8(netSend6)|bit(k))
					n.set8(netSlot(k, 0), netStAITalk)
				}
			}
			n.set8(netSlot(int32(p), 0xb), 0xff)
			n.set8(netSlot(int32(p), 0), netStIdle)
		}
	}
	// A talk with him ends as if he had said good bye.
	if dn := &g.dip.net; dn.inTalk && (int(dn.talker) == p || int(dn.listener) == p) {
		dn.escPressed, dn.synchUp = 1, 1
	}
}

// netLostOpponent ends the game when a battle here waits for lost player p
// (the lockstep, a defender's result): it cannot be finished without him.
func (g *Game) netLostOpponent(p int) {
	if p >= 0 && p < 8 && g.net.dropped[p] {
		g.netAbort(fmt.Sprintf("%s has lost the connection in a battle with you. The game ends.", g.netWho(p)))
	}
}

// netRestartPoint unwinds a client's turn once its host was lost.
func (g *Game) netRestartPoint() {
	if g.net.restart {
		panic(netRestartTurn{})
	}
}

// netRestartTurnState sets the election up again (the turn's host was lost);
// the others get the stardate, which must be theirs too.
func (g *Game) netRestartTurnState() {
	s, n := &g.state, &g.net
	me := uint32(s.PlayerNum)
	n.restart = false
	n.restartAt = s.Stardate
	n.recv, n.resBuf, g.asked = nil, nil, nil
	n.set16(netChunks, 0)
	n.set16(netRecvChunks, 0)
	n.set16(netHostID, -1)
	n.set8(netState, 8)
	n.set8(netReady+me, 1)
	if n.i8(netVote+me) == -1 {
		n.set8(netVote+me, uint8(me))
	}
	for k := uint32(0); k < 8; k++ {
		n.set8(netSentVote+k, 0xff)
	}
	if n.ep != nil {
		n.ep.Send(netplay.Broadcast, netCtlRestart, binary.LittleEndian.AppendUint32(nil, s.Stardate))
	}
	g.netNotice("The host of the turn has left. The turn starts over.")
}

// netNotice tells the player what happened: a GNN line on the waiting
// screen at once, and a message box on the main screen.
func (g *Game) netNotice(msg string) {
	n := &g.net
	if n.chat != nil {
		line := []byte(msg)
		g.receiveChatMsg(8, append(line[:min(len(line), 0x50)], 0))
	}
	n.notices = append(n.notices, msg)
}

// netShowNotices shows the queued notices, one warning box each (the main
// screen's loop).
func (g *Game) netShowNotices() error {
	for len(g.net.notices) > 0 {
		msg := g.net.notices[0]
		g.net.notices = g.net.notices[1:]
		if err := g.warningBox(msg); err != nil {
			return err
		}
	}
	return nil
}

// netEnd ends the game with msg (Run shows it in a warning box, then
// returns it as an *ExitError).
func (g *Game) netEnd(msg string) {
	g.Log("network game ends at stardate %d: %s", g.state.Stardate, msg) // port-only
	if g.net.abortHook != nil {
		g.net.abortHook(msg)
	}
	g.resetSystem()
	panic(&ExitError{Msg: msg, Notice: true})
}

// netAbort ends the game for every machine: the others get msg first.
func (g *Game) netAbort(msg string) {
	if ep := g.net.ep; ep != nil {
		ep.Send(netplay.Broadcast, netCtlAbort, []byte(msg))
	}
	g.netEnd(msg)
}
