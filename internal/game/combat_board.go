package game

import (
	"strconv"

	"github.com/openorion2/openorion2/internal/save"

	"github.com/openorion2/openorion2/internal/gfx"
)

// The human's boarding (COMBAT1.C Initiate_Boarding_Action_, CMBTFIR2.C
// Init_Boarding_Action_, Board_Popup_). Spec: docs/screens/combat-screen.md.

// boardPopup is Board_Popup_'s state (0x19b554..0x19b570).
type boardPopup struct {
	msg                     string
	min, cur, max           int16
	minus, plus, raid, capt *gfx.Picture
}

// initiateBoardingAction is Initiate_Boarding_Action_ (0x459e7): the human's
// ship a boards t, after the checks that may refuse it with a warning.
func (g *Game) initiateBoardingAction(a, t int16) {
	tc := g.tc
	rng := g.rangeToShip(a, t)
	ok, refreshed := true, false
	reach := int16(1)
	if g.combatShipHasSpecial(a, 0x23) {
		reach = 4
	}
	A, T := g.tsp(a), g.tsp(t)
	warn := func(n int) {
		ok = false
		msg, err := g.kenText(n)
		g.must2(err)
		g.e.RefreshPalette()
		g.must2(g.warningBox(msg))
	}
	switch {
	case T.u8(tsOwner) > 8:
		warn(0x3e)
	case t == 0:
		warn(0x3f)
	case T.u8(tsOwner) == A.u8(tsOwner):
		if T.u8(tsBoarded) == 0 {
			warn(0x40)
		} else {
			warn(0x41)
		}
	case rng > reach:
		warn(0x42)
	case reach != 4 && g.calcCurrentSpeed(t) > 0:
		warn(0x43)
	case A.u8(tsMarines) == 0:
		warn(0x44)
	default:
		if reach != 4 || (rng <= 1 && T.i16(tsSpeed) <= 0) {
			break
		}
		if T.i16(tsArc+2*int(facingShield(g.relativeBearing(t, a)))) > 0 {
			warn(0x45)
			g.must2(g.refreshCombatScreenFull())
			refreshed = true
		}
		if g.combatShipHasSpecial(t, 0xd) {
			warn(0x46)
			refreshed = false
		}
	}
	if !ok {
		if !refreshed {
			g.must2(g.refreshCombatScreenFull())
		}
		return
	}
	var n int16
	kind := g.initBoardingAction(tc.cur, &n)
	if kind == 0 {
		return
	}
	C := g.tsp(tc.cur)
	C.set8(tsMarines, C.u8(tsMarines)-uint8(n))
	if reach == 4 && (rng != 1 || g.tsp(t).i16(tsSpeed) > 0) {
		if g.combatShipHasSpecial(t, 7) {
			n /= 3
		}
		if n <= 0 {
			kind = 0
		}
	}
	if tc.netFlag != 0 && (kind == 1 || kind == 2) {
		if to := g.opponentOf(int16(A.u8(tsOwner))); to != g.state.PlayerNum {
			g.sendTacticalCombatMessage(to, tacBoard, t, n, kind)
		}
	}
	cloak := func() {
		switch C.u8(tsCloak) {
		case 1:
			if tc.onScreen == 1 && tc.superFast == 0 {
				g.drawCloak(tc.cur, 2)
			}
			C.set8(tsCloak, 2)
		case 4:
			if tc.onScreen == 1 && tc.superFast == 0 {
				g.drawCloak(tc.cur, 5)
			}
			C.set8(tsCloak, 6)
		}
	}
	switch kind {
	case 1:
		cloak()
		C[0x48] |= 2
		g.resolveRaid(tc.cur, t, &n)
		C.set8(tsMarines, C.u8(tsMarines)+uint8(n))
	case 2:
		cloak()
		C[0x48] |= 2
		T := g.tsp(t)
		g.resolveCapture(tc.cur, t, n, int16(T.u8(tsX))*0x14, int16(T.u8(tsY))*0x14)
	}
	tc.board = 0
	// Also after kind became 0 above, with nothing sent: the other machine
	// never meets this barrier (a deadlock of the original).
	g.tacWaitIf()
}

// initBoardingAction is Init_Boarding_Action_ (0x372bd): Board_Popup_ with
// K text 0x29 and up to half the ship's marines (rounded up).
func (g *Game) initBoardingAction(i int16, n *int16) int16 {
	msg, err := g.kenText(0x29)
	g.must2(err)
	return g.boardPopup(0, int16((int32(g.tsp(i).u8(tsMarines))+1)/2), msg, n)
}

// boardPopup is Board_Popup_ (0x34fc3): the marines slider between min and
// max, RAID (1), CAPTURE (2) or Esc (0); the count chosen goes to *n.
func (g *Game) boardPopup(min, max int16, msg string, n *int16) int16 {
	e := g.e
	tc := g.tc
	b := &boardPopup{msg: msg, min: min, cur: max, max: max}
	tc.scr.board = b
	lbx := g.cmbtLBX(cmbtBoardLBX)
	var err error
	load := func(k int) *gfx.Picture {
		if err != nil {
			return nil
		}
		var p *gfx.Picture
		p, err = g.picture(lbx, k)
		return p
	}
	b.minus, b.plus, b.raid, b.capt = load(6), load(0x28), load(0x27), load(0x29)
	g.must2(err)
	e.V.SetPageBack()
	g.must2(g.fullDrawCombatScreen())
	g.must2(g.drawMainCombatScreen())
	g.must2(e.DrawVisibleFieldsBack())
	e.V.CopyBackToOff()
	g.must2(e.TogglePages())
	e.DeactivateAutoFunction()
	e.AssignAutoFunction(g.drawBoardPopup, 1)
	e.ClearFields()
	_, err = e.AddScrollField(0xbb, 0xf9, int(min), int(max)+1, int(min), int(max), 0xcf, 0xd, &b.cur, "", 0)
	g.must2(err)
	minus, err := e.AddButtonField(0xa4, 0xfd, "", b.minus, "", 0)
	g.must2(err)
	plus, err := e.AddButtonField(0x196, 0xfd, "", b.plus, "", 0)
	g.must2(err)
	raid, err := e.AddButtonField(0xa3, 0x111, "", b.raid, "", 0)
	g.must2(err)
	capt, err := e.AddButtonField(0x159, 0x111, "", b.capt, "", 0)
	g.must2(err)
	esc, err := e.AddHotKey(cStringAt(g.exe, cmbtBoardEsc))
	g.must2(err)
	e.V.SetPageOff()
	g.must2(g.drawBoardPopup())
	g.must2(e.TogglePages())
	g.bqSetMouseList(0x17cf00, 1)
	result := int16(1)
	for done := false; !done; {
		in, err := e.GetInput()
		g.must2(err)
		_, err = e.ScanInput()
		g.must2(err)
		if in == capt {
			done, result = true, 2
		}
		if in == raid {
			done, result = true, 1
		}
		if in == plus && max > b.cur {
			b.cur++
		}
		if in == minus && b.cur > 0 {
			b.cur--
		}
		if in == esc {
			done, result = true, 0
		}
		if !done {
			e.V.SetPageOff()
			g.must2(g.drawBoardPopup())
			g.must2(e.TogglePages())
		}
	}
	e.ClearFields()
	e.AssignAutoFunction(g.drawMainCombatScreen, 1)
	g.must2(g.refreshCombatScreenFull())
	g.assignCombatGrids(tc.cur)
	*n = b.cur
	tc.scr.board = nil
	return result
}

// drawBoardPopup is Draw_Board_Popup_ (0x34ec8): the box, the text, min,
// count and max, and the bar.
func (g *Game) drawBoardPopup() error {
	e := g.e
	b := g.tc.scr.board
	t := e.Text()
	t.SetStyle(2, g.combatGreen())
	bg, err := g.picture(g.cmbtLBX(cmbtBoardLBX), 0x24)
	if err != nil {
		return err
	}
	if err := e.Draw(0x96, 0x64, bg); err != nil {
		return err
	}
	if err := t.PrintParagraph(0xa5, 0x99, 0x105, b.msg, 2); err != nil {
		return err
	}
	for _, p := range []struct {
		x int
		v int16
	}{{0xb6, b.min}, {0x128, b.cur}, {0x198, b.max}} {
		if err := t.PrintCentered(p.x, 0xe7, strconv.Itoa(int(p.v))); err != nil {
			return err
		}
	}
	w := int(int32(b.cur-b.min) * 0xd8 / int32(b.max-b.min))
	e.V.Fill(0xba, 0xfd, int(int16(w+0xb9)), 0x107, 0xc1)
	x := int(int16(w + 0xba))
	e.V.Line(x, 0xfd, x, 0x107, 0)
	return nil
}

// Strings of Resolve_Raid_ and Resolve_Capture_.
const (
	raidStrEOL2    = 0x178ce3 // "\n\n"
	raidStrSpace   = 0x178ce6 // " "
	raidStrEOL     = 0x178ce8 // "\n"
	captStrFirst   = 0x178cea // the body's first variable code
	captStrA       = 0x178ced
	captStrB       = 0x178cf7
	captStrC       = 0x178d26
	captStrD       = 0x178d3b
	combatSoundBox = 0x43
)

// combatHuman reports whether record owner p is a human player.
func (g *Game) combatHuman(p uint8) bool {
	return p < 8 && g.state.Players[p].Objective == save.ObjectiveHuman
}

// combatReportBox is the boxes of Resolve_Raid_/Resolve_Capture_: timed
// (0x36 ticks) unless a human watches.
func (g *Game) combatReportBox(title, body string, timed bool) {
	if timed {
		g.must2(g.doTextBox(&title, body, 0x36))
	} else {
		g.must2(g.textBox(&title, body))
	}
}

// raidReport is the report Resolve_Raid_ (0x37308) shows. The body starts
// over with K 0x5d: the line with the raiders' strength is overwritten.
func (g *Game) raidReport(a, t int16, hits *[0x13]int16, total, raidersLost, crewLost, fs, ds int16) {
	A, T := g.tsp(a), g.tsp(t)
	k := func(n int) string {
		s, err := g.kenText(n)
		g.must2(err)
		return s
	}
	str := func(addr uint32) string { return cStringAt(g.exe, addr) }
	title := k(0x2a) + save.CString(T[:])
	_ = k(0x5c) + strconv.Itoa(int(fs))
	body := k(0x5d) + strconv.Itoa(int(ds)) + str(raidStrEOL2)
	if total < 1 {
		body += k(0x2f)
	} else {
		state := func(cur int16) string {
			if cur < 1 {
				return k(0x2e)
			}
			return k(0x2d)
		}
		if hits[0] > 0 {
			body += k(0x2b)
		}
		if hits[1] > 0 {
			body += k(0x2c) + state(T.i16(tsShieldCur))
		}
		if hits[2] > 0 {
			body += k(0x15) + str(raidStrSpace) + state(T.i16(tsCompCur))
		}
		for w := 0; w < 8; w++ {
			if h := hits[w+3]; h > 0 {
				name, err := g.weaponName(int(T.i16(tw(w, twWeapon))), h >= 2)
				g.must2(err)
				body += strconv.Itoa(int(h)) + str(raidStrSpace) + name + str(raidStrSpace) + k(0x2e)
			}
		}
		for j := 0; j < 8; j++ {
			if h := hits[j+0xb]; h > 0 {
				name, err := g.techName(walksSpecials + 1 + int(g.combatShipSpecialID(t, int16(j))))
				g.must2(err)
				body += strconv.Itoa(int(h)) + str(raidStrSpace) + name + str(raidStrSpace) + k(0x2e)
			}
		}
	}
	plural := func(n int16, one, many int) string {
		if n < 2 {
			return k(one)
		}
		return k(many)
	}
	if raidersLost > 0 {
		body += str(raidStrEOL) + strconv.Itoa(int(raidersLost)) + str(raidStrSpace) + plural(raidersLost, 0x5e, 0x5f)
	}
	if crewLost > 0 {
		body += str(raidStrEOL) + strconv.Itoa(int(crewLost)) + str(raidStrSpace) + plural(crewLost, 0x60, 0x61)
	}
	tc := g.tc
	side := tc.autoAtt
	if int16(A.u8(tsOwner)) == tc.att {
		side = tc.autoDef
	}
	modal := (side == 0 && g.combatHuman(T.u8(tsOwner))) || (tc.autoCombat == 0 && g.combatHuman(A.u8(tsOwner)))
	snd := g.moxPlaySound(0x43) // 0x37c73, faded after the box
	g.combatReportBox(title, body, !modal)
	g.fadeoutSound(snd, 0xa)
}

// captureReport is what Resolve_Capture_ (0x37da8) tells: both sides'
// marines before, lost and after, their strengths and the outcome.
type boardCapture struct {
	a, t                     int16
	n0, def0, n, def, fs, ds int16
	result                   int
}

func (g *Game) captureReport(r *boardCapture) {
	tc := g.tc
	A, T := g.tsp(r.a), g.tsp(r.t)
	k := func(n int) string {
		s, err := g.kenText(n)
		g.must2(err)
		return s
	}
	str := func(addr uint32) string { return cStringAt(g.exe, addr) }
	title := k(0x30) + save.CString(T[:])
	vals := [8]int16{r.n0, r.n0 - r.n, r.n, r.def0, r.def0 - r.def, r.def, r.fs, r.ds}
	p := g.e.Paragraphs()
	for i := range vals {
		p.SetVarSint(i, &vals[i])
	}
	k64 := k(0x64)
	body := str(captStrFirst) + k(0x5c) + str(captStrA) + k(0x5d) + str(captStrB) + k64 + str(captStrC) + k64 + str(captStrD) + k(r.result)
	side := tc.autoDef
	if int16(A.u8(tsOwner)) == tc.att {
		side = tc.autoAtt
	}
	timed := (side != 0 || !g.combatHuman(A.u8(tsOwner))) && (tc.autoCombat != 0 || !g.combatHuman(T.u8(tsOwner)))
	snd := g.moxPlaySound(0x43) // 0x382c9, faded after the box
	g.combatReportBox(title, body, timed)
	g.fadeoutSound(snd, 0xa)
	g.must2(g.refreshCombatScreenFull())
}

// combatShipSpecialID is Get_Combat_Ship_Special_Id_ (0x37254): the k-th
// special the record has, −1 if none.
func (g *Game) combatShipSpecialID(i, k int16) int16 {
	T := g.tsp(i)
	n := int16(0)
	for b := 0; b < 0x27; b++ {
		if testBit(T[tsSpecials:tsSpecials+5], b) {
			if n == k {
				return int16(b)
			}
			n++
		}
	}
	return -1
}
