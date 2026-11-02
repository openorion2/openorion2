package audio

import (
	"testing"

	"github.com/openorion2/openorion2/internal/sound"
)

func TestMixer(t *testing.T) {
	t.Parallel()
	w := &sound.Wave{Channels: 1, Rate: 22050, Bits: 8, Data: []byte{255, 255, 0, 0}}
	m := NewMixer()
	id := m.Start(w, MaxVolume, 64, 0)
	out := make([]int16, 32)
	m.Mix(out)
	if out[0] <= 0 || out[1] <= 0 || out[8] >= 0 {
		t.Errorf("mix %v", out[:10])
	}
	if m.Playing(id) {
		t.Error("the voice did not end")
	}
	id = m.Start(w, MaxVolume, 64, -1)
	m.Mix(out)
	if !m.Playing(id) || out[30] == 0 {
		t.Errorf("looping voice: %v %v", m.Playing(id), out)
	}
}

// TestStreamVoice: a stream voice waits silently when dry and plays what
// Append adds later.
func TestStreamVoice(t *testing.T) {
	t.Parallel()
	m := NewMixer()
	id := m.StartStream(&sound.Wave{Channels: 1, Rate: Rate, Bits: 8}, MaxVolume, 64)
	out := make([]int16, 8)
	m.Mix(out)
	if !m.Playing(id) || out[0] != 0 {
		t.Fatalf("dry stream: playing %v, sample %d", m.Playing(id), out[0])
	}
	m.Append(id, []byte{0xff, 0xff, 0xff, 0xff})
	m.Mix(out)
	if out[0] == 0 || out[1] == 0 {
		t.Errorf("appended data not played: %v", out)
	}
}
