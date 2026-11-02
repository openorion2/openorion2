package game

import (
	"fmt"

	"github.com/openorion2/openorion2/internal/save"
)

// The score: Do_Calculate_Hi_Score_ and the SCORE cheat that shows it.
// Spec: docs/screens/score.md.

const (
	specialBonusTable = 0x17d1fa // _special_bonus: 3 pick values per ability row
	scoreFormat       = 0x179731 // "Current score for %s is %d"
	scoreSeedShip     = 0x68     // see hiScoreModifierPlayer
)

// hiScore is the part of the _hi block (0x125 bytes in _global_data_seg)
// Calculate_Hi_Scores_ fills; offsets in the comments.
type hiScore struct {
	modifierPlayer uint8 // +0x68, never set by Do_Calculate_Hi_Score_
	antares        int16 // +0x69
	captured       int16 // +0x6c
	elimination    int16 // +0x76
	eliminated     int16 // +0x88
	capturedPop    int16 // +0x8c
	pop            int16 // +0xa0
	orion          int16 // +0xa2
	popScore       int16 // +0xa4
	tech           int16 // +0xa8
	total          int16 // +0xaa
	time           int16 // +0xac
	final          int16 // +0xae
	council        int16 // +0xb0
	modifier       int16 // +0xb6
}

// hiScoreModifierPlayer is the unset _hi+0x68: Find_Ship_Stacks_'s flag for
// ship 0x68 (status >= 3), stale with fewer ships (the port takes 0).
func (g *Game) hiScoreModifierPlayer() uint8 {
	s := &g.state
	if int(s.NumShips) > scoreSeedShip && int8(s.Ships[scoreSeedShip].Status) >= 3 {
		return 1
	}
	return 0
}

// doCalculateHiScore is Do_Calculate_Hi_Score_ (0x9db51).
func (g *Game) doCalculateHiScore(p int) (int16, error) {
	h := hiScore{modifierPlayer: g.hiScoreModifierPlayer()}
	if err := g.calculateHiScores(p, &h); err != nil {
		return 0, err
	}
	return h.final, nil
}

// calculateHiScores is Calculate_Hi_Scores_ (0x9d977); the modifier comes
// from the player at h.modifierPlayer, as the hi-score screens set it.
func (g *Game) calculateHiScores(p int, h *hiScore) error {
	s := &g.state
	pl := &s.Players[p]
	h.elimination, h.eliminated = s.eliminationScore(p)
	fields, hyper := s.technologyCounts(p)
	h.tech = fields*3 + hyper*5
	h.antares, h.orion, h.council = 0, 0, 0
	if pl.CapturedAntares != 0 {
		h.antares = 250
	}
	if pl.CapturedOrion != 0 {
		h.orion = 100
	}
	if pl.WonCouncil != 0 {
		h.council = 100
	}
	size := int32(g.settings[0xd6])
	h.capturedPop = pl.CapturedColonists
	h.captured = int16(int32(uint16(pl.CapturedColonists)) * 2 / (size + 1))
	h.pop = s.totalPopulation(p)
	h.popScore = h.pop
	h.time = int16(int32(int16(size+1)*20+80)*int32(s.NumPlayers) - int32(int16(uint16(s.Stardate)-35000)))
	if h.pop == 0 {
		h.time = 0
	}
	h.total = h.time + h.elimination + h.tech + h.antares + h.orion + h.council + h.captured + h.popScore
	m, err := g.playerScoreModifier(int(h.modifierPlayer))
	if err != nil {
		return err
	}
	h.modifier = m
	h.final = int16((int32(h.total)*int32(h.modifier) + 50) / 100)
	return nil
}

// eliminationScore is Get_Elimination_Score_ (0x9e84c): 50 per player p
// has eliminated, and their count.
func (s *State) eliminationScore(p int) (score, n int16) {
	for k := 0; k < int(s.NumPlayers); k++ {
		if s.Players[p].Eliminations[k] != 0 {
			score += 50
			n++
		}
	}
	return
}

// technologyCounts is Get_Technology_ (0x9e90b): researched fields and
// hyper-advanced levels reached.
func (s *State) technologyCounts(p int) (fields, hyper int16) {
	pl := &s.Players[p]
	for _, st := range pl.FieldStatus {
		if st == techKnown {
			fields++
		}
	}
	for _, l := range pl.HyperLevels {
		if l != 0 {
			hyper++
		}
	}
	return
}

// totalPopulation is Get_Total_Population_ (0x9e9da), outposts included.
func (s *State) totalPopulation(p int) int16 {
	var n int16
	for i := 0; i < int(s.NumColonies); i++ {
		if int16(s.Colonies[i].Owner) == int16(p) {
			n += int16(s.Colonies[i].PopCount)
		}
	}
	return n
}

// playerScoreModifier is Player_Score_Modifier_ (0x58f4a): +10% per unspent
// race pick; the race screen globals it sets are not kept.
func (g *Game) playerScoreModifier(q int) (int16, error) {
	arc, err := g.archive("RACESTUF.LBX")
	if err != nil {
		return 0, err
	}
	costs, err := arc.Entry(6)
	if err != nil {
		return 0, err
	}
	pl := &g.state.Players[q]
	buttons, toggles := convertPlayerToFlagsEM(g, &pl.Picks)
	picks := calculatePicks(int16(g.exeU8(initialPicks)), costs, &buttons, &toggles)
	if knows(pl, 0x3e) {
		picks += 4
	}
	if picks > 0 {
		return picks*10 + 100, nil
	}
	return 100, nil
}

// convertPlayerToFlagsEM is Convert_Player_To_Flags_EM_ (0x5b4cd): picks as
// the race screen's buttons and toggles.
func convertPlayerToFlagsEM(g *Game, picks *[31]int8) (buttons [10]int16, toggles [22]int16) {
	buttons[9] = int16(int32(picks[save.PickGovernment]) / 2)
	for k := 0; k < 9; k++ {
		row := uint32(specialBonusTable + (k+1)*3)
		switch v := uint8(picks[k+1]); v {
		case g.exeU8(row):
			buttons[k] = 1
		case g.exeU8(row + 1):
			buttons[k] = 2
		case g.exeU8(row + 2):
			buttons[k] = 3
		}
	}
	off := 0
	for k := 10; k < 31; k++ {
		i := k - 10 + off
		toggles[i] = int16(picks[k])
		if k == save.PickRichPoorHomeWorld {
			if toggles[i] == -1 {
				toggles[i], toggles[i+1] = 0, 1
			} else {
				toggles[i+1] = 0
			}
			off = 1
		}
	}
	return
}

// calculatePicks is Calculate_Picks_ (0x59053): picks left from start after
// paying for every choice.
func calculatePicks(start int16, costs []byte, buttons *[10]int16, toggles *[22]int16) int16 {
	picks := start
	for k := 0; k < 9; k++ {
		if b := buttons[k]; b != 0 {
			picks -= int16(int8(costs[k*3+int(b)-1]))
		}
	}
	picks -= int16(int8(costs[int(buttons[9])+0x1b]))
	for t := 0; t < 22; t++ {
		if toggles[t] == 1 {
			picks -= int16(int8(costs[t+0x1f]))
		}
	}
	return picks
}

// scoreCheat is the SCORE branch of Check_Hot_Keys_ (0x82a68).
func (g *Game) scoreCheat() error {
	s := &g.state
	p := int(s.PlayerNum)
	v, err := g.doCalculateHiScore(p)
	if err != nil {
		return err
	}
	name := save.CString(s.Players[p].EmperorName[:])
	return g.mainUserBox(fmt.Sprintf(cFormat(cStringAt(g.exe, scoreFormat)), name, v))
}
