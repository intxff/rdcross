package tun

/*
 0      7 8     15 16    23 24    31
+--------+--------+--------+--------+
|     Source      |   Destination   |
|      Port       |      Port       |
+--------+--------+--------+--------+
|                 |                 |
|     Length      |    Checksum     |
+--------+--------+--------+--------+
|
|          data octets ...
+---------------- ...                  */

type appProtocol int

const (
    _ appProtocol = iota
    _dns
)

type udpPacket []byte

func (p udpPacket) srcPort() int {
	return int(p[0])<<8 + int(p[1])
}

func (p udpPacket) dstPort() int {
	return int(p[2])<<8 + int(p[3])
}

func (p udpPacket) setSrcPort(b int) {
	t := []byte{byte(b >> 8), byte(b)}
	copy(p[0:2], t)
}

func (p udpPacket) setDstPort(b int) {
	copy(p[2:4], []byte{byte(b >> 8), byte(b)})
}

func (p udpPacket) updateChecksum(ip ipPacket) {
	copy(p[6:8], []byte{0, 0})
	sum := checksum(ip.pseudoSum(), sum(p), uint32(0x11), uint32(len(p)))
	copy(p[6:8], []byte{byte(sum >> 8), byte(sum)})
}
