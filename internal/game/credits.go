package game

import (
	"strings"

	"github.com/openorion2/openorion2/internal/font"
	"github.com/openorion2/openorion2/internal/gfx"
	"github.com/openorion2/openorion2/internal/save"
)

// The credits that scroll over the main menu (MAINMENU.C: Load_Credits_,
// Init_Credits_, Draw_Credits_, Copy_Credit_String_To_Bitmap_). Spec:
// docs/screens/main-menu.md, "Credits".

const (
	creditsFadeIn  = 0x80acc // the entering line's shades (in the code segment)
	creditsNormal  = 0x80ad4
	creditsFadeOut = 0x80adc
	creditsDot     = 0x1796a6 // "."
	creditsTop     = 0xb3
	creditsBottom  = 0x1a6
	creditsX       = 0x42
	creditsLineH   = 0xe
	creditsWidth   = 0x121
)

// creditsNames are Init_Credits_'s files by language (setting 0xd2).
var creditsNames = [5]uint32{0x1796c8, 0x1796d4, 0x1796e1, 0x1796ee, 0x1796fb}

// credits holds the credits' globals.
type credits struct {
	lines   [][2]string // _credits: two columns of 0x7f bytes per line
	y       int32       // _credit_y
	delay   int16       // _credit_num: draws to skip before scrolling
	style   int16       // _credits_font_style
	column  byte        // _credits_column_one: x of the second column
	spacing byte        // _credits_font_spacing
	enabled bool        // _do_credits
	bitmap  *gfx.Picture
}

// initCredits is Init_Credits_ (0x81b7c): the file and font by language.
func (g *Game) initCredits() string {
	c := &g.credits
	lang := g.settings[0xd2]
	if lang > 4 {
		lang = 0
	}
	c.column, c.style, c.spacing = [5]byte{0x7c, 0x9a, 0xa4, 0x92, 0x92}[lang], 2, 1
	switch lang {
	case 0:
		c.style = 3
	case 1:
		c.spacing = 2
	}
	return cStringAt(g.exe, creditsNames[lang])
}

// loadCredits is Load_Credits_ (0x8156b): record 0 of the file's table
// holds the line count, then each line's two columns follow.
func (g *Game) loadCredits() error {
	c := &g.credits
	name := g.initCredits()
	g.e.MoxUpdate()
	a, err := g.archive(strings.ToUpper(name))
	if err != nil {
		return err
	}
	t, err := a.Table(0)
	if err != nil {
		return err
	}
	rec := func(i int) string {
		if i >= t.Count {
			return ""
		}
		return save.CString(t.Record(i))
	}
	g.e.MoxUpdate()
	n := int16(atoi(rec(0)))
	c.lines = c.lines[:0]
	for i := int16(0); i < n; i++ {
		g.e.MoxUpdate()
		c.lines = append(c.lines, [2]string{rec(2*int(i) + 1), rec(2*int(i) + 2)})
	}
	c.enabled, c.delay, c.y = true, 0, creditsBottom
	c.bitmap = gfx.NewBlankBitmap(creditsWidth, creditsLineH)
	g.e.MoxUpdate()
	return nil
}

// drawCredits is Draw_Credits_ (0x81147): the lines move up two pixels per
// draw through the window y 0xb3..0x1a6, the top one fading out and the
// bottom one in; after the last one has gone, ten draws pause.
func (g *Game) drawCredits() error {
	e, c := g.e, &g.credits
	n := int32(len(c.lines))
	if c.delay > 0 {
		c.delay--
	} else {
		c.y -= 2
		top := int16(-1)
		if c.y <= creditsTop {
			top = int16((creditsTop - c.y) / creditsLineH)
		}
		bottom := int16((creditsBottom - c.y) / creditsLineH)
		ly := int16(c.y)
		t := e.Text()
		for i := int16(0); int32(i) < n; i++ {
			var colors font.Colors
			switch i {
			case top:
				k := int32(int16(c.y-creditsTop)) % creditsLineH / 3
				colors = g.exeColors(creditsFadeOut)
				for j := 1; j < 8; j++ {
					colors[j] += byte(2 * k)
				}
			case bottom:
				k := (creditsBottom - c.y) % creditsLineH / 4
				colors = g.exeColors(creditsFadeIn)
				for j := 1; j < 8; j++ {
					colors[j] += byte(2 * k)
				}
			default:
				colors = g.exeColors(creditsNormal)
			}
			t.SetOutlineColor(0xfd, 0xfd)
			t.SetStyleShadow(int(c.style), colors, font.Outline)
			t.SetSpacing(int(c.spacing))
			c.bitmap = gfx.NewBlankBitmap(creditsWidth, creditsLineH)
			if err := g.copyCreditStringToBitmap(int(i), c.bitmap); err != nil {
				return err
			}
			e.V.SetWindow(creditsX, creditsTop, 0x162, creditsBottom)
			e.V.ClipOn()
			err := e.Draw(creditsX, int(ly), c.bitmap)
			e.V.ClipOff()
			if err != nil {
				return err
			}
			ly += creditsLineH
		}
	}
	if d := c.y - creditsTop; max(d, -d) > n*creditsLineH {
		c.resetCredits()
	}
	return nil
}

// resetCredits restarts the credits from the bottom after a pause.
func (c *credits) resetCredits() { c.delay, c.y = 10, creditsBottom }

// copyCreditStringToBitmap is Copy_Credit_String_To_Bitmap_ (0x80c8a): the
// first column at 0, dots up to the second column when both are set, the
// second column four pixels after _credits_column_one.
func (g *Game) copyCreditStringToBitmap(i int, b *gfx.Picture) error {
	c, t := &g.credits, g.e.Text()
	one, two := c.lines[i][0], c.lines[i][1]
	if one == "" && two == "" {
		return nil
	}
	tg := font.Target{Pix: b.Pixels(), Stride: b.Width, H: b.Height}
	x := int16(0)
	if err := t.ToBitmap(tg, int(x), 0, []byte(one)); err != nil {
		return err
	}
	if one != "" {
		x += int16(t.Width([]byte(one)) + 4)
	}
	if one != "" && two != "" {
		dot := cStringAt(g.exe, creditsDot)
		for {
			if err := t.ToBitmap(tg, int(x), 0, []byte(dot)); err != nil {
				return err
			}
			x += int16(t.Width([]byte(dot)))
			if int16(c.column) <= x {
				break
			}
		}
	}
	return t.ToBitmap(tg, int(c.column)+4, 0, []byte(two))
}

// atoi is the C library's atoi_: leading white space, a sign, digits.
func atoi(s string) int32 {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] >= '\t' && s[i] <= '\r') {
		i++
	}
	neg := false
	if i < len(s) && (s[i] == '-' || s[i] == '+') {
		neg = s[i] == '-'
		i++
	}
	var n int32
	for ; i < len(s) && s[i] >= '0' && s[i] <= '9'; i++ {
		n = n*10 + int32(s[i]-'0')
	}
	if neg {
		return -n
	}
	return n
}
