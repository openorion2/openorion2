#!/bin/zsh
# netwatch.sh PORT...: a status line a minute into build/netwatch.log; "EVENT" lines on a desync, a panic or a stall.
cd "$(git -C "$(dirname "$0")" rev-parse --show-toplevel)"
typeset -A last since
while true; do
  line="$(date +%T)"
  for p in "$@"; do
    st=$(curl -s -m 5 http://127.0.0.1:$p/state | python3 -c 'import json,sys
try:
  s=json.load(sys.stdin); p=s.get("player",{})
  print(s.get("stardate"), s.get("screen"), len(s.get("colonies",[])), "contact" if p.get("others") else "-")
except Exception: print("down")' 2>/dev/null)
    line="$line | $p: $st"
    sd=${st%% *}
    if [ "$sd" = "${last[$p]}" ]; then since[$p]=$(( ${since[$p]:-0} + 1 )); else since[$p]=0; last[$p]=$sd; fi
    [ ${since[$p]} -ge 5 ] && echo "EVENT stall $p at $sd for 5 min"
  done
  echo "$line"
  for f in build/netC.log; do
    [ -f $f ] && grep -h 'DESYNC\|^panic\|fatal error' $f | sed "s|^|EVENT $f: |" | sort -u
  done
  sleep 60
done
