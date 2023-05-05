package ingress

import (
	"github.com/intxff/rdcross/component/proxy"
	"github.com/intxff/rdcross/component/transport"
	"github.com/intxff/rdcross/router"
)

const (
    _ int32 = iota
    Ready
    Running
    Closed
)

type Ingress interface {
	Name() string

	// Most suitation can use multi transports and a proxy
	// to handle incoming packets. Sometimes sources from
	// tun/tap or other uncommon ways are different. So need
	// Type() to distinguish
	Type() IngressType

	// Run create `conn` defined in component/conn
	// Then router handles these connections to egress
	Run(r router.Router)

	Proxy() proxy.Proxy
	Transport() []transport.Transport
    Close() <- chan struct{}
}

type IngressType string

const (
	TypeGeneral IngressType = "GENERAL"
	TypeTun     IngressType = "TUN"
)
