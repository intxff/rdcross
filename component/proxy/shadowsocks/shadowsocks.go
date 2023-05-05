package shadowsocks

import (
	"net"

	"github.com/intxff/rdcross/component/conn"
	"github.com/intxff/rdcross/component/message"
	"github.com/intxff/rdcross/component/proxy"
)

type Shadowsocks struct {
	mode     int
	password []byte
	udp      bool
	key      []byte
	cipher   proxy.Cipher
}

func NewProxyShadowsocks(mode int, password, cipher, key string, udp bool) (*Shadowsocks, error) {
	p, k := []byte(password), []byte(key)
	c, k, err := newAeadCipher(p, k, cipher)
	if err != nil {
		return nil, err
	}
	return &Shadowsocks{
		mode:     mode,
		password: p,
		udp:      udp,
		key:      k,
		cipher:   c,
	}, nil
}

func (s *Shadowsocks) Type() proxy.ProxyType {
	return proxy.TypeShadowsocks
}

func (s *Shadowsocks) TcpMux() bool {
	return false
}

func (s *Shadowsocks) Cipher() proxy.Cipher {
	return s.cipher
}

func (s *Shadowsocks) ShadowStreamConn(c net.Conn, extra ...any) (conn.ProxyStreamConn, error) {
	m := extra[0].(*message.Metadata)
	return newStreamShadowsocks(*s, c, m)
}
func (s *Shadowsocks) ShadowPacketConn(c net.PacketConn, extra ...any) (conn.ProxyPacketConn, error) {
	return newPacketShadowsocks(*s, c), nil
}
