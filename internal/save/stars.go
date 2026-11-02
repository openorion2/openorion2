package save

// Star and nebula records. Spec: docs/formats/save-stars.md. "Mask" fields
// hold one bit per player, bit p for player p.

// SpectralBlackHole is the spectral class of a black hole.
const SpectralBlackHole = 6

// Star is one record of the star array (113 bytes).
type Star struct {
	Name              [15]byte
	X, Y              int16
	Size              uint8   // 0..2
	Owner             int8    // displayed owner for the viewing player, -1 none
	Picture           uint8   // 0..2
	SpectralClass     uint8   // 0..5, 6 black hole
	LastOrbit         [8]int8 // per player, -1 none
	BlackHoleBlocked  [9]byte // bit j: the line to star j crosses a black hole
	Special           uint8
	WormholeTo        int8     // -1 none
	Blockaded         uint8    // mask
	BlockadedBy       [8]uint8 // per player, mask of blockaders
	Visited           uint8    // mask
	SpecialPopup      uint8    // mask
	ColonizePromptOff uint8    // mask
	IgnoreCombat      uint8    // mask
	ColonizePlayer    int8     // turn-local: 8 none yet, p sole claimant, -1 contested
	ColonyOwners      uint8    // mask
	WarpInterdictor   uint8    // mask
	Unknown3A         uint8
	BaseTachyon       uint8    // mask
	BaseSubspace      uint8    // mask
	Stargate          uint8    // mask
	Jumpgate          uint8    // mask
	ArtemisNet        uint8    // mask
	DimensionalPortal uint8    // mask
	AIStagePoint      uint8    // mask
	Officer           [8]int8  // per player, governing leader, -1 none
	Planets           [5]int16 // per orbit, -1 empty
	Relocation        [8]int16 // per player, target star, -1 none
	TwinkleFrame      int8
	TwinkleEnd        int8
	Unknown66         uint8
	SurrenderTo       [8]int8 // per player, -1 none
	InNebula          uint8
	ArtifactsHome     uint8
}

// Nebula is one of the four 5-byte entries of the nebula block.
type Nebula struct {
	X, Y    int16
	Picture uint8
}
