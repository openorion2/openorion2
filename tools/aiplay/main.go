// Command aiplay plays an empire with a trained network, for tools/agent
// -turncmd: each line on stdin is the player's view (/act view,
// docs/control.md), each line out the network's orders (/act orders). The
// argument runs the network (serve.py, docs/aiplay.md).
//
//	agent -nollm -turncmd "aiplay 'cd tools/aiplay && python3 serve.py models/rl-survival.pt rqjd 0.5 sample'" 127.0.0.1:7100
package main

import (
	"bufio"
	"fmt"
	"log"
	"os"

	"github.com/openorion2/openorion2/internal/aiplay"
)

func main() {
	log.SetPrefix("aiplay: ")
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: aiplay 'network command'")
		os.Exit(2)
	}
	pc, err := aiplay.Start(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 1<<20), 1<<26)
	out := bufio.NewWriter(os.Stdout)
	var p aiplay.Player
	for in.Scan() {
		g, r, err := p.Turn(in.Bytes())
		var a *aiplay.Action
		if err != nil {
			log.Print(err)
		} else if a, err = pc.Ask(g, r); err != nil {
			log.Print(err)
		}
		if err == nil {
			out.Write(p.Orders(a))
		} else {
			out.WriteString("{}")
		}
		out.WriteString("\n")
		out.Flush()
	}
}
