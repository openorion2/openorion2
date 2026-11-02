// Package save reads and writes SAVEn.GAM, the saved game: the game's global
// state written block by block in the order Save_Game_ (MOX\OTHERS\LOADSAVE.C)
// uses. This file holds the container; the records inside the blocks are
// decoded by the other files of the package.
//
// Spec: docs/formats/save.md.
package save

import (
	"encoding/binary"
	"fmt"
	"os"
)

// Version is the format number the file starts with.
const Version = 0xe0

// Size is the length of every save the game writes.
const Size = 208000

// Block names, in file order.
const (
	BVersion           = "version"
	BDescription       = "save_game_description"
	BStardate          = "stardate"
	BGameType          = "game_type"
	BSettings          = "settings"
	BRandomSeed        = "random_seed"
	BNumColonies       = "NUM_COLONIES"
	BColonies          = "colony"
	BPlanetCount       = "planet_count"
	BPlanets           = "planet"
	BNumStars          = "NUM_STARS"
	BStars             = "star"
	BLeaders           = "leaders"
	BNumPlayers        = "NUM_PLAYERS"
	BPlayers           = "player"
	BNumShips          = "NUM_SHIPS"
	BShips             = "ship"
	BMoveableBox       = "moveable_box"
	BMoveableBoxOrder  = "moveable_box_order"
	BPlayerNum         = "PLAYER_NUM"
	BMaxMapScale       = "max_map_scale"
	BMaxZoomCount      = "max_zoom_count"
	BStarBgIndex       = "star_bg_index"
	BMapMaxX           = "MAP_MAX_X"
	BMapMaxY           = "MAP_MAX_Y"
	BFleetIconButtons  = "fleet_icon_button_count"
	BNebula            = "nebula"
	BNumNebulas        = "NUM_NEBULAS"
	BBillSavegame      = "bill_savegame"
	BNReports          = "n_reports"
	BReports           = "reports"
	BDonePlayerReports = "done_player_reports"
	BSomethingHappened = "something_interesting_happened"
	BDoneColonizeCheck = "done_colonize_check"
	BEventData         = "event_data"
	BMsgs              = "msgs"
	BFirstMsg          = "first_msg"
	BMaxMsgs           = "max_msgs"
	BNMsgs             = "n_msgs"
	BDisplayedMsgs     = "displayed_msgs"
	BDisplayedGNN      = "displayed_gnn"
	BCouncilFlag       = "COUNCIL_FLAG"
	BNextCouncilTurn   = "next_council_meeting_turn"
	BAntaranResources  = "antaran_resource_level"
	BAntaranInvasion   = "chance_for_antaran_invasion"
	BDisallowSaves     = "disallow_saves_code"
	BLastEventYear     = "last_event_year"
	BEventDelayCount   = "event_delay_count"
	BAntarans          = "antarans"
	BCurMapScale       = "cur_map_scale"
	BCurMapX           = "cur_map_x"
	BCurMapY           = "cur_map_y"
	BPadding           = "padding"
)

// Block is one fwrite of Save_Game_: count records of size bytes.
type Block struct {
	Name        string
	Size, Count int
}

// Layout lists the blocks in file order, exactly as Save_Game_ writes them.
var Layout = []Block{
	{BVersion, 4, 1},
	{BDescription, 0x25, 1},
	{BStardate, 4, 1},
	{BGameType, 1, 1},
	{BSettings, 0x229, 1},
	{BRandomSeed, 4, 1},
	{BNumColonies, 2, 1},
	{BColonies, 0x169, 250},
	{BPlanetCount, 2, 1},
	{BPlanets, 0x11, 360},
	{BNumStars, 2, 1},
	{BStars, 0x71, 72},
	{BLeaders, 0x3b, 67},
	{BNumPlayers, 2, 1},
	{BPlayers, 0xea9, 8},
	{BNumShips, 2, 1},
	{BShips, 0x81, 500},
	{BMoveableBox, 0x1c, 5},
	{BMoveableBoxOrder, 2, 5},
	{BPlayerNum, 2, 1},
	{BMaxMapScale, 2, 1},
	{BMaxZoomCount, 2, 1},
	{BStarBgIndex, 1, 1},
	{BMapMaxX, 2, 1},
	{BMapMaxY, 2, 1},
	{BFleetIconButtons, 2, 1},
	{BNebula, 0x14, 1},
	{BNumNebulas, 1, 1},
	{BBillSavegame, 0xc, 1},
	{BNReports, 2, 1},
	{BReports, 0x3b6, 1},
	{BDonePlayerReports, 1, 1},
	{BSomethingHappened, 8, 1},
	{BDoneColonizeCheck, 9, 1},
	{BEventData, 9, 0x24},
	{BMsgs, 0x12, 100},
	{BFirstMsg, 1, 8},
	{BMaxMsgs, 1, 8},
	{BNMsgs, 1, 8},
	{BDisplayedMsgs, 1, 1},
	{BDisplayedGNN, 1, 1},
	{BCouncilFlag, 2, 1},
	{BNextCouncilTurn, 2, 1},
	{BAntaranResources, 2, 1},
	{BAntaranInvasion, 2, 1},
	{BDisallowSaves, 2, 1},
	{BLastEventYear, 4, 1},
	{BEventDelayCount, 2, 1},
	{BAntarans, 0x42, 1},
	{BCurMapScale, 2, 1},
	{BCurMapX, 2, 1},
	{BCurMapY, 2, 1},
	{BPadding, 1000, 1}, // zeros
}

// File is a save held as its blocks. The bytes are kept as written so that
// writing the file back reproduces it exactly.
type File struct {
	data   []byte
	blocks map[string][]byte
}

// Open reads and parses a save.
func Open(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	f, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return f, nil
}

// Parse splits a save into its blocks.
func Parse(data []byte) (*File, error) {
	if len(data) != Size {
		return nil, fmt.Errorf("save is %d bytes, want %d", len(data), Size)
	}
	if v := binary.LittleEndian.Uint32(data); v != Version {
		return nil, fmt.Errorf("save format 0x%x, want 0x%x", v, Version)
	}
	f := &File{data: append([]byte(nil), data...), blocks: map[string][]byte{}}
	off := 0
	for _, b := range Layout {
		n := b.Size * b.Count
		f.blocks[b.Name] = f.data[off : off+n]
		off += n
	}
	return f, nil
}

// Bytes returns the save as it would be written.
func (f *File) Bytes() []byte { return append([]byte(nil), f.data...) }

// Block returns a whole block. The slice aliases the file.
func (f *File) Block(name string) []byte { return f.blocks[name] }

// Record returns record i of a block.
func (f *File) Record(name string, i int) []byte {
	for _, b := range Layout {
		if b.Name == name {
			return f.blocks[name][i*b.Size : (i+1)*b.Size]
		}
	}
	return nil
}

// U16 and U32 read a one-record numeric block.
func (f *File) U16(name string) int    { return int(int16(binary.LittleEndian.Uint16(f.blocks[name]))) }
func (f *File) U32(name string) uint32 { return binary.LittleEndian.Uint32(f.blocks[name]) }

// Description is the name the player gave the save.
func (f *File) Description() string {
	b := f.blocks[BDescription]
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}
