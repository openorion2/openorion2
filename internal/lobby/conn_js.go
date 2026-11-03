//go:build js

package lobby

import (
	"crypto/tls"
	"errors"

	"github.com/openorion2/openorion2/internal/netplay"
	"github.com/openorion2/openorion2/lobbynet/lobbyv1"
)

// ErrUnavailable: the browser build has no lobby yet (it needs WSS).
var ErrUnavailable = errors.New("The online lobby is not available in the browser yet.")

// Conn is a connection to the lobby; the browser has none.
type Conn struct {
	User  string
	State State
}

func Dial(Settings) (*Conn, error)                { return nil, ErrUnavailable }
func (c *Conn) Login(_, _ string) (string, error) { return "", ErrUnavailable }
func (c *Conn) Resume(string) error               { return ErrUnavailable }
func (c *Conn) Logout() error                     { return ErrUnavailable }
func (c *Conn) Close() error                      { return nil }
func SessionGone(error) bool                      { return false }

func (c *Conn) Poll() (bool, error)                                  { return false, ErrUnavailable }
func (c *Conn) Say(string) error                                     { return ErrUnavailable }
func (c *Conn) Whisper(_, _ string) error                            { return ErrUnavailable }
func (c *Conn) MarkRead(*Dialog) error                               { return ErrUnavailable }
func (c *Conn) CreateGame(string, bool) (*lobbyv1.GameJoined, error) { return nil, ErrUnavailable }
func (c *Conn) JoinGame(string) (*lobbyv1.GameJoined, error)         { return nil, ErrUnavailable }
func (c *Conn) LeaveGame() error                                     { return ErrUnavailable }

// Network is netplay.Network over the lobby; the browser has none.
type Network struct{}

func NewNetwork(*Conn, *lobbyv1.GameJoined) *Network { return &Network{} }
func (n *Network) JoinedGame() string                { return "" }
func (n *Network) Create(_, _ string) (netplay.Endpoint, error) {
	return nil, ErrUnavailable
}
func (n *Network) Join(_, _ string) (netplay.Endpoint, error) { return nil, ErrUnavailable }
func (n *Network) Games() ([]netplay.GameInfo, error)         { return nil, ErrUnavailable }
func (c *Conn) StartGame() error                              { return ErrUnavailable }
func (c *Conn) SetOpen(bool) error                            { return ErrUnavailable }
func (c *Conn) Alive() bool                                   { return false }
func (c *Conn) ContinueGame(string) error                     { return ErrUnavailable }
func (c *Conn) ReportCrash(*lobbyv1.CrashReport) error        { return ErrUnavailable }

// SendCrash: no lobby in the browser.
func SendCrash(string, *tls.Config, *lobbyv1.CrashReport) error { return ErrUnavailable }
func (n *Network) GameID() string                               { return "" }
func (n *Network) Continue(string) error                        { return ErrUnavailable }
