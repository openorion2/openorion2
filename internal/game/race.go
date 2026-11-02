package game

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/openorion2/openorion2/internal/font"
	"github.com/openorion2/openorion2/internal/gfx"
	"github.com/openorion2/openorion2/internal/save"
)

// The races screen (screen 6, the RACES button): MOX\RACESCRN.C. Seven
// panels of the races met, the spies sent to each, and four buttons
// (audience, report, declare war, ignore) that ask for a race next. Spec:
// docs/screens/races-screen.md.

// Addresses of RACESCRN.C's data.
const (
	racesLBX           = 0x17a34a // "races.lbx"
	raceSpace          = 0x17a354 // " ", the full-screen field's hot key
	raceHotKeys        = 0x17a380 // Esc, "I", "R", "A", "D", 2 bytes apart
	raceUnitRP         = 0x17a357 // "RP"; "BC", "FP", "MC", "PR" follow, 3 apart
	raceResearchFmt    = 0x17a366 // "\r%s%i%s"
	raceTradeMCFmt     = 0x17a36e // "\r%s%iMC"
	raceTradeBCFmt     = 0x17a376 // "\r%s%iBC"
	raceNewLine        = 0x17a37e // "\r"
	racePicture        = 0x183ff1 // _race_picture, x and y words
	raceBar            = 0x18400d // _race_bar
	raceSliderX        = 0x184029 // _race_slider_x
	raceText           = 0x184037 // _race_text
	raceSpies          = 0x184053 // _race_spies
	raceSpyBtns        = 0x18406f // _race_spy_btns
	raceSpyGroupData   = 0x183f8f // _spy_group_data, 0xe bytes per race
	raceAgentGroupData = 0x18408b // _agent_group_data
	raceMouseGroupData = 0x184099 // _mouse_group_data
	raceSliderRemap    = 0x183f64 // _slider_remap_list
	raceRelationNames  = 0x183f7e // _relation_name_table
	raceSpyBonusFmt    = 0x183f54 // _spy_bonus_fmt
	raceAgentBonusFmt  = 0x183f5c // _agent_bonus_fmt
	raceNormalColors   = 0x1840a7 // _race_normal_color
	raceHighColors     = 0x1840af // _race_high_color
	bannerColorHigh    = 0x1840b7 // _banner_color_high
	bannerColorLow     = 0x1840bf // _banner_color_low
	bannerColorDark    = 0x1840c7 // _banner_color_dark
	racesHelpList      = 0x1837a4 // _races_help_list, 8 entries
	treatyLabels       = 0x199150 // _treaty_labels
	raceIconGroupSize  = 0xe      // a _spy_group_data row
	raceRelations      = 17       // _relation_text entries
)

// iconGroup is a row of spy icons (a 0x20-byte record in RACESCRN.C):
// count icons of width iconW squeezed between x1 and x2.
type iconGroup struct {
	x1, y1, x2, y2 int16        // +0
	h, width       uint16       // +8, +0xa: height, the width the icons take
	count          uint16       // +0xc
	iconW, step    uint16       // +0x10, +0x12
	pic            *gfx.Picture // +0x14
	bitmap         *gfx.Picture // +0x18: icons go into it instead of the page
	erase          bool         // +0x1c is Erase_Mouse_Spies_
}

// raceDisplay is one race's panel (a 0x54-byte record).
type raceDisplay struct {
	idx, player int16
	noSlot      bool // +0xa: past _active_count
	eliminated  bool // +0xb
	group       iconGroup
	mission     int16  // +0x2c: 1 espionage, 2 sabotage, 3 hide
	sliderY     int16  // +0x2e
	barField    int    // +0x30
	relation    string // +0x32
	spyField    int    // +0x36: the spies, or in who mode the panel
	portrait    *gfx.Picture
	slider      *gfx.Picture
	elimPic     *gfx.Picture
	missionPics [3]*gfx.Picture
}

// raceScreenState holds RACESCRN.C's globals.
type raceScreenState struct {
	display       [7]raceDisplay
	agent, mouse  iconGroup
	inSpyMode     bool  // _g_in_spy_mode
	inWhoMode     bool  // _g_in_who_mode
	spyXferCount  int16 // _g_spy_xfer_count
	agentField    int   // _g_agent_field
	netState      byte  // _net_state, network games only: 1 asking, 2 told to wait
	netBox        *gfx.Picture
	netAsking     string // 0x1ad3a0: BILLTEXT 0x4c
	netWaitText   string // 0x1ad334: BILLTEXT 0x4d
	mouseSeg      *gfx.Picture
	arrowSeg      *gfx.Picture
	relationText  [raceRelations]string
	ignoring      string
	noContact     string
	spyBonus      string
	agentBonus    string
	researchLabel string
	tradeLabel    string
	givingLabel   string
	receiveLabel  string
	declareWar    string
	showing       bool // port-only: in the input loop, for tests
}

// raceScreen is Race_Screen_ (0x10acba), screen 6.
func (g *Game) raceScreen() error {
	e, rs, s := g.e, &g.race, &g.state
	me := int(s.PlayerNum)
	saved := e.GetAutoFunction()
	e.SavePalette()
	e.DisableDrawAllFields()
	if err := g.loadRaceTexts(); err != nil {
		return err
	}
	if err := e.FastFadeOut(); err != nil {
		return err
	}
	for {
		// The restart point (0x10b80b): everything is set up again.
		g.fullDrawScreen = false
		g.e.DeactivateHelpList()
		e.SetHelpList(g.helpListAt(racesHelpList, 8))
		e.ClearFields()
		e.V.SetPageOff()
		e.V.SetRefreshStencil()
		mouseBm := gfx.NewBlankBitmap(0xbb, 0x19)
		name := cStringAt(g.exe, racesLBX)
		pic := func(i int) (*gfx.Picture, error) { return g.picture(name, i) }
		bg, err := pic(0)
		if err != nil {
			return err
		}
		if rs.arrowSeg, err = pic(1); err != nil {
			return err
		}
		if rs.mouseSeg, err = pic(4); err != nil {
			return err
		}
		whoEntry := 5
		switch g.settings[0xd2] {
		case 1:
			whoEntry = 0x3d
		case 2:
			whoEntry = 0x3e
		case 4:
			whoEntry = 0x3f
		}
		whoPic, err := pic(whoEntry)
		if err != nil {
			return err
		}
		if rs.netBox, err = pic(0x3b); err != nil { // _netbox_seg
			return err
		}
		g.billPictureToMouse(rs.mouseSeg)
		g.getPlayersDeadOrAliveOrOmniscient(0, false)
		if err := g.initRaceDisplayData(); err != nil {
			return err
		}
		rs.mouse = g.iconGroupFromData(raceMouseGroupData, 0, nil)
		rs.mouse.bitmap = mouseBm
		rs.mouse.erase = true
		calculateIconGroupData(&rs.mouse)
		e.V.Fill(0, 0, 0x27f, 0x1df, 0)
		if err := e.Draw(0, 0, bg); err != nil {
			return err
		}
		if e.FadePercent() > 0 {
			g.turnOffPalettes()
		}
		e.ResetCyclePaletteColor()
		e.Text().SetOutlineColor(0xb1, 0xb1)
		if err := g.drawRacePhotos(); err != nil {
			return err
		}
		if err := g.drawRaceText(); err != nil {
			return err
		}
		g.getActivePlayers(0, false)
		if err := g.drawRelationsSliders(); err != nil {
			return err
		}
		e.V.CopyOffToBack()
		var vars raceButtonVars
		f, err := g.setupMainRaceFields(&vars)
		if err != nil {
			return err
		}
		if err := g.setupRaceDisplayFields(f.last); err != nil {
			return err
		}
		spaceF, err := e.AddHiddenField(0, 0, 0x27f, 0x1df, cStringAt(g.exe, raceSpace), 0)
		if err != nil {
			return err
		}
		e.AssignAutoFunction(g.drawRaceScreen, 2)
		if err := g.drawRaceScreen(); err != nil {
			return err
		}
		e.V.ClearPaletteChanges(0, 0xff)
		if err := e.TogglePages(); err != nil {
			return err
		}
		if err := e.FastFadeIn(); err != nil {
			return err
		}
		rs.inSpyMode, rs.inWhoMode = false, false
		var src *iconGroup // where the spies on the pointer came from
		srcIdx, srcMission := int16(-1), int16(-1)
		whoButton := 0
		putBack := func() {
			g.moveSpyGroupFromMouse(src)
			if src != &rs.agent && srcIdx >= 0 {
				rs.display[srcIdx].mission = srcMission
			}
		}
		rs.showing = true
	input:
		for {
			in, err := e.GetInput()
			if err != nil {
				return err
			}
			if g.fullDrawScreen {
				if rs.inSpyMode {
					g.moveSpyGroupFromMouse(src)
				}
				g.updateSpyStuff()
				break input
			}
			r := int(int16(in))
			if r < 0 {
				r = -r
			}
			switch {
			case r == 0:
				e.MarkTime()
				if err := g.drawRaceScreen(); err != nil {
					return err
				}
				if err := e.TogglePages(); err != nil {
					return err
				}
				if err := e.ReleaseTime(2); err != nil {
					return err
				}
			case r == f.exit:
			case r <= f.last:
				if rs.inSpyMode {
					putBack()
					if rs.mouse.count == 0 {
						rs.inSpyMode = false
					}
				}
				if rs.inWhoMode || r == f.report && g.state.Stars[0].Name[14]&4 != 0 {
					// Leave who mode (the report button does so too when the
					// option in star 0's name byte is set: the 1.31 patch).
					g.billPictureToMouse(rs.mouseSeg)
					vars = raceButtonVars{}
					if err := g.setupRaceDisplayFields(f.last); err != nil {
						return err
					}
					if spaceF, err = e.AddHiddenField(0, 0, 0x27f, 0x1df, cStringAt(g.exe, raceSpace), 0); err != nil {
						return err
					}
					whoButton = 0
					rs.inWhoMode = false
					break
				}
				switch r {
				case f.ignore:
					vars.audience, vars.report, vars.war = 0, 0, 0
				case f.report:
					vars.audience, vars.ignore, vars.war = 0, 0, 0
				case f.audience:
					vars.report, vars.ignore, vars.war = 0, 0, 0
				case f.war:
					vars.report, vars.ignore, vars.audience = 0, 0, 0
				}
				e.EnableDrawAllFields()
				if err := e.DrawVisibleFields(); err != nil {
					return err
				}
				e.DisableDrawAllFields()
				e.V.CopyOffToBack()
				if err := g.setupRaceWhoFields(f.last); err != nil {
					return err
				}
				if spaceF, err = e.AddHiddenField(0, 0, 0x27f, 0x1df, cStringAt(g.exe, raceSpace), 0); err != nil {
					return err
				}
				whoButton = r
				g.pictureToMouse(whoPic)
				rs.inWhoMode = true
			case rs.inWhoMode:
				if r == spaceF {
					break input
				}
				d := &rs.display[r-f.last-1]
				them := int(d.player)
				switch whoButton {
				case f.ignore:
					s.Players[me].Ignoring ^= 1 << uint(d.player&0xff)
				case f.report:
					g.billPictureToMouse(rs.mouseSeg)
					if err := e.FastFadeOut(); err != nil {
						return err
					}
					g.updateSpyStuff()
					rs.showing = false
					if err := g.raceReportScreen(them); err != nil {
						return err
					}
				case f.war:
					g.updateSpyStuff()
					if int8(s.Players[me].TreatyStatus[them]) >= 4 {
						break
					}
					msg, err := g.decodeTextMessage(rs.declareWar, them, -1, -1, -1, -1, -1, -1, 0x78)
					if err != nil {
						return err
					}
					rs.showing = false
					yes, err := g.confirmationBox(msg)
					if err != nil {
						return err
					}
					if yes {
						s.Players[me].DelayedOrder[them] = 0
					}
				case f.audience:
					g.updateSpyStuff()
					s.Players[me].Ignoring &^= 1 << uint(d.player&0xff)
					rs.showing = false
					if g.gameType == 2 || g.gameType == 3 {
						if err := g.raceScreenNetAudience(them); err != nil {
							return err
						}
						break
					}
					if err := g.diplomacyScreen(them); err != nil {
						return err
					}
				}
				break input
			default:
				// Spies picked up and put down (mouse only).
				vars = raceButtonVars{}
				group, idx := g.spyGroupForField(r)
				if rs.inSpyMode {
					to := group
					if group == nil {
						to = src
						if src != &rs.agent && srcIdx >= 0 {
							rs.display[srcIdx].mission = srcMission
						}
					}
					g.moveSpyGroupFromMouse(to)
					if rs.mouse.count == 0 {
						rs.inSpyMode = false
						src = nil
					}
				} else if group != nil && group.count > 0 {
					moveIconGroupIcons(group, &rs.mouse, uint16(rs.spyXferCount))
					if err := g.redrawSpyMouse(rs.arrowSeg); err != nil {
						return err
					}
					srcIdx = idx
					rs.inSpyMode = true
					if idx >= 0 && int(idx) < len(rs.display) {
						srcMission = rs.display[idx].mission
					}
					src = group
				}
				for i := 0; i < int(g.activeCount); i++ {
					if rs.display[i].mission == 0 {
						rs.display[i].mission = 3
					}
				}
			}
			if r == f.exit {
				if rs.inSpyMode {
					putBack()
				}
				g.updateSpyStuff()
				rs.showing = false
				if err := e.FastFadeOut(); err != nil {
					return err
				}
				e.PutAutoFunction(saved)
				e.DeactivateHelpList()
				g.resetMousePicture()
				e.V.SetPaletteChanges(0, 0xff)
				e.RestorePalette()
				g.turnOffPalettes()
				e.EnableDrawAllFields()
				g.currentScreen = ScreenMain
				return nil
			}
		}
		rs.showing = false
	}
}

// raceButtonVars are the four radio buttons' variables.
type raceButtonVars struct{ audience, report, ignore, war int16 }

// raceMainFields are the field numbers Setup_Main_Fields_ gives back.
type raceMainFields struct {
	exit, audience, report, ignore, war int
	last                                int
}

// loadRaceTexts is the start of Race_Screen_: the texts of BILLTEXT.LBX
// (and BILLTEX2.LBX) into RACESCRN.C's buffers.
func (g *Game) loadRaceTexts() error {
	rs := &g.race
	var err error
	for i := range rs.relationText {
		if rs.relationText[i], err = g.textMessage(billTextLBX, 0x21+i, 0xf); err != nil {
			return err
		}
	}
	for _, t := range []struct {
		dst  *string
		file string
		n    int
		max  int
	}{
		{&rs.ignoring, billTextLBX, 0x32, 0x14},
		{&rs.noContact, billTextLBX, 0x33, 0x14},
		{&rs.spyBonus, billTextLBX, 0x34, 0x14},
		{&rs.agentBonus, billTextLBX, 0x35, 0x14},
		{&rs.researchLabel, billTextLBX, 0x39, 0x1e},
		{&rs.tradeLabel, billTextLBX, 0x3a, 0x1e},
		{&rs.givingLabel, billTextLBX, 0x3b, 0x50},
		{&rs.receiveLabel, billTextLBX, 0x3c, 0x50},
		{&rs.netAsking, billTextLBX, 0x4c, 100},
		{&rs.netWaitText, billTextLBX, 0x4d, 100},
		{&rs.declareWar, "billtex2.lbx", 0x1a, 0x78},
	} {
		s, err := g.textMessage(t.file, t.n, t.max)
		if err != nil {
			return err
		}
		if t.dst != nil {
			*t.dst = s
		}
	}
	return nil
}

// xy reads entry i of a table of (x, y) words.
func (g *Game) xy(table uint32, i int) (int16, int16) {
	return g.exeI16(table + 4*uint32(i)), g.exeI16(table + 4*uint32(i) + 2)
}

// iconGroupFromData is an icon group as Init_Race_Display_Data_ and
// Setup_Icon_Group_Data_ fill it from a template row.
func (g *Game) iconGroupFromData(a uint32, count uint16, pic *gfx.Picture) iconGroup {
	gr := iconGroup{
		x1: g.exeI16(a), y1: g.exeI16(a + 2), x2: g.exeI16(a + 4), y2: g.exeI16(a + 6),
		h: uint16(g.exeI16(a + 8)), count: count, pic: pic,
	}
	if pic != nil {
		gr.iconW = uint16(pic.Width)
	}
	return gr
}

// initRaceDisplayData is Init_Race_Display_Data_ (0x10ba3d): a panel per
// slot, the human's spy icon for every row of spies and its agents.
func (g *Game) initRaceDisplayData() error {
	rs, s := &g.race, &g.state
	me := int(s.PlayerNum)
	name := cStringAt(g.exe, racesLBX)
	spyIcon, err := g.picture(name, int(s.Players[me].Race)+0x2e)
	if err != nil {
		return err
	}
	slider, err := g.picture(name, 2)
	if err != nil {
		return err
	}
	elim, err := g.picture(name, 0x1f)
	if err != nil {
		return err
	}
	for i := range rs.display {
		d := &rs.display[i]
		*d = raceDisplay{}
		d.slider = slider
		if i >= int(g.activeCount) {
			d.noSlot = true
			continue
		}
		p := int(g.activePlayer[i])
		rel := int16(s.Players[me].Relations[p]) + 100
		k := 0
		for rel > int16(g.exeU8(raceRelationNames+uint32(k))) {
			k++
		}
		d.relation = rs.relationText[k]
		remap := int16(g.exeU8(raceSliderRemap + uint32(rel>>3)))
		d.idx, d.player = int16(i), int16(p)
		_, by := g.xy(raceBar, i)
		d.sliderY = by + remap - 6
		d.elimPic = elim
		if s.Players[p].Eliminated != 0 {
			d.eliminated = true
		} else {
			d.mission = int16(s.Players[me].Spies[p]>>6) + 1
			d.group = g.iconGroupFromData(raceSpyGroupData+uint32(i)*raceIconGroupSize, uint16(s.Players[me].Spies[p]&0x3f), spyIcon)
			calculateIconGroupData(&d.group)
			for k, base := range [3]int{0xa, 0x11, 0x18} {
				if d.missionPics[k], err = g.picture(name, base+i); err != nil {
					return err
				}
			}
		}
		if d.portrait, err = g.picture(name, int(s.Players[p].Race)+0x20); err != nil {
			return err
		}
	}
	rs.agent = g.iconGroupFromData(raceAgentGroupData, uint16(s.Players[me].Spies[me]&0x3f), spyIcon)
	calculateIconGroupData(&rs.agent)
	return nil
}

// drawRelationsSliders is Draw_Relations_Sliders_ (0x10bd50): each race's
// bar with the slider at its relation; empty slots get the slider alone.
func (g *Game) drawRelationsSliders() error {
	e, rs := g.e, &g.race
	bar, err := g.picture(cStringAt(g.exe, racesLBX), 3)
	if err != nil {
		return err
	}
	for i := range rs.display {
		d := &rs.display[i]
		x, y := g.xy(raceBar, i)
		sx := int(g.exeI16(raceSliderX + 2*uint32(i)))
		if i < int(g.activeCount) {
			if err := e.Draw(int(x), int(y), bar); err != nil {
				return err
			}
			if err := e.Draw(sx, int(d.sliderY), d.slider); err != nil {
				return err
			}
			continue
		}
		if err := e.Draw(sx, int(y+0x4b), d.slider); err != nil {
			return err
		}
	}
	return nil
}

// drawRacePhotos is Draw_Race_Photos_ (0x10bded): the portraits, crossed
// out when eliminated, framed in the banner colours.
func (g *Game) drawRacePhotos() error {
	e, rs, s := g.e, &g.race, &g.state
	for i := 0; i < int(g.activeCount); i++ {
		d := &rs.display[i]
		x, y := g.xy(racePicture, i)
		if err := e.Draw(int(x), int(y), d.portrait); err != nil {
			return err
		}
		if d.eliminated {
			if err := e.Draw(int(x), int(y), d.elimPic); err != nil {
				return err
			}
		}
		c := uint32(s.Players[d.player].Color)
		e.Box(int(x), int(y), 0x4c, 0x58, 1, g.exeU8(bannerColorHigh+c))
		e.Box(int(x+1), int(y+1), 0x4a, 0x56, 1, g.exeU8(bannerColorLow+c))
		dark := g.exeU8(bannerColorDark + c)
		e.V.Dot(int(x), int(y), dark)
		e.V.Dot(int(x+0x4b), int(y), dark)
		e.V.Dot(int(x), int(y+0x57), dark)
		e.V.Dot(int(x+0x4b), int(y+0x57), dark)
	}
	return nil
}

// drawRaceText is Draw_Race_Text_ (0x10bfbd): under each portrait the
// emperor's name, beside it the treaties, trade and research agreements
// and tribute; "no contact" for empty slots and races the human only sees
// by omniscience; below, the spy and agent bonuses.
func (g *Game) drawRaceText() error {
	e, rs, s := g.e, &g.race, &g.state
	me := int(s.PlayerNum)
	pl := &s.Players[me]
	t := e.Text()
	omni := s.playerIsOmniscient(me)
	for i := range rs.display {
		d := &rs.display[i]
		tx, ty := g.xy(raceText, i)
		px, py := g.xy(racePicture, i)
		emperor := save.CString(s.Players[d.player].EmperorName[:])
		if d.noSlot || omni && pl.Contact[d.player] == 0 {
			t.SetStyle(4, g.exeColors(raceNormalColors))
			if err := t.PrintCentered(int(tx+0x5b), int(ty+10), rs.noContact); err != nil {
				return err
			}
			if i < int(g.activeCount) && omni && pl.Contact[d.player] == 0 {
				t.SetStyleShadow(2, g.exeColors(raceHighColors), font.Outline)
				if err := g.squeezeCenteredPrint(int(px), int(py+0x4c), emperor, 0x4c); err != nil {
					return err
				}
			}
			continue
		}
		t.SetStyleShadow(2, g.exeColors(raceHighColors), font.Outline)
		if err := g.squeezeCenteredPrint(int(px), int(py+0x4c), emperor, 0x4c); err != nil {
			return err
		}
		if d.eliminated {
			continue
		}
		text, err := g.raceTreatyText(int(d.player))
		if err != nil {
			return err
		}
		t.SetStyle(2, g.exeColors(raceNormalColors))
		if err := t.SqueezePrintParagraph(int(tx), int(int16(0x2b/2+ty)), 0xb6, 0x2b, text, 2, g.exe.bytes(spaceLimits, 6), true); err != nil {
			return err
		}
	}
	t.SetStyle(3, g.exeColors(raceNormalColors))
	off, def := g.computeSpyBonuses(me)
	str := fmt.Sprintf(cFormat(cStringAt(g.exe, raceSpyBonusFmt)), rs.spyBonus, off)
	if err := g.squeezeCenteredPrint(0x1a1, 0x174, str, 0x62); err != nil {
		return err
	}
	str = fmt.Sprintf(cFormat(cStringAt(g.exe, raceAgentBonusFmt)), rs.agentBonus, def)
	return g.squeezeCenteredPrint(0x207, 0x174, str, 0x61)
}

// raceTreatyText is the paragraph Draw_Race_Text_ builds for player p.
func (g *Game) raceTreatyText(p int) (string, error) {
	rs, s := &g.race, &g.state
	me := int(s.PlayerNum)
	pl := &s.Players[me]
	var b strings.Builder
	if st := int16(int8(pl.TreatyStatus[p])); st > 0 {
		if st >= 6 {
			st = 5
		}
		label, err := g.eStringAt(treatyLabels + 4*uint32(st))
		if err != nil {
			return "", err
		}
		b.WriteString(strings.ToUpper(label))
	}
	if pl.ResearchTreaty[p] != 0 {
		v := int32(pl.ResearchValue[p])
		unit := uint32(raceUnitRP) // and "BC" for a negative value
		switch g.settings[0xd2] {
		case 1:
			unit = raceUnitRP + 6 // "FP", "MC"
		case 4:
			if v >= 0 {
				unit = raceUnitRP + 12 // "PR"
			}
		}
		if v < 0 {
			switch g.settings[0xd2] {
			case 1:
				unit = raceUnitRP + 9
			default:
				unit = raceUnitRP + 3
			}
		}
		b.WriteString(fmt.Sprintf(cFormat(cStringAt(g.exe, raceResearchFmt)), rs.researchLabel, v, cStringAt(g.exe, unit)))
	}
	if pl.TradeTreaty[p] != 0 {
		f := raceTradeBCFmt
		if g.settings[0xd2] == 1 {
			f = raceTradeMCFmt
		}
		b.WriteString(fmt.Sprintf(cFormat(cStringAt(g.exe, uint32(f))), rs.tradeLabel, pl.TradeValue[p]))
	}
	tribute := func(label string, v int16) error {
		b.WriteString(cStringAt(g.exe, raceNewLine))
		n := int32(10)
		if v == 1 {
			n = 5
		}
		str, err := g.decodeTextMessage(label, -1, -1, -1, n, -1, -1, -1, 0x40)
		if err != nil {
			return err
		}
		b.WriteString(str)
		return nil
	}
	if pl.Tribute[p] != 0 {
		if err := tribute(rs.givingLabel, pl.Tribute[p]); err != nil {
			return "", err
		}
	} else if v := s.Players[p].Tribute[me]; v != 0 {
		if err := tribute(rs.receiveLabel, v); err != nil {
			return "", err
		}
	}
	if b.Len() < 1 {
		return g.eStringAt(treatyLabels)
	}
	return b.String(), nil
}

// setupMainRaceFields is Setup_Main_Fields_ (0x10c4e9): RETURN and, with
// races to show, the four radio buttons.
func (g *Game) setupMainRaceFields(v *raceButtonVars) (raceMainFields, error) {
	e := g.e
	e.ClearFields()
	name := cStringAt(g.exe, racesLBX)
	hot := func(i int) string { return cStringAt(g.exe, raceHotKeys+2*uint32(i)) }
	var f raceMainFields
	pic, err := g.picture(name, 9)
	if err != nil {
		return f, err
	}
	if f.exit, err = e.AddButtonField(0x217, 0x1b1, "", pic, hot(0), 0x28); err != nil {
		return f, err
	}
	*v = raceButtonVars{}
	if g.activeCount >= 1 {
		for _, b := range []struct {
			dst  *int
			x, y int
			pic  int
			v    *int16
			key  int
		}{
			{&f.ignore, 0x1ad, 0x1bc, 6, &v.ignore, 1},
			{&f.report, 0x1ad, 0x1a7, 7, &v.report, 2},
			{&f.audience, 0x14e, 0x1a7, 8, &v.audience, 3},
			{&f.war, 0x14e, 0x1bc, 0x3c, &v.war, 4},
		} {
			p, err := g.picture(name, b.pic)
			if err != nil {
				return f, err
			}
			if *b.dst, err = e.AddRadioButtonField(b.x, b.y, "", p, b.v, hot(b.key), 0x28); err != nil {
				return f, err
			}
		}
	}
	f.last = e.FieldsCount() - 1
	return f, nil
}

// setupRaceDisplayFields is Setup_Race_Display_Fields_ (0x10c670): the
// agents' field, per race the three mission buttons, the spies' field and
// the relation bar's field.
func (g *Game) setupRaceDisplayFields(last int) error {
	e, rs := g.e, &g.race
	if g.activeCount < 1 {
		return nil
	}
	e.ClearFieldsAbove(last)
	var err error
	if rs.agentField, err = e.AddHiddenField(0x148, 0x182, 0x265, 0x19f, "", 0); err != nil {
		return err
	}
	for i := 0; i < int(g.activeCount); i++ {
		d := &rs.display[i]
		x, y := g.xy(raceSpyBtns, i)
		for k, dx := range [3]int16{0, 0x4c, 0x95} {
			if _, err := e.AddMultiButtonField(int(x+dx), int(y), "", d.missionPics[k], &d.mission, int16(k+1), "", 0x29); err != nil {
				return err
			}
		}
		sx, sy := g.xy(raceSpies, i)
		if d.spyField, err = e.AddHiddenField(int(sx), int(sy-6), int(sx+0xbb), int(sy+0x19), "", 0); err != nil {
			return err
		}
		bx, by := g.xy(raceBar, i)
		if d.barField, err = e.AddHiddenField(int(bx-6), int(by-6), int(bx+8), int(by+0x58), "", 0); err != nil {
			return err
		}
	}
	return nil
}

// setupRaceWhoFields is Setup_Race_Who_Fields_ (0x10c7f6): in who mode a
// field over each race's whole panel row.
func (g *Game) setupRaceWhoFields(last int) error {
	e, rs := g.e, &g.race
	if g.activeCount < 1 {
		return nil
	}
	e.ClearFieldsAbove(last)
	hot := ""
	if g.listKey {
		hot = "1"
	}
	x := 0
	for i := 0; i < int(g.activeCount); i++ {
		if i == 4 {
			x = 0x140
		}
		_, y := g.xy(racePicture, i)
		if g.diploKeys {
			hot = string(rune('1' + i)) // Options.DiploKeys: panel i gets "1"+i
		}
		f, err := e.AddHiddenField(x, int(y), x+0x13f, int(y+0x58), hot, 0)
		if err != nil {
			return err
		}
		rs.display[i].spyField = f
	}
	rs.agentField = 9999
	return nil
}

// updateSpyStuff is Update_Spy_Stuff_ (0x10c88d): the panels' spies and
// missions, and the agents, back into the human's record.
func (g *Game) updateSpyStuff() {
	rs, s := &g.race, &g.state
	pl := &s.Players[s.PlayerNum]
	for i := 0; i < int(g.activeCount); i++ {
		d := &rs.display[i]
		if d.mission > 0 {
			pl.Spies[d.player] = pl.Spies[d.player]&0x3f | byte(d.mission-1)<<6
		}
		pl.Spies[d.player] = pl.Spies[d.player]&0xc0 | byte(d.group.count)&0x3f
	}
	me := s.PlayerNum
	pl.Spies[me] = pl.Spies[me]&0xc0 | byte(rs.agent.count)&0x3f
}

// drawRaceScreen is Draw_Race_Screen_ (0x10c8e0), the auto function: the
// fields, the spies, what the pointer is over (the spies under it, the
// relation on the bar, in who mode the panel), "Ignoring", and the spies
// carried by the pointer.
func (g *Game) drawRaceScreen() error {
	e, rs, s := g.e, &g.race, &g.state
	scan, err := e.ScanInput()
	if err != nil {
		return err
	}
	scan = int(int16(scan))
	px, py := e.Pointer()
	e.EnableDrawAllFields()
	if err := e.DrawVisibleFields(); err != nil {
		return err
	}
	e.DisableDrawAllFields()
	t := e.Text()
	high := g.exeColors(raceHighColors)
	for i := 0; i < int(g.activeCount); i++ {
		d := &rs.display[i]
		if err := g.drawIconGroup(&d.group); err != nil {
			return err
		}
		x, y := g.xy(racePicture, int(d.idx))
		switch {
		case rs.inWhoMode:
			if d.spyField == scan {
				e.Box(int(x), int(y), 0x4c, 0x58, 1, 0xff)
			}
		case d.spyField == scan:
			if rs.inSpyMode || d.group.count != 0 {
				t.SetStyleShadow(2, high, font.Outline)
				rs.spyXferCount = g.drawIconGroupCursor(&d.group, px, 0xff, true)
			}
		case d.barField == scan:
			t.SetStyleShadow(2, high, font.Outline)
			bx, _ := g.xy(raceBar, int(d.idx))
			if err := t.PrintCentered(int(bx+4), int(d.sliderY+1), d.relation); err != nil {
				return err
			}
		}
		if s.Players[s.PlayerNum].Ignoring&(1<<uint(d.player&0x1f)) != 0 {
			t.SetStyleShadow(1, high, font.Outline)
			if err := t.PrintCentered(int(x+0x25), int(y+0x42), rs.ignoring); err != nil {
				return err
			}
		}
	}
	if err := g.drawIconGroup(&rs.agent); err != nil {
		return err
	}
	if rs.agentField == scan && (rs.agent.count > 0 || rs.inSpyMode) {
		t.SetStyleShadow(2, high, font.Outline)
		rs.spyXferCount = g.drawIconGroupCursor(&rs.agent, px, 0xff, true)
	}
	if rs.mouse.count != 0 {
		e.V.ClipOn()
		err := e.Draw(int(int16(px+3)), int(int16(py+2)), rs.mouse.bitmap)
		e.V.ClipOff()
		if err != nil {
			return err
		}
	}
	e.CyclePaletteColor(0xff, 10, 0x14, 5, 0x28, 0x3c, 10, 7)
	t.SetStyleShadow(4, high, font.Outline)
	// A network request under way: "asking" or "told to wait" in the box.
	y, text := 0xda, rs.netAsking
	switch rs.netState {
	case 1:
	case 2:
		y, text = 0xd1, rs.netWaitText
	default:
		return nil
	}
	if err := e.Draw(0x85, 0xbf, rs.netBox); err != nil {
		return err
	}
	msg, err := g.decodeTextMessage(text, g.netDiplomacyWith(), -1, -1, -1, -1, -1, -1, 100)
	if err != nil {
		return err
	}
	return g.printFormattedParagraph(0x8f, y, 0x15e, msg, 2)
}

// spyGroupForField is Get_Spy_Group_For_Field_ (0x10cbdf): the agents, or
// the spies of the race whose field f is, with its index.
func (g *Game) spyGroupForField(f int) (*iconGroup, int16) {
	rs := &g.race
	if f == rs.agentField {
		return &rs.agent, 0
	}
	for i := 0; i < int(g.activeCount); i++ {
		if rs.display[i].spyField == f {
			return &rs.display[i].group, int16(i)
		}
	}
	return nil, int16(g.activeCount)
}

// redrawSpyMouse is Redraw_Spy_Mouse_ (0x10cc4d): the carried spies, the
// arrow and their number into the pointer's bitmap.
func (g *Game) redrawSpyMouse(arrow *gfx.Picture) error {
	rs := &g.race
	if err := g.drawIconGroup(&rs.mouse); err != nil {
		return err
	}
	if err := rs.mouse.bitmap.AddPicture(1, 1, arrow); err != nil {
		return err
	}
	if rs.spyXferCount > 1 {
		t := g.e.Text()
		t.SetStyleShadow(2, g.exeColors(raceHighColors), font.Outline)
		if err := t.ToBitmap(bitmapTarget(rs.mouse.bitmap), 8, 4, []byte(strconv.Itoa(int(rs.spyXferCount)))); err != nil {
			return err
		}
	}
	g.clearMousePicture()
	return nil
}

// moveSpyGroupFromMouse is Move_Spy_Group_From_Mouse_ (0x10ccc5): the
// carried spies onto dst, at most 63 there.
func (g *Game) moveSpyGroupFromMouse(dst *iconGroup) {
	rs := &g.race
	m := &rs.mouse
	if m.count == 0 || dst == nil {
		return
	}
	n := m.count
	if uint32(dst.count)+uint32(n) > 0x3f {
		n = 0x3f - dst.count
		rs.spyXferCount = int16(m.count - n)
	}
	if m.count < n {
		n = m.count
	}
	if n != 0 {
		dst.count += n
		m.count -= n
		dst.iconW, dst.pic = m.iconW, m.pic
		calculateIconGroupData(dst)
		calculateIconGroupData(m)
	}
	if m.count == 0 {
		g.billPictureToMouse(rs.mouseSeg)
		return
	}
	_ = g.redrawSpyMouse(rs.arrowSeg)
}

// moveIconGroupIcons is Move_Icon_Group_Icons_ (0x10cebe): up to n icons
// from src to dst.
func moveIconGroupIcons(src, dst *iconGroup, n uint16) {
	if src.count < n {
		n = src.count
	}
	if n == 0 {
		return
	}
	dst.count += n
	src.count -= n
	dst.iconW, dst.pic = src.iconW, src.pic
	calculateIconGroupData(dst)
	calculateIconGroupData(src)
}

// calculateIconGroupData is Calculate_Icon_Group_Data_ (0x10cdd8): the
// step between icons so that they fit between x1 and x2.
func calculateIconGroupData(gr *iconGroup) {
	gr.h = uint16(gr.y2-gr.y1) + 1
	w := int16(gr.x2-gr.x1) + 1
	if gr.count < 2 {
		gr.step, gr.width = gr.iconW, gr.iconW
		return
	}
	step := int16((int32(w) - int32(gr.iconW)) / int32(int16(gr.count-1)))
	if int32(gr.iconW) < int32(step) {
		step = int16(gr.iconW)
	}
	if step < 0 {
		step = 0
	}
	gr.step = uint16(step)
	gr.width = gr.step*(gr.count-1) + gr.iconW
	if int32(w) < int32(gr.width) {
		gr.width = uint16(w)
	}
}

// drawIconGroupCursor is Draw_Icon_Group_Cursor_ (0x10cef6): a box from the
// icon under x to the last one (the ones a click picks up), their number
// printed when asked; returns that number.
func (g *Game) drawIconGroupCursor(gr *iconGroup, x int, c byte, number bool) int16 {
	e := g.e
	if gr.count == 0 {
		e.Box(int(gr.x1), int(gr.y1), int(gr.iconW), int(gr.h), 1, c)
		return 0
	}
	right := int16(uint16(gr.x1) + gr.width - 1)
	xx := int16(x)
	if right < xx {
		xx = right
	}
	if int32(xx) < int32(uint16(gr.x1)) {
		xx = gr.x1
	}
	var k int32
	if gr.step != 0 { // the original divides by zero here; no group is that narrow
		k = (int32(xx) - int32(uint16(gr.x1))) / int32(gr.step)
	}
	if int32(gr.count) <= int32(int16(k)) {
		k = int32(gr.count) - 1
	}
	n := int16(int32(gr.count) - k)
	if n <= 0 {
		return 0
	}
	bx := int16(uint16(gr.x1) + gr.step*uint16(k))
	e.Box(int(bx), int(gr.y1), int(right-bx)+1, int(gr.h), 1, c)
	if number && n > 1 {
		_ = e.Text().Print(int(bx+3), int(gr.y1+3), strconv.Itoa(int(n)))
	}
	return n
}

// drawIconGroup is Draw_Icon_Group_ (0x10cfd7): the icons on the page, or
// into the group's bitmap (which Erase_Mouse_Spies_ blanks first).
func (g *Game) drawIconGroup(gr *iconGroup) error {
	if gr.erase {
		*gr.bitmap = *gfx.NewBlankBitmap(0xbb, 0x19)
	}
	x := gr.x1
	for i := 0; i < int(gr.count); i++ {
		if gr.bitmap == nil {
			if err := g.e.Draw(int(x), int(gr.y1), gr.pic); err != nil {
				return err
			}
		} else if err := gr.bitmap.AddPicture(int(x), int(gr.y1), gr.pic); err != nil {
			return err
		}
		x += int16(gr.step)
	}
	return nil
}

// pictureToMouse is Picture_To_Mouse_ (0xc0a5f): pic becomes the pointer
// (image 0x13 of _mouse_list_variable).
func (g *Game) pictureToMouse(pic *gfx.Picture) {
	e := g.e
	bm := gfx.NewBlankBitmap(1, 1)
	bm.DrawPictureToBitmap(pic)
	e.ClearMouseImage(mouseListVarImage - 1)
	e.DrawBitmapToMouse(0, 0, bm, mouseListVarImage-1)
	e.SetMouseList(mouseListVariable)
}

// squeezeCenteredPrint is Squeeze_Centered_Print_ (0x102fa8): _Squeeze_
// Print_ centred in maxw, a smaller style moved down to stay centred.
func (g *Game) squeezeCenteredPrint(x, y int, s string, maxw int) error {
	t := g.e.Text()
	b := []byte(s)
	print := func(y int) error {
		if int16(maxw) < 10 || int16(maxw) > 0x280 || int16(x) < 0 || int16(x) >= 0x280 || int16(y) < 0 || int16(y) >= 0x1e0 {
			return nil
		}
		return t.PrintCentered(int(int16(x+int(int16(maxw))/2)), y, s)
	}
	if t.Width(b) <= maxw {
		return print(y)
	}
	limits := g.exe.bytes(spaceLimits, 6)
	style, space, hsp, shadow := t.Style, t.SpaceWidth(), t.Spacing, t.Shadow
	h0 := t.Height()
	dy := 0
	smaller := false
	for {
		for t.Style < len(limits) && limits[t.Style] < byte(t.SpaceWidth()) && t.Width(b) > maxw {
			t.SetSpaceWidth(t.SpaceWidth() - 1)
		}
		for t.Width(b) > maxw && t.Spacing > 1 {
			t.SetSpacing(t.Spacing - 1)
		}
		if t.Width(b) <= maxw || t.Style < 1 {
			break
		}
		t.SetStyle(t.Style-1, t.Normal)
		dy = (int(uint16(h0)) - int(int16(t.Height()))) / 2
		smaller = true
	}
	t.Shadow = shadow
	err := print(int(int16(y + dy)))
	t.SetSpacing(hsp)
	t.SetSpaceWidth(space)
	if smaller {
		t.SetStyle(style, t.Normal)
		t.Shadow = shadow
	}
	return err
}
