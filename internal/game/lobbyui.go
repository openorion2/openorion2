package game

import (
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/openorion2/openorion2/internal/lobby"
	"github.com/openorion2/openorion2/internal/save"
	"github.com/openorion2/openorion2/lobbynet/lobbyv1"

	"github.com/openorion2/openorion2/lobbynet/client"
)

// port-only: the online lobby (docs/lobby/client.md), behind the multiplayer
// screen's TOTAL ENTERTAINMENT NETWORK button (MULTIGM.LBX 0x100, hot key
// T, whose handler the 1.40 patch removed). The boxes are the TCP address
// boxes (netui.go): the username, then the password shown as '*'.

// lobbyHeading heads the lobby's boxes and notices.
const lobbyHeading = "GALACTIC COUNCIL ONLINE LOBBY"

// lobbyNotice shows the frame with a heading and a line under it while the
// game waits on the network.
func (g *Game) lobbyNotice(line string) error {
	e := g.e
	// The page flip draws the visible fields: the screen under it must
	// not show through (the multiplayer screen's buttons).
	e.ClearFields()
	e.V.SetPageOff()
	if err := g.drawNetAddressBack(&netAddressBox{heading: lobbyHeading}); err != nil {
		return err
	}
	t := e.Text()
	if err := t.Print((0x280-t.Width([]byte(line)))/2, 0xe6, line); err != nil {
		return err
	}
	return e.TogglePages()
}

// lobbyErrorText is an error for the player.
func lobbyErrorText(server string, err error) string {
	var ve *client.VersionError
	var le *client.LoginError
	var se *client.ServerError
	var ne net.Error
	switch {
	case errors.As(err, &ve):
		return ve.Text
	case errors.As(err, &le):
		return le.Error()
	case errors.As(err, &se):
		return se.Text
	case errors.Is(err, client.ErrClosed):
		return "The connection to the lobby was lost."
	case errors.As(err, &ne), errors.Is(err, net.ErrClosed):
		return fmt.Sprintf("Cannot reach the lobby at %s.", server)
	}
	s := err.Error()
	if strings.Contains(s, "certificate") {
		return fmt.Sprintf("The lobby at %s has no valid certificate.", server)
	}
	return fmt.Sprintf("Lobby: %s", s)
}

// lobbyScreen is the TEN button: connect, log in (with the remembered
// session if it still works; at once on a connection kept from a game),
// then the lobby. It returns the game the player created (load: to load a
// saved game into it) or joined there, nil when he left the lobby; with a
// game the connection stays open in g.lobbyConn to play it.
func (g *Game) lobbyScreen() (j *lobbyv1.GameJoined, load bool, err error) {
	conn := g.lobbyConn
	g.lobbyConn = nil
	var rem *lobby.Remembered
	if conn != nil && conn.Alive() {
		r := lobby.LoadRemembered(g.dir)
		rem = &r
	} else {
		if conn != nil {
			conn.Close()
		}
		if conn, rem, err = g.lobbyLogin(); conn == nil || err != nil {
			return nil, false, err
		}
	}
	j, load, err = g.lobbyHome(conn, rem)
	if j != nil && err == nil {
		g.lobbyConn = conn
		return j, load, nil
	}
	conn.Close()
	return nil, false, err
}

// lobbyLogin connects and logs in; nil without an error when the player
// gave up or was told why not.
func (g *Game) lobbyLogin() (*lobby.Conn, *lobby.Remembered, error) {
	set, err := lobby.LoadSettings(g.dir)
	if err != nil {
		return nil, nil, g.messageBox("lobby.yaml: " + err.Error())
	}
	if g.lobbyTLS != nil { // tests
		set.TLS = g.lobbyTLS
	}
	if err := g.lobbyNotice("Connecting to " + set.Server + " ..."); err != nil {
		return nil, nil, err
	}
	conn, err := lobby.Dial(set)
	if err != nil {
		return nil, nil, g.messageBox(lobbyErrorText(set.Server, err))
	}
	ok := false
	defer func() {
		if !ok {
			conn.Close()
		}
	}()

	rem := lobby.LoadRemembered(g.dir)
	if rem.Session != "" {
		err := conn.Resume(rem.Session)
		switch {
		case err == nil:
			ok = true
			return conn, &rem, nil
		case lobby.SessionGone(err):
			rem.Session = ""
			rem.Save(g.dir)
		default:
			return nil, nil, g.messageBox(lobbyErrorText(set.Server, err))
		}
	}
	for {
		user := &netAddressBox{heading: lobbyHeading, label: "Username"}
		copy(user.value[:len(user.value)-1], rem.User)
		typed, err := g.netAddressScreen(user)
		if err != nil || !typed {
			return nil, nil, err
		}
		pass := &netAddressBox{heading: lobbyHeading, label: "Password", masked: true}
		typed, err = g.netAddressScreen(pass)
		if err != nil {
			return nil, nil, err
		}
		rem.User = save.CString(user.value[:])
		if !typed {
			continue
		}
		if err := g.lobbyNotice("Logging in as " + rem.User + " ..."); err != nil {
			return nil, nil, err
		}
		session, err := conn.Login(rem.User, save.CString(pass.value[:]))
		pass.value = [len(pass.value)]byte{}
		if err != nil {
			if err := g.messageBox(lobbyErrorText(set.Server, err)); err != nil {
				return nil, nil, err
			}
			if errors.Is(err, client.ErrClosed) {
				return nil, nil, nil
			}
			continue
		}
		rem.User, rem.Session = conn.User, session
		if err := rem.Save(g.dir); err != nil {
			g.Log("lobby: %v", err)
		}
		ok = true
		return conn, &rem, nil
	}
}

// playLobbyGame plays j, made or joined in the lobby: the network is the
// lobby's until endLobbyGame, the player's name his account's.
func (g *Game) playLobbyGame(j *lobbyv1.GameJoined) {
	n := &g.net
	if !n.lobbyPlaying {
		n.lobbySaved, n.lobbySavedPlayer, n.lobbyPlaying = n.network, n.player, true
	}
	n.network = lobby.NewNetwork(g.lobbyConn, j)
	n.player = g.lobbyConn.User
	g.settings[settingsMPLink] = 1
	if g.mpSetup != nil {
		g.setMultiPlayerGameType()
	}
}

// endLobbyGame goes back to the network the game was started with, out of
// the lobby's game. It reports whether the lobby's connection is still up
// (kept in g.lobbyConn for the lobby's screen); otherwise it is closed.
func (g *Game) endLobbyGame() bool {
	n := &g.net
	if n.lobbyPlaying {
		n.network, n.player, n.lobbyPlaying = n.lobbySaved, n.lobbySavedPlayer, false
	}
	c := g.lobbyConn
	if c == nil {
		return false
	}
	if c.State.Mine != nil {
		c.LeaveGame()
	}
	if c.Alive() {
		return true
	}
	c.Close()
	g.lobbyConn = nil
	return false
}

// saveLobbyLink records next to save slot the lobby game it was made in
// (SAVEn.LBY), or removes an old record for a save made outside the lobby.
func (g *Game) saveLobbyLink(slot int) {
	id := ""
	if lob, ok := g.net.network.(*lobby.Network); ok {
		id = lob.GameID()
	}
	if err := lobby.SaveLink(g.dir, slot, id); err != nil {
		g.Log("lobby: save link: %v", err)
	}
}

// continueLobbyGame tells the lobby the game made for loading save slot
// continues the lobby game the save was made in, if it was.
func (g *Game) continueLobbyGame(slot int) {
	lob, ok := g.net.network.(*lobby.Network)
	if !ok {
		return
	}
	if prev := lobby.LoadLink(g.dir, slot); prev != "" {
		if err := lob.Continue(prev); err != nil {
			g.Log("lobby: continue %s: %v", prev, err)
		}
	}
}
