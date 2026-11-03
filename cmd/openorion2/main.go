// Command openorion2 is the game: the port of Master of Orion II.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/openorion2/openorion2/internal/announce"
	"github.com/openorion2/openorion2/internal/crash"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/openorion2/openorion2/internal/game"
	"github.com/openorion2/openorion2/internal/host"
	"github.com/openorion2/openorion2/internal/netplay"
	"github.com/openorion2/openorion2/internal/update"
)

func main() {
	dir := flag.String("game", "", "directory with the original game files (default: the program's own directory, the current one, or game/ in either, whichever has them)")
	scale := flag.Int("scale", 0, "initial window size in multiples of 640x480 (0: fit the screen)")
	frames := flag.Int("frames", 0, "quit after this many frames (0: run until closed)")
	shot := flag.String("shot", "", "save the last frame shown to this PNG on exit")
	load := flag.Int("load", 0, "continue SAVE<n>.GAM at start, skipping the main menu (development)")
	wshot := flag.String("wshot", "", "save the last frame as shown in the window to this BMP on exit")
	var clicks clickList
	flag.Var(&clicks, "click", "x,y,frame: a scripted left click at that host frame (repeatable, development)")
	flag.Var(dragList{&clicks}, "drag", "x1,y1,x2,y2,frame: a scripted left-button drag (repeatable, development)")
	flag.Var(rclickList{&clicks}, "rclick", "x,y,frame: a scripted right click at that host frame (repeatable, development)")
	var keys keyList
	flag.Var(&keys, "key", "spec,frame: scripted key presses from that host frame, one per frame; spec is a key (e, f10, esc, enter) or alt+letters (alt+einstein) (repeatable, development)")
	headless := flag.Bool("headless", os.Getenv("OPENORION2_HEADLESS") != "", "run without a window or real input (default from OPENORION2_HEADLESS)")
	mouse := flag.String("mouse", "", "x,y: warp the pointer there at start (development)")
	logos := flag.String("logos", "auto", "the start-up logos and intro: on, off, or auto (on with a window and without -load)")
	autopilot := flag.Bool("autopilot", false, "the human is played by the AI: watch the computer play")
	noCouncil := flag.Bool("autopilot-no-council", false, "-autopilot, and the galactic council is held without its screens: the human votes for himself when nominated, else abstains, and accepts only his own election (development)")
	seed := flag.Uint64("seed", 0, "seed the RNG with this instead of the clock, as the original's command line seed")
	var hostAddr hostFlag
	flag.Var(&hostAddr, "host", "[:port]: a network game created here listens on this TCP port (default :52525); Multiplayer, Network, Start New Game")
	join := flag.String("join", "", "host[:port]: the network game to join; Multiplayer, Network, Join Game (none: the games on the LAN)")
	name := flag.String("name", "", "the player's name in network games")
	netDrop := flag.String("netdrop", "ai", "a network player whose link is lost: ai (the computer takes his empire over and the game goes on) or end (the game ends for everyone)")
	netCheck := flag.Bool("netcheck", false, "network games: compare a hash of the state across the machines after every turn and log a mismatch (debugging)")
	netCheckDir := flag.String("netcheckdir", "", "with -netcheck, write this machine's save of a mismatching turn into this directory")
	control := flag.String("control", "", "addr: a remote control over HTTP on this address, for playing the game from another program (development; docs/control.md)")
	noUpdate := flag.Bool("noupdate", false, "do not look for a newer version at the start")
	noAnnounce := flag.Bool("noannounce", false, "do not fetch the project's announcements (main menu, lobby)")
	announceURL := flag.String("announce", announce.URL, "where the announcements are (development)")
	version := flag.Bool("version", false, "print the version and quit")
	os.Args = append(os.Args[:1], joinHostArg(os.Args[1:])...)
	flag.Parse()
	if *version {
		fmt.Println("openorion2", update.Version)
		return
	}
	update.Cleanup()
	var opt game.Options
	if !*headless && !*noUpdate {
		opt.Update = &update.Client{}
	}
	if (!*headless || *announceURL != announce.URL) && !*noAnnounce {
		opt.Announce = &announce.Feed{URL: *announceURL}
		opt.Announce.Start(context.Background())
	}
	opt.Load = *load
	var relay *netplay.Relay
	tcp := &netplay.TCP{Listen: hostAddr.addr, Addr: *join, Started: func(r *netplay.Relay) { relay = r }}
	webNet(tcp)
	opt.Net = tcp
	if *netDrop != "ai" && *netDrop != "end" {
		fmt.Fprintln(os.Stderr, "openorion2: -netdrop: ai or end")
		os.Exit(2)
	}
	opt.NetDropEnd = *netDrop == "end"
	opt.NetCheck = *netCheck || *netCheckDir != ""
	opt.NetCheckDir = *netCheckDir
	opt.NetName = *name
	opt.NetSkipBoxes = hostAddr.set || *join != ""
	opt.Logos = *logos == "on" || (*logos == "auto" && !*headless && *load == 0)
	opt.Seed = uint32(*seed)
	opt.Autopilot = *autopilot || *noCouncil
	opt.AutopilotNoCouncil = *noCouncil
	if *mouse != "" {
		var m [2]int
		if _, err := fmt.Sscanf(*mouse, "%d,%d", &m[0], &m[1]); err != nil {
			fmt.Fprintln(os.Stderr, "openorion2: -mouse:", err)
			os.Exit(2)
		}
		opt.Mouse = &m
	}

	if *dir == "" {
		*dir = gameDir()
	}
	if runtime.GOOS != "js" && !hasGameFiles(*dir) { // the browser's files come from its server
		msg := "No Master of Orion II files (FONTS.LBX) in " + *dir +
			".\n\nPut OpenOrion2 into the game's directory and start it there, or start it with -game DIR."
		fmt.Fprintln(os.Stderr, "openorion2:", strings.ReplaceAll(msg, "\n\n", " "))
		if !*headless {
			host.ShowError("OpenOrion2", msg)
		}
		os.Exit(1)
	}
	if runtime.GOOS != "js" {
		// port-only: a crash leaves its report in the game's directory; the
		// main menu offers a pending one (docs/lobby/client.md).
		crash.Start(*dir)
		opt.CrashReports = !*headless
	}
	err := host.Run(host.Options{Title: "OpenOrion2", Scale: *scale, Frames: *frames, HideCursor: true, Shot: *shot, WindowShot: *wshot, Clicks: clicks, Keys: keys, Headless: *headless, Control: *control},
		func(sys *host.System) error { return game.Run(sys, *dir, opt) })
	if errors.Is(err, update.ErrRestart) {
		if err := update.Restart(); err != nil {
			fmt.Fprintln(os.Stderr, "openorion2: the new version is installed; start it again:", err)
		}
		return
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "openorion2:", err)
	}
	keepRelay(relay)
	if err != nil {
		os.Exit(1)
	}
}

// keepRelay waits, once the game is over here, while the relay this
// machine started still has stations: the others play on (port-only).
func keepRelay(r *netplay.Relay) {
	if r == nil {
		return
	}
	r.Local().Close()
	if n := r.Members(); n > 0 {
		fmt.Fprintf(os.Stderr, "openorion2: still relaying the network game for %d player(s); Ctrl-C ends it for them\n", n)
	}
	<-r.Done()
}

type clickList []host.Click

func (c *clickList) String() string { return fmt.Sprint(*c) }

func (c *clickList) Set(v string) error {
	var k host.Click
	if _, err := fmt.Sscanf(v, "%d,%d,%d", &k.X, &k.Y, &k.Frame); err != nil {
		return err
	}
	*c = append(*c, k)
	return nil
}

type dragList struct{ c *clickList }

func (d dragList) String() string { return "" }

func (d dragList) Set(v string) error {
	k := host.Click{Drag: true}
	if _, err := fmt.Sscanf(v, "%d,%d,%d,%d,%d", &k.X, &k.Y, &k.ToX, &k.ToY, &k.Frame); err != nil {
		return err
	}
	*d.c = append(*d.c, k)
	return nil
}

type rclickList struct{ c *clickList }

func (r rclickList) String() string { return "" }

func (r rclickList) Set(v string) error {
	k := host.Click{Right: true}
	if _, err := fmt.Sscanf(v, "%d,%d,%d", &k.X, &k.Y, &k.Frame); err != nil {
		return err
	}
	*r.c = append(*r.c, k)
	return nil
}

type keyList []host.ScriptedKey

func (k *keyList) String() string { return fmt.Sprint(*k) }

func (k *keyList) Set(v string) error {
	i := strings.LastIndexByte(v, ',')
	if i < 0 {
		return fmt.Errorf("want spec,frame")
	}
	frame, err := strconv.Atoi(v[i+1:])
	if err != nil {
		return err
	}
	keys, err := host.ParseKeys(v[:i])
	if err != nil {
		return err
	}
	for j, key := range keys {
		*k = append(*k, host.ScriptedKey{Key: key, Frame: frame + j})
	}
	return nil
}

// hostFlag is -host with an optional address: -host, -host=:port.
type hostFlag struct {
	set  bool
	addr string
}

func (h *hostFlag) String() string   { return h.addr }
func (h *hostFlag) IsBoolFlag() bool { return true }

func (h *hostFlag) Set(v string) error {
	h.set = true
	if v != "true" {
		if _, err := strconv.Atoi(v); err == nil {
			v = ":" + v
		}
		h.addr = v
	}
	return nil
}

// joinHostArg turns "-host :port" (two arguments) into "-host=:port".
func joinHostArg(args []string) []string {
	var out []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if (a == "-host" || a == "--host") && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
			out = append(out, a+"="+args[i+1])
			i++
			continue
		}
		out = append(out, a)
	}
	return out
}
