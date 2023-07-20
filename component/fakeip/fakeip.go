package fakeip

import (
	"math"
	"math/big"
	"net"
	"sync"

	"github.com/intxff/rdcross/util/lru"
)

type FakeIP struct {
	lru           *lru.LRU
	ipRange       *net.IPNet
	nextIP        *big.Int
	m             sync.Mutex
	networkAddr   net.IP
	broadcastAddr net.IP
	ifaceAddr     net.IP
}

func New(ipCIDR string) (*FakeIP, error) {
	ifaceAddr, ipRange, err := net.ParseCIDR(ipCIDR)
	if err != nil {
		return nil, err
	}

	ones, bits := ipRange.Mask.Size()
	rooms := bits - ones
    lruSize := int(math.Pow(2, float64(rooms)))-3

	nextIP := big.NewInt(0).SetBytes(ipRange.IP)

	over := big.NewInt(0).SetBytes(ipRange.IP)
	over.Exp(big.NewInt(2), big.NewInt(int64(bits)), nil).
		Sub(over, big.NewInt(1)).
		Rsh(over, uint(ones)).
		Or(nextIP, over)
	broadcastAddr := over.Bytes()

	ippool := &FakeIP{
		lru:           lru.New(lruSize),
		ipRange:       ipRange,
		nextIP:        big.NewInt(0).SetBytes(ipRange.IP),
		m:             sync.Mutex{},
		networkAddr:   ipRange.IP,
		broadcastAddr: broadcastAddr,
		ifaceAddr:     ifaceAddr,
	}
	return ippool, err
}

func (f *FakeIP) GetIPByDomain(domain string) (net.IP, bool) {
	ip, exist := f.lru.Get(domain)
	if !exist {
		return nil, false
	}
	return net.ParseIP(ip.(string)), true
}

func (f *FakeIP) GetDomainByIP(ip net.IP) (string, bool) {
	if !f.ipRange.Contains(ip) {
		return "", false
	}
	domain, exist := f.lru.GetKeyFromValue(ip.String())
	if !exist {
		return "", false
	}
	return domain.(string), true
}

func (f *FakeIP) Put(domain string) net.IP {
	f.m.Lock()
	defer f.m.Unlock()

	var ip net.IP
	if f.lru.IsFull() {
        ips := f.lru.GetLastValue().(string)
		f.lru.Put(domain, ips)
		return net.ParseIP(ips)
	}
	for {
		f.nextIP = f.nextIP.Add(f.nextIP, big.NewInt(1))
		ip = net.IP(f.nextIP.Bytes())
		if !f.ipRange.Contains(ip) {
			f.nextIP = big.NewInt(0).SetBytes(f.networkAddr)
			continue
		}
		if f.ifaceAddr.Equal(ip) || f.broadcastAddr.Equal(ip) {
			continue
		}

		if _, exist := f.lru.GetKeyFromValue(ip.String()); !exist {
			break
		}
	}
	f.lru.Put(domain, ip.String())
	return ip
}
