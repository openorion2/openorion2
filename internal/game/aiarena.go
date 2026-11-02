package game

import "encoding/binary"

// aiArena mirrors the part of the block arena (0x192ed4) that holds the AI
// data, from the first AI block's header on, in the original's layout: the
// AI reads bytes of it that it never wrote (docs/turn/ai-data.md).
type aiArena struct {
	mem []byte

	// AIDATA.C statics outside the arena, rewritten by Compute_AI_Data_
	// (docs/turn/ai-data.md): 0x1a8b34, 0x1a74b4 and 0x1aa218.
	basePlanetValues       [8][360]uint16
	contextualPlanetValues [8][360]uint16
	nAllShips              int16
}

// aiBlockHeader is the size of the header Allocate_Next_Block_ (0x110d3c)
// puts before each block: {0, requested size, 0}.
const aiBlockHeader = 12

// alloc is Allocate_Next_Block_: n bytes, rounded up to a multiple of 4,
// after a header. It returns the offset of the block's data. The original
// does not clear the memory; a fresh arena here starts zeroed.
func (a *aiArena) alloc(n int) int {
	r := n
	if r&3 != 0 {
		r = r&^3 + 4
	}
	at := len(a.mem)
	a.mem = append(a.mem, make([]byte, aiBlockHeader+r)...)
	binary.LittleEndian.PutUint32(a.mem[at+4:], uint32(n))
	return at + aiBlockHeader
}

// blocks walks the headers and returns each block's data offset and size.
func (a *aiArena) blocks() (offs, sizes []int) {
	for at := 0; at+aiBlockHeader <= len(a.mem); {
		n := int(binary.LittleEndian.Uint32(a.mem[at+4:]))
		r := n
		if r&3 != 0 {
			r = r&^3 + 4
		}
		offs, sizes = append(offs, at+aiBlockHeader), append(sizes, n)
		at += aiBlockHeader + r
	}
	return offs, sizes
}
