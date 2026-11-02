package game

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/openorion2/openorion2/internal/host"
	"github.com/openorion2/openorion2/internal/smk"
)

// TestPopupFlicsPlay plays the popup wrappers no trace reaches (amoeba,
// dimensional portal, Antaran attack, Orion) over a mid-game's galaxy: their
// files resolve and each ends on its last frame but one. Not compared with
// the original.
func TestPopupFlicsPlay(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("plays videos in real time")
	}
	dir := scratchGameDir(t, filepath.Join("..", "..", "game"))
	putRecipeSave(t, "mid", dir, 1)
	flips := 0
	err := host.Run(host.Options{Headless: true, Frames: 60 * 300}, func(sys *host.System) error {
		g, err := newGame(sys, dir)
		if err != nil {
			return err
		}
		g.Log = t.Logf
		if err := g.continueSave(1); err != nil {
			return err
		}
		g.e.OnToggle = func() { flips++ }
		for _, play := range []func() error{g.playAmoebaFlic, g.playDimensionalPortalFlic, g.playAntaransAttackFlic, g.playTakeOrionFlic} {
			if err := play(); err != nil {
				return err
			}
		}
		sys.Quit()
		return nil
	})
	if err != nil && err != host.ErrQuit {
		t.Fatal(err)
	}
	// Each shows frames 0..n-2 (176, 350, 133, 173 frames).
	if want := 175 + 349 + 132 + 172; flips != want {
		t.Errorf("%d flips, want %d", flips, want)
	}
}

// TestCinematicSound plays AMEBAFIN with music on: track 0 of every frame
// the video decodes goes to the mixer (internal/smk's TestAudioTracks
// checks the samples against FFmpeg's decode).
func TestCinematicSound(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("plays a video in real time")
	}
	dir := scratchGameDir(t, filepath.Join("..", "..", "game"))
	putRecipeSave(t, "mid", dir, 1)
	var played, frames int
	err := host.Run(host.Options{Headless: true, Frames: 60 * 120}, func(sys *host.System) error {
		g, err := newGame(sys, dir)
		if err != nil {
			return err
		}
		if err := g.continueSave(1); err != nil {
			return err
		}
		g.settings[0x13] = 1
		if err := g.playAmoebaFlic(); err != nil {
			return err
		}
		played, frames = g.lastSmackSound, g.lastSmackFrames
		sys.Quit()
		return nil
	})
	if err != nil && err != host.ErrQuit {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join("..", "..", "game", "AMEBAFIN.LBX"))
	if err != nil {
		t.Skip(err)
	}
	v, err := smk.Open(data)
	if err != nil {
		t.Fatal(err)
	}
	want := 0
	for v.Frame < frames {
		if err := v.Next(); err != nil {
			t.Fatal(err)
		}
		want += len(v.Audio[0].Samples)
	}
	if played == 0 || played != want {
		t.Errorf("%d sound bytes for %d frames, want %d", played, frames, want)
	}
}
