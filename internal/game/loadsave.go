package game

import (
	"bytes"
	"fmt"
	"os"

	"github.com/openorion2/openorion2/internal/engine"
	"github.com/openorion2/openorion2/internal/font"
	"github.com/openorion2/openorion2/internal/gfx"
	"github.com/openorion2/openorion2/internal/save"
)

// The game popup (MOX\GAME.C, LOADSAVE.C): the GAME button's menu with its
// options, load and save pages, and Load Game on the main menu. Spec:
// docs/screens/game-popup.md.

const gameLBX = "GAME.LBX"

// Strings and tables of the popup in the image.
const (
	invalidSaveString   = 0x1795a4 // "* INVALID *"
	autoSaveString      = 0x17886b // "(Auto Save)"
	noDescriptionString = 0x178877 // "<< no description >>"
	fileDateFormat      = 0x1795ce // "%3s %2d, %-4d  %2d:%2d"
	saveDescFormat      = 0x1795be // "%s, %s, %d %s"
	mainGameHelpList    = 0x18197a // _main_game_screen_help_list, 12 entries
	loadGameHelpList    = 0x1819fc // 7 entries
	saveGameHelpList    = 0x181a42 // 7 entries
	optionsGameHelpList = 0x181a88 // 18 entries
)

// optionAltKeys are the "(ALT-Fn)" strings printed after the options' rows
// on screen; the slide-in bitmap uses its own set, which skips ALT_F4.
var (
	optionAltKeys       = [13]uint32{0x179624, 0x17962d, 0x179636, 0, 0x17963f, 0x179648, 0x179651, 0x17965a, 0x179663}
	optionAltKeysBitmap = [13]uint32{0x1795e5, 0x1795ee, 0x1795f7, 0, 0x179600, 0x179609, 0x179612, 0x17961b}
)

// optionLabels are the H strings of the options' rows.
var optionLabels = [13]int{0xa6, 0xa7, 0xa8, 0xa9, 0xaa, 0xab, 0xac, 0xad, 0xae, 0xaf, 0xb0, 0xb1, 0x187}

// Slot states of Check_For_Saved_Games_.
const (
	slotOK = iota
	slotMissing
	slotInvalid
	slotNetwork
)

// Pages of the popup (_screen_data).
const (
	pageMain = iota
	pageOptions
	pageLoad
	pageSave
)

// _game_anims indices: offset / 4.
const (
	animMainBG     = 0
	animSaveBtn    = 1
	animLoadBtn    = 2
	animNewBtn     = 3
	animQuitBtn    = 4
	animOptionsBtn = 5
	animReturnBtn  = 6
	animSlider     = 7
	animOptionsBG  = 8
	animCheckBox   = 9
	animOptionsOK  = 10
	animLoadBG     = 11
	animLoadOK     = 12
	animCancel     = 13
	animSaveBG     = 14
	animSaveOK     = 15
	animTypeIcons  = 16 // 16..18: single, hotseat, network/modem
	animDarkLoad   = 23
	animDarkNew    = 24
)

// gamePopupState is _game_anims, _game_popup_fields and the globals around
// them. Field numbers are -1 until added, as the original's 0xff fill.
type gamePopupState struct {
	anims    [25]*gfx.Picture
	pushedUp bool // _game_anims +100: the popup already slid away

	stardates   [10]int32 // fields +0x00
	loadBtn     int       // +0x28
	saveBtn     int       // +0x2a
	quitBtn     int       // +0x2c
	newBtn      int       // +0x2e
	optionsBtn  int       // +0x30
	returnBtn   int       // +0x32
	slotFields  [10]int   // +0x38
	debugField  int       // +0x4c
	optBoxes    [13]int   // +0x52
	optionsOK   int       // +0x6c
	cancelBtn   int       // +0x6e
	escField    int       // +0x70
	wholeField  int       // +0x72
	music, sfx  int16     // +0x74, +0x76: slider positions 0..0x9b
	musicOn     int16     // +0x78
	sfxOn       int16     // +0x7a
	slotLines   [10]int   // +0x7c
	options     [13]int16 // +0x90
	status      [10]byte  // +0xaa
	gameTypes   [10]byte  // +0x226
	loadOK      int       // +0x230
	saveOK      int       // +0x232
	normal      font.Colors
	highlight   font.Colors
	optLabels   [13]int // +0x244
	language    int16   // +0x25e
	musicField  int     // _music_level_field
	sfxField    int     // _sfx_level_field
	darkButtons bool    // !_dont_draw_darkened_buttons

	baseX, baseY int
	months       [12]string
	emptySlot    string
	dates        [10]string
	stardateStr  [10]string

	toBitmap *gfx.Picture // _bitmap while _print_data_to_bitmap is set
}

// gamePopup is _Game_Popup_ (0x8012f), screen 8.
func (g *Game) gamePopup() error {
	e := g.e
	first := true
	g.loadedNewGame = false
	e.ClearFields()
	if g.screenData[0] < 0 || g.screenData[0] > 3 {
		g.screenData[0] = pageMain
	}
	g.gp = gamePopupState{}
	gp := &g.gp
	gp.darkButtons = true
	g.resetGamePopupFields()
	e.V.SetPageOff()
	if err := g.drawMiniMainScreen(); err != nil {
		return err
	}
	e.V.CopyOffToBack()
	e.V.SetRefreshStencil()
	g.fullDrawScreen = false
	if err := g.loadGamePopupPictures(); err != nil {
		return err
	}
	g.setCurrentGameOptionFlags()
	done := false
	for !done {
		e.DeactivateAutoFunction()
		if first && g.settings[6] != 0 {
			e.V.SetPageOff()
			if err := g.animateGamePopup(gp.anims[animMainBG], 0, 1, pageMain); err != nil {
				return err
			}
		} else {
			e.V.SetPageOff()
			if err := g.drawMiniMainScreen(); err != nil {
				return err
			}
			e.V.CopyOffToBack()
			e.V.SetRefreshStencil()
			g.fullDrawScreen = false
		}
		first = false
		page := g.screenData[0]
		if page == pageLoad || page == pageSave {
			g.setUpLoadSavePopup()
			g.returnScreen = g.previousScreen
		}
		e.ClearFields()
		if err := g.addGamePopupFields(); err != nil {
			return err
		}
		var err error
		switch page {
		case pageMain:
			done, err = g.doMainGamePopup()
		case pageOptions:
			done, err = g.doOptionsGamePopup()
		case pageLoad:
			done, _, err = g.doLoadGamePopup()
		case pageSave:
			done, err = g.doSaveGamePopup()
		}
		if err != nil {
			return err
		}
	}
	g.updateGameSettings()
	if err := g.saveSessionRelatedSettings(); err != nil {
		return err
	}
	g.currentScreen = g.returnScreen
	e.DeactivateAutoFunction()
	e.ClearFields()
	if g.settings[6] != 0 && !gp.pushedUp {
		gp.darkButtons = false
		e.V.SetPageOff()
		if err := g.drawMainGamePopup(); err != nil {
			return err
		}
		if err := e.TogglePages(); err != nil {
			return err
		}
		e.MarkTime()
		if err := e.ReleaseTime(7); err != nil {
			return err
		}
		e.V.SetPageOff()
		bg := gp.anims[animMainBG]
		if err := g.animateGamePopup(bg, bg.Height, -1, pageMain); err != nil {
			return err
		}
	}
	if g.currentScreen == ScreenNewGame {
		return e.FastFadeOut()
	}
	if g.loadedNewGame {
		return nil
	}
	e.ClearFields()
	e.DisableDrawAllFields()
	g.ms.drawToBack = true
	e.V.SetPageOff()
	if err := g.drawMiniMainScreen(); err != nil {
		return err
	}
	if err := e.TogglePages(); err != nil {
		return err
	}
	e.EnableDrawAllFields()
	return nil
}

// resetGamePopupFields is the 0xff fill of _game_popup_fields.
func (g *Game) resetGamePopupFields() {
	gp := &g.gp
	for _, p := range []*int{&gp.loadBtn, &gp.saveBtn, &gp.quitBtn, &gp.newBtn, &gp.optionsBtn, &gp.returnBtn,
		&gp.debugField, &gp.optionsOK, &gp.cancelBtn, &gp.escField, &gp.wholeField, &gp.loadOK, &gp.saveOK} {
		*p = -1
	}
	for i := range gp.slotFields {
		gp.stardates[i], gp.slotFields[i], gp.slotLines[i], gp.status[i], gp.gameTypes[i] = -1, -1, -1, 0xff, 0xff
	}
	for i := range gp.optBoxes {
		gp.optBoxes[i], gp.optLabels[i], gp.options[i] = -1, -1, -1
	}
	gp.music, gp.sfx, gp.musicOn, gp.sfxOn, gp.language = -1, -1, -1, -1, -1
}

// loadGamePopupPictures is Load_Game_Popup_Pictures_ (0x7ea5c): GAME.LBX
// 0..0x13 with 8 from 0x1d, and the darkened LOAD and NEW at 0x1b, 0x1c.
func (g *Game) loadGamePopupPictures() error {
	gp := &g.gp
	load := func(slot, entry int) error {
		p, err := g.picture(gameLBX, entry)
		gp.anims[slot] = p
		return err
	}
	for i := 0; i <= 0x13; i++ {
		entry := i
		if i == animOptionsBG {
			entry = 0x1d
		}
		if err := load(i, entry); err != nil {
			return err
		}
	}
	if err := load(animDarkLoad, 0x1b); err != nil {
		return err
	}
	return load(animDarkNew, 0x1c)
}

// setCurrentGameOptionFlags is Set_Current_Game_Option_Flags_ (0x7efef).
func (g *Game) setCurrentGameOptionFlags() {
	gp, st := &g.gp, &g.settings
	gp.music = int16(int(st[0x14]) * 0x9b / 100)
	gp.sfx = int16(int(st[0x12]) * 0x9b / 100)
	gp.musicOn, gp.sfxOn = int16(st[0x13]), int16(st[0x11])
	gp.options[0] = int16(st[0])
	gp.options[1] = 0
	if g.gameType == save.GameSingle {
		gp.options[1] = int16(st[1])
	}
	for i := 2; i < 13; i++ {
		gp.options[i] = int16(st[i+1])
	}
	gp.language = int16(st[0xd2])
	if gp.language > 4 {
		gp.language = 0
	}
}

// updateGameSettings is Update_Game_Settings_ (0x7f14c).
func (g *Game) updateGameSettings() {
	gp, st := &g.gp, &g.settings
	st[0], st[1] = byte(gp.options[0]), byte(gp.options[1])
	for i := 2; i < 13; i++ {
		st[i+1] = byte(gp.options[i])
	}
	st[0xd2] = byte(gp.language)
	if st[0xd2] > 4 {
		st[0xd2] = 0
	}
}

// helpListAt reads a help list of n 10-byte records from the image.
func (g *Game) helpListAt(addr uint32, n int) []engine.HelpEntry {
	list := make([]engine.HelpEntry, n)
	for i := range list {
		a := addr + uint32(i*10)
		list[i] = engine.HelpEntry{ID: int(g.exeI16(a)), X1: int(g.exeI16(a + 2)), Y1: int(g.exeI16(a + 4)),
			X2: int(g.exeI16(a + 6)), Y2: int(g.exeI16(a + 8))}
	}
	return list
}

func (g *Game) setGamePopupHelpList(addr uint32, n int) {
	g.e.DeactivateHelpList()
	g.e.SetHelpList(g.helpListAt(addr, n))
}

// restoreAfterRefusal is what Do_Main_Game_Popup_ draws after NEW or QUIT
// is refused: the main screen on the back page.
func (g *Game) restoreAfterRefusal() error {
	e := g.e
	e.V.SetPageBack()
	e.V.Fill(0, 0, 0x27f, 0x1df, 0)
	if err := g.drawMiniMainScreen(); err != nil {
		return err
	}
	if err := g.drawMainMainScreen(); err != nil {
		return err
	}
	e.V.CopyBackToOff()
	return nil
}

// doMainGamePopup is Do_Main_Game_Popup_ (0x7dd41); done leaves the popup.
func (g *Game) doMainGamePopup() (bool, error) {
	e, gp := g.e, &g.gp
	esc := false
	e.AssignAutoFunction(g.drawMainGamePopup, 1)
	g.setGamePopupHelpList(mainGameHelpList, 12)
	leave, done := false, false
	for !leave {
		r, err := e.GetInput()
		if err != nil {
			return false, err
		}
		switch r {
		case gp.returnBtn:
			leave, done = true, true
		case gp.newBtn:
			ok, err := g.gamePopupUserBox(g.hMessage(0xba), 1)
			if err != nil {
				return false, err
			}
			if ok {
				g.gameType = save.GameSingle
				g.returnScreen = ScreenNewGame
				leave, done = true, true
				break
			}
			if err := g.restoreAfterRefusal(); err != nil {
				return false, err
			}
			leave = true
		case gp.quitBtn:
			ok, err := g.gamePopupUserBox(g.hMessage(0xbb), 1)
			if err != nil {
				return false, err
			}
			if !ok {
				if err := g.restoreAfterRefusal(); err != nil {
					return false, err
				}
				leave = true
				break
			}
			if err := g.saveGame(9); err != nil {
				return false, err
			}
			g.settings[0x15] = 10
			if err := g.saveSessionRelatedSettings(); err != nil {
				return false, err
			}
			if g.gameType == 2 || g.gameType == 3 {
				if err := g.humanAbdicates(int(g.state.PlayerNum)); err != nil {
					return false, err
				}
				g.netUninit()
			}
			g.returnScreen = ScreenQuit
			leave, done = true, true
		case gp.optionsBtn:
			g.screenData[0], leave = pageOptions, true
		case gp.loadBtn:
			e.AssignAutoFunction(g.drawLoadSaveGamePopup, 1)
			g.screenData[0], leave = pageLoad, true
		case gp.saveBtn:
			if g.disallowSavesCode {
				if _, err := g.gamePopupUserBox(g.hMessage(0xb9), 0); err != nil {
					return false, err
				}
				leave, done = true, true
			} else {
				g.screenData[0], leave = pageSave, true
			}
		case gp.musicField:
			g.setMusicForGamePopup()
		case gp.sfxField:
			g.setSoundForGamePopup(true)
		}
		if r == gp.escField {
			leave, done, esc = true, true, true
		}
		if !leave {
			e.V.SetPageOff()
			if err := g.drawMainGamePopup(); err != nil {
				return false, err
			}
			if err := e.TogglePages(); err != nil {
				return false, err
			}
		}
	}
	if esc && g.settings[6] != 0 {
		e.ClearFields()
		bg := gp.anims[animMainBG]
		if err := g.animateGamePopup(bg, bg.Height, -1, pageMain); err != nil {
			return false, err
		}
		gp.pushedUp = true
	}
	return done, nil
}

// setMusicForGamePopup is Set_Music_For_Game_Popup_ (0x80892): the music
// slider; above 5% the music comes on (and starts), else it stops.
func (g *Game) setMusicForGamePopup() {
	gp, st, e := &g.gp, &g.settings, g.e
	level := int(gp.music) * 100 / 0x9b * 100 / 100
	if level < 6 {
		e.StreamOff()
		g.stopBackgroundMusic()
		st[0x13], level, gp.musicOn = 0, 0, 0
	} else {
		gp.musicOn = 1
		if st[0x13] == 0 {
			e.StreamOn()
			st[0x13] = 1
			g.playBackgroundMusic()
		}
	}
	st[0x14] = byte(level)
	e.SetStreamMaxVolume(int(int16(level)))
	e.SetStreamVolume(int(int16(level)))
}

// setSoundForGamePopup is Set_Sound_For_Game_Popup_ (0x80918): the sound
// slider, and with test its sample (Play_Sound_ 0x21).
func (g *Game) setSoundForGamePopup(test bool) {
	gp, st, e := &g.gp, &g.settings, g.e
	st[0x12] = byte(int(gp.sfx) * 100 / 0x9b * 100 / 100)
	g.ub.globalVar = 0 // 0x8093c
	if st[0x12] > 5 {
		if st[0x11] == 0 {
			e.SoundOn()
		}
		st[0x11] = 1
		e.SetSoundMaxVolume(int(st[0x12]))
		if test {
			e.PlaySound(0x21)
		}
		gp.sfxOn = 1
		return
	}
	st[0x12] = 0
	e.SetSoundMaxVolume(0)
	st[0x11], gp.sfxOn = 0, 0
}

// doOptionsGamePopup is Do_Options_Game_Popup_ (0x7e00f).
func (g *Game) doOptionsGamePopup() (bool, error) {
	e, gp := g.e, &g.gp
	lang := gp.language
	g.setGamePopupHelpList(optionsGameHelpList, 0x12)
	e.AssignAutoFunction(g.drawOptionsGamePopup, 1)
	e.ClearFields()
	if err := g.addGamePopupFields(); err != nil {
		return false, err
	}
	leave, done := false, false
	for !leave {
		r, err := e.GetInput()
		if err != nil {
			return false, err
		}
		for i := range gp.options {
			if r == gp.optBoxes[i] || r == gp.optLabels[i] {
				gp.options[i] = int16(b2i(gp.options[i] == 0))
				break
			}
		}
		switch r {
		case gp.escField:
			leave, done = true, true
		case gp.optionsOK:
			g.screenData[0], leave = pageMain, true
		}
		if !leave {
			e.V.SetPageOff()
			if err := g.drawOptionsGamePopup(); err != nil {
				return false, err
			}
			if err := e.TogglePages(); err != nil {
				return false, err
			}
		}
	}
	// 0x7e118: a changed language would be stored (5 and up as 0),
	// Reload_Language_Strings_ and Write_Language_Setting_File_ would run.
	// Nothing on the page writes +0x25e (only Set_Current_Game_Option_Flags_
	// does, from settings +0xd2), so the branch is never taken.
	_ = lang
	return done, nil
}

// doMainmenuLoadScreen is Do_Mainmenu_Load_Screen_ (0x810c7): the popup over
// MAINMENU.LBX 0x15 until it is left; whether a game was loaded.
func (g *Game) doMainmenuLoadScreen() (bool, error) {
	e := g.e
	bg, err := g.picture(mainMenuLBX, 0x15)
	if err != nil {
		return false, err
	}
	e.V.SetRefreshStencil()
	e.V.SetPageBack()
	e.V.Fill(0, 0, 0x289, 0x1df, 0)
	if err := e.Draw(0, 0, bg); err != nil {
		return false, err
	}
	e.V.CopyBackToOff()
	if err := e.TogglePages(); err != nil {
		return false, err
	}
	e.DeactivateHelpList()
	g.previousScreen = ScreenMainMenu
	for {
		done, loaded, err := g.mainmenuLoadGamePopup()
		if err != nil || done {
			return loaded, err
		}
	}
}

// mainmenuLoadGamePopup is Mainmenu_Load_Game_Popup_ (0x804b7); done is its
// out parameter.
func (g *Game) mainmenuLoadGamePopup() (done, loaded bool, err error) {
	g.screenData[0] = pageLoad
	if g.mainMenu.first, err = g.picture(mainMenuLBX, 0x15); err != nil {
		return false, false, err
	}
	g.gp = gamePopupState{}
	g.resetGamePopupFields()
	g.loadMonthNames()
	if err := g.loadMainmenuLoadGamePopup(); err != nil {
		return false, false, err
	}
	g.setUpLoadSavePopup()
	done, loaded, err = g.doLoadGamePopup()
	g.e.ClearFields()
	return done, loaded, err
}

// loadMainmenuLoadGamePopup is Load_Mainmenu_Load_Game_Popup_ (0x803d9):
// GAME.LBX 0x14..0x1a as the load page's pictures.
func (g *Game) loadMainmenuLoadGamePopup() error {
	for i, slot := range []int{animLoadBG, animLoadOK, animCancel, animTypeIcons, animTypeIcons + 1, animTypeIcons + 2, animTypeIcons + 3} {
		p, err := g.picture(gameLBX, 0x14+i)
		if err != nil {
			return err
		}
		g.gp.anims[slot] = p
	}
	return nil
}

// loadMonthNames is Load_Month_Names_ (0x7e913): H strings 0xbe..0xc9.
func (g *Game) loadMonthNames() {
	for i := range g.gp.months {
		g.gp.months[i] = g.hMessage(0xbe + i)
	}
}

// setUpLoadSavePopup is Set_Up_Load_Save_Popup_ (0x80556).
func (g *Game) setUpLoadSavePopup() {
	gp, s := &g.gp, &g.state
	g.getSavedGameDescriptions()
	for i := range s.Descriptions {
		d := &s.Descriptions[i]
		clear(d[cLen(d[:]):])
	}
	g.getSaveGameDateStrings()
	g.getStarDateStrings()
	for i := range s.Descriptions {
		if s.Descriptions[i][0] == 0 && gp.stardateStr[i] != "" {
			setCString(s.Descriptions[i][:], fmt.Sprintf(cFormat(g.hMessage(0xcb)), i))
		}
	}
	if sel := int8(g.settings[0x15]); sel < 0 || sel >= 10 || gp.stardates[sel] <= 0 {
		g.settings[0x15] = 0xff
	}
}

// getSavedGameDescriptions is Get_Saved_Game_Descriptions_ (0x11c83): the
// description and stardate of every SAVE<n>.GAM.
func (g *Game) getSavedGameDescriptions() {
	gp, s := &g.gp, &g.state
	for i := range s.Descriptions {
		d := s.Descriptions[i][:]
		data, err := os.ReadFile(g.findFile(fmt.Sprintf("SAVE%d.GAM", i+1)))
		if err != nil {
			setCString(d, g.hMessage(0x184))
			gp.stardates[i] = -1
			continue
		}
		var hdr [4 + 0x25 + 4]byte
		copy(hdr[:], data)
		copy(d, hdr[4:4+0x25])
		gp.stardates[i] = int32(le32(hdr[0x29:]))
		switch {
		case le32(hdr[:]) != save.Version:
			gp.stardates[i] = -1
			setCString(d, g.hMessage(0x184))
		case i == 9:
			copy(d, g.exe.bytes(autoSaveString, 12))
		case d[0] == 0:
			copy(d, g.exe.bytes(noDescriptionString, 21))
		}
	}
}

// getSaveGameDateStrings is Get_Save_Game_Date_Strings_ (0x7e752) with
// Get_File_Date_String_ (0x7e609): the files' DOS time stamps.
func (g *Game) getSaveGameDateStrings() {
	gp := &g.gp
	for i := range gp.dates {
		gp.dates[i] = ""
		fi, err := os.Stat(g.findFile(fmt.Sprintf("SAVE%d.GAM", i+1)))
		if err != nil {
			continue
		}
		t := fi.ModTime().Local()
		b := []byte(fmt.Sprintf(cFormat(cStringAt(g.exe, fileDateFormat)), cMonth(gp.months[int(t.Month())-1]),
			t.Day(), t.Year(), t.Hour(), t.Minute()))
		for _, k := range []int{0xe, 0x11} {
			if k < len(b) && b[k] == ' ' {
				b[k] = '0'
			}
		}
		gp.dates[i] = string(b[:min(len(b), 0x18)])
	}
}

// cMonth is a month name as _months keeps it: 6-byte slots, 5 characters.
func cMonth(s string) string { return s[:min(len(s), 5)] }

// getStarDateStrings is Get_Star_Date_Strings_ (0x7e7ff): "Stardate 3536.3"
// for a positive stardate, "* INVALID *" for a negative one.
func (g *Game) getStarDateStrings() {
	gp := &g.gp
	for i, sd := range gp.stardates {
		switch {
		case sd < 0:
			gp.stardateStr[i] = cStringAt(g.exe, invalidSaveString)
		case sd == 0:
			gp.stardateStr[i] = ""
		default:
			n := fmt.Sprint(sd)
			gp.stardateStr[i] = g.hMessage(0xca) + n[:len(n)-1] + "." + n[len(n)-1:]
		}
	}
}

// doLoadGamePopup is Do_Load_Game_Popup_ (0x7da76): a click on a slot loads
// it, LOAD loads the last slot used.
func (g *Game) doLoadGamePopup() (done, loaded bool, err error) {
	e, gp, s := g.e, &g.gp, &g.state
	g.loadMonthNames()
	gp.emptySlot = g.hMessage(0x184)
	// _loaded_debug_save = 0: the debug slot is not ported.
	inGame := g.currentScreen != ScreenMainMenu
	if inGame {
		g.setGamePopupHelpList(loadGameHelpList, 7)
	}
	e.AssignAutoFunction(g.drawLoadSaveGamePopup, 1)
	e.ClearFields()
	if err := g.addGamePopupFields(); err != nil {
		return false, false, err
	}
	leave := 0
	for leave == 0 {
		r, err := e.GetInput()
		if err != nil {
			return false, false, err
		}
		slot := -1
		for i, f := range gp.slotFields {
			if r == f {
				slot = i
				break
			}
		}
		if sel := int8(g.settings[0x15]); r == gp.loadOK && slot == -1 && sel >= 0 && sel < 10 {
			// _draw_down = 1: written, never read.
			slot = int(sel)
		}
		if slot >= 0 {
			if st := gp.status[slot]; st == slotOK {
				if g.settings[6] != 0 && g.previousScreen == ScreenMain {
					e.ClearFields()
					gp.pushedUp = true
					e.V.SetPageOff()
					bg := gp.anims[animLoadBG]
					if err := g.animateGamePopup(bg, bg.Height, -1, pageLoad); err != nil {
						return false, false, err
					}
				}
				ok, err := g.loadGame(slot)
				if err != nil {
					return false, false, err
				}
				g.settings[0x15] = byte(slot)
				loaded = ok
				if err := g.saveSessionRelatedSettings(); err != nil {
					return false, false, err
				}
				if !ok {
					if _, err := g.gamePopupUserBox(fmt.Sprintf(cFormat(g.hMessage(0xb8)), slot), 3); err != nil {
						return false, false, err
					}
					g.returnScreen = g.previousScreen
				} else {
					g.loadedNewGame = true
					g.returnScreen = ScreenReports
					if g.gameType == save.GameHotseat {
						g.returnScreen = ScreenHotseat
					}
				}
				if inGame && g.ms.stardateAtBegOfTurn != 0 {
					if err := g.fadeIntoScreen(g.drawMiniMainScreen, 0); err != nil {
						return false, false, err
					}
					g.ms.drawToBack = true
				}
				leave, done = 1, true
			} else {
				leave = 2
				if err := g.badGameFileMessage(st, slot); err != nil {
					return false, false, err
				}
			}
		}
		switch r {
		case gp.cancelBtn:
			g.screenData[0] = pageMain
			leave = 1
			done = done || !inGame
		case gp.escField:
			loaded = false
			leave, done = 1, true
		}
		if leave == 0 {
			e.V.SetPageOff()
			if err := e.InvokeAutoFunction(); err != nil {
				return false, false, err
			}
			if err := e.TogglePages(); err != nil {
				return false, false, err
			}
		}
	}
	for i := range s.Descriptions {
		removeEmbeddedSpecialCodes(s.Descriptions[i][:])
	}
	return done, loaded, nil
}

// doSaveGamePopup is Do_Save_Game_Popup_ (0x7e154): a slot's line edits its
// description; picking the slot again, Enter or SAVE saves there.
func (g *Game) doSaveGamePopup() (bool, error) {
	e, gp, s := g.e, &g.gp, &g.state
	saved, leave, done := false, false, false
	useTyped, saveNow := false, false
	gp.emptySlot = g.hMessage(0x184)
	g.loadMonthNames()
	g.setGamePopupHelpList(saveGameHelpList, 7)
	e.ClearFields()
	sel := -1
	e.AssignAutoFunction(g.drawLoadSaveGamePopup, 1)
	if err := g.addGamePopupFields(); err != nil {
		return false, err
	}
	for !leave {
		r, err := e.GetInput()
		if err != nil {
			return false, err
		}
		if r != 0 {
			for i := range gp.slotFields {
				if r == gp.slotFields[i] {
					if i == sel {
						useTyped, saveNow = true, true
					}
					sel = i
					break
				}
				if r == gp.slotLines[i] {
					if i == sel {
						// _draw_down = 2: written, never read.
						saveNow = true
						break
					}
					e.ActivateInputField(gp.slotFields[i])
					sel = i
				}
			}
		}
		if r == gp.saveOK {
			if sel < 0 || sel > 9 {
				if last := int8(g.settings[0x15]); last >= 0 && last < 10 {
					sel, saveNow = int(last), true
				}
			} else {
				saveNow = true
			}
		}
		if saveNow && sel >= 0 && sel < 10 {
			saveNow = false
			g.settings[0x15] = byte(sel)
			d := s.Descriptions[sel][:]
			switch {
			case d[0] == 0 || save.CString(d) == gp.emptySlot:
				setCString(d, g.defaultSaveDescription())
			case useTyped:
				copyCString(d, e.ContinuousString())
			}
			if err := g.saveGame(sel); err != nil {
				return false, err
			}
			if err := g.saveSessionRelatedSettings(); err != nil {
				return false, err
			}
			g.getSaveGameDateStrings()
			g.getStarDateStrings()
			saved, leave, done = true, true, true
		}
		abs := r
		if abs < 0 {
			abs = -abs
		}
		switch {
		case r == gp.cancelBtn, r < 0 && gp.slotFields[0] <= abs && abs <= gp.slotFields[9]:
			g.screenData[0], leave = pageMain, true
		case r == gp.escField:
			leave, done = true, true
		}
		if !leave {
			e.V.SetPageOff()
			if err := g.drawLoadSaveGamePopup(); err != nil {
				return false, err
			}
			if err := e.TogglePages(); err != nil {
				return false, err
			}
		}
	}
	for i := range s.Descriptions {
		removeEmbeddedSpecialCodes(s.Descriptions[i][:])
	}
	if saved && g.settings[6] != 0 {
		e.ClearFields()
		bg := gp.anims[animSaveBG]
		if err := g.animateGamePopup(bg, bg.Height, -1, pageSave); err != nil {
			return false, err
		}
		gp.pushedUp = true
	}
	return done, nil
}

// defaultSaveDescription is the description Do_Save_Game_Popup_ writes into
// an empty slot: "Emperor, Race, 3 colonies", cut to 36 characters.
func (g *Game) defaultSaveDescription() string {
	s := &g.state
	n := g.nColonies(int(s.PlayerNum))
	word := g.hMessage(0xbd)
	if n == 1 {
		word = g.hMessage(0xbc)
	}
	p := &s.Players[s.PlayerNum]
	d := fmt.Sprintf(cFormat(cStringAt(g.exe, saveDescFormat)), save.CString(p.EmperorName[:]), save.CString(p.RaceName[:]), n, word)
	return d[:min(len(d), 0x24)]
}

// nColonies is N_Colonies_: colonies, not outposts, of player p.
func (g *Game) nColonies(p int) int {
	s := &g.state
	n := 0
	for i := 0; i < int(s.NumColonies); i++ {
		if int(s.Colonies[i].Owner) == p && s.Colonies[i].IsOutpost == 0 {
			n++
		}
	}
	return n
}

// fadeIntoScreen is Fade_Into_Screen_ (0x8f1c4) with palette entry pal.
func (g *Game) fadeIntoScreen(draw func() error, pal int) error {
	e := g.e
	e.DisableDrawAllFields()
	e.V.ClearPaletteChanges(0, 0xff)
	e.SetMouseList(engine.MouseNone)
	if err := e.FastFadeOut(); err != nil {
		return err
	}
	if err := e.LoadPalette(pal, 0, 0xff); err != nil {
		return err
	}
	e.V.SetPaletteChanges(0, 0xff)
	e.RefreshPalette()
	if err := e.DarkenPalette(100); err != nil {
		return err
	}
	e.SetMouseList(engine.MouseArrow)
	e.V.SetPageOff()
	if err := draw(); err != nil {
		return err
	}
	e.V.ClearPaletteChanges(0, 0xff)
	if !g.ms.reloadedShipPalette {
		if err := e.FastFadeIn(); err != nil {
			return err
		}
	} else {
		e.SetFadePercent(0)
		if err := e.DarkenPalette(0); err != nil {
			return err
		}
	}
	if err := e.TogglePages(); err != nil {
		return err
	}
	g.ms.reloadedShipPalette = false
	e.EnableDrawAllFields()
	return nil
}

// addGamePopupFields is Add_Game_Popup_Fields_ (0x7d061) for the current
// page.
func (g *Game) addGamePopupFields() error {
	e, gp, s := g.e, &g.gp, &g.state
	gp.musicField, gp.sfxField = engine.NoField, engine.NoField
	var w, h int
	var err error
	add := func(dst *int, x, y int, p *gfx.Picture, hot string) {
		if err == nil {
			*dst, err = e.AddButtonField(x, y, "", p, hot, 0x28)
		}
	}
	switch page := g.screenData[0]; page {
	case pageMain:
		bg := gp.anims[animMainBG]
		w, h = bg.Width, bg.Height
		gp.baseX, gp.baseY = 0x90, 0x19
		bx, by := gp.baseX, gp.baseY
		if g.gameType == save.GameSingle || g.gameType == save.GameHotseat {
			add(&gp.loadBtn, bx+0x93, by+0x2b, gp.anims[animLoadBtn], "L")
			add(&gp.newBtn, bx+0x28, by+0x58, gp.anims[animNewBtn], "N")
		} else {
			gp.loadBtn, gp.newBtn = engine.NoField, engine.NoField
		}
		add(&gp.saveBtn, bx+0x28, by+0x2b, gp.anims[animSaveBtn], "S")
		add(&gp.quitBtn, bx+0x93, by+0x58, gp.anims[animQuitBtn], "Q")
		add(&gp.optionsBtn, bx+0x28, by+0x133, gp.anims[animOptionsBtn], "O")
		add(&gp.returnBtn, bx+0x97, by+0x133, gp.anims[animReturnBtn], "\x1b")
		if err != nil {
			return err
		}
		if gp.musicField, err = e.AddScrollField(bx+0x3e, by+0xc2, 0, 0x9c, 0, 0x9c, 0x9b, 0xc, &gp.music, "", 0); err != nil {
			return err
		}
		if gp.sfxField, err = e.AddScrollField(bx+0x3e, by+0xd8, 0, 0x9c, 0, 0x9c, 0x9b, 0xc, &gp.sfx, "", 0); err != nil {
			return err
		}
	case pageOptions:
		bg := gp.anims[animOptionsBG]
		w, h = bg.Width, bg.Height
		gp.baseX, gp.baseY = 0x90, 0x19
		box := gp.anims[animCheckBox]
		for i := range gp.optBoxes {
			x, y := gp.baseX+0x1a, gp.baseY+0x29+i*0x11
			if gp.optBoxes[i], err = e.AddHiddenField(x, y, x+box.Width, y+box.Height, "", '('); err != nil {
				return err
			}
			if gp.optLabels[i], err = e.AddHiddenField(x+0x17, y, x+0xd9, y+box.Height, "", ')'); err != nil {
				return err
			}
		}
		add(&gp.optionsOK, gp.baseX+0x61, gp.baseY+0x152, gp.anims[animOptionsOK], "A")
		if err != nil {
			return err
		}
	case pageLoad:
		bg := gp.anims[animLoadBG]
		w, h = bg.Width, bg.Height
		if g.currentScreen == ScreenMainMenu {
			gp.baseX, gp.baseY = (0x280-w)/2, (0x1e0-h)/2
		} else {
			gp.baseX, gp.baseY = 0x90, 0x19
		}
		g.checkForSavedGames()
		gp.normal, gp.highlight = g.gamePopupColors(2)
		e.Text().Special = gp.highlight
		g.embedSelectedSlot()
		for i := range s.Descriptions {
			x, y := gp.baseX+0x1d, gp.baseY+0x18+i*0x1f
			if gp.status[i] == slotInvalid {
				copy(s.Descriptions[i][:], g.exe.bytes(invalidSaveString, 12))
			}
			gp.gameTypes[i] = g.gameTypeFromSavedGame(i)
			if gp.slotFields[i], err = e.AddHiddenField(x, y, x+0xc5, y+0x18, "", ')'); err != nil {
				return err
			}
		}
		if gp.debugField, err = e.AddHiddenField(0x275, 0x1d5, 0x27f, 0x1df, "D", '3'); err != nil {
			return err
		}
		add(&gp.loadOK, gp.baseX+0x25, gp.baseY+0x151, gp.anims[animLoadOK], "L")
		add(&gp.cancelBtn, gp.baseX+0xab, gp.baseY+0x152, gp.anims[animCancel], "C")
		if err != nil {
			return err
		}
	case pageSave:
		bg := gp.anims[animSaveBG]
		w, h = bg.Width, bg.Height
		gp.baseX, gp.baseY = 0x90, 0x19
		if g.currentScreen == ScreenMainMenu {
			gp.baseX, gp.baseY = (0x280-w)/2, (0x1e0-h)/2
		}
		g.checkForSavedGames()
		gp.normal, gp.highlight = g.gamePopupColors(2)
		t := e.Text()
		t.Special = gp.highlight
		g.embedSelectedSlot()
		for i := range s.Descriptions {
			x, y := gp.baseX+0x1d, gp.baseY+0x18+i*0x1f
			t.SetStyle(2, gp.normal)
			if gp.status[i] == slotInvalid {
				copy(s.Descriptions[i][:], g.exe.bytes(invalidSaveString, 12))
			}
			gp.gameTypes[i] = g.gameTypeFromSavedGame(i)
			if gp.slotFields[i], err = e.AddContinuousStringInputField(x-3, y, 200, t.Height(), s.Descriptions[i][:], 0x1e, 0, nil, 0x29, 0); err != nil {
				return err
			}
		}
		for i := range gp.slotLines {
			y := gp.baseY + 0x18 + i*0x1f
			fh := t.Height()
			if gp.slotLines[i], err = e.AddHiddenField(gp.baseX+0x1d, fh+y+1, gp.baseX+0xf4, fh*2+y, "", ')'); err != nil {
				return err
			}
		}
		gp.debugField = engine.NoField
		add(&gp.saveOK, gp.baseX+0x25, gp.baseY+0x152, gp.anims[animSaveOK], "S")
		add(&gp.cancelBtn, gp.baseX+0xab, gp.baseY+0x152, gp.anims[animCancel], "C")
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("Add_Game_Popup_Fields_ for page %d is not ported", page)
	}
	if gp.wholeField, err = e.AddHiddenField(gp.baseX, gp.baseY, gp.baseX+w, gp.baseY+h, "", 0); err != nil {
		return err
	}
	gp.escField, err = e.AddHiddenField(0, 0, 0x27f, 0x1df, "\x1b", ')')
	return err
}

// embedSelectedSlot wraps the selected slot's description in the special
// colour codes and unwraps the others.
func (g *Game) embedSelectedSlot() {
	s := &g.state
	sel := int(int8(g.settings[0x15]))
	for i := range s.Descriptions {
		if i == sel {
			embedSpecialColorCodes(s.Descriptions[i][:])
		} else if s.Descriptions[i][0] == 3 {
			removeEmbeddedSpecialCodes(s.Descriptions[i][:])
		}
	}
}

// checkForSavedGames is Check_For_Saved_Games_ (0x7d954).
func (g *Game) checkForSavedGames() {
	gp := &g.gp
	for i := range gp.status {
		name := fmt.Sprintf("SAVE%d.GAM", i+1)
		if !g.dirMatch(name) {
			gp.status[i] = slotMissing
			continue
		}
		data, _ := os.ReadFile(g.findFile(name))
		var hdr [0x2d]byte
		copy(hdr[:], data)
		if int32(le32(hdr[0x29:])) <= 0 || le32(hdr[:]) != save.Version {
			gp.status[i] = slotInvalid
			continue
		}
		gp.status[i] = slotOK
		if t := g.gameTypeFromSavedGame(i); t == save.GameNetwork || t == save.GameModem {
			gp.status[i] = slotNetwork
		}
	}
}

// gameTypeFromSavedGame is Get_Game_Type_From_Saved_Game_ (0x11fcc): byte
// 0x2d of the file, 0xff without one.
func (g *Game) gameTypeFromSavedGame(slot int) byte {
	data, err := os.ReadFile(g.findFile(fmt.Sprintf("SAVE%d.GAM", slot+1)))
	if err != nil || len(data) <= 0x2d {
		return 0xff
	}
	return data[0x2d]
}

// gamePopupColors is Get_Game_Popup_Colors_ (0x7e6c1): the normal and the
// highlighted colours of a style.
func (g *Game) gamePopupColors(style int) (font.Colors, font.Colors) {
	if g.currentScreen == ScreenMainMenu {
		return g.fontColors2(style, 0xfe, 0xff), g.fontColors2(style, 0xfb, 0xfc)
	}
	g.moxFontColorsOffset = 0x59
	n := g.fontColors2(style, 0x56, 0x5b)
	g.moxFontColorsOffset = 0x5b
	return n, g.fontColors2(style, 0x59, 0x7e)
}

// drawPic is Draw, or Add_Picture_To_Bitmap while printing to a bitmap.
func (g *Game) drawPic(x, y int, p *gfx.Picture) error {
	if b := g.gp.toBitmap; b != nil {
		return b.AddPicture(x, y, p)
	}
	return g.e.Draw(x, y, p)
}

// printText is Print, or Print_To_Bitmap while printing to a bitmap.
func (g *Game) printText(x, y int, s string) error {
	t := g.e.Text()
	if b := g.gp.toBitmap; b != nil {
		return t.ToBitmap(font.Target{Pix: b.Pixels(), Stride: b.Width, H: b.Height}, x, y, []byte(s))
	}
	return t.Print(x, y, s)
}

// drawMainGamePopup is _Draw_Main_Game_Popup_ (0x7f701).
func (g *Game) drawMainGamePopup() error {
	e, gp := g.e, &g.gp
	in := e.AutoInput()
	bx, by := gp.baseX, gp.baseY
	if gp.toBitmap != nil {
		bx, by = 0, 0
	}
	x := bx + 0x3e
	if gp.toBitmap == nil {
		if err := g.drawMiniMainScreen(); err != nil {
			return err
		}
		if err := e.Draw(bx, by, gp.anims[animMainBG]); err != nil {
			return err
		}
		if gp.darkButtons {
			if err := e.Draw(bx+0x93, by+0x2b, gp.anims[animDarkLoad]); err != nil {
				return err
			}
			if err := e.Draw(bx+0x28, by+0x58, gp.anims[animDarkNew]); err != nil {
				return err
			}
		}
		gp.darkButtons = true
		g.fullDrawScreen = false
	}
	if g.gameType != save.GameSingle && gp.toBitmap != nil {
		for _, d := range []struct{ a, x, y int }{{animDarkLoad, 0x93, 0x2b}, {animDarkNew, 0x28, 0x58}} {
			p := gp.anims[d.a]
			tmp := gfx.NewBlankBitmap(p.Width, p.Height)
			if err := tmp.AddPicture(0, 0, p); err != nil {
				return err
			}
			if err := gp.toBitmap.AddPicture(d.x, d.y, tmp); err != nil {
				return err
			}
		}
	}
	if err := g.drawClippedSlider(x, by+0xc2, x+int(gp.music), by+0xcf); err != nil {
		return err
	}
	if err := g.drawClippedSlider(x, by+0xd8, x+int(gp.sfx), by+0xe5); err != nil {
		return err
	}
	switch in {
	case gp.musicField:
		g.setMusicForGamePopup()
	case gp.sfxField:
		g.setSoundForGamePopup(false)
	}
	return nil
}

// drawClippedSlider draws the slider picture cut to (x1, y1)..(x2, y2).
func (g *Game) drawClippedSlider(x1, y1, x2, y2 int) error {
	v, p := g.e.V, g.gp.anims[animSlider]
	if b := g.gp.toBitmap; b != nil {
		tmp := gfx.NewBlankBitmap(x2-x1+1, y2-y1+1)
		if err := tmp.AddPicture(0, 0, p); err != nil {
			return err
		}
		return b.AddPicture(x1, y1, tmp)
	}
	v.SetWindow(x1, y1, x2, y2)
	v.ClipOn()
	err := g.e.Draw(x1, y1, p)
	v.ResetWindow()
	v.ClipOff()
	return err
}

// drawOptionsGamePopup is _Draw_Options_Game_Popup_ (0x7fa28).
func (g *Game) drawOptionsGamePopup() error {
	e, gp := g.e, &g.gp
	t := e.Text()
	g.moxFontColorsOffset = 0x55
	c := g.fontColors2(2, 0x55, 0x7e)
	if gp.toBitmap == nil && g.fullDrawScreen {
		e.V.SetPageOff()
		if err := g.drawMiniMainScreen(); err != nil {
			return err
		}
		e.V.CopyOffToBack()
		e.V.SetRefreshStencil()
		g.fullDrawScreen = false
	}
	if gp.toBitmap == nil {
		if err := e.Draw(gp.baseX, gp.baseY, gp.anims[animOptionsBG]); err != nil {
			return err
		}
	}
	t.SetStyleShadow(2, c, font.Outline)
	t.SetOutlineColor(0, c[0])
	t.SetSpacing(1)
	box := gp.anims[animCheckBox]
	bx, by := gp.baseX+0x1a, gp.baseY+0x29
	alt := optionAltKeys
	if gp.toBitmap != nil {
		bx, by = 0x1a, 0x29
		alt = optionAltKeysBitmap
	} else {
		for i := range gp.options {
			box.CurrentFrame = min(int(gp.options[i]), box.Frames-1)
			if err := e.Draw(bx, by+i*0x11, box); err != nil {
				return err
			}
		}
	}
	tx := bx + 4 + box.Width
	for i, h := range optionLabels {
		y := by + i*0x11 + 2
		if err := g.printText(tx, y, g.hMessage(h)); err != nil {
			return err
		}
		if alt[i] != 0 {
			if err := g.printText(tx+0x96, y, cStringAt(g.exe, alt[i])); err != nil {
				return err
			}
		}
	}
	if gp.toBitmap != nil {
		for i := range gp.options {
			box.CurrentFrame = min(int(gp.options[i]), box.Frames-1)
			if err := gp.toBitmap.AddPicture(0x1a, 0x29+i*0x11, box); err != nil {
				return err
			}
		}
	}
	return nil
}

// drawLoadSaveGamePopup is _Draw_Load_Save_Game_Popup_ (0x7f206).
func (g *Game) drawLoadSaveGamePopup() error {
	e, gp, s := g.e, &g.gp, &g.state
	t := e.Text()
	page := g.screenData[0]
	bg := gp.anims[animLoadBG]
	if page != pageLoad {
		bg = gp.anims[animSaveBG]
	}
	toBitmap := gp.toBitmap != nil
	switch {
	case g.currentScreen == ScreenMainMenu:
		if err := e.Draw(0, 0, g.mainMenu.first); err != nil {
			return err
		}
		if err := e.Draw(gp.baseX, gp.baseY, bg); err != nil {
			return err
		}
		g.fullDrawScreen = false
	case !toBitmap && g.fullDrawScreen:
		e.V.SetPageOff()
		if err := g.drawMiniMainScreen(); err != nil {
			return err
		}
		e.V.CopyOffToBack()
		e.V.SetRefreshStencil()
		if err := e.Draw(gp.baseX, gp.baseY, bg); err != nil {
			return err
		}
		g.fullDrawScreen = false
	case !toBitmap:
		if err := e.Draw(gp.baseX, gp.baseY, bg); err != nil {
			return err
		}
	}
	bx, by := gp.baseX, gp.baseY
	if toBitmap {
		bx, by = 0, 0
	}
	sel := int(int8(g.settings[0x15]))
	for i := range s.Descriptions {
		off := i * 0x1f
		invalid := gp.status[i] == slotInvalid
		if k := gp.gameTypes[i]; k <= 3 {
			if err := g.drawPic(bx+0xe4, by+off+0x24, gp.anims[animTypeIcons+int(min(k, 2))]); err != nil {
				return err
			}
		}
		x, y := bx+0x20, by+0x18+off
		desc := save.CString(s.Descriptions[i][:])
		if invalid {
			desc = cStringAt(g.exe, invalidSaveString)
		}
		gp.normal, gp.highlight = g.gamePopupColors(2)
		t.SetStyle(2, pick(i == sel, gp.highlight, gp.normal))
		switch {
		case !toBitmap && page == pageLoad:
			if err := t.Print(x, y, desc); err != nil {
				return err
			}
		case toBitmap && page == pageSave && (invalid || s.Descriptions[i][0] != 0):
			if err := g.printText(x+4, y, desc); err != nil {
				return err
			}
		case toBitmap && page != pageSave && s.Descriptions[i][0] != 0:
			if err := g.printText(x, y, desc); err != nil {
				return err
			}
		}
		gp.normal, gp.highlight = g.gamePopupColors(1)
		t.SetStyle(1, pick(i == sel, gp.highlight, gp.normal))
		if page == pageSave {
			x += 2
		}
		if toBitmap || gp.emptySlot != save.CString(s.Descriptions[i][:]) {
			if err := g.printText(x, y+0xe, gp.stardateStr[i]); err != nil {
				return err
			}
		}
		if err := g.printText(x+100, y+0xe, gp.dates[i]); err != nil {
			return err
		}
	}
	return nil
}

func pick[T any](c bool, a, b T) T {
	if c {
		return a
	}
	return b
}

// animateGamePopup is Animate_Game_Popup_ (0x7edf2): page's popup drawn into
// a bitmap of bg slides from y (0 or its height) by dir until it is in or out.
func (g *Game) animateGamePopup(bg *gfx.Picture, y, dir, page int) error {
	e, gp, v := g.e, &g.gp, g.e.V
	w, h := bg.Width, bg.Height
	bm := gfx.NewBlankBitmap(w, h)
	bm.DrawPictureToBitmap(bg)
	gp.toBitmap = bm
	var err error
	switch page {
	case pageMain:
		bx, by := gp.baseX, gp.baseY
		gp.baseX, gp.baseY = 0, 0
		err = g.drawMainGamePopup()
		gp.baseX, gp.baseY = bx, by
	case pageLoad, pageSave:
		err = g.drawLoadSaveGamePopup()
	case pageOptions:
		err = g.drawOptionsGamePopup()
	}
	gp.toBitmap = nil
	if err != nil {
		return err
	}
	if err := g.drawMainMainScreen(); err != nil {
		return err
	}
	v.CopyOffToBack()
	v.SetRefreshStencil()
	for step := 1; ; step++ {
		y += h * step / 100 * dir
		if y > h && dir > 0 {
			y = h
		}
		v.SetPageOff()
		y2 := y + 0x1a
		v.SetWindow(0x90, 0x19, w+0x90, y2)
		v.ClipOn()
		top := 0x15 - h + y
		if step == 0xe && dir > 0 {
			top = 0x19
		}
		if err := e.Draw(0x90, top, bm); err != nil {
			return err
		}
		v.ClipOff()
		end := y2-0x19 > h
		if dir < 1 {
			end = y2-0x19 < 0x1a
		}
		if err := e.TogglePages(); err != nil {
			return err
		}
		if end {
			break
		}
	}
	v.CopyOffToBack()
	return nil
}

// gamePopupUserBox is Game_Popup_User_Box_ (0x7e59a): kind 0 a message box,
// 1 a confirmation, 3 a warning; then Restore_Game_Popup_ (0x7e514).
func (g *Game) gamePopupUserBox(msg string, kind int) (bool, error) {
	e := g.e
	e.V.SetRefreshFull()
	var ok bool
	var err error
	switch kind {
	case 0:
		err = g.messageBox(msg)
	case 1:
		ok, err = g.confirmationBox(msg)
	case 3:
		err = g.warningBox(msg)
	default:
		return false, nil
	}
	if err != nil {
		return false, err
	}
	e.ClearFields()
	if g.currentScreen == ScreenMainMenu {
		e.V.SetPageBack()
		e.V.Fill(0, 0, 0x27f, 0x1df, 0)
		if err := e.Draw(0, 0, g.mainMenu.first); err != nil {
			return false, err
		}
	} else {
		e.V.SetPageOff()
		if err := g.drawMiniMainScreen(); err != nil {
			return false, err
		}
		e.V.CopyOffToBack()
		e.V.SetRefreshStencil()
		g.fullDrawScreen = false
	}
	e.V.SetPageOff()
	if err := e.InvokeAutoFunction(); err != nil {
		return false, err
	}
	return ok, e.TogglePages()
}

// badGameFileMessage is Bad_Game_File_Message_ (0x7d8f3).
func (g *Game) badGameFileMessage(status byte, slot int) error {
	var h int
	switch status {
	case slotMissing:
		h = 0xb2
	case slotInvalid:
		h = 0xb3
	case slotNetwork:
		h = 0xb4
	default:
		// The original shows its uninitialised buffer.
		return fmt.Errorf("Bad_Game_File_Message_ with status %d prints stale stack", status)
	}
	_, err := g.gamePopupUserBox(fmt.Sprintf(cFormat(g.hMessage(h)), slot+1), 3)
	return err
}

// embedSpecialColorCodes is Embed_Special_Color_Codes_ (0x806a2): the string
// between 0x03 and 0x01, when it fits.
func embedSpecialColorCodes(d []byte) {
	n := cLen(d)
	if n >= 0xc9 || n >= len(d)-2 || d[0] == 3 {
		return
	}
	copy(d[1:], d[:n])
	d[0], d[n+1] = 3, 1
}

// removeEmbeddedSpecialCodes is Remove_Embedded_Special_Codes_ (0x80658).
func removeEmbeddedSpecialCodes(d []byte) {
	if d[0] != 3 {
		return
	}
	n := cLen(d)
	copy(d, d[1:n])
	if k := bytes.IndexByte(d[:cLen(d)], 1); k >= 0 {
		d[k] = 0
	}
}

// cLen is strlen within b.
func cLen(b []byte) int {
	if k := bytes.IndexByte(b, 0); k >= 0 {
		return k
	}
	return len(b)
}

// copyCString is strcpy of a C string into a fixed buffer, cut to fit.
func copyCString(d, src []byte) { setCString(d, string(src[:cLen(src)])) }

// setCString is strcpy into a fixed buffer, cut to fit.
func setCString(d []byte, s string) {
	n := copy(d[:len(d)-1], s)
	d[n] = 0
}

func le32(b []byte) uint32 {
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}
