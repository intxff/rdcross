package general

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/intxff/rdcross/component/conn"
	"github.com/intxff/rdcross/component/nat"
	"github.com/intxff/rdcross/component/proxy"
	"github.com/intxff/rdcross/component/proxy/none"
	"github.com/intxff/rdcross/component/proxy/shadowsocks"
	"github.com/intxff/rdcross/component/proxy/socks"
	"github.com/intxff/rdcross/component/transport"
	"github.com/intxff/rdcross/ingress"
	"github.com/intxff/rdcross/log"
	"github.com/intxff/rdcross/router"
	"github.com/intxff/rdcross/util"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

type General struct {
	name   string
	trans  []transport.Transport
	proxy  proxy.Proxy
	status atomic.Int32
	conns  sync.Map
}

func NewGeneral(name string, p proxy.Proxy, t ...transport.Transport) *General {
	g := &General{
		name:  name,
		proxy: p,
		trans: t,
		conns: sync.Map{},
	}
	g.status.Store(ingress.Ready)
	return g
}

func (g *General) logString(s string) string {
	return fmt.Sprintf("[Ingress] %v: %v", g.name, s)
}

func (g *General) Type() ingress.IngressType {
	return ingress.TypeGeneral
}

func (g *General) Name() string {
	return g.name
}

func (g *General) Proxy() proxy.Proxy {
	return g.proxy
}

func (g *General) Transport() []transport.Transport {
	return g.trans
}

func (g *General) Close() <-chan struct{} {
	defer func() {
		log.Info(g.logString("closed"))
	}()
	ch := make(chan struct{}, 1)
	if g.status.Load() == ingress.Ready || g.status.Load() == ingress.Closed {
		ch <- struct{}{}
		return ch
	}

	g.conns.Range(func(_, value any) bool {
		value.(io.Closer).Close()
		return true
	})

	g.status.Store(ingress.Closed)

	ch <- struct{}{}
	return ch
}

func (g *General) Run(r router.Router) {
	log.Info(g.logString("starting..."))
	g.status.Store(ingress.Running)
	for _, t := range g.trans {
		switch t.Type() {
		case transport.TypeStream:
			go g.handleStream(t, r)
		case transport.TypePacket:
			go g.handlePacket(t, r)
		}
	}
}

func (g *General) handleStream(t transport.Transport, r router.Router) {
	l, err := t.ListenStream()
	if err != nil {
		log.Error(g.logString("failed to start transport"), zap.Error(err))
	}
	defer l.Close()
	log.Info(g.logString("tcp listening"),
		zap.String("Addr", l.Addr().String()))

	for {
		c, err := l.Accept()
		if err != nil {
			log.Error(g.logString("wrong connection"), zap.Error(err))
			return
		}
		log.Info(g.logString("connection accepted"),
			zap.String("remote", c.RemoteAddr().String()),
			zap.String("local", c.LocalAddr().String()))

		if g.status.Load() == ingress.Closed {
			c.Close()
			return
		}

		// handle every connection
		go func() {
			sc, err := g.proxy.ShadowStreamConn(c, g.Name())
			if err != nil {
				log.Error(g.logString("failded to shadow connection"),
					zap.Error(err))
				return
			}
			log.Info(g.logString("connection shadowed"),
				zap.String("remote", sc.RemoteAddr().String()),
				zap.String("local", sc.LocalAddr().String()))

			g.conns.Store(c.LocalAddr().String(), sc)

			defer func() {
				g.conns.Delete(sc.LocalAddr().String())
				sc.Close()
				log.Info(g.logString("connection closed"),
					zap.String("remote", sc.RemoteAddr().String()),
					zap.String("local", sc.LocalAddr().String()))
			}()
			if g.proxy.TcpMux() {
				/* for {
					msg, err := sc.ReadMux()
					msg.Metadata().WithIngress(g.Name())
					if err != nil {
						if err != io.EOF {
							continue
						}
						log.Error("", zap.Error(err))
					}
					out := r.Dispatch(*msg.Metadata())
					go out.ProcessStream(sc, msg)
				} */
			} else {
				sc.Metadata().WithIngress(g.Name())
				out := r.Dispatch(*(sc.Metadata()))
				log.Info(g.logString("connection dispatched"),
					zap.String("egress", out.Name()))
				out.ProcessStream(sc, nil)
			}
		}()
	}
}

func (g *General) handlePacket(t transport.Transport, r router.Router) {
	// nat
	nat := nat.New()

	// get connection
	c, err := t.ListenPacket()
	if err != nil {
		log.Error(g.logString("wrong connection"), zap.Error(err))
		return
	}
	log.Info(g.logString("udp listening"),
		zap.String("Addr", c.LocalAddr().String()))

	// proxy connection
	sc, err := g.proxy.ShadowPacketConn(c, g.Name())
	if err != nil {
		log.Error(g.logString("failded to shadow connection"),
			zap.Error(err))
		return
	}
	log.Info(g.logString("connection shadowed"),
		zap.String("local", sc.LocalAddr().String()))
	g.conns.Store(sc.LocalAddr().String(), sc)

	defer func() {
		g.conns.Delete(sc.LocalAddr().String())
		sc.Close()
	}()

	// handle connection
	for {
		// try to close connection
		if g.status.Load() == ingress.Closed {
			return
		}

		// read msg from client
		msg, cAddr, err := sc.ReadMsgFrom()
		if err != nil {
			log.Error(g.logString("failed to read"),
				zap.String("remote", cAddr.String()),
				zap.String("local", sc.LocalAddr().String()))
			continue
		}

		msg.Metadata().WithIngress(g.Name())

		// check whether exist in nat
		if lc, exist := nat.Get(cAddr.String()); exist {
			rc, rAddr := lc.PacketConn.(conn.ProxyPacketConn), lc.Addr
			rc.WriteMsgTo(msg, rAddr)
			continue
		}
		out := r.Dispatch(*msg.Metadata())
		log.Info(g.logString("connection dispatched"),
			zap.String("egress", out.Name()))
		out.ProcessPacket(sc, msg)
	}
}

func (g *General) UnmarshalYAML(value *yaml.Node) error {
	var (
		name  string
		p     proxy.Proxy
		trans = make([]transport.Transport, 0)
		err   error
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
			for _, elem := range v.Content {
				tran, err := unmarshalTran(elem)
				if err != nil {
					return err
				}
				trans = append(trans, tran)
			}
		}

	}
	g.name = name
	g.proxy = p
	g.trans = trans
	g.status.Store(ingress.Ready)
	g.conns = sync.Map{}
	return nil
}

func unmarshalTran(value *yaml.Node) (transport.Transport, error) {
	var (
		tran    transport.Transport
		tType   transport.TransType
		ip      string
		port    int
		smux    bool
		faketcp bool
		err     error
	)

	t := make(map[string]interface{})
	if err = value.Decode(&t); err != nil {
		return nil, err
	}

	attrMust := map[string]any{
		"type": &tType,
		"ip":   &ip,
		"port": &port,
	}
	if err = util.MustHave(t, attrMust); err != nil {
		return nil, err
	}

	addr := fmt.Sprintf("%v:%v", ip, port)

	switch transport.TransType(strings.ToUpper(string(tType))) {
	case transport.TypeStream:
		attrMay := map[string]any{
			"smux": &smux,
		}
		if err = util.MayHave(t, attrMay); err != nil {
			return nil, err
		}
		if tran, err = transport.NewTransTCP("tcp", addr, transport.WithSmux(smux)); err != nil {
			return nil, err
		}
	case transport.TypePacket:
		attrMay := map[string]any{
			"faketcp": &faketcp,
		}
		if err = util.MayHave(t, attrMay); err != nil {
			return nil, err
		}

		if tran, err = transport.NewTransUDP("udp", addr, transport.WithFakeTCP(faketcp)); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("invalid transport type %v", tType)
	}

	return tran, nil
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
		p = socks.NewProxySocks(proxy.ModeServer)
	case proxy.TypeNone:
		p = none.NewProxyNone(proxy.ModeServer)
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

		p, err = shadowsocks.NewProxyShadowsocks(proxy.ModeServer, password, cipher, "", udp)
		if err != nil {
			return nil, err
		}
	}
	return p, nil
}
