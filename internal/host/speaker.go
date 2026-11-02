//go:build !js

package host

import (
	"encoding/binary"

	"github.com/Zyko0/go-sdl3/sdl"

	"github.com/openorion2/openorion2/internal/audio"
)

// speaker is the SDL audio output: an audio stream on the default playback
// device, topped up from the mixer once a frame. Without audio (no device,
// SDL_INIT_AUDIO failing) it stays nil and the game plays unheard.
type speaker struct {
	stream *sdl.AudioStream
	buf    []int16
	bytes  []byte
}

// lead is how much sound is kept queued: 1/10 s.
const lead = audio.Rate / 10

func openSpeaker() *speaker {
	if err := sdl.InitSubSystem(sdl.INIT_AUDIO); err != nil {
		return nil
	}
	spec := &sdl.AudioSpec{Format: sdl.AUDIO_S16LE, Channels: 2, Freq: audio.Rate}
	st := sdl.AUDIO_DEVICE_DEFAULT_PLAYBACK.OpenAudioDeviceStream(spec, 0)
	if st == nil {
		return nil
	}
	if err := st.ResumeDevice(); err != nil {
		st.Destroy()
		return nil
	}
	return &speaker{stream: st}
}

// feed queues mixed sound until lead frames are waiting.
func (s *speaker) feed(m *audio.Mixer) {
	if s == nil {
		return
	}
	q, err := s.stream.Queued()
	if err != nil {
		return
	}
	n := lead - int(q)/4
	if n <= 0 {
		return
	}
	if cap(s.buf) < 2*n {
		s.buf = make([]int16, 2*n)
		s.bytes = make([]byte, 4*n)
	}
	buf, b := s.buf[:2*n], s.bytes[:4*n]
	m.Mix(buf)
	for i, v := range buf {
		binary.LittleEndian.PutUint16(b[2*i:], uint16(v))
	}
	s.stream.PutData(b)
}

func (s *speaker) close() {
	if s != nil {
		s.stream.Destroy()
	}
}
