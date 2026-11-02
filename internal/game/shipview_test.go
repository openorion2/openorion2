package game

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"

	"github.com/openorion2/openorion2/internal/engine"
	"github.com/openorion2/openorion2/internal/gfx"
	"github.com/openorion2/openorion2/internal/save"
)

func TestKenText(t *testing.T) {
	t.Parallel()
	g := sysInfoGame(t, "")
	cases := map[int]string{8: "Combat Bonuses", 0x19: "Weapons", 0x1f: "Green Crew (%d EP)", 0x57: "Agility", 0x5b: "^ %d of %d armor"}
	for n, want := range cases {
		if s, err := g.kenText(n); err != nil || s != want {
			t.Errorf("kenText(%#x) = %q, %v; want %q", n, s, err, want)
		}
	}
	if s, err := g.crewQualityString(2); err != nil || s != "Veteran Crew (200 EP)" {
		t.Errorf("crew 2: %q %v", s, err)
	}
	if s, _ := g.crewQualityString(5); s != "" {
		t.Errorf("crew 5: %q", s)
	}
}

func TestShipViewNames(t *testing.T) {
	t.Parallel()
	g := sysInfoGame(t, "")
	for _, c := range []struct {
		walk int
		want string
	}{
		{walksArmor + 1 + 1, "Titanium"}, {walksShields + 1 + 1, "Class I"},
	} {
		s, err := g.techName(c.walk)
		if err != nil || s == "" {
			t.Errorf("walk %d: %q %v", c.walk, s, err)
		}
		t.Logf("walk %d: %q (expect something like %q)", c.walk, s, c.want)
	}
}

// TestLoadDisplayCombatShip fills the record for every combat ship of a
// save; with strategic combat off it must not touch the save state.
func TestLoadDisplayCombatShip(t *testing.T) {
	t.Parallel()
	g := sysInfoGame(t, "1")
	g.settings[0xd8] = 0
	before := g.state.Ships
	n := 0
	for i := range g.state.Ships {
		s := &g.state.Ships[i]
		if s.Status >= 3 || s.Owner >= 8 || s.Design.Type != 0 || s.Design.Drive == 0 {
			continue
		}
		var v viewShip
		if err := g.loadDisplayCombatShip(&v, i); err != nil {
			t.Fatalf("ship %d: %v", i, err)
		}
		v.defense, v.evasion, v.attack = g.getShipCombatBonuses(i)
		if int(v.structure) != g.shipStructure(i) || v.driveMax <= 0 || v.computerMax <= 0 {
			t.Errorf("ship %d: %+v", i, v)
		}
		if n == 0 {
			t.Logf("ship %d %q: att %d def %d eva %d speed %d drive %d/%d", i, save.CString(v.name[:]), v.attack, v.defense, v.evasion, v.speed, v.driveCur, v.driveMax)
		}
		n++
	}
	if n == 0 {
		t.Skip("no player combat ships in SAVE1.GAM")
	}
	if g.state.Ships != before {
		t.Error("the save state changed")
	}
}

// TestCombatDrawViewShip draws the popup for the first player combat ship;
// OPENORION2_SHIPVIEW_PNG names a file to write the page to.
func TestCombatDrawViewShip(t *testing.T) {
	t.Parallel()
	g := sysInfoGame(t, "1")
	g.settings[0xd8] = 0
	fonts, err := g.archive("FONTS.LBX")
	if err != nil {
		t.Skip(err)
	}
	pal, err := fonts.Entry(6) // palette 5, as main2_ loads it
	if err != nil {
		t.Fatal(err)
	}
	v := g.e.V
	for i := range v.Palette {
		v.Palette[i] = engine.Color{Changed: pal[4*i], R: pal[4*i+1], G: pal[4*i+2], B: pal[4*i+3]}
	}
	ship := -1
	for i := range g.state.Ships {
		s := &g.state.Ships[i]
		if s.Status < 3 && int(s.Owner) == int(g.state.PlayerNum) && s.Design.Type == 0 {
			ship = i
			break
		}
	}
	if ship < 0 {
		t.Skip("no player combat ship")
	}
	sv := &shipView{}
	if err := g.loadDisplayCombatShip(&sv.rec, ship); err != nil {
		t.Fatal(err)
	}
	sv.rec.defense, sv.rec.evasion, sv.rec.attack = g.getShipCombatBonuses(ship)
	p, err := g.cached("COMBAT.LBX", 0xb)
	if err != nil {
		t.Fatal(err)
	}
	if err := g.e.CalculatePictureRemapColors(p); err != nil {
		t.Fatal(err)
	}
	if sv.icon, err = g.picture("FLTICONS.LBX", int(g.state.Players[sv.rec.owner].Race)); err != nil {
		t.Fatal(err)
	}
	if sv.rec.officer != -1 {
		sv.rec.officer = -1
	}
	if err := g.combatDrawViewShip(sv); err != nil {
		t.Fatal(err)
	}
	set := 0
	for _, c := range v.Current {
		if c != 0 {
			set++
		}
	}
	if set < 50000 {
		t.Errorf("only %d pixels drawn", set)
	}
	if out := os.Getenv("OPENORION2_SHIPVIEW_PNG"); out != "" {
		img := image.NewNRGBA(image.Rect(0, 0, engine.W, engine.H))
		for i, c := range v.Current {
			pc := v.Palette[c]
			img.Set(i%engine.W, i/engine.W, color.NRGBA{pc.R << 2, pc.G << 2, pc.B << 2, 0xff})
		}
		f, err := os.Create(out)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, img); err != nil {
			t.Fatal(err)
		}
	}
	_ = gfx.ScreenW
}
