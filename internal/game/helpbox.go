package game

import (
	"encoding/binary"
	"fmt"

	"github.com/openorion2/openorion2/internal/gfx"
	"github.com/openorion2/openorion2/internal/lbx"
	"github.com/openorion2/openorion2/internal/save"
)

// The right-click help box: Draw_Help_Entry_ (0xa59a1), which the engine's
// Check_Help_List calls for the help entry under the pointer. Spec:
// docs/screens/help-box.md.

// helpRecordSize is the size of one HELP.LBX record.
const helpRecordSize = 0x57b

// helpMaxChain is how many linked records one box shows at most.
const helpMaxChain = 9

// helpRecord is one 0x57b-byte record of HELP.LBX entry 0.
type helpRecord struct {
	title   []byte // +0x00, 80 bytes; 0x14 first means none
	lbxName string // +0x50, 14 bytes; empty means no picture
	entry   int    // +0x5e, s16
	frame   int    // +0x60, u8
	next    int    // +0x63, s32; -1 means the next record
	text    []byte // +0x67
}

func parseHelpRecord(r []byte) helpRecord {
	return helpRecord{
		title:   r[:0x50],
		lbxName: save.CString(r[0x50:0x5e]),
		entry:   int(int16(binary.LittleEndian.Uint16(r[0x5e:]))),
		frame:   int(r[0x60]),
		next:    int(int32(binary.LittleEndian.Uint32(r[0x63:]))),
		text:    r[0x67:],
	}
}

// helpLBXName is Get_Help_Lbx_Name_ (0xa6547): by the language setting.
func (g *Game) helpLBXName() string {
	switch g.settings[0xd2] {
	case 1:
		return "GER_HELP.LBX"
	case 2:
		return "FRE_HELP.LBX"
	case 3:
		return "SPA_HELP.LBX"
	case 4:
		return "ITA_HELP.LBX"
	}
	return "HELP.LBX"
}

// helpChain reads the records Draw_Help_Entry_ shows for id: each record
// names the next (-1 for id+1), up to record 0 or nine records.
func helpChain(t lbx.Table, id int) ([]helpRecord, error) {
	if t.Size != helpRecordSize {
		return nil, fmt.Errorf("help records are %d bytes, not %d", t.Size, helpRecordSize)
	}
	var recs []helpRecord
	for {
		if id < 0 || (id+1)*t.Size > len(t.Data) {
			return nil, fmt.Errorf("help record %d out of range", id)
		}
		r := parseHelpRecord(t.Record(id))
		recs = append(recs, r)
		if r.next == -1 {
			id++
		} else {
			id = r.next
		}
		if id == 0 || len(recs) >= helpMaxChain {
			return recs, nil
		}
	}
}

// drawHelpEntry is Draw_Help_Entry_ (0xa59a1): the chain of help records
// for id in a text box, titles in style 4 beside an optional picture, the
// text in style 2 below; any input closes it.
func (g *Game) drawHelpEntry(id int) error {
	e, u := g.e, &g.ub
	t := e.Text()
	if err := g.textBoxStartup(); err != nil {
		return err
	}
	b0, b2 := g.getMoxTextBoxBaseColors(0)
	c4 := g.fontColors2(4, b0, b2)
	c2 := g.fontColors2(2, b0, b2)
	g.setRemapFontStyle(4, c4)
	g.setRemapFontStyle(2, c2)
	a, err := g.archive(g.helpLBXName())
	if err != nil {
		return err
	}
	tab, err := a.Table(0)
	if err != nil {
		return err
	}
	recs, err := helpChain(tab, id)
	if err != nil {
		return err
	}
	for _, r := range recs {
		if _, err := g.formattedParagraphMaxHeight(0x153, save.CString(r.text)); err != nil {
			return err
		}
	}
	closeBtn, err := g.picture("TEXTBOX.LBX", 3)
	if err != nil {
		return err
	}
	n := len(recs)
	pics := make([]*gfx.Picture, n)
	titleW, titleY, textY, picX, picY := make([]int, n), make([]int, n), make([]int, n), make([]int, n), make([]int, n)
	y := 0xb
	var ty, th int
	for i, r := range recs {
		pad := 0
		titleW[i], titleY[i] = 0x153, y
		if r.lbxName != "" {
			p, err := g.picture(r.lbxName, r.entry)
			if err != nil {
				return err
			}
			pics[i] = p
			p.CurrentFrame = min(r.frame, p.Frames-1)
			bmp := gfx.NewBlankBitmap(1, 1)
			bmp.DrawPictureToBitmap(p)
			x1, y1, w, h := bmp.ActualSize()
			picX[i] = int(int16(0x162 - (w + x1)))
			picY[i] = int(int16(titleY[i] - y1))
			titleW[i] = int(int16(0x153 - (w + 6)))
			pad = h
		}
		t.SetStyle(4, c4)
		title := save.CString(r.title)
		h := 0
		if r.title[0] != 0x14 {
			if h, err = g.formattedParagraphMaxHeight(titleW[i], title); err != nil {
				return err
			}
		}
		if int16(pad) > 0 && int16(h) < int16(pad) {
			h = pad
		}
		ty = y + h + 4
		if i == 0 && int16(ty) < 0x2a {
			ty = 0x2a
		}
		textY[i] = int(int16(ty))
		t.SetStyle(2, c2)
		if th, err = g.formattedParagraphMaxHeight(0x153, save.CString(r.text)); err != nil {
			return err
		}
		y = ty + th + 8
	}
	bx, by, err := g.boxCenteredOnXY(int(uint16(ty + th - 0x22)))
	if err != nil {
		return err
	}
	for i, r := range recs {
		if r.title[0] != 0x14 {
			g.setRemapFontStyle(4, c4)
			if err := g.printFormattedParagraph(int(int16(bx+0x14)), int(int16(by+titleY[i])), titleW[i], save.CString(r.title), 0); err != nil {
				return err
			}
			g.setRemapFontStyle(2, c2)
		}
		if err := g.printFormattedParagraph(int(int16(bx+0x14)), int(int16(by+textY[i])), 0x153, save.CString(r.text), 0); err != nil {
			return err
		}
		if p := pics[i]; p != nil {
			p.CurrentFrame = min(r.frame, p.Frames-1)
			if err := e.Draw(bx+picX[i], by+picY[i], p); err != nil {
				return err
			}
		}
	}
	e.V.CopyOffToBack()
	if _, err := e.AddHiddenField(0, 0, 0x27f, 0x1df, "\x1b", 0x29); err != nil {
		return err
	}
	if err := g.textBoxGetInput(0); err != nil {
		return err
	}
	closeBtn.CurrentFrame = min(1, closeBtn.Frames-1)
	if err := g.remapDraw(bx+0x9e, u.bottomY-0x1b, closeBtn); err != nil {
		return err
	}
	if err := g.textBoxCleanup(); err != nil {
		return err
	}
	g.fullDrawScreen = true
	return nil
}
