package game

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/openorion2/openorion2/internal/lbx"
)

// TestArchiveCase: the game names its files in any case ("racestuf.lbx"
// from the executable's strings); on a case-sensitive file system (Linux)
// the file on disk, "RACESTUF.LBX" as GOG installs it, is still found.
func TestArchiveCase(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	b := make([]byte, 12) // an LBX of no entries
	binary.LittleEndian.PutUint16(b[2:], lbx.Magic)
	binary.LittleEndian.PutUint32(b[8:], 12)
	if err := os.WriteFile(filepath.Join(dir, "RACESTUF.LBX"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	g := &Game{dir: dir, lbx: map[string]*lbx.Archive{}}
	for _, name := range []string{"racestuf.lbx", "RACESTUF.lbx", "RaceStuf.LBX"} {
		if _, err := g.archive(name); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}
