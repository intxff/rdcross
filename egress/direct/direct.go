package direct

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/intxff/rdcross/component/conn"
	"github.com/intxff/rdcross/component/iface"
	"github.com/intxff/rdcross/component/message"
	"github.com/intxff/rdcross/component/nat"
	"github.com/intxff/rdcross/component/proxy"
	"github.com/intxff/rdcross/component/transport"
	"github.com/intxff/rdcross/dns"
	"github.com/intxff/rdcross/egress"
	"github.com/intxff/rdcross/log"
	"go.uber.org/zap"
)

type directMsg struct {
	payload  []byte
	metadata *message.Metadata
}

func (d *directMsg) Metadata() *message.Metadata {
	return d.metadata
}

func (d *directMsg) Others() any {
	return nil
}

func (d *directMsg) Payload() []byte {
	return d.payload
}

type Direct struct {
	conns sync.Map
}

func NewDirect() *Direct {
	return &Direct{
		conns: sync.Map{},
	}
}

func (d *Direct) Type() egress.EgressType {
	return egress.TypeDirect
}

func (d *Direct) Name() string {
	return string(egress.TypeDirect)
}

func (d *Direct) Proxy() proxy.Proxy {
	return nil
}

func (d *Direct) Transport() (stream, packet transport.Transport) {
	return nil, nil
}

func (d *Direct) Close() <-chan struct{} {
	ch := make(chan struct{}, 1)
	ch <- struct{}{}
    d.conns.Range(func(_, value any) bool {
		value.(io.Closer).Close()
		return true
    })
	return ch
}

func (d *Direct) logString(s string) string {
	return fmt.Sprintf("[Direct]: %v", s)
}

func (d *Direct) ProcessStream(c conn.ProxyStreamConn, msg message.Message) {
	if msg != nil {
		d.processMsgStream(c, msg)
	} else {
		d.processStream(c)
	}
}

func (d *Direct) processStream(c conn.ProxyStreamConn) {
	// dial remote to get remote connection
	rAddr := &net.TCPAddr{Port: c.Metadata().RemotePort}
	m := c.Metadata()
	if m.RemoteIP != nil {
		rAddr.IP = m.RemoteIP
	}
	if m.Domain != "" {
		ips, err := dns.ResolveIPv4(m.Domain)
		if err != nil {
			log.Error(d.logString("failed to resolve domain"),
				zap.Error(err))
			return
		}
		rAddr.IP = ips[0]
	}
    lIP, err := iface.GetIPv4()
    if err != nil {
        return
    }
	rc, err := net.DialTCP("tcp", &net.TCPAddr{IP: lIP, Port: 0}, rAddr)
	if err != nil {
		log.Error("failed to dial remote",
			zap.Error(err))
		return
	}
	d.conns.Store(rc.LocalAddr().String(), rc)

	defer func() {
		d.conns.Delete(rc.LocalAddr().String())
		rc.Close()
		log.Info("connection closed",
			zap.String("client", c.LocalAddr().String()),
			zap.String("remote", rc.LocalAddr().String()))
	}()

	ch := make(chan struct{})
	var errClient, errRemote error
	go func() {
		// if src closes firstly, errRemote will be nil
		// if c closes firstly, errRemote = timeout
		_, errRemote = io.Copy(c, rc)
		c.SetReadDeadline(time.Now().Add(time.Second * 3))
		ch <- struct{}{}
	}()
	_, errClient = io.Copy(rc, c)
	rc.SetReadDeadline(time.Now().Add(time.Second * 3))
	<-ch
	if errors.Is(errRemote, syscall.ECONNRESET) || errors.Is(errClient, syscall.EPIPE) {
		log.Info(d.logString("remote closed"), zap.Error(errRemote))
		return
	}
	if errors.Is(errClient, syscall.ECONNRESET) || errors.Is(errRemote, syscall.EPIPE) {
		log.Info(d.logString("client closed"), zap.Error(errClient))
		return
	}
	if errRemote != nil && !errors.Is(errRemote, os.ErrDeadlineExceeded) {
		log.Error(d.logString("unexpected remote error"), zap.Error(errRemote))
		return
	}
	if errClient != nil && !errors.Is(errClient, os.ErrDeadlineExceeded) {
		log.Error(d.logString("unexpected client error"), zap.Error(errClient))
		return
	}
}

func (d *Direct) processMsgStream(c conn.ProxyStreamConn, msg message.Message) {
	/* // dial remote to get remote connection
	remoteStream, _ := g.Transport()
	rc, err := remoteStream.DialStream()
	if err != nil {
		log.Error("failed to dial remote", zap.Error(err))
	}
	defer rc.Close()

	// send msg to remote, only do once
	_, err = rc.Write(msg.Payload())
	if err != nil {
		log.Error("Error when writing message to remote", zap.Error(err))
		return
	}

	// take response from remote then send to client
	_, err = io.Copy(c, rc)
	if err != nil {
		if err != io.EOF {
			log.Error("Error when copying remote response to client", zap.Error(err))
			return
		}
	} */
}

func (d *Direct) ProcessPacket(c conn.ProxyPacketConn, msg message.Message) {
	buf := make([]byte, 10*1024)
	m := msg.Metadata()
	newMsg := &directMsg{metadata: m, payload: nil}
	// gnat
	gnat := nat.New()

	// get target address
	rAddr := &net.UDPAddr{Port: m.RemotePort}
	if m.RemoteIP != nil {
		rAddr.IP = m.RemoteIP
	}
	if m.Domain != "" {
		ips, err := dns.ResolveIPv4(m.Domain)
		if err != nil {
			log.Error(d.logString("failed to resolve domain"),
				zap.Error(err))
			return
		}
		rAddr.IP = ips[0]
	}

	// listener to do nat map
    lIP, err := iface.GetIPv4()
    if err != nil {
        return
    }
	l, err := net.ListenUDP("udp", &net.UDPAddr{IP: lIP, Port: 0})
	if err != nil {
		log.Error(d.logString("failed to listen udp"), zap.Error(err))
		return
	}

	// add to nat then copy remote response to client connection
	cAddr := net.UDPAddr{IP: msg.Metadata().ClientIP, Port: msg.Metadata().ClientPort}
	gnat.Set(cAddr.String(), nat.LinkPacketConn{PacketConn: l, Addr: l.LocalAddr()})

	go func() {
		defer func() {
			gnat.Delete(cAddr.String())
			log.Info(d.logString("udp nat closed"),
				zap.String("client", cAddr.String()),
				zap.String("remote", rAddr.String()))
		}()

		// send data to remote first
		_, err = l.WriteTo(msg.Payload(), rAddr)
		if err != nil {
			log.Error(d.logString("failed to write"), zap.Error(err))
			return
		}

		for {
			// update deadline
			l.SetReadDeadline(time.Now().Add(time.Minute * 5))

			n, _, err := l.ReadFrom(buf)
			if err != nil {
				log.Error(d.logString("failed to read"), zap.Error(err))
				return
			}
			newMsg.payload = buf[:n]
			err = c.WriteMsgTo(newMsg, &cAddr)
			if err != nil {
				log.Error(d.logString("failed to write"), zap.Error(err))
				return
			}
		}
	}()
}
