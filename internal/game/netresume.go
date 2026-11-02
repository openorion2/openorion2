package game

import (
	"encoding/binary"
	"fmt"
	"slices"
	"strings"

	"github.com/openorion2/openorion2/internal/netplay"
	"github.com/openorion2/openorion2/internal/save"
)

// port-only: a lost player's return (docs/net/hardening.md, "A lost player
// comes back"). Every machine goes back to the start of the turn; the
// lowest station left (the resumer) loads its snapshot of that start and
// waits on the creator's waiting screen of a loaded game, the others on the
// same screen for its broadcast. The lost player joins again as a joiner
// of a loaded game, picks the empire, and the turn is played again. START
// (with a confirmation) goes on without the player: the computer plays it.

const (
	netCtlResume = 0xfe04 // {u32 stardate}: the sender resumes the game from that start
	netCtlGoOn   = 0xfe05 // to the resumer: START was pressed, go on without the lost
)

// netResume is the panic that unwinds to Screen_Control_ (screenControl)
// when a player is lost: netResumeScreen runs next.
type netResume struct{}

type netResumeState struct {
	start    []byte   // the state at the start of the turn, as a save
	active   bool     // netResumeScreen runs
	from     uint16   // the resumer, once its netCtlResume came
	goOn     bool     // the resumer: someone pressed START
	lost     []string // the lost players' names, for the notice
	resumed  int      // tests: completed resumes
	off      bool     // tests: a lost player becomes an AI at once, as before
	checked  int      // tests: desync checks passed against a returner
	waitHook func()   // tests: the waiting screen runs once more
}

// netResumable runs f; a lost player's netResume panic reports again.
func (g *Game) netResumable(f func() error) (again bool, err error) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(netResume); !ok {
				panic(r)
			}
			again, err = true, nil
		}
	}()
	return false, f()
}

// netKeepTurnStart keeps the state at the start of a turn of a network
// game: where a lost player's return goes back to.
func (g *Game) netKeepTurnStart() {
	if g.gameType == 2 {
		g.net.resume.start = g.encodeSave(9)
	}
}

// netCanResume: a lost player may come back (else he becomes an AI at once).
func (g *Game) netCanResume() bool {
	n := &g.net
	return g.gameType == 2 && n.inGame && n.resume.start != nil && n.ep != nil && !n.resume.off
}

// netLose starts the return of lost player p: every screen unwinds.
func (g *Game) netLose(p int) {
	r := &g.net.resume
	if r.active {
		return // the resume loops see the members themselves
	}
	r.lost = append(r.lost, g.netWho(p))
	g.Log("network game: %s lost at stardate %d, back to the turn's start", g.netWho(p), g.state.Stardate)
	panic(netResume{})
}

// netResumer: this machine resumes the game (the lowest station that is
// still a human player, as for the drop-outs).
func (g *Game) netResumer() bool { return g.netInjector() }

// netResumeTakes reports whether a game message is taken while the game
// resumes: on the resumer, the joiners' answers; elsewhere, the resumer's
// messages once its netCtlResume came (what was under way is dropped).
func (g *Game) netResumeTakes(from, id uint16) bool {
	n := &g.net
	if g.netResumer() {
		return id == 0x21 || id == 0x42 || id == 0x27
	}
	return n.resume.from != 0 && from == n.resume.from
}

// netResumeFrame takes the port-only frames of a return.
func (g *Game) netResumeFrame(f netplay.Frame) {
	n := &g.net
	switch f.ID {
	case netCtlResume:
		if !n.resume.active {
			// The resumer saw the loss first: unwind here too.
			n.resume.lost = g.netLostNames()
			n.resume.from = f.From
			panic(netResume{})
		}
		if !g.netResumer() {
			n.resume.from = f.From
			n.recv = nil
			n.set16(netChunks, 0)
			n.set16(netRecvChunks, 0)
		}
	case netCtlGoOn:
		if g.netResumer() {
			n.resume.goOn = true
		}
	}
}

// netLostNames names the players whose stations left.
func (g *Game) netLostNames() []string {
	n, s := &g.net, &g.state
	var out []string
	users := n.ep.Users()
	for k := 0; k < int(s.NumPlayers); k++ {
		pl := &s.Players[k]
		if pl.NetID != -1 && pl.Objective == save.ObjectiveHuman && !slices.Contains(users, uint16(pl.NetID)) {
			out = append(out, g.netWho(k))
		}
	}
	return out
}

// netResumeScreen goes back to the start of the turn and waits for the lost
// players (screen control runs it after a netResume panic).
func (g *Game) netResumeScreen() error {
	n, e := &g.net, g.e
	n.resume.active = true
	n.chat = nil
	n.notices = nil
	e.ClearFields()
	e.DeactivateAutoFunction()
	g.Log("network game: resuming (%s)", strings.Join(n.resume.lost, ", "))
	var err error
	if g.netResumer() {
		err = g.netResumeHost()
	} else {
		err = g.netResumeWait()
	}
	if err != nil {
		return err
	}
	r := &n.resume
	*r = netResumeState{start: r.start, resumed: r.resumed + 1, off: r.off, checked: r.checked, waitHook: r.waitHook}
	g.currentScreen = ScreenReports
	g.previousScreen = ScreenJoinNet // as after Load_Net_Screen_
	return nil
}

// netResumeText is what the waiting screen says while the game resumes.
func (g *Game) netResumeText() string {
	who := strings.Join(g.net.resume.lost, ", ")
	if who == "" {
		who = "A player"
	}
	return fmt.Sprintf("%s has lost the connection. The game goes back to the start of the turn and waits for the player to join again: please stay. START NET GAME goes on without them.", who)
}

// drawResumeText prints netResumeText under the waiting screen's box.
func (g *Game) drawResumeText() error {
	if !g.net.resume.active {
		return nil
	}
	g.netTextColors()
	return g.e.Text().PrintParagraph(0x28, 0x170, 0x230, g.netResumeText(), 0)
}

// netResumeHost is the resumer's side: Load_Net_Screen_ from its waiting
// screen on, with the snapshot loaded and the stations still there placed.
func (g *Game) netResumeHost() error {
	e, n, s := g.e, &g.net, &g.state
	f, err := save.Parse(n.resume.start)
	if err != nil {
		return err
	}
	me := s.PlayerNum
	if err := g.loadSaveFile(f, 9); err != nil {
		return err
	}
	s.PlayerNum = me
	users := n.ep.Users()
	for k := 0; k < int(s.NumPlayers); k++ {
		if id := s.Players[k].NetID; id != -1 && !slices.Contains(users, uint16(id)) {
			s.Players[k].NetID = -1
		}
	}
	n.ep.Send(netplay.Broadcast, netCtlResume, binary.LittleEndian.AppendUint32(nil, s.Stardate))
	if err := n.ep.SetOpen(true); err != nil {
		g.Log("network game: the game cannot take the lost player back: %v", err)
	}
	n.set8(netState, 0x24)
	g.netInitHostElection()
	n.set16(netOwnID, int16(n.ep.ID()))
	n.set16(netGameHost, n.i16(netOwnID))
	n.set32(netPicking, -1)
	n.set8(netNebulasOK, 1) // as Load_Net_Screen_: a joiner's msg 0x42 may say otherwise
	e.AssignAutoFunction(g.drawWaitForNewJoinersScreen, 2)
	if err := g.reloadWaitingForJoinersScreen(); err != nil {
		return err
	}
	e.ClearFields()
	if err := g.addWaitingForJoinersField(); err != nil {
		return err
	}
	humans, placed := 0, 0
	for k := 0; k < int(s.NumPlayers); k++ {
		if s.Players[k].Objective == save.ObjectiveHuman {
			humans++
			if s.Players[k].NetID != -1 {
				placed++
			}
		}
	}
	n.set8(netHumans, uint8(humans))
	for done := false; !done; {
		if !g.netResumer() {
			panic(netResume{}) // a lower station is back in charge
		}
		// A station placed may be lost too: its empire waits as well.
		users := n.ep.Users()
		placed = 0
		for k := 0; k < int(s.NumPlayers); k++ {
			pl := &s.Players[k]
			if pl.NetID != -1 && !slices.Contains(users, uint16(pl.NetID)) {
				pl.NetID = -1
			}
			if pl.Objective == save.ObjectiveHuman && pl.NetID != -1 {
				placed++
			}
		}
		if n.i32(netPicking) != -1 && !slices.Contains(users, uint16(n.i32(netPicking))) {
			n.set32(netPicking, -1) // the joiner asked left before his pick
		}
		if n.i32(netPicking) == -1 {
			for i := len(users) - 1; i >= 0; i-- {
				if g.netIDToPlayerID(int32(users[i])) == -1 {
					n.set32(netPicking, int32(users[i]))
					g.sendPickYourPosition(int32(users[i]))
					break
				}
			}
		}
		if n.resume.waitHook != nil {
			n.resume.waitHook()
		}
		if err := g.drawWaitForNewJoinersScreen(); err != nil {
			return err
		}
		if err := e.TogglePages(); err != nil {
			return err
		}
		if n.i32(netPicking) == -1 {
			switch {
			case placed == humans:
				done = true
			case n.resume.goOn:
				done = true
			default:
				in, err := e.GetInput()
				if err != nil {
					return err
				}
				if in == g.netScr.startF {
					if done, err = g.netResumeGoOnBox(); err != nil {
						return err
					}
				}
			}
		}
		e.MoxUpdate()
	}
	g.netDisableNewPlayers()
	return g.netStartLoadedGame()
}

// netResumeGoOnBox asks before going on without the lost players.
func (g *Game) netResumeGoOnBox() (bool, error) {
	who := strings.Join(g.netLostNamesNow(), ", ")
	if who == "" {
		who = "the lost players"
	}
	return g.confirmationBox(fmt.Sprintf("Go on without %s? The computer will play the empire.", who))
}

// netLostNamesNow names the human empires nobody plays yet (the resumer's
// state) or, elsewhere, those lost.
func (g *Game) netLostNamesNow() []string {
	s := &g.state
	if !g.netResumer() {
		return g.net.resume.lost
	}
	var out []string
	for k := 0; k < int(s.NumPlayers); k++ {
		if s.Players[k].Objective == save.ObjectiveHuman && s.Players[k].NetID == -1 {
			out = append(out, g.netWho(k))
		}
	}
	return out
}

// netResumeWait is the other stations' side: the same waiting screen (its
// START asks the resumer to go on), then the resumer's msg 0x2a and full
// broadcast as a joiner of a loaded game takes them.
func (g *Game) netResumeWait() error {
	e, n := g.e, &g.net
	if n.resume.from != 0 {
		n.recv = nil
		n.set16(netChunks, 0)
		n.set16(netRecvChunks, 0)
	}
	n.set8(netState, 7)
	e.AssignAutoFunction(g.drawWaitForNewJoinersScreen, 2)
	if err := g.reloadWaitingForJoinersScreen(); err != nil {
		return err
	}
	e.ClearFields()
	if err := g.addWaitingForJoinersField(); err != nil {
		return err
	}
	for n.resume.from == 0 || n.i16(netChunks) == 0 {
		if g.netResumer() {
			panic(netResume{}) // the resumer was lost: this one takes over
		}
		if n.resume.waitHook != nil {
			n.resume.waitHook()
		}
		if err := g.drawWaitForNewJoinersScreen(); err != nil {
			return err
		}
		if err := e.TogglePages(); err != nil {
			return err
		}
		in, err := e.GetInput()
		if err != nil {
			return err
		}
		if in == g.netScr.startF && n.resume.from != 0 {
			ok, err := g.netResumeGoOnBox()
			if err != nil {
				return err
			}
			if ok {
				n.ep.Send(n.resume.from, netCtlGoOn, nil)
			}
		}
		e.MoxUpdate()
	}
	n.set16(netGameHost, int16(n.resume.from))
	return g.netTakeLoadedGame()
}
