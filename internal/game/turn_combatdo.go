package game

import "github.com/openorion2/openorion2/internal/save"

// One battle and its aftermath (COMBFIND.C, INVASION.C): Do_1_Combat_ and
// what it calls. Spec: docs/turn/overview.md, phase E; docs/turn/combat.md.

// combatResult is Do_1_Combat_'s 0x6ce-byte frame record; only the parts
// the turn reads back are kept.
type combatResult struct {
	head      [12]byte       // +0x00 the network caller's block (Client_Combat_Loop_)
	hasHead   bool           // port-only: head and planet were set
	planet    int16          // +0x0d
	result    int8           // +0x0a
	outpost   uint8          // +0x0c an outpost was destroyed
	bomb      bombHits       // +0x0f
	invasion  [2]groundForce // +0x58 attacker, +0x8f defender
	seed      uint32         // +0xc6 the seed the invasion started from
	capture   captureReport  // +0xca, +0x272, +0x274 (Invade_'s +0x6e.. from +0x58)
	converted uint8          // +0x276 destroyed by a stellar converter
	mind      uint8          // +0x6cd taken by mind control
	loknar    []int16        // +0x277 Loknar's techs, count +0x27f
	title     string         // +0x281 Strategic_Combat_'s end of combat box
	text      string         // +0x2e5
}

// getCombatShips is Get_Combat_Ships_ (0xe6a0c): the attacker's ships at the
// star in ascending order, the defender's from the top of the same list.
func (g *Game) getCombatShips(star, att, def int16) (attList, defList []int16) {
	s := &g.state
	for i := int16(0); i < s.NumShips; i++ {
		sh := &s.Ships[i]
		if sh.Location != star || sh.Status != save.ShipAtStar {
			continue
		}
		switch int16(sh.Owner) {
		case att:
			attList = append(attList, i)
		case def:
			defList = append([]int16{i}, defList...)
		}
	}
	return
}

// do1Combat is Do_1_Combat_ (0xe938c): the battle at star, then retreats,
// Orion and Antares, and what the winner does to the colony. res is the
// caller's 0x6ce-byte record, nil for Do_1_Combat_'s own zeroed one.
func (g *Game) do1Combat(star, att, def, planet int16, res *combatResult, sneak bool) int8 {
	s := &g.state
	if res == nil {
		res = &combatResult{}
	}
	colony := int16(-1)
	if planet != -1 {
		colony = s.Planets[planet].Colony
	}
	if def == -1 {
		def = int16(s.Colonies[colony].Owner)
	}
	if att < 8 && def < 8 {
		s.Players[att].TurnsAtWar[def] = 0
	}
	net := g.gameType >= 2 && g.gameType <= 3
	if net {
		if g.netIsHost() {
			g.setCur(int(star))
		}
		if def == s.PlayerNum && g.settings[0xd8] == 0 && g.e != nil {
			// 0xe947c: Message_Box_ with who attacks where (UI).
			g.netAttackedMessage(star, att, planet)
		}
	} else {
		g.setCur(int(star))
	}
	attList, defList := g.getCombatShips(star, att, def)
	res.result = g.russCombat(att, attList, def, defList, star, colony, sneak, res)
	colony2 := int16(-1)
	if planet != -1 {
		colony2 = s.Planets[planet].Colony
	}
	if net && (att == s.PlayerNum || def == s.PlayerNum) && g.e != nil {
		g.must2(g.loadNetNextTurnScreen(true))
		g.fullDrawScreen = true
		g.must2(g.chatBoxInputLoop())
	}
	row := g.curRow()
	if res.result >= 0 {
		if def < 8 {
			if row[def][0] == 0 && colony2 == -1 {
				g.killNoncombatShips(defList)
			} else {
				g.determineRetreatShips(star, def, colony2)
			}
		} else {
			*g.curNonplayer() &^= 1 << uint(def-8)
		}
	}
	if res.result > 0 {
		if def == 9 {
			s.Players[att].CapturedOrion = 1
			g.giveLoknarSpecial(att)
			res.loknar = append([]int16(nil), g.loknarTechs...)
			g.takeOrionFlic(att)
		}
		if star == s.NumStars {
			s.Players[att].CapturedAntares = 1
			g.clearNeeds()
			return res.result
		}
		attList, _ = g.getCombatShips(star, att, def)
		if len(attList) == 0 {
			if att >= 8 {
				*g.curNonplayer() &^= 1 << uint(att-8)
			} else {
				g.resetCombatShipsAtCurrentStar(att)
			}
		}
		if colony2 != -1 && s.Colonies[colony2].Owner != -1 {
			g.doAttackerBeatColonyStuff(attList, colony2, res)
		}
		return res.result
	}
	if att < 8 {
		g.determineRetreatShips(star, att, -1)
	} else {
		*g.curNonplayer() &^= 1 << uint(att-8)
	}
	if att == 9 {
		s.Players[def].CapturedOrion = 1
		g.giveLoknarSpecial(def)
		res.loknar = append([]int16(nil), g.loknarTechs...)
		g.takeOrionFlic(def)
	}
	if res.result < 0 {
		if _, defList = g.getCombatShips(star, att, def); len(defList) == 0 {
			if def >= 8 {
				*g.curNonplayer() &^= 1 << uint(def-8)
			} else {
				g.resetCombatShipsAtCurrentStar(def)
			}
		}
	}
	return res.result
}

// clearNeeds is Do_1_Combat_'s memset of 0x1aa3ac's 0x90 bytes after Antares
// falls; a client's pointer is 0 (or a dead frame): address 0 is cleared.
func (g *Game) clearNeeds() {
	if g.cf != nil {
		for i := 0; i < 0x90/2; i++ {
			g.cf.needs[i] = 0
		}
		return
	}
	g.cc.nullHas = [8][11]byte{} // the 0x38 bytes after it are not modelled
}

// nonplayerAt is 0x1aa3b4[star]; without Search_For_Battles_ (a client) the
// pointer is 0, so address star is meant.
func (g *Game) nonplayerAt(star int) *byte {
	if g.cf != nil {
		return &g.cf.nonplayer[star]
	}
	return &g.cc.nullHas[star/11][star%11]
}

// arrivingCur is the battle star's s16 at +8 of p's row (transports
// arriving next turn).
func (g *Game) arrivingCur(p int) uint16 {
	b := &g.curRow()[p]
	return uint16(b[8]) | uint16(b[9])<<8
}

// takeOrionFlic is Do_1_Combat_'s video for a human who took Orion
// (0xe967d, 0xe9759): single player games or tactical combat; then
// Rescued_Loknar_Popup_ (loknar.go).
func (g *Game) takeOrionFlic(p int16) {
	s := &g.state
	if p >= 8 || s.Players[p].Objective != save.ObjectiveHuman || g.e == nil {
		return
	}
	if g.gameType == 0 || g.gameType == 1 || g.settings[0xd8] == 0 {
		g.must2(g.playTakeOrionFlic())
		g.must2(g.rescuedLoknarPopup())
	}
}

// resetCombatShipsAtCurrentStar is Reset_Combat_Ships_At_Current_Star_Has_
// Player_ (0xe9358): p's ship presence at the battle star, from its types.
func (g *Game) resetCombatShipsAtCurrentStar(p int16) {
	b := &g.curRow()[p]
	b[0] = 0
	b[6] = 0
	for t := 4; t >= 0; t-- {
		if b[t] != 0 {
			b[6] = 1
		}
	}
}

// killNoncombatShips is Kill_Noncombat_Ships_ (0xe6b44): an unarmed
// defender without a colony there loses its ships.
func (g *Game) killNoncombatShips(list []int16) {
	s := &g.state
	if len(list) <= 0 {
		return
	}
	// A human owner sees Message_Box_ here (UI).
	for k := len(list) - 1; k >= 0; k-- {
		g.killShip(int(list[k]))
	}
	g.curRow()[s.Ships[list[0]].Owner][6] = 0
	g.updateShipIconsInBetweenTurns()
}

// determineRetreatShips is Determine_Retreat_Ships_ (0xe6caa): the side's
// ships at star retreat (status 9) unless it holds colonies there; under a
// hyperspace flux they are lost instead.
func (g *Game) determineRetreatShips(star, p, colony int16) {
	s := &g.state
	row := g.curRow()
	if (colony == -1 && row[p][7] != 0) || row[p][7] >= 2 {
		for i := int(s.NumShips) - 1; i >= 0; i-- {
			if sh := &s.Ships[i]; int16(sh.Owner) == p && sh.Location == star && sh.Status == save.ShipAtStar {
				return
			}
		}
		row[p][6] = 0
		return
	}
	if p < 8 && s.Players[p].Picks[save.PickTransDimensional] == 0 && g.hyperspaceFlux() {
		for i := int(s.NumShips) - 1; i >= 0; i-- {
			sh := &s.Ships[i]
			if int16(sh.Owner) != p || sh.Location != star || sh.Status != save.ShipAtStar {
				continue
			}
			if o := int(sh.Officer); o != -1 {
				if s.Leaders[o].ETA > 0 {
					g.moveOfficerToPool(o)
				} else {
					g.killOfficer(o)
				}
			}
			g.killShip(i)
		}
		row[p][6] = 0
		return
	}
	for i := int(s.NumShips) - 1; i >= 0; i-- {
		if sh := &s.Ships[i]; int16(sh.Owner) == p && sh.Location == star && sh.Status == save.ShipAtStar {
			sh.Status = save.ShipRetreating
		}
	}
	row[p][6] = 0
	g.updateShipIconsInBetweenTurns()
}

// aiCanRetreat is AI_Can_Retreat_ (0xe7303): p has a colony other than ci.
func (s *State) aiCanRetreat(p, ci int16) bool {
	for i := s.NumColonies - 1; i >= 0; i-- {
		if int16(s.Colonies[i].Owner) == p && i != ci {
			return true
		}
	}
	return false
}

// russCombat is Russ_Combat_ (0xe7343): the strategic resolver when no
// human fights or settings 0xd8 asks for it, else the tactical screen.
func (g *Game) russCombat(att int16, attList []int16, def int16, defList []int16, star, colony int16, sneak bool, res *combatResult) int8 {
	s := &g.state
	strat := g.settings[0xd8] != 0
	attHuman := att < 8 && s.Players[att].Objective == save.ObjectiveHuman
	defHuman := def < 8 && s.Players[def].Objective == save.ObjectiveHuman
	if len(attList) == 0 || (len(defList) == 0 && colony == -1) {
		if star != s.NumStars {
			return 0
		}
	}
	if att < 8 {
		s.SomethingHappened[att] = 1
	}
	if def < 8 {
		s.SomethingHappened[def] = 1
	}
	// A human defender sees Show_Sneak_Attack_Message_ and Player_Attacked_
	// Popup_ here, and the result in Message_Box_Titled_ below (UI).
	local := g.gameType != 2 && g.gameType != 3
	if defHuman && g.e != nil && g.combatUI == nil && local {
		if sneak {
			// Show_Sneak_Attack_Message_ (0x1afa0) falls into
			// Npc_Diplomacy_Screen_(attacker, 1): the delayed message.
			if err := g.npcDiplomacyScreen(int(att), true); err != nil {
				panic(err)
			}
		}
		if err := g.playerAttackedPopup(att, def, star, colony); err != nil {
			panic(err)
		}
		g.messedUpFleetPalette = true
	}
	if !strat && !attHuman && !defHuman {
		strat = true
	}
	var r int16
	if strat {
		r = g.strategicCombat(attList, defList, star, colony)
		res.title, res.text = g.qc.endTitle, g.qc.endText
		if (local || att == s.PlayerNum) && (defHuman || attHuman) && g.e != nil && g.combatUI == nil {
			// 0xe74f6: Message_Box_Titled_(title, text) is Do_Text_Box_ with no ticks.
			g.updateShipIconsInBetweenTurns()
			if err := g.doTextBox(&g.qc.endTitle, g.qc.endText, 0); err != nil {
				panic(err)
			}
			g.updateShipIconsInBetweenTurns()
			g.messedUpFleetPalette = true
		}
	} else {
		canRetreat := true
		switch {
		case att >= 8 || def >= 8:
			canRetreat = false
		case !attHuman:
			canRetreat = s.aiCanRetreat(att, -1)
		case !defHuman:
			canRetreat = s.aiCanRetreat(def, colony)
		}
		if attHuman && defHuman && !local {
			r = g.russNetTactical(att, def, attList, defList, star, colony, canRetreat)
		} else {
			r = g.tacticalCombat(attList, defList, star, colony, canRetreat)
		}
	}
	g.updateShipIconsInBetweenTurns()
	switch {
	case r < 0:
		return -1
	case r > 0:
		return 1
	}
	return 0
}
