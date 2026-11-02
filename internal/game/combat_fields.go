package game

import (
	"github.com/openorion2/openorion2/internal/engine"
)

// The combat screen's fields, help list, cursors and view position
// (COMBAT1.C, COMBINIT.C). Spec: docs/screens/combat-screen.md.

// Hot keys of the combat fields (strings in the image).
const (
	cmbtKeyNone     = 0x178e18 // ""
	cmbtKeyTake     = 0x178e19 // take control
	cmbtKeyAuto     = 0x178e1b
	cmbtKeyScan     = 0x178e1d
	cmbtKeyBoard    = 0x178e1f
	cmbtKeyFlee     = 0x178e21
	cmbtKeyWait     = 0x178e23
	cmbtKeyDone     = 0x178e25
	cmbtKeyTurn     = 0x178e27 // end of turn
	cmbtKeySuper    = 0x178e29 // super fast
	cmbtKeyInfo     = 0x178e2b // the options panel
	cmbtKeyInfoBack = 0x178e2d
	cmbtKeyCenter   = 0x178e2f
	weaponHelpID    = 0x0a     // i16 in a weapon row
	specialHelpIDs  = 0x17eee6 // i16, 0x2f-byte rows by special
)

func (g *Game) cmbtKey(a uint32) string { return cStringAt(g.exe, a) }

// addTacticalCombatFields is Add_Tactical_Combat_Fields_ (0x465d0): no
// fields while the side plays itself, else the grid, the radar, the panel
// buttons, or the options panel's switches.
func (g *Game) addTacticalCombatFields() error {
	e := g.e
	s := &g.state
	tc := g.tc
	sc := tc.scr
	side := int16(0)
	if tc.cur >= 0 && int16(g.tsp(tc.cur).u8(tsOwner)) != tc.att {
		side = 1
	}
	g.setCombatHelpFields()
	auto := tc.autoAtt
	if side == 1 {
		auto = tc.autoDef
	}
	if auto == 1 || (g.gameType != 1 && tc.cur >= 0 && g.tsp(tc.cur).u8(tsOwner) != uint8(s.PlayerNum) && tc.takeControl == 0) {
		for i := range sc.missileField {
			sc.missileField[i] = -1000
		}
		sc.takeControlBtn, sc.spaceField, sc.mapField, sc.mslWarningBtn = -1000, -1000, -1000, -1000
		for _, f := range []*int{&sc.speedx2Btn, &sc.legalBoxesBtn, &sc.mslImpactBtn, &sc.selfDestBtn, &sc.shieldsBtn,
			&sc.gridBtn, &sc.scanField, &sc.boardField, &sc.fleeField, &sc.waitField, &sc.doneField, &sc.endOfTurnBtn} {
			*f = -100
		}
		for i := range sc.weaponBtn {
			sc.weaponBtn[i] = -1000
		}
		sc.centerBtn, sc.weaponsField, sc.specialsField, sc.infoField, sc.superFastBtn = -1000, -1000, -1000, -1000, -1000
		return nil
	}
	e.ClearFields()
	var err error
	add := func(f *int, n int, er error) {
		if err == nil {
			*f, err = n, er
		}
	}
	none := g.cmbtKey(cmbtKeyNone)
	infoY, infoKey := 0x1c0, uint32(cmbtKeyInfo)
	if tc.orderInfo == 1 {
		if err := g.addMissileFields(); err != nil {
			return err
		}
		n, er := e.AddHotKey(g.cmbtKey(cmbtKeyTake))
		add(&sc.takeControlBtn, n, er)
		n, er = e.AddGridField(0, 0, 10, 10, 0x40, 0x24, &sc.xpos, &sc.ypos, 0)
		add(&sc.spaceField, n, er)
		if tc.mslWarning == 1 {
			n, er = e.AddHiddenField(0x1f9, 0x172, 0x26e, 0x1d0, none, 0)
			add(&sc.mslWarningBtn, n, er)
			sc.mapField = -1000
		} else {
			n, er = e.AddGridField(500, 0x16c, 1, 1, 0x79, 0x66, &sc.mapX, &sc.mapY, 0)
			add(&sc.mapField, n, er)
			sc.mslWarningBtn = -1000
		}
		n, er = e.AddMultiButtonField(0x72, 0x169, none, sc.weaponsBtn, &sc.statusDisplay, 0, none, 0x29)
		add(&sc.weaponsField, n, er)
		n, er = e.AddMultiButtonField(0xbb, 0x169, none, sc.specialsBtn, &sc.statusDisplay, 1, none, 0x29)
		add(&sc.specialsField, n, er)
		n, er = e.AddRadioButtonField(0x111, 0x16c, none, sc.autoBtn, &sc.autoBtnValue, g.cmbtKey(cmbtKeyAuto), 0x28)
		add(&sc.autoField, n, er)
		n, er = e.AddRadioButtonField(0x151, 0x16c, none, sc.scanBtn, &tc.skan, g.cmbtKey(cmbtKeyScan), 0x28)
		add(&sc.scanField, n, er)
		n, er = e.AddRadioButtonField(0x111, 0x188, none, sc.boardBtn, &tc.board, g.cmbtKey(cmbtKeyBoard), 0x28)
		add(&sc.boardField, n, er)
		n, er = e.AddButtonField(0x151, 0x188, none, sc.fleeBtn, g.cmbtKey(cmbtKeyFlee), 0x28)
		add(&sc.fleeField, n, er)
		n, er = e.AddButtonField(0x111, 0x1a4, none, sc.waitBtn, g.cmbtKey(cmbtKeyWait), 0x28)
		add(&sc.waitField, n, er)
		n, er = e.AddButtonField(0x151, 0x1a4, none, sc.doneBtn, g.cmbtKey(cmbtKeyDone), 0x28)
		add(&sc.doneField, n, er)
		n, er = e.AddHotKey(g.cmbtKey(cmbtKeyTurn))
		add(&sc.endOfTurnBtn, n, er)
		sc.legalBoxesBtn, sc.mslImpactBtn, sc.speedx2Btn, sc.shieldsBtn, sc.gridBtn, sc.selfDestBtn = -100, -100, -100, -100, -100, -100
		human := func(p int16) bool { return p < 8 && s.Players[p].Objective == 100 }
		if human(tc.att) && human(tc.def) {
			sc.superFastBtn = -1000
		} else {
			n, er = e.AddHotKey(g.cmbtKey(cmbtKeySuper))
			add(&sc.superFastBtn, n, er)
		}
	} else {
		for i := range sc.missileField {
			sc.missileField[i] = -1000
		}
		sc.takeControlBtn, sc.spaceField = -100, -100
		n, er := e.AddGridField(500, 0x16c, 1, 1, 0x79, 0x66, &sc.mapX, &sc.mapY, 0)
		add(&sc.mapField, n, er)
		sc.mslWarningBtn, sc.weaponsField, sc.specialsField = -100, -100, -100
		for _, b := range []struct {
			f  *int
			y1 int
		}{{&sc.mslImpactBtn, 0x17f}, {&sc.speedx2Btn, 399}, {&sc.legalBoxesBtn, 0x19f}, {&sc.shieldsBtn, 0x1af}, {&sc.gridBtn, 0x1bf}} {
			n, er = e.AddHiddenField(0x108, b.y1, 0x197, b.y1+0xd, none, 0x29)
			add(b.f, n, er)
		}
		n, er = e.AddButtonField(0x11b, 0x15c, none, sc.selfDestruct, none, 0x28)
		add(&sc.selfDestBtn, n, er)
		sc.autoField, sc.scanField, sc.boardField, sc.fleeField, sc.waitField, sc.doneField, sc.endOfTurnBtn = -100, -100, -100, -100, -100, -100, -100
		infoY, infoKey = 0x12d, cmbtKeyInfoBack
		sc.superFastBtn = -1000
	}
	n, er := e.AddButtonField(0x12a, infoY, none, sc.infoBtn, g.cmbtKey(infoKey), 0x28)
	add(&sc.infoField, n, er)
	for k := 0; k < 8; k++ {
		n, er = e.AddHiddenField(0x78, 0x17b+0xb*k, 0xff, 0x185+0xb*k, none, 0x29)
		add(&sc.weaponBtn[k], n, er)
	}
	n, er = e.AddHiddenField(0xf, 0x17b, 100, 0x1cc, g.cmbtKey(cmbtKeyCenter), 0x29)
	add(&sc.centerBtn, n, er)
	return err
}

// addMissileFields is Add_Missile_Fields_ (0x45ea7): a field over each
// missile stack in view, and its cursor region.
func (g *Game) addMissileFields() error {
	e := g.e
	tc := g.tc
	sc := tc.scr
	v := e.V
	sc.missileCount = 0
	v.SetWindow(0, 0, 0x27f, 0x15e)
	v.ClipOn()
	defer func() { v.ClipOff(); v.ResetWindow() }()
	for m := int16(0); m < nMissiles; m++ {
		M := g.msl(m)
		r := &sc.mouseGrid[0x241+int(m)]
		if M.i16(tmWeapon) < 1 || !g.onCombatScreen((M.i16(tmX)-8)/0x14, (M.i16(tmY)-8)/0x14) {
			sc.missileField[m] = -1000
			*r = engine.MouseRegion{}
			continue
		}
		x := int(M.i16(tmX)) - int(tc.curX)*0x14
		y := int(M.i16(tmY)) - int(tc.curY)*0x14
		f, err := e.AddClippedHiddenField(x-0x10, y-0x10, x-8, y-8, g.cmbtKey(cmbtKeyNone), 0)
		if err != nil {
			return err
		}
		sc.missileField[m] = f
		*r = engine.MouseRegion{Image: 0xc, Center: 0, X1: x - 0x10, Y1: y - 0x10, X2: x - 8, Y2: y - 8}
		sc.missileCount++
	}
	return nil
}

// setCombatHelpFields is Set_Combat_Help_Fields_ (0x4602a).
func (g *Game) setCombatHelpFields() {
	tc := g.tc
	sc := tc.scr
	var list []engine.HelpEntry
	if tc.orderInfo == 1 {
		list = []engine.HelpEntry{
			help(0x269, 0x124, 0x1be, 0x176, 0x1d8), help(0x26f, 0x10f, 0x16a, 0x14a, 0x182), help(0x270, 0x14c, 0x16a, 0x18a, 0x182),
			help(0x271, 0x10f, 0x185, 0x14a, 0x19e), help(0x272, 0x14d, 0x185, 0x18a, 0x19e), help(0x273, 0x10f, 0x1a2, 0x14a, 0x1b9),
			help(0x274, 0x14d, 0x1a2, 0x18a, 0x1b9),
		}
	} else {
		list = []engine.HelpEntry{
			help(0x2b7, 0x11a, 0x15e, 0x180, 0x173), help(0x2b8, 0x108, 0x17f, 0x197, 0x18c), help(0x2b9, 0x108, 399, 0x197, 0x19c),
			help(0x2ba, 0x108, 0x19f, 0x197, 0x1ac), help(699, 0x108, 0x1af, 0x197, 0x1bc), help(700, 0x108, 0x1bf, 0x197, 0x1cc),
		}
	}
	mapID := 0x26c
	if tc.mslWarning == 1 {
		mapID = 0x26d
	}
	list = append(list, help(0x26a, 10, 0x168, 0x6a, 0x1d7), help(mapID, 0x1f6, 0x168, 0x272, 0x1d7),
		help(0x26e, 399, 0x168, 0x1ef, 0x1d7))
	var rows [8]int
	C := g.tsp(tc.cur)
	if sc.statusDisplay == 0 {
		for k := range rows {
			rows[k] = int(g.wpnI16(int(C.i16(tw(k, twWeapon))), weaponHelpID))
			if rows[k] == 0 {
				rows[k] = -1
			}
		}
	} else {
		n := 0
		for b := 0; b < 0x27 && n < 8; b++ {
			if testBit(C[tsSpecials:tsSpecials+5], b) {
				rows[n] = int(g.exeI16(uint32(specialHelpIDs + 0x2f*b)))
				n++
			}
		}
		for ; n < 8; n++ {
			rows[n] = -1
		}
	}
	for k := range rows {
		y := 0x17b + 0xb*k
		y2 := y + 0xa
		if k == 7 {
			y2 = 0x1d3
		}
		list = append(list, help(rows[k], 0x72, y, 0x108, y2))
	}
	list = append(list, help(0x275, 0x71, 0x168, 0x109, 0x1d7))
	g.e.SetHelpList(list)
}

// assignMouseImages is Assign_Mouse_Images_ (0x46f42): the cursor of each
// square (move arrows by facing, target, own ship) and, for big ships, the
// move regions shifted so the pointer sits at the ship's middle.
func (g *Game) assignMouseImages() {
	e := g.e
	s := &g.state
	tc := g.tc
	sc := tc.scr
	if g.gameType != 1 && g.tsp(tc.cur).u8(tsOwner) != uint8(s.PlayerNum) {
		e.SetMouseList([]engine.MouseRegion{{Image: int(sc.clockFrame), Center: 0, X1: 0, Y1: 0, X2: 0x27f, Y2: 0x1df}})
		return
	}
	if tc.orderInfo != 1 {
		e.SetMouseList(engine.MouseArrow)
		return
	}
	mg := sc.mouseGrid
	k := 1
	for x := 0; x < 0x20; x++ {
		for y := 0; y < 0x12; y++ {
			r := &mg[k]
			r.X1, r.Y1, r.X2 = x*0x14, y*0x14, x*0x14+0x13
			r.Y2 = y*0x14 + 0x13
			if y == 0x11 {
				r.Y2 = y*0x14 + 0x16
			}
			k++
		}
	}
	C := g.tsp(tc.cur)
	off := 0
	switch sz := C.u8(tsSize); {
	case sz == 0:
	case sz < 3:
		off = 10
	default:
		off = 0x14
	}
	for x := 0x1f; x >= 0; x-- {
		for y := 0x11; y >= 0; y-- {
			r := &mg[x*0x12+y+1]
			gx, gy := int(tc.curX)+x, int(tc.curY)+y
			v := tc.grid[gx][gy]
			switch {
			case tc.skan == 1:
				r.Image, r.Center = 0xd, 0xc
				if v < 0 || v >= tc.n {
					r.Image = 0xe
				}
			case tc.board == 1:
				r.Image, r.Center = 3, 0xc
				// The original tests the owners first and reads past the records
				// for a legal-move mark (2000); the image is 2 either way.
				if v < 0 || v >= tc.n || g.tsp(tc.cur).u8(tsOwner) == g.tsp(v).u8(tsOwner) {
					r.Image = 2
				}
			case v < 0 || v >= tc.n:
				switch {
				case v < 0:
					r.Image, r.Center = 2, 0xc
				case v == gridLegalBox:
					f := g.getFacing(int16(C.u8(tsX)), int16(C.u8(tsY)), tc.curX+int16(x), tc.curY+int16(y))
					r.Image, r.Center = int(f/2+4), 0xc
				}
			default:
				own := g.tsp(v).u8(tsOwner) == C.u8(tsOwner)
				switch {
				case own && v == tc.cur:
					r.Image, r.Center = 2, 0xc
				case !own && !g.shipInStasis(v):
					r.Image, r.Center = 0xc, 0xc
				default:
					r.Image, r.Center = 1, 0
				}
			}
		}
	}
	if off != 0 {
		for x := 0x1f; x >= 0; x-- {
			for y := 0x11; y >= 0; y-- {
				i := x*0x12 + y + 1
				r := &mg[i]
				if r.Image <= 3 || r.Image >= 0xc {
					continue
				}
				move := func(j int) {
					if j < len(mg) && (mg[j].Image < 4 || mg[j].Image > 0xb) {
						mg[j].X1, mg[j].X2, mg[j].Y1, mg[j].Y2 = 0, 0, 0, 0
					}
				}
				switch {
				case off == 0x14 && x < 0x1f && y < 0x11:
					mg[i+0x13].Image = r.Image
				case off == 10:
					move(i + 0x13)
					move(i + 1)
					move(i + 0x12)
				}
				r.X1, r.X2 = x*0x14+off, x*0x14+off+0x13
				r.Y1, r.Y2 = y*0x14+off, y*0x14+off+0x13
			}
		}
	}
	e.SetMouseList(mg)
}

// snapCenterCombatScreen is Snap_Center_Combat_Screen_ (0x49d09): the view
// centred on square (x, y), the star layers moved with it.
func (g *Game) snapCenterCombatScreen(x, y int16) error {
	tc := g.tc
	sc := tc.scr
	nx, ny := min(max(x-0x10, 0), 0x31), min(max(y-9, 0), 0x32)
	dx, dy := tc.curX-nx, tc.curY-ny
	sc.nearY += dy * -0xa0
	sc.midX += dx * -0x50
	sc.farX -= dx * 0x14
	sc.midY += dy * -0x50
	sc.farY -= dy * 0x14
	sc.nearX += dx * -0xa0
	wrap := func(v *int16, m int16) {
		for *v < 0 {
			*v += m
		}
		*v %= m
	}
	wrap(&sc.nearX, 0x280)
	wrap(&sc.nearY, 0x1e0)
	wrap(&sc.midX, 0x280)
	wrap(&sc.midY, 0x1e0)
	wrap(&sc.farX, 0x280)
	wrap(&sc.farY, 0x1e0)
	tc.curX, tc.curY = nx, ny
	if err := g.addTacticalCombatFields(); err != nil {
		return err
	}
	g.assignCombatGrids(tc.cur)
	return g.refreshCombatScreenFull()
}

func help(id, x1, y1, x2, y2 int) engine.HelpEntry {
	return engine.HelpEntry{ID: id, X1: x1, Y1: y1, X2: x2, Y2: y2}
}
