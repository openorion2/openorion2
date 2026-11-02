package game

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	"github.com/openorion2/openorion2/internal/playview"
	"github.com/openorion2/openorion2/internal/save"
)

// port-only, development: the player's view and turn orders for the remote
// control's /act (docs/control.md, "Playing by numbers"). What a human in
// PLAYER_NUM's place sees, by record number, and the orders he could give
// on the main screen, for a program that plays the empire.

// queueCode is a queue entry as a queue code: a ship being built (-100 -
// its record) by its kind.
func (s *State) queueCode(v int16) int {
	if v <= save.QueueShipBase && int(-v-100) < len(s.Ships) {
		d := &s.Ships[-v-100].Design
		return -1000 - (int(d.Type)*8 + int(d.Size))
	}
	return int(v)
}

func shipDest(sh *save.Ship) int {
	switch l := int(sh.Location); {
	case l >= save.LocationDeparted:
		return l - save.LocationDeparted
	case l >= save.LocationTransit:
		return l - save.LocationTransit
	}
	return -1
}

// shipStar is the star a ship is at or bound for, or -1.
func shipStar(sh *save.Ship) int {
	if sh.Status == save.ShipAtStar {
		return int(sh.Location)
	}
	return shipDest(sh)
}

func shipGone(sh *save.Ship) bool {
	return sh.Status == save.ShipFree || sh.Status == save.ShipDestroyed || sh.Status == save.ShipScrapped
}

func weaponCount(d *save.Design) int {
	w := 0
	for _, s := range d.Weapons {
		if s.Weapon > 0 {
			w += int(s.Count)
		}
	}
	return w
}

func designView(d *save.Design) playview.Design {
	return playview.Design{Size: int(d.Size), Type: int(d.Type), Shield: int(d.Shield), Drive: int(d.Drive),
		Computer: int(d.Computer), Armor: int(d.Armor), Weapons: weaponCount(d), CombatSpeed: int(d.CombatSpeed), Cost: int(d.Cost)}
}

func jobCounts(c *save.Colony) (n [3]int) {
	for i := 0; i < int(c.PopCount) && i < len(c.Colonists); i++ {
		if j := save.Colonist(c.Colonists[i]).Job(); j < 3 {
			n[j]++
		}
	}
	return n
}

// playerView is empire p's view (p need not be PLAYER_NUM).
func (g *Game) playerView(p int) *playview.View {
	s := &g.state
	pl := &s.Players[p]
	v := &playview.View{Setup: playview.Setup{Difficulty: int(g.settings[0xd4]), GalaxySize: int(g.settings[0xd6]),
		GalaxyAge: int(g.settings[0xd7]), Players: int(s.NumPlayers), TechLevel: int(g.settings[0xd9])}}
	v.Me = playview.Me{Player: p, Stardate: int(s.Stardate), Race: int(pl.Race), BC: int(pl.BC), Tax: int(pl.TaxRate),
		ResearchPoints: int(pl.ResearchPoints), ResearchField: int(pl.ResearchField), ResearchApp: int(pl.ResearchApp),
		Food: int(pl.FoodSurplus), Income: int(pl.NetIncome), Industry: int(pl.IndustryProduced), Research: int(pl.ResearchProduced),
		Population: int(pl.Population), CommandTotal: int(pl.CommandTotal), CommandUsed: int(pl.CommandUsed),
		FreightersTotal: int(pl.FreightersTotal), FreightersFree: int(pl.FreightersFree), FTLSpeed: int(pl.FTLSpeed),
		FuelRange: int(pl.FuelRange), HomePlanet: int(pl.HomePlanet), ResearchOptions: []int{}, ResearchDue: g.researchDue(p)}
	for _, f := range pl.FieldStatus {
		v.Me.FieldStatus = append(v.Me.FieldStatus, int(f))
	}
	for a := range pl.AppStatus {
		if g.mayResearchApp(p, a) {
			v.Me.ResearchOptions = append(v.Me.ResearchOptions, a)
		}
	}
	for i := range pl.Designs {
		v.Designs = append(v.Designs, designView(&pl.Designs[i]))
	}
	for q := 0; q < int(s.NumPlayers); q++ {
		o := &s.Players[q]
		vp := playview.Player{Eliminated: o.Eliminated != 0, Contact: int(pl.Contact[q]), Race: -1}
		if q == p || vp.Contact != 0 {
			vp.Race, vp.Relations, vp.Treaty = int(o.Race), int(pl.Relations[q]), int(pl.TreatyStatus[q])
		}
		v.Players = append(v.Players, vp)
	}
	for i := 0; i < int(s.NumStars); i++ {
		st := &s.Stars[i]
		vs := playview.Star{X: int(st.X), Y: int(st.Y), Size: int(st.Size), Class: int(st.SpectralClass), Special: int(st.Special),
			Wormhole: int(st.WormholeTo), Nebula: int(st.InNebula), Visited: s.playerHasVisited(p, i), Blockade: st.Blockaded&(1<<p) != 0}
		for q := 0; q < int(s.NumPlayers); q++ {
			if st.ColonyOwners&(1<<q) != 0 {
				if q == p {
					vs.Mine = true
				} else {
					vs.Others = vs.Visited
				}
			}
		}
		v.Stars = append(v.Stars, vs)
	}
	detected := g.detectedShips(p)
	for i := 0; i < int(s.NumShips); i++ {
		sh := &s.Ships[i]
		at := shipStar(sh)
		if shipGone(sh) || at < 0 || at >= len(v.Stars) || int(sh.Owner) != p && !detected[i] {
			continue
		}
		vs := &v.Stars[at]
		switch o := int(sh.Owner); {
		case o == p:
			vs.OwnShips++
			vs.OwnWeapons += weaponCount(&sh.Design)
		case o < 8:
			vs.OtherShips++
			vs.OtherWeapons += weaponCount(&sh.Design)
		default:
			vs.Monsters++
		}
	}
	for i := 0; i < int(s.PlanetCount); i++ {
		pn := &s.Planets[i]
		vp := playview.Planet{Star: int(pn.Star), Orbit: int(pn.Orbit), Type: int(pn.Type), Size: int(pn.Size), Gravity: int(pn.Gravity),
			Minerals: int(pn.Minerals), Special: int(pn.Special), Owner: -1}
		if int(pn.Star) < int(s.NumStars) && s.playerHasVisited(p, int(pn.Star)) {
			vp.Known, vp.Climate, vp.MaxFarms = true, int(pn.Climate), int(pn.MaxFarms)
			if c := int(pn.Colony); c >= 0 {
				col := &s.Colonies[c]
				vp.Owner, vp.Outpost = int(col.Owner), col.IsOutpost != 0
				if vp.Owner == p {
					vp.Pop = int(col.PopCount)
				}
			}
		}
		v.Planets = append(v.Planets, vp)
	}
	for i := 0; i < int(s.NumColonies); i++ {
		c := &s.Colonies[i]
		if int(c.Owner) != p {
			continue
		}
		j := jobCounts(c)
		vc := playview.Colony{ID: i, Planet: int(c.Planet), Pop: int(c.PopCount), Farmers: j[0], Workers: j[1], Scientists: j[2],
			Morale: int(c.Morale), Pollution: int(c.Pollution), Outpost: c.IsOutpost != 0, Production: int(c.Production),
			Marines: int(c.Marines), Armor: int(c.Armor), Buildings: []int{}, CanBuild: []int{}}
		for k, o := range c.Output {
			vc.Output[k] = int(o)
		}
		for _, q := range c.Queue {
			vc.Queue = append(vc.Queue, s.queueCode(q))
		}
		for b, on := range c.Buildings {
			if on != 0 {
				vc.Buildings = append(vc.Buildings, b)
			}
		}
		for _, code := range playview.QueueCodes {
			if g.queueCodeSlot(i, code) >= 0 {
				vc.CanBuild = append(vc.CanBuild, code)
			}
		}
		v.Colonies = append(v.Colonies, vc)
	}
	for i := 0; i < int(s.NumShips); i++ {
		sh := &s.Ships[i]
		if int(sh.Owner) != p || shipGone(sh) {
			continue
		}
		at := -1
		if sh.Status == save.ShipAtStar {
			at = int(sh.Location)
		}
		v.Ships = append(v.Ships, playview.Ship{ID: i, Status: int(sh.Status), X: int(sh.X), Y: int(sh.Y), At: at, Dest: shipDest(sh),
			CanOrder: s.canOrderShip(i), Experience: int(sh.Experience), Design: designView(&sh.Design), ETA: int(sh.ETA)})
	}
	return v
}

// researchDue: the empire's research needs a choice (its field is known, or
// the game asks again).
func (g *Game) researchDue(p int) bool {
	pl := &g.state.Players[p]
	return pl.FieldStatus[pl.ResearchField] == techKnown || pl.Unknown323 != 0
}

// mayResearchApp: application a is in a field p may research now, and in
// this game.
func (g *Game) mayResearchApp(p, a int) bool {
	pl := &g.state.Players[p]
	f := g.techAppFieldOf(a)
	return f >= 0 && f < len(pl.FieldStatus) && pl.FieldStatus[f] == 2 && g.playerMayGetTechApp(p, a)
}

// detectedShips marks the others' ships empire p sees on its galaxy map:
// Find_Ship_Stacks_ with Remove_Non_Detected_Ships_ as p.
func (g *Game) detectedShips(p int) []bool {
	s := &g.state
	saved, num := g.stacks, s.PlayerNum
	s.PlayerNum = int16(p)
	defer func() { g.stacks, s.PlayerNum = saved, num }()
	g.findShipStacks()
	out := make([]bool, s.NumShips)
	st := &g.stacks
	for r := int16(0); r < st.count; r++ {
		for n := st.start[r]; n != -1; n = st.node[n].next {
			if k := int(st.node[n].ship); k < len(out) {
				out[k] = true
			}
		}
	}
	return out
}

// queueCodeSlot reports whether colony ci can put code at the head of its
// queue: -1 if not, else 0, or for a ship the design slot it is built from
// (bqRefitSlot for colony ships, outposts and transports). It changes
// nothing.
func (g *Game) queueCodeSlot(ci, code int) int {
	s := &g.state
	c := &s.Colonies[ci]
	switch {
	case c.IsOutpost != 0:
		return -1
	case code == -2 || code == -3:
		return 0
	case code > 0 && code < 0x31 || code == -4 || code == -5 || code == -6 || code == -7 || code == -15:
		if g.colonyCanBuildProduct(ci, int16(code)) != 0 && (code < 0 || c.Buildings[code] == 0) {
			return 0
		}
		return -1
	case code > -1000:
		return -1
	}
	kind := -1000 - code
	typ, size := kind/8, kind%8
	pl := &s.Players[c.Owner]
	switch typ {
	case 1, 2, 4:
		ok := pl.FuelRange > 0 && pl.FTLSpeed > 0
		switch typ {
		case 1:
			ok = ok && pl.AppStatus[appColonyShip] == 3
		case 4:
			ok = ok && pl.AppStatus[appOutpost] == 3
		case 2:
			ok = ok && pl.AppStatus[appTransport] == 3 && c.Buildings[0x16] != 0
		}
		if ok {
			return bqRefitSlot
		}
	case 0:
		for k := 4; k >= 0; k-- {
			if d := &pl.Designs[k]; d.Type == 0 && int(d.Size) == size && g.colonyCanBuildProduct(ci, int16(-(k+50))) != 0 {
				return k
			}
		}
	}
	return -1
}

// queueEntry turns a queue code into an entry of colony ci's queue: a ship
// kind becomes a new ship record of a design of that kind. 0 when the
// colony cannot build it.
func (g *Game) queueEntry(ci, code int) int16 {
	s := &g.state
	c := &s.Colonies[ci]
	slot := g.queueCodeSlot(ci, code)
	switch {
	case slot == -1:
		return 0
	case code > -1000:
		return int16(code)
	}
	owner := int(c.Owner)
	pl := &s.Players[owner]
	if slot == bqRefitSlot {
		d := &pl.Designs[bqRefitSlot]
		switch (-1000 - code) / 8 {
		case 1:
			g.loadColonyShipDesign(owner, d)
		case 4:
			g.loadOutpostShipDesign(owner, d)
		case 2:
			g.loadTransportShipDesign(owner, d)
		}
	}
	sh := g.createShip(owner, slot, ci)
	if sh == -1 {
		return 0
	}
	s.Ships[sh].Status = 6
	return -(sh + 100)
}

// applyOrders carries out o for empire p; it lists what it refused.
func (g *Game) applyOrders(p int, o *playview.Orders) []string {
	s := &g.state
	pl := &s.Players[p]
	refused := []string{}
	no := func(f string, a ...any) { refused = append(refused, fmt.Sprintf(f, a...)) }
	colony := func(k string) int {
		ci, err := strconv.Atoi(k)
		if err != nil || ci < 0 || ci >= int(s.NumColonies) || int(s.Colonies[ci].Owner) != p || s.Colonies[ci].IsOutpost != 0 {
			no("colony %s: not his", k)
			return -1
		}
		return ci
	}
	if len(o.Research) > 0 && g.researchDue(p) {
		done := false
		for _, a := range o.Research {
			if a >= 0 && a < len(pl.AppStatus) && pl.AppStatus[a] != techKnown && g.mayResearchApp(p, a) {
				pl.ResearchApp, pl.ResearchField = uint8(a), uint8(g.techAppFieldOf(a))
				done = true
				break
			}
		}
		if !done {
			no("research: none of %v", o.Research)
		}
	}
	saved := g.colonyHandle
	defer func() { g.colonyHandle = saved }()
	for _, k := range sortedKeys(o.Queue) {
		ci := colony(k)
		if ci < 0 {
			continue
		}
		c := &s.Colonies[ci]
		g.colonyHandle = ci
		head := s.queueCode(c.Queue[0])
		for _, code := range o.Queue[k] {
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
	for _, k := range sortedKeys(o.Jobs) {
		if ci := colony(k); ci >= 0 {
			g.setJobs(ci, o.Jobs[k][0], o.Jobs[k][1])
		}
	}
	for i := 0; i < int(s.NumColonies) && o.Feed && pl.FoodSurplus < 0; i++ {
		c := &s.Colonies[i]
		if int(c.Owner) != p || c.IsOutpost != 0 {
			continue
		}
		for pl.FoodSurplus < 0 {
			j := jobCounts(c)
			if j[1]+j[2] == 0 || !g.oneMoreFarmer(i, j[1] > j[2]) {
				break
			}
		}
	}
	for _, k := range sortedKeys(o.Dest) {
		id, err := strconv.Atoi(k)
		to := o.Dest[k]
		if err != nil || id < 0 || id >= int(s.NumShips) || int(s.Ships[id].Owner) != p || shipGone(&s.Ships[id]) {
			no("ship %s: not his", k)
			continue
		}
		sh := &s.Ships[id]
		if !s.canOrderShip(id) {
			no("ship %d: cannot be ordered now", id)
			continue
		}
		if to < 0 || to >= int(s.NumStars) || to == shipDest(sh) || sh.Status == save.ShipAtStar && to == int(sh.Location) {
			continue
		}
		if sh.Status != save.ShipAtStar && sh.Status != save.ShipInTransit {
			continue
		}
		if info := g.shipsTryToMoveTo([]int16{int16(id)}, to); info.canMove {
			g.makeShipsMoveTo([]int16{int16(id)}, to, &info)
		} else {
			no("ship %d: cannot reach star %d", id, to)
		}
	}
	return refused
}

func sortedKeys[V any](m map[string]V) []string {
	k := make([]string, 0, len(m))
	for s := range m {
		k = append(k, s)
	}
	sort.Slice(k, func(i, j int) bool {
		a, _ := strconv.Atoi(k[i])
		b, _ := strconv.Atoi(k[j])
		return a < b
	})
	return k
}

// oneMoreFarmer makes one more farmer at colony ci, of a worker or a
// scientist.
func (g *Game) oneMoreFarmer(ci int, worker bool) bool {
	j := jobCounts(&g.state.Colonies[ci])
	f, w := j[0]+1, j[1]
	if worker {
		w--
	}
	before := j[0]
	g.setJobs(ci, f, w)
	return jobCounts(&g.state.Colonies[ci])[0] > before
}

// setJobs moves colonists of colony ci, one at a time as the colonies
// screen does, toward f farmers and w workers (the rest scientists).
func (g *Game) setJobs(ci, f, w int) {
	c := &g.state.Colonies[ci]
	move := func(a, b int) bool {
		if b == 0 && g.sumColonists(ci, 0, -1, -1, 0, -1) >= int(c.CanFarm) {
			return false
		}
		for k := 0; k < int(c.PopCount); k++ {
			p := c.Colonists[k]
			if int(p>>7&3) != a || p&0x200 == 0 || p&0xf == 9 {
				continue
			}
			if st := g.popToPopState(ci, k); (st == 3 || st == 6) && b != 0 || st == 4 {
				continue
			}
			c.Colonists[k] = p&^0x180 | uint32(b&3)<<7 | 0x200
			g.colonyCalculation(ci)
			return true
		}
		return false
	}
	for i := 0; i < 2*len(c.Colonists); i++ {
		n := jobCounts(c)
		want := [3]int{f, w, int(c.PopCount) - f - w}
		from, to := -1, -1
		for j := 0; j < 3; j++ {
			if n[j] > want[j] && from < 0 {
				from = j
			}
			if n[j] < want[j] && to < 0 {
				to = j
			}
		}
		if from < 0 || to < 0 || !move(from, to) {
			return
		}
	}
}

// agentViewAct answers /act {"view": {}} and {"orders": {...}}, on the
// main screen.
func (g *Game) agentViewAct(view bool, orders *playview.Orders) string {
	p := int(g.state.PlayerNum)
	if p < 0 || p >= int(g.state.NumPlayers) || g.state.Players[p].Eliminated != 0 {
		return "no empire"
	}
	if view {
		b, err := json.Marshal(g.playerView(p))
		if err != nil {
			return "error: " + err.Error()
		}
		return string(b)
	}
	refused := g.applyOrders(p, orders)
	b, _ := json.Marshal(map[string]any{"ok": true, "refused": refused})
	return string(b)
}
