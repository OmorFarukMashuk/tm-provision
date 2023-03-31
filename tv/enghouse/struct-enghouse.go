package enghouse

import (
	"encoding/xml"
	//    "strconv"
	//   "strings"
)

type Enghouse struct {
	XMLName     xml.Name   `xml:"digeoAPI"`
	MsoCode     string     `xml:"MSOCode,attr"`
	Version     string     `xml:"version,attr"`
	Transaction []EngTrans `xml:"transaction"`
}

type EngTrans struct {
	Action          string `xml:"action,attr"`                  // A string identifier for the transaction to be executed. This version of the Espial Provisioning API supports a single action: account.update
	MSOACCOUNTID    string `xml:"MSOAccountID,attr"`            // A string identifier for the subscriber account to be affected by the transaction. It is assumed that mso_account_ids are unique within an MSO's context. Note that this value is duplicated in the attribute for the transaction and an element within. The element within exists to support updating an account ID.
	TransId         string `xml:"transID,attr"`                 // A string identifier assigned to the transaction by the source system (either the billing system or provisioning layer.) This identifier has meaning only to the originating system. The Espial systems will use this identifier only for logging.
	TransTime       string `xml:"transTime,attr"`               // A time stamp string conforming to the ISO8601 standard, specifically using the format: YYYYMMDDThhmmss Timestamps should always be in UTC format, not in local time.
	AccountStatus   string `xml:"account_status"`               // A string identifying the status of the account. "ACTIVE"|"REMOVED"|”SUSPEND”
	MsoAccountId    string `xml:"mso_account_id"`               // A string identifier for the subscriber account. It is assumed that mso_account_ids are unique within an MSO's context.
	MsoMarketId     string `xml:"mso_market_id"`                // A string identifier for the billing system market containing the subscriber account to be affected by the transaction. It is assumed that MSOMarketIDs are unique within an MSO's context. Note that in the first release an account's MSOMarketID cannot be changed once the account is created. An Espial 'Market' can be equal to the following: CSG: <sys>-<prin> Amdocs: <corp> ICOMS: <site>-<company>-<division>
	FirstName       string `xml:"first_name"`                   // The subscriber’s first name.
	PostalCode      string `xml:"postal_code"`                  // A valid 5-digit US zip code or 6-character Canadian postal code, without punctuation or spaces.
	OldMsoAccountId string `xml:"old_mso_account_id,omitempty"` // The old MSO-defined ID for the account. If specified, this effectively transfers the old account to the new account number (which is specified in the element “mso_account_id”). Note that the XML must still specify the devices associated with the account and the service codes.
	MsoHeadendId    string `xml:"headend_id,omitempty"`         // A string identifier for the physical headend. This is optional, and will usually be omitted. If not set here, it must be set during installation via the installer tool.
	ChannelMapId    string `xml:"channelmap_id,omitempty"`      // A string identifier for the channelmap. This is optional, and will usually be omitted. If not set here, it must be set during installation via the installer tool. If the type is not specified or is “cable,” this will be the channelmap identifier. If the type is “ota,” this will be the zip code for the OTA channel.
	Country         string `xml:"country"`                      // ISO3166-1 two-character codes to identify the subscriber's country. Valid values are “CA” for Canada and “US” for United States. If no value is sent, US will be implied.

	Service []EngService `xml:"service_codes"`
	Devices []EngDevices `xml:"devices"`
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
	Text              string `xml:",chardata"`
	HardwareDeviceId  string `xml:"hardwareDeviceID,attr"`
	AlternateDeviceId string `xml:"alternateDeviceID,attr"`
	Nickname          string `xml:"nickname,attr"`
	DeviceGroup       string `xml:"deviceGroup,attr"`
}

/*
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
	PostalCode	string	`xml:"postal_code"`
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
*/
