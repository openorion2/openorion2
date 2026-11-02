package game

import (
	"fmt"
	"strings"

	"github.com/openorion2/openorion2/internal/engine"
	"github.com/openorion2/openorion2/internal/font"
	"github.com/openorion2/openorion2/internal/gfx"
	"github.com/openorion2/openorion2/internal/save"
)

// The race statistics page of the info screen (INFO.C). Spec:
// docs/screens/info-screen.md, "Race statistics".

const (
	raceBitmapX       = 0x183e57 // _race_bitmap_x
	raceBitmapY       = 0x183e5f // _race_bitmap_y
	raceStatsLine     = 0x183b24 // the colour of the line under each name
	specialsBullet    = 0x17a2d5 // "^ "
	specialsMinus     = 0x17a2d8 // "-"
	specialsPlus      = 0x17a2da // "+"
	specialsOneBC     = 0x17a2dc // "1 BC"
	specialsHalfBC    = 0x17a2e1 // "0.5 BC"
	specialsMinusHalf = 0x17a2e8 // "-0.5"
	specialsPlusOne   = 0x17a2ed // "+1"
	specialsPlusTwo   = 0x17a2f0 // "+2"
	specialsLevelFmt  = 0x17a2f3 // "%s%+i"
	specialAbilities  = 0x192190 // _special_ability_names
	raceStufLBX       = 0x1789b2 // "RACESTUF.lbx"
)

// raceStatsSubscreen is Race_Stats_Subscreen_ (0x1089c6): four races'
// picks a page, a button to the other four when there are more.
func (g *Game) raceStatsSubscreen(first int) (int, error) {
	e, is, s := g.e, &g.info, &g.state
	elim, err := g.textMessage(billTextLBX, 0xe, 0x28)
	if err != nil {
		return 0, err
	}
	none, err := g.textMessage(billTextLBX, 0xf, 0x28)
	if err != nil {
		return 0, err
	}
	if _, err := g.textMessage(billTextLBX, 0x10, 0x28); err != nil {
		return 0, err
	}
	title, err := g.textMessage(billTextLBX, 7, 0x28)
	if err != nil {
		return 0, err
	}
	t := e.Text()
	if err := t.PrintCentered(0x1a0, 0x1f, title); err != nil {
		return 0, err
	}
	g.setUpPlayerColors()
	g.activePlayer[0] = uint8(s.PlayerNum)
	g.getPlayersDeadOrAliveOrOmniscient(1, true)
	elimBm := gfx.NewBlankBitmap(0xb7, 0x14)
	t.SetStyleShadow(2, g.exeColors(infoRedColors), font.Outline)
	if err := t.CenteredToBitmap(bitmapTarget(elimBm), 0x5b, 2, []byte(elim)); err != nil {
		return 0, err
	}
	var bms [8]*gfx.Picture
	for i := range bms {
		bm := gfx.NewBlankBitmap(0xb7, 0xa4)
		bms[i] = bm
		if i >= int(g.activeCount) {
			t.SetStyle(4, g.exeColors(infoNormalColors))
			if err := t.CenteredToBitmap(bitmapTarget(bm), 0x5b, 0, []byte(none)); err != nil {
				return 0, err
			}
			continue
		}
		p := int(g.activePlayer[i])
		t.SetStyle(4, is.histPal[p])
		name := strings.ToUpper(save.CString(s.Players[p].RaceName[:]))
		if err := t.CenteredToBitmap(bitmapTarget(bm), 0x5b, 0, []byte(name)); err != nil {
			return 0, err
		}
		t.SetStyle(2, is.histPal[p])
		if err := g.printPlayerSpecialsToBitmap(0, 0x16, p, bm); err != nil {
			return 0, err
		}
	}
	if g.activeCount > 4 {
		if _, err := e.AddButtonField(0x181, 0x1ab, "", is.pics[0x14], "", 0x28); err != nil {
			return 0, err
		}
	}
	e.AssignAutoFunction(func() error { return nil }, 2) // Draw_Race_Stats_Subscreen_
	for {
		is.specials = is.specials[:0]
		g.drawDropAreas(infoRaceStatsDrops, infoRaceStatsDrops+4*dropAreaSize)
		idx := 4
		if is.racePage {
			idx = 0
		}
		for k := 0; k < 4; k, idx = k+1, idx+1 {
			x, y := g.exeI16(raceBitmapX+uint32(2*k)), g.exeI16(raceBitmapY+uint32(2*k))
			e.V.Line(int(x+10), int(y+0x11), int(x+0xad), int(y+0x11), g.exeU8(raceStatsLine))
			if err := e.Draw(int(x), int(y), bms[idx]); err != nil {
				return 0, err
			}
			// Slots past _active_count still hold earlier players.
			p := int(g.activePlayer[idx])
			if s.Players[p].Eliminated != 0 {
				if err := e.Draw(int(x), int(y+0xb), elimBm); err != nil {
					return 0, err
				}
			} else if idx < int(g.activeCount) {
				g.raceStatisticsSpecialsHelp(int(x), int(y+0x16), p)
			}
		}
		is.specials = append(is.specials,
			engine.HelpEntry{ID: 0xf2, X1: 0, Y1: 0x28, X2: 0xcd, Y2: 0xc5},
			engine.HelpEntry{ID: 0xf3, X1: 0, Y1: 0xc6, X2: 0xcd, Y2: 0x1df},
			engine.HelpEntry{ID: 0xf8, X1: 0xce, Y1: 0, X2: 0x27f, Y2: 0x1df})
		e.DeactivateHelpList()
		e.SetHelpList(is.specials)
		e.V.CopyOffToBack()
		if err := g.introduceSubscreen(); err != nil {
			return 0, err
		}
		is.showing = infoRaceStats
		var r int
		for {
			in, err := e.GetInput()
			if err != nil {
				return 0, err
			}
			r = int(int16(in))
			if g.fullDrawScreen {
				return 0, nil
			}
			if r >= first {
				is.racePage = !is.racePage
			} else if r == 0 {
				e.MarkTime()
				if err := e.TogglePages(); err != nil {
					return 0, err
				}
				if err := e.ReleaseTime(2); err != nil {
					return 0, err
				}
			}
			if r != 0 {
				break
			}
		}
		if r < first {
			return r, nil
		}
	}
}

// printPlayerSpecialsToBitmap is Print_Player_Specials_To_Bitmap_
// (0x1086d6): player p's government and picks, a line each.
func (g *Game) printPlayerSpecialsToBitmap(x, y, p int, bm *gfx.Picture) error {
	s := &g.state
	if p >= int(s.NumPlayers) {
		return nil
	}
	t := g.e.Text()
	str := func(a uint32) string { return cStringAt(g.exe, a) }
	for k := 0; k < 31; k++ {
		v := s.Players[p].Picks[k]
		if v == 0 && k != 0 {
			continue
		}
		var name string
		var err error
		switch {
		case k == 0:
			name, err = g.eStringAt(governmentLabels + uint32(4*int32(v)))
		case k == 15 && v < 0:
			name, err = g.specialAbilityName(0x1f)
		default:
			name, err = g.specialAbilityName(k)
		}
		if err != nil {
			return err
		}
		line := str(specialsBullet) + name
		switch {
		case k == 0 || k >= 10:
		case k == 2:
			switch {
			case v < 0:
				line += str(specialsMinusHalf)
			case v == 2:
				line += str(specialsPlusOne)
			default:
				line += str(specialsPlusTwo)
			}
		case k == 5:
			if v < 0 {
				line += str(specialsMinus)
			} else {
				line += str(specialsPlus)
			}
			if v == 2 {
				line += str(specialsOneBC)
			} else {
				line += str(specialsHalfBC)
			}
		default:
			line = fmt.Sprintf(cFormat(str(specialsLevelFmt)), line, v)
		}
		if err := t.ToBitmap(bitmapTarget(bm), x, y, []byte(line)); err != nil {
			return err
		}
		y += t.LineFeed()
	}
	return nil
}

// raceStatisticsSpecialsHelp is Race_Statistics_Specials_Help_ (0x1088a0):
// a help entry over each line Print_Player_Specials_To_Bitmap_ printed.
func (g *Game) raceStatisticsSpecialsHelp(x, y, p int) {
	is := &g.info
	for k := 0; k < 31; k++ {
		v := g.state.Players[p].Picks[k]
		if v == 0 && k != 0 {
			continue
		}
		var id int
		switch {
		case k == 0:
			id = int(v)/2 + 0x1da
		case k == 15 && v < 0:
			id = 0x1ca
		case k == 15:
			id = 0x1c9
		case k < 15:
			id = 0x1ba + k
		default:
			id = 0x1bb + k
		}
		is.specials = append(is.specials, engine.HelpEntry{ID: int(int16(id)), X1: x, Y1: y, X2: x + 0xaf, Y2: y + 9})
		y += 0xd
	}
}

// specialAbilityName is _special_ability_names[k] as Load_Player_Special_
// Names_ (0x13652) sets it: the strings of RACESTUF.LBX entry 8+language,
// runs of NULs between them.
func (g *Game) specialAbilityName(k int) (string, error) {
	a, err := g.archive(cStringAt(g.exe, raceStufLBX))
	if err != nil {
		return "", err
	}
	data, err := a.Entry(int(int16(uint16(g.settings[0xd2]) + 8)))
	if err != nil {
		return "", err
	}
	at := func(i int) byte {
		if i < len(data) {
			return data[i]
		}
		return 0
	}
	start, i := 0, 0
	for n := 1; n <= k; n++ {
		for at(i) != 0 {
			i++
		}
		for at(i) == 0 && i < len(data) {
			i++
		}
		start = i
	}
	return save.CString(data[min(start, len(data)):]), nil
}
