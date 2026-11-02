// Package playview is the player's view and a turn's orders that the
// remote control's /act serves (docs/control.md, "Playing by numbers"):
// what a human in PLAYER_NUM's place sees, by record number, and the
// orders he could give on the main screen. Port-only.
package playview

// View is what /act {"view": {}} answers.
type View struct {
	Setup    Setup    `json:"setup"`
	Me       Me       `json:"me"`
	Designs  []Design `json:"designs"`
	Players  []Player `json:"players"`
	Stars    []Star   `json:"stars"`
	Planets  []Planet `json:"planets"`
	Colonies []Colony `json:"colonies"`
	Ships    []Ship   `json:"ships"`
}

// Setup is the game's settings.
type Setup struct {
	Difficulty int `json:"difficulty"`
	GalaxySize int `json:"galaxy_size"`
	GalaxyAge  int `json:"galaxy_age"`
	Players    int `json:"players"`
	TechLevel  int `json:"tech_level"`
}

// Me is the player's empire.
type Me struct {
	Player          int   `json:"player"`
	Stardate        int   `json:"stardate"` // 35000 at the start
	Race            int   `json:"race"`
	BC              int   `json:"bc"`
	Tax             int   `json:"tax"`
	ResearchPoints  int   `json:"research_points"`
	ResearchField   int   `json:"research_field"`
	ResearchApp     int   `json:"research_app"`
	Food            int   `json:"food"`
	Income          int   `json:"income"`
	Industry        int   `json:"industry"`
	Research        int   `json:"research"`
	Population      int   `json:"population"`
	CommandTotal    int   `json:"command_total"`
	CommandUsed     int   `json:"command_used"`
	FreightersTotal int   `json:"freighters_total"`
	FreightersFree  int   `json:"freighters_free"`
	FTLSpeed        int   `json:"ftl_speed"`
	FuelRange       int   `json:"fuel_range"`
	HomePlanet      int   `json:"home_planet"`
	FieldStatus     []int `json:"field_status"`
	// The applications that may be researched now, and whether a choice is
	// due (the research choice comes up on the next turn).
	ResearchOptions []int `json:"research_options"`
	ResearchDue     bool  `json:"research_due"`
}

// Design is a ship design, in short.
type Design struct {
	Size        int `json:"size"`
	Type        int `json:"type"`
	Shield      int `json:"shield"`
	Drive       int `json:"drive"`
	Computer    int `json:"computer"`
	Armor       int `json:"armor"`
	Weapons     int `json:"weapons"` // weapon count
	CombatSpeed int `json:"combat_speed"`
	Cost        int `json:"cost"`
}

// Player is an empire; its race and the relations only once met.
type Player struct {
	Eliminated bool `json:"eliminated"`
	Contact    int  `json:"contact"` // his contact with it, 0 never met
	Race       int  `json:"race"`    // -1 unknown
	Relations  int  `json:"relations"`
	Treaty     int  `json:"treaty"`
}

type Star struct {
	X        int  `json:"x"`
	Y        int  `json:"y"`
	Size     int  `json:"size"`
	Class    int  `json:"class"`
	Special  int  `json:"special"`
	Wormhole int  `json:"wormhole"`
	Nebula   int  `json:"nebula"`
	Visited  bool `json:"visited"`
	Mine     bool `json:"mine"`   // a colony of his
	Others   bool `json:"others"` // another empire's colony (once visited)
	Blockade bool `json:"blockaded"`
	// Ships at or bound for it: his, the others' he detects, monsters'.
	OwnShips     int `json:"own_ships"`
	OwnWeapons   int `json:"own_weapons"`
	OtherShips   int `json:"other_ships"`
	OtherWeapons int `json:"other_weapons"`
	Monsters     int `json:"monsters"`
}

// Planet: the planet's nature always; the rest once its star was
// visited (Known), the population of his own colonies only.
type Planet struct {
	Star     int  `json:"star"`
	Orbit    int  `json:"orbit"`
	Type     int  `json:"type"`
	Size     int  `json:"size"`
	Gravity  int  `json:"gravity"`
	Minerals int  `json:"minerals"`
	Special  int  `json:"special"`
	Known    bool `json:"known"`
	Owner    int  `json:"owner"` // -1 none, 0..7 an empire, 8.. monsters
	Pop      int  `json:"pop"`
	Outpost  bool `json:"outpost"`
	Climate  int  `json:"climate"`
	MaxFarms int  `json:"max_farms"`
}

type Colony struct {
	ID         int    `json:"id"`
	Planet     int    `json:"planet"`
	Pop        int    `json:"pop"`
	Farmers    int    `json:"farmers"`
	Workers    int    `json:"workers"`
	Scientists int    `json:"scientists"`
	Output     [4]int `json:"output"` // food, industry, research, BC
	Morale     int    `json:"morale"`
	Pollution  int    `json:"pollution"`
	Outpost    bool   `json:"outpost"`
	Production int    `json:"production"`
	Marines    int    `json:"marines"`
	Armor      int    `json:"armor"`
	Queue      []int  `json:"queue"`     // queue codes, -1 empty
	Buildings  []int  `json:"buildings"` // the buildings it has
	CanBuild   []int  `json:"can_build"` // the queue codes it may put at the head
}

type Ship struct {
	ID         int    `json:"id"`
	Status     int    `json:"status"`
	X          int    `json:"x"`
	Y          int    `json:"y"`
	At         int    `json:"at"`   // the star it is at, -1
	Dest       int    `json:"dest"` // the star it is bound for, -1
	CanOrder   bool   `json:"can_order"`
	Experience int    `json:"experience"`
	Design     Design `json:"design"`
	ETA        int    `json:"eta"`
}

// Queue codes: a building (1..0x30) or a special item (-2..-15) as the
// queue holds it; a new ship as -1000 - (design type * 8 + hull size).
var QueueCodes = func() []int {
	var c []int
	for k := 1; k <= 48; k++ {
		c = append(c, k)
	}
	for k := 50; k <= 63; k++ {
		c = append(c, 48-k)
	}
	for k := 0; k <= 5; k++ {
		c = append(c, -1000-k)
	}
	return append(c, -1008, -1016, -1032)
}()

// Orders is a turn's orders for empire P (/act "orders"). Maps are
// keyed by colony or ship record.
type Orders struct {
	// Research: the first of these applications that may be researched,
	// when a choice is due.
	Research []int `json:"research"`
	// Queue: the first of these codes the colony may build goes to the
	// head of its queue (none if the head is already one of them).
	Queue map[string][]int `json:"queue"`
	// Jobs: farmers and workers (the rest scientists), moved one at a time
	// as the colonies screen does; Feed then takes farmers back where the
	// empire would starve.
	Jobs map[string][2]int `json:"jobs"`
	Feed bool              `json:"feed"`
	// Dest: a ship at a star goes to that star (-1 stays); one in flight
	// turns there when it can be ordered.
	Dest map[string]int `json:"dest"`
}
