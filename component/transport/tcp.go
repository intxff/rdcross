package transport

import (
	"net"

	"github.com/intxff/rdcross/component/iface"
)

type TransTCP struct {
	net.TCPAddr
	ipType uint8
	Smux   bool
}

type tcpOptionFunc func(t *TransTCP)

func NewTransTCP(network, addr string, opts ...tcpOptionFunc) (*TransTCP, error) {
	tcpAddr, err := net.ResolveTCPAddr(network, addr)
	if err != nil {
		return nil, err
	}

	t := &TransTCP{
		TCPAddr: *tcpAddr,
		Smux:    false,
	}

	if tcpAddr.IP.To4() != nil {
		t.ipType = ipv4
	} else {
		t.ipType = ipv6
	}

	for _, opt := range opts {
		opt(t)
	}

	return t, nil
}

func WithSmux(opt bool) tcpOptionFunc {
	if opt {
		return func(t *TransTCP) {
			t.Smux = true
		}
	}
	return func(t *TransTCP) {
		t.Smux = false
	}
}

func (t *TransTCP) Address() net.Addr {
	return &t.TCPAddr
}

func (t *TransTCP) DialStream() (net.Conn, error) {
	lAddr := &net.TCPAddr{Port: 0}
	if t.ipType == ipv4 {
		lIP, err := iface.GetIPv4()
		if err != nil {
			return nil, err
		}
		lAddr.IP = lIP
	} else {
		lIP, err := iface.GetIPv6()
		if err != nil {
			return nil, err
		}
		lAddr.IP = lIP
	}

	conn, err := net.DialTCP("tcp", lAddr, &t.TCPAddr)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func (t *TransTCP) DialPacket() (net.PacketConn, error) {
	return nil, errNotSupported
}

func (t *TransTCP) ListenStream() (net.Listener, error) {
	l, err := net.ListenTCP("tcp", &t.TCPAddr)
	if err != nil {
		return l, err
	}
	return l, nil
}

func (t *TransTCP) ListenPacket() (net.PacketConn, error) {
	return nil, errNotSupported
}

func (t *TransTCP) Type() TransType {
	return TypeStream
}

func (t *TransTCP) ListenAddr() net.Addr {
	return &t.TCPAddr
}
