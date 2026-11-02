package game

import (
	"fmt"

	"github.com/openorion2/openorion2/internal/save"
)

// What the winner of a battle does to the colony (COMBFIND.C, INVASION.C):
// Do_Attacker_Beat_Colony_Stuff_ with bombing, invasion and surrender.
// Spec: docs/turn/overview.md, phase E.

// doAttackerBeatColonyStuff is Do_Attacker_Beat_Colony_Stuff_ (0xe87d2):
// outposts are razed, monsters bomb, a human chooses (a popup), an AI
// mind-controls, takes a surrender, bombs and invades.
func (g *Game) doAttackerBeatColonyStuff(list []int16, ci int16, res *combatResult) {
	s := &g.state
	row := g.curRow()
	C := &s.Colonies[ci]
	att := int16(s.Ships[list[0]].Owner)
	star := int(s.Planets[C.Planet].Star)
	owner := int16(C.Owner)
	switch {
	case C.IsOutpost != 0:
		res.outpost = 1
		g.destroyColony(int(ci), 1)
		row[owner][7]--
	case att >= 8:
		if att >= 10 && att <= 14 {
			g.russEventMonsterUpdate(int(att), C.Planet, -1)
		}
		res.bomb = bombHits{Attacker: int8(att), Colony: uint8(ci)}
		if att == 10 {
			res.bomb.Hits, res.bomb.BioHits = 0x14, 0
		}
		g.bombardment(&res.bomb)
		g.applyBombHits(&res.bomb)
		if s.Players[owner].Objective == save.ObjectiveHuman && g.netShowsDefender(owner) && res.bomb.Hit != 0 {
			if att == 10 && g.e != nil {
				g.must2(g.playAmoebaFlic())
				g.messedUpFleetPalette = true // 0xe8922
			}
			if att == 10 && res.bomb.Emptied == 0 {
				pl := &s.Planets[C.Planet]
				pl.Climate, pl.FoodPerFarmer = 0, 0
				s.colonyEnvironmentalStuff(C)
				g.enforcePopulationLimitsAtColony(int(ci))
				g.colonyCalculation(int(ci))
			}
			g.colonyBombingScreen(&res.bomb)
		}
		if res.bomb.Emptied != 0 {
			g.destroyColony(int(ci), 1)
		}
	case s.Players[att].Objective == save.ObjectiveHuman:
		if g.gameType < 2 || g.gameType > 3 || att == s.PlayerNum {
			g.humanBeatColony(list, ci, att, owner, star, res)
		}
	case g.playerCanMindControlColony(att, ci):
		res.mind = 1
		g.changeColonyOwnership(int(ci), int(att), &res.capture)
		row[att][7]++
		row[owner][7]--
	default:
		g.aiBeatColony(list, ci, att, owner, star, res)
	}
	if att == 8 {
		for k := len(list) - 1; k >= 0; k-- {
			s.restockAntaranShip(int(s.Ships[list[k]].Design.Size))
			g.killShip(int(list[k]))
		}
		*g.nonplayerAt(star) &^= 1
	}
	g.updateShipIconsInBetweenTurns()
}

// humanBeatColony is the human branch of Do_Attacker_Beat_Colony_Stuff_
// (0xe89da..0xe8dae): Colony_Selection_Popup_ and what was chosen, until
// the colony is gone, taken or left.
func (g *Game) humanBeatColony(list []int16, ci, att, owner int16, star int, res *combatResult) {
	s := &g.state
	row := g.curRow()
	C := &s.Colonies[ci]
	if g.e == nil || g.combatUI != nil {
		g.Log("Do_Attacker_Beat_Colony_Stuff_: no screen for the human attacker's Colony_Selection_Popup_: colony left alone")
		return
	}
	est0 := bombHits{Attacker: int8(att), Colony: uint8(ci)}
	g.bombardment(&est0)
	est := int16(int32(est0.Hits) + int32(est0.BioHits))
	conv := uint8(0)
	if g.hasStellarConverter(list) {
		conv = 1
	}
	g.getInvasionInfo(list, int(ci), &res.invasion[0], &res.invasion[1])
	r := g.must16(g.colonySelectionPopup(&res.invasion, s.Random.Get(), ci, est, conv))
	for done := false; !done; {
		switch r {
		case -1:
			done = true
		case 1:
			pl := C.Planet
			pop := int16(C.PopCount)
			res.converted = 1
			g.destroyColony(int(ci), 2)
			g.must2(g.playPlanetaryDestructionFlic())
			row[owner][7]--
			P := &s.Planets[pl]
			P.Type = 1
			if P.Special != 0 {
				P.Special = 0
				s.Stars[P.Star].Special = 0
			}
			done = true
			g.russChangeRelations(-10000, int(att), int(owner), 10, int16(star), pop)
		case 2:
			seed := s.Random.Get()
			res.bomb = bombHits{Attacker: int8(att), Colony: uint8(ci)}
			g.bombardment(&res.bomb)
			g.getInvasionInfo(list, int(ci), &res.invasion[0], &res.invasion[1])
			r = g.must16(g.colonyBombingPopup(&res.bomb, &res.invasion, seed, ci, est, conv))
			g.russChangeRelations(-10000, int(att), int(owner), 10, int16(star), res.bomb.PopKilled)
			if res.bomb.Emptied != 0 {
				g.destroyColony(int(ci), 1)
				done = true
				row[owner][7]--
			} else if res.invasion[0].count[1] == 0 && conv == 0 && !g.playerCanMindControlColony(att, ci) {
				done = true
			}
			est = -1
		case 3:
			g.invade(list, ci, &res.invasion, &res.seed, &res.capture)
			g.groundBattleReplay(res, ci)
			if res.capture.n != 0 {
				g.must2(g.showOffCapturedTech(res.capture.apps[:], res.capture.n, res.capture.planet))
			}
			g.russChangeRelations(-10000, int(att), int(owner), 10, int16(star), 0)
			if int16(C.Owner) == att {
				row[att][7]++
				row[owner][7]--
			}
			done = true
		case 4:
			g.changeColonyOwnership(int(ci), int(att), &res.capture)
			res.mind = 1
			if res.capture.n != 0 {
				g.must2(g.showOffCapturedTech(res.capture.apps[:], res.capture.n, C.Planet))
			}
			g.russChangeRelations(-10000, int(att), int(owner), 10, int16(star), int16(C.PopCount))
			row[att][7]++
			row[owner][7]--
			done = true
		default:
			panic(fmt.Sprintf("Do_Attacker_Beat_Colony_Stuff_: popup result %d loops forever", r))
		}
	}
	if res.bomb.BioPopKilled != 0 {
		g.applyBioWeaponPenalty(att, int16(star), res.bomb.BioPopKilled)
	}
	if g.gameType == 2 || g.gameType == 3 {
		// 0xe8db3: back to the waiting screen (att or owner is at the controls)
		g.must2(g.loadNetNextTurnScreen(false))
		g.fullDrawScreen = true
	}
}

// netShowsDefender is the test before the defender's bombing and invasion
// screens (0xe88d8, 0xe913a, 0xe921f): in a network game's strategic combat
// only the host's own player sees them here (the others get msg 0x16).
func (g *Game) netShowsDefender(owner int16) bool {
	if g.settings[0xd8] == 0 || (g.gameType != 2 && g.gameType != 3) {
		return true
	}
	return g.state.Players[owner].NetID == g.net.i16(netHostID)
}

// must16 panics on a screen error inside the turn's logic.
func (g *Game) must16(v int16, err error) int16 {
	if err != nil {
		panic(err)
	}
	return v
}

// aiBeatColony is the AI branch of Do_Attacker_Beat_Colony_Stuff_: the
// colony may surrender its star, else it is bombed (as far as an invasion
// needs, or flat out) or converted, then invaded.
func (g *Game) aiBeatColony(list []int16, ci, att, owner int16, star int, res *combatResult) {
	s := &g.state
	row := g.curRow()
	C := &s.Colonies[ci]
	converter := g.hasStellarConverter(list)
	destroyed, surrender, ownPop := false, false, false
	for k := int(C.PopCount) - 1; k >= 0; k-- {
		if int16(C.Colonists[k]&0xf) == att {
			ownPop = true
		}
	}
	res.bomb = bombHits{Attacker: int8(att), Colony: uint8(ci)}
	g.bombardment(&res.bomb)
	hits, bio := int32(res.bomb.Hits), int32(res.bomb.BioHits)
	if hits != 0 || bio != 0 || converter {
		if s.Players[owner].Objective != save.ObjectiveHuman && g.settings[0xd4] >= 3 &&
			s.maySurrenderStar(star, int(owner), int(att)) && 2*int32(C.PopCount) <= hits {
			surrender = true
		}
		if surrender {
			s.Stars[star].SurrenderTo[owner] = int8(att) + 8
			g.declarePeace(owner, att)
			return
		}
		bombStep := func() {
			if bio != 0 {
				bio--
				g.resolveBiologicalHit(int(ci), &res.bomb)
			} else if n := g.resolveBombHit(int(ci), &res.bomb, hits); n != 0 {
				hits -= n
			} else {
				hits = 0
			}
		}
		A := &s.Players[att]
		switch {
		case ownPop:
			for !g.aiInvasionWillSucceed(att, list, C) {
				if C.PopCount == 1 && int16(C.Colonists[0]&0xf) == att && A.Personality != 1 &&
					(s.playerOwnsTransports(att) || (A.Picks[save.PickTelepathic] != 0 && s.Players[owner].Picks[save.PickTelepathic] == 0)) {
					break
				}
				if bombStep(); hits+bio == 0 {
					break
				}
			}
		case s.nEnemyStarsInRange(att, owner) >= 2 || s.nEnemyStars(owner) == 1:
			if converter {
				destroyed = true
				res.converted = 1
				g.destroyColony(int(ci), 2)
			} else {
				g.applyBombHits(&res.bomb)
			}
		default:
			for !g.aiInvasionWillSucceed(att, list, C) && C.PopCount != 1 {
				if bombStep(); hits+bio == 0 {
					break
				}
			}
		}
		if res.bomb.BioPopKilled != 0 {
			g.applyBioWeaponPenalty(att, int16(star), res.bomb.BioPopKilled)
		}
		if s.Players[owner].Objective == save.ObjectiveHuman && g.netShowsDefender(owner) && res.bomb.Hit != 0 {
			g.colonyBombingScreen(&res.bomb)
		}
		g.russChangeRelations(-10000, int(att), int(owner), 10, int16(star), res.bomb.PopKilled)
		if res.bomb.Emptied != 0 {
			destroyed = true
			g.destroyColony(int(ci), 1)
		}
	}
	if destroyed || !g.aiPlayerInvades(list, ci) {
		return
	}
	g.invade(list, ci, &res.invasion, &res.seed, &res.capture)
	if s.Players[owner].Objective == 100 && g.netShowsDefender(owner) && g.e != nil && g.combatUI == nil {
		g.groundBattleReplay(res, ci)
	}
	g.russChangeRelations(-10000, int(att), int(owner), 10, int16(star), 0)
	if int16(C.Owner) == att {
		row[att][7]++
		row[owner][7]--
	}
}

// groundBattleReplay is the callers' frame of Colony_Combat_Screen_: the
// replay between a saved and a restored seed.
func (g *Game) groundBattleReplay(res *combatResult, ci int16) {
	s := &g.state
	seed := s.Random.Get()
	g.must2(g.colonyCombatScreen(&res.invasion[0], &res.invasion[1], res.seed, ci))
	s.Random.Set(seed)
}

// hasStellarConverter is Has_Stellar_Converter_ (0xe86a0): one of the ships
// (from the last) at a star carries weapon 0x26.
func (g *Game) hasStellarConverter(list []int16) bool {
	s := &g.state
	for k := len(list) - 1; k >= 0; k-- {
		if s.Ships[list[k]].Status != save.ShipAtStar {
			continue
		}
		if s.shipHasWeapon(int(list[k]), 0x26) {
			return true
		}
	}
	return false
}

// shipHasWeapon is Ship_Has_Weapon_ (0x58270).
func (s *State) shipHasWeapon(k int, w int16) bool {
	for i := range s.Ships[k].Design.Weapons {
		if s.Ships[k].Design.Weapons[i].Weapon == w {
			return true
		}
	}
	return false
}

// nEnemyStarsInRange is N_Enemy_Stars_In_Range_ (0xe86e7): q's colony
// stars within p's normal range.
func (s *State) nEnemyStarsInRange(p, q int16) int32 {
	n := int32(0)
	for st := int(s.NumStars) - 1; st >= 0; st-- {
		if s.Stars[st].ColonyOwners>>uint(q)&1 != 0 && s.starInNormalRangeOfPlayer(st, int(p)) {
			n++
		}
	}
	return n
}

// nEnemyStars is N_Enemy_Stars_ (0xe8739): stars where q has a colony.
func (s *State) nEnemyStars(q int16) int32 {
	n := int32(0)
	for st := int(s.NumStars) - 1; st >= 0; st-- {
		if s.Stars[st].ColonyOwners>>uint(q)&1 != 0 {
			n++
		}
	}
	return n
}

// playerOwnsTransports is Player_Owns_Transports_ (0xe8776): any transport
// of p that is not a free slot.
func (s *State) playerOwnsTransports(p int16) bool {
	for i := int(s.NumShips) - 1; i >= 0; i-- {
		if sh := &s.Ships[i]; int16(sh.Owner) == p && sh.Design.Type == 2 && sh.Status < 5 {
			return true
		}
	}
	return false
}

// playerCanMindControlColony is Player_Can_Mind_Control_Colony_ (0xc622a):
// a telepath with a ship of size 2 or more there takes a colony of a
// non-telepath without a governor or officer with general skill 0.
func (g *Game) playerCanMindControlColony(p, ci int16) bool {
	s := &g.state
	C := &s.Colonies[ci]
	star := int16(s.Planets[C.Planet].Star)
	if s.Players[p].Picks[save.PickTelepathic] == 0 || !s.playerHasShipSizeOrLargerAtStar(p, 2, star) {
		return false
	}
	if g.playerHasLeaderWithGeneralSkillAtStar(int16(C.Owner), 0, star) {
		return false
	}
	return s.Players[C.Owner].Picks[save.PickTelepathic] == 0
}

// playerHasShipSizeOrLargerAtStar is Player_Has_Ship_Size_Or_Larger_At_
// Star_ (0xc60e9), over all 500 ship slots.
func (s *State) playerHasShipSizeOrLargerAtStar(p, size, star int16) bool {
	for i := range s.Ships {
		sh := &s.Ships[i]
		if sh.Location == star && sh.Status == save.ShipAtStar && int16(sh.Owner) == p && int16(sh.Design.Size) >= size {
			return true
		}
	}
	return false
}

// playerHasLeaderWithGeneralSkillAtStar is Player_Has_Leader_With_General_
// Skill_At_Star_ (0xc6052): an assigned leader of p at star (on a ship or
// governing) with the skill.
func (g *Game) playerHasLeaderWithGeneralSkillAtStar(p, skill, star int16) bool {
	s := &g.state
	for o := 0; o < 0x43; o++ {
		L := &s.Leaders[o]
		if int16(L.Owner) != p || L.Status != 1 || L.ETA != 0 {
			continue
		}
		at := (L.Type == 0 && L.Location >= 0 && s.Ships[L.Location].Location == star) || (L.Type == 1 && L.Location == star)
		if at && g.officerHasGeneralSkill(o, skill) {
			return true
		}
	}
	return false
}

// officerHasGeneralSkill is Officer_Has_General_Skill_ (0x9467d).
func (g *Game) officerHasGeneralSkill(o int, skill int16) bool {
	mask := uint32(g.exeU16(uint32(0x17d219+0x12*int(skill)))) | uint32(g.exeU16(uint32(0x17d21b+0x12*int(skill))))<<16
	return g.state.Leaders[o].CommonSkills&mask != 0
}

// applyBioWeaponPenalty is Apply_Bio_Weapon_Penalty_ (0x4b11e): every AI
// but the attacker thinks less of it.
func (g *Game) applyBioWeaponPenalty(att, star, killed int16) {
	s := &g.state
	if att >= 8 {
		return
	}
	for p := int16(0); p < s.NumPlayers; p++ {
		if s.Players[p].Objective == save.ObjectiveHuman || p == att {
			continue
		}
		g.russChangeRelations(int16(-int32(s.Random.Random(10))), int(att), int(p), 6, star, killed)
	}
}

// bombardment is Bombardment_ (0x4267b): the attacker's armed ships at the
// colony's star bomb it, strategically or with the tactical engine.
func (g *Game) bombardment(b *bombHits) {
	s := &g.state
	att := b
	star := int16(s.Planets[s.Colonies[att.Colony].Planet].Star)
	var list []int16
	for i := int16(0); i < 500; i++ {
		sh := &s.Ships[i]
		if sh.Location == star && sh.Design.Type == 0 && sh.Status == save.ShipAtStar && sh.Owner == att.Attacker {
			list = append(list, i)
		}
	}
	if g.settings[0xd8] == 1 {
		r := g.strategicBombardment(list, int16(att.Colony))
		b.Kind = 1
		b.Accident = 0
		b.BioHits = 0
		b.Hits = r
		return
	}
	g.tacticalBombardment(list, b)
}

// strategicBombardment is Strategic_Bombardment_ (0x4257e): three volleys
// of the ships' missiles, beams and bombs at the colony; a hit per 40
// points. The records are not cleared first.
func (g *Game) strategicBombardment(list []int16, colony int16) int16 {
	s := &g.state
	if g.qc == nil {
		g.qc = &qcombat{}
	}
	q := g.qc
	q.colony = colony
	q.att = int16(s.Ships[list[0]].Owner)
	q.nDef = 0
	q.n = 1
	q.nAtt = int16(len(list))
	q.def = int16(s.Colonies[colony].Owner)
	q.flags = 0
	g.qloadShips(list, 0, 0, 0)
	g.qloadShips(nil, 0, 0, 0)
	for v := 0; v < 3; v++ {
		for i := int16(1); i < q.n; i++ {
			g.missileAttack(i, 0)
			g.beamAttack(i, 0)
			g.stratBombAttack(i)
			if q.rec[0].damage > 30000 {
				v = 6
				break
			}
		}
	}
	return q.rec[0].damage / 0x28
}

// aiInvasionWillSucceed is AI_Invasion_Will_Succeed_ (0xcf26e): ten
// simulated ground battles (troops landing next turn included); seven
// wins are needed, and the simulation stops once that is decided.
func (g *Game) aiInvasionWillSucceed(p int16, list []int16, C *save.Colony) bool {
	var A, B groundForce
	g.getInvasionInfo(list, g.colonyIndex(C), &A, &B)
	if n := g.arrivingCur(int(p)); int16(n) > 0 {
		A.count[1] += int16(n) * 4
		A.class = 1
		A.count[1] -= 2
	}
	if A.count[1] <= 0 {
		return false
	}
	a0, b0 := A, B
	wins := int16(0)
	for k := int16(10); k > 0 && wins < 7 && int32(k)+int32(wins) >= 7; {
		k--
		g.resolveGroundCombat(&A, &B)
		if A.class < 4 {
			wins++
		}
		A, B = a0, b0
	}
	return wins >= 7
}

func (g *Game) colonyIndex(C *save.Colony) int {
	for i := range g.state.Colonies {
		if &g.state.Colonies[i] == C {
			return i
		}
	}
	return -1
}

// aiPlayerInvades is AI_Player_Invades_ (0xcf34c): the ships carry marines.
func (g *Game) aiPlayerInvades(list []int16, ci int16) bool {
	var A, B groundForce
	g.getInvasionInfo(list, int(ci), &A, &B)
	return A.count[1] != 0
}

// resolveGroundCombat is Resolve_Ground_Combat_ (0xec601).
func (g *Game) resolveGroundCombat(A, B *groundForce) {
	for A.class < 4 && B.class < 4 {
		g.groundCombatRound(A, B)
	}
}

// getInvasionInfo is Get_Invasion_Info_ (0xec831): four marines per
// transport against the colony's tanks, marines and militia.
func (g *Game) getInvasionInfo(list []int16, ci int, A, B *groundForce) {
	s := &g.state
	c := &s.Colonies[ci]
	*A = groundForce{player: int16(s.Ships[list[0]].Owner), enemy: int16(c.Owner), colony: int16(ci), ships: list}
	for k := len(list) - 1; k >= 0; k-- {
		if s.Ships[list[k]].Design.Type == 2 {
			A.count[1] += 4
		}
	}
	*B = groundForce{player: int16(c.Owner), enemy: A.player, colony: int16(ci), ships: list}
	B.count[0] = c.Armor
	B.count[1] = c.Marines
	B.count[2] = int16(colonyNMilitia(c))
	g.computePlayerGroundCombatBonuses(int(A.player), &A.bonus, false)
	g.computeAttackerOfficerBonus(list, &A.bonus)
	g.computePlayerGroundCombatBonuses(int(A.enemy), &B.bonus, true)
	g.computeColonyOfficerBonus(c, &B.bonus)
	computeGroundCombatInfo(A, &A.bonus, &B.bonus)
	computeGroundCombatInfo(B, &B.bonus, &A.bonus)
}

// computeAttackerOfficerBonus is Compute_Attacker_Officer_Bonus_ (0xec2fd):
// the best ground skill among the officers aboard, from the last ship.
func (g *Game) computeAttackerOfficerBonus(list []int16, b *groundBonus) {
	s := &g.state
	for k := len(list) - 1; k >= 0; k-- {
		o := s.Ships[list[k]].Officer
		if o < 0 || s.Leaders[o].ETA != 0 {
			continue
		}
		v := int32(0)
		switch L := &s.Leaders[o]; {
		case L.CommonSkills&8 != 0:
			v = (int32(s.ownedOfficerLevel(int(o)))*15 + 16) / 2
		case L.CommonSkills&4 != 0:
			v = (int32(s.ownedOfficerLevel(int(o))) + 1) * 5
		}
		if b.officer() < int16(v) {
			b[0x10], b[0x11] = byte(v), byte(v>>8)
			b[0x12] = byte(o)
		}
	}
}

// invade is Invade_ (0xed48b): the ground battle; the winner takes the
// colony, else the defenders' survivors stay and the transports are lost.
func (g *Game) invade(list []int16, ci int16, info *[2]groundForce, seed *uint32, rep *captureReport) {
	s := &g.state
	C := &s.Colonies[ci]
	var A, B groundForce
	g.getInvasionInfo(list, int(ci), &A, &B)
	info[0], info[1] = A, B
	rep.n, rep.invaded = 0, true
	*seed = s.Random.Get()
	g.resolveGroundCombat(&A, &B)
	if A.class < 4 {
		g.changeColonyOwnership(int(ci), int(A.player), rep)
		g.resolveInvasionTroops(A)
	} else {
		C.Armor = B.count[0]
		C.Marines = B.count[1]
		for k := len(list) - 1; k >= 0; k-- {
			if s.Ships[list[k]].Design.Type == 2 {
				g.killShip(int(list[k]))
			}
		}
	}
	g.updateShipIconsInBetweenTurns()
}

// resolveInvasionTroops is Resolve_Invasion_Troops_ (0xece05): the landed
// marines garrison the colony up to its limit; a transport whose four
// marines are still counted survives, the others are spent.
func (g *Game) resolveInvasionTroops(A groundForce) {
	s := &g.state
	c := &s.Colonies[A.colony]
	A.count[1]--
	lim := int32(int16(g.colonyInfantryLimit(c)))
	c.Marines = 1
	for k := len(A.ships) - 1; k >= 0; k-- {
		if s.Ships[A.ships[k]].Design.Type != 2 {
			continue
		}
		if A.count[1] >= 4 {
			A.count[1] -= 4
		} else {
			g.killShip(int(A.ships[k]))
		}
	}
	c.Marines += A.count[1]
	if int32(c.Marines) >= lim {
		c.Marines = int16(lim)
	}
	s.computeStarColonyStuff(int(s.Planets[c.Planet].Star))
}
