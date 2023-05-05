package rule

import (
	"errors"
	"fmt"

	"github.com/intxff/rdcross/component/message"
	"github.com/intxff/rdcross/component/mmdb"
	"github.com/intxff/rdcross/log"
	"github.com/oschwald/geoip2-golang"
	"go.uber.org/zap"
)

var _ Rule = (*GEOIP)(nil)

type GEOIP struct {
	Action map[string]*Action
	Mmdb   *geoip2.Reader
}

func NewRuleGEOIP(path string) *GEOIP {
	path += "/mmdb"
	if err := mmdb.InitMMDB(path); err != nil {
		log.Panic("can not init mmdb", zap.Error(err))
	}
	r := GEOIP{
		Action: make(map[string]*Action),
		Mmdb:   mmdb.Instance(path),
	}
	return &r
}

func (g *GEOIP) Name() string {
	return "GEOIP"
}

func (g *GEOIP) Match(m message.Metadata, others ...any) (*Action, bool) {
	ip := m.RemoteIP
	if ip == nil {
		fmt.Printf("nil ip: %v, match: %v\n", ip, false)
		return nil, false
	}
	country, err := g.Mmdb.Country(ip)
	if err != nil {
		log.Error(fmt.Sprintf("get geoip error when match ip:%v", ip.String()), zap.Error(err))
		fmt.Printf("ip: %v, match: %v\n", ip, false)
		return nil, false
	}
	if action, exist := g.Action[country.Country.IsoCode]; exist {
        fmt.Printf("ip: %v, match: %v, action: %v\n", ip, true, action)
		return action, true
	}
	fmt.Printf("ip: %v, match: %v\n", ip, false)
	return nil, false
}

func (g *GEOIP) Insert(a ...any) error {
	country, ok := a[0].(string)
	if !ok {
		return errors.New("invalid ingress to insert into ROUTE")
	}
	action, ok := a[1].(*Action)
	if !ok {
		return errors.New("invalid action to insert into ROUTE")
	}
	g.Action[country] = action
	return nil
}

func (g *GEOIP) Empty() bool {
	return len(g.Action) == 0
}
