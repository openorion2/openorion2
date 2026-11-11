package game

import "testing"

// TestRemoveEmbeddedSpecialCodes: the highlight codes come off a slot's
// description whole; a description saved with only the leading 0x03 (as
// SAVE1.GAM "\x0311") loses it without its last character doubling.
func TestRemoveEmbeddedSpecialCodes(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ in, want string }{
		{"\x0311", "11"},
		{"\x03Karaaw Hrik\x01", "Karaaw Hrik"},
		{"\x03x\x01", "x"},
		{"plain", "plain"},
	} {
		var d [37]byte
		copy(d[:], c.in)
		removeEmbeddedSpecialCodes(d[:])
		if got := string(d[:cLen(d[:])]); got != c.want {
			t.Errorf("%q: %q, want %q", c.in, got, c.want)
		}
	}
	// A full buffer keeps its end terminated.
	var d [4]byte
	copy(d[:], "\x03abc")
	removeEmbeddedSpecialCodes(d[:])
	if got := string(d[:cLen(d[:])]); got != "abc" {
		t.Errorf("full buffer: %q", got)
	}
}
