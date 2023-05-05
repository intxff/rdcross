package rule

import (
	"errors"

	"github.com/intxff/rdcross/component/message"
	"github.com/intxff/rdcross/util/trie"
)

var _ Rule = (*Domain)(nil)

type Domain struct {
	*trie.Trie
}

func NewRuleDomain(t *trie.Trie) *Domain {
	return &Domain{Trie: t}
}

func (d *Domain) Name() string {
	return "DOMAIN"
}

func (d *Domain) Match(m message.Metadata, others ...any) (*Action, bool) {
	domain := m.Domain
    if domain == "" {
        return nil, false
    }
	t, err := d.Search(domain)
	if err != nil {
		return nil, false
	}
	if data, ok := t.Value().(Action); ok {
		return &data, true
	}
	return nil, false
}

func (d *Domain) Insert(a ...any) error {
	domain, ok := a[0].(string)
	if !ok {
		return errors.New("invalid domain to insert into domain trie")
	}
	action, ok := a[1].(*Action)
	if !ok {
		return errors.New("invalid action to insert into domain trie")
	}
	return d.Trie.Insert(domain, action)
}

func (d *Domain) Empty() bool {
    return d.Trie.Empty()
}
