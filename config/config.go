package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/intxff/rdcross/egress"
	"github.com/intxff/rdcross/egress/direct"
	"github.com/intxff/rdcross/egress/reject"
	"github.com/intxff/rdcross/ingress"
	"github.com/intxff/rdcross/ingress/tun"
	"github.com/intxff/rdcross/log"
	"github.com/intxff/rdcross/router"
	"github.com/intxff/rdcross/util"
	"github.com/intxff/rdcross/util/trie"
	"gopkg.in/yaml.v3"
)

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

// parse raw config to get binary marshaled structure
func ParseRawConfig(path string) (*RdConfig, error) {
	config := RdConfig{}
	path, err := util.GetAbsPath(path)
	if err != nil {
		return nil, err
	}

	buf, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal(buf, &config)
	if err != nil {
		return nil, err
	}
	config.Log.Path, _ = util.GetAbsPath(config.Log.Path)
	config.Path = path
    config.Dir = filepath.Dir(path)

	return &config, nil
}

// parse ingress
func (c *RdConfig) ParseIngress() (map[string]ingress.Ingress, error) {
	ingresses := make(map[string]ingress.Ingress)
	for i := 0; i < len(c.Ingress); i++ {
		if c.Ingress[i].Type() == ingress.TypeTun {
			c.Ingress[i].(*tun.Tun).WithDNS(c.DNS.Listen)
		}
		name := c.Ingress[i].Name()
		if _, exist := ingresses[name]; exist {
			return nil, ErrDup{Zone: "ingress", Name: name}
		}
		ingresses[name] = c.Ingress[i]
	}
	return ingresses, nil
}

// parse ingress group
func (c *RdConfig) ParseIngressGroup() (map[string][]string, error) {
	ingressGroup := make(map[string][]string)
	for i := 0; i < len(c.IngressGroup); i++ {
		name := c.IngressGroup[i].Name
		members := c.IngressGroup[i].Member
		if _, exist := ingressGroup[name]; exist {
			return nil, ErrDup{Zone: "ingress", Name: name}
		}
		ingressGroup[name] = make([]string, len(members))
		copy(ingressGroup[name], members)
	}
	return ingressGroup, nil
}

// parse egress
func (c *RdConfig) ParseEgress() (map[string]egress.Egress, error) {
	egresses := make(map[string]egress.Egress)
	for i := 0; i < len(c.Egress); i++ {
		name := c.Egress[i].Name()
		if _, exist := egresses[name]; exist {
			return nil, ErrDup{Zone: "egress", Name: name}
		}
		egresses[name] = c.Egress[i]
	}
	direct := direct.NewDirect()
	reject := reject.NewReject()
	egresses[direct.Name()] = direct
	egresses[reject.Name()] = reject
	return egresses, nil
}

// parse egress group
func (c *RdConfig) ParseEgressGroup() (map[string][]string, error) {
	egressGroup := make(map[string][]string)
	for i := 0; i < len(c.EgressGroup); i++ {
		name := c.EgressGroup[i].Name
		members := c.EgressGroup[i].Member
		if _, exist := egressGroup[name]; exist {
			return nil, ErrDup{Zone: "egress", Name: name}
		}
		egressGroup[name] = make([]string, len(members))
		copy(egressGroup[name], members)
	}
	return egressGroup, nil
}

// parse router
func (c *RdConfig) ParseRouter(t *trie.Trie, e map[string]egress.Egress) router.Router {
	r := router.NewDefaultRouter(e)

	for index := range c.Rule {
		entry := strings.Split(c.Rule[index], ",")
		if len(entry) < 2 {
			log.Panic(fmt.Sprintf("invalid rule %v", c.Rule[index]))
		}
		// append none to make rule in uniformed format
		if len(entry) < 4 && entry[0] != "PRIOR" {
			entry = append(entry, "none")
		}

		if len(entry) == 3 && entry[0] == "DEFAULT" {
			r.Insert("DEFAULT", "", entry[1], entry[2])
			break
		}

		switch strings.ToUpper(entry[0]) {
		case "PRIOR":
			r.Prior = append(r.Prior, entry[1:]...)
		case "DOMAIN":
			ruleType, pattern, out, p := entry[0], entry[1], entry[2], entry[3]
			r.Insert(ruleType, pattern, out, p, t)
		case "GEOIP":
			ruleType, pattern, out, p := entry[0], entry[1], entry[2], entry[3]
			r.Insert(ruleType, pattern, out, p, c.Dir)
		default:
			ruleType, pattern, out, p := entry[0], entry[1], entry[2], entry[3]
			r.Insert(ruleType, pattern, out, p)
		}
	}

	// check prior,if not set, set to default sort
	if len(r.Prior) == 0 {
		log.Info("PRIOR not set, use default: ROUTE PRGPATH PRGNAME DOMAIN GEOIP")
		r.Prior = append(r.Prior, "ROUTE", "PRGPATH", "PRGNAME", "DOMAIN", "GEOIP")
	}

	return r
}

// parse log
func (c *RdConfig) ParseLog() *log.Log {
	return &c.Log
}
