// Package engine ports the parts of the WLIBS engine every screen runs on:
// video pages, the palette, timing, drawing primitives and, on top of them,
// fields and input.
//
// Spec: docs/engine/video-and-timing.md, docs/engine/fields-and-input.md.
package engine

// Screen geometry.
const (
	W    = 640
	H    = 480
	Size = W * H
)

// Refresh modes, screen_refresh_flag.
const (
	RefreshFull         = 0
	RefreshMasked       = 1 // dead in the original
	RefreshDifferential = 2
	RefreshStencil      = 3
)

// span is one row of refresh_scan_list or refresh_stencil_list: dword columns
// start..end inclusive; start -1 means the row is clean.
type span struct{ start, end int16 }

// Color is one entry of current_palette: a changed flag and 6-bit RGB.
type Color struct{ Changed, R, G, B byte }

// Video holds the page buffers and the state Toggle_Pages works from.
type Video struct {
	Off, Back, Screen []byte
	// Current is current_video_page, the buffer primitives draw into.
	Current []byte

	refresh int // screen_refresh_flag
	dump    int // screen_dump_flag: 0 full, 1 masked, 2 scan list

	scan, stencil [H]span

	Palette [256]Color   // current_palette
	DAC     [256][3]byte // what the display shows, 6-bit

	// Clipping window, used when Clipping is set (clipping_flag).
	Clipping                   bool
	WinX1, WinY1, WinX2, WinY2 int

	// Cursor rectangle bookkeeping for partial flips (last_on_page_mouse_x/y).
	lastMouseX, lastMouseY int
}

// NewVideo returns the state Init_Video_Drivers and Create_Screen_Buffers
// leave: zeroed pages, refresh full, drawing to off.
func NewVideo() *Video {
	v := &Video{Off: make([]byte, Size), Back: make([]byte, Size), Screen: make([]byte, Size)}
	v.Current = v.Off
	v.WinX2, v.WinY2 = W-1, H-1
	v.clearScan()
	v.clearStencil()
	return v
}

func (v *Video) clearScan() {
	for i := range v.scan {
		v.scan[i] = span{-1, -1}
	}
}

func (v *Video) clearStencil() {
	for i := range v.stencil {
		v.stencil[i] = span{-1, -1}
	}
}

// SetPageOff is Set_Page_Off.
func (v *Video) SetPageOff() { v.Current = v.Off }

// SetPageBack is Set_Page_Back.
func (v *Video) SetPageBack() { v.Current = v.Back }

// CopyOffToBack is Copy_Off_To_Back; it does not touch the scan lists.
func (v *Video) CopyOffToBack() { copy(v.Back, v.Off) }

// CopyBackToOff is Copy_Back_To_Off.
func (v *Video) CopyBackToOff() { copy(v.Off, v.Back) }

// SetRefreshFull is Set_Refresh_Full.
func (v *Video) SetRefreshFull() { v.refresh, v.dump = RefreshFull, 0 }

// SetRefreshDifferential is Set_Refresh_Differential.
func (v *Video) SetRefreshDifferential() { v.refresh, v.dump = RefreshDifferential, 0 }

// SetRefreshStencil is Set_Refresh_Stencil.
func (v *Video) SetRefreshStencil() { v.refresh, v.dump = RefreshStencil, 0 }

// RefreshMode is Get_Refresh_Mode.
func (v *Video) RefreshMode() int { return v.refresh }

// AddSquare is Add_Square_To_Scan_List: clamp to the screen (not the window)
// and merge the dword columns into the scan list, and into the stencil list
// unless the mode is differential. Callers decide whether to call it.
func (v *Video) AddSquare(x1, y1, x2, y2 int) {
	x1, x2 = clampInt(x1, 0, W-1), clampInt(x2, 0, W-1)
	y1, y2 = clampInt(y1, 0, H-1), clampInt(y2, 0, H-1)
	c1, c2 := int16(x1>>2), int16(x2>>2)
	for y := y1; y <= y2; y++ {
		merge(&v.scan[y], c1, c2)
		if v.refresh != RefreshDifferential {
			merge(&v.stencil[y], c1, c2)
		}
	}
}

// merge folds columns c1..c2 into a row. An empty row takes them as given,
// even reversed.
func merge(s *span, c1, c2 int16) {
	if s.start == -1 {
		*s = span{c1, c2}
		return
	}
	s.start = min(s.start, c1)
	s.end = max(s.end, c2)
}

// Fill is Fill: an inclusive rectangle in one colour on the current page.
func (v *Video) Fill(x1, y1, x2, y2 int, color byte) {
	if x1 > x2 {
		x1, x2 = x2, x1
	}
	if y1 > y2 {
		y1, y2 = y2, y1
	}
	if v.Clipping {
		if x1 >= v.WinX2 || x2 <= v.WinX1 || y1 >= v.WinY2 || y2 <= v.WinY1 {
			return
		}
		x1, y1 = max(x1, v.WinX1), max(y1, v.WinY1)
		x2, y2 = min(x2, v.WinX2), min(y2, v.WinY2)
	}
	if v.refresh >= RefreshDifferential {
		v.AddSquare(x1, y1, x2, y2)
	}
	for y := y1; y <= y2; y++ {
		for x := x1; x <= x2; x++ {
			if o := y*W + x; o >= 0 && o < Size {
				v.Current[o] = color
			}
		}
	}
}

// flip is the dump part of Toggle_Pages, from step 3 on: get off onto the
// screen, refresh the palette, and do the stencil bookkeeping. mouseX/Y is
// where the cursor is now; the cursor itself is overlaid by the host, but its
// rectangles still enter the lists as in the original.
func (v *Video) flip(mouseX, mouseY int) {
	if v.dump == 0 {
		copy(v.Screen, v.Off)
		switch v.refresh {
		case RefreshDifferential, RefreshStencil:
			v.dump = 2
		case RefreshMasked:
			v.dump = 1
		}
		v.lastMouseX, v.lastMouseY = mouseX, mouseY
	} else {
		v.AddSquare(v.lastMouseX, v.lastMouseY, v.lastMouseX+26, v.lastMouseY+24)
		v.lastMouseX, v.lastMouseY = mouseX, mouseY
		v.AddSquare(mouseX, mouseY, mouseX+26, mouseY+24)
		v.partialDump()
	}
	v.refreshPalette()
	if v.refresh == RefreshStencil {
		v.stencilOutBackground()
		v.scan = v.stencil
		v.clearStencil()
	} else {
		v.clearScan()
	}
	v.SetPageOff()
}

// partialDump is Partial_Screen_Dump_Hi_Res. The four rows that straddle a
// 64 KB bank are copied whole whenever they are dirty at all, and a span whose
// end is before its start still copies one dword.
func (v *Video) partialDump() {
	for y := 0; y < H; y++ {
		s := v.scan[y]
		if s.start == -1 {
			continue
		}
		if y == 102 || y == 204 || y == 307 || y == 409 {
			copy(v.Screen[y*W:(y+1)*W], v.Off[y*W:(y+1)*W])
			continue
		}
		n := int(s.end) - int(s.start) + 1
		if n <= 0 {
			n = 1
		}
		o := (y*160 + int(s.start)) * 4
		copyClamped(v.Screen, v.Off, o, o, 4*n)
	}
}

// stencilOutBackground is Stencil_Out_Background, with the bounds patch the
// shipped binary carries: a span that would reach or pass the end of the page
// copies one dword of back to the start of off instead of restoring its row.
func (v *Video) stencilOutBackground() {
	for y := 0; y < H; y++ {
		s := v.stencil[y]
		if s.start == -1 {
			continue
		}
		o := (y*160 + int(s.start)) * 4
		n := int(s.end) - int(s.start) + 1
		dst := o
		if o < 0 || o >= Size || n < 0 || o+4*n >= Size {
			dst, n = 0, 1
		}
		copyClamped(v.Off, v.Back, dst, o, 4*n)
	}
}

// copyClamped copies n bytes from src[so:] to dst[do:], dropping whatever
// would fall outside either buffer.
func copyClamped(dst, src []byte, do, so, n int) {
	for i := 0; i < n; i++ {
		if do+i >= 0 && do+i < len(dst) && so+i >= 0 && so+i < len(src) {
			dst[do+i] = src[so+i]
		}
	}
}

// refreshPalette is Refresh_Palette: entries marked changed reach the DAC and
// the flags are cleared. The vsync wait per run is the host's frame pacing.
func (v *Video) refreshPalette() {
	for i := range v.Palette {
		c := &v.Palette[i]
		if c.Changed != 0 {
			v.DAC[i] = [3]byte{c.R, c.G, c.B}
			c.Changed = 0
		}
	}
}

// SetPaletteChanges is Set_Palette_Changes: mark start..end changed.
func (v *Video) SetPaletteChanges(start, end int) {
	for i := start; i <= end; i++ {
		v.Palette[i].Changed = 1
	}
}

// ClearPaletteChanges is Clear_Palette_Changes.
func (v *Video) ClearPaletteChanges(start, end int) {
	for i := start; i <= end; i++ {
		v.Palette[i].Changed = 0
	}
}

// DarkenPalette is Darken_Palette_: the DAC shows current_palette darkened by
// percent, and every entry is marked changed so the next flip restores full
// brightness unless the caller clears the flags.
func (v *Video) DarkenPalette(percent int) {
	f := int(byte(100 - percent))
	for i, c := range v.Palette {
		v.DAC[i] = [3]byte{byte(int(c.R) * f / 100), byte(int(c.G) * f / 100), byte(int(c.B) * f / 100)}
		v.Palette[i].Changed = 1
	}
}

func clampInt(v, lo, hi int) int { return max(lo, min(v, hi)) }

// SetWindow is Set_Window: the clipping window, cut to the screen.
func (v *Video) SetWindow(x1, y1, x2, y2 int) {
	v.WinX1, v.WinY1 = max(x1, 0), max(y1, 0)
	v.WinX2, v.WinY2 = min(x2, W-1), min(y2, H-1)
}

// ResetWindow is Reset_Window: the whole screen.
func (v *Video) ResetWindow() { v.WinX1, v.WinY1, v.WinX2, v.WinY2 = 0, 0, W-1, H-1 }

// ClipOn and ClipOff are Clip_On and Clip_Off.
func (v *Video) ClipOn()  { v.Clipping = true }
func (v *Video) ClipOff() { v.Clipping = false }
