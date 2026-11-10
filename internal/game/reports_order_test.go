package game

import (
	"testing"

	"github.com/openorion2/openorion2/internal/save"
)

// TestReportOrder: community patch 1.51's begin-of-turn order (queried, as
// Must_Display_Reports_ does): a report before the tech pick, exploration
// before colonization, colonization before the hire offer.
func TestReportOrder(t *testing.T) {
	t.Parallel()
	type setup func(g *Game, p, st int)
	report := func(g *Game, p, st int) {
		s := &g.state
		s.NReports = 1
		rec := s.Reports[:reportSize]
		rec[0], rec[1], rec[2], rec[3] = 1, byte(p), 0, 0
	}
	breakthrough := func(g *Game, p, st int) { g.state.Players[p].BreakthroughPending = 1 }
	explored := func(g *Game, p, st int) { g.state.Stars[st].SpecialPopup |= 1 << uint(p) }
	colonize := func(g *Game, p, st int) {
		s := &g.state
		for _, pl := range s.Stars[st].Planets {
			if pl != -1 && s.Planets[pl].Colony != -1 {
				s.Colonies[s.Planets[pl].Colony].Buildings[11] = 1
			}
		}
		s.Stars[st].ColonizePlayer = int8(p)
		s.DoneColonizeCheck[st/8] &^= 1 << uint(st%8)
	}
	hire := func(g *Game, p, st int) {
		s := &g.state
		for l := range s.Leaders {
			if s.Leaders[l].Status == -1 {
				s.Players[p].OfficerForHire = int8(l)
				return
			}
		}
		t.Fatal("no leader for hire")
	}
	for _, c := range []struct {
		name string
		sets []setup
		want reportCheck
	}{
		{"report before breakthrough", []setup{report, breakthrough}, checkStolenTech},
		{"exploration before colonization", []setup{explored, colonize}, checkExplored},
		{"colonization before hire", []setup{colonize, hire}, checkColonization},
		{"exploration before hire", []setup{explored, hire}, checkExplored},
		{"breakthrough alone", []setup{breakthrough}, checkBreakthrough},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			g := sysInfoGame(t, "start")
			s := &g.state
			p := int(s.PlayerNum)
			pl := &s.Players[p]
			s.Stardate = 35010
			s.DonePlayerReports, s.DisplayedGNN = 0, 0xff
			s.NReports = 0
			pl.ResearchField = 1
			pl.OfficerForHire = -1
			pl.BreakthroughPending = 0
			pl.Picks[save.PickOmniscient] = 0
			for i := range s.DoneColonizeCheck {
				s.DoneColonizeCheck[i] = 0xff
			}
			home := -1
			for st := 0; st < int(s.NumStars); st++ {
				S := &s.Stars[st]
				S.SpecialPopup = 0
				if int8(S.Special) >= 0x32 {
					S.Special = 0
				}
				for _, k := range S.Planets {
					if k != -1 && s.Planets[k].Colony != -1 && int(s.Colonies[s.Planets[k].Colony].Owner) == p {
						home = st
					}
				}
			}
			if home < 0 {
				t.Fatal("no player star")
			}
			for _, set := range c.sets {
				set(g, p, home)
			}
			r, err := g.displayReportAux(p, false)
			if err != nil {
				t.Fatal(err)
			}
			if r == reportNone || g.rep.last != c.want {
				t.Errorf("report %d from check %d, want check %d", r, g.rep.last, c.want)
			}
		})
	}
}
