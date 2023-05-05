package egress

import (
	"github.com/intxff/rdcross/component/conn"
	"github.com/intxff/rdcross/component/message"
	"github.com/intxff/rdcross/component/proxy"
	"github.com/intxff/rdcross/component/transport"
)

const (
	_ int32 = iota
	Ready
	Running
	Closed
)

// Egress is the node that packets flow out
type Egress interface {
	Type() EgressType
	Name() string
	ProcessStream(c conn.ProxyStreamConn, msg message.Message)
	ProcessPacket(c conn.ProxyPacketConn, msg message.Message)
	Proxy() proxy.Proxy
	Transport() (stream, packet transport.Transport)
	Close() <-chan struct{}
}

type EgressType string

const (
	TypeGeneral EgressType = "GENERAL"
	TypeDirect  EgressType = "DIRECT"
	TypeReject  EgressType = "REJECT"
)
