package netplay

import (
	"bytes"
	"encoding/binary"
	"errors"
	"net"
	"slices"
	"testing"
	"time"
)

func TestFrameRoundTrip(t *testing.T) {
	t.Parallel()
	frames := []Frame{
		{ID: 4, From: 1, To: 2, Payload: []byte{0, 0, 0, 0, 1}},
		{ID: 0x36, From: 2, To: 1, Payload: []byte{0}},
		{ID: 3, From: 1, To: Broadcast, Payload: bytes.Repeat([]byte{0xa5}, 0x7800)},
		{ID: 0x7d0, From: 3, To: 1, Payload: []byte{}},
		{ID: 0xffff, From: 0xffff, To: 0xffff, Payload: make([]byte, MaxPayload)},
	}
	var b bytes.Buffer
	for _, f := range frames {
		if err := WriteFrame(&b, f); err != nil {
			t.Fatal(err)
		}
	}
	for i, want := range frames {
		f, err := ReadFrame(&b)
		if err != nil {
			t.Fatalf("frame %d: %v", i, err)
		}
		if f.ID != want.ID || f.From != want.From || f.To != want.To || !bytes.Equal(f.Payload, want.Payload) {
			t.Fatalf("frame %d: %+v", i, f)
		}
	}
	if _, err := ReadFrame(&b); err == nil {
		t.Fatal("read past the end")
	}
	if err := WriteFrame(&b, Frame{Payload: make([]byte, MaxPayload+1)}); err == nil {
		t.Fatal("wrote an oversized payload")
	}
	// A header with a length below 6 or a cut payload is an error.
	if _, err := ReadFrame(bytes.NewReader([]byte{5, 0, 0, 0, 1, 0, 1, 0, 2})); err != ErrFrame {
		t.Fatalf("short length: %v", err)
	}
	if _, err := ReadFrame(bytes.NewReader([]byte{8, 0, 0, 0, 1, 0, 1, 0, 2, 0, 7})); err == nil {
		t.Fatal("cut payload read")
	}
}

// session is one game under test: the creator and two joiners.
type session struct {
	eps  []Endpoint
	drop func(i int) // cuts station i's link
}

// wait polls ep until a frame comes (TCP delivers on other goroutines).
func wait(t *testing.T, ep Endpoint) Frame {
	t.Helper()
	for end := time.Now().Add(5 * time.Second); time.Now().Before(end); time.Sleep(time.Millisecond) {
		if f, ok := ep.Recv(); ok {
			return f
		}
	}
	t.Fatal("no frame")
	return Frame{}
}

func until(t *testing.T, what string, cond func() bool) {
	t.Helper()
	for end := time.Now().Add(5 * time.Second); time.Now().Before(end); time.Sleep(time.Millisecond) {
		if cond() {
			return
		}
	}
	t.Fatal(what)
}

func memSession(t *testing.T) session {
	m := NewMem()
	c, err := m.Create("MOX-0", "a")
	if err != nil {
		t.Fatal(err)
	}
	s := session{eps: []Endpoint{c}}
	for _, n := range []string{"b", "c"} {
		j, err := m.Join("MOX-0", n)
		if err != nil {
			t.Fatal(err)
		}
		s.eps = append(s.eps, j)
	}
	if g, _ := m.Games(); len(g) != 1 || g[0].Name != "MOX-0" || !g[0].Open || len(g[0].Users) != 3 {
		t.Fatalf("games %+v", g)
	}
	s.drop = func(i int) { m.Drop("MOX-0", s.eps[i].ID()) }
	return s
}

func tcpSession(t *testing.T) session {
	r, err := Listen("127.0.0.1:0", "MOX-0")
	if err != nil {
		t.Skip(err)
	}
	t.Cleanup(func() { r.Close() })
	addr := r.Addr().String()
	s := session{eps: []Endpoint{r.Local()}}
	for _, n := range []string{"b", "c"} {
		j, err := Dial(addr, "MOX-0", n)
		if err != nil {
			t.Fatal(err)
		}
		s.eps = append(s.eps, j)
	}
	// Everyone has seen the last join.
	for _, ep := range s.eps {
		until(t, "users", func() bool { return len(ep.Users()) == 3 })
	}
	gi, err := Probe(addr)
	if err != nil || gi.Name != "MOX-0" || !gi.Open || len(gi.Users) != 3 {
		t.Fatalf("probe %+v %v", gi, err)
	}
	s.drop = func(i int) {
		if i == 0 {
			r.Close()
			return
		}
		s.eps[i].(*client).c.Close() // the link breaks under the station
	}
	return s
}

func TestTransport(t *testing.T) {
	t.Parallel()
	for _, k := range []struct {
		name string
		mk   func(*testing.T) session
	}{{"mem", memSession}, {"tcp", tcpSession}} {
		t.Run(k.name, func(t *testing.T) {
			t.Parallel()
			t.Run("routing", func(t *testing.T) { testRouting(t, k.mk(t)) })
			t.Run("joiner drops", func(t *testing.T) { testJoinerDrops(t, k.mk(t)) })
			t.Run("relay drops", func(t *testing.T) { testRelayDrops(t, k.mk(t)) })
			t.Run("creator leaves", func(t *testing.T) { testCreatorLeaves(t, k.mk(t)) })
		})
	}
}

// testRouting: net ids in join order, unicast, Broadcast to every other
// station, local delivery, unknown destinations refused at the sender.
func testRouting(t *testing.T, s session) {
	for i, ep := range s.eps {
		if ep.ID() != uint16(i+1) {
			t.Fatalf("station %d has net id %d", i, ep.ID())
		}
		if u := ep.Users(); !slices.Equal(u, []uint16{1, 2, 3}) {
			t.Fatalf("station %d users %v", i, u)
		}
	}
	a, b, c := s.eps[0], s.eps[1], s.eps[2]
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(b.Send(3, 0xb, []byte{1, 2}))
	if f := wait(t, c); f.ID != 0xb || f.From != 2 || f.To != 3 || !bytes.Equal(f.Payload, []byte{1, 2}) {
		t.Fatalf("b→c %+v", f)
	}
	must(c.Send(Broadcast, 0x36, []byte{0}))
	for _, ep := range []Endpoint{a, b} {
		if f := wait(t, ep); f.ID != 0x36 || f.From != 3 || f.To != Broadcast {
			t.Fatalf("broadcast at %d: %+v", ep.ID(), f)
		}
	}
	must(a.Send(1, 0x3a, []byte{1}))
	if f := wait(t, a); f.ID != 0x3a || f.From != 1 || f.To != 1 {
		t.Fatalf("a→a %+v", f)
	}
	// Order from one sender is kept.
	for k := range 50 {
		must(b.Send(1, uint16(k), nil))
	}
	for k := range 50 {
		if f := wait(t, a); f.ID != uint16(k) {
			t.Fatalf("frame %d is msg %d", k, f.ID)
		}
	}
	if err := a.Send(9, 1, nil); err != ErrNoUser {
		t.Fatalf("send to 9: %v", err)
	}
	// Closing the game to new players shows in the game info.
	if err := b.SetOpen(false); err == nil {
		t.Fatal("a joiner closed the game")
	}
	must(a.SetOpen(false))
	until(t, "closed", func() bool { return !c.Game().Open })
	for _, ep := range s.eps {
		if _, ok := ep.Recv(); ok {
			t.Fatalf("station %d has a stray frame", ep.ID())
		}
	}
}

// testJoinerDrops: the others see station 3 leave, sends to it fail, its
// own link is lost, the rest go on.
func testJoinerDrops(t *testing.T, s session) {
	s.drop(2)
	a, b, c := s.eps[0], s.eps[1], s.eps[2]
	until(t, "c lost", func() bool { return errors.Is(c.Err(), ErrLost) })
	for _, ep := range []Endpoint{a, b} {
		until(t, "users", func() bool { return slices.Equal(ep.Users(), []uint16{1, 2}) })
		if d := ep.Dropped(); !slices.Equal(d, []uint16{3}) {
			t.Fatalf("station %d: dropped %v", ep.ID(), d)
		}
	}
	if err := a.Send(3, 4, nil); err != ErrNoUser {
		t.Fatalf("send to the dropped: %v", err)
	}
	if err := c.Send(1, 4, nil); !errors.Is(err, ErrLost) {
		t.Fatalf("send from the dropped: %v", err)
	}
	if err := b.Send(1, 4, []byte{7}); err != nil {
		t.Fatal(err)
	}
	if f := wait(t, a); f.ID != 4 || f.From != 2 {
		t.Fatalf("%+v", f)
	}
}

// testRelayDrops: the creator leaving ends the game for every station.
func testRelayDrops(t *testing.T, s session) {
	s.drop(0)
	for _, ep := range s.eps[1:] {
		until(t, "lost", func() bool { return errors.Is(ep.Err(), ErrLost) })
		if err := ep.Send(1, 4, nil); err == nil {
			t.Fatal("sent after the relay left")
		}
	}
}

// testCreatorLeaves: the creator's station closing leaves the game like a
// joiner; the others go on through the relay.
func testCreatorLeaves(t *testing.T, s session) {
	a, b, c := s.eps[0], s.eps[1], s.eps[2]
	a.Close()
	for _, ep := range []Endpoint{b, c} {
		until(t, "users", func() bool { return slices.Equal(ep.Users(), []uint16{2, 3}) })
		if d := ep.Dropped(); !slices.Equal(d, []uint16{1}) {
			t.Fatalf("station %d: dropped %v", ep.ID(), d)
		}
	}
	if err := b.Send(Broadcast, 0x27, []byte("hi")); err != nil {
		t.Fatal(err)
	}
	if f := wait(t, c); f.ID != 0x27 || f.From != 2 {
		t.Fatalf("%+v", f)
	}
	b.Close()
	c.Close()
}

// TestTCPRelayOutlivesCreator: once the creator's station and every joiner
// have left, the relay closes itself.
func TestTCPRelayOutlivesCreator(t *testing.T) {
	t.Parallel()
	r, err := Listen("127.0.0.1:0", "MOX-0")
	if err != nil {
		t.Skip(err)
	}
	defer r.Close()
	j, err := Dial(r.Addr().String(), "MOX-0", "b")
	if err != nil {
		t.Fatal(err)
	}
	r.Local().Close()
	until(t, "creator gone", func() bool { return slices.Equal(j.Users(), []uint16{2}) })
	select {
	case <-r.Done():
		t.Fatal("the relay closed with a station left")
	case <-time.After(50 * time.Millisecond):
	}
	if j.Err() != nil {
		t.Fatal(j.Err())
	}
	j.Close()
	select {
	case <-r.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("the relay stayed after everybody left")
	}
}

// rawJoin does the handshake on a bare connection and returns its net id.
func rawJoin(t *testing.T, addr string) (net.Conn, uint16) {
	t.Helper()
	c, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	id, _, _, err := hello(c, helloJoin, "MOX-0", "raw")
	if err != nil {
		t.Fatal(err)
	}
	return c, id
}

// TestTCPDeadPeer: a silent station or relay is a lost link after Dead, pings
// keep idle links up, a station that stops reading is dropped on a write.
func TestTCPDeadPeer(t *testing.T) {
	t.Parallel()
	cfg := Config{Ping: 40 * time.Millisecond, Dead: 400 * time.Millisecond, Write: 300 * time.Millisecond}
	t.Run("silent station", func(t *testing.T) {
		t.Parallel()
		r, err := ListenConfig("127.0.0.1:0", "MOX-0", cfg)
		if err != nil {
			t.Skip(err)
		}
		defer r.Close()
		c, id := rawJoin(t, r.Addr().String())
		defer c.Close()
		go func() { // reads (and drops) the relay's pings, sends nothing
			for {
				if _, err := ReadFrame(c); err != nil {
					return
				}
			}
		}()
		until(t, "dropped", func() bool { return slices.Contains(r.Local().Dropped(), id) })
	})
	t.Run("idle links stay", func(t *testing.T) {
		t.Parallel()
		r, err := ListenConfig("127.0.0.1:0", "MOX-0", cfg)
		if err != nil {
			t.Skip(err)
		}
		defer r.Close()
		j, err := DialConfig(r.Addr().String(), "MOX-0", "b", cfg)
		if err != nil {
			t.Fatal(err)
		}
		defer j.Close()
		time.Sleep(3 * cfg.Dead)
		if j.Err() != nil || len(r.Local().Dropped()) != 0 || len(r.Local().Users()) != 2 {
			t.Fatalf("idle link dropped: %v, %v", j.Err(), r.Local().Dropped())
		}
		if _, ok := j.Recv(); ok {
			t.Fatal("a ping reached the game")
		}
	})
	t.Run("silent relay", func(t *testing.T) {
		t.Parallel()
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Skip(err)
		}
		defer ln.Close()
		go func() {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			defer c.Close()
			if _, err := ReadFrame(c); err != nil {
				return
			}
			gi := GameInfo{Name: "MOX-0", Users: []uint16{1, 2}}
			WriteFrame(c, welcome(2, gi, 1))
			for { // swallows the station's pings, answers nothing
				if _, err := ReadFrame(c); err != nil {
					return
				}
			}
		}()
		j, err := DialConfig(ln.Addr().String(), "MOX-0", "b", cfg)
		if err != nil {
			t.Fatal(err)
		}
		defer j.Close()
		until(t, "lost", func() bool { return errors.Is(j.Err(), ErrLost) })
	})
	t.Run("station stops reading", func(t *testing.T) {
		t.Parallel()
		r, err := ListenConfig("127.0.0.1:0", "MOX-0", cfg)
		if err != nil {
			t.Skip(err)
		}
		defer r.Close()
		c, id := rawJoin(t, r.Addr().String())
		defer c.Close()
		stop := make(chan struct{})
		defer close(stop)
		go func() { // pings the relay but never reads
			for {
				select {
				case <-stop:
					return
				case <-time.After(cfg.Ping):
					b := binary.LittleEndian.AppendUint32(nil, 6)
					b = binary.LittleEndian.AppendUint16(b, ctlPing)
					b = binary.LittleEndian.AppendUint16(b, id)
					b = binary.LittleEndian.AppendUint16(b, RelayID)
					if _, err := c.Write(b); err != nil {
						return
					}
				}
			}
		}()
		big := make([]byte, MaxPayload)
		for end := time.Now().Add(10 * time.Second); time.Now().Before(end); {
			if slices.Contains(r.Local().Dropped(), id) {
				return
			}
			r.Local().Send(id, 3, big)
		}
		t.Fatal("a station that does not read stayed in")
	})
}

func TestTCPRefusals(t *testing.T) {
	t.Parallel()
	r, err := Listen("127.0.0.1:0", "MOX-0")
	if err != nil {
		t.Skip(err)
	}
	defer r.Close()
	addr := r.Addr().String()
	if _, err := Dial(addr, "OTHER", "x"); !errors.Is(err, ErrRefused) {
		t.Fatalf("wrong game: %v", err)
	}
	r.Local().SetOpen(false)
	if _, err := Dial(addr, "MOX-0", "x"); !errors.Is(err, ErrRefused) {
		t.Fatalf("closed game: %v", err)
	}
	r.Local().SetOpen(true)
	var eps []Endpoint
	for range MaxUsers - 1 {
		ep, err := Dial(addr, "MOX-0", "x")
		if err != nil {
			t.Fatal(err)
		}
		eps = append(eps, ep)
	}
	if _, err := Dial(addr, "MOX-0", "x"); !errors.Is(err, ErrRefused) {
		t.Fatalf("ninth station: %v", err)
	}
	for _, ep := range eps {
		ep.Close()
	}
	until(t, "left", func() bool { return len(r.Local().Users()) == 1 })

	if withPort("host") != "host:52525" || withPort("") != ":52525" || withPort("h:1") != "h:1" {
		t.Fatal("withPort")
	}
}
