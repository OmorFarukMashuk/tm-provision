/*
	A simple abstraction package for the DHCP database

*/
package dhcpdb

import (
	"net"
)

// Define the SQL database we are connecting to
type sqlServer struct {
	Hostname string
	Username string
	Password string
}

// A host reservation on the DHCP server, modelled after Kea DHCP data structure
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

// Reservations can have multiple IPv6 objects, typically an IA_NA and an IA_PD for prefix delegation
type ipv6Reservation struct {
	ResID   int
	Address net.IP
	Length  int // The CIDR mask.  Usually 60 a telMAX or 128 for an IA_NA
	Type    int
	Iaid    int
	HostID  int
}
