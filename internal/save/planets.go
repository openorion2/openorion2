package save

// Planet and colony records. Spec: docs/formats/save-planets.md.

// Planet is one record of the planet array (17 bytes).
type Planet struct {
	Colony           int16 // colony index, -1 none
	Star             uint8
	Orbit            uint8 // 0..4
	Type             uint8 // 1 asteroids, 2 gas giant, 3 planet
	Size             uint8 // 0 tiny .. 4 huge
	Gravity          uint8 // 0 low, 1 normal, 2 heavy
	ClimateGroup     uint8
	Climate          uint8 // 0 toxic .. 9 gaia
	Picture          uint8 // 0..2
	Minerals         uint8 // 0 ultra poor .. 4 ultra rich
	FoodPerFarmer    uint8
	TerraformCount   uint8
	MaxFarms         uint8
	ArtificialMaxPop uint8
	Special          uint8
	Flags            uint8 // bit 1 soil enriched
}

// Commodity indices of the four-element colony arrays.
const (
	Food = iota
	Industry
	Research
	BC
)

// Colony is one record of the colony array (361 bytes).
type Colony struct {
	Owner          int8 // -1 empty slot
	ReservedFor    int8
	Planet         int16
	Unknown004     int16
	IsOutpost      uint8
	Morale         int8
	Pollution      int16
	PopCount       uint8
	Specialty      int8       // -1, 0 agricultural, 1 industrial, 2 research
	Colonists      [42]uint32 // see Colonist
	PopFraction    [10]int16  // per race, 1/1000 colonist
	PopGrowth      [10]int16  // per race, 1/1000 per turn
	TurnsOwned     uint8
	FoodPerFarmer  uint8
	IndustryPer    uint8
	ResearchPer    uint8
	CanFarm        uint8
	Unknown0E1     uint8
	Climate        uint8 // effective climate
	Unknown0E3     [4]byte
	Output         [4]int16 // food, industry, research, BC
	Upkeep         [4]uint8
	FoodImport     int16
	IndustryPaid   int16
	Unknown0F7     int16
	BCDeficit      int16
	Recyclotron    uint8
	FoodDemand     [4]uint8
	IndustryDem    [4]uint8
	FoodByRace     [8]uint8
	IndustryByRace [8]uint8
	ReplicatedFood uint8
	Queue          [7]int16 // QueueEmpty, building ids, ship and special codes
	LastBuilt      int16
	Production     int16
	IndustryTaxed  int16
	AutoBuild      uint8
	Unknown12A     [2]byte
	Pending        int16
	Assimilation   uint8
	Occupation     uint8
	Marines        int16
	Armor          int16
	ArmorProgress  uint8
	MarineProgress uint8
	Buildings      [49]uint8 // 1 where building id i stands
	SoldStardate   uint16    // low word of the stardate a building was last sold
}

// Production queue codes besides building ids 1..48 and ships (<= -100).
const (
	QueueEmpty            = -1
	QueueTradeGoods       = -2
	QueueHousing          = -3
	QueueAndroidFarmer    = -4
	QueueAndroidWorker    = -5
	QueueAndroidScientist = -6
	QueueSpy              = -7
	QueueRepeat           = -10
	QueueFreighters       = -15
	QueueShipBase         = -100 // ship code: -(design slot) - 100
)

// Colonist is one packed colonist dword.
type Colonist uint32

// Race is the player whose race this is, 8 android, 9 native.
func (c Colonist) Race() int { return int(c & 0xf) }

// Loyalty is the player the colonist belongs to now.
func (c Colonist) Loyalty() int { return int(c >> 4 & 7) }

// Job is 0 farmer, 1 worker, 2 scientist.
func (c Colonist) Job() int { return int(c >> 7 & 3) }

// Conquered reports a colonist not yet assimilated.
func (c Colonist) Conquered() bool { return c>>10&1 != 0 }
