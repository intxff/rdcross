package trie

import (
	"errors"
	"strings"
)

type Trie struct {
	next map[string]*Trie
	data any
}

func New() *Trie {
	return &Trie{next: make(map[string]*Trie), data: *new(any)} 
}

func (t *Trie) Value() any {
	return t.data
}

func (t *Trie) Insert(s string, data any) error {
	r := strings.Split(s, ".")

	cur := t
	for i := len(r) - 1; i >= 0; i-- {
		if cur.hasString(r[i]) {
			cur = cur.next[r[i]]
			if i == 0 {
				cur.data = data
			}
		} else {
			cur.next[r[i]] = New()
			cur = cur.next[r[i]]
			if i == 0 {
				cur.data = data
			}
		}
	}
	return nil
}

func (t *Trie) Search(s string) (*Trie, error) {
	r := strings.Split(s, ".")

	cur := t
	wild := false
	for i := len(r) - 1; i >= 0; i-- {
		switch {
		case cur.hasString(r[i]):
			wild = false
			cur = cur.next[r[i]]
			if i == 0 {
				if cur.isDataNode() {
					return cur, nil
				}
			}
		case cur.hasWild():
			cur = cur.next["+"]
			wild = true
			if i == 0 {
				if cur.isDataNode() {
					return cur, nil
				}
			}
		case wild:
			if i == 0 {
				if cur.isDataNode() {
					return cur, nil
				}
			}
		}
	}

	return nil, errors.New(strings.Join([]string{"can't find target domain", s}, " "))
}

func (t *Trie) isDataNode() bool {
	return t.data != ""
}

func (t *Trie) hasString(s string) bool {
	if _, exist := t.next[s]; !exist {
		return false
	}
	return true
}

func (t *Trie) hasWild() bool {
	if _, exist := t.next["+"]; !exist {
		return false
	}
	return true
}

func (t *Trie) Empty() bool {
    return len(t.next) == 0
}
