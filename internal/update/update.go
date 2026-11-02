// Package update checks GitHub for a newer release of OpenOrion2 and
// installs it in place of the running binary (port-only).
//
// A release (docs/release.md) carries one binary per platform, named by
// AssetName, plus SHA256SUMS and SHA256SUMS.sig: the checksums signed with
// the release key (ed25519), whose public half is built in. A binary is
// installed only when the signature and its checksum match.
package update

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Version is this build's version, "vX.Y.Z", set at release builds with
// -ldflags "-X github.com/openorion2/openorion2/internal/update.Version=vX.Y.Z".
// Other builds are "dev" and never update.
var Version = "dev"

// Repo is the GitHub repository whose releases are checked.
const Repo = "openorion2/openorion2"

// PublicKey checks the release signatures (tools/release keygen).
var PublicKey = mustKey("0f2cb4d69b3031081d46680d5b67d59284d54556f5b2d87722b95538acb67d73")

// Files of a release besides the binaries.
const (
	SumsName = "SHA256SUMS"
	SigName  = "SHA256SUMS.sig"
)

// ErrRestart is returned up to main once a new binary is installed: main
// then starts it (Restart) in place of this process.
var ErrRestart = errors.New("update: installed, restart")

// Release is a newer release found by Latest.
type Release struct {
	Version string
	assets  map[string]string // name → download URL
}

// Client checks and installs releases. The zero value uses GitHub, the
// built-in key, Version and the running executable.
type Client struct {
	API     string // the GitHub API's base URL
	HTTP    *http.Client
	Key     ed25519.PublicKey
	Current string // this build's version
	Exe     string // the binary to replace
	GOOS    string // the platform whose binary is installed
	GOARCH  string
}

func (c *Client) api() string {
	if c.API != "" {
		return strings.TrimRight(c.API, "/")
	}
	return "https://api.github.com"
}

func (c *Client) http() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 10 * time.Minute}
}

func (c *Client) current() string {
	if c.Current != "" {
		return c.Current
	}
	return Version
}

func (c *Client) key() ed25519.PublicKey {
	if c.Key != nil {
		return c.Key
	}
	return PublicKey
}

func (c *Client) platform() (string, string) {
	goos, goarch := c.GOOS, c.GOARCH
	if goos == "" {
		goos = runtime.GOOS
	}
	if goarch == "" {
		goarch = runtime.GOARCH
	}
	return goos, goarch
}

// Enabled reports whether this build checks for updates at all.
func (c *Client) Enabled() bool {
	_, ok := parseVersion(c.current())
	return ok
}

// AssetName is a platform's binary in a release.
func AssetName(goos, goarch string) string {
	n := "openorion2-" + goos + "-" + goarch
	if goos == "windows" {
		n += ".exe"
	}
	return n
}

// Latest returns the latest release when it is newer than this build and
// has this platform's binary; nil otherwise.
func (c *Client) Latest(ctx context.Context) (*Release, error) {
	if !c.Enabled() {
		return nil, nil
	}
	req, err := http.NewRequestWithContext(ctx, "GET", c.api()+"/repos/"+Repo+"/releases/latest", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "openorion2/"+c.current())
	resp, err := c.http().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil // no release yet
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("update: GitHub answered %s", resp.Status)
	}
	var body struct {
		Tag    string `json:"tag_name"`
		Assets []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&body); err != nil {
		return nil, fmt.Errorf("update: %w", err)
	}
	if !Newer(body.Tag, c.current()) {
		return nil, nil
	}
	r := &Release{Version: body.Tag, assets: map[string]string{}}
	for _, a := range body.Assets {
		r.assets[a.Name] = a.URL
	}
	if r.assets[AssetName(c.platform())] == "" || r.assets[SumsName] == "" || r.assets[SigName] == "" {
		return nil, nil // not for this platform (or not a signed release)
	}
	return r, nil
}

// Install downloads r's binary for this platform, checks it against the
// signed checksums and puts it in place of the executable; progress, if
// set, sees the bytes done of the total (-1 unknown).
func (c *Client) Install(ctx context.Context, r *Release, progress func(done, total int64)) error {
	sums, err := c.get(ctx, r.assets[SumsName], 1<<20, nil)
	if err != nil {
		return err
	}
	sig, err := c.get(ctx, r.assets[SigName], 1<<10, nil)
	if err != nil {
		return err
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(sig)))
	if err != nil || !ed25519.Verify(c.key(), sums, raw) {
		return errors.New("update: the release's signature does not match")
	}
	name := AssetName(c.platform())
	want, ok := parseSums(sums)[name]
	if !ok {
		return fmt.Errorf("update: %s is not in the checksums", name)
	}
	bin, err := c.get(ctx, r.assets[name], 512<<20, progress)
	if err != nil {
		return err
	}
	if got := sha256.Sum256(bin); hex.EncodeToString(got[:]) != want {
		return fmt.Errorf("update: %s does not match its checksum", name)
	}
	return c.replace(bin)
}

// get downloads url, at most max bytes.
func (c *Client) get(ctx context.Context, url string, max int64, progress func(done, total int64)) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "openorion2/"+c.current())
	resp, err := c.http().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("update: %s: %s", filepath.Base(url), resp.Status)
	}
	var buf bytes.Buffer
	total := resp.ContentLength
	chunk := make([]byte, 64<<10)
	for {
		n, err := resp.Body.Read(chunk)
		buf.Write(chunk[:n])
		if int64(buf.Len()) > max {
			return nil, fmt.Errorf("update: %s is too large", filepath.Base(url))
		}
		if progress != nil && n > 0 {
			progress(int64(buf.Len()), total)
		}
		if err == io.EOF {
			return buf.Bytes(), nil
		}
		if err != nil {
			return nil, err
		}
	}
}

// replace puts bin in place of the executable: written next to it, then
// renamed over it (Windows cannot overwrite a running .exe, so the old one
// is renamed aside first and removed by Cleanup at the next start).
func (c *Client) replace(bin []byte) error {
	exe := c.Exe
	if exe == "" {
		var err error
		if exe, err = os.Executable(); err != nil {
			return err
		}
		if exe, err = filepath.EvalSymlinks(exe); err != nil {
			return err
		}
	}
	mode := os.FileMode(0o755)
	if st, err := os.Stat(exe); err == nil {
		mode = st.Mode().Perm()
	}
	next := exe + ".new"
	if err := os.WriteFile(next, bin, mode); err != nil {
		return fmt.Errorf("update: cannot write next to the game (%w); download the new version by hand", err)
	}
	if goos, _ := c.platform(); goos == "windows" {
		old := exe + ".old"
		os.Remove(old)
		if err := os.Rename(exe, old); err != nil {
			os.Remove(next)
			return err
		}
		if err := os.Rename(next, exe); err != nil {
			os.Rename(old, exe)
			return err
		}
		return nil
	}
	if err := os.Rename(next, exe); err != nil {
		os.Remove(next)
		return err
	}
	return nil
}

// Cleanup removes what an earlier update left aside (the old Windows binary).
func Cleanup() {
	if exe, err := os.Executable(); err == nil {
		os.Remove(exe + ".old")
	}
}

// parseSums reads "checksum  name" lines.
func parseSums(b []byte) map[string]string {
	out := map[string]string{}
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) == 2 {
			out[strings.TrimPrefix(f[1], "*")] = strings.ToLower(f[0])
		}
	}
	return out
}

// Newer reports whether version a is newer than b ("vX.Y.Z"; anything else
// is never newer, and nothing is newer than it).
func Newer(a, b string) bool {
	va, ok := parseVersion(a)
	vb, okb := parseVersion(b)
	if !ok || !okb {
		return false
	}
	for i := range va {
		if va[i] != vb[i] {
			return va[i] > vb[i]
		}
	}
	return false
}

func parseVersion(s string) ([3]int, bool) {
	var v [3]int
	s, ok := strings.CutPrefix(s, "v")
	if !ok {
		return v, false
	}
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return v, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return v, false
		}
		v[i] = n
	}
	return v, true
}

func mustKey(h string) ed25519.PublicKey {
	b, err := hex.DecodeString(h)
	if err != nil || len(b) != ed25519.PublicKeySize {
		panic("update: bad public key")
	}
	return b
}
