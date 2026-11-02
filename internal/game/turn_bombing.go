package game

import (
	"bytes"
	"encoding/binary"

	"github.com/openorion2/openorion2/internal/save"
)

// Colony bombing (BOMB.C): the 0x49-byte hit record that Bombardment_ and the
// events fill, and the routines that spend it. Spec: docs/turn/overview.md, "Bombing".

// bombHits is the 0x49-byte bomb-hit record; offsets in the comments.
type bombHits struct {
	Attacker      int8      // +0x00 player, -1 for an event
	Colony        uint8     // +0x01
	Kind          uint8     // +0x02 1 from Bombardment_ and the events
	Hits          int16     // +0x03 bomb hits to spend
	Accident      uint8     // +0x05 1 for an industrial accident
	BioHits       int16     // +0x06 biological hits to spend
	MarinesKilled int16     // +0x08
	ArmorKilled   int16     // +0x0a
	PopKilled     int16     // +0x0c
	Destroyed     [49]uint8 // +0x0e 1 by building id
	BioPopKilled  int16     // +0x3f
	Hit           uint8     // +0x41 something was hit
	Emptied       uint8     // +0x42 the last colonist died
	Production    int16     // +0x43 production lost
	HitCost       int32     // +0x45 bomb hits used on troops
}

// Building ids a bomb never picks: Resolve_Bomb_Hit_ skips the capitol, the
// ship yards and four others (they go with the colony, if ever).
func bombSpared(b int) bool {
	switch b {
	case 8, 9, 0x1a, 0x1b, 0x28, 0x29, 0x2a, 0x2f:
		return true
	}
	return false
}

// addWarLosses2 adds to the colony owner's WarLosses2[att]; the events pass
// attacker -1, which the original indexes as WarLosses[7], the word below.
func (s *State) addWarLosses2(owner int, att int8, v int16) {
	p := &s.Players[owner]
	if att < 0 {
		p.WarLosses[7] += v
		return
	}
	if att < 8 {
		p.WarLosses2[att] += v
		return
	}
	// Monsters (8..14) index past the array into the fields after it.
	b := playerRecordBytes(p)
	off := 0x6ff + 2*int(att)
	binary.LittleEndian.PutUint16(b[off:], binary.LittleEndian.Uint16(b[off:])+uint16(v))
	setPlayerRecordBytes(p, b)
}

func playerRecordBytes(p *save.Player) []byte {
	var b bytes.Buffer
	binary.Write(&b, binary.LittleEndian, p)
	return b.Bytes()
}

func setPlayerRecordBytes(p *save.Player, b []byte) {
	binary.Read(bytes.NewReader(b), binary.LittleEndian, p)
}

// applyBombHits is Apply_Bomb_Hits_ (0xdd2f2): biological hits go first,
// then bombs until they are spent or nothing is left to hit.
func (g *Game) applyBombHits(r *bombHits) {
	ci := int(r.Colony)
	hits, bio := int32(r.Hits), int32(r.BioHits)
	for hits+bio > 0 {
		if bio != 0 {
			g.resolveBiologicalHit(ci, r)
			bio--
			continue
		}
		if n := g.resolveBombHit(ci, r, hits); n != 0 {
			hits -= n
			r.HitCost += n
		} else {
			hits = 0
		}
	}
}

// resolveBombHit is Resolve_Bomb_Hit_ (0xdcebd): one random target among
// the buildings, marines, tanks, stored production and colonists. It returns
// the hits used, 0 when troops need more hits than are left.
func (g *Game) resolveBombHit(ci int, r *bombHits, hits int32) int32 {
	s := &g.state
	c := &s.Colonies[ci]
	owner := &s.Players[c.Owner]
	var list [0x40]uint8
	nb := int32(0)
	for b := 0x30; b >= 0; b-- {
		if c.Buildings[b] != 0 && !bombSpared(b) {
			list[nb] = uint8(b)
			nb++
		}
	}
	targets := nb + int32(c.Marines) + int32(c.Armor)
	if c.Production != 0 {
		targets++
	}
	if c.PopCount == 1 {
		if targets == 0 {
			// The last colonist goes only when nothing else is left.
			race := c.Colonists[0] & 0xf
			if c.PopFraction[race] > 100 {
				c.PopFraction[race] -= 100
			} else {
				r.PopKilled++
				c.PopFraction[race] = 0
				c.PopCount = 0
				if c.Buildings[bldCapitol] != 0 {
					g.removeBuilding(ci, bldCapitol)
				}
				r.Emptied = 1
				s.addWarLosses2(int(c.Owner), r.Attacker, 10)
			}
			r.Hit = 1
			return 1
		}
	} else {
		targets += int32(c.PopCount)
	}
	if targets == 0 {
		return 0
	}
	k := int32(s.Random.Random(uint32(targets))) - 1
	if k < nb {
		g.removeBuilding(ci, int(list[k]))
		r.Destroyed[list[k]] = 1
		r.Hit = 1
		return 1
	}
	k -= nb
	if k < int32(c.Marines) {
		cost := int32(1)
		if owner.AppStatus[0x1a7-0x117] == 3 {
			cost = 2
		}
		if owner.Picks[11] != 0 {
			cost++
		}
		if cost > hits {
			return 0
		}
		c.Marines--
		r.Hit = 1
		r.MarinesKilled++
		return cost
	}
	k -= int32(c.Marines)
	if k < int32(c.Armor) {
		cost := int32(2)
		if owner.AppStatus[0x12f-0x117] == 3 {
			cost = 3
		}
		if owner.Picks[11] != 0 {
			cost++
		}
		if cost > hits {
			return 0
		}
		c.Armor--
		r.Hit = 1
		r.ArmorKilled++
		return cost
	}
	k -= int32(c.Armor)
	// The production slot is tested against the amount, not against 1, and
	// the colonist index is not reduced by it.
	if k < int32(c.Production) {
		r.Production = c.Production
		c.Production = 0
	} else {
		r.PopKilled++
		c.PopCount--
		c.Colonists[k] = c.Colonists[c.PopCount]
		s.addWarLosses2(int(c.Owner), r.Attacker, 1)
	}
	r.Hit = 1
	return 1
}

// resolveBiologicalHit is Resolve_Biological_Hit_ (0xdd13e): a colonist
// other than an android is picked; the hit falls on it or on the troops.
func (g *Game) resolveBiologicalHit(ci int, r *bombHits) {
	s := &g.state
	c := &s.Colonies[ci]
	victim, n := int32(-1), uint32(0)
	for i := int32(c.PopCount) - 1; i >= 0; i-- {
		if c.Colonists[i]&0xf == 8 {
			continue
		}
		n++
		if s.Random.Random(n) == 1 {
			victim = i
		}
	}
	if victim < 0 {
		return
	}
	r.Hit = 1
	marines, armor := int32(c.Marines), int32(c.Armor)
	if c.PopCount >= 2 {
		pop := int32(c.PopCount)
		if int32(s.Random.Random(uint32(pop+marines+armor))) <= pop {
			r.PopKilled++
			r.BioPopKilled++
			c.PopCount--
			c.Colonists[victim] = c.Colonists[c.PopCount]
			s.addWarLosses2(int(c.Owner), r.Attacker, 1)
			return
		}
		if marines != 0 && int32(s.Random.Random(uint32(marines+armor))) <= marines {
			c.Marines--
			r.MarinesKilled++
			return
		}
		c.Armor--
		r.ArmorKilled++
		return
	}
	if marines != 0 && int32(s.Random.Random(uint32(marines+armor))) <= marines {
		c.Marines--
		r.MarinesKilled++
		return
	}
	if armor != 0 {
		c.Armor--
		r.ArmorKilled++
		return
	}
	// The fraction tested is the first colonist's race, the one cut the victim's.
	if c.PopFraction[c.Colonists[0]&0xf] > 100 {
		c.PopFraction[c.Colonists[victim]&0xf] -= 100
		return
	}
	r.PopKilled++
	r.BioPopKilled++
	c.PopFraction[c.Colonists[0]&0xf] = 0
	c.PopCount = 0
	for b := 0x30; b >= 0; b-- {
		if c.Buildings[b] != 0 {
			return
		}
	}
	r.Emptied = 1
	s.addWarLosses2(int(c.Owner), r.Attacker, 10)
}

// colonyBuildingsAndPop counts a colony's buildings plus its colonists, the
// base of the event hit counts.
func (s *State) colonyBuildingsAndPop(ci int) int32 {
	c := &s.Colonies[ci]
	n := int32(0)
	for b := 0; b < 0x31; b++ {
		if c.Buildings[b] != 0 {
			n++
		}
	}
	return n + int32(c.PopCount)
}

// eventBombHits builds an event's hit record (attacker -1, kind 1).
func eventBombHits(ci int) bombHits { return bombHits{Attacker: -1, Colony: uint8(ci), Kind: 1} }

// allocateCometHit is Allocate_Comet_Hit_ (0x23780): (2..4)/10 of the
// colony's buildings and colonists in bomb hits, at least one.
func (g *Game) allocateCometHit(ci int) {
	s := &g.state
	r := eventBombHits(ci)
	n := int16(s.colonyBuildingsAndPop(ci))
	a := int32(s.Random.Random(3))
	a += int32(s.Random.Random(3))
	h := int16(a * int32(n) / 10)
	if h == 0 {
		h = 1
	}
	r.Hits = h
	g.applyBombHits(&r)
}

// allocateIndustrialAccidentHit is Allocate_Industrial_Accident_Hit_
// (0x23833): one bomb and (2..4)/10 of the colonists in biological hits.
func (g *Game) allocateIndustrialAccidentHit(ci int) {
	s := &g.state
	r := eventBombHits(ci)
	r.Hits = 1
	r.Accident = 1
	pop := int32(s.Colonies[ci].PopCount)
	a := int32(s.Random.Random(3))
	a += int32(s.Random.Random(3))
	r.BioHits = int16(a * pop / 10)
	g.applyBombHits(&r)
}

// allocateEarthquakeHit is Allocate_Earthquake_Hit_ (0x238a8): like the
// comet with Random(3)+Random(2); it returns colonists and buildings lost.
func (g *Game) allocateEarthquakeHit(ci int) (pop, buildings int16) {
	s := &g.state
	r := eventBombHits(ci)
	n := int16(s.colonyBuildingsAndPop(ci))
	a := int32(s.Random.Random(3))
	a += int32(s.Random.Random(2))
	h := int16(a * int32(n) / 10)
	if h == 0 {
		h = 1
	}
	r.Hits = h
	g.applyBombHits(&r)
	for _, d := range r.Destroyed {
		if d == 1 {
			buildings++
		}
	}
	return r.PopKilled, buildings
}
