# Porting a 1.51 feature

How a feature of the community patch 1.51 comes into OpenOrion2, one at a
time, straight into master.

## Sources

- The patch: `MOO2-1.51.zip` (the maintainer has it; unpack it outside
  the repo). `patch/CHANGELOG_150.TXT`, `patch/150/docs/PARAMETERS.CFG`,
  `patch/150/docs/config.json` (every config parameter with its
  default), `patch/150/mods/150i/150.CFG` (the "1.50 standard" settings).
- The manual, the best description: moo2mod.com/manual/MANUAL_150.html
  (the same text as `MANUAL_150.PDF` in the zip).
- `ORION150.EXE`: a Watcom LE like the original, but with no debug
  symbols; the patch's own code is a blob appended after the LE image
  (file offset 0x1e30b1 on), without names. Reading it is slow: behaviour
  comes from the manual, and where the manual is silent and it matters, a
  run of the real 1.51 in DOSBox (dosbox-x) on a scratch copy of the game
  files settles it.
- `inventory.md`: the patch by subsystem; `items.md`: every changelog line
  with its fate.

## Steps

1. Pick the feature from `items.md`: its lines marked `[port]`. Only the
   maintainer marks `[port]` and `[skip]`; `[n/a]` (with the reason after
   ` -- `) when our port already behaves so or the patch broke and fixed it
   itself.
2. Branch from master: `p151-<topic>`.
3. Read the manual's section and the changelog lines; check what 1.40b23
   and our port do now (`docs/`, the code, `build/orion2.dis` in gomoo2).
4. Write it. Rules:
   - comments say `community patch 1.51`;
   - a 1.51 rule replaces the 1.40 one, no switch: one game;
   - no strings built into the game: new text comes from our own LBX
     files or not at all;
   - files the player edits live in the game's directory, written from
     embedded defaults when missing.
5. Test it: unit tests and a headless test on the real screens; break the
   code once to see the test fail.
6. Document it, in the same change:
   - the spec for developers under `docs/screens/`, `docs/turn/`, ...;
   - the player's chapter in `docs/manual/` (what changed from 1.40b23,
     how to use it, examples, "Compared with 1.51"), and new keys in
     `docs/manual/keyboard.md`;
   - `items.md`: the lines `[done] ... -- <doc>`.
7. Check: gofmt, `go vet`, the wasm build, `go vet -tags aitrain`, the
   whole test suite, then `go test -race` on the packages touched (a
   separate command, before any push).
8. Merge into master. Pushing, releases and deploys only when the
   maintainer says so.

## Done so far

- Build lists and taking a purchase back: `docs/screens/build-lists.md`,
  `docs/manual/build-queue.md`.
