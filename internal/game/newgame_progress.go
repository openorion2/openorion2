package game

import (
	"strings"

	"github.com/openorion2/openorion2/internal/font"
	"github.com/openorion2/openorion2/internal/gfx"
)

// The progress screen of the generation: a message whose letters change
// colour as the stars and home worlds are made (Universe_Generation_'s
// set-up, Set_Up_Home_Gen_Msg_, Draw_Advancing_Fill_Message_). Spec:
// docs/newgame/generation.md, "Progress".

const (
	genProgressLBX   = 0x1798de // "raceopt.lbx": the background
	genMsgUniverse   = 0xf2     // H_Message_: the galaxy message
	genMsgHomeWorlds = 0xa1     // H_Message_: the home worlds message
	genMsgY          = 0xd7
)

// genProgress holds _galaxy_msg (0x19c2f4), its bitmap (0x19c2e4) and the
// counter (0x19c2f0) the fill advances by, and _HOME_WORLD_GEN_MAX_CTR.
type genProgress struct {
	text    string
	bm      *gfx.Picture
	counter int16
	homeMax int32
}

// genColors are the generation's two colour sets, Font_Colors2_(4, 0x77,
// 0x79) and (4, 0x77, 0x7c): the message and its filled letters.
func (g *Game) genColors() (font.Colors, font.Colors) {
	return g.fontColors2(4, 0x77, 0x79), g.fontColors2(4, 0x77, 0x7c)
}

// genProgressSetup is the shared part of Universe_Generation_'s set-up
// and Set_Up_Home_Gen_Msg_ (0x7c78e): the background on the back page, the
// message printed (outlined) into a blank bitmap.
func (g *Game) genProgressSetup(msg int) error {
	if g.e == nil {
		return nil
	}
	e := g.e
	gp := &g.genProg
	c1, _ := g.genColors()
	e.V.SetPageBack()
	bg, err := g.picture(strings.ToUpper(cStringAt(g.exe, genProgressLBX)), 0)
	if err != nil {
		return err
	}
	if err := e.Draw(0, 0, bg); err != nil {
		return err
	}
	e.V.CopyBackToOff()
	gp.text = g.hMessage(msg)
	gp.bm = gfx.NewBitmap(0x145, 0x1e, nil)
	gp.counter = 0
	t := e.Text()
	t.SetStyleShadow(4, c1, font.OutlineHeavy)
	t.SetOutlineColor(2, 1)
	return t.ToBitmap(bitmapTarget(gp.bm), 0, 0, []byte(gp.text))
}

// universeProgressSetup is Universe_Generation_'s set-up (0x8dc5b).
func (g *Game) universeProgressSetup() error {
	if g.e == nil {
		return nil
	}
	if g.e.FadePercent() != 0 {
		if err := g.e.FastFadeIn(); err != nil {
			return err
		}
	}
	return g.genProgressSetup(genMsgUniverse)
}

// setUpHomeGenMsg is Set_Up_Home_Gen_Msg_ (0x7c78e).
func (g *Game) setUpHomeGenMsg() error {
	n := int32(g.state.NumStars) * 0x19 / 0x14
	if g.state.NumStars > 0x36 {
		n = 0x174
	}
	g.genProg.homeMax = n
	return g.genProgressSetup(genMsgHomeWorlds)
}

// genProgressStep is a progress call site: the message's x, then
// Draw_Advancing_Fill_Message_ (0x8efe1) and _TOGGLE_PAGES_.
func (g *Game) genProgressStep(max int16, full bool) error {
	if g.e == nil {
		return nil
	}
	e := g.e
	gp := &g.genProg
	c1, c2 := g.genColors()
	t := e.Text()
	t.SetStyle(4, c1)
	x := (0x280 - int32(int16(t.Width([]byte(gp.text))))) / 2
	t.SetStyle(4, c1)
	_, right, h := g.advancingBitmapStringRange(gp.counter, max, full)
	replaceBitmapColors(gp.bm, 0, 0, right, h, c1[:], c2[:])
	if err := e.Draw(int(int16(x)), genMsgY, gp.bm); err != nil {
		return err
	}
	gp.counter++
	return e.TogglePages()
}

// advancingBitmapStringRange is Get_Advancing_Bitmap_String_Range_
// (0x8f399): the slice of the message's width step n of max covers, and
// the font height.
func (g *Game) advancingBitmapStringRange(n, max int16, full bool) (left, right, h int16) {
	t := g.e.Text()
	w := int16(t.Width([]byte(g.genProg.text)))
	h = int16(t.Height())
	if w < max {
		step := (int32(max)*10/int32(w) + 9) / 10
		if int32(n)%step != 0 && !full {
			return 0, 0, h
		}
		left = int16(int32(w) * int32(n) * 10 / int32(max) / 10)
		if left > w {
			left = w
		}
		if full {
			right = w
		} else {
			right = left + 1
		}
		if right > w {
			right = w
		}
		return left, right, h
	}
	if full {
		n = max - 1
	}
	step := (int32(w)*10/int32(max) + 9) / 10
	left = int16(int32(n) * step)
	if left > w {
		left = w
	}
	right = left + int16(step)
	if right > w {
		right = w
	}
	return left, right, h
}

// replaceBitmapColors is Replace_Bitmap_Colors_ (0x8fc82) for the region
// [x1, x2) × [y1, y2): each pixel goes through the eight pairs in order.
func replaceBitmapColors(bm *gfx.Picture, x1, y1, x2, y2 int16, from, to []byte) {
	pix := bm.Pixels()
	w := int16(bm.Width)
	for y := y1; y < y2; y++ {
		for x := x1; x < x2; x++ {
			i := int(int32(w)*int32(y) + int32(x))
			if i < 0 || i >= len(pix) {
				continue
			}
			p := pix[i]
			for k := 0; k < 8; k++ {
				if p == from[k] {
					p = to[k]
					pix[i] = p
				}
			}
		}
	}
}
