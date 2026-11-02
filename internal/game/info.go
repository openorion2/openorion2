package game

import (
	"fmt"
	"strconv"

	"github.com/openorion2/openorion2/internal/engine"
	"github.com/openorion2/openorion2/internal/font"
	"github.com/openorion2/openorion2/internal/gfx"
)

// The info screen (screen 9, the INFO button): MOX\INFO.C. Five pages
// (history graph, tech review, race statistics, turn summary, reference)
// beside the maintenance chart. Spec: docs/screens/info-screen.md.

// Addresses of INFO.C's data.
const (
	infoLBX            = 0x17a280 // "INFO.LBX"
	infoEsc            = 0x17a289 // "\x1b"
	infoDateFmt        = 0x17a28c // "%i.%i", "%i,%i" at +6
	infoIntFmt         = 0x17a2a5 // "%i"
	infoPercentFmt     = 0x17a2a8 // "%3i%%"
	infoHighColors     = 0x183b26 // _info_high_color
	infoRedColors      = 0x183b2e // _info_red_color
	infoDarkColors     = 0x183b36 // _info_dark_color
	infoBarColors      = 0x183b77 // _maintenance_bar_colors, 7 pointers
	infoTextColors     = 0x183bcb // _maintenance_text_colors, 7 pointers
	infoHistPalettes   = 0x183be7 // _hist_palettes, 8 bytes per banner colour
	infoSubScrField    = 0x183c27 // _sub_scr_field, x and y bytes
	infoHistoryDrops   = 0x183c41 // _history_graph_drops
	infoTechDrops      = 0x183c63 // _tech_review_drops
	infoRaceStatsDrops = 0x183ca7 // _race_stats_drops
	infoTurnSumDrop    = 0x183ceb // _turn_summary_drop
	infoReferenceDrops = 0x183cfc // _reference_drops
	infoHowToDrops     = 0x183d40 // _how_to_drops
	infoCategoryDrops  = 0x183d62 // _category_drops
	infoTicks          = 0x183d95 // tick lengths of the history graph's axes
	infoGraphButtonX   = 0x183d9f // _graph_button_x
	infoHelpList       = 0x1836a0 // _info_help_list, entry 2 copied per page
	historyHelp        = 0x18370e // History_Graph_Help_
	techReviewHelp     = 0x183704 // Tech_Review_Help_
	raceStatsHelp      = 0x1836fa // Race_Statistics_Help_
	turnSummaryHelp    = 0x1836e6 // Turn_Summary_Help_
	referenceHelp      = 0x1836c8 // Reference_Help_
	refCategoryHelp    = 0x1836d2 // Ref_Category_Help_
	refHowToHelp       = 0x1836dc // Ref_How_To_Help_
	dropAreaSize       = 0x11
)

// Info pages, bits 4..6 of Player.InfoButtons.
const (
	infoHistory = iota
	infoTechReview
	infoRaceStats
	infoTurnSummary
	infoReference
)

// infoState holds INFO.C's statics and Info_Screen_'s locals that its pages
// reach.
type infoState struct {
	pics      [0x1a]*gfx.Picture // INFO.LBX 0..0x19
	bars      [7]*gfx.Picture    // the maintenance chart's columns
	rows      [7]*gfx.Picture    // and its rows
	maint     [7]int32
	page      int16              // the page button's variable
	exitField int                // _g_exit_field
	refType   uint8              // _g_reference_screen_type
	refIx     int16              // _g_reference_ix
	msgIx     uint8              // _g_turn_message_ix (INFO.C's)
	racePage  bool               // _race_statistics_page
	techList  int16              // the tech review's chosen list (0x183e55)
	selApp    int16              // _g_selected_app_id
	histPal   [8]font.Colors     // _hist_graph_palettes, by player
	darkColor byte               // _info_dark_color's first byte
	specials  []engine.HelpEntry // _race_statistics_specials_help
	list      infoListState      // the list pages' globals
	turnHead  listHeader         // _g_turn_header
	catHead   listHeader         // _g_category_header
	showing   int                // port-only: the page in its input loop, -1 none
}

// infoListState holds the globals the list pages share.
type infoListState struct {
	firstField, lastField int        // _g_first_list_field, _g_last_list_field
	prior                 int        // _g_prior_choice
	updateNeeded          bool       // _g_list_update_needed
	sel                   *listEntry // _g_selected_list_p, _g_selected_msg_p
	header                *listHeader
}

// infoScreen is Info_Screen_ (0x106cac), screen 9.
func (g *Game) infoScreen() error {
	e, is, s := g.e, &g.info, &g.state
	me := int(s.PlayerNum)
	e.SavePalette()
	e.ClearFields()
	if err := e.FastFadeOut(); err != nil {
		return err
	}
	e.V.SetPageOff()
	e.V.SetRefreshStencil()
	is.refType = 0
	if g.returnScreen == 0 {
		is.msgIx = 0
	}
	g.activePlayer[0] = uint8(me)
	g.getActivePlayers(1, false)
	name := cStringAt(g.exe, infoLBX)
	for i := range is.pics {
		p, err := g.picture(name, i)
		if err != nil {
			return err
		}
		is.pics[i] = p
	}
	for i := range is.bars {
		is.bars[i] = gfx.NewBlankBitmap(0x18, 0x6c)
		is.rows[i] = gfx.NewBlankBitmap(0xa0, 0x14)
	}
	n := g.initializeMaintenanceValues()
	if err := g.buildMaintChartBitmaps(); err != nil {
		return err
	}
	e.V.SetPaletteChanges(0, 0xff)
	e.DrawPalette(is.pics[1])
	g.turnOffPalettes()
	g.billPictureToMouse(is.pics[1])
	is.page = int16(s.Players[me].InfoButtons&0x70) >> 4
	var err error
	if is.exitField, err = e.AddButtonField(0x217, 0x1b2, "", is.pics[2], "\x1b", 0x28); err != nil {
		return err
	}
	for i := 0; i < 5; i++ {
		x, y := g.exeU8(infoSubScrField+uint32(2*i)), g.exeU8(infoSubScrField+uint32(2*i)+1)
		if _, err := e.AddMultiButtonField(int(x), int(y), "", is.pics[3+i], &is.page, int16(i), "", 0x29); err != nil {
			return err
		}
	}
	counter := e.FieldsCount()
	date := g.stardateString()
	is.racePage = true
	is.showing = -1
	for {
		g.fullDrawScreen = false
		g.activePlayer[0] = uint8(me)
		g.getActivePlayers(1, false)
		e.V.Fill(0, 0, 0x27f, 0x1df, 0)
		g.e.Text().SetOutlineColor(0xb0, 0xb0)
		g.triInterlacedFill(0xd4, 0x17, 0x26c, 0x1c9, 0xb2, 0xb3, 0xb3)
		if err := e.Draw(0, 0, is.pics[0]); err != nil {
			return err
		}
		t := e.Text()
		t.SetStyle(4, g.exeColors(infoNormalColors))
		if err := t.PrintCentered(0x97, 0x1b, date); err != nil {
			return err
		}
		if err := g.drawMaintIncomeChart(n - 1); err != nil {
			return err
		}
		e.ClearFieldsAbove(counter - 1)
		e.ResetCyclePaletteColor()
		t.SetStyle(5, g.exeColors(infoNormalColors))
		var r int
		switch is.page {
		case infoHistory:
			g.infoHelp(historyHelp)
			r, err = g.historySubscreen(counter)
		case infoTechReview:
			g.infoHelp(techReviewHelp)
			r, err = g.techReviewSubscreen(counter)
			g.raceReportJimHack = false
		case infoRaceStats:
			g.infoHelp(raceStatsHelp)
			r, err = g.raceStatsSubscreen(counter)
		case infoTurnSummary:
			g.infoHelp(turnSummaryHelp)
			r, err = g.turnSummarySubscreen(counter)
		case infoReference:
			r, err = g.referenceSubscreen(counter)
		default:
			err = fmt.Errorf("info page %d", is.page)
		}
		if err != nil {
			return err
		}
		is.showing = -1
		e.DeactivateHelpList()
		if r == is.exitField {
			break
		}
	}
	pl := &s.Players[me]
	pl.InfoButtons = pl.InfoButtons&0x8f | byte(is.page)&7<<4
	e.ClearFields()
	e.DeactivateAutoFunction()
	g.currentScreen = ScreenMain
	return nil
}

// stardateString is the "%i.%i" (or "%i,%i") of the stardate.
func (g *Game) stardateString() string {
	f := infoDateFmt
	if g.settings[0xd2] != 0 {
		f += 6
	}
	sd := int32(g.state.Stardate)
	return fmt.Sprintf(cFormat(cStringAt(g.exe, uint32(f))), sd/10, sd%10)
}

// infoHelp is History_Graph_Help_ (0x103a1b) and its siblings from 0x103990:
// the page's entry becomes entry 2 of _info_help_list.
func (g *Game) infoHelp(entry uint32) {
	list := append(g.helpListAt(infoHelpList, 2), g.helpListAt(entry, 1)...)
	g.e.DeactivateHelpList()
	g.e.SetHelpList(list)
}

// initializeMaintenanceValues is Initialize_Maintenance_Values_ (0x1075ab):
// gross income and the six maintenance parts as permille of the largest,
// rounded to percent; returns how many are not zero.
func (g *Game) initializeMaintenanceValues() int {
	is := &g.info
	pl := &g.state.Players[g.state.PlayerNum]
	var v [7]int32
	v[0] = int32(pl.GrossIncome)
	for i, m := range pl.MaintenanceParts {
		v[i+1] = int32(m)
	}
	hi := int32(0)
	for _, x := range v {
		if hi < x {
			hi = x
		}
	}
	n := 0
	if hi < 1 {
		is.maint = [7]int32{}
		return 0
	}
	for i, x := range v {
		is.maint[i] = (x*1000/hi + 5) / 10
		if is.maint[i] != 0 {
			n++
		}
	}
	return n
}

// colorizeBitmap is Colorize_Bitmap_ (0x10a064): pixels lo..hi take the
// table's colours.
func colorizeBitmap(bm *gfx.Picture, table []byte, lo, hi byte) {
	px := bm.Pixels()
	for i, c := range px {
		if c >= lo && c <= hi {
			px[i] = table[c-lo]
		}
	}
}

// buildMaintChartBitmaps is Build_Maint_Chart_Bitmaps_ (0x107214): a column
// per value cut to its height, and a labelled row per value.
func (g *Game) buildMaintChartBitmaps() error {
	is := &g.info
	pl := &g.state.Players[g.state.PlayerNum]
	top := gfx.NewBlankBitmap(0x18, 9)
	if err := top.AddPicture(0, 0, is.pics[0x17]); err != nil {
		return err
	}
	foot := gfx.NewBlankBitmap(0x18, 6)
	if err := foot.AddPicture(0, 0, is.pics[0x19]); err != nil {
		return err
	}
	column := gfx.NewBlankBitmap(0x18, 0x6c)
	if err := column.AddPicture(0, 0, is.pics[0x16]); err != nil {
		return err
	}
	for i := 0; i < 7; i++ {
		colors := g.exe.bytes(g.exe.u32(infoBarColors+uint32(4*i)), 7)
		if is.maint[i] <= 0 && i != 0 {
			is.bars[i] = nil
			continue
		}
		bm := gfx.NewBlankBitmap(0x18, 0x6c)
		is.bars[i] = bm
		if err := bm.AddPicture(0, int(int16(0x64-int16(is.maint[i]))), column); err != nil {
			return err
		}
		bm.MaskOut(0, 0x63, top, true)
		if err := bm.AddPicture(0, 0x66, foot); err != nil {
			return err
		}
		colorizeBitmap(bm, colors, 1, 7)
	}
	t := g.e.Text()
	for i := 0; i < 7; i++ {
		bm := gfx.NewBlankBitmap(0xa0, 0x14)
		is.rows[i] = bm
		if err := bm.AddPicture(0, 0, is.pics[0x18]); err != nil {
			return err
		}
		colorizeBitmap(bm, g.exe.bytes(g.exe.u32(infoBarColors+uint32(4*i)), 7), 1, 7)
		t.SetStyle(2, g.exeColors(g.exe.u32(infoTextColors+uint32(4*i))))
		target := font.Target{Pix: bm.Pixels(), Stride: bm.Width, H: bm.Height}
		if i == 0 {
			label, err := g.textMessage(billTextLBX, 0x13, 0x28)
			if err != nil {
				return err
			}
			s := strconv.Itoa(int(pl.GrossIncome)) + label
			if t.Width([]byte(s)) > 0x7e {
				t.SetSpacing(1)
			}
			if err := t.CenteredToBitmap(target, 0x5c, 3, []byte(s)); err != nil {
				return err
			}
			t.SetSpacing(t.Font.Styles[2].HSpacing)
			continue
		}
		s := fmt.Sprintf(cFormat(cStringAt(g.exe, infoPercentFmt)), is.maint[i])
		if err := t.ToBitmap(target, 0x1b, 3, []byte(s)); err != nil {
			return err
		}
		var c font.Colors
		for k := range c {
			c[k] = byte(k/2 + 1)
		}
		c[0] = [6]byte{0x14, 0xa8, 0xae, 0xa2, 0x97, 0xb9}[i-1]
		t.SetStyle(2, c)
		label, err := g.textMessage(billTextLBX, 0x13+i, 0x28)
		if err != nil {
			return err
		}
		if err := t.CenteredToBitmap(target, 0x6c, 3, []byte(label)); err != nil {
			return err
		}
	}
	is.darkColor = 0xb0
	return nil
}

// drawMaintIncomeChart is Draw_Maint_Income_Chart_ (0x10709a); n is one
// less than the values shown.
func (g *Game) drawMaintIncomeChart(n int) error {
	e, is := g.e, &g.info
	if err := e.Draw(0x1e, 0xd4, is.bars[0]); err != nil {
		return err
	}
	if int16(n) > 0 {
		x := ((int(int16(n))-1)*0x12-0x18)/2 + 0x78
		for i := 6; i > 0; i-- {
			if is.bars[i] != nil {
				if err := e.Draw(int(int16(x)), 0xd4, is.bars[i]); err != nil {
					return err
				}
				x -= 0x12
			}
		}
	}
	for i := 0; i < 7; i++ {
		if err := e.Draw(0x19, 0x158+0x10*i, is.rows[i]); err != nil {
			return err
		}
	}
	t := e.Text()
	t.SetStyle(1, g.exeColors(infoNormalColors))
	for k, x := range [2]int{0x2a, 0x78} {
		s, err := g.textMessage(billTextLBX, 0x11+k, 0x28)
		if err != nil {
			return err
		}
		if err := t.PrintCentered(x, 0x144, s); err != nil {
			return err
		}
	}
	pl := &g.state.Players[g.state.PlayerNum]
	net := int32(pl.GrossIncome) - pl.Maintenance
	c := g.exeColors(infoNormalColors)
	if net < 0 {
		c = g.exeColors(infoRedColors)
	}
	t.SetStyle(2, c)
	s, err := g.textMessage(billTextLBX, 0x1b, 0x40)
	if err != nil {
		return err
	}
	if s, err = g.decodeTextMessage(s, -1, -1, -1, int32(int16(net)), -1, -1, -1, 0x40); err != nil {
		return err
	}
	return t.PrintCentered(0x69, 0xc4, s)
}

// introduceSubscreen is Introduce_Subscreen_ (0x109db5): fade in the first
// page, draw the blinds for the others.
func (g *Game) introduceSubscreen() error {
	e := g.e
	if e.FadePercent() != 0 {
		e.V.ClearPaletteChanges(0, 0xff)
		if err := e.TogglePages(); err != nil {
			return err
		}
		g.turnOffPalettes()
		return e.FastFadeIn()
	}
	return e.BlindsFade(0, 0, 0x27f, 0x1df, 3)
}

// drawDropArea is Draw_Drop_Area_ (0x109e01): a sunken panel in three
// interlaced fills, its corners cut.
func (g *Game) drawDropArea(a uint32) {
	x1, y1 := int(g.exeI16(a)), int(g.exeI16(a+2))
	x2, y2 := int(g.exeI16(a+4)), int(g.exeI16(a+6))
	c := g.exe.bytes(a+8, 9)
	fill := func(x1, y1, x2, y2 int, k int) {
		g.triInterlacedFill(int(int16(x1)), int(int16(y1)), int(int16(x2)), int(int16(y2)), c[k], c[k+1], c[k+2])
	}
	fill(x1+2, y1, x2-2, y1+2, 0)
	fill(x1+2, y1, x1+2, y1+2, 3)
	fill(x2-2, y1, x2-2, y1+2, 6)
	fill(x1, y1+3, x2, y2-3, 0)
	fill(x1, y1+3, x1, y2-3, 3)
	fill(x2, y1+3, x2, y2-3, 6)
	fill(x1+2, y2-2, x2-2, y2, 0)
	fill(x1+2, y2-2, x1+2, y2, 3)
	fill(x2-2, y2-2, x2-2, y2, 6)
}

// drawDropAreas draws the areas from a up to (not including) end.
func (g *Game) drawDropAreas(a, end uint32) {
	for ; a < end; a += dropAreaSize {
		g.drawDropArea(a)
	}
}

// setUpPlayerColors is Set_Up_Player_Colors_ (0x109fba): the graph colours
// (192..255 from INFO.LBX 1's palette) and each player's eight shades.
func (g *Game) setUpPlayerColors() {
	is := &g.info
	g.e.V.SetPaletteChanges(0xc0, 0xff)
	g.e.DrawPalette(is.pics[1])
	for p := 0; p < int(g.state.NumPlayers); p++ {
		is.histPal[p] = g.exeColors(infoHistPalettes + 8*uint32(g.state.Players[p].Color))
	}
}

// getActivePlayers is Get_Active_Players_ (0x10248b): from slot start on,
// the living players the human has met, then (withLost) those it has lost.
func (g *Game) getActivePlayers(start int, withLost bool) {
	s := &g.state
	me := int(s.PlayerNum)
	n := start
	for p := 0; p < int(s.NumPlayers); p++ {
		if p != me && s.Players[p].Eliminated == 0 && s.Players[me].Contact[p] != 0 {
			g.activePlayer[n] = uint8(p)
			n++
		}
	}
	if withLost {
		for p := 0; p < int(s.NumPlayers); p++ {
			if p != me && s.Players[p].Eliminated == 0 && s.Players[me].Contact[p] == 0 && s.Players[me].LostContactCount[p] != 0 {
				g.activePlayer[n] = uint8(p)
				n++
			}
		}
	}
	g.activeCount = int16(n)
}

// addActivePlayer appends p to _active_player unless it is there.
func (g *Game) addActivePlayer(p int) {
	for i := 0; i < int(g.activeCount); i++ {
		if int(g.activePlayer[i]) == p {
			return
		}
	}
	g.activePlayer[g.activeCount] = uint8(p)
	g.activeCount++
}

// getPlayersDeadOrAliveOrOmniscient is Get_Players_Dead_Or_Alive_Or_
// Omniscient_ (0x102597): the active players, then every eliminated one, or
// with an omniscient human every other.
func (g *Game) getPlayersDeadOrAliveOrOmniscient(start int, withLost bool) {
	s := &g.state
	g.getActivePlayers(start, withLost)
	for p := 0; p < int(s.NumPlayers); p++ {
		if p != int(s.PlayerNum) && (s.playerIsOmniscient(int(s.PlayerNum)) || s.Players[p].Eliminated != 0) {
			g.addActivePlayer(p)
		}
	}
}

// getPlayersAliveOrOmniscient is Get_Players_Alive_Or_Omniscient_
// (0x102633): the active players and, with an omniscient human, every
// other living one.
func (g *Game) getPlayersAliveOrOmniscient(start int, withLost bool) {
	s := &g.state
	g.getActivePlayers(start, withLost)
	for p := 0; p < int(s.NumPlayers); p++ {
		if p != int(s.PlayerNum) && s.playerIsOmniscient(int(s.PlayerNum)) && s.Players[p].Eliminated == 0 {
			g.addActivePlayer(p)
		}
	}
}
