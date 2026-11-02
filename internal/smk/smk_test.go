package smk

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"testing"
)

// TestPlanetaryDestruction decodes every frame of PLNTDFIN.LBX and checks
// the decoder ends where the file does.
func TestPlanetaryDestruction(t *testing.T) {
	data, err := os.ReadFile("../../game/PLNTDFIN.LBX")
	if err != nil {
		t.Skip(err)
	}
	v, err := Open(data)
	if err != nil {
		t.Fatal(err)
	}
	if v.Width != 288 || v.Height != 208 || v.Frames != 214 {
		t.Fatalf("header %dx%d %d frames", v.Width, v.Height, v.Frames)
	}
	h := sha256.New()
	for f := 0; f < v.Frames; f++ {
		if err := v.Next(); err != nil {
			t.Fatalf("frame %d: %v", f, err)
		}
		h.Write(v.Pixels)
	}
	t.Logf("frames hash %s", fmt.Sprintf("%x", h.Sum(nil))[:16])
}

// TestAllVideos decodes every frame of the game's other Smacker files.
func TestAllVideos(t *testing.T) {
	t.Parallel()
	for _, n := range []string{"INTRO", "ANATKFIN", "ANWINFIN", "GENWINFN", "LOSERFIN", "WININFIN", "ORIONFIN", "AMEBAFIN", "DIMTVFIN"} {
		data, err := os.ReadFile("../../game/" + n + ".LBX")
		if err != nil {
			t.Skip(err)
		}
		v, err := Open(data)
		if err != nil {
			t.Fatalf("%s: %v", n, err)
		}
		if v.Period.Microseconds() != 76920 {
			t.Errorf("%s: %v per frame", n, v.Period)
		}
		for f := 0; f < v.Frames; f++ {
			if err := v.Next(); err != nil {
				t.Fatalf("%s frame %d: %v", n, f, err)
			}
		}
	}
}

// TestBufferLeftovers: LOSERFIN frame 374 reads past its chunk, into what
// the largest earlier frame left in RAD's frame buffer (the frame oracle
// TestLoseFlicMuted shows the original's pixels); the decoder must read
// those bytes, not zeros.
func TestBufferLeftovers(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("../../game/LOSERFIN.LBX")
	if err != nil {
		t.Skip(err)
	}
	v, err := Open(data)
	if err != nil {
		t.Fatal(err)
	}
	for f := 0; f <= 374; f++ {
		if err := v.Next(); err != nil {
			t.Fatal(err)
		}
	}
	// The pixels the original showed (build/turns/losem, flip 375).
	if h := sha256.Sum256(v.Pixels); fmt.Sprintf("%x", h[:8]) != "0f2cd393825caac7" {
		t.Errorf("frame 374 hash %x", h[:8])
	}
}

// TestAudioTracks decodes every video's audio track and compares its size
// and SHA-256 with FFmpeg's decode of the same file (pcm_u8, taken once).
func TestAudioTracks(t *testing.T) {
	t.Parallel()
	want := map[string]struct {
		size int
		sum  string
	}{
		"INTRO": {2386392, "b1e02dbc46faf588"}, "PLNTDFIN": {362960, "37c40f3f34f0e5c9"},
		"AMEBAFIN": {298508, "f0de544ae4fa4ba9"}, "DIMTVFIN": {593628, "14efd31ab67c7d5d"},
		"ANATKFIN": {225576, "aabb56a9f44080ef"}, "ORIONFIN": {293420, "6f169b06fa4cb254"},
		"WININFIN": {853128, "c5d55e8fd10194f3"}, "GENWINFN": {819208, "23cebd5f3ef596e6"},
		"ANWINFIN": {547832, "6224b2bc94d8a752"}, "LOSERFIN": {656384, "63340e047cf104b5"},
	}
	for n, w := range want {
		t.Run(n, func(t *testing.T) {
			t.Parallel()
			data, err := os.ReadFile("../../game/" + n + ".LBX")
			if err != nil {
				t.Skip(err)
			}
			v, err := Open(data)
			if err != nil {
				t.Fatal(err)
			}
			h := sha256.New()
			size := 0
			for v.Frame < v.Frames {
				if err := v.Next(); err != nil {
					t.Fatal(err)
				}
				h.Write(v.Audio[0].Samples)
				size += len(v.Audio[0].Samples)
			}
			if got := hex.EncodeToString(h.Sum(nil))[:16]; size != w.size || got != w.sum {
				t.Errorf("%d bytes, sha256 %s; want %d, %s", size, got, w.size, w.sum)
			}
		})
	}
}
