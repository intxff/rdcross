package router

import (
	"fmt"

	"github.com/intxff/rdcross/component/message"
	"github.com/intxff/rdcross/egress"
	"github.com/intxff/rdcross/log"
	"github.com/intxff/rdcross/router/policy"
	"github.com/intxff/rdcross/router/rule"
	"github.com/intxff/rdcross/util/trie"
)

type Router interface {
	Dispatch(m message.Metadata) egress.Egress
}

var _ Router = (*DefaultRouter)(nil)

type DefaultRouter struct {
	Prior       []string
	Rules       map[string]rule.Rule
	Egress      map[string]egress.Egress
	EgressGroup map[string][]string
}

func NewDefaultRouter(e map[string]egress.Egress) *DefaultRouter {
	return &DefaultRouter{
		Prior:  make([]string, 0, 10),
		Rules:  make(map[string]rule.Rule),
		Egress: e,
	}
}

func (d *DefaultRouter) Dispatch(m message.Metadata) egress.Egress {
	// match rule to get action
	var action *rule.Action
	var ok bool

	for i := 0; i < len(d.Prior); i++ {
		if entry, exist := d.Rules[d.Prior[i]]; exist {
			if !entry.Empty() {
				action, ok = entry.Match(m)
				if ok {
					break
				}
			}
		}
	}
	if action == nil {
		action, _ = d.Rules["DEFAULT"].Match(m)
	}

	if action.Policy.Type() == policy.TypeNone {
		if action.Egress == "DIRECT" {
			fmt.Printf("DIRECT: %v\n", d.Egress[action.Egress])
		}
		return d.Egress[action.Egress]
	}

	e := action.Policy.Select(d.EgressGroup)
	return d.Egress[e]
}

func (d *DefaultRouter) Insert(ruleType, pattern, out, policy string, others ...any) {
	switch ruleType {
	case "DEFAULT":
		if _, exist := d.Rules["DEFAULT"]; !exist {
			d.Rules["DEFAULT"] = rule.NewRuleDefault()
		}
		d.Rules["DEFAULT"].Insert(rule.NewAction(out, policy))
	case "ROUTE":
		if _, exist := d.Rules["ROUTE"]; !exist {
			d.Rules["ROUTE"] = rule.NewRuleRoute()
		}
		d.Rules["ROUTE"].Insert(pattern, rule.NewAction(out, policy))
	case "DOMAIN":
		if _, exist := d.Rules["DOMAIN"]; !exist {
			d.Rules["DOMAIN"] = rule.NewRuleDomain(others[0].(*trie.Trie))
		}
		d.Rules["DOMAIN"].Insert(pattern, rule.NewAction(out, policy))
	case "GEOIP":
		if _, exist := d.Rules["GEOIP"]; !exist {
			d.Rules["GEOIP"] = rule.NewRuleGEOIP(others[0].(string))
		}
		d.Rules["GEOIP"].Insert(pattern, rule.NewAction(out, policy))
	case "PRGAME":
		if _, exist := d.Rules["PRGNAME"]; !exist {
			d.Rules["PRGNAME"] = rule.NewRulePrgName()
		}
		d.Rules["PRGNAME"].Insert(pattern, rule.NewAction(out, policy))
	case "PRGPATH":
		if _, exist := d.Rules["PRGPATH"]; !exist {
			d.Rules["PRGPATH"] = rule.NewRulePrgPath()
		}
		d.Rules["PRGPATH"].Insert(pattern, rule.NewAction(out, policy))
	default:
		log.Panic(fmt.Sprintf("invalid rule %v", ruleType))
	}
}
