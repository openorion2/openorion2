package random

import "testing"

func TestRandom(t *testing.T) {
	s := Source{Seed: 1}
	// seed 1 -> 0x41c67ea6 (< limit for n=6), 0x41c67ea6 / (0xffffffff/6) + 1.
	got := s.Random(6)
	if s.Seed != 0x41c67ea6 {
		t.Fatalf("seed %#x", s.Seed)
	}
	if want := uint32(0x41c67ea6)/(0xffffffff/6) + 1; got != want {
		t.Fatalf("got %d want %d", got, want)
	}
	for i := 0; i < 1000; i++ {
		if v := s.Random(10); v < 1 || v > 10 {
			t.Fatalf("out of range %d", v)
		}
	}
}
