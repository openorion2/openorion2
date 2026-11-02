package save

import (
	"encoding/binary"
	"reflect"
	"testing"
)

// packedOffset is the offset of a field in the packed (file) layout, which
// differs from Go's padded one.
func packedOffset(t reflect.Type, field string) int {
	off := 0
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Name == field {
			return off
		}
		off += binary.Size(reflect.Zero(f.Type).Interface())
	}
	return -1
}

func TestPlayerLayout(t *testing.T) {
	if n := binary.Size(Player{}); n != 0xea9 {
		t.Fatalf("Player is %d bytes, want 3753", n)
	}
	tp := reflect.TypeOf(Player{})
	for field, want := range map[string]int{
		"RaceName": 0x15, "BC": 0x32, "Settlers": 0x42, "Population": 0xa6, "FieldStatus": 0xc4,
		"AppStatus": 0x117, "ResearchPoints": 0x1eb, "HyperLevels": 0x21c, "Stardates": 0x229,
		"ResearchField": 0x321, "FuelRange": 0x324, "Designs": 0x326, "HullCounters": 0x578,
		"Contact": 0x584, "TradeValue": 0x5a4, "FleetStrength": 0x5ec, "WarDeclared": 0x60f,
		"Relations": 0x617, "TreatyStatus": 0x627, "Modifiers": 0x68f, "TurnsAtWar": 0x717,
		"TreatyTimer": 0x72f, "TechExchangeValue": 0x78f, "AIWarTarget": 0x7c7,
		"LastProposalTurn": 0x817, "SneakStrength": 0x857, "DelayedOrder": 0x897,
		"Picks": 0x89f, "BasePicks": 0x8be, "Evolved": 0x8dd, "History": 0x8df,
		"Spies": 0xe57, "Cheated": 0xe69, "CouncilVote": 0xe71, "TributeReceived": 0xe78,
	} {
		if got := packedOffset(tp, field); got != want {
			t.Errorf("%s at 0x%x, want 0x%x", field, got, want)
		}
	}
}
