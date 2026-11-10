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
// (net id 2, player 1) over an in-memory network; the others are AIs.
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
		g.state.Players[2].Objective = g.state.Players[3].Objective // net3's third human
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

// TestNetBattleBusySlots: a battle the host starts releases only the slots
// busy with its attacker at its own star (community patch 1.51); a slot busy
// with him in a battle elsewhere stays busy.
func TestNetBattleBusySlots(t *testing.T) {
	t.Parallel()
	s1 := recipeSaveFile(t, "net3", 1)
	withGames(t, []string{s1, s1}, func(gs []*Game) error {
		logs := make([][]NetRecord, 2)
		netPair(t, gs, logs)
		h := gs[0]
		h.cf = &combatFind{}
		h.settings[0xd8] = 0 // tactical: a human defender's machine fights
		s, n := &h.state, &h.net
		const att = 3              // an AI
		a, b := int16(0), int16(1) // the stars of two battles
		for _, c := range []struct {
			name string
			prep func()
		}{
			// No defender: Get_AI_Target_ finds no targets.
			{"no defender", func() { h.ct = combatTargets{} }},
			// A defender, human player 1, whose ship is the AI's war target.
			{"defender", func() {
				h.ct = combatTargets{nShipT: 1}
				h.ct.shipT[0] = 1
				P := &s.Players[att]
				P.AIWarTarget = s.Stars[b].Planets[0]
				P.Diplo7C9[1] = 1
			}},
		} {
			// Player 0 is in a battle with the AI at star a; player 2 waits at b.
			for k := range 15 {
				n.set8(slot(k), 0xe)
				n.set8(slot(k)+slBusy, 0xff)
			}
			n.set8(slot(0)+slStar, uint8(a))
			n.set8(slot(0)+slBusy, att)
			n.set8(slot(2)+slStar, uint8(b))
			n.set8(slot(2)+slBusy, att)
			c.prep()
			h.netStartBattle(b, att)
			if c.name == "defender" && n.i16(slot(att)+slDef) != 1 {
				t.Fatalf("%s: the AI's defender is %d, want 1", c.name, n.i16(slot(att)+slDef))
			}
			if got := n.i8(slot(0) + slBusy); got != att {
				t.Errorf("%s: player 0's slot (a battle at star %d) is busy with %d, want %d", c.name, a, got, att)
			}
			if got := n.i8(slot(2) + slBusy); got != -1 {
				t.Errorf("%s: player 2's slot (at the battle's star) is busy with %d, want -1", c.name, got)
			}
		}
		return nil
	})
}
