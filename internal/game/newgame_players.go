package game

import (
	"github.com/openorion2/openorion2/internal/save"
)

// The players of a new game: Init_Players_ and what it runs. Spec:
// docs/newgame/generation.md.

// strcpyField copies a C string into a fixed field, NUL included when it
// fits (the original's strcpy never writes past these fields).
func strcpyField(dst []byte, src string) {
	k := copy(dst, src)
	if k < len(dst) {
		dst[k] = 0
	}
}

// initPlayers is Init_Players_ (0x12983): the AI players' races, banners,
// names and picks; then every player's personality, tech, designs and
// relations.
func (g *Game) initPlayers() error {
	s := &g.state
	var specials [13][]byte
	for i := range specials {
		b, err := g.raceSpecialsRecord(i)
		if err != nil {
			return err
		}
		specials[i] = b
	}
	for p := int32(0); int16(p) < s.NumPlayers; p++ {
		pl := &s.Players[p]
		if pl.Objective == save.ObjectiveHuman {
			pl.BasePicks = pl.Picks
			continue
		}
		var race int16
		for ok := false; !ok; {
			race = int16(s.Random.Random(13)) - 1
			ok = true
			for q := int16(0); q < int16(p) && ok; q++ {
				if int16(s.Players[q].Race) == race {
					ok = false
				}
			}
		}
		pl.Race = uint8(race)
		name, err := g.eStringAt(raceNames + 4*uint32(race))
		if err != nil {
			return err
		}
		strcpyField(pl.RaceName[:], name)
		for i := range pl.Picks {
			pl.Picks[i] = int8(specials[race][i])
			pl.BasePicks[i] = int8(specials[race][i])
		}
		g.raceCustomization(int(p))
		for k := 1; k < 10; k++ {
			if pl.Picks[k] != 0 {
				pl.Picks[k] = g.raceSelPickValue(k, pl.Picks[k])
			} else {
				pl.Picks[k] = 0
			}
			if pl.BasePicks[k] != 0 {
				pl.BasePicks[k] = g.raceSelPickValue(k, pl.BasePicks[k])
			} else {
				pl.BasePicks[k] = 0
			}
		}
		c := int16(s.Random.Random(8)) - 1
		for {
			used := false
			for q := int16(0); q < int16(p); q++ {
				if int16(s.Players[q].Color) == c {
					used = true
				}
			}
			if !used {
				break
			}
			c = int16((int32(c) + 1) % 8)
		}
		pl.Color = uint8(c)
		a, err := g.archive(cStringAt(g.exe, raceNamesLBX))
		if err != nil {
			return err
		}
		t, err := a.Table(0)
		if err != nil {
			return err
		}
		rec := int16(int32(s.Random.Random(8)) - 1 + int32(race)<<3)
		strcpyField(pl.EmperorName[:], save.CString(t.Record(int(rec))))
	}
	g.initNPCPersonalities()
	for p := 0; p < int(s.NumPlayers); p++ {
		pl := &s.Players[p]
		pl.OfficerForHire = -1
		pl.StagingStar = -1
		pl.Contact[p] = 1
		pl.TurnDone = 0
		pl.BC = 50
		pl.Spies[p] = 0
		pl.InfoButtons = 0x4f
		pl.Ignoring = 0
		pl.FreightersTotal = 0
		for q := 0; q < int(s.NumPlayers); q++ {
			pl.Stardates[q] = int32(s.Stardate)
		}
		g.initPlayerTech(p)
		g.initShipDesigns(p)
		pl.FuelRange = g.exeI16(fuelRangeTbl + uint32(int16(g.bestShipFuel(p)))*10)
	}
	s.Players[s.PlayerNum].Contact[s.PlayerNum] = 1
	if err := g.assignHomeStarNames(); err != nil {
		return err
	}
	g.initDiplomaticRelations()
	return nil
}

// raceCustomization is Race_Customization_ (0x150fb): on Hard and
// Impossible one of three variants of the AI race's picks.
func (g *Game) raceCustomization(p int) {
	if g.settings[0xd4] < 3 {
		return
	}
	pl := &g.state.Players[p]
	race := int32(pl.Race)
	d := int32(g.settings[0xd4]) - 3
	r := int32(g.state.Random.Random(3)) - 1
	a := uint32(race*0xaa + d*0x55 + r*0x11)
	for k := uint32(0); k < uint32(g.exeU8(raceCustomA+a)); k++ {
		i, v := g.exeU8(raceCustomA+a+1+2*k), int8(g.exeU8(raceCustomA+a+2+2*k))
		pl.Picks[i] = v
		pl.BasePicks[i] = v
	}
	b := uint32(race*0x32 + d*0x19 + r*5)
	for k := uint32(0); k < uint32(g.exeU8(raceCustomB+b)); k++ {
		pl.BasePicks[g.exeU8(raceCustomB+b+1+2*k)] = int8(g.exeU8(raceCustomB + b + 2 + 2*k))
	}
}

// weightedIndex is the pick of Init_NPC_Personalities_Objectives_Themes_:
// weights halved while their sum exceeds 1000, then Random(sum) walked down.
func (g *Game) weightedIndex(w []int32) uint8 {
	sum := int32(0)
	for _, v := range w {
		sum += v
	}
	for sum > 1000 {
		sum = 0
		for i := range w {
			w[i] /= 2
			sum += w[i]
		}
	}
	r := int32(g.state.Random.Random(uint32(sum))) - w[0]
	n := uint8(0)
	for i := 1; r > 0 && i < len(w); i++ {
		n++
		r -= w[i]
	}
	return n
}

// initNPCPersonalities is Init_NPC_Personalities_Objectives_Themes_
// (0x589d6): personality, objective (kept for humans) and the two ship
// themes, weighted by the race's picks and the difficulty.
func (g *Game) initNPCPersonalities() {
	s := &g.state
	diff := int32(g.settings[0xd4])
	for p := 0; p < int(s.NumPlayers); p++ {
		pl := &s.Players[p]
		roll := int32(s.Random.Random(10)) + 1 - diff
		if roll < 0 {
			roll = 0
		} else if roll > 9 {
			roll = 9
		}
		pl.Personality = g.exeU8(personalityTbl + uint32(roll) + uint32(pl.Race)*10)
		o := []int32{1, 2, 1, 2, 2, 1}     // objectives
		t := []int32{2, 1, 2, 1}           // ship themes
		sp := []int32{2, 2, 1, 2, 1, 2, 3} // special themes
		pk := &pl.Picks
		if pk[1] == 0x32 {
			o[5], o[2] = 0xb, 0xb
		}
		if pk[1] == 0x64 {
			o[5] += 100
			o[2] += 100
		}
		add3 := func(a, b, c int, v int32) { o[a] += v; o[b] += v; o[c] += v }
		if pk[2] == 1 {
			add3(5, 4, 3, 3)
		}
		if pk[2] == 2 {
			add3(5, 4, 3, 100)
		}
		if pk[3] == 1 {
			add3(5, 4, 3, 3)
		}
		if pk[3] == 2 {
			add3(5, 4, 3, 10)
		}
		if pk[4] == 1 {
			o[4] += 3
			o[3] += 10
		}
		if pk[4] == 2 {
			o[4] += 10
			o[3] += 100
		}
		if pk[6] == 0x14 {
			o[1] += 10
			sp[3] += 100
			t[1] += 10
		}
		if pk[6] == 0x28 {
			o[1] += 100
			sp[3] += 1000
			t[1] += 100
		}
		if pk[7] == 0x19 {
			o[1] += 10
			sp[4] += 3
			t[0] += 100
		}
		if pk[7] == 0x32 {
			o[1] += 100
			sp[4] += 10
			t[0] += 1000
		}
		if pk[8] == 0xa {
			o[2] += 10
			sp[1] += 100
		}
		if pk[8] == 0x14 {
			o[2] += 100
			sp[1] += 1000
		}
		if pk[9] == 0xa {
			o[0] += 3
		}
		if pk[9] == 0x14 {
			o[0] += 10
		}
		if pk[10] == 1 {
			o[4] += 10
			o[5] += 3
			o[3] += 3
		}
		if pk[11] == 1 {
			o[2] += 10
			sp[1] += 1000
		}
		if pk[12] == 1 {
			add3(4, 5, 3, 3)
		}
		if pk[13] == 1 {
			o[4] += 10
			o[5] += 3
			o[3] += 3
		}
		if pk[14] == 1 {
			o[4] += 3
			o[5] += 10
		}
		if pk[15] == 1 {
			o[4] += 10
		}
		if pk[17] == 1 {
			o[4] += 3
			sp[5] += 1000
			t[0] += 10
		}
		if pk[18] == 1 {
			o[4] += 3
			o[2] += 10
			o[3] += 3
		}
		if pk[20] == 1 {
			o[0] += 100
		}
		if pk[22] == 1 {
			o[3] += 1000
		}
		if pk[23] == 1 {
			o[4] += 100
		}
		if pk[24] == 1 {
			o[0] += 100
		}
		if pk[28] == 1 {
			o[1] += 100
		}
		if pk[30] == 1 {
			o[2] += 10
		}
		if pk[29] == 1 {
			o[2] += 10
			o[1] += 13
			sp[3] += 100
		}
		if diff == 3 {
			add3(4, 2, 3, 3)
		}
		if diff == 4 {
			add3(4, 2, 3, 10)
		}
		if pl.Personality == 0 {
			sp[2] += 3
		}
		obj := g.weightedIndex(o)
		if pl.Objective != save.ObjectiveHuman {
			pl.Objective = obj
		} else {
			pl.Objective = save.ObjectiveHuman
		}
		pl.ShipTheme = g.weightedIndex(t)
		pl.SpecialTheme = g.weightedIndex(sp)
	}
}

// techLegalForPlayer is Tech_Is_Legal_For_Player_ (0x5e481).
func (g *Game) techLegalForPlayer(p int, app int16) bool {
	pl := &g.state.Players[p]
	ok := true
	if pl.Picks[save.PickGovernment] == 6 && (app == 0x56 || app == 0xc3 || app == 0x8d) {
		ok = false
	}
	if pl.Picks[save.PickTolerant] != 0 && (app == 0x8e || app == 0x13 || app == 0x32 || app == 0x71) {
		ok = false
	}
	if pl.Picks[save.PickLithovore] != 0 {
		switch app {
		case 6, 0x1d, 0x44, 0x57, 0xa2, 0xb2:
			ok = false
		}
	}
	if g.settings[0xd8] == 1 && g.exeU8(uint32(appFieldTbl+5+int(app)*13)) == 0 {
		ok = false
	}
	if app == 0x34 && g.settings[0xd3] == 0 {
		ok = false
	}
	return ok
}

// fieldApp is application j (0..3) of a field's row.
func (g *Game) fieldApp(f int16, j int) int16 {
	return int16(g.techFieldApp(int(f), j))
}

// initPlayerTech is Init_Player_Tech_ (0x5e55f): the starting fields and
// applications, then the tech level's head start (1, 6 or 25 fields).
func (g *Game) initPlayerTech(p int) {
	s := &g.state
	pl := &s.Players[p]
	for _, f := range []int{0x12, 0x37, 0x39, 0x1d, 0x07, 0x16, 0x1c, 0x0a} {
		pl.FieldStatus[f] = 2
	}
	var n int16
	switch g.settings[0xd9] {
	case 0:
		n = 1
	case 1:
		n = 6
	case 2:
		n = 0x19
	}
	pl.FieldStatus[0] = 3
	for a := int16(1); a < 0xd4; a++ {
		f := g.exeI16(appFieldTbl + uint32(a)*13)
		if f == 6 {
			continue
		}
		if f == 0 {
			pl.AppStatus[a] = 3
		} else {
			legal := pl.Picks[save.PickUncreative] == 0 || f == -1 ||
				g.exe.u32(uint32(fieldCost+int(f)*0x17)) == 0x32 || f == 0x17
			if legal && g.techLegalForPlayer(p, a) {
				pl.AppStatus[a] = 1
			} else {
				pl.AppStatus[a] = 0
			}
		}
		if a == 0x34 && g.settings[0xd3] == 0 {
			pl.AppStatus[a] = 0
		}
	}
	for i := 0; i < 4 && g.exeI16(illegalStartApp+uint32(2*i)) > 0; i++ {
		pl.AppStatus[g.exeI16(illegalStartApp+uint32(2*i))] = 0
	}
	switch pl.Picks[save.PickGovernment] {
	case 0:
		pl.AppStatus[42] = 1
	case 2:
		pl.AppStatus[92] = 1
	case 4:
		pl.AppStatus[65] = 1
	case 6:
		pl.AppStatus[77] = 1
		pl.AppStatus[86] = 0
		pl.AppStatus[141] = 0
		pl.AppStatus[195] = 0
	}
	pl.AppStatus[0] = 0
	for c := int16(1); c < 0x4a; c++ {
		if pl.FieldStatus[c] == 0 {
			pl.FieldStatus[c] = 1
		}
		cnt, found := int32(0), false
		app := int16(1)
		for j := 0; j < 4 && app != 0; j++ {
			app = g.fieldApp(c, j)
			if pl.AppStatus[app] > 0 {
				found = true
			} else if app != 0 {
				cnt++
			}
		}
		if found {
			continue
		}
		for {
			app = g.fieldApp(c, int(int16(int32(s.Random.Random(uint32(int16(cnt))))-1)))
			if g.techLegalForPlayer(p, app) {
				break
			}
		}
		pl.AppStatus[app] = 1
	}
	pl.FieldStatus[74] = 0
	for c := 0x4b; c < 0x53; c++ {
		if pl.FieldStatus[c] == 0 {
			pl.FieldStatus[c] = 1
		}
	}
	human := pl.Objective == save.ObjectiveHuman
	if human {
		pl.Objective = 1
	}
	for i := int16(0); i < n; i++ {
		var f int16
		if i < 6 {
			f = g.exeI16(startFields + uint32(2*i))
			pl.FieldStatus[f] = 3
			pl.FieldStatus[g.exeI16(fieldRow+uint32(int32(f)*fieldRowSize))] = 2
			for j := 0; j < 4 && g.fieldApp(f, j) != 0; j++ {
				if a := g.fieldApp(f, j); g.techLegalForPlayer(p, a) {
					pl.AppStatus[a] = 3
				}
			}
			continue
		}
		a := g.chooseTechApplication(p)
		f = g.exeI16(appFieldTbl + uint32(a)*13)
		pl.FieldStatus[f] = 3
		pl.FieldStatus[g.exeI16(fieldRow+uint32(int32(f)*fieldRowSize))] = 2
		if pl.Picks[save.PickCreative] != 0 {
			for j := 0; j < 4 && g.fieldApp(f, j) != 0; j++ {
				if b := g.fieldApp(f, j); g.techLegalForPlayer(p, b) {
					pl.AppStatus[b] = 3
				}
			}
		} else {
			pl.AppStatus[a] = 3
		}
		if f == 6 {
			pl.Picks[save.PickGovernment]++
		}
	}
	if human {
		pl.Objective = save.ObjectiveHuman
	}
	pl.BestWarpDrive = uint8(g.bestWarpDrive(p))
	pl.FTLSpeed = uint8(g.calcPlayerFTLSpeed(p))
}

// assignHomeStarNames is Assign_Home_Star_Names_ (0x7ac71): each race's
// home star name to the star of the player's home planet, once per race.
func (g *Game) assignHomeStarNames() error {
	s := &g.state
	a, err := g.archive(cStringAt(g.exe, homeStarNameLBX))
	if err != nil {
		return err
	}
	t, err := a.Table(0)
	if err != nil {
		return err
	}
	var done [14]bool
	for p := 0; p < int(s.NumPlayers); p++ {
		pl := &s.Players[p]
		star := s.Planets[pl.HomePlanet].Star
		if done[pl.Race] {
			continue
		}
		strcpyField(s.Stars[star].Name[:], save.CString(t.Record(int(pl.Race))))
		done[pl.Race] = true
	}
	return nil
}

// initDiplomaticRelations is Init_Diplomatic_Relations_ (0x4d78e).
func (g *Game) initDiplomaticRelations() {
	s := &g.state
	n := int(s.NumPlayers)
	for p := 0; p < n; p++ {
		pl := &s.Players[p]
		for q := 0; q < n; q++ {
			rel := int8(g.exeU8(baseRelations + uint32(pl.Race)*0x1c + uint32(s.Players[q].Race)*2))
			pl.TreatyStatus[q], pl.TradeTreaty[q], pl.ResearchTreaty[q] = 0, 0, 0
			pl.Relations[q], pl.BaseRelations[q] = rel, rel
			pl.WarDeclared[q] = 0
			pl.MessageParam[q] = 0
			pl.MessageThisTurn[q] = 0
			pl.Diplo66F[q], pl.Diplo67F[q] = 0, 0
			for m := range pl.Modifiers {
				pl.Modifiers[m][q] = 0
			}
			pl.Diplo6CF[q], pl.Diplo6D7[q] = 0, 0
			pl.DemandState[q] = 0
			pl.Diplo7EE[q], pl.Diplo7F6[q] = 0, 0xff
			pl.ThreatTurn[q] = 0
			pl.Diplo7CC[q] = 0
			pl.Diplo7DC[q], pl.Diplo7E4[q] = 0, 0xff
			pl.TreatyTimer[q] = 0
			pl.Tribute[q] = 0
			pl.WarLosses[q], pl.WarLosses2[q] = 0, 0
			pl.Diplo70F[q], pl.TurnsAtWar[q], pl.Diplo727[q] = 0, 0, 0
			pl.PrevTreatyStatus[q] = 0
			pl.MessageFlags[q] = 0
			for m := range pl.TurnScratch {
				pl.TurnScratch[m][q] = 0
			}
			pl.Demand[q] = 0
			pl.AllianceCall[q] = 0
			pl.SneakStar[q] = -1
			pl.DelayedOrder[q] = -1
			pl.Diplo816 = 0
			g.clearLastProposalTurn(int16(p), int16(q))
		}
		pl.CouncilVote = -1
		pl.Diplo7C9[2] = 0xff
		pl.AIWarTarget = -1
		pl.Diplo7C9[0] = 0
		pl.LuckyCounter = 0
	}
	for a := 0; a < n; a++ {
		if s.Players[a].Objective == save.ObjectiveHuman {
			continue
		}
		for b := a; b < n; b++ {
			if s.Players[b].Objective != save.ObjectiveHuman {
				continue
			}
			var v int16
			switch s.Players[a].Personality {
			case 0:
				v = -3
			case 1:
				v = -2
			case 4:
				v = 2
			case 5:
				v = 5
			}
			s.Players[a].HumanBias[b] = v
			s.Players[b].HumanBias[a] = v
		}
	}
	s.CouncilFlag = 0
	s.CouncilWinner = -1
	s.NextCouncilTurn = 0x19
}
