package game

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/openorion2/openorion2/internal/gfx"
	"github.com/openorion2/openorion2/internal/save"
)

// NETMOX.C's diplomacy: the arbiter of talks with an AI (msgs 5..0xa, the
// msg 0x22 rebroadcast) and the requests between humans (msgs 0x1b..0x1f,
// 0x26, 0x44), ported from the assembly. Spec: docs/net/diplomacy.md.

const (
	netDipReq     = 0x1ab054 // u8 bits: humans asking to talk with us
	netDipFirst   = 0x1ab066 // u8: we open the talk (Diplomacy_Screen_'s talker)
	netDipWait    = 0x1ab067 // u8: the asked human said "wait"
	netDipNew     = 0x1ab069 // u8 bits: requests not yet put to the player
	netDipCrossed = 0x1ab0cc // u8: the talk began from his request (msg 0x1c)
	netNoLongerS  = 157      // RSTRING: the eighth NETMOX string (0x1aaebc)
)

// Net states (_net+0x17b, netState) of a talk.
const (
	netStIdle       = 4
	netStAIAsked    = 0x25 // msg 5 sent
	netStAIQueued   = 0x26 // msg 7: someone else talks with that AI
	netStAICancel   = 0x27 // msg 8 sent
	netStAITalk     = 0x28 // msg 6: go ahead
	netStHumanAsked = 0x29 // msg 0x1b sent
	netStRejected   = 0x2a // msg 0x1d
	netStHumanTalk  = 0x2b // msg 0x44 came: talk
	netStAccepted   = 0x2e // msg 0x1c or crossing requests: msg 0x44 due
)

// netSlot is the address of field off of player k's Do_Net_Combat_Update_
// slot, which the arbiter shares: +0 the state, +0xb the AI asked for.
func netSlot(k int32, off uint32) uint32 { return uint32(int32(netCombat) + 12*k + int32(off)) }

// getNextAITalker is Get_Next_AI_Talker_ (0xfa1a3): -1 while a human talks
// with ai, else a weighted pick among those asking (repulsive 1, normal 2,
// charismatic 3; only the highest weight stays).
func (g *Game) getNextAITalker(ai int32) int32 {
	n, s := &g.net, &g.state
	var w [8]uint8
	busy := false
	var top uint8
	for k := int32(s.NumPlayers) - 1; k >= 0; k-- {
		if int32(n.i8(netSlot(k, 0xb))) != ai {
			continue
		}
		if n.u8(netSlot(k, 0)) == netStAITalk {
			busy = true
			continue
		}
		switch {
		case s.Players[k].Picks[20] != 0:
			w[k] = 3
		case s.Players[k].Picks[19] != 0:
			w[k] = 1
		default:
			w[k] = 2
		}
		if top < w[k] {
			top = w[k]
		}
	}
	if busy {
		return -1
	}
	for k := int(s.NumPlayers) - 1; k >= 0; k-- {
		if top > w[k] {
			w[k] = 0
		}
	}
	return int32(getWeightedChoiceChar(&s.Random, w[:s.NumPlayers]))
}

// netRequestAIDiplomacy is Net_Request_AI_Diplomacy_ (0xfa270): msg 5 {ai}
// to the arbiter.
func (g *Game) netRequestAIDiplomacy(ai int8) {
	n := &g.net
	n.set8(netTalkTo, uint8(ai))
	n.set8(netState, netStAIAsked)
	if h := n.i16(netLastHost); h != -1 {
		g.moxSendMessage(int32(h), 5, []byte{byte(ai)})
	}
}

// netMayBeginAIDiplomacy is Net_May_Begin_AI_Diplomacy_ (0xfa2a9). Outside
// states 0x25, 0x26 and 0x28 it returns al as the caller left it; no
// caller reaches it there (the port says false).
func (g *Game) netMayBeginAIDiplomacy() bool { return g.net.u8(netState) == netStAITalk }

// netEndAIDiplomacy is Net_End_AI_Diplomacy_ (0xfa2c5): msg 0xa to the
// arbiter with both player records and the AI's surrender bytes.
func (g *Game) netEndAIDiplomacy() {
	n, s := &g.net, &g.state
	if g.gameType != 2 && g.gameType != 3 {
		return
	}
	ai := int32(n.i8(netTalkTo))
	buf := make([]byte, net22Size)
	binary.LittleEndian.PutUint32(buf, uint32(ai))
	copy(buf[4:], g.playerRecord(int(ai)))
	ns := int(s.NumStars)
	for i := ns - 1; i >= 0; i-- {
		buf[net22Stars+i] = byte(s.Stars[i].SurrenderTo[ai])
	}
	binary.LittleEndian.PutUint32(buf[net22Stars+72:], uint32(int32(s.PlayerNum)))
	copy(buf[net22Stars+72+4:], g.playerRecord(int(s.PlayerNum)))
	n.set8(netTalkTo, 0xff)
	n.set8(netState, netStIdle)
	if h := n.i16(netLastHost); h != -1 {
		var stale []netSpan
		if ns < 72 { // the frame's old stack past the stars
			stale = []netSpan{{net22Stars + ns, net22Stars + 72}}
		}
		g.moxSendMessageStale(int32(h), 0xa, buf, stale)
	}
}

// netCancelAIDiplomacyRequest is Net_Cancel_AI_Diplomacy_Request_ (0xfa38f):
// msg 8 {-1}; the arbiter answers msg 9.
func (g *Game) netCancelAIDiplomacyRequest() {
	n := &g.net
	if g.gameType != 2 && g.gameType != 3 {
		return
	}
	n.set8(netTalkTo, 0xff)
	n.set8(netState, netStAICancel)
	if h := n.i16(netLastHost); h != -1 {
		g.moxSendMessage(int32(h), 8, []byte{0xff})
	}
}

// netCancelAIDiplomacyConfirmed is Net_Cancel_AI_Diplomacy_Confirmed_ (0xfa3d1).
func (g *Game) netCancelAIDiplomacyConfirmed() bool { return g.net.u8(netState) == netStIdle }

// noAIDiplomacyAllowed is No_AI_Diplomacy_Allowed_ (0xfa3dc): no arbiter yet.
func (g *Game) noAIDiplomacyAllowed() bool {
	if g.net.i16(netLastHost) != -1 {
		return false
	}
	g.net.set8(netState, netStIdle)
	return true
}

// netRequestHumanDiplomacy is Net_Request_Human_Diplomacy_ (0xfa3f6): if p
// asked us already the talk begins (msg 0x1c), else msg 0x1b with our record
// and our surrender bytes, unless p has ended his turn.
func (g *Game) netRequestHumanDiplomacy(p int8) {
	n, s := &g.net, &g.state
	bit := uint8(1) << uint(p&7)
	n.set8(netTalkTo, uint8(p))
	if n.u8(netDipReq)>>uint(p)&1 != 0 {
		n.set8(netState, netStAccepted)
		n.set8(netDipCrossed, 1)
		n.set8(netSend44, uint8(p))
	} else {
		n.set8(netState, netStHumanAsked)
		n.set8(netDipCrossed, 0)
	}
	to := int32(s.Players[p].NetID)
	if n.u8(netDipReq)>>uint(p)&1 != 0 {
		n.set8(netDipReq, n.u8(netDipReq)&^bit)
		n.set8(netDipNew, n.u8(netDipNew)&^bit)
		n.set8(netDipFirst, 0)
		g.moxSendMessage(to, 0x1c, g.playerRecord(int(s.PlayerNum)))
		return
	}
	if n.u8(netReady+uint32(p)) != 0 {
		return
	}
	me := int(s.PlayerNum)
	buf := make([]byte, netPlayerSize+72)
	copy(buf, g.playerRecord(me))
	ns := int(s.NumStars)
	for i := ns - 1; i >= 0; i-- {
		buf[netPlayerSize+i] = byte(s.Stars[i].SurrenderTo[me])
	}
	var stale []netSpan
	if ns < 72 {
		stale = []netSpan{{netPlayerSize + ns, netPlayerSize + 72}}
	}
	g.moxSendMessageStale(to, 0x1b, buf[:0xef1], stale)
}

// netMayBeginHumanDiplomacy is Net_May_Begin_Human_Diplomacy_ (0xfa521):
// true in state 0x2b; false in 4, 0x29, 0x2a, 0x2e, 0x2f.
func (g *Game) netMayBeginHumanDiplomacy() bool {
	switch st := g.net.u8(netState); {
	case st == netStHumanTalk:
		return true
	case st == netStIdle, st == netStHumanAsked, st == netStRejected, st == netStAccepted, st == 0x2f:
		return false
	default:
		// ah as the caller left it; no message leads to these states.
		return false
	}
}

// allowedToCancelHumanDiplomacy is Allowed_To_Cancel_Human_Diplomacy_ (0xfa54b).
func (g *Game) allowedToCancelHumanDiplomacy() bool { return g.net.u8(netState) == netStHumanAsked }

// netCancelHumanDiplomacyRequest is Net_Cancel_Human_Diplomacy_Request_
// (0xfa556): msg 0x1f to the human asked.
func (g *Game) netCancelHumanDiplomacyRequest() {
	n, s := &g.net, &g.state
	if g.gameType != 2 && g.gameType != 3 {
		return
	}
	to := int32(s.Players[n.i8(netTalkTo)].NetID)
	n.set8(netState, netStIdle)
	n.set8(netTalkTo, 0xff)
	if to != -1 {
		g.moxSendMessage(to, 0x1f, nil)
	}
}

// humanHitNextTurn is Human_Hit_Next_Turn_ (0xfa5a3).
func (g *Game) humanHitNextTurn(p int) bool { return g.net.u8(netReady+uint32(p)) != 0 }

// netEndHumanDiplomacy is Net_End_Human_Diplomacy_ (0xfa5aa).
func (g *Game) netEndHumanDiplomacy() {
	g.net.set8(netTalkTo, 0xff)
	g.net.set8(netState, netStIdle)
}

// humanDiplomatSaysWait is Human_Diplomat_Says_Wait_ (0xfa5b9): msg 0x26 came.
func (g *Game) humanDiplomatSaysWait() bool {
	w := g.net.u8(netDipWait) != 0
	g.net.set8(netDipWait, 0)
	return w
}

// netDiplomacyWith is Net_Diplomacy_With_ (0xfa5c6).
func (g *Game) netDiplomacyWith() int { return int(g.net.i8(netTalkTo)) }

// humanRequestingDiplomacy is Human_Requesting_Diplomacy_ (0xfa5ce): one of
// the new requests at random (-1 none), taken off the new ones.
func (g *Game) humanRequestingDiplomacy() int {
	n := &g.net
	pick := int32(-1)
	if n.u8(netDipNew) == 0 {
		return -1
	}
	seen := uint32(0)
	for k := int32(g.state.NumPlayers) - 1; k >= 0; k-- {
		if n.u8(netDipNew)>>uint(k)&1 == 0 {
			continue
		}
		seen++
		if g.state.Random.Random(seen) == 1 {
			pick = k
		}
	}
	n.set8(netDipNew, n.u8(netDipNew)&^(uint8(1)<<uint(pick&0x1f)))
	return int(pick)
}

// acceptDiplomacyRequest is Accept_Diplomacy_Request_ (0xfa623): msg 0x1c
// with our record, then wait for his msg 0x44 and talk.
func (g *Game) acceptDiplomacyRequest(p int) error {
	n, s := &g.net, &g.state
	if n.u8(netDipReq)>>uint(p)&1 == 0 { // he withdrew it
		return g.messageBox(g.netRString(netNoLongerS, "That player no longer wishes to conduct diplomacy."))
	}
	bit := uint8(1) << uint(p)
	n.set8(netState, netStAccepted)
	n.set8(netDipCrossed, 1)
	n.set8(netTalkTo, uint8(p))
	n.set8(netSend44, uint8(p))
	n.set8(netDipReq, n.u8(netDipReq)&^bit)
	n.set8(netDipNew, n.u8(netDipNew)&^bit)
	n.set8(netDipFirst, 0)
	g.moxSendMessage(int32(s.Players[p].NetID), 0x1c, g.playerRecord(int(s.PlayerNum)))
	for st := n.u8(netState); st == netStHumanAsked || st == netStAccepted; st = n.u8(netState) {
		if err := g.netWait(); err != nil {
			return err
		}
	}
	// The flag tested is set in state 0x2b and cleared in 4, 0x2a, 0x2f; in
	// other states it is the frame's old byte (no message leads there).
	if g.net.u8(netState) == netStHumanTalk {
		if err := g.diplomacyScreen(p); err != nil {
			return err
		}
		n.set8(netTalkTo, 0xff)
		n.set8(netState, netStIdle)
		return nil
	}
	if err := g.messageBox(g.netRString(netNoLongerS, "That player no longer wishes to conduct diplomacy.")); err != nil {
		return err
	}
	g.netCancelHumanDiplomacyRequest()
	return nil
}

// refuseDiplomacyRequest is Refuse_Diplomacy_Request_ (0xfa728): we ignore
// p from now on and answer msg 0x1d.
func (g *Game) refuseDiplomacyRequest(p int) {
	n, s := &g.net, &g.state
	bit := uint8(1) << uint(p)
	s.Players[s.PlayerNum].Ignoring |= bit
	n.set8(netDipReq, n.u8(netDipReq)&^bit)
	n.set8(netDipNew, n.u8(netDipNew)&^bit)
	g.moxSendMessage(int32(s.Players[p].NetID), 0x1d, nil)
}

// tellDiplomacyRequesterToWait is Tell_Diplomacy_Requester_To_Wait_ (0xfa774): msg 0x26.
func (g *Game) tellDiplomacyRequesterToWait(p int) {
	g.moxSendMessage(int32(g.state.Players[p].NetID), 0x26, nil)
}

// humansRequestingDiplomacy is Humans_Requesting_Diplomacy_ (0xfa795).
func (g *Game) humansRequestingDiplomacy() uint8 { return g.net.u8(netDipReq) }

// netHumanRejectedDiplomacy is Net_Human_Rejected_Diplomacy_ (0xfa79b).
func (g *Game) netHumanRejectedDiplomacy() bool {
	if g.net.u8(netState) != netStRejected {
		return false
	}
	g.net.set8(netState, netStIdle)
	return true
}

// netDiploCase runs Main_Receive_Message_'s diplomacy cases; false for the
// other addresses.
func (g *Game) netDiploCase(c uint32, player int32, from uint16, buf []byte) bool {
	n, s := &g.net, &g.state
	bit := func(k int32) uint8 { return uint8(1) << uint(k&0x1f) }
	switch c {
	case 0xf5d3d, 0xf5d9a, 0xf5dbc, 0xf5dd5, 0xf5e42, 0xf5e55, 0xf6119, 0xf620c, 0xf62a3,
		0xf62af, 0xf63a2, 0xf6465, 0xf6765, 0xf6776:
	default:
		return false
	}
	if player < 0 && c != 0xf63a2 {
		return true // port-only: no sender outside the game in the port
	}
	switch c {
	case 0xf5d3d: // msg 5 {ai}: on the arbiter, a human asks to talk with ai
		ai := int8(buf[0])
		n.set8(netSlot(player, 0xb), uint8(ai))
		n.set8(netSlot(player, 0), netStAIQueued)
		if g.getNextAITalker(int32(ai)) == -1 {
			n.set8(netSend7, n.u8(netSend7)|bit(player))
			break
		}
		// The pick is only tested for -1: the asker himself goes ahead.
		n.set8(netSend6, n.u8(netSend6)|bit(player))
		n.set8(netSlot(player, 0xb), uint8(ai))
		n.set8(netSlot(player, 0), netStAITalk)
	case 0xf5d9a: // msg 6: go ahead
		if st := n.u8(netState); st == netStAIAsked || st == netStAIQueued {
			n.set8(netState, netStAITalk)
		}
	case 0xf5dbc: // msg 7: wait in the queue
		if n.u8(netState) == netStAIAsked {
			n.set8(netState, netStAIQueued)
		}
	case 0xf5dd5: // msg 8: a request withdrawn, on the arbiter
		if n.u8(netSlot(player, 0)) != netStAIQueued {
			n.set8(netSend6, n.u8(netSend6)&^bit(player))
			if k := g.getNextAITalker(int32(n.i8(netSlot(player, 0xb)))); k != -1 {
				n.set8(netSend6, n.u8(netSend6)|bit(k))
				n.set8(netSlot(k, 0), netStAITalk)
			}
		}
		n.set8(netSlot(player, 0xb), 0xff)
		n.set8(netSlot(player, 0), netStIdle)
		n.set8(netSend9, n.u8(netSend9)|bit(player))
	case 0xf5e42: // msg 9: the withdrawal is through
		n.set8(netTalkTo, 0xff)
		n.set8(netState, netStIdle)
	case 0xf5e55: // msg 0xa: a talk with the AI is over, on the arbiter
		ai := int32(n.i8(netSlot(player, 0xb)))
		g.setPlayerRecord(int(ai), buf[4:])
		g.setPlayerRecord(int(player), buf[net22Stars+72+4:])
		for i := int(s.NumStars) - 1; i >= 0; i-- {
			s.Stars[i].SurrenderTo[ai] = int8(buf[net22Stars+i])
		}
		n.set8(netSlot(player, 0xb), 0xff)
		n.set8(netSlot(player, 0), netStIdle)
		n.set8(uint32(int32(netSend22)+ai), 0xff)
		n.set8(uint32(int32(netPartner)+ai), uint8(player))
		if k := g.getNextAITalker(ai); k != -1 {
			n.set8(netSend6, n.u8(netSend6)|bit(k))
			n.set8(netSlot(k, 0), netStAITalk)
		}
	case 0xf63a2: // msg 0x22: an AI's record after another human's talk
		ai := int32(binary.LittleEndian.Uint32(buf))
		partner := int32(binary.LittleEndian.Uint32(buf[net22Stars+72:]))
		g.setPlayerRecord(int(ai), buf[4:])
		g.setPlayerRecord(int(partner), buf[net22Stars+72+4:])
		// The star loop copies the stars into the receive buffer, not
		// the other way: the surrender bytes are not applied.
	case 0xf6119: // msg 0x1b: a human asks to talk with us
		if n.u8(netReady+uint32(s.PlayerNum)) != 0 {
			break
		}
		g.setPlayerRecord(int(player), buf)
		for i := int(s.NumStars) - 1; i >= 0; i-- {
			s.Stars[i].SurrenderTo[player] = int8(buf[netPlayerSize+i])
		}
		if n.u8(netState) == netStHumanAsked && int32(n.i8(netTalkTo)) == player {
			// Crossing requests.
			n.set8(netSend44, uint8(player))
			n.set8(netDipReq, n.u8(netDipReq)&^bit(player))
			n.set8(netDipNew, n.u8(netDipNew)&^bit(player))
			n.set8(netState, netStAccepted)
			first := uint8(0)
			if int16(n.i8(netTalkTo)) > s.PlayerNum {
				first = 1
			}
			n.set8(netDipFirst, first)
			break
		}
		g.netNoteDiplomacyRequest(player)
	case 0xf620c: // msg 0x1c: our request is accepted
		g.setPlayerRecord(int(player), buf)
		if n.u8(netState) == netStHumanAsked && int32(n.i8(netTalkTo)) == player {
			n.set8(netSend44, uint8(player))
			n.set8(netDipReq, n.u8(netDipReq)&^bit(player))
			n.set8(netState, netStAccepted)
			n.set8(netDipFirst, 1)
			n.set8(netDipNew, n.u8(netDipNew)&^bit(player))
			break
		}
		g.netNoteDiplomacyRequest(player)
	case 0xf62a3: // msg 0x1d: refused
		n.set8(netState, netStRejected)
	case 0xf62af: // msg 0x1f: a request withdrawn
		n.set8(netDipReq, n.u8(netDipReq)&^bit(player))
		n.set8(netDipNew, n.u8(netDipNew)&^bit(player))
		if n.u8(netState) == netStHumanTalk && int32(n.i8(netTalkTo)) == player {
			n.set8(netState, netStIdle)
			n.set8(netTalkTo, 0xff)
			n.set8(netDipCrossed, 0)
		}
		if n.u8(netState) == netStAccepted && int32(n.i8(netTalkTo)) == player {
			if n.u8(netDipCrossed) != 0 {
				n.set8(netState, netStIdle)
			} else {
				n.set8(netState, netStHumanAsked)
			}
			n.set8(netDipCrossed, 0)
			n.set8(netSend44, 0xff)
		}
	case 0xf6465: // msg 0x26: the human asked says wait
		if n.u8(netState) == netStHumanAsked && int32(n.i8(netTalkTo)) == player {
			n.set8(netDipWait, 1)
		}
	case 0xf6765: // msg 0x64 {i16}: declare war (no sender found)
		g.declareWar(int16(player), int16(binary.LittleEndian.Uint16(buf)))
	case 0xf6776: // msg 0x44: his record; the talk begins
		g.setPlayerRecord(int(player), buf)
		if n.u8(netState) == netStAccepted && int32(n.i8(netTalkTo)) == player {
			n.set8(netState, netStHumanTalk)
			n.set8(netDipReq, n.u8(netDipReq)&^bit(player))
			n.set8(netDipNew, n.u8(netDipNew)&^bit(player))
		}
	}
	return true
}

// netNoteDiplomacyRequest is the tail of msgs 0x1b and 0x1c (0xf61c6): a
// human we ignore gets msg 0x1d, the others light up.
func (g *Game) netNoteDiplomacyRequest(player int32) {
	n, s := &g.net, &g.state
	b := uint8(1) << uint(player&0x1f)
	if s.Players[s.PlayerNum].Ignoring>>uint(player)&1 != 0 {
		n.set8(netSend1d, n.u8(netSend1d)|b)
		return
	}
	n.set8(netDipReq, n.u8(netDipReq)|b)
	n.set8(netDipNew, n.u8(netDipNew)|b)
}

// setPlayerRecord overwrites player k's 0xea9 bytes with rec.
func (g *Game) setPlayerRecord(k int, rec []byte) {
	full := g.playerRecord(k)
	copy(full, rec)
	binary.Read(bytes.NewReader(full), binary.LittleEndian, &g.state.Players[k])
}

// diplomacyMessageIsInteractive is Diplomacy_Message_Is_Interactive_
// (0x20162): the AI messages the human answers.
func diplomacyMessageIsInteractive(m int16) bool {
	return m >= 0x53 && m <= 0x55 || m >= 0x61 && m <= 0x68 || m == 0x7d || m == 0x56 ||
		m == 0x69 || m == 0x6a || m >= 0x1c && m <= 0x24
}

// replyPopup is MAINSCRN's reply box to a request (_diplomacy_request_popup_seg
// and its three buttons).
type replyPopup struct {
	bg, accept, reject, wait *gfx.Picture
}

// loadReplyToDiplomacyRequestPopup is Load_Reply_To_Diplomacy_Request_Popup_ (0x86111).
func (g *Game) loadReplyToDiplomacyRequestPopup() (replyPopup, error) {
	var r replyPopup
	var err error
	for i, p := range []**gfx.Picture{&r.bg, &r.accept, &r.reject, &r.wait} {
		if *p, err = g.picture(buffer0LBX, 0xf+i); err != nil {
			return r, err
		}
	}
	return r, nil
}

// replyXY is the reply box's corner (the 0x1fa by 0x190 map, centred).
func replyXY(bg *gfx.Picture) (int, int) {
	return (0x1fa-bg.Width)/2 + 0x15, (0x190-bg.Height)/2 + 0x15
}

// doReplyToDiplomacyRequest is Do_Reply_To_Diplomacy_Request_ (0x8354e): 1
// accept, 2 refuse, 3 wait.
func (g *Game) doReplyToDiplomacyRequest(p int) (int, error) {
	e, m := g.e, &g.ms
	e.ClearFields()
	g.setMainScreenFieldsToNull()
	e.V.SetPageOff()
	if err := g.drawMiniMainScreen(); err != nil {
		return 0, err
	}
	e.V.CopyOffToBack()
	e.V.SetRefreshStencil()
	if err := e.TogglePages(); err != nil {
		return 0, err
	}
	g.screenData[0] = int16(p)
	e.ClearFields()
	g.setMainScreenFieldsToNull()
	pop, err := g.loadReplyToDiplomacyRequestPopup()
	if err != nil {
		return 0, err
	}
	// Add_Reply_To_Diplomacy_Request_Fields_ (0x821a6).
	x, y := replyXY(pop.bg)
	accept, err := e.AddButtonField(x+0x40, y+0x5a, "", pop.accept, "A", 0x28)
	if err != nil {
		return 0, err
	}
	reject, err := e.AddButtonField(x+0xbc, y+0x5a, "", pop.reject, "I", 0x28)
	if err != nil {
		return 0, err
	}
	wait, err := e.AddButtonField(x+0x13b, y+0x5a, "", pop.wait, "W", 0x28)
	if err != nil {
		return 0, err
	}
	if _, err := e.AddHiddenField(x, y, x+pop.bg.Width, y+pop.bg.Height, "", 0); err != nil {
		return 0, err
	}
	if _, err := e.AddHiddenField(0, 0, 0x27f, 0x1df, "\x1b", 0x29); err != nil {
		return 0, err
	}
	draw := func() error { return g.drawReplyToDiplomacyRequestPopup(pop) }
	e.AssignAutoFunction(draw, 1)
	g.setDiplomacyRequestHelpList(x, y)
	r := 0
	m.replyShowing = true
	defer func() { m.replyShowing = false }()
	for r == 0 {
		in, err := e.GetInput()
		if err != nil {
			return 0, err
		}
		switch in {
		case accept:
			r = 1
		case reject:
			r = 2
		case wait:
			r = 3
		}
		if r == 0 {
			e.V.SetPageOff()
			if err := draw(); err != nil {
				return 0, err
			}
			if err := e.TogglePages(); err != nil {
				return 0, err
			}
		}
	}
	g.setMainScreenHelpList()
	g.fullDrawScreen = true
	m.alreadyFaded = false
	return r, nil
}

// drawReplyToDiplomacyRequestPopup is Draw_Reply_To_Diplomacy_Request_Popup_
// (0x853d3): "<name> of the <race> requests diplomacy".
func (g *Game) drawReplyToDiplomacyRequestPopup(pop replyPopup) error {
	e, s := g.e, &g.state
	x, y := replyXY(pop.bg)
	p := &s.Players[g.screenData[0]]
	c := mainPalettePlayerColors[p.Color]
	if err := g.drawMiniMainScreen(); err != nil {
		return err
	}
	if err := g.drawMainMainScreen(); err != nil {
		return err
	}
	if err := e.Draw(x, y, pop.bg); err != nil {
		return err
	}
	t := e.Text()
	t.SetStyle(4, g.moxFontColors(4, c, c+3))
	msg := fmt.Sprintf(cFormat(g.hMessage(0xdb)), save.CString(p.EmperorName[:]), save.CString(p.RaceName[:]))
	return t.Print(x+0x27, y+0x3c, msg)
}

// setDiplomacyRequestHelpList is Set_Diplomacy_Request_Help_List_ (0x6fb73):
// the 0x28-byte list at 0x181f92 placed at the box.
func (g *Game) setDiplomacyRequestHelpList(x, y int) {
	l := g.helpListAt(0x181f92, 4)
	for i := range l { // Offset_Help_Field_XYs_ (0x6fc7e): i16 sums
		h := &l[i]
		h.X1, h.Y1 = int(int16(h.X1+x)), int(int16(h.Y1+y))
		h.X2, h.Y2 = int(int16(h.X2+x)), int(int16(h.Y2+y))
	}
	g.e.DeactivateHelpList()
	g.e.SetHelpList(l)
}

// checkForDiplomacyRequest is Check_For_Diplomacy_Request_ (0x8250c): each
// new request is put to the player; 1 if any was.
func (g *Game) checkForDiplomacyRequest() (bool, error) {
	any := false
	for k, p := 0, g.humanRequestingDiplomacy(); p != -1; k, p = k+1, g.humanRequestingDiplomacy() {
		if k > 0 {
			if err := g.fadeIntoMainScreen(); err != nil {
				return false, err
			}
		}
		any = true
		r, err := g.doReplyToDiplomacyRequest(p)
		if err != nil {
			return false, err
		}
		switch r {
		case 1:
			g.ms.forceFade = true
			if err := g.acceptDiplomacyRequest(p); err != nil {
				return false, err
			}
		case 2:
			g.refuseDiplomacyRequest(p)
		case 3:
			g.tellDiplomacyRequesterToWait(p)
		}
	}
	return any, nil
}

// drawDiplomacyRequestLights is Draw_Diplomacy_Request_Lights_ (0x83d06): a
// blinking light in the banner colour of each human asking to talk, from
// the right edge of the map leftwards.
func (g *Game) drawDiplomacyRequestLights() error {
	s, m := &g.state, &g.ms
	req := g.humansRequestingDiplomacy()
	n := 0
	for p := 0; p < int(s.NumPlayers); p++ {
		if req>>uint(p)&1 == 0 {
			continue
		}
		c := s.Players[p].Color
		light := m.playerLights[c]
		x := 0x1fa - n*m.playerLights[0].Width
		saved := light.CurrentFrame
		g.setAnimationFrame(light, int(m.requestFrame[c]))
		if err := g.e.Draw(x, 5, light); err != nil {
			return err
		}
		m.requestFrame[c]++
		if int(m.requestFrame[c]) >= light.Frames {
			m.requestFrame[c] = 0
		}
		n++
		g.setAnimationFrame(light, saved)
	}
	return nil
}

// raceScreenNetAudience is Race_Screen_'s AUDIENCE in a network game
// (0x10b603..0x10b958): an AI is asked through the arbiter, a human by msg
// 0x1b; any input while waiting withdraws the request.
func (g *Game) raceScreenNetAudience(them int) error {
	e, rs, s := g.e, &g.race, &g.state
	wait := func() (bool, error) {
		e.MarkTime()
		e.ClearFields()
		if _, err := e.AddHiddenField(0, 0, 0x27f, 0x1df, cStringAt(g.exe, raceSpace), 0); err != nil {
			return false, err
		}
		in, err := e.GetInput()
		if err != nil {
			return false, err
		}
		if err := g.drawRaceScreen(); err != nil {
			return false, err
		}
		if err := e.TogglePages(); err != nil {
			return false, err
		}
		e.MoxUpdate()
		return in != 0, e.ReleaseTime(2)
	}
	if s.Players[them].Objective != save.ObjectiveHuman {
		cancel := false
		rs.netState = 1
		g.netRequestAIDiplomacy(int8(them))
		for !g.netMayBeginAIDiplomacy() && !cancel && !g.noAIDiplomacyAllowed() {
			var err error
			if cancel, err = wait(); err != nil {
				return err
			}
		}
		rs.netState = 0
		switch {
		case cancel:
			g.netCancelAIDiplomacyRequest()
			for !g.netCancelAIDiplomacyConfirmed() {
				if err := g.netWait(); err != nil {
					return err
				}
			}
		case g.noAIDiplomacyAllowed():
			msg, err := g.textMessage("billtex2.lbx", 0x1b, 0x50)
			if err != nil {
				return err
			}
			return g.messageBox(msg)
		default:
			if err := g.diplomacyScreen(them); err != nil {
				return err
			}
			g.netEndAIDiplomacy()
		}
		return nil
	}
	rejected, cancel := false, false
	rs.netState = 1
	g.netRequestHumanDiplomacy(int8(them))
	for !rejected && !g.netMayBeginHumanDiplomacy() && !cancel && !g.humanHitNextTurn(them) {
		in, err := func() (bool, error) {
			e.MarkTime()
			e.ClearFields()
			if _, err := e.AddHiddenField(0, 0, 0x27f, 0x1df, cStringAt(g.exe, raceSpace), 0); err != nil {
				return false, err
			}
			in, err := e.GetInput()
			return in != 0, err
		}()
		if err != nil {
			return err
		}
		if in && g.allowedToCancelHumanDiplomacy() {
			cancel = true
		}
		if g.humanDiplomatSaysWait() {
			rs.netState = 2
		}
		if g.netHumanRejectedDiplomacy() {
			rejected = true
		}
		if err := g.drawRaceScreen(); err != nil {
			return err
		}
		if err := e.TogglePages(); err != nil {
			return err
		}
		e.MoxUpdate()
		if err := e.ReleaseTime(2); err != nil {
			return err
		}
	}
	rs.netState = 0
	n := 0
	switch {
	case g.humanHitNextTurn(them):
		n = 0x37
	case rejected:
		n = 0x38
	case g.netMayBeginHumanDiplomacy():
		if err := g.diplomacyScreen(them); err != nil {
			return err
		}
		g.netEndHumanDiplomacy()
		return nil
	default:
		g.netCancelHumanDiplomacyRequest()
		return nil
	}
	msg, err := g.textMessage(billTextLBX, n, 0x50)
	if err != nil {
		return err
	}
	if err := g.messageBox(msg); err != nil {
		return err
	}
	g.netEndHumanDiplomacy()
	return nil
}

// mainScreenDiplomacyRequests is Main_Screen_'s 0x86490..0x86505: after a
// request was answered the screen is set up again and faded in.
func (g *Game) mainScreenDiplomacyRequests() error {
	e, m := g.e, &g.ms
	any, err := g.checkForDiplomacyRequest()
	if err != nil || !any {
		return err
	}
	e.AssignAutoFunction(g.drawMainScreen, 1)
	e.ClearFields()
	g.setMainScreenFieldsToNull()
	if err := g.addCheats(); err != nil {
		return err
	}
	if err := e.FastFadeOut(); err != nil {
		return err
	}
	if err := g.addMapFields(); err != nil {
		return err
	}
	if g.moveableBoxSelected(2) {
		m.alreadyFaded = false
		m.reloadedShipPalette = false
	} else if g.moveableBoxSelected(0) {
		g.clearSystemDisplayLoadedFlag()
	}
	return g.fadeIntoMainScreen()
}
