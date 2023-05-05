package proxy

import (
	"errors"
	"net"

	"github.com/intxff/rdcross/component/conn"
)

type ProxyType string

const (
	TypeNone        ProxyType = "NONE"
	TypeShadowsocks ProxyType = "SHADOWSOCKS"
	TypeSocks       ProxyType = "SOCKS"
)

const (
	_ = iota
	ModeServer
	ModeClient
)

var ErrNotSupported = errors.New("not supported")

type Encrypter interface {
	Encrypt(planetext []byte, extra ...any) []byte
}

type Decrypter interface {
	Decrypt(ciphtext []byte, extra ...any) ([]byte, error)
}

type Cipher interface {
	Encrypter(extra ...any) (Encrypter, error)
	Decrypter(extra ...any) (Decrypter, error)
}

// Proxy reads out and write in message in []byte binary format
type Proxy interface {
	Cipher() Cipher
	ShadowStreamConn(c net.Conn, extra ...any) (conn.ProxyStreamConn, error)
	ShadowPacketConn(c net.PacketConn, extra ...any) (conn.ProxyPacketConn, error)
	Type() ProxyType
	// tcpmux implys whether many different msgs within a connection
	TcpMux() bool
}
