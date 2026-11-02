//go:build !js

package lobby

import (
	"testing"

	"github.com/openorion2/openorion2/lobbynet/lobbyv1"
)

func TestGameName(t *testing.T) {
	for desc, want := range map[string]string{
		"Small galaxy": "Small gal",
		"Галактика":    "Lobby",
		"Big 大銀河 war":  "Big  war",
		"":             "Lobby",
	} {
		if got := gameName(&lobbyv1.GameInfo{Description: desc}); got != want {
			t.Errorf("%q: %q, want %q", desc, got, want)
		}
	}
}
