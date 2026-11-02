package game

import (
	"encoding/binary"

	"github.com/openorion2/openorion2/internal/font"
)

// The talk between two humans (DIP-SCRN.C, network part): one talks, the
// other listens, and msg 0x7d0 carries each step. Ported from the assembly;
// spec docs/net/diplomacy.md.

// Dialogue codes in _net_diplomacy_message+4 (the AI's response numbers
// travel there too).
const (
	netDipExit      = 0x82  // the talker said good bye
	netDipTreaty    = 0x3e8 // a treaty proposed: a kind, b the statement
	netDipBreak     = 0x3e9 // a treaty broken: a kind
	netDipWar       = 0x3ea // war declared
	netDipSurrender = 0x3eb // the empire surrendered
	netDipGift      = 0x3ec // a gift: a kind, b the item
	netDipDemand    = 0x3ed // a demand: a kind, b the item
	netDipEsc       = 0x3ee // the listener pressed Esc / Good Bye
	netDipSystem    = 0x3ef // a better offer: star a
	netDipTech      = 0x3f0 // a better offer: technology a
	netDipAskTech   = 0x3f1 // an exchange: a the technology wanted
	netDipGiveTech  = 0x3f2 // the exchange answered: a the choice, b the technology
	netDipSwitch    = 0x3f3 // the turn to talk passes
	netDipSynch     = 0x3f4 // the first synchronisation
	netDipMoney     = 0x3f5 // a better offer: a BC
	netDipChatMax   = 0x64  // msg 0x834's buffer
)

// diplomacyNet holds DIP-SCRN.C's network globals.
type diplomacyNet struct {
	talker, listener uint8 // _talker_ambassador, _listen_ambassador
	chatMode         bool  // _chat_mode_flag
	escPressed       int16 // _net_diplomacy_esc_pressed
	synchUp          int16 // _synch_up_established_flag
	techOffered      int16 // _net_diplomacy_tech_offered
	treatyType       int16 // _net_diplomacy_treaty_type
	chatField        int   // _diplomacy_chat_field
	quitChatField    int   // _diplomacy_quit_chat_field
	chatX, chatY     int16 // _chat_base_x, _chat_base_y
	chatW, chatH     int16 // _chat_width, _chat_height

	pkt      [8]byte             // _net_diplomacy_message: talker, listener, a, code, b
	netChat  [netDipChatMax]byte // _net_diplomacy_chat_message: msg 0x834 as sent
	chatMsg  [netDipChatMax]byte // _chat_message: the input field's string
	recvChat [0xfa]byte          // _receive_chat_message

	inTalk bool // port-only: Get_Net_Diplomacy_Choices_ runs (a lost partner ends it)
}

func (n *diplomacyNet) a() int16       { return int16(binary.LittleEndian.Uint16(n.pkt[2:])) }
func (n *diplomacyNet) code() uint16   { return binary.LittleEndian.Uint16(n.pkt[4:]) }
func (n *diplomacyNet) b() int16       { return int16(binary.LittleEndian.Uint16(n.pkt[6:])) }
func (n *diplomacyNet) setA(v int16)   { binary.LittleEndian.PutUint16(n.pkt[2:], uint16(v)) }
func (n *diplomacyNet) setCode(v int)  { binary.LittleEndian.PutUint16(n.pkt[4:], uint16(v)) }
func (n *diplomacyNet) setB(v int16)   { binary.LittleEndian.PutUint16(n.pkt[6:], uint16(v)) }
func (n *diplomacyNet) set(t, l uint8) { n.pkt[0], n.pkt[1] = t, l }

// netDipPost sends _net_diplomacy_message to player to (msg 0x7d0), then
// Mox_Update_; the fields not set keep the last message's values.
func (g *Game) netDipPost(to int) {
	buf := g.dip.net.pkt
	g.moxSendMessage(int32(g.state.Players[to].NetID), 0x7d0, buf[:])
	g.e.MoxUpdate()
}

// diplomacyReceiveMessage is Diplomacy_Receive_Message_ (0x1f230), ids
// 2000..2999: the dialogue (0x7d0) and its chat (0x834).
func (g *Game) diplomacyReceiveMessage(from, id uint16, buf []byte) {
	d := &g.dip
	n := &d.net
	if id < 0x7d0 || id >= 0xbb8 {
		return
	}
	if id != 0x7d0 {
		if id == 0x834 {
			copyCStr(n.recvChat[:], buf)
		}
		return
	}
	n.netChat[0], n.recvChat[0] = 0, 0
	copy(n.pkt[:], buf)
	switch n.code() {
	case netDipEsc:
		n.escPressed = 1
		return
	case netDipSynch:
		n.synchUp = 1
		return
	case 0x3f6:
		return
	case netDipSwitch:
		n.setCode(0)
		d.response, d.oldResponse = 0, 0
		d.message = ""
	}
	d.response = int16(n.code())
	n.talker, n.listener = n.pkt[0], n.pkt[1]
}

// getListenModeMessage is Get_Listen_Mode_Message_ (0x1f34b): "<p> speaks".
func (g *Game) getListenModeMessage(p int) error {
	d := &g.dip
	d.response, d.oldResponse = 0, 0
	t, err := g.textMessage(jimText2, 0xc, 0xfa)
	if err != nil {
		return err
	}
	d.message, err = g.decodeTextMessage(t, p, -1, -1, -1, -1, -1, -1, 0xfa)
	return err
}

// diplomacyNetSwitcheroo is Diplomacy_Net_Switcheroo_ (0x1ed17): the
// listener becomes the talker (code 0x3f3 to him).
func (g *Game) diplomacyNetSwitcheroo() {
	d := &g.dip
	n := &d.net
	t, l := n.talker, n.listener
	n.talker, n.listener = l, t
	n.set(l, t)
	d.oldResponse = -1
	n.setCode(netDipSwitch)
	n.setB(-1)
	g.netDipPost(int(l))
	d.response, d.oldResponse = 0, 0
}

// diplomacySynchMode is Diplomacy_Synch_Mode_ (0x1f12f): one frame while
// waiting for the other side's code 0x3f4.
func (g *Game) diplomacySynchMode() error {
	e, d := g.e, &g.dip
	d.oldResponse = 0
	e.AssignAutoFunction(g.drawDiplomacyListenMode, 2)
	d.response = 0
	e.MarkTime()
	e.ClearFields()
	if err := g.drawDiplomacySynchMode(); err != nil {
		return err
	}
	if err := e.TogglePages(); err != nil {
		return err
	}
	if err := e.ReleaseTime(2); err != nil {
		return err
	}
	e.AssignAutoFunction(g.drawDiplomacySynchMode, 2)
	return nil
}

// drawDiplomacyListenMode is Draw_Diplomacy_Listen_Mode_ (0x1f0bc): the
// listener's options (chat, good bye) or the chat prompt.
func (g *Game) drawDiplomacyListenMode() error {
	d := &g.dip
	if err := g.drawDiplomacySynchMode(); err != nil {
		return err
	}
	t := g.e.Text()
	t.SetStyleShadow(4, g.exeColors(diplomacyNormalColors), font.ShadowDown)
	if d.net.chatMode {
		return t.Print(0xa, 0x75, d.texts.enterMessage)
	}
	if err := t.Print(0xa, 0x75, d.texts.listenOption); err != nil {
		return err
	}
	if err := t.Print(0xa, 0x8d, d.texts.netList[7]); err != nil {
		return err
	}
	return t.Print(0x12, 0xa6, d.texts.listenGoodBye)
}

// diplomacyListenMode is Diplomacy_Listen_Mode_ (0x1ede9): one frame of the
// side that listens to other; C chats, G or Esc says good bye (code 0x3ee).
func (g *Game) diplomacyListenMode(other, me int) error {
	e, d := g.e, &g.dip
	n := &d.net
	e.AssignAutoFunction(g.drawDiplomacyListenMode, 2)
	e.MarkTime()
	chat := n.chatMode
	if !n.chatMode {
		n.chatField = -1000
		e.ClearFields()
		g.setupFullScreenDiplomacyHelp(0x2b6)
		fc, err := e.AddHiddenField(0xf, 0x8b, 0x9d, 0xa2, "C", 0)
		if err != nil {
			return err
		}
		fg, err := e.AddHiddenField(0xf, 0xa5, 0x9d, 0xbc, "G", 0)
		if err != nil {
			return err
		}
		esc, err := e.AddHotKey("\x1b")
		if err != nil {
			return err
		}
		r, err := e.GetInput()
		if err != nil {
			return err
		}
		if r == fg || r == esc {
			n.set(uint8(other), uint8(me))
			n.setCode(netDipEsc)
			n.setB(-1)
			g.netDipPost(other)
			n.escPressed = 1
		}
		chat = r == fc
	}
	if chat {
		if err := g.diplomacyChat(other, me); err != nil {
			return err
		}
	}
	if err := g.drawDiplomacyListenMode(); err != nil {
		return err
	}
	if n.chatMode {
		e.TintFill(int(n.chatX), int(n.chatY), int(n.chatX+n.chatW), int(n.chatY+n.chatH-1), 0)
	}
	if err := e.TogglePages(); err != nil {
		return err
	}
	if err := e.ReleaseTime(2); err != nil {
		return err
	}
	e.AssignAutoFunction(g.drawDiplomacySynchMode, 2)
	return nil
}

// diplomacyChat is Diplomacy_Chat_ (0x1ef5b): the first call opens the chat
// field, later ones read it; Enter or a click sends the line (msg 0x834).
func (g *Game) diplomacyChat(to, me int) error {
	e, d := g.e, &g.dip
	n := &d.net
	if !n.chatMode {
		g.setupFullScreenDiplomacyHelp(0x2b6)
		e.ClearFields()
		n.chatMode = true
		e.Text().SetStyleShadow(4, g.exeColors(diplomacyNormalColors), font.ShadowDown)
		n.chatMsg[0] = 0
		var err error
		if n.quitChatField, err = e.AddHiddenField(0, 0, 0x27f, 0x1df, "", 0); err != nil {
			return err
		}
		if n.chatField, err = e.AddContinuousStringInputField(int(n.chatX), int(n.chatY), int(n.chatW-0x14), int(n.chatH), n.chatMsg[:], 0x50, 0, nil, 0x29, 0); err != nil {
			return err
		}
		e.ActivateInputField(n.chatField)
		return nil
	}
	if n.escPressed == 1 {
		n.chatMode = false
	}
	r, err := e.GetInput()
	if err != nil {
		return err
	}
	if n.escPressed == 1 {
		n.chatMode = false
	}
	if r == n.chatField || r == n.quitChatField {
		e.ClearFields()
		n.chatMode = false
		copyCStr(n.netChat[:], n.chatMsg[:])
		buf := n.netChat
		g.moxSendMessage(int32(g.state.Players[to].NetID), 0x834, buf[:])
		g.e.MoxUpdate()
		n.netChat[0] = 0
	}
	return nil
}

// diplomacyMainNetChat is Diplomacy_Main_Net_Chat_ (0x200c3): the talker
// chats until the line is sent.
func (g *Game) diplomacyMainNetChat(to, me int) error {
	e, d := g.e, &g.dip
	n := &d.net
	for {
		e.MarkTime()
		if err := g.diplomacyChat(to, me); err != nil {
			return err
		}
		if n.chatMode {
			if err := e.InvokeAutoFunction(); err != nil {
				return err
			}
			t := e.Text()
			t.SetStyleShadow(4, g.exeColors(diplomacyNormalColors), font.ShadowDown)
			if err := t.Print(0xa, 0x75, d.texts.enterMessage); err != nil {
				return err
			}
			e.TintFill(int(n.chatX), int(n.chatY), int(n.chatX+n.chatW), int(n.chatY+n.chatH-1), 0)
			if err := e.TogglePages(); err != nil {
				return err
			}
			if err := e.ReleaseTime(2); err != nil {
				return err
			}
		}
		if !n.chatMode {
			return nil
		}
	}
}

// startNetTreaty is Start_Net_Treaty_ (0x1f17d): the agreed treaty on this
// machine, and its message.
func (g *Game) startNetTreaty() error {
	n := &g.dip.net
	t, l := int16(n.talker), int16(n.listener)
	switch n.treatyType {
	case 1:
		g.startTreaty(t, l, 1)
	case 2:
		g.startTreaty(t, l, 2)
	case 3:
		g.declarePeace(t, l)
	case 7:
		g.startTradeTreaty(t, l)
	case 8:
		g.startResearchTreaty(t, l)
	}
	return g.getHumanFormTreatyMessage(uint16(n.treatyType), true)
}

// netListen is the loop the routines that put a question to the other human
// share: listen until his answer replaces unset in b or he says good bye.
func (g *Game) netListen(other, me int, unset int16) error {
	n := &g.dip.net
	for {
		if err := g.diplomacyListenMode(other, me); err != nil {
			return err
		}
		if n.b() != unset || n.escPressed != 0 {
			return nil
		}
	}
}

// netAsk sends code with a and b from p to amb and waits for his answer in
// b; 0 if he left.
func (g *Game) netAsk(p, amb int, code int, a, b int16) (int16, error) {
	n := &g.dip.net
	n.set(uint8(p), uint8(amb))
	n.setB(b)
	n.setA(a)
	n.setCode(code)
	g.netDipPost(amb)
	if err := g.getListenModeMessage(amb); err != nil {
		return 0, err
	}
	n.setB(-1)
	if err := g.netListen(amb, p, -1); err != nil {
		return 0, err
	}
	if n.escPressed == 1 {
		n.setB(0)
	}
	return n.b(), nil
}

// netTell sends code (a, b = the current response) from p to amb without an
// answer: a war, a surrender or a broken treaty.
func (g *Game) netTell(p, amb int, code int, a int16) {
	d := &g.dip
	n := &d.net
	n.set(uint8(p), uint8(amb))
	n.setB(d.response)
	n.setA(a)
	n.setCode(code)
	g.netDipPost(amb)
}

// netHumanNeedBetterOffer is Diplomacy_Human_Need_Better_Offer_'s network
// part (0x1bed1..): each offer goes to amb, who takes it or not. The choice
// variable is reused, so a star or tech index equal to a later row's
// number opens that row too.
func (g *Game) netHumanNeedBetterOffer(p, amb, c, sysRow, techRow, moneyRow int, techs []uint8) (bool, error) {
	s, d := &g.state, &g.dip
	given := int16(0)
	v := c
	if v == sysRow {
		st, err := g.diplomacySystemDisplay(p, d.ambassador, false)
		if err != nil {
			return false, err
		}
		v = st
		if st != -1 {
			if given, err = g.netAsk(p, amb, netDipSystem, int16(st), d.response); err != nil {
				return false, err
			}
			if given == 1 && s.maySurrenderStar(st, p, amb) {
				s.surrenderStar(st, p, amb)
			}
		}
	}
	if v == techRow {
		names := make([]string, min(len(techs), 8))
		for i := range names {
			n, err := g.technologyApplicationsName(int(techs[i]), int(s.PlayerNum))
			if err != nil {
				return false, err
			}
			names[i] = cSprintf(cStringAt(g.exe, 0x178a8a), n)
		}
		i, err := g.diplomacyGenericList(names, true, d.texts.offerTitle)
		if err != nil {
			return false, err
		}
		v = i
		if i != -1 {
			tech := int(techs[i])
			if given, err = g.netAsk(p, amb, netDipTech, int16(tech), d.response); err != nil {
				return false, err
			}
			if given == 1 {
				g.playerGetsTechApp(amb, tech)
			}
		}
	}
	if v == moneyRow {
		sum, err := g.diplomacyOfferMoney(p, amb, false)
		if err != nil {
			return false, err
		}
		if sum > 0 {
			if given, err = g.netAsk(p, amb, netDipMoney, sum, d.response); err != nil {
				return false, err
			}
			if given == 1 {
				s.Players[p].BC -= int32(sum)
				s.Players[amb].BC += int32(sum)
			}
		}
	}
	return given == 1, nil
}

// netTechExchange is Diplomacy_Exchange_Technology_'s network path
// (0x1cd15..0x1ce76): amb names what he gives for want (code 0x3f1), p
// agrees or not (code 0x3f2).
func (g *Game) netTechExchange(p, amb, want int) error {
	d := &g.dip
	n := &d.net
	n.set(uint8(p), uint8(amb))
	n.setA(int16(want))
	n.setCode(netDipAskTech)
	n.setB(0)
	g.netDipPost(amb)
	if err := g.getListenModeMessage(amb); err != nil {
		return err
	}
	n.setB(-2)
	if err := g.netListen(amb, p, -2); err != nil {
		return err
	}
	if n.escPressed == 1 {
		n.setB(-1)
	}
	if n.b() == -1 {
		return nil
	}
	give := int(n.b())
	if err := g.getHumanTechExchangeMessage(amb, p, want, give); err != nil {
		return err
	}
	title, err := g.textMessage(jimText2, 5, 0x28)
	if err != nil {
		return err
	}
	r, err := g.diplomacyGenericList([]string{d.texts.agree[0], d.texts.agree[1]}, false, title)
	if err != nil {
		return err
	}
	n.set(uint8(p), uint8(amb))
	n.setCode(netDipGiveTech)
	n.setB(int16(give))
	n.setA(int16(r))
	g.netDipPost(amb)
	if r == -1 {
		d.message, err = g.textMessage(jimText2, 0x32, 0xfa) // Get_Human_Tech_Exchange_Decline_Message_
		return err
	}
	if d.message, err = g.textMessage(jimText2, 0x31, 0xfa); err != nil { // ..._Accept_Message_
		return err
	}
	g.changeRelations(12, p, amb, 0, 0, 0)
	g.playerGetsTechApp(p, want)
	g.playerGetsTechApp(amb, give)
	return nil
}

// getNetDiplomacyChoices is Get_Net_Diplomacy_Choices_ (0x1def8): the talk
// between two humans, the talker choosing from the menu and the listener
// answering, until one says good bye.
func (g *Game) getNetDiplomacyChoices() error {
	e, s, d := g.e, &g.state, &g.dip
	n := &d.net
	me := int(s.PlayerNum)
	n.inTalk = true
	defer func() { n.inTalk = false }()
	n.chatMode = false
	n.chatX, n.chatY, n.chatW, n.chatH = 0xa, 0x8d, 0x12c, 0x32
	d.oldResponse = 0
	n.chatMsg[0] = 0
	d.message = ""
	d.response = 0
	// Both sides send code 0x3f4 and wait for the other's.
	other := int(n.talker)
	if int(n.talker) == me {
		other = int(n.listener)
	}
	if err := g.getListenModeMessage(other); err != nil {
		return err
	}
	n.set(uint8(other), uint8(me))
	n.setCode(netDipSynch)
	n.setB(-1)
	g.netDipPost(other)
	for {
		if err := g.diplomacySynchMode(); err != nil {
			return err
		}
		if n.synchUp != 0 {
			break
		}
	}
	done := false
	d.message = ""
	d.oldResponse = 0
	d.response = 0
	if int(n.talker) != me {
		if err := g.getListenModeMessage(int(n.talker)); err != nil {
			return err
		}
	}
	e.SetInputDelay(1)
	for !done && n.escPressed == 0 {
		e.AssignAutoFunction(g.drawDiplomacySynchMode, 2)
		t, l := int(n.talker), int(n.listener)
		var err error
		if t == me {
			n.chatMode = false
			if s.Players[t].Picks[19] == 1 || s.Players[l].Picks[19] == 1 {
				err = g.getMainRepulsiveDiplomacyChoices(t, l, true)
			} else {
				done, err = g.netTalkerMenu(t, l)
			}
			e.ClearFields()
		} else {
			done, err = g.netListenerStep(t, l)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// netTalkerMenu is Get_Net_Diplomacy_Choices_' menu (0x1e0c4..0x1e35c); true
// when the talk ends.
func (g *Game) netTalkerMenu(t, l int) (bool, error) {
	d := &g.dip
	n := &d.net
	active := make([]byte, 11)
	for i := range active {
		active[i] = 1
	}
	g.diplomacySetMainOptions(t, l, active)
	g.diplomacyListStyle(false)
	g.setupDiplomacyHelp(0xb, nil)
	c, err := g.diplomacyList(d.texts.mainTitle, d.texts.netList[:], active)
	if err != nil {
		return false, err
	}
	switch c + 1 {
	case 0, 10: // Esc, Good Bye
		n.set(uint8(t), uint8(l))
		n.setCode(netDipExit)
		n.setB(-1)
		g.netDipPost(l)
		return true, nil
	case 1:
		d.message = ""
		err = g.diplomacyProposeTreaty(t, l)
		n.chatMode = false
		d.response, d.oldResponse = 0, 0
	case 2:
		d.message = ""
		err = g.diplomacyBreakTreaty(t, l)
		d.oldResponse, d.response = 0, 0
		d.message = ""
	case 3:
		d.message = ""
		err = g.diplomacyDemand(t, l)
	case 4:
		d.message = ""
		err = g.diplomacyOfferGift(t, l)
	case 5:
		d.message = ""
		err = g.diplomacyExchangeTechnology(t, l)
	case 6:
		var war bool
		if war, err = g.diplomacyDeclareWarResolution(t, l); err == nil && war {
			err = g.diplomacyDisplayResponse()
		}
	case 7:
		var gone bool
		if gone, err = g.diplomacySurrender(t, l); err == nil && gone {
			return true, nil
		}
	case 8:
		if err = g.diplomacyMainNetChat(l, t); err == nil && n.escPressed == 1 {
			return true, nil
		}
	case 9: // the other side's turn to talk
		g.diplomacyNetSwitcheroo()
		err = g.getListenModeMessage(int(n.talker))
	}
	return false, err
}

// netListenerStep is Get_Net_Diplomacy_Choices_' listening side
// (0x1e361..): a new code from the talker is answered, else one frame of
// Diplomacy_Listen_Mode_. True when the talk ends.
func (g *Game) netListenerStep(t, l int) (bool, error) {
	e, s, d := g.e, &g.state, &g.dip
	n := &d.net
	done := false
	if !n.chatMode {
		e.ClearFields()
	}
	if n.escPressed == 1 {
		done = true
	}
	r := d.response
	if r == d.oldResponse || r == 0x80 {
		return done, g.diplomacyListenMode(t, l)
	}
	n.chatMode = false
	answer := func(code int16, b int16) {
		n.set(uint8(t), uint8(l))
		n.setCode(int(code))
		n.setB(b)
		g.netDipPost(t)
	}
	var err error
	switch r {
	case netDipExit:
		if n.escPressed == 0 {
			err = g.diplomacyDisplayResponse()
		}
		done = true
	case netDipTreaty:
		d.response = n.b()
		n.treatyType = n.a()
		var c int
		if c, err = g.getHumanTreatyChoice(t, l, uint16(n.a())); err != nil {
			return done, err
		}
		d.oldResponse = d.response
		answer(d.response, int16(c))
		d.response, d.oldResponse = 0, 0
		switch c {
		case 3:
			err = g.startNetTreaty()
		case 0, 1:
			err = g.getListenModeMessage(t)
		}
	case netDipBreak:
		d.response = n.b()
		if err = g.getHumanBreakTreatyMessage(t, uint16(n.a())); err != nil {
			return done, err
		}
		switch n.a() {
		case 1, 2:
			g.breakTreaties(int16(t), int16(l))
		case 7:
			g.breakTrade(int16(t), int16(l))
		case 8:
			g.breakResearch(int16(t), int16(l))
		case 9, 10:
			g.breakTribute(int16(t), int16(l))
		}
		d.response, d.oldResponse = 0, 0
	case netDipWar:
		err = g.getHumanMessageNamed(t, 0xe)
		d.response, d.oldResponse = 0, 0
		g.declareWar(int16(t), int16(l))
	case netDipSurrender:
		d.response, d.oldResponse = 0, 0
		done = true
		if err = g.getHumanMessageNamed(t, 0xd); err == nil {
			err = g.diplomacyDisplayResponse()
		}
	case netDipGift:
		d.response, d.oldResponse = 0, 0
		a, b := n.a(), n.b()
		if err = g.getHumanOfferGiftMessage(t, uint16(a), b); err != nil {
			return done, err
		}
		d.response, d.oldResponse = 0, 0
		switch a {
		case 1:
			s.Players[t].BC -= int32(b)
			s.Players[l].BC += int32(b)
		case 2:
			g.startTributeTreaty(int16(t), int16(l), b)
		case 3:
			g.playerGetsTechApp(l, int(b))
		case 4:
			if s.maySurrenderStar(int(b), t, l) {
				s.surrenderStar(int(b), t, l)
			}
		}
	case netDipDemand:
		d.response, d.oldResponse = 0, 0
		a, b := n.a(), n.b()
		if err = g.getHumanDemandMessage(t, l, uint16(a), b); err != nil {
			return done, err
		}
		var c int
		if c, err = g.getHumanDemandChoice(l, t, uint16(a), b); err != nil {
			return done, err
		}
		d.response = 0xa8
		if c == 1 {
			g.diplomacyGiveInToDemand(t, l, uint16(a), b)
			d.response = 0xa9
		}
		d.oldResponse = d.response
		answer(d.response, int16(c))
		err = g.getListenModeMessage(t)
	case netDipSystem:
		d.response = d.oldResponse
		a := n.a()
		var c int
		if c, err = g.diplomacyGenericSystemList(int(a), t, l, false); err != nil {
			return done, err
		}
		b := int16(0)
		if c != -1 {
			if s.maySurrenderStar(int(a), t, l) {
				s.surrenderStar(int(a), t, l)
			}
			if err = g.startNetTreaty(); err != nil {
				return done, err
			}
			b = 1
		} else if err = g.getListenModeMessage(t); err != nil {
			return done, err
		}
		answer(d.response, b)
		d.response, d.oldResponse = 0, 0
	case netDipTech, netDipMoney:
		d.response = d.oldResponse
		a := n.a()
		var row string
		if r == netDipTech {
			name, err := g.technologyApplicationsName(int(a), t)
			if err != nil {
				return done, err
			}
			row = cSprintf(cStringAt(g.exe, 0x178a8a), name)
		} else {
			row = cSprintf(cStringAt(g.exe, 0x178aa8), int(a))
		}
		title, err := g.textMessage(jimText2, 9, 0x28)
		if err != nil {
			return done, err
		}
		c, err := g.diplomacyGenericList([]string{row}, true, title)
		if err != nil {
			return done, err
		}
		b := int16(0)
		if c != -1 {
			if r == netDipTech {
				g.playerGetsTechApp(l, int(a))
			} else {
				s.Players[t].BC -= int32(a)
				s.Players[l].BC += int32(a)
			}
			if err := g.startNetTreaty(); err != nil {
				return done, err
			}
			b = 1
		} else if err := g.getListenModeMessage(t); err != nil {
			return done, err
		}
		answer(d.response, b)
		d.response, d.oldResponse = 0, 0
	case netDipAskTech:
		d.response, d.oldResponse = 0, 0
		a := n.a()
		if err = g.getHumanTechExchangeFirstMessage(t, l, int(a)); err != nil {
			return done, err
		}
		n.techOffered = a
		title, err := g.textMessage(jimText2, 4, 0x28)
		if err != nil {
			return done, err
		}
		list, _ := g.getDifferentialTechList(l, t, 0, 0xc343cb00, true)
		c, err := g.diplomacyGenericExchangeTechList(l, title, list, false)
		if err != nil {
			return done, err
		}
		b := int16(c)
		if c >= 0 {
			b = int16(list[c])
		}
		answer(d.response, b)
		err = g.getListenModeMessage(t)
	case netDipGiveTech:
		d.response, d.oldResponse = 0, 0
		if n.a() != -1 {
			g.playerGetsTechApp(t, int(n.techOffered))
			g.playerGetsTechApp(l, int(n.b()))
			d.message, err = g.textMessage(jimText2, 0x31, 0xfa) // Get_Human_Tech_Exchange_Accept_Message_
		} else {
			d.message, err = g.textMessage(jimText2, 0x32, 0xfa) // Get_Human_Tech_Exchange_Decline_Message_
		}
		d.response, d.oldResponse = 0, 0
	case netDipSwitch:
		d.response, d.oldResponse = 0, 0
		d.message = ""
	}
	return done, err
}

// getHumanMessageNamed is Get_Human_Declare_War_Message_ (n 0xe) and
// Get_Human_Surrender_Message_ (n 0xd): jimtext2 n naming p.
func (g *Game) getHumanMessageNamed(p, n int) error {
	t, err := g.textMessage(jimText2, n, 0xfa)
	if err != nil {
		return err
	}
	g.dip.message, err = g.decodeTextMessage(t, p, -1, -1, -1, -1, -1, -1, 0xfa)
	return err
}

// getHumanBreakTreatyMessage is Get_Human_Break_Treaty_Message_ (0x1883a).
func (g *Game) getHumanBreakTreatyMessage(p int, kind uint16) error {
	n := map[uint16]int{7: 0x1b, 8: 0x1c, 9: 0x1d, 0xa: 0x1d, 1: 0x1e, 2: 0x1f}[kind]
	if n == 0 {
		return nil
	}
	return g.getHumanMessageNamed(p, n)
}

// getHumanTechExchangeFirstMessage is Get_Human_Tech_Exchange_First_Message_
// (0x18ae2): "<p> would like <want> from <amb>".
func (g *Game) getHumanTechExchangeFirstMessage(p, amb, want int) error {
	t, err := g.textMessage(jimText2, 0x30, 0xfa)
	if err != nil {
		return err
	}
	g.dip.message, err = g.decodeTextMessage(t, p, -1, -1, -1, amb, want, -1, 0xfa)
	return err
}
