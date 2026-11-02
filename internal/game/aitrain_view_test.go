//go:build aitrain

package game

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/openorion2/openorion2/internal/aiplay"
)

// TestSimObserveControl: the rows a network gets over -control
// (aiplay.Player on /act's view) equal the simulator's (simObserve), so a
// network plays the game as it was trained.
func TestSimObserveControl(t *testing.T) {
	t.Parallel()
	for _, sv := range []string{"start", "mid", "late"} {
		withGames(t, []string{recipeSaveFile(t, sv, 1)}, func(gs []*Game) error {
			g := gs[0]
			g.currentScreen = ScreenMain
			me := int(g.state.PlayerNum)
			var track simTrack
			g.simTrackTurn(&track)
			var p aiplay.Player
			_, r, err := p.Turn([]byte(g.agentAct([]byte(`{"view":{}}`))))
			if err != nil {
				return err
			}
			want := &SimRecord{P: me}
			g.simObserve(&track, me, want)
			for name, pair := range map[string][2]any{"Me": {want.Me, r.Me}, "Des": {want.Des, r.Des}, "Pl": {want.Pl, r.Pl},
				"Star": {want.Star, r.Star}, "Pn": {want.Pn, r.Pn}, "Co": {want.Co, r.Co}, "Sh": {want.Sh, r.Sh}} {
				if !reflect.DeepEqual(pair[0], pair[1]) {
					return fmt.Errorf("%s: %s differs", sv, name)
				}
			}
			return nil
		})
	}
}
