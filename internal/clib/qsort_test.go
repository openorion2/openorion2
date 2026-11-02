package clib

import (
	"math/rand"
	"sort"
	"testing"
)

func TestQsortSorts(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	for _, size := range []int{4, 12} {
		for n := 0; n < 300; n++ {
			a := make([]int, n)
			for i := range a {
				a[i] = r.Intn(7)
			}
			Qsort(a, size, func(x, y *int) int { return *x - *y })
			if !sort.IntsAreSorted(a) {
				t.Fatalf("size %d n %d not sorted: %v", size, n, a)
			}
		}
	}
}
