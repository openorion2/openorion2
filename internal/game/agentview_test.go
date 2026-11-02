package game

import (
	"encoding/json"
	"fmt"
	"github.com/openorion2/openorion2/internal/playview"
	"testing"
)

// TestAgentView: /act's view of a save's empire, then a turn's orders
// (research, a queue head, jobs, a ship's destination) carried out as the
// screens would.
func TestAgentView(t *testing.T) {
	t.Parallel()
	withGames(t, []string{recipeSaveFile(t, "mid", 1)}, func(gs []*Game) error {
		g := gs[0]
		g.currentScreen = ScreenMain
		s := &g.state
		me := int(s.PlayerNum)
		var v playview.View
		if err := json.Unmarshal([]byte(g.agentAct([]byte(`{"view":{}}`))), &v); err != nil {
			return err
		}
		if v.Me.Player != me || len(v.Stars) != int(s.NumStars) || len(v.Planets) != int(s.PlanetCount) || len(v.Colonies) == 0 || len(v.Ships) == 0 {
			return fmt.Errorf("view: player %d, %d stars, %d planets, %d colonies, %d ships", v.Me.Player, len(v.Stars), len(v.Planets), len(v.Colonies), len(v.Ships))
		}
		for _, sh := range v.Ships {
			if int(s.Ships[sh.ID].Owner) != me {
				return fmt.Errorf("ship %d is not his", sh.ID)
			}
		}
		// A colony with people to move and something new to build.
		var col *playview.Colony
		for i := range v.Colonies {
			if c := &v.Colonies[i]; !c.Outpost && c.Pop >= 3 && len(c.CanBuild) > 1 {
				col = c
				break
			}
		}
		if col == nil {
			return fmt.Errorf("no colony to order")
		}
		code := col.CanBuild[0]
		if code == col.Queue[0] {
			code = col.CanBuild[1]
		}
		o := playview.Orders{
			Queue: map[string][]int{fmt.Sprint(col.ID): {code}},
			Jobs:  map[string][2]int{fmt.Sprint(col.ID): {col.Farmers + 1, col.Workers - 1}},
		}
		if col.Workers == 0 {
			o.Jobs[fmt.Sprint(col.ID)] = [2]int{col.Farmers - 1, 1}
		}
		// A ship that may move, to the nearest other star.
		ship, to := -1, -1
		for _, sh := range v.Ships {
			if !sh.CanOrder || sh.At < 0 {
				continue
			}
			best := 1 << 30
			for i, st := range v.Stars {
				if d := (st.X-sh.X)*(st.X-sh.X) + (st.Y-sh.Y)*(st.Y-sh.Y); i != sh.At && d < best {
					best, to = d, i
				}
			}
			ship = sh.ID
			o.Dest = map[string]int{fmt.Sprint(ship): to}
			break
		}
		// A research choice made due.
		s.Players[me].Unknown323 = 1
		o.Research = v.Me.ResearchOptions
		b, _ := json.Marshal(map[string]any{"orders": o})
		var res struct {
			OK      bool
			Refused []string
		}
		if err := json.Unmarshal([]byte(g.agentAct(b)), &res); err != nil || !res.OK {
			return fmt.Errorf("orders: %v %+v", err, res)
		}
		c := &s.Colonies[col.ID]
		if got := s.queueCode(c.Queue[0]); got != code {
			return fmt.Errorf("queue head %d, want %d", got, code)
		}
		want := o.Jobs[fmt.Sprint(col.ID)]
		if j := jobCounts(c); j[0] != want[0] && j[1] != want[1] {
			return fmt.Errorf("jobs %v, want %v", j, want)
		}
		if len(v.Me.ResearchOptions) > 0 && int(s.Players[me].ResearchApp) != v.Me.ResearchOptions[0] {
			return fmt.Errorf("research %d, want %d", s.Players[me].ResearchApp, v.Me.ResearchOptions[0])
		}
		if ship >= 0 && shipDest(&s.Ships[ship]) != to && len(res.Refused) == 0 {
			return fmt.Errorf("ship %d bound for %d, want %d", ship, shipDest(&s.Ships[ship]), to)
		}
		t.Logf("refused: %v", res.Refused)
		return nil
	})
}
