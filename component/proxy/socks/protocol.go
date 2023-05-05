package socks

// negotitaion
// +----+----------+----------+
// |VER | NMETHODS | METHODS  |
// +----+----------+----------+
// | 1  |    1     | 1 to 255 |
// +----+----------+----------+
//
//       +----+--------+
//       |VER | METHOD |
//       +----+--------+
//       | 1  |   1    |
//       +----+--------+
//
// request
// +----+-----+-------+------+----------+----------+
// |VER | CMD |  RSV  | ATYP | DST.ADDR | DST.PORT |
// +----+-----+-------+------+----------+----------+
// | 1  |  1  | X'00' |  1   | Variable |    2     |
// +----+-----+-------+------+----------+----------+
//
// reply
// +----+-----+-------+------+----------+----------+
// |VER | REP |  RSV  | ATYP | BND.ADDR | BND.PORT |
// +----+-----+-------+------+----------+----------+
// | 1  |  1  | X'00' |  1   | Variable |    2     |
// +----+-----+-------+------+----------+----------+
//
// udp
// +----+------+------+----------+----------+----------+
// |RSV | FRAG | ATYP | DST.ADDR | DST.PORT |   DATA   |
// +----+------+------+----------+----------+----------+
// | 2  |  1   |  1   | Variable |    2     | Variable |
// +----+------+------+----------+----------+----------+

const (
	// version, only socks5
	Ver = byte(0x05)
	// methods
	MethodNoAuth       = byte(0x00)
	MethodGssapi       = byte(0x01)
	MethodPassword     = byte(0x02)
	MethodNoAcceptable = byte(0xff)
	// commands
	CmdConnect      = byte(0x01)
	CmdBind         = byte(0x02)
	CmdUdpAssociate = byte(0x03)
	// reserve
	Rsv = byte(0x00)
	// address types
	AtypIPv4   = byte(0x01)
	AtypDomain = byte(0x03)
	AtypIPv6   = byte(0x04)
	// reply
	ReplySucceeded          = byte(0x00)
	ReplyServerFailed       = byte(0x01)
	ReplyNotAllowed         = byte(0x02)
	ReplyNetworkUnreachable = byte(0x03)
	ReplyHostUnreachable    = byte(0x04)
	ReplyConnRefused        = byte(0x05)
	ReplyTtlExpired         = byte(0x06)
	ReplyCmdNotSupported    = byte(0x07)
	ReplyAtypNotSupported   = byte(0x08)
	ReplyUnassigned         = byte(0x09)
)

const (
	VerPos           = 0
	NMethodsPos      = 1
	ClientMethodsPos = 2
	ServerMethodsPos = 1
	CmdPos           = 1
	ReplyPos         = 1
	RsvPos           = 2
	AtypPos          = 3
	AddrPos          = 4
	DomainPos        = 4
	Ipv4Size         = 4
	Ipv6Size         = 16
	PortSize         = 2
)
