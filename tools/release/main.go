// Command release builds and signs a release of OpenOrion2 (docs/release.md).
//
//	go run ./tools/release keygen KEYFILE        a new release key; prints its public half
//	go run ./tools/release build DIR vX.Y.Z      the binaries of every platform into DIR
//	go run ./tools/release sign DIR KEYFILE      SHA256SUMS and SHA256SUMS.sig in DIR
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/openorion2/openorion2/internal/update"
)

// platforms are the release's binaries.
var platforms = [][2]string{
	{"darwin", "arm64"}, {"darwin", "amd64"},
	{"windows", "amd64"}, {"windows", "arm64"},
	{"linux", "amd64"}, {"linux", "arm64"},
}

func main() {
	if len(os.Args) < 3 || os.Args[1] != "keygen" && len(os.Args) < 4 {
		fmt.Fprintln(os.Stderr, "usage: release keygen KEYFILE | build DIR vX.Y.Z | sign DIR KEYFILE")
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "keygen":
		err = keygen(os.Args[2])
	case "build":
		err = build(os.Args[2], os.Args[3])
	case "sign":
		err = sign(os.Args[2], os.Args[3])
	default:
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "release:", err)
		os.Exit(1)
	}
}

// keygen writes a new private key (its seed, hex) to file, readable only
// by its owner, and prints the public key for update.PublicKey.
func keygen(file string) error {
	if _, err := os.Stat(file); err == nil {
		return fmt.Errorf("%s exists: a new key would orphan every installed game", file)
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(file, []byte(hex.EncodeToString(priv.Seed())+"\n"), 0o600); err != nil {
		return err
	}
	fmt.Println(hex.EncodeToString(pub))
	return nil
}

func build(dir, version string) error {
	if !update.Newer(version, "v0.0.0") {
		return fmt.Errorf("version %q: want vX.Y.Z", version)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, p := range platforms {
		out := filepath.Join(dir, update.AssetName(p[0], p[1]))
		ldflags := "-s -w -X github.com/openorion2/openorion2/internal/update.Version=" + version
		if p[0] == "windows" {
			ldflags += " -H windowsgui" // no console window beside the game's
		}
		cmd := exec.Command("go", "build", "-trimpath", "-ldflags", ldflags, "-o", out, "./cmd/openorion2")
		cmd.Env = append(os.Environ(), "GOOS="+p[0], "GOARCH="+p[1], "CGO_ENABLED=0")
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%s/%s: %w", p[0], p[1], err)
		}
		fmt.Println(out)
	}
	return nil
}

// sign writes the checksums of every binary in dir and their signature.
func sign(dir, keyFile string) error {
	b, err := os.ReadFile(keyFile)
	if err != nil {
		return err
	}
	seed, err := hex.DecodeString(strings.TrimSpace(string(b)))
	if err != nil || len(seed) != ed25519.SeedSize {
		return fmt.Errorf("%s is not a release key", keyFile)
	}
	priv := ed25519.NewKeyFromSeed(seed)
	if !priv.Public().(ed25519.PublicKey).Equal(update.PublicKey) {
		return fmt.Errorf("%s is not the key update.PublicKey checks", keyFile)
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var lines []string
	for _, e := range ents {
		if e.IsDir() || e.Name() == update.SumsName || e.Name() == update.SigName {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		lines = append(lines, hex.EncodeToString(sum[:])+"  "+e.Name())
	}
	slices.Sort(lines)
	sums := []byte(strings.Join(lines, "\n") + "\n")
	sig := ed25519.Sign(priv, sums)
	if err := os.WriteFile(filepath.Join(dir, update.SumsName), sums, 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, update.SigName), []byte(base64.StdEncoding.EncodeToString(sig)+"\n"), 0o644)
}
