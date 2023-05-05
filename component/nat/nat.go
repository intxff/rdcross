package nat

import (
	"net"
	"sync"
)

var one = sync.Once{}
var _nat nat

type LinkPacketConn struct {
    net.PacketConn
    net.Addr // addrress used by packetconn's WriteTo
}

type nat struct {
	table map[string]LinkPacketConn
	l     sync.RWMutex
}

func New() *nat {
	one.Do(func() {
		_nat = nat{
			table: make(map[string]LinkPacketConn),
			l:     sync.RWMutex{},
		}
	})
    return &_nat
}

func (n *nat) Get(key string) (LinkPacketConn, bool) {
	n.l.RLock()
	defer n.l.RUnlock()
	value, exist := n.table[key]
	return value, exist
}

func (n *nat) Set(key string, value LinkPacketConn) {
	n.l.Lock()
	defer n.l.Unlock()
	n.table[key] = value
}

func (n *nat) Delete(key string) {
	n.l.Lock()
	defer n.l.Unlock()
	delete(n.table, key)
}
