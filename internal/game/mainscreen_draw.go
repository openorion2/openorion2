package game

import (
	"github.com/openorion2/openorion2/internal/save"
)

// Drawing the galaxy screen. Spec: docs/screens/main-screen-background.md.

// drawMainScreenFilled is Draw_Main_Screen_Filled_ (0x84a95): the whole
// screen from scratch, copied to the back page when toBack.
func (g *Game) drawMainScreenFilled(toBack bool) error {
	e, v := g.e, g.e.V
	v.ResetWindow()
	v.ClipOff()
	v.Fill(0, 0, 0x27f, 0x1df, 0)
	v.SetWindow(0, 0, 0x27f, 0x1df)
	v.ClipOn()
	if err := g.drawParalax(); err != nil {
		return err
	}
	frame, err := g.cached(buffer0LBX, 0)
	if err != nil {
		return err
	}
	if err := e.Draw(0, 0, frame); err != nil {
		return err
	}
	g.drawWormholeLinks()
	g.drawRelocationLinks()
	gt := g.gameType
	if gt == save.GameNetwork || gt == save.GameModem {
		p, err := g.cached(buffer0LBX, 0xd)
		if err != nil {
			return err
		}
		if err := e.Draw(0x19f, 0, p); err != nil {
			return err
		}
	}
	if gt == save.GameNetwork || gt == save.GameModem || gt == save.GameHotseat {
		p, err := g.cached(buffer0LBX, 0xd)
		if err != nil {
			return err
		}
		if err := e.Draw(0x4a, 0, p); err != nil {
			return err
		}
	}
	v.ClipOff()
	v.ResetWindow()
	g.mapWindow()
	if err := g.drawNebulae(); err != nil {
		return err
	}
	v.ClipOff()
	v.ResetWindow()
	if toBack {
		v.CopyOffToBack()
	}
	if err := g.printStarNames(); err != nil {
		return err
	}
	return g.printMainScreenData()
}

// drawParalax is Draw_Paralax_ (0x8500f): three starfield layers, each in
// four copies that tile the map window.
func (g *Game) drawParalax() error {
	m := &g.ms
	ws := g.saveWindowStatus()
	g.mapWindow()
	layers := []struct{ x, y int16 }{{m.farX, m.farY}, {m.midX, m.midY}, {m.nearX, m.nearY}}
	for i, l := range layers {
		p, err := g.cached("STARBG.LBX", i)
		if err != nil {
			return err
		}
		for _, d := range [4][2]int16{{0, 0}, {-0x280, 0}, {-0x280, -0x1e0}, {0, -0x1e0}} {
			if err := g.e.Draw(int(l.x+d[0]), int(l.y+d[1]), p); err != nil {
				return err
			}
		}
	}
	g.restoreWindowStatus(ws)
	return nil
}

// drawNebulae is Draw_Nebulae_ (0x84f8f).
func (g *Game) drawNebulae() error {
	s := &g.state
	for n := 0; n < int(s.NumNebulas); n++ {
		z := g.mapScaleToZoomLevel()
		x := int16(g.scaled(s.Nebulas[n].X-s.CurMapX) + 0x15)
		y := int16(g.scaled(s.Nebulas[n].Y-s.CurMapY) + 0x15)
		p := g.ms.nebulaPict[n][z]
		if p == nil {
			continue
		}
		if err := g.e.Draw(int(x), int(y), p); err != nil {
			return err
		}
	}
	return nil
}

// drawMiniMainScreen is Draw_Mini_Main_Screen_ (0x84e9d): the map without
// the boxes, used behind popups and while fading in.
func (g *Game) drawMiniMainScreen() error {
	m, s := &g.ms, &g.state
	if m.atLeastOneTacticalCombat {
		m.atLeastOneTacticalCombat = false
		if err := g.fadeIntoMainScreen(); err != nil {
			return err
		}
	}
	zc, scale, my, mx := m.zoomCount, s.CurMapScale, s.CurMapY, s.CurMapX
	if !m.dontDrawToBack {
		if err := g.drawMainScreenFilled(true); err != nil {
			return err
		}
	} else {
		if err := g.drawMainScreenFilled(false); err != nil {
			return err
		}
		m.dontDrawToBack = false
	}
	if err := g.drawMainMainScreen(); err != nil {
		return err
	}
	m.multiColoredLineStart = (m.multiColoredLineStart + 1) % 8
	s.CurMapX, s.CurMapY, s.CurMapScale, m.zoomCount = mx, my, scale, zc
	if g.settings[0x01] == 0 && m.checkNextTurnExitField && g.e.MouseButton() != 0 {
		m.autoTurnExit = true
	}
	if m.checkNextTurnExitField {
		if err := g.e.Draw(0x220, 0x1b9, m.darkenedTurnButton); err != nil {
			return err
		}
	}
	if g.fullDrawScreen {
		m.skipFields = true
		g.fullDrawScreen = false
	}
	return nil
}

// drawMainMainScreen is Draw_Main_Main_Screen_ (0x8440e).
func (g *Game) drawMainMainScreen() error {
	e, v := g.e, g.e.V
	g.mapWindow()
	g.drawWormholeLinks()
	e.MoxUpdate()
	g.mapWindow()
	g.drawRelocationLinks()
	e.MoxUpdate()
	g.mapWindow()
	if err := g.drawStars(); err != nil {
		return err
	}
	e.MoxUpdate()
	g.mapWindow()
	for i := 0; i < int(g.state.NumStars); i++ {
		if err := g.drawAGateIcon(i); err != nil {
			return err
		}
	}
	e.MoxUpdate()
	g.mapWindow()
	if err := g.printStarNames(); err != nil {
		return err
	}
	e.MoxUpdate()
	g.mapWindow()
	if err := g.drawBlackHoles(); err != nil {
		return err
	}
	e.MoxUpdate()
	g.mapWindow()
	if err := g.drawShipIcons(true); err != nil {
		return err
	}
	e.MoxUpdate()
	v.ResetWindow()
	v.ClipOff()
	if err := g.printMainScreenData(); err != nil {
		return err
	}
	e.MoxUpdate()
	if g.gameType != save.GameSingle {
		if err := g.drawDiplomacyRequestLights(); err != nil {
			return err
		}
		e.MoxUpdate()
	}
	return nil
}

// drawMainScreen is Draw_Main_Screen_ (0x84555), the auto function of the
// galaxy screen.
func (g *Game) drawMainScreen() error {
	e, v, m, s := g.e, g.e.V, &g.ms, &g.state
	if m.stencilBoxID < 0 || m.stencilBoxID > 4 {
		g.mapWindow()
		if m.drawToBack || g.fullDrawScreen {
			if err := g.drawMainScreenFilled(true); err != nil {
				return err
			}
			m.drawToBack = false
		} else {
			g.drawWormholeLinks()
			g.drawRelocationLinks()
			if err := g.printStarNames(); err != nil {
				return err
			}
		}
		g.mapWindow()
		if g.settings[0x01] != 0 {
			if err := g.printStardateCorner(); err != nil {
				return err
			}
		}
		if err := g.drawBlackHoles(); err != nil {
			return err
		}
		for i := 0; i < int(s.NumStars); i++ {
			if err := g.drawAGateIcon(i); err != nil {
				return err
			}
		}
		g.doShipDestinationLines()
		if m.etaStarID >= 0 && m.fleetShipScanned == -1 {
			g.drawETADestinationLine()
			if s.Stars[m.etaStarID].SpectralClass != save.SpectralBlackHole {
				if err := g.printHighlightedStarName(int(m.etaStarID)); err != nil {
					return err
				}
				g.pulseSelectedStarColor()
			}
		}
		if m.mainScreenScannedStar != -1 && s.Stars[m.mainScreenScannedStar].SpectralClass != save.SpectralBlackHole {
			if err := g.printHighlightedStarName(int(m.mainScreenScannedStar)); err != nil {
				return err
			}
			g.pulseSelectedStarColor()
		}
		if err := g.drawStars(); err != nil {
			return err
		}
		if err := g.drawShipIcons(false); err != nil {
			return err
		}
		if err := g.fleetBoxPaint(); err != nil {
			return err
		}
		for i := 4; i >= 0; i-- {
			b := int(s.MoveableBoxOrder[i])
			if g.moveableBoxSelected(b) && b == 0 {
				c := s.MoveableBoxes[0].Content
				if s.Stars[c].SpectralClass != save.SpectralBlackHole {
					if err := g.printHighlightedStarName(int(c)); err != nil {
						return err
					}
					g.pulseSelectedStarColor()
				}
			}
			if g.moveableBoxSelected(b) && b == 2 {
				if err := g.drawFleetBoxSelectedShipSeg(); err != nil {
					return err
				}
			}
		}
		if m.mainScreenShipIconScanned >= 0 {
			g.drawScannedShipSeg()
		}
		for i := 4; i >= 0; i-- {
			b := int(s.MoveableBoxOrder[i])
			if g.moveableBoxSelected(b) && int16(b) != m.stencilBoxID {
				if g.e.MouseButton() != 0 {
					bx := &s.MoveableBoxes[b]
					if g.e.AutoInput() == int(bx.DragField) || bx.Dragging == 1 {
						g.moveMoveableBox(b, g.e.MouseButton())
					}
				}
				if err := g.drawMoveableBox(b); err != nil {
					return err
				}
			}
		}
		if m.galaxyMapScannedStar >= 0 && m.galaxyMapScannedStar < s.NumStars {
			if err := g.printGalaxyMapScannedStar(); err != nil {
				return err
			}
		}
		v.ClipOff()
		v.ResetWindow()
		if err := g.printMainScreenData(); err != nil {
			return err
		}
		m.multiColoredLineStart = (m.multiColoredLineStart + 1) % 8
	} else {
		if err := g.drawMainScreenWhileMovingBoxes(); err != nil {
			return err
		}
		v.ClipOff()
		v.ResetWindow()
	}
	if g.gameType == save.GameNetwork || g.gameType == save.GameModem {
		if err := g.drawDiplomacyRequestLights(); err != nil {
			return err
		}
	}
	if g.gameType != save.GameSingle {
		if err := g.drawPlayerDoneLights(); err != nil {
			return err
		}
	}
	g.checkMouseXYFields()
	if g.fullDrawScreen {
		m.skipFields = true
		g.fullDrawScreen = false
	}
	m.redPulseValue += m.redPulseIncr
	if m.redPulseValue == 0 || m.redPulseValue == 8 {
		m.redPulseIncr = -m.redPulseIncr
	}
	_ = e
	return nil
}

// pulseSelectedStarColor steps _selected_star_color_offset after each
// highlighted name: 0, 1, 2, 3, 2, 1, 0, -1, 0, ...
func (g *Game) pulseSelectedStarColor() {
	m := &g.ms
	m.selectedStarColorOffset += m.selectedStarColorMod
	if m.selectedStarColorOffset == 3 || m.selectedStarColorOffset == -1 {
		m.selectedStarColorMod = -m.selectedStarColorMod
	}
}

// drawMoveableBox is Draw_Moveable_Box_ (0x91099).
func (g *Game) drawMoveableBox(b int) error {
	if !g.moveableBoxSelected(b) {
		return nil
	}
	if g.sd.skipDisplay {
		g.sd.skipDisplay = false
		return nil
	}
	switch b {
	case 0:
		return g.drawSystemDisplayBox()
	case 2:
		return g.drawFleetMovementBox()
	case 1:
		return g.drawGalaxyMapBox1()
	}
	return nil
}
