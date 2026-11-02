#!/bin/zsh
# netloop.sh: game after game of netup.sh (the Mac against the Windows
# machine through the lobby); each run's evidence (logs, both sides'
# netcheck saves, screens) into build/netruns/N when it crashes, desyncs,
# stalls or after 90 minutes. WIN, WINDIR as netup.sh's.
: ${WIN:?the ssh login of the Windows machine} ${WINDIR:?its directory, e.g. C:\\oo2test}
cd "$(git -C "$(dirname "$0")" rev-parse --show-toplevel)"
mkdir -p build/netruns
n=$(ls build/netruns | wc -l | tr -d ' ')
while true; do
  n=$((n+1)); d=build/netruns/$n; mkdir -p $d
  : > build/agentC.log; : > build/agentW.log
  # Odd runs at Tutor (long games), even ones at Hard (the AIs attack early).
  # Tactical combat in two runs of four (the lockstep between the machines).
  TUTOR=$((n % 2)) TACTICAL=$((n % 4 / 2)) tools/nettest/netup.sh > $d/netup.log 2>&1
  pkill -f netwatch.sh; nohup tools/nettest/netwatch.sh 7104 7101 > $d/netwatch.log 2>&1 &
  start=$(date +%s); why=time
  while [ $(( $(date +%s) - start )) -lt 5400 ]; do
    sleep 30
    if grep -q 'EVENT.*DESYNC' $d/netwatch.log; then why=desync; break; fi
    if grep -q 'EVENT.*panic\|EVENT.*fatal' $d/netwatch.log; then why=panic; break; fi
    if grep -q 'EVENT stall' $d/netwatch.log; then why=stall; break; fi
    # A side out of the game twice in a row (an empty stardate): it ended
    # (an eliminated player; the other then plays on alone, game type 0).
    if [ $(tail -2 $d/netwatch.log | grep -c ': [ ]') = 2 ]; then why=ended; break; fi
  done
  ssh -n -o BatchMode=yes $WIN "type $WINDIR\\netW.log" > $d/netW.log 2>/dev/null
  over=$(cat build/netC.log $d/netW.log 2>/dev/null | grep -a 'game over\|game ends\|goes on alone\|DESYNC' | LC_ALL=C tr -d '\r' | sort -u | head -4 | tr '\n' ';')
  lvl=hard; [ $((n % 2)) = 1 ] && lvl=tutor; [ $((n % 4 / 2)) = 1 ] && lvl=$lvl,tactical
  echo "$(date +%T) run $n ($lvl): $why $over" | tee -a build/netruns/summary.log
  cp build/netC.log build/agentC.log build/agentW.log $d/ 2>/dev/null
  cp build/netC/netcheck-*.GAM $d/ 2>/dev/null; rm -f build/netC/netcheck-*.GAM
  for f in $(ssh -n -o BatchMode=yes $WIN "dir /b $WINDIR\\netcheck-*.GAM" 2>/dev/null | LC_ALL=C tr -d '\r'); do
    ssh -n -o BatchMode=yes $WIN "type $WINDIR\\$f" > $d/W-$f 2>/dev/null
  done
  for p in 7104 7101; do curl -s -m 5 -o $d/screen-$p.png http://127.0.0.1:$p/screen.png; done
done
