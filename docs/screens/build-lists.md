# Build lists (community patch 1.51)

The keys Q and 0-9 load a text file into a colony's build queue: in the
build screen into the colony on screen, in the colony summary into the
colony under the pointer, or with Shift into every colony of the list. A
BUY on a bought product takes the purchase back. Not in 1.40b23: this is
the feature of the unofficial patch 1.50/1.51 (manual, "Build Queue
Management"), written from its description, not from its code. Code:
`internal/game/buildlist.go`; tests: `buildlist_test.go`.

## Files

| Key | File | Default |
|---|---|---|
| Q | `BUILD.CFG` | merge: the buildings in a set order |
| 0 | `BUILD0.CFG` | clear |
| 1-6 | `BUILD1.CFG`..`BUILD6.CFG` | prepend: design 1..6 |
| 7 | `BUILD7.CFG` | prepend: freighter fleet |
| 8 | `BUILD8.CFG` | prepend: spy |
| 9 | `BUILD9.CFG` | append: housing |

They live in `buildlists/` in the game's directory, file names in any
case. A missing one is written there from the defaults
(`internal/game/buildlists/`, the port's own) before it is read, so the
player finds them to edit. Shift on a US keyboard gives `Q ) ! @ # $ % ^ &
* (`; those are the shifted keys (Q and q are one hot key: the engine's
`LastKey`, port-only, tells them apart).

## Words

Words are separated by spaces and newlines, case does not matter, `#`
starts a comment to the end of the line, `stop` ends the list. A word the
list does not know shows a box with the file and the word, and nothing is
applied.

- Buildings: the English name in lower case with `_` for spaces
  (`automated_factory`, `artificial_planet`; `buildListBuildings`, checked
  against the game's names by `TestBuildListNames`).
- `colony_ship`, `outpost_ship`, `transport`, `design1`..`design6`,
  `android_farmer`, `android_worker`, `android_scientist`, `spy`,
  `freighter_fleet`, `trade_goods`, `housing`; `repeat` marks the product
  before it. `design6` is read and never queued until the game has a
  sixth design slot.
- Modes: `append` (the default), `prepend`, `merge`.
- Commands: `buy`, `buy10%`, `buy50%`, `buy75%`, `unbuy`, `clear`,
  `clear_refits`.

## Applying

Products gather into a group until a mode, a command or the end; then the
group goes into the queue in the current mode. A product joins only what
the build screen would offer for the colony at that moment
(`Calculate_Building_Array_`, `Calculate_Military_Array_`): a building
the colony may build and has not queued, a colony base while the star has
room for one more, androids while the planet has room (no question, they
are skipped), a design `Colony_Can_Build_Product_` allows. Ships are made
at once, as `Add_To_Queue_` makes them (support ships through design slot
5). A product that does not fit in the seven slots (a repeat mark takes
one) is skipped.

- append: each product at the end.
- prepend: the group at the head, in its order, behind a bought product.
- merge: each listed product already queued keeps its slot, missing ones
  are added at the end, then the listed ones are reordered in the list's
  order over the slots they hold; the rest of the queue does not move.
  Queued ships never count as listed, so a design in a merge is added
  every time.

A bought product (colony `Pending`) at the head never moves and is never
cleared.

Commands first apply the group, then:

- `buy`, `buy10%`, `buy50%`, `buy75%`: buy the product at the head when
  `Colony_Can_Buy_Product_0_` allows it and its stored production is at
  least the cost, a tenth, a half or three quarters of it (integer
  division). No question.
- `unbuy`: take a purchase back.
- `clear`: empty the queue but the bought product and ships being refitted.
- `clear_refits`: take out the ships being refitted (they are lost, as
  deleting them in the build screen loses them).

In merge mode a command acts only when the head product is listed (`buy`,
`unbuy`), and `clear` takes out only the listed products and drops the
group, so they are not added after. Ships taken out that the colony's own
queue does not hold are freed at once; the others when the queue is
written back (`Do_Exit_Screen_Cleanup_`, as in the build screen).

In the build screen the list edits the screen's copy of the queue: OK
keeps it, Cancel drops it, except that a buy or unbuy writes the queue to
the colony first, as the BUY button does. In the colony summary each
colony's queue is written back and the colony recalculated at once.

## Taking a purchase back

`Tested_Colony_Buys_Outright_` said E 0x24a (already bought) for a bought
product; now it gives the BC back and drops the bought production
(`colonyUnbuy`). The price is the same as paid: `Production` does not
change between the purchase and the turn's end, when `Pending` is added.
This is the BUY button of the colony screen, the build screen and the
colony summary.
