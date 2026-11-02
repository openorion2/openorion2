package game

import (
	"os"
	"time"

	"github.com/openorion2/openorion2/internal/engine"
	"github.com/openorion2/openorion2/internal/smk"
)

// Cinematics (CINEMATI.C): Smacker videos over the mini main screen. Spec:
// docs/screens/combat-screen.md ("Cinematics").

const (
	plntdfinLBX       = 0x182a7a // "plntdfin.lbx", a Smacker file
	smackPalFirst     = 10       // Set_Smacker_Palette_ copies colours 10..245
	smackPalLast      = 0xf5
	cinematicFrameEnt = 0x53 // MAINPUPS.LBX: the frame round the video
)

// Flic wrappers (CINEMATI.C, 0xc64ab..0xc6782): the popup ones play over
// the mini main screen, the end-of-game ones full screen with fades.
const (
	amoebaLBX = 0x1829ce // "amebafin.lbx"
	dimtvLBX  = 0x182a2e // "dimtvfin.lbx"

	eAntaransAttack = 0xae
	eTakeOrion      = 0x194
	eDiplomaticWin  = 0x22c
	eGeneralWin     = 0x136
	eAntaranWin     = 0xb7
	eLoserFlic      = 0x16b
)

// playPopupFlic is the body every popup wrapper (Play_Amoeba_Flic_ 0xc64ab,
// Play_Planetary_Destruction_Flic_ 0xc6513, Play_Dimensional_Portal_Flic_
// 0xc657b, Play_Antarans_Attack_Flic_ 0xc65e3, Play_Take_Orion_Flic_
// 0xc6650) repeats: setup, the video without fades, the cleanup inline.
func (g *Game) playPopupFlic(file string) error {
	e := g.e
	saved := e.GetAutoFunction()
	if err := g.doCinematicPopupSetup(); err != nil {
		return err
	}
	if g.tc != nil {
		g.tc.recordsLost = true
	}
	if err := g.playCinematic(upperLBX(file), 0, false); err != nil {
		return err
	}
	e.SetMouseList(engine.MouseArrow)
	e.DeactivateAutoFunction()
	e.PutAutoFunction(saved)
	g.cinematicRestoreVolumes()
	return nil
}

// playPlanetaryDestructionFlic is Play_Planetary_Destruction_Flic_
// (0xc6513): the stellar converter's video.
func (g *Game) playPlanetaryDestructionFlic() error {
	return g.playPopupFlic(cStringAt(g.exe, plntdfinLBX))
}

// playAmoebaFlic is Play_Amoeba_Flic_ (0xc64ab).
func (g *Game) playAmoebaFlic() error { return g.playPopupFlic(cStringAt(g.exe, amoebaLBX)) }

// playDimensionalPortalFlic is Play_Dimensional_Portal_Flic_ (0xc657b).
func (g *Game) playDimensionalPortalFlic() error { return g.playPopupFlic(cStringAt(g.exe, dimtvLBX)) }

// playAntaransAttackFlic is Play_Antarans_Attack_Flic_ (0xc65e3).
func (g *Game) playAntaransAttackFlic() error { return g.playPopupFlic(g.eStr(eAntaransAttack)) }

// playTakeOrionFlic is Play_Take_Orion_Flic_ (0xc6650).
func (g *Game) playTakeOrionFlic() error { return g.playPopupFlic(g.eStr(eTakeOrion)) }

// playFadedFlics is the end-of-game wrappers' body: each video with fades
// (Play_Cinematic_'s bl = 1, kept from the first Set_Mouse_List_), then the
// arrow. Play_Diplomatic_Win_Flic_ (0xc66bd) is E 0x22c and 0x136,
// Play_Normal_Win_Flic_ (0xc671b) 0x136, Play_Antaran_Win_Flic_ (0xc673d)
// 0xb7 and 0x136, Play_Loose_Flic_ (0xc6762) 0x16b.
func (g *Game) playFadedFlics(flags uint32, ens ...int) error {
	e := g.e
	e.SetMouseList(engine.MouseNone) // 0x17cf4a
	for _, n := range ens {
		if err := g.playCinematic(upperLBX(g.eStr(n)), flags, true); err != nil {
			return err
		}
		e.SetMouseList(engine.MouseNone)
		// The jumps into Play_Diplomatic_Win_Flic_ keep edx (0x400) only
		// for the first video.
		flags = 0
	}
	e.SetMouseList(engine.MouseArrow)
	return nil
}

// cinematicRestoreVolumes is the tail of Do_Cinematic_Popup_Cleanup_
// (0xc6481): settings +0x12 and +0x14 back from the setup's copies.
func (g *Game) cinematicRestoreVolumes() {
	if g.cinVolume[0] != 0 || g.cinVolume[1] != 0 {
		g.settings[0x12] = byte(g.cinVolume[0])
		g.settings[0x14] = byte(g.cinVolume[1])
	}
}

// doCinematicPopupSetup is Do_Cinematic_Popup_Setup_ (0xc63bb): the mini
// main screen with a frame becomes the auto function and the back page.
func (g *Game) doCinematicPopupSetup() error {
	e := g.e
	e.DeactivateAutoFunction()
	// The music and sound volumes are saved; sound takes the louder one.
	if g.settings[0x11] != 0 || g.settings[0x13] != 0 {
		g.cinVolume = [2]int16{int16(g.settings[0x12]), int16(g.settings[0x14])}
		g.settings[0x14] = max(g.settings[0x14], g.settings[0x12])
	} else {
		g.cinVolume = [2]int16{}
	}
	e.SetMouseList(engine.MouseNone) // 0x17cf4a
	if err := g.cinematicAutofunction(); err != nil {
		return err
	}
	e.AssignAutoFunction(g.cinematicAutofunction, 1)
	e.V.CopyOffToBack()
	return nil
}

// cinematicAutofunction is Cinematic_Autofunction_ (0xc6313).
func (g *Game) cinematicAutofunction() error {
	e := g.e
	e.V.SetPageOff()
	p, err := g.picture(mainpupsLBX, cinematicFrameEnt)
	if err != nil {
		return err
	}
	w, h := int(int16(p.Width)), int(int16(p.Height))
	g.fullDrawScreen = true
	g.ms.dontDrawToBack = true
	if err := g.drawMiniMainScreen(); err != nil {
		return err
	}
	if err := e.Draw((0x280-w)/2+3, (0x1e0-h)/2+1, p); err != nil {
		return err
	}
	e.V.ClearPaletteChanges(0, 0xff)
	return nil
}

// playCinematic is Play_Cinematic_ (0x14df7): a Smacker file centred (a
// quarter down with flags 0x100000), frame by frame until the last but one,
// a key or a button. With fade the screen goes black first and the video
// fades in; it never fades out (Smack_Fade_(4) sees the fade percent still
// at 100) and ends on a black screen with Fast_Fade_Out_.
func (g *Game) playCinematic(file string, flags uint32, fade bool) error {
	e := g.e
	v := e.V
	v.SetPageOff()
	e.FadeStreamDown(0x19)
	var saved engine.AutoFunction
	if fade {
		if err := g.smackFade(4); err != nil {
			return err
		}
		e.SetMouseList(engine.MouseNone) // 0x180178
	} else {
		saved = e.GetAutoFunction()
		e.DeactivateAutoFunction()
		v.SetRefreshStencil()
		e.EnableDrawAllFields()
		if err := e.DrawVisibleFields(); err != nil {
			return err
		}
		v.CopyOffToBack()
	}
	v.SetRefreshDifferential()
	sm := &smackPlayer{}
	e.AssignAutoFunction(func() error { return g.drawSmackerAnimation(sm, sm.x, sm.y) }, 0) // Draw_Smack_Screen_
	e.ClearFields()
	e.DeactivateHelpList()
	v.SetWindow(0, 0, 0x27f, 0x1df)
	v.ClipOn()
	e.StopStream()
	a, err := os.ReadFile(g.findFile(file))
	if err != nil {
		return err
	}
	vid, err := smk.Open(a)
	if err != nil {
		return err
	}
	sm.vid = vid
	// Open_Smacker_Animation_ enables the tracks when music is on (+0x13).
	if t := &vid.Audio[0]; g.settings[0x13] == 1 && t.Present {
		sm.withSnd = true
		ch, bits := 1, 8
		if t.Stereo {
			ch = 2
		}
		if t.Bits16 {
			bits = 16
		}
		sm.sound = e.StartVideoSound(t.Rate, ch, bits)
	}
	sm.x = int(int32(0x280-vid.Width) / 2)
	if flags&0x100000 != 0 {
		sm.y = int(int32(0x1e0-vid.Height) / 4)
	} else {
		sm.y = int(int32(0x1e0-vid.Height) / 2)
	}
	if fade {
		v.SetPageOff()
		v.Fill(0, 0, 0x27f, 0x1df, 0)
		if err := e.TogglePages(); err != nil {
			return err
		}
		v.Fill(0, 0, 0x27f, 0x1df, 0)
		if err := g.drawSmackerAnimation(sm, sm.x, sm.y); err != nil {
			return err
		}
		e.RefreshPalette()
		if err := e.DarkenPalette(100); err != nil {
			return err
		}
		if err := e.TogglePages(); err != nil {
			return err
		}
		e.SetFadePercent(100)
		if err := g.smackFade(3); err != nil {
			return err
		}
	}
	for done := false; !done; {
		if e.Sys.KeyWaiting() {
			e.Sys.PopKey()
			done = true
		}
		if e.MouseButton() != 0 {
			done = true
		}
		v.SetPageOff()
		if err := g.drawSmackerAnimation(sm, sm.x, sm.y); err != nil {
			return err
		}
		if err := e.TogglePages(); err != nil {
			return err
		}
		e.MoxUpdate()
		if sm.frameNum == vid.Frames-1 {
			done = true
		}
	}
	e.DeactivateAutoFunction()
	if fade {
		if err := g.smackFade(4); err != nil {
			return err
		}
		v.SetPageOff()
		v.Fill(0, 0, 0x27f, 0x1df, 0)
		if err := e.TogglePages(); err != nil {
			return err
		}
		e.SetFadePercent(0)
		e.SetFadeSpeedPercent(10)
		if err := e.FastFadeOut(); err != nil {
			return err
		}
	}
	e.StopVideoSound(sm.sound) // SmackClose
	g.lastSmackSound, g.lastSmackFrames = sm.played, sm.vid.Frame
	e.MoxUpdate()
	g.mus.current = -1
	g.playBackgroundMusic()
	if !fade {
		g.fullDrawScreen = true
		e.PutAutoFunction(saved)
	}
	v.SetRefreshStencil()
	return nil
}

// smackFade is Smack_Fade_ (0x15085): mode 3 darkens 90..0 unless the fade
// percent is 0, mode 4 darkens 0..90 unless it is 100; it does not change
// the fade percent.
func (g *Game) smackFade(mode int) error {
	e := g.e
	p, step := 0, 10
	if mode == 3 {
		if e.FadePercent() == 0 {
			return nil
		}
		p, step = 100, -10
	} else if e.FadePercent() == 100 {
		return nil
	}
	for {
		if p >= 0 && p < 100 {
			if err := e.DarkenPalette(p); err != nil {
				return err
			}
		}
		p += step
		if p == 110 || p == -10 {
			break
		}
	}
	e.V.ClearPaletteChanges(0, 0xff)
	return nil
}

// smackPlayer is the open Smack: the decoder, smk->FrameNum, the time the
// next frame is due (SmackWait) and where Draw_Smack_Screen_ puts it.
type smackPlayer struct {
	vid      *smk.Video
	frameNum int
	due      time.Duration
	x, y     int
	sound    int // the mixer voice of track 0 when the video plays with sound
	played   int // sound bytes handed to the mixer
	withSnd  bool
}

// drawSmackerAnimation is Draw_Smacker_Animation_ (0x132869): frame
// FrameNum decoded and copied at (x, y), its palette from colour 10 to 245,
// and FrameNum advanced short of the last.
func (g *Game) drawSmackerAnimation(sm *smackPlayer, x, y int) error {
	v := g.e.V
	vid := sm.vid
	sys := g.e.Sys
	for sys.Elapsed() < sm.due { // SmackWait
		if err := sys.Sleep(sm.due - sys.Elapsed()); err != nil {
			return err
		}
	}
	newPal := false
	for vid.Frame <= sm.frameNum {
		if err := vid.Next(); err != nil {
			return err
		}
		newPal = newPal || vid.NewPal
		if sm.withSnd {
			g.e.VideoSound(sm.sound, vid.Audio[0].Samples)
			sm.played += len(vid.Audio[0].Samples)
		}
	}
	if newPal {
		for i := smackPalFirst; i <= smackPalLast; i++ {
			c := vid.Palette[i]
			v.Palette[i] = engine.Color{Changed: 1, R: c[0], G: c[1], B: c[2]}
		}
	}
	for r := 0; r < vid.Height; r++ {
		o := (y+r)*engine.W + x
		copy(v.Current[o:o+vid.Width], vid.Pixels[r*vid.Width:(r+1)*vid.Width])
	}
	v.AddSquare(x, y, x+vid.Width, y+vid.Height)
	if vid.Frames-1 != sm.frameNum {
		sm.frameNum++ // SmackNextFrame
		sm.due = sys.Elapsed() + vid.Period
	}
	return nil
}
