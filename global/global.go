package global

import (
	"fmt"
	"sync"

	"github.com/intxff/rdcross/component/fakeip"
	"github.com/intxff/rdcross/config"
	"github.com/intxff/rdcross/dns"
	"github.com/intxff/rdcross/egress"
	"github.com/intxff/rdcross/ingress"
	"github.com/intxff/rdcross/log"
	"github.com/intxff/rdcross/router"
	"github.com/intxff/rdcross/util/trie"
)

const (
	DNS          = "dns"
	Ingress      = "ingress"
	IngressGroup = "ingress group"
	Egress       = "egress"
	EgressGroup  = "egress group"
	Logger       = "logger"
	Router       = "router"
)

type ErrInvalidValue string

func (e ErrInvalidValue) Error() string {
	return fmt.Sprintf("invalid value in %v", string(e))
}

type ErrDup struct {
	Name string
	Zone string
}

func (e ErrDup) Error() string {
	return fmt.Sprintf("duplicate %v in %v or %v group", e.Name, e.Zone, e.Zone)
}

func (e ErrDup) Is(err error) bool {
	t, ok := err.(ErrDup)
	if !ok {
		return false
	}
	if t.Zone == e.Zone && t.Name == e.Name {
		return true
	}
	return false
}

var global *resource = &resource{}

func New() *resource {
	return global
}

type resource struct {
	muLogger sync.RWMutex
	Logger   *log.Log

	muDNS sync.RWMutex
	DNS   *dns.DNSServer

	muIngress sync.RWMutex
	Ingress   map[string]ingress.Ingress

	muIg         sync.RWMutex
	IngressGroup map[string][]string

	muEgress sync.RWMutex
	Egress   map[string]egress.Egress

	muEg        sync.RWMutex
	EgressGroup map[string][]string

	muRouter sync.RWMutex
	Router   *router.Router
}

func Register(key string, value ...any) error {
	switch key {
	case DNS:
		return registerDNS(value...)
	case Ingress:
		return registerIngress(value...)
	case IngressGroup:
		return registerIngressGroup(value...)
	case Egress:
		return registerEgress(value...)
	case EgressGroup:
		return registerEgressGroup(value...)
	case Logger:
		return registerLogger(value...)
	case Router:
		return registerRouter(value...)
	}
	return nil
}

func registerRouter(value ...any) error {
	v, ok := value[0].(*router.Router)
	if !ok {
		return ErrInvalidValue(Router)
	}

	global.muRouter.Lock()
	global.Router = v
	global.muRouter.Unlock()
	return nil
}

func registerLogger(value ...any) error {
	v, ok := value[0].(*log.Log)
	if !ok {
		return ErrInvalidValue(Logger)
	}
	global.muLogger.Lock()
	global.Logger = v
	global.muLogger.Unlock()
	return nil
}

func registerDNS(value ...any) error {
	v, ok := value[0].(*dns.DNSServer)
	if !ok {
		return ErrInvalidValue(DNS)
	}
	global.muDNS.Lock()
	global.DNS = v
	global.muDNS.Unlock()
	return nil
}

// value = map[string]ingress.Ingress
func registerIngress(value ...any) error {
	m, ok := value[0].(map[string]ingress.Ingress)
	if !ok {
		return ErrInvalidValue(Ingress)
	}

	global.muIngress.Lock()
	global.Ingress = m
	global.muIngress.Unlock()
	return nil
}

// value = map[string][]string
func registerIngressGroup(value ...any) error {
	m, ok := value[0].(map[string][]string)
	if !ok {
		return ErrInvalidValue(IngressGroup)
	}

	// check whether ingress and ig have the same name
	global.muIngress.RLock()
	for k := range m {
		if _, exist := global.Ingress[k]; exist {
			return ErrDup{Zone: Ingress, Name: k}
		}
	}
	global.muIngress.RUnlock()

	global.muIg.Lock()
	global.IngressGroup = m
	global.muIg.Unlock()
	return nil
}

func registerEgress(value ...any) error {
	m, ok := value[0].(map[string]egress.Egress)
	if !ok {
		return ErrInvalidValue(Egress)
	}

	global.muEgress.Lock()
	global.Egress = m
	global.muEgress.Unlock()
	return nil
}

func registerEgressGroup(value ...any) error {
	m, ok := value[0].(map[string][]string)
	if !ok {
		return ErrInvalidValue(EgressGroup)
	}
	// check whether egress and eg have the same name
	global.muEgress.RLock()
	for k := range m {
		if _, exist := global.Egress[k]; exist {
			return ErrDup{Zone: Egress, Name: k}
		}
	}
	global.muEgress.RUnlock()

	global.muEg.Lock()
	global.EgressGroup = m
	global.muEg.Unlock()
	return nil
}

func Init(c *config.RdConfig) error {
	// prepare shared resource: trie, nat,
	var fakeipPool *fakeip.FakeIP
	domainTrie := trie.New()
	if c.DNS.FakeIP.Enable {
		fakeipPool, _ = fakeip.New(c.DNS.FakeIP.Cidr, 10000)
	}

	// logger
	if err := Register(Logger, c.ParseLog()); err != nil {
		return err
	}

	// egress
	eg, err := c.ParseEgress()
    if err != nil {
        return err
    }
	if err := Register(Egress, eg); err != nil {
		return err
	}

	eGroup, err := c.ParseEgressGroup()
    if err != nil {
        return err
    }
	if err := Register(EgressGroup, eGroup); err != nil {
		return err
	}

	// router
	router := c.ParseRouter(domainTrie, eg)
    fmt.Printf("egress: %v\n", global.Egress)
    fmt.Printf("router: %v\n", router)
	if err := Register(Router, &router); err != nil {
		return err
	}

	// dns
	if c.DNS.Enable {
		d := c.DNS.NewServer(fakeipPool)
		if err := Register(DNS, d); err != nil {
			return err
		}
	}

	// ingress
    ig, err := c.ParseIngress()
    if err != nil {
        return err
    }
	if err := Register(Ingress, ig); err != nil {
		return err
	}

	// ingress group
    iGroup, err := c.ParseEgressGroup()
    if err != nil {
        return err
    }
	if err := Register(IngressGroup, iGroup); err != nil {
		return err
	}

	return nil
}
