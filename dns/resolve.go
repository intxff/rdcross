package dns

import (
	"net"

	"github.com/miekg/dns"
)

type _Resolver struct {
	Upstream []string
}

var resolver = new(_Resolver)

func ResolveIPv4(domain string) ([]net.IP, error) {
	out := make([]net.IP, 0)
	if len(resolver.Upstream) == 0 {
		ips, err := net.LookupIP(domain)
		if err != nil {
			return nil, err
		}
		for _, v := range ips {
			if v.To4() != nil {
				out = append(out, v.To4())
			}
		}
	}
	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(domain), dns.TypeA)
	r, err := asyncQuery(msg, resolver.Upstream)
	if err != nil {
		return nil, err
	}
	for _, v := range r.Answer {
		out = append(out, v.(*dns.A).A)
	}
	return out, nil
}

func ResolveIPv6(domain string) ([]net.IP, error) {
	out := make([]net.IP, 0)
	if len(resolver.Upstream) == 0 {
		ips, err := net.LookupIP(domain)
		if err != nil {
			return nil, err
		}
		for _, v := range ips {
			if v.To16() != nil {
				out = append(out, v)
			}
		}
	}
	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(domain), dns.TypeAAAA)
	r, err := asyncQuery(msg, resolver.Upstream)
	if err != nil {
		return nil, err
	}
	for _, v := range r.Answer {
		out = append(out, v.(*dns.AAAA).AAAA)
	}
	return out, nil
}
