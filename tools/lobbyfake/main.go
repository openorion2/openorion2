// Command lobbyfake runs a lobby for games across machines without the
// real server (internal/lobby/lobbytest: its rooms and relay, no accounts:
// a session is the user it names). It writes its certificate to -cert;
// each game's lobby.yaml points at it:
//
//	server: 192.168.1.5:52531
//	ca: lobbyfake.pem
//
// and its lobby.session names the player: {"user":"bot1","session":"bot1"}.
package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/openorion2/openorion2/internal/lobby/lobbytest"
)

func main() {
	addr := flag.String("addr", ":52531", "the address to listen on")
	certFile := flag.String("cert", "lobbyfake.pem", "where to write the certificate the games trust")
	flag.Parse()
	ips := []net.IP{net.IPv4(127, 0, 0, 1)}
	if as, err := net.InterfaceAddrs(); err == nil {
		for _, a := range as {
			if n, ok := a.(*net.IPNet); ok && n.IP.To4() != nil {
				ips = append(ips, n.IP)
			}
		}
	}
	srv, cli, pemBytes, err := lobbytest.Certs(ips)
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(*certFile, pemBytes, 0o644); err != nil {
		log.Fatal(err)
	}
	ln, err := tls.Listen("tcp", *addr, srv)
	if err != nil {
		log.Fatal(err)
	}
	s := lobbytest.Serve(ln, cli)
	fmt.Fprintf(os.Stderr, "lobbyfake on %s for %v, certificate in %s\n", ln.Addr(), ips, *certFile)
	for {
		time.Sleep(time.Minute)
		log.Printf("relayed %d frames", s.Frames())
	}
}
