package game

import (
	"time"

	"github.com/openorion2/openorion2/internal/crash"
	"github.com/openorion2/openorion2/internal/lobby"
	"github.com/openorion2/openorion2/internal/update"
	"github.com/openorion2/openorion2/lobbynet/lobbyv1"
)

// port-only: crash reports (internal/crash, docs/lobby/client.md, "Crash
// reports"). A panic leaves the log's tail and the state next to the
// runtime's output; at the next start the main menu offers, once, to send
// that report to the lobby. No drops it; a failed send keeps it for the
// next start.

type crashOffer struct {
	offered bool
	send    func(*lobbyv1.CrashReport) error // the lobby; tests replace it
}

// crashWrite saves the log's tail and, in a game, the state as a save, for
// a panic about to end the program.
func (g *Game) crashWrite() {
	if g.crash == nil {
		return
	}
	var save []byte
	if g.state.NumStars > 0 {
		func() {
			defer func() { recover() }() // the state may be what broke
			save = g.encodeSave(9)
		}()
	}
	g.logTail.Write(g.dir, save)
}

// sendCrashReport sends r to the lobby with the player's saved session.
func (g *Game) sendCrashReport(r *lobbyv1.CrashReport) error {
	return lobby.SendCrash(g.dir, g.lobbyTLS, r)
}

// offerCrashReport asks whether to send a pending report and sends it;
// shown reports a box was shown (the menu needs a redraw).
func (g *Game) offerCrashReport() (shown bool, err error) {
	c := g.crash
	if c == nil || c.offered || !crash.Pending(g.dir) {
		return false, nil
	}
	c.offered = true
	yes, err := g.confirmationBox("OpenOrion2 crashed last time.\nSend the crash report to the online lobby? It holds the version, your system, where it crashed, the game's last log lines and a save of the game.")
	if err != nil {
		return true, err
	}
	if !yes {
		crash.Drop(g.dir)
		return true, nil
	}
	r, err := crash.Report(g.dir, update.Version)
	if err != nil {
		crash.Drop(g.dir)
		return true, g.messageBox("The crash report cannot be read: " + err.Error())
	}
	result := make(chan error, 1)
	go func() { result <- c.send(r) }()
	for {
		select {
		case err := <-result:
			if err != nil {
				return true, g.messageBox("The crash report was not sent (" + err.Error() + "). It is kept for the next start.")
			}
			crash.Drop(g.dir)
			return true, g.messageBox("Thank you: the crash report went to the lobby.")
		default:
		}
		if err := g.updateNotice("Sending the crash report..."); err != nil {
			return true, err
		}
		if err := g.e.Sys.Sleep(time.Second / 30); err != nil {
			return true, err
		}
	}
}
