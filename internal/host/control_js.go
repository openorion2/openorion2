//go:build js

package host

import "errors"

type controlState struct{}

// ControlField is an input field of the screen (no control in the browser).
type ControlField struct {
	N              int
	X1, Y1, X2, Y2 int
	Type           int
	Hotkey, Label  string
}

func (s *System) Controlled() bool             { return false }
func (s *System) StateWanted() bool            { return false }
func (s *System) PublishState(any)             {}
func (s *System) PublishFields([]ControlField) {}
func (s *System) TakeAct() []byte              { return nil }
func (s *System) ActDone(string)               {}
func (s *System) controlTick()                 {}
func (s *System) controlShown(*Screen)         {}
func (s *System) serveControl(string) error    { return errors.New("control: not in the browser") }
