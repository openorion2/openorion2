package engine

import "time"

// vsync stands in for Wait_For_Vsync_: one display frame.
func (e *Engine) vsync() error { return e.Sys.Sleep(time.Second / 60) }

// darken is Darken_Palette_ with its two retrace waits; the new DAC is shown
// at once, as writing the DAC changes the picture without a flip.
func (e *Engine) darken(percent int) error {
	if err := e.vsync(); err != nil {
		return err
	}
	if err := e.vsync(); err != nil {
		return err
	}
	e.V.DarkenPalette(percent)
	return e.present(false)
}

// FastFadeOut is Fast_Fade_Out_: fade to black and clear both pages, unless
// already faded.
func (e *Engine) FastFadeOut() error {
	if e.fadePercent != 0 {
		return nil
	}
	e.V.SetRefreshFull()
	for p := 0; p <= 100; p += e.fadeStep {
		if err := e.darken(p); err != nil {
			return err
		}
		e.fadePercent = p
		e.MoxUpdate()
	}
	e.V.SetPageBack()
	e.V.Fill(0, 0, W-1, H-1, 0)
	e.V.CopyBackToOff()
	e.V.ClearPaletteChanges(0, 255)
	if err := e.TogglePages(); err != nil {
		return err
	}
	e.V.SetRefreshStencil()
	return nil
}

// FastFadeIn is Fast_Fade_In_: bring the DAC back up in steps of 10, without
// a flip, if faded.
func (e *Engine) FastFadeIn() error {
	if e.fadePercent == 0 {
		return nil
	}
	e.V.SetRefreshFull()
	for p := e.fadePercent; p >= 0; p -= 10 {
		if err := e.darken(p); err != nil {
			return err
		}
		e.fadePercent = p
		e.MoxUpdate()
	}
	e.V.SetRefreshStencil()
	return nil
}

// FadeIntoScreen is Fade_Into_Screen_: fade out, load palette pal, let draw
// paint the new screen into off, fade the DAC up and flip.
func (e *Engine) FadeIntoScreen(draw func() error, flag *bool, pal int) error {
	e.DisableDrawAllFields()
	if flag != nil {
		*flag = true
	}
	e.V.ClearPaletteChanges(0, 255)
	e.SetMouseList(MouseNone)
	if err := e.FastFadeOut(); err != nil {
		return err
	}
	if err := e.LoadPalette(pal, 0, 255); err != nil {
		return err
	}
	e.V.SetPaletteChanges(0, 255)
	e.V.refreshPalette()
	if err := e.vsync(); err != nil {
		return err
	}
	if err := e.darken(100); err != nil {
		return err
	}
	e.SetMouseList(MouseArrow)
	e.V.SetPageOff()
	if err := draw(); err != nil {
		return err
	}
	e.V.ClearPaletteChanges(0, 255)
	if err := e.FastFadeIn(); err != nil {
		return err
	}
	if err := e.TogglePages(); err != nil {
		return err
	}
	e.EnableDrawAllFields()
	return nil
}

// SetFadePercent is Set_Fade_Percent_: record the fade level without
// touching the DAC; at 0 the arrow cursor comes back.
func (e *Engine) SetFadePercent(p int) {
	e.fadePercent = p
	if p == 0 {
		e.SetMouseList(MouseArrow)
	}
}

// DarkenPalette is Darken_Palette_: wait two retraces and show the palette
// darkened by percent.
func (e *Engine) DarkenPalette(percent int) error { return e.darken(percent) }
