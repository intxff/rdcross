package shadowsocks

import (
	"crypto/rand"
	"errors"
	"io"
	"net"

	"github.com/intxff/rdcross/component/message"
	"github.com/intxff/rdcross/component/proxy"
	"github.com/shadowsocks/go-shadowsocks2/socks"
)

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

type PacketShadowsocks struct {
	Shadowsocks
	net.PacketConn
	rBuffer []byte
	wBuffer []byte
}

func newPacketShadowsocks(s Shadowsocks, c net.PacketConn) *PacketShadowsocks {
	return &PacketShadowsocks{
		Shadowsocks: s,
		PacketConn:  c,
		rBuffer:     make([]byte, _PayloadMaxSize+40),
		wBuffer:     make([]byte, _PayloadMaxSize+40),
	}
}

func (p *PacketShadowsocks) Metadata() *message.Metadata {
	return nil
}

func (p *PacketShadowsocks) ReadMsgFrom() (message.Message, net.Addr, error) {
	m := &udpMsg{metadata: message.NewMetadata()}
	// get salt
	saltSize := len(p.key)
	n, rAddr, err := p.ReadFrom(p.rBuffer)
	if err != nil {
		return nil, rAddr, err
	}
    if n < saltSize {
        return nil, rAddr, errors.New("too short packet")
    }
	salt := p.rBuffer[:saltSize]
	// decrypt
	decrypter, err := p.Cipher().Decrypter(salt)
	if err != nil {
		return nil, rAddr, err
	}
	planetext := make([]byte, n-saltSize)
	planetext, err = decrypter.Decrypt(p.rBuffer[saltSize:n], "packet", planetext)
	if err != nil {
		return nil, rAddr, err
	}
	// get remote address and update metadata
	var (
		ip     net.IP
		port   int
		domain string
	)
	switch planetext[0] {
	// ipv4
	case 0x01:
		ip = net.IP(planetext[1:5])
		port = int(planetext[5])<<8 + int(planetext[6])
		m.payload = planetext[7:]
		m.metadata.WithRemoteIP(ip).
			WithRemotePort(port)
	// ipv6
	case 0x04:
		ip = net.IP(planetext[1:17])
		port = int(planetext[17])<<8 + int(planetext[18])
		m.payload = planetext[19:]
		m.metadata.WithRemoteIP(ip).
			WithRemotePort(port)
	// domain
	case 0x03:
		l := planetext[1]
		domain = string(planetext[2 : 2+l])
		port = int(planetext[2+l])<<8 + int(planetext[3+l])
		m.payload = planetext[4+l:]
		m.metadata.WithDomain(domain).
			WithRemotePort(port)
	}
	if p.mode == proxy.ModeServer {
		m.metadata.WithClientIP(rAddr.(*net.UDPAddr).IP).
			WithClientPort(rAddr.(*net.UDPAddr).Port)
	}

	return m, rAddr, nil
}

func (p *PacketShadowsocks) WriteMsgTo(msg message.Message, addr net.Addr) error {
	// randomize salt
	saltSize := len(p.key)
	if _, err := io.ReadFull(rand.Reader, p.wBuffer[:saltSize]); err != nil {
		return err
	}
	// encrypter
	encrypter, err := p.cipher.Encrypter(p.wBuffer[:saltSize])
	if err != nil {
		return err
	}
	// construct payload
	m := msg.Metadata()
	port := []byte{byte(m.RemotePort >> 8), byte(m.RemotePort)}
	payloadStart := 0
	if m.Domain != "" {
		p.wBuffer[saltSize] = socks.AtypDomainName
		p.wBuffer[saltSize+1] = byte(len(m.Domain))
		n := copy(p.wBuffer[saltSize+2:saltSize+2+len(m.Domain)], []byte(m.Domain))
		copy(p.wBuffer[saltSize+2+n:saltSize+2+n+2], port)
		payloadStart = saltSize + 2 + n + 2
	} else {
		if m.RemoteIP.To4() != nil {
			p.wBuffer[saltSize] = socks.AtypIPv4
			copy(p.wBuffer[saltSize+1:saltSize+5], m.RemoteIP.To4())
			copy(p.wBuffer[saltSize+5:saltSize+7], port)
			payloadStart = saltSize + 7
		} else {
			p.wBuffer[saltSize] = socks.AtypIPv6
			copy(p.wBuffer[saltSize+1:saltSize+17], m.RemoteIP.To16())
			copy(p.wBuffer[saltSize+17:saltSize+19], port)
			payloadStart = saltSize + 19
		}
	}
    n := copy(p.wBuffer[payloadStart:], msg.Payload())
	// encrypt payload
    ciphtext := encrypter.Encrypt(p.wBuffer[saltSize:payloadStart+n], "packet", p.wBuffer[saltSize:])
	_, err = p.WriteTo(p.wBuffer[:saltSize+len(ciphtext)], addr)
	if err != nil {
		return err
	}
	return nil
}
