package game

import (
	"fmt"

	"github.com/openorion2/openorion2/internal/font"
	"github.com/openorion2/openorion2/internal/gfx"
)

// Diplomacy_System_Display_ (DIP-SCRN.C): picking a star on a small galaxy
// map to give or demand. Spec: docs/screens/diplomacy-screen.md.

// diplomacySystem holds its globals.
type diplomacySystem struct {
	starBg     *gfx.Picture // _diplomacy_system_star_background_seg (DIPSTARS.LBX 0)
	giver      int          // _diplomacy_system_giver
	receiver   int          // _diplomacy_system_receiver
	scanFlag   int16        // _diplomacy_scan_input_flag: hovered field, 1000 Cancel, -1
	fields     []int        // _diplomacy_system_fields
	planets    [5]uint8     // _diplomacy_system_planet_list
	planetN    int16        // _diplomacy_system_planet_count
	homeworld  int16        // _diplomacy_system_homeworld_flag
	population int16        // _diplomacy_system_population
}

// diplomacySystemDisplay is Diplomacy_System_Display_ (0x1c479): giver's
// stars on a map; returns the star chosen (one giver may surrender to
// receiver, not a home world) or -1 for Cancel.
func (g *Game) diplomacySystemDisplay(giver, receiver int, demand bool) (int, error) {
	e, s, d := g.e, &g.state, &g.dip
	sy := &d.sys
	saveScreen := g.currentScreen
	g.currentScreen = 6
	gm := &g.ms.galaxyMap
	saveColony := gm.colonyPalette
	gm.colonyPalette = true
	sy.giver, sy.receiver = giver, receiver
	s.StarBgIndex = 0 // _star_bg_index; not restored, so the save keeps it
	help, msg := 0x268, 0x37
	if demand {
		help, msg = 0x261, 0x3f
	}
	g.setupFullScreenDiplomacyHelp(help)
	e.ClearFields()
	e.AssignAutoFunction(g.drawDiplomacySystemDisplay, 2)
	t, err := g.textMessage(jimText2, msg, 0xfa)
	if err != nil {
		return -1, err
	}
	if d.message, err = g.decodeTextMessage(t, receiver, -1, -1, -1, giver, -1, -1, 0xfa); err != nil {
		return -1, err
	}
	result := -1
	sy.scanFlag = -1
	e.ClearFields()
	sy.fields = sy.fields[:0]
	for i := 0; i < int(s.NumStars); i++ {
		x, y := g.galaxyMapStarXY(i, 0xa, 0xab, 0xb4, 0x6e, 0, 0, true)
		f, err := e.AddHiddenField(int(x), int(y), int(x)+5, int(y)+5, g.diploStarKey(i), 0)
		if err != nil {
			return -1, err
		}
		sy.fields = append(sy.fields, f)
	}
	cancelKey := ""
	if g.diploKeys {
		cancelKey = "0"
	}
	cancel, err := e.AddHiddenField(0xf, 0x8b, 0x9d, 0xa2, cancelKey, 0)
	if err != nil {
		return -1, err
	}
	n := len(sy.fields)
	for done := false; !done; {
		e.MarkTime()
		sy.scanFlag = -1
		in, err := e.GetInput()
		if err != nil {
			return -1, err
		}
		k, err := e.ScanInput()
		if err != nil {
			return -1, err
		}
		g.ms.galaxyMapScannedStar = -1
		sy.scanFlag = int16(k)
		switch {
		case k <= 0:
			sy.scanFlag = -1
		case k == cancel:
			sy.scanFlag = 1000
		case n > 0 && k >= sy.fields[0] && k <= sy.fields[n-1] && g.diplomacySystemScan(k-1, giver) == giver:
			st := k - 1
			g.ms.galaxyMapScannedStar = int16(st)
			sy.population = g.getSystemPopulation(giver, st)
			if t, err = g.textMessage(jimText2, 0x38, 0x1e); err != nil {
				return -1, err
			}
			if d.texts.millionColon, err = g.decodeTextMessage(t, -1, -1, -1, int32(sy.population), -1, -1, -1, 0x1e); err != nil {
				return -1, err
			}
			if t, err = g.textMessage(jimText2, 0x3a, 0x23); err != nil {
				return -1, err
			}
			if d.texts.xStarSystem, err = g.decodeTextMessage(t, -1, st, -1, -1, -1, -1, -1, 0x23); err != nil {
				return -1, err
			}
		default:
			sy.scanFlag = -1
		}
		if in == cancel || g.dip.net.escPressed == 1 {
			done = true
		}
		if in != 0 && !done {
			sy.scanFlag = -1
			for j := 0; j < n && !done; j++ {
				if in != sy.fields[j] || g.diplomacySystemScan(in-1, giver) != giver {
					continue
				}
				if s.maySurrenderStar(in-1, giver, receiver) && sy.homeworld == -1 {
					done, result = true, j
				}
			}
		}
		if done {
			break
		}
		if err := g.drawDiplomacySystemDisplay(); err != nil {
			return -1, err
		}
		if err := e.TogglePages(); err != nil {
			return -1, err
		}
		if err := e.ReleaseTime(2); err != nil {
			return -1, err
		}
	}
	e.ClearFields()
	e.DeactivateAutoFunction()
	gm.colonyPalette = saveColony
	g.currentScreen = saveScreen
	return result, nil
}

// diploStarKey is the hot key Options.DiploKeys gives star i's field: the
// letters, then the rest of the keys the tests use.
func (g *Game) diploStarKey(i int) string {
	const keys = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	if !g.diploKeys || i >= len(keys) {
		return ""
	}
	return keys[i : i+1]
}

// diplomacySystemScan is Diplomacy_System_Scan_ (0x1ca7c): owner's planets
// at star st into the planet list, the capitol's planet as home world;
// returns owner, or the star's owner when owner has no colony there.
func (g *Game) diplomacySystemScan(st, owner int) int {
	s, sy := &g.state, &g.dip.sys
	sy.homeworld, sy.planetN = -1, 0
	S := &s.Stars[st]
	if S.ColonyOwners>>uint(owner)&1 == 0 {
		return int(S.Owner)
	}
	for o := 0; o < 5; o++ {
		pl := S.Planets[o]
		if pl == -1 {
			continue
		}
		c := s.Planets[pl].Colony
		if c == -1 || int(s.Colonies[c].Owner) != owner {
			continue
		}
		sy.planets[sy.planetN] = uint8(pl)
		if s.Colonies[c].Buildings[9] != 0 {
			sy.homeworld = pl
		}
		sy.planetN++
	}
	return owner
}

// drawDiplomacySystemDisplay is Draw_Diplomacy_System_Display_ (0x1c7e8),
// the auto function: the ambassador, the map, the title, Cancel (lit while
// hovered) and the hovered star's text.
func (g *Game) drawDiplomacySystemDisplay() error {
	e, d := g.e, &g.dip
	sy := &d.sys
	if err := g.drawDiplomacySynchMode(); err != nil {
		return err
	}
	if err := g.drawDiplomacySystem(); err != nil {
		return err
	}
	t := e.Text()
	t.SetStyleShadow(4, g.exeColors(diplomacyNormalColors), font.ShadowDown)
	if err := t.Print(10, 0x75, d.texts.chooseSystem); err != nil {
		return err
	}
	if sy.scanFlag == 1000 {
		t.SetStyleShadow(4, g.exeColors(diplomacyHighlightColors), font.ShadowDown)
		sy.scanFlag = -1
	}
	if err := t.Print(0x12, 0x8d, d.texts.systemCancel); err != nil {
		return err
	}
	if sy.scanFlag != -1 {
		return g.drawDiplomacySystemText(int(sy.scanFlag-1), sy.homeworld)
	}
	return nil
}

// drawDiplomacySystem is Draw_Diplomacy_System_ (0x1c873): the framed,
// darkened map.
func (g *Game) drawDiplomacySystem() error {
	e := g.e
	e.Box(9, 0xaa, 0xb6, 0x70, 1, 0xfb)
	e.TintFill(10, 0xab, 0xbe, 0x118, 0)
	return g.drawGalaxyMapBox(nil, 0xa, 0xab, 0xb4, 0x6e, 0, 0, 0, 0, 4, 0, 0)
}

// drawDiplomacySystemText is Draw_Diplomacy_System_Text_ (0x1c8d9): the
// star's name (special colours when unsurrenderable or a home world), its
// colonists and planets.
func (g *Game) drawDiplomacySystemText(st int, homeworld int16) error {
	e, s, d := g.e, &g.state, &g.dip
	sy := &d.sys
	t := e.Text()
	special := homeworld != -1
	c := g.exeColors(diplomacyNormalColors)
	if special {
		c = g.exeColors(diplomacySpecialColors)
	}
	t.SetStyleShadow(4, c, font.ShadowDown)
	line := d.texts.xStarSystem
	if !s.maySurrenderStar(st, sy.giver, sy.receiver) {
		t.SetStyleShadow(4, g.exeColors(diplomacySpecialColors), font.ShadowDown)
		special = true
		line += d.texts.unsurrenderabl
	}
	if err := t.Print(10, 0x11f, line); err != nil {
		return err
	}
	c = g.exeColors(diplomacySpecialColors)
	if homeworld == -1 && !special {
		c = g.exeColors(diplomacyNormalColors)
	}
	t.SetStyleShadow(3, c, font.ShadowDown)
	if err := t.Print(10, 0x135, d.texts.millionColon); err != nil {
		return err
	}
	y := 0x143
	for k := 0; k < int(sy.planetN); k++ {
		P := &s.Planets[sy.planets[k]]
		line := fmt.Sprintf(cStringAt(g.exe, 0x178ab0), g.psEString(planetSizeNames, int(P.Size)),
			g.psEString(planetClimateNames, int(P.Climate)))
		if int16(sy.planets[k]) == homeworld {
			line += d.texts.homeworld
		}
		if err := t.Print(10, y, line); err != nil {
			return err
		}
		y += 0xe
	}
	return nil
}
