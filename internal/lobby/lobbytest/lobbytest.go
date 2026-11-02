//go:build !js

// Package lobbytest is a lobby server for the game's tests: the protocol's
// rooms and relay as the real server has them (a separate, private
// repository the game cannot import), without accounts: any session
// resumes, as the user the session names.
package lobbytest

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/openorion2/openorion2/lobbynet/lobbyv1"
	"github.com/openorion2/openorion2/lobbynet/wire"
)

const broadcast = 0x7ffe

// Server is a running test lobby.
type Server struct {
	Addr string
	TLS  *tls.Config // the client's: trusts the server's certificate

	mu        sync.Mutex
	rooms     map[string]*room
	links     map[*link]bool
	nextID    int
	frames    int               // relayed so far
	continues map[string]string // game id → the game it continues (ContinueGame)
	desyncs   []Desync          // the desync reports
}

// Desync is a DesyncReport as the server took it: its game and member.
type Desync struct {
	Game   string
	NetID  uint32
	Report *lobbyv1.DesyncReport
}

// Desyncs is the desync reports taken so far.
func (s *Server) Desyncs() []Desync {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Desync(nil), s.desyncs...)
}

// Frames is how many frames the server relayed so far.
func (s *Server) Frames() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.frames
}

// Continues is each game's ContinueGame: game id → the game it continues.
func (s *Server) Continues() map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]string, len(s.continues))
	for k, v := range s.continues {
		out[k] = v
	}
	return out
}

// everyone pushes env to every link; under mu.
func (s *Server) everyone(env *lobbyv1.Envelope) {
	for l := range s.links {
		l.send(env)
	}
}

type room struct {
	id, description       string
	members               map[uint32]*link
	players               map[string]bool // who was a member: who may come back
	next, gen             uint32
	open, started, public bool
}

type link struct {
	w     *wire.Conn
	user  string
	room  *room
	netID uint32
	mu    sync.Mutex // writes in order
}

func (l *link) send(env *lobbyv1.Envelope) {
	l.mu.Lock()
	l.w.Send(env)
	l.mu.Unlock()
}

// Start runs a test lobby until the test ends.
func Start(t testing.TB) *Server {
	t.Helper()
	srv, cli, _, err := Certs([]net.IP{net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	ln, err := tls.Listen("tcp", "127.0.0.1:0", srv)
	if err != nil {
		t.Skip(err)
	}
	t.Cleanup(func() { ln.Close() })
	return Serve(ln, cli)
}

// Serve runs a lobby on ln (tools/lobbyfake: a lobby for games across
// machines without the real server's accounts). cli is the clients' TLS.
func Serve(ln net.Listener, cli *tls.Config) *Server {
	s := &Server{Addr: ln.Addr().String(), TLS: cli, rooms: map[string]*room{}, links: map[*link]bool{}, continues: map[string]string{}}
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			l := &link{w: wire.NewConn(c)}
			s.mu.Lock()
			s.links[l] = true
			s.mu.Unlock()
			go s.serve(l)
		}
	}()
	return s
}

func reply(l *link, req, ans *lobbyv1.Envelope) {
	ans.Id = req.Id
	l.send(ans)
}

func fail(l *link, req *lobbyv1.Envelope, code lobbyv1.ErrorCode, text string) {
	reply(l, req, &lobbyv1.Envelope{Msg: &lobbyv1.Envelope_Error{Error: &lobbyv1.Error{Code: code, Text: text}}})
}

func ok() *lobbyv1.Envelope { return &lobbyv1.Envelope{Msg: &lobbyv1.Envelope_Ok{Ok: &lobbyv1.Ok{}}} }

func (s *Server) serve(l *link) {
	defer func() {
		s.mu.Lock()
		s.leave(l)
		delete(s.links, l)
		s.mu.Unlock()
		l.w.Close()
	}()
	for {
		env, err := l.w.Recv()
		if err != nil {
			return
		}
		switch m := env.Msg.(type) {
		case *lobbyv1.Envelope_Hello:
			l.send(&lobbyv1.Envelope{Msg: &lobbyv1.Envelope_Welcome{Welcome: &lobbyv1.Welcome{Protocol: lobbyv1.Protocol}}})
		case *lobbyv1.Envelope_Resume:
			l.user = m.Resume.Session
			reply(l, env, &lobbyv1.Envelope{Msg: &lobbyv1.Envelope_LoginOk{LoginOk: &lobbyv1.LoginOk{
				Session: m.Resume.Session, User: &lobbyv1.User{Name: l.user}}}})
			l.send(&lobbyv1.Envelope{Msg: &lobbyv1.Envelope_Snapshot{Snapshot: &lobbyv1.LobbySnapshot{}}})
		case *lobbyv1.Envelope_CreateGame:
			s.mu.Lock()
			s.nextID++
			r := &room{id: fmt.Sprintf("00000000-0000-4000-8000-%012d", s.nextID), description: m.CreateGame.Description,
				members: map[uint32]*link{}, next: 1, open: true, public: m.CreateGame.Public}
			s.rooms[r.id] = r
			ans := s.add(r, l)
			if r.public { // on everyone's list, as the real server tells
				s.everyone(&lobbyv1.Envelope{Msg: &lobbyv1.Envelope_GameUpdate{GameUpdate: &lobbyv1.GameUpdate{Game: &lobbyv1.GameInfo{
					Id: r.id, Description: r.description, Creator: l.user, Public: true,
					State: lobbyv1.GameState_GAME_STATE_OPEN, Players: []*lobbyv1.Player{{Name: l.user, NetId: 1}}}}}})
			}
			s.mu.Unlock()
			reply(l, env, ans)
		case *lobbyv1.Envelope_JoinGame:
			s.mu.Lock()
			r := s.rooms[m.JoinGame.Game]
			// A started game takes back its own players only (the real
			// server's waiting rooms, the game's netresume.go).
			if r == nil || !r.open || r.started && !r.players[l.user] {
				s.mu.Unlock()
				fail(l, env, lobbyv1.ErrorCode_ERROR_CODE_GAME_CLOSED, "closed")
				continue
			}
			ans := s.add(r, l)
			s.mu.Unlock()
			reply(l, env, ans)
		case *lobbyv1.Envelope_LeaveGame:
			s.mu.Lock()
			s.leave(l)
			s.mu.Unlock()
			reply(l, env, ok())
		case *lobbyv1.Envelope_StartGame:
			s.mu.Lock()
			if r := l.room; r != nil {
				r.started, r.open = true, false
				s.members(r)
				if r.public { // off everyone's list, as the real server tells
					s.everyone(&lobbyv1.Envelope{Msg: &lobbyv1.Envelope_GameRemoved{GameRemoved: &lobbyv1.GameRemoved{Id: r.id}}})
				}
			}
			s.mu.Unlock()
			reply(l, env, ok())
		case *lobbyv1.Envelope_SetOpen:
			s.mu.Lock()
			if r := l.room; r != nil {
				r.open = m.SetOpen.Open
				s.members(r)
			}
			s.mu.Unlock()
			reply(l, env, ok())
		case *lobbyv1.Envelope_ContinueGame:
			s.mu.Lock()
			if r := l.room; r != nil {
				s.continues[r.id] = m.ContinueGame.Previous
			}
			s.mu.Unlock()
			reply(l, env, ok())
		case *lobbyv1.Envelope_DesyncReport:
			s.mu.Lock()
			r := l.room
			if r != nil {
				s.desyncs = append(s.desyncs, Desync{Game: r.id, NetID: l.netID, Report: m.DesyncReport})
			}
			s.mu.Unlock()
			if r == nil {
				fail(l, env, lobbyv1.ErrorCode_ERROR_CODE_BAD_REQUEST, "not in a game")
				continue
			}
			reply(l, env, ok())
		case *lobbyv1.Envelope_Frame:
			s.relay(l, m.Frame)
		}
	}
}

func (r *room) ids() []uint32 {
	var ids []uint32
	for id := range r.members {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func (r *room) info() *lobbyv1.GameInfo {
	g := &lobbyv1.GameInfo{Id: r.id, Description: r.description, State: lobbyv1.GameState_GAME_STATE_OPEN}
	for _, id := range r.ids() {
		g.Players = append(g.Players, &lobbyv1.Player{Name: r.members[id].user, NetId: id})
	}
	return g
}

// add puts l in r and tells the others; under mu.
func (s *Server) add(r *room, l *link) *lobbyv1.Envelope {
	id := r.next
	r.next++
	r.members[id] = l
	if r.players == nil {
		r.players = map[string]bool{}
	}
	r.players[l.user] = true
	l.room, l.netID = r, id
	s.members(r)
	return &lobbyv1.Envelope{Msg: &lobbyv1.Envelope_GameJoined{GameJoined: &lobbyv1.GameJoined{
		Game: r.info(), NetId: id, Generation: r.gen, Open: r.open, Members: r.ids()}}}
}

// members bumps the generation and tells every member; under mu.
func (s *Server) members(r *room) {
	r.gen++
	env := &lobbyv1.Envelope{Msg: &lobbyv1.Envelope_Members{Members: &lobbyv1.Members{
		Generation: r.gen, Open: r.open, Ids: r.ids()}}}
	for _, m := range r.members {
		m.send(env)
	}
}

// leave takes l out of its room; under mu.
func (s *Server) leave(l *link) {
	r := l.room
	if r == nil {
		return
	}
	delete(r.members, l.netID)
	lost := &lobbyv1.Envelope{Msg: &lobbyv1.Envelope_MemberLost{MemberLost: &lobbyv1.MemberLost{NetId: l.netID}}}
	for _, m := range r.members {
		m.send(lost)
	}
	l.room, l.netID = nil, 0
	s.members(r)
}

func (s *Server) relay(l *link, f *lobbyv1.GameFrame) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := l.room
	if r == nil {
		return
	}
	env := &lobbyv1.Envelope{Msg: &lobbyv1.Envelope_Frame{Frame: &lobbyv1.GameFrame{
		From: l.netID, To: f.To, Msg: f.Msg, Payload: f.Payload}}}
	for id, m := range r.members {
		if f.To == broadcast && id != l.netID || f.To == id {
			m.send(env)
			s.frames++
		}
	}
}

// Certs is a certificate for ips, the server's TLS config with it, a client
// config trusting it, and the certificate in PEM.
func Certs(ips []net.IP) (server, client *tls.Config, certPEM []byte, err error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, nil, err
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "lobbytest"},
		IPAddresses: ips,
		NotBefore:   time.Now().Add(-time.Hour), NotAfter: time.Now().Add(30 * 24 * time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign, IsCA: true, BasicConstraintsValid: true,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, nil, nil, err
	}
	cert, _ := x509.ParseCertificate(der)
	pool := x509.NewCertPool()
	pool.AddCert(cert)
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	return &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}},
		&tls.Config{RootCAs: pool}, certPEM, nil
}
