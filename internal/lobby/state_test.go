package lobby

import (
	"testing"

	"github.com/openorion2/openorion2/lobbynet/lobbyv1"
)

// TestStatePrivate: the snapshot's private lines make a dialog per other
// user (named as the server spells him), counting the unread ones; a
// pushed line from him is unread, the player's own is not.
func TestStatePrivate(t *testing.T) {
	s := &State{Me: "Alice"}
	s.Open("BOB") // a tab opened before he answered
	s.apply(&lobbyv1.Envelope{Msg: &lobbyv1.Envelope_Snapshot{Snapshot: &lobbyv1.LobbySnapshot{Private: []*lobbyv1.PrivateLine{
		{Id: 1, From: "carol", To: "Alice", Text: "old"},
		{Id: 2, From: "Alice", To: "bob", Text: "hi"},
		{Id: 3, From: "carol", To: "Alice", Text: "new", Unread: true},
	}}}})
	if len(s.Dialogs) != 2 || s.Dialogs[0].With != "carol" || s.Dialogs[0].Unread != 1 || len(s.Dialogs[0].Lines) != 2 {
		t.Fatalf("dialogs %+v", s.Dialogs)
	}
	s.Open("BOB")
	for _, p := range []*lobbyv1.PrivateLine{{Id: 4, From: "bob", To: "Alice", Text: "yo"}, {Id: 5, From: "Alice", To: "bob", Text: "sup"}} {
		s.apply(&lobbyv1.Envelope{Msg: &lobbyv1.Envelope_Private{Private: p}})
	}
	d := s.Dialog("Bob")
	if d == nil || d.With != "bob" || d.Unread != 1 || d.Said != 3 || d.last != 4 || d.Lines[2].User != "Alice" {
		t.Fatalf("bob's dialog %+v", d)
	}
}

// TestStateWaitingGame: a started game the server lists (open again for a
// lost player) shows in a former player's list with a note, private or
// not; a member's own started game does not; its removal takes it off.
// Names keeps who had each net id of his game after they left.
func TestStateWaitingGame(t *testing.T) {
	s := &State{Me: "bob"}
	g := &lobbyv1.GameInfo{Id: "g1", Description: "long one", State: lobbyv1.GameState_GAME_STATE_STARTED}
	s.apply(&lobbyv1.Envelope{Msg: &lobbyv1.Envelope_GameUpdate{GameUpdate: &lobbyv1.GameUpdate{Game: g}}})
	if len(s.Games) != 1 || s.Games[0].Id != "g1" {
		t.Fatalf("games %v", s.Games)
	}
	if n := len(s.Chat); n == 0 || s.Chat[n-1].Text != "Your game waits for you to come back: long one" {
		t.Fatalf("chat %+v", s.Chat)
	}
	s.apply(&lobbyv1.Envelope{Msg: &lobbyv1.Envelope_GameRemoved{GameRemoved: &lobbyv1.GameRemoved{Id: "g1"}}})
	if len(s.Games) != 0 {
		t.Fatalf("games after the removal %v", s.Games)
	}

	m := &State{Me: "alice"}
	m.joined(&lobbyv1.GameJoined{Game: &lobbyv1.GameInfo{Id: "g2", Players: []*lobbyv1.Player{{Name: "alice", NetId: 1}}}, NetId: 1})
	mine := &lobbyv1.GameInfo{Id: "g2", State: lobbyv1.GameState_GAME_STATE_STARTED,
		Players: []*lobbyv1.Player{{Name: "alice", NetId: 1}, {Name: "bob", NetId: 2}}}
	m.apply(&lobbyv1.Envelope{Msg: &lobbyv1.Envelope_GameUpdate{GameUpdate: &lobbyv1.GameUpdate{Game: mine}}})
	if len(m.Games) != 0 {
		t.Fatalf("his own started game listed: %v", m.Games)
	}
	left := &lobbyv1.GameInfo{Id: "g2", State: lobbyv1.GameState_GAME_STATE_STARTED, Players: []*lobbyv1.Player{{Name: "alice", NetId: 1}}}
	m.apply(&lobbyv1.Envelope{Msg: &lobbyv1.Envelope_GameUpdate{GameUpdate: &lobbyv1.GameUpdate{Game: left}}})
	if m.Names[2] != "bob" || m.Names[1] != "alice" {
		t.Fatalf("names %v", m.Names)
	}
}
