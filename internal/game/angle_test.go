package game

import (
	"math"
	"testing"
)

func TestTrig(t *testing.T) {
	t.Parallel()
	x, err := newExeImage("", nil)
	if err != nil {
		t.Skip(err)
	}
	g := &Game{exe: x}
	for a := -400; a < 800; a += 7 {
		want := 1000 * math.Sin(float64(a)*math.Pi/180)
		if got := g.sin(a, 1000); math.Abs(float64(got)-want) > 1.5 {
			t.Fatalf("sin(%d) = %d, want about %.1f", a, got, want)
		}
	}
	for a := 0; a < 360; a += 5 {
		dx, dy := int(1000*math.Cos(float64(a)*math.Pi/180)), int(1000*math.Sin(float64(a)*math.Pi/180))
		got := g.getAngle(dx, dy)
		if d := math.Abs(float64(got - a)); d > 1 && d < 359 {
			t.Fatalf("angle(%d,%d) = %d, want about %d", dx, dy, got, a)
		}
	}
}
