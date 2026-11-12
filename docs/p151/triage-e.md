# Group E triage (rules): proposal

Group E of `items.md`: the changelog's game changes still `[?]`, sorted
by what to do with them. Line numbers are `items.md`'s. Nothing is marked
until the maintainer agrees.

## E1. Deferred after the crash triage (unconfirmed or set aside): port later (11)

- 22 (1.51): Winning an election and rejecting the ruling no longer crashes the game.
- 262 (1.50.19): Fixed artemis system net causing desync in networked tactical combat.
- 706 (1.50.11): Fixed last ship shuttle-captured stall in multiplayer.
- 973 (1.50.7): Fixed division by zero crash when damaging internals of a ship with no systems left but drive with 1 hp.
- 976 (1.50.7): Fixed a crash when ship is destroyed by BHG and ship initiative is off.
- 990 (1.50.7): Fixed a crash in fleet leaders screen when scrolling fleet pane.
- 1232 (1.50.4): Ship captured by shuttles when rotating will not cause desync in network games.
- 1387 (1.50.2): Cluster galaxy now works in multiplayer.
- 1464 (1.50.1): Map edge retreat crash fixed.
- 1474 (1.50.1): Diplomatic relation change overflow fixed.
- 1566 (1.50.0): Evolutionary Mutation desync in combat fixed.

## E2. The patch's own business (its features, DOS, config): n/a (10)

- 26 (1.51): Restored broken command line switches.
- 40 (1.50.26): Fixed extra boarding from AI and AUTO combat for initiative 2.0.
- 346 (1.50.18.2): In Hotseat, it's allowed to reload config and use ALT-x.
- 366 (1.50.18.1): Fixed inability to start in pure DOS or some custom DOSBox setups.
- 529 (1.50.15): Fixed parameter satellite_size_class_plus_one giving Star Fortress Beam Attack bonus to the defending fleet at an outpost or at a colony without a starbase. Also fixed satellite repair (1.50.14 bugs).
- 977 (1.50.7): Enabling strict_combat_sequencing now forces ship initiative.
- 1146 (1.50.5): Enabling strict_combat_sequencing now forces ship initiative in combat.
- 1209 (1.50.4.1): TWF fixups for combat with ship initiative off.
- 1397 (1.50.2): Hard Shields now increase blocked damage in strategic combat.
- 1412 (1.50.2): Zeroth weapon flag usable.

## E3. Changed by the patch and changed back: n/a (14)

- 267 (1.50.19): Made 1.50.1 patch "Improved spy formula ..." optional.
- 268 (1.50.19): Made 1.50.2 patch "Bombardment can destroy capitol" optional.
- 380 (1.50.18): Strategic combat no longer honors crew level bonus for BA/BD/MD (reverted 1.50.9 patch back to classic and created parameter).
- 574 (1.50.14): Marines above ship capacity will no longer die immediately after ship capture (1.50.4 change restored to classic).
- 778 (1.50.10): Scout Lab & Galactic Lore no longer provide bonus against Antarans (reverted back to classic).
- 779 (1.50.10): Warp Interdictor range reverted back to classic 3 parsecs.
- 821 (1.50.9): Strategic combat now honors crew level bonus for BA/BD/MD.
- 1253 (1.50.4): Marines above ship capacity will now die after ship capture.
- 1344 (1.50.3): Reverted telepaths can capture ships multiple times without losing control (1.50.2 change).
- 1393 (1.50.2): Bombardment can destroy capitol.
- 1409 (1.50.2): Scout lab and Galactic Lore now give bonus against antarans.
- 1410 (1.50.2): Telepaths can capture ships multiple times without losing control.
- 1475 (1.50.1): Improved spy formula (63rd spy receives additional +1 bonus).
- 1576 (1.50.0): Warp interdictor range now corresponds to its description.

## E4. AUTO build, decided against: skip (2)

- 25 (1.51): Adjusted colony building scores for AI and AUTO build.
- 710 (1.50.11): Colony is not recalculated in autobuild anymore, so newly grown pop doesn't participate in production when autobuild is on.

## E5. Only matters for mods: later, with mods (9)

- 935 (1.50.7.2): Improved AI assessment of modded Fighter Garrison strength.
- 936 (1.50.7.2): Improved AI assessment of modded Hard Shields strength.
- 981 (1.50.7): Fixed no sound crash if player has no rifle upon ground invasion (only in modded games).
- 1079 (1.50.6): Behold the arti fixed for single and multi player; renamed parameter to finds_presented_by_scientist.
- 1080 (1.50.6): Strength of a modded missile base or ground batteries is now properly assessed by AI.
- 1081 (1.50.6): Added support for ESD custom ratio in determine_beam_weapon_modifications.
- 1084 (1.50.6): Modded leader XP levels no longer cause fired leader to get wrong XP.
- 1208 (1.50.4.1): Uncreative will not lose techs in modded greybox field now.
- 1210 (1.50.4.1): Some inherent mods now work for missiles.

## E6. Ghost and zero colonies (changed back and forth): decide on its own (4)

- 438 (1.50.17): Inbound colonists no longer prevent the loss of a zero colony (reverted 1.50.15 change, behavior can still be tuned with zero_colony_lost).
- 526 (1.50.15): A zero colony is no longer lost when colonists are inbound (reverted back to classic).
- 1391 (1.50.2): Ghost colony can die out even if colonists are inbound.
- 1392 (1.50.2): Starvation can create ghost colonies.

## E7. Balance and rule changes outside combat: maintainer decides (24)

- 127 (1.50.23): Fixed multiple bugs with 'ensure at least one food world' logic.
- 170 (1.50.22): Excluded the Orion system from trim_planets (which is used when mapgen generates more than 250 satellites in a galaxy).
- 439 (1.50.17): Antaran offensive Titan gains fighters.
- 573 (1.50.14): Fighter bays are allowed in post-warp by default.
- 640 (1.50.12): Randomized order in which players roll for leaders.
- 776 (1.50.10): Reduced satellite space compensation for beam arc and missile mods from +40% to +33.3%.
- 816 (1.50.9): Mapgen improvements: • Minimum 3 planets in home systems as intended instead of 2 planets in version 1.40 and 1 in 1.31. • The switch /planets now also accepts 1. • Fixed mapgen generating non-standard systems in some cases. • Fixed issues with trim_planets (which is used when mapgen generates more than 250 satellites in a galaxy). • Wormholes will no longer be placed to the Orion system.
- 882 (1.50.8): Changed freighters_cash_bonus default from 5 to 0 BC so building a freighter fleet no longer generates free cash.
- 884 (1.50.8): Static cheats will no longer persist when starting or loading a game.
- 934 (1.50.7.2): Satellite missile amount distribution: 1-3 in 1 slot; 4 in 2 slots; higher amounts in 3 slots.
- 966 (1.50.7): Satellite and ground defenses auto design reworked. • List of weapon mods available is now configurable. • Space allocation per weapon type is now configurable. • Weapon mods now cost space. • Missiles spread evenly over slots, e.g. 2 2 2 instead of 6 0 0. • Up to 99 missiles per stack instead of 30. • Corrected arc cost for beams in satellites (was +25%) and ground batteries (was +0). • Ground batteries space increased by +50% for the corrected arc cost. • Satellite space increased by +40% for the corrected arc and missile mods cost.
- 984 (1.50.7): Strategic instead of tactical weapon damage values are now considered for weapon selection in strategic mode.
- 985 (1.50.7): Strategic damage values column updated with tactical values to preserve classic behavior, except for special weapons that were bugged or incorrect and received new values.
- 987 (1.50.7): Monsters now consider systems in +10pc wider range for finding new target colony.
- 1058 (1.50.6): Removed best armor minus 1 rule for lander ships, now they have the best armor available.
- 1060 (1.50.6): Newly built Warlord ships get crew level bonus from the turn of build.
- 1228 (1.50.4): Removed Advanced damage control from tree for cybernetic races.
- 1337 (1.50.3): Building & spy no longer give free cash on-build like in 1.40.
- 1340 (1.50.3): MENLO now gives 1 million research points.
- 1394 (1.50.2): Auto-designed ships are equivalent to manually designed.
- 1395 (1.50.2): Satellite generation and repair uses systems size from correct hull table entries 6-8.
- 1569 (1.50.0): Strategic tech tree corrections.
- 1575 (1.50.0): Discrepancy of communications and scanners fixed.
- 1584 (1.50.0): Disabled LB's /nohousing +150 growth bonus.

## E8. Combat, fixes and rule changes: a block of its own (114)

- 24 (1.51): Fixed Bombs and A-M Rockets hit points in tactical combat.
- 86 (1.50.24): Ships now lose multi-phased shields strength if it is destroyed.
- 168 (1.50.22): Fixed occasional boarding of illegal targets by AI and AUTO combat.
- 169 (1.50.22): Combat is stopped if the remaining side is on AUTO combat, even if it can still bomb planets or shoot missiles, so ships won't move aimlessly.
- 214 (1.50.21): AI ships with a leader onboard no longer retreat when out of ammo.
- 269 (1.50.19): One dimensional portal buildable per system, like artemis system net.
- 270 (1.50.19): Fighter garrison strength calculation now accounts for bomber bays.
- 271 (1.50.19): Monster now always goes first in case of tied ship initiative.
- 272 (1.50.19): Energy absorber can now fire at 24 squares instead of 21.
- 273 (1.50.19): When a ship's drive explodes, and leader is onboard, 'nooo' exclamation can be heard (classic was not dependent on leader, just a 10% chance).
- 331 (1.50.18.3): Energy Absorber maximum stored damage is 32767 (was 30k since 1.50.4).
- 378 (1.50.18): Fixed buggy capitol check when homeworld has been bombed or destroyed. Now capitol can be rebuilt immediately.
- 708 (1.50.11): Restored planet sizes for bombardment to classic 3-4-6-7-8.
- 709 (1.50.11): In strategic mode ships now upgrade to the highest available missile/torpedo (was broken since 1.50.7)
- 772 (1.50.10): Combat AI is now aware that Ion Cannon has no effect against planets.
- 773 (1.50.10): Undocumented +1 hit bonus for civilian buildings during bombardment removed.
- 777 (1.50.10): Stealth Field is now mutually exclusive with Cloaking Device and Phasing Cloak.
- 818 (1.50.9): Lightning_field default setting is no longer 'uniform' but 'classic'.
- 822 (1.50.9): Fixed bomb hits calculation for orbital bombardment: • Bomb weapons now get bomb hits equivalent to 10 instead of 5 attacks. • Fighters get the equivalence of 1 strike instead of 0.
- 876 (1.50.8): Improved Combat AI (also used for final and defensive fire), now considers the following: • Damper Field, Energy Absorber, Hard Shields, • Ordnance skill, • Nebula nullifies ship shields, • Planetary shields block the Shield Piercing mod, • Bomb damage against a planet is reduced by ship shields too, • MIRV missile damage versus shields, • Custom bonus for cloaking device, both evasion and jamming modes.
- 877 (1.50.8): AUTO combat AI will no longer retreat ships just because they are out of ammo.
- 878 (1.50.8): Defensive Fire is always active in non-initiative games regardless of DF setting.
- 879 (1.50.8): Turned off (reddened) PD beams and A-M Rockets will now fire during Seeking missiles phase; reddened Spatial Compressor still will not.
- 880 (1.50.8): Fixed Avenger, Antaran Battleship & Titan having bombs with FWD instead of 360 arcs.
- 951 (1.50.7.1): Autodesign won't install heavy mod on beams without heavy mod.
- 970 (1.50.7): Artemis system net built/removed wakes up orbiting fleets now.
- 974 (1.50.7): Ship captured on retreat by Assault Shuttles now stays in battle.
- 975 (1.50.7): AI won't self-destruct ship immobilized by BHG to avoid capture if boarding in black hole is disabled.
- 982 (1.50.7): AI now better assesses strength of ships with BHGs.
- 1056 (1.50.6): Added possibility to acquire a shield tech from a scrapped ship.
- 1059 (1.50.6): Ships with no computer fitted now won't have computer's hit points.
- 1061 (1.50.6): Building a Fighter Garrison now wakes up orbiting ships, in line with other planetary defenses.
- 1062 (1.50.6): Building a planetary shield won't wake up orbiting fleet now.
- 1066 (1.50.6): Fighters now benefit from helmsman.
- 1067 (1.50.6): Fixed buggy fighter counts after returning to the launching ship.
- 1068 (1.50.6): Partial fighter squadrons that return to ship do not cause the loss of weapon bays after the battle.
- 1069 (1.50.6): Corrected Absorber's miss chance due to Displacement Device to 30% (70%).
- 1070 (1.50.6): Beams Displacement parameter now also affects Absorber's chance to miss.
- 1071 (1.50.6): BHG and Stasis Field now free ships correctly when the weapons themselves are destroyed.
- 1073 (1.50.6): Proper visuals in Raid message, now a damaged special is reported as damaged, instead of destroyed, also misleading number of reported hits no longer appear.
- 1074 (1.50.6): Combat ends immediately if the losing side has no missiles or planet.
- 1077 (1.50.6): Amoeba applies its toxic ability to AI colonies too.
- 1083 (1.50.6): Disabled (reddened) point defense weapon does not automatically fire at incoming missiles anymore.
- 1138 (1.50.5): Fixed "leader capture" bug, a leader en route to a captured ship is put to officers pool.
- 1140 (1.50.5): Fixed lingering missile alerts (most common case is a leftover from AI turn).
- 1141 (1.50.5): Fighters can now select any ship as a new target after destroying a missile.
- 1142 (1.50.5): Fixed erroneous overmap display of Captured label.
- 1145 (1.50.5): Shield will not be installed on a starbase if damper field is present.
- 1229 (1.50.4): Ship initiative rule now takes BA differences below +10 into account.
- 1230 (1.50.4): Ships tied by initiative are now sorted by crew experience.
- 1233 (1.50.4): Super-fast combat mode (Z) now takes ship explosions into account.
- 1234 (1.50.4): PD and Spatial Compressor range is now calculated correctly mid flight.
- 1235 (1.50.4): Fixed discrepancy between targeting and damaging ranges for spherical weapons.
- 1236 (1.50.4): Stasis disables wide area jammer, warp dissipator, scout lab.
- 1237 (1.50.4): Ship under stasis now ceases to be affected by tractors and BHGs.
- 1238 (1.50.4): Ship under stasis now ceases to affect others by tractors, BHGs, stasis.
- 1239 (1.50.4): Fixed a number of bugs in defensive fire.
- 1240 (1.50.4): All retreats now happen after all ships have recharged weapons.
- 1241 (1.50.4): Repaired warp dissipator now prevents retreats ordered on previous turn.
- 1242 (1.50.4): Missile alerts will now be triggered by in place rotations.
- 1243 (1.50.4): Correct missile alert halt on move when ship is going to be hit while turning.
- 1244 (1.50.4): Ship with inertial nullifier does not pull missiles when rotating.
- 1245 (1.50.4): Fighters chasing missiles with depleted moves will now move at the end of turn.
- 1246 (1.50.4): Ordnance bonus now works for fighters.
- 1247 (1.50.4): Non-TWF ships now update at the end of TWF turn.
- 1248 (1.50.4): Non-TWF ships do not perform defensive fire in TWF turn.
- 1249 (1.50.4): Fixed bug which allowed to capture satellite sometimes.
- 1250 (1.50.4): Ship capture removes wait and done states.
- 1251 (1.50.4): Fixed integer overflow for Energy Absorber's stored damage.
- 1252 (1.50.4): Different cloaks now work correctly if put on a ship together.
- 1254 (1.50.4): Equal marines and armor limits now apply to colonies with and without Barracks.
- 1255 (1.50.4): The last marine and armor units of a Subterranean colony no longer die at end of turn when the maximum has been reached.
- 1311 (1.50.3.1): Fixed rare ship array overflow related to capture.
- 1325 (1.50.3): Two ships now can destroy each other with BHG simultaneously.
- 1326 (1.50.3): AI now honors board_in_black_hole parameter.
- 1327 (1.50.3): Tractors (both incoming and outgoing) break when ship phase cloaks.
- 1328 (1.50.3): Stasis field recharges correctly now.
- 1329 (1.50.3): Removing stasis field does not zero ship's shots.
- 1330 (1.50.3): Ship which started turn in stasis gets a chance to act this turn if freed.
- 1331 (1.50.3): Fixed a bug when fighters chase unrelated friendly ships.
- 1332 (1.50.3): Fighters now highlight when targeting missiles that target current ship.
- 1333 (1.50.3): Move blocked by missile alert will no longer trigger dogfights.
- 1334 (1.50.3): Ships now decloak if cloak-inducing device is broken.
- 1335 (1.50.3): Specials repaired in combat resume operation (in classic they don't).
- 1336 (1.50.3): Fixed bug in drive repair after combat, now ship is always made mobile.
- 1386 (1.50.2): Planet bombing is now symmetrical, enabling bombing all around any world.
- 1390 (1.50.2): Ghost colony can be bombed or captured.
- 1398 (1.50.2): Fixed Sub-Space Teleporter getting increased range for faster ships.
- 1399 (1.50.2): Fixed Sub-Space Teleporter not breaking tractor beam out of range.
- 1400 (1.50.2): Ship now loses shot right after weapon is destroyed.
- 1401 (1.50.2): Fixed discrepancy between 'immobile' label and real immobility after BHG or tractor beam.
- 1402 (1.50.2): Yellowing or reddening tractor beam slot no longer disables it next turn.
- 1403 (1.50.2): Multiple BHGs and Stellars installed in one slot now fire one by one.
- 1404 (1.50.2): Ship cloaked by Phasing Cloak is not affected by area damage now.
- 1405 (1.50.2): Ship cloaked by Phasing Cloak can't be boarded.
- 1406 (1.50.2): Lightning field buggy roll fixed, now kills 1/2 missiles instead of 1/3.
- 1407 (1.50.2): Fixed Antaran fighters not working on defensive Cruiser and Titan.
- 1462 (1.50.1): Relaxed end of combat (ships act until end of turn).
- 1463 (1.50.1): TWF-equipped ship's presence doesn't affect normal ships now.
- 1472 (1.50.1): Stealth specials now work properly.
- 1553 (1.50.0): No auto-done in combat for ship without shots.
- 1554 (1.50.0): Colony +10 beam attack bonus bug fixed.
- 1555 (1.50.0): Fighters strike immediately point blank left to right.
- 1556 (1.50.0): Plasma Torpedo instant dissipation bug fixed.
- 1557 (1.50.0): Plasma Torpedo correct dissipation on resolve.
- 1558 (1.50.0): Crew experience level now also applies to capture actions.
- 1559 (1.50.0): Boarding in stasis is no longer possible.
- 1560 (1.50.0): Attackers' raiding party bonus is displayed again (1.3 bug).*
- 1561 (1.50.0): Raid can now destroy last ship system.
- 1562 (1.50.0): Combat repair fixes weapons now.
- 1563 (1.50.0): Repaired shield generator charges shield.
- 1564 (1.50.0): Undocumented +1 hits bonus for buildings during bombardment removed.*
- 1570 (1.50.0): Artemis strategic one kill to win bug fixed.
- 1583 (1.50.0): AI fighter size cheat fixed.

## E9. Bug fixes outside combat: port (63)

- 23 (1.51): You can no longer build ship designs that use up too much space.
- 53 (1.50.25): Fix Bulrathi (v1) and Elerian (v3) mutation on impossible difficulty.
- 87 (1.50.24): Galactic Lore does not reveal planet defenses.
- 88 (1.50.24): Loknar can no longer give tech_0 (no tech).
- 379 (1.50.18): Fixed bug in nebula check.
- 486 (1.50.16): Food Replicator can no longer cause negative production.
- 639 (1.50.12): Map knowledge is now transferred on surrender.
- 769 (1.50.10): Antaran ship costs and limits now persist after reloading the game.
- 770 (1.50.10): Fixed AI ships sometimes getting double Augmented Engines +5 speed bonus.
- 774 (1.50.10): Unstable wormhole event is now also allowed for a fleet with eta=2 turns.
- 817 (1.50.9): Fixed actual tech cost of first level Hyper-Advanced from 15k to 25k.
- 883 (1.50.8): Human-to-human tech trade ignores AI's trade decisions, all tech can now be traded.
- 937 (1.50.7.2): Fixed a bug when GNN reports 0 RP to end the plague and prolongs it.
- 967 (1.50.7): Leaving/catching up/levelling up of navigator affects fleets immediately.
- 968 (1.50.7): Telepath or Telepath* leader now protects colonies from Mind Control.
- 969 (1.50.7): Assassin leader no longer protects colonies from Mind Control.
- 971 (1.50.7): Fixed irregularities in ship placement in case of large fleets.
- 978 (1.50.7): Comet now counts as destroyed as soon as 100% of it is destroyed, not the turn after as in classic.
- 979 (1.50.7): Ship scrap total won't overflow. Scrapping ships for total sum of more than ~32k BC overflows in classic.
- 983 (1.50.7): Nulled interceptors tactical damage value (6-15) as AI use separate calculation function for them.
- 986 (1.50.7): GNN news corrected for monster events, colony names and (English) textual errors.
- 989 (1.50.7): Fixed auto-scrap scrapping more ships than needed to pay upkeep.
- 1054 (1.50.6): Fuel range for contact is now consistent with the ability to visit colonies.
- 1055 (1.50.6): Fixed received techs from surrender being only up to #83 from tech list.
- 1057 (1.50.6): Fixed blank "You received the following techs from scrapping this ship" message popups on scrapping lander ships.
- 1063 (1.50.6): Destroying or selling a satellite or ground defense building will now wake orbiting fleets.
- 1064 (1.50.6): Fixed right mouse button stuck when loading pre-battle autosave in hotseat.
- 1072 (1.50.6): Fixed ship special becoming unbreakable when dealt damage equal to its HP.
- 1075 (1.50.6): Fixed a range of GNN monster reports not showing up.
- 1076 (1.50.6): Amoeba stays in game if its target is destroyed before it arrives there.
- 1078 (1.50.6): Fixed hyperspace flux check for comet and monsters so they can't happen during a flux now.
- 1137 (1.50.5): Pirates will no longer seize negative amount of money from richer empires.
- 1139 (1.50.5): Breakthrough & serendipity in one turn won't cause blank box in tech choice.
- 1143 (1.50.5): Human players cannot build spies now without spy network technology.
- 1144 (1.50.5): Capitol can't be built now without capitol building technology.
- 1207 (1.50.4.1): Initial autosave now happens after the mapgen.
- 1321 (1.50.3): Fixed incompatibility with classic's save game format for ships with Augmented Engines.
- 1323 (1.50.3): Fixed Antaran star fortress repair issues.
- 1324 (1.50.3): Fixed spy threshold below 60 not working. Negative thresholds are forbidden.
- 1338 (1.50.3): Fixed AI bought prod appearing as non-zero prod after capturing colony.
- 1339 (1.50.3): RPs produced on turn 0 are not lost even if you did not choose tech yet.
- 1380 (1.50.2.1): Fixed bug causing absence of Unification Production bonus.
- 1388 (1.50.2): Fixed inability to move pop from non-food world to farming on food world.
- 1389 (1.50.2): Using other player's dimensional portal is now prohibited.
- 1413 (1.50.2): No place for a new spy exploit fixed.
- 1465 (1.50.1): Population capacity is clipped to 42 (maximum properly supported by the game).
- 1466 (1.50.1): 43rd android bug fixed.
- 1467 (1.50.1): Pop limit bug for freighted colonists fixed.
- 1468 (1.50.1): Command points not updating when landing colony or outpost ship fixed.
- 1469 (1.50.1): Productivity recalculation after selling a building now works.
- 1470 (1.50.1): Refit displayed cost versus real cost discrepancy fixed.
- 1471 (1.50.1): Feudal wrong refit cost fixed.
- 1473 (1.50.1): Diplomatic blunder and marriage events not working fixed.
- 1571 (1.50.0): Artifacts on-click research bonus bug fixed.
- 1572 (1.50.0): Ships can now be refitted if Cold Fusion tech field is not known.
- 1573 (1.50.0): Outpost ship conflicts colonization bug fixed.
- 1574 (1.50.0): Colony ship conflicts outpost creation bug fixed.
- 1577 (1.50.0): Evolutionary Mutation unclickable picks bug fixed.*
- 1578 (1.50.0): Tolerant races maintenance penalty display fixed.
- 1579 (1.50.0): Navigator skill display bug fixed.*
- 1580 (1.50.0): Telepathic leader's skill level bonus corrected.*
- 1581 (1.50.0): Ordinary assassin skill not working fixed.*
- 1582 (1.50.0): AI personalities racial traits broken logic fixed.
