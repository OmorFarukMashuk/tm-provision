package dhcpdb

import (
	"net"
)

type sqlServer struct {
	Hostname string
	Username string
	Password string
}

type Reservation struct {
	HostID   int    // The DHCP server unique record
	SubnetID int    // The DHCP server subnet ID value
	DhcpID   string // The circuit-id or identifier type to be used
	Pool     string // The named pool this lease belongs to
	Node     string // The node / wirecentre this lease works with
	VlanID   int    // The vlan ID this lease is valid on
	V4Addr   net.IP // IPv4 address
	V6wan    net.IP // IPv6 IA_NA address
	V6dp     net.IP // IPv6 IA_PD address
	V6size   int    // IPv6 delegation size
}

type ipv6Reservation struct {
	ResID   int
	Address net.IP
	Length  int
	Type    int
	Iaid    int
	HostID  int
}
