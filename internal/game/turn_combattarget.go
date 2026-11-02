package game

import "github.com/openorion2/openorion2/internal/save"

// Target choice of the battle search (COMBFIND.C): AI, monsters, humans.

// shipStrengthInfo is Get_Ship_Strength_Info_ (0xe76b2) for att's ships at
// star: vs[o] is o's ships against att, mine[o] att's ships against o
// (inflated for big fleets), shield[l] att's bombs against shield level l.
func (g *Game) shipStrengthInfo(star, att int16) (vs, mine [15]uint32, shield [4]uint32) {
	s := &g.state
	f := g.cf
	size := uint32(0)
	for k := int(s.NumShips) - 1; k >= 0; k-- {
		sh := &s.Ships[k]
		if sh.Location != star || sh.Status != save.ShipAtStar {
			continue
		}
		if int16(sh.Owner) != att {
			vs[sh.Owner] += uint32(uint16(g.shipStrengthVsShips(k, int(att))))
			continue
		}
		size += 1 << sh.Design.Size
		for p := s.NumPlayers - 1; p >= 0; p-- {
			if p == att {
				continue
			}
			if b := &f.has[star][p]; b[6] != 0 || b[7] != 0 {
				mine[p] += uint32(uint16(g.shipStrengthVsShips(k, int(p))))
			}
		}
		for o := 8; o < 15; o++ {
			if f.nonplayer[star]>>uint(o-8)&1 != 0 && f.needs[star]>>uint(o)&1 == 0 {
				mine[o] += uint32(uint16(g.shipStrengthVsShips(k, o)))
			}
		}
		pl := g.planetaryShieldStrengths(k)
		for l := 3; l >= 0; l-- {
			shield[l] += uint32(pl[l])
		}
	}
	if size > 150 {
		for i := 14; i >= 0; i-- {
			v := mine[i]
			mine[i] = v / 150 * size
			if v > mine[i] {
				mine[i] = v
			}
		}
	}
	return
}

// bestColonyTarget is Get_Best_Colony_Target_ (0xe78a7): the colony target
// worth most against the odds; one of player war's colonies wins if any is
// worth attacking.
func (g *Game) bestColonyTarget(vs, mine *[15]uint32, shield *[4]uint32, star, att int16, war int32) (best uint32, col int16) {
	s := &g.state
	f := g.cf
	col = -1
	bestWar, warCol := uint32(0), int32(-1)
	A := &s.Players[att]
	for i := g.ct.nColT; i > 0; {
		i--
		c := g.ct.colT[i]
		C := &s.Colonies[c]
		lvl := colonyPlanetaryShield(C)
		owner := int(C.Owner)
		odds := mine[owner]
		against := vs[owner]
		bombs := shield[lvl]
		space := uint32(uint16(g.colonySpaceStrengthVsPlayer(int(c), int(att))))
		against += space
		ground := uint32(g.colonyGroundStrengthVsPlayer(int(c), int(att)))
		telepath := A.Picks[save.PickTelepathic] != 0 && s.Players[owner].Picks[save.PickTelepathic] == 0
		mine5 := f.has[star][att][5]
		attack := false
		switch {
		case odds == 0 && bombs == 0 && mine5 == 0 && !telepath && C.IsOutpost == 0:
		case against >= odds, bombs <= ground:
		case f.has[star][owner][7] >= 2 && bombs == 0 && mine5 == 0 && !telepath && space == 0:
		case C.PopCount == 1 && int16(C.Colonists[0]&0xf) == att && f.has[star][att][2] == 0 && A.Personality != 1:
		default:
			attack = true
		}
		if !attack {
			continue
		}
		worth := uint32(g.enemyColonyWorthToPlayer(int(c), int(att)))
		if 3*against < odds {
			odds, against = 3, 1
		} else if odds == 0 {
			odds, against = 1, 1
		}
		if 3*ground < bombs {
			bombs, ground = 3, 1
		} else if bombs == 0 {
			bombs, ground = 1, 1
		}
		if against >= 0x10000 {
			odds >>= 8
			against >>= 8
		}
		if ground >= 0x10000 {
			ground >>= 8
			bombs >>= 8
		}
		v := worth * odds / against * bombs / ground
		if v > best {
			best, col = v, c
		}
		if int32(owner) == war && v > bestWar {
			bestWar, warCol = v, int32(c)
		}
	}
	if war != -1 && warCol != -1 {
		best, col = bestWar, int16(warCol)
	}
	return best, col
}

// getAITarget is Get_AI_Target_ (0xe7dca): the colony or ship owner an AI
// attacks at star; a pending sneak attack there is used up and preferred.
func (g *Game) getAITarget(star, att int16, shipT, colT *int16, sneak *bool) {
	s := &g.state
	*sneak = false
	*colT, *shipT = -1, -1
	if star == s.NumStars {
		*shipT = 8
		return
	}
	if g.ct.nColT <= 0 && g.ct.nShipT <= 0 {
		return
	}
	P := &s.Players[att]
	war := int32(-1)
	if P.AIWarTarget != -1 && int16(s.Planets[P.AIWarTarget].Star) == star {
		P.AIWarTarget = -1
		war = int32(int8(P.Diplo7C9[1]))
		P.Diplo816 = 0
		P.Diplo7C9[1] = 0xff
	}
	bestShip, shipOwner := uint32(0), int16(-1)
	vs, mine, shield := g.shipStrengthInfo(star, att)
	bestCol, col := g.bestColonyTarget(&vs, &mine, &shield, star, att, war)
	if war == -1 || col == -1 {
		for i := g.ct.nShipT; i > 0; {
			i--
			o := g.ct.shipT[i]
			odds, against := mine[o], vs[o]
			if against > odds {
				continue
			}
			attack := false
			switch {
			case against < odds:
				attack = true
			case s.Stars[star].ColonyOwners>>uint(att)&1 != 0 && s.Stars[star].BlockadedBy[att]>>uint(o)&1 != 0,
				int32(o) == war, P.Personality <= 1:
				attack = true
			case P.Personality <= 3:
				attack = uint8(s.Random.Random(2))-1 != 0
			}
			if !attack {
				continue
			}
			if 3*against < odds {
				odds, against = 3, 1
			} else if odds == 0 {
				against = 1
			}
			v := uint32(0xffffffff)
			if int32(o) != war {
				v = odds << 4 / against
			}
			if v > bestShip {
				bestShip, shipOwner = v, o
			}
		}
	}
	if bestCol < bestShip {
		*shipT = shipOwner
		*sneak = int32(shipOwner) == war
		return
	}
	if bestCol == 0 {
		return
	}
	*colT = col
	*sneak = int32(s.Colonies[col].Owner) == war
}

// getNPCTarget is Get_NPC_Target_ (0xe8029): monsters (not the Eel) pick a
// colony at random, sparing lucky races if they can, else a ship owner.
func (g *Game) getNPCTarget(star, att int16, shipT, colT *int16) {
	s := &g.state
	*colT, *shipT = -1, -1
	if att == 13 {
		return
	}
	if star == s.NumStars {
		*shipT = 8
		return
	}
	pick := func(i int16, n *uint32, to *int16) {
		*n++
		if s.Random.Random(*n) == 1 {
			*to = i
		}
	}
	if g.ct.nColT > 0 {
		lucky, other := int16(-1), int16(-1)
		var nl, no uint32
		for i := g.ct.nColT - 1; i >= 0; i-- {
			c := g.ct.colT[i]
			if s.Players[s.Colonies[c].Owner].Picks[save.PickLucky] != 0 {
				pick(c, &nl, &lucky)
			} else {
				pick(c, &no, &other)
			}
		}
		if other != -1 {
			*colT = other
		} else {
			*colT = lucky
		}
		return
	}
	if g.ct.nShipT <= 0 {
		return
	}
	lucky, other := int16(-1), int16(-1)
	var nl, no uint32
	for i := g.ct.nShipT - 1; i >= 0; i-- {
		o := g.ct.shipT[i]
		if o >= 8 || s.Players[o].Picks[save.PickLucky] != 0 {
			pick(o, &nl, &lucky)
		} else {
			pick(o, &no, &other)
		}
	}
	if other != -1 {
		*shipT = other
	} else {
		*shipT = lucky
	}
}

// getLocalHumanTarget is Get_Local_Human_Target_ (0xe7cdb): the human at
// the controls picks a target in Defense_Colony_Selection_Popup_ and may
// have to confirm breaking a treaty.
func (g *Game) getLocalHumanTarget(star, p int16, shipT, colT *int16) {
	s := &g.state
	*colT, *shipT = -1, -1
	if star == s.NumStars {
		*shipT = 8
		return
	}
	if g.ct.nColT <= 0 && g.ct.nShipT <= 0 {
		return
	}
	s.SomethingHappened[p] = 1
	s.Stars[star].Visited |= 1 << uint(p)
	s.PlayerNum = p
	g.updateShipIconsInBetweenTurns()
	for {
		*colT, *shipT = -1, -1
		def := int16(-1)
		r := g.defenseColonySelectionPopup(star, p)
		if r != -1 {
			if r >= 15 {
				*colT = r - 15
				def = int16(s.Colonies[*colT].Owner)
			} else {
				def, *shipT = r, r
			}
		}
		if def == -1 || g.humanConfirmsAttack(p, def) {
			return
		}
	}
}

// humanConfirmsAttack is Human_Confirms_Attack_ (0xe7b79): attacking a
// player p has a treaty or tribute with needs a confirmation, once per pair.
func (g *Game) humanConfirmsAttack(p, q int16) bool {
	s := &g.state
	if q >= 8 || g.asked[p][q] != 0 {
		return true
	}
	P := &s.Players[p]
	n := 0
	if t := P.TreatyStatus[q]; t >= 1 && t <= 3 {
		n++
	}
	if P.TradeTreaty[q] != 0 {
		n++
	}
	if P.ResearchTreaty[q] != 0 {
		n++
	}
	if s.Players[q].Tribute[p] != 0 {
		n++
	}
	if n == 0 {
		return true
	}
	ok := g.attackConfirmationBox(p, q)
	if ok {
		g.asked[p][q] = 1
	}
	return ok
}

// defenseColonySelectionPopup stands for Defense_Colony_Selection_Popup_
// (0xc9676), a screen: -1 none, a ship owner, or 15 + colony.
func (g *Game) defenseColonySelectionPopup(star, p int16) int16 {
	if g.combatUI != nil {
		return g.combatUI.selectTarget(star, p)
	}
	if g.e != nil {
		r, err := g.defenseSelectionPopup(int(star), int(p))
		if err != nil {
			panic(err)
		}
		return r
	}
	g.Log("Defense_Colony_Selection_Popup_ (star %d, player %d) without a screen (step harness): no attack", star, p)
	return -1
}

// attackConfirmationBox stands for Human_Confirms_Attack_'s Do_Confirmation_Box_.
func (g *Game) attackConfirmationBox(p, q int16) bool {
	if g.combatUI != nil {
		return g.combatUI.confirmAttack(p, q)
	}
	if g.e != nil {
		msg, err := g.attackConfirmationText(p, q)
		if err == nil {
			var ok bool
			if ok, err = g.confirmationBox(msg); err == nil {
				return ok
			}
		}
		panic(err)
	}
	g.Log("Human_Confirms_Attack_'s confirmation box without a screen (step harness): declined")
	return false
}

// combatUI answers the combat screens the turn cannot decide on its own.
type combatUI interface {
	selectTarget(star, p int16) int16
	confirmAttack(p, q int16) bool
	dragonBribe(p, star int16) int16
}

// port-only: a combatUI that wants the tactical battle on its screen.
type combatUIScreen interface{ tacticalOnScreen() bool }

func combatOnScreen(u combatUI) bool {
	s, ok := u.(combatUIScreen)
	return ok && s.tacticalOnScreen()
}

// doDragonBribe is Do_Dragon_Bribe_ (0xe9cd3): the Dragon (its ship with
// the lowest index at star) moves on to star 'to' and p pays 500 BC.
func (g *Game) doDragonBribe(p, star, to int16) {
	s := &g.state
	ids := []int16{-1, -1}
	for k := int(s.NumShips) - 1; k >= 0; k-- {
		if sh := &s.Ships[k]; sh.Owner == 12 && sh.Location == star {
			ids[0] = int16(k)
		}
	}
	info := g.shipsTryToMoveTo(ids, int(to))
	g.makeShipsMoveTo(ids, int(to), &info)
	s.Players[p].BC -= 500
	g.cf.nonplayer[star] &^= 0x10
}
