package game

import (
	"github.com/openorion2/openorion2/internal/font"
)

// The paged lists of LIST.C: entries with an indented column of items,
// split into pages by height, one hidden field per item. The turn summary
// and the research choice's group list use them. Spec:
// docs/screens/turn-summary.md, "Lists".

// listHeader is the original's list header (static data in .data).
type listHeader struct {
	x, y, width int16 // +0, +2, +4
	maxY        uint16
	itemY       []int16 // +8: item k's y; item n the entry's height
	fieldY1     []int16 // +0xc, +0x10: the items' hidden field rows
	fieldY2     []int16
	spacing     uint8 // +0x14
	entries     []listEntry
	page        int   // +0x1a, index of the page's first entry
	pages, cur  uint8 // +0x1e, +0x1f
	style       int   // +0x20, the entry's text
	itemStyle   int   // +0x21
	print       func(x, y int, id uint8) error
	indent      int16 // +0x26
	arrowColor  uint8 // +0x27
	selColors   font.Colors
	colors      [5]font.Colors // +0x2c.., by state
}

// listEntry is one entry (0x9e bytes).
type listEntry struct {
	x, y      int16 // +0, +2
	pageStart bool  // +4
	selected  bool  // +5
	item      uint8 // +6, the selected item
	text      string
	id        uint8     // +0x47
	state     uint8     // +0x48, the colours
	n         uint8     // +0x49, items
	itemX     int16     // +0x4a
	right     int16     // +0x4c
	itemY     [16]int16 // +0x4e
	fields    [16]uint8 // +0x6e, the items' fields (bytes)
	ids       [16]uint8 // +0x7e
	states    [16]uint8 // +0x8e
}

// readListHeader reads the static part of a list header at addr.
func (g *Game) readListHeader(addr uint32, n int) listHeader {
	x := g.exe
	shorts := func(p uint32) []int16 {
		out := make([]int16, n)
		for i := range out {
			out[i] = g.exeI16(p + uint32(2*i))
		}
		return out
	}
	h := listHeader{
		x: g.exeI16(addr), y: g.exeI16(addr + 2), width: g.exeI16(addr + 4), maxY: uint16(g.exeI16(addr + 6)),
		itemY: shorts(x.u32(addr + 8)), fieldY1: shorts(x.u32(addr + 0xc)), fieldY2: shorts(x.u32(addr + 0x10)),
		spacing: x.bytes(addr+0x14, 1)[0], style: int(x.bytes(addr+0x20, 1)[0]), itemStyle: int(x.bytes(addr+0x21, 1)[0]),
		indent: int16(x.bytes(addr+0x26, 1)[0]), arrowColor: x.bytes(addr+0x27, 1)[0],
		selColors: g.listColors(x.u32(addr + 0x28)),
	}
	for i := range h.colors {
		h.colors[i] = g.listColors(x.u32(addr + 0x2c + uint32(4*i)))
	}
	return h
}

// listColors reads a header's colour pointer: into _tech_color (0x1ad5a4),
// which TECH.C fills at run time, or into .data.
func (g *Game) listColors(p uint32) font.Colors {
	if p >= techColorBase && p < techColorBase+uint32(8*len(g.ts.colors)) {
		return g.ts.colors[(p-techColorBase)/8]
	}
	return g.exeColors(p)
}

// initListData is Init_List_Data_ (0x10a0a7): place the entries and cut
// them into pages.
func (h *listHeader) initListData() {
	h.pages, h.cur = 1, 1
	h.page = 0
	y := uint16(h.y)
	for i := range h.entries {
		en := &h.entries[i]
		en.x = h.x
		en.itemX = h.x + h.indent
		en.selected = false
		en.y = int16(y)
		en.right = h.x + h.width - h.indent
		k := 0
		for ; k < int(en.n); k++ {
			en.itemY[k] = h.itemY[k]
		}
		y += uint16(h.itemY[k]) + uint16(h.spacing)
		if uint16(h.maxY) < y {
			en.pageStart = true
			h.pages++
			y = uint16(h.y) + uint16(h.itemY[k]) + uint16(h.spacing)
			en.y = h.y
		} else {
			en.pageStart = false
		}
	}
	if len(h.entries) > 0 {
		h.entries[0].pageStart = true
	}
}

// pageEnd is the index after the current page's last entry.
func (h *listHeader) pageEnd() int {
	i := h.page
	for i < len(h.entries) {
		i++
		if i < len(h.entries) && h.entries[i].pageStart {
			break
		}
	}
	return i
}

// addFieldsToListPage is Add_Fields_To_List_Page_ (0x10a188): a hidden
// field per item of the page; returns the last field's number.
func (g *Game) addFieldsToListPage(h *listHeader) (int, error) {
	for i := h.page; i < h.pageEnd(); i++ {
		en := &h.entries[i]
		for k := 0; k < int(en.n); k++ {
			f, err := g.e.AddHiddenField(int(en.x), int(en.y+h.fieldY1[k]), int(en.right), int(en.y+h.fieldY2[k]), "", 0)
			if err != nil {
				return 0, err
			}
			en.fields[k] = uint8(f)
		}
	}
	return g.e.FieldsCount() - 1, nil
}

// displayListText is Display_List_Text_ (0x10a3ad): the entry's text in
// its state's colours, then each item through the print routine. With
// _race_report_jim_hack, items the human does not know take colours 4.
func (g *Game) displayListText(h *listHeader, en *listEntry) error {
	if en == nil {
		return nil
	}
	t := g.e.Text()
	c := h.colors[en.state]
	t.SetStyle(h.style, c)
	if err := g.squeezePrint(int(en.x), int(en.y), en.text, int(h.width)); err != nil {
		return err
	}
	if h.print == nil {
		return nil
	}
	t.SetStyle(h.itemStyle, c)
	for k := 0; k < int(en.n); k++ {
		if ck := h.colors[en.states[k]]; ck != c {
			t.SetStyle(h.itemStyle, ck)
			c = ck
		}
		t.SetStyle(h.itemStyle, c)
		if g.raceReportJimHack && g.state.Players[g.state.PlayerNum].AppStatus[en.ids[k]] != techKnown {
			t.SetStyle(h.itemStyle, h.colors[4])
		}
		if err := h.print(int(en.itemX), int(en.y+en.itemY[k]), en.ids[k]); err != nil {
			return err
		}
	}
	return nil
}

// displayListPage is Display_List_Page_ (0x10a4a7).
func (g *Game) displayListPage(h *listHeader) error {
	h.setListPagePointer()
	for i := h.page; i < h.pageEnd(); i++ {
		if err := g.displayListText(h, &h.entries[i]); err != nil {
			return err
		}
	}
	return nil
}

// setListPagePointer is Set_List_Page_Pointer_ (0x10a4e2): the first
// entry of page cur (0 counts as 1).
func (h *listHeader) setListPagePointer() {
	want := h.cur
	h.cur = 1
	if want == 0 {
		want = 1
	}
	for i := range h.entries {
		if want < 2 {
			h.page = i
			return
		}
		if i+1 < len(h.entries) && h.entries[i+1].pageStart {
			want--
			h.cur++
		}
	}
	h.page = 0
}

// nextListPage and previousListPage are Next_List_Page_ (0x10a527) and
// Previous_List_Page_ (0x10a540).
func (h *listHeader) nextListPage() bool {
	if h.cur < h.pages {
		h.cur++
		h.setListPagePointer()
		return true
	}
	return false
}

func (h *listHeader) previousListPage() bool {
	if h.cur > 1 {
		h.cur--
		h.setListPagePointer()
		return true
	}
	return false
}

// selectAndPageByHeadID is Select_And_Page_By_Head_ID_ (0x10a5a3): the
// entry with id (else the first) is selected and its page shown.
func (h *listHeader) selectAndPageByHeadID(id uint8) int {
	i := 0
	for k := range h.entries {
		if h.entries[k].id == id {
			i = k
			break
		}
	}
	h.entries[i].selected = true
	h.gotoPageWithEntry(i)
	return i
}

// gotoPageWithEntry is Goto_Page_With_Entry_ (0x10a554).
func (h *listHeader) gotoPageWithEntry(e int) {
	page, start := uint8(0), 0
	for i := range h.entries {
		if h.entries[i].pageStart {
			page++
			start = i
		}
		if i == e {
			h.cur, h.page = page, start
			return
		}
	}
}

// setSelectedListEntry is Set_Selected_List_Entry_ (0x10a5e3): on the
// page, the entry with an item whose field is f becomes the selection.
func (h *listHeader) setSelectedListEntry(f int) *listEntry {
	var found *listEntry
	for i := h.page; i < h.pageEnd(); i++ {
		en := &h.entries[i]
		en.selected = false
		if found != nil {
			continue
		}
		for k := 0; k < int(en.n); k++ {
			if int16(int8(en.fields[k])) == int16(f) {
				en.selected, en.item, found = true, uint8(k), en
			}
		}
	}
	return found
}

// selectedListEntry is Get_Selected_List_Entry_ (0x10a659): the page's
// selected entry, or nil.
func (h *listHeader) selectedListEntry() *listEntry {
	for i := h.page; i < h.pageEnd(); i++ {
		if h.entries[i].selected {
			return &h.entries[i]
		}
	}
	return nil
}
