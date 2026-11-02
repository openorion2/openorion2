package game

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openorion2/openorion2/internal/host"
	"github.com/openorion2/openorion2/internal/netplay"
)

// netBoxRun plays script on one machine from the multiplayer screen with
// the TCP transport and its address boxes; OPENORION2_NET_SHOTS=dir saves the
// screen before each step.
func netBoxRun(t *testing.T, tcp *netplay.TCP, script []lobbyStep) *Game {
	t.Helper()
	dir := scratchGameDir(t, filepath.Join("..", "..", "game"))
	var g *Game
	step, waited := 0, 0
	err := host.Run(host.Options{Headless: true, Frames: 60 * 60}, func(sys *host.System) error {
		opt := Options{NewGameKeys: true, Seed: 7, Net: tcp}
		opt.testHook = func(gg *Game) {
			g = gg
			g.Log = func(string, ...any) {}
			g.settings[settingsMPLink] = 1
			g.settings[0xd5] = 2
			g.currentScreen = ScreenMultiplayer
			// TCP skips the initializing screen (docs/net/ui.md).
			g.e.OnToggle = func() {
				if g.netScr != nil && g.netScr.entry == 0x19 {
					t.Error("the initializing screen shown")
				}
			}
			g.e.InputHook = func() {
				if step == len(script) {
					sys.Quit()
					return
				}
				if !script[step].when(g) {
					waited = 0
					return
				}
				if waited++; waited < 6 {
					return
				}
				waited = 0
				if d := os.Getenv("OPENORION2_NET_SHOTS"); d != "" {
					g.e.SaveScreenPNG(filepath.Join(d, fmt.Sprintf("%s-%d-%s.png", strings.ReplaceAll(t.Name(), "/", "-"), step, script[step].name)))
				}
				script[step].act(g, sys)
				step++
			}
		}
		return Run(sys, dir, opt)
	})
	if err != nil {
		t.Fatal(err)
	}
	if step < len(script) {
		t.Fatalf("stopped before step %q", script[step].name)
	}
	return g
}

func escKeyPress(_ *Game, sys *host.System) {
	sys.PushKey(host.Key{Code: host.KeyEscape, Scancode: 41})
}

// boxUp is the address box with heading h on screen.
func boxUp(h string) func(g *Game) bool {
	return func(g *Game) bool { return g.net.box != nil && g.net.box.heading == h && g.rsel.box.inputField > 0 }
}

// checkBoxScreen: the box is drawn from the original's pictures, not the
// old black placeholder: the multiplayer frame's corner and the popup.
func checkBoxScreen(g *Game, sys *host.System, t *testing.T) {
	v := g.e.V
	if c := v.Screen[0x20*0x280+0x20]; c == 0 {
		t.Error("no multiplayer frame under the address box")
	}
	b := &g.rsel.box
	colors := map[byte]bool{}
	for y := int(b.y); y < int(b.y)+b.picture.Height; y += 3 {
		for x := int(b.x); x < int(b.x)+b.picture.Width; x += 3 {
			colors[v.Screen[y*0x280+x]] = true
		}
	}
	if len(colors) < 8 {
		t.Errorf("the popup has %d colours", len(colors))
	}
}

// listUp is the game list on screen, no box over it.
func listUp(g *Game) bool { return g.net.box == nil && g.netScr != nil && g.netScr.cancelF > 0 }

// stillTyping is the box up with want in its line (being edited).
func stillTyping(t *testing.T, want string) func(g *Game) bool {
	return func(g *Game) bool {
		if g.net.box == nil {
			t.Fatalf("the box left before %q was typed", want)
		}
		return g.e.InputActive() == g.rsel.box.inputField && strings.TrimSuffix(string(g.e.ContinuousString()), "_") == want
	}
}

// freeUDP is a UDP port no relay answers on.
func freeUDP(t *testing.T) int {
	c, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Skip(err)
	}
	defer c.Close()
	return c.LocalAddr().(*net.UDPAddr).Port
}

// checkBoxButtons: ACCEPT and CANCEL inside the popup, symmetric about its
// centre with a gap of plain metal: no pixel of the popup's own ACCEPT left.
func checkBoxButtons(g *Game, t *testing.T) {
	b, p := &g.rsel.box, g.net.box
	ax1, ay1, ax2, ay2 := g.e.FieldRect(b.buttonField)
	cx1, cy1, cx2, cy2 := g.e.FieldRect(p.cancel)
	bx, bw := int(b.x), b.picture.Width
	if ay1 != cy1 || cx1-ax2 < 0x10 || ax1-bx != bx+bw-cx2 || ax1 < bx || cx2 >= bx+bw ||
		ay2 >= int(b.y)+b.picture.Height || cy2 >= int(b.y)+b.picture.Height {
		t.Errorf("ACCEPT %d..%d, CANCEL %d..%d, box %d+%d", ax1, ax2, cx1, cx2, b.x, bw)
	}
	if ax2-ax1 != b.button.Width || cx2-cx1 != b.button.Width {
		t.Errorf("button fields %d, %d wide, art %d", ax2-ax1, cx2-cx1, b.button.Width)
	}
	v, metal := g.e.V, map[byte]bool{}
	for y := ay1 - 1; y <= ay2; y++ {
		for x := bx + nabMetalX; x < ax1-1; x++ {
			metal[v.Screen[y*0x280+x]] = true
		}
	}
	for y := ay1 - 1; y <= ay2; y++ {
		for x := ax2 + 1; x < cx1-1; x++ {
			if c := v.Screen[y*0x280+x]; !metal[c] {
				t.Errorf("colour %#x at %d,%d in the gap is not the popup's metal", c, x, y)
				return
			}
		}
	}
}

// noRefresh holds the list's periodic refresh: only ADDRESS's OK asks again.
const noRefresh = 1 << 30

// listed is the game list showing game.
func listed(game string) func(g *Game) bool {
	return func(g *Game) bool { return listUp(g) && g.netGetGameName(0) == game }
}

// TestNetAddressBoxes: Join goes straight to the game list, whose ADDRESS
// opens the join box; the create box (docs/net/ui.md). Only ACCEPT, Enter,
// CANCEL and Esc leave a box; clicks elsewhere don't.
func TestNetAddressBoxes(t *testing.T) {
	t.Parallel()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skip(err)
	}
	free := ln.Addr().String()
	ln.Close()
	t.Run("join", func(t *testing.T) {
		t.Parallel()
		r, err := netplay.Listen("127.0.0.1:0", "G1")
		if err != nil {
			t.Skip(err)
		}
		defer r.Close()
		addr := r.Addr().String()
		tcp := &netplay.TCP{Discovery: freeUDP(t)}
		var shot bool
		netBoxRun(t, tcp, []lobbyStep{
			{"multiplayer", func(g *Game) bool { return g.mpSetup != nil }, pushKey('j')},
			{"list", func(g *Game) bool { return listUp(g) && g.netScr.addrF > 0 }, func(g *Game, sys *host.System) {
				if g.e.FadePercent() != 0 {
					t.Errorf("the list at fade %d", g.e.FadePercent())
				}
				g.netScr.listRefresh = noRefresh
				clickFieldOf(func(g *Game) int { return g.netScr.addrF })(g, sys)
			}},
			{"esc", boxUp("JOIN NETWORK GAME"), func(g *Game, sys *host.System) {
				checkBoxScreen(g, sys, t)
				checkBoxButtons(g, t)
				shot = true
				escKeyPress(g, sys)
			}},
			{"list again", listUp, pushKey('a')},
			{"type", boxUp("JOIN NETWORK GAME"), func(g *Game, sys *host.System) {
				for _, c := range []byte("abc") {
					sys.PushKey(letterKey(c))
				}
			}},
			// Clicks in the line, right clicks and clicks beside the buttons keep the box.
			{"click line", stillTyping(t, "abc"), clickFieldOf(func(g *Game) int { return g.rsel.box.inputField })},
			{"rclick line", stillTyping(t, "abc"), func(g *Game, sys *host.System) {
				x1, y1, x2, y2 := g.e.FieldRect(g.rsel.box.inputField)
				sys.Press((x1+x2)/2, (y1+y2)/2, 2)
			}},
			{"click box", stillTyping(t, "abc"), func(g *Game, sys *host.System) {
				sys.Press(int(g.rsel.box.x)+10, int(g.rsel.box.y)+10, 1)
			}},
			{"rclick accept", stillTyping(t, "abc"), func(g *Game, sys *host.System) {
				x1, y1, x2, y2 := g.e.FieldRect(g.rsel.box.buttonField)
				sys.Press((x1+x2)/2, (y1+y2)/2, 2)
			}},
			{"rclick cancel", stillTyping(t, "abc"), func(g *Game, sys *host.System) {
				x1, y1, x2, y2 := g.e.FieldRect(g.net.box.cancel)
				sys.Press((x1+x2)/2, (y1+y2)/2, 2)
			}},
			{"cancel", stillTyping(t, "abc"), clickFieldOf(func(g *Game) int { return g.net.box.cancel })},
			{"list kept", listUp, func(g *Game, sys *host.System) {
				if tcp.Addr != "" || g.netScr.listRefresh < noRefresh/2 {
					t.Errorf("CANCEL changed the address to %q or refreshed", tcp.Addr)
				}
				pushKey('a')(g, sys)
			}},
			{"enter", boxUp("JOIN NETWORK GAME"), func(g *Game, sys *host.System) {
				for _, c := range []byte(addr + "\r") {
					sys.PushKey(letterKey(c))
				}
			}},
			// The refresh is held: only Enter's own one can list the relay's game.
			{"listed", listed("G1"), func(g *Game, sys *host.System) {
				g.netScr.listRefresh = noRefresh
				pushKey('a')(g, sys)
			}},
			{"accept", stillTyping(t, addr), clickFieldOf(func(g *Game) int { return g.rsel.box.buttonField })},
			{"refreshed", func(g *Game) bool { return listUp(g) && g.netScr.listRefresh < noRefresh/2 },
				clickFieldOf(func(g *Game) int { return g.netScr.cancelF })},
			{"left", func(g *Game) bool { return g.currentScreen == ScreenMultiplayer && g.mpSetup != nil }, func(*Game, *host.System) {}},
		})
		if !shot {
			t.Fatal("no box seen")
		}
		if tcp.Addr != addr {
			t.Errorf("join address %q", tcp.Addr)
		}
	})
	t.Run("given", func(t *testing.T) {
		// -join or the page's ?join=: the list shows that relay's game at once.
		t.Parallel()
		r, err := netplay.Listen("127.0.0.1:0", "G2")
		if err != nil {
			t.Skip(err)
		}
		defer r.Close()
		tcp := &netplay.TCP{Addr: r.Addr().String(), Discovery: -1}
		netBoxRun(t, tcp, []lobbyStep{
			{"multiplayer", func(g *Game) bool { return g.mpSetup != nil }, pushKey('j')},
			{"listed", listed("G2"), func(*Game, *host.System) {}},
		})
	})
	t.Run("create", func(t *testing.T) {
		t.Parallel()
		var relay *netplay.Relay
		tcp := &netplay.TCP{Listen: free, Started: func(r *netplay.Relay) { relay = r }}
		g := netBoxRun(t, tcp, []lobbyStep{
			{"multiplayer", func(g *Game) bool { return g.mpSetup != nil }, pushKey('s')},
			{"new game", func(g *Game) bool { return g.currentScreen == ScreenNewGame && g.ng.acceptField > 0 },
				clickFieldOf(func(g *Game) int { return g.ng.acceptField })},
			{"cancel", func(g *Game) bool { return g.net.box != nil }, func(g *Game, sys *host.System) {
				checkBoxScreen(g, sys, t)
				checkBoxButtons(g, t)
				clickFieldOf(func(g *Game) int { return g.net.box.cancel })(g, sys)
			}},
			{"multiplayer again", func(g *Game) bool {
				return g.net.box == nil && g.currentScreen == ScreenMultiplayer && g.mpSetup != nil
			}, pushKey('s')},
			{"new game again", func(g *Game) bool { return g.currentScreen == ScreenNewGame && g.ng.acceptField > 0 },
				clickFieldOf(func(g *Game) int { return g.ng.acceptField })},
			{"box", func(g *Game) bool { return g.net.box != nil }, func(g *Game, sys *host.System) {
				sys.PushKey(letterKey('\r'))
			}},
			{"waiting", func(g *Game) bool { return g.net.box == nil && g.net.ep != nil }, func(*Game, *host.System) {}},
		})
		if relay == nil || relay.Addr().String() != free || tcp.Listen != free {
			t.Errorf("relay %v on %q", relay, tcp.Listen)
		}
		if g.net.ep != nil {
			g.netUninit()
		}
		if relay != nil {
			relay.Close()
		}
	})
}
