# Critical Bug Fixes

Crashes, freezes and network desyncs of 1.40b23 that the community patch
1.51 fixed, as OpenOrion2 fixes them. Some of 1.51's fixes are not here
because OpenOrion2 never had those bugs; the list of every 1.51 fix and
what became of it is `docs/p151/items.md`.

All of these are *(from 1.51)* unless marked otherwise.

## General

**AI freeze on a blockaded colony.** An AI empire no longer freezes the
game on a colony that was freed from an empire with Biomorphic Fungi
while enemy ships blockade it. The colonists who cannot farm there go to
work in industry instead.

**Repeat and androids.** If you place a repeat mark on androids when the
planet is already full and answer No to the warning, nothing is queued.
Before, a bare repeat mark stayed in the queue and adding anything after
it froze the game.

**Star system popup.** After you look at ship details more than once from
the Fleets or Leaders screen, the star system popup shows its planets as
normal. Before, it could fail to draw them.

**The map behind popups after the news.** *(new in OpenOrion2)* With the
GNN setting off, the news comes as a small box over the map; the popup
after it (for example scouts arriving at Orion) showed the map without
its stars, names and figures. The original and 1.51 do this too.

**Ship costs follow their owner's government.** A design's cost now
always uses its own empire's government and technology. Before, the
viewing player's counted: a Feudal player made every rival's ships a
third cheaper, and a Feudal AI got no discount.

## Tactical Combat

**Crystal takes over a ship with fighters out.** When the Crystal (or a
telepath) takes over a ship whose fighters or missiles are in flight,
they keep counting as their original owner's and can still be shot down.
This used to crash or freeze the battle.

**AI and the missiles of a captured ship.** After a ship is captured, the
AI no longer keeps shooting at its own missiles or ignores the enemy's;
this could stall the battle.

**Ships without a drive.** Trans-Dimensional races can fly ships without
a drive. Such ships are now real ships in combat: they deploy with the
fleet, move at the trans-dimensional bonus speed and may retreat. Before,
they were placed and turned like a star base, and winning a battle with
them or scanning one could crash the game. The ship view now judges a
damaged drive by half its hit points, as combat does.

**Retreat of a stopped ship.** A ship ordered to retreat stays in the
battle if, before it can jump out, it is stopped (black hole generator,
tractor beams, a destroyed drive) or captured; its retreat order is
cancelled. Before, such a ship was lost after the battle.

## Network Multiplayer

**Research in battles.** The computer that fights a battle now knows each
empire's research as the host has it, so the battle and what follows it
match on every computer. Long battle messages no longer stall either.

**Boarding with no marines.** If a Damper Field leaves your boarding
party with no marines, the boarding still happens and fails, instead of
freezing a network battle. This holds in single player too: the ship
counts as having boarded and loses its cloak.

**Battles replayed.** A monster or an AI fighting at two stars at once no
longer releases a player from his other battle, which could replay it.

**Dismissed leaders.** A leader you dismiss goes back to the pool one
level higher on every computer, as in single player.

**AI battles during the host's battle.** While the host plays a battle of
his own, the computer players' battles wait for it to end instead of
being fought in the middle of it, which could crash the game.

## Compared with 1.51

- The fixes are 1.51's, read from its code, with two differences: when a
  retreat is cancelled, 1.51 also wipes a few of the ship's special
  systems by mistake, and OpenOrion2 does not; and where 1.51 stops the
  game with "nested combat detected", OpenOrion2 never gets there.
- Several crashes 1.51 fixes cannot happen in OpenOrion2 (sounds, long
  messages, the race list, Alt+X, German and Italian texts, the
  serendipity event, large network battles); they are not listed here.
- Not yet in OpenOrion2: 1.51's fixes for the Artemis System Net in
  network battles, the Cluster galaxy in multiplayer, and a few crashes
  that still need checking (rejecting the Council's ruling, scrolling
  fleets on the Leaders screen, shuttle captures in network battles).
