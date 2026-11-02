package game

import (
	"fmt"

	"github.com/openorion2/openorion2/internal/engine"
	"github.com/openorion2/openorion2/internal/save"
)

// What the fleet screen changes: scrapping ships (with the technology a
// captured ship yields) and star relocations. FLT1.C, FLT2.C. Spec:
// docs/screens/fleet-screen.md.

// Tables Get_Technology_From_Scrapped_Ship_ reads applications from.
const (
	fuelApps     = 0x17ffe8 // i16 app, i16 range; 10-byte rows
	scrapDrive   = 0x17fe76 // driveTable + driveApp
	scrapComp    = 0x17fdf2 // computerTable + computerApp
	scrapArmor   = 0x17f63e // 0xf-byte rows
	scrapWeapon  = 0x17f80d // weaponTable + weaponApp
	scrapSpecial = 0x17eee6 // specialTable + specialApp
	scrapSep1    = 0x1792dd
	scrapSep2    = 0x1792e0
	scrapSep3    = 0x1792e3
)

// buildScrapList is Build_Scrap_List_ (0x72ae4): the selected ships.
func (g *Game) buildScrapList() []int16 {
	f := &g.flt
	var l []int16
	for i := int16(0); i < f.nBig; i++ {
		b := &f.big[i]
		if b.ship > -1 && b.ship < maxShips && b.sel {
			l = append(l, b.ship)
		}
	}
	return l
}

// scrapShips is Scrap_Ships_ (0x74afe): with show, every question is asked
// (no colony there, officers aboard, the value); a no cancels all.
// Officers aboard go back to the pool, the ships die; true when done.
func (g *Game) scrapShips(p int, list []int16, show bool) (bool, error) {
	s, m := &g.state, &g.ms
	flag := &m.drawFleetScreenToBack
	if len(list) == 0 {
		// The original reads list[0] of an empty list; SCRAP needs a selection.
		return false, nil
	}
	first := &s.Ships[list[0]]
	star := g.xyToStarID(first.X, first.Y)
	atColony := true
	cancel := func() (bool, error) {
		_, err := g.userBox(flag, g.hMessage(0x7f), boxWarning)
		return false, err
	}
	if star == -1 || !s.playerHasColony(p, star) {
		atColony = false
		if show {
			race := save.CString(s.Players[p].RaceName[:])
			id := 0x7c
			if len(list) == 1 {
				id = 0x7a
			}
			if race != "" && isVowel(race[0]) {
				id++
			}
			r, err := g.userBox(flag, fmt.Sprintf(cFormat(g.hMessage(id)), race), boxConfirm)
			if err != nil {
				return false, err
			}
			if r == 0 {
				return cancel()
			}
		}
	}
	officers := false
	for _, k := range list {
		sh := &s.Ships[k]
		if sh.Officer <= -1 {
			continue
		}
		if show {
			msg := fmt.Sprintf(cFormat(g.hMessage(0x7e)), save.CString(sh.Design.Name[:]), g.leaderName(int(sh.Officer), true))
			r, err := g.userBox(flag, msg, boxConfirm)
			if err != nil {
				return false, err
			}
			if r == 0 {
				return cancel()
			}
		}
		officers = true
	}
	value := g.scrapShipValue()
	if show && value > 0 {
		id := 0x81
		if len(list) == 1 {
			id = 0x80
		}
		r, err := g.userBox(flag, fmt.Sprintf(cFormat(g.hMessage(id)), int(value)), boxConfirm)
		if err != nil {
			return false, err
		}
		if r == 0 {
			return cancel()
		}
		if atColony {
			s.Players[p].BC += int32(value)
			if err := g.plunderShipsForTech(list, show); err != nil {
				return false, err
			}
		}
	}
	if officers {
		for _, k := range list {
			sh := &s.Ships[k]
			if sh.Officer > -1 {
				l := &s.Leaders[sh.Officer]
				l.Status, l.Location, l.ETA = 0, -1, 0
				sh.Officer = -1
			}
		}
	}
	for _, k := range list {
		g.killShip(int(k))
	}
	g.doPlayerColonyCalculations(p)
	return true, nil
}

// scrapShipValue is Get_Scrap_Ship_Value_ (0x72e61): the scrap value of the
// selected big icons, cut to 16 bits by the caller.
func (g *Game) scrapShipValue() int16 {
	f := &g.flt
	v := 0
	for i := int16(0); i < f.nBig; i++ {
		if f.big[i].sel {
			v += g.shipScrapValue(int(f.big[i].ship))
		}
	}
	return int16(v)
}

// shipScrapValue is Ship_Scrap_Value_ (0xeddd8): a quarter of the cost at
// an own colony.
func (g *Game) shipScrapValue(ship int) int {
	if !g.shipIsAtColony(ship) {
		return 0
	}
	return int(g.state.Ships[ship].Design.Cost) / 4
}

// plunderShipsForTech is Plunder_Ships_For_Tech_ (0x74ab4): ships built by
// someone else may yield technology.
func (g *Game) plunderShipsForTech(list []int16, show bool) error {
	s := &g.state
	for _, k := range list {
		sh := &s.Ships[k]
		if sh.Design.Builder != uint8(sh.Owner) {
			if err := g.getTechnologyFromScrappedShip(int(k), int(s.PlayerNum), show); err != nil {
				return err
			}
		}
	}
	return nil
}

// playerHasTech is Player_Has_Tech_ (0x78457).
func (g *Game) playerHasTech(app, p int) bool {
	st := &g.state.Players[p].AppStatus
	if app < 0 || app >= len(st) {
		return false
	}
	return st[app] == techKnown
}

// bestShipFuel is Best_Ship_Fuel_ (0x5709f).
func (g *Game) bestShipFuel(owner int) int {
	if owner >= 8 {
		return 5
	}
	best, idx := g.exeI16(fuelApps+2), 0
	for i := 0; i < 6; i++ {
		row := uint32(fuelApps + i*10)
		if g.playerHasTech(int(g.exeI16(row)), owner) && best < g.exeI16(row+2) {
			idx, best = i, g.exeI16(row+2)
		}
	}
	return idx
}

// getTechnologyFromScrappedShip is Get_Technology_From_Scrapped_Ship_
// (0x72b33): one of the ship's unknown technologies for certain, each other
// one with a falling chance.
func (g *Game) getTechnologyFromScrappedShip(ship, p int, show bool) error {
	s := &g.state
	sh := &s.Ships[ship]
	d := &sh.Design
	var list []int16
	try := func(addr uint32) {
		if app := g.exeI16(addr); !g.playerHasTech(int(app), p) {
			list = append(list, app)
		}
	}
	if fuel := g.bestShipFuel(int(sh.Owner)); fuel > -1 && fuel < 6 {
		try(uint32(fuelApps + fuel*10))
	}
	try(uint32(scrapDrive + int(d.Drive)*driveStride))
	try(uint32(scrapComp + int(d.Computer)*computerStride))
	try(uint32(scrapArmor + int(d.Armor)*0xf))
	for k := range d.Weapons {
		if w := d.Weapons[k].Weapon; w > 0 {
			try(uint32(scrapWeapon + int(w)*weaponStride))
		}
	}
	for bit := 1; bit < 0x27; bit++ {
		if testBit(d.Specials[:], bit) {
			try(uint32(scrapSpecial + bit*specialStride))
		}
	}
	if len(list) == 0 {
		return nil
	}
	msg := fmt.Sprintf(cFormat(g.lbxMessage(0x74)), save.CString(d.Name[:]))
	r := int16(s.Random.Random(uint32(len(list)))) - 1
	picks := []int16{list[r]}
	for i := int16(0); i < int16(len(list)); i++ {
		if i == r {
			continue
		}
		limit := 0x19 - 5*(len(picks)-1)
		if int(s.Random.Random(100))-1 < limit {
			picks = append(picks, list[i])
		}
	}
	for _, app := range picks {
		if app == 0 {
			continue
		}
		g.playerGetsTechApp(p, int(app))
		name, err := g.techAppName(int(app))
		if err != nil {
			return err
		}
		msg += cStringAt(g.exe, scrapSep1) + cStringAt(g.exe, scrapSep2) + name + cStringAt(g.exe, scrapSep3)
	}
	g.ub.tempString = save.CString(d.Name[:])
	if show {
		_, err := g.userBox(&g.ms.drawFleetScreenToBack, msg, boxTempTitle)
		return err
	}
	return nil
}

// clearAllStarRelocations is Clear_All_Star_Relocations_ (0x77bb1).
func (g *Game) clearAllStarRelocations(p int) {
	s := &g.state
	for i := 0; i < int(s.NumStars); i++ {
		s.Stars[i].Relocation[p] = -1
	}
}

// setAllStarRelocations is Set_All_Star_Relocations_ (0x785ec): every
// relocation of p now leads to star.
func (g *Game) setAllStarRelocations(star, p int) {
	s := &g.state
	for i := 0; i < int(s.NumStars); i++ {
		if s.Stars[i].Relocation[p] != -1 {
			s.Stars[i].Relocation[p] = int16(star)
		}
	}
}

// okayToSetRelocateStar is Okay_To_Set_Relocate_Star_ (0x75035): to tells
// the target from the source; refusals are explained in a box.
func (g *Game) okayToSetRelocateStar(star int, to bool) (bool, error) {
	s, m := &g.state, &g.ms
	flag := &m.drawFleetScreenToBack
	st := &s.Stars[star]
	p := int(s.PlayerNum)
	msg, ok := "", true
	pick := func(toID, fromID int) {
		ok = false
		if to {
			msg = g.hMessage(toID)
		} else {
			msg = g.hMessage(fromID)
		}
	}
	switch {
	case st.SpectralClass == save.SpectralBlackHole:
		pick(0x83, 0x84)
	case !s.playerHasVisited(p, star):
		pick(0x85, 0x86)
	default:
		if monster, guarded := g.starGuardedByMonster(star); guarded {
			race := g.raceName(monster)
			g.e.SetMouseList(engine.MouseArrow)
			if !to {
				ok = false
				break
			}
			msg := fmt.Sprintf(cFormat(g.hMessage(0x87)), save.CString(st.Name[:]), race)
			r, err := g.userBox(flag, msg, boxConfirm)
			return r != 0, err
		}
		if !to && !s.playerHasColony(p, star) {
			msg = fmt.Sprintf(cFormat(g.hMessage(0x88)), save.CString(st.Name[:]))
			ok = false
		}
	}
	if !ok && msg != "" {
		_, err := g.userBox(flag, msg, boxWarning)
		return false, err
	}
	return ok, nil
}

// starGuardedByMonster is Star_Guarded_By_Monster_ (0x7a47a): the owner of
// the first monster orbiting the star.
func (g *Game) starGuardedByMonster(star int) (int, bool) {
	s, st := &g.state, &g.stacks
	for i := int16(0); i < st.count; i++ {
		for n := st.start[i]; n != -1; n = st.node[n].next {
			sh := &s.Ships[st.node[n].ship]
			if sh.Status == 0 && sh.Owner > 7 && int(sh.Location) == star {
				return int(sh.Owner), true
			}
		}
	}
	return 0, false
}

// okayToSetRelocateToStar is Okay_To_Set_Relocate_To_Star_ (0x74faa).
func (g *Game) okayToSetRelocateToStar(from, to int) (bool, error) {
	s := &g.state
	ok, err := g.okayToSetRelocateStar(to, true)
	g.e.SetMouseList(g.exeMouseList(mouseListToFlt, 4))
	if err != nil || !ok {
		return ok, err
	}
	if blackHoleBit(&s.Stars[from], to) {
		msg := fmt.Sprintf(cFormat(g.hMessage(0x82)), save.CString(s.Stars[from].Name[:]), save.CString(s.Stars[to].Name[:]))
		_, err := g.userBox(&g.ms.drawFleetScreenToBack, msg, boxWarning)
		return false, err
	}
	return true, nil
}

// starRelocation is Star_Relocation_ (0x75180): the first click picks the
// source, the second the target (the source again clears it).
func (g *Game) starRelocation(from, to *int16, star int) error {
	s := &g.state
	if *from == -1 {
		ok, err := g.okayToSetRelocateStar(star, false)
		if err != nil {
			return err
		}
		g.e.SetMouseList(g.exeMouseList(mouseListFrom, 4))
		if ok {
			*from = int16(star)
			g.e.SetMouseList(g.exeMouseList(mouseListToFlt, 4))
		}
		return nil
	}
	ok, err := g.okayToSetRelocateToStar(int(*from), star)
	if err != nil || !ok {
		return err
	}
	*to = int16(star)
	g.e.SetMouseList(engine.MouseArrow)
	if *to == *from {
		if err := g.cancelStarRelocation(int(*to)); err != nil {
			return err
		}
	} else {
		s.Stars[*from].Relocation[s.PlayerNum] = int16(star)
	}
	g.e.SetMouseList(g.exeMouseList(mouseListFrom, 4))
	*from, *to = -1, -1
	return nil
}

// cancelStarRelocation is Cancel_Star_Relocation_ (0x7522b).
func (g *Game) cancelStarRelocation(star int) error {
	s := &g.state
	msg := fmt.Sprintf(cFormat(g.hMessage(0x89)), save.CString(s.Stars[star].Name[:]))
	s.Stars[star].Relocation[s.PlayerNum] = -1
	_, err := g.userBox(&g.ms.drawFleetScreenToBack, msg, boxWarning)
	return err
}
