# Network play in the browser

The browser build (`docs/web.md`) cannot listen on a
socket or dial TCP, so network games reach it through the server that
serves the build: `cmd/webserve` answers WebSockets at `/net` and does the
TCP side for the browser. Everything is port-only; the game above the
`Net_*` layer is unchanged, and so are the frames (`transport.md`).

## Design

```
browser (wasm)                cmd/webserve                       native openorion2
TCP{Web: ws://host/net}  ──ws──  /net?create=:52525 ── hosted relay :52525 ──tcp── -join host:52525
                         ──ws──  /net?join=h:p      ── bridge ──tcp──────────────── -host :p (or a hosted relay)
```

- **One byte stream, the same frames.** A WebSocket binary message carries
  bytes of the usual stream (u32 length, u16 id, from, to, payload); the
  browser sends one frame per message. On the server the WebSocket is a
  `net.Conn` (`netplay.AcceptWebSocket`), so the relay and the bridge
  handle it like a TCP link: handshake, pings, deadlines, drops.
- **Create** (`/net?create=:PORT`): the server starts a *hosted relay*
  (`netplay.HostRelay`): a relay with no station of its own, listening on
  TCP PORT. The browser's link is served as its creator
  (`Relay.ServeCreator`): its Hello has kind 2 (create) and names the game,
  it gets net id 1, and its `SetOpen` goes to the relay as control frame
  0xff07 (`Net_Enable/Disable_New_Players_`). Joins are refused until the
  creator is in. As with `-host`, the relay outlives its creator and closes
  once the last station has left (at once if the creator never got in).
- **Join and the game list** (`/net?join=HOST:PORT`): the server dials
  HOST:PORT and bridges. It first reads the browser's Hello, sends only that,
  and passes anything on only once the far end has answered with a Welcome
  or a Refuse, so `/net` reaches netplay relays and nothing else. Its dial
  and wait take at most 7.5 s, inside the browser's 10 s handshake, so a
  failure arrives as a Refuse with the reason. After the handshake both
  directions are copied as they are; a lost end closes the other. The
  query Hello (the game list) goes the same way.
- **Mixing.** A hosted relay is an ordinary TCP relay to native players
  (`-join server:PORT`), and the bridge reaches any native `-host`, so
  browsers and native machines play in one game either way round. The
  addresses a browser types are as seen from the server.
- **Browser side** (`internal/netplay/wsdial_js.go`): `dialWebSocket`
  wraps the browser's `WebSocket` (`binaryType` arraybuffer) as a
  `net.Conn`. Its JS callbacks only queue bytes and signal; `Read` waits on
  the signal with the read deadline, `Write` calls `send` (the browser
  queues, so write deadlines are not enforced). The native build's
  `dialWebSocket` is `netplay.DialWebSocket`, a small RFC 6455 client used
  by the tests.
- **`TCP.Web`.** The transport stays `netplay.TCP` so the create and join
  boxes (`ui.md`) work as they are: with `Web` set, `Create` opens
  `Web?create=Listen`, `Join` and `Games` open `Web?join=Addr`.
  `cmd/openorion2` on `js` sets `Web` to the page's `net` URL (ws: or wss:
  after the page's scheme) and the join address to the page's host name.
- **WebSocket subset** (`internal/netplay/ws.go`, standard library only:
  go.mod has no WebSocket package and the subset is small): the upgrade
  with `Sec-WebSocket-Accept`, binary and continuation frames, ping/pong,
  close; client frames must be masked, server frames are not; no
  extensions or subprotocols; at most 1 MB per frame. A request with an
  `Origin` whose host is not the server's is refused (403), so another
  site's page cannot use the server's relays.

## Running it

```
tools/web/build.sh
go run ./cmd/webserve -addr :8080              # /net on by default
```

- Creator in a browser: open `http://SERVER:8080/`, Multiplayer, Network,
  Start New Game; the create box asks for the TCP port (default `:52525`):
  the server listens there for native players.
- Joiner in another browser (another profile or machine: saves and
  settings live per browser): Multiplayer, Network, Join Game; the game
  list shows the games of the page's host (the join address defaults to
  it), so the game created there is found; ADDRESS types another.
- Native players join a browser's game with
  `openorion2 -join SERVER:52525`; a browser joins a native `openorion2 -host`
  by typing that machine's address (reachable from the server).
- URL parameters, as the command line flags: `?host` or `?host=:PORT`
  (skip the create box), `?join=HOST[:PORT]` (the game list shows that relay's games),
  `?name=NAME`.
- `webserve -net=false` turns `/net` off; `-netgames N` (8) bounds the
  games created from browsers at once. `-auth` covers `/net` as well
  (browsers send the page's credentials with the WebSocket). Behind TLS
  the page uses `wss:`.

## Tests

- `TestWebTransport` (internal/netplay): the transport tests of
  `TestTransport` (routing, unicast and broadcast, SetOpen, a joiner
  dropping, the relay dropping, the creator leaving) on two mixed games: a
  browser creator on a hosted relay with a browser joiner (bridge) and a
  native joiner (TCP); a native creator with a browser and a native joiner.
- `TestWebHostedRelay`: creator id 1, the relay outliving him and closing
  after the last station, joins refused before the creator, a creator who
  never says hello.
- `TestWebRefusals`: a taken port (the reason reaches the browser), a
  wrong game name, a far end that is not a relay, nothing listening, a
  plain GET (400), a cross-origin upgrade (403).
- `TestWSFraming`: RFC 6455's accept key, a fragmented masked message
  with a ping between the fragments (answered), an unmasked client frame
  refused.
- `TestNetTurnsWeb` (internal/game): two machines play three turns, the
  creator on a hosted relay and the joiner through the bridge, both over
  WebSocket.
- Not tested without a browser: `wsdial_js.go` itself (it compiles with
  `GOOS=js GOARCH=wasm`).

## Limits

- The browser's long computations (the AI's turn) block its page, pings
  included: a turn longer than the 30 s dead-link limit would drop that
  player. Turns are far shorter in practice.
- A hosted relay lives in the server process: restarting `webserve` ends
  its games.
- No LAN discovery: the game list shows the server's games (or those
  of `?join=`); another address is typed with ADDRESS.
