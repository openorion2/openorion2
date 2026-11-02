package game

import (
	"encoding/binary"

	"github.com/openorion2/openorion2/internal/save"
)

// Rebellions (INVASION.C), step 36 of Next_Turn_Calc_, with the ground
// combat and change of ownership they share with invasions. Spec:
// docs/turn/overview.md, D14.

// Constant tables of ORION2.EXE (ELF addresses).
const (
	rifleTable          = 0x14a88 // 3-byte rows: i16 app, then bonus at +2
	personalShieldTable = 0x14a9a // 3-byte rows
)

// groundBonus is the 0x13-byte bonus record of Compute_Player_Ground_Combat_Bonuses_.
type groundBonus [0x13]byte

func (b *groundBonus) officer() int16 { return int16(binary.LittleEndian.Uint16(b[0x10:])) }

// groundForce is the 0x37-byte side of a ground battle: strength and hits
// per troop class (armor, marines, militia, rebels), the class fighting,
// and the bonus record at +0x24.
type groundForce struct {
	player   int16
	strength [4]int16 // +2
	count    [5]int16 // +0xa, the fifth aliases strength/hits bytes
	hits     [4]uint8 // +0x12
	class    uint8    // +0x16
	damage   uint8    // +0x17
	hurt     int8     // +0x18 class that took damage, -1 none
	lost     int8     // +0x19 class that lost a unit, -1 none
	colony   int16    // +0x1a
	ships    []int16  // +0x1c, +0x20 the invading ships
	enemy    int16    // +0x22
	bonus    groundBonus
}

// bytes is the force's 0x37-byte record; the ship list's pointer (+0x1c)
// is the machine's own address, written as 0 and marked stale by callers.
func (F *groundForce) bytes() []byte {
	b := make([]byte, groundForceSize)
	binary.LittleEndian.PutUint16(b[0:], uint16(F.player))
	for k, v := range F.strength {
		binary.LittleEndian.PutUint16(b[2+2*k:], uint16(v))
	}
	for k, v := range F.count[:4] {
		binary.LittleEndian.PutUint16(b[0xa+2*k:], uint16(v))
	}
	copy(b[0x12:0x16], F.hits[:])
	b[0x16], b[0x17], b[0x18], b[0x19] = F.class, F.damage, byte(F.hurt), byte(F.lost)
	binary.LittleEndian.PutUint16(b[0x1a:], uint16(F.colony))
	binary.LittleEndian.PutUint16(b[0x20:], uint16(len(F.ships)))
	binary.LittleEndian.PutUint16(b[0x22:], uint16(F.enemy))
	copy(b[0x24:], F.bonus[:])
	return b
}

// parseGroundForce reads a force's 0x37-byte record from another machine;
// its ship list stays empty (the pointer is that machine's).
func parseGroundForce(b []byte) groundForce {
	var F groundForce
	F.player = int16(binary.LittleEndian.Uint16(b[0:]))
	for k := range F.strength {
		F.strength[k] = int16(binary.LittleEndian.Uint16(b[2+2*k:]))
	}
	for k := range F.count {
		F.count[k] = int16(binary.LittleEndian.Uint16(b[0xa+2*k:]))
	}
	copy(F.hits[:], b[0x12:0x16])
	F.class, F.damage, F.hurt, F.lost = b[0x16], b[0x17], int8(b[0x18]), int8(b[0x19])
	F.colony = int16(binary.LittleEndian.Uint16(b[0x1a:]))
	F.enemy = int16(binary.LittleEndian.Uint16(b[0x22:]))
	copy(F.bonus[:], b[0x24:0x37])
	return F
}

const groundForceSize = 0x37

// checkAllRebellions is Check_All_Rebellions_ (0xed44a): colonies in
// ascending order, outposts skipped.
func (g *Game) checkAllRebellions() {
	s := &g.state
	for i := 0; i < int(s.NumColonies); i++ {
		if c := &s.Colonies[i]; c.Owner != -1 && c.IsOutpost == 0 {
			g.checkRebellion(i)
		}
	}
}

// checkRebellion is Check_Rebellion_ (0xed260): conquered pops of a living
// race may rise (one Random(1000) per such colony); the rebels fight the
// garrison and on winning take the colony back for their race.
func (g *Game) checkRebellion(ci int) {
	s := &g.state
	c := &s.Colonies[ci]
	if c.Occupation == 4 {
		return
	}
	var counts [8]uint8
	var n int32
	for i := int(c.PopCount) - 1; i >= 0; i-- {
		col := c.Colonists[i]
		if col&0x400 == 0 {
			continue
		}
		r := col & 0xf
		if s.Players[r].Eliminated == 0 {
			n++
			counts[r]++
		}
	}
	if n <= 0 {
		return
	}
	rebel := int32(getWeightedChoiceChar(&s.Random, counts[:s.NumPlayers]))
	chance := n * 10
	if s.Players[c.Owner].Objective == 100 && s.Players[rebel].Objective != 100 {
		chance += int32(g.settings[0xd4])*4 - 8
	}
	if c.Buildings[1] != 0 {
		chance /= 2
	}
	if c.Occupation == 0 {
		chance *= 2
	}
	if int32(s.Random.Random(1000)) > chance {
		return
	}
	rebels := int16(s.Random.Random(uint32(n)))
	var A, B groundForce
	g.getRebellionInfo(int16(rebel), rebels, ci, &A, &B)
	m := colonyMsg(c, 9, uint8(rebel))
	m[5] = uint8(rebels)
	for A.class < 4 && B.class < 4 {
		g.groundCombatRound(&A, &B)
	}
	m[6] = uint8(rebels) - uint8(A.count[3])
	g.addMsg(int(c.Owner), m)
	g.addMsg(int(int16(rebel)), m)
	if A.class >= 4 {
		return
	}
	g.changeColonyOwnership(ci, int(A.player), nil)
	g.resolveRebellionTroops(&A)
	c.LastBuilt = -41
}

// getRebellionInfo is Get_Rebellion_Info_ (0xec65a): the rebels (class 3)
// against the colony's armor, marines and militia, with their bonuses.
func (g *Game) getRebellionInfo(rebel, n int16, ci int, A, B *groundForce) {
	s := &g.state
	c := &s.Colonies[ci]
	*A = groundForce{player: rebel, enemy: int16(c.Owner), colony: int16(ci)}
	A.count[3] = n
	*B = groundForce{player: int16(c.Owner), enemy: A.player, colony: int16(ci)}
	B.count[0] = c.Armor
	B.count[1] = c.Marines
	B.count[2] = int16(colonyNMilitia(c))
	g.computePlayerGroundCombatBonuses(int(A.player), &A.bonus, false)
	g.computePlayerGroundCombatBonuses(int(A.enemy), &B.bonus, true)
	g.computeColonyOfficerBonus(c, &B.bonus)
	computeGroundCombatInfo(A, &A.bonus, &B.bonus)
	computeGroundCombatInfo(B, &B.bonus, &A.bonus)
}

// colonyNMilitia is Colony_N_Militia_ (0xec61e): one militia per five
// unconquered pops of the players' races.
func colonyNMilitia(c *save.Colony) int32 {
	var n int32
	for i := int(c.PopCount) - 1; i >= 0; i-- {
		if col := c.Colonists[i]; col&0xf < 8 && col&0x400 == 0 {
			n++
		}
	}
	return n / 5
}

// computePlayerGroundCombatBonuses is Compute_Player_Ground_Combat_Bonuses_
// (0xec15c); players 8 and up get fixed values.
func (g *Game) computePlayerGroundCombatBonuses(p int, b *groundBonus, defender bool) {
	*b = groundBonus{}
	if p >= 8 {
		b[0], b[1], b[2], b[3], b[4] = 10, 10, 1, 10, 1
		b[5], b[6] = g.exeU8(0x17f698), g.exeU8(0x17f69b)
		b[9], b[0xa] = g.exeU8(0x14a97), g.exeU8(0x14a99)
		b[7], b[8] = g.exeU8(0x14a9d), g.exeU8(0x14a9f)
		b[0xf] = g.settings[0xd4]*2 - 4
		return
	}
	pl := &g.state.Players[p]
	if pl.AppStatus[9] == techKnown {
		b[0] = 10
	}
	if pl.AppStatus[0x18] == techKnown {
		b[1], b[2] = 10, 1
	}
	if pl.AppStatus[0x90] == techKnown {
		b[3], b[4] = 10, 1
	}
	a := uint32(g.playerBestFromTable(p, 6, armorTable+armorTech, armorStride)) * armorStride
	b[5], b[6] = g.exeU8(armorTable+a), g.exeU8(armorTable+a+3)
	r := uint32(g.playerBestFromTable(p, 5, rifleTable, 3)) * 3
	b[9], b[0xa] = g.exeU8(rifleTable+r), g.exeU8(rifleTable+r+2)
	sh := uint32(g.playerBestFromTable(p, 1, personalShieldTable, 3)) * 3
	b[7], b[8] = g.exeU8(personalShieldTable+sh), g.exeU8(personalShieldTable+sh+2)
	b[0xb] = uint8(pl.Picks[save.PickGroundCombat])
	if pl.Picks[save.PickHighG] != 0 {
		b[0xc] = 1
	} else if pl.Picks[save.PickLowG] != 0 {
		b[0xd] = 0xf6
	}
	if pl.Picks[save.PickSubterranean] != 0 && defender {
		b[0xe] = 10
	}
	if pl.Objective != 100 {
		b[0xf] = g.settings[0xd4] - 2
	}
}

// computeColonyOfficerBonus is Compute_Colony_Officer_Bonus_ (0xec2af):
// the governor's ground skill, with the byte at colony +4.
func (g *Game) computeColonyOfficerBonus(c *save.Colony, b *groundBonus) {
	s := &g.state
	o := s.colonyOfficer(c)
	if o == -1 {
		return
	}
	l := &s.Leaders[o]
	switch {
	case l.CommonSkills&8 != 0:
		binary.LittleEndian.PutUint16(b[0x10:], uint16((int32(s.ownedOfficerLevel(o))+1)*3))
	case l.CommonSkills&4 != 0:
		binary.LittleEndian.PutUint16(b[0x10:], uint16((int32(s.ownedOfficerLevel(o))+1)*2))
	}
	if b.officer() > 0 {
		b[0x12] = uint8(c.Unknown004)
	}
}

// computeGroundCombatInfo is Compute_Ground_Combat_Info_ (0xec3ce): each
// class's strength and hits from the bonuses (militia use the enemy's
// weapons), then the first class with troops.
func computeGroundCombatInfo(F *groundForce, own, enemy *groundBonus) {
	gear := func(b *groundBonus) int16 {
		return int16(b[6]) + int16(b[0]) + int16(b[8]) + int16(b[0xa])
	}
	base := gear(own)
	mod := int16(int8(own[0xd])) + int16(int8(own[0xb])) + int16(own[0xe]) + int16(int8(own[0xf])) + own.officer()
	for k := 3; k >= 0; k-- {
		switch {
		case k != 3:
			F.strength[k] = base
		case enemy != nil:
			F.strength[k] = gear(enemy)
		default:
			F.strength[k] = 0
		}
		F.strength[k] += mod
		F.hits[k] = own[0xc] + 1
		switch k {
		case 0:
			F.strength[k] += 10
			F.hits[k]++
			F.strength[k] += int16(own[1])
			F.hits[k] += own[2]
		case 1:
			F.strength[k] += int16(own[3])
			F.hits[k] += own[4]
		case 2:
			F.strength[k] -= 10
		case 3:
			F.strength[k] -= 20
		}
	}
	F.class, F.hurt, F.lost = 0, -1, -1
	for F.count[F.class] == 0 && F.class < 4 {
		F.class++
	}
	F.damage = 0
}

// groundCombatRound is Ground_Combat_Round_ (0xec4fe): one Random(100) per
// side added to the fighting class's strength; the lower side (both on a
// tie) takes a hit, and a unit dies when its hits run out.
func (g *Game) groundCombatRound(A, B *groundForce) {
	r := &g.state.Random
	ra := int16(r.Random(100)) + A.strength[A.class]
	rb := int16(r.Random(100)) + B.strength[B.class]
	A.hit(ra <= rb)
	B.hit(ra >= rb)
}

func (F *groundForce) hit(hit bool) {
	F.hurt, F.lost = -1, -1
	if !hit {
		return
	}
	F.damage++
	if F.damage != F.hits[F.class] {
		F.hurt = int8(F.class)
		return
	}
	F.lost = int8(F.class)
	F.count[F.class]--
	F.damage = 0
	if F.count[F.class] != 0 {
		return
	}
	for F.count[F.class] == 0 && F.class < 4 {
		F.class++
	}
	F.damage = 0
}

// resolveRebellionTroops is Resolve_Rebellion_Troops_ (0xececa): the
// surviving rebels become marines (at most 4, and the plain limit).
func (g *Game) resolveRebellionTroops(A *groundForce) {
	s := &g.state
	c := &s.Colonies[A.colony]
	lim := int32(int16(g.colonyInfantryLimit(c)))
	c.Marines = min(A.count[3], 4)
	if lim <= int32(c.Marines) {
		c.Marines = int16(lim)
	}
	s.computeStarColonyStuff(int(s.Planets[c.Planet].Star))
}

// captureReport is the part of the invasion report Change_Colony_Ownership_
// fills: up to four captured applications (+0x72), their count (+0x21a)
// and the planet (+0x21c).
type captureReport struct {
	apps   [4]int16
	n      int16
	planet int16
	// port-only: Invade_ ran (the seed and n are set), and
	// Change_Colony_Ownership_ wrote the planet
	invaded, captured bool
}

// changeColonyOwnership is Change_Colony_Ownership_ (0xecf41): the colony
// goes to player to, without troops or defence buildings; with an invasion
// report, buildings and known applications may yield technology.
func (g *Game) changeColonyOwnership(ci, to int, rep *captureReport) {
	s := &g.state
	c := &s.Colonies[ci]
	old := int(c.Owner)
	s.Players[old].WarLosses2[to] += 10
	g.changePopOwnership(ci, to)
	for _, b := range []int{2, 0x16, 1} {
		if c.Buildings[b] != 0 {
			g.removeBuilding(ci, b)
		}
	}
	c.ArmorProgress, c.MarineProgress = 0, 0
	c.Marines, c.Armor = 0, 0
	T := &s.Players[to]
	T.CapturedColonists += int16(c.PopCount)
	if rep == nil {
		return
	}
	var apps [4]int16
	n := int32(0)
	chance := int32(0x7d)
	if T.Objective != 100 {
		chance = int32(g.settings[0xd4])*5 + 0x73
	}
	for b := 0x30; b >= 0; b-- {
		if c.Buildings[b] == 0 {
			continue
		}
		app := g.exeI16(uint32(bldgTable + b*bldgRow + bldgApp))
		if !g.playerMayGetTechApp(to, int(app)) {
			continue
		}
		k := n + 1
		if int32(s.Random.Random(1000)) > chance/k {
			continue
		}
		apps[n] = app
		n = k
		g.playerGetsTechApp(to, int(app))
		if k == 4 {
			break
		}
	}
	if n < 4 {
		pops := int32(c.PopCount)
		chance = 0x41
		if T.Objective != 100 {
			chance = int32(g.settings[0xd4])*5 + 0x37
		}
		var w [numTechApps]uint16
		for a := numTechApps - 1; a >= 0; a-- {
			if s.Players[old].AppStatus[a] == techKnown && g.playerMayGetTechApp(to, a) {
				w[a] = 1
			}
		}
		for {
			k := n + 1
			if int32(s.Random.Random(1000)) <= chance/k {
				if a := getWeightedChoiceShort(&s.Random, w[:]); a == -1 {
					pops = 0
				} else {
					w[a] = 0
					apps[n] = a
					n = k
					g.playerGetsTechApp(to, int(a))
				}
			}
			if pops--; pops <= 0 || n >= 4 {
				break
			}
		}
	}
	copy(rep.apps[:n], apps[:n])
	rep.n = int16(n)
	rep.planet = c.Planet
	rep.captured = true
}

// changePopOwnership is Change_Pop_Ownership_ (0xecbf7): ownership, home
// planet, government buildings and pop loyalty change; the colony starts
// over under occupation.
func (g *Game) changePopOwnership(ci, to int) {
	s := &g.state
	c := &s.Colonies[ci]
	g.handleLeaderAtLostColony(ci)
	old := int(c.Owner)
	s.Players[old].WarLosses2[to] += int16(c.PopCount)
	if c.Buildings[9] != 0 {
		g.removeBuilding(ci, 9)
	}
	if s.Players[old].HomePlanet == c.Planet {
		s.assignNewHomePlanet(old, ci)
	}
	c.Owner = int8(to)
	g.doColonyOwnerChangeStarColonyStuff(c)
	T := &s.Players[to]
	if T.HomePlanet == -1 {
		T.HomePlanet = c.Planet
	}
	c.AutoBuild = 0
	if int32(T.Picks[save.PickGovernment])/2 == 3 {
		if c.Buildings[0x14] != 0 {
			g.removeBuilding(ci, 0x14)
			T.BC += g.bldgCostOf(0x14) / 2
		}
		if c.Buildings[0x1f] != 0 {
			g.removeBuilding(ci, 0x1f)
			T.BC += g.bldgCostOf(0x1f) / 2
		}
	}
	loyal := uint32(to&7) << 4
	for i := int(c.PopCount) - 1; i >= 0; i-- {
		col := c.Colonists[i]
		race := int(col & 0xf)
		switch {
		case race == to:
			c.Colonists[i] = col&^0x470 | loyal
		case race == 8 || race == 9:
			c.Colonists[i] = col&^0x70 | loyal
		case int(col>>4&7) == to:
			c.Colonists[i] = col &^ 0x400
		case T.Picks[save.PickTelepathic] != 0:
			c.Colonists[i] = col&^0x470 | loyal
		default:
			c.Colonists[i] = col | 0x400
		}
	}
	s.colonyEnvironmentalStuff(c)
	g.enforcePopulationLimitsAtColony(ci)
	for k := 0; k < 7; k++ {
		g.deleteFromColonyQueue(ci, 0)
	}
	c.Production = 0
	c.LastBuilt = -40
	c.Assimilation = 0
	c.Occupation = 4
	c.Buildings[0xb] = 0
	s.computeStarColonyStuff(int(s.Planets[c.Planet].Star))
	c.TurnsOwned = 0
}

// doColonyOwnerChangeStarColonyStuff is
// Do_Colony_Owner_Change_Star_Colony_Stuff_ (0xe53cd).
func (g *Game) doColonyOwnerChangeStarColonyStuff(c *save.Colony) {
	g.state.computeStarColonyStuff(int(g.state.Planets[c.Planet].Star))
	if c.Buildings[0x2d] != 0 {
		g.updateShipETAs(-1)
	}
}
