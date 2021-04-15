package main

import (
	log "github.com/sirupsen/logrus"
	//	"go.mongodb.org/mongo-driver/bson"

	//"strings"

	"telmax-provision/structs"
	//"time"
)

func HandleProvision(request telmaxprovision.ProvisionRequest) {
	log.Infof("Got provision request %v", request)
	switch request.RequestType {
	case "New":
		NewRequest(request)

	case "Update":
		//		NewRequest(request)

	case "DeviceReturn":

	case "Cancel":
	}

}

func NewRequest(request telmaxprovision.ProvisionRequest) {
	result := telmaxprovision.ProvisionResult{
		RequestID: request.RequestID,
		Time:      time.Now(),
	}

	subscribe, err := telmax.GetSubscribe(CoreDB, request.AccountCode, request.SubscribeCode)
	if err != nil {
		log.Errorf("Problem getting subscriber %v", err)
	}
	if subscribe.NetworkType == "Fibre" {
		// Check to see if they have any Internet services
		
		// If yes, then assign an IP address in DHCP
		
		for _, device := range request.Devices {
			if device.DeviceType = "AccessEndpoint"{
				var definition telmax.DeviceDefinition
				definition, err = telmax.GetDeviceDefinition()
				
			}
		}
		// Create ONT record
		
		// Create ONT interfaces
		
		// Add services

	} else {
		log.Info("Not a fibre customer")
	}
	return
}
