package game

import (
	"fmt"

	"github.com/openorion2/openorion2/internal/engine"
	"github.com/openorion2/openorion2/internal/font"
	"github.com/openorion2/openorion2/internal/save"
)

// The human's audience with an AI ambassador (DIP-SCRN.C): Diplomacy_Screen_
// from AUDIENCE on the races screen and its menus. Spec:
// docs/screens/diplomacy-screen.md, "The player's diplomacy".

// diplomacyTexts are the menus Load_Diplomacy_Text_Messages_ (0x1f37c) loads.
type diplomacyTexts struct {
	mainTitle      string     // _main_diplomacy_title
	main           [9]string  // _main_diplomacy_list
	repulsive      [5]string  // _main_repulsive_diplomacy_list
	repulsiveNet   [6]string  // 0x19a0e8: the repulsive list between humans
	netList        [11]string // _main_net_diplomacy_list
	listenOption   string     // _net_listen_mode_option_buffer
	listenGoodBye  string     // _net_listen_mode_good_bye_buffer
	enterMessage   string     // _enter_your_message_buffer
	proposeTitle   string     // _propose_treaty_title
	propose        [7]string  // _propose_treaty_diplomacy_list
	breakTitle     string     // _break_treaty_title
	breakList      [7]string  // _break_treaty_diplomacy_list
	demandTitle    string     // _demand_title
	demand         [11]string // _demand_diplomacy_list
	offerTitle     string     // _offer_title
	offer          [7]string  // _offer_diplomacy_list
	agree          [3]string  // _agree_diplomacy_list
	chooseSystem   string     // _choose_a_system_buffer
	systemCancel   string     // _system_cancel_buffer
	millionColon   string     // _system_million_colonists_buffer
	homeworld      string     // _system_homeworld_buffer
	xStarSystem    string     // _system_x_star_system_buffer
	unsurrenderabl string     // _system_unsurrenderable_buffer
}

// diploRowKeys are the list rows' hot keys under Options.DiploKeys (tests).
const diploRowKeys = "1234567890QWERTYUIOP"

// Help entry tables in .data (i16 ids, one per row).
const (
	mainDiplomacyHelp      = 0x180de4
	repulsiveDiplomacyHelp = 0x180e0c
	proposeTreatyHelp      = 0x180e22
	breakTreatyHelp        = 0x180e30
	demandDiplomacyHelp    = 0x180e3e
	offerDiplomacyHelp     = 0x180e54
	diplomacyHelpScreen    = 0x2b6
	diplomacyListX         = 10
	diplomacyListY         = 0x76
	diplomacyListW         = 0xf5
	diplomacyListFill      = 0x11
	diplomacyMaxListRows   = 15
)

// loadDiplomacyTexts is the menu part of Load_Diplomacy_Text_Messages_
// (0x1f37c); the lengths are the buffers' sizes.
func (g *Game) loadDiplomacyTexts() error {
	t := &g.dip.texts
	var err error
	get := func(file string, n, max int) string {
		if err != nil {
			return ""
		}
		var s string
		s, err = g.textMessage(file, n, max)
		return s
	}
	t.mainTitle = get(jimText, 0x1e, 0x1e)
	for i := range 8 {
		t.main[i] = get(jimText, 0x1f+i, 0x1e)
	}
	t.main[8] = ""
	t.repulsive = [5]string{get(jimText, 0x2b, 0x1e), t.main[5], t.main[6], t.main[7], ""}
	for i := range 7 {
		t.netList[i] = t.main[i]
	}
	t.netList[7] = get(jimText2, 0x41, 0x28)
	t.netList[8] = get(jimText, 0x27, 0x28)
	t.netList[9], t.netList[10] = t.main[7], ""
	t.repulsiveNet = [6]string{t.repulsive[0], t.main[5], t.main[6], t.netList[7], t.main[7], ""}
	t.listenOption = get(jimText2, 0xa, 0x14)
	t.listenGoodBye = get(jimText2, 0xb, 0x14)
	t.enterMessage = get(jimText2, 0x47, 0x28)
	t.proposeTitle = get(jimText, 0x28, 0x28)
	for i := range 6 {
		t.propose[i] = get(jimText, 0x29+i, 0x1e)
	}
	t.breakTitle = get(jimText, 0x2f, 0x1e)
	for i := range 5 {
		t.breakList[i] = get(jimText, 0x30+i, 0x28)
	}
	t.breakList[5] = get(jimText, 0x2e, 0x28)
	t.demandTitle = get(jimText, 0x35, 0x1e)
	for i := range 9 {
		t.demand[i] = get(jimText, 0x36+i, 0x28)
	}
	t.demand[9] = get(jimText, 0x2e, 0x23)
	t.offerTitle = get(jimText, 0x3f, 0x1e)
	for i := range 5 {
		t.offer[i] = get(jimText, 0x40+i, 0x1e)
	}
	t.offer[5] = get(jimText, 0x2e, 0x1e)
	t.agree[0], t.agree[1] = get(jimText, 0x45, 0x14), get(jimText, 0x46, 0x14)
	t.chooseSystem = get(jimText2, 2, 0x28)
	t.systemCancel = get(jimText, 0x2e, 0x14)
	t.millionColon = get(jimText2, 0x38, 0x1e)
	t.homeworld = get(jimText2, 0x39, 0x1e)
	t.xStarSystem = get(jimText2, 0x3a, 0x23)
	t.unsurrenderabl = get(jimText2, 0x46, 0x28)
	return err
}

// diplomacyScreen is Diplomacy_Screen_ (0x16c4e), the human's audience with
// the ambassador of amb, reached from AUDIENCE on the races screen.
func (g *Game) diplomacyScreen(amb int) error {
	e, s, d := g.e, &g.state, &g.dip
	pn := int(s.PlayerNum)
	d.currentMusic = -1
	d.net.synchUp = 0
	e.DeactivateHelpList()
	e.EnableDrawAllFields()
	e.V.ResetWindow()
	e.V.ClipOff()
	e.SetMouseList(engine.MouseNone)
	e.DeactivateAutoFunction()
	e.ClearFields()
	d.oldResponse = -1
	d.net.escPressed = 0
	d.ambassador = amb
	e.SetFadePercent(0)
	e.SetFadeSpeedPercent(10)
	if err := e.FastFadeOut(); err != nil {
		return err
	}
	e.SetInputDelay(1)
	e.ClearFields()
	e.V.SetRefreshStencil()
	e.DeactivateAutoFunction()
	P := &s.Players[pn]
	if g.findWorstModifier(pn, amb) <= -100 || s.CouncilFlag == 2 || s.CouncilFlag == 3 {
		d.response = 0x7e
		if int8(P.TreatyStatus[amb]) < 4 {
			d.response++
		}
		d.option = 0
	} else {
		d.response = 0x80
		if P.Relations[amb] < -12 {
			d.response++
		}
		d.option = 1
	}
	human := s.Players[amb].Objective == objectiveHuman
	if !human && g.gameType >= 2 {
		d.option = 1 // in network games an AI always listens
	}
	var err error
	if g.gameType >= 2 && human {
		// Two humans: the talker and the listener (docs/net/diplomacy.md).
		n := &d.net
		if g.net.u8(netDipFirst) == 1 {
			n.talker, n.listener = uint8(pn), g.net.u8(netTalkTo)
		} else {
			n.talker, n.listener = g.net.u8(netTalkTo), uint8(pn)
		}
		d.option = 1
		n.netChat[0] = 0
		if err := g.diplomacyFadeIn(); err != nil {
			return err
		}
		err = g.getNetDiplomacyChoices()
	} else {
		if err := g.diplomacyFadeIn(); err != nil {
			return err
		}
		switch {
		case d.option != 1:
			err = g.diplomacyDisplayResponse()
		case P.Picks[19] == 1 || s.Players[amb].Picks[19] == 1:
			err = g.getMainRepulsiveDiplomacyChoices(pn, amb, false)
		default:
			err = g.getMainDiplomacyChoices()
		}
	}
	if err != nil {
		return err
	}
	g.mus.current = -1 // 0x16ea0: _jim_current_stream
	g.playBackgroundMusic()
	e.DeactivateAutoFunction()
	e.ClearFields()
	e.SetFadePercent(0)
	e.SetFadeSpeedPercent(10)
	return e.FastFadeOut()
}

// diplomacyListStyle sets the font the menus print in: style 4 in the
// diplomacy colours, four pixels of line feed. Outline uses
// Set_Font_Style_Outline_ with outline colour (0, 0xf8), as the main menus do.
func (g *Game) diplomacyListStyle(outline bool) {
	t := g.e.Text()
	if outline {
		t.SetOutlineColor(0, 0xf8)
		t.SetStyleShadow(4, g.exeColors(diplomacyNormalColors), font.Outline)
	} else {
		t.SetStyle(4, g.exeColors(diplomacyNormalColors))
	}
	t.SetHighlightColors(g.exeColors(diplomacyHighlightColors))
	t.SetSpecialColors(g.exeColors(diplomacySpecialColors))
	t.SetLF(4)
}

// diplomacyList runs Get_List_Field over the menu at (10, 0x76), width 0xf5,
// with the row variable starting at 0; nil active enables every row.
func (g *Game) diplomacyList(title string, rows []string, active []byte) (int, error) {
	var v int16
	if active == nil {
		active = make([]byte, len(rows))
		for i := range active {
			active[i] = 1
		}
	}
	c, err := g.e.GetListField(diplomacyListX, diplomacyListY, diplomacyListW, title, rows, &v, active, diplomacyListFill, [4]int{})
	return int(int16(c)), err
}

// setupDiplomacyHelp is Setup_Diplomacy_Help_ (0x1fc13): help for the n-1
// rows from ids and, last, the whole screen.
func (g *Game) setupDiplomacyHelp(n int, ids []int16) {
	e := g.e
	e.DeactivateHelpList()
	if g.netHumanAmbassador() {
		return
	}
	t := e.Text()
	fh := t.Height()
	h := fh + t.VSpacing()
	list := make([]engine.HelpEntry, 0, n)
	y := diplomacyListY + h
	for i := 0; i < n-1; i++ {
		list = append(list, engine.HelpEntry{ID: int(ids[i]), X1: 0xa, Y1: y, X2: 0xff, Y2: y + fh})
		y += h
	}
	list = append(list, engine.HelpEntry{ID: diplomacyHelpScreen, X1: 0, Y1: 0, X2: 0x27f, Y2: 0x1df})
	e.SetHelpList(list)
}

// netHumanAmbassador: the help lists are off when two humans talk (a test
// both Setup_*_Diplomacy_Help_ make).
func (g *Game) netHumanAmbassador() bool {
	return g.gameType >= 2 && g.state.Players[g.dip.ambassador].Objective == objectiveHuman
}

// helpIDs reads n help ids from a table in .data.
func (g *Game) helpIDs(table uint32, n int) []int16 {
	ids := make([]int16, n)
	for i := range ids {
		ids[i] = g.exeTableI16(table, i)
	}
	return ids
}

// setupFullScreenDiplomacyHelp is Setup_Full_Screen_Diplomacy_Help_ (0x1fd06).
func (g *Game) setupFullScreenDiplomacyHelp(id int) {
	g.e.DeactivateHelpList()
	if g.netHumanAmbassador() {
		return
	}
	g.e.SetHelpList([]engine.HelpEntry{{ID: id, X1: 0, Y1: 0, X2: 0x27f, Y2: 0x1df}})
}

// getMainDiplomacyChoices is Get_Main_Diplomacy_Choices_ (0x1758c): the
// main menu, again after each action, until Good Bye, a war, a surrender
// or the ambassador's patience ends.
func (g *Game) getMainDiplomacyChoices() error {
	e, s, d := g.e, &g.state, &g.dip
	pn, amb := int(s.PlayerNum), d.ambassador
	if s.Players[amb].Objective != objectiveHuman { // not for a human ambassador
		g.getExchangeTechList(pn, amb)
	}
	for done := false; !done; {
		active := []byte{1, 1, 1, 1, 1, 1, 1, 1, 1}
		g.diplomacySetMainOptions(pn, amb, active)
		if s.Stars[0].Name[14]&4 != 0 {
			active[4] = 0 // the 1.31 patch (0x1692a2) turns off Exchange Tech
		}
		if g.findWorstModifier(pn, amb) <= -100 {
			if d.response != d.oldResponse {
				if err := g.diplomacyDisplayResponse(); err != nil {
					return err
				}
			}
			d.response = 0x82
			done = true
			if err := g.diplomacyDisplayResponse(); err != nil {
				return err
			}
			continue
		}
		e.AssignAutoFunction(g.drawDiplomacySynchMode, 2)
		e.SetInputDelay(1)
		g.diplomacyListStyle(true)
		g.setupDiplomacyHelp(9, g.helpIDs(mainDiplomacyHelp, 9))
		c, err := g.diplomacyList(d.texts.mainTitle, d.texts.main[:], active)
		if err != nil {
			return err
		}
		switch c {
		case -1, 7:
			done = true
		case 0:
			d.message = ""
			err = g.diplomacyProposeTreaty(pn, amb)
		case 1:
			d.message = ""
			err = g.diplomacyBreakTreaty(pn, amb)
		case 2:
			d.message = ""
			err = g.diplomacyDemand(pn, amb)
		case 3:
			d.message = ""
			err = g.diplomacyOfferGift(pn, amb)
		case 4:
			d.message = ""
			err = g.diplomacyExchangeTechnology(pn, amb)
		case 5:
			var war bool
			if war, err = g.diplomacyDeclareWarResolution(pn, amb); err == nil && war {
				done = true
				err = g.diplomacyDisplayResponse()
			}
		case 6:
			var gone bool
			if gone, err = g.diplomacySurrender(pn, amb); err == nil && gone {
				done = true
			}
		}
		if err != nil {
			return err
		}
	}
	e.AssignAutoFunction(g.drawDiplomacySynchMode, 2)
	e.SetInputDelay(1)
	return nil
}

// diplomacySetMainOptions is Diplomacy_Set_Main_Options_ (0x17847): which
// main menu rows p may choose with amb.
func (g *Game) diplomacySetMainOptions(p, amb int, a []byte) {
	s, d := &g.state, &g.dip
	P := &s.Players[p]
	if int8(P.TreatyStatus[amb]) >= 4 {
		a[1], a[5], a[2], a[4] = 0, 0, 0, 0
	} else {
		if s.Players[amb].Objective == objectiveHuman {
			// 0x17889: what p may ask of a human amb, if amb has anything p lacks.
			d.targetTech = d.targetTech[:0]
			if l, _ := g.getDifferentialTechList(amb, p, 0, 0xc343cb00, true); len(l) > 0 {
				d.targetTech, _ = g.getDifferentialTechList(p, amb, 0, 0xc343cb00, true)
			}
		}
		a[4] = 0
		if len(d.targetTech) > 0 {
			a[4] = 1
		}
	}
	a[1] = 0
	if P.TradeTreaty[amb] == 1 || P.ResearchTreaty[amb] == 1 || P.Tribute[amb] > 0 ||
		P.TreatyStatus[amb] == 1 || P.TreatyStatus[amb] == 2 {
		a[1] = 1
	}
	if !g.validTreatyProposal(p, amb, 7) && !g.validTreatyProposal(p, amb, 8) &&
		!g.validTreatyProposal(p, amb, 3) && !g.validTreatyProposal(p, amb, 1) &&
		!g.validTreatyProposal(p, amb, 2) {
		a[0] = 0
	}
	a[6] = 0
	if s.maySurrenderEmpire(int16(p), int16(amb)) {
		a[6] = 1
	}
}

// getMainRepulsiveDiplomacyChoices is Get_Main_Repulsive_Diplomacy_Choices_
// (0x17227): a repulsive race only talks peace, war or surrender; between
// two humans (net) also chat, and Good Bye is sent (code 0x82).
func (g *Game) getMainRepulsiveDiplomacyChoices(p, amb int, net bool) error {
	e, s, d := g.e, &g.state, &g.dip
	for done := false; !done; {
		active := []byte{1, 1, 1, 1, 1, 1}
		if int8(s.Players[p].TreatyStatus[amb]) >= 4 {
			active[1] = 0
		} else {
			active[0] = 0
		}
		active[2] = 0
		if s.maySurrenderEmpire(int16(p), int16(amb)) {
			active[2] = 1
		}
		if !net && g.findWorstModifier(p, amb) <= -100 {
			if d.response != d.oldResponse {
				if err := g.diplomacyDisplayResponse(); err != nil {
					return err
				}
			}
			d.response = 0x82
			done = true
			if err := g.diplomacyDisplayResponse(); err != nil {
				return err
			}
			continue
		}
		if d.net.escPressed == 1 {
			done = true
		}
		e.AssignAutoFunction(g.drawDiplomacySynchMode, 2)
		e.SetInputDelay(1)
		g.diplomacyListStyle(true)
		var c int
		var err error
		if net {
			g.setupDiplomacyHelp(6, g.helpIDs(0x180e16, 6))
			c, err = g.diplomacyList(d.texts.mainTitle, d.texts.repulsiveNet[:], active)
			if c == 3 {
				c = 5
			}
		} else {
			g.setupDiplomacyHelp(5, g.helpIDs(repulsiveDiplomacyHelp, 5))
			c, err = g.diplomacyList(d.texts.mainTitle, d.texts.repulsive[:], active[:5])
		}
		if err != nil {
			return err
		}
		switch c {
		case 0:
			d.message = ""
			d.oldResponse = -1
			ok, err := g.diplomacyDetermineTreatyProposal(p, amb, 3)
			if err != nil {
				return err
			}
			g.adjustDiplomatModifiers(int16(p), int16(amb))
			if ok {
				g.declarePeace(int16(p), int16(amb))
			}
		case 1:
			war, err := g.diplomacyDeclareWarResolution(p, amb)
			if err != nil {
				return err
			}
			if war {
				done = true
				if err := g.diplomacyDisplayResponse(); err != nil {
					return err
				}
				d.net.escPressed = 1
			}
		case 2:
			gone, err := g.diplomacySurrender(p, amb)
			if err != nil {
				return err
			}
			if gone {
				done = true
				d.net.escPressed = 1
			}
		case 5:
			if err := g.diplomacyMainNetChat(amb, p); err != nil {
				return err
			}
			if d.net.escPressed == 1 {
				done = true
			}
		case -1, 3, 4:
			done = true
			if net {
				n := &d.net
				n.set(uint8(p), uint8(amb))
				n.escPressed = 1
				n.setCode(netDipExit)
				n.setB(-1)
				g.netDipPost(amb)
			}
		}
	}
	e.AssignAutoFunction(g.drawDiplomacySynchMode, 2)
	e.SetInputDelay(1)
	return nil
}

// diplomacyGenericList is Diplomacy_Generic_List_ (0x19de8): up to 8 rows
// and Cancel; without Cancel the last row cannot be chosen (it gives -1,
// as in the original).
func (g *Game) diplomacyGenericList(items []string, cancel bool, title string) (int, error) {
	e := g.e
	k := min(len(items), 8)
	active := make([]byte, 10)
	for i := range active {
		active[i] = 1
	}
	e.ClearFields()
	e.AssignAutoFunction(g.drawDiplomacySynchMode, 2)
	g.setupFullScreenDiplomacyHelp(diplomacyHelpScreen)
	e.SetInputDelay(1)
	rows := make([]string, k+2)
	copy(rows, items[:k])
	if cancel {
		s, err := g.textMessage(jimText, 0x2e, 0x32)
		if err != nil {
			return 0, err
		}
		rows[k] = s
	} else {
		k--
	}
	g.diplomacyListStyle(false)
	c, err := g.diplomacyList(title, rows, active)
	if err != nil {
		return 0, err
	}
	if c == -1 || c == k {
		return -1, nil
	}
	return c, nil
}

// diplomacyDeclareWarResolution is Diplomacy_Declare_War_Resolution_
// (0x179f4): "REALLY DECLARE WAR?!"; true when declared.
func (g *Game) diplomacyDeclareWarResolution(p, amb int) (bool, error) {
	g.setupFullScreenDiplomacyHelp(0x266)
	row, err := g.textMessage(jimText2, 0x3b, 0x28)
	if err != nil {
		return false, err
	}
	title, err := g.textMessage(jimText2, 0x3c, 0x50)
	if err != nil {
		return false, err
	}
	c, err := g.diplomacyGenericList([]string{row}, true, title)
	if err != nil || c == -1 {
		return false, err
	}
	if g.state.Players[amb].Objective == objectiveHuman {
		if g.gameType != 1 {
			g.netTell(p, amb, netDipWar, 0)
		} else if err := g.humanResponseMessage(p, amb, p, 0xe, false); err != nil {
			return false, err
		}
		g.dip.response = 0xa0
	} else {
		g.dip.response = g.getDeclareWarMessage(p, amb)
	}
	g.declareWar(int16(p), int16(amb))
	g.adjustDiplomatModifiers(int16(p), int16(amb))
	return true, nil
}

// diplomacySurrender is Diplomacy_Surrender_ (0x1d0f8): p gives its empire
// to amb.
func (g *Game) diplomacySurrender(p, amb int) (bool, error) {
	s := &g.state
	g.setupFullScreenDiplomacyHelp(0x264)
	row, err := g.textMessage(jimText2, 6, 0x28)
	if err != nil {
		return false, err
	}
	title, err := g.textMessage(jimText2, 7, 0x50)
	if err != nil {
		return false, err
	}
	c, err := g.diplomacyGenericList([]string{row}, true, title)
	if err != nil || c == -1 {
		return false, err
	}
	if s.maySurrenderEmpire(int16(p), int16(amb)) {
		s.surrenderEmpire(int16(p), int16(amb))
	}
	if s.Players[amb].Objective == objectiveHuman {
		if g.gameType != 1 {
			g.netTell(p, amb, netDipSurrender, 0)
			return true, nil
		}
		return true, g.humanResponseMessage(p, amb, p, 0xd, false)
	}
	return true, nil
}

// diplomacyProposeTreaty is Diplomacy_Propose_Treaty_ (0x17d5b).
func (g *Game) diplomacyProposeTreaty(p, amb int) error {
	e, s, d := g.e, &g.state, &g.dip
	active := []byte{0, 0, 0, 0, 0, 1, 1}
	for i, k := range []uint16{7, 8, 3, 1, 2} {
		if g.validTreatyProposal(p, amb, k) {
			active[i] = 1
		}
	}
	e.AssignAutoFunction(g.drawDiplomacySynchMode, 2)
	e.SetInputDelay(1)
	g.diplomacyListStyle(false)
	g.setupDiplomacyHelp(7, g.helpIDs(proposeTreatyHelp, 7))
	c, err := g.diplomacyList(d.texts.proposeTitle, d.texts.propose[:], active)
	if err != nil || c < 0 || c > 4 {
		return err
	}
	kinds := []uint16{7, 8, 3, 1, 2}
	mods := []int{1, 1, 3, 0, 0}
	d.oldResponse = -1
	ok, err := g.diplomacyDetermineTreatyProposal(p, amb, kinds[c])
	if err != nil {
		return err
	}
	P, A := int16(p), int16(amb)
	g.adjustDiplomatModifiers(P, A)
	s.Players[p].Modifiers[mods[c]][amb] -= 20
	if ok {
		switch c {
		case 0:
			g.startTradeTreaty(P, A)
		case 1:
			g.startResearchTreaty(P, A)
		case 2:
			g.declarePeace(P, A)
		case 3:
			g.startTreaty(P, A, 1)
		case 4:
			g.startTreaty(P, A, 2)
		}
		if s.Players[amb].Objective != objectiveHuman { // not for a human ambassador
			g.getExchangeTechList(p, amb)
		}
	}
	return nil
}

// diplomacyDetermineTreatyProposal is Diplomacy_Determine_Treaty_Proposal_
// (0x19997): amb's answer to the treaty, maybe after a price is agreed.
func (g *Game) diplomacyDetermineTreatyProposal(p, amb int, kind uint16) (bool, error) {
	d := &g.dip
	old := d.response
	var off diplomacyOffer
	res := false
	var c int
	if g.state.Players[amb].Objective == objectiveHuman {
		var err error
		if c, err = g.checkHumanTreatyProposal(p, amb, kind); err != nil {
			return false, err
		}
	} else {
		c = int(g.checkTreatyProposal(int16(amb), int16(p), kind, &d.response, &off))
	}
	switch c {
	case 0:
	case 3:
		res = true
	default:
		var ok bool
		var err error
		if g.state.Players[amb].Objective == objectiveHuman {
			ok, err = g.diplomacyHumanNeedBetterOffer(p, amb)
		} else {
			ok, err = g.diplomacyNeedBetterOffer(off)
		}
		if err != nil {
			return false, err
		}
		if ok {
			res = true
			switch kind {
			case 1:
				d.response = 0x83
			case 2:
				d.response = 0x84
			case 3:
				d.response = 0x87
			case 7:
				d.response = 0x85
			case 8:
				d.response = 0x86
			}
		} else {
			d.response = 0x8a
		}
	}
	if g.state.Players[amb].Objective == objectiveHuman {
		if err := g.getHumanFormTreatyMessage(kind, res); err != nil {
			return false, err
		}
	} else {
		g.npcMessageRejectionCheck(amb, p, &d.response)
	}
	if d.response == old {
		d.response = 0x8a
	}
	return res, nil
}

// diplomacyNeedBetterOffer is Diplomacy_Need_Better_Offer_ (0x19b78): the
// ambassador names a price; true when the human agrees (and pays).
func (g *Game) diplomacyNeedBetterOffer(off diplomacyOffer) (bool, error) {
	e, s, d := g.e, &g.state, &g.dip
	n := 0x4b
	if s.Random.Random(2) == 1 {
		n = 0x4a
	}
	title, err := g.textMessage(jimText, n, 0x28)
	if err != nil {
		return false, err
	}
	e.ClearFields()
	e.AssignAutoFunction(g.drawDiplomacySynchMode, 2)
	e.DeactivateHelpList()
	e.SetInputDelay(1)
	g.diplomacyListStyle(false)
	c, err := g.diplomacyList(title, d.texts.agree[:], []byte{1, 1, 1})
	if err != nil {
		return false, err
	}
	if c == -1 || c == 1 {
		return false, nil
	}
	pn, amb := int(s.PlayerNum), d.ambassador
	P, A := &s.Players[pn], &s.Players[amb]
	switch off.kind {
	case 1:
		P.BC -= int32(off.gold)
		A.BC += int32(off.gold)
	case 2:
		if st := int(off.system); s.maySurrenderStar(st, pn, amb) {
			s.surrenderStar(st, pn, amb)
		}
	case 3:
		P.Tribute[amb] = off.tribute
	case 4:
		g.playerGetsTechApp(amb, int(off.tech))
	}
	return true, nil
}

// diplomacyBreakTreaty is Diplomacy_Break_Treaty_ (0x180d4).
func (g *Game) diplomacyBreakTreaty(p, amb int) error {
	e, s, d := g.e, &g.state, &g.dip
	P := &s.Players[p]
	active := make([]byte, 7)
	b := func(v bool) byte {
		if v {
			return 1
		}
		return 0
	}
	active[0] = b(P.TradeTreaty[amb] == 1)
	active[1] = b(P.ResearchTreaty[amb] == 1)
	active[2] = b(P.Tribute[amb] > 0)
	active[3] = b(P.TreatyStatus[amb] == 1)
	active[4] = b(P.TreatyStatus[amb] == 2)
	active[5] = 1
	e.AssignAutoFunction(g.drawDiplomacySynchMode, 2)
	e.SetInputDelay(1)
	g.diplomacyListStyle(false)
	g.setupDiplomacyHelp(7, g.helpIDs(breakTreatyHelp, 7))
	c, err := g.diplomacyList(d.texts.breakTitle, d.texts.breakList[:], active)
	if err != nil || c < 0 || c > 4 {
		return err
	}
	kind := []uint16{7, 8, 0xa, 1, 2}[c]
	if c == 2 && P.Tribute[amb] == 1 {
		kind = 9
	}
	d.oldResponse = -1
	return g.diplomacyBreakTreatyResolution(p, amb, kind)
}

// diplomacyBreakTreatyResolution is Diplomacy_Break_Treaty_Resolution_
// (0x18309) with an AI ambassador.
func (g *Game) diplomacyBreakTreatyResolution(p, amb int, kind uint16) error {
	d := &g.dip
	P, A := int16(p), int16(amb)
	human := g.state.Players[amb].Objective == objectiveHuman
	if human && g.gameType != 1 {
		g.netTell(p, amb, netDipBreak, int16(kind))
	} else if human {
		// Get_Human_Break_Treaty_Message_ names amb (0x1834d).
		n := map[uint16]int{7: 0x1b, 8: 0x1c, 9: 0x1d, 0xa: 0x1d, 1: 0x1e, 2: 0x1f}[kind]
		if n != 0 {
			if err := g.humanResponseMessage(p, amb, amb, n, true); err != nil {
				return err
			}
		} else if err := g.humanBreakTreatyNoMessage(p, amb); err != nil {
			return err
		}
	}
	switch kind {
	case 7:
		g.breakTrade(P, A)
		if human {
			d.response = 0x9c
		} else {
			d.response = g.getBreakAgreementMessage(p, amb, 0x9c)
		}
	case 8:
		g.breakResearch(P, A)
		if human {
			d.response = 0x9e
		} else {
			d.response = g.getBreakAgreementMessage(p, amb, 0x9e)
		}
	case 9, 0xa:
		g.breakTribute(P, A)
		if human {
			d.response = 0xab
		} else {
			d.response = g.getBreakTributeTreatyMessage(p, amb)
		}
	case 1, 2:
		g.breakTreaties(P, A)
		if human {
			d.response = 0x99
		} else {
			d.response = g.getBreakTreatyMessage(p, amb)
		}
	}
	return nil
}

// humanBreakTreatyNoMessage is the hotseat path of
// Diplomacy_Break_Treaty_Resolution_ for a kind without a message: the
// ambassadors still change and the response is cleared.
func (g *Game) humanBreakTreatyNoMessage(p, amb int) error {
	if g.gameType != 1 {
		return nil
	}
	if err := g.diplomacyLoadNewAmbassador(p); err != nil {
		return err
	}
	g.dip.response, g.dip.oldResponse = 0, 0
	if err := g.diplomacyDisplayResponse(); err != nil {
		return err
	}
	return g.diplomacyLoadNewAmbassador(amb)
}

// diplomacyOfferGift is Diplomacy_Offer_Gift_ (0x194c5).
func (g *Game) diplomacyOfferGift(p, amb int) error {
	e, s, d := g.e, &g.state, &g.dip
	P, A := &s.Players[p], &s.Players[amb]
	active := []byte{1, 1, 0, 0, 0, 1, 0}
	switch {
	case A.Tribute[p] > 0, P.Tribute[amb] > 0:
		active[0], active[1] = 0, 0
	}
	techs, _ := g.getDifferentialTechList(amb, p, 0, 0xc343cb00, true)
	if len(techs) > 0 {
		active[2] = 1
	}
	if g.checkSurrenderSystem(p, amb) {
		active[3] = 1
	}
	if P.BC > 100 {
		active[4] = 1
	}
	e.AssignAutoFunction(g.drawDiplomacySynchMode, 2)
	e.SetInputDelay(1)
	g.diplomacyListStyle(false)
	g.setupDiplomacyHelp(7, g.helpIDs(offerDiplomacyHelp, 7))
	c, err := g.diplomacyList(d.texts.offerTitle, d.texts.offer[:], active)
	if err != nil {
		return err
	}
	switch c {
	case 0, 1:
		d.oldResponse = -1
		g.startTributeTreaty(int16(p), int16(amb), int16(c+1))
		if s.Players[amb].Objective == objectiveHuman {
			return g.getHumanGiftResponse(p, amb, 2, int16(c+1))
		}
		d.response = g.getGiftResponse(p, amb, 2, int16(c+1))
		g.npcProposalRejectionAccept(amb, p, true)
	case 2:
		i, err := g.diplomacyGenericExchangeTechList(p, d.texts.offerTitle, techs, false)
		if err != nil {
			return err
		}
		if i == -1 {
			d.message = ""
			return nil
		}
		d.oldResponse = -1
		g.playerGetsTechApp(amb, int(techs[i]))
		d.response = g.getGiftResponse(p, amb, 3, int16(techs[i]))
		g.npcProposalRejectionAccept(amb, p, true)
		g.getExchangeTechList(p, amb)
	case 3:
		st, err := g.diplomacySystemDisplay(p, amb, false)
		if err != nil {
			return err
		}
		if st == -1 {
			d.message = ""
			return nil
		}
		d.oldResponse = -1
		if s.Players[amb].Objective == objectiveHuman {
			if err := g.getHumanGiftResponse(p, amb, 4, int16(st)); err != nil {
				return err
			}
		} else {
			d.response = g.getGiftResponse(p, amb, 4, int16(st))
			g.npcProposalRejectionAccept(amb, p, true)
		}
		if s.maySurrenderStar(st, p, amb) {
			s.surrenderStar(st, p, amb)
		}
	case 4:
		_, err = g.diplomacyOfferMoney(p, amb, true)
	}
	return err
}

// diplomacyOfferMoney is Diplomacy_Offer_Money_ (0x1d237): five sums up to
// p's treasury; with give set they go to amb. Returns the sum or -1.
func (g *Game) diplomacyOfferMoney(p, amb int, give bool) (int16, error) {
	s, d := &g.state, &g.dip
	g.setupFullScreenDiplomacyHelp(0x267)
	P, A := &s.Players[p], &s.Players[amb]
	active := make([]byte, 15)
	for i := range active {
		active[i] = 1
	}
	bc := min(P.BC, 32000)
	v := int16(bc / 20 * 20)
	var sums [5]int16
	n := 5
	if v < 100 {
		sums = [5]int16{20, 40, 60, 80, 100}
		n = int(v / 20)
	} else {
		w := int32(v)
		sums[0] = int16(w / 5 / 20 * 20)
		sums[1] = int16(int32(int16(uint32(2*w)/5)) / 20 * 20)
		sums[2] = int16(int32(int16(uint32(3*w)/5)) / 20 * 20)
		sums[3] = int16(int32(int16(uint32(4*w)/5)) / 20 * 20)
		sums[4] = int16(w / 20 * 20)
	}
	rows := make([]string, 15)
	for i := 0; i < n; i++ {
		rows[i] = fmt.Sprintf(cStringAt(g.exe, 0x178aa8), sums[i])
		if int32(sums[i]) > P.BC {
			active[i] = 0
		}
	}
	var err error
	if rows[n], err = g.textMessage(jimText, 0x2e, 0x28); err != nil {
		return -1, err
	}
	title, err := g.textMessage(jimText2, 8, 0x28)
	if err != nil {
		return -1, err
	}
	c, err := g.diplomacyList(title, rows, active)
	if err != nil || c < 0 || c >= n {
		return -1, err
	}
	sum := sums[c]
	if give {
		P.BC -= int32(sum)
		A.BC += int32(sum)
		d.oldResponse = -1
		if A.Objective == objectiveHuman {
			return sum, g.getHumanGiftResponse(p, amb, 1, sum)
		}
		d.response = g.getGiftResponse(p, amb, 1, sum)
		if int32(sum) >= (int32(s.Stardate)-35000)*5 {
			g.npcProposalRejectionAccept(amb, p, true)
		}
	}
	return sum, nil
}

// diplomacyGenericExchangeTechList is Diplomacy_Generic_Exchange_Tech_List_
// (0x19f26): a paged list of applications (8 a page with forgetIt, else 11)
// named with p's levels; the chosen index or -1.
func (g *Game) diplomacyGenericExchangeTechList(p int, title string, list []uint8, forgetIt bool) (int, error) {
	e := g.e
	page := 0xb
	if forgetIt {
		page = 8
	}
	n := len(list)
	e.ClearFields()
	e.AssignAutoFunction(g.drawDiplomacySynchMode, 2)
	e.SetInputDelay(1)
	g.diplomacyListStyle(false)
	prevText, err := g.textMessage(jimText, 0x4c, 0x28)
	if err != nil {
		return 0, err
	}
	nextText, err := g.textMessage(jimText, 0x4d, 0x28)
	if err != nil {
		return 0, err
	}
	last := 0x2e
	if forgetIt {
		last = 0x46
	}
	lastText, err := g.textMessage(jimText, last, 0x28)
	if err != nil {
		return 0, err
	}
	off, result := 0, -1
	for {
		rows := make([]string, page+4)
		help := make([]int16, page+4)
		prev, next := -1, -1
		rows[0], help[0] = prevText, -1
		shift := 0
		if off != 0 {
			shift, prev = 1, 0
		}
		var k int
		if off+page < n {
			k = page + shift
			next = k
		} else {
			k = n - off
		}
		for i := 0; i < k && off+i < n; i++ {
			name, err := g.technologyApplicationsName(int(list[off+i]), p)
			if err != nil {
				return 0, err
			}
			rows[i+shift] = fmt.Sprintf(cStringAt(g.exe, 0x178a8a), name)
			help[i+shift] = int16(list[off+i])
		}
		if next != -1 {
			rows[k], help[k] = nextText, -1
			k++
		}
		if prev == 0 && next == -1 {
			k++
		}
		rows[k], help[k] = lastText, 0x265
		rows[k+1] = ""
		e.Text().SetLF(4)
		g.setupDiplomacyHelp(k+2, help)
		c, err := g.diplomacyList(title, rows, nil)
		if err != nil {
			return 0, err
		}
		switch {
		case c == -1 || c == k:
			return -1, nil
		case c == prev:
			off -= page
		case c == next:
			off += page
		default:
			result = c + off
			if prev != -1 {
				result--
			}
			return result, nil
		}
	}
}

// diplomacyExchangeTechnology is Diplomacy_Exchange_Technology_ (0x1cb4d)
// with an AI ambassador: the human picks one of amb's applications, amb
// names its price, the human agrees or not.
func (g *Game) diplomacyExchangeTechnology(p, amb int) error {
	s, d := &g.state, &g.dip
	agree := []string{d.texts.agree[0], d.texts.agree[1], d.texts.agree[2]}
	if len(d.targetTech) < 1 {
		return nil
	}
	title, err := g.textMessage(jimText2, 3, 0x28)
	if err != nil {
		return err
	}
	i, err := g.diplomacyGenericExchangeTechList(p, title, d.targetTech, false)
	if err != nil || i == -1 {
		return err
	}
	want := d.targetTech[i]
	if s.Players[amb].Objective == objectiveHuman {
		if g.gameType != 1 {
			return g.netTechExchange(p, amb, int(want))
		}
		return g.humanTechExchange(p, amb, int(want))
	}
	give := d.exchangeTech[i]
	if give == 0 {
		d.oldResponse = -1
		d.response = 0xae
		g.npcMessageRejectionCheck(amb, p, &d.response)
		return nil
	}
	if err := g.getHumanTechExchangeMessage(amb, p, int(want), int(give)); err != nil {
		return err
	}
	if title, err = g.textMessage(jimText2, 5, 0x28); err != nil {
		return err
	}
	c, err := g.diplomacyGenericList(agree[:2], false, title)
	if err != nil {
		return err
	}
	if c == -1 {
		d.message = ""
		return nil
	}
	if d.message, err = g.textMessage(jimText2, 0x31, 0xfa); err != nil {
		return err
	}
	g.changeRelations(12, p, amb, 0, 0, 0)
	g.playerGetsTechApp(p, int(want))
	g.playerGetsTechApp(amb, int(give))
	if s.Players[amb].Objective != objectiveHuman { // not for a human ambassador
		g.getExchangeTechList(p, amb)
	}
	g.adjustDiplomatModifiers(int16(p), int16(amb))
	s.Players[p].Modifiers[2][amb] -= 20
	return nil
}

// getHumanTechExchangeMessage is Get_Human_Tech_Exchange_Message_ (0x18a97):
// "<amb> offers <give> for <want>" straight into _diplomacy_message.
func (g *Game) getHumanTechExchangeMessage(amb, p, want, give int) error {
	t, err := g.textMessage(jimText2, 0x2f, 0xfa)
	if err != nil {
		return err
	}
	g.dip.message, err = g.decodeTextMessage(t, amb, -1, give, -1, p, want, -1, 0xfa)
	return err
}

// diplomacyDemand is Diplomacy_Demand_ (0x18b79): what p demands of amb.
func (g *Game) diplomacyDemand(p, amb int) error {
	e, s, d := g.e, &g.state, &g.dip
	P, A := &s.Players[p], &s.Players[amb]
	active := make([]byte, 11)
	atPeace := g.getDiplomacyDemandTreatyList(p, amb, 5)
	if len(atPeace) > 0 {
		active[0] = 1
	}
	var atWar []uint8
	if g.gameType == 0 {
		if atWar = g.getDiplomacyDemandTreatyList(p, amb, 3); len(atWar) > 0 {
			active[1] = 1
		}
	}
	allied := g.getDiplomacyDemandTreatyList(p, amb, 2)
	if len(allied) > 0 {
		active[2] = 1
	}
	active[3], active[4] = 1, 1
	switch {
	case A.Tribute[p] > 0, P.Tribute[amb] > 0, A.Tribute[p] == 2:
		active[3], active[4] = 0, 0
	case A.Tribute[p] == 1:
		active[3] = 0
	}
	active[5] = 1
	techs, _ := g.getDifferentialTechList(p, amb, 0, 0xc343cb00, true)
	if len(techs) > 0 {
		active[6] = 1
	}
	if s.Stars[0].Name[14]&4 != 0 {
		active[6] = 0 // the 1.31 patch (0x1692c7)
	}
	if g.checkSurrenderSystem(amb, p) {
		active[7] = 1
	}
	if g.playerHasShipsPesteringOtherPlayer(amb, p) {
		active[8] = 1
	}
	active[9] = 1
	e.AssignAutoFunction(g.drawDiplomacySynchMode, 2)
	e.SetInputDelay(1)
	g.diplomacyListStyle(false)
	g.setupDiplomacyHelp(0xb, g.helpIDs(demandDiplomacyHelp, 0xb))
	c, err := g.diplomacyList(d.texts.demandTitle, d.texts.demand[:], active)
	if err != nil {
		return err
	}
	var item int16
	switch c {
	case 0, 1, 2:
		list := [][]uint8{atPeace, atWar, allied}[c]
		names := make([]string, len(list))
		for i, x := range list {
			names[i] = fmt.Sprintf(cStringAt(g.exe, 0x178a8a), save.CString(s.Players[x].EmperorName[:]))
		}
		title, err := g.textMessage(jimText, 0x47+c, 0x28)
		if err != nil {
			return err
		}
		i, err := g.diplomacyGenericList(names, true, title)
		if err != nil || i == -1 {
			return err
		}
		item = int16(list[i])
	case 3, 4, 5, 8:
	case 6:
		i, err := g.diplomacyGenericExchangeTechList(p, d.texts.demandTitle, techs, false)
		if err != nil || i == -1 {
			return err
		}
		item = int16(techs[i])
	case 7:
		st, err := g.diplomacySystemDisplay(amb, p, true)
		if err != nil || st == -1 {
			return err
		}
		item = int16(st)
	default:
		return nil
	}
	d.oldResponse = -1
	accepted := int16(0) // stale stack in the original; see getDemandResponse
	if A.Objective == objectiveHuman {
		if err := g.getHumanDemandResponse(p, amb, uint16(c), item, &accepted); err != nil {
			return err
		}
	} else {
		g.getDemandResponse(p, amb, uint16(c), item, &accepted, &d.response)
	}
	g.adjustDiplomatModifiers(int16(p), int16(amb))
	if accepted != 1 || c == 5 {
		return nil
	}
	g.diplomacyGiveInToDemand(p, amb, uint16(c), item)
	return nil
}

// getDiplomacyDemandTreatyList is Get_Diplomacy_Demand_Treaty_List_
// (0x1a44d): the players both p and amb know whom amb is allied with (kind
// 2), at war with (3) or at peace with (5).
func (g *Game) getDiplomacyDemandTreatyList(p, amb int, kind uint16) []uint8 {
	s := &g.state
	var list []uint8
	for i := 0; i < int(s.NumPlayers); i++ {
		if i == amb || i == p {
			continue
		}
		I := &s.Players[i]
		if I.Contact[amb] != 1 || I.Contact[p] != 1 {
			continue
		}
		st := int8(I.TreatyStatus[amb])
		if kind == 2 && st == 2 || kind == 3 && (st == 4 || st == 5) || kind == 5 && st < 4 {
			list = append(list, uint8(i))
		}
	}
	return list
}

// diplomacyGiveInToDemand is Diplomacy_Give_In_To_Demand_ (0x1b487).
func (g *Game) diplomacyGiveInToDemand(p, amb int, kind uint16, item int16) {
	s := &g.state
	A := &s.Players[amb]
	switch kind {
	case 0, 1, 2:
		A.DelayedOrder[item] = int8(kind)
	case 3, 4:
		g.startTributeTreaty(int16(amb), int16(p), int16(kind-2))
	case 6:
		g.playerGetsTechApp(p, int(item))
	case 7:
		if st := int(item); s.maySurrenderStar(st, amb, p) {
			s.surrenderStar(st, amb, p)
		}
	case 8:
		r := uint8(s.Random.Random(50)) + 10
		A.Diplo80E[p] = r + uint8(s.Random.Random(50))
	}
}
