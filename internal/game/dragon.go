package game

import (
	"github.com/openorion2/openorion2/internal/font"
	"github.com/openorion2/openorion2/internal/gfx"
	"github.com/openorion2/openorion2/internal/save"
)

// The Dragon's bribe (MAINPUPS.C): Dragon_New_Star_Popup_ asks the human
// whose colony the Dragon (owner 12) attacks for 500 BC to send it to a
// star picked on a galaxy map. Spec: docs/screens/main-popups.md, "Dragon".

// dragonPopup is the popup's state.
type dragonPopup struct {
	cancelPic, acceptPic, bg *gfx.Picture // 0x1a1124, 0x1a11a0, 0x1a11a8
	cancel, accept           int          // 0x1a11c8, 0x1a11f6
	state                    int16        // 0x1831cc: 0 ask, 1 pick a star, 2 too poor
	hover, shown             int16        // 0x1831ce, 0x1831d0
	star                     int          // the attacked star
}

// dragonNewStarPopup stands for Dragon_New_Star_Popup_ (0xcc735): a human
// may bribe the Dragon away; -1 means no bribe, else the new star.
func (g *Game) dragonNewStarPopup(p, star int16) int16 {
	if g.combatUI != nil {
		return g.combatUI.dragonBribe(p, star)
	}
	if g.e == nil {
		g.Log("Dragon_New_Star_Popup_ without a screen (step harness): no bribe")
		return -1
	}
	r, err := g.dragonPopupRun(int(p), int(star))
	if err != nil {
		panic(err)
	}
	return r
}

func (g *Game) dragonPopupRun(p, star int) (int16, error) {
	e, mp, d := g.e, &g.mp, &g.drag
	d.state, d.star = 0, star
	mp.playerN = p
	d.hover = -1 // 0x1a1240 and the hover
	e.Text().SetStyleShadow(1, g.exeColors(dragonColors), font.ShadowDown)
	var err error
	// Load_Dragon_New_Star_Popup_ (0xcc359), then Load_Galaxy_Map_Anims_.
	if d.cancelPic, err = g.picture(mainpupsLBX, 0x46); err != nil {
		return -1, err
	}
	if d.acceptPic, err = g.picture(mainpupsLBX, 0x47); err != nil {
		return -1, err
	}
	if d.bg, err = g.picture(mainpupsLBX, 0x48); err != nil {
		return -1, err
	}
	if err := g.doPopupSetup(star, d.bg); err != nil {
		return -1, err
	}
	e.DeactivateAutoFunction()
	e.AssignAutoFunction(g.drawDragonNewStarPopup, 1)
	e.DeactivateHelpList() // Set_Dragon_New_Star_Popup_Help_List_ (0xcdc03)
	e.SetHelpList(g.helpListAt(dragonHelpList, 1))
	g.ericPushFields()
	g.clearGalaxyMapFields()
	if err := g.addDragonNewStarFields(); err != nil {
		return -1, err
	}
	if err := g.drawDragonNewStarPopup(); err != nil {
		return -1, err
	}
	if err := e.TogglePages(); err != nil {
		return -1, err
	}
	e.V.SetRefreshDifferential()
	mp.showing = "dragon"
	defer func() { mp.showing = "" }()
	for {
		in, err := e.GetInput()
		if err != nil {
			return -1, err
		}
		scan, err := e.ScanInput()
		if err != nil {
			return -1, err
		}
		r, done, err := g.evaluateDragonNewStarInput(in, scan, star)
		if err != nil {
			return -1, err
		}
		if err := g.drawDragonNewStarPopup(); err != nil {
			return -1, err
		}
		if err := e.TogglePages(); err != nil {
			return -1, err
		}
		if done {
			g.doPopupCleanup()
			e.DeactivateHelpList()
			mp.playerN = -1
			return int16(r), nil
		}
	}
}

const (
	dragonColors   = 0xc6e76 // the popup's text shades
	dragonHelpList = 0x18351a
)

// addDragonNewStarFields is Add_Dragon_New_Star_Fields_ (0xcc3db): CANCEL,
// ACCEPT while asking, and the map's star fields.
func (g *Game) addDragonNewStarFields() error {
	e, d := g.e, &g.drag
	es, err := g.eStrings()
	if err != nil {
		return err
	}
	if d.cancel, err = e.AddButtonField(0xba, 0x139, es[0xc], d.cancelPic, es[0x87], 0x28); err != nil {
		return err
	}
	d.accept = -1000
	if d.state == 0 {
		if d.accept, err = e.AddButtonField(0x17d, 0x139, es[0xc], d.acceptPic, es[0x231], 0x28); err != nil {
			return err
		}
	}
	return g.addGalaxyMapFields2(0x12e, 0x9c, 0x94, 0x5e, 0, 0, false)
}

// drawDragonNewStarPopup is Draw_Dragon_New_Star_Popup_ (0xcc491): on a full
// draw the backdrop, the map and the offer; always the hovered star's name.
func (g *Game) drawDragonNewStarPopup() error {
	e, s, mp, d := g.e, &g.state, &g.mp, &g.drag
	if d.shown != d.hover {
		mp.fullDraw = true
		d.shown = d.hover
	}
	if mp.fullDraw || g.fullDrawScreen {
		g.ms.drawToBack = true
		if err := g.mainpupsTestedExecuteAutofunction(); err != nil {
			return err
		}
		if err := e.Draw(0, 0, d.bg); err != nil {
			return err
		}
		mp.fullDraw = true
		// The map box takes C_Anims_ slot 0xb as its picture: loaded only
		// by the colony screen, else none (then the box is filled black).
		if err := g.drawGalaxyMapBox(g.cscr.buttons[0xb], 0x12e, 0x9c, 0x94, 0x5e, 0, 0, 0, 0, 0, 0, int16(dragonEntryESI)); err != nil {
			return err
		}
		es, err := g.eStrings()
		if err != nil {
			return err
		}
		P := &s.Players[mp.playerN]
		name, starName := save.CString(P.EmperorName[:]), save.CString(s.Stars[d.star].Name[:])
		var msg string
		switch d.state {
		case 1:
			msg = cSprintf(es[0x1d0], name)
		case 2:
			msg = cSprintf(es[0x247], 500, name, starName)
		default:
			msg = cSprintf(es[0x22b], 500, name, starName)
		}
		if err := e.Text().SqueezePrintFormattedParagraph(0xbc, 0x10f+0x23/2, 0x106, 0x23, msg, 2,
			g.exe.bytes(spaceLimits, 6), true); err != nil {
			return err
		}
		e.V.CopyOffToBack()
		mp.fullDraw = false
	}
	if d.hover != -1 {
		return e.Text().PrintCentered(0x178, 0xfe, save.CString(s.Stars[d.hover].Name[:]))
	}
	return nil
}

// dragonEntryESI is ESI when the popup draws: Dragon_New_Star_Popup_ copies
// its colours from 0xc6e76 with two movsd, leaving 0xc6e7e.
const dragonEntryESI = 0x6e7e

// evaluateDragonNewStarInput is Evaluate_Dragon_New_Star_Input_ (0xcc64b):
// CANCEL ends with -1; ACCEPT asks for a star (or says the treasury is too
// small); then a click on another star ends with it, on this one Help_.
func (g *Game) evaluateDragonNewStarInput(in, scan, star int) (int, bool, error) {
	s, mp, d := &g.state, &g.mp, &g.drag
	d.hover = -1
	switch {
	case in == d.cancel:
		return -1, true, nil
	case in == d.accept:
		d.state = 2
		if s.Players[mp.playerN].BC >= 500 {
			d.state = 1
		}
		mp.fullDraw = true
		return 0, false, nil
	case d.state == 1:
		res, hov, clk := g.scanGalaxyMapFields(scan, in)
		switch res {
		case 4:
			d.hover = int16(hov)
		case 0:
			if clk != star {
				return clk, true, nil
			}
			es, err := g.eStrings()
			if err != nil {
				return 0, false, err
			}
			if err := g.messageBox(es[0x240]); err != nil { // Help_
				return 0, false, err
			}
			mp.fullDraw = true
		}
	}
	if g.fullDrawScreen {
		mp.fullDraw = true
	}
	return 0, false, nil
}
