package game

import (
	"encoding/binary"

	"github.com/openorion2/openorion2/internal/save"
)

// Battles in a network game (COMBFIND.C, NETMOX.C): every machine sits in
// Client_Combat_Loop_ while the turn's host hands each battle to the machine
// of a human in it (Do_Net_Combat_Update_), ported from the assembly.
// Spec: docs/net/combat.md.

const (
	netSlotSize  = 12       // Do_Net_Combat_Update_'s slot, also the own block at netState
	netCStar     = 0x1ab040 // i8 +1: the battle's star
	netCShipT    = 0x1ab041 // i16 +2: the ship owner targeted, -1 none
	netCPlanet   = 0x1ab043 // i16 +4: the planet targeted, -1 none
	netCDef      = 0x1ab045 // i16 +6: the defender
	netCAtt      = 0x1ab048 // i8 +9: the attacker (in a slot: the player it is busy with)
	netRussSync  = 0x1ab0c0 // u8: msg 0x40 sets 1, msg 0x41 2 (Russ_Combat_)
	netBribeStar = 0x1ab0cd // i8: msg 0x25's star (Do_Dragon_Bribe_)
	netResultLen = 0x6ce    // Do_1_Combat_'s result record
	netTargetLen = 0x2c     // Copy_Target_Info_: 0x1aa380..0x1aa3ab
	netPresence  = 0x58     // a star's _star_has_player row
	netLeaders   = 0x43     // leaders in the tables Send_*_Info_ walk
	netOfficers  = 0x117    // player+0x117: the 0xd4 bytes a battle may change
	netOfficerSz = 0xd4
	// community patch 1.51: msgs 0x10/0x12 carry the records from +0xc4
	// (FieldStatus on), so the battle's machines agree on the tech fields
	netTargetRec   = 0xc4
	netTargetRecSz = 0x127
	combfindGNN    = 0x1aa3dc // Load_Combfind_Strings_' GNN combat line
	combfindAtt    = 0x1aa3bc // "%s is attacking %s"
	combfindOut    = 0x1aa3d8 // outpost destroyed
	combfindMind   = 0x1aa3b8 // mind control
)

// Slot fields, offsets from netCombat + 12·player.
const (
	slState  = 0
	slStar   = 1
	slShipT  = 2
	slPlanet = 4
	slDef    = 6
	slBusy   = 9
	slResult = 0xa
)

func slot(k int) uint32 { return uint32(int32(netCombat) + int32(k)*netSlotSize) }

// netIsHost is the host test of the combat code: 0x1aaf86 == 0x1aaf5e.
func (g *Game) netIsHost() bool { return g.net.i16(netHostID) == g.net.i16(netOwnID) }

// netIDOf is player k's net id (player+0x2b).
func (g *Game) netIDOf(k int) int32 { return int32(g.state.Players[k].NetID) }

func (g *Game) isHuman(p int) bool {
	return p < 8 && g.state.Players[p].Objective == save.ObjectiveHuman
}

// playerNameOf is Player_Name_ (0xe5de8): the race's name, or a monster's
// (the E_Strings_ pointers at 0x192644 + 4·owner).
func (g *Game) playerNameOf(p int) string {
	if p < 8 {
		return save.CString(g.state.Players[p].RaceName[:]) // player+0x15
	}
	name, err := g.eStringAt(uint32(0x192644 + 4*p))
	g.must2(err)
	return name
}

// gnnReportsCombat is GNN_Reports_Combat_ (0xea8ca): msg 0x2d with the two
// names in random order.
func (g *Game) gnnReportsCombat(a, b int) {
	first, second := b, a
	if g.state.Random.Random(2) == 1 {
		first, second = a, b
	}
	f, err := g.combfindString(combfindGNN)
	if err != nil {
		panic(err)
	}
	line := cSprintf(f, g.playerNameOf(first), g.playerNameOf(second))
	g.sendGNNChatMsg(append([]byte(line), 0))
}

// netAttackedMessage is Do_1_Combat_'s Message_Box_ for the defender at the
// controls in a network game (0xe947c): who attacks where.
func (g *Game) netAttackedMessage(star, att, planet int16) {
	place := save.CString(g.state.Stars[star].Name[:])
	if planet != -1 {
		place = g.planetName(int(planet))
	}
	f, err := g.combfindString(combfindAtt)
	if err != nil {
		panic(err)
	}
	g.must2(g.messageBox(cSprintf(f, g.playerNameOf(int(att)), place)))
}

// clientCombatLoop is Client_Combat_Loop_ (0xea2f1): every machine waits
// here while the host runs the battles (states 0xe..0x22) and fights its
// own human's battles when the host's messages say so.
func (g *Game) clientCombatLoop() error {
	n, s := &g.net, &g.state
	var asked [8][8]byte
	n.resBuf = make([]byte, netResultLen) // 0x1ab04c: the frame's [ebp-0xde8]
	g.asked = &asked
	me := int16(s.PlayerNum)
	for {
		st := n.u8(netState)
		if st < 0xe || st >= 0x23 {
			break
		}
		g.e.MoxUpdate()
		if err := g.chatBoxInputLoop(); err != nil {
			return err
		}
		g.e.MoxUpdate()
		star := int16(n.i8(netCStar))
		switch n.u8(netState) {
		case 0xf: // the chunks of msgs 0x2e/0x2f
			g.waitChunks()
			g.decodePickTargetInfo(n.recv[:n.recvLen()])
			g.popRecv()
		case 0x10: // pick the target (Get_Local_Human_Target_), msg 0x11
			g.e.AssignAutoFunction(func() error { return nil }, 1) // Null_Autofunction_
			g.e.V.SetPageOff()
			g.ms.dontDrawToBack = true
			if err := g.drawMiniMainScreen(); err != nil {
				return err
			}
			g.e.V.CopyOffToBack()
			g.e.V.SetRefreshStencil()
			g.moxPlaySound(0x33)
			var shipT, colT int16
			g.getLocalHumanTarget(star, me, &shipT, &colT)
			n.set16(netCShipT, shipT)
			planet := int16(-1)
			if colT != -1 {
				planet = s.Colonies[colT].Planet
			}
			n.set16(netCPlanet, planet)
			if err := g.loadNetNextTurnScreen(false); err != nil {
				return err
			}
			n.set8(netState, 0x11)
			switch {
			case colT != -1:
				n.set16(netCDef, int16(s.Colonies[colT].Owner))
			case shipT != -1:
				n.set16(netCDef, shipT)
			default:
				n.set16(netCDef, -1)
				n.set8(netState, 0xe)
			}
			g.moxSendMessage(int32(n.i16(netHostID)), 0x11, n.mem[netCShipT-netBase:netCShipT-netBase+6])
		case 0x12: // the chunks of msgs 0x34/0x35
			g.waitChunks()
			g.decodeYouAreTargetInfo(n.recv[:n.recvLen()])
			g.popRecv()
		case 0x13: // the defender: against a human 0x15, else 0x17
			if a := n.i8(netCAtt); a >= 8 || !g.isHuman(int(a)) {
				n.set8(netState, 0x17)
			} else {
				n.set8(netState, 0x15)
			}
			g.moxPlaySound(0x33)
		case 0x14: // the attacker against a human: the result to the defender (msg 0x18)
			res := g.netNewResult()
			r := g.netDo1Combat(star, me, n.i16(netCShipT), res.planet, res, false)
			def := int(n.i16(netCDef))
			b, stale := g.resultBytes(res)
			g.moxSendMessageStale(g.netIDOf(def), 0x18, b, stale)
			for n.u8(netState) != 0xe {
				if n.dropped[def&7] {
					break // port-only: the defender was lost; his msg 0x19 never comes
				}
				g.netPollWait()
			}
			g.sendBattleResults(star, me, int16(def), n.i16(netCShipT), n.i16(netCPlanet), r)
		case 0x15: // the human defender: wait for the attacker's result (msg 0x18)
			att := int16(n.i8(netCAtt))
			r := g.netDo1Combat(star, att, n.i16(netCShipT), n.i16(netCPlanet), nil, false)
			if n.u8(netState) != 0x1a {
				n.set8(netState, 0xe)
			}
			for n.u8(netState) != 0x1a {
				g.netLostOpponent(int(att)) // port-only
				g.netPollWait()
			}
			g.bombResultsPopups(n.resBuf)
			g.moxSendMessage(g.netIDOf(int(att)), 0x19, nil)
			n.set8(netState, 0xe)
			g.sendBattleResults(star, att, me, n.i16(netCShipT), n.i16(netCPlanet), r)
		case 0x16: // the attacker against an AI or a monster
			r := g.netDo1Combat(star, me, n.i16(netCShipT), n.i16(netCPlanet), nil, false)
			n.set8(netState, 0xe)
			g.sendBattleResults(star, me, n.i16(netCDef), n.i16(netCShipT), n.i16(netCPlanet), r)
		case 0x17: // the defender against an AI or a monster
			att := int16(n.i8(netCAtt))
			r := g.netDo1Combat(star, att, n.i16(netCShipT), n.i16(netCPlanet), nil, false)
			n.set8(netState, 0xe)
			if g.e != nil {
				if err := g.loadNetNextTurnScreen(true); err != nil {
					return err
				}
			}
			g.sendBattleResults(star, att, me, n.i16(netCShipT), n.i16(netCPlanet), r)
		case 0x18: // strategic: the attacker resolves, the human defender gets msg 0x16
			res := g.netNewResult()
			r := g.netDo1Combat(star, me, n.i16(netCShipT), res.planet, res, false)
			if err := g.loadNetNextTurnScreen(true); err != nil {
				return err
			}
			if def := int(n.i16(netCDef)); def < 8 && g.isHuman(def) {
				b, stale := g.resultBytes(res)
				g.moxSendMessageStale(g.netIDOf(def), 0x16, b, stale)
			}
			n.set8(netState, 0xe)
			g.sendBattleResults(star, me, n.i16(netCDef), n.i16(netCShipT), n.i16(netCPlanet), r)
		case 0x19: // strategic: the defender sees the result, then msg 0x17
			g.strategicResultsPopup(n.resBuf)
			g.bombResultsPopups(n.resBuf)
			if err := g.loadNetNextTurnScreen(false); err != nil {
				return err
			}
			n.set8(netState, 0xe)
			g.moxSendMessage(int32(n.i16(netHostID)), 0x17, nil)
		case 0x1c: // the Dragon: pay (msg 0x25) or be attacked (msg 0x24)
			bribe := int32(g.dragonNewStarPopup(me, star))
			if err := g.loadNetNextTurnScreen(false); err != nil {
				return err
			}
			if bribe == -1 {
				n.set8(netState, 0x1d)
				g.moxSendMessage(int32(n.i16(netHostID)), 0x24, nil)
			} else {
				n.set8(netState, 0xe)
				g.moxSendMessage(int32(n.i16(netHostID)), 0x25, binary.LittleEndian.AppendUint32(nil, uint32(bribe)))
			}
		case 0x22: // the host: a client's long battle result (msgs 0x32/0x33)
			g.waitChunks()
			g.decodeBattleResults(n.recv[:n.recvLen()])
			g.popRecv()
			n.set8(netState, 0xe)
		}
	}
	n.resBuf = nil
	g.asked = nil
	g.e.MoxUpdate()
	return nil
}

// netNewResult is the client loop's 0x6ce-byte frame record: zeroed, the
// own block's 12 bytes at +0, the planet at +0xd.
func (g *Game) netNewResult() *combatResult {
	n := &g.net
	res := &combatResult{}
	copy(res.head[:], n.mem[netState-netBase:netState-netBase+netSlotSize])
	res.planet = n.i16(netCPlanet)
	res.hasHead = true
	return res
}

// waitChunks is the loop's wait for the chunks a count message announced.
func (g *Game) waitChunks() {
	n := &g.net
	for n.i16(netRecvChunks) < n.i16(netChunks) {
		g.netRestartPoint() // port-only: the chunks come from the turn's host
		g.netPollWait()
	}
}

func (n *netGame) recvLen() int { return int(binary.LittleEndian.Uint32(n.mem[netRecvLen-netBase:])) }

// popRecv is the loop's Pop_Block_ of the received chunks.
func (g *Game) popRecv() {
	n := &g.net
	n.recv = nil
	n.set16(netChunks, 0)
}

// doNetCombatUpdate is Do_Net_Combat_Update_ (0xeaaa1), run by the turn's
// host in states 0xe..0x22: the battle slots, then the next battle; with
// none left and every human idle the combat phase ends (state 0x23).
func (g *Game) doNetCombatUpdate() {
	n, s := &g.net, &g.state
	g.e.MoxUpdate()
	if n.combatBusy {
		g.e.MoxUpdate()
		return
	}
	n.combatBusy = true
	defer func() {
		n.combatBusy = false
		g.e.MoxUpdate()
	}()
	for k := int(s.NumPlayers) - 1; k >= 0; k-- {
		g.e.MoxUpdate()
		sl := slot(k)
		switch n.u8(sl + slState) {
		case 0x11: // the attacker's choice came in (msg 0x11)
			def := int(n.i16(sl + slDef))
			star := int(n.i8(sl + slStar))
			if def == -1 {
				n.set8(sl+slState, 0xe)
				for j := int(s.NumPlayers) - 1; j >= 0; j-- {
					if int(n.i8(slot(j)+slBusy)) == k {
						g.e.MoxUpdate()
						n.set8(slot(j)+slBusy, 0xff)
					}
				}
				n.set8(netPickDone, 0)
				s.Stars[star].IgnoreCombat |= 1 << uint(k)
				continue
			}
			s.Stars[star].IgnoreCombat = 0
			copy(n.mem[slot(def)+slShipT-netBase:slot(def)+slShipT-netBase+6], n.mem[sl+slShipT-netBase:sl+slShipT-netBase+6])
			for j := int(s.NumPlayers) - 1; j >= 0; j-- {
				if j != k && j != def && int(n.i8(slot(j)+slBusy)) == k {
					g.e.MoxUpdate()
					n.set8(slot(j)+slBusy, 0xff)
				}
			}
			g.gnnReportsCombat(k, def)
			switch {
			case g.settings[0xd8] != 0:
				n.set8(sl+slState, 0x18)
				n.set8(slot(def)+slState, 0x19)
			case def >= 8 || !g.isHuman(def):
				n.set8(sl+slState, 0x16)
			default:
				n.set8(sl+slState, 0x14)
				n.set8(slot(def)+slState, 0x15)
				colony := g.targetPlanetToColony(n.i16(sl + slPlanet))
				g.sendYouAreTargetInfo(def, k, n.i16(sl+slShipT), colony, int16(n.i8(sl+slStar)))
			}
			g.moxSendMessage(g.netIDOf(k), 0x14, nil)
		case 0x1b: // a battle's result is in (Decode_Battle_Results_)
			star := int16(n.i8(sl + slStar))
			if busy := int(n.i8(sl + slBusy)); busy == k {
				g.updatePlayerNeedsToAttack(star, int16(k), n.i16(sl+slDef))
			} else {
				g.updatePlayerNeedsToAttack(star, int16(busy), int16(k))
			}
			n.set8(sl+slState, 0xe)
			n.set8(sl+slBusy, 0xff)
			n.set8(netPickDone, 0)
		case 0x1d: // the Dragon was refused (msg 0x24)
			g.doNonhumanAttackAgainstHuman(k)
			*g.nonplayerAt(int(n.i8(sl + slStar))) &^= 0x10
			n.set8(sl+slState, 0x19)
			n.set8(netPickDone, 0)
			n.set8(slot(12)+slBusy, 0xff) // 0x1ab024
		case 0x1e: // the Dragon was paid (msg 0x25)
			g.doDragonBribe(int16(k), int16(n.i8(sl+slStar)), int16(n.i8(netBribeStar)))
			n.set8(sl+slBusy, 0xff)
			n.set8(sl+slState, 0xe)
			n.set8(netPickDone, 0)
			n.set8(slot(12)+slBusy, 0xff)
		case 0x1f: // a long result waits for the host (msg 0x30): go ahead (msg 0x31)
			if n.u8(netState) == 0xe {
				n.set8(netState, 0x21)
				n.set8(sl+slState, 0x20)
				g.moxSendMessage(g.netIDOf(k), 0x31, nil)
			}
		}
	}
	nb := int16(0)
	if g.cf != nil {
		nb = g.cf.nBattle
	}
	me := int(s.PlayerNum)
	if nb == 0 {
		busy := false
		for k := int(s.NumPlayers) - 1; k >= 0; k-- {
			if g.isHuman(k) && n.u8(slot(k)) != 0xe {
				g.e.MoxUpdate()
				busy = true
			}
		}
		if !busy {
			n.set8(netState, 0x23)
		}
		return
	}
	if st := n.u8(slot(me)); st == 0x10 || st == 0xf || n.u8(netPickDone) != 0 {
		return
	}
	star, att := g.pickStarAndAttacker()
	if star == -1 {
		n.set8(netPickDone, 1)
		return
	}
	g.netStartBattle(star, att)
}

// netStartBattle is the rest of Do_Net_Combat_Update_ (0xeae5e): the slots
// of everyone at the star, then the attacker's target and who fights.
func (g *Game) netStartBattle(star, att int16) {
	n, s := &g.net, &g.state
	shipT, colT := int16(-1), int16(-1)
	sneak := false
	for i := int(g.ct.nShipT) - 1; i >= 0; i-- {
		o := int(g.ct.shipT[i])
		n.set8(slot(o)+slStar, uint8(star))
		n.set8(slot(o)+slBusy, uint8(att))
	}
	for i := int(g.ct.nColT) - 1; i >= 0; i-- {
		o := int(s.Colonies[g.ct.colT[i]].Owner)
		n.set8(slot(o)+slStar, uint8(star))
		n.set8(slot(o)+slBusy, uint8(att))
	}
	sa := slot(int(att))
	n.set8(sa+slStar, uint8(star))
	n.set8(sa+slBusy, uint8(att))
	human := g.isHuman(int(att))
	if human && s.Stars[star].IgnoreCombat>>uint(att)&1 == 0 {
		n.set8(sa+slState, 0x10)
		g.sendPickTargetInfo(int(att), star)
		return
	}
	def := int16(-1)
	switch {
	case human:
	case att >= 8:
		g.getNPCTarget(star, att, &shipT, &colT)
	default:
		g.getAITarget(star, att, &shipT, &colT, &sneak)
	}
	if !human {
		if colT != -1 {
			def = int16(s.Colonies[colT].Owner)
			s.Players[def].CombatSlot = uint8(att)
		} else {
			def = shipT
		}
	}
	n.set16(sa+slShipT, shipT)
	n.set16(sa+slPlanet, g.targetColonyToPlanet(colT))
	n.set16(sa+slDef, def)
	// community patch 1.51 (0x402415, 0x4023f5): a slot busy with the attacker
	// is released only at this star; one in his battle elsewhere stays busy.
	here := func(j int) bool { return int16(n.i8(slot(j)+slStar)) == star }
	if def == -1 {
		n.set8(netPickDone, 0)
		for j := int(s.NumPlayers) - 1; j >= 0; j-- {
			if n.i8(slot(j)+slBusy) == int8(att) && here(j) {
				g.e.MoxUpdate()
				n.set8(slot(j)+slBusy, 0xff)
			}
		}
		return
	}
	s.Stars[star].IgnoreCombat = 0
	sd := slot(int(def))
	copy(n.mem[sd+slShipT-netBase:sd+slShipT-netBase+6], n.mem[sa+slShipT-netBase:sa+slShipT-netBase+6])
	for j := int(s.NumPlayers) - 1; j >= 0; j-- {
		if int16(j) != att && int16(j) != def && n.i8(slot(j)+slBusy) == int8(att) && here(j) {
			g.e.MoxUpdate()
			n.set8(slot(j)+slBusy, 0xff)
		}
	}
	if def >= 8 || !g.isHuman(int(def)) {
		planet := int16(-1)
		if colT != -1 {
			planet = s.Colonies[colT].Planet
		}
		// The sneak flag is not passed here (push 0): a quirk of the original.
		g.netDo1Combat(star, att, shipT, planet, nil, false)
		g.updatePlayerNeedsToAttack(star, att, def)
		n.set8(netPickDone, 0)
		n.set8(sa+slBusy, 0xff)
		n.set8(sd+slBusy, 0xff)
		return
	}
	if att == 12 && colT != -1 {
		n.set8(sd+slState, 0x1c)
		g.sendDragonInfo(colT)
		return
	}
	g.doNonhumanAttackAgainstHuman(int(def))
}

// netDo1Combat is Do_1_Combat_ as Client_Combat_Loop_ and
// Do_Net_Combat_Update_ call it in community patch 1.51 (0x404acf): the
// in-battle flag keeps the host's Mox_Net_Update_ from starting another
// battle inside this one (which moved the current star under it).
func (g *Game) netDo1Combat(star, att, def, planet int16, res *combatResult, sneak bool) int8 {
	n := &g.net
	if n.inBattle {
		panic("nested combat detected") // 1.51 exits with this message; unreachable
	}
	n.inBattle = true
	defer func() { n.inBattle = false }()
	return g.do1Combat(star, att, def, planet, res, sneak)
}

// targetColonyToPlanet is Target_Colony_To_Planet_ (0xe69de).
func (g *Game) targetColonyToPlanet(c int16) int16 {
	if c == -1 {
		return -1
	}
	return g.state.Colonies[c].Planet
}

// targetPlanetToColony is Target_Planet_To_Colony_ (0xe69f7).
func (g *Game) targetPlanetToColony(p int16) int16 {
	if p == -1 {
		return -1
	}
	return g.state.Planets[p].Colony
}

// doNonhumanAttackAgainstHuman is Do_Nonhuman_Attack_Against_Human_
// (0xea91b): an AI or a monster attacks human def. In tactical combat his
// machine fights it (msg 0x12), in strategic combat the host does and sends
// him the result (msg 0x16).
func (g *Game) doNonhumanAttackAgainstHuman(def int) {
	n := &g.net
	sd := slot(def)
	att := int(n.i8(sd + slBusy))
	star := int16(n.i8(sd + slStar))
	g.gnnReportsCombat(att, def)
	if g.settings[0xd8] == 0 {
		colony := g.targetPlanetToColony(n.i16(sd + slPlanet))
		g.sendYouAreTargetInfo(def, att, n.i16(sd+slShipT), colony, star)
		n.set8(sd+slState, 0x17)
		return
	}
	res := &combatResult{hasHead: true}
	copy(res.head[:], n.mem[sd-netBase:sd-netBase+netSlotSize])
	res.planet = n.i16(sd + slPlanet)
	r := g.do1Combat(star, int16(att), n.i16(sd+slShipT), n.i16(sd+slPlanet), res, false)
	res.result = r
	g.updatePlayerNeedsToAttack(star, int16(att), int16(def))
	n.set8(netPickDone, 0)
	n.set8(slot(att)+slBusy, 0xff)
	n.set8(sd+slState, 0x19)
	b, stale := g.resultBytes(res)
	g.moxSendMessageStale(g.netIDOf(def), 0x16, b, stale)
}

// sendDragonInfo is Send_Dragon_Info_ (0xfa104): msg 0x23 {s32 colony} to
// the colony's owner.
func (g *Game) sendDragonInfo(colony int16) {
	o := int(g.state.Colonies[colony].Owner)
	g.moxSendMessage(g.netIDOf(o), 0x23, binary.LittleEndian.AppendUint32(nil, uint32(int32(colony))))
}

// decodeDragonInfo is Decode_Dragon_Info_ (0xfa142): the colony's planet and
// star into the own block.
func (g *Game) decodeDragonInfo(buf []byte) {
	s, n := &g.state, &g.net
	c := int32(binary.LittleEndian.Uint32(buf))
	pl := s.Colonies[c].Planet
	n.set16(netCPlanet, pl)
	n.set8(netCStar, s.Planets[pl].Star)
}

// netBuf is the Push_Block_ buffer the Send_*_Info_ routines fill.
type netBuf struct{ b []byte }

func (w *netBuf) u16(v int16)    { w.b = binary.LittleEndian.AppendUint16(w.b, uint16(v)) }
func (w *netBuf) bytes(p []byte) { w.b = append(w.b, p...) }

// netReader reads one back. port-only: past the end it reads 0xff bytes
// (the original reads on in its block), so a list ends.
type netReader struct {
	b   []byte
	off int
}

func (r *netReader) next(k int) []byte {
	for r.off+k > len(r.b) {
		r.b = append(r.b, 0xff)
	}
	p := r.b[r.off : r.off+k]
	r.off += k
	return p
}
func (r *netReader) i16() int16 { return int16(binary.LittleEndian.Uint16(r.next(2))) }

// sendStarPlayerPresence is Send_Star_Player_Presence_ (0xe691a): star's row
// of _star_has_player and its monster byte; star -1 is the battle's
// (0x1aa3e0, 0x1aa3e4).
func (g *Game) sendStarPlayerPresence(w *netBuf, star int) {
	if star != -1 {
		for p := 0; p < 8; p++ {
			w.bytes(g.cf.has[star][p][:])
		}
		w.bytes([]byte{g.cf.nonplayer[star]})
		return
	}
	row := g.curRow()
	for p := 0; p < 8; p++ {
		w.bytes(row[p][:])
	}
	w.bytes([]byte{*g.curNonplayer()})
}

// decodeStarPlayerPresence is Decode_Star_Player_Presence_ (0xe6979). A
// client's pointers are 0 or stale (combatCur).
func (g *Game) decodeStarPlayerPresence(r *netReader, star int) {
	b := r.next(netPresence)
	var row *[8][11]byte
	if star != -1 {
		row = &g.cf.has[star]
	} else {
		row = g.curRow()
	}
	for p := 0; p < 8; p++ {
		copy(row[p][:], b[p*11:])
	}
	v := r.next(1)[0]
	if star != -1 {
		g.cf.nonplayer[star] = v
	} else {
		*g.curNonplayer() = v
	}
}

func (t *combatTargets) bytes() []byte {
	var w netBuf
	for _, v := range t.colT {
		w.u16(v)
	}
	w.u16(t.nColT)
	for _, v := range t.shipT {
		w.u16(v)
	}
	w.u16(t.nShipT)
	return w.b
}

// copyTargetInfo is Copy_Target_Info_ (0xe6903).
func (g *Game) copyTargetInfo(b []byte) {
	r := &netReader{b: b}
	t := &g.ct
	for i := range t.colT {
		t.colT[i] = r.i16()
	}
	t.nColT = r.i16()
	for i := range t.shipT {
		t.shipT[i] = r.i16()
	}
	t.nShipT = r.i16()
}

// The parts of the save records the battle messages carry.
func (g *Game) officerBytes(p int) []byte {
	return g.playerRecord(p)[netOfficers : netOfficers+netOfficerSz]
}

func (g *Game) setOfficerBytes(p int, b []byte) {
	rec := g.playerRecord(p)
	copy(rec[netOfficers:], b[:netOfficerSz])
	setRec(&g.state.Players[p], rec)
}

// targetRecBytes are the record bytes msgs 0x10 and 0x12 carry (community
// patch 1.51: tech field status, applications and the rest up to +0x1eb).
func (g *Game) targetRecBytes(p int) []byte {
	return g.playerRecord(p)[netTargetRec : netTargetRec+netTargetRecSz]
}

func (g *Game) setTargetRecBytes(p int, b []byte) {
	rec := g.playerRecord(p)
	copy(rec[netTargetRec:], b[:netTargetRecSz])
	setRec(&g.state.Players[p], rec)
}

func (g *Game) starByte(star, off int) byte { return recBytes(&g.state.Stars[star])[off] }

func (g *Game) setStarByte(star, off int, v byte) {
	b := recBytes(&g.state.Stars[star])
	b[off] = v
	setRec(&g.state.Stars[star], b)
}

func (g *Game) planetBytes(pl int) []byte { return recBytes(&g.state.Planets[pl]) }

// sendNetMsg sends a Send_*_Info_ buffer to net id to: up to 0x7800 bytes as
// one message, else a chunk count and chunks of 0x7800, a chat loop each.
// community patch 1.51: was 0x8000 in a 0x7800 buffer, a chat loop per
// Net_Is_Sending_; the longer records need the 0xff00 buffer and chunks.
func (g *Game) sendNetMsg(to int32, one, count, chunk uint16, b []byte) {
	if len(b) <= netChunk {
		g.moxSendMessage(to, one, b)
		return
	}
	chunks := (len(b) + netChunk - 1) / netChunk
	g.moxSendMessage(to, count, binary.LittleEndian.AppendUint16(nil, uint16(chunks)))
	for p := 0; p < len(b); p += netChunk {
		k := min(len(b)-p, netChunk)
		g.e.MoxUpdate()
		g.must2(g.chatBoxInputLoop())
		g.moxSendMessage(to, chunk, b[p:p+k])
	}
}

// sendPickTargetInfo is Send_Pick_Target_Info_ (0xf88d8): the attacker's
// machine gets the targets and, unless it is the host, the star's presence,
// every player's officers, every leader, the planets there with their
// colonies and the ships there (msg 0x10, or 0x2e and 0x2f).
func (g *Game) sendPickTargetInfo(p int, star int16) {
	s := &g.state
	var w netBuf
	w.u16(star)
	w.bytes(g.ct.bytes())
	if g.net.i16(netHostID) != int16(g.netIDOf(p)) {
		g.sendStarPlayerPresence(&w, int(star))
		for k := int(s.NumPlayers) - 1; k >= 0; k-- {
			w.bytes(g.targetRecBytes(k)) // community patch 1.51: from +0xc4
		}
		g.e.MoxUpdate()
		for l := netLeaders - 1; l >= 0; l-- {
			w.u16(int16(l))
			w.bytes(recBytes(&s.Leaders[l]))
			g.e.MoxUpdate()
		}
		w.u16(-1)
		for k := int(s.NumPlayers) - 1; k >= 0; k-- {
			w.bytes([]byte{g.starByte(int(star), 0x42+k)})
		}
		g.e.MoxUpdate()
		w.u16(s.NumColonies)
		g.writeStarPlanets(&w, int(star))
		g.e.MoxUpdate()
		w.u16(s.NumShips)
		for i := 0; i < int(s.NumShips); i++ {
			if sh := &s.Ships[i]; sh.Location == star && sh.Status != 5 {
				w.u16(int16(i))
				w.bytes(recBytes(sh))
				g.e.MoxUpdate()
			}
		}
		w.u16(-1)
	}
	g.e.MoxUpdate()
	g.sendNetMsg(g.netIDOf(p), 0x10, 0x2e, 0x2f, w.b)
}

// writeStarPlanets writes each planet of star from the last orbit: its
// colony, bytes +4 and +8, the colony again and the colony's record.
func (g *Game) writeStarPlanets(w *netBuf, star int) {
	s := &g.state
	for o := 4; o >= 0; o-- {
		pl := s.Stars[star].Planets[o]
		if pl == -1 {
			continue
		}
		pb := g.planetBytes(int(pl))
		w.bytes(pb[0:2])
		w.bytes(pb[4:5])
		w.bytes(pb[8:9])
		w.bytes(pb[0:2])
		if c := s.Planets[pl].Colony; c != -1 {
			w.bytes(recBytes(&s.Colonies[c]))
		}
	}
}

// decodePickTargetInfo is Decode_Pick_Target_Info_ (0xf8c8e), on the
// attacker's machine: its state becomes 0x10 (pick a target).
func (g *Game) decodePickTargetInfo(buf []byte) int {
	s, n := &g.state, &g.net
	r := &netReader{b: append([]byte(nil), buf...)}
	star := int(r.i16())
	n.set8(netCStar, uint8(star))
	n.set8(netCAtt, uint8(s.PlayerNum))
	g.copyTargetInfo(r.next(netTargetLen))
	g.e.MoxUpdate()
	if !g.netIsHost() {
		g.decodeStarPlayerPresence(r, -1)
		for k := int(s.NumPlayers) - 1; k >= 0; k-- {
			g.setTargetRecBytes(k, r.next(netTargetRecSz)) // community patch 1.51
		}
		for l := netLeaders - 1; l >= 0; l-- {
			s.Leaders[l].Owner = -1
		}
		for {
			l := r.i16()
			if l == -1 {
				break
			}
			setRec(&s.Leaders[l], r.next(0x3b))
			g.e.MoxUpdate()
		}
		g.e.MoxUpdate()
		for k := int(s.NumPlayers) - 1; k >= 0; k-- {
			g.setStarByte(star, 0x42+k, r.next(1)[0])
		}
		g.e.MoxUpdate()
		s.NumColonies = r.i16()
		for o := 4; o >= 0; o-- {
			pl := s.Stars[star].Planets[o]
			if pl == -1 {
				continue
			}
			g.readStarPlanet(r, int(pl), true)
		}
		g.e.MoxUpdate()
		s.NumShips = r.i16()
		for i := int(s.NumShips) - 1; i >= 0; i-- {
			s.Ships[i].Status = 5
		}
		for {
			i := r.i16()
			if i == -1 {
				break
			}
			setRec(&s.Ships[i], r.next(0x81))
			g.e.MoxUpdate()
		}
	}
	g.updateShipIconsInBetweenTurns()
	n.set8(slot(int(s.PlayerNum)), 0x10)
	n.set8(netState, 0x10)
	return r.off
}

// readStarPlanet reads writeStarPlanets' entry for planet pl; with clear,
// the colony the first index names loses owner and planet first.
func (g *Game) readStarPlanet(r *netReader, pl int, clear bool) {
	s := &g.state
	P := &s.Planets[pl]
	pb := g.planetBytes(pl)
	copy(pb[0:2], r.next(2))
	pb[4] = r.next(1)[0]
	pb[8] = r.next(1)[0]
	setRec(P, pb)
	if clear && P.Colony != -1 {
		s.Colonies[P.Colony].Owner = -1
		s.Colonies[P.Colony].Planet = -1
	}
	copy(pb[0:2], r.next(2))
	setRec(P, pb)
	if c := P.Colony; c != -1 {
		setRec(&s.Colonies[c], r.next(0x169))
	}
}

// sendYouAreTargetInfo is Send_You_Are_Target_Info_ (0xf8f98): the human
// defender's machine gets the battle (msg 0x12, or 0x34 and 0x35).
func (g *Game) sendYouAreTargetInfo(def, att int, shipT, colony, star int16) {
	s := &g.state
	var w netBuf
	w.u16(star)
	w.u16(int16(att))
	w.u16(shipT)
	w.u16(colony)
	if g.net.i16(netHostID) != int16(g.netIDOf(def)) {
		g.sendStarPlayerPresence(&w, int(star))
		g.e.MoxUpdate()
		// community patch 1.51: the records from +0xc4
		if att < 8 {
			w.bytes(g.targetRecBytes(att))
		}
		if def < 8 {
			w.bytes(g.targetRecBytes(def))
		}
		g.e.MoxUpdate()
		g.writeLeadersOf(&w, att, def)
		if att < 8 {
			w.bytes([]byte{g.starByte(int(star), 0x42+att)})
		}
		if def < 8 {
			w.bytes([]byte{g.starByte(int(star), 0x42+def)})
		}
		g.e.MoxUpdate()
		if colony != -1 {
			pl := s.Colonies[colony].Planet
			w.u16(s.NumColonies)
			w.u16(pl)
			pb := g.planetBytes(int(pl))
			w.bytes(pb[0:2])
			w.bytes(pb[4:5])
			w.bytes(pb[8:9])
			w.bytes(recBytes(&s.Colonies[colony]))
		}
		g.e.MoxUpdate()
		g.writeShipsOf(&w, star, att, def)
		g.e.MoxUpdate()
	}
	g.sendNetMsg(g.netIDOf(def), 0x12, 0x34, 0x35, w.b)
}

// writeLeadersOf writes the leaders of a or b, from the last; then -1.
func (g *Game) writeLeadersOf(w *netBuf, a, b int) {
	s := &g.state
	for l := netLeaders - 1; l >= 0; l-- {
		if o := int(s.Leaders[l].Owner); o == a || o == b {
			w.u16(int16(l))
			w.bytes(recBytes(&s.Leaders[l]))
		}
	}
	w.u16(-1)
}

// writeShipsOf writes NUM_SHIPS and the ships of a or b at star (not
// status 5); then -1.
func (g *Game) writeShipsOf(w *netBuf, star int16, a, b int) {
	s := &g.state
	w.u16(s.NumShips)
	for i := 0; i < int(s.NumShips); i++ {
		sh := &s.Ships[i]
		if sh.Location != star || sh.Status == 5 {
			continue
		}
		if o := int(sh.Owner); o == a || o == b {
			w.u16(int16(i))
			w.bytes(recBytes(sh))
		}
	}
	w.u16(-1)
}

// decodeYouAreTargetInfo is Decode_You_Are_Target_Info_ (0xf93e3), on the
// defender's machine: state 0x13.
func (g *Game) decodeYouAreTargetInfo(buf []byte) int {
	s, n := &g.state, &g.net
	me := int(s.PlayerNum)
	r := &netReader{b: append([]byte(nil), buf...)}
	star := int(r.i16())
	n.set8(netCStar, uint8(star))
	att := int(r.i16())
	n.set8(netCAtt, uint8(att))
	n.set16(netCShipT, r.i16())
	colony := r.i16()
	n.set16(netCDef, int16(me))
	if !g.netIsHost() {
		g.decodeStarPlayerPresence(r, -1)
		// community patch 1.51: the records from +0xc4
		if att < 8 {
			g.setTargetRecBytes(att, r.next(netTargetRecSz))
		}
		if me < 8 {
			g.setTargetRecBytes(me, r.next(netTargetRecSz))
		}
		for l := netLeaders - 1; l >= 0; l-- {
			if o := int(s.Leaders[l].Owner); o == att || o == me {
				s.Leaders[l].Owner = -1
			}
		}
		for {
			l := r.i16()
			if l == -1 {
				break
			}
			setRec(&s.Leaders[l], r.next(0x3b))
		}
		if att < 8 {
			g.setStarByte(star, 0x42+att, r.next(1)[0])
		}
		if me < 8 {
			g.setStarByte(star, 0x42+me, r.next(1)[0])
		}
		if colony != -1 {
			s.NumColonies = r.i16()
			pl := int(r.i16())
			pb := g.planetBytes(pl)
			copy(pb[0:2], r.next(2))
			pb[4] = r.next(1)[0]
			pb[8] = r.next(1)[0]
			setRec(&s.Planets[pl], pb)
			setRec(&s.Colonies[colony], r.next(0x169))
		}
		s.NumShips = r.i16()
		for i := int(s.NumShips) - 1; i >= 0; i-- {
			s.Ships[i].Status = 5
		}
		for {
			i := r.i16()
			if i == -1 {
				break
			}
			setRec(&s.Ships[i], r.next(0x81))
		}
	}
	n.set8(netState, 0x13)
	n.set16(netCPlanet, g.targetColonyToPlanet(colony))
	return r.off
}

// sendBattleResults is Send_Battle_Results_ (0xf97cd): the fighting
// machine's battle to the host (msg 0x15, or 0x30..0x33). The host's own,
// and a defender's against a human attacker, carry only the header.
func (g *Game) sendBattleResults(star, att, def, shipT, planet int16, result int8) {
	s, n := &g.state, &g.net
	var w netBuf
	w.u16(s.PlayerNum)
	w.u16(att)
	w.u16(def)
	w.u16(int16(result))
	short := g.netIsHost() || (s.PlayerNum == def && att < 8 && g.isHuman(int(att)))
	if !short {
		w.u16(star)
		g.sendStarPlayerPresence(&w, -1)
		w.u16(shipT)
		w.u16(planet)
		g.e.MoxUpdate()
		if att < 8 {
			w.bytes(g.officerBytes(int(att)))
			w.bytes([]byte{g.starByte(int(star), 0x42+int(att))})
		}
		if def < 8 {
			w.bytes(g.officerBytes(int(def)))
			w.bytes([]byte{g.starByte(int(star), 0x42+int(def))})
		}
		g.e.MoxUpdate()
		g.writeLeadersOf(&w, int(att), int(def))
		g.e.MoxUpdate()
		if planet != -1 {
			pb := g.planetBytes(int(planet))
			c := s.Planets[planet].Colony
			w.bytes(pb[0:2])
			w.bytes(pb[4:5])
			w.bytes(pb[8:9])
			w.u16(c)
			if c != -1 {
				w.bytes(recBytes(&s.Colonies[c]))
			}
		}
		g.e.MoxUpdate()
		g.writeShipsOf(&w, star, int(att), int(def))
		g.e.MoxUpdate()
	}
	host := int32(n.i16(netHostID))
	if len(w.b) <= 0x8000 {
		g.moxSendMessage(host, 0x15, w.b)
		return
	}
	chunks := (len(w.b) + netChunk - 1) / netChunk
	n.set8(netState, 0x1f)
	g.moxSendMessage(host, 0x30, nil)
	for n.u8(netState) != 0x20 {
		g.netPollWait()
	}
	n.set8(netState, 0xe)
	g.moxSendMessage(host, 0x32, binary.LittleEndian.AppendUint16(nil, uint16(chunks)))
	for p := 0; p < len(w.b); p += netChunk {
		k := min(len(w.b)-p, netChunk)
		g.e.MoxUpdate()
		for {
			g.must2(g.chatBoxInputLoop())
			if !g.netIsSending() {
				break
			}
		}
		g.moxSendMessage(host, 0x33, w.b[p:p+k])
	}
}

// decodeBattleResults is Decode_Battle_Results_ (0xf9c58), on the host:
// the sender's slot becomes 0x1b.
func (g *Game) decodeBattleResults(buf []byte) int {
	s, n := &g.state, &g.net
	r := &netReader{b: append([]byte(nil), buf...)}
	sender := r.i16()
	att := r.i16()
	def := r.i16()
	res := r.next(2)[0]
	n.set8(slot(int(att))+slResult, res)
	n.set8(slot(int(def))+slResult, res)
	g.e.MoxUpdate()
	short := sender == s.PlayerNum || (sender == def && att < 8 && g.isHuman(int(att)))
	if !short {
		star := r.i16()
		g.decodeStarPlayerPresence(r, int(star))
		r.i16() // the ship owner target
		planet := r.i16()
		g.e.MoxUpdate()
		if att < 8 {
			g.setOfficerBytes(int(att), r.next(netOfficerSz))
			g.setStarByte(int(star), 0x42+int(att), r.next(1)[0])
		}
		if def < 8 {
			g.setOfficerBytes(int(def), r.next(netOfficerSz))
			g.setStarByte(int(star), 0x42+int(def), r.next(1)[0])
		}
		for {
			l := r.i16()
			if l == -1 {
				break
			}
			setRec(&s.Leaders[l], r.next(0x3b))
		}
		g.e.MoxUpdate()
		if planet != -1 {
			P := &s.Planets[planet]
			old := P.Colony
			pb := g.planetBytes(int(planet))
			copy(pb[0:2], r.next(2))
			pb[4] = r.next(1)[0]
			pb[8] = r.next(1)[0]
			setRec(P, pb)
			c := r.i16()
			if c == -1 {
				if old != -1 {
					g.destroyColony(int(old), 1)
				}
			} else {
				setRec(&s.Colonies[c], r.next(0x169))
				s.computeStarColonyStuff(int(star))
			}
		}
		g.e.MoxUpdate()
		s.NumShips = r.i16()
		k := int16(0)
		kill := func(i int16) {
			sh := &s.Ships[i]
			if o := int16(sh.Owner); (o == att || o == def) && sh.Status != 5 && sh.Location == star {
				sh.Officer = -1
				g.killShip(int(i))
			}
		}
		for {
			i := r.i16()
			if i != -1 {
				for ; k < i; k++ {
					kill(k)
				}
				setRec(&s.Ships[i], r.next(0x81))
				k++
			}
			g.e.MoxUpdate()
			if i == -1 {
				break
			}
		}
		for ; k < s.NumShips; k++ {
			kill(k)
		}
		if n.battleHook != nil {
			n.battleHook(int(sender), int(att), int(def), int(star), int(planet))
		}
	}
	g.updateShipIconsInBetweenTurns()
	n.set8(slot(int(sender)), 0x1b)
	return r.off
}

// strategicResultsPopup is Strategic_Results_Popup_ (0xe859d): the
// defender's view of a strategic battle from the attacker's record.
func (g *Game) strategicResultsPopup(b []byte) {
	if g.e == nil {
		return
	}
	g.moxPlaySound(0x33)
	g.updateShipIconsInBetweenTurns()
	title := save.CString(b[0x281:0x2e5])
	g.must2(g.doTextBox(&title, save.CString(b[0x2e5:0x6cd]), 0))
	if n := int16(binary.LittleEndian.Uint16(b[0x27f:])); n != 0 {
		g.must2(g.playTakeOrionFlic())
		g.loknarTechs = g.loknarTechs[:0]
		for k := 0; k < int(n) && k < 4; k++ {
			g.loknarTechs = append(g.loknarTechs, int16(binary.LittleEndian.Uint16(b[0x277+2*k:])))
		}
		g.must2(g.rescuedLoknarPopup())
	}
	g.must2(g.loadNetNextTurnScreen(false))
}

// bombResultsPopups is Bomb_Results_Popups_ (0xe85f7): what the other
// machine's battle did to the defender's colony.
func (g *Game) bombResultsPopups(b []byte) {
	if g.e == nil {
		return
	}
	switch {
	case b[0xc] != 0:
		f, err := g.combfindString(combfindOut)
		g.must2(err)
		g.must2(g.messageBox(f))
	case b[0x276] != 0:
		g.must2(g.playPlanetaryDestructionFlic())
		g.messedUpFleetPalette = true
	case b[0xf+0x41] != 0:
		var hits bombHits
		setRec(&hits, b[0xf:0xf+0x49])
		if hits.Kind == 0xa {
			g.must2(g.playAmoebaFlic())
		}
		g.colonyBombingScreen(&hits)
	}
	if binary.LittleEndian.Uint16(b[0x64:]) != 0 {
		// 0xe8640: the attacker's marines: Colony_Combat_Screen_ replays the
		// ground battle from the record's forces and seed, at the colony of the
		// block's planet, between a saved and a restored seed.
		s := &g.state
		seed := s.Random.Get()
		A, B := parseGroundForce(b[0x58:]), parseGroundForce(b[0x8f:])
		ci := s.Planets[int16(binary.LittleEndian.Uint16(b[4:]))].Colony
		g.must2(g.colonyCombatScreen(&A, &B, binary.LittleEndian.Uint32(b[0xc6:]), ci))
		g.must2(g.loadNetNextTurnScreen(true))
		s.Random.Set(seed)
	}
	if b[0x6cd] != 0 {
		f, err := g.combfindString(combfindMind)
		g.must2(err)
		g.must2(g.messageBox(f))
	}
}

// resultBytes is the 0x6ce-byte record of res as msgs 0x16 and 0x18 carry
// it; the ground forces' pointers and the parts the port does not keep are
// marked as stale.
func (g *Game) resultBytes(res *combatResult) ([]byte, []netSpan) {
	b := make([]byte, netResultLen)
	copy(b, res.head[:])
	b[0xa] = byte(res.result)
	b[0xc] = res.outpost
	binary.LittleEndian.PutUint16(b[0xd:], uint16(res.planet))
	copy(b[0xf:0x58], recBytes(&res.bomb))
	b[0x276] = res.converted
	for k, a := range res.loknar {
		if k < 4 {
			binary.LittleEndian.PutUint16(b[0x277+2*k:], uint16(a))
		}
	}
	binary.LittleEndian.PutUint16(b[0x27f:], uint16(len(res.loknar)))
	copy(b[0x281:0x2e4], res.title)
	copy(b[0x2e5:0x6cc], res.text)
	b[0x6cd] = res.mind
	var stale []netSpan
	if inv := &res.invasion; inv[0].player != 0 || len(inv[0].ships) != 0 {
		// Get_Invasion_Info_'s two forces (memset first), their ship lists'
		// pointers this machine's own.
		copy(b[0x58:], inv[0].bytes())
		copy(b[0x8f:], inv[1].bytes())
		stale = append(stale, netSpan{0x58 + 0x1c, 0x58 + 0x20}, netSpan{0x8f + 0x1c, 0x8f + 0x20})
		if rep := &res.capture; !rep.invaded {
			stale = append(stale, netSpan{0xc6, 0x276}) // no Invade_: the frame's
		} else {
			binary.LittleEndian.PutUint32(b[0xc6:], res.seed)
			for k := 0; k < int(rep.n) && k < 4; k++ {
				binary.LittleEndian.PutUint16(b[0xca+2*k:], uint16(rep.apps[k]))
			}
			binary.LittleEndian.PutUint16(b[0x272:], uint16(rep.n))
			stale = append(stale, netSpan{0xca + 2*int(rep.n), 0x272})
			if rep.captured {
				binary.LittleEndian.PutUint16(b[0x274:], uint16(rep.planet))
			} else {
				stale = append(stale, netSpan{0x274, 0x276})
			}
		}
	}
	return b, stale
}

// russNetTactical is Russ_Combat_'s tactical battle between two humans in a
// network game (0xe759b): msgs 0x40 and 0x41 around Tactical_Combat_.
func (g *Game) russNetTactical(att, def int16, attList, defList []int16, star, colony int16, canRetreat bool) int16 {
	s, n := &g.state, &g.net
	other := att
	if att == s.PlayerNum {
		other = def
	}
	g.moxSendMessage(g.netIDOf(int(other)), 0x40, nil)
	for n.u8(netRussSync) == 0 {
		g.netPollWait()
	}
	r := g.tacticalCombat(attList, defList, star, colony, canRetreat)
	g.moxSendMessage(g.netIDOf(int(other)), 0x41, nil)
	for n.u8(netRussSync) == 1 {
		g.netPollWait()
	}
	n.set8(netRussSync, 0)
	return r
}

// russChangeRelations is Russ_Change_Relations_ (0xf975c): in a network game
// msg 0x43 asks the host to change them (Change_Relations_ there).
func (g *Game) russChangeRelations(v int16, a, b int, why, arg5, arg6 int16) {
	if g.gameType != 2 && g.gameType != 3 {
		g.changeRelations(v, a, b, why, arg5, arg6)
		return
	}
	var w netBuf
	for _, x := range []int16{v, int16(a), int16(b), why, arg5, arg6} {
		w.u16(x)
	}
	g.moxSendMessage(int32(g.net.i16(netHostID)), 0x43, w.b)
}

// russEventMonsterUpdate is Russ_Event_Monster_Update_ (0xf9711): in a
// network game msg 0x3c {3 × s32} to the host.
func (g *Game) russEventMonsterUpdate(monster int, loc, star int16) {
	if g.gameType != 2 && g.gameType != 3 {
		g.state.eventMonsterUpdate(monster, loc, star)
		return
	}
	b := binary.LittleEndian.AppendUint32(nil, uint32(int32(monster)))
	b = binary.LittleEndian.AppendUint32(b, uint32(int32(loc)))
	b = binary.LittleEndian.AppendUint32(b, uint32(int32(star)))
	g.moxSendMessage(int32(g.net.i16(netHostID)), 0x3c, b)
}

// netCombatCase runs Main_Receive_Message_'s combat cases (phase 4).
func (g *Game) netCombatCase(c uint32, player int32, buf, sent []byte) bool {
	n, s := &g.net, &g.state
	p := int(player)
	switch c {
	case 0xf5fd2: // msg 0xf: every slot cleared, state 0xe
		for i := uint32(0); i < 0xb4; i++ {
			n.set8(netCombat+i, 0)
		}
		n.set8(netState, 0xe)
	case 0xf5fe3: // msg 0x19: the defender saw the result
		n.set8(netState, 0xe)
	case 0xf5fef: // msg 0x10: pick a target
		n.set8(netState, 0x10)
		g.decodePickTargetInfo(sent)
	case 0xf6009: // msg 0x11 {shipT, planet, def}: the attacker's choice
		copy(n.mem[slot(p)+slShipT-netBase:slot(p)+slShipT-netBase+6], buf[:6])
		n.set8(slot(p), 0x11)
	case 0xf6032: // msg 0x12: you are the target
		g.decodeYouAreTargetInfo(sent)
	case 0xf6045: // msg 0x13
		n.set8(netState, 0x14)
	case 0xf6051: // msg 0x14: fight
		switch d := int(n.i16(netCDef)); {
		case g.settings[0xd8] != 0:
			n.set8(netState, 0x18)
		case d >= 8 || !g.isHuman(d):
			n.set8(netState, 0x16)
		default:
			n.set8(netState, 0x14)
		}
	case 0xf609e: // msg 0x15: a battle's results
		g.decodeBattleResults(sent)
	case 0xf60b1, 0xf60f0: // msgs 0x16, 0x18: the other side's result record
		if n.resBuf == nil {
			n.resBuf = make([]byte, netResultLen) // port-only: outside the loop
		}
		copy(n.resBuf, buf[:netResultLen])
		if c == 0xf60b1 {
			n.set8(netState, 0x19)
		} else {
			n.set8(netState, 0x1a)
		}
	case 0xf60ce: // msg 0x17: the defender saw the strategic result
		n.set8(slot(p), 0xe)
		n.set8(slot(p)+slBusy, 0xff)
		n.set8(netPickDone, 0)
	case 0xf6406: // msg 0x23: the Dragon
		g.decodeDragonInfo(buf)
		n.set8(netState, 0x1c)
	case 0xf643d: // msg 0x24: the Dragon refused
		n.set8(slot(p), 0x1d)
	case 0xf644d: // msg 0x25 {s32 star}: the Dragon paid
		n.set8(slot(p), 0x1e)
		n.set8(netBribeStar, buf[0])
	case 0xf6594, 0xf65e4, 0xf6600: // msgs 0x2e, 0x32, 0x34 {u16 chunks}
		n.set16(netChunks, int16(binary.LittleEndian.Uint16(buf)))
		switch c {
		case 0xf6594:
			n.set8(netState, 0xf)
		case 0xf65e4:
			n.set8(netState, 0x22)
		default:
			n.set8(netState, 0x12)
		}
		n.set16(netRecvChunks, 0)
		binary.LittleEndian.PutUint32(n.mem[netRecvLen-netBase:], 0)
		n.recv = make([]byte, netBlockSize)
	case 0xf65b0: // msgs 0x2f, 0x35: a chunk
		if n.recv == nil {
			n.recv = make([]byte, netBlockSize) // port-only: no count came first
		}
		copy(n.recv[int(n.i16(netRecvChunks))*netChunk:], sent)
		n.set16(netRecvChunks, n.i16(netRecvChunks)+1)
		binary.LittleEndian.PutUint32(n.mem[netRecvLen-netBase:], uint32(n.recvLen()+len(sent)))
	case 0xf65c8: // msg 0x30: a long result waits
		n.set8(slot(p), 0x1f)
	case 0xf65d8: // msg 0x31: send it
		n.set8(netState, 0x20)
	case 0xf66c4: // msg 0x3c: a monster moved or died
		s.eventMonsterUpdate(int(int16(binary.LittleEndian.Uint16(buf))),
			int16(binary.LittleEndian.Uint16(buf[4:])), int16(binary.LittleEndian.Uint16(buf[8:])))
	case 0xf6711: // msg 0x40: the other side is in Russ_Combat_
		n.set8(netRussSync, 1)
	case 0xf671d: // msg 0x41: it left Tactical_Combat_
		n.set8(netRussSync, 2)
	case 0xf673e: // msg 0x43: Change_Relations_
		w := func(o int) int16 { return int16(binary.LittleEndian.Uint16(buf[o:])) }
		g.changeRelations(w(0), int(w(2)), int(w(4)), w(6), w(8), w(10))
	default:
		return false
	}
	return true
}

// someStarPlayerIsBusy is Some_Star_Player_Is_Busy_ (0xe841a): a human with
// ships or colonies at star is in a battle, or (the host itself) waits for
// a long result or holds received chunks.
func (g *Game) someStarPlayerIsBusy(star int16) bool {
	s, n := &g.state, &g.net
	for k := int(s.NumPlayers) - 1; k >= 0; k-- {
		if !g.isHuman(k) {
			continue
		}
		if b := &g.cf.has[star][k]; b[7] == 0 && b[6] == 0 {
			continue
		}
		if n.i8(slot(k)+slBusy) != -1 {
			return true
		}
		if k == int(s.PlayerNum) {
			if st := n.u8(netState); st >= 0x21 && st <= 0x22 || n.recv != nil {
				return true
			}
		}
	}
	return false
}

// staleCur: Search_For_Battles_ returns without clearing 0x1aa3e0/0x1aa3e4
// in a network game, so they point into its dead frame (kept as a copy).
func (g *Game) staleCur() {
	c := &g.cc
	if c.has != nil {
		c.stale, c.has = *c.has, &c.stale
	}
	if c.np != nil {
		c.staleNP, c.np = *c.np, &c.staleNP
	}
}

// netPollWait is netWait (netstart.go) in the combat loops that wait for a
// message (port-only: the headless host yields a frame).
func (g *Game) netPollWait() { g.must2(g.netWait()) }
