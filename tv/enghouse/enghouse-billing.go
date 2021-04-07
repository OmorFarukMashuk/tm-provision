package enghouse

import (
	"bitbucket.org/timstpierre/telmax-common"
	"bytes"
	"crypto/tls"
	"encoding/xml"
	"errors"
	"fmt"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo"
	"io/ioutil"
	"net/http"
	"time"
)

var (
	EHusername      string = "telmax_billing"
	EHpasswd        string = "691DistanceMedium300"
	EHURL           string = "https://billing.moxi.com/billing/UpdateAccount/1?MSO=TELMAX"
	DefaultServices        = []string{
		"IPTV_CUTV",
		"IPTV_NDVR_50",
		"IPTV_PLTV",
		"IPTV_RSTV",
	}
)

// This is now in a separate file
/*
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

*/
func EnghouseRequest(accountdata EngTrans, requestID string) error {
	var err error

	requestdate := time.Now().Format("20060102150405")
	log.Info("Request date string is %v", requestdate)
	accountdata.TransID = requestID
	accountdata.TransTime = requestdate
	transaction := Enghouse{
		MSOCode: "TELMAX",
		Version: "1.0.0",
		Transaction: []EngTrans{
			accountdata,
		},
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	// Create this in a different function.  Use this one just to do the API call.
	/*
		xml_string := Enghouse{
			MSOCode: "TELMAX",
			Version: "1.0.0",
			Transaction: []EngTrans{
				{
					Action:         "account.update",
					MSOAccountID:   "1495738",
					TransID:        "103085",
					TransTime:      "20200616181100",
					Account_status: "ACTIVE",
					MSO_account_id: "tim-test-2",
					//        	Old_mso_account_id: " ",
					MSO_market_id: "Telmax",
					First_name:    "Tim",
					Country:       "CA",
					//        	Headend_id:	"PHE_TELMAX",
					Channelmap_id: "CMAP_TELMAX_UNICAST_RESIDENTIAL",
					Service: []EngService{
						{
							Service_codes: "IPTV_NDVR_50",
						},
					}, // `xml:"service_codes"`
					Devices: []EngDevices{
						{
							//			Temp:			"",
							Device: []EngDevice{
								{
									HardwareDeviceID: "0003E6F6FDA6",
								},
							}, // `xml:"device"`
						}, // `xml:"devices"`
					}, // `xml:"EngTrans"`
				},
			},
		}
	*/
	var xmlStr []byte

	xmlStr, err = xml.Marshal(transaction)
	//    if err != nil{
	fmt.Println(err)
	//    }
	//	myString := []byte(xml.Header + string(xmlStr))
	xmlStr2 := append([]byte(xml.Header), xmlStr...)
	//    fmt.Printf("%s\n", myString)
	//	fmt.Println(xmlStr)

	xmlStr3 := bytes.NewBuffer(xmlStr2)
	log.Debugf("%s\n", xmlStr3)

	// Main Server
	var req *http.Request
	req, err = http.NewRequest("POST", EHURL, bytes.NewBuffer(xmlStr2))

	//  Testing Server
	//    req, err := http.NewRequest("POST", "https://billing-stg.moxi.com/billing/UpdateAccount/1?MSO=TELMAX", bytes.NewBuffer([]byte(myString)))
	req.Header.Add("Content-Type", "text/plain")
	//    	req.Header.Add("Content-Length", strconv.Itoa(len(data.Encode())))
	req.SetBasicAuth(EHusername, EHpasswd)
	resp, err := client.Do(req)
	log.Debug("http", resp.StatusCode, http.StatusText(resp.StatusCode))
	//	fmt.Println(resp)
	//	fmt.Println(err)
	if err != nil {
		log.Error(err)
	}
	if resp.StatusCode != 200 {
		bodyData, _ := ioutil.ReadAll(resp.Body)
		bodyText := string(bodyData)
		//		log.Errorf("Problem with API call to Enghouse %v", bodyText)
		err = errors.New(bodyText)
	} else {
		log.Info("API Call successful!")
	}

	// Handle the error somehow

	return err

}

func EnghouseAccount(CoreDB *mongo.Database, accountcode string, subscribecode string) (accountdata EngTrans, err error) {
	timestring := "" // Use some time functions to create this value

	// Get the subscribe record - this has the essential details in it
	var subscribe telmax.Subscribe
	subscribe, err = telmax.GetSubscribe(CoreDB, accountcode, subscribecode)

	// Rough in the strcuture with the data we know.
	accountdata = EngTrans{
		Action:       "account.update",
		MSOAccountID: subscribe.AccountCode + subscribe.SubscribeCode,
		// How do you want to create a transaction ID?
		//TransID:
		TransTime:      timestring,
		Account_status: "ACTIVE", // Might not always be active, but this is easily fixed
		MSO_account_id: subscribe.AccountCode + subscribe.SubscribeCode,
		MSO_market_id:  "Telmax",
		First_name:     subscribe.FirstName,
		Country:        "CA",
		Channelmap_id:  "CMAP_TELMAX_UNICAST_RESIDENTIAL",
	}

	// Get the channels that this account / subscribe combination have, and sort by TV channels.  We may need to get packages too
	var channels []telmax.SubscribedProduct
	channel_filters := []telmax.Filter{
		telmax.Filter{
			Key:   "account_code",
			Value: subscribe.AccountCode,
		},
		telmax.Filter{
			Key:   "subscribe_code",
			Value: subscribe.SubscribeCode,
		},
		telmax.Filter{
			Key:   "category",
			Value: "TV",
		},
	}

	// Iterate over the channels.  If they have an Enghouse code, push onto the array
	channels, err = telmax.GetServices(CoreDB, channel_filters)

	for _, channel := range channels {
		var productData telmax.Product
		productData, err = telmax.GetProduct(CoreDB, "product_code", channel.ProductCode)
		if productData.EnghouseCode != "" {
			accountdata.Service = append(accountdata.Service, EngService{
				Service_codes: productData.EnghouseCode,
			})
		}
	}

	if len(channels) > 0 {
		for _, defaultchannel := range DefaultServices {
			accountdata.Service = append(accountdata.Service, EngService{
				Service_codes: defaultchannel,
			})
		}
	}

	// Do the same for devices.  Only get the Amino boxes.
	var devices []telmax.Device
	device_filters := []telmax.Filter{
		telmax.Filter{
			Key:   "account_code",
			Value: subscribe.AccountCode,
		},
		telmax.Filter{
			Key:   "subscribe_code",
			Value: subscribe.SubscribeCode,
		},
		telmax.Filter{
			Key:   "devicedefinition_code",
			Value: "DEVIDEFI0036", // This is the definition code for an Amino Amigo7x
		},
	}
	devices, err = telmax.GetDevices(CoreDB, device_filters)
	var deviceList []EngDevice
	for _, device := range devices {
		deviceList = append(deviceList, EngDevice{
			HardwareDeviceID: device.Mac, // Maybe format this to make sure it is upper / no special chars, etc.
		})
	}
	if len(deviceList) > 0 {
		accountdata.Devices = append(accountdata.Devices, EngDevices{
			Device: deviceList,
		})
	}
	log.Infof("Account Data for EngHouse is %v", accountdata)
	return

}
