# Lines

The WLIBS line primitives and the game wrapper the main screen uses for
moving-ship lines. Read from the assembly in `build/orion2.dis`; ELF
addresses. Port: `internal/engine/line.go`.

| Routine | Addr | Port |
|---|---|---|
| `Line_(x1, y1, x2, y2, color)` | 0x134d2d | `Video.Line` |
| `Clip_Line_(&x1, &y1, &x2, &y2)` | 0x135357 | `Video.clipLine` |
| `Draw_Horizontal_Run_` / `Draw_Vertical_Run_` | 0x13579c / 0x135806 | inlined |
| `Multi_Colored_Line_(x1, y1, x2, y2, colors, rot)` | 0x135870 | `Video.MultiColoredLine` |
| `Draw_Horizontal_Run_Multi_` / `Draw_Vertical_Run_Multi_` | 0x135edd / 0x135f5d | inlined |
| `Add_Dot_To_Scan_List_(x, y)` | 0x125e13 | `Video.AddDot` |
| `Draw_Directional_Multi_Colored_Line_` (MOX) | 0xa11c0 | `Video.DirectionalMultiColoredLine` |

Not ported here: `Anti_Alias_Line_` (0x135fdd, the fourth `Clip_Line_`
caller), `Plasma_Line_`, `Bolt_Line_`, `Wide_Bolt_Line_`.

## Arguments and integer width

`Line_`: EAX x1, EDX y1, EBX x2, ECX y2, colour on the stack (low byte;
`ret 4`). `Multi_Colored_Line_`: same registers, then a pointer to 8 colour
bytes and an int `rot` on the stack (`ret 8`). The arguments are spilled as
dwords, but every comparison, `Swap_Short_` and `Clip_Line_` write is 16-bit,
so the coordinates behave as `int16`. Run lengths and the error term are
tested as words too. The port converts to `int16` on entry.

## Line_

1. If `clipping_flag` (word 0x1845d8) == 1 and `Clip_Line_` returns 1:
   return. The clipped ends replace the arguments.
2. If `y1 > y2`, swap the ends (both coordinates). Lines are always drawn
   from the top end down.
3. `p = current_video_page + y1*640 + x1` (no bounds check; the port drops
   writes outside the page).
4. `dx = x2 - x1`; if negative, `adv = -1` and `dx = -dx`, else `adv = 1`.
   `dy = y2 - y1` (>= 0).
5. Cases, in this order:
   - `dx == 0` (vertical, includes a single point): `dy+1` pixels down.
   - `dy == 0` (horizontal): `dx+1` pixels stepping `adv`.
   - `dx == dy` (45°): `dx+1` pixels stepping `640 + adv`.
   - `dx > dy` (x-major) or else y-major: run slicing (Abrash), below.

### Run slicing

With `M` the major delta and `m` the minor one (`m >= 1`, `M > m`):

```
whole    = M / m
adj_up   = (M % m) * 2
adj_down = m * 2
err      = M % m - m * 2
initial  = whole/2 + 1          (sar on whole)
final    = initial
if adj_up == 0 && whole is even: initial -= 1
if whole is odd:                 err += m
run(initial)
repeat m-1 times:
    n = whole
    err += adj_up
    if (int16)err > 0: n += 1; err -= adj_down
    run(n)
run(final)
```

An x-major run (`Draw_Horizontal_Run_`) writes `n` pixels stepping `adv`,
then moves the pointer one row down. A y-major run (`Draw_Vertical_Run_`)
writes `n` pixels down, then steps `adv`. A run of `n <= 0` writes nothing.
The total is always `M + 1` pixels and `m + 1` runs.

## Dirty marking

Only when `screen_refresh_flag >= 2`, after the pixels (or after each run):

- vertical: `Add_Dot(x1, y)` for every `y` in `y1..y2`;
- horizontal: order `x1 <= x2`, `Add_Square(x1, y1, x2, y1)`;
- 45°: `Add_Dot(x1 ± (y - y1), y)` per row, `+` when `x1 < x2`;
- x-major, per run of `n` on row `y`: `Add_Square(x, y, x+n, y)` then
  `x += n` if `adv > 0`, else `Add_Square(x-n, y, x, y)`, `x -= n`; then
  `y++`. **Quirk**: the square covers `n + 1` pixels, one past the run, so a
  run ending on the last pixel of a dword column also marks the next one;
- y-major, per run: `Add_Dot(x, y+i)` for `i < n`, `y += n`, `x += adv`.

`(x, y)` start at the drawn top end (after clipping and the swap).
`Add_Square` clamps to the screen; `Add_Dot` does not.

## Add_Dot_To_Scan_List_ (0x125e13)

`c = x >> 2` (arithmetic, so `x = -1..-4` gives `-1`). For row `y` of
`refresh_scan_list` and, unless `screen_refresh_flag == 2`, of
`refresh_stencil_list`:

- `x_start == -1`: `x_start = x_end = c` (with `c == -1` the row stays
  clean);
- else if `x_start > c`: `x_start = c`;
- else if `x_end < c`: `x_end = c`.

Only one end moves (matters only for a reversed span). There is no check on
`y`: rows outside 0..479 would write past the lists; the port ignores them.

## Clip_Line_ (0x135357)

Window `screen_window_x1/x2/y1/y2` (0x1bba4c/4e/50/52), inclusive: a line
lying on the window edge is drawn (unlike `Fill`). Returns 1 when rejected.
Products are 32-bit `imul`, quotients `idiv` (truncated toward zero), results
stored as words. The x part runs first; **the y part always runs**, even
after the x part rejected (the flag is never cleared, so the result stands).

x part:

- `x1 == x2`: reject if `x1 < wx1` or `x2 > wx2`; else order `y1 <= y2`,
  reject if `y2 < wy1` or `y1 > wy2`, else clamp `y1 = max(y1, wy1)`,
  `y2 = min(y2, wy2)`.
- else order `x1 <= x2` (swap whole ends), `ddy = y2 - y1`, `ddx = x2 - x1`;
  if `x1 < wx1`: `y1 += (wx1 - x1)*ddy/ddx`, `x1 = wx1`. Reject if
  `x1 > x2`. Else if `x2 > wx2`: `y2 = (wx2 - x1)*ddy/ddx + y1` (the new
  `x1, y1`, the old deltas), `x2 = wx2`; reject if `x2 < x1`.

y part, the same with the axes exchanged:

- `y1 == y2`: reject if `y1 < wy1` or `y2 > wy2`; else order `x1 <= x2`,
  reject if `x2 < wx1` or `x1 > wx2`, else clamp x.
- else order `y1 <= y2` (whole ends), `ddx = x2 - x1`, `ddy = y2 - y1`;
  if `y1 < wy1`: `x1 += (wy1 - y1)*ddx/ddy`, `y1 = wy1`; reject (return at
  once) if `y1 > y2`; if `y2 > wy2`: `x2 = (wy2 - y1)*ddx/ddy + x1`,
  `y2 = wy2`; reject if `y2 < y1`.

Neither division can be by zero. Because the intersections truncate, the
clipped line is generally not the same pixel set as the unclipped one cut to
the window (e.g. `(0,20)-(30,5)` in window 10..20 becomes `(20,10)-(10,15)`).

## Multi_Colored_Line_ (0x135870)

`Line_` with the same clipping, swap, cases, runs and marking; the pixel
colour is `colors[rot & 7]` and `rot` is incremented after every pixel, in
drawing order (top end first). Clipping does not advance `rot`, so a clipped
start shifts the pattern. The run helpers take `&rot` and increment it as a
word; only its low 3 bits matter.

## Draw_Directional_Multi_Colored_Line_ (0xa11c0)

`(x1, y1, x2, y2)` in AX, DX, BX, CX (words), `colors` pointer and `start`
on the stack. `rev = 7 - start`, or 0 if that is `<= 0`.

- forward when `x2 > x1 && y2 >= y1`, or `x2 <= x1 && y2 > y1`:
  `Multi_Colored_Line(x1, y1, x2, y2, colors, rev)`;
- otherwise (end 2 above end 1, or level with it and not to its right):
  `buf[7-i] = colors[i]`, `Multi_Colored_Line(x1, y1, x2, y2, buf, start)`.

So the pattern phase is chosen relative to end 1, compensating for
`Multi_Colored_Line_` always drawing from the top. Callers on the main
screen: `Draw_Ship_Destination_Line_` (0xa1299),
`Draw_ETA_Destination_Line_`, `Draw_Nearest_Ship_Directional_Line_`.
Wormhole links and the other main-screen markers use plain `Line_` (see
`docs/screens/main-screen-stars.md`).

## Open questions

1. Offscreen coordinates without clipping write outside the page in the
   original (and `Add_Dot` outside the lists); the port drops them. Check
   that no caller relies on that.
2. `int16` wrap of extreme deltas (|dx| >= 32768) is modelled only loosely;
   no caller comes close.
