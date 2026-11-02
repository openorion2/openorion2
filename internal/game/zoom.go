package game

import (
	"github.com/openorion2/openorion2/internal/engine"
	"github.com/openorion2/openorion2/internal/gfx"
)

// Zoom mode of the galaxy screen. Spec: docs/screens/main-screen-zoom.md.

// mouseListZoomMode is _mouse_list_zoom_mode: cursor 6 over the map, the
// arrow on the frame.
var mouseListZoomMode = []engine.MouseRegion{
	{Image: 6, X1: 0x16, Y1: 0x16, X2: 0x20e, Y2: 0x1a4},
	{Image: 1, X1: 0, Y1: 0, X2: 0x27f, Y2: 0x15},
	{Image: 1, X1: 0, Y1: 0, X2: 0x15, Y2: 0x1df},
	{Image: 1, X1: 0x20f, Y1: 0, X2: 0x27f, Y2: 0x1df},
	{Image: 1, X1: 0, Y1: 0x1a5, X2: 0x27f, Y2: 0x1df},
}

// zoomState holds the zoom box globals.
type zoomState struct {
	zoomBoxX, zoomBoxY int16
	tempBitmap         *gfx.Picture // _temp_bitmap
	zoomBoxColorInc    uint8        // _zoom_box_color_increment, never reset
	zoomBoxCounter     uint8        // _zoom_box_counter, never reset
	zoomBoxIncrement   int8         // _zoom_box_increment_value
}

// enterZoomMode is the entering part of the zoom-in branch (0x8704d): the
// map is captured bright, then darkened behind the box.
func (g *Game) enterZoomMode() error {
	e, m := g.e, &g.ms
	e.DeactivateHelpList()
	m.zoomIn = true
	e.V.SetPaletteChanges(0, 0xff)
	e.UpdateGlassRemapColors()
	e.V.SetRefreshFull()
	e.V.SetPageOff()
	if err := g.drawMiniMainScreen(); err != nil {
		return err
	}
	m.tempBitmap = e.ScreenBitmapCapture(0x16, 0x16, 0x20f, 0x1a5)
	g.darkenFill(0x16, 0x16, 0x20f, 0x1a5)
	e.V.CopyOffToBack()
	e.V.SetRefreshStencil()
	e.AssignAutoFunction(g.drawMainScreenForZooming, 1)
	return nil
}

// darkenFill is _Darken_Fill_ (0x90006).
func (g *Game) darkenFill(x1, y1, x2, y2 int) {
	v := g.e.V
	if v.Clipping {
		x1, y1 = max(x1, v.WinX1), max(y1, v.WinY1)
		x2, y2 = min(x2, v.WinX2), min(y2, v.WinY2)
	}
	g.e.TintFill(x1, y1, x2, y2, 0)
}

// incrementZoomCountWithLimit is Increment_Zoom_Count_With_Limit_: false
// when the box is back to the current view.
func (g *Game) incrementZoomCountWithLimit() bool {
	m := &g.ms
	g.incrementZoomCount()
	if lvl := int16(g.mapScaleToZoomLevel()); lvl <= m.zoomCount {
		m.zoomCount = lvl
		return false
	}
	return true
}

// zoomBoxColors is the box's colour ramp.
var zoomBoxColors = [8]byte{0x0c, 0x0c, 0x10, 0x13, 0x13, 0x10, 0x0c, 0x0c}

// drawMainScreenForZooming is Draw_Main_Screen_For_Zooming_ (0x84bce).
func (g *Game) drawMainScreenForZooming() error {
	e, v, m, s := g.e, g.e.V, &g.ms, &g.state
	seed := s.Random.Get()
	px, py := e.Pointer()
	m.zoomBoxX, m.zoomBoxY = int16(max(px, 0x16)), int16(max(py, 0x16))
	w := int16(int(g.getMapScale(int(m.zoomCount))) * 0x1fa / int(s.CurMapScale))
	h := int16(int(g.getMapScale(int(m.zoomCount))) * 400 / int(s.CurMapScale))
	if m.zoomBoxX+w >= 0x218 {
		m.zoomBoxX = 0x20e - w
	}
	if m.zoomBoxY+h >= 0x1a4 {
		m.zoomBoxY = 0x1a4 - h
	}
	bx, by := int(m.zoomBoxX), int(m.zoomBoxY)
	v.SetWindow(bx, by, bx+int(w), by+int(h))
	v.ClipOn()
	if m.tempBitmap != nil {
		if err := e.Draw(0x16, 0x16, m.tempBitmap); err != nil {
			return err
		}
	}
	g.drawMultiColoredBox(bx, by, bx+int(w), by+int(h), zoomBoxColors, int(m.zoomBoxColorInc))
	m.zoomBoxCounter++
	if m.zoomBoxCounter%3 == 0 {
		m.zoomBoxColorInc += uint8(m.zoomBoxIncrement)
	}
	if m.zoomBoxCounter%12 == 0 {
		m.zoomBoxCounter = 0
		m.zoomBoxIncrement = -1
	}
	v.ClipOff()
	v.ResetWindow()
	s.Random.Set(seed)
	return nil
}

// drawMultiColoredBox is Draw_Multi_Colored_Box_ (0x7a5f7): three nested
// rings of crawling colours.
func (g *Game) drawMultiColoredBox(x1, y1, x2, y2 int, colors [8]byte, st int) {
	v := g.e.V
	s2 := 7 - st
	if s2 <= 0 {
		s2 = 0
	}
	var rev [8]byte
	for i := range colors {
		rev[7-i] = colors[i]
	}
	i16 := func(n int) int { return int(int16(n)) }
	line := func(ax, ay, bx, by int, c [8]byte, rot int) {
		v.MultiColoredLine(i16(ax), i16(ay), i16(bx), i16(by), c, rot)
	}
	line(x1, y1, x2, y1, colors, st)
	line(x1+1, y1+1, x2-1, y1+1, colors, s2)
	line(x1+2, y1+2, x2-2, y1+2, colors, st)
	line(x2, y1, x2, y2, colors, st)
	line(x2-1, y1+1, x2-1, y2-1, colors, s2)
	line(x2-2, y1+2, x2-2, y2-2, colors, st)
	line(x1, y2, x2, y2, rev, st)
	line(x1+1, y2-1, x2-1, y2-1, rev, s2)
	line(x1+2, y2-2, x2-2, y2-2, rev, st)
	line(x1, y1, x1, y2, rev, st)
	line(x1+1, y1+1, x1+1, y2-1, rev, s2)
	line(x1+2, y1+2, x1+2, y2-2, rev, st)
}

// saveAndRemoveBoxes is Save_And_Remove_Boxes_ (0x919a4).
func (g *Game) saveAndRemoveBoxes() {
	m, s := &g.ms, &g.state
	for b := 0; b < numBoxes; b++ {
		bx := &s.MoveableBoxes[b]
		open := bx.Content > m.boxOffFlag[b]
		m.savedBoxes[b], m.savedBoxWasOpen[b] = bx.Content, open
		switch {
		case b == 0 && open:
			g.vanishSystemDisplayBox()
		case b == 2 && open:
			g.vanishFleetMovementBox()
		}
		g.mm.starClick = -1
		bx.Content = m.boxOffFlag[int(bx.ID)%numBoxes]
	}
}
