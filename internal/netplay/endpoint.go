package netplay

import (
	"errors"
	"slices"
	"sync"
)

// Errors of an endpoint.
var (
	ErrLost    = errors.New("netplay: connection lost")
	ErrClosed  = errors.New("netplay: closed")
	ErrNoUser  = errors.New("netplay: no such user")
	ErrRefused = errors.New("netplay: refused")
)

// GameInfo is what the join list shows of a game (Net_Get_Game_Name_,
// Net_Get_Game_Status_).
type GameInfo struct {
	Name  string
	Open  bool     // new players may join (Net_Enable/Disable_New_Players_)
	Users []uint16 // the members' net ids
}

// Endpoint is one station of a game: what the game's Net_* layer uses.
// Send may be called from the game's goroutine only; everything else is
// safe from any goroutine. Receiving never blocks: frames are queued as
// they arrive and the game drains them in Net_Update_.
type Endpoint interface {
	ID() uint16        // Net_Host_ID_: this station's net id
	Game() GameInfo    // the game joined and its members, own id included
	Users() []uint16   // Net_Get_Users_: the members in id order
	Dropped() []uint16 // members that left since this station joined
	// Send queues a message to net id to (or Broadcast); to must be a
	// member. It fails once the station's own link is gone.
	Send(to, id uint16, payload []byte) error
	Recv() (Frame, bool)     // the next queued frame, if any
	SetOpen(open bool) error // the creator only: accept joiners or not
	Err() error              // nil, ErrLost or ErrClosed
	Close() error
}

// Network makes endpoints: what Net_Create_Game_, Net_Join_Game_ and
// Net_Update_Game_List_ reach.
type Network interface {
	Create(game, player string) (Endpoint, error)
	Join(game, player string) (Endpoint, error)
	Games() ([]GameInfo, error)
}

// inbox is a station's side of the session: the queue of received frames
// and what it knows of the members.
type inbox struct {
	mu      sync.Mutex
	id      uint16
	game    string
	open    bool
	q       []Frame
	users   []uint16
	gen     uint32 // the hub's membership generation of users
	dropped []uint16
	err     error
}

func (b *inbox) push(f Frame) {
	b.mu.Lock()
	if b.err == nil {
		b.q = append(b.q, f)
	}
	b.mu.Unlock()
}

func (b *inbox) pop() (Frame, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.q) == 0 {
		return Frame{}, false
	}
	f := b.q[0]
	b.q = b.q[1:]
	return f, true
}

// setUsers applies a membership change unless a newer one came first
// (changes are sent from several goroutines).
func (b *inbox) setUsers(ids []uint16, open bool, gen uint32) {
	b.mu.Lock()
	if gen >= b.gen {
		b.users, b.open, b.gen = slices.Clone(ids), open, gen
	}
	b.mu.Unlock()
}

func (b *inbox) lost(id uint16) {
	b.mu.Lock()
	b.dropped = append(b.dropped, id)
	b.mu.Unlock()
}

func (b *inbox) fail(err error) {
	b.mu.Lock()
	if b.err == nil {
		b.err = err
	}
	b.mu.Unlock()
}

func (b *inbox) state() (error, []uint16) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.err, b.users
}

func (b *inbox) ID() uint16 { return b.id }

func (b *inbox) Game() GameInfo {
	b.mu.Lock()
	defer b.mu.Unlock()
	return GameInfo{Name: b.game, Open: b.open, Users: slices.Clone(b.users)}
}

func (b *inbox) Users() []uint16 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return slices.Clone(b.users)
}

func (b *inbox) Dropped() []uint16 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return slices.Clone(b.dropped)
}

func (b *inbox) Recv() (Frame, bool) { return b.pop() }

func (b *inbox) Err() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.err
}

// checkSend is the station's own test before a send: its link is up and
// the destination is a member (or Broadcast).
func (b *inbox) checkSend(to uint16) error {
	err, users := b.state()
	if err != nil {
		return err
	}
	if to != Broadcast && !slices.Contains(users, to) {
		return ErrNoUser
	}
	return nil
}

// member is a station as the hub sees it: a local inbox or a TCP link.
type member interface {
	deliver(f Frame) error
	members(ids []uint16, open bool, gen uint32)
	left(id uint16)
	drop(err error) // the hub closes: the station's link is gone
}

// hub routes frames between the members of one game: the relay's core,
// shared by the TCP relay and the in-memory transport.
type hub struct {
	mu      sync.Mutex
	game    string
	open    bool
	next    uint16
	assign  []uint16 // tests: the net ids to hand out first, in join order
	order   []uint16 // tests: the order ids lists these in (Mem.Order)
	members map[uint16]member
	gen     uint32 // bumped on every membership or open change
	closed  bool
}

func newHub(game string) *hub {
	return &hub{game: game, open: true, next: 1, members: map[uint16]member{}}
}

func (h *hub) ids() []uint16 {
	ids := make([]uint16, 0, len(h.members))
	for id := range h.members {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	if len(h.order) > 0 {
		rank := func(id uint16) int {
			if i := slices.Index(h.order, id); i >= 0 {
				return i
			}
			return len(h.order) + int(id)
		}
		slices.SortStableFunc(ids, func(a, b uint16) int { return rank(a) - rank(b) })
	}
	return ids
}

// add makes m a member with the next net id; refused when closed or full.
func (h *hub) add(m member) (uint16, []uint16, error) {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return 0, nil, ErrClosed
	}
	if !h.open {
		h.mu.Unlock()
		return 0, nil, errors.New("game closed to new players")
	}
	if len(h.members) >= MaxUsers {
		h.mu.Unlock()
		return 0, nil, errors.New("game full")
	}
	id := h.next
	if len(h.assign) > 0 {
		id, h.assign = h.assign[0], h.assign[1:]
	} else {
		h.next++
	}
	h.members[id] = m
	h.gen++
	ids, open, gen, all := h.ids(), h.open, h.gen, h.snapshot()
	h.mu.Unlock()
	for mid, o := range all {
		if mid != id {
			o.members(ids, open, gen)
		}
	}
	return id, ids, nil
}

func (h *hub) snapshot() map[uint16]member {
	all := make(map[uint16]member, len(h.members))
	for id, m := range h.members {
		all[id] = m
	}
	return all
}

// remove takes id out and tells the others it left.
func (h *hub) remove(id uint16) {
	h.mu.Lock()
	if _, ok := h.members[id]; !ok {
		h.mu.Unlock()
		return
	}
	delete(h.members, id)
	h.gen++
	ids, open, gen, all := h.ids(), h.open, h.gen, h.snapshot()
	h.mu.Unlock()
	for _, o := range all {
		o.left(id)
		o.members(ids, open, gen)
	}
}

// rename names the game (a hosted relay's, from its creator's hello).
func (h *hub) rename(game string) {
	h.mu.Lock()
	h.game = game
	h.mu.Unlock()
}

// setOpen enables or disables new players.
func (h *hub) setOpen(open bool) {
	h.mu.Lock()
	h.open = open
	h.gen++
	ids, gen, all := h.ids(), h.gen, h.snapshot()
	h.mu.Unlock()
	for _, o := range all {
		o.members(ids, open, gen)
	}
}

func (h *hub) info() GameInfo {
	gi, _ := h.infoGen()
	return gi
}

func (h *hub) infoGen() (GameInfo, uint32) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return GameInfo{Name: h.game, Open: h.open && !h.closed, Users: h.ids()}, h.gen
}

// route delivers f from its sender: to one member, or to every member but
// the sender for Broadcast. A member whose link fails is removed.
func (h *hub) route(f Frame) error {
	h.mu.Lock()
	var to []uint16
	if f.To == Broadcast {
		for _, id := range h.ids() {
			if id != f.From {
				to = append(to, id)
			}
		}
	} else if _, ok := h.members[f.To]; ok {
		to = []uint16{f.To}
	} else {
		h.mu.Unlock()
		return ErrNoUser
	}
	all := h.snapshot()
	h.mu.Unlock()
	for _, id := range to {
		if err := all[id].deliver(f); err != nil {
			h.remove(id)
		}
	}
	return nil
}

// close ends the game: every member's link is gone.
func (h *hub) close() {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return
	}
	h.closed = true
	all := h.snapshot()
	h.members = map[uint16]member{}
	h.mu.Unlock()
	for _, m := range all {
		m.drop(ErrLost)
	}
}

// local is a member in this process: the creator of a TCP game, or any
// station of the in-memory transport.
type local struct {
	inbox
	h       *hub
	creator bool
	onClose func() error // the TCP relay's Close, for the creator
}

func (l *local) deliver(f Frame) error {
	l.push(f)
	return nil
}

func (l *local) members(ids []uint16, open bool, gen uint32) { l.setUsers(ids, open, gen) }
func (l *local) left(id uint16)                              { l.lost(id) }
func (l *local) drop(err error)                              { l.fail(err) }

func (l *local) Send(to, id uint16, payload []byte) error {
	if err := l.checkSend(to); err != nil {
		return err
	}
	f := Frame{ID: id, From: l.id, To: to, Payload: slices.Clone(payload)}
	if to == l.id {
		l.push(f)
		return nil
	}
	return l.h.route(f)
}

func (l *local) SetOpen(open bool) error {
	if !l.creator {
		return errors.New("netplay: only the creator opens or closes the game")
	}
	l.h.setOpen(open)
	return nil
}

// Close leaves the game. The creator's station leaves like any other: the
// relay goes on for the rest (a TCP relay closes once they have all left).
func (l *local) Close() error {
	l.fail(ErrClosed)
	if l.creator && l.onClose != nil {
		return l.onClose()
	}
	l.h.remove(l.id)
	return nil
}

// join adds a local station to h.
func join(h *hub, creator bool) (*local, error) {
	l := &local{h: h, creator: creator}
	l.game = h.game
	id, _, err := h.add(l)
	if err != nil {
		return nil, err
	}
	l.id = id
	gi, gen := h.infoGen()
	l.setUsers(gi.Users, gi.Open, gen)
	return l, nil
}
