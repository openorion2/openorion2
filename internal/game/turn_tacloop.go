package game

import (
	"github.com/openorion2/openorion2/internal/clib"
	"github.com/openorion2/openorion2/internal/engine"
	"github.com/openorion2/openorion2/internal/save"
)

// The combat screen's logic (COMBINIT.C, COMBAT1.C): Tactical_Combat_ and
// its turn loop, played with the screen's drawing behind tacticalUI.
// Spec: docs/combat/tactical.md.

// tacticalUI stands for the combat screen. The logic asks it only for the
// human's input; drawing that changes no state is left out.
type tacticalUI interface {
	// HumanInput is Get_Input in Do_Combat_Turn_'s input loop for a human
	// ship; it returns a tacInput* value.
	HumanInput(g *Game) int
	// AutoPressed is Get_Input == _auto_button inside Do_Auto_Ship_Turn_.
	AutoPressed(g *Game) bool
}

// Inputs a tacticalUI hands back.
const (
	tacInputNone = iota
	tacInputAuto // the Auto button, hot key "a"
	tacInputDone // the Done button, hot key "d"
)

// autoUI is the stand-in screen until the combat screen is ported: the
// human presses Auto at the first input, so the AI plays both sides.
type autoUI struct{}

func (autoUI) HumanInput(*Game) int   { return tacInputAuto }
func (autoUI) AutoPressed(*Game) bool { return false }

// colonyHasDefenses is Colony_Has_Defenses_ (0x478a3).
func (g *Game) colonyHasDefenses(ci int16) bool {
	b := &g.state.Colonies[ci].Buildings
	return b[0x28] != 0 || b[0x2f] != 0 || b[8] != 0 || b[0x29] != 0 || b[0x1a] != 0 || b[0x1b] != 0 || b[0x2a] != 0
}

// tacticalCombat is Tactical_Combat_ (0x47939): the battle on the combat
// screen; > 0 the attacker won, 0 a draw, < 0 the defender won.
func (g *Game) tacticalCombat(attList, defList []int16, star, colony int16, canRetreat bool) int16 {
	s := &g.state
	ui := tacticalUI(autoUI{})
	if g.tc != nil && g.tc.ui != nil {
		ui = g.tc.ui
	}
	old := g.tc
	g.tc = &tactical{ui: ui}
	tc := g.tc
	tc.clearBlocks()
	if old != nil {
		// Globals the bombardment engine shares and this routine does not set.
		tc.ordnance = old.ordnance
		tc.fighterBomb, tc.fighterBeam = old.fighterBomb, old.fighterBeam
	}
	if star == s.NumStars && g.e != nil {
		g.must2(g.playDimensionalPortalFlic()) // 0x47980: the way to Antares
	}
	noScreen := false
	result := int16(0)
	popBefore := int16(0)
	tc.turn = 0
	tc.bioWeapon = 0
	tc.superFast = 0
	tc.realAttack = 0
	tc.takeControl = 0
	tc.onScreen = 1
	tc.mslWarning = 0
	tc.orderInfo = 1
	tc.aiRetreat = int16(b2i(canRetreat))
	tc.shieldsFlag = int16(g.settings[0xb7])
	tc.speedx2 = int16(g.settings[0xb4])
	tc.mslImpact = int16(g.settings[0xb6])
	tc.savedMslWarning = tc.mslImpact
	tc.initOrder = int16(g.settings[0xd])
	g.combatInit, g.combatInitSet = tc.initOrder, true
	tc.colony = colony
	tc.n = 1
	tc.att = int16(s.Ships[attList[0]].Owner)
	switch {
	case star == s.NumStars:
		tc.def = 8
	case colony == -1:
		tc.def = int16(s.Ships[defList[0]].Owner)
	default:
		popBefore = int16(s.Colonies[colony].PopCount)
		tc.def = int16(s.Colonies[colony].Owner)
	}
	if star == s.NumStars {
		tc.nebula = 0
	} else {
		tc.nebula = int16(b2i(g.pointIsInSomeNebula(s.Stars[star].X, s.Stars[star].Y)))
	}
	bySize := func(x, y *int16) int {
		a, c := s.Ships[*x].Design.Size, s.Ships[*y].Design.Size
		switch {
		case a == c:
			return 0
		case a < c:
			return 1
		}
		return -1
	}
	clib.Qsort(attList, 2, bySize)
	loaded := 0
	for _, id := range attList {
		if s.Ships[id].Design.Type != 0 {
			continue
		}
		if loaded++; loaded >= 100 {
			continue
		}
		g.loadCombatShip(id, tc.n)
		if !g.combatShipHasSpecial(tc.n, 0x17) {
			tc.realAttack = 1
		}
		tc.n++
	}
	if tc.def < 8 && star < s.NumStars && s.playerHasArtemisNet(int(tc.def), star) {
		// A message box tells the human what the net did (UI).
		noScreen = g.applyTacticalArtemisNet()
		result = -1
	}
	allNonCombat := star != s.NumStars
	for _, id := range defList {
		if !allNonCombat {
			break
		}
		if s.Ships[id].Design.Type == 0 {
			allNonCombat = false
		}
	}
	if allNonCombat && !noScreen && star != s.NumStars && (colony == -1 || !g.colonyHasDefenses(colony)) {
		noScreen = true
		result = 1
	}
	if !noScreen {
		if g.e != nil && (g.combatUI == nil || combatOnScreen(g.combatUI)) {
			// The screen (a test with combat answers plays off screen).
			if err := g.combatScreenLoad(); err != nil {
				panic(err)
			}
		}
		result = g.tacticalCombatScreen(defList, star)
		if colony != -1 {
			popBefore -= int16(s.Colonies[colony].PopCount)
		}
	}
	g.endOfCombat(star, popBefore, result)
	if (result == 1 || result == 0x29a) && tc.att < 8 && tc.def >= 10 && s.Ships[defList[0]].Design.Drive != 0 {
		g.russEventMonsterUpdate(int(tc.def), -1, tc.att)
	}
	if (result == -1 || result == 0x29a) && tc.def < 8 && tc.att >= 10 && s.Ships[attList[0]].Design.Drive != 0 {
		g.russEventMonsterUpdate(int(tc.att), -1, tc.def)
	}
	if !noScreen {
		if tc.scr != nil {
			if err := g.combatScreenClose(); err != nil {
				panic(err)
			}
		} else {
			g.updateShipIconsInBetweenTurns()
		}
	}
	g.settings[0xb7] = byte(tc.shieldsFlag)
	g.settings[0xb4] = byte(tc.speedx2)
	g.settings[0xb6] = byte(tc.mslImpact)
	if tc.scr != nil {
		g.settings[0xb5] = byte(tc.scr.legalBoxes)
		g.settings[0xb8] = byte(tc.scr.gridFlag)
		g.fullDrawScreen = tc.netFlag != 0 // 0x48ec7: _net_flag
	}
	if result == 0x29a {
		result = 0
	}
	return result
}

// tacticalCombatScreen is the part of Tactical_Combat_ that runs with the
// screen: the defenders, deployment and the turn loop.
func (g *Game) tacticalCombatScreen(defList []int16, star int16) int16 {
	s := &g.state
	tc := g.tc
	for i := range tc.mis {
		tc.mis[i] = tmissile{}
	}
	g.tacticalNetSetup()
	if tc.att < 9 {
		tc.fighterBeam[tc.att] = int16(g.bestPointDefenseWeapon(tc.att))
		tc.fighterBomb[tc.att] = int16(g.bestBombWeapon(int(tc.att)))
	}
	if tc.def < 9 {
		tc.fighterBeam[tc.def] = int16(g.bestPointDefenseWeapon(tc.def))
		tc.fighterBomb[tc.def] = int16(g.bestBombWeapon(int(tc.def)))
	}
	if star == s.NumStars {
		g.loadAntaranDefenseFleet()
		if err := g.loadAntaranPlanet(); err != nil {
			panic(err)
		}
	} else {
		clib.Qsort(defList, 2, func(x, y *int16) int {
			a, c := s.Ships[*x].Design.Size, s.Ships[*y].Design.Size
			switch {
			case a == c:
				return 0
			case a < c:
				return 1
			}
			return -1
		})
		loaded := 0
		for _, id := range defList {
			if s.Ships[id].Design.Type != 0 {
				continue
			}
			if loaded++; loaded >= 100 {
				continue
			}
			g.loadCombatShip(id, tc.n)
			tc.n++
		}
		g.loadColonyDefense(tc.colony)
	}
	if tc.scr != nil {
		darken, reflection, tractor, bhg, web, converter := g.combatEffectFlags()
		if star == s.NumStars {
			// The loading loop that sets these is skipped at Antares, which
			// sets the black hole generator and reflection flags itself.
			darken, reflection, tractor, bhg, web, converter = false, true, false, true, false, false
		}
		if err := g.combatScreenRemaps(darken, reflection, tractor, bhg, web, converter); err != nil {
			panic(err)
		}
	}
	g.setCombatOrdnanceValues()
	if tc.scr != nil {
		if err := g.combatShipPictures(); err != nil {
			panic(err)
		}
	}
	for i := int16(0); i < tc.n; i++ {
		g.initSpecialDevices(i)
	}
	g.deployShips()
	tc.autoAtt, tc.autoDef = 0, 0
	tc.skan = 0
	if tc.scr != nil {
		if err := g.addTacticalCombatFields(); err != nil {
			panic(err)
		}
	}
	result := g.checkForWinner()
	if tc.scr != nil {
		g.e.AssignAutoFunction(g.drawMainCombatScreen, 1)
		g.e.SetMouseList(engine.MouseArrow)
	}
	first, second := tc.att, tc.def
	if tc.def == 8 {
		first, second = 8, tc.att
	}
	g.tacticalNetStart()
	for result == 0 {
		for i := int16(0); i < tc.n; i++ {
			if i == 0 && tc.colony == -1 {
				continue
			}
			t := g.tsp(i)
			t.set16(tsSpeed, g.calcCurrentSpeed(i))
			t.set16(tsOcv, g.offensiveCombatBonus(i))
			t.set16(tsDcv, g.defensiveCombatBonus(i, 0))
			t.set16(tsEvasion, g.defensiveCombatBonus(i, 1))
		}
		for m := range tc.mis {
			M := &tc.mis[m]
			if M.i16(tmWeapon) > 0 {
				if M.i16(tmTravel) == 0 {
					M.set16(tmTravel, int16(M[tmSpeed])*0x14)
				}
				M[tmLive] = 1
			}
		}
		tc.fleetTarget = -1
		superFast := func() {
			if tc.superFast != 0 && tc.scr != nil {
				g.must2(g.drawSuperFastScreen())
			}
		}
		if tc.initOrder != 0 {
			// The 1.40 patch's hook at 0x1694b9 runs Increment_Bhg_ then
			// Plasma_Web_Damage_; edx survives it, so the second turn is phase 1.
			g.doCombatTurn(0, 0)
			g.incrementBhg(first)
			g.plasmaWebDamage(first)
			g.incrementBhg(second)
			g.plasmaWebDamage(second)
			g.doCombatTurn(0, 1)
			result = g.checkForWinner()
			superFast()
		} else {
			g.doCombatTurn(first, 0)
			g.checkForWinner()
			g.plasmaWebDamage(second)
			superFast()
			if g.checkForWinner() == 0 {
				tc.fleetTarget = -1
				g.doCombatTurn(second, 0)
				g.checkForWinner()
			}
			g.plasmaWebDamage(first)
			superFast()
			if g.checkForWinner() == 0 {
				tc.fleetTarget = -1
				g.doCombatTurn(first, 1)
				g.incrementBhg(first)
			}
			superFast()
			if g.checkForWinner() == 0 {
				tc.fleetTarget = -1
				g.doCombatTurn(second, 1)
				g.incrementBhg(second)
			}
			superFast()
			result = g.checkForWinner()
		}
		tc.turn++
		g.endOfTurnBookkeeping()
	}
	return result
}

// applyTacticalArtemisNet is Apply_Combat_Artemis_Net_ (0x3868f) with flag
// 0: each attacker may hit the net and take 0x95..0x225 damage in 20s,
// shields down; true when none survived.
func (g *Game) applyTacticalArtemisNet() bool {
	s := &g.state
	tc := g.tc
	all := true
	for i := int16(1); i < tc.n; i++ {
		T := g.tsp(i)
		chance := int32(g.exeI16(uint32(artemisChance + 2*int(T.u8(tsSize)))))
		if int32(s.Random.Random(100)) > chance {
			all = false
			continue
		}
		for a := 0; a < 4; a++ {
			T.set16(tsArc+2*a, 0)
		}
		d := int16(s.Random.Random(0x191) + 0x95)
		for d > 0 && g.isCombatShipDead(i) == 0 {
			d -= 0x14
			v, ab := int16(0x14), int16(0)
			g.applyShipDamage(i, &v, &ab, 0, 0, 0, 0, 0, 0)
		}
		if g.isCombatShipDead(i) == 0 {
			all = false
			mx := g.calcCombatShieldMax(i)
			for a := 0; a < 4; a++ {
				T.set16(tsArc+2*a, mx)
			}
		} else {
			T.set8(tsGone, 5)
		}
	}
	return all
}

// loadColonyDefense is Load_Colony_Defense_ (0x4a9e9): without a colony
// record 0 is a dead placeholder.
func (g *Game) loadColonyDefense(ci int16) {
	tc := g.tc
	if ci < 0 {
		t := g.tsp(0)
		t.set8(tsOwner, 8)
		t.set8(tsBuilder, 8)
		t.set16(tsID, -1)
		t.set8(tsGone, 5)
		return
	}
	g.loadTacticalColony(ci)
	if tc.scr != nil {
		if err := g.loadCombatColonyPicture(ci); err != nil {
			panic(err)
		}
	}
	if k := g.typeOfSpaceStation(tc.colony); k != -1 {
		g.loadCombatSatelite(k)
		tc.n++
	}
}

// typeOfSpaceStation is Type_Of_Space_Station_ (0x4a884): the hull row of
// the colony's star base (6), battle station (7) or star fortress (8).
func (g *Game) typeOfSpaceStation(ci int16) int16 {
	b := &g.state.Colonies[ci].Buildings
	switch {
	case b[0x28] != 0:
		return 6
	case b[8] != 0:
		return 7
	case b[0x29] != 0:
		return 8
	}
	return -1
}

// loadCombatSatelite is Load_Combat_Satelite_ (0x4bbd5): the colony's
// station as the next record, armed from its space with the defender's best
// beams, point defence and missiles, and specials from its technology.
func (g *Game) loadCombatSatelite(kind int16) {
	s := &g.state
	tc := g.tc
	def := tc.def
	t := g.tsp(tc.n)
	t.set8(tsComputer, uint8(g.bestComputer(int(def))))
	armor := uint8(g.bestArmor(int(def)))
	t.set8(tsArmor, armor)
	pct := int32(g.armorField(int(armor), armorHitsPct))
	row := func(off int32) int16 { return g.exeI16(uint32(hullTable + int32(kind)*hullStride + off)) }
	var di, missiles int16
	if kind >= 6 && kind <= 8 {
		// The name (the hull row's string) is not read by the engine.
		t.set16(tsStruct, row(2))
		t.set16(tsArmorHits, int16(pct*int32(row(0))/100)+row(0))
		sp := row(-4)
		if s.Players[def].AppStatus[0x59] == 3 {
			sp = int16(int32(sp) * 5 / 4)
		}
		di = sp / 2
		missiles = sp - di
		t.set8(tsSize, uint8(kind-4))
		t.set16(tsPicture, 0x22+kind)
	}
	t.set16(tsID, -1)
	t.set8(tsDrive, 0)
	t.set8(tsOwner, uint8(def))
	t.set8(tsBuilder, uint8(def))
	t.set8(tsShield, uint8(g.bestShipShield(int(def))))
	t.set8(tsGone, 0)
	for k := 0; k < 5; k++ {
		t[tsSpecials+k] = 0
		t[tsDamaged+k] = 0
	}
	ap := &s.Players[def].AppStatus
	sp := t[tsSpecials : tsSpecials+5]
	n := 0
	first := ap[0x52] == 3
	if first {
		setBit(sp, 0xe)
		t.set16(tsArmorHits, t.i16(tsArmorHits)*3)
		n = 1
	}
	try := func(app int, bit int) {
		if ap[app] == 3 && n < 8 {
			n++
			setBit(sp, bit)
		}
	}
	if ap[0x38] == 3 && b2i(first) < 8 {
		n++
		setBit(sp, 0x1b)
	}
	try(0xa1, 0x1e)
	try(0x51, 0xd)
	try(0x70, 0x14)
	switch {
	case ap[0xc7] == 3 && n < 8:
		n++
		setBit(sp, 0x26)
	case ap[0x6f] == 3 && n < 8:
		n++
		setBit(sp, 0x15)
	case ap[0x39] == 3 && n < 8:
		n++
		setBit(sp, 9)
	}
	try(0xc4, 0x25)
	try(0x9f, 0x1d)
	try(0x1a, 5)
	try(0x55, 0xf)
	try(0xaf, 0x20)
	try(1, 1)
	try(0x97, 0x19)
	if ap[0xbe] == 3 && n < 8 {
		setBit(sp, 0x23)
	}
	t.set16(tsStruct, t.i16(tsStruct)+int16(pct*int32(t.i16(tsStruct))/100))
	if g.combatShipHasSpecial(tc.n, 0x1b) {
		t.set16(tsStruct, t.i16(tsStruct)*3)
	}
	for k := 0; k < 8; k++ {
		copy(t[tw(k, 0):tw(k, 0)+0xb], make([]byte, 0xb))
	}
	mount := func(k int, w, mods, ammo int16) {
		t.set16(tw(k, twWeapon), w)
		t.set8(tw(k, twOn), 1)
		t.set16(tw(k, twAmmo), ammo)
		t.set8(tw(k, twArc), 0xf)
		t.set16(tw(k, twMods), mods)
	}
	count := func(k int, n int16) {
		t.set8(tw(k, twCount), uint8(n))
		t.set8(tw(k, twLeft), uint8(n))
	}
	beam := int16(g.bestBeamWeapon(int(def)))
	mount(0, beam, 2, -1)
	c := int16(int32(di) / 2 / g.weaponSpace(int(def), int(beam), 1, 0xf, 0, 2))
	c = min(max(c, 1), 99)
	count(0, c)
	beam = int16(g.bestBeamWeapon(int(def)))
	mount(1, beam, 0, -1)
	c = int16(int32(di) / 4 / g.weaponSpace(int(def), int(beam), 1, 0xf, 0, 0))
	c = min(max(c, 1), 0xc6)
	k := 1
	for ; c > 99; k++ {
		count(k, 99)
		mount(k+1, int16(g.bestBeamWeapon(int(def))), 0, -1)
		c -= 99
	}
	count(k, c)
	k++
	pd := int16(g.bestPointDefenseWeaponBySize(int(def)))
	mount(k, pd, 4, -1)
	c = int16(int32(di) / 4 / g.weaponSpace(int(def), int(pd), 1, 0xf, 0, 4))
	c = min(max(c, 1), 0xc6)
	for ; c > 99; k++ {
		count(k, 99)
		mount(k+1, int16(g.bestPointDefenseWeaponBySize(int(def))), 4, -1)
		c -= 99
	}
	count(k, c)
	k++
	msl := int16(g.bestMissileWeaponNT(int(def)))
	ml := g.miniaturizationLevel(int(def), int(g.wpnI16(int(msl), weaponApp)))
	mods := int16(0)
	if ml >= int16(g.exeU8(0x17fdc8)) {
		mods = 0x1000
	}
	if ml >= int16(g.exeU8(0x17fdb9)) {
		mods |= 0x800
	}
	mammo := func() int16 {
		if g.wtype(msl) == 1 {
			return 0x14
		}
		return 0
	}
	mount(k, msl, mods|0x400, mammo())
	c = int16(int32(missiles) / g.weaponSpace(int(def), int(msl), 1, 0xf, 0x14, 0))
	c = min(max(c, 1), 0x5a)
	for c > 0x1e && k < 8 {
		count(k, 0x1e)
		k++
		mount(k, msl, mods|0x400, mammo())
		c -= 0x1e
	}
	count(k, min(c, 0x1e))
	for j := 0; j < 8; j++ {
		t[tsSpecialOn+j] = 1
	}
	t.set8(tsCrew, 1)
	t.set16(tsOfficer, -1)
	t.set16(tsSpeed, 0)
	t.set16(tsStructDmg, 0)
	t.set8(0x4b, 0)
	t.set8(tsMarines, g.exeU8(hullMarines+uint32(kind)*hullStride))
	g.initInternalSpaceRec(t)
	if t.u8(tsShield) == 0 {
		t.set16(tsShieldMax, 0)
	} else {
		t.set16(tsShieldMax, g.calcCombatShieldMax(tc.n))
	}
	for a := 0; a < 4; a++ {
		t.set16(tsArc+2*a, t.i16(tsShieldMax))
	}
}

func hullSquares(size uint8) int16 {
	switch {
	case size == 0:
		return 1
	case size < 3:
		return 2
	case size < 6:
		return 3
	}
	return 0
}

// deployShips is Deploy_Ships_ (0x49043): records sorted by size, bases next
// to the planet, each side in columns from its edge.
// In a lockstep battle both machines deploy as the attacker's (_net_flag).
func (g *Game) deployShips() {
	s := &g.state
	tc := g.tc
	P := g.tsp(0)
	var attX, defX int8 = 0x37, 0x19
	var attFace, defFace uint8 = 8, 0
	if tc.att == int16(s.PlayerNum) || tc.netFlag != 0 {
		attX, defX = 0x19, 0x37
		attFace, defFace = 0, 8
		P.set8(tsX, 0x41)
	} else {
		P.set8(tsX, 10)
	}
	attY1, attY2 := int16(0x22), int16(0x22)
	defY1, defY2 := int16(0x22), int16(0x22)
	P.set8(tsSize, 100)
	P.set8(tsY, 0x22)
	var baseSize uint8
	// community patch 1.51 (0x490fa, 0x49153, 0x49184): bases are the
	// satellites, not every record without drive.
	for i := int16(1); i < tc.n; i++ {
		if t := g.tsp(i); t.satellite() {
			baseSize = t.u8(tsSize)
			t.set8(tsSize, 99)
		}
	}
	clib.Qsort(tc.ship[:tc.n], 0x139, func(x, y *tship) int {
		a, c := x.u8(tsSize), y.u8(tsSize)
		switch {
		case a == c:
			return 0
		case a < c:
			return 1
		}
		return -1
	})
	P = g.tsp(0)
	P.set8(tsSize, 3)
	for i := int16(1); i < tc.n; i++ {
		if t := g.tsp(i); t.satellite() {
			t.set8(tsSize, baseSize)
		}
	}
	lastBase := int16(-1)
	var nAtt, nDef uint8
	for i := int16(1); i < tc.n; i++ {
		t := g.tsp(i)
		if t.satellite() {
			if defX < 0x28 {
				t.set8(tsX, P.u8(tsX)+0xb)
			} else {
				t.set8(tsX, P.u8(tsX)-4)
			}
			t.set8(tsY, P.u8(tsY)+1)
			lastBase = i
			continue
		}
		step := func() {
			if tc.att == int16(s.PlayerNum) || tc.netFlag != 0 {
				defX += 3
			} else {
				defX -= 3
			}
		}
		if int16(t.u8(tsOwner)) == tc.att {
			if nAtt&1 != 0 {
				if attY1 < 4 {
					if int16(t.u8(tsOwner)) == int16(s.PlayerNum) || tc.netFlag != 0 {
						attX -= 3
					} else {
						attX += 3
					}
					attY1, attY2 = 0x22, 0x22
				}
				attY1 -= hullSquares(t.u8(tsSize)) + 1
				t.set8(tsX, uint8(attX))
				t.set8(tsY, uint8(attY1))
			} else {
				t.set8(tsX, uint8(attX))
				t.set8(tsY, uint8(attY2))
				attY2 += hullSquares(t.u8(tsSize)) + 1
				if attY2 > 0x40 {
					if tc.att == int16(s.PlayerNum) || tc.netFlag != 0 {
						attX -= 3
					} else {
						attX += 3
					}
					attY1, attY2 = 0x22, 0x22
				}
			}
			nAtt++
			t.set8(tsFacing, attFace)
			continue
		}
		if nDef&1 != 0 {
			for {
				if defY1 < 4 {
					step()
					defY1, defY2 = 0x22, 0x22
				}
				defY1 -= hullSquares(t.u8(tsSize)) + 1
				if g.shipCanDeployAt(int16(defX), defY2, lastBase) {
					break
				}
			}
			t.set8(tsX, uint8(defX))
			t.set8(tsY, uint8(defY1))
		} else {
			t.set8(tsX, uint8(defX))
			t.set8(tsY, uint8(defY2))
			for {
				defY2 += hullSquares(t.u8(tsSize)) + 1
				if defY2 > 0x40 {
					step()
					defY1, defY2 = 0x22, 0x22
				}
				if g.shipCanDeployAt(int16(defX), defY2, lastBase) {
					break
				}
			}
		}
		nDef++
		t.set8(tsFacing, defFace)
	}
}

// shipCanDeployAt is Ship_Can_Deploy_At_ (0x48f49): not on the planet or
// the base next to it. Without a colony every square will do.
func (g *Game) shipCanDeployAt(x, y, base int16) bool {
	s := &g.state
	tc := g.tc
	if tc.colony == -1 {
		return true
	}
	var span int32
	switch s.Planets[s.Colonies[tc.colony].Planet].Size {
	case 0:
		span = 3
	case 1:
		span = 4
	case 2:
		span = 6
	case 3:
		span = 7
	case 4:
		span = 8
	}
	ok := true
	P := g.tsp(0)
	if int32(P.u8(tsX)) <= int32(x)+2 && int32(x) <= int32(P.u8(tsX))+span &&
		int32(y) <= span+int32(P.u8(tsY)) && int32(P.u8(tsY)) <= int32(y)+2 {
		ok = false
	}
	if base > -1 {
		B := g.tsp(base)
		if int32(B.u8(tsX)) <= int32(x)+2 && int32(x) <= int32(B.u8(tsX))+3 &&
			int32(y) <= int32(B.u8(tsY))+3 && int32(B.u8(tsY)) <= int32(y)+2 {
			ok = false
		}
	}
	return ok
}

// checkForWinner is Check_For_Winner_ (0x4545b): 1 the attacker won, -1
// the defender, 0x29a both gone (or a draw after 50 turns with no
// defender), 0 go on.
func (g *Game) checkForWinner() int16 {
	tc := g.tc
	r := int16(0)
	var att, def int16
	if g.tsp(0).u8(tsGone) != 5 && g.doesCombatPlanetHaveDefenses() != 0 {
		def = 1
	}
	for i := int16(1); i < tc.n; i++ {
		t := g.tsp(i)
		if t.u8(tsGone) == 5 || t.u8(tsBoarded) != 0 {
			continue
		}
		if int16(t.u8(tsOwner)) == tc.att {
			att++
		} else {
			def++
		}
	}
	if tc.turn > 0x32 {
		if def < 1 {
			r = 0x29a
		} else {
			r = -1
		}
	}
	switch {
	case att < 1:
		if def < 1 {
			r = 0x29a
		} else {
			r = -1
		}
	case def < 1:
		r = 1
	}
	return r
}

// calcCurrentSpeed is Calc_Current_Speed_ (0x4528f): combat speed from the
// design, the drive's state and tractor beams holding the ship.
func (g *Game) calcCurrentSpeed(i int16) int16 {
	s := &g.state
	tc := g.tc
	t := g.tsp(i)
	v := int32(0)
	if !t.satellite() && !g.shipInBlackHole(i) {
		sp := int16(t.u8(tsCSpeed))
		if testBit(t[tsSpecials:tsSpecials+5], 2) && !testBit(t[tsDamaged:tsDamaged+5], 2) {
			sp = int16(t.u8(tsCSpeed)) + 5
		}
		if t.u8(tsOwner) < 10 && t.u8(tsDrive) == 0 {
			// community patch 1.51 (0x45315 -> 0x40149d): a ship without
			// drive has no drive hits; only trans-dimensional moves it.
			v = 0
		} else if t.u8(tsOwner) < 10 {
			half := int32(t.i16(tsDriveMax)) / 2
			v = (int32(t.i16(tsDriveCur)) - half) * int32(sp) / (int32(t.i16(tsDriveMax)) - half)
		} else {
			v = int32(sp) * int32(t.i16(tsDriveCur)) / int32(t.i16(tsDriveMax))
		}
		if int16(v) < 0 {
			v = 0
		}
		if t.u8(tsOwner) < 8 && s.Players[t.u8(tsOwner)].Picks[save.PickTransDimensional] != 0 {
			v += 4
		}
		sv := int16(v)
		if sv >= 1 {
			var held int16
			for j := int16(0); j < tc.n; j++ {
				if int16(g.tsp(j).u8(0x42)) != i&0xff || !g.combatShipIsFunctional(j) {
					continue
				}
				J := g.tsp(j)
				for k := 0; k < 8; k++ {
					if J.i16(tw(k, twWeapon)) == 0x27 {
						held += int16(J.u8(tw(k, twCount)))
					}
				}
			}
			lim := int32(t.u8(tsSize)) + 1
			if int32(held) < lim {
				v = int32(sv) - int32(held)*int32(sv)/lim
				if int16(v) < 1 {
					v = 1
				}
			} else {
				v = 0
			}
		}
	}
	if int16(v) < 1 {
		t.set8(0x4b, 0)
	}
	return int16(v)
}

// endOfTurnBookkeeping is End_Of_Turn_Bookeeping_ (0x4a575).
func (g *Game) endOfTurnBookkeeping() {
	tc := g.tc
	g.rechargeShields()
	g.repairAllCombatShips()
	if g.tsp(0).u8(tsGone) == 0 && tc.turn%10 == 0 {
		P := g.tsp(0)
		for k := 0; k < 8; k++ {
			if g.wtype(P.i16(tw(k, twWeapon))) == 4 {
				P.set16(tw(k, twAmmo), 1)
			}
		}
	}
}

// rechargeShields is Recharge_Shields_ (0x4a790): 30% (100% with special
// 0x1e) of the arc maximum, a point at a time round the arcs.
func (g *Game) rechargeShields() {
	tc := g.tc
	for i := int16(1); i < tc.n; i++ {
		t := g.tsp(i)
		if t.u8(tsShield) == 0 || g.shipInStasis(i) || t.i16(tsShieldCur) <= 0 {
			continue
		}
		pct := int32(3)
		if g.combatShipHasSpecial(i, 0x1e) {
			pct = 10
		}
		left := int32(t.i16(tsShieldMax)) * pct / 10
		more := true
		for left > 0 && more {
			more = false
			for k := 0; k < 4; k++ {
				if t.i16(tsArc+2*k) < t.i16(tsShieldMax) && left > 0 {
					more = true
					left--
					t.add16(tsArc+2*k, 1)
				}
			}
		}
	}
}

// repairAllCombatShips is Repair_All_Combat_Ships_ (0x4cfe7).
func (g *Game) repairAllCombatShips() {
	s := &g.state
	tc := g.tc
	attB := g.fleetEngineerBonus(tc.att)
	defB := g.fleetEngineerBonus(tc.def)
	if tc.att < 8 && s.Players[tc.att].Picks[save.PickCybernetic] != 0 {
		attB += 10
	}
	if tc.def < 8 && s.Players[tc.def].Picks[save.PickCybernetic] != 0 {
		defB += 10
	}
	for i := int16(0); i < tc.n; i++ {
		t := g.tsp(i)
		if t.u8(tsGone) != 0 || t.u8(tsBoarded) != 0 || g.shipInStasis(i) {
			continue
		}
		var v int16
		if g.combatShipHasSpecial(i, 3) {
			v = 0x14
		}
		b := defB
		if int16(t.u8(tsOwner)) == tc.att {
			b = attB
		}
		if v+b > 0 {
			g.repairCombatShip(i, v+b)
		}
	}
}

// fleetEngineerBonus is Get_Fleet_Engineer_Bonus_ (0x35e0c).
func (g *Game) fleetEngineerBonus(p int16) int16 {
	var best int16
	for i := int16(0); i < g.tc.n; i++ {
		t := g.tsp(i)
		if int16(t.u8(tsOwner)) == p && t.u8(tsBoarded) == 0 && t.u8(tsGone) == 0 && t.i16(tsOfficer) > -1 {
			best = max(best, g.engineerBonus(int(t.i16(tsOfficer))))
		}
	}
	return best
}

// repairCombatShip is Repair_Combat_Ship_ (0x35821): pct percent of the
// structure, then half that of the internal systems.
func (g *Game) repairCombatShip(i, pct int16) {
	t := g.tsp(i)
	if i == 0 {
		o := tsStructDmg
		if t.i16(tsStructDmg) < 1 {
			if t.i16(tsStruct) < 1 {
				return
			}
			o = tsStruct
		}
		v := int32(pct) * int32(t.i16(o)) / 100
		if v < 2 {
			v = 1
		}
		t.add16(o, -int16(v))
		return
	}
	str := t.i16(tsStruct)
	var armor int16
	if t.i16(tsID) > -1 {
		armor = int16(g.shipArmorHits(int(t.i16(tsID))))
	} else {
		pctA := int32(g.armorField(int(t.u8(tsArmor)), armorHitsPct))
		if !t.satellite() { // community patch 1.51 (0x358e0): an Antaran ship
			h := int32(g.hullI16(int(t.u8(tsSize)), hullHits))
			armor = int16(pctA*h/100) + int16(h)
		} else {
			base := int32(g.exeI16(0x1800fc))
			switch sz := t.u8(tsSize); {
			case sz < 2:
			case sz == 2:
				armor = int16(base) + int16(pctA*base/100)
			case sz == 3:
				armor = int16(base*pctA/100) + g.exeI16(0x180120)
			case sz == 4:
				armor = int16(pctA*base/100) + g.exeI16(0x180144)
			}
		}
		if g.combatShipHasSpecial(i, 0xe) {
			armor *= 3
		}
	}
	r := int16(int32(pct) * int32(str+armor) / 100)
	if r < 1 {
		r = 1
	}
	t.add16(tsStructDmg, -r)
	if t.i16(tsStructDmg) < 0 {
		over := t.i16(tsStructDmg)
		t.set16(tsStructDmg, 0)
		t.add16(tsArmorHits, -over)
		if armor < t.i16(tsArmorHits) {
			t.set16(tsArmorHits, armor)
		}
	}
	left := int16(int32(pct) * int32(g.maxTotalInternalSpace(i)) / 200)
	if int32(left) < 2 {
		left = 1
	}
	fix := func(cur, mx int) {
		if t.i16(cur) < t.i16(mx) && left > 0 {
			d := t.i16(mx) - t.i16(cur)
			if left < d {
				d = left
			}
			left -= d
			t.add16(cur, d)
		}
	}
	fix(tsDriveCur, tsDriveMax)
	fix(tsCompCur, tsCompMax)
	for k := 0; k < 8 && left > 0; k++ {
		if t.i16(tsCur+4+2*k) < t.i16(tsWpnSpace+2*k) {
			d := t.i16(tsWpnSpace+2*k) - t.i16(tsCur+4+2*k)
			if left < d {
				d = left
			}
			left -= d
			t.add16(tsCur+4+2*k, d)
		}
	}
	fix(tsShieldCur, tsShieldHP)
	for k := 0; k < 8 && left > 0; k++ {
		if t.i16(tsCur+0x24+2*k) < t.i16(tsSpcSpace+2*k) {
			d := t.i16(tsSpcSpace+2*k) - t.i16(tsCur+0x24+2*k)
			if left < d {
				d = left
			}
			left -= d
			// The original sets the special back to its maximum.
			t.set16(tsCur+0x24+2*k, t.i16(tsSpcSpace+2*k))
		}
	}
}

// incrementBhg is Increment_Bhg_ (0x4a4e4): three turns in a black hole
// destroy a ship.
func (g *Game) incrementBhg(p int16) {
	for i := int16(1); i < g.tc.n; i++ {
		t := g.tsp(i)
		if t.u8(tsGone) != 0 || int16(t.u8(tsOwner)) != p || g.shipInStasis(i) || !g.shipInBlackHole(i) {
			continue
		}
		t.set8(0x136, t.u8(0x136)+1)
		if t.u8(0x136) == 3 {
			g.destroyShip(i, int16(t.u8(tsX))*0x14, int16(t.u8(tsY))*0x14, 1)
		}
	}
}

// plasmaWebDamage is Plasma_Web_Damage_ (0x4a5ce): webbed ships of p take
// the web's strength on every arc, which then fades by 5.
func (g *Game) plasmaWebDamage(p int16) {
	for i := int16(0); i < g.tc.n; i++ {
		t := g.tsp(i)
		if t.u8(tsGone) != 0 || t.i16(0x43) <= 0 || int16(t.u8(tsOwner)) != p || g.shipInStasis(i) {
			continue
		}
		var dmg, abs int16
		for arc := int16(0); arc < 4; arc++ {
			d := t.i16(0x43)
			var ab int16
			g.applyShipDamage(i, &d, &ab, arc, 0, 0, 0, 0, 0)
			dmg += d
			abs += ab
		}
		if tc := g.tc; tc.onScreen == 1 && tc.superFast == 0 {
			// 0x4a691: the burn's damage rises over the ship.
			if tc.scr != nil && !g.shipCompletelyOnScreen(i) {
				g.must2(g.snapCenterCombatScreen(int16(t.u8(tsX)), int16(t.u8(tsY))))
			}
			x, y := (int16(t.u8(tsX))-tc.curX)*0x14, (int16(t.u8(tsY))-tc.curY)*0x14
			g.addDamageIndicatorForShip(x, y, i, 1, dmg, abs)
			g.drawDamageMessageQueueUntilDone()
		}
		if dead := g.isCombatShipDead(i); dead == 0 {
			v := t.i16(0x43) - 5
			if v < 0 {
				v = 0
			}
			t.set16(0x43, v)
		} else {
			g.destroyShip(i, int16(t.u8(tsX))*0x14, int16(t.u8(tsY))*0x14, dead)
		}
	}
}

// retreatShip is Retreat_Ship_ (0x45964). With the screen on, the ship
// first turns to its side's edge (Draw_Retreating_Ship_), which can draw
// missiles and defensive fire.
func (g *Game) retreatShip(i int16) {
	tc := g.tc
	if tc.onScreen == 1 && tc.superFast == 0 {
		if tc.scr != nil && !g.shipOnScreen(i) {
			T := g.tsp(i)
			g.must2(g.snapCenterCombatScreen(int16(T.u8(tsX)), int16(T.u8(tsY))))
		}
		g.drawRetreatingShip(i)
	}
	if g.retreatBlocked(i) {
		return
	}
	t := g.tsp(i)
	t.set8(tsGone, 5)
	t.set8(0x4b, 2)
	g.retargetMissilesAimedAt(i)
	g.removeContinuingEffectSpecials(i)
}

// drawRetreatingShip is Draw_Retreating_Ship_ (0x32d94): the view centred
// on the ship, which turns to face its side's edge (facing 8 for the
// attacker, 0 for the defender); the jump away is drawing only.
func (g *Game) drawRetreatingShip(i int16) {
	tc := g.tc
	T := g.tsp(i)
	face := int16(0)
	if int16(T.u8(tsOwner)) == tc.att {
		face = 8
	}
	if tc.scr != nil {
		g.must2(g.snapCenterCombatScreen(int16(T.u8(tsX)), int16(T.u8(tsY))))
	}
	if int16(T.u8(tsFacing)) != face {
		g.rotateShip(i, face)
	}
	// The test changes the record, so it runs off screen too.
	if blocked := g.retreatBlocked(i); tc.scr == nil || blocked {
		return
	}
	g.retreatJump(i)
}

// retreatBlocked is community patch 1.51's 0x4019c9, which replaces
// Is_Combat_Ship_Dead_ in Retreat_Ship_ and Draw_Retreating_Ship_: a dead or
// immobile ship stays (moves and order cleared), and so does one no longer
// retreating (+0x4b != 1, e.g. captured by assault shuttles meanwhile).
func (g *Game) retreatBlocked(i int16) bool {
	t := g.tsp(i)
	stuck := g.isCombatShipDead(i) != 0 || g.calcCurrentSpeed(i) <= 0
	if !stuck && t.u8(0x4b) == 1 {
		return false
	}
	if stuck {
		t.set8(0x3d, 0) // the low byte of the i16 moves left, as 1.51 does
	}
	// 1.51 clears the word at +0x4b, and specials 0..7 (+0x4c) with it;
	// the port clears the retreat order only.
	t.set8(0x4b, 0)
	return true
}

// retreatJump is the end of Draw_Retreating_Ship_ (0x32eaf..0x33151): the
// jump flash (CMBTSFX 0x2a) beside the ship, which slides off through a
// window at the flash's edge.
func (g *Game) retreatJump(i int16) {
	tc := g.tc
	sc := tc.scr
	e := g.e
	v := e.V
	T := g.tsp(i)
	// 0x3302f: the jump's sound, pitched down by the hull.
	g.pitchBendSound(g.moxPlaySound(0x34), (int(T.u8(tsSize))-2)*-0x9c4)
	att := int16(T.u8(tsOwner)) == tc.att
	k := int16(0x1f)
	if att {
		k = 0x1c
	}
	step1, step2 := k/3, k/4+1
	baseX, baseY := int16(T.u8(tsX))*0x14, int16(T.u8(tsY))*0x14
	p, err := g.cached(g.cmbtLBX(cmbtSfxLBX), 0x2a)
	g.must2(err)
	pw, ph := int16(p.Width), int16(p.Height)
	g.must2(g.drawShipToBitmap(i, 0, 0))
	_, _, bw, bh := sc.drawShipBitmap.ActualSize()
	w, h := int16(bw), int16(bh)
	sc.dontDrawShip = i
	cx, cy := g.shipCenterOffsets(i)
	cx += baseX - tc.curX*0x14
	cy += baseY - tc.curY*0x14
	v.SetPageOff()
	bx := w/2 + k + cx
	if att {
		bx = cx - (w/2 + k + pw)
	}
	px := max(bx, 4)
	if int32(pw)+int32(px) > 0x27f {
		px = 0x276 - pw
	}
	py := max(cy-ph/2, 4)
	if py > 0x15e {
		py = 0x15a - ph
	}
	sx1, sy1, sx2, sy2 := v.WinX1, v.WinY1, v.WinX2, v.WinY2
	x1, y1, x2, y2 := cx-w, py+0x35, px+0x21-3, py+ph
	if att {
		x1, x2, y2 = px+0x21, cx+w, cy+h
	}
	mult := int16(1)
	if tc.fastFX() {
		mult = 3
	}
	off := int16(0)
	for f := int16(0); f < int16(p.Frames); f += mult {
		d := step2 * mult
		if f < 5 {
			d = step1 * mult
		}
		e.MarkTime()
		g.must2(g.drawMainCombatScreen())
		v.ClipOn()
		v.SetWindow(0, 0, 0x27f, 0x15e)
		g.setAnimationFrame(p, int(f))
		g.must2(e.Draw(int(px), int(py), p))
		v.SetWindow(int(x1), int(y1), int(x2), int(y2))
		g.must2(g.drawShip(i, int(baseX-tc.curX*0x14+off), int(baseY-tc.curY*0x14)))
		g.must2(e.TogglePages())
		g.must2(e.ReleaseTime(1))
		if att {
			off -= d
		} else {
			off += d
		}
	}
	v.WinX1, v.WinY1, v.WinX2, v.WinY2 = sx1, sy1, sx2, sy2
}

// retargetMissilesAimedAt is Retarget_Missiles_Aimed_At_ (0x3891e).
func (g *Game) retargetMissilesAimedAt(i int16) {
	for m := int16(0); m < nMissiles; m++ {
		M := g.msl(m)
		if M.i16(tmWeapon) <= 0 || M[tmAtMis] != 0 {
			continue
		}
		if M.i16(tmShip) == i && M.i16(tmTarget) == i {
			g.killMissile(m)
		} else if M.i16(tmTarget) == i {
			g.retargetMissile(m)
		}
	}
}

// removeContinuingEffectSpecials is Remove_Continuing_Effect_Specials_ (0x4588c).
func (g *Game) removeContinuingEffectSpecials(i int16) {
	for j := int16(0); j < g.tc.n; j++ {
		t := g.tsp(j)
		if int16(t.u8(tsStasis)) == i&0xff {
			g.removeStasisBeam(j)
		}
		if int16(t.u8(0x137)) == i&0xff {
			t.set8(0x136, 0)
			t.set8(0x137, 0xff)
		}
		if int16(t.u8(0x42)) == i&0xff {
			t.set8(0x42, 0xff)
		}
	}
}

// initShipForStartOfTurn is Init_Ship_For_Start_Of_Turn_ (0x42b70).
func (g *Game) initShipForStartOfTurn(i int16) {
	tc := g.tc
	t := g.tsp(i)
	captured := int16(b2i(g.beingCaptured(i)))
	t.set8(0x48, 0)
	if captured != 0 {
		t[0x48] |= 0x10
	}
	t.set8(0x49, 0)
	switch {
	case t.u8(0x4b) == 1:
		if tc.netFlag != 0 && tc.initOrder != 0 {
			g.waitUntilNetOpponentFinished(true)
		}
		g.retreatShip(i)
	case t.u8(tsBoarded) == 1:
		t.set16(0x3d, 0)
		t.set8(tsCloak, 2)
		for k := 0; k < 8; k++ {
			t.set8(tw(k, twLeft), 0)
			t.set8(tw(k, twFired), 0)
		}
	default:
		var held int16
		for j := int16(1); j < tc.n; j++ {
			if int16(g.tsp(j).u8(tsStasis)) == i&0xff {
				held++
			}
		}
		sp := g.calcCurrentSpeed(i)
		t.set16(tsSpeed, sp)
		t.set16(0x3d, sp)
		if t.u8(tsCloak) == 2 {
			t.set8(tsCloak, 3)
		}
		if t.u8(tsCloak) == 6 {
			t.set8(tsCloak, 7)
		}
		for k := 0; k < 8; k++ {
			if t.i16(tw(k, twWeapon)) == 0x20 {
				if int16(t.u8(tw(k, twCount))) >= held {
					// The original subtracts the capture flag, not the count held.
					t.set8(tw(k, twLeft), t.u8(tw(k, twCount))-uint8(captured))
				} else {
					t.set8(tw(k, twLeft), 0)
					held -= int16(t.u8(tw(k, twCount)))
				}
			} else {
				t.set8(tw(k, twLeft), t.u8(tw(k, twCount)))
			}
			if int8(t.u8(tw(k, twOn))) != -1 {
				t.set8(tw(k, twOn), 1)
			}
			if g.combatShipHasSpecial(i, 0x10) && g.wtype(t.i16(tw(k, twWeapon))) == 0 && t.u8(tw(k, twFired)) != 1 {
				t.set8(tw(k, twFired), 2)
			}
			if g.combatShipHasSpecial(i, 0xc) && g.wtype(t.i16(tw(k, twWeapon))) == 1 && t.u8(tw(k, twFired)) != 1 {
				t.set8(tw(k, twFired), 2)
			}
			if w := t.i16(tw(k, twWeapon)); w > 0 && g.wtype(w) == 2 && t.i16(tw(k, twAmmo)) > 0 {
				t.add16(tw(k, twAmmo), -1)
			}
		}
	}
}

// nextShip is Next_Ship_ (0x455a4): the next ship in the turn order; 1 when
// there is none (or the battle is over).
func (g *Game) nextShip() int16 { return g.nextShipFrom(false) }

// nextShipFrom is Next_Ship_, from the current ship itself when stay is set
// (community patch 1.51's all-wait, 0x41abca mode 3) and it may move.
func (g *Game) nextShipFrom(stay bool) int16 {
	tc := g.tc
	cur, ptr := tc.cur, tc.curPtr
	if g.checkForWinner() != 0 {
		return 1
	}
	waits := g.allWaitSides()
	if stay && ptr >= 0 && g.allWaitMayMove(tc.order[ptr], waits) {
		return g.nextShipTail(ptr, cur)
	}
	planet := false
	for {
		tc.curPtr++
		p := tc.curPtr
		if tc.order[tc.curPtr] == -10 {
			p = ptr
			if tc.curPtr != 0 && tc.curPtr != 1 {
				tc.curPtr = 0
				p = tc.curPtr
			}
		}
		tc.curPtr = p
		if tc.order[tc.curPtr] == 0 && g.doesCombatPlanetHaveDefenses() != 0 {
			planet = true
		}
		o := tc.order[tc.curPtr]
		if ptr == tc.curPtr {
			break
		}
		if !(o == 0 && !planet) && o != -1 && tc.sideEnd[g.tsp(o).u8(tsOwner)] == 0 && g.combatShipIsFunctional(o) &&
			!g.allWaitSkips(o, waits) {
			break
		}
	}
	return g.nextShipTail(ptr, cur)
}

// nextShipTail is Next_Ship_ from 0x456cc on, the ship at _cur_ptr found:
// the view, the fields and the lockstep; cur and ptr are the ship before.
func (g *Game) nextShipTail(ptr, cur int16) int16 {
	tc := g.tc
	if tc.curPtr < 0 {
		tc.cur = -1
	} else {
		tc.cur = tc.order[tc.curPtr]
	}
	r := int16(0)
	if tc.cur < 0 {
		r = 1
		tc.curPtr, tc.cur = ptr, cur
	} else {
		if tc.scr != nil {
			if tc.initOrder != 0 || (!g.shipCompletelyOnScreen(tc.cur) && tc.startOfTurn != 0) {
				C := g.tsp(tc.cur)
				g.must2(g.snapCenterCombatScreen(int16(C.u8(tsX)), int16(C.u8(tsY))))
			}
			g.must2(g.addTacticalCombatFields())
		}
		g.assignCombatGrids(tc.cur)
		if tc.scr != nil {
			g.must2(g.refreshCombatScreenFull())
		}
		tc.autoCombat = tc.autoDef
		if int16(g.tsp(tc.cur).u8(tsOwner)) == tc.att {
			tc.autoCombat = tc.autoAtt
		}
		g.nextShipNet()
	}
	tc.mslWarning = 0
	if tc.scr != nil && tc.superFast != 0 {
		g.must2(g.drawSuperFastScreen())
	}
	return r
}

// doCombatTurn is Do_Combat_Turn_ (0x42f7f): every ship of p moves and
// fires once (phase 1: only those with the extra-turn device 0x22).
func (g *Game) doCombatTurn(p, phase int16) {
	s := &g.state
	tc := g.tc
	tc.playerMode[tc.att] = g.getPlayerMode(tc.att)
	tc.playerMode[tc.def] = g.getPlayerMode(tc.def)
	for k := range tc.sideEnd {
		tc.sideEnd[k] = 0
	}
	tc.allWait = [15]uint8{} // community patch 1.51 (0x4319c, 0x408746)
	eligible := func(i int16) bool {
		return (phase == 0 || g.combatShipHasSpecial(i, 0x22)) && (tc.initOrder != 0 || int16(g.tsp(i).u8(tsOwner)) == p)
	}
	for i := int16(0); i < tc.n; i++ {
		t := g.tsp(i)
		if t.u8(tsGone) == 0 && t.u8(tsBoarded) == 0 && !g.shipInStasis(i) && eligible(i) {
			g.initShipForStartOfTurn(i)
		}
	}
	n, boarded := int16(0), int16(0)
	for i := int16(0); i < tc.n; i++ {
		t := g.tsp(i)
		if t.u8(tsGone) != 0 || g.shipInStasis(i) || !eligible(i) {
			continue
		}
		if t.u8(tsBoarded) == 0 {
			tc.order[n] = i
		} else {
			boarded++
			tc.order[n] = -1
		}
		n++
	}
	if n == boarded {
		return
	}
	if tc.initOrder != 0 {
		clib.Qsort(tc.order[:n], 2, func(a, b *int16) int { return g.qsortShipInitiative(*a, *b) })
	}
	tc.order[n] = -10
	for n++; n <= 0xd1; n++ {
		tc.order[n] = -1
	}
	tc.curPtr = -1
	tc.startOfTurn = 1
	done := g.nextShip()
	if done != 1 {
		if tc.scr != nil {
			g.e.AssignAutoFunction(g.drawMainCombatScreen, 2)
		}
		tc.skan, tc.board = 0, 0
		for i := int16(1); i < tc.n; i++ {
			t := g.tsp(i)
			// community patch 1.51 (0x4320b): satellites turn, driveless ships not.
			if t.satellite() && t.u8(tsGone) == 0 && !g.shipInStasis(i) {
				g.rotateShip(i, int16((int32(t.u8(tsFacing))+2)%0x10))
			}
		}
		tc.startOfTurn = 0
		for tc.order[tc.curPtr] > -1 && done == 0 {
			if tc.scr != nil {
				done = g.combatScreenPass(p)
				continue
			}
			cur := g.tsp(tc.cur)
			owner := cur.u8(tsOwner)
			computer := (owner > 7 || s.Players[owner].Objective != save.ObjectiveHuman || tc.autoCombat == 1) && tc.takeControl == 0
			if computer && g.tacRemote() {
				computer = false // 0x4331c: the other machine moves its ships
			}
			if computer {
				g.tacticalNetAutoSend()
				g.doAutoShipTurn()
				tc.order[tc.curPtr] = -1
				done = g.nextShip()
				g.tacWaitIf()
				continue
			}
			tc.cur = tc.order[tc.curPtr]
			if g.shipInStasis(tc.cur) {
				tc.order[tc.curPtr] = -1
				done = g.nextShip()
				continue
			}
			if tc.initOrder == 0 && int16(g.tsp(tc.cur).u8(tsOwner)) != p {
				continue
			}
			if g.tacRemote() {
				done = g.netTacticalReplay(p)
				continue
			}
			in := tacInputDone
			if g.isCombatShipDead(tc.cur) == 0 {
				in = tc.ui.HumanInput(g)
			}
			switch in {
			case tacInputAuto:
				g.tacSend(tacAutoField, 0, 0, 0)
				if int16(g.tsp(tc.cur).u8(tsOwner)) == tc.att {
					tc.autoAtt = 1 - tc.autoAtt
					tc.autoCombat = tc.autoAtt
				} else {
					tc.autoDef = 1 - tc.autoDef
					tc.autoCombat = tc.autoDef
				}
				if tc.autoCombat == 0 {
					tc.superFast = 0
				}
				g.tacWaitIf()
			case tacInputDone:
				g.tacSend(tacDone, 0, 0, 0)
				g.tsp(tc.cur)[0x48] |= 4
				tc.order[tc.curPtr] = -1
				done = g.nextShip()
				g.tacWaitIf()
			}
		}
		for i := int16(0); i < tc.n; i++ {
			t := g.tsp(i)
			if t.u8(tsGone) == 0 && t.u8(tsBoarded) == 0 && !g.shipInStasis(i) && (tc.initOrder != 0 || int16(t.u8(tsOwner)) == p) {
				t.set16(tsAbsorbed, 0)
			}
		}
		if phase == 0 {
			g.seekingMissiles(p)
		}
		for i := int16(0); i < tc.n; i++ {
			t := g.tsp(i)
			if t.u8(tsGone) != 0 || t.u8(tsBoarded) != 0 || g.shipInStasis(i) {
				continue
			}
			for k := 0; k < 8; k++ {
				if t.u8(tw(k, twFired)) == 2 {
					t.set8(tw(k, twFired), 1)
				}
			}
			if t.u8(tsCloak) == 3 {
				g.drawCloak(i, 1)
				t.set8(tsCloak, 1)
			}
			if t.u8(tsCloak) == 4 {
				t.set8(0x41, t.u8(0x41)-1)
				if t.u8(0x41) == 0 {
					g.drawCloak(i, 5)
					g.drawCloak(i, 1)
					t.set8(tsCloak, 1)
				}
			}
			if t.u8(tsCloak) == 7 {
				g.drawCloak(i, 4)
				t.set8(tsCloak, 4)
				for m := int16(0); m < nMissiles; m++ {
					M := g.msl(m)
					if M.i16(tmWeapon) > 0 && M.i16(tmTarget) == i && M.i16(tmShip) != i {
						g.retargetMissile(m)
					}
				}
			}
			t.set8(0xb1, uint8(g.maxDefensiveFireRange(i)))
		}
		g.tacticalNetTurnEnd(p)
	}
}

// maxDefensiveFireRange is Max_Defensive_Fire_Range_ (0x37173): -1 without
// a plain beam left, else the reach for defensive fire (at most 0x18).
func (g *Game) maxDefensiveFireRange(i int16) int16 {
	t := g.tsp(i)
	r := int16(-1)
	for k := 0; k < 8; k++ {
		w := t.i16(tw(k, twWeapon))
		mods := t.i16(tw(k, twMods))
		if w > 0 && g.wtype(w) == 0 && mods&2 == 0 && mods&4 == 0 && t.u8(tw(k, twLeft)) != 0 {
			r = 0x18
		}
	}
	if r <= 0 {
		return r
	}
	for j := int16(1); j < g.tc.n; j++ {
		J := g.tsp(j)
		if t.u8(tsOwner) == J.u8(tsOwner) || J.u8(tsBoarded) != 0 || J.u8(tsGone) != 0 {
			continue
		}
		c := g.moveCost(j, int16(t.u8(tsX)), int16(t.u8(tsY))) - g.calcCurrentSpeed(j)
		if c < 1 {
			c = 1
		}
		if c < r {
			r = c
		}
	}
	return r
}

// shipInitiativeValue is Ship_Initiative_Value_ (0x42e66): speed plus a
// tenth of the offence, −5 for an empty slot.
func (g *Game) shipInitiativeValue(i int16) int16 {
	if i == -1 {
		return -5
	}
	t := g.tsp(i)
	return t.i16(tsSpeed) + int16(int32(t.i16(tsOcv))/10)
}

// qsortShipInitiative is qsort_ship_initiative_ (0x42e9c): fastest first.
func (g *Game) qsortShipInitiative(a, b int16) int {
	va, vb := g.shipInitiativeValue(a), g.shipInitiativeValue(b)
	switch {
	case va == vb:
		return 0
	case va > vb:
		return -1
	}
	return 1
}
