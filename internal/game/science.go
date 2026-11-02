package game

import (
	"fmt"
	"strings"

	"github.com/openorion2/openorion2/internal/engine"
	"github.com/openorion2/openorion2/internal/font"
	"github.com/openorion2/openorion2/internal/gfx"
	"github.com/openorion2/openorion2/internal/save"
)

// The science room (SCIENCE.C): a scientist or spy shows off technologies
// one after another, each with its animation and help text. Spec:
// docs/screens/tech-select.md.

const (
	scienceLBX         = "SCIENCE.LBX"
	billTextLBX        = "billtext.lbx"
	scienceHighColors  = 0x18413c // _science_high_color
	scienceNormColors  = 0x184144 // _science_normal_color
	bmScaleX           = 0x18414c // _bm_scale_x, 5 shorts
	bmScaleY           = 0x184158
	bmScaledX1         = 0x184164
	bmScaledY1         = 0x18416e
	weaponModNames     = 0x17fd0b // _weapon_modifications[i].name, 0xf-byte rows
	jumbleTable        = 0x189024 // the 256 random bytes of Jumble_Bitmap_Pixels_
	jumbleToggleInit   = 0x189010 // its side flag in .data
	cycleDirectionInit = 0x1845ec // cycle_direction_flag in .data
	mouseListVarImage  = 0x14     // _mouse_list_variable's image
)

// mouseListVariable is _mouse_list_variable (0x17cf0c).
var mouseListVariable = []engine.MouseRegion{{Image: mouseListVarImage, X2: 639, Y2: 479}}

// scienceRoomState holds what SCIENCE.C keeps between calls.
type scienceRoomState struct {
	jumbleToggle uint32 // 0x189010, Jumble_Bitmap_Pixels_' side
	jumbleInit   bool
	showing      bool // port-only: in the room's input loop, for tests
	settled      bool // port-only: the current text has finished jumbling in
}

// showOffResearchedTech is Show_Off_Researched_Tech_ (0x10d1fc): the
// scientist shows the applications of p's research field that p now knows.
func (g *Game) showOffResearchedTech(p int) error {
	pl := &g.state.Players[p]
	var list []uint16
	if f := int(pl.ResearchField); f != 0 {
		for k := 0; k < 4; k++ {
			if a := g.techFieldApp(f, k); int16(a) > 0 && pl.AppStatus[a] == techKnown {
				list = append(list, uint16(a))
			}
		}
	}
	if len(list) == 0 {
		g.e.ClearFields()
		return g.e.FastFadeOut()
	}
	t, err := g.textMessage(billTextLBX, 1, 0xfa)
	if err != nil {
		return err
	}
	if t, err = g.decodeTextMessage(t, -1, -1, -1, -1, -1, -1, int(pl.ResearchField), 0xfa); err != nil {
		return err
	}
	name := fmt.Sprintf(cStringAt(g.exe, scienceScientistFmt), pl.Race)
	return g.scienceRoom(name, list, t)
}

// The animation file names of the science room (formats in .data).
const (
	scienceTechAnimFmt  = 0x1840dc // "TANM_%03hi.LBX"
	scienceScientistFmt = 0x1840eb // "SR_R%x_SC.LBX"
	scienceSpyFmt       = 0x1840f9 // "SR_R%x_SP.LBX"
	scienceTrooperFmt   = 0x184107 // "SR_R%x_TR.LBX"
)

// showOffStolenTech is Show_Off_Stolen_Tech_ (0x10d041): the human's spy
// shows tech, stolen from player victim.
func (g *Game) showOffStolenTech(tech, victim int) error {
	s := &g.state
	t, err := g.textMessage(billTextLBX, 2, 0x100)
	if err != nil {
		return err
	}
	if t, err = g.decodeTextMessage(t, int(s.PlayerNum), -1, tech, -1, victim, -1, -1, 0x100); err != nil {
		return err
	}
	// Cache_Player_Spy_Anim_ copies the file from the CD when it is missing.
	name := fmt.Sprintf(cStringAt(g.exe, scienceSpyFmt), s.Players[s.PlayerNum].Race)
	if err := g.scienceRoom(name, []uint16{uint16(tech)}, t); err != nil {
		return err
	}
	g.resetMousePicture()
	g.playBackgroundMusic() // 0x10d0ca, 0x10d15d
	return nil
}

// showOffCapturedTech is Show_Off_Captured_Tech_ (0x10d0da): a trooper
// shows the n applications captured with a colony; the text names star
// sys, which the callers fill with a planet number.
func (g *Game) showOffCapturedTech(apps []int16, n, sys int16) error {
	s := &g.state
	t, err := g.textMessage(billTextLBX, 3, 0x100)
	if err != nil {
		return err
	}
	if t, err = g.decodeTextMessage(t, -1, int(sys), -1, -1, -1, -1, -1, 0x100); err != nil {
		return err
	}
	techs := make([]uint16, 0, n)
	for k := int16(0); k < n && int(k) < len(apps); k++ {
		techs = append(techs, uint16(apps[k]))
	}
	name := fmt.Sprintf(cStringAt(g.exe, scienceTrooperFmt), s.Players[s.PlayerNum].Race)
	if err := g.scienceRoom(name, techs, t); err != nil {
		return err
	}
	g.resetMousePicture()
	g.playBackgroundMusic() // 0x10d0ca, 0x10d15d
	return nil
}

// resetMousePicture is Reset_Mouse_Picture_ (0xc0af4).
func (g *Game) resetMousePicture() {
	g.e.SetMouseList(engine.MouseArrow)
	g.e.EnableDrawAllFields()
}

// clearMousePicture is Clear_Mouse_Picture_ (0xc0abc): a blank pointer;
// while colonists are picked up the fields stop drawing.
func (g *Game) clearMousePicture() {
	g.e.ClearMouseImage(mouseListVarImage - 1)
	g.e.SetMouseList(mouseListVariable)
	if c := g.col.clusterN; c != -1 && g.getFirstUnemployed(int(c)) != -1 {
		g.e.DisableDrawAllFields()
	}
}

// scienceRoom is Science_Room_ (0x10d31c): the room fades in around the
// animation in file anim; each click shows the next of techs (an
// application, or with bit 0x4000 an item whose next entry is a mask of
// weapon modifications), its help text jumbling in; Esc, or a click after
// the last, closes the door.
func (g *Game) scienceRoom(anim string, techs []uint16, text string) error {
	e := g.e
	t := e.Text()
	saved := e.GetAutoFunction()
	e.DeactivateHelpList()
	e.V.ClipOff()
	e.ClearFields()
	names := make([]string, len(techs))
	for i := range techs {
		names[i] = fmt.Sprintf(strings.Replace(cStringAt(g.exe, scienceTechAnimFmt), "%03hi", "%03d", 1), int16(techs[i]&0x1ff))
	}
	mousePic, err := g.picture(scienceLBX, 0)
	if err != nil {
		return err
	}
	text1 := gfx.NewBlankBitmap(0xaf, 0xc3)
	text2 := gfx.NewBlankBitmap(0xaf, 0xc3)
	clear(g.cache)
	anims := g.settings[6] != 0
	if !anims {
		door, err := g.cached(scienceLBX, 1)
		if err != nil {
			return err
		}
		setFrame(door, 5)
	}
	t.SetOutlineColor(0, 0xf0)
	t.SetStyleShadow(4, g.exeColors(scienceNormColors), font.Outline)
	e.SetMouseList(engine.MouseNone)
	if err := e.FastFadeOut(); err != nil {
		return err
	}
	scientist, err := g.picture(anim, 0)
	if err != nil {
		return err
	}
	if err := e.OpenMultiFileAnimation(1, scientist); err != nil {
		return err
	}
	if _, err := e.AddHiddenField(0, 0, 0x27f, 0x1df, "", 0); err != nil {
		return err
	}
	esc, err := e.AddHotKey("\x1b")
	if err != nil {
		return err
	}
	e.V.SetPageOff()
	e.AssignAutoFunction(func() error { return nil }, 2) // Draw_Science_Room_
	g.clearMousePicture()
	e.ClearMouseImage(0)
	cursor := gfx.NewBlankBitmap(0x1e, 0x1e)
	cursor.DrawPictureToBitmap(mousePic)
	e.DrawBitmapToMouse(0, 0, cursor, 0)
	e.SetMouseList(engine.MouseArrow)
	itemLabel, err := g.textMessage(billTextLBX, 4, 0x50)
	if err != nil {
		return err
	}
	if _, err := g.cached(scienceLBX, 1); err != nil {
		return err
	}
	e.V.SetPaletteChanges(0, 0xff)
	e.DrawPalette(mousePic)
	g.turnOffPalettes()
	e.V.SetRefreshStencil()
	e.V.Fill(0, 0, 0x27f, 0x1df, 0)
	if err := e.DrawMultiFileAnimation(1); err != nil {
		return err
	}
	e.V.CopyOffToBack()
	g.turnOffPalettes()
	e.Sys.MouseBuffer()
	e.Sys.MouseBuffer2()
	for fade := 100; fade > 0; {
		g.playStreamingMusic(0x75, -2, true) // 0x10d5fb: the lab's theme
		fade -= 20
		e.MarkTime()
		fade = max(fade, 0)
		e.V.SetPaletteChanges(0, 0xff)
		if err := e.DarkenPalette(fade); err != nil {
			return err
		}
		e.V.ClearPaletteChanges(0, 0xff)
		room, err := g.cached(scienceLBX, 2)
		if err != nil {
			return err
		}
		if err := e.Draw(0, 0, room); err != nil {
			return err
		}
		if err := e.TogglePages(); err != nil {
			return err
		}
		if err := e.ReleaseTime(1); err != nil {
			return err
		}
	}
	e.SetFadePercent(0)
	if len(techs) != 0 {
		if err := g.scienceRoomLoop(techs, names, text, itemLabel, esc, text1, text2, anims); err != nil {
			return err
		}
	}
	e.ClearFields()
	e.Sys.MouseBuffer()
	e.Sys.MouseBuffer2()
	e.PutAutoFunction(saved)
	e.SetFadePercent(0)
	return nil
}

// scienceRoomLoop is Science_Room_'s input loop and the closing door.
func (g *Game) scienceRoomLoop(techs []uint16, names []string, text, itemLabel string, esc int, text1, text2 *gfx.Picture, anims bool) error {
	e, sr := g.e, &g.sr
	t := e.Text()
	left := len(techs)
	k, first, ready := 0, true, false
	jumble, rng := 0, 6
	sr.showing = true
	defer func() { sr.showing = false }()
	for {
		g.playStreamingMusic(0x75, -2, true) // 0x10d693
		e.MarkTime()
		sr.settled = ready && int16(jumble) <= 0
		in, err := e.GetInput()
		if err != nil {
			return err
		}
		if in != 0 || first {
			if left == 0 || in == esc {
				return g.scienceRoomClose(text1, text2, anims)
			}
			if err := g.updateScienceAnims(false, true, true); err != nil {
				return err
			}
			if err := e.TogglePages(); err != nil {
				return err
			}
			item, err := g.picture(names[k], 0)
			if err != nil {
				return err
			}
			if err := e.OpenMultiFileAnimation(0, item); err != nil {
				return err
			}
			e.V.SetPageOff()
			e.DrawMultiFileAnimationPalette(0)
			e.RefreshPalette()
			text2.CopyBitmapToBitmap(text1)
			help, err := g.helpRecord(int(techs[k] & 0x1ff))
			if err != nil {
				return err
			}
			*text1 = *gfx.NewBlankBitmap(0xaf, 0xc3)
			target := font.Target{Pix: text1.Pixels(), Stride: text1.Width, H: text1.Height}
			t.SetStyle(3, g.exeColors(scienceHighColors))
			title := save.CString(help.title)
			if err := t.PrintFormattedToBitmap(2, 1, 0xab, title, 2, target); err != nil {
				return err
			}
			h, err := t.FormattedMaxHeight(0xab, title)
			if err != nil {
				return err
			}
			h++
			t.SetStyle(2, g.exeColors(scienceHighColors))
			t.SetLF(1)
			body, y := save.CString(help.text), h
			if techs[k]&0x4000 != 0 {
				k++
				if err := t.ToBitmap(target, 2, int(int16(h)), []byte(itemLabel)); err != nil {
					return err
				}
				body = ""
				for m, i := uint32(techs[k]), 1; m != 0; m, i = uint32(int32(m)>>1), i+1 {
					if m&1 != 0 {
						body += cStringAt(g.exe, g.exe.u32(uint32(weaponModNames+i*0xf))) + "\r"
					}
				}
				y = h + 0xf
			}
			if err := t.PrintFormattedToBitmap(2, int(int16(y)), 0xab, body, 2, target); err != nil {
				return err
			}
			first = false
			k++
			left--
			jumble = 0
			if anims {
				jumble = 100
			}
			rng = 6
			for {
				if err := g.updateScienceAnims(false, true, true); err != nil {
					return err
				}
				if e.MouseButton() == 0 {
					break
				}
			}
			e.Sys.MouseBuffer()
			e.Sys.MouseBuffer2()
		} else if err := g.updateScienceAnims(true, true, true); err != nil {
			return err
		}
		if ready {
			if int16(jumble) <= 0 {
				if err := e.Draw(0xdd, 0x2c, text1); err != nil {
					return err
				}
			} else {
				g.jumbleBitmap(text2, text1, jumble, rng)
				if err := e.Draw(0xdd, 0x2c, text2); err != nil {
					return err
				}
				jumble -= 20
				if rng > 1 {
					rng--
				}
			}
		} else {
			door, err := g.cached(scienceLBX, 1)
			if err != nil {
				return err
			}
			ready = door.CurrentFrame >= 5
		}
		t.SetOutlineColor(0, 0xf2)
		t.SetStyleShadow(4, g.exeColors(scienceNormColors), font.Outline)
		if err := t.PrintFormatted(0x91, 0x1a4, 0x17c, text, 2); err != nil {
			return err
		}
		if err := e.ReleaseTime(1); err != nil {
			return err
		}
		if err := e.TogglePages(); err != nil {
			return err
		}
	}
}

// scienceRoomClose plays the door closing: the text shrinks over the last
// five frames.
func (g *Game) scienceRoomClose(text1, text2 *gfx.Picture, anims bool) error {
	e := g.e
	door, err := g.cached(scienceLBX, 1)
	if err != nil {
		return err
	}
	for f := int16(door.CurrentFrame) - 1; f >= 0 && anims; f-- {
		e.MarkTime()
		setFrame(door, int(f))
		if err := g.updateScienceAnims(false, true, true); err != nil {
			return err
		}
		if f < 5 {
			sx, sy := g.exeI16(uint32(bmScaleX+2*int(f))), g.exeI16(uint32(bmScaleY+2*int(f)))
			if err := text2.ReduceBitmap(text1, int(sx), int(sy)); err != nil {
				return err
			}
			x, y := g.exeI16(uint32(bmScaledX1+2*int(f))), g.exeI16(uint32(bmScaledY1+2*int(f)))
			if err := e.Draw(int(x), int(y), text2); err != nil {
				return err
			}
		} else if err := e.Draw(0xdd, 0x2c, text1); err != nil {
			return err
		}
		if err := e.ReleaseTime(1); err != nil {
			return err
		}
		if err := e.TogglePages(); err != nil {
			return err
		}
	}
	g.playStreamingMusic(0x75, -2, true) // 0x10da4a
	if err := g.updateScienceAnims(false, false, false); err != nil {
		return err
	}
	return e.TogglePages()
}

// updateScienceAnims is Update_Science_Anims_ (0x10da98): the scientist's
// next frame into both pages (without animations the first slot's frame is
// set to 0x12 instead), then the room, the item's frame and the door.
func (g *Game) updateScienceAnims(item, room, door bool) error {
	e := g.e
	if g.settings[6] != 0 {
		if err := e.DrawMultiFileAnimationStencil(1); err != nil {
			return err
		}
	} else if e.FileAnimation() != nil {
		e.SetFileAnimationFrame(0x12)
	}
	if room {
		p, err := g.cached(scienceLBX, 2)
		if err != nil {
			return err
		}
		if err := e.Draw(0, 0, p); err != nil {
			return err
		}
	}
	if item {
		if err := e.DrawMultiFileAnimation(0); err != nil {
			return err
		}
	}
	if door {
		p, err := g.cached(scienceLBX, 1)
		if err != nil {
			return err
		}
		return e.Draw(0, 0, p)
	}
	return nil
}

// jumbleBitmap is Jumble_Bitmap_ (0x12eee7): dst becomes a copy of src
// with pixels swapped where the random table says so; the seed it draws
// from is put back.
func (g *Game) jumbleBitmap(dst, src *gfx.Picture, percent, rng int) {
	sr, s := &g.sr, &g.state
	if !sr.jumbleInit {
		sr.jumbleToggle, sr.jumbleInit = g.exe.u32(jumbleToggleInit), true
	}
	seed := s.Random.Get()
	dst.CopyBitmapToBitmap(src)
	start := int((s.Random.Random(1000) - 1) & 0xff)
	gfx.JumblePixels(src.Pixels(), dst.Pixels(), int(int16(src.Width)), int(int16(src.Height)), rng, percent,
		g.exe.bytes(jumbleTable, 256), start, &sr.jumbleToggle)
	s.Random.Set(seed)
}

// turnOffPalettes is Turn_Off_Palettes_ (0x102cdf).
func (g *Game) turnOffPalettes() {
	_ = g.e.DarkenPalette(100)
	g.e.V.ClearPaletteChanges(0, 0xff)
}

// setFrame is Set_Animation_Frame_ (0x12b753), clamped to the last frame.
func setFrame(p *gfx.Picture, n int) {
	if n >= p.Frames {
		n = p.Frames - 1
	}
	p.CurrentFrame = n
}

// helpRecord is Far_Reload_Data of record n of the help file's entry 0.
func (g *Game) helpRecord(n int) (helpRecord, error) {
	a, err := g.archive(g.helpLBXName())
	if err != nil {
		return helpRecord{}, err
	}
	tab, err := a.Table(0)
	if err != nil {
		return helpRecord{}, err
	}
	if n < 0 || (n+1)*tab.Size > len(tab.Data) {
		return helpRecord{}, fmt.Errorf("help record %d out of range", n)
	}
	return parseHelpRecord(tab.Record(n)), nil
}
