package game

import (
	"strings"

	"github.com/openorion2/openorion2/internal/save"
)

// The colony economy behind Do_Player_Colony_Calculations_ (0xe2d72).
// Spec: docs/formats/tech.md, "Colony calculations".

// Constant tables of ORION2.EXE (ELF addresses).
const (
	tblMineralsPerMine   = 0xdd4b5 // u8 by planet minerals
	tblClimateMaint      = 0xdd4ba // u8 by effective climate
	tblGovMorale         = 0xdd4c4 // i8 by government
	tblNoCapitalMorale   = 0xdd4cc // i8 by government/2
	tblBarracksMorale    = 0xdd4cf // u8 by government
	tblAIJobBonus        = 0xdd4d7 // i8 by difficulty
	tblDeepCoreMinerals  = 0xdd4dc // u8 by planet minerals
	tblPollutionSize     = 0xdd4e1 // u8 by planet size
	tblAIBCBonus         = 0xdd4e6 // u8 by difficulty
	tblFoodPerFarmer     = 0x17d81c
	tblBuildingUpkeep    = 0x17eb49 // i16, 0x13-byte rows by building
	tblBuildingUpkeepRow = 0x13
	tblSkillData         = 0x17d218 // 0x12-byte rows by skill
	tblSkillDataRow      = 0x12
)

// Event slots in _event_data (0x19aba4) the checks read.
const (
	evPlagueState  = 0x91 // plague: state byte, colony word at +2
	evDepletion    = 0x9a // population boom: state byte, colony word at +2
	evStarResearch = 0xd9 // star research event: state byte, star word at +2
	evAnomaly      = 0xe2 // space anomaly: state byte, star word at +2
)

// Commodity indices of the per-colony arrays.
const (
	food, industry, research, bc = save.Food, save.Industry, save.Research, save.BC
)

func (s *State) eventWord(off int) int16 {
	return int16(uint16(s.EventData[off]) | uint16(s.EventData[off+1])<<8)
}

func eventActive(state byte, alsoFour bool) bool {
	return state == 2 || state == 6 || (alsoFour && state == 4)
}

// eventCheckSpaceAnomaly is Event_Check_Space_Anomaly_ (0x2341e).
func (s *State) eventCheckSpaceAnomaly(ci int) bool {
	return eventActive(s.EventData[evAnomaly], true) &&
		int16(s.Planets[s.Colonies[ci].Planet].Star) == s.eventWord(evAnomaly+2)
}

// eventCheckSpaceAnomalyAtStar is Event_Check_Space_Anomaly_At_Star_
// (0x242cf).
func (s *State) eventCheckSpaceAnomalyAtStar(star int16) bool {
	return eventActive(s.EventData[evAnomaly], true) && star == s.eventWord(evAnomaly+2)
}

// eventCheckPopulationBoom is Event_Check_Population_Boom_ (0x23509).
func (s *State) eventCheckPopulationBoom(ci int) bool {
	return eventActive(s.EventData[evDepletion], true) && int16(ci) == s.eventWord(evDepletion+2)
}

// eventCheckPlague is Event_Check_Plague_ (0x234b8).
func (s *State) eventCheckPlague(ci int) bool {
	return eventActive(s.EventData[evPlagueState], true) && int16(ci) == s.eventWord(evPlagueState+2)
}

// eventCheckColonyResearching is Event_Check_Colony_Researching_ (0x23dfe).
func (s *State) eventCheckColonyResearching(ci int) bool {
	if eventActive(s.EventData[evPlagueState], false) && int16(ci) == s.eventWord(evPlagueState+2) {
		return true
	}
	return eventActive(s.EventData[evStarResearch], false) &&
		int16(s.Planets[s.Colonies[ci].Planet].Star) == s.eventWord(evStarResearch+2)
}

func knows(p *save.Player, app int) bool { return p.AppStatus[app] == 3 }

func bld(c *save.Colony, id int) bool { return c.Buildings[id] != 0 }

// colonyOfficer is Colony_Officer_ (0xdd9f2): the star's governor for the
// colony owner, -1 when none or travelling.
func (s *State) colonyOfficer(c *save.Colony) int {
	o := int(s.Stars[s.Planets[c.Planet].Star].Officer[c.Owner])
	if o != -1 && s.Leaders[o].ETA != 0 {
		return -1
	}
	return o
}

// prodBreakdown is the 0x256-byte explanation record Colony_Job_Production_
// and the per-unit routines fill when asked (the production popups).
type prodBreakdown struct {
	base    int32       // +0: per-unit base
	race    [10]int16   // +4: race bonus (8 androids, 9 natives)
	apps    [0xcc]int16 // +0x18: by application
	bldgs   [0x31]int16 // +0x1c0: by building
	aquatic int32       // +0x222
	special int32       // +0x226: research specials, counted in the total
	intel   int32       // +0x22a: research application 0x54
	base20  int32       // +0x22e
	grav    int32       // +0x232
	conq    int32       // +0x236
	pollute int32       // +0x23a: industry's pollution (never set per colonist)
	morale  int32       // +0x23e
	gov     int32       // +0x242
	block   int32       // +0x246
	lead    int32       // +0x24a
	ai      int32       // +0x24e
	total   int32       // +0x252
}

// colonyFood2PerFarmer is Colony_Food2_Per_Farmer_ (0xde03e): food per
// farmer in half units, recorded in bd when given.
func (s *State) colonyFood2PerFarmer(c *save.Colony, p int) int16 {
	return s.colonyFood2PerFarmerB(c, p, nil)
}

func (s *State) colonyFood2PerFarmerB(c *save.Colony, p int, bd *prodBreakdown) int16 {
	v := int16(s.Planets[c.Planet].FoodPerFarmer) * 2
	if bd != nil {
		bd.base = int32(v)
	}
	if v == 0 && knows(&s.Players[p], 0x1d) {
		v = 2
		if bd != nil {
			bd.apps[0x1d] = v
		}
	}
	if v > 0 {
		if bld(c, 0x2e) {
			v += 4
			if bd != nil {
				bd.bldgs[0x2e] = 4
			}
		}
		if bld(c, 4) {
			v += 2
			if bd != nil {
				bd.bldgs[4] = 2
			}
		}
	}
	return v
}

// colonyEnvironmentalStuff is Colony_Environmental_Stuff_ (0xe1ced).
func (s *State) colonyEnvironmentalStuff(c *save.Colony) {
	c.Climate = s.Planets[c.Planet].Climate
	if c.Climate == 1 && (bld(c, 0x17) || bld(c, 0x18) || bld(c, 0x1c)) {
		c.Climate = 2
	}
	c.CanFarm = 0
	if s.colonyFood2PerFarmer(c, int(c.Owner)) != 0 {
		c.CanFarm = 0xff
	}
}

// colonyIndustryPerWorker is Colony_Industry_Per_Worker_ (0xdec95).
func (g *Game) colonyIndustryPerWorker(c *save.Colony, p int) int16 {
	return g.colonyIndustryPerWorkerB(c, p, nil)
}

func (g *Game) colonyIndustryPerWorkerB(c *save.Colony, p int, bd *prodBreakdown) int16 {
	s := &g.state
	v := int16(g.exeU8(tblMineralsPerMine + uint32(s.Planets[c.Planet].Minerals)))
	if bd != nil {
		bd.base = int32(v)
	}
	for _, b := range []struct {
		id int
		v  int16
	}{{4, 1}, {7, 1}, {0x24, 2}, {0xc, 3}} {
		if bld(c, b.id) {
			v += b.v
			if bd != nil {
				bd.bldgs[b.id] = b.v
			}
		}
	}
	if knows(&s.Players[p], 0x6c) {
		v++
		if bd != nil {
			bd.apps[0x6c] = 1
		}
	}
	return v
}

// colonyResearchPerScientist is Colony_Research_Per_Scientist_ (0xdfdc6).
func (s *State) colonyResearchPerScientist(c *save.Colony) int16 {
	return s.colonyResearchPerScientistB(c, nil)
}

func (s *State) colonyResearchPerScientistB(c *save.Colony, bd *prodBreakdown) int16 {
	v := int16(3)
	if bd != nil {
		bd.base = 3
	}
	for _, b := range []struct {
		id int
		v  int16
	}{{4, 1}, {0x1e, 2}, {0x13, 3}, {0x23, 1}} {
		if bld(c, b.id) {
			v += b.v
			if bd != nil {
				bd.bldgs[b.id] = b.v
			}
		}
	}
	switch s.Planets[c.Planet].Special {
	case 0xa:
		v += 2
		if bd != nil {
			bd.special += 2
		}
	case 0xb:
		v += 5
		if bd != nil {
			bd.special += 5
		}
	}
	return v
}

// mixedRaceMoralePenalty is Mixed_Race_Morale_Penalty_ (0xddad4).
func mixedRaceMoralePenalty(c *save.Colony) int8 {
	if c.PopCount < 2 || bld(c, 1) {
		return 0
	}
	r := -1
	for n := int(c.PopCount); n > 0; n-- {
		race := int(c.Colonists[n-1] & 0xf)
		if race >= 8 {
			continue
		}
		if r == -1 {
			r = race
		} else if race != r {
			return -4
		}
	}
	return 0
}

// moraleParts are Colony_Morale_'s terms, which its explanation lists.
type moraleParts struct {
	gov, half                                                 int32
	mixed, lead, b14, app, b1f, noCapital, police, guard, bar int32
	noBarracks                                                bool
}

// colonyMorale is Colony_Morale_ (0xddb25): the colony's morale, and its
// terms for the explanation.
func (g *Game) colonyMorale(c *save.Colony) moraleParts {
	s := &g.state
	p := &s.Players[c.Owner]
	var m moraleParts
	m.gov = int32(p.Picks[save.PickGovernment])
	m.half = m.gov / 2
	m.mixed = int32(mixedRaceMoralePenalty(c))
	c.Morale = int8(g.exeU8(uint32(tblGovMorale + m.gov)))
	if m.half == 3 {
		return m
	}
	if o := s.colonyOfficer(c); o != -1 {
		sk := byte(s.Leaders[o].SpecialSkills >> 8)
		switch {
		case sk&0x80 != 0:
			m.lead = (int32(s.ownedOfficerLevel(o))*3 + 4) / 2
		case sk&0x40 != 0:
			m.lead = int32(s.ownedOfficerLevel(o)) + 1
		}
	}
	if bld(c, 0x14) {
		m.b14 = 4
	}
	if knows(p, 0xc3) {
		m.app = 4
	}
	if bld(c, 0x1f) {
		m.b1f = 6
	}
	// A home planet without a colony makes the original read before the
	// colony array; the port counts it as no capital.
	if hp := p.HomePlanet; hp == -1 || s.Planets[hp].Colony < 0 || !bld(&s.Colonies[s.Planets[hp].Colony], 9) {
		m.noCapital = int32(int8(g.exeU8(uint32(tblNoCapitalMorale + m.half))))
	}
	if uint32(m.half) <= 1 && knows(p, 0x93) {
		m.police = 2
	}
	switch {
	case bld(c, 0x16):
		m.bar = int32(g.exeU8(uint32(tblBarracksMorale + m.gov)))
	case bld(c, 2):
		m.guard = int32(g.exeU8(uint32(tblBarracksMorale + m.gov)))
	case uint32(m.half) <= 1:
		m.noBarracks = true
	}
	v := uint8(c.Morale)
	v += uint8(m.b14) + uint8(m.app) + uint8(m.b1f) + uint8(m.lead) + uint8(m.mixed)
	v += uint8(m.police)
	v += uint8(m.guard) + uint8(m.bar)
	v += uint8(m.noCapital)
	c.Morale = int8(v)
	return m
}

// colonyUpkeepCounts is the colonist loop of Colony_Food_Maintenance_ and
// Colony_Industry_Maintenance_: demand by group and by race.
func colonyUpkeepCounts(c *save.Colony, players *[8]save.Player, per func(*save.Player) uint8, extra int,
	own, other, conquered, extraOut *uint8, byRace *[8]uint8) {
	*byRace = [8]uint8{}
	*own, *extraOut, *conquered, *other = 0, 0, 0, 0
	for i := 0; i < int(c.PopCount); i++ {
		col := c.Colonists[i]
		race := int(col & 0xf)
		if race < 8 {
			v := per(&players[race])
			switch {
			case race == int(c.Owner):
				*own += v
			case col>>10&1 != 0:
				*conquered += v
			default:
				*other += v
			}
			byRace[race] += v
		} else if race == extra {
			*extraOut += 2
		}
	}
}

// colonyIndustryMaintenance is Colony_Industry_Maintenance_ (0xdf546).
func (s *State) colonyIndustryMaintenance(ci int) {
	c := &s.Colonies[ci]
	if s.eventCheckSpaceAnomaly(ci) {
		c.Upkeep[industry] = 0
		return
	}
	d := &c.IndustryDem
	colonyUpkeepCounts(c, &s.Players, func(p *save.Player) uint8 {
		if p.Picks[save.PickCybernetic] != 0 {
			return 1
		}
		return 0
	}, 8, &d[0], &d[2], &d[3], &d[1], &c.IndustryByRace)
	c.Upkeep[industry] = uint8((int32(int16(uint16(d[0])+uint16(d[1])+uint16(d[3])+uint16(d[2]))) + 1) / 2)
}

// colonyFoodMaintenance is Colony_Food_Maintenance_ (0xdeb4b).
func (s *State) colonyFoodMaintenance(ci int) {
	c := &s.Colonies[ci]
	if s.eventCheckSpaceAnomaly(ci) {
		c.Upkeep[food] = 0
		return
	}
	d := &c.FoodDemand
	colonyUpkeepCounts(c, &s.Players, func(p *save.Player) uint8 {
		switch {
		case p.Picks[save.PickCybernetic] != 0:
			return 1
		case p.Picks[save.PickLithovore] != 0:
			return 0
		}
		return 2
	}, 9, &d[0], &d[1], &d[2], &d[3], &c.FoodByRace)
	c.Upkeep[food] = uint8((int32(int16(uint16(d[2])+uint16(d[0])+uint16(d[3])+uint16(d[1]))) + 1) / 2)
}

// colonyProdCache is a Colony_Empire_Base_*_Produced_ cache (0x183568..,
// 0x1aa358..): the last colony and player, a value per race, 99 for none.
type colonyProdCache struct {
	colony int // -1 matches no colony, as the original's null pointer
	player uint8
	vals   [10]int8
}

func (g *Game) prodCache(job int) *colonyProdCache {
	if g.prodCaches == nil {
		g.prodCaches = &[3]colonyProdCache{{colony: -1}, {colony: -1}, {colony: -1}}
	}
	return &g.prodCaches[job]
}

// lookup resets the cache when the colony or player differs from the last
// call, or always when an explanation is recorded.
func (pc *colonyProdCache) lookup(ci, p int, force bool) {
	if pc.colony != ci || uint16(pc.player) != uint16(p) || force {
		for i := range pc.vals {
			pc.vals[i] = 99
		}
		pc.colony, pc.player = ci, uint8(p)
	}
}

// colonyEmpireBaseFood2 is Colony_Empire_Base_Food2_Produced_ (0xde0c6):
// race's food per farmer at colony ci of p, cached per race. With bd the
// value is computed anew (cache reset) and recorded.
func (g *Game) colonyEmpireBaseFood2(ci, p, race int) int16 {
	return g.colonyEmpireBaseFood2B(ci, p, race, nil)
}

func (g *Game) colonyEmpireBaseFood2B(ci, p, race int, bd *prodBreakdown) int16 {
	s := &g.state
	c := &s.Colonies[ci]
	var v int16
	if int16(c.Owner) == int16(p) && bd == nil {
		v = int16(c.FoodPerFarmer)
	} else {
		v = s.colonyFood2PerFarmerB(c, p, bd)
	}
	if v <= 0 {
		return v
	}
	pc := g.prodCache(0)
	pc.lookup(ci, p, bd != nil)
	if pc.vals[race] != 99 {
		return int16(pc.vals[race])
	}
	var bonus, aquatic int16
	switch {
	case race < 8:
		rp := &s.Players[race]
		bonus = int16(rp.Picks[save.PickFarming])
		if rp.Picks[save.PickAquatic] != 0 {
			cl := s.Colonies[pc.colony].Climate
			switch cl {
			case 4, 6:
				aquatic = (int16(g.exeU8(tblFoodPerFarmer+8)) - int16(g.exeU8(tblFoodPerFarmer+uint32(cl)))) * 2
			case 5, 8:
				aquatic = (int16(g.exeU8(tblFoodPerFarmer+9)) - int16(g.exeU8(tblFoodPerFarmer+uint32(cl)))) * 2
			}
		}
	case race == 8:
		bonus = 6
	case race == 9:
		bonus = 4
	}
	if bd != nil {
		bd.race[race] = bonus
		bd.aquatic = int32(aquatic)
	}
	v += bonus
	v += aquatic
	pc.vals[race] = int8(v)
	return v
}

// colonyEmpireBaseIndustry is Colony_Empire_Base_Industry_Produced_
// (0xded47).
func (g *Game) colonyEmpireBaseIndustry(ci, p, race int) int16 {
	return g.colonyEmpireBaseIndustryB(ci, p, race, nil)
}

func (g *Game) colonyEmpireBaseIndustryB(ci, p, race int, bd *prodBreakdown) int16 {
	s := &g.state
	c := &s.Colonies[ci]
	var v int16
	if int16(c.Owner) == int16(p) && bd == nil {
		v = int16(c.IndustryPer)
	} else {
		v = g.colonyIndustryPerWorkerB(c, p, bd)
	}
	pc := g.prodCache(1)
	pc.lookup(ci, p, bd != nil)
	if pc.vals[race] != 99 {
		return int16(pc.vals[race])
	}
	var bonus int16
	switch {
	case race < 8:
		bonus = int16(s.Players[race].Picks[save.PickIndustry])
	case race == 8:
		bonus = 3
	}
	if bd != nil {
		bd.race[race] = bonus
	}
	v += bonus
	pc.vals[race] = int8(v)
	return v
}

// colonyEmpireBaseResearch is Colony_Empire_Base_Research_Produced_
// (0xdfe77).
func (g *Game) colonyEmpireBaseResearch(ci, p, race int) int16 {
	return g.colonyEmpireBaseResearchB(ci, p, race, nil)
}

func (g *Game) colonyEmpireBaseResearchB(ci, p, race int, bd *prodBreakdown) int16 {
	s := &g.state
	c := &s.Colonies[ci]
	var v int16
	if int16(c.Owner) == int16(p) && bd == nil {
		v = int16(c.ResearchPer)
	} else {
		v = s.colonyResearchPerScientistB(c, bd)
	}
	pc := g.prodCache(2)
	pc.lookup(ci, p, bd != nil)
	if pc.vals[race] != 99 {
		return int16(pc.vals[race])
	}
	var bonus, tech int16
	switch {
	case race < 8:
		rp := &s.Players[race]
		bonus = int16(rp.Picks[save.PickScience])
		if race == int(c.Owner) && knows(rp, 0x54) {
			tech = 1
		}
	case race == 8:
		bonus = 3
	}
	if bd != nil {
		bd.race[race] = bonus
		bd.intel = int32(tech)
	}
	v += bonus + tech
	pc.vals[race] = int8(v)
	return v
}

// colonyPopBaseProd is Colony_Pop_Base_Prod_Produced_ (0xde22c): a
// colonist's base output, at least 1 unless farming is impossible.
func (g *Game) colonyPopBaseProd(ci int, col uint32, job int, bd *prodBreakdown) int16 {
	c := &g.state.Colonies[ci]
	race := int(col & 0xf)
	var f int16
	switch job {
	case 0:
		f = g.colonyEmpireBaseFood2B(ci, int(c.Owner), race, bd)
	case 1:
		f = g.colonyEmpireBaseIndustryB(ci, int(c.Owner), race, bd)
	default:
		f = g.colonyEmpireBaseResearchB(ci, int(c.Owner), race, bd)
	}
	var floor int16
	if f <= 0 && (job != 0 || c.FoodPerFarmer > 0) {
		floor = 1
	}
	if f >= floor {
		return f
	}
	return floor
}

// gravityProductionFactor is Gravity_Player_Production_Factor_ (0xddf2c):
// 4 is full output; the gravity penalty is (4 - factor) * 25%.
func (s *State) gravityProductionFactor(c *save.Colony, race int) int16 {
	if bld(c, 0x19) || int16(race) >= 8 {
		return 4
	}
	picks := &s.Players[race].Picks
	switch s.Planets[c.Planet].Gravity {
	case 0:
		if picks[save.PickLowG] != 0 {
			return 4
		}
		return 3
	case 1:
		if picks[save.PickLowG] != 0 {
			return 3
		}
		return 4
	case 2:
		if picks[save.PickHighG] != 0 {
			return 4
		}
		return 2
	}
	// The original returns its caller's EBX; gravity is always 0..2.
	return 4
}

// colonyJobProduction is Colony_Job_Production_ (0xde280): the output of
// job 0 (farmers), 1 or 2, rounded.
func (g *Game) colonyJobProduction(ci, job int) int32 {
	return g.colonyJobProductionB(ci, job, nil)
}

// colonyJobProductionB is Colony_Job_Production_ with its explanation
// record: each colonist's record is summed into acc. A colonist's total
// also takes the record's +0x226 (research specials), so with acc the
// result can exceed the plain one, as in the original.
func (g *Game) colonyJobProductionB(ci, job int, acc *prodBreakdown) int32 {
	s := &g.state
	c := &s.Colonies[ci]
	owner := int(c.Owner)
	p := &s.Players[owner]
	blockaded := maskBit(s.Stars[s.Planets[c.Planet].Star].Blockaded, owner)
	gov := int32(p.Picks[save.PickGovernment])
	moraleCounts := gov/2 != 3
	var lead int16
	if o := s.colonyOfficer(c); o != -1 {
		sk := s.Leaders[o].SpecialSkills
		lvl := s.ownedOfficerLevel(o)
		var hi, lo uint32
		switch job {
		case 0:
			hi, lo = 0x8, 0x4
		case 1:
			hi, lo = 0x200, 0x100
		case 2:
			hi, lo = 0x2000, 0x1000
		}
		switch {
		case sk&hi != 0:
			lead = (lvl + 1) * 3
		case sk&lo != 0:
			lead = (lvl + 1) * 2
		}
	}
	var a prodBreakdown
	for i := int(c.PopCount) - 1; i >= 0; i-- {
		col := c.Colonists[i]
		if col>>9&1 == 0 || int(col>>7&3) != job {
			continue
		}
		race := int(col & 0xf)
		grav := 4 - s.gravityProductionFactor(c, race)
		flag := moraleCounts
		var in prodBreakdown
		var bd *prodBreakdown
		if acc != nil {
			bd = &in
		}
		base := int32(g.colonyPopBaseProd(ci, col, job, bd))
		if acc != nil {
			a.base += in.base
			a.race[race] += in.race[race]
			for k := range a.apps {
				a.apps[k] += in.apps[k]
			}
			for k := range a.bldgs {
				a.bldgs[k] += in.bldgs[k]
			}
			a.aquatic += in.aquatic
			a.special += in.special
			a.intel += in.intel
		}
		in.base20 = base * 20
		a.base20 += in.base20
		if job != 1 && race == 8 {
			flag = false
		}
		in.grav = int32(grav) * base * -5
		a.grav += in.grav
		in.lead = base * int32(lead)
		a.lead += in.lead
		if col>>10&1 != 0 {
			in.conq = base * -5
			a.conq += in.conq
		}
		if flag {
			in.morale = base * int32(c.Morale)
		}
		a.morale += in.morale
		switch {
		case uint16(job) <= 1:
			switch uint32(gov) {
			case 6:
				in.gov = base * 10
			case 7:
				in.gov = base * 20
			}
			a.gov += in.gov
			if blockaded {
				in.block = base * -10
				a.block += in.block
			}
		case job == 2:
			switch uint32(gov) {
			case 0:
				in.gov = base * -10
			case 1:
				in.gov = base * -5
			case 4:
				in.gov = base * 10
			case 5:
				in.gov = base * 15
			}
			a.gov += in.gov
		}
		if p.Objective != save.ObjectiveHuman {
			in.ai = int32(int8(g.exeU8(tblAIJobBonus + uint32(g.settings.Difficulty()))))
			a.ai += in.ai
		}
		in.total = in.base20 + in.grav + in.lead + in.conq + in.morale + in.pollute + in.gov + in.block + in.special + in.ai
		if in.total > 0 {
			a.total += in.total
		}
	}
	if acc != nil {
		*acc = a
	}
	if job == 0 {
		return (a.total + 20) / 40
	}
	return (a.total + 10) / 20
}

// colonyFoodProduction is Colony_Food_Production_ (0xde664) without the
// explanation text (which alone adds the replicated food).
func (g *Game) colonyFoodProduction(ci int) { g.colonyFoodProductionB(ci, nil) }

// foodTerms are Colony_Food_Production_'s building terms.
type foodTerms struct {
	anomaly  bool
	b15, b2b int32
}

func (g *Game) colonyFoodProductionB(ci int, bd *prodBreakdown) foodTerms {
	s := &g.state
	c := &s.Colonies[ci]
	var t foodTerms
	if c.PopCount == 0 {
		c.Output[food] = 0
		return t
	}
	if s.eventCheckSpaceAnomaly(ci) {
		c.Output[food] = 0
		t.anomaly = true
		return t
	}
	var v int32
	if c.FoodPerFarmer != 0 {
		v = g.colonyJobProductionB(ci, 0, bd)
	} else if bd != nil {
		*bd = prodBreakdown{}
	}
	if bld(c, 0x15) {
		t.b15 = 2
	}
	if bld(c, 0x2b) {
		t.b2b = 4
	}
	c.Output[food] = int16(v + t.b15 + t.b2b)
	return t
}

// colonyIndustryProduction is Colony_Industry_Production_ (0xdee1b)
// without the explanation text; it also sets pollution.
func (g *Game) colonyIndustryProduction(ci int) { g.colonyIndustryProductionB(ci, nil) }

// industryTerms are Colony_Industry_Production_'s terms besides the job.
type industryTerms struct {
	anomaly                 bool
	b7, b24, bc, deep, recy int32
}

func (g *Game) colonyIndustryProductionB(ci int, bd *prodBreakdown) industryTerms {
	s := &g.state
	c := &s.Colonies[ci]
	n := int32(c.PopCount)
	c.Recyclotron = 0
	var t industryTerms
	if n == 0 {
		c.Output[industry] = 0
		return t
	}
	if s.eventCheckSpaceAnomaly(ci) {
		c.Output[industry] = 0
		t.anomaly = true
		return t
	}
	pl := &s.Planets[c.Planet]
	job := int32(int16(g.colonyJobProductionB(ci, 1, bd)))
	if bld(c, 7) {
		t.b7 = 5
	}
	if bld(c, 0x24) {
		t.b24 = 10
	}
	if bld(c, 0xc) {
		t.bc = 15
	}
	if bld(c, 0x22) {
		t.deep = int32(g.exeU8(tblDeepCoreMinerals + uint32(pl.Minerals)))
	}
	if bld(c, 0x21) {
		t.recy = n
		c.Recyclotron = uint8(t.recy)
	}
	owner := &s.Players[c.Owner]
	if owner.Objective != save.ObjectiveHuman {
		c.Recyclotron += uint8((int32(g.settings.Difficulty())*n + 4) / 8)
	}
	total := t.b7 + job + t.b24 + t.bc + t.deep + t.recy
	if bld(c, 0xd) {
		c.Pollution = 0
		c.Output[industry] = int16(total)
		return t
	}
	off := s.colonyOfficer(c)
	tolerance := int32(g.exeU8(tblPollutionSize + uint32(pl.Size)))
	poll := job + t.deep
	if knows(owner, 0x71) {
		tolerance *= 2
	}
	if bld(c, 0x20) {
		poll = (poll + 1) / 2
	}
	if bld(c, 5) {
		poll = (poll + 3) / 4
	}
	if off != -1 {
		var pct int32
		switch sk := s.Leaders[off].SpecialSkills; {
		case sk&2 != 0:
			pct = (int32(s.ownedOfficerLevel(off)) + 1) * 15
		case sk&1 != 0:
			pct = (int32(s.ownedOfficerLevel(off)) + 1) * 10
		}
		poll -= pct * poll / 100
	}
	poll = (poll - tolerance + 1) / 2
	if poll > 0 {
		var tol int32
		for i := int(n) - 1; i >= 0; i-- {
			race := int(c.Colonists[i] & 0xf)
			if (race < 8 && s.Players[race].Picks[save.PickTolerant] != 0) || race == 8 {
				tol++
			}
		}
		if tol != 0 {
			poll = (poll*(n-tol) + n/2) / n
		}
	}
	if poll > 0 {
		if bd != nil {
			bd.pollute = poll * -20
		}
		total -= poll
	} else {
		poll = 0
	}
	c.Pollution = int16(poll)
	c.Output[industry] = int16(total)
	return t
}

// colonyReplicators is Colony_Replicators_ (0xdf66f): with building 0x10,
// spare industry turns into food for unfed colonists, group by group.
func colonyReplicators(c *save.Colony) {
	c.ReplicatedFood = 0
	if !bld(c, 0x10) {
		return
	}
	half := func(v int16) int16 { return int16(int32(v) / 2) }
	fd, id := &c.FoodDemand, &c.IndustryDem
	ind := c.Output[industry] * 2
	fod := c.Output[food] * 2
	var used int16
	// step converts up to avail (already halved) industry into food for
	// the demand not yet met; it reports whether there was industry left.
	step := func(avail int16, demand int16) bool {
		if avail <= 0 {
			return false
		}
		f := int16((int32(int16(demand-fod)) + 1) / 2)
		if f > 0 {
			t := min(int32(f)*2, int32(avail)) / 2 * 2
			used += int16(t)
			fod += int16(t)
			ind = int16(int32(ind) - t/2)
		}
		return true
	}
	ok := step(half(ind-int16(id[0])), int16(fd[0])) &&
		step(half(ind-int16(id[0])-int16(id[1])-int16(id[2])), int16(fd[0])+int16(fd[1])) &&
		step(half(ind-int16(id[0])-int16(id[1])-int16(id[2])-int16(id[3])), int16(fd[0])+int16(fd[1])+int16(fd[2]))
	if ok {
		step(half(ind-int16(id[0])-int16(id[1])-int16(id[2])-int16(id[3])), int16(fd[1])+int16(fd[0])+int16(fd[2])+int16(fd[3]))
	}
	c.ReplicatedFood = uint8(half(used))
	c.Output[industry] -= used
	c.Output[food] += half(used)
}

// colonyResearchProduction is Colony_Research_Production_ (0xdff74)
// without the explanation text.
func (g *Game) colonyResearchProduction(ci int) { g.colonyResearchProductionB(ci, nil) }

// researchTerms are Colony_Research_Production_'s building terms.
type researchTerms struct {
	anomaly           bool
	b23, b1e, b13, b6 int32
}

func (g *Game) colonyResearchProductionB(ci int, bd *prodBreakdown) researchTerms {
	s := &g.state
	c := &s.Colonies[ci]
	var t researchTerms
	if c.PopCount == 0 {
		c.Output[research] = 0
		return t
	}
	if s.eventCheckSpaceAnomaly(ci) {
		c.Output[research] = 0
		t.anomaly = true
		return t
	}
	v := int32(int16(g.colonyJobProductionB(ci, 2, bd)))
	if bld(c, 0x23) {
		t.b23 = 5
	}
	if bld(c, 0x1e) {
		t.b1e = 10
	}
	if bld(c, 0x13) {
		t.b13 = 15
	}
	if bld(c, 6) {
		t.b6 = 30
	}
	c.Output[research] = int16(v + t.b23 + t.b1e + t.b13 + t.b6)
	return t
}

// colonyIndustryToTax is Colony_Industry_To_Tax_ (0xe08f6).
func (s *State) colonyIndustryToTax(c *save.Colony) {
	x := int32(c.Output[industry]) - int32(c.Upkeep[industry])
	if x <= 0 {
		c.IndustryTaxed = 0
		return
	}
	t := int16(int32(s.Players[c.Owner].TaxRate) * x / 100)
	c.IndustryTaxed = t
	c.Output[industry] -= t
}

// colonyBCProduction is Colony_BC_Production_ (0xe03f1) without the
// explanation text.
func (g *Game) colonyBCProduction(ci int) { g.colonyBCProductionT(ci) }

// bcTerms are Colony_BC_Production_'s terms, which its explanation lists.
type bcTerms struct {
	anomaly                                                bool
	base, morale, special, trade, app, b1d, b27, gov, lead int32
}

func (g *Game) colonyBCProductionT(ci int) bcTerms {
	s := &g.state
	c := &s.Colonies[ci]
	var t bcTerms
	if c.PopCount == 0 {
		c.Output[bc] = 0
		return t
	}
	if s.eventCheckSpaceAnomaly(ci) {
		c.Output[bc] = 0
		t.anomaly = true
		return t
	}
	p := &s.Players[c.Owner]
	off := s.colonyOfficer(c)
	perPop := int32(p.Picks[save.PickMoney])*2 + 4
	var n int32
	for i := int(c.PopCount) - 1; i >= 0; i-- {
		if c.Colonists[i]&0xf < 8 {
			n++
		}
	}
	if p.Objective != save.ObjectiveHuman {
		perPop += int32(g.exeU8(tblAIBCBonus + uint32(g.settings.Difficulty())))
	}
	base := (n*perPop + 2) / 4
	var morale int32
	if n > 0 && int32(p.Picks[save.PickGovernment])/2 != 3 {
		if m := int32(c.Morale) * base; m >= 0 {
			morale = (m + 10) / 20
		} else {
			morale = -((-m + 10) / 20)
		}
	}
	var special int32
	switch s.Planets[c.Planet].Special {
	case 4:
		special = 5
	case 5:
		special = 10
	}
	var trade int32
	switch uint16(c.Queue[0]) {
	case 0xffdb, 0xfffe, 0xffff:
		if x := int32(c.Output[industry]) - int32(c.Upkeep[industry]); x > 0 {
			if p.Picks[save.PickFantasticTraders] != 0 {
				trade = x
			} else {
				trade = (x + 1) / 2
			}
		}
	}
	b := base + special
	var app, b1d, b27, gov, lead int32
	if knows(p, 0x4b) {
		app = b / 2
	}
	if bld(c, 0x1d) {
		b1d = b
	}
	if bld(c, 0x27) {
		b27 = b / 2
	}
	switch uint8(p.Picks[save.PickGovernment]) {
	case 4:
		gov = b / 2
	case 5:
		gov = b * 3 / 4
	}
	if off != -1 {
		sk := s.Leaders[off].SpecialSkills
		switch {
		case sk&0x20 != 0:
			lead = ((int32(s.ownedOfficerLevel(off))+1)*b*3 + 10) / 20
		case sk&0x10 != 0:
			lead = ((int32(s.ownedOfficerLevel(off))+1)*b + 5) / 10
		}
	}
	v := base + morale + special + trade + app + b1d + b27 + gov
	c.Output[bc] = int16(lead + int32(int16(v)+c.IndustryTaxed))
	return bcTerms{base: base, morale: morale, special: special, trade: trade, app: app, b1d: b1d, b27: b27, gov: gov, lead: lead}
}

// colonyBCMaintenance is Colony_BC_Maintenance_ (0xe094f).
func (g *Game) colonyBCMaintenance(ci int) {
	s := &g.state
	c := &s.Colonies[ci]
	switch {
	case c.PopCount == 0, s.eventCheckSpaceAnomaly(ci):
		c.Upkeep[bc] = 0
	default:
		var sum int32
		for id := 0; id < 0x31; id++ {
			if bld(c, id) {
				sum += int32(g.exeI16(uint32(tblBuildingUpkeep + id*tblBuildingUpkeepRow)))
			}
		}
		mod := int32(g.exeU8(tblClimateMaint+uint32(c.Climate))) + 100
		c.Upkeep[bc] = uint8(((sum*100+int32(c.ReplicatedFood)*100)*mod + 5000) / 10000)
	}
	c.BCDeficit = int16(c.Upkeep[bc]) - c.Output[bc]
}

// preImportComputing is Pre_Import_Computing_ (0xe1d59): everything about
// a colony that does not depend on food shipped in.
func (g *Game) preImportComputing(ci int) {
	s := &g.state
	c := &s.Colonies[ci]
	c.Upkeep = [4]uint8{}
	c.FoodPerFarmer = uint8(s.colonyFood2PerFarmer(c, int(c.Owner)))
	s.colonyEnvironmentalStuff(c)
	c.IndustryPer = uint8(g.colonyIndustryPerWorker(c, int(c.Owner)))
	c.ResearchPer = uint8(s.colonyResearchPerScientist(c))
	g.colonyMorale(c)
	s.colonyIndustryMaintenance(ci)
	g.colonyIndustryProduction(ci)
	g.colonyFoodProduction(ci)
	s.colonyFoodMaintenance(ci)
	colonyReplicators(c)
	c.IndustryPaid = c.Output[industry]
	if int16(c.Upkeep[industry]) < c.Output[industry] {
		c.IndustryPaid = int16(c.Upkeep[industry])
	}
	g.colonyResearchProduction(ci)
	s.colonyIndustryToTax(c)
	g.colonyBCProduction(ci)
	g.colonyBCMaintenance(ci)
}

// passOutImports is Pass_Out_Imports_ (0xdf8f0): freighters carry surplus
// food to starving colonies; colony >= 0 also regrows the fed ones and it.
func (g *Game) passOutImports(p int, colony int) {
	s := &g.state
	pl := &s.Players[p]
	free := pl.FreightersTotal - pl.SettlersCount*5
	var list [250]int16
	n := int16(0)
	var need, surplus int16
	for i := 0; i < int(s.NumColonies); i++ {
		c := &s.Colonies[i]
		if int16(c.Owner) != int16(p) || c.IsOutpost != 0 || s.eventCheckSpaceAnomaly(i) {
			continue
		}
		x := c.Output[food] - int16(c.Upkeep[food])
		blockaded := maskBit(s.Stars[s.Planets[c.Planet].Star].Blockaded, int(c.Owner))
		if x < 0 {
			c.FoodImport = 0
			if !blockaded {
				list[n] = int16(i)
				n++
				need -= x
			}
		} else {
			c.FoodImport = -x
			if !blockaded {
				surplus += x
			}
		}
	}
	pl.FoodSurplus = surplus - need
	if surplus >= need && free >= need {
		for k := int16(0); k < n; k++ {
			c := &s.Colonies[list[k]]
			c.FoodImport = int16(c.Upkeep[food]) - c.Output[food]
		}
		pl.FoodShipped = need
		pl.FreightersFree = free - need
	} else {
		free0, surplus0 := free, surplus
		// Each pass feeds one food at a time round robin until no colony
		// below the threshold is left or the food or freighters run out.
		pass := func(below func(c *save.Colony) bool) {
			k, fails := int16(0), int16(0)
			for free > 0 && fails < n {
				c := &s.Colonies[list[k]]
				if surplus > 0 && below(c) {
					fails = 0
					surplus--
					free--
					c.FoodImport++
				} else {
					fails++
				}
				k++
				if k == n {
					k = 0
				}
			}
		}
		got := func(c *save.Colony) int32 { return (int32(c.FoodImport) + int32(c.Output[food])) * 2 }
		pass(func(c *save.Colony) bool { return got(c) < int32(c.FoodDemand[0]) })
		pass(func(c *save.Colony) bool { return got(c) < int32(c.FoodDemand[0])+int32(c.FoodDemand[1]) })
		pass(func(c *save.Colony) bool {
			return got(c) < int32(c.FoodDemand[0])+int32(c.FoodDemand[1])+int32(c.FoodDemand[2])
		})
		pass(func(c *save.Colony) bool { return int32(c.FoodImport)+int32(c.Output[food]) < int32(c.Upkeep[food]) })
		pl.FoodShipped = surplus0 - surplus
		if free == 0 {
			m := need
			if need >= surplus0 {
				m = surplus0
			}
			pl.FreightersFree = -(m - pl.FoodShipped)
		} else {
			pl.FreightersFree = free0 - pl.FoodShipped
		}
	}
	if colony < 0 {
		return
	}
	done := false
	for k := int16(0); k < n; k++ {
		g.colonyPopGrows(int(list[k]))
		colonySpecialty(&s.Colonies[list[k]])
		if int(list[k]) == colony {
			done = true
		}
	}
	if !done {
		g.colonyPopGrows(colony)
		colonySpecialty(&s.Colonies[colony])
	}
}

// colonyPopGrows is Colony_Pop_Grows_ (0xe1839): growth per race in 1/1000
// colonist, with the patch at 0x16946e.
func (g *Game) colonyPopGrows(ci int) {
	s := &g.state
	c := &s.Colonies[ci]
	n := int32(c.PopCount)
	if n == 0 {
		c.PopGrowth = [10]int16{}
		c.PopFraction = [10]int16{}
		return
	}
	if s.eventCheckSpaceAnomaly(ci) {
		c.PopGrowth = [10]int16{}
		return
	}
	boom := s.eventCheckPopulationBoom(ci)
	plague := s.eventCheckPlague(ci)
	fod := (int32(c.Output[food]) + int32(c.FoodImport)) * 2
	f0, f1, f2, f3 := int32(c.FoodDemand[0]), int32(c.FoodDemand[1]), int32(c.FoodDemand[2]), int32(c.FoodDemand[3])
	i0, i1, i2, i3 := int32(c.IndustryDem[0]), int32(c.IndustryDem[1]), int32(c.IndustryDem[2]), int32(c.IndustryDem[3])
	ind := int32(c.Output[industry]) * 2
	for _, d := range []*int32{&f0, &f1, &f2, &f3} {
		for fod > 0 && *d > 0 {
			*d--
			fod--
		}
	}
	for ind > 0 && i0 > 0 {
		ind--
		i0--
	}
	for ind > 1 && i1 > 1 {
		ind -= 2
		i1 -= 2
	}
	for ind > 0 && i2 > 0 {
		i2--
		ind--
	}
	for ind > 0 && i3 > 0 {
		i3--
		ind--
	}
	c.PopGrowth = [10]int16{}
	if own := i0 + f0; own > 0 {
		c.PopGrowth[c.Owner] = int16(own * -25)
	} else {
		c.PopGrowth[c.Owner] = 0
	}
	if i1 > 0 {
		c.PopGrowth[8] = int16(i1 * -500)
	} else {
		c.PopGrowth[8] = 0
	}
	if t := f2 + f1 + i3 + i2; t > 0 {
		d := t * -25
		div := int32(c.FoodDemand[2]) + int32(c.FoodDemand[1]) + int32(c.IndustryDem[3]) + int32(c.IndustryDem[2])
		for b := 0; b < int(s.NumPlayers); b++ {
			if b == int(c.Owner) {
				continue
			}
			v := (int32(c.FoodByRace[b]) + int32(c.IndustryByRace[b])) * d / div
			c.PopGrowth[b] = int16(int32(c.PopGrowth[b]) + v)
		}
	} else {
		for b := 0; b < int(s.NumPlayers); b++ {
			if b != int(c.Owner) {
				c.PopGrowth[b] = 0
			}
		}
	}
	if f3 > 0 {
		c.PopGrowth[9] = int16(f3 * -25)
	} else {
		c.PopGrowth[9] = 0
	}
	var cnt, conq [10]uint8
	for i := int(n) - 1; i >= 0; i-- {
		col := c.Colonists[i]
		cnt[col&0xf]++
		if col>>10&1 != 0 {
			conq[col&0xf]++
		}
	}
	owner := &s.Players[c.Owner]
	for b := int(s.NumPlayers) - 1; b >= 0; b-- {
		if cnt[b] == 0 {
			continue
		}
		var gr int32
		lim := int32(int16(g.colonyRacePopLimit(c, b)))
		if lim > n {
			off := s.colonyOfficer(c)
			rate := int32(s.Players[b].Picks[save.PickPopulation]) + 100
			var extra int32
			if s.Stars[0].Name[14]&8 != 0 {
				rate += 150
			}
			if knows(owner, 0xc1) {
				rate += 50
			} else if knows(owner, 0x6b) {
				rate += 25
			}
			if off != -1 {
				sk := byte(s.Leaders[off].SpecialSkills >> 8)
				switch {
				case sk&8 != 0:
					rate += (int32(s.ownedOfficerLevel(off)) + 1) * 15
				case sk&4 != 0:
					rate += (int32(s.ownedOfficerLevel(off)) + 1) * 10
				}
			}
			if bld(c, 0xa) {
				extra += int32(cnt[b]) * 100 / n
			}
			if boom {
				rate += 100
			}
			if plague {
				rate -= 200
			}
			if c.Queue[0] == save.QueueHousing {
				rate += (int32(c.Output[industry]) - int32(c.Upkeep[industry])) * 40 / n
			}
			if b != int(c.Owner) && conq[b] > 0 {
				if rate > 0 && c.Occupation == 0 {
					rate = (int32(cnt[b]) - int32(conq[b])) * rate / int32(cnt[b])
				}
			} else if owner.Objective != save.ObjectiveHuman {
				rate += int32(g.settings.Difficulty())
			}
			x := (lim - n) * int32(cnt[b]) * 2000 / lim
			gr = int32(isqrt(uint32(x)))*rate/100 + extra
		}
		c.PopGrowth[b] = int16(int32(c.PopGrowth[b]) + gr)
	}
}

// colonySpecialty is Colony_Specialty_ (0xe1e1f): the output that is more
// than half of food + industry + research, or -1.
func colonySpecialty(c *save.Colony) {
	var sum, best int16
	which := int16(0)
	for k := int16(0); k < 3; k++ {
		v := c.Output[k]
		sum += v
		if best < v {
			which, best = k, v
		}
	}
	if int32(sum)/2 < int32(best) {
		c.Specialty = int8(which)
	} else {
		c.Specialty = -1
	}
}

// leaderAtAnomaly is Leader_At_Anomaly_ (0xe1f9e).
func (s *State) leaderAtAnomaly(o int) bool {
	l := &s.Leaders[o]
	return l.Status == 1 && l.ETA == 0 && l.Type == 1 && s.eventCheckSpaceAnomalyAtStar(l.Location)
}

// tributeReceivedFrom is Tribute_Received_From_ (0xe1fc7): what player k
// pays p this turn.
func (s *State) tributeReceivedFrom(p, k int) int32 {
	o := &s.Players[k]
	v := int32(o.Tribute[p]) * (int32(o.GrossIncome) - o.TributeReceived) / 20
	return max(v, 0)
}

// officerSkillBonus is Officer_Skill_Bonus_ (0x94bb2).
func (g *Game) officerSkillBonus(o, skill int) int32 {
	s := &g.state
	l := &s.Leaders[o]
	var lvl int32
	if l.Owner == -1 {
		lvl = int32(min(officerBaseLevel(l.Experience, false), 5))
	} else {
		lvl = int32(s.ownedOfficerLevel(o))
	}
	row := uint32(tblSkillData + skill*tblSkillDataRow)
	div := int32(g.exeU8(row + 8))
	mul := int32(g.exeI16(row + 6))
	return int32(int16((lvl+1+div-1)/div)) * mul / 10
}

func (g *Game) leaderHasSkill(o, skill int) bool {
	mask := g.exe.u32(uint32(tblSkillData + skill*tblSkillDataRow + 1))
	return g.state.Leaders[o].CommonSkills&mask != 0
}

// bonusForFamousHeroes is Bonus_For_Famous_Heroes_ (0x9488f). The skill
// bonus is not reset between leaders, as in the original.
func (g *Game) bonusForFamousHeroes(p int) int16 {
	s := &g.state
	var best, v int16
	for o := 0; o < maxLeaders; o++ {
		l := &s.Leaders[o]
		if int16(l.Owner) != int16(p) || (l.Status != 0 && l.Status != 1) {
			continue
		}
		switch {
		case g.leaderHasSkill(o, 0xa):
			v = int16(g.officerSkillBonus(o, 0xa))
		case g.leaderHasSkill(o, 0xb):
			v = int16(g.officerSkillBonus(o, 0xb))
		}
		if best < v {
			best = v
		}
	}
	return best
}

// officerCost is Officer_Cost_ (0x97a2d).
func (g *Game) officerCost(p, o int) int16 {
	l := &g.state.Leaders[o]
	lvl := min(officerBaseLevel(l.Experience, false), 5)
	c := uint16(int32(l.SkillValue) * (int32(uint8(lvl)) + 1) * 10)
	famous := g.bonusForFamousHeroes(p)
	if g.leaderHasSkill(o, 0x1c) || g.leaderHasSkill(o, 0x1d) {
		c *= 2
	}
	if r := int32(int16(c)) - int32(famous); r > 0 {
		return int16(r)
	}
	return 0
}

// officerMaintenance is Officer_Maintenance_ (0x94a9d).
func (g *Game) officerMaintenance(p, o int) int16 {
	m := (int32(g.officerCost(p, o)) + 99) / 100
	if g.leaderHasSkill(o, 0x1c) || g.leaderHasSkill(o, 0x1d) || o == 0x41 {
		return 0
	}
	return int16(max(int16(m), 1))
}

// computePlayerMaintenance is Compute_Player_Maintenance_ (0xe2000) without
// the explanation text: command points and the maintenance parts.
func (g *Game) computePlayerMaintenance(p int) { g.computePlayerMaintenanceText(p, nil) }

// computePlayerMaintenanceText is Compute_Player_Maintenance_ with its text
// buffer: the command point explanation goes to buf when not nil.
func (g *Game) computePlayerMaintenanceText(p int, buf *strings.Builder) {
	s := &g.state
	pl := &s.Players[p]
	pl.CommandTotal, pl.CommandUsed = 0, 0
	pl.MaintenanceParts = [6]int16{}
	var best int32
	bestOfficer := -1
	for o := maxLeaders - 1; o >= 0; o-- {
		l := &s.Leaders[o]
		if int16(l.Owner) != int16(p) || uint8(l.Status) >= 3 || s.leaderAtAnomaly(o) {
			continue
		}
		pl.MaintenanceParts[5] += g.officerMaintenance(p, o)
		var cp int32
		sk := byte(l.CommonSkills >> 8)
		switch {
		case sk&8 != 0:
			cp = (int32(s.ownedOfficerLevel(o)) + 1) * 3
		case sk&4 != 0:
			cp = int32(s.ownedOfficerLevel(o))*2 + 2
		}
		if cp > best {
			best, bestOfficer = cp, o
		}
	}
	var sb, bs, sf, warlord int32
	for i := int(s.NumColonies) - 1; i >= 0; i-- {
		c := &s.Colonies[i]
		if int16(c.Owner) != int16(p) || c.IsOutpost != 0 {
			continue
		}
		pl.MaintenanceParts[0] += int16(c.Upkeep[bc])
		if pl.Picks[save.PickWarlord] != 0 {
			warlord += 2
		}
		switch {
		case bld(c, 0x28):
			sb++
		case bld(c, 8):
			bs++
		case bld(c, 0x29):
			sf++
		}
	}
	var comm int32
	commApp := -1
	switch {
	case knows(pl, 0x5b):
		comm, commApp = 3, 0x5b
	case knows(pl, 0xb0):
		comm, commApp = 2, 0xb0
	case knows(pl, 0xb4):
		comm, commApp = 1, 0xb4
	}
	comm *= bs + sb + sf
	pl.CommandTotal = int16(sb + 5 + bs*2 + sf*3 + comm + best + warlord)
	var half int32
	if pl.Picks[save.PickGovernment] == 3 {
		half = int32(pl.CommandTotal) / 2
		pl.CommandTotal += int16(half)
	}
	if buf != nil {
		g.commandPointsSourcesText(buf, sb, bs, sf, comm, commApp, best, bestOfficer, warlord, half, pl.CommandTotal)
	}
	if pl.FreightersFree > 0 {
		pl.MaintenanceParts[1] = int16((int32(pl.FreightersTotal) - int32(pl.FreightersFree)) / 2)
	} else {
		pl.MaintenanceParts[1] = int16(int32(pl.FreightersTotal) / 2)
	}
	var bySize [9]int32
	var support int32
	for i := 0; i < int(s.NumShips); i++ {
		sh := &s.Ships[i]
		if int16(sh.Owner) != int16(p) || int8(sh.Status) > 2 || sh.JustBuilt != 0 {
			continue
		}
		if sh.Design.Type == 0 {
			bySize[sh.Design.Size]++
		} else {
			support++
		}
		pl.CommandUsed += int16(sh.Design.Size) + 1
	}
	if buf != nil {
		g.commandPointsUsedText(buf, &bySize, support, pl.CommandUsed)
	}
	switch {
	case pl.CommandUsed == 0:
		pl.MaintenanceParts[2] = 0
	case pl.CommandUsed <= pl.CommandTotal:
		pl.MaintenanceParts[2] = 0
		if buf != nil {
			buf.WriteString(cSprintf(cStringAt(g.exe, cpmNoPenalty), g.cpmString(58)))
		}
	default:
		mul := int32(10)
		if pl.Objective != save.ObjectiveHuman {
			mul = 12 - int32(g.settings.Difficulty())
		}
		diff := int32(pl.CommandUsed) - int32(pl.CommandTotal)
		pl.MaintenanceParts[2] = int16(mul * diff)
		if buf != nil {
			buf.WriteString(cSprintf(cStringAt(g.exe, cpmPenalty), g.cpmString(58), 0, int32(pl.CommandTotal), mul, diff, int32(pl.MaintenanceParts[2])))
		}
	}
	for k := 0; k < int(s.NumPlayers); k++ {
		pl.MaintenanceParts[3] += int16(pl.Spies[k] & 0x3f)
	}
	pl.MaintenanceParts[4] = 0
	for k := int(s.NumPlayers) - 1; k >= 0; k-- {
		if pl.Tribute[k] != 0 {
			pl.MaintenanceParts[4] += int16(s.tributeReceivedFrom(k, p))
		}
	}
	pl.Maintenance = 0
	for k := 5; k >= 0; k-- {
		pl.Maintenance += int32(pl.MaintenanceParts[k])
	}
}

// updatePlayerStats is Update_Player_Stats_ (0xe2710): the empire totals of
// the player record.
func (g *Game) updatePlayerStats(p int) {
	s := &g.state
	pl := &s.Players[p]
	var pop, fod, fodFree, upkeepFree, ind, res, gross int32
	for i := int(s.NumColonies) - 1; i >= 0; i-- {
		c := &s.Colonies[i]
		if int32(c.Owner) != int32(p) || c.IsOutpost != 0 {
			continue
		}
		pop += int32(c.PopCount)
		fod += int32(c.Output[food])
		if !maskBit(s.Stars[s.Planets[c.Planet].Star].Blockaded, int(c.Owner)) {
			fodFree += int32(c.Output[food])
			upkeepFree += int32(c.Upkeep[food])
		}
		ind += int32(c.Output[industry])
		if !s.eventCheckColonyResearching(i) {
			res += int32(c.Output[research])
		}
		gross += int32(c.Output[bc])
	}
	for i := int(s.NumShips) - 1; i >= 0; i-- {
		sh := &s.Ships[i]
		if int32(sh.Owner) == int32(p) && testBit(sh.Design.Specials[:], 0x1c) && int8(sh.Status) < 4 {
			res += 1 << sh.Design.Size
		}
	}
	for o := maxLeaders - 1; o >= 0; o-- {
		l := &s.Leaders[o]
		if int32(l.Owner) != int32(p) || (l.Status != 0 && l.Status != 1) || s.leaderAtAnomaly(o) {
			continue
		}
		sk := byte(l.CommonSkills >> 8)
		switch {
		case sk&2 != 0:
			gross += 15
		case sk&1 != 0:
			gross += 10
		}
		switch {
		case sk&0x20 != 0:
			res += (int32(s.ownedOfficerLevel(o)) + 1) * 75 / 10
		case sk&0x10 != 0:
			res += (int32(s.ownedOfficerLevel(o)) + 1) * 5
		}
	}
	pl.TributeReceived = 0
	for k := int(s.NumPlayers) - 1; k >= 0; k-- {
		if pl.TradeTreaty[k] != 0 {
			gross += int32(pl.TradeValue[k])
		}
		if pl.ResearchTreaty[k] != 0 {
			if v := pl.ResearchValue[k]; v >= 0 {
				res += int32(v)
			} else {
				gross += int32(v)
			}
		}
		if s.Players[k].Tribute[p] != 0 {
			pl.TributeReceived += s.tributeReceivedFrom(p, k)
		}
	}
	pl.Population = int16(pop)
	pl.FoodProduced = int16(fod)
	gross += pl.TributeReceived
	pl.IndustryProduced = int16(ind)
	pl.ResearchProduced = int16(min(res, 0x7fff))
	pl.FoodSurplus = int16(fodFree - upkeepFree)
	if v := pl.FoodSurplus; v > 0 {
		if pl.Picks[save.PickFantasticTraders] == 0 {
			v /= 2
		}
		gross += int32(v)
	}
	gross = min(gross, 0x7fff)
	pl.GrossIncome = int16(gross)
	g.computePlayerMaintenance(p)
	pl.NetIncome = int16(gross) - int16(pl.Maintenance)
}

// doPlayerColonyPostProduction is Do_Player_Colony_Post_Production_
// (0xe2d09).
func (g *Game) doPlayerColonyPostProduction(p int) {
	s := &g.state
	g.passOutImports(p, -1)
	for i := int(s.NumColonies) - 1; i >= 0; i-- {
		c := &s.Colonies[i]
		if int16(c.Owner) == int16(p) && c.IsOutpost == 0 {
			g.colonyPopGrows(i)
			colonySpecialty(c)
		}
	}
	g.updatePlayerStats(p)
}

// doPlayerColonyCalculations is Do_Player_Colony_Calculations_ (0xe2d72).
// Mox_Update_ (sound and network polling) has nothing to do here.
func (g *Game) doPlayerColonyCalculations(p int) {
	s := &g.state
	for i := int(s.NumColonies) - 1; i >= 0; i-- {
		c := &s.Colonies[i]
		if int16(c.Owner) == int16(p) && c.IsOutpost == 0 {
			g.preImportComputing(i)
		}
	}
	g.doPlayerColonyPostProduction(p)
}
