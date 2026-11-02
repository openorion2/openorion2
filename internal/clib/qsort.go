// Package clib holds the C runtime routines whose behaviour the player can
// see, ported from the Watcom C 10.x code in ORION2.EXE.
package clib

// Qsort is Watcom's qsort_ (0x133ef4): a 2-pass shell sort below 16
// elements, else a three-way quicksort with median-of-3 (median-of-9 above
// 42). size is the C element size: for 4-byte elements the pivot is a copy
// taken before partitioning, for larger ones it is swapped to the front.
// The base is assumed 4-byte aligned, as for every array the game sorts.
// Spec: docs/engine/qsort.md.
func Qsort[T any](a []T, size int, cmp func(x, y *T) int) {
	wordSize := size == 4 // swaptype 0
	type span struct{ base, n int }
	var stack []span
	base, n := 0, len(a)
	for {
		for n > 1 {
			if n < 16 {
				for shell := 3; shell > 0; shell -= 2 {
					for p1 := base + shell; p1 < base+n; p1 += shell {
						for p2 := p1; p2 > base && cmp(&a[p2-shell], &a[p2]) > 0; p2 -= shell {
							a[p2], a[p2-shell] = a[p2-shell], a[p2]
						}
					}
				}
				break
			}
			mid := base + n>>1
			if n > 29 {
				p1, p2 := base, base+n-1
				if n > 42 {
					s := n >> 3
					p1 = med3(a, p1, p1+s, p1+2*s, cmp)
					mid = med3(a, mid-s, mid, mid+s, cmp)
					p2 = med3(a, p2-2*s, p2-s, p2, cmp)
				}
				mid = med3(a, p1, mid, p2, cmp)
			}
			var pv *T
			if wordSize {
				v := a[mid]
				pv = &v
			} else {
				a[base], a[mid] = a[mid], a[base]
				pv = &a[base]
			}
			pa, pb := base, base
			pc, pd := base+n-1, base+n-1
			for {
				for ; pb <= pc; pb++ {
					c := cmp(&a[pb], pv)
					if c > 0 {
						break
					}
					if c == 0 {
						a[pa], a[pb] = a[pb], a[pa]
						pa++
					}
				}
				for ; pb <= pc; pc-- {
					c := cmp(&a[pc], pv)
					if c < 0 {
						break
					}
					if c == 0 {
						a[pc], a[pd] = a[pd], a[pc]
						pd--
					}
				}
				if pb > pc {
					break
				}
				a[pb], a[pc] = a[pc], a[pb]
				pb++
				pc--
			}
			pn := base + n
			s := min(pa-base, pb-pa)
			swapBlock(a, base, pb-s, s)
			s = min(pd-pc, pn-pd-1)
			swapBlock(a, pb, pn-s, s)
			r, s := pb-pa, pd-pc
			if s < r {
				if r <= 1 {
					break
				}
				stack = append(stack, span{base, r})
				base, n = pn-s, s
			} else {
				stack = append(stack, span{pn - s, s})
				n = r
			}
		}
		if len(stack) == 0 {
			return
		}
		top := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		base, n = top.base, top.n
	}
}

// swapBlock exchanges the k elements at i with those at j, front to back.
func swapBlock[T any](a []T, i, j, k int) {
	for x := 0; x < k; x++ {
		a[i+x], a[j+x] = a[j+x], a[i+x]
	}
}

// med3 is Watcom 10's med3_, which differs from the Open Watcom one.
func med3[T any](a []T, x, y, z int, cmp func(x, y *T) int) int {
	if cmp(&a[x], &a[y]) <= 0 {
		if cmp(&a[x], &a[z]) >= 0 {
			return x
		}
		if cmp(&a[y], &a[z]) <= 0 {
			return y
		}
		return z
	}
	if cmp(&a[x], &a[z]) <= 0 {
		return x
	}
	if cmp(&a[y], &a[z]) > 0 {
		return y
	}
	return z
}
