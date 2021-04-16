package main

import (
	"flag"
)

var (
	DHCPSQL = flag.String("dhcpsql", "dhcp01.lab.dc1.osh.telmax.ca", "DHCP server database string")
)

func DHCPDBConnect() {

}

func DHCPDBClose() {

}

func AllocateLease(node string, pool string, subscriber string) (lease DHCPLease, err error) {

}

func GetLease(filter []filter) (lease DHCPLease, err error) {

}

func UpdateCircuitLease(circuit string) (lease DHCPLease, err error) {

}

type DHCPLease struct {
	ID       int    // DHCP lease ID
	Node     string // Wirecentre this lease belongs to
	Pool     string // Pool name
	Vlan     int    // The vLAN ID used
	VlanName string // The name of the vlan
	IPv4     string // The string representation of the IPv4 address
	IPv6NA   string // The IA_NA host address for the lease
	IPv6PD   string // The delegated prefix for the lease
	PDSize   int    // The length of the delegated prefix

}

type filter struct {
	Key   string
	Value interface{}
}
