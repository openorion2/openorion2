package game

import (
	"strings"

	"github.com/openorion2/openorion2/internal/save"
)

// The command points click (screen 0x2b, Show_Command_Points_Screen_) and
// Compute_Player_Maintenance_'s explanation text. Spec:
// docs/screens/command-points.md.

// Formats of Compute_Player_Maintenance_ in .data.
const (
	cpmCountLine = 0x179fe1 // "%d %s\n"
	cpmColonLine = 0x179fe8 // "%d : %s\n"
	cpmRule      = 0x179ff1 // "-------------------\n"
	cpmNoPenalty = 0x17a006 // "(%s 0 BC)\n"
	cpmPenalty   = 0x17a011 // "(%s %d * %d + %d * %d : %d BC)\n"
)

// Colcalc strings of the command point text (Load_Colcalc_Strings_ order).
const (
	ccCPBase           = 42 // _command_base_msg 0x1aa2e0
	ccCPStarbases      = 43
	ccCPStarbase       = 44
	ccCPBattlestations = 45
	ccCPBattlestation  = 46
	ccCPFortresses     = 47
	ccCPFortress       = 48
	ccCPWarlord        = 49
	ccCPGovernment     = 50
	ccCPTotal          = 51
	ccCPShip           = 52
	ccCPShips          = 53
	ccCPSupportShip    = 54
	ccCPSupportShips   = 55
	ccCPUsed           = 56
	ccCPUsedPlural     = 57
	ccCPPenalty        = 58
	ccCPSummary        = 59 // _command_summary_msg 0x1aa25c, the title
)

// cpmString is Colcalc string i, empty (and logged) when missing.
func (g *Game) cpmString(i int) string {
	v, err := g.colcalc(i)
	if err != nil {
		g.logOnce(err.Error())
	}
	return v
}

// commandPointsSourcesText is the text Compute_Player_Maintenance_ writes
// after the total (0xe223d..0xe23a4): each source of command points.
func (g *Game) commandPointsSourcesText(b *strings.Builder, sb, bs, sf, comm int32, commApp int, best int32, officer int, warlord, half int32, total int16) {
	cc, f := g.cpmString, func(a uint32) string { return cStringAt(g.exe, a) }
	b.WriteString(cSprintf(f(cpmCountLine), 5, cc(ccCPBase)))
	one := func(n, unit int32, single, plural int) {
		switch {
		case n == 1:
			b.WriteString(cSprintf(cc(single), unit))
		case n != 0:
			b.WriteString(cSprintf(cc(plural), n*unit, n))
		}
	}
	one(sb, 1, ccCPStarbase, ccCPStarbases)
	one(bs, 2, ccCPBattlestation, ccCPBattlestations)
	one(sf, 3, ccCPFortress, ccCPFortresses)
	if comm != 0 {
		name, err := g.techAppName(commApp)
		if err != nil {
			g.logOnce(err.Error())
		}
		b.WriteString(cSprintf(f(cpmColonLine), comm, name))
	}
	if best != 0 {
		b.WriteString(cSprintf(f(cpmColonLine), best, save.CString(g.state.Leaders[officer].Name[:])))
	}
	if warlord != 0 {
		b.WriteString(cSprintf(f(cpmCountLine), warlord, cc(ccCPWarlord)))
	}
	if half != 0 {
		b.WriteString(cSprintf(f(cpmCountLine), half, cc(ccCPGovernment)))
	}
	if total > 5 {
		b.WriteString(cSprintf(f(cpmCountLine), int32(total), cc(ccCPTotal)))
	}
	b.WriteString(f(cpmRule))
}

// commandPointsUsedText is the text of the ships using command points
// (0xe2445..0xe2515): combat ships by hull, support ships, the total.
func (g *Game) commandPointsUsedText(b *strings.Builder, bySize *[9]int32, support int32, used int16) {
	cc, f := g.cpmString, func(a uint32) string { return cStringAt(g.exe, a) }
	for size, n := range bySize {
		if n == 0 {
			continue
		}
		plural := n != 1
		name, err := g.hullName(size, plural)
		if err != nil {
			g.logOnce(err.Error())
		}
		if !plural {
			b.WriteString(cSprintf(cc(ccCPShip), int32(size+1), name))
		} else {
			b.WriteString(cSprintf(cc(ccCPShips), n*int32(size+1), n, name))
		}
	}
	switch {
	case support == 1:
		b.WriteString(cSprintf(cc(ccCPSupportShip), support))
	case support != 0:
		b.WriteString(cSprintf(cc(ccCPSupportShips), support, support))
	}
	u := ccCPUsedPlural
	if used == 1 {
		u = ccCPUsed
	}
	b.WriteString(cSprintf(f(cpmCountLine), int32(used), cc(u)))
	b.WriteString(f(cpmRule))
}

// showCommandPoints is Show_Command_Points_ (0xe2644): the explanation in
// a text box titled _command_summary_msg (Show_Morale_'s tail).
func (g *Game) showCommandPoints(p int) error {
	var b strings.Builder
	g.computePlayerMaintenanceText(p, &b)
	title := g.cpmString(ccCPSummary)
	return g.textBox(&title, b.String())
}

// showCommandPointsScreen is Show_Command_Points_Screen_ (0x8bab9).
func (g *Game) showCommandPointsScreen() error {
	e := g.e
	e.DeactivateHelpList()
	e.AssignAutoFunction(g.drawMiniMainScreen, 1)
	e.ClearFields()
	if _, err := e.AddHiddenField(0, 0, 0x27f, 0x1df, "\x1b", 0x29); err != nil {
		return err
	}
	e.DisableDrawAllFields()
	e.V.SetPageOff()
	e.V.Fill(0, 0, 0x27f, 0x1df, 0)
	if err := g.drawMiniMainScreen(); err != nil {
		return err
	}
	if err := e.TogglePages(); err != nil {
		return err
	}
	e.EnableDrawAllFields()
	if err := g.showCommandPoints(int(g.state.PlayerNum)); err != nil {
		return err
	}
	g.currentScreen = g.returnScreen
	return nil
}
