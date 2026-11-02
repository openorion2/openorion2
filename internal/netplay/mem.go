package netplay

import (
	"slices"
	"sync"
)

// Mem is the in-memory transport: every station of every game lives in this
// process, as the machines of a test do. A send lands in the receiver's
// queue at once, so under host.Lockstep (one machine runs at a time) a
// message sent in frame f by machine i is drained by machine j > i in frame
// f and by j < i in frame f+1, in every run.
type Mem struct {
	mu    sync.Mutex
	games map[string]*hub
	order []string

	// Assign (tests) are the net ids a new game hands out first, in join
	// order, as an oracle run's IPX ids were; then the usual 1, 2, ...
	Assign []uint16
	// Order (tests) is the order Users lists these ids in, as an oracle
	// run's IPX listed its stations (not by id); others follow by id.
	Order []uint16
	// Hosted (tests): the relay is not the creator's machine, as with the
	// lobby or a hosted relay: Drop of the creator loses him alone.
	Hosted bool
}

// NewMem makes an empty in-memory network.
func NewMem() *Mem { return &Mem{games: map[string]*hub{}} }

// Create starts a game and joins it as its creator (net id 1).
func (m *Mem) Create(game, player string) (Endpoint, error) {
	m.mu.Lock()
	if h, ok := m.games[game]; ok && len(h.info().Users) > 0 {
		m.mu.Unlock()
		return nil, ErrRefused
	}
	h := newHub(game)
	h.assign = slices.Clone(m.Assign)
	h.order = slices.Clone(m.Order)
	if _, ok := m.games[game]; !ok {
		m.order = append(m.order, game)
	}
	m.games[game] = h
	m.mu.Unlock()
	return join(h, true)
}

// Join joins a game by name.
func (m *Mem) Join(game, player string) (Endpoint, error) {
	m.mu.Lock()
	h, ok := m.games[game]
	m.mu.Unlock()
	if !ok {
		return nil, ErrRefused
	}
	l, err := join(h, false)
	if err != nil {
		return nil, err
	}
	return l, nil
}

// Games lists the games that still have members, in creation order.
func (m *Mem) Games() ([]GameInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []GameInfo
	for _, name := range m.order {
		if gi := m.games[name].info(); len(gi.Users) > 0 {
			out = append(out, gi)
		}
	}
	return out, nil
}

// Drop cuts station id of game off as a lost link would (tests): the
// station's own Err becomes ErrLost and the others see it leave. Dropping
// the creator ends the game for everyone, as the relay leaving does.
func (m *Mem) Drop(game string, id uint16) {
	m.mu.Lock()
	h := m.games[game]
	m.mu.Unlock()
	if h == nil {
		return
	}
	h.mu.Lock()
	mb, ok := h.members[id]
	h.mu.Unlock()
	if !ok {
		return
	}
	if l, ok := mb.(*local); ok && l.creator && !m.Hosted {
		h.close()
		return
	}
	mb.drop(ErrLost)
	h.remove(id)
}

// Members lists the net ids in game (tests).
func (m *Mem) Members(game string) []uint16 {
	m.mu.Lock()
	h := m.games[game]
	m.mu.Unlock()
	if h == nil {
		return nil
	}
	return slices.Clone(h.info().Users)
}
