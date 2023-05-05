package tun

import (
	"net"

	"github.com/intxff/rdcross/component/message"
)

type tunStream struct {
    net.Conn
	metadata *message.Metadata
}

func newTunStream(c net.Conn, m *message.Metadata) *tunStream {
    return &tunStream{c, m}
}

func (t *tunStream) ReadMux() (msg message.Message, err error) {
    return nil, errNotSupported
}

func (t *tunStream) WriteMux(msg message.Message) (err error) {
    return errNotSupported
}

func (t *tunStream) Metadata() *message.Metadata {
    return t.metadata
}
