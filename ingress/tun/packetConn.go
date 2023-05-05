package tun

import (
	"net"

	"github.com/intxff/rdcross/component/conn"
	"github.com/intxff/rdcross/component/message"
)

var _ message.Message = (*tunMsg)(nil)

type tunMsg struct {
	metadata *message.Metadata
	payload  []byte
}

func (t *tunMsg) Metadata() *message.Metadata {
	return t.metadata
}

func (t *tunMsg) Payload() []byte {
	return t.payload
}

func (t *tunMsg) Others() any {
	return nil
}

var _ conn.ProxyPacketConn = (*tunPacket)(nil)

type tunPacket struct {
	net.PacketConn
}

func newTunPacket(c net.PacketConn) *tunPacket {
	return &tunPacket{c}
}

func (t *tunPacket) Metadata() *message.Metadata {
	return nil
}

func (t *tunPacket) ReadMsgFrom() (message.Message, net.Addr, error) {
	buf := make([]byte, 64*1024)
	n, cAddr, err := t.ReadFrom(buf)
	if err != nil {
		return nil, cAddr, err
	}
	msg := &tunMsg{
		payload:  buf[:n],
		metadata: message.NewMetadata(),
	}
	msg.metadata.WithClientIP(cAddr.(*net.UDPAddr).IP).
		WithClientPort(cAddr.(*net.UDPAddr).Port)
	return msg, cAddr, nil
}

func (t *tunPacket) WriteMsgTo(m message.Message, addr net.Addr) error {
	if _, err := t.WriteTo(m.Payload(), addr); err != nil {
		return err
	}
	return nil
}
