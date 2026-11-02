# Network play between two machines, played by networks

Scripts to test network play for crashes, hangs and desyncs: the Mac hosts
a lobby game against a Windows machine, each empire played by a trained
network over `-control` (`tools/agent -nollm -turncmd` with
`tools/aiplay`, both agents on the Mac; `docs/aiplay.md`), which
also answers the screens and presses TURN; both games run `-netcheck`.

- `netup.sh` — one game through the real lobby, from nothing to turns.
  Environment, required: `WIN` the Windows machine's ssh login, `WINDIR`
  its directory, `SESSION_C` and `SESSION_W` lobby.session files of two
  lobby users (out of git); `POLICY` the network (`models/rl-survival.pt`).
- `lobbyrun.sh HOST JOIN` — the clicks from the main menu to the game.
- `netwatch.sh PORT...` — a status line a minute, `EVENT` lines on a
  desync, a panic or a stall.
- `netloop.sh` — game after game (odd runs at Tutor, tactical combat in
  two of four); a run ends on a desync, a panic, a stall, a side out of
  the game or after 90 minutes. Each run's evidence goes into
  `build/netruns/N`, a line per run with why it ended into
  `build/netruns/summary.log`.

The Mac needs `build/openorion2`, `build/agent`, `build/aiplay` and a
Python with torch in `build/venv`. The Windows side needs ssh and `WINDIR` with `openorion2.exe` and
`game\`; `netup.sh` copies a newer `build/openorion2.exe` there. Its control port is reached through an ssh tunnel
(127.0.0.1:7101), and the game starts in the logged-in player's session
through a scheduled task dated in the past (it never fires by itself), run
and deleted at once.

For a lobby without accounts, `tools/lobbyfake` and `ca:` in `lobby.yaml`.
