package game

import (
	"bytes"
	"encoding/binary"
	"os"

	"github.com/openorion2/openorion2/internal/clib"
	"github.com/openorion2/openorion2/internal/engine"
	"github.com/openorion2/openorion2/internal/font"
	"github.com/openorion2/openorion2/internal/gfx"
	"github.com/openorion2/openorion2/internal/save"
)

// The end of the game (HISCORE.C): the score screen, the Hall of Fame and
// its file HOF.M2. Spec: docs/screens/endgame.md.

const (
	scoreLBX        = 0x179ba0 // "SCORE.LBX"
	hofFileName     = 0x179bd0 // "HOF.M2"
	hofEscKey       = 0x179baa // "\x1b"
	hofClearKey     = 0x179bdb // "C"
	scoreDot        = 0x179bcb // "."
	scoreFmtInt     = 0x179bc8 // "%d"
	scoreFmtFrac    = 0x179bb1 // ".%d"
	scoreFmtDec     = 0x179bb5 // "%d.%d"
	scoreFmtTimes   = 0x179bbb // "(%d x %s) %d"
	hofDefaultNames = 0x182318 // ten 20-byte names, shuffled in place by Create_Default_HOF_File_
	hofSize         = 0x1c4
	hofVersion      = 0x82
)

// HOF.M2's layout (0x19c5fc in memory).
const (
	hofNames     = 0x2  // [10][20] names
	hofScores    = 0xca // [10]int16
	hofRaces     = 0xde // [10]int16
	hofDiff      = 0xf2 // [10]uint8
	hofRaceNames = 0xfc // [10][20]
)

// hofState holds HISCORE.C's globals: the file's image, the _hi block of
// the screens (0x125 bytes in screen_seg) and the shuffled default names.
type hofState struct {
	file       [hofSize]byte
	entryFlash int16       // _entry_to_flash
	difficulty [5]string   // _difficulty_string, 15 bytes each
	defaults   []byte      // the default names table, copied on first use
	h          hiScore     // the score block's fields
	flag6b     bool        // +0x6b: the next draw clears the screen first
	y          int16       // +0xb2: the next score line
	colors     font.Colors // +0x6e
	colors78   font.Colors // +0x78
	colors80   font.Colors // +0x80
	flash      int8        // +0xb8
	flashDir   int8        // +0xb9
	flashTick  uint8       // +0xba
	hoverRow   int8        // +0xbb
	hover      int         // +0x4e
	escField   int         // +0x4c
	clearField int         // +0x50
	rowFields  [10]int     // +0x54
	sorted     [10][2]int16
	background *gfx.Picture     // +4, SCORE.LBX 0
	elimFrame  *gfx.Picture     // +0x44, SCORE.LBX 1
	racePics   [13]*gfx.Picture // +0xc, SCORE.LBX 2..14
	hall       *gfx.Picture     // +0x48, SCORE.LBX 15
	cursor     *gfx.Picture     // +0, SCORE.LBX 16
	popCount   int16            // +0x9e
	popIcons   [16][2]int16     // +0xbd: colony, colonist
	capRaces   [16]uint8        // +0x8e
	capCount   int16            // +0x8a
	showing    int              // port-only: 1 the score screen, 2 the Hall of Fame
}

func (hf *hofState) i16(off int) int16 { return int16(binary.LittleEndian.Uint16(hf.file[off:])) }

func (hf *hofState) set16(off int, v int16) { binary.LittleEndian.PutUint16(hf.file[off:], uint16(v)) }

func (hf *hofState) name(k int) string {
	return save.CString(hf.file[hofNames+20*k : hofNames+20*k+20])
}

func (hf *hofState) raceName(k int) string {
	return save.CString(hf.file[hofRaceNames+20*k : hofRaceNames+20*k+20])
}

// strcpyTo copies s and its NUL into dst; the bytes after it stay.
func strcpyTo(dst []byte, s []byte) {
	if i := bytes.IndexByte(s, 0); i >= 0 {
		s = s[:i]
	}
	copy(dst, s)
	if len(s) < len(dst) {
		dst[len(s)] = 0
	}
}

// hofPath is HOF.M2 in the game directory.
func (g *Game) hofPath() string { return g.findFile(cStringAt(g.exe, hofFileName)) }

// readHOFFile is Read_HOF_File_ (0x9f4ad): the file, sorted by score,
// lowest first.
func (g *Game) readHOFFile() bool {
	b, err := os.ReadFile(g.hofPath())
	if err != nil {
		return false
	}
	copy(g.hof.file[:], b)
	g.sortHOFScores(&g.hof.sorted)
	g.moveSortedHOFData(&g.hof.sorted)
	return true
}

// writeHOFFile is Write_HOF_File_ (0x9f505). A scratch game directory links
// the original's files: never write through such a link.
func (g *Game) writeHOFFile() {
	path := g.hofPath()
	if fi, err := os.Lstat(path); err == nil && fi.Mode()&os.ModeSymlink != 0 {
		os.Remove(path)
	}
	if err := os.WriteFile(path, g.hof.file[:], 0o644); err != nil {
		g.Log("HOF.M2: %v", err)
	}
}

// sortHOFScores is Sort_HOF_Scores_ (0x9fbe9): (score, entry) pairs in
// ascending order with Watcom's qsort.
func (g *Game) sortHOFScores(out *[10][2]int16) {
	for k := 0; k < 10; k++ {
		out[k] = [2]int16{g.hof.i16(hofScores + 2*k), int16(k)}
	}
	clib.Qsort(out[:], 4, func(a, b *[2]int16) int {
		switch {
		case a[0] > b[0]:
			return 1
		case a[0] < b[0]:
			return -1
		case a[1] > b[1]:
			return 1
		case a[1] < b[1]:
			return -1
		}
		return 0
	})
}

// moveSortedHOFData is Move_Sorted_HOF_Data_ (0x9fc27): the entries in the
// sorted order. The original builds them on its stack with strcpy, so the
// name bytes after each NUL are stale stack (left zero here).
func (g *Game) moveSortedHOFData(sorted *[10][2]int16) {
	hf := &g.hof
	var out [hofSize]byte
	copy(out[:2], hf.file[:2])
	for k := 0; k < 10; k++ {
		i := int(sorted[k][1])
		strcpyTo(out[hofNames+20*k:hofNames+20*k+20], hf.file[hofNames+20*i:hofNames+20*i+20])
		copy(out[hofScores+2*k:], hf.file[hofScores+2*i:hofScores+2*i+2])
		copy(out[hofRaces+2*k:], hf.file[hofRaces+2*i:hofRaces+2*i+2])
		out[hofDiff+k] = hf.file[hofDiff+i]
		strcpyTo(out[hofRaceNames+20*k:hofRaceNames+20*k+20], hf.file[hofRaceNames+20*i:hofRaceNames+20*i+20])
	}
	hf.file = out
}

// createDefaultHOFFile is Create_Default_HOF_File_ (0x9f540): ten scores
// from 1500 down with random races and the default names, which it
// shuffles in its own table for good.
func (g *Game) createDefaultHOFFile() {
	hf := &g.hof
	r := &g.state.Random
	if hf.defaults == nil {
		hf.defaults = append([]byte(nil), g.exe.bytes(hofDefaultNames, 200)...)
	}
	hf.file = [hofSize]byte{}
	hf.set16(0, hofVersion)
	var perm [13]int16
	for k := range perm {
		perm[k] = int16(k)
	}
	for range 10 {
		a := int16(r.Random(0xb)) + 1
		b := int16(r.Random(0xb)) + 1
		if a != b {
			perm[a], perm[b] = perm[b], perm[a]
		}
	}
	tab := hf.defaults
	entry := func(i int) []byte { return tab[20*i : 20*i+20] }
	for range 8 {
		i := int(int16(r.Random(8)) + 1)
		j := int(int16(r.Random(8)) + 1)
		var tmp [20]byte
		strcpyTo(tmp[:], entry(i))
		strcpyTo(entry(i), entry(j))
		strcpyTo(entry(j), tmp[:])
	}
	count, base := int32(10), int32(10*0x7d+0xfa)
	for k := 0; k < 10; k++ {
		a := int32(r.Random(uint32(int16(count)+1))) - 1
		v := a*15 + int32(r.Random(5))*10 + base
		hf.set16(hofScores+2*k, int16(v))
		hf.file[hofDiff+k] = byte(k % 5)
		strcpyTo(hf.file[hofNames+20*k:hofNames+20*k+20], entry(k))
		slot := uint32(raceNames + 0x14)
		if k >= 2 {
			slot = raceNames + 4*uint32(perm[k])
		}
		name, err := g.eStringAt(slot)
		if err != nil {
			g.Log("Create_Default_HOF_File_: %v", err)
		}
		strcpyTo(hf.file[hofRaceNames+20*k:hofRaceNames+20*k+20], []byte(name))
		base -= 0x7d
		count--
	}
	g.writeHOFFile()
}

// loadDifficultyStrings fills _difficulty_string from the E strings.
func (g *Game) loadDifficultyStrings() error {
	es, err := g.eStrings()
	if err != nil {
		return err
	}
	for k, n := range []int{0x221, 0x118, 0xbe, 0x13e, 0x148} {
		g.hof.difficulty[k] = es[n]
	}
	return nil
}

// hallOfFameScreen is Hall_Of_Fame_Screen_ (0x9f286): the ten best,
// entry flash (-1 none) blinking; Esc leaves, C restores the defaults.
func (g *Game) hallOfFameScreen(flash int16) error {
	e, hf := g.e, &g.hof
	hf.entryFlash = flash
	if err := g.loadDifficultyStrings(); err != nil {
		return err
	}
	if !g.readHOFFile() || hf.i16(0) != hofVersion {
		g.createDefaultHOFFile()
	}
	e.DisableCancel()
	e.DeactivateHelpList()
	g.readHOFFile()
	e.AssignAutoFunction(g.drawHallOfFameScreen, 1)
	if err := g.loadHallOfFameScreen(); err != nil {
		return err
	}
	if err := g.initHallOfFameScreen(); err != nil {
		return err
	}
	hf.hover = -1000
	hf.showing = 2
	defer func() { hf.showing = 0 }()
	for {
		r, err := e.GetInput()
		if err != nil {
			return err
		}
		if hf.hover, err = e.ScanInput(); err != nil {
			return err
		}
		// The hover test against +0x52, never updated from -1000, always
		// clears the hovered row.
		if hf.hover != -1000 {
			hf.hoverRow = -1
		}
		if r == hf.escField {
			break
		}
		if r == hf.clearField {
			g.createDefaultHOFFile()
			hf.entryFlash = -1
			if err := g.initHallOfFameScreen(); err != nil {
				return err
			}
		}
		e.V.SetPageOff()
		if err := g.drawHallOfFameScreen(); err != nil {
			return err
		}
		if err := e.TogglePages(); err != nil {
			return err
		}
	}
	if err := e.FastFadeOut(); err != nil {
		return err
	}
	g.currentScreen = g.returnScreen
	e.ClearFields()
	e.DeactivateAutoFunction()
	return nil
}

// loadHallOfFameScreen is Load_Hall_Of_Fame_Screen_ (0x9ede1).
func (g *Game) loadHallOfFameScreen() error {
	hf := &g.hof
	hf.clearBlock()
	file := cStringAt(g.exe, scoreLBX)
	var err error
	if hf.hall, err = g.picture(file, 0xf); err != nil {
		return err
	}
	hf.cursor, err = g.picture(file, 0x10)
	return err
}

// clearBlock is the memset of the _hi block.
func (hf *hofState) clearBlock() {
	hf.h = hiScore{}
	hf.flag6b, hf.y = false, 0
	hf.colors, hf.colors78, hf.colors80 = font.Colors{}, font.Colors{}, font.Colors{}
	hf.flash, hf.flashDir, hf.flashTick, hf.hoverRow = 0, 0, 0, 0
	hf.hover, hf.escField, hf.clearField = 0, 0, 0
	hf.rowFields = [10]int{}
	hf.sorted = [10][2]int16{}
	hf.popCount, hf.capCount = 0, 0
	hf.popIcons = [16][2]int16{}
	hf.capRaces = [16]uint8{}
}

// initHallOfFameScreen is Init_Hall_Of_Fame_Screen_ (0x9f981): the rows
// by score (highest first), the fields, the colours, the fade in.
func (g *Game) initHallOfFameScreen() error {
	e, hf := g.e, &g.hof
	e.ClearFields()
	hf.hoverRow = -2
	hf.flag6b = true
	for k := 0; k < 10; k++ {
		hf.sorted[k] = [2]int16{hf.i16(hofScores + 2*k), int16(k)}
	}
	clib.Qsort(hf.sorted[:], 4, func(a, b *[2]int16) int {
		switch {
		case a[0] > b[0]:
			return -1
		case a[0] < b[0]:
			return 1
		case a[1] > b[1]:
			return -1
		case a[1] < b[1]:
			return 1
		}
		return 0
	})
	y := 0x83
	for k := 0; k < 10; k++ {
		f, err := e.AddHiddenField(0x8b, y, 0x9b, y+0x16, "", 0)
		if err != nil {
			return err
		}
		hf.rowFields[k] = f
		y += 0x1d
	}
	var err error
	if hf.escField, err = e.AddHiddenField(0, 0, 0x27f, 0x1df, cStringAt(g.exe, hofEscKey), 0x29); err != nil {
		return err
	}
	g.moxFontColorsOffset = 0xf0
	hf.colors = g.fontColors2(4, 0xef, 0xf4)
	hf.flash, hf.flashDir, hf.flashTick = 3, 1, 0
	if hf.clearField, err = e.AddMultiHotKeyField(cStringAt(g.exe, hofClearKey)); err != nil {
		return err
	}
	e.DisableDrawAllFields()
	e.SetMouseList(engine.MouseNone)
	if err := e.FastFadeOut(); err != nil {
		return err
	}
	e.V.SetPageOff()
	e.V.Fill(0, 0, 0x27f, 0x1df, 0)
	if err := e.TogglePages(); err != nil {
		return err
	}
	if err := e.LoadPalette(0xc, 0, 0xff); err != nil {
		return err
	}
	e.V.SetPaletteChanges(0, 0xff)
	e.RefreshPalette()
	if err := e.DarkenPalette(100); err != nil {
		return err
	}
	if err := g.loadDifficultyStrings(); err != nil {
		return err
	}
	e.V.SetPageOff()
	if err := g.drawHallOfFameScreen(); err != nil {
		return err
	}
	e.V.ClearPaletteChanges(0, 0xff)
	if err := e.FastFadeIn(); err != nil {
		return err
	}
	e.V.CopyOffToBack()
	if err := e.TogglePages(); err != nil {
		return err
	}
	e.EnableDrawAllFields()
	e.DrawPalette(hf.hall)
	return nil
}

// drawHallOfFameScreen is Draw_Hall_Of_Fame_Screen_ (0x9ee43): the
// smallest fitting styles of the race names and difficulties, then the
// rows, the cursor picture at the pointer and the flash.
func (g *Game) drawHallOfFameScreen() error {
	e, hf := g.e, &g.hof
	t := e.Text()
	style, sp1, sp2 := 4, 2, 2
	diff := func(i int) string {
		if d := int(hf.file[hofDiff+i]); d < len(hf.difficulty) {
			return hf.difficulty[d]
		}
		return ""
	}
	for k := 0; k < 10; k++ {
		i := int(hf.sorted[k][1])
		g.setFittedFontStyle(hf.raceName(i), hf.colors, 4, 2, -1, 0x43, 1)
		style, sp1 = min(style, t.Style), min(sp1, t.Spacing)
		g.setFittedFontStyle(diff(i), hf.colors, 4, 2, -1, 0x56, 1)
		style, sp2 = min(style, t.Style), min(sp2, t.Spacing)
	}
	if hf.flag6b || hf.hoverRow < -2 {
		hf.hoverRow = -1
		e.V.Fill(0, 0, 0x27f, 0x1df, 0)
		if err := e.Draw(0, 0, hf.hall); err != nil {
			return err
		}
		g.fullDrawScreen = false
	}
	lang := g.settings[0xd2] != 0
	v := e.V
	y := 0x87
	for k := 0; k < 10; k++ {
		i := int(hf.sorted[k][1])
		var add int8
		if int16(i) == hf.entryFlash {
			add = hf.flash
		}
		hf.colors78[0] = hf.colors[0]
		for c := 1; c < 8; c++ {
			hf.colors78[c] = hf.colors[c] + byte(add)
		}
		t.SetStyle(4, hf.colors78)
		t.SetSpacing(1)
		v.SetWindow(0xa4, y, 0x119, y+0x14)
		v.ClipOn()
		if err := t.Print(0xa4, y, hf.name(i)); err != nil {
			return err
		}
		if lang {
			t.SetStyle(style, hf.colors78)
			t.SetSpacing(sp1)
		}
		v.SetWindow(0x124, y, 0x162, y+0x14)
		v.ClipOn()
		if err := t.Print(0x124, int(int16(0x10-t.Height()+y)), hf.raceName(i)); err != nil {
			return err
		}
		if lang {
			t.SetStyle(style, hf.colors78)
			t.SetSpacing(sp2)
		}
		v.SetWindow(0x16c, y, 0x1c1, y+0x14)
		v.ClipOn()
		if err := t.Print(0x16c, int(int16(0x10-t.Height()+y)), diff(i)); err != nil {
			return err
		}
		if lang {
			t.SetStyle(4, hf.colors78)
			t.SetSpacing(1)
		}
		v.SetWindow(0x1ca, y, 0x1f4, y+0x14)
		v.ClipOn()
		if err := t.PrintRight(0x1f4, y, itoa10(int32(hf.i16(hofScores+2*i)))); err != nil {
			return err
		}
		y += 0x1d
	}
	v.ResetWindow()
	v.ClipOff()
	if !hf.flag6b {
		v.SetWindow(0, 0, 0x280, 0x1e0)
		v.ClipOn()
		x, py := e.Pointer()
		x, py = min(int(int16(x)), 0x27c), min(int(int16(py)), 0x1dc)
		if err := e.Draw(x, py, hf.cursor); err != nil {
			return err
		}
		v.ResetWindow()
		v.ClipOff()
	}
	if hf.entryFlash > -1 && hf.entryFlash < 10 {
		hf.flashTick++
		if hf.flashTick%6 == 0 {
			hf.flash += hf.flashDir
			switch hf.flash {
			case -1:
				hf.flashDir = 1
				hf.flashTick = 0
			case 6:
				hf.flashDir = -1
				hf.flashTick = 0
			}
		}
	}
	hf.flag6b = false
	return nil
}
