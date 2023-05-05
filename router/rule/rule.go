package rule

import (
	"github.com/intxff/rdcross/component/message"
	"github.com/intxff/rdcross/router/policy"
)

type Rule interface {
	Name() string
	Match(message.Metadata, ...any) (*Action, bool)
	Insert(...any) error
	Empty() bool
}

type Action struct {
	Egress string
	Policy policy.Policy
}

func NewAction(e string, p string) *Action {
    var po policy.Policy
    switch p {
    case "none":
        po = policy.NewPolicyNone()
    default:
        po = policy.NewPolicyNone()
    }
	return &Action{Egress: e, Policy: po}
}
