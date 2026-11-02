//go:build !js

package lobby

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/openorion2/openorion2/internal/netplay"
	"github.com/openorion2/openorion2/lobbynet/lobbyv1"
)

// Network is netplay.Network over the lobby: the game the player created
// or joined in the lobby, played through the server's room. Create and
// Join hand out its endpoint (the room is already joined); Games lists
// only it.
type Network struct {
	conn *Conn
	game *lobbyv1.GameJoined
}

// NewNetwork plays game, joined over conn.
func NewNetwork(conn *Conn, game *lobbyv1.GameJoined) *Network {
	return &Network{conn: conn, game: game}
}

// GameID is the lobby game's UUID.
func (n *Network) GameID() string { return n.game.Game.Id }

// Continue links the game to the lobby game a loaded save was made in.
func (n *Network) Continue(previous string) error { return n.conn.ContinueGame(previous) }

// JoinedGame is the game's name for the join list (the game picks it at
// once: there is nothing else to choose).
func (n *Network) JoinedGame() string { return gameName(n.game.Game) }

// gameName is what the original's 9-character game name shows: the
// description's ASCII (the game's fonts have no more), "Lobby" if none.
func gameName(g *lobbyv1.GameInfo) string {
	var b []byte
	for _, r := range g.Description {
		if r >= 0x20 && r < 0x7f && len(b) < 9 {
			b = append(b, byte(r))
		}
	}
	if s := strings.TrimSpace(string(b)); s != "" {
		return s
	}
	return "Lobby"
}

func (n *Network) endpoint() (netplay.Endpoint, error) {
	if n.conn.ep != nil {
		return nil, errors.New("lobby: the game is already being played")
	}
	if n.conn.State.Mine == nil || n.conn.State.Mine.Id != n.game.Game.Id {
		return nil, errors.New("lobby: no longer in that game")
	}
	ep := &endpoint{conn: n.conn, id: uint16(n.game.NetId), name: gameName(n.game.Game),
		open: n.game.Open, gen: n.game.Generation, creator: n.game.NetId == 1}
	for _, id := range n.game.Members {
		ep.users = append(ep.users, uint16(id))
	}
	n.conn.attach(ep)
	return ep, nil
}

// Create is Net_Create_Game_: the creator's endpoint.
func (n *Network) Create(game, player string) (netplay.Endpoint, error) { return n.endpoint() }

// Join is Net_Join_Game_: a joiner's endpoint.
func (n *Network) Join(game, player string) (netplay.Endpoint, error) { return n.endpoint() }

// ReportDesyncTimeout bounds a desync report's send.
const ReportDesyncTimeout = time.Minute

// ReportDesync sends the desync check's finding to the lobby in the
// background (the game does not wait): the state hashes of this machine
// and of net id other at a stardate, and this machine's save of it. done,
// if set, hears how it went (on another goroutine).
func (n *Network) ReportDesync(stardate uint32, other uint16, own, theirs [32]byte, build string, save []byte, done func(error)) {
	var z bytes.Buffer
	w := gzip.NewWriter(&z)
	w.Write(save)
	w.Close()
	r := &lobbyv1.DesyncReport{Stardate: stardate, OtherNetId: uint32(other), Hash: own[:], OtherHash: theirs[:],
		Build: build, Save: z.Bytes()}
	c := n.conn.c
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), ReportDesyncTimeout)
		defer cancel()
		err := c.ReportDesync(ctx, r)
		if done != nil {
			done(err)
		}
	}()
}

// Games is Net_Update_Game_List_: the one game.
func (n *Network) Games() ([]netplay.GameInfo, error) {
	g := netplay.GameInfo{Name: gameName(n.game.Game), Open: true}
	for _, id := range n.game.Members {
		g.Users = append(g.Users, uint16(id))
	}
	return []netplay.GameInfo{g}, nil
}

// endpoint is one station of a lobby game. The game drains it on its own
// goroutine (Recv polls the connection); the lock covers the readers the
// netplay contract allows on others.
type endpoint struct {
	conn    *Conn
	id      uint16
	name    string
	creator bool

	mu      sync.Mutex
	started bool // the creator has started the game (StartGame)
	frames  []netplay.Frame
	users   []uint16
	dropped []uint16
	open    bool
	gen     uint32
	err     error
}

var _ netplay.Endpoint = (*endpoint)(nil)

// handle folds one push about the room in.
func (e *endpoint) handle(env *lobbyv1.Envelope) {
	e.mu.Lock()
	defer e.mu.Unlock()
	switch m := env.Msg.(type) {
	case *lobbyv1.Envelope_Frame:
		f := m.Frame
		e.frames = append(e.frames, netplay.Frame{ID: uint16(f.Msg), From: uint16(f.From), To: uint16(f.To), Payload: f.Payload})
	case *lobbyv1.Envelope_Members:
		if m.Members.Generation < e.gen {
			return
		}
		e.users = e.users[:0]
		for _, id := range m.Members.Ids {
			e.users = append(e.users, uint16(id))
		}
		e.open, e.gen = m.Members.Open, m.Members.Generation
	case *lobbyv1.Envelope_MemberLost:
		e.dropped = append(e.dropped, uint16(m.MemberLost.NetId))
	case *lobbyv1.Envelope_GameUpdate: // FINISHED (roomPush): the room is gone
		if e.err == nil {
			e.err = netplay.ErrLost
		}
	}
}

func (e *endpoint) fail(err error) {
	e.mu.Lock()
	if e.err == nil {
		e.err = err
	}
	e.mu.Unlock()
}

func (e *endpoint) ID() uint16 { return e.id }

func (e *endpoint) Game() netplay.GameInfo {
	e.mu.Lock()
	defer e.mu.Unlock()
	return netplay.GameInfo{Name: e.name, Open: e.open && e.err == nil, Users: slices.Clone(e.users)}
}

func (e *endpoint) Users() []uint16 {
	e.mu.Lock()
	defer e.mu.Unlock()
	return slices.Clone(e.users)
}

func (e *endpoint) Dropped() []uint16 {
	e.mu.Lock()
	defer e.mu.Unlock()
	return slices.Clone(e.dropped)
}

func (e *endpoint) Send(to, id uint16, payload []byte) error {
	e.mu.Lock()
	err := e.err
	member := to == netplay.Broadcast || slices.Contains(e.users, to)
	e.mu.Unlock()
	switch {
	case err != nil:
		return err
	case !member:
		return netplay.ErrNoUser
	case to == e.id:
		e.mu.Lock()
		e.frames = append(e.frames, netplay.Frame{ID: id, From: e.id, To: to, Payload: slices.Clone(payload)})
		e.mu.Unlock()
		return nil
	}
	if err := e.conn.c.SendFrame(to, id, payload); err != nil {
		e.fail(netplay.ErrLost)
		return netplay.ErrLost
	}
	return nil
}

func (e *endpoint) Recv() (netplay.Frame, bool) {
	if _, err := e.conn.Poll(); err != nil {
		e.fail(netplay.ErrLost)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.frames) == 0 {
		return netplay.Frame{}, false
	}
	f := e.frames[0]
	e.frames = e.frames[1:]
	return f, true
}

// SetOpen is Net_Enable/Disable_New_Players_. The game closes a lobby game
// only to start it: that is the lobby's StartGame (off the list for good).
//
// After the start any member may open and close the started game again,
// to take a lost player back (port-only, the game's netresume.go).
func (e *endpoint) SetOpen(open bool) error {
	e.mu.Lock()
	start := e.creator && !e.started && !open
	if start {
		e.started = true
	}
	e.mu.Unlock()
	if start {
		return e.conn.StartGame()
	}
	return e.conn.SetOpen(open)
}

func (e *endpoint) Err() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.err == nil && e.conn.c.Err() != nil {
		return netplay.ErrLost
	}
	return e.err
}

// Close leaves the game; the lobby connection stays.
func (e *endpoint) Close() error {
	e.fail(netplay.ErrClosed)
	e.conn.detach(e)
	if e.conn.State.Mine != nil {
		return e.conn.LeaveGame()
	}
	return nil
}
