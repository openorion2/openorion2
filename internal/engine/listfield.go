package engine

import (
	"fmt"

	"github.com/openorion2/openorion2/internal/font"
)

// String list fields (type 0xa) and Get_List_Field (fields.c). Spec:
// docs/engine/fields-and-input.md, "String list fields".

// AddStringListField is Add_String_List_Field (0x11c83d): a row of a list at
// (x, y), one font height tall, standing for value; *v holds the selected
// value. The hot key is the first character of hotkey.
func (e *Engine) AddStringListField(x, y, w int, s string, active int, v *int16, value int, fillType int, fill [4]int, hotkey string) (int, error) {
	f := &e.f
	t := e.text
	return e.addField(Field{X1: x, Y1: y - 1, X2: x + w, Y2: t.Height() + y + 1, Type: FieldList,
		TextStyle: t.Style, Label: s, ListVar: v, ListValue: int16(value), Active: int16(active),
		Hotkey: upperHotkey(hotkey), FillType: fillType, Fill: fill, TextColors: f.listNormal})
}

// GetListField is Get_List_Field (0x11c4c1): a title row and the rows of
// items under it, one font height plus the vertical spacing apart; returns
// the chosen row, or -1 when cancelled. *v is the highlighted row, which
// follows the pointer. A nil active enables every row.
func (e *Engine) GetListField(x, y, w int, title string, items []string, v *int16, active []byte, fillType int, fill [4]int) (int, error) {
	f := &e.f
	t := e.text
	f.ExitListField = false
	f.listFieldOn = true
	delay := f.autoDelay
	e.SetInputDelay(1)
	e.ClearFields()
	fh, vs := t.Height(), t.VSpacing()
	choice, found, n := -1, false, 0
	ymin := y
	for i, s := range items {
		if s == "" {
			break
		}
		a := 1
		if active != nil {
			a = int(active[i])
		}
		if !found && a != 0 {
			choice, found = i, true
		}
		ymin += fh + vs
		hot := ""
		if f.ListKeyOne {
			hot = "1"
		}
		if i < len(f.ListRowKeys) {
			hot = f.ListRowKeys[i:]
		}
		if _, err := e.AddStringListField(x, ymin, w, s, a, v, i, fillType, fill, hot); err != nil {
			return 0, err
		}
		n++
	}
	t.SetStyle(t.Style, t.Normal)
	f.listNormal, f.listHighlight, f.listSpecial = t.Normal, t.Highlight, t.Special
	for i := 1; i < f.count; i++ {
		if f.fields[i].Type == FieldList {
			f.fields[i].TextColors = f.listNormal
		}
	}
	var titleVar int16
	titleHot := ""
	if f.ListKeyOne {
		titleHot = "1"
	}
	titleField, err := e.AddStringListField(x, y, w, title, 1, &titleVar, 1, 0, [4]int{}, titleHot)
	if err != nil {
		return 0, err
	}
	if *v < 0 || int(*v) >= n || int(*v) < choice {
		if choice < 0 || choice >= n {
			*v = -1
		} else {
			*v = f.fields[choice+1].ListValue
		}
	}
	c := 0
	for {
		e.MarkTime()
		if c, err = e.GetInput(); err != nil {
			return 0, err
		}
		done := c != 0
		if c > 0 && active != nil && c-1 < len(active) && active[c-1] == 0 || c == titleField {
			done = false
		}
		if f.ExitListField {
			done = true
		}
		if done {
			break
		}
		e.V.SetPageOff()
		if err := e.invokeAuto(); err != nil {
			return 0, err
		}
		k, err := e.ScanInput()
		if err != nil {
			return 0, err
		}
		if k != titleField && k > 0 {
			*v = f.fields[k].ListValue
		}
		if err := e.TogglePages(); err != nil {
			return 0, err
		}
		if err := e.ReleaseTime(delay); err != nil {
			return 0, err
		}
	}
	e.ClearFields()
	f.listFieldOn = false
	e.Sys.MouseBuffer()
	e.Sys.MouseBuffer2()
	if c < 0 {
		return -1, nil
	}
	return c - 1, nil
}

// drawListField is Draw_Visible_Fields for a string list row: the selected
// row highlighted (fill type 0x11) or on a gradient, inactive rows in the
// special colours.
func (e *Engine) drawListField(fl *Field) error {
	t := e.text
	f := &e.f
	t.SetStyle(fl.TextStyle, fl.TextColors)
	if *fl.ListVar != fl.ListValue {
		if fl.Active == 0 {
			t.SetStyle(fl.TextStyle, f.listSpecial)
		}
		return t.Print(fl.X1, fl.Y1+1, fl.Label)
	}
	if fl.FillType != 0x11 {
		return fmt.Errorf("Gradient_Fill of list field fill type %d is not ported", fl.FillType)
	}
	t.SetStyle(fl.TextStyle, f.listHighlight)
	return t.Print(fl.X1, fl.Y1+1, fl.Label)
}

// SetListRowKeys gives Get_List_Field's rows the hot keys of keys, row i
// keys[i] (tests); empty restores the original's none.
func (e *Engine) SetListRowKeys(keys string) { e.f.ListRowKeys = keys }

// ListFieldOn reports whether Get_List_Field is running (_list_field_on).
func (e *Engine) ListFieldOn() bool { return e.f.listFieldOn }

// SetHighlightColors is Set_Font_Highlight_Colors.
func (t *Text) SetHighlightColors(c font.Colors) { t.Highlight = c }

// SetSpecialColors is Set_Font_Special_Colors.
func (t *Text) SetSpecialColors(c font.Colors) { t.Special = c }

// SetListKeyOne gives Get_List_Field hot key 1 on every row (unless
// SetListRowKeys names others) and on the title field (tests).
func (e *Engine) SetListKeyOne(on bool) { e.f.ListKeyOne = on }

// ExitListField is Exit_List_Field_ (0x11c817): the running Get_List_Field
// returns as if cancelled.
func (e *Engine) ExitListField() { e.f.ExitListField = true }
