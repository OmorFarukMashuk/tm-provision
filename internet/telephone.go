package main

import (
	"encoding/json"
	"errors"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"time"
)

var (
	TelAPI = "http://int02.dc1.osh.telmax.ca:5002"
)

func GetDID(number string) (did DID, err error) {
	url := TelAPI + "/getdid/" + number
	log.Debugf("Query string is %v", url)
	response, err := http.Get(url)
	if err != nil {
		log.Errorf("Problem with HTTP request execution %v", err)
		return
	}
	defer response.Body.Close()
	var result []byte
	result, err = ioutil.ReadAll(response.Body)
	log.Debugf("TelAPI response raw was %v", string(result))
	var apiresponse struct {
		Status string `json:"status"`
		Error  string `json:"error,omitempty"`
		Data   DID    `json:"data"`
	}
	json.Unmarshal(result, &apiresponse)
	if apiresponse.Status == "ok" {
		did = apiresponse.Data
	} else {
		err = errors.New(apiresponse.Error)
	}
	return
}

type DID struct {
	Number         string       `json:"number" bson:"number"`
	Ratecentre     string       `json:"ratecentre" bson:"ratecentre"`
	DIDType        string       `bson:"type,omitempty" json:"type,omitempty"`
	State          string       `json:"state" bson:"state"`
	Provider       string       `json:"provider" bson:"provider"`
	BillingData    *BillingData `json:"billing_data" bson:"billing"`
	BillmaxAccount int          `json:"billmax_account" bson:"billmax_account"`
	UserData       *TelUserData `json:"userData,omitempty" bson:"userData,omitempty"`
	CreatedTime    time.Time    `json:"created,omitempty" bson:"created,omitempty"`
	UpdatedTime    time.Time    `json:"updated,omitempty" bson:"updated,omitempty"`
	PortedTime     time.Time    `json:"ported,omitempty" bson:"ported,omitempty"`
	PortOutTime    time.Time    `json:"portout,omitempty" bson:"portout,omitempty"`
	AgingTime      time.Time    `json:"agingtime,omitempty" bson:"agingtime,omitempty":`
	Origin         string       `json:"origin,omitempty" bson:"origin,omitempty"`
}

type TelUserData struct {
	Voicemail          string `json:"voicemail,omitempty" bson:"voicemail,omitempty"`
	Email              string `json:"email" bson:"email"`
	SIPPassword        string `json:"sipPassword" bson:"sipPassword"`
	PstnCallerIDNumber string `json:"pstnCallerIDNumber" bson:"pstnCallerIDNumber"`
	PstnCallerIDName   string `json:"pstnCallerIDName" bson:"pstnCallerIDName"`
	Callerid           string `json:"callerid" bson:"callerid"`
	EmergencyCallerID  string `json:"emergencyCallerID" bson:"emergencyCallerID"`
	Timeout            int32  `json:"timeout" bson:"timeout"`
	Package            string `json:"package" bson:"package"`
	RateDeck           string `json:"ratedeck" bson:"ratedeck"`
}

type BillingData struct {
	AccountCode    string    `json:"account_code" bson:"account_code"`
	SubscribeCode  string    `json:"subscribe_code" bson:"subscribe_code"`
	ProductCode    string    `json:"product_code" bson: "product_code"`
	ProductName    string    `json:"product_name" bson: "product_name"`
	RegularPrice   float64   `json:"regular_price" bson:"regular_price"`
	DiscountPrice  float64   `json:"discount_price" bson:"discount_price"`
	Status         string    `json:"status" bson:"subscribe_product_status"`
	Discount       bool      `json:"discount" bson:"discount_bool"`
	DiscountMonths int32     `json:"discount_months" bson:"discount_months"`
	StartDate      time.Time `json:"start_date" bson:"start_date"`
	EndDate        time.Time `json:"end_date" bson:"end_date"`
	CreatedDate    time.Time `json:"created_date" bson:"create_date"`
}
