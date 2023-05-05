// Provide maybe modified tcp or udp connection to proxy
package transport

import (
	"errors"
	"net"
)

type Transport interface {
	Type() TransType
    Address() net.Addr
	DialStream() (net.Conn, error)
	DialPacket() (net.PacketConn, error)
    ListenAddr() net.Addr
	ListenStream() (net.Listener, error)
	ListenPacket() (net.PacketConn, error)
}

type TransType string
const (
    TypeStream TransType = "TCP"
    TypePacket TransType = "UDP"
)

const (
    _ uint8 = iota
    ipv4
    ipv6
)

var errNotSupported = errors.New("not supported")
