package game

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/openorion2/openorion2/internal/save"
)

// The host's broadcast of the whole game state (msgs 2 and 3) and its
// decode on the clients, ported from NETMOX.C. Spec: docs/net/broadcast.md.

const (
	netBlockSize   = 0xe1000  // the Push_Block_ of the broadcast and of its receipt
	netChunk       = 0x7800   // the largest msg 3
	netGameHost    = 0x1aaf68 // i16: the net id of the game's creator (msg 0's +0)
	netHumans      = 0x1aaf5d // u8: the network humans
	netRecvLen     = 0x1aaf6a // u32: bytes of the broadcast received
	netRecvChunks  = 0x1aaf6e // i16: chunks received
	netChunks      = 0x1aaf70 // i16: chunks announced by msg 2
	netModemErr    = 150      // RSTRING: "modem disk file error" (_modem_disk_error, 0x1aaea4)
	netDiffErr     = 151      // "modem diff buffer error" (0x1aaeac)
	netSendingS    = 156      // "sending chunk %d of %d\n" (0x1aaeb0)
	netDecodeErr   = 0x17a159 // "error decoding broadcast"
	netDecodeErr1  = 0x17a0ff // Decode_Difference_Buffer_'s four messages
	netDecodeErr2  = 0x17a110
	netDecodeErr3  = 0x17a121
	netDecodeErr4  = 0x17a138
	netStarRecSize = 0x71
)

// netRString is RSTRING string i, or fallback.
func (g *Game) netRString(i int, fallback string) string {
	if rs, err := g.rStrings(); err == nil && len(rs) > i {
		return rs[i]
	}
	return fallback
}

// netBroadcastData builds Broadcast_Game_Data_Differential_'s buffer
// (0xf74cd .. 0xf786a) at the start of blk and returns its length; stale
// lists byte ranges the original takes from outside the state.
func (g *Game) netBroadcastData(blk []byte) (int, []netSpan) {
	s := &g.state
	var b bytes.Buffer
	w := func(v any) {
		if err := binary.Write(&b, binary.LittleEndian, v); err != nil {
			panic(err)
		}
	}
	var stale []netSpan
	w(g.net.i16(netGameHost))
	w(s.Stardate)
	w(g.settings[2])
	w(g.settings[0xd4 : 0xd9+1])
	w(g.settings[0xd3])
	g.e.MoxUpdate()
	w(s.Random.Get())
	w(s.NumPlayers)
	for k := 0; k < int(s.NumPlayers); k++ {
		w(&s.Players[k])
	}
	w(s.NumStars)
	// NUM_STARS + 1 records: one past the last star.
	for i := 0; i <= int(s.NumStars); i++ {
		if i < maxStars {
			w(&s.Stars[i])
			continue
		}
		stale = append(stale, netSpan{b.Len(), b.Len() + netStarRecSize})
		w(make([]byte, netStarRecSize))
	}
	w(s.PlanetCount)
	for i := 0; i < int(s.PlanetCount); i++ {
		w(&s.Planets[i])
	}
	w(s.NumColonies)
	w(&s.Colonies)
	w(s.NumShips)
	w(&s.Ships)
	g.e.MoxUpdate()
	w(&s.Leaders)
	w(s.NReports)
	w(&s.Reports)
	w(&s.EventData)
	np := int(s.NumPlayers)
	w(s.FirstMsg[:np])
	w(s.MaxMsgs[:np])
	w(s.NMsgs[:np])
	g.net.msgsOff = b.Len()
	w(&s.Msgs)
	w(s.CouncilFlag)
	w(s.NextCouncilTurn)
	w(&s.Antarans)
	w(s.NumNebulas)
	for i := 0; i < int(s.NumNebulas); i++ {
		w(&s.Nebulas[i])
	}
	w(s.LastEventYear)
	w(s.EventDelayCount)
	w(&s.BillSavegame)
	n := copy(blk, b.Bytes())
	g.e.MoxUpdate()
	return n, stale
}

// makeDifferenceBuffer is Make_Difference_Buffer_ (0xf70eb): the new
// buffer blk[:n] against the previous broadcast, written at blk[n:] (at most
// max bytes); it returns the diff's length. Codes: 0..0xf4 copy that many
// bytes of the previous buffer, 0xf5+k take k literal bytes (0..9), 0xff
// ends. A previous buffer longer than n reads on into the diff itself.
func (g *Game) makeDifferenceBuffer(prev, blk []byte, n, max int) int {
	start, out, limit := n, n, n+max
	pos, p, count, idx := 0, 0, 0, 0
	state := 0
	lit := func(k, from int) { // a literal of k bytes
		blk[out] = byte(k + 0xf5)
		copy(blk[out+1:out+1+k], blk[from:from+k])
		out += 1 + k
	}
	for {
		c := -1
		if pos < len(prev) {
			c = int(prev[pos])
			pos++
		} else if p != n {
			g.exitWithMessage(g.netRString(netModemErr, "modem disk file error"))
		}
		run := p - count
		switch state {
		case 0:
			if c == -1 {
				state = 4
				break
			}
			state = 2
			if c == int(blk[p]) {
				state = 1
			}
			count = 1
		case 1:
			if c == int(blk[p]) {
				count++
				if count == 0xf4 {
					state = 0
					blk[out] = 0xf4
					out++
				}
				break
			}
			blk[out] = byte(count)
			out++
			if c == -1 {
				state = 4
			} else {
				state, count = 2, 1
			}
		case 2, 3:
			if c == -1 {
				lit(count, run)
				state = 4
				break
			}
			if state == 3 && c == int(blk[p]) {
				// Two equal bytes in a row end the literal before them.
				lit(count-1, run)
				state, count = 1, 2
				break
			}
			count++
			if count == 9 {
				lit(9, p-8)
				state = 0
			} else if state == 2 && c == int(blk[p]) {
				state = 3
			} else {
				state = 2
			}
		}
		if out >= limit {
			g.exitWithMessage(g.netRString(netDiffErr, "modem diff buffer error"))
		}
		idx++
		if idx%0x100 == 0 {
			g.e.MoxUpdate()
		}
		p++
		if state == 4 {
			break
		}
	}
	blk[out] = 0xff
	return out + 1 - start
}

// decodeDifferenceBuffer is Decode_Difference_Buffer_ (0xf72d1): the diff
// blk[in:inEnd] against the previous broadcast, written from blk[out] (at
// most max bytes); it returns the decoded length.
func (g *Game) decodeDifferenceBuffer(prev, blk []byte, in, inEnd, out, max int) int {
	start, limit, pos := out, out+max, 0
	fail := func(a uint32) { g.exitWithMessage(cStringAt(g.exe, a)) }
	for in < inEnd {
		b := int(blk[in])
		switch {
		case b < 0xf5:
			// fread copies first and fails after when it ran past the end.
			copy(blk[out:out+b], prev[min(pos, len(prev)):min(pos+b, len(prev))])
			pos += b
			if pos > len(prev) {
				fail(netDecodeErr2)
			}
			g.e.MoxUpdate()
			out += b
			in++
		case b < 0xff:
			k := b - 0xf5
			pos += k
			copy(blk[out:out+k], blk[in+1:in+1+k])
			out += k
			in += k + 1
			g.e.MoxUpdate()
		default:
			in++
			if in > inEnd || limit < out {
				fail(netDecodeErr4)
			}
		}
	}
	return out - start
}

// broadcastGameDataDifferential is Broadcast_Game_Data_Differential_
// (0xf74cd): the state to every human, whole (full) or as a diff against
// the previous broadcast, as msg 2 {chunks} and msgs 3 of up to 0x7800.
func (g *Game) broadcastGameDataDifferential(full bool) error {
	n := &g.net
	if n.broadcastHook != nil {
		n.broadcastHook()
	}
	blk := make([]byte, netBlockSize)
	size, stale := g.netBroadcastData(blk)
	from, end := 0, size
	if full {
		n.prev = append([]byte(nil), blk[:size]...)
	} else {
		d := g.makeDifferenceBuffer(n.prev, blk, size, netBlockSize-size)
		g.e.MoxUpdate()
		from, end = size, size+d
		stale = nil
		n.prev = append(n.prev[:0], blk[:size]...)
	}
	g.e.MoxUpdate()
	g.e.MoxUpdate()
	chunks := int16((end - from + netChunk - 1) / netChunk)
	var cb [2]byte
	binary.LittleEndian.PutUint16(cb[:], uint16(chunks))
	g.moxSendMessage(netAllHumans, 2, cb[:])
	for i, p := 0, from; p < end; i, p = i+1, p+netChunk {
		k := min(end-p, netChunk)
		g.net.tempString = fmt.Sprintf(cFormat(g.netRString(netSendingS, "sending chunk %d of %d\n")), i+1, int(chunks))
		for {
			g.e.V.SetPageOff()
			if err := g.e.InvokeAutoFunction(); err != nil {
				return err
			}
			if err := g.e.TogglePages(); err != nil {
				return err
			}
			g.e.MoxUpdate()
			if !g.netIsSending() {
				break
			}
		}
		var st []netSpan
		for _, sp := range stale {
			if sp[1] > p && sp[0] < p+k {
				st = append(st, netSpan{max(sp[0], p) - p, min(sp[1], p+k) - p})
			}
		}
		g.moxSendMessageStale(netAllHumans, 3, blk[p:p+k], st)
	}
	return nil
}

// decodeBroadcastGameDataDifferential is
// Decode_Broadcast_Game_Data_Differential_ (0xf79f6): blk[:size] (whole
// when full, else a diff against the previous broadcast) into the state;
// true once done (a diff that fails ends the game inside).
func (g *Game) decodeBroadcastGameDataDifferential(blk []byte, size int, full bool) bool {
	n, s := &g.net, &g.state
	data := blk[:size]
	if !full {
		d := g.decodeDifferenceBuffer(n.prev, blk, 0, size, size, netBlockSize-size)
		g.e.MoxUpdate()
		if d == 0 {
			g.exitWithMessage(cStringAt(g.exe, netDecodeErr))
		}
		data = blk[size : size+d]
	}
	n.prev = append(n.prev[:0], data...)
	g.e.MoxUpdate()
	g.e.MoxUpdate()
	r := bytes.NewReader(data)
	rd := func(v any) {
		if err := binary.Read(r, binary.LittleEndian, v); err != nil {
			panic(err)
		}
	}
	var host int16
	rd(&host)
	n.set16(netGameHost, host)
	rd(&s.Stardate)
	rd(&g.settings[2])
	rd(g.settings[0xd4 : 0xd9+1])
	rd(&g.settings[0xd3])
	var seed uint32
	rd(&seed)
	s.Random.Set(seed)
	g.e.MoxUpdate()
	rd(&s.NumPlayers)
	for k := 0; k < int(s.NumPlayers); k++ {
		rd(&s.Players[k])
	}
	n.set8(netHumans, 0)
	for k := int(s.NumPlayers) - 1; k >= 0; k-- {
		pl := &s.Players[k]
		if pl.Eliminated != 0 {
			pl.NetID, pl.Objective = -1, 0
		} else if pl.Objective == save.ObjectiveHuman {
			n.set8(netHumans, n.u8(netHumans)+1)
		}
	}
	rd(&s.NumStars)
	for i := 0; i <= int(s.NumStars); i++ {
		if i < maxStars {
			rd(&s.Stars[i])
		} else {
			r.Seek(netStarRecSize, 1) // past _star's block: not modelled
		}
	}
	g.e.MoxUpdate()
	rd(&s.PlanetCount)
	for i := 0; i < int(s.PlanetCount); i++ {
		rd(&s.Planets[i])
	}
	rd(&s.NumColonies)
	rd(&s.Colonies)
	rd(&s.NumShips)
	rd(&s.Ships)
	rd(&s.Leaders)
	rd(&s.NReports)
	rd(&s.Reports)
	g.clientInitializeReports()
	rd(&s.EventData)
	np := int(s.NumPlayers)
	rd(s.FirstMsg[:np])
	rd(s.MaxMsgs[:np])
	rd(s.NMsgs[:np])
	rd(&s.Msgs)
	g.e.MoxUpdate()
	rd(&s.CouncilFlag)
	rd(&s.NextCouncilTurn)
	rd(&s.Antarans)
	rd(&s.NumNebulas)
	for i := 0; i < int(s.NumNebulas); i++ {
		rd(&s.Nebulas[i])
	}
	rd(&s.LastEventYear)
	rd(&s.EventDelayCount)
	rd(&s.BillSavegame)
	g.e.MoxUpdate()
	return true
}

// clientInitializeReports is Client_Initialize_Reports_ (0xfd7ea): the
// tail of Initialize_Reports_, without the reports and their count.
func (g *Game) clientInitializeReports() {
	s := &g.state
	s.DoneColonizeCheck = [9]byte{}
	s.DonePlayerReports = 0
	s.DisplayedMsgs = 0
	s.DisplayedGNN = 0
	s.Antarans[0x41] = 0
}
