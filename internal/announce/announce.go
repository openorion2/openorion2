// Package announce fetches the project's announcements (docs/announce.md):
// a JSON file at announce.openorion2.org with short messages for every
// player (a new version, a lobby restart), shown on the main menu, in the
// lobby and in games played through it. Port-only.
package announce

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// URL is where the announcements are.
const URL = "https://announce.openorion2.org/announce.json"

// Every is how often a Feed fetches them: a restart is announced a minute
// ahead.
const Every = time.Minute

// Message is one announcement.
type Message struct {
	ID    string     `json:"id"`
	Text  string     `json:"text"`
	Level string     `json:"level"` // "info" or "warning"
	From  *time.Time `json:"from,omitempty"`
	Until *time.Time `json:"until,omitempty"`
}

// Warning reports whether m is to stand out.
func (m Message) Warning() bool { return m.Level == "warning" }

// Key is what tells a message from another: a message changed under the
// same id is new.
func (m Message) Key() string { return m.ID + "\x00" + m.Text }

type file struct {
	Messages []Message `json:"messages"`
}

// Feed keeps the latest announcements; the zero value fetches URL.
type Feed struct {
	URL    string
	Client *http.Client
	// Now is the clock (tests); time.Now when nil.
	Now func() time.Time

	mu   sync.Mutex
	msgs []Message
}

// Start fetches now and then Every in the background, until ctx ends.
func (f *Feed) Start(ctx context.Context) {
	go func() {
		t := time.NewTicker(Every)
		defer t.Stop()
		for {
			f.Fetch(ctx)
			select {
			case <-ctx.Done():
				return
			case <-t.C:
			}
		}
	}()
}

// Fetch gets the announcements once; on an error the last ones stay.
func (f *Feed) Fetch(ctx context.Context) error {
	url, c := f.URL, f.Client
	if url == "" {
		url = URL
	}
	if c == nil {
		c = &http.Client{Timeout: 15 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("announce: %s", resp.Status)
	}
	var v file
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&v); err != nil {
		return fmt.Errorf("announce: %v", err)
	}
	f.mu.Lock()
	f.msgs = v.Messages
	f.mu.Unlock()
	return nil
}

// Active is the announcements to show now: those whose From has come and
// Until not passed, in the file's order.
func (f *Feed) Active() []Message {
	if f == nil {
		return nil
	}
	now := time.Now()
	if f.Now != nil {
		now = f.Now()
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []Message
	for _, m := range f.msgs {
		if m.Text == "" || m.From != nil && now.Before(*m.From) || m.Until != nil && !now.Before(*m.Until) {
			continue
		}
		out = append(out, m)
	}
	return out
}
