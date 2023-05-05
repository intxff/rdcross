package socks

import (
	"errors"
	"net"

	"github.com/intxff/rdcross/component/conn"
	"github.com/intxff/rdcross/component/message"
	"github.com/intxff/rdcross/component/proxy"
)

const (
	_ = iota
	StatusFree
	StatusHandshaking
	StatusConnected
)

var (
	ErrNotSupported    error = errors.New("not supported")
	ErrVerNotSupported error = errors.New("need socks verion 5")
	ErrAuthFailed      error = errors.New("auth failed")
	ErrCmdNotSupported error = errors.New("cmd not supported")
)

type Socks struct {
	mode int
}

func NewProxySocks(mode int) *Socks {
	return &Socks{mode: mode}
}

func (s *Socks) Cipher() proxy.Cipher {
	return nil
}

func (s *Socks) ShadowStreamConn(c net.Conn, extra ...any) (conn.ProxyStreamConn, error) {
	if s.mode == proxy.ModeServer {
		m := message.NewMetadata().WithIngress(extra[0].(string))
		return NewStreamSocks(*s, c, m)
	}
	return NewStreamSocks(*s, c, extra[0].(*message.Metadata))
}

func (s *Socks) ShadowPacketConn(c net.PacketConn, extra ...any) (conn.ProxyPacketConn, error) {
	return NewPacketSocks(*s, c)
}

func (s *Socks) Type() proxy.ProxyType {
	return proxy.TypeSocks
}

func (s *Socks) TcpMux() bool {
	return false
}
