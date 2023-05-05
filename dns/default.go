package dns

import "github.com/miekg/dns"

type defaultDNS struct {
	Upstream  []string
}

func newDeafaultDNS(d *DNS) *defaultDNS {
    return &defaultDNS{Upstream: d.Upstream}
}

func (d *defaultDNS) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
    r, _ = asyncQuery(r, d.Upstream)
    w.WriteMsg(r)
}
