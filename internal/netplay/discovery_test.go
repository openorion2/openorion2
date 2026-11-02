package netplay

import (
	"net"
	"testing"
	"time"
)

// TestLANDiscovery: a join without an address finds the relay by the UDP
// search (port-only, docs/net/ui.md "LAN games"), lists its game and
// joins it.
func TestLANDiscovery(t *testing.T) {
	t.Parallel()
	pc, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Skip(err)
	}
	udp := pc.LocalAddr().(*net.UDPAddr).Port
	pc.Close()
	var relay *Relay
	creator, err := TCP{Listen: "127.0.0.1:0", Discovery: udp, Started: func(r *Relay) { relay = r }}.Create("MOX-7", "a")
	if err != nil {
		t.Skip(err)
	}
	defer relay.Close()
	defer creator.Close()
	addrs, err := Discover(udp)
	if err != nil || len(addrs) != 1 {
		t.Fatalf("Discover: %v, %v", addrs, err)
	}
	if _, port, _ := net.SplitHostPort(relay.Addr().String()); addrs[0] != net.JoinHostPort("127.0.0.1", port) {
		t.Errorf("found %s, the relay is on %s", addrs[0], relay.Addr())
	}
	j := TCP{Discovery: udp}
	gs, err := j.Games()
	if err != nil || len(gs) != 1 || gs[0].Name != "MOX-7" {
		t.Fatalf("Games: %+v, %v", gs, err)
	}
	ep, err := j.Join("MOX-7", "b")
	if err != nil {
		t.Fatal(err)
	}
	defer ep.Close()
	for deadline := time.Now().Add(5 * time.Second); len(creator.Users()) != 2; {
		if time.Now().After(deadline) {
			t.Fatalf("the creator sees %v", creator.Users())
		}
		time.Sleep(10 * time.Millisecond)
	}
	// Off: nothing is asked, nothing answers.
	if gs, _ := (TCP{Discovery: -1, Addr: "127.0.0.1:1"}).Games(); len(gs) != 0 {
		t.Errorf("without discovery: %+v", gs)
	}
}
