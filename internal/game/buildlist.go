package game

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Build lists (community patch 1.51): keys Q and 0-9 load a text file into
// a colony's build queue. Spec: docs/screens/build-lists.md.

// buildListDir is where the lists live, in the game's directory; a missing
// list is written there from the defaults first.
const buildListDir = "buildlists"

//go:embed buildlists/*.CFG
var buildListDefaults embed.FS

// buildListKeys are the keys and their files.
var buildListKeys = []struct {
	key  byte
	file string
}{
	{'q', "BUILD.CFG"}, {'0', "BUILD0.CFG"}, {'1', "BUILD1.CFG"}, {'2', "BUILD2.CFG"},
	{'3', "BUILD3.CFG"}, {'4', "BUILD4.CFG"}, {'5', "BUILD5.CFG"}, {'6', "BUILD6.CFG"},
	{'7', "BUILD7.CFG"}, {'8', "BUILD8.CFG"}, {'9', "BUILD9.CFG"},
}

// buildListShifted are the keys with Shift on a US keyboard, by list.
var buildListShifted = []byte{'Q', ')', '!', '@', '#', '$', '%', '^', '&', '*', '('}

// Building names in lists, by building id (lowercased English, _ for space).
var buildListBuildings = [0x31]string{
	"", "alien_control_center", "armor_barracks", "artemis_system_net", "astro_university",
	"atmosphere_renewer", "autolab", "automated_factory", "battlestation", "capitol",
	"cloning_center", "colony_base", "deep_core_mine", "core_waste_dump", "dimensional_portal",
	"biospheres", "food_replicators", "gaia_transformation", "currency_exchange", "galactic_cybernet",
	"holo_simulator", "hydroponic_farm", "marine_barracks", "barrier_shield", "flux_shield",
	"gravity_generator", "missile_base", "ground_batteries", "radiation_shield", "stock_exchange",
	"supercomputer", "pleasure_dome", "pollution_processor", "recyclotron", "robotic_factory",
	"research_lab", "robo_miner_plant", "soil_enrichment", "space_academy", "spaceport",
	"star_base", "star_fortress", "stellar_converter", "subterranean_farms", "terraforming",
	"warp_interdictor", "weather_controller", "fighter_garrison", "artificial_planet",
}

// blDesign6 stands for design6 until the sixth design slot exists: read,
// never queued.
const blDesign6 = -1006

// Other products in lists, by name.
var buildListProducts = map[string]int16{
	"colony_ship": bqColonyShip, "outpost_ship": bqOutpost, "transport": bqTransport,
	"design1": -50, "design2": -51, "design3": -52, "design4": -53, "design5": -54, "design6": blDesign6,
	"android_farmer": -4, "android_worker": -5, "android_scientist": -6,
	"spy": -7, "freighter_fleet": -0xf, "trade_goods": -2, "housing": -3, "repeat": -10,
}

type blMode int

const (
	blAppend blMode = iota
	blPrepend
	blMerge
)

// blOp is one word of a list: a mode, a product or a command.
type blOp struct {
	mode    blMode
	product int16 // 0 for a mode or a command
	cmd     string
	isMode  bool
}

var buildListCommands = []string{"buy", "buy10%", "buy50%", "buy75%", "unbuy", "clear", "clear_refits"}

// parseBuildList reads a list's words (# starts a comment, "stop" ends
// it); an unknown word is an error naming it.
func parseBuildList(text string) ([]blOp, error) {
	var ops []blOp
	for line := range strings.SplitSeq(text, "\n") {
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i]
		}
		for _, w := range strings.Fields(line) {
			w = strings.ToLower(w)
			switch {
			case w == "stop":
				return ops, nil
			case w == "append":
				ops = append(ops, blOp{isMode: true, mode: blAppend})
			case w == "prepend":
				ops = append(ops, blOp{isMode: true, mode: blPrepend})
			case w == "merge":
				ops = append(ops, blOp{isMode: true, mode: blMerge})
			case slices.Contains(buildListCommands, w):
				ops = append(ops, blOp{cmd: w})
			default:
				if p, ok := buildListProducts[w]; ok {
					ops = append(ops, blOp{product: p})
				} else if b := slices.Index(buildListBuildings[:], w); b > 0 {
					ops = append(ops, blOp{product: int16(b)})
				} else {
					return nil, fmt.Errorf("%s", w)
				}
			}
		}
	}
	return ops, nil
}

// readBuildList reads list file name from the game's directory, writing
// the default there first when it is missing.
func (g *Game) readBuildList(name string) (string, error) {
	dir := filepath.Join(g.dir, buildListDir)
	if ents, err := os.ReadDir(dir); err == nil {
		for _, e := range ents {
			if strings.EqualFold(e.Name(), name) {
				b, err := os.ReadFile(filepath.Join(dir, e.Name()))
				return string(b), err
			}
		}
	}
	b, err := buildListDefaults.ReadFile("buildlists/" + name)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err == nil {
		if err := os.WriteFile(filepath.Join(dir, name), b, 0o644); err != nil {
			g.Log("build list: %v", err)
		}
	}
	return string(b), nil
}

// blUnit is a queue entry with its repeat mark.
type blUnit struct {
	v   int16
	rep bool
}

func blSlots(u []blUnit) int {
	n := 0
	for _, x := range u {
		n++
		if x.rep {
			n++
		}
	}
	return n
}

// blRun applies a list to the queue being edited (_current_item) of the
// colony _colony_handle names.
type blRun struct {
	g     *Game
	ci    int
	mode  blMode
	group []int16 // the products since the last mode or command
}

func (r *blRun) units() []blUnit {
	var u []blUnit
	for _, v := range r.g.bq.cur {
		switch {
		case v == -1:
		case v == -10 && len(u) > 0:
			u[len(u)-1].rep = true
		case v != -10:
			u = append(u, blUnit{v: v})
		}
	}
	return u
}

func (r *blRun) store(u []blUnit) {
	q := &r.g.bq.cur
	i := 0
	for _, x := range u {
		q[i] = x.v
		i++
		if x.rep {
			q[i] = -10
			i++
		}
	}
	for ; i < 7; i++ {
		q[i] = -1
	}
}

// locked is how many leading units nothing moves: the bought product.
func (r *blRun) locked(u []blUnit) int {
	if r.g.state.Colonies[r.ci].Pending != 0 && len(u) > 0 {
		return 1
	}
	return 0
}

func (r *blRun) isRefit(v int16) bool {
	return v <= -100 && r.g.state.Ships[-(int(v)+100)].Status == 4
}

// drop frees a ship leaving the queue that the colony's own queue does not
// hold (Delete_From_Queue_'s rule); the others go at Do_Exit_Screen_Cleanup_.
func (r *blRun) drop(x blUnit) {
	s := &r.g.state
	if x.v <= -100 && !colonyQueueListHas(&s.Colonies[r.ci], x.v, 0xe) {
		sh := &s.Ships[-(int(x.v) + 100)]
		sh.Status = 5
		sh.Location = -1
	}
}

// canAdd reports whether product p may join the queue now: the build
// screen's lists offer it, a building is not queued yet, androids fit.
func (r *blRun) canAdd(p int16) bool {
	g := r.g
	g.calculateBuildingArray()
	g.calculateMilitaryArray()
	switch {
	case p == blDesign6 || p == -10:
		return false
	case slices.Contains(g.bq.bldg, p):
		return !g.isInQueue(p)
	case !slices.Contains(g.bq.mil, p):
		return false
	case bqIsDesign(p):
		return g.colonyCanBuildProduct(r.ci, p) != 0
	case p == -4 || p == -5 || p == -6:
		s := &g.state
		c := &s.Colonies[r.ci]
		n := int(c.PopCount) + int(g.nProdsInQueue(-6)) + int(g.nProdsInQueue(-4)) + int(g.nProdsInQueue(-5)) + 1
		return n < int(g.planetMaxPopulationForPlayer(int(c.Planet), int(s.PlayerNum)))
	}
	return true
}

// make turns product p into its queue value: support ships and designs
// become ships now (Add_To_Queue_'s way); -1 when no ship is left.
func (r *blRun) make(p int16) int16 {
	g := r.g
	s := &g.state
	switch {
	case p == bqColonyShip || p == bqOutpost || p == bqTransport:
		owner := int(s.Colonies[r.ci].Owner)
		d := &s.Players[owner].Designs[bqRefitSlot]
		switch autoDesignType(p) {
		case 1:
			g.loadColonyShipDesign(owner, d)
		case 4:
			g.loadOutpostShipDesign(owner, d)
		case 2:
			g.loadTransportShipDesign(owner, d)
		}
		return r.ship(bqRefitSlot)
	case bqIsDesign(p):
		return r.ship(int(-(p + 50)))
	}
	return p
}

func (r *blRun) ship(k int) int16 {
	sh := r.g.colbldgCreateShip(k)
	if sh == -1 {
		return -1
	}
	return -(sh + 100)
}

// add puts product p (with a repeat mark) at unit index at, if it may join
// and fits.
func (r *blRun) add(at int, p int16, rep bool) bool {
	if bqIsSetting(p) {
		rep = false
	}
	u := r.units()
	need := 1
	if rep {
		need = 2
	}
	if blSlots(u)+need > 7 || !r.canAdd(p) {
		return false
	}
	v := r.make(p)
	if v == -1 {
		return false
	}
	u = slices.Insert(u, min(at, len(u)), blUnit{v: v, rep: rep})
	r.store(u)
	return true
}

// entries pairs the group's products with their repeat marks.
func (r *blRun) entries() []blUnit {
	var e []blUnit
	for _, p := range r.group {
		if p == -10 {
			if len(e) > 0 {
				e[len(e)-1].rep = true
			}
			continue
		}
		e = append(e, blUnit{v: p})
	}
	return e
}

// flush applies the group in the current mode.
func (r *blRun) flush() {
	e := r.entries()
	r.group = r.group[:0]
	switch r.mode {
	case blAppend:
		for _, x := range e {
			r.add(7, x.v, x.rep)
		}
	case blPrepend:
		at := r.locked(r.units())
		for _, x := range e {
			if r.add(at, x.v, x.rep) {
				at++
			}
		}
	case blMerge:
		r.merge(e)
	}
}

// merge adds the missing listed products, then puts the listed ones into
// the list's order over the slots they hold; ships never count as queued.
func (r *blRun) merge(e []blUnit) {
	u := r.units()
	lock := r.locked(u)
	used := make([]bool, len(u))
	for _, x := range e {
		found := false
		for i := lock; i < len(u); i++ {
			if !used[i] && u[i].v == x.v && x.v > -100 {
				used[i], found = true, true
				if x.rep && !bqIsSetting(x.v) && blSlots(u) < 7 {
					u[i].rep = true
				}
				break
			}
		}
		if found {
			continue
		}
		r.store(u)
		if r.add(7, x.v, x.rep) {
			u = r.units()
			used = append(used, true)
		}
	}
	order := func(v int16) int {
		for k, x := range e {
			if x.v == v {
				return k
			}
		}
		return len(e)
	}
	var pos []int
	var picked []blUnit
	for i := lock; i < len(u); i++ {
		if used[i] {
			pos = append(pos, i)
			picked = append(picked, u[i])
		}
	}
	slices.SortStableFunc(picked, func(a, b blUnit) int { return order(a.v) - order(b.v) })
	for k, i := range pos {
		u[i] = picked[k]
	}
	r.store(u)
}

// listed reports whether v is one of the group's products.
func (r *blRun) listed(v int16) bool { return slices.Contains(r.group, v) }

// command runs c; in merge mode it acts only on the listed products and
// takes the group, so they are not added afterwards.
func (r *blRun) command(c string) {
	merge := r.mode == blMerge
	if !(merge && c == "clear") {
		r.flush()
	}
	g, s := r.g, &r.g.state
	switch c {
	case "clear", "clear_refits":
		u := r.units()
		lock := r.locked(u)
		keep := u[:lock:lock]
		for _, x := range u[lock:] {
			gone := false
			switch {
			case c == "clear_refits":
				gone = r.isRefit(x.v)
			case merge:
				gone = r.listed(x.v)
			default:
				gone = !r.isRefit(x.v)
			}
			if gone {
				r.drop(x)
			} else {
				keep = append(keep, x)
			}
		}
		r.store(keep)
		r.group = r.group[:0]
	case "unbuy":
		if s.Colonies[r.ci].Pending != 0 && (!merge || r.listed(g.bq.cur[0])) {
			g.doExitScreenCleanup()
			g.colonyUnbuy(r.ci)
		}
	default:
		if merge && !r.listed(g.bq.cur[0]) {
			return
		}
		g.doExitScreenCleanup()
		col := &s.Colonies[r.ci]
		if bqIsSetting(col.Queue[0]) || !g.colonyCanBuyProduct0(r.ci) {
			return
		}
		cost := int32(int16(g.colonyProductCost(col, col.Queue[0])))
		var need int32
		switch c {
		case "buy10%":
			need = cost / 10
		case "buy50%":
			need = cost / 2
		case "buy75%":
			need = cost * 3 / 4
		}
		if int32(col.Production) >= need {
			g.colonyBuyOutright(r.ci)
		}
	}
}

// colonyUnbuy (community patch 1.51) refunds a purchase: Production has
// not moved since it, so the price is the same.
func (g *Game) colonyUnbuy(ci int) {
	s := &g.state
	c := &s.Colonies[ci]
	if c.Pending == 0 {
		return
	}
	c.Pending = 0
	s.Players[c.Owner].BC += g.colonyCostToBuyProduct(c)
}

// applyBuildList runs list ops on the queue being edited; the colony
// queue gets it at Do_Exit_Screen_Cleanup_.
func (g *Game) applyBuildList(ops []blOp) {
	r := &blRun{g: g, ci: g.colonyHandle}
	for _, op := range ops {
		switch {
		case op.isMode:
			r.flush()
			r.mode = op.mode
		case op.cmd != "":
			r.command(op.cmd)
		default:
			r.group = append(r.group, op.product)
		}
	}
	r.flush()
	g.calculateBuildingArray()
	g.calculateMilitaryArray()
}

// loadBuildList reads and parses list k; nil when it is unreadable (logged)
// or has a word it does not know (shown: the file and the word).
func (g *Game) loadBuildList(k int) ([]blOp, error) {
	name := buildListKeys[k].file
	text, err := g.readBuildList(name)
	if err != nil {
		g.Log("build list %s: %v", name, err)
		return nil, nil
	}
	ops, err := parseBuildList(text)
	if err != nil {
		return nil, g.messageBox(name + ": " + err.Error())
	}
	return ops, nil
}

// buildListToColony applies ops to colony ci's own queue, off the build
// screen, and recalculates the colony.
func (g *Game) buildListToColony(ci int, ops []blOp) {
	bq := &g.bq
	handle, cur := g.colonyHandle, bq.cur
	bldg, mil := slices.Clone(bq.bldg), slices.Clone(bq.mil)
	g.colonyHandle = ci
	bq.cur = g.state.Colonies[ci].Queue
	g.applyBuildList(ops)
	g.doExitScreenCleanup()
	g.colonyCalculation(ci)
	g.colonyHandle, bq.cur, bq.bldg, bq.mil = handle, cur, bldg, mil
}

// buildListKey is the list a hot key field stands for, and whether Shift
// was down; -1 for none.
func (g *Game) buildListKey(r int, fields, shifted *[11]int) (int, bool) {
	for k := range buildListKeys {
		switch r {
		case fields[k]:
			return k, g.e.LastKey()&0xff == int(buildListShifted[k])
		case shifted[k]:
			return k, true
		}
	}
	return -1, false
}

// addBuildListKeys adds the lists' hot keys, plain and shifted (port-only).
func (g *Game) addBuildListKeys(fields, shifted *[11]int) error {
	for k, l := range buildListKeys {
		var err error
		if fields[k], err = g.e.AddHotKey(string(l.key)); err != nil {
			return err
		}
		if k == 0 {
			shifted[k] = fields[k] // Q and q are one hot key; LastKey tells them apart
			continue
		}
		if shifted[k], err = g.e.AddHotKey(string(buildListShifted[k])); err != nil {
			return err
		}
	}
	return nil
}

func (g *Game) isBuildListField(r int, fields, shifted *[11]int) bool {
	k, _ := g.buildListKey(r, fields, shifted)
	return r > 0 && k >= 0
}

// colsumBuildList loads the list of hot key field r into the colony under
// the pointer, or with Shift into every colony of the list (port-only).
func (g *Game) colsumBuildList(r int) error {
	cs := &g.cs
	k, all := g.buildListKey(r, &cs.fList, &cs.fListShift)
	ops, err := g.loadBuildList(k)
	if err != nil || ops == nil {
		return err
	}
	if all {
		var list [250]int16
		g.buildGlobalColonyList(&list)
		for _, ci := range list {
			if ci == -1 {
				break
			}
			g.buildListToColony(int(ci), ops)
		}
	} else if ci := g.col.colonyN; ci >= 0 && int(g.state.Colonies[ci].Owner) == int(g.state.PlayerNum) {
		g.buildListToColony(int(ci), ops)
	}
	g.e.DeactivateAutoFunction()
	return g.restoreControl()
}
