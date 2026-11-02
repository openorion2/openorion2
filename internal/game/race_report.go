package game

import (
	"fmt"
	"strings"

	"github.com/openorion2/openorion2/internal/gfx"
	"github.com/openorion2/openorion2/internal/save"
)

// The race report (MOX\RACERPRT.C): a race's emperor, personality and
// objective, its spies in the human's empire, its allies and enemies, and
// the tech review of what it knows. Spec: docs/screens/races-screen.md.

const (
	racerprtLBX          = 0x17a30e // "racerprt.lbx"
	reportListSep        = 0x17a31b // "\n"
	reportTreatiesFmt    = 0x183f28 // _TREATIES_FMT, "%s\n%s\n%s\n%s"
	reportPersonality    = 0x183f34 // _PERSONALITY_FMT, "%s\r%s"
	reportNormalColors   = 0x183f3a // _report_normal_color
	reportSpyGroup       = 0x183f43 // RACERPRT.C's _spy_group_data
	reportHelpList       = 0x1837f4 // _report_help_list, 1 entry (Report_Help_)
	personalityLabels    = 0x197fa0 // _personality_labels
	objectiveLabels      = 0x192210 // _objective_labels
	reportHumanObjective = 100
)

// raceReportScreen is Race_Report_Screen_ (0x10a6c0) for player p.
func (g *Game) raceReportScreen(p int) error {
	e, s := g.e, &g.state
	me := int(s.PlayerNum)
	pp := &s.Players[p]
	pers, err := g.eStringAt(personalityLabels + 4*uint32(pp.Personality))
	if err != nil {
		return err
	}
	if pp.Objective != reportHumanObjective {
		obj, err := g.eStringAt(objectiveLabels + 4*uint32(pp.Objective))
		if err != nil {
			return err
		}
		pers = fmt.Sprintf(cFormat(cStringAt(g.exe, reportPersonality)), pers, obj)
	}
	// Set_Paragraph_Integer_Pointer_(0) points at an unset local; the
	// treaties paragraph does not use it.
	e.ClearFields()
	e.DeactivateHelpList()
	e.SetHelpList(g.helpListAt(reportHelpList, 1))
	e.EnableDrawAllFields()
	races := cStringAt(g.exe, racesLBX)
	portrait, err := g.picture(races, int(pp.Race)+0x20)
	if err != nil {
		return err
	}
	spyIcon, err := g.picture(races, int(pp.Race)+0x2e)
	if err != nil {
		return err
	}
	var pics [10]*gfx.Picture
	for i := range pics {
		if pics[i], err = g.picture(cStringAt(g.exe, racerprtLBX), i); err != nil {
			return err
		}
	}
	spies := s.theirSpyNumber(p, me)
	group := g.iconGroupFromData(reportSpyGroup, uint16(spies), spyIcon)
	calculateIconGroupData(&group)
	sep := cStringAt(g.exe, reportListSep)
	var allies, wars strings.Builder
	nAllies, nWars := 0, 0
	for i := 0; i < int(s.NumPlayers); i++ {
		if i == p {
			continue
		}
		switch st := int16(int8(pp.TreatyStatus[i])); {
		case st == 2:
			allies.WriteString(save.CString(s.Players[i].RaceName[:]) + sep)
			nAllies++
		case st >= 4:
			wars.WriteString(save.CString(s.Players[i].RaceName[:]) + sep)
			nWars++
		}
	}
	if nAllies == 0 {
		none, err := g.textMessage(billTextLBX, 0x1d, 0x28)
		if err != nil {
			return err
		}
		allies.WriteString(none + sep)
	}
	if nWars == 0 {
		none, err := g.textMessage(billTextLBX, 0x1e, 0x28)
		if err != nil {
			return err
		}
		wars.WriteString(none + sep)
	}
	alliesLabel, err := g.textMessage(billTextLBX, 0x1f, 0x28)
	if err != nil {
		return err
	}
	warsLabel, err := g.textMessage(billTextLBX, 0x20, 0x28)
	if err != nil {
		return err
	}
	treaties := fmt.Sprintf(cFormat(cStringAt(g.exe, reportTreatiesFmt)), alliesLabel, allies.String(), warsLabel, wars.String())
	is := &g.info
	if is.exitField, err = e.AddButtonField(0x216, 0x1b1, "", pics[1], "\x1b", 0x28); err != nil {
		return err
	}
	first := true
	limits := g.exe.bytes(spaceLimits, 6)
	for {
		if g.fullDrawScreen {
			e.ClearFieldsAbove(is.exitField)
		}
		g.fullDrawScreen = false
		e.V.SetPageOff()
		e.V.Fill(0, 0, 0x27f, 0x1df, 0)
		g.triInterlacedFill(0xd4, 0x17, 0x26c, 0x1c9, 0xb2, 0xb3, 0xb3)
		if err := e.Draw(0, 0, pics[0]); err != nil {
			return err
		}
		if first {
			g.turnOffPalettes()
		}
		if err := e.Draw(0x46, 0x1c, portrait); err != nil {
			return err
		}
		c := uint32(pp.Color)
		e.Box(0x46, 0x1c, 0x4c, 0x58, 1, g.exeU8(bannerColorHigh+c))
		e.Box(0x47, 0x1d, 0x4a, 0x56, 1, g.exeU8(bannerColorLow+c))
		t := e.Text()
		normal := g.exeColors(reportNormalColors)
		t.SetStyle(3, normal)
		if err := t.SqueezePrintParagraph(0x2a, 0x1a/2+0x87, 0x80, 0x1a, save.CString(pp.EmperorName[:]), 2, limits, true); err != nil {
			return err
		}
		t.SetStyle(4, normal)
		if err := t.SqueezePrintParagraph(0x18, 0x2a/2+0xa7, 0xa4, 0x2a, pers, 2, limits, true); err != nil {
			return err
		}
		var line string
		if spies != 0 {
			if err := g.drawIconGroup(&group); err != nil {
				return err
			}
			t.SetStyle(2, normal)
			if spies > 1 {
				if line, err = g.textMessage(billTextLBX, 0x4a, 0x28); err != nil {
					return err
				}
				if line, err = g.decodeTextMessage(line, -1, -1, -1, int32(spies), -1, -1, -1, 0x28); err != nil {
					return err
				}
			} else if line, err = g.textMessage(billTextLBX, 0x4b, 0x28); err != nil {
				return err
			}
		} else {
			t.SetStyle(3, normal)
			if line, err = g.textMessage(billTextLBX, 0x1c, 0x28); err != nil {
				return err
			}
		}
		if err := g.squeezeCenteredPrint(0x18, 0xde, line, 0xa4); err != nil {
			return err
		}
		t.SetStyle(4, normal)
		if err := t.SqueezePrintFormattedParagraph(0x18, 0xaa/2+0x123, 0xa4, 0xaa, treaties, 2, limits, true); err != nil {
			return err
		}
		e.V.CopyOffToBack()
		if err := e.FastFadeIn(); err != nil {
			return err
		}
		first = false
		g.raceReportJimHack = true
		r, err := g.techReviewPage(pics[2:10], is.exitField+1, p)
		if err != nil {
			return err
		}
		g.raceReportJimHack = false
		if r == is.exitField {
			break
		}
	}
	return e.FastFadeOut()
}
