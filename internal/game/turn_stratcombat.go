package game

import "github.com/openorion2/openorion2/internal/save"

// Strategic combat (COMBAT.C): the auto-resolver for battles without a human
// at the tactical screen. Spec: docs/turn/combat.md.

// qship is one 0x21-byte record of _qcombat_data (0x192844); slot 0 is the
// defending colony, ships follow from slot 1, satellites after the defenders.
type qship struct {
	owner   uint8 // +0x00
	size    uint8 // +0x01 hull, 4 colony, 6..8 satellites
	ship    int16 // +0x02, -1 for colony and satellites
	unk4    int16 // +0x04, always -1
	dead    uint8 // +0x06 0 alive, 1 destroyed this round, 2 out
	officer int16 // +0x07
	shield  int16 // +0x09
	missile int16 // +0x0b weapon
	beam    int16 // +0x0d weapon
	bomb    int16 // +0x0f weapon
	special int16 // +0x11 weapon
	mdef    int16 // +0x13 missile defence
	defense int16 // +0x15
	ocv     int16 // +0x17
	beamPct int16 // +0x19 beam damage bonus
	misPct  int16 // +0x1b missile damage bonus
	hits    int16 // +0x1d
	damage  int16 // +0x1f
}

// qcombat is the strategic combat state: the records in screen_seg and the
// globals around them. The block survives between battles, as screen_seg
// does: Strategic_Bombardment_ reuses it without clearing.
type qcombat struct {
	// Allocate_First_Block_ asks for 0x4075 bytes (499 records) but
	// Clear_Structure_ clears 0x4095: 501, the colony and every ship.
	rec    [0x4095 / 0x21]qship
	n      int16 // 0x1998c0 records in use
	nAtt   int16 // 0x1998a2 live attackers
	nDef   int16 // 0x1998be live defenders
	turn   int16 // 0x199890
	att    int16 // 0x1998ac
	def    int16 // 0x19988e
	colony int16 // 0x199878
	flags  uint8 // 0x199ef6 colony defences: 1 missile base, 2 batteries, 4 converter, 8 fighters

	// Build_End_Of_Combat_Message_'s title and text (Russ_Combat_'s
	// result buffers), built only when a display is attached.
	endTitle, endText string
}

// Constant tables (ELF addresses).
const (
	hullDefense    = 0x12 // i16, hull table
	hullHits       = 0x14
	armorHitsPct   = 0x04     // i16, armour table
	armorHitsBonus = 0x08     // u8, QGet_Hits_Bonus_
	weaponMinDmg   = 0x14     // i16, weapon table
	weaponMaxDmg   = 0x16     // i16
	converterDmg   = 0x17fc41 // i16 stellar converter damage per shot
	artemisChance  = 0x180ec8 // i16 by hull, Artemis net hit chance
)

func (g *Game) hullI16(size int, off uint32) int16 {
	return g.exeI16(uint32(hullTable + size*hullStride + int(off)))
}

func (g *Game) weaponI16(w int16, off uint32) int16 {
	return g.exeI16(rowAddr(weaponTable, weaponStride, int(w), off))
}

// combatOfficerBonus is the shape of Helmsman_Bonus_ (0x3677d), Leader_
// Ordnance_Bonus_ (0x35f4d) and Galactic_Lore_Bonus_ (0x367df): a special
// skill bit pair of byte b gives (level+1)*5 or (level+1)*15/2.
func (g *Game) combatOfficerBonus(o, b int, lo, hi byte) int16 {
	sk := byte(g.state.Leaders[o].SpecialSkills >> (8 * b))
	switch {
	case sk&lo != 0:
		return (g.state.ownedOfficerLevel(o) + 1) * 5
	case sk&hi != 0:
		return int16((int32(g.state.ownedOfficerLevel(o)) + 1) * 15 / 2)
	}
	return 0
}

// engineerBonus is Engineer_Bonus_ (0x3607a).
func (g *Game) engineerBonus(o int) int16 {
	sk := byte(g.state.Leaders[o].SpecialSkills)
	switch {
	case sk&1 != 0:
		return (g.state.ownedOfficerLevel(o) + 1) * 2
	case sk&2 != 0:
		return (g.state.ownedOfficerLevel(o) + 1) * 3
	}
	return 0
}

// leaderCombatBonuses is Get_Leader_Combat_Bonuses_ (0x40034): the best of
// each bonus among the side's officers that are aboard.
func (g *Game) leaderCombatBonuses(list []int16, helm, ord, eng, weap, lore *int16) {
	s := &g.state
	for _, k := range list {
		o := int(s.Ships[k].Officer)
		if o == -1 || s.Leaders[o].ETA != 0 {
			continue
		}
		*helm = max(*helm, g.combatOfficerBonus(o, 0, 0x40, 0x80))
		*ord = max(*ord, g.combatOfficerBonus(o, 1, 0x04, 0x08))
		*eng = max(*eng, g.engineerBonus(o))
		*weap = max(*weap, g.combatOfficerBonus(o, 1, 0x40, 0x80))
		*lore = max(*lore, g.combatOfficerBonus(o, 0, 0x10, 0x20))
	}
}

// baseGenericOcv is Base_Generic_Ocv_ (0x5eb39).
func (g *Game) baseGenericOcv(p int16) int16 {
	v := g.exeI16(rowAddr(computerTable, computerStride, g.bestComputer(int(p)), computerBonus))
	if p < 8 {
		v += int16(g.state.Players[p].Picks[save.PickShipAttack])
	}
	return v
}

// bestMissileWeapon is Best_Missile_Weapon_ (0x56b26): missiles and
// fighters, by strength.
func (g *Game) bestMissileWeapon(p int) int {
	switch {
	case int16(p) < 8:
		return g.bestWeapon(p, 0xe, 1, 2)
	case uint16(p-8) <= 6:
		return monsterMissile[p-8]
	}
	return 0xe
}

// bestStratSpecial is Best_Strat_Special_Weapon_ (0x56d3a) with its monster
// table (0x56d1e).
func (g *Game) bestStratSpecial(p, size int) int {
	if int16(p) < 8 {
		return g.bestStratSpecialWeapon(p, size)
	}
	if k := uint16(p - 8); k <= 6 {
		return [7]int{0x25, 0, 0x2d, 0, 0, 0x2c, 0}[k]
	}
	return 0
}

// qloadShips is Qload_Ships_ (0x416cf): the side's combat ships with the
// owner's best equipment, then slot 0 (the colony, if it is the defender's)
// and its satellite.
func (g *Game) qloadShips(list []int16, helm, ord, weap int16) {
	s := &g.state
	q := g.qc
	var ocvSp, beamSp, defSp, mdefSp, misSp int16
	var owner int16
	if len(list) == 0 {
		owner = int16(s.Colonies[q.colony].Owner)
	} else {
		owner = int16(s.Ships[list[0]].Owner)
	}
	o := int(owner)
	missile := int16(g.bestMissileWeapon(o))
	beam := int16(g.bestBeamWeapon(o))
	bomb := int16(g.bestBombWeapon(o))
	shield := int16(g.bestShipShield(o))
	ocv := g.baseGenericOcv(owner)
	warp := int16((g.bestWarpDrive(o) - 1) * 10)
	if owner < 8 {
		warp += int16(s.Players[o].Picks[save.PickShipDefense])
	}
	armor := g.armorField(g.bestArmor(o), armorHitsPct)
	for _, k := range list {
		sh := &s.Ships[k]
		if sh.Design.Type != 0 {
			continue
		}
		size := int16(sh.Design.Size)
		special := int16(g.bestStratSpecial(o, int(size)))
		g.updateStrategicShip(int(k))
		r := &q.rec[q.n]
		r.dead, r.unk4, r.owner = 0, -1, uint8(sh.Owner)
		r.beam, r.ship, r.size = beam, k, uint8(size)
		r.missile, r.bomb, r.special, r.shield = missile, bomb, special, shield
		if r.owner <= 9 {
			r.hits = int16((int32(armor) + 100) * int32(g.hullI16(int(size), hullHits)) / 100)
		} else {
			r.hits = g.hullI16(int(size), hullHits)
		}
		r.mdef, r.misPct, r.beamPct = 0, 0, 0
		r.ocv = ocv
		r.defense = g.hullI16(int(size), hullDefense) + warp
		for b := 0; b < 0x27; b++ {
			if !s.shipHasSpecial(int(k), b) {
				continue
			}
			v := g.exeI16(rowAddr(specialTable, specialStride, b, specialValue))
			switch g.exeU8(rowAddr(specialTable, specialStride, b, specialKind)) {
			case 1:
				ocvSp = max(ocvSp, v)
			case 2:
				beamSp = max(beamSp, v)
			case 3:
				defSp = max(defSp, v)
			case 4:
				mdefSp = max(mdefSp, v)
			case 5:
				misSp = max(misSp, v)
			case 6:
				defSp = max(defSp, v)
				mdefSp = max(mdefSp, v)
			case 7:
				misSp = max(misSp, v)
				beamSp = max(beamSp, v)
			}
		}
		r.defense += defSp + helm
		r.misPct += misSp + ord
		r.ocv += ocvSp + weap
		r.beamPct += beamSp + ord
		r.mdef += mdefSp
		r.officer = sh.Officer
		r.damage = sh.StructureDamage
		q.n++
	}
	fighters := int16(g.bestStratSpecial(o, 5))
	c0 := &q.rec[0]
	c0.size, c0.ship, c0.unk4 = 4, -1, -1
	c0.damage, c0.hits = 0, 0
	c0.missile, c0.beam, c0.bomb, c0.special, c0.mdef, c0.defense = 0, 0, 0, 0, 0, 0
	if q.colony == -1 || owner != q.def {
		c0.dead = 2
		return
	}
	C := &s.Colonies[q.colony]
	c0.owner = uint8(C.Owner)
	c0.hits = int16(g.colonyHits())
	c0.ship = -1
	if C.Buildings[0x1c] != 0 {
		c0.mdef += 10
		c0.defense *= 10
	}
	if C.Buildings[0x17] != 0 {
		c0.mdef += 30
		c0.defense *= 30
	}
	if C.Buildings[0x18] != 0 {
		c0.mdef += 20
		c0.defense *= 20
	}
	c0.missile = 0
	if C.Buildings[0x1a] != 0 {
		q.flags++
		c0.missile = missile
	}
	if C.Buildings[0x2f] != 0 {
		q.flags += 8
		c0.special = fighters
	}
	if C.Buildings[0x1b] != 0 {
		q.flags += 2
		c0.beam = beam
	}
	if C.Buildings[0x2a] != 0 {
		q.flags += 4
	}
	if q.flags != 0 {
		c0.dead = 0
	} else {
		c0.dead = 2
	}
	sat := 0
	switch {
	case C.Buildings[0x28] != 0:
		sat = 6
	case C.Buildings[8] != 0:
		sat = 7
	case C.Buildings[0x29] != 0:
		sat = 8
	default:
		return
	}
	r := &q.rec[q.n]
	r.ship, r.size, r.dead, r.unk4, r.owner = -1, uint8(sat), 0, -1, uint8(C.Owner)
	r.beam, r.special, r.missile, r.bomb, r.shield = beam, fighters, missile, bomb, shield
	r.mdef = 0
	r.hits = int16((int32(armor) + 100) * int32(g.hullI16(sat, hullHits)) / 100)
	r.damage = 0
	r.defense = g.hullI16(sat, hullDefense)
	r.ocv = ocv
	q.n++
}

// colonyHits is Get_Colony_Hits_ (0x42371): 40 per pop, marine, tank and
// building other than the satellites, in 16 bits.
func (g *Game) colonyHits() uint16 {
	C := &g.state.Colonies[g.qc.colony]
	n := uint16(C.PopCount) + uint16(C.Marines) + uint16(C.Armor)
	for b := 0; b < 0x31; b++ {
		if C.Buildings[b] != 0 && b != 8 && b != 0x28 && b != 0x29 {
			n++
		}
	}
	return n * 0x28
}

// qloadAntaranDefenseFleet is QLoad_Antaran_Defense_Fleet_ (0x3fef2): the
// ships guarding Antares by size, then the Antaran home world.
func (g *Game) qloadAntaranDefenseFleet() {
	s := &g.state
	q := g.qc
	if s.antWord(0xe) < 1 {
		s.setAntWord(0xe, 1)
	}
	for size := 0; size < 5; size++ {
		for k := int16(0); k < s.antWord(6+2*size); k++ {
			g.qloadCombatAntaranShip(q.n, size)
			q.n++
			q.nDef++
		}
	}
	r := &q.rec[q.n]
	r.owner, r.ship, r.size, r.dead, r.unk4 = 8, -1, 8, 0, -1
	r.missile = int16(g.bestMissileWeapon(8))
	r.beam = int16(g.bestBeamWeapon(8))
	r.bomb, r.special = 0, 0
	r.hits = int16(int32(g.hullI16(8, hullHits)) * 1000 / 100)
	r.shield = 7
	r.defense = g.hullI16(8, hullDefense) + 0x2d
	r.mdef = 0
	r.misPct = 0
	q.nDef++
	r.beamPct = 0x69
	r.damage = 0
	r.ocv = g.baseGenericOcv(8) + 0x2d
	q.n++
}

// qloadCombatAntaranShip is QLoad_Combat_Antaran_Ship_ (0x3fdb4).
func (g *Game) qloadCombatAntaranShip(i int16, size int) {
	r := &g.qc.rec[i]
	missile := int16(g.bestMissileWeapon(8))
	beam := int16(g.bestBeamWeapon(8))
	bomb := int16(g.bestBombWeapon(8))
	shield := int16(g.bestShipShield(8))
	ocv := g.baseGenericOcv(8)
	armor := g.armorField(g.bestArmor(8), armorHitsPct)
	r.special = int16(g.bestStratSpecial(8, size))
	r.shield = shield
	r.ship, r.dead, r.unk4 = -1, 0, -1
	r.size, r.owner = uint8(size), 8
	r.missile, r.beam, r.bomb = missile, beam, bomb
	r.hits = int16((int32(armor) + 100) * int32(g.hullI16(size, hullHits)) / 100)
	r.mdef, r.misPct, r.beamPct = 0, 0, 0x69
	r.officer = -1
	r.defense = g.hullI16(size, hullDefense) + 0x5f
	r.damage = 0
	r.ocv = ocv + 0x2d
}

// qgetTarget is QGet_Target_ (0x41e88): a random live enemy record; with
// none, the attacker side bombs a live colony (slot 0).
func (g *Game) qgetTarget(i int16) int16 {
	q := g.qc
	own := q.rec[i].owner
	var cand []int16
	for j := int16(1); j < q.n; j++ {
		if r := &q.rec[j]; r.owner != own && r.dead == 0 {
			cand = append(cand, j)
		}
	}
	if len(cand) == 0 {
		if int16(own) == q.att && q.colony != -1 && q.rec[0].dead == 0 {
			return 0
		}
		return -1
	}
	return cand[g.state.Random.Random(uint32(len(cand)))-1]
}

// qgetTargetSC is QGet_Target_SC_ (0x41f20): the live enemy with the most
// hits left, for the planetary stellar converter.
func (g *Game) qgetTargetSC(i int16) int16 {
	q := g.qc
	own := q.rec[i].owner
	best, left := int16(-1), int16(0)
	for j := int16(1); j < q.n; j++ {
		r := &q.rec[j]
		if r.owner == own || r.dead != 0 {
			continue
		}
		if v := r.hits - r.damage; v > left {
			best, left = j, v
		}
	}
	return best
}

// applyDamageToQship is Apply_Damage_To_Qship_ (0x40c2a): shields take
// their strength off each hit, class 7 (Antaran) a quarter of it.
func (g *Game) applyDamageToQship(i, d int16) {
	r := &g.qc.rec[i]
	if r.shield == 7 {
		d /= 4
	} else {
		d -= g.exeI16(rowAddr(shieldTable, shieldStride, int(r.shield), shieldStrength))
	}
	if d < 0 {
		d = 0
	}
	r.damage += d
	if r.damage > 30000 {
		r.damage = 30000
	}
	if r.damage >= r.hits {
		r.dead = 1
	}
}

// missileAttack is Missile_Attack_ (0x41f80).
func (g *Game) missileAttack(a, t int16) {
	s := &g.state
	q := g.qc
	A := &q.rec[a]
	thr := q.rec[t].mdef
	w := A.missile
	n := g.hullI16(int(A.size), hullMissiles)
	if w <= 0 || n <= 0 {
		return
	}
	dmg := g.weaponI16(w, weaponMaxDmg)
	if A.misPct > 0 {
		dmg = int16((int32(A.misPct) + 100) * int32(dmg) / 100)
	}
	if A.owner < 8 {
		apps := &s.Players[A.owner].AppStatus
		switch {
		case apps[0xa0] == 3:
			thr -= 0x46
		case apps[0x74] == 3:
			thr -= 0x28
		case apps[0xb5] == 3:
			thr -= 0x14
		}
	} else {
		thr -= 0x46
	}
	for {
		r := int16(s.Random.Random(100))
		if r > 0x5f {
			r = 1000
		}
		if thr <= r {
			g.applyDamageToQship(t, dmg)
			if T := &q.rec[t]; T.damage >= T.hits {
				T.dead = 1
				t = g.qgetTarget(a)
			}
		}
		n--
		if n <= 0 || t == -1 {
			return
		}
	}
}

// beamAttack is Beam_Attack_ (0x4221f): to hit 40 minus the attack/defence
// difference (capped at 95), damage uniform in the weapon's range.
func (g *Game) beamAttack(a, t int16) {
	s := &g.state
	q := g.qc
	A := &q.rec[a]
	n := g.hullI16(int(A.size), hullBeams)
	chance := 0x28 - (A.ocv - q.rec[t].defense)
	if chance > 0x5f {
		chance = 0x5f
	}
	w := A.beam
	if w <= 0 || n <= 0 {
		return
	}
	lo := int32(g.weaponI16(w, weaponMinDmg))
	hi := int32(g.weaponI16(w, weaponMaxDmg))
	if A.beamPct > 0 {
		m := int32(A.beamPct) + 100
		lo = m * lo / 100
		hi = hi * m / 100
	}
	span := uint32(hi - lo + 1)
	for {
		if int16(s.Random.Random(100)) >= chance {
			d := int16(int32(s.Random.Random(span)) + lo - 1)
			g.applyDamageToQship(t, d)
			if q.rec[t].dead == 1 {
				t = g.qgetTarget(a)
			}
		}
		n--
		if n <= 0 || t == -1 {
			return
		}
	}
}

// stratSpecialAttack is Strat_Special_Attack_ (0x4215c): weapons 0x25 fire
// half as often, 0x26 a quarter, and never miss.
func (g *Game) stratSpecialAttack(a, t int16) {
	s := &g.state
	q := g.qc
	w := q.rec[a].special
	n := g.hullI16(int(q.rec[a].size), hullSpecials)
	switch w {
	case 0x25:
		n = int16((int32(n) + 1) / 2)
	case 0x26:
		n = int16((int32(n) + 3) / 4)
	}
	if w <= 0 || n <= 0 {
		return
	}
	for {
		lo := int32(g.weaponI16(w, weaponMinDmg))
		d := int16(int32(s.Random.Random(uint32(int32(g.weaponI16(w, weaponMaxDmg))-lo+1))) + lo - 1)
		g.applyDamageToQship(t, d)
		if q.rec[t].dead == 1 {
			t = g.qgetTarget(a)
		}
		n--
		if n <= 0 || t == -1 {
			return
		}
	}
}

// stratBombAttack is Strat_Bomb_Attack_ (0x420c0): every bomb hits the
// colony.
func (g *Game) stratBombAttack(a int16) {
	s := &g.state
	q := g.qc
	w := q.rec[a].bomb
	n := g.hullI16(int(q.rec[a].size), hullBombs)
	if w <= 0 || n <= 0 {
		return
	}
	lo := g.weaponI16(w, weaponMinDmg)
	hi := g.weaponI16(w, weaponMaxDmg)
	for ; n > 0; n-- {
		d := int16(int32(s.Random.Random(uint32(int32(hi)-int32(lo)+1))) + int32(lo) - 1)
		g.applyDamageToQship(0, d)
		if c := &q.rec[0]; c.damage >= c.hits {
			c.dead = 1
		}
	}
}

// resolveStratColonyDamage is Resolve_Strat_Colony_Damage_ (0x423f6): each
// 40 points of colony damage destroy a defence building (converter, missile
// base, batteries, fighters); a colony without defences left is beaten.
func (g *Game) resolveStratColonyDamage() {
	q := g.qc
	c := &q.rec[0]
	if c.dead > 1 || c.damage/0x28 <= 0 || q.colony == -1 {
		return
	}
	for _, d := range [4]struct {
		flag uint8
		b    int
	}{{4, 0x2a}, {1, 0x1a}, {2, 0x1b}, {8, 0x2f}} {
		if c.damage/0x28 > 0 && q.flags&d.flag != 0 {
			q.flags ^= d.flag
			g.removeBuilding(int(q.colony), d.b)
			if d.flag == 2 {
				c.mdef -= 10
			}
			c.damage -= 0x28
		}
	}
	c.damage %= 0x28
	if q.flags == 0 && c.dead == 0 {
		c.dead = 1
	}
}

// applyCombatArtemisNet is Apply_Combat_Artemis_Net_ (0x3868f) in its
// strategic form: each attacking ship may hit the net; it returns true when
// none survived.
func (g *Game) applyCombatArtemisNet() bool {
	s := &g.state
	q := g.qc
	all := true
	for i := int16(1); i < q.n; i++ {
		r := &q.rec[i]
		chance := int32(g.exeI16(uint32(artemisChance + 2*int(r.size))))
		if int32(s.Random.Random(100)) > chance {
			all = false
			continue
		}
		sh := r.shield
		d := int16(s.Random.Random(0xc9) + 0x4a)
		r.shield = 0
		g.applyDamageToQship(i, d)
		r.shield = sh
		if r.dead != 1 {
			all = false
			continue
		}
		q.nAtt--
		r.dead = 2
	}
	return all
}

// playerHasArtemisNet is Player_Has_Artemis_Net_ (0x79cf9).
func (s *State) playerHasArtemisNet(p int, star int16) bool {
	return s.Stars[star].ArtemisNet>>uint(p)&1 != 0
}

// killAntaranDefensiveShip is Kill_Antaran_Defensive_Ship_ (0x646f9): an
// offensive ship, if any, takes the place of a lost guard.
func (s *State) killAntaranDefensiveShip(size int) {
	def, off := 6+2*size, 0x18+2*size
	s.setAntWord(def, max(s.antWord(def)-1, 0))
	if s.antWord(off) > 0 {
		s.setAntWord(off, max(s.antWord(off)-1, 0))
		s.setAntWord(def, s.antWord(def)+1)
	}
}

// killAntaranOffensiveShip is Kill_Antaran_Offensive_Ship_ (0x6474b).
func (s *State) killAntaranOffensiveShip(size int) {
	off, stock := 0x18+2*size, 0x2a+2*size
	s.setAntWord(off, max(s.antWord(off)-1, 0))
	s.setAntWord(stock, max(s.antWord(stock)-1, 0))
}

// strategicCombat is Strategic_Combat_ (0x40148): up to 50 rounds between
// the two lists (the defenders may be only a colony, or Antares); 1 or more
// means the attacker won, 0 a draw, -1 a loss.
func (g *Game) strategicCombat(att, def []int16, star, colony int16) int16 {
	s := &g.state
	if g.qc == nil {
		g.qc = &qcombat{}
	}
	q := g.qc
	var repair [20]int16
	kills := int32(0)
	result := int16(0)
	var offList []int16
	flags0 := uint16(0) // [ebp+0x2e]: the defences loaded, for the result box
	q.colony = colony
	q.turn = 0
	*q = qcombat{colony: colony}
	q.att = int16(s.Ships[att[0]].Owner)
	switch {
	case star == s.NumStars:
		q.def = 8
	case len(def) == 0:
		q.def = int16(s.Colonies[colony].Owner)
	default:
		q.def = int16(s.Ships[def[0]].Owner)
	}
	if q.att == 9 && q.def >= 8 {
		for _, k := range def {
			g.killShip(int(k))
		}
		return 1
	}
	if q.def == 9 && q.att >= 8 {
		for _, k := range att {
			g.killShip(int(k))
		}
		return -1
	}
	q.n = 1
	q.flags = 0
	var helm, ord, weap, lore int16
	g.leaderCombatBonuses(att, &helm, &ord, &repair[q.att], &weap, &lore)
	g.qloadShips(att, helm, ord, weap)
	q.nAtt = q.n - 1
	artemis := false
	if q.def < 8 && star < s.NumStars && s.playerHasArtemisNet(int(q.def), star) {
		if artemis = g.applyCombatArtemisNet(); artemis {
			result = -1
		}
	}
	if !artemis {
		if star == s.NumStars {
			g.qloadAntaranDefenseFleet()
		} else {
			var helm, ord, weap, lore int16
			g.leaderCombatBonuses(def, &helm, &ord, &repair[q.def], &weap, &lore)
			g.qloadShips(def, helm, ord, weap)
		}
		q.nDef = q.n - q.nAtt
		if q.flags == 0 {
			q.nDef--
		}
		flags0 = uint16(q.flags)
		for q.nAtt > 0 && q.nDef > 0 && q.turn < 50 {
			q.turn++
			for i := int16(0); i < q.n; i++ {
				if q.rec[i].dead == 2 {
					continue
				}
				t := g.qgetTarget(i)
				if t == -1 {
					continue
				}
				if i == 0 && q.flags&4 != 0 {
					sc := g.qgetTargetSC(i)
					for k := 0; k < 4; k++ {
						g.applyDamageToQship(sc, g.exeI16(converterDmg))
					}
					t = g.qgetTarget(i)
				}
				if t != -1 && (q.turn < 4 || q.rec[i].size > 5 || i == 0) {
					if q.rec[i].owner == 8 {
						g.beamAttack(i, t)
					} else {
						g.missileAttack(i, t)
					}
				}
				if t != -1 && (q.turn > 2 || i == 0) {
					g.stratSpecialAttack(i, t)
				}
				if t != -1 && (q.turn > 2 || i == 0) {
					g.beamAttack(i, t)
				}
				if q.turn > 3 && int16(q.rec[i].owner) == q.att && q.rec[0].dead == 0 && q.colony != -1 {
					g.stratBombAttack(i)
				}
			}
			g.resolveStratColonyDamage()
			for i := int16(0); i < q.n; i++ {
				if r := &q.rec[i]; r.dead == 1 {
					r.dead = 2
					if int16(r.owner) == q.att {
						q.nAtt--
					} else {
						q.nDef--
					}
				}
			}
		}
		switch {
		case q.turn < 50:
			result = q.nAtt - q.nDef
		case q.nDef > 0:
			result = -1
		default:
			result = 0
		}
	}

	for i := int16(0); i < q.n; i++ {
		r := &q.rec[i]
		if r.dead > 0 && r.size > 4 && star != s.NumStars {
			switch r.size {
			case 6:
				g.removeBuilding(int(q.colony), 0x28)
			case 7:
				g.removeBuilding(int(q.colony), 8)
			case 8:
				g.removeBuilding(int(q.colony), 0x29)
			}
		}
		if r.ship != -1 {
			sh := &s.Ships[r.ship]
			switch {
			case repair[r.owner] > 0 || (r.owner < 8 &&
				(s.Players[r.owner].AppStatus[4] == 3 || s.Players[r.owner].Picks[save.PickCybernetic] != 0)):
				r.damage = 0
				sh.StructureDamage = 0
			case g.settings[0xd8] == 1:
				sh.StructureDamage = r.damage
			default:
				pct := int32(r.damage) * 100 / int32(r.hits)
				sh.StructureDamage = int16(int32(int16(g.shipStructure(int(r.ship)))) * pct / 100)
			}
		}
		if r.dead == 0 || r.ship == -1 {
			continue
		}
		if int16(r.owner) != q.att {
			kills += int32(r.size) + 1
		}
		o := r.officer
		if o <= -1 {
			continue
		}
		switch {
		case s.Leaders[o].ETA > 0:
			g.moveOfficerToPool(int(o))
			s.Ships[r.ship].Officer = -1
		case s.Players[r.owner].AppStatus[0xb3] == 3:
			offList = append(offList, o)
		default:
			g.killOfficer(int(o))
		}
	}
	if q.att < 8 && q.def < 8 {
		if q.colony != -1 {
			g.russChangeRelations(-10000, int(q.att), int(q.def), 10, star, 1)
		}
		g.russChangeRelations(int16(-15*kills), int(q.att), int(q.def), 0xb, star, 0)
	}
	for _, o := range offList {
		found, seated := false, false
		L := &s.Leaders[o]
		for j := int16(1); j < q.n && !found; j++ {
			r := &q.rec[j]
			if r.dead != 0 || r.ship == -1 || int8(L.Owner) != int8(r.owner) {
				continue
			}
			if r.officer == -1 {
				s.Ships[r.ship].Officer = o
				r.officer = o
				s.Ships[L.Location].Officer = -1
				found = true
				L.Location = r.ship
			}
			seated = true
		}
		switch {
		case found:
		case seated:
			g.moveOfficerToPool(int(o))
		default:
			g.killOfficer(int(o))
		}
	}
	for i := int16(0); i < q.n; i++ {
		r := &q.rec[i]
		if star == s.NumStars && r.owner == 8 && r.dead > 0 && r.size <= 5 {
			s.killAntaranDefensiveShip(int(r.size))
			continue
		}
		if r.dead == 0 || r.ship == -1 {
			continue
		}
		if r.owner == 8 {
			s.killAntaranOffensiveShip(int(r.size))
		}
		if o := s.Ships[r.ship].Officer; o > -1 {
			g.moveOfficerToPool(int(o))
		}
		g.killShip(int(r.ship))
	}
	if result >= 1 && q.att < 8 && q.def >= 10 && s.Ships[def[0]].Design.Drive > 0 {
		result = 1
		g.russEventMonsterUpdate(int(q.def), -1, q.att)
	}
	if result <= -1 && q.def < 8 && q.att >= 10 && s.Ships[att[0]].Design.Drive > 0 {
		result = -1
		g.russEventMonsterUpdate(int(q.att), -1, q.def)
	}
	if g.e != nil {
		var err error
		if q.endTitle, q.endText, err = g.buildEndOfCombatMessage(star, flags0, result); err != nil {
			g.logOnce(err.Error())
		}
	}
	if q.colony != -1 && s.Colonies[q.colony].PopCount == 0 {
		empty := true
		for _, b := range s.Colonies[q.colony].Buildings {
			if b != 0 {
				empty = false
				break
			}
		}
		if empty {
			g.destroyColony(int(q.colony), 1)
		}
	}
	return result
}
