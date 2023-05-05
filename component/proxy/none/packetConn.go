package none

import (
	"net"

	"github.com/intxff/rdcross/component/message"
	"github.com/intxff/rdcross/component/proxy"
)

const udpBufferSize = 1024 * 1024

type PacketNone struct {
	None
	net.PacketConn
}

func NewPacketNone(n None, c net.PacketConn) (*PacketNone, error) {
	return &PacketNone{None: n, PacketConn: c}, nil
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

func (p *PacketNone) Metadata() *message.Metadata {
	return nil
}

func (p *PacketNone) ReadMsgFrom() (message.Message, net.Addr, error) {
	buf := make([]byte, udpBufferSize)
	n, rAddr, err := p.ReadFrom(buf)
	if err != nil {
		return nil, rAddr, err
	}
	msg := &udpMsg{
		payload:  buf[:n],
		metadata: message.NewMetadata(),
	}
	if p.mode == proxy.ModeServer {
		msg.metadata.WithClientIP(rAddr.(*net.UDPAddr).IP).
			WithClientPort(rAddr.(*net.UDPAddr).Port)
	}
	return msg, rAddr, nil
}

func (p *PacketNone) WriteMsgTo(m message.Message, addr net.Addr) error {
	if _, err := p.WriteTo(m.Payload(), addr); err != nil {
		return err
	}
	return nil
}
