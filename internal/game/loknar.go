package game

import (
	"bytes"
	"encoding/binary"
)

// Loknar, the officer of the Guardian of Orion, and the techs he brings
// (OFFICERS.C, the Orion reward). Spec: docs/turn/overview.md, Do_1_Combat_.

// loknar is leader 0x41.
const loknar = 0x41

// giveLoknarSpecial is Give_Player_Loknar_Special_ (0x980db): the Avenger
// with Loknar aboard (if the player has room for another officer), the
// Death Ray app and up to three random Antaran techs. The techs are kept
// for Rescued_Loknar_Popup_ (Do_1_Combat_'s +0x277, count +0x27f).
func (g *Game) giveLoknarSpecial(p int16) {
	s := &g.state
	k := g.assignLoknarShipToPlayer(int(p))
	none := k == -1 || s.nOfficersOwned(int(p), 0) >= 4
	L := &s.Leaders[loknar]
	if none {
		L.Location, L.Owner, L.Status = -1, -1, 6
		if k != -1 {
			s.Ships[k].Officer = -1
		} else {
			g.logOnce("Give_Player_Loknar_Special_ writes ship -1's officer; skipped")
		}
	} else {
		L.Status, L.Location, L.Owner = 1, int16(k), int8(p)
		g.giveLeaderSpecialsToPlayer(loknar, int(p))
	}
	g.loknarTechs = g.loknarTechs[:0]
	if g.playerMayGetTechApp(int(p), 0x2f) {
		g.playerGetsTechApp(int(p), 0x2f)
		g.loknarTechs = append(g.loknarTechs, 0x2f)
	}
	g.getRandomAntaranTech(int(p), 3)
	g.updateShipIconsInBetweenTurns()
}

// assignLoknarShipToPlayer is Assign_Loknar_Ship_To_Player_ (0x983b8): the
// first ship of the Guardian (owner 9) becomes p's Avenger at star 0 with
// Loknar as its officer; -1 when there is none.
func (g *Game) assignLoknarShipToPlayer(p int) int {
	s := &g.state
	k := -1
	for i := 0; i < int(s.NumShips) && k == -1; i++ {
		if s.Ships[i].Owner == 9 {
			k = i
		}
	}
	if k <= -1 || k >= int(s.NumShips) {
		return k
	}
	rec := make([]byte, 0x81)
	copy(rec, g.loknarShipDesign())
	rec[0x63], rec[0x5d] = byte(p), byte(p)
	binary.LittleEndian.PutUint16(rec[0x65:], 0)
	binary.LittleEndian.PutUint16(rec[0x67:], uint16(s.Stars[0].X))
	binary.LittleEndian.PutUint16(rec[0x69:], uint16(s.Stars[0].Y))
	binary.LittleEndian.PutUint16(rec[0x74:], loknar)
	rec[0x64] = 0
	g.must2(binary.Read(bytes.NewReader(rec), binary.LittleEndian, &s.Ships[k]))
	return k
}

// loknarShipDesign is Load_Loknar_Ship_Design_ (0x58487): the 0x63-byte
// design "Avenger".
func (g *Game) loknarShipDesign() []byte {
	b := make([]byte, 0x63)
	copy(b, "Avenger\x00")
	b[0x11], b[0x12], b[0x15], b[0x10], b[0x16], b[0x14], b[0x13] = 0, 0, 5, 3, 6, 8, 6
	slot := func(o int, w uint16, arc byte, mods uint16, ammo, count, working byte) {
		binary.LittleEndian.PutUint16(b[o+0x1c:], w)
		b[o+0x20] = arc
		binary.LittleEndian.PutUint16(b[o+0x21:], mods)
		b[o+0x23] = ammo
		b[o+0x1e], b[o+0x1f] = count, working
	}
	slot(0, 4, 1, 2, 0, 4, 4)
	slot(8, 0xb, 1, 2, 0, 2, 2)
	slot(0x10, 4, 1, 0, 0, 0xa, 0xa)
	slot(0x18, 0xd, 0xf, 0, 0, 2, 2)
	slot(0x20, 4, 1, 4, 0, 0xa, 0xa)
	slot(0x28, 0x18, 1, 0, 0xa, 0xf, 0xf)
	for _, n := range []int{7, 0x18, 0x21} {
		setBit(b[0x17:], n)
	}
	b[0x5c], b[0x5d] = 0x2c, 8
	binary.LittleEndian.PutUint16(b[0x5e:], 500)
	b[0x60] = g.exeU8(shipCombatSpeedTbl+uint32(b[0x13])*0x2e+uint32(b[0x10])) + 2
	return b
}

// getRandomAntaranTech is Get_Random_Antaran_Tech_ (0x982b6): up to n of
// the apps of field 0x4a the player may get, picked at random.
func (g *Game) getRandomAntaranTech(p, n int) {
	var avail []int16 // Get_Available_Antaran_Tech_ (0x98363)
	for a := 0; a < 0xd4; a++ {
		if g.techAppFieldOf(a) == 0x4a && g.playerMayGetTechApp(p, a) {
			avail = append(avail, int16(a))
		}
	}
	if n > len(avail) {
		n = len(avail)
	}
	for ; n != 0; n-- {
		k := int(g.state.Random.Random(uint32(len(avail)))) - 1
		app := avail[k]
		g.playerGetsTechApp(p, int(app))
		g.loknarTechs = append(g.loknarTechs, app)
		avail[k] = avail[len(avail)-1]
		avail = avail[:len(avail)-1]
	}
}

// rescuedLoknarPopup is Rescued_Loknar_Popup_ (0x98282): with techs
// gained, Make_Loknar_Message_'s text in a User_Box_, then a full redraw.
func (g *Game) rescuedLoknarPopup() error {
	if len(g.loknarTechs) <= 0 {
		return nil
	}
	h := 0x26
	if g.state.Leaders[loknar].Status == 6 {
		h = 0x25
	}
	msg := g.hMessage(h)
	for _, a := range g.loknarTechs {
		name, err := g.techAppName(int(a))
		if err != nil {
			return err
		}
		msg += cStringAt(g.exe, 0x179b1d) + cStringAt(g.exe, 0x179b20) + name + cStringAt(g.exe, 0x179b23)
	}
	if _, err := g.userBox(&g.ms.drawToBack, msg, boxMessage); err != nil {
		return err
	}
	g.fullDrawScreen = true
	return nil
}
