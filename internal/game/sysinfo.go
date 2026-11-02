package game

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/openorion2/openorion2/internal/save"
)

// The system display's hover boxes: planet info, ship info and gate info.
// Spec: docs/screens/system-display.md, "Info boxes".

const (
	eStringCount = 0x32c  // Load_E_Strings_ loop bound
	eStringsSize = 0x5208 // the static buffer at 0x1a1370
	eStringPatch = 0x170  // overwritten with the 13 bytes at 0x179eff
)

// Walks Load_Tech_Names_ (0x5df0a) makes before a table's first record:
// fields 1..82 (field 0 is the start), 212 applications, then the
// buildings; hull names come after 13 tables totalling 541 walks.
const (
	techWalksBuildings = 82 + 212
	techWalksHulls     = 82 + 212 + 49 + 40 + 7 + 6 + 46 + 6 + 7 + 4 + 6 + 15 + 15 + 46
)

// Pointer tables that Load_H_And_Pro_File_Strings_ (0xce0e5) fills with
// E_Strings_(i); the indices are immediates in that routine.
var (
	planetSizeEStr    = [5]int{0x2ab, 0x1e0, 0x173, 0x168, 0x143}                                                                               // 0x192be0
	planetMineralEStr = [5]int{0x2ac, 0x1a7, 0x2ad, 0x1c2, 0x2ae}                                                                               // 0x192bf4
	planetGravityEStr = [3]int{0x2af, 0x2b0, 0x2b1}                                                                                             // 0x192c74
	planetClimateEStr = [10]int{0x21b, 0x2cf, 0x2d0, 0x2d1, 0x2d2, 0x18f, 0x1f5, 0xb8, 0x2d3, 0x12f}                                            // 0x18f990
	planetSpecialEStr = [16]int{0x2b9, 0x2ba, 0x2bb, 0x2bc, 0x2bd, 0x2be, 0x2bf, 0x2c0, 0x2c1, 0x2c2, 0x2c3, 0x2c4, 0x2b5, 0x2b6, 0x2b7, 0x2b8} // 0x18f9b8
)

// eStrings is Load_E_Strings_ (0xcdf65) for language 0: ESTRINGS.LBX entry 0
// split into 0x32c strings, then string 0x170 patched in place.
func (g *Game) eStrings() ([]string, error) {
	a, err := g.archive("ESTRINGS.LBX")
	if err != nil {
		return nil, err
	}
	t, err := a.Table(0)
	if err != nil {
		return nil, err
	}
	buf := make([]byte, eStringsSize)
	copy(buf, t.Record(0))
	var offs [eStringCount]int
	p := 0
	for i := range offs {
		offs[i] = p
		n := -1
		if p < len(buf) {
			n = bytes.IndexByte(buf[p:], 0)
		}
		if n < 0 {
			return nil, fmt.Errorf("you blew the array eric")
		}
		p += n + 1
	}
	// The patch copies 13 bytes, so a shorter string 0x170 would spill into
	// the next one; pointers already computed keep their offsets.
	copy(buf[offs[eStringPatch]:], g.exe.bytes(0x179eff, 13))
	out := make([]string, eStringCount)
	for i, o := range offs {
		out[i] = save.CString(buf[o:])
	}
	return out, nil
}

// techName is the string Load_Tech_Names_ (0x5df0a) reaches in TECHNAME.LBX
// entry 0 after n walks; a walk skips a string and the NUL run after it.
func (g *Game) techName(n int) (string, error) {
	a, err := g.archive("TECHNAME.LBX")
	if err != nil {
		return "", err
	}
	data, err := a.Entry(0)
	if err != nil {
		return "", err
	}
	p := 0
	for ; n > 0; n-- {
		for p < len(data) && data[p] != 0 {
			p++
		}
		for p < len(data) && data[p] == 0 {
			p++
		}
	}
	if p >= len(data) {
		return "", nil
	}
	return save.CString(data[p:]), nil
}

// buildingName is _buildings[id].name (0x17eb3d, 19-byte rows).
func (g *Game) buildingName(id int) (string, error) {
	return g.techName(techWalksBuildings + 1 + id)
}

// hullName is the hull table's singular (0x18003a) or plural (0x18003e)
// name, 36-byte rows, nine of each.
func (g *Game) hullName(size int, plural bool) (string, error) {
	n := techWalksHulls + 1 + size
	if plural {
		n += 9
	}
	return g.techName(n)
}

// playerHasColony is Player_Has_Colony_ (0x79d1c): bit p of the bit field at
// star+0x38.
func (s *State) playerHasColony(p, star int) bool {
	st := s.starRecord(star)
	if p >= 0 && p < 8 {
		return st.ColonyOwners>>p&1 != 0
	}
	return starByte(st, 0x38+p>>3)>>(p&7)&1 != 0
}

// planetNumber is Planet_Number_ (0x79c6b): how many occupied orbits of the
// planet's star come before it.
func (s *State) planetNumber(pl int) int {
	st := &s.Stars[s.Planets[pl].Star]
	n := 0
	for o := 0; o < 5; o++ {
		if int(st.Planets[o]) == pl {
			break
		}
		if st.Planets[o] > -1 {
			n++
		}
	}
	return n
}

// planetNameNoPrime is Get_Planet_Name_No_Prime_ (0x77b42): "%s %s" of the
// star name and the Roman numeral (pointer table 0x17d5e4), cut to 30 chars.
func (g *Game) planetNameNoPrime(pl int) string {
	s := &g.state
	n := s.planetNumber(pl)
	roman := cStringAt(g.exe, g.exe.u32(uint32(0x17d5e4+4*n)))
	name := fmt.Sprintf("%s %s", save.CString(s.Stars[s.Planets[pl].Star].Name[:]), roman)
	if len(name) > 0x1e {
		name = name[:0x1e]
	}
	return name
}

// sizeAndClimateMaxPopulation is Size_And_Climate_Max_Population_ (0xe0a18):
// tables _planet_size_max_pop (0x17d806) and the climate percentages at
// 0xdd4eb, +25% for tolerant, capped at 100%.
func (g *Game) sizeAndClimateMaxPopulation(size, climate int, tolerant bool) int32 {
	sz := int32(g.exe.bytes(uint32(0x17d806+size), 1)[0])
	pct := int32(g.exe.bytes(uint32(0xdd4eb+climate), 1)[0])
	if tolerant {
		pct += 0x19
	}
	pct = min(pct, 100)
	return (pct*sz + 50) / 100
}

// sizeAndClimateRacePopLimit is Size_And_Climate_Race_Pop_Limit_ (0xe0a93);
// race 8 (androids) counts as tolerant, races above 7 have no picks.
func (g *Game) sizeAndClimateRacePopLimit(size, climate, race int) int32 {
	s := &g.state
	var picks *[31]int8
	if race < 8 {
		picks = &s.Players[race].Picks
	}
	tolerant := race == 8 || (picks != nil && picks[save.PickTolerant] != 0)
	if picks != nil && picks[save.PickAquatic] != 0 {
		switch climate {
		case 4, 6: // tundra, swamp
			climate = 8
		case 5, 8: // ocean, terran
			climate = 9
		}
	}
	v := g.sizeAndClimateMaxPopulation(size, climate, tolerant)
	if picks != nil && picks[save.PickSubterranean] != 0 {
		v += int32(int16((size + 1) * 2))
	}
	return v
}

// colonyRacePopLimit is Colony_Race_Pop_Limit_ (0xe0c1d): the owner's
// application 3 adds 5, building 15 (Biospheres) adds 2.
func (g *Game) colonyRacePopLimit(c *save.Colony, race int) int32 {
	s := &g.state
	v := int32(int16(g.sizeAndClimateRacePopLimit(int(s.Planets[c.Planet].Size), int(c.Climate), race)))
	if s.Players[c.Owner].AppStatus[3] == 3 {
		v += 5
	}
	if c.Buildings[15] != 0 {
		v += 2
	}
	return v
}

// planetMaxPopulationForPlayer is Planet_Max_Population_For_Player_
// (0xe0b4f): for a colony, the best limit among the races living there.
func (g *Game) planetMaxPopulationForPlayer(pl, player int) int16 {
	s := &g.state
	p := &s.Planets[pl]
	if p.Colony == -1 || s.Colonies[p.Colony].IsOutpost != 0 {
		v := int32(int16(g.sizeAndClimateRacePopLimit(int(p.Size), int(p.Climate), player)))
		if s.Players[player].AppStatus[3] == 3 {
			v += 5
		}
		return int16(v)
	}
	c := &s.Colonies[p.Colony]
	if c.PopCount == 0 {
		return int16(g.colonyRacePopLimit(c, player))
	}
	var present [16]bool
	for n := min(int(c.PopCount), len(c.Colonists)); n > 0; n-- {
		present[c.Colonists[n-1]&0xf] = true
	}
	best := int32(0)
	for r := 9; r >= 0; r-- {
		if present[r] {
			if v := int32(int16(g.colonyRacePopLimit(c, r))); best < v {
				best = v
			}
		}
	}
	return int16(best)
}

// infoBox accumulates the measure-mode size of an info box; in print mode it
// prints instead.
type infoBox struct {
	g     *Game
	print bool
	w, h  int16
}

// line prints s at (x, y) or widens the box to width(s) + extra.
func (b *infoBox) line(x, y int, s string, extra int16) error {
	t := b.g.e.Text()
	if b.print {
		return t.Print(int(int16(x)), int(int16(y)), s)
	}
	if v := int16(t.Width([]byte(s))) + extra; v > b.w {
		b.w = v
	}
	return nil
}

// doPrintPlanetInfo is Do_Print_Planet_Info_ (0xa2123); x, y are X+dx+L and
// Y+dy+T. Measure mode returns the text size.
func (g *Game) doPrintPlanetInfo(x, y, planet int, print bool) (int16, int16, error) {
	s, t := &g.state, g.e.Text()
	b := &infoBox{g: g, print: print, h: 0xc}
	pn := int(s.PlayerNum)
	p := &s.Planets[planet]
	detail := g.sd.playerHasShip || s.playerHasColony(pn, int(p.Star)) || s.playerIsOmniscient(pn)
	if _, err := g.cached(buffer0LBX, 0x49); err != nil {
		return 0, 0, err
	}
	colors := g.fontColors2(2, 0x3c, 0x43)
	col := int(p.Colony)
	t.SetStyle(2, colors)
	t.SetSpacing(1)
	lh := int16(uint8(t.Height() + 1))
	x, y = x+6, y+7
	es, err := g.eStrings()
	if err != nil {
		return 0, 0, err
	}
	finish := func(err error) (int16, int16, error) {
		b.h += 2
		return b.w, b.h, err
	}
	switch p.Type {
	case 2:
		return finish(b.line(x, y, g.hMessage(0x15f), 0))
	case 1:
		return finish(b.line(x, y, g.hMessage(0x160), 0))
	}
	climate := p.Climate
	var c *save.Colony
	if col > -1 {
		c = &s.Colonies[col]
		climate = c.Climate
	}
	l := g.planetNameNoPrime(planet) + " "
	if c != nil {
		l += "(" + save.CString(s.Players[c.Owner].RaceName[:])
		if c.IsOutpost != 0 {
			l += g.hMessage(0x161)
		} else {
			l += ")"
		}
	}
	if err := b.line(x, y, l, 0); err != nil {
		return 0, 0, err
	}
	if print {
		t.SetStyle(2, colors)
		t.SetSpacing(1)
	}
	next := func(s string) error {
		if print {
			y += int(lh)
		} else {
			b.h += lh
		}
		return b.line(x, y, s, 0)
	}
	if err := next(es[planetSizeEStr[p.Size%5]] + ", " + es[planetClimateEStr[climate%10]]); err != nil {
		return 0, 0, err
	}
	maxPop := int(g.planetMaxPopulationForPlayer(planet, pn))
	l = fmt.Sprintf(g.hMessage(0x162), maxPop)
	if c != nil {
		l = fmt.Sprintf(g.hMessage(0x181), int(c.PopCount), maxPop)
	}
	if err := next(l); err != nil {
		return 0, 0, err
	}
	y += int(lh)
	b.h += lh
	l = es[planetMineralEStr[p.Minerals%5]]
	if p.Gravity != 1 {
		l += ", " + es[planetGravityEStr[p.Gravity%3]]
	}
	if err := b.line(x, y, l, 0); err != nil {
		return 0, 0, err
	}
	nb := 0
	if c != nil && detail {
		if nb, err = g.printSpecialBuildingData(c, x, &y, b, lh); err != nil {
			return 0, 0, err
		}
	}
	if sp := int8(p.Special); sp != 0 && sp != 11 {
		if nb == 0 {
			y += 4
			b.h += 4
		}
		y += int(lh)
		b.h += lh
		return finish(b.line(x, y, es[planetSpecialEStr[int(sp)&15]], 0))
	}
	return finish(nil)
}

// printSpecialBuildingData is Print_Special_Building_Data_ (0xa58ce): one
// line per standing building of the list at 0xa1c66 (bases, stellar
// converter, …), after a gap of font style + 3; returns the line count.
func (g *Game) printSpecialBuildingData(c *save.Colony, x int, y *int, b *infoBox, lh int16) (int, error) {
	n := 0
	for i := 0; i < 7; i++ {
		id := int(int16(g.exeU16(uint32(0xa1c66 + 2*i))))
		if id < 0 || id >= len(c.Buildings) || c.Buildings[id] == 0 {
			continue
		}
		if n == 0 {
			gap := int16(g.e.Text().Style + 3)
			*y += int(gap)
			b.h += gap
		}
		*y += int(lh)
		if !b.print {
			b.h += lh
		}
		name, err := g.buildingName(id)
		if err != nil {
			return n, err
		}
		if err := b.line(x, *y, name, 0); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

// drawPlanetInfoBox is step 8 of Draw_System_Display_Popup_: measure, the
// darkened box at (BX+L, BY+T), then print.
func (g *Game) drawPlanetInfoBox(bx, by, planet int) error {
	w, h, err := g.doPrintPlanetInfo(0, 0, planet, false)
	if err != nil {
		return err
	}
	g.drawDarkenedBox(bx+g.sysL(), by+g.sysT(), int(int16(w+9)), int(int16(h+4)), 0x3f)
	_, _, err = g.doPrintPlanetInfo(bx+g.sysL(), by+g.sysT(), planet, true)
	return err
}

// shipCounts is what N_Ships_By_Type_At_Location_ (0xa432f) counts.
type shipCounts struct {
	total, colony, outpost, transport int16
	hulls                             [6]int16 // combat ships by hull size
}

// nShipsByTypeAtLocation is N_Ships_By_Type_At_Location_ (0xa432f): the
// owner's ships with status < 3 at (x, y), by design type.
func (s *State) nShipsByTypeAtLocation(x, y int16, owner int) shipCounts {
	var r shipCounts
	for i := 0; i < int(s.NumShips); i++ {
		sh := &s.Ships[i]
		if int(sh.Owner) != owner || int8(sh.Status) >= 3 || sh.X != x || sh.Y != y {
			continue
		}
		switch sh.Design.Type {
		case 0:
			r.hulls[min(int(sh.Design.Size), 5)]++
		case 1:
			r.colony++
		case 2:
			r.transport++
		case 4:
			r.outpost++
		}
		r.total++
	}
	return r
}

// doPrintSystemShipData is Do_Print_System_Ship_Data_ (0xa26e3); bx, by are
// X+dx and Y+dy. Nodes from 5000 up name the ship of node−5000.
func (g *Game) doPrintSystemShipData(bx, by int, node int16, print bool) (int16, int16, error) {
	s, st, t := &g.state, &g.stacks, g.e.Text()
	b := &infoBox{g: g, print: print}
	style := 2
	x, y := bx+0x12, by+0x36
	if g.currentScreen == 0x1d {
		x, y = bx+8, by+0xa
	}
	var name string
	if node >= 5000 {
		if n := int(node) - 5000; n < maxNodes {
			name = save.CString(s.Ships[st.node[n].ship].Design.Name[:])
		}
	} else if node >= 0 && int(node) < maxNodes {
		if sh := &s.Ships[st.node[node].ship]; sh.Owner >= 8 {
			name = save.CString(sh.Design.Name[:])
		}
	}
	if name != "" {
		t.SetStyle(style, g.fontColors2(1, 0x6e, 0x73))
		t.SetSpacing(1)
		if print {
			return 0, 0, t.Print(int(int16(x)), int(int16(y)), name)
		}
		b.line(x, y, name, 0)
		return b.w, int16(t.Height()), nil
	}
	if node < 0 || int(node) >= maxNodes {
		return 0, 0, nil
	}
	if _, err := g.cached(buffer0LBX, 0x49); err != nil {
		return 0, 0, err
	}
	ship := &s.Ships[st.node[node].ship]
	owner := int(ship.Owner)
	pl := &s.Players[owner&7]
	c := mainPalettePlayerColors[pl.Color&7]
	colors := g.fontColors2(1, c, c+5)
	t.SetStyle(style, colors)
	t.SetSpacing(1)
	lh := int(uint8(t.Height() + 1))
	n := s.nShipsByTypeAtLocation(ship.X, ship.Y, owner)
	lines := 0
	for _, v := range []int16{n.colony, n.outpost, n.transport} {
		if v > 0 {
			lines++
		}
	}
	for _, v := range n.hulls {
		if v > 0 {
			lines++
		}
	}
	head := fmt.Sprintf(g.hMessage(0x163), save.CString(pl.RaceName[:]))
	t.SetStyle(style, colors)
	t.SetSpacing(1)
	if err := b.line(x, y, head, 0); err != nil {
		return 0, 0, err
	}
	ix, extra := x+style+1, int16(style+1)
	prev := head // the line buffer's last contents
	for i, v := range n.hulls {
		if v <= 0 {
			continue
		}
		hull, err := g.hullName(i, v != 1)
		if err != nil {
			return 0, 0, err
		}
		y += lh
		prev = fmt.Sprintf("%2d %s", v, hull)
		if err := b.line(ix, y, prev, extra); err != nil {
			return 0, 0, err
		}
	}
	for _, k := range []struct {
		n      int16
		one, m int
	}{{n.colony, 0x164, 0x165}, {n.outpost, 0x166, 0x167}, {n.transport, 0x168, 0x169}} {
		if k.n <= 0 {
			continue
		}
		var l string
		switch {
		case k.n == 1:
			l = fmt.Sprintf(g.hMessage(k.one), k.n)
		case k.one == 0x164:
			// No count is pushed for H 0x165: %d reads the line buffer's
			// first four bytes, which sit just above the arguments.
			b4 := []byte(prev + "\x00\x00\x00\x00")
			l = fmt.Sprintf(g.hMessage(k.m), int32(binary.LittleEndian.Uint32(b4)))
		default:
			l = fmt.Sprintf(g.hMessage(k.m), k.n)
		}
		y += lh
		if err := b.line(ix, y, l, extra); err != nil {
			return 0, 0, err
		}
	}
	return b.w, int16((lines + 1) * lh), nil
}

// drawShipInfoBox is the hovered branch of Draw_System_Ship_Icons_ (0xa3bc4):
// measure, the darkened box at (BX+L, BY+T), print.
func (g *Game) drawShipInfoBox(bx, by int, node int16) error {
	w, h, err := g.doPrintSystemShipData(bx, by, node, false)
	if err != nil {
		return err
	}
	g.drawDarkenedBox(bx+g.sysL(), by+g.sysT(), int(int16(w+9)), int(int16(h+4)), 0x3f)
	_, _, err = g.doPrintSystemShipData(bx, by, node, true)
	return err
}

// printGateInfo is Print_Gate_Info_ (0xa4989): gate 0 lists the emperors
// with a dimensional portal at the star, gate 3 the races with an Artemis net.
func (g *Game) printGateInfo(star, gate, bx, by int) error {
	s, t := &g.state, g.e.Text()
	x, y := int(int16(bx+g.sysL())), int(int16(by+g.sysT()))
	c := mainPalettePlayerColors[s.Players[s.PlayerNum].Color&7]
	t.SetStyle(1, g.fontColors2(1, c, c+6))
	var buf string
	width := 0
	list := func(h int, has func(st *save.Star, p int) bool, name func(p *save.Player) string) {
		buf = g.hMessage(h)
		width = t.Width([]byte(buf))
		st := s.starRecord(star)
		for p := 0; p < int(s.NumPlayers); p++ {
			if has(st, p) {
				buf += name(&s.Players[p]) + ", "
			}
		}
		if len(buf) >= 2 {
			buf = buf[:len(buf)-2]
		}
		buf += ")"
	}
	switch uint16(gate) {
	case 0:
		list(0x17c, func(st *save.Star, p int) bool { return maskBit(st.DimensionalPortal, p) },
			func(p *save.Player) string { return save.CString(p.EmperorName[:]) })
	case 1:
		buf = g.hMessage(0x17a)
		width = t.Width([]byte(buf))
	case 2:
		buf = g.hMessage(0x179)
		width = t.Width([]byte(buf))
	case 3:
		list(0x17b, func(st *save.Star, p int) bool { return maskBit(st.ArtemisNet, p) },
			func(p *save.Player) string { return save.CString(p.RaceName[:]) })
	}
	pw := int(int16(width + 2))
	// Measured with the formatted printer, printed with the simple one.
	mw, err := t.FormattedMaxWidth(pw, buf, 0)
	if err != nil {
		return err
	}
	mh, err := t.FormattedMaxHeight(pw, buf)
	if err != nil {
		return err
	}
	g.drawDarkenedBox(x, y, int(int16(mw+0xc)), int(int16(mh+5)), 0x3f)
	return t.PrintParagraph(int(int16(x+7)), int(int16(y+5)), pw, buf, 0)
}
