package transport

import (
	"net"
)

type TransUDP struct {
	net.UDPAddr
	fakeTCP bool
}

type udpOptionFunc func(u *TransUDP)

func NewTransUDP(network, addr string, opts ...udpOptionFunc) (*TransUDP, error) {
	ip, err := net.ResolveUDPAddr(network, addr)
	if err != nil {
		return nil, err
	}

	u := &TransUDP{
		UDPAddr: *ip,
	}

	for _, opt := range opts {
		opt(u)
	}

	return u, nil
}

func WithFakeTCP(opt bool) udpOptionFunc {
	if opt {
		return func(u *TransUDP) {
			u.fakeTCP = true
		}
	}
	return func(u *TransUDP) {
		u.fakeTCP = false
	}
}

func (t *TransUDP) Address() net.Addr {
	return &t.UDPAddr
}

func (t *TransUDP) Type() TransType {
	return TypePacket
}
func (t *TransUDP) DialStream() (net.Conn, error) {
	return nil, errNotSupported
}
func (t *TransUDP) DialPacket() (net.PacketConn, error) {
	conn, err := net.DialUDP(t.Network(), nil, &t.UDPAddr)
	if err != nil {
		return nil, err
	}
	return conn, nil
}
func (t *TransUDP) ListenStream() (net.Listener, error) {
	return nil, errNotSupported
}
func (t *TransUDP) ListenPacket() (net.PacketConn, error) {
	conn, err := net.ListenUDP(t.Network(), &t.UDPAddr)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func (t *TransUDP) ListenAddr() net.Addr {
	return &t.UDPAddr
}
