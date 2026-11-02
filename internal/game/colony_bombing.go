package game

import "strings"

// Colony_Bombing_Screen_ (BOMBING.C, 0xb4d02): what a human sees when an
// enemy bombs one of their colonies. Spec: docs/screens/colony-bombing.md.

const (
	italianDestroyed = 0x179d94 // "Edificio distrutto: %s"
	italianLost      = 0x179dab // "Perdita di:%s"
)

// colonyBombingScreen is Colony_Bombing_Screen_: the summary text box over
// whatever is on screen, then the colony screen's globals for the colony
// and _current_screen = _return_screen, also without a screen.
func (g *Game) colonyBombingScreen(b *bombHits) {
	if g.e != nil {
		if err := g.doSummaryStringPopup(b); err != nil {
			g.Log("Colony_Bombing_Screen_: %v", err)
		}
	}
	g.setColonyGlobals(int(b.Colony))
	g.currentScreen = g.returnScreen
}

// setColonyGlobals is Set_Colony_Globals_ (0xbb16a).
func (g *Game) setColonyGlobals(ci int) {
	s, cs := &g.state, &g.cscr
	g.colonyHandle = ci
	if s.Colonies[ci].Planet < 0 {
		// port-only: a colony the bombing destroyed (its record freed, planet
		// -1): the original reads the planet before the array.
		return
	}
	cs.planet = int(s.Colonies[ci].Planet)
	cs.starHandle = int(s.Planets[cs.planet].Star)
	cs.orbitN = int(s.Planets[cs.planet].Orbit)
}

// doSummaryStringPopup is Do_Summary_String_Popup_ (0xb494b): the losses as
// lines of a Text_Box_ titled E 0x1f2; at most 15 lines before E 0x17a.
// The Italian build's own formats (setting 0xd2 = 4) end without a newline.
func (g *Game) doSummaryStringPopup(b *bombHits) error {
	s := &g.state
	g.setColonyFontTo(4)
	italian := g.settings[0xd2] == 4
	planet := ""
	if pl := int(s.Colonies[b.Colony].Planet); pl >= 0 { // port-only: -1 when the bombing destroyed it
		planet = g.planetName(pl)
	}
	var buf strings.Builder
	buf.WriteString(cSprintf(g.eStr(0x33), planet, g.raceName(int(b.Attacker))))
	lines := int16(1)
	if b.PopKilled != 0 {
		buf.WriteString(cSprintf(g.eStr(0x1d), int32(b.PopKilled)))
		lines = 2
	}
	if b.MarinesKilled != 0 {
		buf.WriteString(cSprintf(g.eStr(0x18), int32(b.MarinesKilled)))
		lines++
	}
	if b.ArmorKilled != 0 {
		buf.WriteString(cSprintf(g.eStr(0x11), int32(b.ArmorKilled)))
		lines++
	}
	if b.Accident != 0 {
		buf.WriteString(cSprintf(g.eStr(0x1b), int32(b.BioPopKilled)))
		lines++
	}
	for i := 0; i < 0x31 && lines < 0xf; i++ {
		if b.Destroyed[i] == 0 {
			continue
		}
		name, err := g.buildingName(i)
		if err != nil {
			return err
		}
		f := g.eStr(0x37)
		if italian {
			f = cStringAt(g.exe, italianDestroyed)
		}
		buf.WriteString(cSprintf(f, name))
		lines++
	}
	if lines > 0xf {
		buf.WriteString(g.eStr(0x17a))
	}
	if b.Emptied != 0 {
		f := g.eStr(0x37)
		if italian {
			f = cStringAt(g.exe, italianLost)
		}
		buf.WriteString(cSprintf(f, planet))
	}
	title := g.eStr(0x1f2)
	return g.textBox(&title, buf.String())
}
