package netplay

import (
	"bytes"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

// LAN discovery (port-only, docs/net/ui.md "LAN games"): where IPX listed
// every game on the network by itself, a relay answers UDP datagrams on
// the discovery port with its TCP port, and a join without an address asks
// the whole LAN (and this machine) by broadcast, then probes each answer
// over TCP for the game as usual.

const (
	// DiscoveryPort is the UDP port relays answer on by default.
	DiscoveryPort = 52525
	// discoveryWait is how long a search collects answers.
	discoveryWait = 400 * time.Millisecond
)

var (
	discoveryAsk    = []byte("OPENORION2?")
	discoveryAnswer = []byte("OPENORION2 ")
)

// discoveryPort is the UDP port of a TCP transport: p, the default when 0,
// none when negative.
func discoveryPort(p int) int {
	if p == 0 {
		return DiscoveryPort
	}
	return p
}

// answerDiscovery answers on UDP port udp with the relay's TCP port until
// the relay closes. A port another relay holds is left alone: that one
// answers for its own game only, as before discovery.
func (r *Relay) answerDiscovery(udp int) {
	if udp < 0 {
		return
	}
	// On the TCP listener's address: a relay on 127.0.0.1 answers there only.
	host, port, _ := net.SplitHostPort(r.ln.Addr().String())
	ip := net.ParseIP(host)
	if ip != nil && ip.IsUnspecified() {
		ip = nil
	}
	pc, err := net.ListenUDP("udp4", &net.UDPAddr{IP: ip, Port: udp})
	if err != nil {
		return
	}
	reply := append(append([]byte(nil), discoveryAnswer...), port...)
	go func() {
		<-r.done
		pc.Close()
	}()
	go func() {
		buf := make([]byte, 64)
		for {
			n, from, err := pc.ReadFromUDP(buf)
			if err != nil {
				return
			}
			if bytes.Equal(buf[:n], discoveryAsk) {
				pc.WriteToUDP(reply, from)
			}
		}
	}()
}

// Discover asks the LAN and this machine for relays on UDP port udp and
// returns their TCP addresses (host:port) in the order they answered.
func Discover(udp int) ([]string, error) {
	pc, err := listenBroadcast()
	if err != nil {
		return nil, err
	}
	defer pc.Close()
	sent := false
	for _, ip := range []net.IP{net.IPv4bcast, net.IPv4(127, 0, 0, 1)} {
		if _, err := pc.WriteToUDP(discoveryAsk, &net.UDPAddr{IP: ip, Port: udp}); err == nil {
			sent = true
		}
	}
	if !sent {
		return nil, nil
	}
	local := map[string]bool{}
	if as, err := net.InterfaceAddrs(); err == nil {
		for _, a := range as {
			if n, ok := a.(*net.IPNet); ok {
				local[n.IP.String()] = true
			}
		}
	}
	var out []string
	seen := map[string]bool{}
	buf := make([]byte, 64)
	pc.SetReadDeadline(time.Now().Add(discoveryWait))
	for {
		n, from, err := pc.ReadFromUDP(buf)
		if err != nil {
			break
		}
		s, ok := strings.CutPrefix(string(buf[:n]), string(discoveryAnswer))
		if !ok {
			continue
		}
		if p, err := strconv.Atoi(s); err != nil || p <= 0 || p > 0xffff {
			continue
		}
		ip := from.IP.String()
		if local[ip] {
			ip = "127.0.0.1" // this machine answers on several addresses
		}
		a := net.JoinHostPort(ip, s)
		if !seen[a] {
			seen[a] = true
			out = append(out, a)
		}
	}
	return out, nil
}

// lanGames probes every relay Discover finds: the games and their
// addresses, in the same order.
func (t TCP) lanGames() ([]GameInfo, []string) {
	addrs, _ := Discover(discoveryPort(t.Discovery))
	var gs []GameInfo
	var as []string
	for _, a := range addrs {
		if gi, err := Probe(a); err == nil {
			gs = append(gs, gi)
			as = append(as, a)
		}
	}
	return gs, as
}

// lanCache keeps the last search per discovery port, so that the game
// list, which asks every second, does not wait for each search: only the
// first one is waited for, later ones run in the background.
type lanCache struct {
	mu      sync.Mutex
	games   []GameInfo
	addrs   []string
	done    bool // a search has finished
	running bool
}

var (
	lanMu     sync.Mutex
	lanByPort = map[int]*lanCache{}
)

// cachedLANGames is lanGames from the cache, refreshed in the background.
func (t TCP) cachedLANGames() ([]GameInfo, []string) {
	port := discoveryPort(t.Discovery)
	lanMu.Lock()
	c := lanByPort[port]
	if c == nil {
		c = &lanCache{}
		lanByPort[port] = c
	}
	lanMu.Unlock()
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.done {
		c.mu.Unlock()
		gs, as := t.lanGames()
		c.mu.Lock()
		c.games, c.addrs, c.done = gs, as, true
		return gs, as
	}
	if !c.running {
		c.running = true
		go func() {
			gs, as := t.lanGames()
			c.mu.Lock()
			c.games, c.addrs, c.running = gs, as, false
			c.mu.Unlock()
		}()
	}
	return c.games, c.addrs
}
