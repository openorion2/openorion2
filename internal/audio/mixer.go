// Package audio mixes the game's sounds for the host: sample voices (the
// digital effects), the music stream and video sound, at Rate Hz stereo
// signed 16-bit. It holds no game state; the engine drives it and the host
// pulls samples from it (headless runs never pull, so they stay silent).
package audio

import (
	"sync"

	"github.com/openorion2/openorion2/internal/sound"
)

// Rate is the output sample rate.
const Rate = 44100

// MaxVolume is AIL's full volume.
const MaxVolume = 127

// Voice is one playing sample.
type Voice struct {
	wave   *sound.Wave
	pos    uint64 // 32.32 fixed point frame position
	step   uint64
	Volume int  // 0..127
	Pan    int  // 0 left .. 64 centre .. 127 right
	Loops  int  // further passes: -1 for ever
	stream bool // fed by Append: waits for more data at its end
	done   bool
}

// Mixer is a set of voices, safe for one producer and one consumer.
type Mixer struct {
	mu     sync.Mutex
	voices map[int]*Voice
	next   int
}

// NewMixer returns an empty mixer.
func NewMixer() *Mixer { return &Mixer{voices: map[int]*Voice{}, next: 1} }

// Start plays w and returns its id.
func (m *Mixer) Start(w *sound.Wave, volume, pan, loops int) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := m.next
	m.next++
	m.voices[id] = &Voice{wave: w, step: uint64(w.Rate) << 32 / Rate, Volume: volume, Pan: pan, Loops: loops}
	return id
}

// StartStream starts a voice of w's format whose data comes in later through
// Append (a video's sound track); it waits silently when it runs dry.
func (m *Mixer) StartStream(w *sound.Wave, volume, pan int) int {
	id := m.Start(w, volume, pan, 0)
	m.mu.Lock()
	m.voices[id].stream = true
	m.mu.Unlock()
	return id
}

// Append adds data to a stream voice; false if it has ended.
func (m *Mixer) Append(id int, data []byte) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.voices[id]
	if !ok || v.done {
		return false
	}
	v.wave.Data = append(v.wave.Data, data...)
	return true
}

// Set changes a voice: f runs under the lock; false if it has ended.
func (m *Mixer) Set(id int, f func(v *Voice)) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.voices[id]
	if !ok || v.done {
		return false
	}
	f(v)
	return true
}

// Bend changes the voice's playback rate by d frames a second (AIL's
// sample playback rate), not below 0.
func (v *Voice) Bend(d int) {
	r := int(v.step*Rate>>32) + d
	v.step = uint64(max(r, 0)) << 32 / Rate
}

// Stop ends a voice.
func (m *Mixer) Stop(id int) {
	m.mu.Lock()
	delete(m.voices, id)
	m.mu.Unlock()
}

// Playing reports whether the voice still sounds.
func (m *Mixer) Playing(id int) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.voices[id]
	return ok && !v.done
}

// Mix adds every voice into out (interleaved stereo) and drops voices
// that ended.
func (m *Mixer) Mix(out []int16) {
	acc := make([]int32, len(out))
	m.mu.Lock()
	for id, v := range m.voices {
		n := v.wave.Frames()
		if n == 0 && !v.stream {
			v.done = true
		}
		for i := 0; i+1 < len(acc) && !v.done; i += 2 {
			f := int(v.pos >> 32)
			if f >= n {
				if v.stream {
					break
				}
				if v.Loops == 0 {
					v.done = true
					break
				}
				if v.Loops > 0 {
					v.Loops--
				}
				v.pos -= uint64(n) << 32
				f = int(v.pos >> 32)
			}
			l, r := int32(v.wave.Sample(f, 0)), int32(v.wave.Sample(f, 1))
			vol := int32(v.Volume)
			acc[i] += l * vol * int32(127-v.Pan) / (127 * 64)
			acc[i+1] += r * vol * int32(v.Pan) / (127 * 64)
			v.pos += v.step
		}
		if v.done {
			delete(m.voices, id)
		}
	}
	m.mu.Unlock()
	for i, a := range acc {
		out[i] = int16(max(-32768, min(32767, a)))
	}
}
