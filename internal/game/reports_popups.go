package game

import (
	"bytes"
	"fmt"

	"github.com/openorion2/openorion2/internal/save"
)

// The popups of the begin-of-turn reports (officers, discoveries, report
// texts). Spec: docs/screens/reports-and-begin-of-turn.md.

// calculateOfficerCost is Calculate_Officer_Cost_ (0x97a29), which falls
// into Officer_Cost_ and ignores whether the officer is hired.
func (g *Game) calculateOfficerCost(p, l int, hire bool) int16 {
	return g.officerCost(p, l)
}

// drawSpecial is Draw_Special_ (0xfd875): the special shown in p's discovery
// popup of star st. Markers (50 and up) carry the player in their last digit;
// 90 and up (Orion) show to everyone.
func (g *Game) drawSpecial(st, p int) int8 {
	sp := int8(g.state.Stars[st].Special)
	if sp >= 0x32 {
		var r int8
		switch {
		case sp >= 0x5a:
			r = 10
		case sp >= 0x50:
			r = 8
		case sp >= 0x46:
			r = 7
		case sp >= 0x3c:
			r = 3
		default:
			r = 2
		}
		if int16(sp%10) == int16(p) || r == 10 {
			return r
		}
		return 0
	}
	switch uint8(sp) {
	case 4, 5, 6, 10, 11:
		return sp
	}
	return 0
}

// sprintfReport is sprintf_report_ (0xfd69f): the text of a report record.
// Type 1 is a theft by p's spies (a technology or a sabotaged building, and
// who was blamed), type 2 one suffered from another player.
func (g *Game) sprintfReport(rec []byte) (string, error) {
	s := &g.state
	rs, err := g.reportStrings()
	if err != nil {
		return "", err
	}
	race := func(p int) string { return save.CString(s.Players[p].RaceName[:]) }
	i16 := func(o int) int { return int(int16(uint16(rec[o]) | uint16(rec[o+1])<<8)) }
	tech := func() (string, error) { return g.techAppName(i16(6)) }
	building := func() (string, error) { return g.buildingName(int(rec[7])) }
	switch rec[0] {
	case 1:
		var out string
		switch rec[3] {
		case 0:
			t, err := tech()
			if err != nil {
				return "", err
			}
			out = cSprintf(rs[0], t, race(int(rec[5])))
		case 1:
			b, err := building()
			if err != nil {
				return "", err
			}
			out = cSprintf(rs[1], race(int(rec[5])), b, g.planetName(int(rec[6])))
		}
		if b := int8(rec[4]); b != -1 {
			out += cSprintf(rs[2], race(int(b)))
		}
		return out, nil
	case 2:
		switch rec[3] {
		case 0:
			t, err := tech()
			if err != nil {
				return "", err
			}
			return cSprintf(rs[3], race(i16(4)), t), nil
		case 1:
			b, err := building()
			if err != nil {
				return "", err
			}
			return cSprintf(rs[4], race(i16(4)), b, g.planetName(int(rec[6]))), nil
		}
		return "", nil
	}
	// The original leaves its buffer as it was: stack garbage.
	g.Log("sprintf_report_: report type %d prints an unset buffer", rec[0])
	return "", nil
}

// Report string indices in RSTRING<lang>.LBX: the loaders Load_Colcalc_,
// Combfind_, Msg_, Netmox_, Netstart_ and Netturn_Strings_ take 60, 10, 80,
// 8, 9 and 2 strings before Load_Report_Strings_ (0xfd5ee) takes 8.
const reportStringsFirst = 60 + 10 + 80 + 8 + 9 + 2

// reportStrings returns the eight report formats in the pointer order
// 0x1ab12c, 0x1ab144, 0x1ab148, 0x1ab13c, 0x1ab130, 0x1ab134, 0x1ab138,
// 0x1ab140.
func (g *Game) reportStrings() ([]string, error) {
	rs, err := g.rStrings()
	if err != nil {
		return nil, err
	}
	if len(rs) < reportStringsFirst+8 {
		return nil, fmt.Errorf("rstring: %d strings", len(rs))
	}
	return rs[reportStringsFirst : reportStringsFirst+8], nil
}

// rStrings splits record 0 of RSTRING<language>.LBX entry 0 (0x1964 bytes,
// loaded to 0x1ab510 at start-up) into its strings.
func (g *Game) rStrings() ([]string, error) {
	if g.rstrings != nil {
		return g.rstrings, nil
	}
	a, err := g.archive(fmt.Sprintf("rstring%d.lbx", g.settings[0xd2]))
	if err != nil {
		return nil, err
	}
	t, err := a.Table(0)
	if err != nil {
		return nil, err
	}
	data := t.Record(0)
	var out []string
	for len(data) > 0 {
		n := bytes.IndexByte(data, 0)
		if n < 0 {
			n = len(data)
		}
		out = append(out, string(data[:n]))
		if n == len(data) {
			break
		}
		data = data[n+1:]
	}
	g.rstrings = out
	return out, nil
}

// planetName is Get_Planet_Name_ (0x77b28): the star's name and the planet's
// numeral, with H message 0xa0 for the first.
func (g *Game) planetName(pl int) string {
	s := &g.state
	n := s.planetNumber(pl)
	roman := cStringAt(g.exe, g.exe.u32(uint32(0x17d5e4+4*n)))
	if roman == "I" {
		roman = g.hMessage(0xa0)
	}
	name := fmt.Sprintf("%s %s", save.CString(s.Stars[s.Planets[pl].Star].Name[:]), roman)
	if len(name) > 0x1e {
		name = name[:0x1e]
	}
	return name
}
