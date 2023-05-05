package shadowsocks

import (
	"crypto/rand"
	"io"
	"net"

	"github.com/intxff/rdcross/component/message"
	"github.com/intxff/rdcross/component/proxy"
	"github.com/intxff/rdcross/component/proxy/socks"
)

type StreamShadowsocks struct {
	Shadowsocks
	net.Conn
	encrypter *aeadEncryter
	decrypter *aeadDecryter
	metadata  *message.Metadata
	rBuffer   []byte
	wBuffer   []byte
	// left planetext in rBuffer[lLeft, rLeft)
	rLeft int
	lLeft int
}

func newStreamShadowsocks(p Shadowsocks, c net.Conn, m *message.Metadata) (*StreamShadowsocks, error) {
	// handshake firstly
	sss := &StreamShadowsocks{
		Shadowsocks: p,
		Conn:        c,
		encrypter:   nil,
		decrypter:   nil,
		rLeft:       0,
		lLeft:       0,
		metadata:    m,
	}
	if err := sss.handshake(); err != nil {
		return nil, err
	}
	return sss, nil
}

func (s *StreamShadowsocks) handshake() error {
	if s.mode == proxy.ModeServer {
		return s.serverHandshake()
	}
	return s.clientHandShake()
}

func (s *StreamShadowsocks) serverHandshake() error {
	keySize := len(s.key)
	salt := make([]byte, keySize)

	// get client's salt
	if _, err := s.Conn.Read(salt); err != nil {
		return err
	}
	decrypter, err := s.Cipher().Decrypter(salt)
	if err != nil {
		return err
	}
	s.decrypter = decrypter.(*aeadDecryter)
	l := 2 + s.decrypter.Overhead() + _PayloadMaxSize + s.decrypter.Overhead()
	s.rBuffer = make([]byte, l)
	s.wBuffer = make([]byte, l)

	// get client's metadata
	client := s.Conn.RemoteAddr().(*net.TCPAddr)
	m := message.NewMetadata().WithClientIP(client.IP).
		WithClientPort(client.Port)
	buf := make([]byte, 260)
	n, err := s.Read(buf)
	if err != nil {
		return err
	}
	switch atyp := buf[0]; atyp {
	case socks.AtypDomain:
		lDomain := int(buf[1])
		domain := buf[2 : 2+lDomain]
		m.Domain = string(domain)
	case socks.AtypIPv4:
		ipv4 := buf[1:5]
		m.RemoteIP = ipv4
	case socks.AtypIPv6:
		ipv6 := buf[1:17]
		m.RemoteIP = ipv6
	}
	port := buf[n-2 : n]
	m.RemotePort = int(port[0])<<8 + int(port[1])
	s.metadata = m

	// send server's salt
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return err
	}
	if _, err := s.Conn.Write(salt); err != nil {
		return err
	}

	encrypter, err := s.Cipher().Encrypter(salt)
	if err != nil {
		return err
	}
	s.encrypter = encrypter.(*aeadEncryter)

	return nil
}

func (s *StreamShadowsocks) clientHandShake() error {
	salt := make([]byte, len(s.key))

	// send client's salt
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return err
	}
	if _, err := s.Conn.Write(salt); err != nil {
		return err
	}
	encrypter, err := s.Cipher().Encrypter(salt)
	if err != nil {
		return err
	}
	s.encrypter = encrypter.(*aeadEncryter)
	l := 2 + s.encrypter.Overhead() + _PayloadMaxSize + s.encrypter.Overhead()
	s.rBuffer = make([]byte, l)
	s.wBuffer = make([]byte, l)

	// send metadata
	buf := make([]byte, 260)
	n := 0
	if s.metadata.Domain != "" {
		lDomain := byte(len(s.metadata.Domain) & 0xff)
		buf[0] = socks.AtypDomain
		buf[1] = lDomain
		n = copy(buf[2:], []byte(s.metadata.Domain))
		n += 2
		goto port
	}
	if ipv4 := s.metadata.RemoteIP.To4(); ipv4 != nil {
		buf[0] = socks.AtypIPv4
		n = copy(buf[1:], ipv4)
		n++
		goto port
	}
	if ipv6 := s.metadata.RemoteIP.To16(); ipv6 != nil {
		buf[0] = socks.AtypIPv6
		n = copy(buf[1:], ipv6)
		n++
		goto port
	}
port:
	p := s.metadata.RemotePort & 0xffff
	buf[n], buf[n+1] = byte(p>>8), byte(p)
	if _, err := s.Write(buf[:n+2]); err != nil {
		return err
	}

	return nil
}

func (s *StreamShadowsocks) ReadMux() (message.Message, error) {
	return nil, proxy.ErrNotSupported
}

func (s *StreamShadowsocks) WriteMux(msg message.Message) error {
	return proxy.ErrNotSupported
}

func (s *StreamShadowsocks) Read(b []byte) (int, error) {
	// check whether can decrypt
	if s.decrypter == nil {
		// get server's salt
		keySize := len(s.key)
		if _, err := s.Conn.Read(s.rBuffer[:keySize]); err != nil {
			return 0, err
		}
		salt := s.rBuffer[:keySize]
		decrypter, err := s.Cipher().Decrypter(salt)
		if err != nil {
			return 0, err
		}
		s.decrypter = decrypter.(*aeadDecryter)
	}

	var n int
	buf := s.rBuffer
	lenB := len(b)
	// copy left planetext to b
	n = copy(b, buf[s.lLeft:s.rLeft])
	if n == lenB {
		s.lLeft += n
		return n, nil
	}

	// read extra ciphertext to fill in b
	s.lLeft, s.rLeft = 0, 0
	lenSize := 2 + s.decrypter.Overhead()
	// decrypt payload length
	if _, err := io.ReadFull(s.Conn, buf[:lenSize]); err != nil {
		return n, err
	}
	if _, err := s.decrypter.Decrypt(buf[:lenSize]); err != nil {
		return n, err
	}
	l := (int(buf[0])<<8 + int(buf[1])) & _PayloadMaxSize
	// decrypt payload
	if _, err := io.ReadFull(s.Conn, buf[:l+s.decrypter.Overhead()]); err != nil {
		return n, err
	}
	if _, err := s.decrypter.Decrypt(buf[:l+s.decrypter.Overhead()]); err != nil {
		return n, err
	}
	nExtra := copy(b[n:], buf[:l])
	n += nExtra
	s.lLeft, s.rLeft = nExtra, l
	return n, nil
}

func (s *StreamShadowsocks) Write(b []byte) (int, error) {
    l := len(b)
	buf := s.wBuffer
	lenSize, tagSize := 2, s.encrypter.Overhead()
	n := 0
	for n < l {
        nc := copy(buf[(lenSize+tagSize):(len(buf)-tagSize)], b[n:])
		n += nc

		// encrypt payload length
		buf[0], buf[1] = byte(nc>>8), byte(nc)
		s.encrypter.Encrypt(buf[:2])
		// encrypt payload
		s.encrypter.Encrypt(buf[(lenSize + tagSize):(lenSize + tagSize + nc)])

		if _, err := s.Conn.Write(buf[:lenSize+tagSize+nc+tagSize]); err != nil {
			return n, err
		}
	}
	return n, nil
}

func (s *StreamShadowsocks) Metadata() *message.Metadata {
	return s.metadata
}
