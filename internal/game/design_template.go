package game

import (
	"encoding/binary"

	"github.com/openorion2/openorion2/internal/save"
)

// The ship design template (_design, 0x125 bytes of _global_data_seg) and
// DESIGN.C's routines that build, cost and store it. Spec:
// docs/screens/design-screen.md, "The template".

// Template offsets.
const (
	dtName        = 0x00  // name, edited in place by the name field
	dtShield      = 0x11  // i16
	dtShieldCost  = 0x13  // i16
	dtShieldSpace = 0x15  // i16
	dtDrive       = 0x1b  // i16
	dtDriveSpeed  = 0x1d  // i16, the drive's speed
	dtDriveCost   = 0x1f  // i16
	dtDriveSpace  = 0x21  // i16
	dtSpeed       = 0x25  // i16 combat speed
	dt29          = 0x29  // i16, cleared
	dtComputer    = 0x2b  // i16
	dtCompCost    = 0x2d  // i16
	dtFuel        = 0x2f  // i16
	dtArmor       = 0x31  // i16
	dtArmorCost   = 0x33  // i16
	dtWeapon      = 0x35  // 8 x i16
	dtCount       = 0x45  // 8 x i16
	dtArc         = 0x55  // 8 x u8
	dtMods        = 0x5d  // 8 x i16
	dtWeaponCost  = 0x6d  // 8 x i32
	dtWeaponSpace = 0x8d  // 8 x i32
	dtAmmo        = 0xad  // 8 x u8
	dtSpecial     = 0xb5  // 8 x i16
	dtSpecialCost = 0xc5  // 8 x i16
	dtSpecialSpc  = 0xd5  // 8 x i16
	dtSize        = 0xe5  // i16
	dtPicture     = 0xe7  // i16
	dtSpaceTotal  = 0xe9  // i32
	dtSpaceUsed   = 0xed  // i32
	dtCost        = 0xf1  // i32
	dtHelpRefresh = 0xf5  // u8: Update_Calculated_Design_Data_ resets the help list
	dtNWeapons    = 0xf6  // i16
	dtNSpecials   = 0xf8  // i16
	dtChoiceOK    = 0xfa  // 0x28 bytes: the replacement boxes' choices that fit
	dtSpecialTop  = 0x122 // u8, the special systems box's first row
	dtSpecialBar  = 0x123 // i16, its thumb position
	dtLen         = 0x125
)

// Constant tables of DESIGN.C (ELF addresses).
const (
	hullSpaceTbl     = 0x180020 // _hull_data+2: i16 space by size
	hullCostTbl      = 0x18001e // _hull_data: i16 cost
	driveSpeedTbl    = 0x17fe9c // _drives+0x2c: i16 speed
	driveAppTbl      = 0x17fe76 // _drives+6: i16 application
	driveCostTbl     = 0x17fe84 // _drives+0x14: i16 cost by size
	driveSpaceTbl    = 0x17fe78 // _drives+8: i16 space by size
	driveMinSpeedTbl = 0x17fe90 // _drives+0x20: u8 combat speed by size
	driveMaxSpeedTbl = 0x17fe96 // _drives+0x26: u8
	shieldCostTbl    = 0x17f6b5 // _shields+0x14: i16 by size
	shieldSpaceTbl   = 0x17f6a9 // _shields+8: i16 by size
	shieldAppTbl     = 0x17f6a7 // _shields+6
	shieldValueTbl   = 0x17f6c1 // _shields+0x20: i16 damage blocked
	armorCostPctTbl  = 0x17f644 // _armor+0xc: i16 percent of the hull cost
	armorHitsPctTbl  = 0x17f642 // _armor+0xa: i16 percent of the hull's hits
	hullArmorTbl     = 0x180024 // _hull_data+6: i16 armour hits
	compCostTbl      = 0x17fdf4 // _computers+8: i16 by size
	compAppTbl       = 0x17fdf2 // _computers+6
	compAttackTbl    = 0x17fe00 // _computers+0x14: i16 beam attack
	specCostTbl      = 0x17eef4 // _specials+0x14: i16 by size
	specSpaceTbl     = 0x17eee8 // _specials+8: i16 by size
	specAppTbl       = 0x17eee6 // _specials+6
	specExcludeTbl   = 0x17ef00 // _specials+0x20: 6 x i16
	wpnAppTbl        = 0x17f80d // _weapons+0xa
	wpnTypeTbl       = 0x17f80f // _weapons+0xc: u8
	wpnSpaceTbl      = 0x17f811 // _weapons+0xe
	wpnMinDmgTbl     = 0x17f817 // _weapons+0x14
	wpnMaxDmgTbl     = 0x17f819 // _weapons+0x16
	wpnModsTbl       = 0x17f81d // _weapons+0x1a: allowed modifications
	wpnTractor       = 0x27     // the tractor beam
	appTypeTbl       = 0x17e085 // _technology_applications+0xc: u8 type
	fieldCostTbl     = 0x17d916 // i32 research cost, 0x17-byte field rows
	modLevelTbl      = 0x17fd14 // u8 miniaturisation level a modification needs
	specDamper       = 7        // the damper field: no shield
	specAugmented    = 2        // augmented engines: +5 combat speed
	specHeavyArmor   = 0xe
	specHardShields  = 0xd
	specShieldPlus   = 0x14
	specReinforced   = 0x1b
	appHullBonus     = 0x59 // known: hull space x125%
	appHeavyMount    = 0x3b // the modification 0xd (heavy mount)
	designSlotSize   = 0x63
)

// designTemplate is _design: a byte array, so routines port with their
// offsets and widths.
type designTemplate [dtLen]byte

func (t *designTemplate) i16(o int) int16      { return int16(binary.LittleEndian.Uint16(t[o:])) }
func (t *designTemplate) set16(o int, v int16) { binary.LittleEndian.PutUint16(t[o:], uint16(v)) }
func (t *designTemplate) i32(o int) int32      { return int32(binary.LittleEndian.Uint32(t[o:])) }
func (t *designTemplate) set32(o int, v int32) { binary.LittleEndian.PutUint32(t[o:], uint32(v)) }
func (t *designTemplate) weapon(k int) int16   { return t.i16(dtWeapon + 2*k) }
func (t *designTemplate) count(k int) int16    { return t.i16(dtCount + 2*k) }
func (t *designTemplate) mods(k int) int16     { return t.i16(dtMods + 2*k) }
func (t *designTemplate) special(k int) int16  { return t.i16(dtSpecial + 2*k) }
func (t *designTemplate) name() []byte         { return t[:cstrLen(t[:])] }
func (t *designTemplate) size() int16          { return t.i16(dtSize) }
func (t *designTemplate) wspace(k int) int32   { return t.i32(dtWeaponSpace + 4*k) }
func (t *designTemplate) arc(k int) uint8      { return t[dtArc+k] }
func (t *designTemplate) ammo(k int) uint8     { return t[dtAmmo+k] }
func (t *designTemplate) spaceLeft() int32     { return t.i32(dtSpaceTotal) - t.i32(dtSpaceUsed) }
func (t *designTemplate) setName(s []byte)     { strcpyBytes(t[:], s) }
func (t *designTemplate) hasSpecial(sp int16) bool {
	for k := 0; k < 8; k++ {
		if t.special(k) == sp {
			return true
		}
	}
	return false
}

// cstrLen is strlen.
func cstrLen(b []byte) int {
	for i, c := range b {
		if c == 0 {
			return i
		}
	}
	return len(b)
}

// strcpyBytes is strcpy of the C string s (up to its NUL) into dst.
func strcpyBytes(dst, s []byte) {
	n := copy(dst, s[:cstrLen(s)])
	if n < len(dst) {
		dst[n] = 0
	}
}

// designRecord is the player's design slot.
func (g *Game) designRecord(p, slot int) *save.Design {
	return &g.state.Players[p].Designs[slot]
}

// designName is Design_Name_ (0x77ae6): the slot's name (a copy in a
// static buffer).
func (g *Game) designName(p, slot int) []byte {
	d := g.designRecord(p, slot)
	return append([]byte(nil), d.Name[:cstrLen(d.Name[:])]...)
}

// hullSpaceFor is Total_Design_Space_ (0x6e81f) for a size: the hull's
// space, 125% with application 0x59.
func (g *Game) hullSpaceFor(size int16) int32 {
	v := int32(g.exeI16(hullSpaceTbl + uint32(int32(size)*0x24)))
	if g.state.Players[g.state.PlayerNum].AppStatus[appHullBonus] == 3 {
		v = v * 0x7d / 100
	}
	return v
}

// buildInitialDesignTemplate is Build_Initial_Design_Template_ (0x6a7c8):
// the template from design slot of player p with the player's best drive,
// computer, armour, fuel and shield.
func (g *Game) buildInitialDesignTemplate(p, slot int) {
	t := &g.des.t
	*t = designTemplate{}
	d := g.designRecord(p, slot)
	t.setName(g.designName(p, slot))
	drive := int16(g.bestWarpDrive(p))
	t.set16(dtDrive, drive)
	t.set16(dtDriveSpeed, g.exeI16(driveSpeedTbl+uint32(int32(drive)*0x2e)))
	t.set16(dtSize, int16(d.Size))
	t.set32(dtSpaceTotal, g.hullSpaceFor(int16(d.Size)))
	t.set16(dtComputer, int16(g.bestComputer(p)))
	t.set16(dtPicture, int16(d.Picture))
	t.set16(dtArmor, int16(g.bestArmor(p)))
	t.set16(dtFuel, int16(g.bestShipFuel(p)))
	t.set16(dtShield, int16(g.bestShipShield(int(g.state.PlayerNum))))
	for k := 0; k < 8; k++ {
		t.set32(dtWeaponSpace+4*k, 0)
		t.set32(dtWeaponCost+4*k, 0)
	}
	t.set16(dtNWeapons, 0)
	for k := 0; k < 8; k++ {
		w := &d.Weapons[k]
		t.set16(dtWeapon+2*k, w.Weapon)
		t.set16(dtCount+2*k, int16(w.Count))
		t.set16(dtMods+2*k, int16(w.Mods))
		t[dtArc+k] = w.Arc
		t[dtAmmo+k] = w.Ammo
		wt := g.exeU8(wpnTypeTbl + uint32(int32(t.weapon(k))*0x1c))
		if wt == 2 || (g.weaponNeedsRacks(t.weapon(k)) && t[dtArc+k]&0xf == 0) {
			if t[dtArc+k] != 0x10 {
				t[dtArc+k] = 0x10
			}
		}
		if t.weapon(k) > 0 {
			t.set16(dtNWeapons, t.i16(dtNWeapons)+1)
		}
	}
	for k := 0; k < 8; k++ {
		t.set16(dtSpecial+2*k, 0)
		t.set16(dtSpecialCost+2*k, 0)
		t.set16(dtSpecialSpc+2*k, 0)
	}
	n := int16(0)
	for b := 0; b < 0x27 && n < 8; b++ {
		if testBit(d.Specials[:], b) {
			t.set16(dtSpecial+2*int(n), int16(b))
			n++
		}
	}
	t.set16(dtNSpecials, n)
	speed := uint8(g.currentDesignBaseCombatSpeed(t))
	if t.hasSpecial(specAugmented) && g.des.augmented {
		speed += 5
	}
	t.set16(dtSpecialBar, 0x8c)
	t.set16(dtSpeed, int16(speed))
	g.updateCalculatedDesignData(p)
}

// weaponNeedsRacks is Weapon_Needs_Racks_ (0x6edae): type 1 (missiles).
func (g *Game) weaponNeedsRacks(w int16) bool {
	return g.exeU8(wpnTypeTbl+uint32(int32(w)*0x1c)) == 1
}

// weaponTypeOf is Weapon_Type_ (0x6efeb).
func (g *Game) weaponTypeOf(w int16) uint16 {
	return uint16(g.exeU8(wpnTypeTbl + uint32(int32(w)*0x1c)))
}

// currentDesignBaseCombatSpeed is Current_Design_Base_Combat_Speed_
// (0x6b86a): the drive's speed for the hull, slowed by the space used, not
// below the minimum.
func (g *Game) currentDesignBaseCombatSpeed(t *designTemplate) int32 {
	row := uint32(int32(t.i16(dtDrive))*0x2e + int32(t.size()))
	lo := uint16(g.exeU8(driveMinSpeedTbl + row))
	hi := g.exeU8(driveMaxSpeedTbl + row)
	used := t.i32(dtSpaceUsed) * 100 / t.i32(dtSpaceTotal)
	v := int32(hi) - used*int32(int16(uint16(hi)-lo))/100
	if int16(lo) < int16(v) {
		return v
	}
	return int32(lo)
}

// currentDesignCombatSpeed is Current_Design_Combat_Speed_ (0x6b7cf).
func (g *Game) currentDesignCombatSpeed(t *designTemplate) uint8 {
	v := uint8(g.currentDesignBaseCombatSpeed(t))
	if t.hasSpecial(specAugmented) && g.des.augmented {
		v += 5
	}
	return v
}

// hasDamperField is Has_Damper_Field_ (0x6ad3e): a damper field takes the
// shield out.
func (g *Game) hasDamperField() {
	if g.des.t.hasSpecial(specDamper) {
		g.des.t.set16(dtShield, 0)
	}
}

// updateCalculatedDesignData is Update_Calculated_Design_Data_ (0x6e8da).
func (g *Game) updateCalculatedDesignData(p int) {
	t := &g.des.t
	g.hasDamperField()
	g.designTemplateCosts(p)
	g.designTemplateSpaceRequirements(p)
	t.set32(dtCost, g.totalDesignCost())
	t.set32(dtSpaceUsed, g.totalSpaceUsed())
	t.set32(dtSpaceTotal, g.hullSpaceFor(t.size()))
	var nw, ns uint8
	for k := 0; k < 8; k++ {
		if t.weapon(k) > 0 {
			nw++
		}
	}
	g.des.nWeapons = nw
	for k := 0; k < 8; k++ {
		if t.special(k) > 0 {
			ns++
		}
	}
	g.des.nSpecials = ns
	g.des.printedSpc = t.spaceLeft()
	g.des.printedCst = t.i32(dtCost)
	g.updateNSpecSystemsInDesign()
	t.set16(dtNWeapons, 0)
	for k := 0; k < 8; k++ {
		if t.weapon(k) > 0 {
			t.set16(dtNWeapons, t.i16(dtNWeapons)+1)
		}
	}
	if t[dtHelpRefresh] != 0 {
		g.setDesignScreenHelpList(t.i16(dtNWeapons), t.i16(dtNSpecials))
		t[dtHelpRefresh] = 0
	}
}

// updateNSpecSystemsInDesign is Update_N_Spec_Systems_In_Design_ (0x6ab4e).
func (g *Game) updateNSpecSystemsInDesign() {
	t := &g.des.t
	t.set16(dtNSpecials, 0)
	for k := 0; k < 8; k++ {
		if t.special(k) > 0 {
			t.set16(dtNSpecials, t.i16(dtNSpecials)+1)
			if t.i16(dtNSpecials) > 7 {
				break
			}
		}
	}
}

// designTemplateCosts is Design_Template_Costs_ (0x6c087).
func (g *Game) designTemplateCosts(p int) {
	t := &g.des.t
	sh := t.i16(dtShield)
	c := g.exeI16(shieldCostTbl + uint32(int32(sh)*0x3b+2*int32(t.size())))
	if sh > 0 && c == 0 {
		c = 1
	}
	t.set16(dtShieldCost, costGivenMLevel(c, g.techLevel(p, int(g.exeI16(shieldAppTbl+uint32(int32(sh)*0x3b))))))
	dr := int32(t.i16(dtDrive)) * 0x2e
	lvl := g.techLevel(p, int(g.exeI16(driveAppTbl+uint32(dr))))
	t.set16(dtDriveCost, costGivenMLevel(g.exeI16(driveCostTbl+uint32(dr+2*int32(t.size()))), lvl))
	hull := int32(g.exeI16(hullCostTbl + uint32(int32(t.size())*0x24)))
	t.set16(dtArmorCost, int16(hull*int32(g.exeI16(armorCostPctTbl+uint32(int32(t.i16(dtArmor))*0xf)))/100))
	for k := 0; k < 8; k++ {
		if t.weapon(k) > 0 {
			t.set32(dtWeaponCost+4*k, int32(g.weaponCost(t.weapon(k), t.count(k), t.arc(k), t.ammo(k), t.mods(k))))
		}
	}
	for k := 0; k < 8; k++ {
		sp := int32(t.special(k)) * 0x2f
		c := g.exeI16(specCostTbl + uint32(sp+2*int32(t.size())))
		switch {
		case t.special(k) <= 0:
		case t.special(k) == 4:
			t.set16(dtSpecialCost+2*k, c)
		default:
			t.set16(dtSpecialCost+2*k, costGivenMLevel(c, g.techLevel(p, int(g.exeI16(specAppTbl+uint32(sp))))))
		}
	}
	cp := t.i16(dtComputer)
	c = g.exeI16(compCostTbl + uint32(int32(cp)*0x16+2*int32(t.size())))
	if c == 0 && cp > 0 {
		c = 1
	}
	t.set16(dtCompCost, costGivenMLevel(c, g.techLevel(p, int(g.exeI16(compAppTbl+uint32(int32(t.i16(dtComputer))*0x16))))))
}

// designTemplateSpaceRequirements is Design_Template_Space_Requirements_
// (0x6c287); the shield's level is _PLAYER_NUM's, the rest p's.
func (g *Game) designTemplateSpaceRequirements(p int) {
	t := &g.des.t
	dr := int32(t.i16(dtDrive)) * 0x2e
	app := g.exeI16(driveAppTbl + uint32(dr))
	cat := g.miniaturizationCategory(app)
	t.set16(dt29, 0)
	t.set16(dtDriveSpace, spaceGivenMLevel(g.exeI16(driveSpaceTbl+uint32(dr+2*int32(t.size()))), g.techLevel(p, int(app)), cat))
	for k := 0; k < 8; k++ {
		if t.weapon(k) > 0 {
			t.set32(dtWeaponSpace+4*k, g.weaponSpace(int(g.state.PlayerNum), int(t.weapon(k)), t.count(k), int16(t.arc(k)), int16(t.ammo(k)), t.mods(k)))
		}
	}
	sh := t.i16(dtShield)
	sp := g.shieldSpace(sh, t.size())
	t.set16(dtShieldSpace, spaceGivenMLevel(sp, g.techLevel(int(g.state.PlayerNum), int(g.exeI16(shieldAppTbl+uint32(int32(sh)*0x3b)))), 0))
	for k := 0; k < 8; k++ {
		s := t.special(k)
		switch {
		case s <= 0:
		case s == 4:
			t.set16(dtSpecialSpc+2*k, g.exeI16(specSpaceTbl+uint32(int32(s)*0x2f+2*int32(t.size()))))
		default:
			app := g.exeI16(specAppTbl + uint32(int32(s)*0x2f))
			cat := g.miniaturizationCategory(app)
			lvl := g.techLevel(p, int(app))
			t.set16(dtSpecialSpc+2*k, spaceGivenMLevel(g.exeI16(specSpaceTbl+uint32(int32(t.special(k))*0x2f+2*int32(t.size()))), lvl, cat))
		}
	}
}

// shieldSpace is Shield_Space_ (0x6e369): at least 1 for a shield.
func (g *Game) shieldSpace(sh, size int16) int16 {
	v := g.exeI16(shieldSpaceTbl + uint32(int32(sh)*0x3b+2*int32(size)))
	if sh > 0 && v == 0 {
		v = 1
	}
	return v
}

// totalDesignCost is Total_Design_Cost_ (0x6e777).
func (g *Game) totalDesignCost() int32 {
	t := &g.des.t
	base := int32(t.i16(dtShieldCost)) + int32(g.exeI16(hullCostTbl+uint32(int32(t.size())*0x24))) +
		int32(t.i16(dtDriveCost)) + int32(t.i16(dtArmorCost)) + int32(t.i16(dtCompCost))
	var w, s int32
	for k := 0; k < 8; k++ {
		if t.weapon(k) > 0 {
			w += t.i32(dtWeaponCost + 4*k)
		}
	}
	for k := 0; k < 8; k++ {
		if t.special(k) > 0 {
			s += int32(t.i16(dtSpecialCost + 2*k))
		}
	}
	return g.costReductionForGovtType(int(g.state.PlayerNum), s+base+w)
}

// totalSpaceUsed is Total_Space_Used_ (0x6e862).
func (g *Game) totalSpaceUsed() int32 {
	t := &g.des.t
	v := int32(t.i16(dtShieldSpace)) + int32(t.i16(dtDriveSpace))
	var w, s int32
	for k := 0; k < 8; k++ {
		if t.weapon(k) > 0 {
			w += t.wspace(k)
		}
	}
	for k := 0; k < 8; k++ {
		if t.special(k) > 0 {
			s += int32(t.i16(dtSpecialSpc + 2*k))
		}
	}
	return v + w + s
}

// clearDesign is Clear_Design_ (0x6b247): the best equipment, no weapons
// and no specials.
func (g *Game) clearDesign() {
	t := &g.des.t
	p := int(g.state.PlayerNum)
	drive := int16(g.bestWarpDrive(p))
	t.set16(dtDrive, drive)
	t.set16(dtDriveSpeed, g.exeI16(driveSpeedTbl+uint32(int32(drive)*0x2e)))
	t.set16(dtComputer, int16(g.bestComputer(p)))
	t.set16(dtArmor, int16(g.bestArmor(p)))
	t.set16(dtFuel, int16(g.bestShipFuel(p)))
	for k := 0; k < 8; k++ {
		t.set16(dtWeapon+2*k, 0)
		t.set16(dtCount+2*k, 0)
		t[dtArc+k] = 0
		t.set16(dtMods+2*k, 0)
		t[dtAmmo+k] = 0
		t.set32(dtWeaponSpace+4*k, 0)
		t.set32(dtWeaponCost+4*k, 0)
	}
	for k := 0; k < 8; k++ {
		t.set16(dtSpecial+2*k, 0)
		t.set16(dtSpecialCost+2*k, 0)
		t.set16(dtSpecialSpc+2*k, 0)
	}
	g.updateCalculatedDesignData(p)
}

// clearWeaponSlot is Clear_Weapon_Slot_ (0x6b428): the slots after k move
// up, the last loaded one is cleared.
func (g *Game) clearWeaponSlot(k int16) {
	t := &g.des.t
	n := int32(g.des.nWeapons)
	for ; int32(k) < n-1; k++ {
		i := int(k)
		copy(t[dtWeapon+2*i:dtWeapon+2*i+2], t[dtWeapon+2*i+2:])
		copy(t[dtCount+2*i:dtCount+2*i+2], t[dtCount+2*i+2:])
		t[dtArc+i] = t[dtArc+i+1]
		copy(t[dtMods+2*i:dtMods+2*i+2], t[dtMods+2*i+2:])
		copy(t[dtWeaponCost+4*i:dtWeaponCost+4*i+4], t[dtWeaponCost+4*i+4:])
		copy(t[dtWeaponSpace+4*i:dtWeaponSpace+4*i+4], t[dtWeaponSpace+4*i+4:])
		t[dtAmmo+i] = t[dtAmmo+i+1]
	}
	l := int(n) - 1
	t.set16(dtWeapon+2*l, 0)
	t.set16(dtCount+2*l, 0)
	t[dtArc+l] = 0
	t.set16(dtMods+2*l, 0)
	t.set32(dtWeaponCost+4*l, 0)
	t.set32(dtWeaponSpace+4*l, 0)
	t[dtAmmo+l] = 0
}

// clearSpecialSystemSlot is Clear_Special_System_Slot_ (0x6b3b4).
func (g *Game) clearSpecialSystemSlot(k int16) {
	t := &g.des.t
	n := int32(g.des.nSpecials)
	for ; int32(k) < n-1; k++ {
		i := int(k)
		t.set16(dtSpecial+2*i, t.special(i+1))
		t.set16(dtSpecialCost+2*i, t.i16(dtSpecialCost+2*i+2))
		t.set16(dtSpecialSpc+2*i, t.i16(dtSpecialSpc+2*i+2))
	}
	l := int(n) - 1
	t.set16(dtSpecial+2*l, 0)
	t.set16(dtSpecialCost+2*l, 0)
	t.set16(dtSpecialSpc+2*l, 0)
}

// trimName is Trim_ (0x13303e) on the template's name: leading and
// trailing blanks go.
func trimName(b []byte) {
	i := 0
	for b[i] == ' ' {
		i++
	}
	copy(b, b[i:cstrLen(b)+1])
	for k := cstrLen(b) - 1; k >= 0 && b[k] == ' '; k-- {
		b[k] = 0
	}
}

// updatePlayerDesign is Update_Player_Design_ (0x6ea48): BUILD stores the
// template in design slot of player p. The type byte and builder stay.
func (g *Game) updatePlayerDesign(p, slot int) {
	t := &g.des.t
	trimName(t[:])
	if t[0] == 0 {
		t.setName(g.designName(int(g.state.PlayerNum), slot))
	}
	d := g.designRecord(p, slot)
	strcpyBytes(d.Name[:], t.name())
	d.Size = t[dtSize]
	d.Shield = t[dtShield]
	d.Drive = t[dtDrive]
	d.Computer = t[dtComputer]
	d.Armor = t[dtArmor]
	d.Specials = [5]byte{}
	for k := 0; k < 8; k++ {
		if t.special(k) > 0 {
			setBit(d.Specials[:], int(t.special(k)))
		}
	}
	for k := 0; k < 8; k++ {
		w := &d.Weapons[k]
		w.Weapon = t.weapon(k)
		w.Count = t[dtCount+2*k]
		w.Working = w.Count
		w.Arc = t[dtArc+k]
		w.Mods = uint16(t.mods(k))
		w.Ammo = t[dtAmmo+k]
	}
	d.Picture = t[dtPicture]
	d.Cost = t.i16(dtCost)
	d.BuildDate = uint16(g.state.Stardate)
	g.des.augmented = false
	d.CombatSpeed = g.currentDesignCombatSpeed(t)
	d.FTLSpeed = uint8(g.ftlSpeed(p, int(t.i16(dtDrive))))
}

// buildDesignTemplate is Build_Design_Template_ (0x6a6a1): the template as
// a 99-byte design record.
func (g *Game) buildDesignTemplate() save.Design {
	t := &g.des.t
	var d save.Design
	strcpyBytes(d.Name[:], t.name())
	d.Size = t[dtSize]
	d.Type = 0
	d.Drive = t[dtDrive]
	d.Computer = t[dtComputer]
	d.Armor = t[dtArmor]
	for k := 0; k < 8; k++ {
		if t.special(k) > 0 {
			setBit(d.Specials[:], int(t.special(k)))
		}
	}
	for k := 0; k < 8; k++ {
		w := &d.Weapons[k]
		w.Weapon = t.weapon(k)
		w.Count = t[dtCount+2*k]
		w.Arc = t[dtArc+k]
		w.Mods = uint16(t.mods(k))
		w.Ammo = t[dtAmmo+k]
	}
	d.Picture = t[dtPicture]
	d.Cost = t.i16(dtCost)
	d.BuildDate = uint16(g.state.Stardate)
	speed := uint8(g.currentDesignBaseCombatSpeed(t))
	if t.hasSpecial(specAugmented) && g.des.augmented {
		speed += 5
	}
	d.CombatSpeed = speed
	return d
}
