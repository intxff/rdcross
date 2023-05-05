package rule

import (
	"errors"

	"github.com/intxff/rdcross/component/message"
)

var _ Rule = (*PrgName)(nil)

type PrgName map[string]*Action

func NewRulePrgName() *PrgName {
	r := make(PrgName)
	return &r
}

func (r *PrgName) Name() string {
	return "PRGNAME"
}

func (r *PrgName) Match(m message.Metadata, others ...any) (*Action, bool) {
	prgName := m.ProcessName
	if prgName == "" {
		return nil, false
	}
	action, exist := (*r)[prgName]
	if !exist {
		return nil, false
	}
	return action, true
}

func (r *PrgName) Insert(a ...any) error {
	prgName, ok := a[0].(string)
	if !ok {
		return errors.New("invalid program name to insert into PRGNAME")
	}
	action, ok := a[1].(*Action)
	if !ok {
		return errors.New("invalid action to insert into PRGNAME")
	}
	(*r)[prgName] = action
	return nil
}

func (r *PrgName) Empty() bool {
	return len(*r) == 0
}
