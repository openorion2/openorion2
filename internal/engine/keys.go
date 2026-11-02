package engine

import "github.com/openorion2/openorion2/internal/host"

// BIOS keystroke words the input code tests for.
const (
	KeyEsc        = 0x011B
	KeyEnter      = 0x1C0D
	KeyPadEnter   = 0xE00D
	KeyF11        = 0x8500
	KeyF12        = 0x8600
	keyF1         = 0x3B00
	keyAltF1      = 0x6800
	keyBackspace  = 0x0E08
	keyTab        = 0x0F09
	extendedASCII = 0xE0 // gray cursor-block keys on an enhanced keyboard
)

// biosKey is one physical key: its XT scan code and the ASCII it gives
// plain and with Shift.
type biosKey struct {
	scan           byte
	plain, shifted byte
}

// keysByHID maps USB HID usages (SDL scancodes) to the XT keyboard. Letters
// are listed lowercase; Shift and Ctrl are applied in biosWord.
var keysByHID = map[int]biosKey{
	4: {0x1E, 'a', 'A'}, 5: {0x30, 'b', 'B'}, 6: {0x2E, 'c', 'C'}, 7: {0x20, 'd', 'D'},
	8: {0x12, 'e', 'E'}, 9: {0x21, 'f', 'F'}, 10: {0x22, 'g', 'G'}, 11: {0x23, 'h', 'H'},
	12: {0x17, 'i', 'I'}, 13: {0x24, 'j', 'J'}, 14: {0x25, 'k', 'K'}, 15: {0x26, 'l', 'L'},
	16: {0x32, 'm', 'M'}, 17: {0x31, 'n', 'N'}, 18: {0x18, 'o', 'O'}, 19: {0x19, 'p', 'P'},
	20: {0x10, 'q', 'Q'}, 21: {0x13, 'r', 'R'}, 22: {0x1F, 's', 'S'}, 23: {0x14, 't', 'T'},
	24: {0x16, 'u', 'U'}, 25: {0x2F, 'v', 'V'}, 26: {0x11, 'w', 'W'}, 27: {0x2D, 'x', 'X'},
	28: {0x15, 'y', 'Y'}, 29: {0x2C, 'z', 'Z'},
	30: {0x02, '1', '!'}, 31: {0x03, '2', '@'}, 32: {0x04, '3', '#'}, 33: {0x05, '4', '$'},
	34: {0x06, '5', '%'}, 35: {0x07, '6', '^'}, 36: {0x08, '7', '&'}, 37: {0x09, '8', '*'},
	38: {0x0A, '9', '('}, 39: {0x0B, '0', ')'},
	40: {0x1C, 0x0D, 0x0D}, 41: {0x01, 0x1B, 0x1B}, 42: {0x0E, 0x08, 0x08}, 43: {0x0F, 0x09, 0x00},
	44: {0x39, ' ', ' '}, 45: {0x0C, '-', '_'}, 46: {0x0D, '=', '+'}, 47: {0x1A, '[', '{'},
	48: {0x1B, ']', '}'}, 49: {0x2B, '\\', '|'}, 51: {0x27, ';', ':'}, 52: {0x28, '\'', '"'},
	53: {0x29, '`', '~'}, 54: {0x33, ',', '<'}, 55: {0x34, '.', '>'}, 56: {0x35, '/', '?'},
	86: {0x4A, '-', '-'}, 87: {0x4E, '+', '+'}, // keypad minus and plus
}

// grayKeys are the cursor block keys, which the BIOS stores with 0xE0 in the
// ASCII byte.
var grayKeys = map[int]byte{
	73: 0x52, 74: 0x47, 75: 0x49, 76: 0x53, 77: 0x4F, 78: 0x51, // Ins Home PgUp Del End PgDn
	79: 0x4D, 80: 0x4B, 81: 0x50, 82: 0x48, // Right Left Down Up
}

// biosWord turns a host key press into the word the original's INT 9 handler
// puts into last_key_pressed (scan << 8 | ASCII), or 0 for keys the BIOS
// would not report.
func biosWord(k host.Key) int {
	hid := k.Scancode
	switch {
	case hid >= 58 && hid <= 67: // F1..F10
		if k.Alt {
			return keyAltF1 + (hid-58)<<8
		}
		return keyF1 + (hid-58)<<8
	case hid == 68:
		return KeyF11
	case hid == 69:
		return KeyF12
	case hid == 88: // keypad Enter
		return KeyPadEnter
	}
	if s, ok := grayKeys[hid]; ok {
		return int(s)<<8 | extendedASCII
	}
	b, ok := keysByHID[hid]
	if !ok {
		return 0
	}
	ascii := b.plain
	switch {
	case k.Alt:
		ascii = 0
	case k.Ctrl && b.plain >= 'a' && b.plain <= 'z':
		ascii = b.plain - 'a' + 1
	case k.Shift:
		ascii = b.shifted
	}
	return int(b.scan)<<8 | int(ascii)
}

// KeyASCII is the character key k types, as the BIOS gives it (with Shift),
// 0 for none.
func KeyASCII(k host.Key) byte { return byte(biosWord(k)) }
