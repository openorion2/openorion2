package netplay

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"slices"
	"strconv"
	"sync"
	"time"
)

// handshakeTimeout bounds the Hello/Welcome exchange.
const handshakeTimeout = 10 * time.Second

// Config holds a link's timeouts; zero fields take the defaults.
type Config struct {
	Ping  time.Duration // a ping goes out this often on an idle link (5 s)
	Dead  time.Duration // a link silent this long is lost (30 s)
	Write time.Duration // a frame not written within this is a lost link (20 s)
}

func (c Config) orDefaults() Config {
	if c.Ping <= 0 {
		c.Ping = 5 * time.Second
	}
	if c.Dead <= 0 {
		c.Dead = 30 * time.Second
	}
	if c.Write <= 0 {
		c.Write = 20 * time.Second
	}
	return c
}

// keepAlive turns on the OS's TCP keepalive as a second line behind the pings.
func keepAlive(c net.Conn, cfg Config) {
	if tc, ok := c.(*net.TCPConn); ok {
		tc.SetKeepAlive(true)
		tc.SetKeepAlivePeriod(cfg.Dead / 2)
	}
}

// TCP is the network transport: Create listens on Listen and relays,
// Join and Games dial Addr (without one, the games LAN discovery finds).
type TCP struct {
	Listen string // the creator's address, ":52525" when empty
	Addr   string // the relay to join, host or host:port
	Config Config
	// Started, if set, gets the relay Create starts (cmd/openorion2 keeps it
	// running for the others after the creator's game ends).
	Started func(*Relay)
	// Web, when set, is the ws:// URL of a WebRelay (cmd/webserve's /net):
	// the browser build cannot listen or dial TCP, so the server does both.
	Web string
	// Discovery is the UDP port of LAN discovery (port-only): the relay
	// answers there, and an empty Addr searches the LAN; 0 is
	// DiscoveryPort, negative turns it off.
	Discovery int
}

func withPort(addr string) string {
	if addr == "" {
		return ":" + strconv.Itoa(DefaultPort)
	}
	if _, _, err := net.SplitHostPort(addr); err != nil {
		return net.JoinHostPort(addr, strconv.Itoa(DefaultPort))
	}
	return addr
}

// Create listens and returns the creator's endpoint (net id 1).
func (t TCP) Create(game, player string) (Endpoint, error) {
	if t.Web != "" {
		return t.webClient("create", t.Listen, helloCreate, game, player)
	}
	r, err := ListenConfig(withPort(t.Listen), game, t.Config)
	if err != nil {
		return nil, err
	}
	r.answerDiscovery(discoveryPort(t.Discovery))
	if t.Started != nil {
		t.Started(r)
	}
	return r.Local(), nil
}

// Join dials the relay and joins its game.
func (t TCP) Join(game, player string) (Endpoint, error) {
	if t.Web != "" {
		return t.webClient("join", t.Addr, helloJoin, game, player)
	}
	addr := withPort(t.Addr)
	if t.Addr == "" && t.Discovery >= 0 {
		// No address: the LAN game of that name, else this machine's.
		gs, as := t.cachedLANGames()
		for i, gi := range gs {
			if gi.Name == game {
				addr = as[i]
				break
			}
		}
	}
	return DialConfig(addr, game, player, t.Config)
}

// Games asks the relay for its game.
func (t TCP) Games() ([]GameInfo, error) {
	probe := Probe
	if t.Web != "" {
		probe = func(addr string) (GameInfo, error) {
			c, err := dialWebSocket(webURL(t.Web, "join", addr))
			if err != nil {
				return GameInfo{}, err
			}
			return probeConn(c)
		}
	}
	if t.Addr == "" && t.Web == "" && t.Discovery >= 0 {
		if gs, _ := t.cachedLANGames(); len(gs) > 0 {
			return gs, nil
		}
	}
	gi, err := probe(withPort(t.Addr))
	if err != nil {
		return nil, err
	}
	return []GameInfo{gi}, nil
}

// Relay is the creator's side of a TCP game: it relays between the stations
// (Local is his own); it outlives his station and closes after the last one.
type Relay struct {
	h      *hub
	ln     net.Listener
	local  *local
	cfg    Config
	wg     sync.WaitGroup
	hosted bool // HostRelay: no station of its own, the creator links in

	mu       sync.Mutex
	creator  uint16 // hosted: the creator's net id once he is in
	orphan   bool   // the creator's station has left
	once     sync.Once
	closeErr error
	done     chan struct{}
}

// Listen starts a relay for game on addr; the creator is net id 1.
func Listen(addr, game string) (*Relay, error) { return ListenConfig(addr, game, Config{}) }

// ListenConfig is Listen with the links' timeouts.
func ListenConfig(addr, game string, cfg Config) (*Relay, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	r := &Relay{h: newHub(game), ln: ln, cfg: cfg.orDefaults(), done: make(chan struct{})}
	if r.local, err = join(r.h, true); err != nil {
		ln.Close()
		return nil, err
	}
	r.local.onClose = r.creatorLeft
	r.wg.Add(1)
	go r.accept()
	return r, nil
}

// HostRelay starts a relay on addr without a station of its own: the
// first station, linked in with ServeCreator, creates the game and is net
// id 1 (cmd/webserve runs these for browsers). Joins wait for him.
func HostRelay(addr string, cfg Config) (*Relay, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	r := &Relay{h: newHub(""), ln: ln, cfg: cfg.orDefaults(), done: make(chan struct{}), hosted: true}
	r.wg.Add(1)
	go r.accept()
	return r, nil
}

// ServeCreator runs a hosted relay's creator link c until it ends. The
// relay outlives him as usual and closes once its last station has left
// (at once if he never got in).
func (r *Relay) ServeCreator(c net.Conn) {
	r.serve(c, true)
	r.mu.Lock()
	r.orphan = true
	r.mu.Unlock()
	r.closeIfAbandoned()
}

// Addr is the address the relay listens on.
func (r *Relay) Addr() net.Addr { return r.ln.Addr() }

// Local is the creator's endpoint (nil for a hosted relay).
func (r *Relay) Local() Endpoint {
	if r.local == nil {
		return nil
	}
	return r.local
}

// Members is the number of stations still in the game.
func (r *Relay) Members() int { return len(r.h.info().Users) }

// Done is closed once the relay has closed.
func (r *Relay) Done() <-chan struct{} { return r.done }

// Close ends the game: the listener and every link close.
func (r *Relay) Close() error {
	r.once.Do(func() {
		r.closeErr = r.ln.Close()
		r.h.close()
		r.wg.Wait()
		close(r.done)
	})
	return r.closeErr
}

// creatorLeft is the creator's Close: his station leaves, the relay stays
// for the others (the original's game goes on without a quitter).
func (r *Relay) creatorLeft() error {
	r.h.remove(r.local.id)
	r.mu.Lock()
	r.orphan = true
	r.mu.Unlock()
	r.closeIfAbandoned()
	return nil
}

// closeIfAbandoned closes the relay once its creator and every station left.
func (r *Relay) closeIfAbandoned() {
	r.mu.Lock()
	orphan := r.orphan
	r.mu.Unlock()
	if orphan && r.Members() == 0 {
		go r.Close() // not on a goroutine Close waits for
	}
}

func (r *Relay) accept() {
	defer r.wg.Done()
	for {
		c, err := r.ln.Accept()
		if err != nil {
			return
		}
		r.wg.Add(1)
		go func() {
			defer r.wg.Done()
			r.serve(c, false)
		}()
	}
}

// remote is a member linked over TCP, as the relay sees it.
type remote struct {
	c     net.Conn
	cfg   Config
	mu    sync.Mutex // one frame at a time on the link
	wrote time.Time  // the last frame written (pings fill idle links)
}

// write sends f within the write timeout; a link that cannot take it is
// closed, so that its reader ends and the station leaves.
func (m *remote) write(f Frame) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.c.SetWriteDeadline(time.Now().Add(m.cfg.Write))
	err := WriteFrame(m.c, f)
	if err != nil {
		m.c.Close()
	}
	m.wrote = time.Now()
	return err
}

// pinger writes a ping whenever the link was idle for cfg.Ping, until done.
func pinger(cfg Config, done <-chan struct{}, idle func() time.Duration, ping func() error) {
	t := time.NewTicker(cfg.Ping / 2)
	defer t.Stop()
	for {
		select {
		case <-done:
			return
		case <-t.C:
			if idle() >= cfg.Ping && ping() != nil {
				return
			}
		}
	}
}

func (m *remote) deliver(f Frame) error { return m.write(f) }

func (m *remote) members(ids []uint16, open bool, gen uint32) {
	b := binary.LittleEndian.AppendUint32(nil, gen)
	m.write(Frame{ID: ctlUsers, Payload: putIDs(append(b, b2u8(open)), ids)})
}

func (m *remote) left(id uint16) {
	m.write(Frame{ID: ctlLost, Payload: binary.LittleEndian.AppendUint16(nil, id)})
}

func (m *remote) drop(error) { m.c.Close() }

func b2u8(b bool) byte {
	if b {
		return 1
	}
	return 0
}

// serve runs one station's link: the handshake, then its frames; creator
// is a hosted relay's creator link.
func (r *Relay) serve(c net.Conn, creator bool) {
	defer c.Close()
	keepAlive(c, r.cfg)
	m := &remote{c: c, cfg: r.cfg}
	c.SetReadDeadline(time.Now().Add(handshakeTimeout))
	f, err := ReadFrame(c)
	if err != nil || f.ID != ctlHello || len(f.Payload) < 3 {
		return
	}
	c.SetReadDeadline(time.Time{})
	ver, kind := binary.LittleEndian.Uint16(f.Payload), f.Payload[2]
	_, rest, err := cstr(f.Payload[3:])
	if err != nil {
		return
	}
	game, _, err := cstr(rest)
	if err != nil {
		return
	}
	refuse := func(why string) { m.write(Frame{ID: ctlRefuse, Payload: []byte(why)}) }
	if ver != Version {
		refuse("incompatible versions")
		return
	}
	gi, gen := r.h.infoGen()
	if kind == helloQuery {
		m.write(welcome(0, gi, gen))
		return
	}
	if creator {
		if kind != helloCreate {
			refuse("not a create")
			return
		}
		r.h.rename(game)
	} else if kind != helloJoin || game != gi.Name || r.hosted && r.creatorID() == 0 {
		refuse("no such game")
		return
	}
	id, _, err := r.h.add(m)
	if err != nil {
		refuse(err.Error())
		return
	}
	if creator {
		r.mu.Lock()
		r.creator = id
		r.mu.Unlock()
	}
	defer r.closeIfAbandoned()
	defer r.h.remove(id)
	gi, gen = r.h.infoGen()
	if m.write(welcome(id, gi, gen)) != nil {
		return
	}
	done := make(chan struct{})
	defer close(done)
	go pinger(r.cfg, done, func() time.Duration {
		m.mu.Lock()
		defer m.mu.Unlock()
		return time.Since(m.wrote)
	}, func() error { return m.write(Frame{ID: ctlPing}) })
	for {
		c.SetReadDeadline(time.Now().Add(r.cfg.Dead))
		f, err := ReadFrame(c)
		if err != nil {
			return
		}
		if f.To == RelayID {
			if creator && f.ID == ctlOpen && len(f.Payload) == 1 {
				r.h.setOpen(f.Payload[0] != 0)
			}
			continue // a ping, or a control frame the relay does not take
		}
		f.From = id
		if r.h.route(f) == ErrNoUser {
			continue
		}
	}
}

func welcome(id uint16, gi GameInfo, gen uint32) Frame {
	b := binary.LittleEndian.AppendUint16(nil, Version)
	b = binary.LittleEndian.AppendUint16(b, id)
	b = binary.LittleEndian.AppendUint32(b, gen)
	b = append(b, b2u8(gi.Open))
	b = append(append(b, gi.Name...), 0)
	return Frame{ID: ctlWelcome, Payload: putIDs(b, gi.Users)}
}

// hello sends the handshake and reads the answer.
func hello(c net.Conn, kind byte, game, player string) (uint16, GameInfo, uint32, error) {
	b := binary.LittleEndian.AppendUint16(nil, Version)
	b = append(b, kind)
	b = append(append(b, player...), 0)
	b = append(append(b, game...), 0)
	c.SetDeadline(time.Now().Add(handshakeTimeout))
	defer c.SetDeadline(time.Time{})
	if err := WriteFrame(c, Frame{ID: ctlHello, Payload: b}); err != nil {
		return 0, GameInfo{}, 0, err
	}
	f, err := ReadFrame(c)
	if err != nil {
		return 0, GameInfo{}, 0, err
	}
	switch {
	case f.ID == ctlRefuse:
		return 0, GameInfo{}, 0, fmt.Errorf("%w: %s", ErrRefused, f.Payload)
	case f.ID != ctlWelcome || len(f.Payload) < 9:
		return 0, GameInfo{}, 0, ErrFrame
	}
	p := f.Payload
	id, gen := binary.LittleEndian.Uint16(p[2:]), binary.LittleEndian.Uint32(p[4:])
	gi := GameInfo{Open: p[8] != 0}
	name, rest, err := cstr(p[9:])
	if err != nil {
		return 0, GameInfo{}, 0, err
	}
	gi.Name = name
	if gi.Users, _, err = getIDs(rest); err != nil {
		return 0, GameInfo{}, 0, err
	}
	return id, gi, gen, nil
}

func (r *Relay) creatorID() uint16 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.creator
}

// Probe asks the relay at addr for its game without joining.
func Probe(addr string) (GameInfo, error) {
	c, err := net.DialTimeout("tcp", addr, handshakeTimeout)
	if err != nil {
		return GameInfo{}, err
	}
	return probeConn(c)
}

// probeConn asks for the game over the link c and closes it.
func probeConn(c net.Conn) (GameInfo, error) {
	defer c.Close()
	_, gi, _, err := hello(c, helloQuery, "", "")
	return gi, err
}

// client is a joined station's endpoint.
type client struct {
	inbox
	creator bool // a hosted relay's creator: SetOpen goes to the relay
	c       net.Conn
	cfg     Config
	wmu     sync.Mutex
	wrote   time.Time // the last frame written
	done    chan struct{}
	stop    chan struct{} // ends the pinger
}

// Dial joins game at the relay addr as player.
func Dial(addr, game, player string) (Endpoint, error) {
	return DialConfig(addr, game, player, Config{})
}

// DialConfig is Dial with the link's timeouts.
func DialConfig(addr, game, player string, cfg Config) (Endpoint, error) {
	cfg = cfg.orDefaults()
	c, err := net.DialTimeout("tcp", addr, handshakeTimeout)
	if err != nil {
		return nil, err
	}
	keepAlive(c, cfg)
	return startClient(c, helloJoin, game, player, cfg)
}

// startClient runs the handshake of kind over the link c, then its reader
// and pinger.
func startClient(c net.Conn, kind byte, game, player string, cfg Config) (Endpoint, error) {
	id, gi, gen, err := hello(c, kind, game, player)
	if err != nil {
		c.Close()
		return nil, err
	}
	cl := &client{c: c, cfg: cfg, done: make(chan struct{}), stop: make(chan struct{}), wrote: time.Now(), creator: kind == helloCreate}
	cl.id, cl.game = id, gi.Name
	cl.setUsers(gi.Users, gi.Open, gen)
	go cl.read()
	go pinger(cfg, cl.stop, func() time.Duration {
		cl.wmu.Lock()
		defer cl.wmu.Unlock()
		return time.Since(cl.wrote)
	}, func() error { return cl.write(Frame{ID: ctlPing, From: cl.id, To: RelayID}) })
	return cl, nil
}

// write sends f within the write timeout; a failure is a lost link.
func (cl *client) write(f Frame) error {
	cl.wmu.Lock()
	cl.c.SetWriteDeadline(time.Now().Add(cl.cfg.Write))
	err := WriteFrame(cl.c, f)
	cl.wrote = time.Now()
	cl.wmu.Unlock()
	if err != nil {
		cl.fail(ErrLost)
		cl.c.Close() // the reader ends too
		return ErrLost
	}
	return nil
}

// read is the link's reader goroutine: it queues game frames and applies
// membership changes; a read error, or no frame within cfg.Dead (the relay
// pings), is a lost link.
func (cl *client) read() {
	defer close(cl.done)
	defer close(cl.stop)
	for {
		cl.c.SetReadDeadline(time.Now().Add(cl.cfg.Dead))
		f, err := ReadFrame(cl.c)
		if err != nil {
			cl.fail(ErrLost)
			cl.c.Close()
			return
		}
		switch f.ID {
		case ctlPing:
		case ctlUsers:
			if p := f.Payload; len(p) >= 5 {
				if ids, _, err := getIDs(p[5:]); err == nil {
					cl.setUsers(ids, p[4] != 0, binary.LittleEndian.Uint32(p))
				}
			}
		case ctlLost:
			if len(f.Payload) >= 2 {
				cl.lost(binary.LittleEndian.Uint16(f.Payload))
			}
		default:
			cl.push(f)
		}
	}
}

func (cl *client) Send(to, id uint16, payload []byte) error {
	if err := cl.checkSend(to); err != nil {
		return err
	}
	f := Frame{ID: id, From: cl.id, To: to, Payload: slices.Clone(payload)}
	if to == cl.id {
		cl.push(f)
		return nil
	}
	return cl.write(f)
}

func (cl *client) SetOpen(open bool) error {
	if !cl.creator {
		return errors.New("netplay: only the creator opens or closes the game")
	}
	return cl.write(Frame{ID: ctlOpen, From: cl.id, To: RelayID, Payload: []byte{b2u8(open)}})
}

// Close leaves the game and waits for the reader to end.
func (cl *client) Close() error {
	cl.fail(ErrClosed)
	err := cl.c.Close()
	<-cl.done
	return err
}
