package rule

import (
	"errors"

	"github.com/intxff/rdcross/component/message"
)

var _ Rule = (*Route)(nil)

type Route map[string]*Action

func NewRuleRoute() *Route {
	r := make(Route)
	return &r
}

func (r *Route) Name() string {
	return "ROUTE"
}

func (r *Route) Match(m message.Metadata, others ...any) (*Action, bool) {
	ingress := m.Ingress
	if ingress == "" {
		return nil, false
	}
	action, exist := (*r)[ingress]
	if !exist {
		return nil, false
	}
	return action, true
}

func (r *Route) Insert(a ...any) error {
	ingress, ok := a[0].(string)
	if !ok {
		return errors.New("invalid ingress to insert into ROUTE")
	}
	action, ok := a[1].(*Action)
	if !ok {
		return errors.New("invalid action to insert into ROUTE")
	}
	(*r)[ingress] = action
	return nil
}

func (r *Route) Empty() bool {
	return len(*r) == 0
}
