package game

import (
	"github.com/openorion2/openorion2/internal/engine"
	"github.com/openorion2/openorion2/internal/font"
	"github.com/openorion2/openorion2/internal/gfx"
)

// The Antaran messenger (ANTAROOM.C): Antarans_ shows it when an attack is
// heading for one of the human's colonies, else the attack cinematic.
// Spec: docs/screens/antarans.md.

const (
	antaroomLBX         = "antaroom.LBX"
	antarmsgLBX         = "antarmsg.LBX"
	antaranRoomColors   = 0x180170 // _antaran_room_normal_colors
	antaranRoomMouseImg = 0x180164 // _antaran_room_blank_mouse: image, centre, rectangle
)

// antaranRoomState holds ANTAROOM.C's globals.
type antaranRoomState struct {
	message string // _antaran_room_message
	showing bool   // port-only: in the input loop, for tests
}

// mainAntaranRoomScreen is Main_Antaran_Room_Screen_ (0x14aac): the
// Antaran speaks (message set 1 − attacking: 1 before the attack date).
func (g *Game) mainAntaranRoomScreen(attacking bool) error {
	e := g.e
	e.ClearFields()
	e.DeactivateHelpList()
	e.EnableDrawAllFields()
	e.V.ResetWindow()
	e.V.ClipOff()
	blank := g.mouseListAt(antaranRoomMouseImg)
	e.SetMouseList(blank)
	g.playStreamingMusic(0x78, -1, true) // 0x14ae9: the Antarans' theme
	e.SetFadePercent(0)
	e.SetFadeSpeedPercent(10)
	if err := e.FastFadeOut(); err != nil {
		return err
	}
	e.ClearFields()
	e.V.SetRefreshStencil()
	saved := e.GetAutoFunction()
	e.DeactivateAutoFunction()
	e.ClearFields()
	if err := g.reloadMainAntaranRoomScreen(); err != nil {
		return err
	}
	set := 1
	if attacking {
		set = 0
	}
	if err := g.getAntaranRoomMessage(set); err != nil {
		return err
	}
	e.AssignAutoFunction(g.drawMainAntaranRoomScreen, 2)
	g.ar.showing = true
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
		if in == f || e.Sys.KeyWaiting() {
			break
		}
		if err := g.drawMainAntaranRoomScreen(); err != nil {
			return err
		}
		if err := e.TogglePages(); err != nil {
			return err
		}
		if err := e.ReleaseTime(2); err != nil {
			return err
		}
	}
	g.ar.showing = false
	g.ms.autoTurnExit = true
	e.DeactivateAutoFunction()
	e.ClearFields()
	e.SetMouseList(blank)
	g.playBackgroundMusic() // 0x14bc9
	e.SetFadePercent(0)
	e.SetFadeSpeedPercent(10)
	if err := e.FastFadeOut(); err != nil {
		return err
	}
	e.PutAutoFunction(saved)
	return nil
}

// mouseListAt reads a one-entry mouse list from the executable.
func (g *Game) mouseListAt(addr uint32) []engine.MouseRegion {
	r := func(o uint32) int { return int(g.exeI16(addr + o)) }
	return []engine.MouseRegion{{Image: r(0), Center: r(2), X1: r(4), Y1: r(6), X2: r(8), Y2: r(10)}}
}

// reloadMainAntaranRoomScreen is Reload_Main_Antaran_Room_Screen_
// (0x14c83): the cursor and palette from antaroom.LBX 0, the room (entry
// 1) as a file animation, first frame on both pages.
func (g *Game) reloadMainAntaranRoomScreen() error {
	e := g.e
	pic, err := g.picture(antaroomLBX, 0)
	if err != nil {
		return err
	}
	e.ClearMouseImage(0)
	cursor := gfx.NewBlankBitmap(0x1e, 0x1e)
	cursor.DrawPictureToBitmap(pic)
	e.V.SetPageOff()
	e.DrawPalette(pic)
	e.V.SetPaletteChanges(0, 0xff)
	e.V.Fill(0, 0, 0x27f, 0x1df, 0)
	e.V.SetRefreshStencil()
	room, err := g.picture(antaroomLBX, 1)
	if err != nil {
		return err
	}
	e.OpenFileAnimation(room)
	e.V.SetPageOff()
	if err := e.DrawFileAnimation(room); err != nil {
		return err
	}
	e.RefreshPalette()
	e.V.CopyOffToBack()
	e.DrawBitmapToMouse(0, 0, cursor, 0)
	e.SetMouseList(engine.MouseArrow)
	return nil
}

// getAntaranRoomMessage is Get_Antaran_Room_Message_ (0x14d7c): one of four
// wordings of message set n, picked with Random_(4).
func (g *Game) getAntaranRoomMessage(n int) error {
	r := int(g.state.Random.Random(4))
	a, err := g.archive(antarmsgLBX)
	if err != nil {
		return err
	}
	t, err := a.Table(n*4 + int(g.settings[0xd2])*8 + r - 1)
	if err != nil {
		return err
	}
	rec := t.Record(0)
	g.ar.message = string(rec[:min(len(rec), 200)])
	if i := indexNUL(g.ar.message); i >= 0 {
		g.ar.message = g.ar.message[:i]
	}
	return nil
}

func indexNUL(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == 0 {
			return i
		}
	}
	return -1
}

// drawMainAntaranRoomScreen is Draw_Main_Antaran_Room_Screen_ (0x14bfd).
func (g *Game) drawMainAntaranRoomScreen() error {
	e := g.e
	t := e.Text()
	e.V.SetPageOff()
	t.SetOutlineColor(0xfa, 0xfa)
	if g.settings[6] == 1 {
		if err := e.DrawFileAnimationStencil(); err != nil {
			return err
		}
	}
	t.SetOutlineColor(0, 0xfa)
	t.SetStyleShadow(4, g.exeColors(antaranRoomColors), font.Outline)
	h, err := t.ParagraphMaxHeight(0x226, g.ar.message)
	if err != nil {
		return err
	}
	err = t.PrintParagraph(0x37, int(int16(0x1d6-h)), 0x226, g.ar.message, 2)
	g.playStreamingMusic(0x78, -1, true) // 0x14c7a
	return err
}
