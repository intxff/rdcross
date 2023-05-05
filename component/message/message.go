package message

import "net"

type AddrType uint8

const (
	_ = iota
	AddrTypeIP
	AddrTypeDomain
)

type Metadata struct {
	// take from connect socket
	// client starts the connection
	ClientIP   net.IP
	ClientPort int
	// take from message below
	// remote is what client want to connect to
	RemoteIP    net.IP
	RemotePort  int
	Domain      string
	ProcessName string
	ProcessPath string
	// take from ingress
	Ingress string
}

func NewMetadata() *Metadata {
    return &Metadata{}
}
func (m *Metadata) WithClientIP(ip net.IP) *Metadata {
    m.ClientIP = ip
    return m
}
func (m *Metadata) WithClientPort(port int) *Metadata {
    m.ClientPort = port
    return m
}
func (m *Metadata) WithRemoteIP(ip net.IP) *Metadata {
    m.RemoteIP = ip
    return m
}
func (m *Metadata) WithRemotePort(port int) *Metadata {
    m.RemotePort = port
    return m
}
func (m *Metadata) WithDomain(d string) *Metadata {
    m.Domain = d
    return m
}
func (m *Metadata) WithProcessName(d string) *Metadata {
    m.ProcessName = d
    return m
}
func (m *Metadata) WithProcessPath(d string) *Metadata {
    m.ProcessPath = d
    return m
}
func (m *Metadata) WithIngress(d string) *Metadata {
    m.Ingress = d
    return m
}

type Message interface {
    Payload() []byte
    Metadata() *Metadata
    Others() any
}
