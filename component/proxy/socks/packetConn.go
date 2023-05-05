package socks

import (
	"net"

	"github.com/intxff/rdcross/component/message"
	"github.com/intxff/rdcross/component/proxy"
)

const udpBufferSize = 1024 * 1024

type PacketSocks struct {
	Socks
	net.PacketConn
}

func NewPacketSocks(s Socks, c net.PacketConn) (*PacketSocks, error) {
	return &PacketSocks{Socks: s, PacketConn: c}, nil
}

type udpMsg struct {
	payload  []byte
	metadata *message.Metadata
}

func (u *udpMsg) Metadata() *message.Metadata {
	return u.metadata
}

func (u *udpMsg) Others() any {
	return nil
}

func (u *udpMsg) Payload() []byte {
	return u.payload
}

func (p *PacketSocks) Metadata() *message.Metadata {
	return nil
}

func (p *PacketSocks) ReadMsgFrom() (message.Message, net.Addr, error) {
	buf := make([]byte, udpBufferSize)
	n, rAddr, err := p.ReadFrom(buf)
	if err != nil {
		return nil, rAddr, err
	}
	m, err := p.deserialize(buf[:n])
	if err != nil {
		return m, rAddr, err
	}
	if p.mode == proxy.ModeServer {
		m.Metadata().WithClientIP(rAddr.(*net.UDPAddr).IP).
			WithClientPort(rAddr.(*net.UDPAddr).Port)
	}
	return m, rAddr, nil
}

func (p *PacketSocks) WriteMsgTo(m message.Message, addr net.Addr) error {
	data := make([]byte, 4, udpBufferSize)
	port := m.Metadata().RemotePort
	switch {
	case m.Metadata().RemoteIP != nil:
		ip := m.Metadata().RemoteIP
		if ip.To4() != nil {
			data = append(data, ip.To4()...)
			data = append(data, []byte{byte(port >> 8), byte(port)}...)
			break
		}
		if ip.To16() != nil {
			data = append(data, ip.To16()...)
			data = append(data, []byte{byte(port >> 8), byte(port)}...)
		}
	case m.Metadata().Domain != "":
		d := []byte(m.Metadata().Domain)
		l := len(d)
		data = append(data, byte(l))
		data = append(data, d[:l]...)
		data = append(data, []byte{byte(port >> 8), byte(port)}...)
	}
	data = append(data, m.Payload()...)
	if _, err := p.WriteTo(data, addr); err != nil {
		return err
	}
	return nil
}

func (p *PacketSocks) deserialize(b []byte) (message.Message, error) {
	var payload []byte
	m := message.NewMetadata()
	rsv, frag, atyp := int(b[0])<<8+int(b[1]), int(b[2]), b[3]
	if rsv != 0x0000 || frag != 0x00 {
		return nil, proxy.ErrNotSupported
	}
	switch atyp {
	case AtypIPv4:
		m.WithRemoteIP(net.IP(b[4:8])).
			WithRemotePort(int(b[8])<<8 + int(b[9]))
		payload = b[10:]
	case AtypIPv6:
		m.WithRemoteIP(net.IP(b[4:20])).
			WithRemotePort(int(b[20])<<8 + int(b[21]))
		payload = b[22:]
	case AtypDomain:
		l := int(b[4])
		m.WithDomain(string(b[5 : 5+l])).
			WithRemotePort(int(b[5+l])<<8 + int(b[6+l]))
		payload = b[7+l:]
	default:
		return nil, proxy.ErrNotSupported
	}

	return &udpMsg{
		payload:  payload,
		metadata: m,
	}, nil
}
