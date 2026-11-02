package game

import (
	"os"
	"strings"

	"github.com/openorion2/openorion2/internal/engine"
	"github.com/openorion2/openorion2/internal/font"
	"github.com/openorion2/openorion2/internal/gfx"
	"github.com/openorion2/openorion2/internal/save"
)

// The custom race picks, MOX\OTHERS\RACEOPT.C. Spec: docs/screens/new-game.md.

const (
	raceOptLBXOpt  = 0x178f09 // "RACEOPT.LBX"
	raceStufLower  = 0x178efc // "racestuf.lbx": entry language the option names, 6 the costs
	patch13Title   = 0x178f15 // "patch13.lbx"
	raceOptDot     = 0x178f22 // "."
	raceOptPercent = 0x178f24 // "%"
	raceOptAccept  = 0x178f38 // "a"
	raceOptDefault = 0x178f3a // "c"
	raceOptEsc     = 0x178f36 // "\x1b"
	lastRaceWrite  = 0x178f29 // "lastrace.rac"
	startPicks     = 0x178513 // the 1.40 patch's picks to spend (Calculate_Picks_ 0xf4d53)
	optToggles     = 22
)

// The columns and rows of the picks (locals of the RACEOPT.C routines).
var (
	raceOptX = [4]int{0x50, 0xfe, 0x1ba, 0x283}
	raceOptY = [5]int{0x48, 0x93, 0xde, 0x129, 0x174}
)

// raceOptState holds RACEOPT.C's globals.
type raceOptState struct {
	colorOpt                font.Colors       // _color_opt
	specialSeg, defaultSeg  *gfx.Picture      // RACEOPT.LBX 1, 2
	acceptSeg, offSeg       *gfx.Picture      // 3, 5
	optString               [0x40]string      // _opt_string
	costs                   []byte            // _ability_costs
	button                  [10]int16         // _button_variable: 9 groups and the government
	toggle                  [optToggles]int16 // _option_toggle
	optionButton            [optToggles]int   // _option_button
	groupButton             [0x29]int         // _group_button
	picks, negativePicks    int16
	defalt, exitKen, accept int
	nameButton              int // _name_button
	curRaceName             [0x12]byte
}

func (g *Game) cost(i int) int16 { return int16(int8(g.rsel.opt.costs[i])) }

// reloadRacialOptionScreen is Reload_Racial_Option_Screen_ (0x58fc3) with
// Reload_Option_Strings_ (0x5aef4).
func (g *Game) reloadRacialOptionScreen() error {
	o := &g.rsel.opt
	name := cStringAt(g.exe, raceOptLBXOpt)
	var err error
	for _, p := range []struct {
		dst **gfx.Picture
		n   int
	}{{&o.specialSeg, 1}, {&o.defaultSeg, 2}, {&o.acceptSeg, 3}, {&o.offSeg, 5}} {
		if *p.dst, err = g.picture(name, p.n); err != nil {
			return err
		}
	}
	a, err := g.archive(strings.ToUpper(cStringAt(g.exe, raceStufLower)))
	if err != nil {
		return err
	}
	data, err := a.Entry(int(g.settings[0xd2]))
	if err != nil {
		return err
	}
	if o.costs, err = a.Entry(6); err != nil {
		return err
	}
	at := func(i int) byte {
		if i < len(data) {
			return data[i]
		}
		return 0
	}
	i := 0
	o.optString[0] = save.CString(data)
	for n := 1; n < 0x40; n++ {
		for at(i) != 0 {
			i++
		}
		for at(i) == 0 && i < len(data) {
			i++
		}
		o.optString[n] = save.CString(data[min(i, len(data)):])
	}
	return nil
}

// calculateRacePicks is Calculate_Picks_ (0x59053): the picks left and the
// sum of the negative picks taken.
func (g *Game) calculateRacePicks() {
	o := &g.rsel.opt
	picks := int16(g.exeU8(startPicks))
	neg := int16(0)
	take := func(c int16) {
		picks -= c
		if c < 0 {
			neg += c
		}
	}
	for k := 0; k < 9; k++ {
		if o.button[k] != 0 {
			take(g.cost(k*3 + int(o.button[k]) - 1))
		}
	}
	take(g.cost(int(o.button[9]) + 0x1b))
	for t := 0; t < optToggles; t++ {
		if o.toggle[t] == 1 {
			take(g.cost(t + 0x1f))
		}
	}
	o.negativePicks, o.picks = neg, picks
}

// convertPlayerToFlags is Convert_Player_To_Flags_ (0x5ad97): the picks
// into the buttons and toggles; pick 15 (rich or poor home world) takes
// two toggles.
func (g *Game) convertPlayerToFlags(pl *save.Player) {
	o := &g.rsel.opt
	p := &pl.Picks
	o.button[9] = int16(int32(p[0]) / 2)
	for k := 0; k < 9; k++ {
		o.button[k] = int16(p[k+1])
	}
	off := 0
	for k := 10; k < 0x1f; k++ {
		i := k - 10 + off
		o.toggle[i] = int16(p[k])
		if k == 0xf {
			if o.toggle[i] == -1 {
				o.toggle[i], o.toggle[i+1] = 0, 1
			} else {
				o.toggle[i+1] = 0
			}
			off = 1
		}
	}
}

// convertFlagsToPlayer is Convert_Flags_To_Player_ (0x5ae38).
func (g *Game) convertFlagsToPlayer(pl *save.Player) {
	o := &g.rsel.opt
	p := &pl.Picks
	p[0] = int8(o.button[9] * 2)
	for k := 1; k < 10; k++ {
		if o.button[k-1] != 0 {
			p[k] = g.raceSelPickValue(k, int8(o.button[k-1]))
		} else {
			p[k] = 0
		}
	}
	off := 0
	for k := 10; k < 0x20; k++ {
		if k == 0xf {
			switch {
			case o.toggle[5] == 1:
				p[k] = 1
			case o.toggle[6] == 1:
				p[k] = -1
			default:
				p[k] = 0
			}
			off = 1
			k++
			continue
		}
		p[k-off] = int8(o.toggle[k-10])
	}
}

// realAbilityCost is Real_Ability_Cost_ (0x5af69): the cost of switching
// toggle k, counting what it switches off.
func (g *Game) realAbilityCost(k int) int16 {
	o := &g.rsel.opt
	c := g.cost(k + 0x1f)
	if o.toggle[k] == 1 {
		c = -c
	}
	on := func(t int) bool { return o.toggle[t] == 1 }
	switch {
	case k == 5 && on(6):
		c -= g.cost(0x25)
	case k == 6 && on(5):
		c -= g.cost(0x24)
	case k == 0 && on(1):
		c -= g.cost(0x20)
	case k == 1 && on(0):
		c -= g.cost(0x1f)
	case k == 9:
		if on(8) {
			c -= g.cost(0x27)
		}
		if o.button[1] != 0 {
			c -= g.cost(int(o.button[1]) + 2)
		}
	case k == 8 && on(9):
		c -= g.cost(0x28)
	case k == 10 && on(11):
		c -= g.cost(0x2a)
	case k == 11 && on(10):
		c -= g.cost(0x29)
	case k == 13 && on(12):
		c -= g.cost(0x2b)
	case k == 12 && on(13):
		c -= g.cost(0x2c)
	}
	return c
}

// setRaceOptionHelpList is Set_Race_Option_Help_List_ (0x59fa1).
func (g *Game) setRaceOptionHelpList() {
	h := func(id, x1, y1, x2, y2 int) engine.HelpEntry {
		return engine.HelpEntry{ID: id, X1: x1, Y1: y1, X2: x2, Y2: y2}
	}
	list := []engine.HelpEntry{
		h(0x1bb, 0x36, 0x31, 0xd1, 0x6f), h(0x1bc, 0x36, 0x7b, 0xd1, 0xb9), h(0x1bd, 0x36, 0xc5, 0xd0, 0x103),
		h(0x1be, 0x36, 0x10f, 0xd0, 0x14d), h(0x1bf, 0x36, 0x159, 0xd0, 0x19b), h(0x1c0, 0xe5, 0x31, 0x18b, 0x70),
		h(0x1c1, 0xe5, 0x7b, 0x18b, 0xba), h(0x1c2, 0xe5, 0xc5, 0x18b, 0x104), h(0x1c3, 0xe5, 0x10f, 0x18b, 0x14e),
	}
	for k := 0; k < optToggles; k++ {
		list = append(list, h(0x1c4+k, 0x1a6, 0x48+k*0x10, 0x252, 0x51+k*0x10))
	}
	list = append(list,
		h(0x1df, 0xbe, 0x1ba, 0x135, 0x1d0), h(0x1e0, 0x146, 0x1ba, 0x1c9, 0x1d1), h(0x1de, 0x49, 0x1be, 0x92, 0x1d7),
		h(0x1e1, 0x1f8, 0x1bf, 0x23e, 0x1d6), h(0x1da, 0xe3, 0x173, 0x18e, 0x17e), h(0x1db, 0xe3, 0x180, 0x18e, 0x18b),
		h(0x1dc, 0xe3, 0x18d, 0x18e, 0x198), h(0x1dd, 0xe3, 0x19a, 0x18e, 0x1a5))
	g.e.SetHelpList(list)
}

// addRaceOptionsFields is Add_Race_Options_Fields_ (0x5aad4): the race name
// field (kept once added), Esc, ACCEPT "a", DEFAULT "c", a hidden field per
// level of the nine groups and the government (none for a group's title),
// one per toggle.
func (g *Game) addRaceOptionsFields() error {
	e, o := g.e, &g.rsel.opt
	var err error
	if o.nameButton > 0 {
		e.ClearFieldsAbove(o.nameButton)
	} else {
		e.ClearFields()
		e.Text().SetStyle(4, o.colorOpt)
		if o.nameButton, err = e.AddContinuousStringInputField(0xf8, 0xf, 0x96, 0xe, o.curRaceName[:], 0xf, 0, nil, 0, 0); err != nil {
			return err
		}
	}
	if o.exitKen, err = e.AddHotKey(cStringAt(g.exe, raceOptEsc)); err != nil {
		return err
	}
	if o.accept, err = e.AddButtonField(0x1fa, 0x1c0, "", o.acceptSeg, cStringAt(g.exe, raceOptAccept), 0x28); err != nil {
		return err
	}
	if o.defalt, err = e.AddButtonField(0x4c, 0x1c0, "", o.defaultSeg, cStringAt(g.exe, raceOptDefault), 0x28); err != nil {
		return err
	}
	hidden := func(x1, y1, x2, y2 int) (int, error) {
		return e.AddHiddenField(int(int16(x1)), int(int16(y1)), int(int16(x2)), int(int16(y2)), "", 0x29)
	}
	row, col := 0, 0
	si := 0
	for ; si < 0x24; row++ {
		o.groupButton[si] = -1000
		si++
		for yoff := 0; yoff != 0x27; yoff += 0xd {
			if o.groupButton[si], err = hidden(raceOptX[col]-0x14, raceOptY[row]+yoff, raceOptX[col+1]-0x32, raceOptY[row]+9+yoff); err != nil {
				return err
			}
			si++
		}
		if row == 4 {
			row, col = -1, col+1
		}
	}
	o.groupButton[si] = -1000
	si++
	for yoff := 0; yoff != 0x34; yoff += 0xd {
		if o.groupButton[si], err = hidden(raceOptX[col]-0x14, raceOptY[row]+yoff, raceOptX[col+1]-0x32, raceOptY[row]+9+yoff); err != nil {
			return err
		}
		si++
	}
	col++
	y := raceOptY[0]
	for t := 0; t < optToggles; t++ {
		if o.optionButton[t], err = hidden(raceOptX[col]-0x14, y, raceOptX[col]+0x98, y+9); err != nil {
			return err
		}
		y += 0x10
	}
	return nil
}

// optLine prints one pick: its name, dots and its cost right-aligned at
// the next column (the pattern all three RACEOPT.C drawers repeat).
// dotsFirst is the order Draw_Race_Option_Background_'s toggles use.
func (g *Game) optLine(col, y, cx, c int, dotsFirst bool) error {
	t := g.e.Text()
	o := &g.rsel.opt
	buf := itoa10(int32(int8(o.costs[c])))
	name := o.optString[cx]
	x0, x1 := raceOptX[col], raceOptX[col+1]
	dots := func() error {
		di := int16(int16(t.Width([]byte(name))) + int16(x0) - 1)
		limit := int16(int16(x1-0x32) - int16(t.Width([]byte(buf))))
		for int32(di) < int32(limit)-4 {
			if err := t.Print(int(di), y, cStringAt(g.exe, raceOptDot)); err != nil {
				return err
			}
			di += 4
		}
		return nil
	}
	if dotsFirst {
		if err := dots(); err != nil {
			return err
		}
	}
	if err := t.Print(x0, y, name); err != nil {
		return err
	}
	if !dotsFirst {
		if err := dots(); err != nil {
			return err
		}
	}
	return t.PrintRight(int(int16(x1-0x32)), y, buf)
}

func optGroupCost(cx int) int { return cx - (cx+3)/4 }

// drawRaceOptionBackground is Draw_Race_Option_Background_ (0x59105): the
// page with every pick's name, dots and cost.
func (g *Game) drawRaceOptionBackground() error {
	e, o := g.e, &g.rsel.opt
	t := e.Text()
	c2 := font.Colors{0x91, 0x95, 0x95, 0x95, 0x95, 0x95, 0x95, 0x95}
	bg, err := g.cached(cStringAt(g.exe, raceOptLBXOpt), 4)
	if err != nil {
		return err
	}
	if err := e.Draw(0, 0, bg); err != nil {
		return err
	}
	title, err := g.cached(strings.ToUpper(cStringAt(g.exe, patch13Title)), 0)
	if err != nil {
		return err
	}
	if err := e.Draw(0x83, 0, title); err != nil {
		return err
	}
	head := func(col, y, cx int) error {
		t.SetStyleShadow(4, o.colorOpt, font.ShadowDown)
		t.SetOutlineColor(0x77, 0)
		return t.Print(int(int16(raceOptX[col]-0x16)), int(int16(y-0x16)), o.optString[cx])
	}
	box := func(col, y int) error {
		o.specialSeg.CurrentFrame = 0
		return e.Draw(int(int16(raceOptX[col]-0x14)), y, o.specialSeg)
	}
	row, col, cx := 0, 0, 0
	for cx < 0x24 {
		if err := head(col, raceOptY[row], cx); err != nil {
			return err
		}
		cx++
		for yoff := 0; yoff != 0x27; yoff += 0xd {
			y := raceOptY[row] + yoff
			if o.groupButton[cx] > 0 {
				if err := box(col, y); err != nil {
					return err
				}
			}
			t.SetStyleShadow(2, c2, font.ShadowDown)
			if err := g.optLine(col, y, cx, optGroupCost(cx), false); err != nil {
				return err
			}
			cx++
		}
		if row++; row >= 5 {
			row, col = 0, col+1
		}
	}
	if err := head(col, raceOptY[row], cx); err != nil {
		return err
	}
	cx++
	for yoff := 0; yoff != 0x34; yoff += 0xd {
		y := raceOptY[row] + yoff
		if err := box(col, y); err != nil {
			return err
		}
		t.SetStyleShadow(2, c2, font.ShadowDown)
		if err := g.optLine(col, y, cx, optGroupCost(cx), false); err != nil {
			return err
		}
		cx++
	}
	col++
	if err := head(col, raceOptY[0], cx); err != nil {
		return err
	}
	cx++
	y := raceOptY[0]
	for ; cx < 0x40; cx++ {
		if err := box(col, y); err != nil {
			return err
		}
		t.SetStyleShadow(2, c2, font.ShadowDown)
		if err := g.optLine(col, y, cx, cx-0xb, true); err != nil {
			return err
		}
		y += 0x10
	}
	t.SetStyleShadow(4, o.colorOpt, font.ShadowDown)
	return nil
}

// drawRacialOptionScreen is Draw_Racial_Option_Screen_ (0x596a5), also the
// auto function: the picks left and the score, then over the background
// every pick's box: chosen, or greyed out with its text in grey when it
// cannot be taken.
func (g *Game) drawRacialOptionScreen() error {
	e, o := g.e, &g.rsel.opt
	t := e.Text()
	c1 := font.Colors{0x89, 0x91, 0x91, 0x91, 0x91, 0x91, 0x91, 0x91}
	t.SetStyleShadow(3, o.colorOpt, font.ShadowDown)
	t.SetOutlineColor(0x77, 0)
	// Print_Integer_Right_ (0x121153) is itoa and Print_Right_.
	if err := t.PrintRight(0x12f, 0x1c1, itoa10(int32(o.picks))); err != nil {
		return err
	}
	score := int16(o.picks*10 + 100)
	if score < 0 {
		score = 0
	}
	if err := t.PrintRight(0x1b7, 0x1c1, itoa10(int32(score))); err != nil {
		return err
	}
	if err := t.PrintRight(0x1c5, 0x1c1, cStringAt(g.exe, raceOptPercent)); err != nil {
		return err
	}
	at := func(col int) int { return int(int16(raceOptX[col] - 0x14)) }
	selected := func(col, y int) error {
		o.specialSeg.CurrentFrame = 1
		return e.Draw(at(col), y, o.specialSeg)
	}
	off := func(col, y int) error { return e.Draw(at(col), y, o.offSeg) }
	text := func(col, y, cx, c int, chosen bool) error {
		colors := c1
		if chosen {
			colors = o.colorOpt
		}
		t.SetStyleShadow(2, colors, font.ShadowDown)
		return g.optLine(col, y, cx, c, false)
	}
	row, col, cx := 0, 0, 1
	for cx-1 < 0x24 {
		for l := 0; l < 3; l++ {
			y := raceOptY[row] + l*0xd
			grp := cx / 4
			b := o.button[grp]
			var di int16
			if b == 0 {
				di = g.cost(3*grp + l)
			} else {
				di = g.cost(3*grp+l) - g.cost(3*grp+int(b)-1)
			}
			chosen := int(b)-1 == l
			tooMany := g.cost(optGroupCost(cx))+o.negativePicks < -10
			var err error
			switch {
			case !(o.groupButton[cx] > 0 || chosen),
				o.picks-di < 0 && !chosen,
				tooMany && !chosen:
				err = off(col, y)
			case chosen:
				err = selected(col, y)
			}
			if err != nil {
				return err
			}
			blocked := cx >= 4 && cx < 8 && o.toggle[9] == 1
			if blocked {
				if err := off(col, y); err != nil {
					return err
				}
			}
			if chosen || tooMany || o.picks-di < 0 || blocked || o.groupButton[cx] == -1000 {
				if err := text(col, y, cx, optGroupCost(cx), chosen); err != nil {
					return err
				}
			}
			cx++
		}
		if row++; row >= 5 {
			row, col = 0, col+1
		}
		cx++
	}
	cx--
	for gl := int16(0); gl < 4; gl++ {
		cx++
		y := raceOptY[row] + int(gl)*0xd
		delta := g.cost(optGroupCost(cx)) - g.cost(int(o.button[9])+0x1b)
		chosen := o.button[9] == gl
		tooMany := g.cost(optGroupCost(cx))+o.negativePicks < -10
		var err error
		switch {
		case o.groupButton[cx] < 0, o.picks-delta < 0 && !chosen, tooMany && !chosen:
			err = off(col, y)
		case chosen:
			err = selected(col, y)
		}
		if err != nil {
			return err
		}
		if chosen || tooMany || o.picks-delta < 0 {
			if err := text(col, y, cx, optGroupCost(cx), chosen); err != nil {
				return err
			}
		}
	}
	col++
	cx += 2
	y := raceOptY[0]
	for k := 0; k < optToggles; k++ {
		on := o.toggle[k] == 1
		var c int16
		if on {
			if err := selected(col, y); err != nil {
				return err
			}
		} else {
			c = g.realAbilityCost(k)
			if o.optionButton[k] < 0 || o.picks-c < 0 || g.cost(cx-0xb)+o.negativePicks < -10 {
				if err := off(col, y); err != nil {
					return err
				}
			}
		}
		if on || g.cost(cx-0xb)+o.negativePicks < -10 || o.picks-c < 0 || o.optionButton[k] < 0 {
			if err := text(col, y, cx, cx-0xb, on); err != nil {
				return err
			}
		}
		y += 0x10
		cx++
	}
	t.SetStyleShadow(4, o.colorOpt, font.ShadowDown)
	return nil
}

// raceOptWarning is the warning box Racial_Option_Screen_ shows (Ken text
// n) and the redraw after it.
func (g *Game) raceOptWarning(n int) error {
	msg, err := g.kenText(n)
	if err != nil {
		return err
	}
	if err := g.warningBox(msg); err != nil {
		return err
	}
	return g.raceOptRedraw()
}

func (g *Game) raceOptRedraw() error {
	e := g.e
	e.V.SetPageBack()
	if err := g.drawRaceOptionBackground(); err != nil {
		return err
	}
	e.V.CopyBackToOff()
	e.V.SetPageOff()
	if err := g.drawRacialOptionScreen(); err != nil {
		return err
	}
	return e.TogglePages()
}

// racialOptionScreen is Racial_Option_Screen_ (0x5a3bc) for the player being set up: the
// custom race picks over the chosen base race, then its ruler name and
// banner; the race is saved to lastrace.rac. 1 when done, 0 on Esc.
func (g *Game) racialOptionScreen() (int16, error) {
	e, o := g.e, &g.rsel.opt
	pl := g.rsel.player
	o.nameButton = 0
	g.setRaceOptionHelpList()
	e.ClearFields()
	g.convertPlayerToFlags(pl)
	if err := g.reloadRacialOptionScreen(); err != nil {
		return 0, err
	}
	o.curRaceName = [0x12]byte{}
	copy(o.curRaceName[:], save.CString(pl.RaceName[:]))
	g.calculateRacePicks()
	e.AssignAutoFunction(g.drawRacialOptionScreen, 1)
	o.colorOpt = font.Colors{0x8e, 0x9d, 0x9c, 0x9d, 0x9d, 0x9d, 0x9d, 0x9d}
	e.Text().SetStyle(4, o.colorOpt)
	if err := g.addRaceOptionsFields(); err != nil {
		return 0, err
	}
	saved := o.button
	if err := g.addRaceOptionsFields(); err != nil {
		return 0, err
	}
	e.V.SetPageBack()
	if err := g.drawRaceOptionBackground(); err != nil {
		return 0, err
	}
	e.V.CopyBackToOff()
	e.V.SetPageOff()
	if err := g.drawRacialOptionScreen(); err != nil {
		return 0, err
	}
	if err := e.TogglePages(); err != nil {
		return 0, err
	}
	done := false
	for !done {
		g.fullDrawScreen = false
		r, err := e.GetInput()
		if err != nil {
			return 0, err
		}
		if r != 0 {
			if done, err = g.racialOptionInput(r, &saved); err != nil {
				return 0, err
			}
			if r == o.exitKen {
				e.ClearFields()
				e.DeactivateAutoFunction()
				return 0, nil
			}
			saved = o.button
			g.calculateRacePicks()
		}
		if g.fullDrawScreen {
			e.V.SetPageBack()
			if err := g.drawRaceOptionBackground(); err != nil {
				return 0, err
			}
			e.V.CopyBackToOff()
		}
		if !done {
			if err := g.addRaceOptionsFields(); err != nil {
				return 0, err
			}
			e.V.SetPageOff()
			if err := g.drawRacialOptionScreen(); err != nil {
				return 0, err
			}
			if err := e.TogglePages(); err != nil {
				return 0, err
			}
		}
	}
	e.DeactivateHelpList()
	e.DeactivateAutoFunction()
	return 1, nil
}

// racialOptionInput is the body of Racial_Option_Screen_'s loop for field r
// (0x5a4f7..0x5aa56); it reports whether the race was finished.
func (g *Game) racialOptionInput(r int, saved *[10]int16) (bool, error) {
	o := &g.rsel.opt
	for si := 0; si < 0x24; si++ {
		if r != o.groupButton[si] {
			continue
		}
		grp := si / 4
		if grp == 1 && o.toggle[9] == 1 {
			if err := g.raceOptWarning(0x49); err != nil {
				return false, err
			}
			o.button[grp] = 0
			continue
		}
		l := int16(si % 4)
		if saved[grp] == l {
			o.button[grp] = 0
			g.calculateRacePicks()
			continue
		}
		if g.cost(optGroupCost(si))+o.negativePicks < -10 {
			if err := g.raceOptWarning(0x4a); err != nil {
				return false, err
			}
			o.button[grp] = saved[grp]
			g.calculateRacePicks()
			continue
		}
		o.button[grp] = l
	}
	for si := 0x25; si < 0x29; si++ {
		if r != o.groupButton[si] {
			continue
		}
		g.calculateRacePicks()
		if int(o.button[9]) != si-0x25 && g.cost(si-0xa)+o.negativePicks < -10 {
			if err := g.raceOptWarning(0x4a); err != nil {
				return false, err
			}
			o.button[9] = saved[9]
			g.calculateRacePicks()
			continue
		}
		o.button[9] = int16(si - 0x25)
	}
	for si := 0; si < optToggles; si++ {
		if r != o.optionButton[si] {
			continue
		}
		o.toggle[si] = 1 - o.toggle[si]
		g.calculateRacePicks()
		if o.toggle[si] == 0 {
			continue
		}
		if o.negativePicks < -10 {
			if err := g.raceOptWarning(0x4a); err != nil {
				return false, err
			}
			o.toggle[si] = 0
		}
		switch si {
		case 5:
			o.toggle[6] = 0
		case 6:
			o.toggle[5] = 0
		case 0:
			o.toggle[1] = 0
		case 1:
			o.toggle[0] = 0
		case 9:
			o.toggle[8] = 0
			o.button[1] = 0
		case 8:
			o.toggle[9] = 0
		case 10:
			o.toggle[11] = 0
		case 11:
			o.toggle[10] = 0
		case 12:
			o.toggle[13] = 0
		case 13:
			o.toggle[12] = 0
		}
	}
	if r == o.defalt {
		for i := 0; i < 9; i++ {
			o.button[i] = 0
		}
		o.button[9] = 1
		o.toggle = [optToggles]int16{}
	}
	if r == o.exitKen {
		return false, nil
	}
	if r != o.accept {
		return false, nil
	}
	if o.picks < 0 {
		return false, g.raceOptWarning(0x4b)
	}
	return g.finishCustomRace()
}

// finishCustomRace is ACCEPT of Racial_Option_Screen_ (0x5a8cc): the picks
// into the player, the ruler name and banner over RACEOPT.LBX 4 and 6, and
// the race written to lastrace.rac.
func (g *Game) finishCustomRace() (bool, error) {
	e, o := g.e, &g.rsel.opt
	pl := g.rsel.player
	g.convertFlagsToPlayer(pl)
	e.AssignAutoFunction(func() error { return nil }, 2)
	name := cStringAt(g.exe, raceOptLBXOpt)
	back := func() error {
		e.V.SetPageBack()
		p, err := g.cached(name, 4)
		if err != nil {
			return err
		}
		if err := e.Draw(0, 0, p); err != nil {
			return err
		}
		p, err = g.cached(name, 6)
		if err != nil {
			return err
		}
		return e.Draw(0, 0x1ae, p)
	}
	if err := back(); err != nil {
		return false, err
	}
	e.V.SetPageOff()
	var res int16
	for {
		e.DeactivateHelpList()
		var err error
		if res, err = g.namingPopup(pl.EmperorName[:], int16(pl.Race)); err != nil {
			return false, err
		}
		if err := back(); err != nil {
			return false, err
		}
		e.V.CopyBackToOff()
		if err := e.TogglePages(); err != nil {
			return false, err
		}
		if res == 1 {
			f, err := g.flagScreen(&pl.Color)
			if err != nil {
				return false, err
			}
			res += f
		}
		if res != 1 {
			break
		}
	}
	if res != 2 {
		g.setRaceOptionHelpList()
		o.nameButton = 0
		e.AssignAutoFunction(g.drawRacialOptionScreen, 2)
		return false, nil
	}
	k := copy(pl.RaceName[:], save.CString(o.curRaceName[:]))
	if k < len(pl.RaceName) {
		pl.RaceName[k] = 0
	}
	var rec []byte
	for _, p := range pl.Picks {
		rec = append(rec, byte(p))
	}
	rec = append(rec, pl.Race)
	rec = append(rec, pl.RaceName[:]...)
	path := g.findFile(cStringAt(g.exe, lastRaceWrite))
	// A scratch game directory links the original's files: never write
	// through such a link.
	if fi, err := os.Lstat(path); err == nil && fi.Mode()&os.ModeSymlink != 0 {
		os.Remove(path)
	}
	if err := os.WriteFile(path, rec, 0o644); err != nil {
		g.Log("lastrace.rac: %v", err)
	}
	g.currentScreen = g.returnScreen
	return true, e.FastFadeOut()
}
