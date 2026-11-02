// Package sound decodes the game's sound data: the RIFF WAVE entries of
// SOUND.LBX, STREAM.LBX and STREAMHD.LBX (8- or 16-bit PCM). Spec:
// docs/formats/sound.md.
package sound

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// Wave is a PCM sample.
type Wave struct {
	Channels int
	Rate     int
	Bits     int    // 8 (unsigned) or 16 (signed little-endian)
	Data     []byte // the data chunk
}

// ParseWAV reads a RIFF WAVE file with a PCM fmt chunk.
func ParseWAV(b []byte) (*Wave, error) {
	if len(b) < 12 || string(b[:4]) != "RIFF" || string(b[8:12]) != "WAVE" {
		return nil, errors.New("sound: not a RIFF WAVE file")
	}
	le := binary.LittleEndian
	w := &Wave{}
	fmtSeen := false
	for p := 12; p+8 <= len(b); {
		id, n := string(b[p:p+4]), int(le.Uint32(b[p+4:]))
		body := b[p+8:]
		if n > len(body) {
			// A data chunk may claim more than the entry holds.
			n = len(body)
		}
		body = body[:n]
		switch id {
		case "fmt ":
			if n < 16 {
				return nil, errors.New("sound: short fmt chunk")
			}
			if tag := le.Uint16(body); tag != 1 {
				return nil, fmt.Errorf("sound: format tag %d, want PCM", tag)
			}
			w.Channels = int(le.Uint16(body[2:]))
			w.Rate = int(le.Uint32(body[4:]))
			w.Bits = int(le.Uint16(body[14:]))
			fmtSeen = true
		case "data":
			w.Data = body
		}
		p += 8 + n + n&1
	}
	if !fmtSeen || w.Data == nil {
		return nil, errors.New("sound: no fmt or data chunk")
	}
	if w.Bits != 8 && w.Bits != 16 {
		return nil, fmt.Errorf("sound: %d-bit samples", w.Bits)
	}
	return w, nil
}

// Frames is the number of sample frames.
func (w *Wave) Frames() int { return len(w.Data) / (w.Channels * w.Bits / 8) }

// Sample returns frame i of channel c as a signed 16-bit value.
func (w *Wave) Sample(i, c int) int16 {
	if c >= w.Channels {
		c = w.Channels - 1
	}
	if w.Bits == 8 {
		return int16(int(w.Data[i*w.Channels+c])-128) << 8
	}
	o := (i*w.Channels + c) * 2
	return int16(binary.LittleEndian.Uint16(w.Data[o:]))
}
