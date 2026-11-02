package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/openorion2/openorion2/internal/game"
	"github.com/openorion2/openorion2/internal/save"
)

// TestNetNormalize: what differs by design between machines is gone, a
// real difference stays.
func TestNetNormalize(t *testing.T) {
	dir := filepath.Join("..", "..", "game")
	if _, err := os.Stat(filepath.Join(dir, "FONTS.LBX")); err != nil {
		t.Skip(err)
	}
	// A network game of two humans, made from the game files (no one's save).
	b, err := game.NewSave(dir, game.SaveRecipe{Seed: 21, Difficulty: 1, Size: 1, Age: 1, Players: 4, TechLevel: 1, Race: 3, Turns: 20, Network: 2})
	if err != nil {
		t.Fatal(err)
	}
	a, _ := save.Parse(b)
	c, _ := save.Parse(b)
	c.Block(save.BPlayerNum)[0]++
	c.Block(save.BSettings)[0x10]++
	clear(c.Record(save.BPlanets, c.U16(save.BPlanetCount)))
	c.Record(save.BPlanets, c.U16(save.BPlanetCount))[0] = 7
	for _, f := range []*save.File{a, c} {
		if err := netNormalize(f); err != nil {
			t.Fatal(err)
		}
	}
	if d, _ := save.Diff(a, c); len(d) != 0 {
		t.Fatalf("by design: %v", d)
	}
	var p save.Player
	c.Decode(save.BPlayers, 0, &p)
	p.BC++
	c.Encode(save.BPlayers, 0, &p)
	if d, _ := save.Diff(a, c); len(d) != 1 {
		t.Fatalf("a player's BC: %v", d)
	}
}
