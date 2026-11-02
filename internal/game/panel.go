package game

import (
	"bytes"
	"fmt"

	"github.com/openorion2/openorion2/internal/font"
)

// The data panel right of the map and the message strings. Spec:
// docs/screens/main-screen-panel.md.

// loadHStrings is Load_H_Strings_: HESTRNGS.LBX entry 0, a one-record table
// of NUL-separated strings. The language is always 0 in this build.
func (g *Game) loadHStrings() error {
	a, err := g.archive("HESTRNGS.LBX")
	if err != nil {
		return err
	}
	t, err := a.Table(0)
	if err != nil {
		return err
	}
	data := t.Record(0)
	g.hStrings = g.hStrings[:0]
	for len(g.hStrings) < 0x18d && len(data) > 0 {
		n := bytes.IndexByte(data, 0)
		if n < 0 {
			n = len(data)
		}
		if n > 0x181 {
			return fmt.Errorf("HESTRNGS.LBX: String %d Too Long", len(g.hStrings))
		}
		g.hStrings = append(g.hStrings, string(data[:n]))
		if n == len(data) {
			break
		}
		data = data[n+1:]
	}
	return nil
}

// hMessage is H_Message_: out-of-range indices give string 0.
func (g *Game) hMessage(i int) string {
	if i < 0 || i >= 0x18d || i >= len(g.hStrings) {
		i = 0
	}
	return g.hStrings[i]
}

// commaString is Get_Comma_String_.
func commaString(v int32) string {
	abs := v
	if abs < 0 {
		abs = -abs
	}
	digits := fmt.Sprintf("%d", abs)
	l := len(digits)
	k := int(int16(l-1)) / 3
	var out string
	if k == 0 {
		out = digits
	} else {
		buf := make([]byte, l+k+1)
		di := l
		c := 0
		for pos := l + k; pos >= 0; pos-- {
			switch {
			case pos == l+k:
				di--
				buf[pos] = 0
			case c%4 == 0 && c > 1 && k > 0:
				k--
				buf[pos] = ','
			default:
				buf[pos] = digits[di]
				di--
			}
			c++
		}
		out = string(buf[:len(buf)-1])
	}
	if v < 0 {
		out = "-" + out
	}
	return out
}

func sign(v int) string {
	if v < 0 {
		return ""
	}
	return "+"
}

// setFittedFontStyle is Set_Fitted_Font_Style_: shrink spacing, then style,
// until s is narrower than maxw.
func (g *Game) setFittedFontStyle(s string, colors font.Colors, style, sp int, shadow font.Shadow, maxw, minStyle int) {
	t := g.e.Text()
	start := sp
	if shadow == -1 {
		shadow = 0
	}
	for {
		t.SetStyle(style, colors)
		t.SetSpacing(sp)
		t.Shadow = shadow
		if t.Width([]byte(s)) < maxw {
			return
		}
		sp--
		if sp < 1 {
			style--
			sp = start
			if style < minStyle {
				return
			}
		}
	}
}

// setPostFittedFontStyle is Set_Post_Fitted_Font_Style_.
func (g *Game) setPostFittedFontStyle(a, b byte) {
	t := g.e.Text()
	style, sp := t.Style, t.Spacing
	c := g.fontColors2(style, a, b)
	t.SetOutlineColor(2, 1)
	t.SetStyleShadow(style, c, font.Outline)
	t.SetSpacing(sp)
}

// increaseFontColorRange is Increase_Font_Color_Range_.
func (g *Game) increaseFontColorRange(n int) {
	t := g.e.Text()
	c := t.Current
	for i := 1; i < 8; i++ {
		c[i] = byte(min(n+int(c[i]), 0xff))
	}
	t.Normal, t.Current = c, c
}

// researchCost is Player_Research_Cost_ (0xe1e96).
func (g *Game) researchCost(p, field int) int32 {
	if field <= 0 {
		return 0
	}
	c := int32(g.exe.u32(uint32(0x17d916 + field*0x17)))
	if field > 0x4a {
		c += int32(g.state.Players[p].HyperLevels[field-0x4b]) * 10000 // player+0x1d1+field
	}
	return c
}

// breakthroughAux is Chance_For_Research_Breakthrough_Aux_.
func breakthroughAux(total, _, cost int32) int32 {
	if cost <= 0 || cost >= total {
		return 0
	}
	return max(1, min(100, (total-cost)*100/cost))
}

// chanceForResearchBreakthrough is Chance_For_Research_Breakthrough_.
func (g *Game) chanceForResearchBreakthrough(p int) int32 {
	pl := &g.state.Players[p]
	cost := g.researchCost(p, int(pl.ResearchField))
	return breakthroughAux(pl.ResearchPoints+int32(pl.ResearchProduced), int32(pl.ResearchProduced), cost)
}

// turnsUntilResearchComplete is Player_N_Turns_Until_Research_Complete_.
func (g *Game) turnsUntilResearchComplete(p int) int {
	pl := &g.state.Players[p]
	if pl.FieldStatus[pl.ResearchField] == 3 {
		return 0
	}
	rp := int32(pl.ResearchProduced)
	cost := g.researchCost(p, int(pl.ResearchField))
	acc := pl.ResearchPoints
	if rp == 0 && acc <= cost {
		return -1
	}
	n := 0
	for ; acc <= cost; acc += rp {
		n++
	}
	for sum := int32(0); sum < 100; {
		acc += rp
		n++
		sum += breakthroughAux(acc, rp, cost)
	}
	return n
}

// printMainScreenData is Print_Main_Screen_Data_ (0x87bae). A and B are
// the two colour buffers the original keeps on its stack.
func (g *Game) printMainScreenData() error {
	s, t := &g.state, g.e.Text()
	p := int(s.PlayerNum)
	pl := &s.Players[p]
	const left, width, maxw = 0x224, 0x3e, 0x43
	cx := width/2 + left
	pulse := int(byte(g.ms.redPulseValue))
	var a, b font.Colors

	// 1. Stardate; the language is always English, so the separator is '.'.
	t.SetStyle(0, b)
	sd := s.Stardate
	str := fmt.Sprintf("%d%c%d", sd/10, '.', sd%10)
	g.setFittedFontStyle(str, a, 2, 2, -1, maxw, 1)
	st := t.Style
	a = g.fontColors2(st, 0x0e, 0x17)
	t.SetOutlineColor(2, 1)
	t.SetStyleShadow(st, a, font.Outline)
	if err := t.PrintCentered(width/2+0x225, 0x1d, str); err != nil {
		return err
	}
	// 2. Hyperspace flux: Event_Check_Hyperspace_Flux_ reads the event
	// state, which is not decoded yet.
	if g.hyperspaceFlux() {
		b = g.fontColors2(3, 0x49, 0x4c)
		t.SetStyle(3, b)
		g.increaseFontColorRange(pulse * 5 / 9)
		msg := g.hMessage(0xe0)
		x0 := 0x20e - t.Width([]byte(msg))
		g.e.V.Line(x0, 0x15, 0x20e, 0x15, b[0])
		g.e.V.Line(x0, 0x16, 0x20e, 0x16, b[1])
		if err := t.PrintRight(0x20e, 0x18, msg); err != nil {
			return err
		}
		h := t.Height()
		g.e.V.Line(x0, h+0x19, 0x20e, h+0x19, b[1])
		g.e.V.Line(x0, h+0x1a, 0x20e, h+0x1a, b[0])
	}
	// 3. The income style decides the colours of the reserve.
	inc := int(pl.NetIncome)
	incStr := fmt.Sprintf("%s%s %s", sign(inc), commaString(int32(inc)), "BC")
	g.setFittedFontStyle(incStr, a, 2, 2, font.Outline, maxw, 1)
	st = t.Style
	b = g.fontColors2(st, 0x4b, 0x4d)
	a = g.fontColors2(st, 0x0e, 0x17)
	// 4. Reserve.
	str = fmt.Sprintf("%s %s", commaString(pl.BC), "BC")
	if pl.BC < 0 {
		g.setFittedFontStyle(str, b, 2, 2, font.Outline, maxw, 1)
		g.setPostFittedFontStyle(0x4b, 0x4d)
		g.increaseFontColorRange((pulse << 2) / 7)
		t.SetOutlineColor(0x49, 0x49)
	} else {
		g.setFittedFontStyle(str, a, 2, 2, font.Outline, maxw, 1)
		t.SetOutlineColor(2, 1)
	}
	if err := t.PrintCentered(cx, 0x60, str); err != nil {
		return err
	}
	// 5. Income.
	g.setFittedFontStyle(incStr, a, 2, 2, font.Outline, maxw, 1)
	st, sp := t.Style, t.Spacing
	b = g.fontColors2(st, 0x4b, 0x4d)
	a = g.fontColors2(st, 0x0e, 0x17)
	if inc < 0 {
		t.SetOutlineColor(0x49, 0x49)
		t.SetStyleShadow(st, b, font.Outline)
		t.SetSpacing(sp)
		g.increaseFontColorRange((pulse << 2) / 7)
	} else {
		t.SetStyleShadow(st, a, font.Outline)
		t.SetOutlineColor(2, 1)
		t.SetSpacing(sp)
	}
	if err := t.PrintCentered(cx, t.Height()+0x60, incStr); err != nil {
		return err
	}
	// 6. Command points.
	d := int(int16(pl.CommandTotal - pl.CommandUsed))
	tot := int(pl.CommandTotal)
	str = fmt.Sprintf("%s%d (%d)", sign(d), d, tot)
	g.setFittedFontStyle(str, a, 2, 2, font.Outline, maxw, 0)
	st, sp = t.Style, t.Spacing
	b = g.fontColors2(st, 0x4b, 0x4d)
	a = g.fontColors2(st, 0x0e, 0x17)
	s1 := fmt.Sprintf("%s%d", sign(d), d)
	w1 := t.Width([]byte(s1))
	s2 := fmt.Sprintf("(%d)", tot)
	w2 := t.Width([]byte(s2))
	x := left + 2 + width/2 - (w1+4+w2)/2
	if d < 0 {
		t.SetStyleShadow(st, b, font.Outline)
		t.SetOutlineColor(0x49, 0x49)
		t.SetSpacing(sp)
		g.increaseFontColorRange((pulse << 2) / 7)
	} else {
		t.SetStyleShadow(st, a, font.Outline)
		t.SetOutlineColor(2, 1)
		t.SetSpacing(sp)
	}
	if err := t.Print(x, 0xb5, s1); err != nil {
		return err
	}
	t.SetStyle(st, a)
	t.SetSpacing(sp)
	if err := t.Print(x+w1+4, 0xb5, s2); err != nil {
		return err
	}
	hcp := t.Height()
	// 7. Food.
	f := int(pl.FoodSurplus)
	str = fmt.Sprintf("%s%s", sign(f), commaString(int32(f)))
	if f < 0 {
		g.setFittedFontStyle(str, b, 2, 2, font.Outline, maxw, 1)
		g.setPostFittedFontStyle(0x4b, 0x4d)
		t.SetOutlineColor(0x49, 0x49)
		g.increaseFontColorRange((pulse << 2) / 7)
	} else {
		g.setFittedFontStyle(str, a, 2, 2, font.Outline, maxw, 1)
		g.setPostFittedFontStyle(0x0e, 0x17)
		t.SetOutlineColor(2, 1)
	}
	if err := t.PrintCentered(cx, 0xff, str); err != nil {
		return err
	}
	// 8. Freighters.
	fr, ftot := int(pl.FreightersFree), int(pl.FreightersTotal)
	str = fmt.Sprintf("%s%d (%d)", sign(fr), fr, ftot)
	g.setFittedFontStyle(str, b, 2, 2, font.Outline, maxw, 1)
	st = t.Style
	b = g.fontColors2(st, 0x4b, 0x4d)
	a = g.fontColors2(st, 0x0e, 0x17)
	sp = t.Spacing
	wf := t.Width([]byte(str)) + 4
	s1 = fmt.Sprintf("%s%d", sign(fr), fr)
	if fr < 0 {
		t.SetStyleShadow(st, b, font.Outline)
		t.SetSpacing(sp)
		g.increaseFontColorRange((pulse << 2) / 7)
		t.SetOutlineColor(0x49, 0x49)
	} else {
		t.SetStyleShadow(st, a, font.Outline)
		t.SetSpacing(sp)
		t.SetOutlineColor(2, 1)
	}
	x = left + 2 + width/2 - wf/2
	if err := t.Print(x, 0x14a, s1); err != nil {
		return err
	}
	t.SetOutlineColor(2, 1)
	w1 = t.Width([]byte(s1))
	str = fmt.Sprintf("(%d)", ftot)
	t.SetStyleShadow(st, a, font.Outline)
	t.SetSpacing(sp)
	if err := t.Print(x+w1+2, 0x14a, str); err != nil {
		return err
	}
	// 9. Research.
	const ry = 0x193
	switch {
	case pl.BreakthroughPending != 0:
		t.SetOutlineColor(2, 1)
		t.SetStyleShadow(0, a, font.Outline)
		return t.PrintCentered(cx, ry, g.hMessage(0x183))
	case pl.ResearchField == 0:
		t.SetOutlineColor(2, 1)
		g.setFittedFontStyle(str, a, 2, 2, font.Outline, maxw, 1) // str still holds "(tot)"
		g.setPostFittedFontStyle(0x0e, 0x17)
		return t.PrintCentered(cx, ry, g.hMessage(0x188))
	}
	n := g.turnsUntilResearchComplete(p)
	fit := func(s string) {
		g.setFittedFontStyle(s, a, 2, 2, font.Outline, maxw, 1)
		g.setPostFittedFontStyle(0x0e, 0x17)
		t.SetOutlineColor(2, 1)
	}
	switch {
	case n < 0:
		str = fmt.Sprintf("0 %s", "RP")
		fit(str)
		return t.PrintCentered(cx, ry, str)
	case n == 0:
		c := g.chanceForResearchBreakthrough(p)
		y := ry - 0xc
		str = fmt.Sprintf("%d%%", c)
		fit(str)
		if err := t.PrintCentered(cx, y, str); err != nil {
			return err
		}
		y += hcp
		str = fmt.Sprintf("%d %s", pl.ResearchProduced, "RP")
		fit(str)
		return t.PrintCentered(cx, y, str)
	}
	if c := g.chanceForResearchBreakthrough(p); c > 0 {
		str = fmt.Sprintf("%d%%", c)
		fit(str)
		if err := t.PrintCentered(cx, ry-2*t.Height(), str); err != nil {
			return err
		}
	}
	msg := 0xe2
	if n == 1 {
		msg = 0xe1
	}
	turns := fmt.Sprintf(g.hMessage(msg), n)
	g.setFittedFontStyle("@"+turns, a, 2, 2, font.Outline, maxw, 1)
	g.setPostFittedFontStyle(0x0e, 0x17)
	y := ry - t.Height()
	t.SetOutlineColor(2, 1)
	x = (width-t.Width([]byte(turns)))/2 + left
	if err := t.Print(x, y+3, "@"); err != nil {
		return err
	}
	if err := t.Print(x+t.Width([]byte("@")), y, turns); err != nil {
		return err
	}
	y += t.Height()
	str = fmt.Sprintf("%s %s", commaString(int32(pl.ResearchProduced)), "RP")
	fit(str)
	return t.PrintCentered(cx, y, str)
}

// hyperspaceFlux is Event_Check_Hyperspace_Flux_ (0x233fa): event 9 running.
func (g *Game) hyperspaceFlux() bool { return g.state.evActive(eventHyperspaceFlux) }
