// Command agent plays a running game through its remote control
// (-control, docs/control.md) with a language model deciding what a human
// player would: each turn it reads /state, asks the model for a plan (what
// to build, who works where, where fleets go), carries it out through the
// game's screens and presses TURN; boxes, research choices and battles it
// answers as they come. The model runs locally (Ollama). With -turncmd a
// program decides the turns instead (/act view and orders), with -nollm
// nothing does: either way the screens are answered by fixed rules.
//
//	go run ./tools/agent -name Psilons 127.0.0.1:7100
//	go run ./tools/agent -nollm -turncmd "python3 player.py" 127.0.0.1:7100
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"
)

var (
	modelURL = flag.String("ollama", "http://127.0.0.1:11434", "the Ollama server")
	model    = flag.String("model", "qwen2.5:14b", "the model")
	name     = flag.String("name", "", "the player's name in the log")
	logFile  = flag.String("log", "", "append every action here as JSON lines (to replay a desync)")
	noLLM    = flag.Bool("nollm", false, "no model: only answer the screens and press TURN (with -turncmd, or the empire as it is)")
	turnCmd  = flag.String("turncmd", "", "a command that plays the empire: each turn, before TURN, it gets the view (/act view) as a JSON line on stdin and answers the orders (/act orders) as one")
)

type option struct {
	Label string `json:"label"`
	X, Y  int
}

type state struct {
	Screen   string `json:"screen"`
	Box      string `json:"box"`
	BoxKind  string `json:"box_kind"`
	Stardate string `json:"stardate"`
	Player   *struct {
		Race     string   `json:"race"`
		BC       int      `json:"bc"`
		Research string   `json:"research"`
		Food     int      `json:"food"`
		Designs  []string `json:"designs"`
		Others   []struct {
			Race      string `json:"race"`
			Human     bool   `json:"human"`
			Relations int    `json:"relations"`
			Treaty    string `json:"treaty"`
		} `json:"others"`
	} `json:"player"`
	Colonies []struct {
		Name                        string   `json:"name"`
		Star                        string   `json:"star"`
		Outpost                     bool     `json:"outpost"`
		Pop                         int      `json:"pop"`
		Farmers, Workers, Scientist int      `json:"-"`
		F                           int      `json:"farmers"`
		W                           int      `json:"workers"`
		S                           int      `json:"scientists"`
		Output                      [4]int   `json:"output"`
		Queue                       []string `json:"queue"`
		CanBuild                    []string `json:"can_build"`
	} `json:"colonies"`
	Fleets []struct {
		At    string   `json:"at"`
		To    string   `json:"to"`
		ETA   int      `json:"eta"`
		Ships []string `json:"ships"`
	} `json:"fleets"`
	Stars []struct {
		Name    string `json:"name"`
		X, Y    int
		Owner   string `json:"owner"`
		Planets []struct {
			Name     string `json:"name"`
			Kind     string `json:"kind"`
			Size     int    `json:"size"`
			Climate  int    `json:"climate"`
			Minerals int    `json:"minerals"`
			Owner    string `json:"owner"`
		} `json:"planets"`
	} `json:"stars"`
	Options []option `json:"options"`
}

type agent struct {
	addr     string
	notes    string    // the model's own notes, carried from turn to turn
	events   []string  // what happened since the last plan: results, boxes
	planned  string    // the stardate whose plan was carried out
	turned   time.Time // when TURN was last pressed
	tries    int
	logw     io.Writer
	settled  map[string]int // -nollm: planets tried this stardate
	settleAt string
	turnIn   io.WriteCloser // -turncmd's stdin and stdout
	turnOut  *bufio.Reader
}

// playTurn has -turncmd decide the turn: the view in, the orders out.
func (a *agent) playTurn() {
	if a.turnIn == nil {
		cmd := exec.Command("sh", "-c", *turnCmd)
		cmd.Stderr = os.Stderr
		in, err := cmd.StdinPipe()
		if err != nil {
			log.Fatal(err)
		}
		out, err := cmd.StdoutPipe()
		if err != nil {
			log.Fatal(err)
		}
		if err := cmd.Start(); err != nil {
			log.Fatal(err)
		}
		a.turnIn, a.turnOut = in, bufio.NewReaderSize(out, 1<<20)
	}
	view, err := a.post("/act", nil, []byte(`{"view":{}}`))
	if err != nil || !strings.HasPrefix(strings.TrimSpace(view), "{") {
		a.logf("turn: no view: %v %s", err, firstLine(view))
		return
	}
	if _, err := io.WriteString(a.turnIn, strings.TrimSpace(view)+"\n"); err != nil {
		log.Fatalf("turn command: %v", err)
	}
	orders, err := a.turnOut.ReadString('\n')
	if err != nil {
		log.Fatalf("turn command: %v", err)
	}
	res, _ := a.post("/act", nil, []byte(`{"orders":`+strings.TrimSpace(orders)+`}`))
	a.record("orders", json.RawMessage(strings.TrimSpace(orders)))
	a.logf("turn: %s", strings.TrimSpace(res))
}

func (a *agent) logf(f string, args ...any) { log.Printf("%s: "+f, append([]any{*name}, args...)...) }

func (a *agent) record(kind string, v any) {
	if a.logw == nil {
		return
	}
	b, _ := json.Marshal(map[string]any{"t": time.Now().Format(time.RFC3339), "kind": kind, "v": v})
	a.logw.Write(append(b, '\n'))
}

func (a *agent) state() (*state, error) {
	r, err := http.Get("http://" + a.addr + "/state")
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()
	if r.StatusCode != 200 {
		return nil, fmt.Errorf("state: %s", r.Status)
	}
	var s state
	return &s, json.NewDecoder(r.Body).Decode(&s)
}

func (a *agent) post(path string, q url.Values, body []byte) (string, error) {
	r, err := http.Post("http://"+a.addr+path+"?"+q.Encode(), "application/json", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	defer r.Body.Close()
	b, _ := io.ReadAll(r.Body)
	return strings.TrimSpace(string(b)), nil
}

func (a *agent) key(k string) {
	a.record("key", k)
	a.post("/key", url.Values{"name": {k}}, nil)
	time.Sleep(400 * time.Millisecond)
}

// click clicks the option labelled label on the screen, waiting a moment
// for the screen to show it; on the colonies list it scrolls to it. False
// when the screen has none.
func (a *agent) click(label string) bool {
	for try := 0; try < 12; try++ {
		s, err := a.state()
		if err != nil {
			return false
		}
		for _, o := range s.Options {
			if strings.EqualFold(o.Label, label) {
				a.record("click", label)
				a.post("/click", url.Values{"x": {fmt.Sprint(o.X)}, "y": {fmt.Sprint(o.Y)}}, nil)
				time.Sleep(700 * time.Millisecond)
				return true
			}
		}
		if s.Screen == "colonies" && try >= 3 {
			// Ten rows at a time: scroll down, from the top again after.
			if try == 3 {
				a.key("home")
			} else {
				a.key("pgdn")
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return false
}

// waitScreen waits until the screen is name.
func (a *agent) waitScreen(name string) bool {
	for range 20 {
		if s, err := a.state(); err == nil && s.Screen == name && s.BoxKind == "" {
			return true
		}
		time.Sleep(300 * time.Millisecond)
	}
	return false
}

// ask sends the model a prompt and decodes its JSON answer into v.
func ask(system, prompt string, v any) error { return askSchema(system, prompt, "json", v) }

// askSchema is ask with the answer held to a JSON schema.
func askSchema(system, prompt string, format any, v any) error {
	req := map[string]any{
		"model": *model, "stream": false, "format": format,
		"options":  map[string]any{"num_ctx": 8192, "temperature": 0.6},
		"messages": []map[string]string{{"role": "system", "content": system}, {"role": "user", "content": prompt}},
	}
	b, _ := json.Marshal(req)
	r, err := http.Post(*modelURL+"/api/chat", "application/json", bytes.NewReader(b))
	if err != nil {
		return err
	}
	defer r.Body.Close()
	var out struct {
		Message struct{ Content string } `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&out); err != nil {
		return err
	}
	return json.Unmarshal([]byte(out.Message.Content), v)
}

const playerSystem = `You play Master of Orion II as a human player, against another human and AIs. Think like a good player.

Rules that matter:
- Each population unit is a farmer (food), worker (industry) or scientist (research). Every unit eats 1 food; food below 0 starves colonies. Food output per farmer depends on the planet (terran/ocean best).
- Industry builds the queue's first item; BC (money) can buy it outright.
- Automated Factory: more industry. Research Lab: more research. Hydroponic Farm: +2 food. Biospheres/Housing: more population. Space Academy, Missile Base: defense.
- Outpost Ship claims any free planet (no population, extends fuel range); Colony Ship settles a planet with population (needs a habitable one: climate 2+ and size 1+ is fine).
- Housing only helps a colony far below its population limit; early on factories, labs and expansion are worth far more.
- Expansion early wins the game: keep an Outpost or Colony Ship in some queue until good nearby planets are taken. Scouts explore unexplored stars.
- Ships move only within fuel range of your colonies: a move answered "out of range" means a nearer star.
- Win by out-growing everyone, then build warships and take the rival's colonies.
Keep notes for yourself: your plan for the next turns, what you learned (ranges, failures). You get them back next turn.`

type plan struct {
	Notes    string `json:"notes"`
	Colonies []struct {
		Colony string   `json:"colony"`
		Focus  string   `json:"focus"`
		Build  []string `json:"build"`
	} `json:"colonies"`
	Expand []string `json:"expand"` // stars to settle, best first
	Attack string   `json:"attack"` // a star to send the warships to, or ""
}

// describe is the state for the model, short.
func describe(s *state) string {
	var b strings.Builder
	p := s.Player
	fmt.Fprintf(&b, "Stardate %s. You are the %s. Treasury %d BC. Researching %s. Food surplus %d.\n", s.Stardate, p.Race, p.BC, p.Research, p.Food)
	home := map[string]bool{}
	b.WriteString("Your colonies (food, industry, research, BC output; building queue):\n")
	for _, c := range s.Colonies {
		home[c.Star] = true
		kind := ""
		if c.Outpost {
			kind = " outpost"
		}
		fmt.Fprintf(&b, "- %s%s: pop %d (farmers %d, workers %d, scientists %d), output %v, queue %v; can build: %s\n", c.Name, kind, c.Pop, c.F, c.W, c.S, c.Output, c.Queue, strings.Join(c.CanBuild, ", "))
	}
	fmt.Fprintf(&b, "Your ship designs: %s\n", strings.Join(p.Designs, ", "))
	for _, o := range p.Others {
		who := "AI"
		if o.Human {
			who = "human"
		}
		fmt.Fprintf(&b, "Met the %s (%s): relations %d, %s\n", o.Race, who, o.Relations, o.Treaty)
	}
	b.WriteString("Your fleets:\n")
	for _, f := range s.Fleets {
		if f.At != "" {
			fmt.Fprintf(&b, "- at %s: %s\n", f.At, strings.Join(f.Ships, ", "))
		} else if f.To != "" {
			fmt.Fprintf(&b, "- moving to %s (%d turns): %s\n", f.To, f.ETA, strings.Join(f.Ships, ", "))
		}
	}
	// Stars by distance from the empire.
	type near struct {
		d    float64
		line string
	}
	var homes [][2]int
	for _, st := range s.Stars {
		if home[st.Name] {
			homes = append(homes, [2]int{st.X, st.Y})
		}
	}
	var ns []near
	for _, st := range s.Stars {
		d := math.Inf(1)
		for _, h := range homes {
			d = math.Min(d, math.Hypot(float64(st.X-h[0]), float64(st.Y-h[1])))
		}
		line := st.Name
		if st.Owner != "" {
			line += " (" + st.Owner + ")"
		}
		if len(st.Planets) == 0 {
			line += ": unexplored"
		} else {
			var ps []string
			for _, pl := range st.Planets {
				x := fmt.Sprintf("%s %s size %d climate %d minerals %d", pl.Name, pl.Kind, pl.Size, pl.Climate, pl.Minerals)
				if pl.Owner != "" {
					x += " owned by " + pl.Owner
				}
				ps = append(ps, x)
			}
			line += ": " + strings.Join(ps, "; ")
		}
		ns = append(ns, near{d, fmt.Sprintf("- %s, %.0f away", line, d)})
	}
	sort.Slice(ns, func(i, j int) bool { return ns[i].d < ns[j].d })
	b.WriteString("Stars, nearest first (distance ~10 a turn for slow ships):\n")
	for i, n := range ns {
		if i >= 14 {
			break
		}
		b.WriteString(n.line + "\n")
	}
	return b.String()
}

func enumOr(list []string) map[string]any {
	if len(list) == 0 {
		return map[string]any{"type": "string"}
	}
	return map[string]any{"type": "string", "enum": list}
}

type starInfo struct {
	x, y      int
	explored  bool
	free      int // planets without an owner (asteroids and gas giants too, for outposts)
	habitable int // free planets a colony ship can settle
	owner     string
}

func stars(s *state) map[string]*starInfo {
	m := map[string]*starInfo{}
	for _, st := range s.Stars {
		in := &starInfo{x: st.X, y: st.Y, explored: len(st.Planets) > 0, owner: st.Owner}
		for _, p := range st.Planets {
			if p.Owner == "" {
				in.free++
				if p.Kind == "planet" && p.Climate >= 2 && p.Size >= 1 {
					in.habitable++
				}
			}
		}
		m[st.Name] = in
	}
	return m
}

func dist(a, b *starInfo) float64 { return math.Hypot(float64(a.x-b.x), float64(a.y-b.y)) }

func (a *agent) planTurn(s *state) {
	var colonies, items, expandable, attackable []string
	seenItem := map[string]bool{}
	for _, c := range s.Colonies {
		if c.Outpost {
			continue
		}
		colonies = append(colonies, c.Name)
		for _, it := range c.CanBuild {
			if !seenItem[it] {
				seenItem[it] = true
				items = append(items, it)
			}
		}
	}
	si := stars(s)
	for _, st := range s.Stars {
		in := si[st.Name]
		if in.free > 0 {
			expandable = append(expandable, st.Name)
		}
		if in.owner != "" && in.owner != s.Player.Race {
			attackable = append(attackable, st.Name)
		}
	}
	obj := func(props map[string]any) map[string]any {
		var req []string
		for k := range props {
			req = append(req, k)
		}
		sort.Strings(req)
		return map[string]any{"type": "object", "properties": props, "required": req}
	}
	arr := func(item any, n int) map[string]any {
		return map[string]any{"type": "array", "items": item, "maxItems": n}
	}
	schema := obj(map[string]any{
		"notes":    map[string]any{"type": "string"},
		"colonies": arr(obj(map[string]any{"colony": enumOr(colonies), "focus": enumOr([]string{"food", "industry", "research"}), "build": arr(enumOr(items), 3)}), len(colonies)),
		"expand":   arr(enumOr(expandable), 4),
		"attack":   enumOr(append([]string{""}, attackable...)),
	})
	prompt := describe(s)
	if a.notes != "" {
		prompt += "\nYour notes from last turn:\n" + a.notes + "\n"
	}
	if len(a.events) > 0 {
		prompt += "\nWhat happened since your last plan:\n- " + strings.Join(a.events, "\n- ") + "\n"
	}
	prompt += `
Decide as the empire's ruler; your officers carry it out (they keep food at zero or above, send scouts exploring, and send outpost and colony ships to your expansion targets).
Answer JSON:
- notes: your strategy for the next turns and anything to remember, a few lines.
- colonies: for each colony, focus (food, industry or research; most colonies industry or research, food only where farming is rich) and build: what its queue should hold, 1 to 3 items from its can-build list, most important first (the queue is changed to exactly this).
- expand: stars to settle next, best first (near, with free planets; good planets for colony ships, any for outposts).
- attack: an enemy star to send your warships to, or "" for none.`
	var p plan
	t0 := time.Now()
	if err := askSchema(playerSystem, prompt, schema, &p); err != nil {
		a.logf("plan: %v", err)
		return
	}
	a.events = nil
	a.notes = p.Notes
	a.logf("plan in %.1fs: %s", time.Since(t0).Seconds(), firstLine(strings.ReplaceAll(p.Notes, "\n", " ")))
	a.record("plan", p)
	done := func(f string, args ...any) {
		m := fmt.Sprintf(f, args...)
		a.logf("%s", m)
		a.events = append(a.events, m)
	}

	// Fleets: the officers' part.
	var targets []string
	for _, t := range p.Expand {
		if si[t] != nil && si[t].free > 0 {
			targets = append(targets, t)
		}
	}
	taken := map[string]bool{}
	for _, f := range s.Fleets {
		if f.To != "" {
			taken[f.To] = true
		}
	}
	order := func(from, kind string, pick func(name string, in *starInfo) bool) {
		here := si[from]
		type cand struct {
			name string
			d    float64
		}
		var cs []cand
		for n, in := range si {
			if n != from && pick(n, in) {
				cs = append(cs, cand{n, dist(here, in)})
			}
		}
		sort.Slice(cs, func(i, j int) bool { return cs[i].d < cs[j].d })
		for _, c := range cs {
			res, _ := a.post("/act", nil, mustJSON(map[string]any{"move": map[string]string{"from": from, "to": c.name, "kind": kind}}))
			if strings.HasPrefix(res, "ok") {
				taken[c.name] = true
				done("%s from %s to %s: %s", kind, from, c.name, res)
				return
			}
			if strings.HasPrefix(res, "no such ships") {
				return
			}
		}
	}
	for _, f := range s.Fleets {
		if f.At == "" {
			continue
		}
		has := func(kind string) bool {
			for _, sh := range f.Ships {
				if strings.HasSuffix(sh, "("+kind+")") {
					return true
				}
			}
			return false
		}
		atFree := si[f.At] != nil && si[f.At].free > 0
		if has("scout") {
			order(f.At, "scouts", func(n string, in *starInfo) bool { return !in.explored && !taken[n] })
		}
		ours := func(in *starInfo) bool { return in.owner == "" || in.owner == s.Player.Race }
		if has("outpost") && !atFree {
			// The ruler's targets first, else the nearest free planet.
			order(f.At, "outposts", func(n string, in *starInfo) bool { return slicesContains(targets, n) && in.free > 0 && !taken[n] })
			order(f.At, "outposts", func(n string, in *starInfo) bool { return in.free > 0 && ours(in) && !taken[n] })
		}
		if has("colony") && !(si[f.At] != nil && si[f.At].habitable > 0) {
			order(f.At, "colony", func(n string, in *starInfo) bool { return in.habitable > 0 && !taken[n] && slicesContains(targets, n) })
			order(f.At, "colony", func(n string, in *starInfo) bool { return in.habitable > 0 && ours(in) && !taken[n] })
		}
		if has("warship") && p.Attack != "" && f.At != p.Attack {
			res, _ := a.post("/act", nil, mustJSON(map[string]any{"move": map[string]string{"from": f.At, "to": p.Attack, "kind": "warships"}}))
			done("warships from %s to attack %s: %s", f.At, p.Attack, res)
		}
	}

	// Colonies: jobs by focus, queues to the plan.
	cur := map[string][]string{}
	can := map[string][]string{}
	for _, c := range s.Colonies {
		cur[c.Name], can[c.Name] = c.Queue, c.CanBuild
	}
	var edits []struct {
		colony string
		want   []string
	}
	for _, c := range p.Colonies {
		if !slicesContains(colonies, c.Colony) {
			continue
		}
		res, _ := a.post("/act", nil, mustJSON(map[string]any{"jobs": map[string]string{"colony": c.Colony, "focus": c.Focus}}))
		done("%s focus %s: %s", c.Colony, c.Focus, res)
		var want []string
		for _, it := range c.Build {
			if slicesContains(can[c.Colony], it) && !slicesContains(want, it) {
				want = append(want, it)
			}
		}
		if len(want) > 0 && strings.Join(want, "|") != strings.Join(cur[c.Colony], "|") {
			edits = append(edits, struct {
				colony string
				want   []string
			}{c.Colony, want})
		}
	}
	a.expansionOfficer(s, si, cur, can, &edits)
	if len(edits) == 0 {
		return
	}
	if !a.click("COLONIES") || !a.waitScreen("colonies") {
		return
	}
	for _, e := range edits {
		if !a.click(e.colony) || !a.waitScreen("colony") {
			done("queue at %s: colony not on the list", e.colony)
			continue
		}
		if a.click("CHANGE") && a.waitScreen("build queue") {
			a.setQueue(e.want)
			done("queue at %s: %s", e.colony, strings.Join(e.want, ", "))
			a.click("OK")
		}
		a.waitScreen("colony")
		a.click("RETURN")
		a.waitScreen("colonies")
	}
	a.click("RETURN")
	a.waitScreen("main")
}

// expansionOfficer keeps an Outpost Ship coming while free planets are
// near and none is built, flying or queued, and a Colony Ship while a
// habitable one is free: in the colony with the most industry, first.
func (a *agent) expansionOfficer(s *state, si map[string]*starInfo, cur, can map[string][]string, edits *[]struct {
	colony string
	want   []string
}) {
	have := map[string]bool{}
	for _, f := range s.Fleets {
		if f.At == "" && f.To == "" {
			continue // free ship records (an older /state lists them)
		}
		for _, sh := range f.Ships {
			if strings.HasSuffix(sh, "(outpost)") {
				have["Outpost Ship"] = true
			}
			if strings.HasSuffix(sh, "(colony)") {
				have["Colony Ship"] = true
			}
		}
	}
	for _, q := range cur {
		for _, it := range q {
			have[it] = true
		}
	}
	for _, e := range *edits {
		for _, it := range e.want {
			have[it] = true
		}
	}
	var mine []*starInfo
	for _, c := range s.Colonies {
		if si[c.Star] != nil {
			mine = append(mine, si[c.Star])
		}
	}
	near := func(ok func(*starInfo) bool) bool {
		for _, in := range si {
			if !ok(in) {
				continue
			}
			for _, m := range mine {
				if dist(m, in) < 120 {
					return true
				}
			}
		}
		return false
	}
	var need []string
	if !have["Outpost Ship"] && near(func(in *starInfo) bool { return in.free > 0 && (in.owner == "" || in.owner == s.Player.Race) }) {
		need = append(need, "Outpost Ship")
	}
	if !have["Colony Ship"] && near(func(in *starInfo) bool { return in.habitable > 0 && (in.owner == "" || in.owner == s.Player.Race) }) {
		need = append(need, "Colony Ship")
	}
	a.logf("officer: have %v, need %v", have, need)
	for _, it := range need {
		best, bestInd := "", -1
		for _, c := range s.Colonies {
			if !c.Outpost && slicesContains(can[c.Name], it) && c.Output[1] > bestInd {
				best, bestInd = c.Name, c.Output[1]
			}
		}
		if best == "" {
			continue
		}
		found := false
		for i := range *edits {
			if (*edits)[i].colony == best {
				(*edits)[i].want = append([]string{it}, (*edits)[i].want...)
				found = true
			}
		}
		if !found {
			*edits = append(*edits, struct {
				colony string
				want   []string
			}{best, append([]string{it}, cur[best]...)})
		}
		a.events = append(a.events, "your officers queued an "+it+" at "+best+" for expansion")
	}
}

// setQueue makes the build queue on screen hold want: what is not wanted
// goes (a row is clicked twice), what is missing is added.
func (a *agent) setQueue(want []string) {
	for range 10 {
		s, err := a.state()
		if err != nil {
			return
		}
		var have []string
		var drop string
		for _, o := range s.Options {
			if n, ok := strings.CutPrefix(o.Label, "queue "); ok {
				_, item, _ := strings.Cut(n, ": ")
				have = append(have, item)
				if !slicesContains(want, item) && drop == "" {
					drop = o.Label
				}
			}
		}
		if drop != "" {
			a.click(drop)
			a.click(drop)
			continue
		}
		for _, w := range want {
			if !slicesContains(have, w) {
				a.click(w)
			}
		}
		return
	}
}

func slicesContains(l []string, v string) bool {
	for _, x := range l {
		if x == v {
			return true
		}
	}
	return false
}

func mustJSON(v any) []byte { b, _ := json.Marshal(v); return b }

// step answers whatever is on the screen.
func (a *agent) step() error {
	s, err := a.state()
	if err != nil {
		return err
	}
	switch {
	case s.BoxKind == "confirm" && *noLLM:
		a.logf("box %q: yes", firstLine(s.Box))
		a.key("y")
	case s.BoxKind == "confirm":
		var ans struct{ Answer string }
		ask(playerSystem, "A question comes up in your game:\n"+s.Box+`
Answer JSON {"answer":"yes"} or {"answer":"no"}.`, &ans)
		a.logf("box %q: %s", firstLine(s.Box), ans.Answer)
		a.events = append(a.events, fmt.Sprintf("asked %q, answered %s", firstLine(s.Box), ans.Answer))
		if strings.HasPrefix(strings.ToLower(ans.Answer), "y") {
			a.key("y")
		} else {
			a.key("n")
		}
	case s.BoxKind == "message":
		a.logf("box %q: ok", firstLine(s.Box))
		a.events = append(a.events, "message: "+strings.ReplaceAll(s.Box, "\n", " "))
		a.key("enter")
	case s.Screen == "research choice" && *noLLM:
		for _, o := range s.Options {
			if o.Label != "AUTO" {
				a.logf("research: %s", o.Label)
				a.click(o.Label)
				break
			}
		}
	case s.Screen == "research choice":
		var labels []string
		for _, o := range s.Options {
			labels = append(labels, o.Label)
		}
		var ans struct{ Choice string }
		ask(playerSystem, "Choose your next research, one of:\n"+strings.Join(labels, "\n")+`
Answer JSON {"choice":"<exactly one line from the list>"}.`, &ans)
		a.logf("research: %s", ans.Choice)
		if !a.click(ans.Choice) && len(labels) > 0 {
			a.click(labels[0])
		}
	case s.Screen == "choose planet":
		var labels []string
		for _, o := range s.Options {
			if o.Label != "CLOSE" && !strings.HasSuffix(o.Label, "(taken)") {
				labels = append(labels, o.Label)
			}
		}
		if len(labels) == 0 {
			a.click("CLOSE")
			break
		}
		if *noLLM {
			// The first planet not refused yet ("You cannot build there"),
			// else close.
			if a.settleAt != s.Stardate {
				a.settleAt, a.settled = s.Stardate, map[string]int{}
			}
			pick := ""
			for _, l := range labels {
				if a.settled[l] == 0 {
					pick = l
					break
				}
			}
			if pick == "" {
				a.logf("settle: none left, close")
				a.click("CLOSE")
				break
			}
			a.settled[pick]++
			a.logf("settle %s", pick)
			a.click(pick)
			break
		}
		var ans struct{ Choice string }
		askSchema(playerSystem, describe(s)+"\nYour ship arrived; choose the planet to settle: "+strings.Join(labels, ", ")+`. Answer JSON {"choice":"..."}.`,
			map[string]any{"type": "object", "properties": map[string]any{"choice": enumOr(labels)}, "required": []string{"choice"}}, &ans)
		a.logf("settle %s", ans.Choice)
		a.events = append(a.events, "settled "+ans.Choice)
		if !a.click(ans.Choice) {
			a.click(labels[0])
		}
	case s.Screen == "tactical combat":
		a.click("AUTO")
		time.Sleep(2 * time.Second)
	case s.Screen == "main":
		a.tries = 0
		if s.Stardate != a.planned {
			a.planned = s.Stardate
			switch {
			case *turnCmd != "":
				a.playTurn()
			case !*noLLM:
				a.planTurn(s)
			}
			a.logf("TURN %s", s.Stardate)
			a.click("TURN")
			a.turned = time.Now()
			return nil
		}
		// Still the main screen's own loop long after TURN: a box took the
		// press. (Waiting for the others is the network screen.)
		if time.Since(a.turned) > 8*time.Second {
			a.logf("TURN %s again", s.Stardate)
			a.click("TURN")
			a.turned = time.Now()
		}
		time.Sleep(time.Second)
	case s.Screen == "colonies" || s.Screen == "colony":
		a.click("RETURN")
	case s.Screen == "build queue":
		a.click("CANCEL")
	default:
		// Anything else: Enter, Esc, a click on its first field, in turn.
		switch a.tries++; a.tries % 3 {
		case 1:
			a.key("enter")
		case 2:
			a.key("esc")
		default:
			var fs []struct {
				Rect [4]int `json:"rect"`
			}
			if r, err := http.Get("http://" + a.addr + "/fields"); err == nil {
				json.NewDecoder(r.Body).Decode(&fs)
				r.Body.Close()
			}
			for _, f := range fs {
				x, y := (f.Rect[0]+f.Rect[2])/2, (f.Rect[1]+f.Rect[3])/2
				if x >= 0 && x < 640 && y >= 30 && y < 480 {
					a.record("click", fmt.Sprintf("%d,%d", x, y))
					a.post("/click", url.Values{"x": {fmt.Sprint(x)}, "y": {fmt.Sprint(y)}}, nil)
					break
				}
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return nil
}

func firstLine(s string) string {
	s, _, _ = strings.Cut(s, "\n")
	if len(s) > 60 {
		s = s[:60]
	}
	return s
}

func main() {
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: agent [-name n] host:port")
		os.Exit(2)
	}
	a := &agent{addr: flag.Arg(0)}
	if *name == "" {
		*name = a.addr
	}
	if *logFile != "" {
		f, err := os.OpenFile(*logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			log.Fatal(err)
		}
		a.logw = f
	}
	for fails := 0; ; {
		if err := a.step(); err != nil {
			if fails++; fails > 60 {
				log.Fatal(err)
			}
			time.Sleep(time.Second)
			continue
		}
		fails = 0
	}
}
