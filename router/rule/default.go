package rule

import (
	"errors"

	"github.com/intxff/rdcross/component/message"
	"github.com/intxff/rdcross/router/policy"
)

var _ Rule = (*Default)(nil)

type Default Action

func NewRuleDefault() *Default {
    return &Default{Egress: "DIRECT", Policy: policy.NewPolicyNone()}
}

func (d *Default) Name() string {
	return "DEFAULT"
}

func (d *Default) Match(m message.Metadata, others ...any) (*Action, bool) {
	return (*Action)(d), true
}

func (d *Default) Insert(a ...any) error {
	action, ok := a[0].(*Action)
	if !ok {
		return errors.New("invalid action to insert into DEFAULT")
	}
    d.Egress = action.Egress
    d.Policy = action.Policy
	return nil
}

func (d *Default) Empty() bool {
    return false
}
