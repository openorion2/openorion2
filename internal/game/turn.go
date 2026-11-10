package game

import (
	"github.com/openorion2/openorion2/internal/engine"
	"github.com/openorion2/openorion2/internal/save"
)

// turnStep is one call of Next_Turn_Calc_ (0x136b3), in call order; run is
// nil until the step is ported.
type turnStep struct {
	name string
	run  func(*Game)
}

// turnSteps lists the steps of Next_Turn_Calc_. Conditions on a call live in
// the step function, so a step can run on its own in tests. In a release
// build Check_Release_Version_ returns 1, so Assert_Marooned_Leaders_ never runs.
func turnSteps() []turnStep {
	return []turnStep{
		{"Allocate_Msg_Slots_", (*Game).allocateMsgSlots},
		{"Antaran_Invasion_Check_", (*Game).antaranInvasionStep},
		{"Apply_Evolutionary_Upgrades_", (*Game).applyEvolutionaryUpgrades},
		{"Compute_AI_Data_", (*Game).computeAIData},
		{"Resolve_Delayed_Diplomacy_Orders_", (*Game).resolveDelayedDiplomacyOrders},
		{"Set_Opportunity_Attacks_", (*Game).setOpportunityAttacks},
		{"Clear_Diplomacy_Messages_", (*Game).clearDiplomacyMessages},
		{"NPC_Diplomacy_", (*Game).npcDiplomacy},
		{"Diplomacy_Growth_", (*Game).diplomacyGrowth},
		{"Do_AI_Leaders_", (*Game).doAILeaders},
		{"Compute_Empire_Building_Needs_", (*Game).computeEmpireBuildingNeeds},
		{"All_Colony_AI_", (*Game).allColonyAI},
		{"Move_All_AI_", (*Game).moveAllAI},
		{"All_AI_Colonize_", (*Game).allAIColonize},
		{"All_AI_Tech_Select_", (*Game).allAITechSelect},
		{"Make_Scrap_Ships_Dead_", (*Game).makeScrapShipsDead},
		{"Initialize_Reports_", (*Game).initializeReports},
		{"Process_Trade_And_Research_Agreements_", (*Game).processTradeAndResearchAgreements},
		{"Move_All_Ships_Toward_Stars_", (*Game).moveAllShipsTowardStars},
		{"Resolve_Spies_", (*Game).resolveSpies},
		{"Apply_All_Player_Changes_", (*Game).applyAllPlayerChanges},
		{"Apply_All_Colony_Changes_", (*Game).applyAllColonyChanges},
		{"Do_Surrenders_", (*Game).doSurrenders},
		{"Deallocate_AI_Data_", (*Game).deallocateAIData},
		{"Search_For_Battles_", (*Game).searchForBattles},
		{"Determine_Event_", (*Game).determineEvent},
		{"Do_All_Ships_XP_Check_", (*Game).doAllShipsXPCheck},
		{"Event_Twiddle_", (*Game).eventTwiddle},
		{"Lose_Out_Of_Range_Ships_", (*Game).loseOutOfRangeShips},
		{"Do_Colony_Calculations_", (*Game).doColonyCalculations},
		{"Compute_Blockades_", (*Game).computeBlockades},
		{"Move_Settlers_", (*Game).moveSettlers},
		{"Do_Colony_Calculations_", (*Game).doColonyCalculations},
		{"Allocate_New_Ship_Slots_", (*Game).allocateNewShipSlots},
		{"Check_For_Officer_Level_", (*Game).checkForOfficerLevel},
		{"Decrement_Officer_ETA_", (*Game).decrementOfficerETA},
		{"Check_All_Rebellions_", (*Game).checkAllRebellions},
		{"Repair_Ships_At_Colonies_", (*Game).repairShipsAtColonies},
		{"Allocate_New_Colony_Slots_", (*Game).allocateNewColonySlots},
		{"Compute_Contacts_", (*Game).computeContacts},
		{"Determine_First_Contacts_", (*Game).determineFirstContacts},
		{"Reset_Needless_Ignore_Combat_Ships_Flags_", (*Game).resetNeedlessIgnoreCombatShipsFlags},
		{"Determine_Diplomacy_Messages_", (*Game).determineDiplomacyMessages},
		{"End_Of_Turn_Diplomacy_Adjustments_", (*Game).endOfTurnDiplomacyAdjustments},
		{"Check_For_Council_Meeting_", (*Game).checkForCouncilMeetingStep},
		{"Random_Officer_Check_", (*Game).randomOfficerCheck},
		{"Add_Ongoing_Event_Msgs_", (*Game).addOngoingEventMsgs},
		{"Check_Release_Version_", func(*Game) {}},
		{"Assert_Marooned_Leaders_", func(*Game) {}},
		{"Record_History_", (*Game).recordHistory},
	}
}

// allocateMsgSlots is Allocate_Msg_Slots_ (0xeeb3a): the 100 message slots
// are split evenly among the players still in the game; stale messages stay.
func (g *Game) allocateMsgSlots() {
	s := &g.state
	alive := int32(0)
	for p := 0; p < int(s.NumPlayers); p++ {
		if s.Players[p].Eliminated == 0 {
			alive++
		}
	}
	s.FirstMsg, s.MaxMsgs, s.NMsgs = [8]uint8{}, [8]uint8{}, [8]uint8{}
	lo, hi := int32(0), int32(100)
	for p := 0; p < int(s.NumPlayers); p++ {
		if s.Players[p].Eliminated != 0 {
			continue
		}
		s.FirstMsg[p] = uint8(lo / alive)
		s.MaxMsgs[p] = uint8(hi/alive) - s.FirstMsg[p]
		lo += 100
		hi += 100
	}
}

// initializeReports is Initialize_Reports_ (0xfd81c), which ends in the body
// of Client_Initialize_Reports_ (0xfd7ea): the report state of a new turn.
func (g *Game) initializeReports() {
	s := &g.state
	s.SomethingHappened = [8]byte{}
	s.NReports = 0
	s.DoneColonizeCheck = [9]byte{}
	s.DonePlayerReports = 0
	s.DisplayedMsgs = 0
	s.DisplayedGNN = 0
	s.Antarans[0x41] = 0
}

// doColonyCalculations is Do_Colony_Calculations_ (0xe2b31; the turn map calls
// it All_Colony_Calculations_): every colony's economy, then the per-player
// tallies of researched fields, fleet size and colonies of 5+ pop.
func (g *Game) doColonyCalculations() {
	s := &g.state
	for i := int(s.NumColonies) - 1; i >= 0; i-- {
		if c := &s.Colonies[i]; c.Owner != -1 && c.IsOutpost == 0 {
			g.preImportComputing(i)
		}
	}
	// The original passes a null colony pointer (xor edx, edx): none.
	for p := 0; p < int(s.NumPlayers); p++ {
		g.passOutImports(p, -1)
	}
	for i := int(s.NumColonies) - 1; i >= 0; i-- {
		if c := &s.Colonies[i]; c.Owner != -1 && c.IsOutpost == 0 {
			g.colonyPopGrows(i)
			colonySpecialty(c)
		}
	}
	for p := 0; p < int(s.NumPlayers); p++ {
		g.updatePlayerStats(p)
	}
	for p := 0; p < int(s.NumPlayers); p++ {
		pl := &s.Players[p]
		pl.FieldsResearched = 0
		for f := len(pl.FieldStatus) - 1; f >= 0; f-- {
			if pl.FieldStatus[f] == 3 {
				pl.FieldsResearched++
			}
		}
		pl.FleetSizeScore = 0
		pl.ColoniesPop5 = 0
	}
	for i := int(s.NumColonies) - 1; i >= 0; i-- {
		if c := &s.Colonies[i]; c.Owner >= 0 && c.PopCount >= 5 {
			s.Players[c.Owner].ColoniesPop5++
		}
	}
	for i := int(s.NumShips) - 1; i >= 0; i-- {
		if sh := &s.Ships[i]; sh.Owner < 8 && sh.Status < 5 && sh.Design.Type == 0 {
			s.Players[sh.Owner].FleetSizeScore += 1 << sh.Design.Size
		}
	}
}

// deallocateAIData is Deallocate_AI_Data_ (0xd574d): the original pops the
// AI data arena; the AI tables are valid only between steps 3 and 23. The
// AIDATA.C statics outside the arena stay for the battle search.
func (g *Game) deallocateAIData() {
	if g.ai != nil {
		g.aiKept = &aiArena{basePlanetValues: g.ai.basePlanetValues,
			contextualPlanetValues: g.ai.contextualPlanetValues, nAllShips: g.ai.nAllShips}
	}
	g.ai = nil
}

// nextTurnCalc is Next_Turn_Calc_ (0x136b3): the steps in order, then the
// stardate; an empty auto function meanwhile (single and hotseat games),
// and the arrow again afterwards unless the next turn is automatic.
func (g *Game) nextTurnCalc() {
	if g.e != nil && g.gameType <= 1 {
		g.e.AssignAutoFunction(func() error { return nil }, 1) // 0xfe8d4: ret
	}
	for k, st := range turnSteps() {
		if g.stepOverride != nil && g.stepOverride(k, st) {
			continue
		}
		if st.run == nil {
			g.Log("Next_Turn_Calc_: %s is not ported", st.name)
			continue
		}
		st.run(g)
	}
	g.state.Stardate++
	if g.e != nil && !g.ms.inAutoNextTurnMode {
		g.e.SetMouseList(engine.MouseArrow)
	}
}

// mouseListClock is _mouse_list_clock: four 12-byte lists, the turning
// clock pointer of the automatic next turn.
const mouseListClock = 0x182248

// incrementMouseClockImage is Increment_Mouse_Clock_Image_ (0x8f855).
func (g *Game) incrementMouseClockImage() {
	m := &g.ms
	if m.mouseClockImageNum > 0xf {
		m.mouseClockImageNum = 0
	}
	g.e.SetMouseList(g.exeMouseList(mouseListClock+uint32(m.mouseClockImageNum>>2)*12, 1))
	m.mouseClockImageNum++
}

// ScreenNextTurn is screen 0xc, Next_Turn_.
const ScreenNextTurn = 0xc

// autosaveDescription is "(Auto Save)", 12 bytes copied into slot 10's description.
const autosaveDescription = 0x1789c0

// doNextTurn is TURN on the main screen: Main_Screen_'s part, then
// Do_Next_Turn_ (0x83411) with DL = 0.
func (g *Game) doNextTurn() error {
	g.ms.nextTurnHit, g.ms.autoTurnExit = true, false
	return g.doNextTurnAt(false)
}

// doNextTurnAt is Do_Next_Turn_ (0x83411); auto (DL = 1) comes from
// Do_Begin_Of_Turn_'s automatic next turn, which any input stops.
func (g *Game) doNextTurnAt(auto bool) error {
	m, s, e := &g.ms, &g.state, g.e
	m.atLeastOneReport, m.previousReport, g.fb.justMoved = false, 0, false
	m.animatedStar = -1
	m.cheatWarnMask = 1 << uint(s.PlayerNum)
	if g.gameType == save.GameHotseat {
		g.saveHotseatMapInfo()
		s.Players[s.PlayerNum].TurnDone = 1
		g.currentScreen = ScreenHotseat
	} else {
		g.currentScreen, g.returnScreen = ScreenNextTurn, ScreenReports
		if g.gameType == 2 || g.gameType == 3 {
			g.currentScreen = ScreenNetNextTurn
		}
	}
	m.mainscrPalette = 0x65
	m.shipIconPlayerID = -1
	g.clearSystemDisplayLoadedFlag()
	if auto {
		m.inAutoNextTurnMode = true
		g.incrementMouseClockImage()
		r, err := e.GetInput()
		if err != nil {
			return err
		}
		m.autoTurnExit = int8(r) != 0
		if m.autoTurnExit {
			m.checkNextTurnExitField = false
			e.SetMouseList(engine.MouseArrow)
			m.nextTurnHit = false
			e.ClearFields()
			g.currentScreen = ScreenMain
		}
		return nil
	}
	m.inAutoNextTurnMode = false
	e.ClearFields()
	g.setMainScreenFieldsToNull()
	if g.settings[0x01] == 0 {
		// _next_turn_exit_field: it stays in the field array as field 1, which
		// later text boxes find as their stale field 0 (Esc on GNN news).
		if _, err := e.AddHiddenField(0, 0, 0x27f, 0x1df, "\x1b", 0x29); err != nil {
			return err
		}
	}
	return nil
}

// doAutosave is Do_Autosave_ (0x13822): every stardate when the option is
// on; community patch 1.51 (0x13826 mov al, 1): local games too, not every 4th.
func (g *Game) doAutosave() error {
	if g.settings[0xb] == 0 {
		return nil
	}
	copy(g.state.Descriptions[9][:12], g.exe.bytes(autosaveDescription, 12))
	return g.saveGame(9)
}

// nextTurn is Next_Turn_ (0x13870, screen 0xc).
func (g *Game) nextTurn() error {
	if g.gameType == 0 || g.gameType == 1 {
		if err := g.doAutosave(); err != nil {
			return err
		}
	}
	g.nextTurnCalc()
	g.currentScreen = g.returnScreen
	if g.gameType != 2 && g.gameType != 3 && !g.allAIGame {
		if err := g.checkEndOfGame(); err != nil {
			return err
		}
	}
	s := &g.state
	for p := 0; p < int(s.NumPlayers); p++ {
		s.Players[p].TurnDone = 0
	}
	g.ms.oldSystemIndex = -1
	return nil
}

// checkEndOfGame is the tail of Next_Turn_ (0x138bf..0x139b7): the human
// is gone, one empire is left, the council elected someone, or Antares
// fell, each with its cinematic (docs/screens/cinematics.md).
func (g *Game) checkEndOfGame() error {
	s, e := &g.state, g.e
	lose := func() error {
		if err := g.playFadedFlics(0, eLoserFlic); err != nil { // Play_Loose_Flic_
			return err
		}
		if err := g.endOfGameHiScore(int(s.PlayerNum)); err != nil {
			return err
		}
		g.currentScreen = ScreenMainMenu
		return nil
	}
	switch {
	case g.nLivingHumanPlayersAt() < 1:
		// port-only: why the game ends, in the log (tools/nettest reads it).
		g.Log("game over at stardate %d: no human left (player %d)", s.Stardate, s.PlayerNum)
		if err := e.FastFadeOut(); err != nil {
			return err
		}
		e.V.SetPageOff()
		e.V.Fill(0, 0, 0x27f, 0x1df, 0)
		if err := e.TogglePages(); err != nil {
			return err
		}
		e.V.CopyOffToBack()
		e.V.SetRefreshStencil()
		return lose()
	case g.nLivingPlayers() <= 1:
		w := g.getWinner()
		g.Log("game over at stardate %d: player %d is the last one", s.Stardate, w)
		if err := e.FastFadeOut(); err != nil {
			return err
		}
		if err := g.playFadedFlics(0x400, eGeneralWin); err != nil { // Play_Normal_Win_Flic_
			return err
		}
		if err := g.endOfGameHiScore(int(w)); err != nil {
			return err
		}
		g.currentScreen = ScreenMainMenu
	case s.CouncilWinner != -1:
		g.Log("game over at stardate %d: the council elected player %d", s.Stardate, s.CouncilWinner)
		if s.CouncilWinner != s.PlayerNum {
			return lose()
		}
		if err := g.endOfGameHiScore(int(s.PlayerNum)); err != nil {
			return err
		}
		g.currentScreen = ScreenMainMenu
	default:
		for p := int(s.NumPlayers) - 1; p >= 0; p-- {
			if s.Players[p].CapturedAntares == 0 {
				continue
			}
			g.Log("game over at stardate %d: player %d took Antares", s.Stardate, p)
			e.DeactivateAutoFunction()
			if err := g.playFadedFlics(0x400, eAntaranWin, eGeneralWin); err != nil { // Play_Antaran_Win_Flic_
				return err
			}
			if err := g.endOfGameHiScore(p); err != nil {
				return err
			}
			g.currentScreen = ScreenMainMenu
		}
	}
	return nil
}

// getWinner is Get_Winner_ (0x139e8): the council's winner, else the last
// player who took Antares or is still in the game.
func (g *Game) getWinner() int16 {
	s := &g.state
	if s.CouncilWinner != -1 {
		return s.CouncilWinner
	}
	w := int16(-1)
	for p := int16(0); p < s.NumPlayers; p++ {
		if pl := &s.Players[p]; pl.CapturedAntares != 0 || pl.Eliminated == 0 {
			w = p
		}
	}
	return w
}

// nLivingPlayers is N_Living_Players_ (0xe502a).
func (g *Game) nLivingPlayers() int {
	n := 0
	for p := 0; p < int(g.state.NumPlayers); p++ {
		if g.state.Players[p].Eliminated == 0 {
			n++
		}
	}
	return n
}

// nLivingHumanPlayersAt is Next_Turn_'s call of N_Living_Human_Players_;
// under the autopilot it asks whether PLAYER_NUM is still in the game.
func (g *Game) nLivingHumanPlayersAt() int {
	if g.autopilot {
		if g.state.Players[g.state.PlayerNum].Eliminated == 0 {
			return 1
		}
		return 0
	}
	return g.nLivingHumanPlayers()
}

// nLivingHumanPlayers is N_Living_Human_Players_ (0xe505e).
func (g *Game) nLivingHumanPlayers() int {
	n := 0
	for p := 0; p < int(g.state.NumPlayers); p++ {
		if pl := &g.state.Players[p]; pl.Eliminated == 0 && pl.Objective == 100 {
			n++
		}
	}
	return n
}
