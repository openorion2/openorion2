package game

import (
	"bytes"
	"encoding/binary"

	"github.com/openorion2/openorion2/internal/random"
	"github.com/openorion2/openorion2/internal/save"
)

// Offsets in the antarans block (0x199174), all int16 unless noted.
const (
	antPressure   = 0x00 // grows while ships wait; the attack roll is Random(200) − 1 < it + 1
	antOffRes     = 0x02
	antDefRes     = 0x04
	antDefCount   = 0x06 // [9]
	antOffCount   = 0x18 // [9]
	antDeployed   = 0x2a // [9] offensive ships sent so far
	antSkip       = 0x3c // byte
	antLastAttack = 0x3d // uint32 stardate
	antSizes      = 9
	antaranOwner  = 8
)

// Antaran tables in .data. Build_Antaran_Ships_ changes them, so they are
// program state that lives until exit, never saved and never reset.
const (
	antaranCostTbl        = 0x181734 // u8 by size
	antaranMaxOffTbl      = 0x18173d
	antaranMaxDefTbl      = 0x181746
	shipCombatSpeedTbl    = 0x17fe90 // u8, 0x2e-byte rows by drive, by hull size
	shipCombatSpeedRow    = 0x2e
	antaranShipXPTbl      = 0x17d17e // i16: experience of a crew level 2 ship, minus one
	antaranAttackPressure = 200
)

type antaranTables struct {
	cost, maxOff, maxDef [antSizes]uint8
	titanBuilt           bool // _offensive_titan_ship_built (0x19b89a)
}

func (g *Game) antaran() *antaranTables {
	if g.antaranTables == nil {
		t := &antaranTables{}
		copy(t.cost[:], g.exe.bytes(antaranCostTbl, antSizes))
		copy(t.maxOff[:], g.exe.bytes(antaranMaxOffTbl, antSizes))
		copy(t.maxDef[:], g.exe.bytes(antaranMaxDefTbl, antSizes))
		g.antaranTables = t
	}
	return g.antaranTables
}

func (s *State) antWord(off int) int16 {
	return int16(binary.LittleEndian.Uint16(s.Antarans[off:]))
}

func (s *State) setAntWord(off int, v int16) {
	binary.LittleEndian.PutUint16(s.Antarans[off:], uint16(v))
}

// antaranInvasionStep is the Antaran_Invasion_Check_ call of Next_Turn_Calc_.
func (g *Game) antaranInvasionStep() {
	if g.settings.AntaransAttack() && g.state.Antarans[antSkip] == 0 {
		g.antaranInvasionCheck()
	}
}

// antaranInvasionCheck is Antaran_Invasion_Check_ (0x63d92).
func (g *Game) antaranInvasionCheck() {
	s := &g.state
	t := g.antaran()
	if s.deltaStardate() == 0 {
		t.titanBuilt = false
	}
	g.incrementAntaranResourceLevel()
	g.buildAntaranShips(antOffRes, antOffCount, t.maxOff[:], true)
	g.buildAntaranShips(antDefRes, antDefCount, t.maxDef[:], false)
	if g.antaransShouldAttack() {
		s.setAntWord(antPressure, 0)
		if g.deployAntaranShips() {
			binary.LittleEndian.PutUint32(s.Antarans[antLastAttack:], s.Stardate)
		}
	}
}

// antaranDelay is Antaran_Delay_ (0x63e4c): turns before they build, by tech level.
func (g *Game) antaranDelay() int16 {
	switch g.settings.TechLevel() {
	case 0:
		return 200
	case 1:
		return 100
	}
	return 0
}

// antaranDifficultyPercent is the 100/150/200 % scale by difficulty that
// several Antaran routines inline.
func (g *Game) antaranDifficultyPercent() int32 {
	switch g.settings.Difficulty() {
	case 3:
		return 150
	case 4:
		return 200
	}
	return 100
}

// incrementAntaranResourceLevel is Increment_Antaran_Resource_Level_ (0x645ec):
// every 25 turns after the delay both budgets grow; a full defence gives its
// share to the offence.
func (g *Game) incrementAntaranResourceLevel() {
	s := &g.state
	d := s.deltaStardate() - int32(g.antaranDelay())
	if d <= 0 || d%25 != 0 {
		return
	}
	k := d / 25
	pct := g.antaranDifficultyPercent()
	off := (pct*k + 99) / 100
	def := (pct*k + 99) / 100
	s.setAntWord(antOffRes, s.antWord(antOffRes)+int16(off))
	if g.antaranDefensiveForceAtMaximum() {
		s.setAntWord(antOffRes, s.antWord(antOffRes)+int16(def))
	} else {
		s.setAntWord(antDefRes, s.antWord(antDefRes)+int16(def))
	}
}

// antaranDefensiveForceAtMaximum is Antaran_Defensive_Force_At_Maximum_ (0x646bd).
func (g *Game) antaranDefensiveForceAtMaximum() bool {
	s := &g.state
	t := g.antaran()
	for b := range antSizes {
		if t.maxDef[b] > 0 && int16(t.maxDef[b]) > s.antWord(antDefCount+2*b) {
			return false
		}
	}
	return true
}

// noSmallShipsAfter is No_Small_Ships_After_N_Turns_ (0x647d7).
func (g *Game) noSmallShipsAfter() int32 {
	if g.settings.Difficulty() < 3 {
		return 100
	}
	return 12500 / g.antaranDifficultyPercent()
}

// noMediumShipsAfter is No_Medium_Ships_After_N_Turns_ (0x6481b).
func (g *Game) noMediumShipsAfter() int32 {
	if g.settings.Difficulty() < 3 {
		return 199
	}
	return 20000 / g.antaranDifficultyPercent()
}

// buildAntaranShips is Build_Antaran_Ships_ (0x63ff0) with the offensive
// (0x63fcb) and defensive (0x63f9c) entries: buy the smallest allowed size
// that is under its cap while the budget lasts. Its debug log is dead output.
func (g *Game) buildAntaranShips(resOff, cntOff int, max []uint8, offensive bool) {
	s := &g.state
	t := g.antaran()
	delay := s.deltaStardate() - int32(g.antaranDelay())
	if int16(t.cost[0]) > s.antWord(resOff) {
		return
	}
	for {
		pick := -1
		b := 0
		if g.settings.TechLevel() == 2 {
			b = 1
		}
		for ; b < antSizes; b++ {
			allowed := true
			if b == 0 && g.noSmallShipsAfter() < delay || b == 1 && g.noMediumShipsAfter() < delay {
				allowed = false
			}
			if allowed && t.cost[b] > 0 && s.antWord(resOff) >= int16(t.cost[b]) &&
				int16(max[b]) > s.antWord(cntOff+2*b) {
				pick = b
				break
			}
		}
		if pick == -1 {
			return
		}
		n := s.antWord(cntOff+2*pick) + 1
		s.setAntWord(cntOff+2*pick, n)
		// The cap cleared is always the offensive one, even for defence.
		if pick <= 2 && int16(t.maxOff[pick]) == n {
			t.maxOff[pick] = 0
		}
		s.setAntWord(resOff, s.antWord(resOff)-int16(t.cost[pick]))
		if pick == 4 && !t.titanBuilt && g.settings.Difficulty() > 2 {
			t.titanBuilt = true
			for k := range t.cost {
				t.cost[k] = uint8(int32(t.cost[k]) * 90 / 100)
			}
		}
	}
}

// antaranForce is Antaran_Defensive_Force_ (0x63dda) or Antaran_Offensive_Force_
// (0x63e73): cost times count per size, each product kept to 16 bits.
func (g *Game) antaranForce(cntOff int) uint32 {
	s := &g.state
	t := g.antaran()
	sum := uint32(0)
	for b := range antSizes {
		sum += uint32(uint16(t.cost[b]) * uint16(s.antWord(cntOff+2*b)))
	}
	return sum
}

// antaransShouldAttack is Antarans_Should_Attack_ (0x63edd).
func (g *Game) antaransShouldAttack() bool {
	s := &g.state
	def := g.antaranForce(antDefCount)
	off := g.antaranForce(antOffCount)
	total := int16(uint16(off+def) + uint16(s.antWord(antOffRes)) + uint16(s.antWord(antDefRes)))
	if int32(int16(off))*4 < int32(total) {
		return false
	}
	waiting := false
	for b := 0; b < antSizes && !waiting; b++ {
		if int32(s.antWord(antOffCount+2*b))-int32(s.antWord(antDeployed+2*b)) > 0 {
			waiting = true
		}
	}
	if waiting {
		s.setAntWord(antPressure, s.antWord(antPressure)+1)
	}
	p := s.antWord(antPressure)
	return p > 0 && int32(int16(s.Random.Random(antaranAttackPressure)-1)) < int32(p)+1
}

// nextAntaranShipSize is Next_Antaran_Ship_Size_ (0x6478d): the largest size
// with offensive ships not yet sent, or -1.
func (s *State) nextAntaranShipSize() int {
	for b := antSizes - 1; b >= 0; b-- {
		if d := int32(s.antWord(antOffCount+2*b)) - int32(s.antWord(antDeployed+2*b)); d > 0 && int16(d) > 0 {
			return b
		}
	}
	return -1
}

// deployAntaranShips is Deploy_Antaran_Ships_ (0x643a0): up to six offensive
// ships, largest first, appear near a star of the victim and head for it.
// It reports false only when the first ship could not be placed.
func (g *Game) deployAntaranShips() bool {
	s := &g.state
	victim := g.getAntaranVictim()
	target := g.getAntaranTargetStar(victim)
	dest := int16(int32(target) + 500)
	var d save.Design // reused like the original's stack buffer
	var x, y int16
	for n := int16(0); ; n++ {
		size := s.nextAntaranShipSize()
		if size < 0 {
			return true
		}
		g.loadAntaranShipDesign(&d, size)
		var ship int16
		if n == 0 {
			if ship = g.setNonplayerShipXY(target, antaranOwner, &d, true); ship == -1 {
				return false
			}
			x, y = s.Ships[ship].X, s.Ships[ship].Y
		} else {
			ship = g.createNonplayerShip(antaranOwner, &d, x, y)
		}
		if ship == -1 {
			return true
		}
		s.setAntWord(antDeployed+2*size, s.antWord(antDeployed+2*size)+1)
		ids := []int16{ship, -1}
		info := g.shipsTryToMoveTo(ids, int(target))
		if info.canMove {
			g.makeShipsMoveTo(ids, int(target), &info)
			sh := &s.Ships[ship]
			sh.Status = 1
			sh.Location = dest
			sh.CrewLevel = 2
			sh.Experience = g.exeI16(antaranShipXPTbl) + 1
		}
		if n >= 5 {
			return true
		}
	}
}

// getAntaranVictim is Get_Antaran_Victim_ (0x22f5c): a weighted pick among
// the living players other than the least populous, by squared distance of
// their (difficulty and luck scaled) population from that minimum.
func (g *Game) getAntaranVictim() int16 {
	s := &g.state
	var weights [8]uint16
	players := [8]int16{-1, -1, -1, -1, -1, -1, -1, -1}
	n := 0
	for p := int16(0); p < s.NumPlayers; p++ {
		pl := &s.Players[p]
		if pl.Eliminated != 0 {
			continue
		}
		players[n] = p
		w := int32(pl.Population)
		if pl.Objective == save.ObjectiveHuman {
			switch g.settings.Difficulty() {
			case 0:
				w /= 10
			case 1:
				w /= 5
			case 3:
				w += int32(int16(w)) / 2
			case 4:
				w *= 3
			}
		}
		if pl.Picks[save.PickLucky] == 1 {
			w = int32(int16(w)) / 3
		}
		weights[n] = uint16(w)
		n++
	}
	// The original leaves the index unset when no one has less than 32000.
	least, min := 0, int16(32000)
	for k := 0; k < n; k++ {
		if pop := s.Players[players[k]].Population; min > pop {
			least, min = k, pop
		}
	}
	n--
	for k := least; k < n; k++ {
		players[k], weights[k] = players[k+1], weights[k+1]
	}
	for k := 0; k < n; k++ {
		d := int32(weights[k]) - int32(min)
		weights[k] = uint16(d * d)
	}
	return players[getWeightedChoice(&s.Random, weights[:n])]
}

// getWeightedChoice is Get_Weighted_Choice_ (0x586d4). The weights are halved
// in place until they sum below 512; no draw when they sum to 0.
func getWeightedChoice(r *random.Source, w []uint16) int {
	var sum uint32
	for again := true; again; {
		again = false
		sum = 0
		for i := range w {
			if sum += uint32(w[i]); sum >= 512 {
				for k := range w {
					w[k] /= 2
				}
				again = true
				break
			}
		}
	}
	if sum == 0 {
		return 0
	}
	left := int32(r.Random(sum)) - int32(w[0])
	i := 0
	for left > 0 && i < len(w)-1 {
		i++
		left -= int32(w[i])
	}
	return i
}

// getAntaranTargetStar is Get_Antaran_Target_Star_ (0x645a4): one of the
// victim's stars, by reservoir sampling; -1 if it has none.
func (g *Game) getAntaranTargetStar(victim int16) int16 {
	s := &g.state
	list := g.buildListOfStarsByPlayer(victim)
	res := int16(-1)
	for d := range list {
		if s.Random.Random(uint32(d+1)) == 1 {
			res = list[d]
		}
	}
	return res
}

// buildListOfStarsByPlayer is Build_List_Of_Stars_By_Player_ (0x64311): the
// stars with a colony of p, shuffled by n/2 random swaps.
func (g *Game) buildListOfStarsByPlayer(p int16) []int16 {
	s := &g.state
	var list []int16
	for c := int16(0); c < s.NumStars; c++ {
		if s.nColoniesAtStar(c, p) > 0 {
			list = append(list, c)
		}
	}
	n := int32(len(list))
	for i := int32(0); n/2 > i; i++ {
		a := int16(s.Random.Random(uint32(n)) - 1)
		b := int16(s.Random.Random(uint32(n)) - 1)
		if a != b {
			list[a], list[b] = list[b], list[a]
		}
	}
	return list
}

// nColoniesAtStar is N_Colonies_At_Star_ (0x7826a): colonies (not outposts)
// of p at the star, any owner's for p == -1.
func (s *State) nColoniesAtStar(star, p int16) uint8 {
	n := uint8(0)
	for _, pi := range s.Stars[star].Planets {
		if pi <= -1 {
			continue
		}
		c := s.Planets[pi].Colony
		if c == -1 {
			continue
		}
		if col := &s.Colonies[c]; col.IsOutpost == 0 && (int16(col.Owner) == p || p == -1) {
			n++
		}
	}
	return n
}

// antaranSlot is one weapon mount of an Antaran design as the loaders write it.
type antaranSlot struct {
	weapon         uint16
	count, working uint8
	arc            uint8
	mods           uint16
	ammo           uint8
}

// antaranDesign is what one of the Load_*_Antaran_Design_ routines writes.
type antaranDesign struct {
	name    uint32 // the name is copied from here, nameLen bytes
	nameLen int
	slots   []antaranSlot
	bits    []int
	byDiff  [5]int // extra special by difficulty, 0 for none
}

var antaranDesigns = [5]antaranDesign{
	{0x178ea0, 7, []antaranSlot{{4, 2, 2, 1, 0, 0}, {0x18, 1, 1, 0xf, 0, 0}},
		[]int{7, 0x18}, [5]int{0, 0, 0, 9, 9}},
	{0x178ea7, 9, []antaranSlot{{4, 3, 3, 1, 0, 0}, {4, 1, 1, 1, 2, 0}, {0x18, 1, 1, 0xf, 0, 0xa}},
		[]int{7, 0x18}, [5]int{0, 0, 0, 9, 0x15}},
	{0x178eb0, 9, []antaranSlot{{4, 4, 4, 1, 0, 0}, {4, 2, 2, 1, 2, 0}, {0x18, 5, 5, 0xf, 0, 0xa},
		{0xd, 2, 2, 0xf, 0, 0}, {0x1f, 3, 3, 0xf, 0, 0}},
		[]int{7, 0x18}, [5]int{0, 0, 9, 0x15, 0x26}},
	{0x178eb9, 12, []antaranSlot{{4, 0xa, 0xa, 1, 0, 0}, {4, 4, 4, 1, 2, 0}, {0x18, 0xf, 0xf, 1, 0, 0xa},
		{0xd, 2, 2, 0xf, 0, 0}, {4, 0xa, 0xa, 1, 4, 0}, {0xb, 4, 4, 1, 0, 0}},
		[]int{7, 0x18, 0x21, 0x1a}, [5]int{0, 9, 0x15, 0x26, 0x26}},
	{0x178ec5, 10, []antaranSlot{{4, 0x14, 0x14, 1, 0, 0}, {4, 5, 5, 1, 2, 0}, {0x18, 0x1e, 0x1e, 1, 0, 0xa},
		{0xd, 3, 3, 0xf, 0, 0}, {4, 0x14, 0x14, 1, 4, 0}, {0xb, 5, 5, 1, 2, 0}, {0x25, 1, 1, 0xf, 0, 0},
		{0x25, 0xc, 0xc, 0xf, 0, 0}},
		[]int{7, 0x18, 0x21, 0x1a}, [5]int{9, 0x15, 0x26, 0x26, 0x26}},
}

// loadAntaranShipDesign is Load_Antaran_Ship_Design_ (0x5514c) with the five
// loaders it dispatches to (0x55364 …); other sizes leave d untouched.
func (g *Game) loadAntaranShipDesign(d *save.Design, size int) {
	if size < 0 || size > 4 {
		return
	}
	a := &antaranDesigns[size]
	var b [0x63]byte
	copy(b[:], g.exe.bytes(a.name, a.nameLen))
	b[0x10] = uint8(size) // hull
	b[0x13] = 5           // drive
	b[0x14] = 1           // FTL
	b[0x15] = 5           // computer
	b[0x16] = 6           // armour
	for k, sl := range a.slots {
		o := 0x1c + 8*k
		binary.LittleEndian.PutUint16(b[o:], sl.weapon)
		b[o+2], b[o+3], b[o+4] = sl.count, sl.working, sl.arc
		binary.LittleEndian.PutUint16(b[o+5:], sl.mods)
		b[o+7] = sl.ammo
	}
	for _, bit := range a.bits {
		b[0x17+bit/8] |= 1 << (bit % 8)
	}
	if diff := g.settings.Difficulty(); diff < len(a.byDiff) && a.byDiff[diff] != 0 {
		bit := a.byDiff[diff]
		b[0x17+bit/8] |= 1 << (bit % 8)
	}
	b[0x5c] = uint8(size) // picture
	b[0x5d] = antaranOwner
	binary.LittleEndian.PutUint16(b[0x5e:], 500)
	b[0x60] = g.exeU8(uint32(shipCombatSpeedTbl + int(b[0x13])*shipCombatSpeedRow + size))
	if err := binary.Read(bytes.NewReader(b[:]), binary.LittleEndian, d); err != nil {
		panic(err)
	}
}

// createNonplayerShip is Create_Nonplayer_Ship_ (0x100010). Only slots last
// held by an Antaran ship are reused: a dead one, or one past NUM_SHIPS.
func (g *Game) createNonplayerShip(owner int8, d *save.Design, x, y int16) int16 {
	s := &g.state
	i := int16(0)
	for ; i < maxShips; i++ {
		sh := &s.Ships[i]
		if sh.Owner == antaranOwner && (sh.Status == 5 || s.NumShips <= i) {
			break
		}
	}
	if i == maxShips {
		return -1
	}
	if s.NumShips <= i {
		s.NumShips = i + 1
	}
	sh := &s.Ships[i]
	*sh = save.Ship{}
	if d != nil {
		sh.Design = *d
	}
	sh.Status = 0
	sh.Owner = owner
	sh.Design.Builder = uint8(owner)
	sh.Location = int16(g.xyToStarID(x, y))
	sh.Officer = -1
	sh.X, sh.Y = x, y
	sh.Design.BuildDate = uint16(s.Stardate)
	return i
}

// setNonplayerShipXY is Set_Nonplayer_Ship_XY_ (0xa1762): creates the ship at
// the star, or with scatter in a random walk of 2–6 steps of up to 30
// map-scaled units, kept inside the map margins.
func (g *Game) setNonplayerShipXY(star int16, owner int8, d *save.Design, scatter bool) int16 {
	s := &g.state
	step := int32(g.scaled(30))
	steps := int32(s.Random.Random(5)) + 1
	st := &s.Stars[star]
	sx := int32(g.scaled(st.X))
	sy := int32(g.scaled(st.Y))
	reach := step*steps + 10
	var x, y int16
	if !scatter {
		x, y = st.X, st.Y
	} else {
		r := int32(int16(reach))
		right := int32(int16(sx))+r < 0x20e
		left := int32(int16(sx))-r > 0x15
		down := int32(int16(sy))+r < 0x1a4
		up := int32(int16(sy))-r > 0x15
		dx := g.scatterDirection(right, left)
		dy := g.scatterDirection(down, up)
		ox, oy := step*dx, step*dy
		for i := int16(1); i < int16(steps); i++ {
			n := uint32(int32(int16(step)))
			ox += int32(s.Random.Random(n)) * dx
			oy += int32(s.Random.Random(n)) * dy
		}
		nx, ny := sx+ox, sy+oy
		halfW := int32(g.mapMarginX) / 2
		if int32(int16(nx)) < halfW+22 {
			nx = halfW + 22
		} else if lim := int32(int16(g.scaled(s.MapMaxX))) - halfW; int32(int16(nx)) > lim {
			nx = lim
		}
		// The bottom edge keeps the whole margin, not half of it.
		halfH := int32(g.mapMarginY) / 2
		if int32(int16(ny)) < halfH+22 {
			ny = halfH + 22
		} else if int32(int16(ny)) > int32(int16(g.scaled(s.MapMaxY)))-int32(g.mapMarginY) {
			ny = int32(int16(g.scaled(s.MapMaxY)) - g.mapMarginY)
		}
		x = int16(g.upScaled(int16(nx)))
		y = int16(g.upScaled(int16(ny)))
	}
	return g.createNonplayerShip(owner, d, x, y)
}

// scatterDirection picks +1 or −1 for a scatter axis: towards the free side,
// a coin flip when both are free (Random(2) == 1 is −1), 0 when neither is.
func (g *Game) scatterDirection(plus, minus bool) int32 {
	switch {
	case plus && minus:
		if g.state.Random.Random(2) == 1 {
			return -1
		}
		return 1
	case plus:
		return 1
	case minus:
		return -1
	}
	return 0
}
