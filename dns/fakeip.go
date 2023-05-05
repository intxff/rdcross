package dns

import (
	"net"

	"github.com/intxff/rdcross/component/fakeip"

	"github.com/miekg/dns"
)

var fake *fakeipDNS

func GetDomainByIP(ip net.IP) (string, bool) {
	return fake.fakeip.GetDomainByIP(ip)
}

type fakeipDNS struct {
	fakeip   *fakeip.FakeIP
	upstream []string
}

func newFakeIPDNS(upstream []string, pool *fakeip.FakeIP) *fakeipDNS {
	t := &fakeipDNS{
		fakeip:   pool,
		upstream: upstream,
	}
	fake = t
	return t
}

func (h *fakeipDNS) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	var (
		rr    dns.RR
		ip    net.IP
		exist bool
	)
	qname := r.Question[0].Name
	qtype := r.Question[0].Qtype
	m := new(dns.Msg)
	m.SetReply(r)
	m.Compress = true

	if ip, exist = h.fakeip.GetIPByDomain(qname); !exist {
		ip = h.fakeip.Put(qname)
	}
    if qtype == dns.TypePTR {
        ptrIP := ExtractAddressFromReverse(r.Question[0].Name)
        domain, exist := h.fakeip.GetDomainByIP(net.ParseIP(ptrIP))
        if !exist {
            domain = ""
        }
        rr = &dns.PTR {
			Hdr: dns.RR_Header{Name: r.Question[0].Name, Rrtype: dns.TypePTR, Class: dns.ClassINET, Ttl: 0},
            Ptr: domain,
        }
    }
	if qtype == dns.TypeA {
		rr = &dns.A{
			Hdr: dns.RR_Header{Name: r.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 0},
			A:   ip,
		}
	}
	if qtype == dns.TypeAAAA {
		rr = &dns.AAAA{
			Hdr:  dns.RR_Header{Name: r.Question[0].Name, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 0},
			AAAA: ip.To16(),
		}
	}
	m.Answer = append(m.Answer, rr)
	w.WriteMsg(m)
}
