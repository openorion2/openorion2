package game

import (
	"encoding/json"
	"fmt"
	"github.com/openorion2/openorion2/internal/playview"
	"strings"

	"github.com/openorion2/openorion2/internal/save"
)

// port-only, development: the game as an agent sees it, for the remote
// control's /state (docs/control.md): the screen and its box, the
// player's empire, the stars he has visited, his fleets.

type agentState struct {
	Screen   string        `json:"screen"`
	Box      string        `json:"box,omitempty"` // a message, confirmation or text box up, its text
	BoxKind  string        `json:"box_kind,omitempty"`
	Stardate string        `json:"stardate"`
	Player   *agentPlayer  `json:"player,omitempty"`
	Colonies []agentColony `json:"colonies,omitempty"`
	Fleets   []agentFleet  `json:"fleets,omitempty"`
	Stars    []agentStar   `json:"stars,omitempty"`
	Options  []agentOption `json:"options,omitempty"` // what can be clicked here, by name
}

// agentOption is a field of the screen with what it means.
type agentOption struct {
	Label string `json:"label"`
	X     int    `json:"x"`
	Y     int    `json:"y"`
}

type agentPlayer struct {
	Race       string       `json:"race"`
	BC         int32        `json:"bc"`
	Research   string       `json:"research,omitempty"`
	ResearchRP int32        `json:"research_rp"`
	Food       int          `json:"food"` // the empire's food surplus
	Designs    []string     `json:"designs"`
	Others     []agentRival `json:"others,omitempty"` // the empires met
}

type agentRival struct {
	Race      string `json:"race"`
	Human     bool   `json:"human"`
	Relations int8   `json:"relations"` // -100..100
	Treaty    string `json:"treaty"`
	Trade     bool   `json:"trade_treaty,omitempty"`
	Research  bool   `json:"research_treaty,omitempty"`
}

type agentColony struct {
	Name       string   `json:"name"`
	Star       string   `json:"star"`
	Outpost    bool     `json:"outpost,omitempty"`
	Pop        int      `json:"pop"`
	Farmers    int      `json:"farmers"`
	Workers    int      `json:"workers"`
	Scientists int      `json:"scientists"`
	Output     [4]int16 `json:"output"` // food, industry, research, BC
	Queue      []string `json:"queue"`
	CanBuild   []string `json:"can_build"`
}

type agentFleet struct {
	At    string   `json:"at,omitempty"`
	To    string   `json:"to,omitempty"`
	ETA   int      `json:"eta,omitempty"`
	Ships []string `json:"ships"`
}

type agentStar struct {
	Name    string        `json:"name"`
	X       int16         `json:"x"`
	Y       int16         `json:"y"`
	Owner   string        `json:"owner,omitempty"`
	Planets []agentPlanet `json:"planets,omitempty"`
}

type agentPlanet struct {
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	Size    int    `json:"size"`
	Climate int    `json:"climate"`
	Mineral int    `json:"minerals"`
	Owner   string `json:"owner,omitempty"`
}

var agentScreens = map[int]string{
	ScreenMain: "main", ScreenColony: "colony", ScreenFleets: "fleets", ScreenRaces: "races",
	ScreenGamePopup: "game menu", ScreenInfo: "info", ScreenMainMenu: "main menu",
	ScreenNewGame: "new game", ScreenMultiplayer: "multiplayer", ScreenColonySummary: "colonies",
	ScreenPlanetSummary: "planets", ScreenLeaders: "leaders", ScreenReports: "reports",
	ScreenTurnSummary: "turn summary", ScreenTechChange: "tech change", ScreenLanding: "landing",
	ScreenColonizeInMain: "colonize", ScreenOutpostInMain: "outpost", ScreenTransportInMain: "transport",
}

// agentState describes the game now; it runs on the game's goroutine.
func (g *Game) agentState() any {
	st := agentState{Screen: agentScreens[g.currentScreen]}
	if st.Screen == "" {
		st.Screen = "screen " + itoa(g.currentScreen)
	}
	switch {
	case g.ts.showing:
		st.Screen = "research choice"
	case g.tc != nil && g.tc.scr != nil && g.tc.scr.humanInput:
		st.Screen = "tactical combat"
	case g.lobbyScr != nil:
		st.Screen = "lobby"
	case g.mp.agentSelecting:
		st.Screen = "choose planet"
	case g.sr.showing:
		st.Screen = "science room"
	case g.currentScreen == ScreenMain && !g.agentMainUp():
		// Something over the map with its own loop (an ambassador, a
		// report): not the main screen's own input.
		st.Screen = "over main"
	}
	if g.currentScreen == ScreenBuildQueue {
		st.Screen = "build queue"
	}
	u := &g.ub
	switch {
	case u.confirming:
		st.BoxKind = "confirm"
	case u.textBoxing, u.exploding:
		st.BoxKind = "message"
	}
	if st.BoxKind != "" {
		st.Box = u.agentText
	}
	s := &g.state
	if s.NumPlayers == 0 || g.currentScreen == ScreenMainMenu || g.currentScreen == ScreenMultiplayer {
		return st
	}
	st.Options = g.agentOptions()
	st.Stardate = g.stardateString()
	me := int(s.PlayerNum)
	if me < 0 || me >= int(s.NumPlayers) {
		return st
	}
	p := &s.Players[me]
	ap := &agentPlayer{Race: g.agentRace(me), BC: p.BC, ResearchRP: int32(p.ResearchProduced), Food: int(p.FoodSurplus)}
	if name, err := g.technologyFieldsName(int(p.ResearchField)); err == nil {
		ap.Research = name
	}
	st.Player = ap
	for k := range p.Designs {
		if n := save.CString(p.Designs[k].Name[:]); n != "" {
			ap.Designs = append(ap.Designs, n)
		}
	}
	treaties := []string{"none", "non-aggression pact", "alliance", "peace", "war", "war", "war"}
	for k := 0; k < int(s.NumPlayers); k++ {
		if k == me || p.Contact[k] == 0 || s.Players[k].Eliminated != 0 {
			continue
		}
		t := "?"
		if int(p.TreatyStatus[k]) < len(treaties) {
			t = treaties[p.TreatyStatus[k]]
		}
		ap.Others = append(ap.Others, agentRival{Race: g.agentRace(k), Human: s.Players[k].Objective == save.ObjectiveHuman,
			Relations: p.Relations[k], Treaty: t, Trade: p.TradeTreaty[k] != 0, Research: p.ResearchTreaty[k] != 0})
	}

	for ci := 0; ci < int(s.NumColonies); ci++ {
		c := &s.Colonies[ci]
		if int(c.Owner) != me {
			continue
		}
		ac := agentColony{Name: g.planetName(int(c.Planet)), Star: save.CString(s.Stars[s.Planets[c.Planet].Star].Name[:]),
			Outpost: c.IsOutpost != 0, Pop: int(c.PopCount), Output: c.Output}
		for k := 0; k < int(c.PopCount); k++ {
			switch c.Colonists[k] >> 7 & 3 {
			case 0:
				ac.Farmers++
			case 1:
				ac.Workers++
			case 2:
				ac.Scientists++
			}
		}
		for _, q := range c.Queue {
			if q != -1 && q != 0 {
				ac.Queue = append(ac.Queue, g.selectionName(q))
			}
		}
		ac.CanBuild = g.agentCanBuild(ci)
		st.Colonies = append(st.Colonies, ac)
	}

	starName := func(i int) string {
		if i < 0 || i >= int(s.NumStars) {
			return ""
		}
		return save.CString(s.Stars[i].Name[:])
	}
	fleets := map[[3]int]*agentFleet{}
	var order [][3]int
	for i := 0; i < int(s.NumShips); i++ {
		sh := &s.Ships[i]
		if int(sh.Owner) != me || sh.Status == save.ShipDestroyed || sh.Status == save.ShipScrapped || sh.Status == save.ShipBuilding || sh.Status == save.ShipFree {
			continue
		}
		key := [3]int{int(sh.Status), int(sh.Location), int(sh.ETA)}
		f := fleets[key]
		if f == nil {
			f = &agentFleet{}
			switch loc := int(sh.Location); {
			case loc >= save.LocationDeparted:
				f.To = starName(loc - save.LocationDeparted)
			case loc >= save.LocationTransit:
				f.To = starName(loc - save.LocationTransit)
			default:
				f.At = starName(loc)
			}
			if f.To != "" {
				f.ETA = int(sh.ETA)
			}
			fleets[key] = f
			order = append(order, key)
		}
		kind := map[uint8]string{0: "warship", 1: "colony", 2: "transport", 4: "outpost"}[sh.Design.Type]
		if kind == "warship" && strings.Contains(strings.ToLower(save.CString(sh.Design.Name[:])), "scout") {
			kind = "scout"
		}
		if kind == "" {
			kind = "other"
		}
		f.Ships = append(f.Ships, save.CString(sh.Design.Name[:])+" ("+kind+")")
	}
	for _, k := range order {
		st.Fleets = append(st.Fleets, *fleets[k])
	}

	kinds := map[uint8]string{1: "asteroids", 2: "gas giant", 3: "planet"}
	for i := 0; i < int(s.NumStars); i++ {
		star := &s.Stars[i]
		as := agentStar{Name: starName(i), X: star.X, Y: star.Y}
		if star.Owner >= 0 && int(star.Owner) < int(s.NumPlayers) {
			as.Owner = g.agentRace(int(star.Owner))
		}
		if star.Visited&(1<<me) != 0 {
			for _, pl := range star.Planets {
				if pl < 0 {
					continue
				}
				P := &s.Planets[pl]
				ap := agentPlanet{Name: g.planetName(int(pl)), Kind: kinds[P.Type], Size: int(P.Size), Climate: int(P.Climate), Mineral: int(P.Minerals)}
				if P.Colony >= 0 {
					ap.Owner = g.agentRace(int(s.Colonies[P.Colony].Owner))
				}
				as.Planets = append(as.Planets, ap)
			}
		}
		st.Stars = append(st.Stars, as)
	}
	return st
}

func (g *Game) agentRace(p int) string {
	if p < 0 || p >= int(g.state.NumPlayers) {
		return ""
	}
	return save.CString(g.state.Players[p].RaceName[:])
}

// agentOptions are the screen's fields the game knows the meaning of.
func (g *Game) agentOptions() []agentOption {
	e := g.e
	var out []agentOption
	add := func(label string, f int) {
		if f <= 0 || f >= e.FieldsCount() || label == "" {
			return
		}
		x1, y1, x2, y2 := e.FieldRect(f)
		if x1 < 0 || y1 < 0 || x1 > 639 || y1 > 479 {
			return
		}
		out = append(out, agentOption{Label: label, X: (x1 + x2) / 2, Y: (y1 + y2) / 2})
	}
	s := &g.state
	u := &g.ub
	switch {
	case u.confirming || u.textBoxing || u.exploding:
		return nil // keys answer boxes: y, n, enter
	case g.ts.showing:
		for i := range g.ts.entries {
			en := &g.ts.entries[i]
			if en.field == 0 {
				continue
			}
			for k := 0; k < int(en.napps); k++ {
				add(en.name+": "+en.appNames[k], int(en.appFields[k]))
			}
		}
	case g.tc != nil && g.tc.scr != nil && g.tc.scr.humanInput:
		add("AUTO", g.tc.scr.autoField)
	case g.mp.agentSelecting:
		for i := range g.sd.disp {
			pl := g.sd.disp[i].planet
			if pl < 0 || int(pl) >= len(s.Planets) {
				continue
			}
			P := &s.Planets[pl]
			l := g.planetName(int(pl))
			if P.Colony >= 0 {
				l += " (taken)"
			}
			add(l, g.sd.planetButtons[i])
		}
		add("CLOSE", g.mp.closeField)
	case g.currentScreen == ScreenBuildQueue:
		bq := &g.bq
		for i := 0; i < len(bq.bldg) && i < len(bq.bldgF); i++ {
			add(g.selectionName(bq.bldg[i]), bq.bldgF[i])
		}
		for i := 0; i < len(bq.mil) && i < len(bq.milF); i++ {
			add(g.selectionName(bq.mil[i]), bq.milF[i])
		}
		for i, v := range bq.cur {
			if v != -1 && i < len(bq.qF) {
				add(fmt.Sprintf("queue %d: %s", i+1, g.selectionName(v)), bq.qF[i])
			}
		}
		add("OK", bq.fOK)
		add("CANCEL", bq.fCancel)
	case g.currentScreen == ScreenColony:
		cs := &g.cscr
		add("CHANGE", cs.fields[cfBuild])
		add("BUY", cs.fields[cfBuy])
		add("RETURN", cs.fields[cfReturn])
	case g.currentScreen == ScreenColonySummary:
		cs := &g.cs
		for i, c := range cs.listCol {
			if c < 0 || int(c) >= int(s.NumColonies) {
				continue
			}
			name := g.planetName(int(s.Colonies[c].Planet))
			add(name, cs.listF[i])
			for j, job := range []string{"farmers", "workers", "scientists"} {
				add(name+" "+job, cs.jobF[i][j])
			}
		}
		add("RETURN", cs.ret)
	case g.currentScreen == ScreenMain:
		f := &g.ms.fields
		for _, b := range []struct {
			l string
			f int
		}{{"COLONIES", f.colonies}, {"PLANETS", f.planets}, {"FLEETS", f.fleets}, {"LEADERS", f.leaders},
			{"RACES", f.races}, {"INFO", f.info}, {"TURN", f.next}} {
			add(b.l, b.f)
		}
	}
	return out
}

// agentAct carries out an order the screens would give, on the main screen:
//
//	{"move": {"from": "Mentar", "to": "Zin", "kind": "all|scouts|outposts|colony|warships"}}
func (g *Game) agentAct(req []byte) string {
	var a struct {
		Move   *struct{ From, To, Kind string } `json:"move"`
		Jobs   *struct{ Colony, Focus string }  `json:"jobs"`
		View   *struct{}                        `json:"view"`
		Orders *playview.Orders                 `json:"orders"`
	}
	if err := json.Unmarshal(req, &a); err != nil {
		return "bad request: " + err.Error()
	}
	if g.currentScreen != ScreenMain || g.ub.confirming || g.ub.textBoxing || g.ub.exploding {
		return "not on the main screen"
	}
	if a.View != nil || a.Orders != nil {
		return g.agentViewAct(a.View != nil, a.Orders)
	}
	s := &g.state
	star := func(name string) int {
		for i := 0; i < int(s.NumStars); i++ {
			if strings.EqualFold(save.CString(s.Stars[i].Name[:]), name) {
				return i
			}
		}
		return -1
	}
	if j := a.Jobs; j != nil {
		return g.agentJobs(j.Colony, j.Focus)
	}
	switch m := a.Move; {
	case m != nil:
		from, to := star(m.From), star(m.To)
		if from < 0 || to < 0 {
			return "no such star"
		}
		var ids []int16
		for i := 0; i < int(s.NumShips); i++ {
			sh := &s.Ships[i]
			if int(sh.Owner) != int(s.PlayerNum) || sh.Status != save.ShipAtStar || int(sh.Location) != from {
				continue
			}
			t := sh.Design.Type
			switch strings.ToLower(m.Kind) {
			case "scouts":
				if t != 0 || !strings.Contains(strings.ToLower(save.CString(sh.Design.Name[:])), "scout") {
					continue
				}
			case "outposts":
				if t != 4 {
					continue
				}
			case "colony":
				if t != 1 {
					continue
				}
			case "warships":
				if t != 0 || strings.Contains(strings.ToLower(save.CString(sh.Design.Name[:])), "scout") {
					continue
				}
			}
			ids = append(ids, int16(i))
		}
		if len(ids) == 0 {
			return "no such ships there"
		}
		info := g.shipsTryToMoveTo(ids, to)
		if !info.canMove {
			return "out of range"
		}
		g.makeShipsMoveTo(ids, to, &info)
		return fmt.Sprintf("ok: %d ship(s) to %s in %d turn(s)", len(ids), m.To, info.turns)
	}
	return "nothing to do"
}

// agentMainUp reports whether the main screen's own input loop runs: its
// TURN field in the bottom right corner is a field now.
func (g *Game) agentMainUp() bool {
	f := g.ms.fields.next
	if f <= 0 || g.e.FieldsCount() <= f {
		return false
	}
	x1, y1, _, _ := g.e.FieldRect(f)
	return x1 > 500 && y1 > 400
}

// agentCanBuild is what colony ci can build, as its build queue lists it.
func (g *Game) agentCanBuild(ci int) []string {
	bq := &g.bq
	bldg, mil, handle := append([]int16(nil), bq.bldg...), append([]int16(nil), bq.mil...), g.colonyHandle
	defer func() { bq.bldg, bq.mil, g.colonyHandle = bldg, mil, handle }()
	g.colonyHandle = ci
	g.calculateBuildingArray()
	g.calculateMilitaryArray()
	var out []string
	for _, v := range append(append([]int16(nil), bq.bldg...), bq.mil...) {
		if n := g.selectionName(v); n != "" {
			out = append(out, n)
		}
	}
	return out
}

// agentJobs puts colony name's population to its focus (food, industry or
// research) while keeping its food at its population or above where it can
// farm, one colonist at a time as the colonies screen moves them.
func (g *Game) agentJobs(name, focus string) string {
	s := &g.state
	ci := -1
	for i := 0; i < int(s.NumColonies); i++ {
		if int(s.Colonies[i].Owner) == int(s.PlayerNum) && strings.EqualFold(g.planetName(int(s.Colonies[i].Planet)), name) {
			ci = i
		}
	}
	if ci < 0 {
		return "no such colony"
	}
	want := map[string]int{"food": 0, "industry": 1, "research": 2}[strings.ToLower(focus)]
	c := &s.Colonies[ci]
	// move one colonist from job a to job b; false when none can go.
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
	// The empire's food (the panel's), not the colony's: its output counts
	// the food the freighters bring.
	pl := &s.Players[s.PlayerNum]
	fed := func() bool { return pl.FoodSurplus >= 0 }
	moved := 0
	// Food first, from the jobs that are not the focus.
	for i := 0; i < 42 && !fed(); i++ {
		ok := false
		for _, from := range []int{3 - want, want} {
			if from != 0 && from <= 2 && move(from, 0) {
				ok = true
				break
			}
		}
		if !ok {
			break
		}
		moved++
	}
	// Then everyone else to the focus, farmers while food stays enough.
	for _, from := range []int{0, 1, 2} {
		if from == want {
			continue
		}
		for i := 0; i < 42; i++ {
			if !move(from, want) {
				break
			}
			if from == 0 && !fed() {
				move(want, 0)
				break
			}
			moved++
		}
	}
	return fmt.Sprintf("ok: %d moved; farmers %d, workers %d, scientists %d; the empire's food %+d",
		moved, g.sumColonists(ci, 0, -1, -1, 0, -1), g.sumColonists(ci, 1, -1, -1, 0, -1), g.sumColonists(ci, 2, -1, -1, 0, -1), pl.FoodSurplus)
}
