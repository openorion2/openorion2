package game

import "github.com/openorion2/openorion2/internal/save"

// Ship strengths as the AI rates them (AIPOWER.C). Spec: docs/turn/ai-data.md.

const (
	planetShieldLevels = 0x5eae1  // 4 x i16, the planetary shields bombs are rated against
	pdMinMiniaturize   = 0x17fd32 // u8, miniaturisation a point defence weapon needs
)

// shipStrengthVsShips is Ship_Strength_Vs_Ships_ (0x5ef17): against player
// p's best ship shield.
func (g *Game) shipStrengthVsShips(k, p int) uint32 {
	return g.shipStrengthVsShield(k, p, g.bestShipShieldValue(int16(p)))
}

// shipStrengthVsShipsInNebula is Ship_Strength_Vs_Ships_In_Nebula_ (0x5ef09):
// shields do not work there.
func (g *Game) shipStrengthVsShipsInNebula(k, p int) uint32 {
	return g.shipStrengthVsShield(k, p, 0)
}

// bestShipShieldValue is Get_Best_Ship_Shield_Value_ (0x5f871).
func (g *Game) bestShipShieldValue(p int16) int16 {
	switch {
	case p < 8:
		return g.exeI16(rowAddr(shieldTable, shieldStride, g.bestShipShield(int(p)), shieldStrength))
	case p == 8:
		return 6
	case p == 9:
		return 8
	case p <= 14:
		return 0
	}
	return p // the original returns its argument register
}

// shipStrengthVsShield is Ship_Strength_Vs_Shield_ (0x5ef4b): ship k's
// weapons against shield, scaled by its own shield and remaining hits.
func (g *Game) shipStrengthVsShield(k, player int, shield int16) uint32 {
	s := &g.state
	sh := &s.Ships[k]
	owner := int16(sh.Owner)
	strategic := g.settings[0xd8] != 0
	var attack int16
	if strategic {
		attack = g.exeI16(rowAddr(computerTable, computerStride, g.bestComputer(int(owner)), computerBonus))
		if owner < 8 {
			attack += int16(s.Players[owner].Picks[save.PickShipAttack])
		}
		g.updateStrategicShip(k)
	} else {
		_, _, attack = g.getShipCombatBonuses(k)
	}
	dcv := g.baseGenericDcv(int16(player))
	total := uint32(0)
	for w := 0; w < 8; w++ {
		ws := &sh.Design.Weapons[w]
		bx := ws.Weapon
		if bx <= 0 {
			continue
		}
		cx := int32(ws.Working)
		lo := uint8(ws.Mods)
		var n int32
		if lo&0x40 != 0 || bx == 0x22 || bx == 0xd {
			n = cx * int32(g.wpnI16(int(bx), weaponStrength))
		} else {
			c := cx
			switch {
			case bx == 0x1f:
				c = cx * 3
				bx = int16(g.bestPointDefenseWeapon(owner))
			case bx == 0x1e || bx == 0x1d:
				c = cx * 3
				bx = int16(g.bestBombWeapon(int(owner)))
			case bx == 0x1c:
				bx = 0x1b
				c = cx * 3
			case bx == 0x24:
				c = cx * 3
			}
			str := g.wpnI16(int(bx), weaponStrength)
			if lo&2 != 0 {
				str = int16((int32(str)*0x32+0x32)/100 + int32(str))
			} else if lo&4 != 0 {
				str = int16((int32(str)*0x32 + 0x32) / 100)
			}
			d := int32(str) - int32(shield)
			if d < 0 {
				d = 0
			}
			n = int32(int16(c)) * d
		}
		if n != 0 {
			if t := g.wpnU8(int(bx), weaponType); t <= 5 {
				q := uint32(n*3) >> 2
				switch t {
				case 0:
					v := attack + 0x32 - dcv
					if ws.Mods != 0 {
						v += 0x19
					}
					v = min(max(v, 10), 100)
					n = int32(uint32(int32(v)*n) / 100)
				case 1:
					if ws.Ammo == 2 {
						n = int32(q)
					}
				case 2:
					n = int32(q)
				case 3:
					n = 0
				}
			}
		}
		total += uint32(uint16(g.modifiedWeaponStrength(uint16(n), ws.Mods)))
	}
	d := int16(g.bestShipShield(int(owner))) - int16(sh.Design.Shield)
	f := int32(100)
	if d < 0 {
		f = 100 - abs32(int32(d))*10
	}
	if int16(f) <= 10 {
		f = 10
	}
	total = uint32(int32(int16(f))*int32(total)) / 100
	if strategic {
		return min(total, 0xfa00)
	}
	hits := int16(g.maxShipHits(k)) - sh.StructureDamage - sh.ArmorDamage
	pos := uint32(0)
	if total > 0 {
		pos = 1
	}
	if t := uint32(int32(strengthHitsModifier(hits))*int32(total)) / 100; pos < t {
		return min(t, 0xfa00)
	}
	return min(pos, 0xfa00)
}

// strengthHitsModifier is Strength_Hits_Modifier_ (0x5eed4).
func strengthHitsModifier(hits int16) int16 {
	if hits <= 100 {
		return int16((100-int32(hits))/10 + int32(hits))
	}
	return int16((int32(hits)-100)/4 + 100)
}

// baseGenericDcv is Base_Generic_Dcv_ (0x5eae9): the beam defence a player's
// generic ship would have, by its best drive.
func (g *Game) baseGenericDcv(p int16) int16 {
	v := int16(int8(g.exeU8(rowAddr(driveTable, driveStride, g.bestWarpDrive(int(p)), driveDefence)))) * 5
	if p < 8 {
		P := &g.state.Players[p]
		v += int16(P.Picks[save.PickShipDefense])
		if P.Picks[save.PickTransDimensional] != 0 {
			v += 0x14
		}
	}
	return v
}

// bestPointDefenseWeapon is Best_Point_Defense_Weapon_ (0x5699c).
func (g *Game) bestPointDefenseWeapon(owner int16) int {
	if owner >= 8 {
		return 4
	}
	best, val := 3, int16(0)
	for i := int16(0); i < 0x28; i++ {
		app := g.wpnI16(int(i), weaponApp)
		if !g.playerKnows(int(owner), app) || g.wpnU8(int(i), weaponFlags)&4 == 0 || g.wpnU8(int(i), weaponType) != 0 {
			continue
		}
		if g.miniaturizationLevel(int(owner), int(app)) < int16(g.exeU8(pdMinMiniaturize)) {
			continue
		}
		if v := g.wpnI16(int(i), weaponStrength); v > val {
			best, val = int(i), v
		}
	}
	return best
}

// modifiedWeaponStrength is Modified_Weapon_Strength_ (0x5ee27): the percent
// bonuses of weapon modification bits 1, 2, 5, 7 and 8..14.
func (g *Game) modifiedWeaponStrength(v uint16, mods uint16) uint32 {
	pct := int16(100)
	for _, b := range []uint{1, 2, 5, 7, 8, 9, 10, 11, 12, 13, 14} {
		if mods>>b&1 != 0 {
			pct += g.exeI16(uint32(modTable + int(b)*modStride + 0xc))
		}
	}
	r := uint32(int32(pct) * int32(v) / 100)
	return min(r, 0xfa00)
}

// shipStrengthVsPlanetaryShield is Ship_Strength_Vs_Planetary_Shield_
// (0x5f379) writing to arena offset out.
func (g *Game) shipStrengthVsPlanetaryShield(k, out int) {
	v := g.planetaryShieldStrengths(k)
	for j := range v {
		g.ai.setU16(out+j*2, v[j])
	}
}

// planetaryShieldStrengths is Ship_Strength_Vs_Planetary_Shield_ (0x5f379):
// bombing strength per planetary shield level.
func (g *Game) planetaryShieldStrengths(k int) (out [4]uint16) {
	s := &g.state
	sh := &s.Ships[k]
	var acc [4]int32
	if g.settings[0xd8] == 1 {
		v := int32(sh.Design.Size) * int32(sh.Design.Size) * 10
		acc[0] = v
		for i := 1; i < 4; i++ {
			acc[i] = int32((uint32(v) + uint32(i) - 1) / uint32(i))
		}
	} else {
		for w := 0; w < 8; w++ {
			ws := &sh.Design.Weapons[w]
			if ws.Weapon <= 0 {
				continue
			}
			str := g.wpnI16(int(ws.Weapon), weaponStrength)
			if t := g.wpnU8(int(ws.Weapon), weaponType); t == 0 || t == 2 {
				str = int16(int32(str) / 2)
			}
			for j := 0; j < 4; j++ {
				lv := g.exeI16(uint32(planetShieldLevels + j*2))
				if d := (int32(str) - int32(lv)) * int32(ws.Working); d > 0 {
					acc[j] += d
				}
			}
		}
	}
	for j := 0; j < 4; j++ {
		cur := int32(int16(g.currentShipHits(k)))
		v := uint32(acc[j]*cur) / uint32(int32(int16(g.maxShipHits(k))))
		out[j] = uint16(min(v, 0xfa00))
	}
	return out
}

// shipHasStealthDevice is Ship_Has_Stealth_Device_ (0x5d3db).
func (s *State) shipHasStealthDevice(k int) bool {
	return s.shipHasSpecial(k, 0x1f) || s.shipHasSpecial(k, 6) || s.shipHasSpecial(k, 0x17)
}
