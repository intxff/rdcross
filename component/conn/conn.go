// other parts are machines. connection is product.
// connection records all the information from
// ingress to egress. connection is created in
// ingress and then transported to other parts.
package conn

import (
	"net"

	"github.com/intxff/rdcross/component/message"
)

type ConnType uint8

const (
	_ ConnType = iota
	StreamConn
	PacketConn
)

type ProxyPacketConn interface {
	net.PacketConn
	Metadata() *message.Metadata
	ReadMsgFrom() (message.Message, net.Addr, error)
	WriteMsgTo(message.Message, net.Addr) error
}

type ProxyStreamConn interface {
	net.Conn
	// many msg with one conn
	ReadMux() (msg message.Message, err error)
	WriteMux(msg message.Message) (err error)
	// only one conn, metadata info should be stored
	Metadata() *message.Metadata
}
