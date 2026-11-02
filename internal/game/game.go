// Package game ports the game's own code (the MOX modules) on top of the
// engine: screens, their loops and the game state.
package game

import (
	"crypto/tls"
	"fmt"
	"github.com/openorion2/openorion2/internal/announce"
	"image"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/openorion2/openorion2/internal/engine"
	"github.com/openorion2/openorion2/internal/font"
	"github.com/openorion2/openorion2/internal/gfx"
	"github.com/openorion2/openorion2/internal/host"
	"github.com/openorion2/openorion2/internal/lbx"
	"github.com/openorion2/openorion2/internal/lobby"
	"github.com/openorion2/openorion2/internal/netplay"
	"github.com/openorion2/openorion2/internal/save"
	"github.com/openorion2/openorion2/internal/update"
)

// Screen numbers, _current_screen.
const (
	ScreenQuit         = 7
	ScreenMainMenu     = 10
	ScreenNewGame      = 13
	ScreenHallOfFame   = 14
	ScreenMultiplayer  = 15
	ScreenHotseat      = 0x11
	ScreenHotseatSetup = 0x10
	ScreenMain         = 0
	ScreenGamePopup    = 8
	ScreenColony       = 1
	ScreenReports      = 0x27
	ScreenTurnSummary  = 0x28
	ScreenLanding      = 0x21
	ScreenFleets       = 4
	ScreenLeaders      = 0x1d
	ScreenTechChange   = 0x24

	ScreenColonizeInMain  = 0x1e
	ScreenOutpostInMain   = 0x1f
	ScreenTransportInMain = 0x22
	ScreenScrapFreighters = 0x23
	ScreenCommandPoints   = 0x2b
	ScreenStarDistances   = 0x1c

	ScreenRaces         = 6
	ScreenInfo          = 9
	ScreenColonySummary = 0x14
	ScreenPlanetSummary = 0x20
)

// Game is the game state the ported screens share.
type Game struct {
	e   *engine.Engine
	dir string
	lbx map[string]*lbx.Archive

	currentScreen, previousScreen, returnScreen int
	fullDrawScreen                              bool // _FULL_DRAW_SCREEN_FLAG
	panMainMenuPanel                            bool // _pan_main_menu_panel
	disallowSavesCode                           bool
	globalEsc                                   bool // _global_esc
	gameType                                    int
	screenData                                  [5]int16 // _screen_data (0x199830..0x199838)

	mainMenu mainMenuState
	boltBuf  boltBuffer // graphics_buffer of the bolt lines
	gp       gamePopupState

	loadedNewGame bool // _loaded_new_game

	settings save.Settings // _settings, MOX.SET
	// _combat_init (0x17d853): Tactical_Combat_ sets it, the off-screen
	// bombardment reads whatever the last one left.
	combatInit    int16
	combatInitSet bool
	state         State
	ms            mainScreenState
	exe           *exeImage
	stacks        shipStacks
	mm            mapMouse
	sd            systemDisplay
	fb            fleetBox
	flt           fltScreen // FLT1.C, FLT2.C
	off           offScreen // OFFICER.C
	// prevLeaders is _prev_leaders, set while the leaders screen runs.
	prevLeaders *[maxLeaders]prevLeader
	logged      map[string]bool
	// staleScreen lists, while trackStale is set (frame oracles), the screen
	// pixels drawn from memory the port does not model since the last flip.
	staleScreen []image.Point
	trackStale  bool
	noCouncil   bool               // Options.AutopilotNoCouncil
	autopilot   bool               // Options.Autopilot
	nameHL      *starNameHighlight // .data, initialised on first use

	prodCaches *[3]colonyProdCache // the three production caches (0x1aa358..), lifetime of the program

	mergingRelocations bool          // _merging_relocations
	ub                 userBoxState  // MSGBOX and TEXTBOX.C statics
	lobbyTLS           *tls.Config   // port-only: the online lobby's TLS settings (tests); nil: the system's
	update             *updateCheck  // port-only: the lookup for a newer release
	lobbyScr           *lobbyHomeScr // port-only: the lobby's screen while it is up
	lobbyJoinScr       *lobbyJoinScr // port-only: the lobby's JOIN box while it is up
	lobbyConn          *lobby.Conn   // port-only: the lobby, while a game made there is played
	buttonArt          *buttonArt    // port-only: the button art mapped to the palette last drawn with
	rep                reportState
	rstrings           []string           // RSTRING<lang>.LBX, loaded once
	dip                diplomacyScreen    // DIP-SCRN.C statics
	mp                 mainPopups         // MAINPUPS.C statics
	slider             sliderPopup        // Slider_Popup_'s statics
	credits            credits            // the main menu's credits
	ccp                colonyCombatPopup  // Colony_Combat_Popup_'s statics
	gcs                groundCombatScreen // Colony_Combat_Screen_'s statics
	sr                 scienceRoomState   // SCIENCE.C statics
	colonyHandle       int                // _colony_handle, the colony screen's colony
	currentEvent       int                // _current_event (EVENTS.C)
	gnn                gnnState           // EVENTS.C, the GNN screen
	hof                hofState           // HISCORE.C, the score screen and the Hall of Fame
	ar                 antaranRoomState   // ANTAROOM.C statics
	ts                 techSelectState    // TECH.C statics
	tsum               turnSummaryState   // TURNSUM.C statics
	ps                 planetSummaryState
	psStat             planetSummaryStatics // PLNTSUM.C's initialised statics
	col                colonyGlobals        // COLONY.C globals
	cscr               colonyScreenState    // COLONY.C, the colony screen
	bq                 buildQueueState      // COLBLDG.C, the build queue popup
	des                designState          // DESIGN.C, DESBOX.C, the design screen
	rf                 refitState           // COLREFIT.C, the refit popups
	cs                 colonySummaryState
	csStat             colonySummaryStatics // COLSUM.C's initialised statics
	xp                 xportState           // COLXPORT.C, the transport popup
	info               infoState            // INFO.C
	race               raceScreenState      // RACESCRN.C
	ng                 newGameState         // NEWGAME.C, FAKEBUTS.C click through fields
	rsel               raceSelectState      // RACESEL.C, RACEOPT.C
	patchFlags         map[int]byte         // bytes of the 1.40 patch's data the patch changed
	activePlayer       [8]uint8             // _active_player
	activeCount        int16                // _active_count
	raceReportJimHack  bool                 // _race_report_jim_hack
	listKey            bool                 // Options.ListKey
	diploKeys          bool                 // Options.DiploKeys
	newGameKeys        bool                 // Options.NewGameKeys
	newGameHook        func(g *Game)        // Options.NewGameHook
	newGameSavedHook   func(g *Game)        // Options.NewGameSavedHook
	cinVolume          [2]int16             // 0x18319f, 0x1831a1: volumes Do_Cinematic_Popup_Setup_ saved
	logoFade           int                  // 0x19acf4: where Slow_Fade_In_ stopped
	mus                music                // MOX.C music globals
	landSound          [4]int               // 0x19f398, 0x19f390, 0x19f38c, 0x19f39c: the landing's sounds
	hotSeatFlag        int16                // _hot_seat_flag (written only)
	hs                 hotseat              // hotseat.go
	tax                taxPopup             // taxrate.go
	drag               dragonPopup          // dragon.go
	loknarTechs        []int16              // Give_Player_Loknar_Special_'s techs (loknar.go)
	lastSmackSound     int                  // sound bytes of the last cinematic (tests)
	lastSmackFrames    int                  // frames it decoded (tests)
	hsSetup            hotseatSetup         // hotseat_setup.go
	mpSetup            *mpSetup             // multiplayer.go, nil outside the screen
	netScr             *netScreen           // netstart.go: _g_mp_screen of the network screens
	cmdLineSeed        uint32               // _cmd_line_seed
	newGameStarted     bool                 // a race was picked; Init_New_Game_ would run
	gen                genState             // HAROLD's generation globals
	adv                *advCiv              // Advanced_Civilization_Colonies_'s arena blocks
	genProg            genProgress          // the generation's progress message
	firstMapDone       bool                 // _first_time_map_is_generated (initially 1) cleared
	// userWantsSpaceMonsters is _user_wants_n_space_monsters, -1 from main2_.
	userWantsSpaceMonsters int8
	// offscreenAskForHirePopup is _offscrn_ask_for_hire_popup (0x19c542).
	offscreenAskForHirePopup bool

	renamedHomeStar [8]bool // _given_chance_to_rename_home_star
	antaranTables   *antaranTables
	// _spy_bonuses (0x1ace78): offence and defence per player, unsaved;
	// entries of eliminated players keep their last values.
	spyBonus [8][2]int16
	stale    []staleSpan // bytes the original takes from stale stack
	net      netGame     // the network game (net.go)
	// shipWorth is Player_Maintenance_'s local table (0x1aa3f0 points to
	// it while Kill_Ships_ runs).
	shipWorth      []int32
	ai             *aiArena    // between Compute_AI_Data_ and Deallocate_AI_Data_
	aiKept         *aiArena    // AIDATA.C statics after Deallocate_AI_Data_ (no arena)
	interceptStack [72]int8    // the original's stack under interceptAt (turn_aiintercept.go)
	cf             *combatFind // Search_For_Battles_' tables while it runs
	ct             combatTargets
	asked          *[8][8]byte // 0x1aa378: attacks a human has confirmed
	cc             combatCur
	combatUI       combatUI // answers the combat popups; nil declines
	qc             *qcombat // strategic combat records (screen_seg), kept between battles
	// messedUpFleetPalette is _messed_up_fleet_palette (0x183578): a combat
	// box covered the screen; Search_For_Battles_ then reloads the palette.
	messedUpFleetPalette bool
	tc                   *tactical    // tactical combat records and globals, kept between battles
	needs                aiBuildNeeds // AIBUILD.C statics, from Compute_Empire_Building_Needs_
	// _best_category (0x1ab10c), Set_Competition_Tech_Values_'s results.
	bestCategory [12]int16
	proposal     diplomacyProposal
	council      councilState
	allAIGame    bool   // _all_ai_game, set by Make_All_AI_Game_
	release      string // port-only: update.Version, which network games compare
	// port-only: the announcements (announce.go) and those already shown
	announceFeed                *announce.Feed
	announceLobby, announceGame map[string]bool
	// Margins that keep placed ships off the map edge (0x19996e, 0x199954):
	// 0x21 once the main screen is loaded, 0x25 during galaxy generation.
	mapMarginX, mapMarginY int16
	gridX, gridY           int // _main_screen_grid_x/y

	moxFontColorsOffset byte                      // _mox_font_colors_offset
	hStrings            []string                  // H_Message_ strings
	cache               map[cacheKey]*gfx.Picture // _global_cache_seg

	// Now, if set, replaces the host clock (Add_Ongoing_Event_Msgs_).
	Now func() time.Time

	// stepOverride, if set, may take over step k of Next_Turn_Calc_ (tests
	// put the original's state in for steps not ported yet).
	stepOverride func(k int, st turnStep) bool

	// Log receives notes about paths that are not ported yet.
	Log func(format string, args ...any)
}

// Options are development switches that are not part of the original.
type Options struct {
	// NewGameHook (tests) sees the game where Init_New_Game_ would start,
	// NewGameSavedHook right after its Save_Game_ (the oracle's gen.GAM).
	NewGameHook, NewGameSavedHook func(g *Game)

	// Load, when 1..10, skips the main menu and continues SAVE<Load>.GAM
	// the way the menu's Continue does.
	Load int
	// Mouse, when set, warps the pointer there at start (for screenshots).
	Mouse *[2]int
	// ListKey gives list fields and response boxes hot key 1, so tests
	// answer them by keys.
	ListKey bool
	// DiploKeys gives the diplomacy lists' rows and the system map's stars
	// hot keys (tests).
	DiploKeys bool
	// Autopilot plays the human of a single-player game by the AI
	// (Make_Player_Into_AI_).
	Autopilot bool
	// AutopilotNoCouncil (port only) holds the council without its screens:
	// the human votes for himself when nominated, else abstains.
	AutopilotNoCouncil bool
	// Logos plays Draw_Logos_ (the logos and the intro) first, as main2_
	// does; tests start at the main menu without them.
	Logos bool
	// NewGameKeys gives the new game screens' click-only fields hot keys
	// (tests, docs/screens/new-game.md).
	NewGameKeys bool
	// Seed, when not 0, is _cmd_line_seed: main2_ seeds the RNG with it
	// instead of the clock (Randomize_), as the original's command line can.
	Seed uint32

	// Net is the transport behind Net_Create_Game_, Net_Join_Game_ and
	// Net_Update_Game_List_ (netplay.TCP, or netplay.Mem in tests); nil
	// leaves Net_Init_ failing as without IPX. Port-only.
	Net netplay.Network
	// NetLog gets every message sent and received (tests). Port-only.
	NetLog func(NetRecord)
	// NetName is the player's name for the transport (the original passes
	// "myname"); NetSkipBoxes skips the port-only create box (-host or
	// -join gave the address). Port-only.
	NetName      string
	NetSkipBoxes bool
	// NetDropEnd: a lost player ends the game for everyone instead of becoming
	// an AI. Port-only (docs/net/hardening.md).
	NetDropEnd bool
	// NetCheck: the desync check after each broadcast; NetCheckDir gets the own
	// save of a mismatch. Port-only, debugging (docs/net/hardening.md).
	NetCheck    bool
	NetCheckDir string

	// Update, if set, looks for a newer release at the start and offers it
	// on the main menu (port-only, updateui.go).
	Update *update.Client
	// Announce, if set, is the project's announcements (port-only,
	// announce.go).
	Announce *announce.Feed

	// testHook (tests) sees the game before the save loads.
	testHook func(g *Game)
}

// Run plays the game in dir until the player quits or the window closes.
// A network error ends it with an *ExitError (Exit_With_Message_).
func Run(sys *host.System, dir string, opt Options) (err error) {
	g, err := newGame(sys, dir)
	if err != nil {
		return err
	}
	defer g.recoverRunExit(&err)
	g.net.network, g.net.log = opt.Net, opt.NetLog
	g.net.player, g.net.noLobbyBoxes = opt.NetName, opt.NetSkipBoxes
	g.net.dropEnd, g.net.check, g.net.checkDir = opt.NetDropEnd, opt.NetCheck, opt.NetCheckDir
	if opt.Mouse != nil {
		sys.WarpMouse(opt.Mouse[0], opt.Mouse[1])
	}
	g.listKey = opt.ListKey
	g.e.SetListKeyOne(opt.ListKey)
	g.newGameKeys = opt.NewGameKeys
	g.newGameHook, g.newGameSavedHook = opt.NewGameHook, opt.NewGameSavedHook
	g.diploKeys = opt.DiploKeys
	if g.diploKeys {
		g.e.SetListRowKeys(diploRowKeys)
	}
	g.autopilot = opt.Autopilot
	g.noCouncil = opt.AutopilotNoCouncil
	g.cmdLineSeed = opt.Seed
	if opt.Logos {
		// main2_ (0x10164): then palette 5 and the arrow again.
		if err := g.drawLogos(); err != nil {
			return err
		}
		if err := g.e.LoadPalette(5, 0, 255); err != nil {
			return err
		}
		g.e.SetMouseList(engine.MouseArrow)
	}
	g.playBackgroundMusic() // main2_ 0x10193
	// main2_ (0x101a2): the command line seed, else Randomize_.
	if g.cmdLineSeed != 0 {
		g.state.Random.Set(g.cmdLineSeed)
	} else {
		g.randomize()
	}
	g.startUpdateCheck(opt.Update)
	g.announceFeed = opt.Announce
	g.e.ControlState, g.e.ControlAct = g.agentState, g.agentAct
	if opt.testHook != nil {
		opt.testHook(g)
	}
	if opt.Load >= 1 && opt.Load <= 10 {
		if err := g.continueSave(opt.Load); err != nil {
			return err
		}
	}
	return g.screenControl()
}

// newGame does what main2_ does before the first screen: settings, fonts,
// palette, cursor, the executable's tables and the global pictures and
// strings. It starts on the main menu with the panel sliding out.
func newGame(sys *host.System, dir string) (*Game, error) {
	g := &Game{dir: dir, lbx: map[string]*lbx.Archive{}, cache: map[cacheKey]*gfx.Picture{}, release: update.Version, Log: func(f string, a ...any) {
		fmt.Fprintf(os.Stderr, "openorion2: "+f+"\n", a...)
	}}
	fonts, err := g.archive("FONTS.LBX")
	if err != nil {
		return nil, err
	}
	g.e = engine.New(sys, fonts)
	// The executable's tables, built in (the default settings read
	// strings from them).
	if g.exe, err = newExeImage(g.findFile("ORION2.EXE"), func(f string, a ...any) { g.Log(f, a...) }); err != nil {
		return nil, err
	}
	g.loadSettings()
	fe, err := fonts.Entry(0)
	if err != nil {
		return nil, err
	}
	f, err := font.Parse(fe)
	if err != nil {
		return nil, err
	}
	g.e.SetFont(f)
	g.e.SetUpdateFunction(g.e.MoxUpdate)
	g.e.NetUpdate = g.moxNetUpdate
	// The drivers' start-up sets SOUND.LBX as the sound file; without it
	// the game runs silent. main2_ then applies the settings (0x1014a).
	if err := g.e.InitSound(dir, "SOUND.LBX"); err != nil {
		g.Log("sound: %v", err)
	}
	g.mus = music{-1, -1, -1, -1}
	g.setSoundVolumeToSettings()
	g.e.SetHelpFunc(g.drawHelpEntry)
	// main2_ loads palette 5 and the arrow cursor before the first screen.
	if err := g.e.LoadPalette(5, 0, 255); err != nil {
		return nil, err
	}
	g.e.SetMouseList(engine.MouseArrow)
	// Load_Pictures_ and Load_Mox_Global_Strings_.
	g.e.SetCycleDirection(int32(g.exe.u32(cycleDirectionInit)))
	g.initPlanetSummaryStatics()
	g.initColonyStatics()
	g.initColonyScreenStatics()
	g.info.techList = g.exeI16(techReviewChoice)
	if err := g.loadMainScreenPictures(); err != nil {
		return nil, err
	}
	if err := g.loadHStrings(); err != nil {
		return nil, err
	}
	g.currentScreen = ScreenMainMenu
	g.panMainMenuPanel = true
	g.userWantsSpaceMonsters = -1
	g.firstMapDone = g.exeU8(firstTimeMapInit) == 0
	return g, nil
}

// continueSave loads SAVE<n>.GAM as the main menu's Continue does.
func (g *Game) continueSave(n int) error {
	ok, err := g.loadGame(n - 1)
	if err != nil {
		return err
	}
	g.previousScreen = ScreenMainMenu
	g.afterMainMenuLoad(ok, 0xcf)
	return nil
}

// screenControl is Screen_Control_: run the current screen until one sets
// _current_screen to 7. Each screen leaves its own number in
// _previous_screen. Screens not ported yet go back to the main menu as if
// they had run.
func (g *Game) screenControl() error {
	for g.currentScreen != ScreenQuit {
		g.globalEsc = false
		if g.currentScreen == ScreenMainMenu && g.net.lobbyPlaying {
			// port-only: a game made in the online lobby ends back there.
			g.currentScreen = ScreenMultiplayer
		}
		switch cur := g.currentScreen; cur {
		case ScreenMainMenu:
			if err := g.MainMenuScreen(); err != nil {
				return err
			}
			g.previousScreen = cur
		case ScreenHallOfFame:
			if err := g.hallOfFameScreen(-1); err != nil {
				return err
			}
			g.currentScreen = ScreenMainMenu
			g.previousScreen = cur
		case ScreenMain:
			if err := g.mainScreen(); err != nil {
				return err
			}
			g.previousScreen = cur
		case ScreenMultiplayer:
			if err := g.multiPlayerScreen(); err != nil {
				return err
			}
			g.previousScreen = cur
		case ScreenHotseatSetup:
			if err := g.hotseatScreen(); err != nil {
				return err
			}
			g.previousScreen = cur
		case ScreenHotseat:
			if err := g.hotseatSelectPlayer(); err != nil {
				return err
			}
			g.previousScreen = cur
		case ScreenNextTurn:
			if err := g.nextTurn(); err != nil {
				return err
			}
			g.previousScreen = cur
		case ScreenReports:
			if err := g.reportsScreen(); err != nil {
				return err
			}
			g.previousScreen = cur
		case ScreenTurnSummary:
			if err := g.turnSummaryPopup(); err != nil {
				return err
			}
			g.previousScreen = cur
		case ScreenGamePopup:
			if err := g.gamePopup(); err != nil {
				return err
			}
			g.previousScreen = cur
		case ScreenColonySummary:
			if err := g.colonySummaryScreen(); err != nil {
				return err
			}
			g.previousScreen = cur
		case ScreenPlanetSummary:
			if err := g.planetSummaryScreen(); err != nil {
				return err
			}
			g.previousScreen = cur
		case ScreenFleets:
			if err := g.fleetScreen(); err != nil {
				return err
			}
			g.previousScreen = cur
		case ScreenLeaders:
			if err := g.officersScreen(); err != nil {
				return err
			}
			g.previousScreen = cur
		case ScreenInfo:
			if err := g.infoScreen(); err != nil {
				return err
			}
			g.previousScreen = cur
		case ScreenRaces:
			if err := g.raceScreen(); err != nil {
				return err
			}
			g.previousScreen = cur
		case ScreenTechChange:
			if err := g.techChange(); err != nil {
				return err
			}
			g.previousScreen = cur
		case ScreenStartNet, ScreenJoinNet, ScreenLoadNet:
			// Screen_Control_ 0x10653, 0x1066e, 0x1076e: on success the
			// reports; Load_Net_Screen_ shares Join_Net_Screen_'s tail (0x16).
			start := g.startNetScreen
			switch cur {
			case ScreenJoinNet:
				start = g.joinNetScreen
			case ScreenLoadNet:
				start = g.loadNetScreen
			}
			ok, err := start()
			if err != nil {
				return err
			}
			if ok {
				g.currentScreen = ScreenReports
				g.net.inGame = true // port-only: drops are handled from here on
			}
			g.previousScreen = cur
			if cur == ScreenLoadNet {
				g.previousScreen = ScreenJoinNet
			}
		case ScreenNetNextTurn:
			// Screen_Control_ 0x10753: it sets the screens itself.
			if err := g.netNextTurn(); err != nil {
				return err
			}
		case ScreenNewGame:
			if err := g.newGameScreenControl(); err != nil {
				return err
			}
			g.previousScreen = cur
		case ScreenColony:
			if err := g.colonyScreen(); err != nil {
				return err
			}
			g.previousScreen = cur
		case ScreenBuildQueue:
			if err := g.buildQueuePopup(); err != nil {
				return err
			}
			g.previousScreen = cur
		case ScreenDesign:
			if err := g.designScreen(); err != nil {
				return err
			}
			g.previousScreen = cur
		case ScreenLanding:
			if err := g.colonyLandingScreen(); err != nil {
				return err
			}
			g.previousScreen = cur
		case ScreenColonizeInMain:
			if err := g.planetColonizationInMainScreen(); err != nil {
				return err
			}
			g.previousScreen = cur
		case ScreenOutpostInMain:
			if err := g.buildingOutpostsInMainScreen(); err != nil {
				return err
			}
			g.previousScreen = cur
		case ScreenTransportInMain:
			if err := g.unloadingTransportsInMainScreen(); err != nil {
				return err
			}
			g.previousScreen = cur
		case ScreenScrapFreighters:
			if err := g.scrapFreighters(); err != nil {
				return err
			}
			g.previousScreen = cur
		case ScreenStarDistances:
			if err := g.distanceBetweenStarsScreen(); err != nil {
				return err
			}
			g.previousScreen = cur
		case ScreenCommandPoints:
			if err := g.showCommandPointsScreen(); err != nil {
				return err
			}
			g.previousScreen = cur
		default:
			g.Log("screen %#x is not ported yet; back to the main menu", cur)
			g.previousScreen = cur
			g.currentScreen = ScreenMainMenu
		}
	}
	return nil
}

// archive opens an LBX file by name, once.
func (g *Game) archive(name string) (*lbx.Archive, error) {
	if a, ok := g.lbx[name]; ok {
		return a, nil
	}
	a, err := lbx.Open(filepath.Join(g.dir, name))
	if err != nil {
		return nil, err
	}
	g.lbx[name] = a
	return a, nil
}

// picture loads a fresh copy of a picture, as Far_Reload does: its frame
// counter starts from the file again.
func (g *Game) picture(file string, entry int) (*gfx.Picture, error) {
	a, err := g.archive(file)
	if err != nil {
		return nil, err
	}
	e, err := a.Entry(entry)
	if err != nil {
		return nil, err
	}
	p, err := gfx.Parse(e)
	if err != nil {
		return nil, fmt.Errorf("%s entry %d: %w", file, entry, err)
	}
	return p, nil
}

// dirMatch is DIR(pattern): whether a file matching the DOS wildcard pattern
// exists in the game directory. Names compare case-insensitively.
func (g *Game) dirMatch(pattern string) bool {
	ents, err := os.ReadDir(g.dir)
	if err != nil {
		return false
	}
	for _, e := range ents {
		if ok, _ := filepath.Match(strings.ToLower(pattern), strings.ToLower(e.Name())); ok {
			return true
		}
	}
	return false
}
