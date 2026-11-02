package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/openorion2/openorion2/internal/audio"
	"github.com/openorion2/openorion2/internal/lbx"
	"github.com/openorion2/openorion2/internal/sound"
)

// The WLIBS sound layer (sound.c): sample handles 1..16 over AIL, the
// sound file the effects come from, and one streamed music file. Volumes
// are AIL's 0..127 inside, percentages at the edges. Spec:
// docs/engine/sound.md. Nothing here touches game state or the RNG.

const nSoundHandles = 16

type soundState struct {
	on        bool // 0x184380: Sound_On_
	streamOn  bool // 0x184453: Stream_On_
	maxVolume int  // 0x1843a7, 0..127
	pan       int  // 0x1843a5
	file      *lbx.Archive
	dir       string
	voice     [nSoundHandles + 1]int // mixer voice per handle
	fade      [nSoundHandles + 1]int // 0x1b06b4: Fadeout_Sound_ steps
	cache     map[string]*sound.Wave

	streamFile   string // Set_Stream_Filename_
	stream       int    // 0x184447: the stream's handle
	streamMax    int    // 0x18444b, percent
	streamUp     bool   // 0x1843ab: fading up
	streamUpStep int    // 0x18444f
	streamLoops  int
}

// InitSound sets the sound file (Set_Sound_File_) from dir and turns the
// layer on, as the start-up's driver installation does when a card is
// found.
func (e *Engine) InitSound(dir, soundFile string) error {
	s := &e.snd
	s.dir = dir
	s.cache = map[string]*sound.Wave{}
	a, err := lbx.Open(findCase(dir, soundFile))
	if err != nil {
		return err
	}
	s.file = a
	s.on, s.streamOn = true, true
	s.maxVolume, s.pan, s.streamMax = audio.MaxVolume, 64, 100
	return nil
}

func findCase(dir, name string) string {
	if ents, err := os.ReadDir(dir); err == nil {
		for _, e := range ents {
			if strings.EqualFold(e.Name(), name) {
				return filepath.Join(dir, e.Name())
			}
		}
	}
	return filepath.Join(dir, name)
}

func (e *Engine) mixer() *audio.Mixer {
	if e.Sys == nil {
		return nil
	}
	return e.Sys.Audio
}

// playing is Is_Sound_Playing_.
func (e *Engine) playing(h int) bool {
	m := e.mixer()
	return h >= 1 && h <= nSoundHandles && e.snd.voice[h] != 0 && m != nil && m.Playing(e.snd.voice[h])
}

// freeHandle is getFreeSampleHandle_: the first handle not playing, else 0.
func (e *Engine) freeHandle() int {
	for h := 1; h <= nSoundHandles; h++ {
		if !e.playing(h) {
			return h
		}
	}
	return 0
}

func (e *Engine) wave(file string, n int) (*sound.Wave, error) {
	s := &e.snd
	key := fmt.Sprintf("%s#%d", strings.ToUpper(file), n)
	if w, ok := s.cache[key]; ok {
		return w, nil
	}
	if file != "" {
		// One stream is kept: a music file is megabytes.
		for k := range s.cache {
			if !strings.HasPrefix(k, "#") {
				delete(s.cache, k)
			}
		}
	}
	var b []byte
	var err error
	if file == "" {
		b, err = s.file.Entry(n)
	} else {
		b, err = lbx.ReadEntry(findCase(s.dir, file), n)
	}
	if err != nil {
		return nil, err
	}
	w, err := sound.ParseWAV(b)
	if err != nil {
		return nil, err
	}
	s.cache[key] = w
	return w, nil
}

// PlaySound is Play_Sound_ (0x112399): entry n of the sound file on a free
// handle at the maximum volume; the handle, or 0.
func (e *Engine) PlaySound(n int) int {
	s := &e.snd
	m := e.mixer()
	if !s.on || n == 0 || s.file == nil || m == nil {
		return 0
	}
	h := e.freeHandle()
	if h == 0 {
		return 0
	}
	w, err := e.wave("", n)
	if err != nil {
		return 0
	}
	s.fade[h] = 0
	s.voice[h] = m.Start(w, s.maxVolume, s.pan, 0)
	return h
}

func (e *Engine) withVoice(h int, f func(v *audio.Voice)) {
	if h >= 1 && h <= nSoundHandles && e.snd.on && e.snd.voice[h] != 0 && e.mixer() != nil {
		e.mixer().Set(e.snd.voice[h], f)
	}
}

// FadeoutSound is Fadeout_Sound_ (0x112c96): Sound_Update_ takes step off
// the handle's volume each call and stops it at 0.
func (e *Engine) FadeoutSound(h, step int) {
	if h >= 1 && h <= nSoundHandles && e.snd.on {
		e.snd.fade[h] = step
	}
}

// PitchBendSound is Pitch_Bend_Sound_ (0x112cd9): the playback rate plus d.
func (e *Engine) PitchBendSound(h, d int) {
	e.withVoice(h, func(v *audio.Voice) { v.Bend(d) })
}

// LoopSound is Loop_Sound_ (0x112c29): n more passes, -1 for ever.
func (e *Engine) LoopSound(h, n int) {
	if n == 0 {
		return
	}
	e.withVoice(h, func(v *audio.Voice) { v.Loops = n })
}

// StopSound is Stop_Sound_.
func (e *Engine) StopSound(h int) {
	if h >= 1 && h <= nSoundHandles && e.snd.voice[h] != 0 && e.mixer() != nil {
		e.mixer().Stop(e.snd.voice[h])
		e.snd.voice[h], e.snd.fade[h] = 0, 0
	}
}

// StopAllSounds is Stop_All_Sounds_ (the stream's handle included).
func (e *Engine) StopAllSounds() {
	for h := 1; h <= nSoundHandles; h++ {
		e.StopSound(h)
	}
}

// SoundOn and SoundOff are Sound_On_ and Sound_Off_.
func (e *Engine) SoundOn() {
	if e.snd.file != nil {
		e.snd.on = true
	}
}

func (e *Engine) SoundOff() {
	e.StopAllSounds()
	e.snd.on = false
}

// SetSoundMaxVolume is Set_Sound_Max_Volume_: percent of AIL's full scale.
func (e *Engine) SetSoundMaxVolume(pct int) {
	if pct >= 0 && pct <= 100 {
		e.snd.maxVolume = pct * audio.MaxVolume / 100
	}
}

// SetStreamFilename is Set_Stream_Filename_.
func (e *Engine) SetStreamFilename(name string) { e.snd.streamFile = name }

// PlayStream is Play_Stream_ (0x113a20): entry n of the stream file on its
// own handle, at the stream volume.
func (e *Engine) PlayStream(n int) {
	s := &e.snd
	m := e.mixer()
	if !s.on || !s.streamOn || m == nil {
		return
	}
	e.StopStream()
	h := e.freeHandle()
	if h == 0 {
		return
	}
	w, err := e.wave(s.streamFile, n)
	if err != nil {
		return
	}
	s.fade[h] = 0
	s.voice[h] = m.Start(w, s.streamMax*audio.MaxVolume/100, 64, 0)
	s.stream = h
}

// StopStream is Stop_Stream_.
func (e *Engine) StopStream() {
	s := &e.snd
	if s.stream != 0 {
		e.StopSound(s.stream)
		s.stream, s.streamUp = 0, false
	}
}

// IsStreamPlaying is Is_Stream_Playing_.
func (e *Engine) IsStreamPlaying() bool { return e.snd.stream != 0 && e.playing(e.snd.stream) }

// LoopStream is Loop_Stream_: n more passes, -1 for ever.
func (e *Engine) LoopStream(n int) { e.LoopSound(e.snd.stream, n) }

// SetStreamVolume is Set_Stream_Volume_ (0x113af2): percent, capped by the
// stream's maximum.
func (e *Engine) SetStreamVolume(pct int) {
	s := &e.snd
	if !s.on {
		return
	}
	pct = min(pct, s.streamMax)
	e.withVoice(s.stream, func(v *audio.Voice) { v.Volume = pct * audio.MaxVolume / 100 })
}

// StreamVolume is Get_Stream_Volume_ (0x113b56): percent, 0 without a
// stream.
func (e *Engine) StreamVolume() int {
	s := &e.snd
	vol := 0
	e.withVoice(s.stream, func(v *audio.Voice) { vol = v.Volume * 100 / audio.MaxVolume })
	return vol
}

// FadeStreamDown is Fade_Stream_Down_ (0x113c28).
func (e *Engine) FadeStreamDown(step int) {
	s := &e.snd
	if s.on && s.stream > 0 {
		s.streamUp = false
		e.FadeoutSound(s.stream, step)
	}
}

// FadeStreamUp is Fade_Stream_Up_ (0x113cbd).
func (e *Engine) FadeStreamUp(step int) {
	s := &e.snd
	if s.stream >= 1 {
		s.fade[s.stream] = 0
	}
	s.streamUpStep, s.streamUp = step, true
}

// StreamOn, StreamOff and SetStreamMaxVolume are Stream_On_, Stream_Off_
// and Set_Stream_Max_Volume_.
func (e *Engine) StreamOn() {
	if e.snd.file != nil {
		e.snd.streamOn = true
	}
}

func (e *Engine) StreamOff() {
	e.StopStream()
	e.snd.streamOn = false
}

func (e *Engine) SetStreamMaxVolume(pct int) {
	if pct >= 0 && pct <= 100 {
		e.snd.streamMax = pct
	}
}

// soundUpdate is Sound_Update_ (0x1131f0), from Mox_Update_: the fades,
// one step a call, and the end of the stream.
func (e *Engine) soundUpdate() {
	s := &e.snd
	if !s.on {
		return
	}
	for h := 1; h <= nSoundHandles; h++ {
		if s.fade[h] == 0 || !e.playing(h) {
			continue
		}
		step, stop := s.fade[h], false
		e.withVoice(h, func(v *audio.Voice) {
			v.Volume = max(v.Volume-step, 0)
			stop = v.Volume == 0
		})
		if stop {
			e.StopSound(h)
			if h == s.stream {
				s.stream = 0
			}
		}
	}
	if s.stream != 0 && s.streamUp {
		v := e.StreamVolume() + s.streamUpStep
		if v >= s.streamMax {
			v, s.streamUp = s.streamMax, false
		}
		e.SetStreamVolume(v)
	}
	if s.stream != 0 && !e.playing(s.stream) {
		s.stream = 0
	}
}

// StartVideoSound opens a sound track of a Smacker video (SMACKSOUNDUSEMSS
// with SmackVolumePan 0x8000, 0x8000: full volume, centred); 0 without a
// mixer. VideoSound appends a frame's samples, StopVideoSound ends it.
func (e *Engine) StartVideoSound(rate, channels, bits int) int {
	m := e.mixer()
	if m == nil {
		return 0
	}
	return m.StartStream(&sound.Wave{Channels: channels, Rate: rate, Bits: bits}, audio.MaxVolume, 64)
}

func (e *Engine) VideoSound(id int, data []byte) {
	if m := e.mixer(); m != nil && id != 0 {
		m.Append(id, data)
	}
}

func (e *Engine) StopVideoSound(id int) {
	if m := e.mixer(); m != nil && id != 0 {
		m.Stop(id)
	}
}
