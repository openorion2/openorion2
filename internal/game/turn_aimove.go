package game

import (
	"github.com/openorion2/openorion2/internal/clib"
	"github.com/openorion2/openorion2/internal/save"
)

// The AI's fleet orders (AIMOVE.C): Move_All_AI_ and Move_AI_ with the
// smaller passes. Spec: docs/turn/overview.md, Move_All_AI_.

// moveAI is Move_AI_'s state in globals 0x1aa23c..0x1aa24e: the player, the
// planets worth colonising, stars to flee by turn and monsters by star.
type moveAI struct {
	p         int
	planets   []colonizable
	flee      [72][10]uint8
	monsterAt [72]uint8
}

// moveAllAI is Move_All_AI_ (0xdbb29): AI players with an FTL drive move
// unless a hyperspace flux holds them; then the monsters.
func (g *Game) moveAllAI() {
	s := &g.state
	// What the stack holds before the turn's first rating is not modelled: 0.
	g.interceptStack = [72]int8{}
	for p := int(s.NumPlayers) - 1; p >= 0; p-- {
		P := &s.Players[p]
		if P.Objective == objectiveHuman || P.FTLSpeed == 0 {
			continue
		}
		if P.Picks[save.PickTransDimensional] != 0 || !g.hyperspaceFlux() {
			g.moveAIFor(p)
		}
	}
	g.moveNPCs()
}

// moveAIFor is Move_AI_ (0xdb47e).
func (g *Game) moveAIFor(p int) {
	s := &g.state
	a := g.ai
	ma := &moveAI{p: p}
	ma.planets = g.getColonizablePlanets(p)
	clib.Qsort(ma.planets, 6, func(x, y *colonizable) int { return int(int32(x.value - y.value)) })
	g.markFleeStars(ma)
	for i := int(s.NumShips) - 1; i >= 0; i-- {
		if sh := &s.Ships[i]; sh.Owner >= 8 && sh.Status == 0 {
			ma.monsterAt[sh.Location] = uint8(sh.Owner)
		}
	}
	ma.planets = g.deleteAlreadyAssignedPlanets(p, ma.planets)
	g.launchSneakAttack(ma)
	g.launchAttacks(ma, 6, 2)
	g.interceptInvaders(ma)
	g.launchAttacks(ma, 1, 2)
	g.assignNewColonyShipDestinations(ma)
	g.launchAttacks(ma, 2, 2)
	g.launchAttacks(ma, 6, 0)
	g.launchAttacks(ma, 1, 0)
	g.launchAttacks(ma, 2, 0)
	g.moveTransports(ma)
	g.moveToStagePoints(ma)
	info := a.at(aiPlayerInfo) + p*0x3a
	if a.u16(info+0x18) == a.u16(info+0xa) || g.settings.Difficulty() == 0 {
		g.moveToUnexploredStars(ma)
	}
}

// fleeByte reads the flee table flat; a 0 eta indexes the byte before a
// star's row, as in the original (before the table it reads 0).
func (ma *moveAI) fleeByte(i int) uint8 {
	if i < 0 {
		return 0
	}
	return ma.flee[i/10][i%10]
}

// markFleeStars is Mark_Flee_Stars_ (0xdb3e5): every colony star of a
// player p flees from (player +0x80e) is to be avoided on all turns.
func (g *Game) markFleeStars(ma *moveAI) {
	s := &g.state
	P := &s.Players[ma.p]
	for q := int(s.NumPlayers) - 1; q >= 0; q-- {
		if P.Diplo80E[q] == 0 {
			continue
		}
		for st := int(s.NumStars) - 1; st >= 0; st-- {
			if ma.flee[st][0] != 0 || !maskBit(s.Stars[st].ColonyOwners, q) {
				continue
			}
			for e := 9; e >= 0; e-- {
				ma.flee[st][e] = 1
			}
		}
	}
}

// gatherPlayerShipsAtStar is Gather_Player_Ships_At_Star_ (0xd93f8): p's
// combat ships at star from _i_all_combat_ships on (interceptors only at the
// staging star), ending in -1.
func (g *Game) gatherPlayerShipsAtStar(p, star int) []int16 {
	s := &g.state
	a := g.ai
	idx := int(int16(a.u16(a.at(aiAllCombatShipsIndex) + p*0x92 + star*2)))
	ship := func(k int) int16 { return int16(a.u16(a.at(aiAllShips) + k*2)) }
	out := []int16{ship(idx)}
	for k := idx + 1; int16(k) != a.nAllShips; k++ {
		i := ship(k)
		sh := &s.Ships[i]
		if int16(sh.Owner) != int16(p) || int16(star) != sh.Location {
			break
		}
		if int16(star) != int16(s.Players[p].StagingStar) && sh.AIMission == 5 {
			continue
		}
		out = append(out, i)
	}
	return append(out, -1)
}

// launchSneakAttack is Launch_Sneak_Attack_ (0xdb257): the fleet at the
// staging star strikes the planet chosen by diplomacy (+0x7c7) of player
// +0x7ca; war is declared once the strike would arrive next turn.
func (g *Game) launchSneakAttack(ma *moveAI) {
	s := &g.state
	a := g.ai
	p := ma.p
	P := &s.Players[p]
	if P.AIWarTarget != -1 && g.playerWarLevel(p) != 0 {
		P.AIWarTarget = -1
	}
	if P.AIWarTarget == -1 {
		return
	}
	pl := &s.Planets[P.AIWarTarget]
	q := int(int8(P.Diplo7C9[1]))
	if pl.Colony != -1 && int(s.Colonies[pl.Colony].Owner) == q {
		stage := int(P.StagingStar)
		at := a.at(aiAllCombatShipsIndex) + p*0x92 + stage*2
		if int16(a.u16(at)) != -1 {
			dest := int(pl.Star)
			ships := g.gatherPlayerShipsAtStar(p, stage)
			res := g.shipsTryToMoveTo(ships, dest)
			if res.canMove {
				for i := len(ships) - 2; i >= 0; i-- {
					s.Ships[ships[i]].AIMission = 6
				}
				g.makeShipsMoveTo(ships, dest, &res)
				a.setU16(at, 0xffff)
			}
		}
	}
	st := int(s.Planets[P.AIWarTarget].Star)
	if a.u32(a.at(aiEnrouteShipStrengths)+st*0xa00+p*0x140+q*0x28+4) != 0 {
		g.declareWar(int16(p), int16(q))
	}
}

// unloadTransports is Unload_Transports_ (0xed6b7): each transport adds 4
// marines until the colony is full.
func (g *Game) unloadTransports(ci int, ships []int16) int {
	s := &g.state
	c := &s.Colonies[ci]
	lim := g.colonyInfantryLimit(c)
	k := 0
	for int32(c.Marines) < lim && k < len(ships) {
		g.killShip(int(ships[k]))
		k++
		c.Marines += 4
	}
	g.updatePlayerStats(int(c.Owner))
	return k
}

// moveTransports is Move_Transports_ (0xda99c): transports reinforce
// threatened colonies, follow the invasion fleets (AI missions 1 and 6) to
// arrive with them, or wait at the staging star or a safe colony.
func (g *Game) moveTransports(ma *moveAI) {
	s := &g.state
	a := g.ai
	p := ma.p
	P := &s.Players[p]
	var inv, att [72]uint8
	var fleetAt [72]int8
	for i := range fleetAt {
		fleetAt[i] = -1
	}
	var T []int16
	diff := g.settings.Difficulty()
	for i := int(s.NumShips) - 1; i >= 0; i-- {
		sh := &s.Ships[i]
		if int16(sh.Owner) != int16(p) || int8(sh.Status) > 2 {
			continue
		}
		switch sh.Design.Type {
		case 0:
			var arr *[72]uint8
			switch sh.AIMission {
			case 1:
				arr = &inv
			case 6:
				arr = &att
			default:
				continue
			}
			if sh.Location < 0x48 {
				arr[sh.Location] = 1
				fleetAt[sh.Location] = int8(sh.Location)
				continue
			}
			d := shipDestinationStar(sh)
			if arr[d] == 0 || int32(sh.ETA)+1 < int32(arr[d]) {
				arr[d] = sh.ETA + 1
			}
		case 2:
			if sh.Location < 0x48 && diff >= 1 && maskBit(s.Stars[sh.Location].ColonyOwners, p) &&
				(diff >= 2 || P.Objective == 1) {
				S := &s.Stars[sh.Location]
				for k := 4; k >= 0; k-- {
					pl := S.Planets[k]
					if pl == -1 || s.Planets[pl].Colony == -1 {
						continue
					}
					ci := int(s.Planets[pl].Colony)
					c := &s.Colonies[ci]
					if int16(c.Owner) != int16(p) || c.IsOutpost != 0 {
						continue
					}
					info := a.colonyInfo(ci)
					if info[5] == 0 && int32(info[6])*2 <= int32(c.Marines) {
						continue
					}
					if g.unloadTransports(ci, []int16{int16(i)}) == 1 {
						break
					}
				}
			}
			if sh.Status != 5 {
				T = append(T, int16(i))
			}
		}
	}
	for n := len(T) - 1; n >= 0; n-- {
		id := T[n]
		one := []int16{id, -1}
		loc := int(s.Ships[id].Location)
		if loc >= 0x48 {
			continue
		}
		if tgt := int(fleetAt[loc]); tgt != -1 {
			var via, turns int16
			g.moveShipsWithPossibleIntermediate(ma, one, tgt, &via, &turns)
			if via != -1 {
				if int32(att[tgt])-1 > int32(turns) {
					if int32(inv[tgt])-1 == int32(turns) {
						res := g.shipsTryToMoveTo(one, int(via))
						g.makeShipsMoveTo(one, int(via), &res)
					}
				} else {
					res := g.shipsTryToMoveTo(one, int(via))
					g.makeShipsMoveTo(one, int(via), &res)
				}
				continue
			}
		}
		g.transportFollowFleets(ma, one, loc, &inv, &att)
	}
}

// transportFollowFleets is Move_Transports_'s search for a transport with no
// fleet at its star: the soonest attack it can join in time, else the
// staging star, else a safe colony when its own star is to be fled.
func (g *Game) transportFollowFleets(ma *moveAI, one []int16, loc int, inv, att *[72]uint8) {
	s := &g.state
	a := g.ai
	p := ma.p
	bestVia, bestT, isInv := int16(-1), int16(0x7fff), false
	for st := int(s.NumStars) - 1; st >= 0; st-- {
		if inv[st] == 0 && att[st] == 0 {
			continue
		}
		if inv[st] < 11 && inv[loc] >= 2 && ma.fleeByte(st*10+int(inv[st])-1) != 0 {
			continue
		}
		if att[st] < 11 && att[loc] >= 2 && ma.fleeByte(st*10+int(att[st])-1) != 0 {
			continue
		}
		var via, turns int16
		g.moveShipsWithPossibleIntermediate(ma, one, st, &via, &turns)
		if via == -1 {
			continue
		}
		t := int32(turns)
		cl, bad := false, false
		val := turns
		ai, ii := int32(att[st])-1, int32(inv[st])-1
		switch {
		case t < ai, t < ii:
			val, cl = int16(inv[st])-1, true
		case t == ii || att[st] != 0:
			bad = turns >= 10 || ma.flee[st][turns] != 0
		default:
			bad = true
		}
		if !bad && val < bestT {
			bestT, isInv, bestVia = val, cl, via
		}
	}
	if bestVia != -1 {
		if !isInv {
			res := g.shipsTryToMoveTo(one, int(bestVia))
			g.makeShipsMoveTo(one, int(bestVia), &res)
		}
		return
	}
	// Without a staging star (-1) the original reads before its arrays.
	stage := int(s.Players[p].StagingStar)
	if stage >= 0 && ma.flee[stage][9] == 0 && !maskBit(s.Stars[stage].Blockaded, p) && loc != stage {
		if res := g.shipsTryToMoveTo(one, stage); res.canMove {
			g.makeShipsMoveTo(one, stage, &res)
			return
		}
	}
	if ma.flee[loc][1] == 0 {
		return
	}
	best, bestTurns := -1, int32(0x7fff)
	for st := int(s.NumStars) - 1; st >= 0; st-- {
		if ma.flee[st][9] != 0 || !maskBit(s.Stars[st].ColonyOwners, p) {
			continue
		}
		res := g.shipsTryToMoveTo(one, st)
		if !res.canMove {
			continue
		}
		t := uint8(res.turns)
		if a.starInfo(st)[6*p+6] != 0 {
			t -= 0x18
		}
		if int32(t) < bestTurns {
			bestTurns, best = int32(t), st
		}
	}
	if best != -1 {
		res := g.shipsTryToMoveTo(one, best)
		g.makeShipsMoveTo(one, best, &res)
	}
}

// moveToStagePoints is Move_To_Stage_Points_ (0xda485): idle warships go to
// p's stage points, each to the one weakest for its weight; the staging
// star's weight grows by the total, times war level + 1.
func (g *Game) moveToStagePoints(ma *moveAI) {
	s := &g.state
	a := g.ai
	p := ma.p
	P := &s.Players[p]
	warp := s.evActive(eventWarpBeast) && P.Picks[save.PickLucky] == 0
	type stagePoint struct {
		star uint8
		str  uint32
	}
	var sp [72]stagePoint
	nSP := 0
	var spIdx [72]int8
	for i := range spIdx {
		spIdx[i] = -1
	}
	weight := func(st int) int { return a.at(aiStarStageWeights) + st*0x20 + p*4 }
	var sum uint32
	for st := int(s.NumStars) - 1; st >= 0; st-- {
		if ma.flee[st][9] != 0 || !maskBit(s.Stars[st].AIStagePoint, p) {
			continue
		}
		spIdx[st] = int8(nSP)
		sp[nSP] = stagePoint{uint8(st), 0}
		nSP++
		sum += a.u32(weight(st))
	}
	w := weight(int(P.StagingStar))
	a.setU32(w, a.u32(w)+sum*uint32(int32(g.playerWarLevel(p))+1))
	var S []int16
	for i := int(s.NumShips) - 1; i >= 0; i-- {
		sh := &s.Ships[i]
		if int16(sh.Owner) != int16(p) || int8(sh.Status) > 2 || sh.Design.Type != 0 {
			continue
		}
		switch sh.AIMission {
		case 1, 2, 4, 5, 6, 7:
			continue
		}
		if s.canOrderShip(i) {
			S = append(S, int16(i))
		} else if d := shipDestinationStar(sh); spIdx[d] != -1 {
			sp[spIdx[d]].str += uint32(int32(int16(g.shipRoughStrengthAtStar(ma, i, int(d)))))
		}
	}
	if nSP == 0 {
		return
	}
	if nSP == 1 {
		tgt := int(sp[0].star)
		for i := len(S) - 1; i >= 0; i-- {
			id := S[i]
			sh := &s.Ships[id]
			nav := testBit(sh.Design.Specials[:], 0xb)
			one := []int16{id, -1}
			var via, turns int16
			if sh.Location < 0x48 {
				g.playerNTurnsFromStarToStarWithIntermediate(p, int(sh.Location), tgt, &via, nav)
			} else {
				g.moveShipsWithPossibleIntermediate(ma, one, tgt, &via, &turns)
			}
			if via == -1 {
				continue
			}
			sh.AIMission = 3
			res := g.shipsTryToMoveTo(one, int(via))
			if uint8(res.turns) > 1 && warp {
				continue
			}
			g.makeShipsMoveTo(one, int(via), &res)
		}
		return
	}
	shuffleSint(&s.Random, S)
	for len(S) > 0 && nSP > 0 {
		bestK, bestStr, bestW := -1, ^uint32(0), uint32(1)
		bestShip, bestVia := -1, int16(-1)
		bestT := int32(0xff)
		if warp {
			bestT = 2
		}
		for k := nSP - 1; k >= 0; k-- {
			str, wv := sp[k].str, a.u32(weight(int(sp[k].star)))
			better := bestK == -1
			if !better {
				switch c := compareRatios(str, wv, bestStr, bestW); {
				case c < 0:
					better = c == -1
				case c == 0:
					better = bestW < wv
				}
			}
			if better {
				bestK, bestStr, bestW = k, str, wv
			}
		}
		tgt := int(sp[bestK].star)
		for i := len(S) - 1; i >= 0; i-- {
			sh := &s.Ships[S[i]]
			nav := testBit(sh.Design.Specials[:], 0xb)
			switch {
			case int(sh.Location) == tgt:
				bestT, bestVia, bestShip = 0, int16(tgt), i
			case sh.Location < 0x48:
				var via int16
				if t := int32(g.playerNTurnsFromStarToStarWithIntermediate(p, int(sh.Location), tgt, &via, nav)); t < bestT {
					bestShip, bestT, bestVia = i, t, via
				}
			default:
				var via, turns int16
				g.moveShipsWithPossibleIntermediate(ma, []int16{S[i], -1}, tgt, &via, &turns)
				if int32(turns) < bestT {
					bestShip, bestT, bestVia = i, int32(turns), via
				}
			}
		}
		if bestShip == -1 {
			nSP--
			spIdx[bestK] = spIdx[nSP]
			continue
		}
		id := S[bestShip]
		s.Ships[id].AIMission = 3
		one := []int16{id, -1}
		res := g.shipsTryToMoveTo(one, int(bestVia))
		g.makeShipsMoveTo(one, int(bestVia), &res)
		sp[bestK].str += uint32(int32(int16(g.shipRoughStrengthAtStar(ma, int(id), tgt))))
		n := len(S) - 1
		S[bestShip] = S[n]
		S = S[:n]
	}
}

// eventWarpBeast is the event Event_Check_Warp_Beast_ (0x234e5) tests.
const eventWarpBeast = 26

// moveToUnexploredStars is Move_To_Unexplored_Stars_ (0xdafd4): idle
// warships scout unvisited stars in p's range that no ship of p is at or
// bound for and no enemy will reach first; AI mission 7.
func (g *Game) moveToUnexploredStars(ma *moveAI) {
	s := &g.state
	a := g.ai
	p := ma.p
	var occupied [72]bool
	var S []int16
	for i := int(s.NumShips) - 1; i >= 0; i-- {
		sh := &s.Ships[i]
		if int16(sh.Owner) != int16(p) {
			continue
		}
		switch sh.Status {
		case 0:
			if (sh.AIMission == 0 || sh.AIMission == 3) && sh.Design.Type == 0 {
				S = append(S, int16(i))
			}
			occupied[sh.Location] = true
		case 1, 2:
			occupied[shipDestinationStar(sh)] = true
		}
	}
	diff := g.settings.Difficulty()
	for st := int(s.NumStars) - 1; st >= 0; st-- {
		S0 := &s.Stars[st]
		if !a.extRange(st, p) || maskBit(S0.Visited, p) || occupied[st] || diff == 0 && S0.ColonyOwners != 0 {
			continue
		}
		var perTurn, cum [10]uint32
		g.getEnemyStrengthOnTurn(ma, st, &perTurn, &cum)
		if len(S) == 0 {
			return
		}
		best, bestT := -1, int32(0xff)
		for i := len(S) - 1; i >= 0; i-- {
			sh := &s.Ships[S[i]]
			if !testBit(sh.Design.Specials[:], 0xb) && !a.inRange(st, p) {
				continue
			}
			t := int32(a.u8(a.at(aiNTurnsFromTo) + int(sh.Location)*0x240 + st*8 + p))
			if t < bestT && t < 10 && cum[t] == 0 {
				bestT, best = t, i
			}
		}
		if best == -1 {
			continue
		}
		id := S[best]
		n := len(S) - 1
		S[best] = S[n]
		S = S[:n]
		one := []int16{id, -1}
		res := g.shipsTryToMoveTo(one, st)
		s.Ships[id].AIMission = 7
		g.makeShipsMoveTo(one, st, &res)
	}
}

// Monster ship owners and messages of Move_NPCs_.
const (
	ownerEel       = 13
	msgEelSpawned  = 0x12
	msgMonsterGone = 0x13
)

// moveNPCs is Move_NPCs_ (0xdb8d8): monsters at a star move on to a colony
// star nobody else heads for, or vanish; the Eel (owner 13) waits 30 turns
// at a star, then spawns another Eel while there are fewer than four.
func (g *Game) moveNPCs() {
	s := &g.state
	var eelAt, mark [72]uint8
	nEels := int32(-1)
	for i := int(s.NumShips) - 1; i >= 0; i-- {
		sh := &s.Ships[i]
		if sh.Owner < 8 || sh.Design.FTLSpeed == 0 {
			continue
		}
		if sh.Status == 1 {
			mark[int(sh.Location)-500] = 1
			continue
		}
		if sh.Status != 0 {
			continue
		}
		again := false
		if sh.Owner != ownerEel {
			again = true
		} else {
			loc := sh.Location
			if eelAt[loc] != 0 {
				again = true
			}
			eelAt[loc], mark[loc] = 1, 1
			if sh.Design.BuildDate++; sh.Design.BuildDate == 30 {
				d := g.npcDest(i, &mark)
				sh.Design.BuildDate = 0
				if d != -1 && nEels == -1 {
					nEels = 0
					for k := int(s.NumShips) - 1; k >= 0; k-- {
						o := &s.Ships[k]
						if o.Owner == ownerEel && o.Design.FTLSpeed != 0 && int8(o.Status) < 5 {
							nEels++
						}
					}
				}
				if nEels < 4 {
					var d save.Design
					g.loadEelShipDesign(&d, true)
					S := &s.Stars[loc]
					if n := g.createNonplayerShip(ownerEel, &d, S.X, S.Y); n != -1 {
						var m [msgSize]byte
						m[0], m[2] = msgEelSpawned, byte(loc)
						g.addMsgAll(m)
						s.Ships[n].Design.BuildDate = 0
					}
					nEels++
				}
			}
		}
		if !again {
			continue
		}
		d := g.npcDest(i, &mark)
		if d == -1 {
			var m [msgSize]byte
			m[0], m[2] = msgMonsterGone, byte(sh.Location)
			g.addMsgAll(m)
			g.killShip(i)
			continue
		}
		one := []int16{int16(i), -1}
		res := g.shipsTryToMoveTo(one, d)
		g.makeShipsMoveTo(one, d, &res)
	}
}

// addMsgAll is Add_Msg_All_ (0xef6be): the message to every player.
// Move_NPCs_ sets bytes 0 and 2 only; the rest is stale stack.
func (g *Game) addMsgAll(m [msgSize]byte) {
	for p := int(g.state.NumPlayers) - 1; p >= 0; p-- {
		i := g.addMsg(p, m)
		g.markStaleMsg(i, 1, 2)
		g.markStaleMsg(i, 3, msgSize)
	}
}

// npcCand is one 12-byte candidate of NPC_Dest_.
type npcCand struct {
	star, parsecs  uint8
	lucky, normal  uint8
	luckyStr, nStr uint32
}

// npcDest is NPC_Dest_ (0xdb6d2): a random one of the first seven colony
// stars within 15 parsecs, sorted by owners (Lucky ones last), distance and
// the owners' strength; -1 if none.
func (g *Game) npcDest(ship int, mark *[72]uint8) int {
	s := &g.state
	loc := int(s.Ships[ship].Location)
	var str [8]uint32
	for q := int(s.NumPlayers) - 1; q >= 0; q-- {
		Q := &s.Players[q]
		if Q.Eliminated != 0 {
			continue
		}
		v := int32(isqrt(isqrt(uint32(Q.BC)))) + int32(Q.FleetSizeScore)
		str[q] = uint32(v + int32(Q.FieldsResearched)/4 + int32(Q.ColoniesPop5)*4)
	}
	var cand [72]npcCand
	for ci := int(s.NumColonies) - 1; ci >= 0; ci-- {
		c := &s.Colonies[ci]
		q := int(c.Owner)
		if q == -1 {
			continue
		}
		st := int(s.Planets[c.Planet].Star)
		if st == loc || mark[st] != 0 {
			continue
		}
		r := &cand[st]
		if s.Players[q].Picks[save.PickLucky] != 0 {
			r.lucky = 1
			r.luckyStr = max(r.luckyStr, str[q])
		} else {
			r.normal = 1
			r.nStr = max(r.nStr, str[q])
		}
		r.star = uint8(st)
		r.parsecs = uint8(g.parsecsBetweenStars(loc, st))
	}
	list := cand[:s.NumStars]
	clib.Qsort(list, 12, func(a, b *npcCand) int {
		if d := (int(a.lucky) - int(a.normal)) - (int(b.lucky) - int(b.normal)); d != 0 {
			return d
		}
		if d := int(a.parsecs)/8 - int(b.parsecs)/8; d != 0 {
			return d
		}
		if d := int32(b.nStr - a.nStr); d != 0 {
			return int(d)
		}
		if d := int32(b.luckyStr - a.luckyStr); d != 0 {
			return int(d)
		}
		return int(a.parsecs) - int(b.parsecs)
	})
	best, n := -1, uint32(0)
	for i := 0; i < len(list) && n < 7; i++ {
		if list[i].parsecs == 0 || list[i].parsecs >= 15 {
			continue
		}
		n++
		if s.Random.Random(n) == 1 {
			best = int(list[i].star)
		}
	}
	return best
}
