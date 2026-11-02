package game

import (
	"github.com/openorion2/openorion2/internal/engine"
	"github.com/openorion2/openorion2/internal/font"
	"github.com/openorion2/openorion2/internal/gfx"
	"github.com/openorion2/openorion2/internal/save"
)

// The new game screen, MOX\OTHERS\NEWGAME.C, screen 0xd, and the click
// through fields of FAKEBUTS.C with the 1.40 patch's hooks. Spec:
// docs/screens/new-game.md.

const (
	newGameLBX  = 0x182a6e // "NEWGAME.LBX"
	raceOptLBX  = 0x182a87 // "RACEOPT.LBX"
	newGameGX   = 0x1831d4 // _g_x, _g_y: the screen's origin, never changed
	newGameHelp = 0x18348e // _new_game_screen_help_list, 11 entries
	hackMsgFmt1 = 0x179e78 // "Piccola"
	hackMsgFmt2 = 0x179e80 // "%sen"
	hackMsgFmt3 = 0x179e85 // "Media"

	// The 1.40 patch's data (0x1784dd, see patchByte): +1 the extra tech
	// level "Post Warp", +2 the extra galaxy size "Cluster"; their names
	// follow at +0x37 and +3.
	patchPostWarp     = 1
	patchCluster      = 2
	patchClusterName  = 0x1784e0
	patchPostWarpName = 0x178514
)

// clickThruField is one 0x17-byte record of _click_thru_fields (0x1a09fe).
type clickThruField struct {
	x1, y1, x2, y2 int16
	field, linked  int       // +0xc, +0xa: its hidden field and the picture's
	strings        []*string // +0xc: one per value
	n              int16
	v              *int16
	color          byte // +0x16: box colour, 0 none
}

// newGameState holds NEWGAME.C's globals and FAKEBUTS.C's click through
// fields.
type newGameState struct {
	background   *gfx.Picture    // _background_seg, RACEOPT.LBX 0
	galAgeAnims  [3]*gfx.Picture // NEWGAME.LBX 1..3
	diffAnims    [5]*gfx.Picture // 4..8
	galSizeAnims [4]*gfx.Picture // 9..12
	playersAnims [7]*gfx.Picture // 13..19
	techAnims    [3]*gfx.Picture // 20..22
	// _anims: 23 tactical combat, 24 random events, 25 Antarans, 26 cancel,
	// 27 accept, 28 the frame, 29 the network box.
	anims [7]*gfx.Picture

	hackMsg                            string // _hack_msg
	diffStrings                        [5]*string
	galSizeStrings                     [4]*string
	galAgeStrings, civStrings          [3]*string
	oppStrings                         [7]*string
	combatStrings                      [2]*string
	colorKen                           font.Colors // _color_ken
	diff, galSize, galAge, opp, civ    int16       // the button variables
	combat, randomEvents, xenonAttacks int16
	cancelField, escField, acceptField int
	clickThru                          []clickThruField
	helpFirst                          int // the help list's first id, once set
}

// patchByte reads byte +i of the 1.40 patch's data block (0x1784dd); the
// patch's own code toggles +1 and +2 at run time (patchFlags).
func (g *Game) patchByte(i int) byte {
	if g.patchFlags == nil {
		g.patchFlags = map[int]byte{}
	}
	if v, ok := g.patchFlags[i]; ok {
		return v
	}
	return g.exeU8(0x1784dd + uint32(i))
}

func (g *Game) setPatchByte(i int, v byte) {
	g.patchByte(i)
	g.patchFlags[i] = v
}

// loadNewGameStrings is Load_New_Game_Strings_ (0xcc81c).
func (g *Game) loadNewGameStrings() {
	ng := &g.ng
	str := func(i int) *string { s := g.eStr(i); return &s }
	ng.diffStrings = [5]*string{str(0x221), str(0x118), str(0xbe), str(0x13e), str(0x148)}
	lang := g.settings[0xd2]
	ng.galSizeStrings[0] = str(0x1e0)
	if lang == 4 {
		ng.hackMsg = cStringAt(g.exe, hackMsgFmt1)
		ng.galSizeStrings[0] = &ng.hackMsg
	}
	ng.galSizeStrings[1] = str(0x173)
	// French and Italian put the shared _hack_msg in slot 2, which the
	// next string replaces at once (the slot index is not advanced).
	switch lang {
	case 2:
		ng.hackMsg = cSprintf(cStringAt(g.exe, hackMsgFmt2), g.eStr(0x173))
		ng.galSizeStrings[2] = &ng.hackMsg
	case 4:
		ng.hackMsg = cStringAt(g.exe, hackMsgFmt3)
		ng.galSizeStrings[2] = &ng.hackMsg
	}
	ng.galSizeStrings[2] = str(0x168)
	ng.galSizeStrings[3] = str(0x143)
	ng.galAgeStrings = [3]*string{str(0x179), str(0xbe), str(0x192)}
	ng.combatStrings = [2]*string{str(0x1f7), str(0x1f0)}
	ng.oppStrings = [7]*string{str(0x94), str(0x96), str(0x98), str(0x9a), str(0x9c), str(0x9d), str(0x9e)}
	ng.civStrings = [3]*string{str(0x1ad), str(0xbe), str(0xa9)}
}

// reloadNewgameScreen is Reload_Newgame_Screen_ (0xcca1c).
func (g *Game) reloadNewgameScreen() error {
	ng := &g.ng
	var err error
	if ng.background, err = g.picture(cStringAt(g.exe, raceOptLBX), 0); err != nil {
		return err
	}
	name := cStringAt(g.exe, newGameLBX)
	n := 1
	load := func(dst []*gfx.Picture) error {
		for i := range dst {
			if dst[i], err = g.picture(name, n); err != nil {
				return err
			}
			n++
		}
		return nil
	}
	for _, d := range [][]*gfx.Picture{ng.galAgeAnims[:], ng.diffAnims[:], ng.galSizeAnims[:], ng.playersAnims[:], ng.techAnims[:], ng.anims[:]} {
		if err := load(d); err != nil {
			return err
		}
	}
	g.loadNewGameStrings()
	return nil
}

// newGameOrigin is _g_x, _g_y.
func (g *Game) newGameOrigin() (int, int) {
	return int(g.exeI16(newGameGX)), int(g.exeI16(newGameGX + 2))
}

// drawNewGamePictures is Draw_Pictures_ (0xccc3d): the five pictures in
// columns of 0x9b and rows of 0x8c; row 1 has two.
func (g *Game) drawNewGamePictures() error {
	ng := &g.ng
	gx, gy := g.newGameOrigin()
	for col := 0; col < 3; col++ {
		for row := 0; row < 2; row++ {
			if row == 1 && col > 1 {
				continue
			}
			var p *gfx.Picture
			switch {
			case col == 0 && row == 0:
				p = ng.diffAnims[ng.diff]
			case col == 1 && row == 0:
				p = ng.galSizeAnims[ng.galSize]
			case col == 2 && row == 0:
				p = ng.galAgeAnims[ng.galAge]
			case col == 0:
				p = ng.playersAnims[ng.opp]
			default:
				p = ng.techAnims[ng.civ]
			}
			if err := g.e.Draw(int(int16(gx+0x79+col*0x9b)), int(int16(gy+0x77+row*0x8c)), p); err != nil {
				return err
			}
		}
	}
	return nil
}

// drawNewgameScreen is Draw_Newgame_Screen_ (0xccd3c), also the auto
// function.
func (g *Game) drawNewgameScreen() error {
	e, ng := g.e, &g.ng
	t := e.Text()
	gx, gy := g.newGameOrigin()
	t.SetStyleShadow(4, ng.colorKen, font.ShadowDown)
	t.SetOutlineColor(0, 0x40)
	if err := e.Draw(gx, gy, ng.anims[5]); err != nil {
		return err
	}
	t.SetStyle(2, ng.colorKen)
	e.V.SetPageOff()
	if g.col.fullDraw {
		e.V.Fill(0, 0, 0x27f, 0x1df, 0)
		if err := e.Draw(0, 0, ng.background); err != nil {
			return err
		}
		t.SetStyle(3, ng.colorKen)
		e.V.CopyOffToBack()
		g.col.fullDraw = false
	}
	if err := g.drawClickThruFields(); err != nil {
		return err
	}
	if err := g.drawNewGamePictures(); err != nil {
		return err
	}
	if g.gameType == 2 {
		if err := e.Draw(int(int16(gx+0xd9)), int(int16(gy+0x179)), ng.anims[6]); err != nil {
			return err
		}
	}
	t.SetStyle(3, ng.colorKen)
	t.SetAliasColor(0x9a)
	return nil
}

// addNewGameFields is Add_Fields_ (0xcce2e).
func (g *Game) addNewGameFields() error {
	e, ng := g.e, &g.ng
	gx, gy := g.newGameOrigin()
	e.Text().SetStyle(3, ng.colorKen)
	none := g.eStr(0xc)
	at := func(v int) int { return int(int16(v)) }
	type pic struct {
		x1, y1, x2, y2 int // the picture's hidden field
		bx, by         int // the label's field, 0x64 by 0x14
		strings        []*string
		v              *int16
	}
	pics := []pic{
		{0x79, 0x77, 0xbc, 0xb8, 0x69, 0xcc, ng.diffStrings[:], &ng.diff},
		{0x114, 0x77, 0x157, 0xb8, 0x105, 0xcc, ng.galSizeStrings[:], &ng.galSize},
		{0x1af, 0x77, 0x1f2, 0xb8, 0x1a1, 0xcc, ng.galAgeStrings[:], &ng.galAge},
		{0x79, 0x103, 0xbc, 0x144, 0x69, 0x15d, ng.oppStrings[:], &ng.opp},
		{0x114, 0x103, 0x157, 0x144, 0x105, 0x15d, ng.civStrings[:], &ng.civ},
	}
	// key is the hot key: E string 0xc (none), or under Options.NewGameKeys
	// the tests' one.
	key := func(i int) string {
		if !g.newGameKeys {
			return none
		}
		return g.eStr([]int{0x93, 0x95, 0x97, 0x99, 0x9b, 0xa6, 0xc1, 0xd5, 0x8e}[i])
	}
	for i, p := range pics {
		hidden, err := e.AddHiddenField(at(gx+p.x1), at(gy+p.y1), at(gx+p.x2), at(gy+p.y2), key(i), 0)
		if err != nil {
			return err
		}
		if _, err := g.addClickThruField(at(gx+p.bx), at(gy+p.by), at(gx+p.bx+0x64), at(gy+p.by+0x14), p.strings, p.v, 0, none, 0, hidden); err != nil {
			return err
		}
	}
	radios := []struct {
		y   int
		pic *gfx.Picture
		v   *int16
	}{{0x127, ng.anims[1], &ng.randomEvents}, {0x14a, ng.anims[2], &ng.xenonAttacks}, {0x103, ng.anims[0], &ng.combat}}
	for i, r := range radios {
		if _, err := e.AddRadioButtonField(at(gx+0x17c), at(gy+r.y), none, r.pic, r.v, key(5+i), 0x29); err != nil {
			return err
		}
	}
	if g.gameType == 2 {
		// The network game's name, from settings+0x17 (0xcd261).
		n := &g.net
		n.name = [10]byte{}
		copy(n.name[:], g.settings[0x17:0x17+9])
		if _, err := e.AddContinuousStringInputField(at(gx+0xe1), at(gy+0x18a), 0xa8, 0x12, n.name[:], 8, 0, nil, 0, 0); err != nil {
			return err
		}
	}
	var err error
	if ng.cancelField, err = e.AddButtonField(at(gx+100), at(gy+0x182), none, ng.anims[3], none, 0x28); err != nil {
		return err
	}
	if ng.acceptField, err = e.AddButtonField(at(gx+0x1a2), at(gy+0x183), none, ng.anims[4], key(8), 0x28); err != nil {
		return err
	}
	ng.escField, err = e.AddHotKey(g.eStr(0x87))
	return err
}

// setNewGameScreenHelpList is Set_New_Game_Screen_Help_List_ (0xcdb72):
// the first entry explains the tech level shown.
func (g *Game) setNewGameScreenHelpList(civ int16) {
	g.e.DeactivateHelpList()
	list := g.helpListAt(newGameHelp, 0xb)
	switch civ {
	case 0:
		g.ng.helpFirst = 0x226
	case 1:
		g.ng.helpFirst = 0x225
	case 2:
		g.ng.helpFirst = 0x224
	}
	if g.ng.helpFirst != 0 {
		list[0].ID = g.ng.helpFirst
	}
	g.e.SetHelpList(list)
}

// globalsToTemps is Globals_To_Temps_ (0xcd3bc), inlined in Newgame_Screen_.
func (g *Game) globalsToTemps() {
	ng, st := &g.ng, &g.settings
	ng.diff = int16(st[0xd4])
	ng.galSize = int16(st[0xd6])
	ng.galAge = int16(st[0xd7])
	ng.opp = int16(st[0xd5]) - 2
	ng.combat = b2i16(st[0xd8] == 0)
	ng.civ = int16(st[0xd9])
	ng.randomEvents = int16(st[0x02])
	ng.xenonAttacks = int16(st[0xd3])
}

// tempsToGlobals is Temps_To_Globals_ (0xcd361), inlined in Newgame_Screen_.
func (g *Game) tempsToGlobals() {
	ng, st := &g.ng, &g.settings
	st[0xd4] = byte(ng.diff)
	st[0xd6] = byte(ng.galSize)
	st[0xd7] = byte(ng.galAge)
	st[0xd5] = byte(ng.opp) + 2
	st[0xd8] = byte(b2i16(ng.combat == 0))
	st[0xd9] = byte(ng.civ)
	st[0x02] = byte(ng.randomEvents)
	st[0xd3] = byte(ng.xenonAttacks)
}

func b2i16(b bool) int16 {
	if b {
		return 1
	}
	return 0
}

// newgameScreen is Newgame_Screen_ (0xcd435), screen 0xd for single player
// and hotseat games. It returns 1 when a race was picked.
func (g *Game) newgameScreen() (int16, error) {
	e, ng, s := g.e, &g.ng, &g.state
	ng.colorKen = font.Colors{0x79, 0x7b, 0x7a, 0x7c, 0x7c, 0x7c, 0x7c, 0x7c}
	civ := ng.civ
	s.Stars = [maxStars]save.Star{}
	s.Planets = [maxPlanets]save.Planet{}
	s.Colonies = [maxColonies]save.Colony{}
	s.Ships = [maxShips]save.Ship{}
	s.Players = [maxPlayers]save.Player{}
	s.Leaders = [maxLeaders]save.Leader{}
	e.SetMouseList(engine.MouseNone)
	if err := e.FastFadeOut(); err != nil {
		return 0, err
	}
	if err := e.LoadPalette(9, 0, 0xff); err != nil {
		return 0, err
	}
	if g.gameType == 0 || g.gameType == 1 {
		g.rsel.colorFlag = [8]bool{}
		g.rsel.raceFlag = [14]bool{}
	}
	if err := g.reloadNewgameScreen(); err != nil {
		return 0, err
	}
	g.globalsToTemps()
	setup := func() error {
		g.clearClickThruFields()
		e.ClearFields()
		return g.addNewGameFields()
	}
	if err := setup(); err != nil {
		return 0, err
	}
	g.col.fullDraw = true
	e.V.SetRefreshStencil()
	g.setNewGameScreenHelpList(ng.civ)
	e.AssignAutoFunction(g.drawNewgameScreen, 2)
	e.V.SetPageOff()
	if err := g.drawNewgameScreen(); err != nil {
		return 0, err
	}
	if err := e.DarkenPalette(100); err != nil {
		return 0, err
	}
	e.V.ClearPaletteChanges(0, 0xff)
	if err := e.FastFadeIn(); err != nil {
		return 0, err
	}
	e.SetMouseList(engine.MouseArrow)
	result := int16(0)
	for done := false; !done; {
		r, err := e.GetInput()
		if err != nil {
			return 0, err
		}
		g.evaluateClickThruFields(r)
		if r == ng.cancelField || r == ng.escField {
			done = true
			g.currentScreen = g.returnScreen
		}
		if r == ng.acceptField {
			g.tempsToGlobals()
			switch g.gameType {
			case 1, 2, 3:
				g.currentScreen = 0x15
				if g.gameType == 1 {
					g.currentScreen = 0x10
				}
				result, done = 1, true
			default:
				if result, err = g.raceSelectionScreen(&s.Players[0]); err != nil {
					return 0, err
				}
				if result == 1 {
					s.Players[0].Objective = save.ObjectiveHuman
					done = true
					break
				}
				if err := g.reloadNewgameScreen(); err != nil {
					return 0, err
				}
				e.V.SetRefreshStencil()
				g.globalsToTemps()
				g.col.fullDraw = true
				if err := setup(); err != nil {
					return 0, err
				}
				e.AssignAutoFunction(g.drawNewgameScreen, 2)
				g.setNewGameScreenHelpList(ng.civ)
			}
		}
		if civ != ng.civ {
			civ = ng.civ
			g.setNewGameScreenHelpList(civ)
		}
		if !done {
			e.Text().SetStyle(3, ng.colorKen)
			if err := g.drawNewgameScreen(); err != nil {
				return 0, err
			}
			e.Text().SetStyle(3, ng.colorKen)
			if err := e.TogglePages(); err != nil {
				return 0, err
			}
		}
	}
	if result == 1 {
		g.tempsToGlobals()
	}
	e.ClearFields()
	g.clearClickThruFields()
	e.DeactivateHelpList()
	e.DeactivateAutoFunction()
	if g.gameType == 2 || g.gameType == 3 {
		// The game's name back to settings+0x17 (0xcd8b4).
		k := copy(g.settings[0x17:], save.CString(g.net.name[:]))
		g.settings[0x17+k] = 0
	}
	return result, nil
}

// clearClickThruFields is Clear_Click_Thru_Fields_ (0xc6a26).
func (g *Game) clearClickThruFields() { g.ng.clickThru = g.ng.clickThru[:0] }

// addClickThruField is Add_Click_Thru_Field_ (0xc6a43): a hidden field
// over the label that cycles v through the strings, as does linked.
func (g *Game) addClickThruField(x1, y1, x2, y2 int, strings []*string, v *int16, color byte, hotkey string, sound, linked int) (int, error) {
	n, err := g.e.AddHiddenField(x1, y1, x2, y2, hotkey, sound)
	if err != nil {
		return 0, err
	}
	g.ng.clickThru = append(g.ng.clickThru, clickThruField{
		x1: int16(x1), y1: int16(y1), x2: int16(x2), y2: int16(y2), field: n, linked: linked,
		strings: strings, n: int16(len(strings)), v: v, color: color,
	})
	return n, nil
}

// evaluateClickThruFields is Evaluate_Click_Thru_Fields_ (0xc6aa4): the
// first record whose field or linked field is r steps its value. The 1.40
// patch (0x169044) adds "Post Warp" after Average tech and, entered from the
// main menu, "Cluster" after Large; their flags live in the patch's data.
func (g *Game) evaluateClickThruFields(r int) {
	ng := &g.ng
	for i := range ng.clickThru {
		c := &ng.clickThru[i]
		if r != c.field && r != c.linked {
			continue
		}
		v := int32(*c.v)
		switch c.v {
		case &ng.civ:
			if v == 1 {
				b := g.patchByte(patchPostWarp) ^ 1
				g.setPatchByte(patchPostWarp, b)
				if b != 0 {
					v--
				}
			}
		case &ng.galSize:
			if g.previousScreen == ScreenMainMenu && int8(v) == 2 {
				b := g.patchByte(patchCluster) ^ 1
				g.setPatchByte(patchCluster, b)
				if b != 0 {
					v--
				}
			}
		}
		*c.v = int16((v + 1) % int32(c.n))
		return
	}
}

// drawClickThruFields is Draw_Click_Thru_Fields_ (0xc68c4) with the 1.40
// patch's label hook (0x169088).
func (g *Game) drawClickThruFields() error {
	e, ng := g.e, &g.ng
	for i := range ng.clickThru {
		c := &ng.clickThru[i]
		if c.color > 0 {
			e.Box(int(c.x1), int(c.y1), int(int16(c.x2-c.x1)), int(int16(c.y2-c.y1)), 1, c.color)
		}
		e.V.SetWindow(int(c.x1), int(c.y1), int(c.x2), int(c.y2))
		e.V.ClipOn()
		label := ""
		switch {
		case c.v == &ng.civ && g.patchByte(patchPostWarp) == 1:
			label = cStringAt(g.exe, patchPostWarpName)
		case c.v == &ng.galSize:
			if g.previousScreen != ScreenMainMenu {
				g.setPatchByte(patchCluster, 0)
			}
			if g.patchByte(patchCluster) == 1 {
				label = cStringAt(g.exe, patchClusterName)
				break
			}
			label = *c.strings[*c.v]
		default:
			label = *c.strings[*c.v]
		}
		w, h := int16(c.x2-c.x1), int16(c.y2-c.y1-2)
		if err := e.Text().SqueezePrintParagraph(int(c.x1), int(int16(c.y1+2)+h/2), int(w), int(h), label, 2, g.exe.bytes(spaceLimits, 6), true); err != nil {
			return err
		}
		e.V.ClipOff()
	}
	return nil
}

// newGameScreenControl is Screen_Control_'s case 0xd (0x1059e): single
// player runs Newgame_Screen_, and a picked race starts the game through
// the 1.40 patch's wrapper of Init_New_Game_ (0x169410), which leaves the
// reports screen (turn 1) to run next. In multiplayer games Newgame_Screen_
// returns to the multiplayer screen or goes on to the human setup itself.
func (g *Game) newGameScreenControl() error {
	if g.gameType != 0 {
		g.returnScreen = ScreenMultiplayer
		_, err := g.newgameScreen()
		return err
	}
	ok, err := g.newgameScreen()
	if err != nil {
		return err
	}
	if ok == 0 {
		g.currentScreen = ScreenMainMenu
		return nil
	}
	g.newGameStarted = true
	if g.newGameHook != nil {
		g.newGameHook(g)
	}
	return g.initNewGameWrapper()
}
