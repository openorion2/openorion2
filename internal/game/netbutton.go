package game

import (
	"github.com/openorion2/openorion2/internal/engine"
	"github.com/openorion2/openorion2/internal/font"
)

// Port-only buttons of the TCP lobby (docs/net/ui.md, "Port-only
// buttons"): an original button with its label erased and another printed.
type netButtonArt struct {
	file   string
	entry  int
	remap  bool               // drawn through the picture remap, as the input box's ACCEPT
	base   [2]int             // the picture's frame drawn up and pressed
	cols   [2][2]int          // the label's columns in frames 0 (up) and 1 (pressed)
	rows   [2][2]int          // and its rows
	text   [2]func(byte) bool // the old label's pixels in each frame
	style  int
	colors [2]font.Colors
	shadow [2]byte // the shadow's colour below the label, 0 for none
}

// netAcceptArt is the input box's ACCEPT (INBOX.LBX 1): dark letters on a
// shaded plate, lit when pressed (on the up frame: the pressed one's A runs
// into the bezel).
var netAcceptArt = netButtonArt{file: "INBOX.LBX", entry: 1, remap: true,
	cols:   [2][2]int{{17, 80}, {17, 80}},
	rows:   [2][2]int{{9, 18}, {9, 18}},
	text:   [2]func(byte) bool{func(v byte) bool { return v < 0x0a }, func(v byte) bool { return v < 0x0a }},
	style:  3,
	colors: [2]font.Colors{font.Solid(0x02), font.Solid(0x1c)},
	shadow: [2]byte{0x05, 0x27}}

// netCancelArt is the game list's CANCEL (MULTIGM.LBX 0x101); its L runs
// into the bezel, whose shades are kept.
var netCancelArt = netButtonArt{file: multiGMLBX, entry: 0x101, base: [2]int{0, 1},
	cols: [2][2]int{{17, 69}, {18, 71}},
	rows: [2][2]int{{4, 15}, {5, 16}},
	text: [2]func(byte) bool{func(v byte) bool { return v != 0xc0 && v != 0xd6 && v != 0xe9 }, func(v byte) bool {
		return v == 0xee || v == 0x63 || v == 0x83 || v >= 0xf7 && v <= 0xfa
	}},
	style:  3,
	colors: [2]font.Colors{font.Solid(0x01), font.Solid(0xf9)},
	shadow: [2]byte{0xe3, 0xee}}

// drawNetButton draws a's frame (0 up, 1 pressed) at (x, y) with label.
func (g *Game) drawNetButton(a *netButtonArt, x, y, frame int, label string) error {
	p, err := g.picture(a.file, a.entry)
	if err != nil {
		return err
	}
	c := p.NewCanvas()
	for i := 0; i <= a.base[frame]; i++ {
		if err := p.Draw(c, i); err != nil {
			return err
		}
	}
	w, isText := p.Width, a.text[frame]
	x1, x2 := a.cols[frame][0], a.cols[frame][1]
	for row := a.rows[frame][0]; row <= a.rows[frame][1]; row++ {
		// Each old letter pixel takes the plate's shade left of it.
		line := c.Pix[row*w : (row+1)*w]
		last := byte(0)
		for col := x1; col <= x2; col++ {
			if !isText(line[col]) {
				last = line[col]
				break
			}
		}
		for col := x1; col <= x2; col++ {
			if isText(line[col]) {
				line[col] = last
			} else {
				last = line[col]
			}
		}
	}
	e, v := g.e, g.e.V
	var remap *[256]byte
	if a.remap {
		remap = e.PictureRemap()
	}
	for row := range p.Height {
		for col := range w {
			i := row*w + col
			if !c.Opaque[i] {
				continue
			}
			px := c.Pix[i]
			if remap != nil {
				px = remap[px]
			}
			v.Current[(y+row)*engine.W+x+col] = px
		}
	}
	if v.RefreshMode() > engine.RefreshMasked {
		v.AddSquare(x, y, x+w-1, y+p.Height-1)
	}
	cl, sh := a.colors[frame], a.shadow[frame]
	if remap != nil {
		for i := range cl {
			cl[i] = remap[cl[i]]
		}
		sh = remap[sh]
	}
	t := e.Text()
	t.SetStyle(a.style, cl)
	// Narrower spacing, then a smaller style, until the label fits.
	for room := x2 - x1 - 1; t.Width([]byte(label)) > room; {
		if t.Spacing > 0 {
			t.SetSpacing(t.Spacing - 1)
			continue
		}
		st := t.Style - 1
		t.SetStyle(st, cl)
		if st == 0 {
			break
		}
	}
	if a.shadow[frame] != 0 {
		t.SetOutlineColor(sh, sh)
		t.Shadow = font.ShadowDown
	}
	r0, r1 := a.rows[frame][0], a.rows[frame][1]
	err = t.PrintCentered(x+(x1+x2+1)/2, y+(r0+r1+1-t.Height())/2, label)
	t.Shadow = font.ShadowNone
	return err
}
