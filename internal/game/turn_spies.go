package game

import "github.com/openorion2/openorion2/internal/save"

// Spies (SPY.C, the accessors in BILL.C): Resolve_Spies_, step 19 of
// Next_Turn_Calc_. Spec: docs/turn/overview.md, D4.

// Constant tables of ORION2.EXE (ELF addresses) the spy code reads.
const (
	spyRaceDefence = 0x100a36 // s8 by government pick
	techFieldCost  = techFieldTable + 0x12
)

// theirSpyNumber is Get_Their_Spy_Number_ (0x1026cf): p's spies in k's
// empire, at home for k == p.
func (s *State) theirSpyNumber(p, k int) int16 {
	return int16(s.Players[p].Spies[k] & 0x3f)
}

// setTheirSpyNumber is Set_Their_Spy_Number_ (0x10278d); the mission bits
// stay.
func (s *State) setTheirSpyNumber(p, k int, n int16) {
	b := &s.Players[p].Spies[k]
	*b = *b&0xc0 | uint8(n)&0x3f
}

// theirSpyMission is Get_Their_Spy_Mission_ (0x1026f1): 0 steal,
// 1 sabotage, 2 hide.
func (s *State) theirSpyMission(p, k int) int16 {
	return int16(s.Players[p].Spies[k] >> 6)
}

// setTheirSpyMission is Set_Their_Spy_Mission_ (0x1027b5).
func (s *State) setTheirSpyMission(p, k int, m int16) {
	b := &s.Players[p].Spies[k]
	*b = *b&0x3f | uint8(m)<<6
}

// addTheirSpy is Add_Their_Spy_ (0x10294b): one more spy, which also clears
// the mission; 0 when already at 63.
func (s *State) addTheirSpy(p, k int) int16 {
	n := s.theirSpyNumber(p, k) + 1
	if n > 0x3f {
		return 0
	}
	s.Players[p].Spies[k] = uint8(n)
	return n
}

// resolveSpies is Resolve_Spies_ (0x10192b).
func (g *Game) resolveSpies() {
	s := &g.state
	g.resolveAssassins()
	var live []uint8
	for p := s.NumPlayers - 1; p >= 0; p-- {
		if s.Players[p].Eliminated != 0 {
			continue
		}
		live = append(live, uint8(p))
		g.spyBonus[p][0], g.spyBonus[p][1] = g.computeSpyBonuses(int(p))
	}
	shuffleChar(&s.Random, live)
	for i := len(live) - 1; i >= 0; i-- {
		if p := int(live[i]); s.Players[p].Objective != 100 {
			g.allocateAISpies(p)
		}
	}
	shuffleChar(&s.Random, live)
	for i := len(live) - 1; i >= 0; i-- {
		g.resolvePlayerSpies(int(live[i]))
	}
}

// resolveAssassins is Resolve_Assassins_ (0x1018a3). The original tests
// skill bit 1 twice, so the weaker assassin branch never runs.
func (g *Game) resolveAssassins() {
	s := &g.state
	for o := maxLeaders - 1; o >= 0; o-- {
		l := &s.Leaders[o]
		if l.Owner == -1 || (l.Status != 0 && l.Status != 1) {
			continue
		}
		var chance int32
		if l.CommonSkills&2 != 0 {
			chance = (int32(s.ownedOfficerLevel(o)) + 1) * 3
		} else if l.CommonSkills&2 != 0 {
			chance = int32(s.ownedOfficerLevel(o))*2 + 2
		}
		if chance != 0 && int32(s.Random.Random(100)) <= chance {
			g.leaderAssassinates(o)
		}
	}
}

// leaderAssassinates is Leader_Assassinates_ (0x101768): one spy of a
// player spying on the leader's owner dies, picked by spy count; failing
// that, a home spy of a player the owner spies on.
func (g *Game) leaderAssassinates(o int) {
	s := &g.state
	owner := int(s.Leaders[o].Owner)
	victim, from := -1, -1
	var n1, n2 int32
	for k := int(s.NumPlayers) - 1; k >= 0; k-- {
		if k == owner || s.Players[k].Eliminated != 0 {
			continue
		}
		if n := int32(s.theirSpyNumber(k, owner)); n > 0 {
			n1 += n
			if int32(s.Random.Random(uint32(n1))) <= n {
				victim, from = k, owner
			}
		}
		if n1 != 0 || s.theirSpyNumber(owner, k) <= 0 {
			continue
		}
		if n := int32(s.theirSpyNumber(k, k)); n > 0 {
			n2 += n
			if int32(s.Random.Random(uint32(n2))) <= n {
				victim, from = k, k
			}
		}
	}
	if victim == -1 {
		return
	}
	// Byte 1 (priority) and bytes 6..17 are stale stack in the original.
	var m [msgSize]byte
	m[0], m[2], m[3], m[4], m[5] = 0xf, uint8(victim), uint8(from), uint8(o), uint8(owner)
	for _, q := range []int{owner, victim} {
		i := g.addMsg(q, m)
		g.markStaleMsg(i, 1, 2)
		g.markStaleMsg(i, 6, msgSize)
	}
	s.setTheirSpyNumber(victim, from, s.theirSpyNumber(victim, from)-1)
}

// techSpyBonus is Tech_Spy_Bonus_ (0x100a3e).
func techSpyBonus(apps *[212]uint8) int16 {
	var v int16
	for _, a := range []struct {
		app int
		v   int16
	}{{0x2b, 10}, {0x72, 10}, {0x93, 10}, {0xad, 10}, {0xb6, 5}} {
		if apps[a.app] == techKnown {
			v += a.v
		}
	}
	return v
}

// computeSpyBonuses is Compute_Spy_Bonuses_ (0x100a83): offence and
// defence from technology, race picks and the best spy leaders.
func (g *Game) computeSpyBonuses(p int) (off, def int16) {
	s := &g.state
	pl := &s.Players[p]
	base := int32(techSpyBonus(&pl.AppStatus)) + int32(pl.Picks[9]) + int32(pl.Picks[25])*10
	var bestOff, bestDef int32
	for o := maxLeaders - 1; o >= 0; o-- {
		l := &s.Leaders[o]
		if int32(l.Owner) != int32(p) || uint8(l.Status) >= 3 || s.leaderAtAnomaly(o) {
			continue
		}
		var v int32
		if l.CommonSkills&0x8000 != 0 {
			v = (int32(s.ownedOfficerLevel(o)) + 1) * 3
		} else if l.CommonSkills&0x4000 != 0 {
			v = int32(s.ownedOfficerLevel(o))*2 + 2
		}
		bestOff = max(bestOff, v)
		v = 0
		if l.CommonSkills&0x20000 != 0 {
			v = int32(s.ownedOfficerLevel(o)) * 3
		} else if l.CommonSkills&0x10000 != 0 {
			v = int32(s.ownedOfficerLevel(o)) * 2
		}
		bestDef = max(bestDef, v)
	}
	race := int32(int8(g.exeU8(uint32(int32(spyRaceDefence) + int32(pl.Picks[0])))))
	return int16(base + bestOff), int16(base + bestDef + race)
}

// aiContacts counts the players p has contact with, as Compute_AI_Data_
// stores them at _ai_player_info[p]+0xa.
func (s *State) aiContacts(p int) int16 {
	var n int16
	for k := int(s.NumPlayers) - 1; k >= 0; k-- {
		if k != p && s.Players[p].Contact[k] != 0 {
			n++
		}
	}
	return n
}

// allocateAISpies is Allocate_AI_Spies_ (0x100d19): an AI calls all its
// spies home, may train one more, keeps enough at home against the largest
// enemy force and sends the rest after the most valuable technology.
func (g *Game) allocateAISpies(p int) {
	s := &g.state
	pl := &s.Players[p]
	diff := g.settings[0xd4]
	if pl.Eliminated != 0 || diff == 0 {
		return
	}
	var total, aiMax, humanMax int32
	for k := int(s.NumPlayers) - 1; k >= 0; k-- {
		theirs := s.theirSpyNumber(k, p)
		total += int32(s.theirSpyNumber(p, k))
		s.setTheirSpyNumber(p, k, 0)
		if theirs <= 0 || k == p {
			continue
		}
		if s.Players[k].Objective == 100 {
			humanMax = max(humanMax, int32(theirs))
		} else {
			aiMax = max(aiMax, int32(theirs))
		}
	}
	if total == 0 && int16(g.ai.u16(g.ai.at(aiPlayerInfo)+p*0x3a+0xa)) > 0 && s.Stardate > 0x88d6 {
		switch diff {
		case 4:
			// The draws decide nothing; a spy is trained regardless.
			if pl.Objective != 0 {
				s.Random.Random(8)
			}
			if pl.Picks[9] > 0 {
				s.Random.Random(5)
			}
			total = 1
		case 3:
			switch {
			case pl.Objective == 0 && s.Random.Random(7) == 1,
				s.Random.Random(25) == 1,
				pl.Picks[9] > 0 && s.Random.Random(6) == 1:
				total = 1
			}
		case 2:
			switch {
			case pl.Objective == 0 && s.Random.Random(13) == 1,
				pl.Picks[9] > 0 && s.Random.Random(13) == 1:
				total = 1
			}
		}
	}
	if total <= 0 {
		return
	}
	home := total
	if g.spyBonus[p][0] >= 0 {
		keep := humanMax
		if diff <= 2 {
			keep = max(humanMax, aiMax)
		}
		if total >= keep {
			home = keep
		}
	}
	home = min(home, 0x3f)
	s.setTheirSpyNumber(p, p, int16(home))
	if home >= total {
		return
	}
	var w [8]uint32
	left := total - home
	for k := int(s.NumPlayers) - 1; k >= 0; k-- {
		if k == p || pl.Contact[k] == 0 || pl.Diplo806[k] != 0 {
			continue
		}
		theirHome := int32(s.theirSpyNumber(k, k))
		var v uint32
		for a := 0; a < numTechApps; a++ {
			if s.Players[k].AppStatus[a] != techKnown || !g.playerMayGetTechApp(p, a) {
				continue
			}
			if f := int32(g.exeI16(uint32(techAppTable + a*techAppRow + techAppField))); f != 0 {
				v += g.exe.u32(uint32(int32(techFieldCost) + f*techFieldRow))
			}
		}
		if v == 0 {
			continue
		}
		if theirHome > 0 {
			var others int32
			for j := int(s.NumPlayers) - 1; j >= 0; j-- {
				if j != k {
					others += int32(s.theirSpyNumber(j, k))
				}
			}
			if int16(others) > 0 {
				v = uint32((int32(int16(others))+left)*int32(v)) / uint32(left)
			}
			def := int32(g.spyBonus[k][1])
			v = uint32((100-def)*int32(v)) / 100
			if def >= 25 {
				v = uint32((125-def)*int32(v)) / 100
			}
			v /= uint32(theirHome)
		}
		if left > theirHome {
			v *= uint32(left - theirHome)
		}
		w[k] = v + 1
	}
	for left > 0 {
		k := int(getWeightedChoiceInt(&s.Random, w[:s.NumPlayers]))
		if k == -1 {
			s.setTheirSpyNumber(p, p, int16(min(home+left, 0x3f)))
			left = 0
			continue
		}
		if left <= 0x3f {
			s.setTheirSpyNumber(p, k, int16(left))
			left = 0
		} else {
			s.setTheirSpyNumber(p, k, 0x3f)
			left -= 0x3f
		}
		if pl.Personality == 3 && s.Random.Random(8) == 1 {
			s.setTheirSpyMission(p, k, 1)
		} else {
			s.setTheirSpyMission(p, k, 0)
		}
	}
}

// nSpiesBonus is N_Spies_Bonus_ (0x101483).
func nSpiesBonus(n int32) int32 {
	switch {
	case n <= 5:
		return n * 2
	case n <= 10:
		return n + 5
	}
	return (n-10)/2 + 15
}

// resolvePlayerSpies is Resolve_Player_Spies_ (0x1014a4): every player with
// spies in p's empire tries its mission and may be caught; p's defenders
// may be lost.
func (g *Game) resolvePlayerSpies(p int) {
	s := &g.state
	home := int32(s.theirSpyNumber(p, p))
	homeBonus := nSpiesBonus(home)
	var lost int32
	for k := int(s.NumPlayers) - 1; k >= 0; k-- {
		if k == p {
			continue
		}
		n := int32(s.theirSpyNumber(k, p))
		kb := nSpiesBonus(n)
		if n == 0 {
			continue
		}
		r1 := int32(s.Random.Random(100))
		r2 := int32(s.Random.Random(100))
		v := kb + r1 + int32(g.spyBonus[k][0]) - (r2 + int32(g.spyBonus[p][1]) + homeBonus)
		if s.Players[k].Objective != 100 && s.Players[p].Objective == 100 {
			v += int32(g.settings[0xd4]) - 2
		}
		switch s.theirSpyMission(k, p) {
		case 0:
			if (v >= 60 || r1 == 100) && (v >= 80 || r1 == 100) {
				g.stealApp(k, p, v >= 90 || r1 == 100)
			}
		case 1:
			if v >= 70 || r1 == 100 {
				framed := int16(k)
				if v >= 90 || r1 == 100 {
					g.frameAnother(k, p, &framed, -1)
				}
				g.destroyRandomBuilding(k, p, framed)
			}
		}
		r3 := int32(s.Random.Random(100))
		r4 := int32(s.Random.Random(100))
		v = r3 + int32(g.spyBonus[k][0]) + kb - (r4 + int32(g.spyBonus[p][1]) + homeBonus + 20)
		if s.theirSpyMission(k, p) == 2 {
			v += 20
		}
		if v >= 80 || r3 == 100 {
			lost++
		}
		if v <= -80 || r4 == 100 {
			s.setTheirSpyNumber(k, p, int16(n-1))
			var m [msgSize]byte
			m[0] = 0xa
			m[2], m[3] = uint8(p), uint8(p>>8)
			g.addMsg(k, m)
		}
	}
	if lost <= 0 {
		return
	}
	left := max(home-lost, 0)
	s.setTheirSpyNumber(p, p, int16(left))
	if killed := home - left; killed > 0 {
		var m [msgSize]byte
		m[0] = 0xb
		m[2], m[3] = uint8(killed), uint8(killed>>8)
		g.addMsg(p, m)
	}
}

// frameAnother is Frame_Another_ (0x100bc5): a third player in contact
// with both (without app, if one is given) to blame, preferring those the
// thief is at war with; out is kept when there is none.
func (g *Game) frameAnother(thief, victim int, out *int16, app int) bool {
	s := &g.state
	var count [7]uint8
	pick := [7]int8{-1, -1, -1, -1, -1, -1, -1}
	var n int32
	for j := int(s.NumPlayers) - 1; j >= 0; j-- {
		J := &s.Players[j]
		if j == thief || j == victim || J.Contact[thief] == 0 || J.Contact[victim] == 0 {
			continue
		}
		if app != -1 && J.AppStatus[app] == techKnown {
			continue
		}
		b := s.Players[thief].TreatyStatus[j]
		count[b]++
		n++
		if s.Random.Random(uint32(count[b])) == 1 {
			pick[b] = int8(j)
		}
	}
	for _, b := range []int{6, 5, 4, 0, 1, 3, 2} {
		if pick[b] != -1 {
			*out = int16(pick[b])
			break
		}
	}
	return n > 0
}

// stealApp is Steal_App_ (0x10119c): thief learns an application the
// victim knows, picked by field cost; relations and two reports follow.
func (g *Game) stealApp(thief, victim int, frame bool) {
	s := &g.state
	T := &s.Players[thief]
	pick := int16(-1)
	var total int32
	for a := int16(0); a < numTechApps; a++ {
		if s.Players[victim].AppStatus[a] != techKnown || !g.playerMayGetTechApp(thief, int(a)) {
			continue
		}
		f := int32(g.exeI16(uint32(techAppTable + int32(a)*techAppRow + techAppField)))
		if f == 0 || int32(T.ResearchField) == f {
			continue
		}
		if c := int32(g.exe.u32(uint32(int32(techFieldCost) + f*techFieldRow))); c > 0 {
			total += c
			if int32(s.Random.Random(uint32(total))) <= c {
				pick = a
			}
		}
	}
	if pick < 0 {
		return
	}
	framed := int16(thief)
	g.playerGetsTechApp(thief, int(pick))
	if frame {
		g.frameAnother(thief, victim, &framed, int(pick))
	}
	typ := int16(1)
	if int32(framed) != int32(thief) {
		typ = 2
	}
	r := -int32(s.Random.Random(15))
	r -= int32(s.Random.Random(5))
	g.changeRelations(int16(r), int(framed), victim, typ, 0, pick)
	blame := uint8(framed)
	if int32(framed) == int32(thief) {
		blame = 0xff
	}
	// Bytes 8..18 of both reports are stale stack in the original.
	var rep [reportSize]byte
	rep[0], rep[1], rep[2], rep[3], rep[4], rep[5] = 1, uint8(thief), uint8(thief>>8), 0, blame, uint8(victim)
	rep[6], rep[7] = uint8(pick), uint8(pick>>8)
	g.markStaleReport(s.addReport(rep), 8)
	rep[0], rep[1], rep[2], rep[3] = 2, uint8(victim), uint8(victim>>8), 0
	rep[4], rep[5] = uint8(framed), uint8(framed>>8)
	g.markStaleReport(s.addReport(rep), 8)
}

// destroyRandomBuilding is Destroy_Random_Building_ (0x10130a): one of the
// victim's buildings other than the capitol, picked by cost.
func (g *Game) destroyRandomBuilding(thief, victim int, framed int16) {
	s := &g.state
	col, b := -1, -1
	var total int32
	for i := 0; i < int(s.NumColonies); i++ {
		c := &s.Colonies[i]
		if int16(c.Owner) != int16(victim) {
			continue
		}
		for k := 0; k < 0x31; k++ {
			if k == 9 || c.Buildings[k] == 0 {
				continue
			}
			cost := int32(g.exe.u32(uint32(0x17eb45 + k*0x13)))
			total += cost
			if int32(s.Random.Random(uint32(total))) <= cost {
				col, b = i, k
			}
		}
	}
	if col < 0 {
		return
	}
	g.removeBuilding(col, b)
	planet := s.Colonies[col].Planet
	typ := int16(3)
	if int32(framed) != int32(thief) {
		typ = 4
	}
	r := -int32(s.Random.Random(15))
	r -= int32(s.Random.Random(5))
	g.changeRelations(int16(r), int(framed), victim, typ, planet, int16(b))
	blame := uint8(framed)
	if int32(framed) == int32(thief) {
		blame = 0xff
	}
	// Bytes 8..18 of both reports are stale stack in the original.
	var rep [reportSize]byte
	rep[0], rep[1], rep[2], rep[3] = 1, uint8(thief), uint8(thief>>8), 1
	rep[4], rep[5], rep[6], rep[7] = blame, uint8(victim), uint8(planet), uint8(b)
	g.markStaleReport(s.addReport(rep), 8)
	rep[0], rep[1], rep[2] = 2, uint8(victim), uint8(victim>>8)
	rep[4], rep[5] = uint8(framed), uint8(framed>>8)
	g.markStaleReport(s.addReport(rep), 8)
}

// reportSize is the size of one report record.
const reportSize = 0x13

// addReport is Add_Report_ (0xfd846): appended while fewer than 50; it
// returns the record index, -1 for none.
func (s *State) addReport(r [reportSize]byte) int {
	if s.NReports >= 50 {
		return -1
	}
	i := int(s.NReports)
	copy(s.Reports[i*reportSize:], r[:])
	s.NReports++
	return i
}

// markStaleReport records stale bytes [lo, 19) of report i.
func (g *Game) markStaleReport(i, lo int) {
	if i >= 0 {
		g.stale = append(g.stale, staleSpan{save.BReports, i*reportSize + lo, reportSize - lo})
	}
}
