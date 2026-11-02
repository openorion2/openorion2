package aiplay

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// Client is the network: serve.py running as a child process (an
// empire's turn per JSON line each way), or rl.py over TCP.
type Client struct {
	in  *json.Encoder
	out *bufio.Scanner
}

// Start runs command (through sh, cmd on Windows) as the policy, or
// connects to tcp://host:port (training's rl.py).
func Start(command string) (*Client, error) {
	if addr, ok := strings.CutPrefix(command, "tcp://"); ok {
		c, err := net.Dial("tcp", addr)
		if err != nil {
			return nil, err
		}
		sc := bufio.NewScanner(c)
		sc.Buffer(make([]byte, 1<<20), 1<<26)
		return &Client{in: json.NewEncoder(c), out: sc}, nil
	}
	cmd := exec.Command("sh", "-c", command)
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/C", command)
	}
	cmd.Stderr = os.Stderr
	w, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	r, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<20), 1<<26)
	return &Client{in: json.NewEncoder(w), out: sc}, nil
}

// pad gives rows the decided columns training data has, as -1.
func pad(rows [][]int, n int) [][]int {
	out := make([][]int, len(rows))
	for i, r := range rows {
		out[i] = append(append([]int(nil), r...), make([]int, n)...)
		for j := len(r); j < len(out[i]); j++ {
			out[i][j] = -1
		}
	}
	return out
}

// Ask gives the policy what empire r.P sees and returns its decisions.
func (p *Client) Ask(g *Game, r *Record) (*Action, error) {
	stars := make([][]int, len(r.Star))
	for i := range stars {
		stars[i] = append(append([]int(nil), g.Stars[i]...), r.Star[i]...)
	}
	planets := make([][]int, len(r.Pn))
	for i := range planets {
		planets[i] = append(append([]int(nil), g.Planets[i]...), r.Pn[i]...)
	}
	me := append(append([]int(nil), r.Me...), -1, -1)
	q := map[string]any{"setup": g.Setup, "me": me, "des": r.Des, "pl": r.Pl, "stars": stars, "planets": planets,
		"co": pad(r.Co, 10), "sh": pad(r.Sh, 1)}
	if err := p.in.Encode(q); err != nil {
		return nil, err
	}
	if !p.out.Scan() {
		return nil, fmt.Errorf("policy: %v", p.out.Err())
	}
	var ans struct {
		Research []int             `json:"rs"`
		Queue    map[string][]int  `json:"cq"`
		Jobs     map[string][2]int `json:"cj"`
		Dest     map[string]int    `json:"sd"`
	}
	if err := json.Unmarshal(p.out.Bytes(), &ans); err != nil {
		return nil, fmt.Errorf("policy: %v: %.200s", err, p.out.Bytes())
	}
	a := &Action{Research: ans.Research, Queue: map[int][]int{}, Jobs: map[int][2]int{}, Dest: map[int]int{}}
	for k, v := range ans.Queue {
		i, _ := strconv.Atoi(k)
		a.Queue[i] = v
	}
	for k, v := range ans.Jobs {
		i, _ := strconv.Atoi(k)
		a.Jobs[i] = v
	}
	for k, v := range ans.Dest {
		i, _ := strconv.Atoi(k)
		a.Dest[i] = v
	}
	return a, nil
}
