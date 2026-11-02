package announce

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFeed(t *testing.T) {
	t.Parallel()
	body := `{"messages": [
		{"id": "a", "text": "now"},
		{"id": "b", "text": "later", "from": "2026-11-05T12:00:00Z"},
		{"id": "c", "text": "gone", "until": "2026-11-01T00:00:00Z"},
		{"id": "d", "text": "", "level": "warning"},
		{"id": "e", "text": "restart", "level": "warning", "until": "2026-11-05T16:05:00Z"}]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }))
	defer srv.Close()
	now := time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)
	f := &Feed{URL: srv.URL, Now: func() time.Time { return now }}
	if f.Active() != nil {
		t.Fatal("before a fetch")
	}
	if err := f.Fetch(context.Background()); err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, m := range f.Active() {
		got = append(got, m.ID)
	}
	if fmt.Sprint(got) != "[a e]" || !f.Active()[1].Warning() {
		t.Fatalf("active %v", got)
	}
	now = time.Date(2026, 11, 5, 13, 0, 0, 0, time.UTC)
	got = nil
	for _, m := range f.Active() {
		got = append(got, m.ID)
	}
	if fmt.Sprint(got) != "[a b e]" {
		t.Fatalf("later %v", got)
	}
	// A failed fetch keeps the last ones.
	srv.Close()
	if f.Fetch(context.Background()) == nil || len(f.Active()) != 3 {
		t.Fatal("after a failed fetch")
	}
	var nilFeed *Feed
	if nilFeed.Active() != nil {
		t.Fatal("a nil feed")
	}
}
