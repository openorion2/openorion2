package game

import (
	"fmt"
	"strconv"

	"github.com/openorion2/openorion2/internal/gfx"

	"github.com/openorion2/openorion2/internal/engine"
	"github.com/openorion2/openorion2/internal/font"
	"github.com/openorion2/openorion2/internal/save"
)

// The AI ambassador's screen (DIP-SCRN.C): Npc_Diplomacy_Screen_ and what it
// shows. Spec: docs/screens/diplomacy-screen.md.

// diplomacyScreen holds DIP-SCRN.C's globals.
type diplomacyScreen struct {
	ambassador      int   // _current_ambassador
	response        int16 // _response_message
	oldResponse     int16 // _old_response_message
	option          byte  // _ambassador_option
	goodMusic       int16 // _diplomacy_good_music
	badMusic        int16 // _diplomacy_bad_music
	currentMusic    int16 // _diplomacy_current_music
	message         string
	listChoice      [2]string // _diplomacy_list_choice: accept, reject
	listVar         [4]int16  // Get_Non_Player_Proposals_' local_1c
	playerNameAmb   string    // _player_name_ambassador_buffer
	playerNumName   string    // _player_number_name_buffer
	framed          string    // _you_were_framed_buffer
	percentTribute  string    // _percent_tribute_treaty_buffer
	statementTrade  string    // _statement_trade_treaty_buffer
	statementResrch string    // _statement_research_treaty_buffer

	// The human's side (Diplomacy_Screen_).
	targetTech   []uint8 // _target_tech_list, _exchange_tech_count long
	exchangeTech []uint8 // _exchange_tech_list
	texts        diplomacyTexts
	// Diplomacy_System_Display_'s state.
	sys diplomacySystem
	// The talk between two humans (netdiplo_talk.go).
	net diplomacyNet
}

// Colours of the diplomacy text in .data.
const (
	diplomacyNormalColors    = 0x180dcc
	diplomacyHighlightColors = 0x180dd4
	diplomacySpecialColors   = 0x180ddc
)

// diplomatColorShade is the title colour by banner, the cases of a switch
// in Draw_Diplomacy_Synch_Mode_.
var diplomatColorShade = [8]byte{0xf3, 0xf0, 0xfd, 0xe3, 0xf5, 0xeb, 0xf4, 0xf1}

const (
	diplomatLBX = "DIPLOMAT.LBX"
	jimText     = "jimtext.lbx"
	jimText2    = "jimtext2.lbx"
)

// npcDiplomacyScreen is Npc_Diplomacy_Screen_ (0x1afa6; its body sits at
// 0x1a570..0x1abbd): the ambassador of amb delivers his message to the
// human, who answers demands and proposals. With delayed set the message
// is the delayed one (+0x7c9).
func (g *Game) npcDiplomacyScreen(amb int, delayed bool) error {
	e, s, d := g.e, &g.state, &g.dip
	if s.Players[amb].Eliminated == 1 {
		return nil
	}
	pn := int(s.PlayerNum)
	e.DeactivateHelpList()
	e.EnableDrawAllFields()
	e.V.ResetWindow()
	e.V.ClipOff()
	e.SetMouseList(engine.MouseNone)
	saved := e.GetAutoFunction()
	e.DeactivateAutoFunction()
	e.ClearFields()
	d.oldResponse = -1
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
	d.option = 1
	if delayed {
		d.response = int16(s.Players[amb].Diplo7C9[0])
	} else {
		d.response = int16(s.Players[amb].MessageFlags[pn])
	}
	if err := g.diplomacyFadeIn(); err != nil {
		return err
	}
	msg := d.response
	if msg >= 0x53 && msg <= 0x55 || msg >= 0x61 && msg <= 0x68 || msg == 0x7d || msg == 0x56 ||
		msg == 0x69 || msg == 0x6a || msg >= 0x1c && msg <= 0x24 {
		d.option = 2
		d.response = 0x80
		if s.Players[pn].Relations[amb] < -12 {
			d.response = 0x81
		}
	}
	P, A := int16(pn), int16(amb)
	ally := func() int16 { return int16(int8(s.Players[amb].AllianceCall[pn])) }
	if d.option == 1 {
		if err := g.diplomacyDisplayResponse(); err != nil {
			return err
		}
		switch r := uint16(d.response); {
		case r >= 0x30 && r <= 0x33:
			g.breakTrade(P, A)
			g.breakResearch(P, A)
			g.breakTribute(P, A)
			g.breakTreaties(P, A)
		case r == 0x39 || r == 0x3a:
			g.declareWar(P, A)
		case r == 0x60 || r == 0x6b:
			g.npcGiveGift(amb, pn)
		}
		return g.npcDiplomacyScreenEnd(saved)
	}
	if s.Players[amb].Objective != objectiveHuman {
		if err := g.diplomacyDisplayResponse(); err != nil {
			return err
		}
	}
	d.response = msg
	choice, err := g.getNonPlayerProposals()
	if err != nil {
		return err
	}
	if choice != 0 {
		g.npcProposalRejectionAccept(amb, pn, false)
		show := true
		switch r := uint16(d.response); {
		case r >= 0x1c && r <= 0x24:
			g.breakTrade(P, A)
			g.breakResearch(P, A)
			g.breakTribute(P, A)
			g.breakTreaties(P, A)
			d.response = 0x25
		case r == 0x56:
			g.breakTreaties(P, A)
			d.response = 0x57
		case r == 0x68:
			g.breakTreaties(P, A)
			show = false
		case r == 0x69:
			g.changeRelations(-50, amb, pn, 0, 0, 0)
			d.response = 0x25
		case r == 0x6a:
			if st := s.Players[amb].SneakStar[pn]; st != -1 {
				s.Players[amb].AIWarTarget = st
				s.Players[amb].Diplo7C9[0] = s.Players[amb].Diplo887[pn]
				show = false
			} else {
				g.declareWar(A, P)
				d.response = 0xb3
			}
		default:
			show = false
		}
		if show {
			if err := g.diplomacyDisplayResponse(); err != nil {
				return err
			}
		}
		return g.npcDiplomacyScreenEnd(saved)
	}
	g.npcProposalRejectionAccept(amb, pn, true)
	switch r := uint16(d.response); {
	case r >= 0x1c && r <= 0x24, r == 0x69 || r == 0x6a:
		d.response = 0x26
		if err := g.diplomacyDisplayResponse(); err != nil {
			return err
		}
		g.acceptedNpcDemandResolution(pn, amb)
		g.npcGiveGift(pn, amb)
	case r == 0x53:
		g.declarePeace(P, A)
	case r == 0x54:
		g.npcGiveGift(pn, amb)
		g.declarePeace(P, A)
		g.acceptedNpcDemandResolution(pn, amb)
	case r == 0x55:
		g.npcGiveGift(amb, pn)
		g.declarePeace(P, A)
	case r == 0x56:
		g.declareWar(P, ally())
		d.response = 0x58
		if err := g.diplomacyDisplayResponse(); err != nil {
			return err
		}
	case r == 0x61:
		g.startTreaty(P, A, 1)
	case r == 0x62:
		g.startTreaty(P, A, 2)
	case r == 0x63:
		g.startTradeTreaty(P, A)
	case r == 0x64:
		g.startResearchTreaty(P, A)
	case r == 0x66:
		g.declareWar(P, ally())
	case r == 0x67:
		g.npcGiveGift(amb, pn)
		g.declareWar(P, ally())
	case r == 0x68:
		g.breakTreaties(P, ally())
	}
	return g.npcDiplomacyScreenEnd(saved)
}

// npcDiplomacyScreenEnd is the tear-down at 0x1ab69.
func (g *Game) npcDiplomacyScreenEnd(saved engine.AutoFunction) error {
	e := g.e
	e.FadeStreamDown(0xa) // 0x1ab6e
	e.DeactivateAutoFunction()
	e.PutAutoFunction(saved)
	e.ClearFields()
	e.SetFadePercent(0)
	e.SetFadeSpeedPercent(10)
	if err := e.FastFadeOut(); err != nil {
		return err
	}
	g.mus.current = -1 // 0x1abac: _jim_current_stream
	g.playBackgroundMusic()
	return nil
}

// diplomacyFadeIn is Diplomacy_Fade_In_ (0x17c07): load the ambassador, word
// the message and, with animations on, play the opening frames until a
// click or a key.
func (g *Game) diplomacyFadeIn() error {
	e, d := g.e, &g.dip
	if err := g.diplomacyMouseInit(d.ambassador); err != nil {
		return err
	}
	if err := g.getDiplomacyStatement(d.response, int(g.state.PlayerNum), d.ambassador); err != nil {
		return err
	}
	if err := g.setupBackPage(); err != nil {
		return err
	}
	e.SetMouseList(engine.MouseArrow)
	if d.option == 0 {
		return nil
	}
	e.AssignAutoFunction(g.drawDiplomacyFadeInScreen, 2)
	e.SetFileAnimationFrame(1)
	skipped := g.settings[6] == 0
	for i := 0; i < 0x26 && !skipped; i++ {
		e.MarkTime()
		e.ClearFields()
		f, err := e.AddHiddenField(0, 0, 0x27f, 0x1df, "", 0)
		if err != nil {
			return err
		}
		in, err := e.GetInput()
		if err != nil {
			return err
		}
		if in == f || e.Sys.KeyWaiting() {
			skipped = true
		}
		if !skipped {
			e.V.SetPageOff()
			if err := e.DrawFileAnimationStencil(); err != nil {
				return err
			}
			g.playStreamingMusic(g.dip.currentMusic+100, -1, true) // 0x17cec
			if err := e.TogglePages(); err != nil {
				return err
			}
			if err := e.ReleaseTime(2); err != nil {
				return err
			}
		}
	}
	return g.setupAmbassadorPic(skipped)
}

// diplomacyMouseInit is Diplomacy_Mouse_Init_ (0x1b92e): the race's palette
// and texts, and the music for bad news, picked at random.
func (g *Game) diplomacyMouseInit(amb int) error {
	e, d, s := g.e, &g.dip, &g.state
	race := int(s.Players[amb].Race)
	icon, err := g.picture(diplomatLBX, race)
	if err != nil {
		return err
	}
	// The race icon becomes cursor image 0, the arrow's: the screen's cursor.
	// Load_Palette_ brings the arrow back after the screen.
	e.ClearMouseImage(0)
	cursor := gfx.NewBlankBitmap(0x1e, 0x1e)
	cursor.DrawPictureToBitmap(icon)
	e.V.SetPageOff()
	e.V.Fill(0, 0, 0x27f, 0x1df, 0)
	e.DrawPalette(icon)
	e.UpdateGlassRemapColors()
	e.V.SetPaletteChanges(0, 0xff)
	e.V.Fill(0, 0, 0x27f, 0x1df, 0)
	e.DrawBitmapToMouse(0, 0, cursor, 0)
	// DIPSTARS.LBX: the system map's background and its eleven stars, which
	// share _fleet_galaxy_star_seg with the fleet screen.
	if d.sys.starBg, err = g.picture("DIPSTARS.LBX", 0); err != nil {
		return err
	}
	for i := range g.flt.galaxyStars {
		if g.flt.galaxyStars[i], err = g.picture("DIPSTARS.LBX", i+1); err != nil {
			return err
		}
	}
	if err := g.loadDiplomacyTextMessages(); err != nil {
		return err
	}
	d.goodMusic = int16(race) + 1
	d.currentMusic = -1
	d.badMusic = int16(s.Random.Random(3)) + 0xd
	return nil
}

// loadDiplomacyTextMessages is the part of Load_Diplomacy_Text_Messages_
// (0x1f37c) this screen reads.
func (g *Game) loadDiplomacyTextMessages() error {
	d := &g.dip
	for _, t := range []struct {
		n   int
		dst *string
	}{
		{0x43, &d.playerNumName}, {0x44, &d.playerNameAmb}, {0x45, &d.framed},
		{0x48, &d.percentTribute}, {0x49, &d.statementTrade}, {0x4a, &d.statementResrch},
	} {
		s, err := g.textMessage(jimText2, t.n, 0x28)
		if err != nil {
			return err
		}
		*t.dst = s
	}
	return g.loadDiplomacyTexts()
}

// textMessage is Get_Text_Message_ (0x249f9): record 0 of entry n*6+language
// of file; the caller's buffer holds max bytes.
func (g *Game) textMessage(file string, n, max int) (string, error) {
	lang := int(int16(g.settings[0xd2]))
	if lang > 5 || lang < 0 {
		lang = 0
	}
	a, err := g.archive(file)
	if err != nil {
		return "", err
	}
	t, err := a.Table(lang + n*6)
	if err != nil {
		return "", err
	}
	r := t.Record(0)
	s := save.CString(r[:min(len(r), 100)])
	if len(s) >= max-1 || len(s) >= 99 {
		return "", fmt.Errorf("Message %d in %s too long (%d)", n, file, len(s))
	}
	return s, nil
}

// setupBackPage is Setup_Back_Page_ (0x17b51): the room with the
// ambassador's first frame on both pages.
func (g *Game) setupBackPage() error {
	e, s := g.e, &g.state
	g.fullDrawScreen = false
	p, err := g.picture(diplomatLBX, int(s.Players[g.dip.ambassador].Race)*2+0xd)
	if err != nil {
		return err
	}
	e.OpenFileAnimation(p)
	e.V.SetPageOff()
	e.V.Fill(0, 0, 0x27f, 0x1df, 0)
	if err := e.DrawFileAnimation(p); err != nil {
		return err
	}
	e.V.CopyOffToBack()
	return nil
}

// setupAmbassadorPic is Setup_Ambassador_Pic_ (0x17bb2): the ambassador's
// loop, from its first frame when the opening was skipped.
func (g *Game) setupAmbassadorPic(skipped bool) error {
	e, s := g.e, &g.state
	p, err := g.picture(diplomatLBX, int(s.Players[g.dip.ambassador].Race)*2+0xe)
	if err != nil {
		return err
	}
	e.OpenFileAnimation(p)
	if skipped {
		e.SetFileAnimationFrame(0)
	} else {
		e.SetFileAnimationFrame(1)
	}
	if g.settings[6] == 0 {
		return e.DrawFileAnimationStencil()
	}
	return nil
}

// drawDiplomacyFadeInScreen is Draw_Diplomacy_Fade_In_Screen_ (0x17d1d).
func (g *Game) drawDiplomacyFadeInScreen() error {
	g.e.V.SetPageOff()
	err := g.e.DrawFileAnimationStencil()
	g.playStreamingMusic(g.dip.currentMusic+100, -1, true) // 0x17d3f
	return err
}

// diplomacyDisplayResponse is Diplomacy_Display_Response_ (0x19919): show
// the current message until the player clicks.
func (g *Game) diplomacyDisplayResponse() error {
	e := g.e
	e.AssignAutoFunction(g.drawDiplomacySynchMode, 2)
	e.DeactivateHelpList()
	hot := ""
	if g.listKey {
		hot = "1" // Options.ListKey: this field's hot key too
	}
	for {
		e.MarkTime()
		e.ClearFields()
		f, err := e.AddHiddenField(0, 0, 0x27f, 0x1df, hot, 0)
		if err != nil {
			return err
		}
		in, err := e.GetInput()
		if err != nil {
			return err
		}
		if in == f {
			break
		}
		if err := g.drawDiplomacySynchMode(); err != nil {
			return err
		}
		if err := e.TogglePages(); err != nil {
			return err
		}
		if err := e.ReleaseTime(2); err != nil {
			return err
		}
	}
	e.ClearFields()
	e.DeactivateAutoFunction()
	return nil
}

// drawDiplomacySynchMode is Draw_Diplomacy_Synch_Mode_ (0x16f00): the
// ambassador, his words and the title. A new message is worded once, which
// draws its variant at random.
func (g *Game) drawDiplomacySynchMode() error {
	e, d, s := g.e, &g.dip, &g.state
	e.V.SetPageOff()
	if g.fullDrawScreen {
		if err := g.setupBackPage(); err != nil {
			return err
		}
		if err := g.setupAmbassadorPic(true); err != nil {
			return err
		}
	}
	if d.option != 0 && g.settings[6] == 1 {
		if err := e.DrawFileAnimationStencil(); err != nil {
			return err
		}
		if e.FileAnimationFrame() == 0 {
			e.SetFileAnimationFrame(1)
		}
	}
	t := e.Text()
	t.SetOutlineColor(0, 0xf8)
	t.SetStyleShadow(4, g.exeColors(diplomacyNormalColors), font.Outline)
	if d.response != d.oldResponse {
		if d.response < 1000 {
			d.oldResponse = d.response
			if err := g.getDiplomacyStatement(d.response, int(s.PlayerNum), d.ambassador); err != nil {
				return err
			}
		} else {
			d.response, d.oldResponse = 0, 0
		}
	}
	h, err := g.formattedParagraphMaxHeight(0x1d6, d.message)
	if err != nil {
		return err
	}
	if err := g.printFormattedParagraph(0x50, int(int16(0x1b8-h/2)), 0x1d6, d.message, 2); err != nil {
		return err
	}
	amb := &s.Players[d.ambassador]
	var c font.Colors
	for i := 1; i < 8; i++ {
		c[i] = diplomatColorShade[amb.Color&7]
	}
	t.SetOutlineColor(0, 0)
	t.SetStyleShadow(4, c, font.ShadowDown)
	var title string
	if amb.Objective == objectiveHuman {
		if title, err = g.decodeTextMessage(d.playerNumName, d.ambassador, -1, -1, int32(d.ambassador)+1, -1, -1, -1, 0x78); err != nil {
			return err
		}
	} else {
		if title, err = g.decodeTextMessage(d.playerNameAmb, d.ambassador, -1, -1, -1, -1, -1, -1, 0x78); err != nil {
			return err
		}
	}
	if err := t.PrintCentered(0x140, 10, title); err != nil {
		return err
	}
	if g.gameType >= 2 && amb.Objective == objectiveHuman && d.net.recvChat[0] != 0 {
		// The other human's chat line (msg 0x834).
		t.SetOutlineColor(0xf8, 0xf8)
		t.SetStyleShadow(4, g.exeColors(diplomacyNormalColors), font.ShadowDown)
		line := cSprintf(cStringAt(g.exe, 0x178a65), save.CString(amb.EmperorName[:]), save.CString(d.net.recvChat[:]))
		if err := t.PrintCentered(0x140, 0x1d, line); err != nil {
			return err
		}
	}
	g.playStreamingMusic(d.currentMusic+100, -1, true) // 0x171df
	e.MoxUpdate()
	if d.net.escPressed == 1 {
		e.ExitListField()
	}
	t.SetOutlineColor(0xf8, 0xf8)
	return nil
}

// getNonPlayerProposals is Get_Non_Player_Proposals_ (0x1ac12): the player's
// answer, 0 accept, 1 or a cancel reject; -1 when nothing was chosen.
func (g *Game) getNonPlayerProposals() (int, error) {
	e, d := g.e, &g.dip
	active := make([]byte, 10)
	for i := range active {
		active[i] = 1
	}
	e.DeactivateHelpList()
	d.listVar[2], d.listVar[3] = -1000, -1
	title, err := g.textMessage(jimText, 0x4a, 0x50)
	if err != nil {
		return 0, err
	}
	r := d.response
	if r >= 0x1c && r <= 0x24 || r >= 0x53 && r <= 0x56 || r >= 0x61 && r <= 0x64 || r >= 0x66 && r <= 0x6a || r == 0x7d {
		for i, n := range []int{0x4f, 0x50} {
			if d.listChoice[i], err = g.textMessage(jimText, n, 0x32); err != nil {
				return 0, err
			}
		}
	}
	fallback := 1
	e.AssignAutoFunction(g.drawDiplomacySynchMode, 2)
	e.SetInputDelay(1)
	d.listVar[0], d.listVar[1] = 1, 0
	t := e.Text()
	t.SetStyle(4, g.exeColors(diplomacyNormalColors))
	t.SetHighlightColors(g.exeColors(diplomacyHighlightColors))
	t.SetSpecialColors(g.exeColors(diplomacySpecialColors))
	t.SetLF(4)
	e.SetHelpList([]engine.HelpEntry{{ID: 0x2b6, X1: 0, Y1: 0, X2: 0x27f, Y2: 0x1df}})
	var c int
	switch r {
	case 0x65:
		// The ambassador's technologies up to the value he asked for; the
		// index chosen is the answer, so only the first counts as a yes.
		fallback, d.listVar[2] = -1, -1
		pn, amb := int(g.state.PlayerNum), d.ambassador
		A := &g.state.Players[amb]
		techs, _ := g.getDifferentialTechList(pn, amb, 0, uint32(A.TechExchangeValue[pn]), true)
		t, err := g.textMessage(jimText, 0x51, 0x50)
		if err != nil {
			return 0, err
		}
		if c, err = g.diplomacyGenericExchangeTechList(amb, t, techs, true); err != nil {
			return 0, err
		}
		if c != -1 {
			g.playerGetsTechApp(pn, int(techs[c]))
			g.playerGetsTechApp(amb, int(A.TechExchangeApp[pn]))
		}
	case 0x7d:
		e.ClearFields()
		if _, err := e.AddHiddenField(0, 0, 0x27f, 0x1df, "", 0); err != nil {
			return 0, err
		}
		for {
			in, err := e.GetInput()
			if err != nil {
				return 0, err
			}
			if in != 0 {
				break
			}
			e.MarkTime()
			if err := g.drawDiplomacySynchMode(); err != nil {
				return 0, err
			}
			if err := e.TogglePages(); err != nil {
				return 0, err
			}
			if err := e.ReleaseTime(2); err != nil {
				return 0, err
			}
		}
		c = 1
	default:
		if c, err = e.GetListField(10, 0x76, 0xf5, title, d.listChoice[:], &d.listVar[0], active, 0x11, [4]int{}); err != nil {
			return 0, err
		}
	}
	if c == -1 {
		c = fallback
	}
	if int16(c) == d.listVar[2] {
		c = -1
	}
	return c, nil
}

// getDiplomacyStatement is Get_Diplomacy_Statement_ (0x1d565): one of the
// message's wordings in DIPLOMS?.LBX, picked at random, with the names and
// terms filled in; p is the human, amb the ambassador.
func (g *Game) getDiplomacyStatement(msg int16, p, amb int) error {
	s, d := &g.state, &g.dip
	file := "DIPLOMSE.LBX"
	switch g.settings[0xd2] {
	case 1:
		file = "DIPLOMSG.LBX"
	case 2:
		file = "DIPLOMSF.LBX"
	case 3:
		file = "DIPLOMSS.LBX"
	case 4:
		file = "DIPLOMSI.LBX"
	case 5:
		file = "DIPLOMSP.LBX"
	}
	framed := false
	switch msg {
	case 0x14, 0x16, 0x1d, 0x1f, 0x28, 0x2a, 0x31, 0x33, 0x3a, 0x3c, 0x48, 0x4a, 0x8f, 0x91:
		framed = true
	}
	entry := int(msg)
	if framed || msg == 0x6a {
		entry--
	}
	a, err := g.archive(file)
	if err != nil {
		return err
	}
	tb, err := a.Table(entry)
	if err != nil {
		return err
	}
	rec := tb.Record(0)
	if len(rec) < 2 {
		return fmt.Errorf("%s entry %d: short record", file, entry)
	}
	k := int(s.Random.Random(uint32(rec[1])))
	off := 2 + (k-1)*200
	var src []byte
	if off >= 0 && off < len(rec) {
		src = rec[off:min(len(rec), off+200)]
	}
	A := &s.Players[amb]
	name := func(i int) string { return save.CString(s.Players[i].EmperorName[:]) }
	race := func(i int) string { return save.CString(s.Players[i].RaceName[:]) }
	out := []byte{}
	for _, c := range src {
		if c == 0 {
			break
		}
		var ins string
		switch c {
		case 0x80:
			ins = name(p)
		case 0x81:
			ins = race(p)
		case 0x82:
			ins = race(amb)
		case 0x83:
			ins = name(amb)
		case 0x84:
			ins = an(g.settings[0xd2], race(p))
		case 0x85:
			ins = an(g.settings[0xd2], race(amb))
		case 0x86, 0x93:
			ins = save.CString(s.Stars[A.Diplo67F[p]].Name[:])
		case 0x87, 0x88, 0x91:
			ins = race(int(int8(A.AllianceCall[p])))
		case 0x89:
			if ins, err = g.technologyApplicationsName(int(A.TechExchangeApp[p]), amb); err != nil {
				return err
			}
		case 0x8a:
			switch {
			case A.TreatyStatus[p] == 1 || A.TreatyStatus[p] == 2:
				ins, err = g.treatyLabel(int(A.TreatyStatus[p]))
			case A.TradeTreaty[p] != 0:
				ins = d.statementTrade
			case A.ResearchTreaty[p] != 0:
				ins = d.statementResrch
			}
		case 0x8b:
			ins = name(int(int8(A.AllianceCall[p])))
		case 0x8c:
			if v := A.Diplo6E7[p]; v >= 1 && v <= 4 {
				ins, err = g.textMessage(jimText2, 0x4a+int(v), 0x50)
			} else {
				ins = cStringAt(g.exe, 0x178b05)
			}
		case 0x8d:
			ins, err = g.buildingName(int(A.Diplo66F[p]))
		case 0x8e:
			f := "%d.%d"
			if g.settings[0xd2] != 0 {
				f = "%d,%d"
			}
			ins = fmt.Sprintf(f, s.Stardate/10, s.Stardate%10)
		case 0x8f:
		case 0x90, 0x95:
			ins, err = g.statementOffer(p, amb)
		case 0x92:
			switch v := A.Diplo6DF[p]; v {
			case 7:
				ins = d.statementTrade
			case 8:
				ins = d.statementResrch
			default:
				ins, err = g.treatyLabel(int(v))
			}
		case 0x94:
			ins = race(int(int8(A.Diplo7F6[p])))
		default:
			out = append(out, c)
			continue
		}
		if err != nil {
			return err
		}
		out = append(out, ins...)
	}
	if framed {
		out = append(out, d.framed...)
	}
	music := d.goodMusic
	if rec[0] == 0 {
		music = d.badMusic
	}
	d.currentMusic = music
	d.message = string(out)
	return nil
}

// statementOffer is the term of an offer or demand (codes 0x90, 0x95): a
// tribute, a technology, money or a star.
func (g *Game) statementOffer(p, amb int) (string, error) {
	s, d := &g.state, &g.dip
	A := &s.Players[amb]
	switch {
	case A.TurnScratch[1][p] != 0:
		v := int32(10)
		if A.TurnScratch[1][p] == 1 {
			v = 5
		}
		return g.decodeTextMessage(d.percentTribute, -1, -1, -1, v, -1, -1, -1, 0x28)
	case A.TurnScratch[2][p] != 0:
		return g.technologyApplicationsName(int(A.TurnScratch[2][p]), amb)
	case A.Demand[p] != 0:
		return fmt.Sprintf(cStringAt(g.exe, 0x178b17), A.Demand[p]), nil
	case A.Diplo7B7[p] == -1:
		return "", nil
	}
	t, err := g.textMessage(jimText2, 0x3a, 0x50)
	if err != nil {
		return "", err
	}
	return g.decodeTextMessage(t, -1, int(A.Diplo7B7[p]), -1, -1, -1, -1, -1, 0x32)
}

// an is An_ (0x1de17): the English article for a word, nothing in other
// languages.
func an(lang byte, w string) string {
	if lang != 0 {
		return ""
	}
	if w != "" {
		switch w[0] {
		case 'A', 'a', 'E', 'e', 'I', 'i', 'O', 'o', 'U', 'u':
			return "an"
		}
	}
	return "a"
}

// treatyLabel is _treaty_labels[i] (E_Strings_(0x275+i), filled by
// Load_H_And_Pro_File_Strings_).
func (g *Game) treatyLabel(i int) (string, error) {
	es, err := g.eStrings()
	if err != nil {
		return "", err
	}
	if i < 0 || 0x275+i >= len(es) {
		return "", fmt.Errorf("treaty label %d", i)
	}
	return es[0x275+i], nil
}

// technologyApplicationsName is Technology_Applications_Name_ (0x10f7a1):
// the leveled applications (0xcc and up, one per hyper-advanced field) get
// p's level, as a roman numeral up to 20.
func (g *Game) technologyApplicationsName(app, p int) (string, error) {
	name, err := g.techAppName(app)
	if err != nil || int16(app) < 0xcc {
		return name, err
	}
	lvl := g.state.Players[p].HyperLevels[app-0xcc]
	name += " "
	if lvl < 0x15 && lvl != 0 {
		return name + cStringAt(g.exe, g.exe.u32(romanNumerals+4*uint32(lvl))), nil
	}
	return name + strconv.Itoa(int(lvl)), nil
}

// decodeTextMessage is Decode_Text_Message_ (0x24aca): codes 0x80..0x8a of
// t replaced by the names and numbers given (-1 for unused); the result
// must stay below max bytes.
func (g *Game) decodeTextMessage(t string, name, system, tech int, stat int32, p2, tech2, field int, max int) (string, error) {
	s := &g.state
	out := []byte{}
	for i := 0; i < len(t) && i < 200; i++ {
		var ins string
		var err error
		switch c := t[i]; c {
		case 0x80:
			ins = save.CString(s.Players[name].EmperorName[:])
		case 0x81:
			ins = save.CString(s.Stars[system].Name[:])
		case 0x82:
			ins = strconv.Itoa(int(stat))
		case 0x83:
			ins, err = g.technologyApplicationsName(tech, name)
		case 0x84:
			ins = save.CString(s.Players[p2].EmperorName[:])
		case 0x85:
			ins, err = g.technologyApplicationsName(tech2, p2)
		case 0x86:
			ins = save.CString(s.Players[name].RaceName[:])
		case 0x87:
			ins = save.CString(s.Players[p2].RaceName[:])
		case 0x88:
			ins = fmt.Sprintf("%d.%d", s.Stardate/10, s.Stardate%10)
			if g.settings[0xd2] != 0 {
				ins = fmt.Sprintf("%d,%d", s.Stardate/10, s.Stardate%10)
			}
		case 0x89:
			ins, err = g.technologyFieldsName(field)
		case 0x8a:
			ins = g.planetName(system)
		default:
			out = append(out, c)
			continue
		}
		if err != nil {
			return "", err
		}
		out = append(out, ins...)
	}
	if len(out) >= max-1 {
		return "", fmt.Errorf("Decode_Text_Message_: %q too long (%d)", out, max)
	}
	return string(out), nil
}

// acceptedNpcDemandResolution is Accepted_Npc_Demand_Resolution_ (0x1abbe):
// giving in pleases amb and ends the threat between them.
func (g *Game) acceptedNpcDemandResolution(p, amb int) {
	s := &g.state
	g.changeRelations(int16(s.Random.Random(13))+11, p, amb, 0, 0, 0)
	s.Players[p].DemandState[amb] = 0
	s.Players[amb].DemandState[p] = 0
}

// npcGiveGift is Npc_Give_Gift_ (0x1aeb5): from hands to to what was asked:
// a tribute, a technology, money or a star.
func (g *Game) npcGiveGift(from, to int) {
	s := &g.state
	F, T := &s.Players[from], &s.Players[to]
	switch {
	case F.TurnScratch[1][to] != 0:
		amount := int16(2)
		if F.TurnScratch[1][to] == 1 {
			amount = 1
		}
		g.startTributeTreaty(int16(from), int16(to), amount)
	case F.TurnScratch[2][to] != 0:
		g.playerGetsTechApp(to, int(F.TurnScratch[2][to]))
	case F.Demand[to] != 0:
		T.BC += int32(F.Demand[to])
		F.BC -= int32(F.Demand[to])
		if F.BC < 1 {
			F.BC = 0
		}
	case F.Diplo7B7[to] != -1:
		if st := int(F.Diplo7B7[to]); s.maySurrenderStar(st, from, to) {
			s.surrenderStar(st, from, to)
		}
	}
}

// npcProposalRejectionAccept is NPC_Proposal_Rejection_Accept_ (0x276e6):
// the ambassador's bias for p moves with the answer, by personality.
func (g *Game) npcProposalRejectionAccept(amb, p int, accepted bool) {
	s := &g.state
	A := &s.Players[amb]
	v := A.HumanBias[p]
	r := v
	if accepted {
		switch {
		case v >= 0, A.Personality == 0:
			r = v + 1
		case A.Personality <= 5:
			r = 1
		}
	} else {
		switch {
		case v <= 0:
			r = v - 1
		case A.Personality == 0:
			r = -1
		case A.Personality <= 3:
			r = v / 2
		case A.Personality == 4:
			if A.Diplo727[p] == 1 {
				r = -1
			}
		case A.Personality == 5:
			r = v - 1
		}
	}
	A.HumanBias[p] = r
	s.Players[p].HumanBias[amb] = r
}

// startTradeTreaty is Start_Trade_Treaty_ (0x5223f): a and b sign a trade
// treaty, which runs longer by both personalities.
func (g *Game) startTradeTreaty(a, b int16) {
	g.addTreatyTime(a, b)
	g.startTradeAgreement(a, b)
}

// startResearchTreaty is Start_Research_Treaty_ (0x52150).
func (g *Game) startResearchTreaty(a, b int16) {
	g.addTreatyTime(a, b)
	g.startResearchAgreement(a, b)
}

// addTreatyTime is the opening both treaty starters share: each side's
// treaty timer grows by its diplomacy personality's peace duration.
func (g *Game) addTreatyTime(a, b int16) {
	s := &g.state
	s.Players[a].TreatyTimer[b] += g.peaceDuration(s.getPlayerDiplomacyPersonality(a, b))
	s.Players[b].TreatyTimer[a] += g.peaceDuration(s.getPlayerDiplomacyPersonality(b, a))
}

// startTradeAgreement is Start_Trade_Agreement_ (0x101ee3).
func (g *Game) startTradeAgreement(a, b int16) {
	s := &g.state
	v := g.baseTradeAgreementGoal(a, b)
	A, B := &s.Players[a], &s.Players[b]
	A.TradeTreaty[b] = 1
	A.TradeValue[b] = int16(-v)
	A.TradeGoal[b] = g.tradeAgreementGoal(a, b)
	B.TradeTreaty[a] = 1
	B.TradeValue[a] = int16(-v)
	B.TradeGoal[a] = g.tradeAgreementGoal(b, a)
	g.updatePlayerStats(int(a))
	g.updatePlayerStats(int(b))
}

// startResearchAgreement is Start_Research_Agreement_ (0x101f82): both
// start at minus half the smaller research pool.
func (g *Game) startResearchAgreement(a, b int16) {
	s := &g.state
	A, B := &s.Players[a], &s.Players[b]
	v := min(A.ResearchPops, B.ResearchPops) / 2
	A.ResearchTreaty[b] = 1
	A.ResearchValue[b] = -v
	A.ResearchGoal[b] = g.researchAgreementGoal(a, b)
	B.ResearchTreaty[a] = 1
	B.ResearchValue[a] = -v
	B.ResearchGoal[a] = g.researchAgreementGoal(b, a)
	g.updatePlayerStats(int(a))
	g.updatePlayerStats(int(b))
}
