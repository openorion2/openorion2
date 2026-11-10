# Build Queue Management

*(from 1.51)*

## Overview

Setting up a colony's build queue by hand, colony after colony, is most
of the clicking in a long game. Build lists do it with one key. A build
list is a plain text file naming what to build; pressing its key loads it
into the queue.

There are eleven lists, on the keys Q and 0 to 9. They work in two
places:

- In the Build screen (the colony's construction orders), a list goes
  into the queue of the colony on screen.
- In the Colonies screen, a list goes into the queue of the colony under
  the mouse pointer. Hold Shift and it goes into the queue of every
  colony at once.

The lists you get at first:

| Key | File | What it does |
|---|---|---|
| Q | BUILD.CFG | queues the buildings the colony can build, in a sensible order, and puts those already queued into that order |
| 0 | BUILD0.CFG | empties the queue |
| 1 | BUILD1.CFG | puts a ship of design 1 at the head of the queue |
| 2-6 | BUILD2.CFG - BUILD6.CFG | the same for designs 2 to 6 |
| 7 | BUILD7.CFG | puts a freighter fleet at the head of the queue |
| 8 | BUILD8.CFG | puts a spy at the head of the queue |
| 9 | BUILD9.CFG | adds housing at the end of the queue |

Q alone is enough to set up a new colony in a game against the AI: press
it whenever a colony has nothing sensible to build, or after you research
a new building.

The files are in the folder `buildlists` inside the game's folder. They
appear there the first time you press a list's key; open them in any
text editor (Notepad will do) and change them as you like. To get a
default list back, delete its file.

## Taking a purchase back

You can now undo a purchase. Press BUY again on a product you bought this
turn, in the Colony screen, the Build screen or the Colonies screen: the
BC you paid come back and the production you bought is gone. In 1.40b23
the game only said the product was already bought.

## What a list says

A list is a series of words, separated by spaces or line breaks. Capital
letters do not matter. Everything after a `#` on a line is a comment; you
can use comments and empty lines as you like.

Each word is a product, a mode or a command.

### Products

- Buildings: the building's name as the game writes it, in lower case,
  with `_` instead of spaces: `automated_factory`, `hydroponic_farm`,
  `research_lab`, `star_base`, `artificial_planet`. The default
  BUILD.CFG names most buildings; copy from there if unsure.
- Ships: `colony_ship`, `outpost_ship`, `transport`, and your designs
  `design1` to `design6`.
- Androids: `android_farmer`, `android_worker`, `android_scientist`.
- Others: `spy`, `freighter_fleet`, `trade_goods`, `housing`.
- `repeat`: repeat the product before it (as the REPEAT BUILD button).

### Modes

Modes decide where products go. A mode stays until the next one.

- `append`: at the end of the queue. This is the mode a list starts in.
- `prepend`: at the head of the queue, in the order the list names them.
  A product you have bought stays first.
- `merge`: products already in the queue are put into the list's order,
  in the places they hold now; the list's products not yet in the queue
  are added and put in order too. Whatever else is in the queue does not
  move. Use it to queue newly researched buildings where they belong.

### Commands

- `buy`: buy the product at the head of the queue, if you can afford it.
- `buy10%`, `buy50%`, `buy75%`: buy it only if a tenth, half or three
  quarters of it is built already (buying a product that is partly built
  is cheaper).
- `unbuy`: take a purchase back.
- `clear`: empty the queue, except a product you bought and ships being
  refitted.
- `clear_refits`: take ships being refitted out of the queue. As in the
  Build screen, a ship taken out of a refit is lost.
- `stop`: ignore the rest of the file.

In merge mode, commands only act on the products the list names: `buy`
buys the product at the head only if the list names it, and `clear` takes
out only the products the list names.

### What a list cannot do

A list only queues what you could queue yourself in the Build screen: a
building the colony can build and has not queued yet, colony bases while
the system has planets left for them, androids while the planet has room,
ships you can build. Anything else in the list is skipped, as is whatever
no longer fits in the queue's seven places (a repeat takes a place too).

If a list has a word the game does not know, nothing is done and the game
shows the file's name and that word, for example `BUILD3.CFG: desing1`.

## Examples

Add a transport at the end of the queue:

```
transport
```

The same, with the mode said:

```
append
transport
```

Put a spy and a freighter fleet at the head of the queue:

```
prepend
spy
freighter_fleet
```

Empty the queue but bought production and refits, then add two android
workers:

```
clear
android_worker
android_worker
```

Throw everything out of the queue and build design 5 over and over:

```
unbuy
clear
clear_refits
design5
repeat
```

Queue a research lab on a colony that has its automated factory, and
both where it does not:

```
merge
automated_factory
research_lab
```

Take only the hydroponic farms and food replicators out of the queue:

```
merge
hydroponic_farm
food_replicators
clear
```

Buy the automated factory if it is the product at the head of the queue,
nothing else:

```
merge
automated_factory
buy
```

Buy whatever is first in the queue once it is half built:

```
buy50%
```

## Compared with 1.51

- The default lists are our own, not 1.51's; they do the same jobs on the
  same keys, but BUILD.CFG orders the buildings differently.
- The lists are in `buildlists` in the game's folder (1.51: `150\build`),
  and the game writes the default ones there itself.
- Design 6: OpenOrion2 does not have a sixth design slot yet, so
  `design6` and the key 6 do nothing for now.
- Androids that would not fit on the planet are skipped without a
  question.
- In merge mode, ships in the queue never count as the ones the list
  names, so a list that merges `design1` adds a new ship each time.
- We wrote the feature from the 1.51 manual, not from 1.51's code. Where
  the manual says nothing (a full queue, rounding of `buy10%`), 1.51 may
  do it differently.
- 1.51 has a `flush` word its manual does not describe; OpenOrion2 does
  not know it.
- The Shift key in the Colonies screen works as in 1.51: Q or a digit for
  the colony under the pointer, Shift with it for every colony.
