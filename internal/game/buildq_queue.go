package game

import (
	"bytes"
	"encoding/binary"
	"sort"

	"github.com/openorion2/openorion2/internal/save"
)

// The build queue popup's lists and its copy of the colony queue
// (_current_item), edited until OK, Buy or another colony writes it back.

// Fields of the player record the lists test (offsets 0xc4 and 0x117).
const (
	appColonyShip  = 0x29 // 0x140
	appFreighters  = 0x45 // 0x15c
	appOutpost     = 0x6d // 0x184
	appTitan       = 0xba // 0x1d1
	appTransport   = 0xbd // 0x1d4
	appDoomStar    = 0x37 // 0x14e
	fieldRefitA    = 0x16 // 0xda
	fieldRefitB    = 0x17 // 0xdb
	outpostNames   = 0x17d10e
	bldgMaintTable = 0x17eb49 // i16 maintenance, 19-byte building rows
	bldgAppTable   = 0x17eb43 // i16 help record
)

// calculateBuildingArray is Calculate_Building_Array_ (0xafefc) with the
// 1.40 patch at 0x16948f: trade goods and housing head the list (or the
// empty item and trade goods when bit 3 of star 0's byte 0xe is set), then
// the buildings the colony can build, by name.
func (g *Game) calculateBuildingArray() {
	s, bq := &g.state, &g.bq
	bq.bldg = append(bq.bldg[:0], -2, -3)
	if s.Stars[0].Name[14]&8 != 0 {
		bq.bldg[0], bq.bldg[1] = -1, -2
	}
	for b := int16(1); b < 0x31; b++ {
		can := g.colonyCanBuildProduct(g.colonyHandle, b)
		if (len(bq.bldg) <= 0x36 && can == 1) || g.cscr.cheats {
			if b != bqColonyBase {
				bq.bldg = append(bq.bldg, b)
			}
		}
	}
	rest := bq.bldg[2:]
	sort.SliceStable(rest, func(i, j int) bool {
		a, _ := g.buildingName(int(rest[i]))
		b, _ := g.buildingName(int(rest[j]))
		return strcmp(a, b) < 0
	})
}

// calculateMilitaryArray is Calculate_Military_Array_ (0xaff9e): colony
// bases while the star has room, freighters, then with a drive the support
// ships, the five designs and the doom star; separators (-9) between
// groups; the androids and spies last.
func (g *Game) calculateMilitaryArray() {
	s, bq := &g.state, &g.bq
	ci := g.colonyHandle
	c := &s.Colonies[ci]
	add := func(v int16) { bq.mil = append(bq.mil, v) }
	bq.mil = bq.mil[:0]
	star := int(s.Planets[c.Planet].Star)
	if g.starNUncolonizedWorlds(star) > g.nProdsInQueue(bqColonyBase) && g.colonyCanBuildProduct(ci, bqColonyBase) != 0 {
		add(bqColonyBase)
	}
	me := &s.Players[s.PlayerNum]
	if me.AppStatus[appFreighters] == 3 && g.colonyCanBuildProduct(ci, -0xf) != 0 {
		add(-0xf)
	}
	pl := &s.Players[c.Owner]
	if pl.FuelRange > 0 && pl.FTLSpeed > 0 {
		if pl.AppStatus[appColonyShip] == 3 {
			add(bqColonyShip)
		}
		if pl.AppStatus[appOutpost] == 3 {
			add(bqOutpost)
		}
		if pl.AppStatus[appTransport] == 3 && c.Buildings[0x16] != 0 {
			add(bqTransport)
		}
		if len(bq.mil) != 0 {
			add(bqSeparator)
		}
		for k := 0; k < 5; k++ {
			item := int16(-50 - k)
			if g.selectionName(item) == g.eStr(0xc) {
				continue
			}
			if g.settings[0xd8] != 0 && me.Designs[k].Size == 4 && pl.AppStatus[appTitan] != 3 {
				continue
			}
			add(item)
		}
		if pl.AppStatus[appDoomStar] == 3 && g.settings[0xd8] != 0 {
			add(bqDoomStar)
		}
	}
	if len(bq.mil) != 0 {
		add(bqSeparator)
	}
	for _, v := range []int16{-4, -5, -6, -7} {
		if g.colonyCanBuildProduct(ci, v) != 0 {
			add(v)
		}
	}
}

// starNUncolonizedWorlds is Star_N_Uncolonized_Worlds_ (0xafe6e): planets
// (type 3) without a colony, outposts counting as none.
func (g *Game) starNUncolonizedWorlds(star int) int16 {
	s := &g.state
	var n int16
	for i := 0; i < 5; i++ {
		pl := s.Stars[star].Planets[i]
		if pl == -1 {
			continue
		}
		if ci := s.Planets[pl].Colony; ci != -1 && s.Colonies[ci].IsOutpost == 0 {
			continue
		}
		if s.Planets[pl].Type == 3 {
			n++
		}
	}
	return n
}

// nProdsInQueue is N_Prods_In_Queue_ (0xafde9) on _current_item.
func (g *Game) nProdsInQueue(prod int16) int16 {
	var n int16
	for _, v := range g.bq.cur {
		if v == prod {
			n++
		}
	}
	return n
}

// isInQueue is Is_In_Queue_ (0xafa36).
func (g *Game) isInQueue(prod int16) bool {
	for _, v := range g.bq.cur {
		if v == prod {
			return true
		}
	}
	return false
}

// firstProdQueueSlot is First_Prod_Queue_Slot_ (0xb1d57) on _current_item:
// designs are never found; codes from 0x31 up name the slot code-0x31.
func (g *Game) firstProdQueueSlot(prod int16) int16 {
	q := &g.bq.cur
	if bqIsDesign(prod) {
		return -1
	}
	for i := int16(0); i < 7; i++ {
		switch {
		case prod > 0 && prod < 0x31:
			if prod == q[i] {
				return i
			}
		case q[i] <= -100:
			if prod == q[i] {
				return i
			}
		case prod >= 0x31:
			if prod-0x31 == i {
				return i
			}
		case prod == -6 || prod == -4 || prod == -5 || prod == -7 || bqIsSetting(prod) || prod == -0xf:
			if prod == q[i] {
				return i
			}
		}
	}
	return -1
}

// colonyQueueListHas is Sint_Is_In_List_ (0xc5a84) on a colony's queue
// with n entries: COLBLDG passes 14, reading on past the queue into the
// colony record.
func colonyQueueListHas(c *save.Colony, v int16, n int) bool {
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.LittleEndian, c); err != nil {
		panic(err)
	}
	rec := buf.Bytes()
	for k := 0; k < n; k++ {
		if int16(binary.LittleEndian.Uint16(rec[0x115+2*k:])) == v {
			return true
		}
	}
	return false
}

// autoDesignType is Auto_Design_Type_ (0xb16f0): which support ship an item
// loads into design slot 5.
func autoDesignType(item int16) int16 {
	switch item {
	case bqColonyShip:
		return 1
	case bqOutpost:
		return 4
	case bqTransport:
		return 2
	}
	return 0
}

// loadOutpostShipDesign is Load_Outpost_Ship_Design_ (0x56586).
func (g *Game) loadOutpostShipDesign(p int, d *save.Design) {
	g.loadSupportShipDesign(p, d, outpostNames, 4, 0x2e)
	setBit(d.Specials[:], 0xb)
}

// colbldgCreateShip is Colbldg_Create_Ship_ (0xaf76f): a ship of the
// colony's owner from design slot k, status 6 while queued.
func (g *Game) colbldgCreateShip(k int) int16 {
	s := &g.state
	ci := g.colonyHandle
	sh := g.createShip(int(s.Colonies[ci].Owner), k, ci)
	if sh != -1 {
		s.Ships[sh].Status = 6
	}
	return sh
}

// addToQueue is Add_To_Queue_ (0xb1715): item into the first free slot.
// Ships are created now; a custom design leaves for the design screen.
// While an entry is held the new one goes before it; while the repeat mark
// is placed a repeat follows.
func (g *Game) addToQueue(item int16) (bool, error) {
	s, bq := &g.state, &g.bq
	ci := g.colonyHandle
	c := &s.Colonies[ci]
	slot := -1
	for i, v := range bq.cur {
		if v == -1 {
			slot = i
			break
		}
	}
	if slot == -1 {
		if bq.mode != 0 {
			bq.mode = 0
			g.resetMousePicture()
		}
		return false, nil
	}
	create := func(k int) error {
		sh := g.colbldgCreateShip(k)
		if sh == -1 {
			if err := g.bqHelp(0x21a); err != nil {
				return err
			}
			bq.mode = 0
			return nil
		}
		bq.cur[slot] = -(sh + 100)
		return nil
	}
	switch {
	case (item > 0 && item < 0x31) || bqIsSetting(item) || item == -0xf || item <= -100:
		bq.cur[slot] = item
	case item == -7:
		bq.cur[slot] = item
	case item == -6 || item == -4 || item == -5:
		n := int(c.PopCount) + int(g.nProdsInQueue(-6)) + int(g.nProdsInQueue(-4)) + int(g.nProdsInQueue(-5)) + 1
		if n < int(g.planetMaxPopulationForPlayer(int(c.Planet), int(s.PlayerNum))) {
			bq.cur[slot] = item
		} else {
			ok, err := g.confirmationBox(g.eStr(0x208))
			if err != nil {
				return false, err
			}
			if ok {
				bq.cur[slot] = item
			}
		}
	case item == bqTransport || item == bqColonyShip || item == bqCustomShip || item == bqOutpost || item == bqDoomStar:
		owner := int(c.Owner)
		d := &s.Players[owner].Designs[bqRefitSlot]
		switch autoDesignType(item) {
		case 1:
			g.loadColonyShipDesign(owner, d)
		case 4:
			g.loadOutpostShipDesign(owner, d)
		case 2:
			g.loadTransportShipDesign(owner, d)
		default:
			if item != bqDoomStar {
				bq.fromDesign = true
				g.screenData[4] = 4
				c.AutoBuild = uint8(bq.auto)
				return true, nil
			}
			if g.colonyCanBuildProduct(ci, -(bqRefitSlot+50)) == 0 {
				return true, g.bqHelp(0x245)
			}
		}
		if err := create(bqRefitSlot); err != nil {
			return false, err
		}
	case bqIsDesign(item):
		if g.colonyCanBuildProduct(ci, item) != 0 {
			if err := create(int(-(item + 50))); err != nil {
				return false, err
			}
			break
		}
		owner := &s.Players[c.Owner]
		msg := 0x237
		switch g.state.Players[s.PlayerNum].Designs[-(item + 50)].Size {
		case 4:
			if owner.AppStatus[appTitan] != 3 {
				msg = 0x246
			}
		case 5:
			if owner.AppStatus[appDoomStar] != 3 {
				msg = 0x245
			}
		}
		if err := g.bqHelp(msg); err != nil {
			return false, err
		}
	default:
		return false, g.bqHelp(0x14c)
	}
	if bq.active != -1 && bq.mode != 2 {
		g.insertBeforeQueueSlot(slot, int(int16(bq.active-bqActiveSlot0)))
		g.resetMousePicture()
	}
	if bq.mode == 2 {
		bq.mode = 0
		g.resetMousePicture()
		if !bqIsSetting(item) {
			ok, err := g.addToQueue(-10)
			if err != nil {
				return false, err
			}
			if !ok {
				if err := g.bqHelp(0x188); err != nil {
					return false, err
				}
				if bq.mode != 0 {
					bq.mode = 0
					g.resetMousePicture()
				}
			}
		}
	}
	g.defragmentQueue()
	return true, nil
}

// bqFreeQueuedShip frees a ship that leaves the queue: refits (status 4)
// only after a confirmation; ships still in the colony's own queue stay.
// False when the player keeps it.
func (g *Game) bqFreeQueuedShip(v int16) (bool, error) {
	s := &g.state
	sh := &s.Ships[-(int(v) + 100)]
	if sh.Status == 4 {
		ok, err := g.confirmationBox(g.eStr(0x213))
		if err != nil {
			return false, err
		}
		if !ok {
			return false, g.bqRestore()
		}
	}
	if !colonyQueueListHas(&s.Colonies[g.colonyHandle], v, 0xe) {
		sh.Status = 5
		sh.Location = -1
	}
	return true, g.bqRestore()
}

// deleteFromQueue is Delete_From_Queue_ (0xb1e77): a repeat mark goes with
// the entry before it, an entry with the mark after it.
func (g *Game) deleteFromQueue(slot int) error {
	bq := &g.bq
	if slot == -1 {
		return nil
	}
	if v := bq.cur[slot]; v <= -100 {
		if ok, err := g.bqFreeQueuedShip(v); err != nil || !ok {
			return err
		}
	}
	switch {
	case slot != 0 && bq.cur[slot] == -10:
		if v := bq.cur[slot-1]; v <= -100 {
			if ok, err := g.bqFreeQueuedShip(v); err != nil || !ok {
				return err
			}
		}
		g.squishOutQueueSlot(slot)
		g.squishOutQueueSlot(slot - 1)
	case slot < 6 && bq.cur[slot+1] == -10:
		g.squishOutQueueSlot(slot + 1)
		g.squishOutQueueSlot(slot)
	default:
		g.squishOutQueueSlot(slot)
	}
	return nil
}

// squishOutQueueSlot is Squish_Out_Queue_Slot_ (0xb2150); repeat marks
// moving into the slot go too.
func (g *Game) squishOutQueueSlot(slot int) {
	q := &g.bq.cur
	for {
		for i := slot; i < 6; i++ {
			q[i] = q[i+1]
		}
		q[6] = -1
		if q[slot] != -10 {
			return
		}
	}
}

// defragmentQueue is Defragment_Queue_ (0xb20bd).
func (g *Game) defragmentQueue() {
	for pass := 0; pass < 2; pass++ {
		for i := 0; i < 7; i++ {
			if g.bq.cur[i] == -1 {
				g.squishOutQueueSlot(i)
			}
		}
	}
}

// insertBeforeQueueSlot is Insert_Before_Queue_Slot_ (0xb230d): entry a
// (with its repeat mark) moves before slot b. The mark is squeezed out with
// the entry, so the second squeeze takes one more entry (a quirk).
func (g *Game) insertBeforeQueueSlot(a, b int) {
	bq := &g.bq
	q := &bq.cur
	at := func(i int) int16 {
		if i >= 0 && i < 7 {
			return q[i]
		}
		return -1 // _current_item[7] is _last_app, never -10
	}
	v, next := at(a), at(a+1)
	if at(b) == -10 {
		b--
	}
	if v == -10 {
		a--
		v, next = at(a), at(a+1)
	}
	g.squishOutQueueSlot(a)
	for i := 5; i >= b; i-- {
		q[i+1] = q[i]
	}
	q[b] = v
	if next == -10 {
		k := 0
		if b <= a {
			k = 1
		}
		g.squishOutQueueSlot(a + k)
		for i := 5; i >= b+k; i-- {
			q[i+1] = q[i]
		}
		q[b+k] = next
	}
	if bq.mode != 2 {
		bq.active = -1
	}
}

// removeShips is Remove_Ships_ (0xb23fa): ships created in the popup but
// not kept (colony false), or dropped from the colony's queue (true), are
// freed with their officers.
func (g *Game) removeShips(colony bool) {
	s := &g.state
	ci := g.colonyHandle
	for i := 0; i < 7; i++ {
		var v int16
		if colony {
			v = s.Colonies[ci].Queue[i]
			if v > -100 || g.firstProdQueueSlot(v) != -1 {
				continue
			}
		} else {
			v = g.bq.cur[i]
			if v > -100 || s.firstProdColonyQueueSlot(ci, v) != -1 {
				continue
			}
		}
		sh := &s.Ships[-(int(v) + 100)]
		if sh.Officer != -1 {
			g.moveOfficerToPool(int(sh.Officer))
			sh.Officer = -1
		}
		sh.Status = 5
		sh.Location = -1
	}
}

// doExitScreenCleanup is Do_Exit_Screen_Cleanup_ (0xb2501): the edited
// queue becomes the colony's.
func (g *Game) doExitScreenCleanup() {
	g.removeShips(true)
	g.state.Colonies[g.colonyHandle].Queue = g.bq.cur
}

// somethingBehindSetting is Something_Behind_Settting_ (0xb09ce): an entry
// follows trade goods, housing or a repeat mark.
func (g *Game) somethingBehindSetting() bool {
	found := false
	setting := -1
	for i, v := range g.bq.cur {
		if setting == -1 {
			if v != -1 && bqIsSetting(v) {
				setting = i
			}
		} else if v != -1 {
			found = true
		}
	}
	return found
}

// addRefitedShipToQueue is Add_Refited_Ship_To_Queue_ (0xb2190): the ship
// takes design slot 5 and status 4; its cost becomes the refit cost, twice
// the difference but at least a quarter of the hull.
func (g *Game) addRefitedShipToQueue() error {
	s, bq := &g.state, &g.bq
	k := bq.refitShip
	sh := &s.Ships[k]
	old := sh.Design.Cost
	bq.refitAdd = false
	ok, err := g.addToQueue(-(k + 100))
	if err != nil {
		return err
	}
	if !ok {
		if err := g.bqHelp(0x189); err != nil {
			return err
		}
	} else {
		owner := int(s.Colonies[g.colonyHandle].Owner)
		d := &s.Players[owner].Designs[bqRefitSlot]
		sh.Design = *d
		sh.Status = 4
		floor := int32(g.exeI16(hullCostTable+uint32(sh.Design.Size)*0x24)) / 4
		diff := (int32(int16(g.costOfDesign(owner, d))) - int32(old)) * 2
		sh.Design.Cost = int16(max(floor, diff))
	}
	g.updateShipIconsInBetweenTurns()
	return nil
}
