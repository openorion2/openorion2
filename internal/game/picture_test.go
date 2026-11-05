package game

import (
	"bytes"
	"testing"
)

// TestPictureRecolourLeavesArchive: recolouring a loaded picture (a
// battle's ships for the defender) leaves the archive alone, so the next
// load is the file's picture again. It once changed the archive in memory:
// after a battle as the defender, those ship models kept the defender's
// colours as the attacker of a later battle, until the game started again.
func TestPictureRecolourLeavesArchive(t *testing.T) {
	t.Parallel()
	g, _ := recipeState(t, "start")
	var err error
	if g.exe, err = newExeImage(g.findFile("ORION2.EXE"), t.Logf); err != nil {
		t.Fatal(err)
	}
	file := g.cmbtLBX(cmbtShipsLBX) // the battle's ship pictures
	a, err := g.archive(file)
	if err != nil {
		t.Fatal(err)
	}
	var high [0x20]byte
	for k := range high {
		high[k] = byte(0x60 + k)
	}
	for i := 0; i < a.Len(); i++ {
		e, err := a.Entry(i)
		if err != nil {
			t.Fatal(err)
		}
		before := append([]byte(nil), e...)
		p, err := g.picture(file, i)
		if err != nil {
			continue // not a picture
		}
		p.ReplaceColorRange(high[:], 0x20, 0x20)
		if after, _ := a.Entry(i); !bytes.Equal(after, before) {
			t.Fatalf("recolouring %s entry %d changed the archive", file, i)
		}
	}
}
