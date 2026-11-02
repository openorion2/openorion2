package engine

import "github.com/openorion2/openorion2/internal/font"

// PrintParagraphToBitmap is _Print_Paragraph_Bill_ (0x10370a) with print
// flag 1 and a bitmap: the lines go through the To_Bitmap printers.
func (t *Text) PrintParagraphToBitmap(x, y, maxw int, s string, typ int, bm font.Target) error {
	draw := func(x, y, maxw int16, line []byte, typ int) error {
		switch typ {
		case ParaLeft:
			return t.ToBitmap(bm, int(x), int(y), line)
		case ParaRight:
			return t.RightToBitmap(bm, int(int16(x+maxw-1)), int(y), line)
		case ParaCentered:
			return t.CenteredToBitmap(bm, int(int16(x+maxw/2)), int(y), line)
		case ParaFull:
			return t.FullToBitmap(bm, int(x), int(y), line, int(maxw))
		}
		return nil
	}
	return t.paragraphBill(x, y, maxw, s, typ, draw)
}
