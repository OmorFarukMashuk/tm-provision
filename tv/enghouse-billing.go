package main

import (
   "fmt"
   "bytes"
   "io/ioutil"
   "log"
   "net/http"
   "crypto/tls"
    "encoding/xml"
)

type Enghouse struct {
	XMLName  	xml.Name `xml:"digeoAPI"`
        MSOCode         string    `xml:"MSOCode,attr"`
        Version         string         `xml:"version,attr"`
	Transaction	[]EngTrans	`xml:"transaction"`
}

type EngTrans struct {
//	Transaction     xml.Name      `xml:"transaction"`
	Action		string	   `xml:"action,attr"`
        MSOAccountID    string      `xml:"MSOAccountID,attr"`
        TransID         string     `xml:"transID,attr"`
        TransTime       string     `xml:"transTime,attr"`
        Account_status  string  `xml:"account_status"`
        MSO_account_id  string     `xml:"mso_account_id"`
//        Old_mso_account_id      string     `xml:"old_mso_account_id"`
        MSO_market_id   string  `xml:"mso_market_id"`
        First_name      string  `xml:"first_name"`
        Country         string  `xml:"country"`
//        Headend_id      string  `xml:"headend_id"`
        Channelmap_id   string  `xml:"channelmap_id"`
	Service		[]EngService		`xml:"service_codes"`
	Devices		[]EngDevices		`xml:"devices"`
}

type EngService struct {
	Text	  	string  `xml:",chardata"`
	Service_codes  string	`xml:"service_code"`
}

type EngDevices struct {
	Text		 string   `xml:",chardata"`
	Device 		[]EngDevice   `xml:"device"`
}

type EngDevice struct {
	Text 		string	`xml:",chardata"`
	HardwareDeviceID	string   `xml:"hardwareDeviceID,attr"`
}


func main() {

    var username string = "telmax_billing"
    var passwd string = "691DistanceMedium300"

    tr := &http.Transport{
        TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
    }
    client := &http.Client{Transport: tr}

    xml_string :=  Enghouse{
        MSOCode:  "TELMAX",
        Version: "1.0.0",
        Transaction: []EngTrans{
		{
		Action:	"account.update",
        	MSOAccountID:	"1495738",
        	TransID:	"103085",
        	TransTime:	"20200616181100",
		Account_status:	"ACTIVE",
        	MSO_account_id:	"tim-test-2",
//        	Old_mso_account_id: " ",
        	MSO_market_id:	"Telmax",
        	First_name:	"Tim",
        	Country:	"CA",
//        	Headend_id:	"PHE_TELMAX",
        	Channelmap_id:	"CMAP_TELMAX_UNICAST_RESIDENTIAL",
        	Service: []EngService{
			{
			Service_codes:	"IPTV_NDVR_50",
			},
		}, // `xml:"service_codes"`
		Devices: []EngDevices{
			{
//			Temp:			"",
			Device: []EngDevice{
			{
     			HardwareDeviceID:	"0003E6F6FDA6",
			}, 
			}, // `xml:"device"`
		},  // `xml:"devices"`
		},  // `xml:"EngTrans"`
		},
	},
    }

    xmlStr, err := xml.Marshal(xml_string)
//    if err != nil{
        fmt.Println(err)
//    }
//	myString := []byte(xml.Header + string(xmlStr))
	xmlStr2 := append([]byte(xml.Header), xmlStr...)
//    fmt.Printf("%s\n", myString)
//	fmt.Println(xmlStr)


 xmlStr3 := bytes.NewBuffer(xmlStr2)
 fmt.Printf("%s\n", xmlStr3)

// Main Server
    req, err := http.NewRequest("POST", "https://billing.moxi.com/billing/UpdateAccount/1?MSO=TELMAX
", bytes.NewBuffer(xmlStr2))

//  Testing Server
//    req, err := http.NewRequest("POST", "https://billing-stg.moxi.com/billing/UpdateAccount/1?MSO=
TELMAX", bytes.NewBuffer([]byte(myString)))
	req.Header.Add("Content-Type", "text/plain; charset=utf-8")
//    	req.Header.Add("Content-Length", strconv.Itoa(len(data.Encode())))
    req.SetBasicAuth(username, passwd)
    resp, err := client.Do(req)
	fmt.Println("http", resp.StatusCode, http.StatusText(resp.StatusCode))
	fmt.Println(resp)
	fmt.Println(err)
    if err != nil{
        log.Fatal(err)
    }
    bodyText, err := ioutil.ReadAll(resp.Body)
    s := string(bodyText)
	fmt.Println(s)

}