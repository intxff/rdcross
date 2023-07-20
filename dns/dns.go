package dns

import (
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/intxff/rdcross/component/fakeip"
	"github.com/intxff/rdcross/component/iface"
	"github.com/miekg/dns"
)

type DNS struct {
	Enable   bool     `yaml:"enable"`
	Listen   string   `yaml:"listen"`
	Upstream []string `yaml:"upstream"`
	FakeIP   FakeIP   `yaml:"fakeip"`
}

type FakeIP struct {
	Enable bool   `yaml:"enable"`
	Cidr   string `yaml:"cidr"`
	Ttl    int    `yaml:"ttl"`
}

type DNSServer struct {
	*dns.Server
}

func (d *DNS) NewServer(pool *fakeip.FakeIP) *DNSServer {
	resolver.Upstream = d.Upstream
	t := &DNSServer{
		Server: &dns.Server{
			Addr:    d.Listen,
			Net:     "udp",
			Handler: newDeafaultDNS(d),
		},
	}
	if d.FakeIP.Enable {
		t.Handler = newFakeIPDNS(d.Upstream, pool)
	}
	return t
}

func asyncQuery(m *dns.Msg, upstream []string) (*dns.Msg, error) {
	type response struct {
		m *dns.Msg
		e error
	}

	var (
		l   = len(upstream)
		rs  response
	)
	res := make(chan response, l)

	for i := 0; i < l; i++ {
		lIP, err := iface.GetIP()
        if err != nil {
            return nil, err
        }
		go func(i int) {
			var rs *dns.Msg
			// bind to avoid route decision
			rAddrString := strings.Split(upstream[i], ":")
			rIP := net.ParseIP(rAddrString[0])
			rPort, _ := strconv.Atoi(rAddrString[1])
			rAddr := &net.UDPAddr{
				IP:   rIP,
				Port: rPort,
			}
			conn, err := net.DialUDP("udp", &net.UDPAddr{IP: lIP}, rAddr)
            if err != nil {
				res <- response{nil, err}
				return
            }
            dnsConn := &dns.Conn{Conn: conn}
            dnsConn.SetWriteDeadline(time.Now().Add(1*time.Second))
            err = dnsConn.WriteMsg(m)
			if err != nil {
				res <- response{nil, err}
				return
			}
            dnsConn.SetReadDeadline(time.Now().Add(1*time.Second))
            rs, err = dnsConn.ReadMsg()
            if err != nil {
				res <- response{nil, err}
				return
            }
			res <- response{rs, err}
		}(i)
	}

	for i := 0; i < l; i++ {
		rs = <-res
		if rs.e == nil {
			return rs.m, rs.e
		}
	}
	return rs.m, rs.e
}
