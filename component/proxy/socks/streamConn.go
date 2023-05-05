package socks

import (
	"net"

	"github.com/intxff/rdcross/component/message"
	"github.com/intxff/rdcross/component/proxy"
)

type StreamSocks struct {
	Socks
	net.Conn
	status   int
	metadata *message.Metadata
}

func NewStreamSocks(s Socks, c net.Conn, m *message.Metadata) (*StreamSocks, error) {
	ss := &StreamSocks{
		Socks:    s,
		Conn:     c,
		status:   StatusFree,
		metadata: m,
	}
	switch ss.mode {
	case proxy.ModeServer:
		if err := ss.serverHandShake(); err != nil {
			return nil, err
		}
	case proxy.ModeClient:
		if err := ss.clientHandShake(); err != nil {
			return nil, err
		}
	}
	return ss, nil
}

func (s *StreamSocks) Metadata() *message.Metadata {
	return s.metadata
}

func (s *StreamSocks) serverHandShake() error {
	var (
		atyp byte
		addr string
		port int
	)
	buffer := make([]byte, 300)
	switch s.status {
	// check whether version is correct
	case StatusFree:
		_, err := s.Conn.Read(buffer[:2])
		if err != nil {
			return err
		}
		if buffer[VerPos] != Ver {
			return ErrVerNotSupported
		}
		s.status = StatusHandshaking
		fallthrough
	case StatusHandshaking:
		// select method
		nmethods := int(buffer[NMethodsPos])
		if _, err := s.Conn.Read(buffer[:nmethods]); err != nil {
			s.status = StatusFree
			return err
		}
		// no check then return method no auth
		if _, err := s.Write([]byte{Ver, MethodNoAuth}); err != nil {
			s.status = StatusFree
			return err
		}
		// read first 4 bytes of request
		if _, err := s.Conn.Read(buffer[:(AtypPos + 1)]); err != nil {
			s.status = StatusFree
			return err
		}
		// check command
		if cmd := buffer[CmdPos]; cmd != CmdConnect {
			if cmd != CmdUdpAssociate {
				s.Write([]byte{Ver, ReplyCmdNotSupported, Rsv, AtypIPv4, 0, 0, 0, 0, 0, 0})
				return ErrCmdNotSupported
			}
            ip := s.LocalAddr().(*net.TCPAddr).IP
            port := s.LocalAddr().(*net.TCPAddr).Port
            rep := append([]byte{Ver, ReplyCmdNotSupported, Rsv, AtypIPv4}, ip...)
            rep = append(rep, byte(port>>8), byte(port))
			s.Write(rep)
            // stay here
            for {
                _, err := s.Read(make([]byte, 10))
                if err != nil {
                    return nil
                }
            }
		}
		// directly return succeed
		if _, err := s.Write([]byte{Ver, ReplySucceeded, Rsv, AtypIPv4, 0, 0, 0, 0, 0, 0}); err != nil {
			s.status = StatusFree
			return err
		}
		// get address and port
		atyp = buffer[AtypPos]
		switch atyp {
		case AtypDomain:
			if _, err := s.Conn.Read(buffer[:1]); err != nil {
				s.status = StatusFree
				return err
			}
			if n, err := s.Conn.Read(buffer[:buffer[0]]); err != nil {
				s.status = StatusFree
				return err
			} else {
				addr = string(buffer[:n])
			}
			s.metadata = message.NewMetadata().WithDomain(addr)
		case AtypIPv4:
			if _, err := s.Conn.Read(buffer[:Ipv4Size]); err != nil {
				s.status = StatusFree
				return err
			}
			addr = string(net.IPv4(buffer[0], buffer[1], buffer[2], buffer[3]))
			s.metadata = message.NewMetadata().WithRemoteIP(net.IP(addr))
		case AtypIPv6:
			if _, err := s.Conn.Read(buffer[:Ipv6Size]); err != nil {
				s.status = StatusFree
				return err
			}
			addr = string(net.IP(buffer[:Ipv6Size]))
			s.metadata = message.NewMetadata().WithRemoteIP(net.IP(addr))
		}
		if _, err := s.Conn.Read(buffer[:PortSize]); err != nil {
			s.status = StatusFree
			return err
		}
		port = int(buffer[0])<<8 + int(buffer[1])
		// create metadata
		// get client ip and port
		cAddr := s.RemoteAddr().(*net.TCPAddr)
		s.metadata.WithClientIP(cAddr.IP).
			WithClientPort(cAddr.Port).
			WithRemotePort(port)
		s.status = StatusConnected
		fallthrough
	case StatusConnected:
		return nil
	}
	return nil
}

func (s *StreamSocks) clientHandShake() error {
	buffer := make([]byte, 1500)
	switch s.status {
	case StatusFree:
		// start handshaking
		if _, err := s.Conn.Write([]byte{Ver, 0x01, MethodNoAuth}); err != nil {
			return err
		}
		if _, err := s.Conn.Read(buffer[:2]); err != nil {
			return err
		}
		s.status = StatusHandshaking
		fallthrough
	case StatusHandshaking:
		// negotation command and remote address
		bPort := []byte{byte(s.metadata.RemotePort >> 8), byte(s.metadata.RemotePort & 0xff)}
		if s.metadata.Domain != "" {
			t := append([]byte{Ver, CmdConnect, Rsv, AtypDomain}, s.metadata.Domain[:]...)
			t = append(t, bPort...)
			if _, err := s.Conn.Write(t); err != nil {
				s.status = StatusFree
				return err
			}
			goto WaitForReply
		}
		if s.metadata.RemoteIP.To4() != nil {
			t := append([]byte{Ver, CmdConnect, Rsv, AtypIPv4}, s.metadata.RemoteIP...)
			t = append(t, bPort...)
			if _, err := s.Conn.Write(t); err != nil {
				s.status = StatusFree
				return err
			}
			goto WaitForReply
		}
		if s.metadata.RemoteIP.To16() != nil {
			t := append([]byte{Ver, CmdConnect, Rsv, AtypIPv6}, s.metadata.RemoteIP...)
			t = append(t, bPort...)
			if _, err := s.Conn.Write(t); err != nil {
				s.status = StatusFree
				return err
			}
		}
	WaitForReply:
		if _, err := s.Conn.Read(buffer[:4]); err != nil {
			s.status = StatusFree
			return err
		}
		if buffer[ReplyPos] != ReplySucceeded {
			s.status = StatusFree
			return ErrNotSupported
		}
		// consume reply left
		switch buffer[AtypPos] {
		case AtypDomain:
			if _, err := s.Conn.Read(buffer[:1]); err != nil {
				s.status = StatusFree
				return err
			}
			if _, err := s.Conn.Read(buffer[:(buffer[0] + PortSize)]); err != nil {
				s.status = StatusFree
				return err
			}
		case AtypIPv4:
			if _, err := s.Conn.Read(buffer[:Ipv4Size+PortSize]); err != nil {
				s.status = StatusFree
				return err
			}
		case AtypIPv6:
			if _, err := s.Conn.Read(buffer[:Ipv6Size+PortSize]); err != nil {
				s.status = StatusFree
				return err
			}
		}
		s.status = StatusConnected
		fallthrough
	case StatusConnected:
		return nil
	}
	return nil
}

func (s *StreamSocks) ReadMux() (message.Message, error) {
	return nil, ErrNotSupported
}

func (s *StreamSocks) WriteMux(msg message.Message) error {
	return ErrNotSupported
}
