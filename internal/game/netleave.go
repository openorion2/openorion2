package game

import "github.com/openorion2/openorion2/internal/host"

// Leaving a network game (NETMOX.C): a human who quits or surrenders
// becomes an AI on every machine (msgs 0x37, 0x38). Ported from the
// assembly. Spec: docs/net/leaving.md.

const (
	netDropOutSize = netPlayerSize + 72 // the player record and its 72 star bytes
)

// humanAbdicates is Human_Abdicates_ (0xfa97c): Quit in the game menu.
func (g *Game) humanAbdicates(p int) error { return g.sendDropOutInfo(p, 0x37) }

// humanSurrenders is Human_Surrenders_ (0xfa989): Turn after surrendering.
func (g *Game) humanSurrenders(p int) error { return g.sendDropOutInfo(p, 0x38) }

// sendDropOutInfo is Send_Drop_Out_Info_ (0xfa7b0): the player leaves the
// network and becomes an AI; msg id with his record and his star bytes
// (star+0x67+p) goes to each other human once that one takes msg 0x22.
func (g *Game) sendDropOutInfo(p int, id uint16) error {
	s, n := &g.state, &g.net
	s.Players[p].NetID = -1
	g.makePlayerIntoAI(p)
	buf := make([]byte, netDropOutSize)
	copy(buf, g.playerRecord(p))
	ns := int(s.NumStars)
	for i := ns - 1; i >= 0; i-- {
		buf[netPlayerSize+i] = byte(s.Stars[i].SurrenderTo[p])
	}
	var stale []netSpan
	if ns < 72 { // the rest of the frame's 72 bytes is old stack
		stale = []netSpan{{netPlayerSize + ns, netDropOutSize}}
	}
	np := int(s.NumPlayers)
	left := make([]bool, np)
	for k := range left {
		left[k] = true
	}
	left[s.PlayerNum] = false
	for more := true; more; {
		more = false
		for k := np - 1; k >= 0; k-- {
			if !left[k] {
				continue
			}
			more = true
			nid := s.Players[k].NetID
			if nid != -1 {
				if n.u8(netGets22+uint32(k)) == 0 {
					// The original spins here without pumping; port-only: the
					// receive side runs so that msg 0x36 can come in.
					g.netUpdate()
					g.e.Sys.KeyWaiting()
					if g.e.Sys.Closed() {
						return host.ErrQuit
					}
					continue
				}
				g.moxSendMessageStale(int32(nid), id, buf, stale)
				for {
					g.e.MoxUpdate()
					if !g.netIsSending() {
						break
					}
				}
			}
			left[k] = false
		}
	}
	return nil
}

// decodeDropOutInfo is Decode_Drop_Out_Info_ (0xfa8a2), msgs 0x37 and 0x38:
// player p is an AI now, with the record he sent; he counts as ready and,
// on the host, as uploaded.
func (g *Game) decodeDropOutInfo(p int, buf []byte) {
	s, n := &g.state, &g.net
	if n.i16(netLastHost) == s.Players[p].NetID {
		n.set16(netLastHost, -1)
	}
	g.makePlayerIntoAI(p)
	setRec(&s.Players[p], buf[:netPlayerSize])
	for i := int(s.NumStars) - 1; i >= 0; i-- {
		s.Stars[i].SurrenderTo[p] = int8(buf[netPlayerSize+i])
	}
	g.decodeDropOutTail(p)
}

// decodeDropOutTail is the end of Decode_Drop_Out_Info_: the counts and the
// host's upload; for a lost player (port-only) also his fixups (netdrop.go).
func (g *Game) decodeDropOutTail(p int) {
	s, n := &g.state, &g.net
	lost := n.dropping != 0
	n.set8(netHumans, n.u8(netHumans)-1)
	n.set8(netReady+uint32(p), 1)
	if lost {
		g.netLostFixups(p)
	}
	if n.i16(netHostID) != n.i16(netOwnID) {
		return
	}
	if int(n.i8(netUploader)) == p {
		n.set8(netUploader, 0xff)
		if lost && n.u8(netState) == 0xa {
			// port-only: his upload never comes; the host takes the next.
			n.upDropped = true
			n.set8(netState, 9)
		}
	}
	n.set8(netWantsSend+uint32(p), 0)
	if lost {
		// port-only: an upload already in is not counted again.
		if n.u8(netHasUpload+uint32(p)) != 0 {
			return
		}
		n.set8(netHasUpload+uint32(p), 1)
	}
	n.set8(netUploaded, n.u8(netUploaded)+1)
	if int16(n.u8(netUploaded)) == s.NumPlayers {
		n.set8(netState, 0xb)
	}
}
