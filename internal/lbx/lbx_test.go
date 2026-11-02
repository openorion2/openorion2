package lbx

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGameArchives parses every archive shipped with the game. The game files
// are not in the repository, so the test skips when they are absent.
func TestGameArchives(t *testing.T) {
	files, _ := filepath.Glob(filepath.Join("..", "..", "game", "*"))
	var n, videos int
	for _, f := range files {
		if !strings.EqualFold(filepath.Ext(f), ".lbx") {
			continue
		}
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		a, err := Parse(data)
		if errors.Is(err, ErrSmacker) {
			videos++
			continue
		}
		if err != nil {
			t.Errorf("%s: %v", f, err)
			continue
		}
		n++
		// Every shipped archive packs its entries right after the header
		// and up to the last byte of the file.
		if a.Len() > 0 && (a.Offsets[0] != HeaderSize || int(a.Offsets[a.Len()]) != len(data)) {
			t.Errorf("%s: entries span 0x%x..0x%x in a 0x%x-byte file", f, a.Offsets[0], a.Offsets[a.Len()], len(data))
		}
		for i := 0; i < a.Len(); i++ {
			if _, err := a.Entry(i); err != nil {
				t.Errorf("%s: %v", f, err)
			}
		}
	}
	if n == 0 {
		t.Skip("game files not present")
	}
	t.Logf("%d archives, %d Smacker videos", n, videos)
}

func TestParseRejects(t *testing.T) {
	for name, data := range map[string][]byte{
		"short":    {1, 2, 3},
		"magic":    {1, 0, 0xad, 0xfe + 1, 0, 0, 0, 0, 0, 8, 0, 0, 0, 8, 0, 0},
		"smacker":  []byte("SMK2\x00\x00\x00\x00"),
		"overrun":  {1, 0, 0xad, 0xfe, 0, 0, 0, 0, 0x10, 0, 0, 0, 0xff, 0, 0, 0},
		"backward": {1, 0, 0xad, 0xfe, 0, 0, 0, 0, 0x10, 0, 0, 0, 0x08, 0, 0, 0},
	} {
		if _, err := Parse(data); err == nil {
			t.Errorf("%s: parsed without error", name)
		}
	}
}
