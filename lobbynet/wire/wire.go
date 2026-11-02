// Package wire frames lobby messages on a byte stream: a big-endian u32
// length, then one lobbyv1.Envelope of that many bytes.
package wire

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/openorion2/openorion2/lobbynet/lobbyv1"
)

// MaxFrame bounds an envelope: a game frame carries at most 0xffff bytes
// of payload, chat and lists far less.
const MaxFrame = 1 << 20

// Timeouts of a link: a ping goes out when nothing was written for
// PingEvery, and a link that brought nothing for DeadAfter is dead.
const (
	PingEvery    = 10 * time.Second
	DeadAfter    = 30 * time.Second
	WriteTimeout = 20 * time.Second
)

// Write sends one envelope.
func Write(w io.Writer, env *lobbyv1.Envelope) error {
	b, err := proto.Marshal(env)
	if err != nil {
		return err
	}
	if len(b) > MaxFrame {
		return fmt.Errorf("wire: frame of %d bytes", len(b))
	}
	buf := make([]byte, 4, 4+len(b))
	binary.BigEndian.PutUint32(buf, uint32(len(b)))
	_, err = w.Write(append(buf, b...))
	return err
}

// Read receives one envelope.
func Read(r io.Reader) (*lobbyv1.Envelope, error) {
	var head [4]byte
	if _, err := io.ReadFull(r, head[:]); err != nil {
		return nil, err
	}
	n := binary.BigEndian.Uint32(head[:])
	if n > MaxFrame {
		return nil, fmt.Errorf("wire: frame of %d bytes", n)
	}
	b := make([]byte, n)
	if _, err := io.ReadFull(r, b); err != nil {
		return nil, unexpected(err)
	}
	env := &lobbyv1.Envelope{}
	if err := proto.Unmarshal(b, env); err != nil {
		return nil, fmt.Errorf("wire: %w", err)
	}
	return env, nil
}

func unexpected(err error) error {
	if err == io.EOF {
		return io.ErrUnexpectedEOF
	}
	return err
}

// Conn is a link that many goroutines may write to; one goroutine reads.
// It pings when idle and fails reads after DeadAfter of silence.
type Conn struct {
	c    net.Conn
	mu   sync.Mutex
	last time.Time // of the last write
	done chan struct{}
	once sync.Once
}

// NewConn wraps c and starts its pinger.
func NewConn(c net.Conn) *Conn {
	w := &Conn{c: c, last: time.Now(), done: make(chan struct{})}
	go w.pinger()
	return w
}

func (w *Conn) pinger() {
	t := time.NewTicker(PingEvery / 4)
	defer t.Stop()
	for {
		select {
		case <-w.done:
			return
		case <-t.C:
			w.mu.Lock()
			idle := time.Since(w.last)
			w.mu.Unlock()
			if idle >= PingEvery {
				if w.Send(&lobbyv1.Envelope{Msg: &lobbyv1.Envelope_Ping{Ping: &lobbyv1.Ping{}}}) != nil {
					w.Close()
					return
				}
			}
		}
	}
}

// Send writes one envelope.
func (w *Conn) Send(env *lobbyv1.Envelope) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.c.SetWriteDeadline(time.Now().Add(WriteTimeout))
	w.last = time.Now()
	return Write(w.c, env)
}

// Recv reads the next envelope that is not a ping.
func (w *Conn) Recv() (*lobbyv1.Envelope, error) {
	for {
		w.c.SetReadDeadline(time.Now().Add(DeadAfter))
		env, err := Read(w.c)
		if err != nil {
			return nil, err
		}
		if env.GetPing() == nil {
			return env, nil
		}
	}
}

// RemoteAddr is the peer's address.
func (w *Conn) RemoteAddr() net.Addr { return w.c.RemoteAddr() }

// Close closes the link; reads and writes fail from then on.
func (w *Conn) Close() error {
	var err error
	w.once.Do(func() {
		close(w.done)
		err = w.c.Close()
	})
	return err
}
