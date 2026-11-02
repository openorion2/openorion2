package game

import (
	"encoding/binary"
	"fmt"

	"github.com/openorion2/openorion2/internal/save"
)

// Technology: what a player may research, gaining an application and the
// knock-on updates, and the EINSTEIN cheat. Spec: docs/formats/tech.md.

// Constant tables of ORION2.EXE (ELF addresses).
const (
	techFieldTable = 0x17d904 // _technology_fields, 0x17-byte rows
	techFieldRow   = 0x17
	techFieldNext  = 0x08 // i16 the field researched after this one
	techFieldApps  = 0x0a // 4 x i16 applications, 0 for none

	techAppTable    = 0x17e079 // _technology_applications, 0xd-byte rows
	techAppRow      = 0x0d
	techAppField    = 0x06 // i16 field, -1 for none
	techAppTactical = 0x0b // u8, 0: not available with settings 0xd8 set

	numTechFields = 83
	numTechApps   = 0xcc // apps the routines consider, 0..0xcb

	fuelTable = 0x17ffde // Update_Player_Ship_Range_ reads i16 app at +i, range at +i+2
)

// Field and application status values (Player.FieldStatus, AppStatus).
const (
	techUnknown      = 0
	techObtainable   = 1
	techResearchable = 2
	techKnown        = 3
)

func (g *Game) techFieldNextOf(f int) int {
	return int(g.exeI16(uint32(techFieldTable + f*techFieldRow + techFieldNext)))
}

// techFieldApp is application k (0..3) of field f, as Init_Tech_ lists
// them.
func (g *Game) techFieldApp(f, k int) int {
	g.initTech()
	return int(g.exeI16(uint32(techFieldTable + f*techFieldRow + techFieldApps + 2*k)))
}

// initTech is the table part of Init_Tech_ (0x5e1e3): application kinds and
// the field application slots the executable ships empty, written on first use.
func (g *Game) initTech() {
	g.exe.techOnce.Do(g.initTechTables)
}

func (g *Game) initTechTables() {
	x := g.exe
	kinds := []struct {
		table, stride uint32
		n             int
		kind          byte
	}{
		{0x17eb43, 0x13, 0x31, 1}, // buildings
		{0x17eee6, 0x2f, 0x28, 9}, // specials
		{0x17f63e, 0x0f, 0x07, 7}, // armour
		{0x17f6a7, 0x3b, 0x06, 2}, // shields
		{0x17f80d, 0x1c, 0x2e, 3}, // weapons
		{0x17fdf2, 0x16, 0x06, 8}, // computers
		{0x17fe76, 0x2e, 0x07, 5}, // drives
		{0x17ffe8, 0x0a, 0x06, 4}, // fuel cells
	}
	for _, k := range kinds {
		for i := 1; i < k.n; i++ {
			app := int32(g.exeI16(k.table + uint32(i)*k.stride))
			x.bytes(uint32(techAppTable+0xc+app*techAppRow), 1)[0] = k.kind
		}
	}
	for a := 0; a < 0xd4; a++ {
		f := g.techAppFieldOf(a)
		if f == 0 || f == -1 || f == 0x4a {
			continue
		}
		slot := -1
		for k := 0; k < 4 && slot == -1; k++ {
			if g.exeI16(uint32(techFieldTable+f*techFieldRow+techFieldApps+2*k)) == 0 {
				slot = k
			}
		}
		if slot == -1 {
			panic(fmt.Sprintf("%d%s", a, cStringAt(x, 0x178fbd)))
		}
		binary.LittleEndian.PutUint16(x.bytes(uint32(techFieldTable+f*techFieldRow+techFieldApps+2*slot), 2), uint16(a))
	}
}

func (g *Game) techAppFieldOf(a int) int {
	return int(g.exeI16(uint32(techAppTable + a*techAppRow + techAppField)))
}

// playerMayGetTechApp is Player_May_Get_Tech_App_ (0xe412b): app is not
// known yet and exists in this game (Antarans, government, combat type).
func (g *Game) playerMayGetTechApp(p, app int) bool {
	pl := &g.state.Players[p]
	if pl.AppStatus[app] == techKnown {
		return false
	}
	gov := uint8(pl.Picks[save.PickGovernment])
	switch uint32(app) {
	case 0, 0x20:
		return false
	case 0x34:
		return g.settings[0xd3] != 0
	case 0x2a:
		return gov == 0
	case 0x41:
		return gov == 4
	case 0x4d:
		return gov == 6
	case 0x5c:
		return gov == 2
	}
	if g.settings[0xd8] != 0 && g.exeU8(uint32(techAppTable+app*techAppRow+techAppTactical)) == 0 {
		return false
	}
	return app < numTechApps && g.techAppFieldOf(app) != -1
}

// fieldIsGrayBox is Field_Is_Gray_Box_ (0xe41db): the fields whose
// applications all come at once.
func fieldIsGrayBox(f int) bool {
	switch uint32(f) {
	case 0x16, 0x17, 0x1c, 0x1d, 0x37, 0x39:
		return true
	}
	return false
}

// advancePlayerField is Advance_Player_Field_ (0xe401d): finish fields
// until one offers an application; with keep the first round only looks.
func (g *Game) advancePlayerField(p, f int, keep bool) {
	pl := &g.state.Players[p]
	// port-only: field 0 (no field) is its own next and offers nothing; the
	// loop never ends there. A research breakthrough (event 18) leaves the
	// field 0 and a second one before a new choice asks for it.
	if f <= 0 {
		g.logOnce("Advance_Player_Field_ on field 0: nothing to advance")
		return
	}
	found := 0
	for round := 0; ; round++ {
		if !keep || round != 0 {
			pl.FieldStatus[f] = techKnown
			f = g.techFieldNextOf(f)
			pl.FieldStatus[f] = techResearchable
		}
		for k := 3; k >= 0; k-- {
			if a := g.techFieldApp(f, k); a > 0 && pl.AppStatus[a] == techObtainable {
				found++
			}
		}
		if found != 0 || f >= 0x4b {
			return
		}
	}
}

// ensureUncreativeFieldOK is Ensure_Uncreative_Field_OK_ (0xe408f): with
// nothing offered in field f, one unknown application is drawn at random.
func (g *Game) ensureUncreativeFieldOK(p, f int) {
	s := &g.state
	pl := &s.Players[p]
	pick, obtainable, n := -1, 0, uint32(0)
	for k := 3; k >= 0; k-- {
		a := g.techFieldApp(f, k)
		if a <= 0 {
			continue
		}
		switch pl.AppStatus[a] {
		case techObtainable:
			obtainable++
		case techUnknown:
			if (g.settings[0xd3] == 0 && a == 0x34) || f == 6 {
				continue
			}
			if g.settings[0xd8] != 0 && g.exeU8(uint32(techAppTable+a*techAppRow+techAppTactical)) == 0 {
				continue
			}
			n++
			if s.Random.Random(n) == 1 {
				pick = a
			}
		}
	}
	if obtainable == 0 && n > 0 {
		pl.AppStatus[pick] = techObtainable
	}
}

// playerGetsTechApp is Player_Gets_Tech_App_ (0xe4204): p learns app, with
// its knock-on updates, colony recalculation and next research target.
func (g *Game) playerGetsTechApp(p, app int) {
	s := &g.state
	pl := &s.Players[p]
	pl.AppStatus[app] = techKnown
	pl.Unknown323 = 1
	switch uint16(app) {
	case 0x5c:
		pl.Picks[save.PickGovernment] = 3
	case 0x4d:
		pl.Picks[save.PickGovernment] = 7
	case 0x2a:
		pl.Picks[save.PickGovernment] = 1
	case 0x41:
		pl.Picks[save.PickGovernment] = 5
	case 0x63, 0xaa, 0xb0, 0xb4:
		for i := int(s.NumStars) - 1; i >= 0; i-- {
			s.computeStarColonyStuff(i)
		}
	case 0x18:
		if g.playerMayGetTechApp(p, 0xe) {
			g.playerGetsTechApp(p, 0xe)
		}
	default:
		g.updatePlayerShipDesigns(p)
		g.updateAllShipRanges()
	}
	g.doPlayerColonyCalculations(p)
	if int16(app) == int16(pl.ResearchApp) && pl.FieldStatus[pl.ResearchField] == techResearchable {
		field := int(pl.ResearchField)
		done := true
		if pl.Picks[save.PickCreative] != 0 || fieldIsGrayBox(field) {
			for k := 0; k < 4; k++ {
				if a := g.techFieldApp(field, k); a != app && pl.AppStatus[a] == techObtainable {
					done = false
					pl.ResearchApp = uint8(a)
				}
			}
		}
		if done {
			g.advancePlayerField(p, int(pl.ResearchField), true)
			pl.ResearchField = 0
			pl.ResearchPoints = 0
		}
		return
	}
	f := g.techAppFieldOf(app)
	if pl.Picks[save.PickUncreative] != 0 {
		g.ensureUncreativeFieldOK(p, f)
	}
	if pl.FieldStatus[f] == techResearchable {
		g.advancePlayerField(p, f, true)
	}
}

// giveMeAllTechAdvances is Give_Me_All_Tech_Advances_ (0x85ff2), the
// EINSTEIN cheat: every application the player may get, then the count.
func (g *Game) giveMeAllTechAdvances() error {
	n := g.allTechAdvances(int(g.state.PlayerNum))
	return g.mainUserBox(fmt.Sprintf(cFormat(g.hMessage(0xdc)), n))
}

// allTechAdvances is the loop of Give_Me_All_Tech_Advances_: applications
// 1..0xcb in order, counted in a short.
func (g *Game) allTechAdvances(p int) int16 {
	var n int16
	for a := 1; a < numTechApps; a++ {
		if g.playerMayGetTechApp(p, a) {
			n++
			g.playerGetsTechApp(p, a)
		}
	}
	return n
}

// computeStarColonyStuff is Compute_Star_Colony_Stuff_ (0xe5296): the
// star's per-owner masks of colonies and system-wide buildings and techs.
func (s *State) computeStarColonyStuff(star int) {
	st := &s.Stars[star]
	st.ColonyOwners, st.WarpInterdictor = 0, 0
	st.Jumpgate, st.Stargate = 0, 0
	st.BaseTachyon, st.BaseSubspace = 0, 0
	st.ArtemisNet, st.DimensionalPortal = 0, 0
	for orbit := 4; orbit >= 0; orbit-- {
		pi := st.Planets[orbit]
		if pi == -1 || s.Planets[pi].Colony == -1 {
			continue
		}
		c := &s.Colonies[s.Planets[pi].Colony]
		bit := uint8(1) << (uint8(c.Owner) & 31)
		st.ColonyOwners |= bit
		if bld(c, 0x2d) {
			st.WarpInterdictor |= bit
		}
		var pl *save.Player
		if c.Owner >= 0 && int(c.Owner) < maxPlayers {
			pl = &s.Players[c.Owner]
		}
		if bld(c, 0x28) || bld(c, 8) || bld(c, 0x29) {
			if pl != nil && knows(pl, 0xb4) {
				st.BaseTachyon |= bit
			}
			if pl != nil && knows(pl, 0xb0) {
				st.BaseSubspace |= bit
			}
		}
		if bld(c, 3) {
			st.ArtemisNet |= bit
		}
		if bld(c, 0xe) {
			st.DimensionalPortal |= bit
		}
		if pl != nil && knows(pl, 0x63) {
			st.Jumpgate |= bit
		}
		if pl != nil && knows(pl, 0xaa) {
			st.Stargate |= bit
		}
	}
}

// updatePlayerShipRange is Update_Player_Ship_Range_ (0x10034d): the fuel
// range of the best fuel cell technology known, 0 with none.
func (g *Game) updatePlayerShipRange(p int) {
	pl := &g.state.Players[p]
	for i := uint32(0x3c); i > 10; i -= 10 {
		if pl.AppStatus[g.exeI16(fuelTable+i)] == techKnown {
			pl.FuelRange = g.exeI16(fuelTable + i + 2)
			return
		}
	}
	pl.FuelRange = 0
}

// calcPlayerFTLSpeed is Calc_Player_FTL_Speed_ (0x57597).
func (g *Game) calcPlayerFTLSpeed(p int) int {
	if int16(p) >= 8 {
		return 1
	}
	return g.ftlSpeed(p, g.bestWarpDrive(p))
}

// updateAllShipRanges is Update_All_Ship_Ranges_ (0x10038c).
func (g *Game) updateAllShipRanges() {
	s := &g.state
	for p := int(s.NumPlayers) - 1; p >= 0; p-- {
		g.updatePlayerShipRange(p)
		s.Players[p].BestWarpDrive = uint8(g.bestWarpDrive(p))
		s.Players[p].FTLSpeed = uint8(g.calcPlayerFTLSpeed(p))
	}
}
