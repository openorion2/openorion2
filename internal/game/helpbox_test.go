package game

import (
	"path/filepath"
	"testing"

	"github.com/openorion2/openorion2/internal/lbx"
	"github.com/openorion2/openorion2/internal/save"
)

func TestHelpChain(t *testing.T) {
	t.Parallel()
	a, err := lbx.Open(filepath.Join("..", "..", "game", "HELP.LBX"))
	if err != nil {
		t.Skip("no HELP.LBX in game/")
	}
	tab, err := a.Table(0)
	if err != nil {
		t.Fatal(err)
	}
	recs, err := helpChain(tab, 284)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 || save.CString(recs[0].title) != "Star Date" || recs[0].lbxName != "" {
		t.Errorf("284: %d records, first %q", len(recs), save.CString(recs[0].title))
	}
	recs, err = helpChain(tab, 300)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) < 2 || recs[0].lbxName != "BUFFER0.LBX" || recs[0].entry != 6 || recs[0].next != 301 {
		t.Errorf("300: %d records, %+v", len(recs), recs[0])
	}
	for _, r := range recs[:len(recs)-1] {
		if r.next == 0 {
			t.Errorf("chain continued past next 0")
		}
	}
}

func TestHelpLBXName(t *testing.T) {
	t.Parallel()
	g := &Game{}
	for lang, want := range []string{"HELP.LBX", "GER_HELP.LBX", "FRE_HELP.LBX", "SPA_HELP.LBX", "ITA_HELP.LBX"} {
		g.settings[0xd2] = byte(lang)
		if got := g.helpLBXName(); got != want {
			t.Errorf("language %d: %s, want %s", lang, got, want)
		}
	}
}
