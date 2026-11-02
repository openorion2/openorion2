package game

// Dragging the moveable boxes. Spec: docs/screens/system-display.md,
// "Closing, dragging, top".

// moveMoveableBox is Move_Moveable_Box_ (0x91349): while a button is held
// on the box's title bar the drag offset follows the pointer; on release
// the box moves there.
func (g *Game) moveMoveableBox(id, button int) {
	e, m, s := g.e, &g.ms, &g.state
	b := &s.MoveableBoxes[id]
	var title int16
	switch id {
	case 2:
		title = 0x1c
		g.fb.buttonsSet = m.mouseButtonPressed == 2
	case 0:
		title = 0x2b
	}
	var w, h int16
	switch {
	case id == 2:
		h, w = g.fb.fullH, g.fb.width
	case m.boxOffFlag[id] < b.Content:
		w, h = b.Width, b.Height
	default:
		w, h = b.ClosedWidth, b.ClosedHeight
	}
	if button != 0 && (e.AutoInput() == int(b.DragField) || b.Dragging == 1) {
		e.DeactivateHelpList()
		m.mouseButtonPressed = button
		e.DisableMousePressedFieldUpdate()
		e.DisableDrawAllFields()
		m.skipFields = true
		if m.stencilBoxID == -1 {
			m.drawMainScreenStencilled = true
			m.stencilBoxID = int16(id)
		}
		px, py := e.Pointer()
		qx, qy := e.PressedMouse()
		b.DragDX, b.DragDY = int16(px-qx), int16(py-qy)
		if int(b.X)+int(b.DragDX) < 0x16 {
			b.DragDX = 0x16 - b.X
		}
		if int(0x20e-w) < int(b.X)+int(b.DragDX) {
			b.DragDX = 0x20f - w - b.X
		}
		if int(b.DragDY)+int(b.Y) < 0x16 {
			b.DragDY = 0x16 - b.Y
		}
		if button == 2 && id == 0 {
			if int(title)+int(b.DragDY)+int(b.Y) >= 0x1a5 {
				b.DragDY = 0x1a5 - b.Y - title
			}
		} else if int(b.DragDY)+int(b.Y) > int(0x1a4-h) {
			b.DragDY = 0x1a5 - h - b.Y
		}
		b.Dragging = 1
		return
	}
	m.drawToBack = true
	e.EnableDrawAllFields()
	g.setMainScreenHelpList()
	e.ClearFields()
	g.clearMoveableBoxFields(id)
	m.skipFields = true
	if b.Dragging == 1 {
		px, py := e.Pointer()
		qx, qy := e.PressedMouse()
		b.X += int16(px - qx)
		m.stencilBoxID = -2
		b.Y += int16(py - qy)
	}
	if b.X < 0x16 {
		b.X = 0x16
	}
	if int(0x20e-w) < int(b.X) {
		b.X = 0x20f - w
	}
	if b.Y < 0x16 {
		b.Y = 0x16
	}
	bottom := h
	if m.mouseButtonPressed == 2 && id == 0 {
		bottom = title
	}
	if int(b.Y) > int(0x1a4-bottom) {
		b.Y = 0x1a5 - bottom
	}
	b.DragDX, b.DragDY, b.Dragging = 0, 0, 0
	g.moveBoxToTop(id)
	if id == 2 {
		g.fb.x, g.fb.y = b.X, b.Y
	}
	m.drawToBack = true
	g.setMainScreenHelpList()
}

// drawMainScreenWhileMovingBoxes is Draw_Main_Screen_While_Moving_Boxes_
// (0x84db2).
func (g *Game) drawMainScreenWhileMovingBoxes() error {
	m, s := &g.ms, &g.state
	if m.drawMainScreenStencilled {
		err := g.drawMainScreenBeforeStencil()
		m.drawMainScreenStencilled = false
		return err
	}
	g.mapWindow()
	for i := 4; i >= 0; i-- {
		b := int(s.MoveableBoxOrder[i])
		if g.moveableBoxSelected(b) && int16(b) != m.stencilBoxID {
			if err := g.drawMoveableBox(b); err != nil {
				return err
			}
		}
	}
	if m.stencilBoxID >= 0 && m.stencilBoxID < 5 {
		g.moveMoveableBox(int(m.stencilBoxID), g.e.MouseButton())
	}
	if m.stencilBoxID >= 0 && m.stencilBoxID < 5 {
		if err := g.drawMoveableBox(int(m.stencilBoxID)); err != nil {
			return err
		}
	}
	g.e.V.ClipOff()
	g.e.V.ResetWindow()
	if err := g.printMainScreenData(); err != nil {
		return err
	}
	if m.stencilBoxID == -2 {
		g.e.V.SetPageOff()
		if err := g.drawMainScreen(); err != nil {
			return err
		}
		m.stencilBoxID = -1
	}
	return nil
}

// drawMainScreenBeforeStencil is Draw_Main_Screen_Before_Stencil_ (0x849cc):
// the map without the moving box into both pages.
func (g *Game) drawMainScreenBeforeStencil() error {
	e, m, s := g.e, &g.ms, &g.state
	g.mapWindow()
	if err := g.drawMainMainScreen(); err != nil {
		return err
	}
	e.V.CopyOffToBack()
	for i := 4; i >= 0; i-- {
		b := int(s.MoveableBoxOrder[i])
		if g.moveableBoxSelected(b) && int16(b) != m.stencilBoxID {
			g.moveMoveableBox(b, e.MouseButton())
			if err := g.drawMoveableBox(b); err != nil {
				return err
			}
		}
	}
	m.nextButton.CurrentFrame = 0
	if err := e.Draw(0x220, 0x1b9, m.nextButton); err != nil {
		return err
	}
	m.gameButton.CurrentFrame = 0
	if err := e.Draw(0xf9, 5, m.gameButton); err != nil {
		return err
	}
	return g.drawMoveableBox(int(m.stencilBoxID))
}
