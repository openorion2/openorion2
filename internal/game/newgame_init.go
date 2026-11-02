package game

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/openorion2/openorion2/internal/save"
)

// The galaxy generation, MOX\HAROLD\INITGAME.C and the routines it runs.
// Spec: docs/newgame/generation.md.

const (
	skillCount      = 0x36     // rows of the skill table (tblSkillData)
	appFieldTbl     = 0x17e07f // i16 field of an application, 13-byte rows
	fieldRow        = 0x17d90c // 0x17-byte field rows: +0 next field, +2 four apps, +0xa cost
	fieldRowSize    = 0x17
	startFields     = 0x18111c // i16 fields an advanced start researches first
	illegalStartApp = 0x17d998 // i16 apps nobody starts with, up to 4
	personalityTbl  = 0x181090 // personality by race*10 + roll
	raceCustomA     = 0x180184 // Race_Customization_'s picks for both sets
	raceCustomB     = 0x180a26 // and for the base picks only
	baseRelations   = 0x180ed4 // starting relations by race*0x1c + race*2
	fuelRangeTbl    = 0x17ffea // i16 fuel range by Best_Ship_Fuel_*10
	raceNamesLBX    = 0x178983 // "RACENAME.LBX"
	homeStarNameLBX = 0x179534 // the home stars' names, 13 of 0xf bytes
)

// genSeed is Set_Game_Random_Seed_ (0x107ca): the command line seed, else
// Randomize_ (the clock, or Options.Clock).
func (g *Game) genSeed() {
	if g.cmdLineSeed != 0 {
		g.state.Random.Set(g.cmdLineSeed)
		return
	}
	g.randomize()
}

// randomize is Randomize_ (0x124878): time(NULL) as the seed.
func (g *Game) randomize() {
	g.state.Random.Randomize()
}

// setUniverseSeed stores Get_Random_Seed_ in _settings +0xda.
func (g *Game) setUniverseSeed() {
	binary.LittleEndian.PutUint32(g.settings[0xda:], g.state.Random.Get())
}

// initShipsArray is Init_Ships_Array_ (0x12268).
func (g *Game) initShipsArray() {
	s := &g.state
	s.Ships = [maxShips]save.Ship{}
	for i := range s.Ships {
		s.Ships[i].Status = 5
		s.Ships[i].Location = -1
		s.Ships[i].Officer = -1
	}
	s.NumShips = maxShips
	s.NumShips = 0
	g.allocateNewShipSlots()
}

// initColoniesArray is Init_Colonies_Array_ (0x1241a).
func (g *Game) initColoniesArray() {
	s := &g.state
	s.Colonies = [maxColonies]save.Colony{}
	for i := range s.Colonies {
		s.Colonies[i].Planet = -1
		s.Colonies[i].Owner = -1
		s.Colonies[i].Unknown004 = -1
	}
	s.NumColonies = 0
}

// initLeaders is Init_Leaders_ (0x1307f): the 67 leaders from the officer
// data, random skills for leaders 0x29 and 0x34, random techs for 0x41.
func (g *Game) initLeaders() error {
	s := &g.state
	a, err := g.archive(g.officerDataLBX())
	if err != nil {
		return err
	}
	t, err := a.Table(0)
	if err != nil {
		return err
	}
	if t.Size != 0x3b {
		return fmt.Errorf("%s: record size %d", g.officerDataLBX(), t.Size)
	}
	for i := range s.Leaders {
		if err := setLeaderBytes(&s.Leaders[i], t.Record(i)); err != nil {
			return err
		}
	}
	for i := 0; i < leaderCount; i++ {
		l := &s.Leaders[i]
		switch i {
		case 0x29:
			g.getRandomSkills(&l.SpecialSkills, &l.CommonSkills, 2, 5, true)
		case 0x34:
			g.getRandomSkills(&l.SpecialSkills, &l.CommonSkills, 1, 5, true)
		case 0x41:
			l.Techs[0] = uint8(s.Random.Random(0x66)) - 1
			l.Techs[1] = uint8(s.Random.Random(0x66)) + 0x65
		}
		l.SkillValue = g.officerSkillValue(i)
	}
	return nil
}

// skillRow reads row k of the skill table: mask, kind (0 common, 1 ship,
// 2 colony), value.
func (g *Game) skillRow(k int) (uint32, uint8, uint8) {
	row := uint32(tblSkillData + k*tblSkillDataRow)
	return g.exe.u32(row + 1), g.exeU8(row + 5), g.exeU8(row + 9)
}

// getRandomSkills is Get_Random_Skills_ (0x93eb2): n skills in pairs of
// levels, common ones (kind 0) or of the kind; with mixed 3 in 7 are common.
func (g *Game) getRandomSkills(special, common *uint32, kind int16, n int16, mixed bool) {
	r := &g.state.Random
	*special, *common = 0, 0
	for k := int32(0); int16(k) < n; k++ {
		pick := int16(0)
		cur := kind
		cnt := int16(0)
		if mixed && r.Random(7) < 3 {
			cur = 0
		}
		for b := int16(0); b < skillCount; b += 2 {
			m0, kd, _ := g.skillRow(int(b))
			m1, _, _ := g.skillRow(int(b) + 1)
			set := *special
			if cur == 0 {
				if kd != 0 {
					continue
				}
				set = *common
			} else if int16(kd) != kind {
				continue
			}
			if set&m0 != 0 || set&m1 != 0 {
				continue
			}
			cnt++
			if r.Random(uint32(int32(cnt))) == 1 {
				if r.Random(2) == 1 {
					pick = b
				} else {
					pick = b + 1
				}
			}
		}
		m, _, _ := g.skillRow(int(pick))
		if cur == 0 {
			*common += m
		} else {
			*special += m
		}
	}
}

// officerSkillValue is Officer_Skill_Value_ (0x946b7): the worth of a
// leader's skills and techs. Only the low word is kept by its callers.
func (g *Game) officerSkillValue(o int) int16 {
	l := &g.state.Leaders[o]
	var v int16
	for k := 0; k < skillCount; k++ {
		_, kd, val := g.skillRow(k)
		if kd == 0 && g.officerHasGeneralSkill(o, int16(k)) {
			a := int16(val)
			if k == 2 || k == 3 {
				a = 1
				if l.Type == 0 {
					a = 3
				}
				if k == 3 {
					a += a
				}
			}
			v += a
		}
		if l.Type == 0 && kd == 1 || l.Type == 1 && kd == 2 {
			if g.leaderHasSpecialSkill(o, k) {
				v += int16(val)
			}
		}
	}
	for t := 0; t < 3; t++ {
		if l.Techs[t] == 0 {
			continue
		}
		f := g.exeI16(appFieldTbl + uint32(l.Techs[t])*13)
		if f <= -1 {
			v = 1
			continue
		}
		switch c := int32(g.exe.u32(uint32(fieldCost + int(f)*0x17))); {
		case c < 1000:
			v++
		case c < 5000:
			v += 3
		default:
			v += 4
		}
	}
	return v
}

// initNewGameWrapper is the 1.40 patch's wrapper (0x169410) of
// Init_New_Game_: Cluster is generated as the next galaxy size.
func (g *Game) initNewGameWrapper() error {
	st := &g.settings
	size := int16(st[0xd6])
	if g.patchByte(patchCluster)&1 != 0 {
		size = int16(int8(int8(size) + 1))
	}
	return g.initNewGame(int16(st[0xd4]), size, int16(st[0xd7]), int16(st[0xd5]))
}

// initNewGame is Init_New_Game_ (0x12479): leaders, players, the galaxy
// (again until the home worlds fit), monsters, wormholes and the start of
// turn 1; the result is saved as SAVE10.GAM.
func (g *Game) initNewGame(difficulty, size, age, players int16) error {
	s := &g.state
	g.setUniverseSeed()
	g.genSeed()
	s.Stardate = 35000
	s.NumPlayers = players
	g.initShipsArray()
	g.initColoniesArray()
	s.Antarans = [0x42]byte{}
	if g.gameType != 2 {
		s.PlayerNum = 0
	}
	if err := g.initLeaders(); err != nil {
		return err
	}
	if err := g.initPlayers(); err != nil {
		return err
	}
	g.allocateMsgSlots()
	g.genSeed()
	g.setUniverseSeed()
	for ok := false; !ok; {
		if err := g.generateUniverse(size, age, difficulty); err != nil {
			return err
		}
		ok = g.generateHomeWorlds()
		if !ok {
			continue
		}
		if g.e != nil {
			g.e.V.SetPageOff()
			g.e.V.Fill(0, 0, 0x27f, 0x1df, 0)
			if err := g.e.TogglePages(); err != nil {
				return err
			}
		}
		g.enforcePlanetMax(0xfa)
		g.setBlackHoleVisitedFlags()
		if g.settings[0xd9] == 2 {
			g.advancedCivilizationColonies()
			g.assignAdvancedCivilizationStartingShips()
		}
	}
	g.makeSystemMonsters()
	g.makeSystemMonstersIntoShips()
	g.generateWormholeLinks()
	if g.settings[0xd9] == 2 {
		g.allocateAdvCivGameOfficers()
	}
	g.assignMaroonedHeroes()
	g.initializeBlackHoleBlocks(0x2d)
	g.initializeStarInNebulaInfo()
	s.CurMapX, s.CurMapY = 0, 0
	g.currentScreen = ScreenReports
	s.CouncilWinner = -1
	g.haroldDefaults()
	g.billDefault()
	g.haroldInit()
	g.initEvents()
	g.billInit()
	s.updateStarOwners()
	g.allocateNewColonySlots()
	g.twiddleInitialHomeworlds()
	g.allocateNewShipSlots()
	g.computeAllStarColonyStuff()
	g.processTradeAndResearchAgreements()
	if g.settings[7] != 0 {
		g.setInitialColonyAutoSelects()
	}
	g.initializeReports()
	g.initSurrenders()
	if err := g.saveGame(9); err != nil {
		return err
	}
	if g.newGameSavedHook != nil {
		g.newGameSavedHook(g)
	}
	if err := g.saveGameSettings(); err != nil {
		return err
	}
	if err := g.loadNebulaPictures(); err != nil {
		return err
	}
	g.setDefaults()
	g.newGamePatchEnd()
	return nil
}

// newGamePatchEnd is the 1.40 patch's code at the end of Init_New_Game_
// (0x1691e7): /droids' android techs, Post Warp's head start, and the
// universe seed as the RNG's seed.
func (g *Game) newGamePatchEnd() {
	s := &g.state
	give := func(list uint32, n int) {
		for p := 0; p < int(s.NumPlayers); p++ {
			for k := 0; k < n; k++ {
				g.playerGetsTechApp(p, int(g.exeU16(list+uint32(2*k))))
			}
		}
	}
	if g.exeU8(patchDroids) == 0xff {
		give(patchDroidApps, 3)
	}
	if g.patchByte(patchPostWarp) == 1 {
		if g.ng.combat == 1 {
			give(patchPostWarpCombatApps, 5)
		}
		give(patchPostWarpApps, 0x17)
		for p := 0; p < int(s.NumPlayers); p++ {
			pl := &s.Players[p]
			pl.FieldStatus[3], pl.FieldStatus[4] = 3, 3
			pl.FieldStatus[20], pl.FieldStatus[21] = 2, 3
		}
	}
	s.Random.Set(g.settings.UniverseSeed())
}

const (
	patchDroids             = 0x1787ad // /droids
	patchDroidApps          = 0x1787c2 // three apps
	patchPostWarpCombatApps = 0x178802 // five apps, with tactical combat
	patchPostWarpApps       = 0x17880c // 0x17 apps
)

// enforcePlanetMax is Enforce_Planet_Max_ (0x8bc39): while more than max
// planets (gas giants not counted) remain, random ones away from the home
// stars go, then the rest are packed and the stars' slots rebuilt.
func (g *Game) enforcePlanetMax(max int16) {
	s := &g.state
	old := int(s.PlanetCount)
	gas := int16(0) // Gas_Giants_ (0x8be4d)
	for st := 0; st < int(s.NumStars); st++ {
		for _, pi := range s.Stars[st].Planets {
			if pi > -1 && s.Planets[pi].Type == 2 {
				gas++
			}
		}
	}
	if int32(s.PlanetCount)-int32(gas) < int32(max) {
		return
	}
	var home [72]bool
	for p := 0; p < 8; p++ {
		if p < int(s.NumPlayers) {
			home[s.Planets[s.Players[p].HomePlanet].Star] = true
		}
	}
	tmp := s.Planets               // the 0x17e8-byte copy
	gone := make([]bool, len(tmp)) // the copy's colony word set to -2
	for int32(s.PlanetCount)-int32(gas) >= int32(max) {
		k := int(int16(s.Random.Random(uint32(int32(s.PlanetCount))) - 1))
		pl := &tmp[k]
		if gone[k] || home[pl.Star] {
			continue
		}
		// Quirk: the star's slot is indexed by the planet number, reading
		// far past the five slots (into later stars).
		v := g.starSlotWord(int(pl.Star), k)
		if v <= -1 {
			continue
		}
		if int(v) >= len(tmp) {
			g.logOnce("Enforce_Planet_Max_ reads past the planet copy; skipped")
			continue
		}
		if tmp[v].Type == 2 {
			continue
		}
		s.PlanetCount--
		gone[k] = true
	}
	for i := 0; i < int(s.NumStars); i++ { // Clear_Planets_In_Star_Structure_
		for j := range s.Stars[i].Planets {
			s.Stars[i].Planets[j] = -1
		}
	}
	n := int16(0)
	for i := 0; i < old; i++ {
		if gone[i] {
			continue
		}
		for p := 0; p < int(s.NumPlayers); p++ { // Home_Planet_ (0x8ceac)
			if int(s.Players[p].HomePlanet) == i {
				s.Players[p].HomePlanet = n
				break
			}
		}
		s.Planets[n] = tmp[i]
		s.Stars[tmp[i].Star].Planets[tmp[i].Orbit] = n
		n++
	}
}

// starSlotWord is the int16 at stars[star] + 0x4a + 2·k in the star array
// (0x1fc8 bytes); past its end the original reads the next DOS block.
func (g *Game) starSlotWord(star, k int) int16 {
	off := star*0x71 + 0x4a + 2*k
	if off+2 > 0x1fc8 {
		g.logOnce("Enforce_Planet_Max_ reads past the star array; taken as -1")
		return -1
	}
	var b bytes.Buffer
	binary.Write(&b, binary.LittleEndian, &g.state.Stars)
	return int16(binary.LittleEndian.Uint16(b.Bytes()[off:]))
}

// setBlackHoleVisitedFlags is Set_Black_Hole_Visited_Flags_ (0x8d65d).
func (g *Game) setBlackHoleVisitedFlags() {
	s := &g.state
	for i := 0; i < int(s.NumStars); i++ {
		if s.Stars[i].SpectralClass == 6 {
			s.Stars[i].Visited = 0xff
		}
	}
}

// haroldInit is Harold_Init_ (0x7962c) with Init_Moving_Boxes_ (0x79756):
// the five main screen boxes, closed but the fleet box's size, the system
// display's from its picture.
func (g *Game) haroldInit() {
	s := &g.state
	m := &g.ms
	m.boxOffFlag = [numBoxes]int16{-1, 0, -1, 0, 0}
	s.MoveableBoxes = [numBoxes]save.MoveableBox{}
	for k := 0; k < numBoxes; k++ {
		b := &s.MoveableBoxes[k]
		b.ID = int16(k)
		s.MoveableBoxOrder[k] = int16(k)
		b.X, b.Y = -1, -1
		b.Content = m.boxOffFlag[k]
	}
	// Init_Fleet_Movement_Box_ (0x7971f).
	b := &s.MoveableBoxes[2]
	b.Height, b.Width, b.X, b.Y = 0x110, 0xc8, 0x16, 0x16
	b.ClosedWidth, b.ClosedHeight = 0xc8, 0x27
	// Init_System_Display_Box_ (0x797dd): BUFFER0.LBX 0's size.
	a, err := g.archive("BUFFER0.LBX")
	if err == nil {
		var e []byte
		if e, err = a.Entry(0); err == nil {
			b := &s.MoveableBoxes[0]
			b.Width = int16(binary.LittleEndian.Uint16(e))
			b.Height = int16(binary.LittleEndian.Uint16(e[2:]))
		}
	}
	if err != nil {
		g.Log("Init_System_Display_Box_: %v", err)
	}
	if s.MoveableBoxes[0].X < 0 {
		s.MoveableBoxes[0].X = 0x16
	}
	if s.MoveableBoxes[0].Y < 0 {
		s.MoveableBoxes[0].Y = 0x16
	}
	g.renamedHomeStar = [8]bool{}
}

// billInit is Bill_Init_ (0x102037): _bill_savegame's words.
func (g *Game) billInit() {
	for i, v := range []uint16{2, 0, 1, 1, 1, 1} {
		binary.LittleEndian.PutUint16(g.state.BillSavegame[2*i:], v)
	}
}

// initSurrenders is Init_Surrenders_ (0xe4e54).
func (g *Game) initSurrenders() {
	s := &g.state
	for i := 0; i < int(s.NumStars); i++ {
		for p := 0; p < int(s.NumPlayers); p++ {
			s.Stars[i].SurrenderTo[p] = -1
		}
	}
	for p := 0; p < int(s.NumPlayers); p++ {
		s.Players[p].SurrenderTo = -1
	}
}

// computeAllStarColonyStuff is Compute_All_Star_Colony_Stuff_ (0xe5408).
func (g *Game) computeAllStarColonyStuff() {
	for i := int(g.state.NumStars) - 1; i >= 0; i-- {
		g.state.computeStarColonyStuff(i)
	}
}

// setInitialColonyAutoSelects is Set_Initial_Colony_Auto_Selects_
// (0x123ce): each star's colony orbit for the viewing player.
func (g *Game) setInitialColonyAutoSelects() {
	s := &g.state
	for i := 0; i < int(s.NumStars); i++ {
		st := &s.Stars[i]
		for k := range st.LastOrbit {
			st.LastOrbit[k] = -1
		}
		g.setColonyAutoSelectForStar(i, int(s.PlayerNum))
	}
}

// twiddleInitialHomeworlds is Twiddle_Initial_Homeworlds_ (0xe5832): the
// starting jobs, and on advanced tech money, explored stars, freighters
// and ships.
func (g *Game) twiddleInitialHomeworlds() {
	s := &g.state
	g.computeContacts()
	for ci := int(s.NumColonies) - 1; ci >= 0; ci-- {
		g.twiddleInitialHomeworld(ci)
	}
	g.updateAllShipRanges()
	g.doColonyCalculations()
	minCmd := int32(30000)
	for p := int(s.NumPlayers) - 1; p >= 0; p-- {
		pl := &s.Players[p]
		s.Stars[s.Planets[pl.HomePlanet].Star].ColonizePlayer = int8(p)
		if v := int32(pl.CommandTotal); v < minCmd {
			minCmd = v
		}
	}
	if g.settings[0xd9] != 2 {
		return
	}
	half := (minCmd - 2) / 2
	for p := int(s.NumPlayers) - 1; p >= 0; p-- {
		pl := &s.Players[p]
		s.Stars[s.Planets[pl.HomePlanet].Star].ColonizePlayer = int8(p)
		pl.BC = (int32(pl.Picks[5]) + 2) * 400 / 4
		for i := int(s.NumStars) - 1; i >= 0; i-- {
			st := &s.Stars[i]
			if st.Special == 7 || st.Special == 8 || !s.starInNormalRangeOfPlayer(i, p) {
				continue
			}
			st.Visited |= 1 << uint(p)
			if (st.Special == 2 || st.Special == 3) && !s.monsterAtStar(i) {
				st.Special = 0
			}
		}
		n := int32(10)
		if pl.FreightersFree < 0 {
			n = 10 - int32(pl.FreightersFree)
		}
		if pl.FoodSurplus < 0 {
			n -= int32(pl.FoodSurplus)
		}
		pl.FreightersTotal = int16(5 * ((n + 4) / 5))
		col := 0
		for int(s.Colonies[col].Owner) != p {
			col++
		}
		size, left := half, minCmd
		if size < 0 {
			size = 0
		} else if size >= 4 {
			size = 3
			if pl.AppStatus[186] == 3 {
				size = 4
			}
		}
		for left != 0 {
			if left-1 < size {
				size = left - 1
			}
			s.Ships[s.NumShips].Owner = int8(p)
			g.updatePlayerShipDesigns(p)
			g.createShip(p, int(size), col)
			left -= size + 1
		}
	}
	g.doColonyCalculations()
}

// monsterAtStar is Monster_At_Star_ (0xe57f4).
func (s *State) monsterAtStar(star int) bool {
	for i := int(s.NumShips) - 1; i >= 0; i-- {
		if s.Ships[i].Owner >= 8 && int(s.Ships[i].Location) == star {
			return true
		}
	}
	return false
}

// setJob sets a colonist's job (0 farmer, 1 worker, 2 scientist).
// Index -1 is the dword before the list: the job bits land in Pollution.
func colonistJob(c *save.Colony, i int, job uint32) {
	if i == -1 {
		c.Pollution = int16(uint16(c.Pollution)&^0x180 | uint16(job)<<7)
		return
	}
	c.Colonists[i] = c.Colonists[i]&^0x180 | job<<7
}

// twiddleInitialHomeworld is Twiddle_Initial_Homeworld_ (0xe55be): all
// scientists, then workers until industry covers its upkeep, farmers until
// food does, workers while research leads; the star's start specials go.
func (g *Game) twiddleInitialHomeworld(ci int) {
	s := &g.state
	c := &s.Colonies[ci]
	end := int(c.PopCount)
	for i := end - 1; i >= 0; i-- {
		colonistJob(c, i, 2)
	}
	star := int(s.Planets[c.Planet].Star)
	s.Stars[star].ColonizePromptOff |= 1 << uint(c.Owner&7)
	g.preImportComputing(ci)
	k := 0
	for {
		g.passOutImports(int(c.Owner), ci)
		if k >= end || int16(c.Upkeep[1]) <= c.Output[1] {
			break
		}
		colonistJob(c, k, 1)
		g.preImportComputing(ci)
		k++
	}
	if c.FoodPerFarmer != 0 {
		for k < end && int16(c.Upkeep[0]) > c.Output[0] {
			colonistJob(c, k, 0)
			g.preImportComputing(ci)
			k++
			g.passOutImports(int(c.Owner), ci)
		}
	}
	for k < end && c.Output[1] < c.Output[2] {
		colonistJob(c, k, 1)
		g.preImportComputing(ci)
		k++
		g.passOutImports(int(c.Owner), ci)
	}
	if c.Output[2] <= c.Output[1] {
		k--
		colonistJob(c, k, 2)
		g.preImportComputing(ci)
		g.passOutImports(int(c.Owner), ci)
	}
	st := &s.Stars[star]
	switch st.Special {
	case 2, 3:
		st.Special = 0
	case 6, 7:
		st.Special = 0
		for o := 4; o >= 0; o-- {
			if pi := st.Planets[o]; pi != -1 {
				s.Planets[pi].Special = 0
			}
		}
	case 8:
		for o := leaderCount - 1; o >= 0; o-- {
			l := &s.Leaders[o]
			if l.Status == 5 && int(l.Location) == star {
				g.deassignOfficer(o)
			}
		}
		st.Special = 0
	case 10:
		st.ArtifactsHome = 1
	}
}
