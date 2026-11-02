// Package aiplay plays an empire with a trained network over the remote
// control (docs/aiplay.md): the player's view (playview) becomes the
// network's rows, its decisions become a turn's orders. The network runs
// as a child process (tools/aiplay/serve.py). Port-only.
package aiplay

import (
	"encoding/json"
	"strconv"

	"github.com/openorion2/openorion2/internal/playview"
)

// Game is what does not change in a game. Row widths: docs/aiplay.md.
type Game struct {
	ID      uint64
	Setup   []int   // difficulty, size, age, players, tech level
	Stars   [][]int // x, y, size, class, special, wormhole, nebula
	Planets [][]int // star, orbit, type, size, gravity, minerals, special
}

// Record is one empire's turn as the network sees it. Owners are relative
// to P: 0 itself, 1..7 the others in turn, 8.. monsters and Antarans, -1
// none.
type Record struct {
	Game uint64
	Turn int
	P    int
	Me   []int   // the empire, its FieldStatus, what it may research
	Des  []int   // its six designs
	Pl   []int   // the eight players, from P on
	Star [][]int // per star: what is there
	Pn   [][]int // per planet: owner, colony
	Co   [][]int // its colonies
	Sh   [][]int // its ships
}

// Action is the network's decisions for a Record.
type Action struct {
	Research []int         // applications, when a choice is due
	Queue    map[int][]int // colony: queue codes for its head
	Jobs     map[int][2]int
	Dest     map[int]int // ship: the star to go to, -1 to stay
}

// QueueClasses are the queue codes of the network's 74 queue classes, 0
// for none.
var QueueClasses = func() (c [74]int) {
	for k := 1; k <= 48; k++ {
		c[k] = k
	}
	for k := 50; k <= 63; k++ {
		c[k] = 48 - k
	}
	for k := 64; k <= 69; k++ {
		c[k] = -1000 - (k - 64)
	}
	c[70], c[71], c[72] = -1008, -1016, -1032
	return c
}()

// RelOwner is owner as seen from empire p.
func RelOwner(owner, p int) int {
	switch {
	case owner < 0:
		return -1
	case owner < 8:
		return (owner - p + 8) % 8
	}
	return owner
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// FromView is v's rows; idle and age are a ship's turns at its star and a
// colony's turns on its queue head.
func FromView(v *playview.View, idle, age func(int) int) (*Game, *Record) {
	p := v.Me.Player
	sg := &Game{Setup: []int{v.Setup.Difficulty, v.Setup.GalaxySize, v.Setup.GalaxyAge, v.Setup.Players, v.Setup.TechLevel}}
	for _, st := range v.Stars {
		sg.Stars = append(sg.Stars, []int{st.X, st.Y, st.Size, st.Class, st.Special, st.Wormhole, st.Nebula})
	}
	for _, pn := range v.Planets {
		sg.Planets = append(sg.Planets, []int{pn.Star, pn.Orbit, pn.Type, pn.Size, pn.Gravity, pn.Minerals, pn.Special})
	}
	m := &v.Me
	r := &Record{Turn: m.Stardate - 35000, P: p}
	r.Me = []int{m.Stardate - 35000, m.Race, 0, 0, m.BC, m.Tax, m.ResearchPoints, m.ResearchField, m.ResearchApp, m.Food, m.Income,
		m.Industry, m.Research, m.Population, m.CommandTotal, m.CommandUsed, m.FreightersTotal, m.FreightersFree, m.FTLSpeed,
		m.FuelRange, m.HomePlanet}
	r.Me = append(r.Me, m.FieldStatus...)
	mask := make([]int, 7)
	for _, a := range m.ResearchOptions {
		mask[a/32] |= 1 << (a % 32)
	}
	r.Me = append(r.Me, mask...)
	r.Me = append(r.Me, b2i(m.ResearchDue))
	for _, d := range v.Designs {
		r.Des = append(r.Des, DesignRow(d)...)
	}
	for k := 0; k < 8; k++ {
		q := (p + k) % 8
		if q >= len(v.Players) {
			r.Pl = append(r.Pl, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0)
			continue
		}
		o := &v.Players[q]
		if q != p && o.Contact == 0 {
			r.Pl = append(r.Pl, 1, b2i(o.Eliminated), -1, 0, 0, 0, 0, 0, 0, 0)
			continue
		}
		r.Pl = append(r.Pl, 1, b2i(o.Eliminated), o.Race, o.Relations, o.Treaty, o.Contact, 0, 0, 0, 0)
	}
	for _, st := range v.Stars {
		r.Star = append(r.Star, []int{b2i(st.Visited), b2i(st.Mine), b2i(st.Others), b2i(st.Blockade),
			st.OwnShips, st.OwnWeapons, st.OtherShips, st.OtherWeapons, st.Monsters})
	}
	for _, pn := range v.Planets {
		if !pn.Known {
			r.Pn = append(r.Pn, []int{-2, 0, 0, 0, 0, 0})
			continue
		}
		r.Pn = append(r.Pn, []int{RelOwner(pn.Owner, p), pn.Pop, b2i(pn.Outpost), pn.Climate, pn.MaxFarms, 1})
	}
	for _, c := range v.Colonies {
		row := []int{c.ID, c.Planet, c.Pop, c.Farmers, c.Workers, c.Scientists, c.Output[0], c.Output[1], c.Output[2], c.Output[3],
			c.Morale, c.Pollution, b2i(c.Outpost), c.Production, c.Marines, c.Armor}
		row = append(row, c.Queue...)
		var bits [2]int
		for _, b := range c.Buildings {
			bits[b/32] |= 1 << (b % 32)
		}
		row = append(row, bits[0], bits[1])
		legal := make([]int, 3)
		for _, code := range c.CanBuild {
			for k, cl := range QueueClasses {
				if cl == code && cl != 0 {
					legal[k/32] |= 1 << (k % 32)
				}
			}
		}
		row = append(row, legal...)
		r.Co = append(r.Co, append(row, age(c.ID)))
	}
	for _, sh := range v.Ships {
		row := []int{sh.ID, sh.Status, sh.X, sh.Y, sh.At, sh.Dest, b2i(sh.CanOrder), sh.Experience}
		row = append(row, DesignRow(sh.Design)...)
		r.Sh = append(r.Sh, append(row, sh.ETA, idle(sh.ID)))
	}
	return sg, r
}

// DesignRow is a design as the network's row.
func DesignRow(d playview.Design) []int {
	return []int{d.Size, d.Type, d.Shield, d.Drive, d.Computer, d.Armor, d.Weapons, d.CombatSpeed, d.Cost}
}

// OrdersOf is the network's action a for the record r as a turn's orders.
func OrdersOf(r *Record, a *Action) *playview.Orders {
	o := &playview.Orders{Queue: map[string][]int{}, Jobs: map[string][2]int{}, Dest: map[string]int{}}
	if a == nil || r == nil {
		return o
	}
	if len(r.Me) > 111 && r.Me[111] == 1 {
		o.Research = a.Research
	}
	for ci, items := range a.Queue {
		o.Queue[strconv.Itoa(ci)] = items
	}
	for ci, j := range a.Jobs {
		o.Jobs[strconv.Itoa(ci)] = j
	}
	o.Feed = len(a.Jobs) > 0
	for _, row := range r.Sh {
		id, at, dest := row[0], row[4], row[5]
		to, ok := a.Dest[id]
		if !ok || row[6] == 0 || to == dest || to < 0 || to == at {
			continue
		}
		o.Dest[strconv.Itoa(id)] = to
	}
	return o
}

// Player plays an empire turn by turn: each view in, the network's rows;
// its action out as orders. It keeps what the rows remember between
// turns.
type Player struct {
	shipLoc, shipIdle, head, headAge map[int]int
	last                             *Record
}

// Turn reads a view (JSON) and returns the network's rows.
func (vp *Player) Turn(view []byte) (*Game, *Record, error) {
	var v playview.View
	if err := json.Unmarshal(view, &v); err != nil {
		return nil, nil, err
	}
	if vp.shipLoc == nil {
		vp.shipLoc, vp.shipIdle, vp.head, vp.headAge = map[int]int{}, map[int]int{}, map[int]int{}, map[int]int{}
	}
	for _, sh := range v.Ships {
		loc := -1
		if sh.Status == 0 { // at a star
			loc = sh.At
		}
		if prev, ok := vp.shipLoc[sh.ID]; ok && prev == loc && loc >= 0 {
			vp.shipIdle[sh.ID]++
		} else {
			vp.shipIdle[sh.ID] = 0
		}
		vp.shipLoc[sh.ID] = loc
	}
	for _, c := range v.Colonies {
		h := c.Queue[0]
		if prev, ok := vp.head[c.ID]; ok && prev == h {
			vp.headAge[c.ID]++
		} else {
			vp.headAge[c.ID] = 0
		}
		vp.head[c.ID] = h
	}
	g, r := FromView(&v, func(i int) int { return vp.shipIdle[i] }, func(i int) int { return vp.headAge[i] })
	vp.last = r
	return g, r, nil
}

// Orders is the last view's action a as an /act orders body.
func (vp *Player) Orders(a *Action) []byte {
	b, _ := json.Marshal(OrdersOf(vp.last, a))
	return b
}
