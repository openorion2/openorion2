# Community patch 1.51: item by item

Every line of the patch's `CHANGELOG_150.TXT` (1.50.0 - 1.51) and the
"1.50 standard" settings from `150.CFG`, each with what the port does
about it. Overview and decisions: `inventory.md`.

Status, at the start of each line:

- `[?]` undecided
- `[port]` to port
- `[skip]` not ported
- `[done]` ported (name the commit or doc after the text when useful)
- `[n/a]` nothing to port: our port already behaves so, or the patch
  broke it itself and fixed it later (say which after the text)

A note after the text goes after ` -- `.

## 1.51

### Game changes

- `[?]` Winning an election and rejecting the ruling no longer crashes the game.
- `[?]` You can no longer build ship designs that use up too much space.
- `[?]` Fixed Bombs and A-M Rockets hit points in tactical combat.
- `[?]` Adjusted colony building scores for AI and AUTO build.
- `[?]` Restored broken command line switches.

### New config parameters

- `[?]` building_score, sets values of build priority for AI and AUTO build.

### Mods

- `[?]` Updated 1.50 improved.

## 1.50.26

### Game changes

- `[?]` Fixed extra boarding from AI and AUTO combat for initiative 2.0.
- `[?]` Proper fix for Warlord ships getting +1 level in combat after refit. (replaced prior 1.50.19 fix for 1.50.6 bug)

### Mods

- `[?]` Updated 1.50 improved.
- `[?]` Updated ICE to version 35.
- `[?]` Updated VDC to version X12-V72-B10.

## 1.50.25

### Game changes

- `[?]` Fix Bulrathi (v1) and Elerian (v3) mutation on impossible difficulty.

### Interface changes

- `[?]` F9 shows star distance if black hole is in between two stars.
- `[?]` Race specific Doom Star name is shown in Build screen.
- `[?]` Avenger and Antarans correct weapon arcs shown in Fleets screen.
- `[?]` Fix for broken custom tech names after new game (1.50.19 issue).

### Config changes

- `[?]` In networked multiplayer, interface parameters are now also synced.

### Changed config parameters

- `[?]` loknar_gives_no_tech now works properly (1.50.19 bug).

### Lua scripting

- `[?]` set_planets: food_base is now updated after evaluating climate.
- `[?]` set_planets: when planet turns into asteroid or gas giant, outpost is no longer destroyed (colony still is).

### Mods

- `[?]` Updated 1.50 improved.
- `[?]` Updated ICE to version 33.
- `[?]` Updated VDC to version X12-V72-B5.

## 1.50.24

### Game changes

- `[?]` Freighted pop info is no longer lost when loading older saves, restoring backwards compatibility with 1.31, 1.40 and 1.50 saves (1.50.18 bug).
- `[?]` Ships now lose multi-phased shields strength if it is destroyed.
- `[?]` Galactic Lore does not reveal planet defenses.
- `[?]` Loknar can no longer give tech_0 (no tech).

### Interface changes

- `[?]` With new hotkey 'h' in combat, player let's ALL his ships wait at once. In classic ship initiative, if both parties use 'h', defender moves last.
- `[?]` Ship Officer Ruola is shown in Refit Ship popup.
- `[?]` Population graph includes freighted population.

### New config parameters

- `[?]` hit_threshold_fixed_value, enables changing fixed value of beam formula.
- `[?]` ion_affects_antarans, allows firing ion cannon at Antaran ships.
- `[?]` eel_spawn_frequency, set frequency of wandering Eel producing offspring.
- `[?]` max_eels, sets max amount of wandering Eels in the galaxy.

### Changed config parameters

- `[?]` mapgen -mterraform, repurposed; it now terraforms monster guarded deserts and tundras with natives into swamp climate.
- `[?]` mapgen -mgrav, repurposed; it now adjusts guarded Rich and Urich planets to follow classic gravity table.
- `[?]` mapgen -mmonst, now upgrades guarded planets to Terran (was Tundra).
- `[?]` planet_rangemaster, planet_hef, planet_analyzer and planet_achilles now only fit these specials on combat planet if Ground Batteries or Stellar Converter is present.

### Lua scripting

- `[?]` Added set_player_fields.
- `[?]` Added stardate and galaxy_age to get_game.
- `[?]` Improved alignment of script texts.

### Mods

- `[?]` Updated 1.50 improved.
- `[?]` Updated ICE to version 32.
- `[?]` Updated VDC to version X12_V69_B4.
- `[?]` Removed Orange map.

## 1.50.23

### Game changes

- `[?]` Fixed multiple bugs with 'ensure at least one food world' logic.
- `[?]` Fixed Absorber expected damage for AI and AUTO combat (1.50.8 bug).

### Interface changes

- `[?]` Import/export grains & pollution barrels no longer shown during anomaly.

### New config parameters

- `[?]` star_special_chance, sets chance value to generate a special at a star.
- `[?]` anim_speed_cloak, sets animation speed for cloaking/decloaking in combat.
- `[?]` anti_missile_rocket_chance_to_hit, configures A-M Rocket chance to hit.

### Changed config parameters

- `[?]` /noreport now forbids offer tech in diplomacy.

### Lua scripting

- `[?]` Added get_colonies, get_pop_info_of_colony, get_buildings_of_colony, get_build_queue_of_colony, get_queued_ships, get_leaders, get_active_combat_ship, randomize_star, upgrade_star_guarded_by_monster, get_coordinate_distance_of_stars, get_memory_used, get_milliseconds.
- `[?]` Added population in transit to get_players.
- `[?]` Extracted get_combat_ship_weapons and -specials from get_combat_ships and fixed a crash related to planetary shields. Scope is now universal.
- `[?]` Fixed del_planets not properly clearning their values.
- `[?]` Main1.lua now emits population in transit report.
- `[?]` Moved tech differential report to Main3.lua, it now sorts by RP by default.
- `[?]` Moved exploration report to Main9.lua.
- `[?]` Main8.lua now emits ship damage report when in tactical combat.
- `[?]` Fixed a memory issue with Main0.lua and added listing of postgame scripts.

### Mods

- `[?]` Updated 1.50 improved.
- `[?]` Updated 1.50 multiplayer.
- `[?]` Updated ICE to version 31.

## 1.50.22

### Game changes

- `[n/a]` Fixed large fleet (>99 ships) tactical combat network freeze. -- long messages go over TCP whole (docs/net/combat.md)
- `[n/a]` Fixed an overflow crash in large (>127 ships) tactical combat. -- a battle has at most 200 records and our arrays hold 210 (TestCombatRecordLimit)
- `[?]` Fixed occasional boarding of illegal targets by AI and AUTO combat.
- `[?]` Combat is stopped if the remaining side is on AUTO combat, even if it can still bomb planets or shoot missiles, so ships won't move aimlessly.
- `[?]` Excluded the Orion system from trim_planets (which is used when mapgen generates more than 250 satellites in a galaxy).

### Interface changes

- `[?]` Freighted pop is now printed in the Colonies screen.
- `[?]` Assassin turn summary report states race name instead of ruler name.

### New config parameters

- `[?]` print_research_value, sets type research value shown in Colonies screen.
- `[?]` antaran_attack_chance_lucky_divisor, sets chance reduction for Lucky race to become Antaran target.
- `[?]` satellite_ship_class, sets minimum ship class that requires a star base to be built.

### Changed config parameters

- `[?]` Renamed mineral_to_gravity to planet_size_to_gravity.
- `[?]` Fixed Shield table inverted cost and space column labels.
- `[?]` Fixed leader_data advanced_government_leader_id.

### Mods

- `[?]` Updated 1.50 improved.
- `[?]` Updated 1.50 multiplayer.
- `[?]` Updated GoodMap3 and GoodMap4.
- `[?]` Added back GoodMap5.
- `[?]` Added flavor of mirror mod which mirrors only the home systems.

## 1.50.21.2

### Interface changes

- `[?]` Colonies screen: Fixed incorrect production info for cybernetic races in the Colony Info Summary window (1.50.15 issue; prior 1.50.18.3 fix was lost in 1.50.19).

## 1.50.21.1

### Game changes

- `[?]` Fixed anti-missile weapons not always firing automatically (1.50.19 bug).

## 1.50.21

### Game changes

- `[done]` Fixed a crash involving the Crystal capturing a ship while missiles or fighters are present. -- docs/combat/tactical.md
- `[?]` AI ships with a leader onboard no longer retreat when out of ammo.

### Interface changes

- `[?]` Left-click on Freighter window now triggers correct message when no freighters may be scrapped.

### New config parameters

- `[?]` scan_bonus scout_lab, sets overmap scan bonus for scout lab.
- `[?]` wait_movement_cost, sets movement points lost each time ship waits.

### Mods

- `[?]` Updated 1.50 improved.
- `[?]` Updated 1.50 multiplayer.
- `[?]` Updated ICE to version 30.
- `[?]` Updated VDC to version X11_V68_B3.

## 1.50.20

### Interface changes

- `[?]` Food shortage shown for colonies with no food production and no imports.

### Changed config parameters

- `[?]` Parameter -sarti now works properly on mirrored maps.

### New config parameters

- `[?]` send_native, allows to send natives to another colony.
- `[?]` ai_planet_value, AI colonization valuation of planets with a special.

### Lua scripting

- `[?]` Main3.lua simplified exploration report.
- `[?]` Main4.lua removed miniaturization status report.

### Mods

- `[?]` Updated 1.50 improved.
- `[?]` Updated 1.50 multiplayer.
- `[?]` Updated ICE to version 28.

## 1.50.19

### Game changes

- `[?]` Fixed artemis system net causing desync in networked tactical combat.
- `[n/a]` Fixed tractor crash in big battles (1.50.2 bug). -- a 1.50.2 bug, fixed later
- `[?]` Removed bugged hard shields patch for strategic combat (1.50.2 issue).
- `[?]` Fixed warlord ship get +1 level in combat after refit (1.50.6 bug).
- `[?]` Fixed PD weapons not firing automatically in some cases (1.50.8 bug).
- `[?]` Made 1.50.1 patch "Improved spy formula ..." optional.
- `[?]` Made 1.50.2 patch "Bombardment can destroy capitol" optional.
- `[?]` One dimensional portal buildable per system, like artemis system net.
- `[?]` Fighter garrison strength calculation now accounts for bomber bays.
- `[?]` Monster now always goes first in case of tied ship initiative.
- `[?]` Energy absorber can now fire at 24 squares instead of 21.
- `[?]` When a ship's drive explodes, and leader is onboard, 'nooo' exclamation can be heard (classic was not dependent on leader, just a 10% chance).

### Interface changes

- `[?]` Hyperspace beast is now displayed on main screen.
- `[?]` Dimensional portal icon has race name instead of ruler name.

### Config

- `[?]` alt-ri reloads interface options only and is allowed during MP games.

### New config parameters

- `[?]` Added starbg.lbx, design.lbx.
- `[?]` capitol_can_be_bombed, capitol building can be destroyed by bombardment.
- `[?]` n_spies_bonus, sets number of spies that get +2, +1 and +1/2 bonus.
- `[?]` xeno_psychology, sets spy bonus, making tech useful in MP games.
- `[?]` spy_random_bonus, sets random spy bonus (classic is 100).
- `[?]` spy_roll_random_success, sets attacker spy roll guaranteed success value.
- `[?]` telepathic_multiple_capture, allows telepath to control a ship in combat after re-capture from non-telepath opponent.
- `[?]` Added special slot for system and event Crystal.
- `[?]` missile_x3_shot, replaces x15 shot with x3 shot missile (AI will use it).
- `[?]` leader_dies_with_no, 'nooo' exclamation is also heard for ships destroyed by regular or quantum detonator explosion if leader is onboard.
- `[?]` marine_kill_damage_blocker, sets special that blocks marine kill damage.
- `[?]` loknar_gives_no_tech, no extra techs from taking Orion like in v1.1-1.2.
- `[?]` invasion_tech_chance, sets value for chance to get tech from invasion.
- `[?]` fixed_research_cost, removes random aspect from tech breakthrough.
- `[?]` keep_eotw_setting, prevents game type changing End Of Turn Wait setting.

### Changed config parameters

- `[?]` Parameter /noreport now works on mirrored maps. Removed the /noreport command line switch.
- `[?]` max_pop_freighted now works (1.50.18 bug).
- `[?]` If federation_no_mixed_races_penalty is enabled, alien control center is no longer buildable on colonies with mixed (assimilated) races.

### Lua scripting

- `[?]` Changed field_id16 to just field. Lua mods that use it require an update.
- `[?]` Added spies to get_player.
- `[?]` Added get_player_fields.
- `[?]` Added get_hyper_advanced_progress.
- `[?]` Main2.lua emits spying report.
- `[?]` Main3.lua emits exploration report.
- `[?]` Main4.lua emits miniaturization status report.
- `[?]` Main0.lua emits technical game info.

### Mods

- `[?]` Updated 1.50 improved.
- `[?]` Removed 1.50 improved + picks (150ip).
- `[?]` Added 1.50 multiplayer (150m).
- `[?]` Added 1.50 multiplayer map.
- `[?]` Updated VDC to version X9_V66_B11.

## 1.50.18.3

### Game changes

- `[?]` Energy Absorber maximum stored damage is 32767 (was 30k since 1.50.4).

### Interface changes

- `[?]` Colonies screen: Fixed incorrect production info for cybernetic races in the Colony Info Summary window (1.50.15 issue).

### Mods

- `[?]` Updated 1.50 improved.
- `[?]` Updated VDC to version X9_V66_B3.

## 1.50.18.2

### Game changes

- `[?]` In Hotseat, it's allowed to reload config and use ALT-x.

### Interface changes

- `[?]` Unload Troops button now triggers correct message when no transport ships have been selected.

### Config

- `[?]` Config from save is now always loaded first.

### Mods

- `[?]` Updated 1.50 improved.
- `[?]` Updated ICE to version 27.
- `[?]` Updated VDC to version X9_V66_B2.

## 1.50.18.1

### Game changes

- `[?]` Fixed inability to start in pure DOS or some custom DOSBox setups.

### Mods

- `[?]` Added flavor of mirror mod which doesn't mirror home systems.
- `[?]` Updated VDC to version X9_V66_B1.

## 1.50.18

### Game changes

- `[done]` Fixed combat crash in network multiplayer game with AI. Occured when AI versus AI combat had been processed in the middle of human combat. -- docs/net/combat.md
- `[?]` Fixed buggy capitol check when homeworld has been bombed or destroyed. Now capitol can be rebuilt immediately.
- `[?]` Fixed bug in nebula check.
- `[?]` Strategic combat no longer honors crew level bonus for BA/BD/MD (reverted 1.50.9 patch back to classic and created parameter).

### Interface changes

- `[?]` Leader experience points are now shown in Leaders screen.
- `[?]` Fixed build outpost button appearing when travelling.
- `[?]` Better disable_vsync, reduces occurences of screen flashes.

### Config

- `[?]` Game configuration is now stored in save file. Every time the game is loaded full configuration is restored.
- `[?]` Changed how paths are handled in config, a\b\c is now a relative path.
- `[?]` Accumulative string parameters (needed for mod scripts).
- `[?]` newgame_postprocessor_script is now accumulative.
- `[?]` Tech tree designs leading to empty fields no longer crash the game.

### New config parameters

- `[?]` no_future_tech_peek, forbids seeing future techs in tech tree.
- `[?]` send_last_pop, allows to create ghost colony.
- `[?]` max_pop_freighted, sets max pop that can be in transit at one time.
- `[?]` housing_no_popagg, allows to change housing formula.
- `[?]` strategic_xp_bonus, makes strategic crew level bonus optional.
- `[?]` Extra weapon slot for both system and event Dragon.
- `[?]` Table satellite_orbit_chance, holds satellite chance per orbit for each galaxy age.

### Changed config parameters

- `[?]` growth_rate_bonus boom_event is now capped to 127.
- `[?]` Split galaxy size table into two levels.
- `[?]` Renamed group_to_climate_young_avg_gal to climate_chance_young_avg_gal.
- `[?]` Renamed group_to_climate_old_gal to climate_chance_old_gal.
- `[?]` Renamed planet_specials_chance to planet_special_chance.
- `[?]` Renamed gal_age_to_star_class to star_class_chance.
- `[?]` Renamed gravity_table to mineral_to_gravity.

### Lua scripting

- `[?]` In-nebula status of stars is now recalculated after scripts (1.50.7 bug).
- `[?]` Fixed zoom cur/max in get_game (1.50.8 bug).
- `[?]` Experimental rget/rset/radd/rdel API for leaders, nebulas, ships, players.
- `[?]` Added script_pregame for randomizing picks and techs.

### Mods

- `[?]` Added map mirroring mod.
- `[?]` Added Randomize Tech mod.
- `[?]` Updated 150 improved.
- `[?]` Updated ICE to version 26.
- `[?]` Removed Vanilla-MP mod.
- `[?]` Removed GM3_MP map.
- `[?]` Removed Fixed Square map.
- `[?]` Removed 'No AI surrenders' option in MOO2 Launcher.

## 1.50.17

### Game changes

- `[?]` Inbound colonists no longer prevent the loss of a zero colony (reverted 1.50.15 change, behavior can still be tuned with zero_colony_lost).
- `[?]` Antaran offensive Titan gains fighters.

### Interface changes

- `[?]` Redirecting out of a Jump Gate now works without warning dialog and redirecting back to Jump Gate path is possible until end of turn.
- `[done]` Loading a build list (e.g. with Q) now always updates income. Previously, negative income resulting from Q would not be displayed immediately which could lead to unexpected scrapping of stuff. -- docs/screens/build-lists.md
- `[done]` Renamed buy1-3 build list commands and added a new one: • buy75% - when product is 75% built (new), • buy50% - when product is 50% built (was buy1), • buy10% - when product is 10% built (was buy2), • buy - regardless of build progress (was buy3). -- docs/screens/build-lists.md

### Config

- `[?]` $LANG$, ./relative and ?optional paths are allowed in all lbx parameters, build.cfg, main.lua and newgame_postprocessor_script.

### New config parameters

- `[?]` prod_format, enables sorting and coloring of build list items.
- `[?]` font_color_group, defines up to 10 color schemes for build list coloring.
- `[?]` ship_initiative, enables full control of initiative from config, replacing strict_combat_sequencing parameter.
- `[?]` ground_commando_attacker_x2, allows lower multiplier from x5 to x2.
- `[?]` ground_commando_defender_x5, allows higher multiplier from x2 to x5.
- `[?]` ai_ship_design_bhg_chance, sets AI chance to build BHG ship.
- `[?]` ai_ship_design_stellar_chance, sets AI chance to build Stellar ship.
- `[?]` impossible_human_attack_chance_multiplier, tunes chance of antarans attacking human player on impossible difficulty.

### Changed config parameters

- `[?]` leaders_no_limit now takes into account joining of special leader Loknar.
- `[?]` ground_defense_armor_multiplier value range is 1-10000 instead of 0-127.
- `[?]` Added 2nd weapon to system Amoeba.
- `[?]` All monster weapons can now have mods.
- `[?]` Added 4th special for Antaran Cruiser and 6th special for Antaran Titan.
- `[?]` Renamed Antaran battle_ship to battleship.
- `[?]` Renamed deposites to deposits for both gold_ and gem_deposites.

### Lua scripting

- `[?]` Added planet and star special fields in scripts.

### Mods

- `[?]` Updated 150 improved.
- `[?]` Updated ICE to version 25.
- `[?]` Updated VDC to version X8_V65_B2.

## 1.50.16

### Game changes

- `[?]` Food Replicator can no longer cause negative production.
- `[?]` Fixed a case of fleet redirection resulting in too low travel speeds when Jump Gate is involved (1.50.7.1 bug).

### Interface changes

- `[?]` Artificial planet created from gas giant or asteroid with outpost, is now always depicted as a barren instead of a random climate.
- `[?]` Build screen: Removed junk info from repeat option.

### Config

- `[?]` USER.CFG now has all parameters commented out by default, giving more control to mod designer.

### New config parameters

- `[?]` replicator_food_pp, tunable production cost per food.
- `[?]` no_scanner in scan_bonus, sets base scan range separately from bonus for space scanner.
- `[?]` planet_achilles, enables achilles targeting unit on the combat planet.
- `[?]` planet_analyzer, enables structural analyzer on the combat planet.
- `[?]` planet_hef, enables high energy focus on the combat planet.
- `[?]` no_graph_smoothing, shows more detail in info graphs.
- `[?]` no_center_on_next_ship, camera focus in combat with ship initiative on behaves like non-initative combat.
- `[?]` death_spore_list_damage, lists damage value instead of 'special' in ship design dock.
- `[?]` bio_terminator_list_damage, same as above but for bio terminator.

### Changed config parameters

- `[?]` Renamed replicator_food_cost_no_bc to replicator_food_bc (tunable bc).
- `[?]` Renamed missile_ammo to missile_ammo_x2 (choice is only x2 or x5 now).

### Mods

- `[?]` Updated ICE to version 24.
- `[?]` Updated VDC to version X7_V65_B1.

## 1.50.15

### Game changes

- `[n/a]` Fixed freeze when no tech field is selected and serendipity event occurs. -- advancePlayerField already returns on field 0 (tech.go)
- `[n/a]` Fixed crash on WinXP related to proc_check_for_winner and proc_next_ship, which happened when retreating from combat involving monsters or Antarans (1.50.12 bug). -- a 1.50.12 bug, fixed later
- `[?]` A zero colony is no longer lost when colonists are inbound (reverted back to classic).
- `[?]` Fixed score calculation in games with modded picks (1.50.2 bug).
- `[?]` Fixed cluster not getting 71 but 54 stars (1.50.10 bug).
- `[?]` Fixed parameter satellite_size_class_plus_one giving Star Fortress Beam Attack bonus to the defending fleet at an outpost or at a colony without a starbase. Also fixed satellite repair (1.50.14 bugs).

### Interface changes

- `[done]` Colonies screen: Swapped around meaning of SHIFT for build shortcuts; SHIFT-X now acts on all colonies, X only on current colony. -- docs/screens/build-lists.md
- `[?]` Colonies screen: Added extra info about population and production to the Colony Info Summary window.
- `[?]` Planets screen: Fixed misalignment of the scrollbar with the planets list when using the filter buttons.
- `[?]` Build screen: Removed junk info from trade goods and housing options.

### Config changes

- `[?]` Json describing config parameters is now supplied with the pack.
- `[?]` Added support for relative paths in include command.
- `[?]` Weapon mod flags now have mask syntax, both for input and extraction.
- `[?]` Fixed tech apps not syncing correctly on startup of multiplayer games when client chose another mod than host and during on the fly reloads in single player games (1.50.1 bug).

### New config parameters

- `[?]` zero_colony_lost, causes a ghost colony without buildings but with colonists inbound to be lost immediately.
- `[?]` feudal_affects_freighters, causes feudal ship cost discount to apply to freighters.
- `[?]` double_ship_scrap_value, grants half of the original build cost value for scrap instead of classic one quarter.
- `[?]` ai_tech_prio, sets 2 tech fields and 2 tech apps that have higher priority for AI in research selection.
- `[?]` planet_rangemaster, enables rangemaster unit on the combat planet.

### Changed config parameters

- `[?]` Added slot_14 to starbase_special, and slot 6 now has 3 sub slots like slot 5.
- `[?]` Renamed stars_in_galaxy average to medium.
- `[?]` Renamed research_colony_bonus super_computer to supercomputer.
- `[?]` Renamed no_ship_designs_update_from_physics to no_designs_update_for_laser.

### Mods

- `[?]` Added Version 1.2 mod (set120).
- `[?]` Updated 150 improved.
- `[?]` Added 1.50 improved + picks (150ip).
- `[?]` Updated ICE to version 23.
- `[?]` Added Vanilla-MP by IR.
- `[?]` Added VDC X6_V64_B1 by IR.

## 1.50.14

### Game changes

- `[?]` Fighter bays are allowed in post-warp by default.
- `[?]` Marines above ship capacity will no longer die immediately after ship capture (1.50.4 change restored to classic).

### Interface changes

- `[?]` Enemy Moves in Settings menu now functions, showing red destination lines for all enemy fleets within scanning range.
- `[?]` Low-G planets in Planets screen show "-25% prod" for a Heavy-G race instead of "-50% prod".
- `[?]` New Leader Window_2 is now only shown on Tutor level. Removed parameter new_leader_popup2_diff_level.
- `[?]` Ships in fleets containing captured ships will no longer have incorrect pictures (1.50.12 bug).

### New config parameters

- `[?]` kill_excess_marines_after_capture, causes marines above capacity on a captured ship or base to die immediately instead of after combat.
- `[?]` amoeba_regeneration, enables the regeneration special. Regeneration percentage can be set in repair_rate.
- `[?]` transport_ship_extended_range, places extended tanks on transports.
- `[?]` satellite_size_class_plus_one, increases the size class of combat satellites by one.
- `[?]` recloak_next_turn, removes full turn wait for Cloaking Device ship to re-cloak.
- `[?]` rephase_cloak_next_turn, removes full turn wait for Phasing Cloak ship to re-cloak.
- `[?]` non_ics_phasing_cloak_mode, Phasing Cloak has BD/ME bonus immediately after decloaking.
- `[?]` non_ics_tractor_and_bhg, Tractor Beam and BHG insta-stop target, but do not instantly remobilize target.
- `[?]` no_outpost_prompt, skips the outpost creation prompt upon arrival at a star system.
- `[?]` first_pop_to_production, causes the first pop on a new colony to always become a worker.
- `[?]` new_farmer_food_threshold, sets base food per farmer value above which a new grown pop may be assigned to farming.
- `[?]` housing_creates_no_farmers, prevents a new grown pop on a housing colony from becoming a farmer.

### Changed config parameters

- `[?]` Under strict_combat_sequencing, combat AI should now auto-wait instead of auto-done when out of options.
- `[?]` Renamed satellite_can_use_torpedoes to satellite_uses_torpedoes.
- `[?]` Renamed pop_base to pop_max.
- `[?]` Renamed disable_physics_updates_ship_designs to no_ship_designs_update_from_physics.

### Mods

- `[?]` Updated 150 improved, PICKS.CFG and several mapmods.
- `[?]` Updated ICE to version 22.

## 1.50.13

### Game changes

- `[n/a]` Fixed crash in German language version when requesting opponent to break treaties with another player. -- the port runs English only

### Interface changes

- `[?]` In modded custom race screen allowing for >10 negative picks, government selection buttons no longer grey out when they shouldn't.

### New config parameters

- `[?]` screen_fade_speed, makes screen fade-in and fade-out speeds tunable.
- `[?]` govt_assimilation_points, sets assimilation speeds for governments.
- `[?]` aquatic_treats_tundra_as_swamp, sets tundra to be treated as swamp instead of as terran by aquatic races.

### Changed config parameters

- `[?]` Modded scanner missile evasion values now affect strategic combat and AI weapon strength assessment.

### Mods

- `[?]` Updated ICE to version 21.
- `[?]` Updated PICKS.CFG.

## 1.50.12

### Game changes

- `[?]` Map knowledge is now transferred on surrender.
- `[?]` Randomized order in which players roll for leaders.
- `[n/a]` Fixed possible crash when clicking a for hire leader who brings a technology in the Leaders screen (1.50.10 bug). -- a 1.50.10 bug, fixed later
- `[n/a]` In the Fleets screen, clicking the right-arrow below the galaxy map no longer freezes the game when there are no fleets to show. (1.50.7 bug). -- a 1.50.7 bug, fixed later
- `[n/a]` Fixed crystal-captured ships sometimes crashing on combat load (1.50.6 bug). -- a 1.50.6 bug, fixed later
- `[n/a]` Fixed non-initiative tactical combat freezing when the combat round has ended if all remaining opponents ships are in stasis (1.50.3 bug). -- a 1.50.3 bug, fixed later
- `[?]` Combat is stopped if the remaining side is an AI, even if it can still bomb planets or shoot missiles, preventing AI ships from moving aimlessly (1.50.1 issue).
- `[n/a]` Improved bomb hits calculation to reduce slow down when flyers involved, preventing possible crash when bombing (1.50.9 bug). -- a 1.50.9 bug, fixed later

### Interface changes

- `[done]` Default build list 7 is now 'prepend freighter fleet', 8 is 'prepend spy' and 9 is 'append housing'. -- docs/screens/build-lists.md
- `[?]` Fixed 3 detection giveaways when undetected ships are indirectly revealed: • In fleet summary tooltip in Star System popup. • In summary below galaxy map in Fleets screen. • When scrolling in Fleets screen via <> with either ships filter off, seemingly empty fleets would appear suggesting the presence of filtered ships (normally empty fleets are skipped).
- `[?]` In Fleets screen "support / combat ships" filter buttons now always work.
- `[?]` Opponents' ships will no longer color green in the Fleets screen when no player owned ships are present.
- `[?]` Fixed part of color palette of Star System popup sometimes changing to bright green.
- `[?]` Correct race trait help popup in the Race Statistics subscreen of Info, if number of race traits exceeds 11 (1.50.3 issue).

### New config parameters

- `[?]` Added cmbtshp.lbx, officer.lbx, races.lbx, racename.lbx, ships.lbx, shipname.lbx, starname.lbx.
- `[?]` Table race_personality, sets AI personality chances.
- `[?]` democracy_gets_psionics_morale, applies psionics morale bonus to democracies and federations.
- `[?]` lithovore_has_fungi_in_tech_tree, places biomorphic fungi in the tech tree of lithovore races.
- `[?]` stealth_no_size_penalty, defines to which stealth type the (detection) size penalty does not apply: race, race and tech, or none.
- `[?]` stealth_field_beam_defense, enables stealth field to act like a cloaking device.
- `[?]` ranged_to_hit_penalty, sets accuracy penalties for all 8 ranges.
- `[?]` ranged_damage_penalty, sets damage dissipation for all 8 ranges.
- `[?]` starbase_fleet_beam_offense added to table combat_bonus.
- `[?]` garrison_slots, enables to set 1 or 3 slots instead of the default 2 for fighter garrison.
- `[?]` all_planet_shields_block_bioweapons, extends blocking effect to radiation and flux shields.
- `[?]` first_council_delay, sets first turn that a council meeting can happen.
- `[?]` council_frequency, sets number of turns between each council meeting.
- `[?]` dragon_bribe_bc, sets amount BC needed to bribe the roaming Dragon.
- `[?]` computer_virus, sets both fixed and random amount research lost when event occurs.
- `[?]` donation_bc, exposes 3 parameters of formula: turn_divisor, multiplier and base amount.
- `[?]` pirate_raid, sets both fixed and random percentage cash reserve lost when event occurs.
- `[?]` nova_minimum_research_per_turn, sets a minimum research points needed per turn on the colonies in the system to the invent the solar rejuvenator that ends the supernova threat (classic is 0).
- `[?]` bc_system_discovery, exposing space_debris and pirate_cache BC amounts.
- `[?]` menlo_research_points, sets amount of research points for menlo cheat.

### Changed config parameters

- `[?]` board_in_black_hole now also prohibits boarding from ships trapped in a black hole.
- `[?]` Renamed offensive_ships_crew_level to elite_offensive_ants, separated it from antaran_fleet table, and corrected it to include crew experience points in addition to crew level.
- `[?]` Renamed jamming_cloak_bonus to cloaking_device_missile_defense and inserted it into missile_defense table.
- `[?]` Removed now redundant jamming_cloak; cloaking_device_missile_defense simply applies if set >0.
- `[?]` Renamed toxic_can_become_barren to terraform_toxic_building.
- `[?]` Renamed ai_difficulty_bonus_productivity to ai_productivity_bonus.
- `[?]` Renamed ai_difficulty_bonus_income to ai_income_bonus.

### Mods

- `[?]` Added MOO2 Launcher option 'No AI surrenders', preventing AI surrenders before turn 1000.
- `[?]` Removed MOO2 Launcher option 'No black holes or wormholes'.
- `[?]` Updated 1.50 improved.
- `[?]` Updated ICE to version 19.
- `[?]` Renamed mod '1.50 Melee' to just 'Melee'.
- `[?]` Added multiplayer maps 'ICE MP' and 'GM3 MP' (adaption of VDCGM3 map).
- `[?]` Removed maps GM4, GM5 and ICE-cold.
- `[?]` Updated the other mapmods.
- `[?]` Added PICKS.CFG intended to simplify pickhacking (enable it in the file 150i.CFG for it to take effect).

## 1.50.11

### Game changes

- `[?]` Fixed last ship shuttle-captured stall in multiplayer.
- `[?]` Restored corner area where bombing is allowed to classic (1.50.4 bug).
- `[?]` Restored planet sizes for bombardment to classic 3-4-6-7-8.
- `[?]` In strategic mode ships now upgrade to the highest available missile/torpedo (was broken since 1.50.7)
- `[?]` Colony is not recalculated in autobuild anymore, so newly grown pop doesn't participate in production when autobuild is on.

### Interface changes

- `[?]` Planet graphics are now drawn centered in bombing area.

### New config parameters

- `[?]` no_starting_ships, removes scouts and colony ship from average and post-warp games.
- `[?]` no_initial_tech_prompt, disables game asking to select an initial technology.
- `[?]` crew_no_turn_xp, disables ship crews gaining 1 XP each global turn.
- `[?]` hire_chance_delay, enables to tune default value of 5 turns until first leader can appear.
- `[?]` multibuild, allows to build multiple items per turn (for melee mod!).

### Changed config parameters

- `[?]` Missile space and cost can now be set separately.
- `[?]` Shuttle shots are no longer tied with Bomber shots and are always single shot now.
- `[?]` Renamed arc_cost_space to arc_cost_and_space.
- `[?]` Renamed iface_info_menu to info_lbx_menu.
- `[?]` Renamed human_player_fleet_can_retreat to human_player_fleet_auto_retreat.

### Lua scripting

- `[?]` Scripts can access combat info, no ship control yet.
- `[?]` Scripts can delete or scrap ships overmap.

### Mods

- `[?]` Updated 1.50 improved.
- `[?]` Addded 1.50 melee, a new experimental mod for 2 player tactical combat.
- `[?]` Updated ICE to version 18a, see README_ICE.TXT for changelog.
- `[?]` Updated maps Orange II and GoodMap3, 4, 5.
- `[?]` Added melee map.

## 1.50.10.1

### Game changes

- `[?]` Fixed possible memory corruption when starting new game (1.50.10 bug).

### Interface changes

- `[?]` Reverted 1.50.10 fix for multiplayer games Leaders screen issues, since it caused a new graphical problem in single player games Leaders screen.

### Lua scripting

- `[?]` Fixed newgame_postprocessor_script not working (1.50.10 bug).
- `[?]` Mapgen script now happens before autosave.

### Mods

- `[?]` Updated 1.50 improved.
- `[?]` Updated ICE to version 18.

## 1.50.10

### Game changes

- `[?]` Antaran ship costs and limits now persist after reloading the game.
- `[?]` Fixed AI ships sometimes getting double Augmented Engines +5 speed bonus.
- `[done]` Fixed another case of ship design slot corruption when a ship couldn't be built due to the 500 ships limit (in addition to 1.50.7 fix). -- docs/turn/design6.md
- `[?]` Combat AI is now aware that Ion Cannon has no effect against planets.
- `[?]` Undocumented +1 hit bonus for civilian buildings during bombardment removed.
- `[?]` Unstable wormhole event is now also allowed for a fleet with eta=2 turns.
- `[done]` Fixed the game freezing when an AI without Fungi tech gets stuck optimizing pop for food on a liberated and blockaded non-food planet in case previous owner has Fungi. -- docs/turn/overview.md
- `[?]` Reduced satellite space compensation for beam arc and missile mods from +40% to +33.3%.
- `[?]` Stealth Field is now mutually exclusive with Cloaking Device and Phasing Cloak.
- `[?]` Scout Lab & Galactic Lore no longer provide bonus against Antarans (reverted back to classic).
- `[?]` Warp Interdictor range reverted back to classic 3 parsecs.
- `[done]` Fixed canceling warning for repeat+android resulting in a bare repeat (adding a new product to it caused the game to freeze). -- docs/screens/build-queue.md

### Interface changes

- `[?]` Leader tech is shown both at arrival if player has insufficient money and upon clicking Hire from Leaders screen.
- `[?]` In multiplayer games the Leaders screen will always default to colony leaders instead of to the tab where the previous player left it, and shows proper colonies, stars and fleets at loading.

### New config parameters

- `[?]` replicator_food_cost_no_bc, removes the 1 BC cost for food production.
- `[?]` pop_bonus_subterranean, allows to set subt bonus per planet size class.
- `[?]` pop_bonus_tolerant, allows to set Tolerant extra pop % bonus.
- `[?]` spies_can_frame_anyone, removes restriction 'only frame players who do not have the stolen tech'.
- `[?]` between_events_delay, allows to change classic delay of 5 turns.
- `[?]` lab_and_lore_against_antarans, enables BA bonus against invading Antarans.
- `[?]` reinforced_hull_multiplier, for both regular satellites and the Antaran fortress.
- `[?]` marine_kill_damage, for weapons with inherent mod #4 (Neutron Blaster/ Death Ray).
- `[?]` antaran_fleet offensive_ships_crew_level, allows change from veteran level.
- `[?]` ground_defense_armor_multiplier, allows to increase base defense hp.
- `[?]` civilian_armor table, allows to set non-defense hp per armor level (classic always 100 hp).

### Changed config parameters

- `[?]` Renamed 3rd special of both Antaran frigates to special_3_hard_imp.
- `[?]` Renamed Antaran parameter phrase stop_sending to cease_building.

### Mods

- `[?]` Renamed '1.50 standard' to '1.50 improved'.
- `[?]` The '1.50 improved' mod from previous release has been dropped.
- `[?]` Updated ICE to version 17, see README_ICE.TXT for changelog.

## 1.50.9

### Game changes

- `[?]` Mapgen improvements: • Minimum 3 planets in home systems as intended instead of 2 planets in version 1.40 and 1 in 1.31. • The switch /planets now also accepts 1. • Fixed mapgen generating non-standard systems in some cases. • Fixed issues with trim_planets (which is used when mapgen generates more than 250 satellites in a galaxy). • Wormholes will no longer be placed to the Orion system.
- `[?]` Fixed actual tech cost of first level Hyper-Advanced from 15k to 25k.
- `[?]` Lightning_field default setting is no longer 'uniform' but 'classic'.
- `[?]` Satellite can no longer be destroyed by bombing the planet or earthquake (1.50.2 bug).
- `[?]` Antaran defensive Titan 8th weapon slot restored (1.50.8 bug).
- `[?]` Strategic combat now honors crew level bonus for BA/BD/MD.
- `[?]` Fixed bomb hits calculation for orbital bombardment: • Bomb weapons now get bomb hits equivalent to 10 instead of 5 attacks. • Fighters get the equivalence of 1 strike instead of 0.

### Interface changes

- `[?]` Eliminated empires are now shown on history graphs.
- `[?]` Tech breakthrough turn counter is more precise.
- `[?]` When you have a Galactic Lore leader, the filter 'No Enemy Presence' in Planets screen now only shows planets that have been visited, preventing the sending of colony ships to monster guarded worlds.
- `[?]` Fixed display of artifacts status for visited non-HW artifact planet in the Planets screen.
- `[?]` Tractor beam drawing is now always removed immediately after the target is destroyed.
- `[?]` Restored galaxy size 1 label to 'Medium' (was 'Average' since 1.50.5).
- `[?]` Alt+x now updates ship routes and colonies.
- `[n/a]` Alt+x no longer switches to defeated empires, which could crash the game. -- the port has no Alt+X cheat

### Config changes

- `[?]` EXTRACT.CFG retains core mod metadata.

### New config parameters

- `[?]` jump_gate_pass_nebula, causes fleets in Jump Gate to ignore nebulae.
- `[?]` ai_builds_colony_bases_prewarp, allows AI to build colony bases without drive and fuel techs.
- `[?]` fighter_shots, exposes number of shots each fighter can do in one run.
- `[?]` bombardment_use_armor_for_buildings, gives non-defense planetary structures 1/2 hp of defense buildings, with a minimum of 100 as before.
- `[?]` disable_physics_updates_ship_designs, prevents auto-updating of ship designs upon completion of the Physics (Laser) tech field.
- `[?]` design_button_shield, causes shield of design not to be updated.
- `[?]` design_button_computer, causes computer of design not to be updated.

### Changed config parameters

- `[?]` Split clear_button into clear_button_shield and clear_button_computer.
- `[?]` Renamed monster_drive.crystal to monster_drive.crystal_and_guardian.
- `[?]` Removed armor_extra.

### Lua scripting

- `[?]` Added get_ships.
- `[?]` Fixed /noscan being checked instead of /noreport for get_player_techs (1.50.7 bug).
- `[?]` Main1.lua emits warning when /noreport=1.
- `[?]` Fixed get_conf not working for scalar parameters (1.50.7 bug).
- `[?]` Fixed add_planet overwriting existing planet in case of orbit clash (1.50.8 bug).
- `[?]` Fixed typo and possible crash in Lua error message (1.50.8 bug).

### Mods

- `[?]` Updated the 1.50 standard ruleset (SET150).
- `[?]` Updated the 1.50 improved ruleset (SET150i).
- `[?]` Updated ICE version 16, see ICE_readme.txt for changelog.
- `[?]` Added maps 1.50 Improved, GoodMap4 and GoodMap5.
- `[?]` Updated Orange2 map to have unicolor stars: White in young, orange in avg, and red in old galaxies.

## 1.50.8

### Game changes

- `[?]` Improved Combat AI (also used for final and defensive fire), now considers the following: • Damper Field, Energy Absorber, Hard Shields, • Ordnance skill, • Nebula nullifies ship shields, • Planetary shields block the Shield Piercing mod, • Bomb damage against a planet is reduced by ship shields too, • MIRV missile damage versus shields, • Custom bonus for cloaking device, both evasion and jamming modes.
- `[?]` AUTO combat AI will no longer retreat ships just because they are out of ammo.
- `[?]` Defensive Fire is always active in non-initiative games regardless of DF setting.
- `[?]` Turned off (reddened) PD beams and A-M Rockets will now fire during Seeking missiles phase; reddened Spatial Compressor still will not.
- `[?]` Fixed Avenger, Antaran Battleship & Titan having bombs with FWD instead of 360 arcs.
- `[?]` Fixed Lightning Field uniform mode not applying (1.50.5 bug).
- `[?]` Changed freighters_cash_bonus default from 5 to 0 BC so building a freighter fleet no longer generates free cash.
- `[?]` Human-to-human tech trade ignores AI's trade decisions, all tech can now be traded.
- `[?]` Static cheats will no longer persist when starting or loading a game.
- `[n/a]` Fixed crash when loading jimtext.lbx and current language is Italian. -- the port runs English only

### Interface changes

- `[?]` Added GUI MOO2 Launcher and improved mod format.
- `[?]` Corrected icons of tiny toxic, radiated and barren planets in the System Overview in the Colony screen that were swapped.

### New config parameters

- `[?]` toxic_can_become_barren, allows terraforming of toxic planets with a building of choice.
- `[?]` ai_hypertech_threshold, allows AI to research a set number of advanced technology levels.
- `[?]` human_player_fleet_can_retreat, enables player's ships to retreat like AI on AUTO combat.
- `[?]` combat_ships_without_ammo_stay, toggles retreat for out of ammo ships on AUTO combat.
- `[?]` proton_torpedo_apply_speed_base, limits proton torpedo speed.
- `[?]` missile_ammo, exposes default missile volleys in ship design.
- `[?]` satellite_ammo_basis, exposes ammo amount used as basis for space calculation.
- `[?]` missile_base_ammo_basis, exposes ammo amount used as basis for space calculation.
- `[?]` Exposed missile base and fighter garrison amount of shots available.
- `[?]` With antaran_star_fortress_space, weaponry space is set independently from hull table.
- `[?]` simplified_beam_formula, enables the range penalty and PD bonus to be calculated in a manner uniform with other to hit bonuses.
- `[?]` 2x_range_accuracy_penalty, toggles double accuracy penalty for the 2x weapon mod.
- `[?]` 2x_range_damage_penalty, toggles double damage dissipation for the 2x weapon mod.
- `[?]` With weapon_range, special weapons ranges can be set.
- `[?]` With spherical_range, spherical weapons ranges can be set to both ships and missiles.

### Changed config parameters

- `[?]` Renamed weather_control_min_climate to weather_control_climate.
- `[?]` Setting weather_control_climate to 'radiated' no longer blocks weather control building on toxic planets.
- `[?]` growth_rate_bonus contains general modifier, microbiotics, antidote, boom and plague.
- `[?]` Renamed missile_volley to missile_ammo.
- `[?]` Adjusted Hydra special parameter, showing it is shared between system and event Hydra.
- `[?]` Parameter vdc_mode removed.

### Lua scripting

- `[?]` Mapgen scripts with full access to stars and planets.

### Mods

- `[?]` Updated SET150 (default config).
- `[?]` Added SET150i (1.50 improved ruleset, replacing Vanilla+ mod).
- `[?]` Added the ICE mod in distribution, see ICE_readme.txt for changelog.

## 1.50.7.2

### Game changes

- `[?]` Fixed incorrect Plasma Torpedo dissipation on impact (1.50.2 bug).
- `[?]` Satellite missile amount distribution: 1-3 in 1 slot; 4 in 2 slots; higher amounts in 3 slots.
- `[?]` Improved AI assessment of modded Fighter Garrison strength.
- `[?]` Improved AI assessment of modded Hard Shields strength.
- `[?]` Fixed a bug when GNN reports 0 RP to end the plague and prolongs it.

### Interface changes

- `[?]` Fixed Jump Gate warnings appearing for fleets that are still at a star (1.50.7 issue).

### Changed config parameters

- `[?]` Removed unused parameter Second_Weapon_item6 for event_dragon.

## 1.50.7.1

### Game changes

- `[?]` Autodesign won't install heavy mod on beams without heavy mod.
- `[?]` Fixed long messages after raiding in super-fast combat mode (1.50.7 bug)
- `[n/a]` Fixed fleets using Jump Gate being lost in black hole (1.50.7 bug) -- a 1.50.7 bug, fixed later
- `[n/a]` Antaran defensive Raider's weapon_2_amount now won't cause first letter of a ship name to change or other irregularities (1.50.6 bug) -- a 1.50.6 bug, fixed later

### Interface changes

- `[?]` Press PageUp/PageDown anywhere in the game to speed up/slow down animation.
- `[?]` Removed unused fast mod from anti-matter torpedo, fixing irregular appearance of mod buttons in design screen.
- `[?]` Tech no longer appears more than once in the acquired tech list when scrapping a captured ship.

## 1.50.7

### Game changes

- `[?]` Satellite and ground defenses auto design reworked. • List of weapon mods available is now configurable. • Space allocation per weapon type is now configurable. • Weapon mods now cost space. • Missiles spread evenly over slots, e.g. 2 2 2 instead of 6 0 0. • Up to 99 missiles per stack instead of 30. • Corrected arc cost for beams in satellites (was +25%) and ground batteries (was +0). • Ground batteries space increased by +50% for the corrected arc cost. • Satellite space increased by +40% for the corrected arc and missile mods cost.
- `[?]` Leaving/catching up/levelling up of navigator affects fleets immediately.
- `[?]` Telepath or Telepath* leader now protects colonies from Mind Control.
- `[?]` Assassin leader no longer protects colonies from Mind Control.
- `[?]` Artemis system net built/removed wakes up orbiting fleets now.
- `[?]` Fixed irregularities in ship placement in case of large fleets.
- `[n/a]` Fixed a crash when ICS is on and ship loses speed while moving making its move points negative. -- ICS is a 1.50 feature
- `[?]` Fixed division by zero crash when damaging internals of a ship with no systems left but drive with 1 hp.
- `[?]` Ship captured on retreat by Assault Shuttles now stays in battle.
- `[?]` AI won't self-destruct ship immobilized by BHG to avoid capture if boarding in black hole is disabled.
- `[?]` Fixed a crash when ship is destroyed by BHG and ship initiative is off.
- `[?]` Enabling strict_combat_sequencing now forces ship initiative.
- `[?]` Comet now counts as destroyed as soon as 100% of it is destroyed, not the turn after as in classic.
- `[?]` Ship scrap total won't overflow. Scrapping ships for total sum of more than ~32k BC overflows in classic.
- `[n/a]` Fixed game crashing on random sounds, for example in system window or colony view. -- a bad sound index plays nothing (lbx.Archive.Entry checks it)
- `[?]` Fixed no sound crash if player has no rifle upon ground invasion (only in modded games).
- `[?]` AI now better assesses strength of ships with BHGs.
- `[?]` Nulled interceptors tactical damage value (6-15) as AI use separate calculation function for them.
- `[?]` Strategic instead of tactical weapon damage values are now considered for weapon selection in strategic mode.
- `[?]` Strategic damage values column updated with tactical values to preserve classic behavior, except for special weapons that were bugged or incorrect and received new values.
- `[?]` GNN news corrected for monster events, colony names and (English) textual errors.
- `[?]` Monsters now consider systems in +10pc wider range for finding new target colony.
- `[done]` Fixed 6th slot corruption when a ship couldn't be built due to the 500 ships limit. -- docs/turn/design6.md
- `[?]` Fixed auto-scrap scrapping more ships than needed to pay upkeep.
- `[?]` Fixed a crash in fleet leaders screen when scrolling fleet pane.
- `[n/a]` Fixed a crash when message box has too much text. -- text boxes draw bounds-checked; cosmetic at most
- `[n/a]` Fixed a crash when raid message is too long. -- the raid report is a Go string

### Interface changes

- `[?]` 150.SET is used instead of MOX.SET to avoid version conflicts.
- `[?]` LASTR150.RAC is used instead of LASTRACE.RAC to avoid version conflicts.
- `[?]` Updated non-English New Game screen labels.
- `[n/a]` Race selection screen won't crash on races with too many traits. -- the trait list is a Go string, no 300-byte buffer
- `[?]` Warning will now appear upon redirecting fleet out of Jump Gate.
- `[?]` List of techs received on Orion now includes techs provided by Loknar.
- `[?]` Colony leader commando skill is now listed in Colony View and in Ground Combat screen.
- `[?]` AI bonus is now listed in Ground Combat screen.
- `[?]` Buying an item now deselects currently selected queue item.
- `[n/a]` Fixed a bug that allowed to insert multiple consecutive repeats using build lists. -- a bug of 1.50 build lists; ours never inserts consecutive repeats
- `[?]` Colonized brown stars will no longer be displayed as uninhabited on minimaps.
- `[?]` A number of visual fixes and improvements in Fleets and Leaders screens.
- `[?]` Fixed a bug that made missiles to face away from their target sometimes.
- `[?]` Fixed broken quick win check when strict_combat_sequencing is on.
- `[?]` Sound effects are silenced in super-fast combat mode.

### New config parameters

- `[?]` satellite_space_allocation, for customizing satellite weaponry.
- `[?]` defenses_allowed_mods, for configuring mods installed on defenses.
- `[?]` troop_type, a table for tank, militia and rebel unit bonuses.
- `[?]` troop_bonus, a table with various troop combat bonuses and hits.
- `[?]` rebel_display, to display rebels in colony views military popup.
- `[?]` alternate_diplomacy, enables separate diplomacy patience counters.

### Changed config parameters

- `[?]` Missile Base now also uses torpedoes via satellite_can_use_torpedoes.
- `[?]` Simplified starbase special table, can be configured with tech names.
- `[?]` Postwarp tech list has 7 instead of 6 exclusions for strategic mode.
- `[?]` Renamed 'troop_bonus_rifles' table to simpler 'rifle_bonus'.
- `[?]` Renamed defensive_range to defensive_range_bonus.

### Lua scripting

- `[?]` Added preliminary Lua scripting support.

### Mods

- `[?]` Updated SET150 (default config) and Vanilla+.

## 1.50.6.1

### 1.50.6 bug fixes

- `[?]` Building of an artificial planet causing damage of the planetary array.
- `[?]` Planet Info displaying wrong food value for Aquatic races.
- `[?]` Incorrect food output on mapgen generated planets.
- `[n/a]` Possible crash at new game. -- a 1.50.6 bug, fixed in 1.50.6.1

### Mods

- `[?]` Updated SET150 (default config).

## 1.50.6

### Game changes

- `[?]` Fuel range for contact is now consistent with the ability to visit colonies.
- `[?]` Fixed received techs from surrender being only up to #83 from tech list.
- `[?]` Added possibility to acquire a shield tech from a scrapped ship.
- `[?]` Fixed blank "You received the following techs from scrapping this ship" message popups on scrapping lander ships.
- `[?]` Removed best armor minus 1 rule for lander ships, now they have the best armor available.
- `[?]` Ships with no computer fitted now won't have computer's hit points.
- `[?]` Newly built Warlord ships get crew level bonus from the turn of build.
- `[?]` Building a Fighter Garrison now wakes up orbiting ships, in line with other planetary defenses.
- `[?]` Building a planetary shield won't wake up orbiting fleet now.
- `[?]` Destroying or selling a satellite or ground defense building will now wake orbiting fleets.
- `[?]` Fixed right mouse button stuck when loading pre-battle autosave in hotseat.
- `[n/a]` Fixed Beam_SFX crash on fighters resolve on top-left corner. -- our Random(0) returns 1 without dividing
- `[?]` Fighters now benefit from helmsman.
- `[?]` Fixed buggy fighter counts after returning to the launching ship.
- `[?]` Partial fighter squadrons that return to ship do not cause the loss of weapon bays after the battle.
- `[?]` Corrected Absorber's miss chance due to Displacement Device to 30% (70%).
- `[?]` Beams Displacement parameter now also affects Absorber's chance to miss.
- `[?]` BHG and Stasis Field now free ships correctly when the weapons themselves are destroyed.
- `[?]` Fixed ship special becoming unbreakable when dealt damage equal to its HP.
- `[?]` Proper visuals in Raid message, now a damaged special is reported as damaged, instead of destroyed, also misleading number of reported hits no longer appear.
- `[?]` Combat ends immediately if the losing side has no missiles or planet.
- `[?]` Fixed a range of GNN monster reports not showing up.
- `[?]` Amoeba stays in game if its target is destroyed before it arrives there.
- `[?]` Amoeba applies its toxic ability to AI colonies too.
- `[?]` Fixed hyperspace flux check for comet and monsters so they can't happen during a flux now.
- `[?]` Behold the arti fixed for single and multi player; renamed parameter to finds_presented_by_scientist.
- `[?]` Strength of a modded missile base or ground batteries is now properly assessed by AI.
- `[?]` Added support for ESD custom ratio in determine_beam_weapon_modifications.
- `[done]` Dismissed leaders desync fixed, they will now always acquire +1 level in a network game. -- docs/net/turn.md
- `[?]` Disabled (reddened) point defense weapon does not automatically fire at incoming missiles anymore.
- `[?]` Modded leader XP levels no longer cause fired leader to get wrong XP.
- `[?]` Terraforming now works correctly with arrivals, arriving colonists come on a terraformed planet instead of dying (1.50.1 bug).
- `[?]` Fixed steal and uncreative bugs with keep_on_find, of not being directed to the tech choice screen when that same tech was being researched (1.50.5 bug).

### Interface changes

- `[?]` Moved tractors from equipment to weapons category in Tech Review screen.

### Configuration

- `[?]` Command 'import' is no longer available in config.

### New config parameters

- `[?]` Added tables for all economic bonuses, food, production, research and BC.
- `[?]` Added AI food/prod/research bonus and AI BC bonus.
- `[?]` All food bonuses use 0.5 scale: A bonus of 2 now means 1 actual food.
- `[?]` Added version-dependent old save fix. Planets from older saves will have food_base fixed.
- `[?]` gravity_table, for mapgen.
- `[?]` govt_morale_penalty.
- `[?]` no_capitol_morale_penalty.
- `[?]` barracks_morale_bonus.
- `[?]` federation_no_mixed_races_penalty.
- `[?]` Added parameter to set minimum climate where weather controller can be built.
- `[?]` Lightning field gets classic mode via lightning_field_mode.
- `[?]` leaders_no_limit, enabling to have more than 4 leaders of any kind.
- `[?]` arc_cost_space, to set weapons arc cost/space consumption.
- `[?]` satellite_warlord_bonus, enabling Warlord for satellites and planets.
- `[?]` leader_warlord_bonus, making Warlord leaders function at +1 higher level.
- `[?]` Added turn delay table for all events.
- `[?]` crystal_keep, causes ships captured by Crystal to not die and become part of its fleet.
- `[?]` monsters_stay_chance, to tweak the chance a monster leaves after battle.
- `[?]` Added monsters_stay, forces monsters to not leave the galaxy (test parameter).
- `[?]` battle_pods_speed_bonus, prevents (empty) ships gaining extra speed from battle pods. When enabled, ships can be overloaded up to immobile status.
- `[?]` minimum_combat_speed, sets minimum speed below which ship cannot be overloaded.

### Changed config parameters

- `[?]` Tech, techfield, specials numbers are no longer settable in config.
- `[?]` UNITS table removed from config.
- `[?]` Removed +279 nonsense from starbase specials table.
- `[?]` Monster drive and hit points table split into separate tables.
- `[?]` General config cosmetics pass, renamed parameters now all in lowercase.

### Mods

- `[?]` Updated SET150 (default config) and Vanilla+.

## 1.50.5

### Game changes

- `[done]` System popup freeze fixed. -- docs/screens/system-display.md
- `[?]` Pirates will no longer seize negative amount of money from richer empires.
- `[?]` Fixed "leader capture" bug, a leader en route to a captured ship is put to officers pool.
- `[?]` Breakthrough & serendipity in one turn won't cause blank box in tech choice.
- `[?]` Fixed lingering missile alerts (most common case is a leftover from AI turn).
- `[?]` Fighters can now select any ship as a new target after destroying a missile.
- `[?]` Fixed erroneous overmap display of Captured label.
- `[?]` Human players cannot build spies now without spy network technology.
- `[?]` Capitol can't be built now without capitol building technology.
- `[?]` Shield will not be installed on a starbase if damper field is present.
- `[?]` Enabling strict_combat_sequencing now forces ship initiative in combat.
- `[?]` Fixed heavy fighters not spending ammo when attacking missiles (1.50.1 bug).
- `[?]` Fixed garbage appearing in retreating ship's weapon list (1.50.3.1 bug).
- `[n/a]` Fixed a crash with ship captures during battle at Antares (1.50.2 bug). -- a 1.50.2 bug, fixed later
- `[n/a]` Build list no longer allows buying on blockaded colonies (1.50.4 bug). -- a 1.50.4 bug, fixed in 1.50.5
- `[n/a]` Prepend will no longer push down a bought item (1.50.4 bug). -- a 1.50.4 bug, fixed in 1.50.5
- `[n/a]` Fixed fighters disappearances during missile seeking phase (1.50.4 bug). -- a 1.50.4 bug, fixed later
- `[?]` Fixed instant refit when the queue is full (1.50.4 bug).
- `[n/a]` Fixed possible stall on engine explosions in Super-fast combat mode (1.50.4 bug). -- a 1.50.4 bug, fixed later

### Interface changes

- `[?]` Added compatibility with non-English versions and support of multilingual mods.
- `[?]` Destroy outpost and mind control popups now expire by timeout.
- `[?]` Default popups timeout is about 1 hour now.
- `[?]` Fixed spelling of Darloks button in racesel.lbx.
- `[?]` During retreats the retreating ship is always displayed as current.
- `[?]` Status bar always displays an active ship between turns.
- `[?]` Ability to format turn counter and stored production separately (format prod2).
- `[?]` Introduced templates for better control over in-game labels.
- `[?]` Config extractor no longer omits last comment.

### New Config parameters

- `[?]` defensive_fire_range, sets range from which defensive fire is active.
- `[?]` raid_items_per_marine, sets the max number of items destroyed on raids per marine.
- `[?]` raid_damage_per_marine, controls damage per successful marine raid.
- `[?]` ship_command_points_usage table, controls per size class command points consumption.
- `[?]` landers_size_class, controls size class of landers.
- `[?]` keep_research_on_finds, prevents RPs from being zeroed when the tech currently being researched is obtained otherwise.
- `[?]` keep_research_on_breakthrough, causes RPs above twice the tech cost to not be lost after a breakthrough.

### Changed config parameters

- `[?]` Fixed various parameters for monsters.

### Mods

- `[?]` AISHIPS enabled in SET150, improving ship designs of the AI.
- `[?]` New herodata.lbx changes 3 leaders who had the Tactics skill & mod Yota.
- `[?]` Added new mod Vanilla+.
- `[?]` Added new mapmod Ice Cold (cold.cfg).

## 1.50.4.2

### Game changes

- `[done]` Previously overlooked case of feudal bug fixed. -- docs/formats/tech.md

### Interface changes

- `[?]` Fixed garbage appearing after race pick labels for network client.

### New config parameters

- `[?]` Added table COMPUTERS_name.

## 1.50.4.1

### Game changes

- `[?]` Initial autosave now happens after the mapgen.
- `[?]` Uncreative will not lose techs in modded greybox field now.
- `[?]` TWF fixups for combat with ship initiative off.
- `[?]` Some inherent mods now work for missiles.

### New config parameters

- `[?]` Added wormhole_color parameter.

### Changed config parameters

- `[?]` Mapgen parameter /monsters now works.
- `[?]` Hero removed by -smarooned is now replaced with a pirate cache.
- `[?]` Parameters -tgaia, -tgoodhw and -mmonst are now consistent with mapgen.exe.
- `[?]` vdc_mode updated.

## 1.50.4

### Game changes

- `[done]` Monster bug fixed, no combat rollbacks when fighting identical monsters. -- docs/net/combat.md
- `[?]` Removed Advanced damage control from tree for cybernetic races.
- `[?]` Ship initiative rule now takes BA differences below +10 into account.
- `[?]` Ships tied by initiative are now sorted by crew experience.
- `[n/a]` "Mark Block Stack Size Exceeded" crash in races screen fixed. -- no block allocator in the port
- `[?]` Ship captured by shuttles when rotating will not cause desync in network games.
- `[?]` Super-fast combat mode (Z) now takes ship explosions into account.
- `[?]` PD and Spatial Compressor range is now calculated correctly mid flight.
- `[?]` Fixed discrepancy between targeting and damaging ranges for spherical weapons.
- `[?]` Stasis disables wide area jammer, warp dissipator, scout lab.
- `[?]` Ship under stasis now ceases to be affected by tractors and BHGs.
- `[?]` Ship under stasis now ceases to affect others by tractors, BHGs, stasis.
- `[?]` Fixed a number of bugs in defensive fire.
- `[?]` All retreats now happen after all ships have recharged weapons.
- `[?]` Repaired warp dissipator now prevents retreats ordered on previous turn.
- `[?]` Missile alerts will now be triggered by in place rotations.
- `[?]` Correct missile alert halt on move when ship is going to be hit while turning.
- `[?]` Ship with inertial nullifier does not pull missiles when rotating.
- `[?]` Fighters chasing missiles with depleted moves will now move at the end of turn.
- `[?]` Ordnance bonus now works for fighters.
- `[?]` Non-TWF ships now update at the end of TWF turn.
- `[?]` Non-TWF ships do not perform defensive fire in TWF turn.
- `[?]` Fixed bug which allowed to capture satellite sometimes.
- `[?]` Ship capture removes wait and done states.
- `[?]` Fixed integer overflow for Energy Absorber's stored damage.
- `[?]` Different cloaks now work correctly if put on a ship together.
- `[?]` Marines above ship capacity will now die after ship capture.
- `[?]` Equal marines and armor limits now apply to colonies with and without Barracks.
- `[?]` The last marine and armor units of a Subterranean colony no longer die at end of turn when the maximum has been reached.
- `[n/a]` Corrected race pick costs sync when loading net game (1.50.2 bug). -- a 1.50.2 bug, fixed later
- `[?]` Fixed broken cloaking device still being able to cloak (1.50.3 bug).
- `[?]` Fixed phasing cloak recloaking even with zero counter (1.50.3 bug).

### Interface changes

- `[done]` Advanced build list with the ability to apply on all colonies. -- docs/screens/build-lists.md
- `[done]` Undo buy action by clicking buy button again. -- docs/screens/build-lists.md
- `[?]` Fixed most build queue bugs including repeat build cheat.
- `[?]` Colony Base button disappearance trigger corrected.
- `[?]` Disabled weapon slots are colored red even if spent (used to be yellow).
- `[?]` In super fast combat mode (Z) boarding messages are skipped now.
- `[?]` Mapgen -tfixedhw will now generate lowg Arid planets for low-G races.
- `[?]` Restored classic behavior for Stellar Converter, it now fires the whole slot at once.
- `[?]` Added confirm_battle_message_timeout for "X attacks Y at Z" popups.
- `[?]` Fixes and improvements in New Game screen help.
- `[?]` Fixed broken command summary display in main screen (1.50.3 bug).
- `[?]` Fixed excessive orbital bombardment death tolls (1.50.3 bug).
- `[n/a]` Fixed stall when refitting a ship with queue full (1.50.2 bug). -- a 1.50.2 bug, fixed later

### New Config parameters

- `[?]` disable_vsync, speeds up screen updates.
- `[?]` ai_surrender_delay, specifies the turn after which AI can surrender.
- `[?]` stars_in_galaxy, table which sets amount of stars per galaxy size.
- `[?]` tech_mini, per-tech miniaturization group table.
- `[?]` Added some growth related parameters (growth formula, housing, cloners).
- `[?]` Added separate android costs to misc_costs table.
- `[?]` trade_goods_ratio, controls trade goods effectiveness.
- `[?]` initial_buildings, used for colony initialization.
- `[?]` planet_damper, enables damper field for planetary defenses.**
- `[?]` systems_repair_percent, controls repair rate for internal systems.
- `[?]` defensive_fire, allows to disable defensive fire altogether.
- `[?]` spatial_per_size_class_damage, makes Spatial Compressor damage larger ships more.
- `[?]` broken_engine_explodes, disables explosions for testing.
- `[?]` Added configs aiships, balance and map_150, expanding gameplay options.
- `[?]` vdc_mode, enables VDC compatibility patches.*

### Changed config parameters

- `[?]` Setting damper_kill_ratio to 0 now prevents the use of transporters against damper field.
- `[?]` Setting initial phasing cloak counter to 0 now means no phasing cloak state at all in combat.
- `[?]` Renamed several config parameters and labels.

## 1.50.3.2

### Game changes

- `[n/a]` Fixed freeze when ship remains tractored at the end of the combat (1.50.3.1 bug). -- a 1.50.3.1 bug, fixed later

## 1.50.3.1

### Game changes

- `[n/a]` Fixed cur_ptr crash related to strict_combat_sequencing (1.50.3 bug). -- strict combat sequencing is a 1.50 feature
- `[?]` Fixed rare ship array overflow related to capture.

### Changed config parameters

- `[?]` strict_combat_sequencing is now disregarded if ship initiative is off.

## 1.50.3

### Game changes

- `[?]` Fixed incompatibility with classic's save game format for ships with Augmented Engines.
- `[done]` Fixed ship retreat disappearances bug, immobilized ship now halts retreat. -- docs/combat/tactical.md
- `[?]` Fixed Antaran star fortress repair issues.
- `[?]` Fixed spy threshold below 60 not working. Negative thresholds are forbidden.
- `[?]` Two ships now can destroy each other with BHG simultaneously.
- `[?]` AI now honors board_in_black_hole parameter.
- `[?]` Tractors (both incoming and outgoing) break when ship phase cloaks.
- `[?]` Stasis field recharges correctly now.
- `[?]` Removing stasis field does not zero ship's shots.
- `[?]` Ship which started turn in stasis gets a chance to act this turn if freed.
- `[?]` Fixed a bug when fighters chase unrelated friendly ships.
- `[?]` Fighters now highlight when targeting missiles that target current ship.
- `[?]` Move blocked by missile alert will no longer trigger dogfights.
- `[?]` Ships now decloak if cloak-inducing device is broken.
- `[?]` Specials repaired in combat resume operation (in classic they don't).
- `[?]` Fixed bug in drive repair after combat, now ship is always made mobile.
- `[?]` Building & spy no longer give free cash on-build like in 1.40.
- `[?]` Fixed AI bought prod appearing as non-zero prod after capturing colony.
- `[?]` RPs produced on turn 0 are not lost even if you did not choose tech yet.
- `[?]` MENLO now gives 1 million research points.
- `[?]` Fixed ship defense racial trait not applying in combat (1.50.2 bug).
- `[?]` Fixed ship defense -20 displaying as -19 in ship info (1.50.2 bug).
- `[?]` Fixed bizarre damage visuals from ESD weapons (1.50.2 bug).
- `[?]` Reverted telepaths can capture ships multiple times without losing control (1.50.2 change).

### Interface changes

- `[done]` Corrupt 6th design slot (when loading pre 1.50.1 saves) is automatically fixed. -- docs/turn/design6.md
- `[?]` Autosave in local games works like in network games, saves every turn before battles.
- `[?]` Option to play against a random number of AIs.
- `[?]` Added pictures for cluster, random and post warp in newgame.lbx.
- `[?]` Added greyed Wait button for strict combat sequencing mode in combat.lbx.
- `[?]` Option to skip delete housing/trade goods dialog without deleting them.
- `[?]` Fixed broken money bonus display in summary in race selection screen.
- `[?]` Fixed broken money bonus display in race statistics.
- `[?]` Fixed broken +1/2 food display in race statistics.
- `[?]` Smaller font is used in race statistics if race has too many traits.

### Config changes

- `[?]` Config reload now reports success or failure via popup.

### New Config parameters

- `[?]` Optional strict combat sequencing mode.
- `[?]` Optional continuous BHG.
- `[?]` Optional satellite capture.
- `[?]` Support for custom shield and armor names.
- `[?]` Exposed mapmod tables, removed fixed mods ice, or2, gm3+ and added them as config files.
- `[?]` Configurable pre warp field set.
- `[?]` Freighter on-build income is tunable.
- `[?]` Added scout_lab_research_bonus table.
- `[?]` Added leader skill cost table and separate commando cost.
- `[?]` Optionally Megawealth does not double leader hire cost.*

## 1.50.2.1

### Game changes

- `[?]` Fixed bug causing absence of Unification Production bonus.

## 1.50.2

### Game changes

- `[?]` Planet bombing is now symmetrical, enabling bombing all around any world.
- `[?]` Cluster galaxy now works in multiplayer.
- `[?]` Fixed inability to move pop from non-food world to farming on food world.
- `[?]` Using other player's dimensional portal is now prohibited.
- `[?]` Ghost colony can be bombed or captured.
- `[?]` Ghost colony can die out even if colonists are inbound.
- `[?]` Starvation can create ghost colonies.
- `[?]` Bombardment can destroy capitol.
- `[?]` Auto-designed ships are equivalent to manually designed.
- `[?]` Satellite generation and repair uses systems size from correct hull table entries 6-8.
- `[done]` Fixed strategic Doom Star having cost and displayed info of tactical one. -- docs/turn/design6.md
- `[?]` Hard Shields now increase blocked damage in strategic combat.
- `[?]` Fixed Sub-Space Teleporter getting increased range for faster ships.
- `[?]` Fixed Sub-Space Teleporter not breaking tractor beam out of range.
- `[?]` Ship now loses shot right after weapon is destroyed.
- `[?]` Fixed discrepancy between 'immobile' label and real immobility after BHG or tractor beam.
- `[?]` Yellowing or reddening tractor beam slot no longer disables it next turn.
- `[?]` Multiple BHGs and Stellars installed in one slot now fire one by one.
- `[?]` Ship cloaked by Phasing Cloak is not affected by area damage now.
- `[?]` Ship cloaked by Phasing Cloak can't be boarded.
- `[?]` Lightning field buggy roll fixed, now kills 1/2 missiles instead of 1/3.
- `[?]` Fixed Antaran fighters not working on defensive Cruiser and Titan.
- `[done]` Fixed AI stall when targeting missile from captured ship. -- docs/combat/tactical.md
- `[?]` Scout lab and Galactic Lore now give bonus against antarans.
- `[?]` Telepaths can capture ships multiple times without losing control.
- `[done]` Crashes and quirks related to driveless trans dimensional ships fixed. -- docs/combat/tactical.md
- `[?]` Zeroth weapon flag usable.
- `[?]` No place for a new spy exploit fixed.

### Interface changes

- `[done]` A sixth ship design slot is now available. -- docs/manual/ships.md
- `[?]` Corrected display of colony ship count for 2 or more ships in Star System popup.
- `[?]` Visual fixes for ship, satellite and planet scan both for tactical and strategic modes. Strategic mode is now far less confusing.
- `[?]` Proper display of non-integer racial food bonus.
- `[?]` auto_close_board_confirm_popup option to close boarding report confirmations without user action.
- `[?]` Exposed difficulty level for which extra confirmation of new leader popup appears.
- `[?]` SFX volume in video is separate from music now.
- `[?]` Cheat warning will always be issued on the next turn after cheating.
- `[?]` Cheat warning will never be lost now.
- `[?]` Game settings for tactical combat, ship initiative and legal moves are set to 'ON' by default.

### New Config parameters

- `[?]` Added large amount of new parameters covering whole OCL and much more.
- `[?]` More lbxs support specifying alternative file in config.
- `[?]` interactive_combat_stats, makes ship stats update within the combat turn.
- `[?]` defensive_range, improves range for satellites and planetary defenses.
- `[?]` lander_ship_use_stealth.**
- `[?]` Added fungi for controlling biomorphic fungi mechanics.
- `[?]` Exposed parameters for several random events.
- `[?]` Exposed lucky chance.
- `[?]` Exposed boarding range in squares both ordinary and for transporters.
- `[?]` Optionally cloaking device gives missile evasion bonus instead of miss chance.
- `[?]` augmented_engines_speed_bonus.
- `[?]` sub_space_teleporter_range.
- `[?]` round_teleport, makes Sub-Space Teleporter jump area approximately round (octagonal).
- `[?]` Exposed trans dimensional bonuses.
- `[?]` Exposed proton torpedo range.
- `[?]` Exposed energy absorber damage reduction.
- `[?]` Exposed damper field damage reduction.
- `[?]` Score values exposed, orion, antares, council win.
- `[?]` Optionally enveloping weapon bonus does not work against planets. Renamed parameters
- `[?]` /picks > maximum_Positive_Picks
- `[?]` speed_td_bonus > td_combat_speed_bonus
- `[?]` torpedo_miniaturization_missile > torpedo_miniaturization_group_1 Removed parameters
- `[?]` shortened_anomaly_beast_flux (can be set individually now)
- `[?]` AI_races_improved (now fully customizable in config)
- `[?]` AI_ships_improved (now fully customizable in config)
- `[?]` miniaturization_tweak (split to independent parameters)
- `[?]` planet_size_class (removed)

## 1.50.1

### Game changes

- `[?]` Relaxed end of combat (ships act until end of turn).
- `[?]` TWF-equipped ship's presence doesn't affect normal ships now.
- `[?]` Map edge retreat crash fixed.
- `[?]` Population capacity is clipped to 42 (maximum properly supported by the game).
- `[?]` 43rd android bug fixed.
- `[?]` Pop limit bug for freighted colonists fixed.
- `[?]` Command points not updating when landing colony or outpost ship fixed.
- `[?]` Productivity recalculation after selling a building now works.
- `[?]` Refit displayed cost versus real cost discrepancy fixed.
- `[?]` Feudal wrong refit cost fixed.
- `[?]` Stealth specials now work properly.
- `[?]` Diplomatic blunder and marriage events not working fixed.
- `[?]` Diplomatic relation change overflow fixed.
- `[?]` Improved spy formula (63rd spy receives additional +1 bonus).

### Interface changes

- `[?]` Config extractor built into the executable.
- `[?]` Added parameter mod_name to distinguish mods in main menu.
- `[?]` Added table format to config for customizing in-game text labels.
- `[?]` Added parameter scroll_interval_ms to control scrolling speed in the combat screen.
- `[?]` Added parameter clear_button to control design screen clear button behavior.
- `[?]` Added parameters help.lbx and techdesc.lbx for custom lbxs.
- `[?]` Auto delete trade goods or prompt deletion on < >.
- `[?]` Colony Current Production window now includes bought production.
- `[?]` Reordered techs in Info: Tech Review subscreen.
- `[?]` Alt+MENLO now gives 31000 research points.

### Config changes

- `[?]` Host config is broadcast to clients in network multiplayer.
- `[?]` Config supports string parameters.
- `[?]` Added helper commands include and stop.

### New config parameters

- `[?]` shortened_anomaly_beast_flux, tweaks random events.
- `[?]` pop_base, sets base pop for each planet size.
- `[?]` pop_climate, sets pop bonus for each planet climate.
- `[?]` basic_growth_bonus, sets additional base growth percent.
- `[?]` AI_races_variants, extends the AI races pool.
- `[?]` AI_races_improved, turns on better races.
- `[?]` AI_ships_improved, turns on better ships.
- `[?]` tech, table of all technologies.
- `[?]` tech_field, table of all technology fields.
- `[?]` ground_defences, sets miscellaneous ground defenses traits.
- `[?]` hull, table with ship hull stats.
- `[?]` lander_ships_costs, sets colony ship, outpost ship and transport cost.
- `[?]` stealth, sets stealth bonus per stealth modifier.
- `[?]` scan, sets scan bonus per scan modifier.
- `[?]` spy_tech, sets spy bonus for given spy tech.
- `[?]` spy_govt, sets spy bonus for given government.
- `[?]` alternative_refit, enables alternative refit cost formula.
- `[?]` miniaturization_tweak, enables mini for augs and ext. fuel tanks.
- `[?]` torpedo_miniaturization_missile, sets torps to miniaturize as missiles.
- `[?]` heavy_fighters_can_scramble, allows to target missiles.
- `[?]` absorber_no_discharge, keeps absorber's charge indefinitely.
- `[?]` plasma_web_dissipation_step, sets dissipation to halve instead of -5).**
- `[?]` Added ability to rename weapons, specials, buildings, techs and fields.

### Changed config parameters

- `[?]` Updated mapmod (added ICE, removed GM3 and ICE-COLD).
- `[?]` Updated table weapon, added strategic min and max damage.
- `[?]` Updated table mapgen to latest mapgen 0.41.

### Other

- `[?]` Most of the patch code moved to external library ORION2.BIN.

## 1.50.0.2

### Game changes

- `[n/a]` Plasma Torpedo crash on impact fixed. -- a hotfix of 1.50.0's own torpedo code
- `[n/a]` Crash on malformed build list fixed. -- a bug of 1.50.0 build lists; our parser rejects bad input

### Interface changes

- `[done]` Build list now supports artificial_planet. -- docs/screens/build-lists.md

## 1.50.0.1

### Game changes

- `[n/a]` Post-combat repair crash fixed. -- a hotfix of 1.50.0's own repair code

## 1.50.0

### Game changes

- `[?]` No auto-done in combat for ship without shots.
- `[?]` Colony +10 beam attack bonus bug fixed.
- `[?]` Fighters strike immediately point blank left to right.
- `[?]` Plasma Torpedo instant dissipation bug fixed.
- `[?]` Plasma Torpedo correct dissipation on resolve.
- `[?]` Crew experience level now also applies to capture actions.
- `[?]` Boarding in stasis is no longer possible.
- `[?]` Attackers' raiding party bonus is displayed again (1.3 bug).*
- `[?]` Raid can now destroy last ship system.
- `[?]` Combat repair fixes weapons now.
- `[?]` Repaired shield generator charges shield.
- `[?]` Undocumented +1 hits bonus for buildings during bombardment removed.*
- `[done]` Tech field combat desync fixed. -- docs/net/combat.md
- `[?]` Evolutionary Mutation desync in combat fixed.
- `[done]` Network hang for zero marines board fixed.* -- docs/net/combat.md
- `[done]` Ship cost exchange bug aka feudal bug fixed. -- docs/formats/tech.md
- `[?]` Strategic tech tree corrections.
- `[?]` Artemis strategic one kill to win bug fixed.
- `[?]` Artifacts on-click research bonus bug fixed.
- `[?]` Ships can now be refitted if Cold Fusion tech field is not known.
- `[?]` Outpost ship conflicts colonization bug fixed.
- `[?]` Colony ship conflicts outpost creation bug fixed.
- `[?]` Discrepancy of communications and scanners fixed.
- `[?]` Warp interdictor range now corresponds to its description.
- `[?]` Evolutionary Mutation unclickable picks bug fixed.*
- `[?]` Tolerant races maintenance penalty display fixed.
- `[?]` Navigator skill display bug fixed.*
- `[?]` Telepathic leader's skill level bonus corrected.*
- `[?]` Ordinary assassin skill not working fixed.*
- `[?]` AI personalities racial traits broken logic fixed.
- `[?]` AI fighter size cheat fixed.
- `[?]` Disabled LB's /nohousing +150 growth bonus.

### Interface changes

- `[done]` Load build queue from BUILD.CFG. -- docs/screens/build-lists.md
- `[?]` Faster fade-ins and fade-outs.
- `[?]` Additional shortcuts added.
- `[?]` Design screen clear button behavior changed.
- `[?]` Stored prod is now displayed in colony.
- `[?]` Start of turn reports reordered.

### New config parameters

- `[?]` Added game-affecting LB's switches to config (/picks, etc.).
- `[?]` Added table mapgen (modelled after tf8's mapgen and LB's map switches).
- `[?]` Added mapmod for creating different maps.
- `[?]` steal_mode, controls what spies can and can't steal.
- `[?]` satellite_racial_defense_bonus.
- `[?]` satellite_can_use_torpedoes.
- `[?]` missiles_initiative, toggles missile wait on launch.
- `[?]` missile_glue_old, alternative missile stack merge rule.
- `[?]` fighter_beam_no_plus_one, disables the +1 bonus for energy beams.
- `[?]` fighters_use_pd, enables use of PD mount on fighters.
- `[?]` tractors_to_stop, controls number of tractors per size class needed to stop a ship.
- `[?]` dmg_mount, configures mount and high energy focus bonuses.
- `[?]` damper_kill_ratio, controls damper effectiveness against transporters.
- `[?]` board_in_black_hole, forbids boarding of BHG-affected ships.
- `[?]` speed_base, sets missiles and fighters base speeds.
- `[?]` speed_fast_bonus, sets fast weapon mod speed bonus.
- `[?]` speed_td_bonus, sets missiles and fighters trans-dimensional speed bonus.
- `[?]` weapon, table for configuring stats of all weapons.
- `[?]` special, table for configuring stats of all ship specials.
- `[?]` building, table for configuring stats of all colony buildings.

## 1.50 standard settings (150.CFG)

- `[?]` print_research_value = 3: breakthrough chance shown in Colonies screen
- `[?]` warn_blockers = 0: no warning for housing, trade goods or repeat builds in queue
- `[?]` missile_ammo_x2 = 1: default x2 racks instead of x5 in Ship Design
- `[?]` anim_speed_cloak = 2 4: faster cloak animation
- `[?]` first_pop_to_production = 1: first pop on a new colony always becomes a worker
- `[?]` housing_creates_no_farmers = 1: new pop on a housing colony never becomes a farmer
- `[?]` keep_research_on_finds = 1: RP not reset when the current research is found otherwise
- `[?]` leader_dies_with_no = 1: leader always shouts nooo when his ship is destroyed
- `[?]` event_delay comet 150 (classic 200)
- `[?]` event_delay supernova 150 (classic 200)
- `[?]` event_delay time_space_anomaly 150 (classic 50)
