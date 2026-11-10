# Interface Changes

All of these are *(from 1.51)*.

## Auto Save

With Auto Save on (Game Settings), a single-player game now saves to slot
10 every turn, before the turn's battles, as network games always did. In
1.40b23 it saved every fourth turn. To go back one turn after a lost
battle, load slot 10.

## Animation Speed

Press PageUp anywhere in the game to speed up all animation, and PageDown
to slow it down, up to three steps either way. The setting lasts until you
quit the game. (In the online lobby these keys scroll the chat and the
list of games instead.)

## Tactical Combat: All Wait

In combat with ship initiative on, press H to make all your ships wait at
once, like T for all done. Your ships are asked again only after the
enemy has moved all of theirs; if both sides press H, the defender waits
longer. The WAIT button works as before. This works in network battles
too.

## Main Screen

**Enemy Moves.** Turn on Enemy Moves in the Game Settings menu to see red
destination lines for every enemy fleet in your scanning range, not just
those heading for your systems. In 1.40b23 the option did nothing.

**Distance between two stars.** F9 now shows the distance between two
stars even when a black hole lies between them. Before, it only said that
a black hole was in the way.

## Colony Screen

The build box now also shows how much production is stored for the
current product, bought production included, against the product's total
cost, for example `49 / 150`. The turns left move to the left of the box.

## Leaders Screen

Each leader shows his experience points and the points needed for the
next level, at the bottom right of his row, for example `184/300 EP`.
Past 500 the target stays at 1000.

## Start of Turn Reports

The reports at the start of a turn come in a more sensible order:

- a spy's report of a stolen technology comes before the screen for
  choosing new research after a breakthrough;
- newly explored systems are shown before you are asked to colonize;
- the offer to hire a leader comes after both, so a marooned hero found
  this turn can still join even if the offer would have filled your last
  place.

## Compared with 1.51

- 1.51 shows "Animation speed +N" in combat for a moment after PageUp or
  PageDown; OpenOrion2 shows nothing, since the game's own texts have no
  words for it.
- The colony screen says the turns left with the game's own "turn(s)"
  text, and the leaders screen labels experience with the game's own
  abbreviation for crew experience (EP in English); 1.51 writes its own
  words there.
- 1.51 also ships a batch file that copies each new slot 10 save; there is
  no such helper here.
