// Package client is the game's side of a lobby link: requests that wait for
// their answers, and pushes (chat, game list, frames) queued for the game
// to poll.
package client

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/openorion2/openorion2/lobbynet/lobbyv1"
	"github.com/openorion2/openorion2/lobbynet/srp"
	"github.com/openorion2/openorion2/lobbynet/wire"
)

// DialTimeout bounds the connection and the Hello's answer.
const DialTimeout = 10 * time.Second

// VersionError is a server of another protocol; Text is for the player.
type VersionError struct {
	Server uint32
	Text   string
}

func (e *VersionError) Error() string { return e.Text }

// ServerError is a request the server refused; Text is for the player.
type ServerError struct {
	Code lobbyv1.ErrorCode
	Text string
}

func (e *ServerError) Error() string { return e.Text }

// LoginError is a refused login.
type LoginError struct {
	Reason     lobbyv1.LoginFailure
	RetryAfter time.Duration
}

func (e *LoginError) Error() string {
	switch e.Reason {
	case lobbyv1.LoginFailure_LOGIN_FAILURE_WRONG:
		return "Wrong username or password."
	case lobbyv1.LoginFailure_LOGIN_FAILURE_TOO_MANY:
		return fmt.Sprintf("Too many attempts. Try again in %d s.", int((e.RetryAfter+time.Second-1)/time.Second))
	case lobbyv1.LoginFailure_LOGIN_FAILURE_NOT_CONFIRMED:
		return "The account is not confirmed yet: enter the code from the e-mail on the website."
	case lobbyv1.LoginFailure_LOGIN_FAILURE_SESSION_GONE:
		return "The session has ended. Log in again."
	case lobbyv1.LoginFailure_LOGIN_FAILURE_BANNED:
		if e.RetryAfter > 0 {
			return fmt.Sprintf("This account is banned from the lobby for %s more.", e.RetryAfter.Round(time.Minute))
		}
		return "This account is banned from the lobby."
	}
	return "Login failed."
}

// ErrClosed is a request on a link that is gone.
var ErrClosed = errors.New("lobby: connection lost")

// Client is one lobby link.
type Client struct {
	w *wire.Conn

	mu      sync.Mutex
	next    uint32
	pending map[uint32]chan *lobbyv1.Envelope
	pushes  []*lobbyv1.Envelope
	err     error // set once the link is gone
	done    chan struct{}
}

// Dial connects to addr over TLS and says hello. cfg may be nil (the
// system's roots, the address's host name). build names the game's build
// in the server's logs.
func Dial(addr string, cfg *tls.Config, build string) (*Client, error) {
	d := &tls.Dialer{NetDialer: &net.Dialer{Timeout: DialTimeout}, Config: cfg}
	nc, err := d.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	w := wire.NewConn(nc)
	if err := hello(nc, w, build); err != nil {
		w.Close()
		return nil, err
	}
	c := &Client{w: w, pending: map[uint32]chan *lobbyv1.Envelope{}, done: make(chan struct{})}
	go c.read()
	return c, nil
}

func hello(nc net.Conn, w *wire.Conn, build string) error {
	err := w.Send(&lobbyv1.Envelope{Msg: &lobbyv1.Envelope_Hello{Hello: &lobbyv1.Hello{
		Protocol: lobbyv1.Protocol, Build: build}}})
	if err != nil {
		return err
	}
	nc.SetReadDeadline(time.Now().Add(DialTimeout))
	env, err := wire.Read(nc)
	if err != nil {
		return err
	}
	switch m := env.Msg.(type) {
	case *lobbyv1.Envelope_Welcome:
		return nil
	case *lobbyv1.Envelope_VersionMismatch:
		return &VersionError{Server: m.VersionMismatch.ServerProtocol, Text: m.VersionMismatch.Text}
	}
	return errors.New("lobby: unexpected answer to hello")
}

func (c *Client) read() {
	defer close(c.done)
	for {
		env, err := c.w.Recv()
		c.mu.Lock()
		if err != nil {
			c.err = ErrClosed
			for id, ch := range c.pending {
				close(ch)
				delete(c.pending, id)
			}
			c.mu.Unlock()
			return
		}
		if ch := c.pending[env.Id]; env.Id != 0 && ch != nil {
			delete(c.pending, env.Id)
			ch <- env
		} else {
			c.pushes = append(c.pushes, env)
		}
		c.mu.Unlock()
	}
}

// Next returns the next pushed message, if any; it never blocks.
func (c *Client) Next() (*lobbyv1.Envelope, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.pushes) == 0 {
		return nil, false
	}
	env := c.pushes[0]
	c.pushes[0] = nil
	c.pushes = c.pushes[1:]
	return env, true
}

// Err is nil while the link is up, ErrClosed after.
func (c *Client) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err
}

// Request sends env and waits for its answer. A server Error comes back
// as a *ServerError.
func (c *Client) Request(ctx context.Context, env *lobbyv1.Envelope) (*lobbyv1.Envelope, error) {
	ch := make(chan *lobbyv1.Envelope, 1)
	c.mu.Lock()
	if c.err != nil {
		c.mu.Unlock()
		return nil, c.err
	}
	c.next++
	if c.next == 0 {
		c.next = 1
	}
	env.Id = c.next
	c.pending[env.Id] = ch
	c.mu.Unlock()
	if err := c.w.Send(env); err != nil {
		c.w.Close()
		return nil, ErrClosed
	}
	select {
	case ans, ok := <-ch:
		if !ok {
			return nil, ErrClosed
		}
		if e := ans.GetError(); e != nil {
			return nil, &ServerError{Code: e.Code, Text: e.Text}
		}
		return ans, nil
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pending, env.Id)
		c.mu.Unlock()
		return nil, ctx.Err()
	}
}

// Login logs in with SRP: the password does not leave the game. It returns
// the session, for Resume next time, and the account's name as the server
// spells it.
func (c *Client) Login(ctx context.Context, username, password string) (session, name string, err error) {
	ans, err := c.Request(ctx, &lobbyv1.Envelope{Msg: &lobbyv1.Envelope_LoginStart{
		LoginStart: &lobbyv1.LoginStart{Username: username}}})
	if err != nil {
		return "", "", err
	}
	if f := ans.GetLoginFailed(); f != nil {
		return "", "", loginError(f)
	}
	ch := ans.GetLoginChallenge()
	if ch == nil || ch.Kdf == nil || ch.Kdf.Threads > 255 {
		return "", "", errors.New("lobby: unexpected answer to the login")
	}
	kdf := srp.KDF{Time: ch.Kdf.Time, MemoryKiB: ch.Kdf.MemoryKib, Threads: uint8(ch.Kdf.Threads)}
	if kdf.MemoryKiB > 1<<20 || kdf.Time > 100 {
		return "", "", errors.New("lobby: the server asks for too much work")
	}
	sc := srp.NewClient(srp.Group3072)
	m1, err := sc.Proof(srp.X(username, password, ch.Salt, kdf), ch.B)
	if err != nil {
		return "", "", err
	}
	ans, err = c.Request(ctx, &lobbyv1.Envelope{Msg: &lobbyv1.Envelope_LoginProof{
		LoginProof: &lobbyv1.LoginProof{A: sc.Public(), M1: m1}}})
	if err != nil {
		return "", "", err
	}
	if f := ans.GetLoginFailed(); f != nil {
		return "", "", loginError(f)
	}
	ok := ans.GetLoginOk()
	if ok == nil {
		return "", "", errors.New("lobby: unexpected answer to the login")
	}
	if !sc.Check(ok.M2) {
		return "", "", errors.New("lobby: the server could not prove it knows the account")
	}
	return ok.Session, ok.GetUser().GetName(), nil
}

// Resume logs in with a session of an earlier login.
func (c *Client) Resume(ctx context.Context, session string) (name string, err error) {
	ans, err := c.Request(ctx, &lobbyv1.Envelope{Msg: &lobbyv1.Envelope_Resume{
		Resume: &lobbyv1.Resume{Session: session}}})
	if err != nil {
		return "", err
	}
	if f := ans.GetLoginFailed(); f != nil {
		return "", loginError(f)
	}
	ok := ans.GetLoginOk()
	if ok == nil {
		return "", errors.New("lobby: unexpected answer to the resume")
	}
	return ok.GetUser().GetName(), nil
}

// Logout ends the session.
func (c *Client) Logout(ctx context.Context) error {
	_, err := c.Request(ctx, &lobbyv1.Envelope{Msg: &lobbyv1.Envelope_Logout{Logout: &lobbyv1.Logout{}}})
	return err
}

// SendFrame sends a game message to net id to (or 0x7ffe, every other
// member) of the current game; it is not answered.
func (c *Client) SendFrame(to, msg uint16, payload []byte) error {
	c.mu.Lock()
	err := c.err
	c.mu.Unlock()
	if err != nil {
		return err
	}
	if err := c.w.Send(&lobbyv1.Envelope{Msg: &lobbyv1.Envelope_Frame{Frame: &lobbyv1.GameFrame{
		To: uint32(to), Msg: uint32(msg), Payload: payload}}}); err != nil {
		c.w.Close()
		return ErrClosed
	}
	return nil
}

// ReportDesync sends a desync report about the current game; answered by
// Ok (a server that does not know it answers an Error).
func (c *Client) ReportDesync(ctx context.Context, r *lobbyv1.DesyncReport) error {
	_, err := c.Request(ctx, &lobbyv1.Envelope{Msg: &lobbyv1.Envelope_DesyncReport{DesyncReport: r}})
	return err
}

// ReportCrash sends a crash report (the player agreed); answered Ok.
func (c *Client) ReportCrash(ctx context.Context, r *lobbyv1.CrashReport) error {
	_, err := c.Request(ctx, &lobbyv1.Envelope{Msg: &lobbyv1.Envelope_CrashReport{CrashReport: r}})
	return err
}

// Say sends a line to the lobby's chat; it comes back as a ChatLine push
// like everyone's.
func (c *Client) Say(ctx context.Context, text string) error {
	_, err := c.Request(ctx, &lobbyv1.Envelope{Msg: &lobbyv1.Envelope_Say{Say: &lobbyv1.Say{Text: text}}})
	return err
}

// Whisper sends a private line to user to, online or not.
func (c *Client) Whisper(ctx context.Context, to, text string) error {
	_, err := c.Request(ctx, &lobbyv1.Envelope{Msg: &lobbyv1.Envelope_Whisper{Whisper: &lobbyv1.Whisper{To: to, Text: text}}})
	return err
}

// ReadPrivate marks the private lines from user from up to id upTo read.
func (c *Client) ReadPrivate(ctx context.Context, from string, upTo uint64) error {
	_, err := c.Request(ctx, &lobbyv1.Envelope{Msg: &lobbyv1.Envelope_ReadPrivate{ReadPrivate: &lobbyv1.ReadPrivate{From: from, UpTo: upTo}}})
	return err
}

// CreateGame makes a game and joins it as its creator (net id 1).
func (c *Client) CreateGame(ctx context.Context, description string, public bool) (*lobbyv1.GameJoined, error) {
	return c.joined(c.Request(ctx, &lobbyv1.Envelope{Msg: &lobbyv1.Envelope_CreateGame{
		CreateGame: &lobbyv1.CreateGame{Description: description, Public: public}}}))
}

// JoinGame joins a game by its UUID.
func (c *Client) JoinGame(ctx context.Context, id string) (*lobbyv1.GameJoined, error) {
	return c.joined(c.Request(ctx, &lobbyv1.Envelope{Msg: &lobbyv1.Envelope_JoinGame{
		JoinGame: &lobbyv1.JoinGame{Game: id}}}))
}

func (c *Client) joined(ans *lobbyv1.Envelope, err error) (*lobbyv1.GameJoined, error) {
	if err != nil {
		return nil, err
	}
	j := ans.GetGameJoined()
	if j == nil || j.Game == nil {
		return nil, errors.New("lobby: unexpected answer to a game request")
	}
	return j, nil
}

// ContinueGame (the creator) links the current game to the lobby game a
// loaded save was made in.
func (c *Client) ContinueGame(ctx context.Context, previous string) error {
	_, err := c.Request(ctx, &lobbyv1.Envelope{Msg: &lobbyv1.Envelope_ContinueGame{
		ContinueGame: &lobbyv1.ContinueGame{Previous: previous}}})
	return err
}

// LeaveGame leaves the current game.
func (c *Client) LeaveGame(ctx context.Context) error {
	_, err := c.Request(ctx, &lobbyv1.Envelope{Msg: &lobbyv1.Envelope_LeaveGame{LeaveGame: &lobbyv1.LeaveGame{}}})
	return err
}

// StartGame (the creator) closes the game to joiners and takes it off the list.
func (c *Client) StartGame(ctx context.Context) error {
	_, err := c.Request(ctx, &lobbyv1.Envelope{Msg: &lobbyv1.Envelope_StartGame{StartGame: &lobbyv1.StartGame{}}})
	return err
}

// SetOpen (the creator) lets joiners in or not.
func (c *Client) SetOpen(ctx context.Context, open bool) error {
	_, err := c.Request(ctx, &lobbyv1.Envelope{Msg: &lobbyv1.Envelope_SetOpen{SetOpen: &lobbyv1.SetOpen{Open: open}}})
	return err
}

func loginError(f *lobbyv1.LoginFailed) error {
	return &LoginError{Reason: f.Reason, RetryAfter: time.Duration(f.RetryAfterMs) * time.Millisecond}
}

// Close ends the link.
func (c *Client) Close() error {
	err := c.w.Close()
	<-c.done
	return err
}
