package general

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/intxff/rdcross/component/conn"
	"github.com/intxff/rdcross/component/iface"
	"github.com/intxff/rdcross/component/message"
	"github.com/intxff/rdcross/component/nat"
	"github.com/intxff/rdcross/component/proxy"
	"github.com/intxff/rdcross/component/proxy/none"
	"github.com/intxff/rdcross/component/proxy/shadowsocks"
	"github.com/intxff/rdcross/component/proxy/socks"
	"github.com/intxff/rdcross/component/transport"
	"github.com/intxff/rdcross/egress"
	"github.com/intxff/rdcross/log"
	"github.com/intxff/rdcross/util"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

type General struct {
	name       string
	tranStream transport.Transport
	tranPacket transport.Transport
	proxy      proxy.Proxy
	conns      sync.Map
	status     atomic.Int32
}

func NewGeneral(name string, p proxy.Proxy, stream, packet transport.Transport) *General {
	g := &General{
		name:       name,
		proxy:      p,
		tranStream: stream,
		tranPacket: packet,
		conns:      sync.Map{},
	}
	g.status.Store(egress.Ready)
	return g
}

func (g *General) Type() egress.EgressType {
	return egress.TypeGeneral
}

func (g *General) Name() string {
	return g.name
}

func (g *General) Proxy() proxy.Proxy {
	return g.proxy
}

func (g *General) Transport() (stream, packet transport.Transport) {
	return g.tranStream, g.tranPacket
}

func (g *General) logString(s string) string {
	return fmt.Sprintf("[Egress] %v: %v", g.name, s)
}

func (g *General) Close() <-chan struct{} {
	defer func() {
		log.Info(g.logString("closed"))
	}()
	ch := make(chan struct{}, 1)
	if g.status.Load() == egress.Ready || g.status.Load() == egress.Closed {
		ch <- struct{}{}
		return ch
	}

	g.status.Store(egress.Closed)
	g.conns.Range(func(_, value any) bool {
		value.(io.Closer).Close()
		return true
	})

	ch <- struct{}{}
	return ch
}

func (g *General) ProcessStream(c conn.ProxyStreamConn, msg message.Message) {
	g.status.Store(egress.Running)
	if msg != nil {
		g.processMsgStream(c, msg)
	} else {
		g.processStream(c)
	}
}

func (g *General) processStream(c conn.ProxyStreamConn) {
	// dial remote to get remote connection
	remoteStream, _ := g.Transport()
	rc, err := remoteStream.DialStream()
	if err != nil {
		log.Error(g.logString("failed to dial remote"),
			zap.Error(err))
		return
	}

	remoteAddr := rc.RemoteAddr().String()
	localAddr := rc.LocalAddr().String()

	g.conns.Store(rc.LocalAddr().String(), rc)
	log.Info(g.logString("connected to remote"),
		zap.String("local", localAddr),
		zap.String("remote", remoteAddr))

	defer func() {
		g.conns.Delete(rc.LocalAddr().String())
		rc.Close()
		log.Info(g.logString("connect closed"),
			zap.String("local", localAddr),
			zap.String("remote", remoteAddr))
	}()

	src, err := g.proxy.ShadowStreamConn(rc, c.Metadata())
	if err != nil {
		log.Error(g.logString("failed to shadow connection"),
			zap.Error(err),
			zap.String("local", localAddr),
			zap.String("remote", remoteAddr))
		return
	}

	var errClient, errRemote error
	ch := make(chan struct{}, 1)
	go func() {
		// if src closes firstly, errRemote will be nil
		// if c closes firstly, errRemote = timeout
		_, errRemote = io.Copy(c, src)
		c.SetReadDeadline(time.Now().Add(time.Second * 5))
		ch <- struct{}{}
	}()
	_, errClient = io.Copy(src, c)
	src.SetReadDeadline(time.Now().Add(time.Second * 5))
	<-ch
	if errors.Is(errRemote, syscall.ECONNRESET) || errors.Is(errClient, syscall.EPIPE) {
		log.Info(g.logString("remote closed"), zap.Error(errRemote))
		return
	}
	if errors.Is(errClient, syscall.ECONNRESET) || errors.Is(errRemote, syscall.EPIPE) {
		log.Info(g.logString("client closed"), zap.Error(errClient))
		return
	}
	if errRemote != nil && !errors.Is(errRemote, os.ErrDeadlineExceeded) {
		log.Error(g.logString("unexpected remote error"), zap.Error(errRemote))
		return
	}
	if errClient != nil && !errors.Is(errClient, os.ErrDeadlineExceeded) {
		log.Error(g.logString("unexpected client error"), zap.Error(errClient))
		return
	}
}

func (g *General) processMsgStream(c conn.ProxyStreamConn, msg message.Message) {
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

func (g *General) ProcessPacket(c conn.ProxyPacketConn, msg message.Message) {
	g.status.Store(egress.Running)
	// gnat
	gnat := nat.New()

	// get target address
	_, remotePacket := g.Transport()
	rAddr := remotePacket.Address()

	// bind to interface avoid route decision
	lAddr := &net.UDPAddr{}
	rIP := rAddr.(*net.UDPAddr).IP
	if rIP.To4() != nil {
		lIP, err := iface.GetIPv4()
		if err != nil {
			return
		}
		lAddr.IP = lIP
	} else {
		lIP, err := iface.GetIPv6()
		if err != nil {
			return
		}
		lAddr.IP = lIP
	}

	// create udp listener to implement fullcone nat
	l, err := net.ListenUDP("udp", lAddr)
	if err != nil {
		log.Error(g.logString("failed to listen udp"),
			zap.Error(err))
		return
	}
	sl, err := g.proxy.ShadowPacketConn(l)
	if err != nil {
		log.Error(g.logString("failed to shadow connection"),
			zap.Error(err))
		return
	}

	// add to nat then copy remote response to client connection
	cAddr := net.UDPAddr{IP: msg.Metadata().ClientIP, Port: msg.Metadata().ClientPort}
	gnat.Set(cAddr.String(), nat.LinkPacketConn{PacketConn: sl, Addr: sl.LocalAddr()})
	log.Info(g.logString("udp nat created"),
		zap.String("client", cAddr.String()),
		zap.String("remote", rAddr.String()))

	go func() {
		defer func() {
			gnat.Delete(cAddr.String())
			log.Info(g.logString("udp nat entry deleted"),
				zap.String("client", cAddr.String()),
				zap.String("remote", rAddr.String()))
		}()

		// send data to remote first
		err = sl.WriteMsgTo(msg, rAddr)
		if err != nil {
			log.Error(g.logString("failed to write"),
				zap.Error(err))
			return
		}

		for {
			// try to close
			if g.status.Load() == egress.Closed {
				sl.Close()
				return
			}

			// update deadline
			sl.SetReadDeadline(time.Now().Add(time.Minute * 5))

			msg, _, err := sl.ReadMsgFrom()
			if err != nil {
				if errors.Is(err, syscall.ECONNRESET) &&
					g.status.Load() == egress.Closed {
					return
				}
				log.Error(g.logString("failed to read"),
					zap.Error(err))
				return
			}
			err = c.WriteMsgTo(msg, &cAddr)
			if err != nil {
				log.Error(g.logString("failed to write"),
					zap.Error(err))
				return
			}
		}
	}()
}

func (g *General) UnmarshalYAML(value *yaml.Node) error {
	var (
		name       string
		p          proxy.Proxy
		tranStream transport.Transport
		tranPacket transport.Transport
		err        error
	)
	for i := 0; i < 4; i++ {
		k := value.Content[2*i]
		v := value.Content[2*i+1]
		switch k.Value {
		case "name":
			name = v.Value
		case "proxy":
			if p, err = unmarshalProxy(v); err != nil {
				return err
			}
		case "transport":
			if tranStream, tranPacket, err = unmarshalTran(v); err != nil {
				return err
			}
		}
	}
	g.name = name
	g.proxy = p
	g.tranStream = tranStream
	g.tranPacket = tranPacket
	g.status.Store(egress.Ready)
	g.conns = sync.Map{}
	return nil
}

func unmarshalTran(value *yaml.Node) (transport.Transport, transport.Transport, error) {
	var (
		tranStream transport.Transport
		tranPacket transport.Transport
		tType      transport.TransType
		ip         string
		port       int
		smux       bool
		faketcp    bool
		err        error
	)

	for _, v := range value.Content {
		t := make(map[string]interface{})
		if err = v.Decode(&t); err != nil {
			return nil, nil, err
		}

		attrMust := map[string]any{
			"type": &tType,
			"ip":   &ip,
			"port": &port,
		}
		if err = util.MustHave(t, attrMust); err != nil {
			return nil, nil, err
		}

		addr := fmt.Sprintf("%v:%v", ip, port)

		switch transport.TransType(strings.ToUpper(string(tType))) {
		case transport.TypeStream:
			attrMay := map[string]any{
				"smux": &smux,
			}
			if err = util.MayHave(t, attrMay); err != nil {
				return nil, nil, err
			}
			if tranStream, err = transport.NewTransTCP("tcp", addr, transport.WithSmux(smux)); err != nil {
				return nil, nil, err
			}
		case transport.TypePacket:
			attrMay := map[string]any{
				"faketcp": &faketcp,
			}
			if err = util.MayHave(t, attrMay); err != nil {
				return nil, nil, err
			}

			if tranPacket, err = transport.NewTransUDP("udp", addr, transport.WithFakeTCP(faketcp)); err != nil {
				return nil, nil, err
			}
		default:
			return nil, nil, fmt.Errorf("invalid transport type %v", tType)
		}
	}

	return tranStream, tranPacket, nil
}

func unmarshalProxy(value *yaml.Node) (proxy.Proxy, error) {
	var (
		p     proxy.Proxy
		pType proxy.ProxyType
		err   error
	)

	t := make(map[string]interface{})
	if err = value.Decode(&t); err != nil {
		return nil, err
	}

	attrMust := map[string]any{
		"type": &pType,
	}
	if err = util.MustHave(t, attrMust); err != nil {
		return nil, err
	}

	switch proxy.ProxyType(strings.ToUpper(string(pType))) {
	case proxy.TypeSocks:
		p = socks.NewProxySocks(proxy.ModeClient)
	case proxy.TypeNone:
		p = none.NewProxyNone(proxy.ModeClient)
	case proxy.TypeShadowsocks:
		var (
			password string
			cipher   string
			attrMust = map[string]any{
				"password": &password,
				"cipher":   &cipher,
			}
		)
		if err = util.MustHave(t, attrMust); err != nil {
			return nil, err
		}

		var (
			udp     bool
			key     string
			attrMay = map[string]any{
				"udp": &udp,
				"key": &key,
			}
		)
		if err = util.MayHave(t, attrMay); err != nil {
			return nil, err
		}

		p, err = shadowsocks.NewProxyShadowsocks(proxy.ModeClient, password, cipher, "", udp)
		if err != nil {
			return nil, err
		}
	}
	return p, nil
}
