package game

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/openorion2/openorion2/internal/aiplay"
)

// TestAIPlay: an empire played by a network over /act (internal/aiplay):
// the view becomes rows of the widths the network reads, and its action
// comes back as orders the game carries out.
func TestAIPlay(t *testing.T) {
	t.Parallel()
	for _, sv := range []string{"SAVE1.GAM", "SAVE2.GAM", "SAVE3.GAM"} {
		withGames(t, []string{filepath.Join(savesDir(), sv)}, func(gs []*Game) error {
			g := gs[0]
			g.currentScreen = ScreenMain
			s := &g.state
			var p aiplay.Player
			gm, r, err := p.Turn([]byte(g.agentAct([]byte(`{"view":{}}`))))
			if err != nil {
				return err
			}
			if len(r.Me) != 112 || len(r.Des) != 54 || len(r.Pl) != 80 || len(r.Star) != int(s.NumStars) || len(r.Pn) != int(s.PlanetCount) ||
				len(gm.Stars) != int(s.NumStars) || len(gm.Planets) != int(s.PlanetCount) || len(r.Co) == 0 || len(r.Sh) == 0 {
				return fmt.Errorf("%s: rows %d me, %d des, %d pl, %d stars, %d planets, %d colonies, %d ships",
					sv, len(r.Me), len(r.Des), len(r.Pl), len(r.Star), len(r.Pn), len(r.Co), len(r.Sh))
			}
			if w := len(r.Co[0]); w != 29 {
				return fmt.Errorf("%s: a colony row of %d", sv, w)
			}
			if w := len(r.Sh[0]); w != 19 {
				return fmt.Errorf("%s: a ship row of %d", sv, w)
			}
			// The action: all farmers but one on a colony, a ship to the
			// nearest other star.
			a := &aiplay.Action{Jobs: map[int][2]int{}, Dest: map[int]int{}}
			col := -1
			for _, row := range r.Co {
				if row[12] == 0 && row[2] >= 3 {
					col = row[0]
					a.Jobs[col] = [2]int{row[2] - 1, 1}
					break
				}
			}
			ship, to := -1, -1
			for _, row := range r.Sh {
				if row[6] == 0 || row[4] < 0 {
					continue
				}
				best := 1 << 30
				for i := 0; i < int(s.NumStars); i++ {
					dx, dy := int(s.Stars[i].X)-row[2], int(s.Stars[i].Y)-row[3]
					if d := dx*dx + dy*dy; i != row[4] && d < best {
						best, to = d, i
					}
				}
				ship = row[0]
				a.Dest[ship] = to
				break
			}
			var res struct {
				OK      bool
				Refused []string
			}
			if err := json.Unmarshal([]byte(g.agentAct([]byte(`{"orders":`+string(p.Orders(a))+`}`))), &res); err != nil || !res.OK {
				return fmt.Errorf("%s: orders: %v %+v", sv, err, res)
			}
			if col >= 0 {
				if j := jobCounts(&s.Colonies[col]); j[1] == 0 && j[2] == 0 {
					return fmt.Errorf("%s: colony %d jobs %v", sv, col, j)
				}
			}
			if ship >= 0 && shipDest(&s.Ships[ship]) != to && len(res.Refused) == 0 {
				return fmt.Errorf("%s: ship %d bound for %d, want %d", sv, ship, shipDest(&s.Ships[ship]), to)
			}
			return nil
		})
	}
}
