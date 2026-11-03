//go:build !js

package lobby

import (
	"context"
	"crypto/tls"
	"errors"
	"time"

	"github.com/openorion2/openorion2/lobbynet/client"
	"github.com/openorion2/openorion2/lobbynet/lobbyv1"
)

// Build names the game in the server's logs.
const Build = "openorion2"

// Conn is a connection to the lobby.
type Conn struct {
	c     *client.Client
	User  string // the logged-in account, as the server spells it
	State State  // what the lobby looks like, kept up by Poll

	ep    *endpoint           // the game being played through the lobby
	roomQ []*lobbyv1.Envelope // its pushes that came before its endpoint
}

// roomPush reports a push about the room this player is in.
func (c *Conn) roomPush(env *lobbyv1.Envelope) bool {
	switch m := env.Msg.(type) {
	case *lobbyv1.Envelope_Frame, *lobbyv1.Envelope_Members, *lobbyv1.Envelope_MemberLost:
		return true
	case *lobbyv1.Envelope_GameUpdate:
		// Over: a FINISHED update of his game (GameRemoved only takes a
		// game off the list, as at its start).
		g := m.GameUpdate.Game
		return c.State.Mine != nil && g.GetId() == c.State.Mine.Id && g.GetState() == lobbyv1.GameState_GAME_STATE_FINISHED
	}
	return false
}

// attach makes ep the room's endpoint and gives it what came before.
func (c *Conn) attach(ep *endpoint) {
	c.ep = ep
	for _, env := range c.roomQ {
		ep.handle(env)
	}
	c.roomQ = nil
}

func (c *Conn) detach(ep *endpoint) {
	if c.ep == ep {
		c.ep = nil
	}
}

// Dial connects and checks the protocol version; a *client.VersionError's
// text is for the player.
func Dial(s Settings) (*Conn, error) {
	c, err := client.Dial(s.Server, s.TLS, Build)
	if err != nil {
		return nil, err
	}
	return &Conn{c: c}, nil
}

// LoginTimeout bounds a login: Argon2 on this machine and two round trips.
const LoginTimeout = 30 * time.Second

// Login logs in with the password and returns the new session.
func (c *Conn) Login(user, password string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), LoginTimeout)
	defer cancel()
	session, name, err := c.c.Login(ctx, user, password)
	if err == nil {
		c.User, c.State.Me = name, name
	}
	return session, err
}

// Resume logs in with a remembered session.
func (c *Conn) Resume(session string) error {
	ctx, cancel := context.WithTimeout(context.Background(), LoginTimeout)
	defer cancel()
	name, err := c.c.Resume(ctx, session)
	if err == nil {
		c.User, c.State.Me = name, name
	}
	return err
}

// Logout ends the session on the server.
func (c *Conn) Logout() error {
	ctx, cancel := context.WithTimeout(context.Background(), LoginTimeout)
	defer cancel()
	return c.c.Logout(ctx)
}

// Close drops the connection.
func (c *Conn) Close() error { return c.c.Close() }

// SessionGone reports a login refused because the session ended.
func SessionGone(err error) bool {
	var le *client.LoginError
	return errors.As(err, &le) && le.Reason == lobbyv1.LoginFailure_LOGIN_FAILURE_SESSION_GONE
}

// RequestTimeout bounds a lobby request.
const RequestTimeout = 15 * time.Second

func (c *Conn) ctx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), RequestTimeout)
}

// Poll applies the pushes that arrived to the lobby's state; it reports
// whether anything changed, and the connection's error once it is gone.
func (c *Conn) Poll() (bool, error) {
	changed := false
	for {
		env, ok := c.c.Next()
		if !ok {
			break
		}
		if c.roomPush(env) {
			if c.ep != nil {
				c.ep.handle(env)
			} else if c.State.Mine != nil {
				c.roomQ = append(c.roomQ, env)
			}
		}
		changed = c.State.apply(env) || changed
	}
	return changed, c.c.Err()
}

// Say sends a line to the chat.
func (c *Conn) Say(text string) error {
	ctx, cancel := c.ctx()
	defer cancel()
	return c.c.Say(ctx, text)
}

// Whisper sends a private line to user to.
func (c *Conn) Whisper(to, text string) error {
	ctx, cancel := c.ctx()
	defer cancel()
	return c.c.Whisper(ctx, to, text)
}

// MarkRead marks d's lines read, on the server too.
func (c *Conn) MarkRead(d *Dialog) error {
	d.Unread = 0
	if d.last <= d.read {
		return nil
	}
	d.read = d.last
	ctx, cancel := c.ctx()
	defer cancel()
	return c.c.ReadPrivate(ctx, d.With, d.last)
}

// CreateGame makes a game and joins it.
func (c *Conn) CreateGame(description string, public bool) (*lobbyv1.GameJoined, error) {
	ctx, cancel := c.ctx()
	defer cancel()
	j, err := c.c.CreateGame(ctx, description, public)
	if err == nil {
		c.State.joined(j)
		c.roomQ = nil
	}
	return j, err
}

// JoinGame joins a game by its id.
func (c *Conn) JoinGame(id string) (*lobbyv1.GameJoined, error) {
	ctx, cancel := c.ctx()
	defer cancel()
	j, err := c.c.JoinGame(ctx, id)
	if err == nil {
		c.State.joined(j)
		c.roomQ = nil
	}
	return j, err
}

// LeaveGame leaves the current game.
func (c *Conn) LeaveGame() error {
	ctx, cancel := c.ctx()
	defer cancel()
	err := c.c.LeaveGame(ctx)
	if err == nil {
		c.State.Mine, c.State.MyID, c.roomQ = nil, 0, nil
	}
	return err
}

// ContinueGame (the creator) links the game to the one a save was made in.
func (c *Conn) ContinueGame(previous string) error {
	ctx, cancel := c.ctx()
	defer cancel()
	return c.c.ContinueGame(ctx, previous)
}

// ReportCrash sends a crash report of an earlier run (port-only).
func (c *Conn) ReportCrash(r *lobbyv1.CrashReport) error {
	ctx, cancel := context.WithTimeout(context.Background(), ReportDesyncTimeout)
	defer cancel()
	return c.c.ReportCrash(ctx, r)
}

// ErrNoSession: the player has not logged in to the lobby on this machine.
var ErrNoSession = errors.New("log in to the online lobby once (Multiplayer) to send it")

// SendCrash sends r with the session the game in dir keeps, over its own
// connection (tls, if set, as in the tests).
func SendCrash(dir string, tls *tls.Config, r *lobbyv1.CrashReport) error {
	set, err := LoadSettings(dir)
	if err != nil {
		return err
	}
	if tls != nil {
		set.TLS = tls
	}
	rem := LoadRemembered(dir)
	if rem.Session == "" {
		return ErrNoSession
	}
	conn, err := Dial(set)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := conn.Resume(rem.Session); err != nil {
		return err
	}
	return conn.ReportCrash(r)
}

// StartGame (the creator) takes the game off the list and closes it.
func (c *Conn) StartGame() error {
	ctx, cancel := c.ctx()
	defer cancel()
	return c.c.StartGame(ctx)
}

// SetOpen (the creator) lets joiners in or not.
func (c *Conn) SetOpen(open bool) error {
	ctx, cancel := c.ctx()
	defer cancel()
	return c.c.SetOpen(ctx, open)
}

// Alive reports the connection is still up.
func (c *Conn) Alive() bool { return c.c.Err() == nil }
