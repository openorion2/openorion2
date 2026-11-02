package game

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/openorion2/openorion2/internal/host"
	"github.com/openorion2/openorion2/internal/update"
)

// fakeUpdate is a GitHub with release v0.3.0 of this platform's binary
// bin, signed; the client it returns replaces the file exe.
func fakeUpdate(t *testing.T, bin []byte) (*update.Client, string) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	name := update.AssetName(runtime.GOOS, runtime.GOARCH)
	sum := sha256.Sum256(bin)
	sums := []byte(hex.EncodeToString(sum[:]) + "  " + name + "\n")
	sig := base64.StdEncoding.EncodeToString(ed25519.Sign(priv, sums))
	var srv *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/"+update.Repo+"/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		type asset struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		}
		json.NewEncoder(w).Encode(map[string]any{"tag_name": "v0.3.0", "assets": []asset{
			{name, srv.URL + "/bin"}, {update.SumsName, srv.URL + "/sums"}, {update.SigName, srv.URL + "/sig"}}})
	})
	mux.HandleFunc("GET /bin", func(w http.ResponseWriter, r *http.Request) { w.Write(bin) })
	mux.HandleFunc("GET /sums", func(w http.ResponseWriter, r *http.Request) { w.Write(sums) })
	mux.HandleFunc("GET /sig", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(sig)) })
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	exe := filepath.Join(t.TempDir(), "openorion2")
	if err := os.WriteFile(exe, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	return &update.Client{API: srv.URL, Key: pub, Current: "v0.2.0", Exe: exe}, exe
}

// updateRun plays the game from the main menu with c, answering the
// update's question with answer, then Enter to the box after it.
func updateRun(t *testing.T, c *update.Client, answer byte) (error, int) {
	t.Helper()
	dir := scratchGameDir(t, filepath.Join("..", "..", "game"))
	asked, step := 0, 0
	err := host.Run(host.Options{Headless: true, Frames: 60 * 120}, func(sys *host.System) error {
		opt := Options{Update: c}
		opt.testHook = func(g *Game) {
			g.Log = func(string, ...any) {}
			g.e.InputHook = func() {
				switch {
				case step == 0 && g.ub.confirming:
					asked++
					sys.PushKey(letterKey(answer))
					step++
				case step == 1 && answer == 'n' && g.currentScreen == ScreenMainMenu && !g.ub.confirming && g.update.offered:
					step++
					sys.Quit() // back on the menu, not asked again
				case step == 1 && answer == 'y' && g.ub.textBoxing:
					sys.PushKey(letterKey('\r'))
					step++
				}
			}
		}
		return Run(sys, dir, opt)
	})
	return err, asked
}

// TestUpdateInstall: a newer release is offered on the main menu; Yes
// installs it in place of the binary and ends the game for main to start
// the new one.
func TestUpdateInstall(t *testing.T) {
	t.Parallel()
	c, exe := fakeUpdate(t, []byte("new"))
	err, asked := updateRun(t, c, 'y')
	if !errors.Is(err, update.ErrRestart) {
		t.Fatalf("the game ended with %v", err)
	}
	if asked != 1 {
		t.Errorf("asked %d times", asked)
	}
	if b, _ := os.ReadFile(exe); string(b) != "new" {
		t.Errorf("the binary is %q", b)
	}
}

// TestUpdateDeclined: No leaves the binary and the game goes on.
func TestUpdateDeclined(t *testing.T) {
	t.Parallel()
	c, exe := fakeUpdate(t, []byte("new"))
	err, asked := updateRun(t, c, 'n')
	if err != nil && !errors.Is(err, host.ErrQuit) {
		t.Fatalf("the game ended with %v", err)
	}
	if asked != 1 {
		t.Errorf("asked %d times", asked)
	}
	if b, _ := os.ReadFile(exe); string(b) != "old" {
		t.Errorf("the binary is %q", b)
	}
}
