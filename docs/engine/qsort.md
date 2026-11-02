# qsort_ (Watcom C 10.x)

`qsort_` 0x133ef4 and `med3_` (just before it). Ported in `internal/clib`.
The order of elements with equal keys is visible in the game (ship icons,
ships in a stack), so the algorithm is ported, not replaced.

- `swaptype`: 2 if base or size is not a multiple of 4, else 1 if size > 4,
  else 0. Type 0 keeps the pivot as a copy taken from the median; the other
  types swap the median to the front and use it in place.
- n < 16: shell passes with gaps 3 and 1 (insertion with `cmp(prev, cur) > 0`).
- n ≥ 16: median of first/middle/last for n > 29, of three medians of
  three (step n/8) for n > 42. Three-way partition with the pointers pa, pb,
  pc, pd as in Bentley–McIlroy, the equal runs swapped into the middle, the
  smaller part handled next and the larger pushed (the `r <= size` test only
  on one side, as in the source).
- `med3_` differs from the Open Watcom v2 source:

```
if cmp(a,b) <= 0: return a if cmp(a,c) >= 0; b if cmp(b,c) <= 0; else c
else:             return a if cmp(a,c) <= 0; b if cmp(b,c) > 0;  else c
```
