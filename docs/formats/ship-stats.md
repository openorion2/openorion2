# Ship hit points: structure, armour, hull and armour tables

How the game derives a ship's hit points from its design (record layouts in
`save-ships.md`). Addresses are ELF addresses in `build/orion2.elf`; the
tables are read from ORION2.EXE at those addresses (`internal/game/exe.go`).
Go: `internal/game/shipstats.go`. Marks: **?** means inferred.

`DAT_00199cb4` is `_settings + 0xd8`, the combat setting (1 strategic,
0 tactical; `save-misc.md`). Below it is called *strategic*.

## Routines

| Address | Name | Go | What it does |
|---|---|---|---|
| 0x5f2f6 | `Max_Ship_Hits_(ship)` | `maxShipHits` | not strategic: `Get_Ship_Structure_ + Get_Ship_Armor_Hits_`. Strategic: `h = hull.strategic_hits`; if `Best_Armor_(owner) > 0`, `h + armor[best].pct*h/100` (ship's own armour ignored) |
| 0x5f2c3 | `Current_Ship_Hits_(ship)` | `currentShipHits` | 16-bit: max − `structure_damage` (+0x7d), and − `armor_damage` (+0x7b) unless strategic |
| 0x58387 | `Get_Ship_Structure_(ship)` | `shipStructure` | owner ≤ 9 (signed): `Get_Design_Structure_(ship design)`; owners 10..14: fixed values below; owner ≥ 15 returns the caller's ECX (garbage; Go returns 0) |
| 0x582bf | `Get_Design_Structure_(design by value)` | `designStructure` | not strategic: `s = hull.structure + armor[design.armor].pct*hull.structure/100`, ×3 with special bit 27 (reinforced hull). Strategic: `hull.strategic_hits` (u16) |
| 0x58425 | `Get_Ship_Armor_Hits_(ship)` | `shipArmorHits` | `a = hull.armor + armor[design.armor].pct*hull.armor/100`, ×3 with special bit 14 (heavy armour); independent of the setting |
| 0x5685f | `Best_Armor_(owner)` | `bestArmor` | players 0..7: highest class i in 2..6 whose tech application `armor[i].tech` has status 3 in `player.app_status` (+0x117), else 1. Owners 8, 9 (Antarans, Guardian): 6; 10..14: 0; 15+: 1. The ≥ 8 tests are unsigned 16-bit |
| 0x78182 | `Is_Combat_Ship_(ship)` | `isCombatShip` | design `type == 0` |

Not ported (not called by the above): `Get_Structure_` 0x58330 computes the
same structure from a hull size, `Best_Armor_(player)` and a reinforced flag.

All divisions are C `idiv` (truncation toward zero); percentages are signed.
Armour class 0 has pct −100, so a design with no armour gets 0 armour hits
and 0 structure.

### Monster structure (`Get_Ship_Structure_`, jump table at 0x58373)

Design byte 0x13 (`drive`) selects the stronger variant when non-zero.

| Owner | Monster | drive = 0 | drive ≠ 0 |
|---|---|---|---|
| 10 | Amoeba | 400 | 750 |
| 11 | Crystal | 500 | 2500 |
| 12 | Dragon | 500 | 2500 |
| 13 | Eel | 300 | 1000 |
| 14 | Hydra | 500 | 1500 |

Armour hits for monsters still come from the design (`Get_Ship_Armor_Hits_`).

## Hull table `_hull_data` (0x180024, 0x24-byte rows, index = design size 0..5)

| Off | Size | Meaning | Values (frigate..doom star) |
|---|---|---|---|
| 0x00 | i16 | base armour hits (`Get_Ship_Armor_Hits_`) | 4, 10, 30, 50, 80, 150 |
| 0x02 | i16 | base structure (`Get_Design_Structure_`, `Get_Structure_`) | 4, 10, 30, 50, 80, 150 |
| 0x14 | u16 | strategic hit total (`Max_Ship_Hits_`, `Get_Design_Structure_`) | 4, 15, 25, 40, 80, 300 |

Other fields are not decoded here.

## Armour table (0x17f63e, 0x0f-byte rows, index = armour class)

| Off | Size | Meaning | Values (class 0..6) |
|---|---|---|---|
| 0x00 | i16 | tech application that gives the armour (`Best_Armor_`) | 0, 187, 191, 203, 117, 2, 201 |
| 0x04 | i16 | percent added to the hull's base hits | −100, 0, 100, 300, 500, 700, 900 |
| 0x09 | u32 | pointer, 0x178a04 in every row **?** | |

Classes 1..6 are titanium, tritanium, zortrium, neutronium, adamantium,
xentronium **?** (from the percentages).

## Checks

`internal/game/shipstats_test.go`: hand-computed values from the tables
(frigate/titanium 4+4, battleship/zortrium 200+200, reinforced and heavy
×3, Hydra 1500, strategic Antaran doom star 300·10 = 3000), and over every
non-free ship of the ten saves in both settings: `0 < current ≤ max` and
max = structure + armour.
