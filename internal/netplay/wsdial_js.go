//go:build js

package netplay

import (
	"errors"
	"io"
	"net"
	"os"
	"sync"
	"syscall/js"
	"time"
)

// dialWebSocket opens the browser's own WebSocket to url and makes it a
// net.Conn. Its callbacks only queue and signal: they never block, as JS
// callbacks must not (docs/web.md).
func dialWebSocket(url string) (net.Conn, error) {
	ws := js.Global().Get("WebSocket").New(url)
	ws.Set("binaryType", "arraybuffer")
	c := &jsConn{ws: ws, signal: make(chan struct{}, 1)}
	opened := make(chan bool, 1)
	c.on("open", func(js.Value) {
		select {
		case opened <- true:
		default:
		}
	})
	c.on("message", func(ev js.Value) {
		data := js.Global().Get("Uint8Array").New(ev.Get("data"))
		b := make([]byte, data.Get("length").Int())
		js.CopyBytesToGo(b, data)
		c.mu.Lock()
		c.buf = append(c.buf, b...)
		c.mu.Unlock()
		c.wake()
	})
	c.on("close", func(js.Value) {
		c.mu.Lock()
		c.eof = true
		c.mu.Unlock()
		c.wake()
		select {
		case opened <- false:
		default:
		}
	})
	select {
	case ok := <-opened:
		if !ok {
			c.Close()
			return nil, errors.New("netplay: cannot reach the server's /net")
		}
	case <-time.After(handshakeTimeout):
		c.Close()
		return nil, os.ErrDeadlineExceeded
	}
	return c, nil
}

// jsConn is a browser WebSocket as a net.Conn.
type jsConn struct {
	ws     js.Value
	funcs  []js.Func
	signal chan struct{} // a message came or the socket closed

	mu     sync.Mutex
	buf    []byte
	eof    bool
	closed bool
	rdl    time.Time
}

func (c *jsConn) on(ev string, f func(js.Value)) {
	fn := js.FuncOf(func(this js.Value, args []js.Value) any {
		f(args[0])
		return nil
	})
	c.funcs = append(c.funcs, fn)
	c.ws.Call("addEventListener", ev, fn)
}

func (c *jsConn) wake() {
	select {
	case c.signal <- struct{}{}:
	default:
	}
}

func (c *jsConn) Read(p []byte) (int, error) {
	for {
		c.mu.Lock()
		if len(c.buf) > 0 {
			n := copy(p, c.buf)
			c.buf = c.buf[n:]
			c.mu.Unlock()
			return n, nil
		}
		eof, closed, dl := c.eof, c.closed, c.rdl
		c.mu.Unlock()
		switch {
		case closed:
			return 0, net.ErrClosed
		case eof:
			return 0, io.EOF
		}
		if dl.IsZero() {
			<-c.signal
			continue
		}
		d := time.Until(dl)
		if d <= 0 {
			return 0, os.ErrDeadlineExceeded
		}
		t := time.NewTimer(d)
		select {
		case <-c.signal:
			t.Stop()
		case <-t.C:
			return 0, os.ErrDeadlineExceeded
		}
	}
}

// Write hands p to the browser, which sends it as one binary message.
func (c *jsConn) Write(p []byte) (int, error) {
	c.mu.Lock()
	bad := c.closed || c.eof
	c.mu.Unlock()
	if bad {
		return 0, net.ErrClosed
	}
	a := js.Global().Get("Uint8Array").New(len(p))
	js.CopyBytesToJS(a, p)
	c.ws.Call("send", a)
	return len(p), nil
}

func (c *jsConn) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return net.ErrClosed
	}
	c.closed = true
	c.mu.Unlock()
	c.ws.Call("close")
	c.wake()
	// The callbacks stay: the close event may still come.
	return nil
}

type jsAddr struct{}

func (jsAddr) Network() string { return "websocket" }
func (jsAddr) String() string  { return "websocket" }

func (c *jsConn) LocalAddr() net.Addr  { return jsAddr{} }
func (c *jsConn) RemoteAddr() net.Addr { return jsAddr{} }

func (c *jsConn) SetDeadline(t time.Time) error { return c.SetReadDeadline(t) }

func (c *jsConn) SetReadDeadline(t time.Time) error {
	c.mu.Lock()
	c.rdl = t
	c.mu.Unlock()
	return nil
}

// SetWriteDeadline is a no-op: the browser queues every send.
func (c *jsConn) SetWriteDeadline(time.Time) error { return nil }
