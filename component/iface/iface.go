// provide interfaces that can connect to internet
package iface

import (
	"errors"
	"net"
	"sync"
	"time"
)

var (
	errNotExist   = errors.New("not exist")
	errNoInternet = errors.New("no interface can access internet")
	defaultIfaces ifaces
	mu            = sync.RWMutex{}
)

func init() {
	defaultIfaces, _ = newInterfaces()
	go update()
}

type iff struct {
	net.Interface
	ipv4 []net.IP
	ipv6 []net.IP
}

type ifaces map[string]iff

func validIface(netif net.Interface) bool {
	if netif.Flags&net.FlagUp != net.FlagUp ||
		netif.Flags&net.FlagLoopback == net.FlagLoopback ||
		netif.Flags&net.FlagRunning != net.FlagRunning ||
        netif.Flags&net.FlagPointToPoint == net.FlagPointToPoint {
		return false
	}
	return true
}

func newInterfaces() (ifaces, error) {
	_ifaces := make(map[string]iff)

	// get all interfaces
	iffs, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, v := range iffs {
		v := v
		// drop down and loopback interface
		if !validIface(v) {
			continue
		}

		// then have a maybe ready interface
		_iff := iff{ipv4: make([]net.IP, 0), ipv6: make([]net.IP, 0)}
		_iff.Interface = v
		addrs, err := v.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			addr := addr
			if ip := addr.(*net.IPNet).IP; ip.To4() != nil {
				_iff.ipv4 = append(_iff.ipv4, ip.To4())
			} else {
				_iff.ipv6 = append(_iff.ipv6, ip)
			}
		}
		if len(_iff.ipv4) == 0 && len(_iff.ipv6) == 0 {
			continue
		}
		_ifaces[_iff.Name] = _iff
	}

	if len(_ifaces) == 0 {
		return nil, errNoInternet
	}

	return _ifaces, nil
}

func (iffs ifaces) getIPv4ByName(name string) (net.IP, error) {
	iff, exist := iffs[name]
	if !exist {
		return nil, errNotExist
	}
	return iff.ipv4[0], nil
}
func (iffs ifaces) getIPv6ByName(name string) (net.IP, error) {
	iff, exist := iffs[name]
	if !exist {
		return nil, errNotExist
	}
	return iff.ipv6[0], nil
}

func GetIPv4ByName(name string) (net.IP, error) {
	mu.RLock()
	defer mu.RUnlock()
	if defaultIfaces == nil {
		return nil, errNoInternet
	}

	return defaultIfaces.getIPv4ByName(name)
}

func GetIPv6ByName(name string) (net.IP, error) {
	mu.RLock()
	defer mu.RUnlock()
	if defaultIfaces == nil {
		return nil, errNoInternet
	}
	return defaultIfaces.getIPv6ByName(name)
}

func GetIPv4() (net.IP, error) {
	mu.RLock()
	defer mu.RUnlock()
	if defaultIfaces == nil {
		return nil, errNoInternet
	}

	out := make(net.IP, 4)
	for _, v := range defaultIfaces {
		if len(v.ipv4) == 0 {
			continue
		}
		copy(out, v.ipv4[0])
		break
	}
	if len(out) == 0 {
		return nil, errors.New("no ipv4 address")
	}
	return out, nil
}

func GetIPv6() (net.IP, error) {
	mu.RLock()
	defer mu.RUnlock()
	if defaultIfaces == nil {
		return nil, errNoInternet
	}

	out := make(net.IP, 16)
	for _, v := range defaultIfaces {
		if len(v.ipv6) == 0 {
			continue
		}
		copy(out, v.ipv6[0])
		break
	}
	if len(out) == 0 {
		return nil, errors.New("no ipv6 address")
	}
	return out, nil
}

func update() {
	ticker := time.NewTicker(time.Second * 60)
	for {
		<-ticker.C
		mu.Lock()
		newifaces, err := newInterfaces()
		if err != nil {
			continue
		}
		defaultIfaces = newifaces
		mu.Unlock()
	}
}
