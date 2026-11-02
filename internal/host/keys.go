package host

import (
	"fmt"
	"strconv"
	"strings"
)

// keyScancodes are the USB HID usages of the named keys.
var keyScancodes = map[string]int{"enter": 40, "esc": 41, "backspace": 42, "tab": 43, "space": 44,
	"minus": 45, "equals": 46, "comma": 54, "period": 55, "kp_minus": 86, "kp_plus": 87,
	"pgup": 75, "pgdn": 78, "home": 74, "end": 77, "right": 79, "left": 80, "down": 81, "up": 82}

// scancode is the HID usage of a letter, digit or named key.
func scancode(name string) (int, bool) {
	if c, ok := keyScancodes[name]; ok {
		return c, true
	}
	if len(name) == 1 {
		switch c := name[0]; {
		case c >= 'a' && c <= 'z':
			return 4 + int(c-'a'), true
		case c >= '1' && c <= '9':
			return 30 + int(c-'1'), true
		case c == '0':
			return 39, true
		case c == ' ':
			return 44, true
		}
	}
	if len(name) > 1 && name[0] == 'f' {
		if n, err := strconv.Atoi(name[1:]); err == nil && n >= 1 && n <= 12 {
			return 57 + n, true
		}
	}
	return 0, false
}

// ParseKeys reads a key spec (development): a key (e, f10, esc, enter) or
// letters typed one after another, after any of alt+, shift+, ctrl+
// (alt+einstein is Alt held over each letter).
func ParseKeys(spec string) ([]Key, error) {
	spec = strings.ToLower(spec)
	var mods Key
	for {
		switch {
		case strings.HasPrefix(spec, "alt+"):
			mods.Alt, spec = true, spec[4:]
			continue
		case strings.HasPrefix(spec, "shift+"):
			mods.Shift, spec = true, spec[6:]
			continue
		case strings.HasPrefix(spec, "ctrl+"):
			mods.Ctrl, spec = true, spec[5:]
			continue
		}
		break
	}
	names := []string{spec}
	if _, ok := scancode(spec); !ok {
		names = strings.Split(spec, "")
	}
	var out []Key
	for _, n := range names {
		c, ok := scancode(n)
		if !ok {
			return nil, fmt.Errorf("unknown key %q", n)
		}
		k := mods
		k.Scancode = c
		if len(n) == 1 {
			k.Code = uint32(n[0])
		}
		out = append(out, k)
	}
	return out, nil
}
