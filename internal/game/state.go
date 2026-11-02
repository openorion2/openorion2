package game

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/openorion2/openorion2/internal/random"
	"github.com/openorion2/openorion2/internal/save"
)

// Array sizes of the saved game state.
const (
	maxColonies = 250
	maxPlanets  = 360
	maxStars    = 72
	maxLeaders  = 67
	maxPlayers  = 8
	maxShips    = 500
	numBoxes    = 5
)

// State is the game's global state that a save holds: the globals
// Load_Game_ reads, under their own names. Blocks not decoded yet stay raw.
type State struct {
	Descriptions [10][0x25]byte // _save_game_description, one per slot
	Stardate     uint32
	GameType     uint8

	NumColonies int16
	Colonies    [maxColonies]save.Colony
	PlanetCount int16
	Planets     [maxPlanets]save.Planet
	NumStars    int16
	Stars       [maxStars]save.Star
	Leaders     [maxLeaders]save.Leader
	NumPlayers  int16
	Players     [maxPlayers]save.Player
	NumShips    int16
	Ships       [maxShips]save.Ship

	MoveableBoxes    [numBoxes]save.MoveableBox
	MoveableBoxOrder [numBoxes]int16
	PlayerNum        int16
	MaxMapScale      int16
	MaxZoomCount     int16
	StarBgIndex      uint8
	MapMaxX, MapMaxY int16
	FleetIconButtons int16
	Nebulas          [4]save.Nebula
	NumNebulas       uint8

	BillSavegame          [0xc]byte
	NReports              int16
	Reports               [0x3b6]byte
	DonePlayerReports     uint8
	SomethingHappened     [8]byte
	DoneColonizeCheck     [9]byte
	EventData             [9 * 0x24]byte
	Msgs                  [0x12 * 100]byte
	FirstMsg, MaxMsgs     [8]uint8
	NMsgs                 [8]uint8
	DisplayedMsgs         uint8
	DisplayedGNN          uint8
	CouncilFlag           int16
	NextCouncilTurn       int16
	AntaranResources      int16
	AntaranInvasionChance int16
	DisallowSavesCode     int16
	LastEventYear         uint32
	EventDelayCount       int16
	Antarans              [0x42]byte
	CurMapScale           int16
	CurMapX, CurMapY      int16

	CouncilWinner int16 // _council_winner

	rangeResidue rangeResidue

	Random random.Source
}

// readBlocks fills the state from a parsed save in Load_Game_'s order. The
// description goes to slot min(slot, 9).
func (s *State) readBlocks(f *save.File, slot int) error {
	d := slot
	if d > 8 {
		d = 9
	}
	copy(s.Descriptions[d][:], f.Block(save.BDescription))
	targets := []struct {
		block string
		v     any
	}{
		{save.BStardate, &s.Stardate},
		{save.BGameType, &s.GameType},
		{save.BNumColonies, &s.NumColonies},
		{save.BColonies, &s.Colonies},
		{save.BPlanetCount, &s.PlanetCount},
		{save.BPlanets, &s.Planets},
		{save.BNumStars, &s.NumStars},
		{save.BStars, &s.Stars},
		{save.BLeaders, &s.Leaders},
		{save.BNumPlayers, &s.NumPlayers},
		{save.BPlayers, &s.Players},
		{save.BNumShips, &s.NumShips},
		{save.BShips, &s.Ships},
		{save.BMoveableBox, &s.MoveableBoxes},
		{save.BMoveableBoxOrder, &s.MoveableBoxOrder},
		{save.BPlayerNum, &s.PlayerNum},
		{save.BMaxMapScale, &s.MaxMapScale},
		{save.BMaxZoomCount, &s.MaxZoomCount},
		{save.BStarBgIndex, &s.StarBgIndex},
		{save.BMapMaxX, &s.MapMaxX},
		{save.BMapMaxY, &s.MapMaxY},
		{save.BFleetIconButtons, &s.FleetIconButtons},
		{save.BNebula, &s.Nebulas},
		{save.BNumNebulas, &s.NumNebulas},
		{save.BBillSavegame, &s.BillSavegame},
		{save.BNReports, &s.NReports},
		{save.BReports, &s.Reports},
		{save.BDonePlayerReports, &s.DonePlayerReports},
		{save.BSomethingHappened, &s.SomethingHappened},
		{save.BDoneColonizeCheck, &s.DoneColonizeCheck},
		{save.BEventData, &s.EventData},
		{save.BMsgs, &s.Msgs},
		{save.BFirstMsg, &s.FirstMsg},
		{save.BMaxMsgs, &s.MaxMsgs},
		{save.BNMsgs, &s.NMsgs},
		{save.BDisplayedMsgs, &s.DisplayedMsgs},
		{save.BDisplayedGNN, &s.DisplayedGNN},
		{save.BCouncilFlag, &s.CouncilFlag},
		{save.BNextCouncilTurn, &s.NextCouncilTurn},
		{save.BAntaranResources, &s.AntaranResources},
		{save.BAntaranInvasion, &s.AntaranInvasionChance},
		{save.BDisallowSaves, &s.DisallowSavesCode},
		{save.BLastEventYear, &s.LastEventYear},
		{save.BEventDelayCount, &s.EventDelayCount},
		{save.BAntarans, &s.Antarans},
		{save.BCurMapScale, &s.CurMapScale},
		{save.BCurMapX, &s.CurMapX},
		{save.BCurMapY, &s.CurMapY},
	}
	for _, t := range targets {
		if err := f.DecodeBlock(t.block, t.v); err != nil {
			return err
		}
	}
	return nil
}

// loadGameMessages are the Load_Game_ warnings for a save of another
// version, by language; %d is the slot number from 1.
var loadGameMessages = [5]string{
	"SAVE%d.GAM is invalid. Try another saved game.",
	"", "", "", "", // German, French, Spanish, Italian: not extracted yet
}

// loadGame is Load_Game_(slot): SAVE<slot+1>.GAM into the state, then the
// post-load fix-ups. It reports whether the game was loaded.
func (g *Game) loadGame(slot int) (bool, error) {
	name := fmt.Sprintf("SAVE%d.GAM", slot+1)
	g.e.MoxUpdate()
	f, err := save.Open(g.findFile(name))
	if err != nil {
		// The original prints a debug message and waits for a key.
		g.Log("Load_Game_: %v", err)
		return false, nil
	}
	if binary.LittleEndian.Uint32(f.Block(save.BVersion)) != save.Version {
		g.logBoxError(g.warningBox(fmt.Sprintf(loadGameMessages[0], slot+1)))
		g.e.V.SetPageOff()
		g.e.V.Fill(0, 0, 0x27f, 0x1df, 0)
		return false, nil
	}
	s := &g.state
	if err := s.readBlocks(f, slot); err != nil {
		return false, err
	}
	saved := f.Settings()
	seed := f.RandomSeed()
	g.gameType = int(s.GameType)
	g.disallowSavesCode = s.DisallowSavesCode != 0
	// _nebula_screen_seg is always allocated, so NUM_NEBULAS stays.

	g.setDefaults()
	s.Random.Set(seed)
	// Not ported yet: Cache_Player_Scientist_Anim_, Rebuild_WFI_List_.
	if s.NumNebulas != 0 {
		if err := g.loadNebulaPictures(); err != nil {
			return false, err
		}
	}
	for i := 0; i < int(s.NumStars); i++ {
		s.Stars[i].Owner = int8(s.starOwner(i, int(s.PlayerNum)))
	}
	g.buildShipIcons()
	g.setShipIconXYs()
	if g.moveableBoxSelected(2) {
		g.reloadFleetBoxData()
	}
	g.settings.ApplyUniverse(&saved)
	// Confirm_Planet_And_Star_Index_Link_ only reports broken links.
	s.updateStarOwners()
	if err := g.loadLeaderTitles(); err != nil {
		return false, err
	}
	for p := range s.Players {
		pl := &s.Players[p]
		if pl.LegacyResearch != 0 {
			pl.ResearchPoints = int32(pl.LegacyResearch)
			pl.LegacyResearch = 0
		}
	}
	for p := 0; p < int(s.NumPlayers); p++ {
		for j := 0; j < int(s.NumPlayers); j++ {
			s.Players[p].DelayedOrder[j] = -1
		}
	}
	return true, nil
}

// setDefaults is Set_Defaults_ with the part of Harold_Defaults_ that
// touches ported state; Ken_Init_ and Eric_Defaults_ are empty, Bill_Default_
// resets the paragraph system.
func (g *Game) setDefaults() {
	s := &g.state
	s.CurMapX, s.CurMapY = 0, 0
	g.currentScreen = ScreenReports
	s.CouncilWinner = -1
	g.haroldDefaults()
	g.billDefault()
}

// updateStarOwners is Update_Star_Owners_.
func (s *State) updateStarOwners() {
	for i := 0; i < int(s.NumStars); i++ {
		s.Stars[i].Owner = int8(s.starOwner(i, int(s.PlayerNum)))
	}
}

// starOwner is Star_Owner_: the viewing player when it has a colony at the
// star, else the player with most colonies there (first on a tie), or -1.
// Without Galactic Lore or Omniscience only known players count.
func (s *State) starOwner(star, player int) int {
	p := &s.Players[player]
	knowsAll := p.GalacticLore != 0 || p.Picks[save.PickOmniscient] != 0
	counts, _ := s.colonyOwners(star, !knowsAll)
	if counts[s.PlayerNum] > 0 {
		return int(s.PlayerNum)
	}
	best, owner := int16(0), -1
	for i := 0; i < int(s.NumPlayers); i++ {
		if best < counts[i] {
			best, owner = counts[i], i
		}
	}
	return owner
}

// colonyOwners is Colony_Owners_: colonies at the star per owner and the
// number of distinct owners. With onlyKnown, colonies of players the
// viewing player has no contact with are skipped.
func (s *State) colonyOwners(star int, onlyKnown bool) (counts [8]int16, owners int) {
	for orbit := 0; orbit < 5; orbit++ {
		pl := int(s.Stars[star].Planets[orbit])
		if pl < 0 {
			continue
		}
		c := int(s.Planets[pl].Colony)
		if c < 0 {
			continue
		}
		o := int(s.Colonies[c].Owner)
		if o < 0 {
			continue
		}
		var counted bool
		switch {
		case !onlyKnown || o == int(s.PlayerNum):
			counted = true
		case o >= int(s.NumPlayers):
			counted = false
		default:
			counted = s.Players[s.PlayerNum].Contact[o] != 0
		}
		if counted {
			if counts[o] == 0 {
				owners++
			}
			counts[o]++
		}
	}
	return counts, owners
}

// officerDataLBX is Get_Officer_Data_LBX_Name_.
func (g *Game) officerDataLBX() string {
	switch g.settings.Language() {
	case 1:
		return "HERODATG.LBX"
	case 2:
		return "HERODATF.LBX"
	case 3:
		return "HERODATS.LBX"
	case 4:
		return "HERODATI.LBX"
	}
	return "HERODATA.LBX"
}

// loadLeaderTitles is Load_Leader_Titles_: each leader's title is copied,
// strcpy-style, from the officer data of the current language.
func (g *Game) loadLeaderTitles() error {
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
	for i := range g.state.Leaders {
		src := t.Record(i)[0xf:]
		n := bytes.IndexByte(src, 0)
		if n < 0 {
			return fmt.Errorf("leader %d title has no terminator", i)
		}
		// strcpy writes into the leader record from its title on, NUL
		// included, which may run past the 20-byte title.
		raw := leaderBytes(&g.state.Leaders[i])
		copy(raw[0xf:], src[:n+1])
		if err := setLeaderBytes(&g.state.Leaders[i], raw); err != nil {
			return err
		}
	}
	return nil
}

func leaderBytes(l *save.Leader) []byte {
	var b bytes.Buffer
	binary.Write(&b, binary.LittleEndian, l)
	return b.Bytes()
}

func setLeaderBytes(l *save.Leader, b []byte) error {
	return binary.Read(bytes.NewReader(b), binary.LittleEndian, l)
}

// billDefault is Bill_Default_ (0x10206e); it reads settings +0xd4 before
// Load_Game_ applies the saved settings.
func (g *Game) billDefault() {
	pa := g.e.Paragraphs()
	if g.settings[0xd4] < 2 {
		g.setPlyrInfoBtns(4)
		pa.InitializeFormatted()
		return
	}
	pa.Tabs = [0x16]byte{}
	pa.Global = [0x5e]byte{}
	pa.VarPtr = [16]*int32{}
	pa.ResetVariables()
}

// setPlyrInfoBtns is Set_Plyr_Info_Btns_ (0x10a036): bits 4..6 of the
// viewing player's +0xe5f become n & 7.
func (g *Game) setPlyrInfoBtns(n int) {
	P := &g.state.Players[g.state.PlayerNum]
	P.InfoButtons = P.InfoButtons&0x8f | uint8(n&7)<<4
}
