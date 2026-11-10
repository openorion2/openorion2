package game

import (
	"fmt"
	"slices"
	"testing"

	"github.com/openorion2/openorion2/internal/netplay"
)

// Network fixes of the community patch 1.51 (docs/net/combat.md,
// docs/net/turn.md).

// netPair attaches gs[0] (net id 1, the turn's host, player 0) and gs[1]
// (net id 2, player 1) over an in-memory network; the other players are AIs.
func netPair(t *testing.T, gs []*Game, logs [][]NetRecord) {
	t.Helper()
	mem := netplay.NewMem()
	for m, g := range gs {
		ep, err := mem.Create("G", "a")
		if m > 0 {
			ep, err = mem.Join("G", "b")
		}
		if err != nil {
			t.Fatal(err)
		}
		attach(g, ep, &logs[m])
		for k := range g.state.Players {
			g.state.Players[k].NetID = -1
		}
		g.state.Players[0].NetID, g.state.Players[1].NetID = 1, 2
		g.state.PlayerNum = int16(m)
		g.net.set16(netHostID, 1)
	}
}

// TestNetTargetInfoFields: msgs 0x10 and 0x12 carry each side's tech field
// status (community patch 1.51), so the battle's machine has the host's.
func TestNetTargetInfoFields(t *testing.T) {
	t.Parallel()
	s1 := recipeSaveFile(t, "net3", 1)
	withGames(t, []string{s1, s1}, func(gs []*Game) error {
		logs := make([][]NetRecord, 2)
		netPair(t, gs, logs)
		h, c := gs[0], gs[1]
		h.cf = &combatFind{} // Search_For_Battles_' tables, as in Next_Turn_
		star := int16(h.state.Planets[h.state.Colonies[0].Planet].Star)
		for i, send := range []struct {
			name string
			f    func()
		}{
			{"msg 0x10", func() { h.sendPickTargetInfo(1, star) }},
			{"msg 0x12", func() { h.sendYouAreTargetInfo(1, 0, -1, -1, star) }},
		} {
			// An earlier battle on the host moved both sides' research on.
			for _, p := range []int{0, 1} {
				h.state.Players[p].FieldStatus[7+i] ^= 1
				h.state.Players[p].FieldStatus[40+i] ^= 2
			}
			send.f()
			c.netUpdate()
			for _, p := range []int{0, 1} {
				a, b := h.playerRecord(p)[0xc4:0x1eb], c.playerRecord(p)[0xc4:0x1eb]
				if !slices.Equal(a, b) {
					t.Errorf("%s: player %d's record +0xc4..+0x1eb differs on the fighting machine", send.name, p)
				}
			}
		}
		return nil
	})
}

// TestNetTargetInfoChunks: a pick target message longer than 0x7800 bytes
// goes in chunks (community patch 1.51: was 0x8000).
func TestNetTargetInfoChunks(t *testing.T) {
	t.Parallel()
	s1 := recipeSaveFile(t, "net3", 1)
	withGames(t, []string{s1, s1}, func(gs []*Game) error {
		logs := make([][]NetRecord, 2)
		netPair(t, gs, logs)
		h := gs[0]
		if err := h.loadNetNextTurnScreen(false); err != nil {
			return err
		}
		for _, k := range []struct {
			n    int
			want string
		}{{0x7800, "[0x10]"}, {0x7801, "[0x2e 0x2f 0x2f]"}, {0x7f00, "[0x2e 0x2f 0x2f]"}} {
			logs[0] = nil
			h.sendNetMsg(2, 0x10, 0x2e, 0x2f, make([]byte, k.n))
			var ids []string
			for _, r := range logs[0] {
				if r.Dir == 'S' {
					ids = append(ids, fmt.Sprintf("%#x", r.ID))
				}
			}
			if got := fmt.Sprint(ids); got != k.want {
				t.Errorf("%#x bytes: sent %s, want %s", k.n, got, k.want)
			}
		}
		return nil
	})
}
