package game

import "github.com/openorion2/openorion2/internal/save"

// How much an AI fleet wants to strike an enemy colony (AIMOVE.C) and the
// colony strengths it weighs (AIPOWER.C), for the fleet AI and for
// diplomacy's opportunity attacks and threats. Spec: docs/turn/overview.md.

// aiIntercept is the 10-byte record Find_Best_Attack_From_Star_ and
// Compute_Attack_Worthiness_ fill for Intercept_Invaders_ and the opportunity search.
type aiIntercept struct {
	player int16 // +0 the only owner considered
	needed int16 // +2 the ratio a target must beat
	planet int16 // +4 best target found
	worth  int16 // +6
	ratio  int16 // +8 the last target's ratio
}

// aiMayAttackPlayer is AI_May_Attack_Player_ (0xd7669): at war (treaty
// 4..6) and none of the three hold-off flags set.
func (g *Game) aiMayAttackPlayer(p, q int) bool {
	if int16(p) == int16(q) {
		return false
	}
	P := &g.state.Players[p]
	if P.Diplo80E[q] != 0 || P.TradeTreaty[q] != 0 || P.ResearchTreaty[q] != 0 {
		return false
	}
	t := P.TreatyStatus[q]
	return t >= 4 && t <= 6
}

// playerIsHostileToPlayer is Player_Is_Hostile_To_Player_ (0xd8de1): 1 at
// war or for aggressive personalities, 0 for personalities 4 and 5.
func (g *Game) playerIsHostileToPlayer(p, q int) uint8 {
	P := &g.state.Players[p]
	r := uint8(2)
	if t := P.TreatyStatus[q]; t >= 4 && t <= 6 {
		r = 1
	}
	if r == 2 {
		switch d := P.Personality; {
		case d <= 3:
			r = 1
		case d <= 5:
			r = 0
		}
	}
	if r == 2 && P.Objective == 1 {
		r = 1
	}
	if r == 2 {
		r = 1
	}
	return r
}

// colonyPlanetaryShield is Colony_Planetary_Shield_ (0xe5ae7): 3, 2, 1 for
// buildings 0x17, 0x18, 0x1c, else 0.
func colonyPlanetaryShield(c *save.Colony) int {
	switch {
	case c.Buildings[0x17] != 0:
		return 3
	case c.Buildings[0x18] != 0:
		return 2
	case c.Buildings[0x1c] != 0:
		return 1
	}
	return 0
}

// enemyColonyWorthToPlayer is Enemy_Colony_Worth_To_Player_ (0xd8d11): the
// planet's base value to p and to its owner, weighted by their treaty.
func (g *Game) enemyColonyWorthToPlayer(ci, p int) int32 {
	s := &g.state
	c := &s.Colonies[ci]
	q := int(c.Owner)
	var mine, theirs int32
	switch s.Players[p].TreatyStatus[q] {
	case 4:
		mine, theirs = 2, 4
	case 6:
		mine, theirs = 0, 6
	default:
		mine, theirs = 1, 5
	}
	if s.Players[p].Picks[save.PickTelepathic] != 0 && s.Players[q].Picks[save.PickTelepathic] == 0 {
		mine++
		theirs--
	}
	a := g.ai
	if a == nil {
		a = g.aiKept // the statics outlive Deallocate_AI_Data_
	}
	return (int32(a.basePlanetValues[q][c.Planet])*theirs + int32(a.basePlanetValues[p][c.Planet])*mine) / 6
}

// nEnemyStarsReachable is N_Enemy_Stars_Reachable_ (0xd8e52): q's colony
// stars within p's fuel range of st, not cut off by a black hole.
func (g *Game) nEnemyStarsReachable(p, q, st int) int32 {
	s := &g.state
	n := int32(0)
	for j := int(s.NumStars) - 1; j >= 0; j-- {
		if !maskBit(s.Stars[j].ColonyOwners, q) {
			continue
		}
		if g.parsecsBetweenStars(j, st) > int32(s.Players[p].FuelRange) {
			continue
		}
		if !blackHoleBit(&s.Stars[j], int(int16(st))) {
			n++
		}
	}
	return n
}

// Monster beams and missiles (jump tables 0x568cf and 0x56bde), owners 8..14.
var monsterBeam = [7]int{0xb, 4, 1, 0x2a, 9, 1, 0x2b}
var monsterMissile = [7]int{0x11, 0x14, 0x11, 0x11, 0x28, 0x11, 0x11}

// bestBeamWeapon is Best_Beam_Weapon_ (0x568eb).
func (g *Game) bestBeamWeapon(p int) int {
	switch {
	case int16(p) < 8:
		return g.bestWeapon(p, 3, 0)
	case uint16(p-8) <= 6:
		return monsterBeam[p-8]
	}
	return 3
}

// bestMissileWeaponNT is Best_Missile_Weapon_NT_ (0x56bfa): missiles only.
func (g *Game) bestMissileWeaponNT(p int) int {
	switch {
	case int16(p) < 8:
		return g.bestWeapon(p, 0xe, 1)
	case uint16(p-8) <= 6:
		return monsterMissile[p-8]
	}
	return 0xe
}

func (g *Game) strategic() bool { return g.settings[0xd8] != 0 }

// colonySpaceStrengthVsPlayer is Colony_Space_Strength_Vs_Player_ (0x5f804):
// the strength of the colony's best space station.
func (g *Game) colonySpaceStrengthVsPlayer(ci, vs int) uint32 {
	c := &g.state.Colonies[ci]
	typ := int16(-1)
	switch {
	case c.Buildings[0x28] != 0:
		typ = 1
	case c.Buildings[8] != 0:
		typ = 2
	case c.Buildings[0x29] != 0:
		typ = 3
	}
	return g.sateliteStrengthVsShips(typ, int(c.Owner), vs)
}

// clampWeaponHit is the "below zero is zero" of the AIPOWER.C formulas.
func clampWeaponHit(v int32) int32 { return max(v, 0) }

// sateliteStrengthVsShips is Satelite_Strength_Vs_Ships_ (0x5eb72): a
// station of hull typ+5 armed like the owner's best ship would be.
func (g *Game) sateliteStrengthVsShips(typ int16, owner, vs int) uint32 {
	if typ == -1 {
		return 0
	}
	att := int16(g.exeI16(rowAddr(computerTable, computerStride, g.bestComputer(owner), computerBonus)))
	if int16(owner) < 8 {
		att += int16(g.state.Players[owner].Picks[save.PickShipAttack])
	}
	hit := att + 100 - g.baseGenericDcv(int16(vs))
	if hit < 10 {
		hit = 10
	}
	if hit > 100 {
		hit = 100
	}
	beam := g.bestBeamWeapon(owner)
	missile := g.bestMissileWeaponNT(owner)
	hull := uint32(int32(typ)+5) * 0x24
	var nBeam, nHeavy, nMissile int16
	if g.strategic() {
		nBeam = g.exeI16(0x18002e + hull)
		nMissile = g.exeI16(0x180030 + hull)
	} else {
		space := int32(g.exeI16(hullSpaceTable + hull))
		half := space / 2
		rest := int32(int16(space - half))
		nBeam = int16(half / 4 / g.weaponSpace(owner, beam, 1, 0xf, 0, 0))
		nHeavy = int16(half / 2 / g.weaponSpace(owner, beam, 1, 0xf, 0, 2))
		nMissile = int16(rest / g.weaponSpace(owner, missile, 1, 0xf, 0x14, 0))
	}
	sh := int32(g.bestShipShieldValue(int16(vs)))
	dmg := int32(g.wpnI16(beam, weaponStrength))
	v := uint32(int32(nBeam) * clampWeaponHit(dmg-sh) * int32(hit) / 100)
	v += uint32(int32(nHeavy) * clampWeaponHit(dmg*3/2-sh) * int32(hit) / 100)
	v += uint32(clampWeaponHit(int32(g.wpnI16(missile, weaponStrength))-sh) * int32(nMissile))
	if !g.strategic() {
		hits := g.exeI16(0x180026 + hull)
		var m int32
		if hits <= 100 {
			m = (100-int32(hits))/10 + int32(hits)
		} else {
			m = (int32(hits)-100)/4 + 100
		}
		r := uint32(int32(int16(m))) * v / 100
		if r > 1 {
			r = uint32(int32(strengthHitsModifier(hits))) * v / 100
		} else {
			r = 1
		}
		v = r * 0x4b / 100
	}
	return min(v, 0xfa00)
}

// colonyGroundStrengthVsPlayer is Colony_Ground_Strength_Vs_Player_
// (0x5f747): missile and beam bases, ground batteries and fighter garrisons,
// summed in 16 bits.
func (g *Game) colonyGroundStrengthVsPlayer(ci, vs int) uint16 {
	c := &g.state.Colonies[ci]
	o := int(c.Owner)
	var v uint32
	if c.Buildings[0x1a] != 0 {
		v = g.missileBaseStrength(o, vs)
	}
	if c.Buildings[0x1b] != 0 {
		v += g.phasorBaseStrength(o, vs)
	}
	if c.Buildings[0x2a] != 0 {
		if g.settings[0xd8] == 1 {
			v += 200
		} else {
			v += 500
		}
	}
	if c.Buildings[0x2f] != 0 {
		v += uint32(g.fighterGarrisonStrength(o, vs))
	}
	if uint16(v) > 0xfa00 {
		return 0xfa00
	}
	return uint16(v)
}

// missileBaseStrength is Missile_Base_Strength_ (0x5f524).
func (g *Game) missileBaseStrength(o, vs int) uint32 {
	if g.settings[0xd8] == 1 {
		return 0x5a
	}
	m := g.bestMissileWeaponNT(o)
	n := 300 / g.weaponSpace(o, m, 1, 0x10, 5, 0)
	d := clampWeaponHit(int32(g.wpnI16(m, weaponStrength)) - int32(g.bestShipShieldValue(int16(vs))))
	return min(uint32(d*int32(int16(n))*2), 0xfa00)
}

// phasorBaseStrength is Phasor_Base_Strength_ (0x5f5a3).
func (g *Game) phasorBaseStrength(o, vs int) uint32 {
	if g.settings[0xd8] == 1 {
		return 0x78
	}
	b := g.bestBeamWeapon(o)
	n := 225 / g.weaponSpace(o, b, 1, 1, 1, 2)
	sh := int32(g.bestShipShieldValue(int16(vs)))
	d := clampWeaponHit(int32(g.wpnI16(b, weaponStrength))*3/2 - sh)
	d *= int32(int16(n))
	if g.wpnU8(b, 0x12)&8 != 0 {
		d *= 3
	}
	return min(uint32(d), 0xfa00)
}

// fighterGarrisonStrength is Fighter_Garrison_Strength_ (0x5f64c): the sum
// is kept to 16 bits before it is halved.
func (g *Game) fighterGarrisonStrength(o, vs int) uint16 {
	if g.settings[0xd8] == 1 {
		return 0x78
	}
	pd := g.bestPointDefenseWeapon(int16(o))
	bomb := g.bestBombWeapon(o)
	sh := int32(g.bestShipShieldValue(int16(vs)))
	P := &g.state.Players[o]
	var heavy, interceptor, light int32 = 0, 0, 10
	if P.AppStatus[0x53] == techKnown {
		heavy, light = 4, 0
	} else if P.AppStatus[0x1f] == techKnown {
		interceptor, light = 6, 0
	}
	fighters := int16((2*heavy + light) * 4)
	bombers := int16((heavy*3/2 + interceptor) * 4)
	a := clampWeaponHit(int32(g.wpnI16(pd, weaponStrength))-sh) * int32(fighters)
	b := clampWeaponHit(int32(g.wpnI16(bomb, weaponStrength))-sh) * int32(bombers)
	v := uint16(uint32(uint16(a))+uint32(b)) >> 1
	return min(v, 0xfa00)
}

// computeAttackWorthiness is Compute_Attack_Worthiness_ (0xd8ed2): the worth
// of attacking colony ci with the ships (arriving in eta turns), or 0. kind
// becomes 1 (invade), 2 (bomb or blockade) or 6 (invade with crushing
// force); an unwinnable target marks its star seen and flee for p.
func (g *Game) computeAttackWorthiness(ma *moveAI, ci int, ships []int16, eta int16, kind *uint8, seen *uint8, ip *aiIntercept, mode int32) uint32 {
	s := &g.state
	a := g.ai
	p := ma.p
	c := &s.Colonies[ci]
	q := int(c.Owner)
	st := int(s.Planets[c.Planet].Star)
	nebula := s.Stars[st].InNebula != 0
	hostile := uint32(g.playerIsHostileToPlayer(p, q))
	if int32(c.PopCount) < mode && c.IsOutpost == 0 && s.Players[p].Personality != 1 {
		for i := int(c.PopCount) - 1; i >= 0; i-- {
			if int(c.Colonists[i]&0xf) == p {
				return 0
			}
		}
	}
	shield := colonyPlanetaryShield(c)
	var str, bomb, size uint32
	for k := 0; ships[k] != -1; k++ {
		sh := int(ships[k])
		blk := aiShipStrengths
		if nebula {
			blk = aiShipStrengthsInNebula
		}
		str += uint32(a.u16(a.at(blk) + sh*16 + q*2))
		bomb += uint32(a.u16(a.at(aiShipBombStrengths) + sh*8 + shield*2))
		size += 1 << s.Ships[sh].Design.Size
	}
	if ip == nil && size > 150 {
		if v := str / 150 * size; v >= str {
			str = v
		}
	}
	enemy := a.u32(a.at(aiPossibleShipStrengths) + st*0xa00 + q*0x140 + p*0x28 + int(eta)*4)

	// Ships the owner can still finish or buy before we arrive.
	var bought uint32
	n := int32(0)
	bc := s.Players[q].BC
	prod := int32(c.Production)
	for k := 0; k < 7 && bc+prod > 0 && int32(eta) > n; k++ {
		qv := c.Queue[k]
		if qv > -100 {
			continue
		}
		si := -(int(qv) + 100)
		if s.Ships[si].Design.Type != 0 {
			continue
		}
		if cost := int32(s.Ships[si].Design.Cost); cost <= prod {
			bought += uint32(uint16(g.shipStrengthVsShips(si, p)))
			n++
			prod -= cost
		} else if buy := g.colonyCostToBuyGivenProduct(c, qv, prod); buy <= bc {
			bought += uint32(uint16(g.shipStrengthVsShips(si, p)))
			bc -= buy
			n++
			prod = 0
		}
	}
	enemy += bought

	if bomb+str == 0 || enemy*4 > str*3 {
		*seen = 1
		for e := int(eta); e < 10; e++ {
			ma.flee[st][e] = 1
		}
		return 0
	}
	worth := uint32(g.enemyColonyWorthToPlayer(ci, p))
	space := g.colonySpaceStrengthVsPlayer(ci, p)
	ground := uint32(g.colonyGroundStrengthVsPlayer(ci, p))
	var r uint32
	if def := uint32(uint16(space)) + enemy; def < str+hostile && ground < bomb+hostile {
		*kind = 1
		switch {
		case def*3 < str:
			str, def = 3, 1
		case str == 0:
			str, def = 1, 1
		}
		switch {
		case ground*3 < bomb:
			bomb, ground = 3, 1
		case bomb == 0:
			bomb, ground = 1, 1
		}
		if ip == nil && str == 3 && def == 1 {
			*kind = 6
		}
		if def >= 0x10000 {
			str >>= 8
			def >>= 8
		}
		if ground >= 0x10000 {
			bomb >>= 8
			ground >>= 8
		}
		r = worth * str / def * bomb / ground
		r = isqrt(uint32(g.nEnemyStarsReachable(p, q, st)*100)) * r / 10
		if ip != nil {
			ip.ratio = int16(str * 100 / def * bomb / ground)
			if ip.needed >= ip.ratio {
				r = 0
			} else {
				ip.planet = c.Planet
				ip.worth = int16(worth)
			}
		}
	} else {
		*kind = 2
		switch {
		case enemy*3 < str:
			str, enemy = 3, 1
		case str == 0:
			str, enemy = 1, 1
		}
		r = worth * str / (enemy * 2)
		S := &s.Stars[st]
		if maskBit(S.Blockaded, q) {
			b := uint32(S.BlockadedBy[q])
			if b&^(uint32(1)<<uint(p)) != 0 || b>>uint(p)&1 == 0 {
				r = 0
			}
		}
	}
	if eta <= 1 {
		r += r / 6
	}
	return min(r, 0xffff)
}

// findBestAttackFromStar is Find_Best_Attack_From_Star_ (0xd94b3): p's fleet
// at star picks an enemy colony in range by weighted worth and flies there
// with AI mission = the attack kind. With ip, it only rates targets.
func (g *Game) findBestAttackFromStar(ma *moveAI, star int, ip *aiIntercept, mission, mode int32) {
	s := &g.state
	a := g.ai
	p := ma.p
	var local aiIntercept
	var lp *aiIntercept
	if ip != nil {
		local = *ip
		lp = &local
	}
	var seen [72]uint8
	var w [250]uint16
	var kind [250]uint8
	found := false
	best := uint16(0)
	ships := g.gatherPlayerShipsAtStar(p, star)
	n := len(ships) - 1
	for ci := int(s.NumColonies) - 1; ci >= 0; ci-- {
		c := &s.Colonies[ci]
		q := int(c.Owner)
		if q == -1 {
			continue
		}
		if ip == nil && !g.aiMayAttackPlayer(p, q) || ip != nil && int(ip.player) != q {
			continue
		}
		st := int(s.Planets[c.Planet].Star)
		if seen[st] != 0 || !a.inRange(st, p) {
			continue
		}
		res := g.shipsTryToMoveTo(ships, st)
		if !res.canMove || uint8(res.turns) >= 10 {
			continue
		}
		w[ci] = uint16(g.computeAttackWorthiness(ma, ci, ships, int16(uint8(res.turns)), &kind[ci], &seen[st], lp, mode))
		if w[ci] == 0 {
			continue
		}
		switch kind[ci] {
		case 1:
			if mission != 1 {
				w[ci] = 0
				break
			}
			if best < w[ci] {
				best = w[ci]
				if lp != nil && local.planet != -1 {
					ip.planet, ip.worth, ip.ratio = local.planet, local.worth, local.ratio
				}
			}
			found = true
		case 2, 6:
			if int32(kind[ci]) != mission {
				w[ci] = 0
				break
			}
			best = max(best, w[ci])
			found = true
		}
	}
	if !found || ip != nil {
		return
	}
	for i := 249; i >= 0; i-- {
		if int32(w[i])*3 < int32(best)*2 {
			w[i] = 0
		}
	}
	k := int(getWeightedChoiceShort(&s.Random, w[:]))
	m := kind[k]
	dest := int(s.Planets[s.Colonies[k].Planet].Star)
	a.setU16(a.at(aiAllCombatShipsIndex)+p*0x92+star*2, 0xffff)
	res := g.shipsTryToMoveTo(ships, dest)
	g.makeShipsMoveTo(ships, dest, &res)
	for i := n - 1; i >= 0; i-- {
		s.Ships[ships[i]].AIMission = m
	}
}

// launchAttacks is Launch_Attacks_ (0xd97ee).
func (g *Game) launchAttacks(ma *moveAI, mission, mode int32) {
	s := &g.state
	a := g.ai
	for st := int(s.NumStars) - 1; st >= 0; st-- {
		if int16(a.u16(a.at(aiAllCombatShipsIndex)+ma.p*0x92+st*2)) != -1 {
			g.findBestAttackFromStar(ma, st, nil, mission, mode)
		}
	}
}

// aiCombatShipsIndex is _i_all_combat_ships[p][star].
func (a *aiArena) aiCombatShipsIndex(p, star int) int16 {
	return int16(a.u16(a.at(aiAllCombatShipsIndex) + p*0x92 + star*2))
}

// possibleShipStrength is _possible_ship_strengths[star][owner][vs][turns].
func (a *aiArena) possibleShipStrength(star, owner, vs, turns int) uint32 {
	return a.u32(a.at(aiPossibleShipStrengths) + star*0xa00 + owner*0x140 + vs*0x28 + turns*4)
}

// playerThreatensPlayer is Player_Threatens_Player_ (0xdbcc8): the first
// star (from the last) where b's possible strength against a reaches a
// quarter of a's own strength against b and an attack by a would pay; it
// returns the star, the strength ratio in percent and the planet, or -1s.
func (g *Game) playerThreatensPlayer(a, b int16) (star, pct, planet int32) {
	s := &g.state
	star, planet = -1, -1
	if s.Players[a].Picks[29] == 0 && g.hyperspaceFlux() {
		return
	}
	ai := g.ai
	own := ai.u32(ai.at(aiPlayerInfo)+int(a)*0x3a+0x1a+int(b)*4) >> 2
	for st := int32(s.NumStars) - 1; st >= 0; st-- {
		if own > ai.possibleShipStrength(int(st), int(a), int(b), 0)/10 {
			continue
		}
		var p, w int16
		var r int32
		g.findOpportunityAttackAux(a, b, 150, int16(st), &p, &w, &r)
		if p == -1 {
			continue
		}
		return st, r, int32(p)
	}
	return
}

// findOpportunityAttackAux is Find_Opportunity_Attack_Aux_ (0xdbb9f): the
// best colony of b that a's ships at star could take with a strength ratio
// above thresh percent; planet -1 for none. Difficulty 0 and 1 never look.
func (g *Game) findOpportunityAttackAux(a, b, thresh, star int16, planet, worth *int16, pct *int32) {
	*planet = -1
	if g.ai.aiCombatShipsIndex(int(a), int(star)) == -1 || g.settings.Difficulty() <= 1 {
		return
	}
	// _flee_stars points at a scratch stack array here; the marks are dropped.
	ma := &moveAI{p: int(a)}
	ip := aiIntercept{player: b, needed: thresh, planet: -1}
	g.findBestAttackFromStar(ma, int(star), &ip, 1, 2)
	if ip.planet != -1 {
		*planet, *worth, *pct = ip.planet, ip.worth, int32(ip.ratio)
	}
}

// findOpportunityAttack is Find_Opportunity_Attack_ (0xdbc5c): the attack
// a's ships at its staging star could make on b.
func (g *Game) findOpportunityAttack(a, b, thresh int16, planet, worth *int16, pct *int32) {
	*planet = -1
	if g.ai == nil || (g.state.Players[a].Picks[29] == 0 && g.hyperspaceFlux()) {
		return
	}
	g.findOpportunityAttackAux(a, b, thresh, int16(g.state.Players[a].StagingStar), planet, worth, pct)
}
