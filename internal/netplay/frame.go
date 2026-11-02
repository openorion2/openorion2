// Package netplay is the transport under the game's Net_* calls: a TCP
// relay (star topology, the creator relays), its WebSocket side for the
// browser build and an in-memory hub with the same API for tests. The whole package is port-only; the original's IPX
// and modem layers are not ported. Spec: docs/net/transport.md.
package netplay

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const (
	// Broadcast is the original's destination 0x7ffe: every human but the
	// sender.
	Broadcast = 0x7ffe
	// DefaultPort is the relay's TCP port when none is given.
	DefaultPort = 52525
	// Version is the protocol version the handshake compares; it replaces
	// the original's build-date check at the transport level.
	Version = 1
	// MaxPayload is the largest payload: the original's lengths are u16.
	MaxPayload = 0xffff
	// MaxUsers is the most stations a game takes (eight empires).
	MaxUsers = 8
	// RelayID is the destination of control frames; stations count from 1.
	RelayID = 0
)

// Control frames (to RelayID, or from it) carry the session handshake and
// membership; their ids are above every game message id (< 0x900).
const (
	ctlHello   = 0xff01 // station → relay: version, kind, player name, game name
	ctlWelcome = 0xff02 // relay → station: version, net id, open, game name, users
	ctlRefuse  = 0xff03 // relay → station: the reason, then the link closes
	ctlUsers   = 0xff04 // relay → stations: the members after a change
	ctlLost    = 0xff05 // relay → stations: a member that left
	ctlPing    = 0xff06 // either way, empty: the link is alive (port-only keepalive)
	ctlOpen    = 0xff07 // creator → hosted relay: u8 open (Net_Enable/Disable_New_Players_)
)

// Hello kinds.
const (
	helloJoin  = 0
	helloQuery = 1 // the game list: a Welcome with net id 0, then close
	// helloCreate makes the station the creator (net id 1) of a hosted
	// relay, one without a station of its own (cmd/webserve's, for browsers).
	helloCreate = 2
)

// Frame is one message: the original's (msg id, from, to, payload).
type Frame struct {
	ID, From, To uint16
	Payload      []byte
}

// ErrFrame is returned for a malformed frame on the wire.
var ErrFrame = errors.New("netplay: bad frame")

// WriteFrame writes f as u32 n (little endian, 6 + payload length), then
// u16 id, from, to and the payload.
func WriteFrame(w io.Writer, f Frame) error {
	if len(f.Payload) > MaxPayload {
		return fmt.Errorf("netplay: payload of %d bytes", len(f.Payload))
	}
	b := make([]byte, 10+len(f.Payload))
	binary.LittleEndian.PutUint32(b, uint32(6+len(f.Payload)))
	binary.LittleEndian.PutUint16(b[4:], f.ID)
	binary.LittleEndian.PutUint16(b[6:], f.From)
	binary.LittleEndian.PutUint16(b[8:], f.To)
	copy(b[10:], f.Payload)
	_, err := w.Write(b)
	return err
}

// ReadFrame reads one frame written by WriteFrame.
func ReadFrame(r io.Reader) (Frame, error) {
	var h [10]byte
	if _, err := io.ReadFull(r, h[:4]); err != nil {
		return Frame{}, err
	}
	n := binary.LittleEndian.Uint32(h[:4])
	if n < 6 || n > 6+MaxPayload {
		return Frame{}, ErrFrame
	}
	if _, err := io.ReadFull(r, h[4:]); err != nil {
		return Frame{}, unexpected(err)
	}
	f := Frame{
		ID:   binary.LittleEndian.Uint16(h[4:]),
		From: binary.LittleEndian.Uint16(h[6:]),
		To:   binary.LittleEndian.Uint16(h[8:]),
	}
	f.Payload = make([]byte, n-6)
	if _, err := io.ReadFull(r, f.Payload); err != nil {
		return Frame{}, unexpected(err)
	}
	return f, nil
}

func unexpected(err error) error {
	if err == io.EOF {
		return io.ErrUnexpectedEOF
	}
	return err
}

// putIDs appends a u8 count and the u16 ids.
func putIDs(b []byte, ids []uint16) []byte {
	b = append(b, byte(len(ids)))
	for _, id := range ids {
		b = binary.LittleEndian.AppendUint16(b, id)
	}
	return b
}

// getIDs reads what putIDs wrote.
func getIDs(b []byte) ([]uint16, []byte, error) {
	if len(b) < 1 || len(b) < 1+2*int(b[0]) {
		return nil, nil, ErrFrame
	}
	n := int(b[0])
	ids := make([]uint16, n)
	for i := range ids {
		ids[i] = binary.LittleEndian.Uint16(b[1+2*i:])
	}
	return ids, b[1+2*n:], nil
}

// cstr splits a NUL-terminated string off b.
func cstr(b []byte) (string, []byte, error) {
	for i, c := range b {
		if c == 0 {
			return string(b[:i]), b[i+1:], nil
		}
	}
	return "", nil, ErrFrame
}
