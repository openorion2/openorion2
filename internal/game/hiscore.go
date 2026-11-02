package game

import (
	"bytes"
	"encoding/binary"

	"github.com/openorion2/openorion2/internal/clib"
	"github.com/openorion2/openorion2/internal/engine"
	"github.com/openorion2/openorion2/internal/font"
	"github.com/openorion2/openorion2/internal/save"
)

// The score screen at the end of a game and the entry into the Hall of
// Fame (HISCORE.C). Spec: docs/screens/endgame.md.

const raceiconLBXName = raceiconLBX

// endOfGameHiScore is End_Of_Game_Hi_Score_ (0x9f712): p's score screen,
// then p enters the Hall of Fame if the score beats an entry (not after
// cheating), and the Hall of Fame shows; the game is over.
func (g *Game) endOfGameHiScore(p int) error {
	hf, s := &g.hof, &g.state
	score, err := g.hiScoreScreen(p)
	if err != nil {
		return err
	}
	g.e.AssignAutoFunction(func() error { return nil }, 1)
	cheated := false
	var best int16
	for k := 0; k < 10; k++ {
		if v := hf.i16(hofScores + 2*k); best < v {
			best = v
		}
	}
	var sorted [10][2]int16
	g.sortHOFScores(&sorted)
	rank := int16(-1)
	for k := 0; k < 10; k++ {
		if score >= sorted[k][0] {
			rank = sorted[k][1]
		}
	}
	pl := &s.Players[p]
	if rank > -1 {
		var msg string
		switch {
		case pl.Cheated != 0:
			msg, cheated = g.lbxMessage(0x17), true
		case best <= score:
			msg = cSprintf(g.hMessage(0x27), save.CString(pl.EmperorName[:]))
		default:
			msg = cSprintf(g.hMessage(0x28), save.CString(pl.EmperorName[:]))
		}
		if err := g.warningBox(msg); err != nil {
			return err
		}
		if cheated {
			g.e.SetMouseList(engine.MouseArrow)
		}
		if !cheated {
			f := hf.file[:]
			for k := 0; k < int(rank); k++ {
				strcpyTo(f[hofNames+20*k:hofNames+20*k+20], f[hofNames+20*(k+1):hofNames+20*(k+1)+20])
				hf.set16(hofScores+2*k, hf.i16(hofScores+2*(k+1)))
				hf.set16(hofRaces+2*k, hf.i16(hofRaces+2*(k+1)))
				f[hofDiff+k] = f[hofDiff+k+1]
				strcpyTo(f[hofRaceNames+20*k:hofRaceNames+20*k+20], f[hofRaceNames+20*(k+1):hofRaceNames+20*(k+1)+20])
			}
			r := int(rank)
			strcpyTo(f[hofNames+20*r:hofNames+20*r+20], pl.EmperorName[:])
			hf.set16(hofScores+2*r, score)
			hf.set16(hofRaces+2*r, int16(pl.Race))
			f[hofDiff+r] = g.settings[0xd4]
			strcpyTo(f[hofRaceNames+20*r:hofRaceNames+20*r+20], pl.RaceName[:])
		}
	}
	if rank > -1 && !cheated {
		g.writeHOFFile()
	}
	if err := g.hallOfFameScreen(rank); err != nil {
		return err
	}
	g.currentScreen = ScreenMainMenu
	g.previousScreen = ScreenHallOfFame
	return nil
}

// hiScoreScreen is Hi_Score_Screen_ (0x9ea3b): p's score, line by line,
// until Esc or a click; returns the final score.
func (g *Game) hiScoreScreen(p int) (int16, error) {
	e, hf := g.e, &g.hof
	e.DeactivateHelpList()
	if !g.readHOFFile() {
		g.createDefaultHOFFile()
	}
	e.AssignAutoFunction(g.drawHiScoreScreen, 1)
	e.ClearFields()
	if err := g.loadHiScoreScreen(); err != nil {
		return 0, err
	}
	hf.h.modifierPlayer = uint8(p)
	if err := g.calculateHiScores(p, &hf.h); err != nil {
		return 0, err
	}
	hf.hoverRow = -1
	g.getHiScoreColors()
	var err error
	if hf.escField, err = e.AddHiddenField(0, 0, 0x27f, 0x1df, cStringAt(g.exe, hofEscKey), 0x29); err != nil {
		return 0, err
	}
	seed := g.state.Random.Get()
	g.setHiScorePopIconIds(p)
	g.state.Random.Set(seed)
	g.buildCapturedColonistsArray(p)
	// Fade_Into_Screen_'s flag is +0x6b, its palette EBX = 0xc left over.
	hf.flag6b = true
	if err := g.fadeIntoScreen(g.drawHiScoreScreen, 0xc); err != nil {
		return 0, err
	}
	e.UpdateGlassRemapColors()
	hf.showing = 1
	for {
		r, err := e.GetInput()
		if err != nil {
			return 0, err
		}
		if r == hf.escField {
			break
		}
		e.V.SetPageOff()
		if err := g.drawHiScoreScreen(); err != nil {
			return 0, err
		}
		if err := e.TogglePages(); err != nil {
			return 0, err
		}
	}
	hf.showing = 0
	e.V.SetPageOff()
	if err := g.drawHiScoreScreen(); err != nil {
		return 0, err
	}
	e.V.CopyOffToBack()
	e.V.SetRefreshStencil()
	g.currentScreen = g.returnScreen
	e.ClearFields()
	e.DeactivateAutoFunction()
	return hf.h.final, nil
}

// loadHiScoreScreen is Load_Hi_Score_Screen_ (0x9eb42).
func (g *Game) loadHiScoreScreen() error {
	hf := &g.hof
	hf.clearBlock()
	file := cStringAt(g.exe, scoreLBX)
	var err error
	if hf.background, err = g.picture(file, 0); err != nil {
		return err
	}
	if hf.elimFrame, err = g.picture(file, 1); err != nil {
		return err
	}
	for k := range hf.racePics {
		if hf.racePics[k], err = g.picture(file, 2+k); err != nil {
			return err
		}
	}
	return nil
}

// getHiScoreColors is Get_Hi_Score_Colors_ (0x9e7c0).
func (g *Game) getHiScoreColors() {
	hf := &g.hof
	hf.colors = g.fontColors2(4, 0xd0, 0xd2)
	hf.colors80 = g.fontColors2(4, 0xd0, 0xd2)
	g.moxFontColorsOffset = 0xf3
	hf.colors78 = g.fontColors2(4, 0xd0, 0xd2)
}

// setHiScorePopIconIds is Set_Hi_Score_Pop_Icon_Ids_ (0x9fcf0): up to 16
// of p's colonists, each race by its share, picked from a shuffled list.
func (g *Game) setHiScorePopIconIds(p int) {
	s, hf := &g.state, &g.hof
	var total int16
	for ci := 0; ci < int(s.NumColonies); ci++ {
		if int(s.Colonies[ci].Owner) == p {
			total += int16(s.Colonies[ci].PopCount)
		}
	}
	type entry struct {
		colony int16
		race   uint8
		k      uint8
		used   uint8
	}
	var counts [14]int16
	list := make([]entry, max(int(total), 0))
	n := int16(0)
	full := false
	for ci := 0; ci < int(s.NumColonies) && !full; ci++ {
		c := &s.Colonies[ci]
		if int(c.Owner) != p {
			continue
		}
		for k := 0; k < int(c.PopCount) && !full; k++ {
			race := g.playerRaceAnyIndex(int(g.colonist(ci, k) & 0xf))
			if race >= 0xe {
				continue
			}
			if n == total {
				full = true
				continue
			}
			counts[race]++
			list[n] = entry{colony: int16(ci), race: race, k: uint8(k)}
			n++
		}
	}
	for k := int16(0); k < total; k++ {
		a := int16(s.Random.Random(uint32(total))) - 1
		b := int16(s.Random.Random(uint32(total))) - 1
		list[a], list[b] = list[b], list[a]
	}
	for r := 0; r < 0xe; r++ {
		if counts[r] <= 0 {
			continue
		}
		m := (int32(counts[r])*0xa0/int32(total) + 5) / 10
		if int32(hf.popCount)+m > 0x10 && m > 1 {
			m = 0x10 - int32(hf.popCount)
		}
		for j := int32(0); j < m && hf.popCount < 0x10; j++ {
			// Get_From_Tmp_Pop_ (0x9ff36): the first unused colonist of race r.
			for i := range list {
				if int(list[i].race) == r && list[i].used == 0 {
					hf.popIcons[hf.popCount] = [2]int16{list[i].colony, int16(list[i].k)}
					list[i].used = 1
					break
				}
			}
			hf.popCount++
		}
	}
}

// buildCapturedColonistsArray is Build_Captured_Colonists_Array_ (0x9d816):
// the races of up to 16 captured colonists, drawn by weight (2 for a war
// declared, 5 for a race eliminated), sorted; the seed is put back.
func (g *Game) buildCapturedColonistsArray(p int) {
	s, hf := &g.state, &g.hof
	seed := s.Random.Get()
	var w [8]uint16
	pl := &s.Players[p]
	for k := 0; k < 8; k++ {
		if k != p && pl.WarDeclared[k] == 1 {
			w[k] = 2
		}
		if pl.Eliminations[k] != 0 {
			w[k] = 5
		}
	}
	hf.capCount = min(hf.h.capturedPop, 0x10)
	n := int16(0)
	for i := int16(0); i < hf.capCount; i++ {
		c := getWeightedChoiceShort(&s.Random, w[:])
		if c > -1 && c < s.NumPlayers && int(c) != p {
			n++
			hf.capRaces[i] = s.Players[c].Race
		}
	}
	clib.Qsort(hf.capRaces[:], 1, func(a, b *uint8) int { return int(*a) - int(*b) })
	hf.capCount = n
	s.Random.Set(seed)
}

// drawHiScoreScreen is Draw_Hi_Score_Screen_ (0x9e27a).
func (g *Game) drawHiScoreScreen() error {
	e, hf := g.e, &g.hof
	if hf.flag6b {
		e.V.Fill(0, 0, 0x27f, 0x1df, 0)
		hf.flag6b = false
	}
	if err := e.Draw(0, 0, hf.background); err != nil {
		return err
	}
	if err := g.drawTimeScore(); err != nil {
		return err
	}
	h := &hf.h
	if err := g.drawGenericScore(h.popScore, cSprintf(g.hMessage(0x15c), h.pop), func() error {
		return g.drawPop(false, 4, hf.popCount)
	}); err != nil {
		return err
	}
	if err := g.drawEliminationScore(); err != nil {
		return err
	}
	f := g.hMessage(0x155)
	if h.capturedPop == 1 {
		f = g.hMessage(0x154)
	}
	if err := g.drawGenericScore(h.captured, cSprintf(f, h.capturedPop), func() error {
		return g.drawPop(true, 2, hf.capCount)
	}); err != nil {
		return err
	}
	for _, l := range []struct {
		v   int16
		msg int
	}{{h.tech, 0x15d}, {h.orion, 0x15b}, {h.antares, 0x153}} {
		if err := g.drawGenericScore(l.v, g.hMessage(l.msg), nil); err != nil {
			return err
		}
	}
	if h.council > 0 {
		if err := g.drawGenericScore(h.council, g.hMessage(0x156), nil); err != nil {
			return err
		}
	}
	return g.drawTotalScore()
}

// drawTimeScore is Draw_Time_Score_ (0x9e68b): the first line, at 0x8b.
func (g *Game) drawTimeScore() error {
	hf := &g.hof
	t := g.e.Text()
	t.SetStyle(4, hf.colors)
	t.SetSpacing(1)
	h := t.Height()
	msg := cSprintf(g.hMessage(0x15e), int16(uint16(g.state.Stardate)-35000))
	if err := g.printScoreString(0x8c, 0x8b, hf.h.time, msg); err != nil {
		return err
	}
	hf.y = int16(h + 0x92)
	return nil
}

// drawGenericScore is Draw_Generic_Score_ (0x9e207): a score line, then
// what goes under it.
func (g *Game) drawGenericScore(v int16, msg string, under func() error) error {
	hf := &g.hof
	t := g.e.Text()
	y := hf.y
	t.SetStyle(4, hf.colors)
	t.SetSpacing(1)
	h := int16(t.Height())
	if err := g.printScoreString(0x8c, int(y), v, msg); err != nil {
		return err
	}
	hf.y = y + h + 2
	if under != nil {
		return under()
	}
	return nil
}

// printScoreString is Print_Score_String_ (0x9ec4a): the text, a dotted
// leader and the number right-aligned at 0x1f3.
func (g *Game) printScoreString(x, y int, v int16, msg string) error {
	hf := &g.hof
	t := g.e.Text()
	dots := g.fontColors2(4, 0xd0, 0xd1)
	dot := cStringAt(g.exe, scoreDot)
	dw := t.Width([]byte(dot))
	tw := t.Width([]byte(msg))
	num := itoa10(int32(v))
	nw := t.Width([]byte(num)) + 3
	y += 4
	t.SetStyle(4, hf.colors)
	t.SetSpacing(1)
	if err := t.Print(x+4, y, msg); err != nil {
		return err
	}
	t.SetStyle(4, dots)
	t.SetSpacing(1)
	for dx := int16(x + 4 + tw + dw); int32(dx) < int32(0x1f3-int16(nw)); dx += int16(dw) {
		if err := t.Print(int(dx), y, dot); err != nil {
			return err
		}
	}
	t.SetStyle(4, hf.colors)
	t.SetSpacing(1)
	return t.PrintRight(0x1f3, y, num)
}

// drawEliminationScore is Draw_Elimination_Score_ (0x9e17c).
func (g *Game) drawEliminationScore() error {
	h := &g.hof.h
	var msg string
	switch h.eliminated {
	case 0:
		msg = g.hMessage(0x158)
	case 1:
		msg = cSprintf(g.hMessage(0x159), h.eliminated)
	default:
		msg = cSprintf(g.hMessage(0x15a), h.eliminated)
	}
	return g.drawGenericScore(h.elimination, msg, g.drawEliminatedRacePictures)
}

// drawEliminatedRacePictures is Draw_Eliminated_Race_Pictures_ (0x9def7):
// a portrait per race the player eliminated, with the date and the
// emperor's name.
func (g *Game) drawEliminatedRacePictures() error {
	e, hf, s := g.e, &g.hof, &g.state
	t, v := e.Text(), e.V
	pl := &s.Players[hf.h.modifierPlayer]
	x := 0x8e
	y0 := int(hf.y)
	any := false
	var ph int
	for k := 0; k < int(s.NumPlayers); k++ {
		el := pl.Eliminations[k]
		if el == 0 {
			continue
		}
		pic := hf.racePics[s.Players[k].Race]
		pw := pic.Width
		ph = pic.Height
		if err := e.Draw(x, y0+2, pic); err != nil {
			return err
		}
		bottom := y0 + 2 + ph
		v.SetWindow(x, y0+2, x+pw, bottom)
		v.ClipOn()
		t.SetStyleShadow(1, hf.colors, font.Outline)
		t.SetOutlineColor(2, 1)
		t.SetSpacing(1)
		u := uint16(el)
		cx := int(int16(pw)/2) + x
		if err := t.PrintCentered(cx, y0+3, cSprintf(cStringAt(g.exe, scoreFmtDec), u/10, u%10)); err != nil {
			return err
		}
		t.SetStyleShadow(1, hf.colors, font.Outline)
		t.SetOutlineColor(2, 1)
		any = true
		name := save.CString(s.Players[k].EmperorName[:])
		if pw < t.Width([]byte(name)) {
			if err := t.Print(x+1, bottom-t.Height()-1, name); err != nil {
				return err
			}
		} else if err := t.PrintCentered(cx, bottom-t.Height()-1, name); err != nil {
			return err
		}
		v.ResetWindow()
		v.ClipOff()
		if err := e.Draw(x-1, y0+1, hf.elimFrame); err != nil {
			return err
		}
		e.Box(x-2, y0, pw+3, ph+3, 1, 0x4b)
		e.Box(x-1, y0+1, pw+1, ph+1, 1, 0xf)
		x += pw + 4
	}
	if any {
		hf.y += int16(ph + 4)
	}
	return nil
}

// drawPop is Draw_Pop_ (0x9e3d1): count colonist icons (conquered races
// with captured) in rows from 0x8c, closer together the more there are.
func (g *Game) drawPop(captured bool, n int, count int16) error {
	hf := &g.hof
	y := int(hf.y)
	cpic, err := g.cAnims(0xd)
	if err != nil {
		return err
	}
	if err := g.e.CalculatePictureRemapColors(cpic); err != nil {
		return err
	}
	cnt := int(count)
	var step int
	switch {
	case cnt < n*0x30:
		step = 0xc
	case cnt < n*0x3a:
		step = 0xa
	case cnt < n*0x48:
		step = 8
	case cnt < n*0x61:
		step = 6
	default:
		step = 4
	}
	drawn, col, dx, dy := 0, 0, 0, 0
	for k := 0; k < cnt; k++ {
		if k >= n*0x61 {
			continue
		}
		var err error
		var pic = cpic
		if captured {
			// Race_Conquered_Anim_ (0xbb53e).
			pic, err = g.cached(cStringAt(g.exe, raceiconLBXName), int(hf.capRaces[k])*0xd+0xc)
		} else {
			pic, err = g.colonyPopAnim(int(hf.popIcons[k][0]), int(hf.popIcons[k][1]))
		}
		if err != nil {
			return err
		}
		if cnt <= n*0x1b {
			dx = (drawn % 0x19) * 0x16
			dy = (drawn / 0x19) * 0x16
			if err := g.remapDraw(0x8c+dx, int(int16(y+dy)), pic); err != nil {
				return err
			}
		} else {
			if col%2 == 0 {
				if err := g.remapDraw(0x8c+dx, y, pic); err != nil {
					return err
				}
				dy = 0
			} else {
				if err := g.remapDraw(0x8c+dx, int(int16(y+step)), pic); err != nil {
					return err
				}
				dy = step
			}
			dx += step
			if int(int16(dx))+0x18 > 0x168 {
				col = 0
				y += 0x1e
				dx, dy = 0, 0
			}
		}
		drawn++
		col++
	}
	if count > 0 {
		hf.y = int16(y + dy + 0x1c)
	} else {
		hf.y += 3
	}
	return nil
}

// drawTotalScore is Draw_Total_Score_ (0x9dc76): a triple rule, then the
// total (with the race modifier when it applies) after a dotted leader.
func (g *Game) drawTotalScore() error {
	e, hf := g.e, &g.hof
	t := e.Text()
	h := &hf.h
	y := int(hf.y) + 6
	t.SetStyle(4, hf.colors)
	t.SetSpacing(1)
	label := g.hMessage(0x157)
	lw := t.Width([]byte(label))
	var num string
	if h.final == h.total {
		num = itoa10(int32(h.final))
	} else {
		var m string
		if h.modifier < 100 {
			m = cSprintf(cStringAt(g.exe, scoreFmtFrac), h.modifier)
		} else {
			m = cSprintf(cStringAt(g.exe, scoreFmtDec), h.modifier/100, h.modifier%100)
		}
		num = cSprintf(cStringAt(g.exe, scoreFmtTimes), h.total, m, h.final)
	}
	nw := t.Width([]byte(num)) + 2
	x1 := int(int16(500 - nw))
	e.V.Line(x1, y, 0x1f2, y, 0xd1)
	e.V.Line(x1, y+1, 0x1f2, y+1, 0xd2)
	e.V.Line(x1, y+2, 0x1f2, y+2, 0xd1)
	dots := g.fontColors2(4, 0xd0, 0xd1)
	dot := cStringAt(g.exe, scoreDot)
	dw := t.Width([]byte(dot))
	t.SetStyle(4, hf.colors)
	t.SetSpacing(1)
	if err := t.Print(0x8c+4, y+4, label); err != nil {
		return err
	}
	t.SetStyle(4, dots)
	t.SetSpacing(1)
	x := int16(0x8c + 4 + lw + dw)
	for ; int32(x) < int32(int16(500-nw)); x += int16(dw) {
		if err := t.Print(int(x), y+4, dot); err != nil {
			return err
		}
	}
	t.SetStyle(4, hf.colors)
	t.SetSpacing(1)
	return t.Print(int(x), y+4, num)
}

// playerRaceAnyIndex is players[i].race (+0x25) for a colonist's owner
// nibble, which is 8 or 9 for natives and androids: past the 8 records the
// original reads the next heap block, the colonies (Allocate_Data_Space_
// mallocs players, then colonies; 4-byte malloc tag + 12-byte header).
func (g *Game) playerRaceAnyIndex(i int) uint8 {
	s := &g.state
	if i < 8 {
		return s.Players[i].Race
	}
	off := (i-8)*0xea9 + 0x15
	ci, at := off/0x169, off%0x169
	var b bytes.Buffer
	binary.Write(&b, binary.LittleEndian, &s.Colonies[ci])
	return b.Bytes()[at]
}
