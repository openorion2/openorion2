//go:build !unix

package netplay

import "net"

// listenBroadcast is a UDP socket; without SO_BROADCAST here only this
// machine is asked.
func listenBroadcast() (*net.UDPConn, error) { return net.ListenUDP("udp4", nil) }
