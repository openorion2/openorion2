package update

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestNewer(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		a, b string
		want bool
	}{
		{"v0.3.0", "v0.2.9", true}, {"v0.2.10", "v0.2.9", true}, {"v1.0.0", "v0.9.9", true},
		{"v0.2.0", "v0.2.0", false}, {"v0.1.9", "v0.2.0", false},
		{"v0.3.0", "dev", false}, {"0.3.0", "v0.2.0", false}, {"v0.3.0-rc1", "v0.2.0", false},
	} {
		if got := Newer(c.a, c.b); got != c.want {
			t.Errorf("Newer(%q, %q) = %v", c.a, c.b, got)
		}
	}
}

// fakeRelease is a GitHub with one release of a binary for darwin/arm64,
// signed with key; break* spoil the signature or the checksum.
type fakeRelease struct {
	tag       string
	bin       []byte
	key       ed25519.PrivateKey
	breakSig  bool
	breakSum  bool
	noRelease bool
}

func (f *fakeRelease) server(t *testing.T) *httptest.Server {
	name := AssetName("darwin", "arm64")
	sum := sha256.Sum256(f.bin)
	if f.breakSum {
		sum[0] ^= 1
	}
	sums := []byte(hex.EncodeToString(sum[:]) + "  " + name + "\n")
	sig := ed25519.Sign(f.key, sums)
	if f.breakSig {
		sig[0] ^= 1
	}
	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("GET /repos/"+Repo+"/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		if f.noRelease {
			http.NotFound(w, r)
			return
		}
		type asset struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		}
		json.NewEncoder(w).Encode(map[string]any{"tag_name": f.tag, "assets": []asset{
			{name, srv.URL + "/dl/" + name}, {SumsName, srv.URL + "/dl/" + SumsName}, {SigName, srv.URL + "/dl/" + SigName},
		}})
	})
	mux.HandleFunc("GET /dl/"+name, func(w http.ResponseWriter, r *http.Request) { w.Write(f.bin) })
	mux.HandleFunc("GET /dl/"+SumsName, func(w http.ResponseWriter, r *http.Request) { w.Write(sums) })
	mux.HandleFunc("GET /dl/"+SigName, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(base64.StdEncoding.EncodeToString(sig) + "\n"))
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func testClient(t *testing.T, f *fakeRelease, goos string) (*Client, string) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	f.key = priv
	srv := f.server(t)
	exe := filepath.Join(t.TempDir(), "openorion2")
	if err := os.WriteFile(exe, []byte("old binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	return &Client{API: srv.URL, Key: pub, Current: "v0.2.0", Exe: exe, GOOS: goos, GOARCH: "arm64"}, exe
}

// TestInstall: a newer signed release is found, downloaded, checked and
// put in place of the binary, its mode kept.
func TestInstall(t *testing.T) {
	t.Parallel()
	f := &fakeRelease{tag: "v0.3.0", bin: []byte("new binary")}
	c, exe := testClient(t, f, "darwin")
	r, err := c.Latest(context.Background())
	if err != nil || r == nil || r.Version != "v0.3.0" {
		t.Fatalf("latest %v, %v", r, err)
	}
	var done, total int64
	if err := c.Install(context.Background(), r, func(d, n int64) { done, total = d, n }); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(exe); string(b) != "new binary" {
		t.Errorf("the binary is %q", b)
	}
	if st, _ := os.Stat(exe); runtime.GOOS != "windows" && st.Mode().Perm() != 0o755 { // Windows has no execute bits
		t.Errorf("mode %v", st.Mode())
	}
	if done != int64(len("new binary")) || total != done {
		t.Errorf("progress %d of %d", done, total)
	}
}

// TestReplaceWindows: the running .exe is renamed aside, the new one takes
// its name.
func TestReplaceWindows(t *testing.T) {
	t.Parallel()
	exe := filepath.Join(t.TempDir(), "openorion2.exe")
	os.WriteFile(exe, []byte("old"), 0o755)
	c := &Client{Exe: exe, GOOS: "windows"}
	if err := c.replace([]byte("new")); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(exe); string(b) != "new" {
		t.Errorf("the binary is %q", b)
	}
	if b, _ := os.ReadFile(exe + ".old"); string(b) != "old" {
		t.Errorf("the old one is %q", b)
	}
}

// TestInstallRefused: a bad signature or checksum leaves the binary alone.
func TestInstallRefused(t *testing.T) {
	t.Parallel()
	for name, f := range map[string]*fakeRelease{
		"signature": {tag: "v0.3.0", bin: []byte("evil"), breakSig: true},
		"checksum":  {tag: "v0.3.0", bin: []byte("evil"), breakSum: true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c, exe := testClient(t, f, "darwin")
			r, err := c.Latest(context.Background())
			if err != nil || r == nil {
				t.Fatalf("latest %v, %v", r, err)
			}
			if err := c.Install(context.Background(), r, nil); err == nil {
				t.Fatal("installed")
			}
			if b, _ := os.ReadFile(exe); string(b) != "old binary" {
				t.Errorf("the binary is %q", b)
			}
			if _, err := os.Stat(exe + ".new"); err == nil {
				t.Error("a .new file was left")
			}
		})
	}
}

// TestLatestNone: no release, an older one, another platform's, or a dev
// build: nothing to offer, and a dev build does not even ask.
func TestLatestNone(t *testing.T) {
	t.Parallel()
	for name, f := range map[string]*fakeRelease{
		"no release": {noRelease: true},
		"older":      {tag: "v0.1.0", bin: []byte("x")},
		"same":       {tag: "v0.2.0", bin: []byte("x")},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c, _ := testClient(t, f, "darwin")
			if r, err := c.Latest(context.Background()); err != nil || r != nil {
				t.Errorf("latest %v, %v", r, err)
			}
		})
	}
	t.Run("other platform", func(t *testing.T) {
		t.Parallel()
		c, _ := testClient(t, &fakeRelease{tag: "v0.3.0", bin: []byte("x")}, "linux")
		if r, err := c.Latest(context.Background()); err != nil || r != nil {
			t.Errorf("latest %v, %v", r, err)
		}
	})
	t.Run("dev", func(t *testing.T) {
		t.Parallel()
		c := &Client{API: "http://127.0.0.1:1", Current: "dev"}
		if c.Enabled() {
			t.Error("a dev build checks")
		}
		if r, err := c.Latest(context.Background()); err != nil || r != nil {
			t.Errorf("latest %v, %v", r, err)
		}
	})
}
