package game

import (
	"github.com/openorion2/openorion2/internal/engine"
	"github.com/openorion2/openorion2/internal/gfx"
)

// The galaxy screen, MOX\MAINSCR.C and friends. Specs:
// docs/screens/main-screen-background.md, main-screen-stars.md,
// main-screen-ships.md, main-screen-panel.md.

const buffer0LBX = "BUFFER0.LBX"

type gfxPicture = gfx.Picture

// The map window, Set_Window(0x16, 0x16, 0x20f, 0x1a5).
const (
	mapX1, mapY1, mapX2, mapY2 = 0x16, 0x16, 0x20f, 0x1a5
)

// mainScreenState holds the main screen's globals.
type mainScreenState struct {
	// Pictures of Load_Main_Screen_Pictures_ (loaded once).
	gameButton, nextButton                *gfx.Picture
	coloniesButton, planetsButton         *gfx.Picture
	fleetsButton, zoomInButton            *gfx.Picture
	zoomOutButton, leadersButton          *gfx.Picture
	racesButton, infoButton               *gfx.Picture
	fleetBoxSelection, darkenedTurnButton *gfx.Picture
	playerLights                          [8]*gfx.Picture
	requestFrame                          [8]uint8           // 0x19c148: the request lights' frames by banner
	nebulaPict                            [4][4]*gfx.Picture // _nebula_pict_seg[n][zoom]

	farX, farY, midX, midY, nearX, nearY int16 // parallax offsets

	drawToBack, dontDrawToBack  bool // _draw_main_screen_to_back, _dont_draw_main_screen_to_back
	drawFleetScreenToBack       bool
	fadeInFlag, forceFade       bool // _main_screen_fade_in_flag, _force_main_screen_fade
	alreadyFaded                bool // _already_faded_into_main_screen
	reloadedShipPalette         bool
	atLeastOneReport            bool
	atLeastOneTacticalCombat    bool
	stardateAtBegOfTurn         uint32
	previousReport              int
	nextTurnHit                 bool
	checkNextTurnExitField      bool
	autoTurnExit                bool
	inAutoNextTurnMode          bool  // 0x19c196, _in_auto_next_turn_mode
	mouseClockImageNum          uint8 // 0x19c324, _mouse_clock_image_num
	cheatWarnMask               byte  // 0x19c19f, Warn_About_Cheating_Players_ (network)
	gnnShowing                  bool  // port-only: an event's text box is up, for tests
	replyShowing                bool  // port-only: the reply to a request is asked, for tests
	oldSystemIndex              int16 // _old_system_index (0x1999e2), -1 after each turn
	inReportsPhase              bool
	skipFields                  bool
	clearFleetFields            bool
	trackTime                   bool
	needToRestoreHelpList       bool // _need_to_restore_help_list (0x199f35)
	officerScreenReturnScreen   int
	officerScrnType             int
	drawingOfficerPopup         bool // _drawing_officer_popup
	mainscrPalette              byte
	zoomCount                   int16
	zoomIn                      bool
	stencilBoxID                int16
	etaStarID                   int16
	fleetShipScanned            int16
	systemIconScanned           int16
	galaxyMapScannedStar        int16
	scannedField                int16 // _scanned_field (0x1999bc)
	mainScreenScannedStar       int16
	mainScreenShipIconScanned   int16
	firstNodePtrInStack         int16
	shipIconPlayerID            int16
	animatedStar                int16
	selectedStarColorOffset     int8
	selectedStarColorMod        int8
	multiColoredLineStart       int16
	starButton                  []int16 // _star_button: Add_Star_Fields_'s disc fields
	multiColoredBoxColorStart   int16   // _multi_colored_box_color_start (0x199972)
	redPulseValue, redPulseIncr int8
	blackHoleAnim               [11]byte // _black_hole_anim_count; slot 10 aliases _h_string_ix
	nTwinklingStars             int16
	boxOffFlag                  [numBoxes]int16
	savedBoxes                  [numBoxes]int16
	savedBoxWasOpen             [numBoxes]bool

	zoomState
	galaxyMap galaxyMapState // box 1, galaxymap.go

	mouseButtonPressed       int  // _mouse_button_pressed: button of the current drag
	drawMainScreenStencilled bool // _draw_main_screen_stencilled

	// remapGrayPercentages is _remap_gray_percentages (0x1823e0); the .data
	// values 10..100 are replaced by Harold_Defaults_.
	remapGrayPercentages [10]int

	// Field numbers, -1000 when absent (Set_Main_Screen_Fields_To_Null_).
	fields mainScreenFields
}

type mainScreenFields struct {
	game, next, zoomIn, zoomOut                       int
	colonies, planets, fleets, leaders, races, info   int
	treasury, commandPoints, food, freighters, resrch int
	exitGame, grid, whole, debug                      int
	cheats                                            [5]int
	lastField                                         int
}

// haroldDefaults is the part of Harold_Defaults_ (0x792xx) for the main
// screen globals. Spec: docs/screens/load-game.md.
func (g *Game) haroldDefaults() {
	m := &g.ms
	s := &g.state
	m.remapGrayPercentages = [10]int{0, 4, 9, 0xf, 0x16, 0x1e, 0x27, 0x30, 0x38, 0x40}
	m.farX, m.farY, m.midX, m.midY, m.nearX, m.nearY = 0, 0, 0, 0, 0, 0
	m.boxOffFlag = [numBoxes]int16{-1, 0, 0, 0, 0}
	m.selectedStarColorOffset, m.selectedStarColorMod = 0, 1
	g.hotseatDefaults()
	for b := 0; b < numBoxes; b++ {
		m.savedBoxes[b] = -1000
		m.savedBoxWasOpen[b] = false
		s.MoveableBoxes[b].Content = m.boxOffFlag[b]
	}
	m.galaxyMapScannedStar = -1
	g.galaxyMapDefaults()
	m.previousReport = 0
	m.alreadyFaded = false
	m.animatedStar = -1
	s.StarBgIndex = 2
	m.firstNodePtrInStack = -1
	m.etaStarID = -1
	m.stencilBoxID = -1
	m.fleetShipScanned = -1
	m.systemIconScanned = -1
	m.mainscrPalette = 0xff
	m.stardateAtBegOfTurn = 0
	m.nextTurnHit = false
	m.officerScrnType = 1
	g.flt.supportStatus, g.flt.combatStatus = 1, 1
	m.fadeInFlag = false
	m.mainScreenShipIconScanned = -1
	s.CurMapScale = s.MaxMapScale
	m.redPulseValue, m.redPulseIncr = 0, 1
	for i := 0; i < 10; i++ {
		m.blackHoleAnim[i] = byte(2 * i)
	}
	// Clear_Twinkling_Star_Data_.
	for i := range s.Stars {
		s.Stars[i].TwinkleFrame, s.Stars[i].TwinkleEnd = -1, -1
	}
	m.nTwinklingStars = 0
	m.multiColoredLineStart = 0
}

// loadMainScreenPictures is Load_Main_Screen_Pictures_, run once at
// start-up (the pictures stay in _global_screen_seg).
func (g *Game) loadMainScreenPictures() error {
	m := &g.ms
	g.mapMarginX, g.mapMarginY = 0x21, 0x21
	pics := []**gfx.Picture{
		&m.gameButton, &m.nextButton, &m.coloniesButton, &m.planetsButton, &m.fleetsButton,
		&m.zoomInButton, &m.zoomOutButton, &m.leadersButton, &m.racesButton, &m.infoButton,
		&m.fleetBoxSelection, &m.darkenedTurnButton,
	}
	for i, p := range pics {
		var err error
		if *p, err = g.picture(buffer0LBX, 1+i); err != nil {
			return err
		}
	}
	for i := range m.playerLights {
		var err error
		if m.playerLights[i], err = g.picture(buffer0LBX, 0x109+i); err != nil {
			return err
		}
	}
	return g.loadGalaxyMapPictures()
}

// loadNebulaPictures is Load_Nebula_Pictures_.
func (g *Game) loadNebulaPictures() error {
	s := &g.state
	for n := 0; n < int(s.NumNebulas) && n < len(s.Nebulas); n++ {
		for z := 0; z < 4; z++ {
			p, err := g.picture("STARBG.LBX", 6+4*int(s.Nebulas[n].Picture)+z)
			if err != nil {
				return err
			}
			g.ms.nebulaPict[n][z] = p
		}
	}
	return nil
}

// cached is Buffer_Reload into _global_cache_seg: one shared picture per
// file and entry, so its frame counter carries over between calls.
func (g *Game) cached(file string, entry int) (*gfx.Picture, error) {
	k := cacheKey{file, entry}
	if p, ok := g.cache[k]; ok {
		return p, nil
	}
	p, err := g.picture(file, entry)
	if err != nil {
		return nil, err
	}
	g.cache[k] = p
	return p, nil
}

type cacheKey struct {
	file  string
	entry int
}

// moveableBoxSelected is Moveable_Box_Selected_: the box is open.
func (g *Game) moveableBoxSelected(b int) bool {
	return g.ms.boxOffFlag[b] < g.state.MoveableBoxes[b].Content
}

// setMainScreenFieldsToNull is Set_Main_Screen_Fields_To_Null_ for the
// fields ported so far.
func (g *Game) setMainScreenFieldsToNull() {
	f := &g.ms.fields
	for _, p := range []*int{&f.zoomIn, &f.zoomOut, &f.game, &f.races, &f.fleets, &f.colonies,
		&f.info, &f.next, &f.grid, &f.debug, &f.cheats[0], &f.cheats[1], &f.cheats[2], &f.cheats[3]} {
		*p = engine.NoField
	}
	for i := range g.ms.starButton {
		g.ms.starButton[i] = engine.NoField
	}
	g.nullFleetBoxFields()
}

// windowStatus is what Save_Window_Status_ keeps: the window and the clip
// flag. Restore_Window_Status_ puts both back.
type windowStatus struct {
	x1, y1, x2, y2 int
	clip           bool
}

func (g *Game) saveWindowStatus() windowStatus {
	v := g.e.V
	return windowStatus{v.WinX1, v.WinY1, v.WinX2, v.WinY2, v.Clipping}
}

func (g *Game) restoreWindowStatus(w windowStatus) {
	v := g.e.V
	v.WinX1, v.WinY1, v.WinX2, v.WinY2, v.Clipping = w.x1, w.y1, w.x2, w.y2, w.clip
}

// mapWindow sets the map window with clipping on, the pair of calls every
// map drawing routine starts with.
func (g *Game) mapWindow() {
	g.e.V.SetWindow(mapX1, mapY1, mapX2, mapY2)
	g.e.V.ClipOn()
}
