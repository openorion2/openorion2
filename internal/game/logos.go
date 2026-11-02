package game

import (
	"github.com/openorion2/openorion2/internal/engine"
)

// The start-up logos and intro (MOX.C): Draw_Logos_ (0x24ed3) from main2_
// (0x10164). Spec: docs/screens/cinematics.md.

const (
	logoLBX   = 0x178c17 // "logo.LBX": the two logos, file animations
	simtexLBX = 0x178c20 // "simtex.lbx", a Smacker file played if present
	introLBX  = 0x178c2b // "intro.lbx"
)

// drawLogos is Draw_Logos_ (0x24ed3): LOGO.LBX 0 fades in slowly and holds
// 15 ticks on its last frame, fades out; LOGO.LBX 1 fades in fast, holds 5
// ticks, fades out; then the intro video with fades. A key or a button ends
// a logo and skips the rest, the intro included.
func (g *Game) drawLogos() error {
	e := g.e
	v := e.V
	e.DeactivateAutoFunction()
	e.SetMouseList(engine.MouseNone) // 0x180ebc
	g.logoFade = 0
	e.ClearFields()
	v.SetRefreshStencil()
	v.SetPageOff()
	v.Fill(0, 0, 0x27f, 0x1df, 0)
	skip := false
	// hold runs a logo's loop: one frame a tick, n ticks on the last.
	hold := func(n int, draw func() error) error {
		total := e.FileAnimation().Frames - 1
		for done := false; !done && !skip; {
			e.MarkTime()
			if e.Sys.KeyWaiting() {
				e.Sys.PopKey()
				skip = true
			}
			if e.MouseButton() != 0 {
				skip, done = true, true
				continue
			}
			v.SetPageOff()
			if e.FileAnimationFrame() == total {
				if n--; n < 1 {
					done = true
				}
			} else if err := draw(); err != nil {
				return err
			}
			if err := e.TogglePages(); err != nil {
				return err
			}
			e.MoxUpdate()
			if err := e.ReleaseTime(1); err != nil {
				return err
			}
		}
		return nil
	}
	open := func(entry int) error {
		p, err := g.picture(upperLBX(cStringAt(g.exe, logoLBX)), entry)
		if err != nil {
			return err
		}
		e.OpenFileAnimation(p)
		return nil
	}
	if g.dirMatch(cStringAt(g.exe, simtexLBX)) { // Check_For_Simtex_Logo_
		if err := g.playCinematic(upperLBX(cStringAt(g.exe, simtexLBX)), 0, false); err != nil {
			return err
		}
		e.StopStream()
		g.mus.current = -1
	} else {
		if err := open(0); err != nil {
			return err
		}
		v.SetPageOff()
		if err := e.DarkenPalette(100); err != nil {
			return err
		}
		e.SetMouseList(engine.MouseNone)
		v.ClearPaletteChanges(0, 0xff)
		if err := e.TogglePages(); err != nil {
			return err
		}
		if err := e.DrawFileAnimation(e.FileAnimation()); err != nil {
			return err
		}
		v.CopyOffToBack()
		e.MoxUpdate()
		if err := e.TogglePages(); err != nil {
			return err
		}
		g.logoFade = 0
		var err error
		if skip, err = g.slowFadeIn(); err != nil {
			return err
		}
		e.SetMouseList(engine.MouseNone)
		if err := hold(0xf, func() error { return e.DrawFileAnimation(e.FileAnimation()) }); err != nil {
			return err
		}
	}
	if !skip {
		if err := g.logoFadeOut(); err != nil {
			return err
		}
		v.SetPageOff()
		v.Fill(0, 0, 0x27f, 0x1df, 0)
		if err := open(1); err != nil {
			return err
		}
		if err := e.DrawFileAnimation(e.FileAnimation()); err != nil {
			return err
		}
		v.CopyOffToBack()
		e.MoxUpdate()
		if err := e.TogglePages(); err != nil {
			return err
		}
		if err := e.FastFadeIn(); err != nil {
			return err
		}
		e.SetMouseList(engine.MouseNone)
		if err := hold(5, e.DrawFileAnimationStencil); err != nil {
			return err
		}
	}
	if err := g.logoFadeOut(); err != nil {
		return err
	}
	e.MoxUpdate()
	v.SetPageOff()
	v.Fill(0, 0, 0x27f, 0x1df, 0)
	v.CopyOffToBack()
	if !skip {
		return g.playCinematic(upperLBX(cStringAt(g.exe, introLBX)), 0, true)
	}
	return nil
}

// logoFadeOut is Logo_Fade_Out_ (0x2518f): darken from where the slow fade
// in stopped to 100, then black pages.
func (g *Game) logoFadeOut() error {
	e := g.e
	v := e.V
	v.SetRefreshFull()
	for p := g.logoFade; p <= 100; p += 10 {
		if err := e.DarkenPalette(p); err != nil {
			return err
		}
		e.MoxUpdate()
	}
	v.SetPageBack()
	v.Fill(0, 0, 0x27f, 0x1df, 0)
	v.CopyBackToOff()
	v.ClearPaletteChanges(0, 0xff)
	if err := e.TogglePages(); err != nil {
		return err
	}
	v.SetRefreshStencil()
	return nil
}

// slowFadeIn is Slow_Fade_In_ (0x251ef): darken 100 down to 0 one percent
// a step; a key or a button stops it there and is reported.
func (g *Game) slowFadeIn() (bool, error) {
	e := g.e
	e.V.SetRefreshFull()
	stop := false
	p := 100
	for ; p >= 0 && !stop; p-- {
		if e.Sys.KeyWaiting() {
			stop = true
			e.Sys.PopKey()
		}
		if e.MouseButton() != 0 {
			stop = true
		}
		if err := e.DarkenPalette(p); err != nil {
			return false, err
		}
		e.MoxUpdate()
	}
	e.V.SetRefreshStencil()
	if stop {
		g.logoFade = p
	}
	return stop, nil
}
