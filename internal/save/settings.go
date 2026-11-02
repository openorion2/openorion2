package save

import "encoding/binary"

// Settings and the header blocks. Spec: docs/formats/save-misc.md.

// Game types, the game_type block.
const (
	GameSingle  = 0
	GameHotseat = 1
	GameNetwork = 2
	GameModem   = 3
)

// Settings is the settings block, byte for byte the MOX.SET file. Load_Game_
// takes only the universe settings from a save; the rest is the player's
// MOX.SET.
type Settings [0x229]byte

// Settings returns a copy of the settings block.
func (f *File) Settings() Settings {
	var s Settings
	copy(s[:], f.Block(BSettings))
	return s
}

// RandomEvents is option +0x02.
func (s *Settings) RandomEvents() bool { return s[0x02] != 0 }

// Language: 0 English, 1 German, 2 French, 3 Spanish, 4 Italian.
func (s *Settings) Language() int { return int(s[0xd2]) }

// AntaransAttack is universe setting +0xd3.
func (s *Settings) AntaransAttack() bool { return s[0xd3] != 0 }

// Difficulty: 0 Tutor, 1 Easy, 2 Average, 3 Hard, 4 Impossible.
func (s *Settings) Difficulty() int { return int(s[0xd4]) }

// Players is the number of players including the human, 2..8.
func (s *Settings) Players() int { return int(s[0xd5]) }

// GalaxySize: 0 small (20 stars), 1 medium (36), 2 large (54), 3 huge (72).
func (s *Settings) GalaxySize() int { return int(s[0xd6]) }

// GalaxyAge: 0 mineral rich, 1 average, 2 organic rich.
func (s *Settings) GalaxyAge() int { return int(s[0xd7]) }

// StrategicCombat is byte 0xd8: 1 for strategic, 0 for tactical combat
// (Newgame_Screen_ passes tactical = (0xd8 == 0); docs/formats/save-misc.md).
func (s *Settings) StrategicCombat() bool { return s[0xd8] != 0 }

// TechLevel: 0 pre-warp, 1 average, 2 advanced.
func (s *Settings) TechLevel() int { return int(s[0xd9]) }

// UniverseSeed is the RNG state just before the galaxy was generated.
func (s *Settings) UniverseSeed() uint32 { return binary.LittleEndian.Uint32(s[0xda:]) }

// Version is the MOX.SET version, 0x82.
func (s *Settings) Version() int { return int(binary.LittleEndian.Uint16(s[0xe2:])) }

// ApplyUniverse copies the settings Load_Game_ takes from a save into s:
// random events, +0xd3..+0xd9 and the universe seed.
func (s *Settings) ApplyUniverse(from *Settings) {
	s[0x02] = from[0x02]
	copy(s[0xd3:0xde], from[0xd3:0xde])
}

// GameType is the game_type block.
func (f *File) GameType() int { return int(f.Block(BGameType)[0]) }

// RandomSeed is the RNG seed at the moment of saving.
func (f *File) RandomSeed() uint32 { return f.U32(BRandomSeed) }
