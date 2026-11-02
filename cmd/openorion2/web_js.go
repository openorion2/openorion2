//go:build js

package main

import (
	"syscall/js"

	"github.com/openorion2/openorion2/internal/netplay"
)

// webNet points the transport at the serving host's /net WebSocket: the
// browser cannot listen or dial TCP (docs/net/web.md). Joining defaults
// to a game created on that same host.
func webNet(t *netplay.TCP) {
	loc := js.Global().Get("location")
	u := js.Global().Get("URL").New("net", loc.Get("href"))
	if u.Get("protocol").String() == "https:" {
		u.Set("protocol", "wss:")
	} else {
		u.Set("protocol", "ws:")
	}
	u.Set("search", "")
	u.Set("hash", "")
	t.Web = u.Call("toString").String()
	if t.Addr == "" {
		t.Addr = loc.Get("hostname").String()
	}
}
