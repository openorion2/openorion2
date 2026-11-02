// Package random ports the engine's random number generator (random.c): a
// 32-bit LCG with rejection sampling so that every result is equally likely.
package random

import "time"

// Source holds _random_seed.
type Source struct {
	Seed uint32
}

// Set is Set_Random_Seed.
func (s *Source) Set(n uint32) {
	s.Seed = n
}

// Get is Get_Random_Seed.
func (s *Source) Get() uint32 { return s.Seed }

// Randomize seeds from the wall clock, as time(NULL) does.
func (s *Source) Randomize() { s.Set(uint32(time.Now().Unix())) }

// Random is Random_: a value in 1..n. The original divides by n before its
// zero check and so faults on n == 0; we return 1 like the guarded path.
func (s *Source) Random(n uint32) uint32 {
	if n == 0 {
		return 1
	}
	q := 0xffffffff / n
	limit := q * n
	for {
		s.Seed = s.Seed*0x41c64e6d + 0x3039
		if s.Seed < limit {
			break
		}
	}
	r := s.Seed/q + 1
	return r
}
