package game

// Ship and officer steps of Next_Turn_Calc_ (docs/turn/overview.md, phases
// D and F).

// makeScrapShipsDead is Make_Scrap_Ships_Dead_ (0xedf92): scrapped and
// destroyed ships free their slots, from the last ship down.
func (g *Game) makeScrapShipsDead() {
	s := &g.state
	for i := int(s.NumShips) - 1; i >= 0; i-- {
		if st := s.Ships[i].Status; st == 7 || st == 8 {
			g.killShip(i)
		}
	}
}

// killShip is Kill_Ship_ (0xa163a): the first officer aboard goes to the
// pool and the slot becomes free; nothing else of the record is cleared.
func (g *Game) killShip(ship int) {
	s := &g.state
	g.shipOwner(ship)
	for o := 0; o < maxLeaders; o++ {
		l := &s.Leaders[o]
		if l.Type == 0 && l.Location == int16(ship) && l.ETA > 0 {
			g.moveOfficerToPool(o)
			break
		}
	}
	s.Ships[ship].Location = -1
	s.Ships[ship].Status = 5
}

// moveOfficerToPool is Move_Officer_To_Pool_ (0x944a3): the leader leaves
// its ship or star and waits unassigned.
func (g *Game) moveOfficerToPool(o int) {
	s := &g.state
	l := &s.Leaders[o]
	if l.Location != -1 && l.Status != 5 {
		switch l.Type {
		case 0:
			if sh := &s.Ships[l.Location]; sh.Officer == int16(o) {
				sh.Officer = -1
			}
		case 1:
			st := &s.Stars[l.Location]
			if l.Owner == -1 {
				// Officer[-1] is the byte before the array.
				st.AIStagePoint = 0xff
			} else {
				st.Officer[l.Owner] = -1
			}
		}
	}
	l.Status = 0
	l.ETA = 0
	l.Location = -1
	if l.Type == 1 {
		g.doPlayerColonyCalculations(int(l.Owner))
	}
}

// doAllShipsXPCheck is Do_All_Ships_XP_Check_ (0x14a27): every player ship
// of all 500 slots gains a turn of crew experience.
func (g *Game) doAllShipsXPCheck() {
	s := &g.state
	var bonus [8]int16
	g.bestInstructorBonus(&bonus)
	for i := 0; i < maxShips; i++ {
		sh := &s.Ships[i]
		if int8(sh.Status) < 5 && uint32(int32(sh.Owner)) < 8 {
			g.doXPForShip(i, bonus[g.shipOwner(i)])
		}
	}
}

// bestInstructorBonus is Best_Instructor_Bonus_ (0x14915): per owner the
// best instructor bonus of its colony leaders in place (ETA 0).
func (g *Game) bestInstructorBonus(best *[8]int16) {
	s := &g.state
	for o := 0; o < maxLeaders; o++ {
		l := &s.Leaders[o]
		// The original also compares the owner with itself here.
		if l.Type != 1 || l.Status < 0 || l.ETA != 0 || l.Owner < 0 || l.Owner >= 8 {
			continue
		}
		var v int16
		if g.leaderHasSpecialSkill(o, 0x16) {
			v = int16(g.officerSkillBonus(o, 0x16))
		} else if g.leaderHasSpecialSkill(o, 0x17) {
			v = int16(g.officerSkillBonus(o, 0x17))
		}
		if v > best[l.Owner] {
			best[l.Owner] = v
		}
	}
}

// leaderHasSpecialSkill is Officer_Has_Special_Skill_ (0x9469e).
func (g *Game) leaderHasSpecialSkill(o, skill int) bool {
	mask := g.exe.u32(uint32(tblSkillData + skill*tblSkillDataRow + 1))
	return g.state.Leaders[o].SpecialSkills&mask != 0
}

// doXPForShip is Do_XP_For_Ship_ (0x149d5).
func (g *Game) doXPForShip(ship int, bonus int16) {
	sh := &g.state.Ships[ship]
	g.doAcademyAtLocationCheck(ship)
	xp := int32(sh.Experience) + 1 + int32(bonus)
	// The cap is the original's 0x17d186, crew row 2's threshold, so the
	// fourth threshold is never reached.
	if m := int32(g.exeI16(rowAddr(crewTable, crewStride, 2, crewXP))); xp > m {
		xp = m
	}
	sh.Experience = int16(xp)
	g.calcShipLevel(ship)
}

// doAcademyAtLocationCheck is Do_Academy_At_Location_Check_ (0x1487a): +1
// experience per own colony with a military academy at the ship's star.
func (g *Game) doAcademyAtLocationCheck(ship int) {
	s := &g.state
	sh := &s.Ships[ship]
	if sh.Status != 0 {
		return
	}
	st := &s.Stars[sh.Location]
	for i := 0; i < 5; i++ {
		p := st.Planets[i]
		if p <= -1 {
			continue
		}
		c := s.Planets[p].Colony
		if c <= -1 {
			continue
		}
		if col := &s.Colonies[c]; col.Owner == sh.Owner && col.Buildings[0x26] != 0 {
			sh.Experience++
		}
	}
}

// calcShipLevel is Calc_Ship_Level_ (0x147e7): crew level from the
// experience thresholds, +1 for Warlord, at most 5.
func (g *Game) calcShipLevel(ship int) {
	s := &g.state
	sh := &s.Ships[ship]
	lvl := int16(0)
	for i := 0; i < 4; i++ {
		if g.exeI16(rowAddr(crewTable, crewStride, i, crewXP)) <= sh.Experience {
			lvl++
		}
	}
	if s.Players[sh.Owner].Picks[30] != 0 {
		lvl++
	}
	sh.CrewLevel = uint8(min(lvl, 5))
}

// allocateNewShipSlots is Allocate_New_Ship_Slots_ (0x10011b): every free
// ship slot of all 500 is reserved again, for the Antarans, the human and
// then round-robin from player stardate % NUM_PLAYERS by build queue length.
func (g *Game) allocateNewShipSlots() {
	s := &g.state
	ant := int32(0)
	for i := 0x10; i >= 0; i -= 2 {
		ant += int32(int16(uint16(s.Antarans[0x18+i]) | uint16(s.Antarans[0x19+i])<<8))
	}
	if ant > 20 {
		ant = 20
	} else {
		ant += 2
	}
	var want [8]int32
	for i := 0; i < int(s.NumColonies); i++ {
		c := &s.Colonies[i]
		if c.Owner == -1 || c.IsOutpost != 0 {
			continue
		}
		for j := 6; j >= 0; j-- {
			if c.Queue[j] > -100 {
				want[c.Owner]++
			}
		}
	}
	reserve := int32(0)
	if g.gameType == 0 {
		reserve = min(want[s.PlayerNum], 20)
	}
	var list [maxShips]int32
	n, nAnt, given := 0, int32(0), int32(0)
	for i := 0; i < maxShips; i++ {
		if int(s.NumShips) > i && s.Ships[i].Status != 5 {
			continue
		}
		switch {
		case ant > nAnt:
			nAnt++
			s.Ships[i].Owner = 8
		case given < reserve:
			given++
			s.Ships[i].Owner = int8(s.PlayerNum)
		default:
			list[n] = int32(i)
			n++
		}
	}
	var count [8]int32
	count[s.PlayerNum] = given
	np := int32(s.NumPlayers)
	p := int32(s.Stardate) % np
	k, miss := 0, int32(0)
	for k < n && np > miss {
		if count[p] < want[p] {
			s.Ships[list[k]].Owner = int8(p)
			k++
			count[p]++
			miss = 0
		} else {
			miss++
		}
		if p++; p == np {
			p = 0
		}
	}
	// The leftovers go to every player still in the game; with none left the
	// original loops forever.
	for k < n {
		if s.Players[p].Eliminated == 0 {
			s.Ships[list[k]].Owner = int8(p)
			k++
			count[p]++
		}
		if p++; p == np {
			p = 0
		}
	}
}

// tblOfficerLevelXP holds the experience at the start of each officer
// level, copied onto the stack by Increase_Officer_Level_By_One_.
const tblOfficerLevelXP = 0x925a2

// crewXP is the s16 experience at which a crew passes its row's level.
const crewXP = 6

// checkForOfficerLevel is Check_For_Officer_Level_ (0x92fda): every owned
// leader in the pool or assigned gains one experience.
func (g *Game) checkForOfficerLevel() {
	s := &g.state
	for o := 0; o < leaderCount; o++ {
		l := &s.Leaders[o]
		if (l.Status == 1 || l.Status == 0) && l.Owner != -1 {
			g.giveOfficerXP(int(l.Owner), o, 1)
		}
	}
}

// officerLevel is the level as Give_Officer_XP_ and Officer_Skill_Bonus_
// compute it: unowned leaders never count Warlord.
func (s *State) officerLevel(o int) int16 {
	l := &s.Leaders[o]
	if l.Owner == -1 {
		return min(officerBaseLevel(l.Experience, false), 5)
	}
	return s.ownedOfficerLevel(o)
}

// giveOfficerXP is Give_Officer_XP_ (0x97c64): experience up to 30000; a
// new level flags the leader and recomputes a governed star for player p.
func (g *Game) giveOfficerXP(p, o int, xp int16) {
	s := &g.state
	l := &s.Leaders[o]
	old := s.officerLevel(o)
	l.Experience += xp
	if l.Experience > 30000 {
		l.Experience = 30000
	}
	if s.officerLevel(o) == old {
		return
	}
	if l.Type == 1 && l.Location != -1 {
		g.starColonyCalculation(int(l.Location), p)
	}
	l.LevelUp = 1
}

// starColonyCalculation is Star_Colony_Calculation_ (0xe2ab1): player p's
// colonies at the star, from the outer orbit in, then p's totals.
func (g *Game) starColonyCalculation(star, p int) {
	s := &g.state
	for i := 4; i >= 0; i-- {
		pl := s.Stars[star].Planets[i]
		if pl == -1 {
			continue
		}
		ci := s.Planets[pl].Colony
		if ci == -1 {
			continue
		}
		if c := &s.Colonies[ci]; int16(c.Owner) == int16(p) && c.IsOutpost == 0 {
			g.preImportComputing(int(ci))
			g.passOutImports(int(c.Owner), int(ci))
		}
	}
	g.updatePlayerStats(p)
}

// decrementOfficerETA is Decrement_Officer_ETA_ (0x934cf): offers expire
// after 30 turns; leaders on their way count down and take office.
func (g *Game) decrementOfficerETA() {
	s := &g.state
	for o := 0; o < leaderCount; o++ {
		l := &s.Leaders[o]
		switch l.Status {
		case 4:
			if l.ETA++; l.ETA >= 30 {
				g.deassignOfficer(o)
			}
		case 1:
			if l.ETA <= 0 {
				continue
			}
			if l.ETA--; l.ETA == 0 && l.Type == 1 {
				g.starColonyCalculation(int(l.Location), int(l.Owner))
			}
		}
	}
}

// deassignOfficer is Deassign_Officer_ (0x933f2): the leader returns to
// the hire list, one level up, unlinked from its star or ship.
func (g *Game) deassignOfficer(o int) {
	s := &g.state
	l := &s.Leaders[o]
	owner := l.Owner
	g.increaseOfficerLevelByOne(o)
	switch l.Type {
	case 1:
		if l.Location == -1 {
			break
		}
		st := &s.Stars[l.Location]
		if l.Status == 5 {
			st.Special = 0
		}
		if owner != -1 && int16(st.Officer[owner]) == int16(o) {
			st.Officer[owner] = -1
		}
	case 0:
		if owner != -1 && l.Location != -1 && int8(g.shipOwner(int(l.Location))) == owner {
			if sh := &s.Ships[l.Location]; sh.Officer == int16(o) {
				sh.Officer = -1
			}
		}
	}
	l.ETA = 0
	l.Status = -1
	l.Location = -1
	l.Owner = -1
}

// increaseOfficerLevelByOne is Increase_Officer_Level_By_One_ (0x979a0):
// experience jumps to the start of the next level, if there is one.
func (g *Game) increaseOfficerLevelByOne(o int) {
	s := &g.state
	l := &s.Leaders[o]
	warlord := l.Owner != -1 && s.Players[l.Owner].Picks[30] != 0
	if n := int(officerBaseLevel(l.Experience, warlord)) + 1; n < 6 {
		l.Experience = g.exeI16(uint32(tblOfficerLevelXP + 2*n))
	}
}

// repairShipsAtColonies is Repair_Ships_At_Colonies_ (0x580f5): ships at a
// star refit to their owner's best drive; combat ships at an own colony and
// all monsters are repaired.
func (g *Game) repairShipsAtColonies() {
	s := &g.state
	var best [8]int16
	for p := 0; p < 8; p++ {
		best[p] = int16(g.bestWarpDrive(p))
	}
	for i := 0; i < int(s.NumShips); i++ {
		sh := &s.Ships[i]
		d := &sh.Design
		if sh.Owner < 8 && sh.Status == 0 && int16(d.Drive) != best[sh.Owner] {
			old := g.exe.bytes(rowAddr(driveTable, driveStride, int(d.Drive), driveCombat+uint32(d.Size)), 1)[0]
			base := uint8(int16(d.CombatSpeed) - int16(int8(old)))
			d.Drive = uint8(best[sh.Owner])
			d.CombatSpeed = base + g.exeU8(rowAddr(driveTable, driveStride, int(d.Drive), driveCombat+uint32(d.Size)))
		}
		if sh.Owner > 8 {
			g.repairShipFull(i)
			continue
		}
		if sh.Location >= 0x48 || sh.Status != 0 || sh.Location <= -1 || d.Type != 0 {
			continue
		}
		// Bit 8 (Antarans) is bit 0 of the next byte, WarpInterdictor.
		st := &s.Stars[sh.Location]
		if testBit([]byte{st.ColonyOwners, st.WarpInterdictor}, int(sh.Owner)) {
			g.repairShipFull(i)
		}
	}
}

// repairShipFull is Repair_Ship_Full_ (0x581f3).
func (g *Game) repairShipFull(ship int) {
	sh := &g.state.Ships[ship]
	sh.StructureDamage = 0
	sh.DriveDamage = 0
	sh.ShieldDamage = 0
	sh.ComputerDamage = 0
	sh.ArmorDamage = 0
	for w := range sh.Design.Weapons {
		sh.Design.Weapons[w].Working = sh.Design.Weapons[w].Count
	}
	sh.DamagedSpecials = [5]byte{}
}

// resetNeedlessIgnoreCombatShipsFlags is
// Reset_Needless_Ignore_Combat_Ships_Flags_ (0x10087d): a player's
// ignore-combat bit at a star is cleared once nobody else is there.
func (g *Game) resetNeedlessIgnoreCombatShipsFlags() {
	s := &g.state
	var present [maxStars]uint16
	for i := int(s.NumShips) - 1; i >= 0; i-- {
		if sh := &s.Ships[i]; sh.Owner != -1 && sh.Status == 0 {
			present[sh.Location] |= uint16(1 << (uint8(sh.Owner) & 31))
		}
	}
	for i := int(s.NumColonies) - 1; i >= 0; i-- {
		if c := &s.Colonies[i]; c.Owner != -1 {
			present[s.Planets[c.Planet].Star] |= uint16(1 << (uint8(c.Owner) & 31))
		}
	}
	for i := int(s.NumStars) - 1; i >= 0; i-- {
		st := &s.Stars[i]
		if st.IgnoreCombat == 0 {
			continue
		}
		for p := int(s.NumPlayers) - 1; p >= 0; p-- {
			if maskBit(st.IgnoreCombat, p) && uint32(present[i])&^(1<<p) == 0 {
				st.IgnoreCombat &^= 1 << p
			}
		}
	}
}
