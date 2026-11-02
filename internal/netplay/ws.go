package netplay

import (
	"bufio"
	"crypto/rand"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// A minimal WebSocket (RFC 6455) for the browser build's link to
// cmd/webserve: binary messages only, no extensions. A wsConn carries the
// same byte stream as a TCP link, so frames go over it unchanged
// (docs/net/web.md).

const wsGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

// wsMaxFrame bounds one WebSocket frame: a netplay frame is at most
// 10 + 0xffff bytes, and the browser sends each in one message.
const wsMaxFrame = 1 << 20

const (
	wsCont   = 0x0
	wsText   = 0x1
	wsBinary = 0x2
	wsClose  = 0x8
	wsPing   = 0x9
	wsPong   = 0xa
)

var errWSFrame = errors.New("netplay: bad websocket frame")

func wsAccept(key string) string {
	h := sha1.Sum([]byte(key + wsGUID))
	return base64.StdEncoding.EncodeToString(h[:])
}

// wsConn is a WebSocket as a net.Conn: Read returns the data messages'
// bytes as one stream, Write sends each call as one binary message.
type wsConn struct {
	c      net.Conn
	br     *bufio.Reader
	client bool // a client masks what it sends

	rmu    sync.Mutex // one reader at a time
	remain int64      // bytes left in the current data frame
	mask   [4]byte
	masked bool
	mpos   int

	wmu    sync.Mutex
	closed bool
	once   sync.Once
}

func (w *wsConn) Read(p []byte) (int, error) {
	w.rmu.Lock()
	defer w.rmu.Unlock()
	for w.remain == 0 {
		if err := w.next(); err != nil {
			return 0, err
		}
	}
	if int64(len(p)) > w.remain {
		p = p[:w.remain]
	}
	n, err := w.br.Read(p)
	if w.masked {
		for i := range n {
			p[i] ^= w.mask[w.mpos&3]
			w.mpos++
		}
	}
	w.remain -= int64(n)
	return n, err
}

// next reads frame headers up to the next data frame; control frames are
// answered on the way.
func (w *wsConn) next() error {
	var h [14]byte
	if _, err := io.ReadFull(w.br, h[:2]); err != nil {
		return err
	}
	op, masked := h[0]&0x0f, h[1]&0x80 != 0
	if h[0]&0x70 != 0 || masked == w.client {
		return errWSFrame // reserved bits, or the wrong side's masking
	}
	n := int64(h[1] & 0x7f)
	switch n {
	case 126:
		if _, err := io.ReadFull(w.br, h[2:4]); err != nil {
			return unexpected(err)
		}
		n = int64(binary.BigEndian.Uint16(h[2:4]))
	case 127:
		if _, err := io.ReadFull(w.br, h[2:10]); err != nil {
			return unexpected(err)
		}
		n = int64(binary.BigEndian.Uint64(h[2:10]))
	}
	if n > wsMaxFrame {
		return errWSFrame
	}
	w.masked, w.mpos = masked, 0
	if masked {
		if _, err := io.ReadFull(w.br, w.mask[:]); err != nil {
			return unexpected(err)
		}
	}
	switch op {
	case wsCont, wsBinary, wsText:
		w.remain = n
		return nil
	case wsClose, wsPing, wsPong:
		if n > 125 || h[0]&0x80 == 0 {
			return errWSFrame
		}
		b := make([]byte, n)
		if _, err := io.ReadFull(w.br, b); err != nil {
			return unexpected(err)
		}
		if masked {
			for i := range b {
				b[i] ^= w.mask[i&3]
			}
		}
		switch op {
		case wsPing:
			return w.send(wsPong, b)
		case wsClose:
			w.send(wsClose, nil)
			return io.EOF
		}
		return nil
	}
	return errWSFrame
}

// send writes one whole frame.
func (w *wsConn) send(op byte, p []byte) error {
	w.wmu.Lock()
	defer w.wmu.Unlock()
	if w.closed {
		return net.ErrClosed
	}
	if op == wsClose {
		w.closed = true
	}
	b := make([]byte, 0, 14+len(p))
	b = append(b, 0x80|op)
	var mbit byte
	if w.client {
		mbit = 0x80
	}
	switch n := len(p); {
	case n < 126:
		b = append(b, mbit|byte(n))
	case n <= 0xffff:
		b = binary.BigEndian.AppendUint16(append(b, mbit|126), uint16(n))
	default:
		b = binary.BigEndian.AppendUint64(append(b, mbit|127), uint64(n))
	}
	if w.client {
		var m [4]byte
		rand.Read(m[:])
		b = append(b, m[:]...)
		for i, c := range p {
			b = append(b, c^m[i&3])
		}
	} else {
		b = append(b, p...)
	}
	_, err := w.c.Write(b)
	return err
}

func (w *wsConn) Write(p []byte) (int, error) {
	if err := w.send(wsBinary, p); err != nil {
		return 0, err
	}
	return len(p), nil
}

// Close sends a close frame (if the link still takes one) and closes.
func (w *wsConn) Close() error {
	err := net.ErrClosed
	w.once.Do(func() {
		w.c.SetWriteDeadline(time.Now().Add(time.Second))
		w.send(wsClose, nil)
		err = w.c.Close()
	})
	return err
}

func (w *wsConn) LocalAddr() net.Addr                { return w.c.LocalAddr() }
func (w *wsConn) RemoteAddr() net.Addr               { return w.c.RemoteAddr() }
func (w *wsConn) SetDeadline(t time.Time) error      { return w.c.SetDeadline(t) }
func (w *wsConn) SetReadDeadline(t time.Time) error  { return w.c.SetReadDeadline(t) }
func (w *wsConn) SetWriteDeadline(t time.Time) error { return w.c.SetWriteDeadline(t) }

func headerHas(h http.Header, name, token string) bool {
	for _, v := range h.Values(name) {
		for _, t := range strings.Split(v, ",") {
			if strings.EqualFold(strings.TrimSpace(t), token) {
				return true
			}
		}
	}
	return false
}

// AcceptWebSocket upgrades an HTTP request to a WebSocket. A browser's
// request from another origin is refused: a page elsewhere must not use
// the server's relays.
func AcceptWebSocket(w http.ResponseWriter, r *http.Request) (net.Conn, error) {
	key := r.Header.Get("Sec-WebSocket-Key")
	if r.Method != http.MethodGet || !headerHas(r.Header, "Connection", "upgrade") ||
		!headerHas(r.Header, "Upgrade", "websocket") || key == "" {
		http.Error(w, "websocket only", http.StatusBadRequest)
		return nil, errors.New("netplay: not a websocket request")
	}
	if r.Header.Get("Sec-WebSocket-Version") != "13" {
		w.Header().Set("Sec-WebSocket-Version", "13")
		http.Error(w, "websocket version 13 only", http.StatusUpgradeRequired)
		return nil, errors.New("netplay: websocket version")
	}
	if o := r.Header.Get("Origin"); o != "" {
		if u, err := url.Parse(o); err != nil || !strings.EqualFold(u.Host, r.Host) {
			http.Error(w, "cross-origin websocket", http.StatusForbidden)
			return nil, fmt.Errorf("netplay: websocket from origin %q", o)
		}
	}
	c, rw, err := http.NewResponseController(w).Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return nil, err
	}
	c.SetDeadline(time.Time{})
	_, err = fmt.Fprintf(c, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: %s\r\n\r\n", wsAccept(key))
	if err != nil {
		c.Close()
		return nil, err
	}
	return &wsConn{c: c, br: rw.Reader}, nil
}

// DialWebSocket opens a WebSocket to a ws:// or wss:// URL from Go (the
// browser build uses the browser's own WebSocket instead; tests use this).
func DialWebSocket(rawURL string) (net.Conn, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	host := u.Host
	if u.Port() == "" {
		port := "80"
		if u.Scheme == "wss" {
			port = "443"
		}
		host = net.JoinHostPort(u.Hostname(), port)
	}
	d := net.Dialer{Timeout: handshakeTimeout}
	var c net.Conn
	switch u.Scheme {
	case "ws":
		c, err = d.Dial("tcp", host)
	case "wss":
		c, err = tls.DialWithDialer(&d, "tcp", host, &tls.Config{ServerName: u.Hostname()})
	default:
		return nil, fmt.Errorf("netplay: %q is not a websocket URL", rawURL)
	}
	if err != nil {
		return nil, err
	}
	var k [16]byte
	rand.Read(k[:])
	key := base64.StdEncoding.EncodeToString(k[:])
	c.SetDeadline(time.Now().Add(handshakeTimeout))
	_, err = fmt.Fprintf(c, "GET %s HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: %s\r\nSec-WebSocket-Version: 13\r\n\r\n", u.RequestURI(), u.Host, key)
	if err != nil {
		c.Close()
		return nil, err
	}
	br := bufio.NewReader(c)
	resp, err := http.ReadResponse(br, &http.Request{Method: http.MethodGet})
	if err != nil {
		c.Close()
		return nil, err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusSwitchingProtocols || resp.Header.Get("Sec-WebSocket-Accept") != wsAccept(key) {
		c.Close()
		return nil, fmt.Errorf("netplay: websocket refused: %s", resp.Status)
	}
	c.SetDeadline(time.Time{})
	return &wsConn{c: c, br: br, client: true}, nil
}
