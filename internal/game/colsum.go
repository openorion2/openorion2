package game

import (
	"encoding/binary"
	"fmt"

	"github.com/openorion2/openorion2/internal/engine"
	"github.com/openorion2/openorion2/internal/font"
	"github.com/openorion2/openorion2/internal/gfx"
	"github.com/openorion2/openorion2/internal/save"
)

// The colony summary (screen 0x14, the COLONIES button): MOX\OTHERS\COLSUM.C
// with the COLONY.C routines it draws through. Spec:
// docs/screens/colony-summary.md.

// Addresses of COLSUM.C's and COLONY.C's data.
const (
	colsumLBX          = 0x182a16 // "COLSUM.LBX"
	colbldgLBX         = 0x1829f1
	colony2LBX         = 0x182984
	raceiconLBX        = 0x1829a9
	gstarLBX           = 0x179dd4 // "gstar.lbx"
	colsumHelpList     = 0x18329a // _colony_summary_screen_help_list, 0x16 entries
	colsumBuyX         = 0xc221f  // Add_Fields_'s ten buy button x and y
	colsumBuyY         = 0xc2233
	colsumLinkColors   = 0xc2253 // Colsum_Connect_Galaxy_Map_Stars_' six colours
	colsumEventGood    = 0x183160
	colsumWeeEvents    = 0x19fa2c // Load_Colsum_Strings_' slots, by event
	colsumRaceOrder    = 0xb4d47  // Do_Colony_Info_Pop_Stuff_For_Pop_'s race order
	colsumBuildingCost = 0x17eb45 // _buildings[].cost
	colorsBrightBlue   = 0x182c26 // _bright_blue_font_color_array
	colorsFont         = 0x182c2e // _font_color_array
	colorsFontBright   = 0x182c3e // _font_bright_color_array
	colorsRed          = 0x182c46 // _red_font_color_array, then its bright set
	strBlue1           = 0x179e0e // "\x1b1"
	strBlue2           = 0x179e11 // "\x1b2"
	strBlue0           = 0x179e14 // "\x1b0"
	strNothing         = 0x179e17 // ""
	strRedFmt          = 0x179e18
	strTab1            = 0x179e00 // "\x1a1"
	strTab0            = 0x179e03 // "\x1a0"
	strSortP           = 0x179e06 // "p"
	strSortF           = 0x179e08
	strSortS           = 0x179e0a
	strSpace           = 0x179e0c
	loadColsumStrings  = 0xc2259 // Load_Colsum_Strings_ .. Clear_First_
	loadColsumStrEnd   = 0xc248a
)

// colonyGlobals are the COLONY.C globals the colony summary shares with
// the colony screen.
type colonyGlobals struct {
	colonyN    int16 // _g_colony_n (0x182ab7)
	clusterN   int16 // _cluster_colony_n (0x182974)
	scannedPop int16 // _scanned_pop (0x182abc)
	stepSquish int16 // _step_squish (0x182ab9)
	fullDraw   bool  // _full_draw (0x182aca)
	// _g_hacky_dont_draw_cluster_flag (0x182976): set by the transport popup.
	dontDrawCluster bool
	count           int16 // _g_count (0x19f7b4)
}

// colonySummaryStatics are COLSUM.C's initialised statics: the scroll
// position and the sort survive leaving the screen.
type colonySummaryStatics struct {
	first, slider int16 // _first, _slider_bar_position
	sortIndex     int16 // _g_sort_index: name, population, food, industry, science, producing, BC
	savedScreen   int   // _colony_summary_saved_screen
}

// colonySummaryState is the rest of COLSUM.C's state while the screen runs.
type colonySummaryState struct {
	anims   [21]*gfx.Picture // _anims: COLSUM.LBX 0..0x14 over COLBLDG.LBX 0..6
	palAnim *gfx.Picture     // _colsum_pal_anim, COLSUM.LBX 0x15
	list    [250]int16       // the screen's local list _g_colony_list_ptr points at

	listCol  [10]int16 // _list_col
	buyData  [10]int16 // _list_buy_data
	listF    [10]int   // _list_fields
	prodF    [10]int   // _list_col_prod_fields
	buyF     [10]int   // _list_buy_fields
	jobF     [10][3]int
	popRef   [10][3][42]int16
	popCount [10][3]int16
	coord    [10][3]int16

	up, down, ret, esc, redraw, scroll, whole int
	sortBtn                                   [7]int

	drawPlanetSum bool // _draw_planet_sum

	fList, fListShift [11]int // port-only: build lists
}

func (g *Game) initColonyStatics() {
	cg := &g.col
	cg.clusterN = g.exeI16(0x182974)
	cg.colonyN = g.exeI16(0x182ab7)
	cg.stepSquish = g.exeI16(0x182ab9)
	cg.scannedPop = g.exeI16(0x182abc)
	cs := &g.csStat
	cs.first, cs.slider = g.exeI16(0x18315a), g.exeI16(0x18315c)
	cs.sortIndex = g.exeI16(0x183104)
}

// colsumEString is the E string Load_Colsum_Strings_ (0xc2259) stores in
// slot, read from its code: "mov eax, i ... call E_Strings_; mov [slot], eax".
func (g *Game) colsumEString(slot uint32) string {
	code := g.exe.bytes(loadColsumStrings, loadColsumStrEnd-loadColsumStrings)
	for i := 0; i+10 <= len(code); i++ {
		if code[i] != 0xe8 || code[i+5] != 0xa3 || binary.LittleEndian.Uint32(code[i+6:]) != slot {
			continue
		}
		for j := i - 5; j >= max(i-16, 0); j-- {
			if code[j] == 0xb8 {
				return g.eStr(int(binary.LittleEndian.Uint32(code[j+1:])))
			}
		}
	}
	g.logOnce(fmt.Sprintf("no colony summary string at %#x", slot))
	return ""
}

// eStr is E_Strings_(i).
func (g *Game) eStr(i int) string {
	es, err := g.eStrings()
	if err != nil || i < 0 || i >= len(es) {
		g.logOnce(fmt.Sprintf("E string %#x unavailable", i))
		return ""
	}
	return es[i]
}

func (g *Game) colors(addr uint32) font.Colors {
	var c font.Colors
	copy(c[:], g.exe.bytes(addr, 8))
	return c
}

// setColonyFontTo is Set_Colony_Font_To_ (0xbb04e).
func (g *Game) setColonyFontTo(style int) {
	t := g.e.Text()
	t.SetOutlineColor(0x50, 0x50)
	t.SetStyleShadow(style, g.colors(colorsFont), font.ShadowDown)
	t.SetHighlightColors(g.colors(colorsBrightBlue))
	t.SetSpecialColors(g.colors(colorsRed))
}

// setColonyFontToBlue is Set_Colony_Font_To_Blue_ (0xbaf84).
func (g *Game) setColonyFontToBlue(style int, bright bool) {
	t := g.e.Text()
	t.SetOutlineColor(0x50, 0x50)
	c, red := g.colors(colorsFont), g.colors(colorsRed)
	if bright {
		c, red = g.colors(colorsFontBright), g.colors(colorsRed+8)
	}
	t.SetStyleShadow(style, c, font.ShadowDown)
	t.SetHighlightColors(g.colors(colorsBrightBlue))
	t.SetSpecialColors(red)
}

// setColonyFont is Set_Colony_Font_ (0xbaefe).
func (g *Game) setColonyFont() { g.setColonyFontTo(3) }

// colonySummaryScreen is Colony_Summary_Screen_ (0xc4562), screen 0x14.
func (g *Game) colonySummaryScreen() error {
	e, cs, cg := g.e, &g.cs, &g.col
	*cs = colonySummaryState{}
	cs.drawPlanetSum = false
	if g.currentScreen != g.returnScreen && !g.bq.fromDesign && g.returnScreen != 0x19 {
		g.csStat.savedScreen = g.returnScreen
	}
	if err := g.loadColSumAnims(); err != nil {
		return err
	}
	g.buildGlobalColonyList(&cs.list)
	g.sortColList()
	if err := g.updateColList(); err != nil {
		return err
	}
	if cs.listCol[9] == -1 {
		g.csStat.first = 0
		for i := range cs.listCol {
			cs.listCol[i] = -1
		}
		if err := g.updateColList(); err != nil {
			return err
		}
		if err := g.updateColList(); err != nil {
			return err
		}
	}
	e.Text().SetStyleShadow(1, g.colors(colorsFont), font.ShadowDown)
	saved := e.GetAutoFunction()
	e.SetMouseList(engine.MouseNone)
	e.DeactivateAutoFunction()
	fade := g.fastFadeOutNeededForColonyPalette()
	if fade {
		if err := e.FastFadeOut(); err != nil {
			return err
		}
		e.V.Fill(0, 0, 0x27f, 0x1df, 0)
	}
	if err := e.LoadPalette(1, 0, 0xff); err != nil {
		return err
	}
	e.DrawPalette(cs.anims[0])
	e.UpdateGlassRemapColors()
	if fade {
		if err := e.DarkenPalette(100); err != nil {
			return err
		}
		e.V.ClearPaletteChanges(0, 0xff)
	}
	e.RefreshPalette()
	cg.colonyN = cs.listCol[0]
	g.ms.galaxyMap.colonyPalette = true
	cg.fullDraw = true
	g.ms.galaxyMapScannedStar = int16(g.colonyStar(int(cg.colonyN)))
	g.setColonySummaryScreenHelpList()
	e.AssignAutoFunction(g.drawColonySummaryScreen, 1)
	if err := g.drawColonySummaryScreen(); err != nil {
		return err
	}
	e.V.ClearPaletteChanges(0, 0xff)
	e.V.SetRefreshStencil()
	if err := e.DrawVisibleFields(); err != nil {
		return err
	}
	if err := e.TogglePages(); err != nil {
		return err
	}
	cg.fullDraw = false
	if err := e.FastFadeIn(); err != nil {
		return err
	}
	e.SetMouseList(engine.MouseArrow)
	e.V.SetRefreshStencil()
	if err := g.updateColList(); err != nil {
		return err
	}
	leave := func() {
		g.clearGalaxyMapFields()
		e.ClearFields()
		e.DeactivateAutoFunction()
	}
	for {
		e.MarkTime()
		r, err := e.GetInput()
		if err != nil {
			return err
		}
		scan, err := e.ScanInput()
		if err != nil {
			return err
		}
		ok, err := g.colsumEvaluateInput(&r, scan)
		if err != nil {
			return err
		}
		if !ok {
			leave()
			g.ms.galaxyMap.colonyPalette = false
			return nil
		}
		if err := g.drawColonySummaryScreen(); err != nil {
			return err
		}
		if cg.fullDraw {
			if err := g.updateColList(); err != nil {
				return err
			}
			cg.fullDraw = false
		}
		if err := e.TogglePages(); err != nil {
			return err
		}
		if err := e.ReleaseTime(1); err != nil {
			return err
		}
		if r == cs.ret {
			break
		}
	}
	leave()
	e.DeactivateHelpList()
	e.PutAutoFunction(saved)
	g.ms.galaxyMap.colonyPalette = false
	return nil
}

// colonyStar is the star of colony ci's planet; ci = -1 reads the record
// before the array, which the port does not model (star 0).
func (g *Game) colonyStar(ci int) int {
	s := &g.state
	if ci < 0 {
		return 0
	}
	return int(s.Planets[s.Colonies[ci].Planet].Star)
}

// fastFadeOutNeededForColonyPalette is
// Fast_Fade_Out_Needed_For_Colony_Palette_ (0xc5c85).
func (g *Game) fastFadeOutNeededForColonyPalette() bool {
	// _hacky_need_to_fade_to_colony_palette_flag is only set by screens not
	// ported yet.
	p := g.previousScreen
	switch c := g.currentScreen; {
	case c < 0x14:
		return c != 1 || (p != 0x19 && p != 0x21)
	case c == 0x14:
		return p != 0x19
	case c < 0x19 || c > 0x19:
		return true
	}
	return p != 1 && p != 0x21 && p != 0x14
}

// setColonySummaryScreenHelpList is Set_Colony_Summary_Screen_Help_List_
// (0xcdb18).
func (g *Game) setColonySummaryScreenHelpList() {
	g.e.DeactivateHelpList()
	g.e.SetHelpList(g.helpListAt(colsumHelpList, 0x16))
}

// loadColSumAnims is Load_Col_Sum_Anims_ (0xc24ca) with Load_Build_Anims_
// (0xc0a24) and Load_Galaxy_Map_Anims_ (0xcb91d).
func (g *Game) loadColSumAnims() error {
	cs, gm := &g.cs, &g.ms.galaxyMap
	var err error
	for i := 0; i < 7 && err == nil; i++ {
		cs.anims[i], err = g.picture(cStringAt(g.exe, colbldgLBX), i)
	}
	if err != nil {
		return err
	}
	// _bldg_bitm is allocated here for the colony screen's buildings.
	if cs.palAnim, err = g.picture(cStringAt(g.exe, colsumLBX), 0x15); err != nil {
		return err
	}
	for i := range cs.anims {
		if cs.anims[i], err = g.picture(cStringAt(g.exe, colsumLBX), i); err != nil {
			return err
		}
	}
	for i := 0; i < 10; i++ {
		if gm.colsumStarSeg[i], err = g.picture(cStringAt(g.exe, gstarLBX), 0x17+i); err != nil {
			return err
		}
	}
	return nil
}

// buildGlobalColonyList is Build_Global_Colony_List_ (0xc58d1): the
// player's colonies, not outposts, then -1 up to 250, into the list
// _g_colony_list_ptr points at.
func (g *Game) buildGlobalColonyList(list *[250]int16) {
	s := &g.state
	n := 0
	for i := 0; i < int(s.NumColonies); i++ {
		c := &s.Colonies[i]
		if int(c.Owner) == int(s.PlayerNum) && c.IsOutpost == 0 {
			list[n] = int16(i)
			n++
		}
	}
	for ; n < len(list); n++ {
		list[n] = -1
	}
}

// sortColList is Sort_Col_List_ (0xc3947): 250 bubble passes.
func (g *Game) sortColList() {
	l := &g.cs.list
	for k := 0; k < 0xfa; k++ {
		for i := 0; i < 0xf9; i++ {
			if g.switchedCmp(l[i], l[i+1]) > 0 {
				l[i], l[i+1] = l[i+1], l[i]
			}
		}
	}
}

// switchedCmp is Switched_cmp_ (0xc386b) as Sort_Col_List_ reads it (a short).
func (g *Game) switchedCmp(a, b int16) int16 {
	if a == -1 || b == -1 || g.csStat.sortIndex >= 7 {
		return 0
	}
	s := &g.state
	A, B := &s.Colonies[a], &s.Colonies[b]
	var x, y int16
	switch g.csStat.sortIndex {
	case 0:
		return int16(g.cmpAlpha(int(a), int(b)))
	case 1:
		x, y = int16(A.PopCount), int16(B.PopCount)
	case 2, 3, 4, 6:
		k := []int{2: save.Food, 3: save.Industry, 4: save.Research, 6: save.BC}[g.csStat.sortIndex]
		x, y = A.Output[k], B.Output[k]
	case 5:
		return g.cmpProd(int(a), int(b))
	}
	return -cmpShort(x, y)
}

// cmpShort is cmp_ (0xc34dd).
func cmpShort(a, b int16) int16 {
	switch {
	case a < b:
		return -1
	case b < a:
		return 1
	}
	return 0
}

// cmpAlpha is cmp_Alpha_ (0xc34f3): stricmp of the planet names.
func (g *Game) cmpAlpha(a, b int) int {
	s := &g.state
	return stricmp(g.planetNameNoPrime(int(s.Colonies[a].Planet)), g.planetNameNoPrime(int(s.Colonies[b].Planet)))
}

// stricmp is Watcom's stricmp_: the difference of the first unequal pair
// of lower-cased characters.
func stricmp(a, b string) int {
	lower := func(c byte) int {
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		return int(c)
	}
	for i := 0; ; i++ {
		var x, y byte
		if i < len(a) {
			x = a[i]
		}
		if i < len(b) {
			y = b[i]
		}
		if lower(x) != lower(y) || x == 0 {
			return lower(x) - lower(y)
		}
	}
}

// strcmp is Watcom's strcmp_ as callers read it: the sign of the first
// unequal byte pair.
func strcmp(a, b string) int {
	for i := 0; ; i++ {
		var x, y byte
		if i < len(a) {
			x = a[i]
		}
		if i < len(b) {
			y = b[i]
		}
		if x != y {
			if x < y {
				return -1
			}
			return 1
		}
		if x == 0 {
			return 0
		}
	}
}

// prodToSortType is Prod_To_Sort_Type_ (0xc3555).
func (g *Game) prodToSortType(p int16) int16 {
	switch {
	case p < -99:
		return int16(g.state.Ships[-(int(p)+100)].Design.Size) + 10000
	case p == -0xf:
		return 9999
	case p > 0 && p < 0x31:
		return g.exeI16(colsumBuildingCost+uint32(p)*0x13) + 10
	case p == -6 || p == -4 || p == -5 || p == -7:
		return -1
	case p == -3 || p == -2 || p == -10:
		return -2
	case p == -1:
		return -1
	}
	return 0
}

// cmpProd is cmp_Prod_ (0xc3614): by product kind, name, then turns.
func (g *Game) cmpProd(a, b int) int16 {
	s := &g.state
	A, B := &s.Colonies[a], &s.Colonies[b]
	r := cmpShort(g.prodToSortType(A.Queue[0]), g.prodToSortType(B.Queue[0]))
	if r == 0 {
		r = int16(strcmp(g.selectionName(A.Queue[0]), g.selectionName(B.Queue[0])))
		if r == 0 {
			return cmpShort(g.calculateCurrentProductionTurnCount(a), g.calculateCurrentProductionTurnCount(b))
		}
	}
	return -r
}

// selectionName is Selection_Name_ (0xc2ffa).
func (g *Game) selectionName(p int16) string {
	n, err := g.productName(int(p))
	if err != nil {
		g.logOnce(err.Error())
	}
	return n
}

// calculateCurrentProductionTurnCount is
// Calculate_Current_Production_Turn_Count_ (0xcdc93).
func (g *Game) calculateCurrentProductionTurnCount(ci int) int16 {
	c := &g.state.Colonies[ci]
	n := g.colonyNTurnsToProduce(c)
	if c.Pending != 0 {
		return 1
	}
	if n == -1 {
		return 10000
	}
	return n
}

// updateColList is Update_Col_List_ (0xc3996): the visible rows from
// _first, then the fields again.
func (g *Game) updateColList() error {
	cs := &g.cs
	for i := 0; i < 10; i++ {
		c := cs.list[int(g.csStat.first)+i]
		cs.listCol[i] = c
		cs.buyData[i] = 0
		if c != -1 {
			cs.buyData[i] = g.state.Colonies[c].Pending
		}
	}
	g.e.ClearFields()
	g.clearGalaxyMapFields()
	return g.colsumAddFields()
}

// updateFirst is Update_First_ (0xc2685): keep _first in step with the bar.
func (g *Game) updateFirst(n int16) {
	st := &g.csStat
	cnt := int16(g.nColonies(int(g.state.PlayerNum)))
	if cnt < n {
		st.slider, st.first = -1, 0
		return
	}
	if st.slider != cnt-n-st.first {
		g.col.fullDraw = true
		g.col.scannedPop = -1
		st.first = cnt - n - st.slider
	}
}

// incrementFirst is Increment_First_ (0xc42b4): one row up.
func (g *Game) incrementFirst(n int16) {
	st := &g.csStat
	cnt := int16(g.nColonies(int(g.state.PlayerNum)))
	if n <= cnt {
		st.first = max(st.first-1, 0)
		st.slider = cnt - n - st.first
		g.col.fullDraw = true
	}
}

// colsumAddFields is Add_Fields_ (0xc2e4d).
func (g *Game) colsumAddFields() error {
	e, cs := g.e, &g.cs
	n := g.nColonies(int(g.state.PlayerNum))
	var err error
	add := func(dst *int, fn func() (int, error)) {
		if err == nil {
			*dst, err = fn()
		}
	}
	add(&cs.up, func() (int, error) { return e.AddButtonField(0x26b, 0xf, g.eStr(0xc), cs.anims[2], g.eStr(0x89), 0x28) })
	add(&cs.down, func() (int, error) {
		return e.AddButtonField(0x26b, 0x13c, g.eStr(0xc), cs.anims[3], g.eStr(0xb), 0x28)
	})
	add(&cs.ret, func() (int, error) {
		return e.AddButtonField(0x213, 0x1bd, g.eStr(0xc), cs.anims[1], g.eStr(0xc), 0x28)
	})
	add(&cs.esc, func() (int, error) { return e.AddHotKey(g.eStr(0x87)) })
	sorts := []struct {
		x   int
		hot string
	}{
		{0x59, g.eStr(0x180)}, {0x8c, cStringAt(g.exe, strSortP)}, {0xdb, cStringAt(g.exe, strSortF)},
		{0x106, g.eStr(0x147)}, {0x146, cStringAt(g.exe, strSortS)}, {0x189, g.eStr(0x1b4)}, {0x1e0, g.eStr(0xc1)},
	}
	for i, so := range sorts {
		add(&cs.sortBtn[i], func() (int, error) {
			return e.AddMultiButtonField(so.x, 0x1be, g.eStr(0xc), cs.anims[4+i], &g.csStat.sortIndex, int16(i), so.hot, 0x29)
		})
	}
	if n < 10 {
		cs.scroll = engine.NoField
	} else {
		add(&cs.scroll, func() (int, error) {
			return e.AddScrollField(0x26d, 0x28, 0, n, 0, n-10, 5, 0x10f, &g.csStat.slider, g.eStr(0xc), 0)
		})
	}
	add(&cs.redraw, func() (int, error) { return e.AddHotKey(g.eStr(0x254)) })
	if err != nil {
		return err
	}
	if err := g.addBuildListKeys(&cs.fList, &cs.fListShift); err != nil {
		return err
	}
	y1, y2 := 0x23, 0x41
	for i := 0; i < 10; i++ {
		c := cs.listCol[i]
		if c == -1 {
			cs.listF[i], cs.prodF[i], cs.buyF[i] = engine.NoField, engine.NoField, engine.NoField
		} else {
			a, b := y1, y2
			nameKey, prodKey := g.eStr(0xc), g.eStr(0xc)
			add(&cs.listF[i], func() (int, error) { return e.AddHiddenField(0xc, a, 0x65, b, nameKey, 0) })
			add(&cs.prodF[i], func() (int, error) { return e.AddHiddenField(0x200, a, 0x255, b, prodKey, 0) })
			x := int(g.exeI16(colsumBuyX + 2*uint32(i)))
			y := int(g.exeI16(colsumBuyY + 2*uint32(i)))
			pic := cs.anims[0xb+i]
			if !g.colonyCanBuyProduct0(int(c)) && g.state.Colonies[c].Pending == 0 {
				add(&cs.buyF[i], func() (int, error) { return e.AddHiddenField(x, y, x+pic.Width, y+pic.Height, g.eStr(0xc), 0) })
			} else {
				add(&cs.buyF[i], func() (int, error) {
					return e.AddLockedButtonField(x, y, g.eStr(0xc), pic, &cs.buyData[i], g.eStr(0xc), 0x29)
				})
			}
		}
		y1 += 0x1f
		y2 += 0x1f
		if err != nil {
			return err
		}
		if err := g.addFieldsPopFor(i); err != nil {
			return err
		}
	}
	add(&cs.whole, func() (int, error) { return e.AddHiddenField(0, 0, 0x27f, 0x1df, cStringAt(g.exe, strSpace), 0) })
	return err
}

// colonyCanBuyProduct0 is Colony_Can_Buy_Product_0_ (0xcefa8).
func (g *Game) colonyCanBuyProduct0(ci int) bool {
	s := &g.state
	c := &s.Colonies[ci]
	if !g.colonyCanAffordToBuyProduct(ci) || c.Pending != 0 {
		return false
	}
	if s.Stars[g.colonyStar(ci)].Blockaded>>s.PlayerNum&1 != 0 {
		return false
	}
	return g.colonyCostToBuyProduct(c) > 0
}

// colonyCanAffordToBuyProduct is Colony_Can_Afford_To_Buy_Product_ (0xe10ea).
func (g *Game) colonyCanAffordToBuyProduct(ci int) bool {
	s := &g.state
	c := &s.Colonies[ci]
	return g.colonyCostToBuyGivenProduct(c, c.Queue[0], int32(c.Production)) <= s.Players[c.Owner].BC
}

// jobX are the left edges of the farmer, worker and scientist columns
// (and the building column after them).
var jobX = [4]int16{0x65, 0xec, 0x17a, 0x200}

// popStuff calls Do_Colony_Info_Pop_Stuff_For_Pop_ for row i's job column.
func (g *Game) popStuff(i, job, mode int, y int16) (int16, error) {
	cs := &g.cs
	return g.doColonyInfoPopStuffForPop(int(cs.listCol[i]), job, mode, jobX[job], y, jobX[job+1]-10, false,
		cs.popRef[i][job][:], &cs.popCount[i][job], &cs.coord[i][job], &cs.jobF[i][job])
}

// addFieldsPopFor is Add_Fields_Pop_For_ (0xc2b72).
func (g *Game) addFieldsPopFor(i int) error {
	cs := &g.cs
	for job := 0; job < 3; job++ {
		if cs.listCol[i] == -1 {
			cs.jobF[i][job] = engine.NoField
			continue
		}
		if _, err := g.popStuff(i, job, 1, int16(i*0x1f+0x22)); err != nil {
			return err
		}
	}
	return nil
}

// drawInfoPopFor is Draw_Info_Pop_For_ (0xc28a8).
func (g *Game) drawInfoPopFor(i int) error {
	for job := 0; job < 3; job++ {
		if _, err := g.popStuff(i, job, 0, int16(i*0x1f+0x26)); err != nil {
			return err
		}
	}
	return nil
}

// getScannedPop is Get_Scanned_Pop_ (0xc2994): the colonist under the
// pointer in row i's job column; the row's colony becomes the scanned one.
func (g *Game) getScannedPop(i, job int) (int16, error) {
	p, err := g.popStuff(i, job, 4, int16(i*0x1f+0x26))
	c := g.cs.listCol[i]
	g.col.colonyN = c
	g.ms.galaxyMapScannedStar = int16(g.colonyStar(int(c)))
	return p, err
}

// getSelectedPop is Get_Selected_Pop_ (0xc2a9a).
func (g *Game) getSelectedPop(i, job int) (int16, error) {
	return g.popStuff(i, job, 3, int16(i*0x1f+0x26))
}

// colonist is colony ci's colonist k.
func (g *Game) colonist(ci, k int) save.Colonist {
	return save.Colonist(g.state.Colonies[ci].Colonists[k])
}

// popToPopState is Pop_To_Pop_State_ (0xccacf): 3 for race 9, 4 for 8,
// else 2.
func (g *Game) popToPopState(ci, k int) int {
	switch g.colonist(ci, k) & 0xf {
	case 9:
		return 3
	case 8:
		return 4
	}
	return 2
}

// getEffectivePopPlayer is Get_Effective_Pop_Player_ (0xcadcc): races 8
// and 9 count as the colony's owner.
func (g *Game) getEffectivePopPlayer(ci, k int) int {
	r := int(g.colonist(ci, k) & 0xf)
	if r > 7 && r < 10 {
		return int(g.state.Colonies[ci].Owner)
	}
	return r
}

// colonyPopAnim is Colony_Pop_Anim_ (0xcb4a9).
func (g *Game) colonyPopAnim(ci, k int) (*gfx.Picture, error) {
	c := uint32(g.colonist(ci, k))
	state := g.popToPopState(ci, k)
	race := int(g.state.Players[g.getEffectivePopPlayer(ci, k)].Race)
	if c&0x400 == 0 {
		return g.peopleAnim(int(c&0x1ff)>>7, state, race)
	}
	return g.cached(cStringAt(g.exe, raceiconLBX), race*0xd+0xc)
}

// peopleAnim is People_Anim_ (0xbb54d).
func (g *Game) peopleAnim(job, state, race int) (*gfx.Picture, error) {
	var n int
	switch state {
	case 0:
		n = job*2 + race*0xd
	case 2:
		n = job*2 + race*0xd + 1
	case 3:
		n = 0xaa
	case 4:
		n = 0xa9
	default:
		return nil, nil
	}
	return g.cached(cStringAt(g.exe, raceiconLBX), n)
}

// calculateSquishStep is Calculate_Squish_Step_ (0xc4e64): how much n
// icons of width w overlap to fit between x1 and x2.
func (g *Game) calculateSquishStep(x1, x2, n, w int16) {
	room := x2 - x1 - w/3
	d := int(max(n, 1))
	step := int16(1)
	if int(room)/d >= 2 {
		step = int16(int(room) / d)
	}
	g.col.stepSquish = 0
	if step < w {
		g.col.stepSquish = w - step
	}
}

// globalCycler is Global_Cycler_ (0xd5d97): clock_() / period % n, clock_
// counting hundredths of a second.
func (g *Game) globalCycler(period, n int) int {
	return int(g.e.Sys.Elapsed().Milliseconds()/10) / period % n
}

// doColonyInfoPopStuffForPop is Do_Colony_Info_Pop_Stuff_For_Pop_
// (0xc4ef6): walk colony ci's colonists of job (-1 all) in display order
// (state, flag, race 9 then 0..8) and draw them (mode 0), list them (1),
// count them (2), or find the one at x (3 by *coord, 4 by the pointer).
func (g *Game) doColonyInfoPopStuffForPop(ci, job, mode int, x1, y, x2 int16, shadows bool, ref []int16, count, coord *int16, field *int) (int16, error) {
	e, s, cg := g.e, &g.state, &g.col
	if mode != 2 {
		if _, err := g.doColonyInfoPopStuffForPop(ci, job, 2, x1, y, x2, shadows, ref, count, coord, field); err != nil {
			return 0, err
		}
	}
	from, to := job, job+1
	if job == -1 {
		from, to = 0, 3
	}
	c := &s.Colonies[ci]
	if mode == 0 && job == 0 && c.CanFarm == 0 {
		g.setColonyFontTo(3)
		yy, h := y+5, 0x1c
		if g.settings[0xd2] == 1 && g.currentScreen == ScreenColonySummary {
			yy, h = y-1, 0x22
		}
		if err := e.Text().SqueezePrintParagraph(int(x1), int(yy), int(x2-x1), h, g.eStr(0x183), 2, g.exe.bytes(spaceLimits, 6), false); err != nil {
			return 0, err
		}
	}
	n := int16(0)
	step := func(k int16) int { return int(x1) + int(0x1e-cg.stepSquish)*int(k) }
	for state := 0; state < 7; state++ {
		for flag := 0; flag < 2; flag++ {
			for j := from; j < to; j++ {
				for o := 0; o < 10; o++ {
					race := g.exeI16(colsumRaceOrder + 2*uint32(o))
					for k := 0; k < int(c.PopCount); k++ {
						p := uint32(c.Colonists[k])
						if int16(p&0xf) != race || g.popToPopState(ci, k) != state || int(p>>7&3) != j || p&0x200 == 0 || int(p>>10&1) != flag {
							continue
						}
						switch mode {
						case 0:
							g.setColonyFontTo(2)
							if int16(k) == cg.scannedPop && cg.colonyN == int16(ci) && g.globalCycler(0x1e, 2) != 0 {
								if err := g.drawPopDark(step(n), int(y), ci, k); err != nil {
									return 0, err
								}
							} else {
								pic, err := g.colonyPopAnim(ci, k)
								if err != nil {
									return 0, err
								}
								if err := e.Draw(step(n), int(y), pic); err != nil {
									return 0, err
								}
							}
							n++
						case 1:
							ref[n] = int16(k)
							n++
						case 2:
							n++
						case 3:
							if int(*coord) <= step(n+1) || n == *count {
								return ref[n], nil
							}
							n++
						case 4:
							px, _, _ := e.Sys.Mouse()
							if px <= step(n+1) || n == *count {
								return ref[n], nil
							}
							n++
						}
					}
				}
			}
		}
	}
	switch mode {
	case 0:
		if shadows && job == 0 && c.CanFarm != 0xff && n < int16(c.CanFarm) {
			sh, err := g.createShadowFarmer()
			if err != nil {
				return 0, err
			}
			for n < int16(c.CanFarm) {
				if err := e.Draw(step(n), int(y), sh); err != nil {
					return 0, err
				}
				n++
			}
		}
		cg.stepSquish = 0
	case 1:
		*count = n - 1
		f, err := e.AddScrollField(int(x1), int(y), int(x1), int(x2)+8, int(x1), int(x2), int(x2-x1)+8, 0x1e, coord, g.eStr(0xc), 0)
		if err != nil {
			return 0, err
		}
		*field = f
		cg.stepSquish = 0
	case 2:
		if shadows && job == 0 && c.CanFarm != 0xff {
			n = int16(c.CanFarm)
		}
		g.calculateSquishStep(x1, x2, n, 0x1e)
	default:
		cg.stepSquish = 0
	}
	return -1, nil
}

// createShadowFarmer is Create_Shadow_Farmer_ (0xc0b09): a farmer of the
// player's race turned into a glassed silhouette with an outline, for the
// farm places not taken.
func (g *Game) createShadowFarmer() (*gfx.Picture, error) {
	s := &g.state
	p, err := g.peopleAnim(0, 2, int(s.Players[s.PlayerNum].Race))
	if err != nil {
		return nil, err
	}
	bm := gfx.NewBlankBitmap(0x1e, 0x1e)
	bm.DrawPictureToBitmap(p)
	bm.ReplaceColorAll(0xf0)
	bm.SetGlassed()
	bm.Outline(bm, 0xe5, false)
	g.cscr.shadow = bm
	return bm, nil
}

// drawPopDark is Draw_Pop_Dark_ (0xca90b): the colonist tinted on a 30x30
// bitmap.
func (g *Game) drawPopDark(x, y, ci, k int) error {
	pic, err := g.colonyPopAnim(ci, k)
	if err != nil {
		return err
	}
	bm := gfx.NewBlankBitmap(0x1e, 0x1e)
	if err := bm.AddPicture(0, 0, pic); err != nil {
		return err
	}
	g.e.TintBitmap(bm, 2)
	return g.e.Draw(x, y, bm)
}

// drawColonySummaryScreen is Draw_Colony_Summary_Screen_ (0xc3d34), the
// auto function: the frame and empire totals on a full draw, the map, the
// rows, the scanned colony, the bar.
func (g *Game) drawColonySummaryScreen() error {
	e, cs, cg := g.e, &g.cs, &g.col
	v := e.V
	cg.count++
	v.SetPageOff()
	v.SetWindow(0, 0, 0x27f, 0x1df)
	v.ClipOn()
	g.updateFirst(10)
	if cg.fullDraw {
		e.Text().SetStyleShadow(1, g.colors(colorsFont), font.ShadowDown)
		v.Fill(0, 0, 0x27f, 0x1df, 0)
		if err := e.Draw(0, 0, cs.anims[0]); err != nil {
			return err
		}
		for i := range cs.listCol {
			cs.listCol[i] = -1
		}
		if err := g.updateColList(); err != nil {
			return err
		}
		if err := g.drawEmpireInfo(); err != nil {
			return err
		}
	}
	if err := g.drawGalaxyMapBox(nil, 0x17c, 0x15d, 0x80, 0x5b, 0, 0, 0, 0, 3, 0, 0); err != nil {
		return err
	}
	for i := range cs.listCol {
		if cs.listCol[i] != -1 {
			if err := g.drawColonySummaryForColony(i); err != nil {
				return err
			}
		}
	}
	if cg.fullDraw {
		if !e.DrawAllFields() {
			e.EnableDrawAllFields()
			if err := e.DrawVisibleFields(); err != nil {
				return err
			}
			e.DisableDrawAllFields()
		} else if err := e.DrawVisibleFields(); err != nil {
			return err
		}
		v.CopyOffToBack()
	}
	if err := g.drawColonyScanInfo(); err != nil {
		return err
	}
	g.setColonyFontTo(1)
	if st := g.ms.galaxyMapScannedStar; st != -1 {
		if err := e.Text().PrintCentered(0x1bc, 0x1af, save.CString(g.state.Stars[st].Name[:])); err != nil {
			return err
		}
	}
	if cg.clusterN != -1 && cg.colonyN != -1 {
		g.colsumConnectGalaxyMapStars(g.colonyStar(int(cg.colonyN)), g.colonyStar(int(cg.clusterN)))
	}
	if cs.drawPlanetSum {
		pl := g.state.Stars[g.screenData[0]].Planets[g.screenData[1]]
		if err := g.drawPlanetSummary(0xe5, 0x87, int(pl)); err != nil {
			return err
		}
	}
	g.drawBarIndicator()
	// Draw_Fake_Buttons_: nothing adds fake buttons on this screen.
	if err := e.DrawVisibleFields(); err != nil {
		return err
	}
	px, py, _ := e.Sys.Mouse()
	if err := g.drawCluster(px, py); err != nil {
		return err
	}
	v.ClipOff()
	return nil
}

// drawEmpireInfo is Draw_Empire_Info_ (0xc26f4): reserve, income,
// population, freighters, food and research, one line each.
func (g *Game) drawEmpireInfo() error {
	s := &g.state
	p := &s.Players[s.PlayerNum]
	g.setColonyFontTo(2)
	t1, t0 := cStringAt(g.exe, strTab1), cStringAt(g.exe, strTab0)
	red := cStringAt(g.exe, strRedFmt)
	if p.NetIncome >= 0 {
		red = g.eStr(0xc)
	}
	buf := ""
	line := func(n int, f string, args ...any) {
		l := cSprintf(f, args...)
		if n == 0 {
			buf = l
			return
		}
		buf = cSprintf(g.eStr(0x47), buf, 0xd, l)
	}
	line(0, g.eStr(0x76), t0, t1, int(p.BC))
	line(1, g.eStr(0x6a), t0, t1, red, int(p.NetIncome))
	line(2, g.eStr(0x72), t0, t1, int(p.Population))
	line(3, g.eStr(0x67), t0, t1, int(p.FreightersFree))
	line(4, g.eStr(0x66), t0, t1, int(p.FoodSurplus))
	line(5, g.eStr(0x75), t0, t1, int(p.ResearchProduced))
	if err := g.e.Text().PrintFormatted(0x208, 0x162, 0x68, buf, 3); err != nil {
		return err
	}
	g.setColonyFont()
	return nil
}

// drawColonySummaryForColony is Draw_Colony_Summary_For_Colony_ (0xc3111):
// row i's name (with its event, or blockade colour) and, on a full draw,
// its colonists and what it builds.
func (g *Game) drawColonySummaryForColony(i int) error {
	e, cs, cg, s := g.e, &g.cs, &g.col, &g.state
	t := e.Text()
	ci := cs.listCol[i]
	c := &s.Colonies[ci]
	t.SetOutlineColor(0x50, 0x50)
	if ci == cg.colonyN {
		if err := g.drawInfoPopFor(i); err != nil {
			return err
		}
	}
	ev := s.colonyHasEvent(ci)
	g.setColonyFontToBlue(2, ci == cg.colonyN)
	name := g.planetNameNoPrime(int(c.Planet))
	y := i*0x1f + 0x26
	var msg string
	w := 0x59
	if ev != -1 {
		col := strBlue2
		if g.exe.bytes(colsumEventGood+uint32(ev), 1)[0] != 0 {
			col = strBlue1
		}
		pre := cSprintf(g.eStr(0x46), cStringAt(g.exe, uint32(col)))
		msg = cSprintf(g.eStr(0x55), name, pre, g.colsumEString(colsumWeeEvents+4*uint32(ev)))
		w = 0x57
	} else {
		pre, post := g.eStr(0xc), g.eStr(0xc)
		if s.Stars[g.colonyStar(int(ci))].Blockaded>>s.PlayerNum&1 != 0 {
			pre, post = cSprintf(g.eStr(0x46), cStringAt(g.exe, strBlue2)), g.eStr(0x86)
		}
		msg = cSprintf(g.eStr(0x51), cSprintf(pre), name, cSprintf(post))
	}
	if err := t.SqueezePrintFormattedParagraph(0xc, 0x17/2+y, w, 0x17, msg, 0, g.exe.bytes(spaceLimits, 6), true); err != nil {
		return err
	}
	if !cg.fullDraw {
		return nil
	}
	t.SetStyleShadow(0, g.colors(colorsFont), font.ShadowDown)
	if err := g.drawInfoPopFor(i); err != nil {
		return err
	}
	turns := g.calculateCurrentProductionTurnCount(int(ci))
	g.setColonyFontTo(1)
	q := c.Queue[0]
	b1, b0 := cStringAt(g.exe, strBlue1), cStringAt(g.exe, strBlue0)
	switch {
	case q == -1 && c.AutoBuild == 0:
		return nil
	case q == -1:
		msg = cSprintf(g.eStr(0x5d), b1, b0, g.selectionName(q))
	case turns > 0 && turns < 10000 && c.AutoBuild != 0:
		msg = cSprintf(g.eStr(0x5c), b1, b0, g.selectionName(q), int(turns))
		g.setColonyFontToBlue(2, false)
	case turns > 0 && turns < 10000:
		msg = cSprintf(g.eStr(0x27), g.selectionName(q), int(turns))
		g.setColonyFontToBlue(2, false)
	case c.AutoBuild != 0:
		msg = cSprintf(g.eStr(0x5d), b1, b0, g.selectionName(q))
		g.setColonyFontToBlue(2, false)
	default:
		msg = cSprintf(g.eStr(0x46), g.selectionName(q))
		g.setColonyFontToBlue(2, false)
	}
	if err := t.SqueezePrintFormattedParagraph(0x200, y, 0x55, 0x16, msg, 0, g.exe.bytes(spaceLimits, 6), false); err != nil {
		return err
	}
	t.SetStyleShadow(1, g.colors(colorsFont), font.ShadowDown)
	return nil
}

// drawColonyScanInfo is Draw_Colony_Scan_Info_ (0xc3b3c): the scanned
// colony's output icons, morale and planet description.
func (g *Game) drawColonyScanInfo() error {
	s, cg := &g.state, &g.col
	ci := int(cg.colonyN)
	if ci == -1 {
		return nil
	}
	c := &s.Colonies[ci]
	for k := 0; k < 4; k++ {
		if err := g.drawColonyProdBoth(ci, k, 0x6a, int16(0x15d+0x12*k), 0x16e, 0x14, true); err != nil {
			return err
		}
	}
	if err := g.drawInfoMoraleBoth(ci, 0x6a, 0x1a5, 0x16e, true); err != nil {
		return err
	}
	g.setColonyFontTo(1)
	growth := int16(0)
	for _, x := range c.PopGrowth {
		growth += x
	}
	// Sum_Colonists_ runs here for its count, which nothing reads.
	pl := &s.Planets[c.Planet]
	col := cStringAt(g.exe, strNothing)
	if growth < 0 {
		col = cStringAt(g.exe, strBlue2)
	}
	msg := cSprintf(g.eStr(0x4a), col, g.psEString(planetSizeNames, int(pl.Size)),
		g.psEString(planetClimateNames, int(c.Climate)), g.psEString(planetGravityNames, int(pl.Gravity)),
		g.psEString(planetMineralNames, int(pl.Minerals)), int(c.PopCount),
		int(g.planetMaxPopulationForPlayer(int(c.Planet), int(s.PlayerNum))), col, int(growth), cStringAt(g.exe, strBlue0))
	return g.e.Text().SqueezePrintFormattedParagraph(0xd, 0x162, 0x50, 0x58, msg, 0, g.exe.bytes(spaceLimits, 6), false)
}

// colonyOutputExtra is a colony's word at 0xf3 + 2k: food imported,
// industry paid, research (unused), BC deficit.
func colonyOutputExtra(c *save.Colony, k int) int16 {
	switch k {
	case 0:
		return c.FoodImport
	case 1:
		return c.IndustryPaid
	case 2:
		return c.Unknown0F7
	}
	return c.BCDeficit
}

func abs16(v int16) int16 {
	if v < 0 {
		return -v
	}
	return v
}

// colony2Pic is a picture of COLONY2.LBX through the global cache.
func (g *Game) colony2Pic(n int) (*gfx.Picture, error) {
	return g.cached(cStringAt(g.exe, colony2LBX), n)
}

// prodAnims, importAnims are Prod_Anims_ (0xcbc46) and Import_Anims_
// (0xcbc6f): the output icon of kind k, ten (big) or one, small (wee).
func (g *Game) prodAnims(k, big int, wee bool) (*gfx.Picture, error) {
	n := big*4 + k
	if wee {
		n += 0x1b
	}
	return g.colony2Pic(n)
}

func (g *Game) importAnims(k, big int, wee bool) (*gfx.Picture, error) {
	n := big*4 + k + 8
	if wee {
		n = big*4 + k + 0x23
	}
	return g.colony2Pic(n)
}

// shortAnims is Short_Anims_ (0xcbc8b): the import icon outlined on a
// 30x30 bitmap.
func (g *Game) shortAnims(k, big int, wee bool) (*gfx.Picture, error) {
	p, err := g.importAnims(k, big, wee)
	if err != nil {
		return nil, err
	}
	bm := gfx.NewBlankBitmap(0x1e, 0x1e)
	if err := bm.AddPicture(1, 1, p); err != nil {
		return nil, err
	}
	bm.Outline(bm, 0xed, false)
	return bm, nil
}

// drawColonyProdBoth is Draw_Colony_Prod_Both_ (0xc55cf): colony ci's
// output k as rows of tens and ones, imports and upkeep shown apart.
func (g *Game) drawColonyProdBoth(ci, k int, x, y, xEnd, w int16, wee bool) error {
	e, cg := g.e, &g.col
	c := &g.state.Colonies[ci]
	imp := colonyOutputExtra(c, k)
	var tens2c, ones34, tens24, ones14 int16
	if k != 1 {
		tens2c, ones34 = abs16(imp)/10, abs16(imp)%10
	}
	switch k {
	case 0:
		a := abs16(c.IndustryPaid)
		tens24, ones14 = a/10, a%10
	case 1:
		tens24, ones14 = c.Pollution/10, c.Pollution%10
	}
	short := int16(c.Upkeep[k]) - (imp + c.Output[k])
	short = max(short, 0)
	var tens28, ones5 int16
	switch {
	case int8(imp) >= 0 && (c.Upkeep[1] == 0 || k != 1):
		tens28, ones5 = c.Output[k]/10, c.Output[k]%10
	case k == 1:
		v := max(int32(c.Output[1])-int32(c.Upkeep[1]), 0)
		tens28, ones5 = int16(v/10), int16(v%10)
	default:
		v := c.Output[k] - abs16(imp)
		tens28, ones5 = v/10, v%10
	}
	s3, s4 := short/10, short%10
	n := ones34 + ones5 + ones14 + s4
	if imp != 0 && k != 1 {
		n++
	}
	if ones14 != 0 || tens24 != 0 {
		n++
	}
	g.calculateSquishStep(x, xEnd, n+s3+tens2c+tens28+tens24, w)
	if w <= cg.stepSquish+1 {
		cg.stepSquish = w - 2
		e.V.SetWindow(int(x), int(y)-0x14, int(xEnd), int(y)+0x14)
		e.V.ClipOn()
	}
	pos := int16(0)
	draw := func(p *gfx.Picture, err error) error {
		if err != nil {
			return err
		}
		xx := x + (w-cg.stepSquish)*pos
		pos++
		return e.Draw(int(xx), int(y), p)
	}
	run := func(cnt int16, fn func(int, int, bool) (*gfx.Picture, error), kind, big int) error {
		for i := int16(0); i < cnt; i++ {
			if err := draw(fn(kind, big, wee)); err != nil {
				return err
			}
		}
		return nil
	}
	steps := []struct {
		cnt       int16
		fn        func(int, int, bool) (*gfx.Picture, error)
		kind, big int
	}{{tens28, g.prodAnims, k, 1}, {ones5, g.prodAnims, k, 0}}
	for _, st := range steps {
		if err := run(st.cnt, st.fn, st.kind, st.big); err != nil {
			return err
		}
	}
	if tens24 != 0 || ones14 != 0 {
		switch k {
		case 1:
			if err := run(tens24, g.importAnims, 1, 1); err != nil {
				return err
			}
			if err := run(ones14, g.importAnims, 1, 0); err != nil {
				return err
			}
		case 0:
			if err := run(tens24, g.prodAnims, 1, 1); err != nil {
				return err
			}
			if err := run(ones14, g.prodAnims, 1, 0); err != nil {
				return err
			}
		}
		pos++
	}
	pos++
	if imp < 1 || k == 1 {
		if err := run(tens2c, g.prodAnims, k, 1); err != nil {
			return err
		}
		if err := run(ones34, g.prodAnims, k, 0); err != nil {
			return err
		}
	} else {
		for _, st := range []struct {
			cnt int16
			fn  func(int, int, bool) (*gfx.Picture, error)
			big int
		}{{tens2c, g.importAnims, 1}, {ones34, g.importAnims, 0}, {s3, g.shortAnims, 1}, {s4, g.shortAnims, 0}} {
			if err := run(st.cnt, st.fn, k, st.big); err != nil {
				return err
			}
		}
	}
	e.V.ClipOff()
	return nil
}

// drawInfoMoraleBoth is Draw_Info_Morale_Both_ (0xb53cc): a morale icon per
// two points, up to twenty; the full size first draws the government icon.
func (g *Game) drawInfoMoraleBoth(ci int, x, y, xEnd int16, wee bool) error {
	e, cg, s := g.e, &g.col, &g.state
	c := &s.Colonies[ci]
	half := int16(c.Morale) / 2
	n := abs16(half)
	gov := s.Players[c.Owner].Picks[save.PickGovernment]
	if gov > 5 {
		n = 0
	}
	if !wee {
		p, err := g.colony2Pic(int(int16(int8(gov))) + 0x13) // Govt_Anims_ (0xbbb8a)
		if err != nil {
			return err
		}
		if err := e.Draw(int(x), int(y-1), p); err != nil {
			return err
		}
		x += 0x1e
	}
	g.calculateSquishStep(x, xEnd, n, 0x1e)
	if gov >= 6 {
		return nil
	}
	for i := int16(0); i < min(abs16(half), 20); i++ {
		a, dy := 8, int16(1)
		if half < 1 {
			a = 9
		}
		if !wee {
			a, dy = 5, 2
			if half < 1 {
				a = 6
			}
		}
		p, err := g.cAnims(a)
		if err != nil {
			return err
		}
		if err := e.Draw(int((0x1e-cg.stepSquish)*i+x), int(y+dy), p); err != nil {
			return err
		}
	}
	return nil
}

// cAnims is C_Anims_ (0xcba8e) for the cases this screen uses.
func (g *Game) cAnims(n int) (*gfx.Picture, error) {
	entry := map[int]int{5: 0x10, 6: 0x11, 7: 0x12, 8: 0x2b, 9: 0x2c, 10: 0x2d, 0xd: 0x32, 0xe: 0x33, 0xf: 0x34, 1: 0x31}
	if m, ok := entry[n]; ok {
		return g.colony2Pic(m)
	}
	return nil, fmt.Errorf("C_Anims_(%d) is not ported", n)
}

// drawBarIndicator is Draw_Bar_Indicator_ (0xc2539): the scroll thumb.
func (g *Game) drawBarIndicator() {
	v, st := g.e.V, &g.csStat
	n := int16(g.nColonies(int(g.state.PlayerNum)))
	g.updateFirst(10)
	if n > 9 {
		y1 := int16(int(st.first)*0x10f/int(n)) + 0x28
		y2 := int16((int(st.first)+10)*0x10f/int(n)) + 0x28
		v.SetWindow(0, 0, 0x27f, 0x1df)
		v.ClipOn()
		v.Fill(0x26d, int(y1), 0x272, int(y2), 0xe5)
		v.Line(0x26d, int(y1), 0x272, int(y1), 0xe6)
		v.Line(0x26d, int(y1), 0x26d, int(y2), 0xe6)
		v.Line(0x272, int(y1), 0x272, int(y2), 0xe4)
		v.Line(0x26d, int(y2), 0x272, int(y2), 0xe4)
		v.Dot(0x26d, 0x28, 0x50)
		v.Dot(0x272, 0x28, 0x50)
		v.Dot(0x26d, 0x137, 0x50)
		v.Dot(0x272, 0x137, 0x50)
	}
	v.ClipOff()
}

// colsumConnectGalaxyMapStars is Colsum_Connect_Galaxy_Map_Stars_ (0xc3a07):
// a moving line between the two colonies' stars on the small map.
func (g *Game) colsumConnectGalaxyMapStars(a, b int) {
	var colors [8]byte
	copy(colors[:6], g.exe.bytes(colsumLinkColors, 6))
	x1, y1 := g.galaxyMapStarXY(a, 0x17c, 0x15d, 0x80, 0x5b, 0, 0, false)
	x2, y2 := g.galaxyMapStarXY(b, 0x17c, 0x15d, 0x80, 0x5b, 0, 0, false)
	g.e.V.DirectionalMultiColoredLine(int(x1+2), int(y1+2), int(x2+2), int(y2+2), colors, int(g.col.count%6))
}

// drawCluster is Draw_Cluster_ (0xc9b49): the colonists being moved, by
// the pointer, while _cluster_colony_n has unemployed ones.
func (g *Game) drawCluster(px, py int) error {
	e, cg, s := g.e, &g.col, &g.state
	if cg.clusterN == -1 || cg.fullDraw || g.getFirstUnemployed(int(cg.clusterN)) == -1 {
		return nil
	}
	if len(g.ub.savedMouseList) != 0 || cg.dontDrawCluster { // Was_Mouse_List_Saved_
		return nil
	}
	e.EnableDrawAllFields()
	if err := e.DrawVisibleFields(); err != nil {
		return err
	}
	e.DisableDrawAllFields()
	e.V.SetWindow(0, 0, 0x27f, 0x1df)
	e.V.ClipOn()
	frame, err := g.colony2Pic(0x34) // C_Anims_(0xf)
	if err != nil {
		return err
	}
	if err := e.Draw(px, py, frame); err != nil {
		return err
	}
	x := px + 5
	c := &s.Colonies[cg.clusterN]
	for k := 0; k < int(c.PopCount); k++ {
		if c.Colonists[k]&0x200 != 0 {
			continue
		}
		pic, err := g.colonyPopAnim(int(cg.clusterN), k)
		if err != nil {
			return err
		}
		if err := e.Draw(x, py-10, pic); err != nil {
			return err
		}
		x += 0x14
	}
	e.V.ClipOff()
	return nil
}

// getFirstUnemployed is Get_First_Unemployed_ (0xc9ad7): the first colonist
// picked up (bit 0x200 clear), -1 for none.
func (g *Game) getFirstUnemployed(ci int) int {
	c := &g.state.Colonies[ci]
	for k := 0; k < int(c.PopCount); k++ {
		if c.Colonists[k]&0x200 == 0 {
			return k
		}
	}
	return -1
}

// getNumUnemployed is Get_Num_Unemployed_ (0xc9b0f).
func (g *Game) getNumUnemployed(ci int) int {
	n := 0
	c := &g.state.Colonies[ci]
	for k := 0; k < int(c.PopCount); k++ {
		if c.Colonists[k]&0x200 == 0 {
			n++
		}
	}
	return n
}

// popsIdentical is Pops_Identical_ (0xc9ce3): same job, state, race and flag.
func (g *Game) popsIdentical(ca, ka, cb, kb int) bool {
	a, b := uint32(g.colonist(ca, ka)), uint32(g.colonist(cb, kb))
	return a>>7&3 == b>>7&3 && g.popToPopState(ca, ka) == g.popToPopState(cb, kb) && a&0xf == b&0xf && a>>10&1 == b>>10&1
}

// getCluster is Get_Cluster_ (0xc9c3d): pick up colonist k of colony ci and
// those after it that look the same.
func (g *Game) getCluster(ci, k int) error {
	s := &g.state
	if g.colonist(ci, k)&0xf == 9 {
		if err := g.messageBox(g.eStr(0x17e)); err != nil {
			return err
		}
		return g.resetFieldMode()
	}
	g.col.clusterN = int16(ci)
	c := &s.Colonies[ci]
	for j := k; j < int(c.PopCount); j++ {
		if g.popsIdentical(ci, k, ci, j) {
			c.Colonists[j] &^= 0x200
		}
	}
	g.colonyCalculation(ci)
	g.clearMousePicture()
	return nil
}

// clearCluster is Clear_Cluster_ (0xc9a8c): put the colonists back.
func (g *Game) clearCluster() {
	cg := &g.col
	for {
		k := g.getFirstUnemployed(int(cg.clusterN))
		if k == -1 {
			break
		}
		g.state.Colonies[cg.clusterN].Colonists[k] |= 0x200
	}
	cg.clusterN = -1
	g.resetMousePicture()
}

// resetFieldMode is Reset_Field_Mode_ (0xbb2cb): put picked-up colonists
// of the colony screen's colony back and recalculate it. Field mode 2
// (moving troops) is never set in this build.
func (g *Game) resetFieldMode() error {
	if g.getFirstUnemployed(g.colonyHandle) != -1 {
		if g.cscr.fieldMode == 1 {
			g.clearCluster()
		}
		g.colCalcWrapper(g.colonyHandle)
	}
	g.cscr.fieldMode = 0
	return nil
}

// colonyPopString is Colony_Pop_String_ (0xc9da2): "Conquered Android
// expert workers"; language 4 orders the words differently.
func (g *Game) colonyPopString(ci, k int) string {
	s := &g.state
	p := uint32(g.colonist(ci, k))
	conquered := g.eStr(0xc) // Conquered_String_ (0xcb121)
	if p>>10&1 != 0 {
		conquered = g.eStr(0x256)
	}
	var race string // Pop_Race_String_ (0xcae1a)
	switch r := p & 0xf; {
	case r == 8:
		race = g.eStr(0xb3)
	case r == 9:
		race = g.eStr(0x255)
	default:
		race = save.CString(s.Players[r].RaceName[:])
	}
	state := g.eStr(0xc) // Pop_State_String_ (0xcb147)
	if g.popToPopState(ci, k) == 0 {
		state = g.eStr(0x11e)
	}
	job := g.eStr(0x22d) // Job_String_ (0xcb0f0)
	switch p >> 7 & 3 {
	case 0:
		job = g.eStr(0x123)
	case 2, 3:
		job = g.eStr(0x1c7)
	}
	if g.settings[0xd2] == 4 {
		return cSprintf(g.eStr(0x4c), job, race, conquered, state)
	}
	return cSprintf(g.eStr(0x4c), conquered, race, state, job)
}

// settleInfo is Pop_Tries_To_Settle_'s 15-byte result.
type settleInfo [15]byte

// popTriesToSettle is Pop_Tries_To_Settle_ (0x10ee4c): whether colonist k of
// colony src may move to colony dst, and why not.
func (g *Game) popTriesToSettle(src, k, dst int) settleInfo {
	s := &g.state
	var r settleInfo
	S, D := &s.Colonies[src], &s.Colonies[dst]
	P := &s.Players[S.Owner]
	from, to := g.colonyStar(src), g.colonyStar(dst)
	switch {
	case S.Owner != D.Owner:
		r[1] = 1
	case s.Stars[from].Blockaded>>uint(S.Owner)&1 != 0:
		r[2] = 1
	case s.eventCheckSpaceAnomaly(src):
		r[3] = 1
	case s.eventCheckPlague(src):
		r[8] = 1
	case D.IsOutpost != 0:
		r[4] = 1
	case S.PopCount == 1:
		r[5] = 1
	case S.Colonists[k]&0x400 != 0:
		r[0xe] = 1
	case src == dst:
		r[0] = 1
	case from != to && int(P.FreightersTotal) < (int(P.SettlersCount)+1)*5:
		r[6] = 1
	case from != to && P.SettlersCount == 0x19:
		r[7] = 1
	default:
		if !g.roomForAnother(D, int(S.Colonists[k]&0xf), true) {
			r[9] = 1
		}
		if s.Stars[to].Blockaded>>uint(D.Owner)&1 != 0 {
			r[0xb] = 1
		}
		if s.eventCheckSpaceAnomaly(dst) {
			r[0xc] = 1
		}
		r[0], r[0xd] = 1, byte(g.settlerETA(int(S.Owner), from, to))
	}
	return r
}

// settlerETA is Settler_ETA_ (0x10ed3f): turns at the player's FTL speed
// (at least 2), at most 15.
func (g *Game) settlerETA(p, a, b int) int {
	d := int(g.parsecsBetweenStars(a, b))
	if d == 0 {
		return 0
	}
	v := max(int(g.state.Players[p].FTLSpeed), 2)
	return min((d-1+v)/v, 0xf)
}

// settlePop is Settle_Pop_ (0x10f116): colonist k leaves colony src for
// colony dst, at once in the same system, else as a settler in transit.
// ecx is what the caller left in ECX, whose high bits seed the settler word.
func (g *Game) settlePop(src, k, dst int, ecx uint32) {
	s := &g.state
	if src == dst {
		return
	}
	S, D := &s.Colonies[src], &s.Colonies[dst]
	P := &s.Players[S.Owner]
	from, to := g.colonyStar(src), g.colonyStar(dst)
	pop := S.Colonists[k]
	v := ecx&0xfcf0ffff | (pop&0xf)<<16 | (pop>>7&3)<<24
	if from == to {
		g.addSettlerToColony(v, D)
	} else {
		eta := uint32(g.settlerETA(int(S.Owner), from, to))
		hi := v>>16&0xff0f | (eta&0xf)<<4
		P.Settlers[P.SettlersCount] = hi<<16 | uint32(byte(src)) | uint32(byte(D.Planet))<<8
		P.SettlersCount++
	}
	S.PopCount--
	S.Colonists[k] = S.Colonists[S.PopCount]
}

// giveColonistNewJob is Give_Colonist_New_Job_ (0xca5da).
func (g *Game) giveColonistNewJob(ci, k, job int) (bool, error) {
	s := &g.state
	c := &s.Colonies[ci]
	st := g.popToPopState(ci, k)
	var msg int
	switch {
	case (st == 3 || st == 6) && (job == 2 || job == 1):
		msg = 0x20a
	case st == 4 && job != int(c.Colonists[k]>>7&3):
		msg = 0x20c
	default:
		n := g.sumColonists(ci, job, -1, -1, 0, -1)
		if n < 0x2a && (n < int(c.CanFarm) || job != 0) {
			c.Colonists[k] = c.Colonists[k]&^0x180 | uint32(job&3)<<7 | 0x200
			return true, nil
		}
		msg = 0x187
	}
	return false, g.messageBox(g.eStr(msg))
}

// sumColonists is Sum_Colonists_ (0xbc972): colonists of colony ci with the
// given job, state, race (by player's race) and conquered flag (the sixth
// argument; the fifth is never read); -1 matches any. Only employed
// colonists count.
func (g *Game) sumColonists(ci, job, state, race, unused, flag int) int {
	s := &g.state
	c := &s.Colonies[ci]
	n := 0
	for k := 0; k < int(c.PopCount); k++ {
		pl := g.getEffectivePopPlayer(ci, k)
		p := c.Colonists[k]
		r := int(p & 0xf)
		if !((int(s.Players[pl].Race) == race && r == pl) || race == -1 || (r > 0xd && r == race)) {
			continue
		}
		if (g.popToPopState(ci, k) == state || state == -1) && (int(p>>7&3) == job || job == -1) &&
			(int(p>>10&1) == flag || flag == -1) && p&0x200 != 0 {
			n++
		}
	}
	return n
}

// sendCluster is Send_Cluster_ (0xc9e94): the picked-up colonists go to
// colony dst (with job, or -1 to keep theirs), after confirmations, with a
// report.
func (g *Game) sendCluster(dst, job int) error {
	s, cg := &g.state, &g.col
	if dst < 0 {
		return g.messageBox(g.eStr(0x210))
	}
	src := int(cg.clusterN)
	if src < 0 {
		// Unemployed colonists from a save, not picked up: the original reads
		// the record before the colony array.
		g.logOnce("Send_Cluster_ without a cluster reads colony -1, not ported")
		return nil
	}
	sent := 0
	changed := false
	asked := 0
	var info settleInfo
	var buf, popStr string
	k := g.getFirstUnemployed(src)
	if k != -1 {
		popStr = g.colonyPopString(src, k)
	}
	add := func(n *int, f string, args ...any) {
		l := cSprintf(f, args...)
		if *n == 0 {
			buf = l
		} else {
			buf = cSprintf(g.eStr(0x47), buf, 0xd, l)
		}
		*n++
	}
	for k != -1 {
		if src == dst {
			if job == -1 || int(s.Colonies[src].Colonists[k]>>7&3) == job {
				s.Colonies[src].Colonists[k] |= 0x200
			} else if ok, err := g.giveColonistNewJob(src, k, job); err != nil || !ok {
				return err
			}
			k = g.getFirstUnemployed(src)
			continue
		}
		info = g.popTriesToSettle(src, k, dst)
		eta, noRoom := info[0xd], info[9] != 0
		if info[0] != 0 && (eta > 0 || noRoom) {
			var msg string
			ask := 0
			switch {
			case eta > 0 && !noRoom && asked != 1:
				msg, ask = cSprintf(g.eStr(0x209), int(eta)), 1
			case noRoom && eta == 0 && asked != 2:
				n := g.getNumUnemployed(src)
				if sent == 0 {
					msg = cSprintf(g.eStr(0x205))
				} else {
					msg = cSprintf(g.eStr(0x203), sent, n)
				}
				ask = 2
			case noRoom && eta > 0 && asked != 3:
				n := g.getNumUnemployed(src)
				if sent == 0 {
					msg = cSprintf(g.eStr(0x206), int(eta))
				} else {
					msg = cSprintf(g.eStr(0x204), sent, n, int(eta))
				}
				ask = 3
			}
			if ask != 0 {
				g.resetMousePicture()
				asked = ask
				ok, err := g.confirmationBox(msg)
				if err != nil {
					return err
				}
				cg.fullDraw = true
				info[0] = byte(b2i(ok))
				g.clearMousePicture()
			}
		}
		if info[0] == 0 {
			n := 0
			switch {
			case eta > 0 && !noRoom:
				add(&n, g.eStr(0xfe), int(eta))
			case noRoom && eta == 0:
				add(&n, g.eStr(0xfd))
			case noRoom && eta > 0:
				add(&n, g.eStr(0xfc), int(eta))
			}
			for _, f := range []struct {
				i int
				e int
			}{{4, 0x1fd}, {5, 0x241}, {7, 0x24d}, {2, 0x207}, {6, 0x184}, {1, 0x211}, {0xa, 0x20f}, {0xe, 0x20e}} {
				if info[f.i] != 0 {
					add(&n, g.eStr(f.e))
				}
			}
			g.resetMousePicture()
			cg.fullDraw = true
			if n > 0 {
				if sent > 0 {
					m := n
					add(&m, g.eStr(0x80), sent)
				}
				title := g.eStr(0x1d6)
				if err := g.doTextBox(&title, buf, 0); err != nil {
					return err
				}
			} else if err := g.messageBox(g.eStr(0xe4)); err != nil {
				return err
			}
			g.clearMousePicture()
			break
		}
		if info[0xa] != 0 {
			changed = true
		}
		if int(s.Colonies[src].Colonists[k]>>7&3) != job && job != -1 {
			if ok, err := g.giveColonistNewJob(src, k, job); err != nil || !ok {
				return err
			}
			changed = true
			s.Colonies[src].Colonists[k] &^= 0x200
		}
		sent++
		g.settlePop(src, k, dst, uint32(src*0x169))
		k = g.getFirstUnemployed(src)
	}
	g.colonyCalculation(src)
	if src != dst {
		g.colonyCalculation(dst)
	}
	if sent > 0 {
		n := 0
		add(&n, g.eStr(0x1d1), sent, popStr, g.planetName(int(s.Colonies[src].Planet)), g.planetName(int(s.Colonies[dst].Planet)), int(info[0xd]))
		if info[0xb] != 0 {
			n = 1
			add(&n, g.eStr(0x84))
		}
		if info[0xa] != 0 {
			add(&n, g.eStr(0x85))
		}
		if changed {
			m := n
			add(&m, g.eStr(0x83))
		}
		g.resetMousePicture()
		title := g.eStr(0x1d5)
		if err := g.doTextBox(&title, buf, 0); err != nil {
			return err
		}
		g.clearMousePicture()
		g.colonyCalculation(src)
		g.colonyCalculation(dst)
	}
	if g.getFirstUnemployed(src) == -1 {
		g.resetMousePicture()
		if err := g.resetFieldMode(); err != nil {
			return err
		}
		cg.clusterN = -1
	}
	return nil
}

// drawPlanetSummary is Draw_Planet_Summary_ (0xc0e1d), the right-click
// description of a row's planet. The planet's picture only sets up the
// remap colours; it is never drawn.
func (g *Game) drawPlanetSummary(x, y, pl int) error {
	e, s := g.e, &g.state
	e.Text().SetStyleShadow(1, g.colors(colorsFont), font.ShadowDown)
	e.V.Fill(x+5, y+2, x+0xb0, y+0xc3, 0xe9)
	P := &s.Planets[pl]
	climate := int(P.Climate)
	if P.Colony != -1 {
		climate = int(s.Colonies[P.Colony].Climate)
	}
	mini, err := g.cached(g.eStr(0xde), int(P.Size)+climate*5) // Mini_Planet_Anim_ (0xc0dbc)
	if err != nil {
		return err
	}
	if err := e.CalculatePictureRemapColors(mini); err != nil {
		g.logOnce(err.Error())
	}
	frame, err := g.cAnims(0xe)
	if err != nil {
		return err
	}
	if err := e.Draw(x, y, frame); err != nil {
		return err
	}
	return e.Text().SqueezePrintFormattedParagraph(x+0xe, y+0x2d, 0x9a, 0x8c, g.planetDescription(pl), 0, g.exe.bytes(spaceLimits, 6), false)
}

// planetDescription is Build_Generic_Planet_Or_Colony_Description_String_
// Formatted_ (0xca16d).
func (g *Game) planetDescription(pl int) string {
	s := &g.state
	P := &s.Planets[pl]
	a, b := cStringAt(g.exe, 0x179e6e), cStringAt(g.exe, 0x179e6b)
	e := g.eStr
	var d string
	if P.Colony == -1 {
		if P.Type != 3 {
			return cSprintf(e(0x56), g.planetName(pl)) + cSprintf(e(0x133))
		}
		info := g.computePotentialColonyInfo(int(s.PlayerNum), pl)
		d = cSprintf(e(0x52), a, g.planetName(pl))
		d += cSprintf(e(0x4d), a, g.psEString(planetSizeNames, int(P.Size)), g.psEString(planetClimateNames, int(P.Climate)))
		d += cSprintf(e(0x6f), a, g.psEString(planetMineralNames, int(P.Minerals)))
		d += cSprintf(e(0x53), a, g.psEString(planetGravityNames, int(P.Gravity)))
		if f := info[4]; f%2 == 0 {
			d += cSprintf(e(0x65), a, b, int(f/2))
		} else {
			d += cSprintf(e(100), a, b, int(f/2), int(f%2))
		}
		d += cSprintf(e(0x6b), a, b, int(info[5]))
		d += cSprintf(e(0x74), a, b, int(info[6]))
		d += cSprintf(e(0x6c), a, b, int(info[1]))
		d += cSprintf(e(0x79), a, b, int(info[3]))
		d += cSprintf(e(0x6d), a, b, int(g.planetMaxPopulationForPlayer(pl, int(s.PlayerNum))))
	} else {
		c := &s.Colonies[P.Colony]
		d = cSprintf(e(0x49), a, g.planetName(pl), save.CString(s.Players[c.Owner].RaceName[:]))
		d += cSprintf(e(0x4d), a, g.psEString(planetSizeNames, int(P.Size)), g.psEString(planetClimateNames, int(c.Climate)))
		d += cSprintf(e(0x6f), a, g.psEString(planetMineralNames, int(P.Minerals)))
		d += cSprintf(e(0x53), a, g.psEString(planetGravityNames, int(P.Gravity)))
		d += cSprintf(e(0x5e), a, b, int(c.FoodPerFarmer/2))
		d += cSprintf(e(0x5f), a, b, int(c.IndustryPer))
		d += cSprintf(e(0x60), a, b, int(c.ResearchPer))
		if int(c.Owner) == int(s.PlayerNum) {
			d += cSprintf(e(0x70), a, b, int(c.Morale)*5)
		}
		d += cSprintf(e(0x71), a, b, int(c.PopCount), int(g.planetMaxPopulationForPlayer(pl, int(c.Owner))))
	}
	if sp := int8(P.Special); sp != 0 {
		d += cSprintf(e(0x50), a, g.psEString(planetSpecialNames, int(sp)))
	}
	return d
}

// testedColonyBuysOutright is Tested_Colony_Buys_Outright_ (0xcfc0b): buy
// the colony's product after a confirmation, or say why not. Its last steps
// set the colony screen's auto function and help, which the caller replaces.
func (g *Game) testedColonyBuysOutright(ci int) error {
	s := &g.state
	c := &s.Colonies[ci]
	var msg string
	switch {
	case c.Pending != 0:
		// Community patch 1.51: BUY again takes the purchase back (it said
		// E 0x24a, already bought).
		g.colonyUnbuy(ci)
		g.colonyCalculation(ci)
		return nil
	case s.Stars[g.colonyStar(ci)].Blockaded>>s.PlayerNum&1 != 0:
		msg = g.eStr(0x239)
	case c.Queue[0] == -2 || c.Queue[0] == -3:
		msg = g.eStr(0x238)
	case g.colonyCostToBuyProduct(c) < 1:
		msg = g.eStr(0x242)
		if c.Queue[0] == -1 {
			msg = g.eStr(0x24b)
		}
	case !g.colonyCanAffordToBuyProduct(ci):
		msg = cSprintf(g.eStr(0x234), int(int16(g.colonyCostToBuyProduct(c))-int16(s.Players[c.Owner].BC)))
	case g.colonyCanBuyProduct0(ci):
		left := int(int16(g.colonyProductCost(c, c.Queue[0]))) - int(c.Production)
		ok, err := g.confirmationBox(cSprintf(g.eStr(0x10d), int(g.colonyCostToBuyProduct(c)), left))
		if err != nil {
			return err
		}
		if ok {
			g.colonyBuyOutright(ci)
		}
		// Col_Calc_Wrapper_ (0xcb1a6): _field_mode is 0 here.
		g.colonyCalculation(ci)
		return nil
	default:
		msg = g.eStr(0x1b0)
	}
	if err := g.messageBox(msg); err != nil {
		return err
	}
	g.fullDrawScreen = false
	g.e.DeactivateAutoFunction()
	g.col.fullDraw = true
	return nil
}

// colsumEvaluateInput is Evaluate_Input_ (0xc4343); false leaves the
// screen at once.
func (g *Game) colsumEvaluateInput(r *int, scan int) (bool, error) {
	cs, st, cg := &g.cs, &g.csStat, &g.col
	n := int16(g.nColonies(int(g.state.PlayerNum)))
	switch {
	case *r == cs.up:
		g.incrementFirst(10)
		return true, nil
	case *r == cs.down && cs.list[int(st.first)+10] != -1:
		if n < 10 {
			return true, nil
		}
		st.first++
		cg.fullDraw = true
		st.slider = n - 10 - st.first
		return true, nil
	case *r == cs.ret || *r == cs.esc:
		if cg.clusterN != -1 {
			g.clearCluster()
			g.resetMousePicture()
		}
		if g.currentScreen == g.returnScreen && g.bq.fromDesign {
			g.currentScreen = ScreenBuildQueue
		} else {
			g.currentScreen = st.savedScreen
			g.returnScreen = st.savedScreen
		}
		if *r == cs.esc {
			*r = cs.ret
		}
		return true, nil
	case *r == cs.redraw:
		cg.fullDraw = true
		return true, nil
	case g.isBuildListField(*r, &cs.fList, &cs.fListShift):
		return true, g.colsumBuildList(*r)
	}
	for _, b := range cs.sortBtn {
		if *r == b {
			g.sortColList()
			for i := range cs.listCol {
				cs.listCol[i] = -1
			}
			st.first = 0
			g.incrementFirst(10)
			g.updateFirst(10)
			if err := g.updateColList(); err != nil {
				return false, err
			}
			cg.fullDraw = true
			return true, nil
		}
	}
	if g.fullDrawScreen {
		g.e.DeactivateAutoFunction()
		if err := g.restoreControl(); err != nil {
			return false, err
		}
		g.fullDrawScreen = false
		return true, nil
	}
	ok, err := g.evaluateColListInput(r, scan)
	if err != nil {
		return false, err
	}
	g.checkGalaxyMapFields(*r, scan)
	return ok, nil
}

// restoreControl is Restore_Control_ (0xc2502): the whole screen again.
func (g *Game) restoreControl() error {
	e := g.e
	e.DeactivateAutoFunction()
	e.AssignAutoFunction(g.drawColonySummaryScreen, 1)
	g.col.fullDraw = true
	e.V.SetPageOff()
	if err := g.drawColonySummaryScreen(); err != nil {
		return err
	}
	e.V.CopyOffToBack()
	e.V.SetRefreshStencil()
	g.setColonySummaryScreenHelpList()
	return nil
}

// evaluateColListInput is Evaluate_Col_List_Input_ (0xc3f30): the rows'
// fields.
func (g *Game) evaluateColListInput(r *int, scan int) (bool, error) {
	cs, cg, s := &g.cs, &g.col, &g.state
	if cs.drawPlanetSum {
		if *r == 0 {
			return true, nil
		}
		cs.drawPlanetSum = false
		if *r >= 0 {
			*r = 0
		}
	}
	for i := 0; i < 10; i++ {
		ci := int(cs.listCol[i])
		at := func() {
			pl := &s.Planets[s.Colonies[ci].Planet]
			g.screenData[0], g.screenData[1] = int16(pl.Star), int16(pl.Orbit)
		}
		switch {
		case *r == cs.listF[i] && cg.clusterN != -1:
			if err := g.sendCluster(ci, -1); err != nil {
				return false, err
			}
			g.e.DeactivateAutoFunction()
			if err := g.restoreControl(); err != nil {
				return false, err
			}
		case *r == cs.listF[i]:
			at()
			g.returnScreen = g.currentScreen
			g.currentScreen = ScreenColony
			*r = cs.ret
		case *r == cs.prodF[i]:
			g.e.DeactivateAutoFunction()
			g.clearGalaxyMapFields()
			g.e.ClearFields()
			at()
			s.Planets[s.Stars[g.screenData[0]].Planets[g.screenData[1]]].Colony = int16(ci)
			if cg.clusterN != -1 {
				g.clearCluster()
				g.resetMousePicture()
			}
			g.returnScreen = g.currentScreen
			g.currentScreen = 0x19
			return false, nil
		case -*r == cs.listF[i]:
			at()
			cs.drawPlanetSum = true
		case *r == cs.buyF[i]:
			if err := g.testedColonyBuysOutright(ci); err != nil {
				return false, err
			}
			g.e.DeactivateAutoFunction()
			if err := g.restoreControl(); err != nil {
				return false, err
			}
		}
		if err := g.evaluateColonyPopInput(*r, scan, i); err != nil {
			return false, err
		}
	}
	return true, nil
}

// evaluateColonyPopInput is Evaluate_Colony_Pop_Input_ (0xc4169).
func (g *Game) evaluateColonyPopInput(r, scan, i int) error {
	cs, cg := &g.cs, &g.col
	if i == 0 {
		cg.scannedPop = -1
	}
	for job := 0; job < 3; job++ {
		if r == cs.jobF[i][job] {
			if cg.clusterN != -1 {
				if err := g.sendCluster(int(cs.listCol[i]), job); err != nil {
					return err
				}
				g.e.DeactivateAutoFunction()
				if err := g.restoreControl(); err != nil {
					return err
				}
			} else {
				p, err := g.getSelectedPop(i, job)
				if err != nil {
					return err
				}
				if p != -1 {
					if err := g.getCluster(int(cs.listCol[i]), int(p)); err != nil {
						return err
					}
				}
				cg.fullDraw = true
			}
		}
		if scan == cs.jobF[i][job] {
			p, err := g.getScannedPop(i, job)
			if err != nil {
				return err
			}
			cg.scannedPop = p
		}
	}
	if scan == cs.listF[i] || scan == cs.prodF[i] || scan == cs.buyF[i] {
		cg.colonyN = cs.listCol[i]
		g.ms.galaxyMapScannedStar = int16(g.colonyStar(int(cg.colonyN)))
	}
	return nil
}

// checkGalaxyMapFields is Check_Galaxy_Map_Fields_ (0xc47c1). The star
// fields are cleared with the screen's fields and never added, so it
// matches nothing; it is kept for fidelity.
func (g *Game) checkGalaxyMapFields(r, scan int) {
	s, cs, st, gm := &g.state, &g.cs, &g.csStat, &g.ms.galaxyMap
	for i := 0; i < int(s.NumStars); i++ {
		pl := g.pickBestPlanetToGotoAtStar(i)
		if r == gm.starFieldOf(i) && pl != -1 {
			for k := 0; k < int(s.NumColonies); k++ {
				c := cs.list[k]
				if c != -1 && pl == s.Colonies[c].Planet {
					st.first = int16(k)
					for cs.list[int(st.first)+9] == -1 {
						g.incrementFirst(10)
					}
					st.slider = int16(g.nColonies(int(s.PlayerNum))) - 10 - st.first
					g.updateFirst(10)
					if err := g.updateColList(); err != nil {
						g.logOnce(err.Error())
					}
					g.col.fullDraw = true
				}
			}
		}
		if scan == gm.starFieldOf(i) {
			pl := g.pickBestPlanetToGotoAtStar(i)
			g.ms.galaxyMapScannedStar = int16(i)
			if pl != -1 {
				g.col.colonyN = s.Planets[pl].Colony
			}
		}
	}
}

// pickBestPlanetToGotoAtStar is Pick_Best_Planet_To_Goto_At_Star_
// (0xc48ef): the planet the player last chose there, else its last colony.
func (g *Game) pickBestPlanetToGotoAtStar(st int) int16 {
	s := &g.state
	p := int(s.PlayerNum)
	best := int16(s.Stars[st].LastOrbit[p])
	if lo := s.Stars[st].LastOrbit[p]; lo != -1 {
		if c := s.Planets[lo].Colony; c != -1 && s.Colonies[c].Owner == 0 {
			return best
		}
	}
	for k := 0; k < 5; k++ {
		q := s.Stars[st].Planets[k]
		if q != -1 {
			if c := s.Planets[q].Colony; c != -1 && int(s.Colonies[c].Owner) == p {
				best = q
			}
		}
	}
	return best
}
