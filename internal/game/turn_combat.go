package game

import (
	"github.com/openorion2/openorion2/internal/engine"
	"github.com/openorion2/openorion2/internal/save"
)

// The battle search of the turn (COMBFIND.C). Spec: docs/turn/overview.md,
// phase E; the strategic resolver is in docs/turn/combat.md.

// combatFind holds Search_For_Battles_' stack tables, which COMBFIND.C
// reaches through globals (0x1aa3b0, 0x1aa3b4, 0x1aa3ac, 0x1aa37c, …).
type combatFind struct {
	// has is _star_has_player: per star and player 11 bytes (+0..4 ship
	// type present, +5 transports, +6 any ship, +7 colonies, +8 s16
	// transports arriving next turn, +10 transports in transit).
	has       [combatStars][8][11]byte
	nonplayer [combatStars]byte   // monster owners 8..14 at the star, bit owner-8
	needs     [combatStars]uint16 // owners that want to fight there
	battle    [combatStars]uint8  // _battle_stars
	nBattle   int16               // 0x1aa3e8
	asked     [8][8]byte          // what 0x1aa378 points at while it runs
}

// combatTargets is Collect_Targets_For_Combat_'s result (0x1aa380, 0x2c
// bytes), a global: Send_Pick_Target_Info_ ships it to the attacker's machine.
type combatTargets struct {
	colT   [5]int16  // 0x1aa380, colony targets
	nColT  int16     // 0x1aa38a
	shipT  [15]int16 // 0x1aa38c, ship owner targets
	nShipT int16     // 0x1aa3aa
}

// combatCur is what 0x1aa3e0 and 0x1aa3e4 point at: the battle star's row of
// _star_has_player and its monster byte. Before the machine ever pointed them
// (a client that never hosted) they do not share bytes: the oracle's joiner
// sent back the monster byte and the row's first byte apart (netbattle1).
type combatCur struct {
	has     *[8][11]byte
	np      *byte
	nullHas [8][11]byte // where the unset 0x1aa3e0 reaches
	nullNP  byte        // where the unset 0x1aa3e4 reaches
	stale   [8][11]byte // a dead Search_For_Battles_ frame's row
	staleNP byte
}

// row is *0x1aa3e0.
func (g *Game) curRow() *[8][11]byte {
	if g.cc.has == nil {
		return &g.cc.nullHas
	}
	return g.cc.has
}

// curNonplayer is *0x1aa3e4.
func (g *Game) curNonplayer() *byte {
	if g.cc.np == nil {
		return &g.cc.nullNP
	}
	return g.cc.np
}

// setCur points 0x1aa3e0/0x1aa3e4 at star's row of Search_For_Battles_' tables.
func (g *Game) setCur(star int) {
	g.cc.has, g.cc.np = &g.cf.has[star], &g.cf.nonplayer[star]
}

// clearCur is Search_For_Battles_' reset of both pointers to 0 (0xea1a6).
func (g *Game) clearCur() { g.cc.has, g.cc.np = nil, nil }

// combatStars is the tables' length: 0x1918 / 0x58 stars (Antares included).
const combatStars = 73

func (f *combatFind) arriving(star, p int) uint16 {
	b := &f.has[star][p]
	return uint16(b[8]) | uint16(b[9])<<8
}

// searchForBattles is Search_For_Battles_ (0xe9d62): every star where a
// side wants to fight, in shuffled order, until no attacker finds a target;
// then retreats, discoveries and eliminations.
func (g *Game) searchForBattles() {
	s := &g.state
	var saved engine.AutoFunction
	if g.e != nil {
		saved = g.e.GetAutoFunction()
		if g.gameType <= 1 {
			g.e.AssignAutoFunction(g.drawMiniMainScreen, 1)
		}
	}
	g.updateShipIconsInBetweenTurns()
	f := &combatFind{}
	g.cf = f
	defer func() { g.cf = nil }()
	for i := int(s.NumColonies) - 1; i >= 0; i-- {
		c := &s.Colonies[i]
		if c.Owner == -1 {
			continue
		}
		f.has[s.Planets[c.Planet].Star][c.Owner][7]++
	}
	f.nonplayer[s.NumStars] |= 1
	for i := 0; i < int(s.NumShips); i++ {
		sh := &s.Ships[i]
		loc := int(sh.Location)
		if sh.Location < 0x48 {
			if sh.Status != save.ShipAtStar {
				continue
			}
			if loc < 0 {
				continue // would write below the original's table; never seen
			}
			if sh.Owner >= 8 {
				f.nonplayer[loc] |= 1 << uint(sh.Owner-8)
				f.needs[loc] |= 1 << uint(sh.Owner)
				continue
			}
			b := &f.has[loc][sh.Owner]
			b[6] = 1
			typ := sh.Design.Type
			if int(typ) < len(b) {
				b[typ] = 1
			}
			if typ == 2 {
				b[5] = 1
			} else if typ == 0 {
				f.needs[loc] |= 1 << uint(sh.Owner)
			}
			continue
		}
		if sh.Design.Type != 2 || sh.Status != save.ShipInTransit {
			continue
		}
		dest := int(int16(uint16(sh.Location) - 500))
		if dest < 0 || dest >= combatStars {
			continue // outside the original's table; transits are 500+star
		}
		b := &f.has[dest][sh.Owner]
		if sh.ETA == 1 {
			n := f.arriving(dest, int(sh.Owner)) + 1
			b[8], b[9] = byte(n), byte(n>>8)
		} else {
			b[10] = 1
		}
	}
	for st := int16(0); st <= s.NumStars; st++ {
		if f.needs[st] != 0 && !s.eventCheckSpaceAnomalyAtStar(st) {
			f.battle[f.nBattle] = uint8(st)
			f.nBattle++
		}
	}
	shuffleChar(&s.Random, f.battle[:f.nBattle])
	if g.gameType <= 1 {
		g.asked = &f.asked
		for f.nBattle > 0 {
			star, att := g.pickStarAndAttacker()
			if star == -1 {
				continue
			}
			g.fightAtStar(star, att)
		}
		g.asked = nil
	} else if g.gameType <= 3 {
		// 0xea1db: the battles run under Client_Combat_Loop_; then msg 0x1a.
		g.net.set8(netState, 0xe)
		if err := g.clientCombatLoop(); err != nil {
			panic(err)
		}
		g.moxSendMessage(netAllHumans, 0x1a, nil)
		g.staleCur()
	}
	g.processRetreatingShips(true)
	if g.messedUpFleetPalette {
		// Set_Reload_Fleet_Box_Palette_Flag_ (0x8a159)
		g.ms.mainscrPalette, g.ms.shipIconPlayerID = 0x65, -1
		g.messedUpFleetPalette = false
	}
	for st := int(s.NumStars) - 1; st >= 0; st-- {
		g.doSystemDiscoveriesAtStar(st)
	}
	if g.e != nil {
		g.e.PutAutoFunction(saved)
	}
	g.cf = nil
	g.checkForEliminatedPlayers()
}

// fightAtStar is the body of Search_For_Battles_' loop: the attacker's
// target, then the battle.
func (g *Game) fightAtStar(star, att int16) {
	s := &g.state
	shipT, colT := int16(-1), int16(-1)
	sneak := false
	switch {
	case att >= 8:
		g.getNPCTarget(star, att, &shipT, &colT)
	case s.Players[att].Objective == save.ObjectiveHuman:
		if s.Stars[star].IgnoreCombat>>uint(att)&1 != 0 {
			break
		}
		g.getLocalHumanTarget(star, att, &shipT, &colT)
		if colT == -1 && shipT == -1 {
			s.Stars[star].IgnoreCombat |= 1 << uint(att)
		}
	default:
		g.getAITarget(star, att, &shipT, &colT, &sneak)
	}
	def := shipT
	if colT != -1 {
		def = int16(s.Colonies[colT].Owner)
		s.Players[def].CombatSlot = uint8(att)
	}
	if att == 12 && colT != -1 {
		bribe := int16(-1)
		if s.Players[def].Objective == save.ObjectiveHuman {
			bribe = g.dragonNewStarPopup(def, star)
		}
		if bribe != -1 {
			g.doDragonBribe(def, star, bribe)
			def = -1
		}
	}
	if def == -1 {
		return
	}
	planet := int16(-1)
	if colT != -1 {
		planet = s.Colonies[colT].Planet
	}
	s.Stars[star].IgnoreCombat = 0
	g.do1Combat(star, att, shipT, planet, nil, sneak)
	g.clearCur()
	g.updatePlayerNeedsToAttack(star, att, def)
}

// pickStarAndAttacker is Pick_Star_And_Attacker_ (0xe84a5): from the end of
// the battle list, a star and an attacker there with something to attack;
// stars left without attackers drop out of the list.
func (g *Game) pickStarAndAttacker() (star, att int16) {
	f := g.cf
	di := f.nBattle - 1
	star = -1
	for {
		if star != -1 || di < 0 {
			return star, att
		}
		if g.e != nil {
			g.e.MoxUpdate()
		}
		star = int16(f.battle[di])
		if (g.gameType == 2 || g.gameType == 3) && g.someStarPlayerIsBusy(star) {
			di--
			star = -1
			continue
		}
		for {
			att = g.pickAttackerAtStar(star)
			if att == -1 {
				f.nBattle--
				star = -1
				f.battle[di] = f.battle[f.nBattle]
				di--
			} else {
				f.needs[star] &^= 1 << uint(att)
				g.collectTargetsForCombat(star, att)
				if g.ct.nShipT == 0 && g.ct.nColT == 0 {
					att = -1
				}
			}
			if att != -1 || star == -1 {
				break
			}
		}
	}
}

// pickAttackerAtStar is Pick_Attacker_At_Star_ (0xe8194): a uniform pick
// among the monsters at the star if any, else among the players.
func (g *Game) pickAttackerAtStar(star int16) int16 {
	s := &g.state
	m := g.cf.needs[star]
	pick := int16(-1)
	if m == 0 {
		return -1
	}
	k := uint32(0)
	lo, hi := int16(0), s.NumPlayers
	if m >= 0x100 {
		lo, hi = 8, 15
	}
	for o := lo; o < hi; o++ {
		if m>>uint(o)&1 != 0 {
			k++
			if s.Random.Random(k) == 1 {
				pick = o
			}
		}
	}
	return pick
}

// playerMayAttackPlayer is Player_May_Attack_Player_ (0xe8231): humans and
// monsters always may; an AI at war may, at peace without treaties only an
// aggressive one, a personality-3 one one time in ten.
func (g *Game) playerMayAttackPlayer(a, b int) bool {
	s := &g.state
	if a == b {
		return false
	}
	if a >= 8 || s.Players[a].Objective == save.ObjectiveHuman {
		return true
	}
	P := &s.Players[a]
	t := P.TreatyStatus[b]
	switch {
	case t > 0 && t < 4:
		return false
	case t >= 4 && t <= 6:
		return true
	case t > 6:
		return false
	}
	if P.Diplo80E[b] != 0 || P.TradeTreaty[b] != 0 || P.ResearchTreaty[b] != 0 {
		return false
	}
	switch P.Personality {
	case 0:
		return true
	case 3:
		return s.Random.Random(10) == 1
	}
	return false
}

// collectTargetsForCombat is Collect_Targets_For_Combat_ (0xe82b4): the
// colonies att may attack (orbits from the last), the players with ships
// but no colony there, then the monsters from owner 14 down.
func (g *Game) collectTargetsForCombat(star, att int16) {
	s := &g.state
	f := g.cf
	g.ct.nColT = 0
	if star < s.NumStars {
		st := &s.Stars[star]
		for o := 4; o >= 0; o-- {
			pl := st.Planets[o]
			if pl < 0 {
				continue
			}
			ci := s.Planets[pl].Colony
			if ci < 0 {
				continue
			}
			if g.playerMayAttackPlayer(int(att), int(s.Colonies[ci].Owner)) {
				g.ct.colT[g.ct.nColT] = ci
				g.ct.nColT++
			}
		}
	}
	g.ct.nShipT = 0
	for p := s.NumPlayers - 1; p >= 0; p-- {
		if p == att {
			continue
		}
		b := &f.has[star][p]
		if b[6] != 0 && b[7] == 0 && g.playerMayAttackPlayer(int(att), int(p)) {
			g.ct.shipT[g.ct.nShipT] = p
			g.ct.nShipT++
		}
	}
	if f.nonplayer[star] == 0 {
		return
	}
	for bit := 6; bit >= 0; bit-- {
		o := int16(bit + 8)
		if o != att && f.nonplayer[star]>>uint(bit)&1 != 0 {
			g.ct.shipT[g.ct.nShipT] = o
			g.ct.nShipT++
		}
	}
}

// updatePlayerNeedsToAttack is Update_Player_Needs_To_Attack_ (0xe6aa2):
// sides with nothing left at the star stop wanting to fight there.
func (g *Game) updatePlayerNeedsToAttack(star, a, b int16) {
	f := g.cf
	for _, p := range [2]int16{a, b} {
		var present bool
		if p >= 8 {
			present = f.nonplayer[star]>>uint(p)&1 != 0
		} else {
			present = f.has[star][p][6] != 0
		}
		if !present {
			f.needs[star] &^= 1 << uint(p)
		}
	}
}

// resetOneShotSystemSpecial is Reset_One_Shot_System_Special_ (0xe98f2,
// body in patch code at 0xf4f74): last turn's discovery markers 50..89 clear
// the planets' specials and the star's, 90 and up only the star's.
func (s *State) resetOneShotSystemSpecial(star int) {
	st := &s.Stars[star]
	if st.Special < 50 {
		return
	}
	if st.Special < 90 {
		for o := 4; o >= 0; o-- {
			if pl := st.Planets[o]; pl != -1 {
				s.Planets[pl].Special = 0
			}
		}
	}
	st.Special = 0
}

// findPlanetAtStarSatisfying is Find_Planet_At_Star_Satisfying_ (0xe9871):
// the first orbit whose planet passes ok, skipping empty orbits; -1 if none.
func (s *State) findPlanetAtStarSatisfying(star int, ok func(*save.Planet) bool) int {
	for _, pl := range s.Stars[star].Planets {
		if pl != -1 && ok(&s.Planets[pl]) {
			return int(pl)
		}
	}
	return -1
}

// doSystemDiscoveriesAtStar is Do_System_Discoveries_At_Star_ (0xe9927): a
// random player with ships at a star without monsters picks up its special;
// the star keeps a marker (special + 10·kind + player) for the report.
func (g *Game) doSystemDiscoveriesAtStar(star int) {
	s := &g.state
	f := g.cf
	st := &s.Stars[star]
	s.resetOneShotSystemSpecial(star)
	if st.Special == 0 || f.nonplayer[star] != 0 {
		return
	}
	var present [8]uint8
	n := 0
	for p := int(s.NumPlayers) - 1; p >= 0; p-- {
		if f.has[star][p][6] != 0 {
			n++
			present[p] = 1
		}
	}
	if n <= 0 {
		return
	}
	p := getWeightedChoiceChar(&s.Random, present[:s.NumPlayers])
	P := &s.Players[p]
	switch st.Special {
	case 2:
		P.BC += 50
		st.Special = uint8(p) + 50
	case 3:
		P.BC += 100
		st.Special = uint8(p) + 60
	case 7:
		st.Special = uint8(p) + 70
		f.has[star][p][7] = 1
		ci := 0
		for ci < int(s.NumColonies) && s.Colonies[ci].Planet != -1 {
			ci++
		}
		if ci >= int(s.NumColonies) {
			s.NumColonies = int16(ci + 1)
		}
		planet := s.findPlanetAtStarSatisfying(star, func(pl *save.Planet) bool { return pl.Special == 7 })
		g.initColony(ci, planet, int(p))
		c := &s.Colonies[ci]
		lim := uint8(g.colonyRacePopLimit(c, int(p)))
		c.PopCount = min(lim, 3)
		c.Buildings = [len(c.Buildings)]uint8{}
		c.Colonists = [len(c.Colonists)]uint32{}
		for k := int(c.PopCount) - 1; k >= 0; k-- {
			v := uint32(p)&0xf | (uint32(p)&7)<<4
			switch {
			case s.Planets[planet].FoodPerFarmer == 0 || P.Picks[save.PickLithovore] != 0:
				v |= (s.Random.Random(2) & 3) << 7
			case P.Picks[save.PickCybernetic] != 0 && k == 0:
				v |= 1 << 7
			}
			c.Colonists[k] = v | 1<<9
		}
		s.computeStarColonyStuff(star)
		g.colonyCalculation(ci)
		c.LastBuilt = -39
	case 8:
		st.Special = uint8(p) + 80
	case 10:
		if st.ArtifactsHome != 0 {
			return
		}
		st.Special = uint8(p) + 90
		st.ArtifactsHome = 1
		k := s.Random.Random(4)>>2 + 1
		var m [msgSize]byte
		m[0], m[2] = 0x14, byte(star)
		for k != 0 {
			k--
			n := uint32(0)
			for app := 1; app < numTechApps; app++ {
				fs := uint8(uint16(P.MaintenanceParts[5]) >> 8) // FieldStatus[-1]
				if fl := g.techAppFieldOf(app); fl >= 0 {
					fs = P.FieldStatus[fl]
				}
				if fs == 2 && g.playerMayGetTechApp(int(p), app) {
					n++
					if s.Random.Random(n) == 1 {
						m[3+k] = byte(app)
					}
				}
			}
			if n != 0 {
				g.playerGetsTechApp(int(p), int(m[3+k]))
			}
		}
		if m[3] != 0 {
			// Byte 1 (priority) and bytes 5.. are stale stack in the original.
			i := g.addMsg(int(p), m)
			g.markStaleMsg(i, 1, 2)
			g.markStaleMsg(i, 5, msgSize)
		}
	}
}

// checkForEliminatedPlayers is Check_For_Eliminated_Players_ (0xe4eb3):
// players left without a populated colony are out; every player still in
// clears its combat slot.
func (g *Game) checkForEliminatedPlayers() {
	s := &g.state
	var alive [8]bool
	for i := int(s.NumColonies) - 1; i >= 0; i-- {
		c := &s.Colonies[i]
		if uint32(int32(c.Owner)) < 8 && c.IsOutpost == 0 {
			alive[c.Owner] = true
		}
	}
	for p := int(s.NumPlayers) - 1; p >= 0; p-- {
		P := &s.Players[p]
		if P.Eliminated != 0 {
			continue
		}
		if !alive[p] {
			g.eliminatePlayer(p)
			s.EventData[0x106] = 1 // Event_Race_Eliminated_ (0x233ab)
			s.EventData[0x105] = byte(p)
		}
		P.CombatSlot = 0xff
	}
}
