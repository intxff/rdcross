package reject

import (
	"github.com/intxff/rdcross/component/conn"
	"github.com/intxff/rdcross/component/message"
	"github.com/intxff/rdcross/component/proxy"
	"github.com/intxff/rdcross/component/transport"
	"github.com/intxff/rdcross/egress"
)

var _ egress.Egress = (*Reject)(nil)

type Reject struct{}

func NewReject() *Reject {
	return &Reject{}
}

func (g *Reject) Type() egress.EgressType {
	return egress.TypeReject
}

func (g *Reject) Name() string {
	return string(egress.TypeReject)
}

func (g *Reject) Proxy() proxy.Proxy {
	return nil
}

func (g *Reject) Transport() (stream, packet transport.Transport) {
	return nil, nil
}

func (g *Reject) Close() <-chan struct{} {
	ch := make(chan struct{}, 1)
	ch <- struct{}{}
	return ch
}

func (g *Reject) ProcessStream(c conn.ProxyStreamConn, msg message.Message) {
}

func (g *Reject) ProcessPacket(c conn.ProxyPacketConn, msg message.Message) {
}
