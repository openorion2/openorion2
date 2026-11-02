package game

import "github.com/openorion2/openorion2/internal/save"

// The AI's research choice (AITECH.C, NEWTECH.C): All_AI_Tech_Select_, and
// Calc_Tech_Value_ (TECHVAL.C), which diplomacy's tech trades share.
// Spec: docs/turn/overview.md, All_AI_Tech_Select_.

// Tables of the research choice.
const (
	techAppKind      = 0x17e082 // u8 in a tech application row: its kind
	techAppStrategic = 0x17e084 // u8: allowed in strategic combat
	techKindValue    = 0x17d196 // u8 pairs by kind: base value, field-progress flag
	fieldRank        = 0x17d91a // u8 in a field row (0x17d916 + 4): the field's level
	rankValue        = 0x18360c // u32 by level (0..0x16)
)

// competitionKinds are the kinds Set_Competition_Tech_Values_ (0xfd219)
// stores at 0x1ab10c.. in this order.
var competitionKinds = [12]int{0x19, 0x12, 0xa, 0x13, 0xc, 0x10, 0xf, 0x1c, 0x18, 0x1a, 0x15, 0x21}

// setCompetitionTechValues is Set_Competition_Tech_Values_ (0xfd219): the
// others' best levels into _best_category, which Calc_Tech_Value_ reads.
func (g *Game) setCompetitionTechValues(p int) {
	for i, k := range competitionKinds {
		g.bestCategory[i] = int16(g.getCompetitionTechValues(k, p))
	}
}

func (g *Game) appKind(a int) int { return int(g.exeU8(uint32(techAppKind + a*techAppRow))) }
func (g *Game) appRank(a int) uint16 {
	return uint16(g.exeU8(uint32(fieldRank + g.techAppFieldOf(a)*0x17)))
}

// allAITechSelect is All_AI_Tech_Select_ (0xdca69). Compute_AI_Tech_Info_
// runs again on a stack copy of _aitech; the arena keeps the old block.
func (g *Game) allAITechSelect() {
	s := &g.state
	a := g.ai
	o := a.at(aiTech)
	saved := append([]byte(nil), a.mem[o:o+aiBlocks[aiTech].size]...)
	g.computeAITechInfo()
	copy(a.mem[o:], saved)
	for p := int(s.NumPlayers) - 1; p >= 0; p-- {
		P := &s.Players[p]
		if P.Eliminated != 0 || P.Objective == objectiveHuman {
			continue
		}
		g.aiTechSelect(p)
		if P.AppStatus[0x3e] == techKnown && P.Evolved == 0 {
			// The AI branch has no screen and cannot fail.
			_ = g.setupEvolutionaryUpgrade(p)
		}
	}
}

// aiTechSelect is AI_Tech_Select_ (0xdc288): a new application once the
// current field is done or a new tech came in.
func (g *Game) aiTechSelect(p int) {
	P := &g.state.Players[p]
	again := P.FieldStatus[P.ResearchField] == techKnown || P.Unknown323 != 0
	P.Unknown323 = 0
	if !again {
		return
	}
	app := g.chooseTechApplication(p)
	P.ResearchApp = uint8(app)
	P.ResearchField = uint8(g.techAppFieldOf(int(uint8(app))))
	if P.ResearchField >= 0x4b {
		P.Unknown59D = 1
	}
}

// fieldStatusRaw reads FieldStatus[f] as the original does for field -1:
// the byte before it, the high byte of MaintenanceParts[5].
func fieldStatusRaw(P *save.Player, f int) uint8 {
	if f == -1 {
		return uint8(uint16(P.MaintenanceParts[5]) >> 8)
	}
	return P.FieldStatus[f]
}

// allGrayBoxesResearched is All_Gray_Boxes_Researched_ (0xfd2f9).
func allGrayBoxesResearched(P *save.Player) bool {
	for _, f := range []int{0x37, 0x17, 0x39, 0x1d, 0x16, 0x1c} {
		if P.FieldStatus[f] != techKnown {
			return false
		}
	}
	return true
}

// getCompetitionPlayerTechValue is Get_Competition_Player_Tech_Value_
// (0xfc7ac): the highest field level among q's known applications 0..0x4a
// of the kind.
func (g *Game) getCompetitionPlayerTechValue(q, kind int) uint16 {
	best := uint16(0)
	for a := 0; a < 0x4b; a++ {
		if g.appKind(a) != kind || g.state.Players[q].AppStatus[a] != techKnown {
			continue
		}
		best = max(best, g.appRank(a))
	}
	return best
}

// getCompetitionTechValues is Get_Competition_Tech_Values_ (0xfd199): the
// same over every other player.
func (g *Game) getCompetitionTechValues(kind, p int) uint16 {
	best := uint16(0)
	for q := 0; q < int(g.state.NumPlayers); q++ {
		if q != p {
			best = max(best, g.getCompetitionPlayerTechValue(q, kind))
		}
	}
	return best
}

// chooseTechApplication is Choose_Tech_Application_ (0xfd335): weights of
// the researchable applications by value over cost (cost divided by player
// 0's research, not p's), trimmed by objective, then a weighted pick; with
// none, a hyper-advanced one.
func (g *Game) chooseTechApplication(p int) int16 {
	s := &g.state
	P := &s.Players[p]
	gray := allGrayBoxesResearched(P)
	g.setCompetitionTechValues(p)
	var vals, w [0xd4]uint32
	any := false
	for a := 1; a < 0xd4; a++ {
		if P.AppStatus[a] != 1 {
			continue
		}
		if g.settings[0xd8] != 0 && g.exeU8(uint32(techAppStrategic+a*techAppRow)) == 0 {
			continue
		}
		f := g.techAppFieldOf(a)
		if f >= 0x4b || fieldStatusRaw(P, f) != 2 {
			continue
		}
		v := g.calcTechValue(p, a)
		if gray {
			if f == 4 {
				v *= 2
			}
			if a == 0x72 {
				v *= 2
			}
			if a == 0x33 {
				v *= 5
			}
			if f == 0x49 {
				v *= 2
			}
			if v == 0 {
				v = 1
			}
		} else {
			switch f {
			case 0x16, 0x17, 0x1c, 0x1d, 0x37, 0x39:
				v++
			default:
				v = 0
			}
		}
		vals[a] = uint32(v)
		any = true
	}
	var maxW uint32
	if any {
		res := uint32(int32(s.Players[0].ResearchProduced))
		if res == 0 {
			res = 1
		}
		thr := int32(15)
		for done := false; !done; thr = thr * 3 / 2 {
			for a := 1; a < 0xd4; a++ {
				if vals[a] == 0 {
					w[a] = 0
					continue
				}
				c := int32(g.exe.u32(uint32(fieldCost+g.techAppFieldOf(a)*0x17)) / res)
				if c == 0 {
					c = 1
				}
				if c > thr {
					w[a] = 0
					continue
				}
				w[a] = vals[a] * uint32(thr) / uint32(c)
				maxW = max(maxW, w[a])
				done = true
			}
		}
	}
	if g.settings.Difficulty() > 0 && maxW > 0 {
		for a := 1; a < 0xd4; a++ {
			var drop bool
			switch P.Objective {
			case 0, 5:
				drop = w[a]*2 < maxW
			case 1, 2:
				drop = w[a]*6 < maxW
			case 3, 4:
				drop = w[a]*5 < maxW*4
			}
			if drop {
				w[a] = 0
			}
		}
	}
	if maxW > 0 {
		return getWeightedChoiceInt(&s.Random, w[:])
	}
	return g.chooseHyperAdvancedTech()
}

// chooseHyperAdvancedTech is Choose_Hyper_Advanced_Tech_ (0xfc734): one of
// the hyper-advanced applications 0xcc..0xd3 by Random(20).
func (g *Game) chooseHyperAdvancedTech() int16 {
	switch r := g.state.Random.Random(20); {
	case r == 1:
		return 0xcc
	case r < 2:
		return -1
	case r <= 6:
		return 0xcd
	case r < 12:
		return 0xce
	case r <= 13:
		return 0xcf
	case r < 0x12:
		return 0xd0
	case r == 0x12:
		return 0xd1
	case r == 0x13:
		return 0xd2
	case r == 0x14:
		return 0xd3
	}
	return -1
}

// themeValue is the ship and special theme part of Calc_Tech_Value_ (jump
// tables 0xfc819 and 0xfc829): a kind the theme favours gets that value.
func themeValue(ship, special uint8, kind int, v int32) int32 {
	switch ship {
	case 0:
		switch kind {
		case 0x1a, 0x1b, 0x19:
			v = 100
		}
	case 1:
		switch kind {
		case 0x1a:
			v = 50
		case 0x13, 0x1e:
			v = 100
		case 0x19:
			v = 20
		}
	case 2:
		switch kind {
		case 0x15, 0x1d, 0x18:
			v = 100
		}
	case 3:
		switch kind {
		case 0x24:
			v = 50
		case 0x26:
			v = 100
		}
	}
	switch special {
	case 0:
		if kind == 0x12 || kind == 0x17 {
			v = 50
		}
	case 1:
		switch kind {
		case 0xf, 0x10:
			v = 20
		case 0x20:
			v = 50
		case 0x23:
			v = 100
		}
	case 2:
		switch kind {
		case 0x12:
			v = 50
		case 0x14:
			v = 100
		}
	case 3:
		switch kind {
		case 0x1c:
			v = 100
		case 0x1a:
			v = 20
		}
	case 4:
		if kind == 0x25 {
			v = 100
		}
	case 5:
		if kind == 0x20 || kind == 0x1f {
			v = 100
		}
	case 6:
		if kind == 0x21 || kind == 0x22 {
			v = 100
		}
	}
	return v
}

// objectiveValue is Calc_Tech_Value_'s objective part.
func objectiveValue(obj uint8, kind int, v int32) int32 {
	switch obj {
	case 0:
		if kind == 0xc || kind == 3 || kind == 0xb {
			v = 100
		}
	case 1:
		if kind == 0x11 || kind == 0x21 {
			v = 100
		}
	case 2:
		switch kind {
		case 0x27:
			v = 50
		case 9, 0xa:
			v = 100
		}
	case 3:
		if kind == 2 {
			v = 100
		}
	case 4:
		if kind == 1 || kind == 4 {
			v = 100
		}
	case 5:
		if kind == 0 || kind == 4 {
			v = 100
		}
	}
	return v
}

// pickValue is Calc_Tech_Value_'s race pick part: what a player is good
// or bad at makes the kind worth more or less.
func (g *Game) pickValue(P *save.Player, kind int, v int32) int32 {
	pk := func(i int) int8 { return P.Picks[i] }
	switch kind {
	case 0:
		switch {
		case pk(save.PickFarming) < 0:
			v = 100
		case pk(save.PickFarming) > 0:
			v = 10
		}
		switch {
		case pk(save.PickLithovore) != 0:
			v = 1
		case pk(save.PickCybernetic) != 0:
			v = 20
		}
	case 1:
		if pk(save.PickIndustry) < 0 {
			v = 100
		}
	case 2:
		if pk(save.PickScience) != 0 {
			v = 100
		}
	case 3:
		switch {
		case pk(save.PickMoney) < 0:
			v = 100
		case pk(save.PickMoney) > 0:
			v = 20
		}
	case 4:
		if pk(save.PickIndustry) > 0 {
			v = 100
		}
		if pk(save.PickTolerant) != 0 {
			v = 1
		}
	case 6:
		if pk(save.PickSubterranean) != 0 {
			v = 20
		}
		switch {
		case pk(save.PickPopulation) < 0:
			v = 100
		case pk(save.PickPopulation) > 0:
			v = 5
		}
	case 0xc:
		if pk(save.PickSpying) != 0 {
			v = 50
		}
		if pk(save.PickGovernment)/2 == 2 {
			v = 50
		}
	case 0x10:
		if pk(save.PickGroundCombat) < 0 {
			v = 20
		}
	case 0x12:
		if pk(save.PickShipDefense) < 0 {
			v = 50
		}
	case 0x19:
		if pk(save.PickShipAttack) < 0 {
			v = 100
		}
	case 0x1b:
		if pk(save.PickShipAttack) > 0 {
			v = 100
		}
	case 0x1c:
		if pk(save.PickShipDefense) > 0 {
			v = 100
		}
	case 0x25:
		if pk(save.PickStealthyShips) != 0 {
			v = 1
		}
	case 0x28:
		if pk(save.PickGovernment)/2 == 3 {
			v = 1
		}
	}
	return v
}

// calcTechValue is Calc_Tech_Value_ (0xfc845): how much p wants app; the
// others' levels come from the last Set_Competition_Tech_Values_.
func (g *Game) calcTechValue(p, app int) int32 {
	s := &g.state
	P := &s.Players[p]
	comp := &g.bestCategory
	if !g.playerMayGetTechApp(p, app) || P.AppStatus[app] == techKnown {
		return 0
	}
	if P.Objective != objectiveHuman && app == 0x34 {
		return 0
	}
	kind := g.appKind(app)
	progressFlag := g.exeU8(uint32(techKindValue + 1 + kind*2))
	v := int32(g.exeU8(uint32(techKindValue + kind*2)))
	rank := int32(g.appRank(app))
	if rank > 0x16 {
		rank = 0x16
	}
	mult := int32(g.exe.u32(uint32(rankValue + rank*4)))
	if P.ShipTheme <= 3 {
		v = themeValue(P.ShipTheme, 0xff, kind, v)
	}
	if P.SpecialTheme <= 6 {
		v = themeValue(0xff, P.SpecialTheme, kind, v)
	}
	v = objectiveValue(P.Objective, kind, v)
	v = g.pickValue(P, kind, v)
	switch uint16(app) {
	case 5:
		if P.Picks[save.PickTelepathic] != 0 {
			v = 1
		}
	case 0x83:
		switch {
		case P.Picks[save.PickHighG] != 0:
			v = 1
		case P.Picks[save.PickLowG] != 0:
			v = 50
		}
	}
	if P.Objective == objectiveHuman {
		d := int32(g.settings.Difficulty())
		v = d*d*25 + 50
		switch kind {
		case 0x1a, 0x1b, 0x1d, 0x1e, 0x21, 0x15:
			v <<= 2
		}
	}
	if progressFlag == 0 {
		have := int32(0)
		for a := 0; a < 0xcc; a++ {
			if g.appKind(a) == kind && P.AppStatus[a] == techKnown {
				have = int32(g.appRank(a))
			}
		}
		v *= mult
		switch {
		case have+3 <= rank:
			v = (rank - have) * v / 3
		default:
			switch kind {
			case 0xe, 0x10, 0x12, 0x18, 0x19, 0x20, 0x27:
				v = 0
			default:
				v = v * 2 / (have + 3 - rank)
			}
		}
	} else {
		v *= mult
	}
	behind := func(c int16) {
		if mult < int32(c) {
			v *= int32(c) - mult
		}
	}
	switch kind {
	case 0x19:
		behind(comp[1])
	case 0x12:
		behind(comp[0])
	case 0x13:
		behind(comp[2])
	case 0xa:
		behind(comp[3])
	case 0xc:
		behind(comp[4])
	case 0xf:
		behind(comp[6])
	case 8:
		if uint32(g.getCompetitionPlayerTechValue(p, 0xf))+uint32(g.getCompetitionPlayerTechValue(p, 0x10)) < uint32(int32(comp[6])*2) {
			v *= 2
		}
	}
	if P.ShipTheme == 2 && kind == 0x18 {
		behind(comp[7])
	}
	switch P.ShipTheme {
	case 0:
		if kind == 0x1a {
			behind(comp[11])
		}
	case 1:
		if kind == 0x1a {
			behind(comp[11])
		}
		if kind == 0x13 {
			behind(comp[11])
		}
	case 2:
		if kind == 0x15 {
			behind(comp[11])
		}
	}
	if s.deltaStardate() < 150 && kind == 0x12 {
		v *= 2
	}
	f := g.techAppFieldOf(app)
	if P.FieldStatus[f] == techKnown {
		v = int32(uint32(v) * 5 >> 2)
	}
	if v == 0 && P.FieldStatus[f] == 2 {
		other := false
		for a := 0; a < 0xcc && !other; a++ {
			if a != app && g.techAppFieldOf(a) == f && P.AppStatus[a] != techKnown {
				other = true
			}
		}
		if !other {
			v = mult * 10
		}
	}
	return v
}
