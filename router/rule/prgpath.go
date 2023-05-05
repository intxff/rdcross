package rule

import (
	"errors"

	"github.com/intxff/rdcross/component/message"
)

var _ Rule = (*PrgPath)(nil)

type PrgPath map[string]*Action

func NewRulePrgPath() *PrgPath {
	r := make(PrgPath)
	return &r
}

func (r *PrgPath) Name() string {
	return "PRGPATH"
}

func (r *PrgPath) Match(m message.Metadata, others ...any) (*Action, bool) {
	prgPath := m.ProcessPath
	if prgPath == "" {
		return nil, false
	}
	action, exist := (*r)[prgPath]
	if !exist {
		return nil, false
	}
	return action, true
}

func (r *PrgPath) Insert(a ...any) error {
	prgPath, ok := a[0].(string)
	if !ok {
		return errors.New("invalid program path to insert into PRGPATH")
	}
	action, ok := a[1].(*Action)
	if !ok {
		return errors.New("invalid action to insert into PRGPATH")
	}
	(*r)[prgPath] = action
	return nil
}

func (r *PrgPath) Empty() bool {
	return len(*r) == 0
}
