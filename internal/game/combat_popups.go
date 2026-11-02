package game

import (
	"fmt"

	"github.com/openorion2/openorion2/internal/font"
	"github.com/openorion2/openorion2/internal/save"
)

const playerAttackedHelpList = 0x1834fc // _player_attacked_popup_help_list

// The combat popups of a human attacker (MAINPUPS.C, COMBFIND.C):
// Defense_Colony_Selection_Popup_ picks the target, Human_Confirms_Attack_
// asks before a treaty is broken. Spec: docs/screens/combat-screen.md.

const (
	youHaveTreatiesMsg   = 0x1aa3cc
	tradeTreatyMsg       = 0x1aa3c8
	researchTreatyMsg    = 0x1aa3c0
	receivingTributeMsg  = 0x1aa3d4
	reallyRiskWarMsg     = 0x1aa3d0
	treatyLineFormat     = 0x17a034 // "%s" plus the line break
	defenseSelectionText = 0x45     // E string: "%s select combat at %s"
)

// defenseSelectionPopup is Defense_Colony_Selection_Popup_ (0xc9676): the
// system display of star for player p; -1 (close), a ship owner, or 15 +
// a colony.
func (g *Game) defenseSelectionPopup(star, p int) (int16, error) {
	e, mp := g.e, &g.mp
	mp.lastScanned = -1
	mp.playerN = p
	g.state.PlayerNum = int16(p)
	mp.sysDispStar = star
	e.Text().SetStyleShadow(1, font.Colors{0x9f, 0xa0, 0xa1, 0xa2}, font.ShadowDown)
	bg, err := g.cached(buffer0LBX, 0x49)
	if err != nil {
		return 0, err
	}
	if err := g.doPopupSetup(star, bg); err != nil {
		return 0, err
	}
	e.DeactivateAutoFunction()
	e.AssignAutoFunction(g.drawDefenseSelectionPopup, 1)
	g.ericPushFields()
	g.clearSysDispFields()
	if err := g.addDefenseSelectionFields(); err != nil {
		return 0, err
	}
	mp.showing = "defense"
	defer func() { mp.showing = "" }()
	if err := g.drawDefenseSelectionPopup(); err != nil {
		return 0, err
	}
	if err := e.TogglePages(); err != nil {
		return 0, err
	}
	result, done := int16(1), false
	for !done {
		if err := e.ReleaseTime(2); err != nil {
			return 0, err
		}
		e.MarkTime()
		in, err := e.GetInput()
		if err != nil {
			return 0, err
		}
		sc, err := e.ScanInput()
		if err != nil {
			return 0, err
		}
		if int16(in) == int16(mp.closeField) {
			done, result = true, -1
		} else if result, err = g.checkDefenseSelection(in, sc, &done); err != nil {
			return 0, err
		}
		if err := g.drawDefenseSelectionPopup(); err != nil {
			return 0, err
		}
		if err := e.TogglePages(); err != nil {
			return 0, err
		}
	}
	g.doPopupCleanup()
	return result, nil
}

// addDefenseSelectionFields is Add_Defense_Selection_Fields_ (0xc9335):
// the system display's fields with its close button.
func (g *Game) addDefenseSelectionFields() error {
	mp := &g.mp
	if err := g.initSystemDisplay(mp.sysDispStar); err != nil {
		return err
	}
	var drag, whole int
	return g.addSystemDisplayPopupFields(mp.sysDispStar, mp.x, mp.y, 0, 0, &drag, &mp.closeField, &whole)
}

// drawDefenseSelectionPopup is Draw_Defense_Selection_Popup_ (0xc9342).
func (g *Game) drawDefenseSelectionPopup() error {
	e, mp, s := g.e, &g.mp, &g.state
	if mp.fullDraw || g.fullDrawScreen {
		if err := g.mainpupsTestedExecuteAutofunction(); err != nil {
			return err
		}
		e.V.CopyOffToBack()
		mp.fullDraw = false
	}
	title := cSprintf(g.eStr(defenseSelectionText), save.CString(s.Players[mp.playerN].RaceName[:]),
		save.CString(s.Stars[mp.sysDispStar].Name[:]))
	return g.drawSystemDisplayPopup(mp.sysDispStar, true, title, mp.x, mp.y, 0, 0)
}

// resetDefenseSelection is the field and drawing reset after a message.
func (g *Game) resetDefenseSelection() error {
	g.mp.fullDraw = true
	g.ericPopFields()
	g.ericPushFields()
	g.clearSysDispFields()
	if err := g.addDefenseSelectionFields(); err != nil {
		return err
	}
	g.e.V.CopyBackToOff()
	if err := g.drawDefenseSelectionPopup(); err != nil {
		return err
	}
	g.e.AssignAutoFunction(g.drawDefenseSelectionPopup, 1)
	return nil
}

// checkDefenseSelection is Check_System_Display_Fields_Defense_Selection_
// (0xc93b5): a colony of another player, or a fleet of a player without a
// colony at the star, is the target.
func (g *Game) checkDefenseSelection(in, sc int, done *bool) (int16, error) {
	s, mp, sd := &g.state, &g.mp, &g.sd
	ret, _, clkP, hovS, clkS, icon := g.scanSystemDisplayFields(sc, in)
	sd.shipScanned = int16(hovS)
	if g.fullDrawScreen {
		if err := g.resetDefenseSelection(); err != nil {
			return 0, err
		}
	}
	if icon > -1 {
		sd.iconScanned = int16(icon)
	}
	if ret == 0 && clkP > -1 {
		pl := s.Stars[mp.sysDispStar].Planets[clkP]
		P := &s.Planets[pl]
		msg := 0
		switch {
		case P.Colony == -1 && P.Type == 3:
			msg = 0x250
		case P.Colony == -1:
			msg = 0x24f
		case int(s.Colonies[P.Colony].Owner) != mp.playerN:
			*done = true
			return P.Colony + 0xf, nil
		default:
			msg = 0x252
		}
		if err := g.messageBox(g.eStr(msg)); err != nil {
			return 0, err
		}
		if err := g.resetDefenseSelection(); err != nil {
			return 0, err
		}
	}
	if ret == 0 && clkS > -1 {
		o := int16(s.Ships[g.stacks.node[clkS].ship].Owner)
		msg := 0x253
		if int(o) != mp.playerN {
			if !g.playerHasColonyOrOutpostAtStar(int(o), mp.sysDispStar) {
				*done = true
				return o, nil
			}
			msg = 0x24e
		}
		if err := g.messageBox(g.eStr(msg)); err != nil {
			return 0, err
		}
		if err := g.resetDefenseSelection(); err != nil {
			return 0, err
		}
	}
	return -1, nil
}

// playerHasColonyOrOutpostAtStar is Player_Has_Colony_Or_Outpost_At_Star_
// (0xc6ebf).
func (g *Game) playerHasColonyOrOutpostAtStar(p, star int) bool {
	s := &g.state
	for _, pl := range s.Stars[star].Planets {
		if pl == -1 {
			continue
		}
		if c := s.Planets[pl].Colony; c != -1 && int(s.Colonies[c].Owner) == p {
			return true
		}
	}
	return false
}

// attackConfirmationText is the message Human_Confirms_Attack_ (0xe7b79)
// builds: the treaties p has with q, then the question.
func (g *Game) attackConfirmationText(p, q int16) (string, error) {
	s := &g.state
	line := cStringAt(g.exe, treatyLineFormat)
	str := g.combfindString
	t, err := str(youHaveTreatiesMsg)
	if err != nil {
		return "", err
	}
	msg := cSprintf(line, t)
	P := &s.Players[p]
	if k := P.TreatyStatus[q]; k >= 1 && k <= 3 {
		l, err := g.eStringAt(treatyLabels + 4*uint32(k))
		if err != nil {
			return "", err
		}
		msg += cSprintf(line, l)
	}
	for _, c := range []struct {
		on   bool
		slot uint32
	}{{P.TradeTreaty[q] != 0, tradeTreatyMsg}, {P.ResearchTreaty[q] != 0, researchTreatyMsg},
		{s.Players[q].Tribute[p] != 0, receivingTributeMsg}} {
		if c.on {
			l, err := str(c.slot)
			if err != nil {
				return "", err
			}
			msg += cSprintf(line, l)
		}
	}
	r, err := str(reallyRiskWarMsg)
	if err != nil {
		return "", err
	}
	return msg + r, nil
}

// combfindString is the RSTRING string Load_Combfind_Strings_ (0xe6828)
// points slot at: its ten slots take the strings after COLCALC's 60.
func (g *Game) combfindString(slot uint32) (string, error) {
	slots := []uint32{0x1aa3c4, 0x1aa3cc, 0x1aa3c8, 0x1aa3c0, 0x1aa3d4, 0x1aa3d0, 0x1aa3d8, 0x1aa3b8, 0x1aa3bc, 0x1aa3dc}
	rs, err := g.rStrings()
	if err != nil {
		return "", err
	}
	for k, a := range slots {
		if a == slot && 60+k < len(rs) {
			return rs[60+k], nil
		}
	}
	return "", fmt.Errorf("no COMBFIND string at %#x", slot)
}

// playerAttackedPopup is Player_Attacked_Popup_ (0xc97db): the human
// defender learns who attacks at star (or colony) in an exploding message
// box.
func (g *Game) playerAttackedPopup(att, def, star, colony int16) error {
	e := g.e
	s := &g.state
	saved := e.GetAutoFunction()
	place := save.CString(s.Stars[star].Name[:])
	if colony != -1 {
		place = g.planetName(int(s.Colonies[colony].Planet))
	}
	italian := g.settings[0xd2] == 4
	var who, whom string
	if att < 8 {
		f := g.eStr(0x3a)
		if italian {
			f = cStringAt(g.exe, 0x179e2b)
		}
		who = cSprintf(f, save.CString(s.Players[att].RaceName[:]))
	} else {
		n := map[int16]int{8: 0x1ff, 9: 0x202, 10: 0x9f, 0xb: 0xa0, 0xc: 0xa1, 0xd: 0xa2, 0xe: 0xa3}[att]
		if n == 0 {
			n = 0xa4
		}
		who = g.eStr(n)
	}
	if def < 8 {
		whom = cSprintf(g.eStr(0x46), save.CString(s.Players[def].RaceName[:]))
	} else {
		whom = g.eStr(0xa5)
	}
	var msg string
	switch {
	case italian:
		msg = cSprintf(cStringAt(g.exe, 0x179e2e), place, who, whom)
	case colony == -1:
		msg = cSprintf(g.eStr(0x31), who, whom, place)
	default:
		msg = cSprintf(g.eStr(0x30), who, whom, place)
	}
	// Set_Player_Attacked_Popup_Help_List_ (0xcdbc4).
	e.DeactivateHelpList()
	e.SetHelpList(g.helpListAt(playerAttackedHelpList, 1))
	if err := g.messageBoxExploding(msg); err != nil {
		return err
	}
	e.DeactivateHelpList()
	e.DeactivateAutoFunction()
	e.PutAutoFunction(saved)
	return nil
}
