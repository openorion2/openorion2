package lobby

import (
	"sort"
	"strings"

	"github.com/openorion2/openorion2/lobbynet/lobbyv1"
)

// ChatKeep is how many chat lines the game keeps.
const ChatKeep = 200

// Line is one line of the lobby's chat; System lines are the lobby's own
// (a game created, a request refused), shown apart and never sent.
type Line struct {
	User, Text string
	System     bool
}

// Note adds a system line to the chat.
func (s *State) Note(text string) { s.add(Line{Text: text, System: true}) }

func (s *State) add(l Line) {
	s.Chat = append(s.Chat, l)
	if len(s.Chat) > ChatKeep {
		s.Chat = s.Chat[len(s.Chat)-ChatKeep:]
	}
	s.Said++
}

// State is the lobby as its pushes describe it.
type State struct {
	Chat   []Line
	Said   int                 // lines added to Chat since the snapshot (it keeps the last ChatKeep)
	Online []string            // sorted, ignoring case
	Games  []*lobbyv1.GameInfo // the listed games, oldest first
	Mine   *lobbyv1.GameInfo   // the game this player is in, if any
	MyID   uint32              // his net id in it
	// Names is who had each net id of Mine while he was in it, kept after
	// they left (port-only: a lost player gets his own empire back).
	Names map[uint32]string

	Me      string    // the player's name, as the server spells it
	Dialogs []*Dialog // his private lines by the other user, oldest dialog first
}

// Dialog is the private lines between the player and one other user.
type Dialog struct {
	With   string // the other's name, as the server spells it
	Lines  []Line
	Said   int // lines added to Lines (it keeps the last ChatKeep)
	Unread int // his lines not read yet

	last, read uint64 // the id of his latest line; marked read up to
}

// Note adds a system line to the dialog.
func (d *Dialog) Note(text string) { d.add(Line{Text: text, System: true}) }

func (d *Dialog) add(l Line) {
	d.Lines = append(d.Lines, l)
	if len(d.Lines) > ChatKeep {
		d.Lines = d.Lines[len(d.Lines)-ChatKeep:]
	}
	d.Said++
}

// Dialog is the dialog with user name (any case), nil if there is none.
func (s *State) Dialog(name string) *Dialog {
	for _, d := range s.Dialogs {
		if strings.EqualFold(d.With, name) {
			return d
		}
	}
	return nil
}

// Open is the dialog with user name, begun if there is none.
func (s *State) Open(name string) *Dialog {
	d := s.Dialog(name)
	if d == nil {
		d = &Dialog{With: name}
		s.Dialogs = append(s.Dialogs, d)
	}
	return d
}

// private folds a private line in; unread: it counts as not read yet.
func (s *State) private(p *lobbyv1.PrivateLine, unread bool) {
	mine := strings.EqualFold(p.From, s.Me)
	with := p.From
	if mine {
		with = p.To
	}
	d := s.Open(with)
	d.With = with
	d.add(Line{User: p.From, Text: p.Text})
	if !mine {
		d.last = max(d.last, p.Id)
		if unread {
			d.Unread++
		} else {
			d.read = max(d.read, p.Id)
		}
	}
}

// apply folds one push in; it reports whether anything changed.
func (s *State) apply(env *lobbyv1.Envelope) bool {
	switch m := env.Msg.(type) {
	case *lobbyv1.Envelope_Snapshot:
		s.Chat, s.Online, s.Games, s.Said, s.Dialogs = nil, nil, nil, 0, nil
		for _, l := range m.Snapshot.Chat {
			s.say(l)
		}
		for _, p := range m.Snapshot.Private {
			s.private(p, p.Unread)
		}
		for _, u := range m.Snapshot.Online {
			s.Online = append(s.Online, u.Name)
		}
		s.sortOnline()
		s.Games = append(s.Games, m.Snapshot.Games...)
	case *lobbyv1.Envelope_Chat:
		s.say(m.Chat)
	case *lobbyv1.Envelope_Private:
		s.private(m.Private, true)
	case *lobbyv1.Envelope_UserOnline:
		s.Online = append(s.Online, m.UserOnline.User.GetName())
		s.sortOnline()
	case *lobbyv1.Envelope_UserOffline:
		for i, n := range s.Online {
			if n == m.UserOffline.User.GetName() {
				s.Online = append(s.Online[:i], s.Online[i+1:]...)
				break
			}
		}
	case *lobbyv1.Envelope_GameUpdate:
		g := m.GameUpdate.Game
		if s.Mine != nil && s.Mine.Id == g.Id {
			s.Mine = g
			s.noteNames(g)
			if g.State == lobbyv1.GameState_GAME_STATE_FINISHED {
				s.Mine, s.MyID = nil, 0
			}
		}
		back := g.State == lobbyv1.GameState_GAME_STATE_STARTED && (s.Mine == nil || s.Mine.Id != g.Id)
		if !back && (!g.Public || g.State != lobbyv1.GameState_GAME_STATE_OPEN) {
			return true // a private game of his: Mine only
		}
		for i, old := range s.Games {
			if old.Id == g.Id {
				s.Games[i] = g
				return true
			}
		}
		s.Games = append(s.Games, g)
		switch {
		case back:
			// port-only: a started game of his takes him back (netresume.go).
			s.Note("Your game waits for you to come back: " + g.Description)
		case s.Mine == nil || s.Mine.Id != g.Id:
			s.Note(g.Creator + " created a game: " + g.Description)
		}
	case *lobbyv1.Envelope_GameRemoved:
		id := m.GameRemoved.Id
		for i, g := range s.Games {
			if g.Id == id {
				s.Games = append(s.Games[:i], s.Games[i+1:]...)
				break
			}
		}
		// Off the list only: his own game goes on (started) until FINISHED.
	default:
		return false
	}
	return true
}

func (s *State) say(l *lobbyv1.ChatLine) {
	s.add(Line{User: l.User, Text: l.Text})
}

func (s *State) sortOnline() {
	sort.Slice(s.Online, func(i, j int) bool { return strings.ToLower(s.Online[i]) < strings.ToLower(s.Online[j]) })
}

func (s *State) joined(j *lobbyv1.GameJoined) {
	s.Mine, s.MyID, s.Names = j.Game, j.NetId, nil
	s.noteNames(j.Game)
}

// noteNames adds g's members to Names.
func (s *State) noteNames(g *lobbyv1.GameInfo) {
	if s.Names == nil {
		s.Names = map[uint32]string{}
	}
	for _, p := range g.Players {
		s.Names[p.NetId] = p.Name
	}
}
