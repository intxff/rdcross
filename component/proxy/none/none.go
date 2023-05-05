package none

import (
	"errors"
	"net"

	"github.com/intxff/rdcross/component/conn"
	"github.com/intxff/rdcross/component/proxy"
)

const (
    _ = iota
)

var (
	ErrNotSupported     error = errors.New("not supported")
)

type None struct{
    mode int
}

func NewProxyNone(m int) *None {
	return &None{mode: m}
}

func (s *None) Cipher() proxy.Cipher {
	return nil
}

func (s *None) ShadowStreamConn(c net.Conn, extra ...any) (conn.ProxyStreamConn, error) {
    return NewStreamNone(c), nil
}

func (s *None) ShadowPacketConn(c net.PacketConn, extra ...any) (conn.ProxyPacketConn, error) {
	return NewPacketNone(*s, c)
}

func (s *None) Type() proxy.ProxyType {
	return proxy.TypeNone
}

func (s *None) TcpMux() bool {
	return false
}
