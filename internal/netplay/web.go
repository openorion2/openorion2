package netplay

import (
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// The browser's side of the relay (docs/net/web.md). A browser can neither
// listen nor dial TCP, so it opens a WebSocket to the server that serves
// the build and speaks the usual frames over it:
//
//	/net?create=:PORT  the server starts a hosted relay listening on PORT
//	                   (native players join it with -join) and links the
//	                   browser in as its creator, net id 1;
//	/net?join=HOST:PORT the server dials the relay at HOST:PORT (a native
//	                   -host, or one of its own hosted relays) and bridges.

// WebRelay is the http.Handler of /net.
type WebRelay struct {
	Config Config
	// MaxRelays bounds the hosted relays running at once (default 8).
	MaxRelays int
	// Started, if set, gets each hosted relay (tests).
	Started func(*Relay)
	// Log, if set, logs the links that come and go.
	Log *log.Logger

	mu     sync.Mutex
	relays int
}

func (wr *WebRelay) logf(format string, args ...any) {
	if wr.Log != nil {
		wr.Log.Printf(format, args...)
	}
}

func (wr *WebRelay) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	create, join := q.Get("create"), q.Get("join")
	if (create == "") == (join == "") {
		http.Error(w, "create=:port or join=host:port", http.StatusBadRequest)
		return
	}
	c, err := AcceptWebSocket(w, r)
	if err != nil {
		wr.logf("net: %s: %v", r.RemoteAddr, err)
		return
	}
	defer c.Close()
	if create != "" {
		wr.create(c, withPort(create), r.RemoteAddr)
	} else {
		wr.bridge(c, withPort(join), r.RemoteAddr)
	}
}

// create starts a hosted relay on addr and runs the browser's link as its
// creator.
func (wr *WebRelay) create(c net.Conn, addr, from string) {
	limit := wr.MaxRelays
	if limit <= 0 {
		limit = 8
	}
	wr.mu.Lock()
	full := wr.relays >= limit
	if !full {
		wr.relays++
	}
	wr.mu.Unlock()
	if full {
		refuseLink(c, "too many games on this server")
		return
	}
	r, err := HostRelay(addr, wr.Config)
	if err != nil {
		wr.mu.Lock()
		wr.relays--
		wr.mu.Unlock()
		refuseLink(c, err.Error())
		return
	}
	wr.logf("net: %s creates a game on %s", from, r.Addr())
	go func() {
		<-r.Done()
		wr.mu.Lock()
		wr.relays--
		wr.mu.Unlock()
		wr.logf("net: the game on %s is over", addr)
	}()
	if wr.Started != nil {
		wr.Started(r)
	}
	r.ServeCreator(c)
}

// refuseLink answers a station's hello with a Refuse.
func refuseLink(c net.Conn, why string) {
	c.SetDeadline(time.Now().Add(handshakeTimeout))
	if f, err := ReadFrame(c); err == nil && f.ID == ctlHello {
		WriteFrame(c, Frame{ID: ctlRefuse, Payload: []byte(why)})
	}
}

// bridge links the browser's c to the relay at addr. Nothing but the
// browser's hello goes out before the far end answers it as a relay, so
// /net cannot be used to reach other kinds of servers.
func (wr *WebRelay) bridge(c net.Conn, addr, from string) {
	c.SetReadDeadline(time.Now().Add(handshakeTimeout))
	hi, err := ReadFrame(c)
	if err != nil || hi.ID != ctlHello {
		return
	}
	// The far end has less time than the browser's handshake, so that the
	// browser hears why it failed.
	end := time.Now().Add(handshakeTimeout * 3 / 4)
	t, err := (&net.Dialer{Deadline: end}).Dial("tcp", addr)
	if err != nil {
		WriteFrame(c, Frame{ID: ctlRefuse, Payload: []byte(err.Error())})
		return
	}
	defer t.Close()
	keepAlive(t, wr.Config.orDefaults())
	t.SetDeadline(end)
	if WriteFrame(t, hi) != nil {
		return
	}
	ans, err := ReadFrame(t)
	if err != nil || ans.ID != ctlWelcome && ans.ID != ctlRefuse {
		WriteFrame(c, Frame{ID: ctlRefuse, Payload: []byte("no game at " + addr)})
		return
	}
	if WriteFrame(c, ans) != nil || ans.ID == ctlRefuse {
		return
	}
	c.SetDeadline(time.Time{})
	t.SetDeadline(time.Time{})
	wr.logf("net: %s joins %s", from, addr)
	// Both ends keep their own read deadlines (the relay's pings and the
	// station's go through), so a dead end shows as a closed copy.
	done := make(chan struct{}, 2)
	pipe := func(dst, src net.Conn) {
		io.Copy(dst, src)
		dst.Close()
		src.Close()
		done <- struct{}{}
	}
	go pipe(t, c)
	go pipe(c, t)
	<-done
	<-done
	wr.logf("net: %s left %s", from, addr)
}

// webURL is the /net URL for a create or join of addr.
func webURL(base, op, addr string) string {
	sep := "?"
	if strings.Contains(base, "?") {
		sep = "&"
	}
	return base + sep + op + "=" + url.QueryEscape(addr)
}

// webClient creates or joins through the WebRelay at t.Web.
func (t TCP) webClient(op, addr string, kind byte, game, player string) (Endpoint, error) {
	c, err := dialWebSocket(webURL(t.Web, op, withPort(addr)))
	if err != nil {
		return nil, err
	}
	return startClient(c, kind, game, player, t.Config.orDefaults())
}
