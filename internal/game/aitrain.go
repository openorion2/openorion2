//go:build aitrain

package game

import (
	"fmt"
	"github.com/openorion2/openorion2/internal/aiplay"
	"math/rand/v2"
	"os"
	"runtime/debug"

	"github.com/openorion2/openorion2/internal/host"
	"github.com/openorion2/openorion2/internal/lbx"
	"github.com/openorion2/openorion2/internal/save"
)

// port-only: games of the AI against itself, played without screens, that
// record what each empire saw and what the AI decided, to train a network
// to play (tools/aitrain, docs/aitrain.md).

// SimOptions asks Run for AI games instead of the menu.
type SimOptions struct {
	Games int    // games to play
	Turns int    // at most this many turns a game
	Seed  uint64 // the settings and the game RNG of game i come from Seed+i
	// Sample keeps an empire's turn with this probability (0 or 1: all).
	Sample float64
	// Game hears of each game before its first turn, Record of each kept
	// empire turn, Done of its end.
	Game   func(*SimGame)
	Record func(*SimRecord)
	Done   func(id uint64, turns int)
	// Control, when 0..7, is the empire Policy plays (if it exists): its
	// decisions replace the AI's. Done then hears of its result.
	Control int
	Policy  func(*SimRecord) *SimAction
	Result  func(id uint64, r SimResult)
	// With Record and Policy (DAgger): Record gets only Control's turns,
	// labelled with what the AI decided before the policy's decisions
	// replace them; on a Mix share of the turns the AI's stand.
	Mix float64
	// Snapshot, if set, gets a save of the game every SnapshotEvery turns,
	// with Control as the human, to look at in the game.
	Snapshot      func(id uint64, turn int, save []byte)
	SnapshotEvery int
}

// The network's rows and decisions (internal/aiplay).
type (
	SimGame   = aiplay.Game
	SimRecord = aiplay.Record
	SimAction = aiplay.Action
)

// SimResult is how the empire Control played fared at the end.
type SimResult struct {
	Turns, Players, Alive       int
	Eliminated                  bool
	Colonies, Pop, Fields, Rank int // Rank: 1 best, by population then fields
}

func designRow(d *save.Design) []int {
	return []int{int(d.Size), int(d.Type), int(d.Shield), int(d.Drive), int(d.Computer), int(d.Armor), weaponCount(d), int(d.CombatSpeed), int(d.Cost)}
}

func (g *Game) simGame(id uint64, setup []int) *SimGame {
	s := &g.state
	sg := &SimGame{ID: id, Setup: setup}
	for i := 0; i < int(s.NumStars); i++ {
		st := &s.Stars[i]
		sg.Stars = append(sg.Stars, []int{int(st.X), int(st.Y), int(st.Size), int(st.SpectralClass), int(st.Special), int(st.WormholeTo), int(st.InNebula)})
	}
	for i := 0; i < int(s.PlanetCount); i++ {
		pn := &s.Planets[i]
		sg.Planets = append(sg.Planets, []int{int(pn.Star), int(pn.Orbit), int(pn.Type), int(pn.Size), int(pn.Gravity), int(pn.Minerals), int(pn.Special)})
	}
	return sg
}

// simDecide appends what the AI steps left: the research to Me, the queue
// and jobs to each colony row, the destination to each ship row (-1 for
// one gone).
func (g *Game) simDecide(r *SimRecord) {
	s := &g.state
	pl := &s.Players[r.P]
	r.Me = append(r.Me, int(pl.ResearchField), int(pl.ResearchApp))
	for k, row := range r.Co {
		c := &s.Colonies[row[0]]
		for _, q := range c.Queue {
			row = append(row, s.queueCode(q))
		}
		j := jobCounts(c)
		r.Co[k] = append(row, j[0], j[1], j[2])
	}
	for k, row := range r.Sh {
		sh := &s.Ships[row[0]]
		d := -1
		if int(sh.Owner) == r.P && !shipGone(sh) {
			d = shipDest(sh)
		}
		r.Sh[k] = append(row, d)
	}
}

// Simulate plays opt.Games AI games without screens (tools/aitrain): each
// from a fresh Game sharing the archives (state outside State would carry
// over and make a game depend on the ones before it). It writes MOX.SET and
// SAVE10.GAM into dir, as a new game does.
func Simulate(sys *host.System, dir string, opt *SimOptions) error {
	var archives map[string]*lbx.Archive
	for n := 0; n < opt.Games; n++ {
		if err := simGame1(sys, dir, opt, n, &archives); err != nil {
			return err
		}
	}
	return nil
}

// simGame1 plays game n of simulate. A panic loses that game only.
func simGame1(sys *host.System, dir string, opt *SimOptions, n int, archives *map[string]*lbx.Archive) (err error) {
	defer func() {
		if p := recover(); p != nil {
			fmt.Fprintf(os.Stderr, "aitrain: game %d: %v\n%s", opt.Seed+uint64(n), p, debug.Stack())
		}
	}()
	{
		g, err := newGame(sys, dir)
		if err != nil {
			return err
		}
		if *archives != nil {
			g.lbx = *archives
		}
		*archives = g.lbx
		g.setDefaultGameSettings() // not the player's MOX.SET: the same rules everywhere
		id := opt.Seed + uint64(n)
		rng := rand.New(rand.NewPCG(id, 0x4f4f32))
		size := rng.IntN(4)
		players := 2 + rng.IntN([]int{4, 5, 7, 7}[size])
		setup := []int{rng.IntN(5), size, rng.IntN(3), players, rng.IntN(3)}
		g.settings[0xd4], g.settings[0xd6], g.settings[0xd7], g.settings[0xd5], g.settings[0xd9] =
			byte(setup[0]), byte(setup[1]), byte(setup[2]), byte(setup[3]), byte(setup[4])
		// The command line seed: Init_New_Game_ reseeds from it, not the clock.
		g.cmdLineSeed = uint32(id*0x9e3779b9) | 1
		if err := g.initNewGame(int16(setup[0]), int16(setup[1]), int16(setup[2]), int16(setup[3])); err != nil {
			return err
		}
		g.allAIGame, g.noCouncil, g.autopilot = true, true, true
		if opt.Game != nil {
			opt.Game(g.simGame(id, setup))
		}
		var obs []*SimRecord
		var track simTrack
		control := -1
		if opt.Policy != nil && opt.Control >= 0 && opt.Control < setup[3] {
			control = opt.Control
		}
		g.stepOverride = func(k int, st turnStep) bool {
			switch st.name {
			case "All_Colony_AI_":
				obs = obs[:0]
				g.simTrackTurn(&track)
				for p := 0; p < int(g.state.NumPlayers); p++ {
					if g.state.Players[p].Eliminated != 0 {
						continue
					}
					if p != control && (opt.Record == nil || control >= 0 || opt.Sample > 0 && opt.Sample < 1 && rng.Float64() >= opt.Sample) {
						continue
					}
					r := &SimRecord{Game: id, Turn: int(g.state.Stardate) - 35000, P: p}
					g.simObserve(&track, p, r)
					obs = append(obs, r)
				}
			case "Make_Scrap_Ships_Dead_":
				for _, r := range obs {
					if r.P == control {
						a := opt.Policy(r)
						if opt.Record != nil {
							g.simDecide(r)
							opt.Record(r)
						}
						if opt.Mix <= 0 || rng.Float64() >= opt.Mix {
							g.simApply(r, a)
						}
						continue
					}
					g.simDecide(r)
					if opt.Record != nil {
						opt.Record(r)
					}
				}
				obs = obs[:0]
			}
			return false
		}
		turns := 0
		for ; turns < opt.Turns && g.nLivingPlayers() > 1 && g.state.CouncilWinner < 0; turns++ {
			g.nextTurnCalc()
			if opt.Snapshot != nil && opt.SnapshotEvery > 0 && (turns+1)%opt.SnapshotEvery == 0 {
				opt.Snapshot(id, turns+1, g.simSave(opt.Control, turns+1))
			}
		}
		if opt.Done != nil {
			opt.Done(id, turns)
		}
		if opt.Result != nil && opt.Control >= 0 && opt.Control < setup[3] {
			opt.Result(id, g.simResult(opt.Control, turns))
		}
	}
	return nil
}

// simApply puts the policy's decisions over what the AI steps decided for
// r.P (r is what it saw before them).
func (g *Game) simApply(r *SimRecord, a *SimAction) {
	s := &g.state
	pl := &s.Players[r.P]
	if a == nil {
		return
	}
	// Research: only when the AI chose this turn.
	if int(pl.ResearchField) != r.Me[7] || int(pl.ResearchApp) != r.Me[8] {
		for _, app := range a.Research {
			if app < 0 || app >= len(pl.AppStatus) || pl.AppStatus[app] == 3 {
				continue
			}
			if f := g.techAppFieldOf(app); f >= 0 && f < len(pl.FieldStatus) && pl.FieldStatus[f] == 2 {
				pl.ResearchApp, pl.ResearchField = uint8(app), uint8(f)
				break
			}
		}
	}
	saved := g.colonyHandle
	defer func() { g.colonyHandle = saved }()
	for ci, items := range a.Queue {
		c := &s.Colonies[ci]
		if int(c.Owner) != r.P || c.IsOutpost != 0 {
			continue
		}
		g.colonyHandle = ci
		head := s.queueCode(c.Queue[0])
		for _, code := range items {
			if code == head {
				break
			}
			v := g.queueEntry(ci, code)
			if v == 0 {
				continue
			}
			if c.Queue[6] != -1 {
				g.deleteFromColonyQueue(ci, 6)
			}
			copy(c.Queue[1:], c.Queue[:6])
			c.Queue[0] = v
			break
		}
	}
	for ci, want := range a.Jobs {
		c := &s.Colonies[ci]
		if int(c.Owner) != r.P || c.IsOutpost != 0 {
			continue
		}
		g.setJobs(ci, want[0], want[1])
	}
	// The empire must not starve: farmers again where the network took too
	// many (the colonies screen's moves, one at a time).
	for i := 0; i < int(s.NumColonies) && pl.FoodSurplus < 0 && len(a.Jobs) > 0; i++ {
		c := &s.Colonies[i]
		if int(c.Owner) != r.P || c.IsOutpost != 0 {
			continue
		}
		for pl.FoodSurplus < 0 {
			j := jobCounts(c)
			if j[1]+j[2] == 0 || !g.oneMoreFarmer(i, j[1] > j[2]) {
				break
			}
		}
	}
	for _, row := range r.Sh {
		id, at := row[0], row[4]
		if row[6] == 0 { // it could not take orders
			continue
		}
		to, ok := a.Dest[id]
		sh := &s.Ships[id]
		if !ok || int(sh.Owner) != r.P || shipGone(sh) || to == shipDest(sh) || to == at && shipDest(sh) < 0 {
			continue
		}
		if row[1] == save.ShipInTransit {
			// In flight (with the communications to reach it): -1 keeps
			// its course, a star turns it there.
			if to < 0 || sh.Status != save.ShipInTransit || !s.canOrderShip(id) {
				continue
			}
			if info := g.shipsTryToMoveTo([]int16{int16(id)}, to); info.canMove {
				g.makeShipsMoveTo([]int16{int16(id)}, to, &info)
			}
			continue
		}
		// Still the ship seen at that star: at it or just leaving it (the
		// AI steps may have freed it and queued a new ship in its record).
		if sh.Status != save.ShipAtStar && sh.Status != save.ShipDeparting || at < 0 || at >= int(s.NumStars) ||
			sh.X != s.Stars[at].X || sh.Y != s.Stars[at].Y {
			continue
		}
		// Back at its star, then the policy's order.
		sh.Status, sh.Location = save.ShipAtStar, int16(at)
		if to >= 0 && to != at {
			if info := g.shipsTryToMoveTo([]int16{int16(id)}, to); info.canMove {
				g.makeShipsMoveTo([]int16{int16(id)}, to, &info)
			}
		}
	}
}

func (g *Game) simResult(p, turns int) SimResult {
	s := &g.state
	r := SimResult{Turns: turns, Players: int(s.NumPlayers), Eliminated: s.Players[p].Eliminated != 0}
	type score struct{ pop, fields int }
	sc := make([]score, s.NumPlayers)
	for i := 0; i < int(s.NumColonies); i++ {
		c := &s.Colonies[i]
		if c.Owner >= 0 && int(c.Owner) < len(sc) {
			sc[c.Owner].pop += int(c.PopCount)
			if int(c.Owner) == p {
				r.Colonies++
			}
		}
	}
	for q := range sc {
		for _, f := range s.Players[q].FieldStatus {
			if f == 3 {
				sc[q].fields++
			}
		}
		if s.Players[q].Eliminated == 0 {
			r.Alive++
		}
	}
	r.Pop, r.Fields, r.Rank = sc[p].pop, sc[p].fields, 1
	for q := range sc {
		if q != p && (sc[q].pop > sc[p].pop || sc[q].pop == sc[p].pop && sc[q].fields > sc[p].fields) {
			r.Rank++
		}
	}
	return r
}

// simSave is a save of the game seen by empire p. The empire keeps its AI
// objective (a human one would make the autopilot draw a new one and clear
// the queues): -policy plays on the same game.
func (g *Game) simSave(p, turn int) []byte {
	s := &g.state
	if p < 0 || p >= int(s.NumPlayers) {
		p = 0
	}
	num := s.PlayerNum
	s.PlayerNum = int16(p)
	defer func() { s.PlayerNum = num }()
	setCString(s.Descriptions[0][:], fmt.Sprintf("aitrain turn %d", turn))
	return g.encodeSave(0)
}

// simTrack is what the observations remember between turns: how long each
// ship has stood at its star and each colony has built its queue's head.
type simTrack struct {
	shipLoc, shipIdle map[int]int
	head, headAge     map[int]int
}

// simTrackTurn updates t, once a turn before the observations.
func (g *Game) simTrackTurn(t *simTrack) {
	s := &g.state
	if t.shipLoc == nil {
		*t = simTrack{map[int]int{}, map[int]int{}, map[int]int{}, map[int]int{}}
	}
	for i := 0; i < int(s.NumShips); i++ {
		sh := &s.Ships[i]
		loc := -1
		if !shipGone(sh) && sh.Status == save.ShipAtStar {
			loc = int(sh.Location)
		}
		if prev, ok := t.shipLoc[i]; ok && prev == loc && loc >= 0 {
			t.shipIdle[i]++
		} else {
			t.shipIdle[i] = 0
		}
		t.shipLoc[i] = loc
	}
	for i := 0; i < int(s.NumColonies); i++ {
		h := s.queueCode(s.Colonies[i].Queue[0])
		if prev, ok := t.head[i]; ok && prev == h {
			t.headAge[i]++
		} else {
			t.headAge[i] = 0
		}
		t.head[i] = h
	}
}
