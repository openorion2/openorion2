package engine

import (
	"fmt"
	"strings"

	"github.com/openorion2/openorion2/internal/font"
	"github.com/openorion2/openorion2/internal/gfx"
	"github.com/openorion2/openorion2/internal/host"
)

// Field types (s_fields.type) that are ported.
const (
	FieldButton    = 0x0
	FieldRadio     = 0x1 // a button pressed while *Variable is set
	FieldLocked    = 0x2 // a button that sets *Variable to 1 and stays down
	FieldMulti     = 0x3 // one of a group: *Variable = Value while down
	FieldHidden    = 0x7 // hidden fields and hot keys
	FieldScroll    = 0x6
	FieldMultiHot  = 0x8
	FieldGrid      = 0xc
	FieldIrregular = 0xd
	FieldCircle    = 0xe // Add_Clipped_Circle_Field: a disc, hit-tested without the pointer offset
	FieldList      = 0xa // string list rows and their title
	FieldString    = 0xb // continuous string input
)

// maxFields is max_fields_count. The array has one slot more for the
// off-by-one overflow of Add_Irregular_Button_Field.
const maxFields = 250

// NoField is what the game stores for a field it did not create.
const NoField = -1000

// Field is the part of struct s_fields the ported field types use. The
// original's var1..var7 union is split into named members per type.
type Field struct {
	X1, Y1, X2, Y2 int // inclusive
	Type           int
	Hotkey         byte
	Sound          int // button_sound

	// Buttons (0, 0xd): var1 string, var6 picture, var4.min_value full-store
	// flag, text_style / text_color from the font current when added.
	Label      string
	Pic        *gfx.Picture
	FullStore  bool
	TextStyle  int
	TextColors font.Colors
	DrawX      int // var2, 0xd only
	DrawY      int // var3, 0xd only

	// Grid (0xc): var2/var3 box size, var4/var5 the cell outputs.
	BoxW, BoxH int
	GridX      *int
	GridY      *int

	// Multi button (3): var2 the value it stores into var3 (Variable).
	Value int16

	// Scroll (6): var1/var2 the valid range, var3 the variable, var4/var5
	// the value range, var6 0 horizontal / 1 vertical.
	MinValid, MaxValid int16
	Variable           *int16
	MinValue, MaxValue int16
	Vertical           bool

	// Multi hot key (8): var1 string, var2 progress, var3 length.
	Multi    string
	Progress int
	MultiLen int

	// Continuous string input (0xb): var1 the string it edits in place,
	// var6 its limit, var3 the fill colour, var2 a marker, var7 the shadow.
	Str        []byte
	MaxChars   int
	FillColor  int
	Marker     *gfx.Picture
	ShadowMode int16
	Masked     bool // port-only: shown as '*' (a password)
	Long       bool // port-only: takes text past its width, showing the end

	// String list (0xa): var1 the row, var2 the value it stands for, var3
	// the list's variable; active_flag, fill_type and var4..var7 the fill.
	ListVar   *int16
	ListValue int16
	Active    int16
	FillType  int
	Fill      [4]int

	// Circle (0xe): var1/var2 the centre, var3 the squared radius.
	CX, CY int
	R2     int
}

// HelpEntry is one record of a help list: an id and the rectangle it covers.
type HelpEntry struct{ ID, X1, Y1, X2, Y2 int }

// MouseRegion is one s_mouse_list entry: the cursor image to show inside the
// rectangle, and the hit-test offset it carries.
type MouseRegion struct{ Image, Center, X1, Y1, X2, Y2 int }

// Standard cursor lists.
var (
	MouseNone  = []MouseRegion{{0, 0, 0, 0, 639, 479}} // _mouse_list_none
	MouseArrow = []MouseRegion{{1, 0, 0, 0, 639, 479}} // _mouse_list_arrow
)

// fieldState is the fields.c, mouse.c and key.c state the ported paths touch.
type fieldState struct {
	fields            [maxFields + 1]Field
	count             int // fields_count, the next free number
	inputDelay        int
	downMouseButton   int
	selectedCount     int
	autoInputVariable int

	autoActive bool
	autoFn     func() error
	autoDelay  int
	updateFn   func()

	MouseCancelDisabled bool
	MouseAutoExit       bool
	DynamicFieldRefresh bool
	drawFieldsEnabled   bool

	multiActive  bool // multi_hotkey_active_field
	outlineColor int  // field_outline_color, debug outlines only
	downX, downY int  // down_x, down_y: text shift of a pressed button
	drawErr      error

	helpActive bool
	help       []HelpEntry
	helpFn     func(id int) error // Draw_Help_Entry_, set by the game

	pressedButton      bool
	pressedX, pressedY int
	LastButton         int
	LastX, LastY       int

	mouseList    []MouseRegion
	pointerIndex int    // current_mouse_list_ptr, -1 for none
	mouseImages  []byte // mouse_palette: 20 images of 24x24 after the colours

	// list_normal_color, list_highlight_colors, list_special_colors: the
	// colours every string list row prints with, set by Get_List_Field.
	listNormal, listHighlight, listSpecial font.Colors
	listFieldOn                            bool // _list_field_on
	// ListRowKeys, when set, gives row i of Get_List_Field hot key
	// ListRowKeys[i] (tests drive the lists by keys).
	ListRowKeys string
	// ListKeyOne gives every row of Get_List_Field and its title field hot
	// key 1 (tests; at 0x11c5a9, 0x11c682 the original has none).
	ListKeyOne bool

	// The text input state: input_field_active, active_input_field_number,
	// cursor_on, cursor_count, active_input_field_first, continuous_string.
	inputActive   int
	activeInput   int
	cursorOn      int
	cursorCount   int
	inputFirst    bool
	cont          [0x108]byte
	ExitListField bool // exit_the_list_field
}

func (f *fieldState) init() {
	f.count = 1
	f.downMouseButton = -1
	f.autoDelay = 2
	f.DynamicFieldRefresh = true
	f.drawFieldsEnabled = true
	f.mouseList = MouseArrow
	f.outlineColor = 0x0e
	f.downX, f.downY = 1, -1
	f.activeInput = -1
}

// ClearFields is Clear_Fields. It drops a pending click but leaves the key
// queue, the help list, the auto function and selected_count alone.
func (e *Engine) ClearFields() {
	f := &e.f
	f.count = 1
	f.inputDelay = 0
	f.downMouseButton = -1
	f.autoInputVariable = 0
	f.multiActive = false
	f.inputActive, f.activeInput = 0, -1
	e.Sys.MouseBuffer()
	e.Sys.MouseBuffer2()
}

// ClearFieldsAbove is Clear_Fields_Above: keep fields 0..n.
func (e *Engine) ClearFieldsAbove(n int) {
	e.f.count = n + 1
	e.f.downMouseButton = -1
	e.Sys.MouseBuffer()
	e.Sys.MouseBuffer2()
}

// FieldsCount is fields_count, the number the next field will get.
func (e *Engine) FieldsCount() int { return e.f.count }

// DisableCancel is Disable_Cancel: the right button and Esc stop meaning -1;
// a right click on a field returns -field.
func (e *Engine) DisableCancel() { e.f.MouseCancelDisabled = true }

// EnableCancel is Enable_Cancel.
func (e *Engine) EnableCancel() { e.f.MouseCancelDisabled = false }

// CancelDisabled tells whether DisableCancel is in force (port-only).
func (e *Engine) CancelDisabled() bool { return e.f.MouseCancelDisabled }

func (e *Engine) addField(fl Field) (int, error) {
	f := &e.f
	if f.count >= maxFields {
		return 0, fmt.Errorf("Too many fields")
	}
	f.fields[f.count] = fl
	f.count++
	return f.count - 1, nil
}

func upperHotkey(s string) byte {
	if s == "" {
		return 0
	}
	c := s[0]
	if c > 0x60 && c < 0x7b {
		c -= 0x20
	}
	return c
}

// AddHiddenField is Add_Hidden_Field: an invisible clickable rectangle with a
// hot key (first character of hotkey, upper-cased) and a click sound.
func (e *Engine) AddHiddenField(x1, y1, x2, y2 int, hotkey string, sound int) (int, error) {
	if x1 > x2 || y1 > y2 {
		return 0, fmt.Errorf("ERROR: Bad Rect in Add_Hidden_Field")
	}
	return e.addField(Field{X1: x1, Y1: y1, X2: x2, Y2: y2, Type: FieldHidden, Hotkey: upperHotkey(hotkey), Sound: sound})
}

// AddClippedHiddenField is Add_Clipped_Hidden_Field (0x114a00): the field
// cut to the screen window; -10000 when it lies outside.
func (e *Engine) AddClippedHiddenField(x1, y1, x2, y2 int, hotkey string, sound int) (int, error) {
	v := e.V
	if x2 < v.WinX1 || v.WinX2 < x1 || y2 < v.WinY1 || v.WinY2 < y1 {
		return -10000, nil
	}
	x1, y1 = max(x1, v.WinX1), max(y1, v.WinY1)
	x2, y2 = min(x2, v.WinX2), min(y2, v.WinY2)
	return e.addField(Field{X1: x1, Y1: y1, X2: x2, Y2: y2, Type: FieldHidden, Hotkey: upperHotkey(hotkey), Sound: sound})
}

// AddClippedCircleField is Add_Clipped_Circle_Field (0x1146fd): a disc of
// radius r whose box is cut to the screen window; -10000 when outside.
func (e *Engine) AddClippedCircleField(x, y, r int, hotkey string, sound int) (int, error) {
	v := e.V
	x, y, r = int(int16(x)), int(int16(y)), int(int16(r))
	x1, x2, y1, y2 := x-r, x+r, y-r, y+r
	if f := &e.f; f.count >= maxFields {
		return 0, fmt.Errorf("Too many fields")
	}
	if x2 < v.WinX1 || v.WinX2 < x1 || y2 < v.WinY1 || v.WinY2 < y1 {
		return -10000, nil
	}
	x1, y1 = max(x1, v.WinX1), max(y1, v.WinY1)
	x2, y2 = min(x2, v.WinX2), min(y2, v.WinY2)
	return e.addField(Field{X1: x1, Y1: y1, X2: x2, Y2: y2, Type: FieldCircle, CX: x, CY: y, R2: r * r,
		Hotkey: upperHotkey(hotkey), Sound: sound})
}

// PointerImage is Get_Pointer_Image_Number (0x124039): the image of the
// region the last Check_Mouse_Shape found (0 for none).
func (e *Engine) PointerImage() int {
	if e.f.pointerIndex < 0 || e.f.pointerIndex >= len(e.f.mouseList) {
		return 0
	}
	return e.f.mouseList[e.f.pointerIndex].Image
}

// PointerOffset is Get_Pointer_Offset.
func (e *Engine) PointerOffset() int { return e.pointerOffset() }

// AddHotKey is Add_Hot_Key: a hidden field the mouse can never reach. The
// original leaves its sound stale; the slot's previous value is kept.
func (e *Engine) AddHotKey(hotkey string) (int, error) {
	f := &e.f
	sound := 0
	if f.count < maxFields {
		sound = f.fields[f.count].Sound
	}
	return e.addField(Field{X1: 5000, Y1: 5000, X2: 5000, Y2: 5000, Type: FieldHidden, Hotkey: upperHotkey(hotkey), Sound: sound})
}

// SetInputDelay is Set_Input_Delay.
func (e *Engine) SetInputDelay(n int) {
	e.f.inputDelay = n
	e.Sys.MouseBuffer()
	e.Sys.MouseBuffer2()
}

// AssignAutoFunction is Assign_Auto_Function.
func (e *Engine) AssignAutoFunction(fn func() error, delay int) {
	e.f.autoFn = fn
	e.f.autoActive = true
	if delay < 1 || delay > 9 {
		delay = 2
	}
	e.f.autoDelay = delay
}

// DeactivateAutoFunction is Deactivate_Auto_Function.
func (e *Engine) DeactivateAutoFunction() { e.f.autoActive = false }

// ActivateAutoFunction is Activate_Auto_Function.
func (e *Engine) ActivateAutoFunction() { e.f.autoActive = true }

// SetUpdateFunction sets update_function, which main2_ points at Mox_Update_.
func (e *Engine) SetUpdateFunction(fn func()) { e.f.updateFn = fn }

// SetHelpList is Set_Help_List.
func (e *Engine) SetHelpList(list []HelpEntry) {
	e.f.help = list
	e.f.helpActive = true
}

// SetFieldHotKey sets field n's hot key (tests drive click-only fields by
// keys).
func (e *Engine) SetFieldHotKey(n int, key byte) { e.f.fields[n].Hotkey = key }

// HelpList is the help list and whether it is active, as Save_Help_List_
// (0x6fcab) keeps them.
func (e *Engine) HelpList() ([]HelpEntry, bool) { return e.f.help, e.f.helpActive }

// DeactivateHelpList is Deactivate_Help_List.
func (e *Engine) DeactivateHelpList() {
	e.f.help = nil
	e.f.helpActive = false
}

// SetMouseList is Set_Mouse_List.
func (e *Engine) SetMouseList(list []MouseRegion) {
	e.f.mouseList = list
	e.checkMouseShape()
}

// DisableDrawAllFields and EnableDrawAllFields are the fields.c switches
// Fade_Into_Screen_ uses around its redraw.
func (e *Engine) DisableDrawAllFields() { e.f.drawFieldsEnabled = false }

// EnableDrawAllFields re-enables drawing fields on flips.
func (e *Engine) EnableDrawAllFields() { e.f.drawFieldsEnabled = true }

// callAuto is Call_Auto_Function.
func (e *Engine) callAuto() error {
	f := &e.f
	if f.autoActive && f.autoFn != nil {
		e.MarkTime()
		e.V.SetPageOff()
		if err := f.autoFn(); err != nil {
			return err
		}
		if err := e.TogglePages(); err != nil {
			return err
		}
		if err := e.ReleaseTime(f.autoDelay); err != nil {
			return err
		}
	} else if err := e.TogglePages(); err != nil {
		return err
	}
	if f.updateFn != nil {
		f.updateFn()
	}
	return nil
}

// quickCallAuto is Quick_Call_Auto_Function.
func (e *Engine) quickCallAuto() error {
	f := &e.f
	e.MarkTime()
	if f.autoActive && f.autoFn != nil {
		e.V.SetPageOff()
		if err := f.autoFn(); err != nil {
			return err
		}
	}
	if err := e.TogglePages(); err != nil {
		return err
	}
	if err := e.ReleaseTime(1); err != nil {
		return err
	}
	if f.updateFn != nil {
		f.updateFn()
	}
	return nil
}

// checkMouseShape is Check_Mouse_Shape: the last region containing the
// pointer, each tested with its own offset, picks the cursor.
func (e *Engine) checkMouseShape() int {
	mx, my, _ := e.Sys.Mouse()
	f := &e.f
	f.pointerIndex = -1
	for i, r := range f.mouseList {
		if r.X1 <= mx+r.Center && mx+r.Center <= r.X2 && r.Y1 <= my+r.Center && my+r.Center <= r.Y2 {
			f.pointerIndex = i
		}
	}
	if f.pointerIndex == -1 {
		return 0
	}
	return f.mouseList[f.pointerIndex].Image
}

// pointerOffset is Get_Pointer_Offset. With no region the original reads the
// entry before the list; every list in use covers the whole screen.
func (e *Engine) pointerOffset() int {
	if e.f.pointerIndex < 0 {
		return 0
	}
	return e.f.mouseList[e.f.pointerIndex].Center
}

// cursorImage returns the current cursor picture, or nil for none.
func (e *Engine) cursorImage() []byte {
	shape := e.checkMouseShape()
	img := e.f.mouseImages
	if shape <= 0 || len(img) < shape*0x240 {
		return nil
	}
	return img[(shape-1)*0x240 : shape*0x240]
}

func (e *Engine) mouseButton() int {
	_, _, b := e.Sys.Mouse()
	return b & 3
}

// fieldAt is the test Scan_Field (0x113fb9) and the buffered-click path
// (0x11dc58) share: the rectangle with the pointer offset, then for a circle
// the squared distance from the raw point.
func (e *Engine) fieldAt(x, y, off int) (int, error) {
	f := &e.f
	for i := 1; i < f.count; i++ {
		fl := &f.fields[i]
		if x+off < fl.X1 || x+off > fl.X2 || y+off < fl.Y1 || y+off > fl.Y2 {
			continue
		}
		if fl.Type != FieldCircle {
			return i, nil
		}
		dx, dy := int32(x-fl.CX), int32(y-fl.CY)
		if dx*dx+dy*dy <= int32(fl.R2) {
			return i, nil
		}
	}
	return 0, nil
}

// scanField is Scan_Field: the first field under the pointer.
func (e *Engine) scanField() (int, error) {
	e.checkMouseShape()
	off := e.pointerOffset()
	mx, my, _ := e.Sys.Mouse()
	return e.fieldAt(mx, my, off)
}

// ScanInput is Scan_Input: the field under the pointer, for hover effects.
func (e *Engine) ScanInput() (int, error) {
	n, err := e.scanField()
	if err != nil || n == 0 {
		return n, err
	}
	f := &e.f
	fl := &f.fields[n]
	switch fl.Type {
	case FieldButton, FieldRadio, FieldLocked, FieldMulti, FieldHidden, FieldScroll, FieldMultiHot, FieldIrregular, FieldString, FieldCircle:
	case -0x3e9, -1, -2, -3: // buttons Disable_Draw_Field switched off
	case FieldList:
		if fl.Active == 0 {
			return 0, nil
		}
	case FieldGrid:
		// The hovered cell, without pointer offset or clamping; skipped
		// while another grid is pushed (never, after Get_Input).
		d := f.downMouseButton
		if d == -1 || f.fields[d].Type != FieldGrid {
			mx, my, _ := e.Sys.Mouse()
			*fl.GridX = (mx - fl.X1) / fl.BoxW
			*fl.GridY = (my - fl.Y1) / fl.BoxH
		}
	default:
		return 0, fmt.Errorf("Scan_Input on field type %d is not ported", fl.Type)
	}
	return n, nil
}

// pushFieldDown is Push_Field_Down: draw the previously pushed field up and
// the new one down, remember it and warp the pointer to (x, y).
func (e *Engine) pushFieldDown(n, x, y int) error {
	if x < 0 || x >= W || y < 0 || y >= H {
		return nil
	}
	f := &e.f
	if n == f.downMouseButton {
		return nil
	}
	tn := f.fields[n].Type
	if d := f.downMouseButton; d != -1 {
		switch td := f.fields[d].Type; {
		case td == FieldMulti && tn != FieldMulti:
			// A pushed multi button stays down under any other field.
		case td == FieldList:
			// Draw_Field draws nothing for a list row, pushed or not.
		default:
			if err := e.drawField(d, false); err != nil {
				return err
			}
		}
	}
	f.downMouseButton = n
	if err := e.drawField(n, true); err != nil {
		return err
	}
	if tn == 4 {
		return fmt.Errorf("Push_Field_Down on field type 4 is not ported")
	}
	e.Sys.WarpMouse(x, y)
	return nil
}

// GetInput is Get_Input: the field the player chose this poll, -1 for cancel,
// 0 for nothing, or an F-key code.
func (e *Engine) GetInput() (int, error) {
	return e.getInput()
}

func (e *Engine) getInput() (int, error) {
	f := &e.f
	if e.Sys.Controlled() {
		e.publishFields()
		if e.ControlState != nil && e.Sys.StateWanted() {
			e.Sys.PublishState(e.ControlState())
		}
		if a := e.Sys.TakeAct(); a != nil {
			res := "no actions here"
			if e.ControlAct != nil {
				res = e.ControlAct(a)
			}
			e.Sys.ActDone(res)
		}
	}
	if e.InputHook != nil {
		e.InputHook()
	}
	if f.inputDelay > 0 {
		f.inputDelay--
		return 0, nil
	}
	if f.inputDelay < 0 {
		f.inputDelay = 0
	}
	if f.count <= 1 {
		return 0, nil
	}
	if err := f.drawErr; err != nil {
		f.drawErr = nil
		return 0, err
	}
	r, err := e.interpretMouseInput()
	if err != nil {
		return 0, err
	}
	e.V.SetPageOff()
	if r != 0 {
		e.PlaySound(e.fieldSound(r))
	}
	return r, nil
}

// fieldSound is fields[abs(r)].button_sound; F-key results index far past
// the array in the original (open question), and are silent here.
func (e *Engine) fieldSound(r int) int {
	if r < 0 {
		r = -r
	}
	if r >= maxFields {
		return 0
	}
	return e.f.fields[r].Sound
}

// interpretMouseInput is Interpret_Mouse_Input for the ported field types.
// Paths that report clear reset the multi-hot-key progress on the way out.
func (e *Engine) interpretMouseInput() (int, error) {
	r, clear, err := e.interpretMouseInputPaths()
	if err == nil && clear && e.f.multiActive {
		e.clearMultiHotKeyOffsets()
	}
	return r, err
}

func (e *Engine) interpretMouseInputPaths() (int, bool, error) {
	f := &e.f
	f.downMouseButton = -1
	f.autoInputVariable = 0
	mx, my, _ := e.Sys.Mouse()
	e.checkMouseShape()

	// A. A key is waiting.
	if k, ok := e.Sys.PopKey(); ok {
		if e.KeyHook != nil && e.KeyHook(k) {
			return 0, true, nil // a port-only line took it (a paste too)
		}
		if k.Paste != "" {
			e.paste(k.Paste)
			return 0, true, nil
		}
		if k.Text != "" {
			return 0, true, nil // typed text is for the port's own lines (the lobby's)
		}
		return e.interpretKey(biosWord(k), mx, my)
	}

	// B. A button is held.
	if b := e.mouseButton(); b != 0 {
		return e.heldButton(b)
	}

	// C. A press was buffered between polls.
	if e.Sys.MouseBuffer() {
		return e.bufferedPress()
	}

	// D. Nothing.
	f.pressedButton = false
	return 0, false, nil
}

func (e *Engine) interpretKey(k, mx, my int) (int, bool, error) {
	f := &e.f
	field := 0
	c := e.interpretKeyboardInput(k, &field)
	if c == 0 || field < 0 || c == -1 {
		return field, true, nil
	}
	if c == KeyEsc && !f.MouseCancelDisabled {
		return -1, true, nil
	}
	if field != 0 && int(f.fields[field].Hotkey) == c {
		fl := &f.fields[field]
		if fl.Type == 6 {
			return 0, false, nil
		}
		if err := e.pushFieldDown(field, mx, my); err != nil {
			return 0, false, err
		}
		e.toggleField(field)
		if fl.Type != FieldHidden && fl.Type != FieldMultiHot {
			if err := e.quickCallAuto(); err != nil {
				return 0, false, err
			}
		}
		f.downMouseButton = -1
		return field, false, nil
	}
	if c == KeyEnter || c == KeyPadEnter {
		if f.inputActive != 0 {
			n := f.activeInput
			e.copyContinuousString()
			if !f.MouseAutoExit {
				if err := e.quickCallAuto(); err != nil {
					return 0, false, err
				}
			}
			f.downMouseButton = -1
			f.inputActive, f.activeInput = 0, -1
			return n, false, nil
		}
		n, err := e.scanField()
		if err != nil || n == 0 {
			return 0, false, err
		}
		switch {
		case f.fields[n].Type == FieldString && f.inputActive == 0:
			e.startInput(n)
		case f.fields[n].Type == FieldString && f.activeInput == n:
			e.endInput(n)
			f.cursorCount, f.activeInput = 0, -1
		case f.fields[n].Type == FieldString:
			// The new field keeps the old field's text in continuous_string.
			e.copyContinuousString()
			f.cursorCount, f.cursorOn, f.activeInput = 0, 0, n
		case f.inputActive != 0:
			e.endInput(f.activeInput)
			f.cursorCount, f.inputActive, f.activeInput = 0, 0, -1
		}
		if t := f.fields[n].Type; t != 6 && t != 0x0b {
			if err := e.pushFieldDown(n, mx, my); err != nil {
				return 0, false, err
			}
		}
		e.toggleField(n)
		if !f.MouseAutoExit {
			if err := e.quickCallAuto(); err != nil {
				return 0, false, err
			}
		}
		f.downMouseButton = -1
		return n, false, nil
	}
	if c == keyPlus || c == keyMinus {
		n, err := e.scanField()
		if err != nil {
			return 0, false, err
		}
		if n != 0 && f.fields[n].Type == FieldScroll {
			if c == keyPlus {
				incrementScrollBar(&f.fields[n])
			} else {
				decrementScrollBar(&f.fields[n])
			}
			f.downMouseButton = -1
			return n, false, nil
		}
	}
	if f.inputActive > 0 {
		if c == KeyEsc {
			// Reachable only with cancel disabled: the edit is dropped.
			if f.inputActive == 1 {
				e.setCont(f.fields[f.activeInput].Str)
			}
			n := f.activeInput
			f.activeInput, f.cursorCount, f.cursorOn, f.inputActive = -1, 0, 0, 0
			return -n, false, nil
		}
		e.typeKey(k)
	}
	f.downMouseButton = -1
	return 0, false, nil
}

// keyCtrlBackspace clears a text input field.
const keyCtrlBackspace = 0x0e7f

// typeKey is the text input part of Interpret_Mouse_Input for key word k.
func (e *Engine) typeKey(k int) {
	f := &e.f
	fl := &f.fields[max(f.activeInput, 0)]
	switch k {
	case keyCtrlBackspace:
		f.cont[0], f.inputFirst = 0, false
		return
	case keyBackspace:
		if f.inputFirst {
			f.cont[0], f.inputFirst = 0, false
			return
		}
		n := 0
		for f.cont[n] != 0 && f.cont[n] != '_' && n < fl.MaxChars-1 {
			n++
		}
		if n > 0 {
			f.cont[n-1], f.cursorCount = 0, 0
		}
		return
	}
	ch := k % 0x100
	f.inputFirst = false
	if ch < 0x20 || ch > 0x7e || ch == '_' {
		return
	}
	if f.activeInput >= 0 && f.activeInput < f.count {
		e.setInputFieldStyle(fl)
	}
	n := 0
	for f.cont[n] != '_' && f.cont[n] != 0 && n < fl.MaxChars-1 {
		n++
	}
	if n < fl.MaxChars-1 && (fl.Masked || fl.Long || fl.X1+e.text.Width(f.cont[:contLen(f.cont[:])]) <= fl.X2) {
		f.cont[n], f.cont[n+1] = byte(ch), 0
		// Letters already queued go in at once, up to ten.
		for i := 0; i < 10; i++ {
			q, ok := e.Sys.PeekKey()
			if !ok {
				continue
			}
			qc := biosWord(q) % 0x100
			if qc > 0x40 && qc < 0x7b && i+n+1 < fl.MaxChars-1 {
				e.Sys.PopKey()
				f.cont[i+n+1], f.cont[i+n+2] = byte(qc), 0
			}
		}
	} else {
		f.cont[n] = 0
	}
	f.cursorOn, f.cursorCount = 0, 0
}

// paste types text into the active input field (port-only: Ctrl+V), as
// far as the field takes it; nothing without an active field.
func (e *Engine) paste(text string) {
	f := &e.f
	if f.inputActive == 0 || f.activeInput < 0 || f.activeInput >= f.count {
		return
	}
	fl := &f.fields[f.activeInput]
	e.setInputFieldStyle(fl)
	n := 0
	for f.cont[n] != '_' && f.cont[n] != 0 && n < fl.MaxChars-1 {
		n++
	}
	for _, c := range []byte(strings.TrimSpace(text)) {
		if c < 0x20 || c > 0x7e || c == '_' {
			continue
		}
		if n >= fl.MaxChars-1 || !fl.Masked && !fl.Long && fl.X1+e.text.Width(append(f.cont[:n:n], c)) > fl.X2 {
			break
		}
		f.cont[n], f.cont[n+1] = c, 0
		n++
	}
	f.inputFirst = false
	f.cursorOn, f.cursorCount = 0, 0
}

// setInputFieldStyle sets a text input field's font style by its shadow.
func (e *Engine) setInputFieldStyle(fl *Field) {
	t := e.text
	switch fl.ShadowMode {
	case -1:
		t.SetStyleShadow(fl.TextStyle, e.f.listNormal, font.ShadowDown)
	case 1:
		t.SetStyleShadow(fl.TextStyle, e.f.listNormal, font.ShadowUp)
	default:
		t.SetStyle(fl.TextStyle, e.f.listNormal)
	}
}

func contLen(b []byte) int {
	for i, c := range b {
		if c == 0 {
			return i
		}
	}
	return len(b)
}

func (e *Engine) setCont(s []byte) {
	n := copy(e.f.cont[:len(e.f.cont)-1], s[:contLen(s)])
	e.f.cont[n] = 0
}

// startInput makes field n the text input, its string in continuous_string.
func (e *Engine) startInput(n int) {
	f := &e.f
	e.setCont(f.fields[n].Str)
	f.cursorCount, f.cursorOn, f.inputActive, f.activeInput = 0, 0, 1, n
}

// ActivateInputField starts editing field n the way Do_Save_Game_Popup_
// does by hand, with active_input_field_first set.
func (e *Engine) ActivateInputField(n int) {
	e.startInput(n)
	e.f.inputFirst = true
}

// MaskInputField shows input field n as '*' per character (port-only, for
// passwords).
func (e *Engine) MaskInputField(n int) { e.f.fields[n].Masked = true }

// LongInputField lets input field n take text past its width, up to its
// limit, showing the end of it (port-only: addresses and game ids).
func (e *Engine) LongInputField(n int) { e.f.fields[n].Long = true }

// FieldMasked reports MaskInputField on field n.
func (e *Engine) FieldMasked(n int) bool { return e.f.fields[n].Masked }

// InputActive is active_input_field_number, -1 for none.
func (e *Engine) InputActive() int { return e.f.activeInput }

// ContinuousString is the text being edited.
func (e *Engine) ContinuousString() []byte { return e.f.cont[:contLen(e.f.cont[:])] }

// endInput cuts a trailing cursor off continuous_string and writes it to
// field n's string, as Interpret_Mouse_Input does on leaving a field.
func (e *Engine) endInput(n int) {
	f := &e.f
	fl := &f.fields[n]
	k := 0
	for f.cont[k] != 0 && k < fl.MaxChars {
		k++
	}
	if k != 0 && f.cont[k-1] == '_' {
		k--
	}
	f.cont[k] = 0
	copyCString(fl.Str, f.cont[:k+1])
}

// copyContinuousString is Copy_Continuous_String (0x1203e4) into the active
// field.
func (e *Engine) copyContinuousString() {
	if e.f.activeInput >= 0 {
		e.endInput(e.f.activeInput)
	}
}

// copyCString is strcpy into dst; the original's has no bound.
func copyCString(dst, src []byte) {
	n := copy(dst, src[:contLen(src)])
	if n < len(dst) {
		dst[n] = 0
	}
}

// AddContinuousStringInputField is Add_Continuous_String_Input_Field
// (0x115bea). The colours are the current font's, in globals all such
// fields share.
func (e *Engine) AddContinuousStringInputField(x, y, w, h int, str []byte, maxChars, fill int, marker *gfx.Picture, sound int, shadow int16) (int, error) {
	t := e.text
	e.f.listNormal, e.f.listHighlight = t.Normal, t.Normal
	return e.addField(Field{X1: x, Y1: y, X2: x + w, Y2: y + h, Type: FieldString, Sound: sound,
		TextStyle: t.Style, Str: str, MaxChars: min(maxChars, 0x100), FillColor: fill, Marker: marker, ShadowMode: shadow})
}

// Key words Interpret_Mouse_Input tests for scroll fields: '+' and '-' on
// the main keyboard.
const (
	keyPlus  = 0x0d2b
	keyMinus = 0x0c2d
)

// InvokeAutoFunction is Invoke_Auto_Function.
func (e *Engine) InvokeAutoFunction() error { return e.invokeAuto() }

// invokeAuto is Invoke_Auto_Function (0x1192d1): the auto function alone,
// on whatever page is current.
func (e *Engine) invokeAuto() error {
	f := &e.f
	if f.autoActive && f.autoFn != nil {
		return f.autoFn()
	}
	return nil
}

// leaveHeldField is the held-button loop's exit when the pointer left every
// field while one was pushed (only reachable without dynamic refresh).
func (e *Engine) leaveHeldField(d, x, y int) error {
	f := &e.f
	t := f.fields[d].Type
	if t == FieldScroll {
		if err := e.invokeAuto(); err != nil {
			return err
		}
	}
	if t != 3 && t != 10 && t != 0x0b {
		if err := e.drawField(d, false); err != nil {
			return err
		}
		e.Sys.WarpMouse(x, y)
	}
	f.downMouseButton = -1
	return nil
}

// AddScrollField is Add_Scroll_Field (0x115478): a bar of w x h pixels at
// (x, y) whose position, scaled to [minValue, maxValue] and clamped to
// [minValid, maxValid], is written to *v while it is pressed. It is
// vertical unless h < w.
func (e *Engine) AddScrollField(x, y, minValue, maxValue, minValid, maxValid, w, h int, v *int16, hotkey string, sound int) (int, error) {
	return e.addField(Field{X1: x, Y1: y, X2: x + w, Y2: y + h, Type: FieldScroll, Hotkey: upperHotkey(hotkey), Sound: sound,
		MinValid: int16(minValid), MaxValid: int16(maxValid), Variable: v,
		MinValue: int16(minValue), MaxValue: int16(maxValue), Vertical: h >= w})
}

// findBarPosition is Find_Bar_Position (0x1156e2), Draw_Field of a pressed
// scroll field. The vertical value is measured up from the bottom and,
// unlike the horizontal one, is not offset by minValue.
func (e *Engine) findBarPosition(fl *Field) {
	off := e.pointerOffset()
	mx, my, _ := e.Sys.Mouse()
	rng := int(fl.MaxValue) - int(fl.MinValue)
	var v int16
	if !fl.Vertical {
		w := int(int16(fl.X2 - fl.X1))
		p := int16(mx + off)
		switch {
		case int(p) >= fl.X2:
			v = fl.MaxValue
		case int(p) <= fl.X1:
			v = fl.MinValue
		default:
			v = fl.MinValue + int16((int(p)-fl.X1)*rng/w)
		}
	} else {
		h := int(int16(fl.Y2 - fl.Y1))
		p := int16(my + off)
		v = int16((fl.Y2 - int(p)) * rng / h)
		switch {
		case int(p) >= fl.Y2:
			v = fl.MinValue
		case int(p) <= fl.Y1:
			v = fl.MaxValue
		}
	}
	v = max(v, fl.MinValid)
	*fl.Variable = min(v, fl.MaxValid)
}

// incrementScrollBar is Increment_Scroll_Bar (0x115994): one step of 5 %
// of the value range up.
func incrementScrollBar(fl *Field) {
	rng := int16(fl.MaxValue - fl.MinValue)
	pct := int16((int(*fl.Variable)-int(fl.MinValue))*100/int(rng)) + 5
	v := fl.MaxValid
	if pct < 0x65 {
		v = fl.MinValue + int16(int(pct)*int(rng)/100)
	}
	*fl.Variable = min(v, fl.MaxValid)
}

// decrementScrollBar is Decrement_Scroll_Bar (0x115ac0).
func decrementScrollBar(fl *Field) {
	rng := int16(fl.MaxValue - fl.MinValue)
	pct := int16((int(*fl.Variable)-int(fl.MinValue))*100/int(rng)) - 5
	v := fl.MinValid
	if pct >= 0 {
		v = fl.MinValue + int16(int(pct)*int(rng)/100)
	}
	*fl.Variable = max(v, fl.MinValid)
}

// interpretKeyboardInput is Interpret_Keyboard_Input for a key word k.
func (e *Engine) interpretKeyboardInput(k int, field *int) int {
	f := &e.f
	if f.selectedCount >= f.count {
		f.selectedCount = 0
	}
	switch {
	case k >= keyF1 && k <= keyF1+9<<8 && k&0xff == 0:
		*field = -1001 - (k-keyF1)>>8
		return k
	case k >= keyAltF1 && k <= keyAltF1+9<<8 && k&0xff == 0:
		*field = -1101 - (k-keyAltF1)>>8
		return k
	case k == 0x01F0:
		*field = -1500
		return k
	}
	if f.multiActive {
		if n := e.checkMultiHotKeyFields(int(altAlphaKey(k))); n > -1 {
			*field = n
			return -1
		}
	}
	c := int(int16(k)) % 256
	if c == 0 {
		c = k
	}
	if c > 0x60 && c < 0x7b {
		c -= 0x20
	}
	for i := 0; i < f.count && f.activeInput == -1; i++ {
		if int(f.fields[i].Hotkey) == c {
			*field = i
			break
		}
	}
	if int(f.fields[*field].Hotkey) != c {
		c = k
	}
	return c
}

func (e *Engine) heldButton(b int) (int, bool, error) {
	f := &e.f
	if b == 2 && f.helpActive {
		if ok, err := e.checkHelpList(); err != nil || !ok {
			e.Sys.MouseBuffer()
			e.Sys.MouseBuffer2()
			return 0, false, err
		}
	}
	if b == 2 {
		if !f.MouseCancelDisabled {
			for e.mouseButton() == 2 {
				if err := e.quickCallAuto(); err != nil {
					return 0, false, err
				}
			}
			e.Sys.MouseBuffer()
			e.Sys.MouseBuffer2()
			return -1, false, nil
		}
	}
	if !f.pressedButton {
		f.pressedButton = true
		f.pressedX, f.pressedY, _ = e.Sys.Mouse()
	}
	field := 0
	var x, y int
	for {
		if e.mouseButton() == 0 {
			break
		}
		x, y, _ = e.Sys.Mouse()
		e.checkMouseShape()
		field = 0
		if f.DynamicFieldRefresh {
			var err error
			if field, err = e.scanField(); err != nil {
				return 0, false, err
			}
			f.downMouseButton = -1
		}
		if field == 0 {
			if d := f.downMouseButton; d != -1 {
				if err := e.leaveHeldField(d, x, y); err != nil {
					return 0, false, err
				}
			}
			break
		}
		if d := f.downMouseButton; field != d && d != -1 && f.fields[d].Type == FieldScroll {
			if err := e.invokeAuto(); err != nil {
				return 0, false, err
			}
		}
		if f.fields[field].Type != FieldString {
			if err := e.pushFieldDown(field, x, y); err != nil {
				return 0, false, err
			}
		}
		switch {
		case f.fields[field].Type == FieldString:
			if f.inputActive == 0 || f.activeInput != field {
				e.startInput(field)
				f.inputFirst = true
			}
		case f.inputActive != 0:
			e.endInput(f.activeInput)
			f.inputActive, f.activeInput = 0, -1
		}
		f.autoInputVariable = field
		if f.MouseAutoExit {
			break
		}
		if e.mouseButton() != 0 {
			if err := e.callAuto(); err != nil {
				return 0, false, err
			}
		}
		e.Sys.MouseBuffer()
		e.Sys.MouseBuffer2()
	}
	// A released scroll field redraws through the auto function once more.
	if f.fields[f.autoInputVariable].Type == FieldScroll {
		if err := e.invokeAuto(); err != nil {
			return 0, false, err
		}
	}
	f.autoInputVariable = 0
	if field != 0 {
		e.Sys.MouseBuffer()
		e.Sys.MouseBuffer2()
		f.LastButton, f.LastX, f.LastY = b, x, y
		if !e.toggleField(field) && f.fields[field].Type == 4 {
			return 0, false, fmt.Errorf("Push_Field_Down on field type 4 is not ported")
		}
	}
	f.downMouseButton = -1
	switch b {
	case 1:
		return field, true, nil
	case 2:
		return -field, true, nil
	}
	return 0, false, nil
}

func (e *Engine) bufferedPress() (int, bool, error) {
	f := &e.f
	p := e.Sys.BufferedPress()
	if p.Buttons == 2 && f.helpActive {
		if ok, err := e.checkHelpList(); err != nil || !ok {
			e.Sys.MouseBuffer2()
			return 0, false, err
		}
	}
	if p.Buttons == 2 {
		if !f.MouseCancelDisabled {
			e.Sys.MouseBuffer2()
			return -1, true, nil
		}
	}
	e.checkMouseShape()
	off := e.pointerOffset()
	field, err := e.fieldAt(p.X, p.Y, off)
	if err != nil {
		return 0, false, err
	}
	if field != 0 && f.fields[field].Type != 0x0b {
		f.autoInputVariable = field
		if err := e.pushFieldDown(field, p.X, p.Y); err != nil {
			return 0, false, err
		}
		if err := e.quickCallAuto(); err != nil {
			return 0, false, err
		}
	}
	if field != 0 {
		switch t := f.fields[field].Type; {
		case t == FieldString && f.inputActive == 0:
			e.startInput(field)
		case t == FieldString && f.activeInput != field:
			e.setCont(f.fields[field].Str)
			f.cursorCount, f.cursorOn, f.activeInput = 0, 0, field
		case t == FieldRadio, t == FieldLocked:
			e.toggleField(field)
		case t == FieldString, t == 4, t == 9:
		case f.inputActive != 0:
			e.endInput(f.activeInput)
			f.inputActive, f.activeInput = 0, -1
		}
	}
	f.downMouseButton = -1
	if field != 0 {
		e.Sys.MouseBuffer2()
		f.LastButton, f.LastX, f.LastY = p.Buttons, p.X, p.Y
	}
	f.autoInputVariable = 0
	if p.Buttons == 2 {
		return -field, false, nil
	}
	return field, false, nil
}

// checkHelpList is Check_Help_List (0x119732): on the first help entry under
// the pointer, run the game's Draw_Help_Entry_ and return false.
func (e *Engine) checkHelpList() (bool, error) {
	mx, my, _ := e.Sys.Mouse()
	for _, h := range e.f.help {
		if h.ID != -1 && h.X1 <= mx && mx <= h.X2 && h.Y1 <= my && my <= h.Y2 {
			if e.f.helpFn != nil {
				return false, e.f.helpFn(h.ID)
			}
			return false, nil
		}
	}
	return true, nil
}

// addButtonInfo is Add_Button_Info plus the parts Add_Button_Field and
// Add_Irregular_Button_Field share: label, picture, sound, font, hot key.
func (e *Engine) addButtonInfo(fl *Field, label string, pic *gfx.Picture, hotkey string, sound int) {
	fl.Label = label
	fl.Pic = pic
	fl.Sound = sound
	if e.text != nil {
		fl.TextStyle, fl.TextColors = e.text.Style, e.text.Normal
	}
	// Get_Full_Store_Flag tests bit 2 of the header byte at 0x0b.
	fl.FullStore = pic != nil && pic.Mask&4 != 0
	fl.Hotkey = upperHotkey(hotkey)
}

// AddButtonField is Add_Button_Field: a button the size of pic at (x, y),
// drawn by Draw_Visible_Fields in frame 0 and pressed in frame 1.
func (e *Engine) AddButtonField(x, y int, label string, pic *gfx.Picture, hotkey string, sound int) (int, error) {
	f := &e.f
	if f.count >= maxFields {
		return 0, fmt.Errorf("Too many fields")
	}
	if pic == nil {
		return 0, fmt.Errorf("Add_Button_Field without a picture")
	}
	fl := &f.fields[f.count]
	fl.X1, fl.Y1 = x, y
	fl.X2, fl.Y2 = x+pic.Width-1, y+pic.Height-1
	e.addButtonInfo(fl, label, pic, hotkey, sound)
	fl.Type = FieldButton
	f.count++
	return f.count - 1, nil
}

// AddIrregularButtonField is Add_Irregular_Button_Field: an explicit hit
// rectangle with the picture drawn at (dx, dy), and only while pressed.
func (e *Engine) AddIrregularButtonField(x1, y1, x2, y2 int, label string, pic *gfx.Picture, dx, dy int, hotkey string, sound int) (int, error) {
	f := &e.f
	// The original tests fields_count <= max_fields_count: one field too many
	// fits, written past sfields (the spare slot here).
	if f.count > maxFields {
		return 0, fmt.Errorf("Maximum number of button fields exceeded (%d)", maxFields)
	}
	fl := &f.fields[f.count]
	fl.X1, fl.Y1, fl.X2, fl.Y2 = x1, y1, x2, y2
	e.addButtonInfo(fl, label, pic, hotkey, sound)
	fl.DrawX, fl.DrawY = dx, dy
	fl.Type = FieldIrregular
	f.count++
	return f.count - 1, nil
}

// AddMultiHotKeyField is Add_Multi_Hot_Key_Field: a field selected by typing
// s with Alt held (see altAlphaKey). The slot is zeroed and unreachable.
func (e *Engine) AddMultiHotKeyField(s string) (int, error) {
	f := &e.f
	if f.count >= maxFields {
		return 0, fmt.Errorf("You exceeded the max number of fields (150)")
	}
	f.fields[f.count] = Field{X1: -1, Y1: -1, X2: -1, Y2: -1, Type: FieldMultiHot, Multi: s, MultiLen: len(s)}
	f.multiActive = true
	f.count++
	return f.count - 1, nil
}

// altKeys is the scan-word table of Get_Alt_Alpha_Key_ (0x11abad).
var altKeys = map[uint16]byte{
	0x1e00: 'A', 0x3000: 'B', 0x2e00: 'C', 0x2000: 'D', 0x1200: 'E', 0x2100: 'F',
	0x2200: 'G', 0x2300: 'H', 0x1700: 'I', 0x2400: 'J', 0x2500: 'K', 0x2600: 'L',
	0x3200: 'M', 0x3100: 'N', 0x1800: 'O', 0x1900: 'P', 0x1000: 'Q', 0x1300: 'R',
	0x1f00: 'S', 0x1400: 'T', 0x1600: 'U', 0x2f00: 'V', 0x1100: 'W', 0x2d00: 'X',
	0x1500: 'Y', 0x2c00: 'Z',
	// Alt-1..Alt-8 give '0'..'7'.
	0x7800: '0', 0x7900: '1', 0x7a00: '2', 0x7b00: '3', 0x7c00: '4', 0x7d00: '5',
	0x7e00: '6', 0x7f00: '7',
}

// altAlphaKey is Get_Alt_Alpha_Key_: the letter or digit of an Alt key word,
// 0xff for anything else.
func altAlphaKey(k int) byte {
	if c, ok := altKeys[uint16(k)]; ok {
		return c
	}
	return 0xff
}

// checkMultiHotKeyFields is Check_Multi_Hot_Key_Fields_: every type-8 field
// whose next character is c advances, until one completes; that one is
// returned, else -1. A mismatch does not reset progress.
func (e *Engine) checkMultiHotKeyFields(c int) int {
	f := &e.f
	hit := -1
	for i := 0; i < f.count && hit == -1; i++ {
		fl := &f.fields[i]
		if fl.Type != FieldMultiHot {
			continue
		}
		ch := 0 // the string's terminator past its end
		if fl.Progress >= 0 && fl.Progress < len(fl.Multi) {
			ch = int(fl.Multi[fl.Progress])
		}
		if ch != c {
			continue
		}
		fl.Progress++
		if fl.Progress == fl.MultiLen {
			hit = i
		}
	}
	return hit
}

// clearMultiHotKeyOffsets is Clear_Multi_Hot_Key_Offsets_.
func (e *Engine) clearMultiHotKeyOffsets() {
	f := &e.f
	for i := 0; i < f.count; i++ {
		if f.fields[i].Type == FieldMultiHot {
			f.fields[i].Progress = 0
		}
	}
}

// AddGridField is Add_Grid_Field: nx by ny cells of bw by bh from (x, y).
// Pressing it writes the cell under the pointer to *gx, *gy.
func (e *Engine) AddGridField(x, y, bw, bh, nx, ny int, gx, gy *int, sound int) (int, error) {
	f := &e.f
	if f.count >= maxFields {
		return 0, fmt.Errorf("Too many fields")
	}
	fl := &f.fields[f.count]
	fl.X1, fl.Y1 = x, y
	fl.X2, fl.Y2 = bw*nx+x-1, bh*ny+y-1
	fl.Sound = sound
	fl.Type = FieldGrid
	fl.BoxW, fl.BoxH = bw, bh
	fl.Hotkey = 0
	fl.GridX, fl.GridY = gx, gy
	f.count++
	return f.count - 1, nil
}

// AutoInput is Auto_Input: the field held down while the auto function runs.
func (e *Engine) AutoInput() int { return e.f.autoInputVariable }

// SetFieldOutlineColor is Set_Field_Outline_Color (debug outlines only).
func (e *Engine) SetFieldOutlineColor(c int) { e.f.outlineColor = c }

// SetButtonDownOffsets is Set_Button_Down_Offsets: the shift of a pressed
// button's label (1, -1 by default).
func (e *Engine) SetButtonDownOffsets(x, y int) { e.f.downX, e.f.downY = x, y }

// EnableMousePressedFieldUpdate is Enable_Mouse_Pressed_Field_Update.
func (e *Engine) EnableMousePressedFieldUpdate() {
	e.f.DynamicFieldRefresh = true
	e.Sys.MouseBuffer()
	e.Sys.MouseBuffer2()
}

// DisableMousePressedFieldUpdate is Disable_Mouse_Pressed_Field_Update.
func (e *Engine) DisableMousePressedFieldUpdate() { e.f.DynamicFieldRefresh = false }

// setAnimationFrame is Set_Animation_Frame.
func setAnimationFrame(p *gfx.Picture, n int) {
	if n >= p.Frames {
		n = p.Frames - 1
	}
	p.CurrentFrame = n
}

// drawField is Draw_Field for the ported types; up is up_down_flag == 0.
func (e *Engine) drawField(n int, down bool) error {
	f := &e.f
	if !f.drawFieldsEnabled {
		return nil
	}
	fl := &f.fields[n]
	switch t := fl.Type; {
	case t == FieldButton || t == FieldIrregular:
		return e.drawButton(fl, down)
	case t == FieldRadio:
		return e.drawButton(fl, down && *fl.Variable != 1)
	case t == FieldLocked:
		return e.drawButton(fl, *fl.Variable != 0 || down)
	case t == FieldMulti:
		*fl.Variable = NoField
		if down {
			*fl.Variable = fl.Value
		}
		return e.drawButton(fl, *fl.Variable == fl.Value)
	case t == FieldGrid:
		return e.drawGrid(fl, down)
	case t == FieldScroll:
		if down {
			e.findBarPosition(fl)
		}
		return nil
	case t == 2 || t == 3 || (down && t == 4):
		return fmt.Errorf("Draw_Field on field type %d is not ported", t)
	}
	return nil // 5, 7..11, 0xe and up draw nothing
}

// drawButton is Draw_Field for types 0 and 0xd: frame 0 up, frame 1 down.
func (e *Engine) drawButton(fl *Field, down bool) error {
	if fl.Pic == nil {
		return fmt.Errorf("button field without a picture")
	}
	x, y := fl.X1, fl.Y1
	if fl.Type == FieldIrregular {
		x, y = fl.DrawX, fl.DrawY
	}
	if !down || !fl.FullStore {
		fl.Pic.CurrentFrame = 0 // Reset_Animation_Frame
		if err := e.Draw(x, y, fl.Pic); err != nil {
			return err
		}
	} else {
		setAnimationFrame(fl.Pic, 1)
	}
	if down {
		if err := e.Draw(x, y, fl.Pic); err != nil {
			return err
		}
	}
	// The original prints the label only when it is empty (the test is the
	// reverse of Draw_Visible_Fields'), so only font state and squares change.
	if fl.Label != "" {
		return nil
	}
	dx, dy := 0, 0
	if down {
		dx, dy = e.f.downX, e.f.downY
	}
	return e.printFieldLabel(fl, dx, dy)
}

// printFieldLabel is the centred label print Draw_Field and
// Draw_Visible_Fields share, shifted by (dx, dy).
func (e *Engine) printFieldLabel(fl *Field, dx, dy int) error {
	t := e.text
	if t == nil {
		return nil
	}
	t.SetStyle(fl.TextStyle, fl.TextColors)
	h := (t.Font.Styles[t.Style].Height - 1) / 2
	return t.PrintCentered(dx+(fl.X2-fl.X1)/2+fl.X1, fl.Y1+(fl.Y2-fl.Y1)/2+dy-h, fl.Label)
}

// drawGrid is Draw_Field for a grid: when pressed, store the cell under the
// pointer (with the pointer offset), clamped to the grid.
func (e *Engine) drawGrid(fl *Field, down bool) error {
	off := e.pointerOffset()
	if !down {
		return nil
	}
	mx, my, _ := e.Sys.Mouse()
	gx := (off + mx - fl.X1) / fl.BoxW
	gy := (my + off - fl.Y1) / fl.BoxH
	if m := (fl.X2 - fl.X1) / fl.BoxW; m < gx {
		gx = m
	}
	if m := (fl.Y2 - fl.Y1) / fl.BoxH; m < gy {
		gy = m
	}
	*fl.GridX, *fl.GridY = max(gx, 0), max(gy, 0)
	return nil
}

// drawVisibleFields is Draw_Visible_Fields, run on every page flip: the
// pushed field down, type-0 buttons up, the rest invisible. Errors surface
// from the next GetInput, since the flip hook cannot return one.
func (e *Engine) drawVisibleFields() {
	if err := e.drawVisibleFieldsErr(); err != nil && e.f.drawErr == nil {
		e.f.drawErr = err
	}
}

func (e *Engine) drawVisibleFieldsErr() error { return e.drawVisibleFieldsOn(false) }

// drawVisibleFieldsOn is Draw_Visible_Fields_ (0x11e718) or, with back,
// its copy Draw_Visible_Fields_Back_ (0x1174cb) drawing on the back page.
func (e *Engine) drawVisibleFieldsOn(back bool) error {
	v := e.V
	wx1, wy1, wx2, wy2 := v.WinX1, v.WinY1, v.WinX2, v.WinY2
	defer func() { v.WinX1, v.WinY1, v.WinX2, v.WinY2 = wx1, wy1, wx2, wy2 }()
	v.ResetWindow()
	if back {
		v.SetPageBack()
	} else {
		v.SetPageOff()
	}
	f := &e.f
	if !f.drawFieldsEnabled {
		return nil
	}
	for i := 1; i < f.count; i++ {
		fl := &f.fields[i]
		t := fl.Type
		if i == f.downMouseButton && t != 4 && t != FieldGrid && t != 10 {
			if err := e.drawField(i, true); err != nil {
				return err
			}
			continue
		}
		switch {
		case t == FieldButton:
			if fl.Pic == nil {
				return fmt.Errorf("button field without a picture")
			}
			fl.Pic.CurrentFrame = 0
			if err := e.Draw(fl.X1, fl.Y1, fl.Pic); err != nil {
				return err
			}
			if fl.Label != "" {
				if err := e.printFieldLabel(fl, 0, 0); err != nil {
					return err
				}
			}
		case t == FieldRadio, t == FieldLocked:
			if err := e.drawVisibleToggle(fl, *fl.Variable != 0); err != nil {
				return err
			}
		case t == FieldMulti:
			if err := e.drawVisibleToggle(fl, *fl.Variable == fl.Value); err != nil {
				return err
			}
		case t == FieldList:
			if err := e.drawListField(fl); err != nil {
				return err
			}
		case t == FieldString:
			if err := e.drawStringField(i, fl); err != nil {
				return err
			}
		case t == FieldScroll:
			// Draw_Visible_Fields only clamps the variable to the valid range.
			v := max(*fl.Variable, fl.MinValid)
			*fl.Variable = min(v, fl.MaxValid)
		case t == 5 || t == FieldHidden || t == FieldMultiHot || t == 9 || t >= FieldGrid || t < 0:
		default:
			return fmt.Errorf("Draw_Visible_Fields on field type %d is not ported", t)
		}
		// field_box_mode outlines need release_flag == 0: not ported.
	}
	return nil
}

// drawStringField is Draw_Visible_Fields for a text input field: the active
// one prints continuous_string in the highlight colours with a blinking '_'.
func (e *Engine) drawStringField(n int, fl *Field) error {
	f, t := &e.f, e.text
	active := f.activeInput == n
	e.setInputFieldStyle(fl)
	if active {
		t.Highlight = f.listHighlight
		t.Current = t.Highlight
	}
	if fl.FillColor != 0 {
		e.V.Fill(fl.X1-1, fl.Y1, fl.X2, fl.Y2, byte(fl.FillColor))
	}
	s := fl.Str[:contLen(fl.Str)]
	if active {
		old := f.cursorOn
		f.cursorOn = f.cursorCount / 2
		f.cursorCount++
		if f.cursorCount > 3 {
			f.cursorCount = 0
		}
		if old != f.cursorOn {
			k := 0
			if f.cursorOn == 0 {
				for f.cont[k] != '_' && k < fl.MaxChars-1 {
					k++
				}
				f.cont[k] = 0
			} else {
				for f.cont[k] != 0 && k < fl.MaxChars-2 {
					k++
				}
				if k < fl.MaxChars-1 {
					l := contLen(f.cont[:])
					f.cont[l], f.cont[l+1] = '_', 0
				}
			}
		}
		if fl.Marker != nil {
			if err := e.Draw(fl.X1, fl.Y1, fl.Marker); err != nil {
				return err
			}
		}
		s = f.cont[:contLen(f.cont[:])]
	}
	y := fl.Y1
	if h := t.Height(); h < fl.Y2-fl.Y1 {
		y = fl.Y1 + (fl.Y2-fl.Y1)/2 - h/2
	}
	if fl.Masked || fl.Long {
		// The blinking cursor is the last '_' while it shows; the line is
		// laid out as if it always did, or the window would shift by a
		// character with every blink.
		cursor := active && f.cursorOn != 0 && len(s) > 0 && s[len(s)-1] == '_'
		text := s
		if cursor {
			text = s[:len(s)-1]
		}
		if fl.Masked {
			text = []byte(strings.Repeat("*", len(text)))
		}
		// A long line shows its last characters, as far as the field goes.
		for len(text) > 0 && fl.X1+10+t.Width(append(text[:len(text):len(text)], '_')) > fl.X2 {
			text = text[1:]
		}
		s = text
		if cursor {
			s = append(text[:len(text):len(text)], '_')
		}
	}
	err := t.Print(fl.X1+10, y, string(s))
	if active {
		t.Current = t.Normal
	}
	return err
}

// drawVisibleToggle is Draw_Visible_Fields for radio, locked and multi
// buttons: down (label shifted by the down offsets) when their variable says so.
func (e *Engine) drawVisibleToggle(fl *Field, down bool) error {
	if fl.Pic == nil {
		return fmt.Errorf("button field type %d without a picture", fl.Type)
	}
	dx, dy := 0, 0
	if !down {
		fl.Pic.CurrentFrame = 0
	} else {
		if !fl.FullStore {
			fl.Pic.CurrentFrame = 0
			if err := e.Draw(fl.X1, fl.Y1, fl.Pic); err != nil {
				return err
			}
		} else {
			setAnimationFrame(fl.Pic, 1)
		}
		dx, dy = e.f.downX, e.f.downY
	}
	if err := e.Draw(fl.X1, fl.Y1, fl.Pic); err != nil {
		return err
	}
	if fl.Label != "" {
		return e.printFieldLabel(fl, dx, dy)
	}
	return nil
}

// DisableDrawField is Disable_Draw_Field_ (0x117324): buttons of types 0..3
// get a negative type, which nothing draws.
func (e *Engine) DisableDrawField(n int) {
	fl := &e.f.fields[n]
	switch fl.Type {
	case 0:
		fl.Type = -0x3e9
	case 1, 2, 3:
		fl.Type = -fl.Type
	}
}

// EnableDrawField is Enable_Draw_Field_ (0x1173dc), the reverse.
func (e *Engine) EnableDrawField(n int) {
	fl := &e.f.fields[n]
	switch fl.Type {
	case -0x3e9:
		fl.Type = 0
	case -1, -2, -3:
		fl.Type = -fl.Type
	}
}

// AddRadioButtonField is Add_Radio_Button_Field_ (0x11523b): a button
// (Add_Button_Info_) of type 1 tied to *v.
func (e *Engine) AddRadioButtonField(x, y int, label string, pic *gfx.Picture, v *int16, hotkey string, sound int) (int, error) {
	f := &e.f
	if f.count >= maxFields {
		return 0, fmt.Errorf("Too many fields")
	}
	if pic == nil {
		return 0, fmt.Errorf("Add_Radio_Button_Field without a picture")
	}
	fl := &f.fields[f.count]
	fl.X1, fl.Y1 = x, y
	fl.X2, fl.Y2 = x+pic.Width-1, y+pic.Height-1
	e.addButtonInfo(fl, label, pic, hotkey, sound)
	fl.Type = FieldRadio
	fl.Variable = v
	f.count++
	return f.count - 1, nil
}

// AddLockedButtonField is Add_Locked_Button_Field_ (0x1152df): a button of
// type 2 that a press sets *v to 1 for good.
func (e *Engine) AddLockedButtonField(x, y int, label string, pic *gfx.Picture, v *int16, hotkey string, sound int) (int, error) {
	n, err := e.AddRadioButtonField(x, y, label, pic, v, hotkey, sound)
	if err == nil {
		e.f.fields[n].Type = FieldLocked
	}
	return n, err
}

// AddMultiButtonField is Add_Multi_Button_Field_ (0x115383): a button of
// type 3, one of the group sharing *v; pushing it stores value there.
func (e *Engine) AddMultiButtonField(x, y int, label string, pic *gfx.Picture, v *int16, value int16, hotkey string, sound int) (int, error) {
	n, err := e.AddRadioButtonField(x, y, label, pic, v, hotkey, sound)
	if err == nil {
		e.f.fields[n].Type = FieldMulti
		e.f.fields[n].Value = value
	}
	return n, err
}

// toggleField is what Interpret_Mouse_Input does to a chosen radio (flip)
// or locked (set) button's variable; false for other types.
func (e *Engine) toggleField(n int) bool {
	fl := &e.f.fields[n]
	switch fl.Type {
	case FieldRadio:
		*fl.Variable = int16(b2i(*fl.Variable == 0))
	case FieldLocked:
		*fl.Variable = 1
	default:
		return false
	}
	return true
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// AddClippedButtonField is Add_Clipped_Button_Field (0x11c3c5): a button
// field, or NoField without adding one when clipping is on and the picture
// sticks out of the window.
func (e *Engine) AddClippedButtonField(x, y int, label string, pic *gfx.Picture, hotkey string, sound int) (int, error) {
	v := e.V
	if v.Clipping && (x < v.WinX1 || x+pic.Width > v.WinX2 || y < v.WinY1 || y+pic.Height > v.WinY2) {
		return NoField, nil
	}
	return e.AddButtonField(x, y, label, pic, hotkey, sound)
}

// FieldRect returns field n's rectangle, inclusive.
func (e *Engine) FieldRect(n int) (x1, y1, x2, y2 int) {
	fl := &e.f.fields[n]
	return fl.X1, fl.Y1, fl.X2, fl.Y2
}

// FieldListValue returns the value a string list field stands for, and
// whether field n is one.
func (e *Engine) FieldListValue(n int) (int16, bool) {
	fl := &e.f.fields[n]
	return fl.ListValue, fl.Type == FieldList
}

// PressedMouse is Pressed_Mouse_X and Pressed_Mouse_Y: where the button
// held now was first pressed.
func (e *Engine) PressedMouse() (int, int) { return e.f.pressedX, e.f.pressedY }

// Hooks for the game's modal boxes (MOX TEXTBOX.C, MSGBOX): they save and
// restore engine state the original reaches as globals.

// SetHelpFunc installs Draw_Help_Entry_, which Check_Help_List calls.
func (e *Engine) SetHelpFunc(fn func(id int) error) { e.f.helpFn = fn }

// HelpActive and SetHelpActive are help_list_active.
func (e *Engine) HelpActive() bool { return e.f.helpActive }

// SetHelpActive sets help_list_active without touching the list.
func (e *Engine) SetHelpActive(on bool) { e.f.helpActive = on }

// DrawAllFields is draw_fields_flag.
func (e *Engine) DrawAllFields() bool { return e.f.drawFieldsEnabled }

// AutoFunction is the auto function pointer, as Get_Auto_Function_ reads it.
type AutoFunction struct{ fn func() error }

// GetAutoFunction is Get_Auto_Function_ (0x107af): only the pointer.
func (e *Engine) GetAutoFunction() AutoFunction { return AutoFunction{e.f.autoFn} }

// PutAutoFunction is Put_Auto_Function_ (0x107ba).
func (e *Engine) PutAutoFunction(a AutoFunction) { e.f.autoFn = a.fn }

// Run is Execute_Auto_Function_ (0x107c2): call it if set.
func (a AutoFunction) Run() error {
	if a.fn == nil {
		return nil
	}
	return a.fn()
}

// DrawVisibleFields is Draw_Visible_Fields.
func (e *Engine) DrawVisibleFields() error { return e.drawVisibleFieldsErr() }

// DrawVisibleFieldsBack is Draw_Visible_Fields_Back_ (0x1174cb).
func (e *Engine) DrawVisibleFieldsBack() error { return e.drawVisibleFieldsOn(true) }

// QuickCallAutoFunction is Quick_Call_Auto_Function.
func (e *Engine) QuickCallAutoFunction() error { return e.quickCallAuto() }

// MouseList is current_mouse_list, for Save_Mouse_List_.
func (e *Engine) MouseList() []MouseRegion { return e.f.mouseList }

// FieldStats is what Save_Field_Stats_ (0x10fe41) keeps.
type FieldStats struct {
	count, inputDelay, downMouseButton, autoInputVariable int
	fields                                                [maxFields + 1]Field
}

// SaveFieldStats is Save_Field_Stats_: the field array is moved past the
// fields in use (inner field i is outer slot count+i), then Clear_Fields.
func (e *Engine) SaveFieldStats() *FieldStats {
	f := &e.f
	s := &FieldStats{count: f.count, inputDelay: f.inputDelay, downMouseButton: f.downMouseButton,
		autoInputVariable: f.autoInputVariable, fields: f.fields}
	var inner [maxFields + 1]Field
	copy(inner[:], f.fields[f.count:])
	f.fields = inner
	e.ClearFields()
	return s
}

// RestoreFieldStats is Restore_Field_Stats_ (0x10fec1). The inner fields
// stay in the outer slots past count; multi_hotkey_active_field is not restored.
func (e *Engine) RestoreFieldStats(s *FieldStats) {
	f := &e.f
	inner := f.fields
	f.fields = s.fields
	copy(f.fields[s.count:], inner[:])
	f.count, f.inputDelay, f.downMouseButton, f.autoInputVariable = s.count, s.inputDelay, s.downMouseButton, s.autoInputVariable
}

// publishFields gives the host's remote control the fields of this poll
// (port-only, development).
func (e *Engine) publishFields() {
	out := make([]host.ControlField, 0, e.f.count)
	for n := 1; n < e.f.count; n++ {
		fl := &e.f.fields[n]
		cf := host.ControlField{N: n, X1: fl.X1, Y1: fl.Y1, X2: fl.X2, Y2: fl.Y2, Type: fl.Type, Label: fl.Label}
		if fl.Hotkey != 0 {
			cf.Hotkey = string(rune(fl.Hotkey))
		}
		out = append(out, cf)
	}
	e.Sys.PublishFields(out)
}
