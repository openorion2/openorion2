package game

import "encoding/binary"

// The tactical battle between two humans in a network game: both machines
// run Tactical_Combat_; the side whose ship moves sends each action as msg
// 0x3e8 with the RNG seed, the other replays it, and both meet at a barrier
// (msg 0x3ec) after every action. Spec: docs/net/combat.md, "The lockstep".

const (
	tacMsgSize  = 0x10 // _new_message (0x190460)
	tacWaitText = 0x37 // Ken text: waiting for the opponent
)

// Action codes of msg 0x3e8 (_tactical_net_choice).
const (
	tacSelfDestruct = 0x2710 + iota
	tacAuto
	_
	_
	tacDone
	tacMove
	tacFire
	tacMslWarning
	tacAutoField
	tacWait
	tacWeapon
	tacFlee
	tacTurnToFace
	tacStasisOff
	tacSetCur
	tacBoard
	tacEndOfTurn
	tacAllWait // community patch 1.51: H (0x41b192; 0x40b3c2 replays it)
)

// netTac is the lockstep's state of the tactical engine (per battle) and of
// NETMOX's receive side (globals that outlive a battle).
type netTac struct {
	msg    [tacMsgSize]byte // _new_message 0x190460
	choice int16            // _tactical_net_choice 0x1999ac
	done   int8             // _combat_done_waiting 0x199f16
}

// sendTacticalCombatMessage is Send_Tactical_Combat_Message_ (0x42b0f): msg
// 0x3e8 to player to: the view's square, the seed, the action and its three
// arguments.
func (g *Game) sendTacticalCombatMessage(to, code, bx, cx, arg int16) {
	tc := g.tc
	b := make([]byte, tacMsgSize)
	binary.LittleEndian.PutUint16(b[0:], uint16(tc.curX))
	binary.LittleEndian.PutUint16(b[2:], uint16(tc.curY))
	binary.LittleEndian.PutUint32(b[4:], g.state.Random.Get())
	binary.LittleEndian.PutUint16(b[8:], uint16(code))
	binary.LittleEndian.PutUint16(b[0xa:], uint16(bx))
	binary.LittleEndian.PutUint16(b[0xc:], uint16(cx))
	binary.LittleEndian.PutUint16(b[0xe:], uint16(arg))
	g.moxSendMessage(g.netIDOf(int(to)), 0x3e8, b)
}

// tacSend is the send before a local action: only in a lockstep battle and
// when the destination is the other machine.
func (g *Game) tacSend(code, bx, cx, arg int16) {
	tc := g.tc
	if tc.netFlag != 0 && tc.netDest != g.state.PlayerNum {
		g.sendTacticalCombatMessage(tc.netDest, code, bx, cx, arg)
	}
}

// tacWaitIf is the barrier after an action, in a lockstep battle.
func (g *Game) tacWaitIf() {
	if g.tc.netFlag != 0 {
		g.waitUntilNetOpponentFinished(true)
	}
}

// opponentOf is the other side of the battle from p's point of view.
func (g *Game) opponentOf(p int16) int16 {
	if p == g.tc.att {
		return g.tc.def
	}
	return g.tc.att
}

// waitUntilNetOpponentFinished is Wait_Until_Net_Opponent_Finished_
// (0x3fbfe): msg 0x3ec {the other's index} to the other machine, then input
// passes until his msg 0x3ec came (with draw: the screen and "waiting").
func (g *Game) waitUntilNetOpponentFinished(draw bool) {
	s, n := &g.state, &g.net
	other := g.opponentOf(s.PlayerNum)
	g.moxSendMessage(g.netIDOf(int(other)), 0x3ec, binary.LittleEndian.AppendUint16(nil, uint16(other)))
	for n.tac.done == 0 {
		g.netLostOpponent(int(other)) // port-only
		if draw && g.tc.scr != nil {
			e := g.e
			e.MarkTime()
			e.V.SetPageOff()
			g.must2(g.drawMainCombatScreen())
			msg, err := g.kenText(tacWaitText)
			g.must2(err)
			t := e.Text()
			t.SetStyle(3, g.combatGreen())
			g.must2(t.Print(0x14, 8, msg))
			g.must2(e.TogglePages())
			g.must2(e.ReleaseTime(1))
		} else {
			g.netPollWait() // Mox_Update_ (port-only: yields on virtual time)
		}
		if _, err := g.e.GetInput(); err != nil {
			panic(err)
		}
	}
	n.tac.done--
	if n.tacHook != nil {
		n.tacHook()
	}
}

// tacticalCombatReceiveMessage is Tactical_Combat_Receive_Message_
// (0x3fcd8), ids 0x3e8..0x3ed (jump table 0x3fcc0).
func (g *Game) tacticalCombatReceiveMessage(from, id uint16, buf []byte) {
	n := &g.net
	b := append(append([]byte(nil), buf...), make([]byte, tacMsgSize)...)
	w := int16(binary.LittleEndian.Uint16(b))
	switch id {
	case 0x3e8: // an action
		copy(n.tac.msg[:], b[:tacMsgSize])
		n.tac.choice = int16(binary.LittleEndian.Uint16(n.tac.msg[8:]))
	case 0x3ea: // the other side's AUTO cancelled (Toggle_Auto_Combat_Flags_)
		tc := g.tc
		if tc == nil {
			return
		}
		if w == 0 {
			tc.autoAtt = 0
		} else {
			tc.autoDef = 0 // 0x192b10 + 2·w
		}
		if int16(g.tsp(tc.cur).u8(tsOwner)) == tc.att {
			if w == 0 {
				tc.autoCombat = 0
			}
		} else if w == 1 {
			tc.autoCombat = 0
		}
	case 0x3eb: // the moving side's missile impact flag
		if tc := g.tc; tc != nil {
			tc.savedMslImpact = tc.mslImpact
			tc.mslImpact = w
		}
	case 0x3ec: // the other machine is at the barrier
		n.tac.done++
	case 0x3ed: // the other's initiative setting
		if w == 1 {
			if g.tc != nil {
				g.tc.initOrder = 1
			}
			g.combatInit = 1
		}
	}
}

// tacticalNetSetup is Tactical_Combat_'s lockstep flag (0x482d6): both sides
// humans in a network game.
func (g *Game) tacticalNetSetup() {
	s, tc := &g.state, g.tc
	tc.netFlag = 0
	if g.gameType != 2 && g.gameType != 3 {
		return
	}
	if tc.att < s.NumPlayers && g.isHuman(int(tc.att)) && tc.def < s.NumPlayers && g.isHuman(int(tc.def)) {
		tc.netFlag = 1
	}
}

// tacticalNetStart is Tactical_Combat_'s start of the lockstep (0x48a96):
// _new_message cleared, msg 0x3ed {_combat_init} to the other, a barrier.
func (g *Game) tacticalNetStart() {
	tc, n := g.tc, &g.net
	if tc.netFlag == 0 {
		return
	}
	n.tac.msg = [tacMsgSize]byte{}
	other := g.opponentOf(g.state.PlayerNum)
	g.moxSendMessage(g.netIDOf(int(other)), 0x3ed, binary.LittleEndian.AppendUint16(nil, uint16(tc.initOrder)))
	g.waitUntilNetOpponentFinished(false)
}

// nextShipNet is Next_Ship_'s lockstep part (0x4576d): the destination is
// the other side of the new ship; its owner sends his missile impact flag
// (msg 0x3eb); then a barrier.
func (g *Game) nextShipNet() {
	s, tc := &g.state, g.tc
	if tc.netFlag == 0 {
		return
	}
	owner := int16(g.tsp(tc.cur).u8(tsOwner))
	tc.netDest = g.opponentOf(owner)
	if owner == s.PlayerNum {
		tc.mslImpact = tc.savedMslWarning
		g.moxSendMessage(g.netIDOf(int(tc.netDest)), 0x3eb, binary.LittleEndian.AppendUint16(nil, uint16(tc.mslImpact)))
	}
	g.waitUntilNetOpponentFinished(true)
}

// tacRemote reports whether the current ship belongs to the other machine.
func (g *Game) tacRemote() bool {
	tc := g.tc
	return tc.netFlag != 0 && int16(g.tsp(tc.cur).u8(tsOwner)) != g.state.PlayerNum
}

// netTacticalReplay is Do_Combat_Turn_'s other side (0x43457): input passes
// until his action comes (Auto may be pressed meanwhile), then the action
// from the packet's seed, then a barrier. It returns Next_Ship_'s value.
func (g *Game) netTacticalReplay(p int16) int16 {
	tc, n, s := g.tc, &g.net, &g.state
	e := g.e
	for n.tac.choice == 0 {
		g.netLostOpponent(int(p)) // port-only
		e.MarkTime()
		in, err := e.GetInput()
		g.must2(err)
		if tc.scr != nil && in == tc.scr.autoField {
			g.toggleAutoCombatFlags()
		}
		if tc.scr != nil {
			e.V.SetPageOff()
			g.must2(g.drawMainCombatScreen())
			g.must2(e.TogglePages())
		} else {
			g.netPollWait() // port-only: off screen nothing else yields
		}
		if _, err := e.GetInput(); err != nil {
			panic(err)
		}
		g.must2(e.ReleaseTime(1))
	}
	m := n.tac.msg
	w := func(o int) int16 { return int16(binary.LittleEndian.Uint16(m[o:])) }
	if tc.scr != nil {
		g.must2(g.snapCenterCombatScreen(w(0)+0x10, w(2)+9))
	}
	code, bx, cx, arg := w(8), w(0xa), w(0xc), w(0xe)
	s.Random.Set(binary.LittleEndian.Uint32(m[4:]))
	n.tac.msg = [tacMsgSize]byte{}
	done := int16(0)
	next := func() { done = g.nextShip() }
	markNext := func() {
		tc.order[tc.curPtr] = -1
		next()
	}
	fields := func() {
		if tc.scr != nil {
			g.must2(g.addTacticalCombatFields())
		}
		g.assignCombatGrids(tc.cur)
	}
	switch code {
	case tacSelfDestruct:
		C := g.tsp(tc.cur)
		C.set16(tsDriveCur, 0)
		drive := C.u8(tsDrive)
		C.set8(tsDrive, 1)
		dead := g.isCombatShipDead(tc.cur)
		g.destroyShip(tc.cur, int16(C.u8(tsX))*0x14, int16(C.u8(tsY))*0x14, dead)
		g.tsp(tc.cur).set8(tsDrive, drive)
		tc.order[tc.curPtr] = -1
		n.tac.choice = tacSelfDestruct // set again before Next_Ship_ (0x439c4)
		next()
	case tacAuto:
		g.doAutoShipTurn()
		markNext()
	case tacDone:
		markNext()
	case tacMove:
		C := g.tsp(tc.cur)
		owner := C.u8(tsOwner)
		g.moveCost(tc.cur, bx, cx)
		if C.i16(0x3d) < 0 {
			C.set16(0x3d, 0)
		}
		t1 := g.usedTeleporter(tc.cur)
		cost := g.moveShip(tc.cur, bx, cx)
		if t1 == g.usedTeleporter(tc.cur) {
			C.add16(0x3d, -cost)
		}
		if tc.scr != nil {
			g.must2(g.refreshCombatScreenFull())
		}
		if g.shipDoneCheck() || g.tsp(tc.cur).u8(tsOwner) != owner {
			markNext()
		}
	case tacFire:
		if cx != 0 {
			M := g.msl(bx)
			g.fireShip(tc.cur, bx, M.i16(tmX), M.i16(tmY), cx)
		} else {
			T := g.tsp(bx)
			g.fireShip(tc.cur, bx, int16(T.u8(tsX))*0x14, int16(T.u8(tsY))*0x14, cx)
		}
		fields()
		C := g.tsp(tc.cur)
		for k := 0; k < 8; k++ {
			if C.u8(tw(k, twOn)) == 0 {
				C.set8(tw(k, twOn), 1)
			}
		}
	case tacMslWarning:
		tc.mslWarning = (tc.mslWarning + 1) % 3
		fields()
	case tacAutoField:
		if tc.scr != nil {
			e.ClearFields()
			f, err := e.AddHiddenField(0, 0, 0x27f, 0x1df, g.cmbtKey(cmbtKeyNone), 0)
			g.must2(err)
			tc.scr.autoField = f
		}
		if int16(g.tsp(tc.cur).u8(tsOwner)) == tc.att {
			tc.autoAtt = 1 - tc.autoAtt
			tc.autoCombat = tc.autoAtt
		} else {
			tc.autoDef = 1 - tc.autoDef
			tc.autoCombat = tc.autoDef
		}
	case tacWait:
		next()
	case tacWeapon:
		C := g.tsp(tc.cur)
		k := int(bx)
		v := int8(C.u8(tw(k, twOn))) - 1
		if v < -1 {
			v = 1
		}
		C.set8(tw(k, twOn), uint8(v))
		if v != 1 && C.i16(tw(k, twWeapon)) == 0x27 {
			C.set8(0x42, 0xff)
		}
	case tacFlee:
		g.tsp(tc.cur).set8(0x4b, 1)
		markNext()
	case tacTurnToFace:
		g.turnToFace(tc.cur, bx, cx)
	case tacStasisOff:
		g.removeStasisBeam(bx)
	case tacSetCur:
		if tc.initOrder == 0 {
			g.setCurShipTo(bx)
		}
	case tacBoard:
		C := g.tsp(tc.cur)
		cloak := func(drawOn bool) {
			switch C.u8(tsCloak) {
			case 1:
				if drawOn && tc.superFast == 0 {
					g.drawCloak(tc.cur, 2)
				}
				C.set8(tsCloak, 2)
			case 4:
				if drawOn && tc.superFast == 0 {
					g.drawCloak(tc.cur, 5)
				}
				C.set8(tsCloak, 6)
			}
		}
		nn := cx
		if arg == 1 {
			// The original compares the kind (1) with _combat_on_screen.
			cloak(arg == tc.onScreen)
			C[0x48] |= 2
			C.set8(tsMarines, C.u8(tsMarines)-uint8(nn))
			g.resolveRaid(tc.cur, bx, &nn)
			C.set8(tsMarines, C.u8(tsMarines)+uint8(nn))
		} else {
			cloak(tc.onScreen == 1)
			C[0x48] |= 2
			C.set8(tsMarines, C.u8(tsMarines)-uint8(nn))
			T := g.tsp(bx)
			g.resolveCapture(tc.cur, bx, nn, int16(T.u8(tsX))*0x14, int16(T.u8(tsY))*0x14)
		}
	case tacAllWait:
		done = g.allWait()
	case tacEndOfTurn:
		if tc.initOrder != 0 {
			tc.sideEnd[g.tsp(tc.cur).u8(tsOwner)] = 1
			next()
		} else {
			done = 1
		}
	}
	n.tac.choice = 0
	g.waitUntilNetOpponentFinished(true)
	_ = p
	return done
}

// tacticalNetTurnEnd is Do_Combat_Turn_'s end in a lockstep battle
// (0x44e65): on the side that did not move, the missile impact flag before
// the other's msgs 0x3eb comes back; then a barrier.
func (g *Game) tacticalNetTurnEnd(p int16) {
	tc := g.tc
	if tc.netFlag == 0 {
		return
	}
	if p != g.state.PlayerNum {
		tc.mslImpact = tc.savedMslImpact
	}
	g.waitUntilNetOpponentFinished(true)
}

// tacticalNetAutoSend is the computer branch's send (0x4334c): the moving
// machine tells the other to run the same Do_Auto_Ship_Turn_.
func (g *Game) tacticalNetAutoSend() { g.tacSend(tacAuto, 0, 0, 0) }
