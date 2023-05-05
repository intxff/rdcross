package tun

import (
	"errors"
	"net"
)

/*
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|Version| Traffic Class |           Flow Label                  |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|         Payload Length        |  Next Header  |   Hop Limit   |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                                                               |
+                                                               +
|                                                               |
+                         Source Address                        +
|                                                               |
+                                                               +
|                                                               |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                                                               |
+                                                               +
|                                                               |
+                      Destination Address                      +
|                                                               |
+                                                               +
|                                                               |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
 0                   1                   2                   3
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|Version|  IHL  |Type of Service|          Total Length         |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|         Identification        |Flags|      Fragment Offset    |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|  Time to Live |    Protocol   |         Header Checksum       |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                       Source Address                          |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                    Destination Address                        |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                    Options                    |    Padding    |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+ */

type netProtocol uint8

const (
	_ netProtocol = iota
	udp
	tcp
    icmp
    others
)

type ipVersion uint8

const (
	_ ipVersion = iota
	ipv4
	ipv6
)

var (
	errInvalidIPv4 = errors.New("invalid IPv4 address")
	errInvalidIPv6 = errors.New("invalid IPv6 address")
)

type ipPacket []byte

func (p ipPacket) version() ipVersion {
	if p[0]&0xf0 == 0x06 {
		return ipv6
	}
	return ipv4
}

func (p ipPacket) ihl() int {
	return int(p[0]&0x0f) * 4
}

func (p ipPacket) protocol() netProtocol {
	switch p[9] {
	case 0x06:
		return tcp
	case 0x11:
		return udp
    case 0x01:
        return icmp
	}
	return others
}

func (p ipPacket) srcIP() net.IP {
	if p[0]&0xf0 == 0x06 {
		ip := make([]byte, 16)
		copy(ip, p[8:24])
		return net.IP(ip)
	}
	ip := make([]byte, 4)
    copy(ip, p[12:16])
	return net.IP(ip)
}

func (p ipPacket) dstIP() net.IP {
	if p.version() == ipv6 {
		ip := make([]byte, 16)
		copy(ip, p[24:40])
		return net.IP(ip)
	}
	ip := make([]byte, 4)
    copy(ip, p[16:20])
	return net.IP(ip)
}

func (p ipPacket) setSrcIP(s net.IP) error {
	if p.version() == ipv6 {
		if s.To16() == nil {
			return errInvalidIPv6
		}
		copy(p[8:24], []byte(s))
	}
	if s.To4() == nil {
		return errInvalidIPv4
	}
	copy(p[12:16], []byte(s))
	return nil
}

func (p ipPacket) setDstIP(s net.IP) error {
	if p.version() == ipv6 {
		if s.To16() == nil {
			return errInvalidIPv6
		}
		copy(p[24:40], []byte(s))
	}
	if s.To4() == nil {
		return errInvalidIPv4
	}
	copy(p[16:20], s.To4())
	return nil
}

func (p ipPacket) updateChecksum() {
	if p.version() == ipv6 {
		return
	}
	copy(p[10:12], []byte{0, 0})
	sum := checksum(sum(p[:p.ihl()]))
	copy(p[10:12], []byte{byte(sum >> 8), byte(sum)})
}

func (p ipPacket) pseudoSum() uint32 {
	if p.version() == ipv6 {
		return sum(p[8:40])
	}
	return sum(p[12:20])
}

func checksum(s ...uint32) uint16 {
	sum := uint32(0)
	for _, v := range s {
		sum += v
	}
	for sum>>16 != 0 {
		sum = sum>>16 + sum&0xffff
	}
	sum = ^sum
	return uint16(sum)
}

func sum(b []byte) uint32 {
	var sum uint32 = 0
	for i, l := 0, len(b); i < l; i += 2 {
		sum += uint32(b[i]) << 8
		if i+1 < l {
			sum += uint32(b[i+1])
		}
	}
	return sum
}
