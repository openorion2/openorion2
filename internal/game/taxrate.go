package game

import (
	"github.com/openorion2/openorion2/internal/font"
	"github.com/openorion2/openorion2/internal/gfx"
)

// Tax_Rate_Popup_ (0xcc20b): the treasury panel's tax rate, 0..50% in
// steps of 10. Spec: docs/screens/main-popups.md, "Tax rate".

const (
	taxRateColors = 0xc6e6e // Tax_Rate_Popup_'s font colours (in .text)
	taxRateHelp   = 0x183510
)

// taxPopup holds the popup's pictures and fields.
type taxPopup struct {
	rates  [6]*gfx.Picture // MAINPUPS 0x3e..0x43 (0x1a1180..)
	ok, bg *gfx.Picture    // 0x44, 0x45 (0x1a11a0, 0x1a11a4)
	okF    int             // 0x1a11f6
	rateF  [6]int          // 0x1a11ea..
	rate   int16           // 0x1a122e
}

// loadTaxRatePopup is Load_Tax_Rate_Popup_ (0xcbebb).
func (g *Game) loadTaxRatePopup() error {
	tp := &g.tax
	var err error
	for i := range tp.rates {
		if tp.rates[i], err = g.picture(mainpupsLBX, 0x3e+i); err != nil {
			return err
		}
	}
	if tp.ok, err = g.picture(mainpupsLBX, 0x44); err != nil {
		return err
	}
	tp.bg, err = g.picture(mainpupsLBX, 0x45)
	return err
}

// addTaxRateFields is Add_Tax_Rate_Fields_ (0xcbfb5): OK and the six rates.
func (g *Game) addTaxRateFields() error {
	e, tp := g.e, &g.tax
	es, err := g.eStrings()
	if err != nil {
		return err
	}
	if tp.okF, err = e.AddButtonField(0xed, 0xfe, es[0xc], tp.ok, es[0x87], 0x28); err != nil {
		return err
	}
	xs := [6]int{0x93, 0xc1, 0xea, 0x113, 0x13c, 0x165}
	keys := [6]int{0x8e, 0x93, 0x95, 0x97, 0x99, 0x9b}
	for i := range xs {
		if tp.rateF[i], err = e.AddMultiButtonField(xs[i], 0xc6, es[0xc], tp.rates[i], &tp.rate, int16(10*i), es[keys[i]], 0x29); err != nil {
			return err
		}
	}
	return nil
}

// drawTaxRatePopup is Draw_Tax_Rate_Popup_ (0xcc152), the auto function.
func (g *Game) drawTaxRatePopup() error {
	if !g.mp.fullDraw && !g.fullDrawScreen {
		return nil
	}
	g.ms.drawToBack = true
	if err := g.mainpupsTestedExecuteAutofunction(); err != nil {
		return err
	}
	g.mp.fullDraw = true
	if err := g.e.Draw(0, 0, g.tax.bg); err != nil {
		return err
	}
	g.e.V.CopyOffToBack()
	g.mp.fullDraw = false
	return nil
}

// evaluateTaxRateInput is Evaluate_Tax_Rate_Input_ (0xcc198): OK takes the
// rate and leaves; any other field takes it and recomputes the colonies.
func (g *Game) evaluateTaxRateInput(in int, done *bool) {
	s, tp := &g.state, &g.tax
	p := int(s.PlayerNum)
	switch {
	case in == tp.okF:
		*done = true
		s.Players[p].TaxRate = uint8(tp.rate)
	case in != 0:
		s.Players[p].TaxRate = uint8(tp.rate)
		g.doPlayerColonyCalculations(p)
		g.mp.fullDraw = true
	}
	if g.fullDrawScreen {
		g.mp.fullDraw = true
	}
}

// taxRatePopup is Tax_Rate_Popup_ (0xcc20b).
func (g *Game) taxRatePopup() error {
	e, s, tp := g.e, &g.state, &g.tax
	var c font.Colors
	copy(c[:], g.exe.bytes(taxRateColors, 8))
	tp.rate = int16(s.Players[s.PlayerNum].TaxRate)
	g.mp.lastScanned = -1 // 0x1a1240
	e.Text().SetStyleShadow(1, c, font.ShadowDown)
	if err := g.loadTaxRatePopup(); err != nil {
		return err
	}
	if err := g.doPopupSetup(-1, tp.bg); err != nil {
		return err
	}
	e.DeactivateHelpList()
	e.SetHelpList(g.helpListAt(taxRateHelp, 1))
	e.DeactivateAutoFunction()
	e.AssignAutoFunction(g.drawTaxRatePopup, 1)
	g.ericPushFields()
	if err := g.addTaxRateFields(); err != nil {
		return err
	}
	redraw := func() error {
		if g.mp.fullDraw || g.fullDrawScreen {
			g.ms.drawToBack = true
			if err := g.mainpupsTestedExecuteAutofunction(); err != nil {
				return err
			}
			g.mp.fullDraw = true
			if err := e.Draw(0, 0, tp.bg); err != nil {
				return err
			}
			e.V.CopyOffToBack()
			g.mp.fullDraw = false
		}
		return e.TogglePages()
	}
	if err := redraw(); err != nil {
		return err
	}
	e.V.SetRefreshDifferential()
	for done := false; !done; {
		in, err := e.GetInput()
		if err != nil {
			return err
		}
		if _, err := e.ScanInput(); err != nil {
			return err
		}
		g.evaluateTaxRateInput(in, &done)
		if err := redraw(); err != nil {
			return err
		}
	}
	g.doPopupCleanup()
	return nil
}
