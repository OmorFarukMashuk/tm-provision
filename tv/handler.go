package main

import (
	log "github.com/sirupsen/logrus"
	//	"go.mongodb.org/mongo-driver/bson"
	"bitbucket.org/timstpierre/telmax-common"
	//"strings"

	"telmax-provision/structs"
	//"time"
)

// Determine what sort of request it was
func HandleProvision(request telmaxprovision.ProvisionRequest) {
	log.Infof("Got provision request %v", request)

	switch request.RequestType {
	case "New":
		NewRequest(request)

	case "Update":

	case "DeviceReturn":
		log.Info("Handling returned devices")
		DeviceReturn(request)

	case "Cancel":
	}
}

func DeviceReturn(request telmaxprovision.ProvisionRequest) {
	accountlist := map[string]bool{}
	for _, device := range request.Devices {
		if device.DeviceType == "TVSetTopBox" {
			deviceData, err := telmax.GetDevice(CoreDB, "device_code", device.DeviceCode)
			if err != nil {
				log.Errorf("Problem getting device details for code %v - %v", device.DeviceCode, err)
			} else {
				account := deviceData.Accountcode + deviceData.Subscribecode
				accountlist[account] = true
			}
		}
	}

	for tvaccount, _ := range accountlist {
		// Run an update on the TV account here - this will trigger a refresh without the STBs
		log.Infof("Updating TV account %v", tvaccount)

		// Put your account refresh code here with tvaccount as the account code
	}

}

func NewRequest(requst telmaxprovision.ProvisionRequest) {
	var tvaccount string
	for _, product := range request.Products {
		if product.Category == "TV" {
			tvaccount = request.Accountcode + request.Subscribecode
		}
	}

	for _, device := range request.Devices {
		if device.DeviceType == "TVSetTopBox" {
			tvaccount = request.Accountcode + request.Subscribecode
		}
	}
	if tvaccount != "" {
		// Provision the TV account
	}
}
