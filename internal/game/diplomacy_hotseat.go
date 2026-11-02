package game

import "github.com/openorion2/openorion2/internal/engine"

// Diplomacy between two humans of a hotseat game: the other human takes the
// keyboard, sees the proposer's ambassador and answers. Spec:
// docs/screens/hotseat.md, "Diplomacy between humans".

// diplomacyLoadNewAmbassador is Diplomacy_Load_New_Ambassador_ (0x1b881):
// amb's ambassador takes the screen.
func (g *Game) diplomacyLoadNewAmbassador(amb int) error {
	e, s := g.e, &g.state
	e.SetMouseList(engine.MouseNone)
	e.DeactivateAutoFunction()
	e.ClearFields()
	e.SetFadePercent(0)
	e.SetFadeSpeedPercent(10)
	if err := e.FastFadeOut(); err != nil {
		return err
	}
	// The music goes back to -1: the lists then ask for track 99, which the
	// original plays as an empty sample (docs/net/diplomacy.md, "Music").
	if err := g.diplomacyMouseInit(amb); err != nil {
		return err
	}
	race := int(s.Players[amb].Race)
	p, err := g.picture(diplomatLBX, race*2+0xd)
	if err != nil {
		return err
	}
	e.OpenFileAnimation(p)
	e.V.SetPageOff()
	if err := e.DrawFileAnimation(p); err != nil {
		return err
	}
	e.V.CopyOffToBack()
	if p, err = g.picture(diplomatLBX, race*2+0xe); err != nil {
		return err
	}
	g.dip.ambassador = amb
	e.OpenFileAnimation(p)
	e.SetMouseList(engine.MouseArrow)
	return e.DrawFileAnimationStencil()
}

// checkHumanTreatyProposal is Check_Human_Treaty_Proposal_ (0x1b06b) in a
// hotseat game: amb answers p's proposal in front of p's ambassador. 3
// accepted, 0 refused, 1 asks for a better offer.
func (g *Game) checkHumanTreatyProposal(p, amb int, kind uint16) (int, error) {
	d := &g.dip
	switch kind {
	case 7:
		d.response = 0x63
	case 8:
		d.response = 0x64
	case 3:
		d.response = 0x53
	case 1:
		d.response = 0x61
	case 2:
		d.response = 0x62
	}
	if g.gameType != 1 {
		b, err := g.netAsk(p, amb, netDipTreaty, int16(kind), d.response)
		return int(b), err
	}
	if err := g.diplomacyLoadNewAmbassador(p); err != nil {
		return 0, err
	}
	c, err := g.getHumanTreatyChoice(p, amb, kind)
	if err != nil {
		return 0, err
	}
	return c, g.diplomacyLoadNewAmbassador(amb)
}

// getHumanTreatyChoice is Get_Human_Treaty_Choice_ (0x1b1a7): yes, no, and
// "better offer" when p has something to give.
func (g *Game) getHumanTreatyChoice(p, amb int, kind uint16) (int, error) {
	d := &g.dip
	rows := make([]string, 2, 3)
	var err error
	if rows[0], err = g.textMessage(jimText, 0x4f, 0x28); err != nil {
		return 0, err
	}
	if rows[1], err = g.textMessage(jimText, 0x50, 0x28); err != nil {
		return 0, err
	}
	if g.checkBetterOfferAvailable(p, amb) {
		r, err := g.textMessage(jimText, 0x52, 0x28)
		if err != nil {
			return 0, err
		}
		rows = append(rows, r)
	}
	d.response, d.oldResponse = 0, 0
	if err := g.getHumanProposeTreatyMessage(kind); err != nil {
		return 0, err
	}
	g.e.DeactivateHelpList()
	title, err := g.textMessage(jimText, 0x4a, 0x28)
	if err != nil {
		return 0, err
	}
	c, err := g.diplomacyGenericList(rows, false, title)
	if err != nil {
		return 0, err
	}
	if c == -1 {
		c = len(rows) - 1 // the last row cannot be chosen
	}
	switch c {
	case 0:
		switch kind {
		case 7:
			d.response = 0x85
		case 8:
			d.response = 0x86
		case 3:
			d.response = 0x87
		case 1:
			d.response = 0x83
		case 2:
			d.response = 0x84
		}
		return 3, nil
	case 1:
		d.response = 0x8a
		return 0, nil
	case 2:
		return 1, nil
	}
	return 0, nil
}

// checkBetterOfferAvailable is Check_Better_Offer_Available_ (0x1b35a): p
// could add a system, a technology or more than 100 BC.
func (g *Game) checkBetterOfferAvailable(p, amb int) bool {
	if g.checkSurrenderSystem(p, amb) {
		return true
	}
	if techs, _ := g.getDifferentialTechList(amb, p, 0, 0xc343cb00, true); len(techs) > 0 {
		return true
	}
	return g.state.Players[p].BC > 100
}

// getHumanProposeTreatyMessage is Get_Human_Propose_Treaty_Message_
// (0x189ec): the proposal as the other human reads it.
func (g *Game) getHumanProposeTreatyMessage(kind uint16) error {
	n := 0
	switch kind {
	case 7:
		n = 0x2a
	case 8:
		n = 0x2b
	case 9, 10:
		n = 0x2c
	case 1:
		n = 0x2d
	case 2:
		n = 0x2e
	case 3:
		n = 0x42
	default:
		return nil
	}
	var err error
	g.dip.message, err = g.textMessage(jimText2, n, 0xfa)
	return err
}

// getHumanFormTreatyMessage is Get_Human_Form_Treaty_Message_ (0x188e0):
// the human ambassador's answer, jimtext2 0x20.. (none for peace).
func (g *Game) getHumanFormTreatyMessage(kind uint16, accepted bool) error {
	var n int
	switch kind {
	case 7:
		n = 0x20
	case 8:
		n = 0x22
	case 9, 10:
		n = 0x24
	case 1:
		n = 0x26
	case 2:
		n = 0x28
	default:
		return nil
	}
	if !accepted {
		n++
	}
	var err error
	g.dip.message, err = g.textMessage(jimText2, n, 0xfa)
	return err
}

// getHumanDemandResponse is Get_Human_Demand_Response_ (0x1b5b8) in a
// hotseat game: amb gives in to p's demand or not, before p's ambassador.
func (g *Game) getHumanDemandResponse(p, amb int, kind uint16, item int16, accepted *int16) error {
	d := &g.dip
	if g.gameType != 1 {
		b, err := g.netAsk(p, amb, netDipDemand, int16(kind), item)
		*accepted = b
		return err
	}
	d.oldResponse = d.response
	if err := g.getHumanDemandMessage(p, amb, kind, item); err != nil {
		return err
	}
	if err := g.diplomacyLoadNewAmbassador(p); err != nil {
		return err
	}
	c, err := g.getHumanDemandChoice(amb, p, kind, item)
	if err != nil {
		return err
	}
	d.oldResponse = -1
	*accepted = int16(c)
	d.response = 0xa8
	if c == 1 {
		d.response = 0xa9
	}
	return g.diplomacyLoadNewAmbassador(amb)
}

// getHumanDemandMessage is Get_Human_Demand_Message_ (0x186d3).
func (g *Game) getHumanDemandMessage(p, amb int, kind uint16, item int16) error {
	d := &g.dip
	n := []int{0x13, 0x14, 0x15, 0x16, 0x16, 0x17, 0x18, 0x19, 0x1a}
	if kind > 8 {
		return nil
	}
	t, err := g.textMessage(jimText2, n[kind], 0xfa)
	if err != nil {
		return err
	}
	switch kind {
	case 0, 1, 2:
		d.message, err = g.decodeTextMessage(t, p, -1, -1, -1, int(item), -1, -1, 0xfa)
	case 3:
		d.message, err = g.decodeTextMessage(t, p, -1, -1, 5, -1, -1, -1, 0xfa)
	case 4:
		d.message, err = g.decodeTextMessage(t, p, -1, -1, 10, -1, -1, -1, 0xfa)
	case 6:
		d.message, err = g.decodeTextMessage(t, p, -1, int(item), -1, -1, -1, -1, 0xfa)
	case 7:
		d.message, err = g.decodeTextMessage(t, p, int(item), -1, -1, -1, -1, -1, 0xfa)
	default: // 5, 8: Decode_Text_Race_And_Name_
		d.message, err = g.decodeTextMessage(t, p, -1, -1, -1, -1, -1, -1, 0xfa)
	}
	return err
}

// getHumanDemandChoice is Get_Human_Demand_Choice_ (0x1b3b5): 1 when amb
// gives in.
func (g *Game) getHumanDemandChoice(amb, p int, kind uint16, item int16) (int, error) {
	if kind == 7 {
		c, err := g.diplomacyGenericSystemList(int(item), amb, p, true)
		if err != nil || c == -1 {
			return 0, err
		}
		return 1, nil
	}
	rows := make([]string, 2)
	var err error
	if rows[0], err = g.textMessage(jimText, 0x4f, 0x28); err != nil {
		return 0, err
	}
	if rows[1], err = g.textMessage(jimText, 0x50, 0x28); err != nil {
		return 0, err
	}
	g.e.DeactivateHelpList()
	title, err := g.textMessage(jimText2, 0, 0x28)
	if err != nil {
		return 0, err
	}
	c, err := g.diplomacyGenericList(rows, false, title)
	if err != nil {
		return 0, err
	}
	if c == 0 {
		return 1, nil
	}
	return 0, nil
}

// getHumanGiftResponse is Get_Human_Gift_Response_ (0x1b724) in a hotseat
// game: amb is told of p's gift (kind 1 money, 2 tribute, 4 a system) before
// p's ambassador. A technology gift has no human path: the AI answers it.
func (g *Game) getHumanGiftResponse(p, amb int, kind uint16, item int16) error {
	d := &g.dip
	if g.gameType != 1 {
		n := &d.net
		n.setB(item)
		n.set(uint8(p), uint8(amb))
		n.setA(int16(kind))
		n.setCode(netDipGift)
		g.netDipPost(amb)
	} else {
		d.oldResponse = d.response
		if err := g.getHumanOfferGiftMessage(p, kind, item); err != nil {
			return err
		}
		if err := g.diplomacyLoadNewAmbassador(p); err != nil {
			return err
		}
		if err := g.diplomacyDisplayResponse(); err != nil {
			return err
		}
		if err := g.diplomacyLoadNewAmbassador(amb); err != nil {
			return err
		}
	}
	switch kind {
	case 1:
		d.response = 0xa3
	case 2:
		d.response = 0xa2
	case 3:
		d.response = 0xa1
	case 4:
		d.response = 0xa4
	}
	return nil
}

// getHumanOfferGiftMessage is Get_Human_Offer_Gift_Message_ (0x185bd).
func (g *Game) getHumanOfferGiftMessage(p int, kind uint16, item int16) error {
	d := &g.dip
	n := map[uint16]int{1: 0x12, 2: 0xf, 3: 0x10, 4: 0x11}[kind]
	if n == 0 {
		return nil
	}
	t, err := g.textMessage(jimText2, n, 0xfa)
	if err != nil {
		return err
	}
	switch kind {
	case 1:
		d.message, err = g.decodeTextMessage(t, p, -1, -1, int32(item), -1, -1, -1, 0xfa)
	case 2:
		stat := int32(10)
		if item == 1 {
			stat = 5
		}
		d.message, err = g.decodeTextMessage(t, p, -1, -1, stat, -1, -1, -1, 0xfa)
	case 3:
		d.message, err = g.decodeTextMessage(t, p, -1, int(item), -1, -1, -1, -1, 0xfa)
	case 4:
		d.message, err = g.decodeTextMessage(t, p, int(item), -1, -1, -1, -1, -1, 0xfa)
	}
	return err
}

// diplomacyGenericSystemList is Diplomacy_Generic_System_List_ (0x1a1fd):
// star st of giver shown on the map with yes/no; demand words it as
// receiver's demand. 0 for yes, -1 otherwise.
func (g *Game) diplomacyGenericSystemList(st, giver, receiver int, demand bool) (int, error) {
	e, s, d := g.e, &g.state, &g.dip
	sy := &d.sys
	g.currentScreen = 6 // not restored
	gm := &g.ms.galaxyMap
	gm.colonyPalette = true
	s.StarBgIndex = 0
	g.ms.galaxyMapScannedStar = int16(st)
	sy.giver, sy.receiver = giver, receiver
	sy.scanFlag = int16(st)
	g.diplomacySystemScan(st, giver)
	sy.population = g.getSystemPopulation(giver, st)
	t, err := g.textMessage(jimText2, 0x38, 0x1e)
	if err != nil {
		return -1, err
	}
	if d.texts.millionColon, err = g.decodeTextMessage(t, -1, -1, -1, int32(sy.population), -1, -1, -1, 0x1e); err != nil {
		return -1, err
	}
	if t, err = g.textMessage(jimText2, 0x3a, 0x23); err != nil {
		return -1, err
	}
	if d.texts.xStarSystem, err = g.decodeTextMessage(t, -1, st, -1, -1, -1, -1, -1, 0x23); err != nil {
		return -1, err
	}
	active := make([]byte, 10)
	for i := range active {
		active[i] = 1
	}
	title, err := g.textMessage(jimText, 0x4b, 0x28)
	if err != nil {
		return -1, err
	}
	if demand {
		err = g.getHumanDemandMessage(receiver, giver, 7, int16(st))
	} else {
		d.message, err = g.textMessage(jimText, 0x4e, 0xfa)
	}
	if err != nil {
		return -1, err
	}
	rows := make([]string, 3)
	if rows[0], err = g.textMessage(jimText, 0x4f, 0x32); err != nil {
		return -1, err
	}
	if rows[1], err = g.textMessage(jimText, 0x50, 0x32); err != nil {
		return -1, err
	}
	e.ClearFields()
	e.AssignAutoFunction(func() error {
		if err := g.drawDiplomacySynchMode(); err != nil {
			return err
		}
		if err := g.drawDiplomacySystem(); err != nil {
			return err
		}
		return g.drawDiplomacySystemText(int(sy.scanFlag), sy.homeworld)
	}, 2)
	e.SetInputDelay(1)
	g.diplomacyListStyle(false)
	var v int16
	c, err := e.GetListField(diplomacyListX, 0x66, diplomacyListW, title, rows, &v, active, diplomacyListFill, [4]int{})
	gm.colonyPalette = false
	if err != nil {
		return -1, err
	}
	if c := int16(c); c == -1 || c == 1 {
		return -1, nil
	}
	return int(int16(c)), nil
}

// diplomacyHumanNeedBetterOffer is Diplomacy_Human_Need_Better_Offer_
// (0x1bd5b): amb asked p for more; p adds a system, a technology or money
// and amb takes it or not. True when something was given.
func (g *Game) diplomacyHumanNeedBetterOffer(p, amb int) (bool, error) {
	s, d := &g.state, &g.dip
	P, A := &s.Players[p], &s.Players[amb]
	d.net.chatMode = false
	sysRow, techRow, moneyRow := -10, -10, -10
	t, err := g.textMessage(jimText2, 0x33, 0xfa)
	if err != nil {
		return false, err
	}
	if d.message, err = g.decodeTextMessage(t, amb, -1, -1, -1, -1, -1, -1, 0xfa); err != nil {
		return false, err
	}
	var rows []string
	add := func(n int, at *int) error {
		r, err := g.textMessage(jimText2, n, 0x28)
		*at = len(rows)
		rows = append(rows, r)
		return err
	}
	if g.checkSurrenderSystem(p, amb) {
		if err := add(0x34, &sysRow); err != nil {
			return false, err
		}
	}
	techs, _ := g.getDifferentialTechList(amb, p, 0, 0xc343cb00, true) // Get_Diplomacy_Tech_List_
	techs = techs[:min(len(techs), 10)]
	if len(techs) > 0 {
		if err := add(0x35, &techRow); err != nil {
			return false, err
		}
	}
	if P.BC > 100 {
		if err := add(0x40, &moneyRow); err != nil {
			return false, err
		}
	}
	if len(rows) == 0 {
		return false, nil
	}
	g.e.DeactivateHelpList()
	title, err := g.textMessage(jimText2, 0x36, 0x28)
	if err != nil {
		return false, err
	}
	c, err := g.diplomacyGenericList(rows, true, title)
	if err != nil {
		return false, err
	}
	if g.gameType != 1 {
		return g.netHumanNeedBetterOffer(p, amb, c, sysRow, techRow, moneyRow, techs)
	}
	// No fall-through here, unlike the network part: the choice becomes the
	// other human's yes (0) or no (-1), and the later rows are 1 or more.
	given := false
	ask := func(row string) (bool, error) {
		title, err := g.textMessage(jimText2, 9, 0x28)
		if err != nil {
			return false, err
		}
		c, err := g.diplomacyGenericList([]string{row}, true, title)
		return c != -1, err
	}
	switch c {
	case sysRow:
		st, err := g.diplomacySystemDisplay(p, d.ambassador, false)
		if err != nil || st == -1 {
			return false, err
		}
		if err := g.diplomacyLoadNewAmbassador(amb); err != nil {
			return false, err
		}
		r, err := g.diplomacyGenericSystemList(st, p, amb, false)
		if err != nil {
			return false, err
		}
		if r != -1 {
			if s.maySurrenderStar(st, p, amb) {
				s.surrenderStar(st, p, amb)
			}
			given = true
		}
	case techRow:
		names := make([]string, min(len(techs), 8))
		for i := range names {
			n, err := g.technologyApplicationsName(int(techs[i]), int(s.PlayerNum))
			if err != nil {
				return false, err
			}
			names[i] = cSprintf(cStringAt(g.exe, 0x178a8a), n)
		}
		i, err := g.diplomacyGenericList(names, true, d.texts.offerTitle)
		if err != nil || i == -1 {
			return false, err
		}
		tech := int(techs[i])
		name, err := g.technologyApplicationsName(tech, p)
		if err != nil {
			return false, err
		}
		if err := g.diplomacyLoadNewAmbassador(p); err != nil {
			return false, err
		}
		ok, err := ask(cSprintf(cStringAt(g.exe, 0x178a8a), name))
		if err != nil {
			return false, err
		}
		if ok {
			g.playerGetsTechApp(amb, tech)
			given = true
		}
		if err := g.diplomacyLoadNewAmbassador(amb); err != nil {
			return false, err
		}
	case moneyRow:
		sum, err := g.diplomacyOfferMoney(p, amb, false)
		if err != nil || sum <= 0 {
			return false, err
		}
		if err := g.diplomacyLoadNewAmbassador(p); err != nil {
			return false, err
		}
		ok, err := ask(cSprintf(cStringAt(g.exe, 0x178aa8), int(sum)))
		if err != nil {
			return false, err
		}
		if ok {
			P.BC -= int32(sum)
			A.BC += int32(sum)
			given = true
		}
		if err := g.diplomacyLoadNewAmbassador(amb); err != nil {
			return false, err
		}
	}
	return given, nil
}

// humanTechExchange is Diplomacy_Exchange_Technology_'s hotseat path
// (0x1cc1a..0x1cd10, 0x1cf05..): amb picks what to give for want, p agrees
// or not. Relations and the techs follow; the AI's lists and modifiers do not.
func (g *Game) humanTechExchange(p, amb, want int) error {
	s, d := &g.state, &g.dip
	if err := g.diplomacyLoadNewAmbassador(p); err != nil {
		return err
	}
	t, err := g.textMessage(jimText2, 0x30, 0xfa) // Get_Human_Tech_Exchange_First_Message_
	if err != nil {
		return err
	}
	if d.message, err = g.decodeTextMessage(t, p, -1, -1, -1, amb, want, -1, 0xfa); err != nil {
		return err
	}
	title, err := g.textMessage(jimText2, 4, 0x28)
	if err != nil {
		return err
	}
	d.targetTech, _ = g.getDifferentialTechList(amb, p, 0, 0xc343cb00, true)
	c, err := g.diplomacyGenericExchangeTechList(amb, title, d.targetTech, false)
	if err != nil {
		return err
	}
	if err := g.diplomacyLoadNewAmbassador(amb); err != nil {
		return err
	}
	if c == -1 {
		return nil
	}
	give := int(d.targetTech[c])
	if err := g.getHumanTechExchangeMessage(amb, p, want, give); err != nil {
		return err
	}
	if title, err = g.textMessage(jimText2, 5, 0x28); err != nil {
		return err
	}
	agree := []string{d.texts.agree[0], d.texts.agree[1]}
	r, err := g.diplomacyGenericList(agree, false, title)
	if err != nil {
		return err
	}
	if r == -1 {
		d.message, err = g.textMessage(jimText2, 0x32, 0xfa) // Get_Human_Tech_Exchange_Decline_Message_
		return err
	}
	// Get_Human_Tech_Exchange_Accept_Message_ ("No", the last row, gives -1).
	if d.message, err = g.textMessage(jimText2, 0x31, 0xfa); err != nil {
		return err
	}
	g.changeRelations(12, p, amb, 0, 0, 0)
	g.playerGetsTechApp(p, want)
	g.playerGetsTechApp(amb, give)
	_ = s
	return nil
}

// humanResponseMessage: the hotseat Declare War, Surrender, Break Treaty in p's ambassador; inlines
// Get_Human_Declare_War_Message_, Get_Human_Surrender_Message_, Get_Human_Break_Treaty_Message_.
func (g *Game) humanResponseMessage(p, amb, name, n int, reset bool) error {
	d := &g.dip
	if g.gameType != 1 {
		g.Log("diplomacy with a human ambassador in a network game is not ported")
		return nil
	}
	if err := g.diplomacyLoadNewAmbassador(p); err != nil {
		return err
	}
	t, err := g.textMessage(jimText2, n, 0xfa)
	if err != nil {
		return err
	}
	if d.message, err = g.decodeTextMessage(t, name, -1, -1, -1, -1, -1, -1, 0xfa); err != nil {
		return err
	}
	if reset {
		d.response, d.oldResponse = 0, 0
	}
	if err := g.diplomacyDisplayResponse(); err != nil {
		return err
	}
	return g.diplomacyLoadNewAmbassador(amb)
}
