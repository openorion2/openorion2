package sound

import (
	"fmt"
	"testing"

	"github.com/openorion2/openorion2/internal/lbx"
)

// TestSoundFiles parses every WAV entry of the sound archives; the other
// entries are SOUND.LBX 0 (a name table) and 22-byte placeholders.
func TestSoundFiles(t *testing.T) {
	t.Parallel()
	for _, n := range []string{"SOUND.LBX", "STREAM.LBX", "STREAMHD.LBX"} {
		a, err := lbx.Open("../../game/" + n)
		if err != nil {
			t.Skip(err)
		}
		kinds := map[string]int{}
		for i := 0; i < a.Len(); i++ {
			e, err := a.Entry(i)
			if err != nil {
				t.Fatal(err)
			}
			w, err := ParseWAV(e)
			if err != nil {
				if len(e) == 22 || n == "SOUND.LBX" && i == 0 {
					continue
				}
				t.Errorf("%s %d: %v", n, i, err)
				continue
			}
			kinds[fmt.Sprintf("%dHz/%dbit/%dch", w.Rate, w.Bits, w.Channels)]++
		}
		t.Logf("%s: %v", n, kinds)
	}
}
