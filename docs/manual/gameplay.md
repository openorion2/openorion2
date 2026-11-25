# Gameplay Changes

All of these are *(from 1.51)*.

## Production

### What the AI and AUTO Build Choose

Computer empires pick each colony's next building by scoring every
building the colony can build. A colony of yours with AUTO on picks the
same way when its queue runs empty. Six of those scores changed, so the
choice fits the colony's size and needs better:

| Building | 1.40b23 | Now |
|---|---|---|
| Astro University | a fixed 5 | the colony's population |
| Recyclotron | grew with the planet's population limit | the colony's population |
| Autolab | a bigger bonus for a research-minded AI | half that bonus |
| Food Replicators | on any colony | only where the colony does not grow food for all its people |
| Stock Exchange | from 5 population | from 4 population |
| Spaceport | from 3 population | from 4 population |

For example, a computer empire no longer puts Food Replicators on a
farming world that feeds itself, nor a Spaceport on a fresh colony of
three; a large colony now wants an Astro University more than a small
one does.

### AUTO Build and New Population

In 1.40b23 a colony on AUTO was worked out anew whenever it chose its next
building, so the population grown that turn already worked that turn. A
colony with the same queue picked by hand had to wait a turn. Now both
wait: AUTO only chooses the building.

## Compared with 1.51

- The same scores as 1.51. 1.51 compares the population with only the low
  byte of the colony's food, so a colony growing more than 255 food can be
  judged hungry there; OpenOrion2 compares the whole number.
- 1.51 lets a mod set every building's score in its config
  (`building_score`); OpenOrion2 has no config for it yet.
