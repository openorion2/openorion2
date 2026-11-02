#!/bin/zsh
# lobbyrun.sh HOSTPORT JOINPORT: from the main menu (games under -control),
# the host creates a lobby game, the other joins, both start. The clicks are
# relative to the host's MOX.SET (build/netC's: Hard, small, 5 players,
# tactical combat on): galaxy size and players once and twice on, tactical
# combat off unless TACTICAL=1, difficulty to Tutor with TUTOR=1. 2 humans
# and AIs: with 2 of 2 the host starts as the other joins, and the joiner
# waits ("Waiting to join game") until the host has picked race, name and
# banner (msg 0); the clicks below assume START.
H=$1 J=$2
c() { curl -s -X POST "http://127.0.0.1:$1/click?x=$2&y=$3" >/dev/null; sleep ${4:-1.5}; }
k() { curl -s -X POST --data-urlencode "name=$2" "http://127.0.0.1:$1/key" >/dev/null; sleep 1.5; }
c $H 490 250; c $J 490 250 2.5
c $H 315 306; c $J 315 306 4
c $H 98 405 2; c $H 262 238 2.5; c $H 260 312 3
c $H 325 156 1; c $H 170 296 1; c $H 170 296 1
# TACTICAL=1 keeps the MOX.SET's tactical combat on; else it goes off.
[ "$TACTICAL" = 1 ] || c $H 466 279 1
# TUTOR=1: the difficulty from Hard (the MOX.SET's) on to Tutor, a longer game.
[ "$TUTOR" = 1 ] && { c $H 170 156 1; c $H 170 156 1; }
c $H 480 403 4
c $J 319 405 4; c $J 200 95 3; c $J 248 332 10
c $H 315 284 6
c $H 410 351 2; c $J 537 255 3
# Until each side's game has begun: a banner (when asked), then the ruler's name.
sd() { curl -s -m 5 http://127.0.0.1:$1/state | python3 -c 'import json,sys
try: d=json.load(sys.stdin).get("stardate") or ""; print("" if d in ("", "0.0") else d)
except Exception: print("")'; }
for i in $(seq 1 40); do
  done=1
  for p in $H $J; do
    [ -n "$(sd $p)" ] && continue
    done=0
    c $p 385 190 1; c $p 320 238 1; c $p 260 190 1; c $p 320 238 1
  done
  [ $done = 1 ] && break
  sleep 2
done
