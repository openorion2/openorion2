package game

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/openorion2/openorion2/internal/engine"
	"github.com/openorion2/openorion2/internal/update"
)

// port-only: a newer release of OpenOrion2, looked up at the start in the
// background (Options.Update) and offered on the main menu once: Yes
// downloads it, with its progress shown, installs it in place of the
// binary and ends the game with update.ErrRestart, for main to start the
// new one.

// updateCheck is the background lookup's state.
type updateCheck struct {
	client  *update.Client
	found   chan *update.Release // the lookup's answer (nil: nothing newer)
	offered bool
}

// startUpdateCheck looks for a newer release in the background.
func (g *Game) startUpdateCheck(c *update.Client) {
	if c == nil || !c.Enabled() {
		return
	}
	u := &updateCheck{client: c, found: make(chan *update.Release, 1)}
	g.update = u
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		r, err := c.Latest(ctx)
		if err != nil {
			g.Log("update: %v", err)
		}
		u.found <- r
	}()
}

// offerUpdate offers a newer release if the lookup found one and it was
// not offered yet; shown reports a box was shown (the screen needs a
// redraw). It returns update.ErrRestart once the new binary is in place.
func (g *Game) offerUpdate() (shown bool, err error) {
	u := g.update
	if u == nil || u.offered {
		return false, nil
	}
	var r *update.Release
	select {
	case r = <-u.found:
	default:
		return false, nil
	}
	u.offered = true
	if r == nil {
		return false, nil
	}
	current := u.client.Current
	if current == "" {
		current = update.Version
	}
	yes, err := g.confirmationBox(fmt.Sprintf("OpenOrion2 %s is out (this is %s).\nDownload and install it now?", r.Version, current))
	if err != nil || !yes {
		return true, err
	}
	var done, total atomic.Int64
	total.Store(-1)
	result := make(chan error, 1)
	go func() {
		result <- u.client.Install(context.Background(), r, func(d, n int64) { done.Store(d); total.Store(n) })
	}()
	for {
		select {
		case err := <-result:
			if err != nil {
				return true, g.messageBox("The update failed: " + err.Error())
			}
			if err := g.messageBox("OpenOrion2 " + r.Version + " is installed. The game starts again."); err != nil {
				return true, err
			}
			return true, update.ErrRestart
		default:
		}
		line := "Downloading OpenOrion2 " + r.Version + "..."
		if n := total.Load(); n > 0 {
			line = fmt.Sprintf("Downloading OpenOrion2 %s... %d%%", r.Version, done.Load()*100/n)
		}
		if err := g.updateNotice(line); err != nil {
			return true, err
		}
		if err := g.e.Sys.Sleep(time.Second / 30); err != nil {
			return true, err
		}
	}
}

// updateNotice shows line in a box over the main menu's background.
func (g *Game) updateNotice(line string) error {
	e := g.e
	e.ClearFields()
	e.V.SetPageOff()
	e.V.Fill(0, 0, engine.W-1, engine.H-1, 0)
	if bg := g.mainMenu.background; bg != nil {
		if err := e.Draw(0, 0, bg); err != nil {
			return err
		}
	}
	w := uniWidthBold(line) + 32
	x1, y1 := (engine.W-w)/2, 0xd8
	x2, y2 := x1+w-1, y1+uniLineH+15
	ink := byte(nearestColor(&e.V.Palette, [3]uint8{60, 60, 60}))
	rim := byte(nearestColor(&e.V.Palette, [3]uint8{36, 36, 36}))
	e.V.Fill(x1, y1, x2, y2, byte(nearestColor(&e.V.Palette, [3]uint8{4, 4, 6})))
	e.V.Line(x1, y1, x2, y1, rim)
	e.V.Line(x1, y2, x2, y2, rim)
	e.V.Line(x1, y1, x1, y2, rim)
	e.V.Line(x2, y1, x2, y2, rim)
	g.uniPrintBold(x1+16, y1+8, x2-8, line, ink)
	return e.TogglePages()
}
