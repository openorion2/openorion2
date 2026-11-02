package save

// Ship, design, leader and moveable box records. Spec:
// docs/formats/save-ships.md.

// Design is a ship design (99 bytes), stored in ships and, six per player,
// in the player record.
type Design struct {
	Name        [16]byte
	Size        uint8 // hull 0..5, frigate to doom star
	Type        uint8 // 0 combat, 1 colony ship, 2 transport, 4 outpost
	Shield      uint8
	Drive       uint8
	FTLSpeed    uint8
	Computer    uint8
	Armor       uint8
	Specials    [5]byte // bit set, bit n in byte n/8
	Weapons     [8]WeaponSlot
	Picture     uint8
	Builder     uint8
	Cost        int16 // Design_Cost_ reads only the low byte
	CombatSpeed uint8
	BuildDate   uint16 // low 16 bits of the stardate
}

// WeaponSlot is one of a design's eight weapon mounts (8 bytes).
type WeaponSlot struct {
	Weapon  int16 // weapon table row; 0 (or -1) empty
	Count   uint8
	Working uint8
	Arc     uint8
	Mods    uint16
	Ammo    uint8
}

// Ship status values.
const (
	ShipAtStar       = 0
	ShipInTransit    = 1
	ShipDeparting    = 2
	ShipRefitting    = 4
	ShipFree         = 5
	ShipBuilding     = 6
	ShipScrapped     = 7
	ShipDestroyed    = 8
	ShipRetreating   = 9
	ShipAtAntares    = 10
	LocationTransit  = 500  // location = 500 + destination star
	LocationDeparted = 1000 // location = 1000 + destination star
)

// Ship is one record of the ship array (129 bytes).
type Ship struct {
	Design          Design
	Owner           int8 // 0..7 players, 8 Antarans, 9..14 monsters
	Status          uint8
	Location        int16
	X, Y            int16
	Navigator       uint8
	Speed           int8
	ETA             uint8
	ShieldDamage    uint8
	DriveDamage     uint8
	ComputerDamage  uint8
	CrewLevel       uint8
	Experience      int16
	Officer         int16 // leader index, -1 none
	DamagedSpecials [5]byte
	ArmorDamage     int16
	StructureDamage int16
	AIMission       uint8
	JustBuilt       uint8
}

// Leader is one officer or colony leader (59 bytes), also the HERODATA.LBX
// record.
type Leader struct {
	Name          [15]byte
	Title         [20]byte
	Type          uint8 // 0 ship officer, 1 colony leader
	Experience    int16
	CommonSkills  uint32
	SpecialSkills uint32
	Techs         [3]uint8
	Picture       uint8
	SkillValue    int16
	RankName      uint8
	Location      int16
	ETA           int8
	LevelUp       uint8
	Status        int8 // -2 dead, -1 for hire, 0 pool, 1 assigned, 2 dismissed, 4 offered, 5 marooned
	Owner         int8
}

// MoveableBox is one draggable main-screen window (28 bytes).
type MoveableBox struct {
	ID, Content    int16
	Height, Width  int16
	X, Y           int16
	Dragging       int16
	DragField      int16
	Field2, Field3 int16
	ClosedWidth    int16
	ClosedHeight   int16
	DragDX, DragDY int16
}
