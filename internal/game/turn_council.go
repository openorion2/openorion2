package game

import (
	"fmt"

	"github.com/openorion2/openorion2/internal/engine"
	"github.com/openorion2/openorion2/internal/font"
	"github.com/openorion2/openorion2/internal/gfx"
	"github.com/openorion2/openorion2/internal/save"
)

// The High Council (COUNCIL.C): when it meets, who stands, the votes, the
// human's vote and accepting or defying the ruling. Spec:
// docs/screens/council.md.

const (
	councilLBXUpper   = 0x178a2b // "council.LBX"
	councilLBX        = 0x178a37 // "council.lbx"
	councilText       = 0x178a43 // "jimtext.lbx"
	councilNameFmt    = 0x178a4f // "  %s"
	councmsgLBX       = 0x178a54 // "councmsg.lbx"
	councilTotalFmt   = 0x178a25 // "%d %s"
	councilMouseList  = 0x180cb0 // the blank cursor over the whole screen
	councilColors     = 0x180cbc
	councilHighlight  = 0x180cc4
	councilStateDone  = 0x63
	councilMaxMessage = 0xc8
)

// The members' seats and the nominees' places (Draw_Main_Council_Screen_'s
// locals).
var (
	councilSeatX = [6]int{0x2c, 0x208, 0x8f, 0x1bc, 0xd7, 0x179}
	councilSeatY = [6]int{0x60, 0x64, 0x94, 0x95, 0xa9, 0xa8}
)

// councilState holds the COUNCIL.C globals (0x19a020..0x19a0de).
type councilState struct {
	nominees     [2]uint8 // _council_nominees
	members      [8]uint8 // _council_members, by votes, most first
	memberVotes  [8]int16 // _council_member_votes
	nomineeVotes [2]int16 // _council_nominee_votes
	nMembers     uint8    // _council_total_members
	totalVotes   int16    // _council_total_votes
	tally        [2]int16 // _council_nominee_vote_total
	state        uint8    // _council_state
	voteCast     int16    // _council_vote_cast
	voter        int8     // _council_voter
	winning      uint8    // _winning_player
	voteFor      uint8    // _council_vote_for

	winnerMsgID [20]int8           // _player_voted_winner_message_id
	nomineeSeg  [2]*gfx.Picture    // _council_nominees_seg
	lightSeg    [8]*gfx.Picture    // _council_light_race_seg: the voter
	darkSeg     [8]*gfx.Picture    // _council_dark_race_seg
	noVotesMsg  string             // _council_no_votes_message
	voteMsg     string             // _council_vote_message
	votesMsg    string             // _council_votes_message
	totalMsg    string             // _council_votes_total_message
	message     string             // _council_message
	showing     bool               // port-only: waiting for the click, for tests
	waits       int                // port-only: click waits so far, for tests
	choose      func(list int) int // port-only: the human's answers without a screen
}

// councilHeadless is true when the game runs without an engine (the step
// tests) or with Options.AutopilotNoCouncil: the screens are skipped and
// the human answers through choose (or councilNoScreenChoice).
func (g *Game) councilHeadless() bool { return g.e == nil || g.noCouncil }

// councilNoScreenChoice answers the human's lists under
// Options.AutopilotNoCouncil (port only): the vote goes to the human when he
// is a nominee, else he abstains; he accepts only his own election.
func (g *Game) councilNoScreenChoice(list int) int {
	s, c := &g.state, &g.council
	if list == 1 {
		if s.CouncilFlag == 2 {
			g.Log("council: the human is elected and accepts")
			return 0
		}
		g.Log("council: player %d is elected; the human defies the ruling", c.nominees[c.voteFor])
		return 1
	}
	for k := 0; k < 2; k++ {
		if int16(c.nominees[k]) == s.PlayerNum {
			g.Log("council: the human votes for himself")
			return k
		}
	}
	g.Log("council: the human abstains")
	return 2
}

// checkForCouncilMeetingStep is the Next_Turn_Calc_ call of
// Check_For_Council_Meeting_: single player games only, not all-AI ones.
func (g *Game) checkForCouncilMeetingStep() {
	if g.gameType == 0 && !g.allAIGame {
		if err := g.checkForCouncilMeeting(); err != nil {
			g.Log("Check_For_Council_Meeting_: %v", err)
		}
	}
}

// checkForCouncilMeeting is Check_For_Council_Meeting_ (0x168af): from turn
// 25, every 25 turns (or while COUNCIL_FLAG is 0), with at least three
// players left and half the stars owned.
func (g *Game) checkForCouncilMeeting() error {
	s := &g.state
	var alive, owned int16
	for p := int16(0); p < s.NumPlayers; p++ {
		if s.Players[p].Eliminated == 0 {
			alive++
		}
	}
	for st := int16(0); st < s.NumStars; st++ {
		if s.Stars[st].Owner != -1 {
			owned++
		}
	}
	t := int32(s.Stardate) - 35000
	if t < 25 || t < int32(s.NextCouncilTurn) && s.CouncilFlag != 0 {
		return nil
	}
	if alive <= 2 || int32(owned) < int32(s.NumStars)/2 {
		return nil
	}
	if err := g.mainCouncilScreen(); err != nil {
		return err
	}
	s.NextCouncilTurn = int16(uint16(s.Stardate) - 34975)
	return nil
}

// mainCouncilScreen is Main_Council_Screen_ (0x15239).
func (g *Game) mainCouncilScreen() error {
	s, c, e := &g.state, &g.council, g.e
	if !g.councilHeadless() {
		e.DeactivateHelpList()
		e.EnableDrawAllFields()
		e.V.ResetWindow()
		e.V.ClipOff()
		e.SetMouseList(g.mouseListAt(councilMouseList))
		g.playStreamingMusic(0x77, -1, true) // 0x1526d: the council's theme
		e.SetFadePercent(0)
		e.SetFadeSpeedPercent(10)
		if err := e.FastFadeOut(); err != nil {
			return err
		}
		e.SetInputDelay(1)
		e.ClearFields()
		e.V.SetRefreshStencil()
		g.gnn.savedAuto = e.GetAutoFunction()
		e.DeactivateAutoFunction()
		e.ClearFields()
	}
	if err := g.reloadMainCouncilScreen(); err != nil {
		return err
	}
	s.CouncilFlag = 1
	c.state = 0
	c.tally = [2]int16{}
	if !g.councilHeadless() {
		e.AssignAutoFunction(g.drawMainCouncilScreen, 2)
	}
	for c.state != councilStateDone {
		if err := g.councilMessageAndWait(); err != nil {
			return err
		}
		g.nextCouncilState()
		if c.state == 2 {
			if err := g.councilVotes(); err != nil {
				return err
			}
			if err := g.councilFinishedVoting(); err != nil {
				return err
			}
			if err := g.councilAcceptRuling(); err != nil {
				return err
			}
		}
	}
	s.SomethingHappened[s.PlayerNum] = 1
	if g.councilHeadless() {
		return nil
	}
	e.DeactivateAutoFunction()
	e.ClearFields()
	g.playBackgroundMusic() // 0x1535d
	e.SetFadePercent(0)
	e.SetFadeSpeedPercent(10)
	if err := e.FastFadeOut(); err != nil {
		return err
	}
	e.PutAutoFunction(g.gnn.savedAuto)
	// Fade_Into_Screen_ gets EDX = &_draw_main_screen_to_back, left over.
	g.ms.drawToBack = true
	return g.fadeIntoScreen(g.drawMainScreen, 0)
}

// councilMessageAndWait is the Get_Council_Message_,
// Control_Main_Council_Screen_ pair every step of the meeting makes.
func (g *Game) councilMessageAndWait() error {
	if err := g.getCouncilMessage(); err != nil {
		return err
	}
	return g.controlMainCouncilScreen()
}

// nextCouncilState is Next_Council_State_ (0x15db0), also inlined after
// the votes and the ruling.
func (g *Game) nextCouncilState() {
	c := &g.council
	switch c.state {
	case 0:
		c.state = 1
	case 1:
		c.state = 2
	case 4, 5, 6:
		c.state = councilStateDone
	}
}

// controlMainCouncilScreen is Control_Main_Council_Screen_ (0x153a0):
// the screen until a click.
func (g *Game) controlMainCouncilScreen() error {
	if g.councilHeadless() {
		return nil
	}
	e := g.e
	e.AssignAutoFunction(g.drawMainCouncilScreen, 2)
	g.council.showing = true
	g.council.waits++
	defer func() { g.council.showing = false }()
	for {
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
		if in == f {
			break
		}
		if err := g.drawMainCouncilScreen(); err != nil {
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

// drawMainCouncilScreen is Draw_Main_Council_Screen_ (0x15421).
func (g *Game) drawMainCouncilScreen() error {
	c, e := &g.council, g.e
	t := e.Text()
	e.V.SetPageOff()
	t.SetOutlineColor(0, 0)
	if g.settings[6] == 1 {
		if err := e.DrawMultiFileAnimation(0); err != nil {
			return err
		}
		if e.FileAnimationFrame() == 0 {
			e.SetFileAnimationFrame(1)
		}
	}
	t.SetOutlineColor(0, 0)
	t.SetStyleShadow(2, g.exeColors(councilColors), font.Outline)
	x := 0x1e
	for k := 0; k < 2; k++ {
		var buf string
		var err error
		switch {
		case c.tally[k] < 1:
			buf, err = g.decodeTextMessage(c.noVotesMsg, int(c.nominees[k]), -1, -1, -1, -1, -1, -1, 0x28)
		case c.tally[k] < 2:
			buf, err = g.decodeTextMessage(c.voteMsg, int(c.nominees[k]), -1, -1, int32(c.tally[k]), -1, -1, -1, 0x28)
		default:
			buf, err = g.decodeTextMessage(c.votesMsg, int(c.nominees[k]), -1, -1, int32(c.tally[k]), -1, -1, -1, 0x28)
		}
		if err != nil {
			return err
		}
		if err := t.Print(x, 0x1e, buf); err != nil {
			return err
		}
		x += 0x1a5
	}
	total := fmt.Sprintf(cStringAt(g.exe, councilTotalFmt), c.totalVotes, c.totalMsg)
	if err := t.PrintCentered(0x140, 0x1e, total); err != nil {
		return err
	}
	for k := 0; k < int(c.nMembers); k++ {
		pic := c.darkSeg[k]
		if (c.state == 2 || c.state == 3) && int(c.voter) == int(c.members[k]) {
			pic = c.lightSeg[k]
		}
		if err := e.Draw(councilSeatX[k], councilSeatY[k], pic); err != nil {
			return err
		}
	}
	x = 0x73
	for k := 0; k < 2; k++ {
		pic := c.nomineeSeg[k]
		f := pic.CurrentFrame
		if err := e.Draw(x, 0xe1, pic); err != nil {
			return err
		}
		// Only the nominee casting a vote moves.
		if int(c.voter) != int(c.nominees[k]) {
			setAnimationFrame(pic, f)
		}
		x += 0xcd
	}
	t.SetOutlineColor(0, 0)
	t.SetStyleShadow(4, g.exeColors(councilColors), font.Outline)
	h, err := t.ParagraphMaxHeight(0x226, c.message)
	if err != nil {
		return err
	}
	err = t.PrintParagraph(0x37, int(int16(0x1d6-h)), 0x226, c.message, 2)
	g.playStreamingMusic(0x77, -1, true) // 0x156c3, 0x16bed
	g.e.MoxUpdate()
	return err
}

// reloadMainCouncilScreen is Reload_Main_Council_Screen_ (0x156d4): the two
// most populous players stand, the others vote, sorted by their votes; the
// pictures, the hall and the texts.
func (g *Game) reloadMainCouncilScreen() error {
	s := &g.state
	c := &g.council
	e := g.e
	var cursor *gfx.Picture
	if !g.councilHeadless() {
		pic, err := g.picture(cStringAt(g.exe, councilLBXUpper), 0)
		if err != nil {
			return err
		}
		e.ClearMouseImage(0)
		cursor = gfx.NewBlankBitmap(0x1e, 0x1e)
		cursor.DrawPictureToBitmap(pic)
		e.V.SetPageOff()
		e.DrawPalette(pic)
		e.V.SetPaletteChanges(0, 0xff)
		e.V.Fill(0, 0, 0x27f, 0x1df, 0)
		e.DrawBitmapToMouse(0, 0, cursor, 0)
	}
	pick := func(skip int16) {
		var best int16
		for p := int16(0); p < s.NumPlayers; p++ {
			if s.Players[p].Eliminated != 0 || p == skip {
				continue
			}
			// A nominee only replaces the old value with votes above 0.
			if v := g.calcCouncilVote(uint8(p)); v > best {
				best = v
				if skip < 0 {
					c.nominees[0] = uint8(p)
				} else {
					c.nominees[1] = uint8(p)
				}
			}
		}
	}
	pick(-1)
	pick(int16(c.nominees[0]))
	c.nMembers = 0
	for p := int16(0); p < s.NumPlayers; p++ {
		if s.Players[p].Eliminated == 0 && p != int16(c.nominees[0]) && p != int16(c.nominees[1]) {
			c.members[c.nMembers] = uint8(p)
			c.nMembers++
		}
	}
	c.totalVotes = 0
	for k := 0; k < int(c.nMembers); k++ {
		v := g.calcCouncilVote(c.members[k])
		c.totalVotes += v
		c.memberVotes[k] = v
	}
	for k := 0; k < 2; k++ {
		c.nomineeVotes[k] = g.calcCouncilVote(c.nominees[k])
		c.totalVotes += c.nomineeVotes[k]
	}
	n := int(c.nMembers)
	for range n {
		for k := 0; k < n-1; k++ {
			if c.memberVotes[k] < c.memberVotes[k+1] {
				c.memberVotes[k], c.memberVotes[k+1] = c.memberVotes[k+1], c.memberVotes[k]
				c.members[k], c.members[k+1] = c.members[k+1], c.members[k]
			}
		}
	}
	if !g.councilHeadless() {
		lbxName := cStringAt(g.exe, councilLBX)
		race := func(p uint8) int { return int(s.Players[p].Race) }
		var err error
		for k := 0; k < n; k++ {
			r := race(c.members[k])
			if c.lightSeg[k], err = g.picture(lbxName, r*6+2+k); err != nil {
				return err
			}
			if c.darkSeg[k], err = g.picture(lbxName, r*6+0x50+k); err != nil {
				return err
			}
		}
		if c.nomineeSeg[0], err = g.picture(lbxName, race(c.nominees[0])*2+0x9e); err != nil {
			return err
		}
		if c.nomineeSeg[1], err = g.picture(lbxName, race(c.nominees[1])*2+0x9f); err != nil {
			return err
		}
		e.V.SetRefreshStencil()
		hall, err := g.picture(cStringAt(g.exe, councilLBXUpper), 1)
		if err != nil {
			return err
		}
		if err := e.OpenMultiFileAnimation(0, hall); err != nil {
			return err
		}
		e.V.SetPageOff()
		if err := e.DrawMultiFileAnimation(0); err != nil {
			return err
		}
		e.RefreshPalette()
		e.V.CopyOffToBack()
		e.SetMouseList(engine.MouseArrow)
	}
	if err := g.loadCouncilTextMessages(); err != nil {
		return err
	}
	c.voter = -1
	return nil
}

// loadCouncilTextMessages is Load_Council_Text_Messages_ (0x1695a).
func (g *Game) loadCouncilTextMessages() error {
	c := &g.council
	for _, m := range []struct {
		n   int
		dst *string
	}{{0xb, &c.noVotesMsg}, {0xc, &c.voteMsg}, {0xd, &c.votesMsg}, {0xe, &c.totalMsg}} {
		s, err := g.textMessage(cStringAt(g.exe, councilText), m.n, 0x1e)
		if err != nil {
			return err
		}
		*m.dst = s
	}
	return nil
}

// calcCouncilVote is Calc_Council_Vote_ (0x15b90): one vote per started
// ten population.
func (g *Game) calcCouncilVote(p uint8) int16 {
	pop := int32(g.state.Players[p].Population)
	v := pop / 10
	if pop%10 != 0 {
		v++
	}
	return int16(v)
}

// getCouncilMessage is Get_Council_Message_ (0x15be9): the text of the
// meeting's state.
func (g *Game) getCouncilMessage() error {
	c := &g.council
	file := cStringAt(g.exe, councilText)
	msg := func(n int) (string, error) { return g.textMessage(file, n, councilMaxMessage) }
	var err error
	var m string
	switch c.state {
	case 0:
		c.message, err = msg(0xf)
	case 1:
		if m, err = msg(0x10); err == nil {
			c.message, err = g.decodeTextMessage(m, int(c.nominees[0]), -1, -1, -1, int(c.nominees[1]), -1, -1, councilMaxMessage)
		}
	case 2:
		if m, err = msg(0x11); err == nil {
			c.message, err = g.decodeTextMessage(m, int(c.nominees[c.voteFor]), -1, -1, int32(c.voteCast), -1, -1, -1, councilMaxMessage)
		}
	case 3:
		if m, err = msg(0x12); err == nil {
			c.message, err = g.decodeTextMessage(m, int(c.voter), -1, -1, int32(c.voteCast), -1, -1, -1, councilMaxMessage)
		}
	case 4:
		if m, err = msg(0x13); err == nil {
			c.message, err = g.decodeTextMessage(m, int(c.nominees[c.voteFor]), -1, -1, -1, -1, -1, -1, councilMaxMessage)
		}
	case 5:
		c.message, err = msg(0x14)
	case 6:
		if c.message, err = msg(0x15); err == nil && g.state.CouncilFlag == 2 {
			if m, err = g.textMessage(file, 0x16, 0x50); err == nil {
				if m, err = g.decodeTextMessage(m, int(c.winning), -1, -1, -1, -1, -1, -1, 0x50); err == nil {
					c.message += m
				}
			}
		}
	}
	return err
}

// councilVotes is Council_Votes_ (0x15ebc): the members vote, then each AI
// nominee for itself, then the human.
func (g *Game) councilVotes() error {
	s, c := &g.state, &g.council
	for k := 0; k < int(c.nMembers); k++ {
		m := int16(c.members[k])
		if m == s.PlayerNum || m == int16(c.nominees[0]) || m == int16(c.nominees[1]) {
			continue
		}
		r0 := g.voteCheck(int16(c.nominees[0]), m, int16(c.nominees[1]))
		r1 := g.voteCheck(int16(c.nominees[1]), m, int16(c.nominees[0]))
		var v uint8
		switch {
		case r0 && r1:
		case r0:
			v = 1
		case r1:
			v = 2
		}
		c.voter = int8(c.members[k])
		c.voteCast = c.memberVotes[k]
		if err := g.councilVoteResult(v, c.members[k]); err != nil {
			return err
		}
	}
	for k := 0; k < 2; k++ {
		if int16(c.nominees[k]) == s.PlayerNum {
			continue
		}
		var v uint8
		if c.nomineeVotes[k] != 0 {
			v = uint8(k + 1)
		}
		c.voteCast = c.nomineeVotes[k]
		c.voter = int8(c.nominees[k])
		if err := g.councilVoteResult(v, c.nominees[k]); err != nil {
			return err
		}
	}
	return g.councilPlayerVote()
}

// voteCheck is Vote_Check_ (0x16021): whether voter s votes for nominee c
// over d. The sum is 16 bits wide.
func (g *Game) voteCheck(c, s, d int16) bool {
	st := &g.state
	S, C := &st.Players[s], &st.Players[c]
	var v int16
	switch {
	case S.TreatyStatus[c] >= 4:
		v = -1
	case S.TreatyStatus[c] == 2:
		v = 200
	default:
		v = int16(S.Relations[c]) + int16(S.Diplo6D7[c]) + 2*int16(S.Diplo7EE[c]) - int16(uint16(S.HumanBias[c])*20)
		if S.TreatyStatus[d] >= 4 {
			v += 100
		}
		if int16(g.orionOwner()) == c {
			v += 40
		}
		if C.Picks[20] == 1 {
			v += 40
		}
		if C.Picks[19] == 1 {
			v -= 100
		}
		if C.AppStatus[200] == 3 {
			v += 30
		}
		if int16(S.CouncilVote) == c {
			v += 50
		}
		if S.TreatyStatus[c] == 1 {
			v += 50
		}
		if S.TradeTreaty[c] != 0 {
			v += 25
		}
		if S.ResearchTreaty[c] != 0 {
			v += 25
		}
		if d := int32(g.settings[0xd4]); C.Objective == save.ObjectiveHuman && d > 2 {
			v = int16(int32(v) * 6 / (d + 6))
		}
	}
	return int32(st.Random.Random(200)) <= int32(v)
}

// councilVoteResult is Council_Vote_Result_ (0x161e4): vote 0 abstains, 1
// and 2 go to the first or second nominee; relations follow.
func (g *Game) councilVoteResult(vote, voter uint8) error {
	s, c := &g.state, &g.council
	n0, n1 := int(c.nominees[0]), int(c.nominees[1])
	switch vote {
	case 0:
		c.state = 3
		g.changeRelations(-6, int(voter), n1, 0, 0, 0)
		g.changeRelations(-6, int(voter), n0, 0, 0, 0)
		s.Players[voter].CouncilVote = -1
	case 1, 2:
		k := int(vote - 1)
		for_, against := n0, n1
		if k == 1 {
			for_, against = n1, n0
		}
		c.state = 2
		c.tally[k] += c.voteCast
		c.voteFor = uint8(k)
		g.changeRelations(0x18, int(voter), for_, 0x10, 0, 0)
		g.changeRelations(-12, int(voter), against, 9, 0, 0)
		s.Players[voter].CouncilVote = int8(for_)
	}
	return g.councilMessageAndWait()
}

// councilPlayerVote is Council_Player_Vote_ (0x1633c): the human picks a
// nominee or abstains from a list.
func (g *Game) councilPlayerVote() error {
	s, c := &g.state, &g.council
	file := cStringAt(g.exe, councilText)
	if !g.councilHeadless() {
		g.e.AssignAutoFunction(g.drawMainCouncilScreen, 2)
	}
	c.voteCast = g.calcCouncilVote(uint8(s.PlayerNum))
	c.voter = int8(s.PlayerNum)
	var rows [4]string
	title, err := g.textMessage(file, 0x17, 0x2d)
	if err != nil {
		return err
	}
	if title, err = g.decodeTextMessage(title, -1, -1, -1, int32(c.voteCast), -1, -1, -1, 0x2d); err != nil {
		return err
	}
	for k := 0; k < 2; k++ {
		n := c.nominees[k]
		if int16(n) == s.PlayerNum {
			if rows[k], err = g.textMessage(file, 0x18, 0x19); err != nil {
				return err
			}
		} else {
			rows[k] = fmt.Sprintf(cStringAt(g.exe, councilNameFmt), save.CString(s.Players[n].EmperorName[:]))
		}
	}
	if rows[2], err = g.textMessage(file, 0x19, 0x19); err != nil {
		return err
	}
	c.message = ""
	r, err := g.councilList(0, title, rows[:])
	if err != nil {
		return err
	}
	var vote uint8
	switch r {
	case 0, 1:
		c.winning = c.nominees[r]
		s.Players[c.voter].CouncilVote = int8(c.nominees[r])
		vote = uint8(r + 1)
	case 2:
		s.Players[c.voter].CouncilVote = -1
	}
	if err := g.councilVoteResult(vote, uint8(s.PlayerNum)); err != nil {
		return err
	}
	if !g.councilHeadless() {
		g.e.ClearFields()
		g.e.DeactivateAutoFunction()
	}
	c.voter = -1
	return nil
}

// councilList is the Get_List_Field_ of the vote and the ruling: rows of
// 0x19 bytes under title at (0xd6, 0x184). list 0 is the vote, 1 the ruling.
func (g *Game) councilList(list int, title string, rows []string) (int, error) {
	if g.councilHeadless() {
		if g.council.choose != nil {
			return g.council.choose(list), nil
		}
		if g.noCouncil {
			return g.councilNoScreenChoice(list), nil
		}
		return -1, nil
	}
	t := g.e.Text()
	t.SetOutlineColor(0, 0)
	t.SetStyle(4, g.exeColors(councilColors))
	t.SetHighlightColors(g.exeColors(councilHighlight))
	t.SetLF(4)
	var v int16
	active := []byte{1, 1, 1, 1}
	return g.e.GetListField(0xd6, 0x184, 0xb4, title, rows, &v, active, 0x11, [4]int{})
}

// councilFinishedVoting is Council_Finished_Voting_ (0x15df8): two thirds
// of the votes elect.
func (g *Game) councilFinishedVoting() error {
	s, c := &g.state, &g.council
	need := (int32(c.totalVotes)*2 + 2) / 3
	c.state = 5
	for k := 0; k < 2; k++ {
		if int32(c.tally[k]) >= need {
			c.voteFor = uint8(k)
			c.state = 4
			s.CouncilFlag = 3
			if int16(c.nominees[k]) == s.PlayerNum {
				s.CouncilFlag = 2
			}
			break
		}
	}
	if err := g.councilMessageAndWait(); err != nil {
		return err
	}
	g.nextCouncilState()
	return nil
}

// councilAcceptRuling is Council_Accept_Ruling_ (0x1660b): after an
// election the human accepts (the winner wins the game) or defies it (all
// but the human join the winner, or the human's rival, at war with him).
func (g *Game) councilAcceptRuling() error {
	s, c := &g.state, &g.council
	if !g.councilHeadless() {
		g.e.AssignAutoFunction(g.drawMainCouncilScreen, 2)
	}
	defer func() {
		if !g.councilHeadless() {
			g.e.DeactivateAutoFunction()
			g.e.ClearFields()
		}
	}()
	if s.CouncilFlag == 1 {
		return nil
	}
	c.message = ""
	file := cStringAt(g.exe, councilText)
	var rows [3]string
	var err error
	if rows[0], err = g.textMessage(file, 0x1a, 0x19); err != nil {
		return err
	}
	if rows[1], err = g.textMessage(file, 0x1b, 0x19); err != nil {
		return err
	}
	title, err := g.textMessage(file, 0x1c, 0x28)
	if err != nil {
		return err
	}
	r, err := g.councilList(1, title, rows[:])
	if err != nil {
		return err
	}
	switch r {
	case 0:
		c.state = councilStateDone
		s.CouncilWinner = int16(c.nominees[c.voteFor])
		if s.CouncilWinner == s.PlayerNum {
			if err := g.councilPlayerVotedWinner(); err != nil {
				return err
			}
			if !g.councilHeadless() {
				if err := g.playFadedFlics(0, eDiplomaticWin, eGeneralWin); err != nil { // Play_Diplomatic_Win_Flic_
					return err
				}
			}
		}
		s.Players[s.CouncilWinner].WonCouncil = 1
	case 1:
		c.state = 6
		switch {
		case s.CouncilFlag != 2:
			c.winning = c.nominees[c.voteFor]
		case int16(c.nominees[0]) == s.PlayerNum:
			c.winning = c.nominees[1]
		default:
			c.winning = c.nominees[0]
		}
		if err := g.councilMessageAndWait(); err != nil {
			return err
		}
		g.nextCouncilState()
		w := int16(c.winning)
		for b := int16(0); b < s.NumPlayers; b++ {
			if b == w || b == s.PlayerNum || s.Players[b].Eliminated != 0 {
				continue
			}
			if s.maySurrenderEmpire(b, w) {
				s.surrenderEmpire(b, w)
			}
		}
		g.doSurrenders()
		g.breakTrade(w, s.PlayerNum)
		g.breakResearch(w, s.PlayerNum)
		g.breakTribute(w, s.PlayerNum)
		g.breakTreaties(w, s.PlayerNum)
		g.declareWar(w, s.PlayerNum)
	}
	return nil
}

// councilPlayerVotedWinner is Council_Player_Voted_Winner_ (0x169c9): the
// losing nominee and then every member cheer the human, one screen each.
func (g *Game) councilPlayerVotedWinner() error {
	c := &g.council
	for i := range c.winnerMsgID {
		c.winnerMsgID[i] = -1
	}
	if !g.councilHeadless() {
		g.e.AssignAutoFunction(g.drawCouncilPlayerVotedWinner, 2)
	}
	other := 0
	if c.voteFor == 0 {
		other = 1
	}
	if err := g.driveCouncilPlayerVotedWinner(c.nominees[other]); err != nil {
		return err
	}
	for k := 0; k < int(c.nMembers); k++ {
		// A member already cheered is no longer drawn among the others.
		c.members[k] += 100
		if err := g.driveCouncilPlayerVotedWinner(c.members[k] - 100); err != nil {
			return err
		}
	}
	return nil
}

// driveCouncilPlayerVotedWinner is Drive_Council_Player_Voted_Winner_
// (0x16a3f): player p's cheer until a click.
func (g *Game) driveCouncilPlayerVotedWinner(p uint8) error {
	if err := g.getPlayerVotedWinnerMessage(); err != nil {
		return err
	}
	if g.councilHeadless() {
		return nil
	}
	e := g.e
	pic, err := g.picture(cStringAt(g.exe, councilLBXUpper), int(g.state.Players[p].Race)+0xb8)
	if err != nil {
		return err
	}
	if err := e.OpenMultiFileAnimation(1, pic); err != nil {
		return err
	}
	g.council.showing = true
	g.council.waits++
	defer func() { g.council.showing = false }()
	for {
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
		if in == f {
			return nil
		}
		if err := g.drawCouncilPlayerVotedWinner(); err != nil {
			return err
		}
		if err := e.TogglePages(); err != nil {
			return err
		}
		if err := e.ReleaseTime(2); err != nil {
			return err
		}
	}
}

// drawCouncilPlayerVotedWinner is Draw_Council_Player_Voted_Winner_
// (0x16ad2): without animations neither the hall nor the cheer is drawn.
func (g *Game) drawCouncilPlayerVotedWinner() error {
	c, e := &g.council, g.e
	t := e.Text()
	e.V.SetPageOff()
	t.SetOutlineColor(0, 0)
	if g.settings[6] == 1 {
		if err := e.DrawMultiFileAnimation(0); err != nil {
			return err
		}
		if e.FileAnimationFrame() == 0 {
			e.SetFileAnimationFrame(1)
		}
		if err := e.DrawMultiFileAnimation(1); err != nil {
			return err
		}
	}
	for k := 0; k < int(c.nMembers); k++ {
		if c.members[k] < 100 {
			if err := e.Draw(councilSeatX[k], councilSeatY[k], c.darkSeg[k]); err != nil {
				return err
			}
		}
	}
	t.SetOutlineColor(0, 0)
	t.SetStyleShadow(4, g.exeColors(councilColors), font.Outline)
	h, err := t.ParagraphMaxHeight(0x226, c.message)
	if err != nil {
		return err
	}
	err = t.PrintParagraph(0x37, int(int16(0x1d6-h)), 0x226, c.message, 2)
	g.playStreamingMusic(0x77, -1, true) // 0x156c3, 0x16bed
	g.e.MoxUpdate()
	return err
}

// getPlayerVotedWinnerMessage is Get_Player_Voted_Winner_Message_
// (0x16bfc): one of 14 cheers, each used once, from councmsg.lbx.
func (g *Game) getPlayerVotedWinnerMessage() error {
	c := &g.council
	var r int
	for {
		r = int(int16(g.state.Random.Random(0xe) - 1))
		if c.winnerMsgID[r] == -1 {
			break
		}
	}
	c.winnerMsgID[r] = int8(r)
	a, err := g.archive(cStringAt(g.exe, councmsgLBX))
	if err != nil {
		return err
	}
	tb, err := a.Table(r + int(g.settings[0xd2])*0xe)
	if err != nil {
		return err
	}
	rec := tb.Record(0)
	c.message = save.CString(rec[:min(len(rec), councilMaxMessage)])
	return nil
}

// orionOwner is Orion_Owner_ (0x78398): the owner of the first colonised
// planet of type 3 around star 0, else -1.
func (g *Game) orionOwner() int8 {
	s := &g.state
	for k := 0; k < 5; k++ {
		pl := s.Stars[0].Planets[k]
		if pl == -1 || s.Planets[pl].Type != 3 {
			continue
		}
		if c := s.Planets[pl].Colony; c != -1 {
			return s.Colonies[c].Owner
		}
	}
	return -1
}
