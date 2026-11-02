package game

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// scratchGameDir links the game files into a temporary directory; the files
// the game writes (saves, MOX.SET) are copies, so game/ stays untouched.
func scratchGameDir(t *testing.T, src string) string {
	t.Helper()
	ents, err := os.ReadDir(src)
	if err != nil {
		t.Skip(err)
	}
	dir := t.TempDir()
	for _, e := range ents {
		name := e.Name()
		from, _ := filepath.Abs(filepath.Join(src, name))
		up := strings.ToUpper(name)
		if strings.HasSuffix(up, ".GAM") || up == "MOX.SET" {
			continue
		}
		if up == "LOBBY.YAML" || up == "LOBBY.SESSION" || strings.HasPrefix(up, "SAVE") && strings.HasSuffix(up, ".LBY") {
			continue // the player's lobby settings: a test writes its own, never through a link
		}
		if up == "HOF.M2" { // written on an entry: a copy, never game/'s file
			// The user's games write game/'s HOF.M2; tests start from the
			// fixed copy next to the saves when there is one.
			if fixed := filepath.Join(savesDir(), "HOF.M2"); fileExists(fixed) {
				from = fixed
			}
			copyFile(t, from, filepath.Join(dir, name))
			continue
		}
		if err := os.Symlink(from, filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func copyFile(t *testing.T, from, to string) {
	t.Helper()
	b, err := os.ReadFile(from)
	if err != nil {
		t.Skip(err)
	}
	if err := os.WriteFile(to, b, 0o644); err != nil {
		t.Fatal(err)
	}
}
