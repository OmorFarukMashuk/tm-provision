//
// Go objects for Juniper XML configuration
//
package netconf

import (
	"encoding/xml"
)

type SystemInformation struct {
	HardwareModel string `xml:"system-information>hardware-model"`
	OsName        string `xml:"system-information>os-name"`
	OsVersion     string `xml:"system-information>os-version"`
	SerialNumber  string `xml:"system-information>serial-number"`
	HostName      string `xml:"system-information>host-name"`
}

type InterfaceStatus struct {
	Name        string `xml:"interface-information>physical-interface>name" json:"interface_name"`
	Result      string `xml:"interface-information>rpc-error>error-message" json:"result"`
	AdminStatus string `xml:"interface-information>physical-interface>admin-status" json:"interface_admin_status"`
	LinkStatus  string `xml:"interface-information>physical-interface>oper-status" json:"interface_link_status"`
	SnmpIndex   int32  `xml:"interface-information>physical-interface>local-index" json:"interface_snmp_index"`
	Description string `xml:"interface-information>physical-interface>description" json:"interface_description"`
	Speed       string `xml:"interface-information>physical-interface>speed" json:"interface_speed"`
	MediaType   string `xml:"interface-information>physical-interface>if-media-type" json:"media_type"`
	SignalLevel SignalLevel
}

type SignalLevel struct {
	TxPower      float32 `xml:"interface-information>physical-interface>optics-diagnostics>laser-output-power-dbm" json:"transmit_power"`
	RxPower      float32 `xml:"interface-information>physical-interface>optics-diagnostics>rx-signal-avg-optical-power-dbm" json:"receive_power"`
	RxPowerAlarm string  `xml:"interface-information>physical-interface>optics-diagnostics>laser-rx-power-low-alarm" json:"receive_power_alarm"`
}

type JuniperConfig struct {
	XMLName    xml.Name   `xml:"configuration"`
	Interfaces Interfaces `xml:interfaces`
}

type Interfaces struct {
	XMLName           xml.Name            `xml:"interfaces"`
	PhysicalInterface []PhysicalInterface `xml:"interface"`
}

type PhysicalInterface struct {
	MergeMode     string             `xml:"replace,attr,omitempty"`
	Name          string             `xml:"name"`
	Description   string             `xml:"description,omitempty"`
	EtherOptions  *EtherOptions      `xml:"ether-options,omitempty"`
	Encapsulation string             `xml:"encapsulation,omitempty"`
	LogicalUnits  []LogicalInterface `xml:"unit,omitempty"`
	NativeVlanID  int                `xml:"native-vlan-id,omitempty"`
	FlexVlan      string             `xml:"flexible-vlan-tagging,omitempty"`
}

type EtherOptions struct {
	XMLName xml.Name `xml:"ether-options"`
	Bundle  string   `xml:"ieee-802.3ad>bundle,omitempty"`
}

type LogicalInterface struct {
	Unit          int                `xml:"name"`
	Description   string             `xml:"description,omitempty"`
	Switching     *EthernetSwitching `xml:"family>ethernet-switching,omitempty"`
	Encapsulation string             `xml:"encapsulation,omitempty"`
	Inet          *LogicalIPAddr     `xml:"family>inet,omitempty"`
	Inet6         *LogicalIPAddr     `xml:"family>inet6,omitempty"`
	VlanID        int                `xml:"vlan-id,omitempty"`
	VlanIDList    string             `xml:"vlan-id-list,omitempty"`
	InputFilter   string             `xml:"filter-input,omitempty"`
	OutputFilter  string             `xml:"filter-output,omitempty"`
}

type EthernetSwitching struct {
	PortMode     string        `xml:"interface-mode,omitempty"`
	VlanMembers  []VlanMembers `xml:"vlan"`
	InputFilter  string        `xml:"filter-input,omitempty"`
	OutputFilter string        `xml:"filter-output,omitempty"`
}

type LogicalIPAddr struct {
	IPAddress    string `xml:"address>name,omitempty"`
	InputFilter  string `xml:"filter-input,omitempty"`
	OutputFilter string `xml:"filter-output,omitempty"`
}

type VlanMembers struct {
	Member string `xml:"members,omitempty"`
}

type ConfigurationResults struct {
	Results       string     `xml:"load-configuration-results>load-success"`
	ErrorCount    int        `xml:"load-configuration-results>load-error-count"`
	ErrorMessages []RPCError `xml:"load-configuration-results>rpc-error"`
}
type RPCError struct {
	ErrorType    string `xml:"error-type"`
	ErrorMessage string `xml:"error-message"`
}

type CommitResults struct {
	Results       []string   `xml:"commit-results>routing-engine"`
	ErrorMessages []RPCError `xml:"commit-results>rpc-error"`
}

type ConfigureStatus struct {
	Success       bool     `bson:"success" json:"success"`
	ErrorMessages []string `bson:"errors,omitempty" json:"errors,omitempty"`
}
