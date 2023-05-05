package tun

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/intxff/rdcross/component/conn"
	"github.com/intxff/rdcross/component/fakeip"
	"github.com/intxff/rdcross/component/iface"
	"github.com/intxff/rdcross/component/message"
	"github.com/intxff/rdcross/component/nat"
	"github.com/intxff/rdcross/component/proxy"
	"github.com/intxff/rdcross/component/transport"
	"github.com/intxff/rdcross/dns"
	"github.com/intxff/rdcross/ingress"
	"github.com/intxff/rdcross/log"
	"github.com/intxff/rdcross/router"
	"github.com/intxff/rdcross/util"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"
	"gopkg.in/yaml.v3"
)

var _ ingress.Ingress = (*Tun)(nil)

const (
	relayPort int = 8888
)

var errNotSupported = errors.New("not supported")

type Tun struct {
	name      string
	mtu       int
	cidr      *net.IPNet
	fd        *os.File
	ip        net.IP
	relayIP   net.IP
	relayPort int
	status    atomic.Int32
	conns     sync.Map
	udpNat    sync.Map
	tcpNat    sync.Map
	ipPool    *fakeip.FakeIP
	hijack    []hijackEntry
	dnsQuery  chan message.Message
	dnsAddr   net.Addr
}

type natEntry struct {
	from net.Addr
	to   net.Addr
}

type hijackEntry struct {
	ip   net.IP
	port int
}

func newEntry(s string) (*hijackEntry, error) {
	var (
		ip   net.IP
		port int
	)
	l := strings.Split(s, ":")
	if len(l) == 0 {
		return nil, errors.New("empty entry")
	}
	if l[0] == "" {
		ip = net.ParseIP("0.0.0.0")
	} else {
		ip = net.ParseIP(l[0])
		if ip == nil {
			return nil, fmt.Errorf("invalid ip %s", l[0])
		}
	}
	if l[1] == "" {
		return nil, fmt.Errorf("no port in entry %s", s)
	}
	port, err := strconv.Atoi(l[1])
	if err != nil {
		return nil, err
	}
	return &hijackEntry{ip: ip, port: port}, nil
}

func (h *hijackEntry) String() string {
	return fmt.Sprintf("%v:%v", h.ip, h.port)
}

func NewTun(name, iprange string, mtu, port int, hijack []string, tdns string) (*Tun, error) {
	t := &Tun{}
	err := initTun(t, name, iprange, mtu, port, hijack)
	if err != nil {
		return nil, err
	}
	err = t.WithDNS(tdns)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (t *Tun) WithDNS(s string) error {
	dnsAddr, err := net.ResolveUDPAddr("udp", s)
	if err != nil {
		return err
	}
	t.dnsAddr = dnsAddr
	return nil
}

func initTun(t *Tun, name, iprange string, mtu, port int, hijack []string) error {
	ip, cidr, err := net.ParseCIDR(iprange)
	if err != nil {
		return err
	}

	hijacklist := make([]hijackEntry, len(hijack))
	if len(hijack) == 0 {
		hijack = []string{"0.0.0.0:53"}
	}
	for k, v := range hijack {
		e, err := newEntry(v)
		if err != nil {
			return err
		}
		hijacklist[k] = *e
	}

	relayIP := ip

	ipPool, err := fakeip.New(iprange, 10000)
	if err != nil {
		return err
	}

	t.name = name
	t.mtu = mtu
	t.cidr = cidr
	t.ip = ip
	t.relayPort = port
	t.relayIP = relayIP
	t.ipPool = ipPool
	t.conns = sync.Map{}
	t.udpNat = sync.Map{}
	t.tcpNat = sync.Map{}
	t.status.Store(ingress.Ready)
	t.hijack = hijacklist
	t.dnsQuery = make(chan message.Message, 200)

	return nil
}

func (t *Tun) logString(s string) string {
	return fmt.Sprintf("[Tun] %v: %v", t.name, s)
}

func (t *Tun) Name() string {
	return t.name
}

func (t *Tun) Type() ingress.IngressType {
	return ingress.TypeTun
}

func (t *Tun) Proxy() proxy.Proxy {
	return nil
}

func (t *Tun) Transport() []transport.Transport {
	return nil
}

var wild = net.ParseIP("0.0.0.0")

func (t *Tun) NeedHijack(ip net.IP, port int) bool {
	for _, v := range t.hijack {
		if (v.ip.Equal(ip) || v.ip.Equal(wild)) && v.port == port {
			return true
		}
	}
	return false
}

func open(name string) (*os.File, error) {
	fd, err := unix.Open("/dev/net/tun", unix.O_RDWR, 0)
	if err != nil {
		return nil, err
	}

	var ifr struct {
		name  [16]byte
		flags uint16
		_     [22]byte
	}

	copy(ifr.name[:], name)
	ifr.flags = unix.IFF_TUN | unix.IFF_NO_PI
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), unix.TUNSETIFF,
		uintptr(unsafe.Pointer(&ifr)))
	if errno != 0 {
		unix.Close(fd)
		return nil, errno
	}

	if err = unix.SetNonblock(fd, true); err != nil {
		unix.Close(fd)
		return nil, err
	}

	return os.NewFile(uintptr(fd), name), nil
}

func (t *Tun) setup() error {
	ones, _ := t.cidr.Mask.Size()
	addr := t.ip.String() + "/" + strconv.Itoa(ones)
	cmd := []string{
		fmt.Sprintf("ip addr add %v dev %v", addr, t.Name()),
		fmt.Sprintf("ip link set dev %v mtu %v", t.Name(), t.mtu),
		fmt.Sprintf("ip link set dev %v up", t.Name()),
		fmt.Sprintf("ip route add default via %v table 100000", t.ip),
		fmt.Sprintf("ip rule add to %v table 100000 preference 10000", t.cidr.String()),
		"ip rule add ipproto icmp goto 10060 preference 10010",
		"ip rule add not dport 53 table main suppress_prefixlength 0 preference 10020",
		"ip rule add not iif lo table 100000 preference 10030",
		"ip rule add from 0.0.0.0 iif lo uidrange 0-4294967294 table 100000 preference 10040",
		fmt.Sprintf("ip rule add from %v iif lo uidrange 0-4294967294 table 100000 preference 10050", t.ip),
		"ip rule add from all nop preference 10060",
	}
	for _, v := range cmd {
		if _, err := util.ExecCmd(v); err != nil {
			t.clean()
			return err
		}
	}
	return nil
}

func (t *Tun) clean() {
	cmd := []string{
		"ip route delete default table 100000",
		"ip rule delete preference 10000",
		"ip rule delete preference 10010",
		"ip rule delete preference 10020",
		"ip rule delete preference 10030",
		"ip rule delete preference 10040",
		"ip rule delete preference 10050",
		"ip rule delete preference 10060",
	}
	for _, v := range cmd {
		util.ExecCmd(v)
	}

	t.fd.Close()
}

func (t *Tun) Close() <-chan struct{} {
	defer func() {
		log.Info(t.logString("closed"))
	}()
	ch := make(chan struct{}, 1)
	if t.status.Load() == ingress.Closed || t.status.Load() == ingress.Ready {
		ch <- struct{}{}
		return ch
	}

	t.status.Store(ingress.Closed)

	t.clean()

	t.conns.Range(func(_, value any) bool {
		value.(io.Closer).Close()
		return true
	})

	ch <- struct{}{}
	return ch
}

func (t *Tun) Run(r router.Router) {
	log.Info(t.logString("starting..."))
	t.status.Store(ingress.Running)

	fd, err := open(t.name)
	if err != nil {
		log.Error(t.logString("failed to open device"),
			zap.Error(err))
		return
	}
	log.Info(t.logString("device opened"))
	t.fd = fd

	if err = t.setup(); err != nil {
		log.Error(t.logString("failed to setup device"),
			zap.Error(err))
		return
	}
	log.Info(t.logString("device setup done"))

	err = t.relay(r)
	if err != nil {
		log.Error("failed to start local relay", zap.Error(err))
		return
	}

	log.Info(t.logString("processing ip packets"))
	buffer := make([]byte, t.mtu)
	// read packet
	for {
		n, err := t.fd.Read(buffer)
		if err != nil {
			if t.status.Load() != ingress.Running {
				return
			}
			log.Error(t.logString("failed to read from device"),
				zap.Error(err))
			continue
		}

		// check packet type
		packet := ipPacket(buffer[:n])
		switch packet.protocol() {
		case tcp:
			t.processStream(packet)
		case udp:
			t.processPacket(packet)
		case icmp:
			t.processICMP(packet)
		}
	}
}

func (t *Tun) processICMP(packet ipPacket) {
	icmp := icmpPacket(packet[packet.ihl():])
	icmp.setEchoReply()
	srcIP, dstIP := packet.srcIP(), packet.dstIP()
	packet.setSrcIP(dstIP)
	packet.setDstIP(srcIP)
	icmp.updateChecksum()
	t.fd.Write(packet)
}

func (t *Tun) processStream(packet ipPacket) {
	stream := tcpPacket(packet[packet.ihl():])
	srcIP, srcPort, dstIP, dstPort := packet.srcIP(), stream.srcPort(), packet.dstIP(), stream.dstPort()
	srcAddr := &net.TCPAddr{IP: srcIP, Port: srcPort}
	dstAddr := &net.TCPAddr{IP: dstIP, Port: dstPort}
	// check where the packet come from
	// from relay tcp listener
	if srcIP.Equal(t.relayIP) && srcPort == t.relayPort {
		var (
			realDst *net.TCPAddr
			realSrc *net.TCPAddr
		)
		// find mapped applications addtess pair
		entry, _ := t.tcpNat.Load(dstAddr.String())
		realDst = entry.(natEntry).from.(*net.TCPAddr)
		realSrc = entry.(natEntry).to.(*net.TCPAddr)
		// modify src address to mapped address pair
		packet.setSrcIP(realDst.IP)
		packet.setDstIP(realSrc.IP)
		stream.setSrcPort(realDst.Port)
		stream.setDstPort(realSrc.Port)
		// update checksum
		stream.updateChecksum(packet)
		packet.updateChecksum()
	} else {
		// from applications
		var (
			mappedAddr net.Addr
			mappedIP   net.IP
		)
		// check whether src exist in nat
		if entry, ok := t.tcpNat.Load(srcAddr.String()); ok {
			e := entry.(natEntry)
			mappedIP = e.to.(*net.TCPAddr).IP
		} else {
			// get mapped address pair first from pool first
			mappedIP = t.ipPool.Put(dstAddr.String())
			mappedAddr = &net.TCPAddr{IP: mappedIP, Port: srcPort}
			entry := natEntry{from: dstAddr, to: mappedAddr}
			entryReverse := natEntry{from: dstAddr, to: srcAddr}
			// store nat in bidirection
			t.tcpNat.Store(srcAddr.String(), entry)
			t.tcpNat.Store(mappedAddr.String(), entryReverse)
		}
		// modify dst address to relay listener
		packet.setSrcIP(mappedIP)
		packet.setDstIP(t.relayIP)
		stream.setDstPort(t.relayPort)
		// update checksum
		stream.updateChecksum(packet)
		packet.updateChecksum()
	}

	// write packet back
	n, err := t.fd.Write(packet)
	if err != nil {
		log.Error(t.logString("failed to write to device"),
			zap.Error(err))
	}
	if n < len(packet) {
		log.Error(t.logString("short written to device"))
	}
}

func (t *Tun) processPacket(packet ipPacket) {
	p := udpPacket(packet[packet.ihl():])
	srcIP, srcPort, dstIP, dstPort := packet.srcIP(), p.srcPort(), packet.dstIP(), p.dstPort()
	srcAddr := &net.UDPAddr{IP: srcIP, Port: srcPort}
	dstAddr := &net.UDPAddr{IP: dstIP, Port: dstPort}
	// check where the packet come from
	// from relay udp listener
	if srcIP.Equal(t.relayIP) && srcPort == t.relayPort {
		var (
			realDst *net.UDPAddr
			realSrc *net.UDPAddr
		)
		// find mapped applications addtess pair
		entry, _ := t.udpNat.Load(dstAddr.String())
		realDst = entry.(natEntry).from.(*net.UDPAddr)
		realSrc = entry.(natEntry).to.(*net.UDPAddr)
		// modify src address to mapped address pair
		packet.setSrcIP(realDst.IP)
		packet.setDstIP(realSrc.IP)
		p.setSrcPort(realDst.Port)
		p.setDstPort(realSrc.Port)
		// update checksum
		packet.updateChecksum()
		p.updateChecksum(packet)
	} else {
		// from applications
		var (
			mappedAddr net.Addr
			mappedIP   net.IP
		)
		// check whether src exist in nat
		if entry, ok := t.udpNat.Load(srcAddr.String()); ok {
			e := entry.(natEntry)
			mappedIP = e.to.(*net.UDPAddr).IP
		} else {
			// get mapped address pair first from pool first
			mappedIP = t.ipPool.Put(dstAddr.String())
			mappedAddr = &net.UDPAddr{IP: mappedIP, Port: srcPort}
			entry := natEntry{from: dstAddr, to: mappedAddr}
			entryReverse := natEntry{from: dstAddr, to: srcAddr}
			// store nat in bidirection
			t.udpNat.Store(srcAddr.String(), entry)
			t.udpNat.Store(mappedAddr.String(), entryReverse)
		}
		// modify dst address to relay listener
		packet.setSrcIP(mappedIP)
		packet.setDstIP(t.relayIP)
		p.setDstPort(t.relayPort)
		// update checksum
		packet.updateChecksum()
		p.updateChecksum(packet)
	}

	// write packet back
	n, err := t.fd.Write(packet)
	if err != nil {
		log.Error(t.logString("failed to write to device"),
			zap.Error(err))
	}
	if n < len(packet) {
		log.Error(t.logString("short written to device"))
	}
}

func (t *Tun) relay(r router.Router) error {
	log.Info(t.logString("starting local relay"))
	var relayErr = error(nil)
	wg := sync.WaitGroup{}
	wg.Add(3)
	// tcp
	go func() {
		lAddr := &net.TCPAddr{IP: t.relayIP, Port: t.relayPort}

		l, err := net.ListenTCP("tcp", lAddr)
		if err != nil {
			log.Error(t.logString("failed to listen relay tcp"),
				zap.Error(err))
			relayErr = err
			wg.Done()
			return
		}
		wg.Done()
		defer l.Close()
		log.Info(t.logString("local tcp relay started"),
			zap.String("Addr", lAddr.String()))

		for {
			c, err := l.Accept()
			if err != nil {
				log.Error(t.logString("failed to accept connection"),
					zap.Error(err))
				continue
			}

			if t.status.Load() == ingress.Closed {
				c.Close()
				return
			}

			// handle per connection
			go func() {
				// get applications mapped ip and port
				cAddr := c.RemoteAddr().(*net.TCPAddr)
				m := message.NewMetadata()
				m.WithIngress(t.Name()).WithClientIP(cAddr.IP).
					WithClientPort(cAddr.Port)

				// get real addr
				entry, _ := t.tcpNat.Load(cAddr.String())
				realDst := entry.(natEntry).from.(*net.TCPAddr)
				realSrc := entry.(natEntry).to.(*net.TCPAddr)
				var remoteAddr string
				if domain, exist := dns.GetDomainByIP(realDst.IP); exist {
					domain = domain[:len(domain)-1]
					m.WithDomain(domain).WithRemotePort(realDst.Port)
					remoteAddr = domain
				} else {
					m.WithRemoteIP(realDst.IP).WithRemotePort(realDst.Port)
					remoteAddr = realDst.String()
				}
				log.Info(t.logString("accept connection"),
					zap.String("remote", remoteAddr),
					zap.String("local", realSrc.String()))

				// construct proxy conn
				pc := newTunStream(c, m)
				t.conns.Store(pc.LocalAddr().String(), pc)
				defer func() {
					t.conns.Delete(pc.LocalAddr().String())
					pc.Close()
					log.Info(t.logString("connection closed"),
						zap.String("remote", remoteAddr),
						zap.String("local", realSrc.String()))
				}()

				// dispatch
				out := r.Dispatch(*m)
				log.Info(t.logString("connection dispatched"),
					zap.String("egress", out.Name()))
				out.ProcessStream(pc, nil)
			}()
		}
	}()

	// udp
	go func() {
		// nat
		nat := nat.New()

		// get connection
		lAddr := &net.UDPAddr{IP: t.relayIP, Port: t.relayPort}
		c, err := net.ListenUDP("udp", lAddr)
		if err != nil {
			log.Error(t.logString("failed to listen udp relay"),
				zap.Error(err))
			relayErr = err
			wg.Done()
			return
		}
		wg.Done()
		tc := newTunPacket(c)
		t.conns.Store(lAddr.String(), tc)
		log.Info(t.logString("local udp relay started"),
			zap.String("Addr", lAddr.String()))

		defer func() {
			t.conns.Delete(lAddr.String())
			tc.Close()
		}()

		// handle dns query
		go func() {
			ip, _ := iface.GetIPv4()
			l, err := net.ListenUDP("udp", &net.UDPAddr{IP: ip})
			if err != nil {
				log.Error(t.logString("failed to listen udp for hijacking dns"),
					zap.Error(err))
				relayErr = err
				wg.Done()
				return
			}
			wg.Done()
			buf := make([]byte, 4096)
			for {
				msg := <-t.dnsQuery
				cAddr := &net.UDPAddr{IP: msg.Metadata().ClientIP, Port: msg.Metadata().ClientPort}
				l.SetDeadline(time.Now().Add(3 * time.Second))
				_, err := l.WriteTo(msg.Payload(), t.dnsAddr)
				if err != nil {
					continue
				}
				n, err := l.Read(buf)
				if err != nil {
					continue
				}
				tc.WriteTo(buf[:n], cAddr)
			}
		}()

		// handle connection
		for {
			// try to close connection
			if t.status.Load() == ingress.Closed {
				return
			}

			// read msg from client
			msg, cAddr, err := tc.ReadMsgFrom()
			if err != nil {
				log.Error(t.logString("failed to read message"),
					zap.Error(err))
				continue
			}

			m := msg.Metadata()
			m.WithIngress(t.Name())
			entry, _ := t.udpNat.Load(cAddr.String())
			realDst := entry.(natEntry).from.(*net.UDPAddr)
			realSrc := entry.(natEntry).to.(*net.UDPAddr)
			// whether dns
			if t.NeedHijack(realDst.IP, realDst.Port) {
				t.dnsQuery <- msg
				continue
			}
			var remoteAddr string
			// get remote address
			if domain, exist := dns.GetDomainByIP(realDst.IP); exist {
				domain = domain[:len(domain)-1]
				m.WithDomain(domain).WithRemotePort(realDst.Port)
				remoteAddr = domain
			} else {
				m.WithRemoteIP(realDst.IP).WithRemotePort(realDst.Port)
				remoteAddr = realDst.String()
			}
			log.Info(t.logString("relay"),
				zap.String("local", realSrc.String()),
				zap.String("remote", remoteAddr))

			// check whether exist in nat
			if lc, exist := nat.Get(cAddr.String()); exist {
				rAddr := lc.Addr
				// proxy remote
				if rc, ok := lc.PacketConn.(conn.ProxyPacketConn); ok {
					if err := rc.WriteMsgTo(msg, rAddr); err != nil {
						log.Error(t.logString("failed to write msg"),
							zap.Error(err))
					}
					continue
				}
				// direct remote
				if rc, ok := lc.PacketConn.(*net.UDPConn); ok {
					if _, err := rc.WriteTo(msg.Payload(), rAddr); err != nil {
						log.Error(t.logString("failed to write msg"),
							zap.Error(err))
					}
					continue
				}
			}
			out := r.Dispatch(*m)
			fmt.Printf("%v\n", out)
			log.Info(t.logString("connection dispatched"),
				zap.String("egress", out.Name()))
			out.ProcessPacket(tc, msg)
		}
	}()

	wg.Wait()
	return relayErr
}

func (t *Tun) UnmarshalYAML(value *yaml.Node) error {
	var (
		name   string
		cidr   string
		port   int
		mtu    int
		hijack []string
		err    error
	)
	temp := make(map[string]any)
	err = value.Decode(&temp)
	if err != nil {
		return err
	}

	attrMust := map[string]any{
		"name":   &name,
		"cidr":   &cidr,
		"port":   &port,
		"mtu":    &mtu,
		"hijack": &hijack,
	}
	err = util.MustHave(temp, attrMust)
	if err != nil {
		return err
	}

	err = initTun(t, name, cidr, mtu, port, hijack)
	if err != nil {
		return err
	}
	return nil
}
