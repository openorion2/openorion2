package game

import (
	"encoding/binary"
	"fmt"

	"github.com/openorion2/openorion2/internal/save"
)

// sprintf_msg_ (0xeec98): the text of a message record (0x12 bytes) for the
// turn summary. The formats are the 80 strings Load_Msg_Strings_ (0xee4a1)
// takes from RSTRING<lang>.LBX after the Colcalc and Combfind ones. Spec:
// docs/screens/turn-summary.md.

// msgStringsFirst is the index of _native_msg among the report strings.
const msgStringsFirst = 60 + 10

// Load_Msg_Strings_' order (offsets from msgStringsFirst).
const (
	msNative = iota
	msAlliance
	msWar
	msFinancial
	msLostShip
	msScrapShipBC
	msScrapShipHyperspace
	msColonyOrOutpost // 2 strings
	_
	msLostColony
	msLostColonyLeader
	msScrapBuilding
	msBuiltBuilding
	msBuiltShip
	msBuiltRefit
	msBuiltSpy
	msBuiltFarmer
	msBuiltWorker
	msBuiltScientist
	msBuiltFreighter
	msNextQueue
	msAutoBuild
	msGenocide
	msAssimilation
	msStarvationFood
	msStarvationInd
	msStarvationBoth
	ms1SettlerArrived
	msDSettlersArrived
	msSettlersBlockaded
	ms1SettlerOverLimit
	msDSettlersOverLimit
	msPlague
	msBoom
	msComet
	msYourRebellionWon
	msYourRebellionLost
	ms1RebelWon
	msDRebelsWon
	ms1RebelLost
	msDRebelsLost
	msNoRoomForAndroid
	msAndroidKilledOrganic
	msHyperspaceFlux
	msBeastHungry
	msBeastAteYourShip
	msBeastAteOtherShip
	msLostAttackingSpy
	msLostDefendingSpy
	msLostDefendingSpies
	msArtemis
	msArtemisLeader
	ms1ShipKilledRetreating
	msDShipsKilledRetreating
	msDefensiveSpy
	msOffensiveSpy
	msSpyAssassinated
	msSpyAssassinatedVictim
	msAnomaly
	msDiplomacy // 5 strings
	_
	_
	_
	_
	msEelSpawned
	msEelVanished
	msArtifacts
	msApril1
	msScrapLeader
	msFailedRetreatReason // 3 strings
	_
	_
	msFailedRetreatLeader
	msLeaderFate // 3 strings
	_
	_
	msLostColonyReason // 4 strings
)

// sprintfMsg is sprintf_msg_ (0xeec98).
func (g *Game) sprintfMsg(m []byte) (string, error) {
	rs, err := g.rStrings()
	if err != nil {
		return "", err
	}
	if len(rs) < msgStringsFirst+80 {
		return "", fmt.Errorf("rstring: %d strings", len(rs))
	}
	ms := func(i int) string { return rs[msgStringsFirst+i] }
	s := &g.state
	i16 := func(o int) int { return int(int16(binary.LittleEndian.Uint16(m[o:]))) }
	race := func(p int) string { return save.CString(s.Players[p].RaceName[:]) }
	emperor := func(p int) string { return save.CString(s.Players[p].EmperorName[:]) }
	star := func(i int) string { return save.CString(s.Stars[i].Name[:]) }
	leader := func(l int) string { return save.CString(s.Leaders[l].Name[:]) }
	ship := func(i int) string { return save.CString(s.Ships[i].Design.Name[:]) }
	shipCost := func(i int) int { return int(s.Ships[i].Design.Cost) }
	hull := func(i int) (string, error) { return g.hullName(int(s.Ships[i].Design.Size), false) }
	fate := func(l, f int) string {
		return cSprintf(ms(msLeaderFate+f), leader(l))
	}
	switch m[0] {
	case 0:
		return g.sprintfColonyMsg(m, ms)
	case 1:
		return cSprintf(ms(msAlliance), race(i16(2)), race(i16(4))), nil
	case 2:
		return cSprintf(ms(msWar), race(i16(2)), race(i16(4))), nil
	case 4:
		return cSprintf(ms(msFinancial), i16(2), i16(4)), nil
	case 5:
		h, err := hull(i16(4))
		if err != nil {
			return "", err
		}
		out := cSprintf(ms(msLostShip), h, ship(i16(4)), shipCost(i16(4)), star(i16(2)))
		if l := int(int8(m[6])); l != -1 {
			out += fate(l, int(int8(m[7])))
		}
		return out, nil
	case 6:
		h, err := hull(i16(6))
		if err != nil {
			return "", err
		}
		if int16(i16(2)) < s.NumStars {
			return cSprintf(ms(msScrapShipBC), h, ship(i16(6)), shipCost(i16(6)), star(i16(2)), i16(4)), nil
		}
		return cSprintf(ms(msScrapShipHyperspace), h, ship(i16(6)), shipCost(i16(6))), nil
	case 7:
		out := cSprintf(ms(msLostColony), ms(msColonyOrOutpost+int(m[8])), g.planetName(i16(2)), ms(msLostColonyReason+i16(4)))
		if l := i16(6); l != -1 {
			out += cSprintf(ms(msLostColonyLeader), leader(l))
		}
		return out, nil
	case 8:
		return ms(msHyperspaceFlux), nil
	case 9:
		sh := i16(2)
		if sh == -1 {
			return ms(msBeastHungry), nil
		}
		if m[7] != 0 {
			out := cSprintf(ms(msBeastAteYourShip), ship(sh), star(int(int8(m[4]))))
			if l := int(int8(m[5])); l != -1 {
				out += fate(l, int(int8(m[6])))
			}
			return out, nil
		}
		return cSprintf(ms(msBeastAteOtherShip), race(int(s.Ships[sh].Owner))), nil
	case 10:
		return cSprintf(ms(msLostAttackingSpy), race(i16(2))), nil
	case 11:
		if i16(2) == 1 {
			return ms(msLostDefendingSpy), nil
		}
		return cSprintf(ms(msLostDefendingSpies), i16(2)), nil
	case 12:
		return cSprintf(ms(msArtemis), star(int(int8(m[2])))), nil
	case 13:
		return cSprintf(ms(msArtemisLeader), star(int(int8(m[2])))) + fate(int(int8(m[3])), int(int8(m[4]))), nil
	case 14:
		f := ms(msDShipsKilledRetreating)
		if i16(4) == 1 {
			f = ms(ms1ShipKilledRetreating)
		}
		out := cSprintf(f, int(uint16(i16(4))), star(int(m[2])))
		out += "  " + ms(msFailedRetreatReason+int(m[3]))
		if m[6] != 0 {
			out += cSprintf(ms(msFailedRetreatLeader), int(m[6]))
		}
		return out, nil
	case 15:
		var who string
		if m[2] == m[3] {
			who = ms(msDefensiveSpy)
		} else {
			who = cSprintf(ms(msOffensiveSpy), emperor(int(m[3])))
		}
		if int16(m[5]) == s.PlayerNum {
			if g.settings[0xd2] == 4 {
				return cSprintf(ms(msSpyAssassinated), leader(int(m[4])), who, emperor(int(m[2]))), nil
			}
			return cSprintf(ms(msSpyAssassinated), leader(int(m[4])), emperor(int(m[2])), who), nil
		}
		return cSprintf(ms(msSpyAssassinatedVictim), who), nil
	case 16:
		return cSprintf(ms(msAnomaly), star(int(m[2]))), nil
	case 17:
		return cSprintf(ms(msDiplomacy+int(m[2])), race(int(m[3])), race(int(m[4]))), nil
	case 18:
		return cSprintf(ms(msEelSpawned), star(int(m[2]))), nil
	case 19:
		return cSprintf(ms(msEelVanished), star(int(m[2]))), nil
	case 20:
		a, err := g.techAppName(int(m[3]))
		if err != nil {
			return "", err
		}
		what := a
		if m[4] != 0 {
			and, err := g.kenText(0x68)
			if err != nil {
				return "", err
			}
			b, err := g.techAppName(int(m[4]))
			if err != nil {
				return "", err
			}
			what = a + " " + and + " " + b
		}
		return cSprintf(ms(msArtifacts), what, star(int(m[2]))), nil
	case 21:
		return ms(msApril1), nil
	case 22:
		return cSprintf(ms(msScrapLeader), leader(int(m[2]))), nil
	}
	// Type 3 and anything past 0x16 leave the buffer as it was.
	g.logOnce(fmt.Sprintf("sprintf_msg_: message type %d prints an unset buffer", m[0]))
	return "", nil
}

// sprintfColonyMsg is sprintf_msg_'s type 0, a colony's news (subtype +3).
func (g *Game) sprintfColonyMsg(m []byte, ms func(int) string) (string, error) {
	s := &g.state
	i16 := func(o int) int { return int(int16(binary.LittleEndian.Uint16(m[o:]))) }
	race := func(p int) string { return save.CString(s.Players[p].RaceName[:]) }
	planet := g.planetName(int(m[2]))
	switch m[3] {
	case 0:
		b, err := g.buildingName(i16(4))
		if err != nil {
			return "", err
		}
		return cSprintf(ms(msScrapBuilding), b, planet, i16(6)), nil
	case 1:
		var out string
		switch d := i16(4); {
		case d > 0 && d < 0x31:
			b, err := g.buildingName(d)
			if err != nil {
				return "", err
			}
			out = cSprintf(ms(msBuiltBuilding), planet, b)
		case d <= -100:
			f := ms(msBuiltShip)
			if m[8] != 0 {
				f = ms(msBuiltRefit)
			}
			out = cSprintf(f, planet, save.CString(s.Ships[-(d + 100)].Design.Name[:]))
		case d == -0xf:
			out = cSprintf(ms(msBuiltFreighter), planet)
		case d == -7:
			out = cSprintf(ms(msBuiltSpy), planet)
		case d == -6:
			out = cSprintf(ms(msBuiltScientist), planet)
		case d == -5:
			out = cSprintf(ms(msBuiltWorker), planet)
		case d == -4:
			out = cSprintf(ms(msBuiltFarmer), planet)
		}
		if i16(6) == -1 {
			return out, nil
		}
		f := ms(msNextQueue)
		if m[9] != 0 {
			f = ms(msAutoBuild)
		}
		p, err := g.productName(i16(6))
		if err != nil {
			return "", err
		}
		return out + cSprintf(f, p), nil
	case 2:
		who := ms(msNative)
		if int16(m[4]) < s.NumPlayers {
			who = race(int(m[4]))
		}
		return cSprintf(ms(msGenocide), who, planet), nil
	case 3:
		return cSprintf(ms(msAssimilation), race(int(m[4])), planet), nil
	case 4:
		switch {
		case m[4] == 0:
			return cSprintf(ms(msStarvationInd), planet, int(m[5])), nil
		case m[5] != 0:
			return cSprintf(ms(msStarvationBoth), planet, int(m[4]), int(m[5])), nil
		}
		return cSprintf(ms(msStarvationFood), planet, int(m[4])), nil
	case 5:
		f := ms(msDSettlersArrived)
		if m[4] == 1 {
			f = ms(ms1SettlerArrived)
		}
		out := cSprintf(f, int(m[4]), planet)
		if m[6] != 0 {
			return out + ms(msSettlersBlockaded), nil
		}
		if m[5] == 0 {
			return out, nil
		}
		f = ms(msDSettlersOverLimit)
		if m[5] == 1 {
			f = ms(ms1SettlerOverLimit)
		}
		return out + cSprintf(f, int(m[5])), nil
	case 6:
		return cSprintf(ms(msPlague), planet), nil
	case 7:
		return cSprintf(ms(msBoom), planet), nil
	case 8:
		return cSprintf(ms(msComet), planet), nil
	case 9:
		if int16(m[4]) == s.PlayerNum {
			if m[6] < m[5] {
				return cSprintf(ms(msYourRebellionWon), int(m[5]), planet, int(m[6])), nil
			}
			return cSprintf(ms(msYourRebellionLost), int(m[5]), planet), nil
		}
		if m[6] < m[5] {
			f := ms(msDRebelsWon)
			if m[5] == 1 {
				f = ms(ms1RebelWon)
			}
			return cSprintf(f, int(m[5]), race(int(m[4])), planet, int(m[6])), nil
		}
		f := ms(msDRebelsLost)
		if m[5] == 1 {
			f = ms(ms1RebelLost)
		}
		return cSprintf(f, int(m[5]), race(int(m[4])), planet), nil
	case 10:
		return cSprintf(ms(msNoRoomForAndroid), planet), nil
	case 11:
		who := ms(msNative)
		if m[4] < 8 {
			who = race(int(m[4]))
		}
		return cSprintf(ms(msAndroidKilledOrganic), planet, who), nil
	}
	return "", nil
}

// productName is Product_Name_ (0xcf398): what a colony builds next.
// Designs are named from the player of the colony _colony_handle points at,
// which off the colony screen is whatever it last held (0 at start).
func (g *Game) productName(id int) (string, error) {
	s := &g.state
	switch {
	case id == -0x25:
		return cStringAt(g.exe, 0x179f48), nil // "AI trade goods"
	case id > 0 && id < 0x31:
		return g.buildingName(id)
	case id > -100:
		return g.optionString(id)
	}
	sh := &s.Ships[-(id + 100)]
	name := save.CString(sh.Design.Name[:])
	if sh.Status == 4 {
		// A refit: "<Refit> <name>" (it also resets _last_setting/_last_app).
		refit, err := g.optionString(-8)
		if err != nil {
			return "", err
		}
		es, err := g.eStrings()
		if err != nil {
			return "", err
		}
		return cSprintf(es[0x2d], refit, name), nil
	}
	return name, nil
}

// optionString is Option_String_ (0xafc6d): the names of the special
// products, E strings, and the player designs (-50..-99).
func (g *Game) optionString(id int) (string, error) {
	es, err := g.eStrings()
	if err != nil {
		return "", err
	}
	e := map[int]int{-0x11: 0x195, -0x10: 0x10f, -0xf: 0x12d, -0xd: 0xfa, -0xc: 0xe6, -0xb: 0x21e,
		-10: 0x88, -9: 0xc, -1: 0xc, 0: 0xc, -8: 0x1bd, -7: 0x1ec, -6: 0xb1, -5: 0xb2, -4: 0xb0,
		-3: 0x142, -2: 0x21d}
	if n, ok := e[id]; ok {
		return es[n], nil
	}
	if id > -100 && id < -0x31 {
		s := &g.state
		owner := s.Colonies[g.colonyHandle].Owner
		return save.CString(s.Players[owner].Designs[-(id + 0x32)].Name[:]), nil
	}
	return "", fmt.Errorf("Option_String_(%d) returns -1", id)
}
