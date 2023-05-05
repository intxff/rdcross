package lru

import (
	"container/list"
	"sync"
)

type LRU struct {
	capacity       int
	list           *list.List
	keyToElement   sync.Map
	valueToElement sync.Map
	m              sync.Mutex
}

type lruElement struct {
	key   interface{}
	value interface{}
}

func New(cap int) *LRU {
	return &LRU{
		capacity:       cap,
		list:           list.New(),
		keyToElement:   sync.Map{},
		valueToElement: sync.Map{},
		m:              sync.Mutex{},
	}
}

func (l *LRU) Get(key interface{}) (value interface{}, exist bool) {
	l.m.Lock()
	defer l.m.Unlock()
	if v, ok := l.keyToElement.Load(key); ok {
		element := v.(*list.Element)
		l.list.MoveToFront(element)
		return element.Value.(*lruElement).value, true
	}
	return nil, false
}

func (l *LRU) GetKeyFromValue(value interface{}) (key interface{}, exist bool) {
	l.m.Lock()
	defer l.m.Unlock()
	if k, ok := l.valueToElement.Load(value); ok {
		element := k.(*list.Element)
		l.list.MoveToFront(element)
		return element.Value.(*lruElement).key, true
	}
	return nil, false
}

func (l *LRU) Put(key, value interface{}) {
	l.m.Lock()
	defer l.m.Unlock()
	e := &lruElement{key, value}
	if v, ok := l.keyToElement.Load(key); ok {
		element := v.(*list.Element)
		element.Value = e
		l.list.MoveToFront(element)
	} else {
		element := l.list.PushFront(e)
		l.keyToElement.Store(key, element)
		l.valueToElement.Store(value, element)
		if l.list.Len() > l.capacity {
			toBeRemove := l.list.Back()
			l.list.Remove(toBeRemove)
			l.keyToElement.Delete(toBeRemove.Value.(*lruElement).key)
			l.valueToElement.Delete(toBeRemove.Value.(*lruElement).value)
		}
	}
}

func (l *LRU) ReplaceLastValue(key interface{}) interface{} {
	l.m.Lock()
	defer l.m.Unlock()

	element := l.list.Back()
	element.Value.(*lruElement).key = key
	l.list.PushFront(element)
	l.keyToElement.Store(key, element)
	return l.list.Front().Value.(*lruElement).value
}

func (l *LRU) GetLastValue() interface{} {
	l.m.Lock()
	defer l.m.Unlock()

	element := l.list.Back()
	if element == nil {
		return nil
	}
	return element.Value.(*lruElement).value
}

func (l *LRU) IsFull() bool {
	return l.list.Len() >= l.capacity
}

func (l *LRU) IsEmpty() bool {
	return l.list.Len() == 0
}
