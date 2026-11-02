package game

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/openorion2/openorion2/internal/announce"
	"github.com/openorion2/openorion2/internal/lobby"
)

// TestAnnouncements: a new announcement is noted once in the lobby's chat
// and shown once in a game through the lobby; a changed text is new again.
func TestAnnouncements(t *testing.T) {
	t.Parallel()
	text := "The lobby restarts at 16:00 UTC. Рестарт."
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"messages": [{"id": "r", "level": "warning", "text": %q}]}`, text)
	}))
	defer srv.Close()
	feed := &announce.Feed{URL: srv.URL}
	if err := feed.Fetch(context.Background()); err != nil {
		t.Fatal(err)
	}
	g := &Game{announceFeed: feed}
	var st lobby.State
	g.announceToLobby(&st)
	g.announceToLobby(&st)
	if len(st.Chat) != 1 || !strings.Contains(st.Chat[0].Text, text) || !st.Chat[0].System {
		t.Fatalf("the lobby's chat: %+v", st.Chat)
	}
	g.announceInGame() // not a lobby game: nothing
	if len(g.net.notices) != 0 {
		t.Fatalf("a game outside the lobby: %v", g.net.notices)
	}
	g.net.network = lobby.NewNetwork(nil, nil)
	g.announceInGame()
	g.announceInGame()
	// The game's own fonts print ASCII only; the lobby's, all of it.
	if len(g.net.notices) != 1 || !strings.Contains(g.net.notices[0], "16:00 UTC.") || strings.Contains(g.net.notices[0], "Рестарт") {
		t.Fatalf("the game's notices: %v", g.net.notices)
	}
	text = "The lobby restarts within a minute."
	if err := feed.Fetch(context.Background()); err != nil {
		t.Fatal(err)
	}
	g.announceInGame()
	g.announceToLobby(&st)
	if len(g.net.notices) != 2 || len(st.Chat) != 2 {
		t.Fatalf("a changed text: %v, %d lines", g.net.notices, len(st.Chat))
	}
}
