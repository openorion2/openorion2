# Ships and Fleets

## Sixth ship design

*(from 1.51)*

You can now keep six ship designs instead of five. In 1.40b23 the sixth
slot was used by the game itself to build colony ships, outpost ships and
transports and to refit ships, so you never had it.

- A new game gives you six designs: a scout and one design for each hull
  from the frigate up, one hull more than before.
- The sixth design shows up wherever the others do: in the Build screen's
  list of ships, under DESIGN (pick it to redesign it), in the refit popup,
  and as `design6` in build lists (key 6 by default, see [Build Queue
  Management](build-queue.md)).
- Building colony ships, outposts, transports and refitting ships no
  longer touches it.
- In a strategic combat game, ship designs are made by the game as
  before; the sixth one is the doom star, offered once you know the titan
  or the doom star hull. The separate Doom Star entry of the list is gone.

### Older saves

A game saved by 1.40b23 (or by an earlier OpenOrion2) may have a colony
ship, outpost ship or transport in the sixth slot, left there by the game.
When such a game loads, that slot gets a doom star design instead; change
it under DESIGN as you like. A refit leftover is a real design and stays.

### Compared with 1.51

Same as 1.51, with one difference: if you refit a ship while the build
queue is full, 1.51 can lose your sixth design; OpenOrion2 keeps it.
