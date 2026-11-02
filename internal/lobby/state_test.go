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
