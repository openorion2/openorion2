#!/bin/zsh
# netup.sh: a fresh network game through the real lobby, the Mac (C, control
# 127.0.0.1:7104) hosting against the Windows machine (W, control through an
# ssh tunnel at 127.0.0.1:7101), each empire played by a network over
# -control (tools/agent -nollm -turncmd tools/aiplay, both on the Mac).
# Both run -netcheck.
#
# Environment: WIN the Windows machine's ssh login; WINDIR its directory with
# openorion2.exe and game\ (the game's files); SESSION_C, SESSION_W
# lobby.session files of two lobby users (kept out of git); POLICY the model.
cd "$(git -C "$(dirname "$0")" rev-parse --show-toplevel)"
: ${WIN:?the ssh login of the Windows machine} ${WINDIR:?its directory, e.g. C:\\oo2test}
: ${SESSION_C:?a lobby.session for the Mac} ${SESSION_W:?a lobby.session for Windows}
POLICY=${POLICY:-models/rl-survival.pt}
w() { ssh -n -o BatchMode=yes $WIN "$1"; }

# Whatever ran before.
pkill -9 -f 'build/openorion2 -game build/netC'; pkill -f 'build/agent -nollm'; pkill -f 'build/aiplay '; pkill -f 'serve.py models/'
# On Windows only what it started: the game from WINDIR (the player's own
# game there stays).
print -r -- "Get-CimInstance Win32_Process | Where-Object { \$_.Name -eq 'openorion2.exe' -and \$_.ExecutablePath -like '$WINDIR\\*' } | ForEach-Object { Stop-Process -Id \$_.ProcessId -Force }" |
  ssh -o BatchMode=yes $WIN 'powershell -NoProfile -Command -' >/dev/null 2>&1
sleep 2
# A newer build goes to Windows (now that nothing there runs it).
# (the stopped game may hold its file for a few seconds).
for i in 1 2 3 4 5; do
  [ build/openorion2.exe -nt build/.netexe-deployed ] || break
  tar czf - -C build openorion2.exe | ssh -o BatchMode=yes $WIN "tar xzf - -C $WINDIR" && touch build/.netexe-deployed || sleep 3
done
[ build/openorion2.exe -nt build/.netexe-deployed ] && echo "openorion2.exe not deployed"

# The Mac's copy of the game.
mkdir -p build/netC
[ -f build/netC/FONTS.LBX ] || cp -R game/. build/netC/
rm -f build/netC/SAVE*.GAM(N) build/netC/SAVE*.LBY(N) build/netC/netcheck-*.GAM(N)
printf '# The online lobby.\nserver: galacticcouncil.openorion2.org:52530\n' > build/netC/lobby.yaml
cp $SESSION_C build/netC/lobby.session

# Windows: the lobby files, a run script, and a task to start it in the
# player's session (a window needs it): dated in the past so that it never
# fires by itself, run now and deleted at once.
tmp=$(mktemp -d)
printf 'server: galacticcouncil.openorion2.org:52530\r\n' > $tmp/lobby.yaml
cp $SESSION_W $tmp/lobby.session
printf '@echo off\r\ncd /d %s\r\nopenorion2.exe -game %s\\game -noupdate -logos off -netcheck -netcheckdir %s -control 0.0.0.0:7001 > %s\\netW.log 2>&1\r\n' \
  $WINDIR $WINDIR $WINDIR $WINDIR > $tmp/run.cmd
tar czf - -C $tmp lobby.yaml lobby.session run.cmd | ssh -o BatchMode=yes $WIN "tar xzf - -C $WINDIR && move /Y $WINDIR\\lobby.yaml $WINDIR\\game\\ >nul && move /Y $WINDIR\\lobby.session $WINDIR\\game\\ >nul"
rm -r $tmp
w "del /Q $WINDIR\\game\\SAVE*.GAM $WINDIR\\game\\SAVE*.LBY $WINDIR\\netcheck-*.GAM 2>nul" >/dev/null 2>&1
w "schtasks /Create /F /TN oo2nettest /TR $WINDIR\\run.cmd /SC ONCE /SD 01/01/2000 /ST 00:00 /IT" >/dev/null 2>&1
w "schtasks /Run /TN oo2nettest" >/dev/null 2>&1
w "schtasks /Delete /TN oo2nettest /F" >/dev/null 2>&1
pgrep -f 'L 7101:127.0.0.1:7001' >/dev/null || nohup ssh -o BatchMode=yes -o ServerAliveInterval=30 -o ExitOnForwardFailure=yes -N -L 7101:127.0.0.1:7001 $WIN > build/tunnel.log 2>&1 &

# The Mac.
nohup build/openorion2 -game build/netC -noupdate -logos off -netcheck -netcheckdir build/netC -control 127.0.0.1:7104 > build/netC.log 2>&1 &
for i in $(seq 1 30); do curl -s -m 3 -o /dev/null http://127.0.0.1:7101/screen.png && curl -s -m 3 -o /dev/null http://127.0.0.1:7104/screen.png && break; sleep 2; done
sleep 8
tools/nettest/lobbyrun.sh 7104 7101
sleep 5
# Each empire's network: tools/aiplay with serve.py, run by its agent.
T="build/aiplay 'cd tools/aiplay && ../../build/venv/bin/python serve.py $POLICY rqjd 0.5 sample'"
nohup build/agent -nollm -turncmd "$T" -name C -log build/agentC.jsonl 127.0.0.1:7104 >> build/agentC.log 2>&1 &
nohup build/agent -nollm -turncmd "$T" -name W -log build/agentW.jsonl 127.0.0.1:7101 >> build/agentW.log 2>&1 &
echo up
