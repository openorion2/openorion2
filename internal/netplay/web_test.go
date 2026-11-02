package netplay

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"
)

// webServer serves a WebRelay as cmd/webserve does; relays gets each
// hosted relay it starts.
func webServer(t *testing.T, cfg Config) (base string, relays chan *Relay) {
	t.Helper()
	relays = make(chan *Relay, 8)
	wr := &WebRelay{Config: cfg, Started: func(r *Relay) { relays <- r }}
	mux := http.NewServeMux()
	mux.Handle("/net", wr)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return "ws" + strings.TrimPrefix(srv.URL, "http") + "/net", relays
}

// webSession: a browser creates the game on the server (a hosted relay),
// a second browser joins through the bridge, a native player over TCP.
func webSession(t *testing.T) session {
	base, relays := webServer(t, Config{})
	a, err := TCP{Web: base, Listen: "127.0.0.1:0"}.Create("MOX-0", "a")
	if err != nil {
		t.Fatal(err)
	}
	r := <-relays
	t.Cleanup(func() { r.Close() })
	addr := r.Addr().String()
	b, err := TCP{Web: base, Addr: addr}.Join("MOX-0", "b")
	if err != nil {
		t.Fatal(err)
	}
	c, err := TCP{Addr: addr}.Join("MOX-0", "c")
	if err != nil {
		t.Fatal(err)
	}
	s := session{eps: []Endpoint{a, b, c}}
	for _, ep := range s.eps {
		until(t, "users", func() bool { return len(ep.Users()) == 3 })
	}
	gs, err := TCP{Web: base, Addr: addr}.Games()
	if err != nil || len(gs) != 1 || gs[0].Name != "MOX-0" || !gs[0].Open || len(gs[0].Users) != 3 {
		t.Fatalf("games over the web %+v %v", gs, err)
	}
	s.drop = func(i int) {
		if i == 0 {
			r.Close()
			return
		}
		s.eps[i].(*client).c.Close()
	}
	return s
}

// bridgeSession: a native player creates (-host), a browser joins through
// the server's bridge, another native player over TCP.
func bridgeSession(t *testing.T) session {
	base, _ := webServer(t, Config{})
	r, err := Listen("127.0.0.1:0", "MOX-0")
	if err != nil {
		t.Skip(err)
	}
	t.Cleanup(func() { r.Close() })
	addr := r.Addr().String()
	b, err := TCP{Web: base, Addr: addr}.Join("MOX-0", "b")
	if err != nil {
		t.Fatal(err)
	}
	c, err := Dial(addr, "MOX-0", "c")
	if err != nil {
		t.Fatal(err)
	}
	s := session{eps: []Endpoint{r.Local(), b, c}}
	for _, ep := range s.eps {
		until(t, "users", func() bool { return len(ep.Users()) == 3 })
	}
	s.drop = func(i int) {
		if i == 0 {
			r.Close()
			return
		}
		s.eps[i].(*client).c.Close()
	}
	return s
}

func TestWebTransport(t *testing.T) {
	t.Parallel()
	for _, k := range []struct {
		name string
		mk   func(*testing.T) session
	}{{"web", webSession}, {"bridge", bridgeSession}} {
		t.Run(k.name, func(t *testing.T) {
			t.Parallel()
			t.Run("routing", func(t *testing.T) { testRouting(t, k.mk(t)) })
			t.Run("joiner drops", func(t *testing.T) { testJoinerDrops(t, k.mk(t)) })
			t.Run("relay drops", func(t *testing.T) { testRelayDrops(t, k.mk(t)) })
			t.Run("creator leaves", func(t *testing.T) { testCreatorLeaves(t, k.mk(t)) })
		})
	}
}

// TestWebHostedRelay: a hosted relay closes once everybody left, and at
// once when its creator never got in; joins wait for the creator.
func TestWebHostedRelay(t *testing.T) {
	t.Parallel()
	base, relays := webServer(t, Config{})
	a, err := TCP{Web: base, Listen: "127.0.0.1:0"}.Create("MOX-0", "a")
	if err != nil {
		t.Fatal(err)
	}
	r := <-relays
	if a.ID() != 1 || r.Local() != nil {
		t.Fatalf("creator id %d, local %v", a.ID(), r.Local())
	}
	j, err := Dial(r.Addr().String(), "MOX-0", "j")
	if err != nil {
		t.Fatal(err)
	}
	a.Close()
	until(t, "creator gone", func() bool { return slices.Equal(j.Users(), []uint16{2}) })
	select {
	case <-r.Done():
		t.Fatal("the relay closed with a station left")
	case <-time.After(50 * time.Millisecond):
	}
	j.Close()
	select {
	case <-r.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("the relay stayed after everybody left")
	}

	// A relay whose creator hangs up before his hello closes; nobody joins
	// a hosted relay before its creator.
	h, err := HostRelay("127.0.0.1:0", Config{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Dial(h.Addr().String(), "", "j"); !errors.Is(err, ErrRefused) {
		t.Fatalf("join before the creator: %v", err)
	}
	c1, c2 := net.Pipe()
	c2.Close()
	h.ServeCreator(c1)
	select {
	case <-h.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("a relay without creator stayed")
	}
}

func TestWebRefusals(t *testing.T) {
	t.Parallel()
	base, relays := webServer(t, Config{})
	a, err := TCP{Web: base, Listen: "127.0.0.1:0"}.Create("MOX-0", "a")
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	addr := (<-relays).Addr().String()
	// The port is taken: the browser hears why (the system's words: Windows
	// says "Only one usage of each socket address").
	inUse := func(err error) bool {
		return strings.Contains(err.Error(), "in use") || strings.Contains(err.Error(), "Only one usage")
	}
	if _, err := (TCP{Web: base, Listen: addr}).Create("MOX-1", "b"); !errors.Is(err, ErrRefused) || !inUse(err) {
		t.Fatalf("create on a taken port: %v", err)
	}
	if _, err := (TCP{Web: base, Addr: addr}).Join("OTHER", "b"); !errors.Is(err, ErrRefused) {
		t.Fatalf("wrong game: %v", err)
	}
	// Something that is not a relay: only the hello goes out.
	other, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	go func() {
		for {
			c, err := other.Accept()
			if err != nil {
				return
			}
			c.Write([]byte("HTTP/1.1 400 Bad Request\r\n\r\n"))
			c.Close()
		}
	}()
	if _, err := (TCP{Web: base, Addr: other.Addr().String()}).Join("MOX-0", "b"); !errors.Is(err, ErrRefused) {
		t.Fatalf("join a web server: %v", err)
	}
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	closed := ln.Addr().String()
	ln.Close()
	if _, err := (TCP{Web: base, Addr: closed}).Join("MOX-0", "b"); !errors.Is(err, ErrRefused) {
		t.Fatalf("join nothing: %v", err)
	}
	// Plain HTTP, and a page of another origin, are not let in.
	hb := "http" + strings.TrimPrefix(base, "ws")
	if code, err := upgradeStatus(hb+"?join=x", ""); err != nil || code != 400 {
		t.Fatalf("plain GET: %d %v", code, err)
	}
	if code, err := upgradeStatus(hb+"?join=x", "http://evil.example"); err != nil || code != 403 {
		t.Fatalf("cross-origin: %d %v", code, err)
	}
}

// upgradeStatus sends a WebSocket upgrade from origin (a plain GET when
// origin is empty) and returns the status.
func upgradeStatus(u, origin string) (int, error) {
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return 0, err
	}
	if origin != "" {
		req.Header.Set("Connection", "Upgrade")
		req.Header.Set("Upgrade", "websocket")
		req.Header.Set("Sec-WebSocket-Version", "13")
		req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
		req.Header.Set("Origin", origin)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	resp.Body.Close()
	return resp.StatusCode, nil
}

// TestWSFraming: the server side reads a fragmented, masked message with a
// ping in between and answers the ping; the accept key is RFC 6455's.
func TestWSFraming(t *testing.T) {
	t.Parallel()
	if got := wsAccept("dGhlIHNhbXBsZSBub25jZQ=="); got != "s3pPLMBiTxaQ9kYGzzhZRbK+xOo=" {
		t.Fatalf("accept %q", got)
	}
	sc, cc := net.Pipe()
	defer sc.Close()
	defer cc.Close()
	s := &wsConn{c: sc, br: bufio.NewReader(sc)}
	mask := []byte{1, 2, 3, 4}
	frame := func(b0 byte, p []byte) []byte {
		out := append([]byte{b0, 0x80 | byte(len(p))}, mask...)
		for i, c := range p {
			out = append(out, c^mask[i&3])
		}
		return out
	}
	go func() {
		cc.Write(frame(0x02, []byte("hel")))      // binary, not final
		cc.Write(frame(0x89, []byte("p")))        // ping
		cc.Write(frame(0x80, []byte("lo world"))) // continuation, final
	}()
	pong := make(chan []byte, 1)
	got := make([]byte, 11)
	go func() {
		b := make([]byte, 3)
		io.ReadFull(cc, b)
		pong <- b
	}()
	if _, err := io.ReadFull(s, got); err != nil || string(got) != "hello world" {
		t.Fatalf("read %q %v", got, err)
	}
	if b := <-pong; !bytes.Equal(b, []byte{0x8a, 1, 'p'}) {
		t.Fatalf("pong % x", b)
	}
	// A client frame without a mask is refused.
	go cc.Write([]byte{0x82, 1, 'x'})
	if _, err := s.Read(got); err != errWSFrame {
		t.Fatalf("unmasked: %v", err)
	}
}
