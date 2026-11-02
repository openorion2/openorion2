package game

import (
	"encoding/binary"
	"fmt"
	"strconv"

	"github.com/openorion2/openorion2/internal/save"
)

// The history graph page of the info screen (INFO.C). Spec:
// docs/screens/info-screen.md, "History graph".

// historySubscreen is History_Subscreen_ (0x107e95); first is the first
// field number the page adds. Returns the field that leaves the page.
func (g *Game) historySubscreen(first int) (int, error) {
	e, is, s := g.e, &g.info, &g.state
	me := int(s.PlayerNum)
	title, err := g.textMessage(billTextLBX, 5, 0x28)
	if err != nil {
		return 0, err
	}
	if err := e.Text().PrintCentered(0x1a0, 0x1f, title); err != nil {
		return 0, err
	}
	g.setUpPlayerColors()
	g.activePlayer[0] = uint8(me)
	g.getPlayersAliveOrOmniscient(1, false)
	var btn [4]int16
	for i := range btn {
		btn[i] = int16(s.Players[me].InfoButtons & (1 << i))
	}
	if btn[0]+btn[1]+btn[2]+btn[3] == 0 {
		btn = [4]int16{1, 1, 1, 1}
		g.setPlyrHistBtns(&btn)
	}
	for i := range btn {
		x := int(g.exeI16(infoGraphButtonX + uint32(2*i)))
		if _, err := e.AddRadioButtonField(x, 0x1ab, "", is.pics[8+i], &btn[i], "", 0x28); err != nil {
			return 0, err
		}
	}
	e.AssignAutoFunction(func() error { return nil }, 2) // Draw_History_Subscreen_
	for {
		g.drawDropAreas(infoHistoryDrops, infoTechDrops)
		if err := g.displayGraphedPlayersNames(); err != nil {
			return 0, err
		}
		step, err := g.drawTheHistoryGraph()
		if err != nil {
			return 0, err
		}
		if err := g.drawHistories(step); err != nil {
			return 0, err
		}
		e.V.CopyOffToBack()
		if err := g.introduceSubscreen(); err != nil {
			return 0, err
		}
		is.showing = infoHistory
		var r int
		for {
			e.MarkTime()
			in, err := e.GetInput()
			if err != nil {
				return 0, err
			}
			r = int(int16(in))
			if g.fullDrawScreen {
				return 0, nil
			}
			if r >= first {
				g.setPlyrHistBtns(&btn)
			} else if r == 0 {
				if err := e.TogglePages(); err != nil {
					return 0, err
				}
				if err := e.ReleaseTime(2); err != nil {
					return 0, err
				}
			}
			if r != 0 {
				break
			}
		}
		if r < first {
			return r, nil
		}
	}
}

// setPlyrHistBtns is Set_Plyr_Hist_Btns_ (0x1078ed): the graph buttons
// into bits 0..3 of the human's InfoButtons.
func (g *Game) setPlyrHistBtns(btn *[4]int16) {
	pl := &g.state.Players[g.state.PlayerNum]
	for i, b := range btn {
		if b == 0 {
			pl.InfoButtons &^= 1 << i
		} else {
			pl.InfoButtons |= 1 << i
		}
	}
}

// displayGraphedPlayersNames is Display_Graphed_Players_Names_ (0x107dea):
// the names in columns of three in each player's graph colours.
func (g *Game) displayGraphedPlayersNames() error {
	is, s := &g.info, &g.state
	y, x := 0x40, 0x15f
	dx := 0 // uninitialised in the original; only used after the last name
	if g.activeCount > 3 {
		x, dx = 0x127, 0x96
	}
	if g.activeCount > 6 {
		x, dx = 0xe9, 0x7a
	}
	t := g.e.Text()
	for i := 0; i < int(g.activeCount); i++ {
		p := g.activePlayer[i]
		t.SetStyle(4, is.histPal[p])
		if err := t.Print(int(int16(x)), int(int16(y)), save.CString(s.Players[p].RaceName[:])); err != nil {
			return err
		}
		y += 0x13
		if i == 2 || i == 5 {
			y = 0x40
			x += dx
		}
	}
	return nil
}

// drawTheHistoryGraph is Draw_The_History_Graph_ (0x10766e): the axes with
// their ticks and the stardate labels; returns the x step per turn.
func (g *Game) drawTheHistoryGraph() (int, error) {
	v := g.e.V
	tick := g.exe.bytes(infoTicks, 10)
	i := 0
	for x := uint16(0xed); x <= 0x24c; x += 9 {
		y2 := int(int16(uint16(tick[i]) + 0x186))
		i++
		if i >= 10 {
			i = 0
		}
		v.Line(int(int16(x)), 0x185, int(int16(x)), y2, 0xb1)
		x++
		v.Line(int(int16(x)), 0x185, int(int16(x)), y2, 0xb9)
	}
	v.Line(0xe8, 0x186, 0x24c, 0x186, 0xb9)
	v.Line(0xed, 0x8c, 0xed, 0x18c, 0xb1)
	v.Line(0xee, 0x8c, 0xee, 0x18c, 0xb9)
	i = 0
	for y := uint16(0x186); y >= 0x8c; y -= 10 {
		c := int16(0xee - uint16(tick[i]))
		i++
		if i >= 10 {
			i = 0
		}
		v.Dot(int(c-1), int(int16(y)), 0xb1)
		v.Line(int(c), int(int16(y)), 0xef, int(int16(y)), 0xb9)
	}
	t := g.e.Text()
	t.SetStyle(1, g.exeColors(infoNormalColors))
	turns := int16(uint16(g.state.Stardate) - 35000)
	start := int32(35000)
	var label int32
	var step int
	switch {
	case turns <= 0x23:
		label, step = 5, 10
	case turns <= 0x46:
		label, step = 10, 5
	case turns <= 0xaf:
		label, step = 25, 2
	default:
		label, step = 50, 1
		if turns > 0x15e {
			start = (int32(g.state.Stardate) - 0x159) / 10 * 10
		}
	}
	x := 0xef
	for k := 0; k < 8; k++ {
		var str string
		if start%5 == 0 && start%10 != 0 {
			f := infoDateFmt
			if g.settings[0xd2] != 0 {
				f += 6
			}
			str = fmt.Sprintf(cFormat(cStringAt(g.exe, uint32(f))), start/10, start-start/10*10)
		} else {
			str = strconv.Itoa(int(start / 10))
		}
		if err := t.PrintCentered(int(int16(x)), 400, str); err != nil {
			return 0, err
		}
		x += 0x32
		start += int32(int16(label))
	}
	return step, nil
}

// historyParams is Get_History_Params_ (0x1029d1): the samples kept and
// the ring index of the oldest.
func (g *Game) historyParams() (n, start int) {
	turns := int16(uint16(g.state.Stardate) - 35000)
	if turns > historyLen {
		return historyLen, int(int16(binary.LittleEndian.Uint16(g.state.BillSavegame[2:])))
	}
	return int(turns), 0
}

// drawHistories is Draw_Histories_ (0x10794c): per active player the sum
// of the chosen series (times their divisors), smoothed ten times, scaled
// to the first of 25, 50, 100, 250, 500, 1000, 2000, 4000.. above the
// largest, drawn as lines with a shadow.
func (g *Game) drawHistories(step int) error {
	is, s := &g.info, &g.state
	pl := &s.Players[s.PlayerNum]
	var on [4]bool
	n := 0
	for i := range on {
		on[i] = pl.InfoButtons&(1<<i) != 0
		if on[i] {
			n++
		}
	}
	if n == 0 {
		return nil
	}
	word := func(off int) uint32 { return uint32(binary.LittleEndian.Uint16(s.BillSavegame[off:])) }
	var data [8][historyLen]uint32
	var turns int
	for a := int(g.activeCount) - 1; a >= 0; a-- {
		p := g.activePlayer[a]
		h := &s.Players[p].History
		var k int
		turns, k = g.historyParams()
		for i := 0; i < turns; i++ {
			v := uint32(0)
			if on[0] {
				v = uint32(h[histPop][k]) * word(4)
			}
			if on[3] {
				v += word(0xa) * uint32(h[histTech][k])
			}
			if on[2] {
				v += word(8) * uint32(h[histFleet][k])
			}
			if on[1] {
				v += word(6) * uint32(h[histBuildings][k])
			}
			data[p][i] = v
			k++
			if k >= historyLen {
				k = 0
			}
		}
	}
	for a := int(g.activeCount) - 1; a >= 0; a-- {
		d := &data[g.activePlayer[a]]
		for pass := 0; pass < 10; pass++ {
			for i := 0; i < turns; i++ {
				smoothHistoryData(d[:], i, turns)
			}
		}
	}
	hi := uint32(0)
	for a := int(g.activeCount) - 1; a >= 0; a-- {
		for i := 0; i < turns; i++ {
			hi = max(hi, data[g.activePlayer[a]][i])
		}
	}
	scale := uint32(25)
	for hi >= scale {
		switch {
		case scale == 25:
			scale = 50
		case scale == 50:
			scale = 100
		case scale == 100:
			scale = 250
		case scale == 250:
			scale = 500
		case scale == 500:
			scale = 1000
		case scale == 1000:
			scale = 2000
		default:
			scale += 2000
		}
	}
	div := scale * 100 / 250
	for a := int(g.activeCount) - 1; a >= 0; a-- {
		d := &data[g.activePlayer[a]]
		for i := 0; i < turns; i++ {
			d[i] = d[i] * 100 / div
		}
	}
	t := g.e.Text()
	t.SetStyle(1, g.exeColors(infoNormalColors))
	if err := t.Print(0xf6, 0x8a, strconv.FormatUint(uint64(scale), 10)); err != nil {
		return err
	}
	v := g.e.V
	turns--
	for a := int(g.activeCount) - 1; a >= 0; a-- {
		p := g.activePlayer[a]
		d := &data[p]
		c := is.histPal[p][7]
		x := 0xee
		for i := 0; i < turns; i++ {
			y0 := int16(0x186 - uint16(d[i]))
			y1 := int16(0x186 - uint16(d[i+1]))
			x2 := x + step
			v.Line(int(int16(x+1)), int(y0+1), int(int16(x2+1)), int(y1+1), 0xb1)
			v.Line(int(int16(x)), int(y0), int(int16(x2)), int(y1), c)
			x = x2
		}
	}
	return nil
}

// smoothHistoryData is Smooth_History_Data_ (0x107d5b): sample i+1 becomes
// the mean of samples i..i+2 (sample i standing in past the end).
func smoothHistoryData(d []uint32, i, n int) {
	if i >= n {
		return
	}
	a := d[i]
	b := a
	if i+1 < n {
		b = d[i+1]
	}
	c := d[i]
	if i+2 < n {
		c = d[i+2]
	}
	if avg := (c + a + b) / 3; i+1 < n {
		d[i+1] = avg
	}
}
