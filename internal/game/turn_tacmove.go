package game

// Movement on the combat grid (CMBTMOV1.C), the legal-move grid (COMBAT1.C)
// and missiles in flight (CMBTMIS.C, Seeking_Missiles_). Spec:
// docs/combat/tactical.md.

const (
	gridW, gridH = 0x51, 0x44
	gridLegal    = 2000
)

// getFacing is Get_Facing_ (0x3f5f1): the 16-way facing from one point to
// another.
func (g *Game) getFacing(x1, y1, x2, y2 int16) int16 {
	a := -int16(g.getAngle(int(x2-x1), int(y2-y1)))
	if a < 0 {
		a += 0x168
	}
	q := int16((int32(a<<2) + 0x2d) / 0x5a)
	return q % 0x10
}

// turnMultiplier is the cost of one facing step: 2, 1 with special 0x12,
// 0 with 0x11.
func (g *Game) turnMultiplier(i int16) int16 {
	switch {
	case g.combatShipHasSpecial(i, 0x12):
		return 1
	case g.combatShipHasSpecial(i, 0x11):
		return 0
	}
	return 2
}

// moveCost is Move_Cost_ (0x3f95f): squares (rounded up) plus turning.
func (g *Game) moveCost(i, x, y int16) int16 {
	t := g.tsp(i)
	face := int16(t.u8(tsFacing))
	sx, sy := int16(t.u8(tsX)), int16(t.u8(tsY))
	if x == sx && y == sy {
		return 0
	}
	sq := int16(int32(y-sy)*int32(y-sy)) + int16(int32(x-sx)*int32(x-sx))
	d := sq
	if sq != 0 {
		for d = 1; d < 0x51; d++ {
			if int32(sq) < int32(d)*int32(d)+1 {
				break
			}
		}
	}
	f := int32(g.getFacing(sx, sy, x, y)) - int32(face)
	if f < 0 {
		f = -f
	}
	if int16(f) > 8 {
		f = int32(int16(f)) - 0x10
		if f < 0 {
			f = -f
		}
	}
	return d + int16(int32(g.turnMultiplier(i))*f)
}

// assignCombatGrids is Assign_Combat_Grids_ (0x46cc8): which record covers
// each square, and for the current ship the squares it may move to.
func (g *Game) assignCombatGrids(cur int16) {
	s := &g.state
	tc := g.tc
	for x := range tc.grid {
		for y := range tc.grid[x] {
			tc.grid[x][y] = -1
		}
	}
	if tc.orderInfo == 1 {
		for i := int16(0); i < tc.n; i++ {
			t := g.tsp(i)
			if t.u8(tsGone) != 0 {
				continue
			}
			tc.grid[t.u8(tsX)][t.u8(tsY)] = i
			if i == cur && tc.skan != 1 && tc.board != 1 {
				continue
			}
			if i == 0 {
				span := 6
				if tc.colony != -1 {
					switch s.Planets[s.Colonies[tc.colony].Planet].Size {
					case 0:
						span = 3
					case 1:
						span = 4
					case 3:
						span = 7
					case 4:
						span = 8
					}
				}
				for a := 0; a < span; a++ {
					for b := 0; b < span; b++ {
						tc.grid[int(t.u8(tsX))+a][b+int(t.u8(tsY))] = 0
					}
				}
				continue
			}
			n := 0
			switch (int(t.u8(tsSize)) + 1) / 2 {
			case 1:
				n = 2
			case 2, 3:
				n = 3
			}
			for a := 0; a < n; a++ {
				for b := 0; b < n; b++ {
					tc.grid[int(t.u8(tsX))+a][int(t.u8(tsY))+b] = i
				}
			}
		}
	}
	if cur == tc.cur {
		g.setLegalMoves(cur, g.tsp(cur).i16(0x3d), 1)
		if tc.scr != nil {
			g.assignMouseImages()
		}
	}
}

// setLegalMoves is Set_Legal_Moves_ (0x4752f): free squares within reach
// (and, with cost 1, affordable) become 2000.
func (g *Game) setLegalMoves(i, reach, cost int16) {
	tc := g.tc
	if (g.combatShipHasSpecial(i, 0x21) || g.combatShipHasSpecial(i, 0x16)) && !g.usedTeleporter(i) && reach < 0x14 {
		reach = 0x14
	}
	t := g.tsp(i)
	span := int32(t.u8(tsSize)+1) / 2
	if span > 1 {
		span = 2
	}
	sx, sy := int32(t.u8(tsX)), int32(t.u8(tsY))
	for x := int32(0); x < gridW; x++ {
		for y := int32(0); y < gridH; y++ {
			dx, dy := sx-x, sy-y
			if dx < 0 {
				dx = -dx
			}
			if dy < 0 {
				dy = -dy
			}
			if tc.grid[x][y] != -1 || dx > int32(reach) || dy > int32(reach) {
				continue
			}
			tc.grid[x][y] = gridLegal
			if span != 0 {
				n := int32(2)
				if span == 2 {
					n = 3
				}
				for a := int32(0); a < n; a++ {
					for b := int32(0); b < n; b++ {
						if x+a > 0x50 || y+b > 0x43 {
							tc.grid[x][y] = -1
							continue
						}
						if v := tc.grid[x+a][y+b]; v != -1 && v != gridLegal && v != i {
							tc.grid[x][y] = -1
						}
					}
				}
			}
			if 0x51-span < x || 0x44-span < y {
				tc.grid[x][y] = -1
			}
			if tc.grid[x][y] == gridLegal && cost == 1 {
				if t.i16(0x3d) < g.moveCost(i, int16(x), int16(y)) {
					tc.grid[x][y] = -1
				}
				if g.combatShipHasSpecial(i, 0x21) && tc.grid[x][y] == -1 && !g.usedTeleporter(i) &&
					g.specialIsTurnedOn(i, 0x21) && g.rangeToXY(i, int16(x*0x14), int16(y*0x14)) < 0x15 {
					tc.grid[x][y] = gridLegal
				}
			}
		}
	}
}

// specialIsTurnedOn is Special_Is_Turned_On_ (0x3b004): the switch of
// special sp, counted among the ship's specials.
func (g *Game) specialIsTurnedOn(i int16, sp int) bool {
	t := g.tsp(i)
	k := 0
	for b := 0; b < 0x27; b++ {
		if b == sp {
			return t.u8(tsSpecialOn+k) == 1
		}
		if testBit(t[tsSpecials:tsSpecials+5], b) {
			k++
		}
	}
	return false
}

// missileStep is the pixels a missile chasing ship i moves while the ship
// moves step pixels.
func (g *Game) missileStep(i int16, M *tmissile, step int32) int16 {
	v := int16(5)
	if sp := g.tsp(i).i16(tsSpeed); sp != 0 {
		v = int16(step * int32(M[tmSpeed]) / int32(sp))
	}
	if v < 2 {
		v = 2
	}
	return v
}

// rotateShip is Rotate_Ship_ (0x3f628): one facing step at a time; missiles
// chasing it move, and enemies may fire defensively.
func (g *Game) rotateShip(i, face int16) {
	t := g.tsp(i)
	f := int8(t.u8(tsFacing))
	cx, cy := g.shipCenterOffsets(i)
	cx += int16(t.u8(tsX)) * 0x14
	cy += int16(t.u8(tsY)) * 0x14
	px, py := int16(t.u8(tsX))*0x14, int16(t.u8(tsY))*0x14
	d := int16(f) - face
	dir := int8(1)
	if (d < 0 && -d > 8) || (d >= 0 && d < 9) {
		dir = -1
	}
	if tc := g.tc; tc.scr != nil {
		saved := tc.scr.dontDrawShip
		tc.scr.dontDrawShip = i
		defer func() { tc.scr.dontDrawShip = saved }()
	}
	for int16(t.u8(tsFacing)) != face && t.u8(tsGone) == 0 {
		if g.tc.scr != nil {
			g.e.MarkTime()
		}
		f += dir
		if f < 0 {
			f += 0x10
		}
		f %= 0x10
		t.set8(tsFacing, uint8(f))
		var hit []int16
		for m := int16(0); m < nMissiles; m++ {
			M := g.msl(m)
			if M.i16(tmWeapon) > 0 && M.i16(tmTarget) == i {
				if g.moveMissile(m, cx, cy, g.missileStep(i, M, 5)) {
					hit = append(hit, m)
				}
			}
		}
		if g.combatDrawing() && g.shipOnScreen(i) {
			g.rotateShipFrame(i, px, py)
		}
		for _, m := range hit {
			if t.u8(tsGone) == 0 && g.msl(m).i16(tmTarget) == i {
				g.resolveMissile(m, cx, cy)
			}
		}
		if g.isCombatShipDead(i) == 0 {
			g.defensiveFireCheck(i, px, py, px, py)
		}
	}
}

// turnToFace is Turn_To_Face_ (0x3fa61): turn towards a square with the
// movement points left.
func (g *Game) turnToFace(i, x, y int16) {
	t := g.tsp(i)
	if t.i16(0x3d) == 0 {
		return
	}
	mult := g.turnMultiplier(i)
	face := g.getFacing(int16(t.u8(tsX)), int16(t.u8(tsY)), x, y)
	d := int16(t.u8(tsFacing)) - face
	dir := int32(-1)
	if (d < 0 && -d < 9) || (d >= 0 && d > 8) {
		dir = 1
	}
	if d < 0 {
		d += 0x10
	}
	if d > 8 {
		d = 0x10 - d
	}
	if int32(t.i16(0x3d)) < int32(d)*int32(mult) {
		v := (int32(t.i16(0x3d))+int32(mult)-1)/int32(mult)*dir + int32(t.u8(tsFacing))
		if int16(v) < 0 {
			v += 0x10
		}
		face = int16(v) % 0x10
		t.set16(0x3d, 0)
	} else {
		t.add16(0x3d, -d*mult)
	}
	g.rotateShip(i, face)
	g.assignCombatGrids(i)
	if g.tc.scr != nil {
		g.must2(g.refreshCombatScreenFull())
	}
}

// absoluteInterpolateLine is Absolute_Interpolate_Line (0x1344b0): one
// step from (*x, *y) along the line from (sx, sy) to (ex, ey).
func absoluteInterpolateLine(x, y *int16, sx, sy, ex, ey, step int16) {
	if step <= 0 || (*x == ex && *y == ey) {
		return
	}
	dx, dy := ex-sx, ey-sy
	if dx < 0 {
		dx = -dx
	}
	if dy < 0 {
		dy = -dy
	}
	if dy < dx {
		if sx < ex {
			*x += step
			if *x > ex {
				*x = ex
			}
		} else {
			*x -= step
			if *x < ex {
				*x = ex
			}
		}
		*y = sy + int16((int32(*x)-int32(sx))*(int32(ey)-int32(sy))/(int32(ex)-int32(sx)))
		return
	}
	if sy < ey {
		*y += step
		if *y > ey {
			*y = ey
		}
	} else {
		*y -= step
		if *y < ey {
			*y = ey
		}
	}
	*x = sx + int16((int32(*y)-int32(sy))*(int32(ex)-int32(sx))/(int32(ey)-int32(sy)))
}

// shipInLegalSquare is Ship_In_Legal_Square_ (0x3ea09).
func (g *Game) shipInLegalSquare(i, x, y, dx, dy int16) bool {
	ax := int32(x)
	if dx < x {
		ax = int32(x) + 0x14
	}
	ax -= int32(x) % 0x14
	ay := int32(y)
	if dy < y {
		ay = int32(y) + 0x14
	}
	ay -= int32(y) % 0x14
	v := g.tc.grid[int16(ax)/0x14][int16(ay)/0x14]
	return v == gridLegal || v == i
}

// missileSave keeps a missile's position while a move is only tested.
type missileSave struct {
	x, y, travel int16
	saved        bool
}

func (g *Game) saveMissile(sv *[nMissiles]missileSave, m int16) {
	if sv[m].saved {
		return
	}
	M := g.msl(m)
	sv[m] = missileSave{M.i16(tmX), M.i16(tmY), M.i16(tmTravel), true}
}

func (g *Game) restoreMissiles(sv *[nMissiles]missileSave) {
	for m := range sv {
		if sv[m].saved {
			M := g.msl(int16(m))
			M.set16(tmX, sv[m].x)
			M.set16(tmY, sv[m].y)
			M.set16(tmTravel, sv[m].travel)
		}
	}
}

// rotateShipUntilImpact is Rotate_Ship_Until_Impact_ (0x3e7a9): turning
// towards *face, would a missile hit first? *face stops a step short.
func (g *Game) rotateShipUntilImpact(i int16, face *int16) bool {
	var sv [nMissiles]missileSave
	t := g.tsp(i)
	f := int8(t.u8(tsFacing))
	cx, cy := g.shipCenterOffsets(i)
	cx += int16(t.u8(tsX)) * 0x14
	cy += int16(t.u8(tsY)) * 0x14
	d := int16(f) - *face
	dir := int8(1)
	if (d < 0 && -d > 8) || (d >= 0 && d < 9) {
		dir = -1
	}
	hit := false
	for int16(f) != *face && !hit {
		f += dir
		if f < 0 {
			f += 0x10
		}
		f %= 0x10
		for m := int16(0); m < nMissiles; m++ {
			M := g.msl(m)
			if M.i16(tmWeapon) <= 0 || M.i16(tmTarget) != i || M.i16(tmShip) == i {
				continue
			}
			step := g.missileStep(i, M, 5)
			g.saveMissile(&sv, m)
			if g.moveMissile(m, cx, cy, step) && g.willMissileDoDamage(i, m) && M[tmLive] == 1 {
				hit = true
			}
		}
	}
	if hit {
		f -= dir
		if f < 0 {
			f += 0x10
		}
		*face = int16(f % 0x10)
	}
	g.restoreMissiles(&sv)
	return hit
}

// testMoveUntilImpact is Test_Move_Until_Impact_ (0x3eaa5): would a
// missile hit the ship on its way to (*x, *y)? Then the move stops at the
// last legal square before; *face gets the facing it would take.
func (g *Game) testMoveUntilImpact(i int16, x, y, face *int16) bool {
	tc := g.tc
	var sv [nMissiles]missileSave
	dx, dy := *x, *y
	t := g.tsp(i)
	cx, cy := g.shipCenterOffsets(i)
	sx, sy := int16(t.u8(tsX))*0x14, int16(t.u8(tsY))*0x14
	step := int16(2)
	if tc.fastFX() {
		step = 0xc
	}
	hit, rotHit := false, false
	f := g.getFacing(sx, sy, dx, dy)
	if f != int16(t.u8(tsFacing)) {
		hit = g.rotateShipUntilImpact(i, &f)
		rotHit = hit
	}
	*face = f
	px, py := sx, sy
	lx, ly := px, py
	for !hit && !(px == dx && py == dy) {
		if g.shipInLegalSquare(tc.cur, px, py, dx, dy) {
			lx, ly = px, py
		}
		absoluteInterpolateLine(&px, &py, sx, sy, dx, dy, step)
		for m := int16(0); m < nMissiles; m++ {
			M := g.msl(m)
			if M.i16(tmWeapon) <= 0 || M.i16(tmTarget) != i || M.i16(tmShip) == i {
				continue
			}
			var ms int16
			if t.i16(tsSpeed) == 0 {
				ms = 5
			} else {
				ms = int16(int32(step) * int32(M[tmSpeed]) / int32(t.i16(tsSpeed)))
			}
			if ms < 2 {
				ms = 2
			}
			g.saveMissile(&sv, m)
			if g.moveMissile(m, px+cx, py+cy, ms) && g.willMissileDoDamage(i, m) && M[tmLive] == 1 {
				hit = true
			}
		}
	}
	if hit {
		if lx == sx && ly == sy {
			px, py = lx, ly
		} else {
			if dx < lx {
				px = int16(int32(lx) + 0x14 - int32(lx)%0x14)
			} else {
				px = int16(int32(lx) - int32(lx)%0x14)
			}
			if dy < ly {
				py = int16(int32(ly) + 0x14 - int32(ly)%0x14)
			} else {
				py = int16(int32(ly) - int32(ly)%0x14)
			}
		}
	}
	*x, *y = px, py
	g.restoreMissiles(&sv)
	return rotHit
}

// drawTeleportingShip is Draw_Teleporting_Ship_ (0x3e598): the ship fades
// out of its square and into (x, y) over ten steps (five at double speed);
// each Vanish_Bitmap_ between 1 and 99 percent draws once.
func (g *Game) drawTeleportingShip(i, x, y int16) {
	tc := g.tc
	sc := tc.scr
	e := g.e
	if tc.superFast != 0 {
		return
	}
	if sc != nil && !g.onCombatScreen(x, y) {
		g.must2(g.snapCenterCombatScreen(x, y))
	}
	T := g.tsp(i)
	var ox, oy int
	if sc != nil {
		sc.dontDrawShip = i
		ox, oy = g.getDrawShipOffsets(i)
		e.V.SetPageOff()
		g.must2(g.fullDrawCombatScreen())
	}
	fromX := ox + int(int16(T.u8(tsX))-tc.curX)*0x14
	fromY := oy + int(int16(T.u8(tsY))-tc.curY)*0x14
	toX := ox + int(x-tc.curX)*0x14
	toY := oy + int(y-tc.curY)*0x14
	step := int16(1)
	if tc.fastFX() {
		step = 2
	}
	// 0x3e6cd: the teleport's sound, pitched by the hull, faded at the end.
	snd := g.moxPlaySound(0x37)
	g.pitchBendSound(snd, (int(T.u8(tsSize))-2)*-0x9c4)
	defer g.fadeoutSound(snd, 0xa)
	for f := int16(0); f < 10; f += step {
		if sc == nil {
			for _, pct := range []int16{f * 10, 100 - f*10} {
				if pct > 0 && pct < 100 {
					g.state.Random.Random(1000)
				}
			}
			continue
		}
		g.must2(g.drawMainCombatScreen())
		e.V.SetWindow(0, 0, 0x27f, 0x15e)
		e.V.ClipOn()
		g.must2(g.drawShipToBitmap(i, 0, 0))
		g.must2(e.Draw(fromX, fromY, g.vanishBitmap(sc.drawShipBitmap, f*10)))
		g.must2(g.drawShipToBitmap(i, 0, 0))
		g.must2(e.Draw(toX, toY, g.vanishBitmap(sc.drawShipBitmap, 100-f*10)))
		e.V.ClipOff()
		e.V.ResetWindow()
		g.must2(e.TogglePages())
	}
	if sc != nil {
		e.V.ClipOff()
	}
}

// moveShip is Move_Ship_ (0x3ee0f): ship i to square (x, y), turning first;
// missiles chasing it move along and enemies fire defensively every third
// step. It returns the movement points used.
func (g *Game) moveShip(i, x, y int16) int16 {
	tc := g.tc
	t := g.tsp(i)
	cost := int16(0)
	used := g.usedTeleporter(i)
	warned := tc.mslWarning > 0
	dx, dy := x*0x14, y*0x14
	tele := g.combatShipHasSpecial(i, 0x21) || g.combatShipHasSpecial(i, 0x16)
	if tele && !used {
		if g.rangeToXY(i, x*0x14, y*0x14) < 0x15 {
			t[0x48] |= 1 // Set_Teleporter_Used_ (0x3888c)
			g.drawTeleportingShip(i, x, y)
		}
	} else {
		cx, cy := g.shipCenterOffsets(i)
		var tractors []int16
		if t.u8(0x42) != 0xff {
			tractors = append(tractors, i)
		}
		for j := int16(0); j < tc.n; j++ {
			J := g.tsp(j)
			if int16(J.u8(0x42)) == i&0xff && J.u8(tsGone) == 0 {
				tractors = append(tractors, j)
			}
		}
		if tc.grid[x][y] != gridLegal {
			// The original clears _shields_flag first and leaves it so.
			tc.shieldsFlag = 0
			return 0
		}
		sx, sy := int16(t.u8(tsX))*0x14, int16(t.u8(tsY))*0x14
		if tc.scr != nil {
			g.moveShipStart(i)
			defer g.moveShipEnd()
		}
		step := int16(2) // 0x3ef80: 6 pixels a step in network battles
		if tc.netFlag != 0 {
			step = 6
		} else if tc.speedx2 == 1 {
			step = 0xc
		}
		face := int16(t.u8(tsFacing))
		impact := false
		if tc.mslImpact == 1 && tc.autoMslOff == 0 && tc.mslWarning != 2 {
			impact = g.testMoveUntilImpact(i, &dx, &dy, &face)
			if impact || dx != x*0x14 || dy != y*0x14 {
				tc.mslWarning = 1
			}
		}
		if !impact {
			cost = g.moveCost(i, dx/0x14, dy/0x14)
		} else {
			cost = g.turnMultiplier(i) * int16((int32(face)+0x10-int32(t.u8(tsFacing)))%0x10)
		}
		if cost > 0 {
			face = g.getFacing(int16(t.u8(tsX)), int16(t.u8(tsY)), dx/0x14, dy/0x14)
			if warned {
				tc.mslWarning = 0
			}
		}
		if int16(t.u8(tsFacing)) != face {
			g.rotateShip(i, face)
		}
		if t.u8(tsGone) == 0 && !impact && (sx != dx || sy != dy) {
			px, py := sx, sy
			for n := int32(0); (px != dx || py != dy) && g.isCombatShipDead(i) == 0; n++ {
				absoluteInterpolateLine(&px, &py, sx, sy, dx, dy, step)
				var hit []int16
				for m := int16(0); m < nMissiles; m++ {
					M := g.msl(m)
					if M.i16(tmWeapon) > 0 && M.i16(tmTarget) == i {
						if g.moveMissile(m, px+cx, py+cy, g.missileStep(i, M, int32(step))) {
							hit = append(hit, m)
						}
					}
				}
				if tc.scr != nil {
					g.moveShipFrame(i, px, py, tractors)
				}
				for _, m := range hit {
					if t.u8(tsGone) == 0 && g.msl(m).i16(tmTarget) == i {
						g.resolveMissile(m, px+cx, py+cy)
					}
				}
				if g.isCombatShipDead(i) == 0 && t.u8(tsCloak) != 4 && n%3 == 0 {
					g.defensiveFireCheck(i, px, py, dx, dy)
				}
			}
		}
		defer func() {
			for _, j := range tractors {
				J := g.tsp(j)
				if J.u8(tsGone) == 0 && g.rangeToShip(j, int16(J.u8(0x42))) > 4 {
					J.set8(0x42, 0xff)
				}
			}
		}()
	}
	t.set8(tsX, uint8(int32(dx)/0x14))
	t.set8(tsY, uint8(int32(dy)/0x14))
	tc.cur = i
	return cost
}

// defensiveFireCheck is Defensive_Fire_Check_ (0x36ed1): enemies with beams
// ready may shoot at a ship passing (x1, y1) on its way to (x2, y2).
func (g *Game) defensiveFireCheck(i, x1, y1, x2, y2 int16) {
	s := &g.state
	tc := g.tc
	if g.tsp(i).u8(tsCloak) == 4 {
		return
	}
	for j := int16(0); j < tc.n; j++ {
		if g.isCombatShipDead(i) != 0 {
			return
		}
		J := g.tsp(j)
		if J.u8(tsOwner) == g.tsp(i).u8(tsOwner) || !g.combatShipIsFunctional(j) || J.u8(tsCloak) == 1 ||
			J.u8(tsCloak) == 4 || J.u8(0xb1) == 0 {
			continue
		}
		cx, cy := g.shipCenterOffsets(i)
		r := g.rangeToXY(j, x1+cx, y1+cy)
		r2 := g.rangeToXY(j, x2+cx, y2+cy)
		if r < 1 {
			r = 1
		}
		if r >= 9 || r > r2 {
			continue
		}
		chance := int32(int16(int32(9-r) * 100 / (9 - (int32(J.u8(0xb1))+2)/3)))
		if int32(s.Random.Random(100)) > chance {
			continue
		}
		var saved [8]int8
		for k := 0; k < 8; k++ {
			saved[k] = int8(J.u8(tw(k, twOn)))
			w := J.i16(tw(k, twWeapon))
			mods := J.i16(tw(k, twMods))
			if g.wtype(w) == 0 && saved[k] != -1 && mods&2 == 0 && mods&4 == 0 {
				if g.expectedWeaponDamage(j, k, i, 0, r) < 1 {
					J.set8(tw(k, twOn), 0xff)
				} else {
					J.set8(tw(k, twOn), 1)
				}
			} else {
				J.set8(tw(k, twOn), 0xff)
			}
		}
		g.fireShip(j, i, x1, y1, 0)
		J.set8(0xb1, uint8(g.maxDefensiveFireRange(j)))
		for k := 0; k < 8; k++ {
			J.set8(tw(k, twOn), uint8(saved[k]))
		}
	}
}

// coordMovement is Coord_Movement_ (0x3bbf6): a step of n pixels towards
// (x2, y2) along the longer axis.
func coordMovement(x1, y1, x2, y2 int16, n uint8) (dx, dy int16) {
	ddy := y2 - y1
	ddx := x2 - x1
	ay, ax := int32(ddy), int32(ddx)
	if ay < 0 {
		ay = -ay
	}
	if ax < 0 {
		ax = -ax
	}
	if ax < ay {
		s := int16(1)
		if ddy < 0 {
			s = -1
		}
		dy = s * int16(n)
		dx = int16(int32(dy) * int32(ddx) / int32(ddy))
		return
	}
	s := int16(1)
	if ddx < 0 {
		s = -1
	}
	dx = s * int16(n)
	dy = int16(int32(dx) * int32(ddy) / int32(ddx))
	return
}

// didMissileHit is Did_Missile_Hit_ (0x3bccd).
func (g *Game) didMissileHit(m, x, y int16) bool {
	M := g.msl(m)
	dx := int32(x) - int32(M.i16(tmX))
	dy := int32(y) - int32(M.i16(tmY))
	r := int32(0x40)
	if M[tmAtMis] != 1 {
		h := int32(g.exeI16(uint32(0x17d881 + 2*int32(g.tsp(M.i16(tmTarget)).u8(tsSize)))))
		r = h * h
	}
	return dx*dx+dy*dy < r
}

// moveMissile is Move_Missile_ (0x3ceb7): up to n pixels towards (x, y);
// missiles chasing this one come along. True when it reached its target.
func (g *Game) moveMissile(m, x, y, n int16) bool {
	M := g.msl(m)
	if g.wtype(M.i16(tmWeapon)) == 4 && M[tmAtMis] == 0 {
		h := int32(g.exeI16(uint32(0x17d881 + 2*int32(g.tsp(M.i16(tmTarget)).u8(tsSize)))))
		x += int16(h / 2)
	}
	if x == M.i16(tmX) && y == M.i16(tmY) {
		return true
	}
	hit := false
	if M.i16(tmWeapon) > 0 {
		if M.i16(tmTravel) < n {
			n = M.i16(tmTravel)
		}
		ddx, ddy := coordMovement(M.i16(tmX), M.i16(tmY), x, y, uint8(n))
		M.set16(tmX, M.i16(tmX)+ddx)
		M.set16(tmY, M.i16(tmY)+ddy)
		for k := int16(0); k < nMissiles; k++ {
			K := g.msl(k)
			if K.i16(tmWeapon) > 0 && K[tmAtMis] == 1 && K.i16(tmTarget) == m {
				if g.moveMissile(k, M.i16(tmX), M.i16(tmY), n) {
					g.resolveMissile(k, M.i16(tmX), M.i16(tmY))
				}
			}
		}
		if ddx > 0 && int32(x)-3 < int32(M.i16(tmX)) {
			M.set16(tmX, x-3)
		}
		if ddx < 0 && int32(M.i16(tmX)) < int32(x)+3 {
			M.set16(tmX, x+3)
		}
		if ddy > 0 && int32(y)-3 < int32(M.i16(tmY)) {
			M.set16(tmY, y-3)
		}
		if ddy < 0 && int32(M.i16(tmY)) < int32(y)+3 {
			M.set16(tmY, y+3)
		}
		hit = g.didMissileHit(m, x, y)
		M.set16(tmTravel, M.i16(tmTravel)-n)
		M[tmFace] = uint8(g.getFacing(M.i16(tmX), M.i16(tmY), x, y))
	}
	if w := M.i16(tmWeapon); (w == 0x14 || w == 0x28) && !hit && M.i16(tmMods)&0x20 == 0 {
		left := (int16(M[tmBombs])+1)*g.missileSpeed(w, M.i16(tmMods), M.i16(tmOwner))*0x14 - M.i16(tmTravel)
		pct := g.tc.ordnance[M.i16(tmOwner)]
		if M.i16(tmMods)&0x4000 != 0 {
			pct += 0x32
		}
		dmg := int16(int32(g.wpnI16(int(w), weaponMaxDmg)) * int32(pct) / 100)
		var cut int16
		if w == 0x14 {
			cut = left / 0x14 * 5
		} else {
			cut = left / 0x14 * 0xf
		}
		if dmg <= cut {
			g.killMissile(m)
		}
	}
	return hit
}

// missileOnCombatScreen is Missile_On_Combat_Screen_ (0x3ce4f) (UI only).

// seekingMissiles is Seeking_Missiles_ (0x44ea4): p's missiles fly their
// course for the turn; those that arrive are resolved.
func (g *Game) seekingMissiles(p int16) {
	tc := g.tc
	for {
		still, onScreen := true, false
		var hitM, hitT []int16
		for m := int16(0); m < nMissiles; m++ {
			M := g.msl(m)
			t := M.i16(tmTarget)
			if M.i16(tmWeapon) <= 0 || M.i16(tmTravel) <= 0 || M[tmAtMis] != 0 {
				continue
			}
			if !g.seeksThisTurn(M, p) {
				continue
			}
			x, y := g.missileTargetXY(M)
			arrived := g.moveMissile(m, x, y, int16(M[tmSpeed]))
			if tc.scr != nil && g.missileInView(m) {
				onScreen = true
			}
			if x != M.i16(tmX) || y != M.i16(tmY) {
				still = false
			}
			if arrived && (g.wtype(M.i16(tmWeapon)) != 4 || M[tmLive] == 1) {
				hitM = append(hitM, m)
				hitT = append(hitT, M.i16(tmTarget))
			}
			_ = t
		}
		if onScreen {
			g.combatFrame()
		}
		for k, m := range hitM {
			M := g.msl(m)
			t := M.i16(tmTarget)
			if t != hitT[k] {
				continue
			}
			T := g.tsp(t)
			if T.u8(tsGone) == 0 && (M.i16(tmOwner) != int16(T.u8(tsOwner)) || t == M.i16(tmShip)) {
				// The original takes the at-missile flag of stack k, not m.
				var x, y int16
				if g.msl(int16(k))[tmAtMis] == 0 {
					x, y = g.shipCenterOffsets(t)
					x += int16(T.u8(tsX)) * 0x14
					y += int16(T.u8(tsY)) * 0x14
				} else {
					x, y = g.msl(t).i16(tmX), g.msl(t).i16(tmY)
				}
				g.resolveMissile(m, x, y)
				continue
			}
			g.killMissile(m)
		}
		if !still {
			continue
		}
		for m := int16(0); m < nMissiles; m++ {
			M := g.msl(m)
			if M.i16(tmWeapon) < 1 || g.wtype(M.i16(tmWeapon)) == 4 || M[tmBombs] == 0xff || M[tmAtMis] != 0 {
				if M[tmBombs] == 0xff {
					M[tmBombs] = 0
				}
				continue
			}
			if tc.initOrder != 0 || int16(g.tsp(M.i16(tmTarget)).u8(tsOwner)) == p ||
				(p == tc.def && M.i16(tmShip) == M.i16(tmTarget) && int16(g.tsp(M.i16(tmShip)).u8(tsOwner)) != p) {
				M[tmBombs]++
			}
		}
		return
	}
}

// seeksThisTurn is the missile filter of Seeking_Missiles_: missiles fly in
// the turn of the side they are aimed at.
func (g *Game) seeksThisTurn(M *tmissile, p int16) bool {
	tc := g.tc
	t := M.i16(tmTarget)
	if tc.initOrder != 0 {
		// Under initiative a missile seeks unless it was fired this turn.
		return M[tmBombs] != 0xff
	}
	if int16(g.tsp(t).u8(tsOwner)) == p {
		return true
	}
	return p == tc.def && M.i16(tmShip) == t && int16(g.tsp(M.i16(tmShip)).u8(tsOwner)) != p
}

func (g *Game) missileTargetXY(M *tmissile) (int16, int16) {
	t := M.i16(tmTarget)
	if M[tmAtMis] == 0 {
		x, y := g.shipCenterOffsets(t)
		T := g.tsp(t)
		return x + int16(T.u8(tsX))*0x14, y + int16(T.u8(tsY))*0x14
	}
	return g.msl(t).i16(tmX), g.msl(t).i16(tmY)
}

// missileLaunchFX is Missile_Launch_FX_ (0xaf37e): the missile leaves the
// launcher, 36 pixels of its course, and may hit at once.
func (g *Game) missileLaunchFX(m, x, y, mode int16) {
	tc := g.tc
	M := g.msl(m)
	t := M.i16(tmTarget)
	step := int16(3)
	if tc.fastFX() {
		step = 9
	}
	limit := int16(8)
	if w := M.i16(tmWeapon); w == 0x13 || w == 0x28 {
		limit = 10000
		M.set16(tmTravel, 10000)
	}
	if tc.superFast == 0 {
		g.missileLaunchSound(M.i16(tmWeapon))
	}
	hit := false
	for k := int16(0); k < limit && !hit; k += step {
		hit = g.moveMissile(m, x, y, step<<2)
		if g.combatDrawing() {
			g.combatFrame()
		}
		if mode == 1 {
			if g.msl(t).i16(tmCount) < 1 {
				g.retargetMissile(m)
				hit = false
			}
		} else if g.tsp(M.i16(tmTarget)).u8(tsGone) != 0 {
			g.retargetMissile(m)
			hit = false
		}
		if hit {
			g.resolveMissile(m, x, y)
		}
	}
}

// missileLaunchSound is Missile_Launch_FX_'s sound (0xaf40e, jump table
// 0xaf362 on weapon - 0xe): 0x20 pitched +500, -1500 or -2500 for weapons
// 0xe, 0x10, 0x11; 0x3d, 0x3c, 0x3b for 0x12..0x14; else 0x20.
func (g *Game) missileLaunchSound(w int16) {
	switch uint16(w - 0xe) {
	case 0:
		g.pitchBendSound(g.moxPlaySound(0x20), 0x1f4)
	case 2:
		g.pitchBendSound(g.moxPlaySound(0x20), -0x5dc)
	case 3:
		g.pitchBendSound(g.moxPlaySound(0x20), -0x9c4)
	case 4:
		g.moxPlaySound(0x3d)
	case 5:
		g.moxPlaySound(0x3c)
	case 6:
		g.moxPlaySound(0x3b)
	default:
		g.moxPlaySound(0x20)
	}
}
