package enghouse

import (
	"encoding/xml"
	//    "strconv"
	//   "strings"
)

type Enghouse struct {
	XMLName     xml.Name   `xml:"digeoAPI"`
	MSOCode     string     `xml:"MSOCode,attr"`
	Version     string     `xml:"version,attr"`
	Transaction []EngTrans `xml:"transaction"`
}

type EngTrans struct {
	//	Transaction     xml.Name      `xml:"transaction"`
	Action         string `xml:"action,attr"`
	MSOAccountID   string `xml:"MSOAccountID,attr"`
	TransID        string `xml:"transID,attr"`
	TransTime      string `xml:"transTime,attr"`
	Account_status string `xml:"account_status"`
	MSO_account_id string `xml:"mso_account_id"`
	//        Old_mso_account_id      string     `xml:"old_mso_account_id"`
	MSO_market_id string `xml:"mso_market_id"`
	First_name    string `xml:"first_name"`
	Country       string `xml:"country"`
	//        Headend_id      string  `xml:"headend_id"`
	Channelmap_id string       `xml:"channelmap_id"`
	Service       []EngService `xml:"service_codes"`
	Devices       []EngDevices `xml:"devices"`
}

type EngService struct {
	Text          string `xml:",chardata"`
	Service_codes string `xml:"service_code"`
}

type EngDevices struct {
	Text   string      `xml:",chardata"`
	Device []EngDevice `xml:"device"`
}

type EngDevice struct {
	Text             string `xml:",chardata"`
	HardwareDeviceID string `xml:"hardwareDeviceID,attr"`
}
