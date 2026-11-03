package game

import (
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openorion2/openorion2/internal/crash"
	"github.com/openorion2/openorion2/internal/host"
	"github.com/openorion2/openorion2/internal/lobby"
	"github.com/openorion2/openorion2/internal/lobby/lobbytest"
	"github.com/openorion2/openorion2/internal/save"
	"github.com/openorion2/openorion2/lobbynet/lobbyv1"
)

func gunzip(t *testing.T, b []byte) []byte {
	t.Helper()
	r, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// TestCrashWrite: a panic in a game leaves the log's tail and the state
// as a save for the report.
func TestCrashWrite(t *testing.T) {
	t.Parallel()
	g, _ := recipeState(t, "start")
	g.dir = t.TempDir()
	g.crash = &crashOffer{}
	g.logTail.Add("before the crash")
	g.crashWrite()
	if b, _ := os.ReadFile(filepath.Join(g.dir, crash.LogFile)); !strings.Contains(string(b), "before the crash") {
		t.Errorf("log tail %q", b)
	}
	b, err := os.ReadFile(filepath.Join(g.dir, crash.SaveFile))
	if err != nil {
		t.Fatal(err)
	}
	f, err := save.Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	if got := f.U32(save.BStardate); got != g.state.Stardate {
		t.Errorf("the save's stardate %d, want %d", got, g.state.Stardate)
	}
}

// crashRun starts the game with a pending report in its directory and
// answers the main menu's question with answer, then Enter to the box
// after it; send stands for the lobby. It returns the reports sent.
func crashRun(t *testing.T, answer byte, sendErr error) (string, []*lobbyv1.CrashReport, int) {
	t.Helper()
	dir := scratchGameDir(t, filepath.Join("..", "..", "game"))
	os.WriteFile(filepath.Join(dir, crash.PendText), []byte("panic: boom\n"), 0o644)
	os.WriteFile(filepath.Join(dir, crash.PendSave), []byte("SAVE"), 0o644)
	var sent []*lobbyv1.CrashReport
	asked, step := 0, 0
	err := host.Run(host.Options{Headless: true, Frames: 60 * 120}, func(sys *host.System) error {
		opt := Options{CrashReports: true}
		opt.testHook = func(g *Game) {
			g.Log = func(string, ...any) {}
			g.crash.send = func(r *lobbyv1.CrashReport) error {
				sent = append(sent, r)
				return sendErr
			}
			g.e.InputHook = func() {
				switch {
				case step == 0 && g.ub.confirming:
					asked++
					sys.PushKey(letterKey(answer))
					step++
				case step == 1 && answer == 'n' && g.currentScreen == ScreenMainMenu && !g.ub.confirming && g.crash.offered:
					step++
					sys.Quit()
				case step == 1 && answer == 'y' && g.ub.textBoxing:
					sys.PushKey(letterKey('\r'))
					step++
				case step == 2 && g.currentScreen == ScreenMainMenu && !g.ub.textBoxing:
					step++
					sys.Quit() // back on the menu, not asked again
				}
			}
		}
		return Run(sys, dir, opt)
	})
	if err != nil && !errors.Is(err, host.ErrQuit) {
		t.Fatalf("the game ended with %v", err)
	}
	return dir, sent, asked
}

// TestCrashOffer: the main menu offers a pending report once; Yes sends it
// (the crash text and the save) and drops it, No drops it unsent, a failed
// send keeps it for the next start.
func TestCrashOffer(t *testing.T) {
	t.Parallel()
	if !fileExists(filepath.Join("..", "..", "game", "FONTS.LBX")) {
		t.Skip("no game files")
	}
	t.Run("yes", func(t *testing.T) {
		t.Parallel()
		dir, sent, asked := crashRun(t, 'y', nil)
		if asked != 1 || len(sent) != 1 {
			t.Fatalf("asked %d times, sent %d", asked, len(sent))
		}
		if r := sent[0]; string(gunzip(t, r.Crash)) != "panic: boom\n" || string(gunzip(t, r.Save)) != "SAVE" || r.Os == "" {
			t.Errorf("report %v", r)
		}
		if crash.Pending(dir) {
			t.Error("still pending after the send")
		}
	})
	t.Run("no", func(t *testing.T) {
		t.Parallel()
		dir, sent, asked := crashRun(t, 'n', nil)
		if asked != 1 || len(sent) != 0 || crash.Pending(dir) {
			t.Errorf("asked %d times, sent %d, pending %v", asked, len(sent), crash.Pending(dir))
		}
	})
	t.Run("failed", func(t *testing.T) {
		t.Parallel()
		dir, sent, _ := crashRun(t, 'y', errors.New("offline"))
		if len(sent) != 1 || !crash.Pending(dir) {
			t.Errorf("sent %d, pending %v", len(sent), crash.Pending(dir))
		}
	})
}

// TestSendCrash: the report goes to the lobby with the saved session; no
// session, no send.
func TestSendCrash(t *testing.T) {
	t.Parallel()
	srv := lobbytest.Start(t)
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "lobby.yaml"), []byte("server: "+srv.Addr+"\n"), 0o644)
	r := &lobbyv1.CrashReport{Build: "v9", Os: "plan9/386", Crash: []byte("x")}
	if err := lobby.SendCrash(dir, srv.TLS, r); !errors.Is(err, lobby.ErrNoSession) {
		t.Fatalf("without a session: %v", err)
	}
	if err := (lobby.Remembered{User: "alice", Session: "alice"}).Save(dir); err != nil {
		t.Fatal(err)
	}
	if err := lobby.SendCrash(dir, srv.TLS, r); err != nil {
		t.Fatal(err)
	}
	if c := srv.Crashes(); len(c) != 1 || c[0].User != "alice" || c[0].Report.Build != "v9" {
		t.Fatalf("the lobby took %v", c)
	}
}
