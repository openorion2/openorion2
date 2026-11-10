# Community patch 1.51: inventory

What the unofficial MOO2 patch 1.50.0 – 1.51 (DOS, `ORION150.EXE`, Oct 31
2026) changes, sorted by subsystem, with a proposed decision for each item.
Sources: `CHANGELOG_150.TXT`, the patch manual (`MANUAL_150.PDF`, same text
at moo2mod.com/manual/MANUAL_150.html), `PARAMETERS.CFG`, `config.json`
(426 parameters with defaults), `150/mods/150i/150.CFG`.

## What "1.51" means here

The patch is three things in one exe:

1. **Fixes and rule changes on by default.** This is the "1.50 standard"
   game mode: exe defaults plus the few settings in `150.CFG` (listed
   below). This is the porting target.
2. **Config parameters** (426, OCL-style). Almost all default to classic
   behavior and exist for mods. Not ported now; a few may come later as
   game options.
3. **Mods, Lua scripting, the launcher, mapgen switches, LBX overrides.**
   Not ported.

The patch is built on DOS 1.31 with Lord Brazen's 1.40 incorporated, so
its "classic" means 1.31/1.40; our base 1.40b23 is the same game for
almost every item. Items marked `?` need a check against our port.

`ORION150.EXE` is a Watcom LE like the original and keeps the original's
2826 symbol names; the patch's own code has no names. Original routines
can be diffed by name against 1.40b23; new code has to be read raw.

`150.CFG` (1.50 standard) changes from exe defaults:
`print_research_value = 3`, `warn_blockers = 0`, `missile_ammo_x2 = 1`,
`anim_speed_cloak = 2 4`, `first_pop_to_production = 1`,
`housing_creates_no_farmers = 1`, `keep_research_on_finds = 1`,
`leader_dies_with_no = 1`, `event_delay` comet 150 (200), supernova 150
(200), time_space_anomaly 150 (50).

## Legend

- **R**: rule change or rule fix, on by default in 1.51. Port behind the
  1.51 ruleset (classic 1.40 stays bit-exact).
- **U**: interface only, no effect on game state. Port, always on or
  as a setting.
- **C**: config parameter, default classic. Skip for now.
- **G**: new or changed graphics/text in LBX files. Later, separately.
- **X**: not applicable (DOS, launcher, config machinery, Lua, mods).
- **?**: check whether our port or 1.40b23 already behaves this way.

Version in brackets is the patch version that introduced the item.

## Crashes, freezes, stalls

Our port is a Go reproduction: a crash in the original is a panic or a
hang in ours, so these come first regardless of ruleset where the fix
does not change outcomes.

- R Council: win the election, then reject the ruling, crash [1.51]
- R "Mark Block Stack Size Exceeded" in Races screen (leak) [1.50.4] `?`
- R Star system popup freeze, corrupted planet animation [1.50.5] `?`
- R Random sounds crash (F keys in fleet/system/colony view) [1.50.7] `?`
- R AI food balancing freeze (liberated blockaded planet, Fungi) [1.50.10]
- R Serendipity with no tech field selected freezes [1.50.15]
- R Map-edge retreat crash (frigate, left edge) [1.50.1]
- R Driveless trans-dimensional ships: immobile, wrong placement, rotate,
  crash on win, crash on scan [1.50.2]
- R Fighters resolve top-left corner Beam_SFX crash [1.50.6]
- R Ship internals division by zero (no systems, drive 1 HP) [1.50.7]
- R Raid report too long crash; message box too much text crash [1.50.7]
- R Crystal captures ship with missiles/fighters out; Get_Missile_Hits
  returns 0 and AI divides by it [1.50.21]
- R Tractor crash in big battles [1.50.19]; >127 ships overflow [1.50.22]
- R Combat crash when AI-vs-AI combat runs mid human combat (net) [1.50.18]
- R Crystal-captured ships crash on combat load [1.50.12]
- R Non-initiative combat freeze when all opponents are in stasis [1.50.12]
- R Bomb hits slow-down/crash with flyers [1.50.12]
- R Leaders screen: for-hire leader with tech, crash on click [1.50.12]
- R Fleets screen: right arrow with no fleets freezes [1.50.12]
- R Fleet leaders screen crash scrolling fleet pane [1.50.7]
- R Ship with 1 move point losing speed under ICS, negative moves [1.50.7]
- R Ship destroyed by BHG with initiative off crash [1.50.7]
- R No-rifle ground invasion sound crash (modded only) [1.50.7] X
- R German: request to break treaty crash [1.50.13] X (no German text yet)
- R Italian jimtext.lbx crash [1.50.8] X
- R Alt+X switching to defeated empire crash [1.50.9] `?` (cheat)
- R Malformed build list crash [1.50.0.2] (with build lists)
- R Post-combat repair crash; Plasma torpedo impact crash [1.50.0.x]
- R Fixed stall on engine explosions in super-fast mode [1.50.5]
- R Freeze: ship tractored at end of combat [1.50.3.2]
- R Repeat+android cancel leaves bare repeat, adding to it freezes [1.50.10]

## Network desyncs (our netplay runs both sides' engines)

- R Tech field gained on battle turn: combat desync [1.50.0]
- R Evolutionary Mutation on battle turn: desync [1.50.0]
- R Zero-marine boarding (incl. Damper Field) desync [1.50.0]
- R "Monster bug": identical monsters vs two players, rollback [1.50.4]
- R Shuttle capture of rotating ship desync [1.50.4]
- R Shuttle capture of last ship mid-flight stall [1.50.11]
- R Dismissed leader level-up not synced [1.50.6]
- R Artemis net damage desync [1.50.19]
- R >99 ships tactical combat freeze [1.50.22]
- R "Feudal bug": ship cost computed in another player's context
  (hotseat and net) [1.50.0, 1.50.4.2]
- R Race pick costs sync when loading net game [1.50.4]
- R Cluster galaxy works in multiplayer [1.50.2]
- U Leaders screen in MP defaults to colony leaders [1.50.10]

Our netplay is our own lockstep, so several of these may not apply; each
needs a look at whether our engine reproduces the divergence.

## Tactical combat

Turn structure and initiative:
- R Initiative = BA + speed*10 (was trunc(BA/10)+speed); strict tie
  order: crew, crew level, combat-only, combat ID, crew XP, ship ID
  [1.50.4]
- R TWF turn structure rewritten (table in manual): TWF turn is a full
  turn, non-TWF ships skip it; recharge before retreat [1.50.4]
- R Defensive fire: off by default with initiative (`defensive_fire = 0`),
  always on without; numerous DF bug fixes [1.50.0–1.50.8]
- R Relaxed end of combat (ends on each next ship if won and no enemy
  missiles/colony) [1.50.1]; stops when remaining side is AI/AUTO
  [1.50.12, 1.50.22]
- R No auto-done for ship without shots/moves [1.50.0]
- R Monster goes first on tied initiative [1.50.19]
- R Absorber keeps charge in initiative games; max stored 32767; range 24
  [1.50.4, 1.50.18.3, 1.50.19]
- U Hotkey H: all ships wait; defender's H outwaits attacker [1.50.24]
- U No center on next ship option (C) [1.50.16]

Weapons and devices:
- R Plasma torpedo dissipation fixes [1.50.0, 1.50.7.2]
- R Fighters strike point blank in all directions [1.50.0]
- R Fighters chase only related ships, chase non-moving missiles,
  highlight indirect targeting [1.50.3, 1.50.4]
- R Fighter bays: returning flights fill bays properly [1.50.6]
- R Fighters get Helmsman and Ordnance [1.50.4, 1.50.6]
- R Heavy fighters spend ammo on missiles [1.50.5]
- R Missile alerts: rotations trigger, cost computed right, no
  interceptor moves when alert stops ship, no lingering alert [1.50.4–5]
- R Mid-flight ship coordinates update (PD, Spatial, DF ranges) [1.50.4]
- R Spherical weapons: targeting range = damage range, edge distance
  [1.50.4]
- R Destroyed weapon loses shot immediately [1.50.2]
- R Specials +1 HP (indestructible special bug) [1.50.6]
- R Bombs and A-M Rockets hit points (no missile-space bonus) [1.51]
- R Tractor: yellow/red no longer disables beam next turn [1.50.2]
- R Stasis: recharge, release mid-turn acts, shots not zeroed, disables
  WAJ/dissipator/scout lab, stops tractors/BHG [1.50.3, 1.50.4]
- R BHG: one shot at a time; simultaneous kills; destroyed BHG/stasis
  frees correctly [1.50.2, 1.50.3, 1.50.6]
- R Multi-phased shields lose strength when destroyed [1.50.24]
- R Cloaking device: miss 50% (49%), broken cloak decloaks [1.50.3,
  1.50.4]
- R Phasing cloak: no area damage, cuts tractors, no boarding [1.50.2–3]
- R Displacement device 30% vs missiles and absorber [1.50.6]
- R Inertial nullifier rotation does not pull missiles [1.50.4]
- R Sub-space teleporter range and tractor break fixes [1.50.2]
- R Stellar converter fires whole slot [1.50.4]
- R Lightning field kills 1/2 missiles (roll fix) [1.50.2]
- R Repairs: weapons, shield generator, specials resume; drive minimum
  mobile repair [1.50.0, 1.50.3]
- R Warp dissipator repaired blocks earlier retreat orders [1.50.4]
- R Immobilization label consistent with boardability [1.50.2]
- R Energy absorber AI expected damage [1.50.23]
- R Anti-missile weapons auto-fire fixes [1.50.6, 1.50.8, 1.50.21.1]

Boarding:
- R Crew level applies to capture, not just raid [1.50.0]
- R Raid can destroy last system and a 1 HP drive [1.50.0]
- R Raid report: damaged is not destroyed; no misleading counts [1.50.6]
- R No boarding in stasis [1.50.0]
- R Capture clears done/wait states [1.50.4]
- R Shuttle capture on retreat: ship stays [1.50.7]
- R Leader en route to captured ship goes to pool [1.50.5]
- R AUTO/AI extra boarding fixes [1.50.22, 1.50.26]
- R Warlord +1 level after refit fix [1.50.26]

Planets and bombing:
- R Symmetric bombing area [1.50.2]; corner area and planet sizes back to
  classic 3-4-6-7-8 [1.50.11]
- R Ground installation and civilian +1 HP removed [1.50.0, 1.50.10]
- R Planet shield damage-blocked display fix [1.50.2] (U)
- R Colony +10 BA bonus without satellite [1.50.0]
- U Planet drawn centered in bombing area [1.50.11]

AI in combat:
- R Expected damage: ion cannon vs planets, mods, ground shields,
  ordnance, absorber, damper, hard shields, cloak, nebula [1.50.8–10]
- R AI stall targeting missile from captured ship [1.50.2]
- R AUTO no longer retreats just for lack of ammo; leader on board
  keeps ship [1.50.8, 1.50.21]
- R AI no self-destruct when boarding in black hole is off [1.50.7]
- R AI double Augmented Engines +5 [1.50.10]

Super-fast mode (Z):
- R Ship explosions taken into account [1.50.4]
- U No boarding popups, no sound [1.50.4, 1.50.7]

Interface:
- U Disabled weapon slots red even if spent [1.50.4]
- U Status bar shows active ship between turns and on retreat [1.50.5]
- U Tractor beam drawing removed when target destroyed [1.50.9]
- U Missiles no longer face away from target [1.50.7]
- U Scroll interval (we already pace scroll: v0.9.5) [1.50.1]

## Strategic combat

- R Weapon selection and damage both use the strategic column; new
  values for special weapons (table in manual) [1.50.7]
- R Strategic doom star designed as strategic [1.50.2]
- R Artemis net "defender wins" bug [1.50.0]
- R Hard shields in strategic combat (added 1.50.2, removed 1.50.19)
- R Missiles upgrade to best in strategic [1.50.11]
- R Strategic tech tree: remove A-M rockets, absorber, security
  stations, warp dissipator; add warp interdictor [1.50.0]
- C Crew bonus in strategic (`strategic_xp_bonus`, default off)
- U Strategic ship scan shows structure, shields as reduction [1.50.2]

## Ships, fleets, refit, landers

- R Auto-designed ships = manual designs (speed from free space,
  megafluxers, shield space, bomber bays space, pods/tanks from table,
  no Hv on beams without Hv) [1.50.2, 1.50.7.1]
- R No computer, no computer HP [1.50.6]
- R Warlord bonus from turn of build [1.50.6]
- R Destroyed stealth devices lose effect on main screen [1.50.1]
- R Antaran fighters on defensive cruiser/titan; Antaran/Avenger bombs
  360 arc [1.50.2, 1.50.8]
- R Contact range rounds up like visit range [1.50.6]
- R Tachyon and sub-space communications +1 range [1.50.0]
- R Navigator affects only its fleet, immediately; split/leave warnings
  [1.50.7]
- R Other player's dimensional portal unusable [1.50.2]
- R Auto-scrap: right number of ships; leader moves to another ship
  [1.50.7]
- R Ship scrap total overflow [1.50.7]
- R 6th design slot corruption at 500 ships limit [1.50.7, 1.50.10]
- R Jump gate fixes (redirect, black hole, speed) [1.50.7–1.50.17]
- R Unstable wormhole for eta 2 [1.50.10]
- R Refit without Cold Fusion in pre-warp [1.50.0]
- R Refit cost shown = cost charged; Feudal refit cost [1.50.1]
- R Lander ships get best armor (no "best minus 1") [1.50.6]
- R Landing CS/OP updates command points [1.50.1]
- R Outpost ship vs colony ship conflicts [1.50.0]
- R Ship design overflow: can't build a design that uses too much space
  [1.51]
- R Fighter bays allowed in post-warp [1.50.14]
- U 6th ship design slot [1.50.2] (save layout: check)
- U ALT-AD autodesign all slots [1.50.x]
- U Ruola shown in refit popup [1.50.24]
- U Ship info: racial MD not halved, TD speed shown, lander/strategic
  junk values hidden [1.50.2]
- U Design dock: BA includes battle scanner, Aug engines not double in
  BD [1.50.x]
- U Fleets screen: scrollbar top, filters work, green ships, officers
  scrollbar, stealth giveaways [1.50.7, 1.50.12]
- U Enemy moves setting works [1.50.14]
- U Hyperspace beast on main screen [1.50.19]
- U F9 distance across black hole [1.50.25]
- U Doom star race name in build screen; Avenger/Antaran arcs [1.50.25]

## Satellites, ground defenses, troops

- R Satellite auto-design rework: mods on beams, mods cost space,
  missiles spread, 99 per stack, arc cost 50%, more space, new
  allocation 26/13/12/49, hull rows 6-8 for HP [1.50.2, 1.50.7]
- R Satellite can't be destroyed by bombing/earthquake [1.50.9]
- R Rare satellite capture bug [1.50.4]
- R Ground batteries arc cost +50%, space +50% [1.50.7]
- R Subterranean max unit dies and rebuilds [1.50.4]
- R Barracks/no barracks excess troops [1.50.4]
- R Fighter garrison counts bomber bays [1.50.19]
- R Shield not installed on starbase with damper [1.50.5]
- U AI troop bonus added to sum, commando leader listed [1.50.7]

## Production, build queue, economy

- U **Build lists**: keys Q, 0-9 load BUILD*.CFG into the queue; in
  Colonies screen for one colony or with SHIFT for all; modes append,
  prepend, merge; commands buy, buy10%, buy50%, buy75%, unbuy, clear,
  clear_refits, stop [1.50.0, 1.50.4, 1.50.17]. Default lists in
  `150/build/`.
- U Revert buy (BUY again) [1.50.4]
- R **AUTO build / AI building scores** (`building_score` table) [1.51]
- R Autobuild does not recalculate colony (new pop idle) [1.50.11]
- R Build queue bugs: repeat swap cheats, double repeat, swap under
  repeat, buy then swap [1.50.4, 1.50.7]
- R Colony base button recount (others' outposts) [1.50.4]
- R No buying on blockaded colony via list; prepend vs bought [1.50.5]
- R Free cash on building/spy completion removed; freighter bonus 0
  [1.50.3, 1.50.8]
- R Selling building: productivity recalculated; bought product refunded
  [1.50.1]
- R Captured colony: AI bought prod zeroed [1.50.3]
- R One dimensional portal per system [1.50.19]
- R Food replicator can't make production negative [1.50.16]
- R Humans can't build spies without spy network; capitol without
  capitol tech [1.50.5]
- R Capitol rebuildable immediately when homeworld bombed [1.50.18]
- U Loading a list updates income immediately [1.50.17]
- U Colony screen: stored/total production [1.50.0]
- U Colonies screen: summary window (enroute, growth, production) G for
  strings [1.50.15]; freighted pop [1.50.22]; food shortage shown
  [1.50.20]
- U Colonies screen SHIFT semantics [1.50.15]
- U Build screen junk info removed (repeat, trade goods, housing) [1.50.15]
- U Housing/trade goods warning: keep by default (`warn_blockers = 0`)
- U Auto-delete trade goods works with , and . [1.50.1]
- U Food shortage shown for colonies with no food [1.50.20]
- U Time-space anomaly: no grain/pollution barrels [1.50.23]
- C `prod_format` / `font_color_group` coloring of build list

## Research and technology

- R Turn 0 research points kept [1.50.3]
- R Advanced damage control removed for cybernetic [1.50.4]
- R Hyper-advanced level 1 cost 25k (was 15k real) [1.50.9]
- R Shield tech from scrapped ships; blank message fix [1.50.6]
- R Surrender gives all techs (not only up to #83); map knowledge
  [1.50.6, 1.50.12]
- R Human-to-human tech trade ignores AI preferences [1.50.8]
- R Artifacts on-click research bonus [1.50.0]
- R Breakthrough + serendipity blank box [1.50.5]
- R `keep_research_on_finds = 1` (150.CFG)
- R Uncreative keep_on_find bugs [1.50.6]
- R Loknar: shown techs; can't give tech 0 [1.50.7, 1.50.24]
- R GNN plague 0 RP [1.50.7.2]
- U Breakthrough turn counter precise [1.50.9]
- U Tech review placement [1.50.1]
- U Techs from scrapping listed once [1.50.7.1]
- U Leader tech shown when broke and on Hire [1.50.10]

## Spying

- R Spy roster overflow pays half cost (not 100 BC) [1.50.x]
- R Spy threshold below 60 [1.50.3]
- R "No place for new spy" exploit [1.50.2]
- R Ordinary assassin kills spies [1.50.0]
- C Improved spy formula, spy bonuses (optional since 1.50.19)

## AI and diplomacy

- R AI objective uses real race pick values [1.50.x]
- R AI personalities racial traits logic [1.50.0]
- R AI fighter size cheat removed [1.50.0]
- R Diplomatic blunder and marriage events work [1.50.1]
- R Relation change overflow [1.50.1]
- R Bought prod after capture (see production)

## Population

- R Assign to farming from non-food world [1.50.2]
- R Population clipped to 42; 43rd android [1.50.1]
- R Freighted colonists ignore pop limit [1.50.1]
- R Ghost colonies can be bombed/invaded; starvation option C;
  zero colony rules [1.50.2, 1.50.15, 1.50.17]
- R `first_pop_to_production = 1`, `housing_creates_no_farmers = 1`
  (150.CFG)
- R Terraforming with arrivals [1.50.6]
- R Food base fix for planets (food 0.5 scale is a config rewrite, C)

## Leaders

- R Random order of players rolling for leaders [1.50.12]
- R Telepath leader level; telepath blocks mind control, assassin does
  not [1.50.0, 1.50.7]
- R Galactic lore: no planet defenses, Planets screen filter only
  visited [1.50.9, 1.50.24]
- R Navigator display fix [1.50.0] (U)
- R Fired leader XP with modded levels (C only)
- R `leader_dies_with_no = 1` (sound) (150.CFG)
- U Leader experience points shown [1.50.18]
- U Hire window 2 only on Tutor [1.50.14]

## Events and GNN

- R Diplomatic blunder/marriage, pirates overflow [1.50.1, 1.50.5]
- R Comet destroyed at 100% [1.50.7]
- R Comets/monsters not during flux [1.50.6]
- R Monster search range +5 pc [1.50.7]
- R Amoeba toxic for AI; stays if target gone [1.50.6]
- R Nova: food zeroed, enrichment removed [1.50.x]
- R Unstable wormhole eta 2 [1.50.10]
- R Event delays from 150.CFG: comet 150, supernova 150, anomaly 150
- G GNN messages: colony names, monster reports (EVENTMSE.LBX)

## Galaxy generation (new game)

- R Home systems minimum 3 planets [1.50.9]
- R No wormhole to Orion; black hole in nebula replaced properly;
  closest star fixes; trim_planets fixes; food world fixes; in-nebula
  check [1.50.9–1.50.23]
- R Initial autosave after mapgen [1.50.4.1]
- R Random number of opponents (needs NEWGAME.LBX art, G) [1.50.3]
- C Integrated mapgen switches and map mods

## New game, settings, saves

- U Defaults: tactical combat ON, ship initiative ON, legal moves ON
- U Autosave every turn before combat in single player [1.50.3]
- U Cutscene sound if music or SFX is on [1.50.2]
- U Faster fades [1.50.0]; PageUp/PageDown animation speed [1.50.7.1]
- U Hall of fame reset ALT-C
- U Cheats: warning always issued; MENLO amount (C)
- X Saves: 1.50 appends extra data (config, Antaran state, freighted
  pop). Reading 1.51 saves is a separate decision.

## Interface: other screens

- U Next turn report order: spy steal before tech select; exploration
  before colonization; hire after exploration [1.50.0]
- U Race statistics: up to 20 traits, half-point bonuses [1.50.3, 1.50.12]
- U Race selection: too many traits [1.50.7]
- U History graph keeps eliminated empires [1.50.9]
- U Planets screen: Low-G -25% for Heavy-G; artifact status [1.50.9,
  1.50.14]
- U Star system popup: colony ship count, palette green, stealth tooltip
  [1.50.2, 1.50.12]
- U Minimaps: colonized brown stars [1.50.7]
- U Evolutionary mutation unclickable picks [1.50.0]
- U Tolerant maintenance penalty, half food display [1.50.0, 1.50.2]
- U Population graph includes freighted [1.50.24]
- U Hotseat GNN, hotseat right-click after autosave [1.50.6]

## Graphics and text (G)

New game pictures (cluster, random opponents, post-warp, centered 7),
Darloks button, colony screen tiny planet colors, COMBAT.LBX grey wait
(150i only), DESIGN.LBX x3 missile (150i only), INFO.LBX ICE menu,
English text fixes (HELP, HESTRNGS, MAINTEXT, RSTRING0, SKILDESC,
TECHDESC, TECHNAME), ESTRINGS (colonies summary strings, hire texts),
EVENTMSE (GNN). Our port reads the player's own files; shipping patched
LBXs is a separate decision.

## Not ported (X)

Launcher, DOSBox config, `ORION2.CFG`/`USER.CFG`/`ENABLE.CFG` machinery,
426 config parameters (except the defaults they encode), ALT-RE/RI/EEE,
EXTRACT.CFG, Lua scripting and MAIN*.LUA hotkeys 0-9 on the main screen,
mods (150i, 150m, ICE, VDC, Melee, mirror, randtech, 120), map mods,
multibuild, `watch-save10.bat`, command line switches.
