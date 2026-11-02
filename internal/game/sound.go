package game

// The game's sound (MOX.C, MAINPUPS.C): Mox_Play_Sound_ and the music
// routines over the engine's sound layer. None of them draws from the RNG
// or changes game state; the music picks its track by clock_(). Spec:
// docs/engine/sound.md.

const (
	streamHDLBX = 0x178bb1 // "STREAMHD.LBX": music n > 100 is entry n-100
	streamLBX   = 0x178bbe // "STREAM.LBX"
)

// music is the music module's state (all -1 at start).
type music struct {
	current int16 // 0x180eb4: the track playing
	state   int16 // 0x180eb6: Play_Streaming_Music_'s state 1..4
	want    int16 // 0x180eb8: the track asked for
	next    int16 // 0x180eba: what follows it (-1 loop it, -2 a random one)
}

// moxPlaySound is Mox_Play_Sound_ (0x147d7): Play_Sound_ when sound
// effects are on (settings +0x11); the handle, or 0.
func (g *Game) moxPlaySound(n int) int {
	if g.e == nil || g.settings[0x11] == 0 {
		return 0
	}
	return g.e.PlaySound(n)
}

// clockTicks is clock_(): hundredths of a second.
func (g *Game) clockTicks() uint32 {
	return uint32(g.e.Sys.Elapsed().Milliseconds() / 10)
}

// setSoundVolumeToSettings is Set_Sound_Volume_To_Settings_ (0x7a06c).
func (g *Game) setSoundVolumeToSettings() {
	e, st := g.e, &g.settings
	v := min(int(st[0x14]), 100)
	if st[0x13] != 0 {
		e.StreamOn()
	} else {
		e.StreamOff()
	}
	e.SetStreamMaxVolume(v)
	e.SetStreamVolume(v)
	if st[0x11] == 0 {
		e.StopAllSounds()
	} else {
		e.SoundOn()
		// Resume_Sound_(-1) resumes paused samples; none are paused.
	}
	e.SetSoundMaxVolume(int(st[0x12]))
}

func (g *Game) setStreamFor(n int16) int {
	if n > 100 {
		g.e.SetStreamFilename(cStringAt(g.exe, streamHDLBX))
		return int(n - 100)
	}
	g.e.SetStreamFilename(cStringAt(g.exe, streamLBX))
	return int(n)
}

// playStreamingMusic is Play_Streaming_Music_ (0x24677), called each pass
// of a screen: n is the track wanted, next what follows when it ends (-1
// nothing: n loops when loop; -2 a random STREAM.LBX 1..3). It fades the
// old track down, starts n faded up, and then follows it with next.
func (g *Game) playStreamingMusic(n, next int16, loop bool) {
	e, m := g.e, &g.mus
	if e == nil || g.settings[0x13] != 1 {
		return
	}
	if n != m.want {
		m.state, m.want, m.next = 1, n, next
	}
	switch m.state {
	case 1:
		e.FadeStreamDown(0x1e)
		m.state = 2
	case 2:
		if e.StreamVolume() != 0 {
			return
		}
		e.StopStream()
		e.PlayStream(g.setStreamFor(m.want))
		m.current = m.want
		if m.next == -1 && loop {
			e.LoopStream(-1) // eax is still -1 from the compare
		}
		e.SetStreamVolume(0)
		e.FadeStreamUp(0x1e)
		if m.next != -1 {
			m.state = 3
			return
		}
		m.state = 4
	case 3:
		if e.IsStreamPlaying() {
			return
		}
		e.StopStream()
		if m.next == -2 {
			m.current = int16(g.clockTicks()%3 + 1)
			e.SetStreamFilename(cStringAt(g.exe, streamLBX))
			e.PlayStream(int(m.current))
		} else {
			e.PlayStream(g.setStreamFor(m.next))
			m.current = m.next
		}
		if loop {
			e.LoopStream(-1)
		}
		e.SetStreamVolume(0)
		e.FadeStreamUp(0x1e)
		m.state = 4
	}
}

// stopBackgroundMusic is Stop_Background_Music_ (0x24831).
func (g *Game) stopBackgroundMusic() {
	g.mus.want, g.mus.current = -1, -1
	g.e.SetStreamVolume(0)
	g.e.StopStream()
}

// fadeStreamWait fades the stream down by 1 a Sound_Update_ and waits it
// out with Mox_Update_ (the loops of the music routines).
func (g *Game) fadeStreamWait(every int) {
	e := g.e
	e.FadeStreamDown(1)
	for k := 0; e.StreamVolume() > 0; k++ {
		if k%every == 0 {
			e.MoxUpdate()
		}
	}
}

// playBackgroundMusic is Play_Background_Music_ (0x2484f): unless STREAM.LBX
// 1..3 already plays, fade down, then one of them by clock_(), looped,
// faded up.
func (g *Game) playBackgroundMusic() {
	e, m := g.e, &g.mus
	if e == nil || g.settings[0x13] != 1 {
		return
	}
	m.want = -1
	if m.current >= 1 && m.current <= 3 {
		return
	}
	g.fadeStreamWait(1)
	e.StopStream()
	m.current = int16(g.clockTicks()%3 + 1)
	e.SetStreamFilename(cStringAt(g.exe, streamLBX))
	e.PlayStream(int(m.current))
	e.LoopStream(-1)
	m.want = -1
	e.SetStreamVolume(0)
	e.FadeStreamUp(0x1e)
}

// fadeDownBackgroundMusic is Fade_Down_Background_Music_ (0x2490d):
// Mox_Update_ only every 18th pass of its wait.
func (g *Game) fadeDownBackgroundMusic() {
	if g.e == nil || g.settings[0x13] != 1 {
		return
	}
	g.mus.want = -1
	g.fadeStreamWait(0x12)
	g.e.StopStream()
	g.mus.current = -1
}

// playCombatMusic is Play_Combat_Music_ (0x2496c): STREAM.LBX 4..6.
func (g *Game) playCombatMusic() {
	e, m := g.e, &g.mus
	if e == nil || g.settings[0x13] != 1 {
		return
	}
	m.want = -1
	g.fadeStreamWait(1)
	e.StopStream()
	m.current = int16(g.clockTicks()%3 + 4)
	e.SetStreamFilename(cStringAt(g.exe, streamLBX))
	e.PlayStream(int(m.current))
	e.LoopStream(-1)
	e.SetStreamVolume(0)
	e.FadeStreamUp(0x1e)
}
