package enghouse

import (
	"fmt"
	"strings"

	"bitbucket.org/telmaxdc/telmax-common"
	"bitbucket.org/telmaxdc/telmax-common/devices"
	"bitbucket.org/telmaxdc/telmax-common/maxbill"

	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/xml"
	"errors"
	"flag"
	"io/ioutil"
	"net/http"
	"time"

	//log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo"
)

var (
	EHusername          string = "telmax_billing"
	EHpasswd            string = "691DistanceMedium300"
	EHURL               string = "https://telmax-billing.moxi.com/billing/UpdateAccount/1?MSO=TELMAX"
	RootCertificatePath        = flag.String("cacert", "/etc/ssl/cert.pem", "Path to the CA certificate")

	EHSkipVerify    = flag.Bool("skiptls", false, "Skip TLS verification with Enghouse")
	DefaultServices = []string{
		"IPTV_CUTV",
		"IPTV_NDVR_50",
		"IPTV_PLTV",
		"IPTV_RSTV",
	}
)

//func QueryAccount(accountCode, subscribeCode, deviceId string) string {

/*
	<?xml version="1.0" encoding="UTF-8"?>
	<!-- Espial transaction header to provide MSO context and namespace for
	rest of payload -->
	<digeoAPI partnerID="[ESPIALPartnerID]" MSOCode="[MSO]"
	version="1.0.0">
	<transaction action="query.account" MSOAccountID="[AccountID]"
	transID=”[TransactionID]” transTime=”[Timestamp]”/>
	</digeoAPI>
*/
//}

func EnghouseRequest(accountdata EngTrans, requestID string) error {
	requestdate := time.Now().Format("20060102150405")
	//log.Debugf("Request date (%v)", requestdate)
	accountdata.TransId = requestID
	accountdata.TransTime = requestdate
	transaction := Enghouse{
		MsoCode: "TELMAX",
		Version: "1.0.1",
		Transaction: []EngTrans{
			accountdata,
		},
	}
	var client http.Client
	if *EHSkipVerify {
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		client = http.Client{
			Transport: tr,
			Timeout:   10 * time.Second,
		}
	} else {
		//   Changed from skipping TLS, to checking EngHouse Self-signed Cert
		rootCAPool := x509.NewCertPool()
		rootCA, err := ioutil.ReadFile(*RootCertificatePath)
		if err != nil {
			return err
		}
		rootCAPool.AppendCertsFromPEM(rootCA)
		client = http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				IdleConnTimeout: 10 * time.Second,
				TLSClientConfig: &tls.Config{RootCAs: rootCAPool},
			},
		}
	}
	xmlStr, err := xml.Marshal(transaction)
	if err != nil {
		return err
	}
	xmlStr2 := append([]byte(xml.Header), xmlStr...)
	fmt.Println(string(xmlStr2))
	// Main Server
	req, err := http.NewRequest("POST", EHURL, bytes.NewBuffer(xmlStr2))
	if err != nil {
		return err
	}
	//  Testing Server
	//    req, err := http.NewRequest("POST", "https://billing-stg.moxi.com/billing/UpdateAccount/1?MSO=TELMAX", bytes.NewBuffer([]byte(myString)))
	req.Header.Add("Content-Type", "text/plain")
	//    	req.Header.Add("Content-Length", strconv.Itoa(len(data.Encode())))
	req.SetBasicAuth(EHusername, EHpasswd)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	if resp == nil {
		return errors.New("empty response")
	}
	if resp.StatusCode != 200 {
		bodyData, _ := ioutil.ReadAll(resp.Body)
		bodyText := string(bodyData)
		err = errors.New(resp.Status + "\n" + bodyText)
	}
	return err
}

func EnghouseAccount(CoreDB *mongo.Database, accountcode string, subscribecode string) (accountdata EngTrans, err error) {

	// Get the subscribe record - this has the essential details in it
	subscribe, err := maxbill.GetSubscribe(CoreDB, accountcode, subscribecode)
	if err != nil {
		return
	}
	if subscribe.SubscribeCode == "" {
		err = errors.New("subscribe not found")
		return
	}
	if subscribe.TVUsername == "" {
		subscribe.AddTVUser()
		subscribe.Update(CoreDB)
	}

	// Typical defaults
	accountdata = EngTrans{
		Action:        "account.update",
		MSOACCOUNTID:  subscribe.AccountCode + subscribe.SubscribeCode,
		TransId:       "",       // added by request
		TransTime:     "",       // added my request
		AccountStatus: "ACTIVE", // Might not always be active, but this is easily fixed
		MsoAccountId:  subscribe.AccountCode + subscribe.SubscribeCode,
		MsoMarketId:   "Telmax",
		FirstName:     subscribe.FirstName,
		PostalCode:    strings.TrimSpace(subscribe.Address.PostalCode), // this field should be format checked
		//OldMsoAccountId: "", // not needed
		MsoHeadendId: "PHE_TELMAX_CACHE", // hard-code the CACHE 03302023
		//ChannelMapId: "CMAP_TELMAX_UNICAST_RESIDENTIAL", //
		//MsoHeadendId: "Telmax - Cache",
		//ChannelMapId: "Telmax Cache",
		Country: "CA",
	}

	// Get the channels that this account / subscribe combination have, and sort by TV channels.  We may need to get packages too
	var channels []maxbill.SubscribedProduct
	var packages []maxbill.SubscribedProduct

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

	package_filters := []telmax.Filter{
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
			Value: "Package",
		},
	}

	// Iterate over the channels.  If they have an Enghouse code, push onto the array
	channels, err = maxbill.GetServices(CoreDB, channel_filters)
	if err != nil {
		return
	}
	packages, err = maxbill.GetServices(CoreDB, package_filters)
	if err != nil {
		return
	}

	channels = append(channels, packages...) // Add the packgaes to the channels

	for _, channel := range channels {
		var productData maxbill.Product
		productData, err = maxbill.GetProduct(CoreDB, "product_code", channel.ProductCode)
		if err != nil {
			return
		}
		if productData.EnghouseCode != nil && (channel.Status == "New" || channel.Status == "Activate") {
			accountdata.Service = append(accountdata.Service, EngService{
				Service_codes: *productData.EnghouseCode,
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
	var tv_devices []devices.Device
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
	tv_devices, err = devices.GetDevices(CoreDB, device_filters)
	if err != nil {
		return
	}
	var deviceList []EngDevice
	for _, device := range tv_devices {
		deviceList = append(deviceList, EngDevice{
			HardwareDeviceId: device.Mac, // Maybe format this to make sure it is upper / no special chars, etc.
		})
	}
	if len(deviceList) > 0 {
		accountdata.Devices = append(accountdata.Devices, EngDevices{
			Device: deviceList,
		})
	}
	//log.Infof("Account Data for EngHouse is %v", accountdata)
	return

}
