package game

import (
	"fmt"
	"strings"
	"time"

	"github.com/openorion2/openorion2/internal/engine"
	"github.com/openorion2/openorion2/internal/font"
	"github.com/openorion2/openorion2/internal/gfx"
	"github.com/openorion2/openorion2/internal/save"
)

// The popups over the galaxy map (MAINPUPS.C): the shared setup and
// cleanup, and the officer and discovery popups of the begin-of-turn
// reports. Spec: docs/screens/main-popups.md.

const (
	mainpupsLBX      = "MAINPUPS.LBX"
	normalTextColors = 0x182284 // _normal_text_colors
)

// mainPopups holds MAINPUPS.C's globals.
type mainPopups struct {
	agentSelecting  bool                 // port-only: in New_Colony_Selection_Popup_, for the remote control
	autofunctionCtr int                  // _autofunction_ctr (0x1831c0)
	fieldSaves      []*engine.FieldStats // _stats_array, _num_field_saves
	saved, saved2   engine.AutoFunction  // _eric_saved_autofunction(2)
	fullDraw        bool                 // _full_draw (0x182aca)
	explodeOff      bool                 // !_explode_flag (0x1831c4, 1 in .data)
	popupW, popupH  int                  // port-only: the open popup's size, for tests
	noFlicker       bool                 // _no_flicker_between_mainpups_flag
	refreshMode     int                  // _g_refresh_mode
	x, y            int                  // _g_x, _g_y (0x1a123a, 0x1a123c)
	starN           int16                // _g_star_n
	sysDispStar     int                  // _sys_disp_star_handle
	officer         int                  // _g_officer (0x1831c6)
	levelState      int                  // _officer_gains_level_state (0x1831c8)
	bg, officerPic  *gfx.Picture         // the popup's pictures (0x1a113c, 0x1a1140)
	skillIcons      [27]*gfx.Picture     // _officer_skill_icon_seg
	hiddenField     int                  // _mp_fields (0x1a11c4)
	lastScanned     int                  // _last_scanned_field
	showing         string               // port-only: the popup in its input loop, for tests

	hireMsg            string       // 0x1a11b8, Hire_Officer_Popup_'s message
	onOfficerScreen    bool         // _on_officer_screen_flag
	playerN            int          // _g_player_n
	fieldState         int          // _field_state (0x1a1238)
	rejectPic, hirePic *gfx.Picture // 0x1a1144, 0x1a1148
	reject, hire       int          // the buttons (0x1a11d4, 0x1a11d2)
	helpFields         [5]int       // 0x19c408: the skill lines' help fields
	helpSkills         [5]int       // 0x19c412: their skills
	special            int8         // _g_special (0x1831ae)
	specialFrame       *gfx.Picture // 0x1a1114, MAINPUPS entry 0
	specialAnim        *gfx.Picture // 0x1a1130
	specialDesc        string       // _g_msg_ptr's text
	selectionState     int          // _selection_state_flag (0x1831ca)
	usingColonyShip    bool         // _g_using_colony_ship
	closeField         int          // 0x1a11c8, the system display's close button
}

// ericPushFields is Eric_Push_Fields_ (0xc6e7e).
func (g *Game) ericPushFields() {
	g.mp.fieldSaves = append(g.mp.fieldSaves, g.e.SaveFieldStats())
}

// ericPopFields is the restore Do_Popup_Cleanup_ and the hire popup do.
func (g *Game) ericPopFields() {
	if n := len(g.mp.fieldSaves); n > 0 {
		g.e.RestoreFieldStats(g.mp.fieldSaves[n-1])
		g.mp.fieldSaves = g.mp.fieldSaves[:n-1]
	}
}

// doPopupSetup is Do_Popup_Setup_ (0xc702e): centre the popup bg on the
// screen, draw the screen behind it once more, and on the galaxy screen
// bring the star the popup is about into view (star -1: the home star).
func (g *Game) doPopupSetup(star int, bg *gfx.Picture) error {
	e, mp, s := g.e, &g.mp, &g.state
	if g.currentScreen != 0 {
		mp.explodeOff = true
	}
	mp.refreshMode = e.V.RefreshMode()
	e.V.SetRefreshStencil()
	mp.x = 0x140 - int(int16(bg.Width))/2
	mp.y = 0xf0 - int(int16(bg.Height))/2
	mp.popupW, mp.popupH = int(int16(bg.Width)), int(int16(bg.Height))
	if star == -1 {
		star = 0
		if h := s.Players[s.PlayerNum].HomePlanet; h > 0 {
			star = int(s.Planets[h].Star)
		}
	}
	mp.starN, mp.sysDispStar = int16(star), star
	e.Sys.MouseBuffer()
	e.Sys.MouseBuffer2()
	e.SetInputDelay(3)
	e.SetMouseList(engine.MouseArrow)
	e.V.ResetWindow()
	e.V.ClipOff()
	e.V.CopyOffToBack()
	e.V.SetPageOff()
	explode := !mp.explodeOff
	if explode && g.starOnScreen(star) {
		g.centerMap(s.Stars[star].X, s.Stars[star].Y)
	}
	switch mp.autofunctionCtr {
	case 0:
		mp.saved = e.GetAutoFunction()
		if err := mp.saved.Run(); err != nil {
			return err
		}
		if (!explode || !g.starOnScreen(star)) && !mp.noFlicker {
			if err := e.TogglePages(); err != nil {
				return err
			}
			mp.fullDraw = true
			if err := mp.saved.Run(); err != nil {
				return err
			}
		}
		mp.autofunctionCtr++
	case 1:
		mp.saved2 = e.GetAutoFunction()
		mp.fullDraw = true
		if err := mp.saved.Run(); err != nil {
			return err
		}
		mp.fullDraw = true
		if err := mp.saved2.Run(); err != nil {
			return err
		}
		mp.autofunctionCtr++
	}
	if explode {
		if err := e.DrawVisibleFields(); err != nil {
			return err
		}
		if g.settings[4] != 0 && len(mp.fieldSaves) < 1 {
			g.logOnce("Draw_Exploding_Plasma_Box_ is not ported")
		}
	}
	mp.noFlicker = false
	mp.fullDraw = true
	return nil
}

// doPopupCleanup is Do_Popup_Cleanup_ (0xc7283).
func (g *Game) doPopupCleanup() {
	e, mp := g.e, &g.mp
	mp.popupW = 0
	g.ericPopFields()
	e.DeactivateAutoFunction()
	switch mp.autofunctionCtr {
	case 1:
		e.PutAutoFunction(mp.saved)
		mp.autofunctionCtr--
	case 2:
		e.PutAutoFunction(mp.saved2)
		mp.autofunctionCtr--
	}
	e.V.CopyBackToOff()
	g.fullDrawScreen = true
	g.setRefreshMode(mp.refreshMode)
	g.sd.withoutButton = false
	mp.explodeOff = false
	e.DeactivateHelpList()
	g.ms.needToRestoreHelpList = true
}

// setRefreshMode is Set_Refresh_Mode_ (0xc5b64): 0 full, 2 differential,
// 3 stencil; other values change nothing.
func (g *Game) setRefreshMode(n int) {
	switch n {
	case engine.RefreshFull:
		g.e.V.SetRefreshFull()
	case engine.RefreshDifferential:
		g.e.V.SetRefreshDifferential()
	case engine.RefreshStencil:
		g.e.V.SetRefreshStencil()
	}
}

// mainpupsTestedExecuteAutofunction is Mainpups_Tested_Execute_Autofunction_
// (0xc7f84): redraw the screens behind the popup, without touching back.
// paceTicks waits until n timer ticks (55 ms each) have passed since start
// (e.Ticks()), without touching Mark_Time's mark (port-only: loops the
// original ran as fast as a DOS machine drew them, at about that pace).
func (g *Game) paceTicks(start, n int64) error {
	for g.e.Ticks()-start < n {
		if err := g.e.Sys.Sleep(5 * time.Millisecond); err != nil {
			return err
		}
	}
	return nil
}

func (g *Game) mainpupsTestedExecuteAutofunction() error {
	mp, m := &g.mp, &g.ms
	run := func(a engine.AutoFunction) error {
		g.fullDrawScreen, mp.fullDraw, m.dontDrawToBack = true, true, true
		return a.Run()
	}
	switch mp.autofunctionCtr {
	case 1:
		if err := run(mp.saved); err != nil {
			return err
		}
	case 2:
		mp.autofunctionCtr = 0
		if err := run(mp.saved); err != nil {
			return err
		}
		if err := run(mp.saved2); err != nil {
			return err
		}
		mp.autofunctionCtr = 2
	}
	g.fullDrawScreen, m.dontDrawToBack = false, false
	return nil
}

// officerGainsLevelPopup is Officer_Gains_Level_Popup_ (0xc8209): officer l
// gained a level (or, by levelState, a marooned one was found); loc is the
// star to show, -1 for the home star.
func (g *Game) officerGainsLevelPopup(loc, l int) error {
	e, mp := g.e, &g.mp
	e.Text().SetStyleShadow(1, g.exeColors(popupColors), font.ShadowDown)
	mp.sysDispStar = loc
	mp.officer = l
	var err error
	if mp.bg, err = g.picture(mainpupsLBX, 0x33); err != nil {
		return err
	}
	if mp.officerPic, err = g.loadOfficerPicture(l, 0); err != nil {
		return err
	}
	if err := g.reloadOfficerSkillIcons(); err != nil {
		return err
	}
	if err := g.doPopupSetup(loc, mp.bg); err != nil {
		return err
	}
	e.DeactivateAutoFunction()
	e.AssignAutoFunction(g.drawOfficerGainsLevelPopup, 1)
	g.ericPushFields()
	if err := g.addOfficerGainsLevelFields(); err != nil {
		return err
	}
	g.setOfficerInformationPopupHelpList()
	mp.lastScanned = 0
	e.V.SetPageOff()
	if err := g.drawOfficerGainsLevelPopup(); err != nil {
		return err
	}
	if err := e.TogglePages(); err != nil {
		return err
	}
	mp.showing = "level"
	defer func() { mp.showing = "" }()
	for {
		start := e.Ticks()
		in, err := e.GetInput()
		if err != nil {
			return err
		}
		if _, err := e.ScanInput(); err != nil {
			return err
		}
		if err := g.drawOfficerGainsLevelPopup(); err != nil {
			return err
		}
		if err := e.TogglePages(); err != nil {
			return err
		}
		if in != 0 {
			break
		}
		// port-only: a pass a tick, as the leaders screen under it: the
		// system display's planets turn a frame a pass.
		if err := g.paceTicks(start, 1); err != nil {
			return err
		}
	}
	g.doPopupCleanup()
	return nil
}

// addOfficerGainsLevelFields is Add_Officer_Gains_Level_Fields_ (0xc7f2f,
// the tail of Add_Discovery_Fields_): one hidden field over the screen.
func (g *Game) addOfficerGainsLevelFields() error {
	es, err := g.eStrings()
	if err != nil {
		return err
	}
	g.mp.hiddenField, err = g.e.AddHiddenField(0, 0, 0x27f, 0x1df, es[0xc], 0)
	return err
}

// drawOfficerGainsLevelPopup is Draw_Officer_Gains_Level_Popup_ (0xc8020),
// drawn once into off and back.
func (g *Game) drawOfficerGainsLevelPopup() error {
	e, mp := g.e, &g.mp
	if !mp.fullDraw && !g.fullDrawScreen {
		return nil
	}
	g.fullDrawScreen, mp.fullDraw, g.ms.dontDrawToBack = true, true, true
	if err := g.mainpupsTestedExecuteAutofunction(); err != nil {
		return err
	}
	mp.x = 0x140 - int(int16(mp.bg.Width))/2
	mp.y = 0xf0 - int(int16(mp.bg.Height))/2
	if err := e.Draw(mp.x, mp.y, mp.bg); err != nil {
		return err
	}
	if err := g.colonyOfficerPopupData(mp.x, mp.y+0x35, mp.officer); err != nil {
		return err
	}
	if err := e.Draw(mp.x+0xd, mp.y+0x37, mp.officerPic); err != nil {
		return err
	}
	es, err := g.eStrings()
	if err != nil {
		return err
	}
	var msg string
	switch mp.levelState {
	case 0:
		msg = cSprintf(es[1], g.leaderName(mp.officer, false))
	case 1:
		msg = cSprintf(es[0x248], g.leaderName(mp.officer, false))
	case 2:
		msg = cSprintf(es[0x249], g.leaderName(mp.officer, false))
	}
	if err := g.printParagraphCentered(mp.x+0xe, mp.y+0x95, 0x119, 0x3e, msg); err != nil {
		return err
	}
	if err := g.printParagraphCentered(mp.x+0xb, mp.y+0xc, 0x11d, 0x21, g.leaderName(mp.officer, true)); err != nil {
		return err
	}
	e.V.CopyOffToBack()
	mp.fullDraw = false
	return nil
}

// printParagraphCentered is Print_Paragraph_Centered_ (0xc5f14).
func (g *Game) printParagraphCentered(x, y, maxw, h int, s string) error {
	return g.printParagraphCenteredVertically(x, y, maxw, h, s, 2)
}

// setOfficerInformationPopupHelpList is
// Set_Officer_Information_Popup_Help_List_ (0xcdc2d).
func (g *Game) setOfficerInformationPopupHelpList() {
	g.e.DeactivateHelpList()
	g.e.SetHelpList(g.helpListAt(officerInformationPopupHelp, 1))
}

// loadOfficerPicture is Load_Officer_Picture_ (0x925ae): officer l's
// portrait from _officer_lbx_name, large (mode 0), or the two smaller sets.
func (g *Game) loadOfficerPicture(l, mode int) (*gfx.Picture, error) {
	base := [3]int{0x15, 0xd2, 0x115}[mode]
	return g.picture(cStringAt(g.exe, g.exe.u32(officerLBXName)), base+int(g.state.Leaders[l].Picture))
}

// reloadOfficerSkillIcons is Reload_Officer_Skill_Icons_ (0x96ce5): the 27
// skill icons, entries 0x58.. of _officer_lbx_name.
func (g *Game) reloadOfficerSkillIcons() error {
	file := cStringAt(g.exe, g.exe.u32(officerLBXName))
	for i := range g.mp.skillIcons {
		p, err := g.picture(file, 0x58+i)
		if err != nil {
			return err
		}
		g.mp.skillIcons[i] = p
	}
	return nil
}

// leaderName is Leader_Name_ (0x97588): the rank and name of leader l and,
// with title, his title ("…, the <title>").
func (g *Game) leaderName(l int, title bool) string {
	s := &g.state
	ld := &s.Leaders[l]
	var lvl int16
	if ld.Owner == -1 {
		lvl = min(officerBaseLevel(ld.Experience, false), 5)
	} else {
		lvl = s.ownedOfficerLevel(l)
	}
	es, err := g.eStrings()
	if err != nil {
		return ""
	}
	base := 0x308
	if ld.Type != 0 {
		base = 0x30e
	}
	out := es[base+int(lvl)] + " " + save.CString(ld.Name[:])
	if title && ld.Title[0] != 0 {
		switch g.settings[0xd2] {
		case 4:
			g.logOnce("Get_Italian_The_ is not ported")
		case 2:
			if isVowel(ld.Title[0]) {
				out += g.lbxMessage(0x186)
				break
			}
			fallthrough
		default:
			switch l {
			case 10, 0x16, 0x22, 0x25, 0x27, 0x3c:
				out += g.lbxMessage(0x182)
			default:
				out += g.lbxMessage(0x110)
			}
		}
		out += save.CString(ld.Title[:])
	}
	return out
}

// isVowel is Is_Vowel_ for the French article.
func isVowel(c byte) bool {
	switch c {
	case 'A', 'E', 'I', 'O', 'U', 'a', 'e', 'i', 'o', 'u':
		return true
	}
	return false
}

// colonyOfficerPopupData is Wrapper_For_Colony_Officer_Popup_Data_
// (0x972ee): Print_Officer_Data_ for a popup's officer panel at (x, y).
func (g *Game) colonyOfficerPopupData(x, y, l int) error {
	g.offscreenAskForHirePopup = true
	defer func() { g.offscreenAskForHirePopup = false }()
	return g.printOfficerData(-1, l, x+0x5f, y+5, x+0xa, y+2, 0x50, 0x5a, 0xc3)
}

// officerExperience is community patch 1.51's leader experience (0x40e6a6):
// "XP/next level's XP" in the crew experience text of KENTEXT 0x1f.
func (g *Game) officerExperience(l int) string {
	ld := &g.state.Leaders[l]
	next := 1000
	if lv := g.state.ownedOfficerLevel(l); lv < 4 {
		next = [4]int{60, 150, 300, 500}[lv]
	}
	f := "%d"
	if s, err := g.kenText(0x1f); err == nil {
		// "Green Crew (%d EP)": the part in brackets, as "%d EP" or "Esp. %d".
		i, j := strings.LastIndexByte(s, '('), strings.LastIndexByte(s, ')')
		if i >= 0 && j > i && strings.Count(s[i+1:j], "%d") == 1 {
			f = s[i+1 : j]
		}
	}
	f = strings.Replace(f, "%d", "%d/%d", 1)
	return cSprintf(f, int(uint16(ld.Experience)), next)
}

// printOfficerData is Print_Officer_Data_ (0x95baa). On the leaders screen
// (not in its hire popup) it adds the name, the cost or upkeep and the
// status line under the portrait. Then the ETA over the portrait (p5..p7
// its box), the special skills and the general ones, one line each from
// (x, y): icon, name, and the bonus right-aligned near x+w.
func (g *Game) printOfficerData(row, l, x, y, px, py, pw, ph, w int) error {
	e, s := g.e, &g.state
	t := e.Text()
	ld := &s.Leaders[l]
	status, typ, loc := ld.Status, ld.Type, ld.Location
	cost, maint := g.officerCosts(l)
	t.SetOutlineColor(0, 0)
	colors := g.exeColors(normalTextColors)
	ids := &g.flt.officerIDs
	if g.currentScreen == 0x1d && row > -1 && (int16(g.off.scanned) == ids[row] || int16(g.off.selected) == ids[row]) {
		colors = g.exeColors(offBrightTextColors)
	}
	full := g.currentScreen == 0x1d && !g.offscreenAskForHirePopup
	if full {
		name := g.leaderName(l, false)
		t.SetStyleShadow(3, colors, font.Outline)
		t.SetSpacing(1)
		if err := t.Print((0xa0-t.Width([]byte(name)))/2+0x16+x, y, name); err != nil {
			return err
		}
		colors[0] = 0x3b
		t.SetStyle(0, colors)
		unit := g.monetaryUnit()
		var label string
		var lx int
		if status == 4 {
			label = g.hMessage(0x108)
			lw := t.Width([]byte(label))
			rx := x + w - 2
			if err := t.PrintRight(rx, y, fmt.Sprintf(cFormat(cStringAt(g.exe, moneyFormat)), int(cost), unit)); err != nil {
				return err
			}
			lx = rx - lw
		} else {
			label = g.hMessage(0x109)
			lw := t.Width([]byte(label))
			v := g.hMessage(0x10a)
			if maint != 0 {
				v = fmt.Sprintf(cFormat(cStringAt(g.exe, moneyFormat)), int(maint), unit)
			}
			lx = x + w - 2 - lw
			if err := t.Print(lx, y, v); err != nil {
				return err
			}
		}
		if err := t.Print(lx, t.Height()+y, label); err != nil {
			return err
		}
		if err := t.PrintRight(293, 91+t.Height()+y, g.officerExperience(l)); err != nil {
			return err
		}
		colors[0] = 0x3d
	}
	var line string
	switch status {
	case 0:
		t.SetStyle(2, colors)
		line = g.hMessage(0x10b)
	case 2:
		t.SetStyle(2, colors)
		line = g.hMessage(0x10c)
	case 1:
		t.SetStyle(2, colors)
		switch {
		case typ == 0 && loc > -1:
			line = save.CString(s.Ships[loc].Design.Name[:])
		case typ == 1 && loc > -1:
			line = save.CString(s.Stars[loc].Name[:])
		}
	case 4:
		if g.currentScreen == 0x1d {
			line = fmt.Sprintf(cFormat(g.hMessage(0x10d)), 0x1e-int(ld.ETA))
		} else {
			line = g.hMessage(0x10e)
		}
		t.SetStyle(2, g.exeColors(offRedColors))
	}
	if full {
		g.setFittedFontStyle(line, t.Current, t.Style, t.Spacing, font.Outline, 0x4a, 1)
		d := 0xc - (t.Height()+0xc)/2
		if d >= 0 {
			d = 0
		}
		if err := t.PrintCentered(pw/2+px, py+ph+1+d, line); err != nil {
			return err
		}
	}
	if ld.ETA > 0 && status == 1 {
		if err := g.printETAOnOfficerPicture(l, px, py, pw); err != nil {
			return err
		}
	}
	t.SetStyleShadow(3, colors, font.Outline)
	t.SetSpacing(1)
	if full {
		y += 0x10
	}
	right := x + w + 3
	if g.currentScreen == 0x1d {
		right = x + w - 3
	}
	skillLine := func(skill int) error {
		row := uint32(tblSkillData + skill*tblSkillDataRow)
		lx := x + 2
		if err := e.Draw(lx, y-4, g.mp.skillIcons[skill/2]); err != nil {
			return err
		}
		lx += 0x16
		name, err := g.eStringAt(row + 0xa)
		if err != nil {
			return err
		}
		if err := t.Print(lx, y, name); err != nil {
			return err
		}
		bonus := int16(g.officerSkillBonus(l, skill))
		txt := cSprintf(cStringAt(g.exe, g.exe.u32(row+0xe)), bonus)
		if err := t.PrintRight(right, y, txt); err != nil {
			return err
		}
		t.SetSpacing(1)
		y += 0x11
		return nil
	}
	for skill := 0; skill < 0x36; skill++ {
		kind := g.exeU8(uint32(tblSkillData + skill*tblSkillDataRow + 5))
		if (typ == 0 && kind == 1 || typ == 1 && kind == 2) && g.leaderHasSpecialSkill(l, skill) {
			if err := skillLine(skill); err != nil {
				return err
			}
		}
	}
	for skill := 0; skill < 0x36; skill++ {
		kind := g.exeU8(uint32(tblSkillData + skill*tblSkillDataRow + 5))
		if kind == 0 && g.leaderHasSkill(l, skill) {
			if err := skillLine(skill); err != nil {
				return err
			}
		}
	}
	return nil
}

// monetaryUnit is Get_Monetary_Unit_String_ (0x7aba6).
func (g *Game) monetaryUnit() string {
	if g.settings[0xd2] == 1 {
		return cStringAt(g.exe, 0x17952f)
	}
	return cStringAt(g.exe, 0x17952c)
}

// printETAOnOfficerPicture is Print_ETA_On_Officer_Picture_ (0x956e2): the
// turns until the leader arrives, between two red bars, across the
// portrait.
func (g *Game) printETAOnOfficerPicture(l, x, y, w int) error {
	t, v := g.e.Text(), g.e.V
	eta := int(g.state.Leaders[l].ETA)
	t.SetStyleShadow(2, g.exeColors(offRedColors), font.OutlineHeavy)
	t.SetOutlineColor(0, 0)
	h := t.Height()
	yy := y + 0x2d - h/2
	msg := fmt.Sprintf(cFormat(g.hMessage(0x12f)), eta)
	tw := t.Width([]byte(msg))
	X := x + w/2 - tw/2
	if err := t.Print(X, yy, msg); err != nil {
		return err
	}
	X2 := X + tw
	v.Line(X, yy-4, X2, yy-4, 0x81)
	v.Line(X, yy-3, X2, yy-3, 0x4b)
	v.Line(X, yy+h, X2, yy+h, 0x4b)
	v.Line(X, yy+h+1, X2, yy+h+1, 0x81)
	return nil
}

// Hire popup states (_field_state, 0x1a1238).
const (
	hireAsk      = 0 // the offer, Hire and Reject
	hireRejected = 1
	hirePoor     = 2 // not enough BC
	hireHired    = 3
)

// randomNewOfficerPopup is Random_New_Officer_Popup_ (0xc7ada): an officer
// offers his services to p; whether he was hired. The popup shows over
// the star of p's capitol (the original takes the colony index as a planet
// index there).
func (g *Game) randomNewOfficerPopup(p, l int) (bool, error) {
	e, mp, s := g.e, &g.mp, &g.state
	c := g.playerCapitolWorld(p)
	star := int(s.Planets[c].Star)
	if !mp.onOfficerScreen {
		mp.officer = int(s.Players[p].OfficerForHire)
	}
	officer := mp.officer
	mp.playerN = p
	mp.fieldState = hireAsk
	if s.Players[p].BC < int32(g.calculateOfficerCost(p, mp.officer, false)) {
		mp.fieldState = hirePoor
	}
	mp.sysDispStar = star
	e.Text().SetStyleShadow(1, g.exeColors(popupColors), font.ShadowDown)
	mp.bg = nil
	if err := g.loadNewOfficerPopup(); err != nil {
		return false, err
	}
	if err := g.doPopupSetup(star, mp.bg); err != nil {
		return false, err
	}
	e.DeactivateAutoFunction()
	e.AssignAutoFunction(g.drawRandomNewOfficerPopup, 1)
	g.ericPushFields()
	if err := g.addRandomNewOfficerFields(); err != nil {
		return false, err
	}
	mp.lastScanned = mp.reject
	e.V.SetRefreshStencil()
	e.V.SetPageOff()
	if err := g.drawRandomNewOfficerPopup(); err != nil {
		return false, err
	}
	if err := e.TogglePages(); err != nil {
		return false, err
	}
	hired, leave := false, false
	mp.showing = "hire"
	defer func() { mp.showing = "" }()
	for {
		start := e.Ticks()
		in, err := e.GetInput()
		if err != nil {
			return false, err
		}
		if _, err := e.ScanInput(); err != nil {
			return false, err
		}
		if err := g.officerPopupHelpFields(in, officer); err != nil {
			return false, err
		}
		if mp.fieldState == hireAsk {
			switch {
			case in == mp.hire:
				if s.Players[mp.playerN].BC < int32(g.calculateOfficerCost(int(s.PlayerNum), mp.officer, false)) {
					mp.fieldState, hired = hirePoor, false
				} else {
					mp.fieldState, hired = hireHired, true
				}
				g.ericPopFields()
				g.ericPushFields()
				if err := g.addRandomNewOfficerFields(); err != nil {
					return false, err
				}
				if g.settings[0xd4] < 2 && !mp.onOfficerScreen {
					mp.fullDraw = true
				} else {
					leave = true
				}
				e.Sys.MouseBuffer()
				e.Sys.MouseBuffer2()
				e.SetInputDelay(3)
			case in == mp.reject:
				mp.fieldState, hired = hireRejected, false
				g.ericPopFields()
				g.ericPushFields()
				if err := g.addRandomNewOfficerFields(); err != nil {
					return false, err
				}
				if g.settings[0xd4] > 1 || mp.onOfficerScreen {
					mp.fieldState = hireAsk
					leave = true
				} else {
					mp.fullDraw = true
				}
			case in == -mp.reject || in == -mp.hire:
				if err := g.officerButtonHelp(in == -mp.hire); err != nil {
					return false, err
				}
			}
		} else if in > 0 {
			leave = true
		}
		if err := g.drawRandomNewOfficerPopup(); err != nil {
			return false, err
		}
		if err := e.TogglePages(); err != nil {
			return false, err
		}
		if leave {
			mp.playerN = -1
			g.doPopupCleanup()
			return hired, nil
		}
		// port-only: a pass a tick, as the leaders screen under it: the
		// system display's planets turn a frame a pass.
		if err := g.paceTicks(start, 1); err != nil {
			return false, err
		}
	}
}

// officerInformationPopupHelp is _officer_information_popup_help_list: one
// entry, help 0x234 over the whole screen.
const officerInformationPopupHelp = 0x183538

// officerLevelNames is _officer_level_names: E string slots by rank.
const officerLevelNames = 0x190430

// officerButtonHelp is the right click on the hire popup's buttons
// (0xc7dab..0xc7e67): a text box naming the officer and his price, then the
// popup's fields again and a full redraw.
func (g *Game) officerButtonHelp(hire bool) error {
	e, s, mp := g.e, &g.state, &g.mp
	es, err := g.eStrings()
	if err != nil {
		return err
	}
	ld := &s.Leaders[mp.officer]
	rank, err := g.eStringAt(officerLevelNames + 4*uint32(ld.RankName))
	if err != nil {
		return err
	}
	cost := g.calculateOfficerCost(mp.playerN, mp.officer, false)
	fmtN, titleN := 0x1fb, 0x1bf
	if hire {
		fmtN, titleN = 0x1fa, 0x141
	}
	msg := cSprintf(es[fmtN], rank, save.CString(ld.Name[:]), int(int16(cost)))
	title := es[titleN]
	if err := g.textBox(&title, msg); err != nil {
		return err
	}
	e.DeactivateAutoFunction()
	e.AssignAutoFunction(g.drawRandomNewOfficerPopup, 1)
	g.ericPopFields()
	g.ericPushFields()
	if err := g.addRandomNewOfficerFields(); err != nil {
		return err
	}
	mp.fullDraw = true
	return nil
}

// playerCapitolWorld is Player_Capitol_World_ (0xc5fb0): p's first colony
// with a capitol, else a random one of p's colonies.
func (g *Game) playerCapitolWorld(p int) int {
	s := &g.state
	for i := 0; i < int(s.NumColonies); i++ {
		if c := &s.Colonies[i]; c.Buildings[bldCapitol] != 0 && int16(c.Owner) == int16(p) {
			return i
		}
	}
	return g.pickRandomPlayerColony(p)
}

// pickRandomPlayerColony is Pick_Random_Colony_ (0xc5f5c): one of p's
// colonies or outposts, each equally likely (a draw per colony), -1 for none.
func (g *Game) pickRandomPlayerColony(p int) int {
	s := &g.state
	res, n := -1, uint32(0)
	for i := 0; i < int(s.NumColonies); i++ {
		if int16(s.Colonies[i].Owner) != int16(p) {
			continue
		}
		n++
		if s.Random.Random(n) == 1 {
			res = i
		}
	}
	return res
}

// loadNewOfficerPopup is Load_New_Officer_Popup_ (0xc7965), on the officer
// screen ..._For_Officer_Screen_ (0xc79f0: entries 0x39.., no icon reload).
func (g *Game) loadNewOfficerPopup() error {
	mp := &g.mp
	base := 0x30
	if mp.onOfficerScreen {
		base = 0x39
	}
	var err error
	if mp.bg, err = g.picture(mainpupsLBX, base); err != nil {
		return err
	}
	if mp.rejectPic, err = g.picture(mainpupsLBX, base+1); err != nil {
		return err
	}
	if mp.hirePic, err = g.picture(mainpupsLBX, base+2); err != nil {
		return err
	}
	if mp.officerPic, err = g.loadOfficerPicture(mp.officer, 0); err != nil {
		return err
	}
	if mp.onOfficerScreen {
		return nil
	}
	return g.reloadOfficerSkillIcons()
}

// addRandomNewOfficerFields is Add_Random_New_Officer_Fields_ (0xc754a): the
// left button (reject, later OK) and, while asking or after hiring, the
// right one; then the skills' help fields.
func (g *Game) addRandomNewOfficerFields() error {
	e, mp := g.e, &g.mp
	es, err := g.eStrings()
	if err != nil {
		return err
	}
	e.Sys.MouseBuffer()
	e.Sys.MouseBuffer2()
	e.SetInputDelay(3)
	left, right := 0x1b4, 0x13c
	if mp.fieldState != hireAsk {
		left, right = 0x180, 0x231
	}
	if mp.reject, err = e.AddButtonField(mp.x+0x2c, mp.y+0xe2, es[0xc], mp.rejectPic, es[left], 0x28); err != nil {
		return err
	}
	mp.hire = engine.NoField
	if mp.fieldState == hireAsk || mp.fieldState == hireHired {
		if mp.hire, err = e.AddButtonField(mp.x+0xaf, mp.y+0xe2, es[0xc], mp.hirePic, es[right], 0x28); err != nil {
			return err
		}
	}
	mp.hiddenField = engine.NoField
	if mp.fieldState != hireAsk {
		if mp.hiddenField, err = e.AddHiddenField(0, 0, 0x27f, 0x1df, es[0xc], 0); err != nil {
			return err
		}
	}
	return g.addSkillDescriptionHelpFields(mp.x+0x61, mp.y+0x35+1, mp.officer, &mp.helpFields, &mp.helpSkills)
}

// addSkillDescriptionHelpFields is Add_Skill_Description_Help_Fields_
// (0x92a92, through Wrapper_To_Add_Officer_Popup_Help_Fields_): a hidden
// field over each of the first five skill lines, special skills first.
func (g *Game) addSkillDescriptionHelpFields(x, y, l int, fields, skills *[5]int) error {
	e, s := g.e, &g.state
	*fields, *skills = [5]int{}, [5]int{}
	n := 0
	add := func(y1, y2, skill int) error {
		f, err := e.AddHiddenField(x, y1, x+0xbe, y2, "", 0x29)
		if err != nil {
			return err
		}
		if n < 5 {
			fields[n], skills[n] = f, skill
		}
		n++
		return nil
	}
	y1, y2 := y-1, y+0xf
	for skill := 0; skill < 0x36; skill++ {
		kind := g.exeU8(uint32(tblSkillData + skill*tblSkillDataRow + 5))
		ld := &s.Leaders[l]
		if (ld.Type == 0 && kind == 1 || ld.Type == 1 && kind == 2) && g.leaderHasSpecialSkill(l, skill) {
			if err := add(y1, y2, skill); err != nil {
				return err
			}
			y1, y2 = y1+0x11, y2+0x11
		}
	}
	gy2 := n*0x11 + 0xf + y
	for skill := 0; skill < 0x36; skill++ {
		kind := g.exeU8(uint32(tblSkillData + skill*tblSkillDataRow + 5))
		if kind == 0 && g.leaderHasSkill(l, skill) {
			if err := add(n*0x11+y, gy2, skill); err != nil {
				return err
			}
			gy2 += 0x11
		}
	}
	for ; n < 5; n++ {
		fields[n], skills[n] = engine.NoField, 0
	}
	return nil
}

// officerPopupHelpFields is Wrapper_For_Officer_Popup_Help_Fields_ (0x9733e):
// a right click on a skill line explains the skill.
func (g *Game) officerPopupHelpFields(in, l int) error {
	for i, f := range g.mp.helpFields {
		if in == -f && f != engine.NoField {
			skill := g.mp.helpSkills[i]
			if err := g.loadOfficerSkillNameDescriptions(); err != nil {
				return err
			}
			return g.printOfficerSkillHelp(skill, int16(g.officerSkillBonus(l, skill)), l)
		}
	}
	return nil
}

// drawRandomNewOfficerPopup is Draw_Random_New_Officer_Popup_ (0xc76fa);
// unlike the level popup it redraws on every pass once full drawing is on.
func (g *Game) drawRandomNewOfficerPopup() error {
	e, mp, s := g.e, &g.mp, &g.state
	if !mp.fullDraw && !g.fullDrawScreen {
		return nil
	}
	g.ms.drawToBack = true
	if err := g.mainpupsTestedExecuteAutofunction(); err != nil {
		return err
	}
	if err := e.Draw(mp.x, mp.y, mp.bg); err != nil {
		return err
	}
	if err := g.colonyOfficerPopupData(mp.x, mp.y+0x35, mp.officer); err != nil {
		return err
	}
	if err := e.Draw(mp.x+0xd, mp.y+0x37, mp.officerPic); err != nil {
		return err
	}
	if mp.onOfficerScreen {
		if err := g.printParagraphCentered(mp.x+0xe, mp.y+0x95, 0x119, 0x3e, mp.hireMsg); err != nil {
			return err
		}
	} else {
		es, err := g.eStrings()
		if err != nil {
			return err
		}
		name := g.leaderName(mp.officer, false)
		cost := int(int16(g.calculateOfficerCost(mp.playerN, mp.officer, false)))
		var msg string
		switch mp.fieldState {
		case hireAsk:
			tech, err := g.leaderTechnologyString(mp.playerN, mp.officer)
			if err != nil {
				return err
			}
			msg = cSprintf(es[2], name, cost, int(int16(g.officerMaintenance(mp.playerN, mp.officer))), tech)
		case hireRejected:
			msg = cSprintf(es[6], name, cost)
		case hirePoor:
			msg = cSprintf(es[5], name, cost)
		case hireHired:
			msg = cSprintf(es[4], name, cost)
		}
		t := e.Text()
		if err := t.SqueezePrintParagraph(mp.x+0xe, mp.y+0x95+0x3e/2, 0x119, 0x3e, msg, 2,
			g.exe.bytes(spaceLimits, 6), true); err != nil {
			return err
		}
	}
	if err := g.printParagraphCentered(mp.x+0xb, mp.y+0xc, 0x11d, 0x21, g.leaderName(mp.officer, true)); err != nil {
		return err
	}
	e.V.CopyOffToBack()
	_ = s
	return nil
}

// spaceLimits is _space_limits (0x183684), the narrowest space by style.
const spaceLimits = 0x183684

// leaderTechnologyString is Leader_Technology_String_ (0xc760e): the
// techniques officer l knows, and for leader 0x2f the government.
func (g *Game) leaderTechnologyString(p, l int) (string, error) {
	s := &g.state
	es, err := g.eStrings()
	if err != nil {
		return "", err
	}
	out := ""
	for i := 0; i < 3; i++ {
		t := int(s.Leaders[l].Techs[i])
		if t == 0 {
			continue
		}
		if i == 0 {
			out += es[0xd0]
		} else {
			out += cStringAt(g.exe, 0x179e1c)
		}
		name, err := g.techAppName(t)
		if err != nil {
			return "", err
		}
		out += cSprintf(es[7], name)
	}
	if l == 0x2f {
		if gov := int(s.Players[p].Picks[save.PickGovernment]); gov%2 == 0 {
			label, err := g.eStringAt(governmentLabels + 4*uint32(gov+1))
			if err != nil {
				return "", err
			}
			out += cSprintf(es[0xcf], label)
		}
	}
	if out == "" {
		out = es[0xc]
	}
	return out, nil
}

// governmentLabels is _government_labels (0x192228), E string pointers.
const governmentLabels = 0x192228

// newSystemDiscoveryPopup is New_System_Discovery_Popup_ (0xc8556): the
// system display of a newly explored star, with its special over it until
// the first click; a marooned officer (special 8) is found at once.
func (g *Game) newSystemDiscoveryPopup(star int, special int8) error {
	e, mp := g.e, &g.mp
	mp.sysDispStar = star
	e.Text().SetStyleShadow(1, g.exeColors(popupColors), font.ShadowDown)
	mp.specialFrame, mp.specialAnim = nil, nil
	mp.specialDesc = ""
	mp.special = special
	bg, err := g.cached(buffer0LBX, 0x49)
	if err != nil {
		return err
	}
	if err := g.doPopupSetup(star, bg); err != nil {
		return err
	}
	e.V.CopyOffToBack()
	e.DeactivateAutoFunction()
	e.AssignAutoFunction(g.drawDiscoveryPopup, 1)
	e.DeactivateHelpList()
	g.ericPushFields()
	g.clearSysDispFields()
	if err := g.addDiscoveryFields(); err != nil {
		return err
	}
	if err := g.drawDiscoveryPopup(); err != nil {
		return err
	}
	if err := e.TogglePages(); err != nil {
		return err
	}
	mp.showing = "discovery"
	defer func() { mp.showing = "" }()
	for {
		if err := e.ReleaseTime(2); err != nil {
			return err
		}
		e.MarkTime()
		in, err := e.GetInput()
		if err != nil {
			return err
		}
		sc, err := e.ScanInput()
		if err != nil {
			return err
		}
		gx, gy := mp.x, mp.y
		switch {
		case mp.special == 0:
			g.evaluateDiscoveryInput(in, sc)
		case mp.special == 8:
			mp.special = 0
			var who int16
			mp.levelState = 2
			if g.maroonedHeroAtStarCanJoin(star, &who, int(g.state.PlayerNum)) {
				mp.levelState = 1
			}
			if err := g.officerGainsLevelPopup(star, int(who)); err != nil {
				return err
			}
			mp.levelState = 0
			in = 0
			mp.fullDraw = true
			mp.x, mp.y = gx, gy
			e.DeactivateAutoFunction()
			e.AssignAutoFunction(g.drawDiscoveryPopup, 1)
		case in != 0:
			mp.special = 0
			in = 0
			mp.fullDraw = true
		}
		if mp.special != 0 && g.fullDrawScreen {
			mp.fullDraw = true
		}
		if err := g.drawDiscoveryPopup(); err != nil {
			return err
		}
		if err := e.TogglePages(); err != nil {
			return err
		}
		if int16(in) >= 1 && mp.special == 0 {
			break
		}
	}
	g.doPopupCleanup()
	return nil
}

// clearSysDispFields is Clear_Sys_Disp_Fields_ (0xbf500): COLONY.C's
// _sys_disp_planet_fields and _sys_disp_fields. The ported popups keep
// their system display fields in g.sd, which are cleared too.
func (g *Game) clearSysDispFields() {
	for i := range g.cscr.sysPlanetF {
		g.cscr.sysPlanetF[i] = engine.NoField
	}
	for i := range g.cscr.sysDispF {
		g.cscr.sysDispF[i] = engine.NoField
	}
	for i := range g.sd.planetButtons {
		g.sd.planetButtons[i] = engine.NoField
	}
	for i := 0; i < 3; i++ {
		g.sd.gateFields[i] = engine.NoField
	}
}

// addDiscoveryFields is Add_Discovery_Fields_ (0xc836e): the system
// display's fields (Wrapper_For_Informational_System_Fields_, no close
// button) and a hidden field over the screen.
func (g *Game) addDiscoveryFields() error {
	mp := &g.mp
	g.sd.withoutButton = true
	if err := g.initSystemDisplay(mp.sysDispStar); err != nil {
		return err
	}
	var drag, whole int
	if err := g.addSystemDisplayPopupFields(mp.sysDispStar, mp.x, mp.y, 0, 0, &drag, nil, &whole); err != nil {
		return err
	}
	return g.addOfficerGainsLevelFields()
}

// evaluateDiscoveryInput is Evaluate_Discovery_Input_ (0xc8508): hovering
// over the system display's planets and ships.
func (g *Game) evaluateDiscoveryInput(in, sc int) {
	_, _, _, hovS, _, icon := g.scanSystemDisplayFields(sc, in)
	g.sd.shipScanned = int16(hovS)
	if icon > -1 {
		g.sd.iconScanned = int16(icon)
	}
}

// drawDiscoveryPopup is Draw_Discovery_Popup_ (0xc83c5).
func (g *Game) drawDiscoveryPopup() error {
	e, mp, s := g.e, &g.mp, &g.state
	e.V.SetPageOff()
	if mp.fullDraw || g.fullDrawScreen {
		if err := g.mainpupsTestedExecuteAutofunction(); err != nil {
			return err
		}
		e.V.CopyOffToBack()
		mp.fullDraw, g.fullDrawScreen = false, false
	}
	es, err := g.eStrings()
	if err != nil {
		return err
	}
	title := cSprintf(es[0x1c8], save.CString(s.Stars[mp.sysDispStar].Name[:]))
	bg, err := g.cached(buffer0LBX, 0x49)
	if err != nil {
		return err
	}
	mp.x = 0x140 - int(int16(bg.Width))/2
	mp.y = 0xf0 - int(int16(bg.Height))/2
	if err := g.drawSystemDisplayPopup(mp.sysDispStar, true, title, mp.x, mp.y, 0, 0); err != nil {
		return err
	}
	if mp.special == 0 || mp.special == 8 {
		return nil
	}
	desc, err := g.systemSpecialDesc(mp.special)
	if err != nil {
		return err
	}
	anim, err := g.systemSpecialAnim(mp.sysDispStar, mp.special)
	if err != nil {
		return err
	}
	name, err := g.eStringAt(planetSpecialNames + 4*uint32(mp.special))
	if err != nil {
		return err
	}
	return g.drawSpecialInfoPopup(mp.x+6, mp.y+0x4c, anim, name, desc)
}

// Data the discovery popup reads.
const (
	popupColors        = 0xc6e2e // the popups' text shades
	specialInfoColors  = 0xc6e1e // Draw_Special_Info_Popup_'s
	planetSpecialNames = 0x18f9b8
)

// exeColors reads eight shades from the executable.
func (g *Game) exeColors(addr uint32) font.Colors {
	var c font.Colors
	copy(c[:], g.exe.bytes(addr, 8))
	return c
}

// systemSpecialDesc is System_Special_Desc_ (0xc6fb8): record 0 of entry
// special in the file E string 0x170 names, kept once loaded.
func (g *Game) systemSpecialDesc(special int8) (string, error) {
	mp := &g.mp
	if mp.specialDesc == "" {
		es, err := g.eStrings()
		if err != nil {
			return "", err
		}
		a, err := g.archive(es[0x170])
		if err != nil {
			return "", err
		}
		t, err := a.Table(int(special))
		if err != nil {
			return "", err
		}
		r := t.Record(0)
		mp.specialDesc = save.CString(r[:min(len(r), 0x258)])
	}
	return mp.specialDesc, nil
}

// systemSpecialAnim is System_Special_Anim_ (0xc6f51): the special's
// picture (MAINPUPS entry special+4; monsters by star) and the frame.
func (g *Game) systemSpecialAnim(star int, special int8) (*gfx.Picture, error) {
	mp := &g.mp
	var err error
	if mp.specialAnim == nil {
		entry := int(special) + 4
		if special == 9 {
			entry = starToMonster(star) + 0xf
		}
		if mp.specialAnim, err = g.picture(mainpupsLBX, entry); err != nil {
			return nil, err
		}
	}
	if mp.specialFrame == nil {
		if mp.specialFrame, err = g.picture(mainpupsLBX, 0); err != nil {
			return nil, err
		}
	}
	return mp.specialAnim, nil
}

// drawSpecialInfoPopup is Draw_Special_Info_Popup_ (0xc7318): the special's
// picture in a frame, its name and description.
func (g *Game) drawSpecialInfoPopup(x, y int, anim *gfx.Picture, name, desc string) error {
	e, mp := g.e, &g.mp
	f := mp.specialFrame
	e.V.Fill(x+3, y+3, x+f.Width-3, y+f.Height-3, 0)
	if err := e.Draw(x, y, f); err != nil {
		return err
	}
	if err := e.Draw(x+4, y+4, anim); err != nil {
		return err
	}
	t := e.Text()
	t.SetStyle(1, g.exeColors(specialInfoColors))
	if err := t.PrintCentered(x+0xde, y+0xa, name); err != nil {
		return err
	}
	return t.PrintParagraph(x+0x78, y+0x17, 0xcd, desc, 2)
}

// starToMonster is Star_To_Monster_ (0xfeb40): the guardian monster kind
// a star's special 9 shows, star % 5 + 1.
func starToMonster(star int) int { return int(int16(star))%5 + 1 }
