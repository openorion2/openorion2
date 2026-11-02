package game

import (
	"os"
	"strings"

	"github.com/openorion2/openorion2/internal/engine"
	"github.com/openorion2/openorion2/internal/font"
	"github.com/openorion2/openorion2/internal/gfx"
	"github.com/openorion2/openorion2/internal/save"
)

// Race selection, MOX\OTHERS\RACESEL.C: the race buttons, the ruler name,
// the banner. Spec: docs/screens/new-game.md.

const (
	raceSelLBX     = 0x178f59 // "RACESEL.LBX"
	raceStufLBX7   = 0x178f3c // "RACESTUF.LBX", entry 7: the races' picks
	lastRaceFile   = 0x178f4c // "lastrace.rac"
	patch13LBX     = 0x178f65 // "patch13.lbx"
	raceNameLBX    = 0x178f97 // "RACENAME.LBX"
	inBoxLBX       = 0x179ab8 // "inbox.lbx"
	escKey         = 0x178f88 // "\x1b"
	raceFmtHalf    = 0x178f71 // "1/2"
	raceFmtOne     = 0x178f75 // "1"
	raceFmtPlus    = 0x178f77 // "+"
	raceFmtComma   = 0x178f79 // ", "
	pickValueTable = 0x17d1f9 // 3 bytes per pick: the value of setting 1..3
	clockMouseList = 0x17cf82 // _cmbt_mouse_list_clock
	raceButtons    = 14       // 13 races and Custom
	raceCustom     = 13
)

// raceSelectState holds RACESEL.C's globals.
type raceSelectState struct {
	emActive      bool                      // port-only: in Evolutionary_Mutation_Screen_, for tests
	emShowing     bool                      // port-only: in its input loop, for tests
	buttonSeg     [raceButtons]*gfx.Picture // _race_button_seg, RACESEL.LBX 1..14
	specials      [raceButtons][]byte       // _race_specials: 0x1f picks per race, 13 the last custom race
	colorFlag     [8]bool                   // _color_flag: banners taken
	player        *save.Player              // the record being set up (the screens' argument)
	raceFlag      [raceButtons]bool         // _race_flag: races taken
	buttonStatus  [raceButtons]int16        // _race_button_status
	lastRaceSeg   *gfx.Picture              // _last_race_seg, patch13.lbx 1
	lastRaceName  [0xf]byte                 // _last_race_name
	customFlag    int16                     // _custom_flag
	oldRace       int16                     // _old_race
	displayedRace int16                     // _displayed_race

	// NAMESTAR.C's input box (0x19c330..).
	box inputBox
	opt raceOptState // RACEOPT.C
}

// inputBox is the record of NAMESTAR.C's Input_Box_Popup_ (0x19c330).
type inputBox struct {
	button, picture  *gfx.Picture // +0, +4: inbox.lbx 1 and 0
	colors, title    font.Colors  // +8, +0x10
	titleText        string       // +0x18, 0x3c bytes
	titleH           int16        // +0x54: 0x36
	remap            bool         // +0x56
	titleY           int16        // +0x57: y+3
	buttonX, buttonY int16        // +0x59, +0x5b
	fieldX, fieldY   int16        // +0x5d, +0x5f
	x, y             int16        // +0x61, +0x63
	maxChars         int16        // +0x65
	width, fieldW    int16        // +0x67, +0x69
	fieldH           int16        // +0x6b: 0x1a
	text             [0x64]byte   // +0x6d
	inputField       int          // +0xd1
	buttonField      int          // +0xd3
}

// raceSelLBXName is the race selection file; the flag screen names it in
// lower case, which DOS does not mind.
func (g *Game) raceSelLBXName() string { return strings.ToUpper(cStringAt(g.exe, raceSelLBX)) }

// raceSpecialsRecord is Far_Reload_Next_Data_(RACESTUF.LBX, 7, record i).
func (g *Game) raceSpecialsRecord(i int) ([]byte, error) {
	a, err := g.archive(strings.ToUpper(cStringAt(g.exe, raceStufLBX7)))
	if err != nil {
		return nil, err
	}
	t, err := a.Table(7)
	if err != nil {
		return nil, err
	}
	return append([]byte(nil), t.Record(i)...), nil
}

// reloadRaceSelectionScreen is Reload_Race_Selection_Screen_ (0x5bc74).
func (g *Game) reloadRaceSelectionScreen() error {
	rs := &g.rsel
	var err error
	for i := 0; i < 13; i++ {
		if rs.specials[i], err = g.raceSpecialsRecord(i); err != nil {
			return err
		}
	}
	if f, err := os.Open(g.findFile(cStringAt(g.exe, lastRaceFile))); err == nil {
		if rs.specials[raceCustom], err = g.raceSpecialsRecord(12); err != nil {
			f.Close()
			return err
		}
		var race [1]byte
		f.Read(rs.specials[raceCustom][:0x1f])
		convertCustomRaceFlags(g, rs.specials[raceCustom])
		f.Read(race[:])
		f.Read(rs.lastRaceName[:])
		f.Close()
	}
	for i := range rs.buttonSeg {
		if rs.buttonSeg[i], err = g.picture(g.raceSelLBXName(), i+1); err != nil {
			return err
		}
	}
	rs.lastRaceSeg, err = g.picture(strings.ToUpper(cStringAt(g.exe, patch13LBX)), 1)
	return err
}

// convertCustomRaceFlags is Convert_Custom_Race_Flags_ (0x5bc24): picks
// 1..9 from their values back to the setting 1..3, 0 for none.
func convertCustomRaceFlags(g *Game, p []byte) {
	for i := 0; i < 9; i++ {
		row := uint32(pickValueTable + (i+1)*3)
		v := p[i+1]
		switch v {
		case g.exeU8(row + 1):
			p[i+1] = 1
		case g.exeU8(row + 2):
			p[i+1] = 2
		case g.exeU8(row + 3):
			p[i+1] = 3
		default:
			p[i+1] = 0
		}
	}
}

// raceSelPickValue is the value of pick i at setting v (signed table byte).
func (g *Game) raceSelPickValue(i int, v int8) int8 {
	return int8(g.exeU8(uint32(pickValueTable + i*3 + int(v))))
}

// messageBuild is Message_Build_ (0x5c20e): the separator before item
// *count+1 of n: " and " (Ken text 0x68) before the last, ", " otherwise.
func (g *Game) messageBuild(count *int16, n int16, buf *string) {
	*count++
	switch {
	case n == *count:
		if n > 1 {
			s, err := g.kenText(0x68)
			if err != nil {
				g.logOnce(err.Error())
			}
			*buf += s
		}
	case *count > 1:
		*buf += cStringAt(g.exe, raceFmtComma)
	}
}

// drawRaceSelectionScreen is Draw_Race_Selection_Screen_ (0x5bd97), also
// the auto function; the 1.40 patch (0xf4faa) makes the shown race the old
// one at once.
func (g *Game) drawRaceSelectionScreen() error {
	e, rs := g.e, &g.rsel
	t := e.Text()
	colors := font.Colors{0x78, 0x7c, 0x7c, 0x7c, 0x7c, 0x7c, 0x7c, 0x7c}
	name := g.raceSelLBXName()
	if rs.customFlag == 0 {
		p, err := g.cached(name, 0x21)
		if err != nil {
			return err
		}
		if err := e.Draw(0x16e, 0x34, p); err != nil {
			return err
		}
	} else {
		p, err := g.cached(name, 0x1d)
		if err != nil {
			return err
		}
		if err := e.Draw(0x16f, 0x30, p); err != nil {
			return err
		}
	}
	d := rs.displayedRace
	if rs.oldRace != d {
		rs.buttonStatus[rs.oldRace] = 0
		rs.buttonStatus[d] = 1
		if !rs.raceFlag[d] {
			p, err := g.cached(name, int(d)+0xf)
			if err != nil {
				return err
			}
			e.DrawPalette(p)
		}
	} else if !rs.raceFlag[d] {
		p, err := g.cached(name, int(d)+0xf)
		if err != nil {
			return err
		}
		if err := e.Draw(0x36, 0x3f, p); err != nil {
			return err
		}
	} else {
		t.SetStyleShadow(4, colors, font.ShadowDown)
		msg, err := g.kenTextMax(0x48, 0x64) // EBX 0x64 from 0x5bea2
		if err != nil {
			return err
		}
		if err := t.PrintParagraph(0x3c, 0xaf, 0x118, msg, 2); err != nil {
			return err
		}
	}
	rs.oldRace = d
	show := false
	switch {
	case d >= 0xd && rs.specials[raceCustom] != nil:
		show = true
	case d == 0xd || rs.raceFlag[d]:
	default:
		show = true
	}
	if show {
		show = rs.customFlag == 0 && d != 0xd || rs.customFlag == 1 && d == 0xd
	}
	if show {
		if err := g.drawRaceSpecials(colors); err != nil {
			return err
		}
	}
	p, err := g.cached(name, 0)
	if err != nil {
		return err
	}
	return e.Draw(0x30, 0x3a, p)
}

// drawRaceSpecials is the description part of Draw_Race_Selection_Screen_
// (0x5bf3f): the race's picks, joined, and its government.
func (g *Game) drawRaceSpecials(colors font.Colors) error {
	e, rs := g.e, &g.rsel
	t := e.Text()
	d := rs.displayedRace
	sp := rs.specials[d]
	n := int16(1)
	for i := 1; i < 0x1f; i++ {
		if sp[i] != 0 {
			n++
		}
	}
	count := int16(0)
	buf := ""
	for i := 1; i < 10; i++ {
		if sp[i] == 0 {
			continue
		}
		g.messageBuild(&count, n, &buf)
		nm, err := g.specialAbilityName(i)
		if err != nil {
			return err
		}
		buf += nm
		v := g.raceSelPickValue(i, int8(sp[i]))
		var num string
		switch {
		case i == 5 && v == 1, i == 2 && v == 1:
			num = cStringAt(g.exe, raceFmtHalf)
		case i == 5:
			num = cStringAt(g.exe, raceFmtOne)
		case i == 2:
			num = itoa10(int32(v) / 2)
		default:
			num = itoa10(int32(v))
		}
		if v > 0 {
			buf += cStringAt(g.exe, raceFmtPlus)
		}
		buf += num
	}
	for i := 10; i < 0x1f; i++ {
		if sp[i] != 1 {
			continue
		}
		g.messageBuild(&count, n, &buf)
		nm, err := g.specialAbilityName(i)
		if err != nil {
			return err
		}
		buf += nm
	}
	g.messageBuild(&count, n, &buf)
	gov, err := g.eStringAt(governmentLabels + 4*uint32(int8(sp[0])))
	if err != nil {
		return err
	}
	buf += gov
	t.SetStyleShadow(2, colors, font.ShadowDown)
	t.SetOutlineColor(0, 0)
	if err := t.PrintParagraph(0x32, 0x187, 0x127, buf, 0); err != nil {
		return err
	}
	if d == 0xd {
		t.SetStyleShadow(4, colors, font.ShadowDown)
		t.SetOutlineColor(0, 0)
		if err := t.PrintCentered(0xc5, 0x2a, save.CString(rs.lastRaceName[:])); err != nil {
			return err
		}
	}
	return nil
}

func itoa10(v int32) string {
	if v < 0 {
		return "-" + itoa10(-v)
	}
	if v < 10 {
		return string(rune('0' + v))
	}
	return itoa10(v/10) + string(rune('0'+v%10))
}

// setRaceSelectHelp is Set_Race_Select_Help_ (0x5c25b): one entry per
// button (ids 0x276..0x283), then the whole screen (0x284).
func (g *Game) setRaceSelectHelp() {
	var list []engine.HelpEntry
	for i := 0; i < raceButtons; i++ {
		x := 0x15f + i/7*0x7e
		y := 0x5a + i%7*0x30
		list = append(list, engine.HelpEntry{ID: 0x276 + i, X1: x, Y1: y, X2: x + 0x7a, Y2: y + 0x2c})
	}
	list = append(list, engine.HelpEntry{ID: 0x284, X1: 0, Y1: 0, X2: 0x27f, Y2: 0x1df})
	g.e.SetHelpList(list)
}

// addRaceButtons adds the race buttons as Race_Selection_Screen_'s loops do,
// all n of them or the 13 races and then the custom one.
func (g *Game) addRaceButtons(fields *[raceButtons]int, n int) error {
	rs := &g.rsel
	for i := 0; i < n; i++ {
		f, err := g.e.AddRadioButtonField(int(int16(i/7*0x7e+0x15f)), int(int16(i%7*0x30+0x5a)), "", rs.buttonSeg[i], &rs.buttonStatus[i], "", 0x28)
		if err != nil {
			return err
		}
		fields[i] = g.raceKey(f)
	}
	return nil
}

// raceKey gives race button field n, under Options.NewGameKeys, the hot
// key 'A'+n-2 (tests).
func (g *Game) raceKey(n int) int {
	if g.newGameKeys {
		g.e.SetFieldHotKey(n, byte('A'+n-2))
	}
	return n
}

// raceSelectionBackground is the background redraw Race_Selection_Screen_
// repeats: RACEOPT.LBX 0 on the back page, copied to the off page.
func (g *Game) raceSelectionBackground() error {
	e := g.e
	e.V.SetPageBack()
	p, err := g.cached(cStringAt(g.exe, raceOptLBX), 0)
	if err != nil {
		return err
	}
	if err := e.Draw(0, 0, p); err != nil {
		return err
	}
	e.V.CopyBackToOff()
	return nil
}

// raceSelectionScreen is Race_Selection_Screen_ (0x5c510) for player pl:
// 1 when a race, a name and a banner were picked, 0 on Esc.
func (g *Game) raceSelectionScreen(pl *save.Player) (int16, error) {
	e, rs := g.e, &g.rsel
	rs.player = pl
	e.DisableCancel()
	e.DeactivateHelpList()
	e.ClearFields()
	if err := e.FastFadeOut(); err != nil {
		return 0, err
	}
	if err := e.LoadPalette(9, 0, 0xff); err != nil {
		return 0, err
	}
	rs.customFlag = 0
	rs.displayedRace = 3
	rs.buttonStatus = [raceButtons]int16{}
	rs.buttonStatus[3] = 1
	if err := g.reloadRaceSelectionScreen(); err != nil {
		return 0, err
	}
	g.setRaceSelectHelp()
	if err := g.raceSelectionBackground(); err != nil {
		return 0, err
	}
	esc := -1000
	addEsc := func() error {
		esc = -1000
		if g.gameType == 0 || g.gameType == 1 {
			var err error
			esc, err = e.AddHotKey(cStringAt(g.exe, escKey))
			return err
		}
		return nil
	}
	if err := addEsc(); err != nil {
		return 0, err
	}
	var fields [raceButtons]int
	if err := g.addRaceButtons(&fields, raceButtons); err != nil {
		return 0, err
	}
	e.AssignAutoFunction(g.drawRaceSelectionScreen, 2)
	rs.oldRace = 0
	e.V.SetPageOff()
	if err := g.drawRaceSelectionScreen(); err != nil {
		return 0, err
	}
	if err := e.DarkenPalette(100); err != nil {
		return 0, err
	}
	e.V.ClearPaletteChanges(0, 0xff)
	if err := e.TogglePages(); err != nil {
		return 0, err
	}
	if err := e.FastFadeIn(); err != nil {
		return 0, err
	}
	// rebuild puts the race buttons back after a cancelled pick.
	rebuild := func(n int) error {
		if err := g.raceSelectionBackground(); err != nil {
			return err
		}
		if err := addEsc(); err != nil {
			return err
		}
		if err := g.addRaceButtons(&fields, n); err != nil {
			return err
		}
		if n < raceButtons {
			f, err := e.AddRadioButtonField(0x1dd, 0x17a, "", rs.buttonSeg[raceCustom], &rs.buttonStatus[raceCustom], "", 0x28)
			if err != nil {
				return err
			}
			fields[raceCustom] = g.raceKey(f)
		}
		return nil
	}
	done := false
	for {
		if done {
			e.ClearFields()
			e.DeactivateAutoFunction()
			if err := e.FastFadeOut(); err != nil {
				return 0, err
			}
			return 1, nil
		}
		r, err := e.GetInput()
		if err != nil {
			return 0, err
		}
		sc, err := e.ScanInput()
		if err != nil {
			return 0, err
		}
		rs.displayedRace = int16(sc - fields[0])
		if rs.displayedRace < 0 || rs.displayedRace > 0xd {
			rs.displayedRace = rs.oldRace
		}
		for i := 0; i <= 0xc; i++ {
			if r != fields[i] || rs.raceFlag[i] {
				continue
			}
			if ok, err := g.netRequestRace(uint8(i)); err != nil {
				return 0, err
			} else if !ok {
				continue
			}
			if rs.customFlag != 0 {
				// Custom mode: race i is the base of a custom race.
				setPicks(pl, rs.specials[i])
				pl.Race = uint8(i)
				if err := g.copyRaceName(i); err != nil {
					return 0, err
				}
				e.DeactivateAutoFunction()
				e.ClearFields()
				ok, err := g.racialOptionScreen()
				if err != nil {
					return 0, err
				}
				if ok == 1 {
					done = true
					rs.raceFlag[i] = true
					continue
				}
				g.netUnrequestRace(uint8(i))
				if err := g.reloadRaceSelectionScreen(); err != nil {
					return 0, err
				}
				if err := rebuild(13); err != nil {
					return 0, err
				}
				e.AssignAutoFunction(g.drawRaceSelectionScreen, 2)
				g.setRaceSelectHelp()
				rs.customFlag = 0
				continue
			}
			e.MarkTime()
			if err := e.ReleaseTime(5); err != nil {
				return 0, err
			}
			e.ClearFields()
			setPicks(pl, rs.specials[i])
			pl.Race = uint8(i)
			if err := g.copyRaceName(i); err != nil {
				return 0, err
			}
			e.DeactivateHelpList()
			var res int16
			for {
				if res, err = g.namingPopup(pl.EmperorName[:], int16(i)); err != nil {
					return 0, err
				}
				if err := g.raceSelectionBackground(); err != nil {
					return 0, err
				}
				if err := e.TogglePages(); err != nil {
					return 0, err
				}
				rs.oldRace = int16(i)
				if res == 1 {
					f, err := g.flagScreen(&pl.Color)
					if err != nil {
						return 0, err
					}
					res = f + 1
					if res != 2 {
						if err := g.reloadRaceSelectionScreen(); err != nil {
							return 0, err
						}
					}
				}
				if res != 1 {
					break
				}
			}
			if res == 2 {
				for k := 1; k < 10; k++ {
					if pl.Picks[k] != 0 {
						pl.Picks[k] = g.raceSelPickValue(k, pl.Picks[k])
					}
				}
				done = true
				rs.raceFlag[i] = true
				continue
			}
			g.netUnrequestRace(uint8(i))
			g.setRaceSelectHelp()
			rs.oldRace = 0
			rs.buttonStatus = [raceButtons]int16{}
			rs.buttonStatus[rs.displayedRace] = 1
			if err := rebuild(raceButtons); err != nil {
				return 0, err
			}
			e.V.SetPageOff()
			if err := g.drawRaceSelectionScreen(); err != nil {
				return 0, err
			}
			if err := e.TogglePages(); err != nil {
				return 0, err
			}
			e.AssignAutoFunction(g.drawRaceSelectionScreen, 2)
		}
		if r == esc {
			if rs.customFlag != 1 {
				e.ClearFields()
				e.DeactivateAutoFunction()
				return 0, nil
			}
			rs.customFlag = 0
			e.ClearFieldsAbove(fields[12])
			// No hot key here even under Options.NewGameKeys.
			f, err := e.AddRadioButtonField(0x1dd, 0x17a, "", rs.buttonSeg[raceCustom], &rs.buttonStatus[raceCustom], "", 0x28)
			if err != nil {
				return 0, err
			}
			fields[raceCustom] = f
		}
		if r == fields[raceCustom] {
			if err := g.customRaceButton(&fields, &done, rebuild); err != nil {
				return 0, err
			}
		}
		if !done {
			if g.fullDrawScreen {
				e.V.SetPageBack()
				e.V.Fill(0, 0, 0x27f, 0x1df, 0)
				if err := g.raceSelectionBackgroundPage(); err != nil {
					return 0, err
				}
				g.fullDrawScreen = false
			}
			e.V.SetPageOff()
			if err := g.drawRaceSelectionScreen(); err != nil {
				return 0, err
			}
			if err := e.TogglePages(); err != nil {
				return 0, err
			}
		}
	}
}

// raceSelectionBackgroundPage draws RACEOPT.LBX 0 on the current (back)
// page and copies it to the off page.
func (g *Game) raceSelectionBackgroundPage() error {
	p, err := g.cached(cStringAt(g.exe, raceOptLBX), 0)
	if err != nil {
		return err
	}
	if err := g.e.Draw(0, 0, p); err != nil {
		return err
	}
	g.e.V.CopyBackToOff()
	return nil
}

// customRaceButton is the Custom button of Race_Selection_Screen_
// (0x5cbf2): the first press enters custom mode (pick the base race next;
// with lastrace.rac the 1.40 patch adds a button for the saved race), the
// saved race's button loads it into the race picks screen.
func (g *Game) customRaceButton(fields *[raceButtons]int, done *bool, rebuild func(int) error) error {
	e, rs := g.e, &g.rsel
	pl := g.rsel.player
	f, err := os.Open(g.findFile(cStringAt(g.exe, lastRaceFile)))
	have := err == nil
	if rs.customFlag == 0 {
		rs.customFlag = 1
		e.ClearFieldsAbove(fields[12])
		if !have {
			fields[raceCustom] = -1000
			return nil
		}
		defer f.Close()
		if g.currentScreen < 0xf {
			n, err := e.AddRadioButtonField(0x1dd, 0x17a, "", rs.lastRaceSeg, &rs.buttonStatus[raceCustom], "", 0x28)
			if err != nil {
				return err
			}
			fields[raceCustom] = n
		} else {
			// The 1.40 patch skips the button on the multiplayer screens
			// (0xf4f8d) and leaves eax, the x 0x1dd, as the field.
			fields[raceCustom] = 0x1dd
		}
		return nil
	}
	if !have {
		return nil
	}
	defer f.Close()
	var picks [0x1f]byte
	var race [1]byte
	f.Read(picks[:])
	setPicks(pl, rs.specials[raceCustom])
	f.Read(race[:])
	pl.Race = race[0]
	if g.gameType >= 2 {
		// 0x5ccc5 asks for the race scan's leftover index 13, not the saved
		// race; unreachable, as the button is never there in network games.
		ok, err := g.requestRace(13)
		if err != nil {
			return err
		}
		if !ok {
			msg, err := g.kenTextMax(0x48, 0x64)
			if err != nil {
				return err
			}
			if err := g.messageBox(msg); err != nil {
				return err
			}
			g.e.V.SetPageBack()
			return g.raceSelectionBackgroundPage()
		}
	}
	var name [0xf]byte
	f.Read(name[:])
	copy(pl.RaceName[:], name[:])
	e.DeactivateAutoFunction()
	e.ClearFields()
	rs.oldRace = int16(race[0])
	ok, err := g.racialOptionScreen()
	if err != nil {
		return err
	}
	if ok == 1 {
		*done = true
		rs.raceFlag[race[0]] = true
		return nil
	}
	g.netUnrequestRace(race[0]) // the saved race, not the 13 requested
	if err := g.reloadRaceSelectionScreen(); err != nil {
		return err
	}
	if err := rebuild(13); err != nil {
		return err
	}
	e.AssignAutoFunction(g.drawRaceSelectionScreen, 2)
	g.setRaceSelectHelp()
	rs.customFlag = 0
	return nil
}

// copyRaceName is the strcpy of _race_names[i] into the player's race name.
func (g *Game) copyRaceName(i int) error {
	n, err := g.eStringAt(raceNames + 4*uint32(i))
	if err != nil {
		return err
	}
	pl := g.rsel.player
	k := copy(pl.RaceName[:], n)
	if k < len(pl.RaceName) {
		pl.RaceName[k] = 0
	}
	return nil
}

// setPicks is the Copy_Structure_ of 0x1f picks into player +0x89f.
func setPicks(pl *save.Player, b []byte) {
	for i := range pl.Picks {
		pl.Picks[i] = int8(b[i])
	}
}

// drawFlagScreen is Draw_Flag_Screen_ (0x5cf37): the banner frame and the
// free banners of the race.
func (g *Game) drawFlagScreen() error {
	e, rs := g.e, &g.rsel
	name := g.raceSelLBXName()
	p, err := g.cached(name, 0x20)
	if err != nil {
		return err
	}
	if err := e.Draw(0x37, 0x43, p); err != nil {
		return err
	}
	for c := 0; c < 8; c++ {
		if rs.colorFlag[c] {
			continue
		}
		dx := 0
		if c == 3 || c == 7 {
			dx = -1
		}
		p, err := g.cached(name, int(rs.oldRace)*8+c+0x22)
		if err != nil {
			return err
		}
		if err := e.Draw(dx+0x60+c%4*0x7c, 0x91+c/4*0x88, p); err != nil {
			return err
		}
	}
	return nil
}

// flagScreen is Flag_Screen_ (0x5d03c): pick a free banner; 1 when picked,
// 0 on Esc.
func (g *Game) flagScreen(color *uint8) (int16, error) {
	e, rs := g.e, &g.rsel
	if err := e.TogglePages(); err != nil {
		return 0, err
	}
	p, err := g.cached(g.raceSelLBXName(), 0x20)
	if err != nil {
		return 0, err
	}
	e.DrawPalette(p)
	if err := e.TogglePages(); err != nil {
		return 0, err
	}
	e.AssignAutoFunction(g.drawFlagScreen, 2)
	e.SetHelpList([]engine.HelpEntry{{ID: 0x2a4, X1: 0, Y1: 0, X2: 0x27f, Y2: 0x1df}})
	for done := false; ; {
		if done {
			e.ClearFields()
			e.DeactivateAutoFunction()
			list := g.exeMouseList(clockMouseList, 1)
			list[0].Image = 7
			e.SetMouseList(list)
			e.V.SetPageOff()
			if err := g.drawFlagScreen(); err != nil {
				return 0, err
			}
			return 1, e.TogglePages()
		}
		e.ClearFields()
		var fields [8]int
		for c := 0; c < 8; c++ {
			if rs.colorFlag[c] {
				fields[c] = -1000
				continue
			}
			dx := 0
			if c == 3 || c == 7 {
				dx = -1
			}
			x, y := c%4*0x7c, c/4*0x88
			if fields[c], err = e.AddHiddenField(dx+0x60+x, y+0x90, dx+0xb3+x, y+0xf2, g.flagKey(), 0x29); err != nil {
				return 0, err
			}
		}
		esc, err := e.AddHotKey(cStringAt(g.exe, escKey))
		if err != nil {
			return 0, err
		}
		r, err := e.GetInput()
		if err != nil {
			return 0, err
		}
		for c := 0; c < 8; c++ {
			if r != fields[c] {
				continue
			}
			ok := true
			if g.gameType >= 2 {
				if ok, err = g.requestBannerColor(uint8(c)); err != nil {
					return 0, err
				}
			}
			if !ok {
				// Ken text 0x69: the banner went to another player.
				msg, err := g.kenText(0x69)
				if err != nil {
					return 0, err
				}
				if err := g.messageBox(msg); err != nil {
					return 0, err
				}
				continue
			}
			*color = uint8(c)
			done = true
			rs.colorFlag[c] = true
		}
		if r == esc {
			e.ClearFields()
			e.DeactivateAutoFunction()
			g.hotSeatFlag = 0
			return 0, nil
		}
		if !done {
			e.V.SetPageOff()
			if err := g.drawFlagScreen(); err != nil {
				return 0, err
			}
			if err := e.TogglePages(); err != nil {
				return 0, err
			}
		}
	}
}

// flagKey is the banners' hot key: none, or 1 under Options.NewGameKeys,
// so key 1 takes the first free banner.
func (g *Game) flagKey() string {
	if g.newGameKeys {
		return "1"
	}
	return ""
}

// drawNamingPopup is Draw_Naming_Popup_ (0x5d273), the auto function under
// the input box.
func (g *Game) drawNamingPopup() error {
	if !g.fullDrawScreen {
		return nil
	}
	g.e.V.SetPageOff()
	g.e.V.Fill(0, 0, 0x27f, 0x1df, 0)
	p, err := g.cached(cStringAt(g.exe, raceOptLBX), 0)
	if err != nil {
		return err
	}
	return g.e.Draw(0, 0, p)
}

// namingPopup is Naming_Popup_ (0x5d2bb): the ruler name, one of the race's
// eight in RACENAME.LBX by default; 1 when accepted.
func (g *Game) namingPopup(dst []byte, race int16) (int16, error) {
	e := g.e
	e.ClearFields()
	if err := e.TogglePages(); err != nil {
		return 0, err
	}
	e.AssignAutoFunction(g.drawNamingPopup, 1)
	r := int16(g.state.Random.Random(8))
	a, err := g.archive(cStringAt(g.exe, raceNameLBX))
	if err != nil {
		return 0, err
	}
	t, err := a.Table(0)
	if err != nil {
		return 0, err
	}
	rec := t.Record(int(int16(r - 1 + race*8)))
	def := save.CString(rec)
	name := []byte(save.CString(rec))
	title, err := g.kenText(0x6a)
	if err != nil {
		return 0, err
	}
	ok, err := g.inputBoxPopup(&name, 0x14, 0x66, title, 0xb1, 0x7d, true)
	if err != nil {
		return 0, err
	}
	s := string(name)
	if len(s) == 0 {
		s = def
	}
	k := copy(dst, s)
	if k < len(dst) {
		dst[k] = 0
	}
	e.ClearFields()
	return ok, nil
}

// inputBoxPopup is Input_Box_Popup_ (0x91f14), Remapped_Input_Box_Popup_
// (0x91bb4) with remap: a titled box with a text field and a button; 1 when
// accepted (Enter or the button), 0 on cancel with the text put back.
func (g *Game) inputBoxPopup(text *[]byte, maxChars, width int16, title string, x, y int16, remap bool) (int16, error) {
	e, b := g.e, &g.rsel.box
	helpList, helpOn := e.HelpList()
	savedAuto := e.GetAutoFunction()
	g.setInputBoxHelpList()
	*b = inputBox{}
	t := *text
	if len(t) >= 0x64 {
		t = t[:0x64]
	}
	copy(b.text[:], t)
	b.titleText = title
	if len(title) >= 0x3c {
		b.titleText = title[:0x3c]
	}
	b.maxChars = maxChars
	b.titleY = y + 3
	b.remap = remap
	b.buttonX, b.buttonY = x+0x60, y+0x64
	b.fieldX = x + 0x22
	b.titleH = 0x36
	b.fieldY = y + 0x36
	b.fieldW = width
	if width == 0 || width > 0xcd {
		b.fieldW = 0xcd
	}
	b.x, b.y = x, y
	b.fieldH = 0x1a
	b.width = width
	if g.currentScreen == ScreenNewGame {
		g.moxFontColorsOffset = 0x14
		b.colors = g.fontColors2(3, 0, 0x14)
	} else {
		g.moxFontColorsOffset = 0x10
		b.colors = g.fontColors2(3, 0xc, 0x18)
	}
	saved := b.text
	if err := g.setupInputBoxPopup(savedAuto); err != nil {
		return 0, err
	}
	ok := int16(1)
	for done, n := 0, 1; done != 1; {
		if done == 0 {
			r, err := e.GetInput()
			if err != nil {
				return 0, err
			}
			if r == b.inputField {
				if n > 0 {
					done = 1
				}
				n++
			}
			if int16(r) < 0 {
				done = 1
				b.text = saved
				ok = 0
			}
			if r == b.buttonField {
				done = 2
			}
			if done == 0 {
				e.V.SetPageOff()
				if err := g.inputBoxFullRedraw(savedAuto); err != nil {
					return 0, err
				}
				if err := e.TogglePages(); err != nil {
					return 0, err
				}
				continue
			}
		}
		if done == 2 {
			if !b.remap {
				break
			}
			e.V.SetPageOff()
			b.button.CurrentFrame = 1
			if err := g.remapDraw(int(b.buttonX), int(b.buttonY), b.button); err != nil {
				return 0, err
			}
			if err := g.inputBoxFullRedraw(savedAuto); err != nil {
				return 0, err
			}
			if err := e.TogglePages(); err != nil {
				return 0, err
			}
			break
		}
	}
	*text = []byte(trimSpaces(save.CString(b.text[:])))
	e.ClearFields()
	e.DeactivateAutoFunction()
	if helpOn {
		e.SetHelpList(helpList)
	} else {
		e.DeactivateHelpList()
	}
	e.PutAutoFunction(savedAuto)
	e.ActivateAutoFunction()
	g.fullDrawScreen = true
	return ok, nil
}

// inputBoxFullRedraw is the full redraw the input box's loop and its auto
// function (Draw_Input_Box_Popup_ 0x91b89) do when _FULL_DRAW_SCREEN_FLAG
// is set.
func (g *Game) inputBoxFullRedraw(saved engine.AutoFunction) error {
	if !g.fullDrawScreen {
		return nil
	}
	if err := g.setupInputBoxPopup(saved); err != nil {
		return err
	}
	g.fullDrawScreen = false
	return nil
}

// setInputBoxHelpList is Set_Input_Box_Help_List_ (0x6fb5e).
func (g *Game) setInputBoxHelpList() {
	g.e.DeactivateHelpList()
	g.e.SetHelpList(g.helpListAt(0x181f88, 1))
}

// setupInputBoxPopup is Setup_Input_Box_Popup_ (0x91bd4): the box over the
// saved auto function's screen, its title, button and text field.
func (g *Game) setupInputBoxPopup(saved engine.AutoFunction) error {
	e, b := g.e, &g.rsel.box
	t := e.Text()
	e.ClearFields()
	var err error
	name := strings.ToUpper(cStringAt(g.exe, inBoxLBX))
	if b.picture, err = g.picture(name, 0); err != nil {
		return err
	}
	if b.button, err = g.picture(name, 1); err != nil {
		return err
	}
	e.V.SetPageOff()
	if err := saved.Run(); err != nil {
		return err
	}
	e.V.CopyOffToBack()
	if err := e.TogglePages(); err != nil {
		return err
	}
	e.V.SetRefreshStencil()
	e.V.SetPageOff()
	w := int16(b.picture.Width)
	if b.remap {
		if err := e.CalculatePictureRemapColors(b.picture); err != nil {
			return err
		}
		if err := e.CalculatePictureRemapColors(b.button); err != nil {
			return err
		}
		if err := g.remapDraw(int(b.x), int(b.y), b.picture); err != nil {
			return err
		}
		b.button.CurrentFrame = 0
		if err := g.remapDraw(int(b.buttonX), int(b.buttonY), b.button); err != nil {
			return err
		}
	} else {
		if err := e.Draw(int(b.x), int(b.y), b.picture); err != nil {
			return err
		}
		b.title = b.colors
	}
	g.setFittedFontStyle(b.titleText, b.colors, 4, 2, 0, int(int16(w-0x36)), 1)
	style, sp := t.Style, t.Spacing
	if g.currentScreen == ScreenNewGame {
		g.moxFontColorsOffset = 0x14
		b.title = g.fontColors2(style, 0xa, 0x1a)
	} else {
		g.moxFontColorsOffset = 0x13
		b.title = g.fontColors2(style, 0xc, 0x18)
	}
	if b.remap {
		g.setRemapFontStyle(4, b.title)
	} else {
		t.SetStyle(style, b.title)
	}
	t.SetSpacing(sp)
	tx := int(int16((int32(w)-int32(int16(t.Width([]byte(b.titleText)))))/2) + b.x)
	ty := int(int16((int32(b.titleH)-int32(int16(t.Height())))/2) + b.titleY)
	if err := t.Print(tx, ty, b.titleText); err != nil {
		return err
	}
	e.V.CopyOffToBack()
	if err := e.TogglePages(); err != nil {
		return err
	}
	e.V.SetPageOff()
	b.title[0] = 1
	if b.remap {
		g.setRemapFontStyle(3, b.colors)
		f, err := e.AddHiddenField(int(b.buttonX), int(b.buttonY), int(int16(b.buttonX+int16(b.button.Width))), int(int16(b.buttonY+int16(b.button.Height))), "", 0x28)
		if err != nil {
			return err
		}
		b.buttonField = f
	} else {
		t.SetStyle(3, b.colors)
		f, err := e.AddButtonField(int(b.buttonX), int(b.buttonY), "", b.button, "", 0x28)
		if err != nil {
			return err
		}
		b.buttonField = f
	}
	f, err := e.AddContinuousStringInputField(int(b.fieldX), int(b.fieldY), int(b.fieldW), int(b.fieldH), b.text[:], int(b.maxChars), 0, nil, 0x29, 0)
	if err != nil {
		return err
	}
	b.inputField = f
	e.ActivateInputField(f)
	e.AssignAutoFunction(func() error { return g.drawInputBoxPopup(saved) }, 1)
	g.fullDrawScreen = false
	return nil
}

// drawInputBoxPopup is Draw_Input_Box_Popup_ (0x91b89).
func (g *Game) drawInputBoxPopup(saved engine.AutoFunction) error {
	return g.inputBoxFullRedraw(saved)
}

var _ = save.CString

// netRequestRace is the race pick's Request_Race_ in game types 2 and up
// (0x5c718..0x5c739); a refusal shows Ken text 0x48 and redraws the
// background (0x5cb2b). Always true in single player and hotseat.
func (g *Game) netRequestRace(r uint8) (bool, error) {
	if g.gameType == 0 || g.gameType == 1 {
		return true, nil
	}
	ok, err := g.requestRace(r)
	if err != nil || ok {
		return ok, err
	}
	msg, err := g.kenTextMax(0x48, 0x64) // 0x5cb2b
	if err != nil {
		return false, err
	}
	if err := g.messageBox(msg); err != nil {
		return false, err
	}
	g.e.V.SetPageBack()
	return false, g.raceSelectionBackgroundPage()
}

// netUnrequestRace gives a race back: Unrequest_Race_ in game types 2 and
// up, else the flag alone.
func (g *Game) netUnrequestRace(r uint8) {
	if g.gameType == 0 || g.gameType == 1 {
		g.rsel.raceFlag[r] = false
		return
	}
	g.unrequestRace(r)
}
