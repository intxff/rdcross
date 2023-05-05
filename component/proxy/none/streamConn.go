package none

import (
	"net"

	"github.com/intxff/rdcross/component/message"
)

type StreamNone struct {
	None
	net.Conn
	metadata *message.Metadata
}

func NewStreamNone(c net.Conn) *StreamNone {
	ss := &StreamNone{Conn: c, metadata: message.NewMetadata()}
	cAddr := c.RemoteAddr().(*net.TCPAddr)
	ss.metadata.WithClientIP(cAddr.IP).
		WithClientPort(cAddr.Port)
	return ss
}

func (s *StreamNone) Metadata() *message.Metadata {
    return s.metadata
}

func (s *StreamNone) ReadMux() (message.Message, error) {
    return nil, ErrNotSupported
}

func (s *StreamNone) WriteMux(msg message.Message) (error) {
    return ErrNotSupported
}
