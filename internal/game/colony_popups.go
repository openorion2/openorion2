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

// Popups of the colony screen: transport (COLXPORT.C), troops (COLGCBT.C),
// the explanations of the production rows (COLCALC.C) and the cheats.
// Spec: docs/screens/colony-screen.md, "Popups".

// COLXPORT.C, the transport popup: the player's colonies, six at a time
// over the colony screen; the row picked is where the cluster goes.
const (
	colpupsXportPopup = 0x10 // COLPUPS.LBX, the popup
	xportRows         = 6
	xportRowY         = 0x36
	xportRowH         = 0x26
)

// xportState holds COLXPORT.C's globals. The row fields keep their numbers
// when a row is empty, as the original's do.
type xportState struct {
	list                     [250]int16 // Xport_Popup_'s local list
	listCol                  [xportRows]int16
	popRef                   [xportRows][42]int16
	popCount, coord          [xportRows]int16
	popF                     [xportRows]int
	rowF                     [xportRows]int
	scroll, cancel, up, down int
	popup                    *gfx.Picture // _xport_popup_seg
}

// xportPopup is Xport_Popup_ (0xc5934): the colony the cluster goes to, or
// -1 for RETURN.
func (g *Game) xportPopup() (int16, error) {
	e, xp := g.e, &g.xp
	g.col.fullDraw = true
	g.buildGlobalColonyList(&xp.list)
	e.Text().SetStyleShadow(1, g.colors(colonyFontColors), font.ShadowDown)
	saved := e.GetAutoFunction()
	e.DeactivateAutoFunction()
	draw := func() error { return g.drawXportPopup(saved) }
	e.AssignAutoFunction(draw, 1)
	g.setTransportPopupHelpList()
	if err := draw(); err != nil {
		return -1, err
	}
	e.V.SetRefreshStencil()
	e.ClearFields()
	if err := g.xportAddFields(); err != nil {
		return -1, err
	}
	if err := e.TogglePages(); err != nil {
		return -1, err
	}
	result := int16(-1)
	for {
		in, err := e.GetInput()
		if err != nil {
			return -1, err
		}
		switch {
		case in == xp.up:
			g.incrementFirst(xportRows)
		case in == xp.down && xp.list[int(g.csStat.first)+xportRows] != -1:
			g.decrementFirst(xportRows)
		default:
			for i := 0; i < xportRows; i++ {
				if in == xp.rowF[i] {
					result = xp.listCol[i]
				}
			}
		}
		if g.fullDrawScreen {
			g.col.fullDraw = true
			g.fullDrawScreen = false
			g.setTransportPopupHelpList()
		}
		if err := draw(); err != nil {
			return -1, err
		}
		e.ClearFields()
		if err := g.xportAddFields(); err != nil {
			return -1, err
		}
		if err := e.TogglePages(); err != nil {
			return -1, err
		}
		if in == xp.cancel || result != -1 {
			break
		}
	}
	e.ClearFields()
	e.DeactivateHelpList()
	e.DeactivateAutoFunction()
	e.PutAutoFunction(saved)
	return result, nil
}

// decrementFirst is Decrement_First_ (0xc4302): one row down.
func (g *Game) decrementFirst(n int16) {
	st := &g.csStat
	cnt := int16(g.nColonies(int(g.state.PlayerNum)))
	if n <= cnt {
		st.first++
		st.slider = cnt - n - st.first
		g.col.fullDraw = true
	}
}

// xportAddFields is COLXPORT.C's Add_Fields_ (0xc534e): the scroll bar
// from six colonies on, the arrows, RETURN and a field per filled row.
func (g *Game) xportAddFields() error {
	e, xp := g.e, &g.xp
	a := &g.cscr.xportAnims
	n := g.nColonies(int(g.state.PlayerNum))
	var err error
	if n < xportRows {
		xp.scroll = engine.NoField
	} else if xp.scroll, err = e.AddScrollField(0x1c1, 0x4b, 0, n, 0, n-xportRows, 0xe, 0xa4, &g.csStat.slider, g.eStr(0xc), 0); err != nil {
		return err
	}
	up, down, ret := g.eStr(0xc), g.eStr(0xc), g.eStr(0xc)
	if xp.up, err = e.AddButtonField(0x1c0, 0x34, g.eStr(0xc), a[1], up, 0x28); err != nil {
		return err
	}
	if xp.down, err = e.AddButtonField(0x1bf, 0xf3, g.eStr(0xc), a[2], down, 0x28); err != nil {
		return err
	}
	if xp.cancel, err = e.AddButtonField(0x160, 0x11d, g.eStr(0xc), a[0], ret, 0x28); err != nil {
		return err
	}
	y1, y2 := 0x34, 0x53
	for i := 0; i < xportRows; i++ {
		if xp.listCol[i] != -1 {
			if xp.rowF[i], err = e.AddHiddenField(0xf, y1, 0x1b9, y2, g.eStr(0xc), 0); err != nil {
				return err
			}
		}
		y1 += xportRowH
		y2 += xportRowH
	}
	return nil
}

// drawXportPopup is Draw_Xport_Popup_ (0xc5800): on a full draw the colony
// screen under the popup, then the six rows, the scroll bar and the cluster.
func (g *Game) drawXportPopup(under engine.AutoFunction) error {
	e, xp, cg := g.e, &g.xp, &g.col
	g.setColonyFont()
	if cg.fullDraw {
		cg.dontDrawCluster = true
		if err := under.Run(); err != nil {
			return err
		}
		cg.dontDrawCluster = false
		cg.fullDraw = true
		pic, err := g.picture(cStringAt(g.exe, colpupsLBX), colpupsXportPopup)
		if err != nil {
			return err
		}
		xp.popup = pic
		if err := e.Draw(0, 0, pic); err != nil {
			return err
		}
		e.V.CopyOffToBack()
		cg.fullDraw = false
	}
	for i := range xp.listCol {
		xp.listCol[i] = -1
	}
	g.updateFirst(xportRows)
	for i := 0; i < xportRows; i++ {
		if c := xp.list[int(g.csStat.first)+i]; c != -1 {
			if err := g.drawXportColony(int(c), i); err != nil {
				return err
			}
		}
	}
	g.drawXportBarIndicator()
	px, py := e.Pointer()
	return g.drawCluster(px, py)
}

// drawXportColony is Draw_Colony_Summary_For_Colony_ (0xc5426): row i's
// name, red when blockaded or plagued, and its colonists.
func (g *Game) drawXportColony(ci, i int) error {
	s, xp := &g.state, &g.xp
	xp.listCol[i] = int16(ci)
	c := &s.Colonies[ci]
	if s.Stars[s.Planets[c.Planet].Star].Blockaded>>uint(s.PlayerNum)&1 != 0 || s.eventCheckPlague(ci) {
		g.setColonyFontToRed(4, 0)
	} else {
		g.setColonyFontTo(4)
	}
	y := xportRowY + i*xportRowH
	if err := g.e.Text().SqueezePrintParagraph(0xf, y, 0x82, 0x1c, g.planetName(int(c.Planet)), 0, g.exe.bytes(spaceLimits, 6), false); err != nil {
		return err
	}
	_, err := g.doColonyInfoPopStuffForPop(ci, -1, 0, 0x94, int16(y), 0x1b9, false,
		xp.popRef[i][:], &xp.popCount[i], &xp.coord[i], &xp.popF[i])
	return err
}

// drawXportBarIndicator is COLXPORT.C's Draw_Bar_Indicator_ (0xc56bb).
func (g *Game) drawXportBarIndicator() {
	v, st := g.e.V, &g.csStat
	n := int(int16(g.nColonies(int(g.state.PlayerNum))))
	g.updateFirst(xportRows)
	if n < xportRows {
		v.Fill(0x1c1, 0x4b, 0x1c9, 0xef, 0x50)
		return
	}
	y1 := int(int16(int(st.first)*0xa4/n + 0x4b))
	y2 := int(int16((int(st.first)+xportRows)*0xa4/n + 0x4b))
	v.Fill(0x1c1, y1, 0x1c9, y2, 0xe5)
	v.Line(0x1c1, y1, 0x1c9, y1, 0xe6)
	v.Line(0x1c1, y1, 0x1c1, y2, 0xe6)
	v.Line(0x1c9, y1, 0x1c9, y2, 0xe4)
	v.Line(0x1c1, y2, 0x1c9, y2, 0xe4)
	v.Dot(0x1c1, 0x4b, 0x50)
	v.Dot(0x1c9, 0x4b, 0x50)
	v.Dot(0x1c1, 0xef, 0x50)
	v.Dot(0x1c9, 0xef, 0x50)
}

// COLGCBT.C, the troop popup.
const (
	colgcbtLBX          = 0x1829fd // "COLGCBT.LBX"
	colgcbtPopup        = 0x15     // its picture
	transportHelpList   = 0x18343e // _transport_popup_help_list, 7 entries
	colonyFontColors    = 0x182c2e
	printDataFieldFmt   = 0x179f43 // "\x1a0"
	printDataValueFmt   = 0x179f40 // "\x1a1"
	techAppNameBattle9  = 9        // _technology_applications[9]'s name, Print_Best_Summary_'s first line
	troopRowY           = 0x153
	troopRowH           = 0xb
	troopPopupX         = 2
	troopPopupY         = 0x149
	troopPopupDarkY     = 0x14a
	troopPopupDarkY2    = 0x1d9
	troopTotalsStrength = 0x1d1
	troopTotalsHits     = 0x1c8
)

// setTransportPopupHelpList is Set_Transport_Popup_Help_List_ (0xcdb4e).
func (g *Game) setTransportPopupHelpList() {
	g.e.DeactivateHelpList()
	g.e.SetHelpList(g.helpListAt(transportHelpList, 7))
}

// troopPopup is Troop_Popup_ (0xcf1e0): the colony's defenders, until any
// input.
func (g *Game) troopPopup() error {
	e := g.e
	g.col.fullDraw = true
	e.Text().SetStyleShadow(1, g.colors(colonyFontColors), font.ShadowDown)
	saved := e.GetAutoFunction()
	e.DeactivateAutoFunction()
	draw := func() error { return g.drawTroopPopup(saved) }
	e.AssignAutoFunction(draw, 1)
	g.setTransportPopupHelpList()
	if err := draw(); err != nil {
		return err
	}
	e.V.SetRefreshStencil()
	add := func() error {
		e.ClearFields()
		_, err := e.AddHiddenField(0, 0, 0x27f, 0x1df, "", 0)
		return err
	}
	if err := add(); err != nil {
		return err
	}
	if err := e.TogglePages(); err != nil {
		return err
	}
	for {
		in, err := e.GetInput()
		if err != nil {
			return err
		}
		if err := draw(); err != nil {
			return err
		}
		if err := add(); err != nil {
			return err
		}
		if err := e.TogglePages(); err != nil {
			return err
		}
		if in != 0 {
			break
		}
	}
	e.ClearFields()
	e.DeactivateHelpList()
	e.DeactivateAutoFunction()
	e.PutAutoFunction(saved)
	return nil
}

// drawTroopPopup is Draw_Troop_Popup_ (0xcf0d9): once per full draw, the
// colony screen under a darkened box with the bonuses and the three troop
// columns.
func (g *Game) drawTroopPopup(under engine.AutoFunction) error {
	e, s := g.e, &g.state
	g.setColonyFont()
	if !g.col.fullDraw {
		return nil
	}
	if err := under.Run(); err != nil {
		return err
	}
	pic, err := g.picture(cStringAt(g.exe, colgcbtLBX), colgcbtPopup)
	if err != nil {
		return err
	}
	e.TintFill(3, troopPopupDarkY, int(int16(pic.Width))+1, troopPopupDarkY2, 0)
	if err := e.Draw(troopPopupX, troopPopupY, pic); err != nil {
		return err
	}
	cs := &g.cscr
	pl := s.Stars[cs.starHandle].Planets[cs.orbitN]
	var F groundForce
	g.computeColonyGroundCombatInfo(int(s.Planets[pl].Colony), &F)
	line := 0
	if err := g.printBestSummary(&F, &line, pic); err != nil {
		return err
	}
	if err := g.printRaceSummary(&F, &line, pic); err != nil {
		return err
	}
	line++
	if err := g.printTroopTotals(troopPopupX, &F, pic); err != nil {
		return err
	}
	e.V.CopyOffToBack()
	g.col.fullDraw = false
	return nil
}

// computeColonyGroundCombatInfo is Compute_Colony_Ground_Combat_Info_
// (0xed713): the colony's armor, marines and militia with its owner's
// bonuses and governor, no enemy.
func (g *Game) computeColonyGroundCombatInfo(ci int, F *groundForce) {
	c := &g.state.Colonies[ci]
	*F = groundForce{player: int16(c.Owner), colony: int16(ci), enemy: -1}
	F.count[0] = c.Armor
	F.count[1] = c.Marines
	F.count[2] = int16(colonyNMilitia(c))
	g.computeColonyOfficerBonus(c, &F.bonus)
	g.computePlayerGroundCombatBonuses(int(F.player), &F.bonus, true)
	computeGroundCombatInfo(F, &F.bonus, nil)
}

// printData is Print_Data_ (0xcec99): "name value" on line *line.
func (g *Game) printData(name string, v int16, line *int, pic *gfx.Picture) error {
	t := cSprintf(g.eStr(0x4b), cStringAt(g.exe, printDataFieldFmt), name, cStringAt(g.exe, printDataValueFmt), int(v))
	g.setColonyFontTo(2)
	y := int(int16(*line*troopRowH + troopRowY))
	err := g.printFormattedParagraph(0xb, y, int(int16(pic.Width))-0x12, t, 0)
	*line++
	return err
}

// printBestSummary is Print_Best_Summary_ (0xced16): the best ground
// weapons and armour the bonuses came from.
func (g *Game) printBestSummary(F *groundForce, line *int, pic *gfx.Picture) error {
	return g.bestSummary(F, func(name string, v int16) error { return g.printData(name, v, line, pic) })
}

// bestSummary is the body of both Print_Best_Summary_ (0xced16, 0xb8dbe),
// which differ only in their Print_Data_.
func (g *Game) bestSummary(F *groundForce, pd func(name string, v int16) error) error {
	b := &F.bonus
	app := func(a int) string {
		n, err := g.techAppName(a)
		if err != nil {
			g.logOnce(err.Error())
		}
		return n
	}
	if b[0] > 0 {
		if err := pd(app(techAppNameBattle9), int16(b[0])); err != nil {
			return err
		}
	}
	for _, k := range []int{5, 7, 9} {
		if b[k] > 0 {
			if err := pd(app(int(b[k])), int16(b[k+1])); err != nil {
				return err
			}
		}
	}
	return nil
}

// printRaceSummary is Print_Race_Summary_ (0xced9b): race, gravity,
// Subterranean, AI and governor bonuses.
func (g *Game) printRaceSummary(F *groundForce, line *int, pic *gfx.Picture) error {
	return g.raceSummary(F, func(name string, v int16) error { return g.printData(name, v, line, pic) })
}

// raceSummary is the body of both Print_Race_Summary_ (0xced9b, 0xb8e43).
func (g *Game) raceSummary(F *groundForce, pd func(name string, v int16) error) error {
	b := &F.bonus
	type row struct {
		ok   bool
		name string
		v    int16
	}
	rows := []row{
		{int8(b[0xb]) != 0, g.eStr(0x1b5), int16(int8(b[0xb]))},
		{int8(b[0xd]) != 0, g.eStr(0x16c), int16(int8(b[0xd]))},
		{b[0xe] > 0, g.eStr(0x1f1), int16(b[0xe])},
		// Check_Release_Version_ is 1 in this build.
		{int8(b[0xf]) > 0, g.eStr(0x7b), int16(int8(b[0xf]))},
	}
	for _, r := range rows {
		if r.ok {
			if err := pd(r.name, r.v); err != nil {
				return err
			}
		}
	}
	if l := int8(b[0x12]); l > 0 {
		return pd(g.leaderName(int(l), false), b.officer())
	}
	return nil
}

// printTroopTotals is Print_Troop_Totals_ (0xcee50): for armor, marines and
// militia a column with strength, hits, count and picture. Its last
// sprintf (E 0x7c) is never printed.
func (g *Game) printTroopTotals(x int, F *groundForce, pic *gfx.Picture) error {
	s := &g.state
	t := g.e.Text()
	const cols = 3
	race := int(s.Players[F.player].Race)
	for k := 0; k < cols; k++ {
		anim, err := g.playerTroopAnim(int(F.player), k, race)
		if err != nil {
			return err
		}
		g.setColonyFontToPlayer(1, int(F.player))
		cx := int(int16(int32(int16(pic.Width))/(cols+1)*int32(k+1) + int32(x)))
		str := cSprintf(g.eStr(0xd), int(F.strength[k])-int(int8(F.bonus[0xf])))
		if err := t.PrintCentered(cx, troopTotalsStrength, str); err != nil {
			return err
		}
		f := 0x16
		if F.hits[k] > 1 {
			f = 0x17
		}
		if err := t.PrintCentered(cx, troopTotalsHits, cSprintf(g.eStr(f), int(F.hits[k]))); err != nil {
			return err
		}
		str = cSprintf(g.eStr(0xe), int(F.count[k]), g.playerTroopName(int(F.player), k))
		if err := t.PrintCentered(cx, 0x1da-3*9, str); err != nil {
			return err
		}
		if anim != nil {
			if err := g.drawBottomCentered(cx, 0x1da-4*9+8, anim); err != nil {
				return err
			}
		}
	}
	return nil
}

// Colcalc strings: the first 60 of RSTRING<lang>.LBX, in Load_Colcalc_
// Strings_' (0xdd4fd) order.
const (
	ccAnomaly = iota
	ccNoBarracks
	ccUnification
	ccGovMorale
	ccNoCapital
	ccMixedRaces
	ccBonus
	ccLeaderBonus
	ccTotalMorale
	ccMoraleSummary
	ccByFarmers
	ccAquaticBonus
	ccNativeBonus
	ccAndroidBonus
	ccBase
	ccGravityPenalty
	ccConqueredPenalty
	ccBlockaded
	ccMoraleBonus
	ccMoralePenalty
	ccGovBonus
	ccGovPenalty
	ccTotalFood
	ccFoodSummary
	ccByWorkers
	ccUsed
	ccPollution
	ccReplicated
	ccTaxed
	ccTotalIndustry
	ccIndustrySummary
	ccByScientists
	ccArtifactsBonus
	ccIntelligence
	ccTotalResearch
	ccResearchSummary
	ccTaxesCollected
	ccSpecialIncome
	ccTradeGoods
	ccTaxedFromIndustry
	ccTotalIncome
	ccBCSummary
)

// Formats of the explanations in .data.
const (
	explTabs      = 0x179f60 // "\x07T+15,+65." the tab stops
	explTabsAlt   = 0x179f6e // "\x1a0"
	explGovLine   = 0x179f71 // "\t%+d%%\t%s%s\r"
	explPctLine   = 0x179f7e // "\t%+d%%\t%s\r"
	explPctNoSign = 0x179f89 // "\t%d%%\t%s\r"
	explCR        = 0x179f93 // "\r"
)

// colcalc returns Colcalc string i.
func (g *Game) colcalc(i int) (string, error) {
	rs, err := g.rStrings()
	if err != nil {
		return "", err
	}
	if i >= len(rs) {
		return "", fmt.Errorf("rstring: no Colcalc string %d", i)
	}
	return rs[i], nil
}

// showMorale is Show_Morale_ (0xddefb): Colony_Morale_ with its
// explanation in a text box.
func (g *Game) showMorale(ci int) error {
	c := &g.state.Colonies[ci]
	m := g.colonyMorale(c)
	text, err := g.colonyMoraleText(c, m)
	if err != nil {
		return err
	}
	title, err := g.colcalc(ccMoraleSummary)
	if err != nil {
		return err
	}
	return g.textBox(&title, text)
}

// colonyMoraleText is the explanation Colony_Morale_ writes when given a
// buffer: the government's morale, then each other term in percent.
func (g *Game) colonyMoraleText(c *save.Colony, m moraleParts) (string, error) {
	cc := func(i int) string {
		v, err := g.colcalc(i)
		if err != nil {
			g.logOnce(err.Error())
		}
		return v
	}
	if m.half == 3 {
		return cc(ccUnification), nil
	}
	str := func(a uint32) string { return cStringAt(g.exe, a) }
	var b strings.Builder
	b.WriteString(str(explTabs))
	b.WriteString(str(explTabsAlt))
	suffix := ""
	if m.noBarracks {
		suffix = cc(ccNoBarracks)
	}
	gov := int32(int8(g.exeU8(uint32(tblGovMorale + m.gov))))
	b.WriteString(cSprintf(str(explGovLine), gov*5, cc(ccGovMorale), suffix))
	line := func(v int32, what string) {
		if v != 0 {
			b.WriteString(cSprintf(str(explPctLine), v*5, what))
		}
	}
	bname := func(id int) string {
		n, err := g.buildingName(id)
		if err != nil {
			g.logOnce(err.Error())
		}
		return n
	}
	aname := func(app int) string {
		n, err := g.techAppName(app)
		if err != nil {
			g.logOnce(err.Error())
		}
		return n
	}
	line(m.noCapital, cc(ccNoCapital))
	if m.mixed != 0 {
		b.WriteString(cSprintf(str(explPctNoSign), m.mixed*5, cc(ccMixedRaces)))
	}
	line(m.guard, bname(2))
	line(m.bar, bname(0x16))
	line(m.police, aname(0x93))
	line(m.b14, bname(0x14))
	line(m.app, aname(0xc3))
	line(m.b1f, bname(0x1f))
	line(m.lead, cc(ccLeaderBonus))
	b.WriteString(str(explCR))
	b.WriteString(cSprintf(str(explPctLine), int32(c.Morale)*5, cc(ccTotalMorale)))
	return b.String(), nil
}

// More formats of the explanations.
const (
	explPlus      = 0x179f95 // "\t%+d\t%s\r"
	explPlusRace  = 0x179f9e // "\t%+d\t%s %s\r"
	explQuotRace  = 0x179faa // "\t%s\t%s %s\r"
	explPlusSpace = 0x179fb5 // "\t%+d\t %s\r"
	explQuot      = 0x179fbf // "\t%s\t%s\r"
	explRepl      = 0x179fc7 // "\t%d\t%s\r"
	explMinus     = 0x179fcf // "\t-%d\t%s\r"
	explPlusBldg  = 0x179fd8 // "\t+%d\t%s\r"
	quotIntFmt    = 0x179f58 // "%d"
	quotFracFmt   = 0x179f5b // ".%d"
)

// formatQuotient is Format_Quotient_ (0xdda36): v/d signed, with up to
// five decimals when they come out even (".%d" drops leading zeros).
func (g *Game) formatQuotient(v, d int32) string {
	sign := "+"
	if v < 0 {
		sign, v = "-", -v
	}
	out := sign + cSprintf(cStringAt(g.exe, quotIntFmt), v/d)
	r := v % d
	if r == 0 {
		return out
	}
	m := int32(10)
	for k := 1; ; k++ {
		if x := m * r; x/d*d == x || k >= 5 {
			break
		}
		m *= 10
	}
	return out + cSprintf(cStringAt(g.exe, quotFracFmt), m*r/d)
}

// explanation writes the production popups' common parts.
type explanation struct {
	g *Game
	b strings.Builder
}

func (g *Game) newExplanation() *explanation {
	x := &explanation{g: g}
	x.b.WriteString(cStringAt(g.exe, explTabs))
	x.b.WriteString(cStringAt(g.exe, explTabsAlt))
	return x
}

func (x *explanation) f(format uint32, args ...any) {
	x.b.WriteString(cSprintf(cStringAt(x.g.exe, format), args...))
}

func (x *explanation) cc(i int) string {
	v, err := x.g.colcalc(i)
	if err != nil {
		x.g.logOnce(err.Error())
	}
	return v
}

func (x *explanation) bname(id int) string {
	n, err := x.g.buildingName(id)
	if err != nil {
		x.g.logOnce(err.Error())
	}
	return n
}

func (x *explanation) aname(app int) string {
	n, err := x.g.techAppName(app)
	if err != nil {
		x.g.logOnce(err.Error())
	}
	return n
}

// breakdown lists the per-race, android, application and building terms;
// food's are in half units (div 2) and shows natives and negative races.
func (x *explanation) breakdown(bd *prodBreakdown, food bool) {
	s := &x.g.state
	div := int32(1)
	if food {
		div = 2
	}
	for r := 7; r >= 0; r-- {
		v := int32(bd.race[r])
		name := save.CString(s.Players[r].RaceName[:])
		switch {
		case v > 0, !food && v != 0:
			x.f(explPlusRace, v/div, name, x.cc(ccBonus))
		case v < 0:
			x.f(explQuotRace, x.g.formatQuotient(v, 2), name, x.cc(ccBonus))
		}
	}
	if food && bd.race[9] != 0 {
		x.f(explPlusSpace, int32(bd.race[9])/div, x.cc(ccNativeBonus))
	}
	if bd.race[8] != 0 {
		x.f(explPlus, int32(bd.race[8])/div, x.cc(ccAndroidBonus))
	}
	for a := 0xcb; a >= 0; a-- {
		if v := int32(bd.apps[a]); v != 0 {
			x.f(explPlus, v/div, x.aname(a))
		}
	}
	for b := 0x30; b >= 0; b-- {
		if v := int32(bd.bldgs[b]); v != 0 {
			x.f(explPlus, v/div, x.bname(b))
		}
	}
}

// percents lists gravity, conquest, blockade, morale, government and
// leader terms, divided by d.
func (x *explanation) percents(bd *prodBreakdown, d int32) {
	q := func(v int32, msg int) {
		x.f(explQuot, x.g.formatQuotient(v, d), x.cc(msg))
	}
	if bd.grav != 0 {
		q(bd.grav, ccGravityPenalty)
	}
	if bd.conq != 0 {
		q(bd.conq, ccConqueredPenalty)
	}
	if bd.block != 0 {
		q(bd.block, ccBlockaded)
	}
	switch {
	case bd.morale > 0:
		q(bd.morale, ccMoraleBonus)
	case bd.morale < 0:
		q(bd.morale, ccMoralePenalty)
	}
	switch {
	case bd.gov > 0:
		q(bd.gov, ccGovBonus)
	case bd.gov < 0:
		q(bd.gov, ccGovPenalty)
	}
	if bd.lead != 0 {
		q(bd.lead, ccLeaderBonus)
	}
}

// showProduction shows text under title Colcalc string summary.
func (g *Game) showProduction(text string, summary int) error {
	title, err := g.colcalc(summary)
	if err != nil {
		return err
	}
	return g.textBox(&title, text)
}

// showFoodProduction is Show_Food_Production_ (0xdeb1d): Colony_Food_
// Production_ with its explanation, which also adds the replicated food to
// the colony's food.
func (g *Game) showFoodProduction(ci int) error {
	c := &g.state.Colonies[ci]
	var bd prodBreakdown
	t := g.colonyFoodProductionB(ci, &bd)
	if c.PopCount == 0 {
		g.logOnce("Show_*_Production_ of an empty colony shows an unset buffer")
		return g.showProduction("", ccFoodSummary)
	}
	x := g.newExplanation()
	if t.anomaly {
		return g.showProduction(x.cc(ccAnomaly), ccFoodSummary)
	}
	if bd.base != 0 {
		x.f(explPlus, bd.base/2, x.cc(ccByFarmers))
	}
	if bd.aquatic != 0 {
		x.f(explPlus, bd.aquatic/2, x.cc(ccAquaticBonus))
	}
	x.breakdown(&bd, true)
	x.percents(&bd, 0x28)
	if t.b15 != 0 {
		x.f(explPlus, t.b15, x.bname(0x15))
	}
	if t.b2b != 0 {
		x.f(explPlus, t.b2b, x.bname(0x2b))
	}
	if r := c.ReplicatedFood; r != 0 {
		x.f(explPlus, int32(r), x.bname(0x10))
		c.Output[food] += int16(r)
	}
	x.f(explCR)
	x.f(explPlus, int32(c.Output[food]), x.cc(ccTotalFood))
	return g.showProduction(x.b.String(), ccFoodSummary)
}

// showIndustryProduction is Show_Industry_Production_ (0xdf518): Colony_
// Industry_Production_ with its explanation; the replicated food and the
// taxed industry come off the colony's industry.
func (g *Game) showIndustryProduction(ci int) error {
	c := &g.state.Colonies[ci]
	var bd prodBreakdown
	t := g.colonyIndustryProductionB(ci, &bd)
	if c.PopCount == 0 {
		g.logOnce("Show_*_Production_ of an empty colony shows an unset buffer")
		return g.showProduction("", ccIndustrySummary)
	}
	x := g.newExplanation()
	if t.anomaly {
		return g.showProduction(x.cc(ccAnomaly), ccIndustrySummary)
	}
	if bd.base != 0 {
		x.f(explPlus, bd.base, x.cc(ccByWorkers))
	}
	x.breakdown(&bd, false)
	x.percents(&bd, 0x14)
	for _, b := range []struct {
		v  int32
		id int
	}{{t.recy, 0x21}, {t.b7, 7}, {t.b24, 0x24}, {t.bc, 0xc}} {
		if b.v != 0 {
			x.f(explPlus, b.v, x.bname(b.id))
		}
	}
	if t.deep != 0 {
		x.f(explPlusRace, t.deep, x.bname(0x22), x.cc(ccUsed))
	}
	if bd.pollute != 0 {
		x.f(explQuot, g.formatQuotient(bd.pollute, 0x14), x.cc(ccPollution))
	}
	if r := c.ReplicatedFood; r != 0 {
		tmp := cSprintf(x.cc(ccReplicated), int32(r))
		x.f(explRepl, -2*int32(r), tmp)
		c.Output[industry] -= 2 * int16(r)
	}
	if c.IndustryTaxed != 0 {
		x.f(explMinus, int32(c.IndustryTaxed), x.cc(ccTaxedFromIndustry))
		c.Output[industry] -= c.IndustryTaxed
	}
	x.f(explCR)
	x.f(explPlus, int32(c.Output[industry]), x.cc(ccTotalIndustry))
	return g.showProduction(x.b.String(), ccIndustrySummary)
}

// showResearchProduction is Show_Research_Production_ (0xe03c3).
func (g *Game) showResearchProduction(ci int) error {
	c := &g.state.Colonies[ci]
	var bd prodBreakdown
	t := g.colonyResearchProductionB(ci, &bd)
	if c.PopCount == 0 {
		g.logOnce("Show_*_Production_ of an empty colony shows an unset buffer")
		return g.showProduction("", ccResearchSummary)
	}
	x := g.newExplanation()
	if t.anomaly {
		return g.showProduction(x.cc(ccAnomaly), ccResearchSummary)
	}
	if bd.base != 0 {
		x.f(explPlus, bd.base, x.cc(ccByScientists))
	}
	if bd.special != 0 {
		x.f(explPlus, bd.special, x.cc(ccArtifactsBonus))
	}
	if bd.intel != 0 {
		x.f(explPlus, bd.intel, x.cc(ccIntelligence))
	}
	x.breakdown(&bd, false)
	x.percents(&bd, 0x14)
	for _, b := range []struct {
		v  int32
		id int
	}{{t.b23, 0x23}, {t.b1e, 0x1e}, {t.b13, 0x13}, {t.b6, 6}} {
		if b.v != 0 {
			x.f(explPlusBldg, b.v, x.bname(b.id))
		}
	}
	x.f(explCR)
	x.f(explPlus, int32(c.Output[research]), x.cc(ccTotalResearch))
	return g.showProduction(x.b.String(), ccResearchSummary)
}

// showBCProduction is Show_BC_Production_ (0xe08c8).
func (g *Game) showBCProduction(ci int) error {
	c := &g.state.Colonies[ci]
	t := g.colonyBCProductionT(ci)
	if c.PopCount == 0 {
		g.logOnce("Show_*_Production_ of an empty colony shows an unset buffer")
		return g.showProduction("", ccBCSummary)
	}
	x := g.newExplanation()
	if t.anomaly {
		return g.showProduction(x.cc(ccAnomaly), ccBCSummary)
	}
	line := func(v int32, what string) {
		if v != 0 {
			x.f(explPlus, v, what)
		}
	}
	line(t.base, x.cc(ccTaxesCollected))
	switch {
	case t.morale > 0:
		x.f(explPlus, t.morale, x.cc(ccMoraleBonus))
	case t.morale < 0:
		x.f(explPlus, t.morale, x.cc(ccMoralePenalty))
	}
	line(t.special, x.cc(ccSpecialIncome))
	line(t.lead, x.cc(ccLeaderBonus))
	line(t.app, x.bname(0x12))
	line(t.b1d, x.bname(0x1d))
	line(t.b27, x.bname(0x27))
	line(t.gov, x.cc(ccGovBonus))
	line(t.trade, x.cc(ccTradeGoods))
	line(int32(c.IndustryTaxed), x.cc(ccTaxedFromIndustry))
	x.f(explCR)
	x.f(explPlus, int32(c.Output[bc]), x.cc(ccTotalIncome))
	return g.showProduction(x.b.String(), ccBCSummary)
}

// doColonyCheats is Do_Cheats_ (0xc0b87): after "c" with cheats on, one
// key read with getch_ (its PFMT_ menu prints nothing in this build): P and
// Q add 100 and 10000 production, C colonizes the planet shown, F adds five
// freighters, B 100 BC; P and any other key recompute the colony.
func (g *Game) doColonyCheats() error {
	s := &g.state
	k, err := g.getch()
	if err != nil {
		return err
	}
	key := string([]byte{k})
	is := func(n int) bool { return strings.EqualFold(key, g.eStr(n)) }
	ci := g.colonyHandle
	if is(0x197) {
		s.Colonies[ci].Production += 100
		g.col.fullDraw = true
	}
	switch {
	case is(0x1b2):
		s.Colonies[ci].Production += 10000
		g.col.fullDraw = true
	case is(0xd6):
		if g.cscr.display != 2 || g.colonizePlanet(int(g.cscr.planetDisp)) {
			return nil
		}
		if err := g.messageBox(g.eStr(0x1a0)); err != nil {
			return err
		}
		g.restoreColonyControl()
	case is(0x120):
		s.Players[s.PlayerNum].FreightersTotal += 5
	case is(0xc0):
		s.Players[s.PlayerNum].BC += 100
	default:
		g.colCalcWrapper(ci)
	}
	return nil
}

// colonizePlanet is Colonize_Planet_ (0xbb082): a colony of the player on
// an empty planet.
func (g *Game) colonizePlanet(pl int) bool {
	s := &g.state
	if s.Planets[pl].Colony != -1 {
		return false
	}
	ci := int(s.NumColonies)
	s.NumColonies++
	g.initColony(ci, pl, int(s.PlayerNum))
	return true
}

// getch is getch_ of the C library: wait for a key and return its
// character, the screen untouched.
func (g *Game) getch() (byte, error) {
	sys := g.e.Sys
	for {
		if k, ok := sys.PopKey(); ok {
			return byte(k.Code), nil
		}
		if err := sys.Sleep(5 * time.Millisecond); err != nil {
			return 0, err
		}
	}
}
