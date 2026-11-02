//go:build !js

package host

// cooperative is set where the game shares one thread with the host (the
// browser), so a game loop polling input must let the host run.
const cooperative = false

func (s *System) yield() {}
