package save

// Player record (3753 bytes). Spec: docs/formats/save-players.md. Arrays of
// 8 are indexed by the other player; Unknown fields are kept as stored.

// ObjectiveHuman marks a human player in Player.Objective.
const ObjectiveHuman = 100

// Player is one record of the player array.
type Player struct {
	Unknown000          uint8
	EmperorName         [20]byte
	RaceName            [15]byte
	Eliminated          uint8
	Race                uint8 // 0 Alkari .. 12 Trilarian
	Color               uint8 // banner 0..7
	Personality         uint8 // AI personality 0..5
	Objective           uint8 // AI objective 0..5, ObjectiveHuman for humans
	HomePlanet          int16 // planet index, -1 when eliminated
	NetID               int16
	TurnDone            uint8
	LegacyResearch      uint16 // moved to ResearchPoints by Load_Game_
	BreakthroughPending uint8
	TaxRate             uint8
	BC                  int32
	FreightersTotal     int16
	FreightersFree      int16
	CommandTotal        int16
	CommandUsed         int16
	FoodShipped         int16
	SettlersCount       int16
	Settlers            [25]uint32 // packed: source colony, target planet, race, ETA

	Population       int16
	FoodProduced     int16
	IndustryProduced int16
	ResearchProduced int16
	GrossIncome      int16
	FoodSurplus      int16
	NetIncome        int16
	Maintenance      int32
	MaintenanceParts [6]int16 // their sum is Maintenance

	FieldStatus [83]uint8  // 0 unknown, 1 unavailable, 2 researchable, 3 researched
	AppStatus   [212]uint8 // 0 not obtainable, 1 obtainable, 3 known

	ResearchPoints    int32
	CapturedOrion     uint8
	CapturedAntares   uint8
	WonCouncil        uint8
	Eliminations      [8]int16
	CapturedColonists int16
	CombatSlot        uint8
	ShipTheme         uint8
	SpecialTheme      uint8
	Unknown207        [21]byte
	HyperLevels       [8]uint8
	KnownTechCost     int32
	Unknown228        uint8
	Stardates         [8]int32
	Unused249         [216]byte
	ResearchField     uint8
	ResearchApp       uint8
	Unknown323        uint8
	FuelRange         int16

	Designs      [6]Design // slot 5 is the AI transport, cleared for humans
	HullCounters [6]int16

	Contact          [8]uint8
	LostContactMask  uint8
	LostContactCount [8]uint8
	InNormalRange    [8]uint8
	Unknown59D       uint8
	StagingStar      int8
	BestWarpDrive    uint8
	FTLSpeed         uint8
	OfficerForHire   int8
	TradePops        int16
	TradeValue       [8]int16
	TradeGoal        [8]int16
	ResearchPops     int16
	ResearchValue    [8]int16
	ResearchGoal     [8]int16
	Unknown5E6       int16
	FieldsResearched int16
	FleetSizeScore   int16
	FleetStrength    [8]int32
	ColoniesPop5     int16
	Unknown60E       uint8

	// Diplomacy, per other player.
	WarDeclared       [8]uint8
	Relations         [8]int8 // -100..100
	BaseRelations     [8]int8
	TreatyStatus      [8]uint8 // 0 none, 1-2 treaties, 3 timed, 4 war, 5-6 war with a human
	TradeTreaty       [8]uint8
	ResearchTreaty    [8]uint8
	Tribute           [8]int16
	MessageThisTurn   [8]uint8
	MessageFlags      [8]uint8
	MessageParam      [8]int16
	Diplo66F          [8]int16
	Diplo67F          [8]int16
	Modifiers         [4][8]int16
	Diplo6CF          [8]uint8
	Diplo6D7          [8]int8
	Diplo6DF          [8]uint8
	Diplo6E7          [8]uint8
	WarLosses         [8]int16
	WarLosses2        [8]int16
	Diplo70F          [8]uint8
	TurnsAtWar        [8]uint8
	DemandState       [8]uint8
	Diplo727          [8]uint8
	TreatyTimer       [8]uint8
	LastMessageParam  [8]int16
	Diplo747          [8]uint8
	PrevTreatyStatus  [8]uint8
	TurnScratch       [4][8]uint8
	Demand            [8]int16
	AllianceCall      [8]uint8
	TechExchangeValue [8]int32
	TechExchangeApp   [8]uint8
	Diplo7B7          [8]int16
	AIWarTarget       int16
	Diplo7C9          [3]uint8
	Diplo7CC          [8]int16
	Diplo7DC          [8]uint8
	Diplo7E4          [8]uint8
	Diplo7EC          int16
	Diplo7EE          [8]int8
	Diplo7F6          [8]uint8
	Diplo7FE          [8]uint8
	Diplo806          [8]uint8
	Diplo80E          [8]uint8
	Diplo816          uint8
	LastProposalTurn  [8]int16
	HumanBias         [8]int16
	SneakStar         [8]int16
	SneakValue        [8]int16
	SneakStrength     [8]int32
	Unknown877        [16]byte
	Diplo887          [8]uint8
	ThreatTurn        [8]uint8
	DelayedOrder      [8]int8 // -1 none; Load_Game_ resets it

	Picks            [31]int8 // live race picks, RACESTUF.LBX order
	BasePicks        [31]int8 // the set Evolutionary Mutation starts from
	Evolved          uint8
	EvolutionApplied uint8 // Apply_Evolutionary_Upgrades_ copied BasePicks

	History [4][350]uint8 // fleet, technology, population, buildings

	Spies            [8]uint8
	InfoButtons      uint8
	Ignoring         uint8
	UnknownE61       [8]byte
	Cheated          uint8
	OfficerOfferDate uint16
	UnknownE6C       [5]byte
	CouncilVote      int8
	SurrenderTo      int8
	LuckyCounter     int16
	GalacticLore     uint8 // a leader with Galactic Lore serves
	CheatWarning     uint8
	CheatFlag        uint8
	TributeReceived  int32
	UnusedE7C        [45]byte
}

// Race pick indices into Player.Picks.
const (
	PickGovernment = iota
	PickPopulation
	PickFarming
	PickIndustry
	PickScience
	PickMoney
	PickShipDefense
	PickShipAttack
	PickGroundCombat
	PickSpying
	PickLowG
	PickHighG
	PickAquatic
	PickSubterranean
	PickLargeHomeWorld
	PickRichPoorHomeWorld
	PickArtifactsWorld
	PickCybernetic
	PickLithovore
	PickRepulsive
	PickCharismatic
	PickUncreative
	PickCreative
	PickTolerant
	PickFantasticTraders
	PickTelepathic
	PickLucky
	PickOmniscient
	PickStealthyShips
	PickTransDimensional
	PickWarlord
)
