//go:build !js

package netplay

import "net"

// dialWebSocket is DialWebSocket; the browser build has its own.
func dialWebSocket(url string) (net.Conn, error) { return DialWebSocket(url) }
